package repo

import (
	"fmt"
	"os"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/gitcmd"
	"github.com/craigstoller/git-proton-backup/internal/protocol"
	"github.com/craigstoller/git-proton-backup/internal/transport"
)

type Result struct {
	Ref string
	OK  bool
	Err string
}

// Push applies each ref update in ups independently. Multi-ref batches are
// NOT atomic: every update gets its own Result, so partial success (some refs
// updated, others rejected) is expected and correct, never collapsed into a
// single batch-wide outcome.
func Push(t transport.Transport, root, gitDir string,
	ups []protocol.RefUpdate, remote map[string]string) []Result {

	results := make([]Result, 0, len(ups))
	for _, u := range ups {
		results = append(results, pushOne(t, root, gitDir, u, remote))
	}
	return results
}

// pushOne applies a single ref update. Ordering is pack -> idx -> confirm
// both -> ref: a ref must never point at objects that are not fully
// uploaded, because a ref whose index is missing is not fetch-discoverable.
func pushOne(t transport.Transport, root, gitDir string,
	u protocol.RefUpdate, remote map[string]string) Result {

	// fail flattens msg to a single line. Results are rendered by the caller
	// as "error <ref> <reason>\n", one status line per update, so an embedded
	// newline in a reason — and git's own diagnostics are routinely multi-line
	// — would split one status line into two and desynchronise the protocol.
	// This is the single funnel every failure passes through, so it is the
	// right place to guarantee it.
	fail := func(msg string) Result { return Result{Ref: u.Dst, Err: oneLine(msg)} }

	// --- destination namespace ----------------------------------------------
	// FIRST, before resolve and before any packing. Rejecting early is what
	// stops a doomed push from costing a pack upload to the user's paid Drive
	// (and leaving an orphan behind — Stage 2 has no GC). Without this guard
	// refs/heads/feat/x was invisible to the non-recursive ListRefs, so
	// exists came back false, the ancestry check was skipped, a full pack was
	// built and uploaded, and only then did WriteRef fail on a
	// refs/heads/feat folder nobody had created. It also covers the delete
	// path, which otherwise reported OK: true for any destination ListRefs
	// cannot see — including every pseudoref.
	if err := checkDst(u.Dst); err != nil {
		return fail(err.Error())
	}

	oldSha, exists := remote[u.Dst]

	// --- delete -------------------------------------------------------------
	if u.Src == "" {
		if !exists {
			return Result{Ref: u.Dst, OK: true} // already absent
		}
		out, err := t.Trash(root + "/" + u.Dst)
		if err != nil {
			return fail(fmt.Sprintf("delete failed: %v", err))
		}
		if out != transport.Committed {
			// err is nil here, so a bare "%v" would print the useless
			// "delete failed: <nil>". Report the outcome itself instead.
			return fail(fmt.Sprintf("delete failed: outcome %s", out))
		}
		return Result{Ref: u.Dst, OK: true}
	}

	newSha, err := resolve(gitDir, u.Src)
	if err != nil {
		return fail(err.Error())
	}

	// --- branch targets must be commits ------------------------------------
	if isBranch(u.Dst) {
		typ, err := gitcmd.ObjectType(gitDir, newSha)
		if err != nil {
			return fail("cannot determine object type")
		}
		if typ != "commit" {
			return fail(fmt.Sprintf("branch cannot point at a %s", typ))
		}
	}

	// --- ancestry ----------------------------------------------------------
	if exists && !u.Force {
		if !gitcmd.HasObject(gitDir, oldSha) {
			return fail("fetch first")
		}
		// IsAncestor distinguishes "not an ancestor" (exit 1) from a tooling
		// failure. Discarding the error would report a broken git as a
		// confident non-fast-forward rejection.
		ok, err := gitcmd.IsAncestor(gitDir, oldSha, newSha)
		if err != nil {
			return fail("cannot determine ancestry: " + err.Error())
		}
		if !ok {
			return fail("non-fast-forward")
		}
	}

	// --- pack --------------------------------------------------------------
	tmp, err := os.MkdirTemp("", "gpb-pack-*")
	if err != nil {
		return fail(err.Error())
	}
	defer os.RemoveAll(tmp)

	// haves is built from the ref list as it stood when the batch started, and
	// is NOT updated between refs in a multi-ref batch: ref B re-packs
	// everything ref A just uploaded. That is a known cost, recorded here so
	// it is not mistaken for an oversight and "fixed" into a correctness bug.
	// The design's rule for objects that cannot be confirmed on the remote is
	// that they are simply not excluded — "larger pack, never wrong". Feeding
	// B a have that A only just uploaded would mean trusting an upload this
	// process has not read back, and a wrong have produces a pack missing its
	// delta bases, which is unrecoverable. Do not restructure without an
	// answer to that.
	haves := make([]string, 0, len(remote))
	for _, s := range remote {
		if gitcmd.HasObject(gitDir, s) {
			haves = append(haves, s)
		}
	}
	packPath, idxPath, err := gitcmd.WritePack(gitDir, newSha, haves, tmp)
	if err != nil {
		return fail("pack failed: " + err.Error())
	}

	if packPath != "" {
		// Pack, then index, then CONFIRM BOTH before publishing the ref.
		for _, f := range []string{packPath, idxPath} {
			dst := root + "/packs/" + filepathBase(f)
			out, err := t.CreateExclusive(dst, f)
			if err != nil {
				return fail("upload failed: " + err.Error())
			}
			if out == transport.Ambiguous {
				return fail("upload outcome ambiguous; re-run to reconcile")
			}
			// Refused here means identical content already exists (pack/idx
			// filenames are content-addressed) — that is success after
			// presence verification, not an error.
			if _, ok, _ := t.Stat(dst); !ok {
				return fail("uploaded object is not readable back: " + dst)
			}
		}
	}

	// --- publish ------------------------------------------------------------
	out, err := WriteRef(t, root, u.Dst, newSha, exists)
	if err != nil || out == transport.Ambiguous {
		return fail(fmt.Sprintf("ref publish failed: %v", err))
	}
	if out == transport.Refused {
		// WriteRef (refs.go) returns (Refused, nil) — no error — specifically
		// when this is a create (exists == false) and a concurrent creator
		// won the race; it deliberately did not overwrite. That is not the
		// same as success: our newSha was never published, so reporting
		// OK: true here would make git update its remote-tracking ref to a
		// sha that disagrees with what is actually on the remote, with
		// nothing to signal the mismatch. It must be reported as a failure.
		return fail("ref changed concurrently; refusing to overwrite")
	}
	return Result{Ref: u.Dst, OK: true}
}

func isBranch(ref string) bool { return strings.HasPrefix(ref, "refs/heads/") }

// checkDst is the design's "Pseudorefs and unsupported destinations | Explicit
// rejection with a named reason" row. Stage 2 serves exactly two shapes:
// refs/heads/<name> and refs/tags/<name>, with <name> a single non-empty path
// component.
//
// The limitation is real, not conservatism for its own sake. refs.go documents
// that ListRefs lists the direct children of each namespace NON-RECURSIVELY
// and skips directories, so anything deeper is invisible to the advertisement,
// and WriteRef has the mirror-image gap — it would upload into a parent folder
// this package never creates. Anything outside refs/heads and refs/tags is
// worse still: it would be written, reported ok, and then never advertised
// again, so the next push of it fails claiming a concurrent change that never
// happened. Recursive listing and the wider ref namespace belong to Stage 3,
// which owns clone/fetch.
func checkDst(dst string) error {
	parts := strings.Split(dst, "/")
	if len(parts) == 3 && parts[0] == "refs" && parts[2] != "" &&
		(parts[1] == "heads" || parts[1] == "tags") {
		return nil
	}
	return fmt.Errorf("unsupported destination %q: this remote helper serves only "+
		"refs/heads/<name> and refs/tags/<name> with a single name component; "+
		"hierarchical refs, pseudorefs and other namespaces are not supported "+
		"(the remote ref listing is non-recursive, so such a ref could be written "+
		"but never advertised back)", dst)
}

// oneLine collapses every run of whitespace — newlines included — into a
// single space. Push results become "error <ref> <reason>" status lines, one
// per line, so a reason carrying git's multi-line diagnostics would split into
// two lines and desynchronise git's read of the batch.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// resolve turns src (a sha or a ref name/other rev-parse input) into a sha.
// A value already shaped like a sha is returned as-is; anything else goes
// through `git rev-parse` in gitDir.
//
// RevParse deliberately returns three values so callers can interpret them;
// discarding the output and the error, and reporting only "cannot resolve %s",
// threw away the entire reason — git's own message says whether the ref is
// unknown, ambiguous, or the repo unreadable, and the operator sees none of
// that otherwise. fail() flattens the result to one line for the status line.
func resolve(gitDir, src string) (string, error) {
	if shaRe.MatchString(src) {
		return src, nil
	}
	out, code, err := gitcmd.RevParse(gitDir, src)
	if code != 0 {
		if err != nil {
			return "", fmt.Errorf("cannot resolve %s: %s: %w", src, out, err)
		}
		return "", fmt.Errorf("cannot resolve %s: %s (git exited %d)", src, out, code)
	}
	return out, nil
}

// filepathBase returns the final path component of p, accepting both '/' and
// '\' as separators since gitcmd.WritePack's paths are built with
// filepath.Join and so use the host's native separator.
func filepathBase(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[i+1:]
		}
	}
	return p
}
