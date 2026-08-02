# git-remote-proton Stage 3a — `git fetch` and `git clone`

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `git clone -o proton-v2 proton::/my-files/…` produces a working checkout, and `git fetch proton-v2` updates one.

**Architecture:** Fetch downloads **every** pack, verifies each, checks connectivity in a temp area, and only then consolidates the closure into one pack installed into the real object store. Correct by construction because every pack this helper writes is `--no-thin` and self-contained, so the union of all packs is a superset of any closure. Selective discovery is Stage 3b.

**Tech Stack:** Go (stdlib only — no third-party dependencies), the Proton Drive CLI as subprocess transport, git plumbing shelled out for all object work.

## Global Constraints

- **Spec:** `docs/superpowers/specs/2026-08-01-v2-stage3a-fetch-design.md`. Read it first.
- **Parent design:** `docs/v2-remote-helper-design.md` at **v6.2** — normative, settled across six peer-review rounds. Do not re-litigate it; do implement it.
- **Transport contract:** `docs/research/probes/stage1-results.json` is **normative**, certified on Proton Drive CLI **`cli-drive@0.7.0`** only. Findings C1–C15.
- **Stdlib only.** No Go module dependencies.
- **stdout is protocol-only.** All diagnostics go to stderr.
- **Fail closed.** Anything not confirmed is reported to git as failure.
- **Packs can be gigabytes.** Never load one into memory; stream every hash and comparison.
- **The fetch path is strictly read-only on the remote.** It never calls `Bootstrap`, never takes the lock, and never creates anything.
- **Never trust `claimedDigests.sha1`** — Proton flags it `sha1Verified: false`.
- Commit style: repo-conventional prefixes, trailer `Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>`.
- **Do not `git push` the repository.** The user merges and pushes.
- Run Go tests with `go test ./...` from the repo root.
- **Branch:** create `feat/v2-stage3a` from `main`.

**Repo:** `C:\Users\craig\Projects\_Tools\git-proton-backup`

**ENVIRONMENT:** Go was installed mid-session, so shells carry a stale PATH. Prepend to every PowerShell call that runs `go`:

```
$env:Path = "$([Environment]::GetEnvironmentVariable('Path','Machine'));$([Environment]::GetEnvironmentVariable('Path','User'))";
```

`go version` must print `go version go1.26.5 windows/amd64`. `Set-Location` does not persist between PowerShell tool calls — use absolute paths within a single call. `gofmt -l .` is clean as of `main`; if it flags a file you did not touch, that is a `core.autocrlf` artifact — ignore it, do not "fix" line endings.

**Every test is labelled RED or GUARD.** RED must be observed FAILING before its fix — record which assertion fired. GUARD passes before and after; it exists to stop a later change breaking a deliberate decision. A test that cannot fail is not coverage. Two plans in this project were rejected in peer review for containing tests that passed against unpatched code.

---

## File Structure

```
internal/gitcmd/gitcmd.go        + SymbolicRef, ConnectivityOK, RevListNewObjects
internal/gitcmd/gitcmd_test.go   + coverage for all three
internal/transport/contract_test.go   NEW — the shared Fake/CLI contract table
internal/repo/head.go            NEW — DeriveHEAD, WriteHEAD, ReadHEAD
internal/repo/fetch.go           NEW — Fetch orchestration
internal/repo/push.go            + HEAD wiring after refs publish
internal/repo/repo_test.go       + head and fetch coverage
cmd/git-remote-proton/main.go    + fetch capability, list, the fetch batch
```

---

## Task 1 — gitcmd: symbolic-ref, connectivity, and the consolidation object list

**Files:**
- Modify: `internal/gitcmd/gitcmd.go`
- Modify: `internal/gitcmd/gitcmd_test.go`

**Interfaces:**
- Produces: `func SymbolicRef(gitDir, name string) (string, error)`
- Produces: `func ConnectivityOK(gitDir, altObjects string, wants []string) error`
- Produces: `func RevListNewObjects(gitDir, altObjects string, wants []string) (string, error)`

`altObjects` is a path to an **objects directory** — the thing `GIT_ALTERNATE_OBJECT_DIRECTORIES` names. Packs are only found beneath it at `pack/pack-<hash>.{pack,idx}`; dropped flat, git silently does not see them.

**The spec deliberately did not pin the exact rev-list flags**, because nobody had run them against this git. Step 1's test is what pins them: it must fail when an object is genuinely missing. If the invocation below turns out not to behave that way, **change the invocation, not the test** — and say so in your report.

- [ ] **Step 1: Write the failing tests**

Add to `internal/gitcmd/gitcmd_test.go`:

```go
// twoCommitRepo returns a repo with two commits and the sha of each.
func twoCommitRepo(t *testing.T) (dir, first, second string) {
	t.Helper()
	d := newRepo(t) // one commit already
	first = headOf(t, d)
	if err := os.WriteFile(filepath.Join(d, "b.txt"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, a := range [][]string{{"add", "."}, {"commit", "-qm", "c2"}} {
		if err := exec.Command("git", append([]string{"-C", d}, a...)...).Run(); err != nil {
			t.Fatal(err)
		}
	}
	return d, first, headOf(t, d)
}

// altObjectsWith builds an alternate object directory in git's own layout and
// places the given pack (and its .idx) inside it.
func altObjectsWith(t *testing.T, packPath string) string {
	t.Helper()
	alt := filepath.Join(t.TempDir(), "objects")
	packDir := filepath.Join(alt, "pack")
	if err := os.MkdirAll(packDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, src := range []string{packPath, strings.TrimSuffix(packPath, ".pack") + ".idx"} {
		b, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(packDir, filepath.Base(src)), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return alt
}

// RED. ConnectivityOK does not exist. It must PASS on a complete closure and
// FAIL on an incomplete one — the failing half is the whole point.
func TestConnectivityOKDetectsAnIncompleteClosure(t *testing.T) {
	src, first, second := twoCommitRepo(t)

	// A pack of only the SECOND commit: its parent is deliberately absent.
	partial, _, err := WritePack(src, second, []string{first}, t.TempDir())
	if err != nil || partial == "" {
		t.Fatalf("WritePack partial: %v", err)
	}
	// A pack of the whole history.
	full, _, err := WritePack(src, second, nil, t.TempDir())
	if err != nil || full == "" {
		t.Fatalf("WritePack full: %v", err)
	}

	empty := newRepo(t) // a repo that shares no objects with src

	if err := ConnectivityOK(empty, altObjectsWith(t, partial), []string{second}); err == nil {
		t.Error("a pack missing the want's parent must NOT report connectivity ok")
	}
	if err := ConnectivityOK(empty, altObjectsWith(t, full), []string{second}); err != nil {
		t.Errorf("a complete closure must report connectivity ok: %v", err)
	}
}

// RED. RevListNewObjects does not exist. --not --all is what stops an
// incremental fetch reconsolidating history the repo already has.
func TestRevListNewObjectsExcludesWhatTheRepoAlreadyHas(t *testing.T) {
	src, _, second := twoCommitRepo(t)
	full, _, err := WritePack(src, second, nil, t.TempDir())
	if err != nil || full == "" {
		t.Fatalf("WritePack: %v", err)
	}
	alt := altObjectsWith(t, full)

	// Into an empty repo: everything is new.
	empty := newRepo(t)
	objs, err := RevListNewObjects(empty, alt, []string{second})
	if err != nil {
		t.Fatalf("RevListNewObjects: %v", err)
	}
	if !strings.Contains(objs, second) {
		t.Error("an empty repo must be told about the want itself")
	}

	// Into the source repo, which already holds every object: nothing is new.
	objs, err = RevListNewObjects(src, alt, []string{second})
	if err != nil {
		t.Fatalf("RevListNewObjects: %v", err)
	}
	if strings.TrimSpace(objs) != "" {
		t.Errorf("a repo that already has everything must yield no new objects, got:\n%s", objs)
	}
}

// RED. SymbolicRef does not exist. A detached HEAD is ordinary, not an error.
func TestSymbolicRef(t *testing.T) {
	d := newRepo(t)
	got, err := SymbolicRef(d, "HEAD")
	if err != nil {
		t.Fatalf("SymbolicRef: %v", err)
	}
	if got != "refs/heads/main" {
		t.Errorf("HEAD = %q, want refs/heads/main", got)
	}

	if err := exec.Command("git", "-C", d, "checkout", "-q", "--detach").Run(); err != nil {
		t.Fatal(err)
	}
	got, err = SymbolicRef(d, "HEAD")
	if err != nil {
		t.Errorf("a detached HEAD must not be an error: %v", err)
	}
	if got != "" {
		t.Errorf("a detached HEAD must yield \"\", got %q", got)
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/gitcmd/ -run 'TestConnectivityOK|TestRevListNewObjects|TestSymbolicRef'
```

Expected: FAIL — the three functions are undefined. Record it.

- [ ] **Step 3: Implement**

Append to `internal/gitcmd/gitcmd.go`:

```go
// SymbolicRef returns the ref a symbolic ref points at — "refs/heads/main" for
// HEAD in an ordinary checkout. It exists because git never tells a remote
// helper what the client's own HEAD is, and the deterministic HEAD rules need
// it to break a multi-branch tie.
//
// A detached HEAD is not a failure: `symbolic-ref --quiet` exits 1 for "this
// is not a symbolic ref", and that is reported as ("", nil).
func SymbolicRef(gitDir, name string) (string, error) {
	out, code, err := git(gitDir, "symbolic-ref", "--quiet", name)
	switch code {
	case 0:
		return out, nil
	case 1:
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("symbolic-ref %s: %s: %w", name, out, err)
	}
	return "", fmt.Errorf("symbolic-ref %s: %s", name, out)
}

// revListWithAlt runs rev-list against gitDir with altObjects spliced in as an
// alternate object store, feeding the wants on stdin. The wants are passed
// explicitly rather than discovered from refs because at fetch time nothing
// references them yet.
//
// altObjects is an OBJECTS directory: git looks for packs at
// <altObjects>/pack/pack-<hash>.{pack,idx} and silently ignores them anywhere
// else, which presents downstream as "every object is missing".
func revListWithAlt(gitDir, altObjects string, wants []string, args ...string) (string, int, error) {
	cmd := exec.Command("git", append([]string{"-C", gitDir, "rev-list"}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_ALTERNATE_OBJECT_DIRECTORIES="+altObjects)
	cmd.Stdin = strings.NewReader(strings.Join(wants, "\n") + "\n")
	// See waitDelay: DO NOT DELETE as unnecessary.
	cmd.WaitDelay = waitDelay
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := -1
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	if errors.Is(err, exec.ErrWaitDelay) {
		// rev-list itself exited 0 but a grandchild held the pipe. stdout may
		// be truncated, and here stdout IS the object list, so refusing to
		// guess is the only safe answer.
		return "", -1, fmt.Errorf("rev-list output was abandoned after %s; refusing to "+
			"act on a possibly truncated object list", waitDelay)
	}
	if code != 0 {
		return "", code, fmt.Errorf("rev-list %s: %s", strings.Join(args, " "),
			strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), 0, nil
}

// ConnectivityOK reports whether every object reachable from wants is present,
// counting gitDir's own store plus altObjects. A nil return means the closure
// is complete.
//
// This is a traversal, not an fsck: the wants are not referenced by any ref, so
// fsck would never reach them. --quiet suppresses the object list; the exit
// code is the answer.
func ConnectivityOK(gitDir, altObjects string, wants []string) error {
	if len(wants) == 0 {
		return nil
	}
	if _, _, err := revListWithAlt(gitDir, altObjects, wants,
		"--objects", "--stdin", "--not", "--all", "--quiet"); err != nil {
		return fmt.Errorf("closure is incomplete for the requested objects: %w", err)
	}
	return nil
}

// RevListNewObjects returns the objects reachable from wants that gitDir does
// not already have, one per line, ready to feed to pack-objects.
//
// --not --all is load-bearing: without it an incremental fetch reconsolidates
// the entire history into a fresh pack and installs it, silently doubling
// local disk every time. It works perfectly on a two-commit test repo.
func RevListNewObjects(gitDir, altObjects string, wants []string) (string, error) {
	if len(wants) == 0 {
		return "", nil
	}
	out, _, err := revListWithAlt(gitDir, altObjects, wants,
		"--objects", "--stdin", "--not", "--all")
	if err != nil {
		return "", err
	}
	return out, nil
}
```

Add `errors` and `os` to the imports if not already present.

- [ ] **Step 4: Run to verify pass**

```bash
go test ./internal/gitcmd/
```

Expected: PASS. If `TestConnectivityOKDetectsAnIncompleteClosure` does not fail on the partial pack, the flag set is wrong — fix the invocation, not the test, and report what you changed.

- [ ] **Step 5: Commit**

```bash
git add internal/gitcmd
git commit -m "feat(v2): symbolic-ref, connectivity check, and the new-object list"
```

---

## Task 2 — the shared Transport contract table

**Files:**
- Create: `internal/transport/contract_test.go`

**Interfaces:**
- Produces: nothing exported. A test-only table both `*Fake` and `*CLI` run against.

This is the Stage 2 review's named systemic fix. The `Transport` interface documented behaviour but nothing enforced it, so `*Fake` and `*CLI` drifted three times and only real code found it. One table, two harnesses.

- [ ] **Step 1: Write the table**

Create `internal/transport/contract_test.go`:

```go
package transport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// liveEnv gates the *CLI half. CI never sets it, so `go test ./...` on a
// runner exercises only the Fake and can never touch a real account.
const liveEnv = "GPB_LIVE_ACCOUNT"

// liveRoot is the only remote path the live half may write to.
const liveRoot = "/my-files/_cas-probe/contract"

// contractCase is one scenario, expressed against the interface alone.
type contractCase struct {
	name string
	run  func(t *testing.T, tr Transport, root string, stage func(name, content string) string)
}

var contractCases = []contractCase{
	{"stat absence is not an error", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		_, ok, err := tr.Stat(root + "/definitely-absent")
		if err != nil {
			t.Fatalf("absence must be (_, false, nil), got err %v", err)
		}
		if ok {
			t.Error("a node that was never created must not exist")
		}
	}},

	// C11: upload names the node after the LOCAL basename, so the caller
	// contract is that the local basename equals the target leaf.
	{"create names the node after the target leaf", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		local := stage("leafname.txt", "hello")
		if out, err := tr.CreateExclusive(root+"/leafname.txt", local); err != nil || out != Committed {
			t.Fatalf("create: %v %v", out, err)
		}
		if _, ok, err := tr.Stat(root + "/leafname.txt"); err != nil || !ok {
			t.Fatalf("the node must exist under the target leaf: %v %v", ok, err)
		}
	}},

	// A6: a second create of the same name is refused, not overwritten.
	{"create refuses a name already taken", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		p := root + "/taken.txt"
		if out, err := tr.CreateExclusive(p, stage("taken.txt", "first")); err != nil || out != Committed {
			t.Fatalf("first create: %v %v", out, err)
		}
		out, err := tr.CreateExclusive(p, stage("taken.txt", "second"))
		if err != nil {
			t.Fatalf("second create errored: %v", err)
		}
		if out != Refused {
			t.Errorf("a taken name must be Refused, got %v", out)
		}
	}},

	// ReadTo's destination is a DIRECTORY, and the file lands under the
	// node's own remote basename. The Fake and the CLI disagreed on this.
	{"readTo lands under the remote basename in an existing dir", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		p := root + "/readback.txt"
		if out, err := tr.CreateExclusive(p, stage("readback.txt", "payload")); err != nil || out != Committed {
			t.Fatalf("create: %v %v", out, err)
		}
		dir := t.TempDir()
		if err := tr.ReadTo(p, dir); err != nil {
			t.Fatalf("ReadTo: %v", err)
		}
		got, err := os.ReadFile(filepath.Join(dir, "readback.txt"))
		if err != nil {
			t.Fatalf("the download must land under the remote basename: %v", err)
		}
		if string(got) != "payload" {
			t.Errorf("content = %q, want payload", got)
		}
	}},

	{"readTo into a missing directory errors and creates nothing", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		p := root + "/nodir.txt"
		if out, err := tr.CreateExclusive(p, stage("nodir.txt", "x")); err != nil || out != Committed {
			t.Fatalf("create: %v %v", out, err)
		}
		missing := filepath.Join(t.TempDir(), "not-created")
		if err := tr.ReadTo(p, missing); err == nil {
			t.Error("ReadTo must not create its destination directory")
		}
		if _, err := os.Stat(missing); !os.IsNotExist(err) {
			t.Error("the destination directory must still not exist")
		}
	}},

	// C4: trash on a missing target is Committed — the desired end state is
	// "not there", and the CLI's own exit 1 is absorbed by the implementation.
	{"trash on a missing target is committed", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		out, err := tr.Trash(root + "/never-existed.txt")
		if err != nil {
			t.Fatalf("trash of an absent node errored: %v", err)
		}
		if out != Committed {
			t.Errorf("an already-absent node must be Committed, got %v", out)
		}
	}},

	// C5 + the Fake's own gap: an EnsureDir'd empty folder must be visible
	// to List. The Fake ignored f.Dirs and did not show it.
	{"ensureDir is idempotent and its result is listable", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		d := root + "/emptydir"
		if err := tr.EnsureDir(d); err != nil {
			t.Fatalf("EnsureDir: %v", err)
		}
		if err := tr.EnsureDir(d); err != nil {
			t.Fatalf("EnsureDir must be idempotent: %v", err)
		}
		nodes, err := tr.List(root)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, n := range nodes {
			if n.Name == "emptydir" {
				if !n.IsDir {
					t.Error("an EnsureDir'd node must list as a directory")
				}
				return
			}
		}
		t.Error("an EnsureDir'd empty directory must appear in its parent listing")
	}},

	{"list of an empty directory is empty, not an error", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		d := root + "/emptylist"
		if err := tr.EnsureDir(d); err != nil {
			t.Fatalf("EnsureDir: %v", err)
		}
		nodes, err := tr.List(d)
		if err != nil {
			t.Fatalf("an empty listing must not be an error: %v", err)
		}
		if len(nodes) != 0 {
			t.Errorf("want an empty listing, got %d nodes", len(nodes))
		}
	}},
}

// runContract executes the table against one implementation. root must already
// exist and be empty; stage writes a local file under a given basename.
func runContract(t *testing.T, newTransport func(t *testing.T) (Transport, string)) {
	for _, c := range contractCases {
		t.Run(c.name, func(t *testing.T) {
			tr, root := newTransport(t)
			dir := t.TempDir()
			stage := func(name, content string) string {
				p := filepath.Join(dir, name)
				if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
				return p
			}
			c.run(t, tr, root, stage)
		})
	}
}

func TestContractFake(t *testing.T) {
	runContract(t, func(t *testing.T) (Transport, string) {
		f := NewFake()
		root := "/r"
		if err := f.EnsureDir(root); err != nil {
			t.Fatal(err)
		}
		return f, root
	})
}

func TestContractCLI(t *testing.T) {
	if os.Getenv(liveEnv) == "" {
		// LOUD, never silent. Stage 2.1 had a guard rot into a no-op skip,
		// and a silent skip in the table that exists to prevent silent drift
		// would be self-defeating.
		t.Skipf("SKIPPING THE LIVE HALF of the Transport contract table. "+
			"The *Fake half ran; *CLI did not, so fake/real drift is NOT covered by this run. "+
			"Set %s=1 to run it. It writes only under %s and trashes that root afterwards.",
			liveEnv, liveRoot)
	}
	runContract(t, func(t *testing.T) (Transport, string) {
		c := NewCLI("")
		v, err := c.Version()
		if err != nil {
			t.Fatalf("the live half needs a working proton-drive: %v", err)
		}
		if !IsCertified(v) {
			t.Fatalf("live half refuses an uncertified CLI: got %q, certified is %s", v, CertifiedCLI)
		}
		// A per-test root so cases cannot see each other's nodes.
		root := liveRoot + "/" + strings.ReplaceAll(t.Name(), "/", "_")
		if err := c.EnsureDir(liveRoot); err != nil {
			t.Fatalf("EnsureDir %s: %v", liveRoot, err)
		}
		if err := c.EnsureDir(root); err != nil {
			t.Fatalf("EnsureDir %s: %v", root, err)
		}
		t.Cleanup(func() {
			if out, err := c.Trash(root); err != nil {
				t.Errorf("CLEANUP FAILED for %s (outcome %s): %v — remove it by hand", root, out, err)
			}
		})
		return c, root
	})
}
```

`t.Name()` inside a subtest includes the parent, so each case gets its own remote root and the cleanup is per case.

- [ ] **Step 2: Run the fake half and confirm the skip is loud**

```bash
go test ./internal/transport/ -run TestContract -v
```

Expected: every `TestContractFake` case PASSES, and `TestContractCLI` SKIPS with the message naming `GPB_LIVE_ACCOUNT`. **If any Fake case fails, stop and report** — that is a real fake/contract divergence, not a test bug, and it is exactly what this table exists to find.

- [ ] **Step 3: Commit**

```bash
git add internal/transport/contract_test.go
git commit -m "test(v2): shared Transport contract table, live half opt-in"
```

**Do not run the live half in this task.** The controller runs it once, deliberately, at the Task 7 gate.

---

## Task 3 — HEAD: derive, write, read

**Files:**
- Create: `internal/repo/head.go`
- Modify: `internal/repo/repo_test.go`

**Interfaces:**
- Produces: `func DeriveHEAD(candidates []string, clientHEAD string) (string, bool)`
- Produces: `func WriteHEAD(t transport.Transport, root, branch string) (transport.Outcome, error)`
- Produces: `func ReadHEAD(t transport.Transport, root string) (string, bool, error)`

`candidates` are **full ref names** (`refs/heads/main`), and the returned branch is a full ref name too. `clientHEAD` is the client's own symbolic HEAD, or `""` if detached or unknown.

**`WriteHEAD` cannot reuse `WriteRef`** — that function validates its payload against `^[0-9a-f]{40}$` and would refuse a symref outright. It needs its own write-and-verify path, sharing `stagedFile` but not `WriteRef`.

- [ ] **Step 1: Write the failing tests**

Add to `internal/repo/repo_test.go`:

```go
// RED. DeriveHEAD does not exist. A pure function — no transport needed.
func TestDeriveHEAD(t *testing.T) {
	cases := []struct {
		name       string
		candidates []string
		clientHEAD string
		want       string
		wantOK     bool
	}{
		{"none", nil, "refs/heads/main", "", false},
		{"single wins outright", []string{"refs/heads/only"}, "refs/heads/other", "refs/heads/only", true},
		{"client HEAD breaks the tie",
			[]string{"refs/heads/zeta", "refs/heads/alpha", "refs/heads/main"},
			"refs/heads/main", "refs/heads/main", true},
		{"lexicographically first when the client HEAD is absent",
			[]string{"refs/heads/zeta", "refs/heads/alpha"},
			"refs/heads/nowhere", "refs/heads/alpha", true},
		{"lexicographically first when the client is detached",
			[]string{"refs/heads/zeta", "refs/heads/alpha"},
			"", "refs/heads/alpha", true},
		{"non-branches are not candidates", []string{"refs/tags/v1"}, "", "", false},
	}
	for _, c := range cases {
		got, ok := DeriveHEAD(c.candidates, c.clientHEAD)
		if got != c.want || ok != c.wantOK {
			t.Errorf("%s: DeriveHEAD = (%q, %v), want (%q, %v)", c.name, got, ok, c.want, c.wantOK)
		}
	}
}

// RED. WriteHEAD/ReadHEAD do not exist.
func TestWriteAndReadHEAD(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")

	if _, ok, err := ReadHEAD(f, "/r"); err != nil || ok {
		t.Fatalf("a fresh repo has no HEAD: %v %v", ok, err)
	}
	if out, err := WriteHEAD(f, "/r", "refs/heads/main"); err != nil || out != transport.Committed {
		t.Fatalf("WriteHEAD: %v %v", out, err)
	}
	if got := string(f.Files["/r/HEAD"]); got != "ref: refs/heads/main\n" {
		t.Errorf("HEAD content = %q", got)
	}
	branch, ok, err := ReadHEAD(f, "/r")
	if err != nil || !ok {
		t.Fatalf("ReadHEAD: %v %v", ok, err)
	}
	if branch != "refs/heads/main" {
		t.Errorf("ReadHEAD = %q, want refs/heads/main", branch)
	}
}

// RED. A symref payload must not be forced through WriteRef's 40-hex rule,
// and a garbage HEAD must be fatal rather than coerced.
func TestReadHEADRejectsCorruptContent(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	f.Files["/r/HEAD"] = []byte("1111111111111111111111111111111111111111\n")
	if _, _, err := ReadHEAD(f, "/r"); err == nil {
		t.Error("a detached-OID HEAD is not a symref and must be refused, not coerced")
	}
	f.Files["/r/HEAD"] = []byte("ref: refs/tags/v1\n")
	if _, _, err := ReadHEAD(f, "/r"); err == nil {
		t.Error("HEAD must point at a branch")
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/repo/ -run 'TestDeriveHEAD|TestWriteAndReadHEAD|TestReadHEADRejects'
```

Expected: FAIL, undefined `DeriveHEAD`. Record it.

- [ ] **Step 3: Implement `head.go`**

```go
package repo

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// HeadName is the remote file holding the symref.
const HeadName = "HEAD"

const headPrefix = "ref: "

// DeriveHEAD picks the default branch deterministically, so the result never
// depends on the order refs happened to arrive in.
//
// candidates are full ref names; only refs/heads/* are eligible. One eligible
// candidate wins outright. Among several, the client's own HEAD wins if it is
// present, otherwise the lexicographically first. No eligible candidate means
// no HEAD is written and the repo stays headless, which is a defined state.
func DeriveHEAD(candidates []string, clientHEAD string) (string, bool) {
	var branches []string
	for _, c := range candidates {
		if strings.HasPrefix(c, "refs/heads/") {
			branches = append(branches, c)
		}
	}
	if len(branches) == 0 {
		return "", false
	}
	if len(branches) == 1 {
		return branches[0], true
	}
	for _, b := range branches {
		if b == clientHEAD {
			return b, true
		}
	}
	sort.Strings(branches)
	return branches[0], true
}

// WriteHEAD writes the symref and verifies it by read-back.
//
// It deliberately does NOT go through WriteRef: that function validates its
// payload against ^[0-9a-f]{40}$ and would refuse a symref outright. The
// read-back is still required for the same reason it is everywhere else — the
// CLI silently skips a byte-identical rewrite, so Committed alone does not
// prove our bytes landed.
func WriteHEAD(t transport.Transport, root, branch string) (transport.Outcome, error) {
	if !strings.HasPrefix(branch, "refs/heads/") {
		return transport.Ambiguous, fmt.Errorf("refusing to point HEAD at %q: not a branch", branch)
	}
	staged, cleanup, err := stagedFile([]byte(headPrefix+branch+"\n"), HeadName)
	if err != nil {
		return transport.Ambiguous, err
	}
	defer cleanup()

	p := root + "/" + HeadName
	out, err := t.CreateExclusive(p, staged)
	if err != nil {
		return transport.Ambiguous, err
	}
	switch out {
	case transport.Ambiguous:
		return transport.Ambiguous, fmt.Errorf("HEAD write outcome ambiguous for %s; re-run to reconcile", p)
	case transport.Refused:
		// Someone wrote HEAD between our check and our write. We never
		// overwrite an existing HEAD, so adopt theirs.
		return transport.Refused, nil
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

// ReadHEAD returns the branch HEAD points at. Absence is (_, false, nil);
// content that is not a branch symref is fatal, never coerced — the same rule
// the ref-file grammar follows.
func ReadHEAD(t transport.Transport, root string) (string, bool, error) {
	p := root + "/" + HeadName
	if _, ok, err := t.Stat(p); err != nil {
		return "", false, fmt.Errorf("stat %s: %w", p, err)
	} else if !ok {
		return "", false, nil
	}
	dir, err := os.MkdirTemp("", "gpb-head-*")
	if err != nil {
		return "", false, err
	}
	defer os.RemoveAll(dir)
	if err := t.ReadTo(p, dir); err != nil {
		return "", false, fmt.Errorf("cannot read %s: %w", p, err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, HeadName))
	if err != nil {
		return "", false, fmt.Errorf("cannot read back %s: %w", p, err)
	}
	s := strings.TrimRight(string(raw), "\r\n")
	if !strings.HasPrefix(s, headPrefix) {
		return "", false, fmt.Errorf("corrupt %s: %q is not a symref", p, s)
	}
	branch := strings.TrimSpace(strings.TrimPrefix(s, headPrefix))
	if !strings.HasPrefix(branch, "refs/heads/") {
		return "", false, fmt.Errorf("corrupt %s: points at %q, which is not a branch", p, branch)
	}
	return branch, true, nil
}
```

Add `path/filepath` to the imports.

- [ ] **Step 4: Run to verify pass**

```bash
go test ./internal/repo/
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repo/head.go internal/repo/repo_test.go
git commit -m "feat(v2): derive, write and read the remote HEAD symref"
```

---

## Task 4 — wire HEAD into push

**Files:**
- Modify: `internal/repo/push.go`
- Modify: `internal/repo/repo_test.go`

**Interfaces:**
- Consumes: `DeriveHEAD`, `WriteHEAD`, `ReadHEAD` (Task 3); `gitcmd.SymbolicRef` (Task 1).
- Modifies: `Push` — same signature, now writes HEAD when absent.

`Push`'s current signature is `Push(t transport.Transport, root, gitDir string, ups []protocol.RefUpdate, remote map[string]string) []Result`.

**Never touch an existing HEAD.** Changing a default branch stays an explicit operation out of scope for v2. This only *completes* a missing one, the same way `Bootstrap` completes a missing `refs/` or `packs/`.

**The candidate set is every branch on the remote after this push**, not just the ones this push published — a repo whose `main` was pushed last week, receiving a push of `bugfix` today, must not get HEAD pointed at `bugfix`.

- [ ] **Step 1: Write the failing tests**

Add to `internal/repo/repo_test.go`:

```go
// RED. Push does not write HEAD at all today.
func TestPushWritesHeadOnFirstPush(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	d := newGitRepoForPush(t)
	head := headOfPushRepo(t, d)

	res := Push(f, "/r", d, []protocol.RefUpdate{{Src: head, Dst: "refs/heads/main"}}, map[string]string{})
	if len(res) != 1 || !res[0].OK {
		t.Fatalf("push: %+v", res)
	}
	if got := string(f.Files["/r/HEAD"]); got != "ref: refs/heads/main\n" {
		t.Errorf("HEAD = %q, want ref: refs/heads/main", got)
	}
}

// RED. Backfill: an existing repo with branches but no HEAD gets one, and the
// candidate set is every remote branch — not just what this push published.
func TestPushBackfillsHeadFromAllRemoteBranches(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	d := newGitRepoForPush(t)
	head := headOfPushRepo(t, d)

	// "alpha" is already on the remote from an earlier push; no HEAD exists.
	if _, err := WriteRef(f, "/r", "refs/heads/alpha", head, false); err != nil {
		t.Fatal(err)
	}
	remote := map[string]string{"refs/heads/alpha": head}

	// Today we push "zeta" only.
	res := Push(f, "/r", d, []protocol.RefUpdate{{Src: head, Dst: "refs/heads/zeta"}}, remote)
	if len(res) != 1 || !res[0].OK {
		t.Fatalf("push: %+v", res)
	}
	if got := string(f.Files["/r/HEAD"]); got != "ref: refs/heads/alpha\n" {
		t.Errorf("HEAD = %q — the candidate set must include branches this push did not touch", got)
	}
}

// GUARD. An existing HEAD is never rewritten.
func TestPushNeverRewritesAnExistingHead(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	d := newGitRepoForPush(t)
	head := headOfPushRepo(t, d)
	f.Files["/r/HEAD"] = []byte("ref: refs/heads/chosen\n")

	res := Push(f, "/r", d, []protocol.RefUpdate{{Src: head, Dst: "refs/heads/main"}}, map[string]string{})
	if len(res) != 1 || !res[0].OK {
		t.Fatalf("push: %+v", res)
	}
	if got := string(f.Files["/r/HEAD"]); got != "ref: refs/heads/chosen\n" {
		t.Errorf("an existing HEAD must not be touched, got %q", got)
	}
}

// GUARD. A tag-only push leaves the repo headless — a defined state.
func TestPushTagOnlyLeavesRepoHeadless(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	d := newGitRepoForPush(t)
	head := headOfPushRepo(t, d)

	res := Push(f, "/r", d, []protocol.RefUpdate{{Src: head, Dst: "refs/tags/v1"}}, map[string]string{})
	if len(res) != 1 || !res[0].OK {
		t.Fatalf("push: %+v", res)
	}
	if _, ok := f.Files["/r/HEAD"]; ok {
		t.Error("a tag-only push must not write HEAD")
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/repo/ -run 'TestPushWritesHead|TestPushBackfillsHead'
```

Expected: FAIL — no `/r/HEAD` is written. The two GUARDs pass already. Record which assertion fired.

- [ ] **Step 3: Implement**

At the end of `Push`, after the per-ref loop builds `results` and before it returns, add:

```go
	// Complete a missing HEAD. This is the same rule Bootstrap applies to a
	// missing refs/ or packs/ — a partial initialisation is completed, not
	// rejected — and it is why a repo pushed before this shipped is still
	// clonable. An EXISTING HEAD is never touched: changing the default
	// branch stays an explicit operation, out of scope for v2.
	ensureHEAD(t, root, gitDir, ups, remote, results)
	return results
```

Then add:

```go
// ensureHEAD writes HEAD when the remote has none. Failure is reported on
// stderr and does not fail the push: the refs and objects are already
// published and correct, and turning a successful push into an error because
// a convenience symref could not be written would be the wrong trade.
func ensureHEAD(t transport.Transport, root, gitDir string,
	ups []protocol.RefUpdate, remote map[string]string, results []Result) {

	if _, ok, err := ReadHEAD(t, root); err != nil {
		fmt.Fprintf(os.Stderr, "git-remote-proton: cannot read remote HEAD: %v\n", err)
		return
	} else if ok {
		return // never rewrite
	}

	// Candidates are every branch on the remote AFTER this push: those that
	// were already there, plus those this push actually published. Taking
	// only the published set would point HEAD at whatever happened to be in
	// today's batch.
	seen := map[string]bool{}
	for ref := range remote {
		seen[ref] = true
	}
	okNow := map[string]bool{}
	for _, r := range results {
		if r.OK {
			okNow[r.Ref] = true
		}
	}
	for _, u := range ups {
		if u.Src == "" && okNow[u.Dst] {
			delete(seen, u.Dst) // a successful delete removes a candidate
			continue
		}
		if okNow[u.Dst] {
			seen[u.Dst] = true
		}
	}
	candidates := make([]string, 0, len(seen))
	for ref := range seen {
		candidates = append(candidates, ref)
	}

	clientHEAD, err := gitcmd.SymbolicRef(gitDir, "HEAD")
	if err != nil {
		// A detached HEAD is ("", nil), so this is a real failure. It only
		// costs us the tie-break, so warn and continue.
		fmt.Fprintf(os.Stderr, "git-remote-proton: cannot read local HEAD: %v\n", err)
	}
	branch, ok := DeriveHEAD(candidates, clientHEAD)
	if !ok {
		return // headless is a defined state
	}
	if out, err := WriteHEAD(t, root, branch); err != nil {
		fmt.Fprintf(os.Stderr, "git-remote-proton: could not write remote HEAD (%s): %v\n", out, err)
	}
}
```

Add `os` to `push.go`'s imports if not already present.

- [ ] **Step 4: Run to verify pass**

```bash
go test ./... && go vet ./... && gofmt -l .
```

Expected: PASS, `gofmt -l` silent.

- [ ] **Step 5: Commit**

```bash
git add internal/repo
git commit -m "feat(v2): complete a missing remote HEAD on push"
```

---

## Task 5 — fetch orchestration

**Files:**
- Create: `internal/repo/fetch.go`
- Modify: `internal/repo/repo_test.go`

**Interfaces:**
- Consumes: `gitcmd.ConnectivityOK`, `gitcmd.RevListNewObjects` (Task 1); `checkPackChecksum` and `gitcmd.IndexPackVerify` (already on `main`).
- Produces: `func Fetch(t transport.Transport, root, gitDir string, wants []string) (keepPath string, err error)`

An empty `keepPath` with a nil error means "nothing new to install" — a legitimate up-to-date fetch, not a failure.

**Read-only on the remote.** Never `Bootstrap`, never take the lock, never create anything. A missing or unrecognised marker is a hard refusal.

**Verify before install.** Every download, integrity and connectivity failure happens while the objects are still in the temp area, so the local object store is untouched.

**The `.idx` is never checksummed against its basename** — its basename is the *pack's* checksum. Pairs are validated by `IndexPackVerify`; only the `.pack` gets the checksum-vs-basename check. This mirrors `publishPack`/`publishIdx` exactly.

- [ ] **Step 1: Write the failing tests**

Add to `internal/repo/repo_test.go`:

```go
// emptyGitRepo returns a repo with NO commits.
//
// It exists because newGitRepoForPush builds an identical commit every time —
// same content "one", same message "c1", same author — and a git commit sha
// covers the author and committer timestamps at one-second resolution. Two
// such repos created inside the same second get the SAME sha, so a "fetch into
// a repo that lacks the objects" test would silently be fetching into a repo
// that already has them: RevListNewObjects returns nothing, Fetch returns
// ("", nil), and the test fails for a reason unrelated to the code under test.
// Flaky by the clock, which is worse than simply wrong.
func emptyGitRepo(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	for _, a := range [][]string{
		{"init", "-qb", "main"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"},
	} {
		if err := exec.Command("git", append([]string{"-C", d}, a...)...).Run(); err != nil {
			t.Fatalf("git %v: %v", a, err)
		}
	}
	return d
}

// plantRepoOnFake pushes a real repo's history into a Fake as a v2 remote:
// marker, refs, and one pack pair under packs/.
func plantRepoOnFake(t *testing.T, f *transport.Fake, root, gitDir, sha string) {
	t.Helper()
	if err := Bootstrap(f, root); err != nil {
		t.Fatal(err)
	}
	packPath, idxPath, err := gitcmd.WritePack(gitDir, sha, nil, t.TempDir())
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
	if _, err := WriteRef(f, root, "refs/heads/main", sha, false); err != nil {
		t.Fatal(err)
	}
}

// RED. Fetch does not exist. Real git on both ends, the Fake in between.
func TestFetchInstallsTheClosure(t *testing.T) {
	src := newGitRepoForPush(t)
	sha := headOfPushRepo(t, src)
	f := transport.NewFake()
	plantRepoOnFake(t, f, "/r", src, sha)

	dst := emptyGitRepo(t) // genuinely lacks src's objects — see emptyGitRepo
	keep, err := Fetch(f, "/r", dst, []string{sha})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if keep == "" {
		t.Fatal("a fetch that installs objects must return a .keep path")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf(".keep must exist on disk: %v", err)
	}
	if !gitcmd.HasObject(dst, sha) {
		t.Error("the wanted object must be present after a fetch")
	}
}

// RED. A second fetch of the same want installs nothing — this is what
// --not --all buys, and without it every fetch reinstalls all of history.
func TestFetchIsIdempotentAndInstallsNothingWhenUpToDate(t *testing.T) {
	src := newGitRepoForPush(t)
	sha := headOfPushRepo(t, src)
	f := transport.NewFake()
	plantRepoOnFake(t, f, "/r", src, sha)

	dst := emptyGitRepo(t)
	if _, err := Fetch(f, "/r", dst, []string{sha}); err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	keep, err := Fetch(f, "/r", dst, []string{sha})
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if keep != "" {
		t.Errorf("an up-to-date fetch must install nothing, got keep %q", keep)
	}
}

// RED. A corrupt remote pack must be fatal, and must leave the local store
// untouched — fetch is the one path that can damage the user's own repo.
func TestFetchRejectsACorruptPackAndInstallsNothing(t *testing.T) {
	src := newGitRepoForPush(t)
	sha := headOfPushRepo(t, src)
	f := transport.NewFake()
	plantRepoOnFake(t, f, "/r", src, sha)

	for name, b := range f.Files {
		if strings.HasSuffix(name, ".pack") {
			c := append([]byte(nil), b...)
			c[len(c)/2] ^= 0xff
			f.Files[name] = c
		}
	}

	dst := emptyGitRepo(t)
	before := countPackFiles(t, dst)
	if _, err := Fetch(f, "/r", dst, []string{sha}); err == nil {
		t.Fatal("a corrupt remote pack must be fatal")
	}
	if got := countPackFiles(t, dst); got != before {
		t.Errorf("a failed fetch must install nothing: pack count %d -> %d", before, got)
	}
}

// RED. Fetch is read-only: no marker means refuse, never initialise.
func TestFetchRefusesAnUnmarkedRemote(t *testing.T) {
	f := transport.NewFake()
	if err := f.EnsureDir("/r"); err != nil {
		t.Fatal(err)
	}
	dst := emptyGitRepo(t)
	if _, err := Fetch(f, "/r", dst, []string{"1111111111111111111111111111111111111111"}); err == nil {
		t.Error("fetch must refuse a folder with no marker, not initialise it")
	}
	if _, ok := f.Files["/r/gpb-remote.json"]; ok {
		t.Error("fetch must never create a marker")
	}
}

func countPackFiles(t *testing.T, gitDir string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(gitDir, ".git", "objects", "pack"))
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".pack") {
			n++
		}
	}
	return n
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/repo/ -run TestFetch
```

Expected: FAIL, undefined `Fetch`. Record it.

- [ ] **Step 3: Implement `fetch.go`**

```go
package repo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/gitcmd"
	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// Fetch downloads every pack from the remote, verifies each, confirms the
// closure covers wants, and installs it as ONE pack with an adjacent .keep.
// It returns that .keep's path for the caller's `lock` response, or "" when
// the local repository already had everything.
//
// It is READ-ONLY on the remote: no Bootstrap, no lock, nothing created. A
// fetch must never be able to bring a repository into existence.
//
// Stage 3a downloads every pack rather than discovering which are needed.
// That is correct by construction — every pack this helper writes is
// --no-thin and self-contained, so their union is a superset of any closure.
// Selective discovery is Stage 3b.
func Fetch(t transport.Transport, root, gitDir string, wants []string) (string, error) {
	if err := requireMarker(t, root); err != nil {
		return "", err
	}
	if len(wants) == 0 {
		return "", nil
	}

	tmp, err := os.MkdirTemp("", "gpb-fetch-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)

	// git only finds packs in an alternate at <objects>/pack/. Flat, they are
	// silently invisible and every object reads as missing.
	altObjects := filepath.Join(tmp, "objects")
	packDir := filepath.Join(altObjects, "pack")
	if err := os.MkdirAll(packDir, 0o700); err != nil {
		return "", err
	}

	if err := downloadAllPacks(t, root, packDir); err != nil {
		return "", err
	}
	if err := verifyDownloadedPacks(packDir); err != nil {
		return "", err
	}

	// BEFORE install. A failure here must leave the local store untouched.
	if err := gitcmd.ConnectivityOK(gitDir, altObjects, wants); err != nil {
		return "", fmt.Errorf("the remote does not hold a complete closure for the "+
			"requested objects, so nothing was installed: %w", err)
	}

	objs, err := gitcmd.RevListNewObjects(gitDir, altObjects, wants)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(objs) == "" {
		return "", nil // already up to date
	}
	return consolidateAndInstall(gitDir, altObjects, objs)
}

// requireMarker is the read-only half of Bootstrap's check: the marker must be
// present AND recognised. Absent or unrecognised is a hard refusal — the
// helper never guesses whether a folder is one of its repos, and a fetch is
// certainly not licence to initialise one.
func requireMarker(t transport.Transport, root string) error {
	marker := root + "/" + MarkerName
	if _, ok, err := t.Stat(marker); err != nil {
		return fmt.Errorf("stat %s: %w", marker, err)
	} else if !ok {
		return fmt.Errorf("refusing to fetch from %s: no %s — it is not a git-remote-proton repo",
			root, MarkerName)
	}
	// checkMarker takes the marker's own PATH, not the repo root — passing the
	// root would try to download a directory.
	return checkMarker(t, marker)
}

func downloadAllPacks(t transport.Transport, root, packDir string) error {
	nodes, err := t.List(root + "/packs")
	if err != nil {
		return fmt.Errorf("cannot list %s/packs: %w", root, err)
	}
	got := 0
	for _, n := range nodes {
		if n.IsDir || !(strings.HasSuffix(n.Name, ".pack") || strings.HasSuffix(n.Name, ".idx")) {
			continue
		}
		if err := t.ReadTo(root+"/packs/"+n.Name, packDir); err != nil {
			return fmt.Errorf("cannot download %s: %w", n.Name, err)
		}
		if strings.HasSuffix(n.Name, ".pack") {
			got++
		}
	}
	if got == 0 {
		return fmt.Errorf("%s/packs holds no packs; the remote is incomplete", root)
	}
	return nil
}

// verifyDownloadedPacks checks each pair PER MEMBER. Only the .pack is
// checksummed against its basename — a .idx borrows the pack's name, so
// hashing the index and comparing it to that name could never pass. The pair
// is validated by index-pack --verify instead. Same asymmetry as the push
// side's publishPack/publishIdx.
func verifyDownloadedPacks(packDir string) error {
	entries, err := os.ReadDir(packDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".pack") {
			continue
		}
		p := filepath.Join(packDir, e.Name())
		// The repo package already owns both halves of this check, from the
		// push side: packContentChecksum(path) (string, error) recomputes the
		// pack's content hash, and packNameChecksum(base) string extracts the
		// hash the basename claims. There is no single checkPackChecksum.
		got, err := packContentChecksum(p)
		if err != nil {
			return fmt.Errorf("cannot checksum downloaded pack %s: %w", e.Name(), err)
		}
		if want := packNameChecksum(e.Name()); got != want {
			return fmt.Errorf("downloaded pack %s recomputes to %s; the name is the "+
				"content checksum, so this file is not what its name claims", e.Name(), got)
		}
		if _, err := os.Stat(strings.TrimSuffix(p, ".pack") + ".idx"); err != nil {
			return fmt.Errorf("downloaded pack %s has no adjacent .idx", e.Name())
		}
		if err := gitcmd.IndexPackVerify(p); err != nil {
			return fmt.Errorf("downloaded pair failed verification: %w", err)
		}
	}
	return nil
}

// consolidateAndInstall builds ONE pack from the new objects and installs it
// with an adjacent .keep. Git retains only the FIRST lock response, so a
// multi-pack install could not be protected.
//
// The .keep is written BEFORE the caller reports the lock, and git removes it
// once it has updated refs. If this process dies in between, the .keep
// remains and nothing will reclaim the pack — an inert residue that costs
// disk until a human removes the file.
func consolidateAndInstall(gitDir, altObjects, objs string) (string, error) {
	// Ask git where its object store is rather than guessing from the presence
	// of a .git directory. Guessing misfires on a bare repo, on a worktree
	// whose .git is a file, and on a normal repo that happens to have no
	// objects/pack yet — in that last case it would create one in the WORKING
	// TREE, which is silently wrong.
	gitDirAbs, code, err := gitcmd.RevParse(gitDir, "--git-dir")
	if err != nil || code != 0 {
		return "", fmt.Errorf("cannot locate the git directory for %s: %v", gitDir, err)
	}
	if !filepath.IsAbs(gitDirAbs) {
		gitDirAbs = filepath.Join(gitDir, gitDirAbs)
	}
	realPack := filepath.Join(gitDirAbs, "objects", "pack")
	if err := os.MkdirAll(realPack, 0o700); err != nil {
		return "", err
	}

	cmd := exec.Command("git", "-C", gitDir,
		"-c", "pack.packSizeLimit=0",
		"pack-objects", "--no-thin", "--index-version=2", "-q",
		filepath.Join(realPack, "pack"))
	cmd.Env = append(os.Environ(), "GIT_ALTERNATE_OBJECT_DIRECTORIES="+altObjects)
	cmd.Stdin = strings.NewReader(objs)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pack-objects: %s: %w", strings.TrimSpace(stderr.String()), err)
	}
	name := strings.TrimSpace(stdout.String())
	if strings.ContainsAny(name, " \t\r\n") {
		return "", fmt.Errorf("pack-objects emitted more than one pack (%q); the "+
			"one-pack invariant the lock response depends on is broken", name)
	}

	stem := filepath.Join(realPack, "pack-"+name)
	for _, ext := range []string{".pack", ".idx"} {
		if _, err := os.Stat(stem + ext); err != nil {
			return "", fmt.Errorf("pack-objects reported %s but %s is missing: %w", name, stem+ext, err)
		}
	}
	keep := stem + ".keep"
	if err := os.WriteFile(keep, []byte("git-remote-proton fetch\n"), 0o644); err != nil {
		return "", fmt.Errorf("cannot write %s: %w", keep, err)
	}
	return keep, nil
}
```

- [ ] **Step 4: Run to verify pass**

```bash
go test ./... && go vet ./... && gofmt -l .
```

Expected: PASS, `gofmt -l` silent.

- [ ] **Step 5: Commit**

```bash
git add internal/repo
git commit -m "feat(v2): fetch every pack, verify, then install one consolidated pack"
```

---

## Task 6 — wire the protocol: `fetch` capability, `list`, the batch

**Files:**
- Modify: `cmd/git-remote-proton/main.go`

**Interfaces:**
- Consumes: `repo.Fetch`, `repo.ListRefs`, `repo.ReadHEAD`.

The current capability block is `option\npush\n\n` and the option handler replies `unsupported` to everything.

**`check-connectivity` is a capability as well as an option.** Git recognises `connectivity-ok` only from a helper that advertised it, then sends `option check-connectivity true`, whose boolean must be tracked. Replying `unsupported` does not break the fetch — git falls back to its own check — but the parent design puts closure ownership on the helper.

**Do not add `check-connectivity` to the poison list.** Poison exists for options git *ignores our rejection of*; this is the opposite case.

- [ ] **Step 1: Advertise the new capabilities**

Replace the capabilities line:

```go
		case line == "capabilities":
			// check-connectivity is advertised as well as accepted: git only
			// recognises a connectivity-ok response from a helper that
			// advertised the capability.
			fmt.Fprint(out, "option\npush\nfetch\ncheck-connectivity\n\n")
			out.Flush()
```

- [ ] **Step 2: Track the connectivity option**

Add a variable beside `opts` in `loop`:

```go
	var checkConnectivity bool
```

and replace the option case:

```go
		case strings.HasPrefix(line, "option "):
			// check-connectivity is HONOURED, not poisoned: poison exists for
			// options git ignores our rejection of, and this is the opposite.
			if v, ok := strings.CutPrefix(line, "option check-connectivity "); ok {
				checkConnectivity = strings.TrimSpace(v) == "true"
				fmt.Fprint(out, "ok\n")
				out.Flush()
				continue
			}
			opts.Observe(line)
			fmt.Fprint(out, "unsupported\n") // advisory only; poison flag is the real defence
			out.Flush()
```

- [ ] **Step 3: Handle plain `list`**

Add a case before `list for-push`:

```go
		case line == "list":
			// The FETCH-side advertisement. Read-only: no Bootstrap, no lock.
			// A fetch must never bring a repository into existence, and a
			// lock here would wedge the repo for every reader if we crashed.
			refs, err := repo.ListRefs(t, root)
			if err != nil {
				warn(err)
				return 1
			}
			for name, sha := range refs {
				fmt.Fprintf(out, "%s %s\n", sha, name)
			}
			if branch, ok, err := repo.ReadHEAD(t, root); err != nil {
				warn(err)
				return 1
			} else if ok {
				// The symref line is what lets clone check something out.
				fmt.Fprintf(out, "@%s HEAD\n", branch)
			}
			fmt.Fprint(out, "\n")
			out.Flush()
```

- [ ] **Step 4: Handle the fetch batch**

Add a case beside the `push` one:

```go
		case strings.HasPrefix(line, "fetch "):
			var wants []string
			for l := line; ; {
				sp := strings.Fields(l)
				if len(sp) >= 2 {
					wants = append(wants, sp[1]) // "fetch <sha> <name>"
				}
				if !in.Scan() {
					break
				}
				l = in.Text()
				if l == "" {
					break
				}
			}
			keep, err := repo.Fetch(t, root, gitDir, wants)
			if err != nil {
				warn(err)
				return 1
			}
			if keep != "" {
				// Git retains only the FIRST lock, which is why the closure
				// is consolidated into one pack.
				fmt.Fprintf(out, "lock %s\n", keep)
			}
			if checkConnectivity {
				// Only after Fetch verified it. Fetch returns an error
				// otherwise, so reaching here means the closure is complete.
				fmt.Fprint(out, "connectivity-ok\n")
			}
			fmt.Fprint(out, "\n")
			out.Flush()
```

- [ ] **Step 5: Build and run the suite**

```bash
go build ./... && go test ./... && go vet ./... && gofmt -l .
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add cmd
git commit -m "feat(v2): advertise fetch, serve list, and handle the fetch batch"
```

---

## Task 7 — the live gate: clone and fetch against the real account

**Files:** none modified. This task runs the gate and records it.

**LIVE ACCOUNT RULES.** Every remote write stays under `/my-files/GitRemotes/v2s3a` and `/my-files/_cas-probe/contract`. You may create `/my-files/GitRemotes` and `/my-files/_cas-probe` if absent. **Off limits entirely — no writes, no trashing:** `Project Repo Bundles`, `ChatGPT Export Text Backup`, `GitBackups` (v1's, live), `Sensitive Project Sources`. Do not run `auth login`/`auth logout`. If you find yourself about to type any other remote path in a write or trash command, **stop and report**.

- [ ] **Step 1: Install the binary**

```bash
go build -o "$env:USERPROFILE/Tools/git-remote-proton.exe" ./cmd/git-remote-proton
```

- [ ] **Step 2: Run the live half of the contract table**

```bash
$env:GPB_LIVE_ACCOUNT=1; go test ./internal/transport/ -run TestContractCLI -v
```

Expected: every case PASSES against the real CLI. **Any failure here is a genuine fake/real divergence** — capture it verbatim and report rather than working around it. This is the check that exists because three such divergences shipped in Stage 2.

- [ ] **Step 3: Push a repo to fetch from**

```bash
cd $env:TEMP; rm -r -fo v2s3a -ErrorAction SilentlyContinue; mkdir v2s3a; cd v2s3a
git init -qb main; git config user.email t@t; git config user.name t
"one" > a.txt; git add .; git commit -qm "first"
git remote add proton-v2 proton::/my-files/GitRemotes/v2s3a
git push proton-v2 main
```

Expected: `ok refs/heads/main`.

- [ ] **Step 4: Confirm HEAD was written**

```bash
proton-drive filesystem info /my-files/GitRemotes/v2s3a/HEAD --json
proton-drive filesystem list /my-files/GitRemotes/v2s3a --json
```

Expected: `HEAD` exists alongside the marker, `refs/`, `packs/`. **If it does not, Task 4 did not work and the clone gate cannot pass** — report rather than proceeding.

- [ ] **Step 5: THE GATE — clone into a working checkout**

```bash
cd $env:TEMP; rm -r -fo v2s3aclone -ErrorAction SilentlyContinue
git clone -o proton-v2 proton::/my-files/GitRemotes/v2s3a v2s3aclone
cd v2s3aclone; git log --oneline; git status; cat a.txt; git fsck
```

Expected: the clone succeeds, `a.txt` is **present in the working tree**, `git log` shows the commit, `git status` reports a clean tree on `main`, and `git fsck` is clean. **A clone that fetches objects but checks out nothing is a failure, not a partial success.**

- [ ] **Step 6: Incremental fetch**

```bash
cd $env:TEMP/v2s3a; "two" >> a.txt; git commit -qam "second"; git push proton-v2 main
cd $env:TEMP/v2s3aclone; git fetch proton-v2; git log --oneline proton-v2/main
```

Expected: the fetch brings the second commit down. Then confirm `--not --all` is working:

```bash
ls .git/objects/pack
```

Expected: a small number of packs, **not** a fresh full-history pack per fetch. Record the count and sizes.

- [ ] **Step 7: Clean up**

```bash
proton-drive filesystem trash /my-files/GitRemotes/v2s3a
proton-drive filesystem list /my-files/GitRemotes --json
```

If the listing shows anything not created by this task, **stop and report** rather than trashing the parent. If empty:

```bash
proton-drive filesystem trash /my-files/GitRemotes
proton-drive filesystem list /my-files/_cas-probe --json
```

If `_cas-probe` is empty, trash it too. Then confirm `/my-files` holds exactly the four pre-existing folders.

- [ ] **Step 8: Commit the record**

```bash
git commit --allow-empty -m "test(v2): Stage 3a live gate - clone produces a working checkout"
```

---

## Self-Review

**Spec coverage.** Contract table → Task 2 (live half run in Task 7). `DeriveHEAD`/`WriteHEAD`/`ReadHEAD` → Task 3; the backfill rule and the "never rewrite" guard → Task 4. Fetch orchestration, per-member verification, verify-before-install, the temp `objects/pack/` layout, `--not --all`, one-pack consolidation and the `.keep` → Task 5. The `fetch` and `check-connectivity` capabilities, plain `list` with the `@… HEAD` symref, the batch, the single `lock`, `connectivity-ok` → Task 6. The read-only boundary and the no-lock rule → Tasks 5 and 6. The gate → Task 7.

**Interfaces are forward-consistent.** `SymbolicRef`/`ConnectivityOK`/`RevListNewObjects` are produced in Task 1 and consumed in Tasks 4 and 5. `DeriveHEAD`/`WriteHEAD`/`ReadHEAD` are produced in Task 3 and consumed in Tasks 4 and 6. `Fetch` is produced in Task 5 and consumed in Task 6. No task references a name no earlier task defines.

**Deliberately NOT in this plan:** the `.idx` cache, the object-to-pack map, iterative discovery and its termination rule, selective pack download (all Stage 3b); `git clone --reference`; shallow and partial beyond the existing poison flag; tag transitions beyond create; `refs/notes` and other namespaces; hierarchical ref names; the CLI version allowlist and the size ceilings (both Stage 4).

**What peer review changed about this plan.** The first draft would not have compiled and one of its tests would have been flaky by the clock. It called `checkPackChecksum`, which does not exist — the repo package has `packContentChecksum` and `packNameChecksum`. It called `die`, which does not exist in `main.go` — Stage 2.1 restructured that into `warn` plus a return code. It passed the repo root to `checkMarker`, which takes the marker's own path, so the transport would have tried to download a directory. It guessed the object store from the presence of `.git`, which misfires on a bare repo and could create `objects/pack` in the working tree. And most subtly: both `src` and `dst` came from `newGitRepoForPush`, which builds a byte-identical commit every time — and a commit sha covers timestamps at one-second resolution, so two repos made in the same second get the **same sha**, meaning `dst` already held what the test meant to fetch. `Fetch` would have returned "up to date", the RED assertion would have failed for an unrelated reason, and the failure would have come and gone with machine speed. Hence `emptyGitRepo`.

**Known judgment calls.** `ensureHEAD` warns rather than failing the push: refs and objects are already published and correct, and turning a good push into an error over a convenience symref is the wrong trade. `Fetch` returning `("", nil)` means up to date, which the caller must not mistake for a failure. `consolidateAndInstall` falls back to `<gitDir>/objects/pack` when `.git` is absent, so a bare repo works. The exact `rev-list` flag set is pinned by Task 1's test rather than asserted here — the spec deliberately left it open because nobody had run it, and a design asserting an unverified command line is how a plan inherits a wrong one.
