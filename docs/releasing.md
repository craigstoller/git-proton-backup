# Releasing

This is the canonical release procedure for git-proton-backup (both the GitProtonBackup
PowerShell module and the git-remote-proton v2 helper share one version line and one release).
It codifies what Stage 4 established by practice — including a step that was manual and got
forgotten for the v0.3.x releases. Follow it in order; do not skip or reorder steps.

1. **Confirm `main` is green and the working tree is clean.** CI must be passing on the commit
   you intend to release from, and `git status` must show no uncommitted or untracked changes.
   Releasing from a dirty tree or a red `main` risks shipping something other than what was
   reviewed.

2. **Flip `CHANGELOG.md`'s `[Unreleased]` section to `X.Y.Z — YYYY-MM-DD`, bump
   `ModuleVersion` in `GitProtonBackup/GitProtonBackup.psd1` to the same `X.Y.Z`, and commit both
   together — BEFORE tagging.** The CHANGELOG step was manual and was forgotten for the v0.3.x
   releases — the 0.3.1 entry was only dated after publication, in a separate commit (`50704a6`),
   rather than before tagging; it is the reason this document exists. The manifest bump is the
   0.6.0 policy ("the tag and `ModuleVersion` move together") and had been practice without being
   written here (e.g. `07e7d84` for 0.7.0); it is what lets an installed module be told apart from
   the one before it. Do this commit first, and tag it in the next step — never tag a commit whose
   CHANGELOG still says `[Unreleased]` or whose manifest still carries the previous version.

3. **Tag `vX.Y.Z` on the flipped commit.** The tag must point at the commit from step 2, not a
   later one. Push that commit to `main` first and wait for its CI to pass; only then push the tag,
   separately (not with `--follow-tags`). Push the tag only on Craig's word — do not push a
   release tag unilaterally.

4. **The Release workflow builds a draft with exactly three assets.** Pushing the tag triggers
   the `Release` GitHub Actions workflow (`.github/workflows/release.yml`), which builds and
   publishes a **draft** release containing exactly three assets: `git-remote-proton.exe`,
   `git-remote-proton.exe.sha256`, and `install.ps1`. No more, no fewer.

5. **The live gate runs against the draft's bytes.** The gate downloads the draft's assets and
   exercises them against the real Proton account per the current gate brief (see
   `docs/research/gates/brief-checklist.md` for the standing rules every gate brief incorporates).
   Tags are never moved after any artifact has been built from them — if the gate finds a defect,
   the fix ships as a new tag, not a retag. The one alternative is a stand-in, for a release that
   qualifies under "When adoption may stand in for the live gate" below and only on Craig's
   ruling for that release; for such a release the stand-in is this step, and the no-retag rule
   applies to it unchanged.

6. **Craig publishes.** Once the gate passes, Craig — not the gate runner — publishes the draft
   release on GitHub.

7. **The publication digest closure re-downloads the published assets and compares per-asset
   SHA-256 against the gate's staged digests.** Only after this closure confirms the published
   bytes are byte-identical to the bytes the gate tested is the release final. If any digest
   differs, or the published assets are not exactly the three staged ones, the published bytes are
   not the gated bytes: return the release to draft, investigate, and ship the fix under a new tag.

## When adoption may stand in for the live gate

A stand-in replaces step 5's live gate with installing the release on Craig's machine and checking
it there. It is Craig's ruling, made per release and recorded in that release's gate
record; this section says when he may make it and what the stand-in must then do. The 0.8.1 record
(`docs/research/gates/v0.8.1-release-gate.md`) is the worked example.

**Eligible only when nothing that builds or ships the helper has changed since the anchor,** the
most recent release whose own bytes passed a formal live gate (for 0.8.1, the anchor was v0.7.0).
All four must hold:

- `git diff <anchor-tag> <new-tag> -- '*.go' '*.s' '*.c' '*.h' go.mod go.sum go.work` is empty.
  That is every file the Go toolchain compiles, wherever it sits; add any file the helper embeds
  with `go:embed` (none today).
- `git diff <anchor-tag> <new-tag> -- .github/workflows/release.yml` is empty. The whole workflow,
  not chosen steps: its checkout, `setup-go`, Version, Build and Checksum steps all reach the exe,
  and its Create DRAFT release step sets `--draft` and the three-asset list. (A Dependabot bump to
  this file therefore means the next release takes the formal gate.)
- `go version -m` on the draft exe and on the anchor's published exe reports the same Go version
  and the same build settings; on the draft, `vcs.revision` is the tagged commit and
  `vcs.modified=false`. `go.mod` declares a language floor, not a patch release, so the toolchain
  can move between builds; this check shows whether it did, and if it did, the release is not
  eligible.
- The installed Proton Drive CLI reports the certified build (`--version`), the one the anchor's
  live gate ran against.

The two exes then differ by the embedded version string and VCS stamps, and their behaviour is
expected to be the same: an inference from identical source, recipe and toolchain, not a byte
comparison. Anything else takes the formal live gate. The anchor never advances through a stand-in:
it stays the last formally gated release, so a run of stand-ins cannot drift away from live
evidence.

**What the stand-in covers live, and what it does not.** The module reaches Proton only through the
sync folder and the Proton Drive CLI, never through the helper, and the fleet verify (stand-in
step 6) exercises both against the real account. No push or fetch goes through the draft helper;
that evidence comes from the anchor, and the gate record says so. The stand-in's only writes to the
account are the fleet verify's own: the bundles it cuts and the old bundles its retention prunes,
exactly as the daily run makes them; it adds no test writes of its own. Of `brief-checklist.md`'s
rules, rule 4 binds it (stop and report BLOCKED with verbatim output, never patch), and rule 5
(`-count=1`) binds any `go test` it runs; the rest concern a live gate's own test writes, listings
and pushes, which a stand-in does not make.

**The stand-in, in order:**

1. **Offline checks on the draft's bytes.** Exactly three assets; the exe's SHA-256 equals its
   `.sha256` sidecar; `git-remote-proton.exe --version` prints the new tag and the certified CLI
   token; `install.ps1` equals the tagged blob apart from line endings; the `go version -m`
   comparison above. Stage all three digests for release step 7. The module ships through the
   repository, not as an asset, so also record its tree and file blobs at the tag.
2. **Prepare.** Record the installed module and helper versions and keep a copy of their files and
   of each consumer pin's value, so the rollback restores what was actually installed. Confirm no
   backup or verify is running, and that the next scheduled run leaves time for the whole stand-in
   (or disable that task until it is accepted or rolled back). Take a read-only baseline of the
   registry and of the last verify result.
3. **Install the release.** Run `install.ps1 -Force` from an export of the tag's tree, with
   `-HelperExe` pointing at the draft exe and its `.sha256` beside it: the exe is the draft's bytes,
   and the module comes from the tag rather than from a working tree. The export is the copy step 1
   matched to the asset; without the sidecar, the installer skips its checksum check. Then, in a
   fresh `-NoProfile` process, confirm the module version, that its files match the tag's blobs
   after line-ending normalisation (`git hash-object --path=…`), and that
   `Get-Command git-remote-proton -All` finds exactly one, with the draft's hash and version.
4. **Exercise what the release changed, against the installed module,** in an isolated state root
   (`GPB_CONFIG_DIR`, `GPB_LOCK_PATH`, `GPB_HOOK_DISABLED=1`, the root asserted before any action),
   with each check's pass condition written down before the run. Never against the real registry.
   If the release changes nothing the module runs, the record says so and this step is empty.
5. **Bump pinned consumers in the same sitting and run their contract suites,** because under the
   in-place install a stale exact-version pin fails to import. Today that is `RequiredEngineVersion`
   in `project-operating-standards`' `config/backup-engine.psd1`, with its backup suites.
6. **Fleet verify from a fresh session,** with no `GPB_*` overrides, so the real state root is in
   use. Acceptance: exit 0 and complete; the verified repositories are exactly the registry's at the
   baseline; none is in attention that was not already, and none already in attention has a new
   finding.
7. **Write the gate record** under `docs/research/gates/`: Craig's ruling, the anchor and the checks
   that qualified the release, what the stand-in does not cover, each step's result, the staged
   digests, and the rollback.

**Rollback** follows any failed stand-in step from 3 on: reinstall the copies kept in step 2, set
each pin back (reverting a pin commit already made), and confirm with a fleet verify, all in one
sitting; if the rollback itself fails, stop and report BLOCKED. A step that fails for an
infrastructure reason (a download, the network) may be retried with the same bytes; a defect in the
release fails the stand-in, which is a failed gate, and the fix ships as a new tag. Publication and
the digest closure (steps 6 and 7 of the release) follow as for any release.
