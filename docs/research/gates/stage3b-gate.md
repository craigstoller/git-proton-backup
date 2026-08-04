# Stage 3b live gate — measured selectivity against the real account

**Verdict: BLOCKED at Step 2.** The gate did not reach its selectivity measurement. Push 2 of 3
failed with a non-zero exit; per the gate's own rules the run stopped at the first surprise, was
not retried and nothing was patched.

- **Date:** 2026-08-04 (UTC timestamps throughout, as reported by the CLI)
- **Branch / HEAD:** `feat/v2-stage3b` @ `dfad74b`
- **Helper:** built from that tree to `%TEMP%\gpb-gate\bin\git-remote-proton.exe`
- **CLI:** `Proton Drive CLI cli-drive@0.7.0+5174900c` / `Proton Drive SDK js@0.20.0+5174900c` (the certified version)
- **git:** `git version 2.53.0.windows.2`
- **Demo name:** `stage3b-gate`
- **Runner:** live gate runner, executing `task-8-brief.md` steps 1–7 in order

Write confinement held throughout: the only remote paths created, modified or trashed were under
`/my-files/GitRemotes` and `/my-files/_cas-probe`. `GitBackups`, `Sensitive Project Sources`,
`Project Repo Bundles` and `ChatGPT Export Text Backup` were never named in any command other
than the read-only `/my-files` listings.

---

## Step 1 — Pre-run listing, parent provisioning, contract table's live half

### 1.1 Pre-run `/my-files` listing (the cleanup contract)

Recorded before anything else ran.

```
proton-drive filesystem list /my-files --json
```

```json
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~5QY-B6t9Eui6VKsXvJPmuA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g","name":{"ok":true,"value":"Project Repo Bundles"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"creationTime":"2026-06-22T16:25:42.000Z","modificationTime":"2026-06-22T16:25:42.000Z","treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~ThDi8B_92zkL76UXhjqFng","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g","name":{"ok":true,"value":"ChatGPT Export Text Backup"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"creationTime":"2026-05-18T02:36:23.000Z","modificationTime":"2026-05-18T02:36:23.000Z","treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Vho9hzKVaqnBLf7UnwC64w","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g","name":{"ok":true,"value":"GitBackups"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"creationTime":"2026-07-21T00:29:59.000Z","modificationTime":"2026-07-21T00:29:59.000Z","treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~n94X-kyGohebE_FBtxA5Sg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g","name":{"ok":true,"value":"Sensitive Project Sources"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"creationTime":"2026-05-22T15:12:18.000Z","modificationTime":"2026-05-22T15:12:18.000Z","treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```

Exactly the four standing folders. **Neither `_cas-probe` nor `GitRemotes` existed pre-run** —
the round-3 revision's provisioning branch was the live case.

### 1.2 / 1.3 Parent probes and provisioning

```
proton-drive filesystem list /my-files/_cas-probe --json
Node not found: _cas-probe
EXIT=1

proton-drive filesystem list /my-files/GitRemotes --json
Node not found: GitRemotes
EXIT=1
```

Both ABSENT. Recorded per the brief:

- **gate created `_cas-probe`** (`proton-drive filesystem create-folder /my-files _cas-probe`, exit 0, uid `…~YalEhSrh8oFX0lUofsYmew`, creationTime `2026-08-04T05:03:49.812Z`)
- **gate created `GitRemotes`** (`proton-drive filesystem create-folder /my-files GitRemotes`, exit 0, uid `…~CdcWtrrSjXX_3vECaeciuQ`, creationTime `2026-08-04T05:03:53.507Z`)
- **`contract` was ABSENT pre-run** (its parent did not exist), so the `contract` folder the table
  creates is gate-owned and is cleaned up in Step 7.

Both parents are therefore gate-owned in full: everything found under them at cleanup time was
created by this run.

### 1.4 Contract table, live half — **PASS**

```
$env:GPB_LIVE_ACCOUNT = "1"; go test ./internal/transport/ -run 'TestContract' -v
```

Both tests matched the pattern and ran; `TestContractCLI` ran **live and loud** (269.12s — not the
sub-second signature of a skip, and no `GPB_LIVE_ACCOUNT` skip message appeared).

```
=== RUN   TestContractFake
=== RUN   TestContractFake/stat_absence_is_not_an_error
=== RUN   TestContractFake/create_refuses_a_local_basename_that_mismatches_the_target_leaf_(C11)
=== RUN   TestContractFake/create_lands_at_the_target_leaf_when_basenames_agree
=== RUN   TestContractFake/create_refuses_a_name_already_taken
=== RUN   TestContractFake/readTo_lands_under_the_remote_basename_in_an_existing_dir
=== RUN   TestContractFake/readTo_into_a_missing_directory_errors_and_creates_nothing
=== RUN   TestContractFake/trash_on_a_missing_target_is_committed
=== RUN   TestContractFake/ensureDir_is_idempotent_and_its_result_is_listable
=== RUN   TestContractFake/list_of_an_empty_directory_is_empty,_not_an_error
--- PASS: TestContractFake (0.07s)
    --- PASS: TestContractFake/stat_absence_is_not_an_error (0.00s)
    --- PASS: TestContractFake/create_refuses_a_local_basename_that_mismatches_the_target_leaf_(C11) (0.00s)
    --- PASS: TestContractFake/create_lands_at_the_target_leaf_when_basenames_agree (0.01s)
    --- PASS: TestContractFake/create_refuses_a_name_already_taken (0.01s)
    --- PASS: TestContractFake/readTo_lands_under_the_remote_basename_in_an_existing_dir (0.03s)
    --- PASS: TestContractFake/readTo_into_a_missing_directory_errors_and_creates_nothing (0.01s)
    --- PASS: TestContractFake/trash_on_a_missing_target_is_committed (0.00s)
    --- PASS: TestContractFake/ensureDir_is_idempotent_and_its_result_is_listable (0.00s)
    --- PASS: TestContractFake/list_of_an_empty_directory_is_empty,_not_an_error (0.00s)
=== RUN   TestContractCLI
=== RUN   TestContractCLI/stat_absence_is_not_an_error
=== RUN   TestContractCLI/create_refuses_a_local_basename_that_mismatches_the_target_leaf_(C11)
=== RUN   TestContractCLI/create_lands_at_the_target_leaf_when_basenames_agree
=== RUN   TestContractCLI/create_refuses_a_name_already_taken
=== RUN   TestContractCLI/readTo_lands_under_the_remote_basename_in_an_existing_dir
=== RUN   TestContractCLI/readTo_into_a_missing_directory_errors_and_creates_nothing
=== RUN   TestContractCLI/trash_on_a_missing_target_is_committed
=== RUN   TestContractCLI/ensureDir_is_idempotent_and_its_result_is_listable
=== RUN   TestContractCLI/list_of_an_empty_directory_is_empty,_not_an_error
--- PASS: TestContractCLI (269.12s)
    --- PASS: TestContractCLI/stat_absence_is_not_an_error (30.36s)
    --- PASS: TestContractCLI/create_refuses_a_local_basename_that_mismatches_the_target_leaf_(C11) (25.72s)
    --- PASS: TestContractCLI/create_lands_at_the_target_leaf_when_basenames_agree (27.03s)
    --- PASS: TestContractCLI/create_refuses_a_name_already_taken (31.09s)
    --- PASS: TestContractCLI/readTo_lands_under_the_remote_basename_in_an_existing_dir (36.41s)
    --- PASS: TestContractCLI/readTo_into_a_missing_directory_errors_and_creates_nothing (32.22s)
    --- PASS: TestContractCLI/trash_on_a_missing_target_is_committed (18.69s)
    --- PASS: TestContractCLI/ensureDir_is_idempotent_and_its_result_is_listable (33.64s)
    --- PASS: TestContractCLI/list_of_an_empty_directory_is_empty,_not_an_error (33.96s)
PASS
ok  	github.com/craigstoller/git-proton-backup/internal/transport	270.110s
EXIT=0
```

**Assertion — both tests ran, neither skipped, both PASS: PASS.**
Note for Step 2's triage: the live scenario `ensureDir is idempotent and its result is listable`
**passed** against the real CLI (33.64s).

---

## Step 2 — Build, install on PATH, create the demo repo — **FAIL (gate stops here)**

### 2.1 Build and PATH

```
go build -o "$env:TEMP\gpb-gate\bin\git-remote-proton.exe" ./cmd/git-remote-proton
BUILD_EXIT=0
RESOLVED=C:\Users\craig\AppData\Local\Temp\gpb-gate\bin\git-remote-proton.exe
GIT=git version 2.53.0.windows.2
```

**Assertion — the freshly built helper shadows any older copy on PATH: PASS.**

### 2.2 Source repo and push 1 — PASS

```
git init -b main            (in %TEMP%\gpb-gate\src)
git config user.name  "Stage3b Gate"
git config user.email "gate@example.invalid"
<write file1.txt>; git add -A; git commit -m "c1"
git remote add proton-v2 "proton::/my-files/GitRemotes/stage3b-gate"
git push proton-v2 main
```

```
gpb: downloaded /my-files/GitRemotes/stage3b-gate/.lock (115 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate/HEAD (21 bytes)
To proton::/my-files/GitRemotes/stage3b-gate
 * [new branch]      main -> main
gpb: downloaded /my-files/GitRemotes/stage3b-gate/.lock (115 bytes)
PUSH1_EXIT=0
```

### 2.3 Push 2 — **FAIL, exit 128. This is the surprise the gate stopped on.**

```
<write file2.txt>; git add -A; git commit -m "c2"
git push proton-v2 main
```

Verbatim:

```
gpb: downloaded /my-files/GitRemotes/stage3b-gate/gpb-remote.json (42 bytes)
git-remote-proton: ensure dir /my-files/GitRemotes/stage3b-gate/refs/tags: create-folder tags in /my-files/GitRemotes/stage3b-gate/refs failed: A file or folder with that name already exists: exit status 1
PUSH2_EXIT=128
```

Nothing was patched and the push was not retried.

### 2.4 Push 3 — ran, exit 0 (disclosure)

**Disclosure of a runner-procedure deviation:** commits 2 and 3 and their two pushes were issued
in a single batched shell invocation, so push 3 executed automatically before push 2's non-zero
exit could be observed. It was not a deliberate retry past the surprise, and no further gate step
was run once the failure was seen. Its output is recorded for completeness:

```
gpb: downloaded /my-files/GitRemotes/stage3b-gate/gpb-remote.json (42 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate/.lock (115 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate/HEAD (21 bytes)
To proton::/my-files/GitRemotes/stage3b-gate
   db2119e..a0ce16e  main -> main
PUSH3_EXIT=0
gpb: downloaded /my-files/GitRemotes/stage3b-gate/.lock (115 bytes)
```

Local history at the stop point:

```
a0ce16e c3
7e20481 c2
db2119e c1
```

**Assertion — "the remote now holds 3 packs from 3 pushes": FAIL.** Two of three pushes
succeeded; the remote holds 2 packs. Steps 3–6 have no valid precondition and were not attempted.

---

## Remote state at the stop point (read-only capture, before cleanup)

`/my-files/GitRemotes/stage3b-gate` — `gpb-remote.json`, `refs`, `packs`, `HEAD`.

`/my-files/GitRemotes/stage3b-gate/packs` — **2 `.pack` + 2 `.idx`, not the required 3 + 3:**

| name | created |
| --- | --- |
| `pack-5422bec45aa3ba020e5acc09598c0421d5f8d300.pack` (221 B claimed) | 05:10:59Z |
| `pack-5422bec45aa3ba020e5acc09598c0421d5f8d300.idx` (1156 B claimed) | 05:11:08Z |
| `pack-65b1b8e85756233d1052510d1cba29cb40736fb9.pack` (508 B claimed) | 05:13:36Z |
| `pack-65b1b8e85756233d1052510d1cba29cb40736fb9.idx` (1240 B claimed) | 05:13:45Z |

Both stems match `pack-[0-9a-f]{40}`. The first pair is push 1's; the second is push 3's
(carrying c2 and c3 together). Push 2 contributed no pack.

`/my-files/GitRemotes/stage3b-gate/refs` — two folders:

| name | creationTime |
| --- | --- |
| `heads` | `2026-08-04T05:10:17.000Z` |
| `tags`  | `2026-08-04T05:10:24.000Z` |

`/my-files/GitRemotes/stage3b-gate/refs/heads` — `main`, activeRevision at `05:13:56Z` (41 B claimed).
`/my-files/GitRemotes/stage3b-gate/refs/tags` — `[ ]` (empty).
`/my-files/_cas-probe` — `contract` only. `/my-files/_cas-probe/contract` — `[ ]` (empty; the
table's per-test roots were cleaned by the tests themselves).

**Factual observation for triage, offered without diagnosis or remedy:** `refs/tags` was created
at `05:10:24Z` during push 1, and push 2 (~`05:12`) failed while creating that same
`refs/tags` — `create-folder` reported it already exists. Per the design, `EnsureDir` is
`Stat`-then-create precisely because `create-folder` is not idempotent, and the live contract
scenario `ensureDir is idempotent and its result is listable` passed against the real CLI minutes
earlier in this same run. Reconciling those two facts is the controller's call, not the runner's.

---

## Steps 3–6 — NOT RUN

| Step | Status |
| --- | --- |
| 3. Remote shape via CLI (3 `.pack` + 3 `.idx`) | NOT RUN — precondition failed (2 + 2 present) |
| 4. Fresh clone, stderr captured | NOT RUN |
| 5. Incremental fetch downloads exactly the new pair — *the stage's selectivity proof* | NOT RUN |
| 6. Up-to-date re-fetch downloads nothing from `packs/` | NOT RUN |

**The stage's selectivity claim is therefore unmeasured.** Nothing in this record should be read
as evidence for or against it.

---

## Step 7 — Cleanup

Verify-before-trashing was performed at every step; exact paths only, no wildcards. Every target
was gate-created (both parents were verified absent in Step 1.2/1.3, so all descendants are
gate-owned).

1. Verified `/my-files/GitRemotes` held only `stage3b-gate`, and `stage3b-gate` held only this
   gate's repo artifacts → `proton-drive filesystem trash /my-files/GitRemotes/stage3b-gate` → `✅ stage3b-gate`
2. Verified `GitRemotes` had no remaining active children → `proton-drive filesystem trash /my-files/GitRemotes` → `✅ GitRemotes`
3. Verified `/my-files/_cas-probe/contract` empty and gate-owned (absent pre-run) →
   `proton-drive filesystem trash /my-files/_cas-probe/contract` → `✅ contract`
4. Verified `_cas-probe` gate-created and otherwise empty → `proton-drive filesystem trash /my-files/_cas-probe` → `✅ _cas-probe`

### Final assertion — post-cleanup vs pre-run listing

The four standing folders are present and **byte-identical** to the pre-run listing — same uids,
same creation and modification times, no `trashTime`, nothing added, nothing altered:

- `Project Repo Bundles` `…~5QY-B6t9Eui6VKsXvJPmuA` — unchanged
- `ChatGPT Export Text Backup` `…~ThDi8B_92zkL76UXhjqFng` — unchanged
- `GitBackups` `…~Vho9hzKVaqnBLf7UnwC64w` — unchanged
- `Sensitive Project Sources` `…~n94X-kyGohebE_FBtxA5Sg` — unchanged

**Assertion — no untouched folder was modified: PASS.**

**Assertion — post-cleanup listing matches pre-run exactly: PARTIAL, by design.** The listing
carries two extra rows, both gate-created and both now bearing a `trashTime`:

```json
{"…":"…","name":{"ok":true,"value":"_cas-probe"},"creationTime":"2026-08-04T05:03:47.000Z","trashTime":"2026-08-04T05:16:18.000Z","type":"folder"}
{"…":"…","name":{"ok":true,"value":"GitRemotes"},"creationTime":"2026-08-04T05:03:50.000Z","trashTime":"2026-08-04T05:16:12.000Z","type":"folder"}
```

`filesystem list` includes trashed nodes, so `trash` alone cannot restore an
exactly-equal listing — only `delete` / `empty-trash` would, and **permanently deleting data is
outside the runner's authority**. No active residue remains under `/my-files`; the two rows are
in the trash and are the user's to empty (or restore) at will. **This is a known limit of the
brief's "matches exactly" wording, not an escaped write** — a future revision of the gate should
state the assertion as "no *active* nodes beyond the pre-run set".

---

## Verdict

**BLOCKED.** Step 1 passed (contract table green live, 9/9 CLI scenarios). Step 2 failed at push 2
of 3 with `create-folder tags … already exists`, so the 3-pack precondition was never
established and Steps 3–6 — including the stage's entire selectivity proof — did not run. Cleanup
completed; the account is back to its pre-run active state.

---

# Run 2

**Verdict: GATE PASSED.** All seven steps executed and passed. The stage's selectivity claim is
now **measured**, not assumed.

- **Date:** 2026-08-04, 15:11–15:38 UTC
- **Repo HEAD:** `8be5845` — **changed from run 1's `dfad74b`**, but the only commit in between is
  `gate(v2): stage3b gate run 1 - BLOCKED on live EnsureDir/Stat staleness`, which touches
  `docs/research/gates/stage3b-gate.md` alone (`git diff --stat dfad74b..HEAD` = 1 file, +309).
  **No helper or transport code changed between runs**, so run 2 exercises the identical binary
  logic and isolates the one variable the controller changed: the emptied trash.
- **Precondition set by the controller:** Craig emptied the Proton trash entirely between runs, to
  test the trashed-homonym hypothesis (probe C17: trashed nodes sharing names with live paths may
  break path resolution).
- **Demo name:** `stage3b-gate-r2` (fresh, per instruction)
- **CLI / git:** unchanged — `cli-drive@0.7.0+5174900c`, `git version 2.53.0.windows.2`
- **Procedure change from run 1:** every push was its own shell invocation with its exit code
  recorded before the next command ran. Run 1's disclosed batching deviation did not recur.

## Run 2 — Step 1: Pre-run listing, parent provisioning, contract table

### 1.1 Fresh pre-run `/my-files` listing (run 2's cleanup contract)

```
proton-drive filesystem list /my-files --json
```

Four rows, **none carrying `trashTime`** — confirming the trash was emptied:

| name | uid | creationTime |
| --- | --- | --- |
| `Project Repo Bundles` | `…~5QY-B6t9Eui6VKsXvJPmuA` | 2026-06-22T16:25:42.000Z |
| `ChatGPT Export Text Backup` | `…~ThDi8B_92zkL76UXhjqFng` | 2026-05-18T02:36:23.000Z |
| `GitBackups` | `…~Vho9hzKVaqnBLf7UnwC64w` | 2026-07-21T00:29:59.000Z |
| `Sensitive Project Sources` | `…~n94X-kyGohebE_FBtxA5Sg` | 2026-05-22T15:12:18.000Z |

Run 1's two trashed rows (`_cas-probe`, `GitRemotes`) are **gone from the listing entirely**, not
merely flagged — the empty-trash was a permanent delete of those nodes.

### 1.2 / 1.3 Parent probes and provisioning

```
proton-drive filesystem list /my-files/_cas-probe --json
Node not found: _cas-probe
EXIT=1

proton-drive filesystem list /my-files/GitRemotes --json
Node not found: GitRemotes
EXIT=1
```

Both ABSENT again. Recorded:

- **gate created `_cas-probe`** — uid `…~yzxv1W0vPJuYL0Wu_dYXKA`, creationTime `2026-08-04T15:11:49.328Z`
- **gate created `GitRemotes`** — uid `…~IkPH9Xzj_QCEvxF3MFAd8g`, creationTime `2026-08-04T15:11:51.800Z`
- **`contract` ABSENT pre-run** → gate-owned, cleaned in Step 7.

Both uids differ from run 1's (`…~YalEhSrh8oFX0lUofsYmew`, `…~CdcWtrrSjXX_3vECaeciuQ`), independently
confirming run 1's nodes were permanently removed rather than restored.

### 1.4 Contract table — **PASS** (with a caching incident, disclosed)

The first invocation returned a **cached** result:

```
ok  	github.com/craigstoller/git-proton-backup/internal/transport	(cached)
```

Go replayed run 1's output verbatim — identical per-scenario timings down to the centisecond
(`TestContractCLI (269.12s)`). **A cached result is not evidence that the live half ran**, which is
precisely what run 2 needed to establish given the account state had changed. The run was
therefore re-issued with `-count=1` to defeat the cache. This is a procedural correction to make
the step actually execute, not a patch, a retry past a surprise, or a change to any code or
assertion.

```
$env:GPB_LIVE_ACCOUNT = "1"; go test ./internal/transport/ -run 'TestContract' -count=1 -v
```

```
--- PASS: TestContractFake (0.10s)
    --- PASS: TestContractFake/stat_absence_is_not_an_error (0.00s)
    --- PASS: TestContractFake/create_refuses_a_local_basename_that_mismatches_the_target_leaf_(C11) (0.00s)
    --- PASS: TestContractFake/create_lands_at_the_target_leaf_when_basenames_agree (0.02s)
    --- PASS: TestContractFake/create_refuses_a_name_already_taken (0.02s)
    --- PASS: TestContractFake/readTo_lands_under_the_remote_basename_in_an_existing_dir (0.03s)
    --- PASS: TestContractFake/readTo_into_a_missing_directory_errors_and_creates_nothing (0.01s)
    --- PASS: TestContractFake/trash_on_a_missing_target_is_committed (0.00s)
    --- PASS: TestContractFake/ensureDir_is_idempotent_and_its_result_is_listable (0.00s)
    --- PASS: TestContractFake/list_of_an_empty_directory_is_empty,_not_an_error (0.00s)
--- PASS: TestContractCLI (270.99s)
    --- PASS: TestContractCLI/stat_absence_is_not_an_error (27.75s)
    --- PASS: TestContractCLI/create_refuses_a_local_basename_that_mismatches_the_target_leaf_(C11) (24.54s)
    --- PASS: TestContractCLI/create_lands_at_the_target_leaf_when_basenames_agree (31.86s)
    --- PASS: TestContractCLI/create_refuses_a_name_already_taken (31.64s)
    --- PASS: TestContractCLI/readTo_lands_under_the_remote_basename_in_an_existing_dir (28.32s)
    --- PASS: TestContractCLI/readTo_into_a_missing_directory_errors_and_creates_nothing (27.23s)
    --- PASS: TestContractCLI/trash_on_a_missing_target_is_committed (26.98s)
    --- PASS: TestContractCLI/ensureDir_is_idempotent_and_its_result_is_listable (39.28s)
    --- PASS: TestContractCLI/list_of_an_empty_directory_is_empty,_not_an_error (33.40s)
PASS
ok  	github.com/craigstoller/git-proton-backup/internal/transport	271.995s
EXIT=0
```

Fresh, distinct timings confirm a genuine live execution. **Assertion — both tests ran live,
neither skipped, both PASS: PASS.**

**Lesson for the runbook:** the gate command as written in the brief is cache-vulnerable. It
should carry `-count=1` so a live gate can never be satisfied by a replayed result.

## Run 2 — Step 2: Build, PATH, demo repo, three isolated pushes — **PASS**

```
go build -o "$env:TEMP\gpb-gate\bin\git-remote-proton.exe" ./cmd/git-remote-proton
BUILD_EXIT=0
RESOLVED=C:\Users\craig\AppData\Local\Temp\gpb-gate\bin\git-remote-proton.exe
```

Source repo `%TEMP%\gpb-gate\src-r2`, `git init -b main`, local `user.name`/`user.email` set,
remote `proton-v2` → `proton::/my-files/GitRemotes/stage3b-gate-r2`.

| # | commit | invocation | exit |
| --- | --- | --- | --- |
| push 1 | `ad4bae3` c1 | its own | **0** |
| push 2 | `318ee89` c2 | its own | **0** |
| push 3 | `510e12a` c3 | its own | **0** |

**Push 2 — run 1's exact failure point — succeeded.**

```
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/gpb-remote.json (42 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/.lock (115 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/HEAD (21 bytes)
To proton::/my-files/GitRemotes/stage3b-gate-r2
   ad4bae3..318ee89  main -> main
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/.lock (115 bytes)
PUSH2_EXIT=0
```

**The run-1 signature (`ensure dir …/refs/tags: create-folder tags … already exists`) did NOT
recur.** No `EnsureDir` error appeared at any point in run 2, across four pushes. The one changed
variable was the emptied trash; the helper code was byte-identical. This is consistent with the
trashed-homonym hypothesis, though a single non-recurrence is corroboration, not proof — the
controller should weigh it as such.

**Assertion — the remote holds 3 packs from 3 pushes: PASS.**

## Run 2 — Step 3: Remote shape via the CLI (read-only) — **PASS**

```
proton-drive filesystem list /my-files/GitRemotes/stage3b-gate-r2/packs --json
```

Exactly **3 `.pack` + 3 `.idx`**, no trashed rows, every stem matching `pack-[0-9a-f]{40}`:

| name | created | claimedSize |
| --- | --- | --- |
| `pack-5d337b0ffe7a30d1ce7ccdcd0e908f3a11f2d7c2.pack` | 15:21:27Z | 221 |
| `pack-5d337b0ffe7a30d1ce7ccdcd0e908f3a11f2d7c2.idx`  | 15:21:35Z | 1156 |
| `pack-c49f015bf30d53ebb16c85154b72de7f3dabb868.pack` | 15:23:52Z | 277 |
| `pack-c49f015bf30d53ebb16c85154b72de7f3dabb868.idx`  | 15:24:01Z | 1156 |
| `pack-72d0e186b0f632e414ab731dce531e8ec5767eb6.pack` | 15:26:17Z | 305 |
| `pack-72d0e186b0f632e414ab731dce531e8ec5767eb6.idx`  | 15:26:26Z | 1156 |

## Run 2 — Step 4: Fresh clone, stderr captured — **PASS**

```
git clone -o proton-v2 "proton::/my-files/GitRemotes/stage3b-gate-r2" clone-r2 2> clone-stderr.txt
CLONE_EXIT=0
```

Full stderr:

```
Cloning into 'clone-r2'...
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/gpb-remote.json (42 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/HEAD (21 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/gpb-remote.json (42 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/packs/pack-5d337b0ffe7a30d1ce7ccdcd0e908f3a11f2d7c2.idx (1156 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/packs/pack-72d0e186b0f632e414ab731dce531e8ec5767eb6.idx (1156 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/packs/pack-c49f015bf30d53ebb16c85154b72de7f3dabb868.idx (1156 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/packs/pack-72d0e186b0f632e414ab731dce531e8ec5767eb6.pack (305 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/packs/pack-5d337b0ffe7a30d1ce7ccdcd0e908f3a11f2d7c2.pack (221 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/packs/pack-c49f015bf30d53ebb16c85154b72de7f3dabb868.pack (277 bytes)
```

**`/packs/`-scoped download set: exactly 3 `.idx` + 3 `.pack` — PASS.**

Checkout and integrity:

```
git rev-parse --abbrev-ref HEAD   → main
git log --oneline -3              → 510e12a c3 / 318ee89 c2 / ad4bae3 c1
git status --porcelain            → (empty)
git fsck                          → no output, FSCK_EXIT=0
```

Cache shape — `clone-r2/.git/proton-v2/idx-cache/fc9e8e56e13d22f2/`:

```
pack-5d337b0ffe7a30d1ce7ccdcd0e908f3a11f2d7c2.idx
pack-72d0e186b0f632e414ab731dce531e8ec5767eb6.idx
pack-c49f015bf30d53ebb16c85154b72de7f3dabb868.idx
remote
```

**3 `.idx` + the `remote` breadcrumb — PASS.** Everything Stage 3a's gate proved is preserved.

## Run 2 — Step 5: Incremental fetch downloads exactly the new pair — **PASS (the selectivity proof)**

Source repo: commit `6c23229` c4, then `git push proton-v2 main` in its own invocation → `PUSH4_EXIT=0`.
Remote now holds 4 packs (8 files); the new stem is **`pack-38e592fbd2e4aac1028f2ac7b73e608dc09d145f`**
(`.pack` 15:30:36Z, `.idx` 15:30:43Z).

```
git fetch proton-v2 2> fetch-stderr.txt
FETCH_EXIT=0
```

Full stderr:

```
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/gpb-remote.json (42 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/HEAD (21 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/gpb-remote.json (42 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/packs/pack-38e592fbd2e4aac1028f2ac7b73e608dc09d145f.idx (1156 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/packs/pack-38e592fbd2e4aac1028f2ac7b73e608dc09d145f.pack (328 bytes)
```

**`/packs/`-scoped download set: exactly `pack-38e592fb….idx` and `pack-38e592fb….pack` — the new
stem, and nothing else. PASS.** The three pre-existing packs were not re-downloaded; their `.idx`
files were served from the cache. **This is the stage's selectivity claim, measured against the
real account.**

```
git log --oneline -4 proton-v2/main  → 6c23229 c4 / 510e12a c3 / 318ee89 c2 / ad4bae3 c1
git fsck                             → no output, FSCK_EXIT=0
```

Cache after the fetch — 4 `.idx` + `remote`, the new stem added in place:

```
pack-38e592fbd2e4aac1028f2ac7b73e608dc09d145f.idx
pack-5d337b0ffe7a30d1ce7ccdcd0e908f3a11f2d7c2.idx
pack-72d0e186b0f632e414ab731dce531e8ec5767eb6.idx
pack-c49f015bf30d53ebb16c85154b72de7f3dabb868.idx
remote
```

## Run 2 — Step 6: Up-to-date re-fetch downloads nothing from `packs/` — **PASS**

```
git fetch proton-v2 2> refetch-stderr.txt
REFETCH_EXIT=0
```

Full stderr:

```
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/gpb-remote.json (42 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage3b-gate-r2/HEAD (21 bytes)
```

**`/packs/`-scoped download count: 0 — PASS.** The three remaining lines are ref/marker reads,
legitimate and excluded per the spec's gate scoping.

## Run 2 — Step 7: Cleanup — **PASS**

Verify-before-trashing at every step; exact paths, no wildcards. Verified contents immediately
before trashing:

```
/my-files/GitRemotes                   → stage3b-gate-r2 [folder]            (only this gate's repo)
/my-files/GitRemotes/stage3b-gate-r2   → gpb-remote.json, refs, packs, HEAD  (only gate artifacts)
/my-files/_cas-probe                   → contract [folder]                   (gate-owned, absent pre-run)
/my-files/_cas-probe/contract          → (empty)
```

```
proton-drive filesystem trash /my-files/GitRemotes/stage3b-gate-r2  → ✅ stage3b-gate-r2   EXIT=0
proton-drive filesystem trash /my-files/GitRemotes                  → ✅ GitRemotes        EXIT=0
proton-drive filesystem trash /my-files/_cas-probe/contract         → ✅ contract          EXIT=0
proton-drive filesystem trash /my-files/_cas-probe                  → ✅ _cas-probe        EXIT=0
```

### Final assertion — post-cleanup vs the run-2 pre-run listing

```
ROWS=4
  Project Repo Bundles
  ChatGPT Export Text Backup
  GitBackups
  Sensitive Project Sources
```

Active-uid comparison:

```
PRE_UIDS         = …~5QY-B6t9Eui6VKsXvJPmuA, …~n94X-kyGohebE_FBtxA5Sg, …~ThDi8B_92zkL76UXhjqFng, …~Vho9hzKVaqnBLf7UnwC64w
POST_ACTIVE_UIDS = …~5QY-B6t9Eui6VKsXvJPmuA, …~n94X-kyGohebE_FBtxA5Sg, …~ThDi8B_92zkL76UXhjqFng, …~Vho9hzKVaqnBLf7UnwC64w
MATCH=True
```

**Assertion — post-cleanup listing matches the pre-run listing exactly: PASS.** Unlike run 1, the
listing contains **zero `trashTime` rows** — the four gate-created nodes do not appear at all. A
confirming re-list returned the same four rows, so this is stable, not listing lag. Run 1's
"trashed nodes linger in `list`" observation therefore does not reproduce here; with a
freshly-emptied trash the pre/post comparison holds literally, as the brief intended.

The four standing folders are byte-identical to the pre-run capture — same uids, same creation
times, no `trashTime`, nothing added or altered. `GitBackups`, `Sensitive Project Sources`,
`Project Repo Bundles` and `ChatGPT Export Text Backup` were never named as a target in any
command.

## Run 2 — Verdict

**GATE PASSED.** Steps 1–7 all pass. The two run-1 blockers are resolved: push 2 succeeded with no
`EnsureDir` error (the only changed variable being the emptied trash), and the selectivity proof
was measured end to end —

- **clone:** 3 `.idx` + 3 `.pack` from `/packs/`
- **incremental fetch:** exactly the one new pair, `pack-38e592fb….idx` + `.pack`
- **up-to-date re-fetch:** 0 downloads from `/packs/`

with `git fsck` clean at every checkpoint and the account restored exactly to its pre-run state.

Two follow-ups for the controller, neither blocking:

1. **The gate command needs `-count=1`.** Run 2's first invocation was served from Go's test cache
   and replayed run 1's output verbatim — a live gate that can be satisfied by a cached result is
   a false-green vector. Task 8's brief should be amended.
2. **The homonym hypothesis is corroborated, not proven.** One non-recurrence across four pushes
   with byte-identical code is strong but single-trial evidence. If the mechanism matters for the
   design record, it deserves a dedicated probe rather than inference from this gate.
