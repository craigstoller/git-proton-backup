# Proton CLI 0.9.0 certification — record

Plan: `docs/superpowers/plans/2026-10-10-cli-0.9.0-certification.md`. Executed natively (superpowers:executing-plans) on 2026-10-10 by the session with Craig present; the 0.8.0 record (`cli-0.8.0-certification.md`) is the precedent this one follows. Timestamps Pacific (UTC-07:00). Branch `worktree-cli-090-cert`; frozen candidate `203a8bf8bd53b84d8f60a38f7e3d427b19dc8d2c`.

**Outcome:** Task 1 (hermetic) complete; Task 2 (live) **PASS**; Task 3 (release gate) pending Craig's kickoff. The machine runs CLI 0.9.0 since 14:59 PT; the installed helper (v0.8.1) refuses it until the release installs.

---

## Task 1 — Hermetic (13:18–14:10 PT; evidence file `.superpowers/cli-090-cert/task1-audit.md`, untracked)

- **Kickoff (rulings, ledgered):** quiet by the schedule-window fallback (today's 12:30 run finished with result 0; next 10/11 12:30; Craig present); telemetry: proceed under the plan's recommended default. The quiet-check regex also matched the pre-commit `SecretGuardWorker.ps1` (install dir `%LOCALAPPDATA%\ProjectsBackup\io`) — not a backup run; narrowed for the brief.
- **Credential watch baseline: ABSENT.** No Proton entry in Credential Manager; the installed 0.8.0 answered `You need to login first`. Dated from the CLI's own log: at 2026-10-10T02:22:10–11Z (10/9 19:22 PT), `Refreshing session` → `ERROR [cli] Failed to refresh session` → `Removing session auth-session from secrets`; every start since logged `No session loaded`, and the 10/10 12:30 fleet verify failed on all 38 repos (`verify.log`: `exit=1 findings=42`). Pre-existing, not caused by this work (its first CLI call was 11:40 PT). Craig re-authenticated (`auth login`) at ~13:45 PT; the fresh kickoff verify at 14:14 PT was 38/38 ok.
- **State snapshot** `C:\gpb-cert\cli090\state-snapshot-task1\` before any staged execution: 11 files (no `-shm`/`-wal`; a new `events.lock`); `clientUid.json` byte-identical to August's. Six proton-ish directories, the August set.
- **Staging:** feed re-fetched — one release, `0.9.0`, `ReleaseDate 2026-10-05`, 10 platform files, `Platform` key spelled `windows/x64`; windows/x64 SHA-512 equal to the plan's copy. Downloaded in 8 s to `C:\Users\craig\Tools\archive\proton-drive-cli\0.9.0-staged\proton-drive.exe`: 101,469,184 B, SHA-512 `c68f2725b21bd8eddf83b816a059beeab4f1d2f0bb459e6bb53d1b1b972ea971e9d9f191d9cb2aa41f6560c04936aa60f28bb6382191cb0e4e51dd41d36c8bed`, SHA-256 `33e2bc6f6be313ea2ed275e2ef37670d1a80e30a86425bcff4cafe40bd22c066`. The 0.8.0 URL still answered HTTP 200 / 121,838,592 B on 2026-10-10.
- **Captures (unauthenticated):** `--version` → `Proton Drive CLI cli-drive@0.9.0+75612299` / `Proton Drive SDK js@0.22.2+75612299` / `You are running the latest version.` (1.11 s; `version` identical, 0.66 s). **Token: `cli-drive@0.9.0+75612299`.** `filesystem upload -h`: `-f` ∈ `create-new-revision | rename | replace | skip`, `-d` ∈ `merge | rename | replace | skip`, both optional, `-t --skip-thumbnails` new; `filesystem download -h`: `-f` ∈ `rename | remove | skip`, `-d` ∈ `merge | rename | remove | skip`, both optional; `list`/`info`/`create-folder`/`trash` grammar unchanged; new commands `filesystem size` and `takeout run`. Process tree during `--version`: zero children. **Offline `--version`** with a closed-port proxy: exit 0, 0.31 s, exactly the two identity lines — proxy honoured, effective measurement.
- **Post-audit:** state diff vs the snapshot — only `Logs\proton-drive.log` changed (init-class); credential watch unchanged (absent); no migration.
- **Hypotheses after Task 1:** H1 and H3 option surfaces AUDIT-CONFIRMED; H4 AUDIT-CONFIRMED; H2, H5 pending the live table (final statuses at the end of this record).
- **Code (TDD, `-count=1`), commit `a9f4a06`:** token-literal grep confirmed only `cli.go`, `cli_test.go`, `testcli.go` carry token/SDK literals. RED against the 0.8.0 constant: `TestIsCertifiedRefuses080AfterTheBump` FAIL (cli_test.go:1145), `TestEnforceCertifiedOn080RefusesByNameAndOverrideStaysGeneric` FAIL (cli_test.go:1163, `t.Fatal("0.8.0 must refuse without the override")`); GUARDs `TestIsCertifiedTolerantOfTheRealLatestVersionLine` / `…TheRealUpdateNag` PASS before and after. GREEN after `CertifiedCLI = "cli-drive@0.9.0+75612299"`: 11 certification tests PASS; `go test ./... -count=1` 6/6 packages ok; vet 0; gofmt clean. Deliberate regression: both RED tests fail again with the constant reverted; restored. No strategy change (`create-new-revision` / `skip` kept; shim untouched). The 0.7.0 constant's stale "PREVIOUSLY-certified" label re-worded; the older synthetic nag GUARD labelled as synthetic.
- **Docs and the drift check, commit `a0aebdc`:** `scripts/Test-ProtonCliDrift.ps1` (three legs: installed CLI bytes vs feed; installed CLI token vs the helper's certified token; feed version vs certified semver; exit 0/1/2), parser 0 errors, PSSA 0 findings; `docs/releasing.md` steps 3, 5 and 6 amended in place plus a new "Between releases" section (weekly, Craig-owned; first triage step; the in-flight window named); `docs/README.md` indexed. Self-tests: installed pair → `MISMATCH (hash False; token True; version False)`; archived 0.8.0 vs the 0.9.0-certifying build → token `False` (the discriminating run); staged 0.9.0 vs that build → `MATCH`.
- **Final whole-branch review (fresh reviewer, 14:00 PT):** no Critical; three Important, all fixed in commit `203a8bf` with RED→GREEN evidence — the override NOTE no longer calls 0.7.0 "the previously-certified build" (assertion added to `TestEnforceCertifiedOverrideNamesTheKnown070Incompatibility`); the drift script now STOPs on a non-zero `--version` exit from either binary (`tests/DriftCheck.Tests.ps1`, five hermetic cases in a child pwsh with the feed shadowed, RED on the two exit-code cases → GREEN); `docs/releasing.md` steps 3 and 6 stand alone for a release that does not move the pin. Verification after the pass: Go suite 6/6; Pester whole suite 192/192; PSSA clean. Ten Minor findings deferred (ledger). Reviewer's declined-to-judge items ruled (ledger).
- **Task 1 exit:** state diff init-class; credential watch absent/unchanged; installed CLI and helper untouched; scheduler never disabled.

---

## Task 2 — Live certification (runner log)

Appended as work proceeded. Plan: `docs/superpowers/plans/2026-10-10-cli-0.9.0-certification.md` Task 2. Brief: `task2-brief.md` (same dir). Timestamps Pacific (UTC-07:00). Executor: native session (Craig present), 2026-10-10.

---

## Step 0 — Craig's kickoff (13:48–14:25 PT)

- **Session restored by Craig** (`proton-drive auth login` on the installed 0.8.0) after the CLI deleted its own session on 2026-10-09 19:22 PT (Task 1 record, step 0.1). Credential watch **baseline PRESENT**: `LegacyGeneric:target=ch.proton.drive/drive-sdk-cli/auth-session`, Type Generic, User auth-session.
- **Quiet:** `ProjectsBackupDaily` Scheduled Task State Enabled, Status Ready (not Running), Last Run 10/10/2026 12:30:01 PM result 0, Next Run 10/11/2026 12:30:00 PM; no backup worker process (narrowed regex `backup-projects|ProjectsBackup\.psm1|Invoke-ProtonBackup`); no `proton-drive` process. **Ruling (ledgered):** proceeding under a verified schedule window (next run 22 h away; quiet re-verified before the swap); Craig asked to disable the task anyway and to confirm the trash was emptied (rule 6) — neither confirmed at kickoff.
- **Machine CLI untouched:** `Proton Drive CLI cli-drive@0.8.0+06e8c605`.
- **Fleet-verify baseline (Ruling, ledgered: a fresh run instead of the outage's `last-verify.json`):** fresh `-NoProfile` session, module 0.8.1, `Invoke-ProtonBackupVerify` → ExitCode 0, Complete True, 0 findings, **38/38 repos ok**, 1468 s; `last-verify.json` Timestamp 2026-10-10 21:14:17Z. Its writes: the bundles it cut under `GitBackups` (authorised).
- **No `proton::` remote in production use:** 0 of the 38 fleet repos' `.git/config` files contain `proton::`.
- **`/my-files` baseline, six fields (`task2-step0-my-files.json`), 6 rows, no `trashTime`:**

  | uid (suffix) | name | type | creationTime | modificationTime |
  |---|---|---|---|---|
  | ~ThDi8B_92zkL76UXhjqFng | ChatGPT Export Text Backup | folder | 5/18/2026 02:36:23 | 5/18/2026 02:36:23 |
  | ~n94X-kyGohebE_FBtxA5Sg | Sensitive Project Sources | folder | 5/22/2026 15:12:18 | 5/22/2026 15:12:18 |
  | ~5QY-B6t9Eui6VKsXvJPmuA | Project Repo Bundles | folder | 6/22/2026 16:25:42 | 6/22/2026 16:25:42 |
  | ~Vho9hzKVaqnBLf7UnwC64w | GitBackups | folder | 7/21/2026 00:29:59 | 7/21/2026 00:29:59 |
  | ~TfvH3TrgNW1Eyo6Urs7hmQ | _cas-probe | folder | 8/9/2026 21:51:50 | 8/9/2026 21:51:50 |
  | ~GJ9jmpAqIRXr2BzYf2uLeQ | Tax Documents | folder | 9/19/2026 06:19:16 | 9/19/2026 06:19:16 |

  All parentUid = the root `~iMF_ohkUGz7d0J77g_Lb4g`. **`Tax Documents` is new since August and is foreign data — treated as a fifth untouchable.** Observation that settles the plan's open question: `GitBackups`'s `modificationTime` (7/21) equals its `creationTime` although the daily run writes bundles inside it every day — Proton does NOT move a folder's `modificationTime` when its children change, so the six-field comparison will not see the fleet verify's writes or this run's children under `GitRemotes`.
- **`/my-files/GitRemotes`: ABSENT** (`filesystem info` → exit 1, `Node not found: GitRemotes`). This run creates it (two-argument create-folder, authorised) and trashes it at cleanup.
- **`/my-files/_cas-probe` baseline (`task2-step0-cas-probe.json`):** 1 row — `contract` folder, uid `~XKaBzfX59XTZmdI8dNfJVw`, created 8/9/2026 21:52:16 (the Stage 6 gate's default-root leaving), no `trashTime`. The step-3 "after" listing compares against this.
- `last-verify.json` attention set at baseline: empty.

## Step 1 — Freeze the candidate (14:20 PT)

| | |
|---|---|
| Helper commit under test (branch `worktree-cli-090-cert`) | `203a8bf8bd53b84d8f60a38f7e3d427b19dc8d2c` — a19547c plan → a9f4a06 token bump → a0aebdc drift check → 203a8bf final-review fix pass; `git status --porcelain` only `?? .superpowers/` |
| Staged 0.9.0 SHA-512 | `c68f2725b21bd8eddf83b816a059beeab4f1d2f0bb459e6bb53d1b1b972ea971e9d9f191d9cb2aa41f6560c04936aa60f28bb6382191cb0e4e51dd41d36c8bed` (101,469,184 B) |
| Archived 0.8.0 SHA-512 | `03ce039618212c4cb9f175e3d53889657484cee9bfb0b1a24ad3f5b5e703ccf5784119acfa44c65bdda2af2b273c11b91075424ac169824b28edac49399b5a9a` (121,838,592 B) |
| Installed CLI == archived 0.8.0 | True (rollback source byte-verified before anything is overwritten) |
| version.json re-fetch | Version 0.9.0, ReleaseDate 2026-10-05; windows/x64 SHA-512 == staged: True — **no drift** |
| Candidate helper | `C:\gpb-cert\cli090\helper\git-remote-proton.exe` built from HEAD; `--version` → `git-remote-proton dev (certified CLI: cli-drive@0.9.0+75612299)`; SHA-256 `80c370b43dba145a1726e9b6fe1b6da7241872a076a6c3df3964ca676245138f` (5,572,608 B) |

**Freeze PASS.** Any code change from here restarts Task 1's suite and the affected live steps.

## Step 2 — State refresh and first authenticated staged run (14:22 PT)

- Snapshot `C:\gpb-cert\cli090\state-snapshot-task2\` (robocopy exit 1 = copied): **15 files** (the two cache DBs now carry `-shm`/`-wal` after the fleet verify; `events.lock` present); manifest `state-snapshot-task2-manifest.txt`. **This snapshot is the restore point from here on.**
- Credential watch PRE: entry present.
- Staged shell: `proton-drive` resolves to `…\0.9.0-staged\proton-drive.exe`; `GPB_UNCERTIFIED_CLI` unset; `--version` → `cli-drive@0.9.0+75612299` / `js@0.22.2+75612299` / "You are running the latest version."
- 14:22:08 `filesystem info /my-files/GitRemotes --json` → exit 1, `Node not found: GitRemotes` (1.86 s) — absent, as step 0 found.
- 14:22:10 `filesystem create-folder /my-files GitRemotes --json` → exit 0 (2.13 s): uid `tU-Ot1Sq63NwBcxlnl7IcA~IoeTXJ8VDDQbMxkq5KNZnA`, parent the root, type folder, creationTime 2026-10-10T21:22:12.644Z. Re-`info` → exit 0, same uid (3.17 s). **SESSION PORTABILITY CONFIRMED** — the staged 0.9.0 made an authenticated, mutating call on the existing session with no re-auth and no prompt (the STOP class stays defined but did not fire).
- Credential watch POST: unchanged.
- State diff vs the task2 snapshot: CHANGED `Cache\cache-crypto.sqlite` (+`-shm`, `-wal`), `Cache\cache-entities.sqlite` (+`-shm`, `-wal`), `Logs\proton-drive.log`; SAME `Data\clientUid.json`, `Data\events.json`, `Data\events.lock`, logs `.1`–`.5`; zero new, zero missing; top level still `Cache, Data, Logs`; six proton-ish directories (unchanged). **Init-class delta — the CLI's own session refresh, not a migration. No STOP.**

## Step 3 — Contract table live (14:23–14:32 PT; transcript `task2-contract-run.log`)

- **Fail-closed demonstration (zero writes):** with `GPB_CONTRACT_LIVE_ROOT=/my-files/Sensitive Project Sources/x` the table refused in 0.00 s before any live call: `contract_test.go:606: GPB_CONTRACT_LIVE_ROOT must not be inside the untouchable folder "Sensitive Project Sources"`, go test exit 1 after 2 s.
- **Override proven positively:** first log line, written from the same shell as `go test`: `GPB_CONTRACT_LIVE_ROOT=/my-files/GitRemotes/cli090-cert-contract-a1  proton-drive=…\0.9.0-staged\proton-drive.exe  GPB_UNCERTIFIED_CLI set? False  HEAD=203a8bf8…`; `_cas-probe` mentions in the log: 0. After the run: `filesystem list /my-files/GitRemotes/cli090-cert-contract-a1` → exit 0, **0 rows** (the base root exists, created by the table's `EnsureDir(base)`; every per-test root trashed by `t.Cleanup`); `/my-files/_cas-probe` after → the same single `contract` row (uid `~XKaBzfX59XTZmdI8dNfJVw`, 8/9/2026 21:52:16, no `trashTime`) as step 0. The default root was never reached.
- **RESULT: all 17 rows PASS**, `go test exit=0`, **545.6 s** (August on 0.8.0: 605 s). Per-row times 18.7–49.5 s; F1 (`download_of_a_directory_recursively_materialises_the_subtree`) 49.46 s PASS → **H3 directory download LIVE-PINNED**; `nested_list…correct_IsDir` and the stat rows PASS → **H5 node shapes LIVE-PINNED**.
- **Signature re-pins captured live against 0.9.0, byte-identical to the 0.8.0 captures:**
  ```
  contract_test.go:187: live not-found output (must contain notFoundSignature "Node not found"): "Node not found: definitely-absent-t4-notfound-signature\n"
  contract_test.go:322: live directory-download output (F1): "Transfer summary:\n  Downloaded: 4 items (2 B)\n"
  contract_test.go:482: live already-exists output (must contain alreadyExistsSignature "already exists"): "A file or folder with that name already exists\n"
  ```
- The table's own `Version()`/`IsCertified` probe accepted the staged binary (it would have refused before any row otherwise). Credential watch after: unchanged. Env after cleanup: `LIVE='' ROOT=''`.

**Step 3 PASS.**

## Step 4 — Smoke, cross-version reads, update-path invariants, probes

### 4.0 — Forward cross-version read (14:33–14:37 PT): 0.8.0 writes, 0.9.0 reads — PASS

- Shell A (no prepend): `git-remote-proton` → `%LOCALAPPDATA%\Programs\git-proton-backup\git-remote-proton.exe` (`v0.8.1 (certified CLI: cli-drive@0.8.0+06e8c605)`); `proton-drive` → `C:\Users\craig\Tools\proton-drive.exe` (`cli-drive@0.8.0+06e8c605`). `smoke-a1` first commit `cdcc83b814ac04d81a82f9a71e7d04500b210271`.
- Bootstrap push `proton-xver main` → `proton::/my-files/GitRemotes/cli090-xver-a1`: exit 0, **192.1 s**, `* [new branch] main -> main`; the 0.8.1 helper's trace lines show the lock, ref and HEAD downloads.
- Shell B (staged): `git-remote-proton` → `C:\gpb-cert\cli090\helper\git-remote-proton.exe` (`dev (certified CLI: cli-drive@0.9.0+75612299)`); `proton-drive` → the staged exe (`cli-drive@0.9.0+75612299`).
- `git clone -o proton-v2 proton::/my-files/GitRemotes/cli090-xver-a1 C:\gpb-cert\cli090\xver-a1`: exit 0, **63.1 s** (marker, ref, HEAD, one pack/idx pair `pack-001c6aa1…`); `git fsck --full` exit 0; clone `main` = `cdcc83b8…` **equals the pushed commit**. 0.9.0 reads what 0.8.0 wrote. Credential watch unchanged.

### 4.1–4.7 — Smoke and update-path invariants in the staged shell (14:38–14:51 PT) — PASS

Shell B throughout: `git-remote-proton` → the candidate (`dev (certified CLI: cli-drive@0.9.0+75612299)`), `proton-drive` → the staged 0.9.0, `GPB_UNCERTIFIED_CLI` unset.

- **4.1 bootstrap push** `smoke-a1` → `proton::/my-files/GitRemotes/cli090-cert-a1`: exit 0, **158 s**, `* [new branch] main -> main`. **The real allowlist evidence:** `EnforceCertified` ran against the staged 0.9.0 and did not refuse.
- **4.2 BEFORE** (`task2-4.2-before-main.json`): node uid `tU-Ot1Sq63NwBcxlnl7IcA~Dh9CVKZvo9uIe6WR753J_g`, `activeRevision.uid` `…~nC5y3crz49Y7Gof3SXYMcg`, claimedSize 41, active `storageSize` 119, `totalStorageSize` 119; download (5 s) read-back 41 bytes = `cdcc83b8…0271` + `\n` (equals the first commit); `refs/heads` exactly one row (`main`); root rows `gpb-remote.json`=…82hq5Q_0Ng, `refs`=…N14hc0B-HQ, `packs`=…w6PqKpBUUQ, `HEAD`=…lmP3hA4wfg.
- **4.3 H2 raw captures (verbatim; `task2-4.3a/b-*.json`):** absent name → `{"transferredItems":1,"transferredBytes":8,"skippedItems":0,"failedItems":0,"failures":[]}` (3.8 s); occupied → `{"transferredItems":0,"transferredBytes":0,"skippedItems":1,"failedItems":0,"failures":[]}` (6.6 s). All three counter keys present, `failures` empty. **H2 → LIVE-PINNED.** `h2-new` verified (type file, name h2-new, uid `…~aFcQwlZYrdxBS6ohXFdR_A`) and trashed (`ok:true`).
- **4.4 update push** (second commit `df5e9d59f37cebefb5e3569806aa36065c7a5e07`): exit 0, **144.8 s**, `cdcc83b..df5e9d5 main -> main`.
- **4.5 AFTER** (`task2-4.5-after-main.json`) and invariants: node uid **identical** (STABLE IDENTITY: True); `activeRevision.uid` → `…~4gxGfLeeI-eSFiTi6sqjGw` (changed: True); `totalStorageSize` **119 → 238** with active `storageSize` still 119 — consistent with exactly one retained revision (the count is inferred from size; the CLI exposes no revision list); read-back 41 bytes = `df5e9d59…5e07` + `\n` (equals the second commit); `refs/heads` still exactly one row named `main`; sibling uids (`gpb-remote.json`, `refs`, `packs`, `HEAD`) unchanged (True). The plan's architectural STOP (uid change) did not fire.
- **4.6 hierarchical:** `feature/x` pushed (exit 0, 151 s; `refs/heads/feature/x` written); `git ls-remote proton-v2` exit 0 (47.5 s), three lines; `git clone -o proton-v2 … clone1-a1` exit 0 (83.6 s; two pack/idx pairs); `git fetch proton-v2` in the clone exit 0 (47.6 s); `proton-v2/main` and `proton-v2/feature/x` both `df5e9d59…` = the pushed shas (the branch was cut at `main`, so the two shas coincide by construction — a weaker discriminator than August's distinct shas; the nested ref name is what this step pins, and it round-tripped).
- **4.7 strict-fetch junk check:** fixture 41 `x` bytes named `junk-a`; absence pre-check `Node not found: junk-a` (exit 1); upload `-f skip --json` → `transferredItems:1, transferredBytes:41` (7.3 s); `git ls-remote proton-v2` → **exit 128** (39.4 s) with, verbatim (`task2-4.7-ls-remote-refused.txt`):
  ```
  git-remote-proton: skipping /my-files/GitRemotes/cli090-cert-a1/refs/heads/junk-a: not a ref: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
  git-remote-proton: cannot serve a fetch: 1 file(s) under refs/ are not valid refs and a restore would silently lack them:
    /my-files/GitRemotes/cli090-cert-a1/refs/heads/junk-a: not a ref: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
  delete these files first (proton-drive filesystem trash <path>, or the web UI; Proton trash keeps them restorable), then retry
  ```
  — the header, per-file reason (41-byte `%q` preview, no truncation suffix) and remediation sentence match the brief's quotes from `main.go` and `refs.go` character for character; the junk was downloaded (41 bytes), not size-skipped. Verify-before-trash: type file, name junk-a, uid `…~d2pfG3-FNAKDF3lPIxviXw`; trashed `ok:true`; `ls-remote` exit 0 again with the same three lines.
- Credential watch after 4.7: unchanged.

### 4.8 — Case-variant probe (14:52–14:58 PT) — observed, recorded as a known limit (CLI) and a pass (helper)

- `smoke-casevar-a1` bootstrap-pushed to `proton::/my-files/GitRemotes/cli090-casevar-a1` (exit 0, 157.6 s; commit `e2778073fb58ee68ddcc14e3f478c351d4a12418`). A 41-byte valid-sha file named `Main` uploaded into `…/cli090-casevar-a1/refs/heads` (`transferredItems:1`, 4.9 s): **Proton accepted it beside `main`** — the listing shows `'main'[file], 'Main'[file]` (remote names are exact-byte, A5). The hazard exists on the server side.
- Runner defect, disclosed: the first attempt at the two bounded probes launched both processes with no arguments (a helper function whose parameter was named `$args`, PowerShell's reserved automatic variable), so the CLI printed its interactive prompt (exit 2) and git printed usage (exit 1); re-run with correct argument passing against the same fixture (`bounded-4.8-*.txt`).
- **(i) CLI directory download** `filesystem download /my-files/GitRemotes/cli090-casevar-a1/refs/heads C:\gpb-cert\cli090\casevar-dl2` (no strategy flags, 120 s bound): **exit 0 in 4.5 s — no hang** (0.8.0's known hang is gone); output `Transfer summary: Downloaded: 3 items (82 B)`, but **only ONE local file landed: `heads\main` (41 B)** — the second case variant was silently collapsed onto the first on NTFS (82 B = both files counted as downloaded). No ` (1)` rename occurred without a strategy. **Known limit of the CLI's directory download on a case-insensitive filesystem: silent loss of a case-variant sibling.**
- **(ii) Helper `git ls-remote proton-v2`** on that remote (candidate helper + staged 0.9.0, 120 s bound): **exit 0 in 42.1 s, both refs advertised** — `refs/heads/main`, `refs/heads/Main`, `HEAD`, all `e2778073…`; the trace shows the helper downloaded `main` and `Main` individually (per-file reads into their own temp dirs), so it never meets the directory-download collapse. **No silent loss, no hang, no refusal at the helper's advertisement.** (A clone of such a remote onto NTFS would then fail inside git, which cannot hold two refs differing only by case — outside the helper.) Feeds README Honest limits only as the CLI-side note; no helper change.

### 4.9 — Reverse cross-version read (14:54–14:56 PT) — PASS

Shell C: `git-remote-proton` → the installed v0.8.1 (certifies 0.8.0), `proton-drive` → the ARCHIVED 0.8.0. `git clone -o proton-v2 proton::/my-files/GitRemotes/cli090-cert-a1 C:\gpb-cert\cli090\clone-back-a1` → exit 0, **90.4 s** (marker, both refs, HEAD, both pack/idx pairs `pack-001c6aa1…`, `pack-e0348d32…`); `git fsck --full` exit 0; `proton-v2/main` and `proton-v2/feature/x` = `df5e9d59…` = the shas 0.9.0 pushed. **0.8.0 reads what 0.9.0 wrote; rollback after new writes is known to work.** The STOP class did not fire.

### 4.10 — Concurrent-startup probe (14:56 PT) — one trial, collision reproduced (informational)

Two `filesystem list /my-files --json` with the staged 0.9.0 launched within the same second: **A exit 1, B exit 0** (B returned the 7-row listing). A's output (`bounded-4.10-a.out.txt`): `SQLiteError: database is locked — errno: 261, code: "SQLITE_BUSY_RECOVERY"` at `new SQLiteCache (src/cache/sqliteCache.ts:12)` on `PRAGMA journal_mode = WAL` — the Stage 1 finding, byte-for-byte the same signature on the new runtime. **Not observed-absent; observed-present, in one two-process trial.** No code change (the helper serialises its own CLI calls); a retry-with-backoff at CLI startup remains the Stage 7 candidate the design doc names. Credential watch after 4.8–4.10: unchanged.

## Step 5 — Machine swap (14:59–15:19 PT) — PASS

- **Pre-swap gates (14:59:28):** `ProjectsBackupDaily` Enabled / Status Ready (not Running); backup workers 0; `proton-drive` processes 0. Staged SHA-512 == expected `c68f2725…`: True; archived 0.8.0 == expected `03ce0396…`: True; **installed == archived (rollback source byte-verified immediately before overwrite): True.**
- **Swap (14:59:31):** `Copy-Item` staged → `C:\Users\craig\Tools\proton-drive.exe`. Installed AFTER: SHA-512 `c68f2725b21bd8eddf83b816a059beeab4f1d2f0bb459e6bb53d1b1b972ea971e9d9f191d9cb2aa41f6560c04936aa60f28bb6382191cb0e4e51dd41d36c8bed` — **INSTALLED == STAGED (512): True, (256): True**, size 101,469,184.
- **Post-swap, plain PATH (14:59:33):** `proton-drive` resolves to the installed path; `--version` → `Proton Drive CLI cli-drive@0.9.0+75612299 | Proton Drive SDK js@0.22.2+75612299 | You are running the latest version.`, exit 0. Auth probe `filesystem list /my-files` → exit 0, 3.1 s, **7 rows** = the six baseline folders + this run's `GitRemotes` (session carried across the swap; no re-auth). Credential watch unchanged. **No rollback trigger.**
- **Fleet verify (14:59:37–15:19:17; the v1 MODULE, no allowlist, fresh `-NoProfile` session, no `GPB_*` overrides, `proton-drive` on PATH = the swapped 0.9.0):** `ExitCode=0 Complete=True IncompleteReason='' Findings=0`, **1180 s**; `last-verify.json` Timestamp 2026-10-10 22:19:17Z, **38/38 repos ok, attention 0** — equals the step-0 baseline set (38 ok, none in attention). Its writes: the bundles it cut under `GitBackups`, as the daily run makes them.

**SWAP VERDICT: PASS — the machine is on CLI 0.9.0, fleet verify green.** Surface map reminder: the INSTALLED v2 helper (v0.8.1, certifies 0.8.0) now refuses the machine CLI until Task 3 step 4 installs this release — a known window; step 0 confirmed no `proton::` remote is in production use.

## Step 6 — Cleanup, row-set comparison, exit checklist (15:19–15:21 PT) — PASS

- **Full subtree enumeration of `/my-files/GitRemotes` (15:19:55, installed = swapped 0.9.0):** 17 folders, 19 files, **0 foreign nodes**, no `trashTime` on any row. Per root: `cli090-cert-contract-a1` (empty — the table's `t.Cleanup` trashed all 17 per-test roots); `cli090-xver-a1` (`gpb-remote.json` 42 B, `refs/heads/main` 41 B, `refs/tags/` empty, `packs/` one pack/idx pair `pack-001c6aa1…` 385/1128 B, `HEAD` 21 B); `cli090-cert-a1` (marker, `refs/heads/main`, `refs/heads/feature/x`, `refs/tags/` empty, two pack/idx pairs `pack-001c6aa1…` and `pack-e0348d32…`, `HEAD`); `cli090-casevar-a1` (marker, `refs/heads/main` AND `refs/heads/Main`, `refs/tags/` empty, one pair `pack-b4efaad2…`, `HEAD`). The three "unlistable" lines in the raw capture are the empty folders (`[  ]`, the C16 empty-listing shape), not errors.
- **Trash, in order (15:21:08–15:21:29), each `ok:true`:** `cli090-xver-a1` (uid `…~tqvetxGf_SIEJ180QTDzdw`, 2.2 s), `cli090-cert-a1` (`…~lVocIOa-0wewC5BvVvITTg`, 5.3 s), `cli090-casevar-a1` (`…~jwTAgHx6S7YVsuIjUjDYOQ`, 5.4 s), `cli090-cert-contract-a1` (`…~40m_P3bYfrcOriO7z6CQUw`, 3.5 s); `/my-files/GitRemotes` listed **0 live rows on the first listing — no trash lag**; then `GitRemotes` itself (`…~IoeTXJ8VDDQbMxkq5KNZnA` — the uid this run created at 14:22, 5 s). Nothing pre-existing was trashed.
- **Final `/my-files` row set vs step 0, all six fields (`task2-step6-my-files-final.json`):** baseline 6, final 6, MISSING NONE, EXTRA NONE — **ROW-SET EQUAL: True** (the two permitted timestamp differences did not occur: Proton did not move `GitBackups`'s `modificationTime` for the fleet verify's bundle writes, as step 0 predicted). `trashTime` on any final row: 0. `/my-files/_cas-probe` → still the single `contract` row (`…mdI8dNfJVw`, 2026-08-09 21:52:16), unchanged.
- **Trash tally (rule 7):** this step — **18 folders** (the four roots' 17 + `GitRemotes`) and **19 files**; step 4 — the `h2-new` and `junk-a` files (2); step 3 — the table's own 17 per-test root folders and their fixtures (`t.Cleanup`, all succeeded; the contract root listed empty afterwards). Everything is in Proton's trash and restorable; the runner did NOT empty the trash.
- **Exit readings (15:21:42):** credential watch unchanged (entry present, same target/type/user at every one of the run's watch points); state diff vs the task2 snapshot — the init-class set only (both cache DBs with `-shm`/`-wal`, the log), zero new, zero missing; scheduler `Scheduled Task State: Enabled`, `Status: Ready`, `Next Run Time 10/11/2026 12:30` — never disabled this run (schedule-window ruling), so **nothing for Craig to re-enable**; machine CLI `Proton Drive CLI cli-drive@0.9.0+75612299`; installed helper `git-remote-proton v0.8.1 (certified CLI: cli-drive@0.8.0+06e8c605)` — **the known refusal window is open on this machine until Task 3 step 4 installs the release** (no production `proton::` remote; the daily backup runs the v1 module and is unaffected — the post-swap fleet verify proved it).

## Hypothesis ledger — final statuses

| Hypothesis | Final status | Pinned by |
|---|---|---|
| H1 upload vocabulary unchanged | AUDIT-CONFIRMED (option surface); quota branch SOURCE-ACCEPTED | Task 1 step 0.6; steps 3, 4.1–4.7 (every helper upload ran) |
| H2 transfer-summary shape unchanged | LIVE-PINNED | step 4.3 raw captures (both outcomes, verbatim) |
| H3 download semantics; case-variant | LIVE-PINNED (single file: steps 4.2/4.5; directory F1: table row); case-variant: observed — CLI directory download silently collapses the pair; helper advertises both | steps 3, 4.2, 4.5, 4.8 |
| H4 version command; offline bound | AUDIT-CONFIRMED (0.31 s, two lines) | Task 1 step 0.5 |
| H5 node JSON shapes unchanged | LIVE-PINNED | table rows (C9, stat, nested list) |

## Deviations and rulings during Task 2

- D1 — the kickoff ran under a verified schedule window instead of a Craig-disabled task; neither the disable nor the trash-emptying was confirmed by Craig; quiet was re-verified before the swap and held; no `trashTime` appeared on any listing (ledgered).
- D2 — the fleet-verify baseline was a fresh run at kickoff (the read-only `last-verify.json` held the outage run), which also proved the restored session (ledgered).
- D3 — `Tax Documents` (new, 9/19/2026) treated as a fifth untouchable (ledgered).
- D4 — step 4.8's first probe attempt was a runner defect (no arguments passed); re-run against the same fixture, disclosed above.
- D5 — the helper `--version` prints the compiled constant (0.8.0's D2, unchanged): the allowlist evidence is step 4.1's unrefused push.

**CERTIFICATION: PASS** (provisional until Task 3's release gate). The certification of `cli-drive@0.9.0+75612299`: hermetic audit → 17/17 live contract rows → smoke with stable node identity and one new revision per update → cross-version reads both ways → machine swap + green fleet verify (38/38) → account restored to its baseline.

---

## Appendix — Task 2 brief (as run)


Runner: the native session (Craig present), 2026-10-10. Plan: `docs/superpowers/plans/2026-10-10-cli-0.9.0-certification.md` Task 2. Standing rules: `docs/research/gates/brief-checklist.md`, all nine, by reference.

## Frozen candidate

- Helper commit: `203a8bf8bd53b84d8f60a38f7e3d427b19dc8d2c` (branch `worktree-cli-090-cert`).
- Staged CLI: `C:\Users\craig\Tools\archive\proton-drive-cli\0.9.0-staged\proton-drive.exe`, SHA-512 `c68f2725…6c8bed`, token `cli-drive@0.9.0+75612299`.
- Candidate helper: `C:\gpb-cert\cli090\helper\git-remote-proton.exe`, SHA-256 `80c370b4…138f`.
- Rollback source: archived 0.8.0, SHA-512 `03ce0396…99b5a9a`; the installed CLI is hash-identical to it.

## Confinement (rule 2) and fixture naming

Remote writes only under `/my-files/GitRemotes` (ABSENT at step 0 — created by this run via the two-argument `filesystem create-folder /my-files GitRemotes`, trashed at exit) and its children for attempt `a1`: `cli090-xver-a1`, `cli090-cert-a1`, `cli090-casevar-a1`, `cli090-cert-contract-a1` (= `GPB_CONTRACT_LIVE_ROOT`). The table's compiled-in default `liveRoot` `/my-files/_cas-probe/contract` is named here per rule 8 and must never be reached (step 3 proves it: the override printed from the same shell, `_cas-probe` listed before/after). `GitBackups` receives only the fleet verify's own writes (step 5). **Untouchables, never in any trash argument:** `GitBackups` (beyond that), `Sensitive Project Sources`, `Project Repo Bundles`, `ChatGPT Export Text Backup`, **`Tax Documents`** (new since August), `_cas-probe` and its `contract` child. A re-run after an interruption uses suffix `a2` everywhere (env var, roots, local dirs). Local scratch: `C:\gpb-cert\cli090\` (`smoke-a1`, `junk-a1\junk-a`, `h2-a1\h2-new`, `xver-a1`, `clone1-a1`, `clone-back-a1`); an existing local path is a STOP (stale attempt).

Verify-before-trash with full subtree enumeration (rule 3); row sets on all six fields (rule 1) — observed at step 0 that Proton does not move a folder's `modificationTime` when its children change, so child-driven deltas will not appear; `-count=1` (rule 5) and `-timeout 40m` on the table; pushes with a long harness timeout or backgrounded (rule 9); BLOCKED = verbatim output, never patch (rule 4); trash accounting counts folders (rule 7). Rule 6 (trash emptied): Craig had not confirmed at kickoff — any `trashTime` on a listed row is recorded, never acted on.

## Strings asserted from code at the frozen SHA

- `notFoundSignature = "Node not found"` (`internal/transport/cli.go:168`); `alreadyExistsSignature = "already exists"` (`cli.go:277`) — both `strings.Contains`; the live table logs the full text.
- Strict-fetch enumeration (`cmd/git-remote-proton/main.go:373–379`), emitted through `warn` with the `git-remote-proton: ` prefix: header `cannot serve a fetch: %d file(s) under refs/ are not valid refs and a restore would silently lack them:`, then one line per file `  <root>/<path>: <reason>`, then `delete these files first (proton-drive filesystem trash <path>, or the web UI; Proton trash keeps them restorable), then retry`. For the 41-byte `x` fixture the reason is `not a ref: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"` (`internal/repo/refs.go` `classifyRefContent`: `%q` of the first 42 bytes, no `...(truncated)` suffix at 41 bytes). Git's exit code for the refused fetch: **128** (precedented, Stage 6 and 0.8.0 gates).
- Helper `--version` prints the compiled-in token (`git-remote-proton dev (certified CLI: cli-drive@0.9.0+75612299)`) and is NOT evidence about PATH (0.8.0 deviation D2); the real allowlist evidence is a push that `EnforceCertified` did not refuse in a shell where `proton-drive` resolves to the staged exe.

## Failure policy

Read-only rows: one retry after ~60 s on a clearly transport-classified failure. Mutating rows: never replayed blindly — reconcile the fixture's parent listing, then re-run on the next attempt suffix. One observed post-reconciliation invariant violation BLOCKS for Craig. STOP classes: session non-portability (wholesale auth errors), stable-identity failure (node uid changes under `create-new-revision`), reverse cross-version read failure (step 4.9), any migration-class state delta or credential change — each followed by the Global-constraints restore rule.

## Step outline (wall times recorded on every live command)

2. Snapshot `state-snapshot-task2`; staged shell; `filesystem info /my-files/GitRemotes --json` → create-folder if absent; credential watch; state diff (init-class expected).
3. Contract table: the plan's block, with the override printed into the log from the same shell; after: list the base root; list `_cas-probe` and compare with step 0.
4. 0.0 forward cross-version read (installed helper + 0.8.0 CLI push `cli090-xver-a1`; staged clone `-o proton-v2`) → 4.1 bootstrap push `cli090-cert-a1` → 4.2 BEFORE capture → 4.3 H2 raw captures → 4.4 update push → 4.5 AFTER invariants → 4.6 `feature/x`, ls-remote, clone, fetch → 4.7 junk check (absence pre-check; exit 128; verbatim) → 4.8 case-variant probe (download + ls-remote, no clone, 120 s bounds) → 4.9 reverse cross-version read (archived 0.8.0 + installed helper clone) → 4.10 concurrent-startup probe (one trial, stated as such).
5. Swap (quiet re-verified; hashes; copy; verify; `--version`; auth probe; fleet verify, acceptance vs the step-0 baseline: exit 0, Complete, 38 ok, no new attention).
6. Cleanup (enumerate; trash the four roots then `GitRemotes`); six-field `/my-files` comparison; trash tally; exit checklist; gate record.
