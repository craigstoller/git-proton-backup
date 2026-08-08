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

**Remote naming convention used throughout this brief — do not conflate the two.** `proton::<path>`
is the URL **scheme** `git-remote-proton` registers with git (`README.md:97`). Use the literal
scheme for every command that talks to the helper before any local git remote exists for that
path, and for direct invocations of the helper binary itself: the first `git clone -o proton-v2
proton::...` in a given local repo, `git ls-remote proton::...`, and every `git-remote-proton
--set-head proton::...` call. `proton-v2` is a **local remote alias**, not a URL scheme — it exists
only after `git clone -o proton-v2 proton::...` or `git remote add proton-v2 "proton::..."` has run
in that specific local repo, and every subsequent git command in that repo (`git push proton-v2
...`, `git fetch proton-v2 ...`) uses the alias. **`proton-v2::` is never valid anywhere** — git
would look for a nonexistent `git-remote-proton-v2` binary and fail outright. Every command below
has been checked against this distinction.

---

## Preconditions

1. **CLI / git versions.** Record `proton-drive --version` (must be the certified
   `cli-drive@0.7.0+5174900c` build) and `git --version`. See "Allowlist — not re-provoked live
   this stage" immediately below for why this gate only records the version rather than also
   re-running the refusal/override provocation.
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

### Allowlist — not re-provoked live this stage

The certified-CLI allowlist (`internal/transport/cli.go`'s version check; refusal naming the
uncertified build plus `GPB_UNCERTIFIED_CLI`, loud-warning override) is **untouched by any Stage 5
task** — no code in that path changed. It was already proven live, against the real certified CLI
and a scripted shim standing in for an uncertified one, at the Stage 4 gate
(`docs/research/gates/stage4-gate.md`, run 2, steps 2a/2b). Task 13 (this stage's stretch task)
additionally built a **hermetic** end-to-end harness for the identical refusal/override path
(`internal/testcli`, `cmd/git-remote-proton/shim_test.go`) that runs on every `go test ./...`, no
live account required. Given both, this gate's Preconditions step 1 only needs to **confirm the
installed CLI is the certified build** (the ordinary case every other step already depends on) —
re-running the shim-based refusal/override provocation live here would re-prove something already
proven live once and covered hermetically ever since, for no new information. If a future stage
changes the allowlist logic itself, that change should get its own live provocation here; this
stage's changes never touch it.

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

1. **Bootstrap a fresh remote and land the initial branch** — creates
   `/my-files/GitRemotes/<demo>` (the parent `GitRemotes` created first if absent, per the
   write-confinement authorization above), the repo marker, and `refs/heads/main`. Spelled out
   exactly, run from a fresh local working directory — a runner must never improvise this step,
   since outline step 3 below is the load-bearing check immediately downstream of it:
   ```
   git init -b main <demo>
   cd <demo>
   git commit --allow-empty -m "gate: stage5 initial commit"
   git remote add proton-v2 "proton::/my-files/GitRemotes/<demo>"
   git push -u proton-v2 main
   ```
   (`-b main` is explicit rather than relying on `init.defaultBranch`: every later step in this
   brief names `main` literally, and a shell configured to `master` would derail all of them.)
2. **Push the nested branch, a nested tag, and a `refs/notes/*` ref** — same repo as item 1, same
   `proton-v2` alias, each an explicit command:
   ```
   git switch -c feature/x
   git commit --allow-empty -m "gate: nested branch commit"
   git push proton-v2 feature/x
   git tag release/v1
   git push proton-v2 release/v1
   git notes add -m "gate: note on the feature/x commit"
   git push proton-v2 refs/notes/commits:refs/notes/commits
   ```
   (`git notes add` without `-r` annotates whatever commit is currently checked out — `feature/x`'s
   commit at this point in the sequence; which commit it annotates is incidental, since this step
   only needs the ref to exist and carry a verifiable OID, not any particular note content.)
3. **Advertisement assertion via `git ls-remote proton::/my-files/GitRemotes/<demo>`** — note the
   URL **scheme** here, not the `proton-v2` alias: `ls-remote` takes a URL directly and does not
   need (and at this point in a fresh shell may not even have) a local remote configured. This is
   the load-bearing check, not a clone. Assert all three pushed refs appear in the advertisement
   (`refs/heads/feature/x`, the nested tag, `refs/notes/commits`), because **a plain `git clone`
   does NOT prove namespace coverage** — git's own clone only fetches `refs/heads/*` (+ HEAD) by
   default and never touches `refs/notes/*`, so a clone succeeding would be silent on whether notes
   were ever advertised at all.
4. `git clone -o proton-v2 proton::/my-files/GitRemotes/<demo> <short-local-path>` into a short
   local path (Stage 4's R2-2 MAX_PATH lesson — keep the clone destination shallow). The `-o
   proton-v2` names the new clone's remote to match every later step's assumption — a plain `git
   clone <url>` names it `origin`, which item 5's `git fetch proton-v2 ...` would not find. Confirm
   the nested branch and tag are present locally; confirm `refs/notes/commits` is **absent**
   locally (clone alone does not fetch it — this is expected, not a defect).
5. **Explicit fetch of the notes namespace**, run inside the item-4 clone (where the remote is
   named `proton-v2` because of `-o proton-v2` above): `git fetch proton-v2
   "refs/notes/*:refs/notes/*"`. Verify the resulting local `refs/notes/commits` OID matches
   exactly what was pushed in item 2.
6. **Incremental fetch**: back in the ORIGINAL gate repo from items 1–2 (not the item-4 clone),
   push one more small change (`git commit --allow-empty -m "gate: incremental commit"; git push
   proton-v2 feature/x`); then, in the item-4 clone, `git fetch proton-v2` again and confirm only
   the new object(s) download — this doubles as a lead-in to outline step 4's quarantine
   no-regression check.

## Outline step 2 — D/F workflow, including self-heal live

Per spec component 8 outline item 2, exercising `internal/repo/push.go`'s prune
(`pruneEmptyParents`) and self-heal (`createRefHealingCollision`) — the composed
`create → Stat → List → Trash → retry` path the design doc calls out as exactly where this
project's history says seams lie.

**Every command in this step runs from the ORIGINAL gate repo of outline step 1 items 1–2** — the
one whose `proton-v2` alias points at `/my-files/GitRemotes/<demo>` — not the outline step 1 item 4
clone. `main` and `feature/x` both still exist as LOCAL branches there; `feature/x` is the checked-out
one.

1. **Delete `feature/x` and observe the prune.**
   ```
   git push proton-v2 --delete feature/x
   ```
   Remote HEAD still points at `refs/heads/main` at this point, so delete-protection does not fire
   (that is outline step 3's business). **Observe prune in the listing**:
   `proton-drive filesystem list /my-files/GitRemotes/<demo>/refs/heads --json` (or the parent
   `feature` folder specifically) must show the `feature` folder **gone**, not merely empty — prune
   trashes the folder itself, it does not just empty it (checklist item 7, and see "trash
   accounting" in Cleanup below).
   `git push --delete` also removes the local remote-tracking ref `refs/remotes/proton-v2/feature/x`.
   If it survives for any reason, clear it (`git update-ref -d refs/remotes/proton-v2/feature/x`)
   before item 2: git cannot create `refs/remotes/proton-v2/feature` while
   `refs/remotes/proton-v2/feature/x` exists, and that purely LOCAL directory/file failure would
   look like a helper defect.
2. **Clean-path create at the just-vacated name** — spec outline item 2's "push branch `feature`",
   spelled as a **refspec** push:
   ```
   git push proton-v2 main:feature
   ```
   Confirm success, and confirm `refs/heads/feature` is now a **file** in the `refs/heads` listing.
   Two reasons for the refspec form rather than a bare `git push proton-v2 feature`:
   - There is no local branch `feature`, and `git branch feature` would **fail locally** — the
     local `refs/heads/feature/x` from outline step 1 item 2 is still there (item 1 deleted only
     the remote copy), and git's own directory/file rule refuses `refs/heads/feature` beside it.
     `main:feature` needs no local ref of that name.
   - It keeps the step's actual content intact: this creates a ref **file** at the exact name a
     **folder** occupied moments earlier, so it only succeeds if item 1's prune genuinely vacated
     the name rather than leaving a trashed-homonym blocker behind.
3. **Manufacture the two collision states BY HAND, inside the gate repo only** — never against a
   fresh/foreign repo, and never the foreign-file variant (see below):
   - **Empty folder at a branch name (the heal).** Create the folder with
     `proton-drive filesystem create-folder /my-files/GitRemotes/<demo>/refs/heads heal-empty`
     (no ref file inside it — simulating a crashed prune's leftover residue). Then:
     ```
     git push proton-v2 main:heal-empty
     ```
     **Expect the heal**: loud stderr containing `"cleared what is likely residue of an interrupted
     delete — an empty folder at refs/heads/heal-empty"` (the exact wording `push.go`'s
     `createRefHealingCollision` emits), the folder trashed, and the push landing successfully on
     retry.
   - **Folder containing a live ref file (the refusal) — expect the BATCH PREFLIGHT, not
     `describeBlockers`.** Create the nested ref by pushing it, then push the colliding parent:
     ```
     git push proton-v2 main:heal-refused/sub
     git push proton-v2 main:heal-refused
     ```
     **Expect the second push to be refused by rule 2b, the final-state D/F preflight** — exact
     text (`internal/repo/push.go`): `"refs/heads/heal-refused conflicts with
     refs/heads/heal-refused/sub: a ref cannot be both a leaf and a folder containing other refs"`.
     The push fails, **nothing is trashed, and no pack is even built** — the preflight runs in
     phase 2, before phase 3. Verify with a `filesystem list` immediately after:
     `refs/heads/heal-refused/sub` still present, unchanged.
     **Why not `describeBlockers`' `"a conflicting ref, currently <sha>"` wording**, which an
     earlier draft of this brief expected here: `refs/heads/heal-refused/sub` is a well-formed,
     well-named ref, so the helper's own `ListRefs` advertises it, so it is in the batch's final
     ref set, so rule 2b (Task 9a, written after the spec outline) refuses the create in phase 2 and
     `createRefHealingCollision` is never reached. Reaching its runtime refusal live would require
     the blocker to be invisible to the advertisement — either foreign data (hermetic-only, next
     bullet) or a sub-ref created inside the window between the helper's `ListRefs` and phase 5,
     which is a race and not deterministically provokable. That path stays hermetic
     (`TestCreateRefusesFolderWithLiveSubRefs`, `internal/repo/repo_test.go`). What this bullet
     buys live is rule 2b's **first live evidence**, which is new this stage and worth having.
   - **The foreign-file variant stays hermetic-only.** Do **not** manufacture a non-ref foreign
     file (e.g. a stray `notes.txt`) inside a collision folder live, and do not push against one, to
     "trash-test" it. That case is already covered by `TestCreateRefusesFolderWithForeignFileUntouched`
     against the Fake (Task 9b) — the whole point of that hermetic-only rule is that a live gate must
     never be the first place foreign-data destruction is provoked.
4. **One-batch prune → create at the same parent. ADDITION beyond the spec's outline** — the spec's
   component 8 item 2 does not call for this; it is added here because **this is the only shape in
   the whole gate where `EnsureDir` is asked to create a folder at a name the SAME batch trashed
   moments earlier** — the trash-homonym that produced the C17 signature, and the sole live target
   of `reobserveEnsureDirContradiction`, which has never run against a real account.
   ```
   git push proton-v2 :refs/heads/heal-refused/sub main:refs/heads/heal-refused/sub2
   ```
   **Note the refspec form: `--delete` cannot be used here.** `git push --delete` is defined as
   "prefix a colon to *all* listed refs", so `git push proton-v2 --delete heal-refused/sub
   main:heal-refused/sub2` would try to delete both, and `:main:heal-refused/sub2` is not a valid
   refspec. A mixed delete-and-create batch must spell the delete as a leading-colon refspec, and
   both destinations are written fully qualified so git has nothing to disambiguate.
   Trace: phase 4 trashes `refs/heads/heal-refused/sub`, then `pruneEmptyParents` trashes the
   now-empty `refs/heads/heal-refused` folder (stopping at the protected `refs/heads` root); phase 5's
   `ensureRefParents` immediately `EnsureDir`s `refs/heads/heal-refused` **again, at the just-trashed
   name**, before writing `sub2`. Rule 2b permits the batch: valid deletes are subtracted from the
   final set before creates are added, so `sub2` collides with nothing.
   **Expect exit 0**, `ok refs/heads/heal-refused/sub2`, and a `refs/heads/heal-refused` folder
   present again containing only `sub2`.
   **If the push instead fails with any message beginning `"contradiction creating folder ..."`**
   (`internal/transport/cli.go`), that is a live C17 recurrence — the first ever. Record it
   **verbatim** (it quotes both raw observations by design) and report **BLOCKED; do not retry and
   do not patch**, exactly as for the signature-constant pins above. Note that a *resolved*
   contradiction is silent by design (`reobserveEnsureDirContradiction` returns nil on a successful
   re-observation), so a clean exit 0 cannot distinguish "contradiction, resolved" from "no
   contradiction" — that is expected. The value of this step is the failure it can surface, not a
   positive signal.
5. This composed sequence (delete → prune-observed-in-listing → create → manufactured-collision →
   heal/refuse → one-batch prune-and-recreate) is deliberately run **live**, against the real CLI,
   because the hermetic suite (`internal/repo/repo_test.go`,
   `TestCreateSelfHealsEmptyFolderCollision` and siblings) exercised this only against
   `transport.Fake`. This step is the live half Task 9a/9b's reports explicitly flagged as gate
   territory.

**State at the end of outline step 2**, for the next step's benefit: remote `refs/heads` holds
`feature` (a ref FILE, from item 2), `heal-empty`, `heal-refused/sub2`, and `main`; `refs/heads/feature/x`
is **gone**; `refs/tags/release/v1` and `refs/notes/commits` are untouched; HEAD still names
`refs/heads/main`.

## Outline step 3 — `--set-head` to a nested branch + delete-protection + namespace-folder refusal

Per spec component 8 outline item 3, plus the Task 10 fix discovered mid-stage.

**Repo/alias assumed by every `git push` in this step: the ORIGINAL gate repo of outline step 1
items 1–2**, whose `proton-v2` alias points at `/my-files/GitRemotes/<demo>` — item 3 below
included, which an earlier draft left implicit. The `git-remote-proton --set-head` invocations
(items 1 and 4) take the `proton::` URL **scheme** directly and need no local remote at all; item 2's
clone creates its own throwaway repo.

0. **Restore the ref state this step requires — two explicit pushes, before anything else.**
   Outline step 2 left a bare `refs/heads/feature` ref FILE on the remote (its item 2) and **no**
   `refs/heads/feature/x` (its item 1). Items 1 and 4 below need exactly the opposite:
   `refs/heads/feature/x` present, no bare `feature`. Leaving this implicit is a false BLOCK waiting
   to happen — a runner who simply re-pushes `feature/x` over the bare `feature` hits rule 2b's
   `"refs/heads/feature/x conflicts with refs/heads/feature: a ref cannot be both a leaf and a
   folder containing other refs"`, which is **correct** behaviour that would get written up as a
   defect. So, from the original gate repo:
   ```
   git push proton-v2 --delete feature
   git push proton-v2 feature/x
   ```
   The local branch `feature/x` still exists (only its remote copy was deleted), carrying outline
   step 1 item 6's incremental commit. Remote HEAD is still `refs/heads/main` here, so deleting the
   `feature` branch is not delete-protected. Afterwards, confirm `refs/heads/feature` is a **folder**
   containing `x`, not a file.
   Order matters locally as well as remotely, the mirror image of outline step 2 item 1's note: the
   `--delete` must land first so that `refs/remotes/proton-v2/feature` (a FILE, created by outline
   step 2 item 2's push) is gone before the second push wants to create
   `refs/remotes/proton-v2/feature/x` beneath that name. If it lingers, clear it with
   `git update-ref -d refs/remotes/proton-v2/feature` — again a purely LOCAL directory/file
   failure, not a helper defect.
   Two separate pushes deliberately, not the single-batch equivalent (`git push proton-v2
   :refs/heads/feature feature/x` — again a leading-colon refspec, since `--delete` would apply to
   both listed refs): that one-batch form would also be legal (rule 2b subtracts valid deletes
   before adding creates, and phase 4 runs before phase 5), but state restoration must not itself be
   the thing under test — outline step 2 item 4 already covers the one-batch shape on purpose.
1. With both `main` and `refs/heads/feature/x` on the remote (guaranteed by item 0), run
   `git-remote-proton --set-head proton::/my-files/GitRemotes/<demo> feature/x`. Expect
   `HEAD is now refs/heads/feature/x`, exit 0.
2. Verify via `proton-drive filesystem download .../HEAD` and via a **fresh** clone into a new short
   local path — `git clone -o proton-v2 proton::/my-files/GitRemotes/<demo> <short-local-path>-head`,
   then `git branch --show-current` must report `feature/x` — same decisive-postcondition pattern as
   `stage4-gate.md` run 2 §3.4–3.5. This is a throwaway repo; nothing later in this brief uses it.
3. **Delete-protection follows HEAD** (run from the original gate repo, `proton-v2` alias):
   `git push proton-v2 --delete feature/x` must be **refused**, naming `--set-head` as the remedy —
   exact text (`internal/repo/push.go`):
   `"refusing to delete the branch HEAD points at (refs/heads/feature/x); change the default branch
   first (git-remote-proton --set-head <url> <branch>)"`. Deleting the **old** default — which in
   this sequence is `main`, what HEAD named before item 1 — must instead **succeed**:
   `git push proton-v2 --delete main`. That is the decisive pair, same as `stage4-gate.md` run 2 §3.6.
   Note for the steps after this one: **`refs/heads/main` no longer exists on the remote** from here
   on. Outline step 4's push/fetch cycle must therefore drive `feature/x`, not `main` (it is spelled
   out there).
4. **New this stage — the namespace-folder refusal (Task 10's fix)**: `refs/heads/feature/x` exists
   and there is no bare `feature` branch (item 0 guaranteed it; nothing since re-created one), so run
   `git-remote-proton --set-head proton::/my-files/GitRemotes/<demo> feature`. Expect a **refusal
   naming the situation and suggesting the real branches**, not a misleading "not found" — exact
   wording (`internal/repo/sethead.go`):
   `"cannot set HEAD to \"feature\": refs/heads/feature is a namespace folder containing other
   branches, not a branch itself; branches that exist: feature/x, heal-empty, heal-refused/sub2"`.
   The suggestion list is **every branch on the remote at that moment, sorted**
   (`existingBranchNames`); in this brief's sequence that is exactly those three — `main` was deleted
   in item 3, `feature` in item 0. If a preceding step was skipped or varied, the list varies with
   it; the fixed part of the assertion is the sentence before the colon. This is the live half of
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
   **Drive it on `feature/x`, not `main`** — outline step 3 item 3 deleted `refs/heads/main` from
   the remote as the decisive half of the delete-protection pair, so it is no longer there to push
   to. From the original gate repo (which still has `feature/x` checked out):
   ```
   git commit --allow-empty -m "gate: quarantine no-regression commit"
   git push proton-v2 feature/x
   ```
   then, in the outline step 1 item 4 clone, `git fetch proton-v2`. Repeat the pair once more if a
   single cycle does not produce a second round. `feature/x` is HEAD on the remote, which protects
   it against *deletion* only — updating it is unaffected. A stale
   `refs/remotes/proton-v2/main` in either local repo is expected and harmless (`git fetch` does
   not prune by default); do not "clean it up" mid-gate.
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
   `<demo>-parents` and `<demo>-parents/nested` do not). This is a **different** remote path than
   outline steps 1–4's `<demo>`, so it needs its own local repo and its own `proton-v2` alias —
   reusing outline step 1's repo/remote here would push to the wrong URL. Set up explicitly:
   ```
   git init <demo>-parents-test
   cd <demo>-parents-test
   git commit --allow-empty -m "gate: parents-test initial commit"
   git remote add proton-v2 "proton::/my-files/GitRemotes/<demo>-parents/nested/repo"
   ```
   With `GPB_CREATE_PARENTS` unset, run `git push proton-v2 main`. Expect **exit non-zero**, and
   stderr containing the exact actionable
   grammar (`internal/repo/parents.go`):
   `"parent folder /my-files/GitRemotes/<demo>-parents does not exist; create it first (proton-drive
   filesystem create-folder /my-files/GitRemotes <demo>-parents, or the web UI), or set
   GPB_CREATE_PARENTS=1 to let the helper create missing parents"`. Immediately after, take a
   `/my-files/GitRemotes` listing and assert it is **row-set-identical** to the listing taken
   immediately before this push attempt (checklist item 1 — compare `uid`/`name`/`type`/
   `creationTime`/`modificationTime`/`parentUid` fields, never raw JSON byte equality) — nothing was
   created.
2. **Set — parents created with loud stderr, torn down in cleanup.** Same `<demo>-parents-test`
   repo and `proton-v2` alias as item 1. Set `GPB_CREATE_PARENTS=1` for this one shell/command only
   (never persist it), retry the same push (`git push proton-v2 main`). Expect **exit 0**, and
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
  tallying what this gate run sent to trash for the record, include every FOLDER this run trashed,
  not just the files pushed and later explicitly trashed in cleanup. In this brief's sequence that
  is three folder-trashing events before cleanup even begins — outline step 2 item 1's
  delete-and-prune of `refs/heads/feature`, outline step 2 item 3's self-heal of the
  hand-made `refs/heads/heal-empty`, and outline step 2 item 4's in-batch prune of
  `refs/heads/heal-refused` (which phase 5 then re-creates under the same name, so the live tree
  shows it again while the trash holds the pruned original). A files-only count understates what
  actually left the account's live tree this stage, since prune (unlike prior stages) now removes
  folders, not just leaves them empty.
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
