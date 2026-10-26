package transport

import (
	"os"
	"os/exec"
	"syscall"
)

// setOwnGroup starts cmd's child in a new process group (S7). Windows then
// disables Ctrl-C for it, so a later Ctrl-C in the same console does not reach
// it. A typed Ctrl-Break still does [doc]; S10's retry covers that.
func setOwnGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
}

// terminate returns false: Windows has no gentle signal for one process, so
// StopInFlight goes from the grace straight to the kill.
func terminate(p *os.Process) bool {
	return false
}

// killGroup kills p. A Windows process group is not a unit that can be killed,
// so a release call's grandchildren outlive it; the drain bound still ends the
// call when one of them holds the pipe.
func killGroup(p *os.Process) error {
	return p.Kill()
}
