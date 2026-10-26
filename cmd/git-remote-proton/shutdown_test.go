package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/craigstoller/git-proton-backup/internal/repo"
	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// Tests for the shutdown coordinator (shutdown.go). They send no real OS
// signal: each drives c.trigger directly, or, in
// TestShutdown_SecondOSSignalIsReadDuringTheRelease, sends on the signal
// goroutine's own channel. Each runs with shrunk timings, an exitFn that
// records its codes and never exits, a syncBuffer behind stderrTarget, and a
// Fake behind newReleaseTransport. Line-order assertions compare the
// coordinator's own lines only: repo's direct stderr notes are outside that
// order (spec §5.1), and none of these tests produces any.

const (
	sdRoot = "/my-files/r"
	sdLock = sdRoot + "/" + repo.LockName
)

// syncBuffer is a bytes.Buffer safe to write from one goroutine while another
// reads it: the coordinator's writer goroutine writes, the test reads.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// blockingWriter blocks every Write until unblock, then hands it to next (or
// drops it when next is nil): a stderr nobody reads. saveShutdownStateForTest's
// cleanup unblocks every one a test made.
type blockingWriter struct {
	release chan struct{}
	once    sync.Once
	next    io.Writer
}

func newBlockingWriter(next io.Writer) *blockingWriter {
	b := &blockingWriter{release: make(chan struct{}), next: next}
	testWriters = append(testWriters, b)
	return b
}

func (b *blockingWriter) Write(p []byte) (int, error) {
	<-b.release
	if b.next == nil {
		return len(p), nil
	}
	return b.next.Write(p)
}

func (b *blockingWriter) unblock() { b.once.Do(func() { close(b.release) }) }

// slowWriter delays each Write by d before handing it on: a stderr that is
// slow but not stuck.
type slowWriter struct {
	d    time.Duration
	next io.Writer
}

func (s slowWriter) Write(p []byte) (int, error) {
	time.Sleep(s.d)
	return s.next.Write(p)
}

// What saveShutdownStateForTest tears down: every coordinator newCoordinator
// built during the test (through the coordinatorBuilt hook), and every
// blockingWriter the test made.
var (
	testCoords  []*coordinator
	testWriters []*blockingWriter
)

// saveShutdownStateForTest snapshots every package variable a coordinator test
// may change and restores them at cleanup, after tearing down every
// coordinator the test built, whether or not its shutdown began. Every
// coordinator test calls it first, so no test depends on file order and
// -count=2 starts clean.
//
// The cleanup's order is chosen so nothing a coordinator still runs can race
// the restore: it unblocks the test's writers, tears each coordinator down
// (no exit from here on, no further release attempt, notifications stopped,
// done closed, a parked finish let go, the cap timer stopped), reopens the
// gate, and then waits, bounded, for each coordinator's goroutines to end.
// Only then does it put the variables back, empty the register and reset the
// gate and release admission.
func saveShutdownStateForTest(t *testing.T) {
	t.Helper()
	// Earlier tests can leave a lock registered (one whose release failed, say):
	// the coordinator tests' "exactly these locks" assertions must not see it.
	repo.ResetRegister()

	var (
		sExit       = exitFn
		sStderr     = stderrTarget
		sNewRelease = newReleaseTransport
		sStop       = stopInFlight
		sKill       = killReleaseCalls
		sHook       = afterLoopHook
		sBuilt      = coordinatorBuilt
		sCap        = shutdownCap
		sGrace      = childGrace
		sTerm       = childTerm
		sConfirm    = childConfirm
		sPause      = retryPause
		sWait       = outcomeWait
		sSettle     = repo.SettleDelay
		sSilenced   = silenced.Load()
		sCur        = curCoord
		sCoords     = testCoords
		sWriters    = testWriters
	)
	testCoords, testWriters = nil, nil
	coordinatorBuilt = func(c *coordinator) { testCoords = append(testCoords, c) }

	t.Cleanup(func() {
		for _, w := range testWriters {
			w.unblock()
		}
		all := append([]*coordinator(nil), testCoords...)
		if curCoord != nil && curCoord != sCur {
			all = append(all, curCoord)
		}
		seen := map[*coordinator]bool{}
		var torn []*coordinator
		for _, c := range all {
			if seen[c] {
				continue
			}
			seen[c] = true
			c.teardownForTest()
			torn = append(torn, c)
		}
		transport.ReopenGateForTest()
		for _, c := range torn {
			done := make(chan struct{})
			go func() { c.bg.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Errorf("a coordinator's goroutines were still running 10s after its teardown")
			}
		}

		exitFn = sExit
		stderrTarget = sStderr
		newReleaseTransport = sNewRelease
		stopInFlight = sStop
		killReleaseCalls = sKill
		afterLoopHook = sHook
		coordinatorBuilt = sBuilt
		shutdownCap, childGrace, childTerm, childConfirm, retryPause, outcomeWait = sCap, sGrace, sTerm, sConfirm, sPause, sWait
		repo.SettleDelay = sSettle
		silenced.Store(sSilenced)
		curCoord = sCur
		testCoords, testWriters = sCoords, sWriters

		repo.ResetRegister()
		transport.ReopenGateForTest()
		transport.ResetReleaseAdmissionForTest()
	})
}

// teardownForTest ends c for good: no exit and no further release attempt from
// here on, the cap timer stopped, notifications stopped, done closed (its
// goroutines return), and a finish parked on it let go.
func (c *coordinator) teardownForTest() {
	c.mu.Lock()
	c.stopped, c.torn = true, true
	c.endLocked()
	if c.capTimer != nil && c.capTimer.Stop() {
		c.bg.Done() // the cap callback will never run
	}
	c.mu.Unlock()
	c.stop()
	close(c.unpark)
}

// exitRecorder stands in for os.Exit. It records every code it is given, the
// time of the first call, and what stderr held at that moment.
type exitRecorder struct {
	mu     sync.Mutex
	codes  []int
	at     time.Time
	stderr fmt.Stringer
	atExit string
	first  chan struct{}
}

func newExitRecorder(stderr fmt.Stringer) *exitRecorder {
	return &exitRecorder{stderr: stderr, first: make(chan struct{})}
}

func (r *exitRecorder) record(code int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.codes = append(r.codes, code)
	if len(r.codes) == 1 {
		r.at = time.Now()
		if r.stderr != nil {
			r.atExit = r.stderr.String()
		}
		close(r.first)
	}
}

// wait returns the first exit code, failing the test if exitFn is not called
// within d.
func (r *exitRecorder) wait(t *testing.T, d time.Duration) int {
	t.Helper()
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-r.first:
	case <-timer.C:
		t.Fatalf("exitFn was not called within %s", d)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.codes[0]
}

func (r *exitRecorder) calls() []int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]int(nil), r.codes...)
}

func (r *exitRecorder) firstAt() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.at
}

func (r *exitRecorder) stderrAtExit() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.atExit
}

// releaseFake is the release transport the tests hand out through
// newReleaseTransport: a Fake behind a mutex (the release runs on the
// coordinator's goroutine while the test reads the remote), a count of each
// method's calls, the time the last call returned, and an optional hook that
// runs before each call, outside the mutex, so it can block or fail the call.
// A hook error is returned in place of the call.
type releaseFake struct {
	mu    sync.Mutex
	fake  *transport.Fake
	calls map[string]int
	ended time.Time
	hook  func(method string, n int) error
}

func newReleaseFake(f *transport.Fake) *releaseFake {
	return &releaseFake{fake: f, calls: map[string]int{}}
}

func (r *releaseFake) setHook(h func(method string, n int) error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hook = h
}

func (r *releaseFake) do(method string, fn func() error) error {
	r.mu.Lock()
	r.calls[method]++
	n, hook := r.calls[method], r.hook
	r.mu.Unlock()
	var err error
	if hook != nil {
		err = hook(method, n)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err == nil {
		err = fn()
	}
	r.ended = time.Now()
	return err
}

func (r *releaseFake) EnsureDir(p string) error {
	return r.do("EnsureDir", func() error { return r.fake.EnsureDir(p) })
}

func (r *releaseFake) List(p string) (ns []transport.Node, err error) {
	err = r.do("List", func() error {
		var e error
		ns, e = r.fake.List(p)
		return e
	})
	return ns, err
}

func (r *releaseFake) Stat(p string) (n transport.Node, ok bool, err error) {
	err = r.do("Stat", func() error {
		var e error
		n, ok, e = r.fake.Stat(p)
		return e
	})
	return n, ok, err
}

func (r *releaseFake) ReadTo(p, local string) error {
	return r.do("ReadTo", func() error { return r.fake.ReadTo(p, local) })
}

func (r *releaseFake) CreateExclusive(p, local string) (transport.Outcome, error) {
	out := transport.Ambiguous
	err := r.do("CreateExclusive", func() error {
		var e error
		out, e = r.fake.CreateExclusive(p, local)
		return e
	})
	return out, err
}

func (r *releaseFake) UpdateRevision(p, local string) (transport.Outcome, error) {
	out := transport.Ambiguous
	err := r.do("UpdateRevision", func() error {
		var e error
		out, e = r.fake.UpdateRevision(p, local)
		return e
	})
	return out, err
}

func (r *releaseFake) Trash(p string) (transport.Outcome, error) {
	out := transport.Ambiguous
	err := r.do("Trash", func() error {
		var e error
		out, e = r.fake.Trash(p)
		return e
	})
	return out, err
}

func (r *releaseFake) count(method string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls[method]
}

func (r *releaseFake) total() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, c := range r.calls {
		n += c
	}
	return n
}

func (r *releaseFake) lastEnded() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ended
}

func (r *releaseFake) file(p string) ([]byte, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.fake.Files[p]
	return b, ok
}

// shutdownFixture is the usual coordinator-test setup.
type shutdownFixture struct {
	stderr *syncBuffer
	exits  *exitRecorder
	fake   *transport.Fake
	rt     *releaseFake
}

// newShutdownFixture calls saveShutdownStateForTest, then installs the usual
// test seams: a recorded exit, a syncBuffer stderr, a releaseFake release
// transport, a stop that confirms at once, a kill that does nothing, no settle,
// and timings short enough that a test never waits on production values. The
// cap is long (10s): only the tests about the cap shrink it.
func newShutdownFixture(t *testing.T) *shutdownFixture {
	t.Helper()
	saveShutdownStateForTest(t)
	f := &shutdownFixture{stderr: &syncBuffer{}, fake: transport.NewFake()}
	f.rt = newReleaseFake(f.fake)
	f.exits = newExitRecorder(f.stderr)
	exitFn = f.exits.record
	stderrTarget = f.stderr
	newReleaseTransport = func(io.Writer) transport.Transport { return f.rt }
	stopInFlight = func(_, _, _ time.Duration) bool { return true }
	killReleaseCalls = func(time.Duration) {}
	shutdownCap = 10 * time.Second
	childGrace, childTerm, childConfirm = 10*time.Millisecond, 10*time.Millisecond, 10*time.Millisecond
	retryPause = 10 * time.Millisecond
	outcomeWait = 300 * time.Millisecond
	repo.SettleDelay = 0
	return f
}

// holdLock takes the lock on the fixture's remote the way loop does, so it is
// held and registered.
func (f *shutdownFixture) holdLock(t *testing.T) *repo.Lock {
	t.Helper()
	l, err := repo.AcquireLock(f.fake, sdRoot)
	if err != nil || l == nil || l.InDoubt() {
		t.Fatalf("AcquireLock = %v, %v: want a held lock", l, err)
	}
	return l
}

// createKilled makes the lock upload fail as a killed one does: Ambiguous with
// an error, and nothing landed.
type createKilled struct{ *transport.Fake }

func (createKilled) CreateExclusive(string, string) (transport.Outcome, error) {
	return transport.Ambiguous, errors.New("upload killed")
}

// lineWith returns the first line of s containing every one of parts, and its
// index among the lines; the index is -1 when no line matches.
func lineWith(s string, parts ...string) (string, int) {
	for i, l := range strings.Split(s, "\n") {
		ok := true
		for _, p := range parts {
			if !strings.Contains(l, p) {
				ok = false
				break
			}
		}
		if ok {
			return l, i
		}
	}
	return "", -1
}

// outcomeOf returns a result line's outcome in words: what follows
// "(nonce <n>): " up to the next colon. Comparing it, not a substring of the
// whole line, keeps "absent: ... already removed" from reading as removed.
func outcomeOf(line string) string {
	_, rest, ok := strings.Cut(line, "): ")
	if !ok {
		return ""
	}
	word, _, _ := strings.Cut(rest, ":")
	return word
}

// assertPrefixed checks that every line the coordinator wrote starts like warn's.
func assertPrefixed(t *testing.T, out string) {
	t.Helper()
	for _, l := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		if !strings.HasPrefix(l, "git-remote-proton: ") {
			t.Errorf("stderr line %q does not start with %q", l, "git-remote-proton: ")
		}
	}
}

// waitFor polls cond every 5ms and fails the test if it is not true within d.
func waitFor(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %s waiting for %s", d, what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// The no-lock catch line, the manual remedy, and the second-signal line, as
// the tests recognise them.
const (
	noLockMark  = "holds no remote lock"
	remedyMark  = "proton-drive filesystem trash " + sdLock
	stillMark   = "still running"
	pushMark    = "the push did not finish"
	setHeadMark = "HEAD may or may not have changed"
)

func TestShutdown_HeldLockIsRemovedAndExitIs143(t *testing.T) {
	f := newShutdownFixture(t)
	l := f.holdLock(t)

	newCoordinator().trigger(syscall.SIGTERM)

	if code := f.exits.wait(t, 5*time.Second); code != 143 {
		t.Errorf("exit code = %d, want 143 (128 + SIGTERM)", code)
	}
	if _, ok := f.rt.file(sdLock); ok {
		t.Errorf("%s is still on the remote: the coordinator must remove a held lock", sdLock)
	}
	if got := repo.RegisteredLocks(); len(got) != 0 {
		t.Errorf("register = %v, want empty: a removed lock leaves the register", got)
	}
	out := f.stderr.String()
	_, ci := lineWith(out, "received SIGTERM", "checking the remote lock")
	res, ri := lineWith(out, sdLock, l.Nonce())
	if ci < 0 || ri < 0 || ci > ri {
		t.Fatalf("stderr = %q: want the catch line, then a result line naming %s and its nonce", out, sdLock)
	}
	if outcomeOf(res) != "removed" {
		t.Errorf("result line = %q, want it to say removed", res)
	}
	assertPrefixed(t, out)
}

func TestShutdown_InDoubtLockThatNeverLandedIsAbsent(t *testing.T) {
	f := newShutdownFixture(t)
	l, err := repo.AcquireLock(createKilled{f.fake}, sdRoot)
	if err == nil || l == nil || !l.InDoubt() {
		t.Fatalf("AcquireLock = %v, %v: want an in-doubt handle and the upload's error", l, err)
	}

	newCoordinator().trigger(syscall.SIGTERM)

	if code := f.exits.wait(t, 5*time.Second); code != 143 {
		t.Errorf("exit code = %d, want 143", code)
	}
	res, _ := lineWith(f.stderr.String(), sdLock, l.Nonce())
	if outcomeOf(res) != "absent" {
		t.Errorf("result line = %q, want absent: our upload never landed", res)
	}
	if outcomeOf(res) == "removed" {
		t.Errorf("result line = %q must not claim a removal that did not happen", res)
	}
}

func TestShutdown_ForeignLockIsLeftAlone(t *testing.T) {
	f := newShutdownFixture(t)
	l := f.holdLock(t)
	foreign := []byte(`{"nonce":"someone-else","host":"h","pid":1,"acquiredAt":"2026-01-01T00:00:00Z"}`)
	f.fake.Files[sdLock] = foreign

	newCoordinator().trigger(syscall.SIGTERM)

	f.exits.wait(t, 5*time.Second)
	res, _ := lineWith(f.stderr.String(), sdLock, l.Nonce())
	if outcomeOf(res) != "foreign" {
		t.Errorf("result line = %q, want foreign", res)
	}
	if got, ok := f.rt.file(sdLock); !ok || !bytes.Equal(got, foreign) {
		t.Errorf("%s = %q (present %v), want the other holder's lock untouched", sdLock, got, ok)
	}
	if n := f.rt.count("Trash"); n != 0 {
		t.Errorf("Trash ran %d times, want 0: another process's lock is never removed", n)
	}
}

func TestShutdown_RetriesUntilRemoved(t *testing.T) {
	f := newShutdownFixture(t)
	l := f.holdLock(t)
	f.rt.setHook(func(m string, n int) error {
		if m == "Trash" && n <= 2 {
			return errors.New("database is locked")
		}
		return nil
	})

	newCoordinator().trigger(syscall.SIGTERM)

	f.exits.wait(t, 5*time.Second)
	if n := f.rt.count("Trash"); n != 3 {
		t.Errorf("Trash ran %d times, want 3: two failures, then the one that commits", n)
	}
	if _, ok := f.rt.file(sdLock); ok {
		t.Errorf("%s is still on the remote after the retries", sdLock)
	}
	res, _ := lineWith(f.stderr.String(), sdLock, l.Nonce())
	if outcomeOf(res) != "removed" {
		t.Errorf("result line = %q, want removed", res)
	}
}

func TestShutdown_UnreadableIsNotRetried(t *testing.T) {
	f := newShutdownFixture(t)
	l := f.holdLock(t)
	f.fake.Files[sdLock] = []byte("not json")

	newCoordinator().trigger(syscall.SIGTERM)

	f.exits.wait(t, 5*time.Second)
	if n := f.rt.count("ReadTo"); n != 1 {
		t.Errorf("the lock was downloaded %d times, want exactly 1: unreadable is final, not retried", n)
	}
	res, _ := lineWith(f.stderr.String(), sdLock, l.Nonce())
	if outcomeOf(res) != "unreadable" {
		t.Errorf("result line = %q, want unreadable", res)
	}
	if got, ok := f.rt.file(sdLock); !ok || string(got) != "not json" {
		t.Errorf("%s = %q (present %v), want it left as it was", sdLock, got, ok)
	}
}

func TestShutdown_CapEndsANeverSucceedingReleaseWithFailed(t *testing.T) {
	f := newShutdownFixture(t)
	l := f.holdLock(t)
	shutdownCap = 1 * time.Second
	f.rt.setHook(func(m string, _ int) error {
		if m == "Trash" {
			return errors.New("network unreachable")
		}
		return nil
	})

	start := time.Now()
	newCoordinator().trigger(syscall.SIGTERM)

	if code := f.exits.wait(t, 2*time.Second); code != 143 {
		t.Errorf("exit code = %d, want 143", code)
	}
	if el := f.exits.firstAt().Sub(start); el < 1*time.Second {
		t.Errorf("exit after %s, want no sooner than the 1s cap: a failing release is retried until then", el)
	}
	res, _ := lineWith(f.stderr.String(), sdLock, l.Nonce())
	if outcomeOf(res) != "failed" {
		t.Errorf("result line = %q, want failed", res)
	}
	for _, w := range []string{"network unreachable", remedyMark} {
		if !strings.Contains(res, w) {
			t.Errorf("result line = %q, want it to contain %q", res, w)
		}
	}
	if _, ok := f.rt.file(sdLock); !ok {
		t.Errorf("%s is gone, but every Trash failed", sdLock)
	}
}

func TestShutdown_ReleaseTransportThatCannotStartEndsFailedAtCap(t *testing.T) {
	f := newShutdownFixture(t)
	l := f.holdLock(t)
	shutdownCap = 1 * time.Second
	notFound := &exec.Error{Name: "proton-drive", Err: exec.ErrNotFound}
	f.rt.setHook(func(string, int) error { return notFound })

	newCoordinator().trigger(syscall.SIGTERM)

	if code := f.exits.wait(t, 2*time.Second); code != 143 {
		t.Errorf("exit code = %d, want 143", code)
	}
	res, _ := lineWith(f.stderr.String(), sdLock, l.Nonce())
	if outcomeOf(res) != "failed" || !strings.Contains(res, exec.ErrNotFound.Error()) {
		t.Errorf("result line = %q, want failed, naming %q", res, exec.ErrNotFound.Error())
	}
	if outcomeOf(res) == "removed" {
		t.Errorf("result line = %q must not claim a removal", res)
	}
}

func TestShutdown_BlockedStderrDoesNotDelayRelease(t *testing.T) {
	f := newShutdownFixture(t)
	f.holdLock(t)
	stderrTarget = newBlockingWriter(nil)

	newCoordinator().trigger(syscall.SIGTERM)

	f.exits.wait(t, 5*time.Second)
	if _, ok := f.rt.file(sdLock); ok {
		t.Errorf("%s is still on the remote: a stuck stderr must not stop the release", sdLock)
	}
	if gap, limit := f.exits.firstAt().Sub(f.rt.lastEnded()), outcomeWait+500*time.Millisecond; gap > limit {
		t.Errorf("exit came %s after the release ended, want at most %s", gap, limit)
	}
}

func TestShutdown_ErroringStderrDoesNotStopRelease(t *testing.T) {
	f := newShutdownFixture(t)
	f.holdLock(t)
	stderrTarget = failWriter{}

	newCoordinator().trigger(syscall.SIGTERM)

	if code := f.exits.wait(t, 5*time.Second); code != 143 {
		t.Errorf("exit code = %d, want 143", code)
	}
	if _, ok := f.rt.file(sdLock); ok {
		t.Errorf("%s is still on the remote: a stderr that fails must not stop the release", sdLock)
	}
}

func TestShutdown_NoLockMeansNoSettleAndPromptExit(t *testing.T) {
	f := newShutdownFixture(t)
	repo.SettleDelay = 5 * time.Second

	newCoordinator().trigger(os.Interrupt)

	if code := f.exits.wait(t, 500*time.Millisecond); code != 130 {
		t.Errorf("exit code = %d, want 130 (128 + SIGINT)", code)
	}
	if n := f.rt.total(); n != 0 {
		t.Errorf("%d release calls, want none: nothing was registered", n)
	}
	out := f.stderr.String()
	if _, i := lineWith(out, "received SIGINT", noLockMark); i < 0 {
		t.Errorf("stderr = %q, want the no-lock catch line", out)
	}
}

func TestShutdown_SettleRunsWhenALockIsRegistered(t *testing.T) {
	f := newShutdownFixture(t)
	f.holdLock(t)
	repo.SettleDelay = 300 * time.Millisecond

	start := time.Now()
	newCoordinator().trigger(syscall.SIGTERM)

	f.exits.wait(t, 5*time.Second)
	if el := f.exits.firstAt().Sub(start); el < 300*time.Millisecond {
		t.Errorf("exit after %s, want no sooner than the 300ms settle", el)
	}
	if _, ok := f.rt.file(sdLock); ok {
		t.Errorf("%s is still on the remote", sdLock)
	}
}

func TestShutdown_UnconfirmedExitIsNotAttempted(t *testing.T) {
	f := newShutdownFixture(t)
	l := f.holdLock(t)
	stopInFlight = func(_, _, _ time.Duration) bool { return false }
	repo.SettleDelay = 1 * time.Second

	start := time.Now()
	newCoordinator().trigger(syscall.SIGTERM)

	if code := f.exits.wait(t, 3*time.Second); code != 143 {
		t.Errorf("exit code = %d, want 143", code)
	}
	if n := f.rt.total(); n != 0 {
		t.Errorf("%d release calls, want none: the in-flight child may still be writing", n)
	}
	if el := f.exits.firstAt().Sub(start); el >= 1*time.Second {
		t.Errorf("exit after %s, want no settle wait (1s) when nothing is released", el)
	}
	res, _ := lineWith(f.stderr.String(), sdLock, l.Nonce())
	if outcomeOf(res) != "not attempted" || !strings.Contains(res, remedyMark) {
		t.Errorf("result line = %q, want not attempted, with the manual remedy", res)
	}
	if got := repo.RegisteredLocks(); len(got) != 1 || got[0] != l {
		t.Errorf("register = %v, want the lock still in it", got)
	}
	if _, ok := f.rt.file(sdLock); !ok {
		t.Errorf("%s is gone, but nothing should have been released", sdLock)
	}
}

// RED. The catch line's form is chosen from the register when the signal
// lands, the result lines from the snapshot taken later. AcquireLock's Refused
// arm takes its entry out between the two, so a catch line that said "until the
// result line below" must still be followed by one. The stopper seam stands in
// for that arm: it empties the register after the catch line and before the
// snapshot.
func TestShutdown_LockGoneBeforeTheSnapshotStillGetsAResultLine(t *testing.T) {
	f := newShutdownFixture(t)
	f.holdLock(t)
	stopInFlight = func(_, _, _ time.Duration) bool {
		repo.ResetRegister()
		return true
	}

	newCoordinator().trigger(syscall.SIGTERM)

	if code := f.exits.wait(t, 5*time.Second); code != 143 {
		t.Errorf("exit code = %d, want 143 (128 + SIGTERM)", code)
	}
	if n := f.rt.total(); n != 0 {
		t.Errorf("%d release calls, want none: the snapshot held no lock", n)
	}
	out := f.stderr.String()
	_, ci := lineWith(out, "received SIGTERM", "checking the remote lock", "Until the result line below")
	_, ri := lineWith(out, "no lock of ours remains registered; nothing to release")
	if ci < 0 {
		t.Fatalf("stderr = %q: want the lock-holding catch line", out)
	}
	if ri < 0 || ri < ci {
		t.Errorf("stderr = %q: a lock-holding catch line must be followed by a result line, "+
			"here that no lock of ours remains registered", out)
	}
	assertPrefixed(t, out)
}

func TestShutdown_SecondSignalIsNotedAndIgnored(t *testing.T) {
	f := newShutdownFixture(t)
	l := f.holdLock(t)

	c := newCoordinator()
	c.trigger(os.Interrupt)
	c.trigger(syscall.SIGTERM)

	if code := f.exits.wait(t, 5*time.Second); code != 130 {
		t.Errorf("exit code = %d, want 130: the first signal decides it", code)
	}
	out := f.stderr.String()
	_, ci := lineWith(out, "received SIGINT")
	_, si := lineWith(out, "received SIGTERM", stillMark)
	res, ri := lineWith(out, sdLock, l.Nonce())
	if ci < 0 || si < 0 {
		t.Fatalf("stderr = %q: want the catch line for SIGINT and a %q line for SIGTERM", out, stillMark)
	}
	if ci > si {
		t.Errorf("stderr = %q: the second signal's line came before the first signal's catch line", out)
	}
	if strings.Count(out, "received SIGINT") != 1 {
		t.Errorf("stderr = %q: want one catch line", out)
	}
	if ri < 0 || outcomeOf(res) != "removed" {
		t.Errorf("result line = %q, want removed: the second signal changes nothing", res)
	}
	if got := f.exits.calls(); len(got) != 1 {
		t.Errorf("exitFn calls = %v, want exactly one", got)
	}
}

func TestShutdown_ModeLines(t *testing.T) {
	for _, tc := range []struct {
		name          string
		lock          bool
		mark          func(c *coordinator)
		want, notWant []string
	}{
		{"push batch interrupted", true,
			func(c *coordinator) { c.setMode(modePush); c.batchStarted() },
			[]string{"stopping the push", pushMark}, []string{setHeadMark}},
		{"push batch done", true,
			func(c *coordinator) { c.setMode(modePush); c.batchStarted(); c.batchDone() },
			[]string{"stopping the push"}, []string{pushMark, setHeadMark}},
		{"set-head", true,
			func(c *coordinator) { c.setMode(modeSetHead) },
			[]string{"stopping --set-head", setHeadMark}, []string{pushMark}},
		{"no lock, read", false, func(*coordinator) {}, nil, nil},
		{"no lock, push batch", false,
			func(c *coordinator) { c.setMode(modePush); c.batchStarted() }, nil, nil},
		{"no lock, set-head", false,
			func(c *coordinator) { c.setMode(modeSetHead) }, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newShutdownFixture(t)
			if tc.lock {
				f.holdLock(t)
			}
			c := newCoordinator()
			tc.mark(c)
			c.trigger(syscall.SIGTERM)
			f.exits.wait(t, 5*time.Second)

			out := f.stderr.String()
			assertPrefixed(t, out)
			if !tc.lock {
				lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
				if len(lines) != 1 || !strings.Contains(lines[0], noLockMark) {
					t.Errorf("stderr = %q, want the no-lock catch line and nothing else", out)
				}
				return
			}
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("stderr = %q, want it to contain %q", out, w)
				}
			}
			for _, w := range tc.notWant {
				if strings.Contains(out, w) {
					t.Errorf("stderr = %q, want no %q", out, w)
				}
			}
		})
	}
}

func TestShutdown_SaturatedStderrStillExitsOnTime(t *testing.T) {
	f := newShutdownFixture(t)
	l := f.holdLock(t)
	bw := newBlockingWriter(f.stderr)
	stderrTarget = bw
	newReleaseTransport = func(w io.Writer) transport.Transport {
		for i := 0; i < 500; i++ {
			fmt.Fprintf(w, "gpb: noise %d\n", i)
		}
		return f.rt
	}

	c := newCoordinator()
	c.trigger(syscall.SIGTERM)
	for i := 0; i < 5; i++ {
		c.trigger(syscall.SIGTERM)
	}

	f.exits.wait(t, 5*time.Second)
	if _, ok := f.rt.file(sdLock); ok {
		t.Errorf("%s is still on the remote", sdLock)
	}
	if gap, limit := f.exits.firstAt().Sub(f.rt.lastEnded()), outcomeWait+500*time.Millisecond; gap > limit {
		t.Errorf("exit came %s after the release ended, want at most %s", gap, limit)
	}

	// Now let the writer through and see what it had kept.
	bw.unblock()
	select {
	case <-c.out.drained:
	case <-time.After(5 * time.Second):
		t.Fatal("the writer did not drain its queue within 5s of stderr unblocking")
	}
	if got := f.exits.calls(); len(got) != 1 {
		t.Errorf("exitFn calls = %v, want exactly one", got)
	}
	out := f.stderr.String()
	_, ci := lineWith(out, "received SIGTERM", "checking the remote lock")
	res, ri := lineWith(out, sdLock, l.Nonce())
	if ri < 0 || outcomeOf(res) != "removed" {
		t.Fatalf("stderr = %q: want the result line (removed) kept past a full queue", out)
	}
	if ci < 0 || ci > ri {
		t.Errorf("catch line at %d, result line at %d: want the catch line first", ci, ri)
	}
	if n := strings.Count(out, "gpb: noise"); n == 0 || n > 64 {
		t.Errorf("%d noise lines written, want between 1 and 64: ordinary lines are capped while stderr is stuck", n)
	}
}

// TestShutdown_LineQueueKeepsOrderAndResultLines pins the writer's queue on its
// own, with no goroutine timing in it: the lines are all queued before the
// writer runs. Ordinary lines come out in order, capped at 64; the result lines
// are kept past the cap and come out after everything queued before them; and
// nothing queued after the close is written.
func TestShutdown_LineQueueKeepsOrderAndResultLines(t *testing.T) {
	var buf syncBuffer
	q := newLineQueue(&buf)
	for i := 0; i < 70; i++ {
		q.add(fmt.Sprintf("ordinary %d\n", i))
	}
	q.closeWith([]string{"result 0\n", "result 1\n"})
	q.add("after close\n")

	done := make(chan struct{})
	ran := make(chan struct{})
	go func() { q.run(done); close(ran) }()
	select {
	case <-q.drained:
	case <-time.After(5 * time.Second):
		close(done)
		t.Fatal("the writer did not drain a closed queue within 5s")
	}
	<-ran

	var want strings.Builder
	for i := 0; i < 64; i++ {
		fmt.Fprintf(&want, "ordinary %d\n", i)
	}
	want.WriteString("result 0\nresult 1\n")
	if got := buf.String(); got != want.String() {
		t.Errorf("written:\n%s\nwant 64 ordinary lines in order, then both result lines, and nothing after the close", got)
	}
}

func TestShutdown_CapDuringAReleaseCallExitsExactlyOnce(t *testing.T) {
	f := newShutdownFixture(t)
	l := f.holdLock(t)
	const capD = 500 * time.Millisecond
	shutdownCap, outcomeWait = capD, 1*time.Second
	var start time.Time
	f.rt.setHook(func(m string, _ int) error {
		if m != "Trash" {
			return nil
		}
		// Running when the cap fires, and returning inside the cap's wait.
		time.Sleep(time.Until(start.Add(capD + 200*time.Millisecond)))
		return errors.New("trash call cut short")
	})

	start = time.Now()
	newCoordinator().trigger(syscall.SIGTERM)

	if code := f.exits.wait(t, 5*time.Second); code != 143 {
		t.Errorf("exit code = %d, want 143", code)
	}
	// Both the sequence and the cap reach the final step here; give a second
	// exit every chance to happen.
	time.Sleep(2 * time.Second)
	if got := f.exits.calls(); len(got) != 1 {
		t.Errorf("exitFn calls = %v, want exactly one", got)
	}
	res, _ := lineWith(f.stderr.String(), sdLock, l.Nonce())
	if outcomeOf(res) != "failed" || !strings.Contains(res, "trash call cut short") {
		t.Errorf("result line = %q, want failed with the last attempt's error", res)
	}
}

func TestShutdown_CapExitsWhenTheKillCannotFinish(t *testing.T) {
	f := newShutdownFixture(t)
	l := f.holdLock(t)
	const capD, wait = 500 * time.Millisecond, 300 * time.Millisecond
	shutdownCap, outcomeWait = capD, wait
	block := make(chan struct{})
	t.Cleanup(func() { close(block) })
	f.rt.setHook(func(string, int) error {
		<-block
		return errors.New("let go by the test's cleanup")
	})
	killReleaseCalls = func(time.Duration) { <-block }

	start := time.Now()
	newCoordinator().trigger(syscall.SIGTERM)

	limit := capD + 2*wait + 500*time.Millisecond
	if code := f.exits.wait(t, limit); code != 143 {
		t.Errorf("exit code = %d, want 143", code)
	}
	if el := f.exits.firstAt().Sub(start); el > limit {
		t.Errorf("exit after %s, want within %s", el, limit)
	}
	if got := f.exits.calls(); len(got) != 1 {
		t.Errorf("exitFn calls = %v, want exactly one", got)
	}
	res, _ := lineWith(f.stderr.String(), sdLock, l.Nonce())
	if res == "" || outcomeOf(res) == "removed" {
		t.Errorf("result line = %q, want one that does not say removed", res)
	}
}

func TestShutdown_NoLockCatchLineIsWrittenBeforeExit(t *testing.T) {
	f := newShutdownFixture(t)
	outcomeWait = 2 * time.Second
	stderrTarget = slowWriter{d: 150 * time.Millisecond, next: f.stderr}

	newCoordinator().trigger(syscall.SIGTERM)

	f.exits.wait(t, 5*time.Second)
	if got := f.exits.stderrAtExit(); !strings.Contains(got, noLockMark) {
		t.Errorf("stderr at exit = %q, want the no-lock catch line already written", got)
	}
}

func TestShutdown_FinishAfterShutdownBeganBlocksWithoutTheLock(t *testing.T) {
	f := newShutdownFixture(t)
	c := newCoordinator()
	c.trigger(syscall.SIGTERM)

	returned := make(chan int, 1)
	go func() { returned <- c.finish(0) }()
	select {
	case code := <-returned:
		t.Fatalf("finish returned %d after shutdown began: only the coordinator may end the process", code)
	case <-time.After(300 * time.Millisecond):
	}
	set := make(chan struct{})
	go func() { c.setMode(modePush); close(set) }()
	select {
	case <-set:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("setMode blocked: a parked finish must not hold the coordinator's lock")
	}
	f.exits.wait(t, 5*time.Second)
}

func TestShutdown_FinishAfterStopIgnoresALateSignal(t *testing.T) {
	f := newShutdownFixture(t)
	f.holdLock(t)
	c := newCoordinator()
	curCoord = c

	if got := c.finish(0); got != 0 {
		t.Errorf("finish(0) = %d, want 0", got)
	}
	if curCoord != nil {
		t.Errorf("curCoord is still set after a clean finish")
	}
	if got := repo.RegisteredLocks(); len(got) != 0 {
		t.Errorf("register = %v, want it cleared by a clean finish", got)
	}

	c.trigger(syscall.SIGTERM)
	time.Sleep(300 * time.Millisecond)
	if got := f.exits.calls(); len(got) != 0 {
		t.Errorf("exitFn calls = %v, want none: a signal after a clean finish does nothing", got)
	}
	if out := f.stderr.String(); out != "" {
		t.Errorf("stderr = %q, want nothing", out)
	}
	if silenced.Load() {
		t.Error("warn was silenced by a signal that came after a clean finish")
	}
}

func TestShutdown_WarnIsSilencedOnceShutdownBegins(t *testing.T) {
	f := newShutdownFixture(t)
	before := captureStderr(t, func() {
		warn(errors.New("before shutdown"))
		fmt.Fprint(quietStderr, "quiet before\n")
	})
	if !strings.Contains(before, "before shutdown") || !strings.Contains(before, "quiet before") {
		t.Fatalf("stderr = %q, want both writes before shutdown", before)
	}

	newCoordinator().trigger(syscall.SIGTERM)
	f.exits.wait(t, 5*time.Second)

	after := captureStderr(t, func() {
		warn(errors.New("after shutdown"))
		fmt.Fprint(quietStderr, "quiet after\n")
	})
	if after != "" {
		t.Errorf("stderr = %q, want nothing from warn or quietStderr once shutdown began", after)
	}
}

func TestShutdown_SecondOSSignalIsReadDuringTheRelease(t *testing.T) {
	f := newShutdownFixture(t)
	l := f.holdLock(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var enterOnce, releaseOnce sync.Once
	letGo := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(letGo)
	f.rt.setHook(func(m string, _ int) error {
		if m == "Trash" {
			enterOnce.Do(func() { close(entered) })
			<-release
		}
		return nil
	})

	c := installShutdown()
	c.sigCh <- syscall.SIGTERM
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the release did not reach Trash within 5s")
	}
	c.sigCh <- os.Interrupt
	waitFor(t, 5*time.Second, "the second signal's line", func() bool {
		_, i := lineWith(f.stderr.String(), "received SIGINT", stillMark)
		return i >= 0
	})
	letGo()

	if code := f.exits.wait(t, 5*time.Second); code != 143 {
		t.Errorf("exit code = %d, want 143", code)
	}
	out := f.stderr.String()
	_, si := lineWith(out, "received SIGINT", stillMark)
	res, ri := lineWith(out, sdLock, l.Nonce())
	if ri < 0 || si > ri || outcomeOf(res) != "removed" {
		t.Errorf("stderr = %q: want the %q line before a removed result line", out, stillMark)
	}
}

func TestRun_InProcessLeavesNoNotifyBehind(t *testing.T) {
	saveShutdownStateForTest(t)
	sigs := []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGPIPE}
	ignored := make([]bool, len(sigs))
	for i, s := range sigs {
		ignored[i] = signal.Ignored(s)
	}
	// os/signal starts one watcher goroutine on a process's first Notify and
	// never stops it. Start it now, so it is in both counts.
	warm := make(chan os.Signal, 1)
	signal.Notify(warm, os.Interrupt)
	signal.Stop(warm)

	orig := os.Args
	defer func() { os.Args = orig }()
	os.Args = []string{"git-remote-proton", "proton-v2", "proton::/shared-with-me/theirrepo"}

	before := runtime.NumGoroutine()
	for i := 0; i < 2; i++ {
		var got int
		captureStderr(t, func() { got = run() })
		if got != 1 {
			t.Fatalf("run() = %d, want 1 (the address is refused)", got)
		}
	}
	// The coordinator's two goroutines return once finish closes done: give
	// them a moment.
	deadline := time.Now().Add(2 * time.Second)
	for after := runtime.NumGoroutine(); after > before; after = runtime.NumGoroutine() {
		if time.Now().After(deadline) {
			t.Fatalf("%d goroutines after two in-process run() calls, %d before: run() left some behind",
				after, before)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if curCoord != nil {
		t.Error("curCoord is still set after run() returned")
	}
	for i, s := range sigs {
		if got := signal.Ignored(s); got != ignored[i] {
			t.Errorf("signal.Ignored(%v) = %v after run(), want %v as before", s, got, ignored[i])
		}
	}
}
