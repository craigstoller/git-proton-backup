package repo

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
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
	// "/my-files/r" is not itself a mount root (only "/my-files" is), so the
	// stricter EnsureDir (Task 7) needs it seeded as already-existing before
	// this test's own setup calls below can create children under it.
	f.Dirs["/my-files/r"] = true
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
	f.Dirs["/r"] = true
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
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	f.Files["/r/refs/heads/bad"] = []byte("not-a-sha\n")
	if _, err := ListRefs(f, "/r"); err == nil {
		t.Error("a malformed ref file must be fatal, never coerced")
	}
}

// TestListRefsRecursesAllNamespaces is Task 8's headline case: nested
// branches, tags, notes, and refs/stash must all be advertised under their
// FULL name, not just the direct children of refs/heads and refs/tags the
// old two-namespace walk saw.
func TestListRefsRecursesAllNamespaces(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	sha := "1111111111111111111111111111111111111111"
	seed := []string{
		"refs/heads/main",
		"refs/heads/feature/x",
		"refs/tags/v1/rc",
		"refs/notes/commits",
		"refs/stash",
	}
	for _, name := range seed {
		f.Files["/r/"+name] = []byte(sha + "\n")
	}

	refs, err := ListRefs(f, "/r")
	if err != nil {
		t.Fatalf("ListRefs: %v", err)
	}
	for _, name := range seed {
		if refs[name] != sha {
			t.Errorf("refs[%q] = %q, want %q (full map: %v)", name, refs[name], sha, refs)
		}
	}
	if len(refs) != len(seed) {
		t.Errorf("got %d refs, want exactly %d: %v", len(refs), len(seed), refs)
	}
}

// TestListRefsSkipsInvalidNamesWithNoteNeverFatal: a foreign junk LEAF name
// must be skipped, never fatal — one stray web-UI file must not brick the
// whole repo's advertisement (spec round 2). Per the brief, this test
// asserts the MAP result only; the note's exact text is pinned separately in
// TestSkipNoteText below, against an injected io.Writer rather than the real
// os.Stderr.
func TestListRefsSkipsInvalidNamesWithNoteNeverFatal(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	sha := "1111111111111111111111111111111111111111"
	f.Files["/r/refs/heads/main"] = []byte(sha + "\n")
	// "a{b}" is a valid git ref-name COMPONENT (CheckRefName accepts braces)
	// but not advertisable — checkComponent (via checkStageableLeaf) refuses
	// "{" and "}" for this transport (probe C13). It is a well-named LEAF
	// file, not a folder, so this exercises advertisableName's skip path
	// specifically; the folder-skip path is the next test.
	f.Files["/r/refs/heads/a{b}"] = []byte(sha + "\n")

	var refs map[string]string
	var err error
	stderr := captureStderr(t, func() { refs, err = ListRefs(f, "/r") })

	if err != nil {
		t.Fatalf("a foreign junk name must never be fatal: %v", err)
	}
	if refs["refs/heads/main"] != sha {
		t.Errorf("the well-named sibling must still be advertised, got %v", refs)
	}
	if _, ok := refs["refs/heads/a{b}"]; ok {
		t.Errorf("the junk name must not be advertised, got %v", refs)
	}
	// The map assertions above cannot tell "skipped with a note" apart from
	// "skipped silently" — deleting the skipNote call in ListRefs' walk
	// leaves both passing (mutation-verified: with that call commented out,
	// this assertion is the only one of the two ListRefs skip tests that
	// fails here; TestListRefsNeverListsBeneathAnInvalidFolderName's own
	// stderr assertion below catches the folder-skip call). A silently
	// vanishing ref is exactly what the note exists to prevent, so the note
	// itself must be asserted, not just its absence from the map.
	if !strings.Contains(stderr, "refs/heads/a{b}") {
		t.Errorf("skipping the junk name must emit a note naming it, got stderr %q", stderr)
	}
}

// tracedListTransport wraps a Fake and records every path passed to List, so
// TestListRefsNeverListsBeneathAnInvalidFolderName can assert a braced path
// was never handed to a remote List() call at all.
type tracedListTransport struct {
	*transport.Fake
	listed []string
}

func (tr *tracedListTransport) List(p string) ([]transport.Node, error) {
	tr.listed = append(tr.listed, p)
	return tr.Fake.List(p)
}

// TestListRefsNeverListsBeneathAnInvalidFolderName covers round-2 Codex: an
// invalid FOLDER name must skip its whole subtree WITHOUT recursing into it
// — checkComponent runs on every node, directories included, BEFORE
// recursion, precisely so a folder named ".hidden" or "a{b}" never reaches a
// List() argument. Braces are valid to git (CheckRefName accepts them), but
// this transport's remote-glob behaviour on "{" in a List() path is
// UNVERIFIED (probe C13 only confirmed LOCAL glob-expansion on upload) —
// never probing it is the point, not an incidental property of
// skip-with-note.
func TestListRefsNeverListsBeneathAnInvalidFolderName(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	sha := "1111111111111111111111111111111111111111"
	f.Files["/r/refs/heads/main"] = []byte(sha + "\n")
	// Both junk entries are FOLDERS (a nested file underneath, so the Fake's
	// List synthesises a directory node for each) — ".hidden" fails the
	// leading-dot component rule, "a{b}" fails checkStageableLeaf's brace
	// refusal. Neither must ever be handed to List().
	f.Files["/r/refs/heads/.hidden/x"] = []byte(sha + "\n")
	f.Files["/r/refs/heads/a{b}/y"] = []byte(sha + "\n")

	tr := &tracedListTransport{Fake: f}
	var refs map[string]string
	var err error
	stderr := captureStderr(t, func() { refs, err = ListRefs(tr, "/r") })

	if err != nil {
		t.Fatalf("an invalid folder name must never be fatal: %v", err)
	}
	if refs["refs/heads/main"] != sha {
		t.Errorf("the valid sibling must still be advertised, got %v", refs)
	}
	if len(refs) != 1 {
		t.Errorf("only the valid sibling must be advertised, got %v", refs)
	}
	// Same mutation-visible gap as TestListRefsSkipsInvalidNamesWithNoteNeverFatal
	// above: the map and the traced-List assertions alone cannot distinguish
	// "skipped with a note" from "skipped silently" — deleting the skipNote
	// call in ListRefs' checkComponent-failure branch leaves every other
	// assertion in this test passing (mutation-verified). Both junk folder
	// names must be named on stderr.
	if !strings.Contains(stderr, ".hidden") {
		t.Errorf("skipping the .hidden folder must emit a note naming it, got stderr %q", stderr)
	}
	if !strings.Contains(stderr, "a{b}") {
		t.Errorf("skipping the braced folder must emit a note naming it, got stderr %q", stderr)
	}
	for _, p := range tr.listed {
		if strings.Contains(p, "a{b}") {
			t.Errorf("List() must never be called with the braced path, but got %q (all calls: %v)", p, tr.listed)
		}
		if strings.Contains(p, ".hidden") {
			t.Errorf("List() must never be called beneath the invalid folder, but got %q (all calls: %v)", p, tr.listed)
		}
	}
}

// TestSkipNoteText pins the skip-note's exact wording via an injected
// io.Writer rather than capturing the real os.Stderr — the convention is
// that notes always go to os.Stderr in production (ListRefs above always
// calls skipNote with os.Stderr), but the TEXT itself is a focused unit test
// of the helper in isolation.
func TestSkipNoteText(t *testing.T) {
	var buf strings.Builder
	skipNote(&buf, "/r", "refs/heads/a{b}", fmt.Errorf("boom"))
	want := "git-remote-proton: skipping /r/refs/heads/a{b}: boom\n"
	if buf.String() != want {
		t.Errorf("skipNote wrote %q, want %q", buf.String(), want)
	}
}

// TestListRefsMalformedContentStillFatal pins the readRef tightening: the
// OLD TrimRight(sha, "\r\n") tolerated a bare sha with no trailing newline, a
// CRLF terminator, and (because TrimRight strips a whole trailing run of
// \r/\n bytes) even a double-LF terminator — none of those are bytes v2
// itself ever writes (WriteRef always writes sha+"\n"), so all three are
// foreign or damaged data and must be fatal under the spec's exact grammar:
// 40 lowercase hex plus a single trailing newline, nothing else.
func TestListRefsMalformedContentStillFatal(t *testing.T) {
	sha := "1111111111111111111111111111111111111111"
	cases := []struct {
		name    string
		content []byte
	}{
		{"no trailing newline", []byte(sha)},
		{"CRLF terminator", []byte(sha + "\r\n")},
		{"double-LF terminator", []byte(sha + "\n\n")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := transport.NewFake()
			f.Dirs["/r"] = true
			_ = Bootstrap(f, "/r")
			f.Files["/r/refs/heads/bad"] = c.content
			if _, err := ListRefs(f, "/r"); err == nil {
				t.Errorf("%s must be fatal under the exact grammar, got no error", c.name)
			}
		})
	}
}

// TestListRefsIgnoresEmptyFolders: a folder with nothing under it
// contributes nothing and must not error — the walk's base case.
func TestListRefsIgnoresEmptyFolders(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	if err := f.EnsureDir("/r/refs/heads/empty"); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	refs, err := ListRefs(f, "/r")
	if err != nil {
		t.Fatalf("ListRefs: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("an empty tree must advertise nothing, got %v", refs)
	}
}

// captureStderr redirects os.Stderr to a temp file for the duration of fn,
// then returns what was written. Mirrors cmd/git-remote-proton/main_test.go's
// helper of the same name — that one lives in a different package, so this
// package needs its own copy.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stderr-*")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	orig := os.Stderr
	os.Stderr = f
	defer func() { os.Stderr = orig }()

	fn()

	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	b, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatalf("read back stderr: %v", err)
	}
	return string(b)
}

// TestWriteRefRefusesNonSha covers the brief's guard directly: WriteRef must
// reject a non-sha value before ever touching the transport, not attempt to
// stage or upload it. A ref file that is not exactly 40 lowercase hex plus a
// newline is corruption per the task's binding rule, so this must be refused
// outright rather than passed through.
func TestWriteRefRefusesNonSha(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
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
	f.Dirs["/r"] = true
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
	f.Dirs["/r"] = true
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
	f.Dirs["/r"] = true
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
// is an empty directory, not a git repository, so Push's resolve() (which
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

// commitOnPushRepo adds a second commit (file=content) on top of whatever
// HEAD dir currently has, and returns the new HEAD sha.
//
// Added in fix round 1 for TestFetchRejectsACorruptPackAndInstallsNothing:
// once that test primes dst with a genuine first fetch (so `before` is a real
// 1, not 0 by construction), re-fetching the SAME want would let Fetch's
// up-to-date short-circuit return before the corrupt pack under test is ever
// downloaded — the corruption would never be read, and the test would pass
// for the wrong reason. A second, not-yet-fetched commit gives the corrupted
// second fetch an actual want to chase.
func commitOnPushRepo(t *testing.T, dir, file, content string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
	for _, a := range [][]string{{"add", "."}, {"commit", "-qm", "c2"}} {
		if err := exec.Command("git", append([]string{"-C", dir}, a...)...).Run(); err != nil {
			t.Fatalf("git %v: %v", a, err)
		}
	}
	return headOfPushRepo(t, dir)
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
	packPath, idxPath, err := gitcmd.WritePack(gitDir, []string{head}, nil, tmp)
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
	f.Dirs["/r"] = true
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
// empty Src, so Push's phase 2 classifies it and phase 4 executes it without
// ever calling resolve() — and therefore without any `git` invocation being
// reached. The asymmetry with the sibling test above is deliberate, not an
// oversight.
func TestPushDeleteRef(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
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
	f.Dirs["/r"] = true
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
// machinery both perform mutations before Push's phase 3 ever reaches the
// pack upload step, so FailNext cannot selectively target only that step; a
// small local stub — the same technique repo_test.go already uses above for
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
	f.Dirs["/r"] = true
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

// TestPushAcceptsHierarchicalDestinationNowThatListRefsRecurses closes the
// loop this test used to describe as a standing limitation (it was named
// TestPushRejectsHierarchicalDestinationBeforePacking): before Task 8,
// refs/heads/feat/x was invisible to a non-recursive ListRefs, so exists
// came back false, the ancestry check was skipped, a full pack was built and
// uploaded, and only then did WriteRef fail because refs/heads/feat did not
// exist — with a message naming neither the ref shape nor the limitation,
// and an orphan pack left on the remote. ListRefs now recurses the whole
// refs/ tree and checkDst admits any advertisable name under refs/, so this
// push must succeed outright, AND the ref it wrote must be visible to a
// FOLLOWING ListRefs call — proving the write and the read agree, which is
// the whole point of the fix.
func TestPushAcceptsHierarchicalDestinationNowThatListRefsRecurses(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	gitDir := newGitRepoForPush(t)
	head := headOf(t, gitDir)

	ups := []protocol.RefUpdate{{Src: "refs/heads/main", Dst: "refs/heads/feat/x"}}
	res := Push(f, "/r", gitDir, ups, map[string]string{})

	if len(res) != 1 || !res[0].OK {
		t.Fatalf("a hierarchical destination must now be accepted, got %+v", res)
	}
	if packs, idxs := countPackFiles(f, "/r"); packs == 0 || idxs == 0 {
		t.Errorf("a successful push must still upload a pack/idx pair, got pack=%d idx=%d", packs, idxs)
	}
	refs, err := ListRefs(f, "/r")
	if err != nil {
		t.Fatalf("ListRefs: %v", err)
	}
	if refs["refs/heads/feat/x"] != head {
		t.Errorf("the ref just published must be advertised back, got %v", refs)
	}
}

// TestPushRejectsPseudorefDestination covers the design's error-table row
// "Pseudorefs and unsupported destinations | Explicit rejection with a named
// reason" for what is STILL rejected after Task 8's namespace re-enable.
//
// Task 8 retired checkDst's old "exactly refs/heads/<name> or
// refs/tags/<name>" narrowing: "refs/stash" and "refs/notes/commits" are now
// legitimate, advertisable destinations (TestPushAcceptsNamespacedDestinations
// below), and bare "refs/heads" (no trailing slash) is a syntactically valid
// ref name to git that checkDst no longer refuses up front — it still fails,
// just later, at publish, not here
// (TestPushToRefsHeadsItselfFailsAtPublishNotAtCheckDst below). What is left
// genuinely unsupported at the checkDst boundary is HEAD (no "refs/" prefix
// at all) and "refs/heads/" (git's own check-ref-format refuses the trailing
// "/").
func TestPushRejectsPseudorefDestination(t *testing.T) {
	gitDir := newGitRepoForPush(t)

	// A fresh Fake per subtest: assertNothingUploaded inspects the whole
	// Files map, so a shared one would let the first leak taint every later
	// case (or, worse, pass because a sibling already uploaded).
	for _, dst := range []string{"HEAD", "refs/heads/"} {
		t.Run(dst, func(t *testing.T) {
			f := transport.NewFake()
			f.Dirs["/r"] = true
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

// TestPushAcceptsNamespacedDestinations is the positive half of Task 8's
// "namespace re-enable": a push to refs/notes/commits or refs/stash must now
// succeed end-to-end and be visible to a following ListRefs, closing the old
// v6.1 narrowing's gap the hard way (a real Push, not just a checkDst call).
func TestPushAcceptsNamespacedDestinations(t *testing.T) {
	for _, dst := range []string{"refs/notes/commits", "refs/stash"} {
		t.Run(dst, func(t *testing.T) {
			f := transport.NewFake()
			f.Dirs["/r"] = true
			_ = Bootstrap(f, "/r")
			gitDir := newGitRepoForPush(t)

			ups := []protocol.RefUpdate{{Src: "refs/heads/main", Dst: dst}}
			res := Push(f, "/r", gitDir, ups, map[string]string{})
			if len(res) != 1 || !res[0].OK {
				t.Fatalf("%q must now be accepted, got %+v", dst, res)
			}
			refs, err := ListRefs(f, "/r")
			if err != nil {
				t.Fatalf("ListRefs: %v", err)
			}
			if _, ok := refs[dst]; !ok {
				t.Errorf("%q must be advertised back after publish, got %v", dst, refs)
			}
		})
	}
}

// TestPushToRefsHeadsItselfFailsAtPublishNotAtCheckDst pins a real
// consequence of admitting any advertisable name under refs/: "refs/heads"
// (no trailing slash — distinct from the malformed "refs/heads/" checked
// above) is syntactically a valid ref name to both git and advertisableName,
// so checkDst no longer refuses it up front. It still cannot be PUBLISHED,
// though — Bootstrap already created refs/heads as a FOLDER, and WriteRef's
// leaf-named upload to that exact path collides with it (the same D/F guard
// Task 7 gave the Fake) — so the push still fails, just later, and AFTER a
// pack upload the old all-or-nothing narrowing would have avoided.
//
// ADJUDICATED (Task 9a): Task 9a's batch-preflight D/F check does NOT flip
// this to a zero-cost preflight refusal, and that is deliberate, not a gap.
// The preflight is refs-ONLY: it compares this batch's destinations against
// the caller-supplied `remote` map (the advertised REFS) plus this batch's
// own valid changes — it has no visibility into raw transport folder state.
// "refs/heads" here is a plain pre-existing FOLDER from Bootstrap, never a
// ref (ListRefs never advertises a folder), so it never appears in `remote`
// and the preflight has nothing to compare against for this fixture (the
// batch's remote map is empty). The push therefore still reaches phase 3
// (builds and uploads a real pack — the orphan pinned below) before phase
// 5's WriteRef hits the actual Dirs-based collision and fails. Pinned
// deliberately rather than left uncovered: a future checkDst that makes this
// SUCCEED (writing a ref file where refs/heads/<branch> folders live) would
// be wrong, and a future preflight that silently starts inspecting folder
// state would need this test updated on purpose, not by accident.
func TestPushToRefsHeadsItselfFailsAtPublishNotAtCheckDst(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	gitDir := newGitRepoForPush(t)

	if err := checkDst("refs/heads"); err != nil {
		t.Fatalf(`checkDst("refs/heads") = %v, want nil (git accepts this ref name)`, err)
	}

	ups := []protocol.RefUpdate{{Src: "refs/heads/main", Dst: "refs/heads"}}
	res := Push(f, "/r", gitDir, ups, map[string]string{})
	if len(res) != 1 || res[0].OK {
		t.Fatalf("must still fail (D/F collision with the refs/heads folder), got %+v", res)
	}
	if _, ok := f.Files["/r/refs/heads"]; ok {
		t.Error("must not be written to the remote")
	}
	if packs, idxs := countPackFiles(f, "/r"); packs == 0 || idxs == 0 {
		t.Errorf("this failure happens AFTER packing, unlike a genuine checkDst "+
			"rejection — want an orphan pack/idx pair uploaded before the D/F collision "+
			"was discovered, got pack=%d idx=%d", packs, idxs)
	}
}

// TestPushRejectsUnsupportedDeleteDestination covers the delete path against
// a destination checkDst still refuses (HEAD has no "refs/" prefix at all)
// — it must be rejected outright, never reported OK: true just because the
// caller-supplied remote map happens to say the destination does not exist.
// Before Task 8, this used "refs/stash" as the example destination; that is
// no longer unsupported (TestPushAcceptsNamespacedDestinations above), so
// the target moved to one that still is.
func TestPushRejectsUnsupportedDeleteDestination(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	// Seeded so there is something at "/r/HEAD" a wrongly-permitted delete
	// could actually trash — an empty Fake would let a Trash call pass
	// unnoticed (Trash on an absent target is itself Committed, never an
	// error), which is exactly the silent-success shape this test exists to
	// catch.
	f.Files["/r/HEAD"] = []byte("ref: refs/heads/main\n")

	ups := []protocol.RefUpdate{{Src: "", Dst: "HEAD"}}
	res := Push(f, "/r", t.TempDir(), ups, map[string]string{})

	if len(res) != 1 || res[0].OK {
		t.Fatalf("deleting an unsupported destination must be rejected, not reported ok, got %+v", res)
	}
	if string(f.Files["/r/HEAD"]) != "ref: refs/heads/main\n" {
		t.Errorf("a rejected delete must not have trashed anything, HEAD now = %q", f.Files["/r/HEAD"])
	}
}

// TestCheckDstAdmitsAnyAdvertisableNameUnderRefs pins the namespace
// re-enable this task is named for directly against checkDst, independent of
// Push: the old v6.1 narrowing (exactly refs/heads/<name> or
// refs/tags/<name>, one component) is retired now that ListRefs recurses the
// whole tree and can actually advertise these names back.
func TestCheckDstAdmitsAnyAdvertisableNameUnderRefs(t *testing.T) {
	for _, dst := range []string{
		"refs/heads/main",
		"refs/heads/feat/x",
		"refs/tags/v1/rc",
		"refs/notes/commits",
		"refs/stash",
	} {
		if err := checkDst(dst); err != nil {
			t.Errorf("checkDst(%q) = %v, want nil", dst, err)
		}
	}
}

// TestCheckDstStillRejects pins what stays refused after the namespace
// re-enable: destinations with no "refs/" prefix at all (git's own authority
// has nothing to check there), and names that fail either git's real
// check-ref-format or this transport's stageability rules.
func TestCheckDstStillRejects(t *testing.T) {
	for _, dst := range []string{
		"HEAD",
		"refs/heads/",        // git check-ref-format itself refuses a trailing "/"
		"refs/heads/.hidden", // leading-dot component
		"refs/heads/a{b}",    // not stageable (probe C13)
	} {
		if err := checkDst(dst); err == nil {
			t.Errorf("checkDst(%q) = nil, want a refusal", dst)
		}
	}
}

// TestRequiresForce pins the design's conservative other-namespace rule.
// requiresForce is dead code until Task 9a wires it in (see the "wired in
// Task 9a" comment on it, push.go) — this test exists now, ahead of any
// caller, so the behaviour is pinned before it has one, per the brief.
func TestRequiresForce(t *testing.T) {
	cases := []struct {
		dst  string
		want bool
	}{
		{"refs/heads/main", false},
		{"refs/heads/feat/x", false},
		{"refs/tags/v1", false},
		{"refs/notes/commits", true},
		{"refs/stash", true},
		{"refs/heads", true}, // no trailing "/" — not actually under refs/heads/
		{"refs/tags", true},  // same, for refs/tags/
	}
	for _, c := range cases {
		if got := requiresForce(c.dst); got != c.want {
			t.Errorf("requiresForce(%q) = %v, want %v", c.dst, got, c.want)
		}
	}
}

// TestPushForceSkipsAncestryCheck drives a forced update whose remote-known
// old sha does not exist locally at all: without Force, this would be
// rejected as "fetch first" before ever reaching the pack step (see
// TestPushRejectsNonFastForward above). With Force set, Push's phase 2 must
// skip the ancestry gate entirely and still go through pack upload and ref
// publication — proving Force actually short-circuits the check rather than
// merely happening not to trip it.
func TestPushForceSkipsAncestryCheck(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
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
	f.Dirs["/r"] = true
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
	f.Dirs["/r"] = true
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
	f.Dirs["/r"] = true
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
	f.Dirs["/r"] = true
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
	f.Dirs["/r"] = true
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

// ============================================================================
// Task 9a: five-phase batch engine
// ============================================================================

// hashObjectBlob writes content as a blob via `git hash-object -w --stdin` in
// gitDir and returns its sha — a NON-COMMIT object, used to prove that the
// "other namespaces" force rule runs no commit-shaped machinery (ObjectType,
// HasObject, IsAncestor) at all.
func hashObjectBlob(t *testing.T, gitDir, content string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", gitDir, "hash-object", "-w", "--stdin")
	cmd.Stdin = strings.NewReader(content)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hash-object: %v", err)
	}
	sha := strings.TrimSpace(string(out))
	if len(sha) != 40 {
		t.Fatalf("hash-object returned %q, want a 40-char sha", sha)
	}
	return sha
}

// orderTraceTransport wraps a Fake and appends one labelled entry per
// mutating call it observes, in the order they actually happen — proving the
// five-phase engine's EXECUTION ORDER directly, rather than inferring it from
// outcomes alone. CreateExclusive under packs/ is "pack:", CreateExclusive or
// UpdateRevision under refs/ is "ref:", Trash is "trash:".
type orderTraceTransport struct {
	*transport.Fake
	trace *[]string
}

func (o orderTraceTransport) CreateExclusive(p, local string) (transport.Outcome, error) {
	switch {
	case strings.HasPrefix(p, "/r/packs/"):
		*o.trace = append(*o.trace, "pack:"+p)
	case strings.Contains(p, "/refs/"):
		*o.trace = append(*o.trace, "ref:"+p)
	}
	return o.Fake.CreateExclusive(p, local)
}

func (o orderTraceTransport) UpdateRevision(p, local string) (transport.Outcome, error) {
	if strings.Contains(p, "/refs/") {
		*o.trace = append(*o.trace, "ref:"+p)
	}
	return o.Fake.UpdateRevision(p, local)
}

func (o orderTraceTransport) Trash(p string) (transport.Outcome, error) {
	*o.trace = append(*o.trace, "trash:"+p)
	return o.Fake.Trash(p)
}

// TestPushRefusesDuplicateDestinationsWholeBatchUntouched covers the round-1
// Codex finding on the plan: duplicates must be PRE-SCANNED so EVERY holder
// of a duplicated destination is refused, not a first-seen-wins loop that
// lets the first one mutate while later ones alone are reported as failed.
// Two different sources target the same "dup" destination (ambiguous — which
// src should win?); an unrelated ref in the same batch is untouched by the
// collision and must still succeed.
func TestPushRefusesDuplicateDestinationsWholeBatchUntouched(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	gitDir := newGitRepoForPush(t)
	head := headOf(t, gitDir)

	ups := []protocol.RefUpdate{
		{Src: "refs/heads/main", Dst: "refs/heads/dup"},
		{Src: head, Dst: "refs/heads/dup"},
		{Src: "refs/heads/main", Dst: "refs/heads/solo"},
	}
	res := Push(f, "/r", gitDir, ups, map[string]string{})
	if len(res) != 3 {
		t.Fatalf("want 3 results, got %+v", res)
	}
	if res[0].OK || res[1].OK {
		t.Fatalf("both holders of the duplicated destination must be refused, got %+v", res[:2])
	}
	for i, r := range res[:2] {
		if !strings.Contains(r.Err, "duplicate destination") {
			t.Errorf("result %d must name the duplicate, got %q", i, r.Err)
		}
	}
	if !res[2].OK {
		t.Fatalf("an unrelated ref in the same batch must still succeed, got %+v", res[2])
	}
	if _, ok := f.Files["/r/refs/heads/dup"]; ok {
		t.Error("a duplicated destination must be left completely untouched")
	}
}

// TestPushFinalStateDFPreflightRefusesConflictingCreates covers the
// final-state D/F preflight (design 2b): two BRAND NEW creates in one batch
// that conflict with EACH OTHER — a ref cannot be both a leaf and a folder
// containing other refs — must both be refused, with the failure costing
// NOTHING: no pack is ever built (asserted via the Fake's own packs/
// children, not merely the reported outcome).
func TestPushFinalStateDFPreflightRefusesConflictingCreates(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	gitDir := newGitRepoForPush(t)

	ups := []protocol.RefUpdate{
		{Src: "refs/heads/main", Dst: "refs/heads/feature"},
		{Src: "refs/heads/main", Dst: "refs/heads/feature/x"},
	}
	res := Push(f, "/r", gitDir, ups, map[string]string{})
	if len(res) != 2 {
		t.Fatalf("want 2 results, got %+v", res)
	}
	for i, r := range res {
		if r.OK {
			t.Fatalf("both conflicting creates must be refused, got %+v", res)
		}
		if !strings.Contains(r.Err, "refs/heads/feature") {
			t.Errorf("result %d must name the conflicting ref, got %q", i, r.Err)
		}
	}
	refs, err := ListRefs(f, "/r")
	if err != nil {
		t.Fatalf("ListRefs: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("remote listing must be unchanged, got %v", refs)
	}
	if packs, idxs := countPackFiles(f, "/r"); packs != 0 || idxs != 0 {
		t.Errorf("no pack may be built when nothing survives the preflight, got pack=%d idx=%d", packs, idxs)
	}
}

// TestPushPreflightDFAgainstExistingRefs covers the preflight's OTHER
// direction: the conflict need not be batch-internal — a create can conflict
// with a ref that ALREADY exists on the remote (and is not deleted in this
// batch). An unrelated ref in the same batch is untouched by the collision.
func TestPushPreflightDFAgainstExistingRefs(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	gitDir := newGitRepoForPush(t)
	head := headOf(t, gitDir)
	if _, err := WriteRef(f, "/r", "refs/heads/feature/x", head, false); err != nil {
		t.Fatal(err)
	}
	remote := map[string]string{"refs/heads/feature/x": head}

	ups := []protocol.RefUpdate{
		{Src: "refs/heads/main", Dst: "refs/heads/feature"},
		{Src: "refs/heads/main", Dst: "refs/heads/unrelated"},
	}
	res := Push(f, "/r", gitDir, ups, remote)
	if len(res) != 2 {
		t.Fatalf("want 2 results, got %+v", res)
	}
	if res[0].OK {
		t.Fatalf("a create conflicting with an existing ref must fail at preflight, got %+v", res[0])
	}
	if !strings.Contains(res[0].Err, "refs/heads/feature/x") {
		t.Errorf("must name the conflicting existing ref, got %q", res[0].Err)
	}
	if !res[1].OK {
		t.Fatalf("an unrelated ref in the same batch must still succeed, got %+v", res[1])
	}
}

// TestPushFinalStateDFPreflightOrderIndependentOfInputOrder is RED (fix
// round I1, Important): the final-state preflight is SET ALGEBRA (remote
// minus every valid delete, plus every valid non-delete) and must not depend
// on the order ups happens to list entries in. Concretely: remote has
// refs/heads/feature; the batch UPDATES feature, DELETES feature (same
// name, update listed FIRST), and CREATES the dependent feature/x. A single
// pass over ups in input order gets this wrong: it applies the update (a
// no-op re-add of "feature", already present from remote), then the delete
// (removes "feature" from finalSet entirely), and nothing re-adds it
// afterward — so the dependent create sails through the preflight with
// nothing left to conflict against, costs a pack upload, and only fails
// LATE at ensureRefParents. Set algebra says "feature" survives regardless
// of input order (the delete does not "win" merely by being listed after
// the update — execution order is always phase-4-deletes-then-phase-5-
// writes, so the update's value is what's actually there afterward), so the
// dependent create must be refused AT THE CHEAP PREFLIGHT either way. This
// also exercises fix round M4 along the way: the update and the same-name
// delete both succeed (phase 5's exists is batch-aware, so the update
// routes through CreateExclusive against the node phase 4 just trashed,
// not UpdateRevision).
func TestPushFinalStateDFPreflightOrderIndependentOfInputOrder(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	gitDir := newGitRepoForPush(t)
	sha := headOf(t, gitDir)
	if _, err := WriteRef(f, "/r", "refs/heads/feature", sha, false); err != nil {
		t.Fatal(err)
	}
	remote := map[string]string{"refs/heads/feature": sha}

	ups := []protocol.RefUpdate{
		{Src: "refs/heads/main", Dst: "refs/heads/feature", Force: true}, // update, listed FIRST
		{Src: "", Dst: "refs/heads/feature"},                             // delete, listed SECOND
		{Src: "refs/heads/main", Dst: "refs/heads/feature/x"},            // dependent create
	}
	res := Push(f, "/r", gitDir, ups, remote)
	if len(res) != 3 {
		t.Fatalf("want 3 results, got %+v", res)
	}
	if !res[0].OK || !res[1].OK {
		t.Fatalf("the update and the delete must each succeed on their own: %+v", res[:2])
	}
	if res[2].OK {
		t.Fatalf("the dependent create must be refused, got %+v", res[2])
	}
	if !strings.Contains(res[2].Err, "refs/heads/feature") {
		t.Errorf("must name the conflicting ref, got %q", res[2].Err)
	}
	// Caught at the CHEAP preflight (its own wording: "conflicts with ...
	// leaf and a folder"), never the expensive late ensureRefParents path
	// ("occupies that name") — the whole point of the fix.
	if !strings.Contains(res[2].Err, "conflicts with") {
		t.Errorf("must be refused by the preflight, not a late runtime D/F failure, got %q", res[2].Err)
	}
	if _, ok := f.Files["/r/refs/heads/feature/x"]; ok {
		t.Error("the refused create must not have been written")
	}
	sha2, err := readRef(f, "/r/refs/heads/feature")
	if err != nil {
		t.Fatalf("readRef: %v", err)
	}
	if sha2 != sha {
		// Force:true update resolved "refs/heads/main", which IS `sha` in
		// this fixture (newGitRepoForPush's only commit) — same value,
		// different code path (CreateExclusive after the same-batch
		// delete), so the content assertion is trivially satisfied; the
		// real pin is the code-path assertion above via M4's routing.
		t.Errorf("refs/heads/feature = %q, want %q", sha2, sha)
	}
}

// TestPushOtherNamespaceRequiresForce covers the design's conservative
// other-namespace rule end to end: a create needs no force (it is not a
// move), an unforced update is refused naming the force requirement, and a
// forced update succeeds. The unforced-update case deliberately uses an OLD
// tip absent locally and a target that is a non-commit blob — proving that
// no ancestry/fetch-first machinery runs at all for this namespace (round-2
// Codex: the generic block would say "fetch first" or error on the object
// type before ever reaching the force refusal).
func TestPushOtherNamespaceRequiresForce(t *testing.T) {
	gitDir := newGitRepoForPush(t)
	blobSha := hashObjectBlob(t, gitDir, "notes content")
	absentOld := "9999999999999999999999999999999999999999"

	t.Run("create without force is ok", func(t *testing.T) {
		f := transport.NewFake()
		f.Dirs["/r"] = true
		_ = Bootstrap(f, "/r")
		ups := []protocol.RefUpdate{{Src: blobSha, Dst: "refs/notes/commits"}}
		res := Push(f, "/r", gitDir, ups, map[string]string{})
		if len(res) != 1 || !res[0].OK {
			t.Fatalf("a create in another namespace needs no force: %+v", res)
		}
	})

	t.Run("unforced update requires force with no ancestry machinery", func(t *testing.T) {
		f := transport.NewFake()
		f.Dirs["/r"] = true
		_ = Bootstrap(f, "/r")
		ups := []protocol.RefUpdate{{Src: blobSha, Dst: "refs/notes/commits"}}
		res := Push(f, "/r", gitDir, ups, map[string]string{"refs/notes/commits": absentOld})
		if len(res) != 1 || res[0].OK {
			t.Fatalf("an unforced update outside refs/heads/ and refs/tags/ must require force: %+v", res)
		}
		if !strings.Contains(res[0].Err, "force") {
			t.Errorf("must name the force requirement, got %q", res[0].Err)
		}
		if strings.Contains(res[0].Err, "fetch first") || strings.Contains(res[0].Err, "ancestry") ||
			strings.Contains(res[0].Err, "object type") {
			t.Errorf("no ancestry/fetch-first/object-type machinery may run for this namespace, got %q", res[0].Err)
		}
	})

	t.Run("forced update succeeds despite absent old tip and non-commit target", func(t *testing.T) {
		f := transport.NewFake()
		f.Dirs["/r"] = true
		_ = Bootstrap(f, "/r")
		ups := []protocol.RefUpdate{{Src: blobSha, Dst: "refs/notes/commits", Force: true}}
		res := Push(f, "/r", gitDir, ups, map[string]string{"refs/notes/commits": absentOld})
		if len(res) != 1 || !res[0].OK {
			t.Fatalf("a forced update outside refs/heads/ and refs/tags/ must succeed: %+v", res)
		}
	})
}

// TestPushTagUpdateRequiresForceNoAncestry pins the design table's own row
// ("Tag update | Requires force, matching git's rule; no ancestry check")
// directly: an UNFORCED tag update is refused even though it is genuinely
// fast-forwardable. Shipped pushOne has no tag arm and runs the generic
// ancestry block on tag updates instead (it would have ACCEPTED this
// fast-forward) — a pre-existing divergence from the design table that this
// restructure ALIGNS rather than preserves; flagged in the task report.
func TestPushTagUpdateRequiresForceNoAncestry(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	gitDir := newGitRepoForPush(t)
	old := headOf(t, gitDir)
	newSha := commitOnPushRepo(t, gitDir, "b.txt", "two") // old IS an ancestor of newSha

	ups := []protocol.RefUpdate{{Src: newSha, Dst: "refs/tags/v1"}}
	res := Push(f, "/r", gitDir, ups, map[string]string{"refs/tags/v1": old})
	if len(res) != 1 || res[0].OK {
		t.Fatalf("an unforced tag update must be refused even though it is fast-forwardable, got %+v", res)
	}
	if !strings.Contains(res[0].Err, "force") {
		t.Errorf("must name the force requirement, got %q", res[0].Err)
	}
}

// TestPushTagAcceptsNonCommitObjectWithForce is RED (fix round M6): pins the
// OTHER half of the design table's tag row. "no ancestry check" already has
// TestPushTagUpdateRequiresForceNoAncestry above; this pins that the tag arm
// also applies NO OBJECT-TYPE RESTRICTION — real git allows a tag to point
// at any object (a lightweight tag over a tree or blob is legal), unlike a
// branch, which the design's own table restricts to commits. Before this
// test nothing exercised that the tag arm's absence of an object-type check
// was intentional rather than merely untested. Mirrors
// TestPushOtherNamespaceRequiresForce's non-commit-target subtest.
func TestPushTagAcceptsNonCommitObjectWithForce(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	gitDir := newGitRepoForPush(t)
	blobSha := hashObjectBlob(t, gitDir, "tag payload")
	absentOld := "9999999999999999999999999999999999999999"

	ups := []protocol.RefUpdate{{Src: blobSha, Dst: "refs/tags/v1", Force: true}}
	res := Push(f, "/r", gitDir, ups, map[string]string{"refs/tags/v1": absentOld})
	if len(res) != 1 || !res[0].OK {
		t.Fatalf("a forced tag update must accept a non-commit object with no object-type check: %+v", res)
	}
	sha, err := readRef(f, "/r/refs/tags/v1")
	if err != nil || sha != blobSha {
		t.Fatalf("tag not published correctly: sha=%q err=%v want=%q", sha, err, blobSha)
	}
}

// TestPushDeletionsRunAfterPackConfirmBeforeCreates pins the [Both] round-1
// ordering rule directly via a call trace, not merely via outcomes: even
// though the batch lists the CREATE first and the DELETE second (git-order),
// the engine must upload+confirm the pack, THEN run the delete, THEN write
// the dependent create's ref — never the reverse.
func TestPushDeletionsRunAfterPackConfirmBeforeCreates(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	gitDir := newGitRepoForPush(t)
	sha := "2222222222222222222222222222222222222222"
	if _, err := WriteRef(f, "/r", "refs/heads/old", sha, false); err != nil {
		t.Fatal(err)
	}
	remote := map[string]string{"refs/heads/old": sha}

	var trace []string
	tr := orderTraceTransport{Fake: f, trace: &trace}

	ups := []protocol.RefUpdate{
		{Src: "refs/heads/main", Dst: "refs/heads/feature/x"}, // create, listed first
		{Src: "", Dst: "refs/heads/old"},                      // delete, listed second
	}
	res := Push(tr, "/r", gitDir, ups, remote)
	for _, r := range res {
		if !r.OK {
			t.Fatalf("both updates must succeed: %+v", res)
		}
	}

	packIdx, trashIdx, refIdx := -1, -1, -1
	for i, e := range trace {
		switch {
		case packIdx == -1 && strings.HasPrefix(e, "pack:"):
			packIdx = i
		case trashIdx == -1 && strings.HasPrefix(e, "trash:"):
			trashIdx = i
		case refIdx == -1 && strings.HasPrefix(e, "ref:"):
			refIdx = i
		}
	}
	if packIdx == -1 || trashIdx == -1 || refIdx == -1 {
		t.Fatalf("expected pack, trash, and ref events in the trace, got %v", trace)
	}
	if !(packIdx < trashIdx && trashIdx < refIdx) {
		t.Errorf("want pack < trash < ref (pack confirm, then delete, then create), got trace %v", trace)
	}
}

// TestPushPackFailureFailsCreatesButDeletionsProceed is a GUARD, not a RED
// (round-1 Gemini): today's per-ref pushOne already lets an unrelated
// deletion proceed past a failed create in the SAME batch. This pins that the
// behaviour SURVIVES the restructure, with the new all-creates-share-one-
// pack-failure shape asserted on top — FailNext fires on the very next
// mutation, which under the new engine is the batch's single pack upload.
func TestPushPackFailureFailsCreatesButDeletionsProceed(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	gitDir := newGitRepoForPush(t)
	sha := "2222222222222222222222222222222222222222"
	if _, err := WriteRef(f, "/r", "refs/heads/old", sha, false); err != nil {
		t.Fatal(err)
	}
	remote := map[string]string{"refs/heads/old": sha}
	f.FailNext = "inject"

	ups := []protocol.RefUpdate{
		{Src: "refs/heads/main", Dst: "refs/heads/feature/x"},
		{Src: "", Dst: "refs/heads/old"},
	}
	res := Push(f, "/r", gitDir, ups, remote)
	if len(res) != 2 {
		t.Fatalf("want 2 results, got %+v", res)
	}
	if res[0].OK {
		t.Fatalf("the create must fail when the batch's pack upload is ambiguous, got %+v", res[0])
	}
	if !strings.Contains(res[0].Err, "pack") {
		t.Errorf("must name the pack failure, got %q", res[0].Err)
	}
	if !res[1].OK {
		t.Fatalf("the unrelated deletion must still succeed despite the pack failure, got %+v", res[1])
	}
}

// TestPushNonBranchDeleteProceedsUnderUnreadableHEAD is RED (plan round 3,
// Codex): HEAD protection is BRANCHES-ONLY. HEAD is corrupt/unreadable;
// deleting refs/tags/v1 must still succeed (HEAD can only ever name a
// branch), while deleting refs/heads/main in the SAME batch fails closed.
func TestPushNonBranchDeleteProceedsUnderUnreadableHEAD(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	sha := "2222222222222222222222222222222222222222"
	if _, err := WriteRef(f, "/r", "refs/heads/main", sha, false); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteRef(f, "/r", "refs/tags/v1", sha, false); err != nil {
		t.Fatal(err)
	}
	f.Files["/r/HEAD"] = []byte("this is not a symref\n") // corrupt / unreadable

	ups := []protocol.RefUpdate{
		{Src: "", Dst: "refs/tags/v1"},
		{Src: "", Dst: "refs/heads/main"},
	}
	res := Push(f, "/r", t.TempDir(), ups,
		map[string]string{"refs/heads/main": sha, "refs/tags/v1": sha})

	if len(res) != 2 {
		t.Fatalf("want 2 results, got %+v", res)
	}
	if !res[0].OK {
		t.Fatalf("a non-branch delete must succeed despite an unreadable HEAD: %+v", res[0])
	}
	if res[1].OK {
		t.Fatalf("a branch delete must fail closed under an unreadable HEAD: %+v", res[1])
	}
	if !strings.Contains(res[1].Err, "HEAD") {
		t.Errorf("must name the HEAD read failure, got %q", res[1].Err)
	}
}

// TestPushDeleteOfHEADBranchRefusedAtPreflight is RED (round-1 Codex): the
// batch deletes the branch HEAD names AND creates a child underneath it. The
// delete must be refused in PHASE 2 (the batch's single, non-mutating
// ReadHEAD) — and, critically, the REFUSED delete must NOT be subtracted from
// the preflight's final-state set, so the dependent create ALSO fails the D/F
// preflight — and no pack is ever built. Without the phase-2 HEAD read, this
// batch would upload a pack and then fail twice downstream instead.
func TestPushDeleteOfHEADBranchRefusedAtPreflight(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	gitDir := newGitRepoForPush(t)
	sha := headOf(t, gitDir)
	if _, err := WriteRef(f, "/r", "refs/heads/main", sha, false); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteHEAD(f, "/r", "refs/heads/main"); err != nil {
		t.Fatal(err)
	}
	remote := map[string]string{"refs/heads/main": sha}

	ups := []protocol.RefUpdate{
		{Src: "", Dst: "refs/heads/main"},
		{Src: "refs/heads/main", Dst: "refs/heads/main/child"},
	}
	res := Push(f, "/r", gitDir, ups, remote)
	if len(res) != 2 {
		t.Fatalf("want 2 results, got %+v", res)
	}
	if res[0].OK {
		t.Fatalf("deleting the branch HEAD points at must be refused, got %+v", res[0])
	}
	if !strings.Contains(res[0].Err, "HEAD points at") {
		t.Errorf("must name the HEAD-protection reason, got %q", res[0].Err)
	}
	if res[1].OK {
		t.Fatalf("the dependent create must also fail the D/F preflight, got %+v", res[1])
	}
	if !strings.Contains(res[1].Err, "refs/heads/main") {
		t.Errorf("must name the conflicting ref, got %q", res[1].Err)
	}
	if _, ok := f.Files["/r/refs/heads/main"]; !ok {
		t.Error("the ref file must survive a refused delete")
	}
	if packs, idxs := countPackFiles(f, "/r"); packs != 0 || idxs != 0 {
		t.Errorf("no pack may be built when nothing survives the preflight, got pack=%d idx=%d", packs, idxs)
	}
}

// TestPushDeleteThenCreateSameNameOneBatch covers the design's own motivating
// example for deletions-before-creates ordering: `git push origin :feature
// feature/x` must succeed in one batch regardless of git's own send order —
// the delete makes room, and the preflight's final-state set (computed AFTER
// subtracting the valid delete) contains only feature/x, so there is no
// conflict left to refuse.
func TestPushDeleteThenCreateSameNameOneBatch(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	gitDir := newGitRepoForPush(t)
	sha := headOf(t, gitDir)
	if _, err := WriteRef(f, "/r", "refs/heads/feature", sha, false); err != nil {
		t.Fatal(err)
	}
	remote := map[string]string{"refs/heads/feature": sha}

	ups := []protocol.RefUpdate{
		{Src: "", Dst: "refs/heads/feature"},
		{Src: "refs/heads/main", Dst: "refs/heads/feature/x"},
	}
	res := Push(f, "/r", gitDir, ups, remote)
	for _, r := range res {
		if !r.OK {
			t.Fatalf("both the delete and the dependent create must succeed: %+v", res)
		}
	}
	refs, err := ListRefs(f, "/r")
	if err != nil {
		t.Fatalf("ListRefs: %v", err)
	}
	if _, ok := refs["refs/heads/feature"]; ok {
		t.Errorf("the deleted ref must be gone, got %v", refs)
	}
	if refs["refs/heads/feature/x"] != sha {
		t.Errorf("the created ref must be present, got %v", refs)
	}
}

// TestPushCreatesNestedBranchCreatingParents is the hierarchical-create
// happy path end to end: creating refs/heads/feature/deep/x on a fresh
// remote must create the intermediate folders AND the leaf ref file.
func TestPushCreatesNestedBranchCreatingParents(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	gitDir := newGitRepoForPush(t)
	head := headOf(t, gitDir)

	ups := []protocol.RefUpdate{{Src: "refs/heads/main", Dst: "refs/heads/feature/deep/x"}}
	res := Push(f, "/r", gitDir, ups, map[string]string{})
	if len(res) != 1 || !res[0].OK {
		t.Fatalf("push should succeed: %+v", res)
	}
	for _, d := range []string{"/r/refs/heads/feature", "/r/refs/heads/feature/deep"} {
		if !f.Dirs[d] {
			t.Errorf("want folder %s to exist, dirs=%v", d, f.Dirs)
		}
	}
	sha, err := readRef(f, "/r/refs/heads/feature/deep/x")
	if err != nil || sha != head {
		t.Fatalf("ref not published correctly: sha=%q err=%v want=%q", sha, err, head)
	}
}

// TestPushReverseDFRefusedNamingTheBlockingRef exercises ensureRefParents'
// OWN typed reverse-D/F detection specifically — not the phase-2 preflight.
// The blocking ref exists on the TRANSPORT but the caller's `remote` snapshot
// (as if from a stale ListRefs, or a concurrent write the caller has not
// observed yet) does not know about it, so the conflict is invisible to
// phase 2 (which only ever consults the caller-supplied remote map) and is
// only discovered when ensureRefParents' EnsureDir walk actually collides
// with the existing ref FILE in phase 5.
func TestPushReverseDFRefusedNamingTheBlockingRef(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	gitDir := newGitRepoForPush(t)
	sha := headOf(t, gitDir)
	if _, err := WriteRef(f, "/r", "refs/heads/feature", sha, false); err != nil {
		t.Fatal(err)
	}

	ups := []protocol.RefUpdate{{Src: "refs/heads/main", Dst: "refs/heads/feature/x"}}
	res := Push(f, "/r", gitDir, ups, map[string]string{}) // remote map does NOT know about refs/heads/feature
	if len(res) != 1 || res[0].OK {
		t.Fatalf("must be refused (D/F collision with an existing ref file), got %+v", res)
	}
	if !strings.Contains(res[0].Err, "refs/heads/feature") {
		t.Errorf("must name the blocking ref, got %q", res[0].Err)
	}
	if _, ok := f.Files["/r/refs/heads/feature/x"]; ok {
		t.Error("the ref must not have been written")
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

// statErrorTransport wraps a Fake but makes Stat report a transport failure
// for every path, mimicking a broken or uncertified CLI. transport.Fake's
// own Stat never errors — a map miss is always confirmed absence (fake.go) —
// so a small local stub is the only way to drive RequireMarker's err-vs-!ok
// branches independently against the real transport.Transport interface, the
// same technique this file already uses above for unknownOutcomeTransport,
// ambiguousTrashTransport, and lyingWriteTransport.
type statErrorTransport struct {
	*transport.Fake
}

func (s statErrorTransport) Stat(p string) (transport.Node, bool, error) {
	return transport.Node{}, false, fmt.Errorf("simulated transport failure statting %s", p)
}

// TestRequireMarkerSurfacesStatFailureDistinctlyFromNoMarker is Task 4's
// end-to-end check that CLI.Stat's not-found/error split (internal/transport
// cli.go) actually reaches RequireMarker's two distinct messages up here.
// marker.go already branches correctly on err vs !ok — the defect this task
// fixes was entirely inside CLI.Stat folding EVERY nonzero `filesystem info`
// exit into (_, false, nil), which would have made this test unable to ever
// observe the transport-failure branch: every Stat failure looked identical
// to a missing marker, so an operator running against a broken or
// uncertified CLI saw "it is not a git-remote-proton repo" instead of the
// real cause. With a transport whose Stat itself errors, RequireMarker must
// report its "stat ..." wrap (marker.go's err != nil branch), never the
// "no gpb-remote.json" absence message (the !ok branch) — those are two
// different failures and must stay distinguishable.
func TestRequireMarkerSurfacesStatFailureDistinctlyFromNoMarker(t *testing.T) {
	f := transport.NewFake()
	stub := statErrorTransport{f}

	err := RequireMarker(stub, "/my-files/r")
	if err == nil {
		t.Fatal("want a non-nil error when Stat itself fails")
	}
	if !strings.Contains(err.Error(), "stat ") {
		t.Errorf("a transport failure must surface as RequireMarker's stat-wrap message, got %q", err)
	}
	if strings.Contains(err.Error(), "no "+MarkerName) {
		t.Errorf("a transport failure must not be confused with the absent-marker message, got %q", err)
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
	f.Dirs["/r"] = true
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
	f.Dirs["/r"] = true
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

// TestWriteHEADFailsClosedOnUnrecognisedOutcome covers fix-round-1's Important
// finding: before the fix, WriteHEAD's outcome switch handled only Ambiguous
// and Refused, and let everything else — including a value nobody
// recognises — fall through into the read-back verification, where it would
// have been reported as Committed if the read-back happened to match. That is
// the same exposure AcquireLock, Bootstrap, publishPack and publishIdx are all
// already guarded against (each with its own default arm and test); WriteHEAD
// was the one CreateExclusive caller without it. Reuses
// unknownOutcomeTransport (defined above for the Bootstrap/AcquireLock
// variants of this exact test) rather than inventing a second stub, since the
// shape — force CreateExclusive to an out-of-range Outcome for one chosen
// path — is identical.
func TestWriteHEADFailsClosedOnUnrecognisedOutcome(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	tr := unknownOutcomeTransport{Fake: f, forPath: "/r/" + HeadName}

	out, err := WriteHEAD(tr, "/r", "refs/heads/main")
	if err == nil {
		t.Fatal("WriteHEAD must fail closed on an unrecognised outcome")
	}
	if out != transport.Ambiguous {
		t.Errorf("an unrecognised outcome must be reported as Ambiguous, got %v", out)
	}
	if !strings.Contains(err.Error(), "outcome(42)") {
		t.Errorf("the refusal must name the outcome it saw, got: %v", err)
	}
	if _, ok := f.Files["/r/HEAD"]; ok {
		t.Error("no HEAD was actually written; nothing may claim otherwise")
	}
}

// TestWriteHEADNeverOverwritesExistingHEAD covers the Refused path (fix-round
// coverage gap 1): CreateExclusive on an already-present HEAD reports Refused,
// and WriteHEAD must adopt that — report Refused with no error — rather than
// touching the existing content. Never-overwrite is the whole reason HEAD
// uses CreateExclusive and not UpdateRevision.
func TestWriteHEADNeverOverwritesExistingHEAD(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	f.Files["/r/HEAD"] = []byte("ref: refs/heads/existing\n")

	out, err := WriteHEAD(f, "/r", "refs/heads/other")
	if err != nil || out != transport.Refused {
		t.Fatalf("WriteHEAD over an existing HEAD must report (Refused, nil), got %v %v", out, err)
	}
	if got := string(f.Files["/r/HEAD"]); got != "ref: refs/heads/existing\n" {
		t.Errorf("the existing HEAD must be untouched, got %q", got)
	}
}

// TestWriteHEADAmbiguousOutcomeIsReported covers the Ambiguous path (fix-round
// coverage gap 2), using Fake.FailNext rather than a stub since Fake's own
// CreateExclusive already reports Ambiguous once when FailNext is set. Set
// AFTER Bootstrap: Bootstrap's own marker write is a CreateExclusive call too,
// and FailNext only fires on the next mutation, so setting it any earlier
// would consume it before WriteHEAD ever runs.
func TestWriteHEADAmbiguousOutcomeIsReported(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	f.FailNext = "inject"

	out, err := WriteHEAD(f, "/r", "refs/heads/main")
	if err == nil || out != transport.Ambiguous {
		t.Fatalf("an ambiguous CreateExclusive outcome must be reported as (Ambiguous, error), got %v %v", out, err)
	}
}

// TestWriteHEADRefusesNonBranchTarget covers the not-a-branch guard
// (fix-round coverage gap 3): WriteHEAD must reject a target outside
// refs/heads/ before ever touching the transport, mirroring the guard
// TestWriteRefRefusesNonSha already pins for WriteRef.
func TestWriteHEADRefusesNonBranchTarget(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")

	if out, err := WriteHEAD(f, "/r", "refs/tags/v1"); err == nil {
		t.Errorf("WriteHEAD must refuse a non-branch target, got %v, nil error", out)
	}
	if _, ok := f.Files["/r/HEAD"]; ok {
		t.Error("nothing must be written to the transport for a rejected target")
	}
}

// TestReadHEADAcceptsCRLF covers the CRLF round-trip (fix-round coverage gap
// 4). ReadHEAD's TrimRight("\r\n") already handles this; this test exists so a
// future "simplification" that narrows the trim to "\n" only cannot silently
// break a HEAD written or touched by a CRLF-writing tool.
func TestReadHEADAcceptsCRLF(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	f.Files["/r/HEAD"] = []byte("ref: refs/heads/main\r\n")

	branch, ok, err := ReadHEAD(f, "/r")
	if err != nil || !ok {
		t.Fatalf("ReadHEAD: %v %v", ok, err)
	}
	if branch != "refs/heads/main" {
		t.Errorf("ReadHEAD = %q, want refs/heads/main", branch)
	}
}

// --- Task 4: UpdateHEAD (overwrite-capable HEAD write) ----------------------

// RED: UpdateHEAD does not exist.

// 1  UpdateHEAD overwrites an existing HEAD and verifies by read-back.
func TestUpdateHEADOverwritesExistingAndVerifiesByReadBack(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	f.Files["/r/HEAD"] = []byte("ref: refs/heads/old\n")

	out, err := UpdateHEAD(f, "/r", "refs/heads/new")
	if err != nil || out != transport.Committed {
		t.Fatalf("UpdateHEAD: %v %v", out, err)
	}
	if got := string(f.Files["/r/HEAD"]); got != "ref: refs/heads/new\n" {
		t.Errorf("HEAD file = %q, want ref: refs/heads/new", got)
	}
	branch, ok, err := ReadHEAD(f, "/r")
	if err != nil || !ok {
		t.Fatalf("ReadHEAD: %v %v", ok, err)
	}
	if branch != "refs/heads/new" {
		t.Errorf("ReadHEAD = %q, want refs/heads/new", branch)
	}
}

// 2  UpdateHEAD creates HEAD when absent (the headless-remote rescue).
func TestUpdateHEADCreatesWhenAbsent(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")

	out, err := UpdateHEAD(f, "/r", "refs/heads/main")
	if err != nil || out != transport.Committed {
		t.Fatalf("UpdateHEAD: %v %v", out, err)
	}
	branch, ok, err := ReadHEAD(f, "/r")
	if err != nil || !ok {
		t.Fatalf("ReadHEAD: %v %v", ok, err)
	}
	if branch != "refs/heads/main" {
		t.Errorf("ReadHEAD = %q, want refs/heads/main", branch)
	}
}

// 3  UpdateHEAD refuses a non-branch target (mirrors WriteHEAD's rule).
func TestUpdateHEADRefusesNonBranchTarget(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")

	if out, err := UpdateHEAD(f, "/r", "refs/tags/v1"); err == nil {
		t.Errorf("UpdateHEAD must refuse a non-branch target, got %v, nil error", out)
	}
	if _, ok := f.Files["/r/HEAD"]; ok {
		t.Error("nothing must be written to the transport for a rejected target")
	}
}

// ambiguousUpdateTransport wraps a Fake but forces UpdateRevision to report
// Ambiguous for a chosen path. Fake's FailNext field only drives
// CreateExclusive (see fake.go), so UpdateHEAD's exists-branch — which calls
// UpdateRevision — needs its own stub to exercise the Ambiguous arm; this is
// the sibling unknownOutcomeTransport (line ~1205) does not provide, since
// that one overrides CreateExclusive only.
type ambiguousUpdateTransport struct {
	*transport.Fake
	forPath string
}

func (a ambiguousUpdateTransport) UpdateRevision(p, local string) (transport.Outcome, error) {
	if p == a.forPath {
		return transport.Ambiguous, nil
	}
	return a.Fake.UpdateRevision(p, local)
}

// 4  UpdateHEAD reports Ambiguous outcomes as re-run-to-reconcile errors, on
//
//	the UPDATE path (HEAD already exists), via ambiguousUpdateTransport
//	above.
func TestUpdateHEADAmbiguousOutcomeOnUpdatePath(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	f.Files["/r/HEAD"] = []byte("ref: refs/heads/old\n")
	tr := ambiguousUpdateTransport{Fake: f, forPath: "/r/" + HeadName}

	out, err := UpdateHEAD(tr, "/r", "refs/heads/new")
	if err == nil {
		t.Fatal("UpdateHEAD must report an error on an Ambiguous UpdateRevision outcome")
	}
	if out != transport.Ambiguous {
		t.Errorf("must report Ambiguous, got %v", out)
	}
	if !strings.Contains(err.Error(), "re-run to reconcile") {
		t.Errorf("must ask for a re-run to reconcile, got: %v", err)
	}
}

// unknownOutcomeUpdateTransport is unknownOutcomeTransport's sibling for the
// UPDATE path: UpdateHEAD's exists-branch calls UpdateRevision instead of
// CreateExclusive, so the fail-closed default arm needs its own outcome
// source to force there too.
type unknownOutcomeUpdateTransport struct {
	*transport.Fake
	forPath string
}

func (u unknownOutcomeUpdateTransport) UpdateRevision(p, local string) (transport.Outcome, error) {
	if p == u.forPath {
		return transport.Outcome(42), nil
	}
	return u.Fake.UpdateRevision(p, local)
}

// 5  UpdateHEAD fails closed on an unrecognised Outcome on BOTH paths
//
//	(mirrors TestWriteHEADFailsClosedOnUnrecognisedOutcome, line ~1336; the
//	update subtest needs the new decorator just above).
func TestUpdateHEADFailsClosedOnUnrecognisedOutcome(t *testing.T) {
	t.Run("create path", func(t *testing.T) {
		f := transport.NewFake()
		f.Dirs["/r"] = true
		_ = Bootstrap(f, "/r")
		tr := unknownOutcomeTransport{Fake: f, forPath: "/r/" + HeadName}

		out, err := UpdateHEAD(tr, "/r", "refs/heads/main")
		if err == nil {
			t.Fatal("UpdateHEAD must fail closed on an unrecognised outcome")
		}
		if out != transport.Ambiguous {
			t.Errorf("an unrecognised outcome must be reported as Ambiguous, got %v", out)
		}
		if !strings.Contains(err.Error(), "outcome(42)") {
			t.Errorf("the refusal must name the outcome it saw, got: %v", err)
		}
		if _, ok := f.Files["/r/HEAD"]; ok {
			t.Error("no HEAD was actually written; nothing may claim otherwise")
		}
	})

	t.Run("update path", func(t *testing.T) {
		f := transport.NewFake()
		f.Dirs["/r"] = true
		_ = Bootstrap(f, "/r")
		f.Files["/r/HEAD"] = []byte("ref: refs/heads/old\n")
		tr := unknownOutcomeUpdateTransport{Fake: f, forPath: "/r/" + HeadName}

		out, err := UpdateHEAD(tr, "/r", "refs/heads/new")
		if err == nil {
			t.Fatal("UpdateHEAD must fail closed on an unrecognised outcome")
		}
		if out != transport.Ambiguous {
			t.Errorf("an unrecognised outcome must be reported as Ambiguous, got %v", out)
		}
		if !strings.Contains(err.Error(), "outcome(42)") {
			t.Errorf("the refusal must name the outcome it saw, got: %v", err)
		}
		if got := string(f.Files["/r/HEAD"]); got != "ref: refs/heads/old\n" {
			t.Errorf("the existing HEAD must be untouched, got %q", got)
		}
	})
}

// 6  GUARD: WriteHEAD still never overwrites — TestWriteHEADNeverOverwritesExistingHEAD
//    (line ~1361, unmodified) is re-run as part of this same package's test
//    run and must stay green; UpdateHEAD is a separate function and does not
//    touch WriteHEAD's CreateExclusive-only, never-overwrite contract.

// refusedHeadCreateTransport wraps a Fake but forces CreateExclusive to
// report (Refused, nil) for the HEAD path specifically — modelling a
// concurrent non-v2 actor creating HEAD between UpdateHEAD's Stat check and
// its own CreateExclusive call. Mirrors refusedRefCreateTransport's shape
// (line ~915), which forces the same outcome under refs/heads/ for
// WriteRef's concurrent-creator case; this is the CREATE-path sibling for
// UpdateHEAD's Refused arm.
type refusedHeadCreateTransport struct {
	*transport.Fake
	forPath string
}

func (r refusedHeadCreateTransport) CreateExclusive(p, local string) (transport.Outcome, error) {
	if p == r.forPath {
		return transport.Refused, nil
	}
	return r.Fake.CreateExclusive(p, local)
}

// 18 GUARD (fix round 1, Important finding): UpdateHEAD's Refused arm is the
//
//	one arm whose behaviour is NOVEL versus WriteHEAD's — refuse rather
//	than adopt, because "the user asked for THIS branch" — and had no test
//	on either path. Before this test, the entire matrix stayed green even
//	if the arm were mutated to WriteHEAD's form (`return transport.Refused,
//	nil`), a plausible copy-paste regression whose live effect is a
//	non-v2 actor's HEAD silently reported as success. Covers both paths:
//	the update path via the Fake's own byte-identical Refused (modelling
//	probe C2, no new scaffolding needed — UpdateRevision on identical
//	bytes already returns Refused), and the create path via
//	refusedHeadCreateTransport above, which was equally cheap given the
//	existing refusedRefCreateTransport pattern to copy.
func TestUpdateHEADRefusedOutcomeIsAnError(t *testing.T) {
	t.Run("update path", func(t *testing.T) {
		f := transport.NewFake()
		f.Dirs["/r"] = true
		_ = Bootstrap(f, "/r")
		f.Files["/r/HEAD"] = []byte("ref: refs/heads/main\n")

		// Byte-identical rewrite: UpdateRevision reports Refused (fake.go).
		out, err := UpdateHEAD(f, "/r", "refs/heads/main")
		if err == nil {
			t.Fatal("UpdateHEAD must report an error on a Refused outcome, not adopt the existing HEAD")
		}
		if out != transport.Ambiguous {
			t.Errorf("a Refused write must be reported as Ambiguous, got %v", out)
		}
		if !strings.Contains(err.Error(), "refused") || !strings.Contains(err.Error(), "re-run to reconcile") {
			t.Errorf("must name the refusal and ask for a re-run, got: %v", err)
		}
	})

	t.Run("create path", func(t *testing.T) {
		f := transport.NewFake()
		f.Dirs["/r"] = true
		_ = Bootstrap(f, "/r")
		tr := refusedHeadCreateTransport{Fake: f, forPath: "/r/" + HeadName}

		out, err := UpdateHEAD(tr, "/r", "refs/heads/main")
		if err == nil {
			t.Fatal("UpdateHEAD must report an error on a Refused outcome, not adopt whatever landed")
		}
		if out != transport.Ambiguous {
			t.Errorf("a Refused write must be reported as Ambiguous, got %v", out)
		}
		if !strings.Contains(err.Error(), "refused") || !strings.Contains(err.Error(), "re-run to reconcile") {
			t.Errorf("must name the refusal and ask for a re-run, got: %v", err)
		}
		if _, ok := f.Files["/r/HEAD"]; ok {
			t.Error("nothing must be recorded as HEAD content on a refused create")
		}
	})
}

// --- Task 4: SetHead (the --set-head operation) -----------------------------

// RED: SetHead does not exist.

// assertLockReleased fails the test if the Fake still holds a .lock file for
// root. SetHead's defer must release the lock on every exit path, including
// every refusal below the point it successfully acquires one (scenario 15).
func assertLockReleased(t *testing.T, f *transport.Fake, root string) {
	t.Helper()
	if _, ok := f.Files[root+"/"+LockName]; ok {
		t.Errorf("SetHead must release its own lock on refusal, but %s/%s is still present", root, LockName)
	}
}

// 7  SetHead succeeds: two branches, HEAD at A, SetHead("b") → HEAD reads
//
//	back refs/heads/b; returns "refs/heads/b".
func TestSetHeadSucceeds(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	sha := "1111111111111111111111111111111111111111"
	if _, err := WriteRef(f, "/r", "refs/heads/a", sha, false); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteRef(f, "/r", "refs/heads/b", sha, false); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteHEAD(f, "/r", "refs/heads/a"); err != nil {
		t.Fatal(err)
	}

	got, err := SetHead(f, "/r", "b")
	if err != nil {
		t.Fatalf("SetHead: %v", err)
	}
	if got != "refs/heads/b" {
		t.Errorf("SetHead returned %q, want refs/heads/b", got)
	}
	branch, ok, err := ReadHEAD(f, "/r")
	if err != nil || !ok {
		t.Fatalf("ReadHEAD: %v %v", ok, err)
	}
	if branch != "refs/heads/b" {
		t.Errorf("HEAD = %q, want refs/heads/b", branch)
	}
}

// countingTransport wraps a Fake and counts CreateExclusive/UpdateRevision/
// Trash calls PER PATH, so an idempotence test can assert zero mutations to
// HEAD specifically rather than zero mutations overall — AcquireLock/Release
// always perform their own CreateExclusive/Trash against the .lock path, so
// a global counter would never read zero. transport.NewTraced is not usable
// here: it only instruments ReadTo, so it would report zero writes even when
// writes happen (peer-review finding). Trash is wrapped alongside the two
// write methods because an earlier version of this decorator left it
// uncounted: a Trash injected into SetHead's idempotent branch went
// undetected by the "zero writes" loop below even though it silently
// destroyed a branch ref (review mutation finding). EnsureDir is the one
// remaining Transport mutator this decorator does not wrap; SetHead's
// idempotent path never calls it, but a future caller that did would not be
// counted here.
type countingTransport struct {
	*transport.Fake
	writes map[string]int
}

func newCountingTransport(f *transport.Fake) *countingTransport {
	return &countingTransport{Fake: f, writes: map[string]int{}}
}

func (c *countingTransport) CreateExclusive(p, local string) (transport.Outcome, error) {
	c.writes[p]++
	return c.Fake.CreateExclusive(p, local)
}

func (c *countingTransport) UpdateRevision(p, local string) (transport.Outcome, error) {
	c.writes[p]++
	return c.Fake.UpdateRevision(p, local)
}

func (c *countingTransport) Trash(p string) (transport.Outcome, error) {
	c.writes[p]++
	return c.Fake.Trash(p)
}

// 8  Same-target idempotence: SetHead to the branch HEAD already names
//
//	succeeds WITHOUT uploading a new HEAD.
func TestSetHeadIdempotentMakesNoHeadWrite(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	sha := "1111111111111111111111111111111111111111"
	if _, err := WriteRef(f, "/r", "refs/heads/main", sha, false); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteHEAD(f, "/r", "refs/heads/main"); err != nil {
		t.Fatal(err)
	}

	c := newCountingTransport(f)
	got, err := SetHead(c, "/r", "main")
	if err != nil {
		t.Fatalf("SetHead: %v", err)
	}
	if got != "refs/heads/main" {
		t.Errorf("SetHead returned %q, want refs/heads/main", got)
	}
	// Zero CreateExclusive/UpdateRevision/Trash calls to EVERYTHING except the
	// lock path — AcquireLock's CreateExclusive and Release's Trash against
	// .lock are legitimate housekeeping. Asserting only /r/HEAD (as this test
	// originally did) would miss an idempotent run that mutated some OTHER
	// path — a ref, the marker — as a side effect, and counting only the two
	// write methods (an earlier version of this decorator) would miss a
	// Trash of that other path, since Trash is destructive but was not a
	// "write" the decorator counted (review mutation finding). EnsureDir is
	// the only remaining Transport mutator this loop does not cover.
	for p, n := range c.writes {
		if p != "/r/"+LockName && n != 0 {
			t.Errorf("same-target SetHead must write nothing but the lock; got %d write(s) to %s", n, p)
		}
	}
	// And the HEAD bytes really are untouched — zero counted writes proves
	// nothing about mutation through a path the decorator does not count.
	if head := string(f.Files["/r/HEAD"]); head != "ref: refs/heads/main\n" {
		t.Errorf("HEAD = %q after idempotent SetHead, want %q", head, "ref: refs/heads/main\n")
	}
}

// 9  Dangling HEAD refuses: HEAD names refs/heads/gone, branch "gone" does
//
//	not exist, SetHead("gone") → error naming the branches that DO exist.
//	This is the round-3 peer-review finding: branch existence must be
//	verified BEFORE the idempotence short-circuit below, or a HEAD already
//	naming a since-deleted branch would short-circuit straight to success.
func TestSetHeadRefusesDanglingHead(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	sha := "1111111111111111111111111111111111111111"
	if _, err := WriteRef(f, "/r", "refs/heads/main", sha, false); err != nil {
		t.Fatal(err)
	}
	f.Files["/r/HEAD"] = []byte("ref: refs/heads/gone\n")

	if _, err := SetHead(f, "/r", "gone"); err == nil {
		t.Fatal("SetHead must refuse a branch that does not exist, even though HEAD already names it")
	} else if !strings.Contains(err.Error(), "main") {
		t.Errorf("refusal must name the branches that DO exist, got: %v", err)
	}
	assertLockReleased(t, f, "/r")
}

// 10 Unknown branch refuses, error names existing branches.
func TestSetHeadRefusesUnknownBranch(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	sha := "1111111111111111111111111111111111111111"
	if _, err := WriteRef(f, "/r", "refs/heads/main", sha, false); err != nil {
		t.Fatal(err)
	}

	_, err := SetHead(f, "/r", "nope")
	if err == nil {
		t.Fatal("SetHead must refuse a branch that does not exist")
	}
	if !strings.Contains(err.Error(), "main") {
		t.Errorf("refusal must name existing branches, got: %v", err)
	}
	assertLockReleased(t, f, "/r")
}

// 11 Empty repo (marker present, no branches) refuses with "push a branch
//
//	first".
func TestSetHeadRefusesEmptyRepo(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")

	_, err := SetHead(f, "/r", "main")
	if err == nil {
		t.Fatal("SetHead must refuse when no branches exist")
	}
	if !strings.Contains(err.Error(), "push a branch first") {
		t.Errorf("refusal must say to push a branch first, got: %v", err)
	}
	assertLockReleased(t, f, "/r")
}

// 12 Hierarchical name ("feature/x") refuses naming Stage 5.
func TestSetHeadRefusesHierarchicalName(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	sha := "1111111111111111111111111111111111111111"
	if _, err := WriteRef(f, "/r", "refs/heads/main", sha, false); err != nil {
		t.Fatal(err)
	}

	_, err := SetHead(f, "/r", "feature/x")
	if err == nil {
		t.Fatal("SetHead must refuse a hierarchical branch name")
	}
	if !strings.Contains(err.Error(), "Stage 5") {
		t.Errorf("refusal must name Stage 5, got: %v", err)
	}
	assertLockReleased(t, f, "/r")
}

// 13 Tag target ("refs/tags/v1") refuses: HEAD points at branches only.
func TestSetHeadRefusesTagTarget(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	sha := "1111111111111111111111111111111111111111"
	if _, err := WriteRef(f, "/r", "refs/heads/main", sha, false); err != nil {
		t.Fatal(err)
	}

	_, err := SetHead(f, "/r", "refs/tags/v1")
	if err == nil {
		t.Fatal("SetHead must refuse a tag target")
	}
	if !strings.Contains(err.Error(), "branches only") {
		t.Errorf("refusal must say HEAD points at branches only, got: %v", err)
	}
	assertLockReleased(t, f, "/r")
}

// 14 No marker → RequireMarker's refusal, verbatim.
func TestSetHeadRefusesNoMarker(t *testing.T) {
	f := transport.NewFake()
	// Deliberately no Bootstrap: no marker present.

	_, err := SetHead(f, "/r", "main")
	if err == nil {
		t.Fatal("SetHead must refuse when there is no marker")
	}
	if !strings.Contains(err.Error(), MarkerName) {
		t.Errorf("refusal must be RequireMarker's own reason, got: %v", err)
	}
	assertLockReleased(t, f, "/r")
}

// 15 Lock held → AcquireLock's refusal; and (via assertLockReleased calls
//
//	threaded through scenarios 9-14 above) SetHead released ITS OWN lock on
//	every refusal path that got far enough to acquire one.
func TestSetHeadRefusesWhenLockHeld(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	held, err := AcquireLock(f, "/r")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer func() {
		if err := held.Release(); err != nil {
			t.Fatalf("release: %v", err)
		}
	}()

	if _, err := SetHead(f, "/r", "main"); err == nil {
		t.Fatal("SetHead must refuse while the repo is locked by someone else")
	}
	// The pre-existing lock is not SetHead's to release: it must still be
	// present, untouched, after SetHead's own AcquireLock call fails on it.
	if _, ok := f.Files["/r/"+LockName]; !ok {
		t.Error("the other holder's lock must survive SetHead's failed acquisition attempt")
	}
}

// 16 Short-name normalization: SetHead("main") == SetHead("refs/heads/main").
func TestSetHeadNormalizesShortName(t *testing.T) {
	build := func() *transport.Fake {
		f := transport.NewFake()
		f.Dirs["/r"] = true
		_ = Bootstrap(f, "/r")
		sha := "1111111111111111111111111111111111111111"
		if _, err := WriteRef(f, "/r", "refs/heads/main", sha, false); err != nil {
			t.Fatal(err)
		}
		return f
	}

	short, err := SetHead(build(), "/r", "main")
	if err != nil {
		t.Fatalf("SetHead(short): %v", err)
	}
	full, err := SetHead(build(), "/r", "refs/heads/main")
	if err != nil {
		t.Fatalf("SetHead(full): %v", err)
	}
	if short != full || short != "refs/heads/main" {
		t.Errorf("SetHead(%q) = %q, SetHead(%q) = %q; want both refs/heads/main",
			"main", short, "refs/heads/main", full)
	}
}

// 17 Corrupt or unreadable HEAD refuses (fail closed): make ReadHEAD error
//
//	(non-symref HEAD content) and assert SetHead returns that error WITHOUT
//	writing. A ReadHEAD error here is FATAL — it must never license an
//	overwrite, covering transient transport trouble as much as corrupt
//	content. The in-tool repair path for a genuinely corrupt HEAD is
//	delete-HEAD-via-web-UI (existing documented remedy), after which the
//	repo is headless and SetHead's create path applies.
func TestSetHeadRefusesCorruptHead(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	sha := "1111111111111111111111111111111111111111"
	if _, err := WriteRef(f, "/r", "refs/heads/main", sha, false); err != nil {
		t.Fatal(err)
	}
	// Non-symref content: a detached OID, not "ref: refs/heads/...".
	f.Files["/r/HEAD"] = []byte(sha + "\n")

	if _, err := SetHead(f, "/r", "main"); err == nil {
		t.Fatal("SetHead must fail closed when HEAD is corrupt, not overwrite it")
	}
	if got := string(f.Files["/r/HEAD"]); got != sha+"\n" {
		t.Errorf("corrupt HEAD must be left untouched, got %q", got)
	}
	assertLockReleased(t, f, "/r")
}

// RED. Push does not write HEAD at all today.
func TestPushWritesHeadOnFirstPush(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	d := newGitRepoForPush(t)
	head := headOfPushRepo(t, d)

	res := Push(f, "/r", d, []protocol.RefUpdate{{Src: head, Dst: "refs/heads/main"}}, map[string]string{})
	if len(res) != 1 || !res[0].OK {
		t.Fatalf("push: %+v", res)
	}
	if got := string(f.Files["/r/HEAD"]); got != "ref: refs/heads/main\n" {
		t.Errorf("HEAD = %q, want ref: refs/heads/main", got)
	}
}

// RED. Backfill: an existing repo with branches but no HEAD gets one, and the
// candidate set is every remote branch — not just what this push published.
func TestPushBackfillsHeadFromAllRemoteBranches(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	d := newGitRepoForPush(t)
	head := headOfPushRepo(t, d)

	// "alpha" is already on the remote from an earlier push; no HEAD exists.
	if _, err := WriteRef(f, "/r", "refs/heads/alpha", head, false); err != nil {
		t.Fatal(err)
	}
	remote := map[string]string{"refs/heads/alpha": head}

	// Today we push "zeta" only.
	res := Push(f, "/r", d, []protocol.RefUpdate{{Src: head, Dst: "refs/heads/zeta"}}, remote)
	if len(res) != 1 || !res[0].OK {
		t.Fatalf("push: %+v", res)
	}
	if got := string(f.Files["/r/HEAD"]); got != "ref: refs/heads/alpha\n" {
		t.Errorf("HEAD = %q — the candidate set must include branches this push did not touch", got)
	}
}

// GUARD. An existing HEAD is never rewritten.
func TestPushNeverRewritesAnExistingHead(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	d := newGitRepoForPush(t)
	head := headOfPushRepo(t, d)
	f.Files["/r/HEAD"] = []byte("ref: refs/heads/chosen\n")

	res := Push(f, "/r", d, []protocol.RefUpdate{{Src: head, Dst: "refs/heads/main"}}, map[string]string{})
	if len(res) != 1 || !res[0].OK {
		t.Fatalf("push: %+v", res)
	}
	if got := string(f.Files["/r/HEAD"]); got != "ref: refs/heads/chosen\n" {
		t.Errorf("an existing HEAD must not be touched, got %q", got)
	}
}

// GUARD. A tag-only push leaves the repo headless — a defined state.
func TestPushTagOnlyLeavesRepoHeadless(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	d := newGitRepoForPush(t)
	head := headOfPushRepo(t, d)

	res := Push(f, "/r", d, []protocol.RefUpdate{{Src: head, Dst: "refs/tags/v1"}}, map[string]string{})
	if len(res) != 1 || !res[0].OK {
		t.Fatalf("push: %+v", res)
	}
	if _, ok := f.Files["/r/HEAD"]; ok {
		t.Error("a tag-only push must not write HEAD")
	}
}

// RED. The design's ref-transition table is normative on this row: "Delete
// (`push :dst`) | Trash; refuse to delete the branch HEAD points at". Nothing
// implemented it, so an ordinary `git push proton-v2 --delete main` against a
// remote whose HEAD names main trashed the ref and left HEAD dangling — a
// state a later clone cannot recover from, because ensureHEAD returns early
// whenever a HEAD exists and v2 never rewrites one.
//
// The refusal is per-ref, not per-batch: other updates in the same push are
// unaffected, which is why it is expressed as a failed Result rather than an
// error out of Push.
func TestPushRefusesToDeleteTheBranchHeadPointsAt(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	sha := "2222222222222222222222222222222222222222"
	if _, err := WriteRef(f, "/r", "refs/heads/main", sha, false); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteHEAD(f, "/r", "refs/heads/main"); err != nil {
		t.Fatal(err)
	}

	ups := []protocol.RefUpdate{{Src: "", Dst: "refs/heads/main"}}
	res := Push(f, "/r", t.TempDir(), ups, map[string]string{"refs/heads/main": sha})

	if len(res) != 1 {
		t.Fatalf("want one result, got %+v", res)
	}
	if res[0].OK {
		t.Fatalf("deleting the branch HEAD points at must be refused, got OK: %+v", res[0])
	}
	if !strings.Contains(res[0].Err, "HEAD points at") {
		t.Errorf("reason = %q, want it to name why the delete was refused", res[0].Err)
	}
	if !strings.Contains(res[0].Err, "--set-head") {
		t.Errorf("the refusal must name the in-tool remedy, got %q", res[0].Err)
	}
	if _, ok := f.Files["/r/refs/heads/main"]; !ok {
		t.Error("the ref file must survive a refused delete")
	}
	if got := string(f.Files["/r/HEAD"]); got != "ref: refs/heads/main\n" {
		t.Errorf("HEAD = %q, must be left alone", got)
	}
}

// GUARD. The refusal must be scoped to the branch HEAD actually names — a
// delete of any OTHER branch still succeeds. Without this, "refuse every
// delete" would pass the test above.
func TestPushDeletesABranchHeadDoesNotPointAt(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	sha := "2222222222222222222222222222222222222222"
	for _, ref := range []string{"refs/heads/main", "refs/heads/dev"} {
		if _, err := WriteRef(f, "/r", ref, sha, false); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := WriteHEAD(f, "/r", "refs/heads/main"); err != nil {
		t.Fatal(err)
	}

	ups := []protocol.RefUpdate{{Src: "", Dst: "refs/heads/dev"}}
	res := Push(f, "/r", t.TempDir(), ups,
		map[string]string{"refs/heads/main": sha, "refs/heads/dev": sha})

	if len(res) != 1 || !res[0].OK {
		t.Fatalf("deleting a branch HEAD does not point at must succeed: %+v", res)
	}
	if _, ok := f.Files["/r/refs/heads/dev"]; ok {
		t.Error("the ref file must be gone")
	}
}

// RED. A HEAD that cannot be read is not licence to delete. ReadHEAD treats
// content that is not a branch symref as fatal (never coerced), and a delete
// taken on the strength of an unreadable HEAD could be exactly the delete the
// rule above exists to refuse.
func TestPushDeleteFailsClosedOnAnUnreadableHead(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	sha := "2222222222222222222222222222222222222222"
	if _, err := WriteRef(f, "/r", "refs/heads/main", sha, false); err != nil {
		t.Fatal(err)
	}
	f.Files["/r/HEAD"] = []byte("this is not a symref\n")

	ups := []protocol.RefUpdate{{Src: "", Dst: "refs/heads/main"}}
	res := Push(f, "/r", t.TempDir(), ups, map[string]string{"refs/heads/main": sha})

	if len(res) != 1 {
		t.Fatalf("want one result, got %+v", res)
	}
	if res[0].OK {
		t.Fatalf("a delete must fail closed when HEAD cannot be read, got OK: %+v", res[0])
	}
	if !strings.Contains(res[0].Err, "HEAD") {
		t.Errorf("reason = %q, want it to name the HEAD read failure", res[0].Err)
	}
	if _, ok := f.Files["/r/refs/heads/main"]; !ok {
		t.Error("the ref file must survive a delete that failed closed")
	}
}

// GUARD (ensureHEAD delete arithmetic). A batch that deletes a branch and
// recreates it must leave that branch in the candidate set: the delete removes
// it from `seen`, and the recreate in the same batch has to put it back.
// Getting the arithmetic wrong here would leave a repo headless that has a
// perfectly good branch on it.
func TestPushDeleteThenRecreateInOneBatchStillDerivesHead(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	d := newGitRepoForPush(t)
	head := headOfPushRepo(t, d)
	if _, err := WriteRef(f, "/r", "refs/heads/main", head, false); err != nil {
		t.Fatal(err)
	}
	// No HEAD on the remote: this is the backfill case, and it is also why
	// the delete refusal above does not fire here.

	ups := []protocol.RefUpdate{
		{Src: "", Dst: "refs/heads/main"},
		{Src: head, Dst: "refs/heads/main"},
	}
	res := Push(f, "/r", d, ups, map[string]string{"refs/heads/main": head})
	for _, r := range res {
		if !r.OK {
			t.Fatalf("both updates must succeed: %+v", res)
		}
	}
	if got := string(f.Files["/r/HEAD"]); got != "ref: refs/heads/main\n" {
		t.Errorf("HEAD = %q, want ref: refs/heads/main — a branch deleted and recreated in "+
			"one batch is still a candidate", got)
	}
}

// refWriteMethodTransport wraps a Fake and records which write method —
// CreateExclusive or UpdateRevision — was actually invoked for ONE chosen
// remote path. Used to pin fix round M4's batch-aware `exists` flip
// directly: a Fake's UpdateRevision against an already-absent path silently
// succeeds as a blind write (see fake.go), so outcomes alone cannot
// distinguish "took the create path" from "took the update path and got
// lucky against the Fake's permissive behaviour" — this makes the actual
// method call observable.
type refWriteMethodTransport struct {
	*transport.Fake
	watch  string
	method *string
}

func (r refWriteMethodTransport) CreateExclusive(p, local string) (transport.Outcome, error) {
	if p == r.watch {
		*r.method = "create"
	}
	return r.Fake.CreateExclusive(p, local)
}

func (r refWriteMethodTransport) UpdateRevision(p, local string) (transport.Outcome, error) {
	if p == r.watch {
		*r.method = "update"
	}
	return r.Fake.UpdateRevision(p, local)
}

// TestPushDeleteThenRecreateInOneBatchOrderIndependent is RED (fix round I1
// + M4): the sibling test above already pins "delete X, recreate X" with
// the DELETE listed first; this reverses the order — the update/recreate is
// listed FIRST, the delete SECOND — to actually exercise the order-
// independence claim rather than merely assert it in a comment (fix round
// I1's finding: a naive single in-order pass over ups only gets this shape
// right when the delete happens to be listed first). It also pins fix round
// M4 directly: phase 5's `exists` must be BATCH-AWARE, so the update routes
// through CreateExclusive — the one combination live-verified against the
// real CLI (C17b, create-after-trash, 30/30) — rather than UpdateRevision
// against a node phase 4 already trashed, which was never verified live.
func TestPushDeleteThenRecreateInOneBatchOrderIndependent(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	d := newGitRepoForPush(t)
	head := headOfPushRepo(t, d)
	if _, err := WriteRef(f, "/r", "refs/heads/main", head, false); err != nil {
		t.Fatal(err)
	}
	remote := map[string]string{"refs/heads/main": head}

	var method string
	tr := refWriteMethodTransport{Fake: f, watch: "/r/refs/heads/main", method: &method}

	ups := []protocol.RefUpdate{
		{Src: head, Dst: "refs/heads/main"}, // update/recreate, listed FIRST
		{Src: "", Dst: "refs/heads/main"},   // delete, listed SECOND
	}
	res := Push(tr, "/r", d, ups, remote)
	for _, r := range res {
		if !r.OK {
			t.Fatalf("both updates must succeed regardless of input order: %+v", res)
		}
	}
	if method != "create" {
		t.Errorf("phase 5 must route through CreateExclusive after a same-batch delete "+
			"(fix round M4), got method=%q", method)
	}
	sha, err := readRef(f, "/r/refs/heads/main")
	if err != nil || sha != head {
		t.Fatalf("ref not published correctly: sha=%q err=%v want=%q", sha, err, head)
	}
	if got := string(f.Files["/r/HEAD"]); got != "ref: refs/heads/main\n" {
		t.Errorf("HEAD = %q, want ref: refs/heads/main — the ensureHEAD candidate-set "+
			"arithmetic must also be order-independent (fix round I1)", got)
	}
}

// trashFailsAndMethodTracked wraps a Fake, forcing Trash to fail (report
// Ambiguous, nil — the node is left completely untouched, exactly as if the
// trash never ran) for ONE chosen path, while also recording which write
// method — CreateExclusive or UpdateRevision — fires for another chosen
// path. Built specifically to pin fix round 2's correction to M4: a
// same-batch delete that FAILS in phase 4 must not flip the paired
// create/update's routing to CreateExclusive, because the node the delete
// was supposed to remove is still actually there.
type trashFailsAndMethodTracked struct {
	*transport.Fake
	failTrash string
	watch     string
	method    *string
}

func (tr trashFailsAndMethodTracked) Trash(p string) (transport.Outcome, error) {
	if p == tr.failTrash {
		return transport.Ambiguous, nil
	}
	return tr.Fake.Trash(p)
}

func (tr trashFailsAndMethodTracked) CreateExclusive(p, local string) (transport.Outcome, error) {
	if p == tr.watch {
		*tr.method = "create"
	}
	return tr.Fake.CreateExclusive(p, local)
}

func (tr trashFailsAndMethodTracked) UpdateRevision(p, local string) (transport.Outcome, error) {
	if p == tr.watch {
		*tr.method = "update"
	}
	return tr.Fake.UpdateRevision(p, local)
}

// TestPushUpdateRoutesViaUpdateRevisionWhenSameBatchDeleteFails is RED (fix
// round 2, Important): deletedThisBatch must be built from phase 4's ACTUAL
// outcome, not phase-2 validity. A batch deletes refs/heads/main (valid at
// phase 2) AND updates it in the same batch; the transport's Trash is forced
// to fail for that exact path (Ambiguous, node left untouched — a HEAD
// written between phases by a non-v2 actor, or a transient transport fault,
// would produce the same shape). The delete must report its own failure.
// The update — which is unrelated to why the delete failed, and whose
// target node is still genuinely occupied — must still succeed, and it must
// do so via UpdateRevision (the node is there; nothing concurrent
// happened), NOT CreateExclusive. Before this fix, deletedThisBatch was
// built from phase-2 validity alone, so the FAILED delete still flipped the
// update to CreateExclusive, which then hit the still-present node and
// reported the wrong diagnosis: "ref changed concurrently; refusing to
// overwrite" — nothing concurrent happened, the batch's own delete simply
// failed.
func TestPushUpdateRoutesViaUpdateRevisionWhenSameBatchDeleteFails(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	d := newGitRepoForPush(t)
	head := headOfPushRepo(t, d)
	if _, err := WriteRef(f, "/r", "refs/heads/main", head, false); err != nil {
		t.Fatal(err)
	}
	remote := map[string]string{"refs/heads/main": head}
	newHead := commitOnPushRepo(t, d, "b.txt", "two") // descends from head: a genuine fast-forward

	var method string
	tr := trashFailsAndMethodTracked{
		Fake:      f,
		failTrash: "/r/refs/heads/main",
		watch:     "/r/refs/heads/main",
		method:    &method,
	}

	ups := []protocol.RefUpdate{
		{Src: "", Dst: "refs/heads/main"},      // delete — will FAIL (Trash forced Ambiguous)
		{Src: newHead, Dst: "refs/heads/main"}, // update — must still succeed via UpdateRevision
	}
	res := Push(tr, "/r", d, ups, remote)
	if len(res) != 2 {
		t.Fatalf("want 2 results, got %+v", res)
	}
	if res[0].OK {
		t.Fatalf("the delete must report its own failure, got %+v", res[0])
	}
	if !strings.Contains(res[0].Err, "delete failed") {
		t.Errorf("delete failure must name itself, got %q", res[0].Err)
	}
	if !res[1].OK {
		t.Fatalf("the update must still succeed despite the same-batch delete's failure, got %+v", res[1])
	}
	if method != "update" {
		t.Errorf("the update must route via UpdateRevision — the node is still occupied because "+
			"the batch's own delete failed, nothing concurrent happened — got method=%q", method)
	}
	sha, err := readRef(f, "/r/refs/heads/main")
	if err != nil || sha != newHead {
		t.Fatalf("ref not published correctly: sha=%q err=%v want=%q", sha, err, newHead)
	}
}

// GUARD (ensureHEAD delete arithmetic). Deleting the only branch leaves no
// candidates at all, and headless is a DEFINED state: no HEAD is written, and
// nothing fails.
func TestPushDeleteOfTheOnlyBranchLeavesTheRepoHeadless(t *testing.T) {
	f := transport.NewFake()
	f.Dirs["/r"] = true
	_ = Bootstrap(f, "/r")
	sha := "2222222222222222222222222222222222222222"
	if _, err := WriteRef(f, "/r", "refs/heads/main", sha, false); err != nil {
		t.Fatal(err)
	}
	// No HEAD, so the delete is permitted and ensureHEAD's backfill runs
	// against an empty candidate set afterwards.

	ups := []protocol.RefUpdate{{Src: "", Dst: "refs/heads/main"}}
	res := Push(f, "/r", t.TempDir(), ups, map[string]string{"refs/heads/main": sha})

	if len(res) != 1 || !res[0].OK {
		t.Fatalf("delete should succeed when no HEAD points at the branch: %+v", res)
	}
	if _, ok := f.Files["/r/HEAD"]; ok {
		t.Errorf("HEAD = %q, want none: deleting the only branch leaves the repo headless, "+
			"which is a defined state", f.Files["/r/HEAD"])
	}
}

// emptyGitRepo returns a repo with NO commits.
//
// It exists because newGitRepoForPush builds an identical commit every time —
// same content "one", same message "c1", same author — and a git commit sha
// covers the author and committer timestamps at one-second resolution. Two
// such repos created inside the same second get the SAME sha, so a "fetch into
// a repo that lacks the objects" test would silently be fetching into a repo
// that already has them: Fetch's up-to-date short-circuit (ConnectivityOK
// against gitDir alone, no alternate — see fetch.go) would see the object
// already present and return ("", nil), and the test fails for a reason
// unrelated to the code under test. Flaky by the clock, which is worse than
// simply wrong.
func emptyGitRepo(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	for _, a := range [][]string{
		{"init", "-qb", "main"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"},
	} {
		if err := exec.Command("git", append([]string{"-C", d}, a...)...).Run(); err != nil {
			t.Fatalf("git %v: %v", a, err)
		}
	}
	return d
}

// assertSamePackDir fails the test unless want and got name the same
// directory on disk, robust to the 8.3 short-name aliasing that first
// surfaced this comparison as flaky (CI on GitHub's windows-latest runner,
// first-ever run of the Go suite): the runner's TEMP is set to a DOS 8.3
// short spelling (e.g. RUNNER~1), so a want path built by joining
// t.TempDir() onto other segments inherits that short spelling, while got
// -- consolidateAndInstall's `git rev-parse --git-path objects/pack` answer
// -- comes back resolved to the long form (runneradmin) by git/Windows. Same
// physical directory, two spellings; a raw string compare treats that as two
// different directories and fails a correct fetch. Reproduced locally by
// pointing TMP/TEMP at a short-form alias of a scratch directory and running
// the affected test (see the fix report) -- it fails with exactly this
// shape, want and got spelled differently but identical up to the alias.
//
// filepath.EvalSymlinks resolves an 8.3 alias to its final long-form
// spelling on Windows (it is not merely a symlink resolver there -- see its
// doc), so running both sides through it before comparing absorbs the alias
// without weakening the assertion: it must still fail if the pack genuinely
// lands in a different directory, which EvalSymlinks alone would still
// report as a mismatch. os.SameFile is a second, stat-identity-based check
// used only if the normalised strings still differ -- it catches a residual
// false negative (a trailing separator, a UNC-prefix difference) using
// filesystem identity (device+inode / file index) rather than string shape,
// again without accepting a genuinely different directory as a match.
//
// If EvalSymlinks fails on either side (e.g. the directory does not exist),
// that failure is folded into the failure message rather than silently
// passing or panicking -- a missing directory is itself a real assertion
// failure, not grounds to skip the check.
func assertSamePackDir(t *testing.T, context, want, got string) {
	t.Helper()
	wantEval, wErr := filepath.EvalSymlinks(want)
	gotEval, gErr := filepath.EvalSymlinks(got)
	if wErr == nil && gErr == nil {
		if wantEval == gotEval {
			return
		}
		wantInfo, wStatErr := os.Stat(wantEval)
		gotInfo, gStatErr := os.Stat(gotEval)
		if wStatErr == nil && gStatErr == nil && os.SameFile(wantInfo, gotInfo) {
			return
		}
		t.Errorf("%s %s, got %s (normalised: want=%s got=%s)", context, want, got, wantEval, gotEval)
		return
	}
	// Could not normalise one or both sides -- fall back to a raw compare so
	// a genuine mismatch is still reported, with the normalisation errors
	// folded in for diagnosis.
	if want != got {
		t.Errorf("%s %s, got %s (path normalisation failed: want-err=%v got-err=%v)",
			context, want, got, wErr, gErr)
	}
}

// plantRepoOnFake pushes a real repo's history into a Fake as a v2 remote:
// marker, refs, and one pack pair under packs/.
func plantRepoOnFake(t *testing.T, f *transport.Fake, root, gitDir, sha string) {
	t.Helper()
	// root's own parent may not be a mount root (every caller here passes
	// "/r"), so the stricter EnsureDir (Task 7) needs root seeded as
	// already-existing before Bootstrap can create things under it.
	f.Dirs[root] = true
	if err := Bootstrap(f, root); err != nil {
		t.Fatal(err)
	}
	packPath, idxPath, err := gitcmd.WritePack(gitDir, []string{sha}, nil, t.TempDir())
	if err != nil || packPath == "" {
		t.Fatalf("WritePack: %v", err)
	}
	for _, p := range []string{packPath, idxPath} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f.Files[root+"/packs/"+filepath.Base(p)] = b
	}
	if _, err := WriteRef(f, root, "refs/heads/main", sha, false); err != nil {
		t.Fatal(err)
	}
}

// RED. Fetch does not exist. Real git on both ends, the Fake in between.
func TestFetchInstallsTheClosure(t *testing.T) {
	src := newGitRepoForPush(t)
	sha := headOfPushRepo(t, src)
	f := transport.NewFake()
	plantRepoOnFake(t, f, "/r", src, sha)

	dst := emptyGitRepo(t) // genuinely lacks src's objects — see emptyGitRepo
	keep, err := Fetch(f, "/r", dst, "", []string{sha})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if keep == "" {
		t.Fatal("a fetch that installs objects must return a .keep path")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf(".keep must exist on disk: %v", err)
	}
	if !gitcmd.HasObject(dst, sha) {
		t.Error("the wanted object must be present after a fetch")
	}

	// Fix round 1 test strengthening: pin the one-pack invariant the caller's
	// single lock response depends on (consolidateAndInstall must produce
	// exactly ONE pack, never one per downloaded remote pack), and pin that
	// .keep lands beside it in the REAL object store — not in the temp
	// alt-objects area, and not anywhere filepath.IsAbs's fallback join could
	// misplace it (I2, fix round 1).
	if n := countInstalledPackFiles(t, dst); n != 1 {
		t.Errorf("want exactly 1 installed pack, got %d", n)
	}
	wantDir := filepath.Join(dst, ".git", "objects", "pack")
	assertSamePackDir(t, ".keep must live in", wantDir, filepath.Dir(keep))
}

// RED. A second fetch of the same want installs nothing. This is what
// Fetch's up-to-date short-circuit buys (ConnectivityOK against gitDir alone,
// no alternate — see fetch.go's comment on it), NOT what RevListNewObjects's
// own --not --all buys by itself: that flag excludes by ref reachability, and
// Fetch never writes local refs (git's own porcelain does that, after this
// helper exits), so on a destination like dst below — the object physically
// present, no ref reaching it — --not --all alone would recompute and
// reinstall the full closure on every call.
//
// Fix round 1 test strengthening: the packs are deleted from the Fake before
// the second call, so this proves BOTH that "present but unreferenced"
// converges to ("", nil) AND that a genuinely up-to-date fetch never touches
// the remote — before this change the test passed identically with the
// short-circuit deleted, since the packs were still there to redownload.
func TestFetchIsIdempotentAndInstallsNothingWhenUpToDate(t *testing.T) {
	src := newGitRepoForPush(t)
	sha := headOfPushRepo(t, src)
	f := transport.NewFake()
	plantRepoOnFake(t, f, "/r", src, sha)

	dst := emptyGitRepo(t)
	if _, err := Fetch(f, "/r", dst, "", []string{sha}); err != nil {
		t.Fatalf("first fetch: %v", err)
	}

	// Remove every pack/idx from the remote. If the second Fetch call had to
	// redownload anything, it would fail outright (Fetch fatals when
	// listCompletePacks comes back with len(stems) == 0, i.e. an empty
	// packs/ folder) rather than merely reinstalling redundantly — so this
	// also proves the remote is never touched once up to date.
	for name := range f.Files {
		if strings.HasSuffix(name, ".pack") || strings.HasSuffix(name, ".idx") {
			delete(f.Files, name)
		}
	}

	keep, err := Fetch(f, "/r", dst, "", []string{sha})
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if keep != "" {
		t.Errorf("an up-to-date fetch must install nothing, got keep %q", keep)
	}
}

// RED. A corrupt remote pack must be fatal, and must leave the local store
// untouched — fetch is the one path that can damage the user's own repo.
//
// Fix round 1 test strengthening: a genuine first fetch runs BEFORE the
// corruption is injected, so `before` is a real 1 (a pack this test proves
// was actually installed), not merely 0 by construction. Before this change
// `before == after == 0` would have passed even if downloadAndVerifyPack's
// checksum/pair verification were entirely deleted, since dst never had a
// pack to lose either way.
func TestFetchRejectsACorruptPackAndInstallsNothing(t *testing.T) {
	src := newGitRepoForPush(t)
	sha := headOfPushRepo(t, src)
	f := transport.NewFake()
	plantRepoOnFake(t, f, "/r", src, sha)

	dst := emptyGitRepo(t)
	if _, err := Fetch(f, "/r", dst, "", []string{sha}); err != nil {
		t.Fatalf("priming fetch: %v", err)
	}
	before := countInstalledPackFiles(t, dst)
	if before != 1 {
		t.Fatalf("priming fetch must have installed exactly 1 pack, got %d", before)
	}

	// A second, distinct want, so the corrupted pack is actually needed —
	// otherwise the up-to-date short-circuit would return ("", nil) before
	// ever downloading (and therefore before ever reading) the corrupt pack,
	// and this test would pass without exercising downloadAndVerifyPack's
	// verification at all. plantRepoOnFake already wrote one pack pair for
	// `sha`; a second commit on src, packed and planted under a second
	// remote name, gives Fetch a want whose closure is not yet satisfied
	// locally.
	second := commitOnPushRepo(t, src, "b.txt", "two")
	packPath, idxPath, err := gitcmd.WritePack(src, []string{second}, []string{sha}, t.TempDir())
	if err != nil || packPath == "" {
		t.Fatalf("WritePack: %v", err)
	}
	for _, p := range []string{packPath, idxPath} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f.Files["/r/packs/"+filepath.Base(p)] = b
	}

	for name, b := range f.Files {
		if strings.HasSuffix(name, ".pack") {
			c := append([]byte(nil), b...)
			c[len(c)/2] ^= 0xff
			f.Files[name] = c
		}
	}

	if _, err := Fetch(f, "/r", dst, "", []string{second}); err == nil {
		t.Fatal("a corrupt remote pack must be fatal")
	}
	if got := countInstalledPackFiles(t, dst); got != before {
		t.Errorf("a failed fetch must install nothing: pack count %d -> %d", before, got)
	}
}

// RED. Fetch is read-only: no marker means refuse, never initialise.
func TestFetchRefusesAnUnmarkedRemote(t *testing.T) {
	f := transport.NewFake()
	// "/r"'s parent "/" is not a mount root, so the stricter EnsureDir
	// (Task 7) needs it seeded as already-existing.
	f.Dirs["/r"] = true
	dst := emptyGitRepo(t)
	if _, err := Fetch(f, "/r", dst, "", []string{"1111111111111111111111111111111111111111"}); err == nil {
		t.Error("fetch must refuse a folder with no marker, not initialise it")
	}
	if _, ok := f.Files["/r/gpb-remote.json"]; ok {
		t.Error("fetch must never create a marker")
	}
}

// linkedWorktree creates a linked worktree (`git worktree add`) off mainRepo
// and returns its checkout path. mainRepo needs no commits — worktree add
// works fine off an unborn HEAD (confirmed empirically) — which is exactly
// why TestFetchIntoALinkedWorktreeInstallsWhereGitLooks below builds mainRepo
// with emptyGitRepo rather than newGitRepoForPush: two independently created
// newGitRepoForPush commits can collide on sha within the same wall-clock
// second (see emptyGitRepo's own doc), and a colliding mainRepo would already
// share the fetch's wanted object before Fetch ever ran, defeating the test.
// emptyGitRepo has no commit to collide with anything.
func linkedWorktree(t *testing.T, mainRepo string) string {
	t.Helper()
	wt := filepath.Join(t.TempDir(), "wt")
	if err := exec.Command("git", "-C", mainRepo, "worktree", "add", "-q", wt, "-b", "wt-branch").Run(); err != nil {
		t.Fatalf("git worktree add: %v", err)
	}
	return wt
}

// RED (fix round 2, confirmed gap). A linked worktree's `git rev-parse
// --git-dir` answers with the per-worktree ADMIN directory
// (<main>/.git/worktrees/<name>), which holds HEAD and the index but has NO
// object store of its own — git resolves objects through the worktree's
// COMMON dir instead (<main>/.git/objects) and never looks in the admin
// dir's objects/ at all. Before this fix, consolidateAndInstall asked
// --git-dir and packed into "<admin-dir>/objects/pack": Fetch reported
// success and a real .keep existed on disk, but every object it installed
// was invisible to git — with check-connectivity, the caller would skip its
// own check (connectivity-ok) and update refs anyway, producing refs that
// point at objects git can never see. Confirmed this exact failure mode by
// hand against real git before writing this test (see the fix report).
//
// Empirically verified (see the fix report) that `git rev-parse --git-path
// objects/pack` resolves through the commondir indirection correctly in
// every layout tested — ordinary repo, bare repo, and linked worktree — which
// is what the fix below asks instead.
func TestFetchIntoALinkedWorktreeInstallsWhereGitLooks(t *testing.T) {
	src := newGitRepoForPush(t)
	sha := headOfPushRepo(t, src)
	f := transport.NewFake()
	plantRepoOnFake(t, f, "/r", src, sha)

	main := emptyGitRepo(t) // no commits — see linkedWorktree for why
	wt := linkedWorktree(t, main)

	keep, err := Fetch(f, "/r", wt, "", []string{sha})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if keep == "" {
		t.Fatal("a fetch that installs objects must return a .keep path")
	}
	if !gitcmd.HasObject(wt, sha) {
		t.Error("the wanted object must be visible to git FROM THE WORKTREE — installing " +
			"into the worktree's admin dir instead of its common dir would make the pack " +
			"invisible to every git command, even though Fetch reported success")
	}
	wantDir := filepath.Join(main, ".git", "objects", "pack")
	assertSamePackDir(t, ".keep must live in the MAIN repo's real object store", wantDir, filepath.Dir(keep))
}

// chdirForTest changes the process's working directory to dir for the
// duration of the test and restores it afterward via t.Cleanup.
//
// Not testing.Chdir: that requires Go 1.24, and this module's go.mod
// deliberately floors at go 1.22 (documented there — chosen for
// per-iteration loop variables; the floor was never meant to gate on
// testing.Chdir, and bumping the module's declared floor for one test's
// convenience is a bigger decision than this fix belongs to). No test in
// this package calls t.Parallel(), so the process-wide cwd this mutates is
// not at risk of a concurrent sibling test reading it mid-change; if that
// ever stops being true, this helper (and the test using it) must move to a
// serial-only subset or be reworked.
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

// RED (fix round 3, live-gate finding 2, task 7). Reproduces the exact shape
// git actually uses when it spawns this helper: GIT_DIR is set RELATIVE
// (".git" is git's own default) while the process cwd is the worktree root —
// the ORDINARY case, not an edge case. Every other Fetch test in this file
// passes an ABSOLUTE t.TempDir()-derived gitDir, which is exactly why none of
// them could catch this.
//
// Before the fix, consolidateAndInstall's `git -C gitDir rev-parse
// --git-path objects/pack` answer came back relative (e.g. "objects/pack"),
// got joined onto gitDir to make it relative to the ORIGINAL process cwd
// (correctly, at that point), and was then handed as an outStem to a SECOND
// `git -C gitDir pack-objects ...` invocation. -C changes the effective
// working directory for resolving relative ARGUMENTS too, so that second
// subprocess resolved the already-gitDir-relative path a SECOND time,
// relative to gitDir again — doubling the prefix into
// "<gitDir>/<gitDir>/objects/pack/...", which does not exist. Observed live
// on Windows exactly this way (Stage 3a gate, task 7): "pack-objects: error:
// unable to write file .git\objects\pack\pack-....pack: No such file or
// directory" / "unable to rename temporary file to
// '.git\objects\pack\pack-....pack'" — and proton-v2/main never advanced.
func TestFetchWithARelativeGitDirInstallsCorrectly(t *testing.T) {
	src := newGitRepoForPush(t)
	sha := headOfPushRepo(t, src)
	f := transport.NewFake()
	plantRepoOnFake(t, f, "/r", src, sha)

	dst := emptyGitRepo(t)
	chdirForTest(t, dst) // the process cwd IS the worktree — exactly what git sets up

	keep, err := Fetch(f, "/r", ".git", "", []string{sha}) // RELATIVE gitDir, the live shape
	if err != nil {
		t.Fatalf("Fetch with a relative gitDir: %v", err)
	}
	if keep == "" {
		t.Fatal("a fetch that installs objects must return a .keep path")
	}
	if !gitcmd.HasObject(".git", sha) {
		t.Error("the wanted object must be present after a fetch with a relative gitDir")
	}
	wantDir := filepath.Join(dst, ".git", "objects", "pack")
	assertSamePackDir(t, ".keep must live in", wantDir, filepath.Dir(keep))
}

// plantIncrementalPacks pushes src's history onto the Fake as N incremental
// packs — shas[i]'s pack contains its closure minus shas[i-1]'s — with the
// ref left at the LAST sha. Returns the stems in history order.
func plantIncrementalPacks(t *testing.T, f *transport.Fake, root, src string, shas []string) []string {
	t.Helper()
	// root's own parent may not be a mount root (every caller here passes
	// "/r"), so the stricter EnsureDir (Task 7) needs root seeded as
	// already-existing before Bootstrap can create things under it.
	f.Dirs[root] = true
	if err := Bootstrap(f, root); err != nil {
		t.Fatal(err)
	}
	var stems []string
	for i, sha := range shas {
		var haves []string
		if i > 0 {
			haves = []string{shas[i-1]}
		}
		packPath, idxPath, err := gitcmd.WritePack(src, []string{sha}, haves, t.TempDir())
		if err != nil || packPath == "" {
			t.Fatalf("WritePack(%s): %v", sha, err)
		}
		for _, p := range []string{packPath, idxPath} {
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			f.Files[root+"/packs/"+filepath.Base(p)] = b
		}
		stems = append(stems, strings.TrimSuffix(filepath.Base(packPath), ".pack"))
	}
	if _, err := WriteRef(f, root, "refs/heads/main", shas[len(shas)-1], false); err != nil {
		t.Fatal(err)
	}
	return stems
}

// countPackDownloads parses trace output for downloads under <root>/packs/,
// returning per-extension counts and the downloaded names. This helper IS
// the measurement the gate uses; TestCountPackDownloads pins it.
func countPackDownloads(trace, root string) (packs, idxs int, names []string) {
	for _, line := range strings.Split(trace, "\n") {
		rest, ok := strings.CutPrefix(line, "gpb: downloaded ")
		if !ok {
			continue
		}
		p := strings.SplitN(rest, " (", 2)[0]
		if !strings.HasPrefix(p, root+"/packs/") {
			continue
		}
		name := strings.TrimPrefix(p, root+"/packs/")
		names = append(names, name)
		if strings.HasSuffix(name, ".pack") {
			packs++
		}
		if strings.HasSuffix(name, ".idx") {
			idxs++
		}
	}
	return
}

// GUARD on the measurement itself: the counter must distinguish, or every
// selectivity assertion in this suite and the live gate is theater.
func TestCountPackDownloads(t *testing.T) {
	trace := "gpb: downloaded /r/packs/pack-a.pack (5 bytes)\n" +
		"gpb: downloaded /r/packs/pack-a.idx (3 bytes)\n" +
		"gpb: downloaded /r/refs/heads/main (41 bytes)\n" + // ref reads excluded
		"noise\n"
	p, i, names := countPackDownloads(trace, "/r")
	if p != 1 || i != 1 || len(names) != 2 {
		t.Fatalf("p=%d i=%d names=%v", p, i, names)
	}
}

// GUARD, not RED — and the label is load-bearing: this test also passes
// against 3a's download-everything code (downloading every pack converges
// too), so it pins CONVERGENCE across a pack split, not selectivity. The
// loop's distinguishing observable is the trace-counted selectivity test
// below, whose deliberate-regression check (Step 6) is the proof it can
// fail. Convergence still needs its own pin: a discovery bug that fetches
// too little fails HERE first, with the clearest diagnosis.
func TestFetchDiscoversAcrossPacks(t *testing.T) {
	src := newGitRepoForPush(t)
	c1 := headOfPushRepo(t, src)
	c2 := commitOnPushRepo(t, src, "f2.txt", "two")
	f := transport.NewFake()
	plantIncrementalPacks(t, f, "/r", src, []string{c1, c2})

	dst := emptyGitRepo(t)
	keep, err := Fetch(f, "/r", dst, "", []string{c2})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if keep == "" {
		t.Fatal("an installing fetch must return a .keep")
	}
	for _, sha := range []string{c1, c2} {
		if !gitcmd.HasObject(dst, sha) {
			t.Errorf("object %s missing after fetch", sha)
		}
	}
}

// RED: selectivity, measured. dst already holds c1..c2 (via a first fetch);
// after c3 is pushed, the incremental fetch downloads EXACTLY the new pair.
// The full-fetch count beside it is the deliberate-regression twin: the
// measurement demonstrably registers every download, so had the incremental
// fetch over-downloaded, the ==1 assertion would have caught it.
func TestFetchDownloadsOnlyTheNeededPack(t *testing.T) {
	src := newGitRepoForPush(t)
	c1 := headOfPushRepo(t, src)
	c2 := commitOnPushRepo(t, src, "f2.txt", "two")
	f := transport.NewFake()
	stems := plantIncrementalPacks(t, f, "/r", src, []string{c1, c2})

	var trace strings.Builder
	tr := transport.NewTraced(f, &trace)
	dst := emptyGitRepo(t)
	cache := t.TempDir()
	if _, err := Fetch(tr, "/r", dst, cache, []string{c2}); err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	fullPacks, fullIdxs, _ := countPackDownloads(trace.String(), "/r")
	if fullPacks != 2 || fullIdxs != 2 {
		t.Fatalf("full fetch must download both pairs (twin measurement): packs=%d idxs=%d",
			fullPacks, fullIdxs)
	}

	// git updates refs AFTER the helper exits; simulate before the increment.
	run := func(args ...string) {
		if out, err := exec.Command("git", append([]string{"-C", dst}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("update-ref", "refs/heads/main", c2)

	c3 := commitOnPushRepo(t, src, "f3.txt", "three")
	newStems := plantOneMorePack(t, f, "/r", src, c2, c3)
	trace.Reset()
	if _, err := Fetch(tr, "/r", dst, cache, []string{c3}); err != nil {
		t.Fatalf("incremental fetch: %v", err)
	}
	packs, idxs, names := countPackDownloads(trace.String(), "/r")
	if packs != 1 || idxs != 1 {
		t.Errorf("incremental fetch must download exactly one pair, got packs=%d idxs=%d (%v)",
			packs, idxs, names)
	}
	for _, n := range names {
		if !strings.HasPrefix(n, newStems[0]) {
			t.Errorf("downloaded %s; only %s.* was needed", n, newStems[0])
		}
	}
	_ = stems
}

// plantOneMorePack adds ONE more incremental pack (tip..prev) to an
// already-planted Fake and moves the ref. Separate from plantIncrementalPacks
// because Bootstrap and the first WriteRef must not rerun.
func plantOneMorePack(t *testing.T, f *transport.Fake, root, src, prev, tip string) []string {
	t.Helper()
	packPath, idxPath, err := gitcmd.WritePack(src, []string{tip}, []string{prev}, t.TempDir())
	if err != nil || packPath == "" {
		t.Fatalf("WritePack: %v", err)
	}
	for _, p := range []string{packPath, idxPath} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f.Files[root+"/packs/"+filepath.Base(p)] = b
	}
	if _, err := WriteRef(f, root, "refs/heads/main", tip, true); err != nil {
		t.Fatal(err)
	}
	return []string{strings.TrimSuffix(filepath.Base(packPath), ".pack")}
}

// GUARD, not RED, same reasoning as TestFetchDiscoversAcrossPacks: probe 3
// end-to-end — a frontier that deepens through a missing tree converges
// (commit pack, then tree pack, then blob pack). Download-everything also
// passes this; the pin is that multi-round discovery CONVERGES, which is
// exactly what breaks if the loop's termination or restart logic is wrong.
func TestFetchFrontierDeepensThroughHandBuiltPacks(t *testing.T) {
	src := newGitRepoForPush(t)
	commitOnPushRepo(t, src, "deep.txt", "payload")
	sha := headOfPushRepo(t, src)
	out := func(args ...string) string {
		b, err := exec.Command("git", append([]string{"-C", src}, args...)...).Output()
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return strings.TrimSpace(string(b))
	}
	tree := out("rev-parse", sha+"^{tree}")
	blobList := out("rev-list", "--objects", sha)
	f := transport.NewFake()
	f.Dirs["/r"] = true
	if err := Bootstrap(f, "/r"); err != nil {
		t.Fatal(err)
	}
	// One hand-built pack per object CLASS: commit alone, tree(s) alone,
	// blob(s) alone — forcing one discovery round per depth level.
	classes := [][]string{{sha}, nil, nil}
	for _, line := range strings.Split(blobList, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] == sha {
			continue
		}
		if fields[0] == tree || strings.Contains(line, "/") || len(fields) == 1 {
			classes[1] = append(classes[1], fields[0]) // trees (root tree has no path)
		} else {
			classes[2] = append(classes[2], fields[0]) // pathed blobs
		}
	}
	for _, objs := range classes {
		if len(objs) == 0 {
			continue
		}
		dir := t.TempDir()
		name, err := gitcmd.PackObjectsFromList(src, "", strings.Join(objs, "\n"),
			filepath.Join(dir, "pack"))
		if err != nil {
			t.Fatalf("PackObjectsFromList: %v", err)
		}
		for _, ext := range []string{".pack", ".idx"} {
			b, err := os.ReadFile(filepath.Join(dir, "pack-"+name+ext))
			if err != nil {
				t.Fatal(err)
			}
			f.Files["/r/packs/pack-"+name+ext] = b
		}
	}
	if _, err := WriteRef(f, "/r", "refs/heads/main", sha, false); err != nil {
		t.Fatal(err)
	}
	dst := emptyGitRepo(t)
	if _, err := Fetch(f, "/r", dst, "", []string{sha}); err != nil {
		t.Fatalf("Fetch across a deepening frontier: %v", err)
	}
	if !gitcmd.HasObject(dst, sha) {
		t.Error("commit missing after multi-round discovery")
	}
	if out, err := exec.Command("git", "-C", dst, "fsck", "--no-dangling").CombinedOutput(); err != nil {
		t.Errorf("fsck after multi-round fetch: %v: %s", err, out)
	}
}

// RED (fix round 2, I2 revised). validateObjectsPackPath does not exist yet —
// it replaces fix round 1's validateGitDirOutput now that consolidateAndInstall
// asks `--git-path objects/pack` instead of `--git-dir` (see the linked-
// worktree test above for why). RevParse's result is still git()'s COMBINED
// stdout+stderr (trimmed), so the same untrusted-output hazard applies: a
// single git warning merged in alongside a genuine answer must never be
// trusted as a filesystem path outright.
//
// Unlike the old --git-dir check, there is no "confirm it already exists as
// a directory" fallback for a relative answer here: this function's whole
// caller exists to MkdirAll objects/pack, which — on a repo that has never
// held a pack — does not exist yet by definition. The shape check instead
// requires the value to actually look like an objects/pack answer: either
// the literal "objects/pack" (a bare repo, whose git dir IS its own root) or
// a path ending in "/objects/pack" (every other layout).
func TestValidateObjectsPackPathRejectsUntrustedCombinedOutput(t *testing.T) {
	cases := []struct {
		name    string
		v       string
		wantErr bool
	}{
		{"bare-repo shape", "objects/pack", false},
		{"ordinary-repo shape", ".git/objects/pack", false},
		{"worktree absolute shape", "C:/somewhere/main/.git/objects/pack", false},
		{"a warning merged in before a real answer", "warning: something\nobjects/pack", true},
		{"a warning merged in after a real answer", "objects/pack\nwarning: something", true},
		{"does not look like an objects/pack answer at all", "not-a-real-path", true},
		{"close but wrong — trailing s", "objects/packs", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateObjectsPackPath(c.v)
			if c.wantErr && err == nil {
				t.Errorf("validateObjectsPackPath(%q) = nil, want an error", c.v)
			}
			if !c.wantErr && err != nil {
				t.Errorf("validateObjectsPackPath(%q) = %v, want nil", c.v, err)
			}
		})
	}
}

// countInstalledPackFiles reports how many .pack files exist under gitDir's
// OWN .git/objects/pack — the real, installed object store, as opposed to
// countPackFiles above, which counts what a Fake holds under a remote root's
// packs/. Named distinctly because both exist in this file for different
// purposes: this one proves what Fetch actually installed locally.
func countInstalledPackFiles(t *testing.T, gitDir string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(gitDir, ".git", "objects", "pack"))
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
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

func TestResolveIdxCacheDirCreatesUnderCommonDir(t *testing.T) {
	d := emptyGitRepo(t)
	dir, err := ResolveIdxCacheDir(d, "/my-files/GitRemotes/demo")
	if err != nil {
		t.Fatalf("ResolveIdxCacheDir: %v", err)
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("cache dir must be absolute, got %s", dir)
	}
	// Under <repo>/.git/proton-v2/idx-cache/<key>: the common dir resolved
	// through git, not assumed.
	wantPrefix := filepath.Join(d, ".git", "proton-v2", "idx-cache")
	if !strings.HasPrefix(dir, wantPrefix) {
		t.Errorf("cache dir %s not under %s", dir, wantPrefix)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Errorf("cache dir must exist as a directory: %v", err)
	}
	// The breadcrumb records the plain remote path for humans.
	b, err := os.ReadFile(filepath.Join(dir, "remote"))
	if err != nil || !strings.Contains(string(b), "/my-files/GitRemotes/demo") {
		t.Errorf("breadcrumb missing or wrong: %q, %v", b, err)
	}
	// Two different remotes must get two different keys.
	dir2, err := ResolveIdxCacheDir(d, "/my-files/GitRemotes/other")
	if err != nil {
		t.Fatal(err)
	}
	if dir2 == dir {
		t.Error("distinct remote roots must map to distinct cache dirs")
	}
}

func TestEnsureSidecarDownloadsOnceThenHits(t *testing.T) {
	f := transport.NewFake()
	f.Files["/r/packs/pack-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.idx"] = []byte("idx-bytes")
	cache, fallback := t.TempDir(), t.TempDir()
	stem := "pack-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	p1, cached, err := EnsureSidecar(f, "/r", cache, fallback, stem)
	if err != nil || cached {
		t.Fatalf("first call: path=%s cached=%v err=%v; want fresh download", p1, cached, err)
	}
	b, err := os.ReadFile(p1)
	if err != nil || string(b) != "idx-bytes" {
		t.Fatalf("returned path must hold the sidecar bytes: %q %v", b, err)
	}
	// The cache must now hold a copy under the final name.
	if _, err := os.Stat(filepath.Join(cache, stem+".idx")); err != nil {
		t.Fatalf("cache install missing: %v", err)
	}
	// Second call: a hit, no download. Delete the remote copy to prove it.
	delete(f.Files, "/r/packs/"+stem+".idx")
	p2, cached, err := EnsureSidecar(f, "/r", cache, fallback, stem)
	if err != nil || !cached {
		t.Fatalf("second call must hit the cache: cached=%v err=%v", cached, err)
	}
	if b, err := os.ReadFile(p2); err != nil || string(b) != "idx-bytes" {
		t.Fatalf("cache hit returned wrong bytes: %q %v", b, err)
	}
}

// Cache trouble must never become fetch trouble: with an unusable cacheDir
// (a PATH THAT IS A FILE, cross-platform-reliably unusable), the sidecar
// still arrives via fallbackDir.
func TestEnsureSidecarDegradesWhenCacheUnusable(t *testing.T) {
	f := transport.NewFake()
	stem := "pack-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	f.Files["/r/packs/"+stem+".idx"] = []byte("idx2")
	notADir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, cached, err := EnsureSidecar(f, "/r", notADir, t.TempDir(), stem)
	if err != nil || cached {
		t.Fatalf("degraded call: %v", err)
	}
	if b, _ := os.ReadFile(p); string(b) != "idx2" {
		t.Errorf("fallback copy wrong: %q", b)
	}
}

// RefreshSidecar must return FRESH bytes even when a stale cached copy and a
// stale fallback copy both exist (the residue rule applies to sidecars too:
// ReadTo's overwrite behaviour is unpinned, so stale files are deleted first).
func TestRefreshSidecarReplacesStaleCopies(t *testing.T) {
	f := transport.NewFake()
	stem := "pack-cccccccccccccccccccccccccccccccccccccccc"
	f.Files["/r/packs/"+stem+".idx"] = []byte("fresh")
	cache, fallback := t.TempDir(), t.TempDir()
	for _, d := range []string{cache, fallback} {
		if err := os.WriteFile(filepath.Join(d, stem+".idx"), []byte("stale"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := RefreshSidecar(f, "/r", cache, fallback, stem)
	if err != nil {
		t.Fatalf("RefreshSidecar: %v", err)
	}
	if b, _ := os.ReadFile(p); string(b) != "fresh" {
		t.Errorf("refresh returned %q, want fresh bytes", b)
	}
	if b, _ := os.ReadFile(filepath.Join(cache, stem+".idx")); string(b) != "fresh" {
		t.Errorf("cache still holds %q after refresh", b)
	}
}

func TestPruneStaleRemovesOnlyVanishedStems(t *testing.T) {
	cache := t.TempDir()
	keepStem := "pack-dddddddddddddddddddddddddddddddddddddddd"
	goneStem := "pack-eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	for _, n := range []string{keepStem + ".idx", goneStem + ".idx", ".tmp-123", "remote"} {
		if err := os.WriteFile(filepath.Join(cache, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pruneStale(cache, map[string]bool{keepStem: true})
	if _, err := os.Stat(filepath.Join(cache, keepStem+".idx")); err != nil {
		t.Error("kept stem was pruned")
	}
	if _, err := os.Stat(filepath.Join(cache, goneStem+".idx")); !os.IsNotExist(err) {
		t.Error("vanished stem survived pruning")
	}
	if _, err := os.Stat(filepath.Join(cache, ".tmp-123")); !os.IsNotExist(err) {
		t.Error("staging leftover survived pruning")
	}
	if _, err := os.Stat(filepath.Join(cache, "remote")); err != nil {
		t.Error("the breadcrumb must never be pruned")
	}
}

// RED: only complete, grammar-valid pairs survive the listing. Names are
// remote-controlled input; nothing else may ever reach a filesystem join.
func TestListCompletePacksFiltersGrammarAndPairs(t *testing.T) {
	f := transport.NewFake()
	good := "pack-" + strings.Repeat("a", 40)
	orphanPack := "pack-" + strings.Repeat("b", 40)
	orphanIdx := "pack-" + strings.Repeat("c", 40)
	f.Files["/r/packs/"+good+".pack"] = []byte("p")
	f.Files["/r/packs/"+good+".idx"] = []byte("i")
	f.Files["/r/packs/"+orphanPack+".pack"] = []byte("p") // no idx: in-flight push, skip
	f.Files["/r/packs/"+orphanIdx+".idx"] = []byte("i")   // no pack: unrepairable, skip
	f.Files["/r/packs/pack-NOTHEX.pack"] = []byte("x")    // grammar violation
	f.Files["/r/packs/stray.txt"] = []byte("x")           // stray node
	f.Dirs["/r/packs/subdir"] = true                      // directory

	stems, err := listCompletePacks(f, "/r")
	if err != nil {
		t.Fatalf("listCompletePacks: %v", err)
	}
	if len(stems) != 1 || stems[0] != good {
		t.Errorf("want exactly [%s], got %v", good, stems)
	}
}

// RED: the map is built by git's own reader over cached sidecars.
func TestBuildPackMapMapsOidsToStems(t *testing.T) {
	src := newGitRepoForPush(t)
	sha := headOfPushRepo(t, src)
	packPath, idxPath, err := gitcmd.WritePack(src, []string{sha}, nil, t.TempDir())
	if err != nil || packPath == "" {
		t.Fatalf("WritePack: %v", err)
	}
	stem := strings.TrimSuffix(filepath.Base(packPath), ".pack")
	f := transport.NewFake()
	for _, p := range []string{packPath, idxPath} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f.Files["/r/packs/"+filepath.Base(p)] = b
	}
	pm, err := buildPackMap(f, "/r", "", t.TempDir(), []string{stem})
	if err != nil {
		t.Fatalf("buildPackMap: %v", err)
	}
	packs := pm.oidPacks[sha]
	if len(packs) != 1 || packs[0] != stem {
		t.Errorf("commit %s must map to [%s], got %v", sha, stem, packs)
	}
}

// RED: a structurally corrupt CACHED sidecar self-heals as a cache miss; the
// map is built from the re-downloaded truth.
func TestBuildPackMapHealsACorruptCachedSidecar(t *testing.T) {
	src := newGitRepoForPush(t)
	sha := headOfPushRepo(t, src)
	packPath, idxPath, err := gitcmd.WritePack(src, []string{sha}, nil, t.TempDir())
	if err != nil || packPath == "" {
		t.Fatalf("WritePack: %v", err)
	}
	stem := strings.TrimSuffix(filepath.Base(packPath), ".pack")
	f := transport.NewFake()
	for _, p := range []string{packPath, idxPath} {
		b, _ := os.ReadFile(p)
		f.Files["/r/packs/"+filepath.Base(p)] = b
	}
	cache := t.TempDir()
	if err := os.WriteFile(filepath.Join(cache, stem+".idx"), []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	pm, err := buildPackMap(f, "/r", cache, t.TempDir(), []string{stem})
	if err != nil {
		t.Fatalf("buildPackMap must heal the corrupt cached sidecar: %v", err)
	}
	if len(pm.oidPacks[sha]) != 1 {
		t.Errorf("healed map must contain the commit")
	}
	// And the cache must now hold the good bytes.
	want, _ := os.ReadFile(idxPath)
	got, _ := os.ReadFile(filepath.Join(cache, stem+".idx"))
	if !bytes.Equal(want, got) {
		t.Error("cache must be repaired with the fresh sidecar")
	}
}

// RED: a corrupt sidecar FRESH from the remote is fatal naming the file.
func TestBuildPackMapFatalOnCorruptRemoteSidecar(t *testing.T) {
	stem := "pack-" + strings.Repeat("d", 40)
	f := transport.NewFake()
	f.Files["/r/packs/"+stem+".pack"] = []byte("p")
	f.Files["/r/packs/"+stem+".idx"] = []byte("not an index")
	_, err := buildPackMap(f, "/r", "", t.TempDir(), []string{stem})
	if err == nil || !strings.Contains(err.Error(), stem) {
		t.Fatalf("want a fatal naming %s, got %v", stem, err)
	}
}

// RED: greedy cover. Forced singles first, then most-covering, ties
// lexicographic; never a pack contributing nothing.
func TestGreedyCover(t *testing.T) {
	oidX, oidY, oidZ := strings.Repeat("1", 40), strings.Repeat("2", 40), strings.Repeat("3", 40)
	// x in {A,B}, y in {B}: B alone suffices (the round-1 review counterexample).
	m := map[string][]string{
		oidX: {"pack-A", "pack-B"},
		oidY: {"pack-B"},
	}
	got, err := greedyCover([]string{oidX, oidY}, m, map[string]bool{})
	if err != nil {
		t.Fatalf("greedyCover: %v", err)
	}
	if len(got) != 1 || got[0] != "pack-B" {
		t.Errorf("want [pack-B], got %v", got)
	}
	// Tie on coverage: lexicographically first wins, deterministically.
	m2 := map[string][]string{oidZ: {"pack-Q", "pack-P"}}
	got2, err := greedyCover([]string{oidZ}, m2, map[string]bool{})
	if err != nil || len(got2) != 1 || got2[0] != "pack-P" {
		t.Errorf("tie must break lexicographically: %v %v", got2, err)
	}
	// No candidate at all: errCacheSuspect, naming the OID.
	_, err = greedyCover([]string{oidZ}, map[string][]string{}, map[string]bool{})
	if err == nil || !errors.Is(err, errCacheSuspect) || !strings.Contains(err.Error(), oidZ) {
		t.Errorf("no-candidate must wrap errCacheSuspect and name the OID: %v", err)
	}
	// Still missing though its only pack was downloaded: the no-progress
	// signature, also errCacheSuspect. (This is the unit-level home of the
	// no-progress rule; end-to-end it is unreachable with self-consistent
	// remote pairs, since oid-in-idx implies oid-in-pack.)
	_, err = greedyCover([]string{oidZ}, map[string][]string{oidZ: {"pack-A"}},
		map[string]bool{"pack-A": true})
	if err == nil || !errors.Is(err, errCacheSuspect) {
		t.Errorf("no-progress must wrap errCacheSuspect: %v", err)
	}
	// A failure names EVERY offender, sorted — rev-list order is unspecified,
	// so a first-offender error would name a nondeterministic OID (and Task
	// 7's fatal-message assertions depend on this determinism).
	_, err = greedyCover([]string{oidY, oidX}, map[string][]string{}, map[string]bool{})
	if err == nil || !strings.Contains(err.Error(), oidX) || !strings.Contains(err.Error(), oidY) {
		t.Errorf("a no-candidate error must name all offenders: %v", err)
	}
}

// GUARD, not RED: passed immediately against Task 6's code. A parseable-but-
// LYING cached sidecar (valid idx bytes filed under the wrong stem) misroutes
// discovery; the self-heal round must fix it and the fetch must complete.
// This is the spec's "correctness never depends on the cache being right"
// promise. Deliberate-regression check: with heal() stubbed to a no-op, this
// failed with "no pack on the remote contains missing object(s) <c2-sha>:
// ... this can be caused by a stale or corrupt sidecar cache (the sidecar
// metadata was already refreshed ...)" — confirming the assertion is live.
func TestFetchSelfHealsALyingCache(t *testing.T) {
	src := newGitRepoForPush(t)
	c1 := headOfPushRepo(t, src)
	c2 := commitOnPushRepo(t, src, "f2.txt", "two")
	f := transport.NewFake()
	stems := plantIncrementalPacks(t, f, "/r", src, []string{c1, c2})

	// Poison: both cache entries hold pack-1's idx bytes, so c2's objects
	// appear to live nowhere.
	cache := t.TempDir()
	idx1 := f.Files["/r/packs/"+stems[0]+".idx"]
	for _, stem := range stems {
		if err := os.WriteFile(filepath.Join(cache, stem+".idx"), idx1, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dst := emptyGitRepo(t)
	if _, err := Fetch(f, "/r", dst, cache, []string{c2}); err != nil {
		t.Fatalf("Fetch must self-heal a lying cache: %v", err)
	}
	if !gitcmd.HasObject(dst, c2) {
		t.Error("objects missing after healed fetch")
	}
}

// GUARD, not RED: passed immediately against Task 6's code. After the heal,
// the same diagnosis is genuine: an object the remote truly does not hold is
// fatal, names the OID, and says the cache was already refreshed (so the
// message never advises cache-clearing). Deliberate-regression check: with
// fatalAfterHeal's wrap message text swapped out for wording that omits
// "already refreshed", this failed on "fatal must state the metadata was
// already refreshed" — confirming the assertion is live.
func TestFetchFatalAfterHealNamesTheOidAndTheRefresh(t *testing.T) {
	src := newGitRepoForPush(t)
	c1 := headOfPushRepo(t, src)
	c2 := commitOnPushRepo(t, src, "f2.txt", "two")
	f := transport.NewFake()
	stems := plantIncrementalPacks(t, f, "/r", src, []string{c1, c2})
	// Remove pack 1 entirely: c1's closure is genuinely gone from the remote.
	delete(f.Files, "/r/packs/"+stems[0]+".pack")
	delete(f.Files, "/r/packs/"+stems[0]+".idx")

	dst := emptyGitRepo(t)
	_, err := Fetch(f, "/r", dst, t.TempDir(), []string{c2})
	if err == nil {
		t.Fatal("a remote missing part of the closure must be fatal")
	}
	if !strings.Contains(err.Error(), c1) {
		t.Errorf("fatal must name the missing OID %s: %v", c1, err)
	}
	if !strings.Contains(err.Error(), "already refreshed") {
		t.Errorf("fatal must state the metadata was already refreshed: %v", err)
	}
	// Nothing installed: the 3a posture holds.
	if n := countInstalledPackFiles(t, dst); n != 0 {
		t.Errorf("failed fetch must leave the local store untouched; %d packs installed", n)
	}
}

// residueTransport fails the FIRST download of target, leaving partial bytes
// at the destination — then, on the retry, REFUSES if those bytes are still
// there. This pins downloadAndVerifyPack's entry-point removal directly:
// retry correctness must not depend on ReadTo's unpinned overwrite behaviour
// onto an existing file.
type residueTransport struct {
	*transport.Fake
	target   string
	tripped  bool
	sawStale bool
}

func (r *residueTransport) ReadTo(p, local string) error {
	if p == r.target {
		dest := filepath.Join(local, path.Base(p))
		if !r.tripped {
			r.tripped = true
			_ = os.WriteFile(dest, []byte("partial residue"), 0o644)
			return fmt.Errorf("injected transient failure downloading %s", p)
		}
		if _, err := os.Stat(dest); err == nil {
			r.sawStale = true
			return fmt.Errorf("retry found residue at %s; the residue rule is violated", dest)
		}
	}
	return r.Fake.ReadTo(p, local)
}

// GUARD, not RED: passed immediately against Task 6's code. UPDATED for the
// quarantine refactor: downloadAndVerifyPack no longer removes anything on a
// failure path (the old residue rule is deleted — see fetch.go); the ONE
// removal left is the unconditional one at function entry, before ReadTo,
// and it is now the SOLE thing standing between a healed plan's retry and
// stale bytes from the attempt this test injects. Without it, the "partial
// residue" this test's first ReadTo call writes to incomingDir would still
// be sitting there when the healed round retries the same stem, and
// residueTransport's second ReadTo call would see it and refuse.
//
// Deliberate-regression check: with the entry-point removal disabled
// (wrapped in `if false {}`), this failed on
// "Fetch must survive one transient pack-download failure via the heal round
// (sawStale=true): cannot download pack-....pack: retry found residue at
// ...\incoming\pack-....pack; the residue rule is violated ... (the sidecar
// metadata was already refreshed from the remote this run; this indicates
// genuine remote or transport trouble)" — the err != nil check (repo_test.go,
// this function's first assertion) fires before the sawStale check is ever
// reached, because the second failure lands after the one heal round is
// already spent and Fetch fatals. Confirming the entry-point removal is
// live and load-bearing. Reverted after confirming.
func TestFetchRetriesSamePackWithoutResidue(t *testing.T) {
	src := newGitRepoForPush(t)
	c1 := headOfPushRepo(t, src)
	f := transport.NewFake()
	stems := plantIncrementalPacks(t, f, "/r", src, []string{c1})
	rt := &residueTransport{Fake: f, target: "/r/packs/" + stems[0] + ".pack"}

	dst := emptyGitRepo(t)
	if _, err := Fetch(rt, "/r", dst, "", []string{c1}); err != nil {
		t.Fatalf("Fetch must survive one transient pack-download failure via the heal "+
			"round (sawStale=%v): %v", rt.sawStale, err)
	}
	if rt.sawStale {
		t.Error("the retry found the failed attempt's residue; it must have been deleted")
	}
	if !gitcmd.HasObject(dst, c1) {
		t.Error("objects missing after retried fetch")
	}
}

// GUARD, not RED: passed immediately against Task 6's code. A corrupt remote
// PACK selected because of a lying cache: checksum fails, heal reroutes to
// the honest pack, fetch completes WITHOUT the corrupt one. Deliberate-
// regression check: with the checksum-mismatch error no longer wrapping
// errCacheSuspect (making it non-heal-able), this failed with "Fetch must
// heal past the checksum failure: downloaded pack ... recomputes to ...; the
// name is the content checksum, so this file is not what its name claims" —
// confirming the assertion is live.
func TestFetchChecksumFailureHealsAndRoutesAround(t *testing.T) {
	src := newGitRepoForPush(t)
	c1 := headOfPushRepo(t, src)
	c2 := commitOnPushRepo(t, src, "f2.txt", "two")
	c3 := commitOnPushRepo(t, src, "f3.txt", "three")
	f := transport.NewFake()
	stems := plantIncrementalPacks(t, f, "/r", src, []string{c1, c2, c3})

	// dst already holds c1..c2: fetch once with the ref at c2, then advance.
	dst := emptyGitRepo(t)
	if _, err := Fetch(f, "/r", dst, "", []string{c2}); err != nil {
		t.Fatalf("staging fetch: %v", err)
	}
	if out, err := exec.Command("git", "-C", dst, "update-ref", "refs/heads/main", c2).CombinedOutput(); err != nil {
		t.Fatalf("update-ref: %v: %s", err, out)
	}

	// Corrupt pack 2's remote bytes (now unneeded by dst), and poison the
	// cache so c3's missing objects appear to live in pack 2: cache holds
	// pack-3's idx bytes under pack-2's stem, and pack-2's under pack-3's.
	cache := t.TempDir()
	idx2 := f.Files["/r/packs/"+stems[1]+".idx"]
	idx3 := f.Files["/r/packs/"+stems[2]+".idx"]
	if err := os.WriteFile(filepath.Join(cache, stems[1]+".idx"), idx3, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, stems[2]+".idx"), idx2, 0o644); err != nil {
		t.Fatal(err)
	}
	pack2 := f.Files["/r/packs/"+stems[1]+".pack"]
	corrupt := append([]byte{}, pack2...)
	corrupt[len(corrupt)/2] ^= 0xff
	f.Files["/r/packs/"+stems[1]+".pack"] = corrupt

	var trace strings.Builder
	tr := transport.NewTraced(f, &trace)
	if _, err := Fetch(tr, "/r", dst, cache, []string{c3}); err != nil {
		t.Fatalf("Fetch must heal past the checksum failure: %v", err)
	}
	if !gitcmd.HasObject(dst, c3) {
		t.Error("c3 missing after healed fetch")
	}
	// GUARD (M1, Stage 4): the route-around must be visible in the transfer
	// trace — the poisoned pack downloads exactly once (its failed attempt),
	// never again after the heal, and the covering pack exactly once.
	badStem, coverStem := stems[1], stems[2]
	if got := strings.Count(trace.String(), "/packs/"+badStem+".pack"); got != 1 {
		t.Errorf("poisoned pack downloaded %d times, want exactly 1; trace:\n%s", got, trace.String())
	}
	if got := strings.Count(trace.String(), "/packs/"+coverStem+".pack"); got != 1 {
		t.Errorf("covering pack downloaded %d times, want exactly 1; trace:\n%s", got, trace.String())
	}
}

// altPackingIdx builds a SECOND packing of sha's full closure at store-only
// compression: same OIDs, different bytes and name. Its idx is the perfect
// lie — a map built from it is RIGHT about which objects the stem serves,
// so greedy still selects the pack and the failure surfaces at PAIR
// verification, not earlier as a no-candidate heal. (A sidecar with the
// WRONG oids would divert into the heal path before any pack downloads —
// the trap this fixture exists to avoid.)
func altPackingIdx(t *testing.T, src, sha, realStem string) []byte {
	t.Helper()
	objs, err := exec.Command("git", "-C", src, "rev-list", "--objects", sha).Output()
	if err != nil {
		t.Fatalf("rev-list: %v", err)
	}
	dir := t.TempDir()
	cmd := exec.Command("git", "-C", src, "-c", "pack.compression=0",
		"-c", "pack.packSizeLimit=0",
		"pack-objects", "--no-thin", "--index-version=2", "-q", filepath.Join(dir, "pack"))
	cmd.Stdin = bytes.NewReader(objs)
	nameOut, err := cmd.Output()
	if err != nil {
		t.Fatalf("alt pack-objects: %v", err)
	}
	name := "pack-" + strings.TrimSpace(string(nameOut))
	if name == realStem {
		t.Fatal("fixture: the two packings coincide; the lie would be the truth")
	}
	b, err := os.ReadFile(filepath.Join(dir, name+".idx"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// GUARD, not RED: passed immediately against Task 6's code. Pair
// verification with a lying-but-valid CACHED sidecar whose OID set is right
// (see altPackingIdx): pack downloads fine, checksum passes, index-pack
// --verify fails; ONE sidecar re-download fixes it and the fetch completes —
// no heal round consumed. Deliberate-regression check: forcing the second
// (post-refresh) verify attempt to fail unconditionally (overwriting the
// freshly copied sidecar with garbage right before the re-verify) turned the
// recovery into the same "pair is corrupt, member undetermined" fatal that
// TestFetchGenuinelyCorruptPairIsFatalMemberUndetermined pins, and this test
// failed on "Fetch must recover from a lying sidecar via pair-verify
// refresh: ... member undetermined" — confirming a genuinely-recoverable
// case actually depends on the refresh succeeding, not merely on always
// reaching the fatal branch.
func TestFetchPairFailureRefreshesSidecarAndCompletes(t *testing.T) {
	src := newGitRepoForPush(t)
	c1 := headOfPushRepo(t, src)
	f := transport.NewFake()
	stems := plantIncrementalPacks(t, f, "/r", src, []string{c1})
	cache := t.TempDir()
	if err := os.WriteFile(filepath.Join(cache, stems[0]+".idx"),
		altPackingIdx(t, src, c1, stems[0]), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := emptyGitRepo(t)
	if _, err := Fetch(f, "/r", dst, cache, []string{c1}); err != nil {
		t.Fatalf("Fetch must recover from a lying sidecar via pair-verify refresh: %v", err)
	}
	if !gitcmd.HasObject(dst, c1) {
		t.Errorf("%s missing", c1)
	}
	// The cache must have been repaired with the remote's true sidecar.
	got, err := os.ReadFile(filepath.Join(cache, stems[0]+".idx"))
	if err != nil || !bytes.Equal(got, f.Files["/r/packs/"+stems[0]+".idx"]) {
		t.Error("cache must hold the true sidecar after the refresh")
	}
}

// TestFetchPairFailureWithTwoPacksRefreshesAndCompletes: two pushes plant two
// packs (stem1 = c1's own pack, stem2 = c1->c2's delta) both needed by the
// want (dst starts empty). stem1's CACHED sidecar is corrupted with GARBAGE
// BYTES — a bit flip on its own real content, not altPackingIdx's same-OID-
// set alternate packing. Confirmed (Stage 4 fix round 1) that git's own
// show-index rejects bit-flipped bytes, so this is caught by buildPackMap's
// PRE-LOOP show-index cache-miss check (packmap.go:98-101), which heals the
// sidecar BEFORE any round is planned — NOT downloadAndVerifyPack's mid-loop
// pair-refresh or fetch.go's `if refreshed { break }` round-restart
// (fetch.go:160-166), which this fixture never reaches. It is still novel
// coverage: the first end-to-end two-pack proof that a corrupt cached
// sidecar yields a complete, bounded fetch. See
// TestFetchMidRoundPairRefreshWithTwoPacksCompletes below for the mid-round
// path this test does NOT exercise.
func TestFetchPairFailureWithTwoPacksRefreshesAndCompletes(t *testing.T) {
	src := newGitRepoForPush(t)
	c1 := headOfPushRepo(t, src)
	c2 := commitOnPushRepo(t, src, "f2.txt", "two")
	f := transport.NewFake()
	stems := plantIncrementalPacks(t, f, "/r", src, []string{c1, c2})
	stem1, stem2 := stems[0], stems[1]

	cache := t.TempDir()
	idx1 := f.Files["/r/packs/"+stem1+".idx"]
	corrupt := append([]byte{}, idx1...)
	corrupt[len(corrupt)/2] ^= 0xff
	if err := os.WriteFile(filepath.Join(cache, stem1+".idx"), corrupt, 0o644); err != nil {
		t.Fatal(err)
	}

	dst := emptyGitRepo(t)
	var trace strings.Builder
	tr := transport.NewTraced(f, &trace)
	_, err := Fetch(tr, "/r", dst, cache, []string{c2})
	// GUARD (retro-Codex 1, Stage 4): a corrupt cached sidecar, healed by
	// buildPackMap's PRE-LOOP show-index cache-miss check, must still yield a
	// complete two-pack fetch with bounded downloads. HONEST SCOPE: this pins
	// the pre-loop heal path only — it does NOT reach downloadAndVerifyPack's
	// mid-loop pair-refresh or fetch.go's round-restart, so it does not
	// exercise (and cannot stand in for) a genuine mid-round refresh with a
	// second pack still pending; see
	// TestFetchMidRoundPairRefreshWithTwoPacksCompletes for that.
	if err != nil {
		t.Fatalf("fetch must survive a pre-loop sidecar heal with two packs: %v", err)
	}
	if !gitcmd.HasObject(dst, c2) {
		t.Errorf("want %s missing after two-pack refresh fetch", c2)
	}
	if got := strings.Count(trace.String(), "/packs/"+stem1+".idx"); got < 1 {
		t.Errorf("the corrupted sidecar was never re-downloaded; trace:\n%s", trace.String())
	}
	for _, stem := range []string{stem1, stem2} {
		if got := strings.Count(trace.String(), "/packs/"+stem+".pack"); got < 1 || got > 2 {
			t.Errorf("pack %s downloaded %d times, want 1 or 2 (bounded); trace:\n%s",
				stem, got, trace.String())
		}
	}
}

// TestFetchMidRoundPairRefreshWithTwoPacksCompletes: the same two-pack
// fixture (stem1 = c1's own pack, stem2 = c1->c2's delta, both needed since
// dst starts empty), but stem1's CACHED sidecar is corrupted with the
// altPackingIdx-style same-OID reindex — a syntactically valid idx (correct
// object set, different bytes) built from a SECOND, differently-compressed
// packing of the same closure. This is the only known mechanism that passes
// buildPackMap's pre-loop show-index check (the map it produces is right
// about which objects the stem serves, so greedy planning is unaffected) yet
// fails pack-pair verification mid-round once the REAL pack bytes are
// downloaded and paired against it — the same mechanism the existing
// single-pack TestFetchPairFailureRefreshesSidecarAndCompletes uses, proven
// there to reach downloadAndVerifyPack's mid-loop refresh. Here a second pack
// (stem2) is still left in the plan when that refresh fires, which is
// retro-Codex 1's actual motivating scenario.
//
// HONEST SCOPE: because the corruption preserves the same OID set, the
// rebuilt plan after the refresh is unchanged from before it — this does NOT
// empirically distinguish a round restart from a mid-round resume (both
// complete this fixture identically); that distinction stays structurally
// pinned at unit level, not observed here. What this test pins is completion
// and bounded downloads through a GENUINE mid-round pair-refresh with a
// second pack still in the plan — verified by requiring the first stem's
// .idx to reappear in the transfer trace, proof the mid-loop refresh
// actually fired.
func TestFetchMidRoundPairRefreshWithTwoPacksCompletes(t *testing.T) {
	src := newGitRepoForPush(t)
	c1 := headOfPushRepo(t, src)
	c2 := commitOnPushRepo(t, src, "f2.txt", "two")
	f := transport.NewFake()
	stems := plantIncrementalPacks(t, f, "/r", src, []string{c1, c2})
	stem1, stem2 := stems[0], stems[1]

	cache := t.TempDir()
	if err := os.WriteFile(filepath.Join(cache, stem1+".idx"),
		altPackingIdx(t, src, c1, stem1), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := emptyGitRepo(t)
	var trace strings.Builder
	tr := transport.NewTraced(f, &trace)
	_, err := Fetch(tr, "/r", dst, cache, []string{c2})
	// GUARD (retro-Codex 1, Stage 4): a mid-round pair-refresh with a second
	// pack still in the plan must complete the whole fetch. HONEST SCOPE: the
	// same-OID corruption leaves the rebuilt plan unchanged, so this does NOT
	// empirically distinguish restart from resume (both complete this
	// fixture) — that distinction stays structurally pinned at unit level.
	// What this pins is completion and bounded downloads through a genuine
	// mid-round refresh with a second pack still pending.
	if err != nil {
		t.Fatalf("fetch must survive a mid-round sidecar refresh with two packs: %v", err)
	}
	if !gitcmd.HasObject(dst, c2) {
		t.Errorf("want %s missing after mid-round refresh fetch", c2)
	}
	// Mandatory verification (Stage 4 fix round 1): the .idx re-download in
	// the trace is the proof the mid-loop refresh actually fired, not just
	// the pre-loop heal TestFetchPairFailureWithTwoPacksRefreshesAndCompletes
	// exercises.
	if got := strings.Count(trace.String(), "/packs/"+stem1+".idx"); got < 1 {
		t.Errorf("the mid-round refresh never re-downloaded the lying sidecar; "+
			"the mid-loop pair-refresh path was not reached; trace:\n%s", trace.String())
	}
	// Ordering makes the mid-round-vs-pre-loop distinction self-contained:
	// the lying sidecar is a cache HIT that passes show-index, so the
	// pre-loop heal never downloads stem1's .idx — the only .idx entry the
	// trace can hold for stem1 is the mid-loop refresh, and that fires only
	// AFTER stem1's real pack bytes arrived and failed pair verification.
	// An .idx download BEFORE the .pack is the pre-loop heal's shape.
	if pi, ii := strings.Index(trace.String(), "/packs/"+stem1+".pack"),
		strings.Index(trace.String(), "/packs/"+stem1+".idx"); pi >= 0 && ii >= 0 && ii < pi {
		t.Errorf("stem1's .idx re-download precedes its .pack download — that is "+
			"the pre-loop heal, not the mid-loop pair-refresh; trace:\n%s", trace.String())
	}
	for _, stem := range []string{stem1, stem2} {
		if got := strings.Count(trace.String(), "/packs/"+stem+".pack"); got < 1 || got > 2 {
			t.Errorf("pack %s downloaded %d times, want 1 or 2 (bounded); trace:\n%s",
				stem, got, trace.String())
		}
	}
}

// RED (structural + behavioural): downloadAndVerifyPack with a corrupt
// remote pack must (a) return the checksum error, (b) leave packDir with
// ZERO entries, and (c) leave the corrupt bytes IN THE INCOMING DIR — the
// quarantined residue awaiting wholesale teardown. (c) is the discriminator
// a same-shaped test against unpatched code would lack: unpatched code has
// no incoming dir at all and scrubs its packDir download on failure, so
// "packDir empty" alone would prove nothing about quarantine having ever
// existed.
//
// Deliberate-regression check (mutation-verifies (c) specifically): with the
// old residue rule temporarily ported back onto this failure path
// (`os.Remove(inPack)` added right before the checksum-mismatch return in
// fetch.go), this failed on "the corrupt bytes must remain in the incoming
// dir, awaiting wholesale teardown: open ...\pack-....pack: The system
// cannot find the file specified" — confirming (c) actually exercises the
// quarantine-keeps-its-residue behaviour and is not vacuously true. Reverted
// after confirming.
func TestDownloadAndVerifyQuarantinesCorruptPackBytes(t *testing.T) {
	src := newGitRepoForPush(t)
	sha := headOfPushRepo(t, src)
	packPath, idxPath, err := gitcmd.WritePack(src, []string{sha}, nil, t.TempDir())
	if err != nil || packPath == "" {
		t.Fatalf("WritePack: %v", err)
	}
	stem := strings.TrimSuffix(filepath.Base(packPath), ".pack")
	f := transport.NewFake()
	for _, p := range []string{packPath, idxPath} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f.Files["/r/packs/"+filepath.Base(p)] = b
	}
	// Corrupt the REMOTE pack bytes so the checksum-vs-basename comparison
	// fails once downloaded — the stem's name is the ORIGINAL content's
	// checksum, so any bit flip makes the comparison fail deterministically.
	corrupt := append([]byte{}, f.Files["/r/packs/"+stem+".pack"]...)
	corrupt[len(corrupt)/2] ^= 0xff
	f.Files["/r/packs/"+stem+".pack"] = corrupt

	pm, err := buildPackMap(f, "/r", "", t.TempDir(), []string{stem})
	if err != nil {
		t.Fatalf("buildPackMap: %v", err)
	}
	incomingDir := t.TempDir()
	packDir := t.TempDir()

	_, err = downloadAndVerifyPack(f, "/r", incomingDir, packDir, stem, pm)
	if err == nil {
		t.Fatal("a corrupt remote pack must return an error")
	}
	if !errors.Is(err, errCacheSuspect) || !strings.Contains(err.Error(), "recomputes to") {
		t.Errorf("must return the checksum-mismatch error wrapping errCacheSuspect: %v", err)
	}
	entries, rerr := os.ReadDir(packDir)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if len(entries) != 0 {
		t.Errorf("packDir must have ZERO entries after a corrupt-pack failure, got %d: %v",
			len(entries), entries)
	}
	got, rerr := os.ReadFile(filepath.Join(incomingDir, stem+".pack"))
	if rerr != nil {
		t.Fatalf("the corrupt bytes must remain in the incoming dir, awaiting wholesale "+
			"teardown: %v", rerr)
	}
	if !bytes.Equal(got, corrupt) {
		t.Error("the quarantined pack bytes must be exactly what was downloaded")
	}
}

// RED: publishPair renames .pack before .idx — observed via deterministic
// second-rename failure: a DIRECTORY is pre-created at packDir/<stem>.idx so
// the idx rename must fail (os.Rename onto an existing directory errors on
// this platform). Assert: error returned, AND packDir/<stem>.pack IS present
// (the pack landed first, before the idx rename that failed). With the
// renames swapped, the idx rename fails FIRST and the .pack never lands —
// the presence assertion flips, which is exactly the Step-5 deliberate
// regression this test is designed to catch.
//
// Also pins MOVE, not copy: incomingDir/<stem>.pack must be GONE after its
// successful rename. A copyFile-based publishPair would satisfy every other
// assertion here (packDir/<stem>.pack present, error returned) while
// silently leaving the source behind — doubling temp-disk use for every
// published pack and never actually emptying quarantine on success.
func TestPublishPairRenamesPackBeforeIdx(t *testing.T) {
	stem := "pack-" + strings.Repeat("e", 40)
	incomingDir := t.TempDir()
	packDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(incomingDir, stem+".pack"), []byte("pack-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(incomingDir, stem+".idx"), []byte("idx-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Force the SECOND rename to fail deterministically: a directory sits
	// where the .idx destination must land.
	if err := os.MkdirAll(filepath.Join(packDir, stem+".idx"), 0o700); err != nil {
		t.Fatal(err)
	}

	err := publishPair(incomingDir, packDir, stem)
	if err == nil {
		t.Fatal("publishPair must fail when the .idx rename cannot land on a directory")
	}
	if _, statErr := os.Stat(filepath.Join(packDir, stem+".pack")); statErr != nil {
		t.Errorf("the .pack must already have landed before the failing .idx rename "+
			"(pack-before-idx ordering): %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(incomingDir, stem+".pack")); !os.IsNotExist(statErr) {
		t.Errorf("the .pack must be gone from incoming after landing in packDir — "+
			"publishPair renames (moves), it does not copy: stat err %v", statErr)
	}
}

// GUARD, not RED: passed immediately against Task 6's code. A genuinely
// mismatched REMOTE pair — same-OIDs alt-packing idx planted as the remote's
// own sidecar, so the map is right but the pair can never verify — is fatal
// as "corrupt pair, member undetermined", after the one sidecar refresh
// re-downloads the same wrong bytes. Deliberate-regression check: with the
// "member undetermined" wording removed from downloadAndVerifyPack's final
// fatal, this failed on "fatal must not pretend to know which member is bad"
// — confirming the assertion is live.
func TestFetchGenuinelyCorruptPairIsFatalMemberUndetermined(t *testing.T) {
	src := newGitRepoForPush(t)
	c1 := headOfPushRepo(t, src)
	f := transport.NewFake()
	stems := plantIncrementalPacks(t, f, "/r", src, []string{c1})
	f.Files["/r/packs/"+stems[0]+".idx"] = altPackingIdx(t, src, c1, stems[0])

	dst := emptyGitRepo(t)
	_, err := Fetch(f, "/r", dst, "", []string{c1})
	if err == nil {
		t.Fatal("a genuinely mismatched remote pair must be fatal")
	}
	if !strings.Contains(err.Error(), "member undetermined") {
		t.Errorf("fatal must not pretend to know which member is bad: %v", err)
	}
	if n := countInstalledPackFiles(t, dst); n != 0 {
		t.Errorf("nothing may be installed after a pair fatal; got %d", n)
	}
}

// GUARD, not RED: passed immediately against Task 6's code. A listed .pack
// with no .idx (a push in flight) is skipped; a fetch not needing it
// succeeds. Deliberate-regression check: with listCompletePacks's
// pack-only-no-idx branch changed to include the incomplete stem instead of
// skipping it, this failed on "cannot download pack-fff...fff.idx: not
// found: /r/packs/pack-fff...fff.idx" — confirming the assertion is live.
func TestFetchSkipsAnIncompletePairWhenUnneeded(t *testing.T) {
	src := newGitRepoForPush(t)
	c1 := headOfPushRepo(t, src)
	f := transport.NewFake()
	plantIncrementalPacks(t, f, "/r", src, []string{c1})
	f.Files["/r/packs/pack-"+strings.Repeat("f", 40)+".pack"] = []byte("in-flight, no idx")

	dst := emptyGitRepo(t)
	if _, err := Fetch(f, "/r", dst, "", []string{c1}); err != nil {
		t.Fatalf("an unneeded incomplete pair must not break the fetch: %v", err)
	}
	// GUARD (M3, Stage 4): exit-0 alone does not prove the closure landed —
	// assert the want is actually present in the object store.
	if !gitcmd.HasObject(dst, c1) {
		t.Errorf("fetched closure must contain want %s; a fetch that skips the "+
			"incomplete pair must still deliver everything", c1)
	}
}

// GUARD, not RED: passed immediately against Task 6's code. End-to-end cache
// degradation: an unusable cacheDir (a file) never fails the fetch. Deliberate-
// regression check: with a temporary stat-and-fatal guard inserted ahead of
// pruneStale that errors out when cacheDir is not a real directory, this
// failed on "cache trouble must never be fetch trouble: regression-check:
// cache dir ... is unusable" — confirming the assertion is live.
func TestFetchSucceedsWithUnusableCacheDir(t *testing.T) {
	src := newGitRepoForPush(t)
	c1 := headOfPushRepo(t, src)
	f := transport.NewFake()
	plantIncrementalPacks(t, f, "/r", src, []string{c1})
	notADir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := emptyGitRepo(t)
	if _, err := Fetch(f, "/r", dst, notADir, []string{c1}); err != nil {
		t.Fatalf("cache trouble must never be fetch trouble: %v", err)
	}
	if !gitcmd.HasObject(dst, c1) {
		t.Error("objects missing")
	}
}
