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
   CHANGELOG still says `[Unreleased]` or whose manifest still carries the previous version. The
   date in the heading is the flip's and the tag's, not the day Craig publishes: a release
   published the next morning keeps the tag's date (v0.9.0, flipped 2026-10-10 and published
   2026-10-11; Craig's ruling of 2026-10-11).

3. **Tag `vX.Y.Z` on the flipped commit.** The tag must point at the commit from step 2, not a
   later one. Push that commit to `main` first and wait for its CI to pass. Before pushing the
   tag, run the Proton CLI drift check (`scripts/Test-ProtonCliDrift.ps1`; see "Between
   releases" below) with `-Helper` pointing at a helper built from the step-2 commit
   (`go build -o <scratch dir>\git-remote-proton.exe ./cmd/git-remote-proton`, outside the
   tree); for a release that moves the CLI pin the installed helper still carries the old token
   and would fail by construction, while for a release that does not move the pin the installed
   helper carries the same token and may be used instead. Require `DRIFT CHECK: MATCH`. Any
   MISMATCH is a STOP, and the default
   resolution is to re-certify against the current build: a release that certifies a build the
   download page no longer offers recreates the failure the 0.9.0 certification was written for.
   Shipping one behind needs an explicit Craig rationale. Only then push the tag, separately (not
   with `--follow-tags`). Push the tag only on Craig's word — do not push a release tag
   unilaterally.

4. **The Release workflow builds a draft with exactly five assets.** Pushing the tag triggers
   the `Release` GitHub Actions workflow (`.github/workflows/release.yml`), which builds and
   publishes a **draft** release containing exactly five assets: `git-remote-proton.exe`,
   `git-remote-proton.exe.sha256`, `install.ps1`, `git-remote-proton-linux-amd64` and
   `git-remote-proton-linux-amd64.sha256`. No more, no fewer. The Linux binary is a static
   linux/amd64 build (`CGO_ENABLED=0`) cross-built on the same Windows runner.

5. **Two live gates run against the draft's bytes: the Windows live gate and the Linux live
   gate.** Each downloads the draft's assets and exercises its own binary
   (`git-remote-proton.exe` for Windows, `git-remote-proton-linux-amd64` for Linux) against the
   real Proton account per its own current gate brief (see
   `docs/research/gates/brief-checklist.md` for the standing rules every gate brief incorporates).
   For v0.10.0, the first release with a Linux binary, the Linux gate's brief is
   `docs/research/gates/v0.10.0-linux-gate-brief.md`.
   Its Windows gate's brief is `docs/research/gates/v0.10.0-windows-gate-brief.md`; that gate runs
   after the Linux gate has passed, and asserts the draft helper's install rather than making it.
   A release that moves the CLI pin installs the draft helper before the Windows gate smoke, as
   v0.7.0's gate did (Task 3 step 4 of the certification plan, today
   `docs/superpowers/plans/2026-10-10-cli-0.9.0-certification.md`), so that the helper under test
   and the helper on PATH are the same bytes.
   For v0.10.0 the Linux gate's brief installs the draft Windows helper itself and keeps a rollback
   copy (Craig's ruling, 2026-10-16). If a later gate fails or the release is abandoned after that,
   restoring the rollback copy is part of abandoning the release: pause the scheduled tasks,
   restore and verify the copy, then re-enable them.
   Tags are never moved after any artifact has been built from them — if either gate finds a
   defect, the fix ships as a new tag, not a retag. The one alternative is a stand-in, for a
   release that qualifies under "When adoption may stand in for the live gate" below and only on
   Craig's ruling for that release; for such a release the stand-in is this step, and the
   no-retag rule applies to it unchanged.

6. **Craig publishes.** Once the gates pass, run the drift check again with `-Helper` set to the
   draft's exe — the installed one when step 5 installed it (a pin-moving release), otherwise the
   downloaded asset in the Windows gate's staging directory — and require `DRIFT CHECK: MATCH`
   under the same STOP rule as step 3. Then Craig — not the gate runner — publishes the draft
   release on GitHub.

7. **The publication digest closure re-downloads the published assets and compares per-asset
   SHA-256 against the gates' staged digests.** Only after this closure confirms the published
   bytes are byte-identical to the bytes the gates tested is the release final. If any digest
   differs, or the published assets are not exactly the five staged ones, the published bytes are
   not the gated bytes: return the release to draft, investigate, and ship the fix under a new tag.

## Between releases: the Proton CLI drift check

Proton's download page and its `version.json` feed advertise only the current CLI build, and the
helper certifies exactly one build, so a CLI release Proton ships between our releases means every
new install meets an allowlist refusal until we re-certify. Nothing detects that at release time
alone: the 0.9.0 move of 2026-10-05 went unnoticed for five days because the check the 0.8.0
certification plan asked for was never written down. It is written down here.

**Weekly, Craig-owned:** run `scripts/Test-ProtonCliDrift.ps1` with no arguments (the installed
helper and the `proton-drive` first on PATH). It compares five things and names which failed: the
installed CLI's bytes against the feed, the installed CLI's own identity token against the helper's
certified token, the feed's version against the certified token's semver, the feed's
`linux/x64` SHA-512 for that same release against the value the script records
(`$CertifiedLinuxX64Sha512`), and the feed's `macos/arm64` SHA-512 for that same release against
its own recorded value (`$CertifiedMacArm64Sha512`). The fourth leg compares the feed with a
recorded value: it cannot see an installed Linux CLI, because nothing on Windows hashes one, so it
says nothing about the CLI on any Linux machine. The fifth leg, like the fourth, compares the feed
with a recorded value and cannot see a CLI installed on any Mac. A release with no `linux/x64`
entry fails the fourth leg (Proton dropping or delaying Linux is drift worth a certification look),
and a release with no `macos/arm64` entry fails the fifth. `DRIFT CHECK: MATCH` means nothing has
moved.
A MISMATCH between releases means Proton has moved: open a certification task in the shape of
`docs/superpowers/plans/2026-10-10-cli-0.9.0-certification.md`. The same command is the **first
triage step** whenever verify or a user reports an uncertified-CLI refusal.
On Craig's machine a weekly Windows scheduled task runs the script (Craig's ruling of 2026-10-11;
agents cannot create scheduled tasks there), appending its output to
`%LOCALAPPDATA%\GitProtonBackup\drift-check.log` with the script's exit code as the task's last
result. The task is only as good as someone reading that log, which is why the check stays
Craig-owned; the script bounds its one network call (30 s) so an unreachable feed cannot hang the
task, and reports STOP, not MISMATCH, when the feed is reachable but unusable (a `windows/x64`,
`linux/x64` or `macos/arm64` entry whose SHA-512 is not 128 hex characters).

**Two expected shapes, not drift:** while a certification is in flight and the machine's CLI has
already been swapped to the new build but the release that certifies it is not yet installed, the
check reports the five fields `installed CLI == feed: True; installed CLI == helper's certified
token: False; feed version == certified semver: False; feed linux/x64 == certified linux/x64:
False; feed macos/arm64 == certified macos/arm64: False`. The fourth and fifth are False because
the feed's `linux/x64` and `macos/arm64` entries move with the version, until the certification
updates `$CertifiedLinuxX64Sha512` and `$CertifiedMacArm64Sha512`. Once the certification commit
has updated those constants, and while the helper that certifies the new build is still not
installed, a run of the script carrying that commit reports `True; False; False; True; True`. Both
are the known window the certification plan describes (its Task 2 step 5 swap, closed by its
Task 3 step 4 install); check whether a certification task is open before acting on either. The
full set of five is what tells them apart from the shapes that ARE drift, read in the same order:

- `False; True; False; False; False` is the not-yet-swapped machine after Proton has moved (only
  the feed changed, and its `linux/x64` and `macos/arm64` entries moved with it).
- `False; True; True; *; *` is a re-cut of the same version number (same token on the machine,
  different bytes in the feed); the fourth and fifth fields say whether the Linux and macOS builds
  were re-cut too.
- `True; True; True; False; *` or `True; True; True; *; False` is a `linux/x64` or `macos/arm64`
  entry that differs from the recorded value while everything on the Windows side matches: a re-cut
  of that build under the same version number, or a release with no entry for it. A same-token
  re-cut is response-worthy: re-certify. The helper's token check alone would accept it, so these
  legs are the only thing that can show it.

**Certification touch points.** Every CLI certification also runs the 17-row live contract table
on linux/x64, in WSL with Windows interop off (`docs/research/gates/brief-checklist.md` rule 11),
as well as on Windows, and records the Linux SHA-512 from that run (D8(a) of
`docs/superpowers/specs/2026-10-15-v2-linux-port-design.md`). Every certification updates
`$CertifiedLinuxX64Sha512` in
`scripts/Test-ProtonCliDrift.ps1`, **and** the README's Linux CLI URL version and the SHA-512 beside
it. All are certification touch points, and the README-pin Pester guard
(`tests/ReadmePins.Tests.ps1`) checks that the README follows the helper's certified version and the
script's value. Record the new value the way the first was: copy it from Proton's live feed, then
require `sha512sum` of the Linux binary the Linux gate ran to equal it; if they differ, stop, because
Proton has re-cut the build since it was tested and recording either value would bless bytes nothing
tested.

**Certification touch points on the Mac.** Every certification also updates
`$CertifiedMacArm64Sha512` in `scripts/Test-ProtonCliDrift.ps1` and the README's macOS CLI line, and
adds offline checks on the Mac, with no account and networking denied (§7 of
`docs/superpowers/specs/2026-10-27-v2-macos-port-design.md`): download the new `macos/arm64`
build; check its SHA-512 against the feed; check its `--version` token; run it signed out through
the helper; record the SHA-512 for the README's macOS CLI line. Record the value the way the first
was: copy it from Proton's live feed, then require `shasum -a 512` of a fresh download of the
feed's URL to equal it; if they differ, stop, as for Linux. The helper refuses any CLI but the one
it was built to accept, so the signed-out run uses the release candidate that moves the pin: built
from source in a fresh clone of that commit, in a scratch folder outside every clone and never on
PATH, with the certification record naming its commit. It never uses `GPB_UNCERTIFIED_CLI=1`,
which would skip the very check the run exercises. Networking is denied with `sandbox-exec`; if a
later macOS removes it, the signed-out run is made with the Mac's networking switched off, and the
certification record says so. There is no signed-in Mac check at certification (O4(a)).

Stage 7's `--verify` command is where the scheduled form of this check belongs; until it exists,
this section is the control.

## When adoption may stand in for the live gate

A stand-in replaces step 5's live gates with installing the release on Craig's machine and checking
it there. It is Craig's ruling, made per release and recorded in that release's gate record; this
section says when he may make it and what the stand-in must then do. The 0.8.1 record
(`docs/research/gates/v0.8.1-release-gate.md`) is the worked example.

**Eligible only when nothing that builds or ships the helper has changed since the anchor,** the
most recent release whose own bytes passed the formal live gates (for 0.8.1, the anchor was v0.7.0).
All four must hold:

- `git diff <anchor-tag> <new-tag> -- '*.go' '*.s' '*.c' '*.h' go.mod go.sum go.work` is empty.
  That is every file the Go toolchain compiles, wherever it sits; add any file the helper embeds
  with `go:embed` (none today).
- `git diff <anchor-tag> <new-tag> -- .github/workflows/release.yml` is empty. The whole workflow,
  not chosen steps: its checkout, `setup-go`, Version, Build and Checksum steps all reach the
  shipped binaries, and its Create DRAFT release step sets `--draft` and the five-asset list. (A
  Dependabot bump to this file therefore means the next release takes the formal gates.)
- `go version -m` on each draft binary (the exe and the Linux binary) and on the anchor's published
  binary for the same platform reports the same Go version and the same build settings; on each
  draft binary, `vcs.revision` is the tagged commit and `vcs.modified=false`. `go.mod` declares a
  language floor, not a patch release, so the toolchain can move between builds; this check shows
  whether it did, and if it did, the release is not eligible.
- The installed Proton Drive CLI reports the certified build (`--version`), the one the anchor's
  Windows live gate ran against.

The anchor's and the draft's binaries, per platform, then differ by the embedded version string and
VCS stamps, and their behaviour is expected to be the same: an inference from identical source,
recipe and toolchain, not a byte comparison. Anything else takes the formal live gates. The anchor
never advances through a stand-in: it stays the last formally gated release, so a run of stand-ins
cannot drift away from live evidence.

**The first Linux release cannot take a stand-in.** It changes `release.yml` (the `release.yml`
diff bullet above) and has no Linux anchor: no earlier release's Linux bytes passed a Linux live
gate.

**What the stand-in covers live, and what it does not.** The module reaches Proton only through the
sync folder and the Proton Drive CLI, never through the helper, and the fleet verify (stand-in
step 6) exercises both against the real account. No push or fetch goes through the draft helper,
and the Linux binary is never run against the account at all (step 1's offline checks are all it
gets); that evidence comes from the anchor, and the gate record says so. The stand-in's only
writes to the account are the fleet verify's own: the bundles it cuts and the old bundles its
retention prunes, exactly as the daily run makes them; it adds no test writes of its own. Of
`brief-checklist.md`'s rules, rule 4 binds it (stop and report BLOCKED with verbatim output, never
patch), and rule 5 (`-count=1`) binds any `go test` it runs; the rest concern a live gate's own
test writes, listings and pushes, which a stand-in does not make.

**The stand-in, in order:**

1. **Offline checks on the draft's bytes.** Exactly five assets; each binary's SHA-256 equals its
   `.sha256` sidecar (both sidecars); `git-remote-proton.exe --version` prints the new tag and the
   certified CLI token; the Linux binary does the same when run in WSL (interop off, from an ext4
   copy, after `chmod 0755`); `install.ps1` equals the tagged blob apart from line endings; the
   `go version -m` comparison above, on both binaries. Stage all five digests for release step 7.
   The module ships through the repository, not as an asset, so also record its tree and file
   blobs at the tag.
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
