package main

import (
	"errors"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/craigstoller/git-proton-backup/internal/repo"
)

// The Windows end-to-end signal cases (spec §6, layer 3) live here, beside the
// Windows halves of the harness's per-platform helpers.

var (
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procGetExitCodeProcess       = kernel32.NewProc("GetExitCodeProcess")
	procGenerateConsoleCtrlEvent = kernel32.NewProc("GenerateConsoleCtrlEvent")
)

const (
	processQueryLimitedInformation = 0x1000
	stillActive                    = 259
	errorInvalidParameter          = syscall.Errno(87) // OpenProcess: no such process
	ctrlBreakEvent                 = 1                 // CTRL_BREAK_EVENT
)

// processAlive reports whether pid names a live process: OpenProcess succeeds
// and GetExitCodeProcess reports STILL_ACTIVE. An OpenProcess that fails with
// ERROR_INVALID_PARAMETER means no such process, so gone. Any other failure
// reads as alive, so assertNoShimLeft can only fail safe.
func processAlive(pid int) bool {
	h, _, err := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if h == 0 {
		return !errors.Is(err, errorInvalidParameter)
	}
	defer syscall.CloseHandle(syscall.Handle(h))
	var code uint32
	if ok, _, _ := procGetExitCodeProcess.Call(h, uintptr(unsafe.Pointer(&code))); ok == 0 {
		return true
	}
	return code == stillActive
}

// setOwnProcessGroup starts the helper in a new process group
// (helperOpts.OwnGroup), so a console event sent to that group does not reach
// the test.
func setOwnProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
}

// ctrlBreak sends a Ctrl-Break to the console process group pid leads (a
// process started with CREATE_NEW_PROCESS_GROUP leads the group of its own
// id), by GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, pid), the method spec P4
// measured from both the PowerShell tool and Git Bash. Ctrl-C cannot be sent
// this way: CREATE_NEW_PROCESS_GROUP disables it in the new group. The error is
// the call's own, as when this process has no console to share with the group.
func ctrlBreak(t *testing.T, pid int) error {
	t.Helper()
	if ok, _, err := procGenerateConsoleCtrlEvent.Call(ctrlBreakEvent, uintptr(pid)); ok == 0 {
		return err
	}
	return nil
}

// TestSignal_W1_CtrlBreakDuringThePackUpload is W1, E1's Windows twin: a
// Ctrl-Break to the helper's process group (it is started with
// CREATE_NEW_PROCESS_GROUP) while its push is blocked in the pack upload with
// the lock held. Go delivers the event as os.Interrupt, so the helper says
// SIGINT, settles, releases the lock through its own CLI instance, says
// removed, and exits 130.
//
// The blocked shim shares the helper's group, so it dies of the same
// Ctrl-Break, as it would of one typed in the console. W1 therefore proves the
// Windows release path, not that the helper can stop a child that ignored the
// event. That stop path is internal/transport's TestStopInFlightKillsAHungChild
// (Task 2), which also runs on Windows.
func TestSignal_W1_CtrlBreakDuringThePackUpload(t *testing.T) {
	h := startHelper(t, helperOpts{Block: "upload:*.pack:once:before", OwnGroup: true})
	h.pushMain(t)
	shim := readyPid(t, waitReady(t, h.Ready, "ready-upload-", 20*time.Second))
	if !lockPresent(h.Tree, harnessRoot) {
		t.Fatalf("%s/%s is not in the shim tree before the Ctrl-Break", harnessRoot, repo.LockName)
	}
	sent := time.Now()
	if err := ctrlBreak(t, h.Cmd.Process.Pid); err != nil {
		t.Skipf("GenerateConsoleCtrlEvent failed: %v (no console?)", err)
	}
	// The event, not the helper's kill at the end of the grace, is what ends
	// the shim: that is the claim above.
	if died := waitGone(t, shim, sent, 10*time.Second); died >= shrunkGrace {
		t.Errorf("the shim was gone %v after the Ctrl-Break, not inside the %v grace: the event did not reach it",
			died, shrunkGrace)
	}
	code := h.wait(t, 20*time.Second)
	stderr := h.Stderr.String()
	if code != 130 {
		t.Errorf("exit code = %d, want 130 (128 + SIGINT, Go's os.Interrupt for a Ctrl-Break); stderr:\n%s", code, stderr)
	}
	if lockPresent(h.Tree, harnessRoot) {
		t.Errorf("%s/%s is still in the shim tree after the release; stderr:\n%s", harnessRoot, repo.LockName, stderr)
	}
	assertNoShimLeft(t, h.Tree)
	if !strings.Contains(stderr, pushCatch("SIGINT")) {
		t.Errorf("stderr lacks the catch line %q:\n%s", pushCatch("SIGINT"), stderr)
	}
	if _, words := lockResult(t, stderr, harnessRoot); words != removedWords {
		t.Errorf("result = %q, want %q; stderr:\n%s", words, removedWords, stderr)
	}
}

// TestSignal_W2_SecondCtrlBreakDuringTheRelease is W2, E9's Windows twin: W1's
// Ctrl-Break during the pack upload, then a second Ctrl-Break to the helper's
// group while the release's trash call is slow (held 1.5 s, inside the release
// call's bound). The helper only notes the second, the release completes, and
// the exit code is the first's, 130.
//
// The release starts its children in a process group of their own, so the
// second event, sent to the helper's group, does not reach the trash call:
// that is the group targeting this case pins. The lock's removal alone would
// not show it, because a trash killed by the event would be retried, and the
// retry would remove the lock. So the case checks the trash call's shim
// itself: a shim run that returns removes its pid record, and only a killed one
// leaves it behind.
func TestSignal_W2_SecondCtrlBreakDuringTheRelease(t *testing.T) {
	h := startHelper(t, helperOpts{
		Block:    "upload:*.pack:once:before;trash:.lock:once:before:delay=1500",
		OwnGroup: true,
	})
	h.pushMain(t)
	waitReady(t, h.Ready, "ready-upload-", 20*time.Second)
	if !lockPresent(h.Tree, harnessRoot) {
		t.Fatalf("%s/%s is not in the shim tree before the first Ctrl-Break", harnessRoot, repo.LockName)
	}
	if err := ctrlBreak(t, h.Cmd.Process.Pid); err != nil {
		t.Skipf("GenerateConsoleCtrlEvent failed: %v (no console?)", err)
	}
	trash := readyPid(t, waitReady(t, h.Ready, "ready-trash-", 20*time.Second))
	if !lockPresent(h.Tree, harnessRoot) {
		t.Fatalf("%s/%s is gone before the held trash has run", harnessRoot, repo.LockName)
	}
	// The first event reached this console, so a failure now is no skip.
	if err := ctrlBreak(t, h.Cmd.Process.Pid); err != nil {
		t.Fatalf("the second GenerateConsoleCtrlEvent failed after the first succeeded: %v", err)
	}
	code := h.wait(t, 20*time.Second)
	stderr := h.Stderr.String()
	if code != 130 {
		t.Errorf("exit code = %d, want 130 (128 + SIGINT, the first Ctrl-Break); stderr:\n%s", code, stderr)
	}
	if lockPresent(h.Tree, harnessRoot) {
		t.Errorf("%s/%s is still in the shim tree after the release; stderr:\n%s", harnessRoot, repo.LockName, stderr)
	}
	assertNoShimLeft(t, h.Tree)
	for _, p := range recordedShims(t, h.Tree) {
		if p.pid == trash {
			t.Errorf("the release's trash shim (pid %d) left its pid record: it was killed, not left to finish, "+
				"so the second Ctrl-Break reached the release child; stderr:\n%s", trash, stderr)
		}
	}
	if n := strings.Count(stderr, pushCatch("SIGINT")); n != 1 {
		t.Errorf("stderr has %d catch lines, want 1: the second Ctrl-Break must not get one of its own:\n%s", n, stderr)
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
