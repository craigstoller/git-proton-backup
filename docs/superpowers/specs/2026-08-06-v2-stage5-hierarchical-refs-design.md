# v2 Stage 5 — hierarchical refs, quarantine fetch staging, and gate-sourced polish

**Status:** DRAFT for peer review, brainstormed with Craig 2026-08-06. Decisions below were made
interactively and are recorded with their rationale; do not re-litigate without new evidence.
**Baseline:** `main` @ `68a3816`, v0.3.1 published, Stage 4 live gate passed in full
(`docs/research/gates/stage4-gate.md`), CI green.
**Normative design:** `docs/v2-remote-helper-design.md` v6.4. This stage produces v6.5; the
required design-doc edits are enumerated in component 9.
**Release:** Stage 5 ships as **v0.4.0**, draft-until-gated, same three-asset set and digest
closure as Stage 4.

## Scope decided in brainstorm (Craig, 2026-08-06)

1. **Hierarchical refs** with **full namespace re-enable** — not heads/tags-only.
2. **Quarantine fetch staging** (retro-Codex 3), deleting the residue rule and F2's class.
3. **Gate-sourced UX**: opt-in parent auto-create (env var; actionable error by default),
   MAX_PATH documentation plus best-effort hint, marker-absent vs read-failure disambiguation.
4. **Test/infra + process hygiene**: install.ps1 registry seam, `--set-head` wiring injection
   seam (committed) with a scripted-CLI shim harness as a **cuttable stretch**, row-set
   comparison rule for gate briefs, CHANGELOG unreleased→date step in the release procedure.

**Structure:** hygiene-first, one live gate at stage end (Stage 4's shape). Long-parked items
(compaction, PowerShell Gallery, non-Windows) remain out of scope.

---

## Component 1 — Hierarchical refs: storage and advertisement

The `refs/` tree becomes arbitrarily nested folders. Ref files keep the exact grammar — 40
lowercase hex plus `\n`; violations remain fatal, never coerced.

- **`ListRefs` recurses the whole `refs/` tree**, one non-recursive `List` per folder,
  depth-first. Every well-formed ref file found is advertised under its full
  `refs/...` name. Folders containing no ref files contribute nothing — empty parents are
  benign to every read path (they matter only to the create path; see component 2).
- **Discovered names are validated at the read boundary — skip-with-note, never fatal.** A
  non-v2 actor (web UI, another client) can create nodes under `refs/` with names git cannot
  accept, and those must never reach the protocol stream. But making a bad *name* fatal would
  hand any stray web-UI file the power to brick every helper operation on the repo until
  manual cleanup — advertisement runs first, so clone, fetch, and push would all die. So the
  read boundary follows the design's own packs-directory precedent (non-packish nodes are
  skipped with a stderr note): a discovered name that fails validation is **skipped with a
  loud per-name stderr note naming the exact remote path**, never advertised, never touched.
  The existing "malformed ref-file *contents* → fatal" rule is unchanged — it applies to refs
  that were advertised. v2 itself can never create a skipped name (2a refuses them on push),
  so a skip always marks foreign data.
- **Read-boundary validation is in-process, push-boundary validation stays authoritative.**
  The read-side check enforces protocol-stream safety and git's documented ref-name rules
  structurally (no control characters, space, `~^:?*[\`, no `..` or `@{`, no leading/trailing
  or doubled `/`, no trailing `.` or `.lock` suffix, not `@`) without spawning a process per
  ref — `git check-ref-format` takes one name per invocation, and a flat folder of thousands
  of tags must not mean thousands of subprocess launches during advertisement. The push path
  keeps the real `git check-ref-format` (batches are small; authority matters more there),
  so any drift between the two checks errs toward skipping at read and refusing at push —
  both safe directions.
- **Cost is stated, not hidden: one CLI `List` subprocess per folder plus in-process name
  checks per ref.** Serial, deliberately. Typical ref trees are shallow and small; a
  pathological tree makes advertisement slow, not wrong. This is the same accepted posture as
  the design's "discovery cost grows linearly with pack count" note, and the v6.5 edit
  records it beside that note. No parallel listing in Stage 5 (concurrent CLI *startups*
  collide — A7/A8 — and the retry machinery to exploit the post-startup window is not worth
  building for this).
- **The v6.1 namespace rejection is retired.** Recursion makes `refs/notes/*`,
  `refs/replace/*`, `refs/stash`, and other valid namespaces visible without a filter, so they
  are advertised and pushable. v6.1's first justification (non-recursive `ListRefs`) is erased
  by this component. Its second (expensive misleading late failure, orphan packs) is
  **addressed but not erased**: batch preflight (2b) now validates every name and D/F
  conflict before anything is packed, so the *foreseeable* failures move ahead of the pack
  upload — but a runtime folder failure after the pack is uploaded (permission, transport)
  still leaves an inert orphan pack, the design's existing accepted class, and the v6.5 text
  must not claim otherwise. The push rule for these namespaces is the design's existing
  conservative deviation, unchanged in wording: **`CreateExclusive` on create, force required
  for any move, no ancestry check**; delete goes through the same trash+prune path as branches.
  The deviation-from-git note (v2 stricter than git's object-type-aware rules) stays in the
  design doc.
- **HEAD protection is branches-only**, unchanged: the delete-refusal guards the branch `HEAD`
  names, hierarchical or not; nothing outside `refs/heads/` is ever a HEAD target.
- **Pseudorefs and destinations outside `refs/` stay rejected** with a named reason.

## Component 2 — Hierarchical refs: push mechanics

Four normative rules.

**2a. Uniform component validation.** Every path component of a ref name — not only the leaf —
must pass the same stageable-name rules the leaf already does (`checkStageableLeaf`: no `{` or
`}`, no Windows reserved device names, not empty/`.`/`..`), after `git check-ref-format`.
Violations are refused with a named reason at validation time, before anything is packed.
Rationale, recorded so it is not weakened to leaf-only later: (1) consistency — one rule for
every path element; (2) it sidesteps an **unverified** transport hazard rather than probing it —
C13 proved the CLI glob-expands `{` in **local** paths, and whether remote path arguments are
also glob-expanded has never been probed; components appear in remote paths on every
`EnsureDir`; (3) future-proofing — any later code path that mirrors components into local
paths (fetch staging, caches) would meet the device-name and brace hazards on Windows.

**2b. Batch execution order, normative.** The buffered batch executes in five phases. The
invariant, stated precisely: **no ref mutation and no destructive operation precedes phase
4** — phase 3's writes are immutable pack/idx objects, whose worst failure mode is an inert
orphan pack, the design's existing accepted class.

1. Buffer the complete blank-line-terminated batch; poison-flag enforcement (unchanged).
2. **Validate the whole batch before anything moves**: `git check-ref-format` plus rule 2a
   for every destination; duplicate destinations refused; and a **final-state D/F preflight**
   — compute the post-batch ref set (the advertised refs plus this batch's changes) and
   refuse, with a named reason, any create whose name conflicts directory/file-wise with
   that final state (e.g. `feature` and `feature/x` both created in one batch). The
   preflight sees **refs**, not folders: a collision with residual *empty* folders is
   invisible here by construction and is resolved at execution time by self-heal (2c) — the
   preflight claim is "no D/F conflict within the final ref set," nothing stronger.
   Failures here cost nothing: no pack has been built.
3. Build, upload, and confirm the pack for the batch's creates/updates (deletions need no
   objects).
4. Execute deletions (trash + prune, 2c).
5. Execute creates and updates.

**Phase-3 failure control flow, explicit:** a pack failure marks every create/update
`error <dst>` naming the pack failure, and execution **continues to phase 4** — deletions
are object-independent, individually requested operations and run with their own per-ref
statuses. This is the design's own non-atomic per-ref posture, not a new promise; the
alternative (aborting deletions on an unrelated upload failure) was considered and rejected
as coupling refs the protocol treats as independent. Every ref still receives exactly one
`ok`/`error`.

Deletions-before-creates is what makes `git push origin :feature feature/x` succeed in one
batch regardless of the order git sends. Placing deletions **after** pack confirmation is
what prevents the failure both engines' review flagged: without it, a destructive change
could precede the batch's riskiest step. What this order does **not** promise: a create
failing in phase 5 does not resurrect a branch deleted in phase 4 — the user explicitly
requested that deletion, multi-ref batches remain non-atomic per the design, and each ref
still gets its own `ok`/`error` (response order need not match command order).

**2c. Parent creation, prune on delete, self-heal on create.**

- **Create:** for `refs/heads/feature/x`, walk the components under the repo root
  (`refs/heads/feature`), `EnsureDir` each level (Stat-then-create, one level at a time), then
  `CreateExclusive` the leaf. The walk never touches paths above the repo root (component 6
  owns that).
- **Prune on delete:** after trashing the ref file (Stat-first — `Trash` is not idempotent),
  walk parent folders upward: `List` each; if empty, verify-then-`Trash`; stop at the first
  non-empty folder. **Never prune `refs/heads`, `refs/tags`, or `refs/` itself** — they are
  part of the initialised layout. Other namespace roots (`refs/notes`, …) are created on
  demand and are prunable. The prune runs under the batch's lock. A crash mid-prune leaves an
  empty folder; that is tolerable **only because** self-heal exists — the two mechanisms are a
  pair, and a plan that ships one without the other reintroduces the wedge this spec exists to
  prevent.
- **Self-heal on create (D/F reuse, `feature/x` deleted → `feature` created):** when a
  ref-file create does not commit — `Refused` **or** an error, since the CLI's outcome shape
  for a file upload colliding with a folder is unverified until the new contract row pins it —
  and a `Stat` shows a **folder** at that name, the folder's **subtree** is enumerated under
  the lock, and the test is **"contains no ref files," not "first level is empty."** A crash
  mid-prune after deleting `refs/heads/feature/x/y` can leave `feature/` containing the empty
  folder `x/` — a first-level emptiness test would misread that as live sub-refs and
  permanently block branch `feature`, which is precisely the wedge self-heal exists to
  prevent. So: subtree contains no ref files (only folders) → verify-then-`Trash` the folder
  and retry the create once; subtree contains ref files → refuse, naming those **actual ref
  files** as the conflict (git itself refuses this D/F state locally; the refusal is correct
  even if the create's failure was transient, because the create cannot succeed while the
  folder exists — and it never names an empty directory as if it were a ref). If the
  diagnostic `Stat` or any `List` in the enumeration fails, report **that** transport failure
  and heal nothing — self-heal runs only on positive evidence. A `Refused` against an existing ref **file** keeps its current meaning —
  concurrent creator, report, never overwrite.
- **Reverse D/F (component blocked by a ref file):** creating `refs/heads/feature/x` when
  branch `feature` exists as a file and is **not** deleted earlier in this batch → refuse with
  a named reason mirroring git's own local directory/file conflict rule. (If the batch deletes
  it, rule 2b makes the create succeed.)
- **The folder trashes are check-then-act, and this spec says so rather than implying it
  away.** No conditional delete exists on this transport (the design's lock-release rule
  already lives with the same limit). Between verifying emptiness and `Trash`, a **non-v2
  actor** could create a child that the recursive folder trash then carries away — v2 writers
  cannot, because both paths run under the batch lock. The blast radius is bounded: the
  subtree goes to Proton's trash (restorable via the web UI), the affected content is ref
  files whose value-recovery status the design already documents as provisional, and the
  window exists only during an operation the single-writer model says should not be racing
  anything. This is the same accepted posture as every other v2 mutation, extended to two new
  sites — not a new class of promise.
- **`Trash` outcomes are handled closed-set at both new sites.** Prune is best-effort: a
  `Refused`/`Ambiguous`/error on a prune trash logs the folder to stderr and stops ascending —
  the leftover is exactly what self-heal exists for. Self-heal is not best-effort: anything
  but `Committed` on its trash refuses the create, quoting the outcome, because proceeding
  would mean creating a ref at a name whose state is unknown.

**2d. `EnsureDir` contradiction handling (generic robustness, NOT a validated C17 fix).** If
`create-folder` fails with already-exists after a `Stat` reported absent (the C17 signature,
observed once, never reproduced under provocation — C17b), the helper re-`Stat`s once: a
**folder** present → proceed as if it existed all along; a **file** present → the reverse-D/F
refusal from 2c, named, not a proceed; still absent → fail, quoting **both**
observations verbatim so the contradiction is diagnosable. Framed exactly this way in code
comments and the design doc: C17b's ruling stands — this can only be justified as generic
robustness, never claimed as a live-validated fix.

## Component 3 — `--set-head` hierarchical support

The slash refusal (which names Stage 5 today) is lifted. Short names normalise to
`refs/heads/<name>` including embedded slashes; full `refs/heads/…` forms accepted as today;
anything outside `refs/heads/` still refused. Components validated per rule 2a. The remote
existence check uses an **exact-path `Stat` and read of the one target ref file** — it does
not recurse the tree; `--set-head` has no reason to enumerate namespaces it is not touching.
Everything else — lock lifecycle, verify-branch-before-idempotence-short-circuit,
`UpdateHEAD`/`WriteHEAD` split, outcome handling — is untouched. `--set-head` does **not**
honour the parent-create env var (component 6): its precondition is an existing repo with an
existing branch, and a repo cannot exist below a missing parent — the var could only
manufacture folders and then fail the marker check, so offering it there would be a trap.

## Component 4 — Quarantine fetch staging (retro-Codex 3)

Fetch downloads land in a **per-fetch incoming directory**, not in the temp alternate:

- **Placement is normative:** the incoming dir and the temp object dir are siblings under one
  per-fetch temporary root, so they are on the same filesystem by construction and
  publication is a plain `rename`, never a cross-volume copy of a repository-sized pack. The
  root is unique per fetch (no cross-fetch collision to defend against) and is removed
  wholesale at fetch end, success or failure. A crash leaves one orphaned temp root — inert,
  never read as repo state, the same residue story the temp object dir has today. There is no
  delete-at-start of someone else's path.
- Verification runs **in quarantine**: checksum-vs-basename immediately on a `.pack`'s
  arrival; `index-pack --verify` on the pair as soon as both members are local (unchanged
  timing rules from the error table).
- **The trust boundary is planning vs traversal, and only traversal is protected.** The
  design's fetch algorithm downloads `.idx` sidecars *before* their packs and plans from
  them; a lone index is necessarily unverifiable, and the error table already permits exactly
  that — "the lone index is used only to *plan* which packs to fetch, never as a source of
  truth about objects." Quarantine keeps that split and gives it a physical address: the
  planner may read lone, untrusted `.idx` files **in the incoming dir**; the traversal
  (`rev-list` via the alternate) sees **only** published pairs.
- Only fully verified pairs are **published** into the temp alternate's `pack/` directory —
  each member renamed into place, `.pack` first, then `.idx`, and **the `.idx` rename is the
  pair's commit point**: git discovers packs through their indexes, so a pack whose index has
  not yet appeared is invisible to the traversal, which is what makes the two renames an
  atomic publication from the reader's side. The Stage 4 polish trace assertion (`.pack`
  before re-downloaded `.idx` in the mid-round pair refresh) must keep passing — publication
  preserves that ordering.
- **Deleted by this refactor:** the failed-attempt residue-deletion rule, and the F2 failure
  class **narrowed to what quarantine actually removes** — partial or unverified files
  visible to the *traversal*. Planning reading untrusted indexes is not residue and not a
  hazard; it is the algorithm.
  The spec-vs-code note from retro-Codex finding 2 (`listCompletePacks` stderr-notes only
  packish-looking skips) is resolved in the code's favour while touching this path: the design
  doc's letter is aligned to "packish-looking skips get notes" in the v6.5 edit.
- **Kept:** the idx-cache self-heal round for cache-suspect failures (different mechanism —
  cache staleness, not download residue), complete-pairs-only cache admission, the pack-name
  grammar, greedy pack cover, the no-progress termination rule.
- Externally behaviour-invariant: same transport calls in the same order; the trace decorator
  sits at the transport boundary and sees no difference.

The retro-Codex two-pack pair-refresh gap is closed here too: the pair-refresh fixture gains a
**two-pack** variant so restart-vs-resume of a multi-pack plan is empirically pinned, not just
structurally.

## Component 5 — UX: marker and MAX_PATH diagnostics

- **Marker-absent vs read-failure.** `RequireMarker` (and any path reporting "not a
  git-remote-proton repo") distinguishes a **confirmed absence** from a **failed read**
  ("could not read <path>: <cause>"). **The mechanism is the transport contract's existing
  seam, not output-text classification:** `Stat` already defines absence as
  `(_, false, nil)` — a first-class non-error — so the marker check is `Stat`-then-read:
  `Stat` says absent → today's not-a-repo message; `Stat` errors → transport failure,
  reported as such; `Stat` says present but the read fails → transport failure, never
  "absent." No new parsing of CLI error text is introduced anywhere on this path, and the
  certified CLI's not-found signature that `Stat` itself keys on gets a contract row
  (component 8) so the classification is pinned live, not assumed. Kills the Stage 4 gate 2b
  masquerade where a broken CLI under `GPB_UNCERTIFIED_CLI=1` surfaced as "not a repo".
- **MAX_PATH.** README gains a Windows section: deep clone destinations can exceed the
  legacy 260-character limit; remedies are `git config core.longpaths true` (helps for git's
  own file writes) or a shorter destination (always helps), and which applies. Helper-side:
  when a **spawned git command fails** and a computed relevant path is **near or over the
  limit** (threshold ≥ 240 characters, deliberately conservative — the legacy limit counts a
  terminator and directory operations fail below 260), append a one-line hint phrased as a
  **possible** cause naming both remedies — the failure may be unrelated, and the hint must
  not claim otherwise. The trigger is **path-length arithmetic, never stderr
  pattern-matching** (git messages are localised; lengths are not). Two stated limits: no
  detection is promised for paths git computes internally beyond what the helper can see, and
  a clone's **checkout** phase runs after the helper has exited, so checkout-time failures
  are reachable only by the README, never by the hint.

## Component 6 — UX: opt-in parent auto-create

Missing parents of the repo root (Surprise R2-1: raw `Node not found: GitRemotes`) become:

- **Default (unset): an actionable refusal.** Names the first missing parent and gives an
  **executable** remedy in the CLI's real grammar — `proton-drive filesystem create-folder
  <parent> <name>` with the actual values filled in (the command takes parent-plus-name, not
  a path) — alongside the web-UI route and the opt-in env var. Typo'd addresses fail loudly
  instead of manufacturing folder trees.
- **`GPB_CREATE_PARENTS=1`** (exact name fixed here; same conventions as
  `GPB_UNCERTIFIED_CLI`: read fresh from the environment every invocation, never cached or
  remembered): the `EnsureDir` walk extends **above** the repo root, with a **loud stderr
  line per created folder**. **Walk bounds are explicit:** it creates only components
  **strictly below** the mount — never `/my-files` or `/devices` themselves, and never a
  `/devices/<device-id>` node (a device mount is not creatable storage); if the mount itself
  is missing the refusal fires as in the default mode. Canonicalisation already rejects
  everything outside those roots. **Push bootstrap only** — `--set-head` deliberately does
  not honour it (component 3 says why). An env var, not a flag, because git invokes the
  helper on the protocol path and the user cannot pass flags.
- **No rollback, stated.** Parent creation can partially succeed — some folders created, then
  a later step fails. The created folders remain (deleting them would be exactly the unsafe
  folder-removal race component 2c bounds so carefully, for zero benefit); the stderr lines
  say what was created.
- Gate briefs and tests must account for the confinement consequence: with the var set, the
  helper writes **outside the repo root**. The gate exercises both modes (component 8).

## Component 7 — Hygiene and test infrastructure

- **install.ps1 registry seam.** The user-PATH read/write goes through an injectable accessor
  (parameter defaulting to the real `Microsoft.Win32.Registry` implementation). Pester then
  pins the value-kind preservation (REG_EXPAND_SZ vs REG_SZ), the append/no-op idempotence
  logic, and the shadow-detection interplay **hermetically** — the isolation rule (no real
  registry writes in tests) finally gets regression coverage instead of hand-verification.
  The WM_SETTINGCHANGE broadcast stays untested (out of reach hermetically); say so in the
  test file rather than implying coverage.
- **`--set-head` wiring seam (committed).** A small refactor makes the
  `dispatchUtility`→`runSetHead` call site testable against the existing fake transport, so a
  hermetic test pins argv routing, arity errors, and the documented stdout (`HEAD is now …`).
- **Scripted-CLI shim harness (STRETCH — cuttable without renegotiation).** A fake
  `proton-drive` test binary (Go `TestHelperProcess` re-exec pattern) backed by a temp
  directory tree, placed on PATH for the test, so hermetic tests run the **real**
  argv→allowlist→transport-construction→repo path end to end. Reusable later for
  protocol-path hermetic tests. If it lands, the wiring test above is additionally run
  through it; if cut, the injection seam remains the committed coverage.
- **Row-set rule for gate briefs.** The gate-brief template/checklist states: listing equality
  is asserted on the **row set** (uid/name/type/times/parent), never on the serialised JSON —
  `filesystem list` order is unstable (Stage 4 run 2 observed it directly).
- **CHANGELOG step.** The release procedure gains an explicit step: flip the `[Unreleased]`
  section to the version and date **before tagging** (the v0.3.x flips were manual catches).
  v0.4.0 exercises the amended procedure for real.

## Component 8 — Testing and the live gate

**Hermetic:**
- The in-memory fake transport gains full folder fidelity: nested create/trash, folder-vs-file
  collisions on both sides, emptiness observable via `List`, `Trash` non-idempotence on
  folders — everything components 1–2 depend on.
- New live **contract-table rows** (run at gates only, `GPB_LIVE_ACCOUNT=1`) for behaviours v2
  now depends on: `Trash` on an **empty** folder and on a folder **with children** (outcome
  shape both ways — prune assumes the former, the check-then-act analysis in 2c assumes the
  latter is recursive), `create-folder` colliding with a file and vice versa (error shape),
  nested `List`, `upload` of a file colliding with a folder name, and **`Stat` on a missing
  node** — pinning the certified CLI's not-found signature that the component-5
  absent-vs-error split keys on, and that every other failure of the same command classifies
  as an error, never as absence. Fake and CLI halves must
  agree per the existing decorator pattern.
- RED/GUARD labelling, which-assertion-fired reporting, and deliberate-regression checks per
  the runbook — mandatory for prune, self-heal, batch ordering, and the quarantine publish
  ordering, which are the load-bearing new behaviours. The self-heal suite must include the
  **nested-empty residue** case (a folder containing only empty folders heals; a folder
  containing a ref file anywhere in its subtree refuses naming that file), and phase-3
  failure must be pinned continuing into phase 4 with correct per-ref statuses.
- Sha-collision hygiene in fixtures (pin `GIT_COMMITTER_DATE` where content repeats).

**Live gate (one, at stage end), in outline:**
1. Hierarchical end-to-end: push `feature/x` + a nested tag + a `refs/notes/*` ref;
   **advertisement asserted with `git ls-remote`** (a clone alone does not prove namespace
   coverage — clone does not fetch notes by default), then clone, then an **explicit fetch of
   `refs/notes/*` verifying the OID landed**; incremental fetch.
2. The D/F workflow, **including self-heal live**: delete `feature/x` (observe prune — the
   parent folder is gone from the listing), push branch `feature` (clean-path create); then,
   inside the gate repo, manufacture the two collision states by hand (an empty folder at a
   branch name; a folder containing a ref file) and push against each — asserting the heal
   (trash + create succeeds, loud stderr) and the refusal (names the live sub-refs). The
   composed `create → Stat → List → Trash → retry` path is exactly where this project's
   history says seams lie; it runs live, not only against the fake.
3. `--set-head` to a nested branch; delete-protection follows HEAD.
4. Quarantine no-regression: mid-round pair refresh, up-to-date re-fetch with zero `packs/`
   downloads.
5. Parent auto-create: unset → actionable refusal, account listing unchanged; set → parents
   created with loud stderr, then cleaned up in the gate's teardown.
6. Cleanup with verify-before-trash; **all listing comparisons as row sets**; trash accounting
   includes pruned folders.
7. Release: v0.4.0 draft's bytes under test; publication digest closure after Craig publishes.

Confinement rules unchanged: writes only under `/my-files/GitRemotes/<demo>` (plus the
gate-authorised creation of `GitRemotes` itself), untouchables read-only, verify-before-trash,
report BLOCKED verbatim rather than patching.

## Component 9 — Design-doc edits (v6.5)

One revision entry, edits in place per house style:
- Storage layout: nested `refs/` trees; component validation rule (2a); prune/self-heal
  normative rules (2c); `EnsureDir` contradiction handling (2d).
- Ref-transition table: other-namespace row un-narrowed (v6.1 note updated to "re-enabled
  Stage 5"); delete row gains prune; new rows for D/F collisions both directions.
- Push: the five-phase batch execution order (2b), including the final-state D/F preflight
  and pack-before-deletions rule.
- Advertisement: read-boundary name validation; listing-cost note beside the existing
  pack-count scaling note.
- Fetch: quarantine staging replaces the residue rule; `listCompletePacks` stderr-note letter
  aligned to code (packish-looking skips only).
- Utility modes: `--set-head` hierarchical grammar; slash-refusal text superseded.
- New env var documented alongside `GPB_UNCERTIFIED_CLI`; error-table rows for the missing-
  parent refusal (both modes), marker read-failure vs absence, MAX_PATH hint.
- Error table: folder-collision rows updated for self-heal (empty-folder case no longer
  simply "fatal with the specific path").

## Out of scope (unchanged verdicts)

Compaction (`RevListNewObjects` memory note stays parked with it), PowerShell Gallery
publishing, non-Windows support, multi-writer, shallow/partial clone, SHA-256 repos.

## Execution notes for the plan (binding)

- Hygiene-first task order as brainstormed (component 7 items and 5's disambiguation early;
  quarantine before hierarchical so fetch-side tests land on the final architecture; parent
  auto-create after hierarchical, reusing its `EnsureDir` walk; shim stretch last before
  release+gate).
- Plan-supplied code blocks are hypotheses (five SUPERSEDED banners in Stage 4 were all
  plan-code defects); SDD execution in a fresh session whose controller reads
  `v2-sdd-runbook.md` first; commits carry the Opus co-author trailer; nothing pushed without
  Craig's word; hermetic tests only outside gates.

## Decisions log (brainstorm, 2026-08-06)

| Decision | Choice | Rejected alternatives |
|---|---|---|
| Stage 5 scope | All four groups | Deferring UX/hygiene to a Stage 6 |
| Namespace scope | Full re-enable under conservative force rule | heads+tags-only; stretch-task split |
| Empty parents / D/F | Prune on delete + self-heal on create | Either alone; refusing D/F reuse |
| Missing parent UX | Opt-in auto-create via env var, actionable error by default | Message-only; always-on auto-create |
| Wiring coverage | Injection seam committed, shim harness stretch | Seam-only; shim-only |
| Structure | Hygiene-first, one gate, v0.4.0 draft-until-gated | Feature-first; two-stage split |

## Revisions

*Scaffolding for the review loop; delete before the spec is treated as final.*

**Round 1 (Codex + Gemini, 2026-08-06) — applied:** batch execution rewritten as a five-phase
order with final-state D/F preflight and pack-confirmation-before-deletions (both engines);
quarantine's trust boundary corrected to planning-vs-traversal — lone `.idx` files legitimately
plan from quarantine, only the traversal is confined to published pairs (Codex); publication
atomicity specified (same-filesystem siblings, rename, `.idx` as commit point) and incoming-dir
placement pinned (Codex + Gemini); check-then-act window and blast radius of the two folder
trashes stated with closed-set `Trash` outcome handling (Codex); read-boundary ref-name
validation added (Codex); v6.1 orphan-pack justification downgraded from "erased" to "addressed"
(Codex); `--set-head` dropped from `GPB_CREATE_PARENTS` (no coherent success case — Codex) and
switched to exact-path verification (Codex + Gemini); parent-walk bounds, executable
create-folder remedy, and no-rollback note added (Gemini + Codex); MAX_PATH threshold made
conservative, hint phrased as possible cause, checkout-phase limitation stated (Codex + Gemini);
2d re-Stat branches on node type (Codex); live gate extended with self-heal provocation,
`ls-remote` advertisement assertion, explicit notes fetch, and both folder-trash contract rows
(Codex). **Rejected:** traversal/cancellation limits on listing (YAGNI — cost stated instead;
ref trees are user-bounded in practice); rollback of partially-created parents (reintroduces the
unsafe folder-delete race for zero benefit); treating mid-batch namespace-root re-creation
failure as a new class (non-atomic batches with per-ref status already cover it); "delete-first
is data loss" as framed (a requested deletion honoured before an unrelated create fails is git's
own non-atomic remote semantics — the real defect was deletions preceding pack confirmation,
which is fixed).

**Round 2 (Codex + Gemini, 2026-08-06) — applied:** self-heal's emptiness test made recursive
— "subtree contains no ref files," never first-level emptiness — closing the nested-empty
residue wedge, with the refusal naming actual ref files and never an empty directory (both
engines: Codex as Major, Gemini as the diagnostic corollary); read-boundary name validation
changed from fatal to skip-with-loud-note per the packs-directory precedent — a stray foreign
name must not brick every operation on the repo (Gemini) — and moved in-process so
advertisement never spawns one `git check-ref-format` per ref (Codex's O(refs) cost finding);
push-side validation keeps authoritative `git check-ref-format`; 2b's invariant reworded to "no
ref mutation and no destructive operation precedes phase 4" (phase 3 writes immutable objects —
orphan-on-failure is the accepted class), phase-3 failure control flow made explicit — creates/
updates error, execution continues into phase 4, deletions run with their own statuses
(adjudicated in Gemini's direction; Codex asked only that it be defined); preflight's claim
scoped to the final *ref set*, with empty-folder collisions explicitly left to phase-5
self-heal (Gemini); marker absent-vs-error given its mechanism — the transport contract's
`Stat` absence seam, no CLI error-text parsing — plus a contract row pinning the certified
CLI's not-found signature (Codex); component 8 gains the nested-empty hermetic case and the
phase-3-continues-into-phase-4 pin.
