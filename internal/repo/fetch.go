package repo

import (
	"fmt"
	"os"
	"os/exec"
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
	// Ask git where its object store is rather than guessing from the presence
	// of a .git directory. Guessing misfires on a bare repo, on a worktree
	// whose .git is a file, and on a normal repo that happens to have no
	// objects/pack yet — in that last case it would create one in the WORKING
	// TREE, which is silently wrong.
	gitDirAbs, code, err := gitcmd.RevParse(gitDir, "--git-dir")
	if err != nil || code != 0 {
		return "", fmt.Errorf("cannot locate the git directory for %s: %v", gitDir, err)
	}
	if !filepath.IsAbs(gitDirAbs) {
		gitDirAbs = filepath.Join(gitDir, gitDirAbs)
	}
	realPack := filepath.Join(gitDirAbs, "objects", "pack")
	if err := os.MkdirAll(realPack, 0o700); err != nil {
		return "", err
	}

	cmd := exec.Command("git", "-C", gitDir,
		"-c", "pack.packSizeLimit=0",
		"pack-objects", "--no-thin", "--index-version=2", "-q",
		filepath.Join(realPack, "pack"))
	cmd.Env = append(os.Environ(), "GIT_ALTERNATE_OBJECT_DIRECTORIES="+altObjects)
	cmd.Stdin = strings.NewReader(objs)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pack-objects: %s: %w", strings.TrimSpace(stderr.String()), err)
	}
	name := strings.TrimSpace(stdout.String())
	if strings.ContainsAny(name, " \t\r\n") {
		return "", fmt.Errorf("pack-objects emitted more than one pack (%q); the "+
			"one-pack invariant the lock response depends on is broken", name)
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
