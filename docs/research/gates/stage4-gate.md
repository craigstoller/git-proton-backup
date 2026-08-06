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
