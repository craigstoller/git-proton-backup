package transport

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// The process-wide CLI gate and the in-flight record (design S5, S7, S10, S14;
// spec §5.3).
//
// When the helper is signalled, the shutdown coordinator must know that
// nothing of ours can still reach the remote before it releases the lock. So
// it shuts this gate, after which no *CLI starts a child (S14), and stops the
// one child that may already be running (StopInFlight, S5). Then it releases
// through its own instance, NewReleaseCLI's, the gate's one exemption: its
// children run in their own process group (S7), each call is bounded (S10),
// and KillReleaseCalls ends any still running at the cap.
//
// ONE mutex, gateMu, guards everything below together: the shut flag, the
// reopen channel, release admission, and both in-flight sets. That is what
// makes "check the gate, then record the call" atomic, so no child can start
// between the two. It is NEVER held across a child's start: a start that
// stalls must not block ShutGate or KillReleaseCalls, which is why run
// reserves its record before the start and fills it in after.
var (
	gateMu sync.Mutex

	gateShut   bool
	gateReopen = make(chan struct{}) // closed (and replaced) by ReopenGateForTest

	releaseClosed bool // set by KillReleaseCalls

	inFlight        = map[*flight]struct{}{} // gated calls: StopInFlight's set
	releaseInFlight = map[*flight]struct{}{} // release calls: KillReleaseCalls's set
)

var (
	// errGateReopened is what a parked call returns, and only ever in a test:
	// in production nothing reopens the gate, and the coordinator ends the
	// process with the call still parked.
	errGateReopened = errors.New("transport: the CLI gate shut during this call and was reopened (test only)")

	// errReleaseClosed is what a release call returns once KillReleaseCalls
	// has closed admission at the shutdown cap.
	errReleaseClosed = errors.New("transport: release calls are closed: the shutdown cap was reached")
)

// The release instance's bounds (S10). releaseCallTimeout bounds one call from
// its start to the end of its output; releaseDrain bounds the drain after its
// exit, in place of waitDelay's 30 s. NewReleaseCLI copies both when it is
// called. They are vars only so tests can shrink them; nothing in production
// writes to them.
var (
	releaseCallTimeout = 15 * time.Second
	releaseDrain       = 2 * time.Second
)

// startCmd is how run starts a child. A test seam, so a test can make a start
// stall and show that nothing waits on it under the lock.
var startCmd = (*exec.Cmd).Start

// flight is one call's entry in an in-flight set, from just before its start
// to just after its drain. proc and exitSeen are guarded by gateMu.
type flight struct {
	release  bool
	proc     *os.Process   // nil until the start has returned
	exited   chan struct{} // closed once the child's exit is observed, or when no child started
	exitSeen bool          // exited has been closed
}

// admit is the gate. It returns a new record for the call, already in its
// set, or the channel a gated call must park on, or errReleaseClosed.
func admit(release bool) (*flight, <-chan struct{}, error) {
	gateMu.Lock()
	defer gateMu.Unlock()
	if release {
		if releaseClosed {
			return nil, nil, errReleaseClosed
		}
		f := &flight{release: true, exited: make(chan struct{})}
		releaseInFlight[f] = struct{}{}
		return f, nil, nil
	}
	if gateShut {
		return nil, gateReopen, nil
	}
	f := &flight{exited: make(chan struct{})}
	inFlight[f] = struct{}{}
	return f, nil, nil
}

// fill records the started child. If the gate shut (or, for a release call,
// admission closed) while the start was in progress, the child is killed at
// once and stays in flight, so StopInFlight and KillReleaseCalls wait on it
// like any other.
func (f *flight) fill(p *os.Process) {
	gateMu.Lock()
	defer gateMu.Unlock()
	f.proc = p
	if (f.release && releaseClosed) || (!f.release && gateShut) {
		f.kill()
	}
}

// kill ends the child, a release child with its whole group. The caller holds
// gateMu. It does nothing before the start has returned (fill kills then, if
// it must) and nothing once the exit was observed: a release child's group is
// signalled by number, and after the exit that number may already be free.
func (f *flight) kill() {
	if f.proc == nil || f.exitSeen {
		return
	}
	if f.release {
		killGroup(f.proc)
	} else {
		f.proc.Kill()
	}
}

// killAtBound is a release call's kill timer. Timer.Stop cannot recall a
// callback that is already running, so the callback takes the lock and kill
// checks there whether the exit was observed. That check, not Stop, is what
// keeps a late timer from signalling the group once run has recorded the exit.
// What it cannot cover is the instant between the kernel reaping the child
// inside cmd.Wait and run taking the lock (Go has no hook there): a signal in
// that instant reaches the group's other members if any remain, and a stranger
// only if the pid was reused, as a group leader, within it.
func (f *flight) killAtBound() {
	gateMu.Lock()
	defer gateMu.Unlock()
	f.kill()
}

// exit records that the child's exit was observed, or that no child started.
func (f *flight) exit() {
	gateMu.Lock()
	defer gateMu.Unlock()
	f.exitSeen = true
	close(f.exited)
}

// leave removes the record once the call's result is ready. It returns the
// channel to park on when the call is gated and the gate shut meanwhile: the
// caller must never see a result that arrived after that.
func (f *flight) leave() <-chan struct{} {
	gateMu.Lock()
	defer gateMu.Unlock()
	if f.release {
		delete(releaseInFlight, f)
		return nil
	}
	delete(inFlight, f)
	if gateShut {
		return gateReopen
	}
	return nil
}

// noChild ends a call whose child never started: there is no exit to wait
// for. A gated call whose gate shut meanwhile still parks.
func (f *flight) noChild(err error) (string, int, error) {
	f.exit()
	if park := f.leave(); park != nil {
		return parked(park)
	}
	return "", -1, err
}

// parked blocks a gated call until ReopenGateForTest, which in production
// never comes.
func parked(park <-chan struct{}) (string, int, error) {
	<-park
	return "", -1, errGateReopened
}

// ShutGate shuts the gate: from now on run on any non-release *CLI starts no
// child, and a gated call, new or returning, parks. Idempotent.
func ShutGate() {
	gateMu.Lock()
	defer gateMu.Unlock()
	gateShut = true
}

// StopInFlight stops every in-flight non-release child (S5). The record is a
// set, not one slot: the protocol goroutine runs one call at a time today, but
// the set makes that an observation rather than an assumption. Each child gets
// grace to exit on its own; then, on Unix, SIGTERM and term more; then a kill
// and confirm more. The phases share one clock across the children, so the
// whole stop takes at most grace + term + confirm however many there are.
//
// It returns true when nothing was in flight or every exit was confirmed: the
// process reaped, not its output drained (that can run on for waitDelay). It
// returns false otherwise, and the coordinator then releases nothing.
//
// It is meant to run after ShutGate. A child whose start is still in progress
// when the kill comes has no process to kill yet; its own run kills it when
// the start returns, because the gate is shut.
func StopInFlight(grace, term, confirm time.Duration) bool {
	gateMu.Lock()
	left := make([]*flight, 0, len(inFlight))
	for f := range inFlight {
		left = append(left, f)
	}
	gateMu.Unlock()

	left = waitExits(left, grace)
	if len(left) > 0 {
		sent := false
		gateMu.Lock()
		for _, f := range left {
			if f.proc != nil && !f.exitSeen && terminate(f.proc) {
				sent = true
			}
		}
		gateMu.Unlock()
		if sent {
			left = waitExits(left, term)
		}
	}
	if len(left) > 0 {
		gateMu.Lock()
		for _, f := range left {
			f.kill()
		}
		gateMu.Unlock()
		left = waitExits(left, confirm)
	}
	return len(left) == 0
}

// KillReleaseCalls ends the release phase at the cap. Under the lock it first
// closes release admission, so a release call that was about to start cannot
// slip past the kill: from then on a release call returns errReleaseClosed
// without starting a child, and one whose start is in progress is killed as
// soon as the start returns. Then it kills every in-flight release child,
// group and all on Unix, and waits up to within for them to be reaped.
func KillReleaseCalls(within time.Duration) {
	gateMu.Lock()
	releaseClosed = true
	snap := make([]*flight, 0, len(releaseInFlight))
	for f := range releaseInFlight {
		snap = append(snap, f)
		f.kill()
	}
	gateMu.Unlock()
	waitExits(snap, within)
}

// waitExits waits up to d for every record's exit and returns those whose
// exit has still not been observed. It holds no lock.
func waitExits(fs []*flight, d time.Duration) []*flight {
	timer := time.NewTimer(d)
	defer timer.Stop()
	for i, f := range fs {
		select {
		case <-f.exited:
		case <-timer.C:
			var left []*flight
			for _, f := range fs[i:] {
				select {
				case <-f.exited:
				default:
					left = append(left, f)
				}
			}
			return left
		}
	}
	return nil
}

// NewReleaseCLI returns the coordinator's instance (S7, S10, S14): exempt from
// the gate, its children started in their own process group, its warnings
// written to w. It copies releaseCallTimeout and releaseDrain now, so a test
// that changes them later does not race a call already running. Each call's
// whole life stays within the call bound: the child is killed at
// callTimeout - drain (on Unix with its whole group), and the drain after its
// exit gets at most drain.
func NewReleaseCLI(exe string, w io.Writer) *CLI {
	if exe == "" {
		exe = "proton-drive"
	}
	return &CLI{
		Exe:         exe,
		callTimeout: releaseCallTimeout,
		drain:       releaseDrain,
		warnW:       w,
		release:     true,
	}
}

// ReopenGateForTest reopens the gate and releases every parked call, which
// returns errGateReopened. Test only: production never reopens the gate.
func ReopenGateForTest() {
	gateMu.Lock()
	defer gateMu.Unlock()
	gateShut = false
	close(gateReopen)
	gateReopen = make(chan struct{})
}

// ResetReleaseAdmissionForTest reopens release admission after a
// KillReleaseCalls. Test only.
func ResetReleaseAdmissionForTest() {
	gateMu.Lock()
	defer gateMu.Unlock()
	releaseClosed = false
}

// SetReleaseBoundsForTest sets the release bounds for instances made after it,
// for a test outside this package. It panics unless 0 < drain < call/2, so a
// release call always gets real time before its kill. Test only.
func SetReleaseBoundsForTest(call, drain time.Duration) {
	if drain <= 0 || drain >= call/2 {
		panic("transport: SetReleaseBoundsForTest needs 0 < drain < call/2, got call " +
			call.String() + ", drain " + drain.String())
	}
	releaseCallTimeout, releaseDrain = call, drain
}
