package repo

import "sort"

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
