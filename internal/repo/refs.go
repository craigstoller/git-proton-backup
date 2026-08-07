package repo

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// shaRe matches a sha-1 object name and nothing else. A SHA-256 repository's
// 64-hex object names do not match, so such a repo fails closed at the first
// ref write rather than producing a ref file this helper cannot read back —
// correct, but the message has to say so, because "not a 40-hex sha" on its
// own reads like corruption rather than an unsupported repository format.
var shaRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// ListRefs recurses the WHOLE refs/ tree, not just the direct children of
// refs/heads and refs/tags: hierarchical names (refs/heads/feat/x, which git
// accepts and users create constantly), and other namespaces entirely
// (refs/notes/commits, refs/stash, a name some other tool or the web UI left
// behind) are all discovered and, when advertisable, included. This closes
// the Stage 2 boundary the old two-namespace walk documented here, and is
// what lets checkDst (push.go) admit any advertisable name under refs/
// rather than narrowing to exactly two shapes: a name checkDst now accepts
// can actually be advertised back.
//
// COMPONENT VALIDATION (checkComponent) runs on EVERY listed node —
// directories included — BEFORE recursion or any content read. This is not
// mere tidiness: descending into a folder whose name checkComponent has not
// cleared would hand that name straight to a remote t.List() call, and this
// transport's remote-glob behaviour on characters like "{" is UNVERIFIED
// (probe C13 only confirmed LOCAL glob-expansion on upload, never what
// List() does with such a path remotely). checkComponent's refusal is what
// keeps ListRefs from ever probing that: an invalid folder skips its entire
// subtree with a note, and no List() call is ever made naming it.
//
// A node that fails validation — folder or leaf — is SKIPPED WITH A NOTE
// (skipNote, below) to os.Stderr, never fatal: this remote can hold names v2
// itself would never create (a foreign tool, a stray web-UI upload), and one
// such name must not deny the advertisement to every other ref in the repo
// (spec §1). Malformed CONTENT of a well-named, already-advertisable ref is
// a different failure — that ref IS one this remote is supposed to serve, so
// a corrupt sha stays fatal (readRef never coerces it into a best guess).
//
// Cost: one List() subprocess per folder, run serially, plus an in-process
// name check per ref — linear in the size of the ref tree, the same shape as
// the pack-count cost noted beside gitcmd.WritePack.
func ListRefs(t transport.Transport, root string) (map[string]string, error) {
	out := map[string]string{}
	var walk func(rel string) error
	walk = func(rel string) error {
		nodes, err := t.List(root + "/" + rel)
		if err != nil {
			return err
		}
		for _, n := range nodes {
			full := rel + "/" + n.Name
			// Directories included, BEFORE recursion — see the doc comment
			// above: this is what stops an invalid folder name from ever
			// reaching a List() argument.
			if err := checkComponent(n.Name); err != nil {
				skipNote(os.Stderr, root, full, err)
				continue
			}
			if n.IsDir {
				if err := walk(full); err != nil {
					return err
				}
				continue
			}
			if err := advertisableName(full); err != nil {
				// Skip-with-note, NEVER fatal: see the doc comment above.
				skipNote(os.Stderr, root, full, err)
				continue
			}
			sha, err := readRef(t, root+"/"+full)
			if err != nil {
				return err // malformed CONTENT of an advertised ref stays fatal
			}
			out[full] = sha
		}
		return nil
	}
	if err := walk("refs"); err != nil {
		return nil, err
	}
	return out, nil
}

// skipNote writes the standard "skip with a note" line for a name ListRefs
// declined to advertise. Skip notes are never fatal (see ListRefs above) but
// must still tell the operator exactly which remote path was skipped and
// why, so a foreign name is diagnosable rather than silently invisible.
//
// The writer is a parameter rather than a hardcoded os.Stderr specifically so
// the exact note text can be pinned in a focused unit test
// (TestSkipNoteText, repo_test.go) without forking the process or installing
// a pipe to capture the real os.Stderr. ListRefs itself always calls this
// with os.Stderr — the package's convention for advisory warnings that must
// not fail the operation they describe (idxcache.go and sethead.go's
// lock-release warning do the same).
func skipNote(w io.Writer, root, full string, reason error) {
	fmt.Fprintf(w, "git-remote-proton: skipping %s/%s: %v\n", root, full, reason)
}

// readRef downloads the ref file at p and parses its content against the
// EXACT grammar: 40 lowercase hex characters followed by exactly one "\n"
// and nothing else. WriteRef (below) always writes sha+"\n", so only a
// foreign or damaged file can differ from that — no trailing newline, a CRLF
// terminator, a double-LF terminator, or non-hex content — and those are
// precisely what must be fatal here, never coerced into a best guess. This
// used to TrimRight "\r\n" before matching, which silently tolerated all of
// the above; the exact-grammar rewrite (Task 8) closes that gap.
func readRef(t transport.Transport, p string) (string, error) {
	dir, err := os.MkdirTemp("", "gpb-ref-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	if err := t.ReadTo(p, dir); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return "", fmt.Errorf("ref %s could not be read back", p)
	}
	raw, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		return "", err
	}
	if len(raw) != 41 || raw[40] != '\n' || !shaRe.Match(raw[:40]) {
		return "", fmt.Errorf("corrupt ref file %s: content is not exactly 40 lowercase hex "+
			"characters plus a single trailing newline (got %q)", p, string(raw))
	}
	return string(raw[:40]), nil
}

// WriteRef stages under the ref's own LEAF NAME, because `filesystem upload`
// names the uploaded node after the local basename and has no --name flag
// (probe C11). A leaf hostile to a local path is rejected by stagedFile with a
// named reason rather than mangled. It then verifies by read-back, because a
// byte-identical write is silently skipped.
//
// WriteRef does NOT create the ref's parent folder: it uploads into
// <root>/<ref>'s parent as-is, so a hierarchical ref whose parent folder does
// not yet exist on the remote fails here. This is now independent of
// ListRefs (Task 8 made listing recurse the whole tree), and is its own,
// narrower gap: opt-in parent auto-creation is Task 11's job
// (GPB_CREATE_PARENTS), not this function's.
func WriteRef(t transport.Transport, root, ref, sha string, exists bool) (transport.Outcome, error) {
	if !shaRe.MatchString(sha) {
		return transport.Ambiguous, fmt.Errorf("refusing to write non-sha %q to %s "+
			"(ref files are exactly 40 lowercase hex; SHA-256 repositories are not supported)", sha, ref)
	}
	leaf := ref[strings.LastIndex(ref, "/")+1:]
	staged, cleanup, err := stagedFile([]byte(sha+"\n"), leaf)
	if err != nil {
		return transport.Ambiguous, err
	}
	defer cleanup()

	p := root + "/" + ref
	var out transport.Outcome
	if exists {
		out, err = t.UpdateRevision(p, staged)
	} else {
		out, err = t.CreateExclusive(p, staged)
	}
	if err != nil {
		return transport.Ambiguous, err
	}
	if out == transport.Refused && !exists {
		return transport.Refused, nil // concurrent creator
	}

	got, rerr := readRef(t, p)
	if rerr != nil {
		return transport.Ambiguous, rerr
	}
	if got != sha {
		return transport.Ambiguous, fmt.Errorf("ref %s reads back as %s, expected %s", ref, got, sha)
	}
	return transport.Committed, nil
}
