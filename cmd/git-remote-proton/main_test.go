package main

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// errReader always fails with err, simulating a genuine stdin read failure
// (a broken pipe, git crashing mid-session) rather than a clean EOF. It never
// reaches io.EOF, so bufio.Scanner.Err() must return err, not nil.
type errReader struct{ err error }

func (r errReader) Read(p []byte) (int, error) { return 0, r.err }

// Finding 1: a non-EOF scan error must fail closed (exit 1), not be treated
// the same as a clean session end (exit 0). Before the fix, in.Err() was
// never checked after the loop, so this returned 0.
func TestLoop_ScanErrorFailsClosed(t *testing.T) {
	in := bufio.NewScanner(errReader{err: errors.New("broken pipe")})
	out := bufio.NewWriter(&bytes.Buffer{})

	got := loop(transport.NewFake(), "/remote/root", ".", in, out)

	if got != 1 {
		t.Fatalf("loop() = %d, want 1: a non-EOF scan error must be reported as failure, not silently treated as a clean end of session", got)
	}
}

// A clean EOF (no read error) must still succeed — the fix must not turn
// every ordinary session end into a failure.
func TestLoop_CleanEOFSucceeds(t *testing.T) {
	in := bufio.NewScanner(strings.NewReader("capabilities\n"))
	out := bufio.NewWriter(&bytes.Buffer{})

	got := loop(transport.NewFake(), "/remote/root", ".", in, out)

	if got != 0 {
		t.Fatalf("loop() = %d, want 0: a clean EOF with no read error must not be reported as failure", got)
	}
}

// Finding 3: a "push" batch with no preceding successful "list for-push"
// must be rejected before it can write anything, because the lock (acquired
// only inside "list for-push") is what serializes writers. Before the fix
// there was no guard, and the batch would have gone straight into
// repo.Push, writing packs/refs with no mutual exclusion.
func TestLoop_PushWithoutListForPushRejected(t *testing.T) {
	in := bufio.NewScanner(strings.NewReader("push refs/heads/main:refs/heads/main\n\n"))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)
	ft := transport.NewFake()

	got := loop(ft, "/remote/root", ".", in, out)
	out.Flush()

	if got != 1 {
		t.Fatalf("loop() = %d, want 1: push before list for-push must be rejected", got)
	}
	if len(ft.Files) != 0 || len(ft.Dirs) != 0 {
		t.Fatalf("transport was written to despite no lock ever being held: Files=%v Dirs=%v", ft.Files, ft.Dirs)
	}
	if outBuf.Len() != 0 {
		t.Fatalf("stdout got %q, want nothing: a rejected push must not emit protocol output", outBuf.String())
	}
}

// Finding 2 (bonus coverage, same harness): a poisoned batch's error lines
// must carry a single well-formed ref token, produced by the same
// protocol.ParsePushBatch used on the non-poisoned path — not a hand-rolled
// split on ":" that, for a malformed line, would leak the "push " prefix
// into the ref field of "error <ref> <reason>".
func TestLoop_PoisonedBatch_EmitsWellFormedRefToken(t *testing.T) {
	script := "list for-push\n" +
		"option cas true\n" +
		"push refs/heads/main:refs/heads/main\n" +
		"\n"
	in := bufio.NewScanner(strings.NewReader(script))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)
	ft := transport.NewFake()

	got := loop(ft, "/remote/root", ".", in, out)
	out.Flush()

	if got != 0 {
		t.Fatalf("loop() = %d, want 0: a poisoned batch is reported to git via status lines, not a fatal exit", got)
	}
	if !strings.Contains(outBuf.String(), "error refs/heads/main unsupported option cas\n") {
		t.Fatalf("stdout = %q, want a well-formed \"error refs/heads/main unsupported option cas\" line", outBuf.String())
	}
	if strings.Contains(outBuf.String(), "error push ") {
		t.Fatalf("stdout = %q, contains the malformed pre-fix ref token (\"push \" prefix leaked into the ref field)", outBuf.String())
	}
	if _, ok := ft.Files["/remote/root/refs/heads/main"]; ok {
		t.Fatalf("a poisoned batch must not write the ref: found /remote/root/refs/heads/main in the fake transport")
	}
}
