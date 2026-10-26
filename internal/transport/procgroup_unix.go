//go:build !windows

package transport

import (
	"os"
	"os/exec"
	"syscall"
)

// setOwnGroup starts cmd's child as the leader of a new process group (S7), so
// a Ctrl-C typed later in the same terminal, which goes to the terminal's
// foreground group, does not reach it, and killGroup can end it together with
// anything it spawned.
func setOwnGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// terminate asks p to exit with SIGTERM. It returns true: Unix has a gentle
// signal, so StopInFlight's SIGTERM wait applies. A child that is already gone
// makes Signal fail, which changes nothing: the wait sees the exit.
func terminate(p *os.Process) bool {
	p.Signal(syscall.SIGTERM)
	return true
}

// killGroup sends SIGKILL to p's whole process group, which setOwnGroup made p
// the leader of. It signals the group by NUMBER, with nothing like os.Process's
// guard against a reaped pid, so every caller checks under gateMu first that
// p's exit has not been observed (flight.kill).
func killGroup(p *os.Process) error {
	return syscall.Kill(-p.Pid, syscall.SIGKILL)
}
