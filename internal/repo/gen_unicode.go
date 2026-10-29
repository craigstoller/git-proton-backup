//go:build ignore

// gen_unicode generates unicode_tables.go, the canonical-decomposition,
// combining-class and simple-case-folding tables the case key rests on
// (spec 2026-10-27-v2-macos-port-design.md, section 3, "The tables" and M23).
// It is kept out of the build and run by the go:generate directive in canon.go:
//
//	gen_unicode.go -version <v> [-dir <path>] [-out <path>]
//
// It reads UnicodeData.txt, CaseFolding.txt and NormalizationTest.txt from -dir
// (default unicodedata/<v>/), with no network. UnicodeData.txt and
// CaseFolding.txt supply the tables; NormalizationTest.txt is read only for its
// SHA-256. The output records the version and all three checksums, lists every
// table in ascending code point order so the same inputs give the same bytes,
// and is passed through go/format.
//
// Before it writes anything it re-checks four properties of the pinned data
// that canonKey's design leans on, and exits non-zero naming each check that
// does not hold, leaving -out untouched:
//
//  1. no code point has both a C and an S entry in CaseFolding.txt (or two
//     entries of the same kind), so simple folding needs no precedence;
//  2. for every code point with no canonical decomposition, its fold target has
//     none either, so folding never brings back a decomposable character and
//     canonKey needs no second decomposition (M23);
//  3. every code point whose combining class differs from its fold target's has
//     the highest class any code point has, and its target has class 0, so
//     folding never calls for reordering (M23). The fold classes found are
//     printed; for 17.0.0 that is exactly one, the iota class;
//  4. no canonical decomposition produces or consumes U+002F '/'.
//
// Hangul syllables decompose by the standard algorithm, not by table, so the
// tables never list them. For check 2 they count as decomposable.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	unicodeDataFile = "UnicodeData.txt"
	caseFoldingFile = "CaseFolding.txt"
	normTestFile    = "NormalizationTest.txt"

	// maxFailuresShown caps how many failing code points one check lists.
	maxFailuresShown = 20
)

// ucd is what the generator takes from UnicodeData.txt.
type ucd struct {
	ccc    map[rune]uint8  // non-zero canonical combining classes
	maxCCC uint8           // highest class any code point has
	canon  map[rune][]rune // one-level canonical mappings (no <tag>)
}

// foldLine is one C or S line of CaseFolding.txt.
type foldLine struct {
	cp     rune
	status string // "C" or "S"
	to     rune
	line   int
}

type decompEntry struct {
	r rune
	d []rune
}

type classEntry struct {
	r rune
	c uint8
}

type foldEntry struct{ r, to rune }

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gen_unicode", flag.ContinueOnError)
	fs.SetOutput(stderr)
	version := fs.String("version", "", "Unicode version of the pinned inputs, such as 17.0.0 (required)")
	dir := fs.String("dir", "", "directory holding the three input files (default unicodedata/<version>)")
	out := fs.String("out", "unicode_tables.go", "file to write")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "gen_unicode: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	if !validVersion(*version) {
		fmt.Fprintf(stderr, "gen_unicode: -version must look like 17.0.0, got %q\n", *version)
		return 2
	}
	if *dir == "" {
		*dir = filepath.Join("unicodedata", *version)
	}
	if err := generate(*version, *dir, *out, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, "gen_unicode: %v\n", err)
		return 1
	}
	return 0
}

// validVersion accepts digits separated by single dots. The version is
// embedded in a directory name and in generated source, so nothing else may
// pass.
func validVersion(v string) bool {
	if v == "" || v[0] == '.' || v[len(v)-1] == '.' || strings.Contains(v, "..") {
		return false
	}
	for i := 0; i < len(v); i++ {
		if c := v[i]; c != '.' && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

func generate(version, dir, out string, stdout, stderr io.Writer) error {
	udata, err := os.ReadFile(filepath.Join(dir, unicodeDataFile))
	if err != nil {
		return err
	}
	cfold, err := os.ReadFile(filepath.Join(dir, caseFoldingFile))
	if err != nil {
		return err
	}
	ntest, err := os.ReadFile(filepath.Join(dir, normTestFile))
	if err != nil {
		return err
	}
	if err := requireFirstLine(caseFoldingFile, cfold, "# CaseFolding-"+version+".txt"); err != nil {
		return err
	}
	if err := requireFirstLine(normTestFile, ntest, "# NormalizationTest-"+version+".txt"); err != nil {
		return err
	}

	u, err := parseUnicodeData(udata)
	if err != nil {
		return fmt.Errorf("%s: %w", unicodeDataFile, err)
	}
	lines, err := parseCaseFolding(cfold)
	if err != nil {
		return fmt.Errorf("%s: %w", caseFoldingFile, err)
	}
	full, err := fullDecompositions(u.canon)
	if err != nil {
		return fmt.Errorf("%s: %w", unicodeDataFile, err)
	}

	fmt.Fprintf(stdout, "gen_unicode: Unicode %s from %s\n", version, dir)

	// Check 1 comes first: with a code point folding two ways there is no
	// single table to check the others against.
	folds, fails1, nC, nS := buildFolds(lines)
	if fail(stderr, 1, "no code point has both a C and an S entry", fails1) {
		return fmt.Errorf("check 1 failed")
	}
	fmt.Fprintf(stdout, "check 1 ok: no code point has both a C and an S entry (%d C entries, %d S entries)\n", nC, nS)

	var failed []string
	hasDecomp := func(r rune) bool { return len(full[r]) > 0 || isHangulSyllable(r) }

	fails2 := checkNoDecomposableFoldTarget(folds, hasDecomp)
	if fail(stderr, 2, "a code point with no canonical decomposition never folds to one that has", fails2) {
		failed = append(failed, "check 2")
	} else {
		fmt.Fprintf(stdout, "check 2 ok: no code point without a canonical decomposition folds to one with (%d foldings examined)\n", len(folds))
	}

	fails3, mixed := checkCombiningClasses(folds, u)
	if fail(stderr, 3, "a fold that changes the combining class goes from the highest class to class 0", fails3) {
		failed = append(failed, "check 3")
	} else {
		fmt.Fprintf(stdout, "check 3 ok: highest combining class in use is %d; fold classes mixing combining classes: %d\n", u.maxCCC, len(mixed))
		for _, m := range mixed {
			fmt.Fprintln(stdout, "  "+m)
		}
	}

	fails4 := checkSlash(u.canon, full)
	if fail(stderr, 4, "no canonical decomposition produces or consumes U+002F", fails4) {
		failed = append(failed, "check 4")
	} else {
		fmt.Fprintf(stdout, "check 4 ok: no canonical decomposition produces or consumes U+002F (%d mappings examined)\n", len(u.canon))
	}
	if len(failed) > 0 {
		return fmt.Errorf("%s failed", strings.Join(failed, ", "))
	}

	decomps := make([]decompEntry, 0, len(full))
	for r, d := range full {
		decomps = append(decomps, decompEntry{r, d})
	}
	sort.Slice(decomps, func(i, j int) bool { return decomps[i].r < decomps[j].r })
	classes := make([]classEntry, 0, len(u.ccc))
	for r, c := range u.ccc {
		classes = append(classes, classEntry{r, c})
	}
	sort.Slice(classes, func(i, j int) bool { return classes[i].r < classes[j].r })

	src, err := render(version, [3][]byte{udata, cfold, ntest}, decomps, classes, folds)
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, src, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wrote %s: %d decompositions, %d combining classes, %d simple foldings, %d bytes\n",
		out, len(decomps), len(classes), len(folds), len(src))
	return nil
}

// fail prints one check's failures to stderr and reports whether there were
// any. The message names the check by number, which is what a caller greps.
func fail(stderr io.Writer, n int, what string, failures []string) bool {
	if len(failures) == 0 {
		return false
	}
	fmt.Fprintf(stderr, "check %d FAILED (%s): %d failing\n", n, what, len(failures))
	for i, f := range failures {
		if i == maxFailuresShown {
			fmt.Fprintf(stderr, "  ... and %d more\n", len(failures)-maxFailuresShown)
			break
		}
		fmt.Fprintln(stderr, "  "+f)
	}
	return true
}

// requireFirstLine fails when data does not open with the version line the
// file carries in its own header, so a -version that disagrees with the
// directory's contents is caught.
func requireFirstLine(name string, data []byte, want string) error {
	first := data
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		first = data[:i]
	}
	if got := strings.TrimRight(string(first), "\r"); got != want {
		return fmt.Errorf("%s opens with %q, want %q", name, got, want)
	}
	return nil
}

func isHangulSyllable(r rune) bool { return r >= 0xAC00 && r <= 0xD7A3 }

func parseCP(s string) (rune, error) {
	s = strings.TrimSpace(s)
	n, err := strconv.ParseUint(s, 16, 32)
	if err != nil || n > 0x10FFFF {
		return 0, fmt.Errorf("bad code point %q", s)
	}
	return rune(n), nil
}

func parseCPs(s string) ([]rune, error) {
	var out []rune
	for _, f := range strings.Fields(s) {
		r, err := parseCP(f)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty code point sequence")
	}
	return out, nil
}

// parseUnicodeData reads fields 0 (code point), 1 (name), 3 (combining class)
// and 5 (decomposition). A decomposition that opens with a <tag> is a
// compatibility mapping and is skipped. A First/Last range line is skipped
// too, but only if it carries no class and no mapping, which no range in the
// data does; one that did would be silently lost, so it is an error instead.
func parseUnicodeData(data []byte) (*ucd, error) {
	u := &ucd{ccc: map[rune]uint8{}, canon: map[rune][]rune{}}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	prev := rune(-1)
	inRange := false
	rangeName := ""
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Split(line, ";")
		if len(f) < 6 {
			return nil, fmt.Errorf("line %d: %d fields, want at least 6", lineNo, len(f))
		}
		cp, err := parseCP(f[0])
		if err != nil {
			return nil, fmt.Errorf("line %d: %v", lineNo, err)
		}
		if cp <= prev {
			return nil, fmt.Errorf("line %d: U+%04X does not ascend past U+%04X", lineNo, cp, prev)
		}
		prev = cp
		c, err := strconv.ParseUint(f[3], 10, 8)
		if err != nil {
			return nil, fmt.Errorf("line %d: bad combining class %q", lineNo, f[3])
		}
		if uint8(c) > u.maxCCC {
			u.maxCCC = uint8(c)
		}
		name, decomp := f[1], f[5]

		isFirst := strings.HasSuffix(name, ", First>")
		isLast := strings.HasSuffix(name, ", Last>")
		if isFirst || isLast {
			if c != 0 || decomp != "" {
				return nil, fmt.Errorf("line %d: range %q has a combining class or a decomposition, which this generator does not expand", lineNo, name)
			}
			if isFirst {
				if inRange {
					return nil, fmt.Errorf("line %d: range %q opens inside another range", lineNo, name)
				}
				inRange, rangeName = true, strings.TrimSuffix(name, ", First>")
			} else {
				if !inRange || strings.TrimSuffix(name, ", Last>") != rangeName {
					return nil, fmt.Errorf("line %d: range end %q does not match an open range", lineNo, name)
				}
				inRange = false
			}
			continue
		}
		if inRange {
			return nil, fmt.Errorf("line %d: %q appears inside the open range %q", lineNo, name, rangeName)
		}
		if !utf8.ValidRune(cp) {
			return nil, fmt.Errorf("line %d: U+%04X is not a scalar value", lineNo, cp)
		}
		if c != 0 {
			u.ccc[cp] = uint8(c)
		}
		if decomp == "" || strings.HasPrefix(decomp, "<") {
			continue
		}
		if isHangulSyllable(cp) {
			return nil, fmt.Errorf("line %d: Hangul syllable U+%04X has a table mapping, but the tables exclude Hangul", lineNo, cp)
		}
		elems, err := parseCPs(decomp)
		if err != nil {
			return nil, fmt.Errorf("line %d: decomposition of U+%04X: %v", lineNo, cp, err)
		}
		for _, e := range elems {
			if !utf8.ValidRune(e) {
				return nil, fmt.Errorf("line %d: decomposition of U+%04X contains U+%04X, not a scalar value", lineNo, cp, e)
			}
		}
		u.canon[cp] = elems
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if inRange {
		return nil, fmt.Errorf("range %q is never closed", rangeName)
	}
	if len(u.canon) == 0 || len(u.ccc) == 0 {
		return nil, fmt.Errorf("no canonical mappings or combining classes found; is this UnicodeData.txt?")
	}
	return u, nil
}

// parseCaseFolding returns the C and S lines. F (full) and T (Turkic) lines
// are validated and skipped: the spec's simple folding is statuses C and S
// only. An unknown status is an error, so a future status letter cannot be
// ignored by accident.
func parseCaseFolding(data []byte) ([]foldLine, error) {
	var out []foldLine
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		f := strings.Split(line, ";")
		if len(f) < 3 {
			return nil, fmt.Errorf("line %d: %d fields, want code; status; mapping", lineNo, len(f))
		}
		cp, err := parseCP(f[0])
		if err != nil {
			return nil, fmt.Errorf("line %d: %v", lineNo, err)
		}
		status := strings.TrimSpace(f[1])
		to, err := parseCPs(f[2])
		if err != nil {
			return nil, fmt.Errorf("line %d: %v", lineNo, err)
		}
		for _, r := range append([]rune{cp}, to...) {
			if !utf8.ValidRune(r) {
				return nil, fmt.Errorf("line %d: U+%04X is not a scalar value", lineNo, r)
			}
		}
		switch status {
		case "C", "S":
			if len(to) != 1 {
				return nil, fmt.Errorf("line %d: %s entry for U+%04X maps to %d code points, want 1", lineNo, status, cp, len(to))
			}
			out = append(out, foldLine{cp: cp, status: status, to: to[0], line: lineNo})
		case "F", "T":
		default:
			return nil, fmt.Errorf("line %d: unknown status %q", lineNo, status)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no C or S entries found; is this CaseFolding.txt?")
	}
	return out, nil
}

// fullDecompositions expands each one-level canonical mapping recursively, so
// every element of a result has no mapping of its own.
func fullDecompositions(canon map[rune][]rune) (map[rune][]rune, error) {
	full := make(map[rune][]rune, len(canon))
	var expand func(r rune, depth int) ([]rune, error)
	expand = func(r rune, depth int) ([]rune, error) {
		if d, ok := full[r]; ok {
			return d, nil
		}
		m, ok := canon[r]
		if !ok {
			return []rune{r}, nil
		}
		if depth > 32 {
			return nil, fmt.Errorf("decomposition of U+%04X nests deeper than 32 levels; a cycle?", r)
		}
		var out []rune
		for _, e := range m {
			sub, err := expand(e, depth+1)
			if err != nil {
				return nil, err
			}
			out = append(out, sub...)
		}
		full[r] = out
		return out, nil
	}
	keys := make([]rune, 0, len(canon))
	for r := range canon {
		keys = append(keys, r)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, r := range keys {
		if _, err := expand(r, 0); err != nil {
			return nil, err
		}
	}
	return full, nil
}

// buildFolds is check 1 and the folding table in one pass: it lists every
// code point that has more than one C or S line (a C and an S, or two of one
// kind) and, when there are none, returns the foldings sorted by code point.
func buildFolds(lines []foldLine) (folds []foldEntry, failures []string, nC, nS int) {
	first := make(map[rune]foldLine, len(lines))
	for _, l := range lines {
		if l.status == "C" {
			nC++
		} else {
			nS++
		}
		if p, dup := first[l.cp]; dup {
			what := "has two " + l.status + " entries"
			if p.status != l.status {
				what = "has both a " + p.status + " and an " + l.status + " entry"
			}
			failures = append(failures, fmt.Sprintf("U+%04X %s (line %d: %s to U+%04X; line %d: %s to U+%04X)",
				l.cp, what, p.line, p.status, p.to, l.line, l.status, l.to))
			continue
		}
		first[l.cp] = l
	}
	if len(failures) > 0 {
		return nil, failures, nC, nS
	}
	folds = make([]foldEntry, 0, len(first))
	for _, l := range first {
		folds = append(folds, foldEntry{l.cp, l.to})
	}
	sort.Slice(folds, func(i, j int) bool { return folds[i].r < folds[j].r })
	return folds, nil, nC, nS
}

// checkNoDecomposableFoldTarget is check 2.
func checkNoDecomposableFoldTarget(folds []foldEntry, hasDecomp func(rune) bool) []string {
	var failures []string
	for _, f := range folds {
		if !hasDecomp(f.r) && hasDecomp(f.to) {
			failures = append(failures, fmt.Sprintf("U+%04X has no canonical decomposition but folds to U+%04X, which has one", f.r, f.to))
		}
	}
	return failures
}

// checkCombiningClasses is check 3. It also returns one printable line per
// fold class (the code points sharing a fold target, with the target) whose
// members differ in combining class, sorted by target.
func checkCombiningClasses(folds []foldEntry, u *ucd) (failures, mixed []string) {
	members := map[rune][]rune{}
	for _, f := range folds {
		if cr, ct := u.ccc[f.r], u.ccc[f.to]; cr != ct && (cr != u.maxCCC || ct != 0) {
			failures = append(failures, fmt.Sprintf("U+%04X (class %d) folds to U+%04X (class %d); a changed class must go from the highest class in use, %d, to 0",
				f.r, cr, f.to, ct, u.maxCCC))
		}
		members[f.to] = append(members[f.to], f.r)
	}
	targets := make([]rune, 0, len(members))
	for t := range members {
		targets = append(targets, t)
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i] < targets[j] })
	for _, t := range targets {
		class := append([]rune{t}, members[t]...)
		sort.Slice(class, func(i, j int) bool { return class[i] < class[j] })
		differs := false
		parts := make([]string, len(class))
		for i, r := range class {
			parts[i] = fmt.Sprintf("U+%04X class %d", r, u.ccc[r])
			if u.ccc[r] != u.ccc[t] {
				differs = true
			}
		}
		if differs {
			mixed = append(mixed, fmt.Sprintf("{%s} folds to U+%04X", strings.Join(parts, ", "), t))
		}
	}
	return failures, mixed
}

// checkSlash is check 4, over the mappings as written and as fully expanded.
func checkSlash(canon, full map[rune][]rune) []string {
	var failures []string
	seen := map[string]bool{}
	add := func(msg string) {
		if !seen[msg] {
			seen[msg] = true
			failures = append(failures, msg)
		}
	}
	for _, m := range []map[rune][]rune{canon, full} {
		keys := make([]rune, 0, len(m))
		for r := range m {
			keys = append(keys, r)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		for _, r := range keys {
			if r == '/' {
				add("U+002F itself has a canonical decomposition")
			}
			for _, e := range m[r] {
				if e == '/' {
					add(fmt.Sprintf("the canonical decomposition of U+%04X contains U+002F", r))
				}
			}
		}
	}
	return failures
}

func sumHex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// render builds the generated file. Strings go through QuoteToASCII, so the
// output is ASCII whatever the decompositions contain.
func render(version string, inputs [3][]byte, decomps []decompEntry, classes []classEntry, folds []foldEntry) ([]byte, error) {
	var b bytes.Buffer
	w := func(layout string, a ...interface{}) { fmt.Fprintf(&b, layout, a...) }

	w("// Code generated by gen_unicode.go; DO NOT EDIT.\n\n")
	w("package repo\n\n")
	w("// unicodeVersion is the Unicode version the tables below were generated from.\n")
	w("// It is moved only by a reviewed change to the pin (fetch_unicode.go, then go generate).\n")
	w("const unicodeVersion = %q\n\n", version)
	w("// SHA-256 of the input files under unicodedata/<unicodeVersion>/, as the generator read them.\n")
	w("const (\n")
	w("unicodeDataSHA256 = %q\n", sumHex(inputs[0]))
	w("caseFoldingSHA256 = %q\n", sumHex(inputs[1]))
	w("normalizationTestSHA256 = %q\n", sumHex(inputs[2]))
	w(")\n\n")

	w("// canonDecompEntry maps a code point to its full recursive canonical decomposition.\n")
	w("type canonDecompEntry struct {\nr rune\nd string\n}\n\n")
	w("// canonDecompTable lists the %d code points that have a canonical decomposition, ascending.\n", len(decomps))
	w("// Hangul syllables are absent: they decompose by the standard algorithm.\n")
	w("var canonDecompTable = []canonDecompEntry{\n")
	for _, e := range decomps {
		w("{0x%04X, %s},\n", e.r, strconv.QuoteToASCII(string(e.d)))
	}
	w("}\n\n")

	w("// combiningClassEntry maps a code point to its canonical combining class.\n")
	w("type combiningClassEntry struct {\nr rune\nc uint8\n}\n\n")
	w("// combiningClassTable lists the %d code points with a non-zero combining class, ascending.\n", len(classes))
	w("var combiningClassTable = []combiningClassEntry{\n")
	for _, e := range classes {
		w("{0x%04X, %d},\n", e.r, e.c)
	}
	w("}\n\n")

	w("// foldEntry maps a code point to its simple case folding target.\n")
	w("type foldEntry struct {\nr, to rune\n}\n\n")
	w("// foldTable lists the %d code points with a C or S entry in CaseFolding.txt, ascending.\n", len(folds))
	w("var foldTable = []foldEntry{\n")
	for _, e := range folds {
		w("{0x%04X, 0x%04X},\n", e.r, e.to)
	}
	w("}\n")

	src, err := format.Source(b.Bytes())
	if err != nil {
		return nil, fmt.Errorf("formatting the generated source: %w", err)
	}
	return src, nil
}
