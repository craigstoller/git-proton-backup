# Stage 5 live gate brief — v0.4.0 hierarchical refs, prune/self-heal, `--set-head`, `GPB_CREATE_PARENTS`

This is a **brief** (prospective instructions for the gate runner), not a record. It implements
design spec component 8's "Live gate (one, at stage end), in outline"
(`docs/superpowers/specs/2026-08-06-v2-stage5-hierarchical-refs-design.md`, Component 8) **verbatim**,
in the same numbered order, fleshed out with the exact commands, expected wording, and BLOCK
conditions the Stage 5 implementation tasks actually produced. Structure follows
`docs/research/gates/stage4-gate.md` (Preconditions → numbered steps → Cleanup → Confinement
attestation → Verdict) for house style.

**This brief incorporates `docs/research/gates/brief-checklist.md` BY REFERENCE in full.** Every
rule in that checklist (row-set listing comparisons, explicit write-confinement, verify-before-
trash with full subtree enumeration, BLOCKED-verbatim-never-patch, `-count=1` on every `go test`,
empty-trash-before-gate, trash accounting counts pruned folders) applies to every step below
without being re-derived. Where a rule is especially load-bearing for a specific step in this
stage, it is also called out inline at that step — that is a pointer back to the checklist, not a
new or different rule.

Release integration follows `docs/releasing.md` step 5: **this gate runs against the v0.4.0
DRAFT's bytes** (downloaded from the draft, digests staged before any account write), never
against a locally-built exe. Tags are never moved after artifacts exist. The publication digest
closure (`docs/releasing.md` step 7) is a separate, later, read-only pass after Craig publishes —
outline step 7 below.

---

## Preconditions

1. **CLI / git versions.** Record `proton-drive --version` (must be the certified
   `cli-drive@0.7.0+5174900c` build — see the allowlist note below) and `git --version`.
2. **PATH shadow check** (Stage 4 lesson, still binding): in a genuinely fresh shell
   (`$env:Path = [Environment]::GetEnvironmentVariable('Path','Machine') + ';' +
   [Environment]::GetEnvironmentVariable('Path','User')`), confirm `(Get-Command
   git-remote-proton -All).Source`'s FIRST hit is the draft-installed helper. Any shadowing =
   BLOCKED, per the same rule stage4-gate.md's run 1 enforced.
3. **Empty the trash before the gate** (checklist item 6) — cheap insurance, taken on report from
   Craig's web-UI action unless the CLI can verify it directly.
4. **Pre-run `/my-files` listing**, recorded before anything else runs, row-set form (checklist
   item 1): `uid`, `name`, `type`, `creationTime`, `modificationTime`, `parentUid` for every row.
   This is the baseline the cleanup step's post-run listing must match, row-set-for-row-set.
5. **Write confinement, stated explicitly (checklist item 2):** all writes are confined to
   `/my-files/GitRemotes/<stage5-gate-demo-name>`, plus the gate-authorized creation of
   `/my-files/GitRemotes` itself if absent (same authorization stage4-gate.md run 2 exercised for
   Surprise R2-1). **Outline step 5 below additionally and explicitly authorizes parent creation
   OUTSIDE the repo root** for the `GPB_CREATE_PARENTS=1` half — see that step for the exact paths.
   Untouchables (`GitBackups`, `Sensitive Project Sources`, `Project Repo Bundles`, `ChatGPT
   Export Text Backup`) are read-only: they may appear only as rows in listings, never named in any
   write command.

---

## Signature-constant pins — BLOCK rule (applies throughout, load-bearing for steps 1 and 2)

Two hardcoded string constants in `internal/transport/cli.go` drive fail-closed classification
logic added or hardened this stage, and this gate is the **first and only** live evidence for one
of them:

- **`notFoundSignature = "Node not found"`** (`internal/transport/cli.go:142`). Live evidence for
  this string exists **only** for `filesystem list` and `filesystem create-folder`
  (`docs/research/gates/stage3b-gate.md`, `stage4-gate.md`). The **`filesystem info` subcommand's**
  not-found text — which `Stat`'s absent-vs-error split now depends on (Task 4) — is pinned **only**
  by this gate's new live contract row (outline step below: `Stat` on a confirmed-missing path,
  e.g. `proton-drive filesystem info /my-files/GitRemotes/<demo>/refs/heads/does-not-exist --json`).
  Record the verbatim stderr/stdout.
- **`alreadyExistsSignature = "already exists"`** (`internal/transport/cli.go:251`, added Task 9b).
  This is a HYPOTHESIS constant (its own doc comment says so): the create-folder-collision live
  contract row (`internal/transport/contract_test.go`, Task 9b's new row) pins the ordinary,
  deterministically-provoked wording, but the C17 contradiction itself (Stat says absent,
  create-folder then says already-exists) remains unreproduced per
  `docs/research/probes/c17b-provocation-log.md`.

**Rule: if either live observation differs from the shipped constant's text, the gate reports
BLOCKED with the verbatim output and does NOT patch — no adjusting `notFoundSignature` or
`alreadyExistsSignature` in the field, no retrying, no "close enough" judgment call.** These two
contract rows are not decoration; they are the reason the gate exists as a *release* gate and not
just a smoke test. Do not rubber-stamp them by skimming past a PASS without reading the actual
recorded string.

---

## Outline step 1 — Hierarchical end-to-end

Per spec component 8 outline item 1, and covering Component 1/2's advertisement and fetch
behavior:

1. Bootstrap a fresh remote at `/my-files/GitRemotes/<demo>` (creating the parent `GitRemotes` if
   absent, per the write-confinement authorization above).
2. Push, in one or more batches: `refs/heads/feature/x` (a nested branch), a nested **tag**
   (e.g. `refs/tags/release/v1`), and a `refs/notes/*` ref (e.g. `refs/notes/commits`, pointing at
   a note object).
3. **Advertisement assertion via `git ls-remote proton-v2::/my-files/GitRemotes/<demo>`** — this is
   the load-bearing check, not a clone. Assert all three pushed refs appear in the advertisement
   (`refs/heads/feature/x`, the nested tag, `refs/notes/commits`), because **a plain `git clone`
   does NOT prove namespace coverage** — git's own clone only fetches `refs/heads/*` (+ HEAD) by
   default and never touches `refs/notes/*`, so a clone succeeding would be silent on whether notes
   were ever advertised at all.
4. `git clone proton-v2::/my-files/GitRemotes/<demo>` into a short local path (Stage 4's R2-2
   MAX_PATH lesson — keep the clone destination shallow). Confirm the nested branch and tag are
   present locally; confirm `refs/notes/commits` is **absent** locally (clone alone does not fetch
   it — this is expected, not a defect).
5. **Explicit fetch of the notes namespace**: `git fetch proton-v2 "refs/notes/*:refs/notes/*"`.
   Verify the resulting local `refs/notes/commits` OID matches exactly what was pushed in step 2.
6. **Incremental fetch**: push one more small change (e.g. a new commit on `feature/x`), then
   `git fetch proton-v2` again from the same clone and confirm only the new object(s) download —
   this doubles as a lead-in to outline step 4's quarantine no-regression check.

## Outline step 2 — D/F workflow, including self-heal live

Per spec component 8 outline item 2, exercising `internal/repo/push.go`'s prune
(`pruneEmptyParents`) and self-heal (`createRefHealingCollision`) — the composed
`create → Stat → List → Trash → retry` path the design doc calls out as exactly where this
project's history says seams lie.

1. Delete `feature/x` (`git push proton-v2 --delete feature/x`). **Observe prune in the listing**:
   `proton-drive filesystem list /my-files/GitRemotes/<demo>/refs/heads --json` (or the parent
   `feature` folder specifically) must show the `feature` folder **gone**, not merely empty — prune
   trashes the folder itself, it does not just empty it (checklist item 7, and see "trash
   accounting" in Cleanup below).
2. Push `feature` (a plain branch, not nested) — clean-path create against the now-vacated name
   space; confirm success.
3. **Manufacture the two collision states BY HAND, inside the gate repo only** — never against a
   fresh/foreign repo, and never the foreign-file variant (see below):
   - **Empty folder at a branch name**: using `proton-drive filesystem create-folder`, create an
     empty folder at `/my-files/GitRemotes/<demo>/refs/heads/heal-empty` (no ref file inside it —
     simulating a crashed prune's leftover residue). Then `git push proton-v2 <local-branch>:heal-empty`.
     **Expect the heal**: loud stderr containing `"cleared what is likely residue of an interrupted
     delete — an empty folder at refs/heads/heal-empty"` (the exact wording `push.go`'s
     `createRefHealingCollision` emits), the folder trashed, and the push landing successfully on
     retry.
   - **Folder containing a live ref file**: create
     `/my-files/GitRemotes/<demo>/refs/heads/heal-refused/sub` as a genuine ref file (upload 40 hex
     chars + newline, or push a real branch named `heal-refused/sub` first so it's a legitimate
     nested branch). Then `git push proton-v2 <local-branch>:heal-refused` (colliding at the parent
     folder). **Expect the refusal**: the push fails, and the error names the live sub-ref path
     (`refs/heads/heal-refused/sub`) as `"a conflicting ref, currently <sha>"` — per
     `describeBlockers`'s content-based classification (`internal/repo/push.go`) — and **nothing is
     trashed** (verify with a `filesystem list` immediately after: `heal-refused/sub` still present).
   - **The foreign-file variant stays hermetic-only.** Do **not** manufacture a non-ref foreign
     file (e.g. a stray `notes.txt`) inside a collision folder live, and do not push against one, to
     "trash-test" it. That case is already covered by `TestCreateRefusesFolderWithForeignFileUntouched`
     against the Fake (Task 9b) — the whole point of that hermetic-only rule is that a live gate must
     never be the first place foreign-data destruction is provoked.
4. This composed sequence (delete → prune-observed-in-listing → create → manufactured-collision →
   heal/refuse) is deliberately run **live**, against the real CLI, because the hermetic suite
   (`internal/repo/repo_test.go`, `TestCreateSelfHealsEmptyFolderCollision` and siblings) exercised
   this only against `transport.Fake`. This step is the live half Task 9a/9b's reports explicitly
   flagged as gate territory.

## Outline step 3 — `--set-head` to a nested branch + delete-protection + namespace-folder refusal

Per spec component 8 outline item 3, plus the Task 10 fix discovered mid-stage:

1. With both `main` (or another existing branch) and `refs/heads/feature/x` on the remote (push it
   again if step 2 deleted it), run `git-remote-proton --set-head proton::/my-files/GitRemotes/<demo>
   feature/x`. Expect `HEAD is now refs/heads/feature/x`, exit 0.
2. Verify via `proton-drive filesystem download .../HEAD` and via `git clone` (fresh clone checks
   out `feature/x`) — same decisive-postcondition pattern as `stage4-gate.md` run 2 §3.4–3.5.
3. **Delete-protection follows HEAD**: `git push proton-v2 --delete feature/x` must be **refused**,
   naming `--set-head` as the remedy — exact text (`internal/repo/push.go`):
   `"refusing to delete the branch HEAD points at (refs/heads/feature/x); change the default branch
   first (git-remote-proton --set-head <url> <branch>)"`. Deleting the **old** default (whatever
   HEAD pointed at before step 1) must instead **succeed** — the decisive pair, same as
   `stage4-gate.md` run 2 §3.6.
4. **New this stage — the namespace-folder refusal (Task 10's fix)**: with only `refs/heads/feature/x`
   existing (no bare `feature` branch), run `git-remote-proton --set-head proton::/my-files/GitRemotes/<demo>
   feature`. Expect a **refusal naming the situation and suggesting the real branch**, not a
   misleading "not found" — exact wording (`internal/repo/sethead.go`):
   `"cannot set HEAD to \"feature\": refs/heads/feature is a namespace folder containing other
   branches, not a branch itself; branches that exist: feature/x"` (branch list may include others
   present on the remote at the time). This is the live half of
   `TestSetHeadRefusesNamespaceFolderBranch` (Fake-only, Task 10 fix round) — confirm the live CLI's
   behavior on `filesystem download` of a directory path, which had **no verified contract at all**
   before this gate (Task 10 report, "Concerns" section).

## Outline step 4 — Quarantine no-regression

Per spec component 8 outline item 4, exercising Task 6's per-fetch quarantine staging
(`internal/repo/fetch.go`) with zero regression versus pre-Stage-5 fetch behavior:

1. **Mid-round pair refresh**: force a fetch large enough to need more than one packing round (a
   few dozen small commits, or reuse the incremental fetch from outline step 1.6 immediately
   followed by another small push+fetch cycle) so a `.pack`/`.idx` pair is re-downloaded
   mid-round. Confirm the fetch completes without error and without residue in the local `.git`
   (this is the live analogue of `TestFetchMidRoundPairRefreshWithTwoPacksCompletes`, hermetic-only
   until now).
2. **Up-to-date re-fetch, zero `packs/` downloads**: immediately re-run `git fetch proton-v2` with
   nothing new to fetch. Confirm exit 0, no error, and — this is the assertion that actually
   matters — **zero `gpb: downloaded .../packs/...` lines** in the output (compare against
   `stage4-gate.md`'s transcripts, which show exactly this pattern for a genuine up-to-date case).
   A single unnecessary pack download here would indicate the quarantine staging regressed fetch's
   already-up-to-date short-circuit.

## Outline step 5 — `GPB_CREATE_PARENTS` both modes

Per spec component 8 outline item 5, exercising `internal/repo/parents.go`'s `EnsureParents`:

1. **Unset (default) — actionable refusal, listing unchanged as a ROW SET.** Point a fresh push at
   a repo root whose immediate parent does not exist yet
   (e.g. `/my-files/GitRemotes/<demo>-parents/nested/repo`, where `GitRemotes` exists but
   `<demo>-parents` and `<demo>-parents/nested` do not). With `GPB_CREATE_PARENTS` unset, run
   `git push proton-v2 main`. Expect **exit non-zero**, and stderr containing the exact actionable
   grammar (`internal/repo/parents.go`):
   `"parent folder /my-files/GitRemotes/<demo>-parents does not exist; create it first (proton-drive
   filesystem create-folder /my-files/GitRemotes <demo>-parents, or the web UI), or set
   GPB_CREATE_PARENTS=1 to let the helper create missing parents"`. Immediately after, take a
   `/my-files/GitRemotes` listing and assert it is **row-set-identical** to the listing taken
   immediately before this push attempt (checklist item 1 — compare `uid`/`name`/`type`/
   `creationTime`/`modificationTime`/`parentUid` fields, never raw JSON byte equality) — nothing was
   created.
2. **Set — parents created with loud stderr, torn down in cleanup.** Set `GPB_CREATE_PARENTS=1` for
   this one shell/command only (never persist it), retry the same push. Expect **exit 0**, and
   stderr containing one loud line **per created parent**, exact wording
   (`internal/repo/parents.go`): `"git-remote-proton: created parent folder
   /my-files/GitRemotes/<demo>-parents (GPB_CREATE_PARENTS=1)"` followed by the same for
   `/my-files/GitRemotes/<demo>-parents/nested`. Confirm the push itself then succeeds (the repo
   root folder and its marker land normally via `Bootstrap`, unaffected by `EnsureParents`).
   **Write-confinement authorization, stated explicitly per checklist item 2:** this step is the
   ONE place in this gate that authorizes writes to paths under `/my-files/GitRemotes/` that are
   **not** the demo repo root itself — specifically `/my-files/GitRemotes/<demo>-parents` and
   `/my-files/GitRemotes/<demo>-parents/nested`, both created by `EnsureParents` and both to be
   torn down (trashed) explicitly in Cleanup, verify-before-trash, alongside the main demo subtree.
   `GPB_CREATE_PARENTS` never applies to `/my-files` or `/devices/<id>` themselves (the mount is
   never creatable in either mode) — do not attempt to provoke that path live; it is covered by
   `TestEnsureParentsNeverCreatesMountRoots` against the Fake.

## Outline step 6 — Cleanup, row-set comparisons, trash accounting

Per spec component 8 outline item 6, and checklist items 1/3/7 inline:

- **Verify-before-trash with full subtree enumeration** (checklist item 3): before any `trash`
  command, `filesystem list` the full subtree being trashed and confirm it contains only this
  gate's own artifacts — the demo repo subtree from outline steps 1–4, and the two parent folders
  created in outline step 5. Do not trash a folder without having first listed everything beneath
  it, same discipline as `stage4-gate.md` run 2's cleanup section.
- **All listing comparisons as row sets** (checklist item 1) — every pre/post comparison in this
  gate, not only the final cleanup one.
- **Trash accounting counts pruned folders** (checklist item 7, new standing rule this stage): when
  tallying what this gate run sent to trash for the record, include the folder outline step 2.1's
  delete-and-prune sent to trash, not just the files pushed and later explicitly trashed in
  cleanup. A files-only count understates what actually left the account's live tree this stage,
  since prune (unlike prior stages) now removes folders, not just leaves them empty.
- **Post-run `/my-files` listing** must be row-set-identical to the pre-run listing from
  Preconditions step 4 (no `trashTime` on any of the four untouchable rows; same uids, same
  creation/modification times).

## Outline step 7 — Release integration

Per spec component 8 outline item 7, and `docs/releasing.md` steps 4–7:

- **This gate runs against the v0.4.0 DRAFT's bytes**, not a local build. Download the draft's
  three assets (`git-remote-proton.exe`, `git-remote-proton.exe.sha256`, `install.ps1`) into a
  fresh, empty gate directory; record SHA-256 for all three as the **staged baseline** before any
  account write, exactly as `stage4-gate.md` §1.1–1.2 did for v0.3.1. Verify the sidecar
  independently (recompute, don't trust the installer's own check). Run the installer; confirm a
  fresh shell resolves the installed helper first (Preconditions step 2) and `--version` reports
  `git-remote-proton v0.4.0 (certified CLI: cli-drive@0.7.0+5174900c)`.
- **Tags are never moved after artifacts have been built from them.** If this gate finds a defect,
  the fix ships as a new tag (`v0.4.1` or later), never a retag of `v0.4.0` — same rule as
  `docs/releasing.md` step 5.
- **Publication digest closure happens AFTER Craig publishes**, as a separate, later, read-only
  pass: re-download every published asset into a new empty directory, hash, and compare against
  the staged digests recorded above (three-way: recorded-vs-published, staging-dir-rehash-vs-
  recorded, byte-for-byte) — same structure as `stage4-gate.md`'s "Publication digest closure"
  section. Only after that closure PASSes is the release final. This gate brief's own verdict
  (from outline steps 1–6) remains **provisional** until the closure runs, exactly as
  `stage4-gate.md` run 2's verdict was provisional until its closure.

---

## Confinement rules (restated per checklist item 2, unchanged in substance from Stage 4)

- Writes only under `/my-files/GitRemotes/<demo>` and, for outline step 5 specifically,
  `/my-files/GitRemotes/<demo>-parents` and `/my-files/GitRemotes/<demo>-parents/nested` (see that
  step for the explicit authorization) — plus the gate-authorized creation of `/my-files/GitRemotes`
  itself if absent.
- Untouchables (`GitBackups`, `Sensitive Project Sources`, `Project Repo Bundles`, `ChatGPT Export
  Text Backup`) read-only: rows in listings only, never named in a write command.
- Verify-before-trash, full subtree enumeration, every time (checklist item 3).
- **Report BLOCKED with verbatim output, never patch** (checklist item 4) — applies with extra
  force to the two signature-constant pins above: a mismatch there is not a "fix the constant and
  continue" situation, it is a BLOCK.
- `-count=1` on every `go test` invocation run as part of this gate (checklist item 5) — including
  any hermetic re-run used to cross-check a live observation.
- `GPB_LIVE_ACCOUNT` is set only for the specific live contract-row test runs this brief calls for,
  never left set across shells. `GPB_UNCERTIFIED_CLI` is never set at all in this gate (the draft
  is gated against the certified CLI only).
