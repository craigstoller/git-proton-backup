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
- **Linux** — either via CLI-direct upload (bypassing the sync-folder transport entirely) or once
  Proton ships a Linux sync client.
- **PowerShell Gallery** — the manifest is Gallery-ready; publishing is a fast-follow, not a v1
  blocker.

## Disclaimer

This project is not affiliated with or endorsed by Proton AG. "Proton Drive" is a trademark of
Proton AG.
