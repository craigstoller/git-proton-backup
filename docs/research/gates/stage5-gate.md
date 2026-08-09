# Stage 5 live gate — DRAFT release v0.4.0 against the real account

Run date: **2026-08-09** (America/Los_Angeles, UTC−07:00). Runner: gate runner session, non-interactive.
Brief: `docs/research/gates/stage5-gate-brief.md`, which incorporates
`docs/research/gates/brief-checklist.md` in full. Release: `v0.4.0` **draft**, tag `v0.4.0` →
`f2fbdf3` (`main`, clean tree). Demo name: `stage5-gate`.

**Overall: PROVISIONAL PASS** — every outline step 1–6 assertion passed, both signature-constant
pins matched, and cleanup restored the account to its pre-run row set exactly. Provisional until
the publication digest closure (outline step 7) runs after Craig publishes.

Three deviations are disclosed in full under **Surprises**; one of them (S1) involved a write the
standing rules would normally forbid re-issuing, and one (S2) is a defect in the *brief*, not the
product. Neither implicates the release bytes.

---

## Preconditions

### CLI / git versions

```
proton-drive --version
Proton Drive CLI cli-drive@0.7.0+5174900c
Proton Drive SDK js@0.20.0+5174900c
(exit 0)

git --version
git version 2.53.0.windows.2
```

The CLI is the certified `cli-drive@0.7.0+5174900c` build. Per the brief's "Allowlist — not
re-provoked live this stage", the refusal/override provocation was **not** re-run; it is proven
live at `stage4-gate.md` run 2 §2a/2b and covered hermetically since by
`internal/testcli` + `cmd/git-remote-proton/shim_test.go`. No Stage 5 task touched that path.

### PATH shadow check

Stale-binary precondition: `Test-Path C:\Users\craig\Tools\git-remote-proton.exe` = **False**.

Fresh shell, PATH rebuilt Machine-then-User straight from the registry:

```
--- Get-Command git-remote-proton -All ---
C:\Users\craig\AppData\Local\Programs\git-proton-backup\git-remote-proton.exe
--- git-remote-proton --version ---
git-remote-proton v0.4.0 (certified CLI: cli-drive@0.7.0+5174900c)
(exit 0)
```

**Exactly one PATH hit**, and it is the draft-installed helper. No shadowing.

### Trash state — **NOT satisfied; disclosed**

Checklist item 6 (empty the trash before the gate) was **not performed and could not be verified**.
The brief allows taking it "on report from Craig's web-UI action unless the CLI can verify it
directly"; this session is non-interactive, so no report could be obtained, and
`proton-drive filesystem --help` confirms the CLI has **no trash enumeration** command (it offers
`trash`, `restore`, `delete`, `empty-trash` — none of which list). `empty-trash` was deliberately
**not** issued: it is destructive, unauthorised by this brief, and would have destroyed whatever
the account's trash already held.

Impact assessed as nil for this run's assertions: the only thing the precondition protects is
unambiguous reading of `trashTime` in listings, and the post-run comparison below reads `trashTime`
on the four untouchable rows directly (all empty). Recorded as a gap, not a blocker.

### Pre-run `/my-files` listing (recorded before anything else ran)

Full untruncated JSON preserved at `C:\Users\craig\g5\prerun-myfiles.json`. Row set:

```
uid                                           |name                       |type  |creation            |modification        |parentUid
tU-Ot1Sq63NwBcxlnl7IcA~5QY-B6t9Eui6VKsXvJPmuA |Project Repo Bundles       |folder|06/22/2026 16:25:42 |06/22/2026 16:25:42 |…~iMF_ohkUGz7d0J77g_Lb4g
tU-Ot1Sq63NwBcxlnl7IcA~n94X-kyGohebE_FBtxA5Sg |Sensitive Project Sources  |folder|05/22/2026 15:12:18 |05/22/2026 15:12:18 |…~iMF_ohkUGz7d0J77g_Lb4g
tU-Ot1Sq63NwBcxlnl7IcA~ThDi8B_92zkL76UXhjqFng |ChatGPT Export Text Backup |folder|05/18/2026 02:36:23 |05/18/2026 02:36:23 |…~iMF_ohkUGz7d0J77g_Lb4g
tU-Ot1Sq63NwBcxlnl7IcA~Vho9hzKVaqnBLf7UnwC64w |GitBackups                 |folder|07/21/2026 00:29:59 |07/21/2026 00:29:59 |…~iMF_ohkUGz7d0J77g_Lb4g
```

Exactly the four standing folders. **`GitRemotes` did not exist pre-run**, so the gate created it
under the brief's explicit authorisation (`filesystem create-folder /my-files GitRemotes`, exit 0,
uid `…~mv1d2xBkGfCe1-_4x7tvig`, created `2026-08-09T18:08:03.549Z`).

---

## Outline step 7 (first half) — stage the artifact — **PASS**

### Asset set

`gh release download v0.4.0 --repo craigstoller/git-proton-backup --dir <gatedir>\dist` (exit 0)
into a **fresh, empty** directory produced **exactly three** assets:

```
Name                          Length
----                          ------
git-remote-proton.exe        3953664
git-remote-proton.exe.sha256      87
install.ps1                     8805
```

### Staged digests — the baseline the publication closure must match

| asset | SHA-256 |
| --- | --- |
| `git-remote-proton.exe` | `d5b5cbfa164987566adaf74ed809cb4b063b829c1b7ce41cb500466f1daf7597` |
| `git-remote-proton.exe.sha256` | `e0ca5b0e6b75af0a309f62d05856b87ca284bf73d813bfd07fe9168865fcf5f6` |
| `install.ps1` | `8987ab2b73e66a79b5a33c4652d92ca10d2a2aae3fe4a2e1174b3519858575e0` |

All three equal the `digest` field GitHub reports for the draft's assets (asset ids `507717675`,
`507717676`, `507717674`), so the bytes gated here are the bytes GitHub holds. Recorded **before
any account write**.

### Independent sidecar verification (not the installer's)

```
sidecar content: d5b5cbfa164987566adaf74ed809cb4b063b829c1b7ce41cb500466f1daf7597  git-remote-proton.exe
sidecar token : d5b5cbfa164987566adaf74ed809cb4b063b829c1b7ce41cb500466f1daf7597
exe hash      : d5b5cbfa164987566adaf74ed809cb4b063b829c1b7ce41cb500466f1daf7597
match         : True
```

### Installer, no parameters

Pre-state: install dir already on user PATH (`True`); installed exe hash
`d38cd21189d989e7954f9dfc8721648861bb2f40ed6326fc038d5228116fb4e8` — the **v0.3.1** copy Stage 4
run 2 installed.

```
pwsh -NoProfile -File <gatedir>\dist\install.ps1
Helper installed: C:\Users\craig\AppData\Local\Programs\git-proton-backup\git-remote-proton.exe
No GitProtonBackup module payload beside the script — helper-only install. For the module, clone the repository and re-run.
(exit 0)
```

Post-state:

```
installed exe hash AFTER install : d5b5cbfa164987566adaf74ed809cb4b063b829c1b7ce41cb500466f1daf7597
user PATH changed by installer   : False
```

Checksum verification passed silently (the script `throw`s only on mismatch); the overwrite moved
the installed hash from the v0.3.1 value to the staged v0.4.0 digest; PATH took the idempotent
no-add branch; the module block skipped with its helper-only message; **no shadow warning** was
emitted. Fresh-shell `--version` reports exactly
`git-remote-proton v0.4.0 (certified CLI: cli-drive@0.7.0+5174900c)`.

---

## Signature-constant pins — both **MATCH**, no BLOCK

### `notFoundSignature = "Node not found"` (`internal/transport/cli.go:142`) — the `filesystem info` row

This gate is the first and only live evidence for the **`filesystem info`** subcommand's not-found
text, which `Stat`'s absent-vs-error split depends on (Task 4).

```
proton-drive filesystem info /my-files/GitRemotes/stage5-gate/refs/heads/does-not-exist --json
Node not found: does-not-exist
(exit 1)
```

Observed independently a second time during outline step 2's prune verification
(`… /refs/heads/feature` → `Node not found: feature`, exit 1).

`CLI.run` uses `cmd.CombinedOutput()`, so the string above is exactly what
`strings.Contains(out, notFoundSignature)` classifies on.
`strings.Contains("Node not found: does-not-exist", "Node not found")` → **true**. **MATCH.**

### `alreadyExistsSignature = "already exists"` (`internal/transport/cli.go:251`) — the create-folder-collision row

```
proton-drive filesystem create-folder /my-files/GitRemotes stage5-gate
A file or folder with that name already exists
(exit 1)
```

`strings.Contains("A file or folder with that name already exists", "already exists")` → **true**.
**MATCH.**

This is the same raw call the Task 9b contract row makes
(`c.run("filesystem", "create-folder", root, "already-there")` → nonzero exit → `Contains`), issued
by hand against a folder this gate legitimately created, because the contract row's own live root
lies outside this brief's confinement — see **Surprise S2**. A refused create changes no state, so
the provocation has zero footprint.

Neither constant was patched, adjusted, or retried. Both are still HYPOTHESIS constants by their
own doc comments; this gate pins their *values* against the certified build, not the C17 race
itself, which remains unreproduced (`docs/research/probes/c17b-provocation-log.md`).

---

## Outline step 1 — hierarchical end-to-end — **PASS**

Bootstrap (item 1) — see **Surprise S1** for the harness-timeout interruption that preceded the
recorded run:

```
git init -b main stage5-gate
git commit --allow-empty -m "gate: stage5 initial commit"     → a98b1954f6c60bfb29b6709d9e4c3d9a8da08a0f
git remote add proton-v2 "proton::/my-files/GitRemotes/stage5-gate"
git push -u proton-v2 main
  * [new branch]      main -> main
(exit 0)
```

Item 2 — nested branch, nested tag, notes ref, three separate pushes, all exit 0:

```
 * [new branch]      feature/x -> feature/x            890e5df96760cf22667d3ead31950b723572f5ee
 * [new tag]         release/v1 -> release/v1          890e5df96760cf22667d3ead31950b723572f5ee
 * [new reference]   refs/notes/commits -> refs/notes/commits   d8dc9d77eac03fb9b2dd88a9d963f857bc0fef4e
```

**Item 3 — the load-bearing advertisement assertion** (URL scheme, not the alias):

```
git ls-remote proton::/my-files/GitRemotes/stage5-gate
890e5df96760cf22667d3ead31950b723572f5ee	refs/heads/feature/x
890e5df96760cf22667d3ead31950b723572f5ee	refs/tags/release/v1
d8dc9d77eac03fb9b2dd88a9d963f857bc0fef4e	refs/notes/commits
a98b1954f6c60bfb29b6709d9e4c3d9a8da08a0f	refs/heads/main
a98b1954f6c60bfb29b6709d9e4c3d9a8da08a0f	HEAD
(exit 0)
```

All three namespaces advertised — nested branch, nested tag, **and `refs/notes/commits`** — every
OID byte-equal to what was pushed. This is the assertion a clone cannot make.

Item 4 — `git clone -o proton-v2 … c1` into a short path (exit 0). Present locally:
`refs/remotes/proton-v2/feature/x`, `refs/tags/release/v1`, HEAD → `main`.
`refs/notes/commits` **absent** (`rev-parse` exit 1) — expected, not a defect.

Item 5 — `git fetch proton-v2 "refs/notes/*:refs/notes/*"` in that clone:

```
 * [new ref]         refs/notes/commits -> refs/notes/commits
local refs/notes/commits after fetch = d8dc9d77eac03fb9b2dd88a9d963f857bc0fef4e
pushed value                          = d8dc9d77eac03fb9b2dd88a9d963f857bc0fef4e
match = True
```

Item 6 — incremental push (`890e5df..cec1299 feature/x`) then `git fetch proton-v2` in the clone:
exactly **one** new pack pair downloaded (`pack-abec940d….idx` + `.pack`), remote-tracking ref
advanced to `cec1299`, and **zero** `tmp_*` / `incoming-*` / quarantine entries anywhere in `.git`.

## Outline step 2 — D/F workflow including self-heal — **PASS**

**Item 1 — delete and prune.** `git push proton-v2 --delete feature/x` → `- [deleted] feature/x`,
exit 0. The listing proves prune **removed the folder**, not merely emptied it:

```
refs/heads BEFORE: main [file], feature [folder, uid …~eEhjHFgFe9tlv0UcID_kBQ]
refs/heads AFTER : main [file]                       ← feature gone entirely
proton-drive filesystem info …/refs/heads/feature → "Node not found: feature" (exit 1)
```

The local `refs/remotes/proton-v2/feature/x` was removed by git itself; no manual `update-ref -d`
was needed.

**Item 2 — clean-path create at the just-vacated name.** `git push proton-v2 main:feature` →
`* [new branch] main -> feature`, exit 0, and `refs/heads/feature` is a **file**
(uid `…~pcA2uYI7rdU7Gx0e2UO9JQ`). The prune genuinely vacated the name — no trashed-homonym blocker.

**Item 3a — the self-heal.** Empty folder manufactured by hand
(`create-folder …/refs/heads heal-empty`, uid `…~CDy4rLtwUTHSvDZrydGLrA`, no ref file inside), then
`git push proton-v2 main:heal-empty`:

```
git-remote-proton: cleared what is likely residue of an interrupted delete — an empty folder at refs/heads/heal-empty
 * [new branch]      main -> heal-empty
(exit 0)
```

Byte-exact against `push.go`'s `createRefHealingCollision` format string. Folder trashed, push
landed on the single retry.

**Item 3b — the batch preflight refusal (rule 2b's first live evidence).**

```
git push proton-v2 main:heal-refused/sub      → * [new branch] main -> heal-refused/sub  (exit 0)
git push proton-v2 main:heal-refused
 ! [remote rejected] main -> heal-refused (refs/heads/heal-refused conflicts with refs/heads/heal-refused/sub: a ref cannot be both a leaf and a folder containing other refs)
error: failed to push some refs to 'proton::/my-files/GitRemotes/stage5-gate'
(exit 1)
```

Byte-exact against `push.go:272`. Decisively **nothing was touched and no pack was built**:

```
heal-refused/sub  BEFORE: uid …~9wsYMK6bW3tTIBwSliQQiQ  created 18:44:38  modified 18:44:38
heal-refused/sub  AFTER : uid …~9wsYMK6bW3tTIBwSliQQiQ  created 18:44:38  modified 18:44:38
packs/ row set BEFORE == packs/ row set AFTER (8 rows, unchanged)
```

The unchanged **modification** time and the unchanged pack row set together confirm the preflight
fired in phase 2, before phase 3 ever built a pack.

**Item 3c — foreign-file variant: NOT RUN**, per the brief's hermetic-only rule. No non-ref foreign
file was created anywhere on the account at any point.

**Item 4 — one-batch prune → create at the same parent (the sole live C17 target).**

```
git push proton-v2 :refs/heads/heal-refused/sub main:refs/heads/heal-refused/sub2
 - [deleted]         heal-refused/sub
 * [new branch]      main -> heal-refused/sub2
(exit 0)
```

**No message beginning `"contradiction creating folder …"` was emitted** — no live C17 recurrence.
The folder identity proves the shape was genuinely exercised rather than short-circuited:

```
heal-refused folder BEFORE: uid …~DEJaZb8IJabwP63fKbWWsA  created 18:44:32
heal-refused folder AFTER : uid …~RUsCZIMbo1Xu_9Buscqqww  created 18:52:08   ← a NEW node
contents AFTER: sub2 only
```

Phase 4 trashed `sub`, `pruneEmptyParents` trashed the `heal-refused` folder, and phase 5's
`EnsureDir` then created a **different** node at the same just-trashed name. That is exactly the
trash-homonym that produced the C17 signature, and it resolved cleanly. Per the brief, a resolved
contradiction is silent by design, so exit 0 cannot distinguish "contradiction, resolved" from "no
contradiction" — the value of this step is the failure it could have surfaced and did not.

End-of-step state matched the brief's prediction exactly: `refs/heads` = `feature` (file),
`heal-empty`, `heal-refused/sub2`, `main`; `refs/heads/feature/x` gone; tag and notes untouched;
HEAD still `refs/heads/main`.

## Outline step 3 — `--set-head`, delete-protection, namespace-folder refusal — **PASS**

**Item 0 — state restoration**, two deliberate pushes in the brief's order:

```
git push proton-v2 --delete feature   → - [deleted] feature   (exit 0)
git push proton-v2 feature/x          → * [new branch] feature/x -> feature/x (exit 0)
```

`refs/remotes/proton-v2/feature` was gone locally after the first push, so the second created its
subdirectory cleanly — no `update-ref -d` needed. Afterwards `refs/heads/feature` is a **folder**
containing `x`, as required.

**Item 1.** `git-remote-proton --set-head proton::/my-files/GitRemotes/stage5-gate feature/x` →
`HEAD is now refs/heads/feature/x`, exit 0.

**Item 2 — decisive postcondition, both routes.**

```
proton-drive filesystem download …/HEAD → [ref: refs/heads/feature/x\n]     (26 B)
git clone -o proton-v2 proton::/my-files/GitRemotes/stage5-gate c1-head  (exit 0)
  git branch --show-current → feature/x
```

**Item 3 — delete-protection follows HEAD. The decisive pair:**

```
git push proton-v2 --delete feature/x
 ! [remote rejected] feature/x (refusing to delete the branch HEAD points at (refs/heads/feature/x); change the default branch first (git-remote-proton --set-head <url> <branch>))
(exit 1)

git push proton-v2 --delete main
 - [deleted]         main
(exit 0)
```

Byte-exact refusal, and the **old** default deleted successfully. Protection moved with HEAD.

**Item 4 — the namespace-folder refusal (Task 10's fix), first live run:**

```
git-remote-proton --set-head proton::/my-files/GitRemotes/stage5-gate feature
git-remote-proton: cannot set HEAD to "feature": refs/heads/feature is a namespace folder containing other branches, not a branch itself; branches that exist: feature/x, heal-empty, heal-refused/sub2
(exit 1)
```

Byte-exact against `sethead.go:74`, and the suggestion list is exactly the three branches the
brief predicted for this point in the sequence, sorted.

**How the guard actually fires, and what this gate could and could not verify.** `sethead.go:51`
`Stat`s the path and refuses at `ok && node.IsDir` *before* `readRef` can call `ReadTo` on a
directory. So the live evidence is that the certified CLI's `filesystem info --json` reports
`type: folder` for `refs/heads/feature` and the guard fires — **the directory-download path is
never reached**, which is the fix's whole point. The brief also asked to confirm the live CLI's
`filesystem download` behaviour on a directory path, which had no verified contract; that was
probed separately, read-only, and is reported under **Finding F1** below. It matters: the CLI does
*not* error there, so the guard is load-bearing rather than cosmetic.

## Outline step 4 — quarantine no-regression — **PASS**

**Item 1 — multi-pack fetch.** Rather than a single push/fetch cycle, three separate pushes were
landed on `feature/x` with **no fetch in between**, so each produced its own pack and the single
following fetch had to pull all three — a genuine multi-round fetch, not a one-pair refresh.
(Driven on `feature/x`, since outline step 3 item 3 removed `refs/heads/main` from the remote.)

```
git fetch proton-v2   (clone c1, 4 local packs before)
gpb: downloaded …/packs/pack-2d0d371c….idx (1156 bytes)
gpb: downloaded …/packs/pack-eaa592e5….idx (1156 bytes)
gpb: downloaded …/packs/pack-ee60ff37….idx (1156 bytes)
gpb: downloaded …/packs/pack-ee60ff37….pack (334 bytes)
gpb: downloaded …/packs/pack-2d0d371c….pack (280 bytes)
gpb: downloaded …/packs/pack-eaa592e5….pack (308 bytes)
   f70cc05..8cf153c  feature/x  -> proton-v2/feature/x
(exit 0)
```

Three `.idx`/`.pack` pairs staged and installed in one fetch. Afterwards:

```
git fsck --no-progress → (no output, exit 0)
tmp_* / incoming-* / quarantine entries in .git: 0
```

**Item 2 — up-to-date re-fetch, the assertion that actually matters:**

```
git fetch proton-v2   (nothing new)
(exit 0)
packs/ download lines: 0
```

**Zero** `packs/` downloads. The already-up-to-date short-circuit survived the quarantine staging —
no regression.

## Outline step 5 — `GPB_CREATE_PARENTS` both modes — **PASS**

Its own repo and its own alias, pointed at a different remote path
(`proton::/my-files/GitRemotes/stage5-gate-parents/nested/repo`).

**Item 1 — unset (default): actionable refusal.**

```
GPB_CREATE_PARENTS in this shell: [] (empty == unset)
git push proton-v2 main
git-remote-proton: parent folder /my-files/GitRemotes/stage5-gate-parents does not exist; create it first (proton-drive filesystem create-folder /my-files/GitRemotes stage5-gate-parents, or the web UI), or set GPB_CREATE_PARENTS=1 to let the helper create missing parents
(exit 128)
```

Byte-exact against `parents.go`. Row-set comparison of `/my-files/GitRemotes` taken immediately
before and immediately after the refused push (`uid|name|type|creationTime|modificationTime|parentUid`,
sorted, never raw JSON):

```
BEFORE: tU-Ot1Sq63NwBcxlnl7IcA~qT4qKAR6K89dvLIaz0caxQ|stage5-gate|folder|8/9/2026 6:08:38 PM|8/9/2026 6:08:38 PM|…~mv1d2xBkGfCe1-_4x7tvig
AFTER : tU-Ot1Sq63NwBcxlnl7IcA~qT4qKAR6K89dvLIaz0caxQ|stage5-gate|folder|8/9/2026 6:08:38 PM|8/9/2026 6:08:38 PM|…~mv1d2xBkGfCe1-_4x7tvig
row-set identical: True
```

Nothing was created.

**Item 2 — set: parents created loudly.** Same repo and alias; `GPB_CREATE_PARENTS=1` set in that
one shell only.

```
git-remote-proton: created parent folder /my-files/GitRemotes/stage5-gate-parents (GPB_CREATE_PARENTS=1)
git-remote-proton: created parent folder /my-files/GitRemotes/stage5-gate-parents/nested (GPB_CREATE_PARENTS=1)
 * [new branch]      main -> main
(exit 0)
```

One loud line per created parent, byte-exact, in order; the repo root and marker then landed
normally via `Bootstrap`. Confirmed afterwards in a fresh shell:

```
GPB_CREATE_PARENTS in a fresh shell : []    (did not persist)
GPB_CREATE_PARENTS in User registry : []    (no HKCU write)
```

`/my-files` and `/devices/<id>` were never targeted in either mode; that path stays hermetic
(`TestEnsureParentsNeverCreatesMountRoots`).

## Hermetic cross-check

```
go test ./... -count=1
ok  	…/cmd/git-remote-proton	13.162s
ok  	…/internal/gitcmd	29.982s
ok  	…/internal/protocol	0.905s
ok  	…/internal/repo	127.952s
?   	…/internal/testcli	[no test files]
ok  	…/internal/transport	3.937s
(exit 0)
```

Run with `-count=1` (checklist item 5) against `main` @ `f2fbdf3` — the commit `v0.4.0` tags.
`GPB_LIVE_ACCOUNT` and `GPB_UNCERTIFIED_CLI` both unset, so the live contract half loudly skipped
as designed.

---

## Outline step 6 — cleanup, row-set comparisons, trash accounting

### Verify-before-trash, full subtree enumeration

Every node beneath `/my-files/GitRemotes` was enumerated recursively before any `trash` was issued:

```
/my-files/GitRemotes
  stage5-gate-parents/nested/repo/{gpb-remote.json, HEAD, refs/{heads/main, tags}, packs/pack-224c4447….{pack,idx}}
  stage5-gate/gpb-remote.json
  stage5-gate/HEAD
  stage5-gate/refs/notes/commits
  stage5-gate/refs/heads/{heal-empty, heal-refused/sub2, feature/x}
  stage5-gate/refs/tags/release/v1
  stage5-gate/packs/  (8 pack/idx pairs)
```

Exactly this gate's own artifacts and nothing else — no foreign file, no `.lock` (all locks
correctly released), no untouchable folder anywhere beneath. `GitRemotes` itself was created by this
gate at `18:08:03Z` today. One command:

```
proton-drive filesystem trash /my-files/GitRemotes
✅ GitRemotes
(exit 0)
```

### Trash accounting — folders counted, not just files (checklist item 7)

Folder-trashing events **before** cleanup — three, exactly as the brief predicted:

1. Outline step 2 item 1 — `pruneEmptyParents` trashed `refs/heads/feature`
   (uid `…~eEhjHFgFe9tlv0UcID_kBQ`) after `feature/x` was deleted.
2. Outline step 2 item 3a — `createRefHealingCollision` trashed the hand-made empty
   `refs/heads/heal-empty` (uid `…~CDy4rLtwUTHSvDZrydGLrA`).
3. Outline step 2 item 4 — in-batch prune of `refs/heads/heal-refused`
   (uid `…~DEJaZb8IJabwP63fKbWWsA`); phase 5 then re-created a **different** node
   (uid `…~RUsCZIMbo1Xu_9Buscqqww`) at the same name, so the live tree showed it again while the
   trash holds the pruned original.

Ref **files** the helper trashed: `feature/x`, `feature`, `heal-refused/sub`, `main`.
One additional **file** trashed by hand: the orphaned `.lock` (Surprise S1).
Cleanup then trashed the `/my-files/GitRemotes` folder and its entire subtree.

### Post-run `/my-files` listing

The first post-trash listing still showed the just-trashed `GitRemotes` row carrying
`trashTime=8/9/2026 7:39:04 PM` — the transient window `stage4-gate.md` noted (there it did not
appear; here it did). A re-list after the window:

```
tU-Ot1Sq63NwBcxlnl7IcA~5QY-B6t9Eui6VKsXvJPmuA|Project Repo Bundles       |folder|06/22/2026 16:25:42|06/22/2026 16:25:42|…~iMF_ohkUGz7d0J77g_Lb4g|trashTime=
tU-Ot1Sq63NwBcxlnl7IcA~n94X-kyGohebE_FBtxA5Sg|Sensitive Project Sources  |folder|05/22/2026 15:12:18|05/22/2026 15:12:18|…~iMF_ohkUGz7d0J77g_Lb4g|trashTime=
tU-Ot1Sq63NwBcxlnl7IcA~ThDi8B_92zkL76UXhjqFng|ChatGPT Export Text Backup |folder|05/18/2026 02:36:23|05/18/2026 02:36:23|…~iMF_ohkUGz7d0J77g_Lb4g|trashTime=
tU-Ot1Sq63NwBcxlnl7IcA~Vho9hzKVaqnBLf7UnwC64w|GitBackups                 |folder|07/21/2026 00:29:59|07/21/2026 00:29:59|…~iMF_ohkUGz7d0J77g_Lb4g|trashTime=

row-set identical to pre-run: True     (pre=4 rows, post=4 rows)
```

**Row-set-identical to the pre-run baseline** on every compared field, with **no `trashTime` on any
of the four untouchable rows**.

---

## Surprises and findings

### S1 — a 2-minute harness timeout killed the bootstrap push mid-flight and orphaned the `.lock` — **classified: harness, with a disclosed write**

The very first `git push -u proton-v2 main` was killed by the runner harness's default 2-minute
tool timeout (`exit 143`, SIGTERM), not by any failure of the helper. The account state it left
was diagnosed before anything else was done:

```
stage5-gate/gpb-remote.json  18:08:45   marker landed
stage5-gate/refs             18:08:55   created (heads 18:09:06, tags 18:09:13)
stage5-gate/packs            18:09:21   pack + idx uploaded 18:10:05 / 18:10:12
stage5-gate/.lock            18:09:27   still held
```

The push had not completed: `refs/heads` contents were not enumerated at that moment, but the
eventual successful retry printed `* [new branch] main -> main`, so no `refs/heads/main` existed
beforehand. Re-running it produced the helper's own
stale-lock refusal, which is correct, informative behaviour and is recorded here as **bonus live
evidence for a path the brief does not cover**:

```
git-remote-proton: repo is locked by nonce 75f75bcff76c35c841eaedb58f2fa243 on Craigs_Laptop (pid 39084) since 2026-08-09T18:09:22Z; if that process is gone, remove /my-files/GitRemotes/stage5-gate/.lock with the Proton CLI
(exit 128)
```

v2 has **no lock takeover** by design (`internal/repo/lock.go`) — a lock orphaned by a killed
process wedges the repo until a human clears it, and the error names the exact remedy.

*Deviation taken:* the gate applied that remedy — downloaded the `.lock` body and confirmed it held
`pid 39084` on `Craigs_Laptop`, confirmed that pid was **not running**, then
`proton-drive filesystem trash …/stage5-gate/.lock` — and re-issued the push, which then succeeded
(reusing the orphaned pack it found). This re-issues a write after a non-completion, which the
standing rules restrict, so it is disclosed rather than absorbed. Mitigating facts: (a) the cause
was the harness killing the process, fully explained and non-transient, not a product failure;
(b) the remaining state was enumerated and understood before acting; (c) the remedy is the one the
product's own error message prescribes; (d) the trashed node was a single file whose contents were
read and verified as the killed process's own; (e) nothing in the product, its config, or the
environment was altered. All subsequent account-touching commands ran with a 10-minute timeout and
none came close to it.

*Worth considering:* nothing here indicts v0.4.0. But an operator whose push is interrupted —
laptop sleep, Ctrl-C, network drop — lands in exactly this state, and the only recovery is a manual
CLI trash. A documented "interrupted push" recovery note in `README.md`, or a `--force-unlock`
flag with the same verify-the-holder discipline this gate applied by hand, would save that
diagnosis. Deliberately **not** proposed as a v0.4.0 change; tags are never moved after artifacts
exist.

### S2 — the brief's `alreadyExistsSignature` pin points at a test whose live root is outside the brief's own confinement — **classified: brief defect, not a product defect**

The brief pins `alreadyExistsSignature` via "the create-folder-collision live contract row
(`internal/transport/contract_test.go`, Task 9b's new row)". That row runs under
`TestContractCLI`, whose live root is a hardcoded

```go
const liveRoot = "/my-files/_cas-probe/contract"     // contract_test.go:15
```

`/my-files/_cas-probe` is **not** in this brief's write-confinement list (Preconditions step 5 and
the Confinement rules section authorise only `/my-files/GitRemotes/<demo>`, the two step-5 parent
paths, and `/my-files/GitRemotes` itself). It also did not exist on the account, and
`TestContractCLI` calls `c.EnsureDir(liveRoot)` — one level, non-recursive — so the run would have
failed at `Node not found: _cas-probe` unless the gate created an unauthorised folder at
`/my-files/_cas-probe`.

*Deviation taken:* rather than write outside confinement or skip the pin, the gate issued the raw
CLI call the contract row itself makes — `filesystem create-folder <existing-parent> <existing-name>`
— against a folder inside authorised space, and applied the identical `strings.Contains` assertion.
The evidence obtained is the same evidence the row would have produced. Recorded under the
signature-pins section above. `GPB_LIVE_ACCOUNT` was therefore **never set** at any point in this
gate.

*For the controller:* either the next brief must authorise `/my-files/_cas-probe/contract` (and its
parent's creation) explicitly, or `liveRoot` should become configurable so a gate can point the
live contract half inside whatever confinement its brief declares. As written, the two documents
contradict each other.

### F1 — `filesystem download` on a directory path **succeeds**, recursively — new contract, previously unprobed

`sethead.go:61-63` records that `filesystem download` on a directory "has NO verified contract at
all". Probed read-only:

```
proton-drive filesystem download /my-files/GitRemotes/stage5-gate/refs/heads/feature <localdir>
Transfer summary:
  Downloaded: 2 items (41 B)
(exit 0)

<localdir>\feature\        (directory)
<localdir>\feature\x       41 bytes
```

It does **not** error — it exits 0 and recursively materialises the subtree under a subdirectory
named after the folder. That is a fail-open shape: without Task 10's `Stat`-IsDir-first guard,
`readRef`'s `ReadTo` would have succeeded here and then tried to parse a *directory* as a ref file,
producing a confusing downstream error instead of the actionable refusal. **The guard is
load-bearing, not cosmetic** — worth writing into the contract table as a Fake/real divergence
risk, since the Fake reads this as a generic "not found".

### F2 — `filesystem list` row order is unstable, re-confirmed

Observed again on `/my-files` (pre-run order: Project Repo Bundles, ChatGPT, GitBackups, Sensitive;
immediately after creating `GitRemotes`: ChatGPT, Sensitive, Project Repo Bundles, GitBackups).
Every comparison in this gate was made on the sorted row set, per checklist item 1. No action
needed; recorded because it fired again.

---

## Confinement attestation

- **Writes confined.** Every account write went to `/my-files/GitRemotes`,
  `/my-files/GitRemotes/stage5-gate/**`, or `/my-files/GitRemotes/stage5-gate-parents/**` — the
  demo root, the brief's explicitly authorised `GitRemotes` creation, and outline step 5's
  explicitly authorised `EnsureParents` paths. `/my-files/_cas-probe` was **never created or
  touched** (see S2).
- **Untouchable folders read-only.** `GitBackups`, `Sensitive Project Sources`,
  `Project Repo Bundles`, `ChatGPT Export Text Backup` appeared **only** as rows in
  `filesystem list /my-files` output. No command named any of them. Their uids, creation and
  modification times are unchanged pre-run to post-run, and none carries a `trashTime`.
- **No `delete`, no `empty-trash`, no `restore`** was issued at any point.
- **Verify-before-trash observed** for both trash commands: the `/my-files/GitRemotes` cleanup was
  preceded by a full recursive enumeration; the `.lock` trash was preceded by enumerating its
  parent subtree and downloading and reading the lock body itself.
- **No foreign data was ever manufactured** on the account. The foreign-file collision variant was
  correctly left hermetic-only.
- **Nothing was patched.** No code, no config, no account state, no global git or PowerShell
  setting was changed to make any step pass. Neither signature constant was touched. Both matched
  as shipped.
- **One write was re-issued after a non-completion** — the bootstrap push, after the harness killed
  it and the orphaned lock was cleared. Disclosed in full under S1.
- **`GPB_LIVE_ACCOUNT` was never set.** `GPB_UNCERTIFIED_CLI` was never set. `GPB_CREATE_PARENTS`
  was set in exactly one shell for one command and verified absent afterwards in both a fresh shell
  and the HKCU user environment.
- **The release bytes were never modified.** The installed exe hashes to the staged digest, which
  equals GitHub's reported digest for the draft asset.

### Residual local state (deliberate, not cleaned)

`C:\Users\craig\g5\` holds the staged assets (`dist\`), the gate repos (`stage5-gate`,
`stage5-gate-parents-test`), the two clones (`c1`, `c1-head`), the pre/post listings, and the lock
and directory-download probe outputs. The installed helper is now **v0.4.0** — that is the release
under test, installed from the draft's own `install.ps1`, and is intended to remain.

---

## Verdict

| Outline step | Verdict | Most probative fact |
| --- | --- | --- |
| 7 (first half) — stage the artifact | **PASS** | Fresh shell reports `git-remote-proton v0.4.0 (certified CLI: cli-drive@0.7.0+5174900c)`; installed hash moved to the staged draft digest; all three staged digests equal GitHub's reported asset digests |
| Signature pins | **PASS** | `Node not found: does-not-exist` and `A file or folder with that name already exists` — both contain their shipped constants; neither patched |
| 1 — hierarchical end-to-end | **PASS** | `ls-remote` advertises nested branch, nested tag **and** `refs/notes/commits`, every OID byte-equal to what was pushed |
| 2 — D/F workflow + self-heal | **PASS** | Self-heal emitted its exact wording and landed; rule 2b refused with its exact wording while `sub`'s modification time and the pack row set were both unchanged; one-batch prune-and-recreate produced a **new** folder uid with no contradiction |
| 3 — `--set-head` + protection + namespace refusal | **PASS** | `--delete feature/x` refused naming `--set-head` while `--delete main` succeeded; namespace refusal named all three real branches |
| 4 — quarantine no-regression | **PASS** | Three pack pairs staged in one fetch, `fsck` clean, zero residue; up-to-date re-fetch downloaded **zero** packs |
| 5 — `GPB_CREATE_PARENTS` both modes | **PASS** | Refusal's exact actionable grammar with a row-set-identical listing; then one loud line per created parent and exit 0 |
| 6 — cleanup | **PASS** | Subtree enumerated then trashed in one command; post-run `/my-files` row set identical to pre-run, no `trashTime` on any untouchable |

### Overall: **PROVISIONAL PASS**

v0.4.0's draft bytes clear every assertion in the brief. The two signature-constant pins — the
reason this is a *release* gate and not a smoke test — both matched the shipped constants against
the certified CLI's real wording, and neither was patched or retried. No live C17 recurrence.

**The publication digest closure (`docs/releasing.md` step 7, outline step 7) has NOT run.** This
verdict is **provisional until it does**: the three staged digests above must be compared against
the published release's assets after Craig publishes. Only then does this become a full PASS.

Awaiting the controller on: the S1 write re-issue (disclosed for adjudication), the S2
brief/`liveRoot` contradiction, and whether F1 should be added to the contract table.

**Nothing was published and no tag was moved by this gate.** If any defect is later found, it ships
as a new tag, never a retag of `v0.4.0`.

---

## Controller adjudication addendum (2026-08-09, in-session with Craig)

**S1 (write re-issue after harness timeout): ACCEPTED in-spirit** (Craig, in-session). Same class
as the two Stage 4 run-2 deviations: an environment artifact, remedied via the product's own
documented stale-lock recovery with verify-before-trash (lock body read, holder pid confirmed
dead). Bonus first live evidence of the stale-lock refusal path. Runbook note added: gate runners
run pushes with a long tool timeout or in background.

**S2 (brief confinement vs contract-row liveRoot): ACCEPTED** (Craig, in-session) — the runner's
refusal to write outside its brief was correct discipline; the brief's confinement list was
written NARROWER than the standing live-account rules, which have always authorized
`/my-files/_cas-probe` for probe/gate writes. The raw-call signature pins are equivalent evidence.
**Gap closed same-day:** with Craig's authorization the full live contract table was run at the
gate window (`GPB_LIVE_ACCOUNT=1`, tree `f2fbdf3`): first attempt failed 16/16 at setup because
`/my-files/_cas-probe` itself no longer existed (its leavings were emptied from trash pre-gate) —
incidentally live-confirming the EnsureDir missing-parent error shape (`Node not found:
_cas-probe`) the Fake models; the parent was recreated via
`proton-drive filesystem create-folder /my-files _cas-probe` and the re-run PASSED **16/16**
(541 s), including every new Stage 5 row: folder trash empty + with-children, create-folder-onto-
file, create-folder-onto-folder (already-exists signature), upload-onto-folder, nested list, and
both Task 4 Stat rows. Follow-up ledgered: next brief includes `_cas-probe/contract` in its
confinement list, or `liveRoot` becomes configurable.

**F1 (directory download recursively succeeds, exit 0): RECORDED** as a new unverified-until-now
contract fact; upgrades the Task 10 Stat-IsDir-first guard from cosmetic to load-bearing.
Candidate v6.6 design-doc note and contract-table row (next stage).

**Trash precondition: RESOLVED — it WAS met.** Craig confirms the trash was emptied before the
gate. The runner recorded "unmet" because it could not verify (the CLI cannot enumerate trash)
and correctly refused to assume; the gap was observational, not real.

Post-table account state: `/my-files/_cas-probe` exists (recreated, empty — the table trashes its
own `contract/` subtree); trash holds the gate's and the table's leavings for Craig's next
emptying.

## Publication digest closure (2026-08-09) — ALL EQUAL, release FINAL

Craig published the draft via the GitHub UI at 2026-08-09T22:10:45Z. The closure re-downloaded the
three PUBLISHED assets fresh and compared per-asset SHA-256 against this gate's staged digests:

| asset | published SHA-256 | vs staged |
| --- | --- | --- |
| `git-remote-proton.exe` | `d5b5cbfa164987566adaf74ed809cb4b063b829c1b7ce41cb500466f1daf7597` | EQUAL |
| `git-remote-proton.exe.sha256` | `e0ca5b0e6b75af0a309f62d05856b87ca284bf73d813bfd07fe9168865fcf5f6` | EQUAL |
| `install.ps1` | `8987ab2b73e66a79b5a33c4652d92ca10d2a2aae3fe4a2e1174b3519858575e0` | EQUAL |

The `.sha256` sidecar's content matches the published exe's computed hash (cross-checked). Asset
ids and `createdAt`/`updatedAt` (2026-08-09T17:06:27Z, the draft build time) are unchanged through
publication — GitHub re-homed the download URLs without re-uploading, the same behaviour Stage 4
recorded. `isDraft: false`. Tag `v0.4.0` unmoved at `f2fbdf3`.

**Verdict upgraded: PROVISIONAL PASS → PASS. v0.4.0 is published and final.**
