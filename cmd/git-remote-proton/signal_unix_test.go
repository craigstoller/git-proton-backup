//go:build !windows

package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/craigstoller/git-proton-backup/internal/repo"
	"github.com/craigstoller/git-proton-backup/internal/testcli"
	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// The Unix end-to-end signal cases (spec §6, layer 3). Each drives the real
// helper binary through signal_harness_test.go and sends it a real signal.

// processAlive reports whether pid names a live process. kill(pid, 0) succeeds
// for a zombie too, so a killed shim reads as alive until its parent reaps it;
// assertNoShimLeft polls to absorb that. A shim whose parent has gone is
// reparented and reaped, and then reads as gone.
func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// setOwnProcessGroup starts the helper as the leader of a new process group
// (helperOpts.OwnGroup), so a test can signal the group without signalling
// itself.
func setOwnProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// TestSignal_E1_ReleasesWhenSignalledDuringThePackUpload is E1: SIGTERM, SIGINT
// and SIGHUP, each sent to the helper alone while its push is blocked in the
// pack upload with the lock held. The helper stops the shim, settles, releases
// the lock through its own CLI instance, says so, and exits 128 + the signal.
func TestSignal_E1_ReleasesWhenSignalledDuringThePackUpload(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP} {
		t.Run(sig.String(), func(t *testing.T) {
			h := startHelper(t, helperOpts{Block: "upload:*.pack:once:before"})
			h.pushMain(t)
			waitReady(t, h.Ready, "ready-upload-", 20*time.Second)
			if err := h.Cmd.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			code := h.wait(t, 20*time.Second)
			stderr := h.Stderr.String()
			if want := 128 + int(sig); code != want {
				t.Errorf("exit code = %d, want %d (128 + %s); stderr:\n%s", code, want, sig, stderr)
			}
			if lockPresent(h.Tree, harnessRoot) {
				t.Errorf("%s/%s is still in the shim tree after the release; stderr:\n%s",
					harnessRoot, repo.LockName, stderr)
			}
			assertNoShimLeft(t, h.Tree)

			name := map[syscall.Signal]string{
				syscall.SIGTERM: "SIGTERM", syscall.SIGINT: "SIGINT", syscall.SIGHUP: "SIGHUP",
			}[sig]
			if catch := linePrefix + "received " + name + ": stopping the push and checking the remote lock"; !strings.Contains(stderr, catch) {
				t.Errorf("stderr lacks the catch line %q:\n%s", catch, stderr)
			}
			removed := regexp.MustCompile(regexp.QuoteMeta(linePrefix+"lock "+harnessRoot+"/"+repo.LockName) +
				` \(nonce [0-9a-f]{32}\): removed: the lock is gone\n`)
			if !removed.MatchString(stderr) {
				t.Errorf("stderr lacks the removed result line for %s/%s:\n%s", harnessRoot, repo.LockName, stderr)
			}
		})
	}
}

// TestSignal_E2_ProcessGroupSignalLikeTimeout is E2: SIGTERM to the helper's
// whole process group, as GNU timeout sends it, while the push is blocked in
// the pack upload with the lock held. The shim, in that group too, dies of the
// same signal at once, so the helper finds it gone inside the grace, settles,
// releases and exits 143. The helper leads a group of its own, so the test
// does not signal itself.
func TestSignal_E2_ProcessGroupSignalLikeTimeout(t *testing.T) {
	h := startHelper(t, helperOpts{Block: "upload:*.pack:once:before", OwnGroup: true})
	h.pushMain(t)
	ready := waitReady(t, h.Ready, "ready-upload-", 20*time.Second)
	pgid, err := syscall.Getpgid(h.Cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if pgid != h.Cmd.Process.Pid || pgid == syscall.Getpgrp() {
		t.Fatalf("the helper (pid %d) is in group %d, the test in %d: want a group the helper leads",
			h.Cmd.Process.Pid, pgid, syscall.Getpgrp())
	}
	// The blocked shim must be in the signalled group, or this is E1 again.
	shim := readyPid(t, ready)
	if g, err := syscall.Getpgid(shim); err != nil || g != pgid {
		t.Fatalf("the blocked shim (pid %d) is in group %d (%v), not the helper's %d", shim, g, err, pgid)
	}
	if !lockPresent(h.Tree, harnessRoot) {
		t.Fatalf("%s/%s is not in the shim tree before the signal", harnessRoot, repo.LockName)
	}
	sent := time.Now()
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	// The group signal, not the helper's own SIGTERM at the end of the grace,
	// is what ends the shim.
	if died := waitGone(t, shim, sent, 10*time.Second); died >= shrunkGrace {
		t.Errorf("the shim was gone %v after the group SIGTERM, not inside the %v grace: "+
			"the group signal did not reach it", died, shrunkGrace)
	}
	code := h.wait(t, 20*time.Second)
	stderr := h.Stderr.String()
	if code != 143 {
		t.Errorf("exit code = %d, want 143 (128 + SIGTERM); stderr:\n%s", code, stderr)
	}
	if lockPresent(h.Tree, harnessRoot) {
		t.Errorf("%s/%s is still in the shim tree after the release; stderr:\n%s", harnessRoot, repo.LockName, stderr)
	}
	assertNoShimLeft(t, h.Tree)
	if !strings.Contains(stderr, pushCatch("SIGTERM")) {
		t.Errorf("stderr lacks the catch line %q:\n%s", pushCatch("SIGTERM"), stderr)
	}
	if _, words := lockResult(t, stderr, harnessRoot); words != removedWords {
		t.Errorf("result = %q, want %q; stderr:\n%s", words, removedWords, stderr)
	}
}

// TestSignal_E3_IdleAfterListForPush is E3: git has the push advertisement and
// the helper holds the lock, idle on stdin, with no CLI call running, when
// SIGTERM comes. There is nothing to stop, so the helper settles, releases and
// exits 143, and says nothing of an interrupted push: no batch had begun.
func TestSignal_E3_IdleAfterListForPush(t *testing.T) {
	h := startHelper(t, helperOpts{})
	h.exchange(t, 20*time.Second, "capabilities\n", "list for-push\n")
	if !lockPresent(h.Tree, harnessRoot) {
		t.Fatalf("%s/%s is not in the shim tree after list for-push", harnessRoot, repo.LockName)
	}
	if err := h.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	code := h.wait(t, 20*time.Second)
	stderr := h.Stderr.String()
	if code != 143 {
		t.Errorf("exit code = %d, want 143 (128 + SIGTERM); stderr:\n%s", code, stderr)
	}
	if lockPresent(h.Tree, harnessRoot) {
		t.Errorf("%s/%s is still in the shim tree after the release; stderr:\n%s", harnessRoot, repo.LockName, stderr)
	}
	assertNoShimLeft(t, h.Tree)
	if !strings.Contains(stderr, pushCatch("SIGTERM")) {
		t.Errorf("stderr lacks the catch line %q:\n%s", pushCatch("SIGTERM"), stderr)
	}
	if strings.Contains(stderr, pushCutShort) {
		t.Errorf("stderr says the push did not finish, but no batch had begun:\n%s", stderr)
	}
	if _, words := lockResult(t, stderr, harnessRoot); words != removedWords {
		t.Errorf("result = %q, want %q; stderr:\n%s", words, removedWords, stderr)
	}
}

// TestSignal_E4_DuringTheLockUpload is E4: SIGTERM while AcquireLock's own
// upload of .lock is held, the lock registered and in doubt. Held before the
// shim writes .lock, the release finds nothing and says absent; held after it,
// the release finds our nonce and removes it.
func TestSignal_E4_DuringTheLockUpload(t *testing.T) {
	for _, tc := range []struct {
		name, block string
		landed      bool // .lock is on the remote when the signal comes
		words       string
	}{
		{"before", "upload:.lock:once:before", false, absentWords},
		{"after", "upload:.lock:once:after", true, removedWords},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := startHelper(t, helperOpts{Block: tc.block})
			h.pushMain(t)
			waitReady(t, h.Ready, "ready-upload-.lock-", 20*time.Second)
			if got := lockPresent(h.Tree, harnessRoot); got != tc.landed {
				t.Fatalf("%s/%s present = %v before the signal, want %v", harnessRoot, repo.LockName, got, tc.landed)
			}
			if err := h.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
				t.Fatal(err)
			}
			code := h.wait(t, 20*time.Second)
			stderr := h.Stderr.String()
			if code != 143 {
				t.Errorf("exit code = %d, want 143 (128 + SIGTERM); stderr:\n%s", code, stderr)
			}
			if lockPresent(h.Tree, harnessRoot) {
				t.Errorf("%s/%s is in the shim tree after the release; stderr:\n%s", harnessRoot, repo.LockName, stderr)
			}
			assertNoShimLeft(t, h.Tree)
			if !strings.Contains(stderr, pushCatch("SIGTERM")) {
				t.Errorf("stderr lacks the catch line %q:\n%s", pushCatch("SIGTERM"), stderr)
			}
			if strings.Contains(stderr, pushCutShort) {
				t.Errorf("stderr says the push did not finish, but no batch had begun:\n%s", stderr)
			}
			if _, words := lockResult(t, stderr, harnessRoot); words != tc.words {
				t.Errorf("result = %q, want %q; stderr:\n%s", words, tc.words, stderr)
			}
		})
	}
}

// TestSignal_E5_GitDiesMidBatch is E5, a case with no signal of the shutdown
// set: git dies while the batch runs (the test closes both of the helper's
// protocol pipes while the pack upload is held), then the shim carries on.
// The helper's status write then fails. SIGPIPE is caught, so the write fails
// with EPIPE instead of killing the helper with the lock held; loop reads EOF,
// its defer releases on the normal path, and the helper exits 1, not by a
// signal and not through the coordinator.
func TestSignal_E5_GitDiesMidBatch(t *testing.T) {
	h := startHelper(t, helperOpts{Block: "upload:*.pack:once:before:delay=1500"})
	h.pushMain(t)
	waitReady(t, h.Ready, "ready-upload-", 20*time.Second)
	if !lockPresent(h.Tree, harnessRoot) {
		t.Fatalf("%s/%s is not in the shim tree while the batch runs", harnessRoot, repo.LockName)
	}
	// git is gone: nobody reads the helper's stdout or writes its stdin. The
	// shim is still in its delay, so the helper has not answered the batch yet.
	h.Stdin.Close()
	h.StdoutR.Close()
	code := h.wait(t, 20*time.Second) // fails the test on a death by SIGPIPE
	stderr := h.Stderr.String()
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (a session whose answers were lost); stderr:\n%s", code, stderr)
	}
	// The batch itself completed, so the 1 is the lost answers, not a push
	// that failed.
	if _, err := os.Stat(testcli.LocalPath(h.Tree, harnessRoot+"/refs/heads/main")); err != nil {
		t.Errorf("the pushed ref is not on the remote: %v; stderr:\n%s", err, stderr)
	}
	if lockPresent(h.Tree, harnessRoot) {
		t.Errorf("%s/%s is still in the shim tree after the normal-path release; stderr:\n%s",
			harnessRoot, repo.LockName, stderr)
	}
	assertNoShimLeft(t, h.Tree)
	if strings.Contains(stderr, linePrefix+"received ") {
		t.Errorf("stderr has a catch line, but no shutdown signal was sent:\n%s", stderr)
	}
	if strings.Contains(stderr, linePrefix+"lock ") {
		t.Errorf("stderr has a coordinator result line, but the coordinator never started:\n%s", stderr)
	}
}

// TestSignal_E6_InheritedIgnoredSIGHUP is E6, a case with no signal the helper
// handles: it starts with SIGHUP already ignored, through a `trap "" HUP; exec`
// wrapper (as nohup leaves it), so it leaves SIGHUP ignored (S2). A SIGHUP while
// the pack upload runs then does nothing: the push completes, the normal path
// releases the lock, and the helper exits 0.
func TestSignal_E6_InheritedIgnoredSIGHUP(t *testing.T) {
	h := startHelper(t, helperOpts{
		Block:   "upload:*.pack:once:before:delay=1500",
		Wrapper: []string{"sh", "-c", `trap "" HUP; exec "$0" "$@"`},
	})
	h.pushMain(t)
	waitReady(t, h.Ready, "ready-upload-", 20*time.Second)
	if !lockPresent(h.Tree, harnessRoot) {
		t.Fatalf("%s/%s is not in the shim tree while the batch runs", harnessRoot, repo.LockName)
	}
	// The wrapper has exec'd the helper by now (its shim is running), and the
	// kernel holds SIGHUP ignored for it, not caught: the helper left the
	// inherited disposition alone.
	if ign, cgt, ok := procSignalMasks(h.Cmd.Process.Pid); !ok {
		t.Logf("no /proc/%d/status here: the SIGHUP disposition is checked through the outcome only", h.Cmd.Process.Pid)
	} else if hup := uint64(1) << (syscall.SIGHUP - 1); ign&hup == 0 || cgt&hup != 0 {
		t.Fatalf("the helper's SIGHUP is not inherited-ignored: SigIgn %016x, SigCgt %016x", ign, cgt)
	}
	if err := h.Cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	code := h.wait(t, 20*time.Second)
	stderr := h.Stderr.String()
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (the push completed; the SIGHUP was ignored); stderr:\n%s", code, stderr)
	}
	if !slices.Contains(h.pushReplies(), "ok refs/heads/main\n") {
		t.Errorf("git was not told the push landed: replies %q; stderr:\n%s", h.pushReplies(), stderr)
	}
	if _, err := os.Stat(testcli.LocalPath(h.Tree, harnessRoot+"/refs/heads/main")); err != nil {
		t.Errorf("the pushed ref is not on the remote: %v; stderr:\n%s", err, stderr)
	}
	if lockPresent(h.Tree, harnessRoot) {
		t.Errorf("%s/%s is still in the shim tree after the normal-path release; stderr:\n%s",
			harnessRoot, repo.LockName, stderr)
	}
	assertNoShimLeft(t, h.Tree)
	if strings.Contains(stderr, linePrefix+"received ") {
		t.Errorf("stderr has a catch line, but SIGHUP was ignored:\n%s", stderr)
	}
	if strings.Contains(stderr, linePrefix+"lock ") {
		t.Errorf("stderr has a coordinator result line, but the coordinator never started:\n%s", stderr)
	}
}

// procSignalMasks reads the ignored (SigIgn) and caught (SigCgt) signal masks
// of pid from /proc. ok is false where there is no /proc (not Linux).
func procSignalMasks(pid int) (ign, cgt uint64, ok bool) {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/status")
	if err != nil {
		return 0, 0, false
	}
	var gotIgn, gotCgt bool
	for _, line := range strings.Split(string(b), "\n") {
		key, val, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		v, perr := strconv.ParseUint(strings.TrimSpace(val), 16, 64)
		switch {
		case key == "SigIgn" && perr == nil:
			ign, gotIgn = v, true
		case key == "SigCgt" && perr == nil:
			cgt, gotCgt = v, true
		}
	}
	return ign, cgt, gotIgn && gotCgt
}

// TestSignal_E7_SetHeadDuringItsHEADUpload is E7: --set-head, sent SIGTERM
// while its HEAD upload is held, its lock held. The catch line names
// --set-head; the coordinator stops the shim, settles and removes the lock,
// says HEAD may or may not have changed, and exits 143.
func TestSignal_E7_SetHeadDuringItsHEADUpload(t *testing.T) {
	h := startHelper(t, helperOpts{
		Args:  []string{"--set-head", harnessRoot, "main"},
		Block: "upload:HEAD:once:before",
		// --set-head reads the remote at once, so the repo must be there first.
		Seed: func(tree string) {
			seedSetHeadTree(t, tree, harnessRoot, "refs/heads/main", "1111111111111111111111111111111111111111")
		},
	})
	waitReady(t, h.Ready, "ready-upload-HEAD-", 20*time.Second)
	if !lockPresent(h.Tree, harnessRoot) {
		t.Fatalf("%s/%s is not in the shim tree while --set-head runs", harnessRoot, repo.LockName)
	}
	if err := h.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	code := h.wait(t, 20*time.Second)
	stderr := h.Stderr.String()
	if code != 143 {
		t.Errorf("exit code = %d, want 143 (128 + SIGTERM); stderr:\n%s", code, stderr)
	}
	if lockPresent(h.Tree, harnessRoot) {
		t.Errorf("%s/%s is still in the shim tree after the release; stderr:\n%s", harnessRoot, repo.LockName, stderr)
	}
	// Held before the shim wrote it, then killed: HEAD did not change here.
	if _, err := os.Stat(testcli.LocalPath(h.Tree, harnessRoot+"/HEAD")); !os.IsNotExist(err) {
		t.Errorf("%s/HEAD exists after its upload was killed before the write (stat: %v); stderr:\n%s", harnessRoot, err, stderr)
	}
	assertNoShimLeft(t, h.Tree)
	catch := linePrefix + "received SIGTERM: stopping --set-head and checking the remote lock, which takes about a minute"
	if !strings.Contains(stderr, catch) {
		t.Errorf("stderr lacks the catch line %q:\n%s", catch, stderr)
	}
	if strings.Contains(stderr, pushCutShort) {
		t.Errorf("stderr says a push did not finish, but this was --set-head:\n%s", stderr)
	}
	headLine := linePrefix + "--set-head was interrupted: HEAD may or may not have changed; re-running the same --set-head settles it\n"
	at, result := strings.Index(stderr, headLine), strings.Index(stderr, linePrefix+"lock ")
	switch {
	case at < 0:
		t.Errorf("stderr lacks the HEAD line %q:\n%s", headLine, stderr)
	case result >= 0 && at > result:
		t.Errorf("the HEAD line comes after the result line:\n%s", stderr)
	}
	if _, words := lockResult(t, stderr, harnessRoot); words != removedWords {
		t.Errorf("result = %q, want %q; stderr:\n%s", words, removedWords, stderr)
	}
}

// TestSignal_E8_FetchDuringADownload is E8: a fetch, signalled while its pack
// download is held. A fetch takes no lock, so the coordinator says it holds
// none, stops the shim, and exits with no settle and no release: within 3 s at
// the shrunk timings when only the helper is signalled (the shim is stopped
// after the grace). The second subtest is Review Focus 3, a terminal Ctrl-C at
// the production timings: SIGINT to the helper's whole group reaches the shim
// too, which dies of it, so the helper exits 130 within 2 s, inside its own 5 s
// grace, let alone a 5 s settle. A lock tripwire shows no .lock upload ever
// ran.
func TestSignal_E8_FetchDuringADownload(t *testing.T) {
	for _, tc := range []struct {
		name    string
		group   bool // the signal goes to the helper's group, at production timings
		sig     syscall.Signal
		sigName string
		within  time.Duration
	}{
		{"helper SIGTERM", false, syscall.SIGTERM, "SIGTERM", 3 * time.Second},
		{"group signal, production timings", true, syscall.SIGINT, "SIGINT", 2 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seed, sha := pushedRepoSeed(t)
			h := startHelper(t, helperOpts{
				Block:      "download:*.pack:once:before;" + lockTripwire,
				GitDir:     filepath.Join(emptyGitRepo(t), ".git"),
				Seed:       seed,
				OwnGroup:   tc.group,
				Production: tc.group,
			})
			// git's fetch: the advertisement, then one want, whose reply never
			// comes.
			h.exchange(t, 20*time.Second, "capabilities\n", "list\n")
			if _, err := io.WriteString(h.Stdin, "fetch "+sha+" refs/heads/main\n\n"); err != nil {
				t.Fatal(err)
			}
			shim := readyPid(t, waitReady(t, h.Ready, "ready-download-", 20*time.Second))
			if lockPresent(h.Tree, harnessRoot) {
				t.Fatalf("%s/%s is in the shim tree during a fetch", harnessRoot, repo.LockName)
			}
			var sent time.Time
			if tc.group {
				pgid, err := syscall.Getpgid(h.Cmd.Process.Pid)
				if err != nil {
					t.Fatal(err)
				}
				if pgid != h.Cmd.Process.Pid || pgid == syscall.Getpgrp() {
					t.Fatalf("the helper (pid %d) is in group %d, the test in %d: want a group the helper leads",
						h.Cmd.Process.Pid, pgid, syscall.Getpgrp())
				}
				if g, err := syscall.Getpgid(shim); err != nil || g != pgid {
					t.Fatalf("the blocked shim (pid %d) is in group %d (%v), not the helper's %d", shim, g, err, pgid)
				}
				sent = time.Now()
				if err := syscall.Kill(-pgid, tc.sig); err != nil {
					t.Fatal(err)
				}
				// The helper itself would not stop the shim before its 5 s
				// grace: gone sooner, it died of the group signal.
				if died := waitGone(t, shim, sent, 10*time.Second); died >= tc.within {
					t.Errorf("the shim was gone %v after the group %s, not within %v: the group signal did not end it",
						died, tc.sigName, tc.within)
				}
			} else {
				sent = time.Now()
				if err := h.Cmd.Process.Signal(tc.sig); err != nil {
					t.Fatal(err)
				}
			}
			code := h.wait(t, 20*time.Second)
			took := time.Since(sent)
			stderr := h.Stderr.String()
			if want := 128 + int(tc.sig); code != want {
				t.Errorf("exit code = %d, want %d (128 + %s); stderr:\n%s", code, want, tc.sigName, stderr)
			}
			if took > tc.within {
				t.Errorf("the helper exited %v after the %s, not within %v; stderr:\n%s", took, tc.sigName, tc.within, stderr)
			}
			if lockPresent(h.Tree, harnessRoot) {
				t.Errorf("%s/%s is in the shim tree after a fetch; stderr:\n%s", harnessRoot, repo.LockName, stderr)
			}
			assertNoLockUpload(t, h.Ready)
			assertNoShimLeft(t, h.Tree)
			if !strings.Contains(stderr, noLockCatch(tc.sigName)) {
				t.Errorf("stderr lacks the no-lock catch line %q:\n%s", noLockCatch(tc.sigName), stderr)
			}
			if strings.Contains(stderr, linePrefix+"lock ") {
				t.Errorf("stderr has a result line, but no lock was registered:\n%s", stderr)
			}
		})
	}
}

// pushedRepoSeed pushes a new one-commit repo's main into a Fake through loop
// (pushViaLoop), and returns a Seed that copies that remote into a shim tree,
// and the commit pushed: a real marker, ref and pack pair for a fetch to read.
func pushedRepoSeed(t *testing.T) (seed func(tree string), sha string) {
	t.Helper()
	src := newGitRepoWithCommit(t)
	ft := transport.NewFake()
	pushViaLoop(t, ft, harnessRoot, src)
	return func(tree string) {
		for d := range ft.Dirs {
			if err := os.MkdirAll(testcli.LocalPath(tree, d), 0o755); err != nil {
				t.Fatalf("seed %s: %v", d, err)
			}
		}
		for p, body := range ft.Files {
			seedRemote(t, tree, p, body)
		}
	}, headOf(t, src)
}

// lockTripwire is a block rule that changes nothing (a ready file after the
// upload, then no delay) but leaves a ready-upload-.lock-* file behind if a
// .lock upload ever ran: no lock was ever created, not only none is there at
// the end.
const lockTripwire = "upload:.lock:every:after:delay=0"

// assertNoLockUpload fails the test if lockTripwire fired.
func assertNoLockUpload(t *testing.T, ready string) {
	t.Helper()
	entries, err := os.ReadDir(ready)
	if err != nil {
		t.Fatalf("read the ready dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "ready-upload-"+repo.LockName+"-") {
			t.Errorf("a .lock upload ran (%s), but this case must never create a lock", e.Name())
		}
	}
}

// TestSignal_E9_SecondSignalDuringASlowRelease is E9: SIGTERM during the pack
// upload, then SIGINT while the release's trash call is slow (held 1.5 s, well
// inside the release call's bound). The second signal is only noted; the
// release completes, and the exit code is the first signal's.
func TestSignal_E9_SecondSignalDuringASlowRelease(t *testing.T) {
	h := startHelper(t, helperOpts{Block: "upload:*.pack:once:before;trash:.lock:once:before:delay=1500"})
	h.pushMain(t)
	waitReady(t, h.Ready, "ready-upload-", 20*time.Second)
	if !lockPresent(h.Tree, harnessRoot) {
		t.Fatalf("%s/%s is not in the shim tree before the first signal", harnessRoot, repo.LockName)
	}
	if err := h.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	waitReady(t, h.Ready, "ready-trash-", 20*time.Second)
	if !lockPresent(h.Tree, harnessRoot) {
		t.Fatalf("%s/%s is gone before the held trash has run", harnessRoot, repo.LockName)
	}
	if err := h.Cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	code := h.wait(t, 20*time.Second)
	stderr := h.Stderr.String()
	if code != 143 {
		t.Errorf("exit code = %d, want 143 (128 + SIGTERM, the first signal); stderr:\n%s", code, stderr)
	}
	if lockPresent(h.Tree, harnessRoot) {
		t.Errorf("%s/%s is still in the shim tree after the release; stderr:\n%s", harnessRoot, repo.LockName, stderr)
	}
	assertNoShimLeft(t, h.Tree)
	if !strings.Contains(stderr, pushCatch("SIGTERM")) {
		t.Errorf("stderr lacks the catch line %q:\n%s", pushCatch("SIGTERM"), stderr)
	}
	if strings.Contains(stderr, pushCatch("SIGINT")) {
		t.Errorf("the second signal got a catch line of its own:\n%s", stderr)
	}
	still := stillRunning("SIGINT")
	at, result := strings.Index(stderr, still), strings.Index(stderr, linePrefix+"lock ")
	switch {
	case at < 0:
		t.Errorf("stderr lacks the still-running line %q:\n%s", still, stderr)
	case result >= 0 && at > result:
		t.Errorf("the still-running line comes after the result line:\n%s", stderr)
	}
	if !strings.Contains(stderr, pushCutShort) {
		t.Errorf("stderr lacks the line saying the push did not finish:\n%s", stderr)
	}
	if _, words := lockResult(t, stderr, harnessRoot); words != removedWords {
		t.Errorf("result = %q, want %q; stderr:\n%s", words, removedWords, stderr)
	}
}

// TestSignal_E10_ReleaseThatNeverEnds is E10: SIGTERM during the pack upload,
// and every trash of .lock then hangs, so each release attempt is killed at
// its bound and retried until the (shrunk) cap ends the process. The result is
// failed, with the manual remedy for this lock's nonce; .lock stays; the exit
// code is still 143.
func TestSignal_E10_ReleaseThatNeverEnds(t *testing.T) {
	h := startHelper(t, helperOpts{Block: "upload:*.pack:once:before;trash:.lock:every:before"})
	h.pushMain(t)
	waitReady(t, h.Ready, "ready-upload-", 20*time.Second)
	if !lockPresent(h.Tree, harnessRoot) {
		t.Fatalf("%s/%s is not in the shim tree before the signal", harnessRoot, repo.LockName)
	}
	sent := time.Now()
	if err := h.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	code := h.wait(t, shrunkCap+5*time.Second)
	took := time.Since(sent)
	stderr := h.Stderr.String()
	if code != 143 {
		t.Errorf("exit code = %d, want 143 (128 + SIGTERM); stderr:\n%s", code, stderr)
	}
	if took < shrunkCap {
		t.Errorf("the helper exited %v after the signal, before the %v cap: something other than the cap ended it; stderr:\n%s",
			took, shrunkCap, stderr)
	}
	if !lockPresent(h.Tree, harnessRoot) {
		t.Errorf("%s/%s is gone, but no trash was ever let through; stderr:\n%s", harnessRoot, repo.LockName, stderr)
	}
	assertNoShimLeft(t, h.Tree)
	// The release did reach its trash: the hang is the one this case is about.
	if _, err := os.Stat(filepath.Join(h.Ready, "ready-trash-.lock-1")); err != nil {
		t.Errorf("the release's trash never reached the shim: %v; stderr:\n%s", err, stderr)
	}
	if !strings.Contains(stderr, pushCatch("SIGTERM")) {
		t.Errorf("stderr lacks the catch line %q:\n%s", pushCatch("SIGTERM"), stderr)
	}
	if !strings.Contains(stderr, pushCutShort) {
		t.Errorf("stderr lacks the line saying the push did not finish:\n%s", stderr)
	}
	nonce, words := lockResult(t, stderr, harnessRoot)
	reason, ok := strings.CutPrefix(words, "failed: ")
	remedy := "; " + remedyWords(harnessRoot+"/"+repo.LockName, nonce)
	if !ok || !strings.HasSuffix(reason, remedy) || reason == remedy {
		t.Errorf("result = %q, want \"failed: <reason>%s\"; stderr:\n%s", words, remedy, stderr)
	}
}

// TestSignal_E11_DuringTheMarkerUpload is E11: SIGTERM while Bootstrap's marker
// upload into an empty root is held, before any lock exists. The coordinator
// says it holds no lock, stops the shim and exits 143. No .lock upload ever
// runs, and the marker, held before its write, is not there either.
func TestSignal_E11_DuringTheMarkerUpload(t *testing.T) {
	h := startHelper(t, helperOpts{Block: "upload:" + repo.MarkerName + ":once:before;" + lockTripwire})
	h.pushMain(t)
	waitReady(t, h.Ready, "ready-upload-"+repo.MarkerName+"-", 20*time.Second)
	if lockPresent(h.Tree, harnessRoot) {
		t.Fatalf("%s/%s is in the shim tree before Bootstrap has finished", harnessRoot, repo.LockName)
	}
	if err := h.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	code := h.wait(t, 20*time.Second)
	stderr := h.Stderr.String()
	if code != 143 {
		t.Errorf("exit code = %d, want 143 (128 + SIGTERM); stderr:\n%s", code, stderr)
	}
	if lockPresent(h.Tree, harnessRoot) {
		t.Errorf("%s/%s is in the shim tree, but no lock was ever taken; stderr:\n%s", harnessRoot, repo.LockName, stderr)
	}
	assertNoLockUpload(t, h.Ready)
	if _, err := os.Stat(testcli.LocalPath(h.Tree, harnessRoot+"/"+repo.MarkerName)); !os.IsNotExist(err) {
		t.Errorf("%s/%s exists after its upload was killed before the write (stat: %v); stderr:\n%s",
			harnessRoot, repo.MarkerName, err, stderr)
	}
	assertNoShimLeft(t, h.Tree)
	if !strings.Contains(stderr, noLockCatch("SIGTERM")) {
		t.Errorf("stderr lacks the no-lock catch line %q:\n%s", noLockCatch("SIGTERM"), stderr)
	}
	if strings.Contains(stderr, linePrefix+"lock ") {
		t.Errorf("stderr has a result line, but no lock was registered:\n%s", stderr)
	}
}

// TestSignal_E12_DuringTheVersionCheck is E12: SIGTERM while EnforceCertified's
// --version call is held, before the helper reads anything from git, so no
// lock can exist. The coordinator says so, stops the shim after the grace, and
// exits 143 with the shim gone.
func TestSignal_E12_DuringTheVersionCheck(t *testing.T) {
	h := startHelper(t, helperOpts{Block: "--version:*:once:before"})
	shim := readyPid(t, waitReady(t, h.Ready, "ready---version-", 20*time.Second))
	sent := time.Now()
	if err := h.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	// Signalled alone, the shim ends only by the helper's stop: no sooner than
	// the grace.
	if died := waitGone(t, shim, sent, 10*time.Second); died < shrunkGrace {
		t.Errorf("the --version shim was gone %v after the signal, before the %v grace", died, shrunkGrace)
	}
	code := h.wait(t, 20*time.Second)
	stderr := h.Stderr.String()
	if code != 143 {
		t.Errorf("exit code = %d, want 143 (128 + SIGTERM); stderr:\n%s", code, stderr)
	}
	if lockPresent(h.Tree, harnessRoot) {
		t.Errorf("%s/%s is in the shim tree, but no lock was ever taken; stderr:\n%s", harnessRoot, repo.LockName, stderr)
	}
	assertNoShimLeft(t, h.Tree)
	if !strings.Contains(stderr, noLockCatch("SIGTERM")) {
		t.Errorf("stderr lacks the no-lock catch line %q:\n%s", noLockCatch("SIGTERM"), stderr)
	}
	if strings.Contains(stderr, linePrefix+"lock ") {
		t.Errorf("stderr has a result line, but no lock was registered:\n%s", stderr)
	}
}

// TestSignal_E13_RightAfterLoopReturns is E13: the push completes, loop
// returns and its defer removes the lock, and a seam holds run() there, before
// finish stops the notifications, when SIGTERM lands. With nothing registered
// the coordinator gives the no-lock catch line and ends the process with 143.
// Only once that line is out is run() let go, so the signal is the
// coordinator's to handle, not Go's default after finish.
func TestSignal_E13_RightAfterLoopReturns(t *testing.T) {
	h := startHelper(t, helperOpts{Env: []string{holdAfterLoopEnv + "=1"}})
	h.pushMain(t)
	waitReady(t, h.Ready, afterLoopReady, 20*time.Second)
	if !slices.Contains(h.pushReplies(), "ok refs/heads/main\n") {
		t.Fatalf("loop returned without git being told the push landed: replies %q; stderr:\n%s",
			h.pushReplies(), h.Stderr.String())
	}
	if lockPresent(h.Tree, harnessRoot) {
		t.Fatalf("%s/%s is still in the shim tree after loop returned: its defer did not remove it", harnessRoot, repo.LockName)
	}
	if err := h.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	h.waitStderr(t, noLockCatch("SIGTERM"), 20*time.Second)
	// The helper may already be exiting: an error here changes nothing.
	_ = os.WriteFile(filepath.Join(h.Ready, afterLoopGo), nil, 0o644)
	code := h.wait(t, 20*time.Second)
	stderr := h.Stderr.String()
	if code != 143 {
		t.Errorf("exit code = %d, want 143 (128 + SIGTERM); stderr:\n%s", code, stderr)
	}
	if lockPresent(h.Tree, harnessRoot) {
		t.Errorf("%s/%s is in the shim tree after the helper exited; stderr:\n%s", harnessRoot, repo.LockName, stderr)
	}
	assertNoShimLeft(t, h.Tree)
	if strings.Contains(stderr, pushCatch("SIGTERM")) {
		t.Errorf("stderr has the lock-holding catch line, but the lock was already released:\n%s", stderr)
	}
	if strings.Contains(stderr, linePrefix+"lock ") {
		t.Errorf("stderr has a result line, but no lock was registered:\n%s", stderr)
	}
}

// TestSignal_E14_ForeignLockDuringAcquire is E14: another process's lock is
// on the remote, and SIGTERM comes while our own lock upload is held before
// the shim's skip decision. Our attempt is still registered, in doubt, when
// the shim is killed; the release reads the remote, finds another nonce, says
// foreign, and leaves that lock exactly as it was.
//
// (A signal after the Refused acquisition has returned gives the no-lock catch
// line instead: the entry has already left the register.)
func TestSignal_E14_ForeignLockDuringAcquire(t *testing.T) {
	const theirs = "0123456789abcdef0123456789abcdef"
	foreign := []byte(`{"nonce":"` + theirs + `","host":"another-host","pid":4242,"acquiredAt":"2026-10-25T00:00:00Z"}`)
	h := startHelper(t, helperOpts{Block: "upload:.lock:once:before"})
	// An existing repo, locked by someone else. The helper touches nothing
	// remote before list for-push, so seeding now is in time.
	seedRemote(t, h.Tree, harnessRoot+"/"+repo.MarkerName, []byte(`{"format":"git-remote-proton","version":1}`))
	seedRemote(t, h.Tree, harnessRoot+"/"+repo.LockName, foreign)
	h.pushMain(t)
	waitReady(t, h.Ready, "ready-upload-.lock-", 20*time.Second)
	lockFile := testcli.LocalPath(h.Tree, harnessRoot+"/"+repo.LockName)
	if got, err := os.ReadFile(lockFile); err != nil || string(got) != string(foreign) {
		t.Fatalf("the other process's lock is not intact before the signal: %q, %v", got, err)
	}
	if err := h.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	code := h.wait(t, 20*time.Second)
	stderr := h.Stderr.String()
	if code != 143 {
		t.Errorf("exit code = %d, want 143 (128 + SIGTERM); stderr:\n%s", code, stderr)
	}
	if got, err := os.ReadFile(lockFile); err != nil {
		t.Errorf("the other process's lock is gone: %v; stderr:\n%s", err, stderr)
	} else if string(got) != string(foreign) {
		t.Errorf("the other process's lock was changed:\n got %q\nwant %q\nstderr:\n%s", got, foreign, stderr)
	}
	assertNoShimLeft(t, h.Tree)
	if !strings.Contains(stderr, pushCatch("SIGTERM")) {
		t.Errorf("stderr lacks the catch line %q:\n%s", pushCatch("SIGTERM"), stderr)
	}
	if strings.Contains(stderr, pushCutShort) {
		t.Errorf("stderr says the push did not finish, but no batch had begun:\n%s", stderr)
	}
	nonce, words := lockResult(t, stderr, harnessRoot)
	if nonce == theirs {
		t.Errorf("the result line names the other process's nonce, not ours:\n%s", stderr)
	}
	if words != foreignWords {
		t.Errorf("result = %q, want %q; stderr:\n%s", words, foreignWords, stderr)
	}
}

// TestSignal_E15_LateLandingLockUpload is E15: SIGTERM while our lock upload
// is held before the shim writes .lock, and a server that applies that upload
// late, a set delay after the shim has died (the shim's late writer). Landing
// 100 ms after, inside the 300 ms settle, it is seen and removed. Landing 2 s
// after, past the settle and the release, the release finds nothing and says
// absent, and the lock that turns up later bears the nonce that line named:
// the residual the spec accepts, made recognisable.
func TestSignal_E15_LateLandingLockUpload(t *testing.T) {
	for _, tc := range []struct {
		name  string
		late  time.Duration
		words string
	}{
		{"insideTheSettle", 100 * time.Millisecond, removedWords},
		{"afterTheSettle", 2000 * time.Millisecond, absentWords},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := startHelper(t, helperOpts{Block: "upload:.lock:once:before:late=" +
				strconv.FormatInt(tc.late.Milliseconds(), 10)})
			h.pushMain(t)
			waitReady(t, h.Ready, "ready-upload-.lock-", 20*time.Second)
			if lockPresent(h.Tree, harnessRoot) {
				t.Fatalf("%s/%s is in the shim tree before the held upload has been killed", harnessRoot, repo.LockName)
			}
			if err := h.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
				t.Fatal(err)
			}
			code := h.wait(t, 20*time.Second)
			exited := time.Now()
			stderr := h.Stderr.String()
			if code != 143 {
				t.Errorf("exit code = %d, want 143 (128 + SIGTERM); stderr:\n%s", code, stderr)
			}
			if !strings.Contains(stderr, pushCatch("SIGTERM")) {
				t.Errorf("stderr lacks the catch line %q:\n%s", pushCatch("SIGTERM"), stderr)
			}
			if strings.Contains(stderr, pushCutShort) {
				t.Errorf("stderr says the push did not finish, but no batch had begun:\n%s", stderr)
			}
			nonce, words := lockResult(t, stderr, harnessRoot)
			if words != tc.words {
				t.Errorf("result = %q, want %q; stderr:\n%s", words, tc.words, stderr)
			}
			if tc.words == removedWords {
				// The late writer has landed and gone once no shim is left, so
				// nothing can still put .lock back.
				assertNoShimLeft(t, h.Tree)
				if lockPresent(h.Tree, harnessRoot) {
					t.Errorf("%s/%s is in the shim tree after a release that said removed; stderr:\n%s",
						harnessRoot, repo.LockName, stderr)
				}
				return
			}
			// absent: true when it was said, and then the late landing comes.
			if lockPresent(h.Tree, harnessRoot) {
				t.Errorf("%s/%s is already in the shim tree when the helper, having said absent, exits; stderr:\n%s",
					harnessRoot, repo.LockName, stderr)
			}
			for !lockPresent(h.Tree, harnessRoot) {
				if time.Since(exited) > 2500*time.Millisecond {
					t.Fatalf("%s/%s had not landed 2.5s after the helper exited: is the late writer running?",
						harnessRoot, repo.LockName)
				}
				time.Sleep(20 * time.Millisecond)
			}
			if got := remoteLockNonce(t, h.Tree, harnessRoot); got != nonce {
				t.Errorf("the lock that landed late bears nonce %s, but the absent line named %s", got, nonce)
			}
			assertNoShimLeft(t, h.Tree)
		})
	}
}

// TestSignal_E16_ShimIgnoresSIGTERM is E16: SIGTERM to the helper alone while
// the pack upload's shim ignores SIGTERM. The helper's own SIGTERM to it, at
// the end of the grace, does nothing; after the SIGTERM wait it kills the shim,
// sees the exit confirmed, and only then settles and releases.
func TestSignal_E16_ShimIgnoresSIGTERM(t *testing.T) {
	h := startHelper(t, helperOpts{Block: "upload:*.pack:once:before:ignoreterm"})
	h.pushMain(t)
	shim := readyPid(t, waitReady(t, h.Ready, "ready-upload-", 20*time.Second))
	if !lockPresent(h.Tree, harnessRoot) {
		t.Fatalf("%s/%s is not in the shim tree before the signal", harnessRoot, repo.LockName)
	}
	sent := time.Now()
	if err := h.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	// Killed, not ended by the SIGTERM: no sooner than grace + SIGTERM wait,
	// and inside the kill's confirmation, or the helper would release nothing.
	died := waitGone(t, shim, sent, 10*time.Second)
	if died < shrunkGrace+shrunkTerm {
		t.Errorf("the shim was gone %v after the signal, before childGrace+childTerm (%v)", died, shrunkGrace+shrunkTerm)
	}
	if died >= shrunkGrace+shrunkTerm+shrunkConfirm {
		t.Errorf("the shim was gone %v after the signal, after the kill's confirmation window ended at %v",
			died, shrunkGrace+shrunkTerm+shrunkConfirm)
	}
	code := h.wait(t, 20*time.Second)
	stderr := h.Stderr.String()
	if code != 143 {
		t.Errorf("exit code = %d, want 143 (128 + SIGTERM); stderr:\n%s", code, stderr)
	}
	if lockPresent(h.Tree, harnessRoot) {
		t.Errorf("%s/%s is still in the shim tree after the release; stderr:\n%s", harnessRoot, repo.LockName, stderr)
	}
	assertNoShimLeft(t, h.Tree)
	if !strings.Contains(stderr, pushCatch("SIGTERM")) {
		t.Errorf("stderr lacks the catch line %q:\n%s", pushCatch("SIGTERM"), stderr)
	}
	if !strings.Contains(stderr, pushCutShort) {
		t.Errorf("stderr lacks the line saying the push did not finish:\n%s", stderr)
	}
	if _, words := lockResult(t, stderr, harnessRoot); words != removedWords {
		t.Errorf("result = %q, want %q; stderr:\n%s", words, removedWords, stderr)
	}
}

// TestSignal_E17_StderrNeverRead is E17: the helper's stderr is a pipe nobody
// reads, and the test fills it before SIGTERM comes while the pack upload is
// held with the lock held. Every coordinator line then blocks in its writer,
// and the release's traced download goes through the same queue, so neither
// holds up the release: .lock is removed, and the helper exits within about a
// second of that (the result lines get their outcomeWait and no more), far
// inside the cap.
func TestSignal_E17_StderrNeverRead(t *testing.T) {
	h := startHelper(t, helperOpts{Block: "upload:*.pack:once:before", RawStderr: true})
	h.pushMain(t)
	waitReady(t, h.Ready, "ready-upload-", 20*time.Second)
	if !lockPresent(h.Tree, harnessRoot) {
		t.Fatalf("%s/%s is not in the shim tree before the signal", harnessRoot, repo.LockName)
	}
	// 128 KB, twice a usual pipe buffer, into the helper's own stderr pipe. The
	// goroutine blocks once the pipe is full; teardown's close of the pipe ends
	// it.
	const fill = 128 << 10
	var written atomic.Int64
	go func() {
		chunk := []byte(strings.Repeat("x", 1023) + "\n")
		for written.Load() < fill {
			if _, err := h.StderrW.Write(chunk); err != nil {
				return
			}
			written.Add(int64(len(chunk)))
		}
	}()
	// Full once the count has stopped short of fill for 200 ms.
	for last, since, deadline := int64(-1), time.Now(), time.Now().Add(10*time.Second); ; time.Sleep(10 * time.Millisecond) {
		n := written.Load()
		switch {
		case n >= fill:
			t.Fatalf("all %d bytes went into the helper's stderr pipe without a reader: it never filled", n)
		case time.Now().After(deadline):
			t.Fatalf("the stderr pipe had not filled 10s on (%d bytes written)", n)
		case n != last:
			last, since = n, time.Now()
			continue
		}
		if n > 0 && time.Since(since) >= 200*time.Millisecond {
			break
		}
	}
	exitedAt := make(chan time.Time, 1)
	go func() {
		<-h.exited
		exitedAt <- time.Now()
	}()
	sent := time.Now()
	if err := h.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	var gone time.Time
	for deadline := sent.Add(shrunkCap + 5*time.Second); ; {
		if !lockPresent(h.Tree, harnessRoot) {
			gone = time.Now()
			break
		}
		select {
		case <-h.exited:
			if lockPresent(h.Tree, harnessRoot) {
				t.Fatalf("the helper exited with %s/%s still in the shim tree", harnessRoot, repo.LockName)
			}
		case <-time.After(5 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s/%s was still in the shim tree %v after the signal", harnessRoot, repo.LockName, time.Since(sent))
		}
	}
	code := h.wait(t, 20*time.Second)
	var exit time.Time
	select {
	case exit = <-exitedAt: // the exit has been seen, so this is sent or about to be
	case <-time.After(5 * time.Second):
		t.Fatal("the exit time was not recorded 5s after the exit")
	}
	if code != 143 {
		t.Errorf("exit code = %d, want 143 (128 + SIGTERM)", code)
	}
	if d := exit.Sub(gone); d > time.Second {
		t.Errorf("the helper exited %v after %s/%s was removed, not within about 1s", d, harnessRoot, repo.LockName)
	}
	if d := exit.Sub(sent); d >= shrunkCap {
		t.Errorf("the helper exited %v after the signal, at or past the %v cap", d, shrunkCap)
	}
	assertNoShimLeft(t, h.Tree)
	if lockPresent(h.Tree, harnessRoot) {
		t.Errorf("%s/%s is back in the shim tree after the helper exited", harnessRoot, repo.LockName)
	}
}

// TestSignal_E18_NoSignalLockUploadErrorsThenLandsLate is E18, a case with no
// signal: AcquireLock's .lock upload fails (the shim exits 1 with no summary),
// and a server applies that upload late, 100 ms after the shim has died. The
// acquisition is in doubt, so the push fails with exit 1, and loop's defer
// waits out the settle before its one release attempt: by then the late lock
// has landed, and the release reads it and removes it.
func TestSignal_E18_NoSignalLockUploadErrorsThenLandsLate(t *testing.T) {
	h := startHelper(t, helperOpts{Block: "upload:.lock:once:before:late=100:fail"})
	h.pushMain(t)
	waitReady(t, h.Ready, "ready-upload-.lock-", 20*time.Second)
	code := h.wait(t, 20*time.Second)
	stderr := h.Stderr.String()
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (the lock upload failed); stderr:\n%s", code, stderr)
	}
	// Once the late writer has gone, nothing can still put .lock in place.
	assertNoShimLeft(t, h.Tree)
	if lockPresent(h.Tree, harnessRoot) {
		t.Errorf("%s/%s is in the shim tree: the release looked before the late lock landed; stderr:\n%s",
			harnessRoot, repo.LockName, stderr)
	}
	// The release found the late lock and read it. Had it found nothing, it
	// would only have Stat'd, and the failed acquisition read nothing back.
	if !strings.Contains(stderr, "gpb: downloaded "+harnessRoot+"/"+repo.LockName+" ") {
		t.Errorf("stderr shows no download of %s/%s: the release never saw the late lock:\n%s", harnessRoot, repo.LockName, stderr)
	}
	// The helper's only line is the failed upload's warning: no catch line, no
	// result line, and no release failure.
	var ours []string
	for _, line := range strings.SplitAfter(stderr, "\n") {
		if strings.HasPrefix(line, linePrefix) {
			ours = append(ours, line)
		}
	}
	if len(ours) != 1 || !strings.HasPrefix(ours[0], linePrefix+"unparseable upload summary: ") {
		t.Errorf("want exactly one line of the helper's own, the failed lock upload's warning; got %q; stderr:\n%s", ours, stderr)
	}
}

// TestSignal_E19_UnconfirmedChildExit is E19: SIGTERM while the pack upload is
// held with the lock held, and the stop of the in-flight child reports its exit
// unconfirmed (GPB_TEST_UNCONFIRMED_STOP: the shim is still stopped, and only
// the coordinator is told otherwise). Nothing is then settled or released:
// .lock stays, reported not attempted with the manual remedy for its nonce,
// and the exit code is still 143.
func TestSignal_E19_UnconfirmedChildExit(t *testing.T) {
	h := startHelper(t, helperOpts{Block: "upload:*.pack:once:before", Env: []string{unconfirmedStopEnv + "=1"}})
	h.pushMain(t)
	waitReady(t, h.Ready, "ready-upload-", 20*time.Second)
	if !lockPresent(h.Tree, harnessRoot) {
		t.Fatalf("%s/%s is not in the shim tree before the signal", harnessRoot, repo.LockName)
	}
	if err := h.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	code := h.wait(t, 20*time.Second)
	stderr := h.Stderr.String()
	if code != 143 {
		t.Errorf("exit code = %d, want 143 (128 + SIGTERM); stderr:\n%s", code, stderr)
	}
	if !lockPresent(h.Tree, harnessRoot) {
		t.Fatalf("%s/%s is gone, but no release may run when the child's exit is unconfirmed; stderr:\n%s",
			harnessRoot, repo.LockName, stderr)
	}
	assertNoShimLeft(t, h.Tree)
	if !strings.Contains(stderr, pushCatch("SIGTERM")) {
		t.Errorf("stderr lacks the catch line %q:\n%s", pushCatch("SIGTERM"), stderr)
	}
	// No release attempt: nothing was read from the remote after the catch line.
	if at := strings.Index(stderr, pushCatch("SIGTERM")); at >= 0 && strings.Contains(stderr[at:], "gpb: downloaded ") {
		t.Errorf("a release read the remote after the signal, though the child's exit was unconfirmed:\n%s", stderr)
	}
	if !strings.Contains(stderr, pushCutShort) {
		t.Errorf("stderr lacks the line saying the push did not finish:\n%s", stderr)
	}
	nonce, words := lockResult(t, stderr, harnessRoot)
	if got := remoteLockNonce(t, h.Tree, harnessRoot); got != nonce {
		t.Errorf("the result line names nonce %s, but the lock left on the remote bears %s", nonce, got)
	}
	want := "not attempted: the CLI call that was running when the signal arrived did not confirm its exit, " +
		"so a release could have raced that call's own write; " + remedyWords(harnessRoot+"/"+repo.LockName, nonce)
	if words != want {
		t.Errorf("result = %q\nwant     %q\nstderr:\n%s", words, want, stderr)
	}
}

// TestSignal_E20_DuringParentFolderCreation is E20 (Review Focus 4): SIGTERM
// while EnsureParents, under GPB_CREATE_PARENTS=1, is creating a missing parent
// folder of the repo root, before any lock exists. The coordinator says it
// holds no lock, stops the shim and exits 143 promptly, with no settle after
// the stop. No .lock upload ever runs, and no marker is written.
func TestSignal_E20_DuringParentFolderCreation(t *testing.T) {
	const parent = "/my-files/parent" // missing: EnsureParents creates it
	const root = parent + "/r"
	h := startHelper(t, helperOpts{
		Address: root,
		Block:   "create-folder:*:once:before;" + lockTripwire,
		Env:     []string{createParentsEnv + "=1"},
	})
	h.pushMain(t)
	waitReady(t, h.Ready, "ready-create-folder-parent-", 20*time.Second)
	if _, err := os.Stat(testcli.LocalPath(h.Tree, parent)); !os.IsNotExist(err) {
		t.Fatalf("%s is in the shim tree before its creation was let through (stat: %v)", parent, err)
	}
	if lockPresent(h.Tree, root) {
		t.Fatalf("%s/%s is in the shim tree before the push has reached its lock", root, repo.LockName)
	}
	sent := time.Now()
	if err := h.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	code := h.wait(t, 20*time.Second)
	took := time.Since(sent)
	stderr := h.Stderr.String()
	if code != 143 {
		t.Errorf("exit code = %d, want 143 (128 + SIGTERM); stderr:\n%s", code, stderr)
	}
	// The stop takes the grace (the shim is signalled only after it); a settle
	// after the stop would add shrunkSettle.
	if took >= shrunkGrace+shrunkSettle {
		t.Errorf("the helper exited %v after the signal, not before grace + settle (%v): it waited after the stop; stderr:\n%s",
			took, shrunkGrace+shrunkSettle, stderr)
	}
	if lockPresent(h.Tree, root) {
		t.Errorf("%s/%s is in the shim tree, but no lock was ever taken; stderr:\n%s", root, repo.LockName, stderr)
	}
	assertNoLockUpload(t, h.Ready)
	if _, err := os.Stat(testcli.LocalPath(h.Tree, root+"/"+repo.MarkerName)); !os.IsNotExist(err) {
		t.Errorf("%s/%s exists, but the push never reached Bootstrap (stat: %v); stderr:\n%s", root, repo.MarkerName, err, stderr)
	}
	assertNoShimLeft(t, h.Tree)
	if !strings.Contains(stderr, noLockCatch("SIGTERM")) {
		t.Errorf("stderr lacks the no-lock catch line %q:\n%s", noLockCatch("SIGTERM"), stderr)
	}
	if strings.Contains(stderr, linePrefix+"lock ") {
		t.Errorf("stderr has a result line, but no lock was registered:\n%s", stderr)
	}
}
