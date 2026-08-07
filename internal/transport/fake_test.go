package transport

import (
	"os"
	"path/filepath"
	"strings"
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
	// "/refs" is not a mount root (only "/my-files" and "/devices" are), so
	// the stricter EnsureDir (Task 7) needs it seeded as already-existing —
	// this test is about List's behaviour, not about parent validation, so
	// seeding rather than weakening the Fake keeps that scope intact.
	f.Dirs["/refs"] = true
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
	// "/my-files/r" is not itself a mount root (only "/my-files" is), so the
	// stricter EnsureDir (Task 7) needs it seeded as already-existing, mirroring
	// how a real Bootstrap would have created it first.
	f.Dirs["/my-files/r"] = true
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

// TestFakeTrashOnFolderRemovesWholeSubtree: RED (Task 7). The old Trash only
// ever looked at f.Files[p] itself — a folder path was never a key in Files,
// so trashing a non-empty folder silently did nothing to what's beneath it,
// even though the real CLI's `filesystem trash` removes the whole subtree.
// Files under the prefix and nested Dirs entries must all be gone, and a
// sibling outside the trashed prefix must survive untouched (also covers the
// brief's "Stat on dir unchanged" case).
func TestFakeTrashOnFolderRemovesWholeSubtree(t *testing.T) {
	f := NewFake()
	f.Dirs["/my-files/r"] = true
	for _, d := range []string{"/my-files/r/refs", "/my-files/r/refs/heads", "/my-files/r/packs"} {
		if err := f.EnsureDir(d); err != nil {
			t.Fatalf("EnsureDir %s: %v", d, err)
		}
	}
	if _, err := f.CreateExclusive("/my-files/r/refs/heads/main", writeTemp(t, "main", "sha")); err != nil {
		t.Fatalf("seed create: %v", err)
	}
	if _, err := f.CreateExclusive("/my-files/r/packs/pack-x.pack", writeTemp(t, "pack-x.pack", "bytes")); err != nil {
		t.Fatalf("seed create: %v", err)
	}

	out, err := f.Trash("/my-files/r/refs")
	if err != nil {
		t.Fatalf("Trash: %v", err)
	}
	if out != Committed {
		t.Fatalf("Trash of a non-empty folder: want Committed, got %s", out)
	}

	if _, ok := f.Files["/my-files/r/refs/heads/main"]; ok {
		t.Error("a file under the trashed prefix must be gone")
	}
	if f.Dirs["/my-files/r/refs/heads"] {
		t.Error("a nested dir under the trashed prefix must be gone")
	}
	if f.Dirs["/my-files/r/refs"] {
		t.Error("the trashed folder itself must be gone")
	}

	// Sibling untouched: proves the subtree removal is prefix-scoped, and
	// pins Stat's behaviour on a surviving dir as unchanged.
	if !f.Dirs["/my-files/r/packs"] {
		t.Error("a sibling folder outside the trashed prefix must survive")
	}
	if _, ok := f.Files["/my-files/r/packs/pack-x.pack"]; !ok {
		t.Error("a file under a sibling folder must survive")
	}
	node, ok, statErr := f.Stat("/my-files/r/packs")
	if statErr != nil || !ok || !node.IsDir {
		t.Errorf("Stat on the surviving sibling dir: node=%+v ok=%v err=%v, want an unchanged dir", node, ok, statErr)
	}
}

// TestFakeTrashOfEmptyFolderIsCommitted: RED (Task 7). An empty folder — one
// created by EnsureDir with nothing under it — must also be removable by
// Trash. Before this fix Trash never looked at f.Dirs at all, so trashing an
// empty folder silently left it in f.Dirs forever.
func TestFakeTrashOfEmptyFolderIsCommitted(t *testing.T) {
	f := NewFake()
	f.Dirs["/my-files/r"] = true
	if err := f.EnsureDir("/my-files/r/emptydir"); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	out, err := f.Trash("/my-files/r/emptydir")
	if err != nil {
		t.Fatalf("Trash: %v", err)
	}
	if out != Committed {
		t.Fatalf("Trash of an empty folder: want Committed, got %s", out)
	}
	if f.Dirs["/my-files/r/emptydir"] {
		t.Error("the trashed empty folder must be gone from f.Dirs")
	}
	if _, ok, _ := f.Stat("/my-files/r/emptydir"); ok {
		t.Error("Stat must confirm the trashed folder is gone")
	}
}

// TestFakeCreateExclusiveOntoFolderIsRefused and
// TestFakeUpdateRevisionOntoFolderIsRefused: RED (Task 7). A name already
// taken by a FOLDER must refuse a file write the same way a name taken by a
// file does — the D/F collision Task 9b heals, and the live contract row
// (contract_test.go) pins the real CLI's shape for the upload-onto-folder
// case. Before this fix, CreateExclusive/UpdateRevision only ever checked
// f.Files[p], so writing a file over an existing folder name silently
// succeeded and left the folder's own f.Dirs entry orphaned underneath it.
func TestFakeCreateExclusiveOntoFolderIsRefused(t *testing.T) {
	f := NewFake()
	f.Dirs["/my-files/r"] = true
	if err := f.EnsureDir("/my-files/r/taken"); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	out, err := f.CreateExclusive("/my-files/r/taken", writeTemp(t, "taken", "x"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != Refused {
		t.Fatalf("CreateExclusive onto a folder name: want Refused, got %s", out)
	}
	if !f.Dirs["/my-files/r/taken"] {
		t.Error("the folder must survive a refused create")
	}
	if _, ok := f.Files["/my-files/r/taken"]; ok {
		t.Error("nothing must be written for a refused create onto a folder name")
	}
}

func TestFakeUpdateRevisionOntoFolderIsRefused(t *testing.T) {
	f := NewFake()
	f.Dirs["/my-files/r"] = true
	if err := f.EnsureDir("/my-files/r/taken"); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	out, err := f.UpdateRevision("/my-files/r/taken", writeTemp(t, "taken", "x"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != Refused {
		t.Fatalf("UpdateRevision onto a folder name: want Refused, got %s", out)
	}
	if !f.Dirs["/my-files/r/taken"] {
		t.Error("the folder must survive a refused update")
	}
}

// TestFakeEnsureDirOntoFileErrors: RED (Task 7). Before this fix EnsureDir
// was a bare `f.Dirs[p] = true`, so calling it on a path that is already a
// FILE silently turned the file into a directory in f.Dirs while leaving the
// stale f.Files entry behind — a D/F collision the Fake manufactured rather
// than caught. The exact wording is free (reverse-D/F detection is typed via
// Stat elsewhere; Fake and CLI messages need not match) but the error must
// name the file.
func TestFakeEnsureDirOntoFileErrors(t *testing.T) {
	f := NewFake()
	f.Dirs["/my-files/r"] = true
	if _, err := f.CreateExclusive("/my-files/r/leaf", writeTemp(t, "leaf", "x")); err != nil {
		t.Fatalf("seed create: %v", err)
	}

	err := f.EnsureDir("/my-files/r/leaf")
	if err == nil {
		t.Fatal("EnsureDir onto an existing file must error")
	}
	if !strings.Contains(err.Error(), "/my-files/r/leaf") {
		t.Errorf("error must name the file, got: %v", err)
	}
	if f.Dirs["/my-files/r/leaf"] {
		t.Error("a refused EnsureDir must not turn the file into a directory")
	}
}

// TestFakeEnsureDirMissingParentErrors and
// TestFakeEnsureDirBuiltinMountParentsNeedNoSeeding: RED (Task 7). Before
// this fix EnsureDir never looked at its parent at all, so a caller bug that
// tried to create a deeply nested path with no ancestor ever created would
// silently succeed — the Fake was more permissive than the live CLI, which
// genuinely refuses `create-folder` under a parent that does not exist
// ("Node not found: <parent leaf>", cli.go's notFoundSignature shape).
func TestFakeEnsureDirMissingParentErrors(t *testing.T) {
	f := NewFake()
	err := f.EnsureDir("/my-files/never-seeded/child")
	if err == nil {
		t.Fatal("EnsureDir under a missing parent must error")
	}
	if !strings.Contains(err.Error(), "Node not found: never-seeded") {
		t.Errorf("error must embed the live not-found shape naming the parent leaf, got: %v", err)
	}
	if f.Dirs["/my-files/never-seeded/child"] {
		t.Error("a refused EnsureDir must not create the child")
	}
}

// TestFakeEnsureDirBuiltinMountParentsNeedNoSeeding pins the deliberate
// exception: the two Proton Drive top-level namespaces, and up to two path
// components beneath /devices (a device root and one folder inside it), need
// no prior seeding — EnsureDir may create directly under them, mirroring the
// real mount points the CLI itself cannot create. Anything deeper under
// /devices does need seeding, same as everywhere else. Each case uses a
// FRESH Fake so a pass can only be explained by the builtin allowance
// itself, never by an earlier call in the same test having already seeded
// the parent.
func TestFakeEnsureDirBuiltinMountParentsNeedNoSeeding(t *testing.T) {
	if err := NewFake().EnsureDir("/my-files/newrepo"); err != nil {
		t.Errorf("EnsureDir under /my-files must not need seeding: %v", err)
	}
	if err := NewFake().EnsureDir("/devices/newrepo"); err != nil {
		t.Errorf("EnsureDir directly under /devices must not need seeding: %v", err)
	}
	if err := NewFake().EnsureDir("/devices/dev1/onefolder"); err != nil {
		t.Errorf("EnsureDir at depth 1 under /devices (a device root) must not need seeding: %v", err)
	}
	if err := NewFake().EnsureDir("/devices/dev2/onefolder/twolevel"); err != nil {
		t.Errorf("EnsureDir at depth 2 under /devices (one folder inside a device root) must not need seeding: %v", err)
	}
	if err := NewFake().EnsureDir("/devices/dev3/onefolder/twolevel/threelevel"); err == nil {
		t.Error("EnsureDir at depth 3 under /devices is beyond the builtin allowance and must still require an existing parent")
	}
}
