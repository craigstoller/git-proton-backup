# git-remote-proton Stage 4 — productization

**Date:** 2026-08-04. Brainstormed with Craig at Stage 4 open, immediately after the C17b
provocation probe (commit `beb23b3`) returned NOT REPRODUCED — that result shaped this scope.
Baseline: Stage 3b merged (`349ce54`), selective fetch live-gated with measured selectivity.
Design doc at v6.3 (`docs/v2-remote-helper-design.md`).

## Goal

v2 becomes daily-usable. Five deliverables: (1) certified-CLI enforcement with an explicit
escape hatch, (2) a `--set-head` operation closing the only user-facing feature gap (a remote's
default branch cannot be changed in-tool today), (3) a tag-triggered CI release pipeline with a
real install story, (4) v1-coexistence documentation, (5) the deferred test/hygiene debt paid.

No fetch-path behaviour changes — the hygiene wave touches error wrapping, error ordering and
helper-function dedup there, nothing that alters what is downloaded or when. Success is the
Stage 4 live gate passing end to end.

## Decisions taken here

- **Stage framing.** Productization now; hierarchical refs get their own stage (Stage 5), and
  the retro-Codex quarantine-staging refactor rides that stage's fetch-path touch, not this one.
- **EnsureDir already-exists tolerance is DROPPED**, not deferred. C17b could not provoke the
  gate-run-1 failure in 30 trials across three topologies (probe log
  `docs/research/probes/c17b-provocation-log.md`; C17 amended to observed-but-unprovoked), so a
  tolerance could not be validated live and its confirm step's semantics would be guesswork. The
  failure stands at n=1 unexplained; the mitigation is operational (trash hygiene, documented in
  README and the gate procedure) and the item returns only on an n=2 in the wild — for which the
  README names the evidence to capture before clearing anything (see component 4).
- **The helper keeps the name `git-remote-proton`.** v1 ships no binary at all (a PowerShell
  module; its `proton` remote is a local mirror path, no helper scheme), so there is no
  collision; the name is derived from the `proton::` scheme and baked into the marker's
  exact-match `format` seam. Coexistence is a docs problem.
- **Allowlist policy: refuse-by-default with a loud explicit override** (Craig's pick over pure
  no-override refusal and over warn-only). Certified list starts as exactly
  `cli-drive@0.7.0+5174900c` — the build every probe and gate ran. Exact versions, not a floor
  or prefix: C16 and C13 are the evidence that behaviour differences between builds are real and
  silent, which is exactly what a prefix match would wave through. The version-tolerant
  `parseNodeJSON` (0.4.6 + 0.7.0 shapes) is KEPT: the allowlist gates the front door; parser
  tolerance is defense in depth behind it. This resolves the design/code contradiction open
  since Stage 2 in favour of front-door enforcement, and the design doc is updated to say so.
- **The allowlist is a compatibility gate, not a provenance check.** It defends against
  accidental drift — an auto-updated or hand-upgraded CLI silently changing semantics — not
  against a hostile binary on PATH. A spoofed `--version` defeats it trivially (the gate's own
  shim test proves so); that is out of the threat model, because a helper that trusts the CLI
  with every byte of repo data has no meaningful defence against a malicious one anyway.
- **`--set-head` ships as a direct-invocation utility mode**, not a protocol extension — remote
  helpers have no protocol verb for it, and the user (not git) is the natural invoker.
- **Utility-mode dispatch matches a closed set, not a prefix.** `os.Args[1]` equal to
  `--set-head` or `--version` — checked before any protocol I/O — selects utility mode;
  everything else takes the protocol path unchanged. A prefix match (`--*`) would misroute a
  remote whose configured name begins with `--`; with the closed set, only a remote literally
  named `--set-head` or `--version` could collide, and those two strings are documented as
  reserved. Utility-mode stdout is permitted: git never invokes a helper with these argv shapes,
  so utility output can never interleave with the protocol stream. `--version` prints to stdout
  per convention; `--set-head` confirms in one line.
- **Release versioning continues the repo's single `v0.x` line** (existing tags `v0.2.0`–`v0.2.4`
  are the module's). Next release `v0.3.0`; the CHANGELOG states that one version line covers
  both tools in this repo. windows/amd64 is the only supported platform — the only one with gate
  coverage. Other platforms are unsupported: they can build from source and the certified list
  carries no platform qualifier, but nothing beyond the hermetic suite has ever run there.
- **Releases are draft-until-gated.** The tag workflow publishes a DRAFT GitHub Release; the
  live gate runs against the draft's exact bytes; Craig publishes the draft only after the gate
  passes. No rebuild between gate and publication, no public window for ungated artifacts.
- **`listCompletePacks` stderr-note alignment (retro-Codex 2) resolves by narrowing the 3b spec
  to the code**, as a third post-implementation reconciliation bullet: only packish-looking skips
  get notes. Rationale: non-packish nodes in `packs/` are foreign junk the helper deliberately
  ignores, and per-node notes would make stderr volume depend on outside actors.

## Components

### 1. CLI version allowlist (`internal/transport`)

The seam is CLI-transport construction (`NewCLI`, `cli.go`): before the first filesystem
command, run `proton-drive --version` once per process and match the CLI identity token against
a compiled-in certified list — initially `{"cli-drive@0.7.0+5174900c"}`. The SDK line in the
version output is recorded in errors/warnings but not matched.

- **Mismatch, unparseable output, or a nonzero `--version` exit → refuse to run**, before any
  filesystem command, with an error naming what was found (or that it could not be determined),
  what is certified, and the override.
- **`GPB_UNCERTIFIED_CLI=1` → proceed**, printing a prominent stderr warning naming the untested
  version on every helper invocation. Never remembered, never cached across processes.
- Unparseable output is just an extreme case of uncertified: the override covers it (the warning
  then says the version could not be determined).
- A missing `proton-drive` binary fails exactly as it does today (spawn error, fail closed) —
  the allowlist adds no new path there. The version read uses the same `cmd.WaitDelay` guard as
  every exec site (Stage 2.1's rule) and bounds how much output it will read.
- The check runs only where a CLI transport is constructed. `--version` constructs none and is
  therefore never gated — the diagnostic for an uncertified CLI must not itself require a
  certified CLI. `--set-head` constructs one and is gated like everything else.
- All of this speaks on stderr and exit codes only; protocol stdout is untouched.

### 2. `--set-head` utility mode (`cmd/git-remote-proton`, `internal/repo`)

`git-remote-proton --set-head <address> <branch>` where `<address>` is anything `CanonicalRoot`
accepts (with or without the `proton::` prefix).

**Branch-name grammar.** `<branch>` is a short name (normalised to `refs/heads/<branch>`) or a
full `refs/heads/…` ref, and the leaf must be a single path component passing the same
`checkStageableLeaf` rules the push path enforces (no `/` or `\` — hierarchical names are
refused naming Stage 5 — no braces, no Windows device names, not empty/`.`/`..`). Anything
outside `refs/heads/` is refused — HEAD points at branches only, matching `WriteHEAD`'s
existing non-branch refusal. Matching against remote branches is exact byte comparison, no case
folding.

Flow: utility dispatch (length-checked: `--set-head` takes exactly two further arguments;
wrong arity refuses with usage on stderr before anything else runs) → `CanonicalRoot` →
allowlist check (fires at transport construction, component 1) → `RequireMarker` →
`AcquireLock` → verify `refs/heads/<branch>` exists remotely (this check runs on EVERY
invocation, before any short-circuit — a `HEAD` that already names a since-deleted branch must
refuse, not report success) → read `HEAD` under the lock — if it already names the verified
target branch, report success and stop (idempotent, no upload; this same read is what makes an
Ambiguous-outcome re-run self-reconciling) → update `HEAD` to `ref: refs/heads/<branch>\n` →
`Release`. The lock is released via defer on
every exit path — the existing "release on every exit path" rule in `main.go`/`lock.go` applies
unchanged, as do Stage 2's stale-lock and holder-nonce semantics. Within the lock,
verify-then-write is serialized against every v2 writer (branch deletion takes the same lock);
non-v2 actors (the web UI, other Proton clients) can mutate anything at any time, which is the
same accepted model every existing operation lives with.

**A new `UpdateHEAD` function carries the overwrite.** `WriteHEAD` is backfill-only by pinned
contract (`TestWriteHEADNeverOverwritesExistingHEAD`) and is not touched. `UpdateHEAD` uses the
same staged-file CAS path pushes use (local basename `HEAD`, `upload -f merge` per probe C14),
with the same closed-set outcome handling as the marker path: Committed = done, Ambiguous =
"re-run to reconcile" refusal, unrecognised = fail closed. Re-run reconciliation needs no
special mode: every run begins with the read-`HEAD`-under-lock step above, so a re-run after
Ambiguous either finds the write landed (reports success) or makes a fresh attempt.

The branch-delete refusal in `push.go` ("change the default branch first") is extended to name
the remedy: `git-remote-proton --set-head <url> <branch>`. `--version` ships in the same
dispatch: prints the ldflags-embedded version plus the certified CLI list.

### 3. Release pipeline (`.github/workflows/release.yml`, `install.ps1`, CHANGELOG)

- **Workflow, tag runs:** triggers on `v*` tags. Steps: checkout, Go toolchain from `go.mod`,
  full hermetic test suite (the live half's loud skip is the expected shape in CI — no Proton
  account exists there), build windows/amd64 with `-trimpath` and
  `-ldflags "-X main.version=<tag>"`, compute SHA256, create a **draft** GitHub Release and
  attach `git-remote-proton.exe`, its `.sha256`, and `install.ps1`. The workflow declares
  least-privilege `permissions: contents: write` explicitly — draft-release creation must not
  depend on repository default token permissions. The draft is published manually by Craig only
  after the live gate passes against those exact bytes; a failed gate means the draft release
  is deleted and the fixes ship under the **next** version number — tags are never moved or
  reused.
- **Workflow, `workflow_dispatch` runs:** build-only dry run — same steps but version
  `dev+<short-sha>`, artifacts uploaded as workflow artifacts, **no release object created**.
- **`install.ps1` extension:** a `-HelperExe <path>` parameter (default: `git-remote-proton.exe`
  beside the script, if present) installs the helper to
  `$env:LOCALAPPDATA\Programs\git-proton-backup\`, verifies it against a sidecar `.sha256` when
  one is present (refusing on mismatch), and idempotently ensures that directory is on the user
  PATH (persisted user PATH; the script cannot mutate its caller's session — it prints the
  refresh instruction and notes that open terminals need reopening). The helper install is
  **decoupled from the module install's already-installed check**: the existing
  module-destination throw (absent `-Force`) must not block a helper-only or helper-upgrade
  run. A locked destination exe (helper
  processes are per-git-command and short-lived, so this is rare) produces a clear error naming
  the fix, not a half-install. No exe found → informative skip; the module install is unchanged
  either way. No auto-build — the exe comes from a release download or a manual `go build`.
- **CHANGELOG:** `v0.3.0` section covering both tools, plus the one-version-line statement.
- Housekeeping: the stale untracked `git-remote-proton.exe` in the repo root is already
  gitignored; Craig deletes the file once the installed copy exists.

### 4. v1 coexistence docs (README)

A dedicated section: the two tools side by side (v1 = bundles into the sync folder, remote
`proton`; v2 = CAS repo via the CLI, remote `proton-v2`). Safe to wire both in one repo. Keep
v2 remotes under a dedicated root such as `/my-files/GitRemotes/`; never point v2 inside
`GitBackups` — `Bootstrap` hard-refuses non-empty unmarked folders, but an empty subfolder
would be adopted.

**Restore contracts, stated honestly:** a v1 bundle restores with nothing but git, from any
device. A v2 restore needs git, the helper installed, and the certified Proton Drive CLI signed
in — three dependencies where v1 has one. This is an honest-limits difference, not a footnote.

**The trash note, scoped:** a push failing `already exists` out of nowhere has been observed
once, unexplained (probe C17). If it happens: before clearing anything, capture the CLI's
`filesystem list`/`info` output for the failing path and note what the web UI's trash shows —
that capture is what turns the standing n=1 into evidence. Then remove from the trash any items
whose names collide with the repo's remote path and retry. The advice is scoped to those
homonyms — not "empty your trash", which would destroy unrelated recoverable files.

### 5. Test/hygiene wave (the deferred strengthens)

All GUARD-class unless a failure reveals a real defect — in which case stop and report, never
silently patch:

1. M1 — route-around trace assert (fetch trace strengthen from the 3b final review).
2. M3 — `HasObject` assertion in the incomplete-pair test.
3. Two-pack pair-refresh fixture (retro-Codex 1): restart-vs-resume of a multi-pack plan,
   currently structurally-but-not-empirically pinned.
4. The 3b-spec third reconciliation bullet for `listCompletePacks` stderr notes (decision above).
5. `ShowIndex`: wrap the `os.Open` error, add the garbled-line test.
6. `ResolveIdxCacheDir` err-ordering fix.
7. `pruneStale` comment documenting the complete-pairs-vs-name-in-listing divergence.
8. `copyFile`/`linkOrCopy` dedup.
9. Trace decorator: size-unknown branch test.

### 6. Docs/design updates

`docs/v2-remote-helper-design.md` → v6.4: the allowlist section (strategy, threat-model
framing, defense-in-depth reconciliation), the `--set-head` operation (grammar, lock and
outcome semantics), the updated delete-refusal remedy, utility-mode dispatch and stdout
rationale. README per component 4. 3b spec per component 5 item 4.

## Error handling — new rows

| Condition | Behaviour |
| --- | --- |
| CLI version not on the certified list | Refuse before any filesystem command; name found vs certified and the override |
| `--version` output unparseable | Same refusal, treated as uncertified; override still applies |
| `proton-drive --version` exits nonzero (binary present) | Same refusal, treated as version-undetermined; override still applies, warning says the version could not be determined |
| `proton-drive` missing entirely | Existing spawn failure, unchanged — not an allowlist path; the override does not synthesize a binary |
| Utility mode invoked with wrong arity (`--set-head` without its two arguments) | Usage refusal on stderr before any other processing; no-argument invocation keeps today's usage error |
| `GPB_UNCERTIFIED_CLI=1` set | Proceed with a prominent per-invocation stderr warning naming the untested (or undetermined) version |
| `--set-head` on a repo with no/invalid marker | `RequireMarker`'s existing named refusal |
| `--set-head` to a branch that does not exist remotely | Refuse; name the branches that do exist |
| `--set-head` on a repo with no branches at all | Refuse: "no branches exist; push a branch first" |
| `--set-head` to a hierarchical (slash-containing) name | Refuse, naming Stage 5 — same rule as push |
| `--set-head` to a non-`refs/heads/` ref | Refuse — HEAD points at branches only |
| `--set-head` while the lock is held | Existing lock refusal (holder nonce named); lock always released on the refusing side's own exit paths |
| `UpdateHEAD` outcome Ambiguous | Refuse with "re-run to reconcile"; re-run re-reads `HEAD` under the lock — already-correct content is idempotent success |
| Unknown `--flag` in argv[1] outside the closed set | Protocol path (it is a remote name, not a flag) — the closed set is the only utility surface |
| `install.ps1` finds no helper exe | Skip helper install with a message; module install proceeds |
| `install.ps1` checksum mismatch | Refuse the helper install, naming both digests |

## Testing

- **Allowlist matrix, hermetic:** certified → pass; mismatch → refusal naming both sides;
  override → warn + proceed; unparseable → refusal; override + unparseable → warn + proceed;
  missing binary → existing spawn failure; nonzero `--version` exit → refusal; nonzero exit +
  override → "undetermined" warning + proceed. Via an injected
  version source or a fake `proton-drive` on PATH — implementer's choice, but the
  certified-pass case must exercise the real parse of a captured genuine `--version` output.
- **`--set-head` matrix on the Fake:** success + HEAD content; overwrite of an existing HEAD
  (the very point of `UpdateHEAD`); same-target idempotence — asserted as **no upload
  performed** (the read-first short-circuit), not merely exit 0; wrong-arity usage refusals;
  short-name normalisation;
  no-marker refusal; unknown-branch refusal naming existing branches; **dangling-HEAD refusal
  (HEAD already names the target but the branch ref is absent — must refuse, not short-circuit
  to success)**; empty-repo refusal; hierarchical-name refusal; tag refusal; lock-held refusal;
  lock released on every refusal path; refs never touched on any refusal path.
- **`UpdateHEAD` outcome arms:** Committed, Ambiguous, and unrecognised-outcome fail-closed —
  mirroring the existing `TestWriteHEAD*` pattern.
- **Dispatch:** git-style `argv` (`<name> <url>`) is unaffected; `--version` prints version +
  certified list without constructing a transport; argv[1] outside the closed set routes to the
  protocol path even when it begins `--`.
- **Push:** the delete-refusal GUARD updated for the new remedy text.
- Wave items each record which assertion fired; deliberate-regression checks (assert the test
  fails with the guard removed) for anything load-bearing, per the runbook.
- Contract table unchanged; `-count=1` mandated on every gate invocation of it.

## The gate

Run 3b's standing rules verbatim (confinement, verify-before-trash, BLOCKED-and-never-patch,
`-count=1`), plus two now-codified pre-steps: record trash state before assuming anything about
it, and start with an emptied trash (Craig empties; the runner never does — and the emptied
trash is variable isolation for the gate, not a claim that users must run this way). Steps:

1. **Stage the artifact first, so every step tests the shipped bytes:** download ALL of the
   draft release's assets (exe, `.sha256`, `install.ps1`) and record each asset's SHA256 in the
   gate log; verify the exe against its checksum; install via `install.ps1`; from a **fresh**
   shell confirm `git-remote-proton --version` resolves from PATH and reports the tag. Every
   subsequent gate step runs through this PATH-installed binary — never a working-tree build.
2. **Allowlist live:** the real CLI passes the check; a PATH-shimmed fake `proton-drive`
   reporting a wrong version proves the refusal and the override (shim confined to the gate's
   temp dir, never installed).
3. **`--set-head` live, end to end:** demo repo under `/my-files/GitRemotes/<demo>` with two
   branches (pushed through the installed helper); set-head to the second; verify `HEAD`
   content via the CLI and that a fresh `git clone` checks out the new default. Then the
   decisive postconditions: deleting the OLD default now succeeds, deleting the NEW default is
   refused, and the refusal names `--set-head` as the remedy.
4. **Publication with digest closure:** gate passed → Craig publishes the draft release → the
   runner re-downloads **every published asset** — exe, `.sha256`, and `install.ps1`, the
   installer being code users run as much as the exe is — and compares each SHA256 to the gate
   log's staged digests. Only that full comparison substantiates "the published bytes are the
   gated bytes".

## Sequencing

Hygiene wave first (small, de-risks everything after), then allowlist, then `--set-head`, then
release pipeline + docs, then the gate.

## Out of scope

Hierarchical refs (Stage 5, with the quarantine-staging refactor); EnsureDir already-exists
tolerance (dropped per C17b); compaction; any fetch-path behaviour changes; PowerShell Gallery
publishing; macOS/Linux builds and support.

## Revisions

**Round 1 (2026-08-04, Codex xhigh + Gemini 3.1 Pro via agy).** Applied: draft-until-gated
releases replacing the rc-tag scheme, with publish-after-gate binding the shipped bytes to the
gated bytes [Both — Codex critical]; `workflow_dispatch` defined as build-only, no release
object [Codex]; closed-set utility dispatch replacing the `--` prefix match [Gemini critical];
explicit branch-name grammar reusing `checkStageableLeaf`, hierarchical names refused naming
Stage 5, exact-byte matching, empty-repo refusal [Both]; defer-based lock release + concurrency
model stated [Both]; allowlist threat-model paragraph (compatibility gate, not provenance)
[Codex]; `--version` never allowlist-gated, stated with rationale [Codex]; restore contract
names the CLI dependency [Codex]; trash advice scoped to homonyms with an n=2 evidence-capture
plan, replacing blanket empty-your-trash [Codex]; install.ps1 attached to the release, checksum
verified by the installer, session PATH + locked-exe handling, fresh-shell gate verification
[Both]; gate step 2 extended to the delete-old/refuse-new postconditions [Codex]; UpdateHEAD
outcome arms, overwrite, and idempotence added to testing [Codex]; missing-CLI and
nonzero-version rows added [Codex]; non-Windows explicitly unsupported [Codex]. Rejected, with
reasons: relaxing the exact-version pin to a prefix [Gemini — contradicts the design's written
rule and the C16/C13 evidence; the override is the pressure valve]; reinstating EnsureDir
tolerance [Gemini — asserted Proton propagation latency is contradicted by C17's 15/15 and
C17b's 30/30 measurements; the decision protocol was agreed with Craig in advance]; code-level
guards against pointing v2 inside v1's folder [Gemini — would hard-code personal folder names
into a public tool; Bootstrap's non-empty refusal covers the accident case]; U+001A control
characters in the file [Codex — verified false: zero matches, nine real UTF-8 arrows; an
encoding artifact of the reviewer's reader].

**Round 2 (2026-08-04, same engines; both opened with "Blockers: none", six majors raised, all
verified real and applied).** Applied: failed gate now ships under the next version — tags are
never moved or reused, closing the mutable-tag defect the round-1 draft-release fix introduced
[Codex]; the gate stages and installs the draft's checksummed exe as step 1 so every step runs
the shipped bytes, and publication closes with a re-download digest comparison [Codex];
`permissions: contents: write` declared explicitly in the release workflow [Codex]; the
nonzero-`--version`-exit row actually added to the error table with override semantics defined
— round 1 claimed it but only put it in Testing [Codex, catching an incomplete round-1 fix];
the `--set-head` flow now reads `HEAD` under the lock before writing, making same-target calls
upload-free and Ambiguous re-runs self-reconciling — the flow line had contradicted the stated
idempotence semantics [Gemini]; utility-mode arity validation and the no-argument invocation
added to dispatch and the error table [Gemini]. Rejected: nothing this round.

**Round 3 (2026-08-04, same engines, final under the three-round cap; both opened with
"Blockers: none", four majors raised, all verified real and applied).** Applied: branch-existence
verification moved BEFORE the idempotence short-circuit — both engines independently caught
that the round-2 read-first fix let a `HEAD` naming a since-deleted branch report success,
violating the error contract; a dangling-HEAD refusal test added [Both]; the publication digest
closure widened from "the asset" to every asset — exe, `.sha256`, and `install.ps1`, the
installer being code users run — with per-asset digests recorded at staging [Codex]; the
nonzero-exit-plus-override case added to the hermetic test matrix (the contract existed but no
test required it) [Codex]; the install.ps1 claims corrected — a child-process script cannot
mutate its caller's `$env:PATH` (it prints the refresh instruction instead), and the helper
install is decoupled from the module install's already-installed throw so a helper-only run
cannot be blocked by a present module [Gemini]. Rejected: nothing this round. Cap reached; all
tracked findings across three rounds are applied or rejected-with-reason above.
