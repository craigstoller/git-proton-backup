// Package testcli is a scripted stand-in for the real `proton-drive` CLI
// binary, built to be re-exec'd as a subprocess (the same os.Executable()
// re-exec technique internal/transport/cli_test.go already uses for its
// TestHelperProcess roles). Unlike those roles, which each answer one fixed
// canned response, this package serves the six verbs a caller actually
// invokes it with (--version, filesystem info/list/create-folder/upload/
// download/trash) against a REAL local directory tree, so that argv parsing,
// the allowlist check (transport.IsCertified), transport construction, and
// the repo-level callers (RequireMarker, AcquireLock, SetHead, ...) all run
// through their genuine code paths end to end, hermetically, with no live
// Proton account involved.
//
// Task 13 (Stage 5, STRETCH): this exists solely to let one hermetic test
// (cmd/git-remote-proton's shim_test.go) drive the REAL runSetHead — not the
// dispatchUtility test seam (runSetHeadFn) — because runSetHead constructs
// its own live *transport.CLI and there was previously no way to satisfy
// that construction without a real proton-drive binary on PATH.
//
// Fidelity is intentionally narrow: this is not a general Proton Drive
// simulator. It mirrors exactly the JSON shapes internal/transport/cli.go's
// parser expects (docs/research/probes/stage1-results.json's C3, C9, C10
// findings — the parser is the real contract; the probe file is reference)
// and exactly the argv shapes cli.go's own c.run(...) call sites construct
// (Version, Stat, List, EnsureDir, upload, ReadTo, Trash) — not the full
// grammar the real CLI accepts. A caller that constructs `proton-drive`
// arguments any other way is out of scope.
//
// The signal tests (cmd/git-remote-proton's signal_*_test.go) also script the
// shim's timing with block rules (BlockEnv): a call that waits to be killed, a
// call that fails, and a server that applies an upload late, after the shim
// has died. Every run records its pid under the tree (PidsDir), so a test can
// check that no shim outlived it.
package testcli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// TreeEnv names the environment variable a shim process reads to find the
// local directory tree it serves. Every Proton Drive path (always POSIX,
// always leading "/") maps to a file under this tree via LocalPath — e.g.
// "/my-files/r/refs/heads/main" maps to <tree>/my-files/r/refs/heads/main.
const TreeEnv = "GPB_TESTCLI_TREE"

// BlockEnv holds the block rules, separated by ";" (empty fields ignored).
// Each rule is
//
//	verb:leaf:once|every:before|after[:delay=<ms>][:ignoreterm][:late=<ms>][:fail]
//
// where:
//
//   - verb is upload, download, trash, info, create-folder or --version.
//   - leaf is the name the call is matched on, "*" for any, or "*<suffix>"
//     ("*.pack"). For upload it is filepath.Base of the LOCAL file argument,
//     because the CLI names the remote node after the local basename (probe
//     C11, as runUpload does). For create-folder it is the name argument; for
//     download, trash and info, leafOf of the remote path. --version has no
//     path: its name is "version".
//   - once fires the rule at most once per matched name, across processes: the
//     first run to create <ready>/once-<tag> (O_EXCL) takes it. every fires it
//     on each matching call.
//   - before acts before the shim looks at the target at all: before its
//     existence check, its skip decision or any write, so an upload that would
//     be skipped (Refused) still blocks first. after acts once the verb has
//     done its work on the target, whatever it found (written, skipped,
//     missing), and before the shim prints its result.
//
// At most one rule applies to a call: the first, in order, whose verb and leaf
// match and, for once, whose marker this run creates. At its point the rule
// writes <ready>/ready-<tag> holding the shim's pid, then blocks until the
// shim is killed. <tag> is <verb>-<name>-<n>: the MATCHED name with every
// character outside [A-Za-z0-9._-] replaced by "_", and n the rule's 0-based
// index, so it never holds the rule's own "*" or ":", which Windows does not
// allow in file names. The options change what happens at the point:
//
//   - delay=<ms> sleeps that long instead of blocking, then carries on.
//   - ignoreterm ignores SIGTERM from just before the ready file until the
//     block or delay ends (Unix; nothing sends a Windows process SIGTERM).
//   - fail writes the ready file and exits 1 at once, with nothing on stdout
//     (so no summary JSON) and a one-line note on stderr: no block, no delay.
//     With after, the target was acted on first.
//   - late=<ms> (upload only), before anything else the rule does: copies the
//     staged file to <ready>/late-<n>.data, because AcquireLock deletes its
//     staging folder as soon as the upload call returns, and starts a late
//     writer, a detached copy of the shim (its own process group, not waited
//     on, so a kill of the shim's group cannot reach it). It waits until this
//     shim has gone, then sleeps <ms>, then writes the copy to the upload's
//     target: a server applying a request after the client died. Started
//     before the ready file, so a test that sees the ready file knows the late
//     writer is recorded.
//
// A block ends early, with exit 1, when the harness creates <ready>/StopFile
// at teardown; so does a late writer's wait. Each is also bounded by
// blockLimit, so nothing the shim starts outlives a dead test for long.
const BlockEnv = "GPB_TESTCLI_BLOCK"

// ReadyDirEnv names the folder the block rules write their ready files, once
// markers and late-writer data in. Required when BlockEnv holds a rule.
const ReadyDirEnv = "GPB_TESTCLI_READY_DIR"

// StopFile, created in the ready dir, ends every block and late-writer wait.
const StopFile = "stop"

// PidsDir is the folder under the tree where every shim run records its pid,
// as <tree>/.pids/<pid>, removed when the run returns. A run killed mid-call
// leaves its file. A late writer is recorded only as LatePidPrefix+<pid>,
// holding its delay in milliseconds, and removes it when it returns. The
// folder sits outside every remote path (those all start /my-files or
// /devices), so it never reads as remote content.
const (
	PidsDir       = ".pids"
	LatePidPrefix = "late-"
)

// blockLimit bounds a block and a late writer's wait for the shim's death. A
// test kills or stops them long before; this only ends what a dead test left.
const blockLimit = 10 * time.Minute

// lateWriterRole is the first argument the shim starts its late writer with.
// No real CLI invocation starts with it.
const lateWriterRole = "testcli-late-writer"

// sdkLine is appended after the certified CLI line on --version, mirroring
// the genuine two-line captured shape (stage1-results.json's "cli" field is
// the first line only; the SDK line is the second line internal/transport's
// own TestMain roles already fabricate the same way — see cli_test.go's
// runVersionRole). transport.Version() keeps the whole trimmed output, and
// EnforceCertified's IsCertified check only ever inspects the first line's
// token, so the exact SDK version named here carries no behavioural weight.
const sdkLine = "Proton Drive SDK js@0.20.0+5174900c"

// Run dispatches one invocation of the shim, given the argv the real
// proton-drive binary would have received (args[0] is the first argument
// AFTER the executable name — e.g. "--version", or "filesystem"). It reads
// TreeEnv itself, rather than taking the tree as a parameter, because the
// caller of Run is always a freshly re-exec'd subprocess whose only channel
// for that information is its own environment.
//
// It records the run's pid (PidsDir) and applies the block rules (BlockEnv)
// around the verb. A call no rule matches runs exactly as before.
func Run(args []string, stdout, stderr io.Writer) int {
	tree := os.Getenv(TreeEnv)
	if tree == "" {
		fmt.Fprintf(stderr, "testcli: %s is not set\n", TreeEnv)
		return 1
	}
	if len(args) == 0 {
		fmt.Fprintln(stderr, "testcli: no arguments given")
		return 1
	}
	if args[0] == lateWriterRole {
		return runLateWriter(tree, args[1:], stderr)
	}
	pidFile, err := recordPid(tree, strconv.Itoa(os.Getpid()), "")
	if err != nil {
		fmt.Fprintf(stderr, "testcli: cannot record this run's pid: %v\n", err)
		return 1
	}
	defer os.Remove(pidFile)

	rules, err := parseRules(os.Getenv(BlockEnv))
	if err != nil {
		fmt.Fprintf(stderr, "testcli: %s: %v\n", BlockEnv, err)
		return 1
	}
	ready := os.Getenv(ReadyDirEnv)
	r, err := claimRule(rules, ready, args)
	if err != nil {
		fmt.Fprintf(stderr, "testcli: %v\n", err)
		return 1
	}
	if r == nil {
		return dispatch(tree, args, stdout, stderr)
	}
	if r.late >= 0 {
		if err := startLateWriter(tree, ready, r, args); err != nil {
			fmt.Fprintf(stderr, "testcli: rule %d: cannot start the late writer: %v\n", r.n, err)
			return 1
		}
	}
	if !r.after {
		if code, exit := r.hit(ready, stderr); exit {
			return code
		}
	}
	// Held back until an after rule has had its turn: the result is what the
	// caller must not see before the block ends.
	var out, errOut bytes.Buffer
	code := dispatch(tree, args, &out, &errOut)
	if r.after {
		if code, exit := r.hit(ready, stderr); exit {
			return code
		}
	}
	stdout.Write(out.Bytes())
	stderr.Write(errOut.Bytes())
	return code
}

// dispatch runs one verb, with no block rule involved.
func dispatch(tree string, args []string, stdout, stderr io.Writer) int {
	switch args[0] {
	case "--version":
		// The certified line's exact wording ("Proton Drive CLI " prefix,
		// then the bare build token) mirrors stage1-results.json's "cli"
		// field and is what transport.IsCertified's field-scan must find.
		fmt.Fprintln(stdout, "Proton Drive CLI "+transport.CertifiedCLI)
		fmt.Fprintln(stdout, sdkLine)
		return 0
	case "filesystem":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "testcli: filesystem requires a verb")
			return 1
		}
		return runFilesystem(tree, args[1], args[2:], stdout, stderr)
	}
	fmt.Fprintf(stderr, "testcli: unrecognised command %q\n", args[0])
	return 1
}

func runFilesystem(tree, verb string, args []string, stdout, stderr io.Writer) int {
	switch verb {
	case "info":
		return runInfo(tree, args, stdout, stderr)
	case "list":
		return runList(tree, args, stdout, stderr)
	case "create-folder":
		return runCreateFolder(tree, args, stderr)
	case "upload":
		return runUpload(tree, args, stdout, stderr)
	case "download":
		return runDownload(tree, args, stderr)
	case "trash":
		return runTrash(tree, args, stdout, stderr)
	}
	fmt.Fprintf(stderr, "testcli: unrecognised filesystem verb %q\n", verb)
	return 1
}

// LocalPath maps a Proton Drive remote path (always "/"-separated, always
// absolute) onto a file under tree, translating separators for the host
// platform. Exported so a test can seed the tree with the same mapping the
// shim itself uses to read it back, rather than duplicating the rule.
func LocalPath(tree, remote string) string {
	remote = strings.TrimPrefix(remote, "/")
	return filepath.Join(tree, filepath.FromSlash(remote))
}

// leafOf returns the last "/"-separated component of a remote path — the
// same shape the certified CLI's own not-found text names (Stage 4 gate:
// "Node not found: <leaf>", never the full path).
func leafOf(remote string) string {
	remote = strings.TrimRight(remote, "/")
	if i := strings.LastIndex(remote, "/"); i >= 0 {
		return remote[i+1:]
	}
	return remote
}

// notFound writes the certified CLI's confirmed-absence signature
// (cli.go's notFoundSignature: "Node not found") so the real Stat/Trash/
// ReadTo parsing logic classifies it exactly the way it classifies the real
// binary's output.
func notFound(w io.Writer, remote string) {
	fmt.Fprintf(w, "Node not found: %s\n", leafOf(remote))
}

// nodeName and nodeOut mirror cli.go's nodeWire/parseNodeJSON exactly,
// including the "name":{"value":...} wrapper (probe C9/C10) and the 0.7.0
// UNWRAPPED activeRevision shape (parseNodeJSON tries unwrapped first).
// ActiveRevision is a pointer so folders — which carry none (C9: "Folders
// carry no activeRevision") — omit the key entirely rather than emitting a
// zero-valued one parseNodeJSON would still (harmlessly) accept.
type nodeName struct {
	Value string `json:"value"`
}

type activeRevision struct {
	State       string `json:"state"`
	ClaimedSize int64  `json:"claimedSize"`
}

type nodeOut struct {
	Name           nodeName        `json:"name"`
	Type           string          `json:"type"`
	ActiveRevision *activeRevision `json:"activeRevision,omitempty"`
}

func nodeFor(name string, isDir bool, size int64) nodeOut {
	n := nodeOut{Name: nodeName{Value: name}}
	if isDir {
		n.Type = "folder"
		return n
	}
	n.Type = "file"
	n.ActiveRevision = &activeRevision{State: "active", ClaimedSize: size}
	return n
}

func writeJSONLine(w io.Writer, v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		// Unreachable for the fixed shapes this package ever marshals; a
		// failure here would be a testcli bug, not a shim-of-CLI behaviour
		// worth scripting, so there's no separate error path for it.
		panic(fmt.Sprintf("testcli: marshal: %v", err))
	}
	w.Write(b)
	fmt.Fprintln(w)
}

// runInfo mirrors *CLI.Stat's call site: c.run("filesystem", "info", p,
// "--json"). args is [path, "--json"].
func runInfo(tree string, args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "testcli: info requires a path")
		return 1
	}
	remote := args[0]
	fi, err := os.Stat(LocalPath(tree, remote))
	if err != nil {
		notFound(stderr, remote)
		return 1
	}
	writeJSONLine(stdout, nodeFor(leafOf(remote), fi.IsDir(), fi.Size()))
	return 0
}

// runList mirrors *CLI.List's call site: c.run("filesystem", "list", p,
// "--json"). args is [path, "--json"].
func runList(tree string, args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "testcli: list requires a path")
		return 1
	}
	remote := args[0]
	entries, err := os.ReadDir(LocalPath(tree, remote))
	if err != nil {
		notFound(stderr, remote)
		return 1
	}
	nodes := make([]nodeOut, 0, len(entries))
	for _, e := range entries {
		var size int64
		if !e.IsDir() {
			if info, ierr := e.Info(); ierr == nil {
				size = info.Size()
			}
		}
		nodes = append(nodes, nodeFor(e.Name(), e.IsDir(), size))
	}
	writeJSONLine(stdout, nodes)
	return 0
}

// runCreateFolder mirrors *CLI.EnsureDir's call site: c.run("filesystem",
// "create-folder", parent, name). args is [parent, name].
func runCreateFolder(tree string, args []string, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "testcli: create-folder requires a parent and a name")
		return 1
	}
	parent, name := args[0], args[1]
	target := filepath.Join(LocalPath(tree, parent), name)
	if _, err := os.Stat(target); err == nil {
		// Stage 1 C5's exact wording.
		fmt.Fprintln(stderr, "A file or folder with that name already exists")
		return 1
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		fmt.Fprintf(stderr, "create-folder failed: %v\n", err)
		return 1
	}
	return 0
}

// transferSummary mirrors cli.go's own transferSummary field names exactly
// (transferredItems/skippedItems/failedItems) — the shape
// parseTransferSummary decodes.
type transferSummary struct {
	Transferred int `json:"transferredItems"`
	Skipped     int `json:"skippedItems"`
	Failed      int `json:"failedItems"`
}

// runUpload mirrors *CLI.upload's call site exactly: c.run("filesystem",
// "upload", "-f", strategy, "--json", localFile, remoteDir). args is
// ["-f", strategy, "--json", localFile, remoteDir].
//
// Strategy handling covers exactly the two strategies cli.go ever passes
// (CreateExclusive -> "skip", UpdateRevision -> "create-new-revision" as of
// the 0.8.0 certification — Task 1 step 3; "merge" under 0.7.0's now-removed
// general conflict-strategy option, Stage 1 C1/C8): "skip" refuses
// (skippedItems=1) onto an existing target and writes (transferredItems=1)
// onto an absent one; "create-new-revision" always upserts
// (transferredItems=1), matching C8's "UPSERTS" finding (the vocabulary
// renamed; the modelled behaviour did not — Task 2's live table re-verifies
// that 0.8.0's real semantics still match). C2's byte- identical-rewrite
// auto-skip is deliberately NOT reproduced: nothing in this codebase's write
// paths ever re-uploads identical bytes to the same target (every write is
// preceded by either a fresh random nonce (the lock body) or content that
// only gets written once (a ref, HEAD)), so scripting that edge case would
// add fidelity risk with no caller ever exercising it.
func runUpload(tree string, args []string, stdout, stderr io.Writer) int {
	if len(args) != 5 || args[0] != "-f" || args[2] != "--json" {
		fmt.Fprintf(stderr, "testcli: unexpected upload arguments %v\n", args)
		return 1
	}
	strategy, localFile, remoteDir := args[1], args[3], args[4]

	data, err := os.ReadFile(localFile)
	if err != nil {
		fmt.Fprintf(stderr, "testcli: cannot read local file %s: %v\n", localFile, err)
		writeJSONLine(stdout, transferSummary{Failed: 1})
		return 1
	}
	targetDir := LocalPath(tree, remoteDir)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "testcli: cannot use remote dir %s: %v\n", remoteDir, err)
		writeJSONLine(stdout, transferSummary{Failed: 1})
		return 1
	}
	target := filepath.Join(targetDir, filepath.Base(localFile))
	_, statErr := os.Stat(target)
	exists := statErr == nil

	switch strategy {
	case "skip":
		if exists {
			writeJSONLine(stdout, transferSummary{Skipped: 1})
			return 0
		}
	case "create-new-revision":
		// upserts unconditionally — falls through to the write below either way.
	default:
		fmt.Fprintf(stderr, "testcli: unsupported upload strategy %q\n", strategy)
		return 1
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		fmt.Fprintf(stderr, "testcli: write failed: %v\n", err)
		writeJSONLine(stdout, transferSummary{Failed: 1})
		return 1
	}
	writeJSONLine(stdout, transferSummary{Transferred: 1})
	return 0
}

// runDownload mirrors *CLI.ReadTo's call site: c.run("filesystem",
// "download", p, localDir). args is [remotePath, localDestDir]. Unlike the
// real CLI (probe C16: it silently CREATES a missing destination), this
// does not create localDestDir — *CLI.ReadTo already stats it and refuses
// before ever invoking the CLI (cli.go's own doc comment), so every caller
// this shim will ever see has already guaranteed the directory exists; a
// missing one here would signal a bug in the caller, not something to paper
// over.
//
// FILE-ORIENTED ONLY, deliberately, and this is a real divergence from the
// certified CLI, not an oversight: os.ReadFile below fails on a directory
// path, so a directory target here misreports as notFound rather than being
// downloaded. That is NOT the real contract — the Stage 5 gate's Finding F1
// established that `filesystem download` on a directory exits 0 and
// recursively downloads the whole subtree, now pinned by
// internal/transport/contract_test.go's "download of a directory recursively
// materialises the subtree (F1)" row and modelled by Fake.ReadTo's own
// directory branch (internal/transport/fake.go). This shim was never
// extended to match, so no test anywhere in this codebase may rely on it
// downloading a directory — doing so would certify a shim behaviour
// (notFound) that contradicts the certified CLI's real one (recursive
// success).
func runDownload(tree string, args []string, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "testcli: download requires a remote path and a destination directory")
		return 1
	}
	remote, dest := args[0], args[1]
	data, err := os.ReadFile(LocalPath(tree, remote))
	if err != nil {
		notFound(stderr, remote)
		return 1
	}
	target := filepath.Join(dest, leafOf(remote))
	if err := os.WriteFile(target, data, 0o644); err != nil {
		fmt.Fprintf(stderr, "testcli: write failed: %v\n", err)
		return 1
	}
	return 0
}

// trashItem mirrors cli.go's own trashItem field names exactly
// (uid/ok) — Stage 1 C3's {uid,ok} shape, a different shape from
// transferSummary.
type trashItem struct {
	UID string `json:"uid"`
	OK  bool   `json:"ok"`
}

// runTrash mirrors *CLI.Trash's call site: c.run("filesystem", "trash", p,
// "--json"). args is [path, "--json"]. *CLI.Trash always Stats first and
// short-circuits to Committed without ever invoking this verb when the
// target is already absent (Stage 1 C4), so in practice this is only ever
// reached with an existing target — the not-found branch below is
// generic robustness, not a path any test here exercises.
func runTrash(tree string, args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "testcli: trash requires a path")
		return 1
	}
	remote := args[0]
	local := LocalPath(tree, remote)
	if _, err := os.Stat(local); err != nil {
		notFound(stderr, remote)
		return 1
	}
	if err := os.RemoveAll(local); err != nil {
		fmt.Fprintf(stderr, "testcli: trash failed: %v\n", err)
		return 1
	}
	writeJSONLine(stdout, []trashItem{{UID: "testcli-" + leafOf(remote), OK: true}})
	return 0
}

// rule is one parsed block rule (BlockEnv). delay and late are -1 when absent.
type rule struct {
	n          int // 0-based index among the rules
	verb, leaf string
	once       bool
	after      bool
	delay      time.Duration
	late       time.Duration
	ignoreTerm bool
	fail       bool

	name string // the name it matched, set by claimRule
}

var ruleVerbs = map[string]bool{
	"upload": true, "download": true, "trash": true, "info": true, "create-folder": true, "--version": true,
}

// CheckBlockRules reports whether s parses as BlockEnv's rules, so a harness
// can refuse a mistyped rule before any shim runs.
func CheckBlockRules(s string) error {
	_, err := parseRules(s)
	return err
}

func parseRules(s string) ([]*rule, error) {
	var rules []*rule
	for _, field := range strings.Split(s, ";") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		r, err := parseRule(field, len(rules))
		if err != nil {
			return nil, fmt.Errorf("rule %q: %w", field, err)
		}
		rules = append(rules, r)
	}
	return rules, nil
}

func parseRule(s string, n int) (*rule, error) {
	p := strings.Split(s, ":")
	if len(p) < 4 {
		return nil, errors.New("want verb:leaf:once|every:before|after[:delay=<ms>][:ignoreterm][:late=<ms>][:fail]")
	}
	r := &rule{n: n, verb: p[0], leaf: p[1], delay: -1, late: -1}
	if !ruleVerbs[r.verb] {
		return nil, fmt.Errorf("unknown verb %q", r.verb)
	}
	if r.leaf == "" || strings.Contains(r.leaf[1:], "*") {
		return nil, fmt.Errorf("leaf %q: want a name, \"*\", or \"*<suffix>\"", r.leaf)
	}
	switch p[2] {
	case "once":
		r.once = true
	case "every":
	default:
		return nil, fmt.Errorf("want once or every, got %q", p[2])
	}
	switch p[3] {
	case "before":
	case "after":
		r.after = true
	default:
		return nil, fmt.Errorf("want before or after, got %q", p[3])
	}
	seen := map[string]bool{}
	for _, opt := range p[4:] {
		key, val, hasVal := strings.Cut(opt, "=")
		if seen[key] {
			return nil, fmt.Errorf("option %q given twice", key)
		}
		seen[key] = true
		var err error
		switch {
		case key == "delay" && hasVal:
			r.delay, err = parseMillis(val)
		case key == "late" && hasVal:
			r.late, err = parseMillis(val)
		case opt == "ignoreterm":
			r.ignoreTerm = true
		case opt == "fail":
			r.fail = true
		default:
			return nil, fmt.Errorf("unknown option %q", opt)
		}
		if err != nil {
			return nil, fmt.Errorf("option %q: %w", opt, err)
		}
	}
	if r.late >= 0 && r.verb != "upload" {
		return nil, errors.New("late applies to upload only")
	}
	if r.fail && (r.delay >= 0 || r.ignoreTerm) {
		return nil, errors.New("fail does not block, so delay and ignoreterm cannot apply")
	}
	return r, nil
}

func parseMillis(s string) (time.Duration, error) {
	ms, err := strconv.Atoi(s)
	if err != nil || ms < 0 {
		return 0, fmt.Errorf("want a whole number of milliseconds, got %q", s)
	}
	return time.Duration(ms) * time.Millisecond, nil
}

func (r *rule) matches(verb, name string) bool {
	if r.verb != verb {
		return false
	}
	if strings.HasPrefix(r.leaf, "*") {
		return strings.HasSuffix(name, r.leaf[1:])
	}
	return r.leaf == name
}

// tag is <verb>-<matched name>-<n>, safe as a file name on every platform.
func (r *rule) tag() string {
	safe := strings.Map(func(c rune) rune {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-' {
			return c
		}
		return '_'
	}, r.name)
	return fmt.Sprintf("%s-%s-%d", r.verb, safe, r.n)
}

// invocation is the verb and the name a rule's leaf is matched on, for the
// argv shapes the dispatchers accept. ok is false for anything else (list, or
// a malformed call), which no rule matches and the dispatchers refuse.
func invocation(args []string) (verb, name string, ok bool) {
	if args[0] == "--version" {
		return "--version", "version", true
	}
	if args[0] != "filesystem" || len(args) < 3 {
		return "", "", false
	}
	verb, rest := args[1], args[2:]
	switch verb {
	case "upload":
		if len(rest) != 5 {
			return "", "", false
		}
		return verb, filepath.Base(rest[3]), true
	case "create-folder":
		if len(rest) < 2 {
			return "", "", false
		}
		return verb, rest[1], true
	case "download", "trash", "info":
		return verb, leafOf(rest[0]), true
	}
	return "", "", false
}

// claimRule returns the rule that applies to this call, or nil.
func claimRule(rules []*rule, ready string, args []string) (*rule, error) {
	if len(rules) == 0 {
		return nil, nil
	}
	verb, name, ok := invocation(args)
	if !ok {
		return nil, nil
	}
	for _, r := range rules {
		if !r.matches(verb, name) {
			continue
		}
		if ready == "" {
			return nil, fmt.Errorf("%s is not set, and rule %d matches", ReadyDirEnv, r.n)
		}
		r.name = name
		if !r.once {
			return r, nil
		}
		f, err := os.OpenFile(filepath.Join(ready, "once-"+r.tag()), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue // another run took it
		}
		if err != nil {
			return nil, fmt.Errorf("rule %d: once marker: %v", r.n, err)
		}
		f.Close()
		return r, nil
	}
	return nil, nil
}

// hit is what r does at its point. exit reports that the run ends here, with
// code; otherwise the call carries on.
func (r *rule) hit(ready string, stderr io.Writer) (code int, exit bool) {
	if r.ignoreTerm {
		signal.Ignore(syscall.SIGTERM)
		defer signal.Reset(syscall.SIGTERM)
	}
	pid := strconv.Itoa(os.Getpid())
	if err := writeAtomic(ready, "ready-"+r.tag(), pid+"\n"); err != nil {
		fmt.Fprintf(stderr, "testcli: rule %d: cannot write its ready file: %v\n", r.n, err)
		return 1, true
	}
	switch {
	case r.fail:
		fmt.Fprintf(stderr, "testcli: rule %d fails this call\n", r.n)
		return 1, true
	case r.delay >= 0:
		if sleepUnlessStopped(ready, r.delay) {
			return 1, true
		}
		return 0, false
	}
	if sleepUnlessStopped(ready, blockLimit) {
		fmt.Fprintf(stderr, "testcli: rule %d: stopped by the harness\n", r.n)
		return 1, true
	}
	fmt.Fprintf(stderr, "testcli: rule %d blocked for %s and was never killed\n", r.n, blockLimit)
	return 1, true
}

// sleepUnlessStopped sleeps d, and reports true if the harness's StopFile
// appeared in ready first.
func sleepUnlessStopped(ready string, d time.Duration) bool {
	end := time.Now().Add(d)
	for {
		if _, err := os.Stat(filepath.Join(ready, StopFile)); err == nil {
			return true
		}
		left := time.Until(end)
		if left <= 0 {
			return false
		}
		time.Sleep(min(left, 50*time.Millisecond))
	}
}

// writeAtomic writes content to dir/name through a temporary name, so a
// reader polling for name never sees it half written.
func writeAtomic(dir, name, content string) error {
	tmp := filepath.Join(dir, ".tmp-"+name+"-"+strconv.Itoa(os.Getpid()))
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, name))
}

// recordPid writes <tree>/.pids/<name> and returns its path.
func recordPid(tree, name, content string) (string, error) {
	dir := filepath.Join(tree, PidsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, name), writeAtomic(dir, name, content)
}

// startLateWriter copies the upload's staged file and starts the late writer
// for it (BlockEnv's late option). args is the whole upload argv.
func startLateWriter(tree, ready string, r *rule, args []string) error {
	localFile, remoteDir := args[5], args[6]
	b, err := os.ReadFile(localFile)
	if err != nil {
		return err
	}
	data := filepath.Join(ready, fmt.Sprintf("late-%d.data", r.n))
	if err := os.WriteFile(data, b, 0o644); err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	target := filepath.Join(LocalPath(tree, remoteDir), filepath.Base(localFile))
	ms := strconv.FormatInt(r.late.Milliseconds(), 10)
	// No stdio: a late writer that held the shim's output pipe would keep the
	// helper's drain waiting after the shim died.
	cmd := exec.Command(self, lateWriterRole, strconv.Itoa(os.Getpid()), ms, data, target)
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	_, err = recordPid(tree, LatePidPrefix+strconv.Itoa(cmd.Process.Pid), ms+"\n")
	cmd.Process.Release() // never waited on
	return err
}

// runLateWriter is the late writer: args are the shim's pid, the delay in
// milliseconds, the data file and the target. It waits until the shim has
// gone, polling every 20 ms, then sleeps the delay, then writes the data to
// the target, so the delay runs from the shim's death, as a late server
// landing would.
func runLateWriter(tree string, args []string, stderr io.Writer) int {
	if len(args) != 4 {
		fmt.Fprintf(stderr, "testcli: late writer: unexpected arguments %v\n", args)
		return 1
	}
	defer os.Remove(filepath.Join(tree, PidsDir, LatePidPrefix+strconv.Itoa(os.Getpid())))
	shim, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "testcli: late writer: shim pid %q: %v\n", args[0], err)
		return 1
	}
	delay, err := parseMillis(args[1])
	if err != nil {
		fmt.Fprintf(stderr, "testcli: late writer: delay: %v\n", err)
		return 1
	}
	data, target := args[2], args[3]
	b, err := os.ReadFile(data)
	if err != nil {
		fmt.Fprintf(stderr, "testcli: late writer: %v\n", err)
		return 1
	}
	ready := os.Getenv(ReadyDirEnv)
	end := time.Now().Add(blockLimit)
	for processAlive(shim) {
		if time.Now().After(end) || sleepUnlessStopped(ready, 20*time.Millisecond) {
			return 1
		}
	}
	if sleepUnlessStopped(ready, delay) {
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		fmt.Fprintf(stderr, "testcli: late writer: %v\n", err)
		return 1
	}
	// Through a temporary name in the ready dir, so a release reading the
	// target never sees it half written.
	tmp := data + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		fmt.Fprintf(stderr, "testcli: late writer: %v\n", err)
		return 1
	}
	if err := os.Rename(tmp, target); err != nil {
		fmt.Fprintf(stderr, "testcli: late writer: %v\n", err)
		return 1
	}
	return 0
}
