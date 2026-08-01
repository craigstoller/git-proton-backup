package main

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// captureStderr redirects os.Stderr for the duration of fn and returns what
// was written to it. warn() writes straight to os.Stderr, and the point of the
// tests below is precisely that a message REACHES the operator, so the
// assertion has to be made against the real stream rather than an injected
// writer that would not prove anything about the shipped path.
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

// ambiguousTrashTransport wraps a Fake but makes Trash report Ambiguous with a
// nil error — the shape that drives repo.Lock.Release to return its "release
// is unconfirmed" error without any I/O failure occurring. Fake's FailNext is
// only consulted by CreateExclusive, so it cannot produce this.
type ambiguousTrashTransport struct{ *transport.Fake }

func (ambiguousTrashTransport) Trash(string) (transport.Outcome, error) {
	return transport.Ambiguous, nil
}

// TestLoop_LockReleaseFailureIsReportedButDoesNotFailThePush covers the one
// place on this branch where a fully-formed error was silently dropped:
// `_ = lock.Release()`. repo.Lock.Release was given a whole fix round to write
// two operator messages, and neither could ever reach anyone — the push exited
// 0, stderr was silent, .lock stayed on the remote, and since v2 has no
// takeover every subsequent push failed until a human cleared it by hand with
// no idea why.
//
// The second assertion is as important as the first: the exit code must NOT
// escalate. The push itself succeeded, and reporting failure for a completed
// push would make git discard remote-tracking updates for refs that really did
// land.
func TestLoop_LockReleaseFailureIsReportedButDoesNotFailThePush(t *testing.T) {
	ft := transport.NewFake()
	in := bufio.NewScanner(strings.NewReader("list for-push\n\n"))
	out := bufio.NewWriter(&bytes.Buffer{})

	var got int
	stderr := captureStderr(t, func() {
		got = loop(ambiguousTrashTransport{ft}, "/my-files/r", ".", in, out)
	})

	if got != 0 {
		t.Errorf("loop() = %d, want 0: a failed lock release must not turn a completed session into a reported failure", got)
	}
	if !strings.Contains(stderr, "unconfirmed") {
		t.Errorf("stderr = %q, want the unconfirmed-release message; discarding it leaves the operator with a wedged repo and no diagnostic", stderr)
	}
	if !strings.Contains(stderr, "/my-files/r/.lock") {
		t.Errorf("stderr = %q, want the lock path named so it can be cleared by hand", stderr)
	}
}

// TestRun_RefusesUnsupportedAddress proves the canonicalisation is actually
// WIRED IN, not merely present: run() must reject the address and return
// non-zero before it reaches the command loop. /shared-with-me is the case
// worth pinning here because it is a safety rule — the CLI permits creating
// there, and a repo in a folder another person can write to is exactly the
// concurrent-writer failure the single-writer model cannot survive.
//
// Hermetic: run() returns at the canonicalisation check, before it constructs
// a transport or reads a byte of stdin. No proton-drive CLI is invoked.
func TestRun_RefusesUnsupportedAddress(t *testing.T) {
	cases := []struct{ name, addr, wantIn string }{
		{"shared-with-me", "proton::/shared-with-me/theirrepo", "concurrent-writer"},
		{"foreign namespace", "proton::/trash/r", "must lie under"},
		{"empty", "proton::", "empty remote address"},
		{"dotdot", "proton::/my-files/../devices/r", "rejected rather than resolved"},
	}
	orig := os.Args
	defer func() { os.Args = orig }()

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			os.Args = []string{"git-remote-proton", "proton-v2", c.addr}
			var got int
			stderr := captureStderr(t, func() { got = run() })
			if got != 1 {
				t.Errorf("run() = %d, want 1 for %q", got, c.addr)
			}
			if !strings.Contains(stderr, c.wantIn) {
				t.Errorf("stderr = %q, want it to name the reason (%q)", stderr, c.wantIn)
			}
		})
	}
}

// Finding 2 (bonus coverage, same harness): a poisoned batch's error lines
// must carry a single well-formed ref token, produced by the same
// protocol.ParsePushBatch used on the non-poisoned path — not a hand-rolled
// split on ":" that, for a malformed line, would leak the "push " prefix
// into the ref field of "error <ref> <reason>".
//
// On its own this test cannot fail: its push line HAS a colon, so the old
// hand-rolled `dst := b[strings.Index(b, ":")+1:]` would have produced a
// byte-identical status line. The colonless input where the two parsers
// genuinely diverge is covered by the sibling test below, and the two together
// are what make this real coverage.
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

// TestLoop_PoisonedBatch_ColonlessPushLineFailsClosed is the input on which
// the unified parser and the old hand-rolled one actually diverge, and so the
// test that gives the sibling above its teeth.
//
// With no colon in the push line, the old poison path computed
// dst = b[strings.Index(b, ":")+1:] on an Index of -1, yielding the WHOLE raw
// line — "push refs/heads/main" — and emitted "error push refs/heads/main
// unsupported option cas". git reads that as ref="push" followed by garbage:
// a status line for a ref that does not exist, with the real ref silently
// unreported. protocol.ParsePushBatch instead rejects the batch as malformed
// before the poison check runs, so the session fails closed with no protocol
// output at all — poisoned or not, an unparseable batch was never going to be
// applied.
func TestLoop_PoisonedBatch_ColonlessPushLineFailsClosed(t *testing.T) {
	script := "list for-push\n" +
		"option cas true\n" +
		"push refs/heads/main\n" + // no colon: this is the whole point
		"\n"
	in := bufio.NewScanner(strings.NewReader(script))
	var outBuf bytes.Buffer
	out := bufio.NewWriter(&outBuf)
	ft := transport.NewFake()

	var got int
	stderr := captureStderr(t, func() { got = loop(ft, "/remote/root", ".", in, out) })
	out.Flush()

	if got != 1 {
		t.Fatalf("loop() = %d, want 1: a malformed batch must fail closed even when it is also poisoned", got)
	}
	if strings.Contains(outBuf.String(), "error push ") {
		t.Errorf("stdout = %q, contains the malformed ref token the old parser produced (\"push \" leaked into the ref field)", outBuf.String())
	}
	if strings.Contains(outBuf.String(), "unsupported option") {
		t.Errorf("stdout = %q, want no poison status lines at all for a batch that could not be parsed", outBuf.String())
	}
	if !strings.Contains(stderr, "malformed refspec") {
		t.Errorf("stderr = %q, want the parse failure reported", stderr)
	}
	if _, ok := ft.Files["/remote/root/refs/heads/main"]; ok {
		t.Error("a malformed batch must not write the ref")
	}
}
