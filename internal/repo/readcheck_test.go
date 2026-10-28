package repo

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Fixtures for the pure read checks (readcheck.go). The two spellings of
// "cafe" with an acute accent display identically: rcNFC is precomposed
// (U+00E9), rcNFD is decomposed (e, then U+0301). Go escapes keep the source
// unambiguous; no raw combining mark appears in this file.
//
// Each test is labelled RED (it fails against a stub that returns zero values,
// and the label names the assertion that fires) or GUARD (it passes against
// that stub and pins a behaviour that must not change). Before the stub,
// every test here failed to compile.
const (
	rcNFC = "caf\u00e9"
	rcNFD = "cafe\u0301"
)

// rcOID is a 40-character object id made of the one hex digit c.
func rcOID(c byte) string { return strings.Repeat(string(rune(c)), 40) }

func rcSet(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

// rcPos, rcNeg and rcNoDst build specs by hand, so the tests of the mapping
// functions do not depend on ParseFetchRefspecs.
func rcPos(src, dst string) Refspec { return Refspec{Src: src, Dst: dst, Force: true} }
func rcNeg(src string) Refspec      { return Refspec{Src: src, Negative: true} }
func rcNoDst(src string) Refspec    { return Refspec{Src: src, NoDst: true} }

// rcShuffled returns names in the pseudo-random order seed picks.
func rcShuffled(names []string, seed int64) []string {
	out := append([]string(nil), names...)
	rand.New(rand.NewSource(seed)).Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// RED (stub returns nil: "AdvertisedPairs = [], want [...]"). A triple gives
// three pairs, a precomposed and a decomposed spelling give one canonical
// pair, and a pair matching both relations (main/Main, which also share a
// canonical key) is reported once, as letter case.
func TestAdvertisedPairsNamesEachPairOnce(t *testing.T) {
	refs := map[string]string{
		"HEAD":                rcOID('0'),
		"refs/heads/main":     rcOID('1'),
		"refs/heads/Main":     rcOID('2'),
		"refs/heads/MAIN":     rcOID('3'),
		"refs/heads/other":    rcOID('4'),
		"refs/heads/" + rcNFC: rcOID('5'),
		"refs/heads/" + rcNFD: rcOID('6'),
		"refs/tags/v1":        rcOID('7'),
	}
	want := []RefPair{
		{"refs/heads/MAIN", "refs/heads/Main", relCase},
		{"refs/heads/MAIN", "refs/heads/main", relCase},
		{"refs/heads/Main", "refs/heads/main", relCase},
		{"refs/heads/" + rcNFD, "refs/heads/" + rcNFC, relCanon},
	}
	got := AdvertisedPairs(refs)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AdvertisedPairs = %+v\nwant %+v", got, want)
	}
	for _, p := range got {
		if !(p.A < p.B) {
			t.Errorf("pair %+v: A must sort before B", p)
		}
	}
	// A repeated call gives the same answer.
	if again := AdvertisedPairs(refs); !reflect.DeepEqual(again, got) {
		t.Errorf("second call = %+v, first = %+v", again, got)
	}
}

// GUARD (a nil result and an empty one both satisfy it): refs with no pair
// give no pairs, including a nil map and names outside refs/.
func TestAdvertisedPairsOfRefsWithoutPairsIsEmpty(t *testing.T) {
	if got := AdvertisedPairs(nil); len(got) != 0 {
		t.Errorf("AdvertisedPairs(nil) = %+v, want none", got)
	}
	refs := map[string]string{
		"HEAD":             rcOID('0'),
		"refs/heads/main":  rcOID('1'),
		"refs/heads/other": rcOID('2'),
		"refs/tags/main":   rcOID('3'),
	}
	if got := AdvertisedPairs(refs); len(got) != 0 {
		t.Errorf("AdvertisedPairs = %+v, want none: main and tags/main differ in a component", got)
	}
}

// RED (stub returns nil: "directory pairs: got [], want ..."). Two refs under
// directories that differ only by case or normalisation conflict as a pair,
// one pair per cross-directory pair of refs; two refs under the SAME
// directory spelling do not.
func TestAdvertisedPairsCountsDirectoryPairs(t *testing.T) {
	refs := map[string]string{
		"refs/heads/Dir/a": rcOID('1'),
		"refs/heads/Dir/b": rcOID('2'),
		"refs/heads/dir/c": rcOID('3'),
		"refs/heads/dir/d": rcOID('4'),
	}
	want := []RefPair{
		{"refs/heads/Dir/a", "refs/heads/dir/c", relCase},
		{"refs/heads/Dir/a", "refs/heads/dir/d", relCase},
		{"refs/heads/Dir/b", "refs/heads/dir/c", relCase},
		{"refs/heads/Dir/b", "refs/heads/dir/d", relCase},
	}
	if got := AdvertisedPairs(refs); !reflect.DeepEqual(got, want) {
		t.Errorf("directory pairs: got %+v\nwant %+v", got, want)
	}

	canon := map[string]string{
		"refs/heads/" + rcNFC + "/x": rcOID('1'),
		"refs/heads/" + rcNFD + "/y": rcOID('2'),
		"refs/heads/" + rcNFC + "/z": rcOID('3'),
	}
	wantCanon := []RefPair{
		{"refs/heads/" + rcNFD + "/y", "refs/heads/" + rcNFC + "/x", relCanon},
		{"refs/heads/" + rcNFD + "/y", "refs/heads/" + rcNFC + "/z", relCanon},
	}
	if got := AdvertisedPairs(canon); !reflect.DeepEqual(got, wantCanon) {
		t.Errorf("canonical directory pairs: got %+v\nwant %+v", got, wantCanon)
	}
}

// RED (stub returns nil: "order 0: got [], want ..."). The pairs do not depend
// on the order the map was filled in: 50 fill orders give one answer.
func TestAdvertisedPairsDoNotDependOnMapOrder(t *testing.T) {
	names := []string{
		"refs/heads/main", "refs/heads/Main", "refs/heads/MAIN", "refs/heads/zeta",
		"refs/heads/" + rcNFC, "refs/heads/" + rcNFD, "refs/tags/Rel/1", "refs/tags/rel/2",
	}
	want := []RefPair{
		{"refs/heads/MAIN", "refs/heads/Main", relCase},
		{"refs/heads/MAIN", "refs/heads/main", relCase},
		{"refs/heads/Main", "refs/heads/main", relCase},
		{"refs/heads/" + rcNFD, "refs/heads/" + rcNFC, relCanon},
		{"refs/tags/Rel/1", "refs/tags/rel/2", relCase},
	}
	for seed := int64(0); seed < 50; seed++ {
		refs := map[string]string{}
		for i, n := range rcShuffled(names, seed) {
			refs[n] = rcOID(byte('a' + i))
		}
		if got := AdvertisedPairs(refs); !reflect.DeepEqual(got, want) {
			t.Fatalf("order %d: got %+v\nwant %+v", seed, got, want)
		}
	}
}

// RED (stub returns "": "PairLines = \"\", want ..."). One line per pair, in
// the pairs' order, joined by newlines with none at the end. Each name is
// followed by its object id and its %+q form, which tells an NFC name from its
// NFD twin; a pair's relation closes the line. PairWarning is its header, a
// newline and those lines, or "" for no pairs.
func TestPairLinesShowObjectIDsAndEscapeForms(t *testing.T) {
	refs := map[string]string{
		"refs/heads/main":     rcOID('1'),
		"refs/heads/Main":     rcOID('2'),
		"refs/heads/" + rcNFC: rcOID('5'),
		"refs/heads/" + rcNFD: rcOID('6'),
	}
	pairs := []RefPair{
		{"refs/heads/Main", "refs/heads/main", relCase},
		{"refs/heads/" + rcNFD, "refs/heads/" + rcNFC, relCanon},
	}
	line1 := "  refs/heads/Main (" + rcOID('2') + `, "refs/heads/Main") and refs/heads/main (` +
		rcOID('1') + `, "refs/heads/main"): by letter case`
	line2 := "  refs/heads/" + rcNFD + " (" + rcOID('6') + `, "refs/heads/cafe\u0301") and refs/heads/` +
		rcNFC + " (" + rcOID('5') + `, "refs/heads/caf\u00e9"): by Unicode normalisation`

	if got, want := PairLines(pairs, refs), line1+"\n"+line2; got != want {
		t.Errorf("PairLines =\n%s\nwant\n%s", got, want)
	}
	if got, want := PairLines(pairs[:1], refs), line1; got != want {
		t.Errorf("PairLines of one pair =\n%s\nwant\n%s", got, want)
	}

	header := "warning: this remote holds refs that differ only by letter case or Unicode " +
		"normalisation; see README, Honest limits:"
	if got, want := PairWarning(pairs, refs), header+"\n"+line1+"\n"+line2; got != want {
		t.Errorf("PairWarning =\n%s\nwant\n%s", got, want)
	}
}

// GUARD (the stub returns "" too): no pairs, no text.
func TestPairLinesAndWarningAreEmptyForNoPairs(t *testing.T) {
	refs := map[string]string{"refs/heads/main": rcOID('1')}
	if got := PairLines(nil, refs); got != "" {
		t.Errorf("PairLines(nil) = %q, want empty", got)
	}
	if got := PairWarning(nil, refs); got != "" {
		t.Errorf("PairWarning(nil) = %q, want empty", got)
	}
	if got := PairWarning([]RefPair{}, refs); got != "" {
		t.Errorf("PairWarning([]) = %q, want empty", got)
	}
}

// RED (stub returns nil, nil: the valid rows fail with "ParseFetchRefspecs(...)
// = [], want [...]", the invalid rows with "no error"). The refspec shapes
// R4 and PM2 allow, and the ones it refuses; an error names the value.
func TestParseFetchRefspecs(t *testing.T) {
	valid := []struct {
		in   string
		want Refspec
	}{
		{"+refs/heads/*:refs/remotes/o/*", Refspec{Src: "refs/heads/*", Dst: "refs/remotes/o/*", Force: true}},
		{"refs/heads/main:refs/remotes/o/main", Refspec{Src: "refs/heads/main", Dst: "refs/remotes/o/main"}},
		{"refs/heads/main", Refspec{Src: "refs/heads/main", NoDst: true}},
		{"refs/heads/main:", Refspec{Src: "refs/heads/main", NoDst: true}},
		{"+refs/heads/main", Refspec{Src: "refs/heads/main", Force: true, NoDst: true}},
		{"+refs/heads/main:", Refspec{Src: "refs/heads/main", Force: true, NoDst: true}},
		{"^refs/heads/tmp/*", Refspec{Src: "refs/heads/tmp/*", Negative: true}},
		{"^refs/heads/exact", Refspec{Src: "refs/heads/exact", Negative: true}},
		{"refs/heads/feat*:refs/remotes/o/f-*", Refspec{Src: "refs/heads/feat*", Dst: "refs/remotes/o/f-*"}},
		{"+refs/*:refs/*", Refspec{Src: "refs/*", Dst: "refs/*", Force: true}},
		{"refs/tags/v1:refs/tags/v1", Refspec{Src: "refs/tags/v1", Dst: "refs/tags/v1"}},
	}
	for _, c := range valid {
		got, err := ParseFetchRefspecs([]string{c.in})
		if err != nil {
			t.Errorf("ParseFetchRefspecs(%+q) error: %v", c.in, err)
			continue
		}
		if want := []Refspec{c.want}; !reflect.DeepEqual(got, want) {
			t.Errorf("ParseFetchRefspecs(%+q) = %+v, want %+v", c.in, got, want)
		}
	}

	invalid := []string{
		"heads/main:refs/remotes/o/main", // abbreviated source: git expands it (PM2)
		"refs/heads/main:heads/x",        // abbreviated destination
		"HEAD:refs/remotes/o/HEAD",
		"refs/heads/*:refs/remotes/o/x", // wildcard source, literal destination
		"refs/*/a/*:refs/x/*",           // two wildcards in the source
		"",
		"refs/heads/*",                     // wildcard source with no destination
		"refs/heads/*:",                    // the same, with an empty one
		"refs/heads/main:refs/remotes/o/*", // wildcard destination, literal source
		"refs/heads/*:refs/remotes/*/*",    // two wildcards in the destination
		"^heads/x",
		"^HEAD",
		"^refs/heads/a:refs/x/a", // a negative refspec has no destination
		"^refs/heads/a:",
		"+",
		"^",
		"refs/",
		"refs/heads/a:refs/b:refs/c",
		":refs/x",
		"+^refs/heads/a",
		"^+refs/heads/a",
		"main",
		"HEAD",
		" refs/heads/a",
		"refs/heads/a:HEAD",
	}
	for _, in := range invalid {
		got, err := ParseFetchRefspecs([]string{in})
		if err == nil {
			t.Errorf("ParseFetchRefspecs(%+q) = %+v, want an error", in, got)
			continue
		}
		if got != nil {
			t.Errorf("ParseFetchRefspecs(%+q) returned %+v with its error", in, got)
		}
		if !strings.Contains(err.Error(), fmt.Sprintf("%+q", in)) {
			t.Errorf("ParseFetchRefspecs(%+q) error %q does not name the value", in, err)
		}
	}

	// One bad value spoils the list, and the error names that value only.
	_, err := ParseFetchRefspecs([]string{"+refs/heads/*:refs/remotes/o/*", "HEAD:refs/x", "refs/tags/*:refs/tags/*"})
	if err == nil || !strings.Contains(err.Error(), `"HEAD:refs/x"`) || strings.Contains(err.Error(), "remotes/o") {
		t.Errorf("a list holding one bad value: error = %v, want one naming only HEAD:refs/x", err)
	}

	// A list keeps its order, and an empty one is empty.
	got, err := ParseFetchRefspecs([]string{"^refs/heads/tmp/*", "+refs/heads/*:refs/remotes/o/*"})
	want := []Refspec{
		{Src: "refs/heads/tmp/*", Negative: true},
		{Src: "refs/heads/*", Dst: "refs/remotes/o/*", Force: true},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("ParseFetchRefspecs(two values) = %+v, %v; want %+v", got, err, want)
	}
	if got, err := ParseFetchRefspecs(nil); err != nil || len(got) != 0 {
		t.Errorf("ParseFetchRefspecs(nil) = %+v, %v; want nothing", got, err)
	}
}

// RED (stub returns nil: "DefaultFetchRefspecs(origin) = [], want ...").
func TestDefaultFetchRefspecs(t *testing.T) {
	for _, name := range []string{"origin", "back.up"} {
		want := []Refspec{{Src: "refs/heads/*", Dst: "refs/remotes/" + name + "/*", Force: true}}
		if got := DefaultFetchRefspecs(name); !reflect.DeepEqual(got, want) {
			t.Errorf("DefaultFetchRefspecs(%q) = %+v, want %+v", name, got, want)
		}
	}
}

// RED for the rows that map (stub returns nil: "MapRef(...) = [], want [...]");
// the rows that map to nothing are GUARDs against the stub.
func TestMapRef(t *testing.T) {
	def := rcPos("refs/heads/*", "refs/remotes/o/*")
	cases := []struct {
		name  string
		specs []Refspec
		ref   string
		want  []string
	}{
		{"substitution", []Refspec{def}, "refs/heads/feat/x", []string{"refs/remotes/o/feat/x"}},
		{"no spec matches", []Refspec{def}, "refs/tags/v1", nil},
		{"exact spec", []Refspec{rcPos("refs/heads/main", "refs/remotes/o/m")}, "refs/heads/main", []string{"refs/remotes/o/m"}},
		{"exact spec, other ref", []Refspec{rcPos("refs/heads/main", "refs/remotes/o/m")}, "refs/heads/main2", nil},
		{"negative pattern beats positive", []Refspec{def, rcNeg("refs/heads/tmp/*")}, "refs/heads/tmp/a", nil},
		{"negative first, same answer", []Refspec{rcNeg("refs/heads/tmp/*"), def}, "refs/heads/tmp/a", nil},
		{"negative exact beats positive", []Refspec{def, rcNeg("refs/heads/main")}, "refs/heads/main", nil},
		{"negative that does not match", []Refspec{def, rcNeg("refs/heads/tmp/*")}, "refs/heads/ok", []string{"refs/remotes/o/ok"}},
		{"two destinations, sorted", []Refspec{def, rcPos("refs/heads/main", "refs/mirror/main")}, "refs/heads/main",
			[]string{"refs/mirror/main", "refs/remotes/o/main"}},
		{"one destination twice", []Refspec{def, rcPos("refs/heads/m*", "refs/remotes/o/m*")}, "refs/heads/main",
			[]string{"refs/remotes/o/main"}},
		{"NoDst gives nothing", []Refspec{rcNoDst("refs/heads/main")}, "refs/heads/main", nil},
		{"NoDst beside a positive", []Refspec{rcNoDst("refs/heads/main"), rcPos("refs/heads/main", "refs/x/m")}, "refs/heads/main",
			[]string{"refs/x/m"}},
		{"star inside a component", []Refspec{rcPos("refs/heads/a*z", "refs/r/a*z")}, "refs/heads/amz", []string{"refs/r/amz"}},
		{"star matching nothing", []Refspec{rcPos("refs/heads/a*z", "refs/r/a*z")}, "refs/heads/az", []string{"refs/r/az"}},
		{"prefix and suffix may not overlap", []Refspec{rcPos("refs/heads/a*a", "refs/r/*")}, "refs/heads/a", nil},
		{"suffix must match", []Refspec{rcPos("refs/heads/a*z", "refs/r/a*z")}, "refs/heads/amy", nil},
		{"star crosses a slash", []Refspec{rcPos("refs/heads/*", "refs/r/*")}, "refs/heads/a/b/c", []string{"refs/r/a/b/c"}},
		{"no specs", nil, "refs/heads/main", nil},
	}
	for _, c := range cases {
		got := MapRef(c.specs, c.ref)
		if len(got) == 0 && len(c.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: MapRef(%+v, %q) = %v, want %v", c.name, c.specs, c.ref, got, c.want)
		}
	}
}

// RED (stub returns an empty map: "FetchDestinations = map[], want ..."). Every
// advertised tag is a destination, mapped to itself, whether or not a negative
// spec matches it (R6: tag backfill runs without negative filtering); a
// branch a negative spec matches is not.
func TestFetchDestinationsIncludesEveryTag(t *testing.T) {
	adv := map[string]string{
		"HEAD":              rcOID('0'),
		"refs/heads/main":   rcOID('1'),
		"refs/heads/tmp/x":  rcOID('2'),
		"refs/tags/v1":      rcOID('3'),
		"refs/tags/skip":    rcOID('4'),
		"refs/tags/ignored": rcOID('5'),
	}
	specs := []Refspec{
		rcPos("refs/heads/*", "refs/remotes/o/*"),
		rcNeg("refs/heads/tmp/*"),
		rcNeg("refs/tags/skip"),
		rcNeg("refs/tags/ignored"),
	}
	want := map[string]string{
		"refs/remotes/o/main": "refs/heads/main",
		"refs/remotes/o/HEAD": "HEAD",
		"refs/tags/v1":        "refs/tags/v1",
		"refs/tags/skip":      "refs/tags/skip",
		"refs/tags/ignored":   "refs/tags/ignored",
	}
	if got := FetchDestinations(adv, specs); !reflect.DeepEqual(got, want) {
		t.Errorf("FetchDestinations =\n%v\nwant\n%v", got, want)
	}
}

// RED (stub returns an empty map: "FetchDestinations = map[], want ..."). When
// several advertised refs map to one destination, the smallest source name
// is the one recorded, whatever the map's order; a tag that is also another
// spec's destination records the smaller of the two sources.
func TestFetchDestinationsKeepsTheSmallestSource(t *testing.T) {
	specs := []Refspec{
		rcPos("refs/other/*", "refs/remotes/o/*"),
		rcPos("refs/heads/*", "refs/remotes/o/*"),
		rcPos("refs/heads/t", "refs/tags/t"),
	}
	adv := map[string]string{
		"refs/heads/x": rcOID('1'),
		"refs/other/x": rcOID('2'),
		"refs/heads/t": rcOID('3'),
		"refs/tags/t":  rcOID('4'),
	}
	want := map[string]string{
		"refs/remotes/o/x": "refs/heads/x",
		"refs/remotes/o/t": "refs/heads/t",
		"refs/tags/t":      "refs/heads/t",
	}
	// The default-style spec in specs[1] also writes the tracking HEAD.
	want["refs/remotes/o/HEAD"] = "HEAD"
	for i := 0; i < 20; i++ {
		if got := FetchDestinations(adv, specs); !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d: FetchDestinations =\n%v\nwant\n%v", i, got, want)
		}
	}
}

// RED (stub returns nil: "PushTrackingRefs = [], want [...]"). Push tracking
// refs ignore negative specs (R6): git's push lookup can still pick a
// positive wildcard that a negative was meant to exclude. MapRef, which does
// honour the negative, maps the same ref to nothing (that line is a GUARD).
func TestPushTrackingRefsIgnoresNegatives(t *testing.T) {
	ref := "refs/heads/" + rcNFC
	specs := []Refspec{rcPos("refs/heads/*", "refs/remotes/r/*"), rcNeg(ref)}
	if got := MapRef(specs, ref); len(got) != 0 {
		t.Errorf("MapRef = %v, want nothing: the negative excludes the ref", got)
	}
	want := []string{"refs/remotes/r/" + rcNFC}
	if got := PushTrackingRefs(specs, ref); !reflect.DeepEqual(got, want) {
		t.Errorf("PushTrackingRefs = %v, want %v", got, want)
	}
	// Without a negative the two agree, sorted and deduplicated alike.
	two := []Refspec{rcPos("refs/heads/*", "refs/z/*"), rcPos("refs/heads/m*", "refs/z/m*"), rcPos("refs/heads/main", "refs/a/main"), rcNoDst("refs/heads/main")}
	wantTwo := []string{"refs/a/main", "refs/z/main"}
	if got := PushTrackingRefs(two, "refs/heads/main"); !reflect.DeepEqual(got, wantTwo) {
		t.Errorf("PushTrackingRefs = %v, want %v", got, wantTwo)
	}
	if got := PushTrackingRefs(specs, "refs/tags/v1"); len(got) != 0 {
		t.Errorf("PushTrackingRefs of an unmatched ref = %v, want nothing", got)
	}
}

// RED (stub returns an empty map: "PruneCandidates = map[], want ..."). Under
// +refs/heads/tmp/*:refs/remotes/r/Old/*, a local ref in that destination
// pattern that the fetch does not write is a candidate, with the empty source;
// a local ref outside every pattern is not, and neither is one the fetch
// writes. A literal destination covers its one name; negative and NoDst specs
// cover nothing.
func TestPruneCandidates(t *testing.T) {
	specs := []Refspec{rcPos("refs/heads/tmp/*", "refs/remotes/r/Old/*")}
	local := rcSet("refs/remotes/r/Old/a", "refs/remotes/r/Old/b", "refs/remotes/r/Other/z", "refs/heads/main", "refs/remotes/r/main")
	dests := map[string]string{"refs/remotes/r/Old/b": "refs/heads/tmp/b"}
	want := map[string]string{"refs/remotes/r/Old/a": ""}
	if got := PruneCandidates(specs, dests, local); !reflect.DeepEqual(got, want) {
		t.Errorf("PruneCandidates = %v, want %v", got, want)
	}

	// A literal destination, which is covered by name; a negative spec and a
	// spec with no destination cover nothing.
	lit := []Refspec{rcPos("refs/heads/main", "refs/remotes/r/m"), rcNeg("refs/heads/x"), rcNoDst("refs/heads/y")}
	litLocal := rcSet("refs/remotes/r/m", "refs/remotes/r/x", "refs/heads/y", "refs/heads/x")
	if got, want := PruneCandidates(lit, nil, litLocal), map[string]string{"refs/remotes/r/m": ""}; !reflect.DeepEqual(got, want) {
		t.Errorf("PruneCandidates (literal) = %v, want %v", got, want)
	}
	if got := PruneCandidates(lit, map[string]string{"refs/remotes/r/m": "refs/heads/main"}, litLocal); len(got) != 0 {
		t.Errorf("PruneCandidates of a written destination = %v, want nothing", got)
	}
	// Nothing local, nothing to delete.
	if got := PruneCandidates(specs, nil, nil); len(got) != 0 {
		t.Errorf("PruneCandidates over no local refs = %v, want nothing", got)
	}
}

// RED (stub returns an empty map: "refs/remotes/o/HEAD is not a destination").
// Under the default spec the tracking HEAD a clone or followRemoteHEAD writes
// is a destination, with source HEAD, and a remote branch called head then
// conflicts with it under the case relation (Codex). It is synthesized only
// where a positive spec maps refs/heads/* to <p>/*.
func TestFetchDestinationsIncludesTheTrackingHEAD(t *testing.T) {
	adv := map[string]string{"refs/heads/main": rcOID('1'), "refs/heads/head": rcOID('2')}
	dests := FetchDestinations(adv, DefaultFetchRefspecs("o"))
	if got := dests["refs/remotes/o/HEAD"]; got != "HEAD" {
		t.Fatalf("source of refs/remotes/o/HEAD = %q, want HEAD; destinations %v", got, dests)
	}
	cs := DestinationConflicts(dests, nil)
	if len(cs) != 1 {
		t.Fatalf("DestinationConflicts = %+v, want the one HEAD/head pair", cs)
	}
	c := cs[0]
	if c.Dest != "refs/remotes/o/HEAD" || c.Src != "HEAD" || c.Other != "refs/remotes/o/head" ||
		c.OtherSrc != "refs/heads/head" || c.C.Rel != relCase {
		t.Errorf("conflict = %+v, want HEAD (from HEAD) against head (from refs/heads/head) by letter case", c)
	}

	// Another prefix, as a mirror-style spec uses.
	if got := FetchDestinations(nil, []Refspec{rcPos("refs/heads/*", "refs/mirror/*")})["refs/mirror/HEAD"]; got != "HEAD" {
		t.Errorf("source of refs/mirror/HEAD = %q, want HEAD", got)
	}

	// Specs that do not map refs/heads/* to <p>/* synthesize no HEAD (GUARD).
	for name, specs := range map[string][]Refspec{
		"exact branch":          {rcPos("refs/heads/main", "refs/remotes/o/main")},
		"star not at the end":   {rcPos("refs/heads/*", "refs/remotes/o/pre-*")},
		"other source":          {rcPos("refs/tags/*", "refs/rt/*")},
		"negative only":         {rcNeg("refs/heads/*")},
		"no destination":        {rcNoDst("refs/heads/main")},
		"partial source":        {rcPos("refs/heads/m*", "refs/remotes/o/m*")},
		"everything, not heads": {rcPos("refs/*", "refs/other/*")},
	} {
		for d := range FetchDestinations(nil, specs) {
			if strings.HasSuffix(d, "/HEAD") {
				t.Errorf("%s: unexpected destination %q", name, d)
			}
		}
	}
}

// RED (stub returns an empty map: "ExpandSymrefs = map[], want ..."). A
// destination that is a local symbolic ref adds its target, with the same
// source, and so on down a chain; a cycle ends; a symbolic ref that is not a
// destination, an entry with no target and a target that is already a
// destination change nothing; when two destinations reach one target, the
// smaller source wins; and the inputs are left alone.
func TestExpandSymrefsAddsTargets(t *testing.T) {
	dests := map[string]string{
		"refs/remotes/o/foo":  "refs/heads/foo",
		"refs/remotes/o/HEAD": "HEAD",
		"refs/remotes/o/c1":   "refs/heads/c1",
		"refs/remotes/o/p":    "refs/heads/p",
		"refs/remotes/o/q":    "refs/heads/q",
		"refs/remotes/o/old":  "refs/heads/old",
		"refs/remotes/o/new":  "refs/heads/new",
		"refs/remotes/o/d1":   "refs/heads/b",
		"refs/remotes/o/d2":   "refs/heads/a",
	}
	symrefs := map[string]string{
		"refs/remotes/o/foo":   "refs/heads/bar",      // brief's case
		"refs/heads/bar":       "refs/heads/baz",      // a chain
		"refs/remotes/o/HEAD":  "refs/remotes/o/main", // HEAD's target, with source HEAD
		"refs/remotes/o/c1":    "refs/remotes/o/c2",   // a cycle
		"refs/remotes/o/c2":    "refs/remotes/o/c1",
		"refs/remotes/o/p":     "",                   // not symbolic
		"refs/heads/elsewhere": "refs/heads/never",   // not a destination, not reached
		"refs/remotes/o/old":   "refs/remotes/o/new", // the target is a destination already
		"refs/remotes/o/d1":    "refs/heads/shared",  // two destinations, one target
		"refs/remotes/o/d2":    "refs/heads/shared",
	}
	destsBefore, symrefsBefore := map[string]string{}, map[string]string{}
	for k, v := range dests {
		destsBefore[k] = v
	}
	for k, v := range symrefs {
		symrefsBefore[k] = v
	}

	want := map[string]string{
		"refs/remotes/o/foo":  "refs/heads/foo",
		"refs/remotes/o/HEAD": "HEAD",
		"refs/remotes/o/c1":   "refs/heads/c1",
		"refs/remotes/o/p":    "refs/heads/p",
		"refs/remotes/o/q":    "refs/heads/q",
		"refs/remotes/o/old":  "refs/heads/old",
		"refs/remotes/o/new":  "refs/heads/new",
		"refs/remotes/o/d1":   "refs/heads/b",
		"refs/remotes/o/d2":   "refs/heads/a",
		"refs/heads/bar":      "refs/heads/foo",
		"refs/heads/baz":      "refs/heads/foo",
		"refs/remotes/o/main": "HEAD",
		"refs/remotes/o/c2":   "refs/heads/c1",
		"refs/heads/shared":   "refs/heads/a",
	}
	got := ExpandSymrefs(dests, symrefs)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExpandSymrefs =\n%v\nwant\n%v", got, want)
	}
	if !reflect.DeepEqual(dests, destsBefore) || !reflect.DeepEqual(symrefs, symrefsBefore) {
		t.Error("ExpandSymrefs changed one of its inputs")
	}

	// With nothing symbolic, the answer is the destinations (GUARD).
	if got := ExpandSymrefs(destsBefore, nil); !reflect.DeepEqual(got, destsBefore) {
		t.Errorf("ExpandSymrefs with no symrefs = %v, want %v", got, destsBefore)
	}
}

// RED (stub returns "": "DestLines =\n\nwant ..."). Two aliasing destinations
// name each one's source; a destination with a source and a known object id
// reads "from <src>, <oid>"; a prune candidate (empty source) reads "may be
// pruned"; a conflict with a local ref says "local"; and the escape forms are
// those of the destination and the other ref, whole. The lines are joined by
// newlines with none at the end.
func TestDestLinesNamesBothSources(t *testing.T) {
	dFold := "refs/remotes/r/" + rcNFD + "/b"
	dCompose := "refs/remotes/r/" + rcNFC + "/a"
	refs := map[string]string{
		"refs/heads/one/a": rcOID('1'),
		"refs/heads/two/b": rcOID('2'),
	}
	cs := []DestConflict{
		{
			Dest: dFold, Src: "refs/heads/two/b", Other: dCompose, OtherSrc: "refs/heads/one/a",
			C: caseConflict{Ref: dCompose, Ours: "remotes/r/" + rcNFD, Theirs: "remotes/r/" + rcNFC, Rel: relCanon},
		},
		{
			Dest: "refs/remotes/r/Old/a", Src: "", Other: "refs/remotes/r/old/a",
			C: caseConflict{Ref: "refs/remotes/r/old/a", Ours: "remotes/r/Old", Theirs: "remotes/r/old", Rel: relCase},
		},
		{
			Dest: "refs/remotes/o/HEAD", Src: "HEAD", Other: "refs/remotes/o/head",
			C: caseConflict{Ref: "refs/remotes/o/head", Ours: "remotes/o/HEAD", Theirs: "remotes/o/head", Rel: relCase},
		},
	}
	line1 := "  " + dFold + " (from refs/heads/two/b, " + rcOID('2') + ") and " + dCompose +
		" (also written by this fetch, from refs/heads/one/a) " +
		`("refs/remotes/r/cafe\u0301/b" vs "refs/remotes/r/caf\u00e9/a"): by Unicode normalisation`
	line2 := "  refs/remotes/r/Old/a (may be pruned) and refs/remotes/r/old/a (local) " +
		`("refs/remotes/r/Old/a" vs "refs/remotes/r/old/a"): by letter case`
	// HEAD has no object id in refs (a symbolic ref is not advertised by id),
	// so the line names the source alone.
	line3 := "  refs/remotes/o/HEAD (from HEAD) and refs/remotes/o/head (local) " +
		`("refs/remotes/o/HEAD" vs "refs/remotes/o/head"): by letter case`
	if got, want := DestLines(cs, refs), line1+"\n"+line2+"\n"+line3; got != want {
		t.Errorf("DestLines =\n%s\nwant\n%s", got, want)
	}

	// When the advertisement does hold an id for HEAD, it is shown.
	withHead := map[string]string{"HEAD": rcOID('9')}
	want := "  refs/remotes/o/HEAD (from HEAD, " + rcOID('9') + ") and refs/remotes/o/head (local) " +
		`("refs/remotes/o/HEAD" vs "refs/remotes/o/head"): by letter case`
	if got := DestLines(cs[2:], withHead); got != want {
		t.Errorf("DestLines (HEAD with an id) =\n%s\nwant\n%s", got, want)
	}

	// GUARD (the stub gives "" too).
	if got := DestLines(nil, refs); got != "" {
		t.Errorf("DestLines(nil) = %q, want empty", got)
	}
}

// RED (stub returns nil: "MergeConflicts = [], want ..."). A pair b repeats
// from a, in either orientation, is dropped; a pair repeated inside b is kept
// once; the rest is kept; the result is sorted; and the inputs are left alone.
func TestMergeConflictsDropsDuplicates(t *testing.T) {
	mk := func(dest, other string) DestConflict {
		return DestConflict{Dest: dest, Src: "s:" + dest, Other: other, C: caseConflict{Ref: other, Rel: relCase}}
	}
	a := []DestConflict{mk("refs/r/m", "refs/r/M"), mk("refs/r/f", "refs/r/F")}
	b := []DestConflict{
		mk("refs/r/M", "refs/r/m"), // a's first pair, reversed
		mk("refs/r/f", "refs/r/F"), // a's second pair, as it is
		mk("refs/r/k", "refs/r/K"), // new, and repeated just below in the other orientation
		mk("refs/r/K", "refs/r/k"), // b's own repeat; this orientation sorts first, so it is the one kept
		mk("refs/r/b", "refs/r/B"), // new
	}
	aBefore := append([]DestConflict(nil), a...)
	bBefore := append([]DestConflict(nil), b...)

	// Sorted by Dest: "K" (0x4B) sorts before "b" (0x62).
	want := []DestConflict{
		mk("refs/r/K", "refs/r/k"),
		mk("refs/r/b", "refs/r/B"),
		mk("refs/r/f", "refs/r/F"),
		mk("refs/r/m", "refs/r/M"),
	}
	got := MergeConflicts(a, b)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MergeConflicts =\n%+v\nwant\n%+v", got, want)
	}
	if !reflect.DeepEqual(a, aBefore) || !reflect.DeepEqual(b, bBefore) {
		t.Error("MergeConflicts changed one of its inputs")
	}
	// The order of the arguments' contents does not change the result.
	if again := MergeConflicts([]DestConflict{a[1], a[0]}, []DestConflict{b[4], b[3], b[2], b[1], b[0]}); !reflect.DeepEqual(again, want) {
		t.Errorf("MergeConflicts of reordered inputs =\n%+v\nwant\n%+v", again, want)
	}
	// GUARD (the stub gives nothing too): nothing in, nothing out.
	if got := MergeConflicts(nil, nil); len(got) != 0 {
		t.Errorf("MergeConflicts(nil, nil) = %+v", got)
	}
}

// rcConflict is the DestConflict a test expects, with the caseConflict built
// from the two differently spelled prefixes.
func rcConflict(dest, src, other, otherSrc, ours, theirs string, rel relation) DestConflict {
	return DestConflict{Dest: dest, Src: src, Other: other, OtherSrc: otherSrc,
		C: caseConflict{Ref: other, Ours: ours, Theirs: theirs, Rel: rel}}
}

// RED for each subtest that expects a conflict (stub returns nil:
// "DestinationConflicts = [], want ..."); the subtests that expect none are
// GUARDs against the stub.
func TestDestinationConflicts(t *testing.T) {
	t.Run("a destination against a local ref spelled differently", func(t *testing.T) {
		dests := map[string]string{"refs/remotes/o/" + rcNFC: "refs/heads/" + rcNFC}
		local := rcSet("refs/remotes/o/" + rcNFD)
		want := []DestConflict{rcConflict("refs/remotes/o/"+rcNFC, "refs/heads/"+rcNFC,
			"refs/remotes/o/"+rcNFD, "", "remotes/o/"+rcNFC, "remotes/o/"+rcNFD, relCanon)}
		if got := DestinationConflicts(dests, local); !reflect.DeepEqual(got, want) {
			t.Errorf("DestinationConflicts = %+v\nwant %+v", got, want)
		}
	})

	t.Run("the same bytes do not conflict", func(t *testing.T) {
		dests := map[string]string{"refs/remotes/o/" + rcNFC: "refs/heads/" + rcNFC, "refs/remotes/o/main": "refs/heads/main"}
		local := rcSet("refs/remotes/o/"+rcNFC, "refs/remotes/o/main")
		if got := DestinationConflicts(dests, local); len(got) != 0 {
			t.Errorf("DestinationConflicts = %+v, want none", got)
		}
	})

	t.Run("two destinations against each other, reported once (Codex)", func(t *testing.T) {
		dests := map[string]string{
			"refs/remotes/r/" + rcNFC + "/a": "refs/heads/one/a",
			"refs/remotes/r/" + rcNFD + "/b": "refs/heads/two/b",
		}
		// The decomposed spelling sorts first ('e' is below the lead byte of
		// the precomposed letter), so it is the Dest.
		want := []DestConflict{rcConflict(
			"refs/remotes/r/"+rcNFD+"/b", "refs/heads/two/b",
			"refs/remotes/r/"+rcNFC+"/a", "refs/heads/one/a",
			"remotes/r/"+rcNFD, "remotes/r/"+rcNFC, relCanon)}
		if got := DestinationConflicts(dests, nil); !reflect.DeepEqual(got, want) {
			t.Errorf("DestinationConflicts = %+v\nwant %+v", got, want)
		}
		// A local ref of the same bytes as one destination changes nothing.
		if got := DestinationConflicts(dests, rcSet("refs/remotes/r/"+rcNFC+"/a")); !reflect.DeepEqual(got, want) {
			t.Errorf("DestinationConflicts with that destination also local = %+v\nwant %+v", got, want)
		}
	})

	t.Run("a pair of local refs alone is not reported", func(t *testing.T) {
		dests := map[string]string{"refs/remotes/r/unrelated": "refs/heads/unrelated"}
		local := rcSet("refs/remotes/r/Foo", "refs/remotes/r/foo", "refs/heads/main", "refs/heads/Main")
		if got := DestinationConflicts(dests, local); len(got) != 0 {
			t.Errorf("DestinationConflicts = %+v, want none", got)
		}
		if got := DestinationConflicts(nil, local); len(got) != 0 {
			t.Errorf("DestinationConflicts(nil, local) = %+v, want none", got)
		}
	})

	t.Run("one destination against several local refs, sorted by Other", func(t *testing.T) {
		dests := map[string]string{"refs/remotes/r/solo": "refs/heads/solo"}
		local := rcSet("refs/remotes/r/Solo", "refs/remotes/r/SOLO", "refs/remotes/r/solo")
		want := []DestConflict{
			rcConflict("refs/remotes/r/solo", "refs/heads/solo", "refs/remotes/r/SOLO", "", "remotes/r/solo", "remotes/r/SOLO", relCase),
			rcConflict("refs/remotes/r/solo", "refs/heads/solo", "refs/remotes/r/Solo", "", "remotes/r/solo", "remotes/r/Solo", relCase),
		}
		if got := DestinationConflicts(dests, local); !reflect.DeepEqual(got, want) {
			t.Errorf("DestinationConflicts = %+v\nwant %+v", got, want)
		}
	})

	// One fixture with every kind of pair at once, filled in 50 orders.
	destNames := []string{
		"refs/remotes/r/Foo", "refs/remotes/r/foo",
		"refs/remotes/r/" + rcNFC + "/a", "refs/remotes/r/" + rcNFD + "/b",
		"refs/remotes/r/solo", "refs/remotes/r/plain",
	}
	srcOf := map[string]string{
		"refs/remotes/r/Foo":             "refs/heads/Foo",
		"refs/remotes/r/foo":             "refs/heads/foo",
		"refs/remotes/r/" + rcNFC + "/a": "refs/heads/one/a",
		"refs/remotes/r/" + rcNFD + "/b": "refs/heads/two/b",
		"refs/remotes/r/solo":            "refs/heads/solo",
		"refs/remotes/r/plain":           "refs/heads/plain",
	}
	localNames := []string{
		"refs/remotes/r/SOLO", "refs/remotes/r/Solo", // each against the destination solo; also a local pair
		"refs/remotes/r/plain",               // the same bytes as a destination
		"refs/remotes/r/Foo",                 // local and a destination: the pair is a destination pair
		"refs/heads/main", "refs/heads/Main", // a local pair, no destination near
	}
	want := []DestConflict{
		rcConflict("refs/remotes/r/Foo", "refs/heads/Foo", "refs/remotes/r/foo", "refs/heads/foo", "remotes/r/Foo", "remotes/r/foo", relCase),
		rcConflict("refs/remotes/r/"+rcNFD+"/b", "refs/heads/two/b", "refs/remotes/r/"+rcNFC+"/a", "refs/heads/one/a",
			"remotes/r/"+rcNFD, "remotes/r/"+rcNFC, relCanon),
		rcConflict("refs/remotes/r/solo", "refs/heads/solo", "refs/remotes/r/SOLO", "", "remotes/r/solo", "remotes/r/SOLO", relCase),
		rcConflict("refs/remotes/r/solo", "refs/heads/solo", "refs/remotes/r/Solo", "", "remotes/r/solo", "remotes/r/Solo", relCase),
	}
	t.Run("deterministic over 50 orders", func(t *testing.T) {
		for seed := int64(0); seed < 50; seed++ {
			dests := map[string]string{}
			for _, n := range rcShuffled(destNames, seed) {
				dests[n] = srcOf[n]
			}
			local := map[string]bool{}
			for _, n := range rcShuffled(localNames, seed+1000) {
				local[n] = true
			}
			if got := DestinationConflicts(dests, local); !reflect.DeepEqual(got, want) {
				t.Fatalf("order %d: got %+v\nwant %+v", seed, got, want)
			}
		}
	})
}

// RED (stub returns nil: "LocalConflicts = [], want ..."). The same check over
// the local refs less the destinations: a candidate is checked against the
// other local refs alone, OtherSrc is always empty, and Codex's pair of
// destinations gives nothing.
func TestLocalConflictsIgnoresDestinationPairs(t *testing.T) {
	codex := map[string]string{
		"refs/remotes/r/" + rcNFC + "/a": "refs/heads/one/a",
		"refs/remotes/r/" + rcNFD + "/b": "refs/heads/two/b",
	}
	if got := LocalConflicts(codex, nil); len(got) != 0 {
		t.Errorf("LocalConflicts of Codex's pair = %+v, want none", got)
	}
	// Those refs being local too does not make them each other's local refs.
	if got := LocalConflicts(codex, rcSet("refs/remotes/r/"+rcNFC+"/a", "refs/remotes/r/"+rcNFD+"/b")); len(got) != 0 {
		t.Errorf("LocalConflicts with the destinations also local = %+v, want none", got)
	}

	// A prune candidate is a local ref: it is left out of the set it is
	// checked against, and found against another local ref it aliases.
	cands := map[string]string{"refs/remotes/r/Old/a": ""}
	local := rcSet("refs/remotes/r/Old/a", "refs/remotes/r/old/a", "refs/remotes/r/keep", "refs/remotes/r/KEEP")
	want := []DestConflict{rcConflict("refs/remotes/r/Old/a", "", "refs/remotes/r/old/a", "", "remotes/r/Old", "remotes/r/old", relCase)}
	if got := LocalConflicts(cands, local); !reflect.DeepEqual(got, want) {
		t.Errorf("LocalConflicts = %+v\nwant %+v", got, want)
	}

	// Two candidates that alias each other are both leaving the set.
	both := map[string]string{"refs/remotes/r/Old/a": "", "refs/remotes/r/old/b": ""}
	if got := LocalConflicts(both, rcSet("refs/remotes/r/Old/a", "refs/remotes/r/old/b")); len(got) != 0 {
		t.Errorf("LocalConflicts of two candidates = %+v, want none", got)
	}
}

// RED (stub returns an empty map: "UpToDateTracking = map[], want ..."). R1:
// for each advertised ref the local repository holds by the exact same name,
// each push tracking ref that ref maps to (negative specs ignored, R6) is
// recorded with that ref as its source, the smallest if two share a tracking
// ref. A ref held only in another spelling is not up to date by name. Fed to
// LocalConflicts, it finds the stale alias of PM1.
func TestUpToDateTracking(t *testing.T) {
	adv := map[string]string{
		"refs/heads/" + rcNFC: rcOID('1'),
		"refs/heads/main":     rcOID('2'),
		"refs/heads/remote":   rcOID('3'),
		"refs/heads/Ab":       rcOID('4'),
		"refs/other/main":     rcOID('5'),
	}
	local := rcSet(
		"refs/heads/"+rcNFC, "refs/heads/main", "refs/other/main",
		"refs/heads/AB", // held in another spelling than the advertised Ab
		"refs/remotes/origin/"+rcNFD,
	)
	specs := []Refspec{
		rcPos("refs/other/*", "refs/remotes/origin/*"),
		rcPos("refs/heads/*", "refs/remotes/origin/*"),
		rcNeg("refs/heads/" + rcNFC),
	}
	want := map[string]string{
		"refs/remotes/origin/" + rcNFC: "refs/heads/" + rcNFC,
		"refs/remotes/origin/main":     "refs/heads/main", // refs/heads/main sorts before refs/other/main
	}
	got := UpToDateTracking(adv, specs, local)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("UpToDateTracking = %v, want %v", got, want)
	}

	wantConflict := []DestConflict{rcConflict("refs/remotes/origin/"+rcNFC, "refs/heads/"+rcNFC,
		"refs/remotes/origin/"+rcNFD, "", "remotes/origin/"+rcNFC, "remotes/origin/"+rcNFD, relCanon)}
	if cs := LocalConflicts(got, local); !reflect.DeepEqual(cs, wantConflict) {
		t.Errorf("LocalConflicts(UpToDateTracking) = %+v\nwant %+v", cs, wantConflict)
	}

	// GUARD (the stub gives an empty map too): nothing held locally, nothing up to date.
	if got := UpToDateTracking(adv, specs, nil); len(got) != 0 {
		t.Errorf("UpToDateTracking with no local refs = %v, want nothing", got)
	}
}

// RED (the stub's pieces return nothing: "merged = [], want 2 pairs"). The
// pure pieces compose as the fetch check needs: the destinations (tracking
// HEAD, symbolic targets), the destination pass, the prune pass, and the
// merge that keeps a pair found by both passes once.
func TestFetchCheckPiecesCompose(t *testing.T) {
	specs := DefaultFetchRefspecs("o")
	adv := map[string]string{
		"refs/heads/main":     rcOID('1'),
		"refs/heads/" + rcNFC: rcOID('2'),
	}
	local := rcSet(
		"refs/remotes/o/main",
		"refs/remotes/o/Main",                      // not advertised: a prune candidate that aliases the destination main
		"refs/remotes/o/"+rcNFD,                    // not advertised: a prune candidate that aliases the destination cafe
		"refs/remotes/o/old", "refs/remotes/o/Old", // two candidates aliasing only each other
		"refs/heads/main",
	)
	dests := ExpandSymrefs(FetchDestinations(adv, specs), map[string]string{"refs/remotes/o/HEAD": "refs/remotes/o/trunk"})
	wantDests := map[string]string{
		"refs/remotes/o/main":     "refs/heads/main",
		"refs/remotes/o/" + rcNFC: "refs/heads/" + rcNFC,
		"refs/remotes/o/HEAD":     "HEAD",
		"refs/remotes/o/trunk":    "HEAD",
	}
	if !reflect.DeepEqual(dests, wantDests) {
		t.Fatalf("destinations = %v, want %v", dests, wantDests)
	}

	a := DestinationConflicts(dests, local)
	prune := PruneCandidates(specs, dests, local)
	wantPrune := map[string]string{
		"refs/remotes/o/Main": "", "refs/remotes/o/" + rcNFD: "", "refs/remotes/o/old": "", "refs/remotes/o/Old": "",
	}
	if !reflect.DeepEqual(prune, wantPrune) {
		t.Fatalf("prune candidates = %v, want %v", prune, wantPrune)
	}
	b := LocalConflicts(prune, local)
	if len(a)+len(b) != 3 {
		t.Fatalf("the two passes found %d conflicts, want 3 (the main/Main pair twice): a %+v, b %+v", len(a)+len(b), a, b)
	}
	merged := MergeConflicts(a, b)

	var pairs []string
	for _, c := range merged {
		x, y := c.Dest, c.Other
		if y < x {
			x, y = y, x
		}
		pairs = append(pairs, x+" | "+y)
	}
	sort.Strings(pairs)
	wantPairs := []string{
		"refs/remotes/o/Main | refs/remotes/o/main",
		"refs/remotes/o/" + rcNFD + " | refs/remotes/o/" + rcNFC,
	}
	sort.Strings(wantPairs)
	if !reflect.DeepEqual(pairs, wantPairs) {
		t.Errorf("merged pairs = %v, want %v (a: %+v, b: %+v)", pairs, wantPairs, a, b)
	}
	// The text for them is one line each.
	if lines := strings.Split(DestLines(merged, adv), "\n"); len(lines) != 2 {
		t.Errorf("DestLines gave %d lines, want 2: %v", len(lines), lines)
	}
}
