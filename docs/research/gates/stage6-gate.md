# Stage 6 live gate — run record

**Brief executed:** `docs/research/gates/stage6-gate-brief.md` (incorporating
`docs/research/gates/brief-checklist.md` by reference).
**Runner:** Stage 6 live gate runner agent (Claude Opus 5), on Craig's machine, against Craig's
real Proton Drive account, with Craig's explicit in-session authorization.
**Run start:** `2026-08-09T23:00:02.7717168-07:00` (Pacific).
**Verdict status:** PROVISIONAL until the post-publication digest closure runs (a separate,
later, read-only pass — not this run's job; brief "Release integration", third bullet).

This is a **record**, appended step by step as the run proceeded. Every command is given as run;
every string the brief asserts is reproduced verbatim from the observed output.

## Scratch-path allocation (runner environment)

All of the brief's / launching agent's nominated local paths were free at run start (checked
`2026-08-09T23:00:02` Pacific), so **no substitutions were needed**:

```
C:\gpb6 exists=False
C:\gpb6\stage6-gate exists=False
C:\gpb6\clone1 exists=False
C:\gpb6\draft-stage exists=False
C:\gpb6-junk exists=False
```

- demo repo: `C:\gpb6\stage6-gate`
- clone: `C:\gpb6\clone1`
- draft asset staging: `C:\gpb6\draft-stage`
- junk staging (outside the git working tree, per outline step 1 item 1): `C:\gpb6-junk`
- source checkout (outline step 3 working directory; read-only otherwise):
  `C:\Users\craig\Projects\_Tools\git-proton-backup`

Source checkout state confirmed before anything ran (and never modified by this run):

```
git -C C:\Users\craig\Projects\_Tools\git-proton-backup log -1 --format='%H %d'
2d76db7a83b70c6ee5912786f6da11c55c41aefd  (HEAD -> main, tag: v0.5.0, origin/main, origin/HEAD)

git -C ... status --porcelain      -> (no output; clean)
git -C ... rev-parse v0.5.0        -> 2d76db7a83b70c6ee5912786f6da11c55c41aefd
```

`HEAD` IS the `v0.5.0` tag commit, as the launching brief stated.

---

# Release integration (run FIRST, before any account write)

Per brief "Release integration": this gate runs against the **v0.5.0 DRAFT's bytes**, digests
staged before any account write. Executed in the order draft download -> staged baseline ->
independent sidecar verification -> real installer -> fresh-shell PATH check -> `--version`.

## R1. Draft asset download — `2026-08-09T23:00:28.7888302-07:00`

```
gh release download v0.5.0 --repo craigstoller/git-proton-backup --dir C:\gpb6\draft-stage
EXIT=0

Name                          Length
----                          ------
git-remote-proton.exe        3982336
git-remote-proton.exe.sha256      87
install.ps1                     8805
```

Exactly three assets, each byte-length exactly as expected. **PASS.**

## R2. Staged SHA-256 baseline — `2026-08-09T23:00:46.5107319-07:00`

Recorded BEFORE any account write, per the brief's Release-integration bullet 1. These are the
digests the later publication-digest closure (a separate pass, not this run) compares against:

| asset | bytes | SHA-256 (staged baseline) |
|---|---|---|
| `git-remote-proton.exe` | 3982336 | `e633eaf5158a61ef18b79a12fe733dbc03d649f57e4b9fdb4336ff847c65892d` |
| `git-remote-proton.exe.sha256` | 87 | `4248c88f5bd4728970aec1f9bd003a1f7089a3ee33706b3635af3b6050f7e9e6` |
| `install.ps1` | 8805 | `8987ab2b73e66a79b5a33c4652d92ca10d2a2aae3fe4a2e1174b3519858575e0` |

## R3. Independent sidecar verification (recomputed, not the installer's own check)

Sidecar file content, verbatim:

```
e633eaf5158a61ef18b79a12fe733dbc03d649f57e4b9fdb4336ff847c65892d  git-remote-proton.exe
```

Independently recomputed exe digest: `e633eaf5158a61ef18b79a12fe733dbc03d649f57e4b9fdb4336ff847c65892d`.
Identical. **PASS** — satisfies "Verify the sidecar independently (recompute, don't trust the
installer's own check)".

## R4. Pre-install PATH survey — `2026-08-09T23:01:03.1323039-07:00`

```
$c = Get-Command git-remote-proton -All -ErrorAction SilentlyContinue
C:\Users\craig\AppData\Local\Programs\git-proton-backup\git-remote-proton.exe
```

Exactly one pre-existing copy, and it sits at the canonical install directory the installer
targets (a prior stage's install, which this run overwrites). No copy anywhere else on PATH, so
no shadowing candidate exists.

## R5. Real installer run — `2026-08-09T23:01:18.5534933-07:00`

Run as the real installer with **no `-SkipPathUpdate`**, per the launching brief (the PATH-shadow
precondition needs the real install). The script was read in full before execution.

```
& C:\gpb6\draft-stage\install.ps1
Helper installed: C:\Users\craig\AppData\Local\Programs\git-proton-backup\git-remote-proton.exe
No GitProtonBackup module payload beside the script — helper-only install. For the module, clone the repository and re-run.
```

No checksum-mismatch throw (the installer's own sidecar check passed too), and — load-bearing —
**no `Another git-remote-proton.exe is earlier on PATH and shadows the copy just installed`
warning**, which is install.ps1's own shadow detector (lines 116-121) staying silent. **PASS.**

## R6. Preconditions step 2 — fresh-shell PATH shadow check — `2026-08-09T23:01:30.2334279-07:00`

A new PowerShell tool call is a genuinely fresh process, and it re-establishes the effective PATH
from the registry (`Machine` then `User`, in order) exactly as the brief's Precondition 2
prescribes:

```
$env:Path = [Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [Environment]::GetEnvironmentVariable('Path','User')
(Get-Command git-remote-proton -All).Source
C:\Users\craig\AppData\Local\Programs\git-proton-backup\git-remote-proton.exe
```

`-All` returned exactly ONE hit, and that first (only) hit IS the draft-installed helper.
**No shadowing. PASS** (Preconditions step 2; "Any shadowing = BLOCKED" not triggered).

Installed-copy digest re-verified against the staged baseline:

```
(Get-FileHash 'C:\Users\craig\AppData\Local\Programs\git-proton-backup\git-remote-proton.exe' -Algorithm SHA256)
e633eaf5158a61ef18b79a12fe733dbc03d649f57e4b9fdb4336ff847c65892d
```

Identical to R2's staged exe digest — the helper this gate exercises is byte-identical to the
draft asset. **PASS.**

## R7. `--version` assertion

```
git-remote-proton --version
git-remote-proton v0.5.0 (certified CLI: cli-drive@0.7.0+5174900c)
EXIT=0
```

Character-for-character the string the brief's Release-integration bullet 1 requires. **PASS.**

---

# Preconditions

## Precondition 1 — CLI / git versions — `2026-08-09T23:01:30` Pacific

```
proton-drive --version
Proton Drive CLI cli-drive@0.7.0+5174900c
Proton Drive SDK js@0.20.0+5174900c

git --version
git version 2.53.0.windows.2
```

The CLI build is the certified `cli-drive@0.7.0+5174900c`, unchanged since Stage 4/5 as the brief
states. **PASS.**

## Precondition 2 — PATH shadow check

Satisfied at R6 above. **PASS.**

## Precondition 3 — empty trash

**Satisfied on Craig's report.** Craig confirmed in-session (2026-08-09, Pacific) that he emptied
the Proton trash via the web UI. The CLI cannot verify this directly, so it is recorded here as a
report-based precondition exactly as the brief permits ("taken on report from Craig's web-UI
action unless the CLI can verify it directly", Preconditions step 3 / checklist item 6). No
CLI-side evidence is claimed for it.

## Precondition 4 — pre-run `/my-files` listing (THE BASELINE) — `2026-08-09T23:01:45.1531940-07:00`

```
proton-drive filesystem list /my-files --json
EXIT=0
```

Row-set form (checklist item 1 — this is the form every later comparison uses; raw JSON is never
compared byte-wise). Every row's `parentUid` is the `/my-files` root
`tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g`:

| name | type | uid | creationTime | modificationTime |
|---|---|---|---|---|
| `Project Repo Bundles` | folder | `tU-Ot1Sq63NwBcxlnl7IcA~5QY-B6t9Eui6VKsXvJPmuA` | `2026-06-22T16:25:42.000Z` | `2026-06-22T16:25:42.000Z` |
| `_cas-probe` | folder | `tU-Ot1Sq63NwBcxlnl7IcA~TfvH3TrgNW1Eyo6Urs7hmQ` | `2026-08-09T21:51:50.000Z` | `2026-08-09T21:51:50.000Z` |
| `ChatGPT Export Text Backup` | folder | `tU-Ot1Sq63NwBcxlnl7IcA~ThDi8B_92zkL76UXhjqFng` | `2026-05-18T02:36:23.000Z` | `2026-05-18T02:36:23.000Z` |
| `GitBackups` | folder | `tU-Ot1Sq63NwBcxlnl7IcA~Vho9hzKVaqnBLf7UnwC64w` | `2026-07-21T00:29:59.000Z` | `2026-07-21T00:29:59.000Z` |
| `Sensitive Project Sources` | folder | `tU-Ot1Sq63NwBcxlnl7IcA~n94X-kyGohebE_FBtxA5Sg` | `2026-05-22T15:12:18.000Z` | `2026-05-22T15:12:18.000Z` |

Five rows. No `trashTime` on any row (consistent with Precondition 3's emptied trash).

**Two determinations follow directly from this baseline:**

1. **No `GitRemotes` row exists.** This is the **expected fresh-creation case** of outline step
   5's first bullet: this gate creates `/my-files/GitRemotes`, therefore it is this gate's to
   trash, and outline step 5's full close applies (trash both children, then verify and trash
   `/my-files/GitRemotes` itself). The pre-existing-`GitRemotes` branch — with its
   `modificationTime`-only tolerance — does NOT apply to this run.
2. **`_cas-probe` is a pre-existing row this gate must not disturb.** It is not one of the four
   named untouchables, but it is in the baseline, so the strict row-set identity rule covers it.
   Note this is precisely the folder that would have been written into had
   `GPB_CONTRACT_LIVE_ROOT` been left unset (the `liveRoot` const is
   `/my-files/_cas-probe/contract` — checklist item 8); outline step 3 pins the env var to this
   gate's own confined root instead, so `_cas-probe` is never touched.

## Precondition 5 — write confinement (restated as executed)

- Writes permitted ONLY under `/my-files/GitRemotes/stage6-gate` and
  `/my-files/GitRemotes/stage6-gate-contract`, plus the gate-authorized creation — and, per the
  symmetric authorization and the fresh-creation determination above, the cleanup trash — of
  `/my-files/GitRemotes` itself.
- Untouchables `GitBackups`, `Sensitive Project Sources`, `Project Repo Bundles`, `ChatGPT Export
  Text Backup` are read-only: they appear as rows in listings above and below, and are **never
  named in any write command** in this run. (Confirmed at the end of this record.)

---

# Outline step 1 — Junk manufacture, tolerant push, strict `ls-remote`

## 1.0 Bootstrap the demo repo — `2026-08-09T23:03:24.9982895-07:00`

```
git init -b main stage6-gate
Initialized empty Git repository in C:/gpb6/stage6-gate/.git/

cd stage6-gate
git commit --allow-empty -m "gate: stage6 initial commit"
[main (root-commit) 048f39e] gate: stage6 initial commit

git remote add proton-v2 "proton::/my-files/GitRemotes/stage6-gate"
git remote -v
proton-v2	proton::/my-files/GitRemotes/stage6-gate (fetch)
proton-v2	proton::/my-files/GitRemotes/stage6-gate (push)

git rev-parse HEAD
048f39eba95a5c555eefb271be5c1a888bf2c672
```

Bootstrap commit = `048f39eba95a5c555eefb271be5c1a888bf2c672`. The remote is added with the
literal `proton::` URL **scheme**; every later command in this repo uses the `proton-v2` local
**alias** — the distinction the brief's naming-convention paragraph insists on.

### 1.0a First bootstrap push attempt — refused for a missing parent (`2026-08-09T23:03:35`)

```
git push -u proton-v2 main
EXIT=128
--- stdout --- (empty)
--- stderr ---
git-remote-proton: parent folder /my-files/GitRemotes does not exist; create it first (proton-drive filesystem create-folder /my-files GitRemotes, or the web UI), or set GPB_CREATE_PARENTS=1 to let the helper create missing parents
```

**This is expected, and its handling is fixed by the brief, not improvised.** Preconditions step
5 predicted `/my-files/GitRemotes` would be absent (Stage 5's cleanup trashed it) and
**pre-authorized this gate to create it**. The brief's own Confinement rules simultaneously
forbid the other route: "`GPB_CREATE_PARENTS` is not exercised by this brief". So the authorized
remedy is exactly the one the helper's own message names first — an explicit CLI
`create-folder`, run against the gate-authorized path and nothing else.

Recorded as an **in-spirit deviation for adjudication** (see the "Deviations" section at the end):
the brief's step-1.0 command block reads as though `git push` itself creates `/my-files/GitRemotes`
("This creates `/my-files/GitRemotes` if absent"), but the shipped helper refuses missing parents
unless `GPB_CREATE_PARENTS=1`, which this brief forbids. No code, message, or config was changed;
the folder was created through the documented CLI, inside the authorized confinement.

Before creating anything, confirmed the refused push wrote NOTHING to the account — `/my-files`
still held exactly the five baseline names:

```
"value":"_cas-probe"
"value":"ChatGPT Export Text Backup"
"value":"GitBackups"
"value":"Project Repo Bundles"
"value":"Sensitive Project Sources"
```

### 1.0b Gate-authorized creation of `/my-files/GitRemotes` — `2026-08-09T23:04:01`

```
proton-drive filesystem create-folder /my-files GitRemotes
EXIT=0
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~0PkXib50j9udwUr3NZjoig',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g',
  name: { ok: true, value: 'GitRemotes' },
  type: 'folder',
  creationTime: 2026-08-10T06:04:10.246Z,
  modificationTime: 2026-08-10T06:04:10.246Z,
  trashTime: undefined,
```

`GitRemotes` uid = `tU-Ot1Sq63NwBcxlnl7IcA~0PkXib50j9udwUr3NZjoig` (this is the row outline step 5
must remove, restoring the baseline). The verification listing showed the five baseline rows
unchanged plus this one new row — **and in a different order than the baseline listing**, which is
checklist item 1 / F2's unstable-order hazard observed live again in this run; every comparison in
this record is therefore made on the row set, never on serialized order.

### 1.0c Bootstrap push, re-issued — `2026-08-09T23:04:25` -> `2026-08-09T23:07:14` (~2m50s)

Run with a 600000 ms tool timeout (checklist item 9 / brief's standing push note).

```
git push -u proton-v2 main
EXIT=0
--- stdout ---
branch 'main' set up to track 'proton-v2/main'.
--- stderr ---
gpb: downloaded /my-files/GitRemotes/stage6-gate/.lock (115 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/HEAD (21 bytes)
To proton::/my-files/GitRemotes/stage6-gate
 * [new branch]      main -> main
gpb: downloaded /my-files/GitRemotes/stage6-gate/.lock (115 bytes)
```

**PASS.** Repo marker, `refs/heads/main`, and HEAD created. Note the ~2m50s duration — this alone
vindicates checklist item 9: a default 2-minute harness timeout would have killed this push
mid-flight, which is precisely the Stage 5 S1 failure.

## 1.1 Manufacture the two junk files — `2026-08-09T23:07:28.7679394-07:00`

Staged OUTSIDE the git working tree (`C:\gpb6-junk`), per the brief, so they can never appear as
untracked files in the demo repo.

```
New-Item -ItemType Directory -Force -Path C:\gpb6-junk | Out-Null
[System.IO.File]::WriteAllText('C:\gpb6-junk\gate-junk-a', ('x' * 41), [System.Text.Encoding]::ASCII)
[System.IO.File]::WriteAllText('C:\gpb6-junk\gate-junk-b', ('y' * 100), [System.Text.Encoding]::ASCII)

Name        Length
----        ------
gate-junk-a     41
gate-junk-b    100

proton-drive filesystem upload C:\gpb6-junk\gate-junk-a /my-files/GitRemotes/stage6-gate/refs/heads
Transfer summary:
  Uploaded: 1 items (41 B)
EXIT_A=0
proton-drive filesystem upload C:\gpb6-junk\gate-junk-b /my-files/GitRemotes/stage6-gate/refs/heads
Transfer summary:
  Uploaded: 1 items (100 B)
EXIT_B=0
```

### 1.1 verification — mandated size check before relying on the expected strings

```
proton-drive filesystem list /my-files/GitRemotes/stage6-gate/refs/heads --json
EXIT=0
```

Row set (parent `refs/heads` uid = `tU-Ot1Sq63NwBcxlnl7IcA~JjfLMwaG41i1noR6L23GiQ`):

| name | type | uid | claimedSize | creationTime |
|---|---|---|---|---|
| `main` | file | `tU-Ot1Sq63NwBcxlnl7IcA~tb2FFTydAoMHOlidadS2OQ` | **41** | `2026-08-10T06:06:36.000Z` |
| `gate-junk-a` | file | `tU-Ot1Sq63NwBcxlnl7IcA~sL_EEusAdQM-3So23RpHlA` | **41** | `2026-08-10T06:07:32.000Z` |
| `gate-junk-b` | file | `tU-Ot1Sq63NwBcxlnl7IcA~QilRDTI_wE7JaiB0ldGkDQ` | **100** | `2026-08-10T06:07:36.000Z` |

**Observed content sizes are exactly 41 and 100, as the brief predicted — so NO recompute of the
expected note text was required**, and the brief's literal expected reasons apply as written.
(`totalStorageSize` reads 119/119/178 — Proton's encrypted-blob size, not content length;
`claimedSize` is the content byte count the helper's size gate reasons about. `main`'s own 41
bytes are a real ref: 40 hex + newline.) Both junk files landed under their local basenames as
otherwise-valid branch names, confirming probe C11's naming behaviour.

## 1.2 Tolerant push of an unrelated ref — `2026-08-09T23:08:12` -> `2026-08-09T23:10:29` (~2m17s)

Run with a 600000 ms tool timeout.

```
git push proton-v2 main:push-ok
EXIT=0
```

Full verbatim stderr, in observed order:

```
gpb: downloaded /my-files/GitRemotes/stage6-gate/gpb-remote.json (42 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/.lock (115 bytes)
git-remote-proton: skipping /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-b: not a ref: size 100 outside the 40-42 candidate band
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-a (41 bytes)
git-remote-proton: skipping /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-a: not a ref: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/HEAD (21 bytes)
git-remote-proton: skipping /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-b: not a ref: size 100 outside the 40-42 candidate band
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-a (41 bytes)
git-remote-proton: skipping /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-a: not a ref: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/push-ok (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/HEAD (21 bytes)
To proton::/my-files/GitRemotes/stage6-gate
 * [new branch]      main -> push-ok
gpb: downloaded /my-files/GitRemotes/stage6-gate/.lock (115 bytes)
```

**Determinations:**

- **Tolerant direction PASS** — exit 0 and git's own `* [new branch]      main -> push-ok`, with
  two skipped foreign files present under `refs/`. The push was not blocked by them.
- **`skipNote` text PASS** (runner-asserted string 4, template
  `"git-remote-proton: skipping %s/%s: %v\n"` with `(root, full, reason)`) — both rendered lines
  match character-for-character:
  - `git-remote-proton: skipping /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-a: not a ref: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"`
  - `git-remote-proton: skipping /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-b: not a ref: size 100 outside the 40-42 candidate band`
  The quoted run in `gate-junk-a`'s reason was measured programmatically, not by eye:
  `[regex]::Match($t,'not a ref: "(x+)"')` -> `matched=True  x_count=41`. Exactly 41, the
  in-band candidate's full content, as `classifyRefContent`'s generic-junk branch predicts.
- **The `skipNote` count question the brief flags as fresh analysis: TWICE per push, confirmed
  live.** Measured, not eyeballed:
  ```
  gate-junk-a skipNote count : 2
  gate-junk-b skipNote count : 2
  ```
  This is the brief's predicted-but-unconfirmed shape, now observed: `git push` sends
  `list for-push` (whose handler calls `repo.ScanRefs`) and then the `push ` handler calls
  `ScanRefs` a SECOND time to build the phase-2 occupancy set; `ScanRefs` emits `skipNote`
  unconditionally on every walk. The two scan blocks are directly visible in the transcript above,
  separated by the first block's `HEAD` read — and note the SECOND block additionally downloads
  `refs/heads/push-ok`, which the first could not have seen. Not a defect; the brief pre-authorized
  either count.
- **The positive/negative `gpb: downloaded` pair — the spec's "observed via the absence of a
  transfer" requirement — PASS, measured:**
  ```
  'gpb: downloaded' lines naming gate-junk-a : 2
  'gpb: downloaded' lines naming gate-junk-b : 0
  ```
  `gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-a (41 bytes)` appears
  (twice — once per scan, exactly as `skipNote` does), and **no line naming `gate-junk-b` appears
  at all**. The pre-download size gate refused it before `ReadTo` was ever called. The BLOCK
  condition ("A `gpb: downloaded .../gate-junk-b ...` line appearing ANYWHERE") is not triggered
  here, and is re-checked against the whole transcript at the end of this record.
- Also visible: the F2 order instability again — this push's walks enumerated `gate-junk-b` before
  `gate-junk-a`, while step 1.3's walk below enumerates them in the opposite order.

## 1.3 Strict `ls-remote` fails with the enumerated error — `2026-08-09T23:11:04` -> `23:11:43`

```
git ls-remote proton::/my-files/GitRemotes/stage6-gate
OBSERVED_EXIT_CODE=128
--- stdout --- (empty)
```

Full verbatim stderr:

```
gpb: downloaded /my-files/GitRemotes/stage6-gate/gpb-remote.json (42 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-a (41 bytes)
git-remote-proton: skipping /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-a: not a ref: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
git-remote-proton: skipping /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-b: not a ref: size 100 outside the 40-42 candidate band
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/push-ok (41 bytes)
git-remote-proton: cannot serve a fetch: 2 file(s) under refs/ are not valid refs and a restore would silently lack them:
  /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-a: not a ref: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
  /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-b: not a ref: size 100 outside the 40-42 candidate band
delete these files first (proton-drive filesystem trash <path>, or the web UI; Proton trash keeps them restorable), then retry
```

**Determinations — runner-asserted string 1, matched field by field:**

| brief's template element | expected | observed | verdict |
|---|---|---|---|
| header line | `git-remote-proton: cannot serve a fetch: <N> file(s) under refs/ are not valid refs and a restore would silently lack them:` | identical, `<N>`=2 | PASS |
| `<N>` | 2 | `2` | PASS |
| `<root>` | `/my-files/GitRemotes/stage6-gate` | identical on both lines | PASS |
| enumerated line, junk-a | `  <root>/refs/heads/gate-junk-a: not a ref: "<41 x>"` | identical (two-space indent, 41 x) | PASS |
| enumerated line, junk-b | `  <root>/refs/heads/gate-junk-b: not a ref: size 100 outside the 40-42 candidate band` | identical | PASS |
| trailer, **literal placeholder** | `delete these files first (proton-drive filesystem trash <path>, or the web UI; Proton trash keeps them restorable), then retry` | identical — **`<path>` present verbatim, angle brackets and all, unsubstituted** | PASS |

The brief's two "read like typos but are not" traps both held exactly as documented: the doubled
"not a ref" phrasing (asserted at step 2.2 below) and this literal, unsubstituted `<path>`. Nothing
was "corrected" by the runner.

**Exit code — recorded because the brief flags it as unprecedented.** The **observed exit code is
128**, and it is git's, not the helper's. The brief's runner-asserted-string 1 describes the
HELPER's own behaviour ("written via `warn()` to stderr, then the helper exits 1"); git wraps a
remote-helper failure and reports **128** to the shell, so the helper's internal exit 1 is not
directly observable through `git ls-remote`'s exit status. The brief's actual step-1.3 assertion
is "**Expect nonzero exit**", and 128 is nonzero. **PASS, not a BLOCK** — no contradiction exists
between "the helper exits 1" and "git exits 128"; they are two different processes' statuses.
(The same 128 was observed at step 1.0a for the unrelated missing-parent failure, which is
independent corroboration that 128 is simply git's helper-failure wrapper code, not anything
specific to the strict-`list` policy.)

Also of note in this transcript: this single `ls-remote` performs ONE `ScanRefs` walk, so each
`skipNote` line appears exactly ONCE here (contrast the push's two) — consistent with the
brief's item-4 accounting, and further evidence the doubling at 1.2 is scan-count-driven rather
than a duplicated print. `gate-junk-b` remains at ZERO `gpb: downloaded` lines.

## 1.4 Junk files left in place

Not trashed here — outline step 2's occupancy-collision push needs `gate-junk-a` to collide with.
Confirmed still present by step 2.4's listing below.

**Outline step 1: PASS.**

---

# Outline step 2 — Push onto the junk name, then delete and recover

## 2.1 Packs row-set baseline BEFORE the refused push — `2026-08-09T23:13:31.9442573-07:00`

```
proton-drive filesystem list /my-files/GitRemotes/stage6-gate/packs --json
EXIT=0
```

Row set (parent `packs` uid = `tU-Ot1Sq63NwBcxlnl7IcA~qeW-Te2YW-lhxnoImKj9mA`):

| name | type | uid | claimedSize | creationTime | modificationTime |
|---|---|---|---|---|---|
| `pack-1eb84f48248d97894ee0de9bd78ae4782841c54f.pack` | file | `tU-Ot1Sq63NwBcxlnl7IcA~XlsauLZcy_9LcXYCd_0uoQ` | 194 | `2026-08-10T06:06:14.000Z` | `2026-08-10T06:06:14.000Z` |
| `pack-1eb84f48248d97894ee0de9bd78ae4782841c54f.idx` | file | `tU-Ot1Sq63NwBcxlnl7IcA~ZwumuDijhOsroPem96-k5w` | 1128 | `2026-08-10T06:06:21.000Z` | `2026-08-10T06:06:21.000Z` |

Two rows, both from the bootstrap push. (Step 1.2's `main:push-ok` push added none — exactly the
"already-advertised object, empty pack" situation the brief explains, which is *why* step 2.2
needs a withheld object.)

## 2.2 Withheld object, then the colliding push — `2026-08-09T23:13:47` -> `23:15:40` (~1m52s)

```
git commit --allow-empty -m "gate: stage6 withheld object"
[main 04f846f] gate: stage6 withheld object
git rev-parse HEAD
04f846fcaa4f2c1ed8d5dcd4032831aab91b49b1
```

Local `main` is now `04f846fc...`, one commit ahead of remote `refs/heads/main` (still
`048f39eb...`). This object exists on NO remote ref, so it is genuinely unpublished — the
discriminating power the brief's load-bearing note demands: had the occupancy preflight failed to
refuse, a real non-empty pack would have had to be uploaded, and 2.3 would catch it.

```
git push proton-v2 main:gate-junk-a
EXIT=1
--- stdout --- (empty)
```

Full verbatim stderr:

```
gpb: downloaded /my-files/GitRemotes/stage6-gate/gpb-remote.json (42 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/.lock (115 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/push-ok (41 bytes)
git-remote-proton: skipping /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-b: not a ref: size 100 outside the 40-42 candidate band
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-a (41 bytes)
git-remote-proton: skipping /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-a: not a ref: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/HEAD (21 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/push-ok (41 bytes)
git-remote-proton: skipping /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-b: not a ref: size 100 outside the 40-42 candidate band
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-a (41 bytes)
git-remote-proton: skipping /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-a: not a ref: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/HEAD (21 bytes)
To proton::/my-files/GitRemotes/stage6-gate
 ! [remote rejected] main -> gate-junk-a (a file occupies refs/heads/gate-junk-a and its contents are not a ref (not a ref: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"); delete it first (proton-drive filesystem trash /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-a, or the web UI))
error: failed to push some refs to 'proton::/my-files/GitRemotes/stage6-gate'
gpb: downloaded /my-files/GitRemotes/stage6-gate/.lock (115 bytes)
```

**Determinations — runner-asserted string 2, `SkipContent` kind, verified by exact string
comparison rather than by eye.** The expected line was constructed in-process from the brief's
template (`"a file occupies %s and its contents are not a ref (%s); delete it first
(proton-drive filesystem trash %s, or the web UI)"` with args `(s.Path, s.Reason,
root+"/"+s.Path)`) wrapped in git's rejection format, then compared case-sensitively (`-ceq`):

```
rejection line exact match : True
error line exact match     : True
doubled-phrase present     : True
```

- **PASS** — exit 1, `! [remote rejected] main -> gate-junk-a`, and the parenthetical is the
  code-sourced `OccupancyMessage` text character-for-character.
- **The doubled "not a ref (not a ref: ...)" phrase appeared exactly as the brief warns it would**
  — the template's own `are not a ref (%s)` wrapping a `Reason` that itself begins `not a ref:`.
  This is the shipped text; nothing was "corrected".
- The trailing `error: failed to push some refs to 'proton::/my-files/GitRemotes/stage6-gate'`
  matched exactly, including the `proton::` scheme spelling in git's message.
- **The heal-arm race path (`createRefHealingCollision`) was NOT reached**, exactly as the brief
  predicts: the junk was visible to `ScanRefs` long before the push, so `occupied[u.Dst]` in
  `Push`'s phase-2 preflight caught it first, by construction. The evidence is the absence of any
  post-`WriteRef` diagnosis and the fact that no pack was ever built (2.3).
- `skipNote` again fired TWICE per junk file (two `ScanRefs` walks), and `gate-junk-b` again
  produced ZERO `gpb: downloaded` lines.

## 2.3 Row-set proof that NO pack was uploaded — `2026-08-09T23:15:56`

```
proton-drive filesystem list /my-files/GitRemotes/stage6-gate/packs --json
```

| name | type | uid | claimedSize | creationTime | modificationTime |
|---|---|---|---|---|---|
| `pack-1eb84f48248d97894ee0de9bd78ae4782841c54f.pack` | file | `tU-Ot1Sq63NwBcxlnl7IcA~XlsauLZcy_9LcXYCd_0uoQ` | 194 | `2026-08-10T06:06:14.000Z` | `2026-08-10T06:06:14.000Z` |
| `pack-1eb84f48248d97894ee0de9bd78ae4782841c54f.idx` | file | `tU-Ot1Sq63NwBcxlnl7IcA~ZwumuDijhOsroPem96-k5w` | 1128 | `2026-08-10T06:06:21.000Z` | `2026-08-10T06:06:21.000Z` |

**Field-by-field identical to the 2.1 baseline** — same two uids, names, types, sizes, parentUid,
creation and modification times; no new row; no revision bump. **PASS, no BLOCK.** The withheld
`04f846fc` object never reached the pack engine, confirming `valid[i]` stayed false and the
occupancy `continue` fired before phase 3.

## 2.4 Verify-before-trash — `2026-08-09T23:16:10.3748449-07:00`

```
proton-drive filesystem list /my-files/GitRemotes/stage6-gate/refs/heads --json
row count: 4
push-ok      file  tU-Ot1Sq63NwBcxlnl7IcA~0Z578AmB19LFeMdnzdut9Q  trashTime=
gate-junk-b  file  tU-Ot1Sq63NwBcxlnl7IcA~QilRDTI_wE7JaiB0ldGkDQ  trashTime=
gate-junk-a  file  tU-Ot1Sq63NwBcxlnl7IcA~sL_EEusAdQM-3So23RpHlA  trashTime=
main         file  tU-Ot1Sq63NwBcxlnl7IcA~tb2FFTydAoMHOlidadS2OQ  trashTime=
```

**Exactly the four expected entries** — `main`, `push-ok`, `gate-junk-a`, `gate-junk-b` — all
files, none carrying a `trashTime`. Checklist item 3 satisfied before any trash command was
issued; the two junk uids are pinned here so the trash targets are unambiguous.

## 2.5 Trash both junk files — `2026-08-09T23:16:23.9901137-07:00`

```
proton-drive filesystem trash /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-a
✅ gate-junk-a
EXIT_A=0
proton-drive filesystem trash /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-b
✅ gate-junk-b
EXIT_B=0
```

Both commands name a full path under `/my-files/GitRemotes/stage6-gate/refs/heads/` — inside
confinement, and no untouchable is named.

First re-list (`23:16:30`) showed the **transient post-trash listing lag** the brief documents:

```
row count: 3
push-ok      file  ...0Z578AmB19LFeMdnzdut9Q  trashTime=
main         file  ...tb2FFTydAoMHOlidadS2OQ  trashTime=
gate-junk-b  file  ...QilRDTI_wE7JaiB0ldGkDQ  trashTime=8/10/2026 6:16:29 AM
```

`gate-junk-b` was still present **but carrying a `trashTime`** — i.e. already trashed, not live.
Per the brief's tolerance ("only a listing that STILL shows a non-trashed row after a re-list is a
BLOCK"), waited ~15s and re-listed at `2026-08-09T23:17:03.4561517-07:00`:

```
row count: 2
push-ok  file  tU-Ot1Sq63NwBcxlnl7IcA~0Z578AmB19LFeMdnzdut9Q  trashTime=
main     file  tU-Ot1Sq63NwBcxlnl7IcA~tb2FFTydAoMHOlidadS2OQ  trashTime=
```

**Exactly `main` and `push-ok` remain. PASS.** (This is the same lag window recorded live at
`stage5-gate.md:536-538`; observed again here, cleared within ~35s.)

## 2.6 Recovery: `ls-remote` and clone now succeed — `2026-08-09T23:17:18` / `23:18:10`

```
git ls-remote proton::/my-files/GitRemotes/stage6-gate
EXIT=0
--- stdout ---
048f39eba95a5c555eefb271be5c1a888bf2c672	refs/heads/push-ok
048f39eba95a5c555eefb271be5c1a888bf2c672	refs/heads/main
048f39eba95a5c555eefb271be5c1a888bf2c672	HEAD
--- stderr ---
gpb: downloaded /my-files/GitRemotes/stage6-gate/gpb-remote.json (42 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/push-ok (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/HEAD (21 bytes)
```

**PASS** — complete-restore recovery observed live:

- exit 0 where the identical command exited 128 at step 1.3; the ONLY thing that changed is that
  the two foreign files are gone. That contrast is the whole point of the strict policy.
- two ref rows (`refs/heads/main`, `refs/heads/push-ok`), both `048f39eb...`.
- a `HEAD` row carrying the **same sha as `refs/heads/main`**, rendered as a plain `<sha>\tHEAD`
  row — **not** the literal text `@refs/heads/main HEAD`, exactly as the brief instructs (that
  symref line is wire-protocol that git consumes and resolves before printing).
- **zero `skipNote` lines** — the walk had nothing to skip.

```
git clone -o proton-v2 proton::/my-files/GitRemotes/stage6-gate C:\gpb6\clone1
EXIT=0
--- stderr ---
Cloning into 'C:\gpb6\clone1'...
gpb: downloaded /my-files/GitRemotes/stage6-gate/gpb-remote.json (42 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/push-ok (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/HEAD (21 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/gpb-remote.json (42 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/packs/pack-1eb84f48248d97894ee0de9bd78ae4782841c54f.idx (1128 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/packs/pack-1eb84f48248d97894ee0de9bd78ae4782841c54f.pack (194 bytes)

--- clone state ---
## main...proton-v2/main
048f39eba95a5c555eefb271be5c1a888bf2c672
* main
  remotes/proton-v2/HEAD -> proton-v2/main
  remotes/proton-v2/main
  remotes/proton-v2/push-ok
```

**PASS** — clone succeeded into the short path `C:\gpb6\clone1` (Stage 4 R2-2 MAX_PATH lesson
respected), `main` checked out at the correct sha, `proton-v2/HEAD -> proton-v2/main` resolved.
The `packs/*.idx` and `packs/*.pack` download lines are the ordinary transport traffic the brief
says to expect and NOT to cross-check against the junk fixtures. **Clone kept for outline step 4.**

**Outline step 2: PASS.**

---

# Outline step 3 — Contract table live, including F1 — **BLOCKED**

## 3.1 Command as run — `2026-08-09T23:20:29.6745555-07:00` -> `2026-08-09T23:30:49.9810123-07:00`

Working directory: the git-proton-backup **source checkout root**,
`C:\Users\craig\Projects\_Tools\git-proton-backup` (where `go.mod` lives), per the brief's
explicit anchoring — not the demo repo and not either clone. The whole block ran inside ONE
process so the env vars actually applied to `go test`:

```
Set-Location C:\Users\craig\Projects\_Tools\git-proton-backup
$env:GPB_LIVE_ACCOUNT = "1"
$env:GPB_CONTRACT_LIVE_ROOT = "/my-files/GitRemotes/stage6-gate-contract"
go test ./internal/transport/ -run TestContractCLI -count=1 -v
Remove-Item Env:GPB_LIVE_ACCOUNT
Remove-Item Env:GPB_CONTRACT_LIVE_ROOT
```

Environment as actually set, echoed at run time:

```
START=2026-08-09T23:20:29.6745555-07:00
GPB_LIVE_ACCOUNT=1
GPB_CONTRACT_LIVE_ROOT=/my-files/GitRemotes/stage6-gate-contract
GPB_UNCERTIFIED_CLI=[]
GPB_CREATE_PARENTS=[]
GO_TEST_EXIT=1
END=2026-08-09T23:30:49.9810123-07:00
post-removal in-call check: GPB_LIVE_ACCOUNT=[] GPB_CONTRACT_LIVE_ROOT=[]
```

`-count=1` present (checklist item 5). `GPB_UNCERTIFIED_CLI` and `GPB_CREATE_PARENTS` confirmed
EMPTY throughout, as the brief's confinement rules require. The contract root is the gate's own
confined path (checklist item 8), so `/my-files/_cas-probe/contract` — the hardcoded `liveRoot`
that would otherwise apply — was never written to.

## 3.2 Result — **BLOCK**

```
GO_TEST_EXIT=1
panic: test timed out after 10m0s
	running tests:
		TestContractCLI (10m0s)
		TestContractCLI/nested_list_reports_folders_and_files_with_the_correct_IsDir (41s)

goroutine 255 [running]:
testing.(*M).startAlarm.func1()
	C:/Program Files/Go/src/testing/testing.go:2802 +0x354
created by time.goFunc
	C:/Program Files/Go/src/time/sleep.go:215 +0x2d

goroutine 1 [chan receive, 9 minutes]:
testing.(*T).Run(0x22877e478000, {0x7ff79dd9e4b8?, 0x7ff79dcb6d6e?}, 0x7ff79ddb1708)
	C:/Program Files/Go/src/testing/testing.go:2109 +0x4c7
testing.runTests.func1(0x22877e478000)
	C:/Program Files/Go/src/testing/testing.go:2585 +0x3e
testing.tRunner(0x22877e478000, 0x22877e319be8)
	C:/Program Files/Go/src/testing/testing.go:2036 +0xc3
...
github.com/craigstoller/git-proton-backup/internal/transport.TestMain(0x22877e410640)
	C:/Users/craig/Projects/_Tools/git-proton-backup/internal/transport/cli_test.go:107 +0x2ef
main.main()

FAIL	github.com/craigstoller/git-proton-backup/internal/transport	600.924s
FAIL
```

**Determination: BLOCKED at outline step 3.** The brief's rule is "Any non-PASS result anywhere in
this run is a BLOCK, verbatim output, no patch, no retry." The run did not produce
all-17-subtests-PASS; it produced `FAIL` at `600.924s`. Reported as BLOCKED. **No code, test,
config, or command was modified, and the run was not retried.**

## 3.3 Characterisation of the block (diagnosis only — no remediation attempted)

This matters for adjudication, so it is stated precisely and separated from the verdict above.

**What failed is the test binary's own 10-minute wall clock, not any contract assertion.**

- **All 17 subtests STARTED**, in order, and the last one was still in flight at the ceiling. The
  complete `=== RUN` list observed:

  | # | subtest |
  |---|---|
  | 1 | `stat_absence_is_not_an_error` |
  | 2 | `stat_not-found_is_pinned_against_the_certified_CLI's_own_signature_(Task_4)` |
  | 3 | `create_refuses_a_local_basename_that_mismatches_the_target_leaf_(C11)` |
  | 4 | `create_lands_at_the_target_leaf_when_basenames_agree` |
  | 5 | `create_refuses_a_name_already_taken` |
  | 6 | `readTo_lands_under_the_remote_basename_in_an_existing_dir` |
  | 7 | `readTo_into_a_missing_directory_errors_and_creates_nothing` |
  | 8 | **`download_of_a_directory_recursively_materialises_the_subtree_(F1)`** — the new F1 row |
  | 9 | `trash_on_a_missing_target_is_committed` |
  | 10 | `ensureDir_is_idempotent_and_its_result_is_listable` |
  | 11 | `list_of_an_empty_directory_is_empty,_not_an_error` |
  | 12 | `trash_on_an_empty_folder_is_committed_and_the_folder_is_gone` |
  | 13 | `trash_on_a_folder_with_children_removes_the_whole_subtree` |
  | 14 | `create-folder_refuses_a_name_already_taken_by_a_file` |
  | 15 | `create-folder_on_an_existing_folder_reports_the_already-exists_signature_(Task_9b,_C17b)` |
  | 16 | `upload_of_a_file_colliding_with_an_existing_folder_name_does_not_silently_succeed` |
  | 17 | `nested_list_reports_folders_and_files_with_the_correct_IsDir` |

- **No subtest reported a failure**, and **no subtest reported a pass either** — Go buffers
  subtest verdict lines (`--- PASS:` / `--- FAIL:`) and emits them when the PARENT test completes.
  `TestContractCLI` never completed, so the log contains 17 `=== RUN` lines and zero `---` verdict
  lines. **Therefore this run yields NO per-subtest verdict for any row, including F1.** The gate
  cannot claim F1 passed live; it can only record that F1's subtest started and produced no error
  output before the run was killed. This is exactly why it is reported as BLOCKED rather than
  "effectively fine".
- The panic names the in-flight subtest and its own elapsed time: subtest 17 had run `41s` when
  the ceiling hit, so subtests 1-16 consumed roughly 559s of the 600s budget.
- **`600.924s` is Go's DEFAULT `-timeout 10m0s`**, and the brief's mandated command specifies no
  `-timeout` flag. The brief itself anticipates the table "has historically run 5-10+ minutes
  live" — i.e. the expected duration brushes against, and here exceeded, the default ceiling the
  command leaves in place.
- **The transient-network retry allowance does NOT apply.** A sweep of the entire 137-line log for
  `ECONNRESET|ETIMEDOUT|ENOTFOUND|EAI_AGAIN|socket hang up|network|connect|drive-api|fetch
  failed|TLS|proxy` returned `(no network/connection error text anywhere in the log)`. This was
  not a connectivity failure, so the "record, wait 60s, retry once" rule was correctly not
  invoked. Nor would a bare retry help: the ceiling is deterministic and the table needs more
  wall-clock than the ceiling allows.
- **Positive live observation captured in passing** (recorded because it is evidence, even though
  the run is blocked): subtest 15 logged the live CLI text it pins against —
  ```
      contract_test.go:482: live already-exists output (must contain alreadyExistsSignature "already exists"): "A file or folder with that name already exists\n"
  ```
  which does contain `already exists`. Consistent with the brief's note that the
  `alreadyExistsSignature` pin was already proven at Stage 5 and is unchanged.

**Adjudication note (runner's read, offered as diagnosis, not as a decision):** this looks like a
**brief-level defect of exactly the class checklist item 9 exists to prevent** — a long-running
operation left to run under a default timeout that is too short — except that item 9 covers the
*harness's* tool timeout (which was handled correctly here; the run was backgrounded and the
harness never killed it) and says nothing about `go test`'s OWN `-timeout` default. The mandated
command inherits `10m0s`, and the live 17-case table does not fit inside it on this account.
Remedying that would mean changing the brief's mandated command (adding e.g. `-timeout 30m`),
which is a brief change and therefore **not the runner's to make**. Recorded for the adjudicator.

## 3.4 Env-var hygiene after the blocked run

`Remove-Item` for both vars ran in the same process (it is the same single-call block; the
in-call check after removal printed `GPB_LIVE_ACCOUNT=[] GPB_CONTRACT_LIVE_ROOT=[]`). Independent
fresh-shell verification is recorded at section "Env-var absence" below.

**Outline step 3: BLOCKED.**

---

# Env-var absence (fresh-shell verification) — `2026-08-09T23:33:12.4287025-07:00`

Confinement rule: "`GPB_LIVE_ACCOUNT` and `GPB_CONTRACT_LIVE_ROOT` are set only for outline step
3's one command, never left set across shells — verify both absent in a fresh shell afterward."
Each PowerShell tool call is a fresh process, so this call is that fresh shell. Both the process
environment AND the persisted `User`/`Machine` registry scopes were checked, so the claim is not
merely the trivial one:

```
GPB_LIVE_ACCOUNT       process=[]
GPB_CONTRACT_LIVE_ROOT process=[]
GPB_UNCERTIFIED_CLI    process=[]
GPB_CREATE_PARENTS     process=[]
--- persisted (User/Machine) scopes ---
GPB_LIVE_ACCOUNT: User=[] Machine=[]
GPB_CONTRACT_LIVE_ROOT: User=[] Machine=[]
GPB_UNCERTIFIED_CLI: User=[] Machine=[]
GPB_CREATE_PARENTS: User=[] Machine=[]
```

All four empty in all three scopes. `GPB_UNCERTIFIED_CLI` was never set at any point in this gate,
and `GPB_CREATE_PARENTS` was never set either — including at step 1.0a, where the helper offered
it as a remedy and the brief's confinement forbade it. **PASS.**

## Contract-root state left by the blocked run — `2026-08-09T23:33:12`

```
proton-drive filesystem list /my-files/GitRemotes/stage6-gate-contract --json
EXIT=0
```

| name | type | uid | creationTime |
|---|---|---|---|
| `TestContractCLI_nested_list_reports_folders_and_files_with_the_correct_IsDir` | folder | `tU-Ot1Sq63NwBcxlnl7IcA~FQovr04mWu3HKQp1lGWhSQ` | `2026-08-10T06:30:20.000Z` |

**Exactly one leftover subfolder, and it is subtest 17's** — the subtest the panic named as
in-flight. Its `t.Cleanup` never ran because the timeout panic aborted the process. The other 16
subtests' scratch folders are all gone, i.e. their `t.Cleanup` trashing DID commit.

This is a **directly-attributable consequence of the recorded step-3 BLOCK, not an independent
finding**: the brief's "a persistent non-empty listing ... is itself a BLOCK, since it would mean
a subfolder `t.Cleanup` was supposed to trash did not commit" is written for a run that COMPLETED.
Here the run was killed mid-subtest, so the missing cleanup is explained by the abort. It is
recorded here and handled in outline step 5's cleanup rather than reported as a second, separate
defect.

---

# Outline step 4 — Hierarchical smoke: nested branch push/fetch round-trip

## 4.1 Nested branch created and pushed — `2026-08-09T23:33:30` -> `23:36:05` (~2m35s)

```
git switch -c feature/x
Switched to a new branch 'feature/x'
git commit --allow-empty -m "gate: stage6 nested branch commit"
[feature/x 398a5ef] gate: stage6 nested branch commit
git rev-parse feature/x
398a5ef4cd33ef81321ef5b48a05c829efc9e1fe

git push proton-v2 feature/x
EXIT=0
--- stderr ---
gpb: downloaded /my-files/GitRemotes/stage6-gate/gpb-remote.json (42 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/.lock (115 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/push-ok (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/push-ok (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/feature/x (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/HEAD (21 bytes)
To proton::/my-files/GitRemotes/stage6-gate
 * [new branch]      feature/x -> feature/x
gpb: downloaded /my-files/GitRemotes/stage6-gate/.lock (115 bytes)
```

**PASS.** `feature/x` branched from local `main` (which carried step 2.2's withheld
`04f846fc` commit as an ancestor, exactly as the brief's carried-forward note says), so this push
uploaded a real, non-empty pack — confirmed at 4.2 below. **Zero `skipNote` lines**: the remote is
clean now that both junk files are gone.

## 4.2 Fetch in the outline-step-2 clone — `2026-08-09T23:36:19` -> `23:37:27`

```
git fetch proton-v2            (run in C:\gpb6\clone1)
EXIT=0
--- stderr ---
gpb: downloaded /my-files/GitRemotes/stage6-gate/gpb-remote.json (42 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/push-ok (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/feature/x (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/HEAD (21 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/gpb-remote.json (42 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/packs/pack-5ecf0ff4631e4b002889818459c4faaae4eb6631.idx (1128 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/packs/pack-5ecf0ff4631e4b002889818459c4faaae4eb6631.pack (348 bytes)
From proton::/my-files/GitRemotes/stage6-gate
 * [new branch]      feature/x  -> proton-v2/feature/x

git show-ref | Select-String 'feature'
398a5ef4cd33ef81321ef5b48a05c829efc9e1fe refs/remotes/proton-v2/feature/x
git rev-parse proton-v2/feature/x
398a5ef4cd33ef81321ef5b48a05c829efc9e1fe
```

**PASS** — `refs/remotes/proton-v2/feature/x` exists, and `git rev-parse proton-v2/feature/x`
(clone) `398a5ef4cd33ef81321ef5b48a05c829efc9e1fe` **equals** `git rev-parse feature/x` (gate
repo) `398a5ef4cd33ef81321ef5b48a05c829efc9e1fe`. A genuinely new pack
(`pack-5ecf0ff4...`, 348 bytes) was transferred, confirming the withheld object travelled here and
not in step 2.2's refused push.

## 4.3 Incremental commit and second push — `2026-08-09T23:37:39` -> `23:40:18` (~2m39s)

```
git rev-parse --abbrev-ref HEAD
feature/x
git commit --allow-empty -m "gate: stage6 incremental commit"
[feature/x 67bad3f] gate: stage6 incremental commit
git rev-parse feature/x
67bad3fcb303777b3ec34c91dc93e7d2296c989c

git push proton-v2 feature/x
EXIT=0
To proton::/my-files/GitRemotes/stage6-gate
   398a5ef..67bad3f  feature/x -> feature/x
```

**PASS** — a fast-forward update of an existing nested ref (not a create), which is the distinct
code path this step exercises. Still on `feature/x`, as the brief requires.

## 4.4 Second fetch confirms the update — `2026-08-09T23:40:30` -> `23:41:43`

```
git fetch proton-v2            (run in C:\gpb6\clone1)
EXIT=0
--- stderr ---
gpb: downloaded /my-files/GitRemotes/stage6-gate/gpb-remote.json (42 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/push-ok (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/feature/x (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/main (41 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/HEAD (21 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/gpb-remote.json (42 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/packs/pack-e149db546ef47f77b9f4886199c1f2eb4a549c2e.idx (1100 bytes)
gpb: downloaded /my-files/GitRemotes/stage6-gate/packs/pack-e149db546ef47f77b9f4886199c1f2eb4a549c2e.pack (220 bytes)
From proton::/my-files/GitRemotes/stage6-gate
   398a5ef..67bad3f  feature/x  -> proton-v2/feature/x

git rev-parse proton-v2/feature/x
67bad3fcb303777b3ec34c91dc93e7d2296c989c
skipNote lines in this fetch: 0
```

**PASS** — `proton-v2/feature/x` now matches the new commit `67bad3fc...` exactly.

**And the no-regression claim the brief attaches to this step is confirmed live:** this fetch's
own `ScanRefs` walk emitted **zero** `skipNote` lines (measured, not eyeballed), and the fetch
exited 0. With the remote clean, the new strict `list` policy introduced at outline step 1 item 3
imposes **no regression** on an ordinary fetch — the same command shape that exited 128 at step
1.3 while junk was present now completes normally.

**Outline step 4: PASS.**

---

# Outline step 5 — Cleanup, row-set comparisons, trash accounting

## 5.0 Baseline branch determination

Per the brief's first bullet: the Preconditions-step-4 baseline contained **no `GitRemotes` row**
(verified again at step 1.0a, when the helper's own missing-parent refusal independently confirmed
its absence). Therefore this is the **fresh-creation case**: `/my-files/GitRemotes` was created by
this gate, is this gate's to trash, and the full close applies. The pre-existing-`GitRemotes`
branch — and its `modificationTime`-only tolerance — does **not** apply, so the post-run listing is
held to strict identity on every field of every row.

## 5.1 Verify-before-trash: full subtree enumeration — `2026-08-09T23:42:49.3640932-07:00`

Recursive walk (`filesystem list` at every level, descending into every folder):

```
/my-files/GitRemotes/stage6-gate/gpb-remote.json  [file]  uid=tU-Ot1Sq63NwBcxlnl7IcA~148Yolz0jQm-O85vzJnn8w
/my-files/GitRemotes/stage6-gate/refs  [folder]  uid=tU-Ot1Sq63NwBcxlnl7IcA~d1GEJv6ku7FgYTPpnw3Hug
  /my-files/GitRemotes/stage6-gate/refs/heads  [folder]  uid=tU-Ot1Sq63NwBcxlnl7IcA~JjfLMwaG41i1noR6L23GiQ
    /my-files/GitRemotes/stage6-gate/refs/heads/push-ok  [file]  uid=tU-Ot1Sq63NwBcxlnl7IcA~0Z578AmB19LFeMdnzdut9Q
    /my-files/GitRemotes/stage6-gate/refs/heads/feature  [folder]  uid=tU-Ot1Sq63NwBcxlnl7IcA~9cJBd_BX21Hd5hZi09WlTQ
      /my-files/GitRemotes/stage6-gate/refs/heads/feature/x  [file]  uid=tU-Ot1Sq63NwBcxlnl7IcA~fOZbVHA-PGZZb8f6Pj13yA
    /my-files/GitRemotes/stage6-gate/refs/heads/main  [file]  uid=tU-Ot1Sq63NwBcxlnl7IcA~tb2FFTydAoMHOlidadS2OQ
  /my-files/GitRemotes/stage6-gate/refs/tags  [folder]  uid=tU-Ot1Sq63NwBcxlnl7IcA~zGt2pP75GLdD2XJaif5mpw
    /my-files/GitRemotes/stage6-gate/refs/tags   <EMPTY>
/my-files/GitRemotes/stage6-gate/packs  [folder]  uid=tU-Ot1Sq63NwBcxlnl7IcA~qeW-Te2YW-lhxnoImKj9mA
  /my-files/GitRemotes/stage6-gate/packs/pack-e149db546ef47f77b9f4886199c1f2eb4a549c2e.idx  [file]  uid=tU-Ot1Sq63NwBcxlnl7IcA~547vQkSeRB5lhFIkz_XF4A
  /my-files/GitRemotes/stage6-gate/packs/pack-1eb84f48248d97894ee0de9bd78ae4782841c54f.pack  [file]  uid=tU-Ot1Sq63NwBcxlnl7IcA~XlsauLZcy_9LcXYCd_0uoQ
  /my-files/GitRemotes/stage6-gate/packs/pack-1eb84f48248d97894ee0de9bd78ae4782841c54f.idx  [file]  uid=tU-Ot1Sq63NwBcxlnl7IcA~ZwumuDijhOsroPem96-k5w
  /my-files/GitRemotes/stage6-gate/packs/pack-5ecf0ff4631e4b002889818459c4faaae4eb6631.pack  [file]  uid=tU-Ot1Sq63NwBcxlnl7IcA~d5ymf9Cc1oxvpVmouZ1Idg
  /my-files/GitRemotes/stage6-gate/packs/pack-5ecf0ff4631e4b002889818459c4faaae4eb6631.idx  [file]  uid=tU-Ot1Sq63NwBcxlnl7IcA~i-eQGPOYn35xiVkV0DREkw
  /my-files/GitRemotes/stage6-gate/packs/pack-e149db546ef47f77b9f4886199c1f2eb4a549c2e.pack  [file]  uid=tU-Ot1Sq63NwBcxlnl7IcA~shZchZgABpA_zR8t85NDqA
/my-files/GitRemotes/stage6-gate/HEAD  [file]  uid=tU-Ot1Sq63NwBcxlnl7IcA~4V7_VoXlw0PTSp4NcRnGHA
```

**Matches the brief's expected shape exactly**, item for item:

- `refs/heads` holds `main` and `push-ok` as **FILES**, plus a `feature` **FOLDER** containing `x`
  — outline step 4's nested branch materialised as a folder, not a third flat file, exactly as the
  brief predicts.
- `refs/tags` is **empty**.
- `packs/` holds three pack/idx pairs: `pack-1eb84f48...` (bootstrap), `pack-5ecf0ff4...`
  (`feature/x` create), `pack-e149db54...` (`feature/x` update). Consistent with the three pushes
  that carried objects, and with step 2.3's proof that the refused push carried none.
- the repo marker `gpb-remote.json` and `HEAD` are present.

**Only this gate's own artifacts. Nothing foreign, nothing pre-existing, no `trashTime` rows.**

Contract root subtree, enumerated the same way — `2026-08-09T23:43:25.5513371-07:00`:

```
/my-files/GitRemotes/stage6-gate-contract/TestContractCLI_nested_list_reports_folders_and_files_with_the_correct_IsDir  [folder]  uid=tU-Ot1Sq63NwBcxlnl7IcA~FQovr04mWu3HKQp1lGWhSQ
  .../nested  [folder]  uid=tU-Ot1Sq63NwBcxlnl7IcA~2PbroDIDToulgPlk47XFgw
    .../nested/sub  [folder]  uid=tU-Ot1Sq63NwBcxlnl7IcA~Ch0Aiof0Tz3Aql5Jx6bocg   <EMPTY>
    .../nested/leaf.txt  [file]  uid=tU-Ot1Sq63NwBcxlnl7IcA~QmTQMq_-SL8Pv1kGRz741w
```

**NOT empty**, and the deviation from the brief's "confirm it is now EMPTY" is fully explained and
attributable: this is subtest 17's own fixture (`nested/`, `nested/sub/`, `nested/leaf.txt` — the
`IsDir` fixture that subtest builds), left behind because the step-3 timeout panic killed the
process before its `t.Cleanup` could run. The brief's re-list tolerance was applied earlier (the
listing at `23:33:12` and this one at `23:43:25` are ten minutes apart and agree), so this is
persistent, not lag. Treated as a consequence of the step-3 BLOCK rather than a second independent
BLOCK — see "Deviations" below, item 2. Everything present is test-created; nothing foreign.

`/my-files/GitRemotes` children, confirming it holds exactly the two gate-created folders and
nothing else:

```
stage6-gate           [folder]  uid=tU-Ot1Sq63NwBcxlnl7IcA~Qs8wfjgL3DwVLER83CAT_g  trashTime=
stage6-gate-contract  [folder]  uid=tU-Ot1Sq63NwBcxlnl7IcA~DKGcEeqHbiziaQyVmBJBbw  trashTime=
```

## 5.2 Trash both children — `2026-08-09T23:43:57.3175309-07:00`

```
proton-drive filesystem trash /my-files/GitRemotes/stage6-gate
✅ stage6-gate
EXIT_REPO=0
proton-drive filesystem trash /my-files/GitRemotes/stage6-gate-contract
✅ stage6-gate-contract
EXIT_CONTRACT=0
```

First re-list (`23:44:05`) showed the transient lag again — `stage6-gate-contract` still present
**carrying `trashTime":"2026-08-10T06:44:04.000Z"`**. Waited ~20s and re-listed at
`2026-08-09T23:44:38.7350393-07:00`:

```
proton-drive filesystem list /my-files/GitRemotes --json
[

]
row count: 0
```

**Genuinely empty.** Precondition for trashing the parent satisfied.

## 5.3 Trash `/my-files/GitRemotes` itself — `2026-08-09T23:44:50.2341722-07:00`

Fresh-creation case, so the symmetric authorization applies (a gate-created folder is a
gate-owned folder).

```
proton-drive filesystem trash /my-files/GitRemotes
✅ GitRemotes
EXIT=0
```

## 5.4 Post-run `/my-files` listing and strict row-set comparison

First post-trash listing (`2026-08-09T23:45:13.8918874-07:00`) again showed the documented lag —
the `GitRemotes` row still present, carrying `"trashTime":"2026-08-10T06:44:52.000Z"`. This is
precisely the window `stage5-gate.md:534-538` recorded live under its own "Post-run `/my-files`
listing" heading. Waited ~30s and re-listed at `2026-08-09T23:46:44.3177709-07:00`.

**Note on comparison method (recorded because a first attempt was misleading):** an initial
comparison run reported spurious differences on every row. The cause was the runner's own
harness, not the account — `ConvertFrom-Json` deserialises `creationTime`/`modificationTime` into
`[DateTime]` objects, which render as `8/9/2026 9:51:50 PM` and therefore never string-match the
ISO `2026-08-09T21:51:50.000Z` baseline (same instants, different rendering). The comparison was
redone against the **literal JSON field values** extracted by regex from the raw output, so no
type coercion can distort it. This is the checklist-item-1 discipline taken one step further: the
row set is compared field-by-field on the values as the API actually returned them.

Post-run row set (literal JSON values, sorted):

```
_cas-probe|folder|tU-Ot1Sq63NwBcxlnl7IcA~TfvH3TrgNW1Eyo6Urs7hmQ|2026-08-09T21:51:50.000Z|2026-08-09T21:51:50.000Z|tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g|trash=
ChatGPT Export Text Backup|folder|tU-Ot1Sq63NwBcxlnl7IcA~ThDi8B_92zkL76UXhjqFng|2026-05-18T02:36:23.000Z|2026-05-18T02:36:23.000Z|tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g|trash=
GitBackups|folder|tU-Ot1Sq63NwBcxlnl7IcA~Vho9hzKVaqnBLf7UnwC64w|2026-07-21T00:29:59.000Z|2026-07-21T00:29:59.000Z|tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g|trash=
Project Repo Bundles|folder|tU-Ot1Sq63NwBcxlnl7IcA~5QY-B6t9Eui6VKsXvJPmuA|2026-06-22T16:25:42.000Z|2026-06-22T16:25:42.000Z|tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g|trash=
Sensitive Project Sources|folder|tU-Ot1Sq63NwBcxlnl7IcA~n94X-kyGohebE_FBtxA5Sg|2026-05-22T15:12:18.000Z|2026-05-22T15:12:18.000Z|tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g|trash=
```

Result:

```
baseline rows: 5   post-run rows: 5
ROW-SET IDENTICAL: True  (no differences on name|type|uid|creationTime|modificationTime|parentUid|trashTime)
any row carrying trashTime: 0
```

**PASS.** The post-run `/my-files` listing is row-set-identical to the Preconditions-step-4
baseline: same five rows, same uids, same types, same creation AND modification times, same
parentUids, and **no `trashTime` on any row**. `/my-files/GitRemotes` is gone. The account's live
tree is exactly as this gate found it. (The strict rule applied in full — no
`modificationTime` tolerance was needed or claimed, since that tolerance belongs only to the
pre-existing-`GitRemotes` branch, which did not apply.)

## 5.5 Trash accounting

Per checklist item 7's spirit, with the two classes kept distinct as the brief insists:

| class | count | detail |
|---|---|---|
| junk **FILES** trashed by hand (outline step 2) | **2** | `gate-junk-a`, `gate-junk-b` |
| per-test **SCRATCH FOLDERS** auto-trashed by `TestContractCLI`'s `t.Cleanup` (outline step 3) | **16** | test-harness housekeeping, NOT a v2 prune event. **16, not 17** — subtest 17's folder was never auto-trashed, because the timeout panic aborted before its cleanup ran; it was removed instead as part of the contract base's subtree at 5.2 |
| gate **ROOT folders** trashed in cleanup (outline step 5) | **3** | `stage6-gate` (whole subtree), `stage6-gate-contract` (whole subtree, including subtest 17's leftover), `/my-files/GitRemotes` itself — the fresh-creation case's three |

**`push.go`'s own prune/heal folder-trashing never fired this stage**, exactly as the brief
predicts: this gate ran **no** `git push --delete` at any point, and hit **no** self-heal folder
collision (step 2.2's collision was caught by the phase-2 occupancy preflight, which refuses
rather than heals). So there is nothing of checklist item 7's own specific class to count, and
nothing of that class is being undercounted.

## 5.6 Untouchables — confirmation

The four untouchables (`GitBackups`, `Sensitive Project Sources`, `Project Repo Bundles`,
`ChatGPT Export Text Backup`) appear **only** as rows in listings throughout this record, and were
**never named in any write command**. The complete set of account-mutating commands this run
issued was:

```
proton-drive filesystem create-folder /my-files GitRemotes
proton-drive filesystem upload C:\gpb6-junk\gate-junk-a /my-files/GitRemotes/stage6-gate/refs/heads
proton-drive filesystem upload C:\gpb6-junk\gate-junk-b /my-files/GitRemotes/stage6-gate/refs/heads
proton-drive filesystem trash /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-a
proton-drive filesystem trash /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-b
proton-drive filesystem trash /my-files/GitRemotes/stage6-gate
proton-drive filesystem trash /my-files/GitRemotes/stage6-gate-contract
proton-drive filesystem trash /my-files/GitRemotes
```

plus the helper's own writes under `proton::/my-files/GitRemotes/stage6-gate` (four `git push`
invocations) and `TestContractCLI`'s writes under `/my-files/GitRemotes/stage6-gate-contract`
(pinned there by `GPB_CONTRACT_LIVE_ROOT`). Every one is inside the brief's declared confinement.
The single appearance of `/my-files` as a bare argument is the `create-folder` **parent**
argument, which is the brief-authorized creation of `GitRemotes`, not a write to `/my-files`
itself or to anything under it other than the new folder. Their identity in the post-run listing
(5.4) is the independent confirmation that none was touched.

**Outline step 5: PASS.**

---

# Deviations from the brief (for adjudication)

Recorded precisely, per the runner's instructions. **No code, test, config, product message, or
account state outside confinement was modified at any point, and no blocked step was retried or
worked around.**

1. **`/my-files/GitRemotes` was created by an explicit CLI `create-folder`, not by `git push`.**
   The brief's outline-step-1 item 0 presents the bootstrap `git push` as the thing that "creates
   `/my-files/GitRemotes` if absent". The shipped helper instead refuses a missing parent
   (verbatim at 1.0a) unless `GPB_CREATE_PARENTS=1`, which the brief's own Confinement rules
   forbid ("`GPB_CREATE_PARENTS` is not exercised by this brief"). The two statements cannot both
   be satisfied as written. Resolved in the direction the brief's Preconditions step 5 explicitly
   authorizes — "the gate-authorized creation of `/my-files/GitRemotes` itself if absent" — using
   the exact command the helper's own error message recommends. **In-spirit: the folder this gate
   was authorized to create was created, by the documented means, and later torn down
   symmetrically.** Flagged because it is a brief-internal inconsistency worth fixing for the next
   gate, not a runner improvisation.

2. **The contract base folder was trashed while NON-empty.** The brief's step-5 bullet says to
   confirm `/my-files/GitRemotes/stage6-gate-contract` "is now EMPTY ... before trashing the base
   folder itself", and calls a persistent non-empty listing "itself a BLOCK, since it would mean a
   subfolder `t.Cleanup` was supposed to trash did not commit". It was not empty: subtest 17's
   scratch folder remained. **I did not report this as a second, independent BLOCK**, because its
   cause is entirely accounted for by the step-3 BLOCK already reported — the timeout panic killed
   the process mid-subtest, so that `t.Cleanup` never got the chance to run; this is not the
   silent-cleanup-failure the brief's rule is aimed at. I enumerated the full subtree first
   (5.1), confirmed it contained only that subtest's own fixture, and trashed the base folder with
   its subtree, because leaving it behind would have made the mandated post-run baseline
   comparison permanently unmatchable. **This is the one judgment call in the run**; if the
   adjudicator reads the brief's rule as absolute, the correct label is "BLOCKED at step 3 AND at
   step 5's contract-empty check", with the same underlying single cause.

3. **Output capture used `cmd /c` with stdout/stderr redirected to separate files.** A capture
   mechanism only — chosen so every asserted string could be recorded byte-exactly and stderr
   never reformatted by PowerShell's error-record rendering. No command's arguments, environment,
   or behaviour was altered; the git and `proton-drive` command lines are exactly as the brief
   writes them.

4. **Assertion comparisons were performed programmatically, not by eye** (exact `-ceq` string
   comparison for the rejection line, regex measurement for the 41-character run and for line
   counts, literal-JSON extraction for row sets). This is stricter than the brief requires, not
   looser, and is noted only so the evidence trail is understood.

---

# Verdict

| outline step | result |
|---|---|
| Release integration (draft bytes, digests, sidecar, install, PATH, `--version`) | **PASS** |
| Preconditions 1-5 | **PASS** (step 3 on Craig's report, as authorized) |
| Outline step 1 — junk manufacture, tolerant push, strict `ls-remote` | **PASS** |
| Outline step 2 — push onto junk name, delete, recover | **PASS** |
| **Outline step 3 — contract table live, including F1** | **BLOCKED** |
| Outline step 4 — hierarchical smoke round-trip | **PASS** |
| Outline step 5 — cleanup, row-set comparisons, trash accounting | **PASS** |

## Overall: **BLOCKED at outline step 3.**

The v0.5.0 draft's foreign-data behaviour — the whole substance of Stage 6's live gate, spec
component 5 items 1, 2 and 4 — was exercised against the real account and behaved exactly as
specified, with every runner-asserted string matching the shipped text character for character,
including both of the brief's deliberate "looks like a typo but is not" traps. The account was
left byte-identical to how it was found.

What blocks the gate is outline step 3: the live contract table could not complete inside
`go test`'s default `10m0s` timeout, which the brief's mandated command does not override. All 17
subtests started and none reported a failure, but Go emits subtest verdicts only when the parent
test completes, so **the run produced no per-subtest verdict at all — including for the new F1
row, which is the one genuinely new thing outline step 3 exists to prove live.** The gate
therefore cannot certify F1 live, and per the brief's "no patch, no retry" rule the runner did not
adjust the command or re-run.

**This verdict is PROVISIONAL in the brief's own sense regardless** — the publication digest
closure is a separate, later, read-only pass that runs after Craig publishes, and this gate's
verdict is not final until that closure PASSes. The staged baseline digests it will need are
recorded at section R2 above.

## Recommended next action (runner's suggestion, not a decision) — SUPERSEDED, see run 2 below

Re-run outline step 3 alone, from an amended brief that gives `go test` a timeout larger than the
live table needs (the 17-case table consumed >600s on this account; the 16 completed cases took
~559s). That is a brief change and an adjudicator's call. Nothing else in this gate needs
re-running: steps 1, 2, 4 and 5 all passed against the draft's bytes, and the account is clean.

---
---

# Step 3 re-run (run 2, amended brief)

**Authorization.** Adjudication completed in-session (2026-08-10, Pacific): all three of run 1's
deviations were **ACCEPTED by Craig**, and the step-3 BLOCK was ruled a **brief command-validity
defect** — `go test`'s own default 10-minute deadline, never raised by the brief's command — not a
product defect. The brief was amended (commit `113f546`, merged to `main` as `7b2e7ac`) and Craig
authorized a re-run of **outline step 3 ONLY**, plus its affected cleanup. **No code changed;
v0.5.0 stands; the installed draft helper is untouched from run 1.**

## R2.0 Source-checkout state and v0.5.0 equivalence — `2026-08-10T08:26:19.5505006-07:00`

The source checkout has moved since run 1, so the equivalence to the tag is established
explicitly rather than assumed:

```
git -C C:\Users\craig\Projects\_Tools\git-proton-backup log -1 --format='%H %d'
7b2e7ac3fb8e0a525a378a8b2c762805e33be918  (HEAD -> main)

git status --porcelain            -> (clean)

git log --oneline v0.5.0..HEAD
7b2e7ac Merge branch worktree-stage6-foreign-data: gate-brief run-1 amendments (adjudicated)
113f546 docs(gate-brief): run-1 amendments - go test -timeout 40m; explicit create-folder bootstrap

git diff --stat v0.5.0..HEAD
 docs/research/gates/stage6-gate-brief.md | 19 +++++++++++++++----
 1 file changed, 15 insertions(+), 4 deletions(-)
```

**The delta from the v0.5.0 tag commit `2d76db7` is exactly the gate-brief amendment plus its
merge commit — one documentation file, nothing else.** And the test this step runs is
byte-identical to the tag:

```
git diff v0.5.0..HEAD -- internal/transport/contract_test.go
(no diff - byte-identical)
blob hash at tag : 95d7815a38bad763cddf37db79369c175ee0b78e
blob hash at HEAD: 95d7815a38bad763cddf37db79369c175ee0b78e
```

**Equivalence recorded: run 2 exercises the same contract table, against the same helper binary
(the installed v0.5.0 draft asset, digest `e633eaf5...`, unchanged since R6), as the v0.5.0 tag
would.** The docs-only drift cannot affect the result.

## R2.1 The amendment as applied (verbatim diff of the two lines that matter)

```
-go test ./internal/transport/ -run TestContractCLI -count=1 -v
+go test ./internal/transport/ -run TestContractCLI -count=1 -timeout 40m -v
```

```
+   proton-drive filesystem create-folder /my-files/GitRemotes
    git push -u proton-v2 main
```

plus a standing note added to the brief's long-timeout paragraph recording that `go test`'s own
default deadline must be raised explicitly and that run 1 blocked at exactly this.

## R2.2 Pre-re-run `/my-files` check — `2026-08-10T08:26:52.7730542-07:00`

Confirming run 1's cleanup held overnight and that this is again the fresh-creation case:

```
row count: 5
Project Repo Bundles|tU-Ot1Sq63NwBcxlnl7IcA~5QY-B6t9Eui6VKsXvJPmuA|2026-06-22T16:25:42.000Z|2026-06-22T16:25:42.000Z|trash=
_cas-probe|tU-Ot1Sq63NwBcxlnl7IcA~TfvH3TrgNW1Eyo6Urs7hmQ|2026-08-09T21:51:50.000Z|2026-08-09T21:51:50.000Z|trash=
ChatGPT Export Text Backup|tU-Ot1Sq63NwBcxlnl7IcA~ThDi8B_92zkL76UXhjqFng|2026-05-18T02:36:23.000Z|2026-05-18T02:36:23.000Z|trash=
GitBackups|tU-Ot1Sq63NwBcxlnl7IcA~Vho9hzKVaqnBLf7UnwC64w|2026-07-21T00:29:59.000Z|2026-07-21T00:29:59.000Z|trash=
Sensitive Project Sources|tU-Ot1Sq63NwBcxlnl7IcA~n94X-kyGohebE_FBtxA5Sg|2026-05-22T15:12:18.000Z|2026-05-22T15:12:18.000Z|trash=
GitRemotes row present: False
```

**Row-set identical to the run-1 Preconditions-step-4 baseline**, no `GitRemotes` row, no
`trashTime` anywhere. Fresh-creation case again; this re-run's cleanup trashes `GitRemotes` again.

## R2.3 Creating `/my-files/GitRemotes` — and a new brief-text nit

The amended brief's bootstrap line was tried **exactly as written first**:

```
proton-drive filesystem create-folder /my-files/GitRemotes
Usage:
    filesystem create-folder parentPath name

You can create folders in your root folder (/my-files), devices (/devices) or in
a shared folder (/shared-with-me).
Expected 2 arguments, got 1

EXIT=1
```

**The amendment's single-path form is not valid CLI usage** — `create-folder` takes two
arguments, `parentPath name`. Verified nothing was created (`GitRemotes row present: False`).
This is a transcription nit introduced by the amendment itself: run 1 used, and the CLI's own
run-1 error message recommended, the two-argument form. Since the authorized action (create
`/my-files/GitRemotes`) is unambiguous, pre-authorized by Preconditions step 5, and identical in
effect, the proven form was used and the nit recorded (deviation R2-D1 below) rather than
blocking the re-run on an argument-spelling error in a docs amendment.

```
proton-drive filesystem create-folder /my-files GitRemotes
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~VKJhyCQWYEN-HoamgvZ6tw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g',
  name: { ok: true, value: 'GitRemotes' },
  type: 'folder',
  creationTime: 2026-08-10T15:28:58.290Z,
  modificationTime: 2026-08-10T15:28:58.290Z,
  trashTime: undefined,
EXIT=0

GitRemotes child count: 0
```

Run-2 `GitRemotes` uid = `tU-Ot1Sq63NwBcxlnl7IcA~VKJhyCQWYEN-HoamgvZ6tw` (a NEW uid; run 1's
`...0PkXib50j9udwUr3NZjoig` is trashed and gone). Created empty.

## R2.4 The amended contract-table run — `2026-08-10T08:29:16.3112193-07:00` -> `2026-08-10T08:39:53.1427446-07:00`

Whole block in ONE PowerShell process, run in background, from the source checkout root:

```
Set-Location C:\Users\craig\Projects\_Tools\git-proton-backup
$env:GPB_LIVE_ACCOUNT = "1"
$env:GPB_CONTRACT_LIVE_ROOT = "/my-files/GitRemotes/stage6-gate-contract"
go test ./internal/transport/ -run TestContractCLI -count=1 -timeout 40m -v
Remove-Item Env:GPB_LIVE_ACCOUNT
Remove-Item Env:GPB_CONTRACT_LIVE_ROOT
```

Environment as actually set, echoed at run time:

```
START=2026-08-10T08:29:16.3112193-07:00
GPB_LIVE_ACCOUNT=1
GPB_CONTRACT_LIVE_ROOT=/my-files/GitRemotes/stage6-gate-contract
GPB_UNCERTIFIED_CLI=[]
GPB_CREATE_PARENTS=[]
cwd=C:\Users\craig\Projects\_Tools\git-proton-backup
GO_TEST_EXIT=0
END=2026-08-10T08:39:53.1427446-07:00
post-removal in-call check: GPB_LIVE_ACCOUNT=[] GPB_CONTRACT_LIVE_ROOT=[]
```

`-count=1` present (checklist item 5); `-timeout 40m` present (the amendment); working directory
confirmed as the source root; `GPB_UNCERTIFIED_CLI` and `GPB_CREATE_PARENTS` empty throughout.

### Result — **PASS**

```
--- PASS: TestContractCLI (613.29s)
PASS
ok  	github.com/craigstoller/git-proton-backup/internal/transport	614.153s
GO_TEST_EXIT=0
```

**All 17 subtests PASS, with per-subtest verdicts — the thing run 1 could not produce.** Complete
verdict list, verbatim:

```
    --- PASS: TestContractCLI/stat_absence_is_not_an_error (28.37s)
    --- PASS: TestContractCLI/stat_not-found_is_pinned_against_the_certified_CLI's_own_signature_(Task_4) (27.28s)
    --- PASS: TestContractCLI/create_refuses_a_local_basename_that_mismatches_the_target_leaf_(C11) (26.98s)
    --- PASS: TestContractCLI/create_lands_at_the_target_leaf_when_basenames_agree (28.70s)
    --- PASS: TestContractCLI/create_refuses_a_name_already_taken (29.98s)
    --- PASS: TestContractCLI/readTo_lands_under_the_remote_basename_in_an_existing_dir (28.65s)
    --- PASS: TestContractCLI/readTo_into_a_missing_directory_errors_and_creates_nothing (31.79s)
    --- PASS: TestContractCLI/download_of_a_directory_recursively_materialises_the_subtree_(F1) (57.22s)
    --- PASS: TestContractCLI/trash_on_a_missing_target_is_committed (26.37s)
    --- PASS: TestContractCLI/ensureDir_is_idempotent_and_its_result_is_listable (39.22s)
    --- PASS: TestContractCLI/list_of_an_empty_directory_is_empty,_not_an_error (34.15s)
    --- PASS: TestContractCLI/trash_on_an_empty_folder_is_committed_and_the_folder_is_gone (40.50s)
    --- PASS: TestContractCLI/trash_on_a_folder_with_children_removes_the_whole_subtree (44.69s)
    --- PASS: TestContractCLI/create-folder_refuses_a_name_already_taken_by_a_file (35.66s)
    --- PASS: TestContractCLI/create-folder_on_an_existing_folder_reports_the_already-exists_signature_(Task_9b,_C17b) (35.77s)
    --- PASS: TestContractCLI/upload_of_a_file_colliding_with_an_existing_folder_name_does_not_silently_succeed (39.45s)
    --- PASS: TestContractCLI/nested_list_reports_folders_and_files_with_the_correct_IsDir (58.51s)
```

Measured tallies: `subtest PASS : 17`, `subtest FAIL : 0`, `subtest SKIP : 0`. A sweep for
`panic|timed out|SKIP|no live account|skipping live` returned `(none)`.

**F1 confirmed live, by exact name** (spec component 3's new row — a live two-level directory
downloaded via `filesystem download`, both files landing under `<dest>/<leaf>/...` with relative
layout preserved):

```
=== RUN   TestContractCLI/download_of_a_directory_recursively_materialises_the_subtree_(F1)
    --- PASS: TestContractCLI/download_of_a_directory_recursively_materialises_the_subtree_(F1) (57.22s)
```

**Duration `614.153s` is emphatically not the sub-second signature of a skip** (the Stage 3b tell
the brief names) — this was a genuine live run against the account.

**Margin note, worth recording for future briefs:** the parent test took `613.29s` against run 1's
`600s` ceiling — run 1 missed completing by roughly **14 seconds**. The per-subtest numbers
reconcile the two runs exactly: run 2's first sixteen subtests sum to `554.78s` and subtest 17
took `58.51s` (554.78 + 58.51 = 613.29, matching the parent line), while run 1 was killed with
subtest 17 at `41s` after ~559s of predecessors. The original brief's command did not fail by a
wide margin, which is precisely why the defect survived review — and why `-timeout 40m` (rather
than a marginal bump) is the right amendment.

## R2.5 Env-var absence, fresh shell — `2026-08-10T08:40:21.1517766-07:00`

New process, all three scopes checked:

```
GPB_LIVE_ACCOUNT: process=[] User=[] Machine=[]
GPB_CONTRACT_LIVE_ROOT: process=[] User=[] Machine=[]
GPB_UNCERTIFIED_CLI: process=[] User=[] Machine=[]
GPB_CREATE_PARENTS: process=[] User=[] Machine=[]
```

**PASS.**

## R2.6 Cleanup (outline step 5's affected subset)

**Verify-before-trash — contract base, full enumeration** (`2026-08-10T08:40:21`):

```
proton-drive filesystem list /my-files/GitRemotes/stage6-gate-contract --json
[

]
row count: 0
```

**EMPTY on the first listing — no lag tolerance needed.** This is the state run 1 could not reach:
**all 17 per-test scratch folders were auto-trashed by `t.Cleanup`**, including subtest 17's,
whose cleanup run 1's panic had prevented. The brief's step-5 expectation ("confirm it is now
EMPTY ... every per-test subfolder already auto-trashed by outline step 3's `t.Cleanup`") is
satisfied as written, and run 1's deviation R1-D2 is retired — it was indeed an artifact of the
aborted run, not a cleanup defect in the test.

**Trash the contract base** (`2026-08-10T08:40:38.5432715-07:00`):

```
proton-drive filesystem trash /my-files/GitRemotes/stage6-gate-contract
✅ stage6-gate-contract
EXIT=0
```

**`/my-files/GitRemotes` listing** (`2026-08-10T08:41:01.0657065-07:00`, after ~20s):

```
proton-drive filesystem list /my-files/GitRemotes --json
[

]
row count: 0
```

Empty. **Trash `/my-files/GitRemotes` itself** — fresh-creation case again, so the symmetric
authorization applies (`2026-08-10T08:41:27.5512439-07:00`):

```
proton-drive filesystem trash /my-files/GitRemotes
✅ GitRemotes
EXIT=0
```

**Final `/my-files` listing and strict row-set comparison** (`2026-08-10T08:42:00.2318247-07:00`,
after ~30s; compared on literal JSON field values, same method as run 1's 5.4):

```
row count: 5
_cas-probe|folder|tU-Ot1Sq63NwBcxlnl7IcA~TfvH3TrgNW1Eyo6Urs7hmQ|2026-08-09T21:51:50.000Z|2026-08-09T21:51:50.000Z|tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g|trash=
ChatGPT Export Text Backup|folder|tU-Ot1Sq63NwBcxlnl7IcA~ThDi8B_92zkL76UXhjqFng|2026-05-18T02:36:23.000Z|2026-05-18T02:36:23.000Z|tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g|trash=
GitBackups|folder|tU-Ot1Sq63NwBcxlnl7IcA~Vho9hzKVaqnBLf7UnwC64w|2026-07-21T00:29:59.000Z|2026-07-21T00:29:59.000Z|tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g|trash=
Project Repo Bundles|folder|tU-Ot1Sq63NwBcxlnl7IcA~5QY-B6t9Eui6VKsXvJPmuA|2026-06-22T16:25:42.000Z|2026-06-22T16:25:42.000Z|tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g|trash=
Sensitive Project Sources|folder|tU-Ot1Sq63NwBcxlnl7IcA~n94X-kyGohebE_FBtxA5Sg|2026-05-22T15:12:18.000Z|2026-05-22T15:12:18.000Z|tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g|trash=

=== STRICT ROW-SET IDENTITY vs RUN-1 PRECONDITIONS BASELINE ===
ROW-SET IDENTICAL: True
```

**PASS.** Identical to the run-1 Preconditions-step-4 baseline on every field of every row — same
five rows, same uids, same creation AND modification times, same parentUids, **no `trashTime` on
any row**. The pre-existing-`GitRemotes` `modificationTime` carve-out was not invoked and does not
apply (fresh-creation case). No untouchable was named in any run-2 write command; the complete
run-2 mutating set was `create-folder /my-files GitRemotes`, `trash .../stage6-gate-contract`,
`trash /my-files/GitRemotes`, plus `TestContractCLI`'s own writes confined under
`GPB_CONTRACT_LIVE_ROOT`.

## R2.7 Trash tally — run 2, and gate total

| class | run 2 | detail |
|---|---|---|
| per-test **SCRATCH FOLDERS** auto-trashed by `t.Cleanup` | **17** | all seventeen this time (confirmed by the empty base listing before any manual trash) — test-harness housekeeping, not a v2 prune event |
| contract **base folder** | **1** | `stage6-gate-contract` |
| gate **root folder** | **1** | `/my-files/GitRemotes` |
| **run-2 total** | **19** | |

Gate total across both runs: **40** trash events — run 1's 21 (2 junk files + 16 auto-trashed
scratch folders + 3 root folders) plus run 2's 19. Scratch folders across both runs: 33. Still
**no `push.go` prune/heal folder-trashing at any point** in either run (no `git push --delete`, no
self-heal folder collision), so checklist item 7's own class remains correctly counted at zero.

## R2.8 Deviations — run 2

**R2-D1 (new, minor, brief-text).** The amendment's bootstrap line
`proton-drive filesystem create-folder /my-files/GitRemotes` is **not valid CLI usage** —
`create-folder` takes two arguments (`parentPath name`), and the single-path form exits 1 with
`Expected 2 arguments, got 1`. It was attempted exactly as written first, created nothing, and the
proven two-argument form `create-folder /my-files GitRemotes` (used in run 1, and recommended by
the CLI's own run-1 error message) was used instead. The authorized action and its effect are
identical; only the argument spelling in the amended brief is wrong. **Recommend a follow-up
one-line brief fix.** Not treated as a BLOCK: the BLOCK rule governs mismatches between live
observations and the product's shipped strings/behaviour, and this is a usage typo in the brief's
own command text, with the intended action unambiguous and pre-authorized by Preconditions step 5.

**R1-D2 retired.** Run 1's "trashed a non-empty contract base" deviation does not recur — run 2's
base was empty before trashing, confirming that deviation's stated cause (the timeout panic
skipping one `t.Cleanup`) was correct.

No other deviations. Every other command ran exactly as the amended brief writes it.

**Step 3 re-run (run 2): PASS.**

---

# Consolidated gate verdict (runs 1 + 2)

| outline step | result | run |
|---|---|---|
| Release integration (draft bytes, digests, sidecar, install, PATH, `--version`) | **PASS** | 1 |
| Preconditions 1-5 | **PASS** | 1 |
| Outline step 1 — junk manufacture, tolerant push, strict `ls-remote` | **PASS** | 1 |
| Outline step 2 — push onto junk name, delete, recover | **PASS** | 1 |
| Outline step 3 — contract table live, including F1 | **PASS** | **2** (run 1 BLOCKED on the brief's timeout defect; amended and re-run under adjudication) |
| Outline step 4 — hierarchical smoke round-trip | **PASS** | 1 |
| Outline step 5 — cleanup, row-set comparisons, trash accounting | **PASS** | 1 + 2 |

## Overall: **PROVISIONAL PASS — all five outline steps.**

All five outline steps of the Stage 6 live gate have now passed against Craig's real Proton Drive
account, exercising the **v0.5.0 draft's own bytes** (installed helper digest
`e633eaf5158a61ef18b79a12fe733dbc03d649f57e4b9fdb4336ff847c65892d`, identical to the staged draft
asset). Every runner-asserted string matched the shipped text character for character — including
both of the brief's deliberate "reads like a typo but is not" traps: the doubled
`not a ref (not a ref: ...)` phrase and the literal, unsubstituted `<path>` placeholder. The
account was left row-set-identical to its pre-gate baseline after both runs.

**The verdict remains PROVISIONAL, by the brief's own definition** ("This gate brief's own verdict
... remains **provisional** until the closure runs"): the publication digest closure is a
separate, later, read-only pass that runs **after Craig publishes** the release — re-downloading
every published asset into a new empty directory, hashing, and comparing three ways against the
staged digests recorded at section R2 of this report. **That closure is not part of this run.**
Only after it PASSes is the v0.5.0 release final.

**Outstanding for the record:** one cosmetic follow-up (R2-D1) — the amended brief's
`create-folder` line needs its two-argument form restored. No product defect was found by either
run.

