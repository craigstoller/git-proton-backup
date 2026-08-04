# Stage 3b — Selective Fetch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** An incremental `git fetch proton-v2` downloads only the pack(s) it actually needs — measured at the transport boundary, not assumed — with everything Stage 3a's gate proved still passing.

**Architecture:** A transport-boundary trace decorator makes every download observable. Fetch replaces `downloadAllPacks` with an iterative discovery loop: `rev-list --missing=print` names the missing frontier, an object-to-pack map (built each run by `git show-index` over cached `.idx` sidecars) resolves it to packs, and a deterministic greedy cover picks which to download. Every cache-suspect failure gets exactly one self-heal round (fresh listing, fresh sidecars, rebuilt map) before its fatal. Everything downstream of discovery — verify-before-install, `ConnectivityOK`, `consolidateAndInstall`, the single `lock` — is 3a's code, untouched.

**Tech Stack:** Go stdlib only (zero dependencies), git plumbing via `internal/gitcmd`, in-memory `transport.Fake` for hermetic tests, real Proton CLI behind `GPB_LIVE_ACCOUNT=1` only.

**Spec:** `docs/superpowers/specs/2026-08-03-v2-stage3b-selective-fetch-design.md` (normative for every rule below). Parent: `docs/v2-remote-helper-design.md` v6.3.

## Global Constraints

- Go stdlib only; `go.mod` gains no dependency.
- stdout is protocol-only; every diagnostic goes to stderr.
- Fail closed: anything unconfirmed is a failure. Cache trouble is the one deliberate exception — it degrades with a stderr warning, never fails a fetch.
- Fetch is strictly read-only on the remote: no `Bootstrap`, no lock. The presence short-circuit at the top of `Fetch` is load-bearing resume-safety — do not remove it.
- The `.idx` is NEVER checksummed against its basename (it borrows its pack's name). Checksum-vs-basename applies to the `.pack` alone; pair truth is `index-pack --verify`, only when both members are local.
- Remote name in docs/messages is `proton-v2`, never `proton`.
- Every commit message ends with the trailer: `Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>`
- Never `git push`. Work happens on branch `feat/v2-stage3b`.
- **Environment, every dispatch:** PowerShell needs `$env:Path = [Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [Environment]::GetEnvironmentVariable('Path','User')` prepended before any `go`/`git` command. `Set-Location` does not persist between tool calls — use absolute paths or `-C`. `gofmt -l` false-positives on CRLF-rewritten working-tree files; committed content is clean — never "fix" line endings.
- Every test is labelled RED (fails before its implementation) or GUARD (pins behaviour that already works). A RED must be observed failing before its fix; record which assertion fired.
- Live account: nothing in Tasks 1–7 touches it. Task 8 (gate) confines writes to `/my-files/GitRemotes/<demo>`; `GitBackups`, `Sensitive Project Sources`, `Project Repo Bundles`, `ChatGPT Export Text Backup` are untouchable. Gate runners report BLOCKED with verbatim output and never patch.

## File Structure

| File | Responsibility |
|---|---|
| `internal/transport/trace.go` (new) | `Traced` decorator: one stderr line per successful `ReadTo` |
| `internal/transport/trace_test.go` (new) | Decorator tests |
| `internal/gitcmd/gitcmd.go` (modify) | Add `ShowIndex`, `RevListMissing` + `parseMissingOIDs` |
| `internal/gitcmd/gitcmd_test.go` (modify) | Their tests, incl. probe-replicating GUARDs |
| `internal/repo/idxcache.go` (new) | `ResolveIdxCacheDir`, `EnsureSidecar`, `RefreshSidecar`, `pruneStale` |
| `internal/repo/packmap.go` (new) | `listCompletePacks`, `packMap`, `buildPackMap`, `greedyCover`, `errCacheSuspect` |
| `internal/repo/fetch.go` (modify) | Discovery loop replaces `downloadAllPacks`/`verifyDownloadedPacks`; new `cacheDir` param |
| `internal/repo/repo_test.go` (modify) | Updated call sites + new loop/heal tests |
| `cmd/git-remote-proton/main.go` (modify) | Wrap transport in `Traced` (Task 1); resolve cache dir in the fetch case (Task 6) |

---

### Task 1: Transport trace decorator, wired into main

**Files:**
- Create: `internal/transport/trace.go`
- Create: `internal/transport/trace_test.go`
- Modify: `cmd/git-remote-proton/main.go:68-85` (construction site)

**Interfaces:**
- Consumes: `transport.Transport` interface (`transport.go:47`), `transport.Node`, `transport.Outcome`, `*transport.Fake` (`fake.go`).
- Produces: `transport.NewTraced(inner Transport, w io.Writer) *Traced` implementing `Transport`. The stderr line format `gpb: downloaded <remote-path> (<n> bytes)\n` — the prefix `gpb: downloaded ` is NORMATIVE (the Task 8 gate parses it; hermetic tests in Tasks 6–7 parse it too).

- [ ] **Step 1: Write the failing tests (RED)**

Append to a new `internal/transport/trace_test.go`:

```go
package transport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RED: NewTraced does not exist. A successful ReadTo emits exactly one line
// with the normative prefix, the remote path, and the landed byte count.
func TestTracedReadToLogsOneLine(t *testing.T) {
	f := NewFake()
	f.Files["/r/packs/pack-abc.pack"] = []byte("hello")
	var buf strings.Builder
	tr := NewTraced(f, &buf)
	dir := t.TempDir()
	if err := tr.ReadTo("/r/packs/pack-abc.pack", dir); err != nil {
		t.Fatalf("ReadTo: %v", err)
	}
	want := "gpb: downloaded /r/packs/pack-abc.pack (5 bytes)\n"
	if buf.String() != want {
		t.Errorf("trace = %q, want %q", buf.String(), want)
	}
	if _, err := os.Stat(filepath.Join(dir, "pack-abc.pack")); err != nil {
		t.Errorf("delegation must still land the file: %v", err)
	}
}

// RED: a FAILED ReadTo must not log — the gate counts these lines as
// transfers, and a failed transfer is not a transfer.
func TestTracedReadToFailureLogsNothing(t *testing.T) {
	f := NewFake()
	var buf strings.Builder
	tr := NewTraced(f, &buf)
	if err := tr.ReadTo("/r/absent", t.TempDir()); err == nil {
		t.Fatal("ReadTo of a missing node must fail")
	}
	if buf.String() != "" {
		t.Errorf("failed ReadTo logged %q; want nothing", buf.String())
	}
}

// RED: every other method delegates verbatim and logs nothing.
func TestTracedDelegatesSilently(t *testing.T) {
	f := NewFake()
	var buf strings.Builder
	tr := NewTraced(f, &buf)
	if err := tr.EnsureDir("/r/d"); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.List("/r"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := tr.Stat("/r/d"); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(t.TempDir(), "leaf")
	if err := os.WriteFile(local, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.CreateExclusive("/r/leaf", local); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.UpdateRevision("/r/leaf", local); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.Trash("/r/leaf"); err != nil {
		t.Fatal(err)
	}
	if !f.Dirs["/r/d"] {
		t.Error("EnsureDir did not delegate")
	}
	if buf.String() != "" {
		t.Errorf("non-ReadTo methods logged %q; want nothing", buf.String())
	}
}
```

- [ ] **Step 2: Run the tests, confirm they fail with "undefined: NewTraced"**

```
go test ./internal/transport/ -run TestTraced -v
```

- [ ] **Step 3: Implement `internal/transport/trace.go`**

```go
package transport

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
)

// Traced decorates a Transport with one diagnostic line per successful
// ReadTo. It wraps the ONLY handle to the remote, so every download from any
// code path is counted — fetch orchestration cannot forget to log, and a
// future bug that adds a download shows up in the count. The Stage 3b gate
// measures selectivity by parsing these lines from git-fetch stderr; hermetic
// tests parse the same lines from a strings.Builder, so the two assert
// against identical output.
//
// The prefix "gpb: downloaded " is NORMATIVE (the gate greps for it). The
// size suffix is informative only.
type Traced struct {
	inner Transport
	w     io.Writer
}

func NewTraced(inner Transport, w io.Writer) *Traced { return &Traced{inner: inner, w: w} }

func (t *Traced) EnsureDir(p string) error                     { return t.inner.EnsureDir(p) }
func (t *Traced) List(p string) ([]Node, error)                { return t.inner.List(p) }
func (t *Traced) Stat(p string) (Node, bool, error)            { return t.inner.Stat(p) }
func (t *Traced) CreateExclusive(p, l string) (Outcome, error) { return t.inner.CreateExclusive(p, l) }
func (t *Traced) UpdateRevision(p, l string) (Outcome, error)  { return t.inner.UpdateRevision(p, l) }
func (t *Traced) Trash(p string) (Outcome, error)              { return t.inner.Trash(p) }

// ReadTo logs only on success: a failed transfer is not a transfer, and the
// gate counts these lines as transfers. The landed file's name is path.Base
// of the REMOTE path (POSIX, always), matching ReadTo's documented contract.
func (t *Traced) ReadTo(p, local string) error {
	if err := t.inner.ReadTo(p, local); err != nil {
		return err
	}
	if fi, err := os.Stat(filepath.Join(local, path.Base(p))); err == nil {
		fmt.Fprintf(t.w, "gpb: downloaded %s (%d bytes)\n", p, fi.Size())
	} else {
		fmt.Fprintf(t.w, "gpb: downloaded %s (size unknown)\n", p)
	}
	return nil
}
```

- [ ] **Step 4: Run the tests, confirm all three pass**

```
go test ./internal/transport/ -run TestTraced -v
```

- [ ] **Step 5: Wire into main.go**

At `cmd/git-remote-proton/main.go:68-85`, the current code constructs `t := transport.NewCLI("")`, runs the advisory version check on `t.Version()` (a `*CLI` method — the check must stay on the concrete type), and ends with `return loop(t, root, gitDir, in, out)`. Change ONLY the construction and the final call:

```go
	cli := transport.NewCLI("")
	// ... (version-check block unchanged, but reading cli.Version()) ...
	return loop(transport.NewTraced(cli, os.Stderr), root, gitDir, in, out)
```

`loop` already takes the `Transport` interface, so nothing else changes. Existing cmd tests construct their transports directly and are unaffected.

- [ ] **Step 6: Run the full suite**

```
go test ./...
```
Expected: all packages pass.

- [ ] **Step 7: Commit**

```
git add internal/transport/trace.go internal/transport/trace_test.go cmd/git-remote-proton/main.go
git commit -m "feat(v2): transport trace decorator - the 3b gate's measurement boundary"  (+ trailer per Global Constraints)
```

---

### Task 2: `gitcmd.ShowIndex`

**Files:**
- Modify: `internal/gitcmd/gitcmd.go` (append)
- Modify: `internal/gitcmd/gitcmd_test.go` (append)

**Interfaces:**
- Consumes: the package-private `waitDelay` var and the exec-site idioms already in `gitcmd.go` (separate stdout/stderr builders, `cmd.WaitDelay`, `exec.ErrWaitDelay` handling — see `PackObjectsFromList:302` for the pattern).
- Produces: `ShowIndex(idxPath string) ([]string, error)` — every object ID in the index, order unspecified. Task 5's `buildPackMap` consumes it.

- [ ] **Step 1: Write the failing tests**

The test needs a real repo and a real pack pair. `gitcmd_test.go` already contains helpers for making repos — reuse whatever exists there for creating a commit (grep for `exec.Command("git"` in that file and follow its idiom); if none fits, build inline as below.

```go
// makeIdxFixture builds a tiny repo and packs its full closure, returning the
// idx path and the object IDs the pack must contain. indexVersion is passed to
// index-pack when rebuilding the index, so the same fixture pins v1 and v2.
func makeIdxFixture(t *testing.T, indexVersion string) (idxPath string, oids map[string]bool) {
	t.Helper()
	d := t.TempDir()
	run := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", d}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-qb", "main")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(d, "f.txt"), []byte("show-index fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "f.txt")
	run("commit", "-qm", "c")
	sha := run("rev-parse", "HEAD")

	out := t.TempDir()
	packPath, gotIdx, err := WritePack(d, sha, nil, out)
	if err != nil || packPath == "" {
		t.Fatalf("WritePack: %v", err)
	}
	idxPath = gotIdx
	if indexVersion == "1" {
		// Rebuild the index as v1: Push deliberately accepts a valid remote v1
		// .idx it did not write, so the map builder must read it too.
		if err := os.Remove(idxPath); err != nil {
			t.Fatal(err)
		}
		if o, err := exec.Command("git", "index-pack", "--index-version=1", packPath).CombinedOutput(); err != nil {
			t.Fatalf("index-pack --index-version=1: %v: %s", err, o)
		}
	}
	oids = map[string]bool{}
	for _, line := range strings.Split(run("rev-list", "--objects", sha), "\n") {
		if f := strings.Fields(line); len(f) > 0 {
			oids[f[0]] = true
		}
	}
	return idxPath, oids
}

// RED: ShowIndex does not exist. It must return exactly the pack's objects.
func TestShowIndexReadsV2(t *testing.T) {
	idx, want := makeIdxFixture(t, "2")
	got, err := ShowIndex(idx)
	if err != nil {
		t.Fatalf("ShowIndex: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("ShowIndex returned %d oids, want %d", len(got), len(want))
	}
	for _, oid := range got {
		if !want[oid] {
			t.Errorf("unexpected oid %s", oid)
		}
	}
}

// RED (same run as above): the v1 GUARD from the spec — a v2-only reader
// would accept an index at push time it cannot fetch later.
func TestShowIndexReadsV1(t *testing.T) {
	idx, want := makeIdxFixture(t, "1")
	got, err := ShowIndex(idx)
	if err != nil {
		t.Fatalf("ShowIndex on a v1 index: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("ShowIndex returned %d oids from v1, want %d", len(got), len(want))
	}
}

// RED: a structurally corrupt index must be an error, never a short answer.
func TestShowIndexRejectsCorruptIndex(t *testing.T) {
	p := filepath.Join(t.TempDir(), "pack-0000000000000000000000000000000000000000.idx")
	if err := os.WriteFile(p, []byte("not an index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ShowIndex(p); err == nil {
		t.Fatal("ShowIndex must reject a corrupt index")
	}
}
```

- [ ] **Step 2: Run, confirm all three fail with "undefined: ShowIndex"**

```
go test ./internal/gitcmd/ -run TestShowIndex -v
```

- [ ] **Step 3: Implement in `gitcmd.go`**

```go
// ShowIndex returns every object ID recorded in the pack index at idxPath,
// via `git show-index` — git's own reader, which speaks index v1 AND v2.
// That is the parent design's trap closed the way it prescribes: Push
// deliberately accepts a valid remote v1 .idx it did not write, so a v2-only
// parser here would accept an index at push time it cannot fetch later.
//
// show-index reads the index on stdin and needs no repository; output lines
// are "<offset> <oid>" (v1) or "<offset> <oid> (<crc32>)" (v2), so the OID is
// always the second field. Any line that does not parse that way is a hard
// error — a truncated or garbled answer must never become a smaller map.
func ShowIndex(idxPath string) ([]string, error) {
	f, err := os.Open(idxPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cmd := exec.Command("git", "show-index")
	cmd.Stdin = f
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// stdout here IS the object list, so an abandoned pipe means a possibly
	// truncated list; fail closed like WritePack's rev-list guard.
	// See waitDelay: DO NOT DELETE as unnecessary.
	cmd.WaitDelay = waitDelay
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrWaitDelay) {
			return nil, fmt.Errorf("show-index output was abandoned after %s; refusing to "+
				"act on a possibly truncated object list", waitDelay)
		}
		return nil, fmt.Errorf("show-index %s: %s: %w", idxPath,
			strings.TrimSpace(stderr.String()), err)
	}
	var oids []string
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
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

Add near the top of the file (package scope):

```go
// oidRe validates a full SHA-1 object name. gitcmd is self-contained (it
// imports nothing from internal/repo), so it holds its own copy rather than
// sharing repo's shaRe. SHA-256 repositories are refused elsewhere by design.
var oidRe = regexp.MustCompile(`^[0-9a-f]{40}$`)
```

and `"regexp"` to the imports.

- [ ] **Step 4: Run, confirm all three pass; run the whole package**

```
go test ./internal/gitcmd/ -v
```

- [ ] **Step 5: Commit**

```
git add internal/gitcmd/gitcmd.go internal/gitcmd/gitcmd_test.go
git commit -m "feat(v2): gitcmd.ShowIndex - the object-to-pack map's reader, v1 and v2"  (+ trailer)
```

---

### Task 3: `gitcmd.RevListMissing`

**Files:**
- Modify: `internal/gitcmd/gitcmd.go` (append)
- Modify: `internal/gitcmd/gitcmd_test.go` (append)

**Interfaces:**
- Consumes: `revListWithAlt(gitDir, altObjects string, wants []string, args ...string)` (`gitcmd.go:377`) — the existing alternate-splicing rev-list runner with WaitDelay/ErrWaitDelay guards; `oidRe` from Task 2; `WritePack`, `PackObjectsFromList` for fixtures.
- Produces: `RevListMissing(gitDir, altObjects string, wants []string) ([]string, error)` — the missing frontier, deduplicated, order unspecified. `parseMissingOIDs(out string) ([]string, error)` package-private, unit-testable. Task 6's loop consumes `RevListMissing`.

These tests replicate probes 1–3 from the spec against the suite's git, turning the probed facts into GUARDs that fail if git's behaviour regresses.

- [ ] **Step 1: Write the failing tests**

**Do NOT define a two-commit fixture — `gitcmd_test.go` already has one.** `twoCommitRepo(t *testing.T) (dir, first, second string)` exists at `gitcmd_test.go:357`: commit 1 adds `a.txt`, commit 2 adds `b.txt` (distinguishing content, deliberately — see its comment about sha collisions). Reuse it; redeclaring it is a compile error. Note for the tests below: the blob added by the second commit is **`b.txt`**, and the second commit's pack-minus-first also carries the `a.txt` blob's ABSENCE (the tree references it, so a B-only alternate reports it missing too — assertions below use containment, never exact-set equality, for exactly this reason).

```go
// packInto packs exactly the objects listed in objs (newline-separated OIDs)
// from src into <altDir>/pack, giving the alternate git's own layout.
func packInto(t *testing.T, src, altDir, objs string) {
	t.Helper()
	packDir := filepath.Join(altDir, "pack")
	if err := os.MkdirAll(packDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := PackObjectsFromList(src, "", objs, filepath.Join(packDir, "pack")); err != nil {
		t.Fatalf("PackObjectsFromList: %v", err)
	}
}

// emptyDst creates a repo that genuinely lacks every fixture object.
func emptyDst(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	if out, err := exec.Command("git", "-C", d, "init", "-qb", "main").CombinedOutput(); err != nil {
		t.Fatalf("init: %v: %s", err, out)
	}
	return d
}

// RED (undefined function), then GUARD forever: probe 1 — a missing PARENT
// commit is reported as missing, not a fatal. This is the loop's normal
// driving case (pack N+1 holds the new commits, the parent lives in pack N).
func TestRevListMissingReportsMissingParent(t *testing.T) {
	src, a, b := twoCommitRepo(t)
	alt := t.TempDir()
	// B's closure minus A's = B-only pack, via git's own enumeration.
	out, err := exec.Command("git", "-C", src, "rev-list", "--objects", b, "^"+a).Output()
	if err != nil {
		t.Fatalf("rev-list fixture: %v", err)
	}
	packInto(t, src, alt, string(out))

	missing, err := RevListMissing(emptyDst(t), alt, []string{b})
	if err != nil {
		t.Fatalf("RevListMissing: %v", err)
	}
	found := false
	for _, oid := range missing {
		if oid == a {
			found = true
		}
	}
	if !found {
		t.Errorf("missing parent %s not reported; got %v", a, missing)
	}
}

// GUARD: probe 2 — a missing TIP is reported, not fatal. This is why the
// loop needs no special round 0. On a git older than the floor this test
// fails, which is exactly the signal the minimum-git decision row wants.
func TestRevListMissingReportsMissingTip(t *testing.T) {
	_, a, _ := twoCommitRepo(t)
	missing, err := RevListMissing(emptyDst(t), t.TempDir(), []string{a})
	if err != nil {
		t.Fatalf("RevListMissing on a missing tip: %v", err)
	}
	if len(missing) != 1 || missing[0] != a {
		t.Errorf("want exactly [%s], got %v", a, missing)
	}
}

// GUARD: probe 3 — a missing TREE conceals the blob beneath it. The frontier
// deepens round by round; missingness is discovered incrementally.
func TestRevListMissingFrontierDeepensThroughATree(t *testing.T) {
	src, _, b := twoCommitRepo(t)
	alt := t.TempDir()
	packInto(t, src, alt, b) // ONLY the commit object; its tree and blob absent
	treeOut, err := exec.Command("git", "-C", src, "rev-parse", b+"^{tree}").Output()
	if err != nil {
		t.Fatal(err)
	}
	tree := strings.TrimSpace(string(treeOut))

	missing, err := RevListMissing(emptyDst(t), alt, []string{b})
	if err != nil {
		t.Fatalf("RevListMissing: %v", err)
	}
	sawTree := false
	for _, oid := range missing {
		if oid == tree {
			sawTree = true
		}
	}
	if !sawTree {
		t.Errorf("the missing tree %s must be reported; got %v", tree, missing)
	}
	// The blob under the missing tree must NOT be reported yet — git cannot
	// enumerate entries of a tree it does not have. If this ever starts
	// failing, the loop still works (it would just converge in fewer rounds);
	// the GUARD exists so the change is noticed, not silently absorbed.
	blobOut, err := exec.Command("git", "-C", src, "rev-parse", b+":b.txt").Output()
	if err != nil {
		t.Fatal(err)
	}
	blob := strings.TrimSpace(string(blobOut))
	for _, oid := range missing {
		if oid == blob {
			t.Errorf("blob %s beneath the missing tree must not be visible yet", blob)
		}
	}
}

// RED: the parser is normative — first whitespace-delimited token after '?',
// 40-hex validated, never trimmed-and-trusted. Object lines and blank lines
// are ignored; a malformed ?-line is a hard error.
func TestParseMissingOIDs(t *testing.T) {
	const oid = "e1789a06e5b16e588eb820f292b123243b73fdb7"
	got, err := parseMissingOIDs("?" + oid + "\n" +
		oid + " some/path.txt\n" + // object line: ignored
		"?" + oid + " trailing/path\n" + // defensive: token before any suffix
		"\n")
	if err != nil {
		t.Fatalf("parseMissingOIDs: %v", err)
	}
	if len(got) != 1 || got[0] != oid {
		t.Errorf("want deduplicated [%s], got %v", oid, got)
	}
	if _, err := parseMissingOIDs("?nothex\n"); err == nil {
		t.Error("a malformed ?-line must be a hard error")
	}
}
```

- [ ] **Step 2: Run, confirm failures are "undefined: RevListMissing" / "undefined: parseMissingOIDs"**

```
go test ./internal/gitcmd/ -run 'TestRevListMissing|TestParseMissingOIDs' -v
```

- [ ] **Step 3: Implement in `gitcmd.go`**

```go
// RevListMissing returns the OIDs of objects reachable from wants but present
// neither in gitDir's store nor in altObjects — the missing frontier the
// selective-fetch loop resolves through the object-to-pack map. Deduplicated;
// empty means discovery is complete.
//
// Requires a git whose `rev-list --missing=print` reports missing tips and
// parents as ?-lines (the Stage 3b minimum-git floor; probed on 2.53, pinned
// by this package's tests). On an older git the traversal dies at the first
// missing tip; the error wrap below names the floor so the failure is a
// diagnosed refusal rather than git's bare fatal.
func RevListMissing(gitDir, altObjects string, wants []string) ([]string, error) {
	if len(wants) == 0 {
		return nil, nil
	}
	out, _, err := revListWithAlt(gitDir, altObjects, wants,
		"--objects", "--missing=print", "--stdin", "--not", "--all")
	if err != nil {
		return nil, fmt.Errorf("%w (note: selective fetch requires a git whose rev-list "+
			"--missing=print reports absent objects as ?-lines; a git older than that floor "+
			"dies here with its own 'bad object' error instead)", err)
	}
	return parseMissingOIDs(out)
}

// parseMissingOIDs extracts ?-prefixed OIDs from rev-list --missing=print
// output. Normative (spec, component 4): the OID is the first whitespace-
// delimited token after the '?', validated as 40-hex — never assumed
// path-free, even though the probed git prints bare OIDs — and a ?-line that
// does not yield one is a hard parse error, never skipped.
func parseMissingOIDs(out string) ([]string, error) {
	seen := map[string]bool{}
	var missing []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if !strings.HasPrefix(line, "?") {
			continue
		}
		fields := strings.Fields(line[1:])
		if len(fields) == 0 || !oidRe.MatchString(fields[0]) {
			return nil, fmt.Errorf("unparseable missing-object line %q from rev-list", line)
		}
		if !seen[fields[0]] {
			seen[fields[0]] = true
			missing = append(missing, fields[0])
		}
	}
	return missing, nil
}
```

- [ ] **Step 4: Run, confirm all pass; then the deliberate-regression check for the GUARD trio**

```
go test ./internal/gitcmd/ -v
```

For `TestRevListMissingReportsMissingParent`: temporarily change `"--missing=print"` to `"--missing=allow-any"` in `RevListMissing`, rerun, and confirm the test FAILS (no `?`-lines are emitted, so the parent is not reported). Revert. Record in the ledger that the guard was seen failing.

- [ ] **Step 5: Commit**

```
git add internal/gitcmd/gitcmd.go internal/gitcmd/gitcmd_test.go
git commit -m "feat(v2): gitcmd.RevListMissing - the discovery loop's frontier query"  (+ trailer)
```

---

### Task 4: The sidecar cache — `internal/repo/idxcache.go`

**Files:**
- Create: `internal/repo/idxcache.go`
- Modify: `internal/repo/repo_test.go` (append tests)

**Interfaces:**
- Consumes: `gitcmd.RevParse(gitDir string, args ...string) (string, int, error)`; `transport.Transport.ReadTo`; `emptyGitRepo(t)` test helper (`repo_test.go:1665`).
- Produces (all consumed by Tasks 5–6):
  - `ResolveIdxCacheDir(gitDir, root string) (string, error)` — creates and returns the per-repo cache dir; error means "no cache" (callers degrade to `""`).
  - `EnsureSidecar(t transport.Transport, root, cacheDir, fallbackDir, stem string) (path string, cached bool, err error)` — cached copy if present, else downloads; `err` only on download failure (cache failures degrade with a stderr warning).
  - `RefreshSidecar(t transport.Transport, root, cacheDir, fallbackDir, stem string) (string, error)` — unconditional fresh download, replacing the cached copy when possible.
  - `pruneStale(cacheDir string, keep map[string]bool)` — best-effort removal of entries whose stem is not in keep, plus leftover `.tmp-*` staging files.
  - Stems are pack basenames WITHOUT extension: `pack-<40hex>`. Remote sidecar path is `root + "/packs/" + stem + ".idx"`.

- [ ] **Step 1: Write the failing tests (all RED — the file does not exist)**

Append to `repo_test.go`:

```go
func TestResolveIdxCacheDirCreatesUnderCommonDir(t *testing.T) {
	d := emptyGitRepo(t)
	dir, err := ResolveIdxCacheDir(d, "/my-files/GitRemotes/demo")
	if err != nil {
		t.Fatalf("ResolveIdxCacheDir: %v", err)
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("cache dir must be absolute, got %s", dir)
	}
	// Under <repo>/.git/proton-v2/idx-cache/<key>: the common dir resolved
	// through git, not assumed.
	wantPrefix := filepath.Join(d, ".git", "proton-v2", "idx-cache")
	if !strings.HasPrefix(dir, wantPrefix) {
		t.Errorf("cache dir %s not under %s", dir, wantPrefix)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Errorf("cache dir must exist as a directory: %v", err)
	}
	// The breadcrumb records the plain remote path for humans.
	b, err := os.ReadFile(filepath.Join(dir, "remote"))
	if err != nil || !strings.Contains(string(b), "/my-files/GitRemotes/demo") {
		t.Errorf("breadcrumb missing or wrong: %q, %v", b, err)
	}
	// Two different remotes must get two different keys.
	dir2, err := ResolveIdxCacheDir(d, "/my-files/GitRemotes/other")
	if err != nil {
		t.Fatal(err)
	}
	if dir2 == dir {
		t.Error("distinct remote roots must map to distinct cache dirs")
	}
}

func TestEnsureSidecarDownloadsOnceThenHits(t *testing.T) {
	f := transport.NewFake()
	f.Files["/r/packs/pack-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.idx"] = []byte("idx-bytes")
	cache, fallback := t.TempDir(), t.TempDir()
	stem := "pack-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	p1, cached, err := EnsureSidecar(f, "/r", cache, fallback, stem)
	if err != nil || cached {
		t.Fatalf("first call: path=%s cached=%v err=%v; want fresh download", p1, cached, err)
	}
	b, err := os.ReadFile(p1)
	if err != nil || string(b) != "idx-bytes" {
		t.Fatalf("returned path must hold the sidecar bytes: %q %v", b, err)
	}
	// The cache must now hold a copy under the final name.
	if _, err := os.Stat(filepath.Join(cache, stem+".idx")); err != nil {
		t.Fatalf("cache install missing: %v", err)
	}
	// Second call: a hit, no download. Delete the remote copy to prove it.
	delete(f.Files, "/r/packs/"+stem+".idx")
	p2, cached, err := EnsureSidecar(f, "/r", cache, fallback, stem)
	if err != nil || !cached {
		t.Fatalf("second call must hit the cache: cached=%v err=%v", cached, err)
	}
	if b, err := os.ReadFile(p2); err != nil || string(b) != "idx-bytes" {
		t.Fatalf("cache hit returned wrong bytes: %q %v", b, err)
	}
}

// Cache trouble must never become fetch trouble: with an unusable cacheDir
// (a PATH THAT IS A FILE, cross-platform-reliably unusable), the sidecar
// still arrives via fallbackDir.
func TestEnsureSidecarDegradesWhenCacheUnusable(t *testing.T) {
	f := transport.NewFake()
	stem := "pack-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	f.Files["/r/packs/"+stem+".idx"] = []byte("idx2")
	notADir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, cached, err := EnsureSidecar(f, "/r", notADir, t.TempDir(), stem)
	if err != nil || cached {
		t.Fatalf("degraded call: %v", err)
	}
	if b, _ := os.ReadFile(p); string(b) != "idx2" {
		t.Errorf("fallback copy wrong: %q", b)
	}
}

// RefreshSidecar must return FRESH bytes even when a stale cached copy and a
// stale fallback copy both exist (the residue rule applies to sidecars too:
// ReadTo's overwrite behaviour is unpinned, so stale files are deleted first).
func TestRefreshSidecarReplacesStaleCopies(t *testing.T) {
	f := transport.NewFake()
	stem := "pack-cccccccccccccccccccccccccccccccccccccccc"
	f.Files["/r/packs/"+stem+".idx"] = []byte("fresh")
	cache, fallback := t.TempDir(), t.TempDir()
	for _, d := range []string{cache, fallback} {
		if err := os.WriteFile(filepath.Join(d, stem+".idx"), []byte("stale"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := RefreshSidecar(f, "/r", cache, fallback, stem)
	if err != nil {
		t.Fatalf("RefreshSidecar: %v", err)
	}
	if b, _ := os.ReadFile(p); string(b) != "fresh" {
		t.Errorf("refresh returned %q, want fresh bytes", b)
	}
	if b, _ := os.ReadFile(filepath.Join(cache, stem+".idx")); string(b) != "fresh" {
		t.Errorf("cache still holds %q after refresh", b)
	}
}

func TestPruneStaleRemovesOnlyVanishedStems(t *testing.T) {
	cache := t.TempDir()
	keepStem := "pack-dddddddddddddddddddddddddddddddddddddddd"
	goneStem := "pack-eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	for _, n := range []string{keepStem + ".idx", goneStem + ".idx", ".tmp-123", "remote"} {
		if err := os.WriteFile(filepath.Join(cache, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pruneStale(cache, map[string]bool{keepStem: true})
	if _, err := os.Stat(filepath.Join(cache, keepStem+".idx")); err != nil {
		t.Error("kept stem was pruned")
	}
	if _, err := os.Stat(filepath.Join(cache, goneStem+".idx")); !os.IsNotExist(err) {
		t.Error("vanished stem survived pruning")
	}
	if _, err := os.Stat(filepath.Join(cache, ".tmp-123")); !os.IsNotExist(err) {
		t.Error("staging leftover survived pruning")
	}
	if _, err := os.Stat(filepath.Join(cache, "remote")); err != nil {
		t.Error("the breadcrumb must never be pruned")
	}
}
```

- [ ] **Step 2: Run, confirm every test fails with undefined identifiers**

```
go test ./internal/repo/ -run 'TestResolveIdxCacheDir|TestEnsureSidecar|TestRefreshSidecar|TestPruneStale' -v
```

- [ ] **Step 3: Implement `internal/repo/idxcache.go`**

```go
package repo

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/gitcmd"
	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// ResolveIdxCacheDir returns (creating it if needed) the per-repo sidecar
// cache directory for the remote at root:
//
//	<git-common-dir>/proton-v2/idx-cache/<sha256-16 of root>/
//
// The common dir — NOT --git-path <name>, which resolves arbitrary names
// into the per-worktree ADMIN dir — so linked worktrees share one cache. The
// answer is validated before being trusted as a path for the same reason
// validateObjectsPackPath exists: RevParse's result is combined
// stdout+stderr, and a merged git warning must not become a directory name.
// It is then absolutised: --git-common-dir answers RELATIVE in an ordinary
// repo (".git"), relative to the -C directory it ran under, i.e. gitDir —
// the Stage 3a relative-GIT_DIR lesson applied before it can bite.
//
// An error return means "no cache this run"; callers pass cacheDir="" and
// every sidecar lives in the fetch's temp dir instead. The key is a hash so
// no character of the remote path ever reaches the filesystem; the "remote"
// breadcrumb file records the plain path for humans (best-effort).
func ResolveIdxCacheDir(gitDir, root string) (string, error) {
	out, code, err := gitcmd.RevParse(gitDir, "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("cannot resolve the git common dir for %s: %w", gitDir, err)
	}
	if code != 0 {
		return "", fmt.Errorf("rev-parse --git-common-dir exited %d: %s", code, out)
	}
	if out == "" || strings.ContainsAny(out, "\r\n") {
		return "", fmt.Errorf("rev-parse --git-common-dir returned %q, which cannot be "+
			"trusted as a path (a git warning may have been merged into the output)", out)
	}
	common := out
	if !filepath.IsAbs(common) {
		common = filepath.Join(gitDir, common)
	}
	abs, err := filepath.Abs(common)
	if err != nil {
		return "", fmt.Errorf("cannot absolutise common dir %q: %w", common, err)
	}
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(root)))[:16]
	dir := filepath.Join(abs, "proton-v2", "idx-cache", key)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	// Best-effort: a failed breadcrumb never fails the cache.
	_ = os.WriteFile(filepath.Join(dir, "remote"), []byte(root+"\n"), 0o644)
	return dir, nil
}

// EnsureSidecar returns a local path holding stem's .idx: the cached copy
// when present, else a fresh download into fallbackDir with a best-effort
// copy installed into the cache. err is non-nil ONLY for a failed download —
// cache trouble degrades with a stderr warning, never fails the caller
// (spec: "cache trouble is never allowed to become fetch trouble").
func EnsureSidecar(t transport.Transport, root, cacheDir, fallbackDir, stem string) (string, bool, error) {
	name := stem + ".idx"
	if cacheDir != "" {
		if p := filepath.Join(cacheDir, name); fileExists(p) {
			return p, true, nil
		}
	}
	p, err := downloadSidecar(t, root, fallbackDir, name)
	if err != nil {
		return "", false, err
	}
	installIntoCache(cacheDir, p, name)
	return p, false, nil
}

// RefreshSidecar unconditionally re-downloads stem's .idx, deleting any
// stale fallback copy first (ReadTo's behaviour onto an existing file is
// unpinned — C2's identical-content skip — so the residue rule applies to
// sidecars exactly as it does to packs) and replacing the cached copy when
// possible. The returned path is the guaranteed-fresh fallback copy.
func RefreshSidecar(t transport.Transport, root, cacheDir, fallbackDir, stem string) (string, error) {
	name := stem + ".idx"
	if err := os.Remove(filepath.Join(fallbackDir, name)); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	// Evict the stale cache entry BEFORE attempting the install: if the
	// install's rename then fails, the next run gets a cache MISS rather
	// than silently trusting the very bytes this refresh exists to replace.
	// Best-effort like every cache write — a failed evict only warns.
	if cacheDir != "" {
		if err := os.Remove(filepath.Join(cacheDir, name)); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "git-remote-proton: cannot evict stale cache entry %s: %v\n",
				name, err)
		}
	}
	p, err := downloadSidecar(t, root, fallbackDir, name)
	if err != nil {
		return "", err
	}
	installIntoCache(cacheDir, p, name)
	return p, nil
}

func downloadSidecar(t transport.Transport, root, destDir, name string) (string, error) {
	if err := t.ReadTo(root+"/packs/"+name, destDir); err != nil {
		return "", fmt.Errorf("cannot download %s: %w", name, err)
	}
	return filepath.Join(destDir, name), nil
}

// installIntoCache copies src into cacheDir under name, via temp-file-then-
// rename in the SAME directory so a crash can never leave a half-written
// sidecar under a valid name (Go's os.Rename replaces an existing file on
// Windows, so a concurrent loser is simply overwritten with identical-
// meaning bytes). Entirely best-effort: every failure warns and returns.
func installIntoCache(cacheDir, src, name string) {
	if cacheDir == "" {
		return
	}
	warn := func(err error) {
		fmt.Fprintf(os.Stderr, "git-remote-proton: sidecar cache write failed (%v); "+
			"continuing without caching %s\n", err, name)
	}
	in, err := os.Open(src)
	if err != nil {
		warn(err)
		return
	}
	defer in.Close()
	tmp, err := os.CreateTemp(cacheDir, ".tmp-*")
	if err != nil {
		warn(err)
		return
	}
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		warn(err)
		return
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		warn(err)
		return
	}
	if err := os.Rename(tmp.Name(), filepath.Join(cacheDir, name)); err != nil {
		_ = os.Remove(tmp.Name())
		warn(err)
	}
}

// pruneStale removes cache entries whose pack no longer appears in the
// remote listing, plus leftover staging files. Runs only on fetches that
// enter discovery (an up-to-date fetch short-circuits before listing).
// v2 never deletes a pack, so firing is defensive, not expected. Best-effort
// throughout: a prune failure warns and moves on.
func pruneStale(cacheDir string, keep map[string]bool) {
	if cacheDir == "" {
		return
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "git-remote-proton: cannot read sidecar cache for pruning: %v\n", err)
		return
	}
	for _, e := range entries {
		n := e.Name()
		stale := strings.HasPrefix(n, ".tmp-") ||
			(strings.HasSuffix(n, ".idx") && !keep[strings.TrimSuffix(n, ".idx")])
		if !stale {
			continue
		}
		if err := os.Remove(filepath.Join(cacheDir, n)); err != nil {
			fmt.Fprintf(os.Stderr, "git-remote-proton: cannot prune stale cache entry %s: %v\n", n, err)
		}
	}
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
```

- [ ] **Step 4: Run the new tests, confirm they pass; run the whole repo package**

```
go test ./internal/repo/ -v -run 'TestResolveIdxCacheDir|TestEnsureSidecar|TestRefreshSidecar|TestPruneStale'
go test ./internal/repo/
```

- [ ] **Step 5: Commit**

```
git add internal/repo/idxcache.go internal/repo/repo_test.go
git commit -m "feat(v2): per-repo sidecar cache - raw .idx files, degrade-never-fail"  (+ trailer)
```

---

### Task 5: Listing filter, object-to-pack map, greedy cover

**Files:**
- Create: `internal/repo/packmap.go`
- Modify: `internal/repo/repo_test.go` (append tests)

**Interfaces:**
- Consumes: `gitcmd.ShowIndex` (Task 2), `EnsureSidecar`/`RefreshSidecar` (Task 4), `transport.Transport.List`.
- Produces (Task 6 consumes all of these):
  - `var errCacheSuspect = errors.New(...)` — wrapped by every failure the self-heal round may cure; Task 6 branches on `errors.Is(err, errCacheSuspect)`.
  - `listCompletePacks(t transport.Transport, root string) ([]string, error)` — sorted stems of complete, grammar-valid pairs.
  - `type packMap struct { oidPacks map[string][]string; sidecars map[string]string; cacheDir, fallbackDir string }` with method `rebuildFromSidecars() error`.
  - `buildPackMap(t transport.Transport, root, cacheDir, fallbackDir string, stems []string) (*packMap, error)`.
  - `refreshPackMap(t transport.Transport, root, cacheDir, fallbackDir string, stems []string) (*packMap, error)` — fresh sidecars for every stem, then build.
  - `greedyCover(missing []string, oidPacks map[string][]string, downloaded map[string]bool) ([]string, error)` — pure; sorted result.

- [ ] **Step 1: Write the failing tests**

```go
// RED: only complete, grammar-valid pairs survive the listing. Names are
// remote-controlled input; nothing else may ever reach a filesystem join.
func TestListCompletePacksFiltersGrammarAndPairs(t *testing.T) {
	f := transport.NewFake()
	good := "pack-" + strings.Repeat("a", 40)
	orphanPack := "pack-" + strings.Repeat("b", 40)
	orphanIdx := "pack-" + strings.Repeat("c", 40)
	f.Files["/r/packs/"+good+".pack"] = []byte("p")
	f.Files["/r/packs/"+good+".idx"] = []byte("i")
	f.Files["/r/packs/"+orphanPack+".pack"] = []byte("p")      // no idx: in-flight push, skip
	f.Files["/r/packs/"+orphanIdx+".idx"] = []byte("i")        // no pack: unrepairable, skip
	f.Files["/r/packs/pack-NOTHEX.pack"] = []byte("x")         // grammar violation
	f.Files["/r/packs/stray.txt"] = []byte("x")                // stray node
	f.Dirs["/r/packs/subdir"] = true                           // directory

	stems, err := listCompletePacks(f, "/r")
	if err != nil {
		t.Fatalf("listCompletePacks: %v", err)
	}
	if len(stems) != 1 || stems[0] != good {
		t.Errorf("want exactly [%s], got %v", good, stems)
	}
}

// RED: the map is built by git's own reader over cached sidecars.
func TestBuildPackMapMapsOidsToStems(t *testing.T) {
	src := newGitRepoForPush(t)
	sha := headOfPushRepo(t, src)
	packPath, idxPath, err := gitcmd.WritePack(src, sha, nil, t.TempDir())
	if err != nil || packPath == "" {
		t.Fatalf("WritePack: %v", err)
	}
	stem := strings.TrimSuffix(filepath.Base(packPath), ".pack")
	f := transport.NewFake()
	for _, p := range []string{packPath, idxPath} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f.Files["/r/packs/"+filepath.Base(p)] = b
	}
	pm, err := buildPackMap(f, "/r", "", t.TempDir(), []string{stem})
	if err != nil {
		t.Fatalf("buildPackMap: %v", err)
	}
	packs := pm.oidPacks[sha]
	if len(packs) != 1 || packs[0] != stem {
		t.Errorf("commit %s must map to [%s], got %v", sha, stem, packs)
	}
}

// RED: a structurally corrupt CACHED sidecar self-heals as a cache miss; the
// map is built from the re-downloaded truth.
func TestBuildPackMapHealsACorruptCachedSidecar(t *testing.T) {
	src := newGitRepoForPush(t)
	sha := headOfPushRepo(t, src)
	packPath, idxPath, err := gitcmd.WritePack(src, sha, nil, t.TempDir())
	if err != nil || packPath == "" {
		t.Fatalf("WritePack: %v", err)
	}
	stem := strings.TrimSuffix(filepath.Base(packPath), ".pack")
	f := transport.NewFake()
	for _, p := range []string{packPath, idxPath} {
		b, _ := os.ReadFile(p)
		f.Files["/r/packs/"+filepath.Base(p)] = b
	}
	cache := t.TempDir()
	if err := os.WriteFile(filepath.Join(cache, stem+".idx"), []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	pm, err := buildPackMap(f, "/r", cache, t.TempDir(), []string{stem})
	if err != nil {
		t.Fatalf("buildPackMap must heal the corrupt cached sidecar: %v", err)
	}
	if len(pm.oidPacks[sha]) != 1 {
		t.Errorf("healed map must contain the commit")
	}
	// And the cache must now hold the good bytes.
	want, _ := os.ReadFile(idxPath)
	got, _ := os.ReadFile(filepath.Join(cache, stem+".idx"))
	if !bytes.Equal(want, got) {
		t.Error("cache must be repaired with the fresh sidecar")
	}
}

// RED: a corrupt sidecar FRESH from the remote is fatal naming the file.
func TestBuildPackMapFatalOnCorruptRemoteSidecar(t *testing.T) {
	stem := "pack-" + strings.Repeat("d", 40)
	f := transport.NewFake()
	f.Files["/r/packs/"+stem+".pack"] = []byte("p")
	f.Files["/r/packs/"+stem+".idx"] = []byte("not an index")
	_, err := buildPackMap(f, "/r", "", t.TempDir(), []string{stem})
	if err == nil || !strings.Contains(err.Error(), stem) {
		t.Fatalf("want a fatal naming %s, got %v", stem, err)
	}
}

// RED: greedy cover. Forced singles first, then most-covering, ties
// lexicographic; never a pack contributing nothing.
func TestGreedyCover(t *testing.T) {
	oidX, oidY, oidZ := strings.Repeat("1", 40), strings.Repeat("2", 40), strings.Repeat("3", 40)
	// x in {A,B}, y in {B}: B alone suffices (the round-1 review counterexample).
	m := map[string][]string{
		oidX: {"pack-A", "pack-B"},
		oidY: {"pack-B"},
	}
	got, err := greedyCover([]string{oidX, oidY}, m, map[string]bool{})
	if err != nil {
		t.Fatalf("greedyCover: %v", err)
	}
	if len(got) != 1 || got[0] != "pack-B" {
		t.Errorf("want [pack-B], got %v", got)
	}
	// Tie on coverage: lexicographically first wins, deterministically.
	m2 := map[string][]string{oidZ: {"pack-Q", "pack-P"}}
	got2, err := greedyCover([]string{oidZ}, m2, map[string]bool{})
	if err != nil || len(got2) != 1 || got2[0] != "pack-P" {
		t.Errorf("tie must break lexicographically: %v %v", got2, err)
	}
	// No candidate at all: errCacheSuspect, naming the OID.
	_, err = greedyCover([]string{oidZ}, map[string][]string{}, map[string]bool{})
	if err == nil || !errors.Is(err, errCacheSuspect) || !strings.Contains(err.Error(), oidZ) {
		t.Errorf("no-candidate must wrap errCacheSuspect and name the OID: %v", err)
	}
	// Still missing though its only pack was downloaded: the no-progress
	// signature, also errCacheSuspect. (This is the unit-level home of the
	// no-progress rule; end-to-end it is unreachable with self-consistent
	// remote pairs, since oid-in-idx implies oid-in-pack.)
	_, err = greedyCover([]string{oidZ}, map[string][]string{oidZ: {"pack-A"}},
		map[string]bool{"pack-A": true})
	if err == nil || !errors.Is(err, errCacheSuspect) {
		t.Errorf("no-progress must wrap errCacheSuspect: %v", err)
	}
	// A failure names EVERY offender, sorted — rev-list order is unspecified,
	// so a first-offender error would name a nondeterministic OID (and Task
	// 7's fatal-message assertions depend on this determinism).
	_, err = greedyCover([]string{oidY, oidX}, map[string][]string{}, map[string]bool{})
	if err == nil || !strings.Contains(err.Error(), oidX) || !strings.Contains(err.Error(), oidY) {
		t.Errorf("a no-candidate error must name all offenders: %v", err)
	}
}
```

Add `"bytes"` and `"errors"` to `repo_test.go` imports if absent.

- [ ] **Step 2: Run, confirm undefined-identifier failures**

```
go test ./internal/repo/ -run 'TestListCompletePacks|TestBuildPackMap|TestGreedyCover' -v
```

- [ ] **Step 3: Implement `internal/repo/packmap.go`**

```go
package repo

import (
	"errors"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/gitcmd"
	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// errCacheSuspect marks failures the self-heal round may cure: a lying or
// stale sidecar cache can present as "remote incomplete", as "no progress",
// or as a download/checksum failure on a pack a truthful map would never
// have selected. Fetch gives every such failure ONE fresh rebuild before its
// fatal; a fatal after healing genuinely indicts the remote or transport.
var errCacheSuspect = errors.New(
	"this can be caused by a stale or corrupt sidecar cache")

// packMemberRe is the normative name grammar. Listed names are remote-
// controlled input: nothing failing this pattern may ever be joined into a
// local path, become a cache key, or be downloaded.
var packMemberRe = regexp.MustCompile(`^pack-[0-9a-f]{40}\.(pack|idx)$`)

// listCompletePacks returns the sorted stems of every grammar-valid,
// COMPLETE pair in the remote listing. A .pack with no .idx is the normal
// signature of a concurrent or crashed push observed mid-publication (Push
// uploads the pack first) — skipped with a note, not an error. A .idx with
// no .pack is the unrepairable direction — silently never a download target.
func listCompletePacks(t transport.Transport, root string) ([]string, error) {
	nodes, err := t.List(root + "/packs")
	if err != nil {
		return nil, fmt.Errorf("cannot list %s/packs: %w", root, err)
	}
	members := map[string]map[string]bool{}
	for _, n := range nodes {
		if n.IsDir {
			continue
		}
		if !packMemberRe.MatchString(n.Name) {
			if strings.HasSuffix(n.Name, ".pack") || strings.HasSuffix(n.Name, ".idx") {
				fmt.Fprintf(os.Stderr, "git-remote-proton: ignoring %s/packs/%s: not a "+
					"valid pack member name\n", root, n.Name)
			}
			continue
		}
		ext := path.Ext(n.Name)
		stem := strings.TrimSuffix(n.Name, ext)
		if members[stem] == nil {
			members[stem] = map[string]bool{}
		}
		members[stem][ext] = true
	}
	var stems []string
	for stem, m := range members {
		switch {
		case m[".pack"] && m[".idx"]:
			stems = append(stems, stem)
		case m[".pack"]:
			fmt.Fprintf(os.Stderr, "git-remote-proton: %s/packs/%s.pack has no index yet "+
				"(a push may be in flight); skipping\n", root, stem)
		}
	}
	sort.Strings(stems)
	return stems, nil
}

// packMap is the in-memory object-to-pack map plus the local sidecar paths
// it was built from. It is rebuilt WHOLE whenever any sidecar is refreshed —
// per-entry surgery invites exactly the desynchronisation both round-1
// reviewers flagged, and rebuilding costs one show-index per pack.
type packMap struct {
	oidPacks    map[string][]string // oid -> sorted stems of packs holding it
	sidecars    map[string]string   // stem -> local path of its .idx
	cacheDir    string              // "" when the cache is unusable this run
	fallbackDir string
}

// buildPackMap ensures every stem's sidecar is locally readable and builds
// the map. A CACHED sidecar show-index rejects self-heals as a cache miss
// (discard, re-download once); a FRESH one that still fails is fatal naming
// the file — the remote's sidecar is bad.
func buildPackMap(t transport.Transport, root, cacheDir, fallbackDir string, stems []string) (*packMap, error) {
	pm := &packMap{
		sidecars:    map[string]string{},
		cacheDir:    cacheDir,
		fallbackDir: fallbackDir,
	}
	for _, stem := range stems {
		p, cached, err := EnsureSidecar(t, root, cacheDir, fallbackDir, stem)
		if err != nil {
			return nil, err
		}
		if _, err := gitcmd.ShowIndex(p); err != nil && cached {
			fmt.Fprintf(os.Stderr, "git-remote-proton: cached sidecar %s.idx is corrupt "+
				"(%v); re-downloading\n", stem, err)
			if p, err = RefreshSidecar(t, root, cacheDir, fallbackDir, stem); err != nil {
				return nil, err
			}
		}
		pm.sidecars[stem] = p
	}
	if err := pm.rebuildFromSidecars(); err != nil {
		return nil, err
	}
	return pm, nil
}

// refreshPackMap is the self-heal round's map: a fresh sidecar for every
// stem, then a full rebuild. After it, every map entry was derived from
// bytes downloaded THIS run.
func refreshPackMap(t transport.Transport, root, cacheDir, fallbackDir string, stems []string) (*packMap, error) {
	pm := &packMap{
		sidecars:    map[string]string{},
		cacheDir:    cacheDir,
		fallbackDir: fallbackDir,
	}
	for _, stem := range stems {
		p, err := RefreshSidecar(t, root, cacheDir, fallbackDir, stem)
		if err != nil {
			return nil, err
		}
		pm.sidecars[stem] = p
	}
	if err := pm.rebuildFromSidecars(); err != nil {
		return nil, err
	}
	return pm, nil
}

// rebuildFromSidecars rebuilds oidPacks from the current sidecar set. A
// failure here is fatal to the fetch: every sidecar in pm.sidecars has
// already had its one self-heal chance.
func (pm *packMap) rebuildFromSidecars() error {
	pm.oidPacks = map[string][]string{}
	for stem, p := range pm.sidecars {
		oids, err := gitcmd.ShowIndex(p)
		if err != nil {
			return fmt.Errorf("sidecar for %s is unreadable even freshly downloaded — "+
				"the remote's index is bad: %w", stem, err)
		}
		for _, oid := range oids {
			pm.oidPacks[oid] = append(pm.oidPacks[oid], stem)
		}
	}
	for oid := range pm.oidPacks {
		sort.Strings(pm.oidPacks[oid])
	}
	return nil
}

// greedyCover picks which packs to download for the missing frontier:
// packs that are some OID's ONLY un-downloaded candidate are forced first;
// remaining uncovered OIDs are covered greedily by the candidate holding
// most of them, ties broken lexicographically. Deterministic; never returns
// a pack contributing nothing to the frontier. Both failure modes wrap
// errCacheSuspect (see its doc).
func greedyCover(missing []string, oidPacks map[string][]string, downloaded map[string]bool) ([]string, error) {
	// Both failure scans run over the WHOLE frontier before erroring, and the
	// error names every offender, sorted: rev-list's output order is
	// unspecified, so erroring at the first offender would make the named OID
	// nondeterministic — and a diagnosis that names one of forty missing
	// objects at random is worse than one that names all forty.
	var noCandidate, noProgress []string
	uncovered := map[string]bool{}
	for _, oid := range missing {
		cands := oidPacks[oid]
		if len(cands) == 0 {
			noCandidate = append(noCandidate, oid)
			continue
		}
		fresh := 0
		for _, c := range cands {
			if !downloaded[c] {
				fresh++
			}
		}
		if fresh == 0 {
			noProgress = append(noProgress, oid)
			continue
		}
		uncovered[oid] = true
	}
	if len(noCandidate) > 0 {
		sort.Strings(noCandidate)
		return nil, fmt.Errorf("no pack on the remote contains missing object(s) %s: %w",
			strings.Join(noCandidate, ", "), errCacheSuspect)
	}
	if len(noProgress) > 0 {
		sort.Strings(noProgress)
		return nil, fmt.Errorf("object(s) %s are still missing although every pack claiming "+
			"to contain them was already downloaded and verified (no progress is possible): %w",
			strings.Join(noProgress, ", "), errCacheSuspect)
	}
	chosen := map[string]bool{}
	covered := func() {
		for oid := range uncovered {
			for _, c := range oidPacks[oid] {
				if chosen[c] {
					delete(uncovered, oid)
					break
				}
			}
		}
	}
	// Pass 1: forced singles.
	for oid := range uncovered {
		var fresh []string
		for _, c := range oidPacks[oid] {
			if !downloaded[c] {
				fresh = append(fresh, c)
			}
		}
		if len(fresh) == 1 {
			chosen[fresh[0]] = true
		}
	}
	covered()
	// Pass 2: greedy most-covering, ties lexicographic.
	for len(uncovered) > 0 {
		count := map[string]int{}
		for oid := range uncovered {
			for _, c := range oidPacks[oid] {
				if !downloaded[c] && !chosen[c] {
					count[c]++
				}
			}
		}
		best := ""
		for c, n := range count {
			if best == "" || n > count[best] || (n == count[best] && c < best) {
				best = c
			}
		}
		if best == "" {
			// Unreachable given the per-OID fresh check above — but the
			// defensive arm must FAIL CLOSED: a silent partial cover would
			// download too little and push the failure downstream to
			// ConnectivityOK with a worse diagnosis.
			return nil, fmt.Errorf("internal: greedy cover could not cover %d remaining "+
				"missing objects: %w", len(uncovered), errCacheSuspect)
		}
		chosen[best] = true
		covered()
	}
	out := make([]string, 0, len(chosen))
	for c := range chosen {
		out = append(out, c)
	}
	sort.Strings(out)
	return out, nil
}
```

- [ ] **Step 4: Run the new tests, confirm they pass; whole package**

```
go test ./internal/repo/
```

- [ ] **Step 5: Commit**

```
git add internal/repo/packmap.go internal/repo/repo_test.go
git commit -m "feat(v2): pack listing grammar, object-to-pack map, greedy cover"  (+ trailer)
```

---

### Task 6: The discovery loop — rewrite `Fetch`

**Files:**
- Modify: `internal/repo/fetch.go` (replace `downloadAllPacks` + `verifyDownloadedPacks` with the loop; new signature)
- Modify: `internal/repo/repo_test.go` (update every existing `Fetch(` call site; add loop tests + helpers)
- Modify: `cmd/git-remote-proton/main.go` (fetch case: resolve cache dir, pass it)

**Interfaces:**
- Consumes: everything Tasks 2–5 produced (`RevListMissing`, `ShowIndex` via `packMap`, `EnsureSidecar`/`RefreshSidecar`/`pruneStale`/`ResolveIdxCacheDir`, `listCompletePacks`, `buildPackMap`, `refreshPackMap`, `greedyCover`, `errCacheSuspect`); existing `packContentChecksum(path) (string, error)` and `packNameChecksum(base) string` (`push.go:577,600`); `gitcmd.IndexPackVerify`, `gitcmd.ConnectivityOK`, `gitcmd.RevListNewObjects`; `consolidateAndInstall` (unchanged).
- Produces: `Fetch(t transport.Transport, root, gitDir, cacheDir string, wants []string) (string, error)` — the ONLY signature change; `cacheDir==""` means no persistent cache (sidecars in the run's temp dir). Test helpers `plantIncrementalPacks` and `countPackDownloads` (Task 7 reuses both — exact signatures below).

- [ ] **Step 1: Extend the signature (body untouched) and update call sites mechanically**

First, in `fetch.go`, change ONLY the signature line to `func Fetch(t transport.Transport, root, gitDir, cacheDir string, wants []string) (string, error)` — with `_ = cacheDir` as the first body line and nothing else touched. This keeps the package compiling with the OLD download-everything behaviour, which Step 3 depends on: the RED must be OBSERVED failing against it before Step 4 replaces the body (the RED-before-fix discipline; a red that was never seen red proves nothing).

Then every existing call `Fetch(f, "/r", dst, wants...)` in `repo_test.go` gains `""` as the new fourth argument: `Fetch(f, "/r", dst, "", []string{sha})`. Grep for `Fetch(` in `repo_test.go` — the sites are in `TestFetchInstallsTheClosure`, `TestFetchIsIdempotentAndInstallsNothingWhenUpToDate`, `TestFetchRejectsACorruptPackAndInstallsNothing`, `TestFetchRefusesAnUnmarkedRemote`, `TestFetchIntoALinkedWorktreeInstallsWhereGitLooks`, `TestFetchWithARelativeGitDirInstallsCorrectly`. Do not change their assertions — with one narrow exception: if `TestFetchRejectsACorruptPackAndInstallsNothing` asserts on the old error MESSAGE text, update only the expected message (the new loop reports the corruption via the heal-then-fatal path, so the wording differs); its substantive assertions (error non-nil, nothing installed) stand unchanged.

In `cmd/git-remote-proton/main.go`, in the `case strings.HasPrefix(line, "fetch "):` block (main.go:392), immediately before the `repo.Fetch` call (main.go:427), insert:

```go
			cacheDir, cerr := repo.ResolveIdxCacheDir(gitDir, root)
			if cerr != nil {
				warn(fmt.Errorf("sidecar cache unavailable (%v); pack indexes will be "+
					"re-downloaded this fetch", cerr))
				cacheDir = ""
			}
```

and change the call to `repo.Fetch(t, root, gitDir, cacheDir, wants)`. (Resolved here, per fetch batch, rather than at startup: push-only sessions never pay for it, and `loop`'s signature stays put.)

- [ ] **Step 2: Write the new failing tests (RED) and helpers**

```go
// plantIncrementalPacks pushes src's history onto the Fake as N incremental
// packs — shas[i]'s pack contains its closure minus shas[i-1]'s — with the
// ref left at the LAST sha. Returns the stems in history order.
func plantIncrementalPacks(t *testing.T, f *transport.Fake, root, src string, shas []string) []string {
	t.Helper()
	if err := Bootstrap(f, root); err != nil {
		t.Fatal(err)
	}
	var stems []string
	for i, sha := range shas {
		var haves []string
		if i > 0 {
			haves = []string{shas[i-1]}
		}
		packPath, idxPath, err := gitcmd.WritePack(src, sha, haves, t.TempDir())
		if err != nil || packPath == "" {
			t.Fatalf("WritePack(%s): %v", sha, err)
		}
		for _, p := range []string{packPath, idxPath} {
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			f.Files[root+"/packs/"+filepath.Base(p)] = b
		}
		stems = append(stems, strings.TrimSuffix(filepath.Base(packPath), ".pack"))
	}
	if _, err := WriteRef(f, root, "refs/heads/main", shas[len(shas)-1], false); err != nil {
		t.Fatal(err)
	}
	return stems
}

// countPackDownloads parses trace output for downloads under <root>/packs/,
// returning per-extension counts and the downloaded names. This helper IS
// the measurement the gate uses; TestCountPackDownloads pins it.
func countPackDownloads(trace, root string) (packs, idxs int, names []string) {
	for _, line := range strings.Split(trace, "\n") {
		rest, ok := strings.CutPrefix(line, "gpb: downloaded ")
		if !ok {
			continue
		}
		p := strings.SplitN(rest, " (", 2)[0]
		if !strings.HasPrefix(p, root+"/packs/") {
			continue
		}
		name := strings.TrimPrefix(p, root+"/packs/")
		names = append(names, name)
		if strings.HasSuffix(name, ".pack") {
			packs++
		}
		if strings.HasSuffix(name, ".idx") {
			idxs++
		}
	}
	return
}

// GUARD on the measurement itself: the counter must distinguish, or every
// selectivity assertion in this suite and the live gate is theater.
func TestCountPackDownloads(t *testing.T) {
	trace := "gpb: downloaded /r/packs/pack-a.pack (5 bytes)\n" +
		"gpb: downloaded /r/packs/pack-a.idx (3 bytes)\n" +
		"gpb: downloaded /r/refs/heads/main (41 bytes)\n" + // ref reads excluded
		"noise\n"
	p, i, names := countPackDownloads(trace, "/r")
	if p != 1 || i != 1 || len(names) != 2 {
		t.Fatalf("p=%d i=%d names=%v", p, i, names)
	}
}

// GUARD, not RED — and the label is load-bearing: this test also passes
// against 3a's download-everything code (downloading every pack converges
// too), so it pins CONVERGENCE across a pack split, not selectivity. The
// loop's distinguishing observable is the trace-counted selectivity test
// below, whose deliberate-regression check (Step 6) is the proof it can
// fail. Convergence still needs its own pin: a discovery bug that fetches
// too little fails HERE first, with the clearest diagnosis.
func TestFetchDiscoversAcrossPacks(t *testing.T) {
	src := newGitRepoForPush(t)
	c1 := headOfPushRepo(t, src)
	c2 := commitOnPushRepo(t, src, "f2.txt", "two")
	f := transport.NewFake()
	plantIncrementalPacks(t, f, "/r", src, []string{c1, c2})

	dst := emptyGitRepo(t)
	keep, err := Fetch(f, "/r", dst, "", []string{c2})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if keep == "" {
		t.Fatal("an installing fetch must return a .keep")
	}
	for _, sha := range []string{c1, c2} {
		if !gitcmd.HasObject(dst, sha) {
			t.Errorf("object %s missing after fetch", sha)
		}
	}
}

// RED: selectivity, measured. dst already holds c1..c2 (via a first fetch);
// after c3 is pushed, the incremental fetch downloads EXACTLY the new pair.
// The full-fetch count beside it is the deliberate-regression twin: the
// measurement demonstrably registers every download, so had the incremental
// fetch over-downloaded, the ==1 assertion would have caught it.
func TestFetchDownloadsOnlyTheNeededPack(t *testing.T) {
	src := newGitRepoForPush(t)
	c1 := headOfPushRepo(t, src)
	c2 := commitOnPushRepo(t, src, "f2.txt", "two")
	f := transport.NewFake()
	stems := plantIncrementalPacks(t, f, "/r", src, []string{c1, c2})

	var trace strings.Builder
	tr := transport.NewTraced(f, &trace)
	dst := emptyGitRepo(t)
	cache := t.TempDir()
	if _, err := Fetch(tr, "/r", dst, cache, []string{c2}); err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	fullPacks, fullIdxs, _ := countPackDownloads(trace.String(), "/r")
	if fullPacks != 2 || fullIdxs != 2 {
		t.Fatalf("full fetch must download both pairs (twin measurement): packs=%d idxs=%d",
			fullPacks, fullIdxs)
	}

	// git updates refs AFTER the helper exits; simulate before the increment.
	run := func(args ...string) {
		if out, err := exec.Command("git", append([]string{"-C", dst}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("update-ref", "refs/heads/main", c2)

	c3 := commitOnPushRepo(t, src, "f3.txt", "three")
	newStems := plantOneMorePack(t, f, "/r", src, c2, c3)
	trace.Reset()
	if _, err := Fetch(tr, "/r", dst, cache, []string{c3}); err != nil {
		t.Fatalf("incremental fetch: %v", err)
	}
	packs, idxs, names := countPackDownloads(trace.String(), "/r")
	if packs != 1 || idxs != 1 {
		t.Errorf("incremental fetch must download exactly one pair, got packs=%d idxs=%d (%v)",
			packs, idxs, names)
	}
	for _, n := range names {
		if !strings.HasPrefix(n, newStems[0]) {
			t.Errorf("downloaded %s; only %s.* was needed", n, newStems[0])
		}
	}
	_ = stems
}

// plantOneMorePack adds ONE more incremental pack (tip..prev) to an
// already-planted Fake and moves the ref. Separate from plantIncrementalPacks
// because Bootstrap and the first WriteRef must not rerun.
func plantOneMorePack(t *testing.T, f *transport.Fake, root, src, prev, tip string) []string {
	t.Helper()
	packPath, idxPath, err := gitcmd.WritePack(src, tip, []string{prev}, t.TempDir())
	if err != nil || packPath == "" {
		t.Fatalf("WritePack: %v", err)
	}
	for _, p := range []string{packPath, idxPath} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f.Files[root+"/packs/"+filepath.Base(p)] = b
	}
	if _, err := WriteRef(f, root, "refs/heads/main", tip, true); err != nil {
		t.Fatal(err)
	}
	return []string{strings.TrimSuffix(filepath.Base(packPath), ".pack")}
}

// GUARD, not RED, same reasoning as TestFetchDiscoversAcrossPacks: probe 3
// end-to-end — a frontier that deepens through a missing tree converges
// (commit pack, then tree pack, then blob pack). Download-everything also
// passes this; the pin is that multi-round discovery CONVERGES, which is
// exactly what breaks if the loop's termination or restart logic is wrong.
func TestFetchFrontierDeepensThroughHandBuiltPacks(t *testing.T) {
	src := newGitRepoForPush(t)
	commitOnPushRepo(t, src, "deep.txt", "payload")
	sha := headOfPushRepo(t, src)
	out := func(args ...string) string {
		b, err := exec.Command("git", append([]string{"-C", src}, args...)...).Output()
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return strings.TrimSpace(string(b))
	}
	tree := out("rev-parse", sha+"^{tree}")
	blobList := out("rev-list", "--objects", sha)
	f := transport.NewFake()
	if err := Bootstrap(f, "/r"); err != nil {
		t.Fatal(err)
	}
	// One hand-built pack per object CLASS: commit alone, tree(s) alone,
	// blob(s) alone — forcing one discovery round per depth level.
	classes := [][]string{{sha}, nil, nil}
	for _, line := range strings.Split(blobList, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] == sha {
			continue
		}
		if fields[0] == tree || strings.Contains(line, "/") || len(fields) == 1 {
			classes[1] = append(classes[1], fields[0]) // trees (root tree has no path)
		} else {
			classes[2] = append(classes[2], fields[0]) // pathed blobs
		}
	}
	for _, objs := range classes {
		if len(objs) == 0 {
			continue
		}
		dir := t.TempDir()
		name, err := gitcmd.PackObjectsFromList(src, "", strings.Join(objs, "\n"),
			filepath.Join(dir, "pack"))
		if err != nil {
			t.Fatalf("PackObjectsFromList: %v", err)
		}
		for _, ext := range []string{".pack", ".idx"} {
			b, err := os.ReadFile(filepath.Join(dir, "pack-"+name+ext))
			if err != nil {
				t.Fatal(err)
			}
			f.Files["/r/packs/pack-"+name+ext] = b
		}
	}
	if _, err := WriteRef(f, "/r", "refs/heads/main", sha, false); err != nil {
		t.Fatal(err)
	}
	dst := emptyGitRepo(t)
	if _, err := Fetch(f, "/r", dst, "", []string{sha}); err != nil {
		t.Fatalf("Fetch across a deepening frontier: %v", err)
	}
	if !gitcmd.HasObject(dst, sha) {
		t.Error("commit missing after multi-round discovery")
	}
	if out, err := exec.Command("git", "-C", dst, "fsck", "--no-dangling").CombinedOutput(); err != nil {
		t.Errorf("fsck after multi-round fetch: %v: %s", err, out)
	}
}
```

Note on `TestFetchFrontierDeepensThroughHandBuiltPacks`'s classification loop: the root tree's rev-list line is the bare tree OID with an EMPTY path (a trailing space) so it lands via `len(fields) == 1`, and any PARENT commits from `newGitRepoForPush`'s earlier history land there too (commits also print bare). The heuristic intentionally over-approximates "tree-ish": the test's purpose is only that objects land in multiple packs split by graph depth, forcing multiple discovery rounds — its assertions do not depend on perfect classification, only on convergence and a clean fsck.

- [ ] **Step 3: Run new tests, confirm the failures are the EXPECTED ones**

```
go test ./internal/repo/ -run 'TestFetch|TestCountPackDownloads' -v
```

Expected, precisely: compile errors first (undefined helpers; any old-signature call site you missed surfaces here). Once compiling — against the still-download-everything implementation — the ONE genuine RED is `TestFetchDownloadsOnlyTheNeededPack` (its `packs != 1` assertion fires: download-everything transfers every pair). `TestFetchDiscoversAcrossPacks` and `TestFetchFrontierDeepensThroughHandBuiltPacks` are GUARDs and may already pass; `TestCountPackDownloads` passes as soon as the helper exists. Record which assertion fired for the RED.

- [ ] **Step 4: Rewrite `fetch.go`**

Delete `downloadAllPacks` and `verifyDownloadedPacks` (their invariants move into the loop). Keep `Fetch`'s head (marker check, empty-wants, presence short-circuit), tail (`ConnectivityOK`, `RevListNewObjects`, `consolidateAndInstall`), and every comment they carry. The new middle:

> **SUPERSEDED (Task 6 review, F1):** the comment block below REPLACED Fetch's
> godoc as written, losing the read-only invariant ("READ-ONLY on the remote: no
> Bootstrap, no lock... must never be able to bring a repository into existence"),
> the return-value description, and the `<objects>/pack/` layout comment at the
> packDir site. Those paragraphs (recoverable via `git show e0bdff9:internal/repo/fetch.go`)
> must be KEPT alongside the new note, not replaced by it. Fixed in the Task 6
> fix round; a re-run must not reintroduce the deletion.

```go
// Fetch signature gains cacheDir: "" means no persistent sidecar cache
// (every sidecar lives in this run's temp dir). Everything below the
// discovery loop — verify-before-install, consolidation, the single .keep —
// is Stage 3a's code, untouched.
func Fetch(t transport.Transport, root, gitDir, cacheDir string, wants []string) (string, error) {
	if err := RequireMarker(t, root); err != nil {
		return "", err
	}
	if len(wants) == 0 {
		return "", nil
	}
	// (presence short-circuit block: UNCHANGED, comment and all)
	if err := gitcmd.ConnectivityOK(gitDir, "", wants); err == nil {
		return "", nil
	}

	tmp, err := os.MkdirTemp("", "gpb-fetch-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	altObjects := filepath.Join(tmp, "objects")
	packDir := filepath.Join(altObjects, "pack")
	fallbackDir := filepath.Join(tmp, "idx")
	for _, d := range []string{packDir, fallbackDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return "", err
		}
	}

	stems, err := listCompletePacks(t, root)
	if err != nil {
		return "", err
	}
	// A marked repo always has at least one pack (a ref cannot be published
	// without one), so an empty listing means the remote is incomplete — and
	// reaching this line means the local store lacks wanted objects, so
	// "up to date" would be a lie. Same invariant as 3a's downloadAllPacks.
	if len(stems) == 0 {
		return "", fmt.Errorf("%s/packs holds no complete pack pairs; the remote is incomplete", root)
	}
	pruneStale(cacheDir, stemSet(stems))
	pm, err := buildPackMap(t, root, cacheDir, fallbackDir, stems)
	if err != nil {
		return "", err
	}

	downloaded := map[string]bool{}
	healed := false
	// heal is the ONE self-heal round: fresh listing, fresh sidecars, whole
	// map rebuilt — after it, every terminal diagnosis rests on metadata
	// downloaded this run. The round restarts automatically because all
	// planning happens at the loop top.
	heal := func() error {
		var herr error
		if stems, herr = listCompletePacks(t, root); herr != nil {
			return herr
		}
		if len(stems) == 0 {
			return fmt.Errorf("%s/packs holds no complete pack pairs; the remote is incomplete", root)
		}
		pm, herr = refreshPackMap(t, root, cacheDir, fallbackDir, stems)
		return herr
	}
	fatalAfterHeal := func(err error) error {
		return fmt.Errorf("%w (the sidecar metadata was already refreshed from the remote "+
			"this run, so this indicates genuine remote trouble, not a stale cache)", err)
	}

	for {
		missing, err := gitcmd.RevListMissing(gitDir, altObjects, wants)
		if err != nil {
			return "", err
		}
		if len(missing) == 0 {
			break // discovery complete
		}
		toGet, err := greedyCover(missing, pm.oidPacks, downloaded)
		if err != nil { // always errCacheSuspect (see greedyCover)
			if healed {
				return "", fatalAfterHeal(err)
			}
			healed = true
			if err := heal(); err != nil {
				return "", err
			}
			continue
		}
		for _, stem := range toGet {
			refreshed, err := downloadAndVerifyPack(t, root, packDir, stem, pm)
			if err != nil {
				if !errors.Is(err, errCacheSuspect) {
					return "", err // pair-corrupt fatal, or a non-heal-able failure
				}
				if healed {
					return "", fatalAfterHeal(err)
				}
				healed = true
				if err := heal(); err != nil {
					return "", err
				}
				break // restart the round: the plan predates the rebuild
			}
			downloaded[stem] = true
			if refreshed {
				// A pair-verify sidecar refresh rebuilt the map mid-round; the
				// rest of toGet was planned on the old bytes. Restart planning.
				break
			}
		}
	}

	// (UNCHANGED from here: ConnectivityOK before install, RevListNewObjects,
	// consolidateAndInstall — comments and all.)
	...
}

func stemSet(stems []string) map[string]bool {
	m := make(map[string]bool, len(stems))
	for _, s := range stems {
		m[s] = true
	}
	return m
}

// downloadAndVerifyPack downloads one pack, checksums it against its own
// basename (the .pack ALONE — the .idx borrows the pack's name, so that
// comparison could never pass), lays the sidecar beside it (git discovers
// packs in an alternate only via the .idx), and runs pair verification.
//
// refreshed reports that the sidecar was re-downloaded and the map rebuilt —
// the caller must restart its round planning.
//
// RESIDUE RULE (spec, round 3): every failure path deletes what it wrote
// before returning. ReadTo's behaviour onto an existing file is deliberately
// unpinned (C2's identical-content skip makes reuse of stale bytes a real
// hazard), and a healed plan may legitimately re-select this same pack, so
// retry correctness must never depend on overwrite semantics. The remove
// BEFORE downloading covers residue from an attempt this process lost track
// of; the removes on each failure path cover this attempt's own leavings.
func downloadAndVerifyPack(t transport.Transport, root, packDir, stem string, pm *packMap) (bool, error) {
	packName := stem + ".pack"
	packPath := filepath.Join(packDir, packName)
	if err := os.Remove(packPath); err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if err := t.ReadTo(root+"/packs/"+packName, packDir); err != nil {
		_ = os.Remove(packPath)
		return false, fmt.Errorf("cannot download %s: %v — a truthful map might never have "+
			"selected this pack: %w", packName, err, errCacheSuspect)
	}
	got, err := packContentChecksum(packPath)
	if err != nil {
		_ = os.Remove(packPath)
		return false, fmt.Errorf("cannot checksum downloaded pack %s: %v: %w",
			packName, err, errCacheSuspect)
	}
	if want := packNameChecksum(packName); got != want {
		_ = os.Remove(packPath)
		return false, fmt.Errorf("downloaded pack %s recomputes to %s; the name is the "+
			"content checksum, so this file is not what its name claims: %w",
			packName, got, errCacheSuspect)
	}
	idxPath := filepath.Join(packDir, stem+".idx")
	if err := copyFile(pm.sidecars[stem], idxPath); err != nil {
		// The SOURCE here may be a cache path, and an unreadable cached
		// sidecar is cache-read trouble — heal-able, not fatal (spec: any
		// cache I/O failure degrades). Wrapping unconditionally is safe: a
		// genuine temp-dir failure wastes one heal round and then fatals.
		return false, fmt.Errorf("cannot lay sidecar beside %s: %v: %w",
			packName, err, errCacheSuspect)
	}
	if err := gitcmd.IndexPackVerify(packPath); err == nil {
		return false, nil
	}
	// Pair failed. The pack proved it matches its NAME, not that it is well
	// formed — so the cached sidecar is the CHEAPER suspect, not the proven
	// one. One fresh sidecar, map rebuilt, one re-verify.
	fresh, rerr := RefreshSidecar(t, root, pm.cacheDir, pm.fallbackDir, stem)
	if rerr != nil {
		return false, rerr
	}
	pm.sidecars[stem] = fresh
	if err := pm.rebuildFromSidecars(); err != nil {
		return false, err
	}
	if err := os.Remove(idxPath); err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if err := copyFile(fresh, idxPath); err != nil {
		return false, err
	}
	if err := gitcmd.IndexPackVerify(packPath); err != nil {
		return false, fmt.Errorf("pack pair %s.{pack,idx} fails verification even with a "+
			"freshly downloaded index; the pair is corrupt, member undetermined: %w", stem, err)
	}
	return true, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
```

Add `"errors"` and `"io"` to fetch.go's imports.

- [ ] **Step 5: Run the full repo package, then everything**

```
go test ./internal/repo/ -v -run TestFetch
go test ./...
```

Expected: all pass, including every pre-existing 3a fetch test (unchanged assertions — the baseline preserved).

- [ ] **Step 6: Deliberate-regression check on selectivity**

Temporarily make `greedyCover` return ALL stems (ignore its algorithm; `return sortedAllStems, nil`). Run `TestFetchDownloadsOnlyTheNeededPack`; confirm it FAILS on the `packs != 1` assertion. Revert. Record in the ledger which assertion fired — this is the hermetic proof the measurement catches over-fetching.

- [ ] **Step 7: Commit**

```
git add internal/repo/fetch.go internal/repo/repo_test.go cmd/git-remote-proton/main.go
git commit -m "feat(v2): selective fetch - discovery loop replaces download-everything"  (+ trailer)
```

---

### Task 7: Heal paths, residue, degradation — the failure-mode suite

**Files:**
- Modify: `internal/repo/repo_test.go` (append)
- Modify: `internal/repo/fetch.go` / `packmap.go` / `idxcache.go` only if a test exposes a defect

**Interfaces:**
- Consumes: `plantIncrementalPacks`, `plantOneMorePack`, `countPackDownloads` (Task 6 — exact signatures in that task), `Fetch(t, root, gitDir, cacheDir string, wants []string)`, `transport.NewTraced`, `errCacheSuspect`, and the PRE-EXISTING test helper `countInstalledPackFiles(t *testing.T, gitDir string) int` (`repo_test.go:2038` — counts `.pack` files in the repo's real object store).
- Produces: nothing new — this task exists so a reviewer can reject the failure-mode coverage independently of the happy path.

- [ ] **Step 1: Write the tests (all RED until proven otherwise — run each against the Task 6 code and record which pass already; any that passes immediately is relabelled GUARD in its comment)**

```go
// A parseable-but-LYING cached sidecar (valid idx bytes filed under the
// wrong stem) misroutes discovery; the self-heal round must fix it and the
// fetch must complete. This is the spec's "correctness never depends on the
// cache being right" promise.
func TestFetchSelfHealsALyingCache(t *testing.T) {
	src := newGitRepoForPush(t)
	c1 := headOfPushRepo(t, src)
	c2 := commitOnPushRepo(t, src, "f2.txt", "two")
	f := transport.NewFake()
	stems := plantIncrementalPacks(t, f, "/r", src, []string{c1, c2})

	// Poison: both cache entries hold pack-1's idx bytes, so c2's objects
	// appear to live nowhere.
	cache := t.TempDir()
	idx1 := f.Files["/r/packs/"+stems[0]+".idx"]
	for _, stem := range stems {
		if err := os.WriteFile(filepath.Join(cache, stem+".idx"), idx1, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dst := emptyGitRepo(t)
	if _, err := Fetch(f, "/r", dst, cache, []string{c2}); err != nil {
		t.Fatalf("Fetch must self-heal a lying cache: %v", err)
	}
	if !gitcmd.HasObject(dst, c2) {
		t.Error("objects missing after healed fetch")
	}
}

// After the heal, the same diagnosis is genuine: an object the remote truly
// does not hold is fatal, names the OID, and says the cache was already
// refreshed (so the message never advises cache-clearing).
func TestFetchFatalAfterHealNamesTheOidAndTheRefresh(t *testing.T) {
	src := newGitRepoForPush(t)
	c1 := headOfPushRepo(t, src)
	c2 := commitOnPushRepo(t, src, "f2.txt", "two")
	f := transport.NewFake()
	stems := plantIncrementalPacks(t, f, "/r", src, []string{c1, c2})
	// Remove pack 1 entirely: c1's closure is genuinely gone from the remote.
	delete(f.Files, "/r/packs/"+stems[0]+".pack")
	delete(f.Files, "/r/packs/"+stems[0]+".idx")

	dst := emptyGitRepo(t)
	_, err := Fetch(f, "/r", dst, t.TempDir(), []string{c2})
	if err == nil {
		t.Fatal("a remote missing part of the closure must be fatal")
	}
	if !strings.Contains(err.Error(), c1) {
		t.Errorf("fatal must name the missing OID %s: %v", c1, err)
	}
	if !strings.Contains(err.Error(), "already refreshed") {
		t.Errorf("fatal must state the metadata was already refreshed: %v", err)
	}
	// Nothing installed: the 3a posture holds.
	if n := countInstalledPackFiles(t, dst); n != 0 {
		t.Errorf("failed fetch must leave the local store untouched; %d packs installed", n)
	}
}

// residueTransport fails the FIRST download of target, leaving partial bytes
// at the destination — then, on the retry, REFUSES if those bytes are still
// there. This pins the residue rule directly: retry correctness must not
// depend on ReadTo's unpinned overwrite behaviour.
type residueTransport struct {
	*transport.Fake
	target   string
	tripped  bool
	sawStale bool
}

func (r *residueTransport) ReadTo(p, local string) error {
	if p == r.target {
		dest := filepath.Join(local, path.Base(p))
		if !r.tripped {
			r.tripped = true
			_ = os.WriteFile(dest, []byte("partial residue"), 0o644)
			return fmt.Errorf("injected transient failure downloading %s", p)
		}
		if _, err := os.Stat(dest); err == nil {
			r.sawStale = true
			return fmt.Errorf("retry found residue at %s; the residue rule is violated", dest)
		}
	}
	return r.Fake.ReadTo(p, local)
}

func TestFetchRetriesSamePackWithoutResidue(t *testing.T) {
	src := newGitRepoForPush(t)
	c1 := headOfPushRepo(t, src)
	f := transport.NewFake()
	stems := plantIncrementalPacks(t, f, "/r", src, []string{c1})
	rt := &residueTransport{Fake: f, target: "/r/packs/" + stems[0] + ".pack"}

	dst := emptyGitRepo(t)
	if _, err := Fetch(rt, "/r", dst, "", []string{c1}); err != nil {
		t.Fatalf("Fetch must survive one transient pack-download failure via the heal "+
			"round (sawStale=%v): %v", rt.sawStale, err)
	}
	if rt.sawStale {
		t.Error("the retry found the failed attempt's residue; it must have been deleted")
	}
	if !gitcmd.HasObject(dst, c1) {
		t.Error("objects missing after retried fetch")
	}
}

// A corrupt remote PACK selected because of a lying cache: checksum fails,
// heal reroutes to the honest pack, fetch completes WITHOUT the corrupt one.
func TestFetchChecksumFailureHealsAndRoutesAround(t *testing.T) {
	src := newGitRepoForPush(t)
	c1 := headOfPushRepo(t, src)
	c2 := commitOnPushRepo(t, src, "f2.txt", "two")
	c3 := commitOnPushRepo(t, src, "f3.txt", "three")
	f := transport.NewFake()
	stems := plantIncrementalPacks(t, f, "/r", src, []string{c1, c2, c3})

	// dst already holds c1..c2: fetch once with the ref at c2, then advance.
	dst := emptyGitRepo(t)
	if _, err := Fetch(f, "/r", dst, "", []string{c2}); err != nil {
		t.Fatalf("staging fetch: %v", err)
	}
	if out, err := exec.Command("git", "-C", dst, "update-ref", "refs/heads/main", c2).CombinedOutput(); err != nil {
		t.Fatalf("update-ref: %v: %s", err, out)
	}

	// Corrupt pack 2's remote bytes (now unneeded by dst), and poison the
	// cache so c3's missing objects appear to live in pack 2: cache holds
	// pack-3's idx bytes under pack-2's stem, and pack-2's under pack-3's.
	cache := t.TempDir()
	idx2 := f.Files["/r/packs/"+stems[1]+".idx"]
	idx3 := f.Files["/r/packs/"+stems[2]+".idx"]
	if err := os.WriteFile(filepath.Join(cache, stems[1]+".idx"), idx3, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, stems[2]+".idx"), idx2, 0o644); err != nil {
		t.Fatal(err)
	}
	pack2 := f.Files["/r/packs/"+stems[1]+".pack"]
	corrupt := append([]byte{}, pack2...)
	corrupt[len(corrupt)/2] ^= 0xff
	f.Files["/r/packs/"+stems[1]+".pack"] = corrupt

	var trace strings.Builder
	tr := transport.NewTraced(f, &trace)
	if _, err := Fetch(tr, "/r", dst, cache, []string{c3}); err != nil {
		t.Fatalf("Fetch must heal past the checksum failure: %v", err)
	}
	if !gitcmd.HasObject(dst, c3) {
		t.Error("c3 missing after healed fetch")
	}
}

// altPackingIdx builds a SECOND packing of sha's full closure at store-only
// compression: same OIDs, different bytes and name. Its idx is the perfect
// lie — a map built from it is RIGHT about which objects the stem serves,
// so greedy still selects the pack and the failure surfaces at PAIR
// verification, not earlier as a no-candidate heal. (A sidecar with the
// WRONG oids would divert into the heal path before any pack downloads —
// the trap this fixture exists to avoid.)
func altPackingIdx(t *testing.T, src, sha, realStem string) []byte {
	t.Helper()
	objs, err := exec.Command("git", "-C", src, "rev-list", "--objects", sha).Output()
	if err != nil {
		t.Fatalf("rev-list: %v", err)
	}
	dir := t.TempDir()
	cmd := exec.Command("git", "-C", src, "-c", "pack.compression=0",
		"-c", "pack.packSizeLimit=0",
		"pack-objects", "--no-thin", "--index-version=2", "-q", filepath.Join(dir, "pack"))
	cmd.Stdin = bytes.NewReader(objs)
	nameOut, err := cmd.Output()
	if err != nil {
		t.Fatalf("alt pack-objects: %v", err)
	}
	name := "pack-" + strings.TrimSpace(string(nameOut))
	if name == realStem {
		t.Fatal("fixture: the two packings coincide; the lie would be the truth")
	}
	b, err := os.ReadFile(filepath.Join(dir, name+".idx"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Pair verification with a lying-but-valid CACHED sidecar whose OID set is
// right (see altPackingIdx): pack downloads fine, checksum passes,
// index-pack --verify fails; ONE sidecar re-download fixes it and the fetch
// completes — no heal round consumed.
func TestFetchPairFailureRefreshesSidecarAndCompletes(t *testing.T) {
	src := newGitRepoForPush(t)
	c1 := headOfPushRepo(t, src)
	f := transport.NewFake()
	stems := plantIncrementalPacks(t, f, "/r", src, []string{c1})
	cache := t.TempDir()
	if err := os.WriteFile(filepath.Join(cache, stems[0]+".idx"),
		altPackingIdx(t, src, c1, stems[0]), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := emptyGitRepo(t)
	if _, err := Fetch(f, "/r", dst, cache, []string{c1}); err != nil {
		t.Fatalf("Fetch must recover from a lying sidecar via pair-verify refresh: %v", err)
	}
	if !gitcmd.HasObject(dst, c1) {
		t.Errorf("%s missing", c1)
	}
	// The cache must have been repaired with the remote's true sidecar.
	got, err := os.ReadFile(filepath.Join(cache, stems[0]+".idx"))
	if err != nil || !bytes.Equal(got, f.Files["/r/packs/"+stems[0]+".idx"]) {
		t.Error("cache must hold the true sidecar after the refresh")
	}
}

// A genuinely mismatched REMOTE pair — same-OIDs alt-packing idx planted as
// the remote's own sidecar, so the map is right but the pair can never
// verify — is fatal as "corrupt pair, member undetermined", after the one
// sidecar refresh re-downloads the same wrong bytes.
func TestFetchGenuinelyCorruptPairIsFatalMemberUndetermined(t *testing.T) {
	src := newGitRepoForPush(t)
	c1 := headOfPushRepo(t, src)
	f := transport.NewFake()
	stems := plantIncrementalPacks(t, f, "/r", src, []string{c1})
	f.Files["/r/packs/"+stems[0]+".idx"] = altPackingIdx(t, src, c1, stems[0])

	dst := emptyGitRepo(t)
	_, err := Fetch(f, "/r", dst, "", []string{c1})
	if err == nil {
		t.Fatal("a genuinely mismatched remote pair must be fatal")
	}
	if !strings.Contains(err.Error(), "member undetermined") {
		t.Errorf("fatal must not pretend to know which member is bad: %v", err)
	}
	if n := countInstalledPackFiles(t, dst); n != 0 {
		t.Errorf("nothing may be installed after a pair fatal; got %d", n)
	}
}

// A listed .pack with no .idx (a push in flight) is skipped; a fetch not
// needing it succeeds.
func TestFetchSkipsAnIncompletePairWhenUnneeded(t *testing.T) {
	src := newGitRepoForPush(t)
	c1 := headOfPushRepo(t, src)
	f := transport.NewFake()
	plantIncrementalPacks(t, f, "/r", src, []string{c1})
	f.Files["/r/packs/pack-"+strings.Repeat("f", 40)+".pack"] = []byte("in-flight, no idx")

	dst := emptyGitRepo(t)
	if _, err := Fetch(f, "/r", dst, "", []string{c1}); err != nil {
		t.Fatalf("an unneeded incomplete pair must not break the fetch: %v", err)
	}
}

// End-to-end cache degradation: an unusable cacheDir (a file) never fails
// the fetch, and the trace still shows the sidecars arriving (via temp).
func TestFetchSucceedsWithUnusableCacheDir(t *testing.T) {
	src := newGitRepoForPush(t)
	c1 := headOfPushRepo(t, src)
	f := transport.NewFake()
	plantIncrementalPacks(t, f, "/r", src, []string{c1})
	notADir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := emptyGitRepo(t)
	if _, err := Fetch(f, "/r", dst, notADir, []string{c1}); err != nil {
		t.Fatalf("cache trouble must never be fetch trouble: %v", err)
	}
	if !gitcmd.HasObject(dst, c1) {
		t.Error("objects missing")
	}
}
```

Add `"path"` to `repo_test.go` imports if absent.

- [ ] **Step 2: Run each new test; record RED/GUARD status per test and which assertion fired for the REDs**

```
go test ./internal/repo/ -run 'TestFetchSelfHeals|TestFetchFatalAfterHeal|TestFetchRetries|TestFetchChecksum|TestFetchPairFailure|TestFetchGenuinelyCorrupt|TestFetchSkipsAnIncomplete|TestFetchSucceedsWithUnusable' -v
```

Any test failing against Task 6's implementation exposes a defect in it — fix the implementation (not the test) until green, following the spec's rules. Any test passing immediately: relabel its comment GUARD, and perform a deliberate-regression check for it (break the guarded behaviour temporarily, observe the failure, revert, record).

- [ ] **Step 3: Full suite**

```
go test ./...
```

- [ ] **Step 4: Commit**

```
git add internal/repo/repo_test.go internal/repo/fetch.go internal/repo/packmap.go internal/repo/idxcache.go
git commit -m "test(v2): 3b failure modes - heal, residue, pair refresh, degradation"  (+ trailer)
```

---

### Task 8: The live gate — measured selectivity against the real account

**Files:**
- Create: `docs/research/gates/stage3b-gate.md` (the runbook + results record; committed after the run)

**Interfaces:**
- Consumes: the built helper (`go build ./cmd/git-remote-proton`), the real Proton CLI (`cli-drive@0.7.0`), `GPB_LIVE_ACCOUNT=1`.
- Produces: the stage's pass/fail verdict, recorded verbatim.

**Rules (repeat: these are hard):** every write confined to the two allowed roots — `/my-files/GitRemotes/<demo>` (pick a fresh demo name, e.g. `stage3b-gate`) and `/my-files/_cas-probe` (the contract table's own `liveRoot` lives under it); `GitBackups`, `Sensitive Project Sources`, `Project Repo Bundles`, `ChatGPT Export Text Backup` untouchable; report BLOCKED with verbatim output on any failure — never patch, never retry past the first surprise; verify-before-trashing-parents on cleanup. **Record the `/my-files` listing BEFORE anything else runs**; the final cleanup assertion is that the post-cleanup listing MATCHES that pre-run listing (nothing new left behind) — not a hardcoded folder count, which would false-fail on pre-existing `_cas-probe`/`GitRemotes` residue this gate does not own.

- [ ] **Step 1: Pre-run listing, parent provisioning, then the contract table's live half**

First capture the pre-run truth and provision the probe parent:

1. `proton-drive filesystem list /my-files --json` → record verbatim; this is the listing the final cleanup assertion compares against.
2. `proton-drive filesystem list /my-files/_cas-probe --json`. If `_cas-probe` is ABSENT, create it (`create-folder`) and record "gate created `_cas-probe`" — the contract table's `liveRoot` is `/my-files/_cas-probe/contract` (`contract_test.go:15`) and its parent is not created recursively, so a missing parent would otherwise block mid-test. If it EXISTS, record its contents — anything already inside (including a pre-existing `contract` folder) is residue this gate does not own and must survive cleanup; record specifically whether `contract` was present.
3. Same for `/my-files/GitRemotes`: if ABSENT, create it and record "gate created `GitRemotes`" — folder creation is non-recursive, so Step 2's first push to `proton::/my-files/GitRemotes/stage3b-gate` cannot bootstrap through a missing parent. If it EXISTS, record its contents.

Then run the table:

```
$env:GPB_LIVE_ACCOUNT = "1"; go test ./internal/transport/ -run 'TestContract' -count=1 -v
```
The tests are `TestContractFake` and `TestContractCLI` (`contract_test.go:203,214`) — the pattern must match BOTH; a pattern matching neither reports PASS with "no tests to run", which is a false green. **`-count=1` is mandatory** (added after gate run 2 caught the omission): without it, Go replays a CACHED result verbatim — identical timings included — and a cached replay is not evidence the live half ran. Expected: `TestContractCLI` RUNS its 9 scenarios live (loudly, not skipped — the skip message names `GPB_LIVE_ACCOUNT`) and passes, `TestContractFake` passes. Any failure → BLOCKED.

- [ ] **Step 2: Build and install the helper on PATH; create the demo repo**

Build `git-remote-proton.exe`, ensure it shadows any older copy on PATH (`(Get-Command git-remote-proton).Source`). Create a local source repo with one commit; `git remote add proton-v2 "proton::/my-files/GitRemotes/stage3b-gate"`; `git push proton-v2 main`. Make two further commits, pushing after EACH (`git push proton-v2 main` twice more) → the remote now holds 3 packs from 3 pushes.

- [ ] **Step 3: Verify the remote shape via the CLI (read-only)**

`proton-drive filesystem list /my-files/GitRemotes/stage3b-gate/packs --json` → exactly 3 `.pack` + 3 `.idx`, names matching `pack-[0-9a-f]{40}`. Record the listing verbatim.

- [ ] **Step 4: Fresh clone, stderr captured**

```
git clone -o proton-v2 "proton::/my-files/GitRemotes/stage3b-gate" clone-dir 2> clone-stderr.txt
```
Assert, from `clone-stderr.txt`'s `gpb: downloaded ` lines scoped to `/packs/`: exactly 3 `.pack` + 3 `.idx`. Assert the checkout: correct branch, correct tip (`git log --oneline -3`), `git status` clean, `git fsck` clean. Assert the cache exists: `clone-dir/.git/proton-v2/idx-cache/<key>/` holds 3 `.idx` + `remote` breadcrumb. Everything 3a's gate proved, preserved.

- [ ] **Step 5: Incremental fetch downloads exactly the new pair**

In the source repo: one more commit, `git push proton-v2 main` (remote now holds 4 packs; note the new pack's name from the packs listing). In the clone:

```
git fetch proton-v2 2> fetch-stderr.txt
```
Assert: the `/packs/`-scoped download lines name exactly `pack-4.idx` and `pack-4.pack` (the new stem) — nothing else. `git log proton-v2/main` shows the new commit; `git fsck` clean. **This is the stage's selectivity proof — measured, not assumed.**

- [ ] **Step 6: Up-to-date re-fetch downloads nothing from packs/**

```
git fetch proton-v2 2> refetch-stderr.txt
```
Assert: zero `gpb: downloaded */packs/*` lines. (Ref/marker reads are legitimate and excluded, per the spec's gate scoping.)

- [ ] **Step 7: Record and clean up**

Write `docs/research/gates/stage3b-gate.md` with every command, every assertion, and the verbatim download-line sets. Cleanup, verify-before-trashing-parents throughout:

1. Trash `/my-files/GitRemotes/stage3b-gate` (verify the path holds only this gate's repo first). If `GitRemotes` itself was created by this gate (check the pre-run listing) and is now empty, trash it too.
2. List `/my-files/_cas-probe`: trash `contract` ONLY if Step 1 recorded it as absent pre-run (then this run's contract table created it, and it holds only per-test roots); a pre-existing `contract` is unowned residue and stays, noted in the gate record. Then — ONLY if Step 1 recorded "gate created `_cas-probe`" and it is now empty — trash `_cas-probe` itself. The same ownership rule applies to `GitRemotes` in step 1 above.
3. Final assertion: `proton-drive filesystem list /my-files --json` matches the PRE-RUN listing from Step 1 exactly.

Commit the gate record (+ trailer).

---

## Plan Self-Review (performed at write time)

- **Spec coverage:** trace decorator (T1), ShowIndex v1/v2 (T2), RevListMissing + normative parsing + probe GUARDs + min-git wrap (T3), cache location/shape/degradation/prune/breadcrumb (T4), grammar + complete pairs + map + self-heal-at-build + greedy (T5), loop + heal + residue + restart-on-rebuild + presence short-circuit + verify-before-install preserved (T6), failure modes incl. fatal-after-heal wording, pair member-undetermined, degradation e2e (T7), measured live gate incl. 3a preservation and zero-download re-fetch (T8). Parked-flag dispositions are spec-recorded and need no code.
- **Known coverage limits, stated:** the older-git missing-tip refusal is exercised only as the error-wrap text (untestable end-to-end on a floor-satisfying git — spec records this); the loop-level no-progress fatal is covered at unit level in `greedyCover` (end-to-end unreachable with self-consistent pairs, as the spec's Testing section notes).
- **Type consistency check:** `Fetch(t, root, gitDir, cacheDir string, wants []string)` used identically in T6 steps 1/4 and T7; `EnsureSidecar` returns `(string, bool, error)` in T4 and is consumed with three values in T5; `greedyCover(missing, oidPacks, downloaded)` matches between T5 and T6; helper signatures (`plantIncrementalPacks`, `countPackDownloads`) declared in T6 and reused unchanged in T7.

## Revisions

**Round 1 (2026-08-03, Gemini + Codex; Codex's first attempt timed out on the full bundle and was rerun narrowed, plan-only, at high effort).** Applied from Gemini: `countInstalledPackFiles` listed in Task 7's Consumes (pre-existing helper — Gemini's compile-failure claim was wrong, the listing hygiene right); `copyFile` failure at the sidecar-laying site wrapped `errCacheSuspect` (the source may be a cache path; cache reads degrade, never fatal); `greedyCover`'s defensive arm now fails closed instead of returning a silent partial cover; `RefreshSidecar` evicts the stale cache entry before installing so a failed rename yields a miss, not a stale hit. Rejected from Gemini: the `path`-import Interfaces listing (the step text already instructs the import; Interfaces blocks carry cross-task symbols, not stdlib). Applied from Codex: the plan's `twoCommitRepo` deleted — `gitcmd_test.go:357` already defines it (redeclare = compile error); Task 8's contract-table run pattern corrected to `'TestContract'` (the old pattern matched nothing — a false green); Task 8's write-confinement rules now name both allowed roots incl. `_cas-probe`, and the cleanup assertion compares pre/post listings instead of a hardcoded folder count; `TestFetchDiscoversAcrossPacks` and `TestFetchFrontierDeepensThroughHandBuiltPacks` relabelled GUARD with the reasoning inline (they pass against download-everything; the loop's one genuine RED is the trace-counted selectivity test), and Task 6 Step 1 now extends `Fetch`'s signature body-untouched so that RED is observable against the old behaviour before the rewrite.

**Round 2 (2026-08-03, Codex + Gemini).** Gemini: blockers none. Codex, both accepted: (1) the round-1 gate-cleanup fix was itself defective — the contract table creates `/my-files/_cas-probe/contract` mid-gate (and blocks outright if the non-recursively-created parent is absent), so a bare pre/post listing comparison could not hold; Task 8 Step 1 now captures the pre-run listings AND provisions `_cas-probe` when absent, and Step 7 verifiably cleans `contract` plus any gate-created parents before the pre/post assertion. (2) `greedyCover` errored at the FIRST candidate-less OID in rev-list order, which is unspecified — Task 7's fatal-message assertion on `c1` was therefore nondeterministic; both failure scans now run over the whole frontier and name every offender, sorted, with a unit test pinning the all-offenders property.

**Round 3 (2026-08-03, Codex + Gemini, final under the three-round cap).** Gemini: blockers none. Codex, both accepted, both in the gate task: (1) `GitRemotes` gets the same absent-parent provisioning as `_cas-probe` — folder creation is non-recursive and past gates end with only the four standing folders, so the parent may genuinely be absent and the first push would block; (2) Step 7's trash of `contract` is now conditioned on Step 1 having recorded it absent pre-run — a pre-existing `contract` is unowned residue and survives, so the round-2 ownership rule actually holds for it.
