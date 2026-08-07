package repo

import (
	"fmt"
	"io"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// EnsureParents is the Task 11 answer to Surprise R2-1: a push against a repo
// root whose PARENT folders do not exist used to surface the CLI's raw
// create-folder failure ("Node not found: GitRemotes") straight to the
// operator, with no remedy stated. It is called ONLY from cmd's "list
// for-push" arm, BEFORE Bootstrap — never from a read path (a fetch or plain
// "list" must never bring anything into existence) and never from
// runSetHead (design spec, component 3/6: a repo cannot exist below a
// missing parent, so the var there could only manufacture folder trees and
// then fail on the marker anyway; SetHead's own RequireMarker refusal is
// already the correct answer for that path — see repo_test.go's
// TestSetHeadNeverCreatesParents).
//
// root is assumed ALREADY CANONICAL (repo.CanonicalRoot has already run):
// it begins with /my-files/... or /devices/<id>/..., normalised, with no
// "." or ".." components. EnsureParents does not re-validate that — it
// trusts the same invariant Bootstrap and every other repo-root consumer
// does.
//
// create selects the two modes the design names:
//
//   - false (default, GPB_CREATE_PARENTS unset): the FIRST missing parent
//     produces an actionable refusal — naming the parent and an EXECUTABLE
//     remedy in the CLI's real grammar (`proton-drive filesystem
//     create-folder <parent> <name>` takes a parent PLUS a name, not a
//     path), the web UI, and the env var. Nothing is created.
//   - true (GPB_CREATE_PARENTS=1): missing parents are created one at a
//     time, each with a loud stderr line, so an operator scrolling back
//     through a push's output can see exactly what the helper did on its
//     own initiative. There is deliberately NO ROLLBACK: if creation fails
//     partway, whatever was already created stays created (removing it would
//     be exactly the unsafe folder-deletion race repo.Push's prune logic
//     bounds so carefully, for zero benefit — see push.go) and stderr's
//     already-written lines are the record of what happened.
//
// Walk bounds are the load-bearing part of both modes: the MOUNT itself
// (/my-files, or /devices/<device-id>) is never a candidate for creation in
// either mode — a device mount is not creatable storage, and /my-files
// existing at all is an account invariant, so an absent one is a real
// transport/account problem the helper must surface, not paper over. The
// mount is Stat'd directly, once, before any parent walk begins; an absent
// mount is refused in BOTH modes with the same message, explicitly stating
// that GPB_CREATE_PARENTS does not apply to it.
//
// The walk itself covers only the PARENTS strictly between the mount and the
// repo root's own leaf component — the leaf is Bootstrap's job (EnsureDir on
// root itself), never created here. Because of that boundary, a root that is
// a direct child of its mount (e.g. "/my-files/repo") walks zero parents:
// EnsureParents Stats the mount, finds nothing left to check, and returns
// nil having created nothing — this is also why a create=false push against
// such a root, missing only the LEAF itself, reaches Bootstrap's own
// EnsureDir(root) failure unwrapped: EnsureParents never had a parent of its
// own to refuse on. Anywhere the parent chain genuinely has a missing
// component, EnsureParents intercepts BEFORE Bootstrap ever runs, which is
// what turns the raw create-folder failure into the actionable refusal.
func EnsureParents(t transport.Transport, root string, create bool, stderr io.Writer) error {
	parts := strings.Split(strings.TrimPrefix(root, "/"), "/") // canonical already
	protectedDepth := 1                                        // /my-files
	if parts[0] == "devices" {
		protectedDepth = 2 // /devices/<device-id>
	}
	prefix := "/" + strings.Join(parts[:protectedDepth], "/")
	// The MOUNT ITSELF is checked first and is never creatable (design spec
	// component 6, both peer-review engines round 1): absent mount → the
	// actionable refusal in BOTH modes, naming it and stating it cannot be
	// created by the helper (a device mount is not creatable storage;
	// /my-files existing is an account invariant, so an absent one is a real
	// transport/account problem the user must see).
	if n, ok, err := t.Stat(prefix); err != nil {
		return fmt.Errorf("checking mount %s: %w", prefix, err)
	} else if !ok {
		return fmt.Errorf("mount %s does not exist or is not reachable; the helper never "+
			"creates mounts (%s does not apply here)", prefix, "GPB_CREATE_PARENTS")
	} else if !n.IsDir {
		return fmt.Errorf("mount %s is not a folder", prefix)
	}
	for i := protectedDepth; i < len(parts)-1; i++ { // parents ONLY; the leaf is Bootstrap's
		prefix += "/" + parts[i]
		n, ok, err := t.Stat(prefix)
		if err != nil {
			return fmt.Errorf("checking parent folder %s: %w", prefix, err)
		}
		if ok {
			if !n.IsDir {
				// A FILE where a parent folder must stand (round-1 Codex: an
				// earlier draft accepted any existing node as a usable
				// folder). Applies in BOTH modes — a name already taken by a
				// file is not something GPB_CREATE_PARENTS can resolve.
				return fmt.Errorf("cannot use %s as a parent folder: a file occupies that name", prefix)
			}
			continue
		}
		if !create {
			parent, name := prefix[:strings.LastIndex(prefix, "/")], parts[i]
			return fmt.Errorf("parent folder %s does not exist; create it first "+
				"(proton-drive filesystem create-folder %s %s, or the web UI), or set "+
				"%s=1 to let the helper create missing parents", prefix, parent, name, "GPB_CREATE_PARENTS")
		}
		if err := t.EnsureDir(prefix); err != nil {
			return fmt.Errorf("creating parent folder %s (GPB_CREATE_PARENTS=1): %w", prefix, err)
		}
		fmt.Fprintf(stderr, "git-remote-proton: created parent folder %s (GPB_CREATE_PARENTS=1)\n", prefix)
	}
	return nil
}
