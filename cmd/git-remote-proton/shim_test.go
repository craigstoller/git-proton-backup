package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/craigstoller/git-proton-backup/internal/testcli"
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

// TestMain is package main's test entry point. Its ONLY job before ever
// touching the go test framework is checking whether THIS invocation is
// really a re-exec'd shim subprocess (see buildShimOnPath, below): if so,
// it dispatches straight into testcli.Run using this process's own argv and
// std streams and exits, without ever calling m.Run() — the same
// before-flag-parsing early-exit shape cli_test.go's own TestMain uses for
// its roles, so a shim subprocess run this way never touches -test.* flag
// parsing at all.
func TestMain(m *testing.M) {
	if os.Getenv(testcliRoleEnv) == testcliRoleShim {
		os.Exit(testcli.Run(os.Args[1:], os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

// buildShimOnPath copies this test binary into a private temp directory
// under the literal name "proton-drive"(+the host's exe extension), points
// GPB_TESTCLI_TREE at a second, empty temp directory (the shim's local
// tree — every Proton Drive remote path the test seeds, or the real code
// under test writes, maps onto a file under it via testcli.LocalPath), and
// prepends that directory to PATH: the mechanism transport.NewCLI("")'s
// default exe name ("proton-drive") resolves through when *transport.CLI
// shells out.
//
// Copying (rather than pointing PATH at os.Executable() directly) is not
// optional: PATH lookup matches on the *basename* transport.NewCLI expects
// ("proton-drive"), and os.Executable() here is this package's own test
// binary, named nothing like that. On Windows the copy additionally needs
// the ".exe" suffix — LookPath's PATHEXT-driven search only finds it under
// that literal name.
func buildShimOnPath(t *testing.T) (treeDir string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	shimDir := t.TempDir()
	shimExe := filepath.Join(shimDir, "proton-drive"+filepath.Ext(self))
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
