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
[Honest limits](#honest-limits)); it never blocks install. Proton's CLI download page hosts only
the current release, so a fresh install already gets the build `git-remote-proton` (v2, below)
certifies against — currently 0.8.0 (`cli-drive@0.8.0+06e8c605`). v1's own CLI use has no version
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
| Restore needs | git only, from any machine — the bundle itself is the backup, no account needed | git, `git-remote-proton` on PATH, and the certified Proton Drive CLI (0.8.0) signed in |

**Install v2 first — `git-remote-proton` doesn't land on PATH by default.** Download the release
assets (`git-remote-proton.exe`, `git-remote-proton.exe.sha256`, `install.ps1`) from a [GitHub
Release](https://github.com/craigstoller/git-proton-backup/releases) into one folder and run
`install.ps1` there: it verifies the exe against the `.sha256`, copies it to
`%LOCALAPPDATA%\Programs\git-proton-backup`, and adds that folder to your user PATH — open a
**new** terminal afterward, since the script can't change its caller's own session. Building from
source works too: `go build ./cmd/git-remote-proton` and put the resulting exe on PATH yourself.

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
restore. A v2 restore needs three things to be true at once: git, `git-remote-proton` installed
and on PATH, and the certified Proton Drive CLI — currently 0.8.0 — signed in. That is a real dependency v1 doesn't
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

### Environment variables

`git-remote-proton` (v2) reads two opt-in environment variables. Both are read fresh from the
environment on every invocation — never cached, never remembered across runs — so setting or
unsetting one takes effect on the very next command.

- **`GPB_UNCERTIFIED_CLI=1`** overrides the certified-CLI allowlist. By default the helper refuses
  to run against anything but the exact certified Proton Drive CLI build — currently
  `cli-drive@0.8.0+06e8c605` — naming what it found versus what's certified. Setting this proceeds
  anyway, printing a loud stderr warning naming the untested (or undetermined) CLI version on every
  invocation. **The override does not make an older CLI viable for pushes that update refs:** if
  the detected build is specifically the previously-certified 0.7.0, the warning names that
  incompatibility directly — 0.8.0 replaced the general upload conflict-strategy option with
  separate file- and folder-scoped ones, and 0.7.0's update path has no way to speak the new
  file-side vocabulary. Meant for troubleshooting a CLI upgrade, not for routine use.
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

The Windows-specific part is the transport, not the language — see the [Roadmap](#roadmap) for
what macOS/Linux support would actually take.

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
