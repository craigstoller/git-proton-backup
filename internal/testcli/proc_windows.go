package testcli

import (
	"errors"
	"os/exec"
	"syscall"
	"unsafe"
)

// detach starts cmd's child in a new process group, so a console event sent
// to the shim's group (a Ctrl-Break) does not reach it.
func detach(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
}

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
// reads as alive, so a caller waiting for the death waits on (bounded) rather
// than acting early.
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
