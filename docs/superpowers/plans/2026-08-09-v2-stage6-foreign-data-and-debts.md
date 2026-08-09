# v2 Stage 6 — Foreign-Data Per-Operation Policy + Debt Ledger: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship per-operation foreign-data policy (tolerant push survey, strict fetch survey with enumeration), occupancy-aware push refusals, and the Stage 5 debt ledger, gated live and released as v0.5.0.

**Architecture:** Three layers unchanged (protocol / repository / transport). The advertisement walk becomes a typed scan (`RefScan`: advertised map + classified skipped set); the protocol layer applies one of two policies per command; the batch engine consumes the skipped set in its preflight and delete arm. The spec is normative: `docs/superpowers/specs/2026-08-09-v2-stage6-foreign-data-and-debts-design.md` (read it before any task).

**Tech Stack:** Go stdlib only; PowerShell 7 + Pester for install.ps1; real `git` for fixtures.

## Global Constraints

- **Go stdlib only.** No new dependencies, no cgo.
- **Every shell step that runs `go` or `git` in PowerShell must first prepend the fresh PATH** (stale-PATH gotcha, runbook): `$env:Path = [Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [Environment]::GetEnvironmentVariable('Path','User')`. Written once here; every `Run:` line below assumes it.
- **Hermetic tests only** in this plan's execution. Live halves run **only at the stage gate** under `GPB_LIVE_ACCOUNT=1`; nothing in this plan touches the real account. NEVER set `GPB_LIVE_ACCOUNT`.
- **All tests run with `-count=1`.**
- **Label every new test RED or GUARD**; fix rounds report which assertion fired; the load-bearing behaviours (typed skip split, size gate, strict-fetch enumeration, occupancy preflight, delete refusal, heal-arm diagnosis) get deliberate-regression checks before their task is reported done.
- **Plan-supplied code blocks are hypotheses** (three SUPERSEDED banners in Stage 5, five in Stage 4). When a defect is found in one, patch this plan with a SUPERSEDED banner.
- **Commit messages end with:** `Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>`. Never push; Craig merges.
- **Env vars** (`GPB_CREATE_PARENTS`, `GPB_UNCERTIFIED_CLI`, and new `GPB_CONTRACT_LIVE_ROOT`) are read fresh every invocation, never cached.
- **The foreign-data rule:** the helper NEVER modifies or deletes a foreign file or folder, on any path, in any task.
- **gofmt on CRLF:** `gofmt -l` false-positives on CRLF-rewritten working-tree files; never "fix" line endings.
- **Sha collisions in fixtures:** pin `GIT_COMMITTER_DATE`/`GIT_AUTHOR_DATE` when a fixture needs distinct histories with repeated content.

## File Structure

```
internal/repo/refs.go                      MOD  ScanRefs (was ListRefs): RefScan result, size gate, typed errMalformedRef
internal/repo/refscan.go                   NEW  RefScan / SkippedRef / SkipKind types + kind-aware occupancy messages
internal/repo/repo_test.go                 MOD  scan, policy, occupancy, heal-arm tests
cmd/git-remote-proton/main.go              MOD  strict list arm; tolerant list for-push; occupancy into Push
cmd/git-remote-proton/main_test.go         MOD  loop-level policy tests (restore shape, degraded states)
internal/repo/push.go                      MOD  Push gains skipped set; occupancy preflight + delete refusal; heal-arm diagnosis
internal/repo/sethead.go                   MOD  suggestion list reads scan.Refs (mechanical)
internal/transport/fake.go                 MOD  ReadTo directory fidelity (F1)
internal/transport/fake_test.go            MOD  F1 fidelity test
internal/transport/contract_test.go        MOD  GPB_CONTRACT_LIVE_ROOT + validation; F1 live row
internal/testcli/testcli.go                MOD  runDownload doc comment cites the F1 row
tests/InstallHelper.Tests.ps1              MOD  GetValue $options spy + 2 cases
internal/repo/repo_test.go (parents)       MOD  mount-is-a-file branch test
docs/research/gates/brief-checklist.md     MOD  contract-root line + long-timeout guidance
docs/v2-remote-helper-design.md            MOD  v6.6
README.md                                  MOD  foreign-data paragraph (per-operation)
CHANGELOG.md                               MOD  Unreleased entries; flip in Task 9
docs/research/gates/stage6-gate-brief.md   NEW  Task 9
```

Dependency spine: Task 1 (scan) precedes 2 (policy wiring) and 3–4 (occupancy consumers); Tasks 5–7 are independent of the spine; Task 8 (docs) after 1–7; Task 9 last.

Code archaeology this plan is grounded in: `transport.Node` carries `Size int64` (transport.go:41-45), populated by the Fake from content length (fake.go:139,161) and by the CLI from `claimedSize` in both node-JSON shapes (cli.go:120,129) — the size gate needs no transport change. `ListRefs` production call sites: main.go:357 (fetch advertisement), main.go:414 (push advertisement), main.go:476 (push execution's remote map — the natural occupancy source, same helper invocation), sethead.go:127 (suggestion list). The HEAD symref is already emitted only when its target is in the advertised map (main.go:365-388) — the HEAD degraded state needs only a note added, not new suppression logic. `readRef` (refs.go:128-150) returns untyped errors today; grammar failure is the `len(raw) != 41 || ...` arm at refs.go:145. Heal wrapper's file arm: push.go:675 (`if !ok || !n.IsDir` surfaces the original result). `finalSet` preflight: push.go:229-270. `liveRoot` const: contract_test.go:14-15, used at :426-439.

---

### Task 1: Typed scan — `ScanRefs`, size-gated classification, `errMalformedRef`

**Files:**
- Create: `internal/repo/refscan.go`
- Modify: `internal/repo/refs.go:52-167` (ListRefs→ScanRefs; readRef typed error; band constants)
- Modify: `cmd/git-remote-proton/main.go:357,414,476`, `internal/repo/sethead.go:127` (mechanical `.Refs`)
- Modify: `internal/repo/repo_test.go`

**Interfaces:**
- Consumes: `transport.Node.Size`; `checkComponent`/`advertisableName`; `skipNote`; `previewBytes`.
- Produces (Tasks 2–4 and 8 rely on these exact names):

```go
// refscan.go
type SkipKind int

const (
	SkipInvalidName       SkipKind = iota // file whose name failed validation; contents never examined
	SkipInvalidNameFolder                 // folder whose name failed validation; subtree never entered
	SkipContent                           // well-named file whose contents are not a ref
)

type SkippedRef struct {
	Path   string   // full path relative to root, e.g. "refs/heads/Thumbs.db"
	Kind   SkipKind
	Reason string   // classified, human-readable; the note/error text body
	Hex    string   // 40-hex recovered from a noncanonical candidate, else ""
}

type RefScan struct {
	Refs    map[string]string // advertised name -> sha (exactly the old ListRefs map)
	Skipped []SkippedRef      // every skipped path, both kinds, walk order
}

// ContentSkips returns the SkipContent subset (the strict fetch survey's trigger).
func (s *RefScan) ContentSkips() []SkippedRef

// OccupancyMessage renders the kind-aware refusal body for a skipped path
// (spec component 2): SkipContent -> "a file occupies <path> and its contents
// are not a ref (<reason>); delete it first (proton-drive filesystem trash
// <root>/<path>, or the web UI)"; SkipInvalidName -> "a file with an invalid
// ref name occupies <path> (contents never examined); delete or rename it
// first (...)"; SkipInvalidNameFolder -> "a folder with an invalid name
// occupies <path>; its contents were never examined - inspect it before
// removing anything (...)".
func OccupancyMessage(root string, s SkippedRef) string
```

- `func ScanRefs(t transport.Transport, root string) (*RefScan, error)` — replaces `ListRefs` (hard rename; no compat wrapper — every caller updates in this task).
- `var errMalformedRef = errors.New(...)` (unexported sentinel, refs.go): `readRef` wraps ONLY the grammar-failure arm with it (`%w`); transport/read failures stay unwrapped. Callers classify with `errors.Is`.
- Band constants (refs.go): `refBandMin = 40`, `refBandMax = 44` (spec: no-LF=40, exact=41, CRLF/double-LF=42, BOM-prefixed up to 44).

- [ ] **Step 1: Write the failing tests** (repo_test.go). Stub `ScanRefs`/types first so REDs are behavioural:
```go
// RED: junk beside good refs — Refs has the good refs only; Skipped carries
// the junk with Kind=SkipContent and its classified Reason; note emitted.
func TestScanRefsClassifiesContentJunkBesideGoodRefs(t *testing.T)
// Fake: refs/heads/main (valid 41B), refs/heads/junk (contents "hello world\n",
// 12B -> out-of-band). Assert Refs == {main}, one Skipped{Path:"refs/heads/junk",
// Kind:SkipContent, Reason contains "size 12"}, captured stderr contains the path.

// RED (size gate, trace-asserted): an out-of-band file is skipped WITHOUT any
// ReadTo — the traced transport records no download of its path.
func TestScanRefsOutOfBandSkipsWithoutDownload(t *testing.T)
// 12B junk: no ReadTo call names it. In-band control: refs/heads/main IS read.

// RED: in-band non-ref (41B of junk, e.g. "x" repeated 40 + "\n") downloads,
// fails grammar, skips with escaped preview in Reason.
func TestScanRefsInBandJunkSkipsWithPreview(t *testing.T)

// RED (noncanonical): 40B no-LF and 42B CRLF fixtures — both downloaded
// (in-band), both skipped, Reason says malformed terminator, Hex carries the
// 40-hex. These are the Stage 5 fatal-pinning fixtures FLIPPED: reuse the
// exact fixture bytes from TestListRefsMalformedContentStillFatal, which this
// task RETITLES to TestScanRefsMalformedContentSkipsWithHex (fatal assertions
// -> skip assertions; the old test name must not survive, its premise is
// retired by the spec).
func TestScanRefsNoncanonicalHexRecovered(t *testing.T)

// RED: 42B non-hex junk (in-band, wrong content) -> SkipContent, no Hex.
func TestScanRefsInBandNonHexHasNoHex(t *testing.T)

// GUARD (typed split — deliberate regression required): a transport ReadTo
// failure during the walk stays FATAL (error names the path; no partial map);
// the grammar failure beside it skips. Build with a failing-ReadTo wrapper
// for one path + a junk file at another.
func TestScanRefsTransportFailureStaysFatalWhileGrammarSkips(t *testing.T)

// RED: name-skips now recorded — invalid-named file -> SkipInvalidName;
// invalid-named folder -> SkipInvalidNameFolder (and STILL no List beneath
// it — extend the existing trace assertion, do not weaken it).
func TestScanRefsRecordsNameSkipsAsOccupancies(t *testing.T)

// RED: OccupancyMessage renders all three kinds; folder text contains
// "inspect" and never the word "file"; SkipInvalidName says "contents never
// examined"; SkipContent contains the CLI trash grammar with the full path.
func TestOccupancyMessageKindAware(t *testing.T)

// GUARD: size-unknown arm — a wrapper transport that zeroes Size... NOTE:
// Fake reports real sizes, so build a listWrapper that sets Size=-1 for one
// file; assert skip-without-download with "size unknown" in Reason.
// (Node.Size zero-value ambiguity: a genuinely 0-byte file IS out-of-band
// (0 < 40) and needs no special arm; use -1 as the cannot-know sentinel in
// the wrapper. The CLI never reports negative sizes; document in the arm.)
func TestScanRefsUnknownSizeSkipsWithoutDownload(t *testing.T)
```
- [ ] **Step 2: Run, expect FAIL** for the right reasons: `go test ./internal/repo/ -run TestScanRefs -count=1` (stubs return empty scan → assertions fire; record which).
- [ ] **Step 3: Implement.** refscan.go: the types + `ContentSkips` + `OccupancyMessage` exactly as the Interfaces block. refs.go: rename, then inside the walk replace the fatal `readRef` arm:
```go
sha, err := readRefClassified(t, root, full, n.Size, scan)
if err != nil {
	return err // transport failure — fatal, unchanged
}
if sha != "" {
	scan.Refs[full] = sha
}
```
with:
```go
// readRefClassified applies the size gate and grammar check for ONE candidate.
// It returns ("", nil) after recording a SkippedRef (skip is not an error);
// (sha, nil) on success; ("", err) ONLY for transport/read failures.
func readRefClassified(t transport.Transport, root, full string, size int64, scan *RefScan) (string, error) {
	if size < refBandMin || size > refBandMax {
		reason := fmt.Sprintf("not a ref: size %d outside the %d-%d candidate band", size, refBandMin, refBandMax)
		if size < 0 {
			reason = "not a ref: size unknown; refusing to download unbounded content"
		}
		s := SkippedRef{Path: full, Kind: SkipContent, Reason: reason}
		scan.Skipped = append(scan.Skipped, s)
		skipNote(os.Stderr, root, full, errors.New(s.Reason))
		return "", nil
	}
	sha, err := readRef(t, root+"/"+full)
	if err == nil {
		return sha, nil
	}
	if !errors.Is(err, errMalformedRef) {
		return "", err // transport failure stays fatal (typed split)
	}
	raw := malformedRaw(err) // the raw bytes readRef captured; see Step 3 note
	s := SkippedRef{Path: full, Kind: SkipContent, Reason: fmt.Sprintf("not a ref: %s", previewEscaped(raw))}
	if hex := noncanonicalHex(raw); hex != "" {
		s.Hex = hex
		s.Reason = fmt.Sprintf("damaged ref? contents are 40-hex with a malformed terminator: %s", hex)
	}
	scan.Skipped = append(scan.Skipped, s)
	skipNote(os.Stderr, root, full, errors.New(s.Reason))
	return "", nil
}
```
Supporting pieces (hypotheses — adapt mechanically): `readRef`'s grammar arm becomes `return "", &malformedRefError{p: p, raw: raw}` where the error type wraps `errMalformedRef` via `Is` and exposes the raw bytes (`malformedRaw(err)` unwraps it; `errors.As` under the hood); `noncanonicalHex(raw []byte) string` returns the hex when `shaRe.Match` succeeds on a 40-hex prefix or BOM-stripped prefix and the remainder is only `\r`/`\n` bytes, else ""; `previewEscaped` = `previewBytes` + hex-escape of control bytes (`%q` on the preview string is acceptable — it escapes control bytes; say so in the comment). Name-skip arms: the two existing `skipNote` sites additionally append `SkippedRef{Kind: SkipInvalidName / SkipInvalidNameFolder, Reason: err.Error()}`. Callers: main.go three sites + sethead.go read `scan.Refs` (behaviour-neutral in THIS task — policy lands in Task 2); every repo_test.go `ListRefs(` call updates mechanically.
- [ ] **Step 4: Run the full suite** (`go test ./... -count=1`); fix mechanical fallout (existing tests asserting fatal-on-content now flip — ONLY the retitled test's assertions change semantically; list every other edited test in the report with one-line reasoning).
- [ ] **Step 5: Deliberate regressions:** (a) make readRefClassified treat transport errors as skips → the typed-split GUARD fails; (b) drop the band check (always download) → the no-download trace test fails; (c) drop the Hex recovery → noncanonical test fails. Record which assertion fired; revert each.
- [ ] **Step 6: Commit.**
```bash
git add internal/repo cmd/git-remote-proton
git commit -m "feat(repo): typed ref scan - size-gated content classification, occupancy set, hex recovery"
```

---

### Task 2: Per-operation policy at the protocol layer

**Files:**
- Modify: `cmd/git-remote-proton/main.go:340-423` (both list arms)
- Modify: `cmd/git-remote-proton/main_test.go`

**Interfaces:**
- Consumes: `repo.ScanRefs`, `scan.ContentSkips()`, `repo.OccupancyMessage`, `SkippedRef.Hex`.
- Produces: the fetch-direction enumerated error (exact shape below — Task 9's gate brief quotes it); the push advertisement unchanged-but-noted; the HEAD skip note.

- [ ] **Step 1: Write the failing loop-level tests** (main_test.go, driving `loop()` like the existing protocol tests):
```go
// RED (the per-operation core): the SAME Fake fixture (good ref + content
// junk) through both commands. "list" -> exit nonzero, stderr carries ONE
// error block enumerating the junk path, its reason, and the trash remedy;
// NO ref lines on stdout. "list for-push" -> refs advertised, junk skipped
// with note, exit flow normal.
func TestLoop_ListIsStrictOnContentSkipsListForPushTolerant(t *testing.T)

// RED: enumerated error includes recovered Hex when present (CRLF fixture).
func TestLoop_ListStrictErrorCarriesDamagedRefHex(t *testing.T)

// RED: name-skipped junk only (file ".hidden" under refs/heads) -> "list"
// SUCCEEDS with notes (the principled line, fetch direction).
func TestLoop_ListTolerantOnNameSkips(t *testing.T)

// RED: HEAD names a content-skipped ref -> "list for-push" advertises others,
// no HEAD line... NOTE: list for-push never emits a HEAD line (main.go:392-423)
// — HEAD suppression is the "list" arm's logic, and strict "list" fails first
// on content-skips. So the HEAD-note case is NAME-skips on "list": HEAD ->
// refs/heads/.hidden (manufactured HEAD content naming a name-skipped ref):
// symref line absent (existing logic), NEW: a note names why.
func TestLoop_ListHeadNamingNameSkippedRefNotedNotAdvertised(t *testing.T)

// RED: all-name-skipped repo -> "list" succeeds with empty advertisement +
// notes (clone-of-empty shape); all-content-skipped -> "list" fails with all
// paths enumerated.
func TestLoop_ListDegradedStates(t *testing.T)
```
- [ ] **Step 2: Run, expect FAIL** (`go test ./cmd/... -run TestLoop_List -count=1`).
- [ ] **Step 3: Implement.** In the `"list"` arm after `ScanRefs`:
```go
if cs := scan.ContentSkips(); len(cs) > 0 {
	// STRICT fetch survey (spec component 1): complete-or-loudly-incomplete.
	var b strings.Builder
	fmt.Fprintf(&b, "cannot serve a fetch: %d file(s) under refs/ are not valid refs "+
		"and a restore would silently lack them:\n", len(cs))
	for _, s := range cs {
		fmt.Fprintf(&b, "  %s/%s: %s\n", root, s.Path, s.Reason)
	}
	fmt.Fprintf(&b, "delete these files first (proton-drive filesystem trash <path>, or the "+
		"web UI; Proton trash keeps them restorable), then retry")
	warn(errors.New(b.String()))
	return 1
}
```
The HEAD block gains, beside the existing suppressed-symref path: if `hasHead` and the branch is in `scan.Skipped` (any kind), `fmt.Fprintf(os.Stderr, "git-remote-proton: HEAD names %s, which was skipped (%s); advertising no default branch\n", branch, reason)`. `"list for-push"` arm: no policy change (scan.Refs advertised; notes already emitted by the walk).
- [ ] **Step 4: Full suite; deliberate regression:** remove the strict check → the per-operation RED fails on its "list must fail" half. Revert. Commit.
```bash
git add cmd/git-remote-proton
git commit -m "feat(protocol): per-operation policy - strict fetch survey with enumeration, tolerant push survey"
```

---

### Task 3: Occupancy-aware push — preflight and delete arm

**Files:**
- Modify: `internal/repo/push.go:53` (signature), `:229-270` (preflight), delete arm
- Modify: `cmd/git-remote-proton/main.go:476-481` (pass the scan)
- Modify: `internal/repo/repo_test.go`

**Interfaces:**
- Consumes: `RefScan.Skipped`, `OccupancyMessage`.
- Produces: `Push(t transport.Transport, root, gitDir string, ups []protocol.RefUpdate, remote map[string]string, skipped []SkippedRef) []Result` — every existing test call site adds `nil`; the three new refusal shapes below.

- [ ] **Step 1: Failing tests** (repo_test.go; drive `Push` directly with a manufactured `skipped` slice + matching Fake state):
```go
// RED: create of a skipped name itself -> refused in phase 2 with
// OccupancyMessage text; NO pack built (countPackFiles == 0).
func TestPushCreateOfSkippedNameRefusedPrePack(t *testing.T)
// RED: create BENEATH a skipped file (refs/heads/foo skipped; create
// refs/heads/foo/bar) -> refused pre-pack naming foo.
func TestPushCreateBeneathSkippedFileRefusedPrePack(t *testing.T)
// RED: create ABOVE skipped content (refs/heads/foo/bar skipped; create
// refs/heads/foo) -> refused pre-pack naming foo/bar.
func TestPushCreateAboveSkippedFileRefusedPrePack(t *testing.T)
// RED: kind-aware — the beneath-case with a SkipInvalidNameFolder occupancy:
// message contains "inspect", never calls the folder a file.
func TestPushSkippedFolderOccupancyMessageKindAware(t *testing.T)
// RED: delete of a skipped name -> refused with OccupancyMessage; the foreign
// file survives byte-identical; NOT reported ok.
func TestPushDeleteOfSkippedNameRefusedUntouched(t *testing.T)
// GUARD: an unrelated ref in the same batch still succeeds (per-ref statuses).
func TestPushOccupancyRefusalIsPerRef(t *testing.T)
// GUARD: nil/empty skipped slice -> exact Stage 5 behaviour (no new refusals).
func TestPushNilSkippedIsStage5Behaviour(t *testing.T)
```
- [ ] **Step 2: Run, expect FAIL** (`go test ./internal/repo/ -run "TestPushCreateOfSkipped|TestPushCreateBeneath|TestPushCreateAbove|TestPushSkippedFolder|TestPushDeleteOfSkipped|TestPushOccupancy|TestPushNilSkipped" -count=1`).
- [ ] **Step 3: Implement.** Build once in phase 2, before the finalSet loop:
```go
occupied := make(map[string]SkippedRef, len(skipped))
for _, s := range skipped {
	occupied[s.Path] = s
}
```
In the per-update validation loop (with `checkDst` already passed): for a DELETE, `if s, ok := occupied[u.Dst]; ok { failed(i, OccupancyMessage(root, s)); continue }`. For a CREATE/UPDATE, three checks in order — exact name, ancestor (walk `parentOf(u.Dst)` upward against `occupied`), descendant (linear scan: `strings.HasPrefix(s.Path, u.Dst+"/")`) — first hit refuses with `OccupancyMessage(root, s)`. All refusals happen before `newShas`/`valid[i]` assignment, so the pack (phase 3) never includes them (the existing no-pack-on-preflight-refusal structure carries this for free — assert it anyway). main.go:476: `scan, err := repo.ScanRefs(t, root)` → `repo.Push(t, root, gitDir, ups, scan.Refs, scan.Skipped)`.
- [ ] **Step 4: Full suite** (every existing `Push(` test call gains `, nil` — mechanical; list any test whose SEMANTICS needed change, expected: none). **Deliberate regression:** drop the descendant check → the above-case test fails; drop the delete check → delete test reports a false ok. Revert; commit.
```bash
git add internal/repo cmd/git-remote-proton
git commit -m "feat(push): occupancy-aware preflight and delete refusals - pre-pack, kind-aware"
```

---

### Task 4: Create-heal wrapper race-window diagnosis

**Files:**
- Modify: `internal/repo/push.go:660-690` (the `!ok || !n.IsDir` arm of `createRefHealingCollision`)
- Modify: `internal/repo/repo_test.go`

**Interfaces:**
- Consumes: `transport.Stat` (Node.Size), `readRef`/`errMalformedRef`, band constants, `previewEscaped`.
- Produces: the race-arm diagnosis message (Task 8 documents it; the gate brief quotes it).

- [ ] **Step 1: Failing tests:**
```go
// RED: occupant appeared post-advertisement (empty skipped slice; Fake holds
// a 12B junk file at the dst) -> create refused; message says a file occupies
// the name, contents not a ref, size-classified WITHOUT download (trace: no
// ReadTo of the occupant), remedy present. NOT "ref changed concurrently".
func TestHealArmDiagnosesOversizedOccupantWithoutDownload(t *testing.T)
// RED: in-band junk occupant (41B non-hex) -> downloaded, diagnosed with
// escaped preview.
func TestHealArmDiagnosesInBandJunkOccupant(t *testing.T)
// GUARD: occupant IS a valid ref (41B, real sha) -> existing concurrent-
// creator message, byte-unchanged (assert exact current text).
func TestHealArmValidRefOccupantKeepsConcurrentCreatorMessage(t *testing.T)
// GUARD: diagnostic Stat/read failure -> the ORIGINAL refusal stands with the
// failure noted; nothing invented.
func TestHealArmDiagnosticFailureFallsBackToOriginal(t *testing.T)
```
- [ ] **Step 2: Run, expect FAIL.**
- [ ] **Step 3: Implement** in the file arm (currently `return out, err // not a folder collision`): when `ok && !n.IsDir`, size-gate `n.Size` against the band; out-of-band → refusal citing size; in-band → `readRef`; `errors.Is(err, errMalformedRef)` → refusal with escaped preview + remedy; valid sha → the existing concurrent-creator return untouched; read failure → original result + `(diagnosis unavailable: <err>)` suffix. Message body: `a file occupies <ref> and its contents are not a ref (<reason>); delete it first (proton-drive filesystem trash <root>/<ref>, or the web UI)` — present-tense observation only (spec: never claim "was skipped at advertisement").
- [ ] **Step 4: Full suite; deliberate regression:** force the arm to always return the original → both RED tests fail with the old concurrent-creator text (record). Revert; commit.
```bash
git add internal/repo
git commit -m "feat(push): heal-arm race diagnosis - size-gated occupant classification with remedy"
```

---

### Task 5: F1 — directory-download contract row + Fake fidelity + shim comment

**Files:**
- Modify: `internal/transport/fake.go` (ReadTo on a directory), `fake_test.go`, `contract_test.go`
- Modify: `internal/testcli/testcli.go` (doc comment only)

**Interfaces:**
- Consumes: Stage 5 gate F1 observation (stage5-gate.md): `filesystem download` of a directory exits 0 and recursively downloads the subtree.
- Produces: `Fake.ReadTo(dirPath, dest)` recursively materialises `dest/<leaf>/...`; the live row `download of a directory recursively materialises the subtree`.

- [ ] **Step 1: Failing Fake test:** seed `Dirs["/r/d"]`, `Files["/r/d/a"]="A"`, `Dirs["/r/d/sub"]`, `Files["/r/d/sub/b"]="B"`; `ReadTo("/r/d", dest)` → nil, `dest/d/a` == "A", `dest/d/sub/b` == "B" (the F1 fixture shape, pinned).
- [ ] **Step 2: Run, expect FAIL** (Fake.ReadTo today misses on `Files[p]` for a dir path — verify the actual failure mode and record it).
- [ ] **Step 3: Implement** in Fake.ReadTo: if `f.Dirs[p]`, walk `Files`/`Dirs` under `p+"/"`, `os.MkdirAll` + write each under `filepath.Join(dest, path.Base(p), rel)`; return nil. Add the live contract row per the existing row pattern: fake half asserts the layout above; live half creates the same tree under the (Task 6) validated root, downloads, asserts layout, records verbatim output. Update `runDownload`'s doc comment in testcli.go: directory targets misreport as notFound; the REAL contract is recursive success — cite the new row; shim stays file-oriented and no shim test may rely on directory download.
- [ ] **Step 4: Full suite; commit.**
```bash
git add internal/transport internal/testcli
git commit -m "test(transport): F1 directory-download contract row + Fake ReadTo fidelity"
```

---

### Task 6: `GPB_CONTRACT_LIVE_ROOT` with fail-closed validation

**Files:**
- Modify: `internal/transport/contract_test.go:14-15,426-439`
- Modify: `docs/research/gates/brief-checklist.md`

**Interfaces:**
- Produces: `func contractLiveRoot(t *testing.T) string` — env override with validation; the brief-checklist lines Task 9 cites.

- [ ] **Step 1: Failing hermetic tests** (subtests of a new `TestContractLiveRootValidation`; validation must run WITHOUT `GPB_LIVE_ACCOUNT` — it is pure string logic; use `t.Setenv`):
reject `/my-files` (root itself), `/my-files/GitBackups/x` + the other three untouchable prefixes, `/other/x` (outside), `/my-files/x/../../etc` and `/my-files/x/../GitBackups` (dot segments — reject ANY `.`/`..` segment on the RAW string), `relative/path`; accept the default and `/my-files/_cas-probe/other`. Each rejection names the offending value.
- [ ] **Step 2: Run, expect FAIL** (helper undefined).
- [ ] **Step 3: Implement:** `contractLiveRoot` reads `GPB_CONTRACT_LIVE_ROOT` (default `liveRoot` const), splits on `/`, validates segment-wise (absolute; first segment `my-files`; ≥2 segments; no empty/`.`/`..` segments; second segment not in the untouchables list `{"GitBackups","Sensitive Project Sources","Project Repo Bundles","ChatGPT Export Text Backup"}` — **verify the exact four names against the runbook/stage5-gate.md before hardcoding; they are account facts, not inventions**), `t.Fatalf` on violation. `TestContractCLI` uses it in place of `liveRoot`. Brief-checklist gains two lines: (1) the contract table's root must be stated in the brief and included in its confinement list (`GPB_CONTRACT_LIVE_ROOT` if non-default); (2) gate runners run pushes with a long tool timeout or in background — a harness timeout mid-push orphans the remote lock (Stage 5 S1).
- [ ] **Step 4: Full suite (validation subtests run hermetically; `TestContractCLI` still skips loudly); commit.**
```bash
git add internal/transport docs/research/gates/brief-checklist.md
git commit -m "test(contract): configurable live root with fail-closed segment validation; checklist lines"
```

---

### Task 7: install.ps1 mock options spy + parents.go leftover test

**Files:**
- Modify: `tests/InstallHelper.Tests.ps1`
- Modify: `internal/repo/repo_test.go` (one test)

**Interfaces:**
- Consumes: the Task 2 (Stage 5) mock key (`New-MockEnvironmentKey`, `.GetCalls`-style recorders via GetNewClosure); `EnsureParents`.
- Produces: options-recording mock; two Pester cases; `TestEnsureParentsMountIsAFile`.

- [ ] **Step 1: Failing Pester cases.** Extend the mock's `GetValue` ScriptMethod to append `$options` to a `GetCalls` list (same closure pattern as SetCalls — the Stage 5 SUPERSEDED banner in the old plan explains why `$script:` scope does NOT work; use the closure). Cases: RED `install.ps1 passes DoNotExpandEnvironmentNames on its PATH read` — after a run, assert at least one recorded `$options` equals `[Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames`; GUARD `the recorder distinguishes a flagless call` — invoke the mock's GetValue directly with `$null`/`None` options and assert the recorded entry differs from the flagged one (pins the spy itself; round-1 Gemini).
- [ ] **Step 2: Run `Invoke-Pester tests/InstallHelper.Tests.ps1 -Output Detailed`** — RED fails while the recorder doesn't exist; then implement; then the RED must PASS against the real install.ps1 (which already passes the flag) — **the deliberate regression IS the point here:** temporarily remove `DoNotExpandEnvironmentNames` from install.ps1's GetValue call, watch the RED fail, restore. Record.
- [ ] **Step 3: Go leftover:** `TestEnsureParentsMountIsAFile` — Fake with `Files["/my-files"]`... **NOT possible: /my-files is a built-in mount in the Fake (Stage 5 Task 11 leniency). Check `isBuiltinMountParent`/`Fake.Stat` first**: manufacture instead a DEVICE mount as a file? Also built-in. The testable arm is the `!n.IsDir` branch of EnsureParents' mount check — reachable via a wrapper transport whose `Stat("/my-files")` returns `(Node{IsDir:false}, true, nil)`. Assert the exact `mount %s is not a folder` error. Run, implement nothing (the branch exists — this is a GUARD), confirm pass.
- [ ] **Step 4: Full Pester suite + full Go suite; commit.**
```bash
git add tests/InstallHelper.Tests.ps1 internal/repo/repo_test.go
git commit -m "test: registry-mock options spy (DoNotExpandEnvironmentNames pinned); EnsureParents mount-file guard"
```

---

### Task 8: Docs — design v6.6, README, CHANGELOG entries

**Files:**
- Modify: `docs/v2-remote-helper-design.md`, `README.md`, `CHANGELOG.md`

**Interfaces:** consumes every shipped behaviour above; spec component 4 is the item-by-item checklist. **Write only after Tasks 1–7 are merged into the stage branch; verify every claim against the code as merged.**

- [ ] **Step 1: v6.6 edits, one revision entry** per spec component 4: per-operation content rows (push-survey skip+note vs fetch-survey enumerated failure — quote the Task 2 error shape); the v6.5 OPEN question replaced with the adjudicated per-operation rationale + spec pointer; occupancy refusal rows (kind-aware, pre-pack) + delete refusal + git-porcelain lock-out row; heal-arm race diagnosis row; HEAD/all-skipped degraded states (per-direction as specced); F1 recorded beside ReadTo/C16 with the row cited; size-gate band + best-effort residual documented.
- [ ] **Step 2: README** foreign-data paragraph exactly as specced (component 4 wording: fetch-blocking class is valid-ref-named files with unparseable contents; invalid-named note-only both directions; mirror-alarm intended; lock-out + CLI/web-UI remedy; trash restorable).
- [ ] **Step 3: CHANGELOG Unreleased entries:** per-operation foreign-data policy (user-visible behaviour change: fetch/clone/ls-remote now fail loudly on unparseable ref files instead of silently succeeding without them — BREAKING-ish, flag prominently; push unaffected); occupancy refusals with actionable messages; damaged-ref hex recovery in errors/notes; F1 contract fact. Do NOT flip the version (Task 9).
- [ ] **Step 4: Commit.**
```bash
git add docs/v2-remote-helper-design.md README.md CHANGELOG.md
git commit -m "docs: design v6.6 - per-operation foreign-data policy, occupancy refusals, F1"
```

---

### Task 9: Release prep (v0.5.0) + gate brief — then STOP for Craig

**Files:**
- Modify: `CHANGELOG.md` (the flip, per `docs/releasing.md`)
- Create: `docs/research/gates/stage6-gate-brief.md`

**Interfaces:** consumes `docs/releasing.md`, `docs/research/gates/brief-checklist.md` (incl. Task 6's new lines), spec component 5's gate outline.

- [ ] **Step 1: Write the gate brief** implementing spec component 5's live-gate outline verbatim (its five steps, in its order — junk manufacture, tolerant-push-then-strict-ls-remote with the junk IN PLACE for step 2's occupancy refusal, THEN deletion and recovery; contract table via `GPB_CONTRACT_LIVE_ROOT` named in the confinement list; hierarchical smoke; cleanup). Carry the standing rules by reference + inline where load-bearing (row sets, `-count=1`, verify-before-trash, BLOCKED-verbatim-never-patch, long-timeout pushes per the new checklist line, empty-trash-before-gate). Quote verbatim the Task 2 enumerated-error shape and the Task 3/4 refusal messages as the strings the runner asserts (BLOCK on mismatch). State the remote scheme-vs-alias convention up front (Stage 5's lesson — `proton::` URL scheme, `proton-v2` alias, `git clone -o`); every command literal; every step names its repo/alias; `git init -b main` everywhere.
- [ ] **Step 2: Flip the CHANGELOG** per `docs/releasing.md` step 2, in the file's own unbracketed convention (`## 0.5.0 — <date>`), commit exactly that with the brief:
```bash
git add CHANGELOG.md docs/research/gates/stage6-gate-brief.md
git commit -m "chore(release): v0.5.0 CHANGELOG flip; Stage 6 live gate brief"
```
- [ ] **Step 3: STOP.** Report to Craig: stage branch ready for merge review; merging, pushing, tagging v0.5.0 (only after CI green on main — the standing dead-tag rule), the Release workflow, the live gate, and publishing are Craig-directed.

---

## Self-Review Notes (run before handing the plan to review)

1. **Spec coverage:** component 1 → Tasks 1–2 (scan, size gate, typed split, notes, degraded states, strict/tolerant policies); component 2 → Tasks 3–4 (preflight/delete occupancy, heal-arm race diagnosis); component 3 → Tasks 5–7 (F1, live root, mock spy, leftovers, checklist lines); component 4 → Task 8; component 5 → every task's tests + Task 9's brief. Execution-note ordering honoured (scan → policy → occupancy; debts interleave; docs after behaviour; release last).
2. **Known deliberate divergences to defend in review:** the `-1` unknown-size sentinel (Node.Size zero-value is a real 0-byte file, out-of-band anyway — the sentinel only exists for the wrapper-transport test; the CLI never reports negatives); `ScanRefs` hard rename (no compat wrapper — four call sites, YAGNI); occupancy checks live in the existing phase-2 loop rather than a separate phase (the invariant "refused before newShas/valid assignment ⇒ no pack" is the existing structure).
3. **Type consistency:** `SkippedRef`/`SkipKind`/`RefScan`/`ContentSkips`/`OccupancyMessage` (Task 1) are the single vocabulary consumed by Tasks 2, 3, 4, 8, 9; `errMalformedRef` + band constants shared by Tasks 1 and 4; `contractLiveRoot` (Task 6) consumed by Task 5's live row and Task 9's brief.

## Revisions

*Scaffolding for the review loop; delete before execution dispatch.*
