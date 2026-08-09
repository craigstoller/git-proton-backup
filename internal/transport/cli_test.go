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

	// The two Stat classification stand-ins, below runStatRole (Task 4). Like
	// the version roles, each ignores its args and just reports a fixed
	// `filesystem info` failure shape on stderr with exit 1 — the roleNonzero-
	// Version convention above, mirrored here because both are CLI FAILURES.
	roleStatNotFound   = "stat-not-found"
	roleStatOtherError = "stat-other-error"

	// The four EnsureDir contradiction stand-ins, below runEnsureDirRole
	// (Task 9b). Unlike every role above, EnsureDir's contradiction path
	// makes MULTIPLE c.run calls within a single method call (info, then
	// create-folder, then a raw re-observing info), so these roles are
	// SCRIPTED SEQUENCES, not single fixed responses — see ensureDirScript.
	roleEnsureDirFile             = "ensuredir-file"
	roleEnsureDirContraFolder     = "ensuredir-contra-folder"
	roleEnsureDirContraFile       = "ensuredir-contra-file"
	roleEnsureDirContraUnresolved = "ensuredir-contra-unresolved"
)

// helperVersionLine is what the stand-in prints, shaped like the real CLI's
// --version output so Version() can be driven end to end through it.
const helperVersionLine = "Proton Drive CLI " + CertifiedCLI

// lingerCap is a backstop only, so a crashed or killed test can never leave a
// process behind forever. The grandchild normally exits within milliseconds of
// the release file appearing; nothing asserts against this value.
const lingerCap = 30 * time.Second

// stepCounterEnv names a file used ONLY by the EnsureDir contradiction roles
// (Task 9b): each of their c.run calls spawns a FRESH subprocess (re-exec'd
// from this same test binary), so "which call is this" cannot be read from
// in-process state the way the single-shot roles above get away with. The
// file survives across those subprocesses — see nextStep.
const stepCounterEnv = "GPB_TEST_STEP_COUNTER_FILE"

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
	case roleStatNotFound, roleStatOtherError:
		runStatRole(os.Getenv(helperEnv))
	case roleEnsureDirFile, roleEnsureDirContraFolder, roleEnsureDirContraFile, roleEnsureDirContraUnresolved:
		runEnsureDirRole(os.Getenv(helperEnv))
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

// runStatRole is the stand-in proton-drive for the Task 4 Stat classification
// tests. Like runVersionRole it never spawns a grandchild. run() always calls
// Stat with "filesystem info ... --json", so there is nothing to branch on in
// the args; the role alone selects which failure shape comes back. Both
// failure messages land on stderr, matching runVersionRole's
// roleNonzeroVersion convention for a CLI failure — and it does not actually
// matter which stream: run()'s cmd.CombinedOutput() (cli.go:51) merges
// stdout and stderr into one string before Stat ever sees it, so a role that
// wrote to stdout instead would exercise the identical code path. It never
// returns.
func runStatRole(role string) {
	switch role {
	case roleStatNotFound:
		// The exact shape the Stage 4 gate captured live (docs/research/gates/
		// stage3b-gate.md, stage4-gate.md): "Node not found: <leaf>".
		fmt.Fprintln(os.Stderr, "Node not found: gpb-remote.json")
	case roleStatOtherError:
		fmt.Fprintln(os.Stderr, "something exploded: quota exceeded")
	}
	os.Exit(1)
}

// ensureDirStep is one scripted response: raw combined output plus exit code.
type ensureDirStep struct {
	out  string
	code int
}

// ensureDirScript returns role's full scripted response sequence, indexed by
// call position (0-based) — see runEnsureDirRole and nextStep. The response
// shapes mirror the genuine ones runStatRole/runVersionRole already use:
// a `filesystem info --json` success prints a parseable node payload and
// exits 0; a failure (not-found, already-exists) prints the CLI's failure
// text and exits nonzero.
func ensureDirScript(role string) []ensureDirStep {
	const (
		fileNode      = `{"uid":"u1","name":{"value":"leaf"},"type":"file"}`
		folderNode    = `{"uid":"u1","name":{"value":"leaf"},"type":"folder"}`
		notFoundOut   = "Node not found: leaf"
		alreadyExists = "create-folder failed: leaf: already exists"
	)
	switch role {
	case roleEnsureDirFile:
		// ONE call: the initial Stat (info) reports a FILE. EnsureDir must
		// refuse without ever reaching create-folder — a second c.run call
		// here would run off the end of the script (see the overrun guard in
		// runEnsureDirRole), which is itself a useful failure signal.
		return []ensureDirStep{{fileNode, 0}}
	case roleEnsureDirContraFolder:
		return []ensureDirStep{{notFoundOut, 1}, {alreadyExists, 1}, {folderNode, 0}}
	case roleEnsureDirContraFile:
		return []ensureDirStep{{notFoundOut, 1}, {alreadyExists, 1}, {fileNode, 0}}
	case roleEnsureDirContraUnresolved:
		return []ensureDirStep{{notFoundOut, 1}, {alreadyExists, 1}, {notFoundOut, 1}}
	}
	return nil
}

// runEnsureDirRole is the stand-in proton-drive for the Task 9b EnsureDir
// contradiction tests. It never spawns a grandchild, and it never branches
// on the args it was given — run() always calls filesystem info/create-
// folder in EnsureDir's own fixed order, so the SCRIPT POSITION alone
// (nextStep) is what selects the response; deliberately not keyed off the
// actual args, because the point of the script is to pin what EnsureDir does
// with a given sequence of raw outputs, not to re-implement the real CLI. It
// never returns.
func runEnsureDirRole(role string) {
	script := ensureDirScript(role)
	n := nextStep()
	if n >= len(script) {
		fmt.Fprintf(os.Stderr, "ensureDir stand-in role %s invoked more times (call #%d) than "+
			"scripted (%d) — EnsureDir made an unexpected extra c.run call\n", role, n+1, len(script))
		os.Exit(1)
		return
	}
	step := script[n]
	fmt.Print(step.out)
	os.Exit(step.code)
}

// nextStep reads-and-increments the step counter file named by
// stepCounterEnv, treating a missing or empty file as step 0. Each of the
// EnsureDir contradiction tests below uses its OWN private counter file
// (t.TempDir()), so there is exactly one writer per file and no locking is
// needed.
func nextStep() int {
	path := os.Getenv(stepCounterEnv)
	n := 0
	if b, err := os.ReadFile(path); err == nil {
		fmt.Sscanf(string(b), "%d", &n)
	}
	os.WriteFile(path, []byte(fmt.Sprintf("%d", n+1)), 0o600)
	return n
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

// TestCLIStatNonNotFoundFailureIsAnErrorNotAbsence is the Stage 4 gate 2b
// masquerade regression: a CLI that ran and reported a NON-not-found failure
// must surface as an error, never as confirmed absence. Before this fix, any
// nonzero `filesystem info` exit was folded into (_, false, nil), so a
// broken CLI under GPB_UNCERTIFIED_CLI=1 read as "not a git-remote-proton
// repo" instead of as the transport failure it actually was.
func TestCLIStatNonNotFoundFailureIsAnErrorNotAbsence(t *testing.T) {
	t.Setenv(helperEnv, roleStatOtherError)
	c := &CLI{Exe: os.Args[0]}

	_, ok, err := c.Stat("/whatever")

	if err == nil {
		t.Fatalf("want a non-nil error for a non-not-found failure, got ok=%v err=nil", ok)
	}
	if !strings.Contains(err.Error(), "quota exceeded") {
		t.Errorf("error must name the underlying failure, got %q", err)
	}
	if ok {
		t.Errorf("want ok=false alongside the error, got ok=true")
	}
}

// TestCLIStatNotFoundSignatureIsConfirmedAbsence is the GUARD: the genuine
// not-found signature — the exact shape the Stage 4 gate captured live —
// stays confirmed absence.
func TestCLIStatNotFoundSignatureIsConfirmedAbsence(t *testing.T) {
	t.Setenv(helperEnv, roleStatNotFound)
	c := &CLI{Exe: os.Args[0]}

	n, ok, err := c.Stat("/whatever")

	if err != nil {
		t.Fatalf("want a nil error for the not-found signature, got %v", err)
	}
	if ok {
		t.Errorf("want ok=false for the not-found signature, got true")
	}
	if n != (Node{}) {
		t.Errorf("want a zero Node alongside confirmed absence, got %+v", n)
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

// TestEnforceCertifiedNeverProceedsOnANonLookupStartFailure is the Task 3
// fix-round-1 regression (plan SUPERSEDED banner, Task 3 review, 2026-08-05).
// The original `errors.Is(verr, exec.ErrNotFound)` never-started check only
// covered the not-on-PATH case: every OTHER way a process can fail to start
// (permission denied, bad executable format, CLI.Exe pointing at a
// directory) fell through to the generic branch and PROCEEDED under
// GPB_UNCERTIFIED_CLI=1, contradicting EnforceCertified's own doc comment
// ("A binary that never STARTED refuses regardless of the override").
//
// A same-named ".exe" file containing plain text is the mechanism, verified
// empirically on this Windows environment (see the scratch check run before
// writing this test): starting it produces `*fs.PathError` — "This version
// of %1 is not compatible with the version of Windows you're running" — not
// exec.ErrNotFound and not *exec.ExitError. A bare directory with NO ".exe"
// suffix was tried first and rejected as the mechanism: on Windows that
// actually resolves through LookPath's PATHEXT probing to the SAME
// ErrNotFound-wrapping error as a plain not-on-PATH miss, so it would not
// have exercised the gap this test exists to close.
func TestEnforceCertifiedNeverProceedsOnANonLookupStartFailure(t *testing.T) {
	dir := t.TempDir()
	fakeExe := filepath.Join(dir, "not-a-real-binary.exe")
	if err := os.WriteFile(fakeExe, []byte("not a real exe, just text\n"), 0o755); err != nil {
		t.Fatalf("writing the bad-executable-format stand-in: %v", err)
	}
	c := NewCLI(fakeExe)

	// Sanity: confirm this really is a never-started, non-ErrNotFound,
	// non-ExitError failure before trusting the assertions below — a false
	// premise here would make the test pass for the wrong reason.
	_, verr := c.Version()
	if verr == nil {
		t.Fatalf("the bad-format stand-in must fail to start, got a nil error")
	}
	if errors.Is(verr, exec.ErrNotFound) {
		t.Fatalf("this regression needs a start failure that is NOT exec.ErrNotFound, got %v", verr)
	}
	var exitErr *exec.ExitError
	if errors.As(verr, &exitErr) {
		t.Fatalf("this regression needs a start failure, not an exit error, got %v", verr)
	}

	if err := EnforceCertified(c, false, io.Discard); err == nil {
		t.Error("a non-ErrNotFound start failure must refuse without the override")
	}
	if err := EnforceCertified(c, true, io.Discard); err == nil {
		t.Error("the override must not proceed on a non-ErrNotFound start failure either")
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

// TestVersionReturnsTheFullOutputIncludingTheSDKLine is the Task 3
// fix-round-1 pin for Version()'s full-output change (plan SUPERSEDED
// banner, Task 3 review, 2026-08-05): the original brief's test list had no
// assertion that would fail if Version() were reverted to first-line-only
// (the old `strings.Cut(strings.TrimSpace(out), "\n")`) — every other
// assertion in this file reads only first-line tokens, which a
// first-line-only Version() also satisfies, so the suite stayed green
// against a silent revert. Deliberate-regression checked (recorded in the
// task report): temporarily restoring the old first-line-only body makes
// this fail on the `strings.Contains(v, "js@0.20.0")` assertion below.
func TestVersionReturnsTheFullOutputIncludingTheSDKLine(t *testing.T) {
	v, err := certifiedRoleCLI(t).Version()
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if !strings.Contains(v, "js@0.20.0") {
		t.Errorf("Version() must carry the SDK line through to diagnostics, got %q", v)
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

// ensureDirRoleCLI sets this test's helper role and its own private step
// counter file, then hands back a *CLI pointed at this same test binary,
// re-exec'd as the stand-in proton-drive via runEnsureDirRole above — the
// same os/exec helper-process technique every other role in this file uses.
func ensureDirRoleCLI(t *testing.T, role string) *CLI {
	t.Helper()
	t.Setenv(helperEnv, role)
	t.Setenv(stepCounterEnv, filepath.Join(t.TempDir(), "step"))
	return &CLI{Exe: os.Args[0]}
}

// TestEnsureDirRefusesAFileAtThePath is the round-1 Codex BLOCKER: today's
// EnsureDir returns nil for ANY existing node without ever reading
// Node.IsDir (cli.go:221-225), so a ref FILE at the path reads as a usable
// folder and the reverse-D/F failure surfaces later with a wrong diagnostic.
// The stand-in role answers the initial info call with a FILE node.
//
// The script has exactly ONE scripted response, but that alone is NOT what
// proves EnsureDir didn't wrongly proceed to create-folder (review round 4,
// M4): a wrongly-proceeding EnsureDir still calls c.run a second time, gets
// the role's overrun-guard failure, and STILL returns a non-nil error —
// so a bare err!=nil assertion passes either way. The content check below
// (the path itself, only ever named by the FIRST-call diagnosis) is what
// actually discriminates "refused correctly, from the Stat alone" from
// "proceeded, then failed for an unrelated overrun reason".
func TestEnsureDirRefusesAFileAtThePath(t *testing.T) {
	c := ensureDirRoleCLI(t, roleEnsureDirFile)
	const path = "/r/refs/heads/main"
	err := c.EnsureDir(path)
	if err == nil {
		t.Fatal("EnsureDir onto a FILE must error, never return nil")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error must name the path, got %q", err.Error())
	}
}

// TestEnsureDirContradictionResolvedByReStatFolder drives the FULL
// contradiction sequence (Task 9b, cli.go's alreadyExistsSignature): the
// initial Stat reports absent, create-folder fails with the already-exists
// signature (the C17 shape), and the re-observation finds a FOLDER —
// EnsureDir must succeed, having resolved the contradiction rather than
// trusting the create-folder failure at face value.
func TestEnsureDirContradictionResolvedByReStatFolder(t *testing.T) {
	c := ensureDirRoleCLI(t, roleEnsureDirContraFolder)
	if err := c.EnsureDir("/r/refs/heads/feature"); err != nil {
		t.Fatalf("a re-observed folder must resolve the contradiction: %v", err)
	}
}

// TestEnsureDirContradictionFileIsNamedConflict is the same sequence, but
// the re-observation finds a FILE: EnsureDir must refuse, naming the
// directory/file conflict rather than the raw (and now misleading)
// create-folder failure text.
func TestEnsureDirContradictionFileIsNamedConflict(t *testing.T) {
	c := ensureDirRoleCLI(t, roleEnsureDirContraFile)
	err := c.EnsureDir("/r/refs/heads/feature")
	if err == nil {
		t.Fatal("a re-observed FILE must refuse, naming the directory/file conflict")
	}
	// Content-check, not just "any error": an UNMODIFIED EnsureDir also
	// errors here (it just wraps create-folder's raw failure and gives up),
	// which would make a bare err!=nil assertion pass for the wrong reason.
	// The re-observation's own diagnosis must actually have run — its error
	// does not wrap c.run's raw exec error (unlike the old bare create-folder
	// wrap, which always ends in "exit status N").
	if strings.Contains(err.Error(), "exit status") {
		t.Errorf("must be the re-observation's OWN file-occupies diagnosis, not the raw "+
			"create-folder wrap, got %q", err.Error())
	}
}

// TestEnsureDirContradictionUnresolvedQuotesBoth is the C17 signature: the
// re-observation STILL reports not-found after create-folder claimed
// already-exists. This is diagnosable, generic robustness ONLY — C17b's own
// ruling (docs/research/probes/c17b-provocation-log.md) is that the
// underlying race was observed once, live, and never reproduced under
// deliberate provocation, so this path must never be claimed as a validated
// live fix, only as generic defence against a hypothesised check-then-create
// race. The error must quote BOTH raw observations verbatim so a genuine
// recurrence stays diagnosable.
func TestEnsureDirContradictionUnresolvedQuotesBoth(t *testing.T) {
	c := ensureDirRoleCLI(t, roleEnsureDirContraUnresolved)
	err := c.EnsureDir("/r/refs/heads/feature")
	if err == nil {
		t.Fatal("an unresolved contradiction must error")
	}
	for _, want := range []string{alreadyExistsSignature, notFoundSignature} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error must quote both raw observations verbatim, missing %q in %q", want, err.Error())
		}
	}
}
