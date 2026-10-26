//go:build !windows

package testcli

import (
	"os/exec"
	"syscall"
)

// detach starts cmd's child as the leader of a process group of its own, so a
// signal to the shim's group (a kill of the helper's whole group) does not
// reach it.
func detach(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// processAlive reports whether pid names a live process: kill(pid, 0)
// succeeds. A killed shim reads as alive until the helper reaps it, which its
// cmd.Wait does at once; a shim whose parent has gone is reparented and reaped.
func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}
