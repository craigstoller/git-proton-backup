package repo

import (
	"bufio"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// GUARD (it passes before and after the move of caseKey onto the pinned
// table, which changes the key's value for some classes but no answer to "do
// these two share a key"). Two strings share a key exactly when Unicode simple
// case folding at the pinned version calls them equal. The expectations are
// written out; there is no strings.EqualFold oracle. The pairs cover ASCII, a
// non-ASCII letter, the Kelvin sign (a three-member class, K, k and U+212A), a
// length mismatch, and sharp s, which SIMPLE folding does not equate with "SS"
// (that is full folding, which the spec rules out). Only whether two keys are
// equal is asserted, not the key's value, which this change moves (see
// TestCaseKeyIotaClassFoldsToTheStarter).
func TestCaseKeyFoldsByThePinnedTable(t *testing.T) {
	pairs := []struct {
		a, b string
		want bool
	}{
		{"main", "Main", true},
		{"Über", "über", true},
		{"K", "\u212A", true}, // LATIN CAPITAL K vs KELVIN SIGN
		{"main", "mainx", false},
		{"ß", "SS", false},
	}
	for _, p := range pairs {
		if got := caseKey(p.a) == caseKey(p.b); got != p.want {
			t.Errorf("caseKey(%+q) == caseKey(%+q) is %v, want %v; keys %+q, %+q",
				p.a, p.b, got, p.want, caseKey(p.a), caseKey(p.b))
		}
	}
}

// RED (every assertion fails against the old smallest-member key: it chose
// the combining mark U+0345 for this class, so all four gave "\u0345"). The
// iota class, U+0345 (combining), U+0399, U+03B9 and U+1FBE, now keys to the
// starter U+03B9, the folding target CaseFolding.txt itself names (M23).
func TestCaseKeyIotaClassFoldsToTheStarter(t *testing.T) {
	for _, r := range []rune{'\u0345', '\u0399', '\u03B9', '\u1FBE'} {
		if got := caseKey(string(r)); got != "\u03B9" {
			t.Errorf("caseKey(%+q) = %+q, want %+q", string(r), got, "\u03B9")
		}
	}
	// In context, beside an unfolded neighbour, so the starter is what lands
	// in the key rather than a lone-rune special case.
	if got, want := caseKey("a\u0345\u00B4"), "a\u03B9\u00B4"; got != want {
		t.Errorf("caseKey(a, U+0345, U+00B4) = %+q, want %+q", got, want)
	}
}

// GUARD (it passes before and after: only the valid runes fold). A fold that
// decoded every invalid byte to U+FFFD, as strings.EqualFold does, would call
// "a\xffb" and "a\xfeb" equal. caseKey copies invalid bytes through unfolded
// instead, so two distinct invalid bytes never collide.
func TestCaseKeyKeepsInvalidBytesDistinct(t *testing.T) {
	if caseKey("a\xffb") == caseKey("a\xfeb") {
		t.Errorf("distinct invalid bytes must give distinct keys, both gave %q", caseKey("a\xffb"))
	}
}

// assignedIn returns, indexed by code point, the set of code points the
// committed UnicodeData.txt of the pinned version assigns: every line's code
// point, with each "<..., First>" / "<..., Last>" pair expanded to the whole
// range between them. The file is read from the repository (the package
// directory), never the network.
func assignedIn(t *testing.T) []bool {
	t.Helper()
	path := filepath.Join("unicodedata", unicodeVersion, "UnicodeData.txt")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	assigned := make([]bool, unicode.MaxRune+1)
	sc := bufio.NewScanner(f)
	first := rune(-1) // the open range's First, or -1
	for ln := 1; sc.Scan(); ln++ {
		fields := strings.SplitN(sc.Text(), ";", 3)
		if len(fields) < 2 {
			t.Fatalf("%s:%d: want at least two fields, got %q", path, ln, sc.Text())
		}
		cp, err := strconv.ParseUint(fields[0], 16, 32)
		if err != nil || cp > unicode.MaxRune {
			t.Fatalf("%s:%d: bad code point %q", path, ln, fields[0])
		}
		r, name := rune(cp), fields[1]
		switch {
		case strings.HasSuffix(name, ", First>"):
			if first >= 0 {
				t.Fatalf("%s:%d: range First inside the range opened at U+%04X", path, ln, first)
			}
			first = r
		case strings.HasSuffix(name, ", Last>"):
			if first < 0 || first > r {
				t.Fatalf("%s:%d: range Last U+%04X without a First before it", path, ln, r)
			}
			for c := first; c <= r; c++ {
				assigned[c] = true
			}
			first = -1
		default:
			if first >= 0 {
				t.Fatalf("%s:%d: U+%04X inside the range opened at U+%04X", path, ln, r, first)
			}
			assigned[r] = true
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if first >= 0 {
		t.Fatalf("%s: range opened at U+%04X never closed", path, first)
	}

	// A parse that silently produced an empty or ragged set would make the
	// tests that use it vacuous, so check it against code points known either
	// way: a line, the middle of an expanded range (no line of its own), and
	// a hole.
	n := 0
	for _, a := range assigned {
		if a {
			n++
		}
	}
	if n < 150000 {
		t.Fatalf("%s assigns only %d code points", path, n)
	}
	for _, r := range []rune{'A', 0x03B9, 0x4E00, 0x5000, 0xAC01, 0xF0001} {
		if !assigned[r] {
			t.Fatalf("%s: U+%04X must be assigned (parse or range expansion is wrong)", path, r)
		}
	}
	for _, r := range []rune{0x0378, 0xFFFE, 0x10FFFF} {
		if assigned[r] {
			t.Fatalf("%s: U+%04X must not be assigned", path, r)
		}
	}
	return assigned
}

// toolchainAssigns reports whether the running toolchain's Unicode tables
// assign r. The unicode package has no unassigned (Cn) table, so a code point
// is assigned when some general category lists it.
func toolchainAssigns(r rune) bool {
	return unicode.In(r, unicode.L, unicode.M, unicode.N, unicode.P, unicode.S, unicode.Z, unicode.C)
}

// orbitID names r's unicode.SimpleFold orbit by its smallest member.
// SimpleFold walks an orbit in increasing order and wraps, so the loop visits
// every member once.
func orbitID(r rune) rune {
	m := r
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		if f < m {
			m = f
		}
	}
	return m
}

// GUARD (spec section 3, "No refusal is lost"). foldRune already exists from
// the table task, so this passes as soon as it is written; it is not RED, and
// it earns its place by failing if the generated classes ever split what a
// toolchain orbit joins, or (on an equal-version toolchain) merge what it
// keeps apart. Every pair a release built on Go's unicode tables refuses
// today must stay refused on the pinned table. For every code point r assigned in the pinned version,
// each member of its unicode.SimpleFold orbit that is also assigned there has
// the same foldRune: the toolchain's orbit lies inside one generated class.
// When the toolchain's Unicode version equals the pin the converse holds as
// well, so the classes are equal.
//
// A failure is investigated as a finding, never silenced and never cleared by
// moving the pin (spec section 3): on a toolchain newer than the pin it would
// mean a later Unicode version merged two fold classes, which disproves the
// stability inference the spec rests on.
func TestSimpleFoldOrbitsLieInsideGeneratedClasses(t *testing.T) {
	assigned := assignedIn(t)

	// 1. Containment: the orbit of every assigned r lies inside one class.
	checked := 0
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if !assigned[r] {
			continue
		}
		want := foldRune(r)
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			if !assigned[f] {
				continue // a character the pinned version does not assign
			}
			checked++
			if got := foldRune(f); got != want {
				t.Errorf("U+%04X and U+%04X share a unicode.SimpleFold orbit (toolchain Unicode %s) "+
					"but fold to U+%04X and U+%04X in the pinned table (%s): a pair the toolchain refuses would be accepted",
					r, f, unicode.Version, want, got, unicodeVersion)
			}
		}
	}

	// 2. Where the toolchain is at the pin, the converse: one generated class
	// is one orbit. Group the assigned code points by foldRune and require a
	// single orbit per group.
	equalBranch := unicode.Version == unicodeVersion
	if equalBranch {
		orbitOfClass := map[rune]rune{} // fold target -> the orbit its first member is in
		for r := rune(0); r <= unicode.MaxRune; r++ {
			if !assigned[r] {
				continue
			}
			cls, orb := foldRune(r), orbitID(r)
			if prev, ok := orbitOfClass[cls]; !ok {
				orbitOfClass[cls] = orb
			} else if prev != orb {
				t.Errorf("U+%04X folds to U+%04X in the pinned table but is in the unicode.SimpleFold orbit of U+%04X, "+
					"not U+%04X like the class's other members (toolchain Unicode %s equals the pin)",
					r, cls, orb, prev, unicode.Version)
			}
		}
	}

	// 3. Information, not an assertion (apart from the equal branch): how
	// many pinned classes span more than one toolchain orbit among the
	// characters both versions assign. Each is a class the pin merged, so a
	// pair the toolchain-built release accepted would now be refused (O1(iv)
	// accepted that).
	orbitsOfClass := map[rune]map[rune]bool{}
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if !assigned[r] || !toolchainAssigns(r) {
			continue
		}
		cls := foldRune(r)
		if orbitsOfClass[cls] == nil {
			orbitsOfClass[cls] = map[rune]bool{}
		}
		orbitsOfClass[cls][orbitID(r)] = true
	}
	merged := 0
	for _, orbits := range orbitsOfClass {
		if len(orbits) > 1 {
			merged++
		}
	}

	branch := "containment only (toolchain Unicode differs from the pin)"
	if equalBranch {
		branch = "containment and equal classes (toolchain Unicode equals the pin)"
	}
	t.Logf("toolchain Unicode %s, pin %s: ran %s; %d orbit members checked; "+
		"%d pinned classes span more than one toolchain orbit among the characters both versions assign",
		unicode.Version, unicodeVersion, branch, checked, merged)
	if equalBranch && merged != 0 {
		t.Errorf("%d pinned classes span more than one toolchain orbit, but the two Unicode versions are equal", merged)
	}
}

// GUARD (controller ruling T3-1). refPrefixes and caseConflicts rely on '/'
// having no case variant, so two prefixes can fold equal only at the same
// depth (refPrefixes' doc comment). caseKey now folds by the pinned table, so
// the table itself must keep '/' alone: it folds to itself, and no other
// assigned code point folds to it.
func TestNothingFoldsToOrFromSlash(t *testing.T) {
	if got := foldRune('/'); got != '/' {
		t.Errorf("foldRune('/') = %+q, want '/'", got)
	}
	assigned := assignedIn(t)
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if r != '/' && assigned[r] && foldRune(r) == '/' {
			t.Errorf("foldRune(U+%04X) = '/': '/' must have no case variant", r)
		}
	}
}

// RED (compile failure).
func TestRefPrefixes(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"refs/heads/grp/a", []string{"heads", "heads/grp", "heads/grp/a"}},
		{"refs/tags/v1", []string{"tags", "tags/v1"}},
	}
	for _, c := range cases {
		if got := refPrefixes(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("refPrefixes(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := refPrefixes("HEAD"); got != nil {
		t.Errorf("refPrefixes(%q) = %q, want nil", "HEAD", got)
	}
}

// RED (compile failure). The three shapes spec F13 found a Windows clone
// mishandling: two leaves, a leaf against a directory prefix, and two
// directory prefixes. Each names its Ref and the (Ours, Theirs) pair.
func TestCaseConflictsShapes(t *testing.T) {
	cases := []struct {
		name  string
		cand  string
		other string
		want  caseConflict
	}{
		{"two leaves", "refs/heads/main", "refs/heads/Main",
			caseConflict{Ref: "refs/heads/Main", Ours: "heads/main", Theirs: "heads/Main"}},
		{"leaf against directory", "refs/heads/feat", "refs/heads/Feat/x",
			caseConflict{Ref: "refs/heads/Feat/x", Ours: "heads/feat", Theirs: "heads/Feat"}},
		{"two directories", "refs/heads/grp/a", "refs/heads/Grp/b",
			caseConflict{Ref: "refs/heads/Grp/b", Ours: "heads/grp", Theirs: "heads/Grp"}},
	}
	for _, c := range cases {
		set := map[string]bool{c.cand: true, c.other: true}
		got := caseConflicts(set, []string{c.cand})
		want := map[string][]caseConflict{c.cand: {c.want}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: caseConflicts = %+v, want %+v", c.name, got, want)
		}
	}
}

// RED (compile failure). One create against two refs, each conflicting at
// its own prefix: the message names both, sorted, each with its own pair.
func TestCaseConflictMessageNamesEachRefWithItsOwnPair(t *testing.T) {
	const dst = "refs/heads/Grp/a"
	set := map[string]bool{dst: true, "refs/heads/grp/b": true, "refs/heads/GRP": true}
	got := caseConflictMessage(dst, caseConflicts(set, []string{dst})[dst])
	want := "refs/heads/Grp/a conflicts by letter case with " +
		"refs/heads/GRP (heads/Grp vs heads/GRP), refs/heads/grp/b (heads/Grp vs heads/grp): " +
		"a Windows clone would mishandle the pair (README, Honest limits), " +
		"so this create is refused on every platform"
	if got != want {
		t.Errorf("message:\n got %q\nwant %q", got, want)
	}
}

// RED (compile failure only: it is a negative assertion, so a zero-value
// stub passes it). refs/tags/main and refs/heads/main share a leaf spelling
// in different namespaces; their prefixes never fold equal, so they coexist.
func TestCaseConflictsCrossNamespaceIsNotAConflict(t *testing.T) {
	set := map[string]bool{"refs/tags/main": true, "refs/heads/main": true}
	if got := caseConflicts(set, []string{"refs/tags/main", "refs/heads/main"}); len(got) != 0 {
		t.Errorf("a tag and a branch of one name must not conflict, got %+v", got)
	}
}

// RED (compile failure). Review Focus 5: the result must not depend on Go's
// map iteration order or on the order the set and candidates were built in.
// It must also be non-empty, so a function that returns nothing cannot pass
// as "deterministic".
func TestCaseConflictsDeterministic(t *testing.T) {
	refs := []string{
		"refs/heads/main", "refs/heads/Main", "refs/heads/MAIN",
		"refs/heads/grp/a", "refs/heads/Grp/b", "refs/heads/GRP",
		"refs/heads/feat", "refs/heads/Feat/x",
		"refs/tags/v1", "refs/tags/V1", "refs/heads/unrelated",
	}
	var first map[string][]caseConflict
	for i := 0; i < 50; i++ {
		order := append([]string(nil), refs...)
		rng := rand.New(rand.NewSource(int64(i)))
		rng.Shuffle(len(order), func(a, b int) { order[a], order[b] = order[b], order[a] })
		set := make(map[string]bool, len(order))
		for _, r := range order {
			set[r] = true
		}
		got := caseConflicts(set, order)
		if i == 0 {
			first = got
			if len(first["refs/heads/main"]) != 2 || len(first["refs/heads/GRP"]) != 2 {
				t.Fatalf("expected conflicts missing from the first result: %+v", first)
			}
			if _, ok := first["refs/heads/unrelated"]; ok {
				t.Fatalf("an unrelated ref must not be reported: %+v", first)
			}
			continue
		}
		if !reflect.DeepEqual(got, first) {
			t.Fatalf("order %d gave a different result:\n got %+v\nwant %+v", i, got, first)
		}
	}
}
