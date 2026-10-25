# Lock Release on Signal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `git-remote-proton` releases its remote `.lock`, nonce-checked, when a push or `--set-head` is ended by a signal it can catch, or by a broken stdout, or when its own lock upload ends in doubt.

**Architecture:** `CLI.run` takes ownership of its child (one merged pipe, exit observed apart from the drain). A process-wide gate in `internal/transport` lets a shutdown coordinator in `cmd/git-remote-proton` park the protocol goroutine, stop the in-flight CLI child, wait a settle, and release every lock in a register kept by `internal/repo`, through its own bounded CLI instance, then exit with 128 + the signal number. The normal exit path releases the caller's own (possibly in-doubt) handle, as today.

**Tech Stack:** Go standard library only (`os/signal`, `os/exec`, `syscall`, `sync/atomic`). No new module dependencies.

**Spec:** `docs/superpowers/specs/2026-10-24-v2-lock-release-on-signal-design.md` (approved 2026-10-24). Read it first; this plan cites its decisions as S1–S18 and its sections as §n.

## Global Constraints

- `go.mod` declares `go 1.22`: use nothing newer (no `os.Process` pidfd APIs, no `x/sys`).
- The Proton CLI pin does not move: `CertifiedCLI = "cli-drive@0.9.0+75612299"`.
- The helper sets no `cmd.Env` on any child (design v6.9, F8; pinned by `TestCLIPassesItsEnvironmentThroughUnfiltered`).
- stdout is the git protocol stream only; every diagnostic goes to stderr, prefixed `git-remote-proton: `.
- Every test is hermetic: `transport.Fake`, the re-exec roles in `internal/transport/cli_test.go`, or the `testcli` shim. Nothing touches the Proton account or the network.
- Run tests with `-count=1`. The Windows full suite needs `-timeout 30m` (the `repo` package takes about 10 minutes).
- `*.go` files are checked out LF (`.gitattributes`); keep them LF, and run `gofmt -l .` before every commit.
- Values the spec pins (S5, S6, S10, S17): child grace **5 s**; Unix SIGTERM wait **3 s**; kill confirmation **2 s**; settle **5 s**; release per-call bound **15 s** including output; release drain **2 s**; retry pause **2 s**; cap **60 s**; outcome-line wait **1 s**; exit code **128 + signal number** (`os.Interrupt` = 2, so 130; SIGTERM 143; SIGHUP 129). Docs: `TimeoutStopSec=` **75 s** or more.
- No push, no tag, no release. Craig pushes. Releases follow `docs/releasing.md`.
- Another session (`fail-closed-on-a-truncat`, its own worktree) is changing the `push ` arm of `loop` in `cmd/git-remote-proton/main.go` to fail closed on a truncated batch. Whichever branch lands second rebases onto the other; do not copy its change into this branch.

## Review Focus

1. **The CLI binary is gone or unstartable when the release runs** (moved or uninstalled mid-push): the release must end *failed* with the start error in its line and stop at the cap, never hang or claim *removed*. Test: Task 5, `TestShutdown_ReleaseTransportThatCannotStartEndsFailedAtCap`.
2. **stderr is closed rather than full** (a terminal hangup makes writes fail with EIO): the release must still run to *removed*. Test: Task 5, `TestShutdown_ErroringStderrDoesNotStopRelease`.
3. **Ctrl-C on a fetch or clone**: with no lock registered there is no settle wait, and the helper exits at once with 130. Test: Task 5, `TestShutdown_NoLockMeansNoSettleAndPromptExit`.
4. **A signal while `EnsureParents` is creating parent folders** (`GPB_CREATE_PARENTS=1`), before any lock exists: prompt exit with no lock created. Test: Task 8, case E20.
5. **SIGINT and SIGTERM arriving together**: the exit code comes from the first, and the second is only noted. Test: Task 5, `TestShutdown_SecondSignalIsNotedAndIgnored`.

---

### Task 1: `CLI.run` owns its child (S13)

**Files:**
- Modify: `internal/transport/cli.go` (the `CLI` struct, `NewCLI`, `run`, and the `waitDelay` doc comment)
- Test: `internal/transport/cli_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces (unexported, used by Task 2):
  - `type CLI struct { Exe string; callTimeout time.Duration; drain time.Duration; warnW io.Writer; release bool }`. Zero values mean today's behaviour: no call timeout, `drain` = `waitDelay`, `warnW` = `os.Stderr`, not a release instance.
  - `func (c *CLI) run(args ...string) (string, int, error)`: signature unchanged.
  - `var testHookExited func(pid int)`: test seam, called once the child's exit is observed and before the drain wait begins; nil in production.

- [ ] **Step 1: Write the failing tests** in `cli_test.go`, reusing the existing roles (`roleCLI` spawns a grandchild that holds the pipe; `roleStatNotFound` writes the not-found signature to stderr):

```go
// The exit is observed before the drain ends: with a grandchild holding the
// pipe, testHookExited fires well before run returns.
func TestRunObservesExitBeforeTheDrainEnds(t *testing.T) {
	old := waitDelay
	waitDelay = 2 * time.Second
	defer func() { waitDelay = old }()
	var exitedAt time.Time
	testHookExited = func(int) { exitedAt = time.Now() }
	defer func() { testHookExited = nil }()
	// ... run roleCLI through (&CLI{Exe: helperExe}).run("--version") with the release file not yet written
	// assert: exitedAt is non-zero, and time.Since(exitedAt) at return >= 1500*time.Millisecond
	// assert: the output still contains helperVersionLine
}

// One merged stream: output written to stderr is in run's output, so Stat
// still reads the not-found signature as absence.
func TestRunMergesStderrIntoTheOutput(t *testing.T) { /* roleStatNotFound: Stat returns (_, false, nil) */ }

// More than a pipe buffer of output before exit completes and is all read.
func TestRunDrainsLargeOutputWhileTheChildRuns(t *testing.T) {
	/* new role roleBigOutput writes 200*1024 bytes of 'x' plus "\nEND\n" to stdout, then exits 0;
	   assert len(out) >= 200*1024 and strings.HasSuffix(strings.TrimSpace(out), "END") */
}
```

Add `roleBigOutput = "big-output"` to the role constants and its case to `TestMain`.

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./internal/transport/ -run 'TestRunObservesExitBeforeTheDrainEnds|TestRunMergesStderrIntoTheOutput|TestRunDrainsLargeOutputWhileTheChildRuns' -count=1 -v`
Expected: the first FAILS to compile (`testHookExited` undefined). After stubbing the variable it fails on the timing assertion, because `CombinedOutput` ties exit and drain together.

- [ ] **Step 3: Rework `run`** in `cli.go`. Create one `os.Pipe()`; set `cmd.Stdout` and `cmd.Stderr` to the **same** write end; `cmd.Start()`; close the parent's write end at once; start a goroutine that `io.Copy`s the read end into a `bytes.Buffer` and closes a `done` channel at EOF; `cmd.Wait()` (it now waits for the process only); call `testHookExited`; then wait on `done` for up to `c.drain` (or `waitDelay` when zero). If the drain times out, close the read end, keep what was read, and write today's warning to `c.warnW` (or `os.Stderr`) with today's wording. A successful exit then reads exactly as an `ErrWaitDelay` success does today. Keep the `ProcessState == nil` → code −1 rule. Rewrite the `waitDelay` comment to say the bound now applies to this explicit drain. Leave `callTimeout` and `release` unused in this task.

- [ ] **Step 4: Run the new tests and the existing WaitDelay tests**

Run: `go test ./internal/transport/ -count=1 -v -run 'TestRun|TestVersion|TestCLI|TestEnforce|TestEnsureDir'`
Expected: PASS, including `TestRunAbandonsAPipeHeldOpenByAGrandchild`, `TestVersionSucceedsThroughAWaitDelay`, `TestRunReportsNoWaitDelayWhenNothingHoldsThePipe` and `TestCLIPassesItsEnvironmentThroughUnfiltered`, all unchanged.

- [ ] **Step 5: Run the whole transport package, vet and gofmt**

Run: `go test ./internal/transport/ -count=1` then `go vet ./...` then `gofmt -l .`
Expected: `ok`, no vet output, no gofmt output.

- [ ] **Step 6: Commit**

```bash
git add internal/transport/cli.go internal/transport/cli_test.go
git commit -m "refactor(transport): run owns its child - one merged pipe, exit observed apart from the drain"
```

---

### Task 2: The gate, the in-flight record, and the release instance (S5, S7, S10, S14)

**Files:**
- Create: `internal/transport/gate.go`
- Create: `internal/transport/procgroup_unix.go` (`//go:build !windows`)
- Create: `internal/transport/procgroup_windows.go`
- Modify: `internal/transport/cli.go` (`run` registers with the gate; `NewReleaseCLI`)
- Test: `internal/transport/gate_test.go` (plus roles added to `cli_test.go`'s `TestMain`)

**Interfaces:**
- Consumes: Task 1's `CLI` fields and `run`.
- Produces:
  - `func ShutGate()`: idempotent. After it returns, `run` on any non-release `*CLI` starts no child. A caller that reaches the gate, or whose child returns after the gate shut, blocks until `ReopenGateForTest` (never, in production) and then returns `("", -1, errGateReopened)`.
  - `func StopInFlight(grace, term, confirm time.Duration) bool`: stops the one in-flight non-release child, if any. Waits `grace` for it to exit; then, on Unix, sends SIGTERM and waits `term`; then `Process.Kill()` and waits `confirm`. Returns true when nothing was in flight or the exit was confirmed (the process was reaped, not its output drained), false otherwise.
  - `func NewReleaseCLI(exe string, w io.Writer) *CLI`: `release: true` (exempt from the gate), `callTimeout: releaseCallTimeout`, `drain: releaseDrain`, `warnW: w`. It starts its children in their own process group. On Unix a call that outlives `callTimeout` is killed group and all.
  - `var releaseCallTimeout = 15 * time.Second`, `var releaseDrain = 2 * time.Second`.
  - `func ReopenGateForTest()`: test-only. Reopens the gate and releases parked callers.
  - `func SetReleaseCallTimeoutForTest(d time.Duration)`: test-only, for `cmd`'s end-to-end helper role (Task 6), which cannot reach the unexported variable.
  - Unix (`procgroup_unix.go`): `func setOwnGroup(cmd *exec.Cmd)` (`SysProcAttr{Setpgid: true}`), `func terminate(p *os.Process) bool` (SIGTERM, returns true), `func killGroup(p *os.Process) error` (`syscall.Kill(-p.Pid, syscall.SIGKILL)`).
  - Windows (`procgroup_windows.go`): `setOwnGroup` sets `CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP`; `terminate` returns false (Windows has no gentle signal for one process); `killGroup` is `p.Kill()`.

- [ ] **Step 1: Write the failing tests** in `gate_test.go`. Add three roles to `cli_test.go`: `roleHang` (writes `GPB_TEST_HELPER_RELEASE`+".started", then sleeps up to `lingerCap`), `roleHangIgnoreTerm` (the same, but `signal.Ignore(syscall.SIGTERM)` first) and `roleShortSleep` (writes ".started", sleeps 300 ms, exits 0). Each test calls `t.Cleanup(ReopenGateForTest)`.

```go
func TestGateParksANewCallAndStartsNoChild(t *testing.T)      // ShutGate(); go run(); assert no ".started" file and run has not returned after 500ms; ReopenGateForTest(); assert run returned errGateReopened
func TestGateParksACallWhoseChildReturnsAfterTheGateShut(t *testing.T) // start roleHang via run in a goroutine; wait for ".started"; ShutGate(); StopInFlight(0, 0, 2*time.Second); assert the run goroutine has not returned; Reopen; assert errGateReopened
func TestStopInFlightReturnsTrueWithNothingInFlight(t *testing.T)
func TestStopInFlightLetsAQuickChildFinish(t *testing.T)       // roleShortSleep in flight: StopInFlight(2*time.Second, time.Second, time.Second) == true, and the role's run reports exit code 0 (it finished, it was not killed)
func TestStopInFlightKillsAHungChild(t *testing.T)             // roleHang: StopInFlight(200*time.Millisecond, 200*time.Millisecond, 2*time.Second) == true, process gone
func TestStopInFlightKillsAChildThatIgnoresSIGTERM(t *testing.T) // roleHangIgnoreTerm, Unix only (t.Skip on windows): == true
func TestReleaseCLIIsExemptFromTheGate(t *testing.T)           // ShutGate(); NewReleaseCLI(helperExe, io.Discard).run("--version") with roleCertified returns code 0
func TestReleaseCLIKillsACallAtItsBound(t *testing.T)          // releaseCallTimeout = 300*time.Millisecond; roleHang returns within 3s with a non-zero code
func TestReleaseCLIDrainsForAtMostTwoSeconds(t *testing.T)     // roleCLI (grandchild holds pipe): returns within releaseDrain+1s; warning lands in the writer passed to NewReleaseCLI, not os.Stderr
func TestReleaseCLIKillsTheWholeGroupOnUnix(t *testing.T)      // Unix only: roleCLI whose grandchild hangs; after the bound, the grandchild is gone too
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./internal/transport/ -run 'TestGate|TestStopInFlight|TestReleaseCLI' -count=1 -v`
Expected: FAIL to compile (`ShutGate`, `StopInFlight`, `NewReleaseCLI`, `ReopenGateForTest` undefined).

- [ ] **Step 3: Implement `gate.go`, the two `procgroup` files, and the `run` changes.** One package mutex guards three things together, so no child can start between the check and the registration: the shut flag, the in-flight `*exec.Cmd` with its exit channel, and the reopen channel. Order inside `run`, for a non-release instance: lock → if shut, unlock and park → start → record in flight → unlock → wait → under the lock, clear the record and check the flag → if shut, park. A release instance skips the gate, calls `setOwnGroup`, and arms a timer at `callTimeout` that calls `killGroup`. `StopInFlight` reads the record under the lock and waits on its exit channel outside it.

- [ ] **Step 4: Run them to make sure they pass, then the whole package**

Run: `go test ./internal/transport/ -count=1 -v -run 'TestGate|TestStopInFlight|TestReleaseCLI'` then `go test ./internal/transport/ -count=1`
Expected: PASS; `ok`.

- [ ] **Step 5: vet, gofmt, and build for Linux**

Run: `go vet ./...`, `gofmt -l .`, `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./...`
Expected: no output from the first two; the cross-build succeeds.

- [ ] **Step 6: Commit**

```bash
git add internal/transport/
git commit -m "feat(transport): process-wide CLI gate, in-flight stop, and a bounded release instance"
```

---

### Task 3: In-doubt handles, the register, and typed release (S8, S12, S15, S17, §5.4, §5.5)

**Files:**
- Modify: `internal/repo/lock.go`
- Create: `internal/repo/register.go`
- Create: `internal/repo/settle_main_test.go` (a `TestMain` for package `repo` that sets `SettleDelay = 0`)
- Test: `internal/repo/lock_signal_test.go`; update one assertion in `internal/repo/repo_test.go`

**Interfaces:**
- Consumes: `transport.Transport` only.
- Produces:
  - `type ReleaseOutcome int` with `Removed`, `Absent`, `Foreign`, `Unreadable`, `Failed`. Its `String()` returns `"removed"`, `"absent"`, `"foreign"`, `"unreadable"` and `"failed"`. `func (o ReleaseOutcome) Final() bool` is true for all but `Failed`.
  - `func (l *Lock) Path() string`, `func (l *Lock) Nonce() string`, `func (l *Lock) InDoubt() bool`.
  - `func (l *Lock) ReleaseVia(t transport.Transport) (ReleaseOutcome, error)`: today's nonce-first logic, run on `t`. `Unreadable` returns today's unreadable message as its error; `Failed` returns the transport error, or today's "release … is unconfirmed" message. A final outcome removes `l` from the register.
  - `func (l *Lock) Release() error`: signature unchanged. If `l.InDoubt()`, call `settle()` first; then `ReleaseVia(l.t)`; return nil for `Removed`, `Absent` and `Foreign`, and the error otherwise (today's mapping).
  - `var SettleDelay = 5 * time.Second` and `var settle = func() { time.Sleep(SettleDelay) }` (a test seam).
  - `func RegisteredLocks() []*Lock` (a snapshot copy) and `func ResetRegister()`.
  - `AcquireLock(t, root) (*Lock, error)`: after staging succeeds, register the lock, then `CreateExclusive`. `Refused` with a nil error unregisters it and returns `(nil, err)`, with today's calls only. `Committed` with our nonce on read-back returns `(lock, nil)`. Every other result returns `(lock marked in doubt, today's error)` and keeps it registered. A staging failure returns `(nil, err)` and registers nothing.

- [ ] **Step 1: Write the failing tests** in `lock_signal_test.go`. Small wrapper transports in the file's own style: `errOnCreateTransport` (writes the body to the Fake, then returns `(Ambiguous, errors.New("killed"))`), `ambiguousNilTransport` (returns `(Ambiguous, nil)` without writing), `countingTransport` (counts every call). Each test calls `t.Cleanup(ResetRegister)`.

```go
func TestAcquireLockRegistersBeforeTheUpload(t *testing.T)            // a CreateExclusive hook sees len(RegisteredLocks()) == 1
func TestAcquireLockRegistersNothingWhenStagingFails(t *testing.T)    // stagedFile's os.MkdirTemp("") fails when TMPDIR, TMP and TEMP all name a missing folder (t.Setenv each); l == nil, err != nil, RegisteredLocks() is empty
func TestAcquireLockRefusedMakesTodaysCallsAndNoHandle(t *testing.T)  // a held lock exists; countingTransport: l == nil, err != nil, register empty, calls == the Stat+info+download sequence counted on the unchanged code (record the number in the test)
func TestAcquireLockInDoubtReturnsHandleAndError(t *testing.T)        // table: errOnCreate, ambiguousNil, unrecognised outcome (reuse unknownOutcomeTransport), read-back I/O error, unparseable read-back, foreign read-back: each l != nil && l.InDoubt() && err != nil, and the register holds l
func TestReleaseViaOutcomes(t *testing.T)                             // removed / absent / foreign / unreadable / failed (ambiguousTrashTransport), each from the remote state that produces it; final ones leave the register
func TestReleaseOfAnInDoubtHandleSettlesFirst(t *testing.T)           // settle seam writes our body into the Fake (the late landing); Release() == nil and .lock is gone
func TestReleaseOfAHeldHandleDoesNotSettle(t *testing.T)              // settle seam fails the test if called
```

Change one existing assertion in `repo_test.go`, `TestAcquireLockFailsClosedOnUnrecognisedOutcome`: replace `if l != nil { t.Error("no Lock may be handed back …") }` with:

```go
if l == nil || !l.InDoubt() {
	t.Error("an unrecognised outcome must hand back an in-doubt handle, never a held one")
}
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./internal/repo/ -run 'TestAcquireLock|TestReleaseVia|TestReleaseOf|TestLock|TestRelease|TestSetHead' -count=1 -v`
Expected: FAIL to compile (`ReleaseOutcome`, `RegisteredLocks`, `InDoubt` undefined).

- [ ] **Step 3: Implement `register.go`** (a mutex-guarded slice; never held across a call; a snapshot on read) and the `lock.go` changes. Make `settle_main_test.go`'s `TestMain` set `SettleDelay = 0` and then `os.Exit(m.Run())`.

- [ ] **Step 4: Make `SetHead` keep an in-doubt handle**: in `internal/repo/sethead.go`, write `lock, err := AcquireLock(t, root)`; `if lock != nil { defer … lock.Release() … }` (today's warning body); then `if err != nil { return "", err }`.

- [ ] **Step 5: Run the tests, then the whole repo package**

Run: `go test ./internal/repo/ -count=1 -run 'TestAcquireLock|TestReleaseVia|TestReleaseOf|TestLock|TestRelease|TestSetHead' -v` then `go test ./internal/repo/ -count=1 -timeout 30m`
Expected: PASS, including the unchanged `TestSetHeadRefusesWhenLockHeld`; `ok`.

- [ ] **Step 6: vet, gofmt, commit**

```bash
go vet ./... && gofmt -l .
git add internal/repo/
git commit -m "feat(repo): in-doubt lock handles, a process-wide register, and typed release outcomes"
```

---

### Task 4: The normal exit path in `loop` (S12, S16)

**Files:**
- Modify: `cmd/git-remote-proton/main.go` (`loop`)
- Test: `cmd/git-remote-proton/main_test.go`

**Interfaces:**
- Consumes: Task 3's `AcquireLock` contract (a handle may come with an error) and `repo.SettleDelay`.
- Produces: `func loop(t transport.Transport, root, gitDir string, in *bufio.Scanner, out *bufio.Writer) (code int)`. The result is named so a deferred step can adjust it; the signature is otherwise unchanged.

- [ ] **Step 1: Write the failing tests** in `main_test.go`. The package `TestMain` (`shim_test.go`) gains `repo.SettleDelay = 0` before `m.Run()`.

```go
// failWriter fails every Write.
type failWriter struct{}
func (failWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

func TestLoop_FailedProtocolWriteExitsOneAndReleases(t *testing.T) {
	ft := transport.NewFake()
	// list for-push, then EOF: the advertisement write fails
	got := loop(ft, "/my-files/r", ".", bufio.NewScanner(strings.NewReader("list for-push\n")), bufio.NewWriter(failWriter{}))
	// want got == 1; want no "/my-files/r/.lock" in ft.Files
}

func TestLoop_InDoubtAcquisitionIsReleasedOnExit(t *testing.T) {
	// a transport whose CreateExclusive for ".lock" writes the body, then returns (Ambiguous, err)
	// want got == 1, the acquisition error on stderr (captureStderr), and no .lock left
}

func TestLoop_CleanSessionStillExitsZero(t *testing.T) // the existing TestLoop_CleanEOFSucceeds shape, plus a full list-for-push session into a bytes.Buffer: 0
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./cmd/git-remote-proton/ -run 'TestLoop_FailedProtocolWrite|TestLoop_InDoubtAcquisition|TestLoop_CleanSession' -count=1 -v`
Expected: the first two FAIL (exit 0 with the lock left behind; and the in-doubt lock left behind).

- [ ] **Step 3: Change `loop`.** Name the result `code`. Replace every bare `out.Flush()` with a helper call that records the first error in a local `writeFailed`. Register a second `defer`, after the lock-release defer so that it runs before it: it flushes once more, records any error, and turns a `code` of 0 into 1 if `writeFailed`. In the `list for-push` arm: `l, err := repo.AcquireLock(t, root); if l != nil { lock = l }; if err != nil { warn(err); return 1 }`. Leave the lock-release defer's body and its comment as they are; `Release()` now settles an in-doubt handle itself.

- [ ] **Step 4: Run the package's tests**

Run: `go test ./cmd/git-remote-proton/ -count=1`
Expected: `ok`, including every existing `TestLoop_*` and `TestLoop_LockReleaseFailureIsReportedButDoesNotFailThePush`.

- [ ] **Step 5: vet, gofmt, commit**

```bash
go vet ./... && gofmt -l .
git add cmd/git-remote-proton/main.go cmd/git-remote-proton/main_test.go cmd/git-remote-proton/shim_test.go
git commit -m "fix(helper): a failed protocol write exits 1, and an in-doubt lock is released on exit"
```

---

### Task 5: The shutdown coordinator (S2–S7, S9–S11, S17, S18, §5.1–§5.3, §5.6)

**Files:**
- Create: `cmd/git-remote-proton/shutdown.go`
- Create: `cmd/git-remote-proton/signals_unix.go` (`//go:build !windows`)
- Create: `cmd/git-remote-proton/signals_windows.go`
- Modify: `cmd/git-remote-proton/main.go` (`run`, `runSetHead`, `warn`, `loop`'s two mode markers)
- Test: `cmd/git-remote-proton/shutdown_test.go`

**Interfaces:**
- Consumes: `transport.ShutGate`, `transport.StopInFlight`, `transport.NewReleaseCLI`, `transport.NewTraced`; `repo.RegisteredLocks`, `(*repo.Lock).ReleaseVia`, `repo.ReleaseOutcome`, `repo.SettleDelay`, `repo.ResetRegister`.
- Produces:
  - `type opMode int` with `modeRead`, `modePush`, `modeSetHead`.
  - `type coordinator struct{ … }` and these functions:
    - `func installShutdown() *coordinator`: called first in `run()`. It calls `signal.Notify` for `shutdownSignals()`, calls `notifySIGPIPE()`, and starts the goroutine that waits for the first signal.
    - `func (c *coordinator) setMode(m opMode)`
    - `func (c *coordinator) batchStarted()`
    - `func (c *coordinator) finish(code int) int`: called on every return from `run()`. Under the coordinator's lock it decides once. If shutdown has begun, it blocks forever (the coordinator ends the process). Otherwise it marks the coordinator stopped (a signal after this point is ignored), calls `signal.Stop` and `repo.ResetRegister()`, and returns `code`.
    - `func (c *coordinator) trigger(s os.Signal)`: the entry point the signal goroutine calls, and tests call directly.
  - Platform: `func shutdownSignals() []os.Signal`. Unix: `os.Interrupt`, `syscall.SIGTERM`, `syscall.SIGHUP`, each dropped if `signal.Ignored`. Windows: `os.Interrupt`, `syscall.SIGTERM`. Also `func notifySIGPIPE()` (Unix: `Notify` into a drained channel; Windows: no-op) and `func signalNumber(s os.Signal) int`.
  - Package seams, so tests can shrink or replace them:
    - `var exitFn = os.Exit`
    - `var stderrTarget io.Writer = os.Stderr`
    - `var newReleaseTransport = func(w io.Writer) transport.Transport { return transport.NewTraced(transport.NewReleaseCLI("", w), w) }`
    - `var stopInFlight = transport.StopInFlight`
    - `var afterLoopHook func()`, called by `run()` after `loop` returns and before `finish`; nil in production
    - `var shutdownCap, childGrace, childTerm, childConfirm, retryPause, outcomeWait = 60*time.Second, 5*time.Second, 3*time.Second, 2*time.Second, 2*time.Second, 1*time.Second`
  - `var silenced atomic.Bool`: `warn` returns at once when it is set.

- [ ] **Step 1: Write the failing tests** in `shutdown_test.go`. Each test builds a coordinator without real OS signals: shrunk timings, `exitFn` replaced by a function that records the code and closes a channel, `stderrTarget` a `*syncBuffer` (a mutex-guarded `bytes.Buffer`), and `newReleaseTransport` returning a `transport.Fake` (or a wrapper). It drives `c.trigger(syscall.SIGTERM)`, with `t.Cleanup` for `transport.ReopenGateForTest` and `repo.ResetRegister`.

```go
func TestShutdown_HeldLockIsRemovedAndExitIs143(t *testing.T)           // register a held lock on the Fake; want exit 143, .lock gone, stderr has the catch line then "removed" with the path and nonce
func TestShutdown_InDoubtLockThatNeverLandedIsAbsent(t *testing.T)     // want "absent"
func TestShutdown_ForeignLockIsLeftAlone(t *testing.T)                  // want "foreign", .lock unchanged
func TestShutdown_RetriesUntilRemoved(t *testing.T)                     // release transport fails Trash twice, then commits; want "removed"
func TestShutdown_UnreadableIsNotRetried(t *testing.T)                  // .lock body "not json"; exactly one download
func TestShutdown_CapEndsANeverSucceedingReleaseWithFailed(t *testing.T) // shutdownCap = 1s; want exit 143 within 2s, "failed", the manual remedy text
func TestShutdown_ReleaseTransportThatCannotStartEndsFailedAtCap(t *testing.T) // Review Focus 1: every call errors with exec.ErrNotFound; "failed" names it
func TestShutdown_BlockedStderrDoesNotDelayRelease(t *testing.T)        // stderrTarget blocks forever on Write; .lock gone, exit recorded within outcomeWait+500ms of the release ending
func TestShutdown_ErroringStderrDoesNotStopRelease(t *testing.T)        // Review Focus 2: stderrTarget returns an error; .lock gone
func TestShutdown_NoLockMeansNoSettleAndPromptExit(t *testing.T)        // Review Focus 3: SettleDelay = 5s, nothing registered, os.Interrupt: exit 130 within 500ms
func TestShutdown_SettleRunsWhenALockIsRegistered(t *testing.T)         // SettleDelay = 300ms: exit no sooner than 300ms
func TestShutdown_UnconfirmedExitIsNotAttempted(t *testing.T)           // stopInFlight returns false: no release call, "not attempted", entry kept, exit 143
func TestShutdown_SecondSignalIsNotedAndIgnored(t *testing.T)           // Review Focus 5: trigger SIGINT then SIGTERM; exit 130; a "still running" line
func TestShutdown_ModeLines(t *testing.T)                               // modePush + batchStarted: the interrupted-push line; modeSetHead: the HEAD line; modeRead: the no-lock line only
func TestShutdown_FinishAfterStopIgnoresALateSignal(t *testing.T)       // finish(0) returns 0; a later trigger leaves exitFn uncalled
func TestShutdown_WarnIsSilencedOnceShutdownBegins(t *testing.T)
func TestRun_InProcessLeavesNoNotifyBehind(t *testing.T)                // run() twice in-process (the TestRun_RefusesUnsupportedAddress shape); signal.Ignored state is unchanged afterwards and no coordinator goroutine is left (runtime.NumGoroutine before vs after, with a short settle)
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./cmd/git-remote-proton/ -run 'TestShutdown_|TestRun_InProcess' -count=1 -v`
Expected: FAIL to compile (`installShutdown`, `coordinator` undefined).

- [ ] **Step 3: Implement `shutdown.go` and the platform files.** `trigger` runs §5.3 in order:
  1. record the signal and arm a `time.AfterFunc(shutdownCap, …)` that sends the outcome lines, waits `outcomeWait` and calls `exitFn`;
  2. `transport.ShutGate()`, then `silenced.Store(true)`, then send the catch line;
  3. `ok := stopInFlight(childGrace, childTerm, childConfirm)`;
  4. if `!ok`, mark every registered lock *not attempted*;
  5. else, if any lock is registered, `time.Sleep(repo.SettleDelay)`, and then for each lock call `ReleaseVia(rt)` until its outcome is `Final()` or the cap, pausing `retryPause` between attempts;
  6. send the result lines, wait at most `outcomeWait`, and `exitFn(128 + signalNumber(first))`.

  Every stderr line goes out through one goroutine fed by a buffered channel, which writes to `stderrTarget` and drops writes that fail. The coordinator never waits on that goroutine, except for the final wait of at most `outcomeWait`. The release transport is built as `newReleaseTransport(asyncW)`, where `asyncW` is an `io.Writer` that feeds that same channel, so the release CLI's drain warning and `Traced`'s `gpb: downloaded` line never block either (S14). Define `syncBuffer` (a mutex-guarded `bytes.Buffer` with `Write` and `String`) in `shutdown_test.go`; Task 6 reuses it. A second signal sends the "still running" line and changes nothing else. Message content follows §5.6. Every line starts `git-remote-proton: `, and each result line names the path, the nonce and the outcome in words.

- [ ] **Step 4: Wire `main.go`.** In `run()`, set the package variable `curCoord = installShutdown()` before `dispatchUtility`, and route every `return x` through `return curCoord.finish(x)`, calling `afterLoopHook` (when non-nil) after `loop` returns. A package variable, not a parameter, because the signatures of `loop` and `dispatchUtility` are fixed by their tests. The coordinator's methods are no-ops on a nil receiver, so tests that call `loop` directly see no change. `dispatchUtility`'s `--set-head` case calls `curCoord.setMode(modeSetHead)` before `runSetHeadFn`. `loop`'s `list for-push` arm calls `curCoord.setMode(modePush)`, and its `push ` arm calls `curCoord.batchStarted()` just before `repo.Push`. `warn` checks `silenced`.

- [ ] **Step 5: Run the tests, then the package**

Run: `go test ./cmd/git-remote-proton/ -run 'TestShutdown_|TestRun_' -count=1 -v` then `go test ./cmd/git-remote-proton/ -count=1`
Expected: PASS; `ok`.

- [ ] **Step 6: Cross-build, vet, gofmt, commit**

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./... && go vet ./... && gofmt -l .
git add cmd/git-remote-proton/
git commit -m "feat(helper): release the remote lock on SIGINT, SIGTERM, SIGHUP and console Ctrl-C/Ctrl-Break"
```

---

### Task 6: The end-to-end harness, and E1 (Unix)

**Files:**
- Modify: `internal/testcli/testcli.go` (block rules and the late writer)
- Modify: `cmd/git-remote-proton/shim_test.go` (`TestMain` dispatches by base name, plus a helper role; `buildShimOnPath` stops setting `GPB_TESTCLI_ROLE`)
- Create: `cmd/git-remote-proton/signal_harness_test.go` (no build tag)
- Create: `cmd/git-remote-proton/signal_unix_test.go` (`//go:build !windows`)

**Interfaces:**
- Consumes: Task 5's seams (`afterLoopHook`, `stopInFlight`, the timing variables) and the existing `newGitRepoWithCommit` and `buildShimOnPath` helpers.
- Produces, for Tasks 7–9:
  - Shim environment: `GPB_TESTCLI_BLOCK` holds rules separated by `;`. Each rule is `verb:leaf:once|every:before|after[:delay=<ms>][:ignoreterm][:late=<ms>]`. `verb` is one of `upload`, `download`, `trash`, `info`, `create-folder` and `--version`; `leaf` is the remote leaf name, or `*`. A rule that matches writes `<GPB_TESTCLI_READY_DIR>/ready-<verb>-<leaf>-<n>`. It then blocks until killed, or for `delay` before carrying on. `before`/`after` places the block before or after the shim writes its file. `ignoreterm` makes the shim ignore SIGTERM while it blocks (Unix). `late` starts a detached copy of the shim, role `late-writer`, that writes the upload's target after that many milliseconds, whether or not the shim survives. `once` is tracked across processes with an `O_EXCL` marker file in the ready dir.
  - `TestMain`: a base name `proton-drive` (without `.exe`) runs `testcli.Run`. `GPB_TEST_HELPER_MODE=helper` stubs `probeRestoreFloor`, applies `GPB_TEST_SHRINK=1` (`shutdownCap=8s`, `childGrace=500ms`, `childTerm=500ms`, `childConfirm=2s`, `retryPause=200ms`, `repo.SettleDelay=300ms`, `releaseCallTimeout` reached through a test-only exported setter `transport.SetReleaseCallTimeoutForTest(d)`), and installs `afterLoopHook` or a `stopInFlight` that returns false when `GPB_TEST_HOLD_AFTER_LOOP` or `GPB_TEST_UNCONFIRMED_STOP` is set. Then `os.Exit(run())` with `os.Args = [exe, "origin", <address>]`.
  - Harness (`signal_harness_test.go`):
    - `type helperRun struct{ Cmd *exec.Cmd; Stdin io.WriteCloser; Stdout *bufio.Reader; Stderr *syncBuffer; Tree, Ready string }`
    - `func startHelper(t *testing.T, opts helperOpts) *helperRun`. `helperOpts` carries the address, the block rules, extra environment, `ownGroup bool`, and the git dir.
    - `func (h *helperRun) pushMain(t *testing.T)`: sends `capabilities`, `list for-push` and `push refs/heads/main:refs/heads/main` with the blank lines git sends, reading each response until its blank line, in a goroutine.
    - `func waitReady(t *testing.T, dir, prefix string, within time.Duration) string`
    - `func (h *helperRun) wait(t *testing.T, within time.Duration) (exitCode int)`: fails the test if the helper died by a signal rather than exiting.
    - `func assertNoShimLeft(t *testing.T, tree string)`: the shim records its pid in `<tree>/.pids/`, and every recorded pid must be gone.
    - `func lockPresent(tree, root string) bool`

- [ ] **Step 1: Write the failing E1 test** in `signal_unix_test.go`:

```go
func TestSignal_E1_ReleasesWhenSignalledDuringThePackUpload(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP} {
		t.Run(sig.String(), func(t *testing.T) {
			h := startHelper(t, helperOpts{Block: "upload:*.pack:once:before"})
			h.pushMain(t)
			waitReady(t, h.Ready, "ready-upload-", 20*time.Second)
			if err := h.Cmd.Process.Signal(sig); err != nil { t.Fatal(err) }
			code := h.wait(t, 20*time.Second)
			// want code == 128+int(sig); !lockPresent(h.Tree, "/my-files/r"); assertNoShimLeft
			// want h.Stderr to contain "removed" and the lock path
		})
	}
}
```

- [ ] **Step 2: Run it to make sure it fails** (in WSL or a Linux CI leg; on Windows the file does not build)

Run (WSL, from the ext4 clone per the repo's Linux test notes): `go test ./cmd/git-remote-proton/ -run TestSignal_E1 -count=1 -v`
Expected: FAIL to compile (`startHelper` undefined).

- [ ] **Step 3: Implement the shim rules, the `TestMain` dispatch and the harness.** The shim's pid file and ready files live outside the remote tree's own paths (`<tree>/.pids`, the ready dir), so they never look like remote content.

- [ ] **Step 4: Run E1 and the package**

Run (WSL): `go test ./cmd/git-remote-proton/ -run TestSignal_E1 -count=1 -v` then `go test ./cmd/git-remote-proton/ -count=1`; on Windows: `go test ./cmd/git-remote-proton/ -count=1`
Expected: PASS everywhere. The existing shim tests (`TestRunSetHead_EndToEndThroughTheScriptedShim` and the two after it) still pass with the base-name dispatch.

- [ ] **Step 5: vet, gofmt, commit**

```bash
go vet ./... && gofmt -l .
git add internal/testcli/testcli.go cmd/git-remote-proton/
git commit -m "test(helper): end-to-end signal harness over the scripted shim; E1"
```

---

### Task 7: Unix end-to-end cases for the release itself (E2–E5, E9, E10, E14–E16)

**Files:**
- Modify: `cmd/git-remote-proton/signal_unix_test.go`

**Interfaces:**
- Consumes: Task 6's harness and shim rules.
- Produces: tests only.

- [ ] **Step 1: Write the tests.** Each one asserts the exit code, the `.lock` state, `assertNoShimLeft`, and the result line (E5's outcome is its own).

```go
func TestSignal_E2_ProcessGroupSignalLikeTimeout(t *testing.T)    // ownGroup: syscall.Kill(-pgid, SIGTERM); 143, lock gone
func TestSignal_E3_IdleAfterListForPush(t *testing.T)             // send only capabilities + list for-push, read the advertisement, signal; 143, lock gone
func TestSignal_E4_DuringTheLockUpload(t *testing.T)              // subtests: "upload:.lock:once:before" -> "absent", lock absent; "upload:.lock:once:after" -> "removed", lock gone
func TestSignal_E5_GitDiesMidBatch(t *testing.T)                  // "upload:*.pack:once:before:delay=1500"; after ready, close Stdin and Stdout's read end; exit 1, lock gone, not killed by a signal
func TestSignal_E9_SecondSignalDuringASlowRelease(t *testing.T)   // "trash:.lock:once:before:delay=1500"; SIGTERM, wait for ready-trash, SIGINT; 143, lock gone, a "still running" line
func TestSignal_E10_ReleaseThatNeverEnds(t *testing.T)            // "trash:.lock:every:before"; exit 143 within shutdownCap+5s, "failed", lock present
func TestSignal_E14_ForeignLockDuringAcquire(t *testing.T)        // seed another nonce's .lock; block "download:.lock:once:before" (the refusal's read-back); signal; "foreign", lock unchanged byte for byte
func TestSignal_E15_LateLandingLockUpload(t *testing.T)           // subtests: "upload:.lock:once:before:late=100" -> "removed", lock gone; ":late=2000" -> "absent" naming the nonce, then lock present after 2.5s
func TestSignal_E16_ShimIgnoresSIGTERM(t *testing.T)              // "upload:*.pack:once:before:ignoreterm"; helper-only SIGTERM; the shim is killed after childGrace+childTerm, then "removed"
```

- [ ] **Step 2: Run them** (WSL)

Run: `go test ./cmd/git-remote-proton/ -run 'TestSignal_E(2|3|4|5|9|10|14|15|16)_' -count=1 -v`
Expected: PASS. Any failure here is a defect in Tasks 1–5: fix it there, test-first, and say so in the commit message.

- [ ] **Step 3: Run the whole package twice in a row** (catches leaked processes and stale ready files)

Run: `go test ./cmd/git-remote-proton/ -count=2`
Expected: `ok`.

- [ ] **Step 4: Commit**

```bash
git add cmd/git-remote-proton/signal_unix_test.go
git commit -m "test(helper): signal cases E2-E5, E9, E10, E14-E16"
```

---

### Task 8: Unix end-to-end cases at the boundaries (E6–E8, E11–E13, E17–E20)

**Files:**
- Modify: `cmd/git-remote-proton/signal_unix_test.go`

**Interfaces:**
- Consumes: Task 6's harness, including `GPB_TEST_HOLD_AFTER_LOOP` and `GPB_TEST_UNCONFIRMED_STOP`.
- Produces: tests only.

- [ ] **Step 1: Write the tests.**

```go
func TestSignal_E6_InheritedIgnoredSIGHUP(t *testing.T)        // start the helper through `sh -c 'trap "" HUP; exec "$0" "$@"'`; SIGHUP mid-upload (with "delay=1500") has no effect; the push completes; exit 0; lock gone
func TestSignal_E7_SetHeadDuringItsHEADUpload(t *testing.T)    // --set-head argv; "upload:HEAD:once:before"; 143; the HEAD line; lock gone
func TestSignal_E8_FetchDuringADownload(t *testing.T)          // seed a pushed repo (pushViaLoop into the shim tree), fetch via the protocol; "download:*.pack:once:before"; 143 within 3s; no .lock ever created; assertNoShimLeft
func TestSignal_E11_DuringTheMarkerUpload(t *testing.T)        // empty root; "upload:gpb-remote.json:once:before"; 143; no .lock
func TestSignal_E12_DuringTheVersionCheck(t *testing.T)        // "--version:*:once:before"; 143; child gone
func TestSignal_E13_RightAfterLoopReturns(t *testing.T)        // GPB_TEST_HOLD_AFTER_LOOP=<file>: the hook writes ready-afterloop and waits for <file>.go; signal, then create <file>.go; exit 143, the no-lock catch line, lock gone (removed by the normal defer)
func TestSignal_E17_StderrNeverRead(t *testing.T)              // leave Stderr's pipe unread and the ReadTo trace on; SIGTERM mid-upload; lock gone; exit within about 1s of the lock disappearing, well inside shutdownCap
func TestSignal_E18_NoSignalLockUploadErrorsThenLandsLate(t *testing.T) // "upload:.lock:once:before:late=100" plus the shim exiting 1 instead of blocking (rule flag "fail"); no signal; exit 1; lock gone after the settle
func TestSignal_E19_UnconfirmedChildExit(t *testing.T)         // GPB_TEST_UNCONFIRMED_STOP=1; "upload:*.pack:once:before"; 143; "not attempted" with the manual remedy; lock present
func TestSignal_E20_DuringParentFolderCreation(t *testing.T)   // Review Focus 4: GPB_CREATE_PARENTS=1, a missing parent, "create-folder:*:once:before"; 143; no .lock; no marker
```

Add the `fail` rule flag to the shim (exit 1 with no summary JSON after the `late` writer has started), and list it in Task 6's rule grammar in the shim's doc comment.

- [ ] **Step 2: Run them** (WSL)

Run: `go test ./cmd/git-remote-proton/ -run 'TestSignal_E(6|7|8|11|12|13|17|18|19|20)_' -count=1 -v`
Expected: PASS. A failure is fixed in the task that owns the code, test-first.

- [ ] **Step 3: The full Unix package, twice**

Run: `go test ./cmd/git-remote-proton/ -count=2`
Expected: `ok`.

- [ ] **Step 4: Commit**

```bash
git add internal/testcli/testcli.go cmd/git-remote-proton/signal_unix_test.go
git commit -m "test(helper): signal cases E6-E8, E11-E13, E17-E20"
```

---

### Task 9: Windows end-to-end cases (W1, W2)

**Files:**
- Create: `cmd/git-remote-proton/signal_windows_test.go`

**Interfaces:**
- Consumes: Task 6's harness. `helperOpts.ownGroup` on Windows means `CREATE_NEW_PROCESS_GROUP`.
- Produces: tests only, plus `func ctrlBreak(t *testing.T, pid int) error`: `syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent").Call(1, uintptr(pid))`, the method measured in spec P4.

- [ ] **Step 1: Write the tests.**

```go
func TestSignal_W1_CtrlBreakDuringThePackUpload(t *testing.T) {
	// ownGroup; "upload:*.pack:once:before"; after ready, ctrlBreak(h.Cmd.Process.Pid)
	// If ctrlBreak returns an error (no console attached), t.Skipf("GenerateConsoleCtrlEvent failed: %v (no console?)", err)
	// want exit 130, lock gone, "removed", assertNoShimLeft
}

func TestSignal_W2_SecondCtrlBreakDuringTheRelease(t *testing.T) {
	// ownGroup; "upload:*.pack:once:before" and "trash:.lock:once:before:delay=1500"
	// first ctrlBreak, wait for ready-trash, second ctrlBreak to the helper's group
	// want exit 130, lock gone (the release child, in its own group, did not receive it), a "still running" line
}
```

- [ ] **Step 2: Run them** (Windows, from the PowerShell tool and from Git Bash; P4 measured both)

Run: `go test ./cmd/git-remote-proton/ -run TestSignal_W -count=1 -v`
Expected: PASS, not SKIP, in both shells. Record which shells were used in the commit message.

- [ ] **Step 3: The Windows package**

Run: `go test ./cmd/git-remote-proton/ -count=1`
Expected: `ok`.

- [ ] **Step 4: Commit**

```bash
git add cmd/git-remote-proton/signal_windows_test.go
git commit -m "test(helper): Windows Ctrl-Break cases W1, W2"
```

---

### Task 10: Documentation (spec §7)

**Files:**
- Modify: `docs/v2-remote-helper-design.md` (the status line; "Concurrency posture" item 5; the error table's "Local disk full, permissions, cancellation, timeout" row; two revision entries)
- Modify: `README.md` ("Honest limits")
- Modify: `docs/research/gates/brief-checklist.md` (rule 9)
- Modify: `CHANGELOG.md` (an "Unreleased" section, if the file has none)
- Modify: `docs/README.md` (the plan link)

**Interfaces:**
- Consumes: the shipped behaviour of Tasks 1–9.
- Produces: docs only.

- [ ] **Step 1: Design doc.** Add **v6.10**, the CLI child ownership of S13: one merged pipe drained while the child runs, exit observed apart from the drain, the in-flight record. Add **v6.11**, signal-driven release, with S1–S12 and S14–S18 and the options not taken, each marked "agent's recommendation, approved by Craig 2026-10-24". Rewrite item 5 so it says what releases the lock and what cannot (§2 in short); keep item 6 unchanged. In the error-table row, name the signal behaviour and the exit code 128 + n. Update the status line's version.

- [ ] **Step 2: README "Honest limits".** On Linux, SIGTERM, SIGINT and SIGHUP release the lock, whether from `kill`, `timeout` or systemd (`KillMode=control-group`, `TimeoutStopSec=` of 75 s or more). On Windows, only Ctrl-C and Ctrl-Break in a console window do. Git Bash's `kill`, `Stop-Process`, Task Scheduler and SIGKILL still leave the lock, and today's manual remedy stays documented. Name the "no result line" state (§5.6).

- [ ] **Step 3: brief-checklist rule 9.** Add one sentence: the rule stays in force after this change, because a harness kill through Git Bash on Windows is uncatchable (spec P1).

- [ ] **Step 4: CHANGELOG.** One entry: what releases the lock now, per platform; that on Windows only console Ctrl-C/Ctrl-Break are covered and S1's kind of kill still leaks; and that a session whose protocol write failed now exits 1.

- [ ] **Step 5: The Linux unattended spec.** Run `git log --oneline -1 main -- docs/superpowers/specs/2026-10-24-v2-linux-unattended-design.md`. If it prints a commit, that spec has landed: update its README guidance and its "a stalled push leaks the lock" row with §5.8's rules. Those are the reader outside the time limit (`timeout 2h git push … 2>&1 | logger` with `pipefail`), `KillMode=control-group`, `TimeoutStopSec=` of 75 s or more, and that `--kill-after` cannot bound the helper. Then rewrite its sentence "That stays true until the helper releases its lock when killed (U6's follow-up)." If it prints nothing, add the same list to the session notes for whoever lands that spec second, and say so in the task's report.

- [ ] **Step 6: Check the README pins still hold**

Run (PowerShell): `Invoke-Pester tests/ReadmePins.Tests.ps1`
Expected: all its tests pass (2/2 on the current tree).

- [ ] **Step 7: Commit**

```bash
git add docs/ README.md CHANGELOG.md
git commit -m "docs: the helper now releases its remote lock on catchable signals (design v6.10, v6.11)"
```

---

### Task 11: Verification across both platforms

**Files:** none changed, unless a step finds a defect (fix it in the owning task's files, test-first).

**Interfaces:** none.

- [ ] **Step 1: Windows full suite**

Run (PowerShell or Git Bash): `go test ./... -count=1 -timeout 30m`, then check the exit code on its own
Expected: every package `ok`; exit code 0.

- [ ] **Step 2: Windows vet and format**

Run: `go vet ./...` then `gofmt -l .`
Expected: no output from either.

- [ ] **Step 3: Linux full suite in the WSL distro** (ext4 clone, interop off; see the repo's Linux test notes)

Run: `GOTOOLCHAIN=go1.27.0 PATH=~/opt/git-2.55.0/bin:$PATH go test ./... -count=1`
Expected: every package `ok`, with the Unix `TestSignal_E*` cases among them.

- [ ] **Step 4: Race detector on the three changed packages (WSL)**

Run: `GOTOOLCHAIN=go1.27.0 go test -race -count=1 ./internal/transport/ ./internal/repo/ ./cmd/git-remote-proton/`
Expected: `ok` for all three, with no `WARNING: DATA RACE`.

- [ ] **Step 5: The Go 1.22 floor**

Run (WSL, the distro's go1.22.2): `GOTOOLCHAIN=local /usr/lib/go-1.22/bin/go build ./...` and `GOTOOLCHAIN=local /usr/lib/go-1.22/bin/go vet ./...`
Expected: both succeed with no output.

- [ ] **Step 6: Pester suite** (the module is untouched, but releasing.md's clean-tree checks assume it is green)

Run (PowerShell): `Invoke-Pester tests/`
Expected: all pass.

- [ ] **Step 7: Report for Craig**, in the session, not committed: each step's result; the CI follow-up, which is that once Craig pushes the branch and opens a PR, the `windows-latest` log must show W1 and W2 as PASS, not SKIP (spec §6), and a SKIP there goes to Craig; and that the release itself follows `docs/releasing.md`, with both formal live gates and each gate brief adding the kill-mid-push check of spec §8.
