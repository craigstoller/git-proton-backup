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

// Push applies the whole batch through a FIVE-PHASE engine (design
// component 2b), not per-ref independence with a per-ref pack. Multi-ref
// batches are still NOT atomic: every update gets its own Result, so partial
// success (some refs updated, others rejected) is expected and correct,
// never collapsed into a single batch-wide outcome — what changed from the
// per-ref shipped version is WHEN each ref's fate is decided and how many
// packs the batch costs, never atomicity itself.
//
//  1. (buffering the complete blank-line-terminated batch is the caller's
//     job, unchanged.)
//  2. Validate the WHOLE batch before anything moves: destination namespace,
//     duplicate destinations (every holder of a duplicated destination is
//     refused, not a first-seen-wins loop), delete-side HEAD protection (one
//     non-mutating ReadHEAD for the whole batch), namespace-specific
//     force/ancestry rules, and a final-state directory/file preflight over
//     the refs that will exist once the batch lands. Failures here cost
//     nothing: no pack has been built yet.
//  3. Build, upload, and confirm ONE pack for every valid create/update —
//     the design's normative "Object transfer per batch", which Stage 2's
//     per-ref packing quietly diverged from (see gitcmd.WritePack's doc
//     comment). On any pack failure, every valid create/update is failed
//     naming it, but execution CONTINUES to phase 4: deletions are
//     object-independent and must not be held hostage by an unrelated pack
//     failure.
//  4. Execute deletions (Stage 4's HEAD-protection logic, verbatim).
//  5. Execute creates/updates: ensureRefParents, then WriteRef.
//
// ensureHEAD runs last, after all five phases, unchanged from the per-ref
// version.
func Push(t transport.Transport, root, gitDir string,
	ups []protocol.RefUpdate, remote map[string]string) []Result {

	results := make([]Result, len(ups))
	// fail/okResult flatten every outcome through the same funnel pushOne
	// used to: Results are rendered by the caller as one
	// "error <ref> <reason>" status line per update, so an embedded newline
	// would desynchronise the protocol.
	fail := func(i int, msg string) { results[i] = Result{Ref: ups[i].Dst, Err: oneLine(msg)} }
	okResult := func(i int) { results[i] = Result{Ref: ups[i].Dst, OK: true} }

	// ======================= Phase 2: whole-batch validation ================

	// Duplicate destinations are pre-scanned so EVERY holder of a duplicated
	// dst is refused — a first-seen-wins loop lets the first duplicate
	// mutate while only later ones are refused (round-1 Codex). Restricted
	// to non-delete entries: a delete and a create/update sharing one dst
	// are NOT ambiguous the way two creates are — phase ordering (deletions
	// always run before creates, phase 4 before phase 5) makes the
	// composition deterministic regardless of the batch's own input order.
	// TestPushDeleteThenRecreateInOneBatchStillDerivesHead already pins
	// "delete X, recreate X" in one batch as a supported idiom, not a
	// conflict — a deliberate, reasoned divergence from the plan's
	// illustrative pseudocode (which counts every entry regardless of
	// delete/non-delete), flagged in the task report.
	dstCount := map[string]int{}
	// hasBranchDelete is computed in the same pre-scan pass so the HEAD read
	// below can be gated on it (fix round M1): a create-only, tag-only, or
	// notes-only batch has no delete for HEAD protection to ever apply to,
	// and ReadHEAD is a real remote round-trip — paying it unconditionally on
	// every push, including ones with no delete at all, was avoidable cost.
	hasBranchDelete := false
	for _, u := range ups {
		if u.Src != "" {
			dstCount[u.Dst]++
		} else if isBranch(u.Dst) {
			hasBranchDelete = true
		}
	}

	// HEAD is read ONCE, non-mutating, for the whole batch (the caller holds
	// the repo lock for the whole Push call), so a delete of the HEAD branch
	// is refused HERE — before it can distort the final-state preflight or
	// cost a pack upload (round-1 Codex) — but ONLY when the batch actually
	// contains a branch delete (M1): HEAD can only ever name a branch (spec
	// §1), so nothing else in the batch can ever consult these three values,
	// and their zero values (headErr == nil, hasHead == false) are never read
	// on a batch where hasBranchDelete is false — every read of them below is
	// itself gated behind isBranch(u.Dst) on a delete entry, which can only be
	// true if hasBranchDelete is also true.
	var head string
	var hasHead bool
	var headErr error
	if hasBranchDelete {
		head, hasHead, headErr = ReadHEAD(t, root)
	}

	valid := make([]bool, len(ups))
	isDelete := make([]bool, len(ups))
	isCreate := make([]bool, len(ups)) // valid non-delete whose dst is not already on the remote
	newShas := make([]string, len(ups))

	for i, u := range ups {
		// --- destination namespace, FIRST: see checkDst's own doc comment
		// for why rejecting early matters (no pack cost, no orphan left
		// behind on the user's paid Drive).
		if err := checkDst(u.Dst); err != nil {
			fail(i, err.Error())
			continue
		}

		if u.Src == "" {
			isDelete[i] = true
			if _, exists := remote[u.Dst]; !exists {
				// Already absent: OK without even consulting HEAD, exactly
				// like shipped pushOne — deleting a name already gone is a
				// no-op, and its validity can never affect the D/F
				// preflight below (an absent name was never in finalSet to
				// begin with, so subtracting it changes nothing).
				valid[i] = true
				continue
			}
			// HEAD protection is BRANCHES-ONLY (spec §1: HEAD can only ever
			// name a branch). An unreadable HEAD must not block deleting
			// tags/notes/etc — shipped pushOne gated EVERY delete on this
			// read; this NARROWS it per the spec (flagged in the task
			// report as a deliberate alignment).
			if isBranch(u.Dst) {
				if headErr != nil {
					fail(i, fmt.Sprintf("refusing to delete %s: remote HEAD could not be read, "+
						"so it is unknown whether HEAD points at this branch: %v", u.Dst, headErr))
					continue
				}
				if hasHead && head == u.Dst {
					fail(i, fmt.Sprintf("refusing to delete the branch HEAD points at (%s); "+
						"change the default branch first (git-remote-proton --set-head <url> <branch>)", u.Dst))
					continue
				}
			}
			valid[i] = true
			continue
		}

		if dstCount[u.Dst] > 1 {
			fail(i, "duplicate destination in one batch")
			continue
		}

		newSha, err := resolve(gitDir, u.Src)
		if err != nil {
			fail(i, err.Error())
			continue
		}

		oldSha, exists := remote[u.Dst]

		// Namespace branching BEFORE any ancestry logic (round-2 Codex): the
		// design's ref-transition table gives each namespace different
		// rules, and running the generic HasObject/IsAncestor block first
		// would surface "fetch first" or an ancestry-tooling error on refs
		// the table says need only a force check — and would run rev-list
		// machinery on non-commit objects (notes trees, replace blobs).
		switch {
		case isBranch(u.Dst):
			typ, err := gitcmd.ObjectType(gitDir, newSha)
			if err != nil {
				fail(i, "cannot determine object type")
				continue
			}
			if typ != "commit" {
				fail(i, fmt.Sprintf("branch cannot point at a %s", typ))
				continue
			}
			if exists && !u.Force {
				if !gitcmd.HasObject(gitDir, oldSha) {
					fail(i, "fetch first")
					continue
				}
				// IsAncestor distinguishes "not an ancestor" (exit 1) from a
				// tooling failure. Discarding the error would report a
				// broken git as a confident non-fast-forward rejection.
				anc, aerr := gitcmd.IsAncestor(gitDir, oldSha, newSha)
				if aerr != nil {
					fail(i, "cannot determine ancestry: "+aerr.Error())
					continue
				}
				if !anc {
					fail(i, "non-fast-forward")
					continue
				}
			}
		case strings.HasPrefix(u.Dst, "refs/tags/"):
			// Design table: "Tag update | Requires force, matching git's
			// rule; no ancestry check." Shipped pushOne has no tag arm at
			// all and runs the generic ancestry block on tag updates
			// instead — a pre-existing divergence from the design table
			// that this restructure ALIGNS rather than preserves (flagged
			// in the task report).
			if exists && !u.Force {
				fail(i, "tag update requires force")
				continue
			}
		default: // other namespaces — the design's conservative deviation
			if requiresForce(u.Dst) && exists && !u.Force {
				fail(i, "updating refs outside refs/heads/ and refs/tags/ requires force "+
					"(conservative rule; see design)")
				continue
			}
		}

		newShas[i] = newSha
		isCreate[i] = !exists
		valid[i] = true
	}

	// Final-state D/F preflight over REFS ONLY (empty folders are runtime,
	// self-heal's job — Task 9b). finalSet is the ref namespace as it will
	// read immediately after this batch: every currently-advertised ref,
	// minus every VALID delete in this batch (a REFUSED delete is NOT
	// subtracted — the ref genuinely still exists on the remote, so a
	// dependent create must still be checked against it), plus every valid
	// create/update's destination.
	finalSet := make(map[string]bool, len(remote)+len(ups))
	for ref := range remote {
		finalSet[ref] = true
	}
	// TWO PASSES, deliberately, not one interleaved pass keyed on ups' own
	// order (fix round I1): finalSet is SET ALGEBRA — remote minus every
	// valid delete, plus every valid non-delete — and must be independent of
	// which order ups happens to list them in. A single pass that applies
	// each entry in input order gets this wrong whenever a delete and a
	// non-delete target the SAME name in one batch: "update X, delete X"
	// (update listed first) folds correctly under set algebra (X survives,
	// carrying the update's new value — the delete does not "win" merely by
	// being processed second), but a naive single in-order pass would apply
	// the update then the delete and lose X, when the batch's actual EXECUTION
	// order (phase 4 deletes, then phase 5 creates/updates) means X survives
	// either way. Subtracting every delete FIRST, then adding every
	// non-delete, makes the non-delete always win a same-name collision,
	// matching execution order and being independent of ups' own order.
	for i, u := range ups {
		if valid[i] && isDelete[i] {
			delete(finalSet, u.Dst)
		}
	}
	for i, u := range ups {
		if valid[i] && !isDelete[i] {
			finalSet[u.Dst] = true
		}
	}
	for i, u := range ups {
		if !valid[i] || isDelete[i] || !isCreate[i] {
			continue
		}
		for other := range finalSet {
			if other == u.Dst {
				continue
			}
			if strings.HasPrefix(other, u.Dst+"/") || strings.HasPrefix(u.Dst, other+"/") {
				fail(i, fmt.Sprintf("%s conflicts with %s: a ref cannot be both a leaf and a "+
					"folder containing other refs", u.Dst, other))
				valid[i] = false
				break
			}
		}
	}

	// ======================= Phase 3: one pack for the whole batch ==========

	var wants []string
	for i := range ups {
		if valid[i] && !isDelete[i] {
			wants = append(wants, newShas[i])
		}
	}

	if len(wants) > 0 {
		// failPending fails every still-valid create/update with msg — the
		// phase-3-continues rule: on a pack failure every valid non-delete
		// is failed, but phase 4's deletions still run (adjudicated round
		// 2; TestPushPackFailureFailsCreatesButDeletionsProceed pins it).
		failPending := func(msg string) {
			for i := range ups {
				if valid[i] && !isDelete[i] {
					fail(i, msg)
					valid[i] = false
				}
			}
		}

		// haves is built from the ref list as it stood when the batch
		// started (the known cost pushOne's own comment documented: a
		// larger pack is never wrong, and one pack per BATCH means this
		// cost is paid once, not once per ref). This deliberately includes
		// the tips of refs THIS SAME BATCH is about to delete (fix round
		// M5): harmless today, because v2 has no object GC and an excluded-
		// but-still-reachable-elsewhere object costs nothing extra. Flag for
		// whoever eventually extends prune/self-heal (Task 9b's territory)
		// to objects, not just refs/folders: at that point a have drawn from
		// a ref this batch deletes could build a pack excluding a closure
		// that is about to become unreachable, which would need its own
		// answer before objects are ever actually reclaimed.
		haves := make([]string, 0, len(remote))
		for _, s := range remote {
			if gitcmd.HasObject(gitDir, s) {
				haves = append(haves, s)
			}
		}

		tmp, err := os.MkdirTemp("", "gpb-pack-*")
		if err != nil {
			failPending(err.Error())
		} else {
			defer os.RemoveAll(tmp)
			packPath, idxPath, perr := gitcmd.WritePack(gitDir, wants, haves, tmp)
			if perr != nil {
				failPending("pack failed: " + perr.Error())
			} else if packPath != "" {
				// Pack, then index, then CONFIRM BOTH before publishing any
				// ref. Confirmation is per member: a .pack is named by its
				// own content checksum, a .idx borrows that name, so they
				// cannot be checked the same way (design v6.2).
				packDst := root + "/packs/" + filepathBase(packPath)
				idxDst := root + "/packs/" + filepathBase(idxPath)
				if err := publishPack(t, packDst, packPath); err != nil {
					failPending(err.Error())
				} else if err := publishIdx(t, idxDst, idxPath, packPath); err != nil {
					failPending(err.Error())
				}
			}
		}
	}

	// ======================= Phase 4: deletions ==============================
	// Phase 2 already refused HEAD-branch deletes via the batch's single
	// non-mutating ReadHEAD; this per-delete HEAD re-check is defense-in-
	// depth — one cheap read that covers a HEAD written between phases by a
	// non-v2 actor. Scoped to branches-only, matching phase 2's narrowing:
	// applying it unconditionally (as shipped pushOne did) would refuse a
	// non-branch delete under an unreadable HEAD right back out again,
	// defeating the branches-only rule phase 2 just implemented.
	for i, u := range ups {
		if !valid[i] || !isDelete[i] {
			continue
		}
		if _, exists := remote[u.Dst]; !exists {
			okResult(i)
			continue
		}
		if isBranch(u.Dst) {
			h, hasH, herr := ReadHEAD(t, root)
			if herr != nil {
				fail(i, fmt.Sprintf("refusing to delete %s: remote HEAD could not be read, "+
					"so it is unknown whether HEAD points at this branch: %v", u.Dst, herr))
				continue
			}
			if hasH && h == u.Dst {
				fail(i, fmt.Sprintf("refusing to delete the branch HEAD points at (%s); "+
					"change the default branch first (git-remote-proton --set-head <url> <branch>)", u.Dst))
				continue
			}
		}
		out, err := t.Trash(root + "/" + u.Dst)
		if err != nil {
			fail(i, fmt.Sprintf("delete failed: %v", err))
			continue
		}
		if out != transport.Committed {
			// err is nil here, so a bare "%v" would print the useless
			// "delete failed: <nil>". Report the outcome itself instead.
			fail(i, fmt.Sprintf("delete failed: outcome %s", out))
			continue
		}
		okResult(i)
		// Best-effort tidiness (Task 9b), called ONLY after a Committed Trash
		// of the ref file itself — never from the already-absent short-circuit
		// above, which trashed nothing and has no empty parent of its own
		// making to clean up. pruneEmptyParents is void and never touches
		// results[i]: a prune failure is advisory only (see its own doc
		// comment), and this delete has already reported ok.
		pruneEmptyParents(t, root, u.Dst)
	}

	// deletedThisBatch tracks every dst whose delete ACTUALLY COMMITTED in
	// phase 4 — built from phase 4's real outcome (results[i].OK), NOT from
	// phase-2 validity (fix round 2, Important): a delete can be VALID at
	// phase 2 and still fail in phase 4 — the per-delete HEAD re-check can
	// refuse it, or t.Trash can return non-Committed or an error (a
	// non-v2 actor wrote HEAD between phases, a transient transport fault,
	// and so on). A dst whose delete failed is still occupied on the
	// remote, so a paired same-dst non-delete in phase 5 must NOT be routed
	// through CreateExclusive on the strength of phase-2 validity alone —
	// that previously produced the wrong diagnosis ("ref changed
	// concurrently") for an update that would have succeeded via
	// UpdateRevision, because nothing concurrent happened; the batch's own
	// delete simply failed. Computed here, after phase 4 has actually run
	// and every delete's Result is final, not right after phase 2.
	deletedThisBatch := map[string]bool{}
	for i, u := range ups {
		if isDelete[i] && results[i].OK {
			deletedThisBatch[u.Dst] = true
		}
	}

	// ======================= Phase 5: creates/updates ========================
	for i, u := range ups {
		if !valid[i] || isDelete[i] {
			continue
		}
		// exists is BATCH-AWARE (fix round M4, corrected in fix round 2), not
		// a bare read of the pre-batch remote map: when a same-batch delete
		// of this exact dst actually COMMITTED in phase 4 (deletedThisBatch,
		// built from phase 4's real outcome, not phase-2 validity), the node
		// genuinely no longer exists on the remote regardless of what
		// remote[u.Dst] said before the batch started — WriteRef must take
		// the CreateExclusive path, not UpdateRevision, for exactly the
		// reason deletedThisBatch's own doc comment above gives. When the
		// same-batch delete instead FAILED in phase 4, deletedThisBatch does
		// not contain the dst, exists falls back to the pre-batch read, and
		// WriteRef correctly takes UpdateRevision against the node that is
		// still actually there.
		_, preExists := remote[u.Dst]
		exists := preExists && !deletedThisBatch[u.Dst]
		if err := ensureRefParents(t, root, u.Dst); err != nil {
			fail(i, err.Error())
			continue
		}
		// CREATES ONLY route through the self-healing wrapper (Task 9b):
		// exists here is the same batch-aware flag deletedThisBatch's own doc
		// comment above establishes, so an update against a node this exact
		// batch just recreated still takes the plain WriteRef/UpdateRevision
		// path unchanged. Self-heal exists for the D/F folder-collision a
		// CREATE can hit — a leftover empty folder from a crashed prune, or a
		// genuine conflicting ref/foreign file underneath — never for an
		// update against a node already known to be there.
		var out transport.Outcome
		var err error
		if exists {
			out, err = WriteRef(t, root, u.Dst, newShas[i], true)
		} else {
			out, err = createRefHealingCollision(t, root, u.Dst, newShas[i])
		}
		if err != nil || out == transport.Ambiguous {
			fail(i, fmt.Sprintf("ref publish failed: %v", err))
			continue
		}
		if out == transport.Refused {
			// WriteRef (refs.go) returns (Refused, nil) — no error —
			// specifically when this is a create (exists == false) and a
			// concurrent creator won the race; it deliberately did not
			// overwrite. createRefHealingCollision can reach this arm the
			// same way, via its own inner WriteRef call, whenever the
			// collision it found was NOT a folder at all (its own doc
			// comment: "not a folder collision: surface the original
			// result"). Either way this is not success: our newSha was never
			// published, so reporting OK: true here would make git update
			// its remote-tracking ref to a sha that disagrees with what is
			// actually on the remote, with nothing to signal the mismatch.
			// It must be reported as a failure.
			fail(i, "ref changed concurrently; refusing to overwrite")
			continue
		}
		okResult(i)
	}

	// Complete a missing HEAD. This is the same rule Bootstrap applies to a
	// missing refs/ or packs/ — a partial initialisation is completed, not
	// rejected — and it is why a repo pushed before this shipped is still
	// clonable. An EXISTING HEAD is never touched: changing the default
	// branch stays an explicit operation, out of scope for v2.
	ensureHEAD(t, root, gitDir, ups, remote, results)
	return results
}

// parentOf returns ref's parent in "/"-joined REF-NAME space (not a remote
// path): everything before the last "/", or "" once there is no "/" left at
// all. In practice pruneEmptyParents' loop below always stops at a protected
// namespace root ("refs", "refs/heads", "refs/tags") before ever reaching
// the "" case, since every real ref begins "refs/...".
func parentOf(ref string) string {
	i := strings.LastIndex(ref, "/")
	if i < 0 {
		return ""
	}
	return ref[:i]
}

// protectedNamespaceRoots are the folders Bootstrap creates as part of the
// initialised layout ("refs", "refs/heads", "refs/tags") — permanent
// scaffolding, never residue of anything THIS helper did. Shared by
// pruneEmptyParents (never prunable, even when momentarily empty) and
// createRefHealingCollision (never self-healable by trashing): a create
// whose destination IS one of these names (e.g. a ref quite literally named
// "refs/heads") collides with a folder git itself needs to exist there, and
// that is a genuine, permanent D/F conflict — not a leftover from a crashed
// prune, even on a freshly-bootstrapped repo where the folder happens to be
// empty because no refs have been pushed into it yet.
var protectedNamespaceRoots = map[string]bool{"refs": true, "refs/heads": true, "refs/tags": true}

// pruneEmptyParents is BEST-EFFORT tidiness under the batch lock. Check-then-
// act: no conditional delete exists on this transport (same accepted limit as
// lock release); the blast radius of the race is bounded — subtree goes to
// Proton's trash, and only a NON-v2 actor can be racing (v2 writers hold the
// lock). Self-heal (createRefHealingCollision) is the correctness mechanism;
// a plan that ships prune without heal reintroduces the wedge (spec §2c).
func pruneEmptyParents(t transport.Transport, root, ref string) {
	for dir := parentOf(ref); dir != "" && !protectedNamespaceRoots[dir]; dir = parentOf(dir) {
		nodes, err := t.List(root + "/" + dir)
		if err != nil || len(nodes) != 0 {
			if err != nil {
				fmt.Fprintf(os.Stderr, "git-remote-proton: prune: cannot list %s/%s: %v\n", root, dir, err)
			}
			return
		}
		out, err := t.Trash(root + "/" + dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "git-remote-proton: prune: leaving empty folder %s/%s "+
				"(trash failed: %v); a later create at this name self-heals\n", root, dir, err)
			return
		}
		if out != transport.Committed {
			// err is nil here (M10) — report the outcome itself instead of a
			// useless "trash reported <nil>/...", matching the delete arm's
			// own earlier fix for the identical shape.
			fmt.Fprintf(os.Stderr, "git-remote-proton: prune: leaving empty folder %s/%s "+
				"(trash reported %s); a later create at this name self-heals\n", root, dir, out)
			return
		}
	}
}

// subtreeFiles returns the full remote path of every FILE anywhere under
// folder (recursive walk via t.List; folders alone do not count). Used by
// self-heal's residue test: the rule is "contains no files OF ANY KIND" — a
// foreign file makes the subtree not-residue and must never be trashed
// (spec §2c, round 3). It deliberately does NOT filter by content: a file
// that fails to read back as a valid ref is still a FILE, and must still
// block the heal — describeBlockers (below), not this function, is where
// "ref vs. foreign" gets decided, and only for the message, never for
// whether healing is allowed to proceed.
//
// checkComponent gates FOLDER names only, BEFORE recursing into them (plan
// round 3, Codex): an invalid folder component, a{b} say, returns an ERROR,
// which makes the heal FAIL CLOSED (no List() call is ever made naming an
// unverifiable remote path — this transport's remote-glob behaviour on
// characters like "{" is UNVERIFIED, probe C13 only confirmed LOCAL
// glob-expansion on upload; a trace test pins that no List ever names the
// invalid path and nothing is trashed). A FILE with an invalid component is
// NOT gated here (review round 4, M6): including it in the returned slice
// makes no further remote call — it was already safely enumerated as part
// of listing its PARENT, an already-verified path — so there is no
// recursion hazard to guard against. describeBlockers is where an
// invalid-named file gets classified, by name alone, without ever
// readRef-ing the suspect path.
func subtreeFiles(t transport.Transport, folder string) ([]string, error) {
	nodes, err := t.List(folder)
	if err != nil {
		// Wrapped with the failing folder's own path (Important 2, review
		// round 4) — the exact defect ListRefs' own walk (refs.go) already
		// documents fixing: without it, a List failure several levels into
		// the subtree surfaces as whatever bare message the transport
		// happened to return, with nothing here naming WHICH folder in the
		// recursion actually failed.
		return nil, fmt.Errorf("listing %s: %w", folder, err)
	}
	var out []string
	for _, n := range nodes {
		full := folder + "/" + n.Name
		if n.IsDir {
			if err := checkComponent(n.Name); err != nil {
				return nil, fmt.Errorf("subtree of %s contains an unverifiable folder name %q: %w",
					folder, n.Name, err)
			}
			sub, err := subtreeFiles(t, full)
			if err != nil {
				return nil, err
			}
			out = append(out, sub...)
			continue
		}
		out = append(out, full)
	}
	return out, nil
}

// maxDescribedBlockers caps how many blocking entries describeBlockers
// actually names and readRef's (review round 4, Important 1): the blockers
// come from a folder's contents, which is FOREIGN-CONTROLLED — any non-v2
// actor can drop an arbitrary number of files there — and each described
// entry costs a readRef (a subprocess plus a temp dir) and grows one joined
// error string without bound. The codebase caps every other piece of
// untrusted rendering the same way (previewBytes: 64 bytes, refs.go; bound:
// 200 characters, cli.go); this is that same discipline applied here.
const maxDescribedBlockers = 5

// describeBlockers renders one clause per blocking file (capped at
// maxDescribedBlockers, with the remainder summarised as a count — Important
// 1), naming each as what it actually IS rather than treating every blocker
// alike: a file whose content reads back through readRef as a valid 40-hex
// sha is a genuine conflicting REF; anything else — content a non-v2 actor
// could have dropped at any advertisable-shaped name, e.g.
// "feature/notes.txt" — is named as foreign data, never as a ref.
// Classification is by CONTENT (readRef), not by the file's name shape: a
// foreign file can sit at a name that is itself perfectly ref-shaped (git
// ref names may contain dots), so name-shape alone cannot tell the two
// apart.
//
// The ONE exception: a leaf whose OWN component fails checkComponent (M6)
// is classified as foreign data by NAME alone, without ever calling readRef
// on it — a structurally invalid ref-name component can never be a
// legitimate ref regardless of content, so there is nothing to gain by
// probing it, and every probe on an unverified name carries the same local
// glob-expansion risk (probe C13) subtreeFiles' folder case guards against.
func describeBlockers(t transport.Transport, root string, files []string) string {
	shown := files
	var remaining int
	if len(files) > maxDescribedBlockers {
		shown = files[:maxDescribedBlockers]
		remaining = len(files) - maxDescribedBlockers
	}
	parts := make([]string, 0, len(shown)+1)
	for _, p := range shown {
		rel := strings.TrimPrefix(p, root+"/")
		leaf := rel[strings.LastIndex(rel, "/")+1:]
		if cerr := checkComponent(leaf); cerr != nil {
			parts = append(parts, fmt.Sprintf("%s (foreign data, not a ref: invalid ref-name "+
				"component: %v)", rel, cerr))
			continue
		}
		if sha, err := readRef(t, p); err == nil {
			parts = append(parts, fmt.Sprintf("%s (a conflicting ref, currently %s)", rel, sha))
		} else {
			parts = append(parts, fmt.Sprintf("%s (foreign data, not a ref: %v)", rel, err))
		}
	}
	if remaining > 0 {
		parts = append(parts, fmt.Sprintf("…and %d more", remaining))
	}
	return strings.Join(parts, "; ")
}

// createRefHealingCollision wraps WriteRef for a phase-5 CREATE (exists ==
// false) only — Push's own routing decides that, never this function.
// pruneEmptyParents' doc comment above explains why a crash mid-prune can
// leave an EMPTY folder residue at exactly the name a later create wants:
// this is the correctness half of that pair — prune's best-effort stance is
// only safe because this exists to clean up what it can leave behind.
//
// A non-Committed outcome that is NOT a folder collision (an ordinary
// concurrent-creator Refused, or any other transport failure) is surfaced
// unmodified: this function invents no new diagnosis for a failure mode it
// has no positive evidence for.
func createRefHealingCollision(t transport.Transport, root, ref, sha string) (transport.Outcome, error) {
	out, err := WriteRef(t, root, ref, sha, false)
	if err == nil && out == transport.Committed {
		return out, nil
	}
	n, ok, serr := t.Stat(root + "/" + ref)
	if serr != nil {
		return transport.Ambiguous, fmt.Errorf("create of %s did not commit and its "+
			"diagnosis failed: %v (original: %v)", ref, serr, err)
	}
	if !ok || !n.IsDir {
		return out, err // not a folder collision: surface the original result
	}
	if protectedNamespaceRoots[ref] {
		// A create whose OWN destination is "refs", "refs/heads", or
		// "refs/tags" collides with permanent Bootstrap scaffolding, not
		// residue — see protectedNamespaceRoots' doc comment. This can be
		// EMPTY on a freshly-bootstrapped repo (no refs pushed yet), which
		// would otherwise look exactly like a healable crashed-prune residue
		// to the files-based test below; the identity check here has to run
		// BEFORE that test, not rely on it, because emptiness alone cannot
		// tell the two apart.
		return out, err // surface the original result: a genuine, permanent collision
	}
	files, ferr := subtreeFiles(t, root+"/"+ref)
	if ferr != nil {
		return transport.Ambiguous, fmt.Errorf("create of %s collided with a folder, but "+
			"enumerating its subtree to check for residue failed: %v; heal aborted, nothing "+
			"trashed", ref, ferr)
	}
	if len(files) > 0 {
		// partition into ref-shaped names vs foreign; name each as what it is.
		// NOTE (M7): this Refused carries a NON-NIL error, unlike WriteRef's
		// own (Refused, nil) concurrent-creator convention (refs.go) — a
		// caller must check err FIRST, exactly as Push's phase 5 already
		// does (`if err != nil || out == transport.Ambiguous` is checked
		// before the bare `out == transport.Refused` branch).
		return transport.Refused, fmt.Errorf("a folder occupies %s and its contents block "+
			"the branch: %s", ref, describeBlockers(t, root, files))
	}
	tout, terr := t.Trash(root + "/" + ref)
	if terr != nil {
		return transport.Ambiguous, fmt.Errorf("empty folder at %s could not be cleared "+
			"(trash failed: %v); refusing to create over unknown state", ref, terr)
	}
	if tout != transport.Committed {
		// terr is nil here, so a bare "%v" would print the useless "trash
		// failed: <nil>" (M10) — report the outcome itself instead, matching
		// the delete arm's own earlier fix for the identical shape.
		return transport.Ambiguous, fmt.Errorf("empty folder at %s could not be cleared "+
			"(trash reported %s); refusing to create over unknown state", ref, tout)
	}
	fmt.Fprintf(os.Stderr, "git-remote-proton: cleared what is likely residue of an "+
		"interrupted delete — an empty folder at %s\n", ref)
	return WriteRef(t, root, ref, sha, false) // retry ONCE
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
	// TWO PASSES, deliberately (fix round I1, the same set-algebra fix as
	// finalSet above, and for the identical reason): subtract every
	// successful delete FIRST, then add every successful non-delete, so the
	// result is independent of which order ups lists them in. A single pass
	// keyed on ups' own order got "create main, delete main" (create listed
	// first, no remote HEAD) wrong — it left `seen` empty and the repo
	// headless even though refs/heads/main demonstrably exists after the
	// push; a batch listing them in the other order was already correct by
	// coincidence, which is exactly the kind of order-dependence set algebra
	// must not have.
	for _, u := range ups {
		if u.Src == "" && okNow[u.Dst] {
			delete(seen, u.Dst) // a successful delete removes a candidate
		}
	}
	for _, u := range ups {
		if u.Src != "" && okNow[u.Dst] {
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

func isBranch(ref string) bool { return strings.HasPrefix(ref, "refs/heads/") }

// checkDst admits any advertisable name under refs/. The v6.1 narrowing is
// retired: recursive ListRefs (Task 8) erased its first justification, batch
// preflight (Task 9a) its second. Pseudorefs and non-refs/ destinations stay
// rejected.
//
// Cost: one `git check-ref-format` subprocess PER DESTINATION — Push's phase
// 2 calls this once per ref update in the batch, same as the per-ref pushOne
// it replaced; the batch-preflight restructure did not change this cost,
// only when the destinations it validates are known.
func checkDst(dst string) error {
	if !strings.HasPrefix(dst, "refs/") {
		return fmt.Errorf("unsupported destination %q: only refs under refs/ are served "+
			"(pseudorefs and other destinations have no representation on this remote)", dst)
	}
	// AUTHORITY FIRST (spec §1, both round-1 engines): the push boundary runs
	// the REAL git check-ref-format; the in-process validator covers only
	// stageability afterwards. Order matters for diagnosability too — a name
	// git rejects gets git's verdict, not the in-process approximation's.
	ok, err := gitcmd.CheckRefFormat(dst)
	if err != nil {
		return fmt.Errorf("cannot validate ref name %q with git: %w", dst, err)
	}
	if !ok {
		return fmt.Errorf("invalid ref name %q (git check-ref-format)", dst)
	}
	return advertisableName(dst)
}

// requiresForce: the design's conservative deviation — any move outside
// refs/heads/* and refs/tags/* requires force (v2 does not inspect object
// types the way git's own namespace rules do; conservative cannot lose
// data). Called from Push's phase-2 "other namespaces" arm; tags and
// branches have their own, different force rules (design table) and are
// dispatched separately before this function is ever consulted, so it is
// always true by construction on the path that calls it — the call still
// documents the fact in place rather than asserting it silently.
func requiresForce(dst string) bool {
	return !strings.HasPrefix(dst, "refs/heads/") && !strings.HasPrefix(dst, "refs/tags/")
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
