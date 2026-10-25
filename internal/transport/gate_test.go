package transport

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- The gate, the in-flight record, and the release instance ----------------
//
// S5, S7, S10, S14. These drive run against real children: the test binary
// re-exec'd as one of the gate roles in cli_test.go (hang, hang-ignore-term,
// short-sleep, hang-with-grandchild), each of which leaves markers named for
// its pid. Children run from helperExe, the copy setupHelper makes, never from
// os.Args[0]: teardownHelper then waits for any a failing test leaves behind,
// instead of one holding the test binary open (see setupHelper).
//
// Every wait here is bounded: a regression fails its test instead of hanging
// the package. Tests whose body only means anything on Unix (SIGTERM, process
// groups) are in gate_unix_test.go; processAlive is per platform beside it.

// gateTest is one gate test's harness. Its cleanup stops whatever the test left
// running, reopens the gate and release admission, waits for every run the
// test started, and only then restores the package variables those runs read.
type gateTest struct {
	t       *testing.T
	markers string // the base writeMarker names files after
	runs    sync.WaitGroup
	unblock func() // set by blockStarts; nil when starts are not blocked

	savedCall, savedDrain, savedWait time.Duration
	savedHook                        func(int)
	savedStart                       func(*exec.Cmd) error
}

// newGateTest isolates one gate test whose children play role. With private,
// the markers (and the hang roles' release file, which nothing writes: they
// are killed instead) live in this test's temp dir, so no other test's markers
// are counted. Without it they use the package's release marker, which a
// roleCLI grandchild needs: nothing writes that until teardownHelper, so the
// grandchild holds its pipe for the whole call, as in the WaitDelay tests.
//
// TempDir and Setenv come before the harness's own Cleanup, so they are undone
// after it (cleanups run last-in, first-out): no child is still running when
// its marker directory goes.
func newGateTest(t *testing.T, role string, private bool) *gateTest {
	t.Helper()
	g := &gateTest{
		t:          t,
		markers:    helperRelease,
		savedCall:  releaseCallTimeout,
		savedDrain: releaseDrain,
		savedWait:  waitDelay,
		savedHook:  testHookExited,
		savedStart: startCmd,
	}
	if private {
		g.markers = filepath.Join(t.TempDir(), "m")
		t.Setenv(helperReleaseEnv, g.markers)
	}
	t.Setenv(helperEnv, role)
	t.Cleanup(g.cleanup)
	return g
}

func (g *gateTest) cleanup() {
	if g.unblock != nil {
		g.unblock()
	}
	StopInFlight(0, 0, 2*time.Second)
	KillReleaseCalls(2 * time.Second)
	ResetReleaseAdmissionForTest()
	ReopenGateForTest()
	joined := make(chan struct{})
	go func() { g.runs.Wait(); close(joined) }()
	select {
	case <-joined:
	case <-time.After(10 * time.Second):
		g.t.Errorf("a run this test started was still running 10s into its cleanup")
	}
	releaseCallTimeout, releaseDrain, waitDelay = g.savedCall, g.savedDrain, g.savedWait
	testHookExited, startCmd = g.savedHook, g.savedStart
}

type runResult struct {
	out  string
	code int
	err  error
}

// run starts c.run on its own goroutine and returns the channel its one result
// arrives on. Set every package variable the call reads before calling this.
func (g *gateTest) run(c *CLI, args ...string) <-chan runResult {
	ch := make(chan runResult, 1)
	g.runs.Add(1)
	go func() {
		defer g.runs.Done()
		out, code, err := c.run(args...)
		ch <- runResult{out, code, err}
	}()
	return ch
}

// blockStarts makes every start wait at the startCmd seam until unblock is
// called (the cleanup calls it too). entered receives once per start that
// reached the seam, so a test knows a call is between reserving its record
// and starting its child.
func (g *gateTest) blockStarts() (entered <-chan struct{}, unblock func()) {
	ent := make(chan struct{}, 8)
	release := make(chan struct{})
	g.unblock = sync.OnceFunc(func() { close(release) })
	real := startCmd
	startCmd = func(cmd *exec.Cmd) error {
		ent <- struct{}{}
		<-release
		return real(cmd)
	}
	return ent, g.unblock
}

// countStarts counts the starts that reach the startCmd seam, without
// blocking them. A child's own marker can lag its start by a second on
// Windows (the first exec of the freshly copied helper is scanned), so "no
// child started" is read here, at the seam, as well as from the markers.
func (g *gateTest) countStarts() func() int {
	var mu sync.Mutex
	n := 0
	real := startCmd
	startCmd = func(cmd *exec.Cmd) error {
		mu.Lock()
		n++
		mu.Unlock()
		return real(cmd)
	}
	return func() int {
		mu.Lock()
		defer mu.Unlock()
		return n
	}
}

// waitEntered waits for n starts to reach the blocked seam. With n > 1, a
// start that holds the gate lock shows here first: the next call cannot even
// pass the gate to reach its own start.
func waitEntered(t *testing.T, entered <-chan struct{}, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatalf("%d of %d starts reached the startCmd seam within 5s (if some did: a stalled "+
				"start is holding the gate lock)", i, n)
		}
	}
}

// markerPids returns the pids of the children that wrote a kind marker.
func (g *gateTest) markerPids(kind string) []int {
	paths, _ := filepath.Glob(g.markers + "." + kind + ".*")
	var pids []int
	for _, p := range paths {
		if pid, err := strconv.Atoi(p[strings.LastIndexByte(p, '.')+1:]); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}

// waitMarkers polls until n children have written a kind marker.
func (g *gateTest) waitMarkers(kind string, n int) []int {
	g.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if pids := g.markerPids(kind); len(pids) >= n {
			return pids
		}
		if time.Now().After(deadline) {
			g.t.Fatalf("%d %q marker(s) after 10s, want %d", len(g.markerPids(kind)), kind, n)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// within waits up to d for a run's result.
func within(t *testing.T, ch <-chan runResult, d time.Duration, what string) runResult {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(d):
		t.Fatalf("%s: run had not returned after %s", what, d)
		return runResult{}
	}
}

// notReturned fails if a run has returned now, or returns within d.
func notReturned(t *testing.T, ch <-chan runResult, d time.Duration, what string) {
	t.Helper()
	select {
	case r := <-ch:
		t.Fatalf("%s: run returned (%q, %d, %v); want it still waiting", what, r.out, r.code, r.err)
	default:
	}
	select {
	case r := <-ch:
		t.Fatalf("%s: run returned (%q, %d, %v); want it still waiting", what, r.out, r.code, r.err)
	case <-time.After(d):
	}
}

// waitNoneInFlight polls until no gated call is in flight. With the gate shut,
// a call leaves the set only to park, so this is how a test knows a call whose
// child returned after the shut has parked; reopening any earlier would let it
// find the gate open and return its result.
func waitNoneInFlight(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for inFlightCount() != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("%d gated call(s) still in flight after 5s", inFlightCount())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// wantReopened checks a parked run's result after ReopenGateForTest.
func wantReopened(t *testing.T, r runResult) {
	t.Helper()
	if !errors.Is(r.err, errGateReopened) || r.code != -1 || r.out != "" {
		t.Errorf("a parked call must return (\"\", -1, errGateReopened) once reopened, got (%q, %d, %v)",
			r.out, r.code, r.err)
	}
}

// waitGone polls until pid is no longer alive.
func waitGone(t *testing.T, pid int, what string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for processAlive(pid) {
		if time.Now().After(deadline) {
			t.Fatalf("%s (pid %d) is still alive 2s on", what, pid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// inFlightCount and releaseRecords read the gate's state under its lock.
func inFlightCount() int {
	gateMu.Lock()
	defer gateMu.Unlock()
	return len(inFlight)
}

func releaseRecords() []*flight {
	gateMu.Lock()
	defer gateMu.Unlock()
	var fs []*flight
	for f := range releaseInFlight {
		fs = append(fs, f)
	}
	return fs
}

// captureStderr runs fn with os.Stderr pointed at a pipe and returns what was
// written there. fn runs on the calling goroutine, and nothing else in these
// tests reads os.Stderr while it runs.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		r.Close()
		got <- string(b)
	}()
	prev := os.Stderr
	os.Stderr = w
	func() {
		defer func() { os.Stderr = prev; w.Close() }()
		fn()
	}()
	return <-got
}

// RED. Once the gate is shut, run starts no child and does not return.
func TestGateParksANewCallAndStartsNoChild(t *testing.T) {
	g := newGateTest(t, roleShortSleep, true)
	starts := g.countStarts()
	ShutGate()
	ch := g.run(&CLI{Exe: helperExe})

	notReturned(t, ch, 500*time.Millisecond, "a call made after the gate shut")
	if n, pids := starts(), g.markerPids("started"); n != 0 || len(pids) != 0 {
		t.Fatalf("the gate is shut, yet a child started (%d starts, marker pids %v)", n, pids)
	}
	ReopenGateForTest()
	wantReopened(t, within(t, ch, 2*time.Second, "after the reopen"))
}

// RED. A call whose child returns after the gate shut never sees that result.
func TestGateParksACallWhoseChildReturnsAfterTheGateShut(t *testing.T) {
	g := newGateTest(t, roleHang, true)
	ch := g.run(&CLI{Exe: helperExe})
	g.waitMarkers("started", 1)

	ShutGate()
	if !StopInFlight(0, 0, 2*time.Second) {
		t.Fatal("StopInFlight could not confirm the hung child's exit")
	}
	waitNoneInFlight(t)
	notReturned(t, ch, 300*time.Millisecond, "a call whose child returned after the gate shut")
	ReopenGateForTest()
	wantReopened(t, within(t, ch, 2*time.Second, "after the reopen"))
}

// RED (compile). With nothing in flight there is nothing to wait for.
func TestStopInFlightReturnsTrueWithNothingInFlight(t *testing.T) {
	newGateTest(t, roleHang, true)
	start := time.Now()
	if !StopInFlight(time.Second, time.Second, time.Second) {
		t.Fatal("StopInFlight must return true with nothing in flight")
	}
	if el := time.Since(start); el > 200*time.Millisecond {
		t.Errorf("StopInFlight took %s with nothing in flight; it must not wait out its grace", el)
	}
}

// RED (compile). A child that exits inside the grace is not signalled: it
// finishes. The gate stays open here, so its run returns its real result.
func TestStopInFlightLetsAQuickChildFinish(t *testing.T) {
	g := newGateTest(t, roleShortSleep, true)
	ch := g.run(&CLI{Exe: helperExe})
	pid := g.waitMarkers("started", 1)[0]

	if !StopInFlight(2*time.Second, time.Second, time.Second) {
		t.Fatal("StopInFlight must confirm a child that exits inside the grace")
	}
	if _, err := os.Stat(fmt.Sprintf("%s.finished.%d", g.markers, pid)); err != nil {
		t.Errorf("the child was stopped before it finished: no finished marker (%v)", err)
	}
	r := within(t, ch, 5*time.Second, "a child that finished")
	if r.code != 0 || r.err != nil {
		t.Errorf("a child that finished must report its own success, got (%d, %v)", r.code, r.err)
	}
}

// RED. No lock is held across a child's start: a start that stalls blocks
// neither ShutGate nor KillReleaseCalls. Once the stalled starts return, each
// child is killed at once (the gate, and release admission, shut meanwhile),
// so neither hang child runs on.
func TestCLIStartDoesNotHoldTheGateLock(t *testing.T) {
	g := newGateTest(t, roleHang, true)
	entered, unblock := g.blockStarts()
	gated := g.run(&CLI{Exe: helperExe})
	release := g.run(NewReleaseCLI(helperExe, io.Discard))
	waitEntered(t, entered, 2)

	returned := make(chan struct{})
	go func() {
		ShutGate()
		KillReleaseCalls(100 * time.Millisecond)
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ShutGate and KillReleaseCalls did not return within 500ms while two starts were stalled: " +
			"a start holds the gate lock")
	}

	unblock()
	if r := within(t, release, 3*time.Second, "a release call whose start returned after admission closed"); r.code == 0 {
		t.Errorf("that release child must have been killed, got (%q, %d, %v)", r.out, r.code, r.err)
	}
	// If this times out, the child started after the gate shut was not killed.
	waitNoneInFlight(t)
	notReturned(t, gated, 0, "a call whose start returned after the gate shut")
	ReopenGateForTest()
	wantReopened(t, within(t, gated, 3*time.Second, "after the reopen"))
}

// RED. A start in progress when the gate shuts is in flight: StopInFlight
// waits for it, and the child it starts is killed at once.
func TestGateShutDuringAStartKillsTheNewChildAndCountsIt(t *testing.T) {
	g := newGateTest(t, roleHang, true)
	entered, unblock := g.blockStarts()
	ch := g.run(&CLI{Exe: helperExe})
	waitEntered(t, entered, 1)

	ShutGate()
	stopped := make(chan bool, 1)
	go func() { stopped <- StopInFlight(5*time.Second, 0, 0) }()
	select {
	case ok := <-stopped:
		t.Fatalf("StopInFlight returned %v while a start was still in progress: the record must "+
			"count from before the start", ok)
	case <-time.After(300 * time.Millisecond):
	}
	unblock()
	select {
	case ok := <-stopped:
		if !ok {
			t.Fatal("StopInFlight could not confirm the exit of the child started after the gate shut")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the child started after the gate shut was not killed when its start returned")
	}
	waitNoneInFlight(t)
	notReturned(t, ch, 200*time.Millisecond, "a call whose start returned after the gate shut")
	ReopenGateForTest()
	wantReopened(t, within(t, ch, 2*time.Second, "after the reopen"))
}

// RED. StopInFlight reports false when an exit was not confirmed: here a start
// stalls with the gate open, so there is no process to kill yet.
func TestStopInFlightReportsFalseWhenAnExitIsNotConfirmed(t *testing.T) {
	g := newGateTest(t, roleShortSleep, true)
	entered, unblock := g.blockStarts()
	ch := g.run(&CLI{Exe: helperExe})
	waitEntered(t, entered, 1)

	if StopInFlight(50*time.Millisecond, 50*time.Millisecond, 100*time.Millisecond) {
		t.Error("StopInFlight returned true with a start in progress: nothing confirmed that child's exit")
	}
	unblock()
	if r := within(t, ch, 5*time.Second, "the stalled call, once its start returned"); r.code != 0 {
		t.Errorf("with the gate open the child runs normally, got (%q, %d, %v)", r.out, r.code, r.err)
	}
}

// RED (compile). A hung child gets the grace, then SIGTERM on Unix, then a
// kill, and true means the process is gone.
func TestStopInFlightKillsAHungChild(t *testing.T) {
	g := newGateTest(t, roleHang, true)
	ch := g.run(&CLI{Exe: helperExe})
	pid := g.waitMarkers("started", 1)[0]

	if !StopInFlight(200*time.Millisecond, 200*time.Millisecond, 2*time.Second) {
		t.Fatal("StopInFlight could not confirm the hung child's exit")
	}
	if processAlive(pid) {
		t.Errorf("StopInFlight confirmed the exit, but process %d is still alive", pid)
	}
	if r := within(t, ch, 5*time.Second, "a stopped child"); r.code == 0 {
		t.Errorf("a stopped child must not report success, got (%q, %d, %v)", r.out, r.code, r.err)
	}
}

// RED. The in-flight record is a set: StopInFlight stops every child in it.
func TestStopInFlightStopsEveryInFlightChild(t *testing.T) {
	g := newGateTest(t, roleHang, true)
	c := &CLI{Exe: helperExe}
	first, second := g.run(c), g.run(c)
	pids := g.waitMarkers("started", 2)

	ShutGate()
	if !StopInFlight(100*time.Millisecond, 100*time.Millisecond, 2*time.Second) {
		t.Fatal("StopInFlight could not confirm both exits")
	}
	for _, pid := range pids {
		if processAlive(pid) {
			t.Errorf("process %d is still alive: StopInFlight did not stop every child", pid)
		}
	}
	waitNoneInFlight(t)
	notReturned(t, first, 300*time.Millisecond, "the first call")
	notReturned(t, second, 0, "the second call")
	ReopenGateForTest()
	wantReopened(t, within(t, first, 2*time.Second, "the first call, after the reopen"))
	wantReopened(t, within(t, second, 2*time.Second, "the second call, after the reopen"))
}

// RED (compile). The release instance is the gate's one exemption.
func TestReleaseCLIIsExemptFromTheGate(t *testing.T) {
	g := newGateTest(t, roleCertified, true)
	ShutGate()
	r := within(t, g.run(NewReleaseCLI(helperExe, io.Discard), "--version"), 5*time.Second,
		"a release call with the gate shut")
	if r.code != 0 || r.err != nil || !strings.Contains(r.out, CertifiedCLI) {
		t.Errorf("a release call must run with the gate shut, got (%q, %d, %v)", r.out, r.code, r.err)
	}
}

// RED (compile).
func TestNewReleaseCLIDefaultsTheExecutable(t *testing.T) {
	if got := NewReleaseCLI("", io.Discard).Exe; got != "proton-drive" {
		t.Errorf("NewReleaseCLI(\"\").Exe = %q, want \"proton-drive\"", got)
	}
	if got := NewReleaseCLI("custom-cli", io.Discard).Exe; got != "custom-cli" {
		t.Errorf("NewReleaseCLI(\"custom-cli\").Exe = %q, want it kept", got)
	}
}

// RED (compile). The production bounds are the spec's (S10), and an instance
// copies them when it is made: a later change to the variables cannot reach a
// call already running. NewCLI's instances stay gated and unbounded.
func TestNewReleaseCLICopiesItsBoundsAtConstruction(t *testing.T) {
	newGateTest(t, roleCertified, true)
	if releaseCallTimeout != 15*time.Second || releaseDrain != 2*time.Second {
		t.Errorf("production release bounds are (%s, %s), want (15s, 2s)", releaseCallTimeout, releaseDrain)
	}
	releaseCallTimeout, releaseDrain = 4*time.Second, time.Second
	var w strings.Builder
	c := NewReleaseCLI("x", &w)
	releaseCallTimeout, releaseDrain = time.Minute, time.Minute

	if !c.release || c.callTimeout != 4*time.Second || c.drain != time.Second || c.warnW != &w {
		t.Errorf("got release=%v callTimeout=%s drain=%s warnW-is-w=%v; want true, 4s, 1s, true",
			c.release, c.callTimeout, c.drain, c.warnW == &w)
	}
	if n := NewCLI(""); n.release || n.callTimeout != 0 || n.drain != 0 || n.warnW != nil {
		t.Errorf("NewCLI must stay a plain gated instance, got %+v", *n)
	}
}

// RED (compile). SetReleaseBoundsForTest refuses bounds that would kill a
// release call before it had real time: it needs 0 < drain < call/2.
func TestSetReleaseBoundsForTestRefusesBoundsThatLeaveNoTime(t *testing.T) {
	newGateTest(t, roleCertified, true)
	for _, b := range []struct{ call, drain time.Duration }{
		{time.Second, 0},
		{time.Second, -time.Millisecond},
		{time.Second, 500 * time.Millisecond},
		{time.Second, 600 * time.Millisecond},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("SetReleaseBoundsForTest(%s, %s) must panic", b.call, b.drain)
				}
			}()
			SetReleaseBoundsForTest(b.call, b.drain)
		}()
		if releaseCallTimeout != 15*time.Second || releaseDrain != 2*time.Second {
			t.Fatalf("a refused SetReleaseBoundsForTest(%s, %s) changed the bounds to (%s, %s)",
				b.call, b.drain, releaseCallTimeout, releaseDrain)
		}
	}
	SetReleaseBoundsForTest(3*time.Second, 200*time.Millisecond)
	if releaseCallTimeout != 3*time.Second || releaseDrain != 200*time.Millisecond {
		t.Errorf("SetReleaseBoundsForTest(3s, 200ms) set (%s, %s)", releaseCallTimeout, releaseDrain)
	}
}

// RED. A release call is killed at callTimeout - drain (700 ms here), so its
// whole life, drain included, stays inside callTimeout.
func TestReleaseCLIKillsACallAtItsBound(t *testing.T) {
	g := newGateTest(t, roleHang, true)
	releaseCallTimeout, releaseDrain = time.Second, 300*time.Millisecond
	c := NewReleaseCLI(helperExe, io.Discard)
	var exitedAt time.Time
	testHookExited = func(int) { exitedAt = time.Now() }

	start := time.Now()
	r := within(t, g.run(c), 1500*time.Millisecond, "a hung release call")
	if r.code == 0 {
		t.Errorf("a release call killed at its bound must not report success, got (%q, %d, %v)", r.out, r.code, r.err)
	}
	if killed := exitedAt.Sub(start); killed < 600*time.Millisecond {
		t.Errorf("the child's exit was observed %s in; the kill must wait for callTimeout - drain (700ms)", killed)
	}
}

// RED. KillReleaseCalls kills a running release child and waits for it to be
// reaped: when it returns, the process is gone.
func TestKillReleaseCallsReapsARunningReleaseChild(t *testing.T) {
	g := newGateTest(t, roleHang, true)
	ch := g.run(NewReleaseCLI(helperExe, io.Discard))
	pid := g.waitMarkers("started", 1)[0]

	start := time.Now()
	KillReleaseCalls(2 * time.Second)
	if el := time.Since(start); el >= 2*time.Second {
		t.Errorf("KillReleaseCalls ran out its 2s wait: the release child was not reaped")
	}
	if processAlive(pid) {
		t.Errorf("KillReleaseCalls returned, but the release child %d is still alive", pid)
	}
	if r := within(t, ch, 3*time.Second, "a killed release call"); r.code == 0 {
		t.Errorf("a killed release call must not report success, got (%q, %d, %v)", r.out, r.code, r.err)
	}
}

// RED. After KillReleaseCalls, a release call starts no child and returns
// errReleaseClosed, until ResetReleaseAdmissionForTest. short-sleep, not the
// brief's version-certified: certified writes no marker, so "no child
// started" could not be seen.
func TestKillReleaseCallsClosesAdmissionFirst(t *testing.T) {
	g := newGateTest(t, roleShortSleep, true)
	starts := g.countStarts()
	KillReleaseCalls(time.Second)

	r := within(t, g.run(NewReleaseCLI(helperExe, io.Discard)), 2*time.Second, "a release call after KillReleaseCalls")
	if !errors.Is(r.err, errReleaseClosed) || r.code != -1 || r.out != "" {
		t.Errorf("want (\"\", -1, errReleaseClosed), got (%q, %d, %v)", r.out, r.code, r.err)
	}
	time.Sleep(300 * time.Millisecond)
	if n, pids := starts(), g.markerPids("started"); n != 0 || len(pids) != 0 {
		t.Fatalf("release admission is closed, yet a child started (%d starts, marker pids %v)", n, pids)
	}

	ResetReleaseAdmissionForTest()
	if r := within(t, g.run(NewReleaseCLI(helperExe, io.Discard)), 5*time.Second, "after the reset"); r.code != 0 || r.err != nil {
		t.Errorf("after ResetReleaseAdmissionForTest a release call must run, got (%q, %d, %v)", r.out, r.code, r.err)
	}
}

// RED. A release call whose start is in progress while KillReleaseCalls waits
// is killed the moment its start returns: admission closed before the kill,
// so the call cannot slip past it and run unkilled.
func TestKillReleaseCallsKillsACallWhoseStartWasInProgress(t *testing.T) {
	g := newGateTest(t, roleHang, true)
	entered, unblock := g.blockStarts()
	ch := g.run(NewReleaseCLI(helperExe, io.Discard))
	waitEntered(t, entered, 1)

	took := make(chan time.Duration, 1)
	go func() {
		start := time.Now()
		KillReleaseCalls(3 * time.Second)
		took <- time.Since(start)
	}()
	// Let KillReleaseCalls take its snapshot and start waiting on the record.
	time.Sleep(200 * time.Millisecond)
	unblock()
	if r := within(t, ch, 2*time.Second, "a release call whose start returned during KillReleaseCalls"); r.code == 0 {
		t.Errorf("that release child must have been killed, got (%q, %d, %v)", r.out, r.code, r.err)
	}
	select {
	case d := <-took:
		if d >= 3*time.Second {
			t.Errorf("KillReleaseCalls ran out its wait: it did not see that child's exit")
		}
	case <-time.After(4 * time.Second):
		t.Fatal("KillReleaseCalls did not return")
	}
}

// RED. StopInFlight confirms the exit, not the drain: with a grandchild
// holding the pipe and a 10 s drain, it returns at once while run is still
// draining.
func TestStopInFlightConfirmsExitBeforeTheDrain(t *testing.T) {
	g := newGateTest(t, roleCLI, true)
	waitDelay = 10 * time.Second
	ch := g.run(&CLI{Exe: helperExe})
	deadline := time.Now().Add(5 * time.Second)
	for inFlightCount() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the call never appeared in the in-flight record")
		}
		time.Sleep(10 * time.Millisecond)
	}

	start := time.Now()
	if !StopInFlight(2*time.Second, time.Second, time.Second) {
		t.Fatal("StopInFlight could not confirm the exit of a child that exits at once")
	}
	if el := time.Since(start); el > 3*time.Second {
		t.Errorf("StopInFlight took %s: it waited on the drain, not the exit", el)
	}
	notReturned(t, ch, 0, "a call whose grandchild still holds the pipe")

	// End the drain now rather than in 10s: this test's grandchild waits on its
	// own release file. It is written here, not in a cleanup, so the temp dir
	// holding it is removed only after the grandchild has exited (run's drain
	// reaches EOF only once it has).
	os.WriteFile(g.markers, nil, 0o600)
	if r := within(t, ch, 5*time.Second, "once the grandchild was released"); r.code != 0 {
		t.Errorf("got (%q, %d, %v)", r.out, r.code, r.err)
	}
}

// RED. The release instance drains for at most releaseDrain (2 s) after the
// exit, not waitDelay's 30 s, and its warning goes to the writer it was given,
// never to os.Stderr. The drain is timed from the observed exit, not from the
// call's start: under -race a child takes about a second more to exit (the
// race runtime's exit sleep), which is not the drain's time.
func TestReleaseCLIDrainsForAtMostTwoSeconds(t *testing.T) {
	g := newGateTest(t, roleCLI, false)
	var w strings.Builder
	c := NewReleaseCLI(helperExe, &w)
	var exitedAt time.Time
	testHookExited = func(int) { exitedAt = time.Now() }

	var r runResult
	var drainedFor time.Duration
	stderr := captureStderr(t, func() {
		r = within(t, g.run(c, "--version"), releaseDrain+5*time.Second, "a release call whose grandchild holds the pipe")
		drainedFor = time.Since(exitedAt)
	})
	if drainedFor > releaseDrain+time.Second {
		t.Errorf("run returned %s after the child's exit; the release drain must end within %s", drainedFor, releaseDrain)
	}
	if !errors.Is(r.err, exec.ErrWaitDelay) || r.code != 0 {
		t.Errorf("want code 0 and exec.ErrWaitDelay, got (%d, %v)", r.code, r.err)
	}
	if !strings.Contains(w.String(), "still held its output pipe open after 2s") {
		t.Errorf("the drain warning must land in the writer given to NewReleaseCLI, got %q", w.String())
	}
	if strings.Contains(stderr, "still held its output pipe") {
		t.Errorf("the release instance wrote its warning to os.Stderr: %q", stderr)
	}
}
