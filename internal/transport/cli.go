package transport

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// waitDelay bounds how long Wait may block draining the child's output pipes
// AFTER the child itself has already exited. It is not a command timeout.
//
// MECHANISM (documented os/exec behaviour, not speculation). CombinedOutput
// sets Stdout/Stderr to a *bytes.Buffer, which is not an *os.File, so os/exec
// creates an os.Pipe(), hands the write end to the child, and copies from the
// read end on a goroutine. Wait does not return until that goroutine sees EOF,
// and EOF requires EVERY holder of the write end to close it — not just the
// direct child. proton-drive is a Node program; a Node process that spawns a
// worker or keeps a helper alive passes the inherited handle to a grandchild,
// and we block forever. WaitDelay makes os/exec close the pipe and give up
// instead (go.dev/issue/23019).
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
// LIMIT, stated plainly: with no Context set, the WaitDelay timer starts when
// Wait observes the child has exited. A child that never exits at all is NOT
// bounded by this. That is the correct scope — the observed hang was after the
// child exited, and a real command timeout is a separate, unrelated decision.
var waitDelay = 30 * time.Second

type CLI struct{ Exe string }

func NewCLI(exe string) *CLI {
	if exe == "" {
		exe = "proton-drive"
	}
	return &CLI{Exe: exe}
}

func (c *CLI) run(args ...string) (string, int, error) {
	cmd := exec.Command(c.Exe, args...)
	// Bounds the post-exit pipe drain so a grandchild holding the inherited
	// write end cannot hang the helper forever. See waitDelay: DO NOT DELETE
	// this as unnecessary — the hang it prevents is invisible when it happens.
	cmd.WaitDelay = waitDelay
	out, err := cmd.CombinedOutput()
	if errors.Is(err, exec.ErrWaitDelay) {
		// The command itself SUCCEEDED. os/exec substitutes ErrWaitDelay for a
		// nil error only when the process exited with a successful status
		// (exec.go: `if err == nil { err = goroutineErr }`), and ProcessState
		// is set, so the exit code returned below is the real one and every
		// caller here — all of which branch on code != 0 first — flows through
		// as the success it is. Warn so a recurrence is diagnosable instead of
		// invisible. stderr, never stdout: stdout is protocol-only.
		fmt.Fprintf(os.Stderr, "git-remote-proton: %s exited but something still held its "+
			"output pipe open after %s; the pipe was abandoned and the command's result used "+
			"as-is (output may be truncated)\n", c.Exe, waitDelay)
	}
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
		if err != nil {
			return Node{}, false, fmt.Errorf("info %s failed: %s: %w", p, strings.TrimSpace(bound(out, 200)), err)
		}
		return Node{}, false, fmt.Errorf("info %s failed: %s", p, strings.TrimSpace(bound(out, 200)))
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
		if err != nil {
			return fmt.Errorf("download %s failed: %s: %w", p, strings.TrimSpace(out), err)
		}
		return fmt.Errorf("download %s failed: %s", p, strings.TrimSpace(out))
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
// Classification, in order: code == -1 (mirrors Stat's own code==-1 guard,
// above) -> the CLI executable itself never started for THIS re-observation
// call, a distinct transport failure, never folded into "not found" or
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
	if infoCode == -1 {
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

// CertifiedCLI is the exact build this helper is certified against — moved
// from 0.7.0 to 0.8.0 (Task 1, 0.8.0 certification: Proton stopped hosting
// the 0.7.0 download the day 0.8.0 shipped, so every new install gets
// 0.8.0). Support is an allowlist, not a floor: successive builds have
// differed in the activeRevision payload shape, in whether a byte-identical
// rewrite is skipped, and now in the upload conflict-strategy vocabulary
// (the general strategy option split into -f/-d, C1's "merge" renamed to
// "create-new-revision" for files — see UpdateRevision), so a floor would
// admit a build that breaks verification silently. 0.7.0's own token now
// REFUSES under this constant (exact-version philosophy, Craig-adjudicated
// since Stage 2) — see legacyCertifiedCLI070 for the override path's
// specific handling of that known-incompatible build.
const CertifiedCLI = "cli-drive@0.8.0+06e8c605"

// legacyCertifiedCLI070 is the PREVIOUSLY-certified build's exact token,
// kept as its own named constant (never inferred from history) so
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
// accepted "cli-drive@0.8.0+06e8c605-extra" — and "exact versions, not a
// floor or prefix" is the design's rule. Output with no qualifying identity
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
			fmt.Fprintf(w, "git-remote-proton: NOTE: %s is the previously-certified build — "+
				"its update path speaks the OLD conflict-strategy vocabulary, not %s's; "+
				"pushes that update an existing ref will fail against it. The override does "+
				"not make %s viable for pushes.\n", legacyCertifiedCLI070, CertifiedCLI, legacyCertifiedCLI070)
		}
		return nil
	}
	return fmt.Errorf("Proton CLI version %s, but this build is certified only "+
		"against %s; refusing to run. Set GPB_UNCERTIFIED_CLI=1 to proceed anyway "+
		"(unvalidated), or install the certified CLI", found, CertifiedCLI)
}

// bound truncates s for quoting in diagnostics.
func bound(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…(truncated)"
}
