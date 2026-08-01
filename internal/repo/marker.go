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
