package repo

import (
	"fmt"
	"sort"
	"strings"
)

// The pure read checks (spec 2026-10-27, section 3; plan Task 7). They decide,
// from data the caller has already read, which refs a fetch or a push could
// write in the local repository and whether any of those writes would meet a
// second ref spelled differently on a case- and normalisation-insensitive
// filesystem such as default APFS. Nothing here reads git, the file system or
// the network, and every result is a function of the argument values alone,
// never of map iteration order. Every conflict test is caseConflicts, in
// casefold.go; there is no second matcher.
//
// The texts PairLines, PairWarning and DestLines return show each ref raw and
// again in strconv.QuoteToASCII form (%+q), because an NFC and an NFD spelling
// display identically. None ends in a newline, so a caller can join one into
// a longer message.

// RefPair is two advertised refs that conflict under one relation (PairLines).
// A sorts before B.
type RefPair struct {
	A, B string
	Rel  relation
}

// AdvertisedPairs finds every unordered pair of refs in refs (name to object
// id) that conflict under either relation, reports each once, and sorts the
// result by (A, B). Names outside refs/ pair with nothing. The relation of a
// pair is the one caseConflicts reports for it; a pair matching both is
// letter case.
func AdvertisedPairs(refs map[string]string) []RefPair {
	set := make(map[string]bool, len(refs))
	for r := range refs {
		set[r] = true
	}
	names := sortedKeys(refs)
	found := caseConflicts(set, names)
	var out []RefPair
	for _, a := range names {
		for _, c := range found[a] {
			// A pair is found from both of its members; keep the one found
			// from the smaller. The relation is the same from either side:
			// each ref has one prefix per depth, so both walks meet the same
			// two prefixes at the same depth.
			if a < c.Ref {
				out = append(out, RefPair{A: a, B: c.Ref, Rel: c.Rel})
			}
		}
	}
	return out
}

// PairLines is one line per pair, in the pairs' order, joined by newlines. A
// line is two spaces and
//
//	<A> (<oid A>, <%+q A>) and <B> (<oid B>, <%+q B>): by <relation>
func PairLines(pairs []RefPair, refs map[string]string) string {
	lines := make([]string, len(pairs))
	for i, p := range pairs {
		lines[i] = fmt.Sprintf("  %s (%s, %+q) and %s (%s, %+q): by %s",
			p.A, refs[p.A], p.A, p.B, refs[p.B], p.B, p.Rel)
	}
	return strings.Join(lines, "\n")
}

// pairWarningHeader is the first line of PairWarning.
const pairWarningHeader = "warning: this remote holds refs that differ only by letter case or " +
	"Unicode normalisation; see README, Honest limits:"

// PairWarning is the warning a list prints when the remote holds pairs: the
// header, a newline and PairLines. It is "" for no pairs.
func PairWarning(pairs []RefPair, refs map[string]string) string {
	if len(pairs) == 0 {
		return ""
	}
	return pairWarningHeader + "\n" + PairLines(pairs, refs)
}

// Refspec is one parsed fetch refspec (R4). Src and Dst are full ref names
// starting refs/, each with exactly one '*' or none, a wildcard on both sides
// or neither. NoDst is a spec with no destination, which fetches into
// FETCH_HEAD and writes no ref (PM2). A Negative spec has a Src only.
type Refspec struct {
	Src, Dst               string
	Force, Negative, NoDst bool
}

// ParseFetchRefspecs parses remote.<name>.fetch values (R4, PM2). It accepts
// "[+]<src>:<dst>", "[+]<src>" and "[+]<src>:" (both NoDst), and "^<src>"
// (negative). Every src and every non-empty dst must begin "refs/" and name
// something below it, with exactly one '*' on each side or none; a wildcard
// source needs a destination with a wildcard. Anything else is an error that
// names the value, because git expands what it does not recognise (HEAD, an
// abbreviated name) in ways this package does not model, so the helper
// refuses rather than guess. The result keeps the values' order.
func ParseFetchRefspecs(values []string) ([]Refspec, error) {
	var out []Refspec
	for _, v := range values {
		sp, err := parseFetchRefspec(v)
		if err != nil {
			return nil, err
		}
		out = append(out, sp)
	}
	return out, nil
}

func parseFetchRefspec(v string) (Refspec, error) {
	bad := func(format string, args ...any) (Refspec, error) {
		return Refspec{}, fmt.Errorf("fetch refspec %+q: %s", v, fmt.Sprintf(format, args...))
	}
	var sp Refspec
	rest := v
	switch {
	case strings.HasPrefix(rest, "^"):
		sp.Negative = true
		rest = rest[1:]
	case strings.HasPrefix(rest, "+"):
		sp.Force = true
		rest = rest[1:]
	}
	src, dst, hasColon := strings.Cut(rest, ":")
	if strings.Contains(dst, ":") {
		return bad("more than one ':'")
	}
	if sp.Negative && hasColon {
		return bad("a negative refspec has no destination")
	}
	if why := refspecNameProblem(src); why != "" {
		return bad("source %+q %s", src, why)
	}
	sp.Src = src
	srcGlob := strings.Contains(src, "*")
	if sp.Negative {
		return sp, nil
	}
	if dst == "" {
		if srcGlob {
			return bad("a wildcard source needs a destination with a wildcard")
		}
		sp.NoDst = true
		return sp, nil
	}
	if why := refspecNameProblem(dst); why != "" {
		return bad("destination %+q %s", dst, why)
	}
	if srcGlob != strings.Contains(dst, "*") {
		return bad("the wildcard must appear on both sides or neither")
	}
	sp.Dst = dst
	return sp, nil
}

// refspecNameProblem says why name cannot be a refspec side, or "" if it can:
// it must start "refs/" and name something below it, and hold at most one '*'.
func refspecNameProblem(name string) string {
	if !strings.HasPrefix(name, "refs/") || len(name) == len("refs/") {
		return `does not name a ref below "refs/" (only full ref names are supported; git expands anything else)`
	}
	if strings.Count(name, "*") > 1 {
		return "has more than one '*'"
	}
	return ""
}

// DefaultFetchRefspecs is the mapping git gives a remote called remoteName
// when it is created: +refs/heads/*:refs/remotes/<name>/*.
func DefaultFetchRefspecs(remoteName string) []Refspec {
	return []Refspec{{Src: "refs/heads/*", Dst: "refs/remotes/" + remoteName + "/*", Force: true}}
}

// refspecMatch matches name against one side of a refspec as git's
// match_name_with_pattern does. A side with no '*' matches only itself. One
// with a '*' matches a name that starts with the text before the star and
// ends with the text after it, the two not overlapping, whatever lies between
// (slashes included, and nothing at all); mid is that middle part.
func refspecMatch(pat, name string) (mid string, ok bool) {
	i := strings.IndexByte(pat, '*')
	if i < 0 {
		return "", name == pat
	}
	pre, suf := pat[:i], pat[i+1:]
	if len(name) < len(pre)+len(suf) || !strings.HasPrefix(name, pre) || !strings.HasSuffix(name, suf) {
		return "", false
	}
	return name[len(pre) : len(name)-len(suf)], true
}

// refspecExpand is dst with its '*', if any, replaced by mid.
func refspecExpand(dst, mid string) string {
	if i := strings.IndexByte(dst, '*'); i >= 0 {
		return dst[:i] + mid + dst[i+1:]
	}
	return dst
}

// mapRef is the mapping behind MapRef and PushTrackingRefs. Only a positive
// spec with a destination (not NoDst, Dst not empty) yields a result.
func mapRef(specs []Refspec, ref string, honourNegatives bool) []string {
	if honourNegatives {
		for _, sp := range specs {
			if sp.Negative {
				if _, ok := refspecMatch(sp.Src, ref); ok {
					return nil
				}
			}
		}
	}
	var out []string
	for _, sp := range specs {
		if sp.Negative || sp.NoDst || sp.Dst == "" {
			continue
		}
		if mid, ok := refspecMatch(sp.Src, ref); ok {
			out = append(out, refspecExpand(sp.Dst, mid))
		}
	}
	return sortedUnique(out)
}

// MapRef is the fetch mapping: the refs a fetch of the remote's ref writes. A
// ref any negative spec matches maps to nothing; otherwise each matching
// positive spec with a destination yields it, by substitution for a wildcard
// and literally otherwise. The result is sorted and has no repeats; it is nil
// for none.
func MapRef(specs []Refspec, ref string) []string {
	return mapRef(specs, ref, true)
}

// PushTrackingRefs is the push mapping: MapRef without the negative specs
// (R6), because git's push tracking lookup can still pick a positive wildcard
// that a negative refspec was meant to exclude.
func PushTrackingRefs(specs []Refspec, ref string) []string {
	return mapRef(specs, ref, false)
}

// FetchDestinations maps each local ref a fetch of the advertised refs could
// write to the advertised ref it comes from, the smallest by name if several
// do. The destinations are:
//   - every MapRef result;
//   - every advertised refs/tags/ name, to itself, whether or not a negative
//     spec matches it, because tag backfill runs without negative filtering
//     (R6);
//   - "<p>/HEAD", sourced "HEAD", for each positive spec that maps
//     refs/heads/* to "<p>/*": the tracking HEAD that a clone, or a fetch under
//     remote.<name>.followRemoteHEAD, writes (R6). On default APFS it aliases
//     a branch called head.
func FetchDestinations(advertised map[string]string, specs []Refspec) map[string]string {
	dests := map[string]string{}
	for ref := range advertised {
		for _, d := range MapRef(specs, ref) {
			setSmallest(dests, d, ref)
		}
		if strings.HasPrefix(ref, "refs/tags/") {
			setSmallest(dests, ref, ref)
		}
	}
	for _, sp := range specs {
		if sp.Negative || sp.NoDst || sp.Src != "refs/heads/*" {
			continue
		}
		if p, ok := strings.CutSuffix(sp.Dst, "/*"); ok && p != "" && !strings.Contains(p, "*") {
			setSmallest(dests, p+"/HEAD", "HEAD")
		}
	}
	return dests
}

// ExpandSymrefs returns dests plus, for each destination that is a local
// symbolic ref, its target with the same source, repeating for the targets
// until none is new (R6): git follows a symbolic tracking ref when it updates
// one. symrefs maps each local symbolic ref to its target; an entry with an
// empty target is not symbolic and adds nothing. A target that is already a
// destination keeps its own source; one reached from several destinations in
// the same round takes the smallest source. dests and symrefs are not changed.
func ExpandSymrefs(dests map[string]string, symrefs map[string]string) map[string]string {
	out := make(map[string]string, len(dests))
	frontier := make([]string, 0, len(dests))
	for d, src := range dests {
		out[d] = src
		frontier = append(frontier, d)
	}
	for len(frontier) > 0 {
		added := map[string]string{}
		for _, d := range frontier {
			target := symrefs[d]
			if target == "" {
				continue
			}
			if _, have := out[target]; have {
				continue
			}
			setSmallest(added, target, out[d])
		}
		frontier = frontier[:0]
		for t, src := range added {
			out[t] = src
			frontier = append(frontier, t)
		}
	}
	return out
}

// PruneCandidates maps to "" each ref of local that a positive spec's
// destination (a pattern, or a literal name) covers and that dests does not
// hold: the refs a pruning fetch could delete (R6). The empty source is what
// DestLines reads as "may be pruned". Negative specs are ignored, which only
// widens the set. Only local's keys matter.
func PruneCandidates(specs []Refspec, dests map[string]string, local map[string]bool) map[string]string {
	out := map[string]string{}
	for ref := range local {
		if _, written := dests[ref]; written {
			continue
		}
		for _, sp := range specs {
			if sp.Negative || sp.NoDst || sp.Dst == "" {
				continue
			}
			if _, ok := refspecMatch(sp.Dst, ref); ok {
				out[ref] = ""
				break
			}
		}
	}
	return out
}

// DestConflict is a ref a fetch could write (Dest, from Src) set against a
// second ref that differs only by letter case or normalisation (Other), with
// the conflict as caseConflicts reports it for Dest (C.Ref is Other).
// OtherSrc is non-empty only when Other is itself a destination of this fetch.
// An empty Src marks a Dest that is only a prune candidate.
type DestConflict struct {
	Dest, Src, Other, OtherSrc string
	C                          caseConflict
}

// DestinationConflicts checks every destination against the local refs and
// the other destinations, with caseConflicts over local plus the destinations.
// It returns one entry per unordered pair that has a destination as a member,
// sorted by (Dest, Other); two destinations that conflict are reported once,
// with the smaller as Dest and the larger as Other (OtherSrc set). A pair of
// local refs alone is not reported. Only local's keys matter.
func DestinationConflicts(dests map[string]string, local map[string]bool) []DestConflict {
	set := make(map[string]bool, len(local)+len(dests))
	for r := range local {
		set[r] = true
	}
	for d := range dests {
		set[d] = true
	}
	cands := sortedKeys(dests)
	found := caseConflicts(set, cands)
	seen := map[string]bool{}
	var out []DestConflict
	for _, d := range cands {
		for _, c := range found[d] {
			k := pairKey(d, c.Ref)
			if seen[k] {
				continue
			}
			seen[k] = true
			dc := DestConflict{Dest: d, Src: dests[d], Other: c.Ref, C: c}
			if otherSrc, isDest := dests[c.Ref]; isDest {
				dc.OtherSrc = otherSrc
			}
			out = append(out, dc)
		}
	}
	sortConflicts(out)
	return out
}

// UpToDateTracking is R1's set of tracking refs an "Everything up-to-date"
// push can rewrite (PM1): for each advertised ref R that local also holds by
// the exact same name, each of PushTrackingRefs(specs, R) maps to R, the
// smallest R if two share a tracking ref. A ref the clone holds only in
// another spelling is not up to date by name and is not here. Only local's
// keys matter.
func UpToDateTracking(advertised map[string]string, specs []Refspec, local map[string]bool) map[string]string {
	out := map[string]string{}
	for r := range advertised {
		if _, held := local[r]; !held {
			continue
		}
		for _, t := range PushTrackingRefs(specs, r) {
			setSmallest(out, t, r)
		}
	}
	return out
}

// LocalConflicts is DestinationConflicts over the local refs less the keys of
// dests: each key of dests is checked against the other local refs alone,
// never against another key, and OtherSrc is always empty. It is the check of
// prune candidates (which are local refs already) and of UpToDateTracking.
// Only local's keys matter.
func LocalConflicts(dests map[string]string, local map[string]bool) []DestConflict {
	set := make(map[string]bool, len(local))
	for r := range local {
		if _, isDest := dests[r]; !isDest {
			set[r] = true
		}
	}
	cands := sortedKeys(dests)
	found := caseConflicts(set, cands)
	var out []DestConflict
	for _, d := range cands {
		for _, c := range found[d] {
			out = append(out, DestConflict{Dest: d, Src: dests[d], Other: c.Ref, C: c})
		}
	}
	sortConflicts(out)
	return out
}

// MergeConflicts is a, then those of b whose unordered {Dest, Other} pair a
// does not report, so the destination pass and the prune pass of one fetch
// check never name a pair twice. A pair b repeats keeps only its smallest
// entry (the order of DestConflict's fields), so the result does not depend on
// b's order. The result is sorted by (Dest, Other); a and b are not changed.
func MergeConflicts(a, b []DestConflict) []DestConflict {
	reported := make(map[string]bool, len(a))
	out := make([]DestConflict, 0, len(a)+len(b))
	for _, c := range a {
		reported[pairKey(c.Dest, c.Other)] = true
		out = append(out, c)
	}
	best := map[string]DestConflict{}
	for _, c := range b {
		k := pairKey(c.Dest, c.Other)
		if reported[k] {
			continue
		}
		if cur, ok := best[k]; !ok || lessConflict(c, cur) {
			best[k] = c
		}
	}
	for _, c := range best {
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil
	}
	sortConflicts(out)
	return out
}

// DestLines is one line per conflict, in cs's order, joined by newlines. A
// line is two spaces and
//
//	<Dest> (<origin>) and <Other> (<other>) (<%+q Dest> vs <%+q Other>): by <relation>
//
// <origin> is "from <Src>, <oid>" for a destination with a source (refs
// gives the oid; a source refs does not hold, such as the synthesized HEAD,
// reads "from <Src>"), or "may be pruned" for an empty source. <other> is
// "also written by this fetch, from <OtherSrc>" when OtherSrc is set, and
// "local" otherwise.
func DestLines(cs []DestConflict, refs map[string]string) string {
	lines := make([]string, len(cs))
	for i, c := range cs {
		origin := "may be pruned"
		if c.Src != "" {
			origin = "from " + c.Src
			if oid := refs[c.Src]; oid != "" {
				origin += ", " + oid
			}
		}
		other := "local"
		if c.OtherSrc != "" {
			other = "also written by this fetch, from " + c.OtherSrc
		}
		lines[i] = fmt.Sprintf("  %s (%s) and %s (%s) (%+q vs %+q): by %s",
			c.Dest, origin, c.Other, other, c.Dest, c.Other, c.C.Rel)
	}
	return strings.Join(lines, "\n")
}

// pairKey names the unordered pair {x, y}. Ref names hold no NUL.
func pairKey(x, y string) string {
	if y < x {
		x, y = y, x
	}
	return x + "\x00" + y
}

// lessConflict is a total order on conflicts: by Dest, then Other, then the
// remaining fields, so a sort of them has one outcome.
func lessConflict(x, y DestConflict) bool {
	switch {
	case x.Dest != y.Dest:
		return x.Dest < y.Dest
	case x.Other != y.Other:
		return x.Other < y.Other
	case x.Src != y.Src:
		return x.Src < y.Src
	case x.OtherSrc != y.OtherSrc:
		return x.OtherSrc < y.OtherSrc
	case x.C.Ref != y.C.Ref:
		return x.C.Ref < y.C.Ref
	case x.C.Ours != y.C.Ours:
		return x.C.Ours < y.C.Ours
	case x.C.Theirs != y.C.Theirs:
		return x.C.Theirs < y.C.Theirs
	}
	return x.C.Rel < y.C.Rel
}

func sortConflicts(cs []DestConflict) {
	sort.Slice(cs, func(i, j int) bool { return lessConflict(cs[i], cs[j]) })
}

// setSmallest records m[k] = v unless m already holds a smaller v for k.
func setSmallest(m map[string]string, k, v string) {
	if cur, ok := m[k]; !ok || v < cur {
		m[k] = v
	}
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// sortedUnique sorts s and drops repeats; it is nil for an empty s.
func sortedUnique(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	sort.Strings(s)
	out := s[:1]
	for _, x := range s[1:] {
		if x != out[len(out)-1] {
			out = append(out, x)
		}
	}
	return out
}
