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
func TestCLIUploadStartFailureIsAmbiguous(t *testing.T) {
	const exe = "nonexistent-xyz-binary-git-proton-backup-test"
	c := NewCLI(exe)

	fns := map[string]func(string, string) (Outcome, error){
		"CreateExclusive": c.CreateExclusive,
		"UpdateRevision":  c.UpdateRevision,
	}
	for name, fn := range fns {
		t.Run(name, func(t *testing.T) {
			got, err := fn("/some/path.txt", "local.txt")
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

// RED. IsCertified does not exist. The first case is the one that matters:
// the real --version line embeds the build id, so equality would reject the
// certified build itself.
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
