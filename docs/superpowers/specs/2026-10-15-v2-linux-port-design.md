# git-remote-proton on Linux — design

**Date:** 2026-10-15. **Scope:** the v2 remote helper (`git-remote-proton`) only. The v1
`GitProtonBackup` module stays Windows-only: its transport is the Proton Drive Windows sync app and
the Windows Cloud Files API (README, "Why PowerShell?"). **Shape:** a stage spec; a dated plan
follows once Craig rules on the decisions below. **Status:** Draft. Decisions D1–D8 await Craig.
Nothing in this document is built yet. **Builds on:** PR #13, merged 2026-10-16 as `7b858ee`. It
fixed the Linux shim-test bug, added the git 2.45.0 floor to the README, and took design revision
v6.8. This stage's README edits build on it.

**Why now.** The README roadmap already says Linux support "means porting v2's build, not a new
mechanism", and the Reddit introduction asked for Linux testers. On 2026-10-15 and 16 a WSL2 distro
on Craig's laptop showed that most of the testing a tester would do can be done here. It turned up
the facts this design rests on:

- a git floor that Ubuntu's own git is below;
- a hard Secret Service requirement in the CLI;
- a CLI identity rule that distro builds would fail;
- case-variant refs, which a case-sensitive client can push and which Windows then mishandles
  silently.

**Basis marks.**

- **[wsl]:** run on 2026-10-15 or 16 in WSL2 `Ubuntu-24.04` (WSL 2.6.3, kernel 6.6.87) against
  `main` at `ba8917a`, with Windows interop disabled (F14). Every CLI run there was inside a network
  namespace with no route out unless the text says otherwise.
- **[win]:** run on 2026-10-16 on this laptop with Git for Windows 2.55.0.windows.3 on NTFS, with
  no account involved.
- **[doc]:** read from the named source on 2026-10-15.
- **[code]:** read from this repo at `ba8917a`.
- **[record]:** a committed gate record.
- **[inference]:** reasoned, not run.
- **[untested]:** nobody has run it.

## What tonight established

| # | Finding | Basis |
|---|---|---|
| F1 | The helper cross-compiles for linux/amd64 from Windows (`GOOS=linux CGO_ENABLED=0`; pure stdlib; go.mod has no requires). The binary runs and prints `git-remote-proton dev-linux-probe (certified CLI: cli-drive@0.9.0+75612299)`. | [wsl], [code] |
| F2 | The hermetic suite on Linux (go1.27.0, ext4 clone) with Ubuntu 24.04's git **2.43.0**: 436 pass, 26 fail, 5 skip. Build, vet and gofmt are clean. | [wsl] |
| F3 | 23 of the 26 failures are the fetch path's git floor: `rev-list --missing=print` must report missing tips as `?`-lines. **The floor is 2.45.0, by test:** 2.44.4 (the newest 2.44.x) fails the same 23, while 2.45.0 and 2.55.0 pass all 23 (459 pass, 3 fail, 5 skip, test count unchanged at 467). A single call shows the same split: `git rev-list --objects --missing=print --stdin` fed one absent object ID, in an existing repo, exits 128 with `fatal: bad object` on 2.43.0 and 2.44.4, and exits 0 printing `?<id>` on 2.45.0 and 2.55.0. It agrees with git's docs: the missing-tips sentence in `Documentation/rev-list-options.txt` is absent at v2.43.0 and v2.44.0 and present at v2.45.0. On old git the helper fails closed with its designed wrapped error, which is the first end-to-end evidence of the path the Stage 3b spec recorded as untestable on the suite's git. | [wsl], [doc] |
| F4 | The other 3 failures were a test-harness bug: `shim_test.go` named the fake CLI after the test binary's extension (`.test` on Linux). PR #13 fixes it. | [wsl], PR #13 |
| F5 | Proton's linux/x64 CLI 0.9.0 (glibc; 96,683,488 bytes; SHA-512 `3533025b…a7e91ca8`, equal to the live `version.json` entry on 2026-10-15) prints `Proton Drive CLI cli-drive@0.9.0+75612299` / `Proton Drive SDK js@0.22.2+75612299`. The helper's own `identityToken`/`IsCertified` accept that output, so **the exact pin needs no per-platform token for linux/x64**. The Linux build has not been live-certified: no contract table has run against it. The arm64 and musl builds were not run. | [wsl] |
| F6 | On Linux the CLI **hard-requires a D-Bus Secret Service** by default. With no session bus it exits 1 before any network attempt: `Failed to load session from secrets (ensure you have secrets available, read the README for more information): Cannot autolaunch D-Bus without X11 $DISPLAY`. With craig's systemd user bus and an unlocked gnome-keyring 46.1, it reached `No session loaded` → `You need to login first` (exit 1). dbus-monitor showed GetAll → OpenSession → SearchItems with `service=ch.proton.drive/drive-sdk-cli`, `account=auth-session`, `xdg:schema=com.oven-sh.bun.Secret`, which is Bun's built-in secrets API. | [wsl] |
| F7 | Through the helper, F6's failure reaches the git user with the CLI's text intact: `git-remote-proton: stat …/gpb-remote.json: info …/gpb-remote.json failed: Failed to load session from secrets (…): Cannot autolaunch D-Bus without X11 $DISPLAY: exit status 1`; git exits 128. | [wsl] |
| F8 | The CLI offers a second credential store. `PROTON_DRIVE_CREDENTIALS_STORE=pass` keeps the session as a GPG-encrypted entry in `pass` (password-store); it was added in CLI 0.6.0 ("Support pass as credentials store"). The README also lists `unsafe_file` ("do not use, for testing only"). The helper starts the CLI with `exec.Command` and no `cmd.Env`, so these variables pass through unchanged. No test pins that today. | [doc] Proton CLI README + CHANGELOG on `ProtonDriveApps/sdk` main; [code] `internal/transport/cli.go:52` |
| F9 | The CLI keeps its state in XDG paths on Linux: `~/.local/share/proton-drive-cli` (clientUid.json, events.lock), `~/.local/state/proton-drive-cli/proton-drive.log`, `~/.cache/proton-drive-cli/cache-{entities,crypto}.sqlite`. Proton's README says `PROTON_DRIVE_CACHE_DIR` moves all three to one root; that part is unverified here. | [wsl]; [doc] for the variable |
| F10 | Sign-in "uses the browser (no password on the command line)". | [doc] |
| F11 | Proton's README says "Unmodified CLI repackaged for a distribution may use `cli-drive-<distro>`". The helper's `identityToken` only recognises a last field starting `cli-drive@`, so such a build has no identity line. `EnforceCertified` then gives the generic refusal, which quotes the full `--version` output and names the certified build and the `GPB_UNCERTIFIED_CLI=1` override. | [doc], [code] `cli.go:597-625, 700-710` |
| F12 | Proton's README says the default `linux/x64` build "crashes at startup with `Illegal instruction`" on CPUs without AVX2, and recommends `linux/x64-baseline` there. The 0.9.0 feed lists no baseline build (only linux x64, x64-musl, arm64 and arm64-musl). So the certified linux/x64 CLI needs AVX2. | [doc] README; feed read 2026-10-15 |
| F13 | **Case-variant refs.** Proton stores `main` and `Main` side by side (remote names are exact-byte), and the helper advertises both [record]. **A Windows clone of such refs succeeds, and local use then goes wrong** [win, bundles built on ext4, cloned onto NTFS; not through the helper]: <br>• A `main`/`Main` clone: rc=0, `fsck` clean, both remote-tracking refs kept. While `refs/heads/main` is a loose file, `git switch Main` puts HEAD on `Main` at **`main`'s commit and files, silently**. Once the refs are packed, the same command works. <br>• A `feat`/`Feat/x` pair: the second local branch is refused loudly ("cannot lock ref"). <br>• A `grp/a`/`Grp/b` pair: the second local branch is **silently re-cased** to `grp/b`. <br>The 0.9.0 record's "a clone would fail inside git" was inference and is superseded. Linux (ext4) holds such pairs naturally. Any platform can create one with an explicit refspec such as `HEAD:refs/heads/Main` [inference]. | [record] `cli-0.9.0-certification.md` §4.8; [win] |
| F14 | **WSL default is a hazard for hermetic runs.** With interop on, the real signed-in Windows `proton-drive.exe` is on the Linux PATH and executes. The test distro sets `/etc/wsl.conf` `[interop] enabled=false, appendWindowsPath=false`; a probe confirmed the exe then exits 126. | [wsl] |

## Goals, non-goals, anti-goals

**Goals.**

- A **linux/amd64 `git-remote-proton` release asset**, built by the Release workflow and gated live
  from Linux against the draft's exact bytes, the same rule as Windows.
- **CI runs the hermetic Go suite on Linux** as well as Windows, so a Windows-only assumption can't
  reach `main` unnoticed again (F4 shows one already did).
- **The README states the Linux prerequisites honestly:** git ≥ 2.45.0; an AVX2 CPU; the official CLI
  build at the certified token and SHA-512; a credential store the CLI can use.
- **Repos stay usable from every machine for every rule the helper can enforce.** A repo pushed from
  Linux must clone, fetch and check out on Windows without silent ref confusion, and the reverse. The
  long-path limit is a separate, already-documented Windows limit (README, "Windows path length"),
  which this stage does not change.

**Non-goals.**

- v1 on Linux. Its transport doesn't exist there.
- macOS, including APFS Unicode normalisation. Nothing here should make it harder, but it isn't built,
  gated or documented in this stage; the README roadmap keeps macOS as an aspiration only.
- Distro packaging.
- The helper handling credentials: signing in, unlocking a keyring, storing a secret. This is the
  design's "implementing Proton auth" non-goal.
- Helper-side remediation of the CLI's secrets failure. F7 shows the CLI's message arrives intact.

**Anti-goals** (failure even if it works):

- Shipping a Linux binary that no live gate ran.
- A Linux change that weakens Windows behaviour, or lets any push create refs a Windows clone
  mishandles silently.
- Any hermetic or test run that can reach the account through WSL interop or a signed-in Linux CLI
  (F14).
- README claims about Linux that no probe, gate or cited source backs, unless they are marked
  "not gated".

## Design

### 1. Build and release

`release.yml` cross-compiles `GOOS=linux GOARCH=amd64 CGO_ENABLED=0` in the existing Windows job,
right after the Windows build, with the same `-trimpath -ldflags "-X main.version=…"`.

The asset list is written out in four places in `release.yml`, and all four change:

- the Build step's `-o`;
- the Checksum step, which also writes the linux sidecar;
- the dispatch dry run's `upload-artifact` `path` glob (`git-remote-proton.exe*` today, which
  would silently drop the linux asset);
- the `gh release create` argument list.

`install.ps1` lists no assets, so it doesn't change. The sidecar uses the existing `<hash>  <name>`
format. PowerShell 7's `Set-Content` writes UTF-8 with no BOM, and `-NoNewline` leaves no CR
[inference]. Gate step 1 runs GNU `sha256sum -c` on the real sidecar, which is the test.

Under D1(a) the draft holds five assets:

- `git-remote-proton.exe`
- `git-remote-proton.exe.sha256`
- `install.ps1`
- `git-remote-proton-linux-amd64`
- `git-remote-proton-linux-amd64.sha256`

*The asset naming is my call.*

Every mention of the asset set, and every exe-only check, in `releasing.md` changes:

- **Step 4:** the asset list.
- **Step 5:** every release runs the live gate on Windows **and** the §7 gate on Linux, each on the
  draft's bytes.
- **Step 7:** the closure covers all five assets.
- **Stand-in eligibility:** the workflow bullet's "three-asset list". The `go version -m` bullet now
  covers both binaries against the anchor's.
- **Stand-in step 1:**
  - "Exactly three assets" becomes five;
  - both sidecars are checked;
  - "Stage all three digests" becomes five;
  - the linux asset's `--version` runs in WSL.

The first Linux release cannot take a stand-in: it changes `release.yml` and has no Linux anchor.

Cross-compiling keeps one toolchain resolution (`go-version-file: go.mod`) and one version string.
The helper has no cgo, so a Windows-hosted build makes the same kind of binary a Linux-hosted one
would [inference from F1]. That binary is built with go.mod's toolchain, so §2 also tests Linux at
that toolchain.

### 2. CI

`ci.yml`'s `go` job becomes a matrix:

- **Windows** stays as it is: `stable`, `pwsh` steps.
- **Linux** runs twice: at `stable`, and at `go-version-file: go.mod`, the toolchain the release
  actually builds with. Its steps use `bash`. *The second Linux leg is my call.*

Both platforms run build, vet, `go test -count=1 ./...` and gofmt. `.gitattributes` pins `*.go` to
LF, so gofmt agrees across platforms. The Pester job stays Windows-only.

**The git floor check on Linux is one explicit, branching shell step** before the suite, not a
test:

1. Run F3's single-call probe in the checkout: feed one absent object ID to
   `git rev-list --objects --missing=print --stdin` and require exit 0.
2. If it fails, add the Ubuntu Git Maintainers PPA (`ppa:git-core/ppa`) and install current git,
   with the apt steps retried.
3. Probe again.
4. If it still fails, exit with a message naming the 2.45.0 floor.

The step prints `git --version` and which path it took. *The PPA fallback is my call.*
`ubuntu-latest`'s git version is [untested]; this step's log is how we find out.

CI's git floats on purpose, as `go-version: stable` does: CI proves the code works on current git,
and the probe guards the floor. Pinning an exact git, and testing at exactly 2.45.0, are left to the
gate venue and the WSL distro (F3), where both versions are already built.

### 3. The git floor in the helper

PR #13 documents 2.45.0 in the README and design v6.8. The helper's wrapped error names the
requirement only by behaviour [code `gitcmd.RevListMissing`]. A push needs no floor, but a restore
does. So on stock Ubuntu 24.04, backups succeed while a restore on that same host fails (F2, F3).
**D5** decides how loudly the helper says so.

### 4. Linux prerequisites and credentials

The helper changes nothing about credentials. It inherits whatever the CLI does (F6, F8).

A new hermetic test pins F8's pass-through. It starts the scripted shim with a `PROTON_DRIVE_*`
variable and an arbitrary unrelated variable set, and asserts the shim sees both. That guards
against both a cleared and a filtered `cmd.Env`. *The test is my call*; it protects D3's
documentation against a later `cmd.Env` change.

The README gains a Linux prerequisites block, linking Proton's README for the stores:

- **git ≥ 2.45.0** for fetch and clone, with the upgrade route on Ubuntu 24.04 (the PPA).
- **An AVX2 CPU**, per Proton's README (F12). This is the CLI's requirement, not the helper's. On
  such a CPU the CLI crashes at startup, before the helper can report anything useful, and no
  certified CLI build exists for it. The README's honest limits say so.
- **The CLI build at the certified token**, installed with the commands in §6 and checked against the
  SHA-512 the Linux gate record holds.
- **A credential store.** The tested store is the default one: a Secret Service (GNOME Keyring,
  KWallet, anything that speaks `org.freedesktop.secrets`), unlocked in the session that runs git.
  `PROTON_DRIVE_CREDENTIALS_STORE=pass` is the CLI's other store, marked **not gated** unless D3
  gates it.

### 5. Case-variant refs

**D2** decides whether pushes may create refs that a Windows clone mishandles (F13). The helper
already refuses Windows device names on every platform for the same reason: "a repo pushed from Linux
with a ref named refs/heads/aux would be unusable from Windows … Refusing it everywhere keeps a repo
usable from every machine" [code `marker.go:292-301`]. That rule likewise takes a name from Linux
users so every machine can use the repo.

If D2 adopts the refusal, the rule is as follows.

**Prefixes and folding.**

- A ref's prefixes are its component chains from the namespace component down: for
  `refs/heads/grp/a`, they are `heads`, `heads/grp` and `heads/grp/a`. The `refs/` root is fixed.
- Two spellings conflict when they differ but fold equal under Unicode simple case folding (the
  comparison Go's `strings.EqualFold` makes, not ASCII).
- NTFS folds case with its own `$UpCase` table, which differs from Unicode simple folding for a
  handful of characters [inference]; that residual is a documented limit. Unicode normalisation (NFC
  against NFD, macOS) is out of scope.
- This covers all three F13 shapes: `main`/`Main` (two leaves), `feat`/`Feat/x` (a leaf against a
  directory prefix), and `grp/a`/`Grp/b` (two directory prefixes).

**Only creates are refused.**

- A create is a destination that does not exist on the remote when it is written. That is the
  batch-aware existence phase 5 already computes, `preExists && !deletedThisBatch` [code
  `push.go:508-509`]. So a same-batch delete-then-recreate of one name counts as a create.
- A create is refused when any of its prefixes conflicts with a prefix of another ref in the set it is
  checked against. The refusal names every conflicting ref, sorted.
- Updates and deletes are never refused by this rule. A remote that already holds a pair still takes
  pushes to its other refs, and a user can repair it by deleting one of the pair.
- The rule is strict where a pair already exists. Under a pre-existing directory pair such as
  `Feat/a` beside `feat/b`, a new ref under either spelling is refused until the pair is repaired.

**Phase 2 (whole-batch validation),** where today's checks already refuse per update [code `push.go`
header]:

- The check set uses the existing `finalSet` two-pass algebra [code `push.go:309-336`]: the pre-batch
  advertised refs, minus the batch's valid deletes, plus every non-delete (create or update) still
  valid after phase 2's other checks. A delete-and-recreate of one name therefore stays in the set.
- Conflicts are computed as groups over that set, with no dependence on iteration order. Every
  create in a conflicting group is refused, so two same-batch creates `main` and `Main` are **both**
  refused.

**Phase 5 (creates and updates).**

- Before each create, the rule is checked again against the live set: the pre-batch advertised refs,
  minus the deletes that **committed** in phase 4, plus the creates already committed in this phase.
- So a create cleared in phase 2 by a same-batch delete is refused if any of the deletes it relied on
  failed.
- **Nothing in phase 5 does this today.** `deletedThisBatch` only chooses create against update for
  the same name [code `push.go:470-515`], and the existing dir/file rule is not re-checked there
  either. This re-check is new.
- **Placement:** the re-check is the first statement of the create arm, **before**
  `ensureRefParents`. That call creates remote parent folders, so placing the check any later would
  leave folders behind for a refused create.
- **Test:** a hermetic test drives this path with `transport.Fake` failing the `Trash` of a
  same-batch delete, which is a phase-4 delete failure.
- **No gate step can reach it deterministically.** A phase-2-valid delete fails in phase 4 only
  through a race or a transport fault [code comment, `push.go:470-483`]. The hermetic test is its
  evidence, and "Still untested" says so.
- **What the user sees:** git's per-ref output reports that ref rejected, naming the delete it
  depended on. Phase 2 did not refuse it, so this is the one case where a ref passes validation and
  is still rejected.
- A pack uploaded in phase 3 for a create refused here is left unreferenced, as with any phase-5
  create failure today [inference]. Pack compaction is where that is cleaned up.

**Where it runs.** Under the repo lock. `main.go` acquires that lock before advertising [code
`main.go:441`], so two clients can't each validate one half of a pair.

**Skipped refs are not in the set,** and that leaves one stated exception.

- Name-skipped refs are never advertised, and a content skip makes `list` and fetch fail closed
  (Stage 6), so neither reaches a clone.
- So a push can create a ref whose case variant sits in a skipped file. The existing occupancy rule
  refuses only creates on, above or beneath the skipped path itself [code `push.go` header].
- The harm is deferred. The pair becomes visible to clones only when someone repairs the skipped
  file, and from then on a Windows clone mishandles it as in F13.
- This is the one way a push under D2(a) can lay down half of a pair. Closing it would mean putting
  skipped names in the check set, which round 1 rejected because they never reach a clone. The
  exception is stated in the README's honest limits instead.

**Existing pairs** stay as they are. A pair can be on a remote without its user ever pushing one:

- from pushes made before this release;
- from a CLI or web-UI upload, as the 0.9.0 certification made one;
- from a repaired skipped file, as above.

The README's honest limits list those sources, say how a Windows clone mishandles a pair (F13), and
give the repair: delete one of the pair.

### 6. Install on Linux

The first cut has no installer (D6). The README gives complete commands for both binaries.

The helper, from the release's two linux assets:

```
mkdir -p ~/.local/bin
sha256sum -c git-remote-proton-linux-amd64.sha256
install -m 0755 git-remote-proton-linux-amd64 ~/.local/bin/git-remote-proton
type -a git-remote-proton   # exactly one, the one just installed
```

The CLI, from Proton's download URL for the certified version:

```
curl -fLo proton-drive https://proton.me/download/drive/cli/0.9.0/linux-x64/proton-drive
echo "<certified SHA-512>  proton-drive" | sha512sum -c
install -m 0755 proton-drive ~/.local/bin/proton-drive
type -a proton-drive
```

The `<certified SHA-512>` is the value the Linux gate record holds (§8), labelled with its
certification date.

The README notes that a fresh `~/.local/bin` joins PATH at the next login on Ubuntu. So the
`type -a` checks run in a new login shell; until then, `~/.local/bin/git-remote-proton --version`
confirms the install directly. The `type -a` lines are the Linux form of `install.ps1`'s shadow
check, which exists because a stale helper earlier on PATH once passed for the new one (Stage 4).
Gate step 1 runs the CLI commands exactly as written.

What the recorded hash proves: the bytes match what the gate tested. Checking against the live feed
proves only that the download matches whatever Proton serves today.

### 7. The live Linux gate

The gate runs in the WSL distro, per **D7**.

**Prep, before the release is tagged: a sign-in probe.** It changes nothing in the stage's results,
and ends logged out.

1. Pause `ProjectsBackupDaily`, and wait for any running instance to finish.
2. Run a read-only `proton-drive filesystem info /my-files` **on Windows**, as a baseline.
3. Craig runs `proton-drive auth login` in the distro, with interop off, into a keyring unlocked with
   a password **he chooses**, and with `PROTON_DRIVE_CACHE_DIR` set to a probe directory.
   - Whether the login can complete there is [untested]. The CLI can't open a Windows browser, and
     may print a URL that WSL2's localhost forwarding carries back.
   - The brief authorises up front any harmless helper the probe needs, such as `BROWSER=echo` to
     print the URL.
4. One read-only CLI command, then confirm that the cache files landed in the probe directory, not
   `~/.cache/proton-drive-cli` (F9).
5. The Windows check again.
6. `auth logout` in WSL, then the Windows check once more.
7. Delete the probe keyring and directory, and unpause the task **only if every Windows check passed**.
   If one fails: STOP for Craig, with the task left paused. Recovery is Craig signing in again on
   Windows and confirming a fleet verify.

If the login cannot complete, D7's alternative venue is set up then, before tagging, not mid-gate.

**Gate setup.**

- `ProjectsBackupDaily` is paused, with no run in flight, for the whole gate. The two-session
  interaction is untested. *That is my call; F9's separate caches only settle SQLite contention.*
- Craig signs in as in the probe.
- `PROTON_DRIVE_CACHE_DIR` points at a gate-scoped directory.
- The distro's keyring holds nothing but the gate session.
- Network is on.
- git 2.55.0 from `~/opt/git-2.55.0` is on PATH ahead of the system git.

**Before tagging, the contract test's untouchable-folder list is brought current.** It still names
the original four [code `contract_test.go:36-41`], while the account now has `Archive` and `Tax
Documents` too [record `v0.9.0-release-gate.md`, `cli-0.9.0-certification.md` D3].

It runs under `brief-checklist.md` rules 1–10, plus the proposed rule 11 (§9).

**Write confinement:** `/my-files/GitRemotes/<gate repos>` plus the contract root (rules 2 and 8).
Every node the gate creates under those roots is expected to end in trash, and is tallied per rule 7.

Gate content:

0. **Preconditions, asserted.**
   - WSL:
     - `/etc/wsl.conf` has interop off;
     - the Windows `proton-drive.exe` cannot execute (exit 126);
     - `command -v proton-drive` resolves to the certified Linux CLI;
     - `command -v git-remote-proton` resolves to the draft asset;
     - the gate cache directory is in use.
   - Windows:
     - the helper git resolves is the draft exe's bytes, installed per `releasing.md` step 5:
       `Get-Command git-remote-proton -All` finds exactly one, and its SHA-256 equals the draft's.
       Its path and hash are recorded. A helper that merely prints the same certified **CLI** token
       does not qualify, because every helper built against this pin prints it;
     - the Windows CLI reports the certified token;
     - the read-only Windows check passes;
     - a `/my-files` row-set baseline is taken (rule 1).
1. **Offline checks on the draft:**
   - the asset list (five under D1(a); the list follows D1);
   - `sha256sum -c` in WSL on the linux sidecar;
   - the exe's sidecar on Windows;
   - `go version -m` on both binaries;
   - the linux asset's `--version` shows the tag and the certified token;
   - the installed Linux CLI's SHA-512 is recorded;
   - §6's CLI commands, run as written into a temp directory: the downloaded bytes must equal the
     installed CLI's and the recorded value, and the README's `sha512sum -c` line must parse.

   A sidecar or asset defect found here ships as a new tag, never a retag (`releasing.md` step 5).
2. **The 17-row live contract table from Linux:** `GPB_LIVE_ACCOUNT=1`, `-count=1 -timeout 40m`,
   `GPB_CONTRACT_LIVE_ROOT` named and in the confinement list (rules 5, 8, 9). Then the read-only
   Windows check once more, so a session problem shows up before step 4 depends on the Windows
   side.
3. **Smoke with the draft linux asset:** bootstrap push, clone with `git fsck --full`, incremental
   fetch, `--set-head`. These are the Stage 5/6 gate shapes, with the stage6 brief's create-folder
   bootstrap.
4. **Cross-platform round trip.** A repo pushed from Linux is cloned on Windows, and a repo pushed
   from Windows is cloned in WSL; both get `fsck` and a ref-set comparison. Then each side pushes an
   update and a branch deletion, and each existing clone fetches them.
5. **Case variants,** all inside the gate root.
   - If D2 refuses them:
     1. A push of **only** a `refs/heads/Main` create beside `refs/heads/main` is refused, naming
        both refs. It makes no ref or pack write; the lock's own create and trash are expected and
        asserted.
     2. A mixed push of a `main` update plus a `Main` create lands `main` and refuses `Main`, with
        one pack. A fresh clone then fetches the result and `git fsck --full` passes, so the pack's
        content is checked as well as its count.
     3. A `refs/heads/Feat/x` create beside `refs/heads/feat` is refused.
     4. A pair made inside the gate root with a CLI upload, as the 0.9.0 certification did, still
        lets an unrelated ref be pushed, and refuses a create of a third variant.
   - If D2 allows them: a Linux clone checks out both variants, and a Windows clone shows F13's
     shapes through the helper.
6. **Cleanup, in this order, on every exit path,** including after a BLOCK:
   1. Enumerate the gate's subtrees and trash them, verify-before-trash (rule 3).
   2. Check the `/my-files` row set against the baseline (rule 1), and tally trash (rule 7).
   3. `auth logout` in WSL, unless D7 keeps the session.
   4. Run the Windows check.
   5. Delete the gate keyring and cache directory.
   6. Unpause the task only if the Windows check passed. Otherwise STOP for Craig, as in the probe.

### 8. Certification afterwards

**D8** decides whether every future CLI certification runs the contract table on linux/x64 too.

The Linux gate record holds the linux/x64 CLI SHA-512 it ran against, and the README's `sha512sum -c`
line (§6) takes it from there. Each later certification adds the new linux/x64 SHA-512 next to the
windows/x64 one, and the README line moves with it.

Under D8(a) the drift script also compares the feed's linux/x64 SHA-512 against that value. The
value lives as a constant in `scripts/Test-ProtonCliDrift.ps1`, which becomes one more certification
touch point; it is one JSON field read, with no Linux runtime needed. Linux users otherwise meet
drift as an allowlist refusal, and the README's honest limits say so.

### 9. Documents that change

- **README:**
  - the roadmap (macOS stays aspiration only);
  - v2 install for Linux (§6);
  - the Linux prerequisites block (§4);
  - honest limits: case variants per D2, unattended use per D3, drift (§8).

  It builds on PR #13's git-floor line.
- **`v2-remote-helper-design.md` → v6.9** (PR #13 takes v6.8):
  - a supported-platforms line;
  - the "Deployment surface" sentence gets F6 and F8's evidence;
  - Stage 4's "windows/amd64 only" line is superseded;
  - D2's rule, if adopted.
- **`releasing.md`:** every place §1 lists.
- **`brief-checklist.md`:** a new rule 11. Hermetic and live runs in WSL keep interop off. A
  hermetic run either has no `proton-drive` resolvable on PATH except a test shim, or runs inside a
  network namespace with no route out.
- **`internal/transport/contract_test.go`:** the untouchable-folder list (§7).
- **`scripts/Test-ProtonCliDrift.ps1`:** under D8(a).
- **The Linux gate record**, which holds the CLI SHA-512.
- **`CHANGELOG.md`**, and the `ModuleVersion` bump `releasing.md` step 2 requires.
- **`docs/README.md`:** link this spec and its plan.

The release is a minor bump (0.10.0) [inference: a new platform]. D2(a) adds a refusal for new
case-variant creates. It changes no existing ref and refuses no update or delete.

## Decisions for Craig

Ordered so the ones the others follow from come first. Every default is my recommendation, not a
ruling. Rulings go into design v6.9's revision history when this ships, with the options not taken.

**D1 — Platform scope of the first Linux release.**
- (a) linux/amd64 glibc only. *Recommended:* it is the only Linux build F5 ran and the only one a
  WSL gate on this x64 laptop can exercise, and the project does not ship what no gate ran. §1's
  asset list follows from it.
- (b) Also ship linux/arm64, built but marked ungated. The project's first ungated asset.
- (c) Also musl. The helper is static, but the musl CLI builds are untested.

**D2 — Case-variant refs.**
- (a) Refuse **creates** that conflict under §5's rule, on every platform. *Recommended:* F13 shows a
  Windows clone silently checks out the wrong commit or silently re-cases a branch, which is worse
  than a failure, and the device-name rule set the precedent. The cost is real, and it isn't only
  about branches:
  - a Linux user can no longer create `main` beside `Main`, which works on Linux;
  - a push of `refs/tags/Release-1` beside `refs/tags/release-1` refuses one of them;
  - a remote that already holds a directory pair takes no new refs under it until repaired;
  - there is no override variable, by the same YAGNI ruling that gave Stage 6's foreign-data policy
    none.
- (b) Allow, and document F13 in the README's honest limits. Simpler, but it lets a push create refs
  a Windows clone mishandles silently, which is one of this spec's anti-goals.

**D3 — Unattended (cron or systemd-timer) use on Linux.**
- (a) Gate and support **attended** use: a session with an unlocked Secret Service. The README names
  `pass` and an unlocked keyring as the CLI's own options for unattended use, marked "not gated".
  *Recommended:* the helper takes no part in credentials. Headless unlock always means storing a
  password or decrypting without a prompt somewhere, and that trade-off belongs to the user, not to
  a default we ship.
- (b) Also gate the `pass` store live. That means a GPG key, a gpg-agent cache, a cron-style run,
  **and** the failure cases: cache expired, and after a reboot. It tests the CLI more than the
  helper, and roughly doubles gate prep.
- (c) Say nothing. Rejected: the first Linux user to schedule a push hits F6 with no warning.

**D4 — CLI builds repackaged by distros (`cli-drive-<distro>@…`, F11).**
- (a) Keep the exact token, and add a specific sentence to the refusal when the output carries the
  `cli-drive-<distro>@` shape, naming the official download. *Recommended:* a different build is
  different bytes, and today's refusal already quotes the output and names the certified build.
  **Constraint:** the new code only adds text on a path that has already refused. It is never an
  acceptance path, so `identityToken`'s exact-token hardening is untouched. The
  `GPB_UNCERTIFIED_CLI=1` override applies unchanged. A hermetic test feeds a fabricated
  `cli-drive-<distro>@` `--version` output and asserts both the refusal and the added sentence, since
  no real build of that shape is known.
- (b) Accept `cli-drive-<distro>@<certified version+build>` as certified. Friendlier, but it certifies
  bytes no gate ran.
- (c) No change.

Whether any distro ships such a build today is [untested]. A distro that suffixes the standard
token (for example `cli-drive@0.9.0+75612299-1`) is already parsed and refused as uncertified.

**D5 — How loudly the helper states the git floor.**
- (a) Name `git 2.45.0 or newer` in the existing fetch error. *Recommended, with (d):* a one-line
  change, and the behaviour-based detection stays.
- (d) Also warn, without refusing, on push when this git cannot restore. *Recommended with (a).*
  - The check is F3's single-call probe: one `git rev-list` process in the repo being pushed, with no
    temp repo.
  - It runs at `list for-push`, before the repo lock is taken, so it adds nothing to the lock's
    critical section.
  - Why: on stock Ubuntu 24.04, backups otherwise succeed silently on a host that cannot restore
    them (F2, F3).
  - Exit 0 with a `?`-line means this git can restore. Anything else warns, naming the floor and
    quoting the probe's own output. The probe can fail for an unrelated reason, such as a broken git,
    so a failed probe is never treated as proof.
  - The probe never refuses a push.
  - The cost is one short git process per push, on every platform, Windows included. It is short
    because the probed object is absent, so git walks nothing (F3) [inference].
  - The warning only helps someone who reads push output. An unattended push on old git still
    succeeds with the warning unread, which is why (a) and the README carry the floor as well.
- (b) Refuse to start below 2.45.0, by parsing `git --version`. Rejected: parsing distro and vendor
  version strings is fragile, and push doesn't need the floor.
- (c) Docs only (PR #13).
- (e) A download-everything fallback on old git. Rejected in the Stage 3b spec's round 1, because a
  second live code path would mask selectivity regressions behind silent fallback. Nothing here
  changes that.

**D6 — Release mechanics and install.**
- (a) Cross-compile in the existing Windows job; the README gives §6's commands; no `install.sh`.
  *Recommended:* the smallest change that ships a gated binary. An installer is easy to add once a
  real Linux user shows what they trip on.
- (b) A separate `ubuntu-latest` release job plus an `install.sh` mirroring `install.ps1` (checksum,
  `~/.local/bin`, shadow check). More surface, more to gate, and two toolchain resolutions to keep
  equal.

**D7 — Gate venue and the WSL session afterwards.**
- Venue (a): the WSL distro on this laptop, interop off, with §7's pre-tag sign-in probe.
  *Recommended:* it exists, it is configured, and it is where F1–F14 came from.
- Venue (b): a separate Linux machine or VM, set up only if the sign-in probe fails. A more
  realistic desktop, but new setup.
- Venue (c): a GitHub runner with the session in repository secrets. Rejected: a live Proton session
  in CI.
- Afterwards (a): log out in WSL and delete the keyring and cache. *Recommended:* it leaves one
  long-lived CLI session on the account, as today.
- Afterwards (b): keep the WSL session for future certifications. A second long-lived session whose
  interaction with the Windows one is [untested].

**D8 — Future CLI certifications.**
- (a) Run the full contract table on linux/x64 in WSL as well as on Windows, and add the drift
  script's linux/x64 comparison (§8). *Recommended:* the pin is per build, and the Linux build is
  different bytes. Cost per certification: the table's ~10 minutes (605–614 s on record, on
  Windows), plus a sign-in and the session checks around it under D7 afterwards (a), roughly
  20–30 minutes in all [inference].
- (b) The Windows table plus a Linux `--version` and smoke only.

## Left to the builder

These are my calls; a plan may change them with a reason.

- The asset names.
- Where the CI probe step and the PPA fallback live in `ci.yml`, and their retry counts.
- The exact README wording.
- The hermetic env pass-through test's shape.
- Where D5(d)'s probe sits within `list for-push`, provided it runs before the lock.

## Revisions

Scaffolding for the review loop; delete before the plan is written.

**Round 1 (2026-10-16; Codex, Gemini via agy, and the repo-aware Fireworks.ai roster: DeepSeek and
Kimi; all four reported).**

- Case-variant mechanism rebuilt [Codex+Gemini+DeepSeek+Kimi].
- F13 and the git floor probed.
- CI moved to a behavioural check [Gemini+Kimi+DeepSeek].
- D5(d) added [Gemini+DeepSeek].
- Gate step 5's lock writes made explicit [Kimi+DeepSeek].
- Gate step 2 gets `GPB_LIVE_ACCOUNT` [DeepSeek].
- Isolation preconditions [DeepSeek+Codex].
- Sign-in probe before tagging [Gemini+DeepSeek].
- Session checks and the task pause [DeepSeek+Gemini].
- Cleanup and existing-clone fetches [Codex+Gemini].
- `release.yml` touch points [Gemini+DeepSeek].
- Install commands [All].
- The certified SHA-512 [Codex+Gemini].
- AVX2 [All].
- F11/D4 corrected [Kimi+DeepSeek].
- D3 and D8 wording [Codex+DeepSeek+Gemini].
- v6.9, the manifest bump, the F8 test, the anti-goal scope.
- Rejected: including skipped names in the fold check [Kimi+Gemini].

**Round 2 (2026-10-16; all four reported; DeepSeek said Blockers: none but listed majors).**

Defects in round-1 fixes, applied:

- **Phase 5.** §5 claimed phase 5 already re-checks via `deletedThisBatch`. It doesn't
  [Gemini, confirmed `push.go:470-515`]. §5 now specifies a new live-set re-check, and order-free
  group conflicts in phase 2 so same-batch pairs are both refused [Gemini].
- **Pre-existing directory pairs** blocking new children is now stated [Gemini].
- **Gate step 5** separates a single-create push (no ref or pack write) from a mixed push (one pack)
  [Kimi].
- **Gate cleanup** trashes the gate subtrees and checks the row set **before** logout [Gemini].
- **The task pause** now starts before the first sign-in; the task is unpaused only on a passing
  Windows check, with a STOP path [Codex].
- **The CI check** was a test whose failure would stop the job before its fallback could run. It is
  now an explicit branching shell step, with `-count=1` on the suite [Kimi+DeepSeek+Gemini].
- **D5(d)** said both "temp repo" and "one call". It is now F3's measured single-call probe, placed
  before the lock [Gemini+DeepSeek+Kimi].
- **`releasing.md`'s list** now includes step 5 and the stand-in's asset-count and digest lines
  [Gemini+DeepSeek].
- **The certified SHA-512's home** is the Linux gate record, and the drift constant is a
  certification touch point [DeepSeek+Gemini+Kimi].

New, applied:

- AVX2 re-based on Proton's README text [Kimi+DeepSeek].
- Windows-side preconditions [Kimi].
- `contract_test.go`'s stale untouchable list (no `Archive`, no `Tax Documents`) [Gemini, confirmed].
- The confinement sentence covers trash [Kimi].
- `PROTON_DRIVE_CACHE_DIR` verified in the probe [Kimi].
- The F8 test pins an unrelated variable too [Kimi].
- The CLI install commands [DeepSeek].
- D4(a)'s refusal-only constraint [DeepSeek].
- The skipped-ref rationale made exact, with the repaired-skip path [DeepSeek].
- The macOS roadmap wording [Kimi].

**Round 3 (2026-10-16; all four reported; Gemini and DeepSeek said Blockers: none, DeepSeek
listing majors; this is the loop's last round, and its fixes have not been re-reviewed).**

Defects in round-2 fixes, applied:

- **Gate step 0** no longer accepts a Windows helper by its CLI token. It must match the draft's
  bytes, with its resolved path and hash recorded [Codex+DeepSeek].
- **The README's CLI hash line** now has a gate step that runs §6's commands as written
  [DeepSeek].
- **The skipped-ref wording** was too strong. The one way a push can lay down half of a pair is now
  stated as an exception, and round 1's rejection stands [DeepSeek].

New, applied:

- **The phase-5 re-check** sits before `ensureRefParents` and gets a hermetic test with a failing
  `Trash`; "Still untested" records that no gate step reaches it [DeepSeek].
- **The check set** uses `finalSet`'s algebra, and "create" means batch-aware non-existence
  [Gemini].
- **The rejection a user sees after phase 2 passed** is described [Kimi].
- **The ways a pair can appear without the user pushing one** are listed [Kimi].
- **A Windows check** runs after the contract table [Kimi].
- **The mixed push** is fetched and checked with `fsck` [Kimi].
- **D5(d)'s failure handling, cost and limits** are spelled out [Kimi+DeepSeek].
- **CI's git** floats on purpose [Kimi].
- **AVX2** is described as the CLI's requirement, with the honest-limit line [Kimi].
- **The install checks** run in a new login shell [Kimi].
- **D4(a)** gets a hermetic test [Kimi].
- **D2's cost** now covers tags and the absence of an override [DeepSeek].
- **The asset count** follows D1 [DeepSeek].
- **Sidecar defects** ship as a new tag [Kimi].

## Still untested, and how it gets tested

| Item | Tested by |
|---|---|
| `ubuntu-latest`'s git version | the CI probe step (§2) |
| `sha256sum -c` on the Windows-written sidecar | gate step 1 |
| `auth login` inside the interop-off distro | the pre-tag sign-in probe (§7) |
| `PROTON_DRIVE_CACHE_DIR` covering every state file | the pre-tag sign-in probe (§7) |
| A second CLI session's effect on the Windows one | the Windows checks (§7) |
| F13's shapes through the helper rather than bundles | gate step 5 |
| §5's phase-5 re-check (a same-batch delete failing in phase 4) | the hermetic test only; no gate step reaches it deterministically |
| The AVX2 requirement | Proton's README only; this x64 venue cannot test it |
| NTFS `$UpCase` against Unicode simple folding | documented limit; not tested |
| The arm64 and musl CLI tokens | out of scope under D1(a) |
| Whether any distro ships a `cli-drive-<distro>` build | D4 changes only a message |
| Linux CLI behaviour against the contract | gate step 2 |

End of the Linux port spec; the plan follows Craig's rulings on D1 to D8.
