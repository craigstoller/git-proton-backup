package repo

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/gitcmd"
	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// ResolveIdxCacheDir returns (creating it if needed) the per-repo sidecar
// cache directory for the remote at root:
//
//	<git-common-dir>/proton-v2/idx-cache/<sha256-16 of root>/
//
// The common dir — NOT --git-path <name>, which resolves arbitrary names
// into the per-worktree ADMIN dir — so linked worktrees share one cache. The
// answer is validated before being trusted as a path for the same reason
// validateObjectsPackPath exists: RevParse's result is combined
// stdout+stderr, and a merged git warning must not become a directory name.
// It is then absolutised: --git-common-dir answers RELATIVE in an ordinary
// repo (".git"), relative to the -C directory it ran under, i.e. gitDir —
// the Stage 3a relative-GIT_DIR lesson applied before it can bite.
//
// An error return means "no cache this run"; callers pass cacheDir="" and
// every sidecar lives in the fetch's temp dir instead. The key is a hash so
// no character of the remote path ever reaches the filesystem; the "remote"
// breadcrumb file records the plain path for humans (best-effort).
func ResolveIdxCacheDir(gitDir, root string) (string, error) {
	out, code, err := gitcmd.RevParse(gitDir, "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("cannot resolve the git common dir for %s: %s: %w",
			gitDir, strings.TrimSpace(out), err)
	}
	if code != 0 {
		return "", fmt.Errorf("rev-parse --git-common-dir exited %d: %s", code, out)
	}
	if out == "" || strings.ContainsAny(out, "\r\n") {
		return "", fmt.Errorf("rev-parse --git-common-dir returned %q, which cannot be "+
			"trusted as a path (a git warning may have been merged into the output)", out)
	}
	common := out
	if !filepath.IsAbs(common) {
		common = filepath.Join(gitDir, common)
	}
	abs, err := filepath.Abs(common)
	if err != nil {
		return "", fmt.Errorf("cannot absolutise common dir %q: %w", common, err)
	}
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(root)))[:16]
	dir := filepath.Join(abs, "proton-v2", "idx-cache", key)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	// Best-effort: a failed breadcrumb never fails the cache.
	_ = os.WriteFile(filepath.Join(dir, "remote"), []byte(root+"\n"), 0o644)
	return dir, nil
}

// EnsureSidecar returns a local path holding stem's .idx: the cached copy
// when present, else a fresh download into fallbackDir with a best-effort
// copy installed into the cache. err is non-nil ONLY for a failed download —
// cache trouble degrades with a stderr warning, never fails the caller
// (spec: "cache trouble is never allowed to become fetch trouble").
func EnsureSidecar(t transport.Transport, root, cacheDir, fallbackDir, stem string) (string, bool, error) {
	name := stem + ".idx"
	if cacheDir != "" {
		if p := filepath.Join(cacheDir, name); fileExists(p) {
			return p, true, nil
		}
	}
	p, err := downloadSidecar(t, root, fallbackDir, name)
	if err != nil {
		return "", false, err
	}
	installIntoCache(cacheDir, p, name)
	return p, false, nil
}

// RefreshSidecar unconditionally re-downloads stem's .idx, deleting any
// stale fallback copy first and replacing the cached copy when possible. The
// returned path is the guaranteed-fresh fallback copy.
//
// The delete-first step is NOT the old pack-dir residue rule (deleted in Task
// 6, when quarantine staging replaced it): it is this function's own
// precondition. ReadTo's behaviour onto an EXISTING file is unpinned — C2's
// identical-content skip means a download can be a silent no-op — so a
// refresh that did not clear its own stale copy first could return the very
// bytes it exists to replace. Under quarantine staging the sidecar cache is
// the only place a stale copy can survive a fetch, which is exactly why the
// clearing has to happen here rather than being inherited from a general
// download-residue rule that no longer exists.
func RefreshSidecar(t transport.Transport, root, cacheDir, fallbackDir, stem string) (string, error) {
	name := stem + ".idx"
	if err := os.Remove(filepath.Join(fallbackDir, name)); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	// Evict the stale cache entry BEFORE attempting the install: if the
	// install's rename then fails, the next run gets a cache MISS rather
	// than silently trusting the very bytes this refresh exists to replace.
	// Best-effort like every cache write — a failed evict only warns.
	if cacheDir != "" {
		if err := os.Remove(filepath.Join(cacheDir, name)); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "git-remote-proton: cannot evict stale cache entry %s: %v\n",
				name, err)
		}
	}
	p, err := downloadSidecar(t, root, fallbackDir, name)
	if err != nil {
		return "", err
	}
	installIntoCache(cacheDir, p, name)
	return p, nil
}

func downloadSidecar(t transport.Transport, root, destDir, name string) (string, error) {
	if err := t.ReadTo(root+"/packs/"+name, destDir); err != nil {
		return "", fmt.Errorf("cannot download %s: %w", name, err)
	}
	return filepath.Join(destDir, name), nil
}

// installIntoCache copies src into cacheDir under name, via temp-file-then-
// rename in the SAME directory so a crash can never leave a half-written
// sidecar under a valid name (Go's os.Rename replaces an existing file on
// Windows, so a concurrent loser is simply overwritten with identical-
// meaning bytes). Entirely best-effort: every failure warns and returns.
func installIntoCache(cacheDir, src, name string) {
	if cacheDir == "" {
		return
	}
	warn := func(err error) {
		fmt.Fprintf(os.Stderr, "git-remote-proton: sidecar cache write failed (%v); "+
			"continuing without caching %s\n", err, name)
	}
	in, err := os.Open(src)
	if err != nil {
		warn(err)
		return
	}
	defer in.Close()
	tmp, err := os.CreateTemp(cacheDir, ".tmp-*")
	if err != nil {
		warn(err)
		return
	}
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		warn(err)
		return
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		warn(err)
		return
	}
	if err := os.Rename(tmp.Name(), filepath.Join(cacheDir, name)); err != nil {
		_ = os.Remove(tmp.Name())
		warn(err)
	}
}

// pruneStale removes cache entries whose pack no longer appears in the
// remote listing, plus leftover staging files. Runs only on fetches that
// enter discovery (an up-to-date fetch short-circuits before listing).
// v2 never deletes a pack, so firing is defensive, not expected. Best-effort
// throughout: a prune failure warns and moves on.
//
// Staleness is keyed on the COMPLETE-pair stems the caller kept, which is
// deliberately narrower than the design's name-in-listing rule: a cached
// .idx whose pack is currently incomplete (mid-push) is pruned here and
// simply re-downloaded if the pair later completes. Recorded as a
// post-implementation reconciliation in the Stage 3b spec.
func pruneStale(cacheDir string, keep map[string]bool) {
	if cacheDir == "" {
		return
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "git-remote-proton: cannot read sidecar cache for pruning: %v\n", err)
		return
	}
	for _, e := range entries {
		n := e.Name()
		stale := strings.HasPrefix(n, ".tmp-") ||
			(strings.HasSuffix(n, ".idx") && !keep[strings.TrimSuffix(n, ".idx")])
		if !stale {
			continue
		}
		if err := os.Remove(filepath.Join(cacheDir, n)); err != nil {
			fmt.Fprintf(os.Stderr, "git-remote-proton: cannot prune stale cache entry %s: %v\n", n, err)
		}
	}
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
