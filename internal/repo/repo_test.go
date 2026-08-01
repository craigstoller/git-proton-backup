package repo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/craigstoller/git-proton-backup/internal/protocol"
	"github.com/craigstoller/git-proton-backup/internal/transport"
)

func TestBootstrapEmptyRemote(t *testing.T) {
	f := transport.NewFake()
	if err := Bootstrap(f, "/my-files/r"); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if _, ok := f.Files["/my-files/r/gpb-remote.json"]; !ok {
		t.Error("marker must be written")
	}
	for _, d := range []string{"/my-files/r/refs", "/my-files/r/refs/heads", "/my-files/r/refs/tags", "/my-files/r/packs"} {
		if !f.Dirs[d] {
			t.Errorf("subdir %s must exist after first bootstrap", d)
		}
	}
}

// TestBootstrapCompletesPartialInitialisation covers the marker-present fast
// path in isolation: a folder with a valid marker but no subdirs yet (e.g. an
// interrupted prior bootstrap) must be completed, not treated as already done.
// Unlike TestBootstrapIdempotent, no first Bootstrap call runs here, so the
// subdirs cannot already exist as a side effect of anything else — this is
// the only test that would fail if the fast path stopped calling
// ensureSubdirs.
func TestBootstrapCompletesPartialInitialisation(t *testing.T) {
	f := transport.NewFake()
	f.Files["/my-files/r/gpb-remote.json"] = []byte(markerContent)
	if err := Bootstrap(f, "/my-files/r"); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	for _, d := range []string{"/my-files/r/refs", "/my-files/r/refs/heads", "/my-files/r/refs/tags", "/my-files/r/packs"} {
		if !f.Dirs[d] {
			t.Errorf("subdir %s must be created to complete a partial initialisation", d)
		}
	}
}

func TestBootstrapIgnoresLockWhenTestingEmptiness(t *testing.T) {
	f := transport.NewFake()
	f.Files["/my-files/r/.lock"] = []byte(`{"nonce":"n"}`)
	if err := Bootstrap(f, "/my-files/r"); err != nil {
		t.Fatalf("a lone .lock must not count as foreign data: %v", err)
	}
}

func TestBootstrapRefusesForeignData(t *testing.T) {
	f := transport.NewFake()
	f.Files["/my-files/r/taxes.pdf"] = []byte("x")
	if err := Bootstrap(f, "/my-files/r"); err == nil {
		t.Error("must refuse a non-empty folder with no marker")
	}
}

// TestBootstrapRefusesFoldersWithNoMarker is the sibling case
// TestBootstrapRefusesForeignData could not cover, because it plants a FILE.
// The realistic loss-of-marker shape is a root that still has its refs/ and
// packs/ FOLDERS but no gpb-remote.json — a marker trashed by hand, or a
// half-completed initialisation from a build that wrote subdirs first. The
// real CLI's `filesystem list` reports those folders, so the shipped
// Bootstrap hard-refuses (design: "Missing or unrecognised format marker on a
// non-empty folder | Hard refusal; never guess"). transport.Fake's List used
// to synthesise directories from file prefixes only and never read f.Dirs, so
// under test the same root looked EMPTY and was silently adopted — the suite
// was certifying a Bootstrap strictly more permissive than the one that runs
// against Proton.
func TestBootstrapRefusesFoldersWithNoMarker(t *testing.T) {
	f := transport.NewFake()
	if err := f.EnsureDir("/my-files/r/refs"); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	if err := f.EnsureDir("/my-files/r/packs"); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	err := Bootstrap(f, "/my-files/r")
	if err == nil {
		t.Fatal("must refuse a root that has repo-shaped subfolders but no marker, never guess")
	}
	if !strings.Contains(err.Error(), MarkerName) {
		t.Errorf("refusal must name the missing marker, got: %v", err)
	}
	if _, ok := f.Files["/my-files/r/"+MarkerName]; ok {
		t.Error("a refused bootstrap must not adopt the folder by writing a marker into it")
	}
}

func TestBootstrapIdempotent(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/my-files/r")
	if err := Bootstrap(f, "/my-files/r"); err != nil {
		t.Errorf("second bootstrap must be a no-op: %v", err)
	}
}

func TestStagedFileUsesTheLeafNameAndRefusesHostileOnes(t *testing.T) {
	// The CLI names the uploaded node after the LOCAL basename (probe C11),
	// so the staged file must BE the leaf name, not a neutral one.
	p, cleanup, err := stagedFile([]byte("x"), "main")
	if err != nil {
		t.Fatalf("staging a plain leaf must succeed: %v", err)
	}
	defer cleanup()
	if filepath.Base(p) != "main" {
		t.Errorf("staged basename must equal the leaf, got %q", filepath.Base(p))
	}
	if b, err := os.ReadFile(p); err != nil || string(b) != "x" {
		t.Errorf("staged content = %q, %v", b, err)
	}

	for _, bad := range []string{"a{b,c}", "con", "nul.txt", "", "..", "a/b"} {
		if _, _, err := stagedFile([]byte("x"), bad); err == nil {
			t.Errorf("%q must be refused with a reason, not mangled", bad)
		}
	}
}

func TestLockAcquireAndRelease(t *testing.T) {
	f := transport.NewFake()
	l, err := AcquireLock(f, "/my-files/r")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if _, ok := f.Files["/my-files/r/.lock"]; !ok {
		t.Fatal("lock file must exist while held")
	}
	if err := l.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, ok := f.Files["/my-files/r/.lock"]; ok {
		t.Error("lock file must be gone after release")
	}
}

func TestLockRefusesWhenHeld(t *testing.T) {
	f := transport.NewFake()
	if _, err := AcquireLock(f, "/my-files/r"); err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireLock(f, "/my-files/r"); err == nil {
		t.Error("second acquire must fail while the first holds it")
	}
}

func TestReleaseDoesNotDeleteSomeoneElsesLock(t *testing.T) {
	f := transport.NewFake()
	l, _ := AcquireLock(f, "/my-files/r")
	f.Files["/my-files/r/.lock"] = []byte(`{"nonce":"someone-else"}`)
	_ = l.Release()
	if _, ok := f.Files["/my-files/r/.lock"]; !ok {
		t.Error("must not delete a lock whose nonce does not match ours")
	}
}

// ambiguousTrashTransport wraps a Fake but forces Trash to report Ambiguous
// with a nil error. transport.Fake's FailNext field (see fake.go) is only
// checked by CreateExclusive, not by Trash, so it cannot drive this case; a
// small local stub is the only way to exercise it against the real
// transport.Transport interface.
type ambiguousTrashTransport struct {
	*transport.Fake
}

func (a ambiguousTrashTransport) Trash(p string) (transport.Outcome, error) {
	return transport.Ambiguous, nil
}

// TestReleaseFailsClosedOnAmbiguousTrash covers the requirement that Release
// must not discard Trash's Outcome. Committed from CreateExclusive does not
// prove a write landed, and symmetrically a non-Committed Trash outcome does
// not prove the lock is gone: reporting success here would let an operator
// believe the repo is unlocked while .lock may still be sitting on the
// remote, and v2 has no takeover to recover from that.
func TestReleaseFailsClosedOnAmbiguousTrash(t *testing.T) {
	f := transport.NewFake()
	l, err := AcquireLock(f, "/my-files/r")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	l.t = ambiguousTrashTransport{f}

	if err := l.Release(); err == nil {
		t.Error("Release must return an error when Trash reports a non-Committed outcome")
	}
	if _, ok := f.Files["/my-files/r/.lock"]; !ok {
		t.Error("lock file must still be reported present; Trash never actually removed it")
	}
}

// TestReleaseFailsClosedOnUnreadableLock covers review finding 1: readLock
// must not conflate "absent" with "present but corrupt". Before the fix, a
// .lock that failed to unmarshal was reported as (_, false, nil) — the exact
// shape Release treats as "not ours any more, leave it alone" — so Release
// returned success without ever calling Trash. This seeds that scenario
// directly: acquire normally, then corrupt the lock's own content in place
// (still this process's lock by path, now unparseable), and confirm Release
// (a) fails instead of reporting success, and (b) leaves the file in place,
// because it cannot prove the corrupt content is safe to delete.
func TestReleaseFailsClosedOnUnreadableLock(t *testing.T) {
	f := transport.NewFake()
	l, err := AcquireLock(f, "/my-files/r")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	f.Files["/my-files/r/.lock"] = []byte("not json")

	if err := l.Release(); err == nil {
		t.Error("Release must return an error when the lock is present but unreadable")
	}
	if _, ok := f.Files["/my-files/r/.lock"]; !ok {
		t.Error("Release must not trash a lock it cannot prove is its own")
	}
}

// TestLockRefusalReportsHolderNonce covers review finding 2: the design's
// binding rule is that a stale lock is reported with holder nonce, host, and
// age — the nonce is the actual identity, since two processes on one machine
// are otherwise indistinguishable by host/pid alone.
func TestLockRefusalReportsHolderNonce(t *testing.T) {
	f := transport.NewFake()
	first, err := AcquireLock(f, "/my-files/r")
	if err != nil {
		t.Fatal(err)
	}
	_, err = AcquireLock(f, "/my-files/r")
	if err == nil {
		t.Fatal("second acquire must fail while the first holds it")
	}
	if !strings.Contains(err.Error(), first.nonce) {
		t.Errorf("refusal message must name the holder's nonce, got: %v", err)
	}
}

// TestAcquireLockRefusalOnUnreadableLockIsDistinct pins the coherent decision
// made in AcquireLock's Refused branch: an unreadable lock is reported
// distinctly from a normal, healthy holder, rather than silently falling
// through to the generic "repo is locked" message.
func TestAcquireLockRefusalOnUnreadableLockIsDistinct(t *testing.T) {
	f := transport.NewFake()
	f.Files["/my-files/r/.lock"] = []byte("not json")

	_, err := AcquireLock(f, "/my-files/r")
	if err == nil {
		t.Fatal("acquire must refuse when .lock exists, even if unreadable")
	}
	if !strings.Contains(err.Error(), "unreadable") {
		t.Errorf("refusal on a corrupt lock must say so distinctly, got: %v", err)
	}
}

func TestWriteAndListRefs(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	sha := "1111111111111111111111111111111111111111"
	if out, err := WriteRef(f, "/r", "refs/heads/main", sha, false); err != nil || out != transport.Committed {
		t.Fatalf("create ref: %v %v", out, err)
	}
	refs, err := ListRefs(f, "/r")
	if err != nil {
		t.Fatal(err)
	}
	if refs["refs/heads/main"] != sha {
		t.Errorf("got %q", refs["refs/heads/main"])
	}
}

func TestListRefsRejectsCorruptRefFile(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	f.Files["/r/refs/heads/bad"] = []byte("not-a-sha\n")
	if _, err := ListRefs(f, "/r"); err == nil {
		t.Error("a malformed ref file must be fatal, never coerced")
	}
}

// TestWriteRefRefusesNonSha covers the brief's guard directly: WriteRef must
// reject a non-sha value before ever touching the transport, not attempt to
// stage or upload it. A ref file that is not exactly 40 lowercase hex plus a
// newline is corruption per the task's binding rule, so this must be refused
// outright rather than passed through.
func TestWriteRefRefusesNonSha(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	if out, err := WriteRef(f, "/r", "refs/heads/main", "not-a-sha", false); err == nil {
		t.Errorf("must refuse a non-sha value, got %v, nil error", out)
	}
	if _, ok := f.Files["/r/refs/heads/main"]; ok {
		t.Error("nothing must be written to the transport for a refused non-sha value")
	}
}

// lyingWriteTransport wraps a Fake but makes CreateExclusive report Committed
// while actually landing different content than what was staged — a
// transport that lies about what it wrote. This is the shape probe C2
// describes in miniature: an Outcome the caller cannot take at face value.
// transport.Fake's own CreateExclusive always writes exactly what was staged,
// so it cannot drive this case; a small local stub is the only way to
// exercise it against the real transport.Transport interface, the same
// technique ambiguousTrashTransport above already uses.
type lyingWriteTransport struct {
	*transport.Fake
}

func (l lyingWriteTransport) CreateExclusive(p, local string) (transport.Outcome, error) {
	l.Fake.Files[p] = []byte("2222222222222222222222222222222222222222\n")
	return transport.Committed, nil
}

// TestWriteRefCatchesReadBackMismatch is the most important test in this
// task: read-back is the design's answer to a transport it cannot trust
// (probe C2's silently-skipped rewrite rests on a digest Proton itself flags
// sha1Verified: false), and an untested read-back is a guarantee on paper
// only. Here CreateExclusive reports Committed but leaves different bytes at
// the path than WriteRef asked for; WriteRef must catch that on read-back
// and report Ambiguous with an error, never Committed.
func TestWriteRefCatchesReadBackMismatch(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	lying := lyingWriteTransport{f}
	sha := "1111111111111111111111111111111111111111"

	out, err := WriteRef(lying, "/r", "refs/heads/main", sha, false)
	if err == nil {
		t.Fatalf("a read-back mismatch must be reported as an error, got out=%v", out)
	}
	if out != transport.Ambiguous {
		t.Errorf("a read-back mismatch must report Ambiguous, got %v", out)
	}
}

// TestWriteRefRejectsHostileLeaf covers a ref whose leaf name
// checkStageableLeaf refuses (mirroring TestStagedFileUsesTheLeafNameAndRefusesHostileOnes).
// WriteRef must surface stagedFile's refusal as an error rather than mangling
// the name into something stageable, and nothing must reach the transport.
func TestWriteRefRejectsHostileLeaf(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	sha := "1111111111111111111111111111111111111111"
	if out, err := WriteRef(f, "/r", "refs/heads/con", sha, false); err == nil {
		t.Errorf("a leaf name checkStageableLeaf rejects must surface as an error, got %v, nil error", out)
	}
	if _, ok := f.Files["/r/refs/heads/con"]; ok {
		t.Error("nothing must be written to the transport for a rejected leaf name")
	}
}

// newGitRepoForPush builds a real, throwaway git repo under t.TempDir() with
// one commit, mirroring internal/gitcmd/gitcmd_test.go's newRepo helper. That
// helper lives in a different package and cannot be reused here, so this is a
// separate copy. Every setup command's error is checked: a bare t.TempDir()
// is an empty directory, not a git repository, so pushOne's resolve() (which
// shells out to `git rev-parse`) would fail before the ancestry logic under
// test is ever reached — the brief's original TestPushRejectsNonFastForward
// passed t.TempDir() directly as gitDir and could not have passed for that
// reason.
func newGitRepoForPush(t *testing.T) string {
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

// headOf returns the full HEAD sha of the repo at d, failing the test loudly
// (rather than silently misbehaving on a short slice) if rev-parse errors or
// returns something other than a 40-char sha.
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

// TestPushRejectsNonFastForward fixes a defect in the brief: passing
// t.TempDir() as gitDir is an empty directory, not a git repository, so
// resolve() would fail at `git rev-parse` before the ancestry logic under
// test is ever reached, and the result's error would be "cannot resolve
// refs/heads/main" rather than the "fetch first" this test asserts. Using a
// real repo (newGitRepoForPush) lets refs/heads/main resolve to a real sha,
// so the intended path is exercised: the ref exists remotely at the unknown
// sha "1111...", HasObject cannot find that object locally, and the result
// is "fetch first".
func TestPushRejectsNonFastForward(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	old := "1111111111111111111111111111111111111111"
	_, _ = WriteRef(f, "/r", "refs/heads/main", old, false)

	gitDir := newGitRepoForPush(t)
	ups := []protocol.RefUpdate{{Src: "refs/heads/main", Dst: "refs/heads/main"}}
	res := Push(f, "/r", gitDir, ups, map[string]string{"refs/heads/main": old})
	if len(res) != 1 || res[0].OK {
		t.Fatalf("want one failed result, got %+v", res)
	}
	if !strings.Contains(res[0].Err, "fetch first") {
		t.Errorf("unknown old sha must report 'fetch first', got %q", res[0].Err)
	}
}

// TestPushDeleteRef is fine using t.TempDir() (not a real repo) as gitDir,
// unlike TestPushRejectsNonFastForward above: a delete update carries an
// empty Src, so pushOne returns before resolve() — and therefore before any
// `git` invocation — is ever reached. The asymmetry with the sibling test
// above is deliberate, not an oversight.
func TestPushDeleteRef(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	sha := "2222222222222222222222222222222222222222"
	_, _ = WriteRef(f, "/r", "refs/heads/tmp", sha, false)

	ups := []protocol.RefUpdate{{Src: "", Dst: "refs/heads/tmp"}}
	res := Push(f, "/r", t.TempDir(), ups, map[string]string{"refs/heads/tmp": sha})
	if len(res) != 1 || !res[0].OK {
		t.Fatalf("delete should succeed: %+v", res)
	}
	if _, ok := f.Files["/r/refs/heads/tmp"]; ok {
		t.Error("ref file must be gone")
	}
}

// countPackFiles reports how many .pack and .idx entries exist under
// root+"/packs/" in a Fake's Files map.
func countPackFiles(f *transport.Fake, root string) (packs, idxs int) {
	prefix := root + "/packs/"
	for p := range f.Files {
		if !strings.HasPrefix(p, prefix) {
			continue
		}
		switch {
		case strings.HasSuffix(p, ".pack"):
			packs++
		case strings.HasSuffix(p, ".idx"):
			idxs++
		}
	}
	return packs, idxs
}

// TestPushOrderingPackAndIdxLandBeforeRef covers the task's central promise:
// a successful push writes the pack AND the idx, and only then the ref, and
// all three end up present. This is a first push of refs/heads/main (remote
// carries no entry for it), so the ancestry check is skipped and the full
// object range is packed.
func TestPushOrderingPackAndIdxLandBeforeRef(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	gitDir := newGitRepoForPush(t)
	head := headOf(t, gitDir)

	ups := []protocol.RefUpdate{{Src: "refs/heads/main", Dst: "refs/heads/main"}}
	res := Push(f, "/r", gitDir, ups, map[string]string{})
	if len(res) != 1 || !res[0].OK {
		t.Fatalf("push should succeed: %+v", res)
	}

	sha, err := readRef(f, "/r/refs/heads/main")
	if err != nil || sha != head {
		t.Fatalf("ref not published correctly: sha=%q err=%v want=%q", sha, err, head)
	}
	if packs, idxs := countPackFiles(f, "/r"); packs == 0 || idxs == 0 {
		t.Errorf("want a pack/idx pair under packs/, got pack=%d idx=%d", packs, idxs)
	}
}

// ambiguousPackUploadTransport wraps a Fake but forces CreateExclusive to
// report Ambiguous for anything staged under packs/, while behaving normally
// for everything else (the marker, refs, the lock). Fake's FailNext field
// only fires for the very next mutating call, and Bootstrap and the ref
// machinery both perform mutations before pushOne ever reaches the pack
// upload step, so FailNext cannot selectively target only that step; a small
// local stub — the same technique repo_test.go already uses above for
// ambiguousTrashTransport and lyingWriteTransport — is the only way to drive
// this deterministically against the real transport.Transport interface.
type ambiguousPackUploadTransport struct {
	*transport.Fake
}

func (a ambiguousPackUploadTransport) CreateExclusive(p, local string) (transport.Outcome, error) {
	if strings.HasPrefix(p, "/r/packs/") {
		return transport.Ambiguous, nil
	}
	return a.Fake.CreateExclusive(p, local)
}

// TestPushRefNotPublishedWhenPackUploadIsAmbiguous is the ordering guarantee
// from the other direction: when the pack upload cannot be confirmed, the ref
// must never be published, because that would leave a ref pointing at
// objects the remote may not actually have. Untested, "pack -> idx -> confirm
// both -> ref" is a promise on paper only.
func TestPushRefNotPublishedWhenPackUploadIsAmbiguous(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	gitDir := newGitRepoForPush(t)
	amb := ambiguousPackUploadTransport{f}

	ups := []protocol.RefUpdate{{Src: "refs/heads/main", Dst: "refs/heads/main"}}
	res := Push(amb, "/r", gitDir, ups, map[string]string{})
	if len(res) != 1 || res[0].OK {
		t.Fatalf("want a failed result when the pack upload is ambiguous, got %+v", res)
	}
	if _, ok := f.Files["/r/refs/heads/main"]; ok {
		t.Error("ref must not be published when the pack upload could not be confirmed")
	}
}

// TestPushForceSkipsAncestryCheck drives a forced update whose remote-known
// old sha does not exist locally at all: without Force, this would be
// rejected as "fetch first" before ever reaching the pack step (see
// TestPushRejectsNonFastForward above). With Force set, pushOne must skip the
// ancestry gate entirely and still go through pack upload and ref
// publication — proving Force actually short-circuits the check rather than
// merely happening not to trip it.
func TestPushForceSkipsAncestryCheck(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	gitDir := newGitRepoForPush(t)
	head := headOf(t, gitDir)

	old := "3333333333333333333333333333333333333333"
	_, _ = WriteRef(f, "/r", "refs/heads/main", old, false)

	ups := []protocol.RefUpdate{{Src: "refs/heads/main", Dst: "refs/heads/main", Force: true}}
	res := Push(f, "/r", gitDir, ups, map[string]string{"refs/heads/main": old})
	if len(res) != 1 || !res[0].OK {
		t.Fatalf("forced push should succeed despite an unresolvable old sha: %+v", res)
	}

	sha, err := readRef(f, "/r/refs/heads/main")
	if err != nil || sha != head {
		t.Fatalf("ref not updated to the new head: sha=%q err=%v want=%q", sha, err, head)
	}
	if packs, idxs := countPackFiles(f, "/r"); packs == 0 || idxs == 0 {
		t.Errorf("forced push must still upload a pack/idx pair, got pack=%d idx=%d", packs, idxs)
	}
}

// refusedRefCreateTransport wraps a Fake but forces CreateExclusive to
// report (Refused, nil) — no error — for a first-time ref create under
// refs/heads/, mimicking a concurrent creator winning the race. refs.go's
// WriteRef turns exactly this shape into (transport.Refused, nil) when
// exists is false (WriteRef never returns Refused on an update, since a
// byte-identical UpdateRevision falls through to a matching read-back and
// reports Committed instead). It must NOT intercept CreateExclusive calls
// under packs/, or this would exercise the ambiguous-pack path instead of
// the ref-publish path under test — the same care
// ambiguousPackUploadTransport above takes in the other direction.
type refusedRefCreateTransport struct {
	*transport.Fake
}

func (r refusedRefCreateTransport) CreateExclusive(p, local string) (transport.Outcome, error) {
	if strings.HasPrefix(p, "/r/refs/heads/") {
		return transport.Refused, nil
	}
	return r.Fake.CreateExclusive(p, local)
}

// TestPushReportsFailureWhenRefCreateLosesConcurrentRace covers a review
// finding: WriteRef legitimately returns (transport.Refused, nil) — no error
// — when a create races a concurrent creator and declines to overwrite
// (refs.go's "concurrent creator" branch). That is neither an error nor
// Ambiguous, so a publish check that only tests `err != nil || out ==
// transport.Ambiguous` lets it fall through and report the push as OK: true.
// That is worse than a plain failure: our newSha was never actually
// published, so git would update its remote-tracking ref to a sha that
// disagrees with what is really on the remote, with nothing to signal the
// mismatch. This drives CreateExclusive to Refused for the ref path only
// (pack/idx uploads still succeed normally through the same Fake) and
// asserts the push is reported as a failure naming the conflict, and that
// our sha was never written to the ref path.
func TestPushReportsFailureWhenRefCreateLosesConcurrentRace(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	gitDir := newGitRepoForPush(t)
	stub := refusedRefCreateTransport{f}

	ups := []protocol.RefUpdate{{Src: "refs/heads/main", Dst: "refs/heads/main"}}
	res := Push(stub, "/r", gitDir, ups, map[string]string{})
	if len(res) != 1 || res[0].OK {
		t.Fatalf("want a failed result when ref create loses a concurrent race, got %+v", res)
	}
	if !strings.Contains(res[0].Err, "concurrent") {
		t.Errorf("failure message must name the conflict, got %q", res[0].Err)
	}
	if _, ok := f.Files["/r/refs/heads/main"]; ok {
		t.Error("our sha must not be treated as published when we lost the create race")
	}
}
