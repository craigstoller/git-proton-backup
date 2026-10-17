package repo

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// RED (compile failure: caseKey did not exist). caseKey is the equivalence
// strings.EqualFold implements, made into a map key: the pairs cover ASCII,
// a non-ASCII letter, the Kelvin sign (a three-member orbit, K/k/U+212A), a
// length mismatch, and sharp s, which Unicode SIMPLE folding does not equate
// with "SS" (that is full folding, which the spec rules out).
func TestCaseKeyMatchesEqualFold(t *testing.T) {
	pairs := []struct {
		a, b string
		want bool // what strings.EqualFold says; asserted, so the table cannot drift
	}{
		{"main", "Main", true},
		{"Über", "über", true},
		{"K", "K", true}, // KELVIN SIGN vs LATIN CAPITAL K
		{"main", "mainx", false},
		{"ß", "SS", false},
	}
	for _, p := range pairs {
		if got := strings.EqualFold(p.a, p.b); got != p.want {
			t.Fatalf("fixture: strings.EqualFold(%q, %q) = %v, table says %v", p.a, p.b, got, p.want)
		}
		if got := caseKey(p.a) == caseKey(p.b); got != p.want {
			t.Errorf("caseKey(%q) == caseKey(%q) is %v, want %v (strings.EqualFold); keys %q, %q",
				p.a, p.b, got, p.want, caseKey(p.a), caseKey(p.b))
		}
	}
}

// RED (compile failure). strings.EqualFold decodes every invalid byte to
// U+FFFD, so it calls "a\xffb" and "a\xfeb" equal. caseKey copies invalid
// bytes through unfolded instead, so two distinct invalid bytes never collide.
func TestCaseKeyKeepsInvalidBytesDistinct(t *testing.T) {
	if caseKey("a\xffb") == caseKey("a\xfeb") {
		t.Errorf("distinct invalid bytes must give distinct keys, both gave %q", caseKey("a\xffb"))
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
