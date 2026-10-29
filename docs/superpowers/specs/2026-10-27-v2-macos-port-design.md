# git-remote-proton on macOS — design

**Date:** 2026-10-27. **Scope:** the v2 remote helper (`git-remote-proton`) only. The v1
`GitProtonBackup` module stays Windows-only: its transport is the Proton Drive Windows sync app and
the Windows Cloud Files API (README, "Why PowerShell?"). **Shape:** a stage spec; its plan is not
written yet. **Status:** Approved by Craig on 2026-10-27, after five review rounds ("I approve the
spec as it stands"). Craig ruled on MD1–MD8 on 2026-10-27, and
on the six questions review then raised, O1–O6, the same day ("Rulings", under Decisions); O1
amends MD6's wording. Nothing in this document is built. **Builds on:** `main` at `8848f41`, which merged
PR #15 (the three path tests that failed on macOS's `/var` → `/private/var` alias). **Template:**
`docs/superpowers/specs/2026-10-15-v2-linux-port-design.md`; its findings (F-numbers) and rulings
(D-numbers) are cited as they stand.

**Why now.** Craig, 2026-10-27: "Mainly my own Mac." A MacBook Air now sits beside the Windows PC,
and v1 cannot run on it, so the v2 helper is the only way this project can back a Mac up to
Proton. Other Mac users come second. The README roadmap calls macOS "an aspiration only, if
there's demonstrated interest"; this spec replaces that line. *Correction, 2026-10-27:* an earlier
draft said the Mac should back up "the way the PC does, including scheduled pushes". The PC's
scheduled backups are v1 bundles: no registered repository has a v2 remote and no scheduled job
pushes through the helper [record: `v0.10.0-windows-gate.md:127-128`; `v0.10.0-windows-gate-brief.md:799`].
So scheduled v2 pushes would be new on any machine. Craig ruled O2(b): the Mac gets them, one
LaunchAgent per repository, each repository pushed from one machine only.

**Basis marks.**

- **[mac]:** measured on Craig's MacBook Air (Apple Silicon, macOS 27.2 build 26B118, APFS
  case-insensitive, Apple git 2.54.0, go1.27.0) on 2026-10-23 or 2026-10-27, with no Proton account
  involved. Every run of Proton's CLI was under `sandbox-exec` with all networking denied.
- **[doc]:** read from the named source on the named date.
- **[code]:** read from this repo at `8848f41`, or from Go's own source as named.
- **[linux]:** a finding or ruling of the Linux port spec.
- **[record]:** a committed gate record or brief.
- **[Craig]:** Craig's own report or words.
- **[review]:** raised by the 2026-10-27 review (a builder check, and Codex, Gemini, DeepSeek and
  Kimi), checked against the repository or the data before it was accepted.
- **[inference]:** reasoned, not run. **[untested]:** nobody has run it.

## What the probes established

The probe scripts and raw logs were kept in the clone's untracked working folder; this table is
the record (AGENTS.md: `.superpowers/` is never a source of truth).

### Hermetic suite, 2026-10-23

| # | Finding | Basis |
|---|---|---|
| M0 | `go build`, `go vet` and `gofmt -l` are clean for darwin/arm64. The hermetic suite gave 522 pass / 3 fail / 5 skip on `ba1065b`; the three failures were one test-harness cause (path strings compared across the `/var` → `/private/var` alias), fixed by PR #15. On `8848f41` it gives **525 / 0 / 5**, the Linux baseline. The 5 skips are the four Windows-only long-path tests and the live half of the contract table. The whole suite takes about 17 s here, against about 590 s for `internal/repo` alone on Windows. | [mac] |
| M11 | Apple's git is 2.54.0 (Apple Git-157) and passes F3's one-call floor probe (exit 0, a `?<oid>` line). | [mac] |

### Proton's macOS CLI, 2026-10-23, no account

| # | Finding | Basis |
|---|---|---|
| M1 | Proton's `version.json` lists only 0.9.0 (2026-10-05). Its platform string is `macos/arm64` while its download path is `https://proton.me/download/drive/cli/0.9.0/darwin-arm64/proton-drive`. Its `linux/x64` SHA-512 still equals the certified value: no drift. | [mac] |
| M2 | The `macos/arm64` CLI is 77,282,784 bytes, a thin arm64 Mach-O, SHA-512 `3f4125de5e41dcc7f4e0e07e8d60737aa18f35c6ecc1ba3fce47ea97b331857eefaf88e74671135af57122d4cb5439b817ec528f8df383550ac7810aeaf0217b`, equal to the feed's. A `curl` download carries no quarantine attribute. | [mac] |
| M3 | It is signed `Developer ID Application: Proton AG (2SB5Z68H26)` with the hardened runtime, and notarized (`codesign --check-notarization -R=notarized` is satisfied; Homebrew's ad-hoc-signed `gh` fails the same check, so the check discriminates). | [mac] |
| M4 | `--version`, offline, prints `Proton Drive CLI cli-drive@0.9.0+75612299` / `Proton Drive SDK js@0.22.2+75612299`. **The exact pin needs no per-platform token for macos/arm64**, as F5 found for linux/x64. | [mac] |
| M5 | Its credential stores are `keychain` (default), `unsafe_file` and `pass` (the CLI's own message for an invalid `PROTON_DRIVE_CREDENTIALS_STORE`). Proton's README: macOS uses Keychain Services, service `ch.proton.drive/drive-sdk-cli`. The Keychain is per macOS user: `PROTON_DRIVE_CACHE_DIR` does not move it. | [mac]; [doc] Proton CLI README at `b61f987c2e3e`, read 2026-10-23 |
| M6 | Signed out, `filesystem info /my-files` fails offline: `You need to login first`, exit 1. Its log shows a Keychain lookup (`Loading session auth-session from secrets` → `No session loaded`) with no prompt, and no Keychain item exists before or after. Unlike F6 on Linux, the default store needs no setup for a logged-in user. | [mac] |
| M7 | State lives in `~/Library/Application Support/proton-drive-cli`, `~/Library/Caches/proton-drive-cli` and `~/Library/Logs/proton-drive-cli`. **`PROTON_DRIVE_CACHE_DIR` moves all three into one folder**, and nothing lands in the defaults (the part F9 left unverified on Linux). | [mac]; [doc] the same README |
| M8 | Through the helper, offline and signed out, `git ls-remote proton::…` passes the helper's certified-CLI check against the real macOS CLI and fails with the CLI's text intact: `git-remote-proton: stat …/gpb-remote.json: info …/gpb-remote.json failed: You need to login first: exit status 1`, git exit 128 (the macOS F7). A stand-in CLI printing `cli-drive@0.9.1+00000000` is refused at that check. `PROTON_DRIVE_CACHE_DIR` set for git reached the CLI the helper started (its log landed there), as F8's pass-through predicts. | [mac] |

### The helper's binaries, the sandbox, and Gatekeeper, 2026-10-23 and 2026-10-27

| # | Finding | Basis |
|---|---|---|
| M9 | A darwin/arm64 helper build is `adhoc,linker-signed` and not notarized; a darwin/amd64 build is not signed at all. Go's linker ad-hoc signs exactly when the target is darwin/arm64 (`NeedCodeSign`, `cmd/link/internal/ld/lib.go:302` in go1.27.0), whatever the build host. | [mac], [code] |
| M10 | Rosetta is not installed on this Mac (`arch -x86_64` → "Bad CPU type in executable"), so no Intel build can run here. | [mac] |
| M22 | `sandbox-exec -p '(version 1)(allow default)(deny network*)'` blocks a process's networking: `curl` to proton.me fails with "Could not resolve host" inside it and returns 200 outside it. It blocks nothing else: files and the Keychain stay reachable [inference from the profile]. Apple marks `sandbox-exec` deprecated [review; doc: `man sandbox-exec`]. | [mac] |
| M18 | A helper copy carrying a Safari-style `com.apple.quarantine` attribute is killed when git starts it: `error: git-remote-proton died of signal 9` / `fatal: remote helper 'proton' aborted session`, git exit 128, after 21 s; the CLI never starts. Whether a dialog appeared is unknown: none was on screen when Craig returned to the laptop later. | [mac], [Craig] |
| M19 | A quarantined copy of Proton's notarized CLI runs normally through an unquarantined helper (about 1 s, `You need to login first`). None was on screen afterwards either. | [mac], [Craig] |
| M20 | `xattr -d com.apple.quarantine` on a quarantined helper copy is enough: it then runs at once through git, with no dialog and no System Settings step. | [mac] |
| — | Neither `spctl --assess` nor `syspolicy_check distribution` predicts M18–M19: both reject both copies. A locally built binary carries no quarantine attribute (M8, M20). | [mac] |

### Ref names that differ only by Unicode normalisation, 2026-10-27

Local only, F13's method: a source repository holding both spellings (git's `reftable` format, so
the two cannot collide on disk), a bundle, clones on this Mac, with system and global git config
disabled. Names: `café` as NFC (`636166c3a9`) and NFD (`63616665cc81`), and the directory pair
`grp-é/a` (NFC) beside `grp-é/b` (NFD), each on its own commit.

| # | Finding | Basis |
|---|---|---|
| M12 | APFS is normalisation-insensitive and preserving: an NFD path reaches a file created under its NFC name, and writing both leaves one entry. | [mac] |
| M13 | Git on macOS sets `core.precomposeunicode=true` at `init` and `clone` and stores the NFC bytes for `git branch <NFD>`, so a Mac user cannot create an NFD ref by typing, a typed name resolves to the NFC ref, and a typed delete therefore cannot name an NFD ref either. Git on Linux and Windows does not normalise [inference], so a pair comes from those clients (including helper versions before this rule), from tools that pass raw bytes, or from a CLI or web upload. | [mac] |
| M14 | A clone of a remote holding a pair succeeds; both remote-tracking refs keep their distinct bytes and `git fsck` is clean. | [mac] |
| M15 | Local use then goes wrong with the default `files` ref store: a typed NFD name lands on the NFC branch, silently; exact NFD bytes put HEAD on the NFD name at the NFC branch's commit and files, silently (F13's `main`/`Main` shape); with refs packed first, the NFC local branch is silently repointed to the NFD branch's commit (worse than F13, where packing helped); a directory-prefix pair is silently re-normalised to NFC (F13's `grp/b` shape). Typing the NFC name of a branch the remote holds only as NFD fails loudly (`fatal: invalid reference`). | [mac] |
| M16 | **Incremental fetch goes wrong too.** The remote moves only the NFD branch: `git fetch` exits 0, quietly, and then the NFC tracking ref shows the NFD branch's new commit, the NFD tracking ref is stale, and `git fsck` reports the NFC branch's real commit dangling, so `git gc` can eventually drop it locally. When the NFC branch moves too, the fetch fails, exit 1, with a misleading `cannot lock ref … File exists … Another git process seems to be running`. The helper hands git objects and git writes the refs [inference from the remote-helper protocol], so fetch through the helper should behave the same [untested; gate step 5 tests it]. | [mac] |
| M17 | A `reftable` clone keeps both spellings right through clone, an exact-bytes switch and both fetches; typed names still resolve to NFC. Reftable needs git 2.45 or newer, the helper's existing fetch floor. Through the helper it is [untested; gate step 5 tests it]. | [mac]; [doc] git `Documentation/RelNotes/2.45.0`, read 2026-10-27 |
| M21 | A stdlib-only alternative rule, "refuse any name containing a combining mark (`unicode.Mn`)", refuses NFD `café` and NFD `が` but also ordinary NFC Hindi `हिंदी` and `हिन्दी` and Thai `อังกฤษ`. | [mac] |
| M23 | **Properties of Unicode simple case folding that the canonical key rests on**, measured on Unicode 17.0.0 (go1.27.0's `unicode` tables, cross-checked with Python's `unicodedata` 17.0.0): U+1FBE has the canonical decomposition U+03B9. Of 1,482 `unicode.SimpleFold` classes, exactly one mixes combining classes, `{U+0345 (ccc 240), U+0399, U+03B9, U+1FBE}`, and 240 is the highest combining class in use. Exactly three classes mix decomposable and plain members, `{K, k, U+212A}`, the iota class and `{Ω, ω, U+2126}`, and in each the decomposable member is a singleton that decomposes to another member of its own class. `caseKey`'s fold maps `ι`, `Ι` and U+1FBE to U+0345, a combining mark. | [mac] |
| M24 | **A delete push from a Mac `files` clone can rewind the surviving member's tracking ref.** A `files` clone of a local bare `reftable` remote holding `café` in NFC (commit A) and NFD (commit B), with `core.precomposeunicode=false`: when both tracking refs are packed (a fresh clone), `git push origin :refs/heads/<NFD bytes>` removes only the NFD tracking ref. When the NFC tracking ref is loose at C (a fetch moved it) over a packed A, the same push exits 0, prints `[deleted] café`, and leaves the NFC tracking ref at A, its stale packed value, while the remote holds C: git's post-push delete of the NFD name unlinked the NFC file. `git fsck` reports nothing. | [mac] |

### Git and Go behaviour, 2026-10-27

Local only, with a logging stand-in helper and scratch repositories, git 2.54.0 and go1.27.0.

| # | Finding | Basis |
|---|---|---|
| M25 | git starts a remote helper with `GIT_DIR=.git`, from the repository's top level, for `ls-remote` or `fetch` run at that top level **or in any subfolder of it**; it leaves `GIT_DIR` unset only when no repository encloses the working directory. The helper's first argument is the remote's name for a named remote (`lt`, `origin`) and the URL itself (`logtest::x`) for an anonymous one, for `fetch` and `ls-remote` alike. | [mac] |
| M26 | git refuses, before contacting the remote, a push that both deletes and writes one ref: `git push origin :refs/heads/main <oid>:refs/heads/main` fails with `dst ref refs/heads/main receives from more than one src`, exit 1. So a same-batch replacement of one name reaches `Push` only from a direct caller, such as a test. | [mac] |
| M27 | `go version -m` reports `vcs.modified=true` for a build made in a clone holding one untracked file that is not ignored, and `vcs.modified=false` for one holding only ignored files (an untracked `.superpowers/` file). | [mac] |
| M28 | **At a clone's `list`**, git has already created the new repository: the helper starts in the parent folder with the remote's name (`origin`) as its first argument and `GIT_DIR` set to the new repository's `.git`, whose format already reads `files` (default) or `reftable` (`--ref-format=reftable`). `remote.origin.url` is written; in a plain clone **`remote.origin.fetch` is not yet**, though a clone given `-c remote.origin.fetch=<spec>` has it set at `list` (MA6) [measured during the plan's Task 8 review, 2026-10-27], and the repository holds no refs. `option cloning true` comes only after `list`. | [mac] |
| M29 | Go's `%q` leaves a combining mark unescaped, so NFC `refs/heads/café` and NFD `refs/heads/café` print identically; `%+q` (`strconv.QuoteToASCII`) prints `"refs/heads/caf\u00e9"` and `"refs/heads/cafe\u0301"`. | [mac] |

### Facts the design rests on, read rather than measured

| Fact | Basis |
|---|---|
| Go's standard `unicode` package is Unicode **15.0.0** on Go 1.22 through 1.26 and **17.0.0** on Go 1.27. | [doc] `src/unicode/tables.go` on Go's `release-branch.go1.22`, `.go1.24`, `.go1.25`, `.go1.26`, read 2026-10-27; [code] go1.27.0 |
| Release binaries are built with go.mod's toolchain: `release.yml` runs `actions/setup-go` with `go-version-file: go.mod` (`go 1.22`). Its Linux build sets `CGO_ENABLED=0`; its Windows build does not. | [code] `.github/workflows/release.yml:15-19, 57, 64` |
| `golang.org/x/text`'s latest, v0.43.0 (2026-10-09), declares `go 1.26.0`; the newest declaring 1.22 or lower is v0.22.0 (2025-02-04, `go 1.18`). | [doc] proxy.golang.org module files, read 2026-10-27 |
| `.github/dependabot.yml` watches GitHub Actions only, with the comment "The module itself has zero dependencies". | [code] |
| The repository is public; GitHub's standard runners are "free and unlimited on public repositories"; `macos-latest` is an arm64 (M1) runner today. | [doc] `gh repo view`; docs.github.com runner reference and Actions billing, read 2026-10-27 |
| A stand-in may replace a release's live gates only when no compiled file, `go.mod`, `release.yml` or toolchain changed since the anchor. | [doc] `docs/releasing.md`, "When adoption may stand in for the live gate" |
| The hermetic suite prepends its shim directory to PATH (`shim_test.go:110`), so a real `proton-drive` later on PATH stays resolvable behind it. | [code] |
| The live contract table writes under `/my-files/_cas-probe/contract` unless `GPB_CONTRACT_LIVE_ROOT` names another root. | [code] `internal/transport/contract_test.go:21, 25` |
| The Windows PC uses the CLI only to read (the v1 module's verify); nothing there pushes through the helper. | [record] `v0.10.0-windows-gate-brief.md:799-804` |
| launchd misses a `StartInterval` firing that falls while the system sleeps, and starts a `StartCalendarInterval` job on the next wake, coalescing the missed ones. | [doc] `man launchd.plist` on this Mac, read 2026-10-27 |
| `caffeinate -i` prevents idle sleep; `caffeinate -s` prevents system sleep, and only on AC power. | [doc] `man caffeinate` on this Mac, read 2026-10-27 |
| The Apple Developer Program costs 99 USD a year. | [doc] developer.apple.com/support/enrollment, read 2026-10-27 (relevant only to the option MD2 did not take) |

## Goals, non-goals, anti-goals

**Goals.**

- **Craig's Mac backs up repos through the v2 helper**, built from source at a release tag (MD2(d)),
  attended and by per-repository LaunchAgents (O2(b)), with the steps written in the README and
  exercised by the gate.
- **CI runs the hermetic suite on macOS**, so a macOS-only break, like PR #15's, cannot reach
  `main` unnoticed (MD3).
- **Repos stay usable from every machine** for every rule the helper can enforce: a push from any
  platform refuses to create a ref that a Mac clone would confuse with another by Unicode
  normalisation, as D2 already refuses case variants, which Windows and Mac clones both mishandle
  (MD6). The rule gives the same answer on every supported build (O1(iv)).
- **A Mac never writes refs it would confuse** (O3(d)): on a Mac with the default ref store the
  helper refuses a fetch or clone that would, and refuses a push write or delete that would, and
  on every platform it warns when a remote holds a pair.
- **The README states the macOS position honestly:** what was gated and when, what is only built
  and CI-tested since, and what a Mac does with a remote that already holds a pair.

**Non-goals.**

- v1 on macOS. Its transport is Windows-only, and Proton's macOS sync app is a different
  mechanism.
- Intel Macs (MD1, MD8). The helper still builds for darwin/amd64; nothing says it works.
- A macOS release asset, an installer, a Homebrew tap, signing or notarization (MD2).
- A backup scheduler for the Mac beyond the README's per-repository LaunchAgent (O2(b) took that,
  and left a Mac counterpart of the PC's scheduled tooling out).
- Two machines pushing one repository to one remote. The README says each repository is pushed
  from one machine only (O2(b)), because a forced or `--mirror` push from either would rewind or
  delete the other's refs.
- The helper handling credentials or the Keychain, as on Linux.

**Anti-goals** (failure even if it works):

- Shipping any macOS binary. Under MD2(d) the project ships source for macOS; a binary in a
  release would be ungated bytes.
- A change that weakens Windows or Linux behaviour, **including any pair the shipped case rule
  refuses today that the widened rule would let through**, or that lets any push create a ref pair
  that a Mac clone mishandles silently (M15, M16), beyond D2's two stated exceptions, which carry
  over: a variant sitting in a skipped file, and the rename residual after a phase-4 delete
  failure.
- A hermetic run on a Mac that can reach the account, or the Mac's Keychain session, through a
  real `proton-drive` (§5).
- A README claim about macOS that no probe, gate or cited source backs, unless marked "not gated".

## Design

### 1. Build and install from source (MD2(d))

No release asset and no change to `release.yml` or the five-asset list in `docs/releasing.md`
step 4. The README gains an "Install on macOS (v2)" section beside "Install on Linux (v2)":

- **Prerequisites:** Apple Silicon; git 2.45.0 or newer (Apple's command-line tools git 2.54.0
  passes, M11); a current Go release, tested on the Mac with go1.27.0 and in CI at `stable`
  (go.mod's 1.22 floor is tested on Linux only, so the README does not claim it for macOS)
  [review: DeepSeek]; the certified Proton CLI; a logged-in macOS user, whose login keychain the
  CLI uses by default (M5, M6).
- **The helper:** in a fresh clone at the release tag (`git clone --branch <tag>`), build with
  `CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=<tag>" -o
  <bin>/git-remote-proton ./cmd/git-remote-proton` (the same recipe as `release.yml`'s Linux
  build), and confirm `--version` prints the tag and the certified token. A
  locally built binary is not quarantined and is ad-hoc signed by Go's linker, so it runs (M8, M9,
  M20). Building from the tag is the only documented way to get the helper on a Mac.
- **The CLI:** `curl` from the versioned `darwin-arm64` URL (M1), checked with `shasum -a 512 -c`
  against the recorded SHA-512 (M2) and installed only if the check passes, chained with `&&` as
  Amendment A3 did for Linux.
- **Where they go:** a user-owned directory on PATH. `~/.local/bin` is not on a Mac's default PATH,
  so the README gives the `~/.zprofile` line for it *(my call; a plan may choose another
  directory)*. Then `type -a git-remote-proton` and `type -a proton-drive` in a new login shell
  must each show exactly one path, the shadow check `install.ps1` and the Linux section make.
- **A helper binary that arrived some other way.** No macOS binary is published. A helper binary
  copied from another Mac by browser, mail, AirDrop or an archive can carry the quarantine
  attribute, and git then reports `died of signal 9` / `remote helper 'proton' aborted session`
  (M18). The README says so, tells the reader to confirm the attribute first (`xattr -p
  com.apple.quarantine <file>`) rather than assume it, and recommends rebuilding from the tag over
  removing the attribute from a file of unknown origin; `xattr -d` is given as what it is, the step
  that made a known-good copy run (M20). Proton's notarized CLI does not need it (M19).
- **Status line, written in two passes** [review: Kimi]. The README the port's release tags says
  macOS is "built from source; the hermetic suite runs on it in CI; not yet live-gated". Only after
  the Mac gate passes do the docs say "live-gated once, at <version>; later releases' Mac builds
  and later CLI versions on the Mac are not live-gated", as v0.10.0's README was updated after its
  Linux gate. It is not "supported" in the sense Windows and Linux are, whose release bytes are
  gated every release. O4(a) kept MD7 as ruled, so the last clause stands.

### 2. CI (MD3(a))

`ci.yml`'s `go` matrix gains `{ os: macos-latest, go-version: stable }`. Its steps run under
`bash`: build, vet, the git-floor probe, `go test -count=1 ./...`, and gofmt, as the Linux legs do.

- **Every one of those steps must run on the macOS leg.** Each step in today's `go` job is keyed on
  `runner.os == 'Windows'` or `'Linux'`, so a matrix entry alone would run only checkout and
  `setup-go` and report green [code: `ci.yml`; review: Kimi]. Whether the plan widens the Linux
  steps to non-Windows or adds macOS twins is *left to the builder*; that the leg's test step ran
  and passed is the requirement. No `go-version-file` leg on macOS: the floor toolchain is already
  tested on Linux, and what macOS adds is OS behaviour. The leg is the Go suite only: the Pester
  suite, PSScriptAnalyzer and the drift script stay on the Windows `test` job, where the drift
  script runs in practice (on the PC) [review: Kimi].
- The leg asserts `uname -m` is `arm64` and fails otherwise, so a future move of the
  `macos-latest` alias cannot silently test darwin/amd64, which MD1 puts out of scope.
- The probe step's fallback on macOS is `brew install git` with `HOMEBREW_NO_AUTO_UPDATE=1`. A
  `$GITHUB_PATH` entry only reaches later steps, so the step also exports
  `PATH="/opt/homebrew/bin:$PATH"` for itself, re-probes, logs `command -v git` and `git
  --version`, and fails if the git it found still misses the floor; it then adds `/opt/homebrew/bin`
  to `$GITHUB_PATH` for the steps after it [review: Gemini, DeepSeek]. The runner's git version is
  [untested] today.
- A step regenerates the decomposition and case-folding tables (O1(iv)) from the **committed**
  copies of the pinned Unicode data files, with no network, and fails if `git diff
  --exit-code` shows a difference, so the committed tables cannot drift from their generator. It
  runs on one leg only *(my call: Linux)* [review: Gemini, DeepSeek].

### 3. The canonical-equivalence rule (MD6(a2))

**The rule.** D2's create refusal widens. On every platform, two prefix spellings conflict when
they differ as bytes **and either** relation holds:

1. **Case** (D2's relation): they share `caseKey`, Unicode simple case folding
   (`internal/repo/casefold.go:27`). Every pair refused today stays refused ("No refusal is lost",
   below).
2. **Canonical caseless** (new): they share `canonKey(p) = caseKey(nfd(p))`, canonical
   decomposition and then the same simple folding.

Both relations are kept because neither contains the other [review: Codex, verified]. `a ι ´`
(U+0061 U+03B9 U+0301) and `a U+0345 ´` share `caseKey` and are refused today (they differ only
within the iota fold class), but decomposition reorders the second's marks to `a ´ U+0345`, so
their `canonKey`s differ; dropping relation 1 would let that pair through, which the anti-goals
forbid. Conversely an NFC/NFD pair shares only `canonKey`. The conflict check is pairwise against
the set, as `caseConflicts` already is, so two relations need no transitive closure: `a ι ´` and
`a ´ ι` (U+0061 U+0301 U+03B9) share neither key and are not refused against each other, though
each conflicts with `a U+0345 ´` by one relation. A pair that matches both relations is reported
once, as a case match. `caseConflicts` records one pair per conflicting ref, the shallowest; with
two relations it records the shallowest pair under either, and at equal depth the case match, so
the message a test asserts is fixed [review: DeepSeek; my call].

**The folding representative, and why `canonKey` needs no second decomposition** (M23). Today's
`caseKey` represents each fold class by its smallest member, and for the iota class that is
U+0345, a combining mark (M23). Under O1(iv) the generated table maps each character to its simple
case-folding target in `CaseFolding.txt` (statuses C and S), the representative Unicode itself
uses, so the iota class folds to U+03B9, a starter *(my call: it changes no class, so neither
relation's answers change)* [review: DeepSeek]. Unicode's canonical caseless match decomposes
again after folding, and with that folding the last pass changes nothing on decomposed input: the
only fold class that mixes combining classes has its combining member at the highest class in use,
so after decomposition that member always ends its run of marks, and folding it to a starter never
calls for reordering; and the only decomposable members of mixed classes are singletons that
decompose into their own class, so folding never brings a decomposable character back [inference
from M23's measured properties]. So `canonKey` is Unicode's canonical caseless match with simple
folding. Had the smallest-member representative been kept, a second pass would have been wrong:
it would reorder U+0345 among accents and merge `a ι ´` with `a ´ ι`. The generator re-checks
both properties for the pinned version and fails if either stops holding.

**`nfd`** is full canonical decomposition: the recursive canonical mappings, Hangul syllables by
the standard algorithm, and canonical reordering, a stable sort by combining class within each
run of non-starters, starters never crossing. It copies invalid UTF-8 bytes through unchanged, as
`caseKey` does, so distinct invalid bytes never compare equal, and an invalid byte counts as a
starter, so no mark is reordered across it; a test vector pins that [review: DeepSeek; my call].
`CaseFolding.txt` never gives one code point both a C and an S entry, so simple folding needs no
precedence between them; the generator asserts it [review: DeepSeek].

**Every place the case key decides equivalence takes both relations** [code; review: DeepSeek,
verified]: the `caseConflicts` index (`casefold.go`, called at `push.go:383` and `:623`) and Push's
rename pairing (`renameTo[caseKey(…)]`, `push.go:403-404`, `:422`), so a composition-changing rename
is refused whole exactly as a case-changing one is (Amendment A2). A same-batch delete is refused
when it pairs with a refused create under either relation, and its message names the smallest
such create, as today's does (`TestPushRenameDeleteNamesTheSmallestRefusedCreate`), so the result
does not depend on which key matched [review: Gemini]. Two message texts change, each naming the
relation and showing both spellings in their escape form: `caseConflictMessage` (`casefold.go:137`,
"conflicts by letter case … a Windows clone would mishandle the pair"), which phase 2's create
refusal and phase 5's re-check (`push.go:630`) share, and phase 2's rename-delete message at
`push.go:423-424`, "the refused case-changing rename to %s". Phase 5's `failedDeleteClause`
(`casefold.go:154`) names no relation and is appended unchanged [code; review: DeepSeek, who named
the clause, checked]. `canonKey` is a new function beside `caseKey`. `caseKey`'s
contract does change under O1(iv): it folds by the pinned table, so its doc comment ("two strings
share a key exactly when `strings.EqualFold` calls them equal") becomes "exactly when Unicode
simple case folding at the pinned version calls them equal" [review: DeepSeek, verified:
`casefold.go:17-27`]. Everything else about D2 stands unchanged: prefixes and the depth argument,
create-only (Amendment A1), phase 2's fixed point, phase 5's re-check before `ensureRefParents`,
the uncapped conflict list, skipped refs out of the set, and no override. `refPrefixes` relies on
`/` having no case variant so that prefixes match only at equal depth; no canonical decomposition
produces or consumes `/` [inference], and the generator asserts it.

**The tables (O1(iv)).** No dependency and no change to the `go 1.22` floor. One pinned Unicode
version, 17.0.0 today, supplies both the decompositions and the simple case folding, in tables
generated into the repository, so every build, whatever Go compiled it, refuses the same set.
`caseKey` therefore takes its folding from the generated table instead of `unicode.SimpleFold`.
Moving the pin is a deliberate change with its own review, never a side effect of a Go upgrade.

- A generator kept out of the build (`//go:build ignore`), run by a `//go:generate` directive that
  lives in a compiled file, reads Unicode's `UnicodeData.txt` (decompositions and combining
  classes) and `CaseFolding.txt` (simple folding, statuses C and S), and writes Go source tables
  that are committed. The generated file records the version and the input checksums.
- **The inputs are committed** [review: Gemini, DeepSeek]. The pinned versions of
  `UnicodeData.txt`, `CaseFolding.txt` and `NormalizationTest.txt` live in the repository, each
  checked against a SHA-256 the generator records, so generating and testing need no network.
  Fetching from `unicode.org` happens only when the pin moves, by a separate manual command.
  Unicode publishes no checksums for these files (2026-10-27: none in the 17.0.0 `ucd/` listing or
  either ReadMe), so the command checks the version line `CaseFolding.txt` and
  `NormalizationTest.txt` each open with and prints the SHA-256s it saw. The data is checked from
  two sides: `NormalizationTest.txt`'s Parts 1 and 2 exercise every decomposition and combining
  class `UnicodeData.txt` supplies, and the containment test on a toolchain at the pinned version
  checks the folding. `UnicodeData.txt` has no version line of its own; `NormalizationTest.txt` is
  its only independent check (Amendment MA2). `.gitattributes` marks those files
  `-text`, so every checkout has their committed bytes exactly, which the recorded SHA-256s require
  and which a Windows line-ending conversion would otherwise break [code: today's `.gitattributes`
  pins only `*.go`, `go.mod` and `go.sum`; review: DeepSeek, Kimi].
- Hermetic tests, no network: every line of Unicode's `NormalizationTest.txt` at the same version
  decomposes to its NFD column, and the canonically equivalent strings on each line share
  `canonKey`; case variants built from `CaseFolding.txt` share both keys where they should; a
  negative test over non-equivalent lines finds no false `canonKey` collisions; the two iota
  examples above give exactly the results stated; and `casefold_test.go`'s existing comparison
  cases keep their results. Tests that assert today's message text
  (`TestCaseConflictMessageNamesEachRefWithItsOwnPair`, and `repo_test.go`'s rename-delete message
  tests) change with the messages [review: Gemini]. The vectors are the committed input files
  above (`NormalizationTest.txt` is a few MB); where in the repository they sit is *left to the
  builder*.

**No refusal is lost.** Moving `caseKey` onto the pinned table must not let through any pair a
release build refuses today [review: DeepSeek]. Unicode's stability policy fixes the canonical
decompositions of assigned characters from Unicode 4.1 ("toNFD<sub>V</sub>(S) = toNFD<sub>U</sub>(S)")
and caseless matching from 5.2, the latter stated formally for full case folding of NFKC strings
[doc: unicode.org/policies/stability_policy.html, read 2026-10-27]; that simple folding classes of
assigned characters only ever gain new characters is [inference] from it. So a test checks the
property directly on every toolchain CI runs: every `unicode.SimpleFold` orbit of the running
toolchain, restricted to characters assigned in the pinned version, lies inside one generated
class, and where the toolchain's Unicode version equals the pin the classes are equal. On the
`go-version-file` leg (Go 1.22, Unicode 15.0.0) that proves every refusal a release build makes
today is kept. Where no toolchain at the pinned version runs, the generated classes rest on the
committed `CaseFolding.txt` and the generator alone [review: Kimi]. On the `stable` leg the test
can go red with no code change, if a later Unicode version ever merges two fold classes; that would
disprove the [inference] above, so it is investigated as a finding, never silenced, and the pin
does not move to clear it [review: Kimi].

**What the user sees.** git's per-ref output reports the ref rejected with a message naming every
conflicting ref and, for each, the two spellings and which relation matched. The harm it names
follows from the filesystems [review: Gemini, checked against M12 and the basis marks]: a case
match harms Windows and Mac clones (NTFS and default APFS are both case-insensitive); a
canonical-only match harms Mac clones (APFS is normalisation-insensitive, NTFS is not); a pair
matching both is reported as a case match. Because NFC and NFD spellings display identically, each
spelling is also shown in Go's ASCII-only quoted form, `%+q` (`strconv.QuoteToASCII`), which
escapes every non-ASCII character; plain `%q` would not do, since it leaves combining marks raw
(M29) [review: DeepSeek]. The form contains no whitespace and so survives `push.go`'s `oneLine`,
and a hermetic test asserts that the two spellings of an NFC/NFD pair render differently *(my
call; exact wording left to the builder)*.

**What the rule costs.** It refuses some names a given filesystem would keep apart: canonical
caseless equivalence is a Unicode definition, not a model of APFS or NTFS, so a pair such as `ᾳ`
beside `αι` (equivalent under it, because Unicode folds U+0345 to `ι`) is refused though no
measurement shows a Mac confusing them [inference]. The README's honest limits name this.

**What no rule here catches at first: a lone NFD name.** Every check in this section compares two
refs. A remote holding `café` only in NFD, pushed from Linux, Windows or a tool, has no pair, so it
is pushed, scanned and cloned without a word, and a Mac user then cannot name it by typing: the
typed NFC spelling fails with `fatal: invalid reference` (M15). At git's Mac default,
`core.precomposeunicode=true`, the branch's first incremental fetch writes a loose NFD file that git
itself then lists precomposed, as a phantom NFC tracking ref beside the stale packed NFD one, and the
next fetch fails in git (`cannot lock ref`). The helper refuses that next fetch first, under rule
(2), because `LocalRefs` reads git's own view (MA4) [measured 2026-10-28, git 2.54.0:
`docs/research/probes/macos-port/lone-nfd-default/`; the helper's half pinned by
`TestLoop_ListAtTheMacDefaultRefusesALoneNFDBranchAfterItsFirstFetch`]. Nothing is lost (`git fsck`
exits 0), so it is left to the README's honest limits, with a `reftable` clone as the remedy and
the exact-bytes form (`core.precomposeunicode=false`, the name given as bytes) as the way to reach
the name [review: Gemini; my call].

**Existing pairs: what a read does (O3(d)).** As under D2, pairs already on a remote stay: pushes
made before this release or from older helper versions, CLI or web uploads, and repaired skipped
files can all lay one down. The create refusal cannot stop those, and M16 shows a Mac fetch of one
corrupting tracking refs silently. So reads are checked too, with both relations and the same
prefix comparison the create rule uses (`refPrefixes`), so a directory pair such as `grp-é/a`
beside `grp-é/b` counts (M15) [review: Gemini].

Where the checks can sit was measured [mac: git 2.54.0, 2026-10-27, a logging stand-in remote
helper on local repositories; probe kept with the others]:

- for `git fetch`, `git ls-remote` (inside a repository or outside one) and `git clone`, git sends
  `capabilities`, `option progress`, `option verbosity` and then `list`, the same in every case;
  `option followtags` (fetch) and `option cloning` (clone) come only after `list`;
- **when every advertised tip is already local, git sends no `fetch` command and still writes the
  tracking ref**, the bypass `check_exist_and_connected` performs [doc: git v2.54.0
  `builtin/fetch.c`; review: Codex, Gemini]. The round-2 draft refused at the `fetch` command, which
  a same-commit rename would never reach.

So `list` is the one point every ref-writing read passes through, and it cannot tell a fetch from
an `ls-remote`. The checks live in `cmd/git-remote-proton/main.go`'s protocol arms, which hold the
advertised set, `gitDir` and stderr; the comparisons are pure functions beside `caseConflicts`
[code: `main.go:373`, `:457`, `:527`].

- **The warning, every platform, at `list` and `list for-push`:** when the advertised refs hold a
  pair under either relation, the helper writes one warning to stderr. It names each pair once, as
  an unordered pair sorted by its spellings, with its relation, both object IDs and both escape
  forms, and the README section. It is written from the protocol arm only, never from the second
  ref scan a push makes later (`main.go:578`). Off a Mac it never blocks anything; on a Mac with the
  `files` ref store it comes with the refusal below.
- **The Mac refusal, at `list`**, on darwin, when git started the helper with `GIT_DIR` set and
  that repository's ref store is `files`. Whether `GIT_DIR` was set is read at startup, before
  `resolveGitDir` (`main.go:268-278`) defaults an unset one to the working directory; git sets it
  for a command run anywhere inside a repository, subfolders included, and leaves it unset only
  where no repository encloses the working directory (M25) [review: Gemini]. The format comes from
  `git -C <gitDir> rev-parse --show-ref-format`, which prints `files` or `reftable` [mac: git
  2.54.0]; `git-rev-parse` documents it at v2.44.0 and v2.45.0, so the 2.45 fetch floor has it
  [doc: git's `Documentation/git-rev-parse.txt` at both tags, read 2026-10-27; run on 2.54.0
  only] [review: DeepSeek]. The helper answers `list` with an error and advertises nothing, so
  git writes no ref, when either
  1. the advertised refs hold a pair under either relation, or
  2. the refs the fetch could write (its destinations), taken together with the refs the local
     repository already holds, contain a pair under either relation with a destination as one of
     its members: a destination against a local ref spelled differently, or two destinations
     against each other, prefixes compared as everywhere else [review: Codex]. A pair among local
     refs that no destination touches is not the fetch's to judge. The destinations are each
     advertised ref mapped through the remote's configured fetch refspecs (`remote.<name>.fetch`,
     positive and negative, with git's matching). `remote.<name>.mirror` is not read: git uses it
     for push only, and a mirror fetch is configured as the refspec `+refs/*:refs/*` [doc: `git
     help config`, git 2.54.0, read 2026-10-27] (Amendment MA1). The destinations also include
     every advertised `refs/tags/*` ref, which git can write by auto-following [review: Gemini, DeepSeek]. git
     passes the remote's name as the helper's first argument (M25), and `run()` threads it into
     `loop()`. For an anonymous URL that argument is the URL itself and no remote is configured,
     so the destinations are the advertised tags alone [review: Gemini; whether git auto-follows
     tags for an anonymous fetch is not settled here, so counting them is the safe side]. For a
     clone the remote is configured but its fetch refspec is not yet written (M28), so the
     destinations are git's default mapping for that name, `refs/heads/*` to
     `refs/remotes/<name>/*`, plus tags [review: DeepSeek; my call]. A clone's mappings change
     names only by a fixed prefix, so rule (1) already catches what this would [inference]; the
     check runs anyway. That
     catches a remote that renamed `café` from NFC to NFD in two separate pushes: it never
     advertises a pair, but an unpruned Mac clone keeps the old packed tracking ref [review: Codex;
     gate step 5.8 measures it]. It also catches two remote branches whose configured mappings land
     in aliasing directories, such as `refs/remotes/r/café/a` and `refs/remotes/r/café/b` with the
     two `café` spellings, when no local ref exists yet.

  `git ls-remote` run inside such a repository is refused like a fetch, since nothing before
  `list` tells them apart. With `GIT_DIR` unset there are no local refs to protect, so `list` only
  warns; the pre-adoption scan and gate step 5.7 therefore run from a folder with no repository
  above it (for example `cd "$(mktemp -d)"`), because inside any repository's tree git sets
  `GIT_DIR` and a `files` repository refuses (M25). A clone is covered too: when git sends its
  `list`, `GIT_DIR` already names the new repository and its format already reads `files` (M28),
  so a clone into the default ref store is refused before any ref is written [mac: stand-in
  helper; gate step 5.5 checks it with the real one] [review: DeepSeek]. A `reftable` repository
  gets the warning only.

  **Its local reads fail closed, and what they cost** [review: Kimi]. On darwin with `GIT_DIR` set,
  `list` makes three local reads that `list` makes nowhere today (`main.go:373` spawns no git):
  the ref format, the remote's refspec configuration, and the local ref names. If any of them
  fails, the helper refuses `list`, naming the read that failed, as its existing reads fail closed
  on content they cannot read; a refusal there is a stop, never a silent skip. The cost is those
  few git subprocesses on every darwin `list` inside a repository, fetch and `ls-remote` alike,
  and none anywhere else: the warning works from the advertised set alone. A remote whose name
  collides with one of `dispatchUtility`'s flags (`--version`, `--set-head`) never reaches the
  protocol loop, today or after this change [code: `main.go`; review: Kimi].

  **What rule (2) cannot see:** a refspec given on the command line, or `--refmap`, never reaches
  the helper, so a fetch that maps refs into another namespace that way is outside the check
  [review: Codex, Gemini]. The README says: on a Mac with the `files` store, fetch through the
  remote's configured refspecs, or use a `reftable` repository.

  **The refusal's remedies follow its cause** [review: DeepSeek]: for a pair on the remote (1),
  repair the remote (below) or clone with `--ref-format=reftable` (M17); for a stale local ref
  (2), re-clone with `--ref-format=reftable`; or, in place, pack every ref with precomposition off
  and only then delete the stale ref by its exact bytes (`git -c core.precomposeunicode=false
  pack-refs --all`, then `git update-ref --stdin` reading `delete <ref>`). A bare delete is not
  safe: with the other spelling's ref loose, APFS resolves both names to that file, git unlinks
  it, and that ref falls back to a stale packed value or vanishes (MA3); for two destinations
  aliasing each other (2), change the remote's refspecs or use a `reftable` clone.
- **The Mac push rule, in `repo.Push`'s phase 2**, on darwin with the `files` store (as at `list`,
  with `GIT_DIR` set, which git always sets for a push). It refuses per ref, each message passing
  through `oneLine`, and every ref it does not refuse proceeds, so D2's ruling that a remote
  holding a pair still takes pushes to its other refs stands [review: Codex]. Below, the tracking
  ref of a pushed name is where the remote's fetch refspecs map it, the ref git updates or
  deletes locally after a successful push; an anonymous URL has none.
  1. **A write** (create or update) of X is refused when its tracking ref conflicts under either
     relation with a local ref spelled differently, or with the tracking ref of another write in
     the batch, the check `list` makes between destinations [review: Codex, Gemini]. An **update**
     of X is also refused when X conflicts under either relation with another ref of
     `batchFinalSet`, the remote's set as the batch would leave it, so a same-batch delete of the
     other member clears this half; a create's remote side is D2's to judge [review: Gemini]. The
     local half judges against the local refs as they stand before the push, because git rewrites
     tracking refs only afterwards, so a same-batch delete never clears it: on a clone holding
     either member's tracking ref the update is refused even when the batch deletes the other
     member, and that delete is judged by part 3 alone [review: Gemini].
  2. **A delete paired with a write this rule refuses is refused with it,** naming that write.
     First by exact name: the delete half of a replacement of X itself, so a replacement can never
     commit its delete while its write is refused (git never sends one, M26; a direct caller of
     `Push` can, so hermetic tests cover it). Then, only when the refused write is a create, the
     delete half of a rename to X: a delete of a name the pre-batch remote holds that matches X
     under either relation, as A1 pairs deletes with D2's refused creates. A refused update has no
     rename half, so a same-batch delete of the other member, which is the repair, still goes
     through [review: Gemini].
  3. **A delete** of Y is refused when Y's tracking ref conflicts under either relation with a
     local ref spelled differently: git's post-push delete of Y's tracking ref can unlink that
     ref's loose file, and the ref then silently falls back to a stale packed value (M24) [review:
     Codex, Gemini]. Round 3's "a delete is never refused" is withdrawn: the repair it protected
     runs from a `reftable` clone, which this rule does not reach. Deletes that cannot harm the
     local repository, which is every delete off a Mac, in a `reftable` clone, or with no aliasing
     local ref, are never refused, so the repair stays possible.

  **Where it sits in phase 2** [review: DeepSeek, Gemini]. Around the existing fixed point, not
  inside one of its passes: D2's widened create check and A1's pairing first run to their own
  fixed point, exactly as today; then this rule's three parts run once over what is still valid;
  if they refused anything, the whole sequence repeats, because a delete this rule keeps leaves
  its ref in `batchFinalSet` and can expose a conflict D2 must judge. So D2 always speaks first:
  an update D2 would refuse in the current state is refused by D2, with D2's message, before this
  rule sees it, and `TestPushRenameRefusalsReachAFixedPoint`'s two-pass case keeps its result
  (interleaving the two rules pass by pass would let this rule take that case's second refusal).
  Each outer round that continues invalidates at least one entry, possibly only a delete that part
  3 refuses, so there are at most (entries + 1) rounds (MA5), and every step is set algebra, so neither outcome nor text depends on batch order. A
  refused update is invalid for every later check and gets exactly one message; this rule's
  refusals never enter A1's `renameTo`, so a delete is paired once, by the rule that refused its
  write.

  **Its interface** [review: DeepSeek, Gemini; my call]. `Push` (`push.go:61`) receives the remote
  root, `gitDir`, the updates, the advertised map and the skipped refs, and reads no local refs or
  remote configuration. So `main.go`'s push arm, which holds the remote's name, makes the local
  reads `list` makes (format, refspecs, local ref names; failing closed the same way, refusing
  every update in the batch and naming the read) and passes `Push` one new argument: the local
  ref names and the tracking-ref mapping, or nil off darwin, for a `reftable` repository, and in
  tests that do not exercise the rule. Phase 2 applies the rule from that data and runs no git
  itself.
- Case pairs count as well as composition pairs throughout, because default APFS is
  case-insensitive (§3, "What the user sees"). *(The trigger points, the refspec mapping, the
  push rule's three parts and its interface, and the Mac-only scope are my calls under O3(d); a
  plan may refine them with a reason.)*
- **Writers first (O3(b)):** before the Mac adopts a remote, every machine that pushes to it runs a
  release carrying this rule, because an older helper can still write a pair. Today no other
  machine pushes through the helper (Facts), and O2(b) keeps each repository to one machine, so
  this binds any second pusher added later; the README says so [review: DeepSeek].
- **The pre-adoption scan (O3(a)):** before the Mac adopts a remote, `git ls-remote
  proton::<address>` it once with the new helper, from a folder with no repository above it (M25);
  the warning above is the scan's report, and the helper's existing fail-closed reads cover
  anything it cannot read. The list of remotes is the repositories Craig gives LaunchAgents
  (O2(b)).

**Repair.** The README's honest limits list the sources of existing pairs, what a Mac clone does
with one (M15, M16), the refusals above, and a repair a Mac can actually perform [review: Codex,
Gemini, DeepSeek]:

1. record both members' names and object IDs, which the warning prints;
2. make a temporary `--ref-format=reftable` clone, which the refusal allows and APFS cannot alias;
3. from it, delete one member **by its exact bytes** (`core.precomposeunicode=false`, the name
   given as bytes, since a typed name precomposes, M13), or delete it from a Linux client, or, for
   a pair that differs by normalisation alone, a Windows one (MA7) [inference: NTFS ignores case
   and keeps normalisation; not measured]. A delete push from inside a Mac `files` clone is what this avoids: git's post-push
   delete of the deleted name's tracking ref can unlink the surviving member's loose file, which
   APFS treats as the same name, and rewind it to a stale packed value (M24) [review: Gemini].
   The Mac push rule's part 3 refuses that delete, so the mistake now fails loudly;
4. then, on every Mac clone of that remote, recover what only that clone holds, a commit `git fsck`
   reports dangling (M16's harm) and any unpushed branch, before `git gc` can drop it, and then
   re-clone, with `--ref-format=reftable` or without. The re-clone is not optional: M24's rewind
   leaves `git fsck` clean, so a clean `fsck` proves nothing about a Mac clone's tracking refs
   [review: Kimi].

Gate step 5.10 performs this repair on the Mac.

**Release.** A minor bump [inference: a new refusal, and a new platform position]. Because it
changes compiled code, it takes the formal Windows and Linux gates (`docs/releasing.md`), plus the
one-time macOS gate (§6).

### 4. Credentials and scheduled use (MD4(b), O2(b))

The helper changes nothing about credentials. On macOS the CLI's default store is the login
keychain (M5), which a logged-in user already has (M6).

- **MD4(b) gated a launchd-agent push** on a premise later corrected (the PC makes no scheduled v2
  pushes). Craig then ruled O2(b): the Mac's scheduled backups are one LaunchAgent per
  repository, each repository pushed from that one machine. The gate's launchd step is made
  representative:
  - the agent is started by its real schedule with the screen locked, not by a warm `launchctl
    kickstart`; the gate's copy uses `StartInterval` at 300 s so the wait is bounded, while the
    README's example uses a daily `StartCalendarInterval` *(my call)* [review: DeepSeek]. A run
    after a fresh login is not part of the gate, which cannot log out mid-run [review: Gemini];
  - **the push it runs** is two commands, `git -C <repository> push proton-v2 --all` and then `git
    -C <repository> push proton-v2 --tags`, so every branch and tag is backed up and a scheduled
    run never deletes a remote ref on its own (no `--mirror`). git refuses `--all` with `--tags`
    in one command, and a bare `git push` sends only the current branch, or nothing when it has no
    upstream [mac: measured on a local repository, git 2.54.0, 2026-10-27; review: DeepSeek];
  - its plist runs a small wrapper script with absolute paths (launchd does not expand `~`, and its
    working directory is `/`), **exports** `PATH` to reach git, the helper and the CLI (launchd's
    default PATH is `/usr/bin:/bin:/usr/sbin:/sbin`, and it reads no `~/.zprofile`), exports the
    gate's `PROTON_DRIVE_CACHE_DIR` for the gate run, so git, the helper and the CLI inherit both
    [review: DeepSeek], and sends stdout and stderr to named files;
  - **a failed run is visible beyond the log** [review: Kimi]: the wrapper posts a macOS
    notification (`osascript -e 'display notification …'`) and leaves a marker file when either push
    exits non-zero *(my call)*, and the README says a refusal there is a stop-and-repair condition,
    not a transient;
  - **a hung run is a failed run** [review: Codex]: the wrapper bounds each push at 30 minutes, the
    gate's own bound (§6 step 6), and on expiry ends the push's whole process group (git, the
    helper and the CLI), then posts the notification and leaves the marker as for a non-zero exit.
    macOS ships no `timeout`, so the mechanism is *left to the builder*; the plan tests the expiry
    path with a stand-in command that never exits, with no account *(my call)*;
  - the README's example LaunchAgent and wrapper are the gate's, minus its cache folder and
    interval, with the one-machine-per-repository rule beside them. Its daily
    `StartCalendarInterval` runs on the next wake when the Mac slept through the hour (Facts).
- **Not gated, and said so:** the `pass` and `unsafe_file` stores; SSH sessions, where the login
  keychain may be locked; a LaunchDaemon, which has no user session; runs during sleep or dark wake;
  and a Keychain prompt after a CLI upgrade [untested; whether a re-signed Proton CLI keeps access
  is an inference from Keychain access control]. O4(a) kept that last one ungated, so the README
  says a scheduled Mac backup should be checked by hand after each CLI upgrade *(my call)*.

### 5. Hermetic runs on a Mac with a signed-in CLI (O6(c))

Once the Mac adopts v2 for real, a signed-in `proton-drive` is on PATH. The hermetic suite reaches
`proton-drive` through PATH only after prepending a test shim (`shim_test.go:110`; `:229`, `:264`
for the lookups), so the real CLI stays resolvable behind the shim; the live contract half at
`contract_test.go:617` runs only under `GPB_LIVE_ACCOUNT=1`. A shim bug like F4's would fall
through to the real CLI: F14's hazard, by another route. On a Mac the harm is not only the
network: the real CLI can read the login keychain and write its local state, so blocking the
network alone does not make a run hermetic [review: Gemini, Codex; M5, M7, M22].

What the suite resolves today [code, grep at `8848f41`]: production code builds the CLI from PATH
only in `runSetHead()` and `run()` (`main.go:129`, `:191`) [review: DeepSeek, who caught the
order]. Tests reach `runSetHead` only behind a
placed shim (`shim_test.go:189`), call `run()` only with addresses refused before the CLI is built
(`main_test.go:432`), and resolve `proton-drive` themselves only behind a shim (`shim_test.go:229`,
`:264`) or in the live contract half (`contract_test.go:617`, `GPB_LIVE_ACCOUNT=1` only). No test
runs git against a `proton::` URL, so none reaches an installed `git-remote-proton` either. So
today's suite never resolves the real CLI; the rule is what keeps that true.

`docs/research/gates/brief-checklist.md` gains **rule 12:** a hermetic test run on a Mac never
resolves a real `proton-drive`. A gate's hermetic run also takes the real CLI off PATH, the
stronger form rule 11 uses in WSL. `sandbox-exec` with networking denied (M22) may be added on top,
never instead: it leaves files and the Keychain reachable, and Apple has deprecated it. A rule-10
audit of a staged CLI is not a hermetic run, and stays under rule 10.

Under O6(c) the rule reaches every test run, not only gates:

- **AGENTS.md** gains one line under Repo Gotchas: a hermetic test run (`go test`, Pester) must
  never resolve a real `proton-drive`; on a Mac with a signed-in CLI, see `brief-checklist.md`
  rule 12; rule-10 audits of a staged CLI are not hermetic runs [review: Kimi].
- **The test code checks it, twice.**
  1. Every helper that places a shim (`buildShimOnPath` in `cmd/git-remote-proton/shim_test.go`,
     and any like it) asserts, after setting PATH, that `exec.LookPath("proton-drive")` resolves to
     the shim it just placed, and fails the test with both paths otherwise: the F4 failure caught
     at its source (Craig's O6(c)).
  2. A shim-wins check cannot see the real CLI sitting behind the shim, or a future test that
     reaches `run()` with an accepted address [review: Gemini]. So each package that can reach
     `transport.NewCLI("")` puts a **trap** first on PATH in `TestMain`: a stand-in `proton-drive`
     (`proton-drive.exe` on Windows, for the PATHEXT reason `shim_test.go`'s comments give) that
     exits non-zero with "a hermetic test reached proton-drive without a shim" [review: DeepSeek]. Any test that
     resolves the CLI without its own shim then hits the trap, never the real CLI, whatever else
     is on PATH, the installed helper included *(my call, within O6(c)'s intent; a plan may choose
     another mechanism with the same effect)*. Unless `GPB_LIVE_ACCOUNT=1`, where the live
     contract half uses the real CLI on purpose.
  Both hold on every platform, the PC included, where a signed-in Windows CLI has always sat on
  PATH behind the shim [inference from `shim_test.go:110`].

### 6. The one live macOS gate (MD5(a), O5)

Run once, for the release that carries this port, on Craig's MacBook Air. It runs under
`brief-checklist.md` rules 1–12. Its place in the release (O5, all four parts):

- **Order:** the Linux gate, then the Windows gate, as for v0.10.0, then this gate last, after the
  draft Windows helper is installed, so step 5's refusal "from Windows" is the new helper's. Never
  concurrently: the gates share the account's row-set baseline, the trash tally and the
  paused-task window. That is between gates; within this one, the Mac and the paired PC session
  run together, the PC session live through Mac steps 5.5 and 5.8, which need its pushes
  [review: DeepSeek]. The release's draft stays unpublished until this gate ends, pass or fail,
  so the draft Windows helper step 0 checks for is still the release's [review: DeepSeek].
- **A Mac-only failure does not hold the release.** No macOS asset ships, so the Windows and Linux
  assets publish on their own gates; the README keeps macOS "not gated" until a later release
  passes a Mac gate, the docs being updated after gates as they were for v0.10.0's Linux gate. A
  defect in shared code found here is a defect in the release, and follows `docs/releasing.md`'s
  no-retag rule like any other.
- **After a failed Mac gate, the next attempt is this whole section again, at a later release**
  [review: Kimi; my call]: its own v0.10.0 build and sign-in probe, its own paired PC session, the
  same order after that release's Windows and Linux gates and its draft Windows helper, and its
  own brief and record. Never the failed steps alone and never a stand-in: the stand-in rule needs
  an unchanged anchor (Facts), and no Mac gate has passed to anchor to. The README's status line
  stays at its first pass until one does.
- **The Windows side** (the sign-in probe's Windows steps, the task pause, every Windows check,
  step 4's and step 5's Windows pushes and clones, step 5.5's NFD move and step 5.8's two rename
  pushes, the cross-machine ref-set comparison) is run by
  a paired agent session on the PC from its own section of the gate brief, and both machines'
  results go into one gate record. The probe's Windows steps are in its scope too, since the
  probe holds the PC's scheduled task paused [review: Kimi; my call within O5(c)].
- **The release's Windows and Linux gates also test the composition refusal** on their own
  binaries, as v0.10.0's tested the case refusal per platform.

**Prep, before tagging, with no account: the v0.10.0 build.** Step 5.5 needs v0.10.0's helper on
the Mac, and v0.10.0 shipped no macOS binary. So, before the sign-in probe, build it from a fresh
clone of the `v0.10.0` tag with §1's recipe into a scratch folder outside every clone and never on
PATH, and confirm its `--version`; a failure here is found before any gate state is held [review:
Kimi, DeepSeek]. The probes
(M0) showed a darwin build compiles and only three tests failed, on a path alias, which does not
affect the binary [inference].

**Prep, before tagging: a sign-in probe.** It changes nothing in the release and ends logged out.
Steps 1, 2, 5, the Windows checks in 6, and the unpause in 7 are the paired PC session's; the rest
run on the Mac.

1. Pause `ProjectsBackupDaily` and wait for any running instance to finish.
2. Run a read-only `proton-drive filesystem info /my-files` **on Windows**, as a baseline. The PC
   uses the CLI only to read (Facts), so a read check exercises what it does.
3. On the Mac, assert first that no Keychain item `ch.proton.drive/drive-sdk-cli` and none of M7's
   default folders exist. Then, with `PROTON_DRIVE_CACHE_DIR` set to a probe folder, Craig runs
   `proton-drive auth login`, which signs in through the browser (F10).
4. One read-only CLI command; confirm the Keychain item now exists and that every state file landed
   in the probe folder (M7).
5. The Windows check again.
6. `auth logout`; confirm the CLI reports `You need to login first` and the Keychain item is gone
   (the Linux probe's step 6 shape [record: `v0.10.0-linux-gate.md`]). If the logout fails or the
   item remains, delete it locally with `security delete-generic-password -s
   ch.proton.drive/drive-sdk-cli` and record that. Then the Windows check once more.
7. Delete the probe folder, confirm M7's default folders are still absent, and unpause the task
   **only if every Windows check passed**. If one fails: STOP for Craig, task left paused.

**Gate setup.** The task paused for the whole gate; Craig signs in as in the probe; a gate-only
`PROTON_DRIVE_CACHE_DIR`; network on; the helper built **from source at the release tag** with the
README's exact commands (§1), in a fresh clone outside every other working tree, so the gate tests
the install the README gives and the build stamps a clean tree (M27) [review: DeepSeek]. The tag
exists before this gate runs, as for every gate under `docs/releasing.md`.

**Gate content,** following Linux §7, with its write confinement and verify-before-trash rules:

0. **Preconditions, asserted.** `type -a` finds exactly one `git-remote-proton`, the one just
   built, and exactly one `proton-drive`, the certified CLI; the CLI's SHA-512 equals M2's;
   `go version -m` on the built helper shows the tag's commit as `vcs.revision` and
   `vcs.modified=false`, which the fresh clone gives (M27; a tree with any untracked file that is
   not ignored would fail it); the gate cache folder is in use and M7's defaults are absent,
   re-checked after step 6, whose wrapper must hand the cache folder on [review: DeepSeek]; the
   Windows helper is the draft's (step 5 needs its refusal); the Windows checks and a `/my-files`
   row-set
   baseline (rule 1). These hold at the gate's start and stay true: the one second helper the gate
   uses, in step 5.5, is never on PATH.
1. **Offline checks:** the helper's `--version` shows the tag and the certified token; the README's
   CLI commands, run as written into a temp folder, give bytes equal to the installed CLI's.
2. **The live contract table from the Mac,** once, under its own root,
   `GPB_CONTRACT_LIVE_ROOT=/my-files/_cas-probe/contract-mac`, named in the brief and the
   confinement list (rule 8) *(my call: the Linux gate's step 2, under a root no other gate
   writes; MD7 keeps later certifications offline)*.
3. **Smoke:** bootstrap push, clone with `git fsck --full`, incremental fetch, `--set-head`.
4. **Cross-platform:** a repo pushed from the Mac is cloned on Windows, and one pushed from Windows
   is cloned on the Mac, each with `fsck` and a ref-set comparison; then an update and a branch
   deletion each way, fetched by the existing clones.
5. **Case and composition variants,** inside the gate root. Every Mac step below that gives an
   NFD name "by exact bytes" runs in a repository whose config sets
   `core.precomposeunicode=false`, as M24's probe did, with the name written as bytes (for example
   `$'cafe\xcc\x81'`), since git precomposes its arguments otherwise (M13) [review: DeepSeek]. Each
   remote of its own is seeded with `main` first, so its `HEAD` names `main` and deleting a `café`
   spelling is never the `HEAD`-branch delete `push.go:181` refuses [review: Gemini].
   1. a create of `refs/heads/café` in NFD beside an NFC one is refused by the Mac's helper and by
      the draft Windows helper, naming both and the relation, with no ref or pack written; the NFD
      name is pushed with exact bytes, because git on a Mac precomposes its arguments (M13);
   2. a mixed push lands its update and refuses the variant create;
   3. D2's case-variant steps rerun from the Mac;
   4. a pair laid down with a CLI upload still lets an unrelated ref be pushed and refuses a third
      spelling;
   5. **a Mac clone of that remote into the default ref store is refused** at `list`, with the
      pair and object IDs named, and leaves no ref written (O3(d)). Then M16 through a real
      helper, measured once rather than inferred *(my call)*: the v0.10.0 helper from prep, used
      for this step's commands alone through a per-command `PATH` [review: Gemini, DeepSeek],
      clones the remote (if the release moved the CLI pin since v0.10.0, that helper refuses the
      certified CLI, so its commands here run with `GPB_UNCERTIFIED_CLI=1`, recorded as such
      [code: `main.go:31` at `v0.10.0`; review: DeepSeek]); then **the NFD member** is moved to a
      new commit by a push from the
      paired Windows session, with the name as exact bytes [review: Kimi, Gemini]. Not from the
      Mac: a Mac `files` clone through the gated helper is refused by the Mac push rule, which is
      what it is for; a push from the clone under test would update its own tracking ref and
      leave the fetch nothing to do; a typed Mac push precomposes to NFC (M13); and a CLI upload
      would move the ref without uploading its pack. Then the clone under test fetches, and what
      it leaves is recorded;
   6. **the same remote cloned with `--ref-format=reftable` through the gated helper** gets the
      warning only, and both spellings survive the clone and a fetch, checking M17's mitigation
      end to end;
   7. `git ls-remote` of that remote from Windows and, on the Mac, from a folder with no
      repository above it prints the warning and succeeds; from inside a Mac `files` repository,
      and from a subfolder of one, it is refused, as a fetch would be (O3(d), M25) [review:
      DeepSeek, Gemini];
   8. **the rename transition**, on a remote of its own [review: Codex, Gemini]: a fresh Mac clone
      holds `café` in NFC; the paired Windows session deletes it and then, in a second push,
      creates the NFD spelling by exact bytes **at the same commit**, so the remote never
      advertises a pair and git would send no `fetch` command; the Mac's next fetch is refused at
      `list` under rule (2) of §3, and nothing is written. Not from the Mac clone under test: its
      own delete push would remove the stale tracking ref this step exists to leave, and with it
      step 5.9.2's precondition [review: Kimi, Gemini];
   9. **the Mac push rule**, each part where nothing else would refuse first [review: Gemini]:
      1. on a remote of its own, a Mac `files` clone pushes `café` in NFC, and the NFD member is
         then laid down at the same commit as step 5.4 lays one, so the Mac holds the member it
         will update and git's fast-forward check (`push.go:266`) passes [review: Gemini]; a Mac
         push that updates the NFC member is then refused for that ref while an unrelated ref in
         the same push lands (part 1, remote half);
      2. on step 5.8's remote, which holds `café` in NFD only, the Mac clone from step 5.8, still
         holding its stale NFC tracking ref, pushes a delete of the NFD name by exact bytes: it is
         refused (part 3), and every local ref, compared by `for-each-ref` before and after, is
         unchanged (M24);
      3. on a remote of its own that holds neither spelling, a Mac clone carrying a tracking ref
         for `café` in NFC with no remote counterpart (made with `git update-ref`, as a never-pruned
         leftover would be) pushes a create of the NFD spelling by exact bytes: D2 has nothing to
         refuse, and it is refused by the local half of part 1 alone;
   10. **the repair** (§3), on step 5.9.1's remote: from a temporary `reftable` clone on the Mac,
      the NFD member, which `HEAD` does not name, is deleted by its exact bytes; the delete is not
      refused; a fresh Mac `files` clone then succeeds with no warning, and step 5.9.1's Mac clone
      is recovered and re-cloned as §3's repair step 4 says [review: Kimi].
6. **The launchd push (MD4(b), O2(b)):** the Mac on mains power with its lid open, and
   `caffeinate -is` held for the step's span, because launchd misses a `StartInterval` firing
   while the system sleeps (Facts) and a sleeping Mac would trip the bound below for the wrong
   reason [review: Kimi; `caffeinate`'s effect on a locked screen is untested]. The agent fires on
   its 300 s `StartInterval` with the screen locked (§4); both pushes succeed, no Keychain prompt
   appears, its log files show the result, and **the remote's ref set then equals the
   repository's local branches and tags**, so a
   push that sent less than everything cannot pass [review: DeepSeek]. If it has not fired within
   two intervals, or its pushes have not finished within 30 minutes *(my call: `brief-checklist.md`
   rule 9 asks for a long timeout but names no number)*, BLOCK: a Keychain prompt on a locked
   screen shows up as exactly that hang [inference; review: DeepSeek].
7. **Cleanup, in order, on every exit path:** first unload the LaunchAgent (`launchctl bootout`)
   and wait for any push it started to finish, so no scheduled write can land during cleanup
   [review: Codex]; then trash the gate's subtrees, verify-before-trash (rule 3); check the row set
   and tally trash (rules 1, 7); `auth logout`, confirm the Keychain item is gone, and fall back to
   `security delete-generic-password` as in the probe; delete the scratch v0.10.0 build; confirm
   M7's default folders are still absent; the Windows check; delete the gate cache folder; unpause
   the task only if the Windows check passed.

**Afterwards (MD5(a), O3).** Real use starts with a fresh sign-in into the default folders, after
the writers-first step and the pre-adoption scan of every remote the Mac will push (§3).

### 7. Certification afterwards (MD7(b))

Each future CLI certification adds offline checks on the Mac, with no account and networking
denied: download the new `macos/arm64` build; check its SHA-512 against the feed; check its
`--version` token; run it signed out through the helper (M8's shape); record the SHA-512 for the
README's macOS CLI line. The helper refuses any CLI but the one it was built to accept (M8), so
the signed-out run uses the release candidate that moves the pin, built from source in a fresh
clone of that commit into a scratch folder outside every clone and never on PATH, as §6's prep
builds v0.10.0, and the certification record names its commit; never `GPB_UNCERTIFIED_CLI=1`,
which would skip the very check the run exercises. `scripts/Test-ProtonCliDrift.ps1` already asks
for the frozen candidate on the other platforms [code; review: DeepSeek]. O4(a) kept it at that:
no signed-in Mac check at certification.
Networking is denied with `sandbox-exec` (M22); if a later macOS removes it, the signed-out run is
made with the Mac's networking switched off, and the certification record says so [review: Kimi].

- `scripts/Test-ProtonCliDrift.ps1` gains a fifth leg comparing the feed's `macos/arm64` entry
  (the feed's platform string, M1) with a recorded `$CertifiedMacArm64Sha512`. Like the Linux leg
  it compares the feed with a recorded value and cannot see a CLI installed on the Mac. A latest
  release with no `macos/arm64` entry is a MISMATCH, as a missing `linux/x64` entry is, because the
  README pins a macOS CLI download *(my call)*.
- `tests/ReadmePins.Tests.ps1` guards the README's macOS CLI URL (the `darwin-arm64` path, M1) and
  SHA-512 as it does Linux's. `tests/DriftCheck.Tests.ps1`, which asserts the verdict line's exact
  text, follows the new leg [code; review: DeepSeek, verified].
- The README says live behaviour of each new CLI on the Mac is not gated.

### 8. Documents that change

- **README:** the "Install on macOS (v2)" section (§1); the note on a binary that arrived another
  way; honest limits for canonical equivalence and its cost (§3), what a Mac does with an existing
  pair and the repair (§3, O3), macOS's gating position, scheduled use (§4) and drift (§7); the
  case-folding sentence at `README.md:412` ("Case is folded the way Go's `strings.EqualFold` does,
  Unicode simple folding"), which O1(iv) makes untrue for release builds [review: DeepSeek], and the
  Unicode line at `README.md:414` ("Unicode normalisation (NFC against NFD, as on macOS) is out of
  scope; neither is checked") are replaced; and every passage that names the platforms without
  macOS: the quick-start's "On Linux, only v2 applies" (`:27`), the Requirements line (`:30`), the
  two-tools table's Platforms row (`:94`) and the release-assets paragraph (`:100`), the "Why
  PowerShell?" close, which says the Roadmap "says what macOS would take" (`:320-321`) [review:
  DeepSeek, verified], and the roadmap line (`:471`); and honest limits for the lone NFD name (§3)
  and for the Mac refusals' reach (command-line refspecs, `--refmap`, the scan's folder).
- **`v2-remote-helper-design.md` → v6.10:** the supported-platforms line gains macOS's position;
  the D2 rule's description widens to the two relations; the rulings below are copied in.
- **`docs/releasing.md`:** a sentence that macOS has no release asset and so no per-release gate,
  and why; the macOS gate's place in the port's release (O5); the certification section's macOS
  offline checks (§7); the drift check's fifth leg, including the prose that says it "compares
  four things".
- **`brief-checklist.md`:** rule 12 (§5). **`AGENTS.md`:** its one line (§5, O6(c)).
- **`.github/workflows/ci.yml`:** the macOS leg and the generate-and-diff step (§2).
- **`internal/repo`:** `nfd`, `canonKey`, the generated tables, their generator and its committed
  inputs, `caseKey` on the generated folding with its doc comment (O1(iv)), both relations at every
  call site, `caseConflictMessage` and the rename-delete message, the pure pair, prefix,
  refspec-mapping and destination-set checks the read rules use, the Mac push rule's three parts
  inside `Push`'s phase-2 fixed point, `Push`'s one new argument carrying the local side, and
  tests, including `casefold_test.go`'s `TestCaseKeyMatchesEqualFold`, the message tests that
  change, and a direct-call replacement batch (M26).
- **`cmd/git-remote-proton/main.go`** and **`main_test.go`**: whether `GIT_DIR` was set, read
  before `resolveGitDir`; the remote's name threaded from `run()` into `loop()`; the darwin local
  reads, failing closed; the warning at `list` and `list for-push`; the Mac refusal at `list`;
  and the local side handed to `Push` from the push arm (§3, O3(d)) [review: Gemini, DeepSeek,
  Kimi].
- **`cmd/git-remote-proton/shim_test.go`** and any other shim helper: the resolution check; and a
  `TestMain` trap in each package that can reach `transport.NewCLI("")` (§5).
- **`.gitattributes`:** `-text` for the committed Unicode data files (§3).
- **`docs/superpowers/specs/2026-10-15-v2-linux-port-design.md`:** a dated superseding note at
  its §5, pointing here for the widened rule, so a reader of D2 there does not take it as
  case-only [review: Kimi]. The note adds; the spec's text is not rewritten.
- **`scripts/Test-ProtonCliDrift.ps1`**, **`tests/ReadmePins.Tests.ps1`** and
  **`tests/DriftCheck.Tests.ps1`** (§7).
- **The macOS gate brief and record** under `docs/research/gates/`, and the Windows and Linux gate
  briefs for the same release (O5).
- **Probes:** the NFC/NFD, combining-mark, fold-class, Gatekeeper, remote-helper-protocol,
  `GIT_DIR`, clone-time, delete-aliasing, same-ref push, `vcs.modified` and quoting probe scripts
  (M12–M29) move into
  `docs/research/probes/` with their pinned results *(my call)*.
- **`CHANGELOG.md`**, the `ModuleVersion` bump `releasing.md` step 2 requires, and
  **`docs/README.md`**, linking this spec and its plan.

## Decisions

**Rulings, 2026-10-27.** Each is Craig's call, accepting the recommended default; none is an
unopposed default. The options are as they were put to him.

| | Ruled | Not taken |
|---|---|---|
| MD1 | (a) darwin/arm64 only | (b) darwin/amd64 built but ungated; (c) a universal binary |
| MD8 | (a) Intel out of scope, ruled with MD1 (the option Craig chose said so) | (b) Rosetta tests on this Mac |
| MD2 | (d) build from source at a release tag; live-gated once at the port's release; CI on macOS after that | (a) a darwin-arm64 release asset gated every release; (b) (a) plus Developer ID signing and notarization; (c) a Homebrew tap |
| MD3 | (a) one `macos-latest` leg at Go `stable`, with the git-floor probe and a `brew` fallback | (b) a second leg at go.mod's toolchain |
| MD4 | (b) gate attended use with the default `keychain` store and one launchd-agent push; `pass`, `unsafe_file`, SSH documented as not gated | (a) attended only (Linux D3(a)); (c) say nothing |
| MD5 | (a) the one gate on this Mac, with a pre-gate sign-in probe, a gate-only cache folder and Windows checks; afterwards log out, confirm the Keychain item gone, and sign in fresh for real use | (b) keep the gate session; (c) a CI runner with a session in secrets |
| MD6 | (a2) refuse canonically equivalent creates on every platform, with the repo's own tables generated from Unicode's data at one pinned Unicode version, for both decomposition and simple case folding; no dependency, no floor change. *Amended by O1(iv) on 2026-10-27; it first read "with the repo's own table generated from Unicode's data at the standard library's Unicode version".* | (a1) `golang.org/x/text/unicode/norm`; (b) refuse any combining mark (M21); (c) document only, with the `reftable` mitigation |
| MD7 | (b) offline macOS checks at each certification, a drift-script leg and a README pin test; confirmed by O4(a) | (a) the full live contract table on the Mac every certification; (c) nothing on the Mac |

**Rulings on the review's questions, 2026-10-27.** Each is Craig's call, taking the recommended
default; the questions as put to him follow under "Questions raised in review".

| | Ruled | Not taken |
|---|---|---|
| O2 | (b) one LaunchAgent per repository, each repository pushed from one machine only; MD4(b) stands on this corrected premise | (a) attended pushes only; (c) a Mac counterpart of the PC's scheduled tooling, as its own project |
| O1 | (iv) one pinned Unicode version, 17.0.0 today, for both decomposition and simple case folding, generated from Unicode's data; amends MD6 | (i) one table per toolchain Unicode version, by build tag; (ii) one table at 15.0.0; (iii) one table at 17.0.0 with case folding still from the toolchain |
| O3 | (d) the pre-adoption scan, writers first, a warning on every platform when the advertised refs hold a pair, and on a Mac with the `files` ref store a refused fetch | (a) the scan only; (b) the scan and writers first; (c) those plus the warning, with no refusal |
| O4 | (a) MD7 as ruled: offline Mac checks only after the port | (b) an attended check on the Mac's own session at each certification and at each transport or credential change |
| O5 | all four: the Mac gate last, after the draft Windows helper, never concurrent; a Mac-only failure does not hold the release; a paired PC session runs the Windows side; the Windows and Linux gates test the composition refusal on their own binaries | any of the four left out |
| O6 | (c) rule 12, an AGENTS.md line, and a shim-resolution check in the test code | (a) the checklist only; (b) the checklist and AGENTS.md |

**Amendments, ruled 2026-10-27 and 2026-10-28, after the spec's approval.** Each corrects a fact
the approved text stated, and each is Craig's call. MA1 and MA2 accept the recommended default;
they were found while writing the plan (`docs/superpowers/plans/2026-10-27-v2-macos-port.md`,
Task 0), where each one's before-and-after text was put to him. MA3 was found by the review of the
plan's Task 13, which probed the remedy in scratch repositories; Craig chose option (c) of three on
2026-10-28, with its before-and-after text in front of him. MA4 to MA9 came from the plan's final
whole-branch review and its task reviews; on 2026-10-28 Craig accepted the recommended default for
each ("Proceed with your recommendations"), with the before-and-after text of MA4 to MA8 in front of
him. MA9 changes no text.

| | Ruled | Not taken |
|---|---|---|
| MA1 | `remote.<name>.mirror` is not a fetch-destination source (§3, rule (2)): git uses it for push only [doc: `git help config`, git 2.54.0] | keep reading it, a key git never uses for fetch |
| MA2 | the pin-moving fetch checks the files' version lines and records their SHA-256s, and the data is cross-checked by `NormalizationTest.txt` and the containment test (§3, "The inputs are committed"); Unicode publishes no checksums [2026-10-27: none in the 17.0.0 `ucd/` listing or either ReadMe] | the original "published checksums" check, which cannot be made |
| MA3 | the stale-local-ref remedy: a bare exact-bytes delete can unlink the other spelling's loose ref on APFS (§3, "The refusal's remedies follow its cause"), so the re-clone comes first and the in-place delete runs only after `pack-refs` with precomposition off; the helper's refusal messages name only the re-clone and point to the README for the in-place route [measured 2026-10-28, git 2.54.0: after a real clone and a fetch, the bare delete rewound the other ref to a stale packed value while `git fsck` exited 0, listing the lost commit only as dangling (wording corrected by MA8); packed first, only the named ref went] | the bare exact-bytes delete; (a) the re-clone alone, everywhere; (b) the pack-first delete alone |
| MA4 | a lone NFD name at git's Mac default (§3, "What no rule here catches at first"): its first incremental fetch leaves git listing a phantom NFC tracking ref, its next fetch fails in git, and the helper refuses that fetch first; `LocalRefs` keeps reading git's own view, so no code changes, and the README names the limit with a `reftable` clone as the remedy [measured 2026-10-28, git 2.54.0, `docs/research/probes/macos-port/lone-nfd-default/`] | (b) read exact bytes (`-c core.precomposeunicode=false`), which lets git fail the fetch itself; (c) refuse a lone non-NFC destination up front on a Mac `files` clone, new code and a gate step |
| MA5 | the push rule's termination bound is (entries + 1): an outer round can continue on a part-3 delete alone (§3, the Mac push rule) | the stated (updates + 1) |
| MA6 | M28 holds for a plain clone; a clone given `-c remote.origin.fetch=<spec>` has that key set at `list`, which is why the empty-repository checks add git's default mapping to the configured specs | M28 as a plain-clone-only statement |
| MA7 | the repair deletes a member from a Linux client or a `reftable` clone, and from Windows only for a pair that differs by normalisation alone (§3, "Repair", step 3) [inference: NTFS ignores case and keeps normalisation; not measured] | Windows as a route for any pair |
| MA8 | MA3's measurement reads "`git fsck` exited 0, listing the lost commit only as dangling", as its record shows (layout L8), not "clean" | the "clean" wording |
| MA9 | a dangling symbolic local ref stays an accepted limit: every git ref enumerator skips it, so a destination written through it is unseen; the common case, a dangling `refs/remotes/<r>/HEAD` after a prune, is still caught by R6's synthesized `<p>/HEAD` (no text change in §3) | a fourth local read that walks loose refs |

The plan also makes six refinements (R1–R6) under O3(d)'s licence that "a plan may refine them with
a reason". They were put to Craig on 2026-10-27 and not objected to, so they stand as unopposed
defaults, recorded in the plan.

**What the rulings rested on, and what has moved since.**

- *MD2:* a macOS release asset means a third live gate on almost every release, because a stand-in
  needs unchanged code (`docs/releasing.md`). That still holds.
- *MD4(b)* rested on "back up the way the PC does, including scheduled pushes", which was wrong
  (Why now). O2(b) re-ruled it on the corrected premise.
- *MD6(a2) over (a1):* no first dependency (x/text's latest would also raise the floor to 1.26;
  the newest that keeps 1.22 is v0.22.0, from 2025-02-04), and, as then stated, "one Unicode version
  throughout (17.0)". That last reason was wrong: the standard library's version depends on the
  toolchain. O1(iv) settled the version, and makes the stated reason true by pinning it. Review
  also noted that x/text v0.22.0 would give one fixed version on every machine without moving the
  floor [review: DeepSeek]; O1(iv) gives the same uniformity without the dependency.
- *The key itself:* the first draft's `nfd(caseKey(nfd(s)))` and then `caseKey(nfd(s))` alone
  were both corrected in review (§3).

## Questions raised in review

As they were put to Craig on 2026-10-27, ordered so the ones the others follow from come first.
All six are ruled (Decisions); the closing line of each records the ruling.

**O2. What will run the Mac's backups, and against which remotes?** No machine makes scheduled v2
pushes today (Why now). (a) Attended pushes only, for now: MD4 reverts to (a), the gate drops step
6, and the README says scheduled use is not gated. (b) A per-repo LaunchAgent from the README,
gated as §4 describes. (c) A Mac counterpart of the PC's scheduled backup tooling, as its own
project. Also: will the Mac and the PC push the same repositories to the same remotes? A forced or
`--mirror` push from either would rewind or delete the other's refs, which this spec does not
cover [review: builder check]. *Ruled (b), each repository pushed from one machine only.*

**O1. The Unicode version, and whether case folding moves to a generated table too.**
(i) one table per Unicode version the toolchains carry, chosen by Go-version build tags; (ii) one
table at 15.0.0; (iii) one table at 17.0.0, with case folding still from the toolchain; (iv) one
pinned version, today 17.0.0, for both decomposition and simple case folding, generated from
Unicode's data, so every build refuses the same set. All five reviews found that (i) makes the
answer depend on the pusher's compiler: release binaries (Go 1.22) and a Mac build (Go 1.27) would
refuse different sets, and a Go release with a new Unicode version would turn `main` red with no
code change. Unicode's normalisation stability policy keeps assigned characters' decompositions
fixed [review: builder check, from recollection; not re-read here], so a newer table only adds
mappings. (iv) also moves D2's existing case relation off the toolchain's tables, which today make
it differ by build in the same way; its answers change only for characters whose folding differs
between Unicode 15 and 17. Choosing (ii), (iii) or (iv) amends MD6's words "at the standard
library's Unicode version". *Ruled (iv).*

**O3. A Mac that reads a remote already holding a pair.** The create refusal stops new pairs only;
M16 shows a Mac fetch of an existing pair corrupting tracking refs silently and leaving a commit
dangling. (a) A pre-adoption scan only (my earlier suggestion): before the Mac uses a remote,
`git ls-remote` it and check every namespace under both relations, failing closed on anything
unreadable. (b) (a), plus a rule that every client pushing to those remotes runs a release with
this rule first, since older helpers can still write a pair. (c) (b), plus the helper warning on
`list` and fetch, on every platform, when the advertised refs hold a pair under either relation.
(d) (c), but on a Mac whose repository uses the `files` ref store the helper refuses the fetch when
the advertised refs hold a pair, naming it and the remedies (repair from another machine, or a
`reftable` clone), as Stage 6 already fails closed on unreadable content. *Ruled (d).*

**O4. A recurring Mac check after the port.** MD7(b) and MD2(d) leave the Mac's own builds, later
CLI versions and the Keychain-after-upgrade question ungated after the port's release, while the
Mac is Craig's backup machine [review: Gemini, Codex, DeepSeek]. (a) As ruled: offline checks only.
(b) At each CLI certification, and at each release that changes transport or credential code,
an attended check on the Mac's own signed-in session: one read-only command and one push to a
scratch remote, which would surface a Keychain prompt. It uses the session real use already has,
so it adds no sign-in. Choosing (b) amends MD7. *Ruled (a): MD7 stands.*

**O5. The Mac gate's place in the port's release.** (a) Order: Linux, then Windows (as for
v0.10.0), then the Mac last, after the draft Windows helper is installed, so step 5's refusal
"from Windows" is the new helper's; never concurrently, since the gates share the account's
row-set baseline, the trash tally and the paused-task window. (b) A Mac-only failure does not hold
the Windows and Linux assets: they publish, and the README keeps macOS "not gated" until a later
release passes a Mac gate (docs are updated after gates, as for v0.10.0's Linux gate). (c) The
Windows side of the Mac gate (the task pause, the Windows checks, step 4's and step 5's Windows
pushes) is run by a paired agent session on the PC with its own brief section, results recorded
in one gate record. (d) The release's Windows and Linux gates also test the composition refusal on
their own binaries, as v0.10.0's tested the case refusal per platform. *Ruled: all four.*

**O6. How far rule 12 reaches.** (a) The gate checklist only. (b) Also a line in AGENTS.md for every
test run on the Mac. (c) Also a check in the shim helper that `exec.LookPath("proton-drive")`
resolves to the shim before any test uses it, failing otherwise. That catches an F4-type bug on
every platform, including the PC, where a signed-in Windows CLI has always been on PATH behind the
shim [inference from `shim_test.go:110`]. *Ruled (c), which includes (a) and (b).*

## Left to the builder

These are my calls; a plan may change them with a reason.

- The README's exact wording, the install directory, and the example LaunchAgent.
- How the CI steps are shared between Linux and macOS, and which leg runs the generate-and-diff
  step.
- Where the committed Unicode data files sit, and the `.gitattributes` rule that pins them.
- The refusal message's exact text, provided it names the relation, the harmed clients, and each
  spelling's `%+q` form (§3, M29).
- Where the generator and tables live inside `internal/repo`.
- The gate briefs' step numbering and BLOCK rules, under `brief-checklist.md`.

## Still untested, and how it gets tested

| Item | Tested by |
|---|---|
| `macos-latest`'s git version | the CI probe step (§2) |
| Sign-in on the Mac, the Keychain item's life cycle, and `PROTON_DRIVE_CACHE_DIR` with a session | the pre-gate sign-in probe (§6) |
| A second long-lived CLI session (the Mac's) beside the Windows one | the gate's Windows checks for the gate's span; afterwards, ordinary use and the PC's verify runs [untested beyond that] |
| A scheduled LaunchAgent push reaching the Keychain without a prompt, screen locked | gate step 6, under O2(b) |
| M16 and M17 through the helper | gate step 5 |
| The canonical-equivalence rule live, on each platform's helper | gate step 5; the Windows and Linux gates under O5(d) |
| Runs during sleep or dark wake, SSH sessions, LaunchDaemons, `pass`, `unsafe_file` | not tested; the README says so |
| A Keychain prompt after a CLI upgrade | not tested (O4(a)); the README asks for a hand check after each upgrade |
| The advertised-set warning, the Mac refusal at `list`, the destination-set check, the Mac push rule's three parts and the repair | gate steps 5.5 to 5.10; hermetic tests, including custom, negative and mirror refspecs, tags, an anonymous URL, a clone's default mapping (M28), two destinations aliasing each other at `list` and in a push batch, each local read failing, a direct-call replacement batch, which git never sends (M26), D2's two-pass fixed-point case with the Mac rule on, and an NFC/NFD pair's two `%+q` forms differing (M29) |
| The tracking-ref rewind of M24 through the helper | measured on a local remote only; gate step 5.9.2 checks that the refusal prevents it |
| Whether git auto-follows tags into `refs/tags/*` on an anonymous-URL fetch | not settled; the check counts advertised tags either way (§3) |
| `caffeinate -is` keeping a locked, lid-open Mac on mains power awake for the launchd step | gate step 6 |
| A local collision that arrives by a route other than the rename transition (for example a ref deleted locally and recreated) | the local-collision check covers it by construction [inference]; not separately gated |
| A fetch whose command-line refspec or `--refmap` writes into another namespace | outside the check; the README constrains it (§3) |
| A clone's new repository existing when git sends `list` | measured with a stand-in helper (M28); gate step 5.5 checks it with the real helper |
| A linked worktree on the Mac, where `GIT_DIR` names `.git/worktrees/<name>` | hermetic tests in the plan [untested]; the local reads fail closed, so the risk is a refusal, not a silent miss [review: Kimi] |
| The scheduled wrapper's 30-minute expiry, killing its push's process group | a plan test with a stand-in command that never exits (§4) |
| `nfd` with invalid UTF-8 bytes inside a run of marks | a test vector (§3) |
| A LaunchAgent push after a fresh login | not tested; the gate cannot log out mid-run |
| `git rev-parse --show-ref-format` on git 2.45 through 2.53 | documented from v2.44.0 (§3); run on 2.54.0 only |
| Whether Gatekeeper showed a dialog for M18 | not tested; the README's advice does not depend on it |
| Whether APFS confuses names that are canonical-caseless equivalent only through case folding (`ᾳ`, `αι`) | not tested; the rule refuses them anyway (§3) |
| Later releases' Mac builds | CI only, by MD2(d) and O4(a) |
| Intel Macs | out of scope |

End of the macOS port spec.
