# Task 2 record — Proton CLI 0.8.0 live certification

Runner log, appended as work proceeds. Brief: `task2-brief.md` (same dir).
Timestamps: `Get-Date -Format o` (Pacific, DST → UTC-07:00).

---

## Step 1 — Preconditions

### 2026-08-13T19:47:10-07:00 — `ProjectsBackupDaily` state (READ-ONLY `schtasks /query`)

```
TaskName:                             \ProjectsBackupDaily
Next Run Time:                        N/A
Status:                               Disabled
Last Run Time:                        8/13/2026 12:30:01 PM
Last Result:                          1
Scheduled Task State:                 Disabled
Schedule Type:                        Daily
Start Time:                           12:30:00 PM
```

**PASS** — Craig's kickoff disable is confirmed in effect (`Scheduled Task State: Disabled`,
`Next Run Time: N/A`). No `/change` was issued by the runner; this is a read-only query.
Craig also reported emptying the Proton trash via the web UI before kickoff (checklist rule 6),
recorded here as his attestation.

### 2026-08-13T19:47:11-07:00 — process check

```
NO proton-drive process running
ProtonDrive.exe PID=18460 Start=2026-08-13T18:09:37.7588027-07:00
```

No CLI process. `ProtonDrive.exe` is the sync app (expected, untouched, informational only).

### 2026-08-13T19:47:12-07:00 — machine CLI `--version` (NOT swapped yet)

```
Proton Drive CLI cli-drive@0.7.0+5174900c
Proton Drive SDK js@0.20.0+5174900c
exit=0
```

Expected 0.7.0 token confirmed — the machine install is untouched at gate entry.

### 2026-08-13T19:47:13-07:00 — credential metadata WATCH BASELINE (`cmdkey /list` filtered, no secret read)

```
Local machine persistence
Target: LegacyGeneric:target=ch.proton.drive/drive-sdk-cli/auth-session
Type: Generic
User: auth-session
```

Identical to Task 1's baseline and post-audit checks.

### 2026-08-13T19:49:22-07:00 — `/my-files` BASELINE row set (authenticated, read-only, 0.7.0 CLI)

`proton-drive filesystem list /my-files --json` → exit 0, 5 rows:

```
uid                                            name                        type    parentUid                                       trashTime
tU-Ot1Sq63NwBcxlnl7IcA~5QY-B6t9Eui6VKsXvJPmuA  Project Repo Bundles        folder  tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g  (none)
tU-Ot1Sq63NwBcxlnl7IcA~TfvH3TrgNW1Eyo6Urs7hmQ  _cas-probe                  folder  tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g  (none)
tU-Ot1Sq63NwBcxlnl7IcA~ThDi8B_92zkL76UXhjqFng  ChatGPT Export Text Backup  folder  tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g  (none)
tU-Ot1Sq63NwBcxlnl7IcA~Vho9hzKVaqnBLf7UnwC64w  GitBackups                  folder  tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g  (none)
tU-Ot1Sq63NwBcxlnl7IcA~n94X-kyGohebE_FBtxA5Sg  Sensitive Project Sources   folder  tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g  (none)
```

No `trashTime` on any row — consistent with Craig's trash-emptying report. Four untouchables
present; `_cas-probe` is a pre-existing folder (the contract table's DEFAULT `liveRoot` parent),
also never touched by this run.

### 2026-08-13T19:49:28-07:00 — `/my-files/GitRemotes` existence probe

```
proton-drive filesystem info /my-files/GitRemotes --json
Node not found: GitRemotes
exit=1
```

**ABSENT** (as anticipated — Stage 6's cleanup trashed it). Parent creation is authorised by the
brief's confinement list; will use the two-argument form.

### 2026-08-13T19:49:43-07:00 — credential re-check after the baseline-listing authenticated stretch

Identical to the baseline (target / type / user unchanged). **No change → no STOP.**

---

## Step 2 — Freeze the candidate (plan Task 2.0)

### 2026-08-13T19:47 PT — worktree HEAD

```
e1a70eba7da7aef0be7af3dda6a591e64cc5dfe6
```

**MATCHES the required freeze point `e1a70eb`.** `git status --porcelain` shows only `??
.superpowers/` (untracked evidence dir) — no tracked modifications. Log:
`e1a70eb` (sentinel fix) → `2dcbcc5` (0.8.0 cert) → `0976eb7` (plan adjudication).

### 2026-08-13T19:49:43-07:00 — binary digests, recomputed

| Binary | Digest | Size |
|---|---|---|
| staged 0.8.0 | SHA-512 `03ce039618212c4cb9f175e3d53889657484cee9bfb0b1a24ad3f5b5e703ccf5784119acfa44c65bdda2af2b273c11b91075424ac169824b28edac49399b5a9a` | 121,838,592 |
| archived 0.7.0 | SHA-256 `180c0633fe05560d65bdea09bb3b9fcc11acbb121b4f3f862562d3cd5061839b` | 121,845,248 |
| installed (machine) | SHA-256 `180c0633fe05560d65bdea09bb3b9fcc11acbb121b4f3f862562d3cd5061839b` | 121,845,248 |

Archive `README.txt` states SHA-256 `180c0633fe05560d65bdea09bb3b9fcc11acbb121b4f3f862562d3cd5061839b`
— **MATCH** against the recomputed archive digest, and the installed machine binary is
**hash-identical** to the archive (so the rollback source is byte-verified before the swap).
Staged SHA-512 matches Task 1's recorded value exactly.

### 2026-08-13T19:49:55 / 19:50:02-07:00 — version.json drift re-check

`https://proton.me/download/drive/cli/version.json` → StatusCode 200.
`Releases[0]` = `{CategoryName: Stable, Version: 0.8.0, ReleaseDate: 2026-08-13}`.

The first probe queried `Releases[0].File` with separate `Platform`/`Arch` fields and returned
null — **a defect in the runner's query, not a malformed feed**; recorded for honesty. The real
shape is `Releases[0].Files[]` with a single combined `Platform` string. Re-queried correctly:

```
Url: https://proton.me/download/drive/cli/0.8.0/windows-x64/proton-drive.exe
Sha512CheckSum: 03ce039618212c4cb9f175e3d53889657484cee9bfb0b1a24ad3f5b5e703ccf5784119acfa44c65bdda2af2b273c11b91075424ac169824b28edac49399b5a9a
Platform: windows/x64
```

**MATCH** with the staged binary's freshly recomputed SHA-512 → **no drift**. All 9
platform entries present (macos arm64/x64, 6 linux variants, windows arm64/x64) — feed is
well-formed, windows/x64 present, fail-closed condition NOT triggered.

**Freeze PASS.**

---

## Step 3 — State refresh (supersedes Task 1's snapshot as the restore point)

### 2026-08-13T19:52:37-07:00 — copy of `%LOCALAPPDATA%\proton-drive-cli`

Copied via `robocopy /MIR` (exit 1 = files copied, success) into
`.superpowers\cli-080-cert\state-snapshot-task2\`. 14 files — same file set as Task 1's snapshot.
A first attempt using `Copy-Item -Force` after a `Remove-Item -Recurse` was **blocked by the
harness sandbox** ("Remove-Item on system path ... is blocked"); no state was changed by the
blocked attempt, and the destination did not yet exist, so the retry simply dropped the
`Remove-Item` and used `robocopy`. Recorded for completeness.

### 2026-08-13T19:52:47-07:00 — SHA-256 manifest (written to `state-snapshot-task2-manifest.txt`)

```
34BCF0318940F6A8A7AE2D6311F4D489C25717688A99A07470DF59E406A4BDE7  Cache\cache-crypto.sqlite  (5255168 bytes)
34329D3DEDB48E1E6BF4E17E86CC53BBAFF570B3E02D1BAD23E3B8458FE911EA  Cache\cache-crypto.sqlite-shm  (32768 bytes)
8F73FF4A191A4E6820AA4378EF8BECA38681FE7BBA084D57E08EE26A9EB0643B  Cache\cache-crypto.sqlite-wal  (5199472 bytes)
9F56590973E5B63581C5378308640D87EEE80689529A7576628DD3E402284ECB  Cache\cache-entities.sqlite  (3215360 bytes)
4FF34E0497B31350E78BD9FC50401142F04E32BAD23E10D55D28E704164209CC  Cache\cache-entities.sqlite-shm  (32768 bytes)
0FD994B182C187A45B18BCC23D40F54F5B7A73AEC030398919864CDC100E5CE2  Cache\cache-entities.sqlite-wal  (6348952 bytes)
A65D35C6B7EBD83AA606DCF7654EAAF7F0EA8471A5C6CB57346BA73D7F16E718  Data\clientUid.json  (69 bytes)
4AEED20CFD21F9AB1E6E65BC5B44D91C4611AD786275FA16C802B824A89853A9  Data\events.json  (269 bytes)
3E07D0EF8AA52BAACC2950AE48DE9F33914C21F580A0083AD5B6EE665653D664  Logs\proton-drive.log  (194296 bytes)
E404E8F7CDCDA4B93F32FBCC61D927BF4120263C817A33A0991C1D812B65CD16  Logs\proton-drive.log.1  (5242851 bytes)
621AB475DE137FE9BF7AC43547C8E501CA9635A4F24F95EA29EAF69A6672B6EA  Logs\proton-drive.log.2  (5242866 bytes)
E62162FBA918358C94DBDA4A0EFCB972FBFF8D40E5B79A9AC1815D6B2008E723  Logs\proton-drive.log.3  (5242814 bytes)
415A97A6129C7ABE625384926F59F4D4DD3EB2CF17F95B0EF26B09C5B4C85E78  Logs\proton-drive.log.4  (5242837 bytes)
1BFBF90A5528E9CD9E3B3D4295AF605A5E50B70EB1405816632AAB1AE9FF44C6  Logs\proton-drive.log.5  (5242867 bytes)
```

**This snapshot — not Task 1's — is the restore point for Task 2.4's rollback.** It differs
from Task 1's because the machine ran normally (0.7.0) between the two tasks: `-shm`/`-wal`,
`events.json` and `proton-drive.log` moved; `clientUid.json` and the rotated logs `.1`–`.5` are
byte-identical to Task 1's.

Credential metadata re-checked immediately after the snapshot — **unchanged**.

---

## Step 4 — Build the frozen candidate helper

### 2026-08-13T19:53:01-07:00

```
go:  C:\Program Files\Go\bin\go.exe
git: C:\Program Files\Git\cmd\git.exe
HEAD at build time: e1a70eba7da7aef0be7af3dda6a591e64cc5dfe6
go build -o C:\gpb-cert\helper\git-remote-proton.exe ./cmd/git-remote-proton   -> exit 0
```

| | |
|---|---|
| Candidate helper | `C:\gpb-cert\helper\git-remote-proton.exe` |
| SHA-256 | `7967470a5d2fed1c7f172ee07a25eaa4fec688359bdb491fff168532c8a30aff` |
| Size | 4,752,896 bytes |

`--version` (run with the staged CLI dir prepended; `proton-drive` resolved to
`C:\Users\craig\Tools\archive\proton-drive-cli\0.8.0-staged\proton-drive.exe`):

```
git-remote-proton dev (certified CLI: cli-drive@0.8.0+06e8c605)
exit=0
```

Matches the brief's deviation **D2** exactly: the helper reports its own build (`dev`, unstamped
— no release ldflags on a local build) and the COMPILED-IN certified token. It does not probe
the CLI on this path, so this output is not evidence about what is on PATH; the real allowlist
check (`EnforceCertified`) runs at push time.

---

## Step 5a — FIRST authenticated staged-0.8.0 run + mandated state diff

### 2026-08-13T19:53:40-07:00 — credential PRE: unchanged from baseline

### 2026-08-13T19:53:47-07:00 — `filesystem create-folder /my-files GitRemotes --json` (staged 0.8.0)

Staged binary's identity in this shell:

```
Proton Drive CLI cli-drive@0.8.0+06e8c605
Proton Drive SDK js@0.21.0+06e8c605
You are running the latest version.
```

Create → exit 0:

```json
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~JRwbSWEhNjJHbYtAE9OblA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g","name":{"ok":true,"value":"GitRemotes"},"directRole":"inherited","type":"folder","mediaType":"Folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-14T02:53:47.140Z","modificationTime":"2026-08-14T02:53:47.140Z","folder":{"isImported":false}}
```

`filesystem info /my-files/GitRemotes --json` → exit 0, same uid
`tU-Ot1Sq63NwBcxlnl7IcA~JRwbSWEhNjJHbYtAE9OblA`.

**SESSION PORTABILITY CONFIRMED** — the staged 0.8.0 performed an authenticated, mutating call
using the EXISTING session with no re-auth and no prompt. The plan's session-non-portability
STOP class is retired.

`/my-files` row set after the create — the 5 baseline rows plus `GitRemotes`, no `trashTime` on
any row. **Row order differed from the baseline listing** (`ChatGPT…` first here vs `Project
Repo Bundles` first at 19:49) — a live demonstration of why checklist rule 1 forbids raw-JSON
equality; the ROW SET is what matched.

### 2026-08-13T19:53:54-07:00 — credential POST: unchanged. No STOP.

### 2026-08-13T19:54:09-07:00 — state diff vs the step-3 snapshot (plan Task 2.1)

```
SAME     Cache\cache-crypto.sqlite
CHANGED  Cache\cache-crypto.sqlite-shm
CHANGED  Cache\cache-crypto.sqlite-wal
CHANGED  Cache\cache-entities.sqlite
CHANGED  Cache\cache-entities.sqlite-shm
CHANGED  Cache\cache-entities.sqlite-wal
SAME     Data\clientUid.json
CHANGED  Data\events.json
CHANGED  Logs\proton-drive.log
SAME     Logs\proton-drive.log.1 … .5
```

Zero new files, zero missing files. No new proton-ish directory anywhere
(`%LOCALAPPDATA%`/`%APPDATA%`/`%USERPROFILE%` re-scanned: the same six as Task 1);
`proton-drive-cli`'s own top level still exactly `Cache` / `Data` / `Logs`.

**Assessment: the CLI's OWN session refresh, not a migration.** The changed files are the SQLite
cache pair's `-shm`/`-wal`, the entities cache, the event cursor, and the appended log — exactly
what an authenticated list/create round-trip touches. Decisive negatives: `clientUid.json` is
**byte-identical** (no client re-registration), the state remained readable throughout, and the
Credential Manager entry is unchanged. **No STOP.**

---

## Step 5b — Contract table (plan Task 2.2)

### 2026-08-13T19:54:32-07:00 — shell verification, recorded IN THE TEST SHELL

```
cwd: C:\Users\craig\Projects\_Tools\git-proton-backup\.claude\worktrees\cli-080-cert
HEAD: e1a70eba7da7aef0be7af3dda6a591e64cc5dfe6
proton-drive resolves to: C:\Users\craig\Tools\archive\proton-drive-cli\0.8.0-staged\proton-drive.exe
Proton Drive CLI cli-drive@0.8.0+06e8c605
Proton Drive SDK js@0.21.0+06e8c605
You are running the latest version.
GPB_UNCERTIFIED_CLI is set? False (value='')
GPB_LIVE_ACCOUNT=1
GPB_CONTRACT_LIVE_ROOT=/my-files/GitRemotes/cli080-cert-contract
```

`(Get-Command proton-drive).Source` IS the staged 0.8.0 exe in the shell that runs the table;
`GPB_UNCERTIFIED_CLI` is unset. Command:
`go test ./internal/transport/ -run TestContractCLI -count=1 -timeout 40m -v`, run in the
background with the env vars removed in the same call. Full transcript: `task2-contract-run.log`.

### Local smoke preparation (2026-08-13T19:55:48-07:00 — no account access)

`git init -b main C:\gpb-cert\smoke`; one empty commit
`2251f08fc2978579bdfd3fc2fd81d5d4f3287858`. Junk fixture written: `C:\gpb-cert\junk-a`,
**41 bytes**, SHA-256 `3164596df4fdd018b2c567ec8c03e79bd76f4e4ca2a3fd020b577300ff9d2aca`.

### Code-derived expectations for the junk check (read at the frozen candidate, before running it)

`ScanRefs` (`internal/repo/refs.go:67-121`) walks from `"refs"`, so the junk lands as
`refs/heads/junk-a`; `advertisableName` accepts that name, so it reaches `readRefClassified`
with `size = 41` — INSIDE the `[40,42]` candidate band, so it is downloaded and grammar-checked
rather than size-skipped. 41 `x` bytes are not 40-hex, so `classifyRefContent` takes the generic
arm. Expected stderr shape (`warn`, main.go:706, prefixes `git-remote-proton: `):

```
cannot serve a fetch: 1 file(s) under refs/ are not valid refs and a restore would silently lack them:
  /my-files/GitRemotes/cli080-cert/refs/heads/junk-a: not a ref: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
delete these files first (proton-drive filesystem trash <path>, or the web UI; Proton trash keeps them restorable), then retry
```

Expected remote layout after bootstrap (`marker.go:102-141`, `ensureSubdirs:251-261`):
`gpb-remote.json` (marker), `refs/`, `refs/heads/`, `refs/tags/`, `packs/`, plus `HEAD` and a
transient `.lock` — the enumeration at verify-before-trash will confirm.

### 2026-08-13T20:04:44-07:00 — RESULT: **all 17 rows PASS**, `go test exit=0`, 605.282s

```
--- PASS: TestContractCLI (604.48s)
    --- PASS: TestContractCLI/stat_absence_is_not_an_error (31.98s)
    --- PASS: TestContractCLI/stat_not-found_is_pinned_against_the_certified_CLI's_own_signature_(Task_4) (28.55s)
    --- PASS: TestContractCLI/create_refuses_a_local_basename_that_mismatches_the_target_leaf_(C11) (24.00s)
    --- PASS: TestContractCLI/create_lands_at_the_target_leaf_when_basenames_agree (32.49s)
    --- PASS: TestContractCLI/create_refuses_a_name_already_taken (29.61s)
    --- PASS: TestContractCLI/readTo_lands_under_the_remote_basename_in_an_existing_dir (29.41s)
    --- PASS: TestContractCLI/readTo_into_a_missing_directory_errors_and_creates_nothing (30.23s)
    --- PASS: TestContractCLI/download_of_a_directory_recursively_materialises_the_subtree_(F1) (56.26s)
    --- PASS: TestContractCLI/trash_on_a_missing_target_is_committed (22.77s)
    --- PASS: TestContractCLI/ensureDir_is_idempotent_and_its_result_is_listable (36.94s)
    --- PASS: TestContractCLI/list_of_an_empty_directory_is_empty,_not_an_error (30.52s)
    --- PASS: TestContractCLI/trash_on_an_empty_folder_is_committed_and_the_folder_is_gone (39.48s)
    --- PASS: TestContractCLI/trash_on_a_folder_with_children_removes_the_whole_subtree (51.72s)
    --- PASS: TestContractCLI/create-folder_refuses_a_name_already_taken_by_a_file (32.67s)
    --- PASS: TestContractCLI/create-folder_on_an_existing_folder_reports_the_already-exists_signature_(Task_9b,_C17b) (36.44s)
    --- PASS: TestContractCLI/upload_of_a_file_colliding_with_an_existing_folder_name_does_not_silently_succeed (36.15s)
    --- PASS: TestContractCLI/nested_list_reports_folders_and_files_with_the_correct_IsDir (55.25s)
PASS
ok  	github.com/craigstoller/git-proton-backup/internal/transport	605.282s
go test exit=0
env after cleanup: LIVE='' ROOT=''
```

**Signature re-pins captured LIVE against 0.8.0** (the code-level claims Task 1 could only make
structurally are now observed):

```
contract_test.go:187: live not-found output (must contain notFoundSignature "Node not found"):
  "Node not found: definitely-absent-t4-notfound-signature\n"
contract_test.go:482: live already-exists output (must contain alreadyExistsSignature "already exists"):
  "A file or folder with that name already exists\n"
contract_test.go:322: live directory-download output (F1):
  "Transfer summary:\n  Downloaded: 4 items (2 B)\n"
```

Both `strings.Contains` matchers still hold against 0.8.0's real error text. C16/C10's empty
listing shape also held (row 11 passed, and the contract root listed as `[\n\n]` afterwards).

### 2026-08-13T20:05:17-07:00 — post-run checks

- Credential metadata: **unchanged**.
- State diff vs the step-3 snapshot: the same session-refresh set (both cache DBs + their
  `-shm`/`-wal`, `events.json`, `proton-drive.log`); `clientUid.json` and logs `.1`–`.5`
  byte-identical; **zero new files, zero missing files**; state readable. Not a migration.
- Contract root `/my-files/GitRemotes/cli080-cert-contract` listed **empty** (`[\n\n]`) — the
  table's own `t.Cleanup` trashed all 17 per-test roots.

---

## Step 6 — Smoke (plan Task 2.3)

Every smoke shell prepended BOTH dirs and recorded the resolutions:
`git-remote-proton → C:\gpb-cert\helper\git-remote-proton.exe`;
`proton-drive → C:\Users\craig\Tools\archive\proton-drive-cli\0.8.0-staged\proton-drive.exe`;
`GPB_UNCERTIFIED_CLI=''`. Transcripts: `task2-smoke-1-bootstrap.log` … `-4-junk.log`.

### 2026-08-13T20:05:38 → 20:08:16-07:00 — bootstrap push (`task2-smoke-1`)

```
To proton::/my-files/GitRemotes/cli080-cert
 * [new branch]      main -> main
branch 'main' set up to track 'proton-v2/main'.
push exit=0
```

This is the **real allowlist evidence**: the helper's `EnforceCertified` ran against whatever
`proton-drive` resolved to and did not refuse, so the staged 0.8.0 satisfied
`CertifiedCLI = "cli-drive@0.8.0+06e8c605"` on the live path (unlike `--version`, which only
prints the constant — deviation D2).

### 2026-08-13T20:08:41-07:00 — update-path BEFORE capture (deviation D1)

`refs/heads` had exactly one row, `main`:

| | BEFORE |
|---|---|
| node uid | `tU-Ot1Sq63NwBcxlnl7IcA~cpMDj3FHenb_XPvYnhqiXg` |
| activeRevision uid | `…~cpMDj3FHenb_XPvYnhqiXg~pCSWT3b-7GImC24T2YS4bA` |
| claimedSize | 41 |
| totalStorageSize | 119 |
| read-back bytes | 41 — `2251f08fc2978579bdfd3fc2fd81d5d4f3287858\n` |

**Revision observability: AVAILABLE.** `filesystem info --json` exposes
`activeRevision.uid`, so the plan's stronger assertion applies — the accepted-limit fallback is
NOT needed.

### 2026-08-13T20:09:11 → 20:11:40-07:00 — the update push (`task2-smoke-2`)

Second empty commit `9a2cbf02c7e4cca544739294ddbbd9a1658c067e`; push:

```
To proton::/my-files/GitRemotes/cli080-cert
   2251f08..9a2cbf0  main -> main
push exit=0
```

### 2026-08-13T20:12:07-07:00 — update-path AFTER capture and INVARIANTS

| | AFTER |
|---|---|
| node uid | `tU-Ot1Sq63NwBcxlnl7IcA~cpMDj3FHenb_XPvYnhqiXg` (**identical**) |
| activeRevision uid | `…~NrciaqeQ0NsF5b-4w23b_w` (**changed**) |
| claimedSize | 41 |
| totalStorageSize | 238 (two revisions retained; active `storageSize` still 119) |
| read-back bytes | 41 — `9a2cbf02c7e4cca544739294ddbbd9a1658c067e\n` |

- **STABLE IDENTITY: PASS** — same node uid across `create-new-revision`. The plan's
  architectural STOP (uid change breaking the ref→node identity model) is **retired**: 0.8.0's
  `create-new-revision` preserves Stage 1 C1's semantics exactly as the rename hypothesised.
- **EXACTLY ONE NEW REVISION: PASS** — revision uid changed once; `totalStorageSize` 119 → 238
  is consistent with revisioning, not with replacement (a `replace` would have trashed and
  recreated the node, changing the uid).
- **SINGLE LISTING ROW: PASS** — `refs/heads` still one row named `main`; no duplicate, no
  renamed sibling.
- **READ-BACK BYTES: PASS** — equal the pushed sha + `"\n"`.
- **SIBLINGS UNTOUCHED: PASS** — repo-root rows `gpb-remote.json`, `refs`, `packs`, `HEAD` all
  carry their original uids.

### 2026-08-13T20:12:45 → 20:18:19-07:00 — hierarchical branch, clone, fetch (`task2-smoke-3`)

`feature/x` (a NESTED ref name, so the remote holds `refs/heads/feature/x`) pushed:
`* [new branch] feature/x -> feature/x`, exit 0. Clean `ls-remote` exit 0:

```
9a2cbf02c7e4cca544739294ddbbd9a1658c067e	refs/heads/main
b19532f0d753bb7df90998f6e2c1eff6f05d886f	refs/heads/feature/x
9a2cbf02c7e4cca544739294ddbbd9a1658c067e	HEAD
```

Clone into `C:\gpb-cert\clone1` exit 0 (3 packs + 3 idx downloaded), checked out `main` at
`9a2cbf0`. `git fetch proton-v2` exit 0. **rev-parse match:** clone `proton-v2/main` =
`9a2cbf02c7e4cca544739294ddbbd9a1658c067e`, clone `proton-v2/feature/x` =
`b19532f0d753bb7df90998f6e2c1eff6f05d886f` — both equal the pushed shas.

### 2026-08-13T20:18:35 → 20:19:30-07:00 — strict-fetch junk check (`task2-smoke-4`)

Upload of the 41-byte fixture: `Uploaded: 1 items (41 B)`, exit 0; it landed as
`refs/heads/junk-a`, uid `tU-Ot1Sq63NwBcxlnl7IcA~HDcdseKfxHgyfBhnw19jSw`. Then:

```
gpb: downloaded /my-files/GitRemotes/cli080-cert/refs/heads/junk-a (41 bytes)
git-remote-proton: skipping /my-files/GitRemotes/cli080-cert/refs/heads/junk-a: not a ref: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
git-remote-proton: cannot serve a fetch: 1 file(s) under refs/ are not valid refs and a restore would silently lack them:
  /my-files/GitRemotes/cli080-cert/refs/heads/junk-a: not a ref: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
delete these files first (proton-drive filesystem trash <path>, or the web UI; Proton trash keeps them restorable), then retry

ls-remote exit=128
```

**PASS** — nonzero as required, **exit 128** (the precedented code), and the enumerated error
matches the text quoted from `main.go` in the brief **verbatim**, including the `skipNote`
line, the count, the full remote path, the `%q`-escaped 41-byte preview, and the closing
remediation sentence. The junk was downloaded (not size-skipped), confirming the 41-byte fixture
exercised the in-band grammar/classification path as designed.

### 2026-08-13T20:19:59 → 20:20:55-07:00 — verify-before-trash, trash, recovery

Verify-before-trash: `filesystem info` on the target confirmed uid
`tU-Ot1Sq63NwBcxlnl7IcA~HDcdseKfxHgyfBhnw19jSw`, name `junk-a`, **type `file`** (so there is no
subtree beneath it), `claimedSize` 41 — this run's own fixture, matching the uid recorded at
upload. Trash → `[{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~HDcdseKfxHgyfBhnw19jSw","ok":true}]`, exit 0.
`refs/heads` then listed 2 rows (`main` file, `feature` folder), no `trashTime` on either — the
trashed row left the live tree immediately, no lag tolerance needed.

`git ls-remote` **succeeds again**, exit 0, same three lines as before the junk. Credential
metadata re-checked: **unchanged**.

**Smoke: PASS.**

---

## Step 7 — Machine swap (plan Task 2.4)

Entered only after steps 5 and 6 both passed. Transcript: `task2-fleet-verify.log`.

### 2026-08-13T20:22:20-07:00 — pre-swap gates

- **Quiesce still holds**: `\ProjectsBackupDaily` → `Status: Disabled`, `Scheduled Task State:
  Disabled`, `Next Run Time: N/A`.
- **No `proton-drive` process running.**
- **Both binaries re-hashed**: staged SHA-512 `03ce0396…399b5a9a` MATCH; archived 0.7.0 SHA-256
  `180c0633…5061839b` MATCH; the installed binary was still byte-identical to the archive
  (`180c0633…`) — so the rollback source was verified good immediately before overwriting.
  Staged SHA-256 recorded for the post-copy check: `3ffb839d9148dc877d10422064ca7c7776befbbf79ad6cbba60bc34002ce19b5`.

### 2026-08-13T20:22:22-07:00 — swap

`Copy-Item` staged → `C:\Users\craig\Tools\proton-drive.exe`. Post-copy verification:

```
installed AFTER SHA-512:   03ce039618212c4cb9f175e3d53889657484cee9bfb0b1a24ad3f5b5e703ccf5784119acfa44c65bdda2af2b273c11b91075424ac169824b28edac49399b5a9a
INSTALLED == STAGED (512): True
INSTALLED == STAGED (256): True
installed size: 121838592
```

### 2026-08-13T20:22:24-07:00 — post-swap version + auth probe

```
Proton Drive CLI cli-drive@0.8.0+06e8c605
Proton Drive SDK js@0.21.0+06e8c605
You are running the latest version.
version exit=0
```

Auth probe `proton-drive filesystem list /my-files` → **exit 0**, returned the full 6-row
listing. Credential metadata re-checked: **unchanged**. **No rollback trigger** — the version
check and the auth probe both passed, so the "0.8.0 broken vs session needs re-auth"
disambiguation never had to be applied.

### 2026-08-13T20:22:42 → 20:27:01-07:00 — fleet verify (the v1 MODULE, no CLI allowlist)

```
proton-drive on PATH: C:\Users\craig\Tools\proton-drive.exe
Proton Drive CLI cli-drive@0.8.0+06e8c605
module imported: 0.6.0
ExitCode         : 0
Complete         : True
IncompleteReason :
Findings         : {}
ExitCode: 0
Findings count: 0
```

**Fleet verify GREEN against the swapped 0.8.0 CLI: exit 0, Complete=True, 0 findings**, every
enumerated repo `State=ok`. The machine is left on the swapped, verified 0.8.0 — never
mid-swap.

---

## Step 8 — Cleanup

### 2026-08-13T20:27:23-07:00 — verify-before-trash, FULL subtree enumeration

```
/my-files/GitRemotes
  cli080-cert-contract  [folder]  uid=…~e4p-4gCYaprI5CTz1ycnjQ            (empty)
  cli080-cert           [folder]  uid=…~dE_SL-muVxOeoIISf5s_3A
    gpb-remote.json     [file]    uid=…~R7zzXKA0RJGGSFD9TNcEcA   size=42
    refs                [folder]  uid=…~CmzsLTPcD88vXeSpiwLbyw
      heads             [folder]  uid=…~jMK0eCIriulBtpMfhSct8A
        main            [file]    uid=…~cpMDj3FHenb_XPvYnhqiXg   size=41
        feature         [folder]  uid=…~rsE7SPZHBCwV-ynsn7Lb1w
          x             [file]    uid=…~Oine3Q_CYJs8W6UE68RExg   size=41
      tags              [folder]  uid=…~arIjxPWjIlXgwEKVyq_4YQ            (empty)
    packs               [folder]  uid=…~Pdth8kgqnynqj_8RXQ-96w
      pack-6eef3a33….pack / .idx  (195 / 1128)
      pack-3dd4ef60….pack / .idx  (232 / 1100)
      pack-d45df1ed….pack / .idx  (198 / 1100)
    HEAD                [file]    uid=…~y-VqV_p5jOTqRJDfk17TUA   size=21
```

Every node is this gate's own artifact; no foreign content; no untouchable folder appears
anywhere in the subtree or in any trash argument. No `trashTime` on any row.

### 2026-08-13T20:28:10-07:00 — trash the two roots

```
trash /my-files/GitRemotes/cli080-cert           -> [{"uid":"…~dE_SL-muVxOeoIISf5s_3A","ok":true}]  exit 0
trash /my-files/GitRemotes/cli080-cert-contract  -> [{"uid":"…~e4p-4gCYaprI5CTz1ycnjQ","ok":true}]  exit 0
list  /my-files/GitRemotes                       -> [\n\n]   live row count: 0
```

`/my-files/GitRemotes` empty on the FIRST listing — **no trash lag**, the ~30 s tolerance was
not needed.

### 2026-08-13T20:28:45-07:00 — trash the parent, final row-set comparison

```
trash /my-files/GitRemotes -> [{"uid":"…~JRwbSWEhNjJHbYtAE9OblA","ok":true}]  exit 0
```

Final `/my-files`: 5 rows. Row-set comparison against the step-1 baseline (uid|name|type|parentUid):

```
baseline count: 5 ; final count: 5
MISSING from final (baseline rows gone): NONE
EXTRA in final (rows not in baseline):   NONE
ROW-SET EQUAL: PASS - account restored to the precondition baseline
```

No `trashTime` on any surviving row. All four untouchables present and unmodified.

### Trash tally

Runner-issued trash operations: **4** — `junk-a` (during the smoke), `cli080-cert`,
`cli080-cert-contract`, `GitRemotes`.

Live nodes removed from the account tree by those operations: **19** — **8 folders**
(`cli080-cert`, `refs`, `refs/heads`, `refs/heads/feature`, `refs/tags`, `packs`,
`cli080-cert-contract`, `GitRemotes`) and **11 files** (`gpb-remote.json`, `refs/heads/main`,
`refs/heads/feature/x`, 6 pack/idx files, `HEAD`, `junk-a`). Folders are counted, per
checklist rule 7.

Additionally, `TestContractCLI`'s own `t.Cleanup` trashed **17 per-test root folders** plus
their fixture contents during the table run (each row gets its own root; all 17 cleanups
succeeded — no `CLEANUP FAILED` line appears in the transcript, and the contract root listed
empty afterwards).

Everything trashed is in Proton's trash and restorable; the trash was **not** emptied by the
runner.

### Final state checks (2026-08-13T20:28:53-07:00)

- Credential metadata: **unchanged** — identical at every one of the run's watch points.
- `\ProjectsBackupDaily`: **still Disabled** — the runner did NOT re-enable it. **This is
  Craig's step.**
- Machine CLI: swapped to 0.8.0, verified, fleet-verify green.

**CERTIFICATION: PASS** (provisional until Task 3's release gate).
