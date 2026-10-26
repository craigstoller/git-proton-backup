package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/craigstoller/git-proton-backup/internal/gitcmd"
	"github.com/craigstoller/git-proton-backup/internal/repo"
	"github.com/craigstoller/git-proton-backup/internal/testcli"
	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// A COPY of this package's own compiled test binary runs in two more roles
// besides go test's own suite, and the copy's base name, not a variable, says
// which. This is the os.Executable() re-exec technique
// internal/transport/cli_test.go's TestHelperProcess pattern already uses,
// keyed on the name instead of a role variable because the helper sets no
// cmd.Env on any child (design v6.9, F8): every variable a test gives the
// helper reaches the shim it starts too, so a role variable would be obeyed
// by both.
const (
	// shimExeBase: placed on PATH under this literal name ("proton-drive",
	// plus .exe on Windows), the copy IS the fake proton-drive CLI a real
	// *transport.CLI shells out to — argv, the certified-version allowlist
	// check, and every c.run call site in internal/transport/cli.go all run
	// for real, with no live Proton account involved (Task 13).
	shimExeBase = "proton-drive"
	// helperExeBase: the copy is the real helper, run() and all, driven over
	// its protocol pipes by the signal harness (startHelper,
	// signal_harness_test.go) and sent real signals.
	helperExeBase = "git-remote-proton-testhelper"
)

// The helper role's seams, read from its environment (startHelper sets the
// first; a test passes the others in helperOpts.Env).
const (
	// shrinkEnv=1 shrinks the shutdown timings (helperRole lists them).
	shrinkEnv = "GPB_TEST_SHRINK"
	// holdAfterLoopEnv, when non-empty, holds run() after loop returns (E13).
	holdAfterLoopEnv = "GPB_TEST_HOLD_AFTER_LOOP"
	// unconfirmedStopEnv, when non-empty, makes the stop of the in-flight
	// child report its exit unconfirmed (E19).
	unconfirmedStopEnv = "GPB_TEST_UNCONFIRMED_STOP"
)

// The shutdown timings helperRole sets under shrinkEnv, in seconds rather than
// a minute per case. A release call is bounded at shrunkReleaseCall and killed
// at shrunkReleaseCall - shrunkReleaseDrain (2.8 s), which leaves room for E9's
// and W2's 1.5 s delay rule and stays well under the cap. Named so the signal
// cases can bound and check their waits by the values the helper really runs
// with.
const (
	shrunkCap          = 8 * time.Second
	shrunkGrace        = 500 * time.Millisecond
	shrunkTerm         = 500 * time.Millisecond
	shrunkConfirm      = 2 * time.Second
	shrunkRetryPause   = 200 * time.Millisecond
	shrunkOutcomeWait  = 300 * time.Millisecond
	shrunkSettle       = 300 * time.Millisecond
	shrunkReleaseCall  = 3 * time.Second
	shrunkReleaseDrain = 200 * time.Millisecond
)

// afterLoopReady is the file the held run() writes in the shim's ready dir
// (testcli.ReadyDirEnv) once loop has returned; afterLoopGo, created there by
// the test, lets run() go on; holdLimit bounds the hold.
const (
	afterLoopReady = "ready-after-loop"
	afterLoopGo    = "after-loop.go"
	holdLimit      = 30 * time.Second
)

// TestMain is package main's test entry point. Its ONLY job before ever
// touching the go test framework, or any variable, is checking which binary
// THIS invocation is (testBinaryRole): a re-exec'd shim (see buildShimOnPath,
// below) dispatches straight into testcli.Run using this process's own argv
// and std streams, and a re-exec'd helper into helperRole, and either exits
// without ever calling m.Run() — the same before-flag-parsing early-exit
// shape cli_test.go's own TestMain uses for its roles, so a subprocess run
// this way never touches -test.* flag parsing at all. Anything else is the
// test runner.
func TestMain(m *testing.M) {
	switch testBinaryRole() {
	case shimExeBase:
		os.Exit(testcli.Run(os.Args[1:], os.Stdout, os.Stderr))
	case helperExeBase:
		os.Exit(helperRole())
	}
	// Every TestLoop_* test that reaches "list for-push" would otherwise spawn
	// a real git for the restore-floor probe (D5(d)). Stub the seam for the whole
	// package; the tests that care about the probe set it themselves and restore
	// it, and TestLoop_ListForPush_RealProbeIsSilentOnTheSuitesGit puts the real
	// function back through realProbeRestoreFloor.
	realProbeRestoreFloor = probeRestoreFloor
	probeRestoreFloor = func(string) (gitcmd.ProbeResult, string) { return gitcmd.ProbeOK, "" }
	// An in-doubt lock handle settles (5 s in production) before its release
	// reads the remote. Nothing here has a late request to wait out, so the
	// wait is only time spent.
	repo.SettleDelay = 0
	os.Exit(m.Run())
}

// realProbeRestoreFloor holds the production probe, saved by TestMain before
// it stubs the seam, so a test can wire the real thing back in.
var realProbeRestoreFloor func(string) (gitcmd.ProbeResult, string)

// testBinaryRole is this executable's base name with any ".exe" stripped: what
// TestMain dispatches on. os.Executable, not os.Args[0], so the answer is the
// file actually running, whatever argv[0] its parent gave it.
func testBinaryRole() string {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	base := filepath.Base(exe)
	if ext := filepath.Ext(base); strings.EqualFold(ext, ".exe") {
		base = strings.TrimSuffix(base, ext)
	}
	return base
}

// helperRole is the helper as git would run it, with the seams the signal
// cases need set from its environment; run(), its signal handling, its CLI and
// its git are the real ones. os.Args is the argv startHelper gave it: [exe,
// "origin", <address>] for a push or fetch, or [exe, "--set-head", <address>,
// <branch>].
func helperRole() int {
	// As the m.Run() path does, or every case would run the real probe.
	probeRestoreFloor = func(string) (gitcmd.ProbeResult, string) { return gitcmd.ProbeOK, "" }
	if os.Getenv(shrinkEnv) == "1" {
		shutdownCap = shrunkCap
		childGrace = shrunkGrace
		childTerm = shrunkTerm
		childConfirm = shrunkConfirm
		retryPause = shrunkRetryPause
		outcomeWait = shrunkOutcomeWait
		repo.SettleDelay = shrunkSettle
		transport.SetReleaseBoundsForTest(shrunkReleaseCall, shrunkReleaseDrain)
	}
	if os.Getenv(holdAfterLoopEnv) != "" {
		afterLoopHook = holdAfterLoop
	}
	if os.Getenv(unconfirmedStopEnv) != "" {
		// The child is still stopped, so nothing is left running; only the
		// coordinator is told its exit was not confirmed (E19).
		stopInFlight = func(grace, term, confirm time.Duration) bool {
			transport.StopInFlight(grace, term, confirm)
			return false
		}
	}
	return run()
}

// holdAfterLoop holds run() between loop's return and finish (E13): it writes
// afterLoopReady in the ready dir, then waits until the test creates
// afterLoopGo there, which it does once its signal's shutdown has begun. After
// holdLimit it gives up and lets run() finish.
func holdAfterLoop() {
	dir := os.Getenv(testcli.ReadyDirEnv)
	ready := filepath.Join(dir, afterLoopReady)
	if err := os.WriteFile(ready, nil, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "git-remote-proton-testhelper: cannot write %s: %v\n", ready, err)
		return
	}
	for end := time.Now().Add(holdLimit); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(filepath.Join(dir, afterLoopGo)); err == nil {
			return
		}
	}
}

// copyTestBinary copies this test binary into dir under base, plus ".exe" on
// Windows only, and returns the copy's path.
//
// Copying (rather than pointing at os.Executable() directly) is not optional:
// PATH lookup matches on the *basename* transport.NewCLI expects
// ("proton-drive"), and TestMain dispatches on the base name too, while
// os.Executable() here is this package's own test binary, named nothing like
// either. On Windows the copy additionally needs the ".exe" suffix — LookPath's
// PATHEXT-driven search only finds it under that literal name. Everywhere else
// it must carry no suffix at all, since LookPath there matches the bare name
// exactly. The suffix is therefore keyed on runtime.GOOS, never borrowed from
// os.Executable(): on Linux this test binary is git-remote-proton.test, whose
// ".test" extension would name the copy "proton-drive.test" and leave it
// unfindable (the first Linux run, 2026-10-15, failed all three shim tests
// exactly that way).
func copyTestBinary(t *testing.T, dir, base string) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	if runtime.GOOS == "windows" {
		base += ".exe"
	}
	dst := filepath.Join(dir, base)
	in, err := os.Open(self)
	if err != nil {
		t.Fatalf("open self: %v", err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatalf("create %s: %v", base, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		t.Fatalf("copy the test binary to %s: %v", base, err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close %s: %v", base, err)
	}
	return dst
}

// buildShimOnPath copies this test binary into a private temp directory
// under the literal name "proton-drive"(.exe on Windows only; see
// copyTestBinary), points GPB_TESTCLI_TREE at a second, empty temp directory
// (the shim's local tree — every Proton Drive remote path the test seeds, or
// the real code under test writes, maps onto a file under it via
// testcli.LocalPath), and prepends that directory to PATH: the mechanism
// transport.NewCLI("")'s default exe name ("proton-drive") resolves through
// when *transport.CLI shells out. The copy's name is what makes it the shim
// (TestMain); no role variable is set.
func buildShimOnPath(t *testing.T) (treeDir string) {
	t.Helper()
	shimDir := t.TempDir()
	copyTestBinary(t, shimDir, shimExeBase)

	treeDir = t.TempDir()
	t.Setenv(testcli.TreeEnv, treeDir)
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	// Deterministic regardless of what the ambient environment happens to
	// carry: runSetHead reads this itself (uncertifiedCLIEnv, main.go), and
	// this test's whole point is exercising the DEFAULT allowlist refusal
	// path succeeding against a genuinely certified shim, not the override.
	t.Setenv(uncertifiedCLIEnv, "")
	return treeDir
}

// seedSetHeadTree writes exactly the remote state runSetHead's happy path
// needs directly under the shim's local tree, using testcli.LocalPath so
// the mapping matches what the shim itself reads back:
//
//   - <root>/gpb-remote.json — the marker RequireMarker reads and
//     checkMarker parses (repo.MarkerName; must decode to format
//     "git-remote-proton", version 1 — repo.checkMarker's own required
//     shape, reconstructed here rather than imported since it is
//     unexported in package repo).
//   - <root>/refs/heads/feature/x — a hierarchical branch (Task 10) holding
//     a well-formed 40-hex-plus-newline ref, the exact grammar readRef
//     enforces.
//
// Deliberately absent: <root>/HEAD (so UpdateHEAD's create path, not its
// overwrite path, is what this test exercises) and <root>/.lock (so
// AcquireLock's happy path, not its refusal path, runs).
func seedSetHeadTree(t *testing.T, treeDir, root, branchRef, sha string) {
	t.Helper()
	marker, err := json.Marshal(struct {
		Format  string `json:"format"`
		Version int    `json:"version"`
	}{"git-remote-proton", 1})
	if err != nil {
		t.Fatalf("marshal marker: %v", err)
	}
	markerPath := testcli.LocalPath(treeDir, root+"/gpb-remote.json")
	if err := os.MkdirAll(filepath.Dir(markerPath), 0o755); err != nil {
		t.Fatalf("mkdir marker parent: %v", err)
	}
	if err := os.WriteFile(markerPath, marker, 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	refPath := testcli.LocalPath(treeDir, root+"/"+branchRef)
	if err := os.MkdirAll(filepath.Dir(refPath), 0o755); err != nil {
		t.Fatalf("mkdir ref parent: %v", err)
	}
	if err := os.WriteFile(refPath, []byte(sha+"\n"), 0o644); err != nil {
		t.Fatalf("write ref: %v", err)
	}
}

// TestRunSetHead_EndToEndThroughTheScriptedShim is Task 13's one hermetic
// end-to-end test. It exercises the REAL runSetHead (never runSetHeadFn,
// the dispatchUtility test seam TestDispatchRoutesSetHeadArgsInOrder pins
// separately in main_test.go) against a real *transport.CLI that shells out
// to a real proton-drive(.exe) on PATH — the scripted shim, serving argv
// this process's own runSetHead constructs for real: EnforceCertified's
// "--version" probe, RequireMarker's "filesystem info" + "filesystem
// download" pair, AcquireLock's "filesystem upload -f skip" plus its own
// read-back, the branch's "filesystem info" + "filesystem download" pair,
// ReadHEAD's absent-HEAD "filesystem info", UpdateHEAD's "filesystem
// upload -f skip" write plus its own read-back verification, and finally
// the deferred Lock.Release's "filesystem trash".
//
// Before Task 13, none of this — including runSetHead's own construction of
// transport.NewCLI("") and its unconditional EnforceCertified probe — could
// run in a hermetic test at all; only Task 3's dispatchUtility seam
// (runSetHeadFn) was covered hermetically, and the real runSetHead was
// pinned only by the live gate.
func TestRunSetHead_EndToEndThroughTheScriptedShim(t *testing.T) {
	treeDir := buildShimOnPath(t)
	const (
		root      = "/my-files/r"
		branchRef = "refs/heads/feature/x"
		sha       = "1111111111111111111111111111111111111111"
	)
	seedSetHeadTree(t, treeDir, root, branchRef, sha)

	var stdout, stderr bytes.Buffer
	code := runSetHead("proton::"+root, "feature/x", &stdout, &stderr)

	if code != 0 {
		t.Fatalf("runSetHead() = %d, want 0: stderr=%q", code, stderr.String())
	}
	const want = "HEAD is now refs/heads/feature/x\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q (stderr=%q)", stdout.String(), want, stderr.String())
	}

	headPath := testcli.LocalPath(treeDir, root+"/HEAD")
	got, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatalf("reading the tree's own HEAD file: %v", err)
	}
	if wantBytes := "ref: refs/heads/feature/x\n"; string(got) != wantBytes {
		t.Fatalf("tree HEAD bytes = %q, want %q", string(got), wantBytes)
	}

	// The lock must not have been left behind: Lock.Release runs in
	// runSetHead's own defer on every exit path.
	if _, err := os.Stat(testcli.LocalPath(treeDir, root+"/.lock")); !os.IsNotExist(err) {
		t.Errorf(".lock must not remain after a successful --set-head, got stat err=%v", err)
	}
}

// TestCreateExclusiveSkipStrategyRefusesAnExistingTarget is the "skip-summary
// fixture" the 0.8.0 certification plan calls for (Task 1 step 3): the only
// other hermetic coverage driving the REAL shim end to end
// (TestRunSetHead_EndToEndThroughTheScriptedShim, above) only ever exercises
// CreateExclusive/UpdateHEAD's "-f skip" upload against an ABSENT target
// (transferredItems=1 -> Committed) — the shim's OTHER "skip" summary shape
// (skippedItems=1 -> Refused), which CreateExclusive's Refused detection
// depends on, had no hermetic coverage through the real *transport.CLI
// wrapper (only through *transport.Fake — see fake_test.go's
// TestFakeCreateExclusiveThenRefuse). This drives it through the real
// shim: create once onto an absent target (Committed), create again onto
// the same now-occupied target (Refused).
func TestCreateExclusiveSkipStrategyRefusesAnExistingTarget(t *testing.T) {
	buildShimOnPath(t)
	cli := transport.NewCLI("")

	local := filepath.Join(t.TempDir(), "leaf.txt")
	if err := os.WriteFile(local, []byte("hello"), 0o644); err != nil {
		t.Fatalf("seed local file: %v", err)
	}

	out, err := cli.CreateExclusive("/my-files/r/leaf.txt", local)
	if err != nil {
		t.Fatalf("first CreateExclusive: %v", err)
	}
	if out != transport.Committed {
		t.Fatalf("first CreateExclusive onto an absent target = %v, want Committed", out)
	}

	out, err = cli.CreateExclusive("/my-files/r/leaf.txt", local)
	if err != nil {
		t.Fatalf("second CreateExclusive: %v", err)
	}
	if out != transport.Refused {
		t.Fatalf("second CreateExclusive onto the same target = %v, want Refused", out)
	}
}

// TestUpdateRevisionCreateNewRevisionStrategyUpsertsThroughTheRealShim pins
// the Task 1 step 3 vocabulary rename (upload("merge", ...) ->
// upload("create-new-revision", ...)) end to end through the real shim, not
// just through *transport.Fake (fake_test.go's
// TestFakeUpdateRevisionSkipsIdenticalContent covers the Fake side, which
// deliberately does NOT model the CLI's byte-identical auto-skip — see that
// test's own doc comment). Before this rename, the shim's runUpload switch
// only recognised "merge"; had the shim not been updated alongside cli.go,
// this would fail with "unsupported upload strategy" instead of upserting.
func TestUpdateRevisionCreateNewRevisionStrategyUpsertsThroughTheRealShim(t *testing.T) {
	buildShimOnPath(t)
	cli := transport.NewCLI("")

	local := filepath.Join(t.TempDir(), "leaf.txt")
	if err := os.WriteFile(local, []byte("first"), 0o644); err != nil {
		t.Fatalf("seed local file: %v", err)
	}

	// Absent target: create-new-revision upserts (Stage 1 C8's "UPSERTS"
	// finding — no prior existence check needed for correctness).
	out, err := cli.UpdateRevision("/my-files/r/leaf.txt", local)
	if err != nil {
		t.Fatalf("first UpdateRevision (absent target): %v", err)
	}
	if out != transport.Committed {
		t.Fatalf("first UpdateRevision onto an absent target = %v, want Committed", out)
	}

	// Existing target, different content: create-new-revision revises in
	// place (Stage 1 C1's "REVISES IN PLACE" finding), never a Refused/skip.
	if err := os.WriteFile(local, []byte("second, different content"), 0o644); err != nil {
		t.Fatalf("rewrite local file: %v", err)
	}
	out, err = cli.UpdateRevision("/my-files/r/leaf.txt", local)
	if err != nil {
		t.Fatalf("second UpdateRevision (existing target): %v", err)
	}
	if out != transport.Committed {
		t.Fatalf("second UpdateRevision onto an existing target = %v, want Committed", out)
	}
}
