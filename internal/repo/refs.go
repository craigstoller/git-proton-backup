package repo

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/transport"
)

var shaRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// ListRefs lists refs/heads and refs/tags NON-RECURSIVELY: it lists the
// direct children of each namespace and skips any entry whose IsDir is true.
// A ref name containing a slash — refs/heads/feat/x, which git accepts and
// users create constantly — therefore lives inside a subdirectory that
// ListRefs never descends into, and is invisible to the advertisement.
// WriteRef has the mirror-image gap: it would try to upload into a
// refs/heads/feat folder that this package never creates.
//
// This is a known Stage 2 boundary, not an oversight. Stage 2's gate is a
// flat `git push proton-v2 main`; recursive listing and parent-directory
// creation for hierarchical ref namespaces belong to Stage 3, which owns
// clone/fetch and the wider ref namespace.
func ListRefs(t transport.Transport, root string) (map[string]string, error) {
	out := map[string]string{}
	for _, ns := range []string{"refs/heads", "refs/tags"} {
		nodes, err := t.List(root + "/" + ns)
		if err != nil {
			return nil, err
		}
		for _, n := range nodes {
			if n.IsDir {
				continue
			}
			sha, err := readRef(t, root+"/"+ns+"/"+n.Name)
			if err != nil {
				return nil, err
			}
			out[ns+"/"+n.Name] = sha
		}
	}
	return out, nil
}

// readRef downloads the ref file at p and parses its content. A ref file is
// exactly 40 lowercase hex plus a newline; anything else is corruption and is
// fatal here, never coerced into a best guess.
func readRef(t transport.Transport, p string) (string, error) {
	dir, err := os.MkdirTemp("", "gpb-ref-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	if err := t.ReadTo(p, dir); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return "", fmt.Errorf("ref %s could not be read back", p)
	}
	raw, err := os.ReadFile(dir + string(os.PathSeparator) + entries[0].Name())
	if err != nil {
		return "", err
	}
	sha := strings.TrimRight(string(raw), "\r\n")
	if !shaRe.MatchString(sha) {
		return "", fmt.Errorf("corrupt ref file %s: %q is not a 40-hex sha", p, sha)
	}
	return sha, nil
}

// WriteRef stages under the ref's own LEAF NAME, because `filesystem upload`
// names the uploaded node after the local basename and has no --name flag
// (probe C11). A leaf hostile to a local path is rejected by stagedFile with a
// named reason rather than mangled. It then verifies by read-back, because a
// byte-identical write is silently skipped.
func WriteRef(t transport.Transport, root, ref, sha string, exists bool) (transport.Outcome, error) {
	if !shaRe.MatchString(sha) {
		return transport.Ambiguous, fmt.Errorf("refusing to write non-sha %q to %s", sha, ref)
	}
	leaf := ref[strings.LastIndex(ref, "/")+1:]
	staged, cleanup, err := stagedFile([]byte(sha+"\n"), leaf)
	if err != nil {
		return transport.Ambiguous, err
	}
	defer cleanup()

	p := root + "/" + ref
	var out transport.Outcome
	if exists {
		out, err = t.UpdateRevision(p, staged)
	} else {
		out, err = t.CreateExclusive(p, staged)
	}
	if err != nil {
		return transport.Ambiguous, err
	}
	if out == transport.Refused && !exists {
		return transport.Refused, nil // concurrent creator
	}

	got, rerr := readRef(t, p)
	if rerr != nil {
		return transport.Ambiguous, rerr
	}
	if got != sha {
		return transport.Ambiguous, fmt.Errorf("ref %s reads back as %s, expected %s", ref, got, sha)
	}
	return transport.Committed, nil
}
