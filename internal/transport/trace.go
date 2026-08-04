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
