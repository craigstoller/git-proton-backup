package repo

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
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

// normLine is one data line of NormalizationTest.txt: the five columns c1..c5
// decoded to strings, with the Part it sits in and its line number.
type normLine struct {
	part, ln int
	c        [5]string
}

// hexSeq decodes a field of space-separated hexadecimal code points into the
// string they spell. A surrogate or an out-of-range value is an error: a Go
// string cannot hold one as a rune, and the file lists none.
func hexSeq(field string) (string, error) {
	var b strings.Builder
	for _, h := range strings.Fields(field) {
		v, err := strconv.ParseUint(h, 16, 32)
		if err != nil || v > unicode.MaxRune || (v >= 0xD800 && v <= 0xDFFF) {
			return "", fmt.Errorf("bad code point %q", h)
		}
		b.WriteRune(rune(v))
	}
	return b.String(), nil
}

// normalizationLines reads the committed NormalizationTest.txt of the pinned
// version (from the package directory, never the network) and returns its data
// lines, with the number of lines the file has. A comment line, a blank line
// and an "@PartN" heading are not data lines.
func normalizationLines(t *testing.T) (lines []normLine, total int) {
	t.Helper()
	path := filepath.Join("unicodedata", unicodeVersion, "NormalizationTest.txt")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	part := -1
	for ln := 1; sc.Scan(); ln++ {
		total = ln
		text := sc.Text()
		if strings.HasPrefix(text, "@Part") {
			n, err := strconv.Atoi(strings.TrimPrefix(strings.Fields(text)[0], "@Part"))
			if err != nil {
				t.Fatalf("%s:%d: bad part heading %q", path, ln, text)
			}
			part = n
			continue
		}
		if i := strings.IndexByte(text, '#'); i >= 0 {
			text = text[:i]
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		cols := strings.Split(text, ";")
		if part < 0 || len(cols) != 6 || strings.TrimSpace(cols[5]) != "" {
			t.Fatalf("%s:%d: want five ';'-terminated columns inside a Part, got %q", path, ln, sc.Text())
		}
		l := normLine{part: part, ln: ln}
		for i := range l.c {
			if l.c[i], err = hexSeq(cols[i]); err != nil {
				t.Fatalf("%s:%d: column c%d: %v", path, ln, i+1, err)
			}
		}
		lines = append(lines, l)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return lines, total
}

// maxReported caps the failures one table-driven test prints, so a broken
// nfd shows its first few lines instead of thousands.
const maxReported = 20

// RED (compile failure: nfd did not exist; with nfd stubbed as the identity
// the per-line assertions fire from Part 0's first data line, line 44; the
// unlisted-code-point check passes under that stub, so it is a GUARD against
// nfd changing a code point it must leave alone). The file is the
// conformance oracle for canonical decomposition. Every data line of every
// Part satisfies the NFD rows of its header: nfd(c1) == nfd(c2) == nfd(c3) ==
// c3 and nfd(c4) == nfd(c5) == c5. Its header (lines 36-40) also states, for
// every code point X assigned in this version and not specifically listed in
// Part 1, X == toNFD(X); Part 1 lists all 11,172 Hangul syllables, the only
// assigned code points outside Part 1 that nfd would change (PM8). A listing
// that missed a syllable would make that second check wrong, so it is checked
// first. A surrogate is skipped: string(rune) turns it into U+FFFD, a check of
// nothing.
func TestNFDMatchesNormalizationTest(t *testing.T) {
	lines, total := normalizationLines(t)
	if len(lines) < 20000 {
		t.Fatalf("read only %d data lines of NormalizationTest.txt (measured 20034 at 17.0.0)", len(lines))
	}

	bad := 0
	for _, l := range lines {
		for _, p := range [][2]int{{0, 2}, {1, 2}, {2, 2}, {3, 4}, {4, 4}} {
			if got := nfd(l.c[p[0]]); got != l.c[p[1]] {
				if bad++; bad <= maxReported {
					t.Errorf("line %d (Part %d): nfd(c%d = %+q) = %+q, want c%d = %+q",
						l.ln, l.part, p[0]+1, l.c[p[0]], got, p[1]+1, l.c[p[1]])
				}
			}
		}
	}
	if bad > maxReported {
		t.Errorf("... and %d more failures", bad-maxReported)
	}

	listed := map[rune]bool{}
	for _, l := range lines {
		if l.part != 1 {
			continue
		}
		rs := []rune(l.c[0])
		if len(rs) != 1 {
			t.Fatalf("line %d: Part 1 c1 = %+q, want one code point", l.ln, l.c[0])
		}
		listed[rs[0]] = true
	}
	for r := rune(0xAC00); r <= 0xD7A3; r++ {
		if !listed[r] {
			t.Fatalf("Part 1 does not list Hangul syllable %U, so the unlisted-code-point check would be wrong for it", r)
		}
	}

	checked := 0
	bad = 0 // the per-line failures above were reported on their own
	for r, ok := range assignedIn(t) {
		if !ok || listed[rune(r)] || (r >= 0xD800 && r <= 0xDFFF) {
			continue
		}
		checked++
		s := string(rune(r))
		if got := nfd(s); got != s {
			if bad++; bad <= maxReported {
				t.Errorf("assigned %U is not in Part 1, so nfd must leave it alone; nfd(%+q) = %+q", r, s, got)
			}
		}
	}
	if bad > maxReported {
		t.Errorf("... and %d more failures", bad-maxReported)
	}
	if checked < 100000 {
		t.Fatalf("the unlisted-code-point check covered only %d code points", checked)
	}
	t.Logf("NormalizationTest.txt: %d lines read, %d data lines (%d in Part 1, listing %d code points); "+
		"%d assigned code points outside Part 1 checked", total, len(lines), countPart(lines, 1), len(listed), checked)
}

func countPart(lines []normLine, part int) int {
	n := 0
	for _, l := range lines {
		if l.part == part {
			n++
		}
	}
	return n
}

// RED (compile failure: canonKey did not exist; with nfd stubbed as the
// identity it fires from Part 0's first data line, line 44). Canonical equivalents share one
// key: canonKey(c1) == canonKey(c2) == canonKey(c3) and canonKey(c4) ==
// canonKey(c5), on every line of every Part.
func TestCanonKeySharedByEquivalentStrings(t *testing.T) {
	lines, _ := normalizationLines(t)
	bad := 0
	for _, l := range lines {
		for _, p := range [][2]int{{0, 1}, {0, 2}, {3, 4}} {
			if a, b := canonKey(l.c[p[0]]), canonKey(l.c[p[1]]); a != b {
				if bad++; bad <= maxReported {
					t.Errorf("line %d (Part %d): canonKey(c%d = %+q) = %+q, canonKey(c%d = %+q) = %+q; want equal",
						l.ln, l.part, p[0]+1, l.c[p[0]], a, p[1]+1, l.c[p[1]], b)
				}
			}
		}
	}
	if bad > maxReported {
		t.Errorf("... and %d more failures", bad-maxReported)
	}
}

// GUARD (compile failure while canonKey did not exist; with nfd stubbed as the
// identity it passes, because the identity does not apply compatibility
// mappings either). It guards the other direction: canonKey is canonical, not
// compatibility, equivalence, so a refactor to an NFKD-style decomposition
// would collapse names the rule must keep apart. The ligature fi is only
// compatibility-equivalent to "fi"; the Ohm sign is canonically U+03A9, which
// folds to U+03C9. Over the file: where the compatibility form c5 differs from
// the canonical form c3 even after folding, c1 and c4 (which only a
// compatibility mapping joins) must not share a key.
func TestCanonKeyIgnoresCompatibilityMappings(t *testing.T) {
	if canonKey("\uFB01") == canonKey("fi") {
		t.Errorf("canonKey(fi ligature) = canonKey(\"fi\") = %+q: a compatibility mapping was applied", canonKey("fi"))
	}
	if canonKey("\u2126") != canonKey("\u03C9") {
		t.Errorf("canonKey(Ohm sign) = %+q, canonKey(small omega) = %+q; want equal", canonKey("\u2126"), canonKey("\u03C9"))
	}

	lines, _ := normalizationLines(t)
	exercised := 0
	for _, l := range lines {
		if l.c[2] == l.c[4] || caseKey(l.c[2]) == caseKey(l.c[4]) {
			continue
		}
		exercised++
		if canonKey(l.c[0]) == canonKey(l.c[3]) {
			t.Errorf("line %d (Part %d): canonKey(c1 = %+q) == canonKey(c4 = %+q) = %+q: only a compatibility mapping joins them",
				l.ln, l.part, l.c[0], l.c[3], canonKey(l.c[0]))
		}
	}
	if exercised == 0 {
		t.Fatalf("no line has c3 != c5 with caseKey(c3) != caseKey(c5): the file-wide check is vacuous")
	}
	t.Logf("compatibility form exercised on %d of %d data lines", exercised, len(lines))
}

// GUARD (compile failure while canonKey did not exist; with nfd stubbed as the
// identity it passes, because folding alone already joins every pair). Every
// C (common) and S (simple) line of CaseFolding.txt is a code point and its
// simple folding target, which both keys must join, whatever either one
// decomposes into. This is the check that decomposing first leaves the
// folding classes intact (spec section 3, the folding representative).
func TestCaseFoldingVariantsShareBothKeys(t *testing.T) {
	path := filepath.Join("unicodedata", unicodeVersion, "CaseFolding.txt")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	n, bad := 0, 0
	sc := bufio.NewScanner(f)
	for ln := 1; sc.Scan(); ln++ {
		text := sc.Text()
		if i := strings.IndexByte(text, '#'); i >= 0 {
			text = text[:i]
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		fields := strings.Split(text, ";")
		if len(fields) < 3 {
			t.Fatalf("%s:%d: want 'code; status; mapping;', got %q", path, ln, sc.Text())
		}
		if s := strings.TrimSpace(fields[1]); s != "C" && s != "S" {
			continue
		}
		from, err1 := hexSeq(fields[0])
		to, err2 := hexSeq(fields[2])
		if err1 != nil || err2 != nil || utf8.RuneCountInString(from) != 1 || utf8.RuneCountInString(to) != 1 {
			t.Fatalf("%s:%d: want one code point on each side, got %q", path, ln, sc.Text())
		}
		n++
		if a, b := caseKey(from), caseKey(to); a != b {
			if bad++; bad <= maxReported {
				t.Errorf("%s:%d: caseKey(%+q) = %+q, caseKey(%+q) = %+q; want equal", path, ln, from, a, to, b)
			}
		}
		if a, b := canonKey(from), canonKey(to); a != b {
			if bad++; bad <= maxReported {
				t.Errorf("%s:%d: canonKey(%+q) = %+q, canonKey(%+q) = %+q; want equal", path, ln, from, a, to, b)
			}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if bad > maxReported {
		t.Errorf("... and %d more failures", bad-maxReported)
	}
	if n < 1500 {
		t.Fatalf("only %d C and S lines in CaseFolding.txt (measured 1512 at 17.0.0)", n)
	}
	t.Logf("%d C and S lines of CaseFolding.txt checked", n)
}

// RED (compile failure: canonKey did not exist; with nfd stubbed as the
// identity both canonKey claims fire: a and b share a canonKey, b and c do
// not). Spec
// section 3's three strings show why neither relation contains the other:
//
//	a = a U+03B9 U+0301   b = a U+0345 U+0301   c = a U+0301 U+03B9
//
// a and b differ only within the iota fold class, so they share caseKey, but
// decomposition reorders b's marks to U+0301 U+0345 and their canonKeys
// differ. b and c are canonically equivalent, so they share canonKey, but
// their caseKeys differ. a and c share neither.
func TestCanonKeyIotaExamples(t *testing.T) {
	a := "a\u03B9\u0301"
	b := "a\u0345\u0301"
	c := "a\u0301\u03B9"
	if caseKey(a) != caseKey(b) {
		t.Errorf("caseKey(a) = %+q, caseKey(b) = %+q; want equal", caseKey(a), caseKey(b))
	}
	if canonKey(a) == canonKey(b) {
		t.Errorf("canonKey(a) = canonKey(b) = %+q; want different", canonKey(a))
	}
	if canonKey(c) != canonKey(b) {
		t.Errorf("canonKey(c) = %+q, canonKey(b) = %+q; want equal", canonKey(c), canonKey(b))
	}
	if caseKey(c) == caseKey(b) {
		t.Errorf("caseKey(c) = caseKey(b) = %+q; want different", caseKey(c))
	}
	if caseKey(a) == caseKey(c) {
		t.Errorf("caseKey(a) = caseKey(c) = %+q; want different", caseKey(a))
	}
	if canonKey(a) == canonKey(c) {
		t.Errorf("canonKey(a) = canonKey(c) = %+q; want different", canonKey(a))
	}
}

// RED (compile failure: nfd did not exist; with nfd stubbed as the identity
// the first syllable fires). Hangul syllables decompose by the standard
// algorithm, not the table: the first is a three-jamo syllable (L, V, T), the
// second has no trailing consonant (L, V), the third is the last syllable.
func TestNFDHangul(t *testing.T) {
	tests := []struct{ in, want string }{
		{"\uD55C", "\u1112\u1161\u11AB"},
		{"\uAC00", "\u1100\u1161"},
		{"\uD7A3", "\u1112\u1175\u11C2"},
	}
	for _, tt := range tests {
		if got := nfd(tt.in); got != tt.want {
			t.Errorf("nfd(%+q) = %+q, want %+q", tt.in, got, tt.want)
		}
	}
}

// RED (compile failure: nfd did not exist; with nfd stubbed as the identity
// the reordering claim "x U+0301 U+0327" fires). An invalid UTF-8 byte is
// copied through and counts as a starter, so no mark is reordered across it
// and distinct invalid bytes never compare equal (spec section 3, "nfd";
// Review Focus 1). The last loop is the no-panic claim: hostile inputs (a
// lone byte, a truncated sequence, an encoded surrogate, a code point above
// U+10FFFF, a leading mark) come back unchanged, since none has a reorderable
// pair.
func TestNFDTreatsInvalidBytesAsStarters(t *testing.T) {
	if got, want := nfd("x\u0301\u0327"), "x\u0327\u0301"; got != want {
		t.Errorf("nfd(%+q) = %+q, want %+q", "x\u0301\u0327", got, want)
	}
	if got, want := nfd("\u0301\u0327"), "\u0327\u0301"; got != want {
		t.Errorf("nfd(%+q) = %+q, want %+q (a run of marks at the start of the string)", "\u0301\u0327", got, want)
	}
	in := "x\u0301\xff\u0327"
	if got := nfd(in); got != in {
		t.Errorf("nfd(%+q) = %+q, want it unchanged: the invalid byte is a starter, so the marks stay on their sides of it", in, got)
	}
	if canonKey("a\xffb") == canonKey("a\xfeb") {
		t.Errorf("distinct invalid bytes must give distinct keys, both gave %+q", canonKey("a\xffb"))
	}
	for _, s := range []string{"", "\xff", "a\xc3", "\xe2\x82", "\xed\xa0\x80", "\xf4\x90\x80\x80", "\xff\u0301", "\u0301\xff"} {
		if got := nfd(s); got != s {
			t.Errorf("nfd(%+q) = %+q, want it unchanged", s, got)
		}
		_ = canonKey(s)
	}
}
