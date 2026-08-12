# Changelog

## Unreleased

- **GitProtonBackup (PowerShell module): `Invoke-ProtonBackupVerify` now gives freshly cut
  bundles a bounded upload-lag grace before reporting them unconfirmed** — so the first run after
  machine downtime no longer takes its verdict the instant the bundles hit disk. After any off
  period every registered repo's digest is stale at once, so that run re-cuts a bundle for every
  repo and then asked Proton about files only seconds old — before the Proton Drive sync app
  could have uploaded them — reporting "newest bundle not confirmed on Proton" fleet-wide
  (observed 2026-08-12: 15/16 repos flagged, all clean on a re-run ten minutes later). The grace
  is one shared window of `VerifySeconds` (the same knob the push hook polls under) for the whole
  fleet, swept in rounds, each repo dropping out as it confirms — total added wall-clock stays
  near one window regardless of fleet size, and a run that cut nothing (or whose cuts confirm on
  the spot) never waits at all. Eligible bundles are the freshly cut ones — by the run itself, or
  moments before it (a push hook firing just ahead of the schedule); an upload that genuinely
  outlives the window still alarms once and clears on the next run. New
  `-GraceSeconds`/`-GracePollSeconds` parameters override the default (`-GraceSeconds 0`
  disables). Bundles already older than the window keep failing fast — they're stuck, not
  lagging — with `MaxUnconfirmedAgeDays` as the escalation path, unchanged. One correctness
  tightening surfaced by review: a push-pending marker written *after* the run surveyed its repo
  (a push deferring while verify holds the lock) is no longer cleared by that run's confirmation —
  it stays pending until a later run actually confirms the coverage it tracks.

## 0.5.0 — 2026-08-09

- **git-remote-proton: `push` no longer fails outright because of a junk file under `refs/`.**
  This is the headline behaviour change, and it is **breaking-ish in one specific way — read it
  before upgrading if you rely on push failing as your junk-file alarm.** Previously, a ref file
  that was well-named but whose contents didn't parse as a ref (a stray web-UI upload, a damaged
  file, foreign junk) was **uniformly fatal**: both `list` and `list for-push` walked the same ref
  tree, and a single such file aborted the whole advertisement — so an unattended cron `push`
  stopped dead on a file it never created, with no way to skip past it but deleting the file by
  hand. `push` now **tolerates** it: the file is skipped with a loud, classified note, the push
  proceeds, and pushing (or deleting) onto that exact name is still refused, actionably, by the new
  occupancy machinery below. **The trade:** a junk file that used to stop backups loudly no longer
  stops them — anyone who was relying on that fatal failure as a "something's wrong" signal loses
  it and should watch the new skip notes instead. `fetch`/`clone`/`ls-remote` **still fail by
  design** on the same class of file — that direction was never silently succeeding, it already
  failed loudly before this change — but the failure is now more useful: one error enumerating
  every content-skipped path, its classified reason, and the remedy, instead of the previous single
  generic error naming just the first file the walk happened to reach. Files with **invalid names**
  (never valid refs to begin with) stay note-only in both directions, unchanged. See the README's
  new foreign-data paragraph and `docs/v2-remote-helper-design.md`'s "Foreign data at the read
  boundary" section (v6.6) for the full rule and remedy.
- **git-remote-proton:** pushing (or deleting) onto a name a foreign file or folder occupies is now
  refused **before any pack is built**, with a message naming exactly what occupies the path and
  how to clear it — instead of failing later, more expensively, and with generic wording. The
  message is kind-aware: a file whose contents aren't a ref names the reason; a file with an
  invalid name says its contents were never examined; a folder with an invalid name says the same
  and tells you to inspect before deleting anything, since blind deletion could destroy a foreign
  subtree the helper never looked inside. A delete of such a name is refused the same way and
  never trashed. Separately: because such a name was never advertised, `git push --delete` of it
  is refused by git itself ("remote ref does not exist") before it ever reaches the helper — the
  CLI or the Proton Drive web UI is the only way to clear it (Proton trash keeps the deletion
  restorable).
- **git-remote-proton:** a damaged ref file with a malformed terminator (no trailing newline, a
  CRLF terminator, or a doubled newline around an otherwise-valid 40-hex sha) now has its
  recoverable hex quoted in the skip note or refusal message, instead of a generic preview — so
  the object pointer isn't lost from the log even though the file itself is skipped.
- **git-remote-proton:** if a foreign file appears at a ref's name in the narrow window between
  push's advertisement and its own create attempt, the create failure now names what's actually
  there (contents-not-a-ref, with the reason) instead of the generic "ref changed concurrently;
  refusing to overwrite" — unless it's a genuine concurrent ref creation, which still reports as
  that.
- **git-remote-proton (internal):** `filesystem download` on a directory is now a pinned contract
  fact (exit 0, recursive download of the whole subtree) rather than an unprobed assumption — no
  user-visible change, but it makes `--set-head`'s directory-vs-branch guard regression-safe.

## 0.4.0 — 2026-08-07

- **git-remote-proton:** hierarchical refs are fully supported — nested branch and tag names
  (`refs/heads/feature/x`), plus `refs/notes/*`, `refs/replace/*`, and every other valid
  namespace beyond `refs/heads/` and `refs/tags/`, are now advertised, fetchable, and pushable.
- **git-remote-proton:** deleting a ref now prunes the empty parent folders it leaves behind, and
  creating a ref that collides with such a leftover folder self-heals (trash the folder, retry)
  instead of refusing permanently.
- **git-remote-proton:** `--set-head` now accepts hierarchical branch names
  (`git-remote-proton --set-head <url> feature/x`) — Stage 4 refused any name containing a slash,
  and that refusal is lifted. Pointing it at a namespace *folder* (`feature`, when only
  `feature/x` exists) is refused with a message that says so and suggests the branches that
  actually exist, instead of a misleading "no such branch".
- **git-remote-proton:** the ref-file grammar is enforced exactly — a ref file's contents must be
  40 lowercase hex characters plus a single `\n`, and nothing else. Variants that were previously
  tolerated (no trailing newline, a CRLF terminator, a doubled newline) are now a hard error
  naming the remote path. This can only affect a foreign or damaged file: the helper has always
  written exactly that grammar itself.
- **git-remote-proton:** `GPB_CREATE_PARENTS=1` opts a push into creating missing parent folders
  above the repo root; unset (the default) stays an actionable refusal naming the exact
  `proton-drive filesystem create-folder` command to run by hand — see the README for the trade
  this makes.
- **git-remote-proton:** marker and "not a git-remote-proton repo" diagnostics now distinguish a
  confirmed absence from a failed read, so a broken CLI (e.g. under `GPB_UNCERTIFIED_CLI=1`) no
  longer masquerades as an empty remote.
- **git-remote-proton:** on Windows, a failure involving a near-limit path now gets a best-effort
  hint naming the legacy `MAX_PATH` remedies (`core.longpaths`, a shorter destination) — see the
  README's Windows path length section.
- **git-remote-proton:** fetch downloads now land and verify in a per-fetch quarantine directory
  before publishing into the local object store, replacing the previous failed-attempt
  residue-deletion rule (internal; no user-visible behaviour change).

## 0.3.1 — 2026-08-06

One version line covers both tools in this repository: the GitProtonBackup
PowerShell module (v1, bundles) and the git-remote-proton helper (v2, CAS
remote). This is the first release to ship the helper as a built artifact.

- **install.ps1** now detects when another `git-remote-proton.exe` earlier on
  PATH shadows the installed helper and warns loudly naming both paths
  (found by Stage 4 gate run 1, which blocked on a silent shadow).
- **git-remote-proton:** certified-CLI allowlist enforced (exact build match;
  `GPB_UNCERTIFIED_CLI=1` overrides with a loud warning).
- **git-remote-proton:** `--set-head` changes a remote's default branch
  in-tool; the branch-delete refusal now names it as the remedy. `--version`
  prints the helper version and the certified CLI build.
- **install.ps1:** also installs the helper exe (checksum-verified when a
  `.sha256` sidecar is present) and adds it to the user PATH.
- Releases are published as drafts and made public only after the live gate
  passes against the exact built bytes.

## 0.3.0 — never released

Tagged, draft built, gate-blocked on the installer shadow defect above;
superseded by 0.3.1.

## 0.2.4 — 2026-08-01

- **Compatibility fix for Proton Drive CLI 0.7.0.** The CLI dropped the `{ok, value}` Result
  wrapper around `activeRevision` in `filesystem info --json`: 0.4.6 returns
  `activeRevision.value.state`, 0.7.0 returns `activeRevision.state`. Reading only the wrapped
  form meant **every healthy bundle reported as unconfirmed on 0.7.0** — a silent fleet-wide
  fail-closed. Both shapes are now accepted, and `ok: false` in the wrapped form is still
  honoured as "no usable revision".

- **Fixed a false `auth_error` that made the above much worse.** Auth detection matched the bare
  substring `auth`, and the success payload contains `keyAuthor`, `nameAuthor` and
  `contentAuthor`. So a healthy bundle on a perfectly valid session reported an authentication
  failure, sending the user to debug a login problem that did not exist. Auth detection is now
  gated on a **non-zero exit code** and uses word-boundary patterns.

  Found by upgrading the CLI to 0.7.0 and re-running the verification path against it, which is
  the whole argument for pinning an exact CLI version rather than a minimum.

## 0.2.3 — 2026-07-29

- **Fail-closed fix in Cloud Files sync-state decoding.** `CF_PLACEHOLDER_STATE_INVALID` is
  `0xffffffff`, returned when Windows cannot parse a file's attributes and reparse tag. It
  arrives through the int-returning P/Invoke as `-1` (every bit set), and
  `ConvertFrom-CfPlaceholderState` was masking bits straight off it — so an unreadable state
  reported both `IsPlaceholder` and `InSync` as **true**. Because `InSync` alone drives
  `backed_up` state, push-marker clearing, and retention pruning whenever the Proton CLI is
  unavailable, a file whose state could not be determined read as "safely in sync" and could
  have permitted a bundle to be pruned. All negative values now fail closed; unknown positive
  bits are still ignored by the masks, so a future Windows state cannot suppress a real
  `IN_SYNC`. Six tests added around the decoder, which previously had none: the two
  negative-state cases were confirmed failing against the old code before the fix; the other
  four pin the valid-state decoding the fix had to preserve.

  Found because a commenter on r/PowerShell asked how the DLL's functions were discovered,
  which sent us back to re-read the P/Invoke.

## 0.2.2 — 2026-07-25

- Upload confirmation now reads the CLI's structured output (`filesystem info --json` →
  `activeRevision.value.state`) instead of pattern-matching the human-readable text for
  `state: 'active'`. The old match is kept as a fallback, so a CLI whose JSON is absent or
  unparseable degrades to the previous behavior rather than reporting a healthy bundle as
  unconfirmed. Closes #4.

## 0.2.1 — 2026-07-21

- `Invoke-ProtonBackupVerify` can no longer go silent on an unexpected throw: the verify pass
  now has a catch-all that converts an escaping error into an incomplete run
  (`Complete = $false`, new `IncompleteReason` value `'error'`) and still writes
  `last-verify.json` and pings the heartbeat. Previously such a throw would skip both —
  leaving the dead-man's switch silently stale. ([#2])
- `Uninstall-ProtonBackup` no longer removes a `proton` remote it doesn't own: the remote is
  removed only when it points into this tool's mirrors root; a foreign remote gets a warning
  and is left in place (any GitProtonBackup mirror for the repo is still cleaned up).
  Ownership-symmetric with install, which refuses foreign remotes. ([#2])

[#2]: https://github.com/craigstoller/git-proton-backup/issues/2

## 0.2.0 — 2026-07-21

- `Invoke-ProtonBackupVerify` now returns `Repos` (the per-repo results already written to
  `last-verify.json`), `Complete` (whether the registered-repo pass actually ran), and
  `IncompleteReason` (`''` | `'lock'` | `'config'`). `last-verify.json` carries the same two
  new fields. Additive — existing fields and exit codes are unchanged. Lets callers
  distinguish "verified, one repo needs attention" from "verified nothing" (lock contention /
  config failure).

## 0.1.0 — 2026-07-20

- `Initialize-ProtonBackup` — guided first-run setup: discovers the Proton Drive sync folder,
  resolves the CLI, probes auth, writes `config.json`.
- `Install-ProtonBackup` (`-SetUpstream`, `-Force`) — wires a repo to a disposable local mirror and
  a `proton` remote; idempotent (re-run to repair).
- `Uninstall-ProtonBackup` — removes the remote, hook, and mirror for a repo; leaves existing
  bundles on Proton Drive untouched.
- `Repair-ProtonBackup` — re-runs install wiring for a moved repo, a deleted mirror, or a module
  upgrade.
- `Get-ProtonBackupStatus` (`-Json`) — per-repo wiring health, whether the current commit state is
  bundled, pending markers, and last-verify freshness.
- `Invoke-ProtonBackupVerify` — reconciliation backstop: re-cuts a stale bundle even without a
  pending marker, prunes retention, and pings an optional heartbeat URL.
- `Install-ProtonBackupTask` — registers a daily scheduled task that runs verify.
- `Get-ProtonBackupConfig` — reads the current configuration.
- `Set-ProtonBackupConfig` — validates and writes a single configuration key.

Note: a pre-release fix corrected the push-hook shim's `VerifySeconds` fallback (it was hardcoding
60s instead of honoring `config.json`'s `VerifySeconds` when no per-mirror `gpb.verifyseconds`
override is set) — any mirror wired before this fix picks it up by re-running
`Repair-ProtonBackup`.

Initial public release.
