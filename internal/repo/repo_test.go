package repo

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/craigstoller/git-proton-backup/internal/gitcmd"
	"github.com/craigstoller/git-proton-backup/internal/protocol"
	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// TestCanonicalRootNormalises covers the normalisation half of the design's
// path-canonicalisation rule. None of it existed: main.go did only
// strings.TrimPrefix(os.Args[2], "proton::"), so a trailing slash produced
// "//refs/heads" paths and an empty root produced "/refs/heads" — addresses
// that would be sent to the Proton CLI verbatim. The canonical form is also
// the cache and lock identity, so two spellings of one repo that normalise
// differently are two locks over one repo.
func TestCanonicalRootNormalises(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/my-files/GitRemotes/myrepo", "/my-files/GitRemotes/myrepo"},
		{"proton::/my-files/r", "/my-files/r"},
		{"proton:://my-files//r/", "/my-files/r"},
		{"/my-files/r/", "/my-files/r"},
		{"/my-files/r///", "/my-files/r"},
		{"//my-files//GitRemotes///myrepo//", "/my-files/GitRemotes/myrepo"},
		{"  /my-files/r  ", "/my-files/r"},
		{"/devices/laptop/repos/r", "/devices/laptop/repos/r"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := CanonicalRoot(c.in)
			if err != nil {
				t.Fatalf("CanonicalRoot(%q) errored: %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("CanonicalRoot(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestCanonicalRootRejects covers the refusal half. The /shared-with-me row is
// a SAFETY rule, not tidiness: the CLI permits creating there, and a repo in a
// folder another person can write to is the one failure mode the single-writer
// model cannot survive. The dot rows pin "rejected outright rather than
// resolved" — resolving would let an address walk out of the namespace the
// check just verified.
func TestCanonicalRootRejects(t *testing.T) {
	cases := []struct{ name, in, wantIn string }{
		{"empty", "", "empty remote address"},
		{"scheme only", "proton::", "empty remote address"},
		{"whitespace only", "   ", "empty remote address"},
		{"relative", "my-files/r", "absolute"},
		{"drive root", "/", "Drive root"},
		{"dot component", "/my-files/./r", "rejected rather than resolved"},
		{"dotdot component", "/my-files/../devices/r", "rejected rather than resolved"},
		{"dotdot escaping the namespace", "/my-files/r/../../shared-with-me/r", "rejected rather than resolved"},
		{"shared-with-me", "/shared-with-me/theirrepo", "concurrent-writer"},
		{"shared-with-me root", "/shared-with-me", "concurrent-writer"},
		{"foreign namespace", "/trash/r", "must lie under"},
		{"my-files itself", "/my-files", "top-level"},
		{"my-files with trailing slash", "/my-files/", "top-level"},
		{"devices itself", "/devices", "top-level"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := CanonicalRoot(c.in)
			if err == nil {
				t.Fatalf("CanonicalRoot(%q) = %q, want a refusal", c.in, got)
			}
			if got != "" {
				t.Errorf("a refused address must return an empty root, got %q", got)
			}
			if !strings.Contains(err.Error(), c.wantIn) {
				t.Errorf("refusal must name the reason (%q), got: %v", c.wantIn, err)
			}
		})
	}
}

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

// TestBootstrapRefusesUnrecognisedMarkerContent covers the half of the design
// rule that was never implemented: "An unrecognised OR absent marker on a
// non-empty folder is a hard refusal — the helper never guesses whether a
// folder is one of its repos." Bootstrap only ever Stat'd the marker and, if
// present, proceeded; it never read it. So a folder holding a gpb-remote.json
// that says {"format":"something-else"} was adopted in silence.
//
// The version case is the one that will actually bite: the design nominates
// `version` as "the forward-compatibility seam: compaction will bump it and
// define its own ordering scheme at that point." A build that bumps to 2 and
// changes what the layout means must not be silently adopted by this build,
// which cannot honour whatever the bump stands for.
func TestBootstrapRefusesUnrecognisedMarkerContent(t *testing.T) {
	cases := []struct {
		name    string
		content string
		wantIn  string
	}{
		{"wrong format", `{"format":"something-else","version":1}`, "something-else"},
		{"future version", `{"format":"git-remote-proton","version":99}`, "99"},
		{"unparseable", `not json at all`, "could not be parsed"},
		{"empty file", ``, "could not be parsed"},
		{"json but missing both fields", `{}`, "format"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := transport.NewFake()
			f.Files["/my-files/r/"+MarkerName] = []byte(c.content)

			err := Bootstrap(f, "/my-files/r")
			if err == nil {
				t.Fatal("an unrecognised marker must be a hard refusal, never adopted")
			}
			if !strings.Contains(err.Error(), c.wantIn) {
				t.Errorf("refusal must name what it found (%q), got: %v", c.wantIn, err)
			}
			if f.Dirs["/my-files/r/refs/heads"] {
				t.Error("a refused bootstrap must not go on to create subdirs in the folder")
			}
		})
	}
}

// TestBootstrapAcceptsItsOwnMarker is the negative control for the test above:
// the validation must not be so strict that the marker this package itself
// writes is rejected.
func TestBootstrapAcceptsItsOwnMarker(t *testing.T) {
	f := transport.NewFake()
	if err := Bootstrap(f, "/my-files/r"); err != nil {
		t.Fatalf("first bootstrap: %v", err)
	}
	if err := Bootstrap(f, "/my-files/r"); err != nil {
		t.Fatalf("the marker this package writes must validate on the next run: %v", err)
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
	out, err := WriteRef(f, "/r", "refs/heads/main", "not-a-sha", false)
	if err == nil {
		t.Errorf("must refuse a non-sha value, got %v, nil error", out)
	}
	if _, ok := f.Files["/r/refs/heads/main"]; ok {
		t.Error("nothing must be written to the transport for a refused non-sha value")
	}
	// shaRe is 40-hex only, so a SHA-256 repository fails here too — true, but
	// "not a 40-hex sha" alone reads as corruption rather than an unsupported
	// repository format. The message must name it.
	if !strings.Contains(err.Error(), "SHA-256") {
		t.Errorf("the refusal must name SHA-256 repositories as unsupported, got: %v", err)
	}
}

// TestPushResolveFailureCarriesGitsOwnReason covers a resolve() that discarded
// both rev-parse's output and its error and reported only "cannot resolve %s".
// gitcmd.RevParse was deliberately given a three-value shape so callers could
// interpret it; git's own message is what distinguishes an unknown ref from an
// ambiguous one from an unreadable repository, and the operator saw none of it.
//
// It also pins the flattening in fail(): git's diagnostic here is genuinely
// multi-line, and results are rendered as one "error <ref> <reason>" status
// line each, so an embedded newline would split one status line into two and
// desynchronise git's read of the batch.
func TestPushResolveFailureCarriesGitsOwnReason(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	gitDir := newGitRepoForPush(t)

	ups := []protocol.RefUpdate{{Src: "refs/heads/no-such-branch", Dst: "refs/heads/main"}}
	res := Push(f, "/r", gitDir, ups, map[string]string{})
	if len(res) != 1 || res[0].OK {
		t.Fatalf("an unresolvable source must fail, got %+v", res)
	}
	t.Logf("reported reason: %q", res[0].Err)

	if !strings.Contains(res[0].Err, "cannot resolve refs/heads/no-such-branch") {
		t.Errorf("must still name what could not be resolved, got %q", res[0].Err)
	}
	if !strings.Contains(res[0].Err, "unknown revision") && !strings.Contains(res[0].Err, "ambiguous argument") {
		t.Errorf("git's own reason must survive into the reported error, got %q", res[0].Err)
	}
	if strings.ContainsAny(res[0].Err, "\r\n") {
		t.Errorf("a reason becomes one \"error <ref> <reason>\" status line and must not contain newlines, got %q", res[0].Err)
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

func headOfPushRepo(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	sha := strings.TrimSpace(string(out))
	if len(sha) != 40 {
		t.Fatalf("rev-parse returned %q", sha)
	}
	return sha
}

// refusingUploadTransport reports Refused for a chosen remote suffix, leaving
// whatever bytes the test planted at that path untouched.
type refusingUploadTransport struct {
	*transport.Fake
	refuseSuffix string
}

func (r *refusingUploadTransport) CreateExclusive(p, local string) (transport.Outcome, error) {
	if strings.HasSuffix(p, r.refuseSuffix) {
		return transport.Refused, nil
	}
	return r.Fake.CreateExclusive(p, local)
}

// plantPack builds the pack this push will produce and returns its remote
// paths plus the local files, so a test can seed the fake with variations.
func plantPack(t *testing.T, gitDir, head string) (packName, idxName string, packBytes, idxBytes []byte) {
	t.Helper()
	tmp := t.TempDir()
	packPath, idxPath, err := gitcmd.WritePack(gitDir, head, nil, tmp)
	if err != nil || packPath == "" {
		t.Fatalf("WritePack: %v", err)
	}
	pb, err := os.ReadFile(packPath)
	if err != nil {
		t.Fatal(err)
	}
	ib, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Base(packPath), filepath.Base(idxPath), pb, ib
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

// assertNothingUploaded fails the test if anything at all was written under
// root+"/packs/". The point of the destination guard is that it fires BEFORE
// any packing or uploading, so "rejected" is not enough — a rejection that
// still cost a pack upload to the user's paid Drive, and left an orphan behind
// that Stage 2 has no GC to reclaim, is the defect, not the fix.
func assertNothingUploaded(t *testing.T, f *transport.Fake, root string) {
	t.Helper()
	for p := range f.Files {
		if strings.HasPrefix(p, root+"/packs/") {
			t.Errorf("rejection must happen before any upload, but %s was written", p)
		}
	}
}

// TestPushRejectsHierarchicalDestinationBeforePacking covers the ref shape git
// accepts and users create constantly — refs/heads/feat/x — against a repo
// layer whose only prefix logic was isBranch. ListRefs is non-recursive
// (refs.go documents this), so the ref was invisible to the advertisement,
// exists came back false, the ancestry check was SKIPPED, a full pack was
// built and uploaded, and only then did WriteRef fail because refs/heads/feat
// does not exist — with a message naming neither the ref shape nor the
// limitation, and an orphan pack left on the remote.
func TestPushRejectsHierarchicalDestinationBeforePacking(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	gitDir := newGitRepoForPush(t)

	ups := []protocol.RefUpdate{{Src: "refs/heads/main", Dst: "refs/heads/feat/x"}}
	res := Push(f, "/r", gitDir, ups, map[string]string{})

	if len(res) != 1 || res[0].OK {
		t.Fatalf("a hierarchical destination must be rejected, got %+v", res)
	}
	if !strings.Contains(res[0].Err, "refs/heads/feat/x") {
		t.Errorf("rejection must name the destination, got %q", res[0].Err)
	}
	if !strings.Contains(res[0].Err, "refs/heads/<name>") {
		t.Errorf("rejection must name the actual limitation, got %q", res[0].Err)
	}
	assertNothingUploaded(t, f, "/r")
}

// TestPushRejectsPseudorefDestination covers the design's error-table row
// "Pseudorefs and unsupported destinations | Explicit rejection with a named
// reason". Before the guard, `git push proton-v2 main:refs/stash` wrote
// <root>/refs/stash, reported ok, and created a ref ListRefs will never
// advertise — so the NEXT push of it failed with "ref changed concurrently",
// a message describing a race that never happened.
func TestPushRejectsPseudorefDestination(t *testing.T) {
	gitDir := newGitRepoForPush(t)

	// A fresh Fake per subtest: assertNothingUploaded inspects the whole
	// Files map, so a shared one would let the first leak taint every later
	// case (or, worse, pass because a sibling already uploaded).
	for _, dst := range []string{"refs/stash", "HEAD", "refs/heads", "refs/heads/", "refs/notes/commits"} {
		t.Run(dst, func(t *testing.T) {
			f := transport.NewFake()
			_ = Bootstrap(f, "/r")
			ups := []protocol.RefUpdate{{Src: "refs/heads/main", Dst: dst}}
			res := Push(f, "/r", gitDir, ups, map[string]string{})
			if len(res) != 1 || res[0].OK {
				t.Fatalf("%q must be rejected with a named reason, got %+v", dst, res)
			}
			if _, ok := f.Files["/r/"+dst]; ok {
				t.Errorf("%q must not be written to the remote", dst)
			}
			assertNothingUploaded(t, f, "/r")
		})
	}
}

// TestPushRejectsUnsupportedDeleteDestination covers the delete path, which
// returned OK: true for any ref it could not see — and it can never see a
// pseudoref, because ListRefs does not advertise one. Reporting success for a
// deletion that certainly did not happen is worse than a plain failure: git
// drops its remote-tracking ref on the strength of it.
func TestPushRejectsUnsupportedDeleteDestination(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	sha := "2222222222222222222222222222222222222222"
	f.Files["/r/refs/stash"] = []byte(sha + "\n")

	// The remote map is empty on purpose: this is what ListRefs actually
	// returns for a pseudoref, so !exists is the branch that used to fire.
	ups := []protocol.RefUpdate{{Src: "", Dst: "refs/stash"}}
	res := Push(f, "/r", t.TempDir(), ups, map[string]string{})

	if len(res) != 1 || res[0].OK {
		t.Fatalf("deleting an unsupported destination must be rejected, not reported ok for a ref we cannot see, got %+v", res)
	}
	if _, ok := f.Files["/r/refs/stash"]; !ok {
		t.Error("a rejected delete must not have trashed anything")
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

// RED. Unpatched, Stat sees the corrupt pack and the ref is published.
func TestPushRefusedCorruptPackIsRejected(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	d := newGitRepoForPush(t)
	head := headOfPushRepo(t, d)
	packName, _, packBytes, _ := plantPack(t, d, head)

	corrupt := append([]byte(nil), packBytes...)
	corrupt[len(corrupt)/2] ^= 0xff
	f.Files["/r/packs/"+packName] = corrupt // same NAME, different bytes

	tr := &refusingUploadTransport{Fake: f, refuseSuffix: ".pack"}
	res := Push(tr, "/r", d, []protocol.RefUpdate{{Src: head, Dst: "refs/heads/main"}}, map[string]string{})
	if len(res) != 1 || res[0].OK {
		t.Fatalf("a refused pack whose bytes differ must fail: %+v", res)
	}
	if _, ok := f.Files["/r/refs/heads/main"]; ok {
		t.Error("no ref may be published when the remote pack does not verify")
	}
}

// RED. Unpatched, Stat sees the corrupt index and the ref is published.
func TestPushRefusedCorruptIdxIsRejected(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	d := newGitRepoForPush(t)
	head := headOfPushRepo(t, d)
	packName, idxName, packBytes, idxBytes := plantPack(t, d, head)

	f.Files["/r/packs/"+packName] = packBytes
	corrupt := append([]byte(nil), idxBytes...)
	corrupt[len(corrupt)/2] ^= 0xff
	f.Files["/r/packs/"+idxName] = corrupt

	tr := &refusingUploadTransport{Fake: f, refuseSuffix: ".idx"}
	res := Push(tr, "/r", d, []protocol.RefUpdate{{Src: head, Dst: "refs/heads/main"}}, map[string]string{})
	if len(res) != 1 || res[0].OK {
		t.Fatalf("a refused index that does not verify must fail: %+v", res)
	}
	if _, ok := f.Files["/r/refs/heads/main"]; ok {
		t.Error("no ref may be published when the remote index does not verify")
	}
}

// GUARD (passes before and after). This pins the deliberate asymmetry: a
// remote index that is VALID but not byte-identical to ours must be accepted.
// Requiring byte equality here would make a legitimate remote object
// permanently fatal, since immutable objects are never overwritten. If a
// future change tightens publishIdx, this test is what catches it.
func TestPushRefusedValidButDifferentIdxIsAccepted(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	d := newGitRepoForPush(t)
	head := headOfPushRepo(t, d)
	packName, idxName, packBytes, _ := plantPack(t, d, head)

	// A v1 index for the same pack: valid, verifies, different bytes.
	tmp := t.TempDir()
	packCopy := filepath.Join(tmp, packName)
	if err := os.WriteFile(packCopy, packBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "index-pack", "--index-version=1", packCopy).Run(); err != nil {
		// FATAL, deliberately, and NOT a skip. This test is the only
		// mechanical defence against a future change reintroducing byte
		// equality on the .idx — the single most peer-reviewed decision in
		// design v6.2, where demanding equality makes a legitimate remote
		// index PERMANENTLY fatal: immutable objects are never overwritten,
		// so the repo would be bricked for that client with no way out.
		//
		// Task 1 of this stage made that tightening more tempting by pinning
		// --index-version=2, which turned "our indexes are v2" from a
		// convention into an enforced invariant; the natural next thought
		// reading publishIdx is "so a remote index that is not v2 is suspect
		// — assert it". On any machine where this skipped, that change landed
		// green. There is also no cheap toolchain-independent replacement: a
		// second v2 index over the same pack is byte-IDENTICAL, because git
		// is deterministic there, so v1 is the only easy vehicle for "valid
		// but byte-different".
		//
		// A red suite is the correct signal if a future git drops v1 index
		// writing. That is precisely the moment a human should decide what
		// replaces the vehicle, rather than the suite quietly going green
		// with the guard gone.
		t.Fatalf("this git cannot write a v1 index (%v), so the guard cannot be satisfied "+
			"on this toolchain. It is NOT safe to skip: this test is the only thing stopping "+
			"publishIdx from being tightened into byte equality on the .idx, which design "+
			"v6.2 rejects because it would make a legitimate remote index permanently fatal "+
			"— immutable objects are never overwritten. Find another way to produce a valid "+
			"but byte-different index before weakening this test", err)
	}
	v1, err := os.ReadFile(strings.TrimSuffix(packCopy, ".pack") + ".idx")
	if err != nil {
		t.Fatal(err)
	}

	f.Files["/r/packs/"+packName] = packBytes
	f.Files["/r/packs/"+idxName] = v1

	tr := &refusingUploadTransport{Fake: f, refuseSuffix: ".idx"}
	res := Push(tr, "/r", d, []protocol.RefUpdate{{Src: head, Dst: "refs/heads/main"}}, map[string]string{})
	if len(res) != 1 || !res[0].OK {
		t.Fatalf("a valid but byte-different remote index must be accepted: %+v", res)
	}
}

// GUARD. A refused pack with no remote index is an orphan this push repairs.
func TestPushRefusedPackWithNoRemoteIdxRepairsTheOrphan(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	d := newGitRepoForPush(t)
	head := headOfPushRepo(t, d)
	packName, idxName, packBytes, _ := plantPack(t, d, head)

	f.Files["/r/packs/"+packName] = packBytes // pack only

	tr := &refusingUploadTransport{Fake: f, refuseSuffix: ".pack"}
	res := Push(tr, "/r", d, []protocol.RefUpdate{{Src: head, Dst: "refs/heads/main"}}, map[string]string{})
	if len(res) != 1 || !res[0].OK {
		t.Fatalf("a refused pack with no remote idx must be repaired: %+v", res)
	}
	if _, ok := f.Files["/r/packs/"+idxName]; !ok {
		t.Error("the orphan's index must have been uploaded")
	}
}

// errAfterReader returns some bytes of zero-value content, then errAfter on
// every following Read call — a synthetic reader standing in for a genuine
// (non-EOF) I/O error partway through a stream. Provoking a real, non-EOF
// read error from an actual file is not reliably portable (Windows and POSIX
// fail mid-read very differently, if at all, for the same underlying fault),
// so this drives readersEqual's loop deterministically on any platform
// instead of reaching for the filesystem.
type errAfterReader struct {
	remaining int
	errAfter  error
}

func (r *errAfterReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, r.errAfter
	}
	if len(p) > r.remaining {
		p = p[:r.remaining]
	}
	r.remaining -= len(p)
	return len(p), nil
}

// TestReadersEqualSurfacesGenuineReadError covers the fix-round-1 finding: a
// genuine (non-EOF) read error on one side, while the other side still has a
// full chunk of data ready (no error yet), must be returned as that error —
// never reported as (false, nil), i.e. "the bytes differ". Before the fix,
// the length/content comparison ran BEFORE the error checks: the errored
// side's short read (na=0) against the healthy side's full chunk (nb=65536)
// made na != nb fire first, so the function returned (false, nil) without
// ever looking at the error. publishPack turns a "not equal" result into a
// permanent, unrecoverable "the remote pack is corrupt" diagnosis — so a
// transient local read failure must never be able to produce that message.
func TestReadersEqualSurfacesGenuineReadError(t *testing.T) {
	wantErr := errors.New("simulated disk read failure")
	// ra fails immediately with a genuine error; rb has a full 64KiB chunk of
	// real data ready and no error yet.
	ra := &errAfterReader{remaining: 0, errAfter: wantErr}
	rb := bytes.NewReader(make([]byte, 64*1024))

	_, err := readersEqual(ra, rb)
	if err == nil {
		t.Fatal("a genuine read error must be returned, not swallowed into a false 'not equal'")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("got error %v, want it to be (or wrap) %v", err, wantErr)
	}
}

// --- M1: a checksum failure after filesEqual blames the right file ---------

// TestIdenticalPackChecksumFailureBlamesTheLocalGit covers the finding that
// this path's DIAGNOSIS pointed at the wrong artifact. It is reached only
// after filesEqual has proven the remote pack byte-identical to the local
// one, so a checksum mismatch there means BOTH copies recompute wrong — our
// own git mis-named its output. The old message ("remote pack %s recomputes
// to %s; ... this file is not what its name claims") sent the operator to
// their live paid account to remediate a file that is byte-for-byte what they
// already hold on disk.
//
// Driven directly rather than through Push: reaching it end to end would need
// git to mis-name a pack it just wrote, which is not something a test can ask
// it to do. This is the exact function and the exact input publishPack hands
// it on that path.
func TestIdenticalPackChecksumFailureBlamesTheLocalGit(t *testing.T) {
	body := bytes.Repeat([]byte("nonsense-pack-body"), 8)
	content := append(append([]byte(nil), body...), make([]byte, 20)...) // + a 20-byte trailer
	local := filepath.Join(t.TempDir(), "pack-"+strings.Repeat("0", 40)+".pack")
	if err := os.WriteFile(local, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha1.Sum(body)
	wantDigest := hex.EncodeToString(sum[:])

	err := checkIdenticalPackChecksum(local, "/r/packs/pack-"+strings.Repeat("0", 40)+".pack")
	if err == nil {
		t.Fatal("a pack whose body does not hash to its name must be refused")
	}
	msg := err.Error()

	// It must say the two copies are the same and that the fault is local.
	for _, want := range []string{"byte-identical", "LOCAL problem", "mis-named its own output",
		"Do not trash or replace the remote copy", wantDigest} {
		if !strings.Contains(msg, want) {
			t.Errorf("message must contain %q, got: %s", want, msg)
		}
	}
	// And it must NOT be the old message, which diagnosed the remote file.
	if strings.Contains(msg, "this file is not what its name claims") {
		t.Errorf("the diagnosis must not point at the remote artifact: %s", msg)
	}
}

// TestIdenticalPackChecksumUnreadableAlsoBlamesLocally is the same rule for
// the other failure this path can produce: a pack too short to have a
// trailer. filesEqual compares sizes first, so if the remote copy is that
// short the local one is too — again a local problem, not a remote one.
func TestIdenticalPackChecksumUnreadableAlsoBlamesLocally(t *testing.T) {
	local := filepath.Join(t.TempDir(), "pack-short.pack")
	if err := os.WriteFile(local, []byte("tiny"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := checkIdenticalPackChecksum(local, "/r/packs/pack-short.pack")
	if err == nil {
		t.Fatal("a pack shorter than its trailer must be refused")
	}
	for _, want := range []string{"byte-identical", "LOCAL problem", "Do not trash or replace"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message must contain %q, got: %s", want, err)
		}
	}
}

// --- M3: the fail-closed default arms on the two remaining switches --------

// unknownOutcomeTransport wraps a Fake but returns an Outcome that is not one
// of the three constants for CreateExclusive on a chosen path. Outcome is a
// closed set inside this module, so nothing else can produce this value —
// which is exactly why only a stub can prove the default arms fail closed
// rather than falling through and being read as Committed.
type unknownOutcomeTransport struct {
	*transport.Fake
	forPath string
}

func (u unknownOutcomeTransport) CreateExclusive(p, local string) (transport.Outcome, error) {
	if p == u.forPath {
		return transport.Outcome(42), nil
	}
	return u.Fake.CreateExclusive(p, local)
}

// TestBootstrapFailsClosedOnUnrecognisedMarkerOutcome: before the default arm,
// Bootstrap's boolean switch had no way to express "anything else", so an
// unrecognised Outcome fell through and the repo was adopted as initialised
// on the strength of a value nobody recognised.
func TestBootstrapFailsClosedOnUnrecognisedMarkerOutcome(t *testing.T) {
	f := transport.NewFake()
	tr := unknownOutcomeTransport{Fake: f, forPath: "/my-files/r/" + MarkerName}

	err := Bootstrap(tr, "/my-files/r")
	if err == nil {
		t.Fatal("Bootstrap must fail closed on an unrecognised marker-creation outcome")
	}
	if !strings.Contains(err.Error(), "outcome(42)") {
		t.Errorf("the refusal must name the outcome it saw, got: %v", err)
	}
	if _, ok := f.Files["/my-files/r/"+MarkerName]; ok {
		t.Error("no marker was actually written; nothing may claim otherwise")
	}
}

// TestAcquireLockFailsClosedOnUnrecognisedOutcome: same exposure on the lock.
// Falling through reached the read-back verification, which would have
// reported a held lock had the remote happened to carry our nonce.
func TestAcquireLockFailsClosedOnUnrecognisedOutcome(t *testing.T) {
	f := transport.NewFake()
	tr := unknownOutcomeTransport{Fake: f, forPath: "/my-files/r/" + LockName}

	l, err := AcquireLock(tr, "/my-files/r")
	if err == nil {
		t.Fatal("AcquireLock must fail closed on an unrecognised outcome")
	}
	if l != nil {
		t.Error("no Lock may be handed back when the outcome was not recognised")
	}
	if !strings.Contains(err.Error(), "outcome(42)") {
		t.Errorf("the refusal must name the outcome it saw, got: %v", err)
	}
}

// RED. DeriveHEAD does not exist. A pure function — no transport needed.
func TestDeriveHEAD(t *testing.T) {
	cases := []struct {
		name       string
		candidates []string
		clientHEAD string
		want       string
		wantOK     bool
	}{
		{"none", nil, "refs/heads/main", "", false},
		{"single wins outright", []string{"refs/heads/only"}, "refs/heads/other", "refs/heads/only", true},
		{"client HEAD breaks the tie",
			[]string{"refs/heads/zeta", "refs/heads/alpha", "refs/heads/main"},
			"refs/heads/main", "refs/heads/main", true},
		{"lexicographically first when the client HEAD is absent",
			[]string{"refs/heads/zeta", "refs/heads/alpha"},
			"refs/heads/nowhere", "refs/heads/alpha", true},
		{"lexicographically first when the client is detached",
			[]string{"refs/heads/zeta", "refs/heads/alpha"},
			"", "refs/heads/alpha", true},
		{"non-branches are not candidates", []string{"refs/tags/v1"}, "", "", false},
	}
	for _, c := range cases {
		got, ok := DeriveHEAD(c.candidates, c.clientHEAD)
		if got != c.want || ok != c.wantOK {
			t.Errorf("%s: DeriveHEAD = (%q, %v), want (%q, %v)", c.name, got, ok, c.want, c.wantOK)
		}
	}
}

// RED. WriteHEAD/ReadHEAD do not exist.
func TestWriteAndReadHEAD(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")

	if _, ok, err := ReadHEAD(f, "/r"); err != nil || ok {
		t.Fatalf("a fresh repo has no HEAD: %v %v", ok, err)
	}
	if out, err := WriteHEAD(f, "/r", "refs/heads/main"); err != nil || out != transport.Committed {
		t.Fatalf("WriteHEAD: %v %v", out, err)
	}
	if got := string(f.Files["/r/HEAD"]); got != "ref: refs/heads/main\n" {
		t.Errorf("HEAD content = %q", got)
	}
	branch, ok, err := ReadHEAD(f, "/r")
	if err != nil || !ok {
		t.Fatalf("ReadHEAD: %v %v", ok, err)
	}
	if branch != "refs/heads/main" {
		t.Errorf("ReadHEAD = %q, want refs/heads/main", branch)
	}
}

// RED. A symref payload must not be forced through WriteRef's 40-hex rule,
// and a garbage HEAD must be fatal rather than coerced.
func TestReadHEADRejectsCorruptContent(t *testing.T) {
	f := transport.NewFake()
	_ = Bootstrap(f, "/r")
	f.Files["/r/HEAD"] = []byte("1111111111111111111111111111111111111111\n")
	if _, _, err := ReadHEAD(f, "/r"); err == nil {
		t.Error("a detached-OID HEAD is not a symref and must be refused, not coerced")
	}
	f.Files["/r/HEAD"] = []byte("ref: refs/tags/v1\n")
	if _, _, err := ReadHEAD(f, "/r"); err == nil {
		t.Error("HEAD must point at a branch")
	}
}
