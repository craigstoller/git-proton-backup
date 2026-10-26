package transport

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// waitDelay bounds how long run may block draining the child's output pipe
// AFTER the child itself has already exited. It is not a command timeout.
//
// MECHANISM. The child's stdout and stderr are one os.Pipe, and EOF on its read
// end requires EVERY holder of the write end to close it — not just the direct
// child. proton-drive is a Node program; a Node process that spawns a worker or
// keeps a helper alive passes the inherited handle to a grandchild, and a
// drain that waits for EOF blocks forever. run therefore reads the pipe on its
// own goroutine, waits for the PROCESS apart from that drain, and gives the
// drain this long to reach EOF once the process has exited; past that it closes
// the read end and uses what it has. (Before run owned the pipe, os/exec's own
// copy goroutine did the draining and cmd.WaitDelay was the bound on it,
// go.dev/issue/23019; the bound is the same, applied to the explicit drain.)
//
// This was OBSERVED during the Stage 2.1 live gate: the first `git push` hung
// past a two-minute timeout AFTER its success output had already been
// captured, with the remote state correct and no leftover .lock — the child
// ran, wrote, exited, and only then did the process fail to exit. Two later
// pushes did not reproduce it, which fits a timing-dependent grandchild.
//
// It is a var, not a const, only so the test can shrink it; nothing in
// production writes to it.
//
// LIMIT, stated plainly: the timer starts when run observes the child has
// exited. A child that never exits at all is NOT bounded by this. That is the
// correct scope — the observed hang was after the child exited, and a real
// command timeout is a separate, unrelated decision.
var waitDelay = 30 * time.Second

// testHookExited is a test seam: when non-nil, run calls it with the child's pid
// once the child's exit is observed and before the drain wait begins (the
// call's in-flight record already shows the exit by then). Nil in production.
// Tests set it before a call and clear it after the call returns, never
// during one.
var testHookExited func(pid int)

// CLI drives the proton-drive executable. Only Exe is set by callers; the
// unexported fields default to today's behaviour when zero: no call timeout,
// drain bounded by waitDelay, warnings to os.Stderr, not a release instance.
// Only NewReleaseCLI (gate.go) sets them.
type CLI struct {
	Exe string

	callTimeout time.Duration // 0: no bound on a single call; else the child is killed at callTimeout - drain
	drain       time.Duration // 0: waitDelay; the post-exit drain bound
	warnW       io.Writer     // nil: os.Stderr; where run's abandoned-pipe warning goes
	release     bool          // exempt from the gate, own process group: the coordinator's instance only
}

func NewCLI(exe string) *CLI {
	if exe == "" {
		exe = "proton-drive"
	}
	return &CLI{Exe: exe}
}

// run starts the CLI and returns its merged stdout+stderr, its exit code and an
// error. It owns the child's output pipe instead of delegating to
// CombinedOutput, so the child's EXIT is observed apart from the DRAIN of its
// output: exit means the process has gone, which is what a caller deciding
// whether the remote is quiet needs to know; the drain can lag it by as long as
// a grandchild holds the pipe open.
//
// Error contract (CombinedOutput's with WaitDelay set, plus the killed case):
//   - the child could not be started: code -1 and the start error, unwrapped
//     (EnforceCertified's never-started test reads it as it is);
//   - the child exited 0 and the drain finished: nil error;
//   - the child exited 0 but the drain ran out: the output read so far, code 0
//     and exec.ErrWaitDelay (the command SUCCEEDED; every caller branches on
//     code != 0 first, so this flows through as the success it is), plus a
//     warning on stderr so a recurrence is diagnosable instead of invisible —
//     stderr, never stdout: stdout is protocol-only;
//   - the child exited non-zero: an *exec.ExitError, whether or not the drain
//     finished, and no warning;
//   - the child ran and the helper killed it (a release call at its bound or
//     at the shutdown cap, a gated call stopped by the shutdown): the child's
//     code (-1 on Unix, where it died of a signal; 1 on Windows, the code
//     Kill's TerminateProcess gives) and an error naming why, "proton CLI call
//     killed at its 15s bound: signal: killed", wrapping the *exec.ExitError.
//
// The gate (gate.go) wraps all of that. A gated call made after ShutGate
// starts no child and parks, and so does one whose result arrives after it; a
// release call returns errReleaseClosed once KillReleaseCalls has run. Each
// call is in an in-flight set from before its start to after its drain, and
// its record shows the child's exit the moment Wait returns, so StopInFlight
// and KillReleaseCalls wait on the exit, never on the drain.
func (c *CLI) run(args ...string) (string, int, error) {
	// The gate comes first: a call it parks creates no pipe and starts no child.
	f, park, err := admit(c.release, c.callTimeout)
	if park != nil {
		return parked(park)
	}
	if err != nil {
		return "", -1, err
	}

	// Bounds the post-exit pipe drain so a grandchild holding the inherited
	// write end cannot hang the helper forever. See waitDelay: DO NOT DELETE
	// this as unnecessary — the hang it prevents is invisible when it happens.
	limit := c.drain
	if limit <= 0 {
		limit = waitDelay
	}

	cmd := exec.Command(c.Exe, args...)
	if c.release {
		setOwnGroup(cmd)
	}
	// ONE pipe for both streams: handing the same *os.File to Stdout and Stderr
	// makes os/exec give the child one handle for both, so the output keeps the
	// interleaving the child wrote it in (what CombinedOutput did).
	pr, pw, err := os.Pipe()
	if err != nil {
		return f.noChild(err)
	}
	cmd.Stdout, cmd.Stderr = pw, pw
	// A bounded call is killed at callTimeout - limit, counted from here, so
	// the drain that follows the kill still ends inside callTimeout.
	killAt := time.Now().Add(c.callTimeout - limit)
	// No lock is held across the start (gate.go).
	if err := startCmd(cmd); err != nil {
		pr.Close()
		pw.Close()
		return f.noChild(err)
	}
	// The child has its own copy now. Ours must go, or EOF could never arrive.
	pw.Close()
	out := readPipe(pr)
	f.fill(cmd.Process)
	var bound *time.Timer
	if c.callTimeout > 0 {
		bound = time.AfterFunc(time.Until(killAt), f.killAtBound)
	}

	// With stdio all *os.File there are no copy goroutines inside os/exec, so
	// Wait returns when the PROCESS exits, whatever still holds the pipe.
	werr := cmd.Wait()
	killedBy := f.exit()
	if bound != nil {
		bound.Stop() // a callback already running finds the exit recorded and does nothing
	}
	if testHookExited != nil {
		testHookExited(cmd.Process.Pid)
	}

	text, drained := out.finish(limit)
	// The drain closed the read end on both its paths (at EOF, or abandoned
	// from a goroutine), so a call that parks here leaks no reader.
	if park := f.leave(); park != nil {
		return parked(park)
	}

	err = werr
	if werr != nil && killedBy != "" {
		// The child ran and we ended it. Said so, so a caller's message does not
		// read as the CLI's own failure, or, with Unix's -1 for a signalled
		// child, as a CLI that never started. %w keeps the *exec.ExitError.
		err = fmt.Errorf("proton CLI call killed %s: %w", killedBy, werr)
	}
	if !drained && werr == nil {
		// The command itself SUCCEEDED: only the drain ran out. This is the
		// exec.ErrWaitDelay carve-out documented above, now produced here.
		err = exec.ErrWaitDelay
		w := c.warnW
		if w == nil {
			w = os.Stderr
		}
		fmt.Fprintf(w, "git-remote-proton: %s exited but something still held its "+
			"output pipe open after %s; the pipe was abandoned and the command's result used "+
			"as-is (output may be truncated)\n", c.Exe, limit)
	}
	// If Wait never produced a ProcessState, ExitCode() would panic. Fail
	// closed instead: report a non-zero code and the error.
	if cmd.ProcessState == nil {
		return text, -1, err
	}
	return text, cmd.ProcessState.ExitCode(), err
}

// pipeReader drains a child's merged output on its own goroutine while the
// child runs: a child that writes more than a pipe buffer blocks until someone
// reads. mu guards buf against the copy finish takes when the drain runs out
// and the goroutine is still appending.
type pipeReader struct {
	pr   *os.File
	mu   sync.Mutex
	buf  []byte
	done chan struct{} // closed when the reader stops
}

func readPipe(pr *os.File) *pipeReader {
	r := &pipeReader{pr: pr, done: make(chan struct{})}
	go func() {
		defer close(r.done)
		chunk := make([]byte, 32*1024)
		for {
			n, rerr := pr.Read(chunk)
			if n > 0 {
				r.mu.Lock()
				r.buf = append(r.buf, chunk[:n]...)
				r.mu.Unlock()
			}
			if rerr != nil {
				return // io.EOF, or the read end closed by finish's timeout
			}
		}
	}()
	return r
}

// finish gives the drain up to limit to reach EOF, once the child has exited,
// and returns what was read and whether the drain finished. The read end is
// closed either way.
func (r *pipeReader) finish(limit time.Duration) (string, bool) {
	timer := time.NewTimer(limit)
	defer timer.Stop()
	drained := false
	select {
	case <-r.done:
		drained = true
		r.pr.Close()
	case <-timer.C:
		// Abandon the pipe. Close from a goroutine nobody waits on: on Windows
		// Close can block behind a Read that is still in progress, and that
		// Read ends only when the grandchild lets go.
		go r.pr.Close()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return string(r.buf), drained
}

// nodeWire mirrors the CLI payload. activeRevision is json.RawMessage because
// its shape differs by version and must be decoded in two attempts.
type nodeWire struct {
	Name struct {
		Value string `json:"value"`
	} `json:"name"`
	Type           string          `json:"type"`
	ActiveRevision json.RawMessage `json:"activeRevision"`
}

// ClaimedSize is a pointer, not a plain int64, for the same reason
// transferSummary's count fields are (below): a JSON key that is ABSENT
// must decode as "missing", never silently default to the zero value — an
// absent claimedSize alongside a valid state must not be confused with a
// genuine "claimedSize": 0 (Task 1.4, 0.8.0 cert plan).
type revWire struct {
	State       string `json:"state"`
	ClaimedSize *int64 `json:"claimedSize"`
}

func parseNodeJSON(b []byte) (Node, error) {
	var w nodeWire
	if err := json.Unmarshal(b, &w); err != nil {
		return Node{}, err
	}
	// C9 pins these two values, identical in `info --json` and `list --json`.
	// A third value means the CLI changed under us: guessing would make every
	// folder read as a file, and ListRefs would then try to read each
	// subdirectory as a ref file. Fail closed and name what we saw.
	var isDir bool
	switch w.Type {
	case "folder":
		isDir = true
	case "file":
		isDir = false
	default:
		return Node{}, fmt.Errorf("unrecognised node type %q (expected \"folder\" or \"file\"); "+
			"the CLI may have changed - the transport contract is certified against %s",
			w.Type, CertifiedCLI)
	}
	n := Node{Name: w.Name.Value, IsDir: isDir}
	if !isDir {
		// type:"file": default to the -1 size-unknown sentinel unless a
		// revision parse below genuinely resolves a claimed size. Before
		// Task 1.4 (0.8.0 cert plan) this fell through to Go's zero value
		// (0) on every parse-miss shape (activeRevision absent, present but
		// claimedSize missing, or claimedSize wrong-typed), indistinguishable
		// from a genuine zero-byte file and making the size-unknown arm
		// (refs.go's readRefClassified band gate, push.go's matching
		// heal-arm gate — both already branch on size<0) UNREACHABLE in
		// production. Folders are untouched: the CLI never gives them an
		// activeRevision at all, and Size has never meant anything for a
		// folder, so this default is scoped to files only.
		n.Size = -1
	}
	if len(w.ActiveRevision) > 0 {
		var r revWire
		// 0.7.0: unwrapped.
		if err := json.Unmarshal(w.ActiveRevision, &r); err == nil && r.State != "" {
			if r.ClaimedSize != nil {
				n.Size = *r.ClaimedSize
			}
			// else: state parsed but claimedSize itself was absent from the
			// JSON — leave the -1 sentinel rather than read a missing field
			// as a genuine zero.
			return n, nil
		}
		// 0.4.6: {ok, value}.
		var wrap struct {
			OK    bool    `json:"ok"`
			Value revWire `json:"value"`
		}
		if err := json.Unmarshal(w.ActiveRevision, &wrap); err == nil && wrap.OK {
			if wrap.Value.ClaimedSize != nil {
				n.Size = *wrap.Value.ClaimedSize
			}
		}
	}
	return n, nil
}

// notFoundSignature is the certified CLI's confirmed-absence text for
// `filesystem info`, pinned live at the Stage 4 gate (docs/research/gates/
// stage3b-gate.md, stage4-gate.md: "Node not found: <leaf>"). It is a
// hypothesis constant, not a guarantee: Stat's not-found/error split below
// depends entirely on this string staying accurate for the certified build,
// which is exactly what contract_test.go's live row pins against reality at
// the gate.
const notFoundSignature = "Node not found"

func (c *CLI) Stat(p string) (Node, bool, error) {
	out, code, err := c.run("filesystem", "info", p, "--json")
	if code == -1 && !ran(err) {
		// code -1 with no *exec.ExitError: the CLI executable itself never
		// started (not on PATH, permission denied, ...), or the gate started no
		// child. That is a transport failure, not a confirmed absence.
		// Folding it into (_, false, nil) would silently read "the CLI is
		// broken" as "this path does not exist" — fail-open, not
		// fail-closed. A CLI that actually ran and reported not-found still
		// returns (_, false, nil); that part of the contract is unchanged. A
		// child that ran and died of a signal also has code -1 on Unix (a
		// release call killed at its bound, say), but it carries an ExitError:
		// it is a failed call, reported below with run's reason, and never
		// "did not start", which would send a person to check their PATH.
		return Node{}, false, fmt.Errorf("proton CLI did not start: %w", err)
	}
	if code != 0 {
		// Absence is (_, false, nil) ONLY on the certified CLI's own
		// not-found signature. Every other nonzero exit is a transport
		// failure and must return an error — this is the Stage 4 gate 2b
		// masquerade fix: the old blanket "any nonzero exit is absence" read
		// a BROKEN CLI (e.g. under GPB_UNCERTIFIED_CLI=1) as "not a
		// git-remote-proton repo" instead of surfacing the real failure.
		if strings.Contains(out, notFoundSignature) {
			return Node{}, false, nil // the certified CLI's confirmed-absence signature
		}
		// Preserve the underlying error per the transport convention List and
		// EnsureDir already follow: dropping c.run's err here would break the
		// %w chain a caller might inspect.
		return Node{}, false, failedCall("info "+p, strings.TrimSpace(bound(out, 200)), err)
	}
	n, err := parseNodeJSON([]byte(out))
	if err != nil {
		return Node{}, false, fmt.Errorf("unparseable info for %s: %w", p, err)
	}
	return n, true, nil
}

// ran reports whether err came from a child that started: a non-zero exit and
// a kill both carry the child's *exec.ExitError, and a start failure (an
// *exec.Error, a PathError, the gate's own refusal) never does. It is the
// partition EnforceCertified's never-started test uses.
func ran(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr)
}

// failedCall is the error for a CLI call that did not exit 0: what failed, the
// CLI's output as the caller trimmed it, and run's error. An empty output is
// left out rather than quoted as nothing: a killed call usually wrote none,
// and "trash p failed: : signal: killed" reads as a fault in the message.
func failedCall(what, out string, err error) error {
	switch {
	case out != "" && err != nil:
		return fmt.Errorf("%s failed: %s: %w", what, out, err)
	case out != "":
		return fmt.Errorf("%s failed: %s", what, out)
	case err != nil:
		return fmt.Errorf("%s failed: %w", what, err)
	}
	return fmt.Errorf("%s failed", what)
}

func (c *CLI) List(p string) ([]Node, error) {
	out, code, err := c.run("filesystem", "list", p, "--json")
	if code != 0 {
		if err != nil {
			return nil, fmt.Errorf("list %s failed: %s: %w", p, strings.TrimSpace(out), err)
		}
		return nil, fmt.Errorf("list %s failed: %s", p, strings.TrimSpace(out))
	}
	if strings.TrimSpace(out) == "" {
		// Defensive fallback, not the normal empty-folder path: on
		// cli-drive@0.7.0 an empty listing is NOT empty output. It is
		// `[\r\n\r\n]\r\n` (8 bytes, exit 0), which survives TrimSpace and
		// parses as a zero-element array via the json.Unmarshal path below
		// (Stage 1 C10). This branch only guards a payload that is truly
		// empty, which C10 did not observe but which would otherwise fail
		// JSON parsing.
		return []Node{}, nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("unparseable listing for %s: %w", p, err)
	}
	nodes := make([]Node, 0, len(raw))
	for _, r := range raw {
		n, err := parseNodeJSON(r)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

// ReadTo ENFORCES the documented "localDir must already exist" contract by
// stat-ing it before ever invoking the CLI. The underlying `proton-drive
// filesystem download` binary does NOT enforce this itself — confirmed live
// (Stage 3a gate, task 7): given a missing destination it silently creates
// the directory and succeeds, contradicting both the Transport interface
// comment and the contract test as they stood before this fix. Rather than
// loosen the contract to match the CLI binary, the wrapper closes the gap:
// every caller in this codebase creates its temp dir with os.MkdirTemp
// before calling ReadTo, so a missing or non-directory localDir here always
// indicates a caller bug, and silently creating a directory nobody asked for
// would paper over exactly that bug instead of surfacing it.
func (c *CLI) ReadTo(p, localDir string) error {
	if st, err := os.Stat(localDir); err != nil {
		return fmt.Errorf("download destination %s: %w", localDir, err)
	} else if !st.IsDir() {
		return fmt.Errorf("download destination %s is not a directory", localDir)
	}
	out, code, err := c.run("filesystem", "download", p, localDir)
	if code != 0 {
		return failedCall("download "+p, strings.TrimSpace(out), err)
	}
	return nil
}

// alreadyExistsSignature is a HYPOTHESIS about the certified CLI's
// create-folder-on-an-existing-folder wording, keyed on the same C17
// observation as EnsureDir's contradiction re-observation below: seen ONCE
// live, never reproduced under deliberate provocation (C17b:
// docs/research/probes/c17b-provocation-log.md). The helper-role tests in
// cli_test.go pin the CODE PATH this constant drives; only a live contract
// row (contract_test.go) can pin the constant's actual VALUE against the
// certified CLI's real wording — this is generic robustness against a
// hypothesised check-then-create race, never claimable as a validated live
// fix on the strength of the hermetic tests alone.
const alreadyExistsSignature = "already exists"

// EnsureDir is Stat-then-create: create-folder exits 1 on an existing folder
// (Stage 1 C5). Swallowing that error generically would also hide real
// permission and path failures, so the existence check is explicit.
//
// The initial Stat branches on node TYPE (Task 9b, round-1 Codex BLOCKER
// fix): before this fix, EnsureDir returned nil for ANY existing node
// without ever reading Node.IsDir, so a ref FILE already at p read as a
// usable folder and the reverse-D/F failure surfaced later, elsewhere, with
// a wrong diagnostic. repo.ensureRefParents' own reverse-D/F detection
// depends on this being TYPED via a fresh Stat call on failure, never on
// error-text matching (its own doc comment says so) — silently returning nil
// here for a file would defeat that by never producing a failure to detect
// in the first place. ok && n.IsDir -> nil (already a usable folder);
// ok && !n.IsDir -> refuse, naming the conflict (exact wording is free: nothing
// in this codebase parses this string); absent -> fall through to create.
func (c *CLI) EnsureDir(p string) error {
	if n, ok, err := c.Stat(p); err != nil {
		return err
	} else if ok {
		if n.IsDir {
			return nil
		}
		return fmt.Errorf("cannot use %s as a folder: a file occupies that name", p)
	}
	i := strings.LastIndex(p, "/")
	if i <= 0 {
		return fmt.Errorf("refusing to create a root-level folder: %s", p)
	}
	parent, name := p[:i], p[i+1:]
	out, code, err := c.run("filesystem", "create-folder", parent, name)
	if code == 0 {
		return nil
	}
	trimmed := strings.TrimSpace(out)
	if strings.Contains(trimmed, alreadyExistsSignature) {
		// The C17 contradiction: the Stat above JUST reported p absent, yet
		// create-folder now claims it already exists. Re-observe once before
		// trusting either observation — see reobserveEnsureDirContradiction.
		return c.reobserveEnsureDirContradiction(p, trimmed)
	}
	if err != nil {
		return fmt.Errorf("create-folder %s in %s failed: %s: %w", name, parent, trimmed, err)
	}
	return fmt.Errorf("create-folder %s in %s failed: %s", name, parent, trimmed)
}

// reobserveEnsureDirContradiction is the C17 contradiction's second half
// (Task 9b, design §2d): it re-observes p via a RAW
// c.run("filesystem","info",p,"--json") call — deliberately NOT the Stat
// wrapper. Stat (above) deliberately discards the CLI's raw output on the
// not-found path, keeping only (_, false, nil); the whole point here is to
// quote BOTH the create-folder output and this info output VERBATIM if the
// contradiction cannot be resolved, and the Stat wrapper cannot supply that
// second verbatim observation.
//
// createOut is create-folder's own trimmed output, already captured by the
// caller; it is quoted here, never re-derived.
//
// Classification, in order: code == -1 with no *exec.ExitError (mirrors Stat's
// own never-started guard, above; a child killed by a signal also has -1 on
// Unix, but it ran, and falls through to the undetermined arm) -> the CLI
// executable itself never started for THIS re-observation call, a distinct
// transport failure, never folded into "not found" or
// "undetermined" (review round 4, M5 — the original version discarded the
// exit code entirely and would have misdiagnosed this as one of those two);
// code == 0 AND the raw output parses as a folder node -> resolved, proceed
// as if the folder was there all along (nil); code == 0 AND parses as a
// file node -> the reverse-D/F refusal, named, not a silent proceed; the
// JSON parse is gated on code == 0 throughout — a nonzero exit is never fed
// to parseNodeJSON, matching Stat's own contract that a successful parse
// only ever happens on a confirmed-successful call; carries
// notFoundSignature -> the C17 signature itself, GENERIC ROBUSTNESS ONLY
// (see alreadyExistsSignature's doc comment: observed once live, never
// reproduced under provocation — C17b's ruling stands, this can never be
// claimed as a validated live fix) — error quoting both raw observations
// verbatim so a genuine recurrence stays diagnosable; anything else -> error
// quoting both as undetermined.
func (c *CLI) reobserveEnsureDirContradiction(p, createOut string) error {
	infoOut, infoCode, infoErr := c.run("filesystem", "info", p, "--json")
	if infoCode == -1 && !ran(infoErr) {
		return fmt.Errorf("contradiction creating folder %s could not be re-observed: the "+
			"Proton CLI did not start for the follow-up info call: %v (original create-folder "+
			"output: %q)", p, infoErr, bound(createOut, 200))
	}
	if infoCode == 0 {
		if n, perr := parseNodeJSON([]byte(infoOut)); perr == nil {
			if n.IsDir {
				return nil // resolved: a folder genuinely is there now
			}
			return fmt.Errorf("cannot use %s as a folder: a file occupies that name "+
				"(create-folder reported %q; a follow-up info call confirms a file)",
				p, bound(createOut, 200))
		}
	}
	trimmedInfo := strings.TrimSpace(infoOut)
	if strings.Contains(trimmedInfo, notFoundSignature) {
		return fmt.Errorf("contradiction creating folder %s: create-folder reported %q, but a "+
			"follow-up info call reports not-found: %q — this may be a benign check-then-create "+
			"race on the CLI's own side, or something removed the node between the two calls; "+
			"refusing to guess which (generic robustness against a hypothesised race, not a "+
			"validated live fix — see docs/research/probes/c17b-provocation-log.md)",
			p, bound(createOut, 200), bound(trimmedInfo, 200))
	}
	return fmt.Errorf("contradiction creating folder %s could not be resolved: create-folder "+
		"reported %q; a follow-up info call reported %q (undetermined)",
		p, bound(createOut, 200), bound(trimmedInfo, 200))
}

// transferSummary mirrors an `upload --json` response. Fields are pointers,
// not plain ints: a JSON key that is absent (a renamed field on a future
// CLI, or any unexpected shape) must decode as "missing", never silently
// default to the zero value. Design: "A missing count field is Ambiguous,
// never defaulted to zero — a renamed field in a future CLI must fail
// loudly rather than silently read as 'nothing happened.'"
type transferSummary struct {
	Transferred *int `json:"transferredItems"`
	Skipped     *int `json:"skippedItems"`
	Failed      *int `json:"failedItems"`
}

// classifyUpload turns an exact, mutually exclusive count tuple into an
// Outcome. Tuples are exact, not >=: for a single-file operation the CLI
// records exactly one of success, skip, or failure, so e.g. (1,1,0) is
// contradictory and Ambiguous. Exit code cannot distinguish success from
// refusal — both are 0 — so this never looks at exit status.
func classifyUpload(transferred, skipped, failed int) Outcome {
	switch {
	case transferred == 1 && skipped == 0 && failed == 0:
		return Committed
	case transferred == 0 && skipped == 1 && failed == 0:
		return Refused
	default:
		return Ambiguous
	}
}

// parseTransferSummary decodes an upload --json body. Missing-field
// detection lives here, in the decoding step, not in classifyUpload: an
// absent count field must be caught before it ever reaches classifyUpload,
// so a renamed or dropped field can never be silently read as a confident
// zero — it fails loudly as Ambiguous instead.
func parseTransferSummary(out string) (Outcome, error) {
	var s transferSummary
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &s); err != nil {
		// Unparseable output is Ambiguous, never assumed-failed: the write may
		// have landed. Callers reconcile by reading remote state.
		return Ambiguous, fmt.Errorf("unparseable upload summary: %s", strings.TrimSpace(out))
	}
	if s.Transferred == nil || s.Skipped == nil || s.Failed == nil {
		return Ambiguous, fmt.Errorf("upload summary missing a count field: %s", strings.TrimSpace(out))
	}
	return classifyUpload(*s.Transferred, *s.Skipped, *s.Failed), nil
}

func (c *CLI) upload(strategy, remoteDir, localFile string) (Outcome, error) {
	out, code, err := c.run("filesystem", "upload", "-f", strategy, "--json", localFile, remoteDir)
	outcome, perr := parseTransferSummary(out)
	if perr == nil {
		// Deliberately NOT gated on exit status: 0 covers both a transfer and
		// a skip, so the counts are the only thing that can classify a
		// successful upload (see classifyUpload). A parseable summary is
		// authoritative regardless of code.
		return outcome, nil
	}
	// No summary to read, so the exit code and start error are the only
	// evidence left of what happened. This method used to discard both
	// (`out, _, _ := c.run(...)`) — the one CLI method that did — and in the
	// never-started case out is empty, so the caller saw "unparseable upload
	// summary: " with the real cause gone. Every other method here wraps err
	// with %w; this now does too.
	if err != nil {
		return outcome, fmt.Errorf("%w (upload exited %d: %w)", perr, code, err)
	}
	return outcome, fmt.Errorf("%w (upload exited %d)", perr, code)
}

// CreateExclusive and UpdateRevision both pass only dirOf(p) to the CLI,
// because `filesystem upload` takes a PARENT path and has no --name flag: the
// remote node is named after localFile's OWN basename (probe C11). CALLER
// CONTRACT: filepath.Base(localFile) MUST equal the leaf of p, or the write
// lands under the wrong remote name. repo.stagedFile is what guarantees it,
// and checkUploadBasename (below) is the second, mechanical line of defence:
// Task 2 review round 1 found that only the Fake enforced this contract — the
// CLI passed localFile straight through, so a caller that violated it against
// the live CLI would silently write to the wrong remote name, which is
// exactly the C11 failure class this whole check exists to prevent.
func (c *CLI) CreateExclusive(p, localFile string) (Outcome, error) {
	if err := checkUploadBasename(p, localFile); err != nil {
		return Ambiguous, err
	}
	return c.upload("skip", dirOf(p), localFile)
}

// UpdateRevision maps to `create-new-revision` (0.8.0 vocabulary; Task 1
// step 3, 0.8.0 cert plan — was `merge` under 0.7.0's general
// conflict-strategy option, which 0.8.0 removed in favour of separate
// -f/-d file/folder strategies). `create-new-revision`'s help text ("Create
// a new revision of the existing file") matches the same intent `merge`
// served: it revises the existing node in place (Stage 1 C1: node uid
// stable, revision uid changes) and upserts a node that does not yet exist
// (Stage 1 C8: exit 0, transferred=1), so no prior existence check is
// needed for correctness. Whether 0.8.0's REMOTE semantics under the new
// name match C1/C8 exactly is a hypothesis Task 2's live invariants
// re-verify (same node uid, no duplicate) — this is a value rename on our
// side, not yet a live-reverified behaviour. NOT `replace`, which trashes
// the node before creating the new one and can destroy a ref on a crash —
// present in both 0.7.0's and 0.8.0's vocabularies, but never the value we
// pass, for the same never-touch-foreign-data reason as before. Subject to
// the same caller contract as CreateExclusive, above, and checked the same
// way: localFile's basename must equal p's leaf (probe C11), or the
// revision lands under the wrong remote name.
func (c *CLI) UpdateRevision(p, localFile string) (Outcome, error) {
	if err := checkUploadBasename(p, localFile); err != nil {
		return Ambiguous, err
	}
	return c.upload("create-new-revision", dirOf(p), localFile)
}

// trashItem mirrors one element of a `trash --json` response: a JSON array
// of {uid, ok} objects (Stage 1 C3) — a different shape from the upload
// transfer summary. Feeding this through the upload parser would silently
// read every count field as missing.
type trashItem struct {
	UID string `json:"uid"`
	OK  bool   `json:"ok"`
}

// parseTrashResult decodes a trash --json body. The delete is Committed
// only when the response affirmatively confirms it: exactly one item,
// ok:true. Anything else — ok:false, zero or multiple items, or
// unparseable JSON — is Ambiguous. Success is never inferred from exit
// status alone (design error table: a failure reported in the body "with
// exit code 0" is "treated as failure — never inferred from exit status").
func parseTrashResult(out string) (Outcome, error) {
	var results []trashItem
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &results); err != nil {
		return Ambiguous, fmt.Errorf("unparseable trash result: %s", strings.TrimSpace(out))
	}
	if len(results) != 1 || !results[0].OK {
		return Ambiguous, fmt.Errorf("trash not affirmatively confirmed: %s", strings.TrimSpace(out))
	}
	return Committed, nil
}

// Trash exits 1 on a missing target (Stage 1 C4), so absence is checked
// first and reported as Committed — the desired end state is "not there".
func (c *CLI) Trash(p string) (Outcome, error) {
	if _, ok, err := c.Stat(p); err != nil {
		return Ambiguous, err
	} else if !ok {
		return Committed, nil
	}
	out, code, err := c.run("filesystem", "trash", p, "--json")
	if code != 0 {
		return Ambiguous, failedCall("trash "+p, strings.TrimSpace(out), err)
	}
	return parseTrashResult(out)
}

func dirOf(p string) string {
	if i := strings.LastIndex(p, "/"); i > 0 {
		return p[:i]
	}
	return "/"
}

// CertifiedCLI is the exact build this helper is certified against. It has
// moved twice since Stage 4, each time under a fresh live certification
// (gate records under docs/research/gates/):
//
//   - 0.7.0 → 0.8.0 on 2026-08-13: Proton stopped hosting the 0.7.0
//     download the day 0.8.0 shipped, so every new install got 0.8.0; and
//     0.8.0 split the general upload conflict-strategy option into -f/-d,
//     renaming C1's "merge" to "create-new-revision" for files (see
//     UpdateRevision).
//   - 0.8.0 → 0.9.0 on 2026-10-10 (cli-0.9.0-certification.md and
//     v0.9.0-release-gate.md): Proton shipped 0.9.0 on 2026-10-05 and the
//     download page advertised only it, so every new install met a build
//     the 0.8.0-certified helper refused. NO vocabulary change this time —
//     the 0.9.0 audit confirmed the same four -f values — so the move is
//     the token alone.
//
// Support is an allowlist, not a floor: successive builds have differed in
// the activeRevision payload shape, in whether a byte-identical rewrite is
// skipped, and in the upload conflict-strategy vocabulary, so a floor would
// admit a build that breaks verification silently. Every formerly certified
// token REFUSES under this constant (exact-version philosophy,
// Craig-adjudicated since Stage 2); 0.8.0 gets the generic override
// warning, while 0.7.0 — the one formerly certified build that speaks the
// pre-split vocabulary — gets the specific note legacyCertifiedCLI070
// drives.
const CertifiedCLI = "cli-drive@0.9.0+75612299"

// legacyCertifiedCLI070 is the exact token of the 0.7.0 build — certified
// from Stage 1 through v0.6.0, and the only FORMERLY CERTIFIED build that
// speaks the pre-split conflict-strategy vocabulary (every pre-0.8.0 build
// does; only 0.7.0 was ever certified). It is kept as its own named
// constant (never inferred from history) so
// EnforceCertified's override path can name the SPECIFIC incompatibility
// when it recognises this exact build, rather than only ever printing the
// generic "UNCERTIFIED" warning every mismatched version gets (round-2
// [Gemini] override de-trap — see EnforceCertified, below).
const legacyCertifiedCLI070 = "cli-drive@0.7.0+5174900c"

// identityToken extracts the certified-build token from --version output,
// if a proper identity line is present. Two shapes are recognised: a BARE
// token (the whole line is nothing but the token — a test/helper
// convenience already pinned by TestIsCertifiedMatchesTheRealVersionLine),
// or the real CLI's own "Proton Drive CLI <token>" line, where the token is
// the field immediately after "CLI". A token that merely APPEARS somewhere
// inside a longer, unrelated line — an update nag mentioning an old or new
// version in passing, say — is neither shape and is NOT an identity line:
// it must never certify on that strength alone (round-3 GUARD b). Multiple
// lines are tolerated in any order or count — hardening is about WHICH LINE
// structurally qualifies, never how many lines there are or where the
// qualifying one sits, so an update-nag line appended before or after the
// real identity line changes nothing (round-3 GUARD a).
func identityToken(versionOutput string) (string, bool) {
	for _, line := range strings.Split(versionOutput, "\n") {
		f := strings.Fields(line)
		n := len(f)
		if n == 0 {
			continue
		}
		last := f[n-1]
		if !strings.HasPrefix(last, "cli-drive@") {
			continue
		}
		if n == 1 || f[n-2] == "CLI" {
			return last, true
		}
	}
	return "", false
}

// IsCertified reports whether --version output names EXACTLY the certified
// build, via identityToken's structural identity-line parse (above).
// Exact-token equality, never containment: identityToken's found field must
// EQUAL CertifiedCLI. Containment was a prefix match by another name — it
// accepted the certified token with an "-extra" suffix — and "exact
// versions, not a floor or prefix" is the design's rule. Output with no qualifying identity
// line is simply not certified (the unparseable case).
func IsCertified(versionOutput string) bool {
	tok, ok := identityToken(versionOutput)
	return ok && tok == CertifiedCLI
}

// Version returns the full trimmed output of `proton-drive --version`
// (both the CLI and SDK lines), not just the first line: the SDK line
// belongs in diagnostics even though EnforceCertified's allowlist match
// only ever looks at the CLI line's token (see IsCertified).
func (c *CLI) Version() (string, error) {
	out, code, err := c.run("--version")
	if code != 0 {
		if err != nil {
			return "", fmt.Errorf("proton-drive --version failed: %s: %w", strings.TrimSpace(out), err)
		}
		return "", fmt.Errorf("proton-drive --version failed: %s", strings.TrimSpace(out))
	}
	return strings.TrimSpace(out), nil
}

// EnforceCertified is the Stage 4 allowlist: the design's "refuse to run"
// rule, enforced. nil means proceed — either the CLI reports the certified
// build, or allowUncertified is set and the loud warning was written to w.
// w must be non-nil: the override path writes to it unconditionally, and
// with two callers now (run and runSetHead) that contract is no longer
// visible from a single call site.
// A binary that never STARTED refuses regardless of the override (the
// override does not synthesize a binary; the spawn failure is the report).
// The check is a compatibility gate against accidental drift, not a
// provenance check: a spoofed --version defeats it trivially, and a helper
// that trusts the CLI with every byte of repo data has no defence against a
// malicious binary anyway (spec, Decisions).
//
// Never-started is detected as "verr is non-nil and is not an
// *exec.ExitError", not narrowly as exec.ErrNotFound (Task 3 fix round 1:
// the ErrNotFound-only check let every OTHER never-started case — permission
// denied, bad executable format, CLI.Exe pointing at a directory — fall
// through to the generic branch and PROCEED under the override, exactly the
// synthesis this doc comment says must not happen). Version()'s errors
// partition cleanly on this test: a process that ran and exited non-zero
// always surfaces *exec.ExitError (version undetermined, override applies —
// the CLI is real, we just could not read a certified line from it), while a
// process that never started surfaces the raw start error instead
// (*exec.Error from LookPath, *os.PathError/*fs.PathError from Start — never
// an ExitError), and a WaitDelay-only hiccup never reaches here as an error
// at all (TestVersionSucceedsThroughAWaitDelay pins that a zero exit
// carrying ErrWaitDelay reads as success).
func EnforceCertified(c *CLI, allowUncertified bool, w io.Writer) error {
	v, verr := c.Version()
	if verr == nil && IsCertified(v) {
		return nil
	}
	var exitErr *exec.ExitError
	if verr != nil && !errors.As(verr, &exitErr) {
		return fmt.Errorf("Proton CLI could not be started: %w", verr)
	}
	// Quoted diagnostics are BOUNDED: --version output is remote-tool
	// output, and an error message must not become a channel for megabytes.
	found := fmt.Sprintf("%q", bound(v, 200))
	if verr != nil {
		found = fmt.Sprintf("could not be determined (%v)", verr)
	}
	if allowUncertified {
		fmt.Fprintf(w, "git-remote-proton: WARNING: proceeding with an UNCERTIFIED "+
			"Proton CLI because GPB_UNCERTIFIED_CLI=1. Version %s; certified: %s. "+
			"Behaviour on this build is unvalidated.\n", found, CertifiedCLI)
		// De-trapped (round-2 [Gemini]): the override still opens the escape
		// hatch unconditionally, but when the DETECTED build is specifically
		// the known-incompatible 0.7.0, name that incompatibility instead of
		// letting the operator find out later, opaquely, mid-push — 0.8.0's
		// upload conflict-strategy vocabulary (create-new-revision) is not
		// one 0.7.0 speaks. A selective hard-ban was considered and
		// rejected: override means override; the cure for the trap is
		// candor, not a second gate. Scoped to an exact match — an
		// unrelated mismatched version gets only the generic warning above,
		// never a false claim about 0.7.0's vocabulary.
		if tok, ok := identityToken(v); ok && tok == legacyCertifiedCLI070 {
			fmt.Fprintf(w, "git-remote-proton: NOTE: %s is a formerly certified build (0.7.0, "+
				"certified through v0.6.0) — its update path speaks the OLD conflict-strategy "+
				"vocabulary, not %s's; pushes that update an existing ref will fail against it. "+
				"The override does not make %s viable for pushes.\n", legacyCertifiedCLI070, CertifiedCLI, legacyCertifiedCLI070)
		}
		return nil
	}
	msg := fmt.Sprintf("Proton CLI version %s, but this build is certified only "+
		"against %s; refusing to run. Set GPB_UNCERTIFIED_CLI=1 to proceed anyway "+
		"(unvalidated), or install the certified CLI", found, CertifiedCLI)
	// A distribution's repackage (cli-drive-debian@...) is refused like any
	// other mismatch; it only gets one more sentence saying so (spec D4(a)).
	// The "." closes the generic text above, which has no final stop of its own
	// and must stay byte-identical for an ordinary mismatch.
	if tok, ok := distroToken(v); ok {
		msg += fmt.Sprintf(". This looks like a distribution's repackaged build (%q); "+
			"install Proton's official build of the certified version instead "+
			"(see the README's install section; Proton's downloads are at "+
			"https://proton.me/download/drive/cli/).", tok)
	}
	return errors.New(msg)
}

// distroTokenRE matches a distribution-repackaged identity token: the real
// product token with a distro name spliced after "cli-drive" (Debian-style
// packaging prints cli-drive-debian@...). Lowercase letters, digits, dots and
// hyphens only, starting with a letter or digit, then "@" and a version.
var distroTokenRE = regexp.MustCompile(`^cli-drive-[a-z0-9][a-z0-9.-]*@\S+$`)

// distroToken reports a distribution-repackaged identity token in --version
// output, for ONE purpose: letting EnforceCertified's refusal say so. It
// accepts exactly the two line shapes identityToken does (the line is nothing
// but the token, or the token is the field right after "CLI"), so a token
// merely mentioned inside a longer line never qualifies. It confers nothing:
// identityToken and IsCertified do not call it, and a repackaged build is
// uncertified like any other mismatch (spec D4(a)). The token is bounded
// because it is remote-tool output headed for an error message.
func distroToken(versionOutput string) (string, bool) {
	for _, line := range strings.Split(versionOutput, "\n") {
		f := strings.Fields(line)
		n := len(f)
		if n == 0 {
			continue
		}
		last := f[n-1]
		if !distroTokenRE.MatchString(last) {
			continue
		}
		if n == 1 || f[n-2] == "CLI" {
			return bound(last, 80), true
		}
	}
	return "", false
}

// bound truncates s for quoting in diagnostics.
func bound(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…(truncated)"
}
