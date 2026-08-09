package repo

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/gitcmd"
	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// Fetch downloads every pack from the remote, verifies each, confirms the
// closure covers wants, and installs it as ONE pack with an adjacent .keep.
// It returns that .keep's path for the caller's `lock` response, or "" when
// the local repository already had everything.
//
// It is READ-ONLY on the remote: no Bootstrap, no lock, nothing created. A
// fetch must never be able to bring a repository into existence.
//
// Fetch signature gains cacheDir: "" means no persistent sidecar cache
// (every sidecar lives in this run's temp dir). Everything below the
// discovery loop — verify-before-install, consolidation, the single .keep —
// is Stage 3a's code, untouched.
func Fetch(t transport.Transport, root, gitDir, cacheDir string, wants []string) (string, error) {
	if err := RequireMarker(t, root); err != nil {
		return "", err
	}
	if len(wants) == 0 {
		return "", nil
	}

	// Short-circuit on gitDir's OWN store, with no alternate spliced in: if it
	// already reads back a complete closure for wants, this fetch is up to
	// date and the remote need not be touched at all.
	//
	// This has to be a PRESENCE check, not the ref-reachability one
	// RevListNewObjects relies on below. Confirmed directly against real git:
	// `rev-list --objects <sha> --not --all` re-lists an object that is
	// already sitting in the object store, unpacked-but-unreferenced, the
	// moment no local ref happens to reach it — "--not --all" excludes by ref
	// reachability, never by store presence. Fetch itself never writes local
	// refs (that is the calling git process's job, done AFTER this helper
	// exits and reports success), so a destination whose wants are not yet
	// reachable from any ref — a fresh clone's destination, or this same
	// scenario replayed in a test that calls Fetch twice with nothing
	// updating refs in between — would otherwise recompute and reinstall the
	// full closure on every call, never converging to ("", nil). Asking
	// ConnectivityOK about gitDir alone sidesteps refs entirely: it is a
	// traversal that only fails when an object is genuinely unreadable, so a
	// nil result here means every object is already present, regardless of
	// what (if anything) references it.
	if err := gitcmd.ConnectivityOK(gitDir, "", wants); err == nil {
		return "", nil
	}

	tmp, err := os.MkdirTemp("", "gpb-fetch-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)

	// git only finds packs in an alternate at <objects>/pack/. Flat, they are
	// silently invisible and every object reads as missing.
	altObjects := filepath.Join(tmp, "objects")
	packDir := filepath.Join(altObjects, "pack")
	fallbackDir := filepath.Join(tmp, "idx")
	// incomingDir is the per-fetch quarantine: downloads land and verify here,
	// never in packDir directly. It is a SIBLING of altObjects under the one
	// per-fetch tmp root, so it is on the same filesystem by construction —
	// publication (downloadAndVerifyPack's publishPair) is a plain os.Rename,
	// never a cross-volume copy of a repository-sized pack. See design doc
	// Component 4 ("Quarantine fetch staging").
	incomingDir := filepath.Join(tmp, "incoming")
	for _, d := range []string{packDir, fallbackDir, incomingDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return "", err
		}
	}

	stems, err := listCompletePacks(t, root)
	if err != nil {
		return "", err
	}
	// A marked repo always has at least one pack (a ref cannot be published
	// without one), so an empty listing means the remote is incomplete — and
	// reaching this line means the local store lacks wanted objects, so
	// "up to date" would be a lie. Same invariant as 3a's downloadAllPacks.
	if len(stems) == 0 {
		return "", fmt.Errorf("%s/packs holds no complete pack pairs; the remote is incomplete", root)
	}
	pruneStale(cacheDir, stemSet(stems))
	pm, err := buildPackMap(t, root, cacheDir, fallbackDir, stems)
	if err != nil {
		return "", err
	}

	downloaded := map[string]bool{}
	healed := false
	// heal is the ONE self-heal round: fresh listing, fresh sidecars, whole
	// map rebuilt — after it, every terminal diagnosis rests on metadata
	// downloaded this run. The round restarts automatically because all
	// planning happens at the loop top.
	heal := func() error {
		var herr error
		if stems, herr = listCompletePacks(t, root); herr != nil {
			return herr
		}
		if len(stems) == 0 {
			return fmt.Errorf("%s/packs holds no complete pack pairs; the remote is incomplete", root)
		}
		pm, herr = refreshPackMap(t, root, cacheDir, fallbackDir, stems)
		return herr
	}
	// fatalAfterHeal composes the terminal message once the one self-heal
	// round has already run. errCacheSuspect's own sentence ("this can be
	// caused by a stale or corrupt sidecar cache") would otherwise survive
	// into this message via %w's %v-equivalent text and sit right next to the
	// claim that follows, self-negating: "...caused by a stale cache...
	// ...this indicates genuine remote trouble, not a stale cache." Stripping
	// that sentence (and the separator that joins it to the rest of err's
	// text) before appending the post-heal explanation keeps the message
	// internally consistent. errors.Is(err, errCacheSuspect) compatibility is
	// deliberately not preserved here — nothing downstream inspects this
	// composed error; it is only ever returned to the caller of Fetch.
	fatalAfterHeal := func(err error) error {
		msg := err.Error()
		msg = strings.TrimSuffix(msg, ": "+errCacheSuspect.Error())
		return fmt.Errorf("%s (the sidecar metadata was already refreshed from the remote "+
			"this run; this indicates genuine remote or transport trouble)", msg)
	}

	for {
		missing, err := gitcmd.RevListMissing(gitDir, altObjects, wants)
		if err != nil {
			return "", err
		}
		if len(missing) == 0 {
			break // discovery complete
		}
		toGet, err := greedyCover(missing, pm.oidPacks, downloaded)
		if err != nil { // always errCacheSuspect (see greedyCover)
			if healed {
				return "", fatalAfterHeal(err)
			}
			healed = true
			if err := heal(); err != nil {
				return "", err
			}
			continue
		}
		for _, stem := range toGet {
			refreshed, err := downloadAndVerifyPack(t, root, incomingDir, packDir, stem, pm)
			if err != nil {
				if !errors.Is(err, errCacheSuspect) {
					return "", err // pair-corrupt fatal, or a non-heal-able failure
				}
				if healed {
					return "", fatalAfterHeal(err)
				}
				healed = true
				if err := heal(); err != nil {
					return "", err
				}
				break // restart the round: the plan predates the rebuild
			}
			downloaded[stem] = true
			if refreshed {
				// A pair-verify sidecar refresh rebuilt the map mid-round; the
				// rest of toGet was planned on the old bytes. Restart planning.
				break
			}
		}
	}

	// BEFORE install. A failure here must leave the local store untouched.
	if err := gitcmd.ConnectivityOK(gitDir, altObjects, wants); err != nil {
		return "", fmt.Errorf("the remote does not hold a complete closure for the "+
			"requested objects, so nothing was installed: %w", err)
	}

	objs, err := gitcmd.RevListNewObjects(gitDir, altObjects, wants)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(objs) == "" {
		return "", nil // already up to date
	}
	return consolidateAndInstall(gitDir, altObjects, objs)
}

func stemSet(stems []string) map[string]bool {
	m := make(map[string]bool, len(stems))
	for _, s := range stems {
		m[s] = true
	}
	return m
}

// downloadAndVerifyPack downloads one pack, checksums it against its own
// basename (the .pack ALONE — the .idx borrows the pack's name, so that
// comparison could never pass), lays the sidecar beside it (git discovers
// packs in an alternate only via the .idx), and runs pair verification — all
// of it in incomingDir, the per-fetch quarantine. Only a fully verified pair
// is published into packDir (see publishPair); nothing unverified ever exists
// there. See design doc Component 4 ("Quarantine fetch staging").
//
// refreshed reports that the sidecar was re-downloaded and the map rebuilt —
// the caller must restart its round planning.
func downloadAndVerifyPack(t transport.Transport, root, incomingDir, packDir, stem string, pm *packMap) (bool, error) {
	packName := stem + ".pack"
	inPack := filepath.Join(incomingDir, packName)
	// A healed plan may re-select this stem; ReadTo onto an existing file is
	// deliberately unpinned, so clear THIS QUARANTINE's own copy first. This
	// is not the old residue rule: it never touches packDir, and it is the
	// only cleanup downloadAndVerifyPack does — every failure below just
	// returns, leaving its bytes in incomingDir for wholesale teardown.
	if err := os.Remove(inPack); err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if err := t.ReadTo(root+"/packs/"+packName, incomingDir); err != nil {
		return false, fmt.Errorf("cannot download %s: %v — a truthful map might never have "+
			"selected this pack: %w", packName, err, errCacheSuspect)
	}
	got, err := packContentChecksum(inPack)
	if err != nil {
		return false, fmt.Errorf("cannot checksum downloaded pack %s: %v: %w",
			packName, err, errCacheSuspect)
	}
	if want := packNameChecksum(packName); got != want {
		return false, fmt.Errorf("downloaded pack %s recomputes to %s; the name is the "+
			"content checksum, so this file is not what its name claims: %w",
			packName, got, errCacheSuspect)
	}
	inIdx := filepath.Join(incomingDir, stem+".idx")
	if err := copyFile(pm.sidecars[stem], inIdx); err != nil {
		// The SOURCE here may be a cache path, and an unreadable cached
		// sidecar is cache-read trouble — heal-able, not fatal (spec: any
		// cache I/O failure degrades). Wrapping unconditionally is safe: a
		// genuine temp-dir failure wastes one heal round and then fatals.
		return false, fmt.Errorf("cannot lay sidecar beside %s: %v: %w",
			packName, err, errCacheSuspect)
	}
	refreshed := false
	if err := gitcmd.IndexPackVerify(inPack); err != nil {
		// Pair failed. The pack proved it matches its NAME, not that it is
		// well formed — so the cached sidecar is the CHEAPER suspect, not the
		// proven one. One fresh sidecar, map rebuilt, one re-verify.
		fresh, rerr := RefreshSidecar(t, root, pm.cacheDir, pm.fallbackDir, stem)
		if rerr != nil {
			return false, rerr
		}
		pm.sidecars[stem] = fresh
		if err := pm.rebuildFromSidecars(); err != nil {
			return false, err
		}
		if err := os.Remove(inIdx); err != nil && !os.IsNotExist(err) {
			return false, err
		}
		if err := copyFile(fresh, inIdx); err != nil {
			return false, err
		}
		if err := gitcmd.IndexPackVerify(inPack); err != nil {
			return false, fmt.Errorf("pack pair %s.{pack,idx} fails verification even with a "+
				"freshly downloaded index; the pair is corrupt, member undetermined: %w", stem, err)
		}
		refreshed = true
	}
	// PUBLISH: only a fully verified pair leaves quarantine.
	if err := publishPair(incomingDir, packDir, stem); err != nil {
		return refreshed, err
	}
	return refreshed, nil
}

// publishPair moves a VERIFIED pair from quarantine into the alternate's pack
// dir: .pack first, then .idx. The .idx rename is the pair's commit point —
// git discovers packs only via their index, so a pack whose idx has not
// landed is invisible to the traversal — which is what makes the two renames
// an atomic publication from the reader's side. Go's os.Rename replaces an
// existing destination file on Windows — the same fact idxcache.go's
// installIntoCache already relies on, and which the mid-round pair-refresh
// exercises via RefreshSidecar's cache write — but publishPair itself does
// not depend on it: greedyCover never re-selects an already-downloaded stem,
// and packDir starts empty every fetch (a fresh os.MkdirTemp), so this
// function's destination is never occupied. Overwrite is not relied on here,
// and is not a hazard either.
//
// inPack was just held open by gitcmd.IndexPackVerify's `index-pack
// --verify`; on its ErrWaitDelay path a grandchild may still hold a handle
// on it, which can turn this rename into a Windows sharing-violation error —
// left fatal-and-loud on purpose (publishPair's error is never wrapped in
// errCacheSuspect, so Fetch spends none of its one heal round on it): a held
// file handle is local trouble a re-download cannot fix.
//
// Extracted so the ordering is unit-testable (TestPublishPairRenamesPackBeforeIdx).
func publishPair(incomingDir, packDir, stem string) error {
	if err := os.Rename(filepath.Join(incomingDir, stem+".pack"),
		filepath.Join(packDir, stem+".pack")); err != nil {
		return err
	}
	return os.Rename(filepath.Join(incomingDir, stem+".idx"),
		filepath.Join(packDir, stem+".idx"))
}

// copyFile copies src to dst. out is closed EXACTLY ONCE on each path, with
// no `defer out.Close()` alongside: a deferred second Close fired after the
// explicit one, got os.ErrClosed, and discarded it — harmless until someone
// adds error handling to the defer, at which point every successful copy
// starts reporting "file already closed". Close's error is worth returning:
// on a filesystem that defers write errors it is the only place a failed
// flush surfaces at all.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// consolidateAndInstall builds ONE pack from the new objects and installs it
// with an adjacent .keep. Git retains only the FIRST lock response, so a
// multi-pack install could not be protected.
//
// The .keep is written BEFORE the caller reports the lock, and git removes it
// once it has updated refs. If this process dies in between, the .keep
// remains and nothing will reclaim the pack — an inert residue that costs
// disk until a human removes the file.
func consolidateAndInstall(gitDir, altObjects, objs string) (string, error) {
	// Ask git where its PACKS actually belong — not merely where its git dir
	// is. Those are the same question in an ordinary repo, but NOT in a
	// LINKED WORKTREE (`git worktree add`): there, --git-dir answers with the
	// per-worktree ADMIN directory (<main>/.git/worktrees/<name>), which
	// holds HEAD and the index but has NO object store of its own — git
	// resolves objects through the worktree's COMMON dir instead
	// (<main>/.git/objects), and never looks in the admin dir's objects/ at
	// all. Asking --git-dir and packing into "<admin-dir>/objects/pack" (what
	// this function used to do) would therefore report a successful fetch
	// while writing objects nowhere git will ever look — with
	// check-connectivity, connectivity-ok tells the calling git to skip its
	// own check and update refs anyway, producing refs that point at objects
	// git cannot see: fail-OPEN, exactly the class of bug this task's whole
	// verify-before-install posture exists to prevent. --git-path objects/pack
	// asks the right question instead: confirmed empirically (see the fix
	// report) that it resolves through the commondir indirection and returns
	// the correct location in every layout — ordinary repo, bare repo, and
	// linked worktree alike.
	out, code, err := gitcmd.RevParse(gitDir, "--git-path", "objects/pack")
	if err != nil {
		return "", fmt.Errorf("cannot locate the pack directory for %s: %w", gitDir, err)
	}
	if code != 0 {
		return "", fmt.Errorf("cannot locate the pack directory for %s: rev-parse "+
			"--git-path objects/pack exited %d: %s", gitDir, code, out)
	}
	// RevParse's result is git()'s COMBINED stdout+stderr (trimmed), not
	// stdout alone. A single warning line merged in alongside a genuinely
	// successful answer — an inaccessible ~/.gitconfig, a broken ref notice —
	// would otherwise be trusted as part of the path outright. See
	// validateObjectsPackPath for the shape this must match instead.
	if err := validateObjectsPackPath(out); err != nil {
		return "", fmt.Errorf("cannot locate the pack directory for %s: %w", gitDir, err)
	}
	// Relative answers (an ordinary or bare repo) resolve against the -C
	// directory RevParse ran with, i.e. gitDir itself — confirmed empirically
	// (see the fix report). A linked worktree's answer is already absolute
	// (it points into the MAIN repo, not gitDir), so this join is a no-op
	// for that case.
	realPack := out
	if !filepath.IsAbs(realPack) {
		realPack = filepath.Join(gitDir, realPack)
	}
	// ALWAYS resolved to absolute from here on, regardless of whether gitDir
	// itself arrived absolute or relative — confirmed live (Stage 3a gate,
	// task 7) that this cannot be left to the caller's hygiene. git commonly
	// invokes this helper with a RELATIVE GIT_DIR (".git" is its own
	// default), and when gitDir is relative, realPack above is only correct
	// relative to THIS PROCESS's cwd. PackObjectsFromList below hands it as
	// an argument to a SECOND `git -C gitDir ...` subprocess — and -C changes
	// the effective cwd for resolving relative ARGUMENTS too, so that second
	// subprocess would resolve the already-gitDir-relative realPack a SECOND
	// time, relative to gitDir again, doubling the prefix
	// (".git/.git/objects/pack/..."). filepath.Abs resolves relative to this
	// process's own cwd — the same resolution realPack already implicitly
	// depends on — so it cannot change realPack's MEANING, only make that
	// meaning unambiguous to a subprocess that resolves paths in a different
	// context. Fetch.go is a library entry point, not merely main.go's
	// caller, and this package's own tests call Fetch directly with a
	// relative gitDir, so this must not silently depend on main.go having
	// already resolved gitDir to absolute upstream.
	//
	// Assigned only AFTER the error check: filepath.Abs returns "" on failure,
	// so assigning straight into realPack would make the message below name an
	// empty path rather than the one that could not be resolved.
	absPack, err := filepath.Abs(realPack)
	if err != nil {
		return "", fmt.Errorf("cannot resolve pack directory %s to an absolute path: %w", realPack, err)
	}
	realPack = absPack
	if err := os.MkdirAll(realPack, 0o700); err != nil {
		return "", err
	}

	// gitcmd.PackObjectsFromList owns the exec site (WaitDelay, the
	// ErrWaitDelay truncation guard, and the multi-pack-rejection check) —
	// see its doc for why this must not be reimplemented here: a bare
	// exec.Command with no WaitDelay can hang forever on a grandchild (e.g. a
	// core.fsmonitor daemon) holding the output pipe open, and it would hang
	// AFTER pack-objects has already written the pack into the live
	// objects/pack computed above — unkept, unknown to git, permanent if the
	// process is killed.
	name, err := gitcmd.PackObjectsFromList(gitDir, altObjects, objs, filepath.Join(realPack, "pack"))
	if err != nil {
		return "", err
	}

	stem := filepath.Join(realPack, "pack-"+name)
	for _, ext := range []string{".pack", ".idx"} {
		if _, err := os.Stat(stem + ext); err != nil {
			return "", fmt.Errorf("pack-objects reported %s but %s is missing: %w", name, stem+ext, err)
		}
	}
	// The .keep lands AFTER the pack it protects, and that ordering leaves a
	// window: a `git gc` running concurrently, between pack-objects writing
	// the pack and this write completing, sees a pack no ref reaches and no
	// .keep protects, and may reclaim it. The fetch would then report success
	// for objects that are gone.
	//
	// It is not expressible any other way with pack-objects, which chooses the
	// pack's name itself (the name is its content checksum) and only prints it
	// once the pack is on disk — so there is no name to write a .keep under
	// beforehand. index-pack --stdin has a --keep flag that closes this, but
	// it takes a pack on stdin rather than building one from an object list,
	// which is the whole job here. The window is narrow, requires a concurrent
	// gc in the destination repo, and the failure is loud (git reports the
	// missing objects) rather than silent corruption — recorded so it is
	// understood as an accepted cost rather than mistaken for an oversight.
	keep := stem + ".keep"
	if err := os.WriteFile(keep, []byte("git-remote-proton fetch\n"), 0o644); err != nil {
		return "", fmt.Errorf("cannot write %s: %w", keep, err)
	}
	return keep, nil
}

// validateObjectsPackPath guards the untrusted combined-output value RevParse
// hands back before consolidateAndInstall treats it as a filesystem path (see
// the comment at its call site for the mechanism this defends against). A
// newline anywhere is refused outright — a genuine --git-path answer is
// always one line, so any newline proves something else (a git warning) was
// merged into it.
//
// Beyond that, v must actually look like an objects/pack answer: either the
// literal "objects/pack" (a bare repo, whose git dir IS its own root) or a
// path ending in "/objects/pack" (every other layout — an ordinary repo's
// ".git/objects/pack", or a linked worktree's absolute path resolved through
// the common dir). Confirmed empirically across all three layouts, on this
// platform, that --git-path always uses forward slashes, even for an
// absolute Windows path (see the fix report).
//
// There is deliberately no "confirm it already exists" fallback here, unlike
// the --git-dir check this replaced: this function's entire caller exists to
// MkdirAll objects/pack, which on a repo that has never held a pack does not
// exist yet by definition — requiring it to pre-exist would make the first
// fetch into a fresh repo fail.
func validateObjectsPackPath(v string) error {
	if strings.ContainsAny(v, "\r\n") {
		return fmt.Errorf("rev-parse --git-path objects/pack returned more than one line "+
			"(%q); this usually means a git warning was merged into the combined output, and "+
			"the result cannot be trusted as a path", v)
	}
	if v == "objects/pack" || strings.HasSuffix(v, "/objects/pack") {
		return nil
	}
	return fmt.Errorf("rev-parse --git-path objects/pack returned %q, which does not look "+
		"like a pack directory; refusing to treat it as one", v)
}
