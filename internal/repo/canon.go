package repo

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// Unicode tables for the case key and the canonical key (spec
// 2026-10-27-v2-macos-port-design.md, section 3, "The tables" and M23).
//
// One pinned Unicode version supplies the canonical decompositions, the
// combining classes and the simple case folding, so every build refuses the
// same set of ref pairs whatever Go compiled it. The version is a deliberate
// choice, never a side effect of a Go upgrade: unicode_tables.go is generated
// by gen_unicode.go from the files committed under unicodedata/<version>/, with
// no network. Moving the pin is: run fetch_unicode.go (by hand, it is the only
// code that touches the network), change -version below, run go generate, and
// delete the old unicodedata directory.
//
//go:generate go run gen_unicode.go -version 17.0.0

// decomposeRune returns r's full recursive canonical decomposition, or "" when
// r has none. Every rune of the result has no decomposition of its own, so
// the caller needs no second pass. Hangul syllables return "": the standard
// algorithm handles them, not the table. Compatibility mappings (the fi
// ligature) are not canonical and return "".
func decomposeRune(r rune) string {
	i := sort.Search(len(canonDecompTable), func(i int) bool { return canonDecompTable[i].r >= r })
	if i < len(canonDecompTable) && canonDecompTable[i].r == r {
		return canonDecompTable[i].d
	}
	return ""
}

// combiningClass returns r's canonical combining class: 0 for a starter or any
// code point the table does not list.
func combiningClass(r rune) uint8 {
	i := sort.Search(len(combiningClassTable), func(i int) bool { return combiningClassTable[i].r >= r })
	if i < len(combiningClassTable) && combiningClassTable[i].r == r {
		return combiningClassTable[i].c
	}
	return 0
}

// foldRune returns r's simple case folding target, the C or S entry of
// CaseFolding.txt at the pinned version, or r itself when it has none. A code
// point with only an F (full) entry, such as sharp s, folds to itself.
func foldRune(r rune) rune {
	i := sort.Search(len(foldTable), func(i int) bool { return foldTable[i].r >= r })
	if i < len(foldTable) && foldTable[i].r == r {
		return foldTable[i].to
	}
	return r
}

// The Hangul syllable block and its jamo (Unicode 3.12, "Conjoining Jamo
// Behavior"). A syllable decomposes by arithmetic, not by table: decomposeRune
// leaves all 11,172 of them out.
const (
	hangulSBase  = 0xAC00
	hangulLBase  = 0x1100
	hangulVBase  = 0x1161
	hangulTBase  = 0x11A7
	hangulLCount = 19
	hangulVCount = 21
	hangulTCount = 28 // including "no trailing consonant"
	hangulNCount = hangulVCount * hangulTCount
	hangulSCount = hangulLCount * hangulNCount
)

// markedRune is a non-starter waiting in nfd's current run of marks.
type markedRune struct {
	r  rune
	cc uint8
}

// nfd returns s in Unicode Normalization Form D at the pinned version: every
// rune replaced by its full canonical decomposition (decomposeRune, with
// Hangul syllables decomposed by the standard algorithm), then canonically
// reordered: within each run of non-starters, a stable sort by combining
// class. A starter, class 0, ends a run and is never moved across, so the
// sort never has to look past one.
//
// Compatibility mappings are not applied (the fi ligature stays), which is
// what keeps canonKey a canonical relation. Invalid UTF-8 bytes are copied
// through unchanged, as caseKey copies them, and each counts as a starter, so
// no mark is reordered across one and distinct invalid bytes stay distinct.
func nfd(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	var marks []markedRune // the pending run of non-starters, in input order

	// flush writes the pending run in canonical order. An insertion sort is
	// stable, and a run is a base character's few marks, so it is the right
	// size of tool.
	flush := func() {
		for i := 1; i < len(marks); i++ {
			m := marks[i]
			j := i
			for j > 0 && marks[j-1].cc > m.cc {
				marks[j] = marks[j-1]
				j--
			}
			marks[j] = m
		}
		for _, m := range marks {
			b.WriteRune(m.r)
		}
		marks = marks[:0]
	}
	// emit takes one rune of the decomposed text: a mark joins the pending
	// run, a starter closes it and is written itself.
	emit := func(r rune) {
		if cc := combiningClass(r); cc != 0 {
			marks = append(marks, markedRune{r, cc})
			return
		}
		flush()
		b.WriteRune(r)
	}

	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			flush()
			b.WriteByte(s[i])
			i++
			continue
		}
		i += size

		if si := r - hangulSBase; si >= 0 && si < hangulSCount {
			emit(hangulLBase + si/hangulNCount)
			emit(hangulVBase + si%hangulNCount/hangulTCount)
			if t := si % hangulTCount; t != 0 {
				emit(hangulTBase + t)
			}
			continue
		}
		if d := decomposeRune(r); d != "" {
			for _, dr := range d {
				emit(dr)
			}
			continue
		}
		emit(r)
	}
	flush()
	return b.String()
}

// canonKey maps s to a key that two strings share exactly when they are
// canonically equivalent caselessly: canonical decomposition, then the same
// simple case folding caseKey uses (spec 2026-10-27, section 3, "The rule").
// There is no second decomposition after the fold, which Unicode's canonical
// caseless match would add: with folding to the CaseFolding.txt targets, the
// pass changes nothing on decomposed input (section 3, "The folding
// representative, and why canonKey needs no second decomposition").
//
// It is the second of the two relations the push refusal takes, beside
// caseKey. Neither contains the other: caseKey calls "a" U+03B9 U+0301 and
// "a" U+0345 U+0301 equal, but canonKey does not, because nfd reorders the
// second one's marks.
func canonKey(s string) string { return caseKey(nfd(s)) }
