package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// HeadName is the remote file holding the symref.
const HeadName = "HEAD"

const headPrefix = "ref: "

// DeriveHEAD picks the default branch deterministically, so the result never
// depends on the order refs happened to arrive in.
//
// candidates are full ref names; only refs/heads/* are eligible. One eligible
// candidate wins outright. Among several, the client's own HEAD wins if it is
// present, otherwise the lexicographically first. No eligible candidate means
// no HEAD is written and the repo stays headless, which is a defined state.
func DeriveHEAD(candidates []string, clientHEAD string) (string, bool) {
	var branches []string
	for _, c := range candidates {
		if strings.HasPrefix(c, "refs/heads/") {
			branches = append(branches, c)
		}
	}
	if len(branches) == 0 {
		return "", false
	}
	if len(branches) == 1 {
		return branches[0], true
	}
	for _, b := range branches {
		if b == clientHEAD {
			return b, true
		}
	}
	sort.Strings(branches)
	return branches[0], true
}

// WriteHEAD writes the symref and verifies it by read-back.
//
// It deliberately does NOT go through WriteRef: that function validates its
// payload against ^[0-9a-f]{40}$ and would refuse a symref outright. The
// read-back is still required for the same reason it is everywhere else — the
// CLI silently skips a byte-identical rewrite, so Committed alone does not
// prove our bytes landed.
func WriteHEAD(t transport.Transport, root, branch string) (transport.Outcome, error) {
	if !strings.HasPrefix(branch, "refs/heads/") {
		return transport.Ambiguous, fmt.Errorf("refusing to point HEAD at %q: not a branch", branch)
	}
	staged, cleanup, err := stagedFile([]byte(headPrefix+branch+"\n"), HeadName)
	if err != nil {
		return transport.Ambiguous, err
	}
	defer cleanup()

	p := root + "/" + HeadName
	out, err := t.CreateExclusive(p, staged)
	if err != nil {
		return transport.Ambiguous, err
	}
	// A value switch with an explicit arm per constant, not a boolean switch
	// with `if out == ...` cases: the boolean form has no way to express
	// "anything else", so an unrecognised Outcome fell through into the
	// read-back below and, had it happened to match, would have been reported
	// as Committed on the strength of a value nobody recognised. Zero risk
	// today, since Outcome is a closed three-constant set in this module, but
	// AcquireLock, Bootstrap, publishPack and publishIdx are all guarded this
	// way; leaving WriteHEAD as the one exception would make the pattern
	// advisory rather than the rule.
	switch out {
	case transport.Committed:
		// Falls out of the switch into the read-back verification below.
	case transport.Refused:
		// Someone wrote HEAD between our check and our write. We never
		// overwrite an existing HEAD, so adopt theirs.
		return transport.Refused, nil
	case transport.Ambiguous:
		return transport.Ambiguous, fmt.Errorf("HEAD write outcome ambiguous for %s; re-run to reconcile", p)
	default:
		return transport.Ambiguous, fmt.Errorf("HEAD write for %s returned an unrecognised outcome %s; "+
			"refusing to guess whether it landed", p, out)
	}
	got, ok, err := ReadHEAD(t, root)
	if err != nil {
		return transport.Ambiguous, err
	}
	if !ok || got != branch {
		return transport.Ambiguous, fmt.Errorf("HEAD reads back as %q, expected %q", got, branch)
	}
	return transport.Committed, nil
}

// UpdateHEAD points HEAD at branch, OVERWRITING any existing symref — the
// write half of the --set-head operation, and deliberately not WriteHEAD:
// that function backfills and never overwrites, a contract its tests pin.
// The caller MUST hold the repo lock. Verified by read-back like every
// write (the CLI silently skips byte-identical rewrites).
//
// UpdateHEAD does NOT short-circuit a rewrite that matches the existing
// target itself: callers must check first, because the CLI comes back
// Refused for a byte-identical write and UpdateHEAD reports that as an
// error, not success. SetHead's read-first short-circuit is the reference
// caller.
func UpdateHEAD(t transport.Transport, root, branch string) (transport.Outcome, error) {
	if !strings.HasPrefix(branch, "refs/heads/") {
		return transport.Ambiguous, fmt.Errorf("refusing to point HEAD at %q: not a branch", branch)
	}
	staged, cleanup, err := stagedFile([]byte(headPrefix+branch+"\n"), HeadName)
	if err != nil {
		return transport.Ambiguous, err
	}
	defer cleanup()

	p := root + "/" + HeadName
	_, exists, err := t.Stat(p)
	if err != nil {
		return transport.Ambiguous, fmt.Errorf("stat %s: %w", p, err)
	}
	var out transport.Outcome
	if exists {
		out, err = t.UpdateRevision(p, staged)
	} else {
		out, err = t.CreateExclusive(p, staged)
	}
	if err != nil {
		return transport.Ambiguous, err
	}
	switch out {
	case transport.Committed:
		// Falls through to the read-back below.
	case transport.Refused:
		// Under the lock no v2 writer can race us, so a refusal here is a
		// non-v2 actor. Do not adopt: the user asked for THIS branch.
		return transport.Ambiguous, fmt.Errorf("HEAD write for %s was refused mid-operation; "+
			"re-run to reconcile", p)
	case transport.Ambiguous:
		return transport.Ambiguous, fmt.Errorf("HEAD update outcome ambiguous for %s; "+
			"re-run to reconcile", p)
	default:
		return transport.Ambiguous, fmt.Errorf("HEAD update for %s returned an unrecognised "+
			"outcome %s; refusing to guess whether it landed", p, out)
	}
	got, ok, err := ReadHEAD(t, root)
	if err != nil {
		return transport.Ambiguous, err
	}
	if !ok || got != branch {
		return transport.Ambiguous, fmt.Errorf("HEAD reads back as %q, expected %q", got, branch)
	}
	return transport.Committed, nil
}

// ReadHEAD returns the branch HEAD points at. Absence is (_, false, nil);
// content that is not a branch symref is fatal, never coerced — the same rule
// the ref-file grammar follows.
func ReadHEAD(t transport.Transport, root string) (string, bool, error) {
	p := root + "/" + HeadName
	if _, ok, err := t.Stat(p); err != nil {
		return "", false, fmt.Errorf("stat %s: %w", p, err)
	} else if !ok {
		return "", false, nil
	}
	dir, err := os.MkdirTemp("", "gpb-head-*")
	if err != nil {
		return "", false, err
	}
	defer os.RemoveAll(dir)
	if err := t.ReadTo(p, dir); err != nil {
		return "", false, fmt.Errorf("cannot read %s: %w", p, err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, HeadName))
	if err != nil {
		return "", false, fmt.Errorf("cannot read back %s: %w", p, err)
	}
	s := strings.TrimRight(string(raw), "\r\n")
	if !strings.HasPrefix(s, headPrefix) {
		return "", false, fmt.Errorf("corrupt %s: %q is not a symref", p, s)
	}
	branch := strings.TrimSpace(strings.TrimPrefix(s, headPrefix))
	if !strings.HasPrefix(branch, "refs/heads/") {
		return "", false, fmt.Errorf("corrupt %s: points at %q, which is not a branch", p, branch)
	}
	return branch, true, nil
}
