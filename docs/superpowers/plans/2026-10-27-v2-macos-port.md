# git-remote-proton on macOS — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the v2 helper usable and safe on Craig's Apple Silicon Mac, built from source. That
means:

- the widened refusal (case **or** canonical equivalence) on every platform, at one pinned Unicode
  version;
- the Mac's read and push rules for the default `files` ref store;
- hermetic-test guards against a signed-in CLI;
- a macOS CI leg;
- the scheduled-push wrapper;
- a fifth drift leg;
- the documents and the macOS gate brief the approved spec calls for.

**Architecture:**

- **Cross-platform rule (`internal/repo`).** Generated Unicode 17.0.0 tables, built from committed
  inputs, feed two keys: `caseKey`, moved onto the pinned folding, and a new `canonKey =
  caseKey(nfd(s))`. `caseConflicts` reports a relation with each pair. Phase 2's rename pairing
  and both message texts follow.
- **Mac rules.** Pure checks live in `internal/repo`: advertised pairs, refspec mapping,
  destinations and their conflicts, and the push rule. Three local git reads go in
  `internal/gitcmd`, on a stdout-only runner. `cmd/git-remote-proton/main.go`:
  - captures whether git set `GIT_DIR`;
  - warns at `list` and `list for-push` on every platform;
  - on a Mac `files` repository, refuses at `list` and at `list for-push`;
  - hands `Push` the local side through a new entry point, `PushWithLocal`.
- **Process files:** CI YAML, the drift script and its Pester tests, a bash wrapper with its own
  test, and Markdown.
- **Release mechanics** (version bump, CHANGELOG flip, tag, the three live gates) follow
  `docs/releasing.md` after this plan, on Craig's word. They are not tasks here.

**Tech Stack:** Go (stdlib only, no cgo; language floor `go 1.22`), bash 3.2 (macOS's `/bin/bash`),
Pester 5+ with PSScriptAnalyzer, GitHub Actions (`macos-latest`, arm64).

**Spec:** `docs/superpowers/specs/2026-10-27-v2-macos-port-design.md` (approved by Craig
2026-10-27; rulings MD1–MD8 and O1–O6 are in its Decisions section). Executors read the spec's §1–§8,
its Decisions and its findings table (M0–M29) before starting. "M24", "§3" and the like mean the
spec's; "PM1" and the like mean this plan's measurements, below.

## Before execution: Craig's three answers (Task 0)

**Answered by Craig on 2026-10-27:** "Approve MA1 and MA2, install PowerShell 7 with Pester and
PSScriptAnalyzer on the Mac for the Pester runs, and use subagent-driven execution." All three
took the recommended answer. MA1 and MA2 are folded into the spec, in §3 and in a new
"Amendments" table under its Decisions. R1–R6 below were put to him the same day and not objected
to, so they stand as unopposed defaults, my calls.

The questions, as they were put:

1. **Amendment MA1: `remote.<name>.mirror` is not a fetch-destination source.**
   - **Before** (spec §3, rule (2)): "the remote's configured fetch refspecs (`remote.<name>.fetch`,
     positive and negative, with git's matching, and `remote.<name>.mirror`)".
   - **After:** "the remote's configured fetch refspecs (`remote.<name>.fetch`, positive and
     negative, with git's matching). `remote.<name>.mirror` is not read: git uses it for push only,
     and a mirror fetch is configured as the refspec `+refs/*:refs/*` [doc: `git help config`,
     git 2.54.0, read 2026-10-27]".
2. **Amendment MA2: Unicode publishes no checksums.**
   - **Before** (spec §3, "The inputs are committed"): "by a separate manual command that checks
     the new files against Unicode's published checksums".
   - **After:** "by a separate manual command. Unicode publishes no checksums for these files
     (2026-10-27: none in the 17.0.0 `ucd/` listing or either ReadMe), so the command checks the
     version line `CaseFolding.txt` and `NormalizationTest.txt` each open with and prints the
     SHA-256s it saw. The data is checked from two sides: `NormalizationTest.txt`'s Parts 1 and 2
     exercise every decomposition and combining class `UnicodeData.txt` supplies, and the
     containment test on a toolchain at the pinned version checks the folding."
   - `UnicodeData.txt` has no version line of its own; `NormalizationTest.txt` is its only
     independent check.
3. **How Pester runs.** This Mac has no `pwsh`, and Tasks 12 and 13 need RED and GREEN Pester runs
   plus PSScriptAnalyzer. The options:
   - **(a)** Craig approves installing PowerShell 7 on the Mac with Homebrew (`brew install --cask
     powershell`), with Pester 5 and PSScriptAnalyzer from the PowerShell Gallery, user-scoped;
   - **(b)** the runs go to CI, each needing Craig's word to push;
   - **(c)** the runs go to a paired PC session, which needs the branch pushed.

   The recommendation is (a): it keeps every RED run local and needs no push.

   *Correction, 2026-10-27, after the ruling:* "this Mac has no `pwsh`" was wrong; it was never
   measured. PowerShell 7.6.9 (Homebrew, linked at `/opt/homebrew/bin/pwsh`) and Pester 6.2.0
   (user-scoped) were installed on 2026-10-25. Only PSScriptAnalyzer 1.25.0 was installed under
   the ruling, user-scoped, from the PowerShell Gallery.

   **PM10, measured the same day:**
   - PSScriptAnalyzer runs clean with CI's settings over `install.ps1`, `GitProtonBackup`, `tests`
     and `scripts`.
   - `ReadmePins.Tests.ps1` passes (2 of 2).
   - `DriftCheck.Tests.ps1` passes 5 of 19. The other 14 fail on this Mac because its `New-Stub`
     writes Windows `.cmd` stubs (`DriftCheck.Tests.ps1:23-29`), which macOS cannot run ("No
     application knows how to open … helper-ok.cmd"). Task 12 makes the stubs portable.

The **measurements** these rest on, all local, with no account, on 2026-10-27 (git 2.54.0, go1.27.0,
`/bin/bash` 3.2.57), scripts kept under `.superpowers/mac-port/probe-state/`:

- **PM1: an "Everything up-to-date" push rewrites an aliasing tracking ref.** In a Mac `files`
  clone holding a stale loose `refs/remotes/origin/café` (NFD, at B), `git push origin
  refs/heads/café:refs/heads/café` (NFC, already equal to the remote's, at A) printed `Everything
  up-to-date`, and the NFD tracking ref then read A. git updates tracking refs for up-to-date
  refs, and sends the helper no push command for them [Codex, from `transport.c` and
  `transport-helper.c` at v2.54.0; reproduced here on a local remote].
- **PM2: fetch refspec shapes.**
  - `refs/heads/main:` and `refs/heads/main` fetch and write nothing.
  - An abbreviated source works (`heads/main:refs/remotes/o/main` wrote `refs/remotes/o/main`).
  - An abbreviated destination is expanded (`refs/heads/main:heads/x` wrote `refs/heads/x`).
- **PM3: colons in remote names.** `git remote add 'a:b'` fails with "'a:b' is not a valid remote
  name". A hand-written `[remote "a:b"]` is still looked up by name.
- **PM4: anonymous URLs have no remote config.** `git config --get 'remote.proton::/my-files/a.b.url'`
  and `--get-all '….fetch'` both exit 1 (unset).
- **PM5: linked worktrees read the same as the main repository.** `git -C .git/worktrees/<w>` gives
  the same `--show-ref-format`, `remote.origin.url` and `for-each-ref` names as the main
  repository's `.git`.
- **PM6: bash 3.2 can bound a push.** Under `set -m`, a background job gets its own process group,
  and its grandchild joins it. `kill -TERM -- -<pgid>` ended both, and the script went on.
- **PM7: vet checks the language floor.** go1.27.0's `go vet` includes `stdversion` ("report uses of
  too-new standard library symbols").
- **PM8: Hangul is in Part 1.** `NormalizationTest.txt` 17.0.0 has Parts 0–5, and Part 1 lists all
  11,172 Hangul syllables (U+AC00–U+D7A3, counted).
- **PM9: a named remote with no fetch refspec writes no refs.** `git fetch o` with `remote.o.url`
  set and no `remote.o.fetch` left the ref list unchanged in both a non-bare and a bare repository.
  It aims at FETCH_HEAD alone; the probe remote had no usable HEAD, so it wrote no FETCH_HEAD
  line.

**Refinements the spec allows.** Spec §3 says the trigger points and the refspec mapping are calls
under O3(d) that "a plan may refine … with a reason". This plan makes six, each marked as my call:

- **R1: the Mac refusal also runs at `list for-push`,** because of PM1, and only where PM1 can
  happen.
  - **Its trigger:** an advertised ref `R` whose exact name the local repository also holds, so a
    push by name (`--all`, matching, `git push <remote> R`) can find `R` up to date, and one of
    whose tracking refs conflicts with a local ref spelled differently.
  - **Why not every advertised ref:** git rewrites tracking refs only for the refs a push matches.
    Checking every advertised ref would lock a Mac clone out of every push as soon as its remote
    held a pair, against D2's ruling that a pair still takes pushes to its other refs, and would
    make gate steps 5.9.1 and 5.9.2 unreachable [review round 2: Gemini, DeepSeek].
  - **Its cost, stated in the README:** while such a ref stands, no push from that clone runs
    until the stale ref is deleted.
  - **What it does not see:** an up-to-date push to `R` from a differently named source given as
    exact bytes (`core.precomposeunicode=false`).
- **R2: phase 5 re-checks a Mac update** whose remote-half conflict a failed same-batch delete
  left standing [Codex].
- **R3: a D2 refusal caused only by a delete the Mac rule kept** carries a Mac message, not "refused
  on every platform" [Gemini].
- **R4: fetch refspecs must use full `refs/` names.** Any other form is refused, failing closed
  (PM2). A refspec with no destination writes nothing.
- **R5: a URL is anonymous when both `remote.<argv[1]>.url` and `remote.<argv[1]>.pushurl` are unset
  or empty** (PM3, PM4), rather than when argv[1] contains `:`. A push-only remote, configured
  with `pushurl` and `fetch`, keeps its fetch mapping, because git uses it for tracking refs
  [review round 3: Codex].
- **R6: the destination model is conservative** wherever git's own rules are wider than the
  refspecs suggest [review round 2: Codex, Kimi].
  - **Push tracking refs** take every positive mapping and ignore negative refspecs: git's push
    tracking lookup can still pick a positive wildcard a negative was meant to exclude.
  - **Fetch destinations** count every advertised tag, negated or not, because tag backfill runs
    without negative filtering.
  - **A fetch that prunes** may delete any local ref under a positive refspec's destination that
    the fetch does not write; each such ref is checked against the local refs.
    - The check runs only when pruning is configured, by `remote.<name>.prune` or else
      `fetch.prune`, which the helper can read.
    - A command-line `--prune` never reaches the helper. It joins command-line refspecs and
      `--refmap` among what rule (2) cannot see, and the README says so, as spec §3 does for
      those [review round 3: Kimi].
  - **No fetch refspec** gives the default mapping only while the repository holds no refs (a
    clone, M28). Otherwise it gives no tracking destinations (PM9).
  - **The tracking `HEAD`.** A clone writes `refs/remotes/<name>/HEAD`, and so does a fetch under
    `remote.<name>.followRemoteHEAD`. It is a destination wherever a positive spec maps
    `refs/heads/*` to `<p>/*`: `<p>/HEAD`. On default APFS it aliases a branch named `head`
    [review round 3: Codex].
  - **Symbolic local refs.** git follows a symbolic tracking ref when it updates one, so a
    destination that is a local symbolic ref adds its target as a destination too [review round 3:
    Codex].

  These rest on Codex's reading of git v2.54.0's source (`refspec.c`, `transport.c`,
  `builtin/fetch.c`, `builtin/clone.c`, `remote.c`), not on a probe, except PM9.

**Branch:** `worktree-macos-port`, a worktree branch, as the Linux port used (`worktree-linux-port`,
merged by PR #14; `git log --first-parent`).
- It starts at `origin/main` `fa8ce41`, which merged PR #16 on 2026-10-25, plus the spec,
  cherry-picked from local `main`'s `c6419a5`.
- PR #16 added 16 lines to `main.go`'s push arm, from `:537` on. Every `main.go` line this plan
  cites from `:537` down is therefore 16 lower than the current code: `warn` is `:760`, the
  poisoned path `:586`, and the `repo.Push` call `:606`.
- Cited lines are at `8848f41` unless a task says otherwise. Implementers locate code by the
  quoted content, not by number.
- PR #16 also opened `CHANGELOG.md`'s `## [Unreleased]` heading.

Commit per task, before any step that relies on CI. Pushing the branch
and opening a **draft** PR are gated actions, on Craig's word at Task 11 Step 4. Every later push
needs his word too. Task 15 requires passing CI on the exact final commit. Merge to `main` only on
Craig's word, after Task 15's stop.

## Global Constraints

- **Go:**
  - Code and tests must compile on Go **1.22**. No `t.Chdir`, no range-over-func, and no standard
    library symbol newer than 1.22.
  - Stdlib only. `go.mod` gains no `require`. The generator and the fetch command are `//go:build
    ignore` files.
  - `gofmt -l .` is empty. Generated Go is written through `go/format`.
- **After every task that touches Go**, on the Mac with go1.27.0, all four must be clean:

  ```bash
  ~/sdk/go1.27.0/bin/go vet ./...
  GOOS=linux GOARCH=amd64 ~/sdk/go1.27.0/bin/go vet ./...
  GOOS=windows GOARCH=amd64 ~/sdk/go1.27.0/bin/go vet ./...
  test -z "$(gofmt -l .)"
  ```

  `vet` compiles each target and runs `stdversion` against go.mod's `go 1.22` (PM7). This is the
  floor check; CI's 1.22 leg confirms it.
- **Tests:**
  - Every `go test` passes `-count=1`. **While this plan runs**, before the Mac adopts v2, every
    run on this Mac is preceded by `[ -z "$(command -v proton-drive)" ]`, and a failure is a STOP.
    After adoption the trap (Task 1) is what keeps a run hermetic; rule 12's PATH removal is the
    gates' stronger form (spec §5) [review round 2: Kimi].
  - Label each new test **RED** (it must fail before its task's change; a compile failure counts)
    or **GUARD**. For each RED test, record which assertion fired.
  - No test reaches the Proton account, writes outside a temp dir, or depends on a machine-wide
    process listing.
- **CLI pin:**
  - `transport.CertifiedCLI` stays `cli-drive@0.9.0+75612299`.
  - `GPB_UNCERTIFIED_CLI` is unchanged.
  - The helper gains no production environment variable. The wrapper's knobs (Task 10) are script
    settings.
- **Floors and assets:**
  - The git floor stays `git 2.45.0 or newer`.
  - The release keeps exactly five assets. No macOS asset, and no change to `release.yml`.
- **The Unicode pin is 17.0.0**, from `https://www.unicode.org/Public/17.0.0/ucd/`, measured on
  2026-10-27:

  | File | Bytes | SHA-256 |
  |---|---|---|
  | `UnicodeData.txt` | 2198209 | `2e1efc1dcb59c575eedf5ccae60f95229f706ee6d031835247d843c11d96470c` |
  | `CaseFolding.txt` | 87539 | `ff8d8fefbf123574205085d6714c36149eb946d717a0c585c27f0f4ef58c4183` |
  | `NormalizationTest.txt` | 2827429 | `5019ffd530751a741900c849c0e010332f142a3612234639bd200b82138a87db` |

  A fresh fetch that differs in any value is a STOP.
- **The certified macOS CLI.** It is 0.9.0, platform `macos/arm64`, downloaded from
  `https://proton.me/download/drive/cli/0.9.0/darwin-arm64/proton-drive`. Spec M2 recorded its
  SHA-512 as
  `3f4125de5e41dcc7f4e0e07e8d60737aa18f35c6ecc1ba3fce47ea97b331857eefaf88e74671135af57122d4cb5439b817ec528f8df383550ac7810aeaf0217b`.
  To record it:
  - Copy it from the **live feed**.
  - Require it to equal M2's value.
  - Require it to equal `shasum -a 512` of a fresh download into a scratch folder.

  Any difference is a STOP.
- **When the Mac rules apply.** All three conditions must hold:
  - `env.goos == "darwin"` (tests set it, so the rules run on every CI platform);
  - git set `GIT_DIR`;
  - the ref format is `files`.

  The warning has no such condition: it applies everywhere.
- **Escape form:** `%+q` (`strconv.QuoteToASCII`), never `%q` (M29).
- **Errors to stderr** go through `warn(err error)` (`main.go:744`): `warn(fmt.Errorf(...))`, never
  a string.

## Review Focus

1. **Ref names holding invalid UTF-8**, which git accepts.
   - Nothing panics.
   - Distinct invalid bytes never compare equal.
   - No mark is reordered across one.
   - Owned by Tasks 4 and 5.
2. **A remote named with dots** (`back.up`), read back exactly. **A hand-written remote whose name
   holds `:`** is treated as a named remote, because its URL is configured (R5, PM3). Owned by
   Tasks 6 and 8.
3. **A `--bare` or `--mirror` clone of a pair**, into the default ref store, is refused at `list` by
   rule (1). Owned by Task 8.
4. **A linked worktree.** The three reads give the main repository's values (PM5). Owned by Task 6.
5. **A Mac push touching refs with no tracking mapping**: tags, notes, refs outside the refspecs, and
   an anonymous URL. The local half is skipped, the remote half still applies, and nothing panics.
   Owned by Task 9.

---

### Task 1: Hermetic guards: the shim check, the trap, rule 12 (§5, O6(c))

**Files:**
- Create: `internal/testtrap/trap.go`, a stdlib-only leaf package. It cannot live in
  `internal/testcli`, which imports `internal/transport`, whose in-package tests must call it.
- Modify: `cmd/git-remote-proton/shim_test.go` (`TestMain` at `:40`, `buildShimOnPath` at
  `:78-117`), `internal/transport/cli_test.go` (`TestMain` at `:102`)
- Modify: `docs/research/gates/brief-checklist.md` (rule 12), `AGENTS.md` (one Repo Gotchas line)
- Test: `internal/testtrap/trap_test.go`, `cmd/git-remote-proton/shim_test.go`

**Which packages need the trap.** Exactly two: the packages that can reach `transport.NewCLI("")`
(the PATH lookup) are `cmd/git-remote-proton` (`main.go:129`, `:191`; `shim_test.go:229`, `:264`)
and `internal/transport` (`contract_test.go:617`) [grep at `8848f41`]. Every other `NewCLI` call
names an explicit executable. Re-run that grep at the start of the task; a new caller in a third
package gets the trap too.

**Interfaces:**
- Produces, in `internal/testtrap`:
  - `const DirEnv = "GPB_TEST_TRAP_DIR"`.
  - `const Message = "a hermetic test reached proton-drive without a shim"`.
  - `func Install() (dir string, err error)`. It copies `os.Executable()` into a new temp dir as
    `proton-drive` (`.exe` on Windows), sets `DirEnv`, and prepends the dir to `PATH` with
    `os.Setenv`.
  - `func Active() bool`. It is true when `os.Executable()`'s directory and `os.Getenv(DirEnv)`
    are the same directory by `os.SameFile` on their `os.Stat`s. A string comparison would trip
    on `/var` → `/private/var` (PR #15). A trap is identified by where it runs, so no role
    variable can be mistaken for it.
  - `func Run(stderr io.Writer) int`, which writes `Message` and a newline and returns `3`.
- In each `TestMain`:
  1. First, `if testtrap.Active() { os.Exit(testtrap.Run(os.Stderr)) }`, ahead of every role
     dispatch.
  2. Then, unless `os.Getenv("GPB_LIVE_ACCOUNT") == "1"`, `Install()`, exiting 2 on error.
  3. After `m.Run()`, an explicit `os.RemoveAll(dir)` before `os.Exit`, because defers do not run
     on `os.Exit`. Its error is ignored: a Windows file lock left by a lingering child must not
     fail the run.

- [ ] **Step 1: Tests.**
  - `TestTrapIsFoundFirstAndFails` (RED, compile failure). `trap_test.go`'s own `TestMain` makes
    the `Active()` dispatch, calls `Install` once, and removes the dir at exit. The test then finds
    `exec.LookPath("proton-drive")` inside `os.Getenv(DirEnv)`. Running it exits 3 with `Message`
    on stderr. The test never calls `Install` a second time, so no dir leaks and `DirEnv` stays
    accurate [review round 3: DeepSeek].
  - `TestShimResolvesBeforeTheTrap` (RED). After `buildShimOnPath`,
    `exec.LookPath("proton-drive")` is the shim's path.
  - `TestNoShimMeansTheTrap` (RED). `transport.NewCLI("").Version()` returns an error whose
    `Error()` contains `testtrap.Message` (`Version` folds the CLI's output into its error,
    `cli.go:636-638`).
- [ ] **Step 2: Run them and see them fail.** `go test -count=1 ./internal/testtrap
  ./cmd/git-remote-proton -run 'Trap|ShimResolves'`.
- [ ] **Step 3: Implement.** Write `trap.go` and both `TestMain` changes. `buildShimOnPath` gains the
  LookPath assertion after its `t.Setenv("PATH", …)`, failing with both paths.
- [ ] **Step 4: Run the suite and the Global vet checks.** Expected: PASS (525 plus the new tests;
  the same 5 skips).
- [ ] **Step 5: Docs.**
  - Rule 12, in rule 11's style: a hermetic test run on a Mac never resolves a real
    `proton-drive`. A gate's hermetic run also takes the real CLI off PATH. `sandbox-exec` may be
    added, never instead, because it leaves files and the Keychain reachable and is deprecated. A
    rule-10 audit is not a hermetic run.
  - The AGENTS.md line, as spec §5 gives it.
- [ ] **Step 6: Commit.** `test: trap a hermetic run that reaches proton-drive without a shim (rule 12, O6(c))`

### Task 2: The pinned Unicode inputs, their generator and the tables (§3 "The tables", O1(iv), MA2)

**Files:**
- Create: `internal/repo/unicodedata/17.0.0/UnicodeData.txt`, `CaseFolding.txt` and
  `NormalizationTest.txt`, as committed bytes.
- Create: `internal/repo/gen_unicode.go` (`//go:build ignore`, `package main`), the offline
  generator.
- Create: `internal/repo/fetch_unicode.go` (`//go:build ignore`), the pin-moving fetch. It is the
  only code that touches the network, and it is run by hand.
- Create: `internal/repo/unicode_tables.go`, generated. Its first line is `// Code generated by
  gen_unicode.go; DO NOT EDIT.`
- Create: `internal/repo/canon.go`, holding the directive `//go:generate go run gen_unicode.go
  -version 17.0.0` and the accessors below.
- Modify: `.gitattributes`: `internal/repo/unicodedata/** -text`.
- Test: `internal/repo/canon_test.go`

**Interfaces:**
- `gen_unicode.go -version <v> [-dir <path>] [-out <path>]`:
  - It reads all three files from `-dir`, which defaults to `unicodedata/<v>/`. It needs
    `UnicodeData.txt` and `CaseFolding.txt` for the tables, and `NormalizationTest.txt` for its
    checksum.
  - It writes `-out`, default `unicode_tables.go`, with `const unicodeVersion = "<v>"` and the three
    input checksums (`unicodeDataSHA256`, `caseFoldingSHA256`, `normalizationTestSHA256`).
  - Table representation is the builder's call (sorted slices or maps). The output must be
    byte-stable: entries sorted, so the same inputs give the same file.
  - It exits non-zero naming the failed check when any of these does not hold:
    1. no code point has both a C and an S entry (measured true for 17.0.0);
    2. for every code point with no canonical decomposition, its fold target has none either:
       folding never brings back a decomposable character, which is why `canonKey` needs no second
       decomposition (§3, M23);
    3. every code point whose combining class differs from its fold target's has the highest class
       any code point has, and its target has class 0. The generator prints the fold classes it
       found; for 17.0.0 that is exactly one, the iota class (M23);
    4. no canonical decomposition produces or consumes U+002F `/`.
- `fetch_unicode.go -version <v>`:
  - It downloads the three files into `unicodedata/<v>/` over HTTPS.
  - It requires the first lines `# CaseFolding-<v>.txt` and `# NormalizationTest-<v>.txt`.
  - It prints each file's size and SHA-256 for review (MA2).
  - Moving the pin means running it, then `go generate` with the new `-version` in the directive,
    then deleting the old directory.
- Produces, in `canon.go`:
  - `func decomposeRune(r rune) string`: the full recursive canonical decomposition, excluding
    Hangul syllables; `""` when there is none.
  - `func combiningClass(r rune) uint8`: 0 when absent.
  - `func foldRune(r rune) rune`: the C or S target, or `r` itself.

- [ ] **Step 1: Fetch and check the inputs.** Download them with `curl` from the Global Constraints
  URLs into `internal/repo/unicodedata/17.0.0/`, and check each byte count and `shasum -a 256`
  against the table. Any difference is a STOP. This is the plan's only networked step, and it
  needs no account. `fetch_unicode.go` is for later pin moves [review round 3: Kimi].
- [ ] **Step 2: Write `canon_test.go` (RED, compile failure).**
  - `TestUnicodeInputsMatchTheRecordedChecksums`: each committed file's SHA-256 equals both its
    generated constant and the table value.
  - `TestFoldRuneSamples`: `'A'→'a'`, `'\u212A'→'k'`, `'\u0345'→'\u03B9'`, `'\u1FBE'→'\u03B9'`,
    `'\u00DF'→'\u00DF'` (F only).
  - `TestDecomposeRuneSamples`: `'\u00E9'→"e\u0301"`, `'\u1FBE'→"\u03B9"`, `'\u01D5'→"U\u0308\u0304"`
    (recursive), `'a'→""`, `'\uFB01'→""` (compatibility only).
  - `TestCombiningClassSamples`: U+0301 is 230, U+0327 is 202, U+0345 is 240, `a` is 0.
- [ ] **Step 3: Write the generator, generate, and pass.** Run `cd internal/repo && go generate .`,
  then `go test -count=1 -run 'Unicode|FoldRune|DecomposeRune|CombiningClass' ./internal/repo`.
- [ ] **Step 4: Prove determinism and fail-closed.**
  - Generate twice with `-out` into two temp files and `cmp` them.
  - Copy all three inputs to a scratch dir, and edit that copy's `CaseFolding.txt` to give one code
    point both C and S. Run the generator with `-dir` there and `-out` to a temp file. It must exit
    non-zero naming check 1. Record the output.
- [ ] **Step 5: `.gitattributes`.** Add the line. Then `git add --renormalize
  internal/repo/unicodedata` must change nothing.
- [ ] **Step 6: Commit,** then `git clone` the branch into a scratch folder and check the three
  SHA-256s there. Commit message: `feat(repo): pinned Unicode 17.0.0 inputs, offline generator and
  tables (O1(iv))`

### Task 3: `caseKey` on the pinned folding, and the containment test (§3 "No refusal is lost")

**Files:**
- Modify: `internal/repo/casefold.go`. In `caseKey` (`:27-41`), use `foldRune(r)` in place of
  `minFold(r)`. Delete `minFold` (`:46-54`) and the `unicode` import. The doc comment (`:17-26`)
  becomes: two strings share a key "exactly when Unicode simple case folding at the pinned version
  (`unicodeVersion`) calls them equal". Keep its invalid-byte paragraph.
- Modify: every other comment that states the old contract. `grep -rn 'EqualFold\|simple folding'
  --include='*.go' .` finds them; one is `push.go`'s phase-2 comment, "(caseConflicts; Unicode
  simple folding, as strings.EqualFold)". The task report lists each hit and what it became
  [review round 3: DeepSeek].
- Test: `internal/repo/casefold_test.go`

**Interfaces:**
- Consumes: `foldRune` and `unicodeVersion`.
- Produces: `caseKey(s string) string`, with the same signature. The iota class now maps to
  U+03B9.

- [ ] **Step 1: Tests.**
  - Rename `TestCaseKeyMatchesEqualFold` to `TestCaseKeyFoldsByThePinnedTable` (GUARD), with
    explicit expectations: `main`/`Main`, `Über`/`über` and `"K"`/`"\u212A"` (Kelvin sign) share a key; `main`/`mainx` and
    `ß`/`SS` do not. There is no `strings.EqualFold` oracle.
  - `TestCaseKeyIotaClassFoldsToTheStarter` (RED): U+0345, U+0399, U+03B9 and U+1FBE all give
    `"\u03B9"`.
  - `TestSimpleFoldOrbitsLieInsideGeneratedClasses` (RED). It parses the committed
    `UnicodeData.txt` into the set of code points assigned in 17.0.0, expanding `First`/`Last`
    ranges. For every assigned `r`:
    - each member of its `unicode.SimpleFold` orbit that is also assigned has the same `foldRune`;
    - when `unicode.Version == unicodeVersion`, the converse holds too, so the classes are equal.

    It logs `toolchain Unicode <v>, pin <p>` and which branch ran. It also logs how many pinned
    classes span more than one toolchain orbit among the characters both versions assign. Those
    are classes the pin merged, so a pair the toolchain-built release accepted would now be
    refused. O1(iv) accepted that, but the count goes in the task report from CI's Go 1.22 leg
    [review round 2: DeepSeek]. A failure is investigated, never silenced (§3).
- [ ] **Step 2: Run them and see them fail.** `go test -count=1 -run 'CaseKey|SimpleFoldOrbits'
  ./internal/repo`.
- [ ] **Step 3: Make the change.**
- [ ] **Step 4: Run the package and the Global vet checks.** Expected: PASS. Every existing
  case-variant `Push` test passes unchanged.
- [ ] **Step 5: Commit.** `refactor(repo): caseKey folds by the pinned Unicode table (O1(iv))`

### Task 4: `nfd` and `canonKey` (§3)

**Files:**
- Modify: `internal/repo/canon.go`
- Test: `internal/repo/canon_test.go`

**Interfaces:**
- Produces:
  - `func nfd(s string) string`. It applies `decomposeRune` per rune, decomposes Hangul syllables
    U+AC00–U+D7A3 arithmetically, then reorders canonically: a stable sort by `combiningClass`
    within each run of non-starters. An invalid UTF-8 byte is copied unchanged and counts as a
    starter.
  - `func canonKey(s string) string { return caseKey(nfd(s)) }`, as the spec fixes it.

- [ ] **Step 1: Tests (RED, compile failure).**
  - `TestNFDMatchesNormalizationTest`:
    - every data line of every part satisfies `nfd(c1) == nfd(c2) == nfd(c3) == c3` and `nfd(c4)
      == nfd(c5) == c5`;
    - every code point assigned in 17.0.0 that Part 1 does not list satisfies `nfd(string(r)) ==
      string(r)`. The file's own header (lines 36–40) states this invariant for every conformant
      implementation, and Part 1 lists all the Hangul syllables (PM8) [review round 3: DeepSeek].

    Record the line count read.
  - `TestCanonKeySharedByEquivalentStrings`: on every line, `canonKey(c1) == canonKey(c2) ==
    canonKey(c3)` and `canonKey(c4) == canonKey(c5)`.
  - `TestCanonKeyIgnoresCompatibilityMappings`:
    - explicitly, `canonKey("\uFB01") != canonKey("fi")` and `canonKey("Ω") ==
      canonKey("ω")` (Ohm is canonical, so it shares a key);
    - over the file, on every line where `c3 != c5` and `caseKey(c3) != caseKey(c5)`,
      `canonKey(c1) != canonKey(c4)`.

    Record how many lines the second form exercised.
  - `TestCaseFoldingVariantsShareBothKeys`: for every C and S line of `CaseFolding.txt`, the code
    point and its target share `caseKey` and `canonKey`.
  - `TestCanonKeyIotaExamples` (spec §3's three strings):
    - `"a\u03B9\u0301"` and `"a\u0345\u0301"` share `caseKey` only;
    - `"a\u0301\u03B9"` and `"a\u0345\u0301"` share `canonKey` only;
    - `"a\u03B9\u0301"` and `"a\u0301\u03B9"` share neither.
  - `TestNFDHangul`: `nfd("\uD55C") == "\u1112\u1161\u11AB"`.
  - `TestNFDTreatsInvalidBytesAsStarters` (Review Focus 1): `"x\u0301\u0327"` gives
    `"x\u0327\u0301"`; `"x\u0301\xff\u0327"` is unchanged byte for byte; `canonKey("a\xffb") !=
    canonKey("a\xfeb")`; nothing panics.
- [ ] **Step 2: Run them and see them fail.** `go test -count=1 -run 'NFD|CanonKey|CaseFolding'
  ./internal/repo`.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run the package and the Global vet checks.** Expected: PASS. Note the package's
  wall time beside Task 3's; the Windows leg already takes about 590 s for `internal/repo` (M0).
- [ ] **Step 5: Commit.** `feat(repo): canonical decomposition and canonKey at Unicode 17.0.0`

### Task 5: Both relations in the create rule, its messages and the rename pairing (§3)

**Files:**
- Modify: `internal/repo/casefold.go` (`caseConflict`, `caseConflicts` and `caseConflictMessage`,
  `:77-146`)
- Modify: `internal/repo/push.go` (phase 2's rename pairing, `:400-427`; phase 5 is unchanged in
  shape, `:600-631`)
- Test: `internal/repo/casefold_test.go`, `internal/repo/repo_test.go`

**Interfaces:**
- Produces, in `casefold.go`:
  - `type relation uint8` with `relCase = 1` and `relCanon = 2`, and a `String()` method returning
    `"letter case"` or `"Unicode normalisation"`.
  - `type caseConflict struct{ Ref, Ours, Theirs string; Rel relation }`.
  - `caseConflicts(set map[string]bool, candidates []string) map[string][]caseConflict` keeps its
    signature. It indexes both keys of every prefix. It walks a candidate's prefixes shallowest
    first, and at each depth checks the case index before the canon index. The first hit per `Ref`
    wins. Equal spellings never conflict, and results are sorted by `Ref`.
  - `caseConflictMessage(dst, cs)` returns exactly `<dst> conflicts with <item>[, <item>…]: <harm>
    (README, Honest limits), so this create is refused on every platform`.
    - Each item is `<Ref> (<Ours> vs <Theirs>, by <Rel>: <%+q Ours> vs <%+q Theirs>)`.
    - `<harm>` is `a Windows or Mac clone would mishandle the pair` if any item is `relCase`, and
      `a Mac clone would mishandle the pair` otherwise.
    - Phase 5 appends `failedDeleteClause` unchanged.
  - `func formatItem(c caseConflict) string` formats one item as above. `caseConflictMessage`
    joins its items with `, `. Task 9 joins its own, adding notes per item.
- Produces, in `push.go` phase 2:
  - Refused creates are indexed in two maps, `byCase[caseKey(x)]` and `byCanon[canonKey(x)]`, each
    keeping its smallest `x`.
  - A delete `D` pairs with `X = min(byCase[caseKey(D)], byCanon[canonKey(D)])`, over whichever
    exist.
  - The message is exactly `<D>: delete refused because it pairs with the refused <kind> rename to
    <X> (<%+q D> to <%+q X>); the rename was not applied`, where `<kind>` is `case-changing` when
    `caseKey(D) == caseKey(X)` and `normalisation-changing` otherwise.

- [ ] **Step 1: `casefold_test.go`.**
  - `TestCaseConflictsShapes` (GUARD): the three F13 shapes, now with `Rel == relCase`.
  - `TestCaseConflictsFindsNormalisationPairs` (RED): NFC against NFD `café` gives the prefix pair
    `heads/café`, and `grp-é/a` against `grp-é/b` gives `heads/grp-é`; both `relCanon`.
  - `TestCaseConflictsReportsTheShallowestPairUnderEitherRelation` (RED):
    `refs/heads/grp-e\u0301/Main` against `refs/heads/grp-é/main` gives `relCanon` at the
    `grp` prefix.
  - `TestCaseConflictsPrefersCaseAtEqualDepth` (GUARD): `Main` against `main` gives `relCase`.
  - `TestCaseConflictsIotaPairsByTheirRelation` (RED): the three iota strings as branch names.
  - `TestCaseConflictMessageNamesEachRefWithItsOwnPair`, rewritten.
    - The NFC/NFD message contains `"heads/caf\u00e9"` and `"heads/cafe\u0301"` as escaped ASCII,
      and the two forms differ (M29); a `%q` build fails this.
    - The harm clause follows the relation.
  - `TestCaseConflictsKeepsInvalidBytesDistinct` (GUARD; Review Focus 1).
- [ ] **Step 2: `repo_test.go`.** Use `seedCaseRemote`, `assertCaseRemote`, `assertNoPack` and
  `assertCaseRefused`, with the last updated to the new text.
  - `TestPushRefusesAnNFDCreateBesideNFC` (RED).
  - `TestPushRefusesANewRefUnderANormalisationDirectoryPair` (RED).
  - `TestPushNormalisationChangingRenameIsRefusedWhole` (RED):
    - the remote holds `main` (HEAD), `café` and `CAFÉ`, all NFC;
    - the batch deletes `CAFÉ` and creates `CAFE\u0301`;
    - both are refused, the delete's `Err` contains `normalisation-changing rename`, and `CAFÉ`
      survives.
  - `TestPushRenameDeletePairsWithTheSmallestCreateUnderEitherKey` (RED): the delete
    `a\u0345\u0301` with refused creates `a\u0301\u03B9` (canon) and `a\u03B9\u0301` (case) names
    `a\u0301\u03B9`, the smaller, with `normalisation-changing`.
  - `TestPushRenameDeleteNamesTheSmallestRefusedCreate`: the text updated, otherwise GUARD.
  - `TestPushPhase5RefusesACreateWhoseVariantDeleteFailed`: the text updated, plus a canonical-only
    twin (RED).
  - `TestPushNormalisationRefusalTextIsOrderIndependent` (RED): NFC, NFD and `Café` in all 6
    orders.
  - **Every existing test that asserts the old texts.** Find them with `grep -n 'conflicts by
    letter case\|case-changing rename' internal/repo/repo_test.go`. Round 2 counted 13 [Gemini],
    from `TestPushRefusesACaseVariantCreateBesideAnExistingRef` to
    `TestPushCreateBeneathAKeptRefIsRefusedInTheSamePass`. Each is updated to the new text, and its
    outcome is unchanged.
- [ ] **Step 3: Run them and see them fail.** `go test -count=1 -run
  'Case|Normalisation|NFD|Rename|Phase5' ./internal/repo`.
- [ ] **Step 4: Implement.** Phase 5 needs no new code.
- [ ] **Step 5: Run the suite and the Global vet checks.** Expected: PASS.
- [ ] **Step 6: Commit.** `feat(repo): refuse canonically equivalent creates on every platform (MD6(a2))`

### Task 6: Three local git reads on a stdout-only runner (§3, MA1, R5)

**Files:**
- Create: `internal/gitcmd/local.go`
- Test: `internal/gitcmd/local_test.go`

**Interfaces:**
- Produces:
  - `func gitStdout(gitDir string, args ...string) (stdout []byte, stderr string, code int, err
    error)`. It runs `git -C gitDir args…` with the same environment scrub and `WaitDelay` as
    `git()` (`gitcmd.go:69-101`), but captures stdout and stderr separately and trims nothing.
    `git()`'s `CombinedOutput` and `TrimSpace` would mix a stderr warning into ref names, and could
    strip a name's trailing bytes [Gemini]. An `exec.ErrWaitDelay` is returned as an error, never
    as complete output, as `revListWithAlt` and `ShowIndex` already treat it (`gitcmd.go:480-486`,
    `:757-760`): a grandchild holding the pipe can leave stdout truncated, and a truncated ref list
    would fail open [review round 2: Gemini].
  - `func RefFormat(gitDir string) (string, error)`. It reads `rev-parse --show-ref-format`; stdout
    minus one trailing `\n` must be `files` or `reftable`. Anything else, or a non-zero exit, is an
    error naming the command and git's stderr.
  - `type RemoteCfg struct{ URL, PushURL string; Fetch []string; Prune bool }`.
  - `func RemoteConfig(gitDir, name string) (RemoteCfg, error)`.
    - It makes **one** call, `config -z --get-regexp '^(remote\.<name>\.(url|pushurl|fetch|prune)|fetch\.prune)$'`,
      with `<name>` regex-escaped. It parses the NUL-separated `key\nvalue` records.
    - `Prune` is `remote.<name>.prune` if set, else `fetch.prune`, read as git's booleans.
    - Exit 1 means none of the keys is set (a zero value, no error). Any other non-zero exit is an
      error.
    - `remote.<name>.mirror` is not read (MA1).
  - `func LocalRefs(gitDir string) (map[string]string, error)`. It reads `for-each-ref
    --format=%(refname)%00%(symref)` and splits stdout on `\n` only. Each name maps to its symbolic
    target, or `""` for a direct ref. Bytes are unchanged.

- [ ] **Step 1: Tests (RED, compile failure).** Each uses temp repositories with repo-scoped
  config.
  - `TestRefFormatReadsFilesAndReftable`.
  - `TestRefFormatFailsOnANonRepository`.
  - `TestRemoteConfigReadsADottedName` (Review Focus 2): a remote `back.up` with two refspecs and a
    negative one, read back exactly, in order.
  - `TestRemoteConfigUnsetIsNotAnError`, which includes the anonymous key
    `remote.proton::/my-files/a.b.url` (PM4) and a remote whose `url` is set but empty (both give
    `""`).
  - `TestGitStdoutTreatsWaitDelayAsAnError`: a command whose grandchild holds stdout open past
    `waitDelay` returns an error.
  - `TestRemoteConfigFailsOnABrokenConfigFile`.
  - `TestRemoteConfigReadsPushURLAndPrune`: `pushurl` with no `url` is read, and `Prune` follows
    `remote.<name>.prune` over `fetch.prune`.
  - `TestLocalRefsKeepsNFDBytes`: a `reftable` repository holding both `café` spellings.
  - `TestLocalRefsReportsSymbolicTargets`: `refs/remotes/o/HEAD` maps to `refs/remotes/o/main`.
  - `TestLocalRefsIgnoresStderr`: a repository with a dangling symbolic ref, so that git warns on
    stderr. The warning never appears among the names.
  - `TestLocalReadsInALinkedWorktree` (Review Focus 4, PM5): `gitDir` is `.git/worktrees/<w>`. All
    three reads equal the main repository's.
- [ ] **Step 2: Run them and see them fail.** `go test -count=1 -run
  'RefFormat|RemoteConfig|LocalRefs|LinkedWorktree|GitStdout' ./internal/gitcmd`.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run them, then the Global vet checks.** Expected: PASS.
- [ ] **Step 5: Commit.** `feat(gitcmd): ref format, remote config and local ref reads`

### Task 7: The pure read checks (§3, R1, R4)

**Files:**
- Create: `internal/repo/readcheck.go`
- Test: `internal/repo/readcheck_test.go`

**Interfaces:**
- Produces:
  - `type RefPair struct{ A, B string; Rel relation }`, with `A < B`.
  - `func AdvertisedPairs(refs map[string]string) []RefPair`. It finds each unordered pair of
    advertised refs conflicting under either relation, reports it once, and sorts by `(A, B)`.
  - `func PairLines(pairs []RefPair, refs map[string]string) string`. It returns one line per pair:
    `  <A> (<oid A>, <%+q A>) and <B> (<oid B>, <%+q B>): by <Rel>`.
  - `func PairWarning(pairs []RefPair, refs map[string]string) string`. It returns `warning: this
    remote holds refs that differ only by letter case or Unicode normalisation; see README, Honest
    limits:` followed by `PairLines`, or `""` for no pairs.
  - `type Refspec struct{ Src, Dst string; Force, Negative, NoDst bool }`.
  - `func ParseFetchRefspecs(values []string) ([]Refspec, error)` (R4, PM2). It accepts:
    - `[+]<src>:<dst>`;
    - `[+]<src>` and `[+]<src>:` (both `NoDst`, writing nothing);
    - `^<src>` (negative).

    Every `src` and every non-empty `dst` must begin `refs/`, with exactly one `*` on each side or
    none. Anything else is an error naming the value. That includes `HEAD`, abbreviated names
    (git expands them, PM2) and an empty string.
  - `func DefaultFetchRefspecs(remoteName string) []Refspec` returns
    `+refs/heads/*:refs/remotes/<name>/*`.
  - `func MapRef(specs []Refspec, ref string) []string`, the fetch mapping. A ref matched by any
    negative spec maps to nothing. Otherwise each matching positive spec with a destination yields
    it, by substitution or literally. Results are sorted and deduplicated.
  - `func PushTrackingRefs(specs []Refspec, ref string) []string`, the push mapping. It is like
    `MapRef` but **ignores negative specs** (R6).
  - `func FetchDestinations(advertised map[string]string, specs []Refspec) map[string]string`. It
    maps each destination to its source (the smallest, if several). The destinations are:
    - every `MapRef` result;
    - **every** advertised `refs/tags/*` name, mapped to itself, negated or not (R6);
    - `<p>/HEAD`, with source `HEAD`, for each positive spec mapping `refs/heads/*` to `<p>/*`.
      That covers the tracking HEAD a clone or `followRemoteHEAD` writes (R6) [review round 3:
      Codex].
  - `func ExpandSymrefs(dests map[string]string, symrefs map[string]string) map[string]string`. It
    adds, for each destination that is a local symbolic ref, its target, with the same source,
    repeating until no new target appears (R6) [review round 3: Codex].
  - `func PruneCandidates(specs []Refspec, dests map[string]string, local map[string]bool)
    map[string]string`. It maps to `""` each local ref that a positive spec's destination pattern
    (or literal destination) covers and that is not in `dests`: the refs a pruning fetch could
    delete (R6). The empty source is what `DestLines` reads as "may be pruned" [review round 3:
    Gemini, DeepSeek].
  - `type DestConflict struct{ Dest, Src, Other, OtherSrc string; C caseConflict }`. `OtherSrc` is
    non-empty only when `Other` is itself a destination of this fetch [review round 3: Gemini].
  - `func DestinationConflicts(dests map[string]string, local map[string]bool) []DestConflict`. It
    runs `caseConflicts` over `local ∪ keys(dests)` with the sorted destinations as candidates, and
    returns one entry per unordered pair that has a destination as one member, sorted. Pairs among
    local refs alone are not reported.
  - `func UpToDateTracking(advertised map[string]string, specs []Refspec, local map[string]bool)
    map[string]string` (R1). It maps `T` to `R` for each advertised `R` that `local` also holds by
    the exact same name, over `PushTrackingRefs(specs, R)`.
  - `func LocalConflicts(dests map[string]string, local map[string]bool) []DestConflict`. It is the
    same over `local` minus `keys(dests)`, so a candidate is checked against the other local refs
    alone, and `OtherSrc` is always empty.
  - `func MergeConflicts(a, b []DestConflict) []DestConflict`. It drops any pair in `b` already
    reported in `a` as an unordered `{Dest, Other}`, so the fetch check's destination and prune
    passes never report a pair twice. The result is sorted [review round 3: Gemini].
  - `func DestLines(cs []DestConflict, refs map[string]string) string`. It returns one line per
    conflict: `  <Dest> (<origin>) and <Other> (<other>) (<%+q Dest> vs <%+q Other>): by <C.Rel>`.
    - `<origin>` is `from <Src>, <oid>` for a destination with a source, or `may be pruned` for an
      empty source.
    - `<other>` is `also written by this fetch, from <OtherSrc>` when `OtherSrc` is set, and
      otherwise `local` [review round 2: DeepSeek].

- [ ] **Step 1: Tests (RED, compile failure).**
  - `TestAdvertisedPairsNamesEachPairOnce`, `TestAdvertisedPairsCountsDirectoryPairs`, and
    `TestPairLinesShowObjectIDsAndEscapeForms`.
  - `TestParseFetchRefspecs`.
    - Valid: `+refs/heads/*:refs/remotes/o/*`, `refs/heads/main:refs/remotes/o/main`,
      `refs/heads/main` and `refs/heads/main:` (both `NoDst`), and `^refs/heads/tmp/*`.
    - Errors: `heads/main:refs/remotes/o/main`, `refs/heads/main:heads/x`, `HEAD:refs/remotes/o/HEAD`,
      `refs/heads/*:refs/remotes/o/x`, `refs/*/a/*:refs/x/*`, and `""`.
  - `TestMapRef`: substitution; an exact spec; a negative beating a positive; two destinations; a
    `NoDst` spec giving nothing.
  - `TestFetchDestinationsIncludesEveryTag`: a tag a negative spec matches still counts (R6).
  - `TestPushTrackingRefsIgnoresNegatives`: `+refs/heads/*:refs/remotes/r/*` with
    `^refs/heads/café` still maps `café` (R6).
  - `TestPruneCandidates`: under `+refs/heads/tmp/*:refs/remotes/r/Old/*`, a local
    `refs/remotes/r/Old/a` with no `tmp/a` advertised is a candidate. A local ref outside every
    destination pattern is not [Codex].
  - `TestFetchDestinationsIncludesTheTrackingHEAD`: under the default spec, `refs/remotes/o/HEAD`
    is a destination. A remote branch `head` then conflicts with it under the case relation
    [Codex].
  - `TestExpandSymrefsAddsTargets`: a destination `refs/remotes/o/foo` that is a local symbolic ref
    to `refs/heads/bar` adds `refs/heads/bar` [Codex].
  - `TestDestLinesNamesBothSources`: two aliasing destinations name each one's source (`OtherSrc`),
    and a prune candidate reads `may be pruned` [Gemini].
  - `TestMergeConflictsDropsDuplicates`.
  - `TestDestinationConflicts`:
    - a destination against a local ref spelled differently is reported, with its `Src`;
    - the same bytes are not;
    - two destinations against each other are reported. This is Codex's case:
      `refs/heads/one/a` and `refs/heads/two/b` mapped to `refs/remotes/r/café/a` (NFC) and
      `refs/remotes/r/cafe\u0301/b`;
    - a local-only pair is not reported;
    - the result is deterministic over 50 orders.
  - `TestLocalConflictsIgnoresDestinationPairs`: Codex's case gives nothing there.
- [ ] **Step 2: Run them and see them fail.** `go test -count=1 -run
  'AdvertisedPairs|PairLines|Refspec|MapRef|Destinations|Conflicts' ./internal/repo`.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run the package and the Global vet checks.** Expected: PASS.
- [ ] **Step 5: Commit.** `feat(repo): pure checks for advertised pairs and ref destinations`

### Task 8: `main.go`: the warning, and the Mac refusals at `list` and `list for-push` (§3, O3(d), M25, M28, R1, R5)

**Files:**
- Modify: `cmd/git-remote-proton/main.go`:
  - `run()` (`:144-207`);
  - `loop` (`:323`), which becomes a wrapper;
  - the `list` arm (`:373-455`), between the content-skip check and the ref printing;
  - the `list for-push` arm (`:457-525`), after `ScanRefs` and before the ref printing.
- Test: `cmd/git-remote-proton/main_test.go`

**Interfaces:**
- Produces:
  - `type helperEnv struct{ goos, remoteName string; gitDirSet bool }`.
  - `func helperEnvFrom(args []string, getenv func(string) string, goos string) helperEnv`. It sets
    `remoteName` to `args[1]` and `gitDirSet` to `getenv("GIT_DIR") != ""`. `run()` calls it before
    `resolveGitDir`, with `os.Args`, `os.Getenv` and `runtime.GOOS`.
  - `func loopEnv(t transport.Transport, root, gitDir string, env helperEnv, in *bufio.Scanner, out
    *bufio.Writer) int`. `loop`'s body moves into it unchanged. `loop(...)` becomes `return
    loopEnv(t, root, gitDir, helperEnv{}, in, out)`, keeping its 92 test calls; the zero value
    means "not a Mac".
  - `type localState struct{ format string; specs []repo.Refspec; anonymous, prune bool; refs
    map[string]bool; symrefs map[string]string }`.
  - `func readLocalState(gitDir, remoteName string) (localState, error)`. It reads through the seams
    `readRefFormat = gitcmd.RefFormat`, `readRemoteConfig = gitcmd.RemoteConfig` and
    `readLocalRefs = gitcmd.LocalRefs` (the `probeRestoreFloor` precedent):
    1. It reads the format. For `reftable` it returns at once.
    2. It reads the remote config, then the local refs. `refs` holds the names; `symrefs` holds
       those with a symbolic target; `prune` is the config's `Prune`.
    3. It decides the specs (R5, R6):
       - a remote whose `url` and `pushurl` are both unset or empty is anonymous, with no specs;
       - a configured fetch list goes through `ParseFetchRefspecs`, and its error fails closed;
       - no fetch refspec gives `DefaultFetchRefspecs` only when the repository holds no refs (a
         clone at `list`, M28), and no specs otherwise, because git then writes no refs (PM9).

    Each error is wrapped as `<read>: <err>`, where `<read>` is `ref format`, `remote config` or
    `local refs`.
- **`list` arm.**
  1. `pairs := repo.AdvertisedPairs(refs)`.
  2. If `env.goos == "darwin" && env.gitDirSet`, read the local state. On an error:
     `warn(fmt.Errorf("cannot check this Mac repository before fetching: %w", err))` and `return 1`.
  3. If the format is `files`:
     - **rule (1):** with any pairs, `warn(fmt.Errorf(...))` whose text is `refusing to fetch on
       this Mac: this repository uses git's default (files) ref store, which cannot keep these refs
       apart (README, Honest limits):\n` + `PairLines` + `\nrepair the remote, or clone with
       --ref-format=reftable`; `return 1`.
     - **rule (2):**
       - `d := repo.ExpandSymrefs(repo.FetchDestinations(refs, specs), local.symrefs)`;
       - `cs := repo.DestinationConflicts(d, local.refs)`;
       - when `local.prune` is true (R6), `cs = repo.MergeConflicts(cs,
         repo.LocalConflicts(repo.PruneCandidates(specs, d, local.refs), local.refs))`.

       With any, `warn(fmt.Errorf(...))` whose text is `refusing to fetch on this Mac: refs this
       fetch would write or prune collide with each other or with refs this repository holds
       (README, Honest limits):\n` + `DestLines` + `\ndelete the stale local ref by its exact bytes
       (git update-ref --stdin), change the remote's fetch refspecs, or use a --ref-format=reftable
       clone`; `return 1` [review round 3: DeepSeek, on the wording].
  4. Otherwise, with any pairs, write `PairWarning` to stderr once, then advertise as today.
- **`list for-push` arm.**
  1. On a Mac `files` repository (as above, with the same fail-closed reads, which say "before
     pushing"), it checks `cs := repo.LocalConflicts(repo.ExpandSymrefs(repo.UpToDateTracking(scan.Refs,
     specs, local.refs), local.symrefs), local.refs)` (R1, PM1, R6).
  2. With any, it calls `warn(fmt.Errorf(...))` whose text is `refusing to push on this Mac: git
     would rewrite tracking refs that collide with refs this repository holds, even for refs that
     are already up to date (README, Honest limits):\n` + `DestLines` + `\ndelete the stale local
     ref by its exact bytes (git update-ref --stdin), or push from a --ref-format=reftable clone`,
     and then `return 1`.

     That happens after `AcquireLock`, so the deferred release runs; the refusal comes before the
     refs are advertised.
  3. Otherwise, on every platform, it writes `PairWarning` once after the refs are printed. It is
     never written from the push arm's second scan (`:578`).
- **Reach, stated in the README.**
  - `git ls-remote` inside a Mac `files` repository is refused like a fetch, because nothing
    before `list` tells them apart (§3).
  - Advertised tags count even under `--no-tags` or `tagOpt`, a deliberate over-refusal.
  - R1 has two narrow residuals, both named:
    - two up-to-date refs whose custom mappings alias each other, with neither tracking ref
      existing yet;
    - an up-to-date push to `R` from a differently named source given as exact bytes.
  - `push --prune` deletes remote refs it was sent as deletes, so Task 9's part 3 sees them.
  - A fetch pruned by configuration is R6's prune check. A command-line `fetch --prune` is
    invisible to the helper, like command-line refspecs and `--refmap`.
  - A clone with `--single-branch` or `--depth` is checked under the default mapping, wider than
    the one git writes later, so it can be refused for a branch it would not have fetched
    [review round 3: Kimi].

- [ ] **Step 1: Tests (RED).** These run through `loopEnv`, on every OS. Remotes are a seeded
  `transport.Fake`, as the existing list tests do. Local repositories come from
  `newGitRepoWithCommit` and `emptyGitRepo`, with `git init --ref-format=reftable` where named.
  - `TestHelperEnvFromReadsGitDirBeforeItIsDefaulted`.
  - `TestLoop_ListWarnsOnAPairOffAMac`: the warning is written exactly once.
  - `TestLoop_ListForPushWarnsOnAPair`.
  - `TestLoop_ListRefusesAPairOnAMacFilesRepo`: exit 1, and no ref line on stdout.
  - `TestLoop_ListOnAMacReftableRepoOnlyWarns`, and `TestLoop_ListOnAMacWithGitDirUnsetOnlyWarns`.
  - `TestLoop_ListRefusesTheRenameTransition` (spec step 5.8's shape).
  - `TestLoop_ListRefusesTwoAliasingDestinations` (Codex).
  - `TestLoop_ListAnonymousURLChecksTagsOnly`: `remoteName "proton::/my-files/r"`, with no remote
    config.
  - `TestLoop_ListTreatsAHandWrittenColonNamedRemoteAsNamed` (Review Focus 2, PM3): `[remote
    "a:b"]` with a URL and a default-shaped refspec gets its refspecs checked.
  - `TestLoop_ListDuringACloneUsesTheDefaultMapping` (M28).
  - `TestLoop_ListBareAndMirrorClonesOfAPairAreRefused` (Review Focus 3).
  - `TestLoop_ListRefusesAnUnsupportedRefspec` (R4): `refs/heads/main:heads/x` gives exit 1, naming
    the value.
  - `TestLoop_ListOnAMacFailsClosedOnEachLocalRead`: three subtests.
  - `TestLoop_ListOffAMacMakesNoLocalReads`: seams that call `t.Fatal`.
  - `TestLoop_ListForPushRefusesAnAliasingTrackingRefOnAMac` (R1, PM1's shape). The local branch
    `café` NFC equals the advertised NFC ref, and the local repo also has a stale NFD tracking ref.
    Exit 1, the lock is released, and no ref is advertised.
  - `TestLoop_ListForPushOnAMacAllowsARemotePairWithALocalTrackingRef` (gate step 5.9.1's shape;
    Gemini). The remote holds both spellings. The local repo holds branch `café` NFC and its NFC
    tracking ref only. `list for-push` succeeds, so the push arm can refuse per ref.
  - `TestLoop_ListForPushOnAMacReachesThePushArmForADeleteOfAnUnheldName` (gate step 5.9.2's
    shape). The remote holds NFD only, and the local repo holds the stale NFC tracking ref and no
    NFD-named ref. `list for-push` succeeds.
  - `TestLoop_ListRefusesAnAliasThatAPruneCouldUnlink` (R6, Codex's prune case), with
    `fetch.prune=true`. The same state with pruning unconfigured fetches [Kimi].
  - `TestLoop_ListRefusesAHeadBranchBesideTheTrackingHEAD` (R6, Codex): during a clone, a remote
    holding `main` and `head` is refused.
  - `TestLoop_ListFollowsALocalSymbolicTrackingRef` (R6, Codex).
  - `TestLoop_ListTreatsAPushOnlyRemoteAsNamed` (R5): `pushurl` and `fetch` set, no `url`, so its
    refspecs are checked [Codex].
  - `TestLoop_ListWithNoRefspecInAPopulatedRepoChecksTagsOnly` (R6, PM9).
- [ ] **Step 2: Run them and see them fail.** `go test -count=1 -run 'HelperEnv|Loop_List'
  ./cmd/git-remote-proton`.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run the suite and the Global vet checks.** Any existing `TestLoop_*` whose fixture
  holds a pair and now sees the warning is updated to expect it, and the report lists each one.
  Expected: PASS.
- [ ] **Step 5: Commit.** `feat(helper): warn on ref pairs; refuse Mac files-store fetches and pushes that would confuse them (O3(d))`

### Task 9: The Mac push rule and its wiring (§3, M24, M26, R2, R3)

**Files:**
- Create: `internal/repo/macpush.go`
- Modify: `internal/repo/push.go`:
  - `Push` (`:61-62`) becomes a wrapper;
  - phase 2's case block (`:332-427`) becomes a closure, run inside a new outer loop;
  - phase 5's re-check (`:600-631`) is extended.
- Modify: `cmd/git-remote-proton/main.go` (the push arm, `:527-598`)
- Test: `internal/repo/repo_test.go`, `cmd/git-remote-proton/main_test.go`

**Interfaces:**
- Produces:
  - `type LocalSide struct{ Refs map[string]bool; Tracking map[string][]string }`. `Refs` holds the
    local ref names before the push. `Tracking` maps a pushed destination to its tracking refs,
    already expanded through local symbolic refs (R6). An update follows a symbolic tracking ref
    to its target, so the target is checked too.
  - `func PushWithLocal(t transport.Transport, root, gitDir string, ups []protocol.RefUpdate, remote
    map[string]string, skipped []SkippedRef, local *LocalSide) []Result`. `Push(...)` becomes
    `return PushWithLocal(..., nil)`, keeping the existing calls (203 lines in `repo_test.go`).
  - `func OneLine(s string) string`, exported from `push.go` as `oneLine`'s public name; `oneLine`
    stays and calls it.
- Phase 2, with `local != nil`: `for { <the D/F preflight over the current valid flags>; <D2 and A1
  to their own fixed point>; if macPass() refused nothing { break } }`.
  - The D/F preflight re-runs each round because a delete part 3 keeps is kept for a reason
    unrelated to case. The preflight comment's "needs no re-run for a kept ref" argument covers
    only A1's keeps [review round 2: DeepSeek].
  - With `local == nil`, phase 2 is exactly today's.
  - Each part reads the state its predecessor left. Each is set algebra, so batch order changes
    nothing.
  1. **Writes**, over the writes valid at the pass's start. A write `X` is refused when either
     holds:
     - **(a)** some `T` in `Tracking[X]` conflicts with `Refs` or with another valid write's
       tracking refs;
     - **(b)** `X` is an update (`!isCreate`) and conflicts with `batchFinalSet`.

     The message lists every failing clause in that order, joined by `; also `:
     - for (a), one clause per `T`, sorted: `its tracking ref <T> would collide with <items>`. Each
       item is `formatItem(c)`, followed by ` (the tracking ref of <X2> in this push)` when the
       item is another write's tracking ref, where `<X2>` is the smallest such write. An item that
       is both a local ref and another write's tracking ref carries the note. Items are sorted by
       `Ref` and joined with `, `.
     - for (b), `the remote also holds <items>`.

     The message is `<X>: refused on this Mac: <clauses>; this repository's default (files) ref
     store cannot keep these apart (README, Honest limits); repair the remote, or push from a
     --ref-format=reftable clone`.
  2. **Paired deletes.** For each `X` refused in part 1:
     - first, a still-valid delete of `X` itself is refused (a replacement);
     - then, **only if `X` is a create**, a still-valid delete of a name the pre-batch remote
       holds, that no still-valid write rewrites, and that matches `X` by either key. It pairs with
       the **smallest** such `X`, as in Task 5.

     The message is `<D>: delete refused because it pairs with <X>, which this Mac refused;
     neither was applied`.
  3. **Deletes.** A still-valid delete `Y` is refused when some `T` in `Tracking[Y]` conflicts with
     `Refs`. The message is `<Y>: delete refused on this Mac: git would then delete its tracking
     ref <T>, which this repository's default (files) ref store keeps under the same file name as
     <items> (README, Honest limits); delete it from a --ref-format=reftable clone`. `<items>` uses
     part 1's item format.

  The deletes parts 2 and 3 keep are recorded in `macKept` [review round 3: Gemini].
- **R3.** In an outer round after the first, D2 may refuse a create `X` whose conflicts include a
  ref in `macKept`. That refusal is caused by this rule.
  - Its message is `<X>: refused on this Mac: this push's <deletes> refused here, and <X> would then
    conflict with <items>; this repository's default (files) ref store cannot keep these apart
    (README, Honest limits); repair the remote, or push from a --ref-format=reftable clone`,
    instead of `caseConflictMessage`. The tail is the same remedy as part 1's [review round 3:
    DeepSeek].
  - `<deletes>` lists only the kept refs `X` actually conflicts with (`cs` ∩ `macKept`), sorted,
    in `failedDeleteClause`'s grammar: `delete of A was`, or `deletes of A, B and C were` [review
    round 3: Gemini].
  - **A1 pairing with an R3 create.** When A1 refuses a delete `D` because it pairs with a create
    R3 attributed to the Mac, `D` gets part 2's message (`… pairs with <X>, which this Mac refused;
    neither was applied`), not A1's, and joins `macKept`. So an inner pass later in the same round
    attributes a create that conflicts only with `D` to the Mac too [review rounds 2 and 3:
    Gemini].
- **Termination.** No step ever sets `valid` back to true. Each inner pass and each outer round that
  continues refuses at least one update, even when parts 2 and 3 refuse only deletes, so the
  loops end within (updates + 1) rounds [review round 3: DeepSeek].
- **R2, in phase 5, with `local != nil`.** An update `X` that is not a replacement whose own delete
  committed is refused when `caseConflicts(live, [X])[X]` is non-empty and some conflicting ref was
  a same-batch delete that did not commit. Its message is part 1(b)'s with `failedDeleteClause`
  appended. It runs before `ensureRefParents`, like D2's re-check.
- Every refused update gets exactly one message, and this rule's refusals never enter A1's maps.
- **`main.go` push arm, when `env.goos == "darwin" && env.gitDirSet`.**
  - After `ScanRefs`, it calls `readLocalState`.
  - On an error, it writes `error <dst> ` + `repo.OneLine("cannot check this Mac repository before
    pushing: " + err.Error())` for every update, then a blank line, and `continue`s, like the
    poisoned path (`:546-553`).
    - `repo.OneLine` exports `push.go`'s `oneLine` (`:1204`). Git's stderr is multi-line, and an
      embedded newline would desynchronise the protocol [review round 2: DeepSeek].
  - With `files`, it builds `LocalSide{Refs: state.refs, Tracking: dst →
    PushTrackingRefs(state.specs, dst)}`, each list expanded through `state.symrefs`. That mapping
    ignores negatives (R6). An anonymous URL gets no tracking.
  - In every other case, it calls `PushWithLocal(..., nil)`.
- **Which parts real git can reach** (the gate needs this):
  - Part 1(a) for a pushed create or update.
  - Part 1(b) whenever a pair is on the remote (gate step 5.9.1).
  - Part 3 for a delete of a name the local repository does not hold. That is the gate's step
    5.9.2: R1 at `list for-push` covers only names held locally.
  - A delete of a locally held name whose tracking ref aliases a local ref never reaches part 3:
    R1 refuses that session first. Part 3 covers it for direct callers.

- [ ] **Step 1: `repo_test.go` (RED).** Use `seedCaseRemote` and `newGitRepoForPush`, with HEAD on
  `main`.
  - `TestPushWithLocalRefusesAnUpdateToAPairMember`, which leaves an unrelated create OK.
  - `TestPushWithLocalClearsTheRemoteHalfWhenTheOtherMemberIsDeleted`.
  - `TestPushWithLocalLocalHalfIsNotClearedBySameBatchDelete`.
  - `TestPushWithLocalRefusesAWriteWhoseTrackingRefCollidesWithALocalRef`.
  - `TestPushWithLocalRefusesTwoWritesWhoseTrackingRefsCollide`.
  - `TestPushWithLocalRefusesADeleteThatWouldUnlinkALocalRef` (M24).
  - `TestPushWithLocalRefusesAReplacementWhole` (M26).
  - `TestPushWithLocalPairsARenameDeleteWithARefusedCreate`.
  - `TestPushWithLocalNamesEveryCollisionSorted`: two tracking refs, and two conflicts for one of
    them, give one message listing all, sorted.
  - `TestPushWithLocalAttributesAKeptDeleteToTheMac` (R3, Gemini's case). The remote holds `café`
    NFD, and `Refs` holds the NFC tracking ref. In `[delete NFD, create NFC]`, the delete is
    refused (part 3), and the create's message contains `refused on this Mac` and not `on every
    platform`.
  - `TestPushWithLocalPhase5RefusesAnUpdateWhenItsCounterpartDeleteFails` (R2). Inject the delete
    failure as `TestPushPhase5RefusesACreateWhoseVariantDeleteFailed` does.
  - `TestPushWithLocalPhase5SparesAReplacement` (R2): the update's own delete committed, so the
    write is not refused.
  - `TestPushWithLocalKeepsD2Precedence`: `TestPushRenameRefusalsReachAFixedPoint`'s batch gives
    the same results and texts as `Push`.
  - `TestPushWithLocalNilIsPush` (GUARD).
  - `TestPushWithLocalRefusalsAreOrderIndependent` (20 orders).
  - `TestPushWithLocalSkipsRefsWithNoTrackingMapping` (Review Focus 5).
  - `TestPushWithLocalAttributesAChainedKeptDelete` (R3 through A1): a two-step chain gives Mac
    messages throughout.
  - `TestPushWithLocalReRunsTheDFPreflightForAKeptDelete`: part 3 keeps `Y`. A same-batch create
    of `Y/sub` is refused by the D/F preflight in the next round, not later in phase 5.
    - `Tracking` maps `Y` alone, as an exact refspec would, and gives `Y/sub` none. Under a
      wildcard, `Y/sub`'s tracking ref shares `Y`'s prefix, and part 1(a) would refuse it first
      [review round 3: Gemini].
  - `TestPushWithLocalAttributesAPart2KeptDelete` (Gemini's iota batch). With `[delete S2, create
    S1, create S3]` and only `S1`'s tracking ref colliding, `S3`'s refusal names `S2` and says
    `refused on this Mac`.
  - `TestPushWithLocalR3NamesOnlyTheConflictingKeptDeletes`: an unrelated part 3 refusal in the
    same batch is not named.
  - `TestPushWithLocalFollowsASymbolicTrackingRef` (R6, Codex).
- [ ] **Step 2: `main_test.go` (RED).**
  - `TestLoop_PushOnAMacFailsClosedOnALocalRead`: a seam returns a two-line error. Each update gets
    exactly one `error <dst> …` line, and the session ends 0.
  - `TestLoop_PushOnAMacRefusesByTrackingRef`.
  - `TestLoop_PushOffAMacUsesNoLocalSide`.
  - `TestLoop_PushOnAMacWithAnAnonymousURLAppliesTheRemoteHalfOnly` (Review Focus 5). The remote
    name is `proton::/my-files/r` with no remote config, and the remote holds a pair. An update of
    one member is refused by part 1(b), an unrelated create lands, and nothing fails closed
    [review round 3: Kimi].
- [ ] **Step 3: Run them and see them fail.** `go test -count=1 -run
  'PushWithLocal|Loop_PushOnAMac|Loop_PushOffAMac' ./internal/repo ./cmd/git-remote-proton`.
- [ ] **Step 4: Implement.**
- [ ] **Step 5: Run the suite and the Global vet checks.** Expected: PASS, with every existing `Push`
  test unchanged.
- [ ] **Step 6: Commit.** `feat(repo): the Mac push rule, judged after D2's fixed point (O3(d))`

### Task 10: The scheduled-push wrapper and its expiry test (§4)

**Files:**
- Create: `scripts/macos/gpb-v2-push.sh` (bash 3.2), the wrapper the README's example and the gate
  both use.
- Create: `scripts/macos/gpb-v2-push.plist.example`
- Create: `scripts/macos/test-gpb-v2-push.sh`, which is offline and uses stand-ins.

**Interfaces:**
- `gpb-v2-push.sh <repository> <remote>`:
  1. **Environment.** It sets and exports `PATH="$GPB_PATH"`, where `GPB_PATH` defaults to
     `$HOME/.local/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin`. The README
     says to set it where git lives elsewhere [review round 3: DeepSeek]. It exports
     `PROTON_DRIVE_CACHE_DIR` if the plist sets it.
  2. **Pushes.** It runs `git -C <repository> push <remote> --all`, then `push <remote> --tags`.
     The second runs even if the first fails, and the wrapper exits with the first non-zero code.
  3. **The bound.** Each push runs in the background under `set -m`, so it gets its own process
     group (PM6). It is bounded at `GPB_PUSH_TIMEOUT` seconds (default 1800).
     - A watchdog, started under `set -m` too, so in a process group of its own, sleeps for the
       bound. On expiry it sends `kill -TERM -- -<push pgid>`, then `kill -KILL -- -<push pgid>`
       after 10 s, and the wrapper records exit `124`.
     - Before it sends TERM, the watchdog writes a sentinel file in the run's temp dir.
     - When `wait` on the push returns, the wrapper checks the sentinel.
       - **Present:** the watchdog fired, so the wrapper `wait`s for the watchdog to finish its
         KILL step. The direct `git` child can die on TERM while a grandchild that ignores TERM is
         still alive, so reaping the watchdog then would skip the KILL [review round 3: Gemini].
       - **Absent:** the push ended first, so the wrapper kills the watchdog's whole group and
         `wait`s on it.
     - No sleeping timer outlives a push, later signals a recycled group, or holds the wrapper's
       output open [review round 2: Gemini].
     - Bash's job notices may go to the log.
  4. **The marker.** It is `$GPB_MARKER_DIR/gpb-v2-push.<cksum>.failed`. `GPB_MARKER_DIR` defaults
     to `~/Library/Logs`, and `<cksum>` is the first field of `printf '%s\n%s' "<abs repository>"
     "<remote>" | cksum`. It is written on any failure and names the repository, the remote, each
     failed push and its exit code. On a `124` it also says: "the push was killed; if the next
     push reports that this repository is locked by this Mac, that lock is the killed push's:
     remove it as that message says (README, scheduled use)". A run that succeeds removes only its
     own marker.
  5. **The notification.** A function `notify "<message>"`:
     - When `GPB_NOTIFY` is unset, it runs `osascript -e "display notification \"$msg\" with title
       \"git-proton-backup\""`, with the message escaped for AppleScript and passed as a single
       argument.
     - When `GPB_NOTIFY` is set, it runs that command with the message as its one argument.
     - Either way it is started with `</dev/null >/dev/null 2>&1 &`, never waited on, and its exit
       code is ignored. A hung or failing notification can never change the wrapper's outcome or
       hold a caller's pipe [review round 2: DeepSeek, Gemini].
- The plist runs the wrapper with absolute paths and a daily `StartCalendarInterval`, sends output
  to `StandardOutPath` and `StandardErrorPath`, and may set `PROTON_DRIVE_CACHE_DIR`.

- [ ] **Step 1: Write the test (RED: no wrapper).** Each case uses a fresh temp dir for
  `GPB_MARKER_DIR`, `GPB_PATH="<stand-in dir>:$PATH"`, and a `GPB_NOTIFY` that touches a file in
  the temp dir. The stand-in `git` records each call and the PIDs it spawns in files.
  - **success:** both pushes run, in order; exit 0; no marker.
  - **failure:** `--all` exits 1; `--tags` still runs; exit 1; the marker names `--all`; the
    notify file exists.
  - **two repositories:** A fails, then B succeeds. A's marker survives.
  - **hang:** the stand-in sleeps and spawns a grandchild that ignores TERM (`trap '' TERM`), under
    `GPB_PUSH_TIMEOUT=2`. The wrapper exits 124 within 20 s, the marker carries the lock sentence,
    and `kill -0` on both recorded PIDs fails afterwards, so KILL reached the TERM-proof one.
  - **notify hangs:** `GPB_NOTIFY` names a stand-in that sleeps 600 s. Run through a pipe (`… |
    cat`), the wrapper exits within 5 s.
  - **default notification:** with `GPB_NOTIFY` unset, a stand-in `osascript` on `GPB_PATH` records
    its argv. It received `-e` and one script argument naming the repository.
  - **watchdog reaped:** on the success case with the default 1800 s bound, the wrapper exits
    within 5 s, and the watchdog PID it records is gone afterwards.
- [ ] **Step 2: Run it and see it fail.** `/bin/bash scripts/macos/test-gpb-v2-push.sh`.
- [ ] **Step 3: Write the wrapper and the plist example.**
- [ ] **Step 4: Run the test under `/bin/bash` 3.2.** Expected: seven PASS lines, exit 0.
- [ ] **Step 5: Commit.** `feat(macos): scheduled-push wrapper with a bounded, reaped push (§4)`

### Task 11: CI: the macOS leg, the table check and the wrapper test (§2, MD3(a))

**Files:**
- Modify: `.github/workflows/ci.yml`, in the `go` job.

- [ ] **Step 1: Edit `ci.yml`.**
  - The matrix gains `{ os: macos-latest, go-version: stable }`.
  - The Linux `Build`, `Vet`, `Test` and `gofmt` steps widen to `runner.os != 'Windows'` and drop
    "(Linux)" from their names. This is the builder's choice; this plan takes it.
  - New steps keyed `if: runner.os == 'macOS'` go **immediately after `Vet` and before `Test`**,
    beside the Linux probe:
    - **Arch:** `test "$(uname -m)" = arm64 || { echo "macOS leg is $(uname -m), not arm64"; exit
      1; }`.
    - **Git floor (macOS):** the same `probe()`. Its fallback is `HOMEBREW_NO_AUTO_UPDATE=1 brew
      install git`, then `export PATH="/opt/homebrew/bin:$PATH"` and a re-probe. It logs `command
      -v git` and `git --version`, fails if the floor is still missed, and on success appends
      `/opt/homebrew/bin` to `$GITHUB_PATH`.
  - New steps after `Test`:
    - **Wrapper**, `if: runner.os == 'macOS'`: `/bin/bash scripts/macos/test-gpb-v2-push.sh`.
    - **Unicode tables are current**, on one leg only, keyed `if: matrix.os == 'ubuntu-latest' &&
      matrix.go-version == 'stable'` so a future matrix entry cannot move it [review round 3:
      DeepSeek]: `cd
      internal/repo && go generate . && git diff --exit-code && test -z "$(git status --porcelain
      .)"`.
- [ ] **Step 2: Run locally what can run.** On this Mac, run the macOS step bodies in file order,
  then the table check. Expected: arm64, `probe: runner git`, the wrapper test passing, and no
  diff.
- [ ] **Step 3: Commit.** `ci: macOS arm64 leg, Unicode table check, wrapper test (MD3(a))`
- [ ] **Step 4: STOP: ask Craig before pushing.** On his word:
  - push and open a **draft** PR;
  - confirm the macOS log lists Arch, the floor probe and Test as **run and passed**, in that
    order;
  - record the runner's git version.

### Task 12: The drift check's fifth leg (§7, MD7(b))

**Files:**
- Modify: `scripts/Test-ProtonCliDrift.ps1`: the header (`:4-15`), `$CertifiedMacArm64Sha512`
  beside `:40`, the scan (`:56-60`), the output (`:103-106`) and the verdict (`:112-117`).
- Modify: `tests/DriftCheck.Tests.ps1`
- Modify: `docs/releasing.md`: "compares four things" (`:92`), the four-field text and tuples at
  `:108-132` (to five fields), and the certification section, which gains spec §7's offline Mac
  checks. The signed-out run goes through the pin-moving candidate built from source, never
  through `GPB_UNCERTIFIED_CLI=1`.

**Interfaces:**
- `$CertifiedMacArm64Sha512` is recorded under Global Constraints' procedure, on one line, in
  single quotes.
- The leg reads the same release's `macos/arm64` or `macos-arm64` entry. No entry is a MISMATCH; a
  value that is not 128 hex characters is a STOP.
- New output lines: `feed macos/arm64   :` and `certified macos/arm64:`.
- The MISMATCH clause gains `; feed macos/arm64 == certified macos/arm64: <bool>` after the Linux
  one.
- `$macOk` joins the MATCH condition, so a macOS-only drift is never a MATCH [review round 2:
  DeepSeek].
- The mac leg mirrors the Linux leg's case-strict check (`:73`): an uppercase entry is a STOP. The
  Windows leg differs, because it lowercases its hash when it reads the feed (`:61`); the mirrored
  tests keep that asymmetry and do not "fix" it [review round 3: Kimi, checked].

- [ ] **Step 0: Make the stubs portable (PM10; my call).** `New-Stub` (`DriftCheck.Tests.ps1:23-29`)
  keeps writing a `.cmd` when `$IsWindows`. Elsewhere it writes an executable `#!/bin/sh` script at
  the same path minus `.cmd`, with each line printed by `printf '%s\n'` and the same exit code, and
  then `chmod +x`. The callers' paths use a helper that adds `.cmd` only on Windows.
  - On this Mac, the existing 19 tests then pass.
  - Windows CI, on the draft PR, confirms the `.cmd` path is unchanged.
  - Commit this on its own (`test(drift): portable stubs, so the drift tests run on macOS`) before
    Step 1, so a reviewer can tell it from the fifth leg.
- [ ] **Step 1: Tests (RED).**
  - `Invoke-Drift` gains `-MacSha512`, `-MacPlatform`, `-NoMac`, `-OlderMac` and
    `-ExtraMacSha512`, mirroring its Linux parameters. Its `$files` builder (`:49-59`) adds the mac
    entry to **every** branch unless `-NoMac`, so the existing Windows and Linux tests keep a
    complete feed [review round 2: Gemini].
  - Mirror **every** Linux-leg test (`DriftCheck.Tests.ps1:146-211`) for macOS.
  - Every existing verdict-line regex gains the fifth clause.
- [ ] **Step 2: Run them and see them fail** by Task 0's Pester route.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run Pester, then PSScriptAnalyzer.** `Invoke-ScriptAnalyzer -Path scripts,tests
  -Recurse -Settings .\PSScriptAnalyzerSettings.psd1` must be clean.
- [ ] **Step 5: Commit.** `feat(drift): fifth leg for the feed's macos/arm64 build (MD7(b))`

### Task 13: The README, its macOS pin guard, and the other documents (§1, §3, §4, §8)

**Files:**
- Modify: `README.md`, `tests/ReadmePins.Tests.ps1`, `docs/v2-remote-helper-design.md`,
  `docs/releasing.md`, `docs/superpowers/specs/2026-10-15-v2-linux-port-design.md` (§5, a dated
  note), `CHANGELOG.md` (`## [Unreleased]`, which PR #16 opened), `docs/README.md`.

- [ ] **Step 1: `ReadmePins.Tests.ps1` (RED).** `BeforeAll` also reads `$CertifiedMacArm64Sha512`
  by the same regex shape, and throws if it is missing [review round 2: DeepSeek]. A `Describe
  'README macOS CLI pins'` then has two tests:
  - the URL `https://proton.me/download/drive/cli/<semver>/darwin-arm64/proton-drive`;
  - the line `<$CertifiedMacArm64Sha512>  proton-drive`.
- [ ] **Step 2: README.**
  - **"Install on macOS (v2)":**
    - prerequisites;
    - `git clone --branch <tag>` and the build recipe;
    - the `curl … && shasum -a 512 -c …` chain;
    - the `~/.zprofile` line;
    - the `type -a` checks;
    - the quarantine note;
    - the first-pass status line.
  - **Scheduled use:**
    - the wrapper and the plist;
    - one machine per repository, `push --all` then `push --tags`;
    - a refusal is stop-and-repair;
    - a timed-out push may leave the remote's lock, removed as the helper's lock message says;
    - a hand check after each CLI upgrade;
    - what is not gated (spec §4's list);
    - a recommendation to give a scheduled repository a `--ref-format=reftable` clone, which no
      Mac refusal reaches (my call) [Codex].
  - **Honest limits:**
    - the canonical-equivalence rule and its cost (`ᾳ`, `αι`);
    - the lone NFD name;
    - existing pairs and what a Mac clone does with one;
    - the Mac refusals at `list` and `list for-push`, with their remedies by cause and their
      reach (Task 8);
    - unsupported refspec forms (R4);
    - command-line refspecs and `--refmap`;
    - another git process on the same Mac changing refs mid-push, which no check sees [Codex];
    - the scan's folder; writers first;
    - the four-step repair, with the mandatory re-clone.

    In Mac context, the repair route comes first ("from Windows, Linux or a `reftable` clone; a Mac
    `files` clone refuses it"), ahead of the generic `git push … --delete`.
  - **Drift:** each new CLI on the Mac is not live-gated.
  - **Replace** `:412` ("Case is folded the way Go's `strings.EqualFold` does…") and `:414` ("…is
    out of scope; neither is checked").
  - **Update** `:27`, `:30`, `:94`, `:100`, the "Why PowerShell?" close (`:320-321`), and `:471`.
- [ ] **Step 3: Other documents.**
  - The design doc goes to **v6.10**: its status line, the supported-platforms line, D2 widened to
    two relations, and the rulings copied in, plus R1–R5.
  - `releasing.md`: macOS has no asset and no per-release gate, and why; the Mac gate's place in
    the port's release (O5); this release's Windows and Linux briefs carry the steps Task 15
    drafts (O5(d)).
  - The Linux spec §5 gains a dated note.
  - CHANGELOG `## [Unreleased]` gets entries, beside PR #16's.
  - `docs/README.md` gets links.
- [ ] **Step 4: Verify.** Run Pester and PSScriptAnalyzer by Task 0's route. Then `grep -n
  'strings.EqualFold\|out of scope; neither is checked' README.md` must print nothing.
- [ ] **Step 5: Commit.** `docs: macOS install, honest limits, design v6.10 (§8)`

### Task 14: Probe scripts into `docs/research/probes/` (§8)

**Files:**
- Create: `docs/research/probes/macos-port/`. This is a subfolder, unlike the flat probes beside
  it, because this port has about twenty files; a `README.md` maps each to its finding (M12–M29,
  PM1–PM9).
  - Everything comes from `.superpowers/mac-port/probe-state/`; nothing depends on a session
    scratchpad. Copied there on 2026-10-27: `nfd-probe*.sh`, `mnprobe.go`, `fold-classes/`,
    `helper-protocol/`, `gitdir-argv/` (including `m26-m27.md`), `clone-at-list/`,
    `delete-aliasing/` (including `uptodate.sh` for PM1), `quoting/`, and `plan-probes/`.
  - `plan-probes/` holds `colon-name.sh` and `worktree-pgrp.sh` with their logs (PM3–PM6), and
    `inline-probes.md`, which records PM2, PM7, PM8 and PM9, run as inline commands.
  - Gatekeeper (M18–M20) was run as inline commands too, so only its outputs exist,
    `gatekeeper-run*.out`; `inline-probes.md` says so. No Gatekeeper script is claimed [review
    round 3: Kimi].

- [ ] **Step 1: Copy, then read every file before committing.** Look for paths, user names and
  tokens; scratch paths are already written as `$SP`. Leave out the Proton README copy,
  `feed-version.json` and `e2e-proton-drive.log`.
- [ ] **Step 2: Commit.** `docs(probes): the macOS port's probe scripts and pinned results`

### Task 15: The macOS gate brief, then STOP for Craig (§6, MD5(a), O5)

**Files:**
- Create: `docs/research/gates/vNEXT-macos-gate-brief.md`, renamed to the version once Craig names
  it.
- Create: `docs/research/gates/vNEXT-composition-refusal-steps.md`. It holds the steps this
  release's Windows and Linux briefs must carry (O5(d)): the NFD create refused on each platform's
  own binary, naming both spellings and the relation, with no ref or pack written.

- [ ] **Step 1: Write the macOS brief** from spec §6, in `brief-checklist.md`'s form under rules
  1–12.
  - Its PC section covers:
    - the sign-in probe's Windows steps;
    - the task pause;
    - the Windows checks;
    - step 4's and step 5's Windows pushes, step 5.5's NFD move and step 5.8's rename pushes;
    - the ref-set comparison.
  - It carries:
    - the fresh-clone build;
    - `caffeinate -is` on mains power;
    - remotes seeded with `main`;
    - `core.precomposeunicode=false`;
    - `GPB_UNCERTIFIED_CLI=1` for the v0.10.0 commands if the pin moved;
    - the held draft;
    - the retry rule.
  - It adds a step for R1, in PM1's shape: a Mac `files` clone holds branch `café` NFC at the
    remote's commit and a stale NFD tracking ref, and its `git push --all` is refused at `list
    for-push`. Step 5.9.2 stays as the spec gives it, because R1 does not reach a delete of a name
    the clone does not hold (Task 9, "Which parts real git can reach").
  - The gate's plist sets `PROTON_DRIVE_CACHE_DIR` to the gate's cache folder, so step 0's re-check
    after step 6 tests something [review round 2: Kimi].
- [ ] **Step 2: Write the composition-refusal steps file.**
- [ ] **Step 3: Peer review both** with the full panel, as Craig's standing instruction asks, and
  apply the findings.
- [ ] **Step 4: STOP.** On Craig's word, push the final head. Report:
  - the branch, and the CI result **for that exact commit**;
  - the two documents;
  - what remains: the release under `docs/releasing.md`, and the gates in O5's order.

  Merge nothing.

## Self-review notes

- **Spec coverage.**
  - §1 → Tasks 13 and 15.
  - §2 → Task 11.
  - §3 → Tasks 2–9.
  - §4 → Tasks 10 and 13.
  - §5 → Task 1.
  - §6 → Task 15.
  - §7 → Tasks 12 and 13.
  - §8 → Tasks 1 and 12–14.
  - "Still untested" rows → Task 11 Step 4 (CI git), Task 6 (worktree), Task 10 (expiry) and Task 4
    (invalid bytes); the rest at the gate.
- **Not tasks:**
  - the README's second-pass status line, after the Mac gate;
  - the version bump and the CHANGELOG flip;
  - the Windows and Linux briefs themselves, written at release time from v0.10.0's plus Task 15's
    steps file.
- **Interfaces across tasks.**
  - Task 2 produces `foldRune`, `decomposeRune`, `combiningClass` and `unicodeVersion`.
  - Task 3 produces `caseKey`, and Task 4 `nfd` and `canonKey`.
  - Task 5 produces `relation`, `caseConflict.Rel` and `formatItem`.
  - Task 6 produces `gitcmd.RefFormat`, `RemoteConfig` (with `RemoteCfg`) and `LocalRefs`.
  - Task 7 also produces `ExpandSymrefs` and `MergeConflicts`.
  - Task 7 produces:
    - `RefPair`, `AdvertisedPairs`, `PairLines` and `PairWarning`;
    - `Refspec`, `ParseFetchRefspecs`, `DefaultFetchRefspecs` and `MapRef`;
    - `PushTrackingRefs`, `FetchDestinations`, `PruneCandidates` and `UpToDateTracking`;
    - `DestConflict`, `DestinationConflicts`, `LocalConflicts` and `DestLines`.
  - Task 8 produces `helperEnv`, `localState` and `readLocalState`.
  - Task 9 consumes all of these.
- **Proportion.** Names, signatures, message texts and assertions are pinned; bodies are left to the
  implementer, except `canonKey`'s one line.
- **Review round 1 (2026-10-27: Codex, Gemini, DeepSeek, Kimi)** shaped Task 0, PM1–PM8 and R1–R5.
  Rejected:
  - Kimi's `args[1] == args[2]` test for an anonymous URL: M25 shows those are `logtest::x` and
    `x`. R5 replaces it.
  - Kimi's Hangul concern: Part 1 lists the syllables (PM8).
  - DeepSeek's "git rejects an empty fetch destination": `refs/heads/main:` fetches and writes
    nothing (PM2).
  - Gemini's "a Mac `files` user cannot repair from their own clone" as a defect: that is the
    spec's design; the repair runs from a `reftable` clone.
- **Review round 2 (2026-10-27: Codex, Gemini, DeepSeek, Kimi).**
  - Gemini called round 1's R1 a blocker: refusing every push whenever any advertised ref aliased
    a local ref broke D2's ruling and gate steps 5.9.1 and 5.9.2. R1 is now scoped to names the
    clone holds, where PM1 can occur.
  - R6 records Codex's three conservative widenings (negatives on push, negated tags, prune) and
    PM9's no-refspec mapping, after Kimi's fail-open concern.
  - Also applied:
    - `gitStdout` rejects `ErrWaitDelay` (Gemini);
    - the push-arm error goes through `OneLine` (DeepSeek);
    - `formatItem`, plural and chained R3 attribution, and the D/F preflight re-run (Gemini,
      DeepSeek);
    - the wrapper's watchdog reaping, quiet stdio and a working default notification (Gemini,
      DeepSeek);
    - the generator's `-out` flag and three inputs (Kimi, DeepSeek);
    - the class-merge count (DeepSeek);
    - the 13 text-asserting tests (Gemini);
    - the drift builder, `$macOk` and the pins' `BeforeAll` (Gemini, DeepSeek);
    - rule 12's scope during this plan (Kimi);
    - the explicit `if:` keys (Kimi);
    - the gate plist's cache folder (Kimi).
  - Kimi's PM8 completeness concern was checked: all 11,172 syllables are listed.
  - Not taken:
    - Kimi's `push --prune` route: those deletes are sent to the helper, so part 3 sees them;
    - DeepSeek's equality assertion on class counts, logged instead because O1(iv) accepts merges.
- **Review round 3 (2026-10-27: Codex, Gemini, DeepSeek, Kimi; Gemini, DeepSeek and Kimi wrote
  "Blockers: none", and Codex labelled none).**
  - **A defect in how this plan was written.** Every `\u` escape typed into it was decoded into a
    raw character when the file was saved, so the vectors read wrongly; Hangul's, for example,
    read as the identity [DeepSeek]. A script restored the escapes in 33 quoted spans, building
    each backslash from `chr(92)` so the restoring save could not decode them again. The spec's
    M29 row had the same fault and was restored the same way; that is a transcription fix, not
    an amendment.
  - **Applied:**
    - the tracking HEAD, symbolic local refs, and push-only remotes in R5 and R6 (Codex);
    - prune checked only when configured, with command-line `--prune` among what the helper
      cannot see (Kimi);
    - `OtherSrc`, `PruneCandidates`' type, `MergeConflicts` (Gemini);
    - part 2's keeps in `macKept`, R3's filtered `<deletes>`, A1's message beside an R3 create,
      and the D/F test's exact mapping (Gemini);
    - R3's remedy tail, termination by monotonicity, the `EqualFold` comment sweep, the trap
      test's single `Install`, `/usr/local/bin` in `GPB_PATH`, and the pinned table-check leg
      (DeepSeek);
    - the watchdog sentinel (Gemini);
    - the anonymous-URL push test, the probe inventory, Task 2's `curl` step, the Windows-leg
      lowercasing note, the explicit Kelvin sign, and the `--single-branch` over-refusal (Kimi);
    - the Part 1 invariant cited to `NormalizationTest.txt`'s own header (DeepSeek).
  - **Checked and not taken:**
    - DeepSeek's worry that the trap message cannot reach `Version()`'s error: `cli.go:58` uses
      `CombinedOutput`;
    - Kimi's "`warn` is at `:718`": it is at `main.go:744`.
  - **The changes made after round 3 have been reviewed by no engine.**

## Execution

Subagent-driven, task by task, a fresh reviewer per task, then a whole-branch review. Tasks 5, 8 and
9 change refusal behaviour that every platform's release ships; a defect there reaches Windows and
Linux users, not only the Mac.

## Execution notes, 2026-10-27 to 2026-10-28

A dated record of the rulings made while this plan ran (subagent-driven, task by task, with a
review after each task and a whole-branch review at the end). Each one changed or superseded the
task text above, which is left as written. Where they differ, the note and the code win. Each is
the executor's call unless it says Craig's. The working ledger behind them is not part of the
repository.

**Craig's calls during execution.**
- **P6, 2026-10-27:** "Run this plan's Pester and PSScriptAnalyzer commands in my Terminal panel
  (Tasks 12 and 13 only), each preceded by the proton-drive PATH check." The session's harness
  refused every `pwsh` run, so Task 0's route (a) could not run as written. Not taken: deferring
  the Pester runs to CI; allowing `pwsh` in the session's permissions.
- **MA3, 2026-10-28, option (c):** the stale-local-ref remedy. A bare exact-bytes delete can
  unlink the other spelling's loose ref on APFS (measured; record in
  `docs/research/probes/macos-port/stale-ref-remedy/`). The helper's two refusal messages and the
  gate name only the `reftable` re-clone. The spec and README give the re-clone first, then the
  pack-first delete. This supersedes the message tails quoted in Task 8 (`:796`, `:806`) and the
  remedy Task 13 was to document. The spec carries MA3 in its Amendments table. Not taken: (a)
  the re-clone alone everywhere; (b) the pack-first delete alone.

**Refinements of the push rule and the read rules.**
- **R7 (T9-1), a refinement under O3(d):** part 1(a) also refuses an *update* whose tracking ref
  aliases a still-valid same-batch delete's tracking ref; a create is not refused. It was measured
  with git 2.54.0, which processes the update first and loses its tracking ref. This supersedes
  Task 9's wording at `:905`, `:915` and `:970-971`. The README and the design doc (v6.10) carry
  R7, and the gate brief records why no §6 step can provoke it.
- **T9-2:** when both halves of part 1 fire, the second clause reads "the remote holds", with no
  doubled "also".
- **T8-2:** when the local repository holds no refs, the checks use the configured fetch specs
  plus git's default mapping, because git adds that mapping to a `-c` fetch spec during clone. It
  supersedes `:163-164` and `:773-774`. Cost: an empty non-clone repository with custom fetch specs
  can be over-refused (the README says so).
- **T8-1:** the pair warning keeps the `git-remote-proton: ` prefix every helper line carries.
- **T6-1:** `LocalRefs` fails closed on any stderr after exit 0. `TestLocalRefsIgnoresStderr`
  (`:624`) became a RED test expecting that error.
- **T6-2, an accepted limit:** a dangling symbolic local ref is invisible to every git enumerator.
  A destination written through one is unseen. The common case is still caught: a dangling
  `refs/remotes/<r>/HEAD` after a prune meets R6's synthesized `<p>/HEAD`. A loose-ref walk would
  amend spec §3; that is Craig's call.
- **T3-1:** Task 3 also pins that no rune folds to or from `/`.
- **T12-2:** the drift check's macOS leg mirrors the Linux leg as built. An uppercase hash passes
  the format check and is a MISMATCH, not the STOP Task 12's Interfaces said. The existing Linux
  test pins that behaviour.

**Tasks 11 to 15.**
- **P4:** Task 11 Step 4's push and draft PR are deferred and merged with Task 15 Step 4. CI has
  not run on this branch; the plan's "passing CI on the exact final commit" waits for that push.
- **T11-1:** the Go 1.22 leg also runs the containment test with `-v`, so the merged-class count
  is in the log.
- **T12-1:** the macOS CLI's fresh download, needed to record its SHA-512, went to a scratch
  folder. It was only hashed, never run, then deleted. The live feed, spec M2 and the download
  agree (2026-10-27).
- **T13-1 to T13-5:**
  - the design doc v6.10 copies in R1–R7 and states T6-2 and T8-2 (not only R1–R5);
  - the README and CHANGELOG prose had house style's pass and its detector, run from this Mac;
  - the README routes a letter-case pair's repair to Linux or a `reftable` clone, and Windows only
    for a normalisation-only pair [inference: NTFS ignores case and keeps normalisation];
  - MA3's probe is committed as its record.
- **T14-1 to T14-5:** Task 14 copied what it lists plus the two `nfd-run1*.log` outputs. It added
  `//go:build ignore` to the copied Go files, because `go vet ./...` failed on two `func main`.
  `run1-default/` (it holds a client UID) and `version.out` stayed out. Scratch paths became `$SP`.
- **P5 and T15-1 to T15-5:**
  - Task 15's implementer wrote the two documents;
  - the executor ran Step 3's full-panel review: Codex, Gemini, DeepSeek and Kimi, three rounds,
    the skill's cap, with five fix rounds;
  - the brief was re-anchored after Tasks 12 and 13;
  - its PowerShell is unparsed on the Mac, so the parse is a PC-side check;
  - the brief carries 13 questions for Craig, each with a default.
- **The final review's fixes:** see the commits after `7c9aca7`. Among them, the lone-NFD finding
  (measured 2026-10-28; record in `docs/research/probes/macos-port/lone-nfd-default/`): at the Mac
  default `core.precomposeunicode=true`, git reports a loose NFD ref name precomposed. A lone NFD
  branch's second update then fails in git itself. The code keeps git's view, and a test pins the
  helper's refusal; whether to change that is Craig's call.

**What this record does not settle.** Five spec texts wait on Craig, each with its before and
after text:
- M28's plain-clone-only text;
- §3 repair step 3's "a Windows or Linux client" for any pair;
- the MA3 row's "with `git fsck` clean" (L8 shows fsck exiting 0 while listing the lost commit as
  dangling), copied into the design doc's v6.10 entry;
- §3's lone-NFD premise ("corrupts nothing"), false at git's Mac default (measured above);
- the push rule's termination bound, "(updates + 1)": a round can continue on a part-3 delete
  alone, so the bound that holds is entries + 1.

**Settled later the same day (Craig's call, 2026-10-28: "Proceed with your recommendations").**
- The five texts are amended as shown to him, as spec amendments MA4 to MA8. The design doc's
  v6.10 entry copies them.
- For the lone NFD name, the helper keeps reading git's own view (MA4). Not taken: reading exact
  bytes; refusing such a name up front.
- The dangling symbolic local ref stays an accepted limit (MA9). Not taken: a loose-ref walk.
- The gate brief's question defaults stand as unopposed defaults, for Craig to confirm when the
  gate is prepared. Question 1, the version name, has no default.

**Kept for later.** The final review's follow-ups, none blocking:
- `fetch_unicode.go` should refuse a non-HTTPS redirect before the Unicode pin next moves.
- R2's phase-5 re-check could be gated like R2's own `uncommitted` set, to save its cost.
- R1 does not compare up-to-date tracking refs with each other; the README states the gap.
- A directory pair of n and m branches prints n×m warning lines on every `list`; cap it.
- A hermetic test of R1's `reftable` remedy path at `list for-push` would back gate step 5.11.
- These coverage gaps: generator checks 1 and 3, supplementary-plane samples, an equal-class mark
  vector, an invalid byte between reordered marks, the canonical twins of two tests, and the
  remaining task-review test gaps.

**The first CI run (run 40821233371, on `92d2168`, 2026-10-28) passed in all five jobs.**
- The macOS leg ran on arm64 (`macos-26-arm64`) with the runner's own git 2.56.0, so the floor
  probe needed no brew fallback. The wrapper test passed 7 of 7, and every package passed,
  including the two tests at git's Mac default.
- The Go 1.22 leg ran the containment test against toolchain Unicode 15.0.0. It checked 2974 orbit
  members and passed; 3 pinned classes span more than one toolchain orbit, which O1(iv) accepts.
- Windows Pester passed 221 of 221, including the 30 drift tests through the `.cmd` stubs.
- The generated tables matched the committed data.

**Release preparation for v0.11.0 (2026-10-28, branch `worktree-release-v0.11.0`).** Craig named the
release v0.11.0 on 2026-10-28. The two files Task 15 created under placeholder names were renamed by
the scripted substitution the brief describes, to `docs/research/gates/v0.11.0-macos-gate-brief.md`
and `docs/research/gates/v0.11.0-composition-refusal-steps.md`; Task 15's text above keeps the old
names as written.
