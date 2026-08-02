package repo

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
		// Confirmation is per member: a .pack is named by its own content
		// checksum, a .idx borrows that name, so they cannot be checked the
		// same way (design v6.2).
		packDst := root + "/packs/" + filepathBase(packPath)
		idxDst := root + "/packs/" + filepathBase(idxPath)

		if err := publishPack(t, packDst, packPath); err != nil {
			return fail(err.Error())
		}
		if err := publishIdx(t, idxDst, idxPath, packPath); err != nil {
			return fail(err.Error())
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

// confirmPresent is the Committed path: the bytes are the ones we just sent,
// so presence is all that remains to check.
func confirmPresent(t transport.Transport, dst, what string) error {
	if _, ok, err := t.Stat(dst); err != nil {
		return fmt.Errorf("%s presence check failed: %w", what, err)
	} else if !ok {
		return fmt.Errorf("uploaded %s is not readable back: %s", what, dst)
	}
	return nil
}

// publishPack uploads a pack and confirms it. A Refused upload is NOT success
// on the strength of the refusal: Stat proves a node exists at that path, not
// what is in it, so a corrupt or truncated remote pack would otherwise be
// accepted and a ref published that no client can ever fetch.
func publishPack(t transport.Transport, dst, local string) error {
	out, err := t.CreateExclusive(dst, local)
	if err != nil {
		return fmt.Errorf("pack upload failed: %w", err)
	}
	switch out {
	case transport.Ambiguous:
		// Reconciliation happens on the retry: the next run's CreateExclusive
		// returns Refused and takes the verification path below. Reporting
		// failure now is the fail-closed answer for THIS push.
		return fmt.Errorf("pack upload outcome ambiguous; re-run to reconcile")
	case transport.Committed:
		return confirmPresent(t, dst, "pack")
	}

	dir, err := os.MkdirTemp("", "gpb-verify-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := t.ReadTo(dst, dir); err != nil {
		return fmt.Errorf("cannot read back the existing pack %s: %w", dst, err)
	}
	remote := filepath.Join(dir, filepathBase(dst))

	same, err := filesEqual(remote, local)
	if err != nil {
		return fmt.Errorf("cannot compare the existing pack %s: %w", dst, err)
	}
	if !same {
		return fmt.Errorf("remote pack %s has the same name but different bytes; "+
			"it is corrupt or was written by something else, and an immutable "+
			"object is never overwritten", dst)
	}
	return checkPackChecksum(remote, filepathBase(dst))
}

// publishIdx uploads an index and confirms it. A Refused index is deliberately
// NOT byte-compared: its name is borrowed from its pack, more than one valid
// index can carry that name, and the v2 pin governs what WE write rather than
// what another writer left there. Demanding byte equality would make a
// legitimate remote index permanently fatal, since it cannot be replaced.
// What matters is that it indexes the pack correctly.
//
// Verification runs against the LOCAL pack, not a second download: publishPack
// has already established the remote pack is byte-identical to it, and packs
// can be gigabytes.
func publishIdx(t transport.Transport, dst, local, localPack string) error {
	out, err := t.CreateExclusive(dst, local)
	if err != nil {
		return fmt.Errorf("index upload failed: %w", err)
	}
	switch out {
	case transport.Ambiguous:
		return fmt.Errorf("index upload outcome ambiguous; re-run to reconcile")
	case transport.Committed:
		return confirmPresent(t, dst, "index")
	}

	dir, err := os.MkdirTemp("", "gpb-verify-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := t.ReadTo(dst, dir); err != nil {
		return fmt.Errorf("cannot read back the existing index %s: %w", dst, err)
	}
	// index-pack --verify needs the pair adjacent under one stem.
	packSide := filepath.Join(dir, filepathBase(localPack))
	if err := linkOrCopy(localPack, packSide); err != nil {
		return err
	}
	if err := gitcmd.IndexPackVerify(packSide); err != nil {
		return fmt.Errorf("remote index %s does not verify against its pack: %w", dst, err)
	}
	return nil
}

// filesEqual streams both files. Packs can be gigabytes; never read one whole.
func filesEqual(a, b string) (bool, error) {
	fa, err := os.Open(a)
	if err != nil {
		return false, err
	}
	defer fa.Close()
	fb, err := os.Open(b)
	if err != nil {
		return false, err
	}
	defer fb.Close()

	sa, err := fa.Stat()
	if err != nil {
		return false, err
	}
	sb, err := fb.Stat()
	if err != nil {
		return false, err
	}
	if sa.Size() != sb.Size() {
		return false, nil
	}
	ba, bb := make([]byte, 64*1024), make([]byte, 64*1024)
	for {
		na, ea := io.ReadFull(fa, ba)
		nb, eb := io.ReadFull(fb, bb)
		if na != nb || !bytes.Equal(ba[:na], bb[:nb]) {
			return false, nil
		}
		if ea != nil || eb != nil {
			done := func(e error) bool { return e == io.EOF || e == io.ErrUnexpectedEOF }
			if done(ea) && done(eb) {
				return true, nil
			}
			if ea != nil && !done(ea) {
				return false, ea
			}
			return false, eb
		}
	}
}

// checkPackChecksum recomputes the pack's content hash and compares it to the
// basename. git hashes every byte EXCEPT the trailing 20, and stores the
// result in those 20 — so this proves the body is what the name claims. It
// cannot detect a change confined to the trailer itself; the byte comparison
// in publishPack and index-pack --verify are what cover that.
//
// SHA-1 only: this design supports SHA-1 repositories, and refs.go rejects
// anything that is not 40 hex.
func checkPackChecksum(path, base string) error {
	const trailer = 20
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if st.Size() < trailer {
		return fmt.Errorf("remote pack %s is truncated", base)
	}
	h := sha1.New()
	if _, err := io.CopyN(h, f, st.Size()-trailer); err != nil {
		return err
	}
	name := strings.TrimSuffix(strings.TrimPrefix(base, "pack-"), ".pack")
	if got := hex.EncodeToString(h.Sum(nil)); got != name {
		return fmt.Errorf("remote pack %s recomputes to %s; the name is the content "+
			"checksum, so this file is not what its name claims", base, got)
	}
	return nil
}

// linkOrCopy places src at dst, preferring a hard link so a large pack is not
// duplicated on disk just to sit beside an index for verification.
func linkOrCopy(src, dst string) error {
	if err := os.Link(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
