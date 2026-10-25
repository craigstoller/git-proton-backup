package main

import (
	"errors"
	"os/exec"
	"syscall"
	"unsafe"
)

// The Windows end-to-end signal cases (spec §6, layer 3) live here, beside the
// Windows halves of the harness's per-platform helpers.

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess        = kernel32.NewProc("OpenProcess")
	procGetExitCodeProcess = kernel32.NewProc("GetExitCodeProcess")
)

const (
	processQueryLimitedInformation = 0x1000
	stillActive                    = 259
	errorInvalidParameter          = syscall.Errno(87) // OpenProcess: no such process
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
