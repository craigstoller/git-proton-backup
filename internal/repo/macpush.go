package repo

import (
	"sort"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/protocol"
)

// The Mac push rule (spec 2026-10-27, section 3, "The Mac push rule, in
// repo.Push's phase 2"). On a Mac whose repository uses git's default files
// ref store, git keeps each ref in a file named after it, and APFS treats two
// names that differ only by letter case or Unicode normalisation as one file.
// After a push, git writes or deletes the tracking ref of every ref the
// remote accepted, so a pushed ref can silently clobber or unlink a local ref
// spelled differently. Push cannot see the local repository, so the push arm
// reads it (main.go) and hands Push a LocalSide; this file judges the batch
// against it. Every message passes through Push's fail, so oneLine.

// LocalSide is what the Mac push rule knows about the local repository.
//
//   - Refs holds the name of every local ref as it stands before the push.
//   - Tracking maps a pushed destination to its tracking refs: where the
//     remote's fetch refspecs map it, positive specs only (R6), expanded
//     through local symbolic refs, since git follows a symbolic tracking ref
//     when it updates one, so the target is checked too. A destination with
//     no tracking ref (an anonymous URL, or one no refspec covers) has no
//     entry, and the rule has no local half to judge for it.
//
// A nil *LocalSide means the rule does not apply: off darwin, in a reftable
// repository, and in every test that does not exercise it.
type LocalSide struct {
	Refs     map[string]bool
	Tracking map[string][]string
}

// macWriteTail ends every refusal of a write by this rule, R3's included.
const macWriteTail = "; this repository's default (files) ref store cannot keep these apart " +
	"(README, Honest limits); repair the remote, or push from a --ref-format=reftable clone"

// macItems is cs as this rule's messages list it: formatItem per conflict,
// in cs's order (caseConflicts sorts by Ref), joined by ", ".
func macItems(cs []caseConflict) string {
	parts := make([]string, len(cs))
	for i, c := range cs {
		parts[i] = formatItem(c)
	}
	return strings.Join(parts, ", ")
}

// macRemoteHalfMessage is part 1(b)'s refusal of an update x alone: the remote
// keeps a ref x conflicts with. Phase 5's re-check (R2) appends
// failedDeleteClause to it.
func macRemoteHalfMessage(x string, cs []caseConflict) string {
	return x + ": refused on this Mac: the remote also holds " + macItems(cs) + macWriteTail
}

// macPairMessage is part 2's refusal of delete d, which pairs with the write x
// this rule refused (by exact name, or as the delete half of a rename to x).
// A1 uses it too, for a delete that pairs with a create R3 attributed to this
// rule.
func macPairMessage(d, x string) string {
	return d + ": delete refused because it pairs with " + x + ", which this Mac refused; neither was applied"
}

// macKeptMessage is R3's refusal of create x, which D2 refused only because
// this rule kept deletes: kept are those of x's conflicts this rule kept,
// sorted, named in failedDeleteClause's grammar; cs are all of x's conflicts.
func macKeptMessage(x string, kept []string, cs []caseConflict) string {
	deletes := "delete of " + kept[0] + " was"
	if len(kept) > 1 {
		deletes = "deletes of " + strings.Join(kept[:len(kept)-1], ", ") + " and " + kept[len(kept)-1] + " were"
	}
	return x + ": refused on this Mac: this push's " + deletes + " refused here, and " + x +
		" would then conflict with " + macItems(cs) + macWriteTail
}

// keptConflicts is the Ref of each conflict in cs that kept holds, in cs's
// (sorted) order: the deletes this rule kept that a D2 refusal rests on (R3).
func keptConflicts(cs []caseConflict, kept map[string]bool) []string {
	var out []string
	for _, c := range cs {
		if kept[c.Ref] {
			out = append(out, c.Ref)
		}
	}
	return out
}

// macPass runs the rule's three parts once, in order, over the valid flags as
// phase 2 leaves them after D2 and A1 reach their fixed point. Each part reads
// the state its predecessor left, and each is set algebra over that state, so
// neither outcome nor text depends on batch order. refuse fails entry i with
// msg and marks it invalid; no part touches an entry that is already invalid,
// so every refused update gets exactly one message. The deletes parts 2 and 3
// keep are added to kept, for R3. macPass reports whether it refused anything:
// then the kept refs can expose conflicts D2 must judge, so phase 2 repeats.
//
// Its refusals never enter A1's maps: a delete is paired once, by the rule
// that refused its write.
func macPass(local *LocalSide, ups []protocol.RefUpdate, remote map[string]string,
	valid, isDelete, isCreate []bool, refuse func(i int, msg string), kept map[string]bool) bool {

	refusedAny := false

	// ---- Part 1: writes, over the writes valid at the pass's start.
	//
	// (a) A tracking ref T of write X is checked against the local refs and
	// the tracking refs of the other valid writes: git writes T after the
	// push, into whatever file the files store names it by, and the local
	// refs judged are those before the push, because git rewrites tracking
	// refs only afterwards, so a same-batch delete never clears this half.
	// R7 (Ruling T9-1): an UPDATE's tracking ref is also checked against
	// the tracking refs of the batch's other valid deletes. Measured
	// 2026-10-27 (git 2.54.0, a files clone that never fetched, pushing over
	// the local transport to a reftable remote holding both names): when
	// the update is listed before the delete in the remote's ref order, git
	// writes the update's tracking ref, then its post-push delete of the
	// delete's aliasing tracking ref unlinks that file, and the push still
	// exits 0. With the delete listed first the tracking ref survived. A
	// create beside such a delete was measured safe (git handled the delete
	// first), so creates are not checked against deletes. The delete itself
	// is not refused: it is the repair half.
	//
	// One caseConflicts call covers every write: a conflict with a ref that
	// is neither local, nor another write's tracking ref, nor (for an
	// update) another delete's tracking ref, such as X's own other tracking
	// ref, is dropped after, which equals leaving it out of the set, since
	// conflicts are pairwise.
	//
	// (b) An update X is checked against batchFinalSet, the remote's set as
	// the batch would leave it, so a same-batch delete of the other member
	// clears it. A create's remote side is D2's to judge.
	var writes []int
	owners := map[string][]string{}   // tracking ref -> the valid writes it belongs to
	deleters := map[string][]string{} // tracking ref -> the valid deletes it belongs to (R7)
	for i, u := range ups {
		if !valid[i] {
			continue
		}
		ts := sortedUnique(append([]string(nil), local.Tracking[u.Dst]...))
		if isDelete[i] {
			for _, tr := range ts {
				deleters[tr] = append(deleters[tr], u.Dst)
			}
			continue
		}
		writes = append(writes, i)
		for _, tr := range ts {
			owners[tr] = append(owners[tr], u.Dst)
		}
	}
	set := make(map[string]bool, len(local.Refs)+len(owners)+len(deleters))
	for r := range local.Refs {
		set[r] = true
	}
	for tr := range deleters {
		set[tr] = true
	}
	tracked := make([]string, 0, len(owners))
	for tr := range owners {
		set[tr] = true
		tracked = append(tracked, tr)
	}
	// smallestOther is the smallest name in names other than x, or "".
	smallestOther := func(names []string, x string) string {
		best := ""
		for _, o := range names {
			if o != x && (best == "" || o < best) {
				best = o
			}
		}
		return best
	}
	byTracking := caseConflicts(set, tracked)
	var updates []string
	for _, i := range writes {
		if !isCreate[i] {
			updates = append(updates, ups[i].Dst)
		}
	}
	// A valid write's destination is unique in the batch (phase 2 refuses
	// every holder of a duplicated one), so one call keyed by it covers all.
	remoteHalf := caseConflicts(batchFinalSet(remote, ups, valid, isDelete), updates)

	type verdict struct {
		i   int
		msg string
	}
	var verdicts []verdict
	for _, i := range writes {
		x := ups[i].Dst
		var clauses []string
		for _, tr := range sortedUnique(append([]string(nil), local.Tracking[x]...)) {
			var items []string
			for _, c := range byTracking[tr] {
				other := smallestOther(owners[c.Ref], x)
				deleter := ""
				if !isCreate[i] {
					deleter = smallestOther(deleters[c.Ref], x)
				}
				if !local.Refs[c.Ref] && other == "" && deleter == "" {
					continue
				}
				item := formatItem(c)
				if other != "" {
					item += " (the tracking ref of " + other + " in this push)"
				}
				if deleter != "" {
					item += " (the tracking ref of " + deleter + ", which this push deletes)"
				}
				items = append(items, item)
			}
			if len(items) > 0 {
				clauses = append(clauses, "its tracking ref "+tr+" would collide with "+strings.Join(items, ", "))
			}
		}
		// After an (a) clause the joiner already says "also" (Ruling T9-2).
		if cs := remoteHalf[x]; !isCreate[i] && len(cs) > 0 {
			holds := "the remote also holds "
			if len(clauses) > 0 {
				holds = "the remote holds "
			}
			clauses = append(clauses, holds+macItems(cs))
		}
		if len(clauses) > 0 {
			verdicts = append(verdicts, verdict{i, x + ": refused on this Mac: " +
				strings.Join(clauses, "; also ") + macWriteTail})
		}
	}
	refusedWrites := map[string]bool{}
	var refusedCreates []string
	for _, v := range verdicts {
		refuse(v.i, v.msg)
		refusedAny = true
		refusedWrites[ups[v.i].Dst] = true
		if isCreate[v.i] {
			refusedCreates = append(refusedCreates, ups[v.i].Dst)
		}
	}

	// ---- Part 2: the deletes paired with a write part 1 refused.
	//
	// First by exact name: the delete half of a replacement of X, so a
	// replacement never commits its delete while its write is refused (git
	// never sends one, M26; a direct caller can).
	for j, u := range ups {
		if valid[j] && isDelete[j] && refusedWrites[u.Dst] {
			refuse(j, macPairMessage(u.Dst, u.Dst))
			kept[u.Dst] = true
			refusedAny = true
		}
	}
	// Then, for a refused create only, the delete half of a rename to it: a
	// still-valid delete of a name the pre-batch remote holds, that no
	// still-valid write rewrites, and that matches the create by either key,
	// paired with the smallest such create, as A1 pairs. A refused update has
	// no rename half, so a same-batch delete of the other member, which is the
	// repair, still goes through.
	if len(refusedCreates) > 0 {
		pairedCreate := renamePairs(refusedCreates)
		rewritten := map[string]bool{}
		for i, u := range ups {
			if valid[i] && !isDelete[i] {
				rewritten[u.Dst] = true
			}
		}
		for j, u := range ups {
			if !valid[j] || !isDelete[j] {
				continue
			}
			if _, held := remote[u.Dst]; !held || rewritten[u.Dst] {
				continue
			}
			if x, ok := pairedCreate(u.Dst); ok {
				refuse(j, macPairMessage(u.Dst, x))
				kept[u.Dst] = true
				refusedAny = true
			}
		}
	}

	// ---- Part 3: deletes. git's post-push delete of a deleted ref's tracking
	// ref T can unlink the loose file of a local ref spelled differently,
	// which then silently falls back to a stale packed value (M24). Every
	// still-valid delete is judged, a delete of a name the remote does not
	// hold included: git deletes its tracking ref all the same. One
	// caseConflicts call covers every delete's tracking refs, since each is
	// judged against the local refs alone.
	var deleteTracked []string
	for j, u := range ups {
		if valid[j] && isDelete[j] {
			deleteTracked = append(deleteTracked, local.Tracking[u.Dst]...)
		}
	}
	localHalf := caseConflicts(local.Refs, sortedUnique(deleteTracked))
	for j, u := range ups {
		if !valid[j] || !isDelete[j] {
			continue
		}
		var clauses []string
		for _, tr := range sortedUnique(append([]string(nil), local.Tracking[u.Dst]...)) {
			if c := localHalf[tr]; len(c) > 0 {
				clauses = append(clauses, "its tracking ref "+tr+", which this repository's default (files) "+
					"ref store keeps under the same file name as "+macItems(c))
			}
		}
		if len(clauses) > 0 {
			refuse(j, u.Dst+": delete refused on this Mac: git would then delete "+strings.Join(clauses, "; also ")+
				" (README, Honest limits); delete it from a --ref-format=reftable clone")
			kept[u.Dst] = true
			refusedAny = true
		}
	}
	return refusedAny
}

// renamePairs indexes refused creates by each key, each index keeping its
// smallest create, and returns the lookup A1 and part 2 pair a delete with: a
// delete d pairs with byCase[caseKey(d)] and byCanon[canonKey(d)], whichever
// exist, and names the smaller, so its text depends neither on batch order
// nor on which key matched. refused is not changed.
func renamePairs(refused []string) func(d string) (string, bool) {
	sorted := append([]string(nil), refused...)
	sort.Strings(sorted)
	byCase, byCanon := map[string]string{}, map[string]string{}
	for _, x := range sorted {
		if _, ok := byCase[caseKey(x)]; !ok {
			byCase[caseKey(x)] = x
		}
		if _, ok := byCanon[canonKey(x)]; !ok {
			byCanon[canonKey(x)] = x
		}
	}
	return func(d string) (string, bool) {
		x, ok := byCase[caseKey(d)]
		if y, canon := byCanon[canonKey(d)]; canon && (!ok || y < x) {
			return y, true
		}
		return x, ok
	}
}
