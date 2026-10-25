# Release the remote lock when a push is killed — design

**Date:** 2026-10-24. **Scope:** the v2 remote helper (`cmd/git-remote-proton`,
`internal/transport`, `internal/repo`, `internal/testcli`), on Windows and Linux. The v1
`GitProtonBackup` module is untouched. **Origin:** ruling U6 (option A) of the scheduled-pushes-on-
Linux spec (`docs/superpowers/specs/2026-10-24-v2-linux-unattended-design.md`, on branch
`claude/strange-dewdney-e8fc56` until it lands), which kept that work docs-only and made this fix a
separate task for both platforms. Its fact K15 found the gap.

**Status: draft, awaiting Craig's approval.** Craig handed every choice to the agent ("Skip the
questions and go with your recommendations", 2026-10-24 21:46 PT). So each decision in §4 is
**the agent's recommendation**, not a ruling. Each becomes one only when Craig approves this
document. No code is written until then.

**Basis marks.** **[code]** read from the tree at `8848f41`. **[doc]** a named external document,
read 2026-10-24. **[record]** a document in this repo. **[measured Pn]** probe *n* in Appendix A,
run 2026-10-24 21:51–22:00 PT. The probes were hermetic: no Proton CLI, no account, writes only to a
scratch folder and to WSL `~/src/sigprobe`. **[inference]** reasoned, not run. **[estimate]** a
number reasoned from a record. **[untested]** a claim nothing here has checked.

## 1. The problem

1. **The helper handles no signals.** No `.go` file imports `os/signal`. The single-writer lock is
   taken in `loop`'s `list for-push` arm (`repo.AcquireLock`, after `EnsureParents`, `Bootstrap`
   and the restore-floor probe). It is released only by `loop`'s deferred `lock.Release()`. **[code]**
   By default a Go program exits on SIGHUP, SIGINT or SIGTERM **[doc]** (`os/signal`), and an exit
   by signal runs no deferred functions **[inference]**. So a push killed by a signal while it holds
   the lock leaves `.lock` on the remote. v2 has no takeover, so every later push then fails with
   `repo is locked by nonce … if that process is gone, remove … with the Proton CLI`
   (`internal/repo/lock.go`) until a person clears it. **[code]**
2. **The design doc already promises this.** "Concurrency posture" item 5 says release happens on
   every exit path, "including … cancellation, or signal". Item 6 says release verifies the nonce
   first and stays check-then-act. The code meets item 6 and misses item 5's signal clause.
   **[record]** `docs/v2-remote-helper-design.md`
3. **A broken stdout kills the helper too, with no one sending a signal.** On Unix, a Go program
   that writes to a broken pipe on stdout or stderr exits with SIGPIPE, unless it has called
   `Notify` for SIGPIPE. With `Notify`, the write just fails with `EPIPE` **[doc, measured P8]**. If
   git dies after `list for-push` (crashed, or killed on its own), the helper finishes its current
   work and then writes its next status line to the dead git. That write kills it, lock held.
   **[inference]** On Windows the same write returns an error and the process carries on
   **[measured P5]**.
4. **A lock whose acquisition is in doubt also leaks, signal or not.** `AcquireLock` makes the nonce,
   then uploads. `loop` keeps the lock only if `AcquireLock` succeeds. On `Ambiguous`, or when the
   read-back after `Committed` fails with an I/O error, the helper exits 1 and never tries to
   release. Yet a lock bearing its nonce may have landed. A signal during that upload is the same
   case. **[code]**
5. **`--set-head` takes the lock too** (`repo.SetHead`, released by its own defer). **[code]**
6. **The CLI child is out of reach.** `transport.CLI.run` uses `exec.Command` plus `CombinedOutput`,
   with no context and no handle on the running child, so nothing can stop it. **[code]**
7. **The one leak on record would not have been prevented.** Stage 5's S1: on Windows, an agent
   harness's tool timeout killed a push through Git Bash (`exit 143`) and orphaned `.lock`
   **[record]** (`docs/research/gates/stage5-gate.md`). Git Bash's `kill -TERM` ends a native
   Windows Go process without running its handler **[measured P1]**. So no signal handler could
   have caught S1. This fix narrows the leak; it does not close it on Windows.
8. **Scheduled Linux pushes are the case that needs it now.** The Linux spec runs each push under a
   time limit and documents clearing a leaked lock by hand "until the helper releases its lock when
   killed" (U6). **[record]**

## 2. What can and cannot be caught

| How the push is ended | Platform | What the helper gets | After this change | Basis |
|---|---|---|---|---|
| Ctrl-C or Ctrl-Break in the console running the push | Windows | `os.Interrupt`. The system sets no time limit | **Releases** | [measured P4] Ctrl-Break; [doc] Go maps Ctrl-C the same way; [doc] Microsoft: no timeout |
| The console window is closed, or Task Manager's **End task** on it | Windows | `SIGTERM` (Go's mapping of `CTRL_CLOSE_EVENT`). The system ends the process 5 s later | **Best effort.** A release usually needs longer than 5 s [estimate, §5.6] | [doc] Go runtime, Microsoft HandlerRoutine |
| Logoff or shutdown of an interactive session | Windows | Nothing: "interactive applications are terminated" | Leaks, as today | [doc] Microsoft |
| `Stop-Process`, `taskkill /F`, a process-tree kill | Windows | Nothing (TerminateProcess) | Leaks, as today | [measured P3] Stop-Process; the rest [inference] |
| Git Bash `kill` of any signal, MSYS `timeout`, a harness timeout through Git Bash (S1) | Windows | Nothing. TERM and HUP terminate; INT is ignored | Leaks, as today | [measured P1, P2] |
| Task Scheduler's stop or time limit | Windows | [untested] Assumed TerminateProcess | Assumed to leak | [inference] |
| `kill -TERM/-INT/-HUP` to the helper alone | Linux | The signal. Its CLI child gets nothing | **Releases.** The helper stops the child first | [measured P6] |
| Ctrl-C or a hangup in a terminal | Linux | SIGINT or SIGHUP to the whole foreground process group | **Releases** | [inference from P6, P7] |
| GNU `timeout` without `--foreground` | Linux | SIGTERM to its process group: git, helper and CLI child | **Releases** | [measured P7] |
| systemd stop or start timeout, `KillMode=control-group` (the default) | Linux | SIGTERM to every process in the unit, then SIGKILL after `TimeoutStopSec=` (90 s by default) | **Releases** if done within `TimeoutStopSec=` | [measured P10]; [doc] systemd.kill |
| systemd with `KillMode=mixed` or `process` | Linux | The helper is not the main process, so it gets only the final SIGKILL | Leaks | [measured P10b] mixed; [doc] |
| git dies mid-push, so the helper's stdout breaks | Linux | SIGPIPE on its next write, which kills it today | **Releases** (SIGPIPE handled, §5.2) | [measured P8] |
| The same | Windows | A write error only | Already releases today; unchanged | [measured P5] |
| SIGKILL, the OOM killer, power loss | Both | Nothing | Leaks, as today | — |
| SIGQUIT (Ctrl-\\) | Linux | Go's default: a goroutine dump, then exit | Leaks, **deliberately** (S2) | decision |

## 3. Goals, non-goals, anti-goals

**Goals.**
- Every termination the helper can observe releases a lock that bears its nonce. That covers the
  catchable signals in §2, a broken stdout, and an acquisition whose outcome is in doubt.
- The remote is left in a state the design already defines: no write is half-issued by the helper
  after it decides to stop.
- The time from the first signal to exit is bounded and documented, so callers can size their own
  stop windows.
- Everything is tested hermetically, with real OS signals and no account.

**Non-goals.**
- Recovering from terminations that cannot be caught (SIGKILL, TerminateProcess). The lock-held
  message's manual remedy stays the recovery, and `brief-checklist.md` rule 9 stays in force.
- Lock takeover or any stale-lock detection (see S1 for why).
- A general CLI startup retry (the design's open "retry with backoff on the SQLite signature"
  item). Only the shutdown release retries (S10).
- Cleaning up local temp folders or local git child processes on a signal. Today's behaviour
  stands: they are inert, under the OS temp folder.
- Any change to the Proton CLI pin.

**Anti-goals** (failure even if the code "works").
- Deleting a lock that is not provably ours. Release keeps its nonce-first check (item 6), and no
  path skips it.
- Releasing while our own CLI child may still be writing to the remote. A second writer could then
  interleave with our write.
- Any change in behaviour when no signal arrives: the same exit codes, messages and CLI calls on
  every path that today ends without a signal. One exception is S8's in-doubt acquisition, which
  gains a release attempt on a failure path only.
- A helper that hangs after a signal, or exits claiming the lock was released when it was not.

## 4. Decisions (the agent's recommendations)

| # | Decision | Considered and not taken | Basis |
|---|---|---|---|
| S1 | **Scope: catchable terminations only.** Signals, a broken stdout, and in-doubt acquisition. Uncatchable kills keep today's manual recovery. | (B) Auto-clear a lock whose holder is on this host and whose PID is gone. That is a form of takeover, which v2 removed on purpose, and hostnames are not unique identities while PIDs get reused. (C) A watchdog process that outlives the helper. A tree kill or job kill takes it too, and it adds a process to reason about. | §1.7, §2; design "Release cannot be conditional, so v2 has no takeover" |
| S2 | **Signals.** Unix: SIGINT, SIGTERM, SIGHUP start shutdown. Each is skipped if it was already ignored when the helper started (`signal.Ignored`), so `nohup` and background jobs keep their meaning. SIGPIPE is `Notify`'d and discarded (§5.2). SIGQUIT keeps Go's default, a goroutine dump with no release: that is the debugging exit, and the dump is most useful on a stuck push. Windows: `os.Interrupt` (Ctrl-C, Ctrl-Break) and `syscall.SIGTERM` (console close). | Handling SIGQUIT too: it would lose the dump. Respecting nothing that was inherited: `Notify` un-ignores SIGHUP and SIGINT [doc], which would break `nohup git push`. | [doc] `os/signal`; [measured P9] |
| S3 | **One shutdown coordinator takes over on the first signal.** It shuts the CLI gate, stops the in-flight child, releases through a fresh CLI instance under a deadline, and ends the process itself. The goroutine running the protocol is parked wherever it is. | Cancel a context and let the code unwind. That needs a context through every transport, repo and gitcmd call. It still cannot interrupt the blocking stdin read where the helper waits for git. And a minutes-long local pack build would eat the caller's stop window before the release even began. | §5.3; [code] |
| S4 | **Release mid-batch is allowed once our in-flight CLI child has exited or been killed.** The rest of the batch is abandoned: no new remote write is started after the signal. | Finish the batch first: unbounded, and a push killed for being stuck never finishes. Release at once without stopping the child: violates the second anti-goal. | §5.7 |
| S5 | **The in-flight child gets 5 s to exit on its own** (it usually received the same signal). Then, on Unix, SIGTERM and 3 s more. Then a kill (SIGKILL, or TerminateProcess). Release starts once the child is known to have exited, or the kill has been delivered. | Kill at once: needlessly cuts a ref write that was about to land. Never kill: a stuck child would block the release forever. | §5.3 |
| S6 | **A 60 s cap from the first signal to exit**, whatever happens. If the release has not finished by then, the helper says so and exits. | No cap: a release stuck on the same network stall that got the push killed would hang until SIGKILL, silently. 30 s: too tight. Each CLI call ran about 2–6 s in S1's timeline [estimate], and a release is 4 calls, plus S5's up to ~10 s. | [record] S1 timeline; systemd's default `TimeoutStopSec=` is 90 s [doc] |
| S7 | **Further signals during shutdown are noted on stderr and otherwise ignored.** The 60 s cap bounds the wait. SIGKILL (`kill -9`, `Stop-Process`) remains the way to stop at once, at the cost of the lock. | A second signal abandons the release. On Windows, every later Ctrl-C typed in that console reaches the helper, even after git has died and the prompt is back [doc], so a user tidying a prompt line would abort the release. A per-platform rule would be two behaviours to document. | [doc] Microsoft "CTRL+C and CTRL+BREAK signals" |
| S8 | **A lock is registered before its upload starts.** Every exit path (normal, error, signal) then attempts a nonce-checked release of every registered lock whose body may have landed. A `Refused` acquisition, where the lock is someone else's, is dropped from the register at once and costs no extra calls. | Leave acquisition as it is: keeps the §1.4 leak, and a signal during the upload would leak exactly as today. | §5.4 |
| S9 | **All three entry points are covered**: the push path, `--set-head`, and the read paths (fetch, `list`, `--version`). The read paths hold no lock: they stop the child and exit. | Push only: `--set-head` holds the same lock. | [code] |
| S10 | **The shutdown release retries the whole nonce-checked `Release`** until it succeeds or the cap is reached, pausing 2 s between attempts. It does not retry an unreadable lock, which needs a person. Each attempt re-reads the lock first, so a retry after a `Trash` that did land finds nothing and succeeds. This covers a CLI startup collision with a push the user starts right after the kill (§5.8), a stray Ctrl-C that kills a release call on Windows (S7), and a passing network blip. | A retry only on the SQLite "database is locked" signature: narrower, but that signature is the only failure it would cover. No retry: one collision would leak the lock this fix exists to release. | [record] design A8: startups collide, and the loser never reaches the network |
| S11 | **Exit code 128 + the first signal's number** (SIGINT 130, SIGTERM 143, SIGHUP 129), on both platforms (`os.Interrupt` counts as 2), whether or not the release succeeded. stderr says which. | Exit 1: loses which signal it was. 0 after a successful release: the push did not complete. | Shell convention |
| S12 | **The normal-path release is unchanged.** One attempt, failures warned, exit code untouched (the `loop` defer's documented contract). It gains only S8's register. | Adding S10's retry there too: a behaviour change outside this fix. A possible follow-up. | [code] `loop` defer comment |

## 5. Design

### 5.1 Where it hooks in

The coordinator is installed at the very start of `run()`, before any CLI child can start. It
covers `dispatchUtility` (`--version`, `--set-head`), `EnforceCertified`, and `loop`. A signal
before installation gets Go's default; no lock can exist that early. **Once shutdown has begun,
only the coordinator ends the process.** `main`'s `os.Exit(run())` must wait for it. Otherwise a
`loop` that returns on its own (git closed stdin, say) would exit the process mid-release.

### 5.2 Signal intake

- Unix: `signal.Notify` for the S2 set, minus any signal that `signal.Ignored` reports at startup.
  Windows: `os.Interrupt` and `syscall.SIGTERM`.
- **SIGPIPE (Unix):** a separate `Notify` whose channel is drained and discarded. Writes to a dead
  git then fail with `EPIPE` [measured P8]. The existing code already ignores `Flush` errors. When
  git has died, `loop` meets EOF on stdin, returns, and releases through its ordinary defer. It
  must be `Notify`, not `signal.Ignore`: an ignored disposition carries across exec, so every git
  and CLI child would inherit it, while a caught one is reset to the default in the child
  [inference].
- The first shutdown signal starts the sequence in §5.3. Later ones follow S7.

### 5.3 The shutdown sequence

1. Record the first signal and the 60 s deadline. Print the catch line (§5.6).
2. **Shut the CLI gate.** From now on, no CLI child may start through the instance the protocol
   goroutine uses. A caller that reaches the gate is parked for good. It must never get a result it
   could mistake for a remote outcome, and must never return into code that could exit the process.
3. **Stop the in-flight child**, if there is one (S5): wait up to 5 s for it to exit; on Unix,
   then send SIGTERM and wait up to 3 s; then kill it and wait up to 2 s for `Wait` to return. The
   gate and the in-flight record share one lock, so no child can start between the check and the
   registration.
4. **Release every registered lock** (§5.4, §5.5) through a **fresh** CLI instance, not the gated
   one. Every call on it is bounded by the deadline: a call still running at the deadline is
   killed. Retry per S10.
5. Print the outcome line. Exit with S11's code.

Local git children (pack builds, the restore-floor probe) are not stopped. They never touch the
remote. When only the helper was signalled they run to completion as orphans and exit; that leaves
temp files behind, as a crash does today. **[inference]**

### 5.4 Locks in doubt: register before upload

`AcquireLock` makes its nonce and registers `(path, nonce)` **before** `CreateExclusive` starts.
Then:

- `Committed`, and the read-back shows our nonce: held, registered.
- `Refused`: someone else's lock. Drop it from the register and return today's error. No release
  attempt, no extra calls.
- `Ambiguous`, an unrecognised outcome, or a read-back I/O error: return today's error, but **keep
  it registered**. The exit path's release then reads `.lock`: our nonce gets trashed, anything
  else is left alone.
- Read-back shows another nonce: today's error. It may stay registered; release will find it is
  not ours and leave it.
- A confirmed release, or one that found the lock absent or not ours, drops it from the register.

`loop`'s and `SetHead`'s defers release through the register instead of through a single
`*Lock`. That way an in-doubt acquisition is covered on the normal exit path too. This is the one
behaviour change outside signals (§3, third anti-goal), and only on paths that already fail. Where
the register lives (package `repo` or `cmd`), and its API, are the builder's (§9). Two rules bind
it: the shutdown path must be able to read it from another goroutine, and concurrent tests must
not see each other's entries.

### 5.5 The release itself

The same nonce-first `Lock.Release` logic, item 6 intact. It now takes the transport it runs on as
an argument (or equivalent), so the coordinator can pass the fresh instance. The outcomes are
unchanged: absent → done; unreadable → reported, not removed, not retried; another nonce → left
alone; ours → `Trash`, which only `Committed` confirms. The check-then-act race of item 6 is
unchanged per attempt.

### 5.6 What the user sees

The required content of each stderr line follows; the exact wording is the builder's. Every line
starts `git-remote-proton:`, like `warn`.

- **On the first signal, with a lock registered:** which signal; that the push is being stopped and
  the remote lock released; that this takes up to a minute; not to push again, or remove the lock
  by hand, until the outcome line; and that `kill -9`/`Stop-Process` abandons the release and leaves
  the lock.
- **On the first signal, no lock:** which signal, and that the helper is stopping.
- **Released:** the lock path; that the operation did not complete; that refs reported `ok` before
  the signal did land; and that re-running the push reconciles the rest.
- **Not released** (cap reached, unconfirmed, or unreadable): the reason and the lock path. Then
  today's manual remedy, worded so it only applies once no push for this repo is still running.
- **A later signal:** that it was received and the release is still running (S7).

In a terminal, git dies at once, so the prompt can come back while the helper is still releasing,
and these lines land after it. On Linux the shell takes the terminal's foreground back, so a later
Ctrl-C does not reach the helper [inference]. On Windows it does [doc]. That is S7's reason.

**How long it takes [estimate].** Stopping the child takes up to ~10 s (S5). A release is four CLI
calls (`Stat` and download to read the lock; `Trash`, which `Stat`s first, then trashes), about
8–24 s at S1's 2–6 s per call. Add up to ~10 s when the child has to be stopped rather than
having died of the same signal. 60 s is the cap. A Windows console close (5 s) will almost never
finish.

### 5.7 Why releasing mid-batch is safe

The batch runs in five phases (design, "Batch execution order"): buffer and poison-check; validate,
with reads only; build, upload and confirm one pack; deletes; creates and updates; then
`ensureHEAD`. Stopping between any two remote calls, or killing the call in flight, leaves a state
a crash leaves today. The design already defines and handles each one: **[record]**

- During phase 3: an orphan pack, or a pack with no `.idx`. These are inert, and the next push of
  the same objects repairs the pair (the "Pack present, `.idx` missing" row).
- During phases 4–5: some refs moved and some not. Multi-ref batches are not atomic. Each ref write
  is a single revision, so a killed write leaves the old value or the new one [inference from the
  revision model, design "Stage 1" C1/C8]. The next push reads the true state under the lock.
- An interrupted prune leaves empty folders, which self-heal handles.
- A first push stopped before `ensureHEAD` leaves the repo headless, a defined state. The next
  branch push writes `HEAD`.

What signal handling adds is only the **timing**: the lock goes away seconds after the stop rather
than when a person clears it. That is safe only if nothing of ours is still writing, which S4 and
step 3 ensure. **Residual, stated:** a request the killed child had fully sent may still be applied
by the server a moment after the child dies. A second writer would have to acquire the lock inside
that moment, which takes it several CLI calls. This is the same class as item 6's race, and is
accepted on the same terms. A grandchild of the CLI that outlives the child's kill is out of reach.
The only one seen (Stage 2.1's `waitDelay` hang) appeared after the child had finished its work.
**[record, inference]**

### 5.8 What callers must do (and the Linux unattended spec must say)

- **systemd:** keep `KillMode=control-group`, the default. `mixed` reaches the helper only with
  SIGKILL [measured P10b]. Keep `TimeoutStopSec=` at 75 s or more; the default 90 s is enough.
- **GNU `timeout`:** git exits on the SIGTERM at once, and `timeout` returns while the helper is
  still releasing [measured P11]. `--kill-after` then never fires, because the command it watches
  (git) has already exited [doc, inference]. A script that starts the next push straight away runs
  it alongside the release. S10's retry makes that cost time, not the lock [inference]. To avoid the
  overlap, the script waits for the push's output pipe to close: piping the push's output through a
  logger or `cat` makes the pipeline wait for the helper [measured P11b].
- **Anyone:** the lock-held message can appear for a few seconds after a signal, while the old
  helper is still releasing. Its manual remedy applies only once no push for that repo is running.
  This was already true of a slow, normal release.

## 6. Testing (hermetic)

Three layers. No account and no network. The fake CLI is the existing `testcli` shim served from
temp folders. Timing constants are package variables so tests can shrink them, as with `waitDelay`
today. New tests run with `-count=1` (brief-checklist rule 5).

**Layer 1: units (Fake transport).**
- The lock is registered before `CreateExclusive` is called (a Fake hook looks at the register
  during the call).
- A release through a different transport removes our nonce and leaves another nonce alone. It is
  idempotent.
- `Ambiguous`, an unrecognised outcome, and a read-back I/O error each lead to a release attempt on
  the exit path. `Refused` adds no calls (pinned by a call count).
- S10: a Fake that fails N times then succeeds is released within the cap. An unreadable lock is
  not retried. A Fake that never succeeds ends at the cap with the not-released message.

**Layer 2: the CLI child (`internal/transport`, the existing re-exec roles).**
- Once the gate is shut, `run` starts no child (a role writes a marker per start) and does not
  return.
- A child that exits within the grace gets no kill. A child that hangs is sent SIGTERM (Unix) and
  then killed. A child that ignores SIGTERM is killed. Each is confirmed exited.
- A deadline-bound instance kills a hung call at the deadline.

**Layer 3: end to end, with real OS signals (`cmd/git-remote-proton`).** The helper runs as a
re-exec'd test binary with the shim on PATH, driven through the protocol over pipes. The shim gains
a knob that names one call to block on. It writes a ready file and blocks; it can block before or
after writing its file. The test waits for the ready file, then signals. Each case asserts the exit
code, that `.lock` is absent from the shim tree (or present, where the case says so), that no shim
process survives, and the stderr outcome line.

| # | Case | Platform |
|---|---|---|
| E1 | SIGTERM, SIGINT, SIGHUP to the helper alone, blocked in the pack upload with the lock held | Unix |
| E2 | SIGTERM to the helper's process group, like `timeout`. The helper is started in its own group, so the test does not signal itself | Unix |
| E3 | A signal while the helper is idle, waiting for the push batch after `list for-push` | Unix |
| E4 | A signal during the lock upload: blocked before the shim writes `.lock`, and blocked after (landed, no reply). Both end with `.lock` absent | Unix |
| E5 | git dies mid-batch: the test closes the helper's stdout and stdin, then unblocks the shim. The helper is not killed by SIGPIPE, and releases on the EOF path | Unix |
| E6 | SIGHUP ignored at start (a `trap '' HUP; exec` wrapper): SIGHUP has no effect, and the push completes and releases normally | Unix |
| E7 | `--set-head`, signalled during its `HEAD` upload | Unix |
| E8 | A fetch, signalled during a download: prompt exit, no lock created, child gone | Unix |
| E9 | A second signal during a slow release is ignored, and the release completes | Unix |
| E10 | A release that never finishes (the shim blocks on `trash`) ends at the shrunk cap with the not-released line and S11's code. `.lock` remains | Unix |
| W1 | Ctrl-Break by `GenerateConsoleCtrlEvent` to a helper started with `CREATE_NEW_PROCESS_GROUP`, blocked in the pack upload. The shim child is in the same group and gets the event too, as with a real Ctrl-C | Windows |

W1 uses the method measured in P4, which worked from both the PowerShell tool and Git Bash. If
`GenerateConsoleCtrlEvent` fails because no console is attached, the test **skips with that
reason** instead of failing. The plan must then confirm from the `windows-latest` CI log that W1
**ran**. If it skipped there, that is reported to Craig, not absorbed. The Unix cases run in the
Linux CI legs and in the WSL distro.

Existing tests stay green. On the paths no signal reaches, the CLI-call counts stay the same
(anti-goal 3), except S8's in-doubt paths.

## 7. Documentation to change

- **v2 design doc:** rewrite "Concurrency posture" item 5 to say what is released and what is not
  (§2's table in short). Change the error table's "cancellation, timeout" row to name the signal
  behaviour. Add revision **v6.10**, recording S1–S12 as approved, with the options not taken.
- **README "Honest limits":** what releases the lock on each platform, and that uncatchable kills
  still need the manual remedy (which stays documented).
- **`docs/research/gates/brief-checklist.md` rule 9:** add a note that it stays in force. On
  Windows, a harness kill through Git Bash is uncatchable [measured P1].
- **The Linux unattended spec and its README section:** §5.8's rules (KillMode, `TimeoutStopSec=`,
  waiting on the output pipe), and narrowing its "a stalled push leaks the lock" row to SIGKILL and
  the other uncatchable kills. Whichever of the two lands second updates the other. If that spec
  lands first, its "until the helper releases its lock when killed" sentence is rewritten when this
  ships.
- **CHANGELOG** entry for the release that carries it; **`docs/README.md`** index links to this
  spec and its plan.

## 8. Release

The change ships in the next helper release, under `docs/releasing.md`, in order. `.go` files
change, so a stand-in is not eligible, and both formal live gates run. Each gate brief adds one
live check on its own scratch remote: kill a push mid-upload with that platform's catchable route
(Linux: SIGTERM to the push's process group; Windows: Ctrl-Break or Ctrl-C), then confirm `.lock`
is gone and the next push succeeds. That check is the first evidence for U1–U3 in §10. The
version number is left to release time. A behaviour change suggests 0.11.0 (the agent's
suggestion). The CLI pin does not move.

## 9. Left to the builder

- API names and file layout. A small `cmd/git-remote-proton/shutdown.go` for the coordinator is the
  suggestion. The gate and in-flight tracking go in `internal/transport/cli.go`; the register in
  `internal/repo/lock.go` or `cmd`.
- How `run` tracks its child, given that `CombinedOutput` owns `Wait` (for example `Start`, then
  `Wait` with a done channel closed after it returns).
- The exact constants, within these limits: S5's waits, S10's pause, and the 60 s cap. Changing the
  cap means changing §5.8's `TimeoutStopSec=` figure and the docs in §7.
- Message wording, within §5.6.
- Whether the fresh release instance is wrapped in `transport.NewTraced` (the suggestion: yes, so
  traces stay consistent).

## 10. Untested, and what stays at risk

- **U1** [untested] How the real `proton-drive` child reacts to SIGINT, SIGTERM and Ctrl-Break, and
  how fast it exits. It is a Bun-built program [record: the Linux probe's `com.oven-sh.bun.Secret`
  schema], so Bun's defaults apply unless the CLI installs handlers.
- **U2** [untested] Whether a killed CLI leaves local state (its SQLite caches, Linux
  `events.lock`) that makes the release's first fresh call fail. S10's retry covers a passing
  failure; a lasting one would leave the lock, said loudly.
- **U3** [estimate] Release duration. The figures in §5.6 come from one record's timestamps. The
  gate check in §8 measures it.
- **U4** [inference] Task Scheduler's stop and time limit terminate without a console event.
  Probing this would need a scheduled task, which is a persistent change, so it was not run.
- **U5** [inference] On Linux, a terminal Ctrl-C typed after the prompt returns does not reach the
  helper.
- **Residual, by design:** every uncatchable kill in §2 still leaks. That includes S1's own route
  on Windows. The manual remedy and rule 9 remain the answer.

## Appendix A — probes, 2026-10-24 21:51–22:00 PT

A throwaway Go program (`sigprobe`, built with go1.27.0; not committed). In `catch` mode it
`Notify`s INT, TERM and HUP. It can start a child of itself that records any signal it gets. On a
signal it simulates a release: N seconds of work, then a fresh child process started after the
signal, then exit 143. Each step is timestamped to a marker file. Windows runs were on this laptop
(Windows 11, 10.0.26220). Linux runs were in the WSL2 Ubuntu-24.04 test distro (kernel 6.6.87),
with the binary cross-built (`GOOS=linux CGO_ENABLED=0`).

| # | What | Result |
|---|---|---|
| P1 | Windows, Git Bash `kill -TERM` and `kill -HUP` on the native probe | No handler ran, in the probe or its child; the shell saw 143 and 129 |
| P2 | Windows, Git Bash `kill -INT` | No effect. The probe ran until stopped by hand 2 min later |
| P3 | Windows, `Stop-Process` on that probe | Ended; no handler ran |
| P4 | Windows, `GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT)` to a probe started with `CREATE_NEW_PROCESS_GROUP`, from the PowerShell tool and from Git Bash | Handler ran (`interrupt`). The same-group child got it too. 3 s of cleanup and a child started after the signal both completed; exit 143 |
| P5 | Windows, writing to stdout after the reader (`head -n 1`) exited, with and without `Notify(SIGPIPE)` | The write returned "The pipe is being closed."; the process carried on in both cases |
| P6 | Linux, `kill -TERM`, `-INT`, `-HUP` to the probe alone | Handler ran each time. The child recorded no signal and was left running after its parent exited. The post-signal child ran |
| P7 | Linux, `timeout -k 10 2`, and `timeout -k 3 2` with 6 s of cleanup | Both the probe and its child got SIGTERM. With `-k 10`, cleanup and the post-signal child finished. With `-k 3`, SIGKILL landed 3 s after the TERM, mid-cleanup |
| P8 | Linux, the P5 write, default and with `Notify(SIGPIPE)` | Default: died on the first write after the reader left. `Notify`: the write returned `broken pipe` and the process carried on |
| P9 | Linux, `signal.Ignored` under `nohup`, under `trap '' INT`, and plain | HUP ignored; INT ignored; neither |
| P10 | Linux, `systemd-run --user` oneshot, `TimeoutStartSec=3`, `TimeoutStopSec=30`, default KillMode | At the timeout the probe and its child got SIGTERM. 4 s of cleanup and the post-signal child finished |
| P10b | The same with `KillMode=mixed`, with `sh` as the main process and the probe its child | The probe got no SIGTERM |
| P11 | Linux, `timeout -k 10 2 sh -c 'probe …; echo'` (`sh` standing in for git) | `timeout` returned at the TERM. The probe's cleanup went on 4 s longer |
| P11b | The same, with output piped through `cat` | The pipeline returned only after the probe finished |

The Go and Windows documentation behind §2, read 2026-10-24: Go's `os/signal` package doc (the
Windows section and SIGPIPE); `runtime/os_windows.go` `ctrlHandler`, in go1.27.0 and in the WSL
distro's go1.22.2 source (it maps `CTRL_CLOSE/LOGOFF/SHUTDOWN_EVENT` to SIGTERM and blocks so the
handler can run); Microsoft's HandlerRoutine (its timeouts table: close 5000 ms, Ctrl-C/Break no
timeout; logoff and shutdown reach only services) and "CTRL+C and CTRL+BREAK signals" (passed to
all processes attached to the console); systemd.kill (`KillMode=`; `control-group` is the
default).
