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

func (f *Fake) EnsureDir(p string) error { f.Dirs[p] = true; return nil }

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
	return Node{}, false, nil
}

// ReadTo mirrors the real CLI's folder-destination download: local must
// already be a directory, and the file lands under path's own remote
// basename (path.Base, not filepath.Base — remote paths are always POSIX
// regardless of host OS). A missing or non-directory local is not created
// on the caller's behalf; os.WriteFile reports it as the error it is.
func (f *Fake) ReadTo(p, local string) error {
	b, ok := f.Files[p]
	if !ok {
		return fmt.Errorf("not found: %s", p)
	}
	return os.WriteFile(filepath.Join(local, path.Base(p)), b, 0o644)
}

// checkUploadBasename mechanically enforces the caller contract cli.go states
// in capitals but cannot itself verify: `filesystem upload` takes a PARENT
// path and has no --name flag, so the real CLI names the remote node after the
// LOCAL file's basename (probe C11). Both Fake write methods used to read
// local purely for its bytes and key on p, which meant a caller staging under
// a neutral name passed the entire suite and then wrote to the wrong remote
// name live — the defect that already reached this branch once and cost a
// design revision. The Fake is the only place that can catch it before a live
// push, so it catches it here.
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
	return fmt.Errorf("caller contract violated (probe C11): local basename %q must equal "+
		"the remote leaf %q of %s — `filesystem upload` has no --name flag, so the real CLI "+
		"would name the node after the local file and the write would land at the wrong "+
		"remote path", got, want, p)
}

func (f *Fake) CreateExclusive(p, local string) (Outcome, error) {
	if err := checkUploadBasename(p, local); err != nil {
		return Ambiguous, err
	}
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
	if err := checkUploadBasename(p, local); err != nil {
		return Ambiguous, err
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

func (f *Fake) Trash(p string) (Outcome, error) {
	if _, ok := f.Files[p]; !ok {
		return Committed, nil // already absent is the desired end state
	}
	delete(f.Files, p)
	return Committed, nil
}
