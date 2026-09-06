# Digest stamp outside the sync root — design

**Date:** 2026-09-05. **Scope:** the `GitProtonBackup` PowerShell module (v1 bundles) only; the
v2 remote helper is untouched. **Shape:** a bounded change to an existing flow, so this document
carries its own task list instead of a separate plan file. Steps that live in other repos or in
Craig's hands are labelled *external*.

## Problem

`Invoke-RepoBundleBackup` records "the last digest I published" in a 64-byte marker file,
`.<BundleBaseName>.lastdigest`, written into the bundle directory — which sits inside the Proton
Drive sync root (`<ProtonDriveRoot>\<BackupSubdir>\<slug>\`). The marker is rewritten immediately
after a bundle (up to ~100 MB) is renamed into place, so the sync app uploads the tiny marker
behind the large bundle.

On 2026-07-30 the Proton Drive Windows app (3.0.6) mis-recorded its own slow upload of one such
marker as a foreign remote edit: the server committed the revision, the event stream delivered it
before the app's own upload call returned, and the app filed it as a remote change. Every sync
cycle from then until 2026-09-05 (~10 min) retried the local edit and failed with
`ContentVersionDiverged`. The app showed a permanent "failed to sync" badge; restarts did not clear
it because the state lives in the app's SQLite sync engine. (App-internal behaviour is taken from
the app's own logs and databases, read during the 2026-09-05 diagnosis; nothing in this repo can
verify it.)

The wedged node was cleared by hand on 2026-09-05 (*external*): deleting or delete-and-recreating
the local file did **not** clear it — the app reconciled a local delete against its phantom remote
edit and failed the same way — and what worked was a fresh **remote** revision of the same node,
uploaded with the Proton CLI (`filesystem upload -f create-new-revision`). That result matters for
this design: a local delete is hygiene, not a cure. This change is about making sure the class
cannot recur on the tool's marker; it is not a remedy for an already-wedged node.

Backups were never affected — the module reads only the local copy of the marker, every bundle is
present server-side, and fleet verify is exit 0 — but the tool's own bookkeeping file was the sole
cause of a five-week standing sync error, and nothing about it needs to be on Proton.

## What the marker is, and who touches it

- **Purpose:** one half of the cache-hit test. A run skips bundling only when the stamped digest
  equals the current ref digest **and** the newest bundle file's name carries that digest's first
  eight hex characters. The stamp alone never claims coverage (`docs/design.md`, "Fail-closed
  publication"); losing it costs one re-cut, never coverage. Conversely, a stamp that is *current*
  can only ever agree with bundles that are really there: a stale stamp forces a re-cut on the next
  ref change, and a current one is confirmed against the bundle name — so which process wrote the
  stamp never matters, only whether it is current. **This bundle-name contract is load-bearing for
  every compatibility argument below** (mixed versions, shared keys, migration windows). It is not
  new: `docs/design.md` states it ("Every bundle filename carries a fragment of its content
  digest" / "a cache hit requires both"), and `tests/GitProtonBackup.Tests.ps1` already pins both
  halves ("bundle filename carries the digest fragment" and "cache hit requires the CURRENT digest
  bundle"). (The stamp is always a 64-character SHA-256 hex string: `Get-RepoRefDigest`'s `EMPTY`
  sentinel never reaches it, because an empty repo fails at `git bundle create` before the stamp is
  written — a positional guarantee of the publish sequence, not a check at the write.)
- **Writer:** `Invoke-RepoBundleBackup` (`GitProtonBackup.psm1`, the `Set-Content` after a
  successful publish). Fail-closed: written only after the bundle is verified and renamed. A
  missing stamp always sends the function into its cut-and-stamp branch, so there is no state in
  which a repo sits stamp-less on a cache hit. Writers never race: the push hook holds the single
  backup lock across its bundle step and Verify holds it across each repo's whole pass, and a push
  that cannot get the lock defers without bundling (`docs/design.md`, "Locking").
- **Readers:** `Invoke-RepoBundleBackup` (cache-hit) and `Get-ProtonBackupStatus`
  (`CurrentBundled`, a read-only replica of the same comparison). `Invoke-ProtonBackupVerify` reaches
  it only through `Invoke-RepoBundleBackup`, which it calls for every registered repo that is
  present on disk, on every run — so the fleet's stale-digest reconciliation is indifferent to
  where the file lives, and one verify run touches every registered, present repo.
- **The only file the tool edits in place inside the sync root.** Bundles are written once under a
  unique digest-suffixed name and later deleted by retention; the `.partial` is renamed, never
  modified. The marker is the one file that is *overwritten* on every publish, and the incident was
  an edit-revision conflict on exactly that overwrite. That is why the incident is marker-shaped —
  and it is also the honest limit of the argument: the underlying race (the server's event stream
  beating the app's own upload acknowledgement) is not proven marker-only. Bundle and `.partial`
  writes stay in the sync root by design; the incident has only ever been observed on the marker,
  and if it ever recurs on a bundle this design has no answer for it (the CLI re-upload above is
  the known manual remedy for a wedged node of any kind).
- **Not a restore input.** Restore is `git clone <bundle>`; the README's Restore section and the
  `HOW-TO-RESTORE.md` note in the live bundle root (*external*, not this repo's file) both say the
  marker is ignorable bookkeeping.
- **Not shared across machines.** The design's "one machine per repo" limit stands; moving the
  stamp local makes it literally true that no state is shared through Proton. Two machines with
  identical refs still converge rather than thrash: each cuts at most one extra bundle, after which
  both see a newest bundle carrying the shared digest fragment and cache-hit on it. Machines with
  *different* refs were never a supported topology, before or after this change.
- **External consumer checked (*external*, read 2026-09-05):**
  `project-operating-standards/scripts/backup/ProjectsBackup.psm1` carries its own fork of the
  bundling function that writes a same-named marker — into its own legacy tree
  (`Project Repo Bundles\<relpath>`), not the `GitBackups\<slug>` directories this module owns — and
  its production sweep runs `EngineMode = 'delegated'`, delegating coverage to this module's
  `Invoke-ProtonBackupVerify`, which operates only on this module's own directories. No shared file,
  so no interaction with this change. (Its own legacy markers are the same class of hazard, and a
  matter for that repo.)

## Decision

Keep the digest stamp under the module's own state root, outside the sync root:

```
<GpbRoot>\digests\<bundle-dir-leaf>.<BundleBaseName>.lastdigest
```

where `<GpbRoot>` is `%LOCALAPPDATA%\GitProtonBackup` (or `GPB_CONFIG_DIR`), alongside the
existing `push-pending\` and `mirrors\`. Uniqueness comes from the bundle directory's leaf alone —
in production the repo slug `<leaf>-<hash10>` from `Get-GpbSlug`, unique per repo path; the base name
is appended for human readability and to mirror the old `<slug>\.<BundleBaseName>.lastdigest` layout
one-to-one, not because it adds uniqueness. (Keying on the full bundle path was considered and
rejected: a hashed name is opaque, and the only scenario where the leaf is ambiguous — the same
slug under a different `ProtonDriveRoot`/`BackupSubdir` — is already safe either way under the
bundle-name contract: if the bundles are not in the current bundle directory the newest-bundle half
of the test forces a re-cut, and if they are, a current stamp is a true statement about them
regardless of which configuration wrote it.)

### Compatibility: read the old location, migrate once, clean up best-effort

1. **Read order.** New location first, then the legacy file beside the bundles, else `''`. A read
   that *fails* (a file held open without read sharing) is treated exactly like an absent file and
   the next candidate is tried, so a locked new-location file with a readable legacy one still
   yields the legacy content. For the bundler an unreadable stamp costs one re-cut; for Status it
   shows `CurrentBundled = $false` until the next push or verify — transient, accepted, and no
   different from today's single-location read, which has no error handling at all. Content that is
   not a digest never equals one, with the same result. One helper serves both readers, so Status
   and the bundling step read the same files in the same order. (The newest-bundle half of the
   comparison remains duplicated between them, as it is today — this change does not unify it.)
2. **One writer helper** creates `digests\` on demand and writes the stamp. Both the publish path
   and the migration below go through it, so a migration on a cache hit — the common case on the
   first post-upgrade sweep, when no publish has yet created the directory — cannot fail for want of
   the directory.
3. **Migration** happens inside `Invoke-RepoBundleBackup` only — which means on every push hook
   run and, because Verify calls it per registered repo, on every verify run — before the cache-hit
   comparison, with the legacy cleanup repeated after a successful publish. Algorithm, every step
   best-effort and none of them throwing:
   1. If no legacy file exists, nothing to do.
   2. Read the legacy content. If the read fails, skip the copy (the cache read will independently
      treat the stamp as absent and re-cut; the next run retries).
   3. If the new-location file does not exist **and** the trimmed content is a well-formed digest
      — 64 hex characters, matched case-insensitively, the same tolerance the cache comparison's
      string equality has, so no stamp the cache would honour can be left behind as "malformed" —
      write it there through the writer helper. Malformed content is never copied into the state
      root. If the write fails, stop here — the legacy file is left in place, still readable through
      the fallback. A new-location file that already exists is never overwritten: the new location
      is authoritative (in the mixed-version window below that can cost one extra re-cut, which is
      the ping-pong already described, never a coverage error).
   4. Once the new-location file exists — because it already did, because step 3 just wrote it, or
      because this run's publish just stamped it — delete the legacy file. If the delete fails (the
      sync app holding a handle), leave it; the next run retries. A malformed legacy file is thus
      removed in the same run as the re-cut it provokes, once the publish has stamped the new
      location.
   Every intermediate state is coherent under the read order: new-only reads the new stamp; both
   present reads the new stamp and deletes the leftover; legacy-only reads the legacy stamp. So a
   refused delete is hygiene debt (a marker lingering in the sync root), never a correctness
   hazard — precisely because the new location is read first.
   Migration runs regardless of how the rest of the call turns out, including a publish that
   fails: the stamp never claims coverage, so moving it before the outcome is known risks nothing
   (a stale stamp that was going to be overwritten by the publish is overwritten by the next
   successful one instead — exactly as a failed publish behaved before).
   Deleting the legacy file is hygiene, and deliberate: it takes the tool's one in-place-edited
   file out of the sync app's purview for good. It is the tool's own 64-byte file (retention already
   deletes the tool's bundles from the same directory); server-side the deletion lands in Proton
   trash, recoverable, and worthless. The app processes the delete in queue order, behind whatever
   upload it already has in flight. It is **not** a cure for a wedged node — see Problem.
4. **`Get-ProtonBackupStatus` reads but never writes.** It is documented read-only (the only
   existing caveat is marker quarantine), and it never needs to write: a repo with no stamp anywhere
   is one the next push or verify will re-cut and re-stamp, exactly as today, and Status showing
   `CurrentBundled = $false` for it until then is the pre-existing meaning of a missing stamp, not a
   new false alarm.
5. **The publish path writes only the new location.** The existing "digest stamp failed (bundle
   published; next run re-bundles)" finding is unchanged.
6. **`Uninstall-ProtonBackup` removes the repo's stamp from both locations**, best-effort, alongside
   the mirror and the push-pending marker it already removes. Without this, `digests\` would
   accumulate entries for repos long unwired, and a repo uninstalled while its legacy marker still
   existed would leave that marker in the sync root forever. Bundles stay untouched; the "Existing
   bundles on Proton Drive were left in place" message remains true. Known leaks, accepted because
   the files are 64 bytes and inert: a repo that is *moved or renamed* gets a new slug and the old
   slug's stamp is never reaped; a registered repo that is missing on disk is skipped by Verify, so
   its legacy marker is not migrated by the sweep and stays until it is uninstalled (which now
   removes it) or removed by hand.
7. **No config key, no version gate.** Nothing in `config.json` changes, so `Read-GpbConfig`'s
   strict missing-key check is untouched.

### What this does not do

- It does not stop the sync app from seeing the `.partial` file during a publish; that file is in
  the bundle directory on purpose (same-volume atomic rename) and is renamed within seconds.
- It does not touch `HOW-TO-RESTORE.md` in the live bundle root or the pos runbook (both
  *external*); those say "ignore the `.lastdigest` files", which becomes moot rather than wrong once
  migration has run.
- It does not, and cannot, un-wedge a sync app. The 2026-07-30 node was cleared by hand before this
  change ships (see Problem), and the live attempts showed that a local delete does not clear a
  wedged node. What the migration's delete achieves is that the app never again tracks this file.
  If a legacy delete keeps failing because the app holds the file open, the module is unaffected
  — its stamp is already local — and the remaining remedy is manual: remove the file with Explorer
  or the Proton Drive web UI. The "no `.lastdigest` beside the bundles" property is therefore
  eventual on a machine where deletes are being refused, not instant.

### Known caveat: mixed module versions re-create the hazard, not just the cost

An older module version knows only the legacy location — it reads there and, worse, **writes**
there. If old and new code both run against the same bundle root, the old one finds no stamp after
migration, re-cuts a bundle on every run, and re-creates the marker *inside the sync root*, handing
the sync app the exact file this change removes. The new code keeps cache-hitting throughout (the
old code's bundle carries the same digest fragment) and migrates the marker away again on each of
its own runs; the old code re-cuts on each of its runs until it is gone. Nothing in the new module
can prevent this; the mitigation is process only. Under the in-place installer (verified: it copies
one flat directory, so the disk holds one version at a time) "old code" means:

- **any PowerShell session that imported the module before the upgrade** — the files on disk are
  replaced, but a session that already holds the old module keeps running old code until it is
  closed or re-imports with `-Force`;
- a scheduled verify already *running* at install time (a task that starts afterwards imports by
  bare name and gets new code);
- a second, older copy retained somewhere on `PSModulePath` (not something the installer produces).

A stale `-RequiredVersion` pin in an *external* caller is a different failure: under a flat install
it finds no module of that version and fails to import — loudly, or fail-soft in a caller built for
it — rather than running old code. It still needs bumping in the same sitting, so the caller
resumes delegating. The `ModuleVersion` bump makes a stale in-memory session *inspectable*
(`(Get-Module GitProtonBackup).Version`); nothing in this design checks it automatically (recording
the loaded version in `last-verify.json` was suggested in review and is deferred as scope creep),
so the actual guard is the adoption order: install, bump any external pin, then run the fleet
sweep **from a fresh session** straight away. The window is minutes, but it is a real hazard, it
belongs in the CHANGELOG, and the test suite does not exercise it (it would need a second, older
module copy).

Practice has been to bump `ModuleVersion` in the release commit together with the CHANGELOG flip
(the 0.6.0 policy line: "the tag and `ModuleVersion` move together"; e.g. release commit `07e7d84`
for 0.7.0) — but `docs/releasing.md` step 2 does not say so, and that procedure exists precisely
because an unwritten manual step was once forgotten. Task 4 writes the bump into step 2.

## Tests (TDD — each written red before its implementation)

`tests/GitProtonBackup.Tests.ps1`
- Derived path: `Get-GpbDigestStatePath` is under `Get-GpbRoot`, keyed by bundle-dir leaf and base
  name, and is not under the supplied `-BundleDir` (the property the incident response depends on,
  pinned directly rather than inferred from a filename).
- Successful publish stamps the **new** location with the digest and leaves **no** `.lastdigest` in
  the bundle directory (the headline property).
- Empty repo and publish failure: no stamp at either location.
- Legacy stamp beside the bundles, `digests\` not yet existing: honored as a cache hit (no new
  bundle, `Created` false), copied to the new location, removed from the bundle directory.
- Precedence: new-location stamp current, legacy stale but well-formed — no re-cut, legacy removed,
  and the new-location content is byte-for-byte unchanged (the no-overwrite rule).
- Malformed legacy content: not copied to the new location; the run re-cuts and stamps the new
  location; the legacy file is removed in that same run.
- Best-effort cleanup, legacy held open **with** read sharing (`FileShare.Read`): the cache hit is
  still honored through the fallback read, the new location is written, the delete is deferred
  (legacy still present), nothing throws; after the handle closes, the next run removes it.
- Best-effort cleanup, legacy held open **without** read sharing (`FileShare.None`): the run
  re-cuts (stamp unreadable), nothing throws, the legacy file survives the run; after the handle
  closes, the next run removes it (the new-location stamp already exists from the re-cut).

`tests/Commands.Tests.ps1`
- The fabricated-state Status test writes its stamp to the new location.
- Status honors a legacy stamp (`CurrentBundled` true) **without** migrating it.
- Verify migrates a legacy stamp without re-cutting (bundle count unchanged across the run).
- Uninstall removes the stamp from both locations and leaves the bundles.

Existing assertions on `.r.lastdigest` / `.e.lastdigest` in the bundle directory flip to the new
location or to "absent everywhere" as appropriate.

## Docs

- `docs/design.md`: the two "Fail-closed publication" bullets that locate the stamp beside the
  bundle get the new location, plus a short "Where the digest stamp lives" paragraph — the incident,
  the mechanism (the one in-place edit in the sync root), the new location, the migration policy,
  the residual.
- `docs/releasing.md` step 2: the `ModuleVersion` bump in `GitProtonBackup.psd1` lands in the same
  commit as the CHANGELOG flip.
- `CHANGELOG.md`: an `[Unreleased]` entry naming the new location, the one-time cleanup users will
  see in their Proton Drive folder (and its trash), the Uninstall change, and the mixed-version
  re-wedge caveat including stale sessions.
- No README change: it never mentioned the marker.

## Revisions

- **Round 1 (2026-09-05; panel: Gemini, DeepSeek, Kimi reported; Codex failed to read the file).**
  Applied: mixed-version caveat now states the re-wedge, not just the re-cut [DeepSeek]; migration
  algorithm spelled out step by step with read/write/delete failure handling and the
  delete-only-after-new-exists rule [Gemini, Kimi]; "never disagree" softened to what the shared
  reader actually unifies [DeepSeek]; migration-before-outcome rationale stated [DeepSeek];
  Uninstall removes both stamps (orphan cleanup) [Gemini, Kimi]; residual stated — the race is not
  proven marker-only, bundles/.partial stay in the sync root, and the marker is the one in-place edit
  [Kimi]; persistent-lock / unprocessed-delete fallback named [Gemini, DeepSeek]; adoption gains an
  explicit fleet-sweeping verify run and a Task 7 observation with a manual fallback [DeepSeek,
  Kimi]; external steps and unverifiable claims labelled [DeepSeek, Kimi]; tests gain the
  no-overwrite assertion and both lock variants [Kimi]. Rejected: "Verify migrating is an
  undeclared side effect" — Verify is the reconciliation pass that re-cuts bundles; it has never
  been read-only [Gemini]. "Status must write a stamp or show false forever" — a missing stamp
  always drives the bundler into its cut branch, which re-stamps; no stamp-less cache-hit state
  exists [Kimi]. "External consumer breaks because Verify looks in new paths" — the consumer's own
  markers are in a different tree and Verify only ever operated on this module's directories
  [Gemini]. "A stale stamp from another config suppresses a needed cut" — a stamp that matches the
  current digest and the newest bundle's name is true regardless of who wrote it [Kimi].
  Deferred: unifying the newest-bundle half of the comparison between Status and the bundler
  (pre-existing duplication, out of scope); a suite test for the mixed-version ping-pong (needs a
  second installed module version); bumping `ModuleVersion` now (release commit).
- **Round 2 (2026-09-05; panel: Gemini, DeepSeek, Kimi reported; Codex failed to read the file
  again).** Applied: a single writer helper creates `digests\` on demand so migration on a cache hit
  cannot fail for want of the directory, with a test [Gemini]; `docs/releasing.md` step 2 gains the
  `ModuleVersion` bump, and the caveat now cites the actual practice instead of a step the
  procedure did not contain [DeepSeek, Kimi]; the mixed-version window now names stale in-memory
  sessions and an in-flight scheduled verify, and adoption says "from a fresh session" [Kimi];
  malformed legacy content is never copied, with a test [Kimi]; the derived-path test also pins "not
  under the bundle dir" [Kimi]; moved/renamed-repo stamp orphans acknowledged under Uninstall
  [DeepSeek]; the `EMPTY` sentinel is shown never to reach the stamp [DeepSeek]. Also applied from
  live evidence, not a review finding: the wedged node was cleared by hand at 20:05 PT by a CLI
  re-upload, and a local delete was shown **not** to clear a wedged node — the Problem, migration
  rationale, "What this does not do" and Task 7 now describe the delete as hygiene and this change
  as prevention. Rejected: "concurrent writers contend for the new stamp file" — the single backup
  lock serializes the push hook's bundle step and Verify's per-repo pass; a push that misses the
  lock defers without bundling [Gemini]. "Localizing the stamp turns two-machine use into storage
  thrash" — identical refs yield identical digest fragments in the bundle names, so both machines
  cache-hit after at most one extra bundle each; divergent refs were never supported [Gemini].
  "Premature migration pollutes the state root on a failed publish" — same behaviour as a failed
  publish today, acknowledged in the text as not a regression [Gemini].
- **Round 3 (2026-09-05; panel: Gemini reported "Blockers: none", DeepSeek and Kimi reported; Codex
  failed to read the file a third time — its sandbox cannot spawn the process it reads files with on
  this machine).** Applied: the migration's well-formed check is case-insensitive on trimmed
  content, the same tolerance as the cache comparison, so no stamp the cache would honour can be
  left behind [DeepSeek]; a stale `-RequiredVersion` pin under a flat install fails to import rather
  than running old code, and the caveat now says so; the `ModuleVersion` bump is described as
  making a stale session inspectable, not observed, with the actual guard named [DeepSeek]; the
  no-overwrite rule is stated as "the new location is authoritative" with its mixed-window cost,
  not as an absolute freshness claim [DeepSeek]; a registered repo missing on disk is skipped by
  the sweep, noted under the accepted leaks [DeepSeek]; the bundle-name contract is named once as
  the load-bearing invariant behind every compatibility argument, with pointers to where
  `docs/design.md` states it and the suite already pins it [Kimi]; the migration's intermediate
  states are argued coherent under the read order, with a refused delete classed as hygiene debt
  [Kimi]; uniqueness attributed to the slug alone, base name as readability [Kimi]; a failed read
  now explicitly falls through to the next candidate, and Status's transient false negative on an
  unreadable stamp is accepted in writing [Kimi]; the `.bad`-quarantine analogy dropped [Kimi]; the
  `EMPTY` guarantee labelled positional [Kimi]; the legacy-marker count in Task 7 labelled as
  observed [Kimi]; the legacy delete also runs right after a successful publish, so a malformed
  leftover is removed in the same run (a simplification that fell out of the coherence argument).
  Rejected: "add a contract test for the digest-fragment-in-filename invariant" — both halves are
  already pinned in `tests/GitProtonBackup.Tests.ps1` [Kimi]. Deferred: recording the loaded
  `ModuleVersion` in `last-verify.json` [DeepSeek]. Loop closed at the three-round cap with every
  tracked finding applied, rejected with a reason, or deferred.

## Tasks

1. RED: path-derivation (incl. not-under-bundle-dir) and headline-property tests; GREEN:
   `Get-GpbDigestStatePath`, `Get-GpbLegacyDigestStatePath`, `Read-GpbLastDigest`,
   `Write-GpbLastDigest` (creates `digests\`), publish path moved.
2. RED: legacy (no `digests\` yet) / precedence / malformed / both-lock tests; GREEN:
   `Move-GpbLegacyDigestStamp` called from `Invoke-RepoBundleBackup`, legacy delete repeated after
   a successful publish.
3. RED: Status, Verify and Uninstall tests in `Commands.Tests.ps1`; GREEN: `Get-ProtonBackupStatus`
   on the shared reader, `Uninstall-ProtonBackup` removing both stamps.
4. Docs: design.md edits, releasing.md step-2 amendment, CHANGELOG entry.
5. Full Pester run (all four suites) and PSScriptAnalyzer with the repo settings; commit.
6. *External.* Release is Craig's call per `docs/releasing.md` (CHANGELOG flip with the
   `ModuleVersion` bump, tag, gate, publish). Adoption on this machine, in one sitting and in this
   order: `install.ps1 -Force`; bump the pos engine pin; open a **fresh** PowerShell session and run
   `Invoke-ProtonBackupVerify` once to migrate the whole fleet in a single pass.
7. *External.* After that sweep, confirm the legacy markers are gone from the bundle folders (24
   were present across 25 registered repos when surveyed on 2026-09-05) and that the Proton Drive
   app processes the deletes cleanly (no new `ContentVersionDiverged` lines in its log, badge stays
   clear). Any marker still present after a few sync cycles is removed by hand — the module no
   longer depends on it either way.
