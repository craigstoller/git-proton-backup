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

**On Linux and macOS, only v2 applies** (v1's transport is the Windows sync app): go to
[Install on Linux (v2)](#install-on-linux-v2) or [Install on macOS (v2)](#install-on-macos-v2).
The rest of this section is Windows and v1.

**Requirements on Windows:** PowerShell 7.4+, git, the Proton Drive desktop app, and the
[Proton Drive CLI](https://proton.me/drive), which is optional but recommended. Without the CLI,
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
| Platforms | Windows (PowerShell 7.4+) | Windows (amd64) and, from v0.10.0, Linux (amd64); macOS (Apple Silicon) built from source, not yet live-gated ([Install on macOS](#install-on-macos-v2)) |
| Transport | Proton Drive's Windows sync app; a single bundle file lands in the synced folder | The Proton Drive CLI, invoked directly by the helper as a real git remote |
| Remote name | `proton` | `proton-v2` |
| Restore needs | git only, from any machine — the bundle itself is the backup, no account needed | git 2.45.0 or newer, `git-remote-proton` on PATH, and the certified Proton Drive CLI (0.9.0) signed in; on Linux, also a credential store the CLI can use ([Linux prerequisites](#install-on-linux-v2)) |

**Install v2 first — `git-remote-proton` doesn't land on PATH by default.** From v0.10.0, a [GitHub
Release](https://github.com/craigstoller/git-proton-backup/releases) carries five assets:
`git-remote-proton.exe`, `git-remote-proton.exe.sha256` and `install.ps1` for Windows, and
`git-remote-proton-linux-amd64` and `git-remote-proton-linux-amd64.sha256` for Linux (v0.9.0
carries only the three Windows assets). No release carries a macOS binary: on a Mac, build the
helper from the release tag ([Install on macOS (v2)](#install-on-macos-v2)). On Windows,
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

### Install on macOS (v2)

**Status: built from source; the hermetic suite runs on it in CI; not yet live-gated.** v2 runs on
Apple Silicon Macs (`darwin/arm64`). Intel Macs are out of scope, and v1 does not run on macOS,
because its transport is the Windows sync app. No release carries a macOS binary, so no release
gate runs Mac bytes: you build the helper yourself from a release tag. CI runs the hermetic Go
suite on an Apple Silicon runner at Go's latest stable release, and the port's design rests on
local probes run on one Mac
([docs/research/probes/macos-port/](docs/research/probes/macos-port/README.md)). The M-numbers
below are findings in the
[macOS port spec](docs/superpowers/specs/2026-10-27-v2-macos-port-design.md), which marks how each
was measured. No live gate has run on a Mac yet. On a Mac the helper also refuses some fetches
and pushes that would let the filesystem mix two refs up; read
[Honest limits on macOS](#honest-limits-on-macos-v2) before you rely on it.

**Prerequisites.**

- **An Apple Silicon Mac, and a logged-in macOS user.** The Proton CLI keeps its sign-in session
  in that user's login keychain by default, which needs no setup: signed out, the CLI looked the
  session up in the keychain and found none, with no prompt (measured). The CLI's other stores,
  `pass` and `unsafe_file`, are not gated.
- **git 2.45.0 or newer** for fetch and clone (above). Apple's git, from the Xcode Command Line
  Tools, was 2.54.0 on the Mac the port was built on, which meets it. Confirm yours with
  `git --version`.
- **Go, a current release,** to build the helper. The port was built and tested on the Mac with
  go1.27.0, and CI uses Go's latest stable release. The older minimum in `go.mod` is tested on
  Linux only, so it is not claimed for macOS.
- **The certified Proton Drive CLI**, version 0.9.0 (`cli-drive@0.9.0+75612299`), Proton's
  `macos/arm64` build, installed with the commands below and checked against its SHA-512. It prints
  the same identity token as the Windows and Linux builds, so the helper accepts it unchanged
  (measured offline, M4).

**Install.** Build the helper in a fresh clone at the release tag, so the build records no local
edits. Set `TAG` to the release you are installing. These commands put both programs in
`~/.local/bin`:

```bash
# The helper, built from source at the release tag:
TAG=vX.Y.Z
git clone --branch "$TAG" https://github.com/craigstoller/git-proton-backup.git "git-proton-backup-$TAG"
cd "git-proton-backup-$TAG"
mkdir -p ~/.local/bin
CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=$TAG" -o ~/.local/bin/git-remote-proton ./cmd/git-remote-proton
~/.local/bin/git-remote-proton --version
cd ..

# Proton's CLI, certified version 0.9.0, macOS build (SHA-512 recorded 2026-10-27; no live gate has run it yet):
curl -fLo proton-drive https://proton.me/download/drive/cli/0.9.0/darwin-arm64/proton-drive \
  && echo "3f4125de5e41dcc7f4e0e07e8d60737aa18f35c6ecc1ba3fce47ea97b331857eefaf88e74671135af57122d4cb5439b817ec528f8df383550ac7810aeaf0217b  proton-drive" | shasum -a 512 -c && install -m 0755 proton-drive ~/.local/bin/proton-drive
```

`--version` must print the tag and the certified CLI token: `git-remote-proton vX.Y.Z (certified
CLI: cli-drive@0.9.0+75612299)`. The SHA-512 is the value Proton's feed listed for this build on
2026-10-27, and it equals `shasum -a 512` of a download of that URL made the same day. The
download, the check and the install are chained with `&&`, so a failed download or a failed check
installs nothing. A failing check means you do not hold the bytes that were recorded (Proton may
have re-cut the build, or the download is damaged), so do not install it. Proton's download page
advertises only the current release, so this versioned URL may stop serving once Proton moves on.

`~/.local/bin` is not on a Mac's default PATH. Add it for login shells, which is what a new
Terminal window starts:

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zprofile
```

In a **new login shell**, check that each name resolves to exactly one path, the one you just
installed:

```bash
type -a git-remote-proton
type -a proton-drive
```

That is the Mac form of `install.ps1`'s shadow check (see the Linux section). Then sign in with
`proton-drive auth login` and use v2 as above. On a Mac, consider cloning with
`--ref-format=reftable` (`git clone --ref-format=reftable -o proton-v2
proton::/my-files/GitRemotes/myrepo`): none of the Mac refusals applies to such a clone.

**A helper binary that arrived some other way.** No macOS binary is published. A helper copied
from another Mac by browser, mail, AirDrop or an archive can carry macOS's quarantine attribute,
and git then reports `error: git-remote-proton died of signal 9` and `fatal: remote helper 'proton'
aborted session` (measured once, with the attribute a Safari download sets, M18). Confirm the
attribute before you act on it: `xattr -p com.apple.quarantine ~/.local/bin/git-remote-proton`
prints it, or reports `No such xattr`. Rebuild from the tag rather than run a file whose origin you
do not know. Removing the attribute, `xattr -d com.apple.quarantine <file>`, is what made a
known-good copy run in the probes (M20). A helper you build yourself carries no quarantine
attribute, and Go's linker signs it ad hoc, so it runs (measured, M9). Proton's CLI is notarized,
and a quarantined copy of it ran normally (measured, M3 and M19).

### Scheduled use on macOS (v2)

`scripts/macos/gpb-v2-push.sh` backs one repository up for a LaunchAgent, and
`scripts/macos/gpb-v2-push.plist.example` is a LaunchAgent that runs it once a day at 03:15. Give
each repository its own LaunchAgent, and push each repository from one machine only: v2 has no
multi-machine coordination.

The wrapper runs `git -C <repository> push proton-v2 --all` and then `git -C <repository> push
proton-v2 --tags`, so every branch and every tag is backed up. git refuses `--all` and `--tags` in
one command, and a bare `git push` sends only the current branch. The second push runs even when
the first fails. Neither uses `--mirror`, so a scheduled run never deletes a remote ref.

To set one up from your clone of this repository, copy the wrapper into place and make a
LaunchAgent from the example:

```bash
install -m 0755 scripts/macos/gpb-v2-push.sh ~/.local/bin/gpb-v2-push.sh
cp scripts/macos/gpb-v2-push.plist.example ~/Library/LaunchAgents/com.example.gpb-v2-push.myrepo.plist
# Edit the copy first: its label, and every /Users/YOU/... path in it.
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.example.gpb-v2-push.myrepo.plist
```

`launchctl bootout` with the same arguments removes it. launchd does not expand `~`, starts the job
in `/`, reads no `~/.zprofile`, and gives it the PATH `/usr/bin:/bin:/usr/sbin:/sbin`, so every
path in the plist is absolute and the wrapper sets PATH itself. A daily run the Mac slept through
runs at the next wake. The wrapper's settings are environment variables in the plist; the helper
reads none of them:

- `GPB_PATH` becomes the whole PATH for git, the helper and the CLI. Its default is
  `$HOME/.local/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin`. A value you
  set replaces that list rather than adding to it, so keep `/usr/bin:/bin` in it.
- `GPB_PUSH_TIMEOUT` is each push's limit in seconds, 1800 by default.
- `GPB_MARKER_DIR` is where a failure marker goes, `~/Library/Logs` by default.
- `GPB_NOTIFY` names one program, by name or path and with no arguments, that is run with the
  failure message as its only argument in place of the macOS notification.
- `PROTON_DRIVE_CACHE_DIR`, when set, gives the Proton CLI a cache folder of its own.

**When a run fails,** the wrapper posts a macOS notification and writes a marker file,
`gpb-v2-push.<number>.failed` in the marker folder. The number is a checksum of the repository's
physical path (as `pwd -P` prints it) and the remote's name. The marker names the repository, the
remote and each push that failed, and the next successful run for the same repository and remote
removes it. git's own output is in the two log files the plist names.

- **A refusal needs a repair before the next run.** If the log shows the helper refusing a ref,
  or the whole push, the next run will refuse it again, so repair what the message names (see
  [Honest limits](#honest-limits)) before you count on that backup.
- **A push that runs past its limit is ended**, with its whole process group (git, the helper and
  the CLI), and counts as a failure. It may leave the remote's lock behind, and the next push then
  stops with `repo is locked by nonce <nonce> on <host> (pid <pid>) since <time>; if that process
  is gone, remove <path> with the Proton CLI`. Before you remove that lock, check that `<host>` is
  this Mac's name, as `hostname` prints it, and that `ps -p <pid>` finds no such process. If it
  names another host, leave the lock alone and find out which machine is pushing.
- **After each Proton CLI upgrade, check a scheduled backup by hand:** start the job once with
  `launchctl kickstart gui/$(id -u)/<label>` and read its log. A re-signed CLI may lose its access
  to the keychain item and raise a prompt that a scheduled run cannot answer [untested; an
  inference from how Keychain access control works].

**Not gated:** the `pass` and `unsafe_file` credential stores; SSH sessions, where the login
keychain may be locked; a LaunchDaemon, which has no user session; runs during sleep or dark wake;
and a Keychain prompt after a CLI upgrade.

**Give a scheduled repository a `reftable` clone.** A repository cloned with
`--ref-format=reftable` keeps its refs in one table instead of one file per ref, and none of the
Mac refusals ([Honest limits on macOS](#honest-limits-on-macos-v2)) applies to it, so a scheduled
push from it cannot stop on one. The create rule that applies on every platform still does. A
`reftable` clone kept both spellings of a name apart where a `files` clone mixed them up (measured
with local repositories; through the helper, not yet tested). It needs git 2.45.0 or newer, the
same floor as fetch.

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

The Windows-specific part is the transport, not the language. v1 stays Windows-only; Linux and
macOS have the v2 remote helper instead ([Install on Linux](#install-on-linux-v2), [Install on
macOS](#install-on-macos-v2)).

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
  from another ref's, or only by Unicode normalisation (the next item). Any component of the path
  counts, so all three shapes above are covered. The refusal names every conflicting ref, and git
  reports that ref rejected. A create is a name the
  remote does not hold yet: a name it already holds is never refused by this rule when the push
  updates it (forced or not) or makes an ordinary delete of it. The one delete this check refuses
  is the delete half of a case-changing rename beside a surviving variant (delete `Main`, create
  `MAIN`, with `main` present): when it refuses the create, it refuses the delete too, so `Main`
  survives. A rename that changes only the normalisation is treated the same way. A create refused
  for another reason (for example a ref and a folder of refs under the same name, or a name a
  skipped foreign file occupies) gets git's ordinary per-ref treatment: its paired delete still
  goes through, and the refused create's commit stays in your local repo.
  There is no override. Case is folded by Unicode simple case folding, from tables this repository
  generates from Unicode 17.0.0's own data files, so every build refuses the same names whichever
  Go release compiled it. NTFS folds with its own table, which probably differs for a handful of
  characters; that difference is not checked.
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
  - **Repair a pair** by deleting one member from Linux or a `reftable` clone (`git push
    proton-v2 --delete Main`). Do not use a Windows `files` clone for a letter-case pair: NTFS
    ignores case, so git's cleanup of the deleted name's tracking ref plausibly unlinks the other
    member's loose tracking ref there, as it does on a Mac [inference; not measured on NTFS].
    Windows is a route only for a pair that differs by normalisation alone, which NTFS keeps
    apart [inference]. A Mac `files` clone that holds the other member's tracking ref refuses
    the delete; on a Mac, follow [the repair on macOS](#honest-limits-on-macos-v2). A remote that
    already holds a pair keeps taking pushes to its other refs, but under a directory pair
    (`Feat/a` beside `feat/b`) a new ref under either spelling is refused until the pair is
    repaired.
- **v2: refs that differ only by Unicode normalisation (two spellings of `café`).** An accented
  letter can be stored as one character (NFC, `é`) or as a plain letter followed by a combining
  accent (NFD, `e` and U+0301). The two spellings look identical. Proton stores them as two names,
  and so, as far as is known, do Linux and Windows clones [inference], but APFS, the Mac's
  filesystem, treats them as one file name. git on macOS stores what you type in NFC
  (`core.precomposeunicode`), so a Mac user cannot create the NFD spelling by typing it, but a
  pair can arrive from Linux, Windows, a tool that writes raw bytes, or a CLI or web upload. A
  Mac clone that uses git's default `files` ref store then goes wrong much as a Windows clone does
  with case (measured with local repositories on one Mac, not through the helper):
  - an NFD name given on the command line lands on the NFC branch, and the exact NFD bytes put you
    on the NFD name with the NFC branch's commit and files, both silently;
  - once the refs are packed, the NFC branch is silently moved to the NFD branch's commit;
  - a branch in a folder spelled in NFD, beside the same folder spelled in NFC, is silently
    re-spelled in NFC;
  - a fetch after the remote moves only the NFD branch exits 0 and leaves the NFC tracking ref at
    the NFD branch's commit. `git fsck` then reports the NFC branch's own commit dangling, so
    `git gc` can eventually drop it. When both branches have moved, the fetch fails with a
    misleading `cannot lock ref … Another git process seems to be running`.

  So the create rule above checks a second relation, on every platform: two spellings also
  conflict when they are canonically equivalent once case is ignored. That is Unicode's canonical
  caseless match with simple folding (decompose to NFD, then fold), from the same Unicode 17.0.0
  tables. For each conflicting ref the refusal names the two spellings that differ, which relation
  matched, and both spellings again in escaped form, because they print alike:
  `"heads/caf\u00e9" vs "heads/cafe\u0301"`. A letter-case match is reported as harming Windows
  and Mac clones; a match by normalisation alone, Mac clones.
  - **What this costs.** Canonical caseless equivalence is Unicode's definition, not a model of
    APFS or NTFS, so it refuses some pairs that no filesystem is known to confuse. `ᾳ` beside `αι`
    is refused, because Unicode folds the iota subscript in `ᾳ` to `ι`, though no measurement shows
    a Mac mixing those two up [inference].
- **v2 on Linux: unattended use is not gated.** Only attended use is in scope for the Linux gate
  and supported: a session with an unlocked Secret Service. Cron or systemd-timer runs are not
  covered. The helper
  takes no part in credentials, and the CLI needs an unlocked Secret Service (or `pass`), which a
  headless session does not have unless you arrange it. Without one, the push fails with the
  CLI's own message (`Failed to load session from secrets …`). Unlocking headless always means
  storing a password, or decrypting without a prompt, somewhere; that trade-off is yours, not a
  default shipped here.
- **v2 on Linux and macOS: Proton CLI drift.** The helper accepts one exact CLI build. A newer
  Proton CLI reaches you as a refusal (`GPB_UNCERTIFIED_CLI=1` overrides it, see [Environment
  variables](#environment-variables)). A re-cut of the same version number passes the helper's
  check, which matches the identity token only; catching it is the maintainer's drift check
  (`scripts/Test-ProtonCliDrift.ps1`, which compares Proton's feed with the recorded Linux and
  macOS SHA-512s). It compares the feed with recorded values, so it cannot see the CLI installed on
  any Linux machine or Mac. The `sha512sum -c` and `shasum -a 512 -c` lines in the install commands
  are your own check of the download. On a Mac, no new CLI version is live-gated: each
  certification checks Proton's macOS build offline, signed out and with networking denied, and
  nothing more.
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

### Honest limits on macOS (v2)

macOS is not live-gated yet ([Install on macOS](#install-on-macos-v2)). Where this section says
*measured*, it means local repositories on one Mac with Apple's git 2.54.0, not runs through the
helper against Proton. The checks below exist because a Mac clone mishandles a pair, as the
normalisation item above describes. Letter case counts too, because default APFS also ignores
case. They apply where git keeps each ref as a file, so that APFS can take two names for one.

- **A name that exists only in NFD.** Every check compares two refs. A remote that holds `café`
  only in NFD, pushed from Linux, Windows or a tool, has no pair. A Mac `files` clone of it works,
  and so does the first fetch after the branch moves. That fetch writes the tracking ref as a loose
  file beside its packed entry, which keeps the old commit. Under git's Mac default,
  `core.precomposeunicode=true`, git reads the file's name as NFC, so it then lists two tracking
  refs: the NFD one at the old commit, and an NFC `café` at the new commit that no file is named.
  After the branch's next move, plain git's fetch fails with `cannot lock ref` (measured
  2026-10-28, git 2.54.0, local repositories; record in
  [docs/research/probes/macos-port/lone-nfd-default/](docs/research/probes/macos-port/lone-nfd-default/)).
  The helper refuses that fetch first, at `list`, with the collision refusal below; a hermetic
  test pins this (`TestLoop_ListAtTheMacDefaultRefusesALoneNFDBranchAfterItsFirstFetch`). The
  remedy is a `--ref-format=reftable` clone. The in-place repair below was not tested on this
  layout. On a Mac, typing the name gives the NFC spelling, which fails with `fatal: invalid
  reference` (measured). That is loud and corrupts nothing. To reach the ref, give git the name as
  bytes with precomposition off, for example `git -c core.precomposeunicode=false switch
  "$(printf 'cafe\314\201')"`.
- **Pairs already on a remote.** A pair can be there from pushes made before this release or by an
  older helper, from a CLI or web upload, or from a repaired skipped file. The create rule cannot
  stop those, and a Mac fetch of one corrupts tracking refs silently (above). So reads are checked
  too, with both relations and the same comparison of path components.
  - **On every platform, a warning.** When the remote's refs hold a pair, `git fetch`,
    `git clone`, `git ls-remote` and `git push` print `git-remote-proton: warning: this remote
    holds refs that differ only by letter case or Unicode normalisation; see README, Honest
    limits:`, then one line per pair with both names, their object IDs, their escaped spellings
    and the relation. Off a Mac it blocks nothing. When the helper refuses the whole session
    before git sees the ref list (either fetch refusal below, the push refusal for up-to-date
    refs, or a local read that fails at that point), the refusal is printed and the warning is
    not. Refusals of single refs in a push, and a local read that fails once the push itself has
    begun, come after the warning has printed.
- **What a Mac `files` repository refuses, and what to do about it.** On a Mac, inside a
  repository whose ref store is `files` (git's default; `git rev-parse --show-ref-format` prints
  which one a repository uses), the helper refuses what APFS would confuse. A `reftable`
  repository gets the warning only.
  - **A fetch, clone or `ls-remote` is refused, and git writes no ref, when the remote holds a
    pair:** `refusing to fetch on this Mac: this repository uses git's default (files) ref store,
    which cannot keep these refs apart (README, Honest limits):`, then the pairs. Repair the remote
    (below), or clone with `--ref-format=reftable`. A clone into the default ref store is refused
    before git writes any ref.
  - **It is also refused when the fetch would write a ref that collides** with a ref this
    repository holds under another spelling, or two refs it would write collide with each other,
    or, with pruning configured (`remote.<name>.prune`, else `fetch.prune`), a ref it could prune
    collides with one that stays: `refusing to fetch on this Mac: refs this fetch would write or
    prune collide with each other or with refs this repository holds (README, Honest limits):`.
    The message ends `use a --ref-format=reftable clone, or change the remote's fetch refspecs; to
    repair a stale local ref in place, follow the README (a plain delete can unlink the other
    spelling's ref)`.
    - **For a stale local ref, re-clone with `--ref-format=reftable`.** To repair the ref in place
      instead, run both of these commands, in this order (measured 2026-10-28, git 2.54.0, on
      APFS in three ref layouts; record in
      [docs/research/probes/macos-port/stale-ref-remedy/](docs/research/probes/macos-port/stale-ref-remedy/));
      the second line removes an NFD tracking ref of `café`:

      ```bash
      git -c core.precomposeunicode=false pack-refs --all
      printf 'delete refs/remotes/proton-v2/cafe\314\201\n' | git update-ref --stdin
      ```

      Both are required. A delete on its own is not safe: while the other spelling's ref is a
      loose file, APFS resolves both names to that one file and git unlinks it, so the other ref
      rewinds to a stale packed value, or disappears, and `git fsck` still exits 0, listing the
      lost commit only as dangling (measured, the record's layouts L6 to L9). Packing every ref
      first leaves no loose file for the delete to unlink [inference], and in each measured
      layout (L10 to L12) only the named ref went. The `-c` is required too: under git's Mac
      default, `core.precomposeunicode=true`, `pack-refs` re-spells the NFD ref in NFC and
      overwrites the NFC ref's value (measured, layout L3). `git update-ref` reads the name from
      its input, so the shell's bytes reach it unchanged.
    - **For two remote refs whose mapped names collide,** change the remote's fetch refspecs, or use
      a `reftable` clone.
  - **A whole push is refused when an up-to-date ref would rewrite a colliding tracking ref.**
    After a push, git rewrites the tracking ref of every ref the push matched by name, even one
    that was already up to date ("Everything up-to-date"), and git does not tell the helper about
    those refs. Take `main`, held both on the remote and in this repository: after any push that
    matches it, git rewrites `refs/remotes/proton-v2/main`. If that tracking ref collides with
    another local ref, such as a stale `refs/remotes/proton-v2/Main` left by an earlier fetch, the
    helper refuses the whole push session: `refusing to push on this Mac: git would rewrite
    tracking refs that collide with refs this repository holds, even for refs that are already up
    to date (README, Honest limits):`, then the collisions, ending `push from a
    --ref-format=reftable clone; to repair the stale local ref in place, follow the README (a
    plain delete can unlink the other spelling's ref)`. While that stale ref stands, no push from
    this clone runs. Push from a `reftable` clone, or repair the stale ref in place with the same
    two commands as above, in the same order, putting the stale ref's own full name, as exact
    bytes, in place of the `café` example: here, `printf 'delete refs/remotes/proton-v2/Main\n' |
    git update-ref --stdin` after the `pack-refs` line. Both are required. For a letter-case pair
    such as this one the route is [inference: the same mechanism; the committed probe covers the
    NFC/NFD pair only].
  - **Single refs in a push are refused, and the rest of the push goes on,** each with a message
    that names this Mac as the reason. A create or update is refused when its tracking ref would
    collide with a local ref or with another pushed ref's tracking ref. An update is also refused
    when the remote keeps a ref its name pairs with, unless the same push deletes that other ref.
    But an update whose tracking ref would share a file with the tracking ref of a ref the same
    push deletes is refused (measured: git removed the tracking ref it had just written, and the
    push still exited 0). A delete is refused when git's cleanup of its tracking ref could unlink
    a local ref under another spelling, which then silently falls back to an old packed value
    (measured). A delete that pairs with a refused write is refused with it.
  - **A local read that fails is a refusal.** Before it answers git, the helper reads the
    repository's ref format, the remote's configuration and the local ref names. If any read
    fails, it refuses and names the read (`cannot check this Mac repository before fetching: …`,
    or `before pushing: …`) rather than skip the check.
  - **Fetch refspecs must use full ref names.** The checks follow the remote's `remote.<name>.fetch`
    refspecs, and model only full `refs/` names on both sides, such as git's default
    `+refs/heads/*:refs/remotes/proton-v2/*`. Any other form, such as `main:main` or `HEAD`, makes
    every fetch and push through that remote refuse, naming the refspec, because git expands such
    forms in ways the helper does not model.
- **What the Mac checks cannot see.**
  - A refspec given on the command line, `--refmap`, and `git fetch --prune` typed on the command
    line never reach the helper. On a Mac `files` repository, fetch through the remote's
    configured refspecs and set pruning in its configuration, or use a `reftable` repository.
    (`git push --prune` is covered: git sends its deletes to the helper like any others.)
  - Another git process on the same Mac that changes refs while a push or fetch runs. The checks
    judge the local refs as they were when the helper read them.
  - Two up-to-date refs whose tracking refs collide with each other. The whole-push check compares
    each up-to-date ref's tracking ref with the other refs this repository holds, never with
    another tracking ref the same push rewrites. So it misses a Mac `files` clone that holds both
    members of a remote pair as local branches under their exact names: `git push --all` then
    rewrites both tracking refs into one APFS file, and the push is not refused [inference,
    untested]. Two up-to-date refs whose custom fetch refspecs map them onto colliding names fall
    in the same gap. The check also misses an up-to-date push to a ref from a source named under
    another spelling and given as exact bytes.
  - A symbolic local ref whose target does not exist. git's ref listings do not show it, so a ref
    written through it is not checked. The common case, a dangling `refs/remotes/<remote>/HEAD`
    after a prune, is still caught where it would collide: the checks count `<remote>/HEAD` as a
    ref a fetch may write, and it collides with a branch named `head`.
  - A remote defined only in a legacy `.git/remotes/<name>` file is checked as an anonymous URL,
    so only the remote's tags count as refs a fetch may write. Define the remote in git's
    configuration instead (`git remote add`).
- **Where the Mac checks refuse more than they need to.**
  - `git ls-remote` inside a Mac `files` repository is refused like a fetch, because nothing git
    sends before asking for the ref list tells the two apart.
  - Every tag the remote advertises counts as a ref a fetch may write, even under `--no-tags` or
    `tagOpt`.
  - A clone with `--single-branch` or `--depth` is checked under git's default mapping, which is
    wider than the one git writes later, so it can be refused for a branch it would not have
    fetched. A bare clone of a remote with a branch named `head` can likewise be refused, through
    the `<remote>/HEAD` the check assumes a clone writes.
  - A repository that holds no refs is checked as a clone is, under its configured refspecs plus
    git's default mapping, because git adds that mapping to a refspec given with `-c` during a
    clone. So an empty repository that is not a clone, with custom refspecs, can be refused for a
    collision those refspecs would not produce. One such case: a repository set up by hand as a
    mirror (`+refs/*:refs/*`, `mirror = true`), fetched while still empty from a remote holding
    branches `main` and `head`, is refused, because the default mapping's `HEAD` collides with
    `head`, though the remote holds no pair.
- **Before a Mac adopts a remote: writers first, then one scan.** An older helper can still write
  a pair, so first make sure every machine that pushes to the remote runs a release with these
  rules. Then scan the remote once with the new helper, from a folder with no repository above it:
  `cd "$(mktemp -d)" && git ls-remote proton::/my-files/GitRemotes/myrepo`. A pair shows up as the
  warning above, and anything the helper cannot read stops the scan with an error. Inside any
  repository's tree git tells the helper which repository it is in, and in a `files` repository
  the scan becomes a fetch check against that repository's refs.
- **Repairing a pair, on a Mac.** Delete one member from Linux or a `reftable` clone, or from
  Windows only when the pair differs by normalisation alone [inference] (see the letter-case item
  above for why); a Mac `files` clone that holds the other member's tracking ref refuses the delete.
  Deleting from such a clone is what the helper prevents: git's cleanup of the deleted name's
  tracking ref can unlink the surviving member's file, which APFS takes for the same name, and
  rewind that ref to an old packed value with `git fsck` reporting nothing (measured). The repair,
  in order:
  1. Record both members' names and object IDs; the warning prints them.
  2. Make a temporary clone with `--ref-format=reftable`, which the refusal allows and whose refs
     APFS cannot merge.
  3. In it, delete one member by its exact bytes, because a typed name is stored in NFC: run
     `git config core.precomposeunicode false`, then for example `git push proton-v2
     ":$(printf 'refs/heads/cafe\314\201')"`. Or delete it from a Linux client, or, for a pair
     that differs by normalisation alone, a Windows one [inference].
  4. Then, on every Mac clone of that remote, recover what only that clone holds before `git gc`
     can drop it: any commit `git fsck` reports dangling, and any branch you never pushed. Then
     re-clone it, with `--ref-format=reftable` or without. The re-clone is not optional: the
     rewind above leaves `git fsck` clean, so a clean `fsck` proves nothing about a Mac clone's
     tracking refs.

  If a Mac `files` clone pushes the delete of one member together with an update of the other, the
  update is refused, because its tracking ref would share a file with the deleted one's. The
  delete goes through unless it is refused as above, so push the update again once the delete is
  done. But a clone that pushes an update of one member usually holds that member's tracking ref,
  and then the delete is refused as above too and both halves fail; the push refusal for
  up-to-date refs can also refuse the whole session before either is judged. That is why the
  repair runs from a `reftable` clone.

## Documentation

The docs index at [docs/README.md](docs/README.md) maps everything: the two designs, the release
procedure, the research evidence (gates and probes), and the dated spec/plan pairs feature work
uses.

## Support

Built for my own use and shared as-is. Issues and PRs are welcome; no support or response time is
promised.

## Roadmap

- **macOS** — `git-remote-proton` (v2) builds from source on Apple Silicon, and CI runs the
  hermetic suite on an Apple Silicon runner. It is not yet live-gated (see [Install on
  macOS](#install-on-macos-v2)). Ruled out for this port: a macOS release asset and Intel Macs;
  v1 stays Windows-only.
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
