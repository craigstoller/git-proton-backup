package transport

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFakeSatisfiesTransport is a compile-time check: if Fake ever drifts
// from the Transport interface, this file fails to build.
var _ Transport = (*Fake)(nil)

func writeTemp(t *testing.T, contents string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "payload")
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
	local := writeTemp(t, "hello")

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
	local := writeTemp(t, "same bytes")

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

	changed := writeTemp(t, "different bytes")
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
	local := writeTemp(t, "x")

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
	local := writeTemp(t, "lock body")
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
// not create it on the caller's behalf, because the real CLI does not either.
// A missing (or non-directory) destination must surface as the error it
// naturally is, not be silently papered over.
func TestFakeReadToIntoMissingDirectoryErrors(t *testing.T) {
	f := NewFake()
	local := writeTemp(t, "x")
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

	local := writeTemp(t, "sha")
	if _, err := f.CreateExclusive("/refs/heads/main", local); err != nil {
		t.Fatalf("seed create: %v", err)
	}
	if _, err := f.CreateExclusive("/refs/heads/dev", local); err != nil {
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
