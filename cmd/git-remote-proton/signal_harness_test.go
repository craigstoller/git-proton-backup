package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/craigstoller/git-proton-backup/internal/repo"
	"github.com/craigstoller/git-proton-backup/internal/testcli"
)

// The end-to-end signal harness (spec §6, layer 3). It runs the REAL helper:
// a copy of this test binary named git-remote-proton-testhelper (TestMain's
// helper role), speaking the git remote-helper protocol on its stdin and
// stdout, shelling out through PATH to the scripted shim (buildShimOnPath),
// whose block rules (testcli.BlockEnv) hold a CLI call where a test wants to
// send its signal. The signal cases live in signal_unix_test.go and
// signal_windows_test.go.
//
// Every wait here is bounded and fails the test at its deadline, and every
// process a case leaves running is ended in t.Cleanup, so a regression fails
// instead of hanging the package and nothing outlives the test.

// harnessRoot is the remote root a helper pushes to when its test gives no
// address.
const harnessRoot = "/my-files/r"

// helperOpts is how a case starts the helper.
type helperOpts struct {
	// Address is the remote address; empty means harnessRoot.
	Address string
	// Args is the argv after the executable; nil means {"origin", Address},
	// git's own shape for a push or fetch. --set-head passes
	// {"--set-head", Address, "main"}.
	Args []string
	// Block is the shim's block rules (testcli.BlockEnv), checked before
	// anything starts.
	Block string
	// Env holds extra KEY=VALUE entries for the helper, applied last.
	Env []string
	// OwnGroup starts the helper in a process group of its own (Unix Setpgid,
	// Windows CREATE_NEW_PROCESS_GROUP), so a case can signal the group
	// without signalling the test.
	OwnGroup bool
	// RawStderr gives the helper a raw stderr pipe that nobody reads
	// (helperRun.StderrR, StderrW) instead of collecting it.
	RawStderr bool
	// GitDir is the local repository's git dir; empty means a new repo with
	// one commit on main (newGitRepoWithCommit).
	GitDir string
	// Production keeps the production shutdown timings. Otherwise the helper
	// gets GPB_TEST_SHRINK=1 (helperRole).
	Production bool
}

// helperRun is one running helper.
//
// StdoutR is the raw read end of the helper's stdout pipe, made with os.Pipe
// and owned by the test, which can close it to break the helper's stdout;
// Stdout reads it (pushMain does, from its goroutine). With RawStderr, the
// helper's stderr is a raw pipe: the test holds the read end, StderrR, and
// never reads it, and keeps its own copy of the write end, StderrW, so it can
// fill the pipe itself; Stderr is then nil. Otherwise Stderr collects stderr,
// and StderrR and StderrW are nil. Tree is the shim's tree, Ready its ready
// dir.
type helperRun struct {
	Cmd              *exec.Cmd
	Stdin            io.WriteCloser
	StdoutR          *os.File
	Stdout           *bufio.Reader
	Stderr           *syncBuffer
	StderrR, StderrW *os.File
	Tree, Ready      string

	exited     chan struct{} // closed once Cmd.Wait has returned
	stderrPipe *os.File      // the collected stderr's read end; nil with RawStderr
	stderrDone chan struct{} // closed once the collected stderr reaches EOF; nil with RawStderr
	pushDone   chan struct{} // closed once pushMain's goroutine returns; nil before pushMain

	mu      sync.Mutex
	replies []string // every line pushMain read, blank lines included
	pushErr error    // what ended pushMain's exchange early, if anything
}

// startHelper starts the helper for opts and registers its teardown. It copies
// this test binary to a temp dir as git-remote-proton-testhelper(.exe), puts
// the shim dir first on PATH (buildShimOnPath), creates the /my-files mount in
// the shim's tree, and runs the helper in the git repo's work tree with GIT_DIR
// set in the helper's own environment, as git sets it: run() reads GIT_DIR
// itself. That is the test setting its child's environment; the helper still
// sets no cmd.Env on its own children.
func startHelper(t *testing.T, opts helperOpts) *helperRun {
	t.Helper()
	if err := testcli.CheckBlockRules(opts.Block); err != nil {
		t.Fatalf("helperOpts.Block: %v", err)
	}
	tree := buildShimOnPath(t)
	// The mount every address is under: EnsureParents refuses a push without it.
	if err := os.MkdirAll(testcli.LocalPath(tree, "/my-files"), 0o755); err != nil {
		t.Fatalf("create the /my-files mount in the shim tree: %v", err)
	}
	ready := t.TempDir()

	gitDir, workDir := opts.GitDir, opts.GitDir
	switch {
	case gitDir == "":
		workDir = newGitRepoWithCommit(t)
		gitDir = filepath.Join(workDir, ".git")
	case filepath.Base(gitDir) == ".git":
		workDir = filepath.Dir(gitDir)
	}
	addr := opts.Address
	if addr == "" {
		addr = harnessRoot
	}
	args := opts.Args
	if args == nil {
		args = []string{"origin", addr}
	}

	cmd := exec.Command(copyTestBinary(t, t.TempDir(), helperExeBase), args...)
	cmd.Dir = workDir
	// The variables this harness decides, whatever the test process carries.
	env := withoutKeys(os.Environ(), append([]string{"GIT_DIR", testcli.BlockEnv, testcli.ReadyDirEnv,
		shrinkEnv, holdAfterLoopEnv, unconfirmedStopEnv, createParentsEnv}, objectPlacementEnv...))
	env = append(env, "GIT_DIR="+gitDir, testcli.BlockEnv+"="+opts.Block, testcli.ReadyDirEnv+"="+ready)
	if !opts.Production {
		env = append(env, shrinkEnv+"=1")
	}
	cmd.Env = append(env, opts.Env...)
	if opts.OwnGroup {
		setOwnProcessGroup(cmd)
	}

	h := &helperRun{Cmd: cmd, Tree: tree, Ready: ready, exited: make(chan struct{})}
	var err error
	if h.Stdin, err = cmd.StdinPipe(); err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	h.StdoutR, h.Stdout = outR, bufio.NewReader(outR)
	cmd.Stdout = outW
	childEnds := []*os.File{outW} // the helper's ends: ours close once it has them
	if opts.RawStderr {
		if h.StderrR, h.StderrW, err = os.Pipe(); err != nil {
			t.Fatalf("stderr pipe: %v", err)
		}
		cmd.Stderr = h.StderrW
	} else {
		errR, errW, err := os.Pipe()
		if err != nil {
			t.Fatalf("stderr pipe: %v", err)
		}
		h.Stderr, h.stderrPipe, cmd.Stderr = &syncBuffer{}, errR, errW
		childEnds = append(childEnds, errW)
	}
	if err := cmd.Start(); err != nil {
		for _, f := range append(childEnds, outR, h.stderrPipe, h.StderrR, h.StderrW) {
			if f != nil {
				f.Close()
			}
		}
		h.Stdin.Close()
		t.Fatalf("start the helper: %v", err)
	}
	for _, f := range childEnds {
		f.Close()
	}
	// The one Wait. Every std stream is an *os.File, so it returns when the
	// process exits, whatever still holds a pipe.
	go func() {
		cmd.Wait()
		close(h.exited)
	}()
	if h.Stderr != nil {
		h.stderrDone = make(chan struct{})
		go func() {
			defer close(h.stderrDone)
			io.Copy(h.Stderr, h.stderrPipe)
		}()
	}
	t.Cleanup(func() { h.teardown(t) })
	return h
}

// withoutKeys is env without the entries for keys.
func withoutKeys(env, keys []string) []string {
	var out []string
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		drop := false
		for _, key := range keys {
			if strings.EqualFold(k, key) {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, kv)
		}
	}
	return out
}

// teardown ends whatever the case left running: the helper by its own handle,
// then every block and late writer through testcli.StopFile (not by pid: a
// recorded pid that has died may be reused by an unrelated process), then the
// harness's goroutines by closing the test's pipe ends. A process or goroutine
// still there at its bound is reported, not waited on forever.
func (h *helperRun) teardown(t *testing.T) {
	select {
	case <-h.exited:
	default:
		h.Cmd.Process.Kill()
		select {
		case <-h.exited:
		case <-time.After(10 * time.Second):
			t.Errorf("teardown: the helper (pid %d) had not exited 10s after its kill", h.Cmd.Process.Pid)
		}
	}
	if t.Failed() {
		t.Logf("helper stderr:\n%s\nprotocol replies read: %q (exchange error: %v)",
			h.stderrText(), h.pushReplies(), h.pushError())
	}
	if err := os.WriteFile(filepath.Join(h.Ready, testcli.StopFile), nil, 0o644); err != nil {
		t.Errorf("teardown: cannot write the shim's stop file: %v", err)
	}
	if left := waitShimsGone(t, h.Tree, func(shimPid) time.Duration { return 10 * time.Second }); len(left) > 0 {
		t.Errorf("teardown: shim processes still running 10s after the stop file: %v", left)
	}
	// Closed from goroutines nobody waits on: on Windows a Close can wait
	// behind a Read still in progress.
	for _, f := range []io.Closer{h.Stdin, h.StdoutR} {
		go f.Close()
	}
	for _, f := range []*os.File{h.stderrPipe, h.StderrR, h.StderrW} {
		if f != nil {
			go f.Close()
		}
	}
	for _, done := range []chan struct{}{h.stderrDone, h.pushDone} {
		if done == nil {
			continue
		}
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Errorf("teardown: a harness goroutine had not returned 10s after its pipes closed")
		}
	}
}

// stderrText is what the helper has written to stderr so far.
func (h *helperRun) stderrText() string {
	if h.Stderr == nil {
		return "(raw stderr: not collected)"
	}
	return h.Stderr.String()
}

// pushMain runs a push of main the way git does, from a goroutine: it sends
// capabilities, list for-push, and the one-line push batch with its blank
// line, reading each response up to its blank line. Then, as git's
// disconnect_helper does once it has the statuses, it sends a blank line and
// closes stdin. A response cut short (the helper died, or the test closed a
// pipe) ends it early, recorded for pushReplies and pushError. Nothing else
// may read Stdout while it runs.
func (h *helperRun) pushMain(t *testing.T) {
	t.Helper()
	if h.pushDone != nil {
		t.Fatal("pushMain: already called for this helper")
	}
	done := make(chan struct{})
	h.pushDone = done
	go func() {
		defer close(done)
		for _, req := range []string{"capabilities\n", "list for-push\n", "push refs/heads/main:refs/heads/main\n\n"} {
			if _, err := io.WriteString(h.Stdin, req); err != nil {
				h.notePushError(fmt.Errorf("send %q: %w", req, err))
				return
			}
			if err := h.readReply(); err != nil {
				h.notePushError(fmt.Errorf("reply to %q: %w", req, err))
				return
			}
		}
		if _, err := io.WriteString(h.Stdin, "\n"); err != nil {
			h.notePushError(fmt.Errorf("send the final blank line: %w", err))
			return
		}
		if err := h.Stdin.Close(); err != nil {
			h.notePushError(fmt.Errorf("close stdin: %w", err))
		}
	}()
}

// readReply reads one response, up to and including its blank line.
func (h *helperRun) readReply() error {
	for {
		line, err := h.Stdout.ReadString('\n')
		if line != "" {
			h.mu.Lock()
			h.replies = append(h.replies, line)
			h.mu.Unlock()
		}
		if err != nil {
			return err
		}
		if line == "\n" {
			return nil
		}
	}
}

func (h *helperRun) notePushError(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pushErr = err
}

// pushReplies is every line pushMain has read so far, blank lines included.
func (h *helperRun) pushReplies() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.replies...)
}

// pushError is what ended pushMain's exchange early, or nil.
func (h *helperRun) pushError() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.pushErr
}

// waitReady waits up to within for a file whose name starts with prefix to
// appear in dir (a shim's ready file, testcli.BlockEnv), and returns its path.
func waitReady(t *testing.T, dir, prefix string, within time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(within)
	for {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("waitReady: %v", err)
		}
		var names []string
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), prefix) {
				return filepath.Join(dir, e.Name())
			}
			names = append(names, e.Name())
		}
		if time.Now().After(deadline) {
			t.Fatalf("waitReady: no %s* in %s after %v (it holds %q)", prefix, dir, within, names)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// wait waits up to within for the helper to exit, and returns its exit code.
// It fails the test if the helper is still running at the deadline, or died
// by a signal rather than exiting.
func (h *helperRun) wait(t *testing.T, within time.Duration) (exitCode int) {
	t.Helper()
	select {
	case <-h.exited:
	case <-time.After(within):
		t.Fatalf("the helper had not exited %v after the wait began; stderr so far:\n%s", within, h.stderrText())
	}
	// The exit closes the helper's end; what it wrote before is still being
	// copied. Nothing else holds the pipe, so this is short.
	if h.stderrDone != nil {
		select {
		case <-h.stderrDone:
		case <-time.After(5 * time.Second):
			t.Errorf("the helper's stderr had not reached EOF 5s after its exit")
		}
	}
	st := h.Cmd.ProcessState
	if st == nil {
		t.Fatalf("the helper's wait ended with no process state; stderr:\n%s", h.stderrText())
	}
	if st.ExitCode() == -1 {
		t.Fatalf("the helper died by a signal (%s) instead of exiting; stderr:\n%s", st, h.stderrText())
	}
	return st.ExitCode()
}

// shimPid is one pid recorded under <tree>/.pids (testcli.PidsDir).
type shimPid struct {
	file  string // the record's name
	pid   int
	delay time.Duration // a late writer's delay; 0 for a plain shim
}

func (p shimPid) String() string { return fmt.Sprintf("%s (pid %d)", p.file, p.pid) }

// recordedShims reads every pid recorded under <tree>/.pids.
func recordedShims(t *testing.T, tree string) []shimPid {
	t.Helper()
	dir := filepath.Join(tree, testcli.PidsDir)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var pids []shimPid
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".tmp-") {
			continue // a record being written
		}
		p := shimPid{file: name}
		num, late := strings.CutPrefix(name, testcli.LatePidPrefix)
		if p.pid, err = strconv.Atoi(num); err != nil {
			t.Fatalf("unexpected file %s in %s", name, dir)
		}
		if late {
			b, err := os.ReadFile(filepath.Join(dir, name))
			if os.IsNotExist(err) {
				continue // the late writer has just finished
			}
			ms, cerr := strconv.Atoi(strings.TrimSpace(string(b)))
			if err != nil || cerr != nil {
				t.Fatalf("late writer record %s: %q, %v", name, b, err)
			}
			p.delay = time.Duration(ms) * time.Millisecond
		}
		pids = append(pids, p)
	}
	return pids
}

// waitShimsGone polls every 100 ms until every recorded pid reads as gone, and
// returns those still alive when their own bound (limit) has passed.
func waitShimsGone(t *testing.T, tree string, limit func(shimPid) time.Duration) []shimPid {
	t.Helper()
	start := time.Now()
	pids := recordedShims(t, tree)
	for {
		var alive, overdue []shimPid
		for _, p := range pids {
			if processAlive(p.pid) {
				alive = append(alive, p)
				if time.Since(start) > limit(p) {
					overdue = append(overdue, p)
				}
			}
		}
		if len(alive) == 0 || len(overdue) > 0 {
			return overdue
		}
		pids = alive
		time.Sleep(100 * time.Millisecond)
	}
}

// assertNoShimLeft fails the test unless every shim recorded under
// <tree>/.pids reads as gone within 5 s, and every late writer within its
// delay plus 5 s. Polling absorbs a killed shim that is still a zombie, which
// kill(pid, 0) reports as alive until it is reaped.
func assertNoShimLeft(t *testing.T, tree string) {
	t.Helper()
	left := waitShimsGone(t, tree, func(p shimPid) time.Duration { return p.delay + 5*time.Second })
	if len(left) > 0 {
		t.Errorf("shim processes still running: %v", left)
	}
}

// lockPresent reports whether root's .lock is in the shim's tree.
func lockPresent(tree, root string) bool {
	_, err := os.Stat(testcli.LocalPath(tree, root+"/"+repo.LockName))
	return err == nil
}
