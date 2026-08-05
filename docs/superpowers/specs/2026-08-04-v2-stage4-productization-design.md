# git-remote-proton Stage 4 — productization

**Date:** 2026-08-04. Brainstormed with Craig at Stage 4 open, immediately after the C17b
provocation probe (commit `beb23b3`) returned NOT REPRODUCED — that result shaped this scope.
Baseline: Stage 3b merged (`349ce54`), selective fetch live-gated with measured selectivity.
Design doc at v6.3 (`docs/v2-remote-helper-design.md`).

## Goal

v2 becomes daily-usable. Five deliverables: (1) certified-CLI enforcement with an explicit
escape hatch, (2) a `--set-head` operation closing the only user-facing feature gap (a remote's
default branch cannot be changed in-tool today), (3) a tag-triggered CI release pipeline with a
real install story, (4) v1-coexistence documentation, (5) the deferred test/hygiene debt paid.

No fetch-path behaviour changes — the hygiene wave touches error wrapping, error ordering and
helper-function dedup there, nothing that alters what is downloaded or when. Success is the
Stage 4 live gate passing end to end.

## Decisions taken here

- **Stage framing.** Productization now; hierarchical refs get their own stage (Stage 5), and
  the retro-Codex quarantine-staging refactor rides that stage's fetch-path touch, not this one.
- **EnsureDir already-exists tolerance is DROPPED**, not deferred. C17b could not provoke the
  gate-run-1 failure in 30 trials across three topologies (probe log
  `docs/research/probes/c17b-provocation-log.md`; C17 amended to observed-but-unprovoked), so a
  tolerance could not be validated live and its confirm step's semantics would be guesswork. The
  failure stands at n=1 unexplained; the mitigation is operational (trash hygiene, documented in
  README and the gate procedure) and the item returns only on an n=2 in the wild.
- **The helper keeps the name `git-remote-proton`.** v1 ships no binary at all (a PowerShell
  module; its `proton` remote is a local mirror path, no helper scheme), so there is no
  collision; the name is derived from the `proton::` scheme and baked into the marker's
  exact-match `format` seam. Coexistence is a docs problem.
- **Allowlist policy: refuse-by-default with a loud explicit override** (Craig's pick over pure
  no-override refusal). Certified list starts as exactly `cli-drive@0.7.0+5174900c` — the build
  every probe and gate ran. The version-tolerant `parseNodeJSON` (0.4.6 + 0.7.0 shapes) is KEPT:
  the allowlist gates the front door; parser tolerance is defense in depth behind it. This
  resolves the design/code contradiction open since Stage 2 in favour of front-door enforcement,
  and the design doc is updated to say so.
- **`--set-head` ships as a direct-invocation utility mode**, not a protocol extension — remote
  helpers have no protocol verb for it, and the user (not git) is the natural invoker.
- **Utility-mode stdout is permitted.** The stdout-is-protocol-only rule binds protocol mode.
  Dispatch on `os.Args[1]` beginning `--` happens before any protocol I/O, and git never invokes
  a helper that way, so utility output can never interleave with the protocol stream. `--version`
  prints to stdout per convention; `--set-head` confirms in one line.
- **Release versioning continues the repo's single `v0.x` line** (existing tags `v0.2.0`–`v0.2.4`
  are the module's). Next release `v0.3.0`; the CHANGELOG states that one version line covers
  both tools in this repo. windows/amd64 only — the only platform with gate coverage; others
  build from source.
- **`listCompletePacks` stderr-note alignment (retro-Codex 2) resolves by narrowing the 3b spec
  to the code**, as a third post-implementation reconciliation bullet: only packish-looking skips
  get notes. Rationale: non-packish nodes in `packs/` are foreign junk the helper deliberately
  ignores, and per-node notes would make stderr volume depend on outside actors.

## Components

### 1. CLI version allowlist (`internal/transport`)

The seam is CLI-transport construction (`NewCLI`, `cli.go`): before the first filesystem
command, run `proton-drive --version` once per process and match the CLI identity token against
a compiled-in certified list — initially `{"cli-drive@0.7.0+5174900c"}`. The SDK line in the
version output is recorded in errors/warnings but not matched.

- **Mismatch or unparseable output → refuse to run**, before any filesystem command, with an
  error naming what was found, what is certified, and the override.
- **`GPB_UNCERTIFIED_CLI=1` → proceed**, printing a prominent stderr warning naming the untested
  version on every invocation. Never remembered, never cached across processes.
- Unparseable output is just an extreme case of uncertified: the override covers it (the warning
  then says the version could not be determined).
- All of this speaks on stderr and exit codes only; protocol stdout is untouched.

### 2. `--set-head` utility mode (`cmd/git-remote-proton`, `internal/repo`)

`git-remote-proton --set-head <address> <branch>` where `<address>` is anything `CanonicalRoot`
accepts (with or without the `proton::` prefix) and `<branch>` is a short name (normalised to
`refs/heads/<branch>`) or a full `refs/heads/…` ref. Anything outside `refs/heads/` is refused —
HEAD points at branches only, matching `WriteHEAD`'s existing non-branch refusal.

Flow: utility dispatch → allowlist check (component 1 applies everywhere) → `CanonicalRoot` →
`RequireMarker` → `AcquireLock` → verify `refs/heads/<branch>` exists remotely → update `HEAD`
to `ref: refs/heads/<branch>\n` → `Release`.

**A new `UpdateHEAD` function carries the overwrite.** `WriteHEAD` is backfill-only by pinned
contract (`TestWriteHEADNeverOverwritesExistingHEAD`) and is not touched. `UpdateHEAD` uses the
same staged-file CAS path pushes use (local basename `HEAD`, `upload -f merge` per probe C14),
with the same closed-set outcome handling as the marker path: Committed = done, Ambiguous =
"re-run to reconcile" refusal, unrecognised = fail closed.

The branch-delete refusal in `push.go` ("change the default branch first") is extended to name
the remedy: `git-remote-proton --set-head <url> <branch>`. `--version` ships in the same
dispatch: prints the ldflags-embedded version plus the certified CLI list.

### 3. Release pipeline (`.github/workflows/release.yml`, `install.ps1`, CHANGELOG)

- **Workflow:** triggers on `v*` tags plus `workflow_dispatch` (for dry runs). Steps: checkout,
  Go toolchain from `go.mod`, full hermetic test suite (the live half's loud skip is the
  expected shape in CI — no Proton account exists there), build windows/amd64 with `-trimpath`
  and `-ldflags "-X main.version=<tag>"`, compute SHA256, create the GitHub Release and attach
  `git-remote-proton.exe` + checksum file.
- **`install.ps1` extension:** a `-HelperExe <path>` parameter (default: `git-remote-proton.exe`
  beside the script, if present) installs the helper to
  `$env:LOCALAPPDATA\Programs\git-proton-backup\` and idempotently ensures that directory is on
  the user PATH. No exe found → informative skip; the module install is unchanged either way.
  No auto-build — the exe comes from a release download or a manual `go build`.
- **CHANGELOG:** `v0.3.0` section covering both tools, plus the one-version-line statement.
- Housekeeping: the stale untracked `git-remote-proton.exe` in the repo root is already
  gitignored; Craig deletes the file once the installed copy exists.

### 4. v1 coexistence docs (README)

A dedicated section: the two tools side by side (v1 = bundles into the sync folder, remote
`proton`, restore needs nothing but git; v2 = CAS repo via the CLI, remote `proton-v2`, restore
needs the helper installed — an honest-limits difference worth stating). Safe to wire both in
one repo. Keep v2 remotes under a dedicated root such as `/my-files/GitRemotes/`; never point v2
inside `GitBackups` — `Bootstrap` hard-refuses non-empty unmarked folders, but an empty
subfolder would be adopted. Plus the operational note from C17: a push failing
`already exists` out of nowhere has been observed once, unexplained; check/empty the Proton
trash and retry.

### 5. Test/hygiene wave (the deferred strengthens)

All GUARD-class unless a failure reveals a real defect — in which case stop and report, never
silently patch:

1. M1 — route-around trace assert (fetch trace strengthen from the 3b final review).
2. M3 — `HasObject` assertion in the incomplete-pair test.
3. Two-pack pair-refresh fixture (retro-Codex 1): restart-vs-resume of a multi-pack plan,
   currently structurally-but-not-empirically pinned.
4. The 3b-spec third reconciliation bullet for `listCompletePacks` stderr notes (decision above).
5. `ShowIndex`: wrap the `os.Open` error, add the garbled-line test.
6. `ResolveIdxCacheDir` err-ordering fix.
7. `pruneStale` comment documenting the complete-pairs-vs-name-in-listing divergence.
8. `copyFile`/`linkOrCopy` dedup.
9. Trace decorator: size-unknown branch test.

### 6. Docs/design updates

`docs/v2-remote-helper-design.md` → v6.4: the allowlist section (strategy + defense-in-depth
reconciliation), the `--set-head` operation, the updated delete-refusal remedy, utility-mode
stdout rationale. README per component 4. 3b spec per component 5 item 4.

## Error handling — new rows

| Condition | Behaviour |
| --- | --- |
| CLI version not on the certified list | Refuse before any filesystem command; name found vs certified and the override |
| `--version` output unparseable | Same refusal, treated as uncertified; override still applies |
| `GPB_UNCERTIFIED_CLI=1` set | Proceed with a prominent per-invocation stderr warning naming the untested (or undetermined) version |
| `--set-head` on a repo with no/invalid marker | `RequireMarker`'s existing named refusal |
| `--set-head` to a branch that does not exist remotely | Refuse; name the branches that do exist |
| `--set-head` to a non-`refs/heads/` ref | Refuse — HEAD points at branches only |
| `--set-head` while the lock is held | Existing lock refusal (holder nonce named) |
| `UpdateHEAD` outcome Ambiguous | Refuse with "re-run to reconcile", matching the marker convention |
| `install.ps1` finds no helper exe | Skip helper install with a message; module install proceeds |

## Testing

- **Allowlist matrix, hermetic:** certified → pass; mismatch → refusal naming both sides;
  override → warn + proceed; unparseable → refusal; override + unparseable → warn + proceed.
  Via an injected version source or a fake `proton-drive` on PATH — implementer's choice, but
  the certified-pass case must exercise the real parse of a captured genuine `--version` output.
- **`--set-head` matrix on the Fake:** success + HEAD content; short-name normalisation;
  no-marker refusal; unknown-branch refusal naming existing branches; tag refusal; lock-held
  refusal; refs never touched on any refusal path.
- **Dispatch:** git-style `argv` (`<name> <url>`) is unaffected; `--version` prints version +
  certified list; unknown `--flag` refused with usage on stderr.
- **Push:** the delete-refusal GUARD updated for the new remedy text.
- Wave items each record which assertion fired; deliberate-regression checks (assert the test
  fails with the guard removed) for anything load-bearing, per the runbook.
- Contract table unchanged; `-count=1` mandated on every gate invocation of it.

## The gate

Run 3b's standing rules verbatim (confinement, verify-before-trash, BLOCKED-and-never-patch,
`-count=1`), plus two now-codified pre-steps: record trash state before assuming anything about
it, and start with an emptied trash (Craig empties; the runner never does). Steps:

1. **Allowlist live:** the real CLI passes the check; a PATH-shimmed fake `proton-drive`
   reporting a wrong version proves the refusal and the override (shim confined to the gate's
   temp dir, never installed).
2. **`--set-head` live:** demo repo under `/my-files/GitRemotes/<demo>` with two branches;
   set-head to the second; verify `HEAD` content via the CLI and that a fresh `git clone` checks
   out the new default; the branch-delete refusal names the new remedy.
3. **Install story end-to-end:** `install.ps1` with the CI-built exe; a push/fetch through the
   PATH-installed helper.
4. **Release workflow proven** on an `-rc` pre-release tag before `v0.3.0` is cut.

## Sequencing

Hygiene wave first (small, de-risks everything after), then allowlist, then `--set-head`, then
release pipeline + docs, then the gate.

## Out of scope

Hierarchical refs (Stage 5, with the quarantine-staging refactor); EnsureDir already-exists
tolerance (dropped per C17b); compaction; any fetch-path changes; PowerShell Gallery publishing;
macOS/Linux builds.

## Revisions

(peer-review rounds append here)
