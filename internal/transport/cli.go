package transport

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type CLI struct{ Exe string }

func NewCLI(exe string) *CLI {
	if exe == "" {
		exe = "proton-drive"
	}
	return &CLI{Exe: exe}
}

func (c *CLI) run(args ...string) (string, int, error) {
	cmd := exec.Command(c.Exe, args...)
	out, err := cmd.CombinedOutput()
	// If the executable never started (not on PATH, permission denied, etc.),
	// cmd.ProcessState is nil and ExitCode() would panic. Fail closed instead:
	// report a non-zero code and the start error.
	if cmd.ProcessState == nil {
		return string(out), -1, err
	}
	code := cmd.ProcessState.ExitCode()
	return string(out), code, err
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

type revWire struct {
	State       string `json:"state"`
	ClaimedSize int64  `json:"claimedSize"`
}

func parseNodeJSON(b []byte) (Node, error) {
	var w nodeWire
	if err := json.Unmarshal(b, &w); err != nil {
		return Node{}, err
	}
	n := Node{Name: w.Name.Value, IsDir: w.Type == "folder"}
	if len(w.ActiveRevision) > 0 {
		var r revWire
		// 0.7.0: unwrapped.
		if err := json.Unmarshal(w.ActiveRevision, &r); err == nil && r.State != "" {
			n.Size = r.ClaimedSize
			return n, nil
		}
		// 0.4.6: {ok, value}.
		var wrap struct {
			OK    bool    `json:"ok"`
			Value revWire `json:"value"`
		}
		if err := json.Unmarshal(w.ActiveRevision, &wrap); err == nil && wrap.OK {
			n.Size = wrap.Value.ClaimedSize
		}
	}
	return n, nil
}

func (c *CLI) Stat(p string) (Node, bool, error) {
	out, code, err := c.run("filesystem", "info", p, "--json")
	if code == -1 {
		// code == -1 only when run's nil-ProcessState guard fired: the CLI
		// executable itself never started (not on PATH, permission denied,
		// ...). That is a transport failure, not a confirmed absence.
		// Folding it into (_, false, nil) would silently read "the CLI is
		// broken" as "this path does not exist" — fail-open, not
		// fail-closed. A CLI that actually ran and reported not-found still
		// returns (_, false, nil); that part of the contract is unchanged.
		return Node{}, false, fmt.Errorf("proton CLI did not start: %w", err)
	}
	if code != 0 {
		return Node{}, false, nil // absence is not an error
	}
	n, err := parseNodeJSON([]byte(out))
	if err != nil {
		return Node{}, false, fmt.Errorf("unparseable info for %s: %w", p, err)
	}
	return n, true, nil
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

func (c *CLI) ReadTo(p, localDir string) error {
	out, code, err := c.run("filesystem", "download", p, localDir)
	if code != 0 {
		if err != nil {
			return fmt.Errorf("download %s failed: %s: %w", p, strings.TrimSpace(out), err)
		}
		return fmt.Errorf("download %s failed: %s", p, strings.TrimSpace(out))
	}
	return nil
}

// EnsureDir is Stat-then-create: create-folder exits 1 on an existing folder
// (Stage 1 C5). Swallowing that error generically would also hide real
// permission and path failures, so the existence check is explicit.
func (c *CLI) EnsureDir(p string) error {
	if _, ok, err := c.Stat(p); err != nil {
		return err
	} else if ok {
		return nil
	}
	i := strings.LastIndex(p, "/")
	if i <= 0 {
		return fmt.Errorf("refusing to create a root-level folder: %s", p)
	}
	parent, name := p[:i], p[i+1:]
	out, code, err := c.run("filesystem", "create-folder", parent, name)
	if code != 0 {
		if err != nil {
			return fmt.Errorf("create-folder %s in %s failed: %s: %w", name, parent, strings.TrimSpace(out), err)
		}
		return fmt.Errorf("create-folder %s in %s failed: %s", name, parent, strings.TrimSpace(out))
	}
	return nil
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
// lands under the wrong remote name. repo.stagedFile is what guarantees it.
func (c *CLI) CreateExclusive(p, localFile string) (Outcome, error) {
	return c.upload("skip", dirOf(p), localFile)
}

// UpdateRevision maps to `merge`, which revises the existing node in place
// (Stage 1 C1: node uid stable, revision uid changes) and upserts a node
// that does not yet exist (Stage 1 C8: exit 0, transferred=1), so no prior
// existence check is needed for correctness. NOT `replace`, which trashes
// the node before creating the new one and can destroy a ref on a crash.
// Subject to the same caller contract as CreateExclusive, above: localFile's
// basename must equal p's leaf (probe C11), or the revision lands under the
// wrong remote name.
func (c *CLI) UpdateRevision(p, localFile string) (Outcome, error) {
	return c.upload("merge", dirOf(p), localFile)
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
		if err != nil {
			return Ambiguous, fmt.Errorf("trash %s failed: %s: %w", p, strings.TrimSpace(out), err)
		}
		return Ambiguous, fmt.Errorf("trash %s failed: %s", p, strings.TrimSpace(out))
	}
	return parseTrashResult(out)
}

func dirOf(p string) string {
	if i := strings.LastIndex(p, "/"); i > 0 {
		return p[:i]
	}
	return "/"
}
