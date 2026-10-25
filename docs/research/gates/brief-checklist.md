# Gate-brief standing checklist

Standing rules every future live-gate brief incorporates by reference. These are not
per-release content — they are procedural rules a gate brief and its runner must follow
regardless of which stage or release they gate. Established by practice across the Stage 3b
and Stage 4 live gates; write new gate briefs against this checklist rather than re-deriving
the rules from scratch.

1. **Listing equality is asserted on the row set, never the serialised JSON.** Compare
   `filesystem list` output field-by-field on the row set (`uid`, `name`, `type`,
   `creationTime`, `modificationTime`, `parentUid`) — not by comparing the raw JSON blobs for
   byte equality. `filesystem list` order is unstable (observed directly in Stage 4 run 2,
   where a naive whole-string comparison reported `False` on two otherwise-identical listings).

2. **Write confinement must be named explicitly per brief**, including whether parent creation
   outside the repo root is authorised. A brief that is silent on this leaves the runner to
   guess whether an `EnsureDir`-style failure may be remedied by creating the missing parent,
   or must instead be reported as BLOCKED.

3. **Verify-before-trash, with full subtree enumeration.** Before issuing any `trash` command,
   enumerate the full subtree being trashed and confirm it contains only this gate's own
   artifacts. Never trash a node — or a folder — without having first listed everything beneath
   it.

4. **Report BLOCKED with verbatim output, never patch.** When a step fails, the gate stops and
   reports the failure with the command's verbatim output. The runner does not patch code,
   config, or account/environment state to make a blocked step pass.

5. **`-count=1` on every `go test` invocation.** Every hermetic `go test` run in a gate context
   passes `-count=1`, so cached results never stand in for a live run.

6. **Empty the trash before the gate, as cheap insurance.** Starting each gate run with an
   empty trash is a low-cost precaution that keeps later verification (e.g. confirming no
   unexpected `trashTime` appears on a listing) unambiguous.

7. **Trash accounting must count folders the run's prune operations trashed, not only files.**
   When tallying what a gate run trashed for the record, include folders that prune operations
   sent to trash — a files-only count understates what actually left the account's live tree.

8. **State the contract table's live root in the brief, and include it in the confinement
   list.** `internal/transport/contract_test.go`'s `TestContractCLI` writes under
   `GPB_CONTRACT_LIVE_ROOT` if set, else the `liveRoot` const
   (`/my-files/_cas-probe/contract`) — whichever applies must be named explicitly and folded
   into the brief's confinement list (rule 2), not left implicit. Established by Stage 5's S2
   (`docs/research/gates/stage5-gate.md`): the gate's authorised confinement did not include
   the hardcoded `liveRoot`, so the table refused to run under the gate's own root — a defect
   in the brief, not the product.

9. **Run pushes with a long tool timeout, or in the background.** A harness's default tool
   timeout can kill a `git push` mid-flight; the push itself keeps running against the account,
   so the runner is left diagnosing and clearing an orphaned remote lock rather than seeing a
   clean failure. Established by Stage 5's S1 (`docs/research/gates/stage5-gate.md`): a
   2-minute harness timeout killed the bootstrap push mid-flight and orphaned the `.lock`,
   which then had to be diagnosed and cleared by hand before the push could be re-issued. The
   rule stays in force after the helper learned to release its lock on a catchable signal
   (design v6.11), because a harness kill through Git Bash on Windows ends the helper without
   running its handler (measured, probes P1 and P2 of
   `docs/superpowers/specs/2026-10-24-v2-lock-release-on-signal-design.md`), so an S1-style leak
   can still happen.

10. **A hermetic audit of a staged CLI binary may run its `--version` and help output
    unauthenticated without asking, under the account's existing telemetry setting.** The
    exposure is the CLI's own version check against Proton's feed and whatever its telemetry
    reports under that setting — which the installed CLI already does on every scheduled run —
    and no account data is touched. Established by the 0.9.0 certification
    (`docs/research/gates/cli-0.9.0-certification.md`), whose Task 1 ran under the plan's
    "proceed" default; Craig confirmed it retroactively as a standing ruling
    (`v0.9.0-release-gate.md`, post-release rulings, 2026-10-11). Certification plans need not
    put the telemetry question to Craig again.

11. **Hermetic test runs in WSL keep Windows interop off, and cannot reach a real `proton-drive`.**
    A hermetic test run (`go test`, Pester) in WSL sets `/etc/wsl.conf` `[interop] enabled=false`
    and `appendWindowsPath=false`, and then either has no `proton-drive` on PATH except a test
    shim, or runs in a network namespace with no route out. With WSL's default interop on, the
    real, signed-in Windows `proton-drive.exe` is on the Linux PATH and executes, so a hermetic
    test could reach the account. Established by the 2026-10-15 F14 probe (finding F14 of
    `docs/superpowers/specs/2026-10-15-v2-linux-port-design.md`): with interop off, the Windows
    exe exits 126. A rule-10 audit of a staged CLI binary's `--version` or help output is not a
    hermetic test run, and stays governed by rule 10. (A live gate in WSL asserts the same
    interop-off precondition: spec §7, step 0.)
