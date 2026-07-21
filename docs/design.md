# git-proton-backup — Design

This document is the "why," not the "how" — for commands and usage, see [../README.md](../README.md).

## Why bundles

A Proton Drive sync folder is a great place to keep a single file current across devices. It is a
bad place to keep a *live git repository*: a repo is thousands of small loose objects, and a
general-purpose file-sync client watching a directory for changes has no idea that a burst of new
object files and a moved `HEAD` are one atomic unit. Sync it mid-write and a restore can see a
git directory that never existed as a coherent state on any machine.

A git bundle sidesteps the whole problem by being a *single file*. `git bundle create` packs the
requested refs into one self-contained blob; from the sync client's point of view there is no
"mid-write" to observe — either the file exists at a given upload, or it doesn't. git-proton-backup
never asks Proton Drive to sync a working repository. It only ever asks it to sync bundles.

## Fail-closed publication

A backup tool that can silently stop backing things up is worse than no backup tool, because it
looks like it's working. Every publish step is built to fail loudly instead of quietly:

- Native exit codes from `git bundle create` and `git bundle verify` are checked. A failure never
  advances anything — no bundle is considered current, and (from the push hook) a pending marker is
  written.
- Publication is atomic: the bundle is written to a temp file in the *same directory* as its final
  name, verified, and only then renamed into place. The "this state is now covered" record (a
  digest stamp) is written only after that rename succeeds — never before.
- Every bundle filename carries a fragment of its content digest:
  `<repo-name>-<timestamp>-<digest8>.bundle`. This closes a same-second race: without a unique
  name, a slow upload could let the verifier confirm a *stale* file sitting at a path a brand-new
  bundle was about to reuse, falsely blessing content that was never actually uploaded.
- A "nothing changed, skip the work" cache hit requires *both* the stamped digest matching the
  current one *and* the newest bundle file on disk actually carrying that digest in its name. If the
  file is missing — deleted, moved, whatever — that's treated as a miss and a fresh bundle gets cut.
  Coverage is never claimed on the strength of a stamp alone.

## The verification outcome table

After a push, the hook polls Proton Drive for up to `VerifySeconds` (default 60) before giving up:

| Condition | Push output | Pending marker |
|---|---|---|
| Proton Drive CLI confirms the bundle is uploaded | `confirmed on Proton` | cleared |
| CLI unavailable, Windows Cloud Files reports in-sync | `staged; in-sync per Cloud Files (CLI verification unavailable)` | cleared |
| CLI available, not confirmed within the window | `staged, not yet confirmed — run Invoke-ProtonBackupVerify (or the scheduled task) to confirm` | kept |
| CLI session expired / auth error | same output, naming the expired session | kept |
| CLI unavailable and Cloud Files not in-sync | same output, naming the unavailable CLI | kept |

`confirmed on Proton` is reserved for the strong case — actual CLI confirmation that the bytes are
on Proton's servers. The Cloud Files fallback is real signal, but it's a weaker guarantee (a local
placeholder-state flag, not a server response), so the wording never blurs the two together.

## Markers + the reconciliation backstop

A pending marker is written *before* any bundling work starts — pessimistically, so a killed
terminal or a hook crash mid-bundle can never lose the backstop. It's kept until the push flow
confirms the upload (marker deleted) or gives up within the verify window (marker kept, with a
reason: still polling, auth expired, CLI unavailable, lock briefly unavailable, or preflight
declined).

But markers alone have a gap: if the hook never runs at all — a deleted mirror, `pwsh` missing from
PATH, a hook script that's silently broken — no marker is ever written, because nothing ran to write
one. A design that only ever cleared or reported on markers would let that failure mode go
completely unnoticed, possibly forever.

`Invoke-ProtonBackupVerify` closes that gap by not trusting markers as its source of truth at all.
Every run, for every registered repo, it independently recomputes the same canonical digest
(heads + tags) the push flow uses and compares it against what's actually stamped and bundled on
disk. If they don't match — or the newest bundle file doesn't carry the current digest — Verify
cuts and publishes a fresh bundle right there, with no marker required to trigger it. That's the
real reason Verify re-cuts even when nothing is pending: a marker only tells you a push is *in
flight*; the digest comparison is what tells you the repo's actual coverage has gone stale, which is
the thing that matters. Markers still carry useful timing detail (how long something's been
pending, and why) — they're just not what decides whether a repo needs a new bundle.

Verify also prunes old bundles under retention once the newest one is confirmed, watches for
bundles that have gone unconfirmed for too long (`MaxUnconfirmedAgeDays`, default 7 — a sign the
sync app itself may be stopped), quarantines a marker file it can't parse (renamed `.bad`, never
deleted — evidence survives for inspection), and evicts markers left behind by repos that are
neither registered nor present on disk anymore. Every run — clean, partial, or a hard configuration
failure — writes a durable report (`last-verify.json`) and, if configured, pings a heartbeat URL, so
even a scheduled task that's stopped running entirely is something `Get-ProtonBackupStatus` can
surface (via that report's age) rather than something that just goes quiet.

## Locking

There is one lock file, and the two callers hold it differently. The push hook takes it
non-blocking, with a short bounded retry (about 15 seconds), around the bundle step only — never
across the push in full, and never across the network verification/polling step that follows (that
step is deliberately lock-free, so a slow or hung Proton CLI call can never make it look like
another backup operation is "active"); on timeout it records a `deferred_lock` marker and lets the
push finish anyway (the marker guarantees the gap still surfaces later). `Invoke-ProtonBackupVerify`
is the opposite: it takes the same lock for its *entire* per-repo pass — including the network
confirmation check against the Proton CLI — with a longer bound (about 30 seconds), so a hook's
brief hold can't make a scheduled verify run skip its work outright; it waits the hold out instead
of bailing, and holds it through confirmation because a reconciliation pass that let another run
start mid-check could double-cut or double-confirm the same bundle.

## Threat model & limits

- **No encryption performed by this tool.** Bundles are plain git data; end-to-end encryption is
  Proton Drive's job, covering transport and cloud storage, not the bundle's contents. Anyone with
  access to your Windows account, or your Proton account, can read them. (See the README's
  crypto-honesty paragraph.)
- **One machine per repo.** There is no coordination protocol for two machines pushing the same repo
  to the same backup location. Digest-suffixed filenames make that scenario safe-but-confusing —
  never corrupting — but it isn't a supported topology.
- **Coverage is push-triggered plus scheduled reconciliation, not continuous.** Uncommitted changes
  are never backed up; this tool never auto-commits anything, and a push that leaves the working
  tree dirty prints a note naming how many files were left out.
- **Restore depends on nothing but git.** `git bundle verify` / `git clone` / `git fetch` work
  whether or not GitProtonBackup is installed, working, or even still exists — the tool is not a
  moving part of your restore path.
- **LFS objects, submodule repos, and shallow-clone bundling are out of scope.** See the README's
  Honest limits section for what that means in practice.

## Origin

git-proton-backup is hard-forked from the author's private tooling; design twice adversarially
peer-reviewed before extraction.
