package testtrap_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/craigstoller/git-proton-backup/internal/testtrap"
)

// trapDir is the directory TestMain's single Install call created. The tests
// read it rather than calling Install again, so no second directory leaks and
// DirEnv stays pointing at the one trap this process owns.
var trapDir string

// TestMain is the call shape every package that can reach
// transport.NewCLI("") uses: the Active check first, ahead of anything else
// (a trap copy re-executes this very binary, so it must exit before flag
// parsing or m.Run), then Install once, then an explicit RemoveAll after
// m.Run, because defers do not run on os.Exit.
func TestMain(m *testing.M) {
	if testtrap.Active() {
		os.Exit(testtrap.Run(os.Stderr))
	}
	dir, err := testtrap.Install()
	if err != nil {
		fmt.Fprintln(os.Stderr, "testtrap.Install:", err)
		os.Exit(2)
	}
	trapDir = dir
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// sameDir reports whether two paths name the same directory. os.SameFile on
// the Stats, never a string comparison: macOS reaches the same temp dir as
// /var/folders/... and as /private/var/folders/....
func sameDir(t *testing.T, a, b string) bool {
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

// TestTrapIsFoundFirstAndFails is the whole contract in one run: after
// Install, a PATH lookup of "proton-drive" lands in the trap directory
// whatever else is on PATH, and running what it found exits 3 with Message
// on stderr.
func TestTrapIsFoundFirstAndFails(t *testing.T) {
	if got := os.Getenv(testtrap.DirEnv); got != trapDir {
		t.Fatalf("%s = %q, want the directory Install returned, %q", testtrap.DirEnv, got, trapDir)
	}

	found, err := exec.LookPath("proton-drive")
	if err != nil {
		t.Fatalf("exec.LookPath(proton-drive) after Install: %v", err)
	}
	if !sameDir(t, filepath.Dir(found), os.Getenv(testtrap.DirEnv)) {
		t.Fatalf("exec.LookPath(proton-drive) = %s, want it inside the trap dir %s: "+
			"some other proton-drive is ahead of the trap on PATH", found, os.Getenv(testtrap.DirEnv))
	}

	cmd := exec.Command(found, "--version")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err = cmd.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("running the trap returned %v, want an *exec.ExitError (it must not succeed)", err)
	}
	if code := exitErr.ExitCode(); code != 3 {
		t.Fatalf("trap exit code = %d, want 3", code)
	}
	if want := testtrap.Message + "\n"; stderr.String() != want {
		t.Fatalf("trap stderr = %q, want %q", stderr.String(), want)
	}
}

// TestRunWritesTheMessageAndReturns3 pins Run on its own, without a process.
func TestRunWritesTheMessageAndReturns3(t *testing.T) {
	var w strings.Builder
	if code := testtrap.Run(&w); code != 3 {
		t.Fatalf("Run() = %d, want 3", code)
	}
	if want := testtrap.Message + "\n"; w.String() != want {
		t.Fatalf("Run wrote %q, want %q", w.String(), want)
	}
}

// TestActiveIsFalseInAnOrdinaryTestBinary pins that a trap is identified by
// where it runs: this binary lives in the go build cache, not in the trap
// dir, so even with DirEnv set (Install did that) Active must say no. Were
// it true, every test process of every package would exit 3 at once.
func TestActiveIsFalseInAnOrdinaryTestBinary(t *testing.T) {
	if os.Getenv(testtrap.DirEnv) == "" {
		t.Fatalf("precondition: %s should be set by Install", testtrap.DirEnv)
	}
	if testtrap.Active() {
		t.Fatal("Active() = true in an ordinary test binary; want false")
	}
	t.Setenv(testtrap.DirEnv, "")
	if testtrap.Active() {
		t.Fatal("Active() = true with DirEnv empty; want false")
	}
}

// TestTrapIsRecognisedThroughASymlinkedDir pins the os.SameFile choice
// (PR #15): the trap run through a symlinked spelling of its directory is
// still the trap, whichever spelling os.Executable reports and whichever
// spelling DirEnv carries. A string comparison breaks on at least one pairing
// on macOS, where /var is a symlink to /private/var.
func TestTrapIsRecognisedThroughASymlinkedDir(t *testing.T) {
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(trapDir, alias); err != nil {
		t.Skipf("cannot create a directory symlink here: %v", err)
	}
	exeName := "proton-drive"
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}
	for _, envSpelling := range []string{trapDir, alias} {
		cmd := exec.Command(filepath.Join(alias, exeName))
		cmd.Env = append(os.Environ(), testtrap.DirEnv+"="+envSpelling)
		var stderr strings.Builder
		cmd.Stderr = &stderr
		err := cmd.Run()
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 3 {
			t.Errorf("run via %s with %s=%s: err = %v, stderr = %q; want exit 3 (the trap)",
				alias, testtrap.DirEnv, envSpelling, err, stderr.String())
		}
	}
}
