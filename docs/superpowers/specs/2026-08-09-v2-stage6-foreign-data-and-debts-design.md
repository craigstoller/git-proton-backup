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

The motivating decision, argued through failure scenarios in-session and **revised once on
re-examination (Craig-directed, after both round-1 engines independently argued for it):
policy is PER-OPERATION**, because the two directions are different trades with different
people present. **This is a backup tool, so the local repo is the source of truth.**

- **The push direction is the unattended path** — cron backups, nobody reading exit codes. A
  trivially-produced foreign file that silently stops backups is the worst failure this
  product can have. The push-side survey is therefore **tolerant**: unparseable contents skip
  with a classified note, and the occupancy machinery (component 2) refuses collisions
  actionably.
- **The fetch direction is the attended path** — a restore or mirror is exactly when a human
  (or a mirror job that *should* alarm) is present and trusting exit codes. A clone that
  succeeds while silently lacking a branch is a false-success restore. The fetch-side survey
  is therefore **strict**: content-skips make `list` (fetch/clone/`ls-remote`) **fail with an
  error enumerating every skipped path, its classified reason, and the remedy** — which also
  puts a damaged ref's recoverable hex in the blocking error a human must read, not a
  scrollable note.
- **The principled line:** only *valid-named, unparseable-content* files trigger fetch-side
  strictness — they are the class that could be a damaged real ref, so only they can
  represent silent loss. *Name*-invalid files can never be refs git could hold; their skip
  stays non-fatal in both directions (the Stage 5 rule, unchanged).
- The wedge this re-accepts is bounded and attended: a junk file blocks restores/mirrors
  until deleted (the error names it and the remedy; Proton trash makes deletion recoverable);
  cron mirror-fetchers get loud failures, which for a mirror is correct and documented.

1. **Content classification at the read boundary** — a well-named ref file whose contents are
   unparseable is classified and skipped from advertisement with a loud note; **tolerant on
   the push-direction survey, fatal-with-enumeration on the fetch-direction survey** (closes
   the v6.5 OPEN question with a per-operation rule).
2. **Occupancy-aware push** — skipped names participate in the batch preflight and delete arm,
   so collisions refuse *before* pack work with an actionable message; the create-heal wrapper
   keeps a race-window diagnosis.
3. **Contract/infra debts** — F1 (directory download) becomes a precisely-pinned contract row +
   Fake fidelity; `TestContractCLI`'s live root becomes configurable with in-code validation;
   the install.ps1 mock pins the `DoNotExpandEnvironmentNames` flag; the small triaged
   leftovers.
4. **Docs** — design doc v6.6; README; CHANGELOG.
5. **One small live gate**, v0.5.0 draft-until-gated.

---

## Component 1 — Content classification at the advertisement boundary

Stage 5's read boundary has two outcomes per discovered file: invalid *name* → skip-with-note;
well-named but unparseable *contents* → fatal for the whole advertisement. Stage 6 replaces
the content rule with **one classification mechanism and two policies**: the scan classifies
foreign data once; the push-direction survey (`list for-push`) tolerates it (skip + note +
occupancy); the fetch-direction survey (`list`, serving fetch/clone/`ls-remote`) fails on it
with full enumeration. In both directions, nothing foreign is ever modified or deleted by the
helper.

**The scan result is structured, not a bare map.** `ListRefs` (the advertisement walk) returns
three things, distinctly typed: the advertised ref map (exactly as today); the **skipped
occupancy set** — **every** skipped path, with a classified reason: content-skipped files
(this component), name-skipped files, and name-skipped **folders** (whose subtrees the Stage 5
walk deliberately never enters — the folder itself is the recorded occupancy); and errors.
(Round-2 Codex: recording only content-skips would let a create over a folder holding an
invalid-named child sail through preflight and reproduce the late-failure defect for that
class.) The set is consumed by component 2's preflight and delete handling, and by HEAD
advertisement below.
Only a **grammar/classification failure is skippable**; every transport or read failure remains
fatal, and the two must be distinguished by **type** (a sentinel/typed error from the ref-read
path), never by message text — a natural implementation that caught all read errors would turn
network failures into false absence, which is exactly the misrepresentation the carve-outs
below exist to prevent. A GUARD test pins it: a transport `ReadTo` failure during the walk
stays fatal while a grammar failure beside it skips.

**Classification is size-gated — downloads are bounded by a candidate band.** A valid ref
file is exactly 41 bytes, and the noncanonical damaged-ref shapes worth diagnosing sit within
a byte of it (no-LF = 40, CRLF = 42, double-LF = 42). **Plan-review disclosed edit
(2026-08-09): the band is 40–42, not the earlier 40–44** — the extra width existed for
BOM-prefixed content, but calling a BOM-corrupted file "40-hex with a malformed terminator"
misstates the corruption (the terminator may be fine; the prefix is the damage), so BOM
shapes are out-of-band generic junk classified by size, and the noncanonical message stays
true for exactly the three shapes it names. The walk classifies from the listing's size
metadata *before* any download:

- size known and **outside the 40–42 byte candidate band** → **skipped without downloading**;
  the note reports the size, never contents (bounds work, and keeps foreign file contents out
  of logs).
- size known and **within the band** → downloaded and grammar-checked (40 lowercase hex +
  `\n`, exactly 41 bytes, is the only advertisable form). On failure, skipped with a note
  carrying the escaped preview (≤42 bytes, control bytes hex-escaped) — and the noncanonical
  classification below. (Round-2 [Both] blocker: the round-1 gate downloaded only exactly-41-
  byte files, which made the damaged-pointer note below physically impossible for the 40- and
  42-byte shapes it exists for; the band restores it while keeping every download ≤44 bytes.)
- size unavailable → **skipped without downloading**, note says so. (Fail-safe: never download
  what cannot be bounded. The certified CLI reports sizes for files; this arm is belt-and-
  braces, and the note makes its firing visible if the assumption ever breaks.)

**The gate is a best-effort metadata bound, and says so.** The size is observed at listing
time; an occupant replaced between that observation and the read can in principle be larger
(round-2 Codex). The certified CLI offers no byte-capped partial download, so this residual is
**accepted and stated** rather than engineered away: the race requires a non-v2 actor writing
mid-operation, the same single-writer posture every check-then-act site in this design already
documents (design §2c), and the blast radius is one oversized transfer, not corruption.

**Notes are classified, and a damaged-ref pointer is never destroyed silently.** The note
grammar extends the existing `skipNote` mechanism — `git-remote-proton: skipping
<root>/<path>: <reason>` — with distinct reasons: `not a ref: size <N> outside the 40-42
candidate band`; `not a ref: <escaped preview>`; and, when an in-band candidate contains
exactly 40 hex characters with a wrong terminator (no-LF at 40 bytes, CRLF/double-LF at 42 —
the noncanonical class), the note **quotes the hex**:
`damaged ref? contents are 40-hex with a malformed terminator: <sha>` — so a recoverable
object pointer survives in the operator's log even though the file is skipped. (Round-1 Codex:
deleting such a file under the old fatal rule's remedy could destroy the only pointer to
remote-only objects; the note preserves it at zero behavioural cost.)

**What stays fatal, deliberately (three carve-outs):**
1. **Transport/`List`/read failures during the walk.** A *failed read* is not a *readable
   non-ref*; skipping on a failed observation would misrepresent state never seen. The walk's
   existing fail-closed error propagation (with the failing folder named) is unchanged, and the
   typed split above is what enforces it.
2. **`--set-head` to a corrupt target.** The user named that branch; the exact-path read errors
   with the bounded preview rather than pretending absence. (Unchanged from Stage 5; restated
   because the boundary rule differs on purpose: the push-direction survey degrades per-ref,
   the fetch-direction survey fails whole-with-enumeration, and a direct address is a demand
   that stays strict.)
3. **`WriteRef`'s read-back verification.** v2 verifying bytes it just wrote; tolerance there
   would mask real write corruption.

**The two policies over the one scan:**
- **`list for-push` (tolerant):** content-skips are absent from the advertisement, each with
  its note; pushes proceed; component 2's occupancy machinery owns collisions.
- **`list` — fetch/clone/`ls-remote` (strict):** a nonempty content-skip set fails the
  command with one error enumerating every content-skipped path, its classified reason
  (including the damaged-ref hex where recovered), and the remedy (delete the file —
  CLI grammar or web UI; Proton trash keeps it restorable). Complete-or-loudly-incomplete is
  the fetch-direction contract. Name-skips do not trigger this (the principled line in the
  scope section); their notes still print.

**Degraded states are defined, not implied.** For CONTENT-skips the strict fetch survey
fails before these states can matter, so they are push-survey states; for NAME-skips they
are reachable in BOTH directions (name-skips never fail a survey — round-5 Gemini):
- **HEAD names a skipped ref (either class)** → the HEAD symref is not advertised either,
  with its own note (advertising a symref to a ref git cannot see would be incoherent).
  Everything else advertises — in the fetch direction too, when the skip is a name-skip.
- **Every ref skipped** → an empty advertisement with one note per file. Push direction:
  `git push` behaves as pushing to an empty remote (creates proceed; component 2 refuses the
  occupied names). Fetch direction (all skips being NAME-skips — any content-skip fails it
  instead): clone behaves as cloning an empty repo, loudly noted.
- **Git-porcelain deletion lock-out is a documented consequence:** because a skipped name is
  not advertised, `git push --delete` of it is refused by *git itself* client-side ("remote
  ref does not exist") and never reaches the helper. The operator's remedy is the CLI/web UI,
  which every refusal message names (component 2). Recorded in README and the v6.6 error
  table. (If a delete of a skipped name does reach the helper — a non-git caller — the delete
  arm refuses it by occupancy, component 2.)

## Component 2 — Occupancy-aware push, and the occupant diagnosis

The advertisement that produces the remote map and the skipped set is the same helper
invocation that then executes the push batch, so the batch engine receives both.

- **Preflight (pre-pack, primary):** the final-state D/F preflight continues to compute over
  the advertised ref set, and **additionally treats every skipped occupancy as an occupied
  name**: a create of a skipped name itself, a create beneath one (`refs/heads/foo/bar` under
  skipped file `foo`), and a create above one (`refs/heads/foo` over skipped file `foo/bar` in
  a folder) are all **refused in phase 2, before any pack is built**, with the occupant named
  by a **kind-aware message built from the scan's classified reason** (round-3 Codex: a
  one-size message called every occupancy "a file whose contents are not a ref", which is
  false for name-skipped entries — never content-examined — and dangerous for folders, whose
  trash remedy could induce deleting a foreign subtree):
  - content-skipped file → `a file occupies <path> and its contents are not a ref (<reason>);
    delete it first (proton-drive filesystem trash <path>, or the web UI)`;
  - name-skipped file → `a file with an invalid ref name occupies <path> (contents never
    examined); delete or rename it first (…)`;
  - name-skipped folder → `a folder with an invalid name occupies <path>; its contents were
    never examined — inspect it before removing anything (…)` — the remedy directs
    inspection, never blind deletion.
  This closes the round-1 [Both] finding that skipped names silently vanished from the
  preflight and pushed the failure past pack upload into infrastructure errors with the wrong
  wording.
- **Delete arm:** a delete whose dst is a skipped occupancy is **refused** with the same
  kind-aware message — never reported as an already-absent success, and never trashed (the
  never-touch rule; deleting a "branch" must not delete an unknown foreign file or folder).
- **Create-heal wrapper (race window, secondary):** the existing arm where the post-refusal
  `Stat` shows a **file** gains the diagnosis for occupants that appeared *after* the
  advertisement: size-gate first (the SAME 40–42-byte candidate band as component 1 — an
  out-of-band occupant is classified by size without downloading; round 3 corrected a stale
  exact-41 phrasing here), then read only in-band candidates. If the occupant parses
  as a valid ref, keep the existing concurrent-creator message (it is true). Otherwise:
  `a file occupies <ref> and its contents are not a ref (<reason>); delete it first (...)`.
  The message asserts only what is observed **now** — it does not claim the file "was skipped
  at advertisement" (round-1 Codex: that history is not this code path's observation). The
  size gate here is the same best-effort metadata bound as component 1's, with the same
  stated residual.
- **The write boundary stays fail-closed in every arm.** Nothing is overwritten, nothing is
  auto-deleted; heal remains folders-only exactly as shipped. If a diagnostic read fails, the
  original refusal stands with the read failure noted — the same only-act-on-positive-evidence
  posture as the folder heal.
- **Not adopted, recorded:** reconciling a create whose occupant parses to the *same* sha as
  an idempotent success. It changes the Stage 2 concurrent-creator semantics for a case with
  no observed occurrence; deferred with the compaction-era backlog.

## Component 3 — Contract and infrastructure debts

- **F1 becomes a precisely-pinned contract row.** Stage 5's gate observed live that
  `filesystem download` of a **directory** exits 0 and recursively downloads. The new live row
  pins the precise shape: exit 0, and the subtree lands under `<dest>/<leaf>/…` with the
  directory's children present (fixture: one folder containing one file and one subfolder with
  a file — both land, relative layout preserved). `Fake.ReadTo` is taught the same behaviour so
  the fake and CLI halves agree — making `--set-head`'s Stat-IsDir-first guard (Task 10)
  permanently regression-safe. **The row certifies the transport layer**; the `testcli` shim's
  `runDownload` may keep its file-oriented behaviour, and its doc comment states that
  divergence and cites the row (no end-to-end shim test may rely on directory download).
- **`TestContractCLI`'s live root becomes configurable — with in-code validation.** New env
  var `GPB_CONTRACT_LIVE_ROOT`, read by the contract table only, default unchanged
  (`/my-files/_cas-probe/contract`). The table **validates before writing**, segment-wise on
  the raw value: the value must be an absolute path strictly below `/my-files/`, must not be
  `/my-files` itself, must not begin with any of the four untouchable top-level folders, and
  **must contain no `.` or `..` segments** (round-3 Codex: a prefix check alone admits
  `/my-files/x/../../outside`-style traversal if the CLI resolves dot segments) — otherwise
  the run refuses with the offending value named (fail-closed beats a checklist; round-1
  Codex). The standing brief
  checklist gains one line: state the table's root and include it in the confinement list.
- **The install.ps1 mock pins the `DoNotExpandEnvironmentNames` flag — as a spy, not a
  Windows-registry model.** The mock records the `$options` argument of every `GetValue` call;
  a RED test asserts install.ps1's real read passes `DoNotExpandEnvironmentNames` (dropping
  the flag — the bug that would bake an expanded `%USERPROFILE%` into the user PATH — now
  fails), and a companion case pins the recorder itself by asserting a flagless call is
  distinguishable (round-1 Gemini: the omitted state must be observable, or the spy can rot
  into always-passing). No expansion semantics are modelled — the spy asserts the contract,
  not the registry.
- **Small leftovers, all triaged OK-TO-DEFER in Stage 5 and collected here:** gate-runner
  long-timeout guidance added to `docs/research/gates/brief-checklist.md` (pushes run with a
  long tool timeout or in background — the S1 lesson); a three-line test for `EnsureParents`'
  mount-is-a-file branch (`parents.go`, the one untested arm); no other code changes ride
  along.

## Component 4 — Docs (v6.6)

One revision entry, edits in place per house style:

- **Error table:** the Stage 5 "malformed discovered *contents*" rows change verdict — at the
  advertisement boundary: skipped with a classified note, never advertised, never modified or
  deleted; on direct address (`--set-head` target, read-back): fatal exactly as before. The
  content rows are **per-operation**: push-survey tolerant (skip + note), fetch-survey fatal
  with enumeration. New rows: create/delete refused by skipped occupancy (pre-pack, with
  kind-aware remedy); the git-porcelain deletion lock-out and its remedy; the
  HEAD-names-a-skipped-ref push-advertisement degradation; the fetch-direction enumerated
  failure.
- **The v6.5 OPEN question is closed** with the adjudicated rationale (backup tool;
  per-operation policy — unattended push direction tolerant, attended fetch direction
  strict-with-enumeration; direct-address boundaries strict as before), replacing the
  open-question paragraph with the decision and a pointer to this spec.
- **F1 recorded** as a verified contract fact beside the ReadTo/C16 material, with the
  directory-download row cited.
- **README:** foreign-data paragraph — files under `refs/` **whose contents are not valid
  refs** (the helper cannot distinguish dropped junk from a damaged ref, and says so) never
  stop backups (skipped loudly on push) but DO stop fetch/clone/`ls-remote` with an error
  naming every such file, so a restore is never silently incomplete — **this fetch-blocking
  class is exactly valid-ref-named files with unparseable contents; invalid-NAMED files stay
  note-only in both directions** (they can never be refs, so their absence is never loss;
  round-5 Codex caught the unqualified wording) — mirror-fetch jobs will
  alarm on them, which is intended; pushing onto one names it and gives the remedy; git
  itself cannot delete such a name (the lock-out), so the remedy is the CLI/web UI (Proton
  trash keeps deletions restorable). `GPB_CONTRACT_LIVE_ROOT` is NOT documented in README
  (test-infrastructure var); it lives in the contract test's own comment and the brief
  checklist.
- **CHANGELOG:** `Unreleased` entries as components land; the version flip is release-prep
  work per `docs/releasing.md`.

## Component 5 — Testing and the live gate

**Hermetic (the Stage 5 discipline unchanged: TDD, RED/GUARD labels, which-assertion-fired,
`-count=1`, deliberate-regression checks for the load-bearing behaviours):**

- Content-skip: junk beside good refs → map contains the good refs only, note asserted on
  captured stderr (mutation-verified); nested junk skips without disturbing siblings; the
  size-gate arms (outside the 40–42 band skipped WITHOUT a ReadTo — pinned by trace assertion;
  in-band downloaded and parsed; the noncanonical-40-hex note quotes the hex — reachable
  because 40- and 42-byte shapes are in-band); the Stage 5 exact-grammar boundary fixtures
  (no-LF, CRLF, double-LF) flip from fatal-pinning to skip-pinning with fixtures preserved,
  each asserting its classified note.
- Occupancy completeness: a create over a folder whose only content is a name-skipped child
  (file or folder) is refused at preflight with NO pack built — the name-skip occupancy case
  (round-2 Codex); the refusal messages are kind-aware (round-3 Codex): a name-skipped FOLDER
  occupancy is never described as a file, and its message directs inspection, not deletion.
- Live-root traversal: hermetic validation cases include `/my-files/x/../../outside` and
  `/my-files/x/../<untouchable>` — both refused (round-3 Codex).
- Typed-split GUARD: a transport `ReadTo` failure during the walk stays fatal (named folder,
  no partial advertisement) while a grammar failure beside it skips.
- Per-operation policy: the SAME junk fixture through both survey directions — `list
  for-push` advertises the good refs and skips the junk with its note; `list` (fetch
  direction) FAILS with the enumerated error naming the junk path, its classified reason,
  and the remedy (mutation-verified: with the strict check removed, the fetch survey
  wrongly succeeds).
- Degraded states: HEAD naming a content-skipped ref (push survey: HEAD not advertised,
  note, others intact); HEAD naming a NAME-skipped ref (BOTH directions: HEAD not
  advertised, note, others intact — the fetch survey does not fail on name-skips);
  all-refs-skipped (push: empty advertisement; fetch with only name-skips: clone-of-empty,
  loudly noted).
- The restore shape: a protocol-loop clone with junk present FAILS with the enumerated
  error (complete-or-loudly-incomplete pinned); after the junk fixture is removed, the same
  clone succeeds with all refs. A clone with only NAME-skipped junk present succeeds with
  notes (the principled line pinned in both directions).
- Occupancy-aware push: create of / beneath / above a skipped name refused at preflight with
  NO pack built (trace-asserted, both D/F directions); delete of a skipped name refused, file
  untouched; the heal-wrapper race arm (occupant appearing post-advertisement): parseable →
  concurrent-creator unchanged (GUARD); unparseable → new message; oversized occupant →
  classified without download; diagnostic-read failure → original refusal + noted.
- Direct-address carve-outs: `--set-head` to a corrupt target still fatal (GUARD, existing
  test retained); read-back verification untouched (existing tests).
- F1: Fake.ReadTo-on-directory fidelity + the live row's fake half with the pinned layout.
- Contract-root validation: table refuses `/my-files`, an untouchable prefix, and a
  non-`/my-files` path (hermetic — validation runs before any live call).
- Mock spy: flag-dropped RED + flagless-call-distinguishable companion.

**Live gate (one, small — roughly a third of Stage 5's):**

1. Manufacture a junk file in the gate repo at a valid ref name (CLI upload of a non-ref file
   — the web-UI-equivalent action), plus one oversized junk file. Then, in order: **push of an
   unrelated ref succeeds** (tolerant direction, classified notes observed verbatim, the
   oversized one classified by size with no download — observed via the absence of a
   transfer); **`git ls-remote` FAILS** with the enumerated error naming both junk paths
   (strict direction, observed verbatim). **The junk files stay in place for step 2** (round-5
   Codex: an earlier draft deleted them here, leaving step 2's push nothing to collide with).
2. Push onto the junk name: the occupancy preflight refusal observed verbatim, and the
   row-set listing proves no pack was uploaded by that refused batch (BLOCK on mismatch, the
   Stage 5 signature-pin discipline). THEN delete both junk files via the CLI
   (verify-before-trash); `git ls-remote` and a clone now succeed with all real refs
   (complete-restore recovery observed live).
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
shallow/partial clone, SHA-256 repos. Also out, from this stage's review rounds: any
auto-deletion or quarantine-move of foreign files (the foreign-data rule is observe-and-report,
never modify or delete); a fetch-side tolerance env var (`GPB_TOLERATE_FOREIGN`-style escape
hatch for deliberate partial restores — YAGNI while delete-the-file remains available and
trash-recoverable; the door is noted, not built); a repository health/verify utility command
(the classified notes are this stage's integrity signal; a `--verify` mode that walks and
reports skipped occupancies is recorded as a Stage 7 candidate for the unattended-monitoring
gap round-1 Codex named); same-sha idempotent-create reconciliation (component 2).

## Execution notes for the plan (binding)

- Task order: the structured scan + content-skip first; occupancy-aware push second (consumes
  the scan); F1/liveRoot/mock/leftovers are independent and may interleave; docs after all
  behaviour lands; release prep + gate brief last with an explicit STOP for Craig.
- Plan-supplied code blocks are hypotheses (three SUPERSEDED banners in Stage 5, five in
  Stage 4); SDD execution in a fresh session whose controller reads `v2-sdd-runbook.md` first
  and deletes the Revisions blocks from spec and plan before dispatch; commits carry the Opus
  co-author trailer; nothing pushed without Craig's word; hermetic tests only outside the gate;
  every `go test` with `-count=1`.

## Decisions log (brainstorm, 2026-08-09)

| Decision | Choice | Rejected alternatives |
|---|---|---|
| Stage 6 anchor | Design debts (fatal-content decision + Stage 5 ledger) | Compaction now; usage-driven feature; minimal cleanup |
| Fatal-content rule | Per-operation: push survey tolerant (skip + occupancy-aware push); fetch survey strict with enumeration | Uniform skip (initial choice, reversed on re-examination); keep fatal with better UX; namespace-dependent middle ground |
| Strictness boundary | Push survey degrades per-ref; fetch survey complete-or-fail; direct address and writes stay strict | Uniform skip everywhere; uniform fatal everywhere |
| Fetch tolerance escape hatch | Not built (delete-the-file is available and trash-recoverable) | GPB_TOLERATE_FOREIGN env var now |
| Scope shape | Full debt ledger rides along, one small gate, v0.5.0 | Decision-only minimal; debts + compaction start |
| Foreign-file handling | Observe and report only, never modify or delete | Auto-trash; quarantine-move |

## Revisions

*Scaffolding for the review loop; delete before the spec is treated as final.*

**Round 1 (Codex + Gemini, 2026-08-09) — applied:** `ListRefs` result restructured (advertised
map + typed skipped-occupancy set + typed errors) and the skip keyed on a TYPED grammar
failure with a transport-failure GUARD ([Codex] critical: an untyped implementation could turn
network failures into false absence); occupancy set fed into the batch preflight and delete
arm so same-name/both-direction D/F collisions and deletes of skipped names refuse PRE-pack
with the remedy message ([Both]: the draft's "preflight unaffected" was wrong — late failures
bypassed the new diagnosis); size-gated classification — only exactly-41-byte candidates are
ever downloaded, wrong-size and unknown-size files skip on metadata alone ([Both]: unbounded
download DoS; also bounds log disclosure to escaped 41-byte previews); noncanonical-40-hex
notes quote the hex so a damaged ref's object pointer survives in the log ([Codex]); the
occupant message asserts present observation only, not "was skipped at advertisement"
([Codex]); degraded states defined + tested (HEAD→skipped, all-skipped) ([Codex]); git-
porcelain delete lock-out documented with the CLI remedy ([Gemini]); `GPB_CONTRACT_LIVE_ROOT`
validated in code (below /my-files, never an untouchable) ([Codex]); F1 row pinned to an
exact fixture/layout and scoped to the transport layer ([Codex]); the install.ps1 mock became
a flag SPY with both present and omitted cases ([Codex] + [Gemini]); README wording
("contents are not valid refs", "never modified or deleted") ([Codex]). **Rejected with
reasons:** per-operation strictness / strict-clone default ([Both] re-litigated the
brainstorm-adjudicated availability decision; the scenario — including exit-0-reads-as-success
— was argued before Craig chose; flagged to Craig at the review gate rather than adopted);
same-sha idempotent create reconciliation ([Codex] — changes Stage 2 concurrent-creator
semantics for an unobserved case; deferred); a verify/health command ([Codex] — real gap,
wrong stage; recorded as a Stage 7 candidate in Out of scope).

**Round 2 (Codex + Gemini, 2026-08-09) — applied:** the round-1 size gate was DEFECTIVE and
both engines caught it as a blocker — "download only exactly 41 bytes" made the damaged-
pointer note physically impossible for the 40-byte (no-LF) and 42-byte (CRLF/double-LF)
shapes it exists for; replaced with a 40–44-byte candidate band (every download still ≤44
bytes; wrong-terminator hex recovery restored; component-5 fixtures now assert their
classified notes) ([Both] blocker). The occupancy set widened from content-skips to ALL
skipped paths — name-skipped files and name-skipped folders included (folder itself recorded;
its subtree is never entered) — closing the late-failure reproduction for invalid-named
foreign data, with a preflight test ([Codex] major). The size gate is now explicitly a
best-effort metadata bound with the replace-between-observe-and-read race accepted and
stated (no byte-capped download exists in the certified CLI; same single-writer posture as
design §2c), in both components 1 and 2 ([Codex] major, honesty option adopted).

**Round 3 (Codex + Gemini, 2026-08-09, final round — cap reached; these fixes are applied but
NOT re-verified by a further engine round; Craig's spec review and the plan's own review
rounds are the net):** round 2's band fix was incompletely propagated and both engines caught
the stale exact-41 phrasings — component 1's note grammar (`!= 41` → outside-the-band) and
noncanonical clause ("41-byte candidate" → in-band candidate at 40/42 bytes), and component
2's create-heal wrapper (now the same 40–44 band) ([Both]). The occupancy refusal messages
became kind-aware — content-skipped file / name-skipped file (contents never examined) /
name-skipped folder (inspect, never blind deletion) — because a one-size "a file whose
contents are not a ref" message was false for name-skips and its trash remedy dangerous for
folders ([Codex] major). `GPB_CONTRACT_LIVE_ROOT` validation is segment-wise and rejects
`.`/`..` segments, with hermetic traversal cases ([Codex] major). No other new blocker/major
findings in round 3; Gemini explicitly reported none beyond the stale-phrasing items.

**Round 4 (Craig-directed reversal, 2026-08-09):** on Craig's review of the round-1 rejection
list, the per-operation-strictness proposal ([Both], round 1) was RE-EXAMINED on merits — the
original rejection reasoned circularly ("re-litigates an adjudicated decision" whose
adjudication had followed the author's own recommendation). The re-argument: the push and
fetch directions are different trades with different people present (unattended cron backups
vs attended restores/alarming mirrors), so uniformity forced one answer onto two questions;
the round-1 structured scan makes per-operation policy a small protocol-layer switch.
**Adopted:** push survey tolerant (unchanged from rounds 1–3); fetch survey
(`list`/fetch/clone/`ls-remote`) fails on a nonempty content-skip set with full enumeration
(reasons + damaged-ref hex + remedy in the blocking error); name-skips stay non-fatal in both
directions (they can never be refs — no silent-loss class); no fetch tolerance env var
(YAGNI, recorded in Out of scope). Components 1, 4, 5, the scope rationale, and the decisions
log were revised accordingly. These edits are Craig-adjudicated and post-date the three
engine rounds.

**Round 5 (Codex + Gemini, 2026-08-09, verification of round 4) — applied:** gate steps 1–2
reordered — round 4's edit deleted the junk files before the push-onto-junk step needed them
to collide with ([Codex] blocker); the README's fetch-blocking class qualified to
valid-ref-named files with unparseable contents, name-skips documented note-only in both
directions ([Codex] major); the degraded states corrected — for NAME-skips they are
reachable in BOTH directions since name-skips never fail a survey (HEAD→name-skipped and
all-name-skipped defined and tested for the fetch direction too) ([Gemini] major). Both
engines otherwise explicitly reported no further blocker/major findings. These round-5 fixes
are propagation corrections applied at the loop's end without a further engine round; the
plan-review rounds re-cover the spec.
