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
package testcli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// TreeEnv names the environment variable a shim process reads to find the
// local directory tree it serves. Every Proton Drive path (always POSIX,
// always leading "/") maps to a file under this tree via LocalPath — e.g.
// "/my-files/r/refs/heads/main" maps to <tree>/my-files/r/refs/heads/main.
const TreeEnv = "GPB_TESTCLI_TREE"

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
// (CreateExclusive -> "skip", UpdateRevision -> "merge"; Stage 1 C1/C8):
// "skip" refuses (skippedItems=1) onto an existing target and writes
// (transferredItems=1) onto an absent one; "merge" always upserts
// (transferredItems=1), matching C8's "UPSERTS" finding. C2's byte-
// identical-rewrite auto-skip is deliberately NOT reproduced: nothing in
// this codebase's write paths ever re-uploads identical bytes to the same
// target (every write is preceded by either a fresh random nonce (the lock
// body) or content that only gets written once (a ref, HEAD)), so scripting
// that edge case would add fidelity risk with no caller ever exercising it.
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
	case "merge":
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
