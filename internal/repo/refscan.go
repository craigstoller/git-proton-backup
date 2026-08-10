package repo

import "fmt"

// SkipKind classifies WHY a path under refs/ was declined from the
// advertisement — see ScanRefs (refs.go) and the design spec's component 1
// ("Content classification at the advertisement boundary") and component 2
// ("Occupancy-aware push"). The three kinds exist because each demands a
// DIFFERENT operator remedy (OccupancyMessage below): a name-invalid file
// can be deleted or renamed sight-unseen (its contents were never read); a
// name-invalid FOLDER must be inspected first (blind deletion could destroy
// a foreign subtree this helper never examined); a well-named file whose
// CONTENT does not parse as a ref carries a classified reason (and, for the
// noncanonical damaged-ref shapes, a recoverable hex) worth reading before
// acting.
type SkipKind int

const (
	SkipInvalidName       SkipKind = iota // file whose name failed validation; contents never examined
	SkipInvalidNameFolder                 // folder whose name failed validation; subtree never entered
	SkipContent                           // well-named file whose contents are not a ref
)

// SkippedRef is one path the scan declined to advertise, classified enough
// for both the operator note (skipNote) and the occupancy-aware push
// (component 2, Task 2) to act on without re-deriving anything.
type SkippedRef struct {
	Path   string // full path relative to root, e.g. "refs/heads/Thumbs.db"
	Kind   SkipKind
	Reason string // classified, human-readable; the note/error text body
	Hex    string // 40-hex recovered from a noncanonical candidate, else ""
}

// RefScan is ScanRefs' result: the advertised ref map — exactly what the old
// ListRefs returned — plus every path the walk declined to advertise, in
// walk order, both kinds (name-skips and content-skips) together. Recording
// BOTH kinds in one set, not just content-skips, is deliberate: a create
// over a folder holding only a name-skipped child must still be refused at
// preflight (component 2), and that requires the name-skip to be visible in
// the same structured result the content-skip is.
type RefScan struct {
	Refs    map[string]string // advertised name -> sha (exactly the old ListRefs map)
	Skipped []SkippedRef      // every skipped path, both kinds, walk order
}

// ContentSkips returns the SkipContent subset of Skipped — the set the
// strict fetch-direction survey (Task 2) inspects to decide whether `list`
// must fail: only unparseable CONTENT on a validly-named file could be a
// damaged real ref, so only that class triggers fetch-side strictness (the
// design's "principled line" — name-invalid files can never be refs git
// could hold, so their absence is never silent loss).
func (s *RefScan) ContentSkips() []SkippedRef {
	var out []SkippedRef
	for _, sk := range s.Skipped {
		if sk.Kind == SkipContent {
			out = append(out, sk)
		}
	}
	return out
}

// OccupancyMessage renders the kind-aware refusal body for a skipped path
// (design spec, component 2): what a push-side occupancy collision (a
// create landing on, above, or beneath a skipped path) reports to the
// operator. Kind-aware because a one-size message is actively wrong for two
// of the three kinds — calling a name-skipped FILE "a file whose contents
// are not a ref" is false (its contents were never examined), and inviting
// deletion of a name-skipped FOLDER risks destroying a foreign subtree this
// helper never looked inside.
func OccupancyMessage(root string, s SkippedRef) string {
	full := root + "/" + s.Path
	switch s.Kind {
	case SkipContent:
		return fmt.Sprintf("a file occupies %s and its contents are not a ref (%s); "+
			"delete it first (proton-drive filesystem trash %s, or the web UI)",
			s.Path, s.Reason, full)
	case SkipInvalidNameFolder:
		return fmt.Sprintf("a folder with an invalid name occupies %s; its contents were "+
			"never examined - inspect it before removing anything (the web UI, or a CLI "+
			"listing of %s, will show what's inside before you decide)",
			s.Path, full)
	default: // SkipInvalidName
		return fmt.Sprintf("a file with an invalid ref name occupies %s (contents never "+
			"examined); delete or rename it first (proton-drive filesystem trash %s, "+
			"or the web UI)",
			s.Path, full)
	}
}
