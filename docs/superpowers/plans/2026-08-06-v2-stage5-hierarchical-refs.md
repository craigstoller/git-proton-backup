# v2 Stage 5 — Hierarchical Refs, Quarantine Fetch, Gate-Sourced Polish: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship hierarchical ref support (full namespace re-enable), quarantine-staged fetch, opt-in parent auto-create, and the Stage 5 hygiene/UX wave, gated live and released as v0.4.0.

**Architecture:** Three layers unchanged (protocol / repository / transport). New code concentrates in `internal/repo` (recursive listing, five-phase batch engine, prune+self-heal, parent walk), `internal/transport` (`Stat` not-found classification, `EnsureDir` contradiction handling, Fake folder fidelity), and `internal/repo/fetch.go` (quarantine staging). The spec is normative: `docs/superpowers/specs/2026-08-06-v2-stage5-hierarchical-refs-design.md` (read it before any task).

**Tech Stack:** Go stdlib only; PowerShell 7 + Pester for install.ps1; real `git` for plumbing and parity tests.

## Global Constraints

- **Go stdlib only.** No new dependencies, no cgo.
- **Every shell step that runs `go` or `git` in PowerShell must first prepend the fresh PATH** (stale-PATH gotcha, runbook): `$env:Path = [Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [Environment]::GetEnvironmentVariable('Path','User')`. Written once here; every `Run:` line below assumes it.
- **Hermetic tests only** in this plan's execution. Live contract-table halves run **only at the stage gate** under `GPB_LIVE_ACCOUNT=1`; nothing in this plan touches the real account.
- **All tests run with `-count=1`** (a Stage 3b gate replayed a cached result as live).
- **Label every new test RED or GUARD in its comment**; fix rounds must report which assertion fired; the load-bearing behaviours (prune, self-heal, batch phases, quarantine publish, Stat classification) get deliberate-regression checks ("show it failing with the guard removed") before their task is reported done.
- **Plan-supplied code blocks are hypotheses**, not gospel (five SUPERSEDED banners in Stage 4, all plan-code defects). When a defect is found in one, patch this plan with a SUPERSEDED banner so a re-run cannot reintroduce it.
- **Commit messages end with:** `Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>`. Never push; Craig merges.
- **Env vars:** `GPB_CREATE_PARENTS` (new, this stage) and `GPB_UNCERTIFIED_CLI` are read fresh from the environment every invocation, never cached.
- **Certified CLI:** `cli-drive@0.7.0+5174900c`. The not-found and already-exists output signatures classified in Tasks 4 and 9b are hypotheses about that build pinned hermetically now and live at the gate — if the gate contradicts them, the gate BLOCKS and reports verbatim; runners never patch.
- **gofmt on CRLF:** `gofmt -l` false-positives on CRLF-rewritten working-tree files; committed content is clean — never "fix" line endings.
- **Sha collisions in fixtures:** identical content+message repos created within 1 s share commit shas; pin `GIT_COMMITTER_DATE`/`GIT_AUTHOR_DATE` when a fixture needs distinct histories.

## File Structure

```
docs/releasing.md                          NEW  release procedure (incl. CHANGELOG flip step)
docs/research/gates/brief-checklist.md     NEW  standing gate-brief rules (row-set comparisons etc.)
install.ps1                                MOD  registry seam parameter
tests/InstallHelper.Tests.ps1              NEW  hermetic PATH-write coverage
cmd/git-remote-proton/main.go              MOD  runSetHeadFn seam; GPB_CREATE_PARENTS wiring
cmd/git-remote-proton/main_test.go         MOD  wiring tests
internal/transport/transport.go            MOD  Stat/EnsureDir doc updates
internal/transport/cli.go                  MOD  Stat not-found classification; EnsureDir contradiction re-Stat
internal/transport/cli_test.go             MOD  helper-process roles for both classifications
internal/transport/fake.go                 MOD  folder fidelity (dir trash, D/F collisions, parent checks)
internal/transport/fake_test.go            MOD  fidelity tests
internal/transport/contract_test.go        MOD  new live rows (folder trash ×2, D/F collisions ×2, nested list, Stat not-found)
internal/gitcmd/gitcmd.go                  MOD  WritePack multi-want; CheckRefFormat; longPathHint
internal/gitcmd/gitcmd_test.go             MOD  matching tests
internal/repo/refname.go                   NEW  in-process CheckRefName + advertisableName
internal/repo/refname_test.go              NEW  rules + git parity test
internal/repo/refs.go                      MOD  recursive ListRefs (skip-with-note); WriteRef parent creation
internal/repo/push.go                      MOD  five-phase Push; checkDst rewrite; prune; self-heal; reverse-D/F
internal/repo/fetch.go                     MOD  quarantine staging
internal/repo/sethead.go                   MOD  hierarchical branch names; exact-path verification
internal/repo/parents.go                   NEW  EnsureParents (opt-in auto-create + actionable refusal)
internal/repo/repo_test.go                 MOD  all repo-layer tests incl. two-pack pair refresh
internal/testcli/  (STRETCH)               NEW  scripted proton-drive shim harness
docs/v2-remote-helper-design.md            MOD  v6.5
README.md                                  MOD  hierarchical refs, GPB_CREATE_PARENTS, MAX_PATH section
CHANGELOG.md                               MOD  v0.4.0 section
```

Dependency spine: Task 4 (Stat classification) precedes Tasks 9b and 11 (both branch on absence-vs-error); Task 7 (fidelity + validator) precedes 8–11; Task 8 (recursive listing) precedes 9a; 9a precedes 9b; 9b precedes 10–11. Tasks 1–6 are independent of the spine (6 touches only fetch.go).

---

### Task 1: Process docs — release procedure and gate-brief checklist

**Files:**
- Create: `docs/releasing.md`
- Create: `docs/research/gates/brief-checklist.md`

**Interfaces:**
- Consumes: nothing.
- Produces: `docs/releasing.md` (Task 14 follows it verbatim); `docs/research/gates/brief-checklist.md` (Task 14's gate brief cites it).

- [ ] **Step 1: Confirm no existing release-procedure doc.** Run: `git grep -il "release procedure" -- docs README.md` and `Get-ChildItem docs -Filter "releas*"`. Expected: no canonical procedure file exists (the Stage 4 procedure lives only in the spec/ledger). If one exists, MOD it instead of creating.

- [ ] **Step 2: Write `docs/releasing.md`.** Content — the Stage-4-established procedure with the missing step added, in order: (1) confirm main green and the working tree clean; (2) **flip `CHANGELOG.md`'s `[Unreleased]` section to `[vX.Y.Z] — YYYY-MM-DD` and commit — BEFORE tagging** (this step was manual/forgotten for v0.3.x; that is why this file exists); (3) tag `vX.Y.Z` on the flipped commit; push the tag only on Craig's word; (4) the Release workflow builds a **draft** with exactly three assets (`git-remote-proton.exe`, `.exe.sha256`, `install.ps1`); (5) the live gate runs against the draft's bytes; tags are never moved after any artifact is built from them; (6) Craig publishes; (7) the publication digest closure re-downloads the published assets and compares per-asset SHA-256 against the gate's staged digests — only then is the release final.

- [ ] **Step 3: Write `docs/research/gates/brief-checklist.md`.** Standing rules every future gate brief incorporates by reference: listing equality asserted on the **row set** (uid/name/type/creationTime/modificationTime/parentUid), never the serialised JSON — `filesystem list` order is unstable (observed Stage 4 run 2); write confinement named explicitly per brief, including whether parent creation outside the repo root is authorised; verify-before-trash with full subtree enumeration; report BLOCKED with verbatim output, never patch; `-count=1` on every go test invocation; empty-trash-before-gate as cheap insurance; trash accounting must count folders the run's prune operations trashed, not only files.

- [ ] **Step 4: Commit.**
```bash
git add docs/releasing.md docs/research/gates/brief-checklist.md
git commit -m "docs: release procedure (CHANGELOG flip step) and standing gate-brief checklist"
```

---

### Task 2: install.ps1 registry seam + hermetic PATH-write coverage

**Files:**
- Modify: `install.ps1:47-84` (the PATH read-modify-write block)
- Create: `tests/InstallHelper.Tests.ps1`

**Interfaces:**
- Consumes: current `install.ps1` params (`-Force`, `-SkipPathUpdate`, `-HelperExe`, `-EffectivePath`).
- Produces: new parameter `-EnvironmentKey` (object with `GetValue($name, $default, $options)`, `GetValueKind($name)`, `SetValue($name, $value, $kind)`, `Close()`); `$null` default opens the real `HKCU\Environment`. Tests never touch the real registry (isolation rule — this is the entire point of the task).

- [ ] **Step 1: Write the failing Pester tests** in `tests/InstallHelper.Tests.ps1`. Build a mock key as a `PSCustomObject` with `Add-Member -MemberType ScriptMethod` for the four methods, backed by a hashtable `@{ Path = '%USERPROFILE%\bin'; Kind = 'ExpandString' }` plus a `$script:setCalls` recorder. Cases (each asserts on the recorder, and a final `AfterAll` asserts the REAL user PATH is byte-unchanged — read it via `[Microsoft.Win32.Registry]` with `DoNotExpandEnvironmentNames` before/after the whole file):
  - RED `preserves REG_EXPAND_SZ kind on append` — existing `ExpandString` value; expect one `SetValue` call with kind `ExpandString` and value `<old>;<helperDir>`.
  - RED `preserves REG_SZ kind` — same with `String` kind.
  - RED `no-op when helperDir already present` (including a `%VAR%`-spelled entry that expands to helperDir) — expect zero `SetValue` calls.
  - RED `fresh empty PATH writes ExpandString` — `GetValue` returns `$null`; expect `SetValue` with kind `ExpandString`, value exactly `$helperDir`.
  - GUARD `Close is called on every path` including the throw path.
  Use `-SkipPathUpdate:$false`, a TestDrive helper exe + sidecar so the checksum block passes, and `-EffectivePath` from TestDrive dirs (never the real registry — the Stage 4 lesson is in the param's own comment).

- [ ] **Step 2: Run to verify failure.** Run: `Invoke-Pester tests/InstallHelper.Tests.ps1 -Output Detailed`. Expected: FAIL — `install.ps1` has no `-EnvironmentKey` parameter.

- [ ] **Step 3: Implement the seam.** Add to `param(...)`: `[object]$EnvironmentKey = $null`. In the PATH block replace `$envKey = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)` with:
```powershell
$envKey = if ($null -ne $EnvironmentKey) { $EnvironmentKey }
          else { [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true) }
```
Everything downstream (`GetValue` with `DoNotExpandEnvironmentNames`, `GetValueKind`, `SetValue`, `finally { $envKey.Close() }`) already speaks exactly this surface — verify no other registry access escapes the seam. Guard the WM_SETTINGCHANGE broadcast behind `if ($null -eq $EnvironmentKey)` (a mocked run must not broadcast).

- [ ] **Step 4: Run tests to verify pass, then the whole Pester suite.** Run: `Invoke-Pester tests -Output Detailed`. Expected: all green, zero real-registry mutation (the AfterAll guard proves it).

- [ ] **Step 5: Commit.**
```bash
git add install.ps1 tests/InstallHelper.Tests.ps1
git commit -m "test(install): injectable registry seam; hermetic PATH value-kind coverage"
```

---

### Task 3: `--set-head` wiring seam + hermetic dispatch test

**Files:**
- Modify: `cmd/git-remote-proton/main.go:46-63` (`dispatchUtility`)
- Modify: `cmd/git-remote-proton/main_test.go`

**Interfaces:**
- Consumes: `dispatchUtility(args []string, stdout, stderr io.Writer) (bool, int)`; `runSetHead(addr, branch string, stdout, stderr io.Writer) int`.
- Produces: package-level `var runSetHeadFn = runSetHead`; `dispatchUtility` calls `runSetHeadFn(args[2], args[3], stdout, stderr)`. Task 13 (stretch) additionally exercises the un-swapped path end to end.

- [ ] **Step 1: Write the failing test** in `main_test.go`:
```go
// RED: pins the dispatchUtility→runSetHead call site — argv routing, argument
// order, and exit-code propagation — which until Stage 5 was pinned only by
// live gates (hermetic tests stopped at dispatchUtility's arity arm).
func TestDispatchRoutesSetHeadArgsInOrder(t *testing.T) {
	orig := runSetHeadFn
	defer func() { runSetHeadFn = orig }()
	var gotAddr, gotBranch string
	runSetHeadFn = func(addr, branch string, stdout, stderr io.Writer) int {
		gotAddr, gotBranch = addr, branch
		fmt.Fprintln(stdout, "HEAD is now refs/heads/x")
		return 42
	}
	var out, errb bytes.Buffer
	handled, code := dispatchUtility(
		[]string{"git-remote-proton", "--set-head", "proton::/my-files/r/repo", "feature/x"}, &out, &errb)
	if !handled || code != 42 {
		t.Fatalf("handled=%v code=%d, want true/42", handled, code)
	}
	if gotAddr != "proton::/my-files/r/repo" || gotBranch != "feature/x" {
		t.Fatalf("args routed as (%q,%q)", gotAddr, gotBranch)
	}
	if !strings.Contains(out.String(), "HEAD is now") {
		t.Fatalf("stdout not forwarded: %q", out.String())
	}
}
```

- [ ] **Step 2: Run to verify it fails.** Run: `go test ./cmd/... -run TestDispatchRoutesSetHead -count=1`. Expected: FAIL, `runSetHeadFn` undefined.

- [ ] **Step 3: Implement.** In `main.go`, above `dispatchUtility`: `var runSetHeadFn = runSetHead` with a comment saying it exists as a test seam for the wiring and nothing else may reassign it. In `dispatchUtility`'s `--set-head` arm replace `runSetHead(args[2], args[3], stdout, stderr)` with `runSetHeadFn(...)` same args.

- [ ] **Step 4: Run and verify pass + deliberate regression.** Run the test; then temporarily swap `args[2], args[3]` to `args[3], args[2]` in the call site and confirm the test FAILS (this is the mutation the seam exists to catch), revert.

- [ ] **Step 5: Commit.**
```bash
git add cmd/git-remote-proton
git commit -m "test(main): seam and hermetic pin for the --set-head dispatch wiring"
```

---

### Task 4: `Stat` not-found classification + marker absent-vs-error split

**Files:**
- Modify: `internal/transport/cli.go:135-155` (`Stat`)
- Modify: `internal/transport/transport.go:52` (contract comment)
- Modify: `internal/transport/cli_test.go` (helper-process roles)
- Modify: `internal/transport/fake.go` (no behaviour change — Fake's `(_, false, nil)` is already the confirmed-absence model; add a comment)
- Modify: `internal/transport/contract_test.go` (new row)

**Interfaces:**
- Consumes: `CLI.run(args...) (string, int, error)`; existing role-based fake-CLI infrastructure in `cli_test.go` (`TestMain`/`runHelperRole`).
- Produces: `Stat` returning `(_, false, nil)` **only** for the certified CLI's not-found signature; any other nonzero exit returns an error naming the output. Tasks 9b and 11 depend on this split. Constant `notFoundSignature = "Node not found"` (unexported, cli.go).

- [ ] **Step 1: Write the failing tests** (cli_test.go, using new helper roles):
```go
// RED: a CLI that ran and reported a NON-not-found failure must surface as an
// error, never as confirmed absence. This is the Stage 4 gate 2b masquerade:
// a broken CLI under GPB_UNCERTIFIED_CLI=1 read as "not a git-remote-proton
// repo" because any nonzero info exit was folded into (_, false, nil).
func TestCLIStatNonNotFoundFailureIsAnErrorNotAbsence(t *testing.T)
// role "stat-other-error": prints "something exploded: quota exceeded" to
// stderr, exits 1. Assert: err != nil, err text contains "quota exceeded",
// ok == false.

// GUARD: the genuine not-found signature stays confirmed absence.
func TestCLIStatNotFoundSignatureIsConfirmedAbsence(t *testing.T)
// role "stat-not-found": prints "Node not found: gpb-remote.json" (the exact
// shape the Stage 4 gate captured live), exits 1. Assert: (_, false, nil).
```
Follow the existing role plumbing (`runHelperRole` switch; roles select via env). Both roles must emit on the stream the real CLI uses — check an existing role for the convention.

- [ ] **Step 2: Run to verify the RED fails.** Run: `go test ./internal/transport/ -run TestCLIStat -count=1`. Expected: the non-not-found test FAILS (current code returns `(_, false, nil)`), the signature test passes.

- [ ] **Step 3: Implement** in `cli.go`:
```go
const notFoundSignature = "Node not found"

// in Stat, replace the blanket `if code != 0 { return Node{}, false, nil }`:
if code != 0 {
	if strings.Contains(out, notFoundSignature) {
		return Node{}, false, nil // the certified CLI's confirmed-absence signature
	}
	return Node{}, false, fmt.Errorf("info %s failed: %s", p, strings.TrimSpace(bound(out, 200)))
}
```
Note `c.run` returns combined output — confirm not-found text lands in `out` (the helper roles pin whichever stream it is; if the real CLI writes it to stderr, `run` must be capturing combined output already — verify at `cli.go:51` and adjust the roles to match). Update `transport.go`'s `Stat` comment: absence is `(_, false, nil)` **only on the CLI's not-found signature**; every other failure is an error.

- [ ] **Step 4: Verify RequireMarker's two messages are now distinct end to end.** Add a repo-layer test in `repo_test.go` with a stub transport whose `Stat` returns an error: assert `RequireMarker` reports `stat ...` (transport failure), not "no gpb-remote.json". (`marker.go` already branches correctly — the defect was entirely in `CLI.Stat`.)

- [ ] **Step 5: Add the live contract row** in `contract_test.go` following the existing row pattern: `Stat` on a missing path inside the gate root → `(_, false, nil)`; and (fake half only, live half deliberately not provokable safely) any other failure → error. The live half must record the CLI's verbatim not-found output so the signature constant is pinned against reality at the gate.

- [ ] **Step 6: Run the full suite.** Run: `go test ./... -count=1`. Expected: green (live rows skip loudly without `GPB_LIVE_ACCOUNT`).

- [ ] **Step 7: Commit.**
```bash
git add internal/transport docs
git commit -m "fix(transport): Stat classifies not-found vs failure; kills the marker masquerade"
```

---

### Task 5: MAX_PATH hint + README Windows section

**Files:**
- Modify: `internal/gitcmd/gitcmd.go` (new `longPathHint`; wrap failures in `WritePack` and `PackObjectsFromList`)
- Modify: `internal/gitcmd/gitcmd_test.go`
- Modify: `README.md`

**Interfaces:**
- Consumes: `WritePack`, `PackObjectsFromList` failure paths.
- Produces: `func longPathHint(paths ...string) string` — `""` unless `runtime.GOOS == "windows"` and some `len(p) >= 240`; otherwise a one-line possible-cause hint. Unexported; only gitcmd uses it.

- [ ] **Step 1: Write the failing tests:**
```go
// RED: hint appears for a ≥240-char path on Windows, phrased as a POSSIBLE cause.
func TestLongPathHintFiresNear240(t *testing.T)   // 240-char path → non-empty, contains "may", "core.longpaths"
func TestLongPathHintSilentOnShortPaths(t *testing.T) // all < 240 → ""
```
Gate both with `if runtime.GOOS != "windows" { t.Skip(...) }` — the helper itself is GOOS-gated, and the suite's platform is Windows.

- [ ] **Step 2: Run, expect FAIL** (`longPathHint` undefined): `go test ./internal/gitcmd/ -run TestLongPathHint -count=1`.

- [ ] **Step 3: Implement:**
```go
// longPathHint returns a one-line POSSIBLE-cause note when any involved path
// approaches Windows' legacy 260-char MAX_PATH (threshold 240, deliberately
// conservative: the limit counts a terminator and directory ops fail below
// it). Path-length arithmetic only — git's messages are localised, so stderr
// pattern-matching is a non-starter. Checkout-phase failures happen after
// this helper has exited and are documented in README instead.
func longPathHint(paths ...string) string {
	if runtime.GOOS != "windows" {
		return ""
	}
	for _, p := range paths {
		if len(p) >= 240 {
			return fmt.Sprintf(" (note: a path involved is %d characters, near Windows' "+
				"260-character MAX_PATH; if this failure is about path length, set "+
				"`git config core.longpaths true` or use a shorter destination)", len(p))
		}
	}
	return ""
}
```
Append `longPathHint(...)` to the error returns of `WritePack` (paths: `outDir` plus the emitted pack path) and `PackObjectsFromList` (path: `outStem`). Keep it out of success paths.

- [ ] **Step 4: README.** Add a "Windows path length" subsection: deep clone destinations can exceed 260 chars; `core.longpaths=true` helps for git's own writes, a shorter destination always helps; checkout-phase failures (after the helper exits) can only be fixed by these, no helper hint is possible there.

- [ ] **Step 5: Run suite; commit.**
```bash
git add internal/gitcmd README.md
git commit -m "feat(gitcmd): best-effort MAX_PATH hint on pack-write failures; README section"
```

---

### Task 6: Quarantine fetch staging

**Files:**
- Modify: `internal/repo/fetch.go:59-269` (`Fetch` temp layout; `downloadAndVerifyPack`)
- Modify: `internal/repo/repo_test.go` (ordering/visibility tests; two-pack pair-refresh fixture)

**Interfaces:**
- Consumes: `Fetch(t, root, gitDir, cacheDir, wants)`; `downloadAndVerifyPack(t, root, packDir, stem, pm)`; `packMap.sidecars`.
- Produces: `downloadAndVerifyPack(t transport.Transport, root, incomingDir, packDir string, stem string, pm *packMap) (bool, error)` — verifies in `incomingDir`, publishes verified pairs into `packDir` by `os.Rename`, `.pack` first, `.idx` second (the pair's commit point: git discovers packs only via `.idx`, so an unpublished index makes the pack invisible to the traversal). The residue rule is GONE: nothing unverified ever exists under `packDir`.

- [ ] **Step 1: Write the failing tests** (repo_test.go, driving `Fetch` against a Fake with fault injection):
```go
// RED: a pack that fails checksum/pair verification must leave the ALTERNATE's
// pack dir EMPTY — quarantine's whole point. Pre-quarantine code wrote into
// packDir and cleaned up per the residue rule; this asserts the class is gone,
// not patched: inject a corrupt remote pack, let Fetch fail (or heal), and
// assert packDir contains no trace of the corrupt stem at any point observable
// after the failure.
func TestQuarantineFailedVerificationNeverTouchesTheAlternate(t *testing.T)

// RED: publication order — .pack must exist in packDir before its .idx does.
// Assert structurally: wrap os.Rename? No — assert the END state plus the
// invariant via a verify-failure injected BETWEEN members (corrupt idx, good
// pack): after the heal path re-verifies, packDir holds either nothing or a
// complete pair; never an .idx without its .pack.
func TestQuarantinePublishesPackThenIdxNeverIdxAlone(t *testing.T)

// RED: two-pack pair-refresh — the retro-Codex gap. Fixture with TWO remote
// packs; corrupt the CACHED sidecar of pack B only. Fetch must succeed,
// re-download B's sidecar (trace shows the .pack-before-refreshed-.idx order
// the Stage 4 polish test pins for one pack), and the plan's restart must
// re-select pack A WITHOUT re-downloading its verified bytes into the
// alternate twice. Pin restart-vs-resume empirically: assert A's pack is
// downloaded exactly once across the whole fetch (count trace lines).
func TestTwoPackPairRefreshRestartsPlanWithoutRedownloadingVerifiedPacks(t *testing.T)
```
Build the two-pack fixture by two sequential pushes to the Fake remote (distinct commits — pin committer dates). Reuse the existing trace-assertion helpers from the Stage 4 polish tests (find them by `git grep -n "pair refresh" internal/repo`).

- [ ] **Step 2: Run, expect FAIL:** `go test ./internal/repo/ -run "Quarantine|TwoPack" -count=1`.

- [ ] **Step 3: Implement.** In `Fetch`: add `incomingDir := filepath.Join(tmp, "incoming")` to the `MkdirAll` loop (sibling of `objects` under the one per-fetch `tmp` ⇒ same filesystem, unique per fetch, deleted by the existing `defer os.RemoveAll(tmp)`; a crash leaves one inert orphaned temp root — same story as today; **no delete-at-start of any shared path**). Rework `downloadAndVerifyPack`:
```go
func downloadAndVerifyPack(t transport.Transport, root, incomingDir, packDir, stem string, pm *packMap) (bool, error) {
	packName := stem + ".pack"
	inPack := filepath.Join(incomingDir, packName)
	// A healed plan may re-select this stem; ReadTo onto an existing file is
	// deliberately unpinned, so clear THIS QUARANTINE's own copy first. This is
	// not the old residue rule: it never touches packDir.
	if err := os.Remove(inPack); err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if err := t.ReadTo(root+"/packs/"+packName, incomingDir); err != nil {
		return false, fmt.Errorf("cannot download %s: %v — a truthful map might never have "+
			"selected this pack: %w", packName, err, errCacheSuspect)
	}
	got, err := packContentChecksum(inPack)
	// ... identical checksum-vs-basename logic, minus every os.Remove cleanup:
	// failures just return; the incoming dir is disposable wholesale.
	inIdx := filepath.Join(incomingDir, stem+".idx")
	if err := copyFile(pm.sidecars[stem], inIdx); err != nil { ...errCacheSuspect wrap as today... }
	refreshed := false
	if err := gitcmd.IndexPackVerify(inPack); err != nil {
		// sidecar refresh + rebuild + re-verify, exactly today's logic but all
		// paths under incomingDir; final failure returns the pair-corrupt error.
		refreshed = true
	}
	// PUBLISH: only a fully verified pair leaves quarantine. Pack first, then
	// idx — the idx rename is the commit point (git cannot see a pack without
	// its index), which is what makes the two renames atomic to the traversal.
	if err := os.Rename(inPack, filepath.Join(packDir, packName)); err != nil {
		return refreshed, err
	}
	if err := os.Rename(inIdx, filepath.Join(packDir, stem+".idx")); err != nil {
		return refreshed, err
	}
	return refreshed, nil
}
```
(The sketch compresses the existing heal branch — keep its exact semantics, relocated to incoming paths; `os.Rename` replaces an existing destination file on Windows in Go, which the mid-round refresh relies on.) Update `Fetch`'s call site with the extra arg. Delete the now-dead removal choreography and the RESIDUE RULE comment block, replacing it with a two-line quarantine comment pointing at the spec.

- [ ] **Step 4: Run the full repo suite** (`go test ./internal/repo/ -count=1`) — the Stage 4 polish trace test (`.pack` before re-downloaded `.idx`) must still pass unmodified; if it needed edits, the refactor changed observable ordering and is wrong.

- [ ] **Step 5: Deliberate regression.** Temporarily swap the two renames (idx first) and confirm `TestQuarantinePublishesPackThenIdxNeverIdxAlone` fails; revert.

- [ ] **Step 6: Commit.**
```bash
git add internal/repo
git commit -m "refactor(fetch): quarantine staging — verify in incoming, publish pack-then-idx; residue rule deleted"
```

---

### Task 7: Ref-name validator (+ git parity) and transport folder fidelity (+ contract rows)

**Files:**
- Create: `internal/repo/refname.go`, `internal/repo/refname_test.go`
- Modify: `internal/gitcmd/gitcmd.go` (add `CheckRefFormat`)
- Modify: `internal/transport/fake.go`, `fake_test.go`
- Modify: `internal/transport/contract_test.go`

**Interfaces:**
- Consumes: `checkStageableLeaf` (marker.go) — reused per component.
- Produces:
  - `repo.CheckRefName(name string) error` — full documented `git-check-ref-format` rule set, in-process, no subprocess.
  - `repo.advertisableName(name string) error` — `CheckRefName` + per-component `checkStageableLeaf` (components split on `/` first, so the leaf check's `/\` rejection never fires spuriously). Used by Task 8's read boundary and Task 9a's push validation.
  - `gitcmd.CheckRefFormat(name string) (bool, error)` — runs `git check-ref-format -- <name>`; `(true,nil)` exit 0, `(false,nil)` exit 1, error otherwise. Authority at the push boundary and the parity oracle.
  - Fake fidelity: `Trash` on a folder removes the folder and its entire subtree (Committed); `CreateExclusive`/`UpdateRevision` where `Dirs[p]` → `Refused`; `EnsureDir` where `Files[p]` exists → error containing "a file exists at"; `EnsureDir` with a missing parent → error containing `notFoundSignature`'s text shape (`Node not found: <name>`) — Tasks 9–11 depend on all four.

- [ ] **Step 1: Write the failing validator tests** (refname_test.go):
```go
// RED: table test, both directions. Accept: refs/heads/main, refs/heads/feature/x,
// refs/tags/v1/rc, refs/notes/commits, refs/stash, refs/heads/a.b, and
// refs/heads/a{b} — braces are a STAGEABILITY refusal, not a git-validity one,
// so CheckRefName accepts them and advertisableName rejects them (the split
// the parity test depends on). Reject: refs/heads/.hidden (leading-dot component — the
// round-3 catch), refs/heads/a..b, refs/heads/a.lock, refs/heads/a/, refs//x,
// refs/heads/a\b, refs/heads/@{, refs/heads/a b, refs/heads/a:b, refs/heads/a~b,
// refs/heads/a^b, refs/heads/a?b, refs/heads/a*b, refs/heads/a[b, @, refs/heads/a.,
// control chars, empty, "/refs/heads/x".
func TestCheckRefNameRules(t *testing.T)

// RED (parity — the round-3 mandate): every fixture above, accept AND reject,
// through gitcmd.CheckRefFormat; verdicts must be identical to CheckRefName's.
// Drift is a test failure, not a silent divergence.
func TestCheckRefNameParityWithGit(t *testing.T)

// RED: advertisableName additionally rejects brace components (refs/heads/a{b}/c),
// Windows device-name components (refs/heads/con/x, refs/heads/x/aux), and accepts
// everything CheckRefName-valid whose components are stageable.
func TestAdvertisableName(t *testing.T)
```

- [ ] **Step 2: Run, expect FAIL** (functions undefined).

- [ ] **Step 3: Implement `CheckRefName`** (refname.go). One pass over components:
```go
// CheckRefName implements the documented git-check-ref-format rule set
// in-process. The NORMATIVE BAR IS GIT'S RULE SET, NOT THIS LIST (spec, round
// 3: a prose list omitted leading-dot components); the parity test in
// refname_test.go is what keeps this honest — extend fixtures before logic.
func CheckRefName(name string) error {
	if name == "" || name == "@" || strings.HasPrefix(name, "/") ||
		strings.HasSuffix(name, "/") || strings.Contains(name, "//") {
		return fmt.Errorf("invalid ref name %q", name)
	}
	if strings.HasSuffix(name, ".") || strings.Contains(name, "..") ||
		strings.Contains(name, "@{") {
		return fmt.Errorf("invalid ref name %q", name)
	}
	for _, c := range []byte(name) {
		if c < 0x20 || c == 0x7f || strings.IndexByte(" ~^:?*[\\", rune(c)|0) >= 0 { // hypothesis: verify byte-set idiom compiles; simplest is strings.ContainsAny per name
			return fmt.Errorf("invalid ref name %q: forbidden character", name)
		}
	}
	for _, comp := range strings.Split(name, "/") {
		if strings.HasPrefix(comp, ".") || strings.HasSuffix(comp, ".lock") {
			return fmt.Errorf("invalid ref name %q: component %q", name, comp)
		}
	}
	return nil
}
```
(The byte-scan line is a known-awkward sketch — implement with `strings.ContainsAny(name, " ~^:?*[\\")` plus an explicit control-char scan; the fixtures decide.) `advertisableName` = `CheckRefName` + `for _, comp := range strings.Split(name, "/") { checkStageableLeaf(comp) }`. `gitcmd.CheckRefFormat` follows the `IsAncestor` exit-code pattern at `gitcmd.go:139`.

- [ ] **Step 4: Write the failing Fake-fidelity tests** (fake_test.go): folder trash removes subtree (`Files` under prefix and nested `Dirs` gone; `Committed`); trash of empty dir; `CreateExclusive` onto a dir path → `Refused`; `EnsureDir` onto a file path → error naming the file; `EnsureDir` under a missing parent → error embedding `Node not found: <parent leaf>`; `Stat` on dir unchanged. Then implement in fake.go:
```go
func (f *Fake) Trash(p string) (Outcome, error) {
	found := false
	if _, ok := f.Files[p]; ok { delete(f.Files, p); found = true }
	if f.Dirs[p] { delete(f.Dirs, p); found = true }
	for k := range f.Files { if strings.HasPrefix(k, p+"/") { delete(f.Files, k); found = true } }
	for k := range f.Dirs  { if strings.HasPrefix(k, p+"/") { delete(f.Dirs, k); found = true } }
	_ = found // already-absent is still Committed (desired end state)
	return Committed, nil
}
```
`EnsureDir`: error if `Files[p]` exists ("create-folder failed: a file exists at %s"); error if the parent (`path.Dir(p)`) is neither a mount root (`/my-files`, `/devices`, or depth ≤ 2 under `/devices`) nor present in `Dirs`/implied by `Files` — message `create-folder %s in %s failed: Node not found: %s` mirroring the live shape. **Expect fixture churn:** existing repo tests that relied on lax `EnsureDir` may now need their Fake seeded with the root's parents — seed via `f.Dirs["/my-files/r"] = true`-style lines in test setup, never by weakening the Fake.
`CreateExclusive`/`UpdateRevision`: before the `Files[p]` check, `if f.Dirs[p] { return Refused, nil }` (name taken by a folder — the D/F collision Task 9b heals; the live contract row pins the real CLI's shape and this models the conservative reading).

- [ ] **Step 5: Run the ENTIRE suite** (`go test ./... -count=1`) and fix fixture fallout from the stricter Fake before proceeding — this fallout is the fidelity paying rent, list every seeded parent in the task report.

- [ ] **Step 6: Add live contract rows** (contract_test.go, fake half asserted now, live half at gate): `Trash` on an empty folder (outcome shape); `Trash` on a folder with children (recursive, outcome shape); `create-folder` colliding with an existing file (error shape); `upload` of a file colliding with an existing folder name (outcome/error shape — **unverified on the real CLI until the gate; the fake models Refused**); nested `List` (folder containing folders and files, correct `IsDir` per row).

- [ ] **Step 7: Commit.**
```bash
git add internal/repo/refname.go internal/repo/refname_test.go internal/gitcmd internal/transport
git commit -m "feat: in-process ref-name validator with git parity; Fake folder fidelity + contract rows"
```

---

### Task 8: Recursive `ListRefs` with skip-notes + namespace re-enable in `checkDst`

**Files:**
- Modify: `internal/repo/refs.go:20-51` (`ListRefs`)
- Modify: `internal/repo/push.go:277-304` (`isBranch` untouched; `checkDst` rewritten)
- Modify: `internal/repo/repo_test.go`

**Interfaces:**
- Consumes: `advertisableName` (Task 7), `readRef`, `transport.List`.
- Produces: `ListRefs(t, root)` — same signature, now recursing the whole `refs/` tree; skipped names go to stderr as `git-remote-proton: skipping <root>/<name>: <reason>`, never fatal, never advertised. `checkDst(dst string) error` — accepts any `advertisableName`-valid name under `refs/`; rejects everything else (pseudorefs, non-`refs/` destinations) with a named reason. New helper `requiresForce(dst string) bool` — true for any destination outside `refs/heads/` and `refs/tags/` (the design's conservative other-namespace rule); Task 9a enforces it.

- [ ] **Step 1: Write the failing tests:**
```go
// RED: nested branches, tags, notes, and refs/stash all advertised with full names.
func TestListRefsRecursesAllNamespaces(t *testing.T)
// Fake seeded: refs/heads/main, refs/heads/feature/x, refs/tags/v1/rc,
// refs/notes/commits, refs/stash — each a valid 40-hex file. Expect all five keys.

// RED: a foreign junk name is SKIPPED with a stderr note naming the exact remote
// path, and everything else still advertises — one stray web-UI file must not
// brick the repo (spec round 2).
func TestListRefsSkipsInvalidNamesWithNoteNeverFatal(t *testing.T)
// Seed refs/heads/.hidden and refs/heads/a{b} beside refs/heads/main; capture
// stderr (ListRefs writes via a package-level warn func? NO — it writes to
// os.Stderr today per convention; route the note through an io.Writer param?
// Keep convention: notes to os.Stderr, assert the MAP result only (main is
// present, junk absent) and pin the note text in a focused unit test of the
// skip helper with an injected writer).

// GUARD: malformed CONTENTS of a well-named ref stay fatal (existing rule).
func TestListRefsMalformedContentStillFatal(t *testing.T)

// RED: empty folders contribute nothing and do not error.
func TestListRefsIgnoresEmptyFolders(t *testing.T)
```

- [ ] **Step 2: Run, expect FAIL** (nested names invisible today).

- [ ] **Step 3: Implement recursive `ListRefs`:**
```go
func ListRefs(t transport.Transport, root string) (map[string]string, error) {
	out := map[string]string{}
	var walk func(rel string) error
	walk = func(rel string) error {
		nodes, err := t.List(root + "/" + rel)
		if err != nil {
			return err
		}
		for _, n := range nodes {
			full := rel + "/" + n.Name
			if n.IsDir {
				if err := walk(full); err != nil {
					return err
				}
				continue
			}
			if err := advertisableName(full); err != nil {
				// Skip-with-note, NEVER fatal: a foreign name must not deny
				// service to the whole repo (spec §1). v2 itself can never
				// create one of these (push refuses them), so this always
				// marks foreign data.
				fmt.Fprintf(os.Stderr, "git-remote-proton: skipping %s/%s: %v\n", root, full, err)
				continue
			}
			sha, err := readRef(t, root+"/"+full)
			if err != nil {
				return err // malformed CONTENT of an advertised ref stays fatal
			}
			out[full] = sha
		}
		return nil
	}
	if err := walk("refs"); err != nil {
		return nil, err
	}
	return out, nil
}
```
Delete the Stage-2 boundary comment block; replace with the cost note ("one List subprocess per folder, serial; in-process name checks per ref — linear, stated in the design doc beside the pack-count note").

- [ ] **Step 4: Rewrite `checkDst`:**
```go
// checkDst admits any advertisable name under refs/. The v6.1 narrowing is
// retired: recursive ListRefs erased its first justification, batch preflight
// (Task 9a) its second. Pseudorefs and non-refs/ destinations stay rejected.
func checkDst(dst string) error {
	if !strings.HasPrefix(dst, "refs/") {
		return fmt.Errorf("unsupported destination %q: only refs under refs/ are served "+
			"(pseudorefs and other destinations have no representation on this remote)", dst)
	}
	return advertisableName(dst)
}

// requiresForce: the design's conservative deviation — any move outside
// refs/heads/* and refs/tags/* requires force (v2 does not inspect object
// types the way git's own namespace rules do; conservative cannot lose data).
func requiresForce(dst string) bool {
	return !strings.HasPrefix(dst, "refs/heads/") && !strings.HasPrefix(dst, "refs/tags/")
}
```
`requiresForce` is dead code until Task 9a wires it — mark with a `// wired in Task 9a` comment so the reviewer of THIS task doesn't flag it, and add its unit test now.

- [ ] **Step 5: Run full suite; fix advertisement-dependent tests** (SetHead's `ListRefs` callers etc. — behaviour is a superset, expect little fallout). Commit.
```bash
git add internal/repo
git commit -m "feat(repo): recursive ref advertisement with skip-notes; namespace re-enable in checkDst"
```

---

### Task 9a: Five-phase batch engine

**Files:**
- Modify: `internal/gitcmd/gitcmd.go:201` (`WritePack` multi-want) + `gitcmd_test.go`
- Modify: `internal/repo/push.go` (Push restructured; pushOne dissolved into phases)
- Modify: `internal/repo/refs.go:90-124` (`WriteRef` gains parent creation)
- Modify: `internal/repo/repo_test.go`

**Interfaces:**
- Consumes: `checkDst`/`requiresForce`/`advertisableName` (Tasks 7–8); `gitcmd` plumbing; `WriteRef`.
- Produces:
  - `gitcmd.WritePack(gitDir string, wants []string, haves []string, outDir string) (string, string, error)` — wants plural; empty wants → `("", "", nil)`.
  - `repo.Push(t, root, gitDir, ups, remote) []Result` — same signature, five-phase internals; **one pack per batch** (this matches the design doc's normative "Object transfer per batch" text, which Stage 2's per-ref packing quietly diverged from; the haves-trust hazard documented in the old `pushOne` comment does not arise because all wants share one pack against pre-batch haves).
  - `repo.ensureRefParents(t, root, ref string) error` — `EnsureDir` walk over the ref's namespace components strictly below `root` (`refs/heads/feature` for `refs/heads/feature/x`). Reverse-D/F: an `EnsureDir` failure naming a file collision surfaces as the named refusal.
- The delete arm keeps Stage 4's HEAD-protection logic verbatim; prune/self-heal arrive in Task 9b.

- [ ] **Step 1: `WritePack` multi-want.** RED test `TestWritePackMultipleWantsOnePack` (two branches with disjoint commits → one pack containing both closures; `countPacks == 1`); GUARD `TestWritePackNoWantsIsNoPack` (empty wants → `("","",nil)`). Implement: parameter `want string` → `wants []string`, rev-list invocation gains all wants; update every call site and existing WritePack tests mechanically (`[]string{sha}`).

- [ ] **Step 2: Write the failing batch-engine tests** (repo_test.go, all against Fake + real gitDir fixtures, all RED unless marked):
```go
// Phase 2 validation, all pre-mutation:
func TestPushRefusesDuplicateDestinationsWholeBatchUntouched(t *testing.T)
func TestPushFinalStateDFPreflightRefusesConflictingCreates(t *testing.T)
//   batch: create refs/heads/feature AND refs/heads/feature/x → both error with
//   a named D/F reason; remote listing unchanged; NO pack was built (assert via
//   Fake: no /packs children added).
func TestPushPreflightDFAgainstExistingRefs(t *testing.T)
//   remote has refs/heads/feature/x (not deleted in batch); create refs/heads/feature
//   → that create errors at preflight; an unrelated ref in the same batch succeeds.
func TestPushOtherNamespaceRequiresForce(t *testing.T)
//   update refs/notes/commits without force → error naming the force requirement;
//   with Force=true → ok. Create without force → ok (create is not a move).

// Phase ordering:
func TestPushDeletionsRunAfterPackConfirmBeforeCreates(t *testing.T)
//   batch: delete refs/heads/old + create refs/heads/feature/x (git-order:
//   create first). Assert via trace: pack upload precedes the Trash, which
//   precedes the ref write. This is the [Both] round-1 ordering rule.
func TestPushPackFailureFailsCreatesButDeletionsProceed(t *testing.T)
//   FailNext on the pack upload: every create/update errors naming the pack
//   failure; the batch's deletion still executes and reports its own ok
//   (spec: phase-3 failure CONTINUES into phase 4 — adjudicated round 2).
func TestPushDeleteThenCreateSameNameOneBatch(t *testing.T)
//   remote has refs/heads/feature; batch deletes it and creates
//   refs/heads/feature/x → both ok (deletes-before-creates makes room; the
//   preflight's final-state set contains only feature/x, no conflict).

// Hierarchical create end to end:
func TestPushCreatesNestedBranchCreatingParents(t *testing.T)
//   create refs/heads/feature/deep/x on a fresh remote → ok; Fake shows dirs
//   refs/heads/feature and refs/heads/feature/deep and the ref file.
func TestPushReverseDFRefusedNamingTheBlockingRef(t *testing.T)
//   remote has file refs/heads/feature; create refs/heads/feature/x (no delete
//   in batch) → error naming refs/heads/feature as the blocking ref.
```

- [ ] **Step 3: Run, expect FAIL** across the board (`go test ./internal/repo/ -run TestPush -count=1`).

- [ ] **Step 4: Implement the engine.** Shape (pushOne's per-ref logic redistributed, not rewritten — lift its blocks verbatim where possible):
```go
func Push(t transport.Transport, root, gitDir string, ups []protocol.RefUpdate, remote map[string]string) []Result {
	results := make([]Result, len(ups))
	failed := func(i int, msg string) { results[i] = Result{Ref: ups[i].Dst, Err: oneLine(msg)} }

	// ---- phase 2: whole-batch validation, nothing has moved --------------
	seenDst := map[string]int{}
	valid := make([]bool, len(ups))
	newShas := make([]string, len(ups))
	for i, u := range ups {
		if err := checkDst(u.Dst); err != nil { failed(i, err.Error()); continue }
		if j, dup := seenDst[u.Dst]; dup {
			failed(i, fmt.Sprintf("duplicate destination (also update %d)", j)); continue
		}
		seenDst[u.Dst] = i
		if u.Src == "" { valid[i] = true; continue } // deletes validate name-only here
		// resolve + object-type + ancestry + force rule: lifted from pushOne
		// verbatim (resolve, isBranch/ObjectType, HasObject/IsAncestor), plus:
		if requiresForce(u.Dst) && !u.Force {
			if _, exists := remote[u.Dst]; exists {
				failed(i, "updating refs outside refs/heads/ and refs/tags/ requires force "+
					"(conservative rule; see design)"); continue
			}
		}
		newShas[i] = /* resolved sha */ ""
		valid[i] = true
	}
	// final-state D/F preflight over REFS ONLY (empty folders are runtime,
	// self-heal's job): finalSet := remote − valid deletes + valid creates/updates;
	// for each valid CREATE c, if any other name in finalSet == c or is
	// c+"/"-prefixed or c is name+"/"-prefixed → failed(...) naming both refs.

	// ---- phase 3: one pack for every valid create/update -----------------
	// wants := newShas of valid non-deletes; haves := pre-batch remote tips
	// present locally (same filter as today). One WritePack + publishPack +
	// publishIdx. On ANY failure: mark every valid non-delete failed with the
	// pack error; deletions still proceed (phase-3-continues rule).

	// ---- phase 4: deletions (HEAD guard verbatim from pushOne) ------------
	// ---- phase 5: creates/updates: ensureRefParents + WriteRef (+Task 9b heal)
	// ensureHEAD(...) unchanged, after all phases.
	return results
}
```
`ensureRefParents`: split `ref` on `/`, walk `root+"/refs"`, `root+"/refs/heads"`, … `EnsureDir` each prefix above the leaf (the first two exist from init; `EnsureDir` is Stat-then-create so that's one Stat each — acceptable; do NOT special-case them away, partial init is a real state). An `EnsureDir` error whose text names a file collision ("a file exists at" from the Fake / the live shape from the contract row) is rewrapped: `creating %s requires folder %s, but branch/ref %s occupies that name (directory/file conflict; delete it first)`.

- [ ] **Step 5: Run the full suite** — expect fallout in every existing push test that asserted per-ref pack behaviour; adapt assertions to batch-level (the multi-ref same-commit reconciliation-cost tests from Stage 3b: one pack now uploads ONCE — those tests' premise is obsolete; update them to assert the new single-upload behaviour and note it in the task report as a deliberate design-doc-aligned change).

- [ ] **Step 6: Deliberate regressions:** (a) reorder phase 4 before phase 3 → `TestPushDeletionsRunAfterPackConfirmBeforeCreates` fails; (b) drop the duplicate-dst check → its test fails. Revert both.

- [ ] **Step 7: Commit.**
```bash
git add internal/repo internal/gitcmd
git commit -m "feat(push): five-phase batch engine — whole-batch preflight, one pack per batch, deletions after pack confirm"
```

---

### Task 9b: Prune on delete, self-heal on create, EnsureDir contradiction handling

**Files:**
- Modify: `internal/repo/push.go` (delete arm + create arm)
- Modify: `internal/transport/cli.go:220-239` (`EnsureDir` re-Stat) + `cli_test.go` (roles)
- Modify: `internal/repo/repo_test.go`

**Interfaces:**
- Consumes: Task 7's Fake fidelity; Task 4's Stat classification; Task 9a's phase structure.
- Produces:
  - `pruneEmptyParents(t transport.Transport, root, ref string)` — best-effort, void; stderr notes on anything not `Committed`.
  - `createRefHealingCollision(t transport.Transport, root, ref, sha string) (transport.Outcome, error)` — wraps `WriteRef(create)`; on a non-Committed outcome with a folder at the name, applies the subtree no-files-of-any-kind rule, healing or refusing; phase 5 calls this for creates.
  - `CLI.EnsureDir` re-Stats once on an already-exists create-folder failure (folder → proceed; file → error naming the file; absent → error quoting both observations verbatim). Constant `alreadyExistsSignature` (value pinned by helper-role now, live contract row at gate).

- [ ] **Step 1: Write the failing tests:**
```go
// RED: deleting refs/heads/feature/x trashes the ref AND the now-empty
// feature/ folder; refs/heads itself survives.
func TestDeletePrunesEmptyParentsStopsAtNamespaceRoots(t *testing.T)
// RED: prune stops at the first non-empty parent (sibling ref present).
func TestDeletePruneStopsAtNonEmptyParent(t *testing.T)
// RED: prune may trash refs/notes when the last note is deleted (namespace
// roots other than heads/tags are prunable).
func TestDeletePrunesOnDemandNamespaceRoot(t *testing.T)
// GUARD: a prune Trash failure is a stderr note, not a delete failure — the
// ref deletion itself still reports ok (best-effort rule).
func TestPruneFailureIsAdvisoryOnly(t *testing.T)

// RED: the D/F reuse workflow — leftover EMPTY folder at refs/heads/feature
// (simulating a crashed prune), then create branch feature → heals: folder
// trashed, create retried once, ok.
func TestCreateSelfHealsEmptyFolderCollision(t *testing.T)
// RED (round-2 [Both] catch): leftover feature/ containing ONLY the empty
// folder feature/x/ — nested empties heal too ("contains no files", not
// "first level empty").
func TestCreateSelfHealsNestedEmptyResidue(t *testing.T)
// RED (round-3 Gemini catch): feature/ containing a FOREIGN file
// (feature/notes.txt) → refuse; the message names notes.txt AS FOREIGN DATA,
// not as a ref; NOTHING is trashed (assert the file survives).
func TestCreateRefusesFolderWithForeignFileUntouched(t *testing.T)
// RED: feature/ containing a real sub-ref → refuse naming refs/heads/feature/x
// as a conflicting ref.
func TestCreateRefusesFolderWithLiveSubRefs(t *testing.T)
// GUARD: heal's diagnostic Stat/List failing → that transport error, no heal.
func TestSelfHealAbortsOnDiagnosticFailure(t *testing.T)

// EnsureDir contradiction (cli_test.go, helper roles):
// RED: role answers info→not-found then create-folder→"already exists" then
// info→folder: EnsureDir succeeds (re-Stat found it).
func TestEnsureDirContradictionResolvedByReStatFolder(t *testing.T)
// RED: …then info→FILE: error naming the directory/file conflict.
func TestEnsureDirContradictionFileIsNamedConflict(t *testing.T)
// RED: …then info→still not-found: error QUOTING BOTH observations
// (the C17 signature, diagnosable; explicitly generic robustness, and the
// test comment must say C17b forbids claiming it as a validated fix).
func TestEnsureDirContradictionUnresolvedQuotesBoth(t *testing.T)
```

- [ ] **Step 2: Run, expect FAIL.**

- [ ] **Step 3: Implement prune:**
```go
// pruneEmptyParents is BEST-EFFORT tidiness under the batch lock. Check-then-
// act: no conditional delete exists on this transport (same accepted limit as
// lock release); the blast radius of the race is bounded — subtree goes to
// Proton's trash, and only a NON-v2 actor can be racing (v2 writers hold the
// lock). Self-heal (createRefHealingCollision) is the correctness mechanism;
// a plan that ships prune without heal reintroduces the wedge (spec §2c).
func pruneEmptyParents(t transport.Transport, root, ref string) {
	protected := map[string]bool{"refs": true, "refs/heads": true, "refs/tags": true}
	for dir := parentOf(ref); dir != "" && !protected[dir]; dir = parentOf(dir) {
		nodes, err := t.List(root + "/" + dir)
		if err != nil || len(nodes) != 0 {
			if err != nil {
				fmt.Fprintf(os.Stderr, "git-remote-proton: prune: cannot list %s/%s: %v\n", root, dir, err)
			}
			return
		}
		out, err := t.Trash(root + "/" + dir)
		if err != nil || out != transport.Committed {
			fmt.Fprintf(os.Stderr, "git-remote-proton: prune: leaving empty folder %s/%s "+
				"(trash reported %v/%s); a later create at this name self-heals\n", root, dir, err, out)
			return
		}
	}
}
```
Call from the delete arm after a Committed Trash of the ref file. `parentOf` = `strings.LastIndex(ref, "/")` slice, "" at top.

- [ ] **Step 4: Implement self-heal:**
```go
// subtreeFiles returns the full remote path of every FILE anywhere under
// folder (recursive walk via t.List; folders alone do not count). Used by
// self-heal's residue test: the rule is "contains no files OF ANY KIND" — a
// foreign file makes the subtree not-residue and must never be trashed
// (spec §2c, round 3).
func subtreeFiles(t transport.Transport, folder string) ([]string, error)

func createRefHealingCollision(t transport.Transport, root, ref, sha string) (transport.Outcome, error) {
	out, err := WriteRef(t, root, ref, sha, false)
	if err == nil && out == transport.Committed {
		return out, nil
	}
	n, ok, serr := t.Stat(root + "/" + ref)
	if serr != nil {
		return transport.Ambiguous, fmt.Errorf("create of %s did not commit and its "+
			"diagnosis failed: %v (original: %v)", ref, serr, err)
	}
	if !ok || !n.IsDir {
		return out, err // not a folder collision: surface the original result
	}
	files, ferr := subtreeFiles(t, root+"/"+ref)
	if ferr != nil { return transport.Ambiguous, fmt.Errorf(...no heal, name ferr...) }
	if len(files) > 0 {
		// partition into ref-shaped names vs foreign; name each as what it is
		return transport.Refused, fmt.Errorf("a folder occupies %s and its contents block "+
			"the branch: %s", ref, describeBlockers(files))
	}
	if tout, terr := t.Trash(root + "/" + ref); terr != nil || tout != transport.Committed {
		return transport.Ambiguous, fmt.Errorf("empty folder at %s could not be cleared "+
			"(trash reported %v/%s); refusing to create over unknown state", ref, terr, tout)
	}
	fmt.Fprintf(os.Stderr, "git-remote-proton: cleared leftover empty folder at %s "+
		"(residue of an interrupted delete)\n", ref)
	return WriteRef(t, root, ref, sha, false) // retry ONCE
}
```
Phase 5 uses this for creates (`exists == false`) only; updates keep plain `WriteRef`.

- [ ] **Step 5: Implement the `EnsureDir` re-Stat** in cli.go: on `code != 0` from create-folder, `if strings.Contains(out, alreadyExistsSignature)` → re-`Stat(p)`: `(dir,true)` → `return nil`; `(file,true)` → error `create-folder %s: a file occupies that name`; `(_,false)` → error quoting both the create-folder output and the re-Stat verbatim. `const alreadyExistsSignature = "already exists"` — **hypothesis about the CLI's wording; the helper role pins the code path, the live contract row (add it now, gate-run later) pins the real text.** Comment must carry the C17b framing: generic robustness, never a validated live fix.

- [ ] **Step 6: Run full suite + deliberate regressions:** (a) change `subtreeFiles`' rule to first-level-only → nested-empty test fails; (b) change it to ref-files-only → foreign-file test fails; (c) remove the prune call → prune tests fail. Revert each.

- [ ] **Step 7: Commit.**
```bash
git add internal/repo internal/transport
git commit -m "feat(push): prune empty parents on delete; self-heal folder collisions on create; EnsureDir contradiction re-Stat"
```

---

### Task 10: `--set-head` hierarchical names + exact-path verification

**Files:**
- Modify: `internal/repo/sethead.go:41-58, 84-100`
- Modify: `internal/repo/repo_test.go`

**Interfaces:**
- Consumes: `advertisableName`; `readRef`; `Stat`.
- Produces: `normalizeBranch` accepts slashes (validates via `advertisableName` on the full name); `SetHead` verifies the target via exact-path `Stat` + `readRef` — no tree recursion for the happy path. The branch-suggestion list on failure still uses `ListRefs` (now recursive — acceptable cost on an error path, and the suggestions now include nested branches).

- [ ] **Step 1: Failing tests:** `TestSetHeadAcceptsHierarchicalBranch` (Fake with `refs/heads/feature/x`; `SetHead(..., "feature/x")` → sets `ref: refs/heads/feature/x`); `TestSetHeadRejectsInvalidHierarchicalName` (`"feature//x"`, `"feature/.hidden"` → named refusal, not the old Stage 5 message); GUARD `TestSetHeadVerifyUsesExactPathNotRecursion` (trace: no `List` of `refs/tags` or `refs/notes` on a successful set — assert via traced Fake that only the target ref path was read); GUARD: missing-branch error still lists existing branches including nested ones.

- [ ] **Step 2: Run, expect FAIL** (slash refusal fires).

- [ ] **Step 3: Implement.** `normalizeBranch`: drop the `ContainsAny(leaf, "/\\")` arm and the single-leaf `checkStageableLeaf`; after prefix normalization run `advertisableName(b)` (which covers per-component stageability and git validity; backslash is rejected by `CheckRefName`). In `SetHead` replace the `ListRefs` + map lookup with:
```go
if _, ok, err := t.Stat(root + "/" + branch); err != nil {
	return "", err
} else if !ok {
	refs, lerr := ListRefs(t, root) // error path only: build the suggestion list
	...existing no-such-branch message...
}
if _, err := readRef(t, root+"/"+branch); err != nil {
	return "", err // exists but corrupt: fatal, never coerced
}
```
(Stat absence is trustworthy post-Task 4.) Everything else — lock, idempotence order, `UpdateHEAD` — untouched.

- [ ] **Step 4: Run suite; commit.**
```bash
git add internal/repo
git commit -m "feat(set-head): hierarchical branch names; exact-path existence check"
```

---

### Task 11: `GPB_CREATE_PARENTS` opt-in parent auto-create

**Files:**
- Create: `internal/repo/parents.go`
- Modify: `cmd/git-remote-proton/main.go` (env const + wiring in `list for-push` arm)
- Modify: `internal/repo/repo_test.go`, `cmd/git-remote-proton/main_test.go`

**Interfaces:**
- Consumes: Task 4's Stat classification (absent vs error); Task 7's Fake parent fidelity.
- Produces: `repo.EnsureParents(t transport.Transport, root string, create bool, stderr io.Writer) error`; `const createParentsEnv = "GPB_CREATE_PARENTS"` in main.go beside `uncertifiedCLIEnv`. Called ONLY from the `list for-push` arm before `repo.Bootstrap`; **never** from read paths and **never** from `runSetHead` (spec §3/§6: a repo cannot exist below a missing parent, so the var there could only manufacture folders and then fail).

- [ ] **Step 1: Failing tests:**
```go
// RED: default (create=false), parent missing → error naming the FIRST missing
// parent and BOTH remedies in the CLI's real grammar (create-folder takes
// parent + name, not a path) plus the env var.
func TestEnsureParentsRefusalIsActionable(t *testing.T)
//   root /my-files/GitRemotes/repo on a Fake without GitRemotes; assert error
//   contains "proton-drive filesystem create-folder /my-files GitRemotes"
//   and "GPB_CREATE_PARENTS=1".
// RED: create=true → parents created, one stderr line per folder, then nil.
func TestEnsureParentsCreatesWithLoudNotes(t *testing.T)
//   root /my-files/a/b/repo → creates a then b (repo itself is Bootstrap's,
//   NOT created here — assert absent), two stderr lines.
// RED: walk bounds — never creates the mounts themselves.
func TestEnsureParentsNeverCreatesMountRoots(t *testing.T)
//   /my-files: parent walk starts BELOW it. /devices/<id>/x/repo: <id> missing
//   → refusal even with create=true (a device mount is not creatable storage).
// GUARD: no rollback — creation fails at b after a succeeded → a remains,
// stderr says what was created (spec: rollback would be the unsafe race).
func TestEnsureParentsPartialCreationIsKeptAndReported(t *testing.T)
// GUARD: wiring — protocol path honours the env var; set-head does not.
//   (main_test: loop-level test with env set via t.Setenv; sethead path
//   asserts EnsureParents is never reached — no parent gets created by a
//   set-head against a missing tree, the marker refusal fires instead.)
```

- [ ] **Step 2: Run, expect FAIL.**

- [ ] **Step 3: Implement `EnsureParents`:**
```go
func EnsureParents(t transport.Transport, root string, create bool, stderr io.Writer) error {
	parts := strings.Split(strings.TrimPrefix(root, "/"), "/") // canonical already
	protectedDepth := 1                 // /my-files
	if parts[0] == "devices" {
		protectedDepth = 2              // /devices/<device-id>
	}
	prefix := "/" + strings.Join(parts[:protectedDepth], "/")
	for i := protectedDepth; i < len(parts)-1; i++ { // parents ONLY; the leaf is Bootstrap's
		prefix += "/" + parts[i]
		_, ok, err := t.Stat(prefix)
		if err != nil {
			return fmt.Errorf("checking parent folder %s: %w", prefix, err)
		}
		if ok {
			continue
		}
		if !create {
			parent, name := prefix[:strings.LastIndex(prefix, "/")], parts[i]
			return fmt.Errorf("parent folder %s does not exist; create it first "+
				"(proton-drive filesystem create-folder %s %s, or the web UI), or set "+
				"%s=1 to let the helper create missing parents", prefix, parent, name, "GPB_CREATE_PARENTS")
		}
		if err := t.EnsureDir(prefix); err != nil {
			return fmt.Errorf("creating parent folder %s (GPB_CREATE_PARENTS=1): %w", prefix, err)
		}
		fmt.Fprintf(stderr, "git-remote-proton: created parent folder %s (GPB_CREATE_PARENTS=1)\n", prefix)
	}
	// The mount itself missing (e.g. /devices/<id> absent) surfaces naturally:
	// the first child Stat errors or the leaf EnsureDir in Bootstrap fails with
	// the CLI's own Node-not-found — with create=false the loop above already
	// refused at the first absent parent below the mount.
	return nil
}
```
Wire in main.go's `list for-push` arm, first line: `if err := repo.EnsureParents(t, root, os.Getenv(createParentsEnv) == "1", os.Stderr); err != nil { warn(err); return 1 }`. **Also handle the `create=false` + missing-parent case for the leaf itself:** `Bootstrap`'s `EnsureDir(root)` failure with the not-found signature should be wrapped by `EnsureParents`' actionable message — cleanest: `EnsureParents` also Stats the leaf's PARENT chain only; if the chain exists but `Bootstrap` still fails, that error stands on its own. Confirm with a loop-level test: fresh Fake with `/my-files` only, push flow without the env → the actionable message (not the raw create-folder error R2-1 showed).

- [ ] **Step 4: Run full suite; commit.**
```bash
git add internal/repo cmd/git-remote-proton
git commit -m "feat: GPB_CREATE_PARENTS opt-in parent auto-create; actionable refusal by default"
```

---

### Task 12: Design doc v6.5, README, CHANGELOG

**Files:**
- Modify: `docs/v2-remote-helper-design.md`
- Modify: `README.md`
- Modify: `CHANGELOG.md`

**Interfaces:** consumes every shipped behaviour above; produces the v6.5 revision the gate brief cites. **Write this only after Tasks 4–11 are merged into the stage branch** — the doc records shipped behaviour, and Stage 4's amendment history shows what describing unshipped code costs.

- [ ] **Step 1: v6.5 edits, one revision entry, each item verified against the code as merged** (spec component 9 is the checklist, verbatim — work through it item by item): storage layout (nested `refs/`, component validation 2a, prune/self-heal 2c, EnsureDir contradiction 2d); ref-transition table (namespace row un-narrowed with a v6.1 back-reference, delete row gains prune, two new D/F rows); Push (five-phase order incl. final-state preflight and pack-before-deletions — note the per-ref→per-batch pack reconciliation as a code-to-design ALIGNMENT, citing the old pushOne comment); advertisement (read-boundary skip-with-note validation; cost note beside the pack-count note); Fetch (quarantine staging replaces the residue rule; `listCompletePacks` stderr-note letter aligned to code: packish-looking skips only); utility modes (`--set-head` hierarchical grammar; slash-refusal text superseded); `GPB_CREATE_PARENTS` documented beside `GPB_UNCERTIFIED_CLI`; error table — missing-parent rows (both modes), marker read-failure vs absence, MAX_PATH hint, **and the split of "Malformed ref name or ref-file contents → Fatal" into name (skip at read / refuse at push) vs contents (fatal)** — the round-3 Codex item; leave the merged row and the design contradicts component 1.

- [ ] **Step 2: README:** hierarchical refs now supported (drop any flat-only caveat); `GPB_CREATE_PARENTS` under the env-var section with the typo warning; MAX_PATH section landed in Task 5 — cross-link it.

- [ ] **Step 3: CHANGELOG:** `[Unreleased]` gains the stage's user-visible entries (hierarchical refs incl. notes/replace namespaces; prune/self-heal; GPB_CREATE_PARENTS; Stat diagnostics; MAX_PATH hint; quarantine fetch internals one-liner). Do NOT flip to a version — that is Task 14 following `docs/releasing.md`.

- [ ] **Step 4: Commit.**
```bash
git add docs README.md CHANGELOG.md
git commit -m "docs: design v6.5 — hierarchical refs, quarantine fetch, Stage 5 error-table updates"
```

---

### Task 13 (STRETCH — cuttable without renegotiation): scripted-CLI shim harness

**Files:**
- Create: `internal/testcli/testcli.go` (+ use from `cmd/git-remote-proton/main_test.go`)

**Interfaces:**
- Consumes: the `TestHelperProcess` re-exec pattern already used in `cli_test.go` (extend, don't duplicate).
- Produces: a fake `proton-drive` executable (built via `os.Executable()` re-exec with a role env var) that serves `--version` (certified string) and the `filesystem` verbs (`info`, `list`, `create-folder`, `upload`, `download`, `trash`) against a local directory tree named by env var, mimicking the CLI's JSON shapes pinned in `stage1-results.json`. One end-to-end hermetic test: place the shim on PATH as `proton-drive`, run the REAL `runSetHead` (not the seam) against a pre-seeded tree, assert `HEAD is now refs/heads/feature/x` and the tree's HEAD bytes.

- [ ] Steps: (1) RED end-to-end test; (2) shim serving the minimal verb set with the JSON shapes from `docs/research/probes/stage1-results.json` (transfer summary tuples, `{uid, ok}` trash shape, node JSON incl. `name.value` wrapper); (3) pass; (4) commit `test: scripted proton-drive shim; hermetic end-to-end --set-head`. **If cut:** note in the ledger; Task 3's seam remains the committed wiring coverage.

---

### Task 14: Release prep (v0.4.0 draft) + gate brief — then STOP for Craig

**Files:**
- Modify: `CHANGELOG.md` (the flip, per `docs/releasing.md`)
- Create: `docs/research/gates/stage5-gate-brief.md`

**Interfaces:** consumes `docs/releasing.md` (Task 1), `docs/research/gates/brief-checklist.md`, spec component 8.

- [ ] **Step 1: Write the gate brief** implementing spec component 8's outline verbatim, incorporating `brief-checklist.md` by reference, and carrying: hierarchical end-to-end (push `feature/x` + nested tag + `refs/notes/*`; **`git ls-remote` advertisement assertion** — clone alone does not fetch notes; explicit `git fetch proton-v2 "refs/notes/*:refs/notes/*"` verifying the OID); the D/F workflow (delete `feature/x`, observe prune in the listing, push `feature`); **live self-heal provocation** (manufacture, inside the gate repo only, an empty folder at a branch name → push heals it; a folder containing a ref file → push refuses naming it; the foreign-file variant stays hermetic-only); `--set-head` to a nested branch + delete-protection following HEAD; quarantine no-regression (mid-round pair refresh, zero-`packs/`-download up-to-date re-fetch); `GPB_CREATE_PARENTS` both modes (unset → actionable refusal with listing byte-unchanged as a row set; set → parents created with loud stderr, torn down in cleanup); the new contract rows' live halves (incl. `Stat` not-found and `create-folder` already-exists signatures — if either differs from the shipped constants, BLOCK, report verbatim, never patch); write confinement (all writes under `/my-files/GitRemotes/…`, parent-creation authorisation stated explicitly); trash accounting counts pruned folders; row-set comparisons everywhere; `-count=1`.

- [ ] **Step 2: Flip the CHANGELOG** per `docs/releasing.md` step 2 (`[Unreleased]` → `[v0.4.0] — <date>`), commit exactly that.

- [ ] **Step 3: STOP.** Report to Craig: stage branch ready for merge review; tagging `v0.4.0`, pushing, running the Release workflow, executing the live gate, and publishing are Craig-directed steps per the runbook (never push without Craig's word; the gate runs against the draft's bytes; digest closure after publication).

```bash
git add CHANGELOG.md docs/research/gates/stage5-gate-brief.md
git commit -m "chore(release): v0.4.0 CHANGELOG flip; Stage 5 live gate brief"
```

---

## Self-Review Notes (run before handing the plan to review)

1. **Spec coverage:** components 1 (Tasks 7, 8), 2a–2d (7, 9a, 9b), 3 (10), 4 (6), 5 (4, 5), 6 (11), 7 (1, 2, 3, 13), 8 (every task's tests + 14's brief), 9 (12); execution-note ordering honoured (hygiene 1–3, UX 4–5, quarantine 6 before hierarchical 7–10, parents 11 after hierarchical, stretch 13 before release 14).
2. **Known deliberate divergences to defend in review:** per-ref→per-batch packing (design-doc-aligned; old pushOne comment's haves hazard dissolved, not overruled); `os.Rename`-replaces-existing-on-Windows assumption (verify with a two-line probe test if review doubts it); `alreadyExistsSignature`/`notFoundSignature` are hypotheses pinned live at the gate.
3. **Type consistency spot-checks:** `advertisableName` (Task 7) is the single validation entry used by Tasks 8, 9a, 10; `EnsureParents` writes nothing on read paths; `createRefHealingCollision` returns `(transport.Outcome, error)` matching `WriteRef`.
