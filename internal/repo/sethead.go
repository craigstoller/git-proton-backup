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
	// The report goes to os.Stderr directly, this package's convention for
	// advisory warnings (idxcache.go does the same) — so tests cannot capture
	// it.
	defer func() {
		if rerr := lock.Release(); rerr != nil {
			fmt.Fprintf(os.Stderr, "git-remote-proton: %v\n", rerr)
		}
	}()

	// Existence check via an EXACT-PATH Stat, not a full-tree ListRefs walk:
	// the target branch's own ref path is Stat'd directly, and Stat absence
	// is trustworthy post-Task 4 (an affirmative "does not exist", never a
	// folded-in transport failure — see transport.Transport.Stat's own doc
	// comment). ListRefs is reserved for the ERROR path only, to build the
	// "branches that exist" suggestion list (now recursive, so nested
	// branches are suggested too) — a successful set never walks the tree.
	// This still runs BEFORE the idempotence short-circuit below, for the
	// same round-3 peer-review reason as before: a HEAD already naming a
	// since-deleted branch must refuse, not short-circuit to success.
	if _, ok, err := t.Stat(root + "/" + branch); err != nil {
		return "", err
	} else if !ok {
		refs, lerr := ListRefs(t, root)
		if lerr != nil {
			return "", lerr
		}
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
	// Confirmed present by Stat above; readRef verifies it is actually a
	// well-formed ref file (exact 40-hex-plus-newline grammar). A
	// present-but-corrupt ref is FATAL here, never coerced into "does not
	// exist" — the same fail-closed rule readRef always applies to content
	// it reaches.
	if _, err := readRef(t, root+"/"+branch); err != nil {
		return "", err
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

// normalizeBranch turns the user's argument into a full refs/heads/ name.
// Hierarchical names ("feature/x") are accepted (Task 10 lifts Stage 4's
// blanket refusal): once the refs/heads/ prefix is settled, the full name is
// validated by advertisableName — the same git-validity-plus-stageability
// check ListRefs and Push apply to every other ref this helper advertises or
// writes, covering both full git validity (CheckRefName) and per-component
// stageability (checkComponent/checkStageableLeaf). Backslash is rejected as
// part of CheckRefName's forbidden-character set, not a bespoke check here.
func normalizeBranch(arg string) (string, error) {
	b := arg
	if !strings.HasPrefix(b, "refs/") {
		b = "refs/heads/" + b
	}
	if !strings.HasPrefix(b, "refs/heads/") {
		return "", fmt.Errorf("refusing to point HEAD at %q: HEAD points at branches only", arg)
	}
	if err := advertisableName(b); err != nil {
		return "", err
	}
	return b, nil
}
