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

**Requirements:** Windows, PowerShell 7.4+, git, the Proton Drive desktop app, and the
[Proton Drive CLI](https://proton.me/drive) — optional but recommended. Without the CLI,
verification degrades to Windows Cloud Files sync-state only (see
[Honest limits](#honest-limits)); it never blocks install.

```powershell
git clone https://github.com/craigstoller/git-proton-backup.git
cd git-proton-backup
.\install.ps1                    # copies the module into your $PSModulePath
Import-Module GitProtonBackup
Initialize-ProtonBackup          # guided first run: finds the Proton Drive sync folder,
                                  # resolves the CLI, probes auth, writes config.json
```

Then wire a repo:

```powershell
Install-ProtonBackup C:\code\myrepo
```

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
| Transport | Proton Drive's Windows sync app; a single bundle file lands in the synced folder | The Proton Drive CLI, invoked directly by the helper as a real git remote |
| Remote name | `proton` | `proton-v2` |
| Restore needs | git only, from any machine — the bundle itself is the backup, no account needed | git, `git-remote-proton` on PATH, and the certified Proton Drive CLI signed in |

```
git clone -o proton-v2 proton::/my-files/GitRemotes/myrepo
git push proton-v2 main
```

Wiring both onto one repo is safe: install v1 as usual (`Install-ProtonBackup`), then add the
`proton-v2` remote alongside it. Full design: [docs/v2-remote-helper-design.md](docs/v2-remote-helper-design.md).

**Keep v2 remotes off `GitBackups`.** Point v2 at its own root — `/my-files/GitRemotes/` is the
convention used above — and never inside `GitBackups`, the folder v1's bundles live in.
Initializing a v2 repo hard-refuses a non-empty folder that has no v2 marker, so pointing it at
your populated `GitBackups` root fails safely — but an *empty* subfolder would be silently
adopted as a v2 repo, so don't create one there even as a placeholder.

**Restore contracts, stated honestly.** A v1 bundle restores with nothing but git —
`git clone <bundle-path>` — from any machine, any OS, no account needed at the moment of
restore. A v2 restore needs three things to be true at once: git, `git-remote-proton` installed
and on PATH, and the certified Proton Drive CLI signed in. That is a real dependency v1 doesn't
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

## Why PowerShell?

Because the domain is Windows, and PowerShell is what Windows already speaks.

- **The transport is the Proton Drive *Windows* sync app** — bundles are published into its sync
  folder, and upload state is read straight from the Windows Cloud Files API. That API call is a
  few lines of inline P/Invoke here; it isn't portable, and neither is the transport.
- **Zero runtime for the audience.** The people this is for — Windows users with git and a Proton
  account — install nothing to run it: no Python, no packaging manager, no binary to trust.
  `install.ps1` copies a module; that's the whole footprint.
- **The job is glue.** Orchestrating git, a vendor CLI, filesystem state, and Task Scheduler is
  exactly what a shell is for. A compiled language would add build weight to a tool whose honest
  job is coordination, not computation.

The Windows-specific part is the transport, not the language — see the [Roadmap](#roadmap) for
what macOS/Linux support would actually take.

## Monitoring

- `Get-ProtonBackupStatus` (add `-Json` for scripting) — per-repo wiring health, whether the current
  commit state is bundled, confirmation from the last verify run, and any pending marker.
- `Invoke-ProtonBackupVerify` — run it ad hoc, or install `Install-ProtonBackupTask` for a daily
  scheduled check (interactive logon, since both the CLI session and the sync app live in your
  desktop session).
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
  only — never fatal, but a weaker guarantee than CLI confirmation.
- **Proton Drive sync stopped:** bundles pile up locally until sync resumes; `Invoke-ProtonBackupVerify`
  surfaces the backlog once it's older than `MaxUnconfirmedAgeDays` (default 7).
- **One machine per repo:** there's no multi-machine coordination. A second machine pushing the same
  repo is safe but confusing — digest-suffixed filenames prevent corruption, not confusion.
- **Worktrees:** a bundle carries full ref history — every branch and tag — but not the checked-out
  working-tree state of any secondary `git worktree` attached to the repo. History is complete;
  restoring a linked worktree's own working copy afterward is a manual `git worktree add` from that
  history, not something the bundle reproduces automatically.

> **What this tool does and doesn't encrypt:** git-proton-backup performs no encryption itself.
> Bundles are ordinary git bundles; anyone with access to your Windows account or your Proton
> account can read them. End-to-end encryption is provided by Proton Drive for transport and cloud
> storage.

## Support

Built for my own use and shared as-is. Issues and PRs are welcome; no support or response time is
promised.

## Roadmap

- **macOS** — if there's demonstrated interest.
- **Linux** — `git-remote-proton` (v2) already does exactly this: CLI-direct upload, bypassing
  the sync-folder transport entirely. It's windows/amd64 only for now — the only platform its
  gate has run on — so Linux support means porting v2's build, not a new mechanism. For v1
  specifically, the alternative is waiting for Proton to ship a Linux sync client.
- **PowerShell Gallery** — the manifest is Gallery-ready; publishing is a fast-follow, not a v1
  blocker.

## Disclaimer

This project is not affiliated with or endorsed by Proton AG. "Proton Drive" is a trademark of
Proton AG.
