# git-remote-proton Stage 2 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `git push proton-v2 main` works end to end against a real Proton Drive account.

**Architecture:** A Go binary named `git-remote-proton` that git spawns when it sees a `proton::` URL. Three layers with hard boundaries: a **protocol** layer speaking git's line-oriented helper protocol on stdin/stdout, a **repo** layer deciding what moves, and a **transport** layer that is the only code aware Proton exists. Transport is an interface, so everything above it is tested against an in-memory fake with no network and no account.

**Tech Stack:** Go (stdlib only — no third-party dependencies), the Proton Drive CLI as subprocess transport, git plumbing (`rev-list`, `pack-objects`, `cat-file`) shelled out for all object work.

## Global Constraints

- **Design:** `docs/v2-remote-helper-design.md`. Read it first. It is settled through four peer-review rounds; do not re-litigate its decisions in code.
- **Transport contract:** `docs/research/probes/stage1-results.json` is **normative**. Certified on Proton Drive CLI **`cli-drive@0.7.0`** only. Behaviour differs on other builds.
- **Stdlib only.** No Go module dependencies. If something seems to need a library, it does not.
- **Remote name is `proton-v2`.** Never `proton` — v1 owns that and rewrites its URL.
- **stdout is protocol-only.** All diagnostics go to stderr. On Windows stdout must be binary mode (no CRLF translation) or git sees corrupted responses.
- **Fail closed.** Anything not confirmed is reported to git as failure.
- **Never trust `claimedDigests.sha1`** — Proton flags it `sha1Verified: false`.
- Commit style: repo-conventional prefixes, trailer `Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>`.
- Do **not** push; the user merges/pushes.
- Run Go tests with `go test ./...` from the repo root.

**Repo:** `C:\Users\craig\Projects\_Tools\git-proton-backup` (branch: create `feat/v2-stage2` from `main`).

---

## Task 0 — PREREQUISITE: Go toolchain (human-gated)

**Go is not installed on this machine.** Nothing in this plan can be built or tested until it is. This is a system-level software install and is the user's call.

- [ ] **Step 1: Install Go**

Download the current Windows amd64 MSI from <https://go.dev/dl/> and install, or:

```bash
winget install --id GoLang.Go --source winget
```

- [ ] **Step 2: Verify**

```bash
go version
```

Expected: `go version go1.2x.x windows/amd64`. If this fails, **stop** — every later task depends on it.

---

## File Structure

```
go.mod                                     module github.com/craigstoller/git-proton-backup
cmd/git-remote-proton/main.go              entry point; stdio wiring, binary stdout
internal/protocol/protocol.go              helper protocol reader/writer; knows no Proton
internal/protocol/protocol_test.go
internal/transport/transport.go            Transport interface, Outcome, Node
internal/transport/fake.go                 in-memory implementation for tests
internal/transport/cli.go                  Proton CLI implementation (the only Proton-aware file)
internal/transport/cli_test.go             parser tests against captured Stage 1 payloads
internal/gitcmd/gitcmd.go                  git plumbing wrapper (rev-list, pack-objects, cat-file)
internal/gitcmd/gitcmd_test.go
internal/repo/marker.go                    format marker + bootstrap ordering
internal/repo/lock.go                      lock lifecycle
internal/repo/refs.go                      ref transition table
internal/repo/push.go                      push orchestration
internal/repo/repo_test.go                 all repo tests, against the fake transport
```

Files split by responsibility, not layer depth. `cli.go` is the single file that changes if the CLI changes; `protocol.go` is the single file that changes if git's protocol changes.

---

## Task 1 — Scaffold and a binary git can spawn

**Files:**
- Create: `go.mod`, `cmd/git-remote-proton/main.go`, `.gitignore` (append)

**Interfaces:**
- Produces: a compiled `git-remote-proton` binary that answers `capabilities` and exits cleanly on a blank line.

- [ ] **Step 1: Initialise the module**

```bash
cd C:\Users\craig\Projects\_Tools\git-proton-backup
git checkout -b feat/v2-stage2
go mod init github.com/craigstoller/git-proton-backup
```

- [ ] **Step 2: Write `cmd/git-remote-proton/main.go`**

```go
package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	// git passes: git-remote-proton <remote-name> <url>
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "git-remote-proton: must be run by git as a remote helper")
		os.Exit(1)
	}
	in := bufio.NewScanner(os.Stdin)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	for in.Scan() {
		switch line := in.Text(); line {
		case "capabilities":
			// Only what Stage 2 implements. `option` is advertised because the
			// unsupported-option state machine depends on receiving them.
			fmt.Fprint(out, "option\npush\n\n")
			out.Flush()
		case "":
			return
		default:
			fmt.Fprintf(os.Stderr, "unsupported command: %q\n", line)
			os.Exit(1)
		}
	}
}
```

- [ ] **Step 3: Build and drive it by hand**

```bash
go build ./cmd/git-remote-proton
printf 'capabilities\n\n' | ./git-remote-proton.exe proton-v2 proton::/my-files/x
```

Expected: prints `option`, `push`, a blank line, then exits 0.

- [ ] **Step 4: Ignore build output**

Append to `.gitignore`:

```
/git-remote-proton
/git-remote-proton.exe
```

- [ ] **Step 5: Commit**

```bash
git add go.mod cmd .gitignore
git commit -m "feat(v2): scaffold git-remote-proton binary with capabilities"
```

---

## Task 2 — Protocol layer: parse a push batch

**Files:**
- Create: `internal/protocol/protocol.go`, `internal/protocol/protocol_test.go`

**Interfaces:**
- Produces:
  - `type RefUpdate struct { Src, Dst string; Force bool }`
  - `func ParsePushBatch(lines []string) ([]RefUpdate, error)`
  - `type Options struct { Poisoned string }` — non-empty means a batch must be refused
  - `func (o *Options) Observe(line string)` — records unsupported safety options

The **`+` prefix is the only forced-push signal**; there is no force option. Getting this wrong silently turns every forced push into a normal one, or worse.

- [ ] **Step 1: Write the failing tests**

```go
package protocol

import "testing"

func TestParsePushBatch(t *testing.T) {
	got, err := ParsePushBatch([]string{
		"push refs/heads/main:refs/heads/main",
		"push +refs/heads/dev:refs/heads/dev",
		"push :refs/heads/gone",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 updates, got %d", len(got))
	}
	if got[0].Force {
		t.Error("plain push must not be forced")
	}
	if !got[1].Force || got[1].Src != "refs/heads/dev" {
		t.Errorf("+ prefix must set Force and be stripped, got %+v", got[1])
	}
	if got[2].Src != "" || got[2].Dst != "refs/heads/gone" {
		t.Errorf("empty src means delete, got %+v", got[2])
	}
}

func TestOptionsPoison(t *testing.T) {
	var o Options
	o.Observe("option cas refs/heads/main:abc123")
	if o.Poisoned == "" {
		t.Error("cas must poison the session: git ignores our rejection and pushes anyway")
	}
	var o2 Options
	o2.Observe("option verbosity 2")
	if o2.Poisoned != "" {
		t.Error("benign options must not poison")
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/protocol/
```

Expected: FAIL, undefined `ParsePushBatch`.

- [ ] **Step 3: Implement**

```go
package protocol

import (
	"fmt"
	"strings"
)

type RefUpdate struct {
	Src   string // empty means delete
	Dst   string
	Force bool
}

func ParsePushBatch(lines []string) ([]RefUpdate, error) {
	var out []RefUpdate
	for _, l := range lines {
		if !strings.HasPrefix(l, "push ") {
			return nil, fmt.Errorf("not a push line: %q", l)
		}
		spec := strings.TrimPrefix(l, "push ")
		u := RefUpdate{}
		if strings.HasPrefix(spec, "+") {
			u.Force = true
			spec = spec[1:]
		}
		parts := strings.SplitN(spec, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("malformed refspec: %q", spec)
		}
		u.Src, u.Dst = parts[0], parts[1]
		if u.Dst == "" {
			return nil, fmt.Errorf("empty destination in %q", spec)
		}
		out = append(out, u)
	}
	return out, nil
}

// poisonOptions are options git sends whose REJECTION IT IGNORES. Replying
// "unsupported" does not stop the operation, so the batch must be refused later.
// Only `atomic` is genuinely honoured at the option response.
var poisonOptions = []string{
	"cas", "depth", "deepen-since", "deepen-not", "deepen-relative",
	"update-shallow", "filter", "from-promisor", "no-dependents", "refetch",
}

type Options struct {
	Poisoned string // name of the first unsupported safety option seen
}

func (o *Options) Observe(line string) {
	if !strings.HasPrefix(line, "option ") || o.Poisoned != "" {
		return
	}
	name := strings.Fields(strings.TrimPrefix(line, "option "))
	if len(name) == 0 {
		return
	}
	for _, p := range poisonOptions {
		if name[0] == p {
			o.Poisoned = p
			return
		}
	}
}
```

- [ ] **Step 4: Run to verify pass**

```bash
go test ./internal/protocol/
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/protocol
git commit -m "feat(v2): parse push batches; poison unsupported safety options"
```

---

## Task 3 — Transport interface and in-memory fake

**Files:**
- Create: `internal/transport/transport.go`, `internal/transport/fake.go`

**Interfaces:**
- Produces:
  - `type Outcome int` with `Committed`, `Refused`, `Ambiguous`
  - `type Node struct { Name string; IsDir bool; Size int64 }`
  - `type Transport interface { ... }` — exact signatures below
  - `func NewFake() *Fake` — implements `Transport` in memory

- [ ] **Step 1: Write `transport.go`**

```go
package transport

type Outcome int

const (
	Committed Outcome = iota // definitely applied
	Refused                  // name existed; nothing changed
	Ambiguous                // unknown; MUST be reconciled by reading remote state
)

func (o Outcome) String() string {
	switch o {
	case Committed:
		return "committed"
	case Refused:
		return "refused"
	default:
		return "ambiguous"
	}
}

type Node struct {
	Name  string
	IsDir bool
	Size  int64
}

type Transport interface {
	// EnsureDir is Stat-then-create: create-folder FAILS on an existing folder
	// (Stage 1 C5), so a bare create would error on every run after the first.
	EnsureDir(path string) error
	List(path string) ([]Node, error)
	Stat(path string) (Node, bool, error) // absence is (_, false, nil), never an error
	// ReadTo downloads the node at path INTO the existing local directory
	// localDir, as a file named after the node's own remote basename. It is
	// never a destination FILE path: the CLI's `download` takes a folder.
	ReadTo(path, localDir string) error
	CreateExclusive(path, localPath string) (Outcome, error)
	UpdateRevision(path, localPath string) (Outcome, error)
	// Trash on a MISSING target fails with exit 1 (Stage 1 C4), so implementations
	// must Stat first and report Committed for an already-absent node.
	Trash(path string) (Outcome, error)
}
```

- [ ] **Step 2: Write `fake.go`**

```go
package transport

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Fake is an in-memory Transport. Everything above the transport layer is
// tested against this — no network, no Proton account.
type Fake struct {
	Files map[string][]byte
	Dirs  map[string]bool
	// FailNext, when non-empty, makes the next mutation return Ambiguous.
	FailNext string
}

func NewFake() *Fake {
	return &Fake{Files: map[string][]byte{}, Dirs: map[string]bool{}}
}

func (f *Fake) EnsureDir(p string) error { f.Dirs[p] = true; return nil }

func (f *Fake) List(p string) ([]Node, error) {
	var out []Node
	seen := map[string]bool{}
	for k := range f.Files {
		if path.Dir(k) == p {
			out = append(out, Node{Name: path.Base(k), Size: int64(len(f.Files[k]))})
		} else if strings.HasPrefix(k, p+"/") {
			rest := strings.TrimPrefix(k, p+"/")
			d := strings.SplitN(rest, "/", 2)[0]
			if !seen[d] {
				seen[d] = true
				out = append(out, Node{Name: d, IsDir: true})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (f *Fake) Stat(p string) (Node, bool, error) {
	if b, ok := f.Files[p]; ok {
		return Node{Name: path.Base(p), Size: int64(len(b))}, true, nil
	}
	if f.Dirs[p] {
		return Node{Name: path.Base(p), IsDir: true}, true, nil
	}
	return Node{}, false, nil
}

// ReadTo mirrors the CLI: localDir is an existing DIRECTORY, and the node
// lands inside it under its own remote basename. Writing straight to localDir
// as if it were a file path is the bug this fake originally shipped — latent
// until Task 7's readLock became the first caller. path.Base, not
// filepath.Base: remote paths are POSIX regardless of host OS.
func (f *Fake) ReadTo(p, localDir string) error {
	b, ok := f.Files[p]
	if !ok {
		return fmt.Errorf("not found: %s", p)
	}
	return os.WriteFile(filepath.Join(localDir, path.Base(p)), b, 0o644)
}

func (f *Fake) CreateExclusive(p, local string) (Outcome, error) {
	if f.FailNext != "" {
		f.FailNext = ""
		return Ambiguous, nil
	}
	if _, ok := f.Files[p]; ok {
		return Refused, nil
	}
	b, err := os.ReadFile(local)
	if err != nil {
		return Ambiguous, err
	}
	f.Files[p] = b
	return Committed, nil
}

func (f *Fake) UpdateRevision(p, local string) (Outcome, error) {
	b, err := os.ReadFile(local)
	if err != nil {
		return Ambiguous, err
	}
	// Mirrors the real CLI: a byte-identical rewrite is silently skipped
	// (Stage 1 C2). Callers must verify by read-back, not by outcome.
	if cur, ok := f.Files[p]; ok && string(cur) == string(b) {
		return Refused, nil
	}
	f.Files[p] = b
	return Committed, nil
}

func (f *Fake) Trash(p string) (Outcome, error) {
	if _, ok := f.Files[p]; !ok {
		return Committed, nil // already absent is the desired end state
	}
	delete(f.Files, p)
	return Committed, nil
}
```

- [ ] **Step 3: Verify it satisfies the interface**

```bash
go vet ./internal/transport/
go build ./...
```

Expected: no output. (`var _ Transport = (*Fake)(nil)` is unnecessary — `go vet` plus later use proves it.)

- [ ] **Step 4: Commit**

```bash
git add internal/transport
git commit -m "feat(v2): transport interface and in-memory fake"
```

---

## Task 4 — CLI transport: read side

**Files:**
- Create: `internal/transport/cli.go`, `internal/transport/cli_test.go`

**Interfaces:**
- Consumes: `Transport`, `Node`, `Outcome` from Task 3.
- Produces: `func NewCLI(exe string) *CLI` implementing `Stat`, `List`, `ReadTo`, `EnsureDir`.
- Produces: `func parseNodeJSON(b []byte) (Node, error)` — handles **both** `activeRevision` shapes.

The payload shape changed between CLI versions: 0.4.6 wraps it as `{ok, value}`, 0.7.0 does not. v1 shipped a bug from reading only the wrapped form. Handle both.

- [ ] **Step 1: Write the failing test**

```go
package transport

import "testing"

const unwrapped = `{"uid":"u1","name":{"ok":true,"value":"x.bundle"},"type":"file",
 "activeRevision":{"uid":"r1","state":"active","claimedSize":8}}`

const wrapped = `{"uid":"u1","name":{"ok":true,"value":"x.bundle"},"type":"file",
 "activeRevision":{"ok":true,"value":{"uid":"r1","state":"active","claimedSize":8}}}`

func TestParseNodeJSONBothShapes(t *testing.T) {
	for name, payload := range map[string]string{"0.7.0": unwrapped, "0.4.6": wrapped} {
		n, err := parseNodeJSON([]byte(payload))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if n.Name != "x.bundle" || n.Size != 8 {
			t.Errorf("%s: got %+v", name, n)
		}
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/transport/
```

Expected: FAIL, undefined `parseNodeJSON`.

- [ ] **Step 3: Implement `cli.go` read side**

```go
package transport

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type CLI struct{ Exe string }

func NewCLI(exe string) *CLI {
	if exe == "" {
		exe = "proton-drive"
	}
	return &CLI{Exe: exe}
}

func (c *CLI) run(args ...string) (string, int, error) {
	cmd := exec.Command(c.Exe, args...)
	out, err := cmd.CombinedOutput()
	code := cmd.ProcessState.ExitCode()
	return string(out), code, err
}

// nodeWire mirrors the CLI payload. activeRevision is json.RawMessage because
// its shape differs by version and must be decoded in two attempts.
type nodeWire struct {
	Name struct {
		Value string `json:"value"`
	} `json:"name"`
	Type           string          `json:"type"`
	ActiveRevision json.RawMessage `json:"activeRevision"`
}

type revWire struct {
	State       string `json:"state"`
	ClaimedSize int64  `json:"claimedSize"`
}

func parseNodeJSON(b []byte) (Node, error) {
	var w nodeWire
	if err := json.Unmarshal(b, &w); err != nil {
		return Node{}, err
	}
	n := Node{Name: w.Name.Value, IsDir: w.Type == "folder"}
	if len(w.ActiveRevision) > 0 {
		var r revWire
		// 0.7.0: unwrapped.
		if err := json.Unmarshal(w.ActiveRevision, &r); err == nil && r.State != "" {
			n.Size = r.ClaimedSize
			return n, nil
		}
		// 0.4.6: {ok, value}.
		var wrap struct {
			OK    bool    `json:"ok"`
			Value revWire `json:"value"`
		}
		if err := json.Unmarshal(w.ActiveRevision, &wrap); err == nil && wrap.OK {
			n.Size = wrap.Value.ClaimedSize
		}
	}
	return n, nil
}

func (c *CLI) Stat(p string) (Node, bool, error) {
	out, code, _ := c.run("filesystem", "info", p, "--json")
	if code != 0 {
		return Node{}, false, nil // absence is not an error
	}
	n, err := parseNodeJSON([]byte(out))
	if err != nil {
		return Node{}, false, fmt.Errorf("unparseable info for %s: %w", p, err)
	}
	return n, true, nil
}

func (c *CLI) List(p string) ([]Node, error) {
	out, code, _ := c.run("filesystem", "list", p, "--json")
	if code != 0 {
		return nil, fmt.Errorf("list %s failed: %s", p, strings.TrimSpace(out))
	}
	if strings.TrimSpace(out) == "" {
		return []Node{}, nil // empty folder: exit 0, no output (Stage 1 C6)
	}
	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("unparseable listing for %s: %w", p, err)
	}
	nodes := make([]Node, 0, len(raw))
	for _, r := range raw {
		n, err := parseNodeJSON(r)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func (c *CLI) ReadTo(p, localDir string) error {
	out, code, _ := c.run("filesystem", "download", p, localDir)
	if code != 0 {
		return fmt.Errorf("download %s failed: %s", p, strings.TrimSpace(out))
	}
	return nil
}

// EnsureDir is Stat-then-create: create-folder exits 1 on an existing folder
// (Stage 1 C5). Swallowing that error generically would also hide real
// permission and path failures, so the existence check is explicit.
func (c *CLI) EnsureDir(p string) error {
	if _, ok, err := c.Stat(p); err != nil {
		return err
	} else if ok {
		return nil
	}
	i := strings.LastIndex(p, "/")
	if i <= 0 {
		return fmt.Errorf("refusing to create a root-level folder: %s", p)
	}
	parent, name := p[:i], p[i+1:]
	out, code, _ := c.run("filesystem", "create-folder", parent, name)
	if code != 0 {
		return fmt.Errorf("create-folder %s in %s failed: %s", name, parent, strings.TrimSpace(out))
	}
	return nil
}
```

- [ ] **Step 4: Run to verify pass**

```bash
go test ./internal/transport/
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/transport
git commit -m "feat(v2): CLI transport read side, both activeRevision shapes"
```

---

## Task 5 — CLI transport: write side

**Files:**
- Modify: `internal/transport/cli.go`
- Modify: `internal/transport/cli_test.go`

**Interfaces:**
- Produces: `CreateExclusive`, `UpdateRevision`, `Trash` on `*CLI`.
- Produces: `func classifyUpload(transferred, skipped, failed int) Outcome`.

Count tuples are **exact**, not `>=`. For a single-file operation the CLI records exactly one of success, skip, or failure, so `(1,1,0)` is contradictory and must be `Ambiguous`. **Exit code cannot distinguish success from refusal — both are 0.**

- [ ] **Step 1: Write the failing test**

```go
func TestClassifyUpload(t *testing.T) {
	cases := []struct {
		t, s, f int
		want    Outcome
	}{
		{1, 0, 0, Committed},
		{0, 1, 0, Refused},
		{0, 0, 1, Ambiguous}, // a reported failure needs reconciliation
		{1, 1, 0, Ambiguous}, // contradictory for one file
		{0, 0, 0, Ambiguous}, // nothing happened; unknown
		{0, 0, 2, Ambiguous},
	}
	for _, c := range cases {
		if got := classifyUpload(c.t, c.s, c.f); got != c.want {
			t.Errorf("(%d,%d,%d): got %v want %v", c.t, c.s, c.f, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/transport/ -run TestClassifyUpload
```

Expected: FAIL, undefined `classifyUpload`.

- [ ] **Step 3: Implement**

```go
type transferSummary struct {
	Transferred int `json:"transferredItems"`
	Skipped     int `json:"skippedItems"`
	Failed      int `json:"failedItems"`
}

func classifyUpload(transferred, skipped, failed int) Outcome {
	switch {
	case transferred == 1 && skipped == 0 && failed == 0:
		return Committed
	case transferred == 0 && skipped == 1 && failed == 0:
		return Refused
	default:
		return Ambiguous
	}
}

func (c *CLI) upload(strategy, remoteDir, localFile string) (Outcome, error) {
	out, _, _ := c.run("filesystem", "upload", "-f", strategy, "--json", localFile, remoteDir)
	var s transferSummary
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &s); err != nil {
		// Unparseable output is Ambiguous, never assumed-failed: the write may
		// have landed. Callers reconcile by reading remote state.
		return Ambiguous, fmt.Errorf("unparseable upload summary: %s", strings.TrimSpace(out))
	}
	return classifyUpload(s.Transferred, s.Skipped, s.Failed), nil
}

// CreateExclusive and UpdateRevision both pass only dirOf(p) to the CLI,
// because `filesystem upload` takes a PARENT path and has no --name flag: the
// remote node is named after localFile's OWN basename (probe C11). CALLER
// CONTRACT: filepath.Base(localFile) MUST equal the leaf of p, or the write
// lands under the wrong remote name. repo.stagedFile is what guarantees it.
func (c *CLI) CreateExclusive(p, localFile string) (Outcome, error) {
	return c.upload("skip", dirOf(p), localFile)
}

// UpdateRevision maps to `merge`, which revises the existing node in place
// (Stage 1 C1: node uid stable, revision uid changes). NOT `replace`, which
// trashes the node before creating the new one and can destroy a ref on crash.
func (c *CLI) UpdateRevision(p, localFile string) (Outcome, error) {
	return c.upload("merge", dirOf(p), localFile)
}

// Trash exits 1 on a missing target (Stage 1 C4), so absence is checked first.
func (c *CLI) Trash(p string) (Outcome, error) {
	if _, ok, err := c.Stat(p); err != nil {
		return Ambiguous, err
	} else if !ok {
		return Committed, nil
	}
	out, code, _ := c.run("filesystem", "trash", p, "--json")
	if code != 0 {
		return Ambiguous, fmt.Errorf("trash %s failed: %s", p, strings.TrimSpace(out))
	}
	return Committed, nil
}

func dirOf(p string) string {
	if i := strings.LastIndex(p, "/"); i > 0 {
		return p[:i]
	}
	return "/"
}
```

- [ ] **Step 4: Run to verify pass**

```bash
go test ./internal/transport/
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/transport
git commit -m "feat(v2): CLI transport write side with exact count-tuple classification"
```

---

## Task 6 — Marker and bootstrap ordering

**Files:**
- Create: `internal/repo/marker.go`, `internal/repo/repo_test.go`

**Interfaces:**
- Consumes: `transport.Transport`.
- Produces: `func Bootstrap(t transport.Transport, root string) error`.

The ordering is load-bearing. Acquiring the lock first creates `.lock`, which makes an empty folder non-empty — so a marker check that treats "non-empty and markerless" as a hard refusal would brick the remote on its first push. **The marker is written before the lock, and `.lock` is excluded from the emptiness test.**

- [ ] **Step 1: Write the failing tests**

```go
package repo

import (
	"testing"

	"github.com/craigstoller/git-proton-backup/internal/transport"
)

func TestBootstrapEmptyRemote(t *testing.T) {
	f := transport.NewFake()
	if err := Bootstrap(f, "/my-files/r"); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if _, ok := f.Files["/my-files/r/gpb-remote.json"]; !ok {
		t.Error("marker must be written")
	}
}

func TestBootstrapIgnoresLockWhenTestingEmptiness(t *testing.T) {
	f := transport.NewFake()
	f.Files["/my-files/r/.lock"] = []byte(`{"nonce":"n"}`)
	if err := Bootstrap(f, "/my-files/r"); err != nil {
		t.Fatalf("a lone .lock must not count as foreign data: %v", err)
	}
}

func TestBootstrapRefusesForeignData(t *testing.T) {
	f := transport.NewFake()
	f.Files["/my-files/r/taxes.pdf"] = []byte("x")
	if err := Bootstrap(f, "/my-files/r"); err == nil {
		t.Error("must refuse a non-empty folder with no marker")
	}
}

func TestBootstrapIdempotent(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/my-files/r")
	if err := Bootstrap(f, "/my-files/r"); err != nil {
		t.Errorf("second bootstrap must be a no-op: %v", err)
	}
}

func TestStagedFileUsesTheLeafNameAndRefusesHostileOnes(t *testing.T) {
	// The CLI names the uploaded node after the LOCAL basename (probe C11),
	// so the staged file must BE the leaf name, not a neutral one.
	p, cleanup, err := stagedFile([]byte("x"), "main")
	if err != nil {
		t.Fatalf("staging a plain leaf must succeed: %v", err)
	}
	defer cleanup()
	if filepath.Base(p) != "main" {
		t.Errorf("staged basename must equal the leaf, got %q", filepath.Base(p))
	}
	if b, err := os.ReadFile(p); err != nil || string(b) != "x" {
		t.Errorf("staged content = %q, %v", b, err)
	}

	for _, bad := range []string{"a{b,c}", "con", "nul.txt", "", "..", "a/b"} {
		if _, _, err := stagedFile([]byte("x"), bad); err == nil {
			t.Errorf("%q must be refused with a reason, not mangled", bad)
		}
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/repo/
```

Expected: FAIL, undefined `Bootstrap`.

- [ ] **Step 3: Implement `marker.go`**

```go
package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/transport"
)

const (
	MarkerName    = "gpb-remote.json"
	LockName      = ".lock"
	markerContent = `{"format":"git-remote-proton","version":1}`
)

func Bootstrap(t transport.Transport, root string) error {
	marker := root + "/" + MarkerName
	if _, ok, err := t.Stat(marker); err != nil {
		return err
	} else if ok {
		return ensureSubdirs(t, root)
	}

	if err := t.EnsureDir(root); err != nil {
		return err
	}
	nodes, err := t.List(root)
	if err != nil {
		return err
	}
	for _, n := range nodes {
		// .lock is our own scaffolding, not repo content. Counting it would
		// make a first push refuse itself after taking the lock.
		if n.Name != LockName {
			return fmt.Errorf("refusing to use %s: it is not empty and has no %s", root, MarkerName)
		}
	}

	staged, cleanup, err := stagedFile([]byte(markerContent), MarkerName)
	if err != nil {
		return err
	}
	defer cleanup()

	switch out, err := t.CreateExclusive(marker, staged); {
	case err != nil:
		return err
	case out == transport.Refused:
		// A concurrent initialiser won. That is fine; adopt their repo.
	case out == transport.Ambiguous:
		return fmt.Errorf("marker creation ambiguous for %s; re-run to reconcile", root)
	}
	return ensureSubdirs(t, root)
}

func ensureSubdirs(t transport.Transport, root string) error {
	for _, d := range []string{"refs", "refs/heads", "refs/tags", "packs"} {
		if err := t.EnsureDir(root + "/" + d); err != nil {
			return err
		}
	}
	return nil
}

// stagedFile writes content into a private temp directory under leafName and
// returns that local path, plus a cleanup func.
//
// The local basename MUST equal the target's remote leaf name. `filesystem
// upload` takes a PARENT path and has no --name flag, so the CLI names the
// uploaded node after the LOCAL file (probe C11). Neutral staging — which an
// earlier design revision specified — is therefore not expressible, and
// upload-then-rename cannot serve UpdateRevision (probe C12).
//
// The cost is that a ref name does appear in a local path, so names hostile to
// one are rejected here with a reason instead of being silently mangled.
func stagedFile(content []byte, leafName string) (string, func(), error) {
	noop := func() {}
	if err := checkStageableLeaf(leafName); err != nil {
		return "", noop, err
	}
	dir, err := os.MkdirTemp("", "gpb-stage-*")
	if err != nil {
		return "", noop, err
	}
	cleanup := func() { os.RemoveAll(dir) }
	p := filepath.Join(dir, leafName)
	if err := os.WriteFile(p, content, 0o600); err != nil {
		cleanup()
		return "", noop, err
	}
	return p, cleanup, nil
}

// windowsReserved are DOS device names. Windows refuses them as filenames on
// every host, and git accepts them as ref names.
var windowsReserved = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// checkStageableLeaf rejects leaf names that cannot survive a local staging
// path. The set is small because git already forbids space, control characters,
// '?', '*', '[', '~', '^' and ':' in ref names; what remains is brace globbing
// (probe C13: 0.7.0 still glob-expands '{') and Windows device names. Refusing
// these is also consistent — such a ref could never be UPDATED on this
// transport, so accepting the create would promise what the update cannot keep.
func checkStageableLeaf(leaf string) error {
	if leaf == "" || leaf == "." || leaf == ".." {
		return fmt.Errorf("refusing to stage the name %q", leaf)
	}
	if strings.ContainsAny(leaf, `{}/\`) {
		return fmt.Errorf("%q cannot be expressed as a local staging path", leaf)
	}
	stem := strings.ToLower(leaf)
	if i := strings.IndexByte(stem, '.'); i >= 0 {
		stem = stem[:i]
	}
	if windowsReserved[stem] {
		return fmt.Errorf("%q is a reserved device name on Windows", leaf)
	}
	return nil
}
```

- [ ] **Step 4: Run to verify pass**

```bash
go test ./internal/repo/
```

Expected: PASS (5 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/repo
git commit -m "feat(v2): marker bootstrap with lock-aware emptiness test"
```

---

## Task 7 — Lock lifecycle

**Files:**
- Create: `internal/repo/lock.go`
- Modify: `internal/repo/repo_test.go`

**Interfaces:**
- Produces: `func AcquireLock(t transport.Transport, root string) (*Lock, error)` and `func (l *Lock) Release() error`.

Acquisition must happen **before** the ref advertisement, or a stale view can overwrite a concurrent writer even with both parties respecting the lock. Identity is a **nonce**, not a hostname — two processes on one machine are otherwise indistinguishable. **v2 has no takeover**: release cannot be made conditional on this transport, so a stale lock is reported for a human to clear.

- [ ] **Step 1: Write the failing tests**

```go
func TestLockAcquireAndRelease(t *testing.T) {
	f := transport.NewFake()
	l, err := AcquireLock(f, "/my-files/r")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if _, ok := f.Files["/my-files/r/.lock"]; !ok {
		t.Fatal("lock file must exist while held")
	}
	if err := l.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, ok := f.Files["/my-files/r/.lock"]; ok {
		t.Error("lock file must be gone after release")
	}
}

func TestLockRefusesWhenHeld(t *testing.T) {
	f := transport.NewFake()
	if _, err := AcquireLock(f, "/my-files/r"); err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireLock(f, "/my-files/r"); err == nil {
		t.Error("second acquire must fail while the first holds it")
	}
}

func TestReleaseDoesNotDeleteSomeoneElsesLock(t *testing.T) {
	f := transport.NewFake()
	l, _ := AcquireLock(f, "/my-files/r")
	f.Files["/my-files/r/.lock"] = []byte(`{"nonce":"someone-else"}`)
	_ = l.Release()
	if _, ok := f.Files["/my-files/r/.lock"]; !ok {
		t.Error("must not delete a lock whose nonce does not match ours")
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/repo/ -run TestLock
```

Expected: FAIL, undefined `AcquireLock`.

- [ ] **Step 3: Implement `lock.go`**

```go
package repo

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/craigstoller/git-proton-backup/internal/transport"
)

type lockBody struct {
	Nonce      string `json:"nonce"`
	Host       string `json:"host"`
	PID        int    `json:"pid"`
	AcquiredAt string `json:"acquiredAt"`
}

type Lock struct {
	t     transport.Transport
	path  string
	nonce string
}

func AcquireLock(t transport.Transport, root string) (*Lock, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	nonce := hex.EncodeToString(b)
	host, _ := os.Hostname()
	body, _ := json.Marshal(lockBody{nonce, host, os.Getpid(), time.Now().UTC().Format(time.RFC3339)})

	// Staged under the lock's own leaf name: the CLI names the uploaded node
	// after the LOCAL basename (probe C11), so it must match.
	staged, cleanup, err := stagedFile(body, LockName)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	p := root + "/" + LockName
	out, err := t.CreateExclusive(p, staged)
	if err != nil {
		return nil, err
	}
	switch out {
	case transport.Refused:
		if held, ok, _ := readLock(t, p); ok {
			return nil, fmt.Errorf("repo is locked by %s (pid %d) since %s; "+
				"if that process is gone, remove %s with the Proton CLI",
				held.Host, held.PID, held.AcquiredAt, p)
		}
		return nil, fmt.Errorf("repo is locked; remove %s with the Proton CLI if stale", p)
	case transport.Ambiguous:
		return nil, fmt.Errorf("lock acquisition ambiguous; re-run to reconcile")
	}

	// Verify by read-back: a byte-identical write is silently skipped by the
	// CLI, so Committed alone does not prove OUR body landed.
	if held, ok, err := readLock(t, p); err != nil {
		return nil, err
	} else if !ok || held.Nonce != nonce {
		return nil, fmt.Errorf("lock read-back mismatch; another writer holds %s", p)
	}
	return &Lock{t: t, path: p, nonce: nonce}, nil
}

// Release is check-then-act and cannot be made atomic on this transport. If a
// human clears a stale lock and another process acquires it in the gap, this
// can still delete the newcomer's lock. Narrow, documented, and unavoidable
// without a conditional delete.
func (l *Lock) Release() error {
	held, ok, err := readLock(l.t, l.path)
	if err != nil {
		return err
	}
	if !ok || held.Nonce != l.nonce {
		return nil // not ours any more; leave it alone
	}
	_, err = l.t.Trash(l.path)
	return err
}

func readLock(t transport.Transport, p string) (lockBody, bool, error) {
	if _, ok, err := t.Stat(p); err != nil || !ok {
		return lockBody{}, false, err
	}
	dir, err := os.MkdirTemp("", "gpb-lock-*")
	if err != nil {
		return lockBody{}, false, err
	}
	defer os.RemoveAll(dir)
	if err := t.ReadTo(p, dir); err != nil {
		return lockBody{}, false, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return lockBody{}, false, err
	}
	raw, err := os.ReadFile(dir + string(os.PathSeparator) + entries[0].Name())
	if err != nil {
		return lockBody{}, false, err
	}
	var lb lockBody
	if err := json.Unmarshal(raw, &lb); err != nil {
		return lockBody{}, false, nil // unreadable lock is treated as held by someone
	}
	return lb, true, nil
}
```

- [ ] **Step 4: Run to verify pass**

```bash
go test ./internal/repo/
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repo
git commit -m "feat(v2): nonce-identified lock, no takeover, read-back verified"
```

---

## Task 8 — git plumbing wrapper

**Files:**
- Create: `internal/gitcmd/gitcmd.go`, `internal/gitcmd/gitcmd_test.go`

**Interfaces:**
- Produces:
  - `func ObjectType(gitDir, sha string) (string, error)` — `git cat-file -t`
  - `func IsAncestor(gitDir, old, new string) (bool, error)`
  - `func HasObject(gitDir, sha string) bool`
  - `func WritePack(gitDir, want string, haves []string, outDir string) (packPath, idxPath string, err error)`

Object type must be resolved locally, because git hands the helper only hashes — the "branch target must be a commit" rule is unenforceable without it.

- [ ] **Step 1: Write the failing test**

```go
package gitcmd

import (
	"os"
	"os/exec"
	"testing"
)

func newRepo(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	for _, a := range [][]string{
		{"init", "-qb", "main"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"},
	} {
		if err := exec.Command("git", append([]string{"-C", d}, a...)...).Run(); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(d+"/a.txt", []byte("one"), 0o644)
	exec.Command("git", "-C", d, "add", ".").Run()
	exec.Command("git", "-C", d, "commit", "-qm", "c1").Run()
	return d
}

func TestObjectTypeAndAncestry(t *testing.T) {
	d := newRepo(t)
	sha, _ := exec.Command("git", "-C", d, "rev-parse", "HEAD").Output()
	head := string(sha[:40])

	if got, err := ObjectType(d, head); err != nil || got != "commit" {
		t.Fatalf("ObjectType = %q, %v; want commit", got, err)
	}
	if !HasObject(d, head) {
		t.Error("HasObject must find HEAD")
	}
	if ok, _ := IsAncestor(d, head, head); !ok {
		t.Error("a commit is its own ancestor")
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/gitcmd/
```

Expected: FAIL, undefined `ObjectType`.

- [ ] **Step 3: Implement**

```go
package gitcmd

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func git(gitDir string, args ...string) (string, int, error) {
	cmd := exec.Command("git", append([]string{"-C", gitDir}, args...)...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), cmd.ProcessState.ExitCode(), err
}

func ObjectType(gitDir, sha string) (string, error) {
	out, code, _ := git(gitDir, "cat-file", "-t", sha)
	if code != 0 {
		return "", fmt.Errorf("cat-file -t %s: %s", sha, out)
	}
	return out, nil
}

func HasObject(gitDir, sha string) bool {
	_, code, _ := git(gitDir, "cat-file", "-e", sha+"^{object}")
	return code == 0
}

func IsAncestor(gitDir, old, new string) (bool, error) {
	_, code, _ := git(gitDir, "merge-base", "--is-ancestor", old, new)
	return code == 0, nil
}

// WritePack builds a NON-THIN pack so it is self-contained with respect to
// delta bases — a thin pack would need objects the remote may not hold.
func WritePack(gitDir, want string, haves []string, outDir string) (string, string, error) {
	revArgs := []string{"rev-list", "--objects", want}
	for _, h := range haves {
		revArgs = append(revArgs, "^"+h)
	}
	objs, code, _ := git(gitDir, revArgs...)
	if code != 0 {
		return "", "", fmt.Errorf("rev-list failed: %s", objs)
	}
	if strings.TrimSpace(objs) == "" {
		return "", "", nil // nothing to send
	}

	cmd := exec.Command("git", "-C", gitDir, "pack-objects", "--no-thin", "-q",
		filepath.Join(outDir, "pack"))
	cmd.Stdin = strings.NewReader(objs + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("pack-objects: %s", strings.TrimSpace(string(out)))
	}
	name := strings.TrimSpace(string(out))
	base := filepath.Join(outDir, "pack-"+name)
	return base + ".pack", base + ".idx", nil
}
```

- [ ] **Step 4: Run to verify pass**

```bash
go test ./internal/gitcmd/
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gitcmd
git commit -m "feat(v2): git plumbing wrapper for type, ancestry, and pack build"
```

---

## Task 9 — Ref advertisement and transitions

**Files:**
- Create: `internal/repo/refs.go`
- Modify: `internal/repo/repo_test.go`

**Interfaces:**
- Consumes: `transport.Transport`, `gitcmd`.
- Produces:
  - `func ListRefs(t transport.Transport, root string) (map[string]string, error)` — refname → sha
  - `func WriteRef(t transport.Transport, root, ref, sha string, exists bool) (transport.Outcome, error)`

A ref file is exactly 40 lowercase hex plus `\n`. Anything else is corruption and is fatal, never coerced.

- [ ] **Step 1: Write the failing tests**

```go
func TestWriteAndListRefs(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	sha := "1111111111111111111111111111111111111111"
	if out, err := WriteRef(f, "/r", "refs/heads/main", sha, false); err != nil || out != transport.Committed {
		t.Fatalf("create ref: %v %v", out, err)
	}
	refs, err := ListRefs(f, "/r")
	if err != nil {
		t.Fatal(err)
	}
	if refs["refs/heads/main"] != sha {
		t.Errorf("got %q", refs["refs/heads/main"])
	}
}

func TestListRefsRejectsCorruptRefFile(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	f.Files["/r/refs/heads/bad"] = []byte("not-a-sha\n")
	if _, err := ListRefs(f, "/r"); err == nil {
		t.Error("a malformed ref file must be fatal, never coerced")
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/repo/ -run TestWriteAndListRefs
```

Expected: FAIL, undefined `WriteRef`.

- [ ] **Step 3: Implement `refs.go`**

```go
package repo

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/transport"
)

var shaRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

func ListRefs(t transport.Transport, root string) (map[string]string, error) {
	out := map[string]string{}
	for _, ns := range []string{"refs/heads", "refs/tags"} {
		nodes, err := t.List(root + "/" + ns)
		if err != nil {
			return nil, err
		}
		for _, n := range nodes {
			if n.IsDir {
				continue
			}
			sha, err := readRef(t, root+"/"+ns+"/"+n.Name)
			if err != nil {
				return nil, err
			}
			out[ns+"/"+n.Name] = sha
		}
	}
	return out, nil
}

func readRef(t transport.Transport, p string) (string, error) {
	dir, err := os.MkdirTemp("", "gpb-ref-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	if err := t.ReadTo(p, dir); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return "", fmt.Errorf("ref %s could not be read back", p)
	}
	raw, err := os.ReadFile(dir + string(os.PathSeparator) + entries[0].Name())
	if err != nil {
		return "", err
	}
	sha := strings.TrimRight(string(raw), "\r\n")
	if !shaRe.MatchString(sha) {
		return "", fmt.Errorf("corrupt ref file %s: %q is not a 40-hex sha", p, sha)
	}
	return sha, nil
}

// WriteRef stages under the ref's own LEAF NAME, because `filesystem upload`
// names the uploaded node after the local basename and has no --name flag
// (probe C11). A leaf hostile to a local path is rejected by stagedFile with a
// named reason rather than mangled. It then verifies by read-back, because a
// byte-identical write is silently skipped.
func WriteRef(t transport.Transport, root, ref, sha string, exists bool) (transport.Outcome, error) {
	if !shaRe.MatchString(sha) {
		return transport.Ambiguous, fmt.Errorf("refusing to write non-sha %q to %s", sha, ref)
	}
	leaf := ref[strings.LastIndex(ref, "/")+1:]
	staged, cleanup, err := stagedFile([]byte(sha+"\n"), leaf)
	if err != nil {
		return transport.Ambiguous, err
	}
	defer cleanup()

	p := root + "/" + ref
	var out transport.Outcome
	if exists {
		out, err = t.UpdateRevision(p, staged)
	} else {
		out, err = t.CreateExclusive(p, staged)
	}
	if err != nil {
		return transport.Ambiguous, err
	}
	if out == transport.Refused && !exists {
		return transport.Refused, nil // concurrent creator
	}

	got, rerr := readRef(t, p)
	if rerr != nil {
		return transport.Ambiguous, rerr
	}
	if got != sha {
		return transport.Ambiguous, fmt.Errorf("ref %s reads back as %s, expected %s", ref, got, sha)
	}
	return transport.Committed, nil
}
```

- [ ] **Step 4: Run to verify pass**

```bash
go test ./internal/repo/
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repo
git commit -m "feat(v2): ref advertisement and read-back-verified ref writes"
```

---

## Task 10 — Push orchestration

**Files:**
- Create: `internal/repo/push.go`
- Modify: `internal/repo/repo_test.go`

**Interfaces:**
- Produces: `func Push(t transport.Transport, root, gitDir string, ups []protocol.RefUpdate, remote map[string]string) []Result`
- Produces: `type Result struct { Ref string; OK bool; Err string }`

Ordering is **pack → idx → confirm both → ref**. A ref must never point at objects that are not fully uploaded. Multi-ref batches are **not atomic**; each ref reports its own status.

- [ ] **Step 1: Write the failing tests**

```go
func TestPushRejectsNonFastForward(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	old := "1111111111111111111111111111111111111111"
	_, _ = WriteRef(f, "/r", "refs/heads/main", old, false)

	ups := []protocol.RefUpdate{{Src: "refs/heads/main", Dst: "refs/heads/main"}}
	res := Push(f, "/r", t.TempDir(), ups, map[string]string{"refs/heads/main": old})
	if len(res) != 1 || res[0].OK {
		t.Fatalf("want one failed result, got %+v", res)
	}
	if !strings.Contains(res[0].Err, "fetch first") {
		t.Errorf("unknown old sha must report 'fetch first', got %q", res[0].Err)
	}
}

func TestPushDeleteRef(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	sha := "2222222222222222222222222222222222222222"
	_, _ = WriteRef(f, "/r", "refs/heads/tmp", sha, false)

	ups := []protocol.RefUpdate{{Src: "", Dst: "refs/heads/tmp"}}
	res := Push(f, "/r", t.TempDir(), ups, map[string]string{"refs/heads/tmp": sha})
	if len(res) != 1 || !res[0].OK {
		t.Fatalf("delete should succeed: %+v", res)
	}
	if _, ok := f.Files["/r/refs/heads/tmp"]; ok {
		t.Error("ref file must be gone")
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/repo/ -run TestPush
```

Expected: FAIL, undefined `Push`.

- [ ] **Step 3: Implement `push.go`**

```go
package repo

import (
	"fmt"
	"os"

	"github.com/craigstoller/git-proton-backup/internal/gitcmd"
	"github.com/craigstoller/git-proton-backup/internal/protocol"
	"github.com/craigstoller/git-proton-backup/internal/transport"
)

type Result struct {
	Ref string
	OK  bool
	Err string
}

func Push(t transport.Transport, root, gitDir string,
	ups []protocol.RefUpdate, remote map[string]string) []Result {

	results := make([]Result, 0, len(ups))
	for _, u := range ups {
		results = append(results, pushOne(t, root, gitDir, u, remote))
	}
	return results
}

func pushOne(t transport.Transport, root, gitDir string,
	u protocol.RefUpdate, remote map[string]string) Result {

	fail := func(msg string) Result { return Result{Ref: u.Dst, Err: msg} }
	oldSha, exists := remote[u.Dst]

	// --- delete -------------------------------------------------------------
	if u.Src == "" {
		if !exists {
			return Result{Ref: u.Dst, OK: true} // already absent
		}
		if out, err := t.Trash(root + "/" + u.Dst); err != nil || out != transport.Committed {
			return fail(fmt.Sprintf("delete failed: %v", err))
		}
		return Result{Ref: u.Dst, OK: true}
	}

	newSha, err := resolve(gitDir, u.Src)
	if err != nil {
		return fail(err.Error())
	}

	// --- branch targets must be commits ------------------------------------
	if isBranch(u.Dst) {
		typ, err := gitcmd.ObjectType(gitDir, newSha)
		if err != nil {
			return fail("cannot determine object type")
		}
		if typ != "commit" {
			return fail(fmt.Sprintf("branch cannot point at a %s", typ))
		}
	}

	// --- ancestry ----------------------------------------------------------
	if exists && !u.Force {
		if !gitcmd.HasObject(gitDir, oldSha) {
			return fail("fetch first")
		}
		// IsAncestor distinguishes "not an ancestor" (exit 1) from a tooling
		// failure. Discarding the error would report a broken git as a
		// confident non-fast-forward rejection.
		ok, err := gitcmd.IsAncestor(gitDir, oldSha, newSha)
		if err != nil {
			return fail("cannot determine ancestry: " + err.Error())
		}
		if !ok {
			return fail("non-fast-forward")
		}
	}

	// --- pack --------------------------------------------------------------
	tmp, err := os.MkdirTemp("", "gpb-pack-*")
	if err != nil {
		return fail(err.Error())
	}
	defer os.RemoveAll(tmp)

	haves := make([]string, 0, len(remote))
	for _, s := range remote {
		if gitcmd.HasObject(gitDir, s) {
			haves = append(haves, s)
		}
	}
	packPath, idxPath, err := gitcmd.WritePack(gitDir, newSha, haves, tmp)
	if err != nil {
		return fail("pack failed: " + err.Error())
	}

	if packPath != "" {
		// Pack, then index, then CONFIRM BOTH before publishing the ref.
		for _, f := range []string{packPath, idxPath} {
			dst := root + "/packs/" + filepathBase(f)
			out, err := t.CreateExclusive(dst, f)
			if err != nil {
				return fail("upload failed: " + err.Error())
			}
			if out == transport.Ambiguous {
				return fail("upload outcome ambiguous; re-run to reconcile")
			}
			if _, ok, _ := t.Stat(dst); !ok {
				return fail("uploaded object is not readable back: " + dst)
			}
		}
	}

	// --- publish ------------------------------------------------------------
	if out, err := WriteRef(t, root, u.Dst, newSha, exists); err != nil || out == transport.Ambiguous {
		return fail(fmt.Sprintf("ref publish failed: %v", err))
	}
	return Result{Ref: u.Dst, OK: true}
}

func isBranch(ref string) bool { return len(ref) > 11 && ref[:11] == "refs/heads/" }

func resolve(gitDir, src string) (string, error) {
	if shaRe.MatchString(src) {
		return src, nil
	}
	out, code, _ := gitcmd.RevParse(gitDir, src)
	if code != 0 {
		return "", fmt.Errorf("cannot resolve %s", src)
	}
	return out, nil
}

func filepathBase(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[i+1:]
		}
	}
	return p
}
```

- [ ] **Step 4: Add `RevParse` to `gitcmd`**

```go
func RevParse(gitDir, rev string) (string, int, error) {
	out, code, err := git(gitDir, "rev-parse", rev)
	return out, code, err
}
```

- [ ] **Step 5: Run to verify pass**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal
git commit -m "feat(v2): push orchestration with pack-before-ref ordering"
```

---

## Task 11 — Wire main.go and prove a real push

**Files:**
- Modify: `cmd/git-remote-proton/main.go`

**Interfaces:**
- Consumes: everything above.

- [ ] **Step 1: Implement the full command loop**

```go
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/protocol"
	"github.com/craigstoller/git-proton-backup/internal/repo"
	"github.com/craigstoller/git-proton-backup/internal/transport"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "git-remote-proton: must be run by git as a remote helper")
		os.Exit(1)
	}
	root := strings.TrimPrefix(os.Args[2], "proton::")
	gitDir := os.Getenv("GIT_DIR")
	if gitDir == "" {
		gitDir = "."
	}

	t := transport.NewCLI("")
	in := bufio.NewScanner(os.Stdin)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	var opts protocol.Options
	var lock *repo.Lock
	defer func() {
		if lock != nil {
			_ = lock.Release() // release on EVERY exit path; a leak wedges the repo
		}
	}()

	for in.Scan() {
		line := in.Text()
		switch {
		case line == "capabilities":
			fmt.Fprint(out, "option\npush\n\n")
			out.Flush()

		case strings.HasPrefix(line, "option "):
			opts.Observe(line)
			fmt.Fprint(out, "unsupported\n") // advisory only; poison flag is the real defence
			out.Flush()

		case line == "list for-push":
			if err := repo.Bootstrap(t, root); err != nil {
				die(err)
			}
			l, err := repo.AcquireLock(t, root) // BEFORE advertising
			if err != nil {
				die(err)
			}
			lock = l
			refs, err := repo.ListRefs(t, root)
			if err != nil {
				die(err)
			}
			for name, sha := range refs {
				fmt.Fprintf(out, "%s %s\n", sha, name)
			}
			fmt.Fprint(out, "\n")
			out.Flush()

		case strings.HasPrefix(line, "push "):
			batch := []string{line}
			for in.Scan() {
				l := in.Text()
				if l == "" {
					break
				}
				if strings.HasPrefix(l, "option ") {
					opts.Observe(l) // options may appear after the last push line
					continue
				}
				batch = append(batch, l)
			}
			// Check poison AFTER the whole batch is buffered, before mutating.
			if opts.Poisoned != "" {
				for _, b := range batch {
					dst := b[strings.Index(b, ":")+1:]
					fmt.Fprintf(out, "error %s unsupported option %s\n", dst, opts.Poisoned)
				}
				fmt.Fprint(out, "\n")
				out.Flush()
				continue
			}
			ups, err := protocol.ParsePushBatch(batch)
			if err != nil {
				die(err)
			}
			remote, err := repo.ListRefs(t, root)
			if err != nil {
				die(err)
			}
			for _, r := range repo.Push(t, root, gitDir, ups, remote) {
				if r.OK {
					fmt.Fprintf(out, "ok %s\n", r.Ref)
				} else {
					fmt.Fprintf(out, "error %s %s\n", r.Ref, r.Err)
				}
			}
			fmt.Fprint(out, "\n")
			out.Flush()

		case line == "":
			return
		default:
			die(fmt.Errorf("unsupported command: %q", line))
		}
	}
}

func die(err error) {
	fmt.Fprintf(os.Stderr, "git-remote-proton: %v\n", err)
	os.Exit(1)
}
```

- [ ] **Step 2: Build and install on PATH**

```bash
go build -o "$env:USERPROFILE/Tools/git-remote-proton.exe" ./cmd/git-remote-proton
```

- [ ] **Step 3: Real end-to-end push**

```bash
cd $env:TEMP; rm -r -fo v2demo -ErrorAction SilentlyContinue; mkdir v2demo; cd v2demo
git init -qb main; git config user.email t@t; git config user.name t
"hello" > a.txt; git add .; git commit -qm "first"
git remote add proton-v2 proton::/my-files/GitRemotes/v2demo
git push proton-v2 main
```

Expected: `ok refs/heads/main`, exit 0.

- [ ] **Step 4: Verify the remote independently**

```bash
proton-drive filesystem list /my-files/GitRemotes/v2demo --json
proton-drive filesystem list /my-files/GitRemotes/v2demo/refs/heads --json
```

Expected: the marker, `refs/`, `packs/`; and a `main` ref file.

- [ ] **Step 5: Prove the ordering guarantee held**

```bash
proton-drive filesystem list /my-files/GitRemotes/v2demo/packs --json
```

Expected: a matching `.pack` and `.idx` pair. A ref with no pack is a Stage 2 failure.

- [ ] **Step 6: Second push is a clean fast-forward**

```bash
"world" >> a.txt; git commit -qam "second"; git push proton-v2 main
```

Expected: `ok refs/heads/main`.

- [ ] **Step 7: Non-fast-forward is rejected**

```bash
git reset --hard HEAD~1; git commit -q --allow-empty -m "divergent"; git push proton-v2 main
```

Expected: rejected with `non-fast-forward`; the remote ref is unchanged.

- [ ] **Step 8: Clean up the demo remote**

```bash
proton-drive filesystem trash /my-files/GitRemotes/v2demo
```

- [ ] **Step 9: Commit**

```bash
git add cmd internal
git commit -m "feat(v2): wire the helper; git push proton-v2 works end to end"
```

---

## Self-Review

**Spec coverage.** Transport contract → Tasks 3–5, with each Stage 1 finding (C1 merge-in-place, C2 read-back, C3 trash shape, C4 trash-missing, C5 EnsureDir, C6 empty list) implemented at its named line. Bootstrap ordering → Task 6. Lock lifecycle including acquire-before-advertise, nonce identity, no takeover, release-on-every-path → Tasks 7 and 11. Option poison state machine including post-batch options → Tasks 2 and 11. Ref transitions → Tasks 9 and 10. Pack ordering → Task 10. Protocol wire format → Tasks 2 and 11.

**Deliberately deferred to Stage 3+ and NOT in this plan:** fetch, clone, tag transitions beyond create, `refs/notes` and other namespaces, initial `HEAD` derivation, and shallow/partial refusal beyond the poison flag. Stage 2's contract is "a real `git push` works", and each of those needs the fetch half or its own transition rules.

**Known judgment calls for the implementer.** `readRef` and `readLock` both download to a temp directory because the CLI's `download` takes a destination *folder*, not a file path — do not assume a file destination. `filepathBase` is hand-rolled rather than `filepath.Base` because remote paths are always POSIX regardless of host OS, and `filepath.Base` would mishandle them on Windows.

**Revised 2026-08-01 during Task 7 — the fake diverged from the CLI a second time.** `Transport.ReadTo` was declared with no comment about its destination. `CLI.ReadTo` runs `filesystem download p localDir`, where the destination is a **folder**; the fake wrote straight to `localPath` as if it were a **file**. Neither contradicted a stated contract, because there wasn't one, and `readLock` was the first caller in the codebase — so it stayed latent through Tasks 3, 4 and 5. The contract is now stated on the interface and the fake is fixed. **This is the second fake/real divergence on this branch**, after the C11 staging bug below, and both were invisible to the deterministic suite. `Trash`, `CreateExclusive` and `UpdateRevision` have never been differentially tested against their CLI counterparts either; that audit belongs in the final review, not in Task 11's live push alone.

**Revised 2026-08-01 during Task 5 (probes C11–C14).** The plan originally staged every ref write through a *neutral* temporary local filename, per design v5. That is not expressible: `filesystem upload` takes a PARENT path and has no `--name` flag, so the CLI names the uploaded node after the LOCAL basename (C11), and upload-then-`rename` cannot serve `UpdateRevision` without first trashing the ref (C12). Staging now happens under the target's own **leaf name**, and leaf names hostile to a local path are rejected with a named reason rather than mangled — the rejected set being brace-globbing names (C13 confirms the hazard is live on 0.7.0) and Windows device names. `docs/v2-remote-helper-design.md` v6 records the same change. Note the in-memory fake would **not** have caught this: it keys on the full target path, so every repo-layer test would have passed while the real transport wrote to the wrong name.
