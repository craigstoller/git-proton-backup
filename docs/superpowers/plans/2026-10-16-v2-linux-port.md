# git-remote-proton on Linux — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a gated linux/amd64 `git-remote-proton` (release 0.10.0). Along with it ship:

- the case-variant ref rule;
- the git 2.45.0 floor messages;
- Linux CI;
- the documentation the approved spec calls for.

**Architecture:**

- **Helper code.** Changes are confined to:
  - `internal/repo` (a new `casefold.go`, plus insertion points in `push.go`'s phase 2 and phase 5);
  - `internal/gitcmd` (one probe function and one error string);
  - `internal/transport/cli.go` (one refusal sentence);
  - one seam and call in `cmd/git-remote-proton/main.go`.
- **Release and process files.** These are YAML, PowerShell and Markdown:
  - CI and release workflows;
  - the drift script;
  - `releasing.md`, README, the design doc, the gate checklist;
  - a new Pester guard that keeps the README's CLI pin in step with the code.
- **Release mechanics** (CHANGELOG flip, tag, live gates) follow `docs/releasing.md` after this plan,
  on Craig's word. They are not tasks here.

**Tech Stack:** Go (stdlib only, no cgo; language floor `go 1.22`), Pester 5+, GitHub Actions,
WSL2 Ubuntu-24.04 for Linux verification.

**Spec:** `docs/superpowers/specs/2026-10-15-v2-linux-port-design.md` (approved 2026-10-16; rulings
D1–D8 recorded there). Executors read the spec's §1–§9 and its Rulings table before starting.

**Spec amendment A1: pending Craig's OK; Tasks 6–7 assume it.** Plan review found that spec §5's
round-3 definition of a create lets a refused recreate destroy the ref it replaced, when a sibling
delete fails in phase 4 [Codex, Kimi]. Its definition was "a destination that does not exist on the
remote when it is written".

A1 changes two things:

1. **A create is a destination absent from the pre-batch advertised set.** A same-batch
   delete-then-recreate of a name that already existed is a replacement. It adds no spelling, and
   the case rule never refuses it.
2. **A case-changing rename is refused whole.** When phase 2 refuses a create, every valid
   same-batch delete of a ref whose full name folds equal to it is refused too.

If Craig declines A1, Tasks 6–7 must be revised before execution.

**Branch:** work on a worktree branch, as Stages 5 and 6 did (`git log --first-parent`). Commit per
task. Pushing the branch and opening a **draft** pull request are gated actions, taken together at
Task 9 Step 4 and only on Craig's word. `ci.yml` runs only on pushes to `main` and on pull requests,
so the draft PR is what runs Task 8's CI. Merge to `main` only on Craig's word, after Task 12's stop.

## Global Constraints

- **Go:**
  - Code and tests must compile on Go **1.22**. The new Linux CI leg runs at `go-version-file:
    go.mod`. No `t.Chdir`, no `slices`/`maps` helpers added after 1.22, no range-over-func.
  - Stdlib only, `CGO_ENABLED=0`. `go.mod` gains no `require`.
  - `gofmt -l .` empty. `.gitattributes` already pins `*.go` to LF.
- **Tests:**
  - Every `go test` run in this plan, and in the workflows this plan touches, passes `-count=1`.
  - No hermetic test may reach the Proton account.
  - Label every new test **RED** (must fail before its task's change, including a test of a function
    that doesn't exist yet, which fails to compile) or **GUARD** (pins behaviour that already
    holds). For every RED test, record in the task report **which assertion fired** (or "compile
    failure") before the change.
- **CLI pin:**
  - `transport.CertifiedCLI` stays `cli-drive@0.9.0+75612299`.
  - `GPB_UNCERTIFIED_CLI` behaviour is unchanged.
  - No new **production** environment variables, and no override for D2 (Craig's ruling). Test
    fixtures may set their own variables, such as Task 2's `GPB_TEST_ARBITRARY_PASSTHROUGH`.
- **The git floor is `2.45.0`.** Text that names it uses exactly `git 2.45.0 or newer`.
- **Release assets (D1(a)):**
  - `git-remote-proton.exe`
  - `git-remote-proton.exe.sha256`
  - `install.ps1`
  - `git-remote-proton-linux-amd64`
  - `git-remote-proton-linux-amd64.sha256`
- **The certified Linux CLI.** Version `0.9.0` (the semver of `CertifiedCLI`). Its linux/x64 SHA-512
  is the value Proton's live `version.json` lists for 0.9.0 `linux/x64`. Spec F5 recorded it
  starting `3533025b` and ending `a7e91ca8`. To record it:
  - Copy it **from the feed**, never from memory.
  - Then require `sha512sum ~/opt/proton-drive-cli/0.9.0/proton-drive`, the binary downloaded and
    feed-verified on 2026-10-15, to equal it.
  - **If they differ, STOP and report to Craig.** Proton has re-cut the build since the probes ran,
    and recording either value would bless bytes nothing tested.

  The value appears in the drift script (Task 10) and the README (Task 11). Task 11's Pester guard
  keeps them equal, and the Linux gate (Task 12) re-checks it.
- **Case folding is Unicode simple folding,** the equivalence `strings.EqualFold` implements.
  Invalid UTF-8 bytes are copied through unfolded, so distinct invalid bytes never compare equal.
  NTFS `$UpCase` differences and NFC/NFD normalisation are documented limits, not code.
- **Untouchable top-level folders** on the live account (six):
  - `GitBackups`
  - `Sensitive Project Sources`
  - `Project Repo Bundles`
  - `ChatGPT Export Text Backup`
  - `Tax Documents`
  - `Archive`
- **WSL hermetic run.** Every Linux test run in this plan uses this procedure, in the
  `Ubuntu-24.04` distro, as `craig`, from a fresh **ext4** clone of the branch (never `/mnt/c`):

  ```bash
  grep -qx 'enabled=false' /etc/wsl.conf && grep -qx 'appendWindowsPath=false' /etc/wsl.conf
  ! /mnt/c/Users/craig/Tools/proton-drive.exe --version >/dev/null 2>&1   # must not execute
  [ -z "$(command -v proton-drive)" ]                                     # no CLI on PATH
  PATH="$HOME/opt/git-2.55.0/bin:$PATH" GOTOOLCHAIN=go1.27.0 go test -count=1 ./...
  ```

  If any of the first three lines fails, STOP; the run is not isolated.

  The distro and its builds were set up on 2026-10-15 and 16 (spec F3, F5, F14):
  - `~/opt/git-2.44.4`, `~/opt/git-2.45.0` and `~/opt/git-2.55.0`, built from kernel.org tarballs
    with sha256 checked (2.55.0 with `NO_RUST=YesPlease`);
  - `~/opt/proton-drive-cli/0.9.0/proton-drive`.

  If any is missing, rebuild it the same way and say so in the task report.

## Review Focus

1. **A variant differing only in a non-ASCII letter** (`refs/heads/Über` beside `refs/heads/über`)
   is refused like an ASCII one. Owned by Task 6.
2. **A remote that already holds a pair:**
   - an update to either member, a delete of either member (the documented repair), a replacement of
     either member, and a push to an unrelated ref all succeed;
   - a new ref under a pre-existing directory pair is refused.
   Owned by Task 6.
3. **A case-changing rename beside a variant** (delete `Main`, create `MAIN`, with `main` present) is
   refused whole, so `Main` survives. A replacement of an existing name is never refused (A1).
   Owned by Task 6.
4. **The restore probe never blocks a push.**
   - A slow, hung, erroring or missing git only warns, within about 12 s (`probeTimeout` plus the
     probe's own 2 s `WaitDelay`).
   - The probe tells "too old" from "could not check".
   - It works with the absolute git dir `main.go` hands it, including a linked worktree's.
   Owned by Task 4.
5. **A refusal message is deterministic and non-empty** whatever the batch order or Go map iteration.
   Owned by Task 6.

---

### Task 1: Bring the contract test's untouchable-folder list current

**Files:**
- Modify: `internal/transport/contract_test.go`: the map's comment (`:27`), the map (`:36-41`), `validateContractLiveRoot`'s doc comment (`:52`, "one of the four"), and the `TestValidateContractLiveRoot` table (`:99-`).

**Interfaces:**
- Produces: `untouchableTopLevelFolders` holding the six names in Global Constraints.

- [ ] **Step 1: Write the failing test.** Add two rows to `TestValidateContractLiveRoot`'s rejected table (RED):
  - `{"Tax Documents prefix", "/my-files/Tax Documents/x"}`
  - `{"Archive prefix", "/my-files/Archive/x"}`
- [ ] **Step 2: Run it and see it fail.** Run `go test -count=1 -run TestValidateContractLiveRoot ./internal/transport`. Expected: FAIL on the two new rows.
- [ ] **Step 3: Add the names, and fix both comments that say "four".** Add both names to the map. Cite `docs/research/gates/cli-0.9.0-certification.md` (D3, Tax Documents) and `docs/research/gates/v0.9.0-release-gate.md` (post-release ruling 5, Archive).
- [ ] **Step 4: Run it and see it pass.** Same command. Expected: PASS.
- [ ] **Step 5: Commit.** `test(contract): untouchable list gains Tax Documents and Archive`

### Task 2: Pin the CLI's environment pass-through

The helper starts the CLI with no `cmd.Env` [spec F8]. This task changes no production code; it is a
GUARD. It catches a cleared environment, or a filter that drops either pinned variable. Other
filters are out of its reach, and the test's comment says so.

**Files:**
- Modify: `internal/transport/cli_test.go`: a new role constant, a `TestMain` case (`:92-104`), a role function, a constructor beside `cli_test.go:912-940`, and a test.

**Interfaces:**
- Produces: `roleEnvEcho = "env-echo"` and `envEchoRoleCLI(t *testing.T) *CLI`.

- [ ] **Step 1: Add the role.** `runEnvEchoRole()` prints `helperVersionLine`, then `PDCS=` + `os.Getenv("PROTON_DRIVE_CREDENTIALS_STORE")`, then `ARB=` + `os.Getenv("GPB_TEST_ARBITRARY_PASSTHROUGH")`, one per line, and exits 0.
- [ ] **Step 2: Write `TestCLIPassesItsEnvironmentThroughUnfiltered` (GUARD).**

```go
t.Setenv("PROTON_DRIVE_CREDENTIALS_STORE", "pass")
t.Setenv("GPB_TEST_ARBITRARY_PASSTHROUGH", "x-7f3a")
out, err := envEchoRoleCLI(t).Version()
// err == nil; out contains "PDCS=pass" AND "ARB=x-7f3a"
```

- [ ] **Step 3: Run it, then prove it detects a filter.** Run it; expected: PASS. Then temporarily set `cmd.Env` in `CLI.run` (`cli.go:52`) to `os.Environ()` with only the `PROTON_DRIVE_CREDENTIALS_STORE=` entry removed. That keeps the role dispatch variable, so the role still runs. Confirm the test FAILS, and record the observed failing assertion. Expected: `PDCS=pass` missing. Revert.
- [ ] **Step 4: Commit.** `test(transport): pin the CLI's environment pass-through (spec F8)`

### Task 3: Name git 2.45.0 in the fetch error (D5(a))

**Files:**
- Modify: `internal/gitcmd/gitcmd.go:568-583` (`RevListMissing`'s doc comment and wrap text)
- Test: `internal/gitcmd/gitcmd_test.go:940-953` (`TestRevListMissingWrapsUnderlyingErrorWithFloorText`)

- [ ] **Step 1: Write the failing test.** Extend the test so it also asserts `strings.Contains(err.Error(), "git 2.45.0 or newer")`. That assertion is RED.
- [ ] **Step 2: Run it and see it fail.** Run `go test -count=1 -run TestRevListMissingWrapsUnderlyingErrorWithFloorText ./internal/gitcmd`. Expected: FAIL on the new assertion.
- [ ] **Step 3: Change the wrap to this exact text:**

  `%w (note: selective fetch requires git 2.45.0 or newer: rev-list --missing=print must report absent objects as ?-lines, and an older git dies here with its own 'bad object' error instead)`

  In the doc comment, replace "the Stage 3b minimum-git floor; probed on 2.53" with "git 2.45.0 or newer, the Stage 3b floor, measured 2026-10-16 and recorded in design v6.8".
- [ ] **Step 4: Run it and see it pass.** Run the same test, then `go test -count=1 ./internal/gitcmd`. Expected: PASS.
- [ ] **Step 5: Commit.** `feat(gitcmd): the fetch error names git 2.45.0 or newer (D5(a))`

### Task 4: Warn on push when this git cannot restore (D5(d))

**Files:**
- Modify: `internal/gitcmd/gitcmd.go`: add the probe.
- Modify: `cmd/git-remote-proton/main.go`: a seam variable beside `runSetHeadFn` (`main.go:63`), and a call in the `case line == "list for-push":` arm (`main.go:425-446`).
- Modify: `cmd/git-remote-proton/shim_test.go`: the package's `TestMain` (`shim_test.go:38-43`, `package main`, the same package as `main_test.go`).
- Test: `internal/gitcmd/gitcmd_test.go`, `cmd/git-remote-proton/main_test.go`

**Interfaces:**
- Produces, in `gitcmd`:
  - `type ProbeResult int`, with constants `ProbeOK`, `ProbeTooOld` and `ProbeUnknown`.
  - `func ProbeMissingTips(gitDir string) (ProbeResult, string)`. The second value comes from `probeDetailLine`.
  - `func probeDetailLine(output string, err error) string`. It returns one line: newlines replaced by ` | `, bounded to 300 bytes after flattening, prefixed `git rev-list: `. When `output` is empty, it uses `err`'s text instead.
  - `const probeMissingTipOID = "0123456789abcdef0123456789abcdef01234567"`. This is the ID measured in spec F3. Its doc comment notes that a repo genuinely holding that object would read as `ProbeUnknown`, a negligible case that only warns.
  - `var probeTimeout = 10 * time.Second`, unexported so a test can shorten it.
- **The probe:**
  - It runs `git -C <gitDir> rev-list --objects --missing=print --stdin` under `exec.CommandContext` with `probeTimeout`, and stdin `probeMissingTipOID + "\n"`.
  - It sets its own `cmd.WaitDelay = 2 * time.Second`, **not** the package's 30 s `waitDelay`, so the whole probe is bounded near 12 s.
  - It captures **all** output into one buffer (`cmd.Stdout` and `cmd.Stderr` both set to the same `bytes.Buffer`). Never leave stdout inherited. The helper's stdout is the git-remote-helper protocol stream, and a stray `?<oid>` line there would desynchronise it.
  - There is no `--not --all`, so it walks nothing.
- **Classification.** It is keyed on what the child did, never on the error's type alone:
  - The child exited 0 (`cmd.ProcessState` non-nil and `ExitCode() == 0`) and the output has a line exactly `?` + `probeMissingTipOID` → `ProbeOK`. That holds even when `Wait` also returned `exec.ErrWaitDelay` or a context error.
  - Output containing `bad object ` + `probeMissingTipOID` → `ProbeTooOld`.
  - Anything else → `ProbeUnknown`: timeout, start failure, another git error.
- Produces, in `main.go`: `var probeRestoreFloor = gitcmd.ProbeMissingTips`.
- **Placement.** In the `list for-push` arm, after `repo.Bootstrap` has created or confirmed the remote layout, and **before** `repo.AcquireLock`. That is before the lock, as spec D5(d) requires. `gitDir` there is already absolute (`resolveGitDir`, `main.go:134`).
- **Output.** Exactly one line to `os.Stderr`, and never a non-zero return:
  - `ProbeTooOld`: `git-remote-proton: WARNING: this git cannot restore from a git-remote-proton remote: fetch and clone need git 2.45.0 or newer (<detail>). This check does not block the push.`
  - `ProbeUnknown`: `git-remote-proton: WARNING: could not check whether this git can restore from a git-remote-proton remote (fetch and clone need git 2.45.0 or newer): <detail>. This check does not block the push.`
  - `ProbeOK`: nothing.

- [ ] **Step 1: Stub the seam for the package's tests.** In `TestMain`, before `m.Run()`:
  - save the real function into a package test variable `realProbeRestoreFloor`;
  - set `probeRestoreFloor` to a func returning `(gitcmd.ProbeOK, "")`.

  Existing `TestLoop_*` tests then spawn no git for the probe.
- [ ] **Step 2: Write the `gitcmd` tests (RED: `ProbeMissingTips` does not exist yet).**
  - `TestProbeMissingTipsOKOnTheSuitesGit`: a fresh `git init` repo, by absolute path → `ProbeOK`. The suite already needs git 2.45.0 or newer.
  - `TestProbeMissingTipsOKInALinkedWorktree`: `git worktree add` a worktree, then probe at `git -C <worktree> rev-parse --absolute-git-dir` → `ProbeOK`. Other `gitcmd` calls already run with `-C` on that path for linked worktrees.
  - `TestProbeMissingTipsUnknownOutsideARepo`: probe at `t.TempDir()` → `ProbeUnknown`, and the detail starts `git rev-list: ` and contains `not a git repository`.
  - `TestProbeMissingTipsUnknownOnTimeout`: set `probeTimeout = time.Nanosecond`, restored in `t.Cleanup` → `ProbeUnknown`, with a non-empty detail.
  - `TestProbeMissingTipsDetailIsOneBoundedLine`: `probeDetailLine(<a 1,000-byte multi-line string>, nil)` contains no `\n` and is at most 300 bytes plus the prefix. `probeDetailLine("", errors.New("boom"))` contains `boom`.
- [ ] **Step 3: Write the `main_test.go` tests.** Follow the existing `TestLoop_*` shape (`captureStderr`, `loop(ft, root, ".", in, out)`, input `"list for-push\n\n"`), setting the seam and restoring it in `t.Cleanup`:
  - `TestLoop_ListForPush_WarnsWhenThisGitIsTooOld` (RED): the seam returns `(ProbeTooOld, "git rev-list: fatal: bad object 0123…")`. Assert:
    - `loop` returns 0;
    - the advertisement completes;
    - stderr has exactly one line containing `WARNING`, and it contains `cannot restore`, `git 2.45.0 or newer` and `does not block the push`.
  - `TestLoop_ListForPush_WarnsWhenTheCheckCannotRun` (RED): `(ProbeUnknown, "git rev-list: …")` → stderr contains `could not check`, and `loop` returns 0.
  - `TestLoop_ListForPush_RealProbeIsSilentOnTheSuitesGit` (RED; pins the production wiring): the seam is set to `realProbeRestoreFloor`, with the gitDir a fresh repo's absolute path → no `WARNING` in stderr.
- [ ] **Step 4: Run them and see the RED tests fail.** Run `go test -count=1 -run 'ProbeMissingTips|ListForPush_(Warns|RealProbe)' ./internal/gitcmd ./cmd/git-remote-proton`. Expected: compile failure. Record it.
- [ ] **Step 5: Implement the probe, the seam and the call** as the Interfaces block specifies.
- [ ] **Step 6: Run them and see them pass.** Run `go test -count=1 ./internal/gitcmd ./cmd/git-remote-proton`. Expected: PASS.
- [ ] **Step 7: Measure the old-git behaviour in WSL (spec F3's evidence).** Use the WSL hermetic run with the `PATH` line changed, running `go test -count=1 -run TestProbeMissingTipsOKOnTheSuitesGit -v ./internal/gitcmd` twice:
  - under `~/opt/git-2.44.4/bin`: expected FAIL, with a `t.Log` of the result showing `ProbeTooOld`. Add the log temporarily and remove it afterwards.
  - under `~/opt/git-2.45.0/bin`: expected PASS.

  Record both.
- [ ] **Step 8: Commit.** `feat: warn on push when this git cannot restore (D5(d))`

### Task 5: A clearer refusal for distro-repackaged CLI builds (D4(a))

**Files:**
- Modify: `internal/transport/cli.go`: add `distroToken`, and one sentence in `EnforceCertified`'s refusal branch (`cli.go:706-708`).
- Test: `internal/transport/cli_test.go`: role `roleDistroToken = "distro-token"`, `distroTokenRoleCLI(t *testing.T) *CLI`.

**Interfaces:**
- Produces `func distroToken(versionOutput string) (string, bool)`.
  - It reports a line whose last field matches `^cli-drive-[a-z0-9][a-z0-9.-]*@\S+$`, where that field is the whole line or follows `CLI`. Those are the two shapes `identityToken` accepts.
  - The token is returned through `bound(tok, 80)`.
- **Constraint (spec D4(a)):** it is used **only** to add text on the refusal path. `identityToken` and `IsCertified` are untouched.

- [ ] **Step 1: Add the role.** It prints `Proton Drive CLI cli-drive-debian@0.9.0+75612299` and `Proton Drive SDK js@0.22.2+75612299`, then exits 0.
- [ ] **Step 2: Write `TestEnforceCertifiedRefusesADistroRepackageWithASpecificNote`.**
  - RED assertions: the error from `EnforceCertified(distroTokenRoleCLI(t), false, io.Discard)` contains `repackaged build` and `https://proton.me/download/drive/cli/`.
  - GUARD assertions, which already hold: it also contains `CertifiedCLI` and `GPB_UNCERTIFIED_CLI`.
- [ ] **Step 3: Write `TestDistroTokenNeverCertifies` (GUARD).** It asserts:
  - `IsCertified(<that output>)` is false;
  - `IsCertified("Proton Drive CLI cli-drive@0.9.0+75612299")` is true;
  - `EnforceCertified(distroTokenRoleCLI(t), true, &buf)` returns nil, with the generic warning in `buf`.
- [ ] **Step 4: Write `TestEnforceCertifiedOmitsTheDistroNoteForAnOrdinaryMismatch` (GUARD).** `wrongVersionRoleCLI`'s refusal does not contain `repackaged`.
- [ ] **Step 5: Run them and see the RED test fail.** Run `go test -count=1 -run 'Distro|OrdinaryMismatch' ./internal/transport`. Expected: FAIL on `repackaged build`. Record it.
- [ ] **Step 6: Implement.** When `distroToken(v)` matches, append this exact sentence after the existing refusal text:

  ` This looks like a distribution's repackaged build (%q); install Proton's official build of the certified version instead (see the README's install section; Proton's downloads are at https://proton.me/download/drive/cli/).`
- [ ] **Step 7: Run them and see them pass.** Run `go test -count=1 ./internal/transport`. Expected: PASS.
- [ ] **Step 8: Commit.** `feat(transport): name distro-repackaged CLI builds in the refusal (D4(a))`

### Task 6: Refuse case-variant creates in phase 2 (D2(a), with A1)

**Files:**
- Create: `internal/repo/casefold.go`
- Create: `internal/repo/casefold_test.go`
- Modify: `internal/repo/push.go`: a new block after the D/F preflight loop (`push.go:337-352`), before `// Phase 3`.
- Test: `internal/repo/repo_test.go`, beside the D/F preflight tests (`repo_test.go:2018-2120`).

**Interfaces:**
- Produces, in `casefold.go`:
  - `func caseKey(s string) string`. It maps each valid rune to the smallest rune in its `unicode.SimpleFold` orbit, and copies invalid UTF-8 bytes through unchanged. For valid UTF-8, `caseKey(a) == caseKey(b)` iff `strings.EqualFold(a, b)`.
  - `func refPrefixes(ref string) []string`. It returns the component chains after `refs/`, so `"refs/heads/grp/a"` gives `["heads", "heads/grp", "heads/grp/a"]`. A name without the `refs/` prefix gives nil.
  - `type caseConflict struct{ Ref, Ours, Theirs string }`. `Ref` is the conflicting ref in the set. `Ours` and `Theirs` are the differently spelled prefixes that share a `caseKey`.
  - `func caseConflicts(set map[string]bool, candidates []string) map[string][]caseConflict`. For each candidate with any prefix whose `caseKey` matches a differently spelled prefix of another ref in `set`, it returns one conflict per conflicting `Ref`, the first pair in `(Ours, Theirs)` order. The result is sorted by `Ref` and is deterministic.
  - `func caseConflictMessage(dst string, cs []caseConflict) string`, which returns exactly:

    `<dst> conflicts by letter case with <Ref1> (<Ours1> vs <Theirs1>)[, <Ref2> (<Ours2> vs <Theirs2>)…]: a Windows clone would mishandle the pair (README, Honest limits), so this create is refused on every platform`
- Consumes: `push.go`'s `valid`, `isDelete`, `isCreate` (`= !exists` against the pre-batch `remote`, `push.go:298`), `remote`, `fail`.
- **A create (A1)** is a valid non-delete with `isCreate[i]`, meaning absent from the pre-batch advertised set. Replacements of existing names are never candidates.

- [ ] **Step 1: Write `casefold_test.go` (RED: compile failure).**
  - `TestCaseKeyMatchesEqualFold`: for the pairs
    - `("main","Main")`
    - `("Über","über")`
    - `("K","K")` (Kelvin sign)
    - `("main","mainx")`
    - `("ß","SS")`

    assert `caseKey(a)==caseKey(b)` equals `strings.EqualFold(a,b)`.
  - `TestCaseKeyKeepsInvalidBytesDistinct`: `caseKey("a\xffb") != caseKey("a\xfeb")`.
  - `TestRefPrefixes`: the example above; `refs/tags/v1` gives `["tags","tags/v1"]`; `"HEAD"` gives nil.
  - `TestCaseConflictsShapes`: the three spec F13 shapes, each naming its `Ref` and its prefix pair:
    - `main`/`Main` gives `heads/main` vs `heads/Main`;
    - `feat`/`Feat/x` gives `heads/feat` vs `heads/Feat`;
    - `grp/a`/`Grp/b` gives `heads/grp` vs `heads/Grp`.
  - `TestCaseConflictMessageNamesEachRefWithItsOwnPair`: `Grp/a` against both `grp/b` and `GRP`. The message names both refs, each with its own pair.
  - `TestCaseConflictsCrossNamespaceIsNotAConflict`: `refs/tags/main` beside `refs/heads/main` gives no conflict.
  - `TestCaseConflictsDeterministic`: build the same set 50 times in different insertion orders, and the result is `reflect.DeepEqual` each time.
- [ ] **Step 2: Write the `Push` tests in `repo_test.go`.** Use the existing helpers: `transport.NewFake`, `Bootstrap`, `newGitRepoForPush`, `headOf`, `WriteRef`, `countPackFiles`, `ScanRefs`. Each fixture keeps HEAD on a branch the batch doesn't delete. A create that a test expects to land, or to cost a pack, points at a commit absent from the pre-batch remote.
  - `TestPushRefusesACaseVariantCreateBesideAnExistingRef` (RED):
    - remote has `refs/heads/main`; push only `Dst: refs/heads/Main`;
    - not OK, and `Err` contains `refs/heads/main` and `conflicts by letter case`;
    - `countPackFiles` is 0, and `ScanRefs` shows only `main`.
  - `TestPushRefusesBothSameBatchCaseVariantCreates` (RED): an empty remote, `main` and `Main` in one batch. Both are refused, each naming the other, and no pack is written.
  - `TestPushCaseVariantRefusalIsPerCreateNotPerBatch` (RED): remote has `main`. The batch updates `main` to a new commit and creates `Main`, so `main` is OK, `Main` is refused, and `countPackFiles` is 1.
  - `TestPushRefusesFileDirAndDirDirCaseVariants` (RED):
    - `Feat/x` beside an existing `feat`;
    - `Grp/b` beside an existing `grp/a`;
    - both are refused.
  - `TestPushRefusesANonASCIICaseVariant` (RED; Review Focus 1): `refs/heads/über` beside an existing `refs/heads/Über`.
  - `TestPushCaseRuleCoversTags` (RED): `refs/tags/release-1` beside `refs/tags/Release-1`.
  - `TestPushTagAndBranchOfTheSameNameStillCoexist` (GUARD): `refs/tags/main` beside an existing `refs/heads/main` is OK.
  - `TestPushExistingPairStillTakesUpdatesDeletesReplacesAndUnrelatedRefs` (GUARD; Review Focus 2): the remote already holds `main` and `Main`.
    - Batch 1: update `main` and create `refs/heads/topic` → both OK.
    - Batch 2: delete `Main` and recreate `Main` at a new commit (a replacement, A1) → both OK, and `ScanRefs` shows `Main` at the new sha.
    - Batch 3: delete `Main` → OK, and `ScanRefs` shows no `Main`.
  - `TestPushExistingDirectoryPairBlocksNewChildren` (RED; Review Focus 2): the remote holds `Feat/a` and `feat/b`. Creating `feat/c` is refused, and so is creating `Feat/d`.
  - `TestPushCaseChangingRenameIsRefusedWhole` (RED; Review Focus 3, A1): the remote holds `main` and `Main`. One batch deletes `Main` and creates `MAIN`.
    - both results are not OK;
    - the delete's `Err` contains `case-changing rename` and `the rename was not applied`;
    - `ScanRefs` still shows `Main` at its old sha.
  - `TestPushRenameRefusalsReachAFixedPoint` (RED; Codex's round-3 counterexample, with healthy transport):
    - the remote holds `main` (HEAD), `Foo/a`, `FOO/a` and `Foo/b`;
    - one batch deletes `Foo/a` and `Foo/b`, and creates `FOO/A` and `FOO/B`;
    - all four results are not OK;
    - `ScanRefs` shows `Foo/a`, `FOO/a` and `Foo/b` at their old shas.
    A single pass would refuse `FOO/A` and keep `Foo/a`, approve `FOO/B`, then delete `Foo/b` and refuse `FOO/B` in phase 5.
  - `TestPushCaseRefusalTextIsOrderIndependent` (RED; Review Focus 5): the three-create batch `main`, `Main`, `MAIN` in all 6 orders. Every result is refused with a non-empty `Err` naming the other two, and each destination's `Err` is identical across orders.
  - `TestPushCaseRuleIgnoresSkippedNames` (GUARD; spec §5 exception): `skipped` is `[]SkippedRef{{Path: "refs/heads/Main", Kind: SkipContent, Reason: "not a ref: junk"}}`, with no advertised `Main`. A create of `refs/heads/main` is OK.
- [ ] **Step 3: Run them and see them fail.** Run `go test -count=1 -run 'Case|casefold|RefPrefixes|Rename|SkippedNames|Coexist' ./internal/repo`. Expected: compile failure, then FAIL on each RED test once stubs compile. Record it.
- [ ] **Step 4: Implement `casefold.go`, then the phase-2 block in `push.go`.**
  - **Repeat to a fixed point.** Refusing a rename's delete keeps that ref on the remote, and the kept ref can expose a new conflict for a create the same pass approved [plan review round 3, Codex]. So the block loops until a pass refuses nothing. Each pass does the four sub-steps below:
    - **Build `caseSet`** from the current `valid`, after the D/F loop: `remote` minus valid deletes, plus valid non-deletes. Use the same two-pass algebra as `finalSet`.
    - **Find the candidates:** valid non-deletes with `isCreate[i]`.
    - **Refuse the creates:** `conflicts := caseConflicts(caseSet, <candidate dsts>)`. For each candidate `i` in `conflicts`, call `fail(i, caseConflictMessage(u.Dst, conflicts[u.Dst]))` and set `valid[i] = false`.
    - **Refuse the renames whole (A1).** For each create `X` refused in this pass, every valid same-batch delete `j` whose ref satisfies `caseKey(ref) == caseKey(X)` gets `fail(j, "<ref>: delete refused because it pairs with the refused case-changing rename to <X>; the rename was not applied")` and `valid[j] = false`.

    Each pass refuses at least one create or stops, so the loop ends within (number of creates + 1) passes. Every pass is set-based, so the result does not depend on order.
  - Add a comment citing spec §5, A1, and the device-name precedent in `marker.go`.
- [ ] **Step 5: Run them and see them pass.** Run `go test -count=1 ./internal/repo`. Expected: PASS, with every pre-existing `Push` test still passing.
- [ ] **Step 6: Commit.** `feat(repo): refuse case-variant creates on every platform (D2(a), phase 2)`

### Task 7: Re-check in phase 5, before any parent folder is created (D2(a), with A1)

**The invariant this task relies on (state it in the code comment):**

- Phase 2 checks every create, meaning every name absent from the pre-batch set (A1), against a set
  that assumes every valid delete commits. It repeats until stable (Task 6), so its final set
  already reflects every delete it refused.
- So phase 5 can only find a **new** conflict for such a name when some phase-2-valid delete failed
  in phase 4.
- Replacements of pre-existing names are never re-checked. They add no spelling, so no refusal can
  destroy the ref they replace.
- One residual needs a transport fault. A case-changing rename that passed phase 2, because its
  conflicting variant was also being deleted, can still lose the renamed-away ref if that variant's
  delete fails. The README's honest limits say so.
- No gate step can reach this path deterministically (spec §5). This task's test is its evidence.

**Files:**
- Modify: `internal/repo/push.go`, phase 5 (`push.go:491-530`).
- Test: `internal/repo/repo_test.go`

**Interfaces:**
- Consumes: Task 6's `caseConflicts` and `caseConflictMessage`; phase 5's `deletedThisBatch` and `preExists`.

- [ ] **Step 1: Write the failing test.** `TestPushPhase5RefusesACreateWhoseVariantDeleteFailed` is RED.
  - Remote: `refs/heads/main` (HEAD's target) and `refs/heads/feat`.
  - Batch: delete `feat`, create `Feat/x` at a new commit absent from the remote.
  - Push through `trashFailsForTransport{*transport.Fake; path string}`, added beside `ambiguousTrashTransport` (`repo_test.go:302-313`). It returns `transport.Ambiguous` only for `/r/refs/heads/feat` and calls through to the Fake for every other path.
  - Assert:
    - the delete result is not OK;
    - the create result is not OK, and its `Err` names `refs/heads/feat`;
    - `f.Dirs["/r/refs/heads/Feat"]` is **false**, so no parent was created;
    - no file `/r/refs/heads/Feat/x` exists;
    - `ScanRefs` still shows `main` and `feat` at their old shas;
    - `countPackFiles` is 1, the one documented unreferenced pack (spec §5).
- [ ] **Step 2: Write `TestPushPhase5NeverRefusesAReplacement` (GUARD; the Codex and Kimi counterexample).**
  - Remote: `main` (HEAD), `feat` and `Feat`.
  - Batch: delete `feat`, delete `Feat`, recreate `Feat` at a new commit. The `feat` trash fails.
  - Assert: the `feat` delete is not OK; the `Feat` delete and the `Feat` recreate are OK; `ScanRefs` shows `feat` (old sha) and `Feat` (new sha). Nothing is lost.
  - After Step 3, temporarily gate the re-check on `exists == false` instead of `!preExists`. Confirm this test then FAILS and record it, then revert.
- [ ] **Step 3: Implement the live set.**
  - Before the phase-5 loop, `live` = `remote` keys minus `deletedThisBatch`.
  - The re-check is the **first statement** of the loop body for an entry with `!preExists`, **before** `ensureRefParents`. If `caseConflicts(live, []string{u.Dst})` is non-empty, call `fail(i, caseConflictMessage(...))` and `continue`.
  - After any successful create or update, add `u.Dst` to `live`.
- [ ] **Step 4: Run them and see them pass.** Run `go test -count=1 ./internal/repo`. Expected: PASS.
- [ ] **Step 5: Run the whole suite on both platforms.**
  - Windows: `go test -count=1 ./...`, `go vet ./...` and `gofmt -l .`.
  - Linux: the WSL hermetic run.

  Expected: zero failures on both.
- [ ] **Step 6: Commit.** `feat(repo): phase-5 live-set re-check before parents are created (D2(a))`

### Task 8: Linux CI legs with a branching git-floor probe

**Files:**
- Modify: `.github/workflows/ci.yml`, the `go:` job (`ci.yml:39-70`).

- [ ] **Step 1: Make the `go` job a matrix** with `fail-fast: false` and three legs:
  - `{os: windows-latest, go-version: stable}`
  - `{os: ubuntu-latest, go-version: stable}`
  - `{os: ubuntu-latest, go-version-file: go.mod}`

  `runs-on: ${{ matrix.os }}`. Use two `actions/setup-go@v7` steps gated by `if:` on which matrix key is set, keeping the existing comment about `stable`.
- [ ] **Step 2: Split each step by shell.** The Windows steps keep `shell: pwsh`. The Linux steps use `shell: bash`.
  - Build, vet and test are `go build ./...`, `go vet ./...` and `go test -count=1 ./...` on every leg. Windows gains `-count=1` too.
  - gofmt on Linux is `test -z "$(gofmt -l .)" || { gofmt -l .; exit 1; }`.
- [ ] **Step 3: Add the Linux-only step `Git floor (2.45.0) probe`** before Test. GitHub runs `bash -eo pipefail`, so every probe and apt call sits inside an `if` or a function:

  ```bash
  OID=0123456789abcdef0123456789abcdef01234567
  probe() { local out; out=$(printf '%s\n' "$OID" | git rev-list --objects --missing=print --stdin 2>&1) || return 1; grep -qx "?$OID" <<<"$out"; }
  retry() { for n in 1 2 3; do "$@" && return 0; sleep 15; done; return 1; }
  git --version
  if probe; then echo "probe: runner git"; else
    retry sudo add-apt-repository -y ppa:git-core/ppa && retry sudo apt-get update && retry sudo apt-get install -y git || { echo "git floor: PPA install failed"; exit 1; }
    git --version
    if probe; then echo "probe: PPA git"; else echo "git floor: fetch tests need git 2.45.0 or newer; runner has $(git --version) (or, implausibly, the checkout holds the probe object $OID)"; exit 1; fi
  fi
  ```

  The checkout is shallow; the probed object is absent either way [inference]. This step's log is the measurement.
- [ ] **Step 4: Check the YAML parses.** Run `python -I -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"`. If PyYAML is absent, say so; the PR run is the real check.
- [ ] **Step 5: Commit.** `ci: Linux go legs (stable and go.mod toolchain) with a git-floor probe`

  The draft PR's CI run, opened at Task 9 Step 4, verifies it: three green `go` legs, and the probe step's log naming the path taken. Record that at Task 9 Step 4 and in Task 12's report.

### Task 9: The Linux release asset, and `releasing.md`'s asset and gate text

**Files:**
- Modify: `.github/workflows/release.yml`, at the test step (`:22`), Build (`:52-56`), Checksum (`:57-61`), Upload (`:62-67`) and Create DRAFT (`:68-73`).
- Modify: `docs/releasing.md`, at steps 4, 5, 6 ("Once the gate passes", `:59`), 7 (including "the three staged ones", `:68`), the stand-in eligibility bullets, stand-in step 1, and the stand-in's "The two exes" paragraph (`:132`).

**Interfaces:**
- Produces: the five asset names in Global Constraints. Tasks 11 and 12 use them verbatim.

- [ ] **Step 1: Test step and Build step.**
  - The test step becomes `go vet ./... && go test -count=1 ./...`.
  - After the existing Windows build, add `$env:GOOS='linux'; $env:GOARCH='amd64'; $env:CGO_ENABLED='0'; go build -trimpath -ldflags "-X main.version=$env:VERSION" -o git-remote-proton-linux-amd64 ./cmd/git-remote-proton`.
  - Each `run:` block is its own pwsh process, so these variables can't leak into later steps.
- [ ] **Step 2: Checksum step.** Write `git-remote-proton-linux-amd64.sha256` exactly as the exe's sidecar is written: lowercase SHA-256, two spaces, the name, `Set-Content -NoNewline`.
- [ ] **Step 3: Upload and Create DRAFT steps.**
  - The `upload-artifact` `path:` becomes `git-remote-proton*`. That covers the four binary and sidecar files, and any future `git-remote-proton*` asset. `install.ps1` is not in the dry-run artifact, as today.
  - `gh release create` lists all five assets explicitly.
  - The notes text becomes: `See CHANGELOG.md. Draft until the Windows and Linux live gates pass against these exact bytes.`
- [ ] **Step 4: Push, open a draft PR, dry-run the workflow, and check both binaries.** On Craig's word:
  - push the branch;
  - open a **draft** pull request against `main` (the app's Create-PR action, or `gh pr create --draft`);
  - run `gh workflow run release.yml --ref <branch>`.

  The draft PR runs `ci.yml`. Record its three `go` legs and the probe step's path (Task 8). Download the `git-remote-proton-dev` artifact.
  - On Windows: the exe's SHA-256 equals its sidecar.
  - In WSL (interop off), from an ext4 copy:
    - `sha256sum -c git-remote-proton-linux-amd64.sha256` prints `OK`;
    - `chmod 0755 git-remote-proton-linux-amd64 && ./git-remote-proton-linux-amd64 --version` prints `git-remote-proton dev+<sha7> (certified CLI: cli-drive@0.9.0+75612299)`.

  Record all three.
- [ ] **Step 5: Edit `releasing.md`.**
  - **Step 4:** the five-asset list.
  - **Step 5:** both live gates, Windows and Linux (Task 12's brief), each on the draft's bytes.
  - **Step 6:** "Once the gates pass".
  - **Step 7:** the closure covers five assets.
  - **Stand-in eligibility:** the five-asset list; `go version -m` on both binaries against the anchor's.
  - **Stand-in step 1:** five assets, both sidecars, five staged digests, the linux `--version` run in WSL.
  - **"The two exes":** now "the anchor's and the draft's binaries, per platform".
  - **A new sentence:** the first Linux release cannot take a stand-in. It changes `release.yml` and has no Linux anchor.
- [ ] **Step 6: Commit.** `release: linux/amd64 asset with sidecar; releasing.md covers five assets and two gates`

### Task 10: The drift check gains a linux/x64 leg (D8(a))

**Files:**
- Modify: `scripts/Test-ProtonCliDrift.ps1`
- Modify: `tests/DriftCheck.Tests.ps1` (the `Invoke-Drift` feed builder, `DriftCheck.Tests.ps1:34-49`; its `$null` case today builds a lone `linux/x64` entry so that windows/x64 is absent)
- Modify: `docs/releasing.md`, "Between releases" (`:71-104`: "compares three things", the triple paragraphs)

**Interfaces:**
- Produces, in the script:
  - `$CertifiedLinuxX64Sha512`: the 128-hex constant recorded under Global Constraints, with its STOP rule.
  - A fourth verdict field, `feed linux/x64 == certified linux/x64: <bool>`.
- **The new leg's rules:**
  - It reads the `linux/x64` or `linux-x64` entry **of the same release** as `$latest` (the windows/x64 entry the script already picks).
  - If that release has no linux entry, the leg is `False` (MISMATCH), not STOP. Proton dropping or delaying Linux is drift worth a certification look.
  - If the entry's SHA isn't 128-hex, the result is STOP (fail closed).
  - The comparison is case-sensitive (`-ceq`).
- **What the leg cannot do.** It checks the feed against the recorded value, and cannot see an installed Linux CLI. Nothing on Windows hashes one. `releasing.md` says so.

- [ ] **Step 1: Write the failing tests.** In `BeforeAll`, read the script's `$CertifiedLinuxX64Sha512` value with a regex into `$script:certLinux`, so the tests follow the constant. Give `Invoke-Drift` a `-LinuxSha512` parameter defaulting to `$script:certLinux`, a `-LinuxPlatform` parameter defaulting to `'linux/x64'`, and a `-NoLinux` switch. The builder today hardcodes `Platform = 'linux/x64'` (`DriftCheck.Tests.ps1:37`).
  - With those defaults every existing test keeps its expected result. Re-run them all to confirm.
  - Change the `$null` case so it builds a feed with **no** windows/x64 entry and **no** linux entry.
  - Add (RED):
    - `It 'reports MISMATCH (exit 1) naming the linux leg when the feed''s linux/x64 SHA-512 differs from the certified value'`
    - `It 'reports MISMATCH (exit 1) naming the linux leg when the latest release has no linux/x64 entry'`
    - `It 'STOPs (exit 2) when the feed''s linux/x64 entry carries a non-128-hex SHA-512 (fail closed)'`
    - `It 'accepts the linux-x64 platform spelling as well as linux/x64'`
- [ ] **Step 2: Run them and see the RED cases fail.** Run `Invoke-Pester -Path tests/DriftCheck.Tests.ps1`. Expected: the new cases FAIL, and every existing case passes. Record it.
- [ ] **Step 3: Implement the leg** as the Interfaces block specifies. The MISMATCH line names all four legs, and the header comment's "Three legs" becomes four. Fill the constant by the Global Constraints procedure, including its STOP rule, and record both hashes.
- [ ] **Step 4: Run them and see them pass.** Run `Invoke-Pester -Path tests` (all of it), then `Invoke-ScriptAnalyzer -Path scripts -Recurse -Settings .\PSScriptAnalyzerSettings.psd1`. Expected: PASS, and analyzer clean.
- [ ] **Step 5: Update `releasing.md`'s "Between releases."**
  - The check compares four things.
  - The expected shapes are rewritten for four fields.
  - The linux leg cannot see an installed Linux CLI.
  - A same-token re-cut is response-worthy: re-certify. The helper's token check alone would accept it.
  - Every certification updates `$CertifiedLinuxX64Sha512`, **and** the README's Linux CLI URL version and SHA-512. All are certification touch points, and Task 11's guard checks them.
- [ ] **Step 6: Commit.** `feat(drift): a linux/x64 leg against the certified SHA-512 (D8(a))`

### Task 11: Documentation, and a guard that the README's pin follows the code

**Files:**
- Create: `tests/ReadmePins.Tests.ps1`
- Modify: `README.md`: the v2 section (`:83-162`, including the asset list at `:96` and "Restore needs" at `:93`), "Why PowerShell?" (`:229`), honest limits (`:272`), and the roadmap (`:317`).
- Modify: `docs/v2-remote-helper-design.md`: the status line, "Deployment surface" (`:40`), Stage 4's staging line, and a new **v6.9** revision entry.
- Modify: `docs/research/gates/brief-checklist.md`: rule 11.
- Modify: `CHANGELOG.md`: a new `## [Unreleased]` section above `## 0.9.0`.
- Modify: `docs/README.md`: link the spec and this plan under "Specs And Plans", and bump "Last updated".

- [ ] **Step 1: Write `tests/ReadmePins.Tests.ps1` (RED: the README has no Linux block yet).**
  - In `BeforeAll`, read two values with regexes: `CertifiedCLI` from `internal/transport/cli.go` (then its semver), and `$CertifiedLinuxX64Sha512` from `scripts/Test-ProtonCliDrift.ps1`.
  - `It 'pins the README''s Linux CLI download to the certified version'`: the README contains `https://proton.me/download/drive/cli/<semver>/linux-x64/proton-drive`.
  - `It 'pins the README''s Linux CLI SHA-512 to the drift script''s certified value'`: the README contains `<that value>  proton-drive`, with two spaces.
  - Run `Invoke-Pester -Path tests/ReadmePins.Tests.ps1`. Expected: FAIL on both.
- [ ] **Step 2: README.**
  - **The Linux install, as two code blocks.**
    - Block 1 installs: the helper commands from spec §6, then the CLI commands with the **real** certified SHA-512 and versioned URL. There is no placeholder.
    - Block 2, introduced as "in a new login shell", verifies: `type -a git-remote-proton` and `type -a proton-drive`.
    - Before it, mention `~/.local/bin/git-remote-proton --version` for an immediate check.
  - **The Linux prerequisites block** from spec §4:
    - git 2.45.0 or newer, with the PPA route;
    - AVX2, per Proton's README, stated as the CLI's requirement;
    - the certified CLI;
    - the credential store: Secret Service tested, `pass` "not gated".
  - **The tested environment.** The helper is a static Go binary; the certified CLI is Proton's glibc linux/x64 build; the gate ran on Ubuntu 24.04 under WSL2. Other distributions are expected to work and are not gated.
  - **Honest limits:**
    - case variants:
      - F13's three shapes;
      - that new pairs are refused and replacements of existing names are not;
      - that a case-changing rename beside a variant is refused whole;
      - the residual rename loss that needs a transport fault (Task 7);
      - the ways a pair appears without the user pushing one;
      - the repair (delete one member);
      - the skipped-file exception;
      - no override;
    - unattended Linux use is not gated;
    - drift: a newer Proton CLI reaches Linux users as an allowlist refusal, while a same-version re-cut passes the helper's check and is the maintainer's drift check's to catch;
    - no certified CLI for non-AVX2 CPUs.
  - **Also:** the asset list at `:96` (all five, with the Linux pair named for Linux); "Restore needs" (Linux's prerequisites); "Why PowerShell?"'s roadmap pointer; the roadmap itself (Linux in this release, macOS aspiration only).
- [ ] **Step 3: Run the guard and see it pass.** Run `Invoke-Pester -Path tests/ReadmePins.Tests.ps1`. Expected: PASS.
- [ ] **Step 4: Design v6.9.**
  - **A supported-platforms line:** windows/amd64 and linux/amd64 (static helper; Proton's glibc linux/x64 CLI).
  - **"Deployment surface":** cite spec F6 and F8.
  - **Stage 4's line:** mark the "windows/amd64 only" sentence superseded by v6.9.
  - **The revision entry** records:
    - the D2 rule (create-only per A1, prefixes, Unicode folding, rename refused whole, phase 2 and phase 5, the skipped-name exception, the residual);
    - the D4 and D5 message changes;
    - the rulings D1–D8, each with its options not taken, copied from the spec's Rulings table;
    - A1, with Craig's ruling on it.
- [ ] **Step 5: `brief-checklist.md` rule 11.** Hermetic **test runs** (`go test`, Pester) in WSL keep interop off, and either have no `proton-drive` on PATH except a test shim, or run in a network namespace with no route out. A rule-10 audit of a staged CLI binary's `--version` or help is not a hermetic test run, and stays governed by rule 10. Cite the 2026-10-15 F14 probe.
- [ ] **Step 6: CHANGELOG `[Unreleased]`.** One bullet each:
  - the Linux asset;
  - the D2 rule;
  - D5(a) and (d);
  - D4(a);
  - Linux CI;
  - the drift leg;
  - the contract untouchables;
  - the README pin guard;
  - the docs.

  Plain user-facing wording, following the 0.9.0 entry's style.
- [ ] **Step 7: Sweep for stale claims.** Run:

  ```bash
  grep -rn -i -E "three-asset|three assets|exactly three|three staged|three things|the triple|four untouchable|four pre-existing|one of the four|windows/amd64 only|Three legs|two exes|feed's version against the pin" README.md docs scripts internal .github
  ```

  Fix every hit this release made stale. `docs/README.md`'s description of the drift check lists its legs, and now needs the fourth. A hit in a dated record (gate records, superseded plans) stays as it is, with a one-line note in the task report.
- [ ] **Step 8: Run all Pester tests.** Run `Invoke-Pester -Path tests`. Expected: PASS.
- [ ] **Step 9: Commit.** `docs: Linux port - README, design v6.9, checklist rule 11, CHANGELOG, README pin guard`

### Task 12: The Linux gate brief, then STOP for Craig

**Files:**
- Create: `docs/research/gates/v0.10.0-linux-gate-brief.md`

**Interfaces:**
- Consumes: spec §7 in full; Task 9's asset names; Task 10's constant; `brief-checklist.md` rules 1–11.

- [ ] **Step 1: Write the brief.** Write it as a runner would execute it, with expected output written before the run. It follows spec §7 in order:
  - **the pre-tag sign-in probe,** steps 1–7, including the STOP and recovery path and `BROWSER=echo` authorised;
  - **gate setup;**
  - **gate steps 0–6,** each with its command, its pass condition, and what counts as BLOCKED.

  Gate step 1 runs README Block 1 as written, and re-checks the certified SHA-512 against the feed and the installed binary. It names the confinement roots (`/my-files/GitRemotes/v010-linux-*` plus `GPB_CONTRACT_LIVE_ROOT`), the six untouchables, and the trash tally. It names both timeouts per rule 9, including `go test -timeout 40m`.
- [ ] **Step 2: Review the brief as a runner (Stage 5 and 6 lesson).** Check:
  - every command runs as written in its shell (WSL bash or Windows pwsh);
  - remote state stays consistent from step to step;
  - every expected string can be seen in the terminal;
  - `git init -b main` is used everywhere;
  - `create-folder` takes two arguments.

  Then check that every message the brief and README quote matches the code exactly: `grep` each of these fragments in `internal/` and `cmd/`:
  - `conflicts by letter case`
  - `case-changing rename`
  - `cannot restore`
  - `could not check`
  - `does not block the push`
  - `repackaged build`
  - `git 2.45.0 or newer`
- [ ] **Step 3: Commit.** `docs(gate): v0.10.0 Linux gate brief`
- [ ] **Step 4: STOP.** Report to Craig with:
  - the branch;
  - the test totals on both platforms;
  - the draft PR's CI run (Tasks 8 and 9);
  - the dry-run release evidence (Task 9);
  - the brief's path.

  What follows is Craig-directed under `docs/releasing.md`: merge, the CHANGELOG/`ModuleVersion` flip to 0.10.0, the pre-tag sign-in probe, the tag, the draft, both gates, publication, and closure.

## Self-review notes

- **Spec coverage.** Each spec item maps to tasks:
  - §1 → Task 9;
  - §2 → Task 8;
  - §3 / D5 → Tasks 3–4;
  - §4 → Tasks 2 and 11;
  - §5 / D2, with A1 → Tasks 6–7;
  - §6 → Task 11;
  - §7 → Tasks 1 and 12;
  - §8 / D8 → Tasks 10–11;
  - §9 → Tasks 9–11;
  - D4 → Task 5;
  - D1, D3, D6, D7 need no code beyond the above.
- **Names used across tasks:**
  - `caseKey`, `refPrefixes`, `caseConflict`, `caseConflicts`, `caseConflictMessage` (Task 6 → 7);
  - `ProbeResult`, `ProbeOK`, `ProbeTooOld`, `ProbeUnknown`, `ProbeMissingTips`, `probeDetailLine`, `probeMissingTipOID`, `probeTimeout`, `probeRestoreFloor`, `realProbeRestoreFloor` (Task 4);
  - `distroToken`, `roleDistroToken` (Task 5);
  - `roleEnvEcho` (Task 2);
  - `$CertifiedLinuxX64Sha512` (Task 10 → 11 → 12);
  - the five asset names (Task 9 → 11 → 12).
- **Not tested here, by design:**
  - the CI workflow (the PR run);
  - the release workflow (the dispatch dry run);
  - all live behaviour (the gates).

  Spec "Still untested" lists the rest.

## Revisions

Scaffolding for the review loop; delete before execution dispatch.

**Round 1 (2026-10-16; all four reported).**

- **Applied:**
  - paired deletes (superseded in round 2);
  - the README placeholder removed, with a pin guard;
  - the probe's typed results, timeout, one bounded line, placement and stub;
  - the message names prefixes and says "mishandle";
  - new guards: repair, cross-namespace, invalid bytes, non-empty order test;
  - Task 7's invariant;
  - CI under `pipefail`;
  - Task 9's `chmod`, prefix and `-count=1`;
  - the drift feed builder's defaults, same-release selection and both spellings;
  - rule 11's carve-out;
  - the install split, tested-environment statement and drift wording;
  - a wider stale sweep;
  - the message check;
  - merge on Craig's word;
  - Task 2's mutation;
  - both "four" comments;
  - the inline WSL procedure.

  [Codex, Gemini, DeepSeek, Kimi]
- **Rejected:**
  - a D2 override (Craig's ruling);
  - a Linux smoke job in `release.yml` (draft-until-gated covers it);
  - citing v6.9 in Task 3 (v6.8 records the floor);
  - phase 5 as "dead code" (the spec requires it).

**Round 2 (2026-10-16; all four reported; Gemini and DeepSeek said Blockers: none, DeepSeek listing
majors).**

- **A defect in round 1's paired-delete fix.** A phase-5 refusal could still destroy a replaced ref
  when a sibling delete failed [Codex, Kimi, with independent counterexamples]. The fix is spec
  amendment A1, pending Craig:
  - a create means absent from the pre-batch set, so replacements are never refused;
  - phase 5 gates on `!preExists`;
  - a counterexample GUARD with a mutation check.
- **Case-changing renames** are refused whole in phase 2, with the transport-fault residual
  documented [DeepSeek].
- **Task 7's fixture** needs a new commit, or the pack count is wrong [DeepSeek].
- **The certified-SHA STOP rule** against a re-cut [DeepSeek].
- **The probe's own 2 s `WaitDelay`,** giving a real bound of about 12 s [DeepSeek].
- **`ProbeOK` keyed on the child's exit,** whatever the error type [DeepSeek].
- **A wiring test through the real probe** [DeepSeek].
- **Two message fragments added** to the check [DeepSeek].
- **The artifact-glob claim narrowed,** with the exe sidecar checked too [DeepSeek].
- **The ReadmePins regex reads named** [DeepSeek].
- **Env-var scope** narrowed to production [DeepSeek].
- **The message gives each ref its own pair** [Kimi].
- **The probe-OID collision note** [Kimi].
- **Placement wording** [Kimi].

**Round 3 (2026-10-16; all four reported; Gemini and DeepSeek said Blockers: none; the loop's last
round, so these fixes have not been re-reviewed).**

- **Applied:**
  - **Phase 2 repeats to a fixed point.** A refused rename's kept ref could expose a new conflict for
    a create the same pass approved, losing a ref with healthy transport. Codex's counterexample is
    now a RED test [Codex].
  - The delete-refusal text says "the rename was not applied" [DeepSeek].
  - `probeDetailLine` is named, and output capture is specified so the protocol stream is safe
    [DeepSeek].
  - `-LinuxPlatform` for the drift-test builder [DeepSeek].
  - The sweep covers `docs/README.md`'s drift description [DeepSeek].
  - The CI probe's collision clause [DeepSeek].
  - The push and draft PR, gated together at Task 9 Step 4, so Task 8's CI actually runs [Kimi].
- **Rejected:**
  - **Kimi's argument that one pass is sound by transitivity.** Codex's counterexample defeats it.
    `Foo/a` folds equal to `FOO/A` as a whole name, but their directory prefixes are spelled
    differently, so the kept ref conflicts with `FOO/B` where the refused create did not.

## Execution

End of the Linux port plan; Task 12 stops for Craig.
