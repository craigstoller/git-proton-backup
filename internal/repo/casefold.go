package repo

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Case-variant refs (spec §5, D2(a), Amendment A1). Proton's namespace keeps
// refs/heads/main and refs/heads/Main side by side, but a Windows clone of
// such a remote mishandles the pair silently (spec F13). Push therefore
// refuses, on every platform, a create that would add such a pair. The
// helpers here are the rule's pure half; Push's phase 2 applies them.

// caseKey maps s to a key that two strings share exactly when
// strings.EqualFold calls them equal (Unicode simple folding, not ASCII and
// not full folding: "ß" and "SS" stay distinct). Each valid rune becomes the
// smallest rune in its unicode.SimpleFold orbit, which is a canonical member
// of the equivalence class EqualFold tests.
//
// Invalid UTF-8 bytes are copied through unfolded. strings.EqualFold decodes
// every invalid byte to U+FFFD and so calls "a\xffb" and "a\xfeb" equal;
// copying them keeps distinct invalid bytes distinct. A literal, validly
// encoded U+FFFD is a rune like any other and is folded (to itself).
func caseKey(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			b.WriteByte(s[i])
			i++
			continue
		}
		b.WriteRune(minFold(r))
		i += size
	}
	return b.String()
}

// minFold returns the smallest rune in r's simple-folding orbit.
// unicode.SimpleFold walks the orbit in increasing order and wraps, so the
// loop visits every member exactly once before returning to r.
func minFold(r rune) rune {
	m := r
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		if f < m {
			m = f
		}
	}
	return m
}

// refPrefixes returns ref's component chains from the namespace component
// down: "refs/heads/grp/a" gives ["heads", "heads/grp", "heads/grp/a"]. The
// refs/ root is fixed, so it is not a prefix. A name outside refs/ gives nil.
//
// '/' has no case variants, so two prefixes can fold equal only at the same
// depth; that is what lets caseConflicts compare them depth by depth.
func refPrefixes(ref string) []string {
	rest, ok := strings.CutPrefix(ref, "refs/")
	if !ok || rest == "" {
		return nil
	}
	var out []string
	for i := 0; i < len(rest); i++ {
		if rest[i] == '/' {
			out = append(out, rest[:i])
		}
	}
	return append(out, rest)
}

// caseConflict is one ref a create conflicts with. Ref is the conflicting
// ref in the checked set; Ours (the create's) and Theirs (Ref's) are the
// differently spelled prefixes that share a caseKey.
type caseConflict struct{ Ref, Ours, Theirs string }

// caseConflicts checks each candidate against every OTHER ref in set and
// returns, per candidate that has any, one caseConflict per conflicting Ref,
// sorted by Ref. A candidate conflicts with a ref when some prefix of each
// is spelled differently but folds equal (spec §5); the pair reported is the
// first in (Ours, Theirs) order, which is the shallowest, because a shallower
// prefix is a proper prefix of every deeper one. Candidates with no conflict
// are absent from the result.
//
// The result depends only on the contents of set and candidates, never on
// map iteration or input order: each Ref carries exactly one prefix at a
// given depth, so the first pair recorded for it is fixed, and the slice is
// sorted. A candidate need not be in set; it is never compared with itself.
func caseConflicts(set map[string]bool, candidates []string) map[string][]caseConflict {
	// index: caseKey(prefix) -> spelling of that prefix -> refs carrying it.
	// Built once, so each candidate costs a lookup per prefix rather than a
	// scan of the whole set.
	index := map[string]map[string][]string{}
	for ref := range set {
		for _, p := range refPrefixes(ref) {
			k := caseKey(p)
			spellings := index[k]
			if spellings == nil {
				spellings = map[string][]string{}
				index[k] = spellings
			}
			spellings[p] = append(spellings[p], ref)
		}
	}

	out := map[string][]caseConflict{}
	for _, cand := range candidates {
		var cs []caseConflict
		seen := map[string]bool{}
		for _, ours := range refPrefixes(cand) {
			for theirs, refs := range index[caseKey(ours)] {
				if theirs == ours {
					continue
				}
				for _, ref := range refs {
					if ref == cand || seen[ref] {
						continue
					}
					seen[ref] = true
					cs = append(cs, caseConflict{Ref: ref, Ours: ours, Theirs: theirs})
				}
			}
		}
		if len(cs) > 0 {
			sort.Slice(cs, func(i, j int) bool { return cs[i].Ref < cs[j].Ref })
			out[cand] = cs
		}
	}
	return out
}

// caseConflictMessage is the refusal text for a create, naming every
// conflicting ref, in cs's (sorted) order, each with its own prefix pair.
func caseConflictMessage(dst string, cs []caseConflict) string {
	parts := make([]string, len(cs))
	for i, c := range cs {
		parts[i] = fmt.Sprintf("%s (%s vs %s)", c.Ref, c.Ours, c.Theirs)
	}
	return fmt.Sprintf("%s conflicts by letter case with %s: a Windows clone would mishandle "+
		"the pair (README, Honest limits), so this create is refused on every platform",
		dst, strings.Join(parts, ", "))
}
