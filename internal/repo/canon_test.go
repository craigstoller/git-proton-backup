package repo

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The three pinned Unicode input files, with the byte counts and SHA-256s
// measured on 2026-10-27 from https://www.unicode.org/Public/17.0.0/ucd/
// (Unicode publishes no checksums; these are what the fetch saw, reviewed
// when the pin was set - Amendment MA2). The values are written out here, a
// second time, on purpose: the generated constants and the committed bytes
// must both agree with this table, so a regenerate cannot quietly move the pin.
var pinnedUnicodeInputs = []struct {
	file   string
	size   int
	sha256 string
	got    string // the generated constant, as the generator recorded it
}{
	{"UnicodeData.txt", 2198209, "2e1efc1dcb59c575eedf5ccae60f95229f706ee6d031835247d843c11d96470c", unicodeDataSHA256},
	{"CaseFolding.txt", 87539, "ff8d8fefbf123574205085d6714c36149eb946d717a0c585c27f0f4ef58c4183", caseFoldingSHA256},
	{"NormalizationTest.txt", 2827429, "5019ffd530751a741900c849c0e010332f142a3612234639bd200b82138a87db", normalizationTestSHA256},
}

// RED (compile failure: unicodeVersion, the three checksum constants and the
// committed input files did not exist). Each committed input file's SHA-256
// equals both the constant the generator recorded in unicode_tables.go and
// the value reviewed when the pin was set. The files are read from the
// repository, not the network.
func TestUnicodeInputsMatchTheRecordedChecksums(t *testing.T) {
	if unicodeVersion != "17.0.0" {
		t.Fatalf("unicodeVersion = %q, want the reviewed pin %q", unicodeVersion, "17.0.0")
	}
	for _, in := range pinnedUnicodeInputs {
		path := filepath.Join("unicodedata", unicodeVersion, in.file)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: %v", in.file, err)
			continue
		}
		if len(data) != in.size {
			t.Errorf("%s: %d bytes, want %d", in.file, len(data), in.size)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != in.sha256 {
			t.Errorf("%s: committed bytes hash to %s, want the reviewed %s", in.file, got, in.sha256)
		}
		if in.got != in.sha256 {
			t.Errorf("%s: generated constant is %s, want the reviewed %s", in.file, in.got, in.sha256)
		}
	}
}

// RED (compile failure: foldRune did not exist). foldRune is the C or S
// target of CaseFolding.txt, or the rune itself. The samples are an ASCII
// pair, the Kelvin sign (K, k and U+212A share a class), the two members of
// the iota class that fold to a starter rather than to the combining
// U+0345 the old smallest-member key chose (M23), and sharp s, which has an
// F entry only: simple folding leaves it alone.
func TestFoldRuneSamples(t *testing.T) {
	tests := []struct {
		in, want rune
	}{
		{'A', 'a'},
		{'a', 'a'},
		{'\u212A', 'k'},
		{'\u0345', '\u03B9'},
		{'\u1FBE', '\u03B9'},
		{'\u00DF', '\u00DF'},
		{'\u1E9E', '\u00DF'}, // an S entry: capital sharp s folds to small sharp s
		{'/', '/'},
	}
	for _, tt := range tests {
		if got := foldRune(tt.in); got != tt.want {
			t.Errorf("foldRune(%+q) = %+q, want %+q", tt.in, got, tt.want)
		}
	}
}

// RED (compile failure: decomposeRune did not exist). decomposeRune is the
// full recursive canonical decomposition, "" when there is none. The samples
// cover a plain two-rune mapping, a singleton (U+1FBE has the canonical
// decomposition U+03B9, M23), a recursive mapping (U+01D5 goes through
// U+00DC), a singleton that lands on a composed character and so recurses
// (the Angstrom sign), no decomposition at all, a compatibility-only mapping
// (the fi ligature), a Hangul syllable (the standard algorithm's job, not the
// table's), and '/'.
func TestDecomposeRuneSamples(t *testing.T) {
	tests := []struct {
		in   rune
		want string
	}{
		{'\u00E9', "e\u0301"},
		{'\u1FBE', "\u03B9"},
		{'\u01D5', "U\u0308\u0304"},
		{'\u212B', "A\u030A"},
		{'a', ""},
		{'\uFB01', ""},
		{'\uAC00', ""},
		{'/', ""},
	}
	for _, tt := range tests {
		if got := decomposeRune(tt.in); got != tt.want {
			t.Errorf("decomposeRune(%+q) = %+q, want %+q", tt.in, got, tt.want)
		}
	}
}

// RED (compile failure: combiningClass did not exist). The classes are the
// ones the spec's examples lean on: 230 above, 202 attached below, 240 for
// the iota subscript (the highest class in use, M23), 0 for a starter.
func TestCombiningClassSamples(t *testing.T) {
	tests := []struct {
		in   rune
		want uint8
	}{
		{'\u0301', 230},
		{'\u0327', 202},
		{'\u0345', 240},
		{'a', 0},
		{'\u0399', 0},
	}
	for _, tt := range tests {
		if got := combiningClass(tt.in); got != tt.want {
			t.Errorf("combiningClass(%+q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

// GUARD (passes once the tables exist; written to fail if the generator ever
// stopped sorting). The accessors binary-search the generated tables, and a
// table out of order would not fail loudly: lookups would simply miss. Each
// table must be strictly ascending.
func TestUnicodeTablesAreSorted(t *testing.T) {
	for i := 1; i < len(canonDecompTable); i++ {
		if canonDecompTable[i-1].r >= canonDecompTable[i].r {
			t.Fatalf("canonDecompTable not strictly ascending at index %d: %U then %U",
				i, canonDecompTable[i-1].r, canonDecompTable[i].r)
		}
	}
	for i := 1; i < len(combiningClassTable); i++ {
		if combiningClassTable[i-1].r >= combiningClassTable[i].r {
			t.Fatalf("combiningClassTable not strictly ascending at index %d: %U then %U",
				i, combiningClassTable[i-1].r, combiningClassTable[i].r)
		}
	}
	for i := 1; i < len(foldTable); i++ {
		if foldTable[i-1].r >= foldTable[i].r {
			t.Fatalf("foldTable not strictly ascending at index %d: %U then %U",
				i, foldTable[i-1].r, foldTable[i].r)
		}
	}
	if len(canonDecompTable) == 0 || len(combiningClassTable) == 0 || len(foldTable) == 0 {
		t.Fatalf("an empty table: %d decompositions, %d classes, %d folds",
			len(canonDecompTable), len(combiningClassTable), len(foldTable))
	}
}

// GUARD. decomposeRune returns the FULL decomposition, so nothing it returns
// may itself decompose (nfd would otherwise need a second pass), and no entry
// may produce or consume '/' (the generator asserts that too; the committed
// table is checked as well so a hand edit of the generated file cannot hide
// it). Folding must also keep the property canonKey rests on: a rune with no
// canonical decomposition never folds to one that has.
func TestDecomposeRuneIsFullyDecomposed(t *testing.T) {
	for _, e := range canonDecompTable {
		if e.r == '/' || strings.ContainsRune(e.d, '/') {
			t.Errorf("%U decomposes to %+q: a canonical decomposition touches '/'", e.r, e.d)
		}
		for _, r := range e.d {
			if decomposeRune(r) != "" {
				t.Errorf("%U decomposes to %+q, which still contains decomposable %U", e.r, e.d, r)
			}
		}
	}
	for _, e := range foldTable {
		if decomposeRune(e.r) == "" && decomposeRune(e.to) != "" {
			t.Errorf("%U has no canonical decomposition but folds to %U, which has one", e.r, e.to)
		}
	}
}
