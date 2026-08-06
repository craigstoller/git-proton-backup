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

	// Complete a missing HEAD. This is the same rule Bootstrap applies to a
	// missing refs/ or packs/ — a partial initialisation is completed, not
	// rejected — and it is why a repo pushed before this shipped is still
	// clonable. An EXISTING HEAD is never touched: changing the default
	// branch stays an explicit operation, out of scope for v2.
	ensureHEAD(t, root, gitDir, ups, remote, results)
	return results
}

// ensureHEAD writes HEAD when the remote has none. Failure is reported on
// stderr and does not fail the push: the refs and objects are already
// published and correct, and turning a successful push into an error because
// a convenience symref could not be written would be the wrong trade.
func ensureHEAD(t transport.Transport, root, gitDir string,
	ups []protocol.RefUpdate, remote map[string]string, results []Result) {

	if _, ok, err := ReadHEAD(t, root); err != nil {
		fmt.Fprintf(os.Stderr, "git-remote-proton: cannot read remote HEAD: %v\n", err)
		return
	} else if ok {
		return // never rewrite
	}

	// Candidates are every branch on the remote AFTER this push: those that
	// were already there, plus those this push actually published. Taking
	// only the published set would point HEAD at whatever happened to be in
	// today's batch.
	seen := map[string]bool{}
	for ref := range remote {
		seen[ref] = true
	}
	okNow := map[string]bool{}
	for _, r := range results {
		if r.OK {
			okNow[r.Ref] = true
		}
	}
	for _, u := range ups {
		if u.Src == "" && okNow[u.Dst] {
			delete(seen, u.Dst) // a successful delete removes a candidate
			continue
		}
		if okNow[u.Dst] {
			seen[u.Dst] = true
		}
	}
	candidates := make([]string, 0, len(seen))
	for ref := range seen {
		candidates = append(candidates, ref)
	}

	clientHEAD, err := gitcmd.SymbolicRef(gitDir, "HEAD")
	if err != nil {
		// A detached HEAD is ("", nil), so this is a real failure. It only
		// costs us the tie-break, so warn and continue.
		fmt.Fprintf(os.Stderr, "git-remote-proton: cannot read local HEAD: %v\n", err)
	}
	branch, ok := DeriveHEAD(candidates, clientHEAD)
	if !ok {
		return // headless is a defined state
	}
	if out, err := WriteHEAD(t, root, branch); err != nil {
		fmt.Fprintf(os.Stderr, "git-remote-proton: could not write remote HEAD (%s): %v\n", out, err)
	}
	// A (Refused, nil) outcome means a concurrent writer's HEAD is now in
	// place and was correctly not overwritten — that is success-by-adoption,
	// not a failure, so it falls through here with nothing to report. If the
	// concurrent HEAD is corrupt, the next ReadHEAD (next push or next
	// advertisement) reports it loudly; this path does not need to.
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
		// The design's ref-transition table is normative here: "Delete
		// (`push :dst`) | Trash; refuse to delete the branch HEAD points at".
		//
		// It is not politeness. v2 never rewrites an existing HEAD (ensureHEAD
		// returns early the moment one is present), so a delete that leaves
		// HEAD naming a ref that no longer exists is PERMANENT: the remote
		// goes on advertising a symref to nothing, and a clone fetches the
		// objects and checks out nothing. Ordinary commands reach it — push
		// main (HEAD is backfilled to it), push dev, delete main. The plain
		// `list` arm in cmd/git-remote-proton has the matching guard, which is
		// what rescues a remote already in that state; this one is what stops
		// any new remote from entering it.
		//
		// An unreadable HEAD fails the delete closed rather than proceeding.
		// ReadHEAD treats anything that is not a branch symref as fatal and
		// never coerces it, so "cannot read" genuinely means we do not know
		// what HEAD names — and the ref about to be trashed may be exactly the
		// one this rule protects. This is per-ref, so other updates in the
		// same batch are unaffected.
		head, hasHead, err := ReadHEAD(t, root)
		if err != nil {
			return fail(fmt.Sprintf("refusing to delete %s: remote HEAD could not be read, "+
				"so it is unknown whether HEAD points at this branch: %v", u.Dst, err))
		}
		if hasHead && head == u.Dst {
			return fail(fmt.Sprintf("refusing to delete the branch HEAD points at (%s); "+
				"change the default branch first (git-remote-proton --set-head <url> <branch>)", u.Dst))
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
	case transport.Refused:
		// Falls out of the switch into the verification path below — this is
		// the safety-critical branch the whole task exists for, so it is
		// named explicitly rather than left as an implicit fall-through.
	default:
		return fmt.Errorf("pack upload returned an unrecognised outcome %s; "+
			"refusing to guess whether it is safe to publish", out)
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
	return checkIdenticalPackChecksum(remote, dst)
}

// checkIdenticalPackChecksum is the last step of publishPack's Refused path,
// and it is reached ONLY after filesEqual has proven the remote pack
// byte-identical to the local one. That fact changes the diagnosis
// completely, which is why this is a separate function with its own messages
// rather than a shared checksum check.
//
// If these bytes do not hash to the name they are stored under, then BOTH
// copies fail to — the remote one is an exact copy of what is on this
// machine's disk. So the fault is that our own local git mis-named its own
// output; the remote file is not corrupt and there is nothing to repair
// there. The generic message ("this file is not what its name claims",
// naming the remote path) pointed the operator at their live paid account to
// remediate a file that is byte-for-byte what they already hold locally.
// That is the same class as the fail-closed-path-with-a-misdirecting-
// diagnosis finding from fix round 1, one step further down this function.
func checkIdenticalPackChecksum(local, dst string) error {
	const bothIdentical = "the remote pack %s is byte-identical to the pack git just built " +
		"locally, so both copies are the same bytes and this is a LOCAL problem: "
	got, err := packContentChecksum(local)
	if err != nil {
		return fmt.Errorf(bothIdentical+"its content checksum could not be recomputed (%v). "+
			"Do not trash or replace the remote copy — it is not the faulty artifact", dst, err)
	}
	if want := packNameChecksum(filepathBase(dst)); got != want {
		return fmt.Errorf(bothIdentical+"both recompute to %s rather than the %s their shared "+
			"name claims, which means the local git that produced this pack mis-named its own "+
			"output. Do not trash or replace the remote copy — it is not the faulty artifact",
			dst, got, want)
	}
	return nil
}

// publishIdx uploads an index and confirms it. A Refused index is deliberately
// NOT byte-compared: its name is borrowed from its pack, more than one valid
// index can carry that name, and the v2 pin governs what WE write rather than
// what another writer left there. Demanding byte equality would make a
// legitimate remote index permanently fatal, since it cannot be replaced.
// What matters is that it indexes the pack correctly.
//
// Verification runs against the LOCAL pack, not a second download. This is
// safe on either path publishPack can have taken: on Committed, we uploaded
// those exact bytes ourselves — confirmPresent only re-checks presence, but
// the content was never in question because we just wrote it; on Refused,
// publishPack has explicitly byte-compared the remote pack against the local
// one and found them identical. Either way the remote pack is known to equal
// the local pack, so downloading it a second time here to verify against
// itself buys nothing — and packs can be gigabytes.
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
	case transport.Refused:
		// Falls out of the switch into the verification path below — the
		// same explicit-branch treatment as publishPack above.
	default:
		return fmt.Errorf("index upload returned an unrecognised outcome %s; "+
			"refusing to guess whether it is safe to publish", out)
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
	return readersEqual(fa, fb)
}

// isCleanEOF reports whether e is io.ReadFull's normal end-of-stream signal
// (io.EOF with nothing read this call, or io.ErrUnexpectedEOF with a short
// final read) rather than a genuine I/O failure.
func isCleanEOF(e error) bool { return e == io.EOF || e == io.ErrUnexpectedEOF }

// readersEqual streams ra and rb 64KiB at a time and reports whether their
// content is identical. Split out of filesEqual so the read loop's error
// handling can be driven directly with a synthetic io.Reader in tests: a
// genuine (non-EOF) I/O error is not reliably reproducible by opening real
// files portably (Windows and POSIX fail mid-read very differently, if at
// all, for the same fault), whereas a fake reader can return a canned error
// deterministically on any platform. This is the exact loop filesEqual uses.
//
// Error checks run BEFORE the length/content comparison — this was fix-round
// 1's finding: a short read carrying a genuine (non-EOF) error must surface
// as that error, never as a false "the bytes differ". publishPack turns a
// "not equal" result into a permanent, unrecoverable remote-corruption
// diagnosis ("an immutable object is never overwritten"); a transient local
// read failure must never produce that diagnosis. The clean-EOF case is
// unchanged: equal-size inputs reach EOF/ErrUnexpectedEOF on the same
// iteration (io.ReadFull requests the same 64KiB from both, and the sizes
// were already confirmed equal by the caller), a zero-byte pair returns true
// on the very first pass, and the loop only ever exits by one of the three
// explicit returns below, never open-ended.
func readersEqual(ra, rb io.Reader) (bool, error) {
	ba, bb := make([]byte, 64*1024), make([]byte, 64*1024)
	for {
		na, ea := io.ReadFull(ra, ba)
		nb, eb := io.ReadFull(rb, bb)

		if ea != nil && !isCleanEOF(ea) {
			return false, ea
		}
		if eb != nil && !isCleanEOF(eb) {
			return false, eb
		}
		if na != nb || !bytes.Equal(ba[:na], bb[:nb]) {
			return false, nil
		}
		if isCleanEOF(ea) && isCleanEOF(eb) {
			return true, nil
		}
	}
}

// packContentChecksum recomputes a pack's content hash. git hashes every byte
// EXCEPT the trailing 20, and stores the result in those 20 — so this is what
// the basename is supposed to be, and it proves the body is what the name
// claims. It cannot detect a change confined to the trailer itself; the byte
// comparison in publishPack and index-pack --verify are what cover that.
//
// SHA-1 only: this design supports SHA-1 repositories, and refs.go rejects
// anything that is not 40 hex.
//
// It returns a digest, never a verdict, and it never names a remote path. The
// CALLER composes the failure message, because on the one path that reaches
// it the correct diagnosis is not the obvious one — see
// checkIdenticalPackChecksum.
func packContentChecksum(path string) (string, error) {
	const trailer = 20
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	if st.Size() < trailer {
		return "", fmt.Errorf("it is %d bytes, shorter than the %d-byte trailing checksum "+
			"every pack ends with", st.Size(), trailer)
	}
	h := sha1.New()
	if _, err := io.CopyN(h, f, st.Size()-trailer); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// packNameChecksum is the content checksum a pack's basename claims to be.
func packNameChecksum(base string) string {
	return strings.TrimSuffix(strings.TrimPrefix(base, "pack-"), ".pack")
}

// linkOrCopy places src at dst, preferring a hard link so a large pack is not
// duplicated on disk just to sit beside an index for verification.
func linkOrCopy(src, dst string) error {
	if err := os.Link(src, dst); err == nil {
		return nil
	}
	return copyFile(src, dst)
}
