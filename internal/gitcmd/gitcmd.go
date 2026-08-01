// Package gitcmd wraps git plumbing commands (cat-file, merge-base,
// rev-list, pack-objects) needed by git-remote-proton. All object work is
// done by shelling out to git rather than reimplementing pack formats.
//
// This package is self-contained: it imports nothing from internal/transport
// or internal/repo.
package gitcmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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
	out, err := cmd.CombinedOutput()
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
	if strings.TrimSpace(objs) == "" {
		return "", "", nil // nothing to send
	}

	// pack-objects writes the pack hash to stdout, which is then parsed into
	// a filename below. stdout must not be mixed with stderr here (unlike
	// the git() helper above) — any warning git writes to stderr would
	// corrupt the parsed name and produce a path that does not exist. stdout
	// is captured alone for parsing; stderr is kept separately and folded
	// into the error message on failure so diagnostics still surface.
	cmd := exec.Command("git", "-C", gitDir, "pack-objects", "--no-thin", "-q",
		filepath.Join(outDir, "pack"))
	cmd.Stdin = strings.NewReader(objs + "\n")
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("pack-objects: %s: %w", strings.TrimSpace(stderr.String()), err)
	}
	name := strings.TrimSpace(stdout.String())
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
