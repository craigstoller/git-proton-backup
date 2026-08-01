package repo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
