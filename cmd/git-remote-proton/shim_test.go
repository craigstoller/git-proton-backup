package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/craigstoller/git-proton-backup/internal/gitcmd"
	"github.com/craigstoller/git-proton-backup/internal/testcli"
	"github.com/craigstoller/git-proton-backup/internal/testtrap"
	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// testcliRoleEnv, when set to testcliRoleShim, makes THIS test binary's
// TestMain re-dispatch into testcli.Run instead of running go test's own
// test suite. This is the "os.Executable() re-exec with a role env var"
// technique internal/transport/cli_test.go's TestHelperProcess pattern
// already uses (see its own TestMain), extended here: a COPY of this
// package's own compiled test binary, placed on PATH under the literal name
// "proton-drive"(.exe on Windows), IS the fake proton-drive CLI a real
// *transport.CLI shells out to — argv, the certified-version allowlist
// check, and every c.run call site in internal/transport/cli.go all run
// for real, with no live Proton account involved (Task 13).
const (
	testcliRoleEnv  = "GPB_TESTCLI_ROLE"
	testcliRoleShim = "shim"
)

// TestMain is package main's test entry point. Before ever touching the go
// test framework it makes two early exits. First, it checks whether
// THIS invocation is the hermetic trap (internal/testtrap, rule 12 in
// docs/research/gates/brief-checklist.md): a copy of this binary named
// proton-drive and placed first on PATH, so a test that resolves the CLI
// without a shim of its own hits the trap and never a real, signed-in
// proton-drive further along PATH. That check comes first because the trap
// re-executes this very binary and must exit before anything else runs.
// Second, it checks whether THIS invocation is really a re-exec'd shim
// subprocess (see buildShimOnPath, below): if so, it dispatches straight into
// testcli.Run using this process's own argv and std streams and exits,
// without ever calling m.Run() — the same before-flag-parsing early-exit shape
// cli_test.go's own TestMain uses for its roles, so a shim subprocess run this
// way never touches -test.* flag parsing at all.
//
// Only after both does the trap get installed: the early exits would otherwise
// leak a directory per child process. GPB_LIVE_ACCOUNT=1 is the one run that
// wants the real CLI, so it installs no trap.
func TestMain(m *testing.M) {
	if testtrap.Active() {
		os.Exit(testtrap.Run(os.Stderr))
	}
	if os.Getenv(testcliRoleEnv) == testcliRoleShim {
		os.Exit(testcli.Run(os.Args[1:], os.Stdout, os.Stderr))
	}
	trapDir := ""
	if !liveAccount() {
		dir, err := testtrap.Install()
		if err != nil {
			fmt.Fprintln(os.Stderr, "testtrap.Install:", err)
			os.Exit(2)
		}
		trapDir = dir
	}
	// Every TestLoop_* test that reaches "list for-push" would otherwise spawn
	// a real git for the restore-floor probe (D5(d)). Stub the seam for the whole
	// package; the tests that care about the probe set it themselves and restore
	// it, and TestLoop_ListForPush_RealProbeIsSilentOnTheSuitesGit puts the real
	// function back through realProbeRestoreFloor.
	realProbeRestoreFloor = probeRestoreFloor
	probeRestoreFloor = func(string) (gitcmd.ProbeResult, string) { return gitcmd.ProbeOK, "" }
	code := m.Run()
	// Explicit, because defers do not run on os.Exit. The error is ignored: a
	// Windows file lock held by a lingering child must not fail the run.
	if trapDir != "" {
		os.RemoveAll(trapDir)
	}
	os.Exit(code)
}

// realProbeRestoreFloor holds the production probe, saved by TestMain before
// it stubs the seam, so a test can wire the real thing back in.
var realProbeRestoreFloor func(string) (gitcmd.ProbeResult, string)

// buildShimOnPath copies this test binary into a private temp directory
// under the literal name "proton-drive"(.exe on Windows only), points
// GPB_TESTCLI_TREE at a second, empty temp directory (the shim's local
// tree — every Proton Drive remote path the test seeds, or the real code
// under test writes, maps onto a file under it via testcli.LocalPath), and
// prepends that directory to PATH: the mechanism transport.NewCLI("")'s
// default exe name ("proton-drive") resolves through when *transport.CLI
// shells out. It then asserts that a PATH lookup of "proton-drive" resolves to
// that copy, and fails with both paths if it does not.
//
// Copying (rather than pointing PATH at os.Executable() directly) is not
// optional: PATH lookup matches on the *basename* transport.NewCLI expects
// ("proton-drive"), and os.Executable() here is this package's own test
// binary, named nothing like that. On Windows the copy additionally needs
// the ".exe" suffix — LookPath's PATHEXT-driven search only finds it under
// that literal name. Everywhere else it must carry no suffix at all, since
// LookPath there matches the bare name exactly. The suffix is therefore
// keyed on runtime.GOOS, never borrowed from os.Executable(): on Linux this
// test binary is git-remote-proton.test, whose ".test" extension would name
// the copy "proton-drive.test" and leave it unfindable (the first Linux run,
// 2026-10-15, failed all three shim tests exactly that way).
func buildShimOnPath(t *testing.T) (treeDir string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	shimDir := t.TempDir()
	shimName := "proton-drive"
	if runtime.GOOS == "windows" {
		shimName += ".exe"
	}
	shimExe := filepath.Join(shimDir, shimName)
	in, err := os.Open(self)
	if err != nil {
		t.Fatalf("open self: %v", err)
	}
	defer in.Close()
	out, err := os.OpenFile(shimExe, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatalf("create shim copy: %v", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		t.Fatalf("copy shim binary: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close shim copy: %v", err)
	}

	treeDir = t.TempDir()
	t.Setenv(testcliRoleEnv, testcliRoleShim)
	t.Setenv(testcli.TreeEnv, treeDir)
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	// The F4 failure caught at its source (O6(c)): if the shim is not what a
	// PATH lookup finds, a test would shell out to some other proton-drive,
	// possibly a real signed-in one. os.SameFile, not a string compare, because
	// macOS spells a temp dir both /var/... and /private/var/....
	resolved, err := exec.LookPath("proton-drive")
	if err != nil {
		t.Fatalf("exec.LookPath(proton-drive) after placing the shim at %s: %v", shimExe, err)
	}
	if !samePath(t, resolved, shimExe) {
		t.Fatalf("exec.LookPath(proton-drive) = %s, want the shim at %s: the shim is not what PATH resolves, "+
			"so a hermetic test would reach another proton-drive", resolved, shimExe)
	}
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

// --- Hermetic guards: the shim wins, and no shim means the trap (rule 12) ---

// liveAccount reports whether this run is the live gate (GPB_LIVE_ACCOUNT=1),
// the one case where TestMain installs no trap and the real CLI is wanted.
func liveAccount() bool { return os.Getenv("GPB_LIVE_ACCOUNT") == "1" }

// samePath reports whether two paths name the same file or directory, by
// os.SameFile on their Stats: macOS spells one temp dir both /var/... and
// /private/var/..., so a string comparison would call equal paths different.
func samePath(t *testing.T, a, b string) bool {
	t.Helper()
	ai, err := os.Stat(a)
	if err != nil {
		t.Fatalf("stat %s: %v", a, err)
	}
	bi, err := os.Stat(b)
	if err != nil {
		t.Fatalf("stat %s: %v", b, err)
	}
	return os.SameFile(ai, bi)
}

// TestShimResolvesBeforeTheTrap pins the PATH order a shim test depends on:
// after buildShimOnPath the lookup finds the shim's directory, and the trap
// directory is still on PATH, behind it. Without the second half a missing
// trap would pass this test for the wrong reason.
func TestShimResolvesBeforeTheTrap(t *testing.T) {
	if liveAccount() {
		t.Skip("GPB_LIVE_ACCOUNT=1: no trap is installed on a live run")
	}
	buildShimOnPath(t)

	trapDir := os.Getenv(testtrap.DirEnv)
	if trapDir == "" {
		t.Fatalf("%s is unset: TestMain did not install the trap", testtrap.DirEnv)
	}
	dirs := filepath.SplitList(os.Getenv("PATH"))
	if len(dirs) < 2 {
		t.Fatalf("PATH = %q, want the shim dir then the trap dir at least", os.Getenv("PATH"))
	}

	found, err := exec.LookPath("proton-drive")
	if err != nil {
		t.Fatalf("exec.LookPath(proton-drive) after buildShimOnPath: %v", err)
	}
	if !samePath(t, filepath.Dir(found), dirs[0]) {
		t.Fatalf("exec.LookPath(proton-drive) = %s, want it inside the shim dir %s (PATH's first entry)", found, dirs[0])
	}
	if samePath(t, filepath.Dir(found), trapDir) {
		t.Fatalf("exec.LookPath(proton-drive) = %s, which is the trap in %s, not the shim", found, trapDir)
	}

	behind := false
	for _, d := range dirs[1:] {
		if samePath(t, d, trapDir) {
			behind = true
			break
		}
	}
	if !behind {
		t.Fatalf("the trap dir %s is not on PATH behind the shim: PATH = %q", trapDir, os.Getenv("PATH"))
	}
}

// TestNoShimMeansTheTrap is the other half: with no shim placed, the code
// that builds transport.NewCLI("") reaches the trap, never a real CLI.
// Version folds the CLI's combined output into its error (cli.go Version), so
// the trap's message is visible in the returned error.
func TestNoShimMeansTheTrap(t *testing.T) {
	if liveAccount() {
		t.Skip("GPB_LIVE_ACCOUNT=1: no trap is installed on a live run")
	}
	v, err := transport.NewCLI("").Version()
	if err == nil {
		t.Fatalf("Version() = %q with no shim placed, want an error from the trap", v)
	}
	if !strings.Contains(err.Error(), testtrap.Message) {
		t.Fatalf("Version() error = %q, want it to contain the trap message %q", err.Error(), testtrap.Message)
	}
}

// TestTrapWinsOverTheShimRoleVariable pins the order in TestMain: the trap
// check runs ahead of the shim dispatch. With the shim role variable set and
// no shim placed, the trap must still be the trap. Were the shim dispatch
// first, the trap would answer as a working fake CLI and Version() would
// succeed against a proton-drive that is not there.
func TestTrapWinsOverTheShimRoleVariable(t *testing.T) {
	if liveAccount() {
		t.Skip("GPB_LIVE_ACCOUNT=1: no trap is installed on a live run")
	}
	t.Setenv(testcliRoleEnv, testcliRoleShim)
	t.Setenv(testcli.TreeEnv, t.TempDir())
	v, err := transport.NewCLI("").Version()
	if err == nil {
		t.Fatalf("Version() = %q: the trap answered as a shim; the Active check must come first", v)
	}
	if !strings.Contains(err.Error(), testtrap.Message) {
		t.Fatalf("Version() error = %q, want it to contain the trap message %q", err.Error(), testtrap.Message)
	}
}
