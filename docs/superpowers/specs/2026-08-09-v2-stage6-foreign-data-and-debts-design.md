# v2 Stage 6 — foreign-data availability at the read boundary, and the Stage 5 debt ledger

**Status:** DRAFT for peer review, brainstormed with Craig 2026-08-09. Decisions below were made
interactively and are recorded with their rationale; do not re-litigate without new evidence.
**Baseline:** `main` @ `96dd4a5`, v0.4.0 published, Stage 5 live gate passed in full
(`docs/research/gates/stage5-gate.md`, including the 16/16 live contract-table supplement), CI green.
**Normative design:** `docs/v2-remote-helper-design.md` v6.5. This stage produces v6.6; the
required edits are enumerated in component 4.
**Release:** Stage 6 ships as **v0.5.0**, draft-until-gated, same three-asset set and digest
closure as Stages 4–5.

## Scope decided in brainstorm (Craig, 2026-08-09)

Stage 6 is the **design-debt stage**: it closes the one open behavioural decision Stage 5
escalated, plus every small contract/infra debt in the Stage 5 ledger. Compaction was considered
and deliberately parked for its own stage (it is a full design of its own; Stage 5 showed 14
tasks is the comfortable upper bound and this stage must stay well under it).

The motivating decision, argued through failure scenarios in-session: **this is a backup tool,
so the local repo is the source of truth**. Writes flowing (backups happening) is the product's
purpose; a trivially-produced foreign file that silently stops unattended backups is a worse
data-protection failure than a loudly-skipped, never-yet-observed corrupt ref. Availability wins
at the *survey* boundary; strictness is kept at every *write* and *directly-addressed* boundary.

1. **Content-skip at the read boundary** — a well-named ref file whose contents are unparseable
   is skipped with a loud note, never fatal (closes the v6.5 OPEN question in the availability
   direction).
2. **Hardened push-side diagnosis** — pushing onto an unparseable occupant names what is
   actually there, with the delete remedy, instead of "ref changed concurrently".
3. **Contract/infra debts** — F1 (directory download) becomes a contract row + Fake fidelity;
   `TestContractCLI`'s live root becomes configurable; the install.ps1 mock honours
   `DoNotExpandEnvironmentNames`; the small triaged leftovers.
4. **Docs** — design doc v6.6; README; CHANGELOG.
5. **One small live gate**, v0.5.0 draft-until-gated.

---

## Component 1 — Content-skip at the advertisement boundary

Stage 5's read boundary has two outcomes per discovered file: invalid *name* → skip-with-note;
well-named but unparseable *contents* → fatal for the whole advertisement. Stage 6 merges them
into **one foreign-data rule**: nothing foreign can stop the world, and nothing foreign is ever
touched.

- **Rule:** during the `ListRefs` walk, a candidate file whose name passes validation but whose
  contents fail the exact ref grammar (40 lowercase hex + `\n`, exactly 41 bytes) is **skipped
  with a loud per-file stderr note** — never advertised, never touched, never fatal. All other
  discovered refs advertise normally.
- **Mechanism:** the existing `skipNote` helper and note grammar —
  `git-remote-proton: skipping <root>/<path>: <reason>` — with a content reason naming the
  grammar and a bounded preview: `contents are not a ref (want 40-hex+LF, got <=64-byte
  preview>)`. Same code path as name-skips, so note emission stays covered by the existing
  mutation-verified emission tests, extended with content cases.
- **Indistinguishability is accepted and stated:** the helper cannot tell foreign junk
  (`Thumbs.db`, a web-UI drop) from a corrupt real ref. The scenario analysis adjudicated this:
  a corrupt real ref has never been observed in five stages of live evidence; its recovery under
  the old fatal rule was already "delete the file and re-push from local", which the skip rule
  preserves — and component 2 makes the corrupt-ref case surface actionably on the next push to
  that name, which for a backup remote is usually the next backup.
- **What stays fatal, deliberately (three carve-outs):**
  1. **Transport/`List` failures during the walk.** A *failed read* is not a *readable
     non-ref*; skipping on a failed observation would misrepresent state never seen. The walk's
     existing fail-closed error propagation (with the failing folder named) is unchanged.
  2. **`--set-head` to a corrupt target.** The user named that branch; the exact-path read
     errors with the bounded preview rather than pretending absence. (Unchanged from Stage 5;
     restated here because the boundary rule differs on purpose: advertisement is a survey and
     degrades per-ref; a direct address is a demand and stays strict.)
  3. **`WriteRef`'s read-back verification.** v2 verifying bytes it just wrote; tolerance there
     would mask real write corruption.
- **Interaction with fetch/clone:** a skipped ref is simply absent from the advertisement; git
  fetches what is advertised. A disaster-recovery clone therefore succeeds and lacks the
  skipped branch, with the note on stderr. This residual — *informed* loss under the old fatal
  rule versus *possibly-unnoticed* loss under skip — was examined explicitly and accepted: the
  fatal rule's remedy produced the same clone content anyway (the corrupt tip was unreachable
  either way); its only advantage was forced awareness, which component 2 substantially
  recovers at push time.
- **Interaction with push preflight:** a skipped name is absent from the caller's remote map,
  so a push to it classifies as a *create* and meets the occupant at `CreateExclusive` —
  fail-closed, no overwrite, diagnosed by component 2. The final-state D/F preflight is
  unaffected (it operates on the advertised ref set, as before).

## Component 2 — Push-side diagnosis of an unparseable occupant

Today the sequence *skip → push to that name → create → `CreateExclusive` → `Refused`* surfaces
as "ref changed concurrently; refusing to overwrite" — a wrong diagnosis for a state the helper
itself observed at advertisement.

- **Where:** the existing create-heal wrapper (`createRefHealingCollision`), in the arm where
  the post-refusal `Stat` shows a **file** (not a folder). Today that arm surfaces the original
  refusal unexamined.
- **Rule:** in that arm, read the occupant (one extra download, refusal path only — same cost
  posture as the heal path's diagnostics). If its contents parse as a valid ref, keep the
  existing concurrent-creator message (it is true). If they do not, report what is actually
  there: `a file occupies <ref> but its contents are not a ref (<bounded preview>); it was
  skipped at advertisement; delete it first (proton-drive filesystem trash <path>, or the web
  UI)` — the CLI remedy in its real grammar, per the component-6 precedent from Stage 5.
- **The write boundary stays fail-closed in every arm.** Nothing is overwritten, nothing is
  auto-deleted; the helper never "heals" a file occupant (heal remains folders-only, exactly as
  shipped). If the diagnostic read itself fails, the original refusal stands with the read
  failure noted — the same only-heal-on-positive-evidence posture as the folder heal.
- **Updates are unaffected:** a skipped ref is never in the remote map, so the update arm
  cannot target it; only the create arm needs the diagnosis.

## Component 3 — Contract and infrastructure debts

- **F1 becomes a contract fact with teeth.** Stage 5's gate observed live that `filesystem
  download` on a **directory** exits 0 and recursively downloads the subtree (it does not
  error). New live contract row pinning exactly that shape, and `Fake.ReadTo` taught the same
  behaviour so the fake and CLI halves agree — this is what makes `--set-head`'s
  Stat-IsDir-first guard (Task 10) permanently regression-safe instead of guarded by a comment.
  The shim's `runDownload` doc comment gains the same fact with a citation to the row (it may
  keep its file-oriented behaviour; the comment states the divergence).
- **`TestContractCLI`'s live root becomes configurable.** New env var `GPB_CONTRACT_LIVE_ROOT`,
  read by the contract table only, default unchanged (`/my-files/_cas-probe/contract`). Gate
  briefs can then point the table anywhere inside their authorized confinement — killing the
  Stage 5 S2 class (a brief whose confinement list and the table's hardcoded root disagree).
  The standing brief checklist gains one line: state the table's root and include it in the
  confinement list.
- **The install.ps1 mock honours `GetValue`'s `$options`.** The Task 2 mock currently returns
  the raw stored value regardless of options, so a regression that dropped
  `DoNotExpandEnvironmentNames` from install.ps1's real registry read — the bug that would bake
  an expanded `%USERPROFILE%` into the user PATH permanently — would pass the suite. The mock
  gains options-awareness (raw value only when the flag is passed; expanded otherwise) plus a
  RED case pinning the flag's presence.
- **Small leftovers, all triaged OK-TO-DEFER in Stage 5 and collected here:** gate-runner
  long-timeout guidance added to `docs/research/gates/brief-checklist.md` (pushes run with a
  long tool timeout or in background — the S1 lesson); a three-line test for
  `EnsureParents`' mount-is-a-file branch (`parents.go`, the one untested arm); no other code
  changes ride along.

## Component 4 — Docs (v6.6)

One revision entry, edits in place per house style:

- **Error table:** the Stage 5 "malformed discovered *contents*" rows change verdict — at the
  advertisement boundary: skipped-with-loud-note, never advertised, never touched; on direct
  address (`--set-head` target, read-back): fatal exactly as before. The reach/remedy row
  Stage 5 added is updated to match (the remedy stays: delete the file; the consequence
  shrinks from "all reads fail" to "that name is not advertised").
- **The v6.5 OPEN question is closed** with the adjudicated rationale (backup tool; survey vs
  direct-address boundary split; scenario D residual accepted and named), replacing the
  open-question paragraph with the decision and a pointer to this spec.
- **F1 recorded** as a verified contract fact beside the ReadTo/C16 material, with the
  directory-download row cited.
- **Component-2 diagnosis** added to the error table (unparseable occupant on create).
- **README:** two lines — foreign files under `refs/` are skipped loudly and never touched;
  pushing onto one names it and gives the remedy. `GPB_CONTRACT_LIVE_ROOT` is NOT documented in
  README (test-infrastructure var, not a user surface); it is documented in the contract test's
  own comment and the brief checklist.
- **CHANGELOG:** `Unreleased` entries as components land; the version flip is release-prep
  work per `docs/releasing.md`, exercised for the second time.

## Component 5 — Testing and the live gate

**Hermetic (the Stage 5 discipline unchanged: TDD, RED/GUARD labels, which-assertion-fired,
`-count=1`, deliberate-regression checks for the load-bearing behaviours):**

- Content-skip: junk file beside good refs → map contains the good refs only, note asserted on
  captured stderr (mutation-verified: remove the skip call, watch the assertion fire); nested
  junk (deep in the tree) skips without disturbing siblings; the exact-grammar boundary
  fixtures (no-LF, CRLF, double-LF, 40-hex-exact) now SKIP at advertisement instead of
  erroring — the Stage 5 fatal-pinning tests flip to skip-pinning tests with their fixtures
  preserved.
- The disaster-recovery-clone shape: a full clone via the protocol loop with a junk file
  present succeeds; all other refs materialize; the note appears on stderr.
- Push-diag: unparseable occupant → new message (with preview and remedy); parseable occupant
  → concurrent-creator message unchanged (GUARD); diagnostic-read failure → original refusal
  stands with the failure noted.
- Direct-address carve-outs: `--set-head` to a corrupt target still fatal (GUARD, existing
  test retained); read-back verification untouched (existing tests).
- F1: Fake.ReadTo-on-directory fidelity test + the live row's fake half.
- Mock options-awareness: RED case proving a dropped `DoNotExpandEnvironmentNames` now fails.

**Live gate (one, small — roughly a third of Stage 5's):**

1. Manufacture a junk file in the gate repo (CLI upload of a non-ref file to a valid ref name —
   the web-UI-equivalent action), then: `git ls-remote` succeeds and omits it (note observed
   verbatim); clone succeeds with all real refs; push of an unrelated ref succeeds.
2. Push onto the junk name: the component-2 message observed verbatim (BLOCK on mismatch, the
   Stage 5 signature-pin discipline).
3. Contract table live via `GPB_CONTRACT_LIVE_ROOT` pointed inside the brief's confinement —
   including the new F1 row.
4. Hierarchical smoke: nested branch push/fetch round-trip (no regression).
5. Cleanup with verify-before-trash; row-set comparisons; trash accounting; the brief follows
   `docs/research/gates/brief-checklist.md` including its new timeout guidance, and its
   confinement list names the contract root explicitly.

**Release:** v0.5.0 draft-until-gated; tag only after CI green on main (the Stage 4 dead-tag
lesson, now standing); three assets; publication digest closure. All Craig-directed at the
stop points, per the runbook.

## Out of scope (unchanged verdicts)

Compaction (deliberately parked for its own stage — the `RevListNewObjects` memory note stays
parked with it), PowerShell Gallery publishing, non-Windows support, multi-writer,
shallow/partial clone, SHA-256 repos. Also out: any auto-deletion or quarantine-move of foreign
files (the foreign-data rule is observe-and-report, never touch), and any attempt to make a
clone fail on skipped names (the helper protocol has no channel for it; the note is the
mechanism).

## Execution notes for the plan (binding)

- Task order: content-skip before push-diag (2 reads 1's skip state); F1/liveRoot/mock/leftovers
  are independent and may interleave; docs after all behaviour lands; release prep + gate brief
  last with an explicit STOP for Craig.
- Plan-supplied code blocks are hypotheses (three SUPERSEDED banners in Stage 5, five in
  Stage 4); SDD execution in a fresh session whose controller reads `v2-sdd-runbook.md` first
  and deletes the Revisions blocks from spec and plan before dispatch; commits carry the Opus
  co-author trailer; nothing pushed without Craig's word; hermetic tests only outside the gate;
  every `go test` with `-count=1`.

## Decisions log (brainstorm, 2026-08-09)

| Decision | Choice | Rejected alternatives |
|---|---|---|
| Stage 6 anchor | Design debts (fatal-content decision + Stage 5 ledger) | Compaction now; usage-driven feature; minimal cleanup |
| Fatal-content rule | Skip-with-note at advertisement + hardened push diagnosis | Keep fatal with better UX; namespace-dependent middle ground |
| Strictness boundary | Survey degrades per-ref; direct address and writes stay strict | Uniform skip everywhere; uniform fatal everywhere |
| Scope shape | Full debt ledger rides along, one small gate, v0.5.0 | Decision-only minimal; debts + compaction start |
| Foreign-file handling | Observe and report only, never touch | Auto-trash; quarantine-move |

## Revisions

*Scaffolding for the review loop; delete before the spec is treated as final.*
