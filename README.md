# git-proton-backup

**Git-native backups — git push your repos to Proton Drive's end-to-end encrypted storage.**

[![CI](https://github.com/craigstoller/git-proton-backup/actions/workflows/ci.yml/badge.svg)](https://github.com/craigstoller/git-proton-backup/actions/workflows/ci.yml)
(CI runs the unit/contract test suite only — no real Proton account or CLI is available in CI, so
the real end-to-end path is verified by hand; see [how it was built](docs/how-it-was-built.md).)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## The 30-second demo

```
PS> Install-ProtonBackup C:\code\myrepo
Wired. Back up with: git push proton   (status: Get-ProtonBackupStatus)

PS> git commit -am "feature"; git push proton
remote: confirmed on Proton

PS> git status
Your branch is up to date with 'proton/main'.
```

<!-- transcript verified against a real run 2026-07-20 -->

## Install & first run

**On Linux, only v2 applies** (v1's transport is the Windows sync app): go to
[Install on Linux (v2)](#install-on-linux-v2). The rest of this section is Windows and v1.

**Requirements:** Windows, PowerShell 7.4+, git, the Proton Drive desktop app, and the
[Proton Drive CLI](https://proton.me/drive) — optional but recommended. Without the CLI,
verification degrades to Windows Cloud Files sync-state only (see
[Honest limits](#honest-limits)); it never blocks install. Proton's CLI download page advertises
only the current release (an older direct link may linger for a while, as 0.8.0's did in October
2026, but do not count on it), so a fresh install gets the build `git-remote-proton` (v2, below)
certifies against — currently 0.9.0 (`cli-drive@0.9.0+75612299`). v1's own CLI use has no version
pin and degrades gracefully on an absent or mismatched CLI; v2 enforces the exact certified build.

```powershell
git clone https://github.com/craigstoller/git-proton-backup.git
cd git-proton-backup
.\install.ps1                    # copies the module into your $PSModulePath (+ the v2 helper exe, when present)
Import-Module GitProtonBackup
Initialize-ProtonBackup          # guided first run: finds the Proton Drive sync folder,
                                  # resolves the CLI, probes auth, writes config.json
```

Then wire a repo:

```powershell
Install-ProtonBackup C:\code\myrepo
```

Pass the repository's own top folder (a submodule's counts); a plain folder inside a repository is
refused rather than wired, and a linked worktree is refused in favour of its main checkout.

`Install-ProtonBackup` is idempotent — re-run it to repair a moved repo, a deleted mirror, or a
module upgrade (that's also what `Repair-ProtonBackup` does under the hood). Two switches:
`-SetUpstream` takes over the branch's upstream even if one is already set (by default, install
only claims an upstream that's unset); `-Force` replaces conflicting `proton`-remote wiring without
ever touching whatever that remote used to point at. `Uninstall-ProtonBackup` removes the remote,
hook, and mirror for a repo — existing bundles on Proton Drive are left in place.

Repairing a *moved* repo re-wires it at its new path, but its old path stays registered until you
deregister it — `Invoke-ProtonBackupVerify` will keep reporting that old entry as a repo missing on
disk, naming `Uninstall-ProtonBackup <old-path>` as the fix, until you run it.

## How it works

`git push proton` doesn't write into your Proton Drive folder directly — a folder full of loose
git objects, live-edited by a general-purpose file-sync client, is a fragile place to keep a git
repository. Instead, the push lands on a disposable local mirror, and that mirror's hook cuts a
single git bundle file and publishes it into the Proton-synced folder atomically: one file, one
upload, nothing for the sync client to ever observe half-written.

The mirror itself is bookkeeping, not custody. It's a bare repo wired as your `proton` remote,
created at install time — delete it and re-run `Install-ProtonBackup`, and nothing is lost, because
the bundle already sitting on Proton Drive is the actual backup, not the mirror.

Because a push can fail quietly — a network hiccup, Proton Drive signed out, the machine asleep —
`Invoke-ProtonBackupVerify` independently re-derives what *should* be backed up and re-cuts a bundle
whenever it's stale, whether or not anything flagged a problem along the way. Run it by hand, or
install it as a daily check. Full rationale for all of this: [docs/design.md](docs/design.md).
Curious how a non-developer shipped this? See [how it was built](docs/how-it-was-built.md).

## Two tools in this repo: bundles (v1) and a git remote (v2)

This repo ships two independent ways to get a git repo onto Proton Drive. They coexist safely in
one repo — different remote names, no shared mirror, no shared lock, no shared config — but they
are not equivalent, and the restore story is where that matters most.

| | v1 — bundles | v2 — `git-remote-proton` |
|---|---|---|
| Platforms | Windows (PowerShell 7.4+) | Windows (amd64) and, from v0.10.0, Linux (amd64) |
| Transport | Proton Drive's Windows sync app; a single bundle file lands in the synced folder | The Proton Drive CLI, invoked directly by the helper as a real git remote |
| Remote name | `proton` | `proton-v2` |
| Restore needs | git only, from any machine — the bundle itself is the backup, no account needed | git 2.45.0 or newer, `git-remote-proton` on PATH, and the certified Proton Drive CLI (0.9.0) signed in; on Linux, also a credential store the CLI can use ([Linux prerequisites](#install-on-linux-v2)) |

**Install v2 first — `git-remote-proton` doesn't land on PATH by default.** From v0.10.0, a [GitHub
Release](https://github.com/craigstoller/git-proton-backup/releases) carries five assets:
`git-remote-proton.exe`, `git-remote-proton.exe.sha256` and `install.ps1` for Windows, and
`git-remote-proton-linux-amd64` and `git-remote-proton-linux-amd64.sha256` for Linux (v0.9.0
carries only the three Windows assets). On Windows,
download the first three into one folder and run `install.ps1` there: it verifies the exe against
the `.sha256`, copies it to `%LOCALAPPDATA%\Programs\git-proton-backup`, and adds that folder to
your user PATH — open a **new** terminal afterward, since the script can't change its caller's own
session. On Linux, follow [Install on Linux (v2)](#install-on-linux-v2) below. Building from
source works too: `go build ./cmd/git-remote-proton` and put the resulting binary on PATH yourself.

**v2 fetch and clone need git 2.45.0 or newer.** The helper downloads only the packs a fetch
needs, and finds them by asking `git rev-list --missing=print` which objects are still absent.
Git reports the objects a fetch starts from that way only from 2.45.0 on; an older git stops the
fetch or clone with a `bad object` error, now followed by a note that selective fetch requires
git 2.45.0 or newer. Check with
`git --version`: Ubuntu 24.04, for one, ships 2.43.0. A push needs no particular git, but on an
older one it prints a single warning that this git cannot restore (it never blocks the push). That
warning only helps whoever reads push output: an unattended push on an old git succeeds with it
unread, so check `git --version` on any machine you may one day restore to.

```
git clone -o proton-v2 proton::/my-files/GitRemotes/myrepo
git push proton-v2 main
```

`git-remote-proton` serves the whole `refs/` tree, not just flat `refs/heads/` and `refs/tags/` —
hierarchical branch and tag names (`refs/heads/feature/x`), `refs/notes/*`, `refs/replace/*`, and
any other valid namespace are all advertised, fetchable, and pushable.

**Foreign files under `refs/` — dropped by another tool, a stray web-UI upload, or an accident —
are handled per operation, never silently.** A file whose contents are not valid refs (the helper
cannot tell a dropped junk file from a damaged ref, and says so) never stops backups: it's skipped
loudly on push, with a note naming the file and why. But it DOES stop `fetch`/`clone`/`ls-remote`
with an error naming every such file, so a restore is never silently incomplete — a clone that
quietly lacked a branch would be a false-success restore. This fetch-blocking class is exactly
**valid-ref-named files with unparseable contents**; files with **invalid names** stay note-only
in both directions, since a name git itself would reject can never be a ref, so its absence is
never loss. If you run a scheduled mirror-fetch job, it will alarm on a content-blocking file —
that's intended. Pushing onto one of these files (creating or updating a ref at that name) is
refused with the file named and a remedy. Git itself cannot delete such a name — `git push
--delete` on it fails client-side with "remote ref does not exist," since the helper never
advertised the name to begin with — so the remedy is always the CLI or the Proton Drive web UI
(`proton-drive filesystem trash <path>`); Proton trash keeps the deletion restorable.

Cloning a deeply nested repo on Windows can hit the legacy `MAX_PATH` limit — see
[Windows path length](#windows-path-length) below if a clone, fetch, or push fails with a
"Filename too long" style error.

Wiring both onto one repo is safe: install v1 as usual (`Install-ProtonBackup`), then add the
`proton-v2` remote alongside it. Full design: [docs/v2-remote-helper-design.md](docs/v2-remote-helper-design.md).

**Keep v2 remotes off `GitBackups`.** Point v2 at its own root — `/my-files/GitRemotes/` is the
convention used above — and never inside `GitBackups`, the folder v1's bundles live in.
Initializing a v2 repo hard-refuses a non-empty folder that has no v2 marker, so pointing it at
your populated `GitBackups` root fails safely — but an *empty* subfolder would be silently
adopted as a v2 repo, so don't create one there even as a placeholder.

**Restore contracts, stated honestly.** A v1 bundle restores with nothing but git —
`git clone <bundle-path>` — from any machine, any OS, no account needed at the moment of
restore. A v2 restore needs three things to be true at once: git 2.45.0 or newer, `git-remote-proton`
installed and on PATH, and the certified Proton Drive CLI — currently 0.9.0 — signed in (on Linux,
into a credential store the CLI can use). That is a real dependency v1 doesn't
have, not a footnote — plan around it if a v2 remote is ever your only copy of something.

**If a v2 push fails with "already exists" out of nowhere:** this has been observed once,
unexplained (writeup: [docs/research/gates/stage3b-gate.md](docs/research/gates/stage3b-gate.md),
run 1; a follow-up 30-trial provocation attempt could not reproduce it —
[docs/research/probes/c17b-provocation-log.md](docs/research/probes/c17b-provocation-log.md)).
Before clearing anything, capture evidence: run `proton-drive filesystem list` / `filesystem
info --json` on the failing path and note what the Proton Drive web UI's trash shows for it —
that capture is what turns a one-off into something reproducible. Only then remove, from the
trash, items whose names collide with the repo's remote path, and retry. The advice is scoped to
those homonyms — never "empty your trash" wholesale, which would take unrelated recoverable
files with it.

### Install on Linux (v2)

From v0.10.0, v2 runs on Linux; v1 does not, because its transport is the Windows sync app. Only
`linux/amd64` on glibc is built, and it is the only Linux build the release procedure covers: no
arm64 helper is built, and Proton's arm64 and musl CLI builds were not run.

**What is tested.** The helper is a static Go binary; the Proton Drive CLI it drives is Proton's
glibc `linux/x64` build. v0.10.0 passed the Linux live gate against the draft's exact bytes, on
Ubuntu 24.04 under WSL2 (the design's venue,
[spec D7](docs/superpowers/specs/2026-10-15-v2-linux-port-design.md); record:
[v0.10.0-linux-gate.md](docs/research/gates/v0.10.0-linux-gate.md)). Later releases follow
[docs/releasing.md](docs/releasing.md): a live gate, or, when eligible, a stand-in that gives the
Linux binary offline checks only. Other distributions are expected to work and are not gated.

**Prerequisites.**

- **git 2.45.0 or newer** for fetch and clone (above). Ubuntu 24.04's own git, 2.43.0, is below
  it. One upgrade route is the Ubuntu Git Maintainers PPA: `sudo add-apt-repository
  ppa:git-core/ppa`, then `sudo apt update && sudo apt install git`. The PPA carries whatever
  version it carries at the time, so confirm with `git --version`.
- **An AVX2 CPU.** This is the Proton CLI's requirement, not the helper's: Proton's README says
  its default `linux/x64` build crashes at startup with `Illegal instruction` on a CPU without
  AVX2 (see [Honest limits](#honest-limits)).
- **The certified Proton Drive CLI**, version 0.9.0 (`cli-drive@0.9.0+75612299`), installed with
  the commands below and checked against its SHA-512.
- **A credential store the CLI can use.** On Linux the CLI keeps its sign-in session in a Secret
  Service (GNOME Keyring, KWallet, anything that speaks `org.freedesktop.secrets`), and it has to be
  unlocked in the session that runs git. That is the store the Linux gate covered. With no session bus the CLI
  fails before any network attempt, and the helper passes its message through unchanged
  (`Failed to load session from secrets … Cannot autolaunch D-Bus`). The CLI's other store,
  `pass` (`PROTON_DRIVE_CREDENTIALS_STORE=pass`), reaches the CLI untouched: the helper sets no
  `PROTON_DRIVE_*` variable and passes the CLI no environment of its own, so the CLI inherits the
  helper's (the helper changes only git's own variables: it unsets `GIT_DIR` and makes relative
  `GIT_COMMON_DIR` and `GIT_OBJECT_DIRECTORY` paths absolute). It is **not gated**. [Proton's CLI
  README](https://github.com/ProtonDriveApps/sdk/blob/main/cli/README.md) describes both stores.

**Install.** Download the two Linux assets of a v0.10.0 or later release into one folder and run
this there:

```bash
# The helper:
mkdir -p ~/.local/bin
sha256sum -c git-remote-proton-linux-amd64.sha256 && install -m 0755 git-remote-proton-linux-amd64 ~/.local/bin/git-remote-proton

# Proton's CLI, certified version 0.9.0 (SHA-512 certified 2026-10-22 by the Linux live gate):
curl -fLo proton-drive https://proton.me/download/drive/cli/0.9.0/linux-x64/proton-drive
echo "3533025ba69ae112b64e3e01fbcc1ad0688136a4043f6cf6a72886967d85fdcd9ec235479c2e113171614be5225bfba93427509a05ca5aa6071d924fa7e91ca8  proton-drive" | sha512sum -c && install -m 0755 proton-drive ~/.local/bin/proton-drive
```

The SHA-512 is the value Proton's feed listed for this build on 2026-10-16, and it equals the
binary the v0.10.0 Linux live gate ran on 2026-10-22
([record](docs/research/gates/v0.10.0-linux-gate.md)). Each `install` is chained to its check with
`&&`, so a
failed check stops the install. A passing check means you hold the bytes that were tested; a
failing one means they are not (Proton may have re-cut the build, or the download is damaged), so
do not install it. Proton's download page advertises only the current release, so this
versioned URL may stop serving once Proton moves on (see [Install & first run](#install--first-run)).

On Ubuntu, a freshly created `~/.local/bin` joins your PATH at your next login. Until then,
`~/.local/bin/git-remote-proton --version` confirms the install directly (it prints the tag and
the certified CLI token). In a **new login shell**, check that each name resolves to exactly one
path, the one you just installed:

```bash
type -a git-remote-proton
type -a proton-drive
```

That is the Linux form of `install.ps1`'s shadow check, which exists because a stale helper
earlier on PATH once passed for the new one. Then sign in with `proton-drive auth login` and use
v2 as above.

A Proton CLI from your distribution's package manager is a different build and is refused. Proton's
README says a CLI repackaged for a distribution may identify itself as `cli-drive-<distro>`; the
helper accepts only the exact certified token, and for such a build the refusal says it looks like
a distribution repackage and points to the README's install section. Whether any distribution ships one today
is untested.

### Environment variables

`git-remote-proton` (v2) reads two opt-in environment variables. Both are read fresh from the
environment on every invocation — never cached, never remembered across runs — so setting or
unsetting one takes effect on the very next command.

- **`GPB_UNCERTIFIED_CLI=1`** overrides the certified-CLI allowlist. By default the helper refuses
  to run against anything but the exact certified Proton Drive CLI build — currently
  `cli-drive@0.9.0+75612299` — naming what it found versus what's certified. Setting this proceeds
  anyway, printing a loud stderr warning naming the untested (or undetermined) CLI version on every
  invocation. A formerly certified 0.8.0 gets that generic warning and nothing more (0.9.0 kept
  0.8.0's upload vocabulary, so nothing is known to break). **The override does not make an older
  CLI viable for pushes that update refs:** if the detected build is specifically the 0.7.0 build
  certified through v0.6.0, the warning names that incompatibility directly — 0.8.0 replaced the
  general upload conflict-strategy option with separate file- and folder-scoped ones, and 0.7.0's
  update path has no way to speak the new file-side vocabulary. Meant for troubleshooting a CLI
  upgrade, not for routine use.
- **`GPB_CREATE_PARENTS=1`** lets a `push` create missing parent folders above the repo root (for
  example `/my-files/GitRemotes` when only `/my-files` exists), instead of the default: an
  actionable refusal naming the exact `proton-drive filesystem create-folder` command to run by
  hand. **This is a real trade, not a free convenience: a typo'd remote address fails loudly by
  default, and setting this variable trades that safety net away** — a mistyped path no longer
  gets caught before it creates something, it just gets a folder tree built at the wrong location.
  Missing parents are created one at a time, each logged to stderr as it happens, bounded to
  strictly below the Drive mount (`/my-files` or `/devices/<id>` themselves are never created,
  since a mount isn't creatable storage). There is no rollback if a later step fails — whatever
  was already created stays created, and the stderr lines are the record of what happened. Applies
  to `push` only; `--set-head` never honours it, since it only ever points HEAD at a branch that
  must already exist.

### Windows path length

A deep clone destination can exceed Windows' legacy 260-character `MAX_PATH` limit — the full
path to an object inside `.git\objects\pack\` adds up fast once the repo's own history is long and
the destination is nested a few folders deep. When that happens, two remedies apply:

- **`git config core.longpaths true`** (repo-local or `--global`) lets git's own writes exceed the
  limit. This is separate from — and required in addition to — Windows' own
  `LongPathsEnabled` registry setting: the OS-level setting alone does not make git itself opt in.
- **A shorter destination** always helps, regardless of `core.longpaths`, since it's the total
  path length that matters.

`git-remote-proton` (v2) does its own pack writing inside `internal/gitcmd`, and a failure there
that involves a path 240 characters or longer gets a best-effort hint appended naming both
remedies — a *possible* cause, since the same failure can have other causes, never asserted as
certain. That hint can only fire while the helper itself is running (advertise/fetch/push). The
**checkout phase** — git materializing files into your working tree after a clone or fetch
completes — happens entirely after the helper has already exited, so no helper hint is possible
there; if a `git clone` or `git checkout` fails with a "Filename too long" style error, the same
two remedies above are the only fix.

## Why PowerShell?

Because the domain is Windows, and PowerShell is what Windows already speaks.

- **The transport is the Proton Drive *Windows* sync app** — bundles are published into its sync
  folder, and upload state is read straight from the Windows Cloud Files API. That API call is a
  few lines of inline P/Invoke here; it isn't portable, and neither is the transport.
- **Zero runtime for the audience.** The people this is for — Windows users with git and a Proton
  account — install nothing to run it: no Python, no packaging manager, no binary to trust.
  `install.ps1` copies a module — plus, when a helper exe sits beside the script, that exe under
  `%LOCALAPPDATA%\Programs\git-proton-backup` and a user-PATH entry — that's the whole footprint.
- **The job is glue.** Orchestrating git, a vendor CLI, filesystem state, and Task Scheduler is
  exactly what a shell is for. A compiled language would add build weight to a tool whose honest
  job is coordination, not computation.

The Windows-specific part is the transport, not the language. v1 stays Windows-only; Linux has the
v2 remote helper instead ([Install on Linux](#install-on-linux-v2)), and the [Roadmap](#roadmap)
says what macOS would take.

## Monitoring

- `Get-ProtonBackupStatus` (add `-Json` for scripting) — per-repo wiring health, whether the current
  commit state is bundled, confirmation from the last verify run, and any pending marker.
- `Invoke-ProtonBackupVerify` — run it ad hoc, or install `Install-ProtonBackupTask` for a daily
  scheduled check (interactive logon, since both the CLI session and the sync app live in your
  desktop session). Freshly cut bundles — by the run itself, or moments before it — get a short
  upload-lag grace, one shared `VerifySeconds` window for the whole fleet, before being reported
  unconfirmed, so the first run after a stretch of downtime doesn't false-alarm every repo at once
  while the sync app catches up.
- **Local toast, no external service** — pipe the exit code into a one-liner. With the
  [BurntToast](https://github.com/Windos/BurntToast) module (`Install-Module BurntToast`):
  `if ((Invoke-ProtonBackupVerify).ExitCode -ne 0) { New-BurntToastNotification -Text 'GitProtonBackup', 'attention needed' }`.
  On Windows Pro/Enterprise, `msg` works with no extra module —
  `if ((Invoke-ProtonBackupVerify).ExitCode -ne 0) { msg $env:USERNAME "GitProtonBackup: attention needed" }`
  — but `msg.exe` isn't present on Windows Home, so BurntToast is the option that works everywhere.
- **Heartbeat** — optional dead-man's-switch: point `HeartbeatUrl` at a healthchecks.io/Cronitor/
  Uptime-Kuma check; the service sees a ping, never your data — though note that whichever provider
  you point it at does see your source IP and the timestamp of every ping, the same as any HTTP
  request to any web service.

## Restore

No plugin, no account, no special client — just git.

```powershell
git bundle verify <path-to.bundle>
git clone <path-to.bundle> C:\restore\myrepo
```

To pull new commits from a bundle into a repo you already have:

```powershell
git fetch <path-to.bundle> <branch>
```

Bundles live under `<Proton Drive>\<BackupSubdir>\<repo-slug>\` (`BackupSubdir` defaults to
`GitBackups`). Any device signed into your Proton account — desktop, mobile, or the web app — can
download them; you don't need this tool, or even a Windows machine, to get your history back.

## Honest limits

- **Git LFS:** bundles carry LFS pointer files, not the LFS objects themselves.
- **Submodules:** a bundle covers the superproject only — wire each submodule as its own repo.
- **Shallow clones:** refused at install; git bundles from a shallow repo are unreliable. Un-shallow
  first (`git fetch --unshallow`).
- **Proton Drive CLI absent or signed out:** verification degrades to Cloud Files sync state
  only — never fatal, but a weaker guarantee than CLI confirmation, and blind to one real
  failure: a sync app that falsely marks files in-sync while uploading nothing looks *healthy* to
  it. Worse than merely looking healthy: degraded verification acts on that false state — verify
  reports ok (exit 0), clears push-pending markers, and lets retention prune the local spool,
  all while nothing has reached Proton (CLI verification is what catches this — next bullet).
- **Proton Drive sync stopped:** bundles pile up locally until sync resumes. When the CLI is
  available, `Invoke-ProtonBackupVerify` catches the tell-tale contradiction on its next run —
  bundles marked in-sync locally yet absent on Proton at the same instant — and says so ("sync
  app appears stalled … restart the Proton Drive app"; diagnosed when two or more repos show it
  at once, or when a lone repo shows it and a nonzero grace window is configured — the strongest
  case being a freshly cut bundle polled across the whole window; an already-old spool is judged
  on its age plus a same-instant double-check, not continuous observation). Without the CLI, the
  backlog still surfaces once it's older than `MaxUnconfirmedAgeDays` (default 7) — but only
  while the files stay honestly pending: a stall that falsely marks them in-sync is exactly the
  blind spot the previous bullet describes, and no age check can see it.
- **One machine per repo:** there's no multi-machine coordination. A second machine pushing the same
  repo is safe but confusing — digest-suffixed filenames prevent corruption, not confusion.
- **Worktrees:** a bundle carries full ref history — every branch and tag — but not the checked-out
  working-tree state of any secondary `git worktree` attached to the repo. History is complete;
  restoring a linked worktree's own working copy afterward is a manual `git worktree add` from that
  history, not something the bundle reproduces automatically.
- **v2: refs that differ only by letter case (`main` and `Main`).** Proton stores both, and a
  case-sensitive client such as Linux can push both. A Windows clone of such a remote succeeds, and
  local use then goes wrong in one of three ways (measured by cloning bundles onto NTFS, not
  through the helper):
  - `main` beside `Main` (two branches): `git switch Main` puts you on `Main` at `main`'s commit
    and files, silently, while `main` is still a loose ref file. Once the refs are packed, the same
    command works.
  - `feat` beside `Feat/x` (a branch against a folder of the same name): the second local branch is
    refused, loudly (`cannot lock ref`).
  - `grp/a` beside `Grp/b` (two folders): the second local branch is silently re-cased to `grp/b`.

  So, on every platform, a push **refuses to create** a ref whose name differs only by letter case
  from another ref's (any component of the path counts, so all three shapes above are covered). The
  refusal names every conflicting ref, and git reports that ref rejected. A create is a name the
  remote does not hold yet: a name it already holds is never refused by this rule when the push
  updates it (forced or not) or makes an ordinary delete of it. The one delete this check refuses
  is the delete half of a case-changing rename beside a surviving variant (delete `Main`, create
  `MAIN`, with `main` present): when it refuses the create, it refuses the delete too, so `Main`
  survives. A create refused for another reason (for example a ref and a folder of refs under the
  same name, or a name a skipped foreign file occupies) gets git's ordinary per-ref treatment: its
  paired delete still goes through, and the refused create's commit stays in your local repo.
  There is no override. Case is folded the way Go's `strings.EqualFold` does, Unicode simple
  folding. NTFS folds with its own table, which probably differs for a handful of characters, and
  Unicode normalisation (NFC against NFD, as on macOS) is out of scope; neither is checked.
  - **One loss remains.** If a rename passes the check because the variant it conflicts with is
    deleted in the same push, and that delete then fails (a race or a transport fault) between the
    push's delete step and its create step, the create is refused after the rename's own delete
    has removed the old name. That old name is then gone from the remote. Only a hermetic test with
    a faked delete failure reaches the refusal; no live gate can reach it deterministically.
  - **A pair can still appear without you pushing one:** from pushes made before this check
    existed; from a Proton CLI or web-UI upload, as the 0.9.0 certification made one; and from a
    skipped foreign file (the v2 section above describes them) that is later repaired. A ref whose
    case variant sits in a skipped file is not checked, because skipped names never reach a clone,
    so a push can create the other half of a pair there; the pair becomes visible to clones once
    the file is repaired.
  - **Repair a pair** by deleting one member (`git push proton-v2 --delete Main`). A remote that
    already holds a pair keeps taking pushes to its other refs, but under a directory pair
    (`Feat/a` beside `feat/b`) a new ref under either spelling is refused until the pair is
    repaired.
- **v2: what happens to the remote lock when a push is killed.** A push or a `--set-head` run
  holds a `.lock` file on the remote, and v2 has no lock takeover, so a lock left behind fails
  every later push with `repo is locked` until someone removes it. When the helper receives a
  signal it can catch, it runs one bounded release before it exits: it retries a failed call until
  its lock reaches an outcome or a 60 s cap passes. That is an attempt; the helper cannot promise
  the lock is gone. The v6.11 entry in the [design's revision
  history](docs/v2-remote-helper-design.md) has the detail.
  - On Linux, SIGTERM, SIGINT and SIGHUP start the attempt, whether they come from `kill`, from
    `timeout` or systemd. Ctrl-C and a closed terminal are expected to start it through the same
    signals; neither has been run in a terminal. A signal that was already ignored when the helper
    started, such as SIGHUP under `nohup`, starts nothing and prints nothing: the push carries on,
    and the lock is then released only when the helper exits in the ordinary way, however the
    push ends (a failed push included), not by the signal. Under systemd the attempt needs
    `KillMode=control-group` (the default) and `TimeoutStopSec=` of 75 s or more. The 75 s is the
    design's recommendation: 15 s over the 60 s cap, and 13 s over the 62 s worst case below, which
    has not been run end to end at full length.
  - On Windows, Ctrl-C and Ctrl-Break in a console window (Windows Terminal or conhost) start the
    attempt and leave it time to finish. Ctrl-Break has been tested by sending it straight to the
    helper, against a stand-in for the Proton CLI. Ctrl-C is expected to behave the same, because
    Go delivers both as an interrupt, but it has not been run. Neither has been run under a real
    `git push`. Closing the console window starts the attempt too, but it cannot finish (see the
    case with no result line, below).
  - If only git dies, no signal arrives. The helper notices when git's end of its pipes closes,
    which the operating system does when git exits. A batch that git never finished sending is
    refused unapplied, and the helper releases the lock the usual way and exits 1. A batch it had
    already received runs to its end, its status writes fail, and it exits 1. With no batch in
    progress it reads the end of its input, releases the lock and exits 0. A git that stays alive
    but stops reading would not be noticed.
  - Once started, the helper stops the Proton CLI call that was running, waits 5 s in case that
    call's last request is still arriving at the server, then reads the remote `.lock` and
    removes it only if it carries this helper's own nonce. A failed release call is retried until
    the lock reaches an outcome or the cap passes. The cap runs from the first signal, so the stop
    and the 5 s wait come out of it. The helper exits within 62 s of the first signal at worst (a
    60 s limit, plus up to 1 s to stop a release call still running and up to 1 s to print the
    result). That bounds how long the helper lives, not whether the lock is gone: only the result
    line says that. Its exit code is 128 plus the signal's number: 143 for SIGTERM, 129 for
    SIGHUP, and 130 for SIGINT and for both Ctrl-C and Ctrl-Break, which it reports as SIGINT. A
    script that runs `git push` sees git's own exit code, or `timeout`'s 124, not the helper's.
    When the signal also reaches git, as with `timeout` or Ctrl-C, git exits at once, so
    `timeout` or the shell prompt can return while the helper is still releasing, for up to 62 s,
    and a push of the same repo started in that time meets the old lock (`repo is locked`). A
    second signal while the helper works is noted on stderr and ignored. A signal also skips the
    helper's local cleanups, as before: a staging or download temp folder, or a `git pack-objects`
    that was building a pack, can be left behind. The tests
    run against a stand-in for the Proton CLI; how the real CLI behaves when it is signalled, and
    how long a release takes against it, have not been measured.
  - For each lock the helper then prints its path, its nonce and one outcome:
    - *removed*: the lock was ours and is gone.
    - *absent*: the remote showed no lock of ours at that moment. Either it was never created, or
      it was already removed, as when the signal arrived while this push's own release was
      finishing. A request the killed CLI call had already sent can still be applied later than
      the 5 s wait, so a lock could appear afterwards. It would carry the nonce printed on this
      line, and it is yours to remove. How late a request can land has not been measured, so
      check the remote again later before assuming it is clean.
    - *foreign*: another process holds the lock (a different nonce). Left alone.
    - *unreadable*: a lock is there but its contents did not parse, so it cannot be shown to be
      ours. Not removed and not retried; it needs a person.
    - *failed*: the release calls kept failing, or a removal was never confirmed, until the time
      limit. The line gives the reason and the manual remedy.
    - *not attempted*: no release ran, either because the CLI call in flight did not confirm that
      it had exited (removing the lock then could race that call's own write) or because the
      time limit came first. The line gives the manual remedy. Locks are released one after
      another, and a lock whose release never reaches a final outcome uses the rest of the 60 s
      cap, so any later lock reports *not attempted*. The helper normally holds one lock: it has one
      repo address and so one `.lock` path, and git sends one `list for-push` per session. A
      second try at that path can show up here only in a session git does not produce: while it
      is under way, or afterwards if it ended in doubt or succeeded (it can succeed only if the
      first lock was removed by hand meanwhile). Either way, every lock the helper took in a
      session is released when it exits. If the second try ended in doubt, the helper keeps the
      lock it already held and releases the second try first, after the 5 s wait, then the held
      one. If it succeeded, the new lock is the one held, and releasing the first attempt finds
      nothing of its own and changes nothing. Separately, any attempt to take the lock, the first
      one or a `--set-head` run's, whose upload was reported refused but whose lock carries the
      nonce that same attempt uploaded is treated as ours and in doubt, and released. That case is
      untested: it needs the Proton CLI to retry an upload internally.
  - Anything that gives the helper no signal it can catch still leaves the lock: SIGKILL, the
    OOM killer and power loss; under systemd, `KillMode=mixed` or `process`; and SIGQUIT, which
    keeps Go's goroutine dump on purpose. On Windows that means Git Bash's `kill -TERM` and
    `kill -HUP` (a `kill -INT` does nothing to a native Windows program) and `Stop-Process`,
    all measured on a probe program, not on the helper; `taskkill /F`, inferred to act like
    `Stop-Process`; logoff and shutdown, which Windows sends only to services; and a Task
    Scheduler stop or time limit, inferred to end the process without a console event, and
    untested. Ctrl-C in Git Bash's own window is also
    assumed to leave the lock, and untested. The one leak on record, an agent tool's timeout
    that killed a push on Windows ([Stage 5 gate](docs/research/gates/stage5-gate.md), S1),
    probably belongs in this group: the gate record says only that the tool's timeout killed the
    push, and that the kill came through Git Bash is inferred.
  - Two situations print no result line. Windows ends the process 5 s after a console window is
    closed. The helper does start its attempt, but it waits 5 s before its first release call, so
    the release cannot begin in time and nothing is printed. After a console close the lock
    stays, unless the push's own release was already under way and removes it. And if stderr is
    blocked or gone (a pipe nobody reads, a closed terminal), the lines are dropped: the release
    still runs, and the helper waits at most 1 s for the write. In both cases the lock is either
    gone or still there, so check.
  - To remove a leftover lock by hand, start from the lock-held error, which names the lock's
    path. First make sure no push or `--set-head` for that repo is running: after a signal the old
    helper can keep releasing for up to 62 s, and that error can appear in the meantime. Then
    trash the lock with `proton-drive filesystem trash <repo path>/.lock`.
- **v2 on Linux: unattended use is not gated.** Only attended use is in scope for the Linux gate
  and supported: a session with an unlocked Secret Service. Cron or systemd-timer runs are not
  covered. The helper
  takes no part in credentials, and the CLI needs an unlocked Secret Service (or `pass`), which a
  headless session does not have unless you arrange it. Without one, the push fails with the
  CLI's own message (`Failed to load session from secrets …`). Unlocking headless always means
  storing a password, or decrypting without a prompt, somewhere; that trade-off is yours, not a
  default shipped here.
- **v2 on Linux: Proton CLI drift.** The helper accepts one exact CLI build. A newer Proton CLI
  reaches you as a refusal (`GPB_UNCERTIFIED_CLI=1` overrides it, see [Environment
  variables](#environment-variables)). A re-cut of the same version number passes the helper's
  check, which matches the identity token only; catching it is the maintainer's drift check
  (`scripts/Test-ProtonCliDrift.ps1`, which compares Proton's feed with the recorded Linux
  SHA-512). The `sha512sum -c` line in the install commands is your own check of the download.
- **v2 on Linux: no certified CLI for CPUs without AVX2.** The certified `linux/x64` build needs
  AVX2. Proton's README names a `linux/x64-baseline` build for older CPUs, but Proton's 0.9.0 feed
  lists no such build, so there is nothing to certify. On such a CPU the CLI crashes at startup
  (`Illegal instruction`). The helper's first CLI call is its version check, so the message you
  most likely see is that check's refusal, `Proton CLI version could not be determined (… signal:
  illegal instruction) …`, ending `or install the certified CLI`. That is the AVX2 tell, not a
  wrong CLI version [inference, untested]. Only Proton's README says the CLI needs AVX2; the gate's
  x64 venue cannot test it.

> **What this tool does and doesn't encrypt:** git-proton-backup performs no encryption itself.
> Bundles are ordinary git bundles; anyone with access to your Windows account or your Proton
> account can read them. End-to-end encryption is provided by Proton Drive for transport and cloud
> storage.

## Documentation

The docs index at [docs/README.md](docs/README.md) maps everything: the two designs, the release
procedure, the research evidence (gates and probes), and the dated spec/plan pairs feature work
uses.

## Support

Built for my own use and shared as-is. Issues and PRs are welcome; no support or response time is
promised.

## Roadmap

- **macOS** — an aspiration only, if there's demonstrated interest. Nothing is built, gated or
  documented for it.
- **Linux** — `git-remote-proton` (v2) gained Linux support in v0.10.0: CLI-direct
  upload, bypassing the sync-folder transport entirely, as a linux/amd64 helper driving Proton's
  glibc CLI. It passed its live gate on Ubuntu 24.04 under WSL2 (see [Install on
  Linux](#install-on-linux-v2)). Still open, and not gated: other architectures and musl,
  unattended use, and other credential stores (see [Honest limits](#honest-limits)). For v1 specifically, the alternative is waiting for Proton to
  ship a Linux sync client.
- **PowerShell Gallery** — the manifest is Gallery-ready; publishing is a fast-follow, not a v1
  blocker.

## Disclaimer

This project is not affiliated with or endorsed by Proton AG. "Proton Drive" is a trademark of
Proton AG.
