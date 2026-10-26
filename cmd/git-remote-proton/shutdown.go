package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/craigstoller/git-proton-backup/internal/repo"
	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// The shutdown coordinator: what the helper does when it is told to stop
// (design S2–S11, S17, S18; spec §5.1–§5.3, §5.6).
//
// run() installs it first, before any CLI child can start. On the first
// catchable signal it takes over and ends the process itself. It shuts the
// process-wide CLI gate, so the protocol goroutine parks at its next CLI call
// and never acts on a killed call's result; it stops the CLI child that may be
// in flight; it waits out the settle; it releases every lock in repo's
// register through its own CLI instance; it reports; and it exits with 128 +
// the signal's number. With no signal, run()'s finish stops it and nothing
// else changes.
//
// The first signal's trigger runs §5.3's steps 1 and 2 itself: it arms the cap,
// shuts the gate, silences warn and queues the catch line. Three goroutines do
// the rest, and none waits on another without a bound:
//
//   - the sequence (sequence), which runs steps 3 to 5 in order;
//   - the cap (atCap), a timer armed before anything else, which ends the
//     process at shutdownCap whatever the sequence is doing. It waits on no
//     transport lock and on no cleanup finishing;
//   - the writer (lineQueue.run), the only goroutine that writes to
//     stderrTarget, so a full or dead stderr delays neither of the other two.
//
// Both the sequence and the cap end in final, which a sync.Once runs once, so
// exitFn is called exactly once.

// opMode is what the process is doing, for the lines a shutdown prints (§5.6).
type opMode int

const (
	modeRead    opMode = iota // fetch, list, --version, or nothing yet: no lock is taken
	modePush                  // from list for-push on: the push lock may be held
	modeSetHead               // --set-head: its own lock may be held
)

// curCoord is this process's coordinator: set by run() before anything else,
// cleared by a clean finish. A package variable, not a parameter, because the
// signatures of loop and dispatchUtility are pinned by their tests. Every
// coordinator method is a no-op on a nil receiver, so a test that calls loop
// directly sees no change.
var curCoord *coordinator

// Seams, so tests can shrink or replace them. Production never writes to any
// of them after init. A coordinator copies them on its first signal
// (shutdownConfig), so a test that restores them later cannot race one of its
// goroutines.
var (
	// exitFn ends the process. Only final calls it.
	exitFn = os.Exit
	// stderrTarget is where the coordinator's lines go. Only its writer
	// goroutine writes to it; warn and quietStderr write to os.Stderr.
	stderrTarget io.Writer = os.Stderr
	// newReleaseTransport builds the release transport: the gate's one
	// exemption (S14), traced like the protocol's, its warnings and traces
	// written to w, the coordinator's non-blocking writer.
	newReleaseTransport = func(w io.Writer) transport.Transport {
		return transport.NewTraced(transport.NewReleaseCLI("", w), w)
	}
	stopInFlight     = transport.StopInFlight
	killReleaseCalls = transport.KillReleaseCalls
	// afterLoopHook, when non-nil, runs in run() after loop returns and before
	// finish, on run()'s goroutine. A test seam (E13); nil in production.
	afterLoopHook func()
	// coordinatorBuilt, when non-nil, is told of every coordinator
	// newCoordinator makes, so a test's cleanup can tear each one down. Nil in
	// production.
	coordinatorBuilt func(*coordinator)
)

// The timings, at the spec's values: the cap from the first signal to exit
// (S6); the in-flight child's grace, SIGTERM wait and kill confirmation (S5);
// the pause between release attempts (S10); and how long the final step gives
// stderr (S6). Worst case from the first signal to exit is shutdownCap, plus
// outcomeWait for the cap's kill, plus outcomeWait for the result lines: 62 s,
// inside the 75 s TimeoutStopSec= the docs require.
var shutdownCap, childGrace, childTerm, childConfirm, retryPause, outcomeWait = 60 * time.Second, 5 * time.Second, 3 * time.Second, 2 * time.Second, 2 * time.Second, 1 * time.Second

// silenced is set when shutdown begins. From then on warn and quietStderr drop
// what they are given, so the coordinator's lines are the main account (§5.1).
// Writes straight to os.Stderr are not covered: repo's notes (SetHead's
// release-failure note among them), warnRestoreFloor and headSkipNote. Each
// prints at most a line before the protocol goroutine parks at its next CLI
// call, a residual the spec accepts.
var silenced atomic.Bool

// quietStderr is os.Stderr, silenced once shutdown begins. run() hands it to
// dispatchUtility, so runSetHead's own writes to its stderr are covered.
var quietStderr io.Writer = quietWriter{}

type quietWriter struct{}

// Write looks up os.Stderr on each call rather than once at init, so a test
// that swaps os.Stderr sees these writes too.
func (quietWriter) Write(p []byte) (int, error) {
	if silenced.Load() {
		return len(p), nil
	}
	return os.Stderr.Write(p)
}

// shutdownConfig is a coordinator's copy of the seams and timings, taken on its
// first signal.
type shutdownConfig struct {
	exit                func(int)
	stderr              io.Writer
	newReleaseTransport func(io.Writer) transport.Transport
	stopInFlight        func(grace, term, confirm time.Duration) bool
	killReleaseCalls    func(within time.Duration)

	cap, grace, term, confirm, retryPause, outcomeWait time.Duration
}

func currentConfig() shutdownConfig {
	return shutdownConfig{
		exit:                exitFn,
		stderr:              stderrTarget,
		newReleaseTransport: newReleaseTransport,
		stopInFlight:        stopInFlight,
		killReleaseCalls:    killReleaseCalls,
		cap:                 shutdownCap,
		grace:               childGrace,
		term:                childTerm,
		confirm:             childConfirm,
		retryPause:          retryPause,
		outcomeWait:         outcomeWait,
	}
}

type coordinator struct {
	sigCh  chan os.Signal
	pipeCh chan os.Signal
	done   chan struct{} // closed by stop: the signal and SIGPIPE goroutines and the writer return
	// unpark is closed only by a test's teardown, to let go of a finish parked
	// after shutdown began. In production nothing closes it.
	unpark chan struct{}
	endCh  chan struct{} // closed with ending set

	stopOnce  sync.Once
	finalOnce sync.Once
	bg        sync.WaitGroup // every goroutine the coordinator starts, and its armed cap timer

	mu      sync.Mutex
	mode    opMode
	inBatch bool
	started bool // the first signal arrived: shutdown has begun
	stopped bool // a clean finish ran: later signals are ignored
	torn    bool // a test's teardown ran: no exit from here on

	// headStarted: UpdateHEAD reported that the upload that writes HEAD is
	// starting. Guarded by mu like the flags above; --set-head's "interrupted"
	// line needs it, as the push line needs inBatch.
	headStarted bool

	// Set once, by the first trigger, before any goroutine that reads them
	// starts.
	first     os.Signal
	cfg       shutdownConfig
	capTimer  *time.Timer
	out       *lineQueue
	catchHeld bool // the catch line took the lock-holding form, which promises a result line

	// ending is the one flag the release checks before each attempt: the cap
	// fired, or the final step began. No attempt starts once it is set.
	ending      bool
	finalBegun  bool
	unconfirmed bool // the in-flight child's exit was never confirmed (S5)
	snapTaken   bool
	states      []*lockState  // the one register snapshot; the result lines report exactly these
	attemptDone chan struct{} // non-nil while a release attempt runs; closed when it returns
}

// lockState is what the coordinator knows about one registered lock. Guarded
// by the coordinator's mu.
type lockState struct {
	lock       *repo.Lock
	attempted  bool // false: not attempted, its own state (the zero outcome reads "failed")
	inProgress bool
	outcome    repo.ReleaseOutcome
	err        error
}

// newCoordinator makes a coordinator that is not yet listening for anything.
// installShutdown is the production constructor; tests drive this one through
// trigger.
func newCoordinator() *coordinator {
	c := &coordinator{
		sigCh:  make(chan os.Signal, 4),
		pipeCh: make(chan os.Signal, 1),
		done:   make(chan struct{}),
		unpark: make(chan struct{}),
		endCh:  make(chan struct{}),
	}
	if coordinatorBuilt != nil {
		coordinatorBuilt(c)
	}
	return c
}

// installShutdown starts listening: for the shutdown signals (S2), and on Unix
// for SIGPIPE, whose channel is drained and discarded so that a write to a dead
// git fails with EPIPE instead of killing the helper (§5.2, S16). The signal
// goroutine loops, and trigger returns at once, so a second signal is still
// read while the release runs (S7).
func installShutdown() *coordinator {
	c := newCoordinator()
	// Never with an empty list: Notify with no signals relays every signal.
	if sigs := shutdownSignals(); len(sigs) > 0 {
		signal.Notify(c.sigCh, sigs...)
	}
	notifySIGPIPE(c.pipeCh)
	c.bg.Add(2)
	go func() {
		defer c.bg.Done()
		for {
			select {
			case <-c.pipeCh:
				// Discarded: the failed write itself is what loop acts on.
			case <-c.done:
				return
			}
		}
	}()
	go func() {
		defer c.bg.Done()
		for {
			select {
			case s := <-c.sigCh:
				c.trigger(s)
			case <-c.done:
				return
			}
		}
	}()
	return c
}

func (c *coordinator) setMode(m opMode) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mode = m
}

// batchStarted marks a push batch as running its remote writes; batchDone, once
// its status lines are flushed, marks it over. A shutdown in between says the
// push did not finish (§5.6).
func (c *coordinator) batchStarted() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.inBatch = true
}

func (c *coordinator) batchDone() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.inBatch = false
}

// headWriteStarted records that the upload that writes HEAD is about to start.
// Until then a --set-head shutdown has not touched HEAD, and says nothing about
// it (§5.6). A signal that lands between UpdateHEAD's Stat and this call can
// still make the line print, conservatively: the call fires after the gate has
// shut, and the CLI call that would write HEAD then parks at the gate.
func (c *coordinator) headWriteStarted() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.headStarted = true
}

// finish is called on every return from run(). The choice between "a signal
// started shutdown, so wait for the coordinator" and "no signal, so stop" is
// made once, under the same lock trigger takes, so the two never both happen
// (§5.1). Nothing is done while holding it.
func (c *coordinator) finish(code int) int {
	if c == nil {
		return code
	}
	// The operation is over, so nothing calls the hook again: leave none for a
	// later in-process run() or test to find.
	repo.OnHEADWrite = nil
	c.mu.Lock()
	began := c.started
	if !began {
		c.stopped = true
	}
	c.mu.Unlock()
	if began {
		// Only the coordinator ends the process now. Otherwise a loop that
		// returned on its own (git closed stdin, say) would exit mid-release.
		<-c.unpark
		return code
	}
	c.stop()
	// loop's and SetHead's defers have released their handles by now; what is
	// left (a failed release) must not reach a later in-process run() (S15).
	repo.ResetRegister()
	curCoord = nil
	return code
}

// stop ends the notifications and the goroutines waiting on them.
func (c *coordinator) stop() {
	c.stopOnce.Do(func() {
		signal.Stop(c.sigCh)
		signal.Stop(c.pipeCh)
		close(c.done)
	})
}

// trigger is the entry point for each signal: the signal goroutine calls it,
// and so do tests. The first signal runs steps 1 and 2 here, under the lock,
// and the rest on a new goroutine; any later one only queues the "still
// running" line (S7). Both return at once: step 2 is a flag under the gate's
// mutex, an atomic store and a queue append. Doing step 2 here, not on the
// sequence's goroutine, puts the catch line first in the queue, before the
// line of a second signal that arrives with the first.
func (c *coordinator) trigger(s os.Signal) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return
	}
	if c.started {
		c.out.add(stillRunningLine(s))
		return
	}
	c.started = true
	c.first = s
	c.cfg = currentConfig()
	// 1. The cap first, so nothing after this can keep the process alive past it.
	c.bg.Add(1)
	c.capTimer = time.AfterFunc(c.cfg.cap, func() {
		defer c.bg.Done()
		c.atCap()
	})
	q := newLineQueue(c.cfg.stderr)
	c.out = q

	// 2. From here no CLI child starts but the coordinator's own, and a call
	// that returns now parks; warn is quiet; then say what is happening.
	transport.ShutGate()
	silenced.Store(true)
	c.catchHeld = len(repo.RegisteredLocks()) > 0
	q.add(catchLine(s, c.mode, c.catchHeld))

	c.bg.Add(2)
	go func() {
		defer c.bg.Done()
		q.run(c.done)
	}()
	go c.sequence()
}

// sequence runs steps 3 to 5 of §5.3 and then the final step, whichever way it
// leaves them.
func (c *coordinator) sequence() {
	defer c.bg.Done()
	defer c.final()

	// 3. Stop the in-flight child, and 4. release nothing unless its exit was
	// confirmed: our own write could still be landing.
	if !c.cfg.stopInFlight(c.cfg.grace, c.cfg.term, c.cfg.confirm) {
		c.mu.Lock()
		c.unconfirmed = true
		c.snapshotLocked()
		c.mu.Unlock()
		return
	}

	// 5. One snapshot of the register, then the settle and the release.
	c.mu.Lock()
	if c.ending {
		c.mu.Unlock()
		return
	}
	c.snapshotLocked()
	states := c.states
	c.mu.Unlock()
	if len(states) == 0 {
		return
	}
	repo.Settle()
	c.releaseAll(states, c.cfg.newReleaseTransport(asyncWriter{c.out}))
}

// snapshotLocked takes the one snapshot of the register, if it has not been
// taken. The sequence takes it before the release; the final step takes it
// when nothing else did (the cap fired first).
func (c *coordinator) snapshotLocked() {
	if c.snapTaken {
		return
	}
	c.snapTaken = true
	for _, l := range repo.RegisteredLocks() {
		c.states = append(c.states, &lockState{lock: l})
	}
}

// releaseAll releases each lock until its outcome is final, pausing retryPause
// between attempts (S10). Before each attempt it checks the one flag, ending,
// and stops when it is set: elapsed time is not the test. Each attempt's
// outcome and error are recorded as it returns, so the cap can report them.
func (c *coordinator) releaseAll(states []*lockState, rt transport.Transport) {
	for _, ls := range states {
		for {
			c.mu.Lock()
			if c.ending {
				c.mu.Unlock()
				return
			}
			done := make(chan struct{})
			ls.attempted, ls.inProgress = true, true
			c.attemptDone = done
			c.mu.Unlock()

			out, err := ls.lock.ReleaseVia(rt)

			c.mu.Lock()
			ls.outcome, ls.err, ls.inProgress = out, err, false
			c.attemptDone = nil
			c.mu.Unlock()
			close(done)
			if out.Final() {
				break
			}
			if !c.pause() {
				return
			}
		}
	}
}

// pause waits retryPause, and reports false if the end began meanwhile.
func (c *coordinator) pause() bool {
	t := time.NewTimer(c.cfg.retryPause)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-c.endCh:
		return false
	}
}

// endLocked sets ending and closes endCh, once.
func (c *coordinator) endLocked() {
	if !c.ending {
		c.ending = true
		close(c.endCh)
	}
}

// atCap is the cap timer's callback (S6). It stops any further release attempt,
// kills the release calls on a goroutine of its own, and gives that kill, and
// an attempt that was running, at most outcomeWait together before the final
// step, whether or not they finished. Waiting for the running attempt lets its
// result line carry what that attempt returned; the bound means the exit
// depends on no transport lock and on no cleanup finishing.
func (c *coordinator) atCap() {
	c.mu.Lock()
	if c.finalBegun || c.torn {
		c.mu.Unlock()
		return
	}
	c.endLocked()
	var attempt <-chan struct{}
	if c.attemptDone != nil {
		attempt = c.attemptDone
	}
	cfg := c.cfg
	c.mu.Unlock()

	killed := make(chan struct{})
	c.bg.Add(1)
	go func() {
		defer c.bg.Done()
		defer close(killed)
		cfg.killReleaseCalls(cfg.outcomeWait)
	}()
	timer := time.NewTimer(cfg.outcomeWait)
	defer timer.Stop()
	var kill <-chan struct{} = killed
	for kill != nil || attempt != nil { // a nil channel is never ready
		select {
		case <-kill:
			kill = nil
		case <-attempt:
			attempt = nil
		case <-timer.C:
			kill, attempt = nil, nil
		}
	}
	c.final()
}

// final is step 6, run once whoever reaches it first: it stops any further
// attempt, stops the cap timer, builds the result lines from the recorded
// state, hands them to the writer, waits until they are written or outcomeWait
// passes, and ends the process with 128 + the first signal's number (S11).
func (c *coordinator) final() {
	c.finalOnce.Do(func() {
		c.mu.Lock()
		if c.torn {
			c.mu.Unlock()
			return
		}
		c.finalBegun = true
		c.endLocked()
		if c.capTimer.Stop() {
			c.bg.Done() // the cap callback will never run
		}
		c.snapshotLocked()
		lines := c.resultLinesLocked()
		code := 128 + signalNumber(c.first)
		cfg, out := c.cfg, c.out
		c.mu.Unlock()

		out.closeWith(lines)
		out.waitDrained(cfg.outcomeWait)
		cfg.exit(code)
	})
}

// signalNumber is s's number, for the exit code: os.Interrupt is 2 (SIGINT) on
// both platforms, and SIGTERM is 15 on both (Windows' syscall.SIGTERM is
// Signal(0xf)).
func signalNumber(s os.Signal) int {
	if s == os.Interrupt {
		return 2
	}
	if n, ok := s.(syscall.Signal); ok {
		return int(n)
	}
	return 0
}

// signalName names s by its Unix name. On Windows, SIGINT is Ctrl-C or
// Ctrl-Break, and SIGTERM a console close.
func signalName(s os.Signal) string {
	switch signalNumber(s) {
	case 1:
		return "SIGHUP"
	case 2:
		return "SIGINT"
	case 15:
		return "SIGTERM"
	}
	return s.String()
}

// The lines a shutdown prints (§5.6). Each starts like warn's.

const linePrefix = "git-remote-proton: "

func catchLine(s os.Signal, m opMode, holdsLock bool) string {
	if !holdsLock {
		return fmt.Sprintf("%sreceived %s: stopping (this process holds no remote lock)\n",
			linePrefix, signalName(s))
	}
	op := "the push"
	if m == modeSetHead {
		op = "--set-head"
	}
	return fmt.Sprintf("%sreceived %s: stopping %s and checking the remote lock, which takes about "+
		"a minute. Until the result line below, do not start another push or --set-head on this "+
		"repo, and do not remove the lock by hand; kill -9 or Stop-Process abandons the check and "+
		"may leave the lock in place\n", linePrefix, signalName(s), op)
}

func stillRunningLine(s os.Signal) string {
	return fmt.Sprintf("%sreceived %s while already stopping: ignored; the shutdown, and any lock "+
		"check, is still running and ends about a minute after the first signal\n", linePrefix, signalName(s))
}

const (
	pushInterruptedLine = linePrefix + "the push did not finish: some of its remote changes may " +
		"have landed while git was told nothing, because statuses go out only after the whole " +
		"batch; re-running the push starts from the remote's real state\n"
	setHeadInterruptedLine = linePrefix + "--set-head was interrupted: HEAD may or may not have " +
		"changed; re-running the same --set-head settles it\n"
	// noLockLeftLine is the result when the catch line took the lock-holding
	// form and the snapshot came back empty: the register's one entry left it
	// in between (AcquireLock's Refused arm, say).
	noLockLeftLine = linePrefix + "no lock of ours remains registered; nothing to release\n"
)

// resultLinesLocked builds the result: a line on the interrupted operation, then
// one line per lock in the snapshot. When the snapshot is empty, nothing if the
// catch line already said there was no lock, and otherwise the one line that
// answers its "until the result line below".
func (c *coordinator) resultLinesLocked() []string {
	if len(c.states) == 0 {
		if c.catchHeld {
			return []string{noLockLeftLine}
		}
		return nil
	}
	var lines []string
	switch {
	case c.mode == modePush && c.inBatch:
		lines = append(lines, pushInterruptedLine)
	case c.mode == modeSetHead && c.headStarted:
		lines = append(lines, setHeadInterruptedLine)
	}
	why := "the shutdown time limit was reached before its release could start"
	if c.unconfirmed {
		why = "the CLI call that was running when the signal arrived did not confirm its exit, " +
			"so a release could have raced that call's own write"
	}
	for _, ls := range c.states {
		lines = append(lines, ls.line(why))
	}
	return lines
}

// line is ls's result line: the lock's path and nonce, then the outcome in
// words. notAttempted is the reason given when no attempt started.
func (ls *lockState) line(notAttempted string) string {
	p, n := ls.lock.Path(), ls.lock.Nonce()
	head := fmt.Sprintf("%slock %s (nonce %s): ", linePrefix, p, n)
	switch {
	case !ls.attempted:
		return head + "not attempted: " + notAttempted + "; " + manualRemedy(p, n) + "\n"
	case ls.outcome == repo.Removed:
		return head + "removed: the lock is gone\n"
	case ls.outcome == repo.Absent:
		return head + "absent: no lock of ours is on the remote (it was never created, or was already removed)\n"
	case ls.outcome == repo.Foreign:
		return head + "foreign: another process holds the lock; it was left alone\n"
	case ls.outcome == repo.Unreadable:
		// ReleaseVia's error is today's unreadable-lock message.
		msg := "the lock is present but its contents are unreadable; not removed"
		if ls.err != nil {
			msg = ls.err.Error()
		}
		return head + "unreadable: " + msg + "\n"
	}
	return head + ls.outcome.String() + ": " + ls.failReason() + "; " + manualRemedy(p, n) + "\n"
}

func (ls *lockState) failReason() string {
	const running = "its release call had not returned when the shutdown time limit was reached"
	switch {
	case ls.inProgress && ls.err != nil:
		return running + " (the attempt before it failed: " + ls.err.Error() + ")"
	case ls.inProgress:
		return running
	case ls.err != nil:
		return ls.err.Error()
	}
	return "the release did not complete"
}

// manualRemedy is the lock-held message's remedy, worded for after the fact: it
// applies only once no push for this repo is running, and a lock bearing this
// nonce that turns up later is ours.
func manualRemedy(p, nonce string) string {
	return fmt.Sprintf("once no push or --set-head for this repo is running, check with the "+
		"Proton CLI whether %s still exists and, if it does, remove it (proton-drive filesystem "+
		"trash %s); a lock bearing nonce %s, if one appears there later, is ours to remove too",
		p, p, nonce)
}

// maxQueuedLines bounds the ordinary lines waiting for the writer. A result
// line is never dropped for it.
const maxQueuedLines = 64

// lineQueue is the coordinator's one ordered stderr writer (S6). Appending
// never blocks: lines wait in a mutex-guarded FIFO for the writer goroutine,
// which writes them in order and drops any whose write fails. An ordinary line
// is dropped when the queue already holds maxQueuedLines; the result lines are
// always appended, so a full queue cannot lose them. Only a stderr that is
// blocked or dead can lose lines, and then nobody could read them anyway.
type lineQueue struct {
	w       io.Writer
	wake    chan struct{} // capacity 1: there may be lines
	drained chan struct{} // closed by the writer once the queue is closed and empty

	mu      sync.Mutex
	pending []string
	closed  bool
}

func newLineQueue(w io.Writer) *lineQueue {
	return &lineQueue{w: w, wake: make(chan struct{}, 1), drained: make(chan struct{})}
}

// add queues an ordinary line, or drops it if the queue is full or closed.
func (q *lineQueue) add(line string) {
	q.mu.Lock()
	if q.closed || len(q.pending) >= maxQueuedLines {
		q.mu.Unlock()
		return
	}
	q.pending = append(q.pending, line)
	q.mu.Unlock()
	q.poke()
}

// closeWith appends the result lines, past the limit, and closes the queue:
// nothing is added after them.
func (q *lineQueue) closeWith(lines []string) {
	q.mu.Lock()
	if !q.closed {
		q.pending = append(q.pending, lines...)
		q.closed = true
	}
	q.mu.Unlock()
	q.poke()
}

func (q *lineQueue) poke() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// waitDrained waits until the writer has written everything up to the close, or
// d has passed.
func (q *lineQueue) waitDrained(d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-q.drained:
	case <-t.C:
	}
}

// run is the writer goroutine. It returns once the closed queue is empty, or
// when done is closed.
func (q *lineQueue) run(done <-chan struct{}) {
	for {
		q.mu.Lock()
		if len(q.pending) > 0 {
			line := q.pending[0]
			q.pending = q.pending[1:]
			q.mu.Unlock()
			q.w.Write([]byte(line)) // a failed write drops the line
			continue
		}
		if q.closed {
			q.mu.Unlock()
			close(q.drained)
			return
		}
		q.mu.Unlock()
		select {
		case <-q.wake:
		case <-done:
			return
		}
	}
}

// asyncWriter is what the release transport writes to: the release CLI's drain
// warning and Traced's "gpb: downloaded" line go through the queue as ordinary
// lines, so they never block the release either (S14).
type asyncWriter struct{ q *lineQueue }

func (a asyncWriter) Write(p []byte) (int, error) {
	a.q.add(string(p))
	return len(p), nil
}
