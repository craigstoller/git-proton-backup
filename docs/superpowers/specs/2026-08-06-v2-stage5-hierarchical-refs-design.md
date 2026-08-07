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
- **The v6.1 namespace rejection is retired.** Recursion makes `refs/notes/*`,
  `refs/replace/*`, `refs/stash`, and other valid namespaces visible without a filter, so they
  are advertised and pushable. v6.1's first justification (non-recursive `ListRefs`) is erased
  by this component; its second (expensive misleading failure, orphan packs) is erased by
  parent creation in component 2. The push rule for these namespaces is the design's existing
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

**2b. Deletes before creates within a batch.** The helper reorders each buffered push batch so
deletions execute before creates and updates, making `git push origin :feature feature/x` (and
the reverse order git may send) succeed in one batch. Per-ref status responses are unchanged —
each `ok`/`error` names its `<dst>`, and the protocol does not require response order to match
command order. The poison-flag rule is unaffected: enforcement still happens after the full
batch is buffered, before any mutation.

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
  and a `Stat` shows a **folder** at that name, `List` it
  under the lock: **empty → verify-then-`Trash` the folder and retry the create once**;
  non-empty → refuse, naming the live sub-refs as the conflict (git itself refuses this D/F
  state locally). A `Refused` against an existing ref **file** keeps its current meaning —
  concurrent creator, report, never overwrite.
- **Reverse D/F (component blocked by a ref file):** creating `refs/heads/feature/x` when
  branch `feature` exists as a file and is **not** deleted earlier in this batch → refuse with
  a named reason mirroring git's own local directory/file conflict rule. (If the batch deletes
  it, rule 2b makes the create succeed.)

**2d. `EnsureDir` contradiction handling (generic robustness, NOT a validated C17 fix).** If
`create-folder` fails with already-exists after a `Stat` reported absent (the C17 signature,
observed once, never reproduced under provocation — C17b), the helper re-`Stat`s once: present
→ proceed as if the folder existed all along; still absent → fail, quoting **both**
observations verbatim so the contradiction is diagnosable. Framed exactly this way in code
comments and the design doc: C17b's ruling stands — this can only be justified as generic
robustness, never claimed as a live-validated fix.

## Component 3 — `--set-head` hierarchical support

The slash refusal (which names Stage 5 today) is lifted. Short names normalise to
`refs/heads/<name>` including embedded slashes; full `refs/heads/…` forms accepted as today;
anything outside `refs/heads/` still refused. Components validated per rule 2a. The remote
existence check recurses per component 1. Everything else — lock lifecycle,
verify-branch-before-idempotence-short-circuit, `UpdateHEAD`/`WriteHEAD` split, outcome
handling — is untouched.

## Component 4 — Quarantine fetch staging (retro-Codex 3)

Fetch downloads land in a **per-fetch incoming directory**, not in the temp alternate:

- The incoming dir is created at fetch start and **deleted wholesale at fetch end, success or
  failure**; any residue from a crashed prior fetch at that location is deleted at start.
  Nothing in it is ever read as repo state.
- Verification runs **in quarantine**: checksum-vs-basename immediately on a `.pack`'s
  arrival; `index-pack --verify` on the pair as soon as both members are local (unchanged
  timing rules from the error table).
- Only fully verified pairs are **published** into the temp alternate's `pack/` directory,
  **pack before idx**, so no reader (rev-list, the map builder) ever observes an unverified or
  incomplete pair. The Stage 4 polish trace assertion (`.pack` before re-downloaded `.idx` in
  the mid-round pair refresh) must keep passing — publication preserves that ordering.
- **Deleted by this refactor:** the failed-attempt residue-deletion rule and the entire F2
  failure class (partially-downloaded or unverified files visible to planning/mapping code).
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
  git-remote-proton repo") distinguishes a **confirmed absence** (transport succeeded, node not
  found → today's message) from a **failed read** (transport errored → "could not read
  <path>: <cause>"). Kills the Stage 4 gate 2b masquerade where a broken CLI under
  `GPB_UNCERTIFIED_CLI=1` surfaced as "not a repo".
- **MAX_PATH.** README gains a Windows section: deep clone destinations can exceed 260 chars;
  remedies are `git config core.longpaths true` or a shorter destination. Helper-side: when a
  **spawned git command fails** and a computed relevant target path exceeds 260 characters on
  Windows, append a one-line hint naming both remedies. The trigger is **path-length
  arithmetic, never stderr pattern-matching** (git messages are localised; lengths are not).
  Best-effort: no detection is promised for paths git computes internally beyond what the
  helper can see.

## Component 6 — UX: opt-in parent auto-create

Missing parents of the repo root (Surprise R2-1: raw `Node not found: GitRemotes`) become:

- **Default (unset): an actionable refusal.** Names the first missing parent, and both
  remedies: create it (web UI or `proton-drive filesystem create-folder …`), or set the
  opt-in env var. Typo'd addresses fail loudly instead of manufacturing folder trees.
- **`GPB_CREATE_PARENTS=1`** (exact name fixed here; same conventions as
  `GPB_UNCERTIFIED_CLI`: read fresh from the environment every invocation, never cached or
  remembered): the `EnsureDir` walk extends **above** the repo root, creating each missing
  parent inside the canonicalised root's bounds (under `/my-files` or `/devices` only —
  canonicalisation already rejects everything else), with a **loud stderr line per created
  folder**. Honoured by the push bootstrap and by `--set-head`; an env var, not a flag,
  because git invokes the helper on the protocol path and the user cannot pass flags.
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
  now depends on: `Trash` on a folder (outcome shape), `create-folder` colliding with a file
  and vice versa (error shape), nested `List`, `upload` of a file colliding with a folder
  name. Fake and CLI halves must agree per the existing decorator pattern.
- RED/GUARD labelling, which-assertion-fired reporting, and deliberate-regression checks per
  the runbook — mandatory for prune, self-heal, batch reordering, and the quarantine publish
  ordering, which are the load-bearing new behaviours.
- Sha-collision hygiene in fixtures (pin `GIT_COMMITTER_DATE` where content repeats).

**Live gate (one, at stage end), in outline:**
1. Hierarchical end-to-end: push `feature/x` + a nested tag + a `refs/notes/*` ref; clone
   (nested refs advertised and checked out correctly); incremental fetch.
2. The D/F workflow: delete `feature/x` (observe prune — the parent folder is gone from the
   listing), then push branch `feature` (clean-path create). Provoking the self-heal branch
   (a leftover folder in the way) is hermetic-only; the live gate asserts the clean path.
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
- Push: batch reordering rule (2b).
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
