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
   git dies after `list for-push` (crashed, or killed on its own), the helper finishes the batch it
   is running and then writes its status lines to the dead git, all at once after the batch
   (`loop` flushes them once, after `repo.Push` returns) **[code]**. That write kills it, lock held.
   **[inference]** On Windows the same write returns an error and the process carries on
   **[measured P5]**. It then meets EOF on stdin and exits **0**, reporting success for a session
   git never finished **[code]**.
4. **A lock whose acquisition is in doubt also leaks, signal or not.** `AcquireLock` makes the nonce,
   then uploads. `loop` keeps the lock only if `AcquireLock` succeeds. Every failure path returns
   `nil, err` and the nonce is lost: an error from `CreateExclusive` itself (which is what a killed
   upload produces, since the CLI wrapper returns `Ambiguous` *with* an error), `Ambiguous`, an
   unrecognised outcome, or an I/O error on the read-back. In each, a lock bearing the nonce may
   have landed, and nothing will remove it. **[code]**
5. **`--set-head` takes the lock too** (`repo.SetHead`, released by its own defer). **[code]**
6. **The CLI child is out of reach.** `transport.CLI.run` uses `exec.Command` plus `CombinedOutput`,
   with no context and no handle on the running child, so nothing can stop it. Its `Wait` also
   blocks up to `waitDelay` (30 s) after the child exits while anything else still holds the
   output pipe, a hang seen once at Stage 2.1. **[code]**
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
| Ctrl-C or Ctrl-Break in a console window (Windows Terminal, conhost) running the push | Windows | `os.Interrupt`. The system sets no time limit | **Releases** | [measured P4] Ctrl-Break; [doc] Go maps Ctrl-C the same way; [doc] Microsoft: no timeout |
| Ctrl-C in Git Bash's own mintty window | Windows | [untested] Probably nothing: MSYS delivers no console events to native programs (P1, P2) | Assumed to leak (U6) | [inference] |
| The console window is closed, or Task Manager's **End task** on it | Windows | `SIGTERM` (Go's mapping of `CTRL_CLOSE_EVENT`). The system ends the process 5 s later | **Best effort** (S18). A release usually needs longer than 5 s [estimate, §5.6] | [doc] Go runtime, Microsoft HandlerRoutine |
| Logoff or shutdown of an interactive session | Windows | Nothing. Go would map those events to SIGTERM, but Windows sends them only to services and terminates interactive programs | Leaks, as today | [doc] Microsoft, Go runtime |
| `Stop-Process`, `taskkill /F`, a process-tree kill | Windows | Nothing (TerminateProcess) | Leaks, as today | [measured P3] Stop-Process; the rest [inference] |
| Git Bash `kill` of any signal, MSYS `timeout`, a harness timeout through Git Bash (S1) | Windows | Nothing. TERM and HUP terminate; INT is ignored | Leaks, as today | [measured P1, P2] |
| Task Scheduler's stop or time limit | Windows | [untested] Assumed TerminateProcess | Assumed to leak | [inference] |
| `kill -TERM/-INT/-HUP` to the helper alone | Linux | The signal. Its CLI child gets nothing | **Releases.** The helper stops the child first | [measured P6] |
| Ctrl-C or a hangup in a terminal | Linux | SIGINT or SIGHUP to the whole foreground process group | **Releases** | [inference from P6, P7] |
| GNU `timeout` without `--foreground` | Linux | SIGTERM to its process group: git, helper and CLI child | **Releases** | [measured P7] |
| systemd stop or start timeout, `KillMode=control-group` (the default) | Linux | SIGTERM to every process in the unit, then SIGKILL after `TimeoutStopSec=` (90 s by default) | **Releases** if done within `TimeoutStopSec=` | [measured P10]; [doc] systemd.kill |
| systemd with `KillMode=mixed` or `process` | Linux | The helper is not the main process, so it gets only the final SIGKILL | Leaks | [measured P10b] mixed; [doc] |
| git dies mid-push, so the helper's stdout breaks | Linux | SIGPIPE on its next write, which kills it today | **Releases**, and exits 1 (S16) | [measured P8] |
| The same | Windows | A write error only | Already releases today; now exits 1, not 0 (S16) | [measured P5] |
| SIGKILL, the OOM killer, power loss | Both | Nothing | Leaks, as today | — |
| SIGQUIT (Ctrl-\\) | Linux | Go's default: a goroutine dump, then exit | Leaks, **deliberately** (S2) | decision |

**What the Windows half buys.** On Windows the only catchable route is an attended Ctrl-C or
Ctrl-Break in a console window. That is still the commonest way a person stops a push they are
watching, and the Windows-specific code is small: the signal set (S2), the console-close shortcut
(S18) and one test (W1). Everything else is shared with Linux. Unattended Windows kills stay
uncatchable, and §7 says so in the release notes.

## 3. Goals, non-goals, anti-goals

**Goals.**
- Every termination the helper can observe ends with a nonce-checked attempt to remove a lock that
  may bear its nonce, and a truthful line saying how it went. That covers the catchable signals in
  §2, a broken stdout, and an acquisition whose outcome is in doubt.
- **No new remote write starts after the signal**, and the lock is not removed until the write in
  flight has stopped and had time to land (S4, S17).
- The time from the first signal to exit is bounded and documented, so callers can size their own
  stop windows.
- Everything is tested hermetically, with real OS signals and no account.

**Non-goals.**
- Recovering from terminations that cannot be caught (SIGKILL, TerminateProcess). The lock-held
  message's manual remedy stays the recovery, and `brief-checklist.md` rule 9 stays in force.
- Lock takeover or any stale-lock detection (see S1 for why).
- A general CLI startup retry (the design's open "retry with backoff on the SQLite signature"
  item). Only the shutdown release retries (S10).
- Noticing git's death in the middle of a batch. The helper cannot see it until its next protocol
  read or write, which comes after the batch (S16).
- Cleaning up local temp folders or local git child processes on a signal. Today's behaviour
  stands: they are inert, under the OS temp folder.
- Any change to the Proton CLI pin.

**Anti-goals** (failure even if the code "works").
- Deleting a lock that is not provably ours. Release keeps its nonce-first check (item 6), and no
  path skips it.
- Removing the lock while our own CLI child may still be writing to the remote. A second writer
  could then interleave with our write.
- Any change in behaviour when no signal arrives: the same exit codes, messages and CLI calls on
  every path that today ends without a signal. Two exceptions, both on paths that already fail: an
  in-doubt acquisition gains a release attempt (S8), and a session whose protocol write failed
  exits 1 instead of 0 (S16).
- A helper that hangs after a signal, or prints that it removed a lock it did not remove.

## 4. Decisions (the agent's recommendations)

| # | Decision | Considered and not taken | Basis |
|---|---|---|---|
| S1 | **Scope: catchable terminations only.** Signals, a broken stdout, and in-doubt acquisition. Uncatchable kills keep today's manual recovery. | (B) Auto-clear a lock whose holder is on this host and whose PID is gone. That is a form of takeover, which v2 removed on purpose, and hostnames are not unique identities while PIDs get reused. (C) A watchdog process that outlives the helper. A tree kill or job kill takes it too, and it adds a process to reason about. | §1.7, §2; design "Release cannot be conditional, so v2 has no takeover" |
| S2 | **Signals.** Unix: SIGINT, SIGTERM, SIGHUP start shutdown. Each is skipped if it was already ignored when the helper started (`signal.Ignored`), so `nohup` and background jobs keep their meaning. SIGPIPE is `Notify`'d and discarded (§5.2). SIGQUIT keeps Go's default, a goroutine dump with no release: that is the debugging exit, and the dump is most useful on a stuck push. Windows: `os.Interrupt` (Ctrl-C, Ctrl-Break) and `syscall.SIGTERM` (console close). | Handling SIGQUIT too: it would lose the dump. Respecting nothing that was inherited: `Notify` un-ignores SIGHUP and SIGINT [doc], which would break `nohup git push`. | [doc] `os/signal`; [measured P9] |
| S3 | **One shutdown coordinator takes over on the first signal.** It shuts the CLI gate, stops the in-flight child, releases through its own CLI instance under a deadline, and ends the process itself. The goroutine running the protocol is parked at the gate, both before it can start a child and after a child it started returns, so it never acts on a killed call's result. | Cancel a context and let the code unwind. That needs a context through every transport, repo and gitcmd call (gitcmd uses one only inside its restore-floor probe). It still cannot interrupt the blocking stdin read where the helper waits for git. And a minutes-long local pack build would eat the caller's stop window before the release even began. | §5.3; [code] |
| S4 | **Release mid-batch is allowed once the remote is quiet**: the in-flight child's exit is confirmed (S5), and, if that call may have changed the remote, the settle wait has passed (S17). The rest of the batch is abandoned: no new remote write starts after the signal. | Finish the batch first: unbounded, and a push killed for being stuck never finishes. Release at once without stopping the child: violates the second anti-goal. | §5.7 |
| S5 | **The in-flight child gets 5 s to exit on its own** (it usually received the same signal). Then, on Unix, SIGTERM and 3 s more. Then a kill (SIGKILL, or TerminateProcess). **Release waits for the operating system to confirm the process has exited** (S13 separates that from draining its output). If the exit is not confirmed by the cap, nothing is released, and the helper says so. | Kill at once: needlessly cuts a ref write that was about to land. Never kill: a stuck child would block the release forever. Treat "kill delivered" as enough: a child in an uninterruptible wait has not stopped. | §5.3 |
| S6 | **A 60 s cap from the first signal to exit**, enforced by its own timer. At the cap the timer kills any release child, tries the outcome line for up to 1 s, and exits, whatever else is running. No stderr write can delay it. | No cap: a release stuck on the same network stall that got the push killed would hang until SIGKILL, silently. 30 s: too tight for §5.6's figures. | [record] S1 timeline; systemd's default `TimeoutStopSec=` is 90 s [doc] |
| S7 | **Further signals during shutdown are noted on stderr and otherwise ignored.** The 60 s cap bounds the wait. SIGKILL (`kill -9`, `Stop-Process`) remains the way to stop at once, at the cost of the lock. **Release-phase CLI children start in their own process group** (Unix `Setpgid`; Windows `CREATE_NEW_PROCESS_GROUP`, which disables Ctrl-C for them), so a later Ctrl-C in the same window does not kill them. Their per-call bound and the cap still apply. | A second signal abandons the release. On Windows, every later Ctrl-C typed in that console reaches the helper, even after git has died and the prompt is back [doc], so a user tidying a prompt line would abort the release. A per-platform rule would be two behaviours to document. | [doc] Microsoft "CTRL+C and CTRL+BREAK signals" |
| S8 | **A lock is registered before its upload starts, and stays registered** unless the upload was `Refused` with no error (someone else's lock) or the CLI never started. Every exit path (normal, error, signal) then attempts a nonce-checked release of every registered lock. Release reports a **typed outcome** (§5.5), so the helper never says it removed a lock it found absent or foreign. | Leave acquisition as it is: keeps the §1.4 leak, and a signal during the upload would leak exactly as today. | §5.4 |
| S9 | **All three entry points are covered**: the push path, `--set-head`, and the read paths (fetch, `list`, `--version`). The read paths hold no lock: they stop the child and exit. | Push only: `--set-head` holds the same lock. | [code] |
| S10 | **The shutdown release retries the whole nonce-checked release** until it reaches a final outcome or the cap, pausing 2 s between attempts. **Each release CLI call is bounded at 15 s**, so one hung call cannot use the whole cap. A transport error, a killed or timed-out call, or a download that failed is retried. Contents that downloaded and did not parse (`unreadable`) are final and are not retried: they need a person. Each attempt re-reads the lock first, so a retry after a `Trash` that did land finds nothing. This covers a CLI startup collision with a push the user starts right after the kill (§5.8), a passing network blip, and a call killed by a stray signal. | A retry only on the SQLite "database is locked" signature: narrower, but that signature is the only failure it would cover. No retry: one collision would leak the lock this fix exists to release. | [record] design A8: startups collide, and the loser never reaches the network |
| S11 | **Exit code 128 + the first signal's number** (SIGINT 130, SIGTERM 143, SIGHUP 129), on both platforms (`os.Interrupt` counts as 2), whatever the release outcome. stderr says which. Who sees it: git, and only when git outlived the helper (`kill` to the helper alone). When git dies of the same signal, a script sees git's or `timeout`'s code, and systemd marks the unit failed for its timeout; neither reads the helper's. | Exit 1: loses which signal it was. 0 after a successful release: the push did not complete. | Shell convention; [inference] git's and systemd's reporting |
| S12 | **The normal-path release is unchanged.** One attempt, failures warned (the `loop` defer's documented contract), now through the register. | Adding S10's retry there too: a behaviour change outside this fix. A possible follow-up. | [code] `loop` defer comment |
| S13 | **`CLI.run` is reworked to own its child.** It starts the child with output pipes the helper creates, so the process's exit (`Wait`) is observed apart from draining its output. It records the child as in flight, and keeps the process handle for S5's stop. After the process exits, the output is drained for up to `waitDelay` as today, with today's warning when that runs out. Every `CLI` method still goes through `run`, so no call site changes. | `exec.CommandContext`: it kills on cancel, but its `Wait` still blocks on the pipe drain, so exit and drain stay tied. Keeping `CombinedOutput`: no handle, and the 30 s drain lands inside the cap. | [code] `cli.go` `run`, `waitDelay` |
| S14 | **One process-wide gate in `internal/transport`** covers every `*CLI` the process creates: `run()`'s, `runSetHead()`'s, and `EnforceCertified`'s calls through them. The coordinator's release instance, made by its own constructor, is the only exemption. It does not repeat `EnforceCertified`: this process already certified the CLI on its PATH at startup, and nothing changes PATH. | A gate per instance: `run()` and `runSetHead()` each build their own CLI, so a signal during the version check would be missed. Re-certifying the release instance: one more CLI startup inside the collision window S10 worries about. | [code] `main.go` |
| S15 | **The register lives in `internal/repo`**, process-wide. `AcquireLock` fills it, so `loop` and `SetHead` need no new plumbing. The coordinator reads it through a `repo` function. It never holds a mutex across a CLI call, so the coordinator and a normal-path release never deadlock. Tests reset it. No test in `internal/repo` or `cmd` runs in parallel [code: both packages say so]. | A register in `cmd`: `AcquireLock` cannot reach it, and its failure paths drop the nonce. A register threaded through `SetHead`/`AcquireLock`: wider API change for the same effect. | [code] `lock.go`, `sethead.go` |
| S16 | **A failed protocol write is remembered, and `loop` then exits 1, not 0.** Release takes the normal path. A broken stdout does not start the coordinator: the next step after a status write is the stdin read, which returns EOF at once when git is gone, and then the defer releases. | Starting the coordinator on a failed write: it would only bound the normal release, which S12 keeps as it is. Exit 0: reports success for a session git never finished. | [code] `loop`; §1.3 |
| S17 | **A settle wait of 5 s before reading the lock**, when the stopped call may have changed the remote (an upload, a trash, a folder create). A request the killed child had already sent can still be applied by the server after the child dies. The wait lets it land first: a late lock upload is then seen and removed, and a late ref write lands before anyone else can take the lock. | No wait: a late-landing lock upload would be read as absent, then appear, and leak while the helper said none was there. A longer wait: costs every signalled push for a delay nobody has measured. | [inference]; [untested] U7 |
| S18 | **On a Windows console close (SIGTERM on Windows) the sequence is cut short**: no grace, no settle; the child got the same event. It goes straight to the release, best effort, inside the system's 5 s. | The full sequence: S5's grace alone would spend the whole 5 s. | [doc] Microsoft: close timeout 5000 ms |

## 5. Design

### 5.1 Where it hooks in

The coordinator is installed at the very start of `run()`, before any CLI child can start. It
covers `dispatchUtility` (`--version`, `--set-head`), `EnforceCertified`, and `loop`. A signal
before installation gets Go's default; no lock can exist that early. When `run()` returns with no
signal received, it stops the notifications (`signal.Stop`) and the coordinator, so tests that
call `run()` in-process leak nothing.

**Once shutdown has begun, only the coordinator ends the process.** `main`'s `os.Exit(run())`
waits for it. Otherwise a `loop` that returns on its own (git closed stdin, say) could exit the
process mid-release. From that moment `warn` is silent, so the user sees one coherent account, the
coordinator's. Package `repo` writes a few notes to stderr directly; one or two may still print
before the protocol goroutine parks at its next CLI call. That is accepted rather than plumbed.

### 5.2 Signal intake

- Unix: `signal.Notify` for the S2 set, minus any signal that `signal.Ignored` reports at startup.
  Windows: `os.Interrupt` and `syscall.SIGTERM`.
- **SIGPIPE (Unix):** a separate `Notify` whose channel is drained and discarded. Writes to a dead
  git then fail with `EPIPE` [measured P8]. It must be `Notify`, not `signal.Ignore`: an ignored
  disposition carries across exec, so every git and CLI child would inherit it, while a caught one
  is reset to the default in the child [inference].
- **A failed protocol write (both platforms)** is recorded by the writer under `loop`'s buffer. The
  code ignores `Flush` errors today. `loop` then reads stdin, meets EOF, returns 1 (S16), and the
  defer releases.
- The first shutdown signal starts the sequence in §5.3. Later ones follow S7.

### 5.3 The shutdown sequence

1. Record the first signal and the 60 s deadline, and start the cap timer (S6).
2. **Shut the CLI gate** (S14). From now on no CLI child starts through any instance but the
   coordinator's. A caller that reaches the gate is parked for good, and so is a caller whose
   child returns after this point: it never sees that result. The gate and the in-flight record
   share one lock, so no child can start between the check and the registration. Only then print
   the catch line (§5.6); if stderr blocks, the timer still ends the process.
3. **Stop the in-flight child**, if there is one (S5): wait up to 5 s for it to exit; on Unix, then
   send SIGTERM and wait up to 3 s; then kill it and wait up to 2 s more. Exit means the process
   has exited, not that its output has drained (S13). If no exit is confirmed, skip to step 6 with
   "not confirmed". On a Windows console close, skip the waits (S18).
4. **Settle** for 5 s if the stopped call may have changed the remote (S17). Skipped on a Windows
   console close.
5. **Release every registered lock** (§5.4, §5.5) through the coordinator's CLI instance: its
   children in their own process group (S7), each call bounded at 15 s, every call killed at the
   deadline. Retry per S10.
6. Print the outcome line. Exit with S11's code.

Local git children (pack builds, the restore-floor probe) are not stopped. They never touch the
remote. When only the helper was signalled they run to completion as orphans and exit; that leaves
temp files behind, as a crash does today. **[inference]** A signal that kills a local git child
(a group signal) can make `repo.Push` take an error path; its next CLI call parks at the gate
(§5.1 says what it may print first).

### 5.4 Locks in doubt: register before upload

`AcquireLock` makes its nonce and registers `(path, nonce)` **before** `CreateExclusive` starts.
Then:

- `CreateExclusive` returns `Refused` with no error: someone else's lock. Drop it from the register
  and return today's error. No release attempt, no extra calls.
- The CLI never started (the binary is missing, or the basename check refused before any upload):
  drop it, return today's error.
- **Any other result keeps it registered**: `Committed`, an error from `CreateExclusive` (a killed
  upload lands here), `Ambiguous`, an unrecognised outcome, a read-back I/O error, or a read-back
  that shows another nonce. Each returns today's error, or the held lock. The exit path's release
  then reads `.lock` and acts on what it finds (§5.5).
- A release whose outcome is final (removed, absent, foreign, unreadable) drops it from the
  register. One that is not confirmed keeps it, so a later attempt can try again.

`loop`'s and `SetHead`'s defers release through the register instead of through a single `*Lock`.
That way an in-doubt acquisition is covered on the normal exit path too: one of the two behaviour
changes outside signals (§3).

### 5.5 The release and its outcomes

The same nonce-first logic as `Lock.Release`, item 6 intact, taking the transport it runs on as an
argument so the coordinator can pass its own instance. It returns one of these outcomes instead of
folding several into `nil`:

| Outcome | When | Final? |
|---|---|---|
| **removed** | `.lock` held our nonce, and `Trash` returned `Committed` | yes |
| **absent** | No `.lock`: our acquisition never landed, or it was already removed | yes |
| **foreign** | `.lock` holds another nonce. Left alone | yes |
| **unreadable** | `.lock` downloaded but did not parse. Not removed, not retried | yes |
| **failed** | Any transport error, a killed or timed-out call, or a `Trash` that did not return `Committed` | no: retried on the shutdown path (S10), reported on the normal path |

The check-then-act race of item 6 is unchanged per attempt.

### 5.6 What the user sees

The required content of each stderr line follows; the exact wording is the builder's. Every line
starts `git-remote-proton:`, like `warn`.

- **On the first signal, with a lock registered:** which signal; that the push is stopping and the
  helper is checking the remote lock; that this takes up to a minute; not to push again, or remove
  the lock by hand, until the result line; and that `kill -9`/`Stop-Process` abandons the check
  and may leave the lock.
- **On the first signal, no lock:** which signal, and that the helper is stopping.
- **The result, one line per registered lock:** the lock path and the nonce, then the outcome in
  plain words. *removed*: the lock is gone. *absent*: no lock of ours was on the remote.
  *foreign*: another process holds it, and it was left alone. *unreadable*: today's message.
  *failed* or *not confirmed*: the reason, and today's manual remedy, worded so it applies only
  once no push for this repo is running, with a note that a lock bearing this nonce, if one
  appears later, is ours to remove.
- **When a push batch was interrupted:** that the push did not finish; that some of its remote
  changes may have landed while git was told nothing (statuses go out only after the whole batch,
  §1.3); and that re-running the push starts from the remote's real state.
- **A later signal:** that it was received and the check is still running (S7).
- **Cut short by the system** (a Windows console close at 5 s): no result line arrives. That state
  is named in the README's lock-recovery steps: the lock is either gone or still there, never
  half-removed, and the usual check applies.

In a terminal, git dies at once, so the prompt can come back while the helper is still releasing,
and these lines land after it. On Linux the shell takes the terminal's foreground back, so a later
Ctrl-C does not reach the helper [inference]. On Windows it does [doc]; S7 covers that.

**How long it takes [estimate].** When the child died of the same signal: the settle wait (5 s, when
the call was a write), then a release of four CLI calls (`Stat` and download to read the lock;
`Trash`, which `Stat`s first, then trashes), about 8–24 s at S1's 2–6 s per call. So about 13–29 s.
When only the helper was signalled, add up to 10 s to stop the child. Retries fill the rest of the
60 s cap. A Windows console close (5 s) will rarely finish.

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
- A signal during `Bootstrap`'s marker upload, before any lock exists, leaves whatever a crash
  there leaves today; this change adds nothing and removes nothing.

What signal handling adds is the **timing**: the lock goes away seconds after the stop rather than
when a person clears it. That is safe only if nothing of ours can still land. S5 makes sure the
child has exited, and S17 gives a request it had already sent time to land.

**Residuals, stated.** A request that the server applies more than the settle wait after the child
died could still land late. A late lock upload would then leak, with the result line having said
*absent*; the nonce in that line lets a person recognise it. A late ref write would need a second
writer to take the lock in the meantime, which takes it several CLI calls. Neither delay has been
measured (U7). This is the same class as item 6's race, and is accepted on the same terms. A
grandchild of the CLI that outlives the child's kill is out of reach. The only one seen (Stage
2.1's `waitDelay` hang) appeared after the child had finished its work. **[record, inference]**

### 5.8 What callers must do (and the Linux unattended spec must say)

- **systemd:** keep `KillMode=control-group`, the default. `mixed` reaches the helper only with
  SIGKILL [measured P10b]. Keep `TimeoutStopSec=` at 75 s or more; the default 90 s is enough.
- **GNU `timeout`:** git exits on the SIGTERM at once, and `timeout` returns while the helper is
  still releasing [measured P11]. A script that starts the next push straight away runs it
  alongside the release; S10's retry makes that cost time, not the lock [inference].
- **To wait for the helper**, read the push's **stderr** through a pipe: `git push … 2>&1 | logger`.
  The helper inherits only git's stderr; its stdin and stdout are git's own protocol pipes
  [inference from how git starts a remote helper; not read here]. A pipeline returns only once
  every process holding its write end has exited [measured P11b, with `sh` holding stdout]. Keep
  the push's failure visible: `set -o pipefail`, or test `PIPESTATUS[0]`, since the pipeline's own
  status is the logger's.
- **`timeout --kill-after`:** it fires only while the command it watches is still running. With
  `git push` as that command it never fires, because git has exited. With a wrapper that waits
  for the helper (a subshell running the pipeline above), it fires on the wrapper and SIGKILLs
  the whole group, helper included. Then `--kill-after` must be 75 s or more [measured P7 at 3 s;
  inference].
- **Exit codes:** a script sees git's code, or `timeout`'s 124, not the helper's (S11).
- **Anyone:** the lock-held message can appear for a few seconds after a signal, while the old
  helper is still releasing. Its manual remedy applies only once no push for that repo is running.
  This was already true of a slow, normal release.

## 6. Testing (hermetic)

Three layers. No account and no network. The fake CLI is the existing `testcli` shim served from
temp folders. Timing constants are package variables so tests can shrink them, as with `waitDelay`
today. New tests run with `-count=1` (brief-checklist rule 5).

**Layer 1: units (Fake transport, `internal/repo`).**
- The lock is registered before `CreateExclusive` is called (a Fake hook looks at the register
  during the call).
- Registration after each `CreateExclusive` result, per §5.4: `Refused` with no error and a CLI
  that never started drop it; an error, `Ambiguous`, an unrecognised outcome, a read-back error and
  a foreign read-back keep it. `Refused` adds no calls (pinned by a call count).
- Each §5.5 outcome, from the remote state that produces it. A release through a different
  transport removes our nonce and leaves another nonce alone.
- S10: a Fake that fails N times then succeeds ends *removed* within the cap. *unreadable* is not
  retried. A Fake that never succeeds ends at the cap with *failed*. A call that hangs is cut at
  the per-call bound, and the next attempt still runs.
- The register is reset between tests, and holds no lock across a call (a Fake call that reads the
  register from inside does not deadlock).

**Layer 2: the CLI child (`internal/transport`, the existing re-exec roles).**
- Once the gate is shut, `run` starts no child (a role writes a marker per start) and does not
  return; a call whose child exits after the gate shut does not return either.
- A child that exits within the grace gets no kill. A child that hangs is sent SIGTERM (Unix) and
  then killed. A child that ignores SIGTERM is killed. Each exit is confirmed.
- A child that exits while a grandchild still holds its output pipe: the exit is confirmed at once,
  and the drain runs on for `waitDelay`, with today's warning (S13). The normal path's behaviour
  through a WaitDelay, `TestVersionSucceedsThroughAWaitDelay` among others, is unchanged.
- The coordinator's instance kills a hung call at its bound, and starts its children in their own
  process group.

**Layer 3: end to end, with real OS signals (`cmd/git-remote-proton`).** The helper runs as a
re-exec'd test binary with the shim on PATH, driven through the protocol over pipes. The helper's
role and the shim's are told apart by something the helper does not pass on, such as the
executable's base name (`proton-drive`) or a role variable the helper clears before it starts
children; today the shim role is an inherited variable, which the helper would also obey. The
shim gains block rules: one or more of *(verb, leaf, once or every time, before or after writing
its file)*. A blocked call writes a ready file and waits. One more rule simulates a server that
applies a request late: after the shim is killed, a detached writer it started puts the file in
place after a set delay.

Each signal case asserts the exit code, the state of `.lock` in the shim tree, that no shim
process survives, and the result line. The non-signal cases (E5, E6) assert their own outcome.

| # | Case | Platform |
|---|---|---|
| E1 | SIGTERM, SIGINT, SIGHUP to the helper alone, blocked in the pack upload with the lock held: *removed* | Unix |
| E2 | SIGTERM to the helper's process group, like `timeout`. The helper is started in its own group, so the test does not signal itself | Unix |
| E3 | A signal while the helper is idle, waiting for the push batch after `list for-push` | Unix |
| E4 | A signal during the lock upload, blocked before the shim writes `.lock` (*absent*) and after (*removed*) | Unix |
| E5 | git dies mid-batch: the test closes the helper's stdout and stdin, then unblocks the shim. The helper is not killed by SIGPIPE, releases on the EOF path, and exits 1 | Unix |
| E6 | SIGHUP ignored at start (a `trap '' HUP; exec` wrapper): SIGHUP has no effect, and the push completes and releases normally, exit 0 | Unix |
| E7 | `--set-head`, signalled during its `HEAD` upload | Unix |
| E8 | A fetch, signalled during a download: prompt exit, no lock created, child gone | Unix |
| E9 | A second signal during a slow release is ignored, and the release completes | Unix |
| E10 | A release whose `trash` blocks every time ends at the shrunk cap with *failed* and S11's code. `.lock` remains | Unix |
| E11 | A signal during `Bootstrap`'s marker upload: exit, no lock created | Unix |
| E12 | A signal during `EnforceCertified`'s `--version` call: exit, child gone | Unix |
| E13 | A signal that lands as `loop` returns normally (a seam holds `main` between `run()`'s return and `os.Exit`): the coordinator, not `main`, ends the process, and the lock ends *removed* or already gone | Unix |
| E14 | Another process's lock is in place; a signal during `AcquireLock`: *foreign*, `.lock` untouched | Unix |
| E15 | A late-landing lock upload inside the settle wait: *removed*. One landing after it: the result says *absent* and names the nonce | Unix |
| E16 | Helper-only SIGTERM while the shim ignores SIGTERM: the shim is killed, then the release runs | Unix |
| E17 | stderr not read by anyone (a full pipe): the helper still exits by the cap | Unix |
| W1 | Ctrl-Break by `GenerateConsoleCtrlEvent` to a helper started with `CREATE_NEW_PROCESS_GROUP`, blocked in the pack upload. The shim child is in the same group and gets the event too, as with a real Ctrl-C: *removed* | Windows |
| W2 | As W1, then a second Ctrl-Break to the helper's group during the release: ignored, and the release child, in its own group, finishes | Windows |

W1 and W2 use the method measured in P4, which worked from both the PowerShell tool and Git Bash,
neither of them an interactive console. If `GenerateConsoleCtrlEvent` fails because no console is
attached, the test **skips with that reason** instead of failing. The plan must then confirm from
the `windows-latest` CI log that W1 and W2 **ran**. If they skipped there, that is reported to
Craig, not absorbed. The Unix cases run in the Linux CI legs and in the WSL distro.

Existing tests stay green. On the paths no signal reaches, the CLI-call counts stay the same
(anti-goal 3), except S8's in-doubt paths.

## 7. Documentation to change

- **v2 design doc:** rewrite "Concurrency posture" item 5 to say what is released and what is not
  (§2's table in short). Change the error table's "cancellation, timeout" row to name the signal
  behaviour. Add revision **v6.10**, recording S1–S18 as approved, with the options not taken.
- **README "Honest limits":** what releases the lock on each platform, that uncatchable kills still
  need the manual remedy (which stays documented), and the "no result line" state of §5.6.
- **`docs/research/gates/brief-checklist.md` rule 9:** add a note that it stays in force. On
  Windows, a harness kill through Git Bash is uncatchable [measured P1].
- **The Linux unattended spec and its README section:** §5.8's rules (KillMode, `TimeoutStopSec=`,
  `--kill-after`, waiting on stderr with `pipefail`), and narrowing its "a stalled push leaks the
  lock" row to SIGKILL and the other uncatchable kills. Whichever of the two lands second updates
  the other. The sentence to rewrite when this ships is that spec's "That stays true until the
  helper releases its lock when killed (U6's follow-up)."
- **CHANGELOG** entry for the release that carries it, saying plainly that on Windows only Ctrl-C
  and Ctrl-Break in a console window are covered, and that S1's kind of kill still leaks.
- **`docs/README.md`:** the index already links this spec; add the plan when it exists, and update
  the spec's status line on approval.

## 8. Release

The change ships in the next helper release, under `docs/releasing.md`, in order. `.go` files
change, so a stand-in is not eligible, and both formal live gates run. Each gate brief adds one
live check on its own scratch remote: kill a push mid-upload with that platform's catchable route,
then confirm `.lock` is gone and the next push succeeds. Linux: SIGTERM to the push's process
group. Windows: Ctrl-Break by `GenerateConsoleCtrlEvent`, the method P4 showed working from the
agent's non-interactive shells, or Ctrl-C by hand. That check is the first evidence for U1–U3 and
U7 in §10. The version number is left to release time. A behaviour change suggests 0.11.0 (the
agent's suggestion). The CLI pin does not move.

## 9. Left to the builder

- API names and file layout. A small `cmd/git-remote-proton/shutdown.go` for the coordinator is the
  suggestion. The gate, in-flight tracking and the release instance's constructor go in
  `internal/transport/cli.go`; the register and the typed release in `internal/repo/lock.go`.
- How `run` creates its pipes and bounds the drain, within S13: the parent closes its copies of the
  write ends after the child starts, or EOF never comes.
- The exact constants, within these limits: S5's waits, S10's pause and per-call bound, S17's
  settle, and the 60 s cap. Changing the cap means changing §5.8's 75 s figures and the docs in §7.
- Message wording, within §5.6.
- Whether the release instance is wrapped in `transport.NewTraced` (the suggestion: yes, so traces
  stay consistent).

## 10. Untested, and what stays at risk

- **U1** [untested] How the real `proton-drive` child reacts to SIGINT, SIGTERM and Ctrl-Break, and
  how fast it exits. It is a Bun-built program [record: the Linux probe's `com.oven-sh.bun.Secret`
  schema], so Bun's defaults apply unless the CLI installs handlers.
- **U2** [untested] Whether a killed CLI leaves local state (its SQLite caches, Linux
  `events.lock`) that makes the release's first call fail. S10's retry covers a passing failure; a
  lasting one would leave the lock, said loudly. Probing it needs a signed-in CLI working against
  the account, so it waits for the gate check in §8.
- **U3** [estimate] Release duration. The figures in §5.6 come from one record's timestamps. The
  gate check in §8 measures it.
- **U4** [inference] Task Scheduler's stop and time limit terminate without a console event.
  Probing this would need a scheduled task, which is a persistent change, so it was not run.
- **U5** [inference] On Linux, a terminal Ctrl-C typed after the prompt returns does not reach the
  helper.
- **U6** [untested] Ctrl-C in Git Bash's mintty window. P1 and P2 suggest MSYS sends native
  programs no console event, so §2 assumes it leaks.
- **U7** [untested] How long after a killed request the server can still apply it, which S17's
  5 s settle wait is meant to cover. Also untested: whether a release child in its own Windows
  process group (S7) behaves normally.
- **U8** [inference] That git gives a remote helper its stderr and keeps the helper's stdin and
  stdout as its own pipes (§5.8's stderr rule). This was not read from git's source here.
- **Residual, by design:** every uncatchable kill in §2 still leaks. That includes S1's own route
  on Windows. The manual remedy and rule 9 remain the answer.

## Revisions

Scaffolding for the review loop; delete it before the plan is written.

- **Round 1** (Codex, Gemini via agy, and DeepSeek reported; Kimi INCOMPLETE, canary missing; its
  findings were checked against the code before use). Applied: confirmed process exit instead of
  "kill delivered", with `run` reworked so exit and output drain are separate (S5, S13); a settle
  wait before reading the lock, for requests that land after the child dies (S17); a killed lock
  upload, which returns an error, keeps its registration (§1.4, §5.4, S8); typed release outcomes,
  so no false "released" (§5.5, §5.6, S8); the "refs reported ok" claim removed (statuses are
  flushed only after the batch); a process-wide gate with one exemption, and the register placed in
  `internal/repo` (S14, S15); the protocol goroutine parked after a child returns too, and its
  warnings suppressed (S3, §5.1); an independent cap timer, gate before any stderr write (S6,
  §5.3); per-call bound and retry categories for the release (S10); release children in their own
  process group (S7); a failed protocol write exits 1 (S16); a cut-short sequence for Windows
  console close (S18); the logoff/shutdown row and the mintty case in §2; the stderr-pipe and
  `pipefail` rules and `--kill-after` sizing in §5.8; who sees S11's exit code; the goals reworded
  to "no new remote write starts"; tests E11–E17 and W2, a shim that can block more than once and
  can land a file late, helper and shim roles told apart, and no `Notify` leak from in-process
  `run()` calls; §7's index wording, the Linux sentence pinned, the Windows caveat in the
  CHANGELOG. Rejected: probing U1/U2 before coding (it needs the signed-in CLI and the account,
  which this task must not touch; the gate check in §8 is the first safe point); starting the
  coordinator on a broken stdout (S16 gives the reason); and the note that S3 overstates the
  context plumbing (gitcmd's one context is local to its probe, so the plumbing S3 describes is
  still needed).

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
handler can run; Windows sends the last two only to services); Microsoft's HandlerRoutine (its
timeouts table: close 5000 ms, Ctrl-C/Break no timeout; logoff and shutdown reach only services)
and "CTRL+C and CTRL+BREAK signals" (passed to all processes attached to the console); systemd.kill
(`KillMode=`; `control-group` is the default).
