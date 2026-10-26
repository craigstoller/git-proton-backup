# Lock Release on Signal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `git-remote-proton` releases its remote `.lock`, nonce-checked, when a push or `--set-head` is ended by a signal it can catch, or by a broken stdout, or when its own lock upload ends in doubt.

**Architecture:** `CLI.run` takes ownership of its child (one merged pipe, exit observed apart from the drain). A process-wide gate in `internal/transport` lets a shutdown coordinator in `cmd/git-remote-proton` park the protocol goroutine, stop the in-flight CLI child, wait a settle, and release every lock in a register kept by `internal/repo`, through its own bounded CLI instance, then exit with 128 + the signal number. The normal exit path releases the caller's own (possibly in-doubt) handle, as today.

**What the machinery buys, stated so a reviewer can weigh it:** the register, the settle wait and the retry loop exist for the case where our own last request lands after the CLI child dies (a lock upload, or a ref write). How long the server can take to apply such a request is unmeasured (spec U7). The approved spec accepts that premise and its residual (S4, S17, §5.7). This plan implements it, and does not reopen it.

**Tech Stack:** Go standard library only (`os/signal`, `os/exec`, `syscall`, `sync/atomic`). No new module dependencies.

**Spec:** `docs/superpowers/specs/2026-10-24-v2-lock-release-on-signal-design.md` (approved 2026-10-24). Read it first; this plan cites its decisions as S1–S18 and its sections as §n.

**Status:** approved by Craig, 2026-10-25 11:13 PT ("Approved, subagent-driven"). Three full-panel review rounds before approval; round 3's fixes were not re-reviewed (the loop's cap), so the per-task reviews are their first check.

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
- A separate fix, `690ebf3` on branch `claude/keen-rhodes-dfce31`, changes the `push ` arm of `loop` in `cmd/git-remote-proton/main.go` to fail closed on a truncated batch. It conflicts with Task 4 and Task 5 inside `loop` itself. Before Task 4, check whether it has landed: `git merge-base --is-ancestor 690ebf3 main` (exit 0 means yes), or, for a squash merge, `git log --oneline main --grep "refuse a push batch"`. If it has landed, bring `main` into this branch first. Otherwise this branch lands first, and that one rebases. Never copy its change across by hand.

## Review Focus

1. **The CLI binary is gone or unstartable when the release runs** (moved or uninstalled mid-push): the release must end *failed* with the start error in its line and stop at the cap, never hang or claim *removed*. Test: Task 5, `TestShutdown_ReleaseTransportThatCannotStartEndsFailedAtCap`.
2. **stderr is closed rather than full** (a terminal hangup makes writes fail with EIO): the release must still run to *removed*. Test: Task 5, `TestShutdown_ErroringStderrDoesNotStopRelease`.
3. **Ctrl-C on a fetch or clone**: with no lock registered there is no settle wait. A terminal Ctrl-C reaches the CLI child too, so it dies and the helper exits within about a second, at production timings. (A `kill` to the helper alone still runs S5's stop sequence, up to about 10 s.) Tests: Task 5, `TestShutdown_NoLockMeansNoSettleAndPromptExit`; Task 8, E8's production-timing subtest.
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
  - `var testHookExited func(pid int)`: test seam, called once the child's exit is observed and before the drain wait begins; nil in production. Tests set it before a call and clear it after the call returns, never during one, so `-race` sees no conflict.
  - The error contract, unchanged from today and pinned by `TestRunAbandonsAPipeHeldOpenByAGrandchild`: when the child exits 0 but the drain runs out, `run` returns the captured output, code 0 and `exec.ErrWaitDelay`. When the drain completes, it returns a nil error for an exit 0, and an `*exec.ExitError` otherwise. A start failure returns code −1 and the start error.

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
Expected: the first FAILS to compile (`testHookExited` undefined). After stubbing the variable it fails on the timing assertion, because `CombinedOutput` ties exit and drain together. The other two pass today: they are guards, there so the rework cannot break the merged stream or large outputs, and not new coverage.

- [ ] **Step 3: Rework `run`** in `cli.go`:
  1. Create one `os.Pipe()`, and set `cmd.Stdout` and `cmd.Stderr` to the **same** write end.
  2. `cmd.Start()`. If it fails, close **both** pipe ends.
  3. Close the parent's write end at once.
  4. Start a reader goroutine that reads the read end in chunks and appends each chunk to a buffer **under a mutex**, then closes a `done` channel at EOF or on any read error.
  5. `cmd.Wait()` now waits for the process only. Then call `testHookExited`.
  6. Wait on `done` for up to `c.drain` (or `waitDelay` when that is zero). If the drain runs out: close the read end **from a separate goroutine, never waited on**, because on Windows `Close` can block behind a read that is still in progress. Take the output as a copy of the buffer made under the mutex. For an exit 0 only, which is today's condition, return `exec.ErrWaitDelay` per the contract above and write today's warning, with today's wording, to `c.warnW` (or `os.Stderr`). A non-zero exit whose drain runs out stays silent, as it is today.

  Keep the `ProcessState == nil` → code −1 rule. Rewrite the `waitDelay` comment to say the bound now applies to this explicit drain. Leave `callTimeout` and `release` unused in this task.

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
  - `func StopInFlight(grace, term, confirm time.Duration) bool`: stops **every** in-flight non-release child. The record is a set, not a single slot. The protocol goroutine runs one call at a time today, but the set makes that an observation rather than an assumption. For each child: wait `grace` for it to exit; then, on Unix, send SIGTERM and wait `term`; then `Process.Kill()` and wait `confirm`. Returns true when nothing was in flight or every exit was confirmed (the process reaped, not its output drained), false otherwise.
  - `func KillReleaseCalls(within time.Duration)`: under the package mutex, first closes release admission. From then on, `run` on a release instance returns `("", -1, errReleaseClosed)` without starting a child. Then it kills every in-flight release child (group and all on Unix) and waits up to `within` for each to be reaped. The coordinator calls it at the cap (Task 5). Closing admission first means a release call that was about to start cannot slip past the kill.
  - `func NewReleaseCLI(exe string, w io.Writer) *CLI`: an empty `exe` becomes `"proton-drive"`, as in `NewCLI`. It sets `release: true` (exempt from the gate), `warnW: w`, and copies `callTimeout: releaseCallTimeout` and `drain: releaseDrain` **at construction**, so a test that changes the variables later does not race a running call. It starts its children in their own process group. A call's whole life stays within `callTimeout`: the kill fires at `callTimeout − drain`, then the drain gets at most `drain`. On Unix the kill takes the whole group. The kill timer is stopped the moment the child's exit is observed. Because `Timer.Stop` cannot recall a callback that is already running, the callback itself takes the package mutex and does nothing when the child's record shows its exit was observed. Only that check makes it impossible to signal a reaped or reused process group.
  - `var releaseCallTimeout = 15 * time.Second`, `var releaseDrain = 2 * time.Second`.
  - `func ReopenGateForTest()`: test-only. Reopens the gate and releases parked callers.
  - `func SetReleaseBoundsForTest(call, drain time.Duration)`: test-only, for `cmd`'s end-to-end helper role (Task 6), which cannot reach the unexported variables. It panics unless `0 < drain` and `drain < call/2`, so a release call always gets real time before its kill.
  - `func ResetReleaseAdmissionForTest()`: test-only. Reopens release admission after a `KillReleaseCalls`.
  - Unix (`procgroup_unix.go`): `func setOwnGroup(cmd *exec.Cmd)` (`SysProcAttr{Setpgid: true}`), `func terminate(p *os.Process) bool` (SIGTERM, returns true), `func killGroup(p *os.Process) error` (`syscall.Kill(-p.Pid, syscall.SIGKILL)`).
  - Windows (`procgroup_windows.go`): `setOwnGroup` sets `CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP`; `terminate` returns false (Windows has no gentle signal for one process); `killGroup` is `p.Kill()`.

- [ ] **Step 1: Write the failing tests** in `gate_test.go`. Add four roles to `cli_test.go`:
  - `roleHang`: writes `GPB_TEST_HELPER_RELEASE`+".started", then sleeps up to `lingerCap`;
  - `roleHangIgnoreTerm`: the same, but `signal.Ignore(syscall.SIGTERM)` first;
  - `roleShortSleep`: writes ".started", sleeps 300 ms, writes ".finished", exits 0;
  - `roleHangWithGrandchild`: starts a `roleHang` grandchild that writes ".grandchild" with its pid, then hangs itself, so the parent is still alive when the bound fires.

  Every role writes its markers with its pid in the name (`.started.<pid>`), so a test waiting for two children counts two files.

  Each test calls `t.Cleanup` to reopen the gate and release admission, and to restore every package variable it changed (`releaseCallTimeout`, `releaseDrain`, `waitDelay`, `testHookExited`).

```go
func TestGateParksANewCallAndStartsNoChild(t *testing.T)      // ShutGate(); go run(); assert no ".started" file and run has not returned after 500ms; ReopenGateForTest(); assert run returned errGateReopened
func TestGateParksACallWhoseChildReturnsAfterTheGateShut(t *testing.T) // start roleHang via run in a goroutine; wait for ".started"; ShutGate(); StopInFlight(0, 0, 2*time.Second); assert the run goroutine has not returned; Reopen; assert errGateReopened
func TestStopInFlightReturnsTrueWithNothingInFlight(t *testing.T)
func TestStopInFlightLetsAQuickChildFinish(t *testing.T)       // the gate stays OPEN in this test: roleShortSleep in flight; StopInFlight(2*time.Second, time.Second, time.Second) == true; its ".finished.<pid>" marker exists (it finished, it was not killed), and its run returns code 0
func TestCLIStartDoesNotHoldTheGateLock(t *testing.T)          // a test seam makes the start step block; meanwhile ShutGate() and KillReleaseCalls(100ms) both return within 500ms
func TestStopInFlightKillsAHungChild(t *testing.T)             // roleHang: StopInFlight(200*time.Millisecond, 200*time.Millisecond, 2*time.Second) == true, process gone
func TestStopInFlightKillsAChildThatIgnoresSIGTERM(t *testing.T) // roleHangIgnoreTerm, Unix only (t.Skip on windows): == true
func TestStopInFlightStopsEveryInFlightChild(t *testing.T)     // two goroutines each run roleHang; both ".started"; StopInFlight(100ms, 100ms, 2s) == true; both runs return after ReopenGateForTest
func TestReleaseCLIIsExemptFromTheGate(t *testing.T)           // ShutGate(); NewReleaseCLI(helperExe, io.Discard).run("--version") with roleCertified returns code 0
func TestNewReleaseCLIDefaultsTheExecutable(t *testing.T)      // NewReleaseCLI("", io.Discard).Exe == "proton-drive"
func TestReleaseCLIKillsACallAtItsBound(t *testing.T)          // releaseCallTimeout = 1*time.Second, releaseDrain = 300*time.Millisecond, set before NewReleaseCLI; roleHang returns within 1500ms with a non-zero code
func TestKillReleaseCallsReapsARunningReleaseChild(t *testing.T) // a release run of roleHang in a goroutine; after ".started", KillReleaseCalls(2*time.Second); the child process is gone and the run returns
func TestKillReleaseCallsClosesAdmissionFirst(t *testing.T)    // KillReleaseCalls(time.Second); then a release run of roleCertified returns errReleaseClosed, and no ".started" file appears
func TestStopInFlightConfirmsExitBeforeTheDrain(t *testing.T)  // roleCLI (it exits; its grandchild holds the pipe) with waitDelay = 10s: StopInFlight(2s, time.Second, time.Second) == true within 3s, long before the drain ends
func TestReleaseCLIDrainsForAtMostTwoSeconds(t *testing.T)     // roleCLI (grandchild holds pipe): returns within releaseDrain+1s; warning lands in the writer passed to NewReleaseCLI, not os.Stderr
func TestReleaseCLIKillsTheWholeGroupOnUnix(t *testing.T)      // Unix only: roleHangWithGrandchild, releaseCallTimeout = 1s, releaseDrain = 200ms; after the bound, the parent and the grandchild named in ".grandchild" are both gone
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./internal/transport/ -run 'TestGate|TestStopInFlight|TestReleaseCLI' -count=1 -v`
Expected: FAIL to compile (`ShutGate`, `StopInFlight`, `NewReleaseCLI`, `ReopenGateForTest` undefined).

- [ ] **Step 3: Implement `gate.go`, the two `procgroup` files, and the `run` changes.** One package mutex guards, together: the shut flag, the set of in-flight `*exec.Cmd`s with their exit channels (a release child is tracked in a second set), and the reopen channel. That way no child can start between the check and the registration. **No lock is ever held across `cmd.Start()`**, so a start that stalls cannot block `ShutGate` or `KillReleaseCalls`. Start goes through a package seam, `var startCmd = (*exec.Cmd).Start`, for `TestCLIStartDoesNotHoldTheGateLock`. Order inside `run`, for a non-release instance: lock → if shut, unlock and park; otherwise reserve a slot in the in-flight set → unlock → `startCmd(cmd)` → lock → fill the slot with the process and its exit channel; if the gate shut meanwhile, kill the new child and treat it as in flight (`StopInFlight` waits on it like any other) → unlock → `cmd.Wait()` → **close the record's exit channel at once, where `testHookExited` fires and before the drain begins** (`StopInFlight` and `KillReleaseCalls` wait on that channel with windows of a few seconds, while a drain may run 30 s) → drain → under the lock, remove the record and check the flag → if shut, close the pipe's read end from a background goroutine, so its reader cannot leak, and park. A release instance skips the gate but follows the same pattern for admission: lock → if admission is closed, return `errReleaseClosed`; otherwise reserve a slot → unlock → `setOwnGroup`, `startCmd` → lock → fill the slot (kill the child if admission closed meanwhile) → unlock. It then arms its kill timer as described in Interfaces. `StopInFlight` and `KillReleaseCalls` copy the set under the lock and wait on the exit channels outside it. A parked caller, including a `repo` release running on a gated instance after the gate shut, stays parked: in production the coordinator ends the process, and in tests `ReopenGateForTest` returns it `errGateReopened`.

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
  - `var SettleDelay = 5 * time.Second`, `var settle = func() { time.Sleep(SettleDelay) }` (a test seam), and `func Settle() { settle() }`, the one settle the coordinator calls too, so a test that replaces `settle` affects both paths.
  - `var stageFile = stagedFile`: a test seam for `AcquireLock`'s staging step.
  - `func RegisteredLocks() []*Lock` (a snapshot copy) and `func ResetRegister()`.
  - `AcquireLock(t, root) (*Lock, error)`: after staging succeeds, register the lock, then `CreateExclusive`. `Refused` with a nil error unregisters it and returns `(nil, err)`, with today's calls only. `Committed` with our nonce on read-back returns `(lock, nil)`. Every other result returns `(lock marked in doubt, today's error)` and keeps it registered. A staging failure returns `(nil, err)` and registers nothing.

- [ ] **Step 1: Write the failing tests** in `lock_signal_test.go`. Small wrapper transports in the file's own style: `errOnCreateTransport` (writes the body to the Fake, then returns `(Ambiguous, errors.New("killed"))`), `ambiguousNilTransport` (returns `(Ambiguous, nil)` without writing), `countingTransport` (counts every call). Each test calls `t.Cleanup(ResetRegister)`.

```go
func TestAcquireLockRegistersBeforeTheUpload(t *testing.T)            // a CreateExclusive hook sees len(RegisteredLocks()) == 1
func TestAcquireLockRegistersNothingWhenStagingFails(t *testing.T)    // stageFile replaced by one that returns an error; l == nil, err != nil, RegisteredLocks() is empty
func TestAcquireLockRefusedMakesTodaysCallsAndNoHandle(t *testing.T)  // a held lock exists; countingTransport: l == nil, err != nil, register empty, calls == the Stat+info+download sequence counted on the unchanged code (record the number in the test)
func TestAcquireLockInDoubtReturnsHandleAndError(t *testing.T)        // table: errOnCreate, ambiguousNil, unrecognised outcome (reuse unknownOutcomeTransport), read-back I/O error, unparseable read-back, foreign read-back: each l != nil && l.InDoubt() && err != nil, and the register holds l
func TestReleaseViaOutcomes(t *testing.T)                             // removed / absent / foreign / unreadable / failed, each from the remote state that produces it; failed three ways: ambiguousTrashTransport, a Trash returning (Refused, nil), and a Stat that errors; final ones leave the register, failed ones keep it
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

- [ ] **Step 3: Implement `register.go`** (a mutex-guarded slice; never held across a call; a snapshot on read) and the `lock.go` changes. Package `repo` has no `TestMain` today: verified 2026-10-24, `grep -n "func TestMain" internal/repo/*_test.go` prints nothing. Re-run that grep before creating `settle_main_test.go`. Its `TestMain` sets `SettleDelay = 0` and then `os.Exit(m.Run())`.

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

- [ ] **Step 3: Change `loop`.** Name the result `code`. Replace every bare `out.Flush()` with a helper call that records the first error in a local `writeFailed`. Register a second `defer` at the top of `loop`, beside the lock-release defer and before the command loop, so that it covers every `return`. It flushes once more, records any error, and turns a `code` of 0 into 1 if `writeFailed`. The order of the two defers does not matter: the release defer never touches `code`. In the `list for-push` arm: `l, err := repo.AcquireLock(t, root); if l != nil { lock = l }; if err != nil { warn(err); return 1 }`. Leave the lock-release defer's body and its comment as they are; `Release()` now settles an in-doubt handle itself. One behaviour is deliberately unchanged: a second `list for-push` in one session (git does not send one) overwrites `lock`, so the normal path releases only the last handle, as today. The register holds both, so a signal would release both. *[Superseded 2026-10-25, from the shipped code (`internal/repo/lock.go`, the `Refused` arm), found in the docs review: a second `list for-push` in one process is refused, because our own lock already holds `.lock`. The refusal unregisters the second entry and `lock` keeps the first handle, so the register never holds two and nothing is overwritten.]* *[Superseded again 2026-10-25, docs review round 2: the note above is wrong in one case. A second acquisition registers before its upload and stays registered if that upload ends in doubt, so the register can hold two; and Task 4's `if l != nil { lock = l }` then let the in-doubt handle replace the held one, stranding the held lock. Fixed in f87739a and 92cb1f8: `loop` adopts a second handle only when its acquisition succeeded (the first `.lock` is then already gone), and keeps the held one when it ends in doubt.]*

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
- Consumes: `transport.ShutGate`, `transport.StopInFlight`, `transport.KillReleaseCalls`, `transport.NewReleaseCLI`, `transport.NewTraced`; `repo.RegisteredLocks`, `(*repo.Lock).ReleaseVia`, `repo.ReleaseOutcome`, `repo.Settle`, `repo.ResetRegister`.
- Produces:
  - `type opMode int` with `modeRead`, `modePush`, `modeSetHead`.
  - `type coordinator struct{ … }` and these functions. Every method is a no-op on a nil receiver.
    - `func installShutdown() *coordinator`: called first in `run()`. It makes the signal channel (capacity 4) and calls `signal.Notify` for `shutdownSignals()`. It makes the SIGPIPE channel (capacity 1), calls `notifySIGPIPE(pipeCh)` and starts its drainer. It starts the signal goroutine. That goroutine **loops**: each signal it receives goes to `c.trigger(s)`, which returns at once, so a second signal is still read while the release runs. Both goroutines also select on the coordinator's `done` channel.
    - `func (c *coordinator) setMode(m opMode)`
    - `func (c *coordinator) batchStarted()` and `func (c *coordinator) batchDone()`: `loop` calls the second once a batch's status lines are flushed.
    - `func (c *coordinator) finish(code int) int`: called on every return from `run()`. It decides under the coordinator's lock and releases the lock before acting. If shutdown has begun, it blocks forever outside the lock (the coordinator ends the process). Otherwise it marks the coordinator stopped (a signal after this point is ignored), calls `signal.Stop` on both channels, closes `done` (both goroutines return), calls `repo.ResetRegister()`, sets `curCoord = nil`, and returns `code`.
    - `func (c *coordinator) trigger(s os.Signal)`: the entry point the signal goroutine calls, and tests call directly. On the first signal it starts the shutdown sequence on a **new goroutine** and returns. On any later one it queues the "still running" line and returns.
    - `var quietStderr io.Writer`: wraps `os.Stderr` and drops writes once `silenced` is set. `run()` passes it to `dispatchUtility` in place of `os.Stderr`, so `runSetHead`'s own writes to its `stderr` argument are silenced too. **Not** silenced, and accepted as the spec's §5.1 residual: writes straight to `os.Stderr` from `repo` (among them `SetHead`'s release-failure note in its defer), from `warnRestoreFloor`, and from `headSkipNote`. Each prints at most a line, and the protocol goroutine parks at its next CLI call.
    - The coordinator alone writes to `stderrTarget`. `warn` and `quietStderr` write to `os.Stderr`, as today. So the coordinator tests capture the coordinator's lines through `stderrTarget`, and nothing else reaches it.
  - Platform: `func shutdownSignals() []os.Signal`. Unix: `os.Interrupt`, `syscall.SIGTERM`, `syscall.SIGHUP`, each dropped if `signal.Ignored`. Windows: `os.Interrupt`, `syscall.SIGTERM`. Go delivers both Ctrl-C and Ctrl-Break as `os.Interrupt`, per `runtime/os_windows.go` and spec P4, so they exit 130. Also `func notifySIGPIPE(ch chan os.Signal)` (Unix: `Notify(ch, syscall.SIGPIPE)`; Windows: no-op) and `func signalNumber(s os.Signal) int` (`os.Interrupt` 2; otherwise the `syscall.Signal` value, so SIGTERM is 15 on both platforms: `syscall/types_windows.go` defines `SIGTERM = Signal(0xf)`, read in go1.27.0).
  - `func saveShutdownStateForTest(t *testing.T)`, in `shutdown_test.go`. It snapshots every package variable a coordinator test may change: `exitFn`, `stderrTarget`, `newReleaseTransport`, `stopInFlight`, `afterLoopHook`, the six timing variables, `repo.SettleDelay`, `silenced` and `curCoord`. Its `t.Cleanup` tears down every coordinator the test built, whether or not shutdown began: `signal.Stop` on its signal and SIGPIPE channels, close its `done`, and release any `finish` parked on it (through a test-only channel `finish` also selects on). It then restores all the variables, stops any armed cap timer, calls `repo.ResetRegister()`, `transport.ReopenGateForTest()` and `transport.ResetReleaseAdmissionForTest()`, and closes any blocking test writer the test created. Every coordinator test calls it first. That way no test depends on file order, and `-count=2` sees a clean start.
  - Package seams, so tests can shrink or replace them:
    - `var exitFn = os.Exit`
    - `var stderrTarget io.Writer = os.Stderr`
    - `var newReleaseTransport = func(w io.Writer) transport.Transport { return transport.NewTraced(transport.NewReleaseCLI("", w), w) }`
    - `var stopInFlight = transport.StopInFlight` and `var killReleaseCalls = transport.KillReleaseCalls`
    - `var afterLoopHook func()`, called by `run()` after `loop` returns and before `finish`; nil in production
    - `var shutdownCap, childGrace, childTerm, childConfirm, retryPause, outcomeWait = 60*time.Second, 5*time.Second, 3*time.Second, 2*time.Second, 2*time.Second, 1*time.Second`
  - `var silenced atomic.Bool`: `warn` returns at once when it is set.

- [ ] **Step 1: Write the failing tests** in `shutdown_test.go`. Each test builds a coordinator without real OS signals: shrunk timings; `exitFn` replaced by a function that records each code it is given; `stderrTarget` a `*syncBuffer` (a mutex-guarded `bytes.Buffer`); and `newReleaseTransport` returning a `transport.Fake` (or a wrapper). It drives `c.trigger(syscall.SIGTERM)` after `saveShutdownStateForTest(t)`, and waits on the recorded exit. Assertions about line order compare the coordinator's own lines only; `repo`'s direct stderr notes are outside that order (spec §5.1).

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
func TestShutdown_ModeLines(t *testing.T)                               // modePush + batchStarted, a lock registered: the interrupted-push line; after batchDone: no interrupted-push line; modeSetHead: the HEAD line *[superseded 2026-10-25: only once HEAD's upload has started, Q5, commit 53c8e65]*; nothing registered, any mode: the no-lock line only
func TestShutdown_SaturatedStderrStillExitsOnTime(t *testing.T)         // stderrTarget blocks forever; 500 lines queued through asyncW and 5 repeated triggers; the lock still ends removed and exitFn is called once, within outcomeWait+500ms of the release ending
func TestShutdown_CapDuringAReleaseCallExitsExactlyOnce(t *testing.T)   // release Trash blocks past shutdownCap; exitFn called exactly once, with 143; the result line carries the last attempt's error
func TestShutdown_CapExitsWhenTheKillCannotFinish(t *testing.T)         // a registered lock; the release transport's call blocks and the KillReleaseCalls seam blocks too; exitFn is still called once, within shutdownCap + 2*outcomeWait + 500ms; the result line does not say "removed"
func TestShutdown_NoLockCatchLineIsWrittenBeforeExit(t *testing.T)      // nothing registered: by the time exitFn is called, stderrTarget already holds the no-lock catch line
func TestShutdown_FinishAfterShutdownBeganBlocksWithoutTheLock(t *testing.T) // trigger first; finish(0) in a goroutine does not return within 300ms, while setMode from another goroutine still returns at once
func TestShutdown_FinishAfterStopIgnoresALateSignal(t *testing.T)       // finish(0) returns 0 and curCoord is nil; a later trigger leaves exitFn uncalled
func TestShutdown_WarnIsSilencedOnceShutdownBegins(t *testing.T)
func TestShutdown_SecondOSSignalIsReadDuringTheRelease(t *testing.T)    // through the signal goroutine itself (send on its channel), not trigger: a slow release, a second signal mid-release, and the "still running" line appears before the result line
func TestRun_InProcessLeavesNoNotifyBehind(t *testing.T)                // run() twice in-process (the TestRun_RefusesUnsupportedAddress shape); signal.Ignored state is unchanged afterwards, and no goroutine was added by those two calls (runtime.NumGoroutine just before them vs just after, with a short settle; earlier tests' goroutines are in both counts)
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./cmd/git-remote-proton/ -run 'TestShutdown_|TestRun_InProcess' -count=1 -v`
Expected: FAIL to compile (`installShutdown`, `coordinator` undefined).

- [ ] **Step 3: Implement `shutdown.go` and the platform files.** `trigger` runs §5.3 in order:
  1. record the signal, and arm a `time.AfterFunc(shutdownCap, …)`. Its callback starts `killReleaseCalls(outcomeWait)` on its own goroutine, waits for it for at most `outcomeWait`, then runs the same final step as item 6 whether or not the kill finished. The cap's exit therefore depends on no transport lock and no cleanup completing. Worst-case latency from the first signal to exit is `shutdownCap` + `outcomeWait` (the kill wait) + `outcomeWait` (the result lines) = **62 s**, inside the 75 s `TimeoutStopSec=` the docs require;
  2. `transport.ShutGate()`, then `silenced.Store(true)`, then queue the catch line (the no-lock form when nothing is registered);
  3. `ok := stopInFlight(childGrace, childTerm, childConfirm)`;
  4. if `!ok`, mark every registered lock *not attempted*;
  5. else, take **one snapshot** of the register; the result lines report exactly these locks. If it holds any, `repo.Settle()`, and then for each lock call `ReleaseVia(rt)` until its outcome is `Final()`, pausing `retryPause` between attempts. Before each attempt, check one flag, "the final step has started", set under the coordinator's lock; stop when it is set. Elapsed time is not the test. Record each lock's latest outcome and error in mutex-guarded coordinator state, so the cap callback can report them;
  6. the final step, run once through a `sync.Once`: set that flag, stop the cap timer, build the result lines from the recorded state, write them, wait until the write finishes or `outcomeWait` passes (whichever comes first), and call `exitFn(128 + signalNumber(first))`.

  **stderr.** Every coordinator line goes through **one** writer goroutine, started by `trigger`, that drains a mutex-guarded FIFO queue in order and drops any line whose write fails. Appending never blocks. An ordinary line (the catch line, "still running", and anything from `asyncW`) is dropped when the queue already holds 64 lines. A result line is always appended, past that limit, so a full queue cannot drop it. Item 6 appends the result lines, marks the queue closed, and waits for the writer to drain it, or for `outcomeWait`, whichever comes first, before `exitFn`. The catch line therefore always comes out before the result lines, and the no-lock case still writes its catch line before exiting. Only a stderr that is blocked or dead can lose lines, and then nobody could read them anyway. Task 10 documents that state. The release transport is built as `newReleaseTransport(asyncW)`, where `asyncW` is an `io.Writer` over the same non-blocking channel, so the release CLI's drain warning and `Traced`'s `gpb: downloaded` line never block either (S14). Define `syncBuffer` (a mutex-guarded `bytes.Buffer` with `Write` and `String`) in `shutdown_test.go`; Task 6 reuses it. A second signal sends the "still running" line and changes nothing else. Message content follows §5.6. Every line starts `git-remote-proton: `, and each result line names the path, the nonce and the outcome in words.

- [ ] **Step 4: Wire `main.go`.** In `run()`, set the package variable `curCoord = installShutdown()` before `dispatchUtility`, and route every `return x` through `return curCoord.finish(x)`, calling `afterLoopHook` (when non-nil) after `loop` returns. A package variable, not a parameter, because the signatures of `loop` and `dispatchUtility` are fixed by their tests. The coordinator's methods are no-ops on a nil receiver, so tests that call `loop` directly see no change. `dispatchUtility`'s `--set-head` case calls `curCoord.setMode(modeSetHead)` before `runSetHeadFn`. `loop`'s `list for-push` arm calls `curCoord.setMode(modePush)`. Its `push ` arm calls `curCoord.batchStarted()` just before `repo.Push`, and `curCoord.batchDone()` after the status lines are flushed. `warn` checks `silenced`. `afterLoopHook` runs on `run()`'s goroutine; the coordinator runs on the signal goroutine, so a signal that arrives while the hook waits is handled independently (E13 relies on that).

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
  - Shim environment: `GPB_TESTCLI_BLOCK` holds rules separated by `;`. Each rule is `verb:leaf:once|every:before|after[:delay=<ms>][:ignoreterm][:late=<ms>][:fail]`.
    - `verb` is one of `upload`, `download`, `trash`, `info`, `create-folder` and `--version`.
    - `leaf` is the name the remote node has, or will have, or `*` (a `*.pack` suffix match is allowed). For `upload` that is `filepath.Base` of the **local** file argument, because the CLI names the remote node after the local basename (probe C11, as `runUpload` already does). For `create-folder` it is the `name` argument. For the other verbs it is `leafOf` of the remote path argument.
    - File names the shim makes from a rule (ready files, `once` markers) use the **matched** name with every character outside `[A-Za-z0-9._-]` replaced by `_`, plus the rule's index. They never contain the rule's own `*` or `:`, which Windows does not allow in file names.
    - A rule that matches writes `<GPB_TESTCLI_READY_DIR>/ready-<verb>-<leaf>-<n>`, then blocks until killed, or for `delay` before carrying on. `before` blocks before the shim looks at the target at all: before its existence check, its skip decision or any write. So an `upload` that would be skipped as already existing (`Refused`) still blocks first. `after` blocks once the shim has finished acting on the target, written or skipped, and before it prints its result.
    - `ignoreterm` makes the shim ignore SIGTERM while it blocks (Unix).
    - `late` makes `runUpload` first copy the staged file's bytes to `<ready>/late-<n>.data`, because `AcquireLock` deletes its staging folder as soon as the upload call returns. It then starts a copy of the shim, role `late-writer`, given the shim's own pid. The late writer first waits until that pid has gone (polling `processAlive` every 20 ms), then sleeps that many milliseconds, then writes the copy to the upload's target. So the delay runs from the shim's death, as a late server landing would, and E15's 100 ms case lands inside the settle wait, not during the stop sequence. The late writer is **detached**: its own process group on Unix (`Setpgid`), `CREATE_NEW_PROCESS_GROUP` on Windows, and not waited on, so a kill of the shim's group cannot reach it. It records its pid only as `<tree>/.pids/late-<pid>`, never as a plain shim pid.
    - `fail` makes the shim exit 1 with no summary JSON, after any `late` writer has started.
    - `once` is tracked across processes with an `O_EXCL` marker file in the ready dir.
    - Every shim run records its pid as `<tree>/.pids/<pid>` and removes it on a normal exit.
  - `TestMain` dispatches by the executable's base name, with any `.exe` stripped, **before** looking at any variable, because the helper sets no `cmd.Env` and its children inherit everything:
    - `proton-drive` runs `testcli.Run`;
    - `git-remote-proton-testhelper` (a copy of the test binary made by `startHelper`) runs the helper role;
    - anything else is the test runner. It sets `repo.SettleDelay = 0` (Task 4's line), stubs the probe, and runs `m.Run()`.

    The helper role stubs `probeRestoreFloor`. With `GPB_TEST_SHRINK=1` it sets `shutdownCap=8s`, `childGrace=500ms`, `childTerm=500ms`, `childConfirm=2s`, `retryPause=200ms`, `outcomeWait=300ms` and `repo.SettleDelay=300ms`, and calls `transport.SetReleaseBoundsForTest(3*time.Second, 200*time.Millisecond)`. That gives a release call 2.8 s before its kill, room for E9's and W2's 1.5 s `delay`, and stays well under `shutdownCap`. It installs `afterLoopHook` when `GPB_TEST_HOLD_AFTER_LOOP` is set, and a `stopInFlight` that returns false when `GPB_TEST_UNCONFIRMED_STOP` is set. Then it runs `os.Exit(run())`, with `os.Args = [exe, "origin", <address>]` for a push or fetch, or `[exe, "--set-head", <address>, <branch>]`. `buildShimOnPath` stops setting `GPB_TESTCLI_ROLE`; the base name replaces it.
  - Harness (`signal_harness_test.go`):
    - `type helperRun struct{ Cmd *exec.Cmd; Stdin io.WriteCloser; StdoutR *os.File; Stdout *bufio.Reader; Stderr *syncBuffer; StderrR, StderrW *os.File; Tree, Ready string }`. `StdoutR` is the raw read end of the helper's stdout pipe, made with `os.Pipe` and owned by the test, which closes it to break the helper's stdout (E5). `Stdout` is a `bufio.Reader` over it. `StderrR` and `StderrW` are set only when `opts.RawStderr`. The helper's stderr is then a raw pipe; the test holds the read end and never reads it, and keeps its own copy of the write end, so it can fill the pipe itself (E17). `Stderr` is nil in that case. Otherwise `Stderr` collects stderr.
    - `func startHelper(t *testing.T, opts helperOpts) *helperRun`. `helperOpts` carries the address, the block rules, extra environment, `ownGroup bool`, `RawStderr bool`, and the git dir. It copies the test binary to a temp dir as `git-remote-proton-testhelper`(`.exe`), and puts the shim dir first on PATH. It sets the helper's working directory to the git repo, and `GIT_DIR` to `opts.GitDir` in **the helper's** environment: `run()` reads `GIT_DIR` itself. That is the test setting its own child's environment, not the helper setting `cmd.Env`. It registers `t.Cleanup` to kill the helper if it is still running.
    - `func (h *helperRun) pushMain(t *testing.T)`: sends `capabilities`, `list for-push` and `push refs/heads/main:refs/heads/main` with the blank lines git sends, reading each response until its blank line, in a goroutine.
    - `func waitReady(t *testing.T, dir, prefix string, within time.Duration) string`
    - `func (h *helperRun) wait(t *testing.T, within time.Duration) (exitCode int)`: fails the test if the helper died by a signal rather than exiting.
    - `func assertNoShimLeft(t *testing.T, tree string)`: polls every 100 ms. Every pid recorded under `<tree>/.pids/`, apart from `late-*` ones, must read as gone within 5 s. Polling absorbs the moment a killed shim is still a zombie, which `kill(pid, 0)` reports as alive until it is reaped. A `late-*` pid must be gone within its delay plus 5 s.
    - `func processAlive(pid int) bool`, one per platform in the test files. Unix: `syscall.Kill(pid, 0)` returns nil (an exited shim is reparented and reaped, so it reads as gone). Windows: `OpenProcess` plus `GetExitCodeProcess` reporting `STILL_ACTIVE` (259), through `syscall.NewLazyDLL("kernel32.dll")`. An `OpenProcess` that fails with `ERROR_INVALID_PARAMETER` (87, no such process) reads as gone. Any other failure reads as alive, so the assertion can only fail safe.
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
func TestSignal_E5_GitDiesMidBatch(t *testing.T)                  // "upload:*.pack:once:before:delay=1500"; after ready, close h.Stdin and h.StdoutR; exit 1, lock gone, not killed by a signal
func TestSignal_E9_SecondSignalDuringASlowRelease(t *testing.T)   // "trash:.lock:once:before:delay=1500"; SIGTERM, wait for ready-trash, SIGINT; 143, lock gone, a "still running" line
func TestSignal_E10_ReleaseThatNeverEnds(t *testing.T)            // "trash:.lock:every:before"; exit 143 within shutdownCap+5s, "failed", lock present
func TestSignal_E14_ForeignLockDuringAcquire(t *testing.T)        // seed another nonce's .lock; block "upload:.lock:once:before" (our own lock upload, still registered, killed in flight); signal; "foreign", lock unchanged byte for byte. (A signal after a Refused acquisition has returned gives the no-lock line instead: the entry is already gone.)
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
func TestSignal_E8_FetchDuringADownload(t *testing.T)          // seed a pushed repo (pushViaLoop into the shim tree), fetch via the protocol; "download:*.pack:once:before"; 143 within 3s; no .lock ever created; assertNoShimLeft. Subtest "group signal, production timings": GPB_TEST_SHRINK unset, ownGroup, SIGINT to the group: the shim dies of it, and the helper exits 130 within 2s
func TestSignal_E11_DuringTheMarkerUpload(t *testing.T)        // empty root; "upload:gpb-remote.json:once:before"; 143; no .lock
func TestSignal_E12_DuringTheVersionCheck(t *testing.T)        // "--version:*:once:before"; 143; child gone
func TestSignal_E13_RightAfterLoopReturns(t *testing.T)        // GPB_TEST_HOLD_AFTER_LOOP=<file>: the hook writes ready-afterloop and waits for <file>.go; signal; wait until the no-lock catch line appears on stderr; only then create <file>.go, ignoring any error (the helper may already be exiting); exit 143, lock gone (removed by the normal defer)
func TestSignal_E17_StderrNeverRead(t *testing.T)              // RawStderr; "upload:*.pack:once:before"; after ready, a test goroutine writes 128 KB into h.StderrW, filling the helper's stderr pipe (nobody reads StderrR; the goroutine blocks and is freed at cleanup); then SIGTERM; lock gone; exit within about 1s of the lock disappearing, well inside shutdownCap
func TestSignal_E18_NoSignalLockUploadErrorsThenLandsLate(t *testing.T) // "upload:.lock:once:before:late=100" plus the shim exiting 1 instead of blocking (rule flag "fail"); no signal; exit 1; lock gone after the settle
func TestSignal_E19_UnconfirmedChildExit(t *testing.T)         // GPB_TEST_UNCONFIRMED_STOP=1; "upload:*.pack:once:before"; 143; "not attempted" with the manual remedy; lock present
func TestSignal_E20_DuringParentFolderCreation(t *testing.T)   // Review Focus 4: GPB_CREATE_PARENTS=1, a missing parent, "create-folder:*:once:before"; 143; no .lock; no marker
```

E18 uses the `fail` rule flag from Task 6's grammar.

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
	// Note in the test's comment: the blocked shim shares the helper's group, so it dies of the
	// same Ctrl-Break, as it would of one typed in the console. W1 therefore proves the Windows
	// release path, not that the helper can stop a child that ignored the event. That stop path
	// is Task 2's TestStopInFlightKillsAHungChild, which also runs on Windows.
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

- [ ] **Step 2: README "Honest limits".** Describe a **bounded, nonce-checked attempt**, not a guarantee:
  - On Linux, SIGTERM, SIGINT and SIGHUP start that attempt, whether from `kill`, `timeout` or systemd (`KillMode=control-group`, `TimeoutStopSec=` of 75 s or more: the helper exits within 62 s of the first signal at worst). On Windows only Ctrl-C and Ctrl-Break in a console window do.
  - Name each outcome the result line can report: *removed*; *absent*, which is what the remote showed at that moment, so a very late upload could still land afterwards; *foreign*, left alone; *unreadable*; *failed*, and *not attempted*. The last two give the manual remedy.
  - Git Bash's `kill`, `Stop-Process`, Task Scheduler and SIGKILL still leave the lock, and today's manual remedy stays documented.
  - Name the "no result line" states: a Windows console close, and a stderr that is blocked or gone (§5.6, Task 5).

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

Run: `GOTOOLCHAIN=go1.27.0 PATH=~/opt/git-2.55.0/bin:$PATH go test ./... -count=1 -timeout 30m`, and check the exit code on its own
Expected: every package `ok`, with the Unix `TestSignal_E*` cases among them.

- [ ] **Step 4: Race detector on the three changed packages (WSL)**

Run: `GOTOOLCHAIN=go1.27.0 PATH=~/opt/git-2.55.0/bin:$PATH go test -race -count=1 -timeout 60m ./internal/transport/ ./internal/repo/ ./cmd/git-remote-proton/` (run it in the background with a 2-hour tool timeout; `-race` slows the `repo` package several-fold)
Expected: `ok` for all three, with no `WARNING: DATA RACE`.

- [ ] **Step 5: The Go 1.22 floor**

Run (WSL, the distro's go1.22.2): `GOTOOLCHAIN=local /usr/lib/go-1.22/bin/go build ./...` and `GOTOOLCHAIN=local /usr/lib/go-1.22/bin/go vet ./...`. The CI go.mod leg runs the whole suite at Go 1.22; this is the local check.
Expected: both succeed with no output.

- [ ] **Step 6: Pester suite** (the module is untouched, but releasing.md's clean-tree checks assume it is green)

Run (PowerShell): `Invoke-Pester tests/`
Expected: all pass.

- [ ] **Step 7: Report for Craig**, in the session, not committed:
  - each step's result;
  - the merge gate: W1 and W2 must already PASS locally in both shells (Task 9). Once Craig pushes the branch and opens a PR, the `windows-latest` CI log must also show them as PASS, not SKIP (spec §6). A SKIP there blocks the merge until Craig decides;
  - that the release itself follows `docs/releasing.md`, with both formal live gates, and each gate brief adding spec §8's kill-mid-push check.

This plan ends here.
