package repo

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// Case-variant refs (spec §5, D2(a), Amendment A1). Proton's namespace keeps
// refs/heads/main and refs/heads/Main side by side, but a Windows clone of
// such a remote mishandles the pair silently (spec F13). Push therefore
// refuses, on every platform, a create that would add such a pair. The
// helpers here are the rule's pure half; Push's phase 2 applies them.
//
// The macOS port (spec 2026-10-27, section 3, "The rule") widens the pair to
// either of two relations: letter case (caseKey), and canonical caseless
// equivalence (canonKey), which an NFC name and its NFD twin share and which
// a Mac clone mishandles as Windows mishandles case. Neither relation
// contains the other, so both are checked, pairwise, with no transitive
// closure.

// caseKey maps s to a key that two strings share exactly when Unicode simple
// case folding at the pinned version (unicodeVersion) calls them equal: not
// ASCII, and not full folding, so "ß" and "SS" stay distinct. Each valid rune
// becomes its simple folding target (foldRune: the C or S entry of
// CaseFolding.txt, or the rune itself), which every member of a fold class
// shares. The table is the generated pinned one, never Go's own, so every
// build keys alike whatever Go compiled it (spec 2026-10-27, section 3).
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
		b.WriteRune(foldRune(r))
		i += size
	}
	return b.String()
}

// refPrefixes returns ref's component chains from the namespace component
// down: "refs/heads/grp/a" gives ["heads", "heads/grp", "heads/grp/a"]. The
// refs/ root is fixed, so it is not a prefix. A name outside refs/ gives nil.
//
// '/' has no case variants (TestNothingFoldsToOrFromSlash checks it), and no
// canonical decomposition produces or consumes it (gen_unicode.go's check 4
// asserts it), so two prefixes can share either key only at the same depth;
// that is what lets caseConflicts compare them depth by depth.
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

// relation names which of the create rule's two equivalences a conflicting
// pair matched (spec 2026-10-27, section 3, "The rule"). A pair that matches
// both is reported as relCase.
type relation uint8

const (
	// relCase: the two prefixes share caseKey (D2's relation). A Windows or
	// a Mac clone mishandles the pair: NTFS and default APFS are both
	// case-insensitive.
	relCase relation = 1
	// relCanon: the two prefixes share canonKey but not caseKey, as an NFC
	// spelling and its NFD twin do. A Mac clone mishandles the pair: APFS is
	// normalisation-insensitive, NTFS is not.
	relCanon relation = 2
)

// String is the relation as the refusal text names it.
func (r relation) String() string {
	switch r {
	case relCase:
		return "letter case"
	case relCanon:
		return "Unicode normalisation"
	}
	return fmt.Sprintf("relation(%d)", uint8(r))
}

// caseConflict is one ref a create conflicts with. Ref is the conflicting
// ref in the checked set; Ours (the create's) and Theirs (Ref's) are the
// differently spelled prefixes that share a key, and Rel says which.
type caseConflict struct {
	Ref, Ours, Theirs string
	Rel               relation
}

// caseConflicts checks each candidate against every OTHER ref in set and
// returns, per candidate that has any, one caseConflict per conflicting Ref,
// sorted by Ref. A candidate conflicts with a ref when some prefix of each
// is spelled differently but shares caseKey or canonKey (spec §5; spec
// 2026-10-27, section 3). Equal spellings never conflict. Candidates with no
// conflict are absent from the result.
//
// The pair recorded for a Ref is the first found walking the candidate's
// prefixes shallowest first and, at each depth, the case index before the
// canonical one: so it is the shallowest pair under either relation, and at
// equal depth a case match, which is also how a pair matching both relations
// is reported once. Both relations are pairwise; nothing is closed
// transitively, so two refs that each conflict with a third need not
// conflict with each other.
//
// The result depends only on the contents of set and candidates, never on
// map iteration or input order: each Ref carries exactly one prefix at a
// given depth, so it sits under one spelling in each index there, the first
// pair recorded for it is fixed, and the slice is sorted. A candidate need
// not be in set; it is never compared with itself.
func caseConflicts(set map[string]bool, candidates []string) map[string][]caseConflict {
	// One index per relation: key(prefix) -> spelling of that prefix -> refs
	// carrying it. Built once, so each candidate costs two lookups per prefix
	// rather than a scan of the whole set.
	type index map[string]map[string][]string
	add := func(ix index, k, p, ref string) {
		spellings := ix[k]
		if spellings == nil {
			spellings = map[string][]string{}
			ix[k] = spellings
		}
		spellings[p] = append(spellings[p], ref)
	}
	byCase, byCanon := index{}, index{}
	for ref := range set {
		for _, p := range refPrefixes(ref) {
			add(byCase, caseKey(p), p, ref)
			add(byCanon, canonKey(p), p, ref)
		}
	}

	out := map[string][]caseConflict{}
	for _, cand := range candidates {
		var cs []caseConflict
		seen := map[string]bool{}
		for _, ours := range refPrefixes(cand) {
			// Case first, so at equal depth a case match wins.
			for _, look := range [...]struct {
				ix  index
				key string
				rel relation
			}{
				{byCase, caseKey(ours), relCase},
				{byCanon, canonKey(ours), relCanon},
			} {
				for theirs, refs := range look.ix[look.key] {
					if theirs == ours {
						continue
					}
					for _, ref := range refs {
						if ref == cand || seen[ref] {
							continue
						}
						seen[ref] = true
						cs = append(cs, caseConflict{Ref: ref, Ours: ours, Theirs: theirs, Rel: look.rel})
					}
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

// formatItem is one conflicting ref as a refusal names it: the ref, its prefix
// pair, the relation that matched, and both spellings again in Go's
// ASCII-only quoted form (%+q, strconv.QuoteToASCII), because an NFC and an
// NFD spelling display identically. Plain %q would not do: it leaves
// combining marks raw (M29). The quoted form holds no whitespace, so it
// survives Push's oneLine.
func formatItem(c caseConflict) string {
	return fmt.Sprintf("%s (%s vs %s, by %s: %+q vs %+q)", c.Ref, c.Ours, c.Theirs, c.Rel, c.Ours, c.Theirs)
}

// caseConflictMessage is the refusal text for a create, naming every
// conflicting ref, in cs's (sorted) order, each with its own prefix pair and
// relation (formatItem). The harm it names follows the relations: a case
// match harms Windows and Mac clones, a canonical-only match Mac clones
// alone, so any relCase item names both.
func caseConflictMessage(dst string, cs []caseConflict) string {
	parts := make([]string, len(cs))
	harm := "a Mac clone would mishandle the pair"
	for i, c := range cs {
		parts[i] = formatItem(c)
		if c.Rel == relCase {
			harm = "a Windows or Mac clone would mishandle the pair"
		}
	}
	return fmt.Sprintf("%s conflicts with %s: %s (README, Honest limits), "+
		"so this create is refused on every platform", dst, strings.Join(parts, ", "), harm)
}

// failedDeleteClause is what phase 5 appends to caseConflictMessage when a
// create it refuses depended on same-batch deletes that did not commit (spec
// §5: the refusal names the delete it depended on). refs are sorted, so the
// text does not depend on batch order: " (the same-batch delete of A did not
// commit)" for one, " (the same-batch deletes of A, B and C did not commit)"
// for several, and "" for none.
func failedDeleteClause(refs []string) string {
	if len(refs) == 0 {
		return ""
	}
	s := append([]string(nil), refs...)
	sort.Strings(s)
	if len(s) == 1 {
		return " (the same-batch delete of " + s[0] + " did not commit)"
	}
	return " (the same-batch deletes of " + strings.Join(s[:len(s)-1], ", ") + " and " +
		s[len(s)-1] + " did not commit)"
}
