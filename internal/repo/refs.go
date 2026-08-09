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
			// Wrapped with the failing folder's own path: without it, a List
			// failure five levels into the tree surfaced as whatever bare
			// message the transport happened to return, with nothing here
			// naming WHICH folder in the recursion actually failed.
			return fmt.Errorf("listing %s: %w", root+"/"+rel, err)
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
				// Malformed CONTENT stays fatal. Note this fires BEFORE the
				// name is advertised — it is a candidate discovered file, not
				// an already-advertised ref — and the recursion widened its
				// reach to any well-named file anywhere under refs/. Recorded
				// as an open question for the owner in the design doc's v6.5
				// revision entry; unchanged here by instruction.
				return err
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
			"characters plus a single trailing newline (got %q)", p, previewBytes(raw))
	}
	return string(raw[:40]), nil
}

// previewBytes caps a diagnostic preview at 64 bytes. This error used to
// embed the WHOLE file via a bare %q on raw — harmless back when every ref
// file readRef could reach lived under this package's own control (only
// refs/heads and refs/tags, listed non-recursively). ListRefs' recursion
// (Task 8) now reaches readRef for any well-NAMED, advertisable ref anywhere
// under refs/, and a well-named file can still hold arbitrary foreign
// content — a multi-megabyte accidental upload, binary junk — with nothing
// here bounding its size. Truncating keeps the error a status line, not a
// multi-megabyte log entry.
func previewBytes(b []byte) string {
	const max = 64
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "...(truncated)"
}

// WriteRef stages under the ref's own LEAF NAME, because `filesystem upload`
// names the uploaded node after the local basename and has no --name flag
// (probe C11). A leaf hostile to a local path is rejected by stagedFile with a
// named reason rather than mangled. It then verifies by read-back, because a
// byte-identical write is silently skipped.
//
// WriteRef does NOT create the ref's parent folder itself: it uploads into
// <root>/<ref>'s parent as-is, so a hierarchical ref whose parent folder does
// not yet exist on the remote fails here. Callers that need a hierarchical
// ref's parents created first call ensureRefParents (below) before WriteRef —
// Push does this unconditionally in its create/update phase (Task 9a).
//
// This is UNRELATED to Task 11's GPB_CREATE_PARENTS: that env var gates
// creating the REPO ROOT's own missing parent folders (e.g. the containing
// directory of /my-files/GitRemotes/myrepo) before Bootstrap runs, a
// one-time, opt-in, whole-repo concern wired into the `list for-push` arm in
// main.go. ensureRefParents is the opposite: unconditional, and scoped to
// the ref namespace strictly BELOW an already-bootstrapped repo root — an
// earlier version of this comment conflated the two and claimed hierarchical
// ref parents were GPB_CREATE_PARENTS' job, which Task 9a's plan review
// adjudicated otherwise.
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

// ensureRefParents creates every folder strictly between root and ref's own
// leaf — for "refs/heads/feature/deep/x" that is root+"/refs",
// root+"/refs/heads", root+"/refs/heads/feature", and
// root+"/refs/heads/feature/deep". The first two exist from Bootstrap, but
// are walked like any other level rather than special-cased away: partial
// initialisation (a marker present but subdirs missing, the exact state
// TestBootstrapCompletesPartialInitialisation exercises) is a real state,
// not a hypothetical one, so skipping "refs" and "refs/heads" as "always
// there" would be an unproven assumption. EnsureDir is Stat-then-create, so
// walking an already-existing level costs one cheap Stat.
//
// Reverse-D/F detection is TYPED, never error-text matching (round-2 Codex:
// the Fake and the real CLI would otherwise need byte-identical phrasing
// forever, and they do not). On any EnsureDir failure, the failing prefix is
// Stat'd directly: a FILE there means a ref file already occupies the name a
// folder now needs — the named refusal below, quoting both the ref being
// created and the ref blocking it. Anything else (Stat also fails, or the
// prefix is absent, or turns out to be a folder after all — a transient
// contradiction) leaves the original EnsureDir error to stand unmodified;
// this function invents no new diagnosis for a failure mode it cannot
// positively identify as the D/F collision.
func ensureRefParents(t transport.Transport, root, ref string) error {
	comps := strings.Split(ref, "/")
	prefix := root
	for _, c := range comps[:len(comps)-1] {
		prefix = prefix + "/" + c
		if err := t.EnsureDir(prefix); err != nil {
			if n, ok, serr := t.Stat(prefix); serr == nil && ok && !n.IsDir {
				return fmt.Errorf("creating %s requires folder %s, but a ref file occupies "+
					"that name (directory/file conflict; delete it first)", ref, prefix)
			}
			return err
		}
	}
	return nil
}
