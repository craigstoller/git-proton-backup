# Releasing

This is the canonical release procedure for git-proton-backup (both the GitProtonBackup
PowerShell module and the git-remote-proton v2 helper share one version line and one release).
It codifies what Stage 4 established by practice — including a step that was manual and got
forgotten for the v0.3.x releases. Follow it in order; do not skip or reorder steps.

1. **Confirm `main` is green and the working tree is clean.** CI must be passing on the commit
   you intend to release from, and `git status` must show no uncommitted or untracked changes.
   Releasing from a dirty tree or a red `main` risks shipping something other than what was
   reviewed.

2. **Flip `CHANGELOG.md`'s `[Unreleased]` section to `[vX.Y.Z] — YYYY-MM-DD` and commit — BEFORE
   tagging.** This step was manual and was forgotten for the v0.3.x releases — the 0.3.1 entry
   was only dated after publication, in a separate commit (`50704a6`), rather than before
   tagging; it is the reason this document exists. Do this commit first, and tag it in the next
   step — never tag a commit whose CHANGELOG still says `[Unreleased]`.

3. **Tag `vX.Y.Z` on the flipped commit.** The tag must point at the commit from step 2, not a
   later one. Push the tag only on Craig's word — do not push a release tag unilaterally.

4. **The Release workflow builds a draft with exactly three assets.** Pushing the tag triggers
   the `Release` GitHub Actions workflow (`.github/workflows/release.yml`), which builds and
   publishes a **draft** release containing exactly three assets: `git-remote-proton.exe`,
   `git-remote-proton.exe.sha256`, and `install.ps1`. No more, no fewer.

5. **The live gate runs against the draft's bytes.** The gate downloads the draft's assets and
   exercises them against the real Proton account per the current gate brief (see
   `docs/research/gates/brief-checklist.md` for the standing rules every gate brief incorporates).
   Tags are never moved after any artifact has been built from them — if the gate finds a defect,
   the fix ships as a new tag, not a retag.

6. **Craig publishes.** Once the gate passes, Craig — not the gate runner — publishes the draft
   release on GitHub.

7. **The publication digest closure re-downloads the published assets and compares per-asset
   SHA-256 against the gate's staged digests.** Only after this closure confirms the published
   bytes are byte-identical to the bytes the gate tested is the release final.
