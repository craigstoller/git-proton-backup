package gitcmd

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newRepo builds a real, throwaway git repo under t.TempDir() with one
// commit. Every step's error is checked (the brief's original version
// ignored errors from the setup commands and from rev-parse, then sliced
// the rev-parse output to [:40] unconditionally — which panics on an empty
// result instead of failing with a useful message). A broken test helper
// must report itself via t.Fatal, not panic.
func newRepo(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	for _, a := range [][]string{
		{"init", "-qb", "main"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"},
	} {
		if err := exec.Command("git", append([]string{"-C", d}, a...)...).Run(); err != nil {
			t.Fatalf("git %v: %v", a, err)
		}
	}
	if err := os.WriteFile(d+"/a.txt", []byte("one"), 0o644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	if err := exec.Command("git", "-C", d, "add", ".").Run(); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := exec.Command("git", "-C", d, "commit", "-qm", "c1").Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	return d
}

// headOf returns the full HEAD sha of the repo at d, failing the test
// loudly if rev-parse errors or returns something other than a 40-char sha
// (rather than panicking on a short slice, as the brief's inline version
// would).
func headOf(t *testing.T, d string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", d, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	sha := strings.TrimSpace(string(out))
	if len(sha) != 40 {
		t.Fatalf("rev-parse HEAD returned %q, want a 40-char sha", sha)
	}
	return sha
}

func TestObjectTypeAndAncestry(t *testing.T) {
	d := newRepo(t)
	head := headOf(t, d)

	if got, err := ObjectType(d, head); err != nil || got != "commit" {
		t.Fatalf("ObjectType = %q, %v; want commit", got, err)
	}
	if !HasObject(d, head) {
		t.Error("HasObject must find HEAD")
	}
	if ok, err := IsAncestor(d, head, head); !ok || err != nil {
		t.Errorf("IsAncestor(head, head) = %v, %v; want true, nil (a commit is its own ancestor)", ok, err)
	}
}

// TestHasObjectMissing covers the negative path Task 10 depends on: a sha
// that is well-formed but does not exist in the repo must report false, not
// merely be left untested alongside the positive "finds HEAD" case above.
func TestHasObjectMissing(t *testing.T) {
	d := newRepo(t)
	missing := "0000000000000000000000000000000000000000"
	if HasObject(d, missing) {
		t.Error("HasObject must not report a nonexistent object as present")
	}
}

// TestIsAncestorNonAncestor covers the genuine negative answer using two
// commits that actually diverge, not just the trivial self-ancestor case:
// a commit on branchA and a commit on branchB, both built on the same base
// commit from newRepo, are ancestors of neither each other.
func TestIsAncestorNonAncestor(t *testing.T) {
	d := newRepo(t)

	commitOn := func(branch, file, content string) string {
		if err := exec.Command("git", "-C", d, "checkout", "-qb", branch).Run(); err != nil {
			t.Fatalf("checkout %s: %v", branch, err)
		}
		if err := os.WriteFile(d+"/"+file, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
		if err := exec.Command("git", "-C", d, "add", ".").Run(); err != nil {
			t.Fatalf("add: %v", err)
		}
		if err := exec.Command("git", "-C", d, "commit", "-qm", branch).Run(); err != nil {
			t.Fatalf("commit %s: %v", branch, err)
		}
		return headOf(t, d)
	}

	commitA := commitOn("branchA", "b.txt", "A")
	if err := exec.Command("git", "-C", d, "checkout", "-q", "main").Run(); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	commitB := commitOn("branchB", "c.txt", "B")

	if ok, err := IsAncestor(d, commitA, commitB); ok || err != nil {
		t.Fatalf("IsAncestor(A, B) = %v, %v; want false, nil for genuinely divergent commits", ok, err)
	}
	if ok, err := IsAncestor(d, commitB, commitA); ok || err != nil {
		t.Fatalf("IsAncestor(B, A) = %v, %v; want false, nil for genuinely divergent commits", ok, err)
	}
}

// TestIsAncestorMalformedObjectIsError proves the new error path added in
// this fix round is actually reachable: merge-base --is-ancestor exits 128
// (verified against a real git invocation), not 1, for a malformed object
// name, so this must come back as a non-nil error rather than a confirmed
// (false, nil). If IsAncestor were reverted to testing only `code == 0`,
// this would observe (false, nil) and fail — that is what makes this a real
// regression test rather than one that could never fail.
func TestIsAncestorMalformedObjectIsError(t *testing.T) {
	d := newRepo(t)
	head := headOf(t, d)

	ok, err := IsAncestor(d, "not-a-real-sha-0000000000000000000000", head)
	if err == nil {
		t.Fatalf("want a non-nil error for a malformed object, got ok=%v err=nil", ok)
	}
	if ok {
		t.Fatalf("want ok=false alongside the error, got true")
	}
}

// TestWritePack covers the function the dispatch flagged as having no
// coverage at all in the brief: the real-pack path (asserting the returned
// .pack and .idx actually exist on disk) and the "nothing to send" path,
// which must stay a legitimate, distinct, non-error outcome.
func TestWritePack(t *testing.T) {
	d := newRepo(t)
	head := headOf(t, d)

	t.Run("real pack", func(t *testing.T) {
		outDir := t.TempDir()
		packPath, idxPath, err := WritePack(d, head, nil, outDir)
		if err != nil {
			t.Fatalf("WritePack: %v", err)
		}
		if packPath == "" || idxPath == "" {
			t.Fatalf("want non-empty paths for a non-empty range, got pack=%q idx=%q", packPath, idxPath)
		}
		if _, err := os.Stat(packPath); err != nil {
			t.Errorf("returned pack path does not exist on disk: %v", err)
		}
		if _, err := os.Stat(idxPath); err != nil {
			t.Errorf("returned idx path does not exist on disk: %v", err)
		}
	})

	t.Run("nothing to send", func(t *testing.T) {
		outDir := t.TempDir()
		// want == the only have: rev-list --objects head ^head is empty.
		packPath, idxPath, err := WritePack(d, head, []string{head}, outDir)
		if err != nil {
			t.Fatalf("WritePack: %v", err)
		}
		if packPath != "" || idxPath != "" {
			t.Fatalf("want empty paths when there is nothing to send, got pack=%q idx=%q", packPath, idxPath)
		}
	})
}

// chdirForTest changes the process's working directory to dir for the
// duration of the test and restores it afterward via t.Cleanup.
//
// Not testing.Chdir: that requires Go 1.24 and this module's go.mod
// deliberately floors at go 1.22 (see the identical helpers in
// internal/repo/repo_test.go and cmd/git-remote-proton/main_test.go, which
// carry the full reasoning). No test in this package calls t.Parallel(), so
// the process-wide cwd this mutates is not at risk of a concurrent sibling
// reading it mid-change.
func chdirForTest(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%s): %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatalf("restore Chdir(%s): %v", orig, err)
		}
	})
}

// RED. WritePack's outDir is handed to PackObjectsFromList as an outStem, and
// that runs `git -C gitDir pack-objects ... <outStem>` — where -C changes the
// directory relative ARGUMENTS resolve against too. A RELATIVE outDir is
// therefore resolved against gitDir by the subprocess, not against this
// process's cwd as the caller (and WritePack's own os.Stat guards below)
// assume: the same path-doubling class the Stage 3a live gate hit in
// repo.consolidateAndInstall, untreated on this side.
//
// Every current caller passes an absolute temp dir, so it is latent — but
// WritePack and consolidateAndInstall are now two callers of ONE exec site
// holding different path disciplines, which is exactly how a latent hazard
// becomes a live one. Absolutising at the top of WritePack means the exec site
// only ever sees paths with a single, unambiguous resolution.
//
// Pre-fix this failed with pack-objects unable to write into
// "<gitDir>/out/pack-...", which does not exist.
func TestWritePackResolvesARelativeOutDirAgainstTheProcessCwd(t *testing.T) {
	d := newRepo(t)
	head := headOf(t, d)

	work := t.TempDir()
	chdirForTest(t, work)
	if err := os.Mkdir("out", 0o700); err != nil {
		t.Fatalf("mkdir out: %v", err)
	}

	packPath, idxPath, err := WritePack(d, head, nil, "out") // RELATIVE, and not under gitDir
	if err != nil {
		t.Fatalf("WritePack with a relative outDir: %v", err)
	}
	if packPath == "" || idxPath == "" {
		t.Fatalf("want real paths, got pack=%q idx=%q", packPath, idxPath)
	}
	if !filepath.IsAbs(packPath) || !filepath.IsAbs(idxPath) {
		t.Errorf("returned paths must be absolute so a caller cannot re-resolve them in "+
			"another context: pack=%q idx=%q", packPath, idxPath)
	}
	for _, p := range []string{packPath, idxPath} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("returned path does not exist on disk: %v", err)
		}
	}
	// The pack must land under the cwd's out/, NOT under gitDir's.
	if got := countPacks(t, filepath.Join(work, "out")); got != 1 {
		t.Errorf("want 1 pack under the process cwd's out/, got %d", got)
	}
	if _, err := os.Stat(filepath.Join(d, "out")); err == nil {
		t.Error("a relative outDir must not be resolved against gitDir: found out/ inside the repo")
	}
}

// bigRepo builds a repo with enough INCOMPRESSIBLE data to exceed git's
// minimum pack-size limit. git clamps pack.packSizeLimit to 1 MiB, so a small
// repo cannot demonstrate the pin at all — the first draft of this test set
// 512 bytes and proved nothing.
func bigRepo(t *testing.T) string {
	t.Helper()
	d := newRepo(t)
	for i := 0; i < 4; i++ {
		buf := make([]byte, 1<<20) // 1 MiB, random => no compression
		if _, err := rand.Read(buf); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, fmt.Sprintf("blob%d.bin", i)), buf, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, a := range [][]string{{"add", "."}, {"commit", "-qm", "big"}} {
		if err := exec.Command("git", append([]string{"-C", d}, a...)...).Run(); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func countPacks(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".pack") {
			n++
		}
	}
	return n
}

// RED. Without the -c pack.packSizeLimit=0 pin, a configured limit splits the
// output and WritePack's single-name parse produces a path that does not exist.
func TestWritePackIgnoresAHostilePackSizeLimit(t *testing.T) {
	d := bigRepo(t)
	head := headOf(t, d)
	if err := exec.Command("git", "-C", d, "config", "pack.packSizeLimit", "1m").Run(); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	packPath, _, err := WritePack(d, head, nil, out)
	if err != nil {
		t.Fatalf("WritePack must survive a configured packSizeLimit: %v", err)
	}
	if packPath == "" {
		t.Fatal("expected a pack")
	}
	if got := countPacks(t, out); got != 1 {
		t.Errorf("want exactly 1 pack despite packSizeLimit, got %d", got)
	}
}

// RED. Without --index-version=2, a configured pack.indexVersion=1 wins.
func TestWritePackPinsIndexVersion2(t *testing.T) {
	d := newRepo(t)
	head := headOf(t, d)
	if err := exec.Command("git", "-C", d, "config", "pack.indexVersion", "1").Run(); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	_, idxPath, err := WritePack(d, head, nil, out)
	if err != nil {
		t.Fatalf("WritePack: %v", err)
	}
	raw, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatal(err)
	}
	// A v2 index starts with the magic \377tOc then a 4-byte version 2.
	// A v1 index has no magic at all — it starts straight into the fanout.
	want := []byte{0xff, 0x74, 0x4f, 0x63, 0, 0, 0, 2}
	if len(raw) < 8 || !bytes.Equal(raw[:8], want) {
		t.Errorf("idx does not start with the v2 header; got % x", raw[:min(8, len(raw))])
	}
}

// twoCommitRepo returns a repo with two commits and the sha of each.
//
// The first commit deliberately does NOT reuse newRepo(t): newRepo's content
// and message ("a.txt" = "one", "c1") are fixed, so two independently created
// newRepo commits share the same tree, parent (none), author, and message —
// they can differ ONLY in commit timestamp, and git records that at
// one-second resolution. Two newRepo calls landing in the same wall-clock
// second are therefore byte-identical and produce the SAME sha.
// TestConnectivityOKDetectsAnIncompleteClosure builds this repo's `first`
// commit alongside a separate `newRepo(t)` destination ("empty"); if the two
// collided, "empty" would already contain what it thinks is the missing
// parent, the walk would never need the alt pack at all, and the fail-closed
// half of that test would pass for the wrong reason — on a genuinely broken
// implementation. Giving this commit distinguishing file content and message
// makes that collision structurally impossible rather than merely unlikely
// on a slow-enough machine. (The same one-second-resolution hazard applies to
// emptyGitRepo in a later task.)
func twoCommitRepo(t *testing.T) (dir, first, second string) {
	t.Helper()
	d := t.TempDir()
	for _, a := range [][]string{
		{"init", "-qb", "main"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"},
	} {
		if err := exec.Command("git", append([]string{"-C", d}, a...)...).Run(); err != nil {
			t.Fatalf("git %v: %v", a, err)
		}
	}
	if err := os.WriteFile(filepath.Join(d, "a.txt"), []byte("src-one"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, a := range [][]string{{"add", "."}, {"commit", "-qm", "src c1"}} {
		if err := exec.Command("git", append([]string{"-C", d}, a...)...).Run(); err != nil {
			t.Fatal(err)
		}
	}
	first = headOf(t, d)

	if err := os.WriteFile(filepath.Join(d, "b.txt"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, a := range [][]string{{"add", "."}, {"commit", "-qm", "c2"}} {
		if err := exec.Command("git", append([]string{"-C", d}, a...)...).Run(); err != nil {
			t.Fatal(err)
		}
	}
	return d, first, headOf(t, d)
}

// altObjectsWith builds an alternate object directory in git's own layout and
// places the given pack (and its .idx) inside it.
func altObjectsWith(t *testing.T, packPath string) string {
	t.Helper()
	alt := filepath.Join(t.TempDir(), "objects")
	packDir := filepath.Join(alt, "pack")
	if err := os.MkdirAll(packDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, src := range []string{packPath, strings.TrimSuffix(packPath, ".pack") + ".idx"} {
		b, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(packDir, filepath.Base(src)), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return alt
}

// RED. ConnectivityOK does not exist. It must PASS on a complete closure and
// FAIL on an incomplete one — the failing half is the whole point.
func TestConnectivityOKDetectsAnIncompleteClosure(t *testing.T) {
	src, first, second := twoCommitRepo(t)

	// A pack of only the SECOND commit: its parent is deliberately absent.
	partial, _, err := WritePack(src, second, []string{first}, t.TempDir())
	if err != nil || partial == "" {
		t.Fatalf("WritePack partial: %v", err)
	}
	// A pack of the whole history.
	full, _, err := WritePack(src, second, nil, t.TempDir())
	if err != nil || full == "" {
		t.Fatalf("WritePack full: %v", err)
	}

	empty := newRepo(t) // a repo that shares no objects with src

	if err := ConnectivityOK(empty, altObjectsWith(t, partial), []string{second}); err == nil {
		t.Error("a pack missing the want's parent must NOT report connectivity ok")
	}
	if err := ConnectivityOK(empty, altObjectsWith(t, full), []string{second}); err != nil {
		t.Errorf("a complete closure must report connectivity ok: %v", err)
	}
}

// RED. RevListNewObjects does not exist. --not --all is what stops an
// incremental fetch reconsolidating history the repo already has.
func TestRevListNewObjectsExcludesWhatTheRepoAlreadyHas(t *testing.T) {
	src, _, second := twoCommitRepo(t)
	full, _, err := WritePack(src, second, nil, t.TempDir())
	if err != nil || full == "" {
		t.Fatalf("WritePack: %v", err)
	}
	alt := altObjectsWith(t, full)

	// Into an empty repo: everything is new.
	empty := newRepo(t)
	objs, err := RevListNewObjects(empty, alt, []string{second})
	if err != nil {
		t.Fatalf("RevListNewObjects: %v", err)
	}
	if !strings.Contains(objs, second) {
		t.Error("an empty repo must be told about the want itself")
	}

	// Into the source repo, which already holds every object: nothing is new.
	objs, err = RevListNewObjects(src, alt, []string{second})
	if err != nil {
		t.Fatalf("RevListNewObjects: %v", err)
	}
	if strings.TrimSpace(objs) != "" {
		t.Errorf("a repo that already has everything must yield no new objects, got:\n%s", objs)
	}
}

// TestRevParseAcceptsMultipleArgs covers fix round 2's variadic change: rev-
// parse's `--git-path <path>` needs the path as a SEPARATE argv element
// (confirmed empirically that `--git-path=objects/pack` is not recognised as
// a flag at all and is echoed back literally rather than resolved), so
// RevParse must be able to pass more than one argument through. Also pins
// that the pre-existing single-arg call shape (RevParse(gitDir, rev)) still
// compiles and behaves identically, since push.go's resolve() depends on it.
func TestRevParseAcceptsMultipleArgs(t *testing.T) {
	d := newRepo(t)
	head := headOf(t, d)

	// The pre-existing single-arg shape still works.
	if got, code, err := RevParse(d, "HEAD"); err != nil || code != 0 || got != head {
		t.Fatalf("RevParse(d, %q) = %q, %d, %v; want %q, 0, nil", "HEAD", got, code, err, head)
	}

	// The new multi-arg shape this fix round exists for.
	got, code, err := RevParse(d, "--git-path", "objects/pack")
	if err != nil || code != 0 {
		t.Fatalf("RevParse(d, --git-path, objects/pack): %q, %d, %v", got, code, err)
	}
	if got != "objects/pack" && !strings.HasSuffix(got, "/objects/pack") {
		t.Errorf("--git-path objects/pack = %q, want it to end in objects/pack", got)
	}
}

// RED. SymbolicRef does not exist. A detached HEAD is ordinary, not an error.
func TestSymbolicRef(t *testing.T) {
	d := newRepo(t)
	got, err := SymbolicRef(d, "HEAD")
	if err != nil {
		t.Fatalf("SymbolicRef: %v", err)
	}
	if got != "refs/heads/main" {
		t.Errorf("HEAD = %q, want refs/heads/main", got)
	}

	if err := exec.Command("git", "-C", d, "checkout", "-q", "--detach").Run(); err != nil {
		t.Fatal(err)
	}
	got, err = SymbolicRef(d, "HEAD")
	if err != nil {
		t.Errorf("a detached HEAD must not be an error: %v", err)
	}
	if got != "" {
		t.Errorf("a detached HEAD must yield \"\", got %q", got)
	}
}

// RED (fix round 1, I1). PackObjectsFromList does not exist yet — it is the
// extraction of WritePack's own exec site so repo.consolidateAndInstall gets
// the same WaitDelay / ErrWaitDelay / multi-pack-rejection guards WritePack
// already has, instead of a second copy of them (which is exactly what
// consolidateAndInstall had before this fix round: an inlined pack-objects
// call with none of those guards).
func TestPackObjectsFromListBuildsFromGitDirAlone(t *testing.T) {
	d := newRepo(t)
	head := headOf(t, d)
	objs, code, err := git(d, "rev-list", "--objects", head)
	if code != 0 || err != nil {
		t.Fatalf("rev-list: %v (code %d)", err, code)
	}

	out := t.TempDir()
	name, err := PackObjectsFromList(d, "", objs, filepath.Join(out, "pack"))
	if err != nil {
		t.Fatalf("PackObjectsFromList: %v", err)
	}
	packPath := filepath.Join(out, "pack-"+name+".pack")
	if _, err := os.Stat(packPath); err != nil {
		t.Errorf("returned name does not name a pack that exists on disk: %v", err)
	}
	if err := IndexPackVerify(packPath); err != nil {
		t.Errorf("built pack must verify: %v", err)
	}
}

// RED (fix round 1, I1). Covers the OTHER caller's shape directly:
// repo.consolidateAndInstall packs objects that live ONLY in a downloaded,
// not-yet-installed alternate store — exactly what altObjects exists for, and
// exactly the path that had no guards before this fix round.
func TestPackObjectsFromListSplicesInAnAlternate(t *testing.T) {
	src, _, second := twoCommitRepo(t)
	full, _, err := WritePack(src, second, nil, t.TempDir())
	if err != nil || full == "" {
		t.Fatalf("WritePack: %v", err)
	}
	alt := altObjectsWith(t, full)

	empty := newRepo(t) // shares no objects with src
	objs, err := RevListNewObjects(empty, alt, []string{second})
	if err != nil {
		t.Fatalf("RevListNewObjects: %v", err)
	}

	out := t.TempDir()
	name, err := PackObjectsFromList(empty, alt, objs, filepath.Join(out, "pack"))
	if err != nil {
		t.Fatalf("PackObjectsFromList: %v", err)
	}
	packPath := filepath.Join(out, "pack-"+name+".pack")
	if err := IndexPackVerify(packPath); err != nil {
		t.Errorf("pack built from an alternate must verify: %v", err)
	}
}

// RED. IndexPackVerify does not exist yet.
func TestIndexPackVerifyAcceptsGoodPairRejectsCorrupt(t *testing.T) {
	d := newRepo(t)
	head := headOf(t, d)
	out := t.TempDir()
	packPath, idxPath, err := WritePack(d, head, nil, out)
	if err != nil || packPath == "" {
		t.Fatalf("WritePack: %v", err)
	}
	if err := IndexPackVerify(packPath); err != nil {
		t.Fatalf("a freshly written pair must verify: %v", err)
	}

	// Same index, a pack corrupted in its body.
	bad := t.TempDir()
	raw, err := os.ReadFile(packPath)
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)/2] ^= 0xff
	badPack := filepath.Join(bad, filepath.Base(packPath))
	if err := os.WriteFile(badPack, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	idxRaw, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, filepath.Base(idxPath)), idxRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := IndexPackVerify(badPack); err == nil {
		t.Error("a corrupted pack must not verify")
	}
}

// makeIdxFixture builds a tiny repo and packs its full closure, returning the
// idx path and the object IDs the pack must contain. indexVersion is passed to
// index-pack when rebuilding the index, so the same fixture pins v1 and v2.
func makeIdxFixture(t *testing.T, indexVersion string) (idxPath string, oids map[string]bool) {
	t.Helper()
	d := t.TempDir()
	run := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", d}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-qb", "main")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(d, "f.txt"), []byte("show-index fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "f.txt")
	run("commit", "-qm", "c")
	sha := run("rev-parse", "HEAD")

	out := t.TempDir()
	packPath, gotIdx, err := WritePack(d, sha, nil, out)
	if err != nil || packPath == "" {
		t.Fatalf("WritePack: %v", err)
	}
	idxPath = gotIdx
	if indexVersion == "1" {
		// Rebuild the index as v1: Push deliberately accepts a valid remote v1
		// .idx it did not write, so the map builder must read it too.
		if err := os.Remove(idxPath); err != nil {
			t.Fatal(err)
		}
		if o, err := exec.Command("git", "index-pack", "--index-version=1", packPath).CombinedOutput(); err != nil {
			t.Fatalf("index-pack --index-version=1: %v: %s", err, o)
		}
	}
	oids = map[string]bool{}
	for _, line := range strings.Split(run("rev-list", "--objects", sha), "\n") {
		if f := strings.Fields(line); len(f) > 0 {
			oids[f[0]] = true
		}
	}
	return idxPath, oids
}

// RED: ShowIndex does not exist. It must return exactly the pack's objects.
func TestShowIndexReadsV2(t *testing.T) {
	idx, want := makeIdxFixture(t, "2")
	got, err := ShowIndex(idx)
	if err != nil {
		t.Fatalf("ShowIndex: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("ShowIndex returned %d oids, want %d", len(got), len(want))
	}
	for _, oid := range got {
		if !want[oid] {
			t.Errorf("unexpected oid %s", oid)
		}
	}
}

// RED (same run as above): the v1 GUARD from the spec — a v2-only reader
// would accept an index at push time it cannot fetch later.
func TestShowIndexReadsV1(t *testing.T) {
	idx, want := makeIdxFixture(t, "1")
	got, err := ShowIndex(idx)
	if err != nil {
		t.Fatalf("ShowIndex on a v1 index: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("ShowIndex returned %d oids from v1, want %d", len(got), len(want))
	}
}

// RED: a structurally corrupt index must be an error, never a short answer.
func TestShowIndexRejectsCorruptIndex(t *testing.T) {
	p := filepath.Join(t.TempDir(), "pack-0000000000000000000000000000000000000000.idx")
	if err := os.WriteFile(p, []byte("not an index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ShowIndex(p); err == nil {
		t.Fatal("ShowIndex must reject a corrupt index")
	}
}
