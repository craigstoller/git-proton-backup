package transport

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- WaitDelay: the grandchild-holds-the-pipe hang -------------------------
//
// These tests drive run()'s WaitDelay behaviour against a REAL process tree,
// hermetically: the test binary re-execs ITSELF as the stand-in
// "proton-drive" (the standard os/exec helper-process technique). No Proton
// CLI is invoked, nothing touches the network, and no account is involved.
//
// helperEnv selects the role:
//
//	cli       — the stand-in proton-drive. Prints a version line, then spawns
//	            a grandchild that INHERITS its stdout/stderr (the write end of
//	            the pipe os/exec created for CombinedOutput) and outlives it.
//	            This is the exact shape that makes Wait block forever without
//	            WaitDelay set: EOF on that pipe needs EVERY holder of the write
//	            end to close it, not just the process we waited for.
//	cli-clean — the same stand-in with NO grandchild: the control case that
//	            proves the delay does not fire on a well-behaved command.
//	linger    — the grandchild. Holds the inherited pipe until released.
const (
	helperEnv        = "GPB_TEST_HELPER_ROLE"
	helperExeEnv     = "GPB_TEST_HELPER_EXE"     // path the grandchild is launched from
	helperReleaseEnv = "GPB_TEST_HELPER_RELEASE" // file whose appearance ends the linger

	roleCLI      = "cli"
	roleCLIClean = "cli-clean"
	roleLinger   = "linger"

	// The four EnforceCertified stand-ins, below runVersionRole. Each ignores
	// its args entirely, exactly like runHelperRole above — run() always
	// calls Version() with "--version", so there is nothing to branch on.
	roleCertified      = "version-certified"
	roleWrongVersion   = "version-wrong"
	roleNonzeroVersion = "version-nonzero"
	roleNoToken        = "version-no-token"
)

// helperVersionLine is what the stand-in prints, shaped like the real CLI's
// --version output so Version() can be driven end to end through it.
const helperVersionLine = "Proton Drive CLI " + CertifiedCLI

// lingerCap is a backstop only, so a crashed or killed test can never leave a
// process behind forever. The grandchild normally exits within milliseconds of
// the release file appearing; nothing asserts against this value.
const lingerCap = 30 * time.Second

var (
	helperDir     string // private temp dir holding the grandchild's binary
	helperExe     string // a COPY of the test binary — see setupHelper
	helperRelease string // release marker, deliberately OUTSIDE helperDir
)

func TestMain(m *testing.M) {
	switch os.Getenv(helperEnv) {
	case roleCLI, roleCLIClean:
		runHelperRole(os.Getenv(helperEnv))
	case roleLinger:
		runLingerRole()
	case roleCertified, roleWrongVersion, roleNonzeroVersion, roleNoToken:
		runVersionRole(os.Getenv(helperEnv))
	}
	if err := setupHelper(); err != nil {
		fmt.Fprintln(os.Stderr, "helper setup:", err)
		os.Exit(2)
	}
	code := m.Run()
	teardownHelper()
	os.Exit(code)
}

// runHelperRole is the stand-in proton-drive. It never returns.
func runHelperRole(role string) {
	fmt.Println(helperVersionLine)
	if role == roleCLI {
		g := exec.Command(os.Getenv(helperExeEnv))
		// exec.Cmd de-duplicates Env keeping the LAST occurrence, so this
		// overrides the inherited role while carrying the other two vars
		// through to the grandchild.
		g.Env = append(os.Environ(), helperEnv+"="+roleLinger)
		// *os.File on both, so these handles are inherited directly: the
		// grandchild ends up holding OUR stdout, which is run()'s pipe.
		g.Stdout, g.Stderr = os.Stdout, os.Stderr
		if err := g.Start(); err != nil {
			fmt.Fprintln(os.Stderr, "helper could not start its grandchild:", err)
			os.Exit(3)
		}
	}
	os.Exit(0) // exits at once; the grandchild keeps the pipe open
}

// runLingerRole is the grandchild. It never returns.
func runLingerRole() {
	release := os.Getenv(helperReleaseEnv)
	deadline := time.Now().Add(lingerCap)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(release); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	os.Exit(0)
}

// runVersionRole is the stand-in proton-drive for the EnforceCertified
// tests. Unlike runHelperRole it never spawns a grandchild: WaitDelay
// interaction is TestRunAbandonsAPipeHeldOpenByAGrandchild's concern, not
// this one's, so each role just prints its --version output (or none) and
// exits. It never returns.
func runVersionRole(role string) {
	switch role {
	case roleCertified:
		// The genuine captured shape: two lines, CLI then SDK. The certified
		// path must exercise the real parse, not a synthetic one-liner.
		fmt.Println("Proton Drive CLI " + CertifiedCLI)
		fmt.Println("Proton Drive SDK js@0.20.0+5174900c")
	case roleWrongVersion:
		fmt.Println("Proton Drive CLI cli-drive@9.9.9+deadbeef")
		fmt.Println("Proton Drive SDK js@0.20.0+5174900c")
	case roleNonzeroVersion:
		fmt.Fprintln(os.Stderr, "proton-drive: internal error")
		os.Exit(1)
	case roleNoToken:
		fmt.Println("unexpected output with no cli-drive@ token")
	}
	os.Exit(0)
}

// setupHelper copies the test binary into a private temp directory. The
// GRANDCHILD is launched from that COPY, never from os.Args[0], and this is
// not fastidiousness: Windows locks a running executable's image file, and
// `go test` deletes the test binary the instant the test process exits. A
// grandchild still running from os.Args[0] makes `go test` fail the package
// with "unlinkat ...transport.test.exe: Access is denied" even though every
// test passed — a green suite reported as red.
func setupHelper() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	helperDir, err = os.MkdirTemp("", "gpb-helper-*")
	if err != nil {
		return err
	}
	// The release marker lives OUTSIDE helperDir on purpose: teardown removes
	// helperDir in a retry loop, and a marker inside it could be deleted on an
	// early attempt, stranding the grandchild until its backstop expired.
	helperRelease = helperDir + ".release"

	helperExe = filepath.Join(helperDir, "helper"+filepath.Ext(self))
	in, err := os.Open(self)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(helperExe, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	os.Setenv(helperExeEnv, helperExe)
	os.Setenv(helperReleaseEnv, helperRelease)
	return nil
}

// teardownHelper releases any lingering grandchild and waits for it to be
// gone. A successful RemoveAll IS the proof it exited: the directory cannot be
// removed while the copy inside it is running.
func teardownHelper() {
	if helperDir == "" {
		return
	}
	os.WriteFile(helperRelease, nil, 0o600)
	deadline := time.Now().Add(lingerCap)
	for {
		if err := os.RemoveAll(helperDir); err == nil || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	os.Remove(helperRelease)
}

// shortWaitDelay installs a test-scale WaitDelay and restores the production
// value afterwards.
func shortWaitDelay(t *testing.T, d time.Duration) {
	t.Helper()
	prev := waitDelay
	t.Cleanup(func() { waitDelay = prev })
	waitDelay = d
}

// TestRunAbandonsAPipeHeldOpenByAGrandchild is the I1 regression test. Without
// cmd.WaitDelay this blocks until the grandchild exits — which, in production,
// is however long a stray Node worker happens to live, with git having printed
// nothing at all when the hang lands on the startup version probe.
//
// Why the assertions are conclusive rather than timing-lucky: the grandchild
// only exits when the release marker appears, and nothing writes that marker
// until teardownHelper runs after the whole package. So the pipe is
// GUARANTEED to still be held while run() is executing, and ErrWaitDelay is
// returned only when os/exec gave up on that pipe and closed it. run()
// returning at all is therefore the evidence, not the clock.
func TestRunAbandonsAPipeHeldOpenByAGrandchild(t *testing.T) {
	shortWaitDelay(t, 250*time.Millisecond)
	t.Setenv(helperEnv, roleCLI)

	start := time.Now()
	out, code, err := (&CLI{Exe: os.Args[0]}).run()
	elapsed := time.Since(start)

	if !errors.Is(err, exec.ErrWaitDelay) {
		t.Fatalf("want exec.ErrWaitDelay after the delay fires, got %v (after %s)", err, elapsed)
	}
	// The carve-out every caller depends on: the command SUCCEEDED. os/exec
	// substitutes ErrWaitDelay for a nil error only on a successful exit, so
	// the exit code must still be the real one.
	if code != 0 {
		t.Errorf("want exit code 0 alongside ErrWaitDelay, got %d", code)
	}
	if !strings.Contains(out, helperVersionLine) {
		t.Errorf("output written before the child exited must still be captured, got %q", out)
	}
	if elapsed >= lingerCap {
		t.Errorf("run took %s: it waited out the grandchild's backstop instead of abandoning the pipe", elapsed)
	}
}

// TestVersionSucceedsThroughAWaitDelay pins the "do not change any caller's
// control flow" half of I1 on the caller that matters most: the version probe
// runs unconditionally at STARTUP, before the capabilities exchange. It
// branches on the exit code first, so a zero exit carrying ErrWaitDelay must
// read as the success it is — not as "could not determine the CLI version".
func TestVersionSucceedsThroughAWaitDelay(t *testing.T) {
	shortWaitDelay(t, 250*time.Millisecond)
	t.Setenv(helperEnv, roleCLI)

	v, err := (&CLI{Exe: os.Args[0]}).Version()
	if err != nil {
		t.Fatalf("Version must succeed when only the pipe drain was abandoned: %v", err)
	}
	if v != helperVersionLine {
		t.Errorf("got %q, want %q", v, helperVersionLine)
	}
	if !IsCertified(v) {
		t.Errorf("%q must still be recognised as certified", v)
	}
}

// TestRunReportsNoWaitDelayWhenNothingHoldsThePipe is the control: an ordinary
// command that leaves no grandchild behind must return a nil error, so the
// warning on the ErrWaitDelay branch cannot become routine noise.
func TestRunReportsNoWaitDelayWhenNothingHoldsThePipe(t *testing.T) {
	shortWaitDelay(t, 250*time.Millisecond)
	t.Setenv(helperEnv, roleCLIClean)

	out, code, err := (&CLI{Exe: os.Args[0]}).run()

	if err != nil {
		t.Fatalf("a command with nothing holding its pipe must not report an error, got %v", err)
	}
	if code != 0 {
		t.Errorf("want exit code 0, got %d", code)
	}
	if !strings.Contains(out, helperVersionLine) {
		t.Errorf("got %q", out)
	}
}

// *CLI satisfies Transport as of Task 5's write side (CreateExclusive,
// UpdateRevision, Trash), mirroring the same assertion for *Fake in
// fake_test.go.
var _ Transport = (*CLI)(nil)

const unwrapped = `{"uid":"u1","name":{"ok":true,"value":"x.bundle"},"type":"file",
 "activeRevision":{"uid":"r1","state":"active","claimedSize":8}}`

const wrapped = `{"uid":"u1","name":{"ok":true,"value":"x.bundle"},"type":"file",
 "activeRevision":{"ok":true,"value":{"uid":"r1","state":"active","claimedSize":8}}}`

func TestParseNodeJSONBothShapes(t *testing.T) {
	for name, payload := range map[string]string{"0.7.0": unwrapped, "0.4.6": wrapped} {
		n, err := parseNodeJSON([]byte(payload))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if n.Name != "x.bundle" || n.Size != 8 {
			t.Errorf("%s: got %+v", name, n)
		}
	}
}

// TestCLIStatStartFailureIsNotConfirmedAbsence locks in the fix from Task 4
// review round 1: when the CLI executable itself cannot start (missing from
// PATH, permission denied, ...), run's nil-ProcessState guard must not let
// Stat fold that into "this path does not exist". A start failure is a
// transport error and must come back as a non-nil error, not as a confirmed
// (_, false, nil) absence, and it must not panic getting there.
func TestCLIStatStartFailureIsNotConfirmedAbsence(t *testing.T) {
	c := NewCLI("nonexistent-xyz-binary-git-proton-backup-test")

	_, ok, err := c.Stat("/whatever")

	if err == nil {
		t.Fatalf("want a non-nil error when the CLI binary cannot start, got ok=%v err=nil (reads as confirmed absence)", ok)
	}
	if ok {
		t.Fatalf("want ok=false alongside the error, got ok=true")
	}
}

// TestOutcomeString pins the rule that an UNRECOGNISED Outcome never renders
// as one of the three real ones. The default arm used to return "ambiguous",
// which turned repo.publishPack's fail-closed guard into "pack upload returned
// an unrecognised outcome ambiguous" — a message that contradicts itself and
// withholds the one fact it exists to report. The unknown values here are
// deliberately out of range; Outcome is a closed set today, and that is
// exactly why nothing else would catch a regression in this function.
func TestOutcomeString(t *testing.T) {
	cases := map[Outcome]string{
		Committed:    "committed",
		Refused:      "refused",
		Ambiguous:    "ambiguous",
		Outcome(3):   "outcome(3)",
		Outcome(-1):  "outcome(-1)",
		Outcome(999): "outcome(999)",
	}
	for o, want := range cases {
		if got := o.String(); got != want {
			t.Errorf("Outcome(%d).String() = %q, want %q", int(o), got, want)
		}
	}
	// The point of the change, stated as an assertion rather than a comment.
	for _, o := range []Outcome{Outcome(3), Outcome(-1), Outcome(999)} {
		if s := o.String(); s == "ambiguous" || s == "committed" || s == "refused" {
			t.Errorf("an unrecognised Outcome must not impersonate a real one, got %q", s)
		}
	}
}

func TestClassifyUpload(t *testing.T) {
	cases := []struct {
		t, s, f int
		want    Outcome
	}{
		{1, 0, 0, Committed},
		{0, 1, 0, Refused},
		{0, 0, 1, Ambiguous}, // a reported failure needs reconciliation
		{1, 1, 0, Ambiguous}, // contradictory for one file
		{0, 0, 0, Ambiguous}, // nothing happened; unknown
		{0, 0, 2, Ambiguous},
	}
	for _, c := range cases {
		if got := classifyUpload(c.t, c.s, c.f); got != c.want {
			t.Errorf("(%d,%d,%d): got %v want %v", c.t, c.s, c.f, got, c.want)
		}
	}
}

// TestParseTransferSummaryMissingFieldIsAmbiguous locks in the design rule
// that a missing count field decodes as Ambiguous, never a defaulted zero.
// Without this, a renamed or dropped `failedItems` combined with
// `skippedItems:1` would silently read as a confident Refused for a write
// that may well have landed.
func TestParseTransferSummaryMissingFieldIsAmbiguous(t *testing.T) {
	cases := []struct {
		name string
		json string
		want Outcome
		// wantErr: only decoding-level problems (unparseable JSON, a missing
		// count field) produce an error. A well-formed body that decodes to
		// a genuinely-ambiguous tuple, e.g. (0,0,0), is still Ambiguous per
		// classifyUpload's own truth table, but that is not a decode error.
		wantErr bool
	}{
		{"all fields present, committed", `{"transferredItems":1,"skippedItems":0,"failedItems":0}`, Committed, false},
		{"all fields present, refused", `{"transferredItems":0,"skippedItems":1,"failedItems":0}`, Refused, false},
		{"all fields present but genuinely zero", `{"transferredItems":0,"skippedItems":0,"failedItems":0}`, Ambiguous, false},
		{"failedItems missing entirely", `{"transferredItems":0,"skippedItems":1}`, Ambiguous, true},
		{"transferredItems missing entirely", `{"skippedItems":0,"failedItems":0}`, Ambiguous, true},
		{"empty object, every field missing", `{}`, Ambiguous, true},
		{"unparseable", `not json`, Ambiguous, true},
		// C3's own {uid,ok} shape has none of the count fields: this is the
		// exact case the design flags as "would silently read nulls" if the
		// fields were plain ints instead of pointers.
		{"trash shape fed to the upload parser", `{"uid":"x","ok":true}`, Ambiguous, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseTransferSummary(c.json)
			if got != c.want {
				t.Errorf("got %v want %v (err=%v)", got, c.want, err)
			}
			if c.wantErr && err == nil {
				t.Errorf("want a non-nil error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Errorf("want a nil error, got %v", err)
			}
		})
	}
}

// TestParseTrashResult locks in the design rule that a delete is Committed
// only when affirmatively confirmed by the C3 {uid,ok} array shape, and
// never inferred from exit status alone.
func TestParseTrashResult(t *testing.T) {
	cases := []struct {
		name string
		json string
		want Outcome
	}{
		// Stage 1 C3's literal evidence payload.
		{"C3 evidence payload, ok true", "[\r\n{\"uid\":\"tU-Ot1Sq63NwBcxlnl7IcA~-Rqa_crVUqB4keyIbsgkQw\",\"ok\":true}\r\n]", Committed},
		{"ok explicitly false", `[{"uid":"x","ok":false}]`, Ambiguous},
		{"empty array", `[]`, Ambiguous},
		{"more than one item", `[{"uid":"a","ok":true},{"uid":"b","ok":true}]`, Ambiguous},
		{"unparseable", `not json`, Ambiguous},
		// The upload transfer-summary shape (an object) is not the trash
		// array shape: applying the wrong parser must fail loudly, not read
		// a false success.
		{"upload transfer-summary shape fed to the trash parser", `{"transferredItems":1,"skippedItems":0,"failedItems":0}`, Ambiguous},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseTrashResult(c.json)
			if got != c.want {
				t.Errorf("got %v want %v (err=%v)", got, c.want, err)
			}
			if c.want == Ambiguous && err == nil {
				t.Errorf("want a non-nil error alongside Ambiguous, got nil")
			}
			if c.want != Ambiguous && err != nil {
				t.Errorf("want a nil error for %v, got %v", c.want, err)
			}
		})
	}
}

// TestCLITrashStartFailureIsAmbiguous mirrors
// TestCLIStatStartFailureIsNotConfirmedAbsence: when the CLI binary cannot
// start at all, Trash's Stat-first check must surface that as Ambiguous
// plus a non-nil error, never fold it into the absent-target Committed
// path. No real proton-drive CLI is invoked — the binary never starts.
func TestCLITrashStartFailureIsAmbiguous(t *testing.T) {
	c := NewCLI("nonexistent-xyz-binary-git-proton-backup-test")

	got, err := c.Trash("/whatever")

	if err == nil {
		t.Fatalf("want a non-nil error when the CLI binary cannot start, got outcome=%v err=nil", got)
	}
	if got != Ambiguous {
		t.Fatalf("want Ambiguous when the CLI binary cannot start, got %v", got)
	}
}

// TestCLIUploadStartFailureIsAmbiguous exercises CreateExclusive and
// UpdateRevision end to end against a binary that cannot start, without
// invoking the real proton-drive CLI: c.run's CombinedOutput on an
// unstartable process yields empty output, which parseTransferSummary must
// treat as unparseable — Ambiguous, not a false Committed or Refused.
//
// It also pins the fix for the one CLI method that discarded both the exit
// code and the start error (`out, _, _ := c.run(...)`). With empty output the
// message degraded to a bare "unparseable upload summary: " and the actual
// cause was lost, which is the worst possible diagnostic for the case an
// operator is most likely to hit — the CLI not being installed. The start
// error must survive into the returned error.
//
// The local and remote basenames below MUST agree ("path.txt" both sides):
// Task 2 review round 1 added a basename guard ahead of the upload call
// (checkUploadBasename, shared with the Fake), and a mismatch here would be
// refused by that guard before ever reaching c.run — exercising the C11 path
// this test is not about, and never reaching the start failure this test
// exists to pin. See TestCLIRefusesBasenameMismatchBeforeUpload for the
// mismatch case.
func TestCLIUploadStartFailureIsAmbiguous(t *testing.T) {
	const exe = "nonexistent-xyz-binary-git-proton-backup-test"
	c := NewCLI(exe)

	fns := map[string]func(string, string) (Outcome, error){
		"CreateExclusive": c.CreateExclusive,
		"UpdateRevision":  c.UpdateRevision,
	}
	for name, fn := range fns {
		t.Run(name, func(t *testing.T) {
			got, err := fn("/some/path.txt", "path.txt")
			if err == nil {
				t.Fatalf("want a non-nil error, got outcome=%v err=nil", got)
			}
			if got != Ambiguous {
				t.Fatalf("want Ambiguous, got %v", got)
			}
			// exec's start error names the executable it could not run; that
			// naming is the whole diagnostic value being preserved here.
			if !strings.Contains(err.Error(), exe) {
				t.Errorf("error must preserve the start failure's cause, got %q", err)
			}
			if !errors.Is(err, exec.ErrNotFound) && !strings.Contains(err.Error(), "exited -1") {
				t.Errorf("error must preserve the exit code or the wrapped exec error, got %q", err)
			}
		})
	}
}

// RED (Task 2 review round 1). CLI.CreateExclusive and CLI.UpdateRevision
// pass localFile straight to `upload` without ever consulting p's leaf, so a
// caller that violates the C11 caller contract (local basename must equal
// the remote leaf) writes to the WRONG REMOTE NAME live — the Fake catches
// this via checkUploadBasename, but the CLI does not. The exe under test does
// not exist, which is how this proves the guard fires BEFORE any process is
// spawned, without a real binary: if CreateExclusive/UpdateRevision ever
// reached c.run, the returned error would carry the exec start-failure
// signature seen in TestCLIUploadStartFailureIsAmbiguous (naming exe, e.g.
// "exited -1" or exec.ErrNotFound) instead of the C11 message asserted below.
func TestCLIRefusesBasenameMismatchBeforeUpload(t *testing.T) {
	const exe = "nonexistent-xyz-binary-git-proton-backup-test"
	c := NewCLI(exe)

	fns := map[string]func(string, string) (Outcome, error){
		"CreateExclusive": c.CreateExclusive,
		"UpdateRevision":  c.UpdateRevision,
	}
	for name, fn := range fns {
		t.Run(name, func(t *testing.T) {
			got, err := fn("/remote/dir/target-leaf.txt", "local-mismatch.txt")
			if err == nil {
				t.Fatalf("want a non-nil error for a basename mismatch, got outcome=%v err=nil", got)
			}
			if got != Ambiguous {
				t.Fatalf("want Ambiguous, got %v", got)
			}
			if !strings.Contains(err.Error(), "C11") {
				t.Errorf("error must cite probe C11, got %q", err)
			}
			if !strings.Contains(err.Error(), "local-mismatch.txt") || !strings.Contains(err.Error(), "target-leaf.txt") {
				t.Errorf("error must name both the local basename and the target leaf, got %q", err)
			}
			if strings.Contains(err.Error(), exe) {
				t.Errorf("error must not carry the exec start-failure signature — the guard must fire before any process is spawned, got %q", err)
			}
		})
	}
}

// TestCLIReadToRefusesAMissingDestinationBeforeInvokingTheCLI covers the
// Stage 3a live-gate finding (task 7): the real proton-drive CLI's
// `filesystem download` silently CREATES a missing destination directory and
// succeeds, contradicting the documented Transport contract and the Fake's
// own behaviour — a genuine fake/real divergence caught for the first time
// by a live run. Decided to enforce the documented contract in the wrapper
// rather than loosen it to match the CLI binary: every caller in this
// codebase creates its temp dir with os.MkdirTemp before calling ReadTo, so
// a missing destination always indicates a caller bug, and *CLI now stats
// localDir itself and refuses before ever invoking the CLI.
//
// The exe under test does not exist, which is how this proves the guard
// fires BEFORE any process is spawned, without a real binary: if ReadTo ever
// reached c.run, the returned error would carry the exec start-failure
// signature seen in TestCLIUploadStartFailureIsAmbiguous (naming exe) instead
// of the plain os.Stat error asserted below — the same technique
// TestCLIRefusesBasenameMismatchBeforeUpload above uses for CreateExclusive.
func TestCLIReadToRefusesAMissingDestinationBeforeInvokingTheCLI(t *testing.T) {
	const exe = "nonexistent-xyz-binary-git-proton-backup-test"
	c := NewCLI(exe)

	t.Run("missing directory", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "not-created")
		err := c.ReadTo("/remote/path.txt", missing)
		if err == nil {
			t.Fatal("want a non-nil error for a missing destination")
		}
		if strings.Contains(err.Error(), exe) {
			t.Errorf("error must not carry the exec start-failure signature — the guard must "+
				"fire before any process is spawned, got %q", err)
		}
		if _, statErr := os.Stat(missing); !os.IsNotExist(statErr) {
			t.Error("ReadTo must not have created the destination directory")
		}
	})

	t.Run("destination exists but is a file, not a directory", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "not-a-dir")
		if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		err := c.ReadTo("/remote/path.txt", file)
		if err == nil {
			t.Fatal("want a non-nil error when localDir is a file, not a directory")
		}
		if strings.Contains(err.Error(), exe) {
			t.Errorf("error must not carry the exec start-failure signature — the guard must "+
				"fire before any process is spawned, got %q", err)
		}
	})
}

// RED. parseNodeJSON currently does IsDir: w.Type == "folder", so an
// unrecognised type silently yields a FILE.
func TestParseNodeJSONRejectsUnknownType(t *testing.T) {
	for _, payload := range []string{
		`{"uid":"u1","name":{"ok":true,"value":"x"},"type":"directory"}`, // renamed
		`{"uid":"u1","name":{"ok":true,"value":"x"}}`,                    // absent
		`{"uid":"u1","name":{"ok":true,"value":"x"},"type":""}`,          // empty
	} {
		if _, err := parseNodeJSON([]byte(payload)); err == nil {
			t.Errorf("%s: an unrecognised type must be an error, not a silent IsDir:false", payload)
		}
	}
	// C9 pins these two, and they must still map correctly.
	for payload, wantDir := range map[string]bool{
		`{"uid":"u1","name":{"ok":true,"value":"d"},"type":"folder"}`: true,
		`{"uid":"u1","name":{"ok":true,"value":"f"},"type":"file"}`:   false,
	} {
		n, err := parseNodeJSON([]byte(payload))
		if err != nil {
			t.Fatalf("%s: %v", payload, err)
		}
		if n.IsDir != wantDir {
			t.Errorf("%s: IsDir = %v, want %v", payload, n.IsDir, wantDir)
		}
	}
}

// certifiedRoleCLI, wrongVersionRoleCLI, nonzeroVersionRoleCLI, and
// noTokenRoleCLI each set this test's helper role and hand back a *CLI
// pointed at this same test binary, re-exec'd as the stand-in proton-drive
// via runVersionRole above — the same os/exec helper-process technique
// TestMain already uses for the WaitDelay tests, so EnforceCertified drives
// the real run()/Version() path end to end rather than a mock.
func certifiedRoleCLI(t *testing.T) *CLI {
	t.Helper()
	t.Setenv(helperEnv, roleCertified)
	return &CLI{Exe: os.Args[0]}
}

func wrongVersionRoleCLI(t *testing.T) *CLI {
	t.Helper()
	t.Setenv(helperEnv, roleWrongVersion)
	return &CLI{Exe: os.Args[0]}
}

func nonzeroVersionRoleCLI(t *testing.T) *CLI {
	t.Helper()
	t.Setenv(helperEnv, roleNonzeroVersion)
	return &CLI{Exe: os.Args[0]}
}

func noTokenRoleCLI(t *testing.T) *CLI {
	t.Helper()
	t.Setenv(helperEnv, roleNoToken)
	return &CLI{Exe: os.Args[0]}
}

// RED (Stage 4): EnforceCertified does not exist. The advisory warn in
// cmd/main.go becomes this enforcing check; the design's "refuse to run"
// rule finally matches the code.
func TestEnforceCertifiedAcceptsTheCertifiedBuild(t *testing.T) {
	// role serving "Proton Drive CLI " + CertifiedCLI on --version
	var buf strings.Builder
	if err := EnforceCertified(certifiedRoleCLI(t), false, &buf); err != nil {
		t.Fatalf("the certified build must pass: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("no warning on the certified path, got %q", buf.String())
	}
}

func TestEnforceCertifiedRefusesAMismatchNamingBothSides(t *testing.T) {
	err := EnforceCertified(wrongVersionRoleCLI(t), false, io.Discard)
	if err == nil {
		t.Fatal("an uncertified version must refuse")
	}
	for _, want := range []string{"cli-drive@9.9.9", CertifiedCLI, "GPB_UNCERTIFIED_CLI"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal must name %q, got %q", want, err.Error())
		}
	}
}

func TestEnforceCertifiedOverrideProceedsWithALoudWarning(t *testing.T) {
	var buf strings.Builder
	if err := EnforceCertified(wrongVersionRoleCLI(t), true, &buf); err != nil {
		t.Fatalf("override must proceed: %v", err)
	}
	for _, want := range []string{"UNCERTIFIED", "cli-drive@9.9.9", CertifiedCLI} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("warning must name %q, got %q", want, buf.String())
		}
	}
}

func TestEnforceCertifiedTreatsAFailedVersionAsUndetermined(t *testing.T) {
	// nonzero-exit role: refusal without override, "could not be determined"
	// warning + proceed with it.
	if err := EnforceCertified(nonzeroVersionRoleCLI(t), false, io.Discard); err == nil {
		t.Error("a failed --version must refuse without the override")
	}
	var buf strings.Builder
	if err := EnforceCertified(nonzeroVersionRoleCLI(t), true, &buf); err != nil {
		t.Errorf("override must proceed on an undetermined version: %v", err)
	}
	if !strings.Contains(buf.String(), "could not be determined") {
		t.Errorf("warning must say the version could not be determined, got %q", buf.String())
	}
}

func TestEnforceCertifiedSurfacesAMissingBinary(t *testing.T) {
	// Spec rows: a missing binary is "not an allowlist path" and "the
	// override does not synthesize a binary" — so a binary that never
	// STARTED refuses even WITH the override, carrying the spawn failure.
	err := EnforceCertified(NewCLI("nonexistent-xyz-binary-gpb-test"), false, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "nonexistent-xyz-binary-gpb-test") {
		t.Errorf("refusal must surface the spawn failure, got %v", err)
	}
	if err := EnforceCertified(NewCLI("nonexistent-xyz-binary-gpb-test"), true, io.Discard); err == nil {
		t.Error("the override must not synthesize a missing binary")
	}
}

func TestIsCertifiedIsExactTokenNotContainment(t *testing.T) {
	// RED against the current strings.Contains implementation — the
	// containment cases below PASS it today and must not survive.
	for _, bad := range []string{
		"Proton Drive CLI cli-drive@0.7.0+5174900c-extra",
		"embedded " + CertifiedCLI + "-suffix",
		"no recognisable token here",
		"",
	} {
		if IsCertified(bad) {
			t.Errorf("%q must not be certified under exact-token matching", bad)
		}
	}
	// The real two-line captured output stays certified (SDK line ignored).
	if !IsCertified("Proton Drive CLI " + CertifiedCLI + "\nProton Drive SDK js@0.20.0+5174900c") {
		t.Error("the genuine two-line --version output must be certified")
	}
}

func TestEnforceCertifiedRefusesUnparseableOutput(t *testing.T) {
	// Output with no cli-drive@ token: refusal without override, proceed
	// with it (spec's unparseable rows).
	if err := EnforceCertified(noTokenRoleCLI(t), false, io.Discard); err == nil {
		t.Error("output with no cli-drive@ token must refuse")
	}
	var buf strings.Builder
	if err := EnforceCertified(noTokenRoleCLI(t), true, &buf); err != nil {
		t.Errorf("override must proceed on unparseable output: %v", err)
	}
}

// TestIsCertifiedMatchesTheRealVersionLine pins IsCertified against the real
// --version line under exact-token matching (Stage 4). Containment used to
// be the rationale here — the certified build id sits inside a longer line
// ("Proton Drive CLI " + CertifiedCLI), so an equality check on the WHOLE
// LINE would flag the certified build as uncertified — but containment also
// accepted a build id embedded as a PREFIX of something else entirely
// (TestIsCertifiedIsExactTokenNotContainment, above), which the design's
// "exact versions, not a floor or prefix" forbids. The fix is exact-token:
// extract the whitespace-delimited field that starts "cli-drive@" and
// compare THAT for equality, so the certified line still matches and a
// suffixed impostor no longer does.
func TestIsCertifiedMatchesTheRealVersionLine(t *testing.T) {
	if !IsCertified("Proton Drive CLI " + CertifiedCLI) {
		t.Error("the real --version line must be recognised as certified")
	}
	if !IsCertified(CertifiedCLI) {
		t.Error("a bare build id must also be recognised")
	}
	for _, bad := range []string{
		"Proton Drive CLI cli-drive@0.7.1+deadbeef",
		"Proton Drive CLI cli-drive@0.4.6+abc",
		"",
	} {
		if IsCertified(bad) {
			t.Errorf("%q must not be recognised as certified", bad)
		}
	}
}

// RED. Version does not exist.
func TestVersionSurfacesStartFailure(t *testing.T) {
	if _, err := NewCLI("nonexistent-xyz-binary-gpb-test").Version(); err == nil {
		t.Error("Version must report a CLI that cannot start")
	}
}

// TestDirOf pins the real behaviour of dirOf, the function the Task 5
// review round 1 finding (C11) turned on: CreateExclusive/UpdateRevision
// pass only dirOf(p) to the CLI, so the CLI process's target directory
// depends entirely on this function being correct. Asserts actual observed
// behaviour, not an assumed one.
func TestDirOf(t *testing.T) {
	cases := []struct {
		name string
		p    string
		want string
	}{
		{"nested path", "/a/b/c", "/a/b"},
		{"parent is root", "/a", "/"},
		{"root itself", "/", "/"},
		{"no slash at all", "filename.txt", "/"},
		{"empty string", "", "/"},
		{"relative nested path, no leading slash", "refs/heads/master", "refs/heads"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := dirOf(c.p); got != c.want {
				t.Errorf("dirOf(%q) = %q, want %q", c.p, got, c.want)
			}
		})
	}
}
