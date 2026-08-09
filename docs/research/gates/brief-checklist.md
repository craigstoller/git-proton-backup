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
