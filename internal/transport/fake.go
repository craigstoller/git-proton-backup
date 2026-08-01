package transport

import (
	"fmt"
	"os"
	"path"
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

func (f *Fake) ReadTo(p, local string) error {
	b, ok := f.Files[p]
	if !ok {
		return fmt.Errorf("not found: %s", p)
	}
	return os.WriteFile(local, b, 0o644)
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
