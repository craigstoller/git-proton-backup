// Package testtrap keeps a hermetic test run away from a real proton-drive.
//
// A hermetic test reaches proton-drive through PATH (transport.NewCLI("")),
// and only after prepending its own shim. Nothing stops a shim bug, or a
// future test that resolves the CLI without placing a shim, from falling
// through to a real, signed-in CLI sitting further along PATH. On a Mac that
// CLI can read the login keychain and write local state as well as reach the
// network (docs/research/gates/brief-checklist.md rule 12).
//
// The trap closes that route. A package's TestMain calls Install, which puts a
// stand-in "proton-drive" first on PATH; the stand-in is a copy of the test
// binary itself, and that copy, run under the name proton-drive, finds itself
// in the trap directory (Active) and exits 3 with Message (Run). A shim a test
// places later is prepended ahead of the trap, so shim tests are unaffected.
//
// This package imports only the standard library on purpose: internal/testcli
// imports internal/transport, whose in-package tests must call the trap, so
// putting the trap there would be an import cycle.
package testtrap

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// DirEnv names the environment variable that carries the trap directory. Install
// sets it; Active compares it with the directory the running binary lives in.
const DirEnv = "GPB_TEST_TRAP_DIR"

// Message is what a trap that gets run writes to stderr.
const Message = "a hermetic test reached proton-drive without a shim"

// exitCode is what Run returns. Neither 0 (success) nor 1/2 (the codes a test
// binary and its TestMain already use), so a trap hit is distinguishable.
const exitCode = 3

// Install copies the running executable into a new temporary directory under
// the name proton-drive (proton-drive.exe on Windows, where exec.LookPath
// finds a program only by an extension listed in PATHEXT), records that
// directory in DirEnv, and prepends it to PATH, both with os.Setenv. It
// returns the directory; the caller removes it once m.Run has returned (a
// defer does not run across os.Exit).
//
// Call it once per process, after the TestMain role dispatches that exit early
// and so would otherwise leak a directory each.
func Install() (dir string, err error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	made, err := os.MkdirTemp("", "gpb-trap-*")
	if err != nil {
		return "", err
	}
	// A failure past this point must not leave the directory behind. Removing
	// `made`, not the named result, because `return "", err` has already blanked
	// dir by the time this deferred call runs.
	defer func() {
		if err != nil {
			os.RemoveAll(made)
		}
	}()

	name := "proton-drive"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err = copyExecutable(self, filepath.Join(made, name)); err != nil {
		return "", err
	}
	if err = os.Setenv(DirEnv, made); err != nil {
		return "", err
	}
	if err = os.Setenv("PATH", made+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		return "", err
	}
	return made, nil
}

func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// Active reports whether this process is a trap: the running executable's
// directory and the directory in DirEnv are the same directory by os.SameFile
// on their Stats. A trap is identified by where it runs, so no role variable
// an ordinary test binary might carry can be mistaken for it. The comparison
// is by file identity, never by string: macOS reaches one temp directory as
// /var/... and as /private/var/... (PR #15).
//
// Any failure to find out answers false.
func Active() bool {
	want := os.Getenv(DirEnv)
	if want == "" {
		return false
	}
	self, err := os.Executable()
	if err != nil {
		return false
	}
	here, err := os.Stat(filepath.Dir(self))
	if err != nil {
		return false
	}
	trap, err := os.Stat(want)
	if err != nil {
		return false
	}
	return os.SameFile(here, trap)
}

// Run is what a trap does: write Message and a newline to stderr and return 3,
// for the caller to pass to os.Exit.
func Run(stderr io.Writer) int {
	fmt.Fprintln(stderr, Message)
	return exitCode
}
