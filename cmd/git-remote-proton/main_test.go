package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/craigstoller/git-proton-backup/internal/gitcmd"
	"github.com/craigstoller/git-proton-backup/internal/repo"
	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// captureStderr redirects os.Stderr for the duration of fn and returns what
// was written to it. warn() writes straight to os.Stderr, and the point of the
// tests below is precisely that a message REACHES the operator, so the
// assertion has to be made against the real stream rather than an injected
// writer that would not prove anything about the shipped path.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stderr-*")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	orig := os.Stderr
	os.Stderr = f
	defer func() { os.Stderr = orig }()

	fn()

	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	b, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatalf("read back stderr: %v", err)
	}
	return string(b)
}

// errReader always fails with err, simulating a genuine stdin read failure
// (a broken pipe, git crashing mid-session) rather than a clean EOF. It never
// reaches io.EOF, so bufio.Scanner.Err() must return err, not nil.
type errReader struct{ err error }

func (r errReader) Read(p []byte) (int, error) { return 0, r.err }

// Finding 1: a non-EOF scan error must fail closed (exit 1), not be treated
// the same as a clean session end (exit 0). Before the fix, in.Err() was
// never checked after the loop, so this returned 0.
func TestLoop_ScanErrorFailsClosed(t *testing.T) {
	in := bufio.NewScanner(errReader{err: errors.New("broken pipe")})
	out := bufio.NewWriter(&bytes.Buffer{})

	got := loop(transport.NewFake(), "/remote/root", ".", in, out)

	if got != 1 {
		t.Fatalf("loop() = %d, want 1: a non-EOF scan error must be reported as failure, not silently treated as a clean end of session", got)
	}
}

// A clean EOF (no read error) must still succeed — the fix must not turn
// every ordinary session end into a failure.
func TestLoop_CleanEOFSucceeds(t *testing.T) {
	in := bufio.NewScanner(strings.NewReader("capabilities\n"))
	out := bufio.NewWriter(&bytes.Buffer{})

	got := loop(transport.NewFake(), "/remote/root", ".", in, out)

	if got != 0 {
		t.Fatalf("loop() = %d, want 0: a clean EOF with no read error must not be reported as failure", got)
	}
}

// Finding 3: a "push" batch with no preceding successful "list for-push"
// must be rejected before it can write anything, because the lock (acquired
// only inside "list for-push") is what serializes writers. Before the fix
// there was no guard, and the batch would have gone straight into
// repo.Push, writing packs/refs with no mutual exclusion.
func TestLoop_PushWithoutListForPushRejected(t *testing.T) {
	in := bufio.NewScanner(strings.NewReader("push refs/heads/main:refs/heads/main\n\n"))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)
	ft := transport.NewFake()

	got := loop(ft, "/remote/root", ".", in, out)
	out.Flush()

	if got != 1 {
		t.Fatalf("loop() = %d, want 1: push before list for-push must be rejected", got)
	}
	if len(ft.Files) != 0 || len(ft.Dirs) != 0 {
		t.Fatalf("transport was written to despite no lock ever being held: Files=%v Dirs=%v", ft.Files, ft.Dirs)
	}
	if outBuf.Len() != 0 {
		t.Fatalf("stdout got %q, want nothing: a rejected push must not emit protocol output", outBuf.String())
	}
}

// TestLoop_TruncatedPushBatchFailsClosed covers a git that dies while still
// sending a push batch: the push line arrives, the blank line that ends the
// batch never does. The batch-buffering loop also stops on EOF or a read
// error, and before the fix nothing checked which, so the truncated batch was
// parsed and handed to repo.Push as if complete — a pack uploaded and a ref
// written that nobody asked for in that shape.
//
// gitDir is a real repository with a commit, so the push WOULD succeed if it
// ran: the no-pack and no-ref assertions fail on the unfixed code rather than
// passing because the push happened to break for some other reason. stdout
// being exactly the list-for-push advertisement proves the lock was acquired
// (it is taken before advertising), so the lock assertion checks a real
// release rather than a lock that was never there.
//
// The refusal must come before any remote READ too, and before the poison
// check: the List count pins the first (a check that slid past the push arm's
// ScanRefs would still write nothing), and the poisoned case the second (a
// check after the poison branch would print its status lines and exit 0).
func TestLoop_TruncatedPushBatchFailsClosed(t *testing.T) {
	const script = "list for-push\npush refs/heads/main:refs/heads/main\n" // no blank line
	for _, tc := range []struct {
		name       string
		in         io.Reader
		wantStderr []string
	}{
		{"eof", strings.NewReader(script), []string{"terminating blank line", "unexpected EOF"}},
		{"eof poisoned", strings.NewReader(script + "option cas true\n"),
			[]string{"terminating blank line", "unexpected EOF"}},
		{"read error", io.MultiReader(strings.NewReader(script), errReader{err: errors.New("broken pipe")}),
			[]string{"terminating blank line", "broken pipe"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ft := transport.NewFake()
			root := "/my-files/r"
			counter := &refsListCounter{Fake: ft, root: root}
			gitDir := newGitRepoWithCommit(t)
			in := bufio.NewScanner(tc.in)
			var outBuf bytes.Buffer
			out := bufio.NewWriter(&outBuf)

			var got int
			stderr := captureStderr(t, func() { got = loop(counter, root, gitDir, in, out) })
			out.Flush()

			if got != 1 {
				t.Errorf("loop() = %d, want 1: a push batch git never finished sending must fail closed", got)
			}
			if outBuf.String() != "\n" {
				t.Errorf("stdout = %q, want only the list-for-push advertisement (\"\\n\"): no push status lines for a batch that was never applied", outBuf.String())
			}
			if counter.n != 1 {
				t.Errorf("List(%s/refs) ran %d times, want 1 (the list-for-push advertisement's scan): a truncated batch must be refused before the push arm reads the remote", root, counter.n)
			}
			for _, w := range tc.wantStderr {
				if !strings.Contains(stderr, w) {
					t.Errorf("stderr = %q, want it to contain %q", stderr, w)
				}
			}
			for p := range ft.Files {
				if strings.HasPrefix(p, root+"/packs/") {
					t.Errorf("no pack may be uploaded for a truncated batch, found %s", p)
				}
			}
			if _, ok := ft.Files[root+"/refs/heads/main"]; ok {
				t.Errorf("a truncated batch must not write the ref: found %s/refs/heads/main", root)
			}
			if _, ok := ft.Files[root+"/"+repo.LockName]; ok {
				t.Errorf("the lock must still be released when a truncated batch fails closed: %s/%s is present", root, repo.LockName)
			}
		})
	}
}

// refsListCounter wraps a Fake and counts List calls on root+"/refs", the
// first call every repo.ScanRefs makes, so a test can tell how many ref scans
// a session ran against the remote.
type refsListCounter struct {
	*transport.Fake
	root string
	n    int
}

func (c *refsListCounter) List(p string) ([]transport.Node, error) {
	if p == c.root+"/refs" {
		c.n++
	}
	return c.Fake.List(p)
}

// ambiguousTrashTransport wraps a Fake but makes Trash report Ambiguous with a
// nil error — the shape that drives repo.Lock.Release to return its "release
// is unconfirmed" error without any I/O failure occurring. Fake's FailNext is
// only consulted by CreateExclusive, so it cannot produce this.
type ambiguousTrashTransport struct{ *transport.Fake }

func (ambiguousTrashTransport) Trash(string) (transport.Outcome, error) {
	return transport.Ambiguous, nil
}

// TestLoop_LockReleaseFailureIsReportedButDoesNotFailThePush covers the one
// place on this branch where a fully-formed error was silently dropped:
// `_ = lock.Release()`. repo.Lock.Release was given a whole fix round to write
// two operator messages, and neither could ever reach anyone — the push exited
// 0, stderr was silent, .lock stayed on the remote, and since v2 has no
// takeover every subsequent push failed until a human cleared it by hand with
// no idea why.
//
// The second assertion is as important as the first: the exit code must NOT
// escalate. The push itself succeeded, and reporting failure for a completed
// push would make git discard remote-tracking updates for refs that really did
// land.
func TestLoop_LockReleaseFailureIsReportedButDoesNotFailThePush(t *testing.T) {
	ft := transport.NewFake()
	in := bufio.NewScanner(strings.NewReader("list for-push\n\n"))
	out := bufio.NewWriter(&bytes.Buffer{})

	var got int
	stderr := captureStderr(t, func() {
		got = loop(ambiguousTrashTransport{ft}, "/my-files/r", ".", in, out)
	})

	if got != 0 {
		t.Errorf("loop() = %d, want 0: a failed lock release must not turn a completed session into a reported failure", got)
	}
	if !strings.Contains(stderr, "unconfirmed") {
		t.Errorf("stderr = %q, want the unconfirmed-release message; discarding it leaves the operator with a wedged repo and no diagnostic", stderr)
	}
	if !strings.Contains(stderr, "/my-files/r/.lock") {
		t.Errorf("stderr = %q, want the lock path named so it can be cleared by hand", stderr)
	}
}

// TestLoop_ListForPush_CreateParentsEnvCreatesParents is RED (Task 11): the
// protocol path honours GPB_CREATE_PARENTS, set via t.Setenv exactly as the
// task's environment rules require — read fresh from the real process
// environment through createParentsEnv, the same route production's run()
// uses, never injected as a parameter. Missing parents above the repo root
// are created with loud stderr notes, and the push still completes
// end-to-end (Bootstrap runs afterward and writes the marker).
func TestLoop_ListForPush_CreateParentsEnvCreatesParents(t *testing.T) {
	t.Setenv(createParentsEnv, "1")
	ft := transport.NewFake()
	ft.Dirs["/my-files"] = true // the mount; "GitRemotes" deliberately not seeded
	root := "/my-files/GitRemotes/repo"

	in := bufio.NewScanner(strings.NewReader("list for-push\n\n"))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)

	var got int
	stderr := captureStderr(t, func() { got = loop(ft, root, ".", in, out) })
	out.Flush()

	if got != 0 {
		t.Fatalf("loop() = %d, want 0: stderr=%q", got, stderr)
	}
	if !ft.Dirs["/my-files/GitRemotes"] {
		t.Error("the missing parent must have been created")
	}
	if !strings.Contains(stderr, "created parent folder /my-files/GitRemotes") {
		t.Errorf("stderr = %q, want a loud note naming the created parent", stderr)
	}
	if !strings.Contains(stderr, "GPB_CREATE_PARENTS=1") {
		t.Errorf("stderr = %q, want the loud note to name the env var responsible", stderr)
	}
	// The push must still have gone on to Bootstrap the repo itself —
	// EnsureParents creating the parent is not the whole story.
	if _, ok := ft.Files[root+"/"+repo.MarkerName]; !ok {
		t.Error("Bootstrap must still run after EnsureParents succeeds: no marker was written")
	}
}

// TestLoop_ListForPush_MissingParentWithoutEnvGivesActionableMessageNotRawError
// is the brief's Step 3 confirmation, RED (Task 11): a fresh Fake with only
// "/my-files" present (its GitRemotes subfolder never seeded) and NO
// GPB_CREATE_PARENTS set must fail the push with EnsureParents' actionable
// refusal — never Bootstrap's raw "Node not found: GitRemotes" (Surprise
// R2-1's original, remedy-less failure). t.Setenv pins the var explicitly
// unset so this is hermetic regardless of the real process environment.
func TestLoop_ListForPush_MissingParentWithoutEnvGivesActionableMessageNotRawError(t *testing.T) {
	t.Setenv(createParentsEnv, "")
	ft := transport.NewFake()
	ft.Dirs["/my-files"] = true
	root := "/my-files/GitRemotes/repo"

	in := bufio.NewScanner(strings.NewReader("list for-push\n\n"))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)

	var got int
	stderr := captureStderr(t, func() { got = loop(ft, root, ".", in, out) })
	out.Flush()

	if got != 1 {
		t.Fatalf("loop() = %d, want 1: a missing parent with the env unset must be refused", got)
	}
	if !strings.Contains(stderr, "proton-drive filesystem create-folder /my-files GitRemotes") {
		t.Errorf("stderr = %q, want the actionable remedy in the CLI's real grammar", stderr)
	}
	if !strings.Contains(stderr, "GPB_CREATE_PARENTS=1") {
		t.Errorf("stderr = %q, want the env var named as the other remedy", stderr)
	}
	if strings.Contains(stderr, "Node not found") {
		t.Errorf("stderr = %q, must not leak the raw create-folder failure text (Surprise R2-1)", stderr)
	}
	if ft.Dirs["/my-files/GitRemotes"] {
		t.Error("a refused push must not create anything")
	}
	if _, ok := ft.Files[root+"/"+repo.MarkerName]; ok {
		t.Error("a refused push must never reach Bootstrap")
	}
}

// stubProbe points the restore-floor seam (D5(d)) at fn for one test and puts
// the previous value back afterwards. TestMain has already stubbed the seam to
// an OK no-op for the whole package, so a test only needs this when it cares
// what the probe says.
func stubProbe(t *testing.T, fn func(gitDir string) (gitcmd.ProbeResult, string)) {
	t.Helper()
	old := probeRestoreFloor
	probeRestoreFloor = fn
	t.Cleanup(func() { probeRestoreFloor = old })
}

// warningLines returns the lines of stderr that carry the WARNING marker.
func warningLines(stderr string) []string {
	var got []string
	for _, l := range strings.Split(stderr, "\n") {
		if strings.Contains(l, "WARNING") {
			got = append(got, l)
		}
	}
	return got
}

// TestLoop_ListForPush_WarnsWhenThisGitIsTooOld is RED (Task 4, D5(d)): git
// 2.45.0 or newer is needed to RESTORE from this remote but not to push to it,
// so on an older git a push succeeds on a host that could never get the data
// back. "list for-push" must say so, once, on stderr, and must not turn the
// warning into a failed push: the advertisement completes and loop returns 0.
//
// The seam closure also pins WHERE the probe runs: after Bootstrap has created
// the remote layout and BEFORE the lock is taken, so a slow probe never holds
// the single-writer lock open.
func TestLoop_ListForPush_WarnsWhenThisGitIsTooOld(t *testing.T) {
	ft := transport.NewFake()
	root := "/my-files/r"
	const detail = "git rev-list: fatal: bad object 0123456789abcdef0123456789abcdef01234567"
	var calls int
	var gotGitDir string
	var markerThere, lockThere bool
	stubProbe(t, func(gitDir string) (gitcmd.ProbeResult, string) {
		calls++
		gotGitDir = gitDir
		_, markerThere = ft.Files[root+"/"+repo.MarkerName]
		_, lockThere = ft.Files[root+"/"+repo.LockName]
		return gitcmd.ProbeTooOld, detail
	})

	in := bufio.NewScanner(strings.NewReader("list for-push\n\n"))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)

	var got int
	stderr := captureStderr(t, func() { got = loop(ft, root, "/the/git/dir", in, out) })
	out.Flush()

	if got != 0 {
		t.Fatalf("loop() = %d, want 0: the warning must never block the push; stderr=%q", got, stderr)
	}
	if outBuf.String() != "\n" {
		t.Errorf("stdout = %q, want the empty advertisement %q: the protocol stream must be unaffected", outBuf.String(), "\n")
	}
	if calls != 1 {
		t.Fatalf("probe ran %d times, want exactly once", calls)
	}
	if gotGitDir != "/the/git/dir" {
		t.Errorf("probe got gitDir %q, want the one loop was given", gotGitDir)
	}
	if !markerThere {
		t.Error("the probe ran before Bootstrap wrote the remote layout; it belongs after")
	}
	if lockThere {
		t.Error("the probe ran with the lock already held; it belongs before AcquireLock")
	}
	warns := warningLines(stderr)
	if len(warns) != 1 {
		t.Fatalf("stderr has %d WARNING lines, want exactly one: %q", len(warns), stderr)
	}
	// GUARD assertion (final-review fix C7(c)): the warning is the ONLY stderr
	// line, so nothing else this path prints can split or bury it.
	if n := strings.Count(stderr, "\n"); n != 1 {
		t.Errorf("stderr has %d lines, want exactly one (the warning): %q", n, stderr)
	}
	for _, want := range []string{
		"cannot restore", "git 2.45.0 or newer", "does not block the push", detail,
	} {
		if !strings.Contains(warns[0], want) {
			t.Errorf("warning %q does not contain %q", warns[0], want)
		}
	}
	if strings.Contains(outBuf.String(), "WARNING") {
		t.Errorf("stdout carried the warning: %q", outBuf.String())
	}
}

// TestLoop_ListForPush_WarnsWhenTheCheckCannotRun is RED (Task 4): a probe that
// could not tell (timeout, git missing, an erroring git) is a different message
// from "too old", and it too never fails the push.
func TestLoop_ListForPush_WarnsWhenTheCheckCannotRun(t *testing.T) {
	stubProbe(t, func(string) (gitcmd.ProbeResult, string) {
		return gitcmd.ProbeUnknown, "git rev-list: context deadline exceeded"
	})
	in := bufio.NewScanner(strings.NewReader("list for-push\n\n"))
	out := bufio.NewWriter(&bytes.Buffer{})

	var got int
	stderr := captureStderr(t, func() { got = loop(transport.NewFake(), "/my-files/r", ".", in, out) })

	if got != 0 {
		t.Fatalf("loop() = %d, want 0: an inconclusive probe must never block the push; stderr=%q", got, stderr)
	}
	warns := warningLines(stderr)
	if len(warns) != 1 {
		t.Fatalf("stderr has %d WARNING lines, want exactly one: %q", len(warns), stderr)
	}
	for _, want := range []string{
		"could not check", "git 2.45.0 or newer", "does not block the push", "context deadline exceeded",
	} {
		if !strings.Contains(warns[0], want) {
			t.Errorf("warning %q does not contain %q", warns[0], want)
		}
	}
	if strings.Contains(warns[0], "cannot restore") {
		t.Errorf("warning %q claims this git cannot restore, which the probe did not establish", warns[0])
	}
}

// TestLoop_ListForPush_UnexpectedProbeResultWarnsLikeUnknown is RED
// (final-review fix C7(b)): a ProbeResult warnRestoreFloor does not know (a
// value added to gitcmd later, say) must read as "could not check", never as
// a silent pass.
func TestLoop_ListForPush_UnexpectedProbeResultWarnsLikeUnknown(t *testing.T) {
	stubProbe(t, func(string) (gitcmd.ProbeResult, string) {
		return gitcmd.ProbeResult(99), "git rev-list: something new"
	})
	in := bufio.NewScanner(strings.NewReader("list for-push\n\n"))
	out := bufio.NewWriter(&bytes.Buffer{})

	var got int
	stderr := captureStderr(t, func() { got = loop(transport.NewFake(), "/my-files/r", ".", in, out) })

	if got != 0 {
		t.Fatalf("loop() = %d, want 0: an unexpected probe result must never block the push; stderr=%q", got, stderr)
	}
	warns := warningLines(stderr)
	if len(warns) != 1 {
		t.Fatalf("stderr has %d WARNING lines, want exactly one: %q", len(warns), stderr)
	}
	for _, want := range []string{
		"could not check", "git 2.45.0 or newer", "does not block the push", "something new",
	} {
		if !strings.Contains(warns[0], want) {
			t.Errorf("warning %q does not contain %q", warns[0], want)
		}
	}
}

// TestLoop_ListForPush_SilentWhenTheProbeIsOK is RED (Task 4; it failed to
// compile before the probe seam existed): a passing probe adds nothing to
// stderr.
func TestLoop_ListForPush_SilentWhenTheProbeIsOK(t *testing.T) {
	stubProbe(t, func(string) (gitcmd.ProbeResult, string) { return gitcmd.ProbeOK, "" })
	in := bufio.NewScanner(strings.NewReader("list for-push\n\n"))
	out := bufio.NewWriter(&bytes.Buffer{})

	var got int
	stderr := captureStderr(t, func() { got = loop(transport.NewFake(), "/my-files/r", ".", in, out) })

	if got != 0 {
		t.Fatalf("loop() = %d, want 0; stderr=%q", got, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want nothing on ProbeOK", stderr)
	}
}

// TestLoop_ListForPush_RealProbeIsSilentOnTheSuitesGit is RED (Task 4) and pins
// the PRODUCTION wiring: with the seam set to the real probe (TestMain saved it
// in realProbeRestoreFloor before stubbing), a fresh repo's absolute git dir on
// the suite's own git (at the floor) yields no warning at all.
func TestLoop_ListForPush_RealProbeIsSilentOnTheSuitesGit(t *testing.T) {
	stubProbe(t, realProbeRestoreFloor)
	gitDir := filepath.Join(emptyGitRepo(t), ".git")
	in := bufio.NewScanner(strings.NewReader("list for-push\n\n"))
	out := bufio.NewWriter(&bytes.Buffer{})

	var got int
	stderr := captureStderr(t, func() { got = loop(transport.NewFake(), "/my-files/r", gitDir, in, out) })

	if got != 0 {
		t.Fatalf("loop() = %d, want 0; stderr=%q", got, stderr)
	}
	if strings.Contains(stderr, "WARNING") {
		t.Errorf("stderr = %q, want no WARNING from the real probe on a git at the floor", stderr)
	}
}

// TestRun_RefusesUnsupportedAddress proves the canonicalisation is actually
// WIRED IN, not merely present: run() must reject the address and return
// non-zero before it reaches the command loop. /shared-with-me is the case
// worth pinning here because it is a safety rule — the CLI permits creating
// there, and a repo in a folder another person can write to is exactly the
// concurrent-writer failure the single-writer model cannot survive.
//
// Hermetic: run() returns at the canonicalisation check, before it constructs
// a transport or reads a byte of stdin. No proton-drive CLI is invoked.
func TestRun_RefusesUnsupportedAddress(t *testing.T) {
	cases := []struct{ name, addr, wantIn string }{
		{"shared-with-me", "proton::/shared-with-me/theirrepo", "concurrent-writer"},
		{"foreign namespace", "proton::/trash/r", "must lie under"},
		{"empty", "proton::", "empty remote address"},
		{"dotdot", "proton::/my-files/../devices/r", "rejected rather than resolved"},
	}
	orig := os.Args
	defer func() { os.Args = orig }()

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			os.Args = []string{"git-remote-proton", "proton-v2", c.addr}
			var got int
			stderr := captureStderr(t, func() { got = run() })
			if got != 1 {
				t.Errorf("run() = %d, want 1 for %q", got, c.addr)
			}
			if !strings.Contains(stderr, c.wantIn) {
				t.Errorf("stderr = %q, want it to name the reason (%q)", stderr, c.wantIn)
			}
		})
	}
}

// chdirForTest changes the process's working directory to dir for the
// duration of the test and restores it afterward via t.Cleanup.
//
// Not testing.Chdir: that requires Go 1.24, and this module's go.mod
// deliberately floors at go 1.22 (documented there — chosen for
// per-iteration loop variables; the floor was never meant to gate on
// testing.Chdir, and bumping the module's declared floor for one test's
// convenience is a bigger decision than this fix belongs to). No test in
// this package calls t.Parallel(), so the process-wide cwd this mutates is
// not at risk of a concurrent sibling test reading it mid-change.
func chdirForTest(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%s): %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatalf("restore Chdir(%s): %v", orig, err)
		}
	})
}

// sameDir reports whether want and got name the same directory on disk, and
// returns a detail for the caller's failure message. A raw string compare is
// wrong for the paths below on macOS: t.TempDir() builds on $TMPDIR, spelled
// /var/folders/..., and /var is a symlink to /private/var. chdirForTest's
// os.Chdir leaves $PWD naming the old directory, so os.Getwd (and so
// filepath.Abs, which resolveGitDir and absolutizeInheritedGitPaths use)
// falls back to the kernel's getcwd and answers /private/var/folders/... Same
// directory, two spellings. The first macOS run of this suite (Apple Silicon,
// 2026-10-23) failed all three tests that compared them as strings; Linux's
// /tmp is not a symlink, so they had always passed there.
//
// This is the comparison internal/repo's assertSamePackDir makes for the same
// class of alias (Windows 8.3 short names): filepath.EvalSymlinks on both
// sides, then os.SameFile if the resolved strings still differ. A genuinely
// different directory still fails. EvalSymlinks needs the path to exist, so
// callers create the directory first; the functions under test are lexical
// and never look at the disk, so that changes nothing they do. If either side
// cannot be resolved, the raw compare decides, with the errors in the detail.
// It compares identity only: a relative got such as "." resolves against the
// cwd, which these tests have made wt, so it would pass. Callers assert
// filepath.IsAbs first.
func sameDir(want, got string) (bool, string) {
	wantEval, wErr := filepath.EvalSymlinks(want)
	gotEval, gErr := filepath.EvalSymlinks(got)
	if wErr == nil && gErr == nil {
		if wantEval == gotEval {
			return true, ""
		}
		wantInfo, wStatErr := os.Stat(wantEval)
		gotInfo, gStatErr := os.Stat(gotEval)
		if wStatErr == nil && gStatErr == nil && os.SameFile(wantInfo, gotInfo) {
			return true, ""
		}
		return false, fmt.Sprintf("normalised: got=%s want=%s", gotEval, wantEval)
	}
	return want == got, fmt.Sprintf("path normalisation failed: got-err=%v want-err=%v", gErr, wErr)
}

// TestResolveGitDirMakesARelativeGITDIRAbsolute covers the Stage 3a live-gate
// finding (task 7): git commonly sets GIT_DIR to a RELATIVE path (".git" is
// its own default), and every incremental fetch failed live because a
// relative gitDir got resolved TWICE by two separate `-C gitDir` subprocess
// invocations inside the fetch install path (see resolveGitDir's doc in
// main.go for the full mechanism). resolveGitDir must turn a relative
// GIT_DIR into an absolute path anchored at the process's cwd, exactly once,
// at the source, before anything downstream ever sees it.
func TestResolveGitDirMakesARelativeGITDIRAbsolute(t *testing.T) {
	wt := t.TempDir()
	// Exists only so sameDir can resolve it; resolveGitDir never looks.
	if err := os.MkdirAll(filepath.Join(wt, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	chdirForTest(t, wt)
	t.Setenv("GIT_DIR", ".git")

	got, err := resolveGitDir()
	if err != nil {
		t.Fatalf("resolveGitDir: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("resolveGitDir() = %q, want an absolute path", got)
	}
	want := filepath.Join(wt, ".git")
	if same, detail := sameDir(want, got); !same {
		t.Errorf("resolveGitDir() = %q, want %q (%s)", got, want, detail)
	}
}

// TestResolveGitDirDefaultsToAbsoluteCwdWhenUnset mirrors run()'s
// pre-existing "" -> "." default, and pins that the default is ALSO resolved
// to absolute — the bug this fix round closes did not care whether GIT_DIR
// was explicitly ".git" or defaulted there; either way it was relative.
func TestResolveGitDirDefaultsToAbsoluteCwdWhenUnset(t *testing.T) {
	wt := t.TempDir()
	chdirForTest(t, wt)
	// t.Setenv, not os.Unsetenv: os.Unsetenv LEAKS — it mutates the process
	// environment for every test that runs after this one in the same binary,
	// with no restore. t.Setenv registers the restore automatically, and an
	// empty GIT_DIR is what resolveGitDir's own "" -> "." default keys on, so
	// it exercises the same branch.
	t.Setenv("GIT_DIR", "")

	got, err := resolveGitDir()
	if err != nil {
		t.Fatalf("resolveGitDir: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("resolveGitDir() = %q, want an absolute path", got)
	}
	if same, detail := sameDir(wt, got); !same {
		t.Errorf("resolveGitDir() = %q, want %q (%s)", got, wt, detail)
	}
}

// TestAbsolutizeInheritedGitPaths covers the rest of GIT_DIR's class. A
// relative GIT_COMMON_DIR or GIT_OBJECT_DIRECTORY means one directory to this
// process and a different one to every `git -C <gitDir> ...` child, and for
// GIT_COMMON_DIR the resulting install path still ENDS in "/objects/pack", so
// validateObjectsPackPath passes it — objects written where git never reads,
// with connectivity-ok reported anyway. Fail-open, which is why absolutising
// (rather than unsetting, which for GIT_OBJECT_DIRECTORY would be fail-open in
// its own right) is the fix. See resolveGitDir's doc.
func TestAbsolutizeInheritedGitPaths(t *testing.T) {
	wt := t.TempDir()
	commonDir := filepath.Join(filepath.Dir(wt), "main", ".git")
	objectDir := filepath.Join(wt, "objects")
	// Exist only so sameDir can resolve them; absolutizeInheritedGitPaths never
	// looks. filepath.Dir(wt) is t.TempDir()'s own per-test root, which the
	// testing package removes whole, so the sibling is cleaned up too.
	for _, d := range []string{commonDir, objectDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	chdirForTest(t, wt)

	t.Setenv("GIT_COMMON_DIR", "../main/.git")
	t.Setenv("GIT_OBJECT_DIRECTORY", "objects")
	// Left alone deliberately: GIT_WORK_TREE cannot move where objects live.
	t.Setenv("GIT_WORK_TREE", "../tree")

	if err := absolutizeInheritedGitPaths(); err != nil {
		t.Fatalf("absolutizeInheritedGitPaths: %v", err)
	}

	for _, name := range []string{"GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY"} {
		got := os.Getenv(name)
		if !filepath.IsAbs(got) {
			t.Errorf("%s = %q, want an absolute path", name, got)
		}
	}
	if same, detail := sameDir(commonDir, os.Getenv("GIT_COMMON_DIR")); !same {
		t.Errorf("GIT_COMMON_DIR = %q, want %q — resolution must be anchored at this "+
			"process's cwd, the same cwd the relative value was already implicitly "+
			"relative to (%s)", os.Getenv("GIT_COMMON_DIR"), commonDir, detail)
	}
	if same, detail := sameDir(objectDir, os.Getenv("GIT_OBJECT_DIRECTORY")); !same {
		t.Errorf("GIT_OBJECT_DIRECTORY = %q, want %q (%s)", os.Getenv("GIT_OBJECT_DIRECTORY"), objectDir, detail)
	}
	if got := os.Getenv("GIT_WORK_TREE"); got != "../tree" {
		t.Errorf("GIT_WORK_TREE = %q, want it untouched: it names the working tree, not the "+
			"object store, so it cannot move where a pack is installed", got)
	}
}

// TestAbsolutizeInheritedGitPathsLeavesUnsetAndAbsoluteValuesAlone is the
// companion guard. An UNSET variable must stay unset — inventing a value would
// nominate an object store the caller never asked for, which is the fail-open
// case this whole function exists to avoid — and an already-absolute value
// must pass through byte-identical rather than being rewritten by Abs's
// cleaning rules.
func TestAbsolutizeInheritedGitPathsLeavesUnsetAndAbsoluteValuesAlone(t *testing.T) {
	chdirForTest(t, t.TempDir())

	abs := filepath.Join(t.TempDir(), "odb")
	t.Setenv("GIT_OBJECT_DIRECTORY", abs)
	t.Setenv("GIT_COMMON_DIR", "")

	if err := absolutizeInheritedGitPaths(); err != nil {
		t.Fatalf("absolutizeInheritedGitPaths: %v", err)
	}
	if got := os.Getenv("GIT_OBJECT_DIRECTORY"); got != abs {
		t.Errorf("GIT_OBJECT_DIRECTORY = %q, want it unchanged at %q", got, abs)
	}
	if got := os.Getenv("GIT_COMMON_DIR"); got != "" {
		t.Errorf("GIT_COMMON_DIR = %q, want it left empty: an empty value is not a relative "+
			"path, and turning it into the cwd would invent an object store", got)
	}
}

// Deliberately NOT testing resolveGitDir's wiring into run() end to end:
// run() reaches gitDir capture only AFTER CanonicalRoot accepts the address,
// and the very next thing run() does after that is construct a *CLI and call
// Version() on it — an unconditional real `proton-drive --version` spawn.
// Driving run() with an address CanonicalRoot accepts to reach the gitDir
// line would therefore invoke the real CLI binary, which this fix round is
// expressly forbidden from doing (and would also need os.Stdin faked to
// avoid loop() blocking on a read that will never come). resolveGitDir is
// therefore covered directly, by the two tests above, and main.go's call
// site (`gitDir, err := resolveGitDir()`) is a one-line substitution for the
// two-line block it replaced — reviewable by inspection, not something that
// needs an unsafe end-to-end harness to justify. The actual end-to-end proof
// that the fix works lives in internal/repo's
// TestFetchWithARelativeGitDirInstallsCorrectly, which reproduces the exact
// live failure and error text against Fetch directly.

// Finding 2 (bonus coverage, same harness): a poisoned batch's error lines
// must carry a single well-formed ref token, produced by the same
// protocol.ParsePushBatch used on the non-poisoned path — not a hand-rolled
// split on ":" that, for a malformed line, would leak the "push " prefix
// into the ref field of "error <ref> <reason>".
//
// On its own this test cannot fail: its push line HAS a colon, so the old
// hand-rolled `dst := b[strings.Index(b, ":")+1:]` would have produced a
// byte-identical status line. The colonless input where the two parsers
// genuinely diverge is covered by the sibling test below, and the two together
// are what make this real coverage.
func TestLoop_PoisonedBatch_EmitsWellFormedRefToken(t *testing.T) {
	script := "list for-push\n" +
		"option cas true\n" +
		"push refs/heads/main:refs/heads/main\n" +
		"\n"
	in := bufio.NewScanner(strings.NewReader(script))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)
	ft := transport.NewFake()
	// The root must be CANONICAL ("/my-files/..." or "/devices/<id>/..."):
	// Task 11 wired repo.EnsureParents into "list for-push" ahead of
	// Bootstrap, and EnsureParents Stats the MOUNT directly, trusting the
	// same canonical-root invariant repo.CanonicalRoot enforces in
	// production before loop() is ever reached. "/my-files" itself needs no
	// seeding (transport.Fake's builtin-mount leniency, fake.go); this test
	// is about poisoned-batch status-line formatting, not about parent
	// validation.
	root := "/my-files/root"

	got := loop(ft, root, ".", in, out)
	out.Flush()

	if got != 0 {
		t.Fatalf("loop() = %d, want 0: a poisoned batch is reported to git via status lines, not a fatal exit", got)
	}
	if !strings.Contains(outBuf.String(), "error refs/heads/main unsupported option cas\n") {
		t.Fatalf("stdout = %q, want a well-formed \"error refs/heads/main unsupported option cas\" line", outBuf.String())
	}
	if strings.Contains(outBuf.String(), "error push ") {
		t.Fatalf("stdout = %q, contains the malformed pre-fix ref token (\"push \" prefix leaked into the ref field)", outBuf.String())
	}
	if _, ok := ft.Files[root+"/refs/heads/main"]; ok {
		t.Fatalf("a poisoned batch must not write the ref: found %s/refs/heads/main in the fake transport", root)
	}
}

// TestLoop_PoisonedBatch_ColonlessPushLineFailsClosed is the input on which
// the unified parser and the old hand-rolled one actually diverge, and so the
// test that gives the sibling above its teeth.
//
// With no colon in the push line, the old poison path computed
// dst = b[strings.Index(b, ":")+1:] on an Index of -1, yielding the WHOLE raw
// line — "push refs/heads/main" — and emitted "error push refs/heads/main
// unsupported option cas". git reads that as ref="push" followed by garbage:
// a status line for a ref that does not exist, with the real ref silently
// unreported. protocol.ParsePushBatch instead rejects the batch as malformed
// before the poison check runs, so the session fails closed with no protocol
// output at all — poisoned or not, an unparseable batch was never going to be
// applied.
func TestLoop_PoisonedBatch_ColonlessPushLineFailsClosed(t *testing.T) {
	script := "list for-push\n" +
		"option cas true\n" +
		"push refs/heads/main\n" + // no colon: this is the whole point
		"\n"
	in := bufio.NewScanner(strings.NewReader(script))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)
	ft := transport.NewFake()
	// See the sibling test above: the root must be CANONICAL for Task 11's
	// EnsureParents wiring ahead of Bootstrap.
	root := "/my-files/root"

	var got int
	stderr := captureStderr(t, func() { got = loop(ft, root, ".", in, out) })
	out.Flush()

	if got != 1 {
		t.Fatalf("loop() = %d, want 1: a malformed batch must fail closed even when it is also poisoned", got)
	}
	if strings.Contains(outBuf.String(), "error push ") {
		t.Errorf("stdout = %q, contains the malformed ref token the old parser produced (\"push \" leaked into the ref field)", outBuf.String())
	}
	if strings.Contains(outBuf.String(), "unsupported option") {
		t.Errorf("stdout = %q, want no poison status lines at all for a batch that could not be parsed", outBuf.String())
	}
	if !strings.Contains(stderr, "malformed refspec") {
		t.Errorf("stderr = %q, want the parse failure reported", stderr)
	}
	if _, ok := ft.Files[root+"/refs/heads/main"]; ok {
		t.Error("a malformed batch must not write the ref")
	}
}

// --- Task 6: fetch capability, plain `list`, and the fetch batch ---

// TestLoop_Capabilities_AdvertisesFetchAndCheckConnectivity is RED against the
// pre-Task-6 block ("option\npush\n\n"), which advertises neither. Git only
// recognises a connectivity-ok response from a helper that advertised
// check-connectivity, and clone/fetch depend on fetch being advertised at
// all — so both must be present, not merely accepted as options.
func TestLoop_Capabilities_AdvertisesFetchAndCheckConnectivity(t *testing.T) {
	in := bufio.NewScanner(strings.NewReader("capabilities\n\n"))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)

	got := loop(transport.NewFake(), "/remote/root", ".", in, out)
	out.Flush()

	if got != 0 {
		t.Fatalf("loop() = %d, want 0", got)
	}
	stdout := outBuf.String()
	for _, want := range []string{"option", "push", "fetch", "check-connectivity"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("capabilities = %q, want it to advertise %q", stdout, want)
		}
	}
}

// TestLoop_PlainList_AdvertisesRefsAndHeadReadOnly seeds a marked repo with one
// ref and a HEAD directly in the Fake (no push involved — plain `list` only
// reads), then proves the advertisement is right and that reading it takes no
// lock. RED before Task 6: "list" (with no "for-push" suffix) hits the
// default case and fails closed with no output at all.
func TestLoop_PlainList_AdvertisesRefsAndHeadReadOnly(t *testing.T) {
	ft := transport.NewFake()
	root := "/my-files/r"
	if err := repo.Bootstrap(ft, root); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	sha := "1111111111111111111111111111111111111111"
	if _, err := repo.WriteRef(ft, root, "refs/heads/main", sha, false); err != nil {
		t.Fatalf("WriteRef: %v", err)
	}
	if _, err := repo.WriteHEAD(ft, root, "refs/heads/main"); err != nil {
		t.Fatalf("WriteHEAD: %v", err)
	}

	in := bufio.NewScanner(strings.NewReader("list\n\n"))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)

	got := loop(ft, root, ".", in, out)
	out.Flush()

	if got != 0 {
		t.Fatalf("loop() = %d, want 0: stdout=%q", got, outBuf.String())
	}
	want := sha + " refs/heads/main\n@refs/heads/main HEAD\n\n"
	if outBuf.String() != want {
		t.Fatalf("stdout = %q, want %q", outBuf.String(), want)
	}
	if _, ok := ft.Files[root+"/"+repo.LockName]; ok {
		t.Error("plain list must never take the lock: found .lock in the fake transport")
	}
}

// TestLoop_PlainList_OmitsTheSymrefWhenHeadPointsAtAnAbsentBranch is the
// advertisement half of the dangling-HEAD fix.
//
// Nothing on the delete path knew about remote HEAD when this branch shipped
// it, so an ordinary sequence — push `main` (HEAD is backfilled to it), push
// `dev`, then `git push proton-v2 --delete main` — left the remote
// advertising `@refs/heads/main HEAD` with no `refs/heads/main` in the ref
// list. A clone against that advertisement fetches the objects and then
// checks out nothing, which the design names as a failure. The state is also
// PERMANENT: ensureHEAD returns early because a HEAD exists, and v2 never
// rewrites an existing HEAD.
//
// The delete refusal (repo.pushOne) stops NEW remotes from reaching this
// state; this guard is what makes an ALREADY-broken remote behave as the
// defined headless state instead of advertising a symref that resolves to
// nothing. Both are needed — the second is the only thing that helps a remote
// already in the wild.
//
// RED before the fix: the arm emitted the `@` line whenever ReadHEAD reported
// a HEAD at all, without checking the ref list it had just built, so stdout
// carried "@refs/heads/main HEAD" for a branch that is not advertised.
func TestLoop_PlainList_OmitsTheSymrefWhenHeadPointsAtAnAbsentBranch(t *testing.T) {
	ft := transport.NewFake()
	root := "/my-files/r"
	if err := repo.Bootstrap(ft, root); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	sha := "1111111111111111111111111111111111111111"
	if _, err := repo.WriteRef(ft, root, "refs/heads/dev", sha, false); err != nil {
		t.Fatalf("WriteRef: %v", err)
	}
	// HEAD points at a branch that is NOT in the ref list — exactly what a
	// pre-fix `--delete main` left behind.
	if _, err := repo.WriteHEAD(ft, root, "refs/heads/main"); err != nil {
		t.Fatalf("WriteHEAD: %v", err)
	}

	in := bufio.NewScanner(strings.NewReader("list\n\n"))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)

	got := loop(ft, root, ".", in, out)
	out.Flush()
	stdout := outBuf.String()

	if got != 0 {
		t.Fatalf("loop() = %d, want 0: a dangling HEAD is the defined headless state, not a fatal error: stdout=%q", got, stdout)
	}
	if strings.Contains(stdout, "@") {
		t.Errorf("stdout = %q, must advertise no symref at all: HEAD names refs/heads/main, "+
			"which is not in the ref list, and a clone told to check out an unadvertised "+
			"branch checks out nothing", stdout)
	}
	if want := sha + " refs/heads/dev\n\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

// TestLoop_PlainList_AdvertisesTheSymrefWhenHeadResolves is the companion
// guard: the fix must not silence a LEGITIMATE symref. Without this, deleting
// the `@` line entirely would pass the test above.
func TestLoop_PlainList_AdvertisesTheSymrefWhenHeadResolves(t *testing.T) {
	ft := transport.NewFake()
	root := "/my-files/r"
	if err := repo.Bootstrap(ft, root); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	sha := "1111111111111111111111111111111111111111"
	if _, err := repo.WriteRef(ft, root, "refs/heads/main", sha, false); err != nil {
		t.Fatalf("WriteRef: %v", err)
	}
	if _, err := repo.WriteHEAD(ft, root, "refs/heads/main"); err != nil {
		t.Fatalf("WriteHEAD: %v", err)
	}

	in := bufio.NewScanner(strings.NewReader("list\n\n"))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)

	got := loop(ft, root, ".", in, out)
	out.Flush()

	if got != 0 {
		t.Fatalf("loop() = %d, want 0: stdout=%q", got, outBuf.String())
	}
	if !strings.Contains(outBuf.String(), "@refs/heads/main HEAD\n") {
		t.Errorf("stdout = %q, want the symref line: HEAD resolves to an advertised ref, "+
			"and without it a clone checks out nothing", outBuf.String())
	}
}

// TestLoop_PlainList_UnmarkedRootRefusesWithTheMarkerReason pins M4: the plain
// `list` arm must apply the same marker check the fetch path already applies,
// so `git ls-remote` against a folder that is not one of our repos refuses
// with the named reason rather than succeeding vacuously.
//
// RED before the fix: with a plain Fake (no List error to hide behind),
// ListRefs on an unmarked root returns an empty map, ReadHEAD reports no HEAD,
// and the arm printed a bare terminator and exited 0 — advertising a repo that
// does not exist. Its sibling test above uses listErrTransport and so could
// never catch this: it fails inside ListRefs before the gap is reachable.
func TestLoop_PlainList_UnmarkedRootRefusesWithTheMarkerReason(t *testing.T) {
	ft := transport.NewFake()
	root := "/my-files/not-a-repo"
	if err := ft.EnsureDir(root); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	in := bufio.NewScanner(strings.NewReader("list\n\n"))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)

	var got int
	stderr := captureStderr(t, func() { got = loop(ft, root, ".", in, out) })
	out.Flush()

	if got != 1 {
		t.Fatalf("loop() = %d, want 1: listing a folder with no marker must refuse, not "+
			"advertise an empty repo (stdout=%q)", got, outBuf.String())
	}
	if !strings.Contains(stderr, repo.MarkerName) {
		t.Errorf("stderr = %q, want the refusal to name %s", stderr, repo.MarkerName)
	}
	if !strings.Contains(stderr, "not a git-remote-proton repo") {
		t.Errorf("stderr = %q, want the named reason, not a raw listing error", stderr)
	}
	if _, ok := ft.Files[root+"/"+repo.MarkerName]; ok {
		t.Error("a refused list must never create the marker")
	}
	if outBuf.Len() != 0 {
		t.Errorf("stdout = %q, want no protocol output at all for a refused list", outBuf.String())
	}
}

// listErrTransport makes List fail for the refs namespace, standing in for
// what the real CLI reports when it lists a folder that does not exist. The
// Fake's own List never errors — an untouched path just comes back as an
// empty slice — which would let ListRefs succeed vacuously on a totally
// unmarked root and mask the thing this test needs to prove: that a list
// which genuinely cannot read the remote fails closed rather than silently
// advertising nothing.
//
// Task 8 made ListRefs recurse the whole refs/ tree starting from a single
// top-level call, t.List(root+"/refs") — before that, it listed
// refs/heads and refs/tags directly as two separate calls, which is what
// this originally intercepted. Intercepting only those two suffixes went
// quietly inert once the walk's first (and, on an empty root, only) List
// call became "/refs" itself: it would sail straight through to the real
// Fake and return an empty slice, same as an untouched path always has.
// "/refs" is intercepted now for that reason; refs/heads and refs/tags stay
// intercepted too, in case anything ever lists them directly again.
//
// List(root) itself (what Bootstrap's own emptiness check would call) is
// passed straight through and would still succeed. That is deliberate: it is
// what makes "no marker afterward" a real, discriminating assertion below
// rather than a tautology — an implementation that (wrongly) called
// Bootstrap before ListRefs would still have written the marker here, since
// Bootstrap would have completed before ListRefs' own List call ever fails.
type listErrTransport struct{ *transport.Fake }

func (l listErrTransport) List(p string) ([]transport.Node, error) {
	if strings.HasSuffix(p, "/refs") || strings.HasSuffix(p, "/refs/heads") || strings.HasSuffix(p, "/refs/tags") {
		return nil, fmt.Errorf("no such folder: %s", p)
	}
	return l.Fake.List(p)
}

// TestLoop_PlainList_UnmarkedRootFailsWithoutBootstrapping is the read-only
// half of rule 3: a fetch-side list must never bring a repository into
// existence, even when it cannot read the remote it was pointed at. RED
// before Task 6 for the same reason as the test above (no "list" case yet);
// it stays meaningful after Task 6 because the assertions pin the read-only
// contract, not merely a non-zero exit code.
func TestLoop_PlainList_UnmarkedRootFailsWithoutBootstrapping(t *testing.T) {
	ft := transport.NewFake()
	root := "/my-files/unmarked"
	lt := listErrTransport{ft}

	in := bufio.NewScanner(strings.NewReader("list\n\n"))
	out := bufio.NewWriter(&bytes.Buffer{})

	var got int
	stderr := captureStderr(t, func() { got = loop(lt, root, ".", in, out) })

	if got != 1 {
		t.Fatalf("loop() = %d, want 1: a list that cannot read the remote must fail closed", got)
	}
	if stderr == "" {
		t.Error("a failed list must report why on stderr")
	}
	if _, ok := ft.Files[root+"/"+repo.MarkerName]; ok {
		t.Error("plain list must never bootstrap: found a marker after a failed list")
	}
	if ft.Dirs[root+"/refs"] {
		t.Error("plain list must never bootstrap: found refs/ created after a failed list")
	}
}

// newGitRepoWithCommit creates a real local git repository with one commit.
// Fetch and Push both shell out to real git via internal/gitcmd, so the
// fetch-batch tests below need a genuine local repository on each side — the
// Fake only stands in for the REMOTE half of the transport. Mirrors the
// pattern internal/repo's own tests (newGitRepoForPush) use for the same
// reason; that helper is unexported in a different package, so it cannot be
// reused directly here.
//
// initArgs go to `git init` after its own flags (Task 8 passes
// --ref-format=reftable); the identity is set per repository.
func newGitRepoWithCommit(t *testing.T, initArgs ...string) string {
	t.Helper()
	d := t.TempDir()
	for _, a := range [][]string{
		append([]string{"init", "-qb", "main"}, initArgs...),
		{"config", "user.email", "t@t"}, {"config", "user.name", "t"},
	} {
		if err := exec.Command("git", append([]string{"-C", d}, a...)...).Run(); err != nil {
			t.Fatalf("git %v: %v", a, err)
		}
	}
	if err := os.WriteFile(filepath.Join(d, "a.txt"), []byte("one"), 0o644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	if err := exec.Command("git", "-C", d, "add", ".").Run(); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := exec.Command("git", "-C", d, "commit", "-qm", "c1").Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	return d
}

// emptyGitRepo creates a real local git repository with no commits — a fetch
// DESTINATION that genuinely lacks the source's objects, unlike reusing the
// source repo itself (which would make Fetch's up-to-date short-circuit fire
// immediately, since the object would already be present in that store).
// initArgs go to `git init` as for newGitRepoWithCommit (Task 8 passes --bare).
func emptyGitRepo(t *testing.T, initArgs ...string) string {
	t.Helper()
	d := t.TempDir()
	for _, a := range [][]string{
		append([]string{"init", "-qb", "main"}, initArgs...),
		{"config", "user.email", "t@t"}, {"config", "user.name", "t"},
	} {
		if err := exec.Command("git", append([]string{"-C", d}, a...)...).Run(); err != nil {
			t.Fatalf("git %v: %v", a, err)
		}
	}
	return d
}

func headOf(t *testing.T, d string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", d, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	sha := strings.TrimSpace(string(out))
	if len(sha) != 40 {
		t.Fatalf("rev-parse returned %q", sha)
	}
	return sha
}

// pushViaLoop drives loop() through a real "list for-push" + "push" exchange
// to seed a Fake with a genuine marker, ref, and pack pair built from gitDir's
// real history — the "seed via a real push first" pattern the fetch-batch
// tests below need, since Fetch requires an actual pack on the remote and an
// actual object closure to verify, neither of which a hand-planted Fake
// state could produce without duplicating repo.Push itself.
func pushViaLoop(t *testing.T, ft *transport.Fake, root, gitDir string) {
	t.Helper()
	script := "list for-push\npush refs/heads/main:refs/heads/main\n\n"
	in := bufio.NewScanner(strings.NewReader(script))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)

	got := loop(ft, root, gitDir, in, out)
	out.Flush()

	if got != 0 {
		t.Fatalf("seeding push failed: loop() = %d, stdout=%q", got, outBuf.String())
	}
	if !strings.Contains(outBuf.String(), "ok refs/heads/main") {
		t.Fatalf("seeding push did not report ok: stdout=%q", outBuf.String())
	}
}

// TestLoop_FetchBatch_InstallsAndReportsConnectivity is RED before Task 6:
// "fetch <sha> <name>" hits the default case and fails closed instead of
// installing anything. After Task 6 it pins the whole batch contract: a
// fetch that actually installs objects reports a real "lock <keepPath>" line,
// and connectivity-ok follows it — never precedes it, since emitting
// connectivity-ok is only correct once Fetch's own verification (which IS the
// connectivity check per rule 1) has already succeeded.
func TestLoop_FetchBatch_InstallsAndReportsConnectivity(t *testing.T) {
	src := newGitRepoWithCommit(t)
	sha := headOf(t, src)
	ft := transport.NewFake()
	root := "/my-files/r"
	pushViaLoop(t, ft, root, src)

	dst := emptyGitRepo(t)
	script := "option check-connectivity true\nfetch " + sha + " refs/heads/main\n\n"
	in := bufio.NewScanner(strings.NewReader(script))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)

	got := loop(ft, root, dst, in, out)
	out.Flush()
	stdout := outBuf.String()

	if got != 0 {
		t.Fatalf("loop() = %d, want 0: stdout=%q", got, stdout)
	}
	lockIdx := strings.Index(stdout, "lock ")
	connIdx := strings.Index(stdout, "connectivity-ok")
	if lockIdx == -1 {
		t.Fatalf("stdout = %q, want a \"lock \" line naming the installed .keep", stdout)
	}
	if connIdx == -1 {
		t.Fatalf("stdout = %q, want a connectivity-ok line", stdout)
	}
	if lockIdx > connIdx {
		t.Fatalf("stdout = %q, want the lock line before connectivity-ok", stdout)
	}
	if !strings.HasSuffix(stdout, "connectivity-ok\n\n") {
		t.Fatalf("stdout = %q, want it to end with connectivity-ok immediately followed by "+
			"the terminating blank line", stdout)
	}

	var lockLine string
	for _, l := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(l, "lock ") {
			lockLine = l
			break
		}
	}
	keep := strings.TrimPrefix(lockLine, "lock ")
	if _, err := os.Stat(keep); err != nil {
		t.Errorf(".keep at %q reported by the lock line must exist on disk: %v", keep, err)
	}
}

// TestLoop_FetchBatch_UpToDateEmitsNoLock covers rule 5: an up-to-date fetch
// (("", nil) from repo.Fetch) is a legitimate outcome, not an error, and must
// emit no "lock" line at all — while still answering connectivity-ok, because
// Fetch returning a nil error is what makes closure verification true
// regardless of whether anything new had to be installed. RED before Task 6
// for the same reason as the sibling test above.
func TestLoop_FetchBatch_UpToDateEmitsNoLock(t *testing.T) {
	src := newGitRepoWithCommit(t)
	sha := headOf(t, src)
	ft := transport.NewFake()
	root := "/my-files/r"
	pushViaLoop(t, ft, root, src)

	dst := emptyGitRepo(t)
	script := "option check-connectivity true\nfetch " + sha + " refs/heads/main\n\n"

	in1 := bufio.NewScanner(strings.NewReader(script))
	var out1Buf bytes.Buffer
	out1 := bufio.NewWriter(&out1Buf)
	got1 := loop(ft, root, dst, in1, out1)
	out1.Flush()
	if got1 != 0 {
		t.Fatalf("priming fetch: loop() = %d, stdout=%q", got1, out1Buf.String())
	}
	if !strings.Contains(out1Buf.String(), "lock ") {
		t.Fatalf("priming fetch must install and report a lock: stdout=%q", out1Buf.String())
	}

	in2 := bufio.NewScanner(strings.NewReader(script))
	var out2Buf bytes.Buffer
	out2 := bufio.NewWriter(&out2Buf)
	got2 := loop(ft, root, dst, in2, out2)
	out2.Flush()
	stdout := out2Buf.String()

	if got2 != 0 {
		t.Fatalf("loop() = %d, want 0: stdout=%q", got2, stdout)
	}
	if strings.Contains(stdout, "lock ") {
		t.Errorf("stdout = %q, an up-to-date fetch must emit no lock line", stdout)
	}
	if !strings.Contains(stdout, "connectivity-ok") {
		t.Errorf("stdout = %q, connectivity-ok must still be reported when the option is on, "+
			"even when the fetch was already up to date", stdout)
	}
}

// --- Fix round 1: the fetch batch parser must fail closed, never producing
// a false connectivity-ok ---
//
// The reviewer's finding: the original batch loop accepted any line with
// two or more whitespace fields as a want (treating field 1 as the sha, with
// no check that the line even started with "fetch "), and silently DROPPED
// any line with fewer than two fields — no error, nothing added to `wants`.
// A batch whose lines are all malformed therefore left `wants` completely
// empty with no error ever reported. repo.Fetch treats an empty wants list
// as ("", nil) — its legitimate "the local store already had everything"
// signal — and when checkConnectivity was on, that signal reached the
// caller as connectivity-ok: a false vouching for a closure that was never
// actually verified, on exactly the path where git skips its OWN check
// because the helper claimed to have done it.

// TestLoop_FetchBatch_MalformedContinuationLineFailsClosed pins the "silently
// dropped" half of the gap. The first line is a genuine, well-formed fetch
// line (so a permissive parser would seed `wants` with one real sha and go
// on to actually fetch); the second line has only one field and is not a
// "fetch ..." line at all. Under the pre-fix parser this line was silently
// ignored, the real want from line 1 still went through, and the fetch
// SUCCEEDED — got 0, with both a lock line and connectivity-ok in stdout —
// even though the batch as sent was malformed. The fix must instead fail the
// whole session on the malformed line, before repo.Fetch is ever called with
// a partial, silently-repaired batch.
//
// RED (pre-fix, run against the code as committed for Task 6): loop()
// returned 0, not 1 — the "got != 1" assertion fired.
func TestLoop_FetchBatch_MalformedContinuationLineFailsClosed(t *testing.T) {
	src := newGitRepoWithCommit(t)
	sha := headOf(t, src)
	ft := transport.NewFake()
	root := "/my-files/r"
	pushViaLoop(t, ft, root, src)

	dst := emptyGitRepo(t)
	script := "option check-connectivity true\n" +
		"fetch " + sha + " refs/heads/main\n" +
		"bogus-line\n" +
		"\n"
	in := bufio.NewScanner(strings.NewReader(script))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)

	var got int
	stderr := captureStderr(t, func() { got = loop(ft, root, dst, in, out) })
	out.Flush()
	stdout := outBuf.String()

	if got != 1 {
		t.Fatalf("loop() = %d, want 1: a fetch batch with a malformed continuation line must "+
			"fail closed, not silently ignore the bad line and fetch anyway (stdout=%q)", got, stdout)
	}
	if stderr == "" {
		t.Error("a malformed fetch batch must report why on stderr")
	}
	if strings.Contains(stdout, "connectivity-ok") {
		t.Errorf("stdout = %q, must NOT contain connectivity-ok: the batch was malformed, so "+
			"nothing about the closure was actually verified", stdout)
	}
	if strings.Contains(stdout, "lock ") {
		t.Errorf("stdout = %q, must NOT contain a lock line: a malformed batch must install nothing", stdout)
	}
}

// TestLoop_FetchBatch_DegenerateFirstLineFailsClosed pins the "empty wants
// reaches Fetch" half of the gap directly: a lone "fetch " line (matching the
// outer switch's bare strings.HasPrefix check, but with no sha and no name
// at all) as the WHOLE batch. Under the pre-fix parser, strings.Fields("fetch
// ") produced a single-element slice, failed the old "len(sp) >= 2" test, and
// was silently dropped — leaving `wants` completely empty with no error.
// root is a genuinely bootstrapped/marked repo (via repo.Bootstrap directly
// on the Fake, no real git needed) so that repo.Fetch's marker check passes
// and its empty-wants short-circuit is what actually gets exercised — this
// is the exact shape that produced a FALSE connectivity-ok pre-fix.
//
// RED (pre-fix): loop() returned 0 and stdout contained "connectivity-ok" —
// both the "got != 1" and the "Contains(stdout, connectivity-ok)" assertions
// fired.
func TestLoop_FetchBatch_DegenerateFirstLineFailsClosed(t *testing.T) {
	ft := transport.NewFake()
	root := "/my-files/r"
	if err := repo.Bootstrap(ft, root); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	script := "option check-connectivity true\nfetch \n\n"
	in := bufio.NewScanner(strings.NewReader(script))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)

	var got int
	stderr := captureStderr(t, func() { got = loop(ft, root, ".", in, out) })
	out.Flush()
	stdout := outBuf.String()

	if got != 1 {
		t.Fatalf("loop() = %d, want 1: a degenerate \"fetch \" line with no sha and no name "+
			"must fail closed, not silently produce an empty want list (stdout=%q)", got, stdout)
	}
	if stderr == "" {
		t.Error("a degenerate fetch line must report why on stderr")
	}
	if strings.Contains(stdout, "connectivity-ok") {
		t.Errorf("stdout = %q, must NOT contain connectivity-ok: an empty want list must never "+
			"reach repo.Fetch, whose up-to-date signal would then falsely vouch for a closure "+
			"nothing verified", stdout)
	}
	if strings.Contains(stdout, "lock ") {
		t.Errorf("stdout = %q, must NOT contain a lock line", stdout)
	}
}

// --- Task 5: utility dispatch (--version, --set-head) ---
//
// RED (Stage 4): dispatchUtility does not exist yet. The dispatch set is
// CLOSED — only exact "--version" / "--set-head" strings at args[1]
// dispatch; anything else, including other "--"-prefixed argv (a remote
// actually named "--upstream"), must reach the protocol path untouched
// (round-1 Gemini finding). Utility stdout is exempt from protocol-only by
// argv disjointness: git never invokes the helper with these argv shapes.

// TestDispatchUtility_Version_PrintsVersionAndCertifiedCLI is test 1: dispatchUtility
// with args[1] == "--version" handles it and writes the version line to stdout.
func TestDispatchUtility_Version_PrintsVersionAndCertifiedCLI(t *testing.T) {
	var stdout, stderr bytes.Buffer
	handled, code := dispatchUtility([]string{"git-remote-proton", "--version"}, &stdout, &stderr)

	if !handled {
		t.Fatal("dispatchUtility must handle --version")
	}
	if code != 0 {
		t.Errorf("code = %d, want 0", code)
	}
	want := fmt.Sprintf("git-remote-proton %s (certified CLI: %s)\n", "dev", transport.CertifiedCLI)
	if stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

// TestDispatchUtility_SetHead_WrongArity_NoCLIConstruction is test 2: a
// "--set-head" invocation with 0 or 1 following args must be handled with a
// usage message on stderr and exit code 1, and must NEVER reach runSetHead —
// which would construct a real *transport.CLI and spawn `proton-drive
// --version`. The load-bearing assertions are the stderr "usage" text (a
// runSetHead failure would instead print "git-remote-proton: ...") and the
// arity check returning before any construction — NOT the empty stdout,
// which stays empty on both paths regardless: EnforceCertified writes only
// to stderr, and Version() returns the child's output rather than printing
// it.
func TestDispatchUtility_SetHead_WrongArity_NoCLIConstruction(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"zero following args", []string{"git-remote-proton", "--set-head"}},
		{"one following arg", []string{"git-remote-proton", "--set-head", "proton::/my-files/r"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			handled, code := dispatchUtility(c.args, &stdout, &stderr)

			if !handled {
				t.Fatal("dispatchUtility must handle a malformed --set-head invocation")
			}
			if code != 1 {
				t.Errorf("code = %d, want 1", code)
			}
			if !strings.Contains(stderr.String(), "usage") {
				t.Errorf("stderr = %q, want a usage message", stderr.String())
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty: an arity error must return before any CLI "+
					"construction or version check", stdout.String())
			}
		})
	}
}

// TestDispatchUtility_OtherArgs_DoesNotHandle is test 3: the closed-set
// guard. Any args[1] outside {"--version", "--set-head"} — including
// another "--"-prefixed string, such as a remote literally named
// "--upstream" — must NOT be handled, so it reaches the protocol path
// unchanged.
func TestDispatchUtility_OtherArgs_DoesNotHandle(t *testing.T) {
	var stdout, stderr bytes.Buffer
	handled, code := dispatchUtility(
		[]string{"git-remote-proton", "--upstream", "proton::/my-files/r"}, &stdout, &stderr)

	if handled {
		t.Fatal("dispatchUtility must not handle an argv[1] outside the closed set")
	}
	if code != 0 {
		t.Errorf("code = %d, want 0 (unused when handled is false)", code)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Errorf("stdout=%q stderr=%q, want both empty: an unhandled call must produce no output",
			stdout.String(), stderr.String())
	}
}

// TestDispatchUtility_NoArgsAtAll_DoesNotHandle is test 4: dispatchUtility
// called with no arguments beyond the program name must report unhandled,
// leaving run()'s own "must be run by git" check (len(os.Args) < 3) to fire.
// This asserts dispatchUtility's own behaviour directly rather than
// run()'s, because run() with too few args goes on to spawn things this
// hermetic test must not trigger.
func TestDispatchUtility_NoArgsAtAll_DoesNotHandle(t *testing.T) {
	var stdout, stderr bytes.Buffer
	handled, _ := dispatchUtility([]string{"git-remote-proton"}, &stdout, &stderr)

	if handled {
		t.Fatal("dispatchUtility must not handle argv with no arguments beyond the program name")
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Errorf("stdout=%q stderr=%q, want both empty", stdout.String(), stderr.String())
	}
}

// RED: pins the dispatchUtility→runSetHead call site — argv routing, argument
// order, exit-code propagation, and WRITER PLUMBING (the stub writes to the
// stdout writer dispatchUtility passed it; the assertion proves that writer
// reaches the callee — it deliberately does NOT pin runSetHead's real message
// text, which no hermetic test can reach without the Task 13 shim; that text
// stays pinned by the live gate). Until Stage 5 this call site was pinned
// only by live gates (hermetic tests stopped at dispatchUtility's arity arm).
func TestDispatchRoutesSetHeadArgsInOrder(t *testing.T) {
	orig := runSetHeadFn
	defer func() { runSetHeadFn = orig }()
	var gotAddr, gotBranch string
	runSetHeadFn = func(addr, branch string, stdout, stderr io.Writer) int {
		gotAddr, gotBranch = addr, branch
		fmt.Fprintln(stdout, "HEAD is now refs/heads/x")
		return 42
	}
	var out, errb bytes.Buffer
	handled, code := dispatchUtility(
		[]string{"git-remote-proton", "--set-head", "proton::/my-files/r/repo", "feature/x"}, &out, &errb)
	if !handled || code != 42 {
		t.Fatalf("handled=%v code=%d, want true/42", handled, code)
	}
	if gotAddr != "proton::/my-files/r/repo" || gotBranch != "feature/x" {
		t.Fatalf("args routed as (%q,%q)", gotAddr, gotBranch)
	}
	if !strings.Contains(out.String(), "HEAD is now") {
		t.Fatalf("stdout not forwarded: %q", out.String())
	}
}

// TestLoop_FetchBatch_CheckConnectivityFalse_NoConnectivityOk is the minor
// fold-in: the negative branch of the option was correct by construction
// (checkConnectivity defaults false, and the "if checkConnectivity" guard
// already existed) but had no direct test. Not a RED — it already passed
// against the code as committed for Task 6, since fix round 1 only tightens
// batch-line parsing and does not touch the option-false path.
func TestLoop_FetchBatch_CheckConnectivityFalse_NoConnectivityOk(t *testing.T) {
	src := newGitRepoWithCommit(t)
	sha := headOf(t, src)
	ft := transport.NewFake()
	root := "/my-files/r"
	pushViaLoop(t, ft, root, src)

	dst := emptyGitRepo(t)
	script := "fetch " + sha + " refs/heads/main\n\n" // no "option check-connectivity" at all
	in := bufio.NewScanner(strings.NewReader(script))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)

	got := loop(ft, root, dst, in, out)
	out.Flush()
	stdout := outBuf.String()

	if got != 0 {
		t.Fatalf("loop() = %d, want 0: stdout=%q", got, stdout)
	}
	if !strings.Contains(stdout, "lock ") {
		t.Errorf("stdout = %q, want a lock line: the fetch itself must still succeed", stdout)
	}
	if strings.Contains(stdout, "connectivity-ok") {
		t.Errorf("stdout = %q, must NOT contain connectivity-ok: the option was never turned on", stdout)
	}
}

// --- Task 2: per-operation policy at the protocol layer ---
//
// Stage 6, design spec component 1 ("The two policies over the one scan"):
// the SAME scan.Skipped set is consumed two different ways depending on
// which command is being served. "list" (fetch/clone/ls-remote, the attended
// direction) is STRICT: a nonempty ContentSkips() fails the whole command
// with one enumerated error, because a well-named ref file whose content
// does not parse could be a damaged real ref, and silently omitting it would
// be a false-success restore. "list for-push" (the unattended cron-backup
// direction) stays TOLERANT: content-skips are skipped with a note, exactly
// as they already were, plus a cost-gated advisory HEAD diagnostic. Name-
// skips (invalid ref NAME, file or folder) never fail either direction — the
// principled line: a name-invalid path can never be a ref git could hold, so
// its absence is never silent loss.

// TestLoop_ListIsStrictOnContentSkipsListForPushTolerant is the per-operation
// core, driving the SAME Fake fixture (good ref + content junk) through both
// commands in sequence (neither one mutates the fixture the other depends
// on: "list" is read-only).
//
// RED before Task 2: "list" advertised the good ref and skipped the junk
// with a note (Task 1's ScanRefs behaviour, wired through unchanged) instead
// of failing closed — the "got1 != 1" assertion below fired, with got1 == 0
// and the good ref present on stdout.
func TestLoop_ListIsStrictOnContentSkipsListForPushTolerant(t *testing.T) {
	ft := transport.NewFake()
	root := "/my-files/r"
	if err := repo.Bootstrap(ft, root); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	sha := "1111111111111111111111111111111111111111"
	if _, err := repo.WriteRef(ft, root, "refs/heads/main", sha, false); err != nil {
		t.Fatalf("WriteRef: %v", err)
	}
	ft.Files[root+"/refs/heads/junk"] = []byte("hello world\n") // 12 bytes, out-of-band content junk

	// "list" (fetch direction): must fail closed with ONE enumerated error
	// block, and NO ref lines on stdout at all — complete-or-loudly-incomplete.
	in1 := bufio.NewScanner(strings.NewReader("list\n\n"))
	var out1Buf bytes.Buffer
	out1 := bufio.NewWriter(&out1Buf)
	var got1 int
	stderr1 := captureStderr(t, func() { got1 = loop(ft, root, ".", in1, out1) })
	out1.Flush()

	if got1 != 1 {
		t.Fatalf("list with content junk present: loop() = %d, want 1: stdout=%q stderr=%q",
			got1, out1Buf.String(), stderr1)
	}
	if out1Buf.String() != "" {
		t.Errorf("list must emit NO ref lines on stdout when it fails, got %q", out1Buf.String())
	}
	if !strings.Contains(stderr1, "refs/heads/junk") {
		t.Errorf("stderr = %q, want the junk path named", stderr1)
	}
	if !strings.Contains(stderr1, "cannot serve a fetch") {
		t.Errorf("stderr = %q, want the enumerated-error preamble", stderr1)
	}
	if !strings.Contains(stderr1, "trash") {
		t.Errorf("stderr = %q, want the trash remedy", stderr1)
	}

	// "list for-push" against the SAME fixture: junk skipped with a note, the
	// good ref still advertised, exit flow normal.
	in2 := bufio.NewScanner(strings.NewReader("list for-push\n\n"))
	var out2Buf bytes.Buffer
	out2 := bufio.NewWriter(&out2Buf)
	var got2 int
	stderr2 := captureStderr(t, func() { got2 = loop(ft, root, ".", in2, out2) })
	out2.Flush()

	if got2 != 0 {
		t.Fatalf("list for-push with content junk present: loop() = %d, want 0: stdout=%q stderr=%q",
			got2, out2Buf.String(), stderr2)
	}
	if !strings.Contains(out2Buf.String(), sha+" refs/heads/main") {
		t.Errorf("stdout = %q, want the good ref advertised", out2Buf.String())
	}
	if strings.Contains(out2Buf.String(), "junk") {
		t.Errorf("stdout = %q, must not advertise the junk path", out2Buf.String())
	}
	if !strings.Contains(stderr2, "refs/heads/junk") {
		t.Errorf("stderr = %q, want the junk path noted (tolerant direction)", stderr2)
	}
}

// TestLoop_ListStrictErrorCarriesDamagedRefHex: the enumerated error must
// carry a recovered Hex forward when the scan found one — a CRLF-terminated
// candidate (42 bytes, in-band) classifies as the noncanonical damaged-ref
// shape, and the strict fetch survey's error is the one place a human is
// guaranteed to read it, since deleting the file without ever seeing the sha
// could destroy the only surviving pointer to remote-only objects.
//
// RED before Task 2: same defect as the previous test — "list" advertised
// instead of failing, so the recovered hex never reached stderr at all.
func TestLoop_ListStrictErrorCarriesDamagedRefHex(t *testing.T) {
	ft := transport.NewFake()
	root := "/my-files/r"
	if err := repo.Bootstrap(ft, root); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	sha := "1111111111111111111111111111111111111111"
	if _, err := repo.WriteRef(ft, root, "refs/heads/main", sha, false); err != nil {
		t.Fatalf("WriteRef: %v", err)
	}
	damagedSha := "2222222222222222222222222222222222222222"
	ft.Files[root+"/refs/heads/damaged"] = []byte(damagedSha + "\r\n") // CRLF terminator, 42 bytes, in-band

	in := bufio.NewScanner(strings.NewReader("list\n\n"))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)

	var got int
	stderr := captureStderr(t, func() { got = loop(ft, root, ".", in, out) })
	out.Flush()

	if got != 1 {
		t.Fatalf("loop() = %d, want 1: stdout=%q stderr=%q", got, outBuf.String(), stderr)
	}
	if !strings.Contains(stderr, damagedSha) {
		t.Errorf("stderr = %q, want the recovered hex %q in the enumerated error", stderr, damagedSha)
	}
	if !strings.Contains(stderr, "refs/heads/damaged") {
		t.Errorf("stderr = %q, want the damaged path named", stderr)
	}
}

// TestLoop_ListTolerantOnNameSkips is a GUARD (round-1 Gemini): it passes
// against UNPATCHED code too — Stage 5's list arm already tolerates
// name-skips — and pins that Task 2's strict check does not overreach onto
// them. Only a well-named file's unparseable CONTENT triggers fetch-side
// strictness; a name-invalid file can never be a ref git could hold, so its
// absence is never silent loss (the "principled line").
func TestLoop_ListTolerantOnNameSkips(t *testing.T) {
	ft := transport.NewFake()
	root := "/my-files/r"
	if err := repo.Bootstrap(ft, root); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	sha := "1111111111111111111111111111111111111111"
	if _, err := repo.WriteRef(ft, root, "refs/heads/main", sha, false); err != nil {
		t.Fatalf("WriteRef: %v", err)
	}
	ft.Files[root+"/refs/heads/.hidden"] = []byte(sha + "\n") // name-invalid leaf, contents never examined

	in := bufio.NewScanner(strings.NewReader("list\n\n"))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)

	var got int
	stderr := captureStderr(t, func() { got = loop(ft, root, ".", in, out) })
	out.Flush()

	if got != 0 {
		t.Fatalf("list with only a name-skip present must succeed: loop() = %d, stdout=%q stderr=%q",
			got, outBuf.String(), stderr)
	}
	if !strings.Contains(outBuf.String(), sha+" refs/heads/main") {
		t.Errorf("stdout = %q, want the good ref advertised", outBuf.String())
	}
	if !strings.Contains(stderr, ".hidden") {
		t.Errorf("stderr = %q, want the name-skipped path noted", stderr)
	}
}

// TestLoop_ListForPushHeadNamingContentSkippedRefNoted is a RED ([Both]
// round-1 blocker — the draft dismissed this spec state): the PUSH survey's
// HEAD note. HEAD names a content-skipped ref directly; the advertisement
// must still complete (others intact), and a stderr note must name HEAD's
// target and why it was skipped. The cost gate (len(scan.Skipped) > 0) is
// armed by the same junk file that makes the ref content-skipped, so this
// also exercises the gated ReadHEAD call.
func TestLoop_ListForPushHeadNamingContentSkippedRefNoted(t *testing.T) {
	ft := transport.NewFake()
	root := "/my-files/r"
	if err := repo.Bootstrap(ft, root); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	sha := "1111111111111111111111111111111111111111"
	if _, err := repo.WriteRef(ft, root, "refs/heads/main", sha, false); err != nil {
		t.Fatalf("WriteRef: %v", err)
	}
	ft.Files[root+"/refs/heads/junk"] = []byte("hello world\n") // content-skip
	if _, err := repo.WriteHEAD(ft, root, "refs/heads/junk"); err != nil {
		t.Fatalf("WriteHEAD: %v", err)
	}

	in := bufio.NewScanner(strings.NewReader("list for-push\n\n"))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)

	var got int
	stderr := captureStderr(t, func() { got = loop(ft, root, ".", in, out) })
	out.Flush()

	if got != 0 {
		t.Fatalf("list for-push must stay tolerant: loop() = %d, stdout=%q stderr=%q",
			got, outBuf.String(), stderr)
	}
	if !strings.Contains(outBuf.String(), sha+" refs/heads/main") {
		t.Errorf("stdout = %q, want the good ref advertised", outBuf.String())
	}
	if !strings.Contains(stderr, "HEAD names refs/heads/junk") {
		t.Errorf("stderr = %q, want a note naming HEAD's skipped target", stderr)
	}
}

// TestLoop_ListHeadNamingNameSkippedRefNotedNotAdvertised: the fetch-survey
// HEAD case reachable here is NAME-skips only — a content-skip beside it
// would already fail the whole "list" command before this code is ever
// reached (the previous tests above). HEAD names a name-skipped ref exactly
// (not a descendant — see the folder/descendant test below): the symref
// line stays absent (existing Stage 5 logic, unchanged), and NEW here, a
// note names why.
func TestLoop_ListHeadNamingNameSkippedRefNotedNotAdvertised(t *testing.T) {
	ft := transport.NewFake()
	root := "/my-files/r"
	if err := repo.Bootstrap(ft, root); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	sha := "1111111111111111111111111111111111111111"
	if _, err := repo.WriteRef(ft, root, "refs/heads/main", sha, false); err != nil {
		t.Fatalf("WriteRef: %v", err)
	}
	ft.Files[root+"/refs/heads/.hidden"] = []byte(sha + "\n") // name-invalid leaf
	if _, err := repo.WriteHEAD(ft, root, "refs/heads/.hidden"); err != nil {
		t.Fatalf("WriteHEAD: %v", err)
	}

	in := bufio.NewScanner(strings.NewReader("list\n\n"))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)

	var got int
	stderr := captureStderr(t, func() { got = loop(ft, root, ".", in, out) })
	out.Flush()
	stdout := outBuf.String()

	if got != 0 {
		t.Fatalf("list with only a name-skip present (even naming HEAD) must succeed: "+
			"loop() = %d, stdout=%q stderr=%q", got, stdout, stderr)
	}
	if strings.Contains(stdout, "@") {
		t.Errorf("stdout = %q, must not advertise a symref to a skipped name", stdout)
	}
	if !strings.Contains(stdout, sha+" refs/heads/main") {
		t.Errorf("stdout = %q, want the good ref advertised", stdout)
	}
	if !strings.Contains(stderr, "HEAD names refs/heads/.hidden") {
		t.Errorf("stderr = %q, want a note naming HEAD's skipped target", stderr)
	}
}

// TestLoop_ListHeadNamingNameSkippedFolderDescendantNotedNotAdvertised is the
// fetch-direction half of the descendant case the brief calls out
// separately from the exact-match test above: ScanRefs never enters an
// invalid folder's subtree, so the ONLY occupancy it can ever record for
// ".hidden/topic" is the folder itself, "refs/heads/.hidden" — HEAD naming a
// path strictly BENEATH that folder must still match via scanSkipMatch's
// prefix rule (SkipInvalidNameFolder only), not be treated as a HEAD naming
// something merely absent.
func TestLoop_ListHeadNamingNameSkippedFolderDescendantNotedNotAdvertised(t *testing.T) {
	ft := transport.NewFake()
	root := "/my-files/r"
	if err := repo.Bootstrap(ft, root); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	sha := "1111111111111111111111111111111111111111"
	if _, err := repo.WriteRef(ft, root, "refs/heads/main", sha, false); err != nil {
		t.Fatalf("WriteRef: %v", err)
	}
	// ".hidden" is a name-invalid FOLDER; the file underneath makes the Fake
	// synthesise it as a directory node, but the walk never lists inside it.
	ft.Files[root+"/refs/heads/.hidden/topic"] = []byte(sha + "\n")
	// HEAD names a DESCENDANT of the skipped folder, never the folder's own
	// path — exactly the case the prefix-match rule exists for.
	if _, err := repo.WriteHEAD(ft, root, "refs/heads/.hidden/topic"); err != nil {
		t.Fatalf("WriteHEAD: %v", err)
	}

	in := bufio.NewScanner(strings.NewReader("list\n\n"))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)

	var got int
	stderr := captureStderr(t, func() { got = loop(ft, root, ".", in, out) })
	out.Flush()
	stdout := outBuf.String()

	if got != 0 {
		t.Fatalf("a name-skipped folder must never fail the fetch survey: loop() = %d, stdout=%q stderr=%q",
			got, stdout, stderr)
	}
	if strings.Contains(stdout, "@") {
		t.Errorf("stdout = %q, must not advertise a symref into an unentered subtree", stdout)
	}
	if !strings.Contains(stdout, sha+" refs/heads/main") {
		t.Errorf("stdout = %q, want the good ref advertised", stdout)
	}
	if !strings.Contains(stderr, "HEAD names refs/heads/.hidden/topic") {
		t.Errorf("stderr = %q, want a note naming HEAD's descendant target", stderr)
	}
}

// TestLoop_ListForPushHeadNamingNameSkippedFolderDescendantNoted is the
// push-direction twin of the descendant test above — both directions share
// scanSkipMatch, and both must be pinned separately since the two call sites
// are two different code paths in loop().
func TestLoop_ListForPushHeadNamingNameSkippedFolderDescendantNoted(t *testing.T) {
	ft := transport.NewFake()
	root := "/my-files/r"
	if err := repo.Bootstrap(ft, root); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	sha := "1111111111111111111111111111111111111111"
	if _, err := repo.WriteRef(ft, root, "refs/heads/main", sha, false); err != nil {
		t.Fatalf("WriteRef: %v", err)
	}
	ft.Files[root+"/refs/heads/.hidden/topic"] = []byte(sha + "\n")
	if _, err := repo.WriteHEAD(ft, root, "refs/heads/.hidden/topic"); err != nil {
		t.Fatalf("WriteHEAD: %v", err)
	}

	in := bufio.NewScanner(strings.NewReader("list for-push\n\n"))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)

	var got int
	stderr := captureStderr(t, func() { got = loop(ft, root, ".", in, out) })
	out.Flush()

	if got != 0 {
		t.Fatalf("list for-push must stay tolerant: loop() = %d, stdout=%q stderr=%q",
			got, outBuf.String(), stderr)
	}
	if !strings.Contains(outBuf.String(), sha+" refs/heads/main") {
		t.Errorf("stdout = %q, want the good ref advertised", outBuf.String())
	}
	if !strings.Contains(stderr, "HEAD names refs/heads/.hidden/topic") {
		t.Errorf("stderr = %q, want a note naming HEAD's descendant target", stderr)
	}
}

// TestLoop_ListForPushHeadDiagnosticFailureIsAdvisory is a GUARD (round-2
// Codex: undefined failure semantics would let a natural fatal-error
// implementation reintroduce the backup-stopping wedge). Corrupt HEAD bytes
// plus a skipped file (arming the cost gate) must produce an advisory stderr
// note and let "list for-push" complete normally — the advertisement is
// intact and the exit flow is unchanged, because a diagnostic read failure
// must never fail a push.
func TestLoop_ListForPushHeadDiagnosticFailureIsAdvisory(t *testing.T) {
	ft := transport.NewFake()
	root := "/my-files/r"
	if err := repo.Bootstrap(ft, root); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	sha := "1111111111111111111111111111111111111111"
	if _, err := repo.WriteRef(ft, root, "refs/heads/main", sha, false); err != nil {
		t.Fatalf("WriteRef: %v", err)
	}
	ft.Files[root+"/refs/heads/junk"] = []byte("hello world\n")      // arms the cost gate
	ft.Files[root+"/"+repo.HeadName] = []byte("not a symref at all") // corrupt HEAD content

	in := bufio.NewScanner(strings.NewReader("list for-push\n\n"))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)

	var got int
	stderr := captureStderr(t, func() { got = loop(ft, root, ".", in, out) })
	out.Flush()

	if got != 0 {
		t.Fatalf("a corrupt HEAD during skip diagnostics must be advisory only: loop() = %d, stdout=%q stderr=%q",
			got, outBuf.String(), stderr)
	}
	if !strings.Contains(outBuf.String(), sha+" refs/heads/main") {
		t.Errorf("stdout = %q, want the advertisement to complete despite the HEAD read failure", outBuf.String())
	}
	if !strings.Contains(stderr, "HEAD unreadable during skip diagnostics") {
		t.Errorf("stderr = %q, want the advisory note naming the failure", stderr)
	}
}

// --- Task 3: occupancy-aware push wired end to end through loop() ---

// TestLoop_PushCollidesWithScannedOccupancy is RED (round-1 Codex): every
// test in internal/repo manufactures its own `skipped` slice directly, so a
// main.go that kept passing nil to repo.Push would stay green against all of
// them. This test instead drives loop() against a Fake holding a REAL junk
// file the scan itself discovers — proving scan.Skipped is actually wired
// from "push "'s ScanRefs call through to repo.Push, not merely that
// repo.Push's own logic works when handed a slice by hand.
//
// refs/heads/blocked is a genuine content-skip (out-of-band size, exactly
// the "junk file" fixture used throughout Task 2's tests); the push batch
// creates refs/heads/blocked/x, an ANCESTOR collision. The refusal must be
// pre-pack (no pack uploaded) and the junk file must survive untouched.
func TestLoop_PushCollidesWithScannedOccupancy(t *testing.T) {
	ft := transport.NewFake()
	root := "/my-files/r"
	if err := repo.Bootstrap(ft, root); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	ft.Files[root+"/refs/heads/blocked"] = []byte("hello world\n") // real junk file: content-skip

	gitDir := newGitRepoWithCommit(t)

	scan, err := repo.ScanRefs(ft, root)
	if err != nil {
		t.Fatalf("ScanRefs: %v", err)
	}
	var sk repo.SkippedRef
	found := false
	for _, s := range scan.Skipped {
		if s.Path == "refs/heads/blocked" {
			sk, found = s, true
		}
	}
	if !found {
		t.Fatalf("fixture setup: want refs/heads/blocked recorded as a skip, got %+v", scan.Skipped)
	}
	wantMsg := repo.OccupancyMessage(root, sk)

	script := "list for-push\npush refs/heads/main:refs/heads/blocked/x\n\n"
	in := bufio.NewScanner(strings.NewReader(script))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)

	got := loop(ft, root, gitDir, in, out)
	out.Flush()

	if got != 0 {
		t.Fatalf("loop() = %d, want 0: a per-ref refusal must not fail the whole session, stdout=%q",
			got, outBuf.String())
	}
	if !strings.Contains(outBuf.String(), "error refs/heads/blocked/x "+wantMsg) {
		t.Errorf("stdout = %q, want the occupancy refusal naming refs/heads/blocked/x with %q",
			outBuf.String(), wantMsg)
	}
	for p := range ft.Files {
		if strings.HasPrefix(p, root+"/packs/") {
			t.Errorf("no pack may be uploaded for a preflight refusal, found %s", p)
		}
	}
	if got := string(ft.Files[root+"/refs/heads/blocked"]); got != "hello world\n" {
		t.Errorf("the junk file must survive byte-identical, got %q", got)
	}
}

// TestLoop_PushCollidesWithScannedNameSkipOccupancy is RED (round-2 Codex):
// with only the content-junk wiring case above, main.go could pass
// scan.ContentSkips() instead of scan.Skipped and stay green — silently
// dropping name-skip protection, since ContentSkips() only ever returns
// Kind==SkipContent entries. This fixture is instead a scanned INVALID-NAMED
// FILE (refs/heads/blocked/.hidden, Kind SkipInvalidName): only present in
// the full scan.Skipped set, never in ContentSkips(). The push targets
// refs/heads/blocked directly — a DESCENDANT collision — and must still be
// refused pre-pack, kind-aware, with the file surviving untouched.
func TestLoop_PushCollidesWithScannedNameSkipOccupancy(t *testing.T) {
	ft := transport.NewFake()
	root := "/my-files/r"
	if err := repo.Bootstrap(ft, root); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	sha := "1111111111111111111111111111111111111111"
	ft.Files[root+"/refs/heads/blocked/.hidden"] = []byte(sha + "\n") // invalid-named FILE; contents never examined

	gitDir := newGitRepoWithCommit(t)

	scan, err := repo.ScanRefs(ft, root)
	if err != nil {
		t.Fatalf("ScanRefs: %v", err)
	}
	var sk repo.SkippedRef
	found := false
	for _, s := range scan.Skipped {
		if s.Path == "refs/heads/blocked/.hidden" {
			sk, found = s, true
		}
	}
	if !found || sk.Kind != repo.SkipInvalidName {
		t.Fatalf("fixture setup: want refs/heads/blocked/.hidden recorded as SkipInvalidName, got %+v", scan.Skipped)
	}
	if len(scan.ContentSkips()) != 0 {
		t.Fatalf("fixture setup: this occupancy must NOT be a content-skip, got %+v", scan.ContentSkips())
	}
	wantMsg := repo.OccupancyMessage(root, sk)

	script := "list for-push\npush refs/heads/main:refs/heads/blocked\n\n"
	in := bufio.NewScanner(strings.NewReader(script))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)

	got := loop(ft, root, gitDir, in, out)
	out.Flush()

	if got != 0 {
		t.Fatalf("loop() = %d, want 0, stdout=%q", got, outBuf.String())
	}
	if !strings.Contains(outBuf.String(), "error refs/heads/blocked "+wantMsg) {
		t.Errorf("stdout = %q, want the pre-pack kind-aware refusal naming refs/heads/blocked/.hidden: %q",
			outBuf.String(), wantMsg)
	}
	for p := range ft.Files {
		if strings.HasPrefix(p, root+"/packs/") {
			t.Errorf("passing only ContentSkips() would let this reach the pack build; no pack may exist, found %s", p)
		}
	}
	if got := string(ft.Files[root+"/refs/heads/blocked/.hidden"]); got != sha+"\n" {
		t.Errorf("the name-skipped file must survive byte-identical, got %q", got)
	}
}

// TestLoop_RestoreShapeBlockedThenRecovers is RED (restore shape, hermetic —
// round-1 Codex: the spec's clone sequence needs a loop-level pin, not only
// the live gate). Phase 1: "list" with content junk present fails with
// the enumerated error. Phase 2: the junk is removed and a FRESH loop drive
// (a real "list" followed by a real fetch batch, against a genuine git
// repository pushed via pushViaLoop) succeeds and materialises the ref's
// object in a destination repo that never had it.
func TestLoop_RestoreShapeBlockedThenRecovers(t *testing.T) {
	src := newGitRepoWithCommit(t)
	sha := headOf(t, src)
	ft := transport.NewFake()
	root := "/my-files/r"
	pushViaLoop(t, ft, root, src)

	// Phase 1: content junk present -> "list" fails closed, no ref lines.
	ft.Files[root+"/refs/heads/junk"] = []byte("hello world\n")

	in1 := bufio.NewScanner(strings.NewReader("list\n\n"))
	var out1Buf bytes.Buffer
	out1 := bufio.NewWriter(&out1Buf)
	var got1 int
	stderr1 := captureStderr(t, func() { got1 = loop(ft, root, ".", in1, out1) })
	out1.Flush()
	if got1 != 1 {
		t.Fatalf("phase 1: loop() = %d, want 1: stdout=%q stderr=%q", got1, out1Buf.String(), stderr1)
	}
	if out1Buf.String() != "" {
		t.Errorf("phase 1: stdout = %q, want no ref lines while blocked", out1Buf.String())
	}

	// Phase 2: remove the junk; a FRESH loop drive succeeds.
	delete(ft.Files, root+"/refs/heads/junk")

	in2 := bufio.NewScanner(strings.NewReader("list\n\n"))
	var out2Buf bytes.Buffer
	out2 := bufio.NewWriter(&out2Buf)
	got2 := loop(ft, root, ".", in2, out2)
	out2.Flush()
	if got2 != 0 {
		t.Fatalf("phase 2 list: loop() = %d, want 0: stdout=%q", got2, out2Buf.String())
	}
	if !strings.Contains(out2Buf.String(), sha+" refs/heads/main") {
		t.Errorf("phase 2 list: stdout = %q, want the real ref advertised", out2Buf.String())
	}

	dst := emptyGitRepo(t)
	script := "option check-connectivity true\nfetch " + sha + " refs/heads/main\n\n"
	in3 := bufio.NewScanner(strings.NewReader(script))
	var out3Buf bytes.Buffer
	out3 := bufio.NewWriter(&out3Buf)
	got3 := loop(ft, root, dst, in3, out3)
	out3.Flush()
	stdout3 := out3Buf.String()
	if got3 != 0 {
		t.Fatalf("phase 2 fetch: loop() = %d, want 0: stdout=%q", got3, stdout3)
	}
	if !strings.Contains(stdout3, "connectivity-ok") {
		t.Errorf("phase 2 fetch: stdout = %q, want connectivity-ok: the closure must materialise", stdout3)
	}
	if err := exec.Command("git", "-C", dst, "cat-file", "-e", sha).Run(); err != nil {
		t.Errorf("phase 2 fetch: object %s did not materialise in %s: %v", sha, dst, err)
	}
}

// TestLoop_ListDegradedStates is RED: pins both all-skipped shapes named in
// the design spec's "Degraded states" section: all-name-skipped succeeds as
// a clone-of-empty (empty advertisement, notes for every file); all-
// content-skipped fails with EVERY path enumerated in the one error, not
// just the first.
func TestLoop_ListDegradedStates(t *testing.T) {
	t.Run("all name-skipped succeeds with empty advertisement", func(t *testing.T) {
		ft := transport.NewFake()
		root := "/my-files/r"
		if err := repo.Bootstrap(ft, root); err != nil {
			t.Fatalf("bootstrap: %v", err)
		}
		sha := "1111111111111111111111111111111111111111"
		ft.Files[root+"/refs/heads/.hidden"] = []byte(sha + "\n")
		ft.Files[root+"/refs/tags/.also-hidden"] = []byte(sha + "\n")

		in := bufio.NewScanner(strings.NewReader("list\n\n"))
		var outBuf bytes.Buffer
		out := bufio.NewWriter(&outBuf)
		var got int
		stderr := captureStderr(t, func() { got = loop(ft, root, ".", in, out) })
		out.Flush()

		if got != 0 {
			t.Fatalf("all-name-skipped: loop() = %d, want 0 (clone-of-empty): stdout=%q stderr=%q",
				got, outBuf.String(), stderr)
		}
		if outBuf.String() != "\n" {
			t.Errorf("stdout = %q, want an empty advertisement (just the terminator)", outBuf.String())
		}
		if !strings.Contains(stderr, ".hidden") || !strings.Contains(stderr, ".also-hidden") {
			t.Errorf("stderr = %q, want a note per skipped file", stderr)
		}
	})

	t.Run("all content-skipped fails with all paths enumerated", func(t *testing.T) {
		ft := transport.NewFake()
		root := "/my-files/r"
		if err := repo.Bootstrap(ft, root); err != nil {
			t.Fatalf("bootstrap: %v", err)
		}
		ft.Files[root+"/refs/heads/junk1"] = []byte("hello world\n")
		ft.Files[root+"/refs/heads/junk2"] = []byte("goodbye world\n")

		in := bufio.NewScanner(strings.NewReader("list\n\n"))
		var outBuf bytes.Buffer
		out := bufio.NewWriter(&outBuf)
		var got int
		stderr := captureStderr(t, func() { got = loop(ft, root, ".", in, out) })
		out.Flush()

		if got != 1 {
			t.Fatalf("all-content-skipped: loop() = %d, want 1: stdout=%q stderr=%q",
				got, outBuf.String(), stderr)
		}
		if outBuf.String() != "" {
			t.Errorf("stdout = %q, want no ref lines while blocked", outBuf.String())
		}
		if !strings.Contains(stderr, "refs/heads/junk1") || !strings.Contains(stderr, "refs/heads/junk2") {
			t.Errorf("stderr = %q, want BOTH junk paths enumerated", stderr)
		}
	})
}

// --- Task 8 (macOS port, spec section 3, O3(d)): the pair warning on every
// platform, and the Mac refusals at "list" and "list for-push" ---
//
// Every test below drives loopEnv with an explicit helperEnv, so the Mac rules
// run on every CI platform: goos is a field, never runtime.GOOS. The remote is
// a seeded transport.Fake; the local repository is a real one made by
// newGitRepoWithCommit or emptyGitRepo, isolated from the host's git
// configuration (macIsolateGit) so a developer's own fetch.prune cannot change
// an outcome.

// The two spellings of "cafe" with an acute accent. NFC holds the accented e
// as one code point; NFD holds e and then a combining acute accent. The q
// forms are the text %+q prints for each.
const (
	tNFC  = "caf\u00e9"
	tNFD  = "cafe\u0301"
	tNFCq = `caf\u00e9`
	tNFDq = `cafe\u0301`
)

// The texts the brief fixes, as git-remote-proton writes them to stderr. They
// are spelled out here rather than taken from main.go, so a change to either
// side fails a test.
const (
	wantWarnHead = "git-remote-proton: warning: this remote holds refs that differ only by letter " +
		"case or Unicode normalisation; see README, Honest limits:\n"
	wantPairRefusalHead = "git-remote-proton: refusing to fetch on this Mac: this repository uses git's " +
		"default (files) ref store, which cannot keep these refs apart (README, Honest limits):\n"
	wantPairRefusalTail = "\nrepair the remote, or clone with --ref-format=reftable\n"
	wantDestRefusalHead = "git-remote-proton: refusing to fetch on this Mac: refs this fetch would write " +
		"or prune collide with each other or with refs this repository holds (README, Honest limits):\n"
	wantDestRefusalTail = "\ndelete the stale local ref by its exact bytes (git update-ref --stdin), " +
		"change the remote's fetch refspecs, or use a --ref-format=reftable clone\n"
	wantPushRefusalHead = "git-remote-proton: refusing to push on this Mac: git would rewrite tracking " +
		"refs that collide with refs this repository holds, even for refs that are already up to " +
		"date (README, Honest limits):\n"
	wantPushRefusalTail = "\ndelete the stale local ref by its exact bytes (git update-ref --stdin), " +
		"or push from a --ref-format=reftable clone\n"
)

// macOriginDefault is a remote "origin" as `git remote add` writes it.
const macOriginDefault = "[remote \"origin\"]\n\turl = proton::/my-files/r\n" +
	"\tfetch = +refs/heads/*:refs/remotes/origin/*\n"

// toid is a 40-hex object id made of c.
func toid(c byte) string { return strings.Repeat(string(c), 40) }

// macPairRemote is a remote ref set holding one pair: refs/heads/<NFC> at
// oid 1 and refs/heads/<NFD> at oid 2.
func macPairRemote() map[string]string {
	return map[string]string{"refs/heads/" + tNFC: toid('1'), "refs/heads/" + tNFD: toid('2')}
}

// macPairLine is PairLines' line for macPairRemote. NFD sorts first: its
// fourth byte is e (0x65), NFC's is 0xc3, the first byte of the accented e.
func macPairLine() string {
	return "  refs/heads/" + tNFD + " (" + toid('2') + `, "refs/heads/` + tNFDq + `") and refs/heads/` +
		tNFC + " (" + toid('1') + `, "refs/heads/` + tNFCq + `"): by Unicode normalisation`
}

// macIsolateGit keeps every git process the test starts, the helper's own
// local reads included (they inherit this environment), away from the host's
// system and global configuration, and clears GIT_DIR and its relatives so a
// run inside a git hook cannot be pointed at the hook's repository.
func macIsolateGit(t *testing.T) {
	t.Helper()
	global := filepath.Join(t.TempDir(), "global.gitconfig")
	if err := os.WriteFile(global, nil, 0o644); err != nil {
		t.Fatalf("write empty global config: %v", err)
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("LC_ALL", "C")
	for _, k := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE"} {
		t.Setenv(k, "") // registers the restore; the next line is the real change
		os.Unsetenv(k)
	}
}

// macGit runs `git -C dir args...` with stdin, fails the test if it fails, and
// returns its stdout.
func macGit(t *testing.T, dir, stdin string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Stdin = strings.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %q: %v: %s", args, err, stderr.String())
	}
	return string(out)
}

// macRepo is newGitRepoWithCommit, isolated (macIsolateGit), with
// core.precomposeunicode off so that macOS folds no fixture name into NFC on
// its way through argv or readdir. It returns the work tree and the git
// directory, the path git hands the helper as GIT_DIR.
func macRepo(t *testing.T, initArgs ...string) (wt, gitDir string) {
	t.Helper()
	macIsolateGit(t)
	wt = newGitRepoWithCommit(t, initArgs...)
	macGit(t, wt, "", "config", "core.precomposeunicode", "false")
	return wt, filepath.Join(wt, ".git")
}

// macEmptyRepo is emptyGitRepo, isolated as macRepo is: a repository with no
// refs at all, as a clone's is when git sends "list" (M28). A bare one's git
// directory is the repository itself.
func macEmptyRepo(t *testing.T, bare bool) (gitDir string) {
	t.Helper()
	macIsolateGit(t)
	if bare {
		d := emptyGitRepo(t, "--bare")
		macGit(t, d, "", "config", "core.precomposeunicode", "false")
		return d
	}
	d := emptyGitRepo(t)
	macGit(t, d, "", "config", "core.precomposeunicode", "false")
	return filepath.Join(d, ".git")
}

// macPackedRefs creates each named ref at HEAD and packs it before creating
// the next. The names reach git on stdin, never as argv, so macOS cannot
// precompose them; and packing each first lets two spellings that a case- or
// normalisation-insensitive file system aliases coexist, as they do in a Mac
// clone holding a stale packed ref beside a loose one.
func macPackedRefs(t *testing.T, wt string, names ...string) {
	t.Helper()
	oid := headOf(t, wt)
	for _, n := range names {
		macGit(t, wt, "create "+n+" "+oid+"\n", "update-ref", "--stdin")
		macGit(t, wt, "", "pack-refs", "--all")
	}
}

// macConfig appends text to the repository's own config file, as bytes, so no
// NFC or NFD value passes through argv.
func macConfig(t *testing.T, gitDir, text string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(gitDir, "config"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open config: %v", err)
	}
	if _, err := f.WriteString(text); err != nil {
		f.Close()
		t.Fatalf("append config: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close config: %v", err)
	}
}

// macRemote is a marked remote at /my-files/r holding refs (name to oid).
func macRemote(t *testing.T, refs map[string]string) (*transport.Fake, string) {
	t.Helper()
	ft := transport.NewFake()
	root := "/my-files/r"
	if err := repo.Bootstrap(ft, root); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	for name, oid := range refs {
		ft.Files[root+"/"+name] = []byte(oid + "\n")
	}
	return ft, root
}

// macEnv is the helper's environment on a Mac where git set GIT_DIR.
func macEnv(remote string) helperEnv {
	return helperEnv{goos: "darwin", remoteName: remote, gitDirSet: true}
}

// macSession runs script through loopEnv and returns the exit code, stdout and
// stderr. GPB_CREATE_PARENTS is pinned unset so stderr holds only what the
// test is about.
func macSession(t *testing.T, tr transport.Transport, root, gitDir string, env helperEnv,
	script string) (code int, stdout, stderr string) {
	t.Helper()
	t.Setenv(createParentsEnv, "")
	in := bufio.NewScanner(strings.NewReader(script))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)
	stderr = captureStderr(t, func() { code = loopEnv(tr, root, gitDir, env, in, out) })
	out.Flush()
	return code, outBuf.String(), stderr
}

// wantAdvertised fails the test unless stdout advertises every ref in refs.
func wantAdvertised(t *testing.T, stdout string, refs map[string]string) {
	t.Helper()
	for name, oid := range refs {
		if !strings.Contains(stdout, oid+" "+name+"\n") {
			t.Errorf("stdout = %+q, want it to advertise %+q at %s", stdout, name, oid)
		}
	}
}

// stubLocalReads points the three local-read seams at the given functions (nil
// keeps the current one) for one test, and puts them back afterwards.
func stubLocalReads(t *testing.T, format func(string) (string, error),
	cfg func(string, string) (gitcmd.RemoteCfg, error), refs func(string) (map[string]string, error)) {
	t.Helper()
	oldFormat, oldCfg, oldRefs := readRefFormat, readRemoteConfig, readLocalRefs
	if format != nil {
		readRefFormat = format
	}
	if cfg != nil {
		readRemoteConfig = cfg
	}
	if refs != nil {
		readLocalRefs = refs
	}
	t.Cleanup(func() { readRefFormat, readRemoteConfig, readLocalRefs = oldFormat, oldCfg, oldRefs })
}

// lockWatch records whether the session ever took the lock, so a test can tell
// a released lock from one that was never taken.
type lockWatch struct {
	*transport.Fake
	root     string
	acquired bool
}

func (l *lockWatch) CreateExclusive(p, local string) (transport.Outcome, error) {
	out, err := l.Fake.CreateExclusive(p, local)
	if p == l.root+"/"+repo.LockName && err == nil && out == transport.Committed {
		l.acquired = true
	}
	return out, err
}

// wantLockTakenAndReleased fails the test unless the session took the lock and
// left none behind.
func wantLockTakenAndReleased(t *testing.T, lw *lockWatch) {
	t.Helper()
	if !lw.acquired {
		t.Errorf("the session never took the lock; the check must run after AcquireLock")
	}
	if _, ok := lw.Files[lw.root+"/"+repo.LockName]; ok {
		t.Errorf("%s/%s is still present: the deferred release must run on the refusal path", lw.root, repo.LockName)
	}
}

// TestHelperEnvFromReadsGitDirBeforeItIsDefaulted is RED (Task 8, M25).
// Whether git set GIT_DIR is read from the environment itself. The path
// resolveGitDir returns cannot answer it: an unset GIT_DIR comes back as the
// working directory, absolute and never empty. And run() unsets GIT_DIR for
// the children right after, so only a read made before then sees git's
// answer. argv[1] is the remote's name, or the URL itself for an anonymous
// one, exactly as git passes it.
func TestHelperEnvFromReadsGitDirBeforeItIsDefaulted(t *testing.T) {
	args := []string{"git-remote-proton", "origin", "proton::/my-files/r"}
	set := helperEnvFrom(args, func(k string) string {
		if k == "GIT_DIR" {
			return ".git"
		}
		return ""
	}, "darwin")
	if want := (helperEnv{goos: "darwin", remoteName: "origin", gitDirSet: true}); set != want {
		t.Errorf("GIT_DIR=.git: helperEnvFrom = %+v, want %+v", set, want)
	}

	chdirForTest(t, t.TempDir())
	t.Setenv("GIT_DIR", "")
	gitDir, err := resolveGitDir()
	if err != nil || gitDir == "" {
		t.Fatalf("fixture: resolveGitDir() = %q, %v; want the defaulted working directory", gitDir, err)
	}
	unset := helperEnvFrom(args, os.Getenv, "darwin")
	if want := (helperEnv{goos: "darwin", remoteName: "origin"}); unset != want {
		t.Errorf("GIT_DIR unset (resolveGitDir defaults it to %s): helperEnvFrom = %+v, want %+v",
			gitDir, unset, want)
	}

	anon := helperEnvFrom([]string{"git-remote-proton", "proton::/my-files/r", "proton::/my-files/r"},
		func(string) string { return "/w/.git" }, "linux")
	if want := (helperEnv{goos: "linux", remoteName: "proton::/my-files/r", gitDirSet: true}); anon != want {
		t.Errorf("anonymous URL: helperEnvFrom = %+v, want %+v", anon, want)
	}
}

// TestLoop_ListWarnsOnAPairOffAMac is RED (Task 8, spec section 3, "The
// warning"). Off a Mac a remote holding a pair still serves a fetch, and
// "list" says so on stderr exactly once, naming the pair with its relation,
// both object ids and both escape forms. gitDir is not a repository: off a Mac
// the helper makes no local read, and one would fail here.
func TestLoop_ListWarnsOnAPairOffAMac(t *testing.T) {
	for _, c := range []struct {
		name string
		env  helperEnv
	}{
		{"zero env (loop)", helperEnv{}},
		{"linux", helperEnv{goos: "linux", remoteName: "origin", gitDirSet: true}},
		{"windows", helperEnv{goos: "windows", remoteName: "origin", gitDirSet: true}},
	} {
		t.Run(c.name, func(t *testing.T) {
			refs := macPairRemote()
			ft, root := macRemote(t, refs)
			code, stdout, stderr := macSession(t, ft, root, t.TempDir(), c.env, "list\n\n")
			if code != 0 {
				t.Fatalf("loopEnv() = %d, want 0: off a Mac a pair only warns; stderr=%q", code, stderr)
			}
			wantAdvertised(t, stdout, refs)
			if want := wantWarnHead + macPairLine() + "\n"; stderr != want {
				t.Errorf("stderr =\n%q\nwant the warning once:\n%q", stderr, want)
			}
		})
	}
}

// TestLoop_ListForPushWarnsOnAPair is RED (Task 8). "list for-push" warns on
// every platform too, once per session: the push arm scans the same remote a
// second time, and that scan never repeats the warning.
func TestLoop_ListForPushWarnsOnAPair(t *testing.T) {
	wt, _ := macRepo(t)
	refs := macPairRemote()
	ft, root := macRemote(t, refs)
	code, stdout, stderr := macSession(t, ft, root, wt, helperEnv{goos: "linux", remoteName: "origin", gitDirSet: true},
		"list for-push\npush refs/heads/main:refs/heads/main\n\n")
	if code != 0 {
		t.Fatalf("loopEnv() = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
	}
	wantAdvertised(t, stdout, refs)
	if !strings.Contains(stdout, "ok refs/heads/main\n") {
		t.Errorf("stdout = %q, want the push of main to land: a pair still takes pushes to other refs (D2)", stdout)
	}
	if n := strings.Count(stderr, wantWarnHead); n != 1 {
		t.Errorf("the warning header appears %d times in stderr, want 1 (never from the push arm's scan):\n%q", n, stderr)
	}
	if !strings.Contains(stderr, wantWarnHead+macPairLine()+"\n") {
		t.Errorf("stderr = %q, want the warning naming the pair", stderr)
	}
}

// TestLoop_ListRefusesAPairOnAMacFilesRepo is RED (Task 8, rule (1)). On a Mac
// whose repository uses the files ref store, a remote pair would land as two
// tracking refs in one file, so "list" refuses, advertises nothing (git then
// writes no ref), and names the remedies for a pair on the remote.
func TestLoop_ListRefusesAPairOnAMacFilesRepo(t *testing.T) {
	_, gitDir := macRepo(t)
	macConfig(t, gitDir, macOriginDefault)
	ft, root := macRemote(t, macPairRemote())
	code, stdout, stderr := macSession(t, ft, root, gitDir, macEnv("origin"), "list\n\n")
	if code != 1 {
		t.Fatalf("loopEnv() = %d, want 1: stdout=%q stderr=%q", code, stdout, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want no ref line: git writes no ref for a refused list", stdout)
	}
	if want := wantPairRefusalHead + macPairLine() + wantPairRefusalTail; stderr != want {
		t.Errorf("stderr =\n%q\nwant\n%q", stderr, want)
	}
}

// TestLoop_ListOnAMacReftableRepoOnlyWarns is RED (Task 8). A reftable
// repository keeps both spellings apart, so on a Mac it gets the warning only,
// and readLocalState stops at the format: the remote config and local refs are
// never read.
func TestLoop_ListOnAMacReftableRepoOnlyWarns(t *testing.T) {
	_, gitDir := macRepo(t, "--ref-format=reftable")
	macConfig(t, gitDir, macOriginDefault)
	stubLocalReads(t, nil,
		func(string, string) (gitcmd.RemoteCfg, error) {
			t.Errorf("the remote config was read for a reftable repository")
			return gitcmd.RemoteCfg{}, errors.New("unexpected read")
		},
		func(string) (map[string]string, error) {
			t.Errorf("the local refs were read for a reftable repository")
			return nil, errors.New("unexpected read")
		})
	refs := macPairRemote()
	ft, root := macRemote(t, refs)
	code, stdout, stderr := macSession(t, ft, root, gitDir, macEnv("origin"), "list\n\n")
	if code != 0 {
		t.Fatalf("loopEnv() = %d, want 0: a reftable repository only warns; stderr=%q", code, stderr)
	}
	wantAdvertised(t, stdout, refs)
	if want := wantWarnHead + macPairLine() + "\n"; stderr != want {
		t.Errorf("stderr =\n%q\nwant\n%q", stderr, want)
	}
}

// TestLoop_ListOnAMacWithGitDirUnsetOnlyWarns is RED (Task 8, M25). git leaves
// GIT_DIR unset only where no repository encloses the working directory, so
// there are no local refs to protect and "list" only warns. gitDir is a real
// files repository on purpose: a check that ignored gitDirSet would read it and
// refuse.
func TestLoop_ListOnAMacWithGitDirUnsetOnlyWarns(t *testing.T) {
	_, gitDir := macRepo(t)
	macConfig(t, gitDir, macOriginDefault)
	refs := macPairRemote()
	ft, root := macRemote(t, refs)
	code, stdout, stderr := macSession(t, ft, root, gitDir, helperEnv{goos: "darwin", remoteName: "origin"}, "list\n\n")
	if code != 0 {
		t.Fatalf("loopEnv() = %d, want 0: with GIT_DIR unset a pair only warns; stderr=%q", code, stderr)
	}
	wantAdvertised(t, stdout, refs)
	if want := wantWarnHead + macPairLine() + "\n"; stderr != want {
		t.Errorf("stderr =\n%q\nwant\n%q", stderr, want)
	}
}

// TestLoop_ListRefusesTheRenameTransition is RED (Task 8, rule (2), spec gate
// step 5.8's shape). The remote renamed refs/heads/<NFC> to <NFD> in two
// pushes, so it never advertises a pair, but this unpruned clone still holds
// the old packed tracking ref. The fetch would write the NFD tracking ref into
// the NFC one's file.
func TestLoop_ListRefusesTheRenameTransition(t *testing.T) {
	wt, gitDir := macRepo(t)
	macConfig(t, gitDir, macOriginDefault)
	macPackedRefs(t, wt, "refs/remotes/origin/"+tNFC)
	ft, root := macRemote(t, map[string]string{"refs/heads/" + tNFD: toid('2')})
	code, stdout, stderr := macSession(t, ft, root, gitDir, macEnv("origin"), "list\n\n")
	if code != 1 {
		t.Fatalf("loopEnv() = %d, want 1: stdout=%q stderr=%q", code, stdout, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want no ref line", stdout)
	}
	want := wantDestRefusalHead +
		"  refs/remotes/origin/" + tNFD + " (from refs/heads/" + tNFD + ", " + toid('2') +
		") and refs/remotes/origin/" + tNFC + ` (local) ("refs/remotes/origin/` + tNFDq +
		`" vs "refs/remotes/origin/` + tNFCq + `"): by Unicode normalisation` + wantDestRefusalTail
	if stderr != want {
		t.Errorf("stderr =\n%q\nwant\n%q", stderr, want)
	}
}

// TestLoop_ListRefusesTwoAliasingDestinations is RED (Task 8, rule (2),
// Codex). Two remote branches with no pair between them map, through the
// remote's configured refspecs, into directories spelled NFC and NFD, which
// alias on a Mac, though no local ref exists there yet.
func TestLoop_ListRefusesTwoAliasingDestinations(t *testing.T) {
	_, gitDir := macRepo(t)
	macConfig(t, gitDir, "[remote \"r\"]\n\turl = proton::/my-files/r\n"+
		"\tfetch = +refs/heads/x/*:refs/remotes/r/"+tNFC+"/*\n"+
		"\tfetch = +refs/heads/y/*:refs/remotes/r/"+tNFD+"/*\n")
	ft, root := macRemote(t, map[string]string{"refs/heads/x/a": toid('1'), "refs/heads/y/b": toid('2')})
	code, stdout, stderr := macSession(t, ft, root, gitDir, macEnv("r"), "list\n\n")
	if code != 1 {
		t.Fatalf("loopEnv() = %d, want 1: stdout=%q stderr=%q", code, stdout, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want no ref line", stdout)
	}
	want := wantDestRefusalHead +
		"  refs/remotes/r/" + tNFD + "/b (from refs/heads/y/b, " + toid('2') + ") and refs/remotes/r/" + tNFC +
		`/a (also written by this fetch, from refs/heads/x/a) ("refs/remotes/r/` + tNFDq +
		`/b" vs "refs/remotes/r/` + tNFCq + `/a"): by Unicode normalisation` + wantDestRefusalTail
	if stderr != want {
		t.Errorf("stderr =\n%q\nwant\n%q", stderr, want)
	}
}

// TestLoop_ListAnonymousURLChecksTagsOnly is RED (Task 8, R5, PM4). For an
// anonymous URL git passes the URL as argv[1] and no remote is configured, so
// the destinations are the advertised tags alone: a tag aliasing a local one is
// refused, and branches map nowhere. The second case is an empty repository,
// so it also pins that "anonymous" is decided before "a clone with no refspec
// takes the default mapping": under that mapping refs/heads/head would land
// beside the synthesized tracking HEAD and be refused.
func TestLoop_ListAnonymousURLChecksTagsOnly(t *testing.T) {
	const anon = "proton::/my-files/r"
	t.Run("a tag aliasing a local tag is refused", func(t *testing.T) {
		wt, gitDir := macRepo(t)
		macPackedRefs(t, wt, "refs/tags/V1")
		ft, root := macRemote(t, map[string]string{"refs/heads/main": toid('1'), "refs/tags/v1": toid('2')})
		code, stdout, stderr := macSession(t, ft, root, gitDir, macEnv(anon), "list\n\n")
		if code != 1 {
			t.Fatalf("loopEnv() = %d, want 1: stdout=%q stderr=%q", code, stdout, stderr)
		}
		want := wantDestRefusalHead + "  refs/tags/v1 (from refs/tags/v1, " + toid('2') +
			`) and refs/tags/V1 (local) ("refs/tags/v1" vs "refs/tags/V1"): by letter case` + wantDestRefusalTail
		if stderr != want {
			t.Errorf("stderr =\n%q\nwant\n%q", stderr, want)
		}
	})
	t.Run("branches map nowhere", func(t *testing.T) {
		gitDir := macEmptyRepo(t, false)
		refs := map[string]string{"refs/heads/main": toid('1'), "refs/heads/head": toid('2')}
		ft, root := macRemote(t, refs)
		code, stdout, stderr := macSession(t, ft, root, gitDir, macEnv(anon), "list\n\n")
		if code != 0 {
			t.Fatalf("loopEnv() = %d, want 0: an anonymous URL has no tracking refs; stderr=%q", code, stderr)
		}
		wantAdvertised(t, stdout, refs)
		if stderr != "" {
			t.Errorf("stderr = %q, want nothing", stderr)
		}
	})
}

// TestLoop_ListTreatsAHandWrittenColonNamedRemoteAsNamed is RED (Task 8, R5,
// PM3). `git remote add 'a:b'` fails, but a hand-written [remote "a:b"] is
// still looked up by name, so a colon in argv[1] does not make a remote
// anonymous: its refspecs are checked, and the rename-transition state is
// refused.
func TestLoop_ListTreatsAHandWrittenColonNamedRemoteAsNamed(t *testing.T) {
	wt, gitDir := macRepo(t)
	macConfig(t, gitDir, "[remote \"a:b\"]\n\turl = proton::/my-files/r\n\tfetch = +refs/heads/*:refs/remotes/ab/*\n")
	macPackedRefs(t, wt, "refs/remotes/ab/"+tNFC)
	ft, root := macRemote(t, map[string]string{"refs/heads/" + tNFD: toid('2')})
	code, stdout, stderr := macSession(t, ft, root, gitDir, macEnv("a:b"), "list\n\n")
	if code != 1 {
		t.Fatalf("loopEnv() = %d, want 1: stdout=%q stderr=%q", code, stdout, stderr)
	}
	want := wantDestRefusalHead +
		"  refs/remotes/ab/" + tNFD + " (from refs/heads/" + tNFD + ", " + toid('2') +
		") and refs/remotes/ab/" + tNFC + ` (local) ("refs/remotes/ab/` + tNFDq +
		`" vs "refs/remotes/ab/` + tNFCq + `"): by Unicode normalisation` + wantDestRefusalTail
	if stderr != want {
		t.Errorf("stderr =\n%q\nwant\n%q", stderr, want)
	}
}

// TestLoop_ListDuringACloneUsesTheDefaultMapping is RED (Task 8, M28). When git
// sends a clone's "list", the new repository has the remote's url and no fetch
// refspec yet, and holds no refs, so the destinations are git's default
// mapping for that remote's name. A clean remote clones; a branch called head
// lands beside the tracking HEAD the clone writes, under the remote's own name.
func TestLoop_ListDuringACloneUsesTheDefaultMapping(t *testing.T) {
	t.Run("a clean remote clones", func(t *testing.T) {
		gitDir := macEmptyRepo(t, false)
		macConfig(t, gitDir, "[remote \"origin\"]\n\turl = proton::/my-files/r\n")
		refs := map[string]string{"refs/heads/main": toid('1'), "refs/heads/topic": toid('2'), "refs/tags/v1": toid('3')}
		ft, root := macRemote(t, refs)
		code, stdout, stderr := macSession(t, ft, root, gitDir, macEnv("origin"), "list\n\n")
		if code != 0 {
			t.Fatalf("loopEnv() = %d, want 0; stderr=%q", code, stderr)
		}
		wantAdvertised(t, stdout, refs)
		if stderr != "" {
			t.Errorf("stderr = %q, want nothing", stderr)
		}
	})
	t.Run("the mapping uses the remote's name", func(t *testing.T) {
		gitDir := macEmptyRepo(t, false)
		macConfig(t, gitDir, "[remote \"up\"]\n\turl = proton::/my-files/r\n")
		ft, root := macRemote(t, map[string]string{"refs/heads/main": toid('1'), "refs/heads/head": toid('2')})
		code, stdout, stderr := macSession(t, ft, root, gitDir, macEnv("up"), "list\n\n")
		if code != 1 {
			t.Fatalf("loopEnv() = %d, want 1: stdout=%q stderr=%q", code, stdout, stderr)
		}
		want := wantDestRefusalHead + `  refs/remotes/up/HEAD (from HEAD) and refs/remotes/up/head (also written ` +
			`by this fetch, from refs/heads/head) ("refs/remotes/up/HEAD" vs "refs/remotes/up/head"): by letter case` +
			wantDestRefusalTail
		if stderr != want {
			t.Errorf("stderr =\n%q\nwant\n%q", stderr, want)
		}
	})
}

// TestLoop_ListBareAndMirrorClonesOfAPairAreRefused is RED (Task 8, rule (1)).
// A bare clone writes refs/heads/* directly and a mirror clone refs/*, so a
// remote pair lands in one file there as surely as under refs/remotes/; rule
// (1) refuses whatever the mapping.
func TestLoop_ListBareAndMirrorClonesOfAPairAreRefused(t *testing.T) {
	for _, c := range []struct{ name, config string }{
		{"bare", "[remote \"origin\"]\n\turl = proton::/my-files/r\n"},
		{"mirror", "[remote \"origin\"]\n\turl = proton::/my-files/r\n\tfetch = +refs/*:refs/*\n\tmirror = true\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			gitDir := macEmptyRepo(t, true)
			macConfig(t, gitDir, c.config)
			ft, root := macRemote(t, macPairRemote())
			code, stdout, stderr := macSession(t, ft, root, gitDir, macEnv("origin"), "list\n\n")
			if code != 1 {
				t.Fatalf("loopEnv() = %d, want 1: stdout=%q stderr=%q", code, stdout, stderr)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want no ref line", stdout)
			}
			if want := wantPairRefusalHead + macPairLine() + wantPairRefusalTail; stderr != want {
				t.Errorf("stderr =\n%q\nwant\n%q", stderr, want)
			}
		})
	}
}

// TestLoop_ListRefusesAnUnsupportedRefspec is RED (Task 8, R4, PM2). git
// expands an abbreviated destination (refs/heads/main:heads/x wrote
// refs/heads/x), which the helper does not model, so the refspec fails the
// remote-config read closed, naming the value.
func TestLoop_ListRefusesAnUnsupportedRefspec(t *testing.T) {
	_, gitDir := macRepo(t)
	macConfig(t, gitDir, "[remote \"origin\"]\n\turl = proton::/my-files/r\n\tfetch = refs/heads/main:heads/x\n")
	ft, root := macRemote(t, map[string]string{"refs/heads/main": toid('1')})
	code, stdout, stderr := macSession(t, ft, root, gitDir, macEnv("origin"), "list\n\n")
	if code != 1 {
		t.Fatalf("loopEnv() = %d, want 1: stdout=%q stderr=%q", code, stdout, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want no ref line", stdout)
	}
	want := `git-remote-proton: cannot check this Mac repository before fetching: remote config: ` +
		`fetch refspec "refs/heads/main:heads/x": destination "heads/x" does not name a ref below ` +
		`"refs/" (only full ref names are supported; git expands anything else)` + "\n"
	if stderr != want {
		t.Errorf("stderr =\n%q\nwant\n%q", stderr, want)
	}
}

// TestLoop_ListOnAMacFailsClosedOnEachLocalRead is RED (Task 8, spec section
// 3, "Its local reads fail closed"). Each of the three reads, failing, stops
// both "list" and "list for-push", naming the read; the reads run in order and
// stop at the first failure; and a refused "list for-push" still releases the
// lock it took.
func TestLoop_ListOnAMacFailsClosedOnEachLocalRead(t *testing.T) {
	boom := errors.New("injected failure")
	for _, c := range []struct {
		read      string
		wantCalls []string
	}{
		{"ref format", []string{"ref format"}},
		{"remote config", []string{"ref format", "remote config"}},
		{"local refs", []string{"ref format", "remote config", "local refs"}},
	} {
		t.Run(c.read, func(t *testing.T) {
			var calls []string
			stubLocalReads(t,
				func(string) (string, error) {
					calls = append(calls, "ref format")
					if c.read == "ref format" {
						return "", boom
					}
					return "files", nil
				},
				func(string, string) (gitcmd.RemoteCfg, error) {
					calls = append(calls, "remote config")
					if c.read == "remote config" {
						return gitcmd.RemoteCfg{}, boom
					}
					return gitcmd.RemoteCfg{URL: "proton::/my-files/r"}, nil
				},
				func(string) (map[string]string, error) {
					calls = append(calls, "local refs")
					if c.read == "local refs" {
						return nil, boom
					}
					return map[string]string{}, nil
				})
			for _, cmd := range []struct{ script, before string }{
				{"list\n\n", "fetching"},
				{"list for-push\n\n", "pushing"},
			} {
				calls = nil
				ft, root := macRemote(t, map[string]string{"refs/heads/main": toid('1')})
				lw := &lockWatch{Fake: ft, root: root}
				code, stdout, stderr := macSession(t, lw, root, t.TempDir(), macEnv("origin"), cmd.script)
				if code != 1 {
					t.Errorf("%q: loopEnv() = %d, want 1", cmd.script, code)
				}
				if stdout != "" {
					t.Errorf("%q: stdout = %q, want no ref line", cmd.script, stdout)
				}
				want := "git-remote-proton: cannot check this Mac repository before " + cmd.before + ": " +
					c.read + ": injected failure\n"
				if stderr != want {
					t.Errorf("%q: stderr = %q, want %q", cmd.script, stderr, want)
				}
				if strings.Join(calls, ",") != strings.Join(c.wantCalls, ",") {
					t.Errorf("%q: reads ran as %q, want %q", cmd.script, calls, c.wantCalls)
				}
				if cmd.before == "pushing" {
					wantLockTakenAndReleased(t, lw)
				}
			}
		})
	}
}

// TestLoop_ListOffAMacMakesNoLocalReads is a GUARD (Task 8, spec section 3:
// the cost is a few git subprocesses on a darwin list inside a repository,
// "and none anywhere else"). Off darwin, or on darwin with GIT_DIR unset,
// neither list command reads the local repository, even for a remote holding a
// pair.
func TestLoop_ListOffAMacMakesNoLocalReads(t *testing.T) {
	for _, c := range []struct {
		name string
		env  helperEnv
	}{
		{"zero env (loop)", helperEnv{}},
		{"linux", helperEnv{goos: "linux", remoteName: "origin", gitDirSet: true}},
		{"windows", helperEnv{goos: "windows", remoteName: "origin", gitDirSet: true}},
		{"darwin, GIT_DIR unset", helperEnv{goos: "darwin", remoteName: "origin"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			stubLocalReads(t,
				func(string) (string, error) { t.Fatal("the ref format was read"); return "", nil },
				func(string, string) (gitcmd.RemoteCfg, error) {
					t.Fatal("the remote config was read")
					return gitcmd.RemoteCfg{}, nil
				},
				func(string) (map[string]string, error) { t.Fatal("the local refs were read"); return nil, nil })
			for _, script := range []string{"list\n\n", "list for-push\n\n"} {
				ft, root := macRemote(t, macPairRemote())
				if code, _, stderr := macSession(t, ft, root, t.TempDir(), c.env, script); code != 0 {
					t.Errorf("%q: loopEnv() = %d, want 0; stderr=%q", script, code, stderr)
				}
			}
		})
	}
}

// TestLoop_ListForPushRefusesAnAliasingTrackingRefOnAMac is RED (Task 8, R1,
// PM1's shape). The local branch refs/heads/<NFC> has the exact name of the
// advertised ref, so a push by name can find it up to date, and git then
// rewrites its tracking ref refs/remotes/origin/<NFC> even though it sends the
// helper no push command for it: into the file the stale NFD tracking ref
// occupies. The refusal comes after the lock is taken, releases it, and
// advertises nothing.
func TestLoop_ListForPushRefusesAnAliasingTrackingRefOnAMac(t *testing.T) {
	wt, gitDir := macRepo(t)
	macConfig(t, gitDir, macOriginDefault)
	macPackedRefs(t, wt, "refs/heads/"+tNFC, "refs/remotes/origin/"+tNFD)
	ft, root := macRemote(t, map[string]string{"refs/heads/" + tNFC: toid('1')})
	lw := &lockWatch{Fake: ft, root: root}
	code, stdout, stderr := macSession(t, lw, root, gitDir, macEnv("origin"), "list for-push\n\n")
	if code != 1 {
		t.Fatalf("loopEnv() = %d, want 1: stdout=%q stderr=%q", code, stdout, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want no ref advertised", stdout)
	}
	wantLockTakenAndReleased(t, lw)
	want := wantPushRefusalHead +
		"  refs/remotes/origin/" + tNFC + " (from refs/heads/" + tNFC + ", " + toid('1') +
		") and refs/remotes/origin/" + tNFD + ` (local) ("refs/remotes/origin/` + tNFCq +
		`" vs "refs/remotes/origin/` + tNFDq + `"): by Unicode normalisation` + wantPushRefusalTail
	if stderr != want {
		t.Errorf("stderr =\n%q\nwant\n%q", stderr, want)
	}
}

// TestLoop_ListForPushOnAMacAllowsARemotePairWithALocalTrackingRef is RED on
// the warning, and a GUARD on the outcome (Task 8, spec gate step 5.9.1's
// shape; Gemini). The remote holds both spellings; this clone holds the NFC
// branch and its NFC tracking ref only, which conflicts with nothing local, so
// "list for-push" succeeds and leaves the per-ref decision to the push arm, as
// D2's ruling that a pair still takes pushes to its other refs requires.
func TestLoop_ListForPushOnAMacAllowsARemotePairWithALocalTrackingRef(t *testing.T) {
	wt, gitDir := macRepo(t)
	macConfig(t, gitDir, macOriginDefault)
	macPackedRefs(t, wt, "refs/heads/"+tNFC, "refs/remotes/origin/"+tNFC)
	refs := macPairRemote()
	ft, root := macRemote(t, refs)
	lw := &lockWatch{Fake: ft, root: root}
	code, stdout, stderr := macSession(t, lw, root, gitDir, macEnv("origin"), "list for-push\n\n")
	if code != 0 {
		t.Fatalf("loopEnv() = %d, want 0; stderr=%q", code, stderr)
	}
	wantAdvertised(t, stdout, refs)
	wantLockTakenAndReleased(t, lw)
	if want := wantWarnHead + macPairLine() + "\n"; stderr != want {
		t.Errorf("stderr =\n%q\nwant the warning only:\n%q", stderr, want)
	}
}

// TestLoop_ListForPushOnAMacReachesThePushArmForADeleteOfAnUnheldName is a
// GUARD (Task 8, spec gate step 5.9.2's shape). The remote holds the NFD
// branch only; this clone holds the stale NFC tracking ref and no NFD-named
// ref, so no push by name can find anything up to date. "list for-push"
// succeeds, and a delete of the NFD branch reaches the push arm, which answers
// for it.
func TestLoop_ListForPushOnAMacReachesThePushArmForADeleteOfAnUnheldName(t *testing.T) {
	wt, gitDir := macRepo(t)
	macConfig(t, gitDir, macOriginDefault)
	macPackedRefs(t, wt, "refs/remotes/origin/"+tNFC)
	refs := map[string]string{"refs/heads/" + tNFD: toid('2')}
	ft, root := macRemote(t, refs)
	code, stdout, stderr := macSession(t, ft, root, gitDir, macEnv("origin"),
		"list for-push\npush :refs/heads/"+tNFD+"\n\n")
	if code != 0 {
		t.Fatalf("loopEnv() = %d, want 0: stdout=%q stderr=%q", code, stdout, stderr)
	}
	wantAdvertised(t, stdout, refs)
	if !strings.Contains(stdout, "ok refs/heads/"+tNFD+"\n") && !strings.Contains(stdout, "error refs/heads/"+tNFD+" ") {
		t.Errorf("stdout = %q, want a push status line for refs/heads/%s: the delete must reach the push arm", stdout, tNFD)
	}
}

// TestLoop_ListRefusesAnAliasThatAPruneCouldUnlink is RED (Task 8, R6, Codex's
// prune case). The remote no longer holds refs/heads/<NFC>, so a pruning fetch
// may delete its tracking ref, and on a Mac that unlinks the file the NFD
// tracking ref also names. Only a configured prune is visible to the helper:
// with fetch.prune=true it refuses; the same state with pruning unconfigured
// fetches [Kimi].
func TestLoop_ListRefusesAnAliasThatAPruneCouldUnlink(t *testing.T) {
	setup := func(t *testing.T) string {
		wt, gitDir := macRepo(t)
		macConfig(t, gitDir, "[remote \"origin\"]\n\turl = proton::/my-files/r\n"+
			"\tfetch = +refs/heads/main:refs/remotes/origin/main\n"+
			"\tfetch = +refs/heads/"+tNFC+":refs/remotes/origin/"+tNFC+"\n")
		macPackedRefs(t, wt, "refs/remotes/origin/main", "refs/remotes/origin/"+tNFC, "refs/remotes/origin/"+tNFD)
		return gitDir
	}
	refs := map[string]string{"refs/heads/main": toid('1')}
	t.Run("fetch.prune=true refuses", func(t *testing.T) {
		gitDir := setup(t)
		macConfig(t, gitDir, "[fetch]\n\tprune = true\n")
		ft, root := macRemote(t, refs)
		code, stdout, stderr := macSession(t, ft, root, gitDir, macEnv("origin"), "list\n\n")
		if code != 1 {
			t.Fatalf("loopEnv() = %d, want 1: stdout=%q stderr=%q", code, stdout, stderr)
		}
		want := wantDestRefusalHead +
			"  refs/remotes/origin/" + tNFC + " (may be pruned) and refs/remotes/origin/" + tNFD +
			` (local) ("refs/remotes/origin/` + tNFCq + `" vs "refs/remotes/origin/` + tNFDq +
			`"): by Unicode normalisation` + wantDestRefusalTail
		if stderr != want {
			t.Errorf("stderr =\n%q\nwant\n%q", stderr, want)
		}
	})
	t.Run("pruning unconfigured fetches", func(t *testing.T) {
		gitDir := setup(t)
		ft, root := macRemote(t, refs)
		code, stdout, stderr := macSession(t, ft, root, gitDir, macEnv("origin"), "list\n\n")
		if code != 0 {
			t.Fatalf("loopEnv() = %d, want 0; stderr=%q", code, stderr)
		}
		wantAdvertised(t, stdout, refs)
		if stderr != "" {
			t.Errorf("stderr = %q, want nothing", stderr)
		}
	})
}

// TestLoop_ListRefusesAHeadBranchBesideTheTrackingHEAD is RED (Task 8, R6,
// Codex). A clone writes refs/remotes/origin/HEAD, and on default APFS that is
// the file refs/remotes/origin/head names too, so during a clone a remote
// holding main and head is refused. The synthesized HEAD's source has no oid.
func TestLoop_ListRefusesAHeadBranchBesideTheTrackingHEAD(t *testing.T) {
	gitDir := macEmptyRepo(t, false)
	macConfig(t, gitDir, "[remote \"origin\"]\n\turl = proton::/my-files/r\n")
	ft, root := macRemote(t, map[string]string{"refs/heads/main": toid('1'), "refs/heads/head": toid('2')})
	code, stdout, stderr := macSession(t, ft, root, gitDir, macEnv("origin"), "list\n\n")
	if code != 1 {
		t.Fatalf("loopEnv() = %d, want 1: stdout=%q stderr=%q", code, stdout, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want no ref line", stdout)
	}
	want := wantDestRefusalHead + `  refs/remotes/origin/HEAD (from HEAD) and refs/remotes/origin/head (also ` +
		`written by this fetch, from refs/heads/head) ("refs/remotes/origin/HEAD" vs ` +
		`"refs/remotes/origin/head"): by letter case` + wantDestRefusalTail
	if stderr != want {
		t.Errorf("stderr =\n%q\nwant\n%q", stderr, want)
	}
}

// TestLoop_ListFollowsALocalSymbolicTrackingRef is RED (Task 8, R6, Codex).
// git follows a symbolic tracking ref when it updates one, so the tracking HEAD
// this fetch writes, a local symbolic ref to refs/remotes/origin/Topic, makes
// Topic a destination, and Topic aliases the packed refs/remotes/origin/topic.
// Without the symbolic ref the same local pair is one no destination touches,
// and the fetch is not refused.
func TestLoop_ListFollowsALocalSymbolicTrackingRef(t *testing.T) {
	setup := func(t *testing.T, symref bool) string {
		wt, gitDir := macRepo(t)
		macConfig(t, gitDir, macOriginDefault)
		macPackedRefs(t, wt, "refs/remotes/origin/Topic", "refs/remotes/origin/topic")
		if symref {
			macGit(t, wt, "", "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/Topic")
		}
		return gitDir
	}
	refs := map[string]string{"refs/heads/main": toid('1')}
	t.Run("through the tracking HEAD", func(t *testing.T) {
		gitDir := setup(t, true)
		ft, root := macRemote(t, refs)
		code, stdout, stderr := macSession(t, ft, root, gitDir, macEnv("origin"), "list\n\n")
		if code != 1 {
			t.Fatalf("loopEnv() = %d, want 1: stdout=%q stderr=%q", code, stdout, stderr)
		}
		want := wantDestRefusalHead + `  refs/remotes/origin/Topic (from HEAD) and refs/remotes/origin/topic ` +
			`(local) ("refs/remotes/origin/Topic" vs "refs/remotes/origin/topic"): by letter case` +
			wantDestRefusalTail
		if stderr != want {
			t.Errorf("stderr =\n%q\nwant\n%q", stderr, want)
		}
	})
	t.Run("a local pair no destination touches", func(t *testing.T) {
		gitDir := setup(t, false)
		ft, root := macRemote(t, refs)
		code, stdout, stderr := macSession(t, ft, root, gitDir, macEnv("origin"), "list\n\n")
		if code != 0 {
			t.Fatalf("loopEnv() = %d, want 0; stderr=%q", code, stderr)
		}
		wantAdvertised(t, stdout, refs)
		if stderr != "" {
			t.Errorf("stderr = %q, want nothing", stderr)
		}
	})
}

// TestLoop_ListTreatsAPushOnlyRemoteAsNamed is RED (Task 8, R5; Codex). A
// remote with pushurl and fetch set and no url is not anonymous: git uses its
// fetch mapping for tracking refs. So its refspecs are checked at both list
// commands, and the rename-transition and PM1 states are refused.
func TestLoop_ListTreatsAPushOnlyRemoteAsNamed(t *testing.T) {
	const cfg = "[remote \"origin\"]\n\tpushurl = proton::/my-files/r\n\tfetch = +refs/heads/*:refs/remotes/origin/*\n"
	t.Run("list", func(t *testing.T) {
		wt, gitDir := macRepo(t)
		macConfig(t, gitDir, cfg)
		macPackedRefs(t, wt, "refs/remotes/origin/"+tNFC)
		ft, root := macRemote(t, map[string]string{"refs/heads/" + tNFD: toid('2')})
		code, stdout, stderr := macSession(t, ft, root, gitDir, macEnv("origin"), "list\n\n")
		if code != 1 {
			t.Fatalf("loopEnv() = %d, want 1: stdout=%q stderr=%q", code, stdout, stderr)
		}
		want := wantDestRefusalHead +
			"  refs/remotes/origin/" + tNFD + " (from refs/heads/" + tNFD + ", " + toid('2') +
			") and refs/remotes/origin/" + tNFC + ` (local) ("refs/remotes/origin/` + tNFDq +
			`" vs "refs/remotes/origin/` + tNFCq + `"): by Unicode normalisation` + wantDestRefusalTail
		if stderr != want {
			t.Errorf("stderr =\n%q\nwant\n%q", stderr, want)
		}
	})
	t.Run("list for-push", func(t *testing.T) {
		wt, gitDir := macRepo(t)
		macConfig(t, gitDir, cfg)
		macPackedRefs(t, wt, "refs/heads/"+tNFC, "refs/remotes/origin/"+tNFD)
		ft, root := macRemote(t, map[string]string{"refs/heads/" + tNFC: toid('1')})
		lw := &lockWatch{Fake: ft, root: root}
		code, stdout, stderr := macSession(t, lw, root, gitDir, macEnv("origin"), "list for-push\n\n")
		if code != 1 {
			t.Fatalf("loopEnv() = %d, want 1: stdout=%q stderr=%q", code, stdout, stderr)
		}
		wantLockTakenAndReleased(t, lw)
		want := wantPushRefusalHead +
			"  refs/remotes/origin/" + tNFC + " (from refs/heads/" + tNFC + ", " + toid('1') +
			") and refs/remotes/origin/" + tNFD + ` (local) ("refs/remotes/origin/` + tNFCq +
			`" vs "refs/remotes/origin/` + tNFDq + `"): by Unicode normalisation` + wantPushRefusalTail
		if stderr != want {
			t.Errorf("stderr =\n%q\nwant\n%q", stderr, want)
		}
	})
}

// TestLoop_ListWithNoRefspecInAPopulatedRepoChecksTagsOnly is RED (Task 8, R6,
// PM9). A named remote with a url and no fetch refspec, in a repository that
// already holds refs, is not a clone: git fetch writes no tracking ref there,
// so branches are not checked (a stale NFC tracking ref and a head branch both
// pass), but tags still are.
func TestLoop_ListWithNoRefspecInAPopulatedRepoChecksTagsOnly(t *testing.T) {
	const cfg = "[remote \"origin\"]\n\turl = proton::/my-files/r\n"
	t.Run("branches write nothing", func(t *testing.T) {
		wt, gitDir := macRepo(t)
		macConfig(t, gitDir, cfg)
		macPackedRefs(t, wt, "refs/remotes/origin/"+tNFC)
		refs := map[string]string{"refs/heads/main": toid('1'), "refs/heads/head": toid('2'), "refs/heads/" + tNFD: toid('3')}
		ft, root := macRemote(t, refs)
		code, stdout, stderr := macSession(t, ft, root, gitDir, macEnv("origin"), "list\n\n")
		if code != 0 {
			t.Fatalf("loopEnv() = %d, want 0; stderr=%q", code, stderr)
		}
		wantAdvertised(t, stdout, refs)
		if stderr != "" {
			t.Errorf("stderr = %q, want nothing", stderr)
		}
	})
	t.Run("a tag is still checked", func(t *testing.T) {
		wt, gitDir := macRepo(t)
		macConfig(t, gitDir, cfg)
		macPackedRefs(t, wt, "refs/tags/V1")
		ft, root := macRemote(t, map[string]string{"refs/heads/main": toid('1'), "refs/tags/v1": toid('2')})
		code, stdout, stderr := macSession(t, ft, root, gitDir, macEnv("origin"), "list\n\n")
		if code != 1 {
			t.Fatalf("loopEnv() = %d, want 1: stdout=%q stderr=%q", code, stdout, stderr)
		}
		want := wantDestRefusalHead + "  refs/tags/v1 (from refs/tags/v1, " + toid('2') +
			`) and refs/tags/V1 (local) ("refs/tags/v1" vs "refs/tags/V1"): by letter case` + wantDestRefusalTail
		if stderr != want {
			t.Errorf("stderr =\n%q\nwant\n%q", stderr, want)
		}
	})
}
