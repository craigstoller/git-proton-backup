//go:build !windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/craigstoller/git-proton-backup/internal/repo"
	"github.com/craigstoller/git-proton-backup/internal/testcli"
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
	still := linePrefix + "received SIGINT while already stopping: ignored; the shutdown, and any lock check, is still running"
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
