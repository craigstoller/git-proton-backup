package repo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/transport"
)

const (
	MarkerName = "gpb-remote.json"
	LockName   = ".lock"
	// markerFormat and markerVersion are what this build can honour. version
	// is the design's forward-compatibility seam — "compaction will bump it
	// and define its own ordering scheme at that point" — so a marker written
	// by a future build must be refused here, not adopted by a build that
	// cannot honour whatever the bump stands for.
	markerFormat  = "git-remote-proton"
	markerVersion = 1
	markerContent = `{"format":"` + markerFormat + `","version":1}`
)

// markerBody is the marker's on-disk shape. Fields are plain values rather
// than pointers because both are compared against a required exact value: an
// absent key decodes to the zero value, which matches neither markerFormat nor
// markerVersion and is therefore refused anyway.
type markerBody struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
}

func Bootstrap(t transport.Transport, root string) error {
	marker := root + "/" + MarkerName
	if _, ok, err := t.Stat(marker); err != nil {
		return fmt.Errorf("stat %s: %w", marker, err)
	} else if ok {
		// Presence is not recognition. Bootstrap used to Stat the marker and
		// proceed without ever reading it, so a folder holding a
		// gpb-remote.json that said {"format":"something-else","version":99}
		// was adopted in silence — only half of the design's rule ("An
		// unrecognised OR absent marker on a non-empty folder is a hard
		// refusal — the helper never guesses whether a folder is one of its
		// repos") was implemented.
		if err := checkMarker(t, marker); err != nil {
			return err
		}
		return ensureSubdirs(t, root)
	}

	if err := t.EnsureDir(root); err != nil {
		return fmt.Errorf("ensure dir %s: %w", root, err)
	}
	nodes, err := t.List(root)
	if err != nil {
		return fmt.Errorf("list %s: %w", root, err)
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
		// A concurrent initialiser won. Adopting their repo means adopting
		// THEIR marker, so it gets the same validation the marker-present
		// path above applies — a racing writer from a future build that
		// bumped `version` is exactly the case that must not slip through.
		if err := checkMarker(t, marker); err != nil {
			return err
		}
	case out == transport.Ambiguous:
		return fmt.Errorf("marker creation ambiguous for %s; re-run to reconcile", root)
	}
	return ensureSubdirs(t, root)
}

// checkMarker downloads the marker and validates its CONTENT. It uses the same
// download-then-read shape as readRef and readLock: the CLI's `download` takes
// a destination FOLDER, so the file lands under whatever name it has remotely.
//
// Every failure here is a hard refusal naming what was actually found. The
// helper never guesses whether a folder is one of its repos, and it never
// adopts a repo written to a contract it does not implement.
func checkMarker(t transport.Transport, p string) error {
	dir, err := os.MkdirTemp("", "gpb-marker-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := t.ReadTo(p, dir); err != nil {
		return fmt.Errorf("refusing to use %s: its %s could not be read: %w", dirOf(p), MarkerName, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return fmt.Errorf("refusing to use %s: %s downloaded no content", dirOf(p), MarkerName)
	}
	raw, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		return err
	}
	var mb markerBody
	if err := json.Unmarshal(raw, &mb); err != nil {
		return fmt.Errorf("refusing to use %s: its %s could not be parsed as JSON (%q); "+
			"this folder is not a git-remote-proton repo, or its marker is damaged",
			dirOf(p), MarkerName, strings.TrimSpace(string(raw)))
	}
	if mb.Format != markerFormat {
		return fmt.Errorf("refusing to use %s: its %s declares format %q, not %q; "+
			"this folder belongs to something else", dirOf(p), MarkerName, mb.Format, markerFormat)
	}
	if mb.Version != markerVersion {
		return fmt.Errorf("refusing to use %s: its %s declares version %d, and this build "+
			"only implements version %d; a newer git-remote-proton wrote this repo and this "+
			"one cannot honour its layout", dirOf(p), MarkerName, mb.Version, markerVersion)
	}
	return nil
}

// dirOf returns the parent of a remote (always POSIX) path, for error messages
// that should name the repo root rather than the marker file inside it.
func dirOf(p string) string {
	if i := strings.LastIndex(p, "/"); i > 0 {
		return p[:i]
	}
	return p
}

func ensureSubdirs(t transport.Transport, root string) error {
	for _, d := range []string{"refs", "refs/heads", "refs/tags", "packs"} {
		if err := t.EnsureDir(root + "/" + d); err != nil {
			// Wrapped: the bare error names neither the subdir that failed
			// nor the root it belongs to, so a partial-initialisation failure
			// gave an operator nothing to act on.
			return fmt.Errorf("ensure dir %s/%s: %w", root, d, err)
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

// windowsReserved are the DOS device names. Windows refuses them as filenames;
// git accepts them as ref names.
//
// checkStageableLeaf applies this list on EVERY platform, not only Windows.
// That is deliberate and it is about portability, not about what the local
// host can do: a repo pushed from Linux with a ref named refs/heads/aux would
// be unusable from Windows, because staging that ref requires a local file of
// that name (probe C11) and Windows cannot create one. Refusing it everywhere
// keeps a repo usable from every machine, instead of making its usability
// depend on which machine happened to push first.
var windowsReserved = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// checkStageableLeaf rejects leaf names that cannot survive a local staging
// path. leaf must be a single path component — stagedFile places it directly
// under a temp dir, so any '/' or '\' is rejected outright as a separator, not
// interpreted as a subpath. Beyond that, the set is small because git already
// forbids space, control characters, '?', '*', '[', '~', '^' and ':' in ref
// names; what remains is brace globbing (probe C13: 0.7.0 still glob-expands
// '{') and Windows device names. Refusing these is also consistent — such a
// ref could never be UPDATED on this transport, so accepting the create would
// promise what the update cannot keep.
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
