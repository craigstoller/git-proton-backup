# git-remote-proton Stage 4 — Productization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make v2 daily-usable: enforce the certified-CLI allowlist with a loud override, add `--set-head` (the only missing user-facing operation), ship a draft-until-gated release pipeline with a real install story, document v1 coexistence, and pay the deferred test/hygiene debt.

**Architecture:** No fetch-path behaviour changes. The allowlist upgrades an existing advisory check in `cmd/git-remote-proton/main.go` to an enforcing one in `internal/transport`. `--set-head` is a utility mode dispatched on a closed argv set before any protocol I/O, orchestrated by a new `repo.SetHead` on top of a new overwrite-capable `repo.UpdateHEAD` (the existing `WriteHEAD` stays backfill-only). Releases build as **draft** GitHub Releases; the live gate runs against the draft's exact bytes and Craig publishes after it passes.

**Tech Stack:** Go stdlib only; PowerShell 7 (`install.ps1`, Pester 5); GitHub Actions (windows-latest, `gh` CLI preinstalled).

**Spec:** `docs/superpowers/specs/2026-08-04-v2-stage4-productization-design.md` — normative. Where this plan and the spec disagree, STOP and report; do not silently pick one.

## Global Constraints

Every task inherits all of these:

- **Go stdlib only.** No new module dependencies; `go.mod` is untouched by this stage.
- **stdout is protocol-only in protocol mode.** Utility modes (`--version`, `--set-head`) may print to stdout — git never invokes the helper with those argv shapes, so interleaving is impossible. Everything else speaks stderr.
- **Fail closed.** Every `transport.Outcome` switch has explicit arms for all three constants plus a refusing `default:`. Never coerce, never guess.
- **Remote name in docs/examples:** `proton-v2`; scheme `proton::`.
- **Every commit message ends with:** `Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>`
- **Never push.** Craig pushes. Commit locally only.
- **PowerShell env, EVERY shell that runs `go`:** first run `$env:Path = [Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [Environment]::GetEnvironmentVariable('Path','User')` (shells carry a stale PATH; Go was installed mid-project). `Set-Location` does not persist between tool calls — use absolute paths or `-C`.
- **gofmt CRLF trap:** `gofmt -l` false-positives on CRLF-rewritten working-tree files (`core.autocrlf=true`). Never "fix" line endings; committed content is clean. Verify formatting with `gofmt -l` only on files you actually edited, and only diff-review actual code changes.
- **Test labels:** every new test is labeled RED (written to fail against current code, then made to pass) or GUARD (pins existing behaviour) in its comment and the task report. Record **which assertion fired** when you ran it failing. For load-bearing GUARDs, run the deliberate-regression check: temporarily break the guarded behaviour, watch the test fail, restore.
- **Hermetic tests only** in tasks 1–7: `go test ./...` with `GPB_LIVE_ACCOUNT` unset (the live half must loudly skip). The live account is touched ONLY by Task 8's gate runner, confined to `/my-files/GitRemotes/<demo>` and `/my-files/_cas-probe`. `GitBackups`, `Sensitive Project Sources`, `Project Repo Bundles`, `ChatGPT Export Text Backup` are untouchable.
- **A hygiene test that FAILS against current code has found a real defect:** stop, report it, do not silently patch.

---

### Task 1: Fetch-path test strengthens (test-only)

**Files:**
- Modify: `internal/repo/repo_test.go` (three existing tests near lines 2710, 2858, 2800; one new test)
- Modify: `internal/transport/trace_test.go` (one new test)

**Interfaces:**
- Consumes: `transport.NewTraced(inner Transport, w io.Writer) *Traced` (`internal/transport/trace.go:26`); `gitcmd.HasObject(gitDir, sha string) bool` (`internal/gitcmd/gitcmd.go:124`); the existing Fake-driven fetch test fixtures in `repo_test.go`.
- Produces: nothing new — strengthened GUARD tests only. No production code changes in this task.

- [ ] **Step 1: M1 — route-around trace assert in `TestFetchChecksumFailureHealsAndRoutesAround`** (`repo_test.go:2710`)

Read the test. If its `Fetch` call does not already go through `transport.NewTraced`, wrap the fake:

```go
var trace strings.Builder
tt := transport.NewTraced(fake, &trace)
// pass tt (not fake) to Fetch
```

Then add, after the existing success assertions (adapt the two stem identifiers to the test's locals — the *contract* is normative: the checksum-failing pack is downloaded exactly once, and the pack that covers the route-around is downloaded exactly once):

```go
// GUARD (M1, Stage 4): the route-around must be visible in the transfer
// trace — the poisoned pack downloads exactly once (its failed attempt),
// never again after the heal, and the covering pack exactly once.
if got := strings.Count(trace.String(), "/packs/"+badStem+".pack"); got != 1 {
	t.Errorf("poisoned pack downloaded %d times, want exactly 1; trace:\n%s", got, trace.String())
}
if got := strings.Count(trace.String(), "/packs/"+coverStem+".pack"); got != 1 {
	t.Errorf("covering pack downloaded %d times, want exactly 1; trace:\n%s", got, trace.String())
}
```

- [ ] **Step 2: Run it**

```
go test ./internal/repo/ -run 'TestFetchChecksumFailureHealsAndRoutesAround' -count=1 -v
```
Expected: PASS. If it FAILS, the strengthen found a real defect — stop and report per Global Constraints.

- [ ] **Step 3: Deliberate-regression check for M1**

Temporarily make the trace assertion impossible (e.g. change `!= 1` to `!= 99`), run, confirm the test FAILS on the new assertion (record which one), restore. This proves the assert is live.

- [ ] **Step 4: M3 — HasObject in `TestFetchSkipsAnIncompletePairWhenUnneeded`** (`repo_test.go:2858`)

After the test's existing success assertions (adapt `want` to the test's local name for the fetched tip):

```go
// GUARD (M3, Stage 4): exit-0 alone does not prove the closure landed —
// assert the want is actually present in the object store.
if !gitcmd.HasObject(gitDir, want) {
	t.Errorf("fetched closure must contain want %s; a fetch that skips the "+
		"incomplete pair must still deliver everything", want)
}
```

- [ ] **Step 5: New two-pack pair-refresh test** (beside `TestFetchPairFailureRefreshesSidecarAndCompletes`, `repo_test.go:2800`)

The existing test uses ONE pack. Write `TestFetchPairFailureWithTwoPacksRefreshesAndCompletes`: read the existing test first — its locals are the `*transport.Fake` (reported as `f`), the repo path (`dst`), and commit shas (`c1`/`c2`); verify the actual names in the body and use THOSE, not this plan's placeholders. Fixture: TWO pushes so the remote holds two packs both needed by the want, then corrupt the CACHED sidecar of the FIRST stem with **garbage bytes** (so pair-verify fails and the refresh machinery genuinely runs — NOT the existing helper that builds an alternate index with the same OID set, which a peer review showed leaves the plan unchanged and the assertions blind). Wrap the fake explicitly before the `Fetch` call:

```go
var trace strings.Builder
tr := transport.NewTraced(f, &trace)
// pass tr to Fetch
```

Assert (adapt local names as above):

```go
// GUARD (retro-Codex 1, Stage 4): a mid-plan pair-refresh with a second
// pack still in the plan must complete the whole fetch. HONEST SCOPE: this
// pins completion and bounded downloads through a two-pack refresh; it does
// NOT empirically distinguish restart from resume (both complete this
// fixture) — that distinction stays structurally pinned at unit level.
if err != nil {
	t.Fatalf("fetch must survive a mid-plan sidecar refresh with two packs: %v", err)
}
if !gitcmd.HasObject(dst, c2) {
	t.Errorf("want %s missing after two-pack refresh fetch", c2)
}
if got := strings.Count(trace.String(), "/packs/"+stem1+".idx"); got < 1 {
	t.Errorf("the corrupted sidecar was never re-downloaded; trace:\n%s", trace.String())
}
for _, stem := range []string{stem1, stem2} {
	if got := strings.Count(trace.String(), "/packs/"+stem+".pack"); got < 1 || got > 2 {
		t.Errorf("pack %s downloaded %d times, want 1 or 2 (bounded); trace:\n%s",
			stem, got, trace.String())
	}
}
```

Skip the deliberate-regression check for this test — a resume would also pass it, and claiming otherwise would be a false pin (record that honestly in the task report).

> **SUPERSEDED (Stage 4 execution, Task 1 review, 2026-08-05):** the garbage-byte corruption
> this step prescribes is caught by `buildPackMap`'s pre-loop `ShowIndex` cache-miss check
> (`packmap.go:98-101`), which heals the sidecar BEFORE any round is planned — the mid-loop
> pair-refresh (`downloadAndVerifyPack`, `fetch.go:244-268`) and the `if refreshed { break }`
> round-restart (`fetch.go:160-166`) are never reached by this fixture, so the prescribed
> GUARD comment's "mid-plan pair-refresh" framing overclaimed what the test pins, and
> retro-Codex 1's motivating scenario (a second pack still pending across a mid-round
> refresh) stayed untested. As executed: (a) the garbage-byte test STANDS with its comment
> corrected to the pre-loop heal it actually exercises — still novel coverage: the first
> end-to-end two-pack proof that a corrupt cached sidecar yields a complete, bounded fetch;
> (b) a SECOND test (`TestFetchMidRoundPairRefreshWithTwoPacksCompletes`) corrupts the first
> stem's cached sidecar with the `altPackingIdx`-style same-OID reindex instead — the only
> known mechanism that passes `ShowIndex` pre-loop yet fails pack-pair verification mid-round
> (the existing single-pack test proves it reaches the mid-loop path). The earlier
> peer-review objection to `altPackingIdx` (same OID set → rebuilt plan unchanged →
> restart-vs-resume indistinguishable) is conceded in that test's HONEST SCOPE comment; what
> it pins is completion and bounded downloads through a genuine mid-round refresh with a
> second pack still in the plan — exactly retro-Codex 1's ask. The implementer must verify
> the mid-loop path actually fires (the refresh trace line appears / the `.idx` re-download
> shows in the transfer trace).

- [ ] **Step 6: Trace size-unknown branch test** (`internal/transport/trace_test.go`, `package transport`)

```go
// readToWithoutLanding reports ReadTo success without creating the local
// file, which is the only way to reach Traced's size-unknown branch.
type readToWithoutLanding struct{ Transport }

func (readToWithoutLanding) ReadTo(p, local string) error { return nil }

// GUARD (Stage 4): the size-unknown fallback must still emit the normative
// "gpb: downloaded" prefix the gate greps for.
func TestTracedReportsSizeUnknownWhenTheFileDidNotLand(t *testing.T) {
	var buf strings.Builder
	tr := NewTraced(readToWithoutLanding{}, &buf)
	if err := tr.ReadTo("/r/packs/pack-x.idx", t.TempDir()); err != nil {
		t.Fatalf("stub ReadTo must succeed: %v", err)
	}
	want := "gpb: downloaded /r/packs/pack-x.idx (size unknown)\n"
	if buf.String() != want {
		t.Errorf("trace line %q, want %q", buf.String(), want)
	}
}
```

- [ ] **Step 7: Run the full hermetic suite**

```
go test ./... -count=1
```
Expected: PASS everywhere (live halves loudly skip).

- [ ] **Step 8: Commit**

```bash
git add internal/repo/repo_test.go internal/transport/trace_test.go
git commit -m "test(v2): stage 4 hygiene wave - M1 trace assert, M3 HasObject, two-pack pair-refresh, trace size-unknown

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 2: Fetch-path code hygiene

**Files:**
- Modify: `internal/gitcmd/gitcmd.go:542-577` (`ShowIndex`)
- Test: `internal/gitcmd/gitcmd_test.go`
- Modify: `internal/repo/idxcache.go:33-44` (`ResolveIdxCacheDir`), `:159-164` (`pruneStale` comment)
- Modify: `internal/repo/fetch.go:271-286` (`copyFile`), `internal/repo/push.go:604-632` (`linkOrCopy`)
- Modify: `docs/superpowers/specs/2026-08-03-v2-stage3b-selective-fetch-design.md` (one reconciliation bullet)

**Interfaces:**
- Consumes: existing signatures, all unchanged: `ShowIndex(idxPath string) ([]string, error)`, `ResolveIdxCacheDir(gitDir, root string) (string, error)`, `copyFile(src, dst string) error`, `linkOrCopy(src, dst string) error`.
- Produces: one new unexported function `parseShowIndexOutput(idxPath, out string) ([]string, error)` in `internal/gitcmd` (used only by `ShowIndex` and its tests). No exported surface changes.

- [ ] **Step 1: `ShowIndex` — wrap the open error, extract the parser**

In `gitcmd.go`, change `ShowIndex`'s open (line 543) to:

```go
	f, err := os.Open(idxPath)
	if err != nil {
		return nil, fmt.Errorf("show-index %s: %w", idxPath, err)
	}
```

Extract the parsing loop (lines 565–575) into a function so the garbled-line arm is testable without forcing real git to emit garbage:

```go
// parseShowIndexOutput turns show-index stdout into the OID list. Any line
// that is not "<offset> <oid> ..." is a hard error — a truncated or garbled
// answer must never become a smaller map.
func parseShowIndexOutput(idxPath, out string) ([]string, error) {
	var oids []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || !oidRe.MatchString(fields[1]) {
			return nil, fmt.Errorf("show-index %s: unparseable line %q", idxPath, line)
		}
		oids = append(oids, fields[1])
	}
	return oids, nil
}
```

`ShowIndex` ends with `return parseShowIndexOutput(idxPath, stdout.String())`.

- [ ] **Step 2: Tests for both new arms** (`gitcmd_test.go`)

```go
// GUARD (Stage 4): the open failure must name show-index and the path.
func TestShowIndexWrapsTheOpenError(t *testing.T) {
	_, err := ShowIndex(filepath.Join(t.TempDir(), "absent.idx"))
	if err == nil || !strings.Contains(err.Error(), "show-index") {
		t.Errorf("want a show-index-prefixed error for a missing idx, got %v", err)
	}
}

// RED then GUARD (Stage 4): the garbled-line arm was never exercised before.
func TestParseShowIndexOutputRefusesGarbledLines(t *testing.T) {
	if _, err := parseShowIndexOutput("x.idx", "12 0123456789012345678901234567890123456789\nnot a line"); err == nil {
		t.Error("a garbled line must be a hard error, not a smaller map")
	}
	oids, err := parseShowIndexOutput("x.idx", "12 0123456789012345678901234567890123456789 (crc)")
	if err != nil || len(oids) != 1 {
		t.Errorf("a valid v2 line must parse: oids=%v err=%v", oids, err)
	}
}
```

Run: `go test ./internal/gitcmd/ -count=1 -v` → PASS (record assertions; the refactor must not change `ShowIndex` behaviour — existing tests stay green).

- [ ] **Step 3: `ResolveIdxCacheDir` — stop discarding the diagnostic in the err branch**

The `err != nil` branch (idxcache.go:35-37) discards `out`, which holds git's merged stdout+stderr — the only diagnostic when the process started but died. Change to:

```go
	if err != nil {
		return "", fmt.Errorf("cannot resolve the git common dir for %s: %s: %w",
			gitDir, strings.TrimSpace(out), err)
	}
```

(The `code != 0` branch already includes `out`; no other ordering change.)

- [ ] **Step 4: `pruneStale` — document the divergence**

Above `func pruneStale` (idxcache.go:164), extend the comment:

```go
// Staleness is keyed on the COMPLETE-pair stems the caller kept, which is
// deliberately narrower than the design's name-in-listing rule: a cached
// .idx whose pack is currently incomplete (mid-push) is pruned here and
// simply re-downloaded if the pair later completes. Recorded as a
// post-implementation reconciliation in the Stage 3b spec.
```

- [ ] **Step 5: Dedup `copyFile`/`linkOrCopy`**

Both are in package `repo`. Replace `linkOrCopy`'s hand-rolled fallback (push.go:610-631) with:

```go
func linkOrCopy(src, dst string) error {
	if err := os.Link(src, dst); err == nil {
		return nil
	}
	return copyFile(src, dst)
}
```

Move `copyFile`'s close-once discipline comment (currently on linkOrCopy's fallback) onto `copyFile` in fetch.go so the rationale survives the dedup — `copyFile` becomes the single owner of the copy logic:

```go
// copyFile copies src to dst. out is closed EXACTLY ONCE on each path, with
// no `defer out.Close()` alongside: a deferred second Close fired after the
// explicit one, got os.ErrClosed, and discarded it — harmless until someone
// adds error handling to the defer, at which point every successful copy
// starts reporting "file already closed". Close's error is worth returning:
// on a filesystem that defers write errors it is the only place a failed
// flush surfaces at all.
```

Run: `go test ./internal/repo/ -count=1` → PASS.

- [ ] **Step 6: 3b-spec reconciliation bullet**

Append to the **Post-implementation reconciliation** list in `docs/superpowers/specs/2026-08-03-v2-stage3b-selective-fetch-design.md`:

```markdown
- **`listCompletePacks` stderr-notes only packish-looking skips.** The Testing section's letter
  says every skipped node gets a note; the shipped code notes only names ending `.pack`/`.idx`
  that fail the grammar, plus the pack-without-index case. Kept as shipped (Stage 4 decision):
  non-packish nodes in `packs/` are foreign junk the helper deliberately ignores, and per-node
  notes would make stderr volume depend on outside actors.
```

- [ ] **Step 7: Full suite, gofmt on touched files, commit**

```
go test ./... -count=1
gofmt -l internal/gitcmd/gitcmd.go internal/gitcmd/gitcmd_test.go internal/repo/idxcache.go internal/repo/fetch.go internal/repo/push.go
```
Expected: tests PASS, gofmt lists nothing (CRLF caveat in Global Constraints).

```bash
git add internal/gitcmd/ internal/repo/ docs/superpowers/specs/2026-08-03-v2-stage3b-selective-fetch-design.md
git commit -m "refactor(v2): stage 4 hygiene - ShowIndex wrap+parser, cache-dir diagnostic, copy dedup, pruneStale comment, 3b spec bullet

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 3: CLI version allowlist — advisory becomes enforcing

**Files:**
- Modify: `internal/transport/cli.go` (beside `CertifiedCLI`/`IsCertified`/`Version`, lines 391–417)
- Test: `internal/transport/cli_test.go`
- Modify: `cmd/git-remote-proton/main.go:68-79` (replace the advisory block)
- Test: `cmd/git-remote-proton/main_test.go`

**Interfaces:**
- Consumes: `(c *CLI) Version() (string, error)` — already fails on nonzero exit; this task CHANGES it to return the full trimmed output (both lines) instead of the first line only, so diagnostics can carry the SDK line (spec: recorded, not matched). Its only caller is the advisory block this task replaces, plus its own tests.
- Produces:
  - `transport.EnforceCertified(c *CLI, allowUncertified bool, w io.Writer) error` — nil means proceed (certified, or uncertified-with-override after writing the warning to `w`); non-nil is the refusal, naming found vs certified and the override. Task 5's `--set-head` path calls this exact function.
  - `IsCertified(versionOutput string) bool` — REWRITTEN to exact-token matching: it extracts the first whitespace-delimited field starting `cli-drive@` and compares it for **equality** against `CertifiedCLI`. The current `strings.Contains` is a peer-review blocker: it accepts `cli-drive@0.7.0+5174900c-extra` and any text embedding the certified string, which is a prefix match by another name — exactly what the spec's "exact versions, not a floor or prefix" forbids. No token at all → not certified (the unparseable case).
- **Seam decision (spec §1 "fires at transport construction"):** enforcement is an explicit `EnforceCertified` call immediately after each `NewCLI` site (there are exactly two after Task 5: `run()` and `runSetHead`), NOT embedded inside `NewCLI`. Embedding would make every existing test that constructs a CLI with a fake exe spawn a version check, breaking the whole cli_test machinery for zero safety gain. Recorded so a reviewer doesn't re-litigate it.

- [ ] **Step 1: Write the failing tests** (`cli_test.go`, using the existing fake-exe TestMain role machinery — see `helperVersionLine` at line 45 and the role pattern at lines 58–63; add roles as needed for a wrong-version line and a nonzero exit)

```go
// RED (Stage 4): EnforceCertified does not exist. The advisory warn in
// cmd/main.go becomes this enforcing check; the design's "refuse to run"
// rule finally matches the code.
func TestEnforceCertifiedAcceptsTheCertifiedBuild(t *testing.T) {
	// role serving "Proton Drive CLI " + CertifiedCLI on --version
	var buf strings.Builder
	if err := EnforceCertified(certifiedRoleCLI(t), false, &buf); err != nil {
		t.Fatalf("the certified build must pass: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("no warning on the certified path, got %q", buf.String())
	}
}

func TestEnforceCertifiedRefusesAMismatchNamingBothSides(t *testing.T) {
	err := EnforceCertified(wrongVersionRoleCLI(t), false, io.Discard)
	if err == nil {
		t.Fatal("an uncertified version must refuse")
	}
	for _, want := range []string{"cli-drive@9.9.9", CertifiedCLI, "GPB_UNCERTIFIED_CLI"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal must name %q, got %q", want, err.Error())
		}
	}
}

func TestEnforceCertifiedOverrideProceedsWithALoudWarning(t *testing.T) {
	var buf strings.Builder
	if err := EnforceCertified(wrongVersionRoleCLI(t), true, &buf); err != nil {
		t.Fatalf("override must proceed: %v", err)
	}
	for _, want := range []string{"UNCERTIFIED", "cli-drive@9.9.9", CertifiedCLI} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("warning must name %q, got %q", want, buf.String())
		}
	}
}

func TestEnforceCertifiedTreatsAFailedVersionAsUndetermined(t *testing.T) {
	// nonzero-exit role: refusal without override, "could not be determined"
	// warning + proceed with it.
	if err := EnforceCertified(nonzeroVersionRoleCLI(t), false, io.Discard); err == nil {
		t.Error("a failed --version must refuse without the override")
	}
	var buf strings.Builder
	if err := EnforceCertified(nonzeroVersionRoleCLI(t), true, &buf); err != nil {
		t.Errorf("override must proceed on an undetermined version: %v", err)
	}
	if !strings.Contains(buf.String(), "could not be determined") {
		t.Errorf("warning must say the version could not be determined, got %q", buf.String())
	}
}

func TestEnforceCertifiedSurfacesAMissingBinary(t *testing.T) {
	// Spec rows: a missing binary is "not an allowlist path" and "the
	// override does not synthesize a binary" — so a binary that never
	// STARTED refuses even WITH the override, carrying the spawn failure.
	err := EnforceCertified(NewCLI("nonexistent-xyz-binary-gpb-test"), false, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "nonexistent-xyz-binary-gpb-test") {
		t.Errorf("refusal must surface the spawn failure, got %v", err)
	}
	if err := EnforceCertified(NewCLI("nonexistent-xyz-binary-gpb-test"), true, io.Discard); err == nil {
		t.Error("the override must not synthesize a missing binary")
	}
}

func TestIsCertifiedIsExactTokenNotContainment(t *testing.T) {
	// RED against the current strings.Contains implementation — the
	// containment cases below PASS it today and must not survive.
	for _, bad := range []string{
		"Proton Drive CLI cli-drive@0.7.0+5174900c-extra",
		"embedded " + CertifiedCLI + "-suffix",
		"no recognisable token here",
		"",
	} {
		if IsCertified(bad) {
			t.Errorf("%q must not be certified under exact-token matching", bad)
		}
	}
	// The real two-line captured output stays certified (SDK line ignored).
	if !IsCertified("Proton Drive CLI " + CertifiedCLI + "\nProton Drive SDK js@0.20.0+5174900c") {
		t.Error("the genuine two-line --version output must be certified")
	}
}

func TestEnforceCertifiedRefusesUnparseableOutput(t *testing.T) {
	// Output with no cli-drive@ token: refusal without override, proceed
	// with it (spec's unparseable rows).
	if err := EnforceCertified(noTokenRoleCLI(t), false, io.Discard); err == nil {
		t.Error("output with no cli-drive@ token must refuse")
	}
	var buf strings.Builder
	if err := EnforceCertified(noTokenRoleCLI(t), true, &buf); err != nil {
		t.Errorf("override must proceed on unparseable output: %v", err)
	}
}
```

(`certifiedRoleCLI`/`wrongVersionRoleCLI`/`nonzeroVersionRoleCLI`/`noTokenRoleCLI` are helpers you write against the existing TestMain role machinery. The certified role must serve the genuine captured TWO-line output — `"Proton Drive CLI " + CertifiedCLI + "\nProton Drive SDK js@0.20.0+5174900c"` — so the certified-pass case exercises the real parse (spec, Testing). Use `cli-drive@9.9.9+deadbeef` for the wrong-version role so the assertions above hold verbatim. The existing `TestIsCertifiedMatchesTheRealVersionLine` (cli_test.go:613) keeps passing under exact-token matching — its positive cases are whole fields, its negatives lack the token — but update its doc comment, which currently explains the containment rationale.)

- [ ] **Step 2: Run to verify they fail**

```
go test ./internal/transport/ -run 'TestEnforceCertified' -count=1 -v
```
Expected: FAIL — `EnforceCertified` undefined. Record it.

- [ ] **Step 3: Implement** (in `cli.go`; rewrite `IsCertified`, adjust `Version`, add `EnforceCertified`)

`Version` (cli.go:407): return the full trimmed output instead of cutting to the first line — the SDK line belongs in diagnostics (spec: recorded, not matched). Update its doc comment; its start-failure test is unaffected.

`IsCertified` rewrite (replace the containment body and its doc):

```go
// IsCertified reports whether --version output names EXACTLY the certified
// build. Exact-token, not containment: the first whitespace-delimited field
// starting "cli-drive@" must EQUAL CertifiedCLI. Containment was a prefix
// match by another name — it accepted "cli-drive@0.7.0+5174900c-extra" —
// and "exact versions, not a floor or prefix" is the design's rule. Output
// with no such token is simply not certified (the unparseable case).
func IsCertified(versionOutput string) bool {
	for _, f := range strings.Fields(versionOutput) {
		if strings.HasPrefix(f, "cli-drive@") {
			return f == CertifiedCLI
		}
	}
	return false
}
```

`EnforceCertified`:

```go
// EnforceCertified is the Stage 4 allowlist: the design's "refuse to run"
// rule, enforced. nil means proceed — either the CLI reports the certified
// build, or allowUncertified is set and the loud warning was written to w.
// A binary that never STARTED refuses regardless of the override (the
// override does not synthesize a binary; the spawn failure is the report).
// The check is a compatibility gate against accidental drift, not a
// provenance check: a spoofed --version defeats it trivially, and a helper
// that trusts the CLI with every byte of repo data has no defence against a
// malicious binary anyway (spec, Decisions).
func EnforceCertified(c *CLI, allowUncertified bool, w io.Writer) error {
	v, verr := c.Version()
	if verr == nil && IsCertified(v) {
		return nil
	}
	if verr != nil && errors.Is(verr, exec.ErrNotFound) {
		return fmt.Errorf("Proton CLI not found: %w", verr)
	}
	// Quoted diagnostics are BOUNDED: --version output is remote-tool
	// output, and an error message must not become a channel for megabytes.
	found := fmt.Sprintf("%q", bound(v, 200))
	if verr != nil {
		found = fmt.Sprintf("could not be determined (%v)", verr)
	}
	if allowUncertified {
		fmt.Fprintf(w, "git-remote-proton: WARNING: proceeding with an UNCERTIFIED "+
			"Proton CLI because GPB_UNCERTIFIED_CLI=1. Version %s; certified: %s. "+
			"Behaviour on this build is unvalidated.\n", found, CertifiedCLI)
		return nil
	}
	return fmt.Errorf("Proton CLI version %s, but this build is certified only "+
		"against %s; refusing to run. Set GPB_UNCERTIFIED_CLI=1 to proceed anyway "+
		"(unvalidated), or install the certified CLI", found, CertifiedCLI)
}

// bound truncates s for quoting in diagnostics.
func bound(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…(truncated)"
}
```

(`errors` and `os/exec` are already imported by cli.go; verify, add if not. Version's underlying read shares `run()`'s CombinedOutput + WaitDelay machinery with every other command — the spec's "bounds how much output it will read" is implemented at the diagnostic-quoting level above; note this narrowing in the task report.)

> **SUPERSEDED (Stage 4 execution, Task 3 review, 2026-08-05):** the `errors.Is(verr,
> exec.ErrNotFound)` check above under-implements the spec's "the override does not synthesize
> a binary" row. `run()` returns code −1 with the RAW start error for the whole never-started
> class (not on PATH, permission denied, bad executable format, `CLI.Exe` pointing at a
> directory), but only the not-on-PATH case is `exec.ErrNotFound` — every other spawn failure
> fell to the generic branch and PROCEEDED under `GPB_UNCERTIFIED_CLI=1`, contradicting both
> the doc comment ("a binary that never STARTED refuses regardless of the override") and the
> spec. As executed: never-started is detected as `verr != nil && !errors.As(verr,
> new(*exec.ExitError))` — `Version()`'s errors partition cleanly into ran-and-exited-nonzero
> (`*exec.ExitError`, version undetermined, override applies) and never-started (raw start
> error: `*exec.Error` or `*os.PathError`, override inapplicable), and the WaitDelay case
> never errors (`TestVersionSucceedsThroughAWaitDelay` pins success). A regression test
> covers a non-ErrNotFound spawn failure with the override set. Additionally, the brief's
> test list had no assertion pinning `Version()`'s full-output change (reverting to
> first-line-only left the suite green); a pin asserting the SDK line survives in the
> certified role's `Version()` output was added.

- [ ] **Step 4: Run to verify they pass**

```
go test ./internal/transport/ -run 'TestEnforceCertified|TestIsCertified|TestVersion' -count=1 -v
```
Expected: PASS.

- [ ] **Step 5: Wire into `main.go`**

Replace the advisory block (main.go lines 69–79, the `// Advisory only…` comment and the `if v, err := cli.Version(); …` chain) with:

```go
	// The Stage 4 allowlist: refuse-by-default against the certified build,
	// with GPB_UNCERTIFIED_CLI=1 as the explicit, loud escape hatch. This
	// closed the design/code contradiction open since Stage 2 — the advisory
	// warn that used to live here is now transport.EnforceCertified.
	if err := transport.EnforceCertified(cli,
		os.Getenv("GPB_UNCERTIFIED_CLI") == "1", os.Stderr); err != nil {
		warn(err)
		return 1
	}
```

- [ ] **Step 6: Full suite + build, commit**

```
go build ./... ; go vet ./... ; go test ./... -count=1
```
Expected: all clean. The `loop()` protocol tests must be untouched (enforcement is pre-loop).

```bash
git add internal/transport/cli.go internal/transport/cli_test.go cmd/git-remote-proton/main.go
git commit -m "feat(v2): enforce the certified-CLI allowlist with GPB_UNCERTIFIED_CLI override

RED: TestEnforceCertified* (5 tests). The advisory warn becomes the
design's refuse-to-run rule; parseNodeJSON tolerance stays as defense
in depth behind the front door.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 4: `repo.UpdateHEAD` + `repo.SetHead`

**Files:**
- Modify: `internal/repo/head.go` (add `UpdateHEAD`; `WriteHEAD` is NOT touched)
- Create: `internal/repo/sethead.go`
- Test: `internal/repo/repo_test.go` (mirror the `TestWriteHEAD*` patterns at lines 1336–1400 for outcome-arm machinery)

**Interfaces:**
- Consumes: `RequireMarker(t, root) error`; `AcquireLock(t, root) (*Lock, error)` / `(*Lock).Release() error`; `ListRefs(t, root) (map[string]string, error)` (keys are full ref names, e.g. `refs/heads/main`); `ReadHEAD(t, root) (string, bool, error)`; `stagedFile`, `checkStageableLeaf`, `headPrefix`, `HeadName` (all already in package `repo`); `transport.Transport` incl. `UpdateRevision(path, localPath string) (Outcome, error)`.
- Produces:
  - `UpdateHEAD(t transport.Transport, root, branch string) (transport.Outcome, error)` — overwrite-capable HEAD write, caller must hold the lock. Returns `(Committed, nil)` or `(Ambiguous, err)`.
  - `SetHead(t transport.Transport, root, branchArg string) (string, error)` — the whole `--set-head` operation minus argv/printing; returns the normalized full ref name on success. Task 5 calls exactly this.

- [ ] **Step 1: Write the failing tests** (GUARD/RED labels as marked; use the Fake plus whatever outcome-forcing hooks `TestWriteHEADAmbiguousOutcomeIsReported` (line 1381) already uses)

Cover, at minimum — each test comment carries its label:

```go
// RED: UpdateHEAD does not exist.
// 1  UpdateHEAD overwrites an existing HEAD and verifies by read-back.
// 2  UpdateHEAD creates HEAD when absent (the headless-remote rescue).
// 3  UpdateHEAD refuses a non-branch target (mirrors WriteHEAD's rule).
// 4  UpdateHEAD reports Ambiguous outcomes as re-run-to-reconcile errors —
//    on the UPDATE path: the existing outcome-forcing decorators override
//    CreateExclusive only (repo_test.go:483, :1205), so write a sibling
//    decorator overriding UpdateRevision for these arms.
// 5  UpdateHEAD fails closed on an unrecognised Outcome on BOTH paths
//    (mirror TestWriteHEADFailsClosedOnUnrecognisedOutcome at line 1336;
//    update path needs the new decorator from 4).
// 6  GUARD: WriteHEAD STILL never overwrites (re-run the existing
//    TestWriteHEADNeverOverwritesExistingHEAD; it must stay green).

// RED: SetHead does not exist.
// 7  SetHead succeeds: two branches, HEAD at A, SetHead("b") → HEAD reads
//    back refs/heads/b; returns "refs/heads/b".
// 8  Same-target idempotence: SetHead to the branch HEAD already names
//    succeeds WITHOUT uploading — wrap the Fake in a small COUNTING
//    decorator (embed transport.Transport, override UpdateRevision and
//    CreateExclusive to increment then delegate) and assert zero writes.
//    transport.NewTraced is NOT usable here: it logs only ReadTo, so it
//    reports zero writes even when writes happen (peer-review finding).
// 9  Dangling HEAD refuses: HEAD names refs/heads/gone, branch "gone" does
//    not exist, SetHead("gone") → error naming the branches that DO exist
//    (round-3 peer-review finding — verification runs BEFORE the
//    short-circuit).
// 10 Unknown branch refuses, error names existing branches.
// 11 Empty repo (marker present, no branches) refuses with "push a branch
//    first".
// 12 Hierarchical name ("feature/x") refuses naming Stage 5.
// 13 Tag target ("refs/tags/v1") refuses: HEAD points at branches only.
// 14 No marker → RequireMarker's refusal, verbatim.
// 15 Lock held → AcquireLock's refusal; and SetHead released ITS lock on
//    every refusal path above (assert the Fake holds no .lock after each).
// 16 Short-name normalization: SetHead("main") == SetHead("refs/heads/main").
// 17 Corrupt or unreadable HEAD refuses (fail closed): make ReadHEAD error
//    (e.g. non-symref HEAD content in the Fake) and assert SetHead returns
//    that error WITHOUT writing. The in-tool repair path for a corrupt HEAD
//    is delete-HEAD-via-web-UI (existing documented remedy), after which
//    the repo is headless and SetHead works via the create path.
```

> **SUPERSEDED (Stage 4 execution, Task 4 review, 2026-08-05):** the 17-scenario matrix above
> omitted the one arm whose behaviour is NOVEL in `UpdateHEAD` — the `Refused` arm that
> refuses rather than adopting (the exact semantic distinguishing it from `WriteHEAD`'s
> adopt-theirs `Refused` arm). The whole matrix stayed green with that arm mutated to
> `WriteHEAD`'s form, i.e. a plausible copy-paste regression silently reporting a foreign
> HEAD as success. As executed, a scenario 18 was added: `UpdateHEAD` on a byte-identical
> rewrite (the Fake's `UpdateRevision` returns `Refused` for it, modelling probe C2) must
> return an error, not success — no new scaffolding needed.

- [ ] **Step 2: Run to verify they fail** (`go test ./internal/repo/ -run 'TestUpdateHEAD|TestSetHead' -count=1 -v`) — expected: FAIL, undefined symbols. Record.

- [ ] **Step 3: Implement `UpdateHEAD`** (in `head.go`, under `WriteHEAD`)

The exists-branch below (`UpdateRevision` when HEAD exists, `CreateExclusive` when absent) is deliberate and peer-review-adjudicated: it is the same branch `WriteRef` takes — "the staged-file CAS path pushes use" — and both primitives are certified by probes. The reviewer-suggested alternative (always `UpdateRevision`, relying on it upserting an absent node) leans on UNCERTIFIED CLI behaviour: C14 proved merge-revises-existing, nothing proved create-on-absent. Do not "simplify" this.

```go
// UpdateHEAD points HEAD at branch, OVERWRITING any existing symref — the
// write half of the --set-head operation, and deliberately not WriteHEAD:
// that function backfills and never overwrites, a contract its tests pin.
// The caller MUST hold the repo lock. Verified by read-back like every
// write (the CLI silently skips byte-identical rewrites).
func UpdateHEAD(t transport.Transport, root, branch string) (transport.Outcome, error) {
	if !strings.HasPrefix(branch, "refs/heads/") {
		return transport.Ambiguous, fmt.Errorf("refusing to point HEAD at %q: not a branch", branch)
	}
	staged, cleanup, err := stagedFile([]byte(headPrefix+branch+"\n"), HeadName)
	if err != nil {
		return transport.Ambiguous, err
	}
	defer cleanup()

	p := root + "/" + HeadName
	_, exists, err := t.Stat(p)
	if err != nil {
		return transport.Ambiguous, fmt.Errorf("stat %s: %w", p, err)
	}
	var out transport.Outcome
	if exists {
		out, err = t.UpdateRevision(p, staged)
	} else {
		out, err = t.CreateExclusive(p, staged)
	}
	if err != nil {
		return transport.Ambiguous, err
	}
	switch out {
	case transport.Committed:
		// Falls through to the read-back below.
	case transport.Refused:
		// Under the lock no v2 writer can race us, so a refusal here is a
		// non-v2 actor. Do not adopt: the user asked for THIS branch.
		return transport.Ambiguous, fmt.Errorf("HEAD write for %s was refused mid-operation; "+
			"re-run to reconcile", p)
	case transport.Ambiguous:
		return transport.Ambiguous, fmt.Errorf("HEAD update outcome ambiguous for %s; "+
			"re-run to reconcile", p)
	default:
		return transport.Ambiguous, fmt.Errorf("HEAD update for %s returned an unrecognised "+
			"outcome %s; refusing to guess whether it landed", p, out)
	}
	got, ok, err := ReadHEAD(t, root)
	if err != nil {
		return transport.Ambiguous, err
	}
	if !ok || got != branch {
		return transport.Ambiguous, fmt.Errorf("HEAD reads back as %q, expected %q", got, branch)
	}
	return transport.Committed, nil
}
```

- [ ] **Step 4: Implement `SetHead`** (new file `internal/repo/sethead.go`)

```go
package repo

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// SetHead is the --set-head operation: point the remote's HEAD at an
// EXISTING branch, under the repo lock. Returns the normalized full ref
// name it set (or confirmed). Order is load-bearing and peer-reviewed:
// branch existence is verified on EVERY run BEFORE the idempotence
// short-circuit, so a HEAD already naming a since-deleted branch refuses
// rather than reporting success (round-3 finding, both engines).
func SetHead(t transport.Transport, root, branchArg string) (string, error) {
	branch, err := normalizeBranch(branchArg)
	if err != nil {
		return "", err
	}
	if err := RequireMarker(t, root); err != nil {
		return "", err
	}
	lock, err := AcquireLock(t, root)
	if err != nil {
		return "", err
	}
	// Release on EVERY exit path; its error is reported, never masking the
	// operation's own result — the same contract cmd's loop defer documents.
	defer func() {
		if rerr := lock.Release(); rerr != nil {
			fmt.Fprintf(os.Stderr, "git-remote-proton: %v\n", rerr)
		}
	}()

	refs, err := ListRefs(t, root)
	if err != nil {
		return "", err
	}
	if _, ok := refs[branch]; !ok {
		var branches []string
		for name := range refs {
			if strings.HasPrefix(name, "refs/heads/") {
				branches = append(branches, strings.TrimPrefix(name, "refs/heads/"))
			}
		}
		if len(branches) == 0 {
			return "", fmt.Errorf("cannot set HEAD to %q: no branches exist; push a branch first", branchArg)
		}
		sort.Strings(branches)
		return "", fmt.Errorf("cannot set HEAD to %q: no such branch; branches that exist: %s",
			branchArg, strings.Join(branches, ", "))
	}

	// Idempotence short-circuit — AFTER the existence check above. A ReadHEAD
	// error is FATAL (fail closed): it covers transient transport trouble as
	// much as corrupt content, and an indeterminate read must never license a
	// destructive overwrite (peer-review finding on this plan). A genuinely
	// corrupt HEAD is repaired by deleting the HEAD file in the web UI —
	// after which the repo is headless and SetHead's create path applies.
	head, hasHead, err := ReadHEAD(t, root)
	if err != nil {
		return "", err
	}
	if hasHead && head == branch {
		return branch, nil
	}

	if _, err := UpdateHEAD(t, root, branch); err != nil {
		return "", err
	}
	return branch, nil
}

// normalizeBranch turns the user's argument into a full refs/heads/ name,
// refusing everything Stage 4 does not support. The hierarchical refusal
// comes FIRST so it gets its own named reason (Stage 5), not the generic
// staging-path one.
func normalizeBranch(arg string) (string, error) {
	b := arg
	if !strings.HasPrefix(b, "refs/") {
		b = "refs/heads/" + b
	}
	if !strings.HasPrefix(b, "refs/heads/") {
		return "", fmt.Errorf("refusing to point HEAD at %q: HEAD points at branches only", arg)
	}
	leaf := strings.TrimPrefix(b, "refs/heads/")
	if strings.ContainsAny(leaf, `/\`) {
		return "", fmt.Errorf("hierarchical ref names are not supported yet (Stage 5): %q", arg)
	}
	if err := checkStageableLeaf(leaf); err != nil {
		return "", err
	}
	return b, nil
}
```

- [ ] **Step 5: Run the matrix** (`go test ./internal/repo/ -run 'TestUpdateHEAD|TestSetHead|TestWriteHEAD' -count=1 -v`) — expected: PASS, including the untouched `TestWriteHEAD*` GUARDs. Record which assertion fired for tests 9 and 15 especially.

- [ ] **Step 6: Deliberate-regression check for the round-3 ordering fix**

Temporarily move the `refs[branch]` existence check BELOW the ReadHEAD short-circuit, run test 9 (dangling HEAD), confirm it FAILS (that is the exact defect both peer-review engines caught), restore. Record.

- [ ] **Step 7: Full suite + commit**

```bash
git add internal/repo/head.go internal/repo/sethead.go internal/repo/repo_test.go
git commit -m "feat(v2): repo.SetHead + overwrite-capable UpdateHEAD

RED: TestUpdateHEAD*/TestSetHead* (16 scenarios). WriteHEAD stays
backfill-only; existence check ordered before the idempotence
short-circuit per round-3 peer review (dangling-HEAD refusal).

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 5: Utility dispatch (`--version`, `--set-head`) + delete-refusal remedy

**Files:**
- Modify: `cmd/git-remote-proton/main.go` (dispatch before the `len(os.Args) < 3` check at line 25; new `version` var; new `runSetHead`)
- Test: `cmd/git-remote-proton/main_test.go`
- Modify: `internal/repo/push.go:167-168` (remedy text)
- Test: `internal/repo/repo_test.go` (the delete-refusal GUARD near line 1500)

**Interfaces:**
- Consumes: `transport.EnforceCertified` (Task 3, exact signature there); `repo.SetHead(t, root, branchArg) (string, error)` (Task 4); `repo.CanonicalRoot(addr) (string, error)`; `transport.NewCLI("")`, `transport.NewTraced(cli, os.Stderr)`, `transport.CertifiedCLI`.
- Produces: `var version = "dev"` in package main (Task 6's ldflags sets `main.version`); the argv contract: `git-remote-proton --version` and `git-remote-proton --set-head <address> <branch>` — a CLOSED set; any other argv shape takes the protocol path unchanged.

- [ ] **Step 1: Write the failing tests** (`main_test.go`)

```go
// RED (Stage 4): utility dispatch does not exist. The set is CLOSED —
// only these two strings dispatch; anything else (including other
// --prefixed argv, e.g. a remote actually named --upstream) must reach
// the protocol path untouched (round-1 Gemini finding).
// 1  dispatchUtility([]string{"git-remote-proton", "--version"}) handles it:
//    stdout gets "git-remote-proton dev (certified CLI: <CertifiedCLI>)".
// 2  dispatchUtility(...--set-head with 0 or 1 following args) handles it:
//    usage on stderr, exit code 1, and NO CLI construction (no version
//    check output).
// 3  dispatchUtility(...--upstream <url>) does NOT handle it (protocol path).
// 4  dispatchUtility with no args at all does not handle (existing
//    "must be run by git" error still fires from run()).
```

Structure main.go so this is testable: extract a `dispatchUtility(args []string, stdout, stderr io.Writer) (handled bool, code int)` that `run()` calls first. The `--set-head` arm with CORRECT arity calls `runSetHead` (which constructs the CLI); arity errors return before any construction.

- [ ] **Step 2: Run to verify they fail** — `go test ./cmd/... -count=1 -v` → FAIL, undefined. Record.

- [ ] **Step 3: Implement**

In `main.go`:

```go
// version is stamped by the release build via
//   -ldflags "-X main.version=<tag>"
// and stays "dev" for a plain `go build`.
var version = "dev"

// dispatchUtility handles the CLOSED set of direct-invocation modes. Only
// exact matches dispatch: a prefix match would misroute a remote whose
// configured name begins with "--" (git passes the remote NAME as argv[1]),
// so only remotes literally named --version or --set-head can collide, and
// those two strings are documented as reserved. Utility stdout is permitted:
// git never invokes the helper with these argv shapes, so it cannot
// interleave with the protocol stream.
func dispatchUtility(args []string, stdout, stderr io.Writer) (bool, int) {
	if len(args) < 2 {
		return false, 0
	}
	switch args[1] {
	case "--version":
		fmt.Fprintf(stdout, "git-remote-proton %s (certified CLI: %s)\n",
			version, transport.CertifiedCLI)
		return true, 0
	case "--set-head":
		if len(args) != 4 {
			fmt.Fprintln(stderr, "usage: git-remote-proton --set-head <proton::address> <branch>")
			return true, 1
		}
		return true, runSetHead(args[2], args[3], stdout, stderr)
	}
	return false, 0
}

func runSetHead(addr, branch string, stdout, stderr io.Writer) int {
	root, err := repo.CanonicalRoot(addr)
	if err != nil {
		fmt.Fprintf(stderr, "git-remote-proton: %v\n", err)
		return 1
	}
	cli := transport.NewCLI("")
	if err := transport.EnforceCertified(cli,
		os.Getenv("GPB_UNCERTIFIED_CLI") == "1", stderr); err != nil {
		fmt.Fprintf(stderr, "git-remote-proton: %v\n", err)
		return 1
	}
	set, err := repo.SetHead(transport.NewTraced(cli, stderr), root, branch)
	if err != nil {
		fmt.Fprintf(stderr, "git-remote-proton: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "HEAD is now %s\n", set)
	return 0
}
```

And at the top of `run()` (BEFORE the `len(os.Args) < 3` check):

```go
	if handled, code := dispatchUtility(os.Args, os.Stdout, os.Stderr); handled {
		return code
	}
```

- [ ] **Step 4: Remedy text in `push.go`**

Change lines 167–168 to:

```go
			return fail(fmt.Sprintf("refusing to delete the branch HEAD points at (%s); "+
				"change the default branch first (git-remote-proton --set-head <url> <branch>)", u.Dst))
```

Update the existing delete-refusal GUARD (repo_test.go near line 1527 asserts `strings.Contains(res[0].Err, "HEAD points at")`) to ALSO assert the remedy:

```go
	if !strings.Contains(res[0].Err, "--set-head") {
		t.Errorf("the refusal must name the in-tool remedy, got %q", res[0].Err)
	}
```

- [ ] **Step 5: Run everything, verify pass, record assertions**

```
go build ./... ; go vet ./... ; go test ./... -count=1
```

- [ ] **Step 6: Commit**

```bash
git add cmd/git-remote-proton/ internal/repo/push.go internal/repo/repo_test.go
git commit -m "feat(v2): --set-head and --version utility modes; delete refusal names the remedy

RED: dispatch tests (closed argv set, arity, protocol path untouched).
Utility stdout is exempt from protocol-only by argv disjointness.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 6: Release pipeline — workflow, installer, CHANGELOG

**Files:**
- Create: `.github/workflows/release.yml`
- Modify: `install.ps1` (full rewrite below; current content is 9 lines, module-only)
- Test: `tests/GitProtonBackup.Tests.ps1` (two Pester tests for the helper block)
- Modify: `CHANGELOG.md` (new `## 0.3.0 — unreleased` section at top)

**Interfaces:**
- Consumes: `main.version` (Task 5) via ldflags; the repo's existing CI conventions (`.github/workflows/ci.yml`: windows-latest, pwsh steps, PSScriptAnalyzer over all `.ps1`, Pester 5.5 in `tests/`).
- Produces: draft-release assets named exactly `git-remote-proton.exe`, `git-remote-proton.exe.sha256`, `install.ps1` (Task 8's gate downloads these three; the `.sha256` format is `<lowercase-hex-sha256>  git-remote-proton.exe`, two spaces, no trailing newline).

- [ ] **Step 1: `release.yml`**

```yaml
name: Release
on:
  push:
    tags: ['v*']
  workflow_dispatch:
# Least-privilege, declared explicitly: draft-release creation must not
# depend on the repository's default token permissions (spec, round-2).
permissions:
  contents: write
jobs:
  release:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v5
        with:
          # Release builds pin the toolchain to go.mod for reproducibility —
          # deliberately unlike ci.yml's 'stable' (which proves newest-compat).
          go-version-file: go.mod
      - name: Test (hermetic; the live half loudly skips in CI)
        shell: pwsh
        run: go vet ./... && go test ./...
      - name: Lint installer (it ships as a release asset)
        shell: pwsh
        run: |
          Install-Module PSScriptAnalyzer -Force -Scope CurrentUser
          $findings = Invoke-ScriptAnalyzer -Path . -Recurse -Settings .\PSScriptAnalyzerSettings.psd1
          if ($findings) { $findings | Format-Table | Out-String | Write-Host; throw "PSScriptAnalyzer: $($findings.Count) finding(s)" }
      - name: Pester (installer tests included)
        shell: pwsh
        run: |
          Install-Module Pester -MinimumVersion 5.5 -Force -SkipPublisherCheck -Scope CurrentUser -AllowClobber
          Import-Module Pester -MinimumVersion 5.5
          git config --global user.email 'ci@ci'; git config --global user.name 'ci'
          Invoke-Pester -Path tests -CI
      - name: Version
        id: ver
        shell: pwsh
        run: |
          if ($env:GITHUB_REF -like 'refs/tags/*') { $v = $env:GITHUB_REF -replace '^refs/tags/','' }
          else { $v = "dev+$($env:GITHUB_SHA.Substring(0,7))" }
          "version=$v" >> $env:GITHUB_OUTPUT
      - name: Build
        shell: pwsh
        env:
          VERSION: ${{ steps.ver.outputs.version }}
        run: go build -trimpath -ldflags "-X main.version=$env:VERSION" -o git-remote-proton.exe ./cmd/git-remote-proton
      - name: Checksum
        shell: pwsh
        run: |
          $hash = (Get-FileHash git-remote-proton.exe -Algorithm SHA256).Hash.ToLower()
          Set-Content git-remote-proton.exe.sha256 -Value "$hash  git-remote-proton.exe" -NoNewline
      - name: Upload build artifact (dispatch dry run only — never a release)
        if: github.event_name == 'workflow_dispatch'
        uses: actions/upload-artifact@v7
        with:
          name: git-remote-proton-dev
          path: git-remote-proton.exe*
      - name: Create DRAFT release (tag runs only)
        if: startsWith(github.ref, 'refs/tags/v')
        shell: pwsh
        env:
          GH_TOKEN: ${{ github.token }}
        run: gh release create $env:GITHUB_REF_NAME --draft --title $env:GITHUB_REF_NAME --notes "See CHANGELOG.md. Draft until the Stage 4 live gate passes against these exact bytes." git-remote-proton.exe git-remote-proton.exe.sha256 install.ps1
```

- [ ] **Step 2: Rewrite `install.ps1`**

Helper block FIRST so the module's already-installed throw can never block a helper-only or helper-upgrade run (spec, round-3). Keep PSScriptAnalyzer clean (`.\PSScriptAnalyzerSettings.psd1` runs over every `.ps1` in CI).

```powershell
<# Installs the GitProtonBackup module (v1) and, when an exe is present, the
   git-remote-proton helper (v2). Helper first: the module's already-installed
   throw must never block a helper-only run. #>
[CmdletBinding()]
param(
    [switch]$Force,
    [string]$HelperExe = (Join-Path $PSScriptRoot 'git-remote-proton.exe')
)
$ErrorActionPreference = 'Stop'

# ---- helper (v2) ----
if (Test-Path $HelperExe) {
    $sidecar = "$HelperExe.sha256"
    if (Test-Path $sidecar) {
        $want = ((Get-Content $sidecar -Raw) -split '\s+')[0].Trim().ToLower()
        $got  = (Get-FileHash $HelperExe -Algorithm SHA256).Hash.ToLower()
        if ($want -ne $got) {
            throw "Checksum mismatch for $HelperExe — expected $want, got $got. Refusing to install the helper."
        }
    }
    $helperDir = Join-Path $env:LOCALAPPDATA 'Programs\git-proton-backup'
    New-Item -ItemType Directory -Force -Path $helperDir | Out-Null
    try {
        Copy-Item $HelperExe (Join-Path $helperDir 'git-remote-proton.exe') -Force
    } catch {
        throw "Cannot replace $helperDir\git-remote-proton.exe (a git process may be using it). Close running git commands and re-run. $_"
    }
    # PATH check tolerates empty user PATHs (no leading semicolon) and
    # trailing-backslash variants (no duplicate entries) — peer-reviewed.
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $entries = @(($userPath -split ';') | Where-Object { $_ } | ForEach-Object { $_.TrimEnd('\') })
    if ($entries -notcontains $helperDir.TrimEnd('\')) {
        $newPath = if ([string]::IsNullOrEmpty($userPath)) { $helperDir } else { "$userPath;$helperDir" }
        [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
        Write-Host "Added $helperDir to your user PATH. Open a NEW terminal for it to take effect (this script cannot change its caller's session)."
    }
    Write-Host "Helper installed: $helperDir\git-remote-proton.exe"
} else {
    Write-Host "No git-remote-proton.exe found at $HelperExe — skipping helper install (module only)."
}

# ---- module (v1), semantics unchanged when the payload is present ----
# A release download is exe + sha256 + this script, with NO module payload
# beside it — that layout must install the helper and SKIP the module, not
# throw (peer-review blocker: the gate's install step is exactly this case).
$payload = Join-Path $PSScriptRoot 'GitProtonBackup'
if (Test-Path $payload) {
    $dest = Join-Path ([Environment]::GetFolderPath('MyDocuments')) 'PowerShell\Modules\GitProtonBackup'
    if ((Test-Path $dest) -and -not $Force) { throw "Already installed at $dest — re-run with -Force to overwrite." }
    New-Item -ItemType Directory -Path $dest -Force | Out-Null
    Copy-Item -Path (Join-Path $payload '*') -Destination $dest -Recurse -Force
    Write-Host "Installed. Start with: Import-Module GitProtonBackup; Initialize-ProtonBackup"
} else {
    Write-Host "No GitProtonBackup module payload beside the script — helper-only install. For the module, clone the repository and re-run."
}
```

- [ ] **Step 3: Pester tests** (append a `Describe 'install.ps1 helper block'` to `tests/GitProtonBackup.Tests.ps1`, matching the file's Pester 5 style)

**Isolation rule for every test here:** copy `install.ps1` into a `TestDrive:` directory WITHOUT a `GitProtonBackup` payload folder (so the module block always takes its skip path and can never write to the real Documents modules directory — an earlier draft of this plan had exactly that contamination bug), and run it in a child `pwsh -NoProfile` with `$env:LOCALAPPDATA` pointed at a `TestDrive:` subdir. Three tests:
1. **Release-asset layout succeeds (the gate's exact case):** stage a fake exe + a `.sha256` naming its real digest beside the script copy → exit 0, exe present under the temp LOCALAPPDATA's `Programs\git-proton-backup\`, output contains "helper-only install".
2. **Checksum mismatch refuses:** `.sha256` naming a wrong digest → exit nonzero, nothing copied into the temp LOCALAPPDATA.
3. **Missing exe skips cleanly:** no exe beside the script copy → exit 0, output contains "skipping helper install".

> **SUPERSEDED (Stage 4 execution, Task 6 review, 2026-08-06):** two defects in this task's
> plan-supplied content, both found by the task review after verbatim transcription.
> (1) **The draft-release step's guard was ref-only.** `if: startsWith(github.ref,
> 'refs/tags/v')` also fires when a `workflow_dispatch` run is manually targeted AT a tag ref
> (the UI's "Use workflow from" dropdown or the API's `ref` parameter allows any ref), so a
> dispatch-on-tag would create the draft release the "build-only dry run" rule forbids. As
> executed the guard is `if: github.event_name == 'push' && startsWith(github.ref,
> 'refs/tags/v')`.
> (2) **The Pester isolation rule missed the registry channel.** Overriding
> `$env:LOCALAPPDATA` for the child `pwsh` sandboxes the filesystem writes, but
> `install.ps1`'s PATH persistence uses `[Environment]::SetEnvironmentVariable('Path', …,
> 'User')` — a direct `HKCU\Environment` registry write no process-env override contains.
> Every test run (and the implementer's manual sandbox runs) permanently appended a
> dangling GUID-temp entry to the REAL user PATH; four such entries were found on the
> machine and removed in the fix round. As executed, `install.ps1` gains a `-SkipPathUpdate`
> switch (default off — shipped behaviour unchanged; the Task 8 gate still proves the real
> PATH update end to end via fresh-shell resolution) and every Pester invocation of the
> installer passes it; the success-path test also asserts the PATH-update message is absent.

- [ ] **Step 4: CHANGELOG**

Add at the top of `CHANGELOG.md`:

```markdown
## 0.3.0 — unreleased

One version line covers both tools in this repository: the GitProtonBackup
PowerShell module (v1, bundles) and the git-remote-proton helper (v2, CAS
remote). This is the first release to ship the helper as a built artifact.

- **git-remote-proton:** certified-CLI allowlist enforced (exact build match;
  `GPB_UNCERTIFIED_CLI=1` overrides with a loud warning).
- **git-remote-proton:** `--set-head` changes a remote's default branch
  in-tool; the branch-delete refusal now names it as the remedy. `--version`
  prints the helper version and the certified CLI build.
- **install.ps1:** also installs the helper exe (checksum-verified when a
  `.sha256` sidecar is present) and adds it to the user PATH.
- Releases are published as drafts and made public only after the live gate
  passes against the exact built bytes.
```

- [ ] **Step 5: Verify lint + tests locally**

```
pwsh -NoProfile -Command "Invoke-ScriptAnalyzer -Path . -Recurse -Settings .\PSScriptAnalyzerSettings.psd1"
pwsh -NoProfile -Command "Invoke-Pester -Path tests -CI"
```
Expected: analyzer clean; Pester green. (The release workflow itself is proven live in Task 8 via a tag run — `workflow_dispatch` can be smoke-tested earlier if Craig wants, but that is his push/tag to make.)

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/release.yml install.ps1 tests/GitProtonBackup.Tests.ps1 CHANGELOG.md
git commit -m "build(v2): draft-until-gated release workflow; install.ps1 installs the helper

Draft releases on v* tags with contents:write declared; dispatch runs
build artifacts only. Helper block runs first and is checksum-verified;
module semantics unchanged.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 7: Docs — README coexistence + design doc v6.4

**Files:**
- Modify: `README.md` (new section after "How it works"; touch the Roadmap line about the CLI if it contradicts)
- Modify: `docs/v2-remote-helper-design.md` (version bump v6.3 → v6.4 + four additions)

**Interfaces:**
- Consumes: the spec's component 4 (README content) and component 6 (design-doc deltas) — both normative for content; Task 3/4/5's shipped behaviour for exact flag/command names (`GPB_UNCERTIFIED_CLI`, `--set-head <proton::address> <branch>`, `--version`).
- Produces: nothing code-facing.

- [ ] **Step 1: README — "Two tools in this repo: bundles (v1) and a git remote (v2)" section**

Write it from the spec's component 4, covering exactly: the side-by-side table (transport, remote name, restore needs); both-in-one-repo is safe; keep v2 remotes under a dedicated root (`/my-files/GitRemotes/`), never inside `GitBackups` (Bootstrap refuses non-empty unmarked folders; an EMPTY subfolder would be adopted — so don't create one there); the restore-contract difference stated honestly (v1: git only; v2: git + helper + certified CLI signed in); the scoped trash note verbatim in spirit: on an out-of-nowhere `already exists` push failure, capture `filesystem list`/`info` output and what the web UI trash shows BEFORE clearing anything (that capture is what turns the standing n=1 into evidence), then remove only project-path homonyms from the trash and retry — never "empty your trash" wholesale.

- [ ] **Step 2: Design doc v6.4**

Bump the version marker and add/update: (a) the allowlist section — refuse-by-default, exact-list, override env var, threat-model framing (compatibility gate, not provenance), and the explicit statement that `parseNodeJSON`'s dual-shape tolerance is defense in depth BEHIND the front door (this closes the documented design/code contradiction); (b) the `--set-head` operation — closed-set dispatch rationale, grammar, lock + verify-before-short-circuit order, `UpdateHEAD` vs `WriteHEAD` split; (c) the ref-transition table's delete row remedy text updated to name `--set-head`; (d) utility-mode stdout rationale (argv disjointness).

- [ ] **Step 3: Cross-check and commit**

Re-read both diffs against the spec's components 4 and 6 — every claim in the docs must match shipped behaviour (flag names, paths, error wording). Then:

```bash
git add README.md docs/v2-remote-helper-design.md
git commit -m "docs(v2): design v6.4 (allowlist, --set-head) + README v1/v2 coexistence and restore contracts

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 8: Stage 4 live gate

**Files:**
- Create: `docs/research/gates/stage4-gate.md` (the gate record, written by the runner)

**Interfaces:**
- Consumes: a draft GitHub Release created by Craig pushing the `v0.3.0` tag (Task 6's workflow); the live account; the spec's "The gate" section — its steps are NORMATIVE, this task summarizes them.
- Produces: the gate verdict. PASS → Craig publishes the draft. BLOCKED → fixes ship under the NEXT version (tags are never moved or reused).

**Precondition (Craig, not the runner):** push main, push the `v0.3.0` tag, confirm the draft release exists with exactly three assets; empty the Proton trash (the runner records trash-adjacent state but never empties).

- [ ] **Step 1: Author the gate brief** for the runner, containing verbatim: the standing rules (write confinement to `/my-files/GitRemotes/<demo>` + `/my-files/_cas-probe`; the four untouchable folders; verify-before-trash with exact paths; BLOCKED-on-surprise, never patch, never retry past a surprise; `-count=1` on the contract-table command; record pre-run `/my-files` listing AND what is known of trash state before assuming anything), plus the spec's four steps: (1) download all three draft assets into an empty gate directory, record each SHA256 in the gate log, verify the exe checksum, run `install.ps1` from that directory (no module payload is present, so the module block must SKIP with its helper-only message — a throw here is a Task 6 defect and a gate BLOCK), then from a fresh shell `git-remote-proton --version` must resolve from PATH and report the tag — every later step runs THIS binary; (2) allowlist live: real CLI passes; a PATH-shimmed wrong-version `proton-drive` (confined to the gate temp dir) proves refusal and `GPB_UNCERTIFIED_CLI=1` override; (3) `--set-head` end to end: demo repo with two branches, set-head to the second, verify HEAD via CLI + fresh clone checks out the new default, then delete the OLD default (must succeed) and attempt delete of the NEW default (must refuse, naming `--set-head`); (4) after Craig publishes: re-download every published asset and compare each SHA256 to the staged digests. Cleanup per standing rules; post-run listing must match pre-run active state.

- [ ] **Step 2: Dispatch the gate runner** (live-account rules from the runbook apply; the runner reports BLOCKED with verbatim output and never patches).

- [ ] **Step 3: Record and commit the INTERIM gate record** (steps 1–3 verdicts; the record states explicitly that the digest closure has not yet run and the verdict is provisional)

```bash
git add docs/research/gates/stage4-gate.md
git commit -m "gate(v2): stage 4 live gate record - steps 1-3, pre-publication

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

- [ ] **Step 4: Digest closure and FINAL verdict** — only after Craig publishes the draft: re-download every published asset, compare each SHA256 to the staged digests, append the comparison and the final verdict to the record, and commit again. The gate is not PASSED until this commit exists (spec: only the full comparison substantiates "the published bytes are the gated bytes").

```bash
git add docs/research/gates/stage4-gate.md
git commit -m "gate(v2): stage 4 gate final - publication digest closure

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

BLOCKED at any step → stop, report, plan the fix wave; the next attempt ships as `v0.3.1` (tags are never moved or reused).

---

## Self-review notes (done at authoring time)

- **Spec coverage:** allowlist → Task 3; `--set-head` (grammar, ordering, UpdateHEAD/WriteHEAD split, remedy text) → Tasks 4–5; release pipeline (draft, permissions, dispatch, installer, checksum, CHANGELOG) → Task 6; coexistence + design v6.4 → Task 7; hygiene items 1–9 → Tasks 1–2 (item 4, the 3b bullet, in Task 2); gate incl. per-asset digest closure → Task 8. EnsureDir tolerance: correctly absent (dropped by spec).
- **Type consistency:** `EnforceCertified(c *CLI, allowUncertified bool, w io.Writer) error` (Tasks 3, 5); `SetHead(t transport.Transport, root, branchArg string) (string, error)` (Tasks 4, 5); `UpdateHEAD(t transport.Transport, root, branch string) (transport.Outcome, error)` (Task 4); `var version = "dev"` / `-X main.version` (Tasks 5, 6); asset names `git-remote-proton.exe`, `.exe.sha256`, `install.ps1` (Tasks 6, 8).
- **Adjudicated judgment calls (peer-reviewed, do not re-litigate without new evidence):** missing binary refuses even with the override, surfacing the spawn error (`errors.Is(err, exec.ErrNotFound)`); a `ReadHEAD` error in `SetHead` is fatal — an indeterminate read never licenses an overwrite, and corrupt-HEAD repair goes delete-via-web-UI → headless → create path; `UpdateHEAD` keeps the `WriteRef`-style exists-branch because always-`UpdateRevision` would lean on uncertified upsert-on-absent behaviour; allowlist enforcement is an explicit call after each `NewCLI` site, not embedded in `NewCLI` (embedding breaks the fake-exe test machinery).

## Plan peer review (2026-08-04, Codex xhigh + Gemini 3.1 Pro via agy, one round)

Codex reviewed against the spec and the real code; Gemini reviewed the plan's code blocks. Applied: **installer works from the three release assets** — module block skips when no payload folder sits beside the script; without this the gate's install step throws and the stage cannot pass [Codex blocker]; **exact-token `IsCertified`** replacing containment (which accepted `…+5174900c-extra`), with unparseable-output tests and the genuine two-line certified fixture [Codex blocker]; missing-binary refusal made override-proof; `Version` returns full output so diagnostics carry the SDK line; bounded diagnostic quoting [Codex]; `SetHead` fails closed on any `ReadHEAD` error (test 17 added) [Codex]; `UpdateRevision`-overriding outcome decorator required for the update-path arms [Codex]; counting decorator (not `NewTraced`) for the zero-writes idempotence assert [Codex]; two-pack fixture corrupts with garbage bytes and states its honest scope (pins completion through mid-plan refresh, does not empirically distinguish restart from resume) [Codex]; hierarchical error names Stage 5 verbatim [Codex]; release workflow pins `go-version-file: go.mod` and runs PSScriptAnalyzer + Pester (the installer is a shipped asset) [Codex]; gate record split into interim and post-publication-closure commits [Codex]; Task 1 step 5 snippet grounded in the real test's locals with an explicit `NewTraced` wrap [Gemini]; installer PATH handles empty user PATH and trailing-backslash duplicates [Gemini]; Pester tests isolated so no test can ever write to the real Documents modules directory [Codex]. Rejected with reasons: always-`UpdateRevision` for `UpdateHEAD` and enforcement-inside-`NewCLI` (both above); byte-level read bounding inside `run()` (shared machinery, WaitDelay-guarded; bound applied at the diagnostic layer and the narrowing recorded).
