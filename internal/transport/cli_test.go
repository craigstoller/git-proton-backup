package transport

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// --- WaitDelay: the grandchild-holds-the-pipe hang -------------------------
//
// These tests drive run()'s drain bound (waitDelay) against a REAL process
// tree, hermetically: the test binary re-execs ITSELF as the stand-in
// "proton-drive" (the standard os/exec helper-process technique). No Proton
// CLI is invoked, nothing touches the network, and no account is involved.
//
// helperEnv selects the role:
//
//	cli       — the stand-in proton-drive. Prints a version line, then spawns
//	            a grandchild that INHERITS its stdout/stderr (the write end of
//	            the pipe run() created for the child) and outlives it. This is
//	            the exact shape that makes a drain wait forever without a bound:
//	            EOF on that pipe needs EVERY holder of the write end to close
//	            it, not just the process we waited for.
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

	// The six EnforceCertified stand-ins, below runVersionRole. Each ignores
	// its args entirely, exactly like runHelperRole above — run() always
	// calls Version() with "--version", so there is nothing to branch on.
	roleCertified      = "version-certified"
	roleWrongVersion   = "version-wrong"
	roleNonzeroVersion = "version-nonzero"
	roleNoToken        = "version-no-token"
	roleLegacy070      = "version-legacy-070"
	roleLegacy080      = "version-legacy-080"

	// A seventh EnforceCertified stand-in (Linux port, Task 5): a distro's
	// repackaged build, whose identity token reads cli-drive-debian@... rather
	// than cli-drive@.... Handled by runVersionRole like the six above.
	roleDistroToken = "distro-token"

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

	// The environment pass-through stand-in, below runEnvEchoRole (Linux port,
	// Task 2). It echoes two inherited variables back so a test can see what
	// CLI.run actually handed the child.
	roleEnvEcho = "env-echo"

	// The large-output stand-in, below runBigOutputRole. It writes more than a
	// pipe buffer to stdout and exits 0, so a test can see that run reads the
	// pipe while the child is still running instead of after it has exited.
	roleBigOutput = "big-output"

	// The four gate stand-ins (gate_test.go), below runHangRole. Each ignores
	// its args and leaves markers named for its pid (writeMarker), so a test
	// waiting for two children counts two files and can look each one up.
	// hang and hang-ignore-term write "started" and then wait out lingerCap
	// unless killed; short-sleep writes "started", sleeps 300 ms, writes
	// "finished" and exits 0; hang-with-grandchild starts a hang grandchild
	// that stays in its process group and writes "grandchild", then hangs.
	roleHang               = "hang"
	roleHangIgnoreTerm     = "hang-ignore-term"
	roleShortSleep         = "short-sleep"
	roleHangWithGrandchild = "hang-with-grandchild"

	// trash-hang answers `filesystem info` with a file node and exits 0, and
	// hangs like hang on every other call: a Trash whose Stat succeeds and
	// whose trash call is the one killed.
	roleTrashHang = "trash-hang"

	// helperGrandchildEnv, set to any value, makes the hang role write a
	// "grandchild" marker instead of "started": roleHangWithGrandchild's child
	// is told apart from its parent by the marker's name.
	helperGrandchildEnv = "GPB_TEST_HELPER_GRANDCHILD"
)

// bigOutputBytes is how many 'x' bytes roleBigOutput writes: several times any
// pipe buffer (64 KiB on Linux and Windows, 16 KiB on some Unixes), so a child
// that blocks on a full pipe can only finish if the parent is reading.
const bigOutputBytes = 200 * 1024

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
	case roleCertified, roleWrongVersion, roleNonzeroVersion, roleNoToken, roleLegacy070, roleLegacy080, roleDistroToken:
		runVersionRole(os.Getenv(helperEnv))
	case roleStatNotFound, roleStatOtherError:
		runStatRole(os.Getenv(helperEnv))
	case roleEnsureDirFile, roleEnsureDirContraFolder, roleEnsureDirContraFile, roleEnsureDirContraUnresolved:
		runEnsureDirRole(os.Getenv(helperEnv))
	case roleEnvEcho:
		runEnvEchoRole()
	case roleBigOutput:
		runBigOutputRole()
	case roleHang, roleHangIgnoreTerm:
		runHangRole(os.Getenv(helperEnv) == roleHangIgnoreTerm)
	case roleShortSleep:
		runShortSleepRole()
	case roleHangWithGrandchild:
		runHangWithGrandchildRole()
	case roleTrashHang:
		runTrashHangRole()
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
	case roleLegacy070:
		// The exact identity line of the 0.7.0 build (certified through
		// v0.6.0), as literal text rather than a symbolic reference to
		// CertifiedCLI, which has moved twice since (0.8.0, then 0.9.0) —
		// this role exists specifically to drive EnforceCertified's override
		// de-trap test against the KNOWN-incompatible 0.7.0 build.
		fmt.Println("Proton Drive CLI cli-drive@0.7.0+5174900c")
		fmt.Println("Proton Drive SDK js@0.20.0+5174900c")
	case roleLegacy080:
		// The build every install made between 2026-08-13 and 2026-10-05 has,
		// printing the real post-0.9.0 update nag (captured 2026-10-10). Literal
		// text, not CertifiedCLI, which now names 0.9.0.
		fmt.Println("Proton Drive CLI cli-drive@0.8.0+06e8c605")
		fmt.Println("Proton Drive SDK js@0.21.0+06e8c605")
		fmt.Println("A newer version is available: 0.9.0 (you have 0.8.0).")
		fmt.Println("Download at https://proton.me/download/drive/cli/index.html")
	case roleDistroToken:
		// A Debian-style repackage: the same build number as the certified CLI
		// (+75612299), but a distribution-suffixed product token. Literal text,
		// so the role does not move when CertifiedCLI does.
		fmt.Println("Proton Drive CLI cli-drive-debian@0.9.0+75612299")
		fmt.Println("Proton Drive SDK js@0.22.2+75612299")
	}
	os.Exit(0)
}

// runStatRole is the stand-in proton-drive for the Task 4 Stat classification
// tests. Like runVersionRole it never spawns a grandchild. run() always calls
// Stat with "filesystem info ... --json", so there is nothing to branch on in
// the args; the role alone selects which failure shape comes back. Both
// failure messages land on stderr, matching runVersionRole's
// roleNonzeroVersion convention for a CLI failure — and it does not actually
// matter which stream: run()'s single merged pipe puts
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

// runEnvEchoRole is the stand-in proton-drive for the environment pass-through
// test. Like runVersionRole it ignores its args (Version() always passes
// "--version") and spawns no grandchild. It prints the version line, so
// Version() succeeds on it, then one line per variable it echoes back: the
// CLI's real credentials-store variable, and an arbitrary one no filter would
// have a reason to name. It never returns.
func runEnvEchoRole() {
	fmt.Println(helperVersionLine)
	fmt.Println("PDCS=" + os.Getenv("PROTON_DRIVE_CREDENTIALS_STORE"))
	fmt.Println("ARB=" + os.Getenv("GPB_TEST_ARBITRARY_PASSTHROUGH"))
	os.Exit(0)
}

// runBigOutputRole is the stand-in proton-drive for the large-output test. It
// ignores its args and spawns no grandchild: it writes bigOutputBytes of 'x'
// and a closing marker line to stdout in ONE write, then exits 0. The write
// cannot complete until the parent has read most of it, which is the point. It
// never returns.
func runBigOutputRole() {
	os.Stdout.WriteString(strings.Repeat("x", bigOutputBytes) + "\nEND\n")
	os.Exit(0)
}

// writeMarker records that this process reached a point: an empty file named
// <GPB_TEST_HELPER_RELEASE>.<kind>.<pid>. The gate tests point that variable
// into their own temp dir, so they count only their own children's markers.
func writeMarker(kind string) {
	os.WriteFile(fmt.Sprintf("%s.%s.%d", os.Getenv(helperReleaseEnv), kind, os.Getpid()), nil, 0o600)
}

// runHangRole is the stand-in for a CLI call that never finishes on its own.
// With ignoreTerm it ignores SIGTERM first, before the marker that tells the
// test it may start signalling. It then waits like runLingerRole: until the
// release file appears or lingerCap passes, so a child a test forgot to kill
// still ends. It never returns.
func runHangRole(ignoreTerm bool) {
	if ignoreTerm {
		signal.Ignore(syscall.SIGTERM)
	}
	kind := "started"
	if os.Getenv(helperGrandchildEnv) != "" {
		kind = "grandchild"
	}
	writeMarker(kind)
	runLingerRole()
}

// runShortSleepRole is the stand-in for a call that finishes inside the stop
// sequence's grace: the "finished" marker exists only if it was not killed. It
// never returns.
func runShortSleepRole() {
	writeMarker("started")
	time.Sleep(300 * time.Millisecond)
	writeMarker("finished")
	os.Exit(0)
}

// runHangWithGrandchildRole starts a hang grandchild, which stays in this
// process's group (nothing here asks for a new one), then hangs itself, so the
// parent is still alive when a release call's bound fires and the kill must
// take the whole group. The grandchild's stdio is not inherited: the test
// looks for its pid, not its output. It never returns.
func runHangWithGrandchildRole() {
	g := exec.Command(os.Getenv(helperExeEnv))
	g.Env = append(os.Environ(), helperEnv+"="+roleHang, helperGrandchildEnv+"=1")
	if err := g.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "helper could not start its grandchild:", err)
		os.Exit(3)
	}
	writeMarker("started")
	runLingerRole()
}

// runTrashHangRole answers an info call (os.Args: exe, filesystem, info, path,
// --json) with a file node, so Trash's Stat finds the target, and hangs on any
// other call. It never returns.
func runTrashHangRole() {
	if len(os.Args) > 2 && os.Args[1] == "filesystem" && os.Args[2] == "info" {
		fmt.Println(`{"uid":"u1","name":{"value":".lock"},"type":"file"}`)
		os.Exit(0)
	}
	runHangRole(false)
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

// shortWaitDelay installs a test-scale waitDelay, run's post-exit drain bound,
// and restores the production value afterwards.
func shortWaitDelay(t *testing.T, d time.Duration) {
	t.Helper()
	prev := waitDelay
	t.Cleanup(func() { waitDelay = prev })
	waitDelay = d
}

// TestRunAbandonsAPipeHeldOpenByAGrandchild is the I1 regression test. Without
// a bound on the drain this blocks until the grandchild exits — which, in production,
// is however long a stray Node worker happens to live, with git having printed
// nothing at all when the hang lands on the startup version probe.
//
// Why the assertions are conclusive rather than timing-lucky: the grandchild
// only exits when the release marker appears, and nothing writes that marker
// until teardownHelper runs after the whole package. So the pipe is
// GUARANTEED to still be held while run() is executing, and ErrWaitDelay is
// returned only when run gave up on that pipe and closed it. run()
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
	// The carve-out every caller depends on: the command SUCCEEDED. run
	// substitutes ErrWaitDelay only for an exit 0 whose drain ran out, so the
	// exit code must still be the real one.
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

// --- run owns its child (S13) ------------------------------------------------
//
// run no longer uses CombinedOutput: it creates one pipe for both streams,
// reads it on its own goroutine, and waits for the PROCESS apart from the
// drain. These tests pin what that separation adds (the exit is seen before the
// drain ends) and what it must not lose (one merged stream; large outputs).

// TestRunObservesExitBeforeTheDrainEnds is RED against CombinedOutput, which
// ties the child's exit to the end of the drain: with a grandchild holding the
// pipe, Wait returns only once the drain has given up, so testHookExited could
// never fire well before run does. Here it must fire at the exit, and run must
// then spend the drain bound still waiting on the pipe.
//
// The grandchild is released the way TestRunAbandonsAPipeHeldOpenByAGrandchild's
// is: the package's release marker, which nothing writes until teardownHelper,
// so the pipe is held for the whole call and no process outlives the package. A
// marker private to this test would have to be deleted with its temp directory
// right after being written, which can beat the grandchild's next poll and
// strand it until lingerCap.
func TestRunObservesExitBeforeTheDrainEnds(t *testing.T) {
	shortWaitDelay(t, 2*time.Second)
	t.Setenv(helperEnv, roleCLI)

	var exitedAt time.Time
	testHookExited = func(int) { exitedAt = time.Now() }
	out, code, err := (&CLI{Exe: os.Args[0]}).run("--version")
	returnedAfter := time.Since(exitedAt)
	testHookExited = nil

	if exitedAt.IsZero() {
		t.Fatal("testHookExited never fired: the child's exit was not observed apart from the drain")
	}
	if returnedAfter < 1500*time.Millisecond {
		t.Errorf("run returned %s after the exit was observed, want about the 2s drain bound: "+
			"the drain did not run on after the exit", returnedAfter)
	}
	if !errors.Is(err, exec.ErrWaitDelay) {
		t.Errorf("want exec.ErrWaitDelay when an exit-0 child's drain runs out, got %v", err)
	}
	if code != 0 {
		t.Errorf("want exit code 0 alongside ErrWaitDelay, got %d", code)
	}
	if !strings.Contains(out, helperVersionLine) {
		t.Errorf("output written before the exit must still be captured, got %q", out)
	}
}

// TestRunMergesStderrIntoTheOutput is a GUARD: both streams share one pipe, so
// what the child writes to stderr is in run's output, and Stat still reads the
// not-found signature as confirmed absence. It passed under CombinedOutput and
// is here so the rework cannot split the streams.
func TestRunMergesStderrIntoTheOutput(t *testing.T) {
	t.Setenv(helperEnv, roleStatNotFound)
	c := &CLI{Exe: os.Args[0]}

	out, code, err := c.run("filesystem", "info", "/whatever", "--json")

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("want an *exec.ExitError for a non-zero exit, got %v", err)
	}
	if code != 1 {
		t.Errorf("want exit code 1, got %d", code)
	}
	if !strings.Contains(out, notFoundSignature) {
		t.Errorf("stderr must be in run's output, got %q", out)
	}
	n, ok, serr := c.Stat("/whatever")
	if serr != nil || ok || n != (Node{}) {
		t.Errorf("Stat must read the signature as absence, got (%+v, %v, %v)", n, ok, serr)
	}
}

// TestRunDrainsLargeOutputWhileTheChildRuns is a GUARD: a child that writes
// more than a pipe buffer before it exits only finishes if the parent reads the
// pipe meanwhile, and every byte must arrive. It passed under CombinedOutput.
func TestRunDrainsLargeOutputWhileTheChildRuns(t *testing.T) {
	t.Setenv(helperEnv, roleBigOutput)

	out, code, err := (&CLI{Exe: os.Args[0]}).run()

	if err != nil {
		t.Fatalf("a clean exit with a full drain must report no error, got %v", err)
	}
	if code != 0 {
		t.Errorf("want exit code 0, got %d", code)
	}
	if len(out) < bigOutputBytes {
		t.Errorf("got %d bytes, want at least %d: output was lost", len(out), bigOutputBytes)
	}
	if !strings.HasSuffix(strings.TrimSpace(out), "END") {
		t.Errorf("the closing marker is missing: output was cut short (last bytes %q)", out[max(0, len(out)-20):])
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

// --- Task 1.4 (0.8.0 cert plan): the -1 size-unknown sentinel ---------------
//
// Before this fix, a type:"file" node whose revision size never resolved
// silently fell through to Go's zero value (0) — indistinguishable from a
// genuine zero-byte file, and making the size-unknown arm (refs.go's
// readRefClassified band gate, push.go's matching heal-arm gate) UNREACHABLE
// in production: both arms only ever saw a real -1 via a hand-built wrapper
// transport (repo_test.go's sizeOverrideTransport), never via a genuine
// parse. These tests pin parseNodeJSON's own -1 defaulting directly; the
// existing wrapper-transport test (TestScanRefsUnknownSizeSkipsWithoutDownload,
// repo_test.go) already confirms refs.go's arm fires on size<0 and is not
// duplicated here (Task 1.4 audit, evidence file: both consumers already
// branch correctly on size<0; the fix is contained to this parser).

// RED: activeRevision absent entirely for a type:"file" node.
func TestParseNodeJSONFileWithNoActiveRevisionGetsTheSizeUnknownSentinel(t *testing.T) {
	const noRevision = `{"uid":"u1","name":{"value":"x.bundle"},"type":"file"}`
	n, err := parseNodeJSON([]byte(noRevision))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.Size != -1 {
		t.Errorf("a file with no activeRevision must get the -1 sentinel, got Size=%d", n.Size)
	}
}

// RED: activeRevision present and its state parses, but claimedSize itself
// is absent from the JSON — must not silently read as a genuine 0.
func TestParseNodeJSONFileWithClaimedSizeMissingGetsTheSizeUnknownSentinel(t *testing.T) {
	const noClaimedSize = `{"uid":"u1","name":{"value":"x.bundle"},"type":"file",
	 "activeRevision":{"uid":"r1","state":"active"}}`
	n, err := parseNodeJSON([]byte(noClaimedSize))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.Size != -1 {
		t.Errorf("a file whose revision omits claimedSize must get the -1 sentinel, got Size=%d", n.Size)
	}
}

// RED: claimedSize present but wrong-typed (a string, not a number) — the
// unmarshal into both known shapes fails, and the sentinel must survive
// rather than a zero value leaking through.
func TestParseNodeJSONFileWithClaimedSizeWrongTypedGetsTheSizeUnknownSentinel(t *testing.T) {
	const wrongTyped = `{"uid":"u1","name":{"value":"x.bundle"},"type":"file",
	 "activeRevision":{"uid":"r1","state":"active","claimedSize":"not-a-number"}}`
	n, err := parseNodeJSON([]byte(wrongTyped))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.Size != -1 {
		t.Errorf("a wrong-typed claimedSize must get the -1 sentinel, got Size=%d", n.Size)
	}
}

// GUARD: a genuine "claimedSize": 0 must still yield a real 0, never
// confused with the -1 sentinel (a real zero-byte file is a legitimate,
// distinct outcome from "size unknown").
func TestParseNodeJSONFileWithGenuineZeroClaimedSizeStaysZero(t *testing.T) {
	const genuineZero = `{"uid":"u1","name":{"value":"empty.txt"},"type":"file",
	 "activeRevision":{"uid":"r1","state":"active","claimedSize":0}}`
	n, err := parseNodeJSON([]byte(genuineZero))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.Size != 0 {
		t.Errorf("a genuine claimedSize:0 must stay 0, got Size=%d", n.Size)
	}
}

// GUARD: the sentinel is FILE-only — a folder (which the CLI never gives an
// activeRevision to at all) must keep Size at plain 0, not -1. Scope check
// against the Task 1.4 cut line: this fix must not touch folder semantics.
func TestParseNodeJSONFolderSizeStaysZeroNotTheSentinel(t *testing.T) {
	const folder = `{"uid":"u1","name":{"value":"sub"},"type":"folder"}`
	n, err := parseNodeJSON([]byte(folder))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.Size != 0 {
		t.Errorf("a folder's Size must stay 0 (the sentinel is file-only), got %d", n.Size)
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
// invoking the real proton-drive CLI: c.run on an
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

func legacy070RoleCLI(t *testing.T) *CLI {
	t.Helper()
	t.Setenv(helperEnv, roleLegacy070)
	return &CLI{Exe: os.Args[0]}
}

func legacy080RoleCLI(t *testing.T) *CLI {
	t.Helper()
	t.Setenv(helperEnv, roleLegacy080)
	return &CLI{Exe: os.Args[0]}
}

func distroTokenRoleCLI(t *testing.T) *CLI {
	t.Helper()
	t.Setenv(helperEnv, roleDistroToken)
	return &CLI{Exe: os.Args[0]}
}

// envEchoRoleCLI sets the env-echo role and hands back a *CLI pointed at this
// test binary, like the version-role constructors above. The caller sets the
// variables it wants echoed before calling.
func envEchoRoleCLI(t *testing.T) *CLI {
	t.Helper()
	t.Setenv(helperEnv, roleEnvEcho)
	return &CLI{Exe: os.Args[0]}
}

// GUARD (Linux port, spec F8). CLI.run starts the CLI with no cmd.Env, so the
// child inherits the helper's whole environment. The Linux setup relies on
// PROTON_DRIVE_CREDENTIALS_STORE=pass reaching the CLI, and that only works
// while the helper passes it through. This pins that: it fails if run()
// clears the environment or filters out either pinned variable (the real one,
// or an arbitrary one, which catches a filter built around the known name). It
// cannot catch other filters: one that drops some third variable, or rewrites
// a value other than these two, passes this test.
func TestCLIPassesItsEnvironmentThroughUnfiltered(t *testing.T) {
	t.Setenv("PROTON_DRIVE_CREDENTIALS_STORE", "pass")
	t.Setenv("GPB_TEST_ARBITRARY_PASSTHROUGH", "x-7f3a")
	out, err := envEchoRoleCLI(t).Version()
	if err != nil {
		t.Fatalf("Version() through the env-echo role: %v", err)
	}
	for _, want := range []string{"PDCS=pass", "ARB=x-7f3a"} {
		if !strings.Contains(out, want) {
			t.Errorf("the CLI's environment lost a variable: output lacks %q, got %q", want, out)
		}
	}
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

// RED (Linux port, spec D4(a)). A distribution's repackage prints a
// distro-suffixed product token (cli-drive-debian@...). The generic refusal
// names the token but not why it is wrong; the refusal gains one sentence that
// says "repackaged build" and where Proton's own download lives. The GUARD
// assertions already hold: the generic refusal text is kept in front of it.
func TestEnforceCertifiedRefusesADistroRepackageWithASpecificNote(t *testing.T) {
	err := EnforceCertified(distroTokenRoleCLI(t), false, io.Discard)
	if err == nil {
		t.Fatal("a distro-repackaged build must refuse")
	}
	// RED
	for _, want := range []string{"repackaged build", "https://proton.me/download/drive/cli/"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal must say %q, got %q", want, err.Error())
		}
	}
	// GUARD
	for _, want := range []string{CertifiedCLI, "GPB_UNCERTIFIED_CLI"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal must still carry %q, got %q", want, err.Error())
		}
	}
}

// GUARD (spec D4(a)): the note is text only. A distro token never certifies,
// the genuine token still does, and the override path is the generic warning,
// not the repackage note.
func TestDistroTokenNeverCertifies(t *testing.T) {
	const distroOutput = "Proton Drive CLI cli-drive-debian@0.9.0+75612299\nProton Drive SDK js@0.22.2+75612299"
	if IsCertified(distroOutput) {
		t.Errorf("IsCertified(%q) = true; a repackaged build must never certify", distroOutput)
	}
	if !IsCertified("Proton Drive CLI cli-drive@0.9.0+75612299") {
		t.Error("the genuine certified line must still certify")
	}
	var buf strings.Builder
	if err := EnforceCertified(distroTokenRoleCLI(t), true, &buf); err != nil {
		t.Fatalf("override must proceed on a repackaged build: %v", err)
	}
	for _, want := range []string{"UNCERTIFIED", "cli-drive-debian@0.9.0+75612299"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("override warning must name %q, got %q", want, buf.String())
		}
	}
	if strings.Contains(buf.String(), "repackaged") {
		t.Errorf("the override warning must be the generic one, without the repackage note, got %q", buf.String())
	}
}

// GUARD (spec D4(a)): an ordinary version mismatch is not called a repackage.
func TestEnforceCertifiedOmitsTheDistroNoteForAnOrdinaryMismatch(t *testing.T) {
	err := EnforceCertified(wrongVersionRoleCLI(t), false, io.Discard)
	if err == nil {
		t.Fatal("an uncertified version must refuse")
	}
	if strings.Contains(err.Error(), "repackaged") {
		t.Errorf("an ordinary mismatch must not mention a repackage, got %q", err.Error())
	}
}

// RED (compile failure: distroToken does not exist yet). distroToken accepts
// exactly the two line shapes identityToken does (a bare token, or the field
// after "CLI") and, like identityToken, never a token buried in a longer line.
func TestDistroTokenAcceptsOnlyTheTwoIdentityLineShapes(t *testing.T) {
	for _, tc := range []struct {
		name, in string
		wantTok  string
		wantOK   bool
	}{
		{"CLI line", "Proton Drive CLI cli-drive-debian@0.9.0+75612299", "cli-drive-debian@0.9.0+75612299", true},
		{"bare line", "cli-drive-debian@0.9.0+75612299", "cli-drive-debian@0.9.0+75612299", true},
		{"dotted distro name", "Proton Drive CLI cli-drive-ubuntu.2@1.0", "cli-drive-ubuntu.2@1.0", true},
		{"after an SDK line", "Proton Drive SDK js@0.22.2+75612299\nProton Drive CLI cli-drive-debian@0.9.0+75612299", "cli-drive-debian@0.9.0+75612299", true},
		{"the genuine token", "Proton Drive CLI cli-drive@0.9.0+75612299", "", false},
		{"buried in a longer line", "Update available for cli-drive-debian@0.9.0+75612299", "", false},
		{"not preceded by CLI", "Proton Drive SDK cli-drive-debian@0.9.0+75612299", "", false},
		{"empty distro name", "Proton Drive CLI cli-drive-@0.9.0", "", false},
		{"uppercase distro name", "Proton Drive CLI cli-drive-Debian@0.9.0", "", false},
		{"nothing after the at sign", "Proton Drive CLI cli-drive-debian@", "", false},
		{"no output", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tok, ok := distroToken(tc.in)
			if ok != tc.wantOK || tok != tc.wantTok {
				t.Errorf("distroToken(%q) = (%q, %v), want (%q, %v)", tc.in, tok, ok, tc.wantTok, tc.wantOK)
			}
		})
	}
	// The token comes back through bound(tok, 80).
	long := "cli-drive-debian@" + strings.Repeat("9", 200)
	if tok, ok := distroToken("Proton Drive CLI " + long); !ok || tok != bound(long, 80) {
		t.Errorf("a long token must come back bounded: got (%q, %v)", tok, ok)
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

// TestIsCertifiedTolerantOfAnAppendedUpdateNagLine is GUARD (a) from the
// 0.8.0 certification plan (Task 1 step 2, round-3): 0.8.0 added an
// up-to-date/update-nag report line after the identity line — hardening
// must tolerate EXTRA output lines without becoming line-count- or
// position-sensitive. The nag text below is SYNTHETIC: it was invented in
// August 2026 before any real update nag existed, and is kept as a
// synthetic-shape GUARD. The two real shapes the CLI prints — the one-line
// "latest version" report and the two-line "newer version is available"
// nag — are pinned by the ...TheRealLatestVersionLine and
// ...TheRealUpdateNag tests below (0.9.0 certification).
func TestIsCertifiedTolerantOfAnAppendedUpdateNagLine(t *testing.T) {
	v := "Proton Drive CLI " + CertifiedCLI + "\n" +
		"Proton Drive SDK js@0.21.0+06e8c605\n" +
		"An update to 0.8.1 is available!"
	if !IsCertified(v) {
		t.Errorf("an appended update-nag line must not break certification, got false for %q", v)
	}
}

// TestIsCertifiedRefusesATokenMerelyMentionedInANagSentence is GUARD (b):
// the certified token appearing INSIDE a longer, unrelated sentence — not
// on a genuine identity line ("Proton Drive CLI <token>", or a bare token
// alone) — must not certify on that strength alone. Before the round-3
// hardening, IsCertified scanned every whitespace field across the WHOLE
// output for the first "cli-drive@" prefix regardless of which line it sat
// on, so a token merely glanced at inside a nag sentence would have
// satisfied it; identityToken's structural per-line parse (cli.go) closes
// that.
func TestIsCertifiedRefusesATokenMerelyMentionedInANagSentence(t *testing.T) {
	v := "Heads up: your previous session used " + CertifiedCLI + " before you updated."
	if IsCertified(v) {
		t.Errorf("a token embedded in a nag sentence must not certify alone, got true for %q", v)
	}
}

// TestIsCertifiedRefuses070AfterTheBump is the 0.8.0 plan's explicit GUARD:
// the exact-version philosophy (Craig-adjudicated, Stage 2) means a
// formerly certified build refuses once CertifiedCLI moves on — no floor,
// no grace window. Uses the literal 0.7.0 token text, not the CertifiedCLI
// constant, so this test cannot silently pass by tautology after any bump
// (CertifiedCLI has since moved on to 0.9.0; the 0.8.0 twin is below).
func TestIsCertifiedRefuses070AfterTheBump(t *testing.T) {
	if IsCertified("Proton Drive CLI cli-drive@0.7.0+5174900c") {
		t.Error("the formerly certified 0.7.0 token must refuse now that CertifiedCLI has moved on (0.8.0, then 0.9.0)")
	}
}

// TestIsCertifiedRefuses080AfterTheBump is the 0.9.0 certification's explicit
// GUARD, the twin of TestIsCertifiedRefuses070AfterTheBump: the previously-
// certified build refuses once CertifiedCLI moves on — no floor, no grace
// window. The fixture is the REAL four-line output every 0.8.0 install has
// printed since 0.9.0 shipped (captured 2026-10-10), literal text so the test
// cannot pass by tautology.
func TestIsCertifiedRefuses080AfterTheBump(t *testing.T) {
	v := "Proton Drive CLI cli-drive@0.8.0+06e8c605\n" +
		"Proton Drive SDK js@0.21.0+06e8c605\n" +
		"A newer version is available: 0.9.0 (you have 0.8.0).\n" +
		"Download at https://proton.me/download/drive/cli/index.html"
	if IsCertified(v) {
		t.Error("the previously-certified 0.8.0 token must refuse now that CertifiedCLI has moved to 0.9.0")
	}
}

// TestEnforceCertifiedOn080RefusesByNameAndOverrideStaysGeneric pins the
// build every current user has: without the override it refuses, naming the
// found token and the certified one; with the override it proceeds with the
// generic warning and must NOT print the 0.7.0 vocabulary note — 0.9.0 kept
// 0.8.0's conflict-strategy vocabulary (H1), so 0.8.0 is merely uncertified,
// not known-incompatible. The refusal quotes bound(v, 200), which trims the
// TAIL of the output; the token survives because the real CLI prints its
// identity line FIRST (version.ts prints both identity lines before the
// update check runs), so it sits in the first 41 bytes — a nag PREPENDED
// before the identity line is a shape the CLI does not produce, and this
// test does not claim to cover it.
func TestEnforceCertifiedOn080RefusesByNameAndOverrideStaysGeneric(t *testing.T) {
	err := EnforceCertified(legacy080RoleCLI(t), false, io.Discard)
	if err == nil {
		t.Fatal("0.8.0 must refuse without the override")
	}
	for _, want := range []string{"cli-drive@0.8.0+06e8c605", CertifiedCLI, "GPB_UNCERTIFIED_CLI"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal must name %q, got %q", want, err.Error())
		}
	}
	var buf strings.Builder
	if err := EnforceCertified(legacy080RoleCLI(t), true, &buf); err != nil {
		t.Fatalf("override must proceed on 0.8.0: %v", err)
	}
	for _, want := range []string{"UNCERTIFIED", "cli-drive@0.8.0+06e8c605", CertifiedCLI} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("warning must name %q, got %q", want, buf.String())
		}
	}
	if strings.Contains(buf.String(), "cli-drive@0.7.0+5174900c") {
		t.Errorf("0.8.0 is not the known-incompatible 0.7.0 build; its override warning must not carry the 0.7.0 note, got %q", buf.String())
	}
}

// TestIsCertifiedTolerantOfTheRealLatestVersionLine: the third line is the
// literal report 0.8.0 printed while it was the latest (captured at the
// 2026-08-13 gate; version.ts is unchanged 0.8.0→0.9.0). The SDK line is a
// placeholder — never matched.
func TestIsCertifiedTolerantOfTheRealLatestVersionLine(t *testing.T) {
	v := "Proton Drive CLI " + CertifiedCLI + "\n" +
		"Proton Drive SDK js@0.0.0+placeholder\n" +
		"You are running the latest version."
	if !IsCertified(v) {
		t.Errorf("the real latest-version line must not break certification, got false for %q", v)
	}
}

// TestIsCertifiedTolerantOfTheRealUpdateNag: the two-line nag in the shape
// 0.8.0 printed on 2026-10-10 (version numbers substituted for the certified
// build's own future) — two extra lines, one carrying two version numbers in
// parentheses, one a URL, neither an identity line. The SDK line is a
// placeholder — never matched.
func TestIsCertifiedTolerantOfTheRealUpdateNag(t *testing.T) {
	v := "Proton Drive CLI " + CertifiedCLI + "\n" +
		"Proton Drive SDK js@0.0.0+placeholder\n" +
		"A newer version is available: 0.10.0 (you have 0.9.0).\n" +
		"Download at https://proton.me/download/drive/cli/index.html"
	if !IsCertified(v) {
		t.Errorf("the real two-line update nag must not break certification, got false for %q", v)
	}
}

// TestEnforceCertifiedOverrideNamesTheKnown070Incompatibility is the
// round-2 [Gemini] override de-trap: GPB_UNCERTIFIED_CLI=1 still opens the
// escape hatch for the KNOWN-incompatible 0.7.0 build, but the warning must
// additionally name the specific incompatibility (0.8.0's conflict-strategy
// vocabulary) rather than fail opaquely mid-push later. A selective hard-ban
// was considered and rejected (plan, round 1): override still means
// override.
func TestEnforceCertifiedOverrideNamesTheKnown070Incompatibility(t *testing.T) {
	var buf strings.Builder
	if err := EnforceCertified(legacy070RoleCLI(t), true, &buf); err != nil {
		t.Fatalf("override must proceed on the legacy 0.7.0 build: %v", err)
	}
	for _, want := range []string{"UNCERTIFIED", "cli-drive@0.7.0+5174900c", CertifiedCLI, "pushes"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("warning must name %q, got %q", want, buf.String())
		}
	}
	// 0.9.0 certification, final-review fix: 0.7.0 has not been THE
	// previously-certified build since 0.8.0 took over on 2026-08-13; the
	// note must call it a formerly certified build, never "the
	// previously-certified" one — an operator is told a false fact otherwise.
	if strings.Contains(buf.String(), "previously-certified") {
		t.Errorf("the 0.7.0 note must not call it the previously-certified build, got %q", buf.String())
	}
}

// TestEnforceCertifiedOverrideOmitsThe070NoteForAnUnrelatedMismatch pins
// the de-trap's scope: the specific 0.7.0 incompatibility note must be
// named ONLY when the detected build genuinely is 0.7.0 — an unrelated
// mismatched version (e.g. a hypothetical 9.9.9) gets the plain generic
// warning, not a false claim about 0.7.0's vocabulary.
func TestEnforceCertifiedOverrideOmitsThe070NoteForAnUnrelatedMismatch(t *testing.T) {
	var buf strings.Builder
	if err := EnforceCertified(wrongVersionRoleCLI(t), true, &buf); err != nil {
		t.Fatalf("override must proceed: %v", err)
	}
	if strings.Contains(buf.String(), "cli-drive@0.7.0+5174900c") {
		t.Errorf("an unrelated version mismatch must not name the 0.7.0 incompatibility, got %q", buf.String())
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
