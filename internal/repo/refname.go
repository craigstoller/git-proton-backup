package repo

import (
	"fmt"
	"strings"
)

// forbiddenRefChars are the literal characters git-check-ref-format refuses
// anywhere in a ref name: space, tilde, caret, colon, question-mark,
// asterisk, open bracket, and backslash. Control characters (< 0x20) and DEL
// (0x7F) are refused too, but are not literal runes suitable for
// strings.ContainsAny — hasControlByte below scans for those explicitly.
const forbiddenRefChars = " ~^:?*[\\"

// hasControlByte reports whether name contains an ASCII control character
// (< 0x20) or DEL (0x7F), both refused by git-check-ref-format anywhere in a
// ref name. A byte scan, not a rune scan: git's own rule operates on raw
// bytes, and a multi-byte UTF-8 rune never decodes to a byte in this range,
// so the two scans agree on every input.
func hasControlByte(name string) bool {
	for i := 0; i < len(name); i++ {
		if c := name[i]; c < 0x20 || c == 0x7f {
			return true
		}
	}
	return false
}

// CheckRefName implements the documented git-check-ref-format rule set
// in-process, with no subprocess. The NORMATIVE BAR IS GIT'S RULE SET, NOT
// THIS FUNCTION (spec, round 3: a prose list of rules omitted leading-dot
// components entirely) — TestCheckRefNameParityWithGit in refname_test.go is
// what keeps this honest, by running every fixture through the real
// `git check-ref-format` and requiring an identical verdict. Extend fixtures
// before logic.
//
// KEY SPLIT this function does NOT make: braces ("{", "}") are valid to git
// (confirmed live) and are accepted here. Refusing them is a STAGEABILITY
// concern — cli-drive@0.7.0 still glob-expands "{" locally (probe C13),
// which is a property of THIS transport, not of git's ref-name grammar — and
// belongs to advertisableName below, not here.
func CheckRefName(name string) error {
	if name == "" {
		return fmt.Errorf("invalid ref name: empty")
	}
	if name == "@" {
		return fmt.Errorf("invalid ref name %q: a bare \"@\" is refused", name)
	}
	if !strings.Contains(name, "/") {
		return fmt.Errorf("invalid ref name %q: must contain at least one \"/\" "+
			"(git's default check-ref-format rules refuse one-level names)", name)
	}
	if strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") {
		return fmt.Errorf("invalid ref name %q: cannot begin or end with \"/\"", name)
	}
	if strings.Contains(name, "//") {
		return fmt.Errorf("invalid ref name %q: cannot contain consecutive \"/\"", name)
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("invalid ref name %q: cannot contain \"..\"", name)
	}
	if strings.HasSuffix(name, ".") {
		return fmt.Errorf("invalid ref name %q: cannot end with \".\"", name)
	}
	if strings.Contains(name, "@{") {
		return fmt.Errorf("invalid ref name %q: cannot contain \"@{\"", name)
	}
	if strings.ContainsAny(name, forbiddenRefChars) {
		return fmt.Errorf("invalid ref name %q: contains a forbidden character (one of %q)",
			name, forbiddenRefChars)
	}
	if hasControlByte(name) {
		return fmt.Errorf("invalid ref name %q: contains a control character", name)
	}
	// Per-component rules that only make sense once the name is known to be
	// slash-separated at all: a component beginning with "." (the round-3
	// catch — refs/heads/.hidden must be refused even though the WHOLE name
	// does not begin with a dot) and a component ending in ".lock" (git
	// reserves that suffix for its own lockfiles).
	for _, comp := range strings.Split(name, "/") {
		if strings.HasPrefix(comp, ".") {
			return fmt.Errorf("invalid ref name %q: component %q begins with \".\"", name, comp)
		}
		if strings.HasSuffix(comp, ".lock") {
			return fmt.Errorf("invalid ref name %q: component %q ends with \".lock\"", name, comp)
		}
	}
	return nil
}

// checkComponent validates ONE path component — the caller has already
// split on "/", so no slash-crossing rule (min-one-slash, no leading/
// trailing slash, no "//") applies here. It combines checkStageableLeaf
// (marker.go: empty, ".", "..", any of "{}/\\", and Windows-reserved device
// names — this transport's own local-staging concern) with git's
// PER-COMPONENT check-ref-format rules: a component cannot begin with ".",
// cannot end with ".lock", cannot contain "..", cannot contain "@{", and
// cannot contain the forbidden-character set.
//
// Deliberately excluded: the "cannot end with a dot" rule. That is a
// WHOLE-NAME rule in real git — it fires only against the very last
// character of the complete ref name, not against every slash-separated
// component (confirmed against real git as part of the parity test) — so a
// mid-path component ending in "." is not, on its own, invalid, and
// checkComponent must not reject what CheckRefName does not.
//
// Task 8's walk applies this to every listed node — folders included —
// before recursing into it.
func checkComponent(name string) error {
	if err := checkStageableLeaf(name); err != nil {
		return err
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("invalid ref path component %q: begins with \".\"", name)
	}
	if strings.HasSuffix(name, ".lock") {
		return fmt.Errorf("invalid ref path component %q: ends with \".lock\"", name)
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("invalid ref path component %q: contains \"..\"", name)
	}
	if strings.Contains(name, "@{") {
		return fmt.Errorf("invalid ref path component %q: contains \"@{\"", name)
	}
	if strings.ContainsAny(name, forbiddenRefChars) {
		return fmt.Errorf("invalid ref path component %q: contains a forbidden character (one of %q)",
			name, forbiddenRefChars)
	}
	if hasControlByte(name) {
		return fmt.Errorf("invalid ref path component %q: contains a control character", name)
	}
	return nil
}

// advertisableName is CheckRefName's git-validity check PLUS a stageability
// check per component. A name can be perfectly valid to git yet unusable by
// this transport, because every ref this helper creates or updates is
// staged as a local file first (stagedFile, marker.go), and a component that
// cannot survive that round trip must be refused before it is ever
// advertised or accepted at the push boundary — Task 8's read boundary
// (files) and Task 9a's push validation both call this, not CheckRefName
// directly.
//
// The load-bearing example is braces: git accepts "refs/heads/a{b}" (so does
// CheckRefName), but checkComponent — via checkStageableLeaf — refuses any
// component containing "{" or "}", because cli-drive@0.7.0 still glob-
// expands "{" locally (probe C13). advertisableName is where that refusal
// actually happens; CheckRefName alone would let it through.
func advertisableName(name string) error {
	if err := CheckRefName(name); err != nil {
		return err
	}
	for _, comp := range strings.Split(name, "/") {
		if err := checkComponent(comp); err != nil {
			return fmt.Errorf("ref %q is not advertisable: %w", name, err)
		}
	}
	return nil
}
