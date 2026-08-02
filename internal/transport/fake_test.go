package transport

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFakeSatisfiesTransport is a compile-time check: if Fake ever drifts
// from the Transport interface, this file fails to build.
var _ Transport = (*Fake)(nil)

// writeTemp stages contents in a fresh temp dir under the given NAME. The name
// is a required parameter, not a fixed "payload", because Fake's write methods
// enforce the C11 caller contract (local basename == remote leaf) — a test
// helper that always staged under one neutral name could only ever exercise
// the violation. Each t.TempDir() call returns a distinct directory, so two
// files may share a basename without colliding.
func writeTemp(t *testing.T, name, contents string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(contents), 0o644); err != nil {
		t.Fatalf("writeTemp: %v", err)
	}
	return p
}

func TestFakeStatAbsenceIsNotError(t *testing.T) {
	f := NewFake()
	node, ok, err := f.Stat("/does/not/exist")
	if err != nil {
		t.Fatalf("absence must not be an error, got %v", err)
	}
	if ok {
		t.Fatalf("want ok=false for absent path, got node=%+v", node)
	}
}

func TestFakeCreateExclusiveThenRefuse(t *testing.T) {
	f := NewFake()
	local := writeTemp(t, "aa", "hello")

	out, err := f.CreateExclusive("/objects/aa", local)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != Committed {
		t.Fatalf("first create: want Committed, got %s", out)
	}

	out, err = f.CreateExclusive("/objects/aa", local)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != Refused {
		t.Fatalf("create on existing name: want Refused, got %s", out)
	}
}

// TestFakeUpdateRevisionSkipsIdenticalContent locks in the deliberate,
// non-obvious behaviour documented in the brief: the real Proton CLI
// silently skips an upload whose content is byte-identical to what's
// already there (Stage 1 C2), so a no-op write is indistinguishable from
// a real one by outcome alone. Callers must verify by reading back.
func TestFakeUpdateRevisionSkipsIdenticalContent(t *testing.T) {
	f := NewFake()
	local := writeTemp(t, "main", "same bytes")

	if _, err := f.CreateExclusive("/refs/heads/main", local); err != nil {
		t.Fatalf("seed create: %v", err)
	}

	out, err := f.UpdateRevision("/refs/heads/main", local)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != Refused {
		t.Fatalf("byte-identical rewrite: want Refused (mirrors real CLI's silent skip), got %s", out)
	}

	changed := writeTemp(t, "main", "different bytes")
	out, err = f.UpdateRevision("/refs/heads/main", changed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != Committed {
		t.Fatalf("genuinely different content: want Committed, got %s", out)
	}
}

// TestFakeTrashOnMissingIsCommitted locks in the idempotent interface
// contract: the desired end state ("not there") already holds, so Trash
// on an absent target reports Committed rather than erroring (the real
// CLI's exit-1-on-missing behaviour is a Task 5 concern, not this one).
func TestFakeTrashOnMissingIsCommitted(t *testing.T) {
	f := NewFake()
	out, err := f.Trash("/objects/never-existed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != Committed {
		t.Fatalf("trash on missing target: want Committed, got %s", out)
	}
}

func TestFakeFailNextForcesAmbiguousOnce(t *testing.T) {
	f := NewFake()
	f.FailNext = "inject"
	local := writeTemp(t, "bb", "x")

	out, err := f.CreateExclusive("/objects/bb", local)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != Ambiguous {
		t.Fatalf("first mutation after FailNext set: want Ambiguous, got %s", out)
	}
	if f.FailNext != "" {
		t.Fatalf("FailNext must be consumed after firing once, got %q", f.FailNext)
	}

	out, err = f.CreateExclusive("/objects/bb", local)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != Committed {
		t.Fatalf("second mutation: want Committed, got %s", out)
	}
}

// TestFakeReadToLandsUnderRemoteBasenameInAnExistingDir pins the contract now
// documented on Transport.ReadTo: local is a pre-existing directory, and the
// downloaded content lands under the remote path's own basename, mirroring
// `filesystem download path... localFolder`. Before this fix Fake.ReadTo
// wrote directly to local as if it were a destination file, which broke the
// first real caller (repo.readLock, Task 7) the moment one existed — nothing
// exercised ReadTo before that. This test exists so a future "simplification"
// back to file-destination semantics fails loudly here instead of silently
// resurfacing in a caller.
func TestFakeReadToLandsUnderRemoteBasenameInAnExistingDir(t *testing.T) {
	f := NewFake()
	local := writeTemp(t, ".lock", "lock body")
	if _, err := f.CreateExclusive("/my-files/r/.lock", local); err != nil {
		t.Fatalf("seed create: %v", err)
	}

	dir := t.TempDir()
	if err := f.ReadTo("/my-files/r/.lock", dir); err != nil {
		t.Fatalf("ReadTo: %v", err)
	}

	want := filepath.Join(dir, ".lock")
	got, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("expected downloaded file at %s: %v", want, err)
	}
	if string(got) != "lock body" {
		t.Errorf("downloaded content = %q, want %q", got, "lock body")
	}
}

// TestFakeReadToIntoMissingDirectoryErrors covers the other half of the
// ReadTo contract: local must already be a directory, and implementations do
// not create it on the caller's behalf. A missing (or non-directory)
// destination must surface as the error it naturally is, not be silently
// papered over.
//
// This comment used to justify that with "because the real CLI does not
// create it either", which the Stage 3a live gate disproved — the binary DOES
// create it and succeed (probe C16). The contract survived the correction on
// its own merits: every caller here creates its temp dir with os.MkdirTemp
// first, so a missing destination always indicates a caller bug, and *CLI now
// stats the destination itself rather than the contract being loosened to
// match the binary. Both implementations therefore still owe this behaviour.
func TestFakeReadToIntoMissingDirectoryErrors(t *testing.T) {
	f := NewFake()
	local := writeTemp(t, "cc", "x")
	if _, err := f.CreateExclusive("/objects/cc", local); err != nil {
		t.Fatalf("seed create: %v", err)
	}

	dest := filepath.Join(t.TempDir(), "does-not-exist")
	if err := f.ReadTo("/objects/cc", dest); err == nil {
		t.Fatal("ReadTo into a missing directory must error, not silently succeed")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("ReadTo must not create the destination directory for the caller; stat error = %v", err)
	}
}

func TestFakeEnsureDirAndList(t *testing.T) {
	f := NewFake()
	if err := f.EnsureDir("/refs/heads"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// EnsureDir must tolerate being called again on an already-existing dir
	// (that's the whole point of Stat-then-create vs. a bare create).
	if err := f.EnsureDir("/refs/heads"); err != nil {
		t.Fatalf("second EnsureDir on existing dir must not error: %v", err)
	}

	if _, err := f.CreateExclusive("/refs/heads/main", writeTemp(t, "main", "sha")); err != nil {
		t.Fatalf("seed create: %v", err)
	}
	if _, err := f.CreateExclusive("/refs/heads/dev", writeTemp(t, "dev", "sha")); err != nil {
		t.Fatalf("seed create: %v", err)
	}

	nodes, err := f.List("/refs/heads")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 2 || nodes[0].Name != "dev" || nodes[1].Name != "main" {
		t.Fatalf("want sorted [dev main], got %+v", nodes)
	}
}

// TestFakeRejectsMismatchedLocalBasename is the mechanical enforcement of the
// caller contract cli.go states in capitals but cannot itself check: because
// `filesystem upload` takes a PARENT path and has no --name flag, the real CLI
// names the remote node after the LOCAL file's basename (probe C11). Before
// this, both Fake write methods read local purely for its bytes and keyed on
// p, so a caller staging under a neutral name passed every test in the suite
// and would have written to the wrong remote name live — which is exactly the
// defect that already reached this branch once and cost a design revision.
func TestFakeRejectsMismatchedLocalBasename(t *testing.T) {
	f := NewFake()
	neutral := writeTemp(t, "payload", "1111111111111111111111111111111111111111\n")

	out, err := f.CreateExclusive("/my-files/r/refs/heads/main", neutral)
	if err == nil {
		t.Fatalf("CreateExclusive must reject a local basename that is not the remote leaf, got %v, nil error", out)
	}
	if out != Ambiguous {
		t.Errorf("CreateExclusive contract violation must report Ambiguous, got %v", out)
	}
	if _, ok := f.Files["/my-files/r/refs/heads/main"]; ok {
		t.Error("nothing must be written for a rejected upload")
	}

	// Seed through the legitimate path so UpdateRevision has something to
	// revise, then prove the same guard applies to the update side.
	if _, err := f.CreateExclusive("/my-files/r/refs/heads/main", writeTemp(t, "main", "x")); err != nil {
		t.Fatalf("seed create: %v", err)
	}
	out, err = f.UpdateRevision("/my-files/r/refs/heads/main", neutral)
	if err == nil {
		t.Fatalf("UpdateRevision must reject a local basename that is not the remote leaf, got %v, nil error", out)
	}
	if out != Ambiguous {
		t.Errorf("UpdateRevision contract violation must report Ambiguous, got %v", out)
	}
	if string(f.Files["/my-files/r/refs/heads/main"]) != "x" {
		t.Errorf("a rejected update must leave the existing content alone, got %q", f.Files["/my-files/r/refs/heads/main"])
	}
}

// TestFakeListIncludesEmptyEnsuredDirs covers a Fake that was more permissive
// than the transport it stands in for. List synthesised directories from FILE
// prefixes only and never read f.Dirs, so a folder created by EnsureDir with
// nothing in it yet was invisible — while the real CLI's `filesystem list`
// reports it. repo.Bootstrap's emptiness test is the one decision in the
// codebase that turns on an empty-folder listing, so the gap meant the suite
// was validating a Bootstrap more permissive than the one that ships.
func TestFakeListIncludesEmptyEnsuredDirs(t *testing.T) {
	f := NewFake()
	if err := f.EnsureDir("/my-files/r/refs"); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	nodes, err := f.List("/my-files/r")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Name != "refs" || !nodes[0].IsDir {
		t.Fatalf("an EnsureDir'd empty folder must be listed as a child dir, got %+v", nodes)
	}

	// A dir that also has files under it must appear exactly once, not twice:
	// the file-prefix synthesis and the f.Dirs pass must be deduplicated.
	if err := f.EnsureDir("/my-files/r/packs"); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	if _, err := f.CreateExclusive("/my-files/r/refs/heads", writeTemp(t, "heads", "x")); err != nil {
		t.Fatalf("seed create: %v", err)
	}
	nodes, err = f.List("/my-files/r")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(nodes) != 2 || nodes[0].Name != "packs" || nodes[1].Name != "refs" {
		t.Fatalf("want exactly [packs refs] with no duplicate, got %+v", nodes)
	}
}
