// Package gitcmd wraps git plumbing commands (cat-file, merge-base,
// rev-list, pack-objects) needed by git-remote-proton. All object work is
// done by shelling out to git rather than reimplementing pack formats.
//
// This package is self-contained: it imports nothing from internal/transport
// or internal/repo.
package gitcmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// waitDelay bounds how long Wait may block draining a git child's output pipes
// AFTER git itself has already exited. It is not a command timeout.
//
// MECHANISM (documented os/exec behaviour). Every exec site in this package
// redirects Stdout/Stderr to something that is not an *os.File — a
// *bytes.Buffer via CombinedOutput, or a *strings.Builder in WritePack — so
// os/exec creates an os.Pipe(), hands the write end to git, and copies from
// the read end on a goroutine. Wait does not return until that goroutine sees
// EOF, and EOF requires EVERY holder of the write end to close it, not just
// the direct child. A git invoked with core.fsmonitor enabled spawns a daemon
// that inherits the pipe and outlives the command, so without this the helper
// blocks forever (go.dev/issue/23019). Identical shape to the hang observed
// against the Proton CLI during the Stage 2.1 live gate; see
// internal/transport/cli.go.
//
// LIMIT: with no Context set, the timer starts when Wait observes the child
// has exited. A git that never exits at all is NOT bounded by this.
//
// A var, not a const, purely for symmetry with the transport-side knob;
// nothing in production writes to it.
var waitDelay = 30 * time.Second

// warnWaitDelay reports an abandoned output pipe on stderr — never stdout,
// which is protocol-only. Without it a recurrence of the grandchild-holds-the-
// pipe case is completely invisible.
func warnWaitDelay(what string) {
	fmt.Fprintf(os.Stderr, "git-remote-proton: %s exited but something still held its output "+
		"pipe open after %s; the pipe was abandoned (output may be truncated)\n", what, waitDelay)
}

// git runs `git -C gitDir <args>` and returns its combined stdout+stderr
// (trimmed), its exit code, and any error starting/running the process.
//
// cmd.ProcessState is nil when the process never starts at all (git missing
// from PATH, permission denied, ...), and ExitCode() on a nil ProcessState
// panics. A panic is not failing closed, so that case is guarded explicitly
// and reported as code -1 with the underlying start error preserved — the
// same shape internal/transport/cli.go's run helper uses, for the same
// reason: callers must be able to distinguish "ran and failed" from "never
// ran", and the cause must survive even when combined output is empty.
func git(gitDir string, args ...string) (string, int, error) {
	cmd := exec.Command("git", append([]string{"-C", gitDir}, args...)...)
	// Bounds the post-exit pipe drain so an fsmonitor daemon (or any other
	// grandchild) holding the inherited write end cannot hang the helper
	// forever. See waitDelay: DO NOT DELETE as unnecessary.
	cmd.WaitDelay = waitDelay
	out, err := cmd.CombinedOutput()
	if errors.Is(err, exec.ErrWaitDelay) {
		// git itself SUCCEEDED — os/exec substitutes ErrWaitDelay for a nil
		// error only on a successful exit — so ProcessState is set, the exit
		// code below is the real one, and callers that branch on it first are
		// unaffected. Truncated output is still handled correctly by every
		// caller: ObjectType would see a type that is not "commit",
		// IsAncestor switches on the exit code, and WritePack guards its own
		// rev-list output explicitly (below), so no path silently narrows.
		warnWaitDelay("git " + strings.Join(args, " "))
	}
	if cmd.ProcessState == nil {
		return strings.TrimSpace(string(out)), -1, err
	}
	return strings.TrimSpace(string(out)), cmd.ProcessState.ExitCode(), err
}

// ObjectType resolves the type of sha locally. git hands the helper only
// hashes, so the design's "a branch target must be a commit" rule is
// unenforceable without this: there is no other way to tell a commit from a
// tag or blob before deciding whether to accept a ref update.
func ObjectType(gitDir, sha string) (string, error) {
	out, code, err := git(gitDir, "cat-file", "-t", sha)
	if code != 0 {
		if err != nil {
			return "", fmt.Errorf("cat-file -t %s: %s: %w", sha, out, err)
		}
		return "", fmt.Errorf("cat-file -t %s: %s", sha, out)
	}
	return out, nil
}

// HasObject reports whether sha exists in gitDir's object store.
//
// A false return means "not confirmed present", not "confirmed absent": the
// signature (fixed by the plan; Task 10 is written against it) has no error
// channel, so a tooling failure (git missing from PATH, a corrupt repo, an
// unexpected exit code) is indistinguishable here from a genuine miss. That
// is tolerable specifically because of how HasObject is used: the design's
// object-transfer rule for advertised tips is that objects that cannot be
// confirmed locally are simply not excluded from the pack — collapsing
// "not confirmed" into "false" only makes the pack larger, never wrong.
func HasObject(gitDir, sha string) bool {
	_, code, _ := git(gitDir, "cat-file", "-e", sha+"^{object}")
	return code == 0
}

// IsAncestor reports whether old is an ancestor of new (a commit is its own
// ancestor). Only exit code 1 from merge-base --is-ancestor is the genuine,
// confirmed negative answer ("not an ancestor"); any other outcome — a
// higher exit code from a malformed or missing object, or the -1 sentinel
// git() returns when the git binary itself never started — means the
// question could not actually be answered, and is surfaced as (false, err)
// rather than silently folded into the same "not an ancestor" result. Task
// 10 gates non-fast-forward rejection on this return value, so a genuine
// tooling failure must not present as "confirmed safe to reject" with no
// diagnostic trail.
func IsAncestor(gitDir, old, new string) (bool, error) {
	out, code, err := git(gitDir, "merge-base", "--is-ancestor", old, new)
	switch code {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		if err != nil {
			return false, fmt.Errorf("merge-base --is-ancestor %s %s: %s: %w", old, new, out, err)
		}
		return false, fmt.Errorf("merge-base --is-ancestor %s %s: %s (exit %d)", old, new, out, code)
	}
}

// RevParse resolves rev (a ref name, HEAD, or any other `git rev-parse`
// input) to its sha. It returns the trimmed output, the raw exit code, and
// any start/run error, the same three-value shape git() itself returns:
// callers (Task 10's resolve()) decide what counts as success themselves,
// the same way IsAncestor and WritePack above interpret git's exit codes
// rather than collapsing them into a single bool.
func RevParse(gitDir, rev string) (string, int, error) {
	out, code, err := git(gitDir, "rev-parse", rev)
	return out, code, err
}

// WritePack builds a NON-THIN pack containing the objects reachable from
// want but not from any of haves, and writes it into outDir. --no-thin is
// deliberate: a thin pack would depend on delta bases the remote may not
// hold, and the remote here is Proton Drive, not another git repo that can
// fill in bases on the fly.
//
// Ordering guarantee: pack, then idx, then confirm both exist, then (by the
// caller) the ref. WritePack itself only does the first three steps — it
// must not report success, or hand back a path, it has not confirmed. A ref
// pointing at a missing index would not be fetch-discoverable.
//
// When rev-list finds nothing to send (want is already covered by haves),
// WritePack returns ("", "", nil): a legitimate, distinct outcome, not an
// error.
func WritePack(gitDir, want string, haves []string, outDir string) (string, string, error) {
	revArgs := []string{"rev-list", "--objects", want}
	for _, h := range haves {
		revArgs = append(revArgs, "^"+h)
	}
	objs, code, err := git(gitDir, revArgs...)
	if code != 0 {
		if err != nil {
			return "", "", fmt.Errorf("rev-list failed: %s: %w", objs, err)
		}
		return "", "", fmt.Errorf("rev-list failed: %s", objs)
	}
	// The one caller of git() where truncated output would be silently WRONG
	// rather than merely noisy: this list IS the pack's contents, so a short
	// read produces a smaller pack that is missing objects, gets published,
	// and is unrecoverable. Every other git() caller fails closed on its own
	// under truncation; this one has to say so. Fail closed here rather than
	// pack what we happen to have read.
	if errors.Is(err, exec.ErrWaitDelay) {
		return "", "", fmt.Errorf("rev-list exited but something still held its output pipe "+
			"open after %s, so its object list cannot be trusted to be complete; refusing to "+
			"build a pack that may be missing objects", waitDelay)
	}
	if strings.TrimSpace(objs) == "" {
		return "", "", nil // nothing to send
	}

	// pack-objects writes the pack hash to stdout, which is then parsed into
	// a filename below. stdout must not be mixed with stderr here (unlike
	// the git() helper above) — any warning git writes to stderr would
	// corrupt the parsed name and produce a path that does not exist. stdout
	// is captured alone for parsing; stderr is kept separately and folded
	// into the error message on failure so diagnostics still surface.
	// Both pins are normative (design v6.2). packSizeLimit must be overridden
	// as CONFIG, not as --max-pack-size=0: git reads 0 there as "unset" and
	// falls back to the very config being overridden. The index-version pin
	// must be on the command line too, because pack.indexVersion is
	// user-configurable and `index-pack --verify` validates an index in
	// whatever version it already is.
	cmd := exec.Command("git", "-C", gitDir,
		"-c", "pack.packSizeLimit=0",
		"pack-objects", "--no-thin", "--index-version=2", "-q",
		filepath.Join(outDir, "pack"))
	cmd.Stdin = strings.NewReader(objs + "\n")
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// Same exposure as git() above, for the same reason: *strings.Builder is
	// not an *os.File, so these are pipes a grandchild can hold open past the
	// child's exit. See waitDelay: DO NOT DELETE as unnecessary.
	cmd.WaitDelay = waitDelay
	if err := cmd.Run(); err != nil {
		// ErrWaitDelay means pack-objects itself exited 0 but the pipe was
		// abandoned, so stdout — which is the pack NAME this function parses a
		// filename out of — may be short. A truncated hash yields a path that
		// does not exist, and the os.Stat guards below would report that as a
		// missing pack, blaming the wrong thing. Name the real cause instead.
		if errors.Is(err, exec.ErrWaitDelay) {
			return "", "", fmt.Errorf("pack-objects exited but something still held its output "+
				"pipe open after %s, so the pack name it printed cannot be trusted to be "+
				"complete; refusing to guess which pack it wrote", waitDelay)
		}
		return "", "", fmt.Errorf("pack-objects: %s: %w", strings.TrimSpace(stderr.String()), err)
	}
	name := strings.TrimSpace(stdout.String())

	// One pack is an invariant the whole publication path rests on: a second
	// pack silently dropped would publish a ref whose objects are only
	// half-uploaded. The size pin above should make this unreachable, so
	// treat it as a hard error rather than parsing the first line.
	if strings.ContainsAny(name, " \t\r\n") {
		return "", "", fmt.Errorf("pack-objects emitted more than one pack (%q); "+
			"the one-pack invariant is broken", name)
	}
	base := filepath.Join(outDir, "pack-"+name)
	packPath, idxPath := base+".pack", base+".idx"

	// Fail closed: confirm both files actually exist before returning them.
	// The design's ordering guarantee is pack -> idx -> confirm both -> ref;
	// a caller that trusts an unconfirmed path could write a ref pointing at
	// a pack or index that was never actually written, which would not be
	// fetch-discoverable.
	if _, err := os.Stat(packPath); err != nil {
		return "", "", fmt.Errorf("pack-objects reported %s but the pack file is missing: %w", packPath, err)
	}
	if _, err := os.Stat(idxPath); err != nil {
		return "", "", fmt.Errorf("pack-objects reported %s but the idx file is missing: %w", idxPath, err)
	}
	return packPath, idxPath, nil
}

// IndexPackVerify runs `git index-pack --verify` on packPath, whose .idx must
// sit beside it under the same stem. It is the only check that proves a pack
// is internally well-formed and agrees with its index; a basename/checksum
// comparison is the only check that proves the file is the one the name
// claims. Neither substitutes for the other (design v6.2, error table).
//
// It verifies an index in whatever version that index already is, which is
// why it cannot be the enforcement point for the index-version pin.
func IndexPackVerify(packPath string) error {
	cmd := exec.Command("git", "index-pack", "--verify", packPath)
	// Same exposure as git() above. See waitDelay: DO NOT DELETE.
	cmd.WaitDelay = waitDelay
	out, err := cmd.CombinedOutput()
	// ErrWaitDelay is substituted for a nil error ONLY on a successful exit,
	// so the verification itself passed and only the pipe drain was abandoned.
	// Returning it as an error would report a passing verification as
	// "the remote index does not verify against its pack" and refuse a
	// legitimate push — a false failure, not a fail-closed one. Warn instead.
	if errors.Is(err, exec.ErrWaitDelay) {
		warnWaitDelay("git index-pack --verify " + packPath)
		return nil
	}
	if err != nil {
		return fmt.Errorf("index-pack --verify %s: %s: %w",
			packPath, strings.TrimSpace(string(out)), err)
	}
	return nil
}
