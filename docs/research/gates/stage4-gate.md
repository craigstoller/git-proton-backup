# Stage 4 live gate — DRAFT release v0.3.0 against the real account (steps 1–3, pre-publication)

**Verdict: BLOCKED at Step 1 (sub-step 1.5).** The installed helper is *shadowed* on `PATH` by a
pre-existing, different `git-remote-proton.exe` at `C:\Users\craig\Tools`. The brief's rule is
explicit — "confirm the FIRST hit is `%LOCALAPPDATA%\Programs\git-proton-backup\git-remote-proton.exe`.
Any shadowing = BLOCKED." Steps 2 and 3 were **not run**: every one of their commands resolves the
helper through `PATH` (step 3 does so via `git` itself), so running them would have exercised the
**stale** binary against the live account rather than the release bytes under test. Nothing was
patched, renamed, or deleted to clear the shadow.

**No writes to the Proton account occurred in this run.** The only account commands issued were two
read-only `/my-files` listings, and they are byte-identical to each other.

- **Date:** 2026-08-06
- **Repo:** `C:\Users\craig\Projects\_Tools\git-proton-backup`, branch `main` @ `dc2a6fa13a764b324e45c8c459e18ee64ec18f7a`, working tree clean
- **Release under test:** GitHub **draft** `v0.3.0` (`craigstoller/git-proton-backup`), created `2026-08-06T15:06:22Z`
- **CLI:** `Proton Drive CLI cli-drive@0.7.0+5174900c` / `Proton Drive SDK js@0.20.0+5174900c` (the certified version)
- **git:** `git version 2.53.0.windows.2`
- **Gate directory:** `…\560c121b-07fb-4c49-b229-e8d6f066df0f\scratchpad\stage4-gate` (created empty, 0 entries)
- **Runner:** Stage 4 live gate runner, executing `task-8-gate-brief.md` steps 1–3 in order

> **The publication digest closure (spec step 4) has NOT yet run.** This verdict is provisional
> with respect to that closure in any case; it is currently a BLOCK on independent grounds.

---

## Preconditions

### Environment / auth

```
proton-drive --version
```

```
Proton Drive CLI cli-drive@0.7.0+5174900c
Proton Drive SDK js@0.20.0+5174900c
```

The pre-run `/my-files` listing below doubles as the auth check; it succeeded.

### Trash state

Craig reports the trash was emptied via the web UI on 2026-08-06 immediately before this run. The
CLI cannot enumerate trash (C17b S3), so this is **taken on report, not verified** by this gate.
No trash operation of any kind was performed in this run.

### Pre-run `/my-files` listing (recorded before anything else ran)

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

Exactly the four untouchable folders. **`GitRemotes` did not exist pre-run.**

---

## Step 1 — stage the artifact

### 1.1 Download the draft's assets — PASS

```
gh release list --repo craigstoller/git-proton-backup
```

```
v0.3.0	Draft	v0.3.0	2026-08-06T15:06:22Z
```

```
gh release download v0.3.0 --repo craigstoller/git-proton-backup --dir <gatedir>
```

Exit code 0, no output. Resulting gate-directory contents — **exactly the three expected assets**:

```
Name                          Length
----                          ------
git-remote-proton.exe        3891712
git-remote-proton.exe.sha256      87
install.ps1                     3312
```

### 1.2 Staged digests (the baseline for the post-publication closure) — PASS

```
Get-FileHash -Algorithm SHA256
```

```
089E66B38C27BCCE303C5A13AB5F82AD5FB10B48A60BD18659093979FE0517D4  git-remote-proton.exe         3891712 bytes
7041532DF750B989A5BA73F578F18C6F31D17C75B8AC32E732DF791F81558015  git-remote-proton.exe.sha256       87 bytes
0ECD317AC6C86DCE4895EE5A1DFD3ED557E634FF6C591B15EA70F6862E6C5971  install.ps1                      3312 bytes
```

**These three digests are the staged baseline** that the post-publication digest closure (spec
step 4) must compare the published assets against.

### 1.3 Independent sidecar verification — PASS

Sidecar raw content (87 bytes, no trailing newline — confirmed by byte dump):

```
089e66b38c27bcce303c5a13ab5f82ad5fb10b48a60bd18659093979fe0517d4  git-remote-proton.exe
```

```
sidecar token: 089e66b38c27bcce303c5a13ab5f82ad5fb10b48a60bd18659093979fe0517d4
exe hash     : 089e66b38c27bcce303c5a13ab5f82ad5fb10b48a60bd18659093979fe0517d4
MATCH: True
```

### 1.4 Run the installer, no parameters — PASS

Pre-install state, recorded first:

- `%LOCALAPPDATA%\Programs\git-proton-backup` — **NOT PRESENT**
- Repo-root stale gitignored exe — **PRESENT**, sha256 `46E0E3E62DB83C5D90730DE18494C2F3A936B92AB6BB4F7E9EEC4C8152477D96`. **Never run.**
- **`C:\Users\craig\Tools\git-remote-proton.exe` — PRESENT** (see 1.5): 4 513 792 bytes, mtime `08/02/2026 13:49:49`, sha256 `11F256C0BA3809B70B3F5258AA6584E39FC21053809ADD31B53B9EB44E526DC5`
- `C:\Users\craig\Tools` is entry **32** of the *User* PATH. It is **not** in the Machine PATH.

```
pwsh -NoProfile -File <gatedir>\install.ps1
```

```
Added C:\Users\craig\AppData\Local\Programs\git-proton-backup to your user PATH. Open a NEW terminal for it to take effect (this script cannot change its caller's session).
Helper installed: C:\Users\craig\AppData\Local\Programs\git-proton-backup\git-remote-proton.exe
No GitProtonBackup module payload beside the script — helper-only install. For the module, clone the repository and re-run.
```

Exit code **0**. All four expectations met: checksum verification passed (no mismatch throw), the
helper was copied to `%LOCALAPPDATA%\Programs\git-proton-backup\`, the PATH message was emitted,
and **the module block SKIPPED with its helper-only message** rather than throwing. **The Task 6
defect this sub-step was written to catch did not occur.**

The installed copy is byte-identical to the release asset:

```
sha256 of %LOCALAPPDATA%\Programs\git-proton-backup\git-remote-proton.exe
089E66B38C27BCCE303C5A13AB5F82AD5FB10B48A60BD18659093979FE0517D4
```

### 1.5 Fresh-shell resolution and version tag — **BLOCKED**

Fresh shell per the brief's recipe (`Machine` PATH + `;` + `User` PATH — the same composition a
genuinely new terminal performs):

```
(Get-Command git-remote-proton -All).Source
```

```
C:\Users\craig\Tools\git-remote-proton.exe
C:\Users\craig\AppData\Local\Programs\git-proton-backup\git-remote-proton.exe
```

**The FIRST hit is not the installed helper.** This is the brief's stated BLOCK condition.

The consequence is not cosmetic. What a user following the documented install actually gets:

```
git-remote-proton --version        # resolved via PATH
```

```
git-remote-proton: must be run by git as a remote helper
exit=1
```

Whereas the **installed** helper, invoked by absolute path, is correct and matches the expected
tag exactly:

```
& "$env:LOCALAPPDATA\Programs\git-proton-backup\git-remote-proton.exe" --version
```

```
git-remote-proton v0.3.0 (certified CLI: cli-drive@0.7.0+5174900c)
exit=0
```

So the **artifact is good; the PATH hand-off is not**. The shadowing binary is a *different, older*
build (2026-08-02) that predates the `--version` flag entirely.

#### Mechanism

`install.ps1` line 37 **appends** to the user PATH:

```powershell
$newPath = if ([string]::IsNullOrEmpty($userPath)) { $helperDir } else { "$userPath;$helperDir" }
```

Post-install User PATH ordering:

```
32	C:\Users\craig\Tools
33	C:\Users\craig\go\bin
34	c:\users\craig\appdata\roaming\python\python314\scripts
35	C:\Users\craig\AppData\Local\Programs\git-proton-backup
```

`Tools` index 32, install dir index 35 — the installer's own entry can never win. The installer's
idempotence guard (`$entries -notcontains $helperDir`) checks only whether *its own directory* is
on PATH; it does not check whether some *other* directory already supplies a `git-remote-proton`
executable. Consequently, on any machine that has ever had the helper placed elsewhere on PATH,
`install.ps1` reports success while the old binary keeps serving `git`.

---

## Step 2 — allowlist live — NOT RUN (blocked upstream)

Both 2a and 2b invoke `git-remote-proton` **from `PATH` in a fresh shell**. Under the step 1.5
shadow that resolves the stale 2026-08-02 binary, so neither sub-step would test the release bytes:
2a's expected marker refusal and 2b's expected version refusal / `GPB_UNCERTIFIED_CLI` override are
all behaviours of the v0.3.0 helper specifically.

The shim was **not** created, `GPB_UNCERTIFIED_CLI` was **never set**, and `GPB_LIVE_ACCOUNT` was
never set at any point in this run.

## Step 3 — `--set-head` end to end — NOT RUN (blocked upstream)

Step 3 is the more serious case. `git push` / `git clone` locate the helper by searching `PATH` for
`git-remote-<transport>`, so every push, clone and delete in step 3 would have been served by the
**stale** helper — writing to `/my-files/GitRemotes/stage4-gate` on the live account with
pre-release code. That would have contaminated both the gate and the account state while proving
nothing about v0.3.0. The run stopped instead.

No demo repo was created; no remote was added; no push, clone, or branch delete was attempted.

---

## Surprises

**S1 — installed helper shadowed on PATH by a pre-existing copy (classified; product-side finding).**

- *Observed:* after a clean, successful `install.ps1` run, `Get-Command git-remote-proton -All`
  in a fresh shell returns `C:\Users\craig\Tools\git-remote-proton.exe` first and the installed
  helper second.
- *Mechanism:* fully explained above — `install.ps1` appends its directory to the end of the user
  PATH and its idempotence check does not consider foreign copies. Deterministic, not transient.
- *Two components, worth separating:*
  1. **Environment:** `C:\Users\craig\Tools\git-remote-proton.exe` is a leftover from earlier
     stage work (2026-08-02, pre-`--version`). It is outside this gate's write confinement and
     was left exactly as found.
  2. **Product:** append-only PATH placement plus a self-only idempotence check means the
     installer cannot guarantee its own helper wins, and **reports success regardless**. On a
     machine with any prior copy on PATH, a user who follows `install.ps1` silently keeps running
     the old binary — with no warning from the installer and no diagnostic from the release.
- *Not reproduced/unclassifiable elements:* none. The mechanism is understood and reproducible.

No other surprises. No transient network errors were observed; no read was retried; no write was
issued, let alone retried.

---

## Confinement attestation

- **Writes to the Proton account: none.** The only account commands issued in this entire run were
  two read-only `proton-drive filesystem list /my-files --json` invocations. `/my-files/GitRemotes`
  was never created; `/my-files/_cas-probe` was never touched or named in any command.
- **Untouchable folders** (`GitBackups`, `Sensitive Project Sources`, `Project Repo Bundles`,
  `ChatGPT Export Text Backup`) appeared **only** as rows in those two read-only listings. They
  were never named in any command.
- **No `trash`, no `delete`, no `empty-trash`, no `restore`** was executed — not on account nodes,
  and not on the shadowing local binary (which was deliberately left in place rather than moved or
  renamed to clear the block).
- **No write was retried.** No read was retried.
- **Nothing was patched** — not the release assets, not `install.ps1`, not the repo, not the PATH
  ordering, not the account state.
- `GPB_LIVE_ACCOUNT` was never set. `GPB_UNCERTIFIED_CLI` was never set.
- The repo-root stale gitignored `git-remote-proton.exe` was hashed for the record but **never run**.
- Local filesystem writes were confined to the gate directory, plus the installer's own intended
  effects (see "Residual local state").

### Post-run `/my-files` listing

```json
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~5QY-B6t9Eui6VKsXvJPmuA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g","name":{"ok":true,"value":"Project Repo Bundles"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"creationTime":"2026-06-22T16:25:42.000Z","modificationTime":"2026-06-22T16:25:42.000Z","treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~ThDi8B_92zkL76UXhjqFng","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g","name":{"ok":true,"value":"ChatGPT Export Text Backup"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"creationTime":"2026-05-18T02:36:23.000Z","modificationTime":"2026-05-18T02:36:23.000Z","treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Vho9hzKVaqnBLf7UnwC64w","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g","name":{"ok":true,"value":"GitBackups"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"creationTime":"2026-07-21T00:29:59.000Z","modificationTime":"2026-07-21T00:29:59.000Z","treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~n94X-kyGohebE_FBtxA5Sg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g","name":{"ok":true,"value":"Sensitive Project Sources"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"creationTime":"2026-05-22T15:12:18.000Z","modificationTime":"2026-05-22T15:12:18.000Z","treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```

**Byte-identical to the pre-run listing** — same four rows, same uids, same creation and
modification times. No node with a `trashTime` appeared, so the C17b incidental-3 re-list was not
needed. The cleanup step had nothing to clean: `GitRemotes` was never created.

### Residual local state (deliberate, not cleaned)

Left in place for the re-run; none of it touches the account:

1. **The helper is installed** at `%LOCALAPPDATA%\Programs\git-proton-backup\git-remote-proton.exe`
   (v0.3.0 bytes, `089E66B3…`) — the installer's intended effect, not reverted.
2. **User PATH entry added** by the installer — `C:\Users\craig\AppData\Local\Programs\git-proton-backup`
   at user-PATH position 35.
3. **`C:\Users\craig\Tools\git-remote-proton.exe` left exactly as found** — removing it is the
   obvious way to clear the block, and doing so would have been patching the environment to make a
   step pass. **That call belongs to Craig, not to this gate.**
4. Gate directory retained with the three downloaded assets.

---

## Verdict

| Step | Result |
| --- | --- |
| 1.1 download three assets | **PASS** — exactly `git-remote-proton.exe`, `.sha256`, `install.ps1` |
| 1.2 record staged digests | **PASS** — baseline recorded for the step-4 closure |
| 1.3 independent sidecar check | **PASS** — token equals exe hash |
| 1.4 run installer | **PASS** — checksum ok, helper copied, module block skipped (no Task 6 throw) |
| 1.5 fresh-shell PATH + version | **BLOCKED** — installed helper shadowed by `C:\Users\craig\Tools` copy |
| 2 allowlist live | **NOT RUN** — depends on PATH resolution |
| 3 `--set-head` end to end | **NOT RUN** — depends on PATH resolution |
| Cleanup | **N/A** — no account writes; post-run listing identical to pre-run |

### Overall: **BLOCKED at step 1**

The release artifact itself cleared every check that could be made without PATH: correct asset set,
self-consistent checksum, clean install, correct embedded version tag
(`git-remote-proton v0.3.0 (certified CLI: cli-drive@0.7.0+5174900c)`), and the module-skip fix
holding. The block is in the **hand-off from installer to `PATH`**, and it is reproducible.

**The publication digest closure (spec step 4) has NOT yet run.** Any eventual verdict from this
gate remains provisional until it does — and at present the gate is BLOCKED on independent grounds
and has produced no PASS to qualify.

---

## Run 2 — v0.3.1 (2026-08-06)

**Verdict: PROVISIONAL PASS (steps 1–3 all green).** The run-1 blocker is cleared: with the stale
`C:\Users\craig\Tools\git-remote-proton.exe` renamed out of the way, the v0.3.1 installer's new
shadow detection ran and stayed silent, and a fresh shell resolves the installed helper reporting
`git-remote-proton v0.3.1 (certified CLI: cli-drive@0.7.0+5174900c)`. The allowlist refuses an
uncertified CLI and the documented override proceeds. `--set-head` works end to end and is proven
by the decisive postcondition pair: deleting the *old* default (`main`) succeeded, deleting the
*new* default (`alt`) was refused with `--set-head` named as the remedy.

Two surprises occurred, both **environment**, neither a product defect; both are recorded verbatim
below with the deviation each required. **Neither was resolved by patching code, config, or account
state to make a product assertion pass.**

- **Date:** 2026-08-06 (run started 10:57:37 −07:00, cleanup complete 11:19 −07:00)
- **Repo:** `C:\Users\craig\Projects\_Tools\git-proton-backup`, branch `main` @ `3756ec3fbffb24c761e07cb9955c8aa7f8b45cdb`, working tree clean
- **Release under test:** GitHub **draft** `v0.3.1` (`craigstoller/git-proton-backup`), created `2026-08-06T16:04:15Z`, `isDraft: true`, exactly 3 assets
- **CLI:** `Proton Drive CLI cli-drive@0.7.0+5174900c` / `Proton Drive SDK js@0.20.0+5174900c` (the certified version)
- **git:** `git version 2.53.0.windows.2`
- **Gate directory:** `…\560c121b-07fb-4c49-b229-e8d6f066df0f\scratchpad\stage4-gate-r2` (created empty, 0 entries; run 1's `stage4-gate` dir untouched)
- **Runner:** Stage 4 live gate runner, run 2, executing `task-8-gate-brief-r2.md` over `task-8-gate-brief.md`

> **The publication digest closure (spec step 4) has NOT yet run.** This PASS is explicitly
> **provisional** until the published release's per-asset digests are compared against the staged
> digests recorded below.

---

### Preconditions (run 2)

#### Step 0 — shadow precondition (new in run 2)

```
Test-Path C:\Users\craig\Tools\git-remote-proton.exe = False
```

Required **False**; observed **False**. The stale binary is still present but renamed, so it is no
longer named `git-remote-proton.exe` and cannot be resolved by PATH lookup:

```
Name                                            Length LastWriteTime
----                                            ------ -------------
git-remote-proton.exe.stale-dev-2026-08-02.bak 4513792 8/2/2026 1:49:49 PM
```

Pre-install baseline in a fresh shell (PATH rebuilt Machine-then-User from the registry):

```
PATH-resolved git-remote-proton (all hits):
C:\Users\craig\AppData\Local\Programs\git-proton-backup\git-remote-proton.exe
---version---
git-remote-proton v0.3.0 (certified CLI: cli-drive@0.7.0+5174900c)
```

Exactly one hit, and it is the run-1 installed copy at `%LOCALAPPDATA%\Programs\git-proton-backup`.
This is the **expected residual state** from run 1 (brief delta 4), not a shadow.

#### Environment / auth

```
proton-drive --version
Proton Drive CLI cli-drive@0.7.0+5174900c
Proton Drive SDK js@0.20.0+5174900c
(exit 0)
```

Signed in — the pre-run `/my-files` listing below returned data (exit 0), which doubles as the auth
check.

#### Trash state

Craig emptied the trash via the web UI on 2026-08-06 immediately before run 1. Run 1 performed
**zero writes and zero trash operations**, so the trash is **BELIEVED still empty** at the start of
run 2 — taken on that chain of report, **not verified**. The CLI cannot enumerate trash (C17b
incidental find); the web UI is the only route.

#### Pre-run listing (before any write)

```
proton-drive filesystem list /my-files --json
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~5QY-B6t9Eui6VKsXvJPmuA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g","name":{"ok":true,"value":"Project Repo Bundles"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"creationTime":"2026-06-22T16:25:42.000Z","modificationTime":"2026-06-22T16:25:42.000Z","treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~ThDi8B_92zkL76UXhjqFng","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g","name":{"ok":true,"value":"ChatGPT Export Text Backup"},…"creationTime":"2026-05-18T02:36:23.000Z","modificationTime":"2026-05-18T02:36:23.000Z",…},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Vho9hzKVaqnBLf7UnwC64w","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g","name":{"ok":true,"value":"GitBackups"},…"creationTime":"2026-07-21T00:29:59.000Z","modificationTime":"2026-07-21T00:29:59.000Z",…},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~n94X-kyGohebE_FBtxA5Sg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g","name":{"ok":true,"value":"Sensitive Project Sources"},…"creationTime":"2026-05-22T15:12:18.000Z","modificationTime":"2026-05-22T15:12:18.000Z",…}
]
```

Exactly the four standing folders. **Neither `GitRemotes` nor `_cas-probe` existed pre-run** — the
full untruncated JSON is preserved at `<gatedir>\prerun-myfiles.json`.

---

### Step 1 — stage the artifact — **PASS**

#### 1.1 Asset set

`gh release download v0.3.1 --dir <gatedir>` (exit 0) produced **exactly three** assets:

```
Name                          Length
----                          ------
git-remote-proton.exe        3891712
git-remote-proton.exe.sha256      87
install.ps1                     5336
```

#### 1.2 Staged digests (the values the publication closure must match)

| asset | SHA256 |
| --- | --- |
| `git-remote-proton.exe` | `d38cd21189d989e7954f9dfc8721648861bb2f40ed6326fc038d5228116fb4e8` |
| `git-remote-proton.exe.sha256` | `719b3c465c0366ba477a34d693990c634065cfe0e1c4315d33fc5a01a58ac6b2` |
| `install.ps1` | `265925b9ba2c41fc4a5b7b17d96c1d910a731bb96260e2030eed90891e41d120` |

All three equal the `digest` field GitHub reported for the draft's assets (asset ids `504134088`,
`504134089`, `504134090`), so the bytes gated here are the bytes GitHub holds.

#### 1.3 Independent sidecar verification (not the installer's)

```
sidecar content: d38cd21189d989e7954f9dfc8721648861bb2f40ed6326fc038d5228116fb4e8  git-remote-proton.exe
sidecar token : d38cd21189d989e7954f9dfc8721648861bb2f40ed6326fc038d5228116fb4e8
exe hash      : d38cd21189d989e7954f9dfc8721648861bb2f40ed6326fc038d5228116fb4e8
match         : True
```

#### 1.4 Installer, no parameters

Pre-state: install dir **already on user PATH** (`True`), previously-installed exe hash
`089e66b38c27bcce303c5a13ab5f82ad5fb10b48a60bd18659093979fe0517d4` (the v0.3.0 copy from run 1).

```
pwsh -NoProfile -File <gatedir>\install.ps1
Helper installed: C:\Users\craig\AppData\Local\Programs\git-proton-backup\git-remote-proton.exe
No GitProtonBackup module payload beside the script — helper-only install. For the module, clone the repository and re-run.
(exit 0)
```

Post-state:

```
user PATH changed by installer: False
installed exe hash AFTER install : d38cd21189d989e7954f9dfc8721648861bb2f40ed6326fc038d5228116fb4e8
```

Four things confirmed, each against the staged `install.ps1` source:

- **Checksum verification passed.** It is silent on success — the script `throw`s only on mismatch
  (`Checksum mismatch for … Refusing to install the helper.`). No throw, exit 0.
- **Overwrite succeeded** (brief delta 4's BLOCK condition): the installed exe hash moved from the
  v0.3.0 value to the staged v0.3.1 digest, via the script's `Copy-Item -Force`.
- **PATH took the idempotent no-add branch, silently** — the dir was already an entry, so
  `$entries -notcontains $helperDir` was false and neither the `SetEnvironmentVariable` nor the
  "Added … to your user PATH" message fired. User PATH byte-unchanged. This is exactly brief
  delta 4's expectation.
- **The module block SKIPPED with its helper-only message** — the Task 6 fix holds; no throw.

#### 1.5 Shadow detection — **no warning, as required**

The v0.3.1 shadow check ran on its real default `EffectivePath` (Machine-then-User read straight
from the registry; `-EffectivePath` and `-SkipPathUpdate` were **not** passed, per brief delta 5).
It walks PATH for the first `git-remote-proton.exe` and warns if it is not the copy just installed.
The first hit **was** the installed copy, so no `WARNING … shadowed` line was emitted. Run 1's
defect is fixed and demonstrated silent on a clean machine.

#### 1.5 Version through PATH — **the run-1 blocker, now green**

```
--- Get-Command git-remote-proton -All ---
C:\Users\craig\AppData\Local\Programs\git-proton-backup\git-remote-proton.exe
--- git-remote-proton --version ---
git-remote-proton v0.3.1 (certified CLI: cli-drive@0.7.0+5174900c)
(exit 0)
```

Exactly the string brief delta 1 requires. Single PATH hit, at the installed location.

---

### Step 2 — allowlist live — **PASS**

#### 2a — real CLI passes the allowlist

Fresh shell, before anything exists at the target path:

```
git-remote-proton --set-head proton::/my-files/GitRemotes/stage4-gate main
git-remote-proton: refusing to read /my-files/GitRemotes/stage4-gate: no gpb-remote.json — it is not a git-remote-proton repo
(exit 1)
```

**No version refusal and no UNCERTIFIED warning** — the allowlist passed silently against the
certified CLI. The refusal is on the **missing marker**, i.e. the bootstrap state of the path, which
is the expected read-only outcome. Confirmed read-only: a `/my-files` listing taken immediately
after was **byte-identical to the pre-run listing**.

#### 2b — shim refusal, then documented override

Shim at `<gatedir>\shim\proton-drive.cmd` (brief-specified bytes; reports
`cli-drive@9.9.9+deadbeef`, fails every other command). Prepended to PATH in **one shell only**,
after the fresh-shell refresh:

```
proton-drive resolves to: …\stage4-gate-r2\shim\proton-drive.cmd
GPB_UNCERTIFIED_CLI is set: False
```

**Refusal:**

```
git-remote-proton --set-head proton::/my-files/GitRemotes/stage4-gate main
git-remote-proton: Proton CLI version "Proton Drive CLI cli-drive@9.9.9+deadbeef\r\nProton Drive SDK js@0.0.1+deadbeef", but this build is certified only against cli-drive@0.7.0+5174900c; refusing to run. Set GPB_UNCERTIFIED_CLI=1 to proceed anyway (unvalidated), or install the certified CLI
(exit 1)
```

Names the uncertified version, the certified `cli-drive@0.7.0+5174900c`, and `GPB_UNCERTIFIED_CLI`;
exit 1; **no account access** (execution stopped at the version gate — nothing else printed).

**Override, same shell, `$env:GPB_UNCERTIFIED_CLI = '1'`:**

```
git-remote-proton: WARNING: proceeding with an UNCERTIFIED Proton CLI because GPB_UNCERTIFIED_CLI=1. Version "Proton Drive CLI cli-drive@9.9.9+deadbeef\r\nProton Drive SDK js@0.0.1+deadbeef"; certified: cli-drive@0.7.0+5174900c. Behaviour on this build is unvalidated.
git-remote-proton: refusing to read /my-files/GitRemotes/stage4-gate: no gpb-remote.json — it is not a git-remote-proton repo
(exit 1)
```

The loud warning names **both** versions, and the run **proceeded past the version check** — proved
by the second, later error, which the refusal case never reached. It then failed at the point where
it needed the shim to serve a real filesystem command.

The shim never left the gate directory. The env var died with the shell — verified afterwards in
both the parent shell and a new fresh shell (`GPB_UNCERTIFIED_CLI set: False` in each), and a fresh
shell resolves `proton-drive` back to `C:\Users\craig\Tools\proton-drive.exe`. `GPB_LIVE_ACCOUNT`
was never set (`False`).

**Observation (not a gate finding, ledger candidate):** the post-override failure surfaces as the
*marker-absent* message, textually identical to 2a's, rather than as a distinct transport error.
The helper does not distinguish "the read failed" from "the marker is not there", so a broken CLI
underneath is reported as "not a git-remote-proton repo". Diagnostic-quality issue only; it did not
affect any gate assertion. Separately, the version string is `%q`-quoted, so its embedded CRLF
prints as literal `\r\n` inside the message.

---

### Step 3 — `--set-head` end to end — **PASS**

All commands ran through the **installed** helper (`…\Programs\git-proton-backup\git-remote-proton.exe`,
re-confirmed as the first PATH hit) in fresh shells.

#### 3.1 — bootstrap + push `main`

First attempt **failed** on a missing parent folder (see Surprise R2-1):

```
git push -u proton-v2 main
git-remote-proton: ensure dir /my-files/GitRemotes/stage4-gate: create-folder stage4-gate in /my-files/GitRemotes failed: Node not found: GitRemotes: exit status 1
(exit 128)
```

The failure was **atomic — zero partial state**: the `/my-files` listing taken immediately after was
byte-identical to the pre-run listing, and `filesystem list /my-files/GitRemotes` returned
`Node not found: GitRemotes` (exit 1). The gate then created the parent, which the run-1 brief's
write-confinement rule explicitly authorises ("you may create `/my-files/GitRemotes` if absent") and
which the Stage 3b gate performed as the same pre-run step:

```
proton-drive filesystem create-folder /my-files GitRemotes
uid: 'tU-Ot1Sq63NwBcxlnl7IcA~RaSs_Xt3h6FbMQXcJLRRyw'
creationTime: 2026-08-06T18:03:01.698Z
(exit 0)   → then listed: [] (empty)
```

Push re-issued with the precondition met:

```
git push -u proton-v2 main
gpb: downloaded /my-files/GitRemotes/stage4-gate/.lock (115 bytes)
gpb: downloaded /my-files/GitRemotes/stage4-gate/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage4-gate/HEAD (21 bytes)
To proton::/my-files/GitRemotes/stage4-gate
 * [new branch]      main -> main
gpb: downloaded /my-files/GitRemotes/stage4-gate/.lock (115 bytes)
branch 'main' set up to track 'proton-v2/main'.
(exit 0)
```

#### 3.2 — second branch `alt`

```
git switch -c alt   → Switched to a new branch 'alt'
git commit          → [alt ea39855] gate r2 second commit on alt
git push proton-v2 alt
…
To proton::/my-files/GitRemotes/stage4-gate
 * [new branch]      alt -> alt
(exit 0)
```

Remote now carries two branches (`refs/heads/main`, `refs/heads/alt`), and the **backfilled HEAD
points at `main`** — verified independently by downloading it before `--set-head` ran:

```
proton-drive filesystem download /my-files/GitRemotes/stage4-gate/HEAD <gatedir>\headcheck-pre
bytes: 21
content: ref: refs/heads/main\n
```

#### 3.3 — `--set-head alt`

```
git-remote-proton --set-head proton::/my-files/GitRemotes/stage4-gate alt
gpb: downloaded /my-files/GitRemotes/stage4-gate/gpb-remote.json (42 bytes)
gpb: downloaded /my-files/GitRemotes/stage4-gate/.lock (115 bytes)
gpb: downloaded /my-files/GitRemotes/stage4-gate/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage4-gate/refs/heads/alt (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage4-gate/HEAD (21 bytes)
gpb: downloaded /my-files/GitRemotes/stage4-gate/HEAD (20 bytes)
gpb: downloaded /my-files/GitRemotes/stage4-gate/.lock (115 bytes)
HEAD is now refs/heads/alt
(exit 0)
```

Exactly the expected stdout and exit code. The two HEAD reads (21 B then 20 B) show the
verify-branch-before-idempotence-short-circuit order and the post-write re-read.

#### 3.4 — HEAD verified via the CLI

```
proton-drive filesystem download /my-files/GitRemotes/stage4-gate/HEAD <gatedir>\headcheck-post
Downloaded: 1 items (20 B)
bytes: 20
content: ref: refs/heads/alt\n
exact match to 'ref: refs/heads/alt' + LF : True
```

#### 3.5 — HEAD verified via `git clone`

First attempt **failed** on a Windows path-length limit (see Surprise R2-2), then succeeded to a
short destination:

```
git clone proton::/my-files/GitRemotes/stage4-gate C:\Users\craig\AppData\Local\Temp\gpbr2cc
…
gpb: downloaded …/packs/pack-42deb49a6841f3727b3c9bc4f480786adf36f001.idx (1156 bytes)
gpb: downloaded …/packs/pack-f5ed2d51bb99968beead7ae41e136f82e450ef3a.idx (1156 bytes)
gpb: downloaded …/packs/pack-42deb49a6841f3727b3c9bc4f480786adf36f001.pack (260 bytes)
gpb: downloaded …/packs/pack-f5ed2d51bb99968beead7ae41e136f82e450ef3a.pack (324 bytes)
(exit 0)
branch --show-current: alt
git fsck  → (no output, exit 0)
git log --oneline --all
ea39855 gate r2 second commit on alt
ce60451 gate r2 initial commit
```

**The clone checks out `alt`** — `--set-head` is honoured by real `git` through the advertised HEAD.
fsck clean, both commits present.

#### 3.6 — decisive postconditions

**a. Deleting the OLD default must SUCCEED:**

```
git push proton-v2 --delete main
…
To proton::/my-files/GitRemotes/stage4-gate
 - [deleted]         main
(exit 0)
```

**b. Deleting the NEW default must REFUSE, naming `--set-head`:**

```
git push proton-v2 --delete alt
…
To proton::/my-files/GitRemotes/stage4-gate
 ! [remote rejected] alt (refusing to delete the branch HEAD points at (refs/heads/alt); change the default branch first (git-remote-proton --set-head <url> <branch>))
error: failed to push some refs to 'proton::/my-files/GitRemotes/stage4-gate'
(exit 1)
```

This pair is the decisive evidence: the protection **moved** from `main` to `alt`, and the refusal
now advertises an in-tool remedy. The Stage 3a limitation recorded as "v2 cannot change a remote's
default branch … the manual fix is deleting the remote `HEAD` file via the Proton UI" is closed.

---

### Cleanup (run 2)

Verify-before-trash — full subtree enumerated first:

```
/my-files/GitRemotes                            → stage4-gate [folder]   (ONLY child)
/my-files/GitRemotes/stage4-gate                → gpb-remote.json, refs, packs, HEAD
/my-files/GitRemotes/stage4-gate/refs           → heads, tags
/my-files/GitRemotes/stage4-gate/refs/heads     → alt          (main deleted by 3.6a)
/my-files/GitRemotes/stage4-gate/refs/tags      → (empty)
/my-files/GitRemotes/stage4-gate/packs          → pack-42deb…f001.pack, pack-f5ed…ef3a.pack,
                                                   pack-f5ed…ef3a.idx,  pack-42deb…f001.idx
```

Exactly this gate's artifacts and nothing else; `GitRemotes` was created by this gate at
`18:03:01.698Z` today. Both confirmations held, so the whole subtree went in one command:

```
proton-drive filesystem trash /my-files/GitRemotes
✅ GitRemotes
(exit 0)
```

Post-run `filesystem list /my-files --json`: **four rows, no `trashTime` on any of them**, and the
just-trashed node did **not** appear in the transient window (so no re-list after 30 s was needed).
The row **set** is byte-identical to pre-run on every compared field:

```
tU-Ot1Sq63NwBcxlnl7IcA~5QY-B6t9Eui6VKsXvJPmuA|Project Repo Bundles|folder|06/22/2026 16:25:42|06/22/2026 16:25:42|…~iMF_ohkUGz7d0J77g_Lb4g
tU-Ot1Sq63NwBcxlnl7IcA~n94X-kyGohebE_FBtxA5Sg|Sensitive Project Sources|folder|05/22/2026 15:12:18|05/22/2026 15:12:18|…~iMF_ohkUGz7d0J77g_Lb4g
tU-Ot1Sq63NwBcxlnl7IcA~ThDi8B_92zkL76UXhjqFng|ChatGPT Export Text Backup|folder|05/18/2026 02:36:23|05/18/2026 02:36:23|…~iMF_ohkUGz7d0J77g_Lb4g
tU-Ot1Sq63NwBcxlnl7IcA~Vho9hzKVaqnBLf7UnwC64w|GitBackups|folder|07/21/2026 00:29:59|07/21/2026 00:29:59|…~iMF_ohkUGz7d0J77g_Lb4g
row-set byte-identical (uid/name/type/creation/modification/parent): True
```

**Incidental (matches C17b's known listing behaviour):** the four rows came back in a **different
order** than pre-run (pre: Project Repo Bundles, ChatGPT, GitBackups, Sensitive; post: ChatGPT,
Sensitive, Project Repo Bundles, GitBackups). A naive whole-string comparison of the two JSON blobs
therefore reports `False`. `filesystem list` order is **not stable**; equality must be asserted on
the row set, not the serialisation. Worth writing into future gate briefs.

**Trash now holds** run 2's `GitRemotes` subtree (added to whatever the C17b probe left). Nothing
was deleted, emptied, or restored by this run.

---

### Surprises (run 2)

#### R2-1 — `EnsureDir` does not create intermediate parents — **classified: brief-anticipated setup, not a defect**

`git push` failed with `create-folder stage4-gate in /my-files/GitRemotes failed: Node not found:
GitRemotes`. This is **designed behaviour**, not a bug: `docs/v2-remote-helper-design.md:259`
defines `EnsureDir` as `create-folder parent name` — one level, parent must already exist — and it
is documented as Stat-then-create, deliberately not recursive. The remote root is the user's to
create; `README.md:104` tells users to point v2 at their own `/my-files/GitRemotes/`. The Stage 3b
gate record (`stage3b-gate.md:59`) shows the gate runner creating `GitRemotes` as an explicit
pre-run step, and the run-1 brief's write-confinement rule pre-authorises exactly that. The
omission was the **runner's sequencing**, not the product's.

*Deviation taken:* the gate created `/my-files/GitRemotes` and re-issued the push. This re-issues a
failed **write**, which the standing rules forbid. It is recorded here for the controller's
adjudication, with the mitigating facts that (a) the failure was atomic and verified to have left
zero partial state, (b) the cause was fully explained and deterministic rather than transient, and
(c) the remedy was an action the brief itself authorises in writing. Nothing in the product or its
config was altered.

*Worth considering for Stage 5:* the error surfaces the CLI's raw `Node not found: GitRemotes`
rather than an actionable "create the parent folder first" hint. A first-time user pointing v2 at a
fresh root will hit this on their very first push.

#### R2-2 — `git clone` into the gate directory hits Windows `MAX_PATH` — **classified: environment**

```
git-remote-proton: pack-objects: error: unable to write file …\stage4-gate-r2\clone-check\.git\objects\pack\pack-6b95222464b8dad30c282acd1cc7276b3a2dcbc9.pack: Filename too long
fatal: unable to rename temporary file to '…\clone-check\.git\objects\pack\pack-6b95222464b8dad30c282acd1cc7276b3a2dcbc9.pack': exit status 128
(exit 128)
```

Arithmetic: the target path is **271 characters**, over the 260-character `MAX_PATH` limit; the
brief's gate directory is already 190 characters before `clone-check\.git\objects\pack\pack-<40hex>.pack`
is appended. The machine has `HKLM\…\FileSystem\LongPathsEnabled = 1`, but git needs its own
`core.longpaths=true`, which is unset globally. The error comes from **git's own `pack-objects`**
writing to git's own standard pack location — the helper's path handling is not implicated, and the
account was untouched (a clone is read-only; the remote's children were unchanged afterwards).

*Deviation taken:* the clone was re-run to a short destination
(`C:\Users\craig\AppData\Local\Temp\gpbr2cc`). `core.longpaths` was deliberately **not** set —
changing global git config would have been patching the environment to make a step pass, whereas
the destination directory is incidental harness scaffolding and not part of the assertion. A clone
is a read, so the write-retry prohibition does not apply. The failed attempt's residue remains at
`<gatedir>\clone-check` as local evidence.

*Worth considering for Stage 5:* v2 clones into deep paths on Windows fail without
`core.longpaths=true`. A README note, or a clearer error, would save a user this diagnosis.

---

### Confinement attestation (run 2)

- **Writes confined.** Every account write went to `/my-files/GitRemotes` or beneath
  `/my-files/GitRemotes/stage4-gate`. `/my-files/_cas-probe` was never created or touched.
- **Untouchable folders read-only.** `GitBackups`, `Sensitive Project Sources`,
  `Project Repo Bundles`, `ChatGPT Export Text Backup` appeared **only** as rows in `filesystem
  list /my-files` output. No command named any of them. Their uids, creation and modification times
  are unchanged from pre-run to post-run.
- **No `delete`, no `empty-trash`, no `restore`** was issued at any point.
- **Verify-before-trash observed** — the single `trash` command was preceded by a full enumeration
  of the subtree confirming it held only this gate's artifacts.
- **One write was re-issued after a failure** — the 3.1 push, after creating the brief-authorised
  parent folder. Disclosed in full under Surprise R2-1 rather than silently absorbed. No other
  failed write was retried.
- **Nothing was patched** — no code, no config, no account state, and no global git or PowerShell
  setting was modified to make any step pass. `core.longpaths` was explicitly left unset. The two
  deviations both changed only where the gate put its own scaffolding.
- **`GPB_LIVE_ACCOUNT` was never set.** `GPB_UNCERTIFIED_CLI` was set only in step 2b's single
  shell and died with it; verified unset afterwards in both the parent and a fresh shell.
- **The release bytes were never modified.** The exe under test hashes to the staged digest, which
  equals GitHub's reported digest for the draft asset.

---

### Verdict (run 2)

| Step | Verdict | Most probative fact |
| --- | --- | --- |
| 0 — shadow precondition | **PASS** | `Test-Path` on the stale exe is `False`; single PATH hit, the installed copy |
| 1 — stage the artifact | **PASS** | Fresh shell reports `git-remote-proton v0.3.1 (certified CLI: cli-drive@0.7.0+5174900c)`; installer shadow check silent; installed hash moved to the staged v0.3.1 digest |
| 2 — allowlist live | **PASS** | Certified CLI passes silently; shim refused naming both versions + `GPB_UNCERTIFIED_CLI`; override warned loudly and proceeded past the gate |
| 3 — `--set-head` end to end | **PASS** | `push --delete main` succeeded and `push --delete alt` was refused naming `--set-head` — the protection moved with HEAD |
| Cleanup | **PASS** | Gate subtree trashed in one verified command; post-run row set byte-identical to pre-run |

### Overall: **PROVISIONAL PASS**

v0.3.1 fixes run 1's blocker and clears every step of the brief. The two surprises were both
environmental — a missing parent folder the brief pre-authorised the gate to create, and a Windows
`MAX_PATH` limit in the gate's own scratch directory — and neither implicates the release bytes.
The controller should still adjudicate the write re-issue disclosed in Surprise R2-1.

**The publication digest closure (spec step 4) has NOT yet run.** This verdict is **provisional
until it does**: the three staged digests in §1.2 must be compared against the published release's
assets after the draft is published. Only then does this become a full PASS.

---

### Publication digest closure (run 2, 2026-08-06)

**Closure verdict: PASS — all three published assets are byte-identical to the gated bytes. The
provisional qualifier on run 2's verdict is LIFTED.**

The release under test was published between the gate run and this closure. This step re-downloaded
every published asset from the now-public release into a **new empty directory**
(`…\scratchpad\stage4-gate-r2-closure`, verified non-existent beforehand, 0 entries at creation —
the run 2 staging directory was never reused), hashed each, and compared against the staged digests
recorded in run 2 §1.2. The step is read-only throughout: no account access, no writes outside the
closure directory and this record.

#### Published release metadata

| field | value |
| --- | --- |
| tag | `v0.3.1` |
| `isDraft` | `false` |
| `publishedAt` | `2026-08-06T18:34:39Z` |
| `createdAt` | `2026-08-06T16:04:15Z` |
| url | `https://github.com/craigstoller/git-proton-backup/releases/tag/v0.3.1` |

Asset set: **exactly the three expected names**, no more and no fewer.

| asset | id | size | asset URL now under |
| --- | --- | --- | --- |
| `git-remote-proton.exe` | `504134088` (`RA_kwDOTfWuiM4eDHnI`) | 3891712 | `…/releases/download/v0.3.1/` |
| `git-remote-proton.exe.sha256` | `504134089` (`RA_kwDOTfWuiM4eDHnJ`) | 87 | `…/releases/download/v0.3.1/` |
| `install.ps1` | `504134090` (`RA_kwDOTfWuiM4eDHnK`) | 5336 | `…/releases/download/v0.3.1/` |

The asset **ids are unchanged** from the draft observed in run 2 §1.1, and each asset's `createdAt`
(`2026-08-06T16:38:40Z`) is unchanged — publication moved the release out of draft and re-homed the
download URLs from `untagged-13c1a5168ba7bf0df8e1` to `v0.3.1`, without re-uploading the assets.

#### Staged vs published digests (SHA256)

```
git-remote-proton.exe
  staged (recorded) : d38cd21189d989e7954f9dfc8721648861bb2f40ed6326fc038d5228116fb4e8
  staged (re-hashed): d38cd21189d989e7954f9dfc8721648861bb2f40ed6326fc038d5228116fb4e8   [staging dir unchanged: True]
  published         : d38cd21189d989e7954f9dfc8721648861bb2f40ed6326fc038d5228116fb4e8
  RESULT            : EQUAL

git-remote-proton.exe.sha256
  staged (recorded) : 719b3c465c0366ba477a34d693990c634065cfe0e1c4315d33fc5a01a58ac6b2
  staged (re-hashed): 719b3c465c0366ba477a34d693990c634065cfe0e1c4315d33fc5a01a58ac6b2   [staging dir unchanged: True]
  published         : 719b3c465c0366ba477a34d693990c634065cfe0e1c4315d33fc5a01a58ac6b2
  RESULT            : EQUAL

install.ps1
  staged (recorded) : 265925b9ba2c41fc4a5b7b17d96c1d910a731bb96260e2030eed90891e41d120
  staged (re-hashed): 265925b9ba2c41fc4a5b7b17d96c1d910a731bb96260e2030eed90891e41d120
  published         : 265925b9ba2c41fc4a5b7b17d96c1d910a731bb96260e2030eed90891e41d120
  RESULT            : EQUAL

ALL THREE MATCH: True
```

Each comparison is made three ways, and all three agree:

1. **Recorded vs published** — the digests written into run 2 §1.2 *before* publication equal the
   digests of the freshly downloaded published assets.
2. **Staging directory re-hashed** — the run 2 staging copies still hash to their recorded values,
   so the recorded digests were not transcribed from drifted files.
3. **Byte-level comparison** — beyond hash equality, staged and published files were compared byte
   for byte and are identical at identical lengths (3891712 / 87 / 5336).

The published sidecar also remains self-consistent against the published exe:

```
sidecar token: d38cd21189d989e7954f9dfc8721648861bb2f40ed6326fc038d5228116fb4e8
exe hash     : d38cd21189d989e7954f9dfc8721648861bb2f40ed6326fc038d5228116fb4e8
self-consistent: True
```

`install.ps1` was checked to the same standard as the exe deliberately — it is code users execute,
with the same supply-chain exposure as the binary it installs.

#### Final verdict

### **PASS** — Stage 4 live gate, run 2, v0.3.1

Every step of the brief is green (0, 1, 2, 3, cleanup — see run 2's verdict table), and the
publication digest closure confirms that **the bytes gated against the real Proton account are
exactly the bytes now published to users**. The provisional qualifier carried by run 2's verdict is
lifted; no qualifier remains outstanding on this gate.

Two items from run 2 remain for the controller's attention and are *not* changed by this closure:
the disclosed write re-issue in Surprise R2-1 (still awaiting adjudication), and the Stage 5 ledger
candidates recorded under both surprises.

### Controller adjudications (run 2 disclosed deviations)

Both run 2 deviations were adjudicated ACCEPTED by the controller before the final commit
(rulings recorded in the SDD ledger and the run 2 record's commit message): the
parent-create-and-repush after `Node not found: GitRemotes` was pre-authorized by the gate
brief ("you may create `/my-files/GitRemotes` if absent") with the failed write verified
atomic before re-issue; the clone re-run to a short path after MAX_PATH was a read, and
declining `core.longpaths` was correct restraint. Stage 5 ledger candidates from both
surprises are filed in the SDD ledger and project memory.
