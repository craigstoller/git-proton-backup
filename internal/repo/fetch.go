package repo

import (
	"fmt"
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
// Stage 3a downloads every pack rather than discovering which are needed.
// That is correct by construction — every pack this helper writes is
// --no-thin and self-contained, so their union is a superset of any closure.
// Selective discovery is Stage 3b.
func Fetch(t transport.Transport, root, gitDir string, wants []string) (string, error) {
	if err := requireMarker(t, root); err != nil {
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
	if err := os.MkdirAll(packDir, 0o700); err != nil {
		return "", err
	}

	if err := downloadAllPacks(t, root, packDir); err != nil {
		return "", err
	}
	if err := verifyDownloadedPacks(packDir); err != nil {
		return "", err
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

// requireMarker is the read-only half of Bootstrap's check: the marker must be
// present AND recognised. Absent or unrecognised is a hard refusal — the
// helper never guesses whether a folder is one of its repos, and a fetch is
// certainly not licence to initialise one.
func requireMarker(t transport.Transport, root string) error {
	marker := root + "/" + MarkerName
	if _, ok, err := t.Stat(marker); err != nil {
		return fmt.Errorf("stat %s: %w", marker, err)
	} else if !ok {
		return fmt.Errorf("refusing to fetch from %s: no %s — it is not a git-remote-proton repo",
			root, MarkerName)
	}
	// checkMarker takes the marker's own PATH, not the repo root — passing the
	// root would try to download a directory.
	return checkMarker(t, marker)
}

func downloadAllPacks(t transport.Transport, root, packDir string) error {
	nodes, err := t.List(root + "/packs")
	if err != nil {
		return fmt.Errorf("cannot list %s/packs: %w", root, err)
	}
	got := 0
	for _, n := range nodes {
		if n.IsDir || !(strings.HasSuffix(n.Name, ".pack") || strings.HasSuffix(n.Name, ".idx")) {
			continue
		}
		if err := t.ReadTo(root+"/packs/"+n.Name, packDir); err != nil {
			return fmt.Errorf("cannot download %s: %w", n.Name, err)
		}
		if strings.HasSuffix(n.Name, ".pack") {
			got++
		}
	}
	if got == 0 {
		return fmt.Errorf("%s/packs holds no packs; the remote is incomplete", root)
	}
	return nil
}

// verifyDownloadedPacks checks each pair PER MEMBER. Only the .pack is
// checksummed against its basename — a .idx borrows the pack's name, so
// hashing the index and comparing it to that name could never pass. The pair
// is validated by index-pack --verify instead. Same asymmetry as the push
// side's publishPack/publishIdx.
func verifyDownloadedPacks(packDir string) error {
	entries, err := os.ReadDir(packDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".pack") {
			continue
		}
		p := filepath.Join(packDir, e.Name())
		// The repo package already owns both halves of this check, from the
		// push side: packContentChecksum(path) (string, error) recomputes the
		// pack's content hash, and packNameChecksum(base) string extracts the
		// hash the basename claims. There is no single checkPackChecksum.
		got, err := packContentChecksum(p)
		if err != nil {
			return fmt.Errorf("cannot checksum downloaded pack %s: %w", e.Name(), err)
		}
		if want := packNameChecksum(e.Name()); got != want {
			return fmt.Errorf("downloaded pack %s recomputes to %s; the name is the "+
				"content checksum, so this file is not what its name claims", e.Name(), got)
		}
		if _, err := os.Stat(strings.TrimSuffix(p, ".pack") + ".idx"); err != nil {
			return fmt.Errorf("downloaded pack %s has no adjacent .idx", e.Name())
		}
		if err := gitcmd.IndexPackVerify(p); err != nil {
			return fmt.Errorf("downloaded pair failed verification: %w", err)
		}
	}
	return nil
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
