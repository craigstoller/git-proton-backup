package repo

import (
	"errors"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/gitcmd"
	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// errCacheSuspect marks failures the self-heal round may cure: a lying or
// stale sidecar cache can present as "remote incomplete", as "no progress",
// or as a download/checksum failure on a pack a truthful map would never
// have selected. Fetch gives every such failure ONE fresh rebuild before its
// fatal; a fatal after healing genuinely indicts the remote or transport.
var errCacheSuspect = errors.New(
	"this can be caused by a stale or corrupt sidecar cache")

// packMemberRe is the normative name grammar. Listed names are remote-
// controlled input: nothing failing this pattern may ever be joined into a
// local path, become a cache key, or be downloaded.
var packMemberRe = regexp.MustCompile(`^pack-[0-9a-f]{40}\.(pack|idx)$`)

// listCompletePacks returns the sorted stems of every grammar-valid,
// COMPLETE pair in the remote listing. A .pack with no .idx is the normal
// signature of a concurrent or crashed push observed mid-publication (Push
// uploads the pack first) — skipped with a note, not an error. A .idx with
// no .pack is the unrepairable direction — silently never a download target.
func listCompletePacks(t transport.Transport, root string) ([]string, error) {
	nodes, err := t.List(root + "/packs")
	if err != nil {
		return nil, fmt.Errorf("cannot list %s/packs: %w", root, err)
	}
	members := map[string]map[string]bool{}
	for _, n := range nodes {
		if n.IsDir {
			continue
		}
		if !packMemberRe.MatchString(n.Name) {
			if strings.HasSuffix(n.Name, ".pack") || strings.HasSuffix(n.Name, ".idx") {
				fmt.Fprintf(os.Stderr, "git-remote-proton: ignoring %s/packs/%s: not a "+
					"valid pack member name\n", root, n.Name)
			}
			continue
		}
		ext := path.Ext(n.Name)
		stem := strings.TrimSuffix(n.Name, ext)
		if members[stem] == nil {
			members[stem] = map[string]bool{}
		}
		members[stem][ext] = true
	}
	var stems []string
	for stem, m := range members {
		switch {
		case m[".pack"] && m[".idx"]:
			stems = append(stems, stem)
		case m[".pack"]:
			fmt.Fprintf(os.Stderr, "git-remote-proton: %s/packs/%s.pack has no index yet "+
				"(a push may be in flight); skipping\n", root, stem)
		}
	}
	sort.Strings(stems)
	return stems, nil
}

// packMap is the in-memory object-to-pack map plus the local sidecar paths
// it was built from. It is rebuilt WHOLE whenever any sidecar is refreshed —
// per-entry surgery invites exactly the desynchronisation both round-1
// reviewers flagged, and rebuilding costs one show-index per pack.
type packMap struct {
	oidPacks    map[string][]string // oid -> sorted stems of packs holding it
	sidecars    map[string]string   // stem -> local path of its .idx
	cacheDir    string              // "" when the cache is unusable this run
	fallbackDir string
}

// buildPackMap ensures every stem's sidecar is locally readable and builds
// the map. A CACHED sidecar show-index rejects self-heals as a cache miss
// (discard, re-download once); a FRESH one that still fails is fatal naming
// the file — the remote's sidecar is bad.
func buildPackMap(t transport.Transport, root, cacheDir, fallbackDir string, stems []string) (*packMap, error) {
	pm := &packMap{
		sidecars:    map[string]string{},
		cacheDir:    cacheDir,
		fallbackDir: fallbackDir,
	}
	for _, stem := range stems {
		p, cached, err := EnsureSidecar(t, root, cacheDir, fallbackDir, stem)
		if err != nil {
			return nil, err
		}
		if _, err := gitcmd.ShowIndex(p); err != nil && cached {
			fmt.Fprintf(os.Stderr, "git-remote-proton: cached sidecar %s.idx is corrupt "+
				"(%v); re-downloading\n", stem, err)
			if p, err = RefreshSidecar(t, root, cacheDir, fallbackDir, stem); err != nil {
				return nil, err
			}
		}
		pm.sidecars[stem] = p
	}
	if err := pm.rebuildFromSidecars(); err != nil {
		return nil, err
	}
	return pm, nil
}

// refreshPackMap is the self-heal round's map: a fresh sidecar for every
// stem, then a full rebuild. After it, every map entry was derived from
// bytes downloaded THIS run.
func refreshPackMap(t transport.Transport, root, cacheDir, fallbackDir string, stems []string) (*packMap, error) {
	pm := &packMap{
		sidecars:    map[string]string{},
		cacheDir:    cacheDir,
		fallbackDir: fallbackDir,
	}
	for _, stem := range stems {
		p, err := RefreshSidecar(t, root, cacheDir, fallbackDir, stem)
		if err != nil {
			return nil, err
		}
		pm.sidecars[stem] = p
	}
	if err := pm.rebuildFromSidecars(); err != nil {
		return nil, err
	}
	return pm, nil
}

// rebuildFromSidecars rebuilds oidPacks from the current sidecar set. A
// failure here is fatal to the fetch: every sidecar in pm.sidecars has
// already had its one self-heal chance.
func (pm *packMap) rebuildFromSidecars() error {
	pm.oidPacks = map[string][]string{}
	for stem, p := range pm.sidecars {
		oids, err := gitcmd.ShowIndex(p)
		if err != nil {
			return fmt.Errorf("sidecar for %s is unreadable even freshly downloaded — "+
				"the remote's index is bad: %w", stem, err)
		}
		for _, oid := range oids {
			pm.oidPacks[oid] = append(pm.oidPacks[oid], stem)
		}
	}
	for oid := range pm.oidPacks {
		sort.Strings(pm.oidPacks[oid])
	}
	return nil
}

// greedyCover picks which packs to download for the missing frontier:
// packs that are some OID's ONLY un-downloaded candidate are forced first;
// remaining uncovered OIDs are covered greedily by the candidate holding
// most of them, ties broken lexicographically. Deterministic; never returns
// a pack contributing nothing to the frontier. Both failure modes wrap
// errCacheSuspect (see its doc).
func greedyCover(missing []string, oidPacks map[string][]string, downloaded map[string]bool) ([]string, error) {
	// Both failure scans run over the WHOLE frontier before erroring, and the
	// error names every offender, sorted: rev-list's output order is
	// unspecified, so erroring at the first offender would make the named OID
	// nondeterministic — and a diagnosis that names one of forty missing
	// objects at random is worse than one that names all forty.
	var noCandidate, noProgress []string
	uncovered := map[string]bool{}
	for _, oid := range missing {
		cands := oidPacks[oid]
		if len(cands) == 0 {
			noCandidate = append(noCandidate, oid)
			continue
		}
		fresh := 0
		for _, c := range cands {
			if !downloaded[c] {
				fresh++
			}
		}
		if fresh == 0 {
			noProgress = append(noProgress, oid)
			continue
		}
		uncovered[oid] = true
	}
	if len(noCandidate) > 0 {
		sort.Strings(noCandidate)
		return nil, fmt.Errorf("no pack on the remote contains missing object(s) %s: %w",
			strings.Join(noCandidate, ", "), errCacheSuspect)
	}
	if len(noProgress) > 0 {
		sort.Strings(noProgress)
		return nil, fmt.Errorf("object(s) %s are still missing although every pack claiming "+
			"to contain them was already downloaded and verified (no progress is possible): %w",
			strings.Join(noProgress, ", "), errCacheSuspect)
	}
	chosen := map[string]bool{}
	covered := func() {
		for oid := range uncovered {
			for _, c := range oidPacks[oid] {
				if chosen[c] {
					delete(uncovered, oid)
					break
				}
			}
		}
	}
	// Pass 1: forced singles.
	for oid := range uncovered {
		var fresh []string
		for _, c := range oidPacks[oid] {
			if !downloaded[c] {
				fresh = append(fresh, c)
			}
		}
		if len(fresh) == 1 {
			chosen[fresh[0]] = true
		}
	}
	covered()
	// Pass 2: greedy most-covering, ties lexicographic.
	for len(uncovered) > 0 {
		count := map[string]int{}
		for oid := range uncovered {
			for _, c := range oidPacks[oid] {
				if !downloaded[c] && !chosen[c] {
					count[c]++
				}
			}
		}
		best := ""
		for c, n := range count {
			if best == "" || n > count[best] || (n == count[best] && c < best) {
				best = c
			}
		}
		if best == "" {
			// Unreachable given the per-OID fresh check above — but the
			// defensive arm must FAIL CLOSED: a silent partial cover would
			// download too little and push the failure downstream to
			// ConnectivityOK with a worse diagnosis.
			return nil, fmt.Errorf("internal: greedy cover could not cover %d remaining "+
				"missing objects: %w", len(uncovered), errCacheSuspect)
		}
		chosen[best] = true
		covered()
	}
	out := make([]string, 0, len(chosen))
	for c := range chosen {
		out = append(out, c)
	}
	sort.Strings(out)
	return out, nil
}
