package repo

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// SetHead is the --set-head operation: point the remote's HEAD at an
// EXISTING branch, under the repo lock. Returns the normalized full ref
// name it set (or confirmed). Order is load-bearing and peer-reviewed:
// branch existence is verified on EVERY run BEFORE the idempotence
// short-circuit, so a HEAD already naming a since-deleted branch refuses
// rather than reporting success (round-3 finding, both engines).
func SetHead(t transport.Transport, root, branchArg string) (string, error) {
	branch, err := normalizeBranch(branchArg)
	if err != nil {
		return "", err
	}
	if err := RequireMarker(t, root); err != nil {
		return "", err
	}
	lock, err := AcquireLock(t, root)
	if err != nil {
		return "", err
	}
	// Release on EVERY exit path; its error is reported, never masking the
	// operation's own result — the same contract cmd's loop defer documents.
	defer func() {
		if rerr := lock.Release(); rerr != nil {
			fmt.Fprintf(os.Stderr, "git-remote-proton: %v\n", rerr)
		}
	}()

	refs, err := ListRefs(t, root)
	if err != nil {
		return "", err
	}
	if _, ok := refs[branch]; !ok {
		var branches []string
		for name := range refs {
			if strings.HasPrefix(name, "refs/heads/") {
				branches = append(branches, strings.TrimPrefix(name, "refs/heads/"))
			}
		}
		if len(branches) == 0 {
			return "", fmt.Errorf("cannot set HEAD to %q: no branches exist; push a branch first", branchArg)
		}
		sort.Strings(branches)
		return "", fmt.Errorf("cannot set HEAD to %q: no such branch; branches that exist: %s",
			branchArg, strings.Join(branches, ", "))
	}

	// Idempotence short-circuit — AFTER the existence check above. A ReadHEAD
	// error is FATAL (fail closed): it covers transient transport trouble as
	// much as corrupt content, and an indeterminate read must never license a
	// destructive overwrite (peer-review finding on this plan). A genuinely
	// corrupt HEAD is repaired by deleting the HEAD file in the web UI —
	// after which the repo is headless and SetHead's create path applies.
	head, hasHead, err := ReadHEAD(t, root)
	if err != nil {
		return "", err
	}
	if hasHead && head == branch {
		return branch, nil
	}

	if _, err := UpdateHEAD(t, root, branch); err != nil {
		return "", err
	}
	return branch, nil
}

// normalizeBranch turns the user's argument into a full refs/heads/ name,
// refusing everything Stage 4 does not support. The hierarchical refusal
// comes FIRST so it gets its own named reason (Stage 5), not the generic
// staging-path one.
func normalizeBranch(arg string) (string, error) {
	b := arg
	if !strings.HasPrefix(b, "refs/") {
		b = "refs/heads/" + b
	}
	if !strings.HasPrefix(b, "refs/heads/") {
		return "", fmt.Errorf("refusing to point HEAD at %q: HEAD points at branches only", arg)
	}
	leaf := strings.TrimPrefix(b, "refs/heads/")
	if strings.ContainsAny(leaf, `/\`) {
		return "", fmt.Errorf("hierarchical ref names are not supported yet (Stage 5): %q", arg)
	}
	if err := checkStageableLeaf(leaf); err != nil {
		return "", err
	}
	return b, nil
}
