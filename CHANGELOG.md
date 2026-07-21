# Changelog

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

Initial public release.
