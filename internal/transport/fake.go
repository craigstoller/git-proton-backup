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
//
// Because it is the ONLY thing those layers ever run against, every contract
// the real CLI imposes but cannot check at compile time has to be enforced
// here or it is not enforced at all: see checkUploadBasename (probe C11) and
// List's handling of f.Dirs. A Fake more permissive than the CLI is a suite
// that certifies code the live transport will reject.
type Fake struct {
	Files map[string][]byte
	Dirs  map[string]bool
	// FailNext, when non-empty, makes the next mutation return Ambiguous.
	FailNext string
}

func NewFake() *Fake {
	return &Fake{Files: map[string][]byte{}, Dirs: map[string]bool{}}
}

// isBuiltinMountParent reports whether p is one of the Proton Drive
// namespaces this Fake treats as already existing without ever being
// explicitly created: the two top-level mounts themselves, and up to two
// path components beneath /devices — a device root ("/devices/<id>") and one
// folder inside it ("/devices/<id>/<folder>").
//
// The depth-1 case ("/devices/<id>" is itself a pre-existing root the CLI
// cannot create, the same way "/my-files" is) is the part that follows
// directly from how Proton Drive's namespaces work. The depth-2 allowance
// ("/devices/<id>/<folder>" also needing no seeding) is a Task-7-brief-
// mandated Fake-only convenience on top of that, NOT a claim that this
// mirrors Task 11's EnsureParents design (docs/superpowers/plans/
// 2026-08-06-v2-stage5-hierarchical-refs.md) — that design's own
// protectedDepth only ever protects depth 1 ("/devices/<device-id>").
// Reconciling (or deliberately keeping distinct) the two definitions is
// Task 11's call to make, not asserted here. Anything deeper than depth 2
// must be genuinely present in f.Dirs or implied by f.Files, same as every
// other path.
func isBuiltinMountParent(p string) bool {
	if p == "/my-files" || p == "/devices" {
		return true
	}
	if rest, ok := strings.CutPrefix(p, "/devices/"); ok && rest != "" {
		return strings.Count(rest, "/") <= 1
	}
	return false
}

// parentExists reports whether p is usable as an EnsureDir parent: a
// builtin mount namespace, an already-created directory, or a directory
// implied by some file living underneath it (a file can only exist if its
// parent folder does).
func (f *Fake) parentExists(p string) bool {
	if isBuiltinMountParent(p) {
		return true
	}
	if f.Dirs[p] {
		return true
	}
	prefix := p + "/"
	for k := range f.Files {
		if strings.HasPrefix(k, prefix) {
			return true
		}
	}
	return false
}

// EnsureDir is Stat-then-create, mirroring the live CLI's own contract
// (cli.go: create-folder fails on an existing folder, Stage 1 C5) plus the
// live create-folder failure the earlier, fully permissive Fake never
// modelled at all: a name already taken by a FILE, and a parent that does
// not exist ("Node not found: <parent leaf>", cli.go's notFoundSignature
// shape). A Fake that accepted either would certify code the live transport
// rejects.
func (f *Fake) EnsureDir(p string) error {
	if f.Dirs[p] {
		return nil // idempotent: already a directory
	}
	if _, ok := f.Files[p]; ok {
		return fmt.Errorf("create-folder failed: a file exists at %s", p)
	}
	parent := path.Dir(p)
	if !f.parentExists(parent) {
		return fmt.Errorf("create-folder %s in %s failed: Node not found: %s",
			path.Base(p), parent, path.Base(parent))
	}
	f.Dirs[p] = true
	return nil
}

// List reports the direct children of p, from BOTH the file map and the
// directory set. Reading f.Dirs is not optional bookkeeping: it used to
// synthesise directories from file prefixes alone, so a folder created by
// EnsureDir with nothing under it yet was invisible here while the real CLI's
// `filesystem list` reports it. repo.Bootstrap's emptiness test is the one
// decision in the codebase that turns on an empty-folder listing — with the
// gap, a root whose marker was lost but whose refs/ and packs/ remained was
// hard-refused live (correct: "never guess whether a folder is one of ours")
// and silently adopted under test.
//
// Names are deduplicated across the three sources. Files are added first so a
// name that is genuinely a file is never shadowed by a synthesised dir entry.
func (f *Fake) List(p string) ([]Node, error) {
	var out []Node
	seen := map[string]bool{}
	add := func(n Node) {
		if seen[n.Name] {
			return
		}
		seen[n.Name] = true
		out = append(out, n)
	}
	childDir := func(full string) (string, bool) {
		if !strings.HasPrefix(full, p+"/") {
			return "", false
		}
		return strings.SplitN(strings.TrimPrefix(full, p+"/"), "/", 2)[0], true
	}
	for k, v := range f.Files {
		if path.Dir(k) == p {
			add(Node{Name: path.Base(k), Size: int64(len(v))})
		}
	}
	for k := range f.Files {
		if path.Dir(k) == p {
			continue
		}
		if d, ok := childDir(k); ok {
			add(Node{Name: d, IsDir: true})
		}
	}
	for d := range f.Dirs {
		if c, ok := childDir(d); ok {
			add(Node{Name: c, IsDir: true})
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
	// A map miss is an AFFIRMATIVE absence, never a failure — this Fake has
	// no notion of a transport error on Stat at all, so every miss is
	// confirmed-absence by construction. This already matches the Task 4
	// contract (*CLI.Stat's not-found/error split, cli.go): (_, false, nil)
	// models "confirmed does not exist", the same meaning the certified
	// CLI's own not-found signature carries, not "any failure whatsoever".
	return Node{}, false, nil
}

// ReadTo mirrors the real CLI's folder-destination download: local must
// already be a directory, and the file lands under path's own remote
// basename (path.Base, not filepath.Base — remote paths are always POSIX
// regardless of host OS). A missing or non-directory local is not created on
// the caller's behalf; os.WriteFile reports it as the error it is.
//
// "Not created on the caller's behalf" is the WRAPPER's contract, not the
// binary's, and this comment used to claim otherwise. The Stage 3a live gate
// (task 7, cli-drive@0.7.0) found that `filesystem download` given a missing
// destination silently CREATES the directory and succeeds — probe C16. The
// contract was kept and enforced rather than loosened, because every caller
// here creates its temp dir with os.MkdirTemp first, so a missing destination
// always means a caller bug that auto-creation would hide: *CLI.ReadTo now
// stats localDir and refuses before spawning anything, and this Fake agrees
// with the wrapper, not with the raw binary.
func (f *Fake) ReadTo(p, local string) error {
	b, ok := f.Files[p]
	if !ok {
		return fmt.Errorf("not found: %s", p)
	}
	return os.WriteFile(filepath.Join(local, path.Base(p)), b, 0o644)
}

// checkUploadBasename mechanically enforces the caller contract: `filesystem
// upload` takes a PARENT path and has no --name flag, so the remote node is
// named after the LOCAL file's basename (probe C11). Both Fake write methods
// used to read local purely for its bytes and key on p, which meant a caller
// staging under a neutral name passed the entire suite and then wrote to the
// wrong remote name live — the defect that already reached this branch once
// and cost a design revision.
//
// It is shared by BOTH implementations and lives here only because this file
// is where it was first written. An earlier version of this comment said "the
// Fake is the only place that can catch it before a live push"; that stopped
// being true in Task 2, when review found *CLI passing localFile straight
// through to the binary and the guard was wired into CreateExclusive and
// UpdateRevision there as well. Enforcing it in one implementation only would
// have meant the suite certified a contract the live transport did not apply.
//
// The asymmetry in the two Base calls is deliberate, not a copy-paste slip:
// local is a HOST path, where the separator is whatever the OS uses, so it
// needs filepath.Base; p is a REMOTE Proton path, which is always POSIX
// regardless of host, so it needs path.Base.
//
// A violation reports Ambiguous, matching the Fake's other "the write did not
// happen and the caller must not assume anything" paths.
func checkUploadBasename(p, local string) error {
	got, want := filepath.Base(local), path.Base(p)
	if got == want {
		return nil
	}
	// Phrased from neither implementation's point of view, since both raise
	// it: "the real CLI would name the node after the local file" read oddly
	// coming from inside the real CLI's own wrapper.
	return fmt.Errorf("caller contract violated (probe C11): local basename %q must equal "+
		"the remote leaf %q of %s — `filesystem upload` takes a PARENT path and has no "+
		"--name flag, so the uploaded node is named after the LOCAL file and this write "+
		"would land under the wrong remote name", got, want, p)
}

func (f *Fake) CreateExclusive(p, local string) (Outcome, error) {
	if err := checkUploadBasename(p, local); err != nil {
		return Ambiguous, err
	}
	if f.FailNext != "" {
		f.FailNext = ""
		return Ambiguous, nil
	}
	// A name already taken by a FOLDER is refused the same way a name
	// already taken by a file is — the D/F collision Task 9b heals; the live
	// contract row pins the real CLI's shape and this models the
	// conservative reading. Checked before the Files[p] lookup so both
	// collisions read the same way to a caller inspecting the Outcome alone.
	if f.Dirs[p] {
		return Refused, nil
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
	if err := checkUploadBasename(p, local); err != nil {
		return Ambiguous, err
	}
	// Same D/F collision guard as CreateExclusive, above.
	if f.Dirs[p] {
		return Refused, nil
	}
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

// Trash removes p and, when p is a folder, its ENTIRE subtree — mirroring
// the wrapper contract (transport.go: implementations Stat first, an absent
// target is still Committed, the desired end state already holding) rather
// than the raw CLI's own non-idempotence on folders (spec, component 8,
// corrected in the same commit as this fix — the wrapper-not-binary
// precedent ReadTo/C16 already established). Absence — of p itself, or of
// anything under it — is never an error: the desired end state already
// holds either way, so nothing here needs to track whether anything was
// actually found.
func (f *Fake) Trash(p string) (Outcome, error) {
	delete(f.Files, p)
	delete(f.Dirs, p)
	prefix := p + "/"
	for k := range f.Files {
		if strings.HasPrefix(k, prefix) {
			delete(f.Files, k)
		}
	}
	for k := range f.Dirs {
		if strings.HasPrefix(k, prefix) {
			delete(f.Dirs, k)
		}
	}
	return Committed, nil
}
