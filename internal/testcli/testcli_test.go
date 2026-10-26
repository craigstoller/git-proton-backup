package testcli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// sleepRoleEnv makes this test binary a stand-in shim that lives for that many
// milliseconds and exits: the process a late writer waits on.
const sleepRoleEnv = "GPB_TESTCLI_TEST_SLEEP_MS"

// TestMain lets this test binary be the late writer, which the shim starts as
// a copy of itself (os.Executable), and the stand-in shim.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == lateWriterRole {
		os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
	}
	if ms := os.Getenv(sleepRoleEnv); ms != "" {
		n, _ := strconv.Atoi(ms)
		time.Sleep(time.Duration(n) * time.Millisecond)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestParseRulesAcceptsTheGrammar(t *testing.T) {
	rules, err := parseRules(" upload:*.pack:once:before ; ;trash:.lock:every:after:delay=1500;" +
		"upload:.lock:once:before:late=100:fail;--version:*:every:before:ignoreterm")
	if err != nil {
		t.Fatal(err)
	}
	want := []rule{
		{n: 0, verb: "upload", leaf: "*.pack", once: true, delay: -1, late: -1},
		{n: 1, verb: "trash", leaf: ".lock", after: true, delay: 1500 * time.Millisecond, late: -1},
		{n: 2, verb: "upload", leaf: ".lock", once: true, delay: -1, late: 100 * time.Millisecond, fail: true},
		{n: 3, verb: "--version", leaf: "*", delay: -1, late: -1, ignoreTerm: true},
	}
	if len(rules) != len(want) {
		t.Fatalf("got %d rules, want %d", len(rules), len(want))
	}
	for i, r := range rules {
		if *r != want[i] {
			t.Errorf("rule %d = %+v, want %+v", i, *r, want[i])
		}
	}
}

func TestParseRulesRefusesWhatTheGrammarDoesNot(t *testing.T) {
	for _, s := range []string{
		"list:*:once:before",                   // not a rule verb
		"upload:a*b:once:before",               // * only as a prefix
		"upload::once:before",                  // no leaf
		"upload:x:twice:before",                // once|every
		"upload:x:once:during",                 // before|after
		"upload:x:once",                        // too few fields
		"upload:x:once:before:delay",           // delay needs a value
		"upload:x:once:before:delay=-1",        // not a duration
		"upload:x:once:before:delay=1:delay=2", // twice
		"upload:x:once:before:bogus",           // unknown option
		"trash:x:once:before:late=1",           // late is for upload
		"upload:x:once:before:fail:delay=1",    // fail does not block
		"upload:x:once:before:fail:ignoreterm", // likewise
	} {
		if _, err := parseRules(s); err == nil {
			t.Errorf("parseRules(%q) accepted it", s)
		}
	}
}

func TestInvocationNamesWhatTheLeafMatches(t *testing.T) {
	for _, c := range []struct {
		args       []string
		verb, name string
		ok         bool
	}{
		{[]string{"--version"}, "--version", "version", true},
		{[]string{"filesystem", "upload", "-f", "skip", "--json", filepath.Join("stage", "pack-1.pack"), "/my-files/r/packs"},
			"upload", "pack-1.pack", true},
		{[]string{"filesystem", "create-folder", "/my-files", "r"}, "create-folder", "r", true},
		{[]string{"filesystem", "download", "/my-files/r/.lock", "dest"}, "download", ".lock", true},
		{[]string{"filesystem", "trash", "/my-files/r/.lock", "--json"}, "trash", ".lock", true},
		{[]string{"filesystem", "info", "/my-files/r/refs/heads/", "--json"}, "info", "heads", true},
		{[]string{"filesystem", "list", "/my-files/r", "--json"}, "", "", false},
		{[]string{"filesystem", "upload", "x"}, "", "", false},
	} {
		verb, name, ok := invocation(c.args)
		if verb != c.verb || name != c.name || ok != c.ok {
			t.Errorf("invocation(%q) = %q, %q, %v; want %q, %q, %v", c.args, verb, name, ok, c.verb, c.name, c.ok)
		}
	}
	r := &rule{n: 4, verb: "upload", leaf: "*.pack"}
	if !r.matches("upload", "pack-1.pack") || r.matches("upload", "pack-1.idx") || r.matches("trash", "pack-1.pack") {
		t.Error("*.pack must match a .pack upload and nothing else")
	}
	if all := (&rule{verb: "info", leaf: "*"}); !all.matches("info", "") || !all.matches("info", ".lock") {
		t.Error("* must match any name")
	}
	r.name = `a:b*c d\é`
	if got, want := r.tag(), "upload-a_b_c_d__-4"; got != want {
		t.Errorf("tag() = %q, want %q", got, want)
	}
}

// shimEnv points a shim at a new tree and ready dir, with rules, and returns
// both.
func shimEnv(t *testing.T, rules string) (tree, ready string) {
	t.Helper()
	tree, ready = t.TempDir(), t.TempDir()
	t.Setenv(TreeEnv, tree)
	t.Setenv(ReadyDirEnv, ready)
	t.Setenv(BlockEnv, rules)
	return tree, ready
}

// stagedLeaf writes a local file named leaf.txt holding content, as
// AcquireLock stages its uploads, and returns the upload argv for it.
func stagedLeaf(t *testing.T, content string) []string {
	t.Helper()
	local := filepath.Join(t.TempDir(), "leaf.txt")
	if err := os.WriteFile(local, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return []string{"filesystem", "upload", "-f", "skip", "--json", local, "/my-files/r"}
}

func runShim(args []string) (code int, stdout string) {
	var out, errOut bytes.Buffer
	code = Run(args, &out, &errOut)
	return code, out.String()
}

func TestRunAppliesFailOnceEveryAndDelay(t *testing.T) {
	tree, ready := shimEnv(t, "upload:leaf.txt:once:before:fail;upload:leaf.txt:once:after:fail;"+
		"upload:*.txt:every:after:delay=0")
	args := stagedLeaf(t, "one")
	target := LocalPath(tree, "/my-files/r/leaf.txt")

	// Rule 0: fails before looking at the target. No summary, nothing written.
	if code, out := runShim(args); code != 1 || out != "" {
		t.Fatalf("first call = %d, %q; want 1 and no output", code, out)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("a before rule wrote the target (stat err %v)", err)
	}
	b, err := os.ReadFile(filepath.Join(ready, "ready-upload-leaf.txt-0"))
	if err != nil || string(b) != strconv.Itoa(os.Getpid())+"\n" {
		t.Fatalf("ready file = %q, %v; want this process's pid", b, err)
	}
	// Rule 0 is spent, so rule 1: the target is written, and still no summary.
	if code, out := runShim(args); code != 1 || out != "" {
		t.Fatalf("second call = %d, %q; want 1 and no output", code, out)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("an after rule did not let the write happen: %v", err)
	}
	// Rule 2 fires every time and carries on after its delay: the skip summary.
	for i := 0; i < 2; i++ {
		if code, out := runShim(args); code != 0 || !strings.Contains(out, `"skippedItems":1`) {
			t.Fatalf("call %d = %d, %q; want 0 and a skip summary", 3+i, code, out)
		}
	}
	if _, err := os.Stat(filepath.Join(ready, "ready-upload-leaf.txt-2")); err != nil {
		t.Fatalf("rule 2 wrote no ready file: %v", err)
	}
	// Every run removed its pid record as it returned.
	if left, _ := os.ReadDir(filepath.Join(tree, PidsDir)); len(left) != 0 {
		t.Fatalf("pid records left after normal returns: %v", left)
	}
}

func TestRunBlocksBeforeTheSkipDecisionUntilTheStopFile(t *testing.T) {
	tree, ready := shimEnv(t, "upload:leaf.txt:once:before")
	args := stagedLeaf(t, "one")
	target := LocalPath(tree, "/my-files/r/leaf.txt")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("theirs"), 0o644); err != nil {
		t.Fatal(err)
	}
	type result struct {
		code int
		out  string
	}
	done := make(chan result, 1)
	go func() {
		code, out := runShim(args)
		done <- result{code, out}
	}()
	// An upload that would be skipped (Refused) still blocks first.
	readyFile := filepath.Join(ready, "ready-upload-leaf.txt-0")
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(readyFile); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no ready file within 10s")
		}
		time.Sleep(10 * time.Millisecond)
	}
	select {
	case r := <-done:
		t.Fatalf("the blocked call returned %d, %q before the stop file", r.code, r.out)
	case <-time.After(300 * time.Millisecond):
	}
	// The harness's assertNoShimLeft reads <tree>/.pids: with no record there it
	// would pass on nothing. The blocked run's pid, from its ready file, must be
	// recorded while it is blocked.
	b, err := os.ReadFile(readyFile)
	if err != nil {
		t.Fatal(err)
	}
	pidRecord := filepath.Join(tree, PidsDir, strings.TrimSpace(string(b)))
	if _, err := os.Stat(pidRecord); err != nil {
		t.Errorf("a blocked run must record its pid as %s: %v", pidRecord, err)
	}
	if err := os.WriteFile(filepath.Join(ready, StopFile), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case r := <-done:
		if r.code != 1 || r.out != "" {
			t.Fatalf("stopped call = %d, %q; want 1 and no summary", r.code, r.out)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the stop file did not end the block within 10s")
	}
	if b, _ := os.ReadFile(target); string(b) != "theirs" {
		t.Fatalf("target = %q, want it untouched", b)
	}
	// A run the stop file ended returned normally, so its record is gone.
	if _, err := os.Stat(pidRecord); !os.IsNotExist(err) {
		t.Errorf("%s is still there after the run returned (stat err %v)", pidRecord, err)
	}
}

// waitRemoved waits up to 10 s for path to be gone.
func waitRemoved(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s still there after 10s", path)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestRunStartsAndRecordsTheLateWriterBeforeFailing(t *testing.T) {
	tree, ready := shimEnv(t, "upload:leaf.txt:once:before:late=100:fail")
	args := stagedLeaf(t, "the staged lock")
	if code, out := runShim(args); code != 1 || out != "" {
		t.Fatalf("call = %d, %q; want 1 and no output", code, out)
	}
	if b, err := os.ReadFile(filepath.Join(ready, "late-0.data")); err != nil || string(b) != "the staged lock" {
		t.Fatalf("late-0.data = %q, %v; want the staged bytes", b, err)
	}
	recs, err := os.ReadDir(filepath.Join(tree, PidsDir))
	if err != nil || len(recs) != 1 || !strings.HasPrefix(recs[0].Name(), LatePidPrefix) {
		t.Fatalf("pid records = %v, %v; want only the late writer's", recs, err)
	}
	rec := filepath.Join(tree, PidsDir, recs[0].Name())
	if b, _ := os.ReadFile(rec); string(b) != "100\n" {
		t.Fatalf("late writer record holds %q, want its delay", b)
	}
	pid, _ := strconv.Atoi(strings.TrimPrefix(recs[0].Name(), LatePidPrefix))
	// The "shim" is this test process, which outlives the test: the late
	// writer waits on it until the stop file ends the wait.
	time.Sleep(200 * time.Millisecond)
	if !processAlive(pid) {
		t.Fatal("the late writer did not wait for the shim's death")
	}
	if err := os.WriteFile(filepath.Join(ready, StopFile), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// It removes its record as it returns. Its pid is no measure here: its
	// parent is this process, which never reaps it, so on Unix it stays a
	// zombie. A real late writer's parent, the shim, is dead by then, so it
	// is reparented and reaped.
	waitRemoved(t, rec)
	if _, err := os.Stat(LocalPath(tree, "/my-files/r/leaf.txt")); !os.IsNotExist(err) {
		t.Errorf("a stopped late writer wrote the target (stat err %v)", err)
	}
}

func TestLateWriterLandsTheDelayAfterTheShimDies(t *testing.T) {
	tree, ready := shimEnv(t, "")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// The shim cannot die sooner than 400 ms after started, so a writer that
	// waits for its death and then 100 ms cannot land sooner than 500 ms after
	// it. That holds on both platforms. The moment the test's Wait returns
	// does not: Windows sets a process's exit code, which processAlive reads,
	// before it signals the handle Wait waits on.
	started := time.Now()
	shim := exec.Command(self)
	shim.Env = append(os.Environ(), sleepRoleEnv+"=400")
	if err := shim.Start(); err != nil {
		t.Fatal(err)
	}
	go shim.Wait() // reaps it, so it reads as gone on Unix
	data := filepath.Join(ready, "late-0.data")
	if err := os.WriteFile(data, []byte("late bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := LocalPath(tree, "/my-files/r/.lock")
	lw := exec.Command(self, lateWriterRole, strconv.Itoa(shim.Process.Pid), "100", data, target)
	detach(lw)
	if err := lw.Start(); err != nil {
		t.Fatal(err)
	}
	waited := make(chan struct{})
	go func() {
		lw.Wait()
		close(waited)
	}()
	t.Cleanup(func() {
		os.WriteFile(filepath.Join(ready, StopFile), nil, 0o644)
		<-waited
	})

	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(target); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the late writer wrote nothing within 10s")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if el := time.Since(started); el < 500*time.Millisecond {
		t.Errorf("landed %v after the shim started; a writer that waited for its death (400ms) "+
			"and then its 100ms delay lands no sooner than 500ms", el)
	}
	if b, _ := os.ReadFile(target); string(b) != "late bytes" {
		t.Errorf("target = %q, want the late bytes", b)
	}
}
