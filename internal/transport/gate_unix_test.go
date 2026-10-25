//go:build !windows

package transport

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// processAlive reports whether pid names a live process. Signal 0 checks that
// the pid exists; a zombie passes that check, so on Linux /proc settles it (a
// killed grandchild is a zombie until whoever inherited it reaps it). EPERM
// means the pid exists but is not ours: reported alive, the cautious answer.
func processAlive(pid int) bool {
	if err := syscall.Kill(pid, 0); err != nil {
		return errors.Is(err, syscall.EPERM)
	}
	if b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid)); err == nil {
		// The state letter follows the parenthesised command name.
		if i := bytes.LastIndexByte(b, ')'); i >= 0 && i+2 < len(b) && b[i+2] == 'Z' {
			return false
		}
	}
	return true
}

// signalOf returns the signal that ended a run's child, or 0.
func signalOf(err error) syscall.Signal {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return ws.Signal()
		}
	}
	return 0
}

// RED. On Unix a hung child is sent SIGTERM after the grace, and one that
// honours it dies of it: the kill is the last step, not the first.
func TestStopInFlightSendsSIGTERMBeforeTheKill(t *testing.T) {
	g := newGateTest(t, roleHang, true)
	ch := g.run(&CLI{Exe: helperExe})
	g.waitMarkers("started", 1)

	if !StopInFlight(100*time.Millisecond, 2*time.Second, 2*time.Second) {
		t.Fatal("StopInFlight could not confirm the exit")
	}
	r := within(t, ch, 3*time.Second, "a child sent SIGTERM")
	if sig := signalOf(r.err); sig != syscall.SIGTERM {
		t.Errorf("the child must have died of SIGTERM, got signal %d (err %v)", sig, r.err)
	}
}

// RED (compile). A child that ignores SIGTERM is killed once the SIGTERM wait
// runs out, and the exit is still confirmed.
func TestStopInFlightKillsAChildThatIgnoresSIGTERM(t *testing.T) {
	g := newGateTest(t, roleHangIgnoreTerm, true)
	ch := g.run(&CLI{Exe: helperExe})
	pid := g.waitMarkers("started", 1)[0]

	start := time.Now()
	if !StopInFlight(200*time.Millisecond, 200*time.Millisecond, 2*time.Second) {
		t.Fatal("StopInFlight could not confirm the exit of a child that ignores SIGTERM")
	}
	if el := time.Since(start); el < 350*time.Millisecond {
		t.Errorf("StopInFlight returned after %s: it did not wait out the grace and the SIGTERM wait", el)
	}
	if processAlive(pid) {
		t.Errorf("StopInFlight confirmed the exit, but process %d is still alive", pid)
	}
	r := within(t, ch, 3*time.Second, "a child that ignored SIGTERM")
	if sig := signalOf(r.err); sig != syscall.SIGKILL {
		t.Errorf("the child must have died of SIGKILL, got signal %d (err %v)", sig, r.err)
	}
}

// RED. A release call that runs out its bound is killed with its whole
// process group: the grandchild named in the "grandchild" marker dies too.
func TestReleaseCLIKillsTheWholeGroupOnUnix(t *testing.T) {
	g := newGateTest(t, roleHangWithGrandchild, true)
	releaseCallTimeout, releaseDrain = time.Second, 200*time.Millisecond
	ch := g.run(NewReleaseCLI(helperExe, io.Discard))
	parent := g.waitMarkers("started", 1)[0]
	grandchild := g.waitMarkers("grandchild", 1)[0]

	if r := within(t, ch, 2*time.Second, "a hung release call with a grandchild"); r.code == 0 {
		t.Errorf("a release call killed at its bound must not report success, got (%q, %d, %v)", r.out, r.code, r.err)
	}
	if processAlive(parent) {
		t.Errorf("the release child %d is still alive after its bound", parent)
	}
	waitGone(t, grandchild, "the grandchild in the release child's process group")
}

// RED. Timer.Stop cannot recall a kill callback that is already running, so
// the callback itself must do nothing once the child's exit was observed.
// This drives the callback into exactly that state: testHookExited runs it
// after run has recorded the exit. The child (cli) has exited, but its
// grandchild, in the same process group, still holds the pipe. A callback that
// signalled the group anyway would kill the grandchild, and the drain would
// then reach EOF instead of running out.
func TestReleaseKillCallbackDoesNothingOnceTheExitWasObserved(t *testing.T) {
	g := newGateTest(t, roleCLI, false)
	releaseCallTimeout, releaseDrain = 10*time.Second, 300*time.Millisecond
	c := NewReleaseCLI(helperExe, io.Discard)
	fired := 0
	testHookExited = func(int) {
		for _, f := range releaseRecords() {
			f.killAtBound()
			fired++
		}
	}

	r := within(t, g.run(c, "--version"), 5*time.Second, "a release call whose grandchild holds the pipe")
	if fired != 1 {
		t.Fatalf("the hook ran the callback for %d release records, want 1", fired)
	}
	if !errors.Is(r.err, exec.ErrWaitDelay) {
		t.Errorf("want exec.ErrWaitDelay (the grandchild still held the pipe), got %v: a kill callback "+
			"that ran after the exit was observed signalled the group", r.err)
	}
}
