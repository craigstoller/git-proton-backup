# Changelog

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
