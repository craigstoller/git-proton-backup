package repo

import (
	"fmt"
	"os"

	"github.com/craigstoller/git-proton-backup/internal/gitcmd"
	"github.com/craigstoller/git-proton-backup/internal/protocol"
	"github.com/craigstoller/git-proton-backup/internal/transport"
)

type Result struct {
	Ref string
	OK  bool
	Err string
}

// Push applies each ref update in ups independently. Multi-ref batches are
// NOT atomic: every update gets its own Result, so partial success (some refs
// updated, others rejected) is expected and correct, never collapsed into a
// single batch-wide outcome.
func Push(t transport.Transport, root, gitDir string,
	ups []protocol.RefUpdate, remote map[string]string) []Result {

	results := make([]Result, 0, len(ups))
	for _, u := range ups {
		results = append(results, pushOne(t, root, gitDir, u, remote))
	}
	return results
}

// pushOne applies a single ref update. Ordering is pack -> idx -> confirm
// both -> ref: a ref must never point at objects that are not fully
// uploaded, because a ref whose index is missing is not fetch-discoverable.
func pushOne(t transport.Transport, root, gitDir string,
	u protocol.RefUpdate, remote map[string]string) Result {

	fail := func(msg string) Result { return Result{Ref: u.Dst, Err: msg} }
	oldSha, exists := remote[u.Dst]

	// --- delete -------------------------------------------------------------
	if u.Src == "" {
		if !exists {
			return Result{Ref: u.Dst, OK: true} // already absent
		}
		out, err := t.Trash(root + "/" + u.Dst)
		if err != nil {
			return fail(fmt.Sprintf("delete failed: %v", err))
		}
		if out != transport.Committed {
			// err is nil here, so a bare "%v" would print the useless
			// "delete failed: <nil>". Report the outcome itself instead.
			return fail(fmt.Sprintf("delete failed: outcome %s", out))
		}
		return Result{Ref: u.Dst, OK: true}
	}

	newSha, err := resolve(gitDir, u.Src)
	if err != nil {
		return fail(err.Error())
	}

	// --- branch targets must be commits ------------------------------------
	if isBranch(u.Dst) {
		typ, err := gitcmd.ObjectType(gitDir, newSha)
		if err != nil {
			return fail("cannot determine object type")
		}
		if typ != "commit" {
			return fail(fmt.Sprintf("branch cannot point at a %s", typ))
		}
	}

	// --- ancestry ----------------------------------------------------------
	if exists && !u.Force {
		if !gitcmd.HasObject(gitDir, oldSha) {
			return fail("fetch first")
		}
		// IsAncestor distinguishes "not an ancestor" (exit 1) from a tooling
		// failure. Discarding the error would report a broken git as a
		// confident non-fast-forward rejection.
		ok, err := gitcmd.IsAncestor(gitDir, oldSha, newSha)
		if err != nil {
			return fail("cannot determine ancestry: " + err.Error())
		}
		if !ok {
			return fail("non-fast-forward")
		}
	}

	// --- pack --------------------------------------------------------------
	tmp, err := os.MkdirTemp("", "gpb-pack-*")
	if err != nil {
		return fail(err.Error())
	}
	defer os.RemoveAll(tmp)

	haves := make([]string, 0, len(remote))
	for _, s := range remote {
		if gitcmd.HasObject(gitDir, s) {
			haves = append(haves, s)
		}
	}
	packPath, idxPath, err := gitcmd.WritePack(gitDir, newSha, haves, tmp)
	if err != nil {
		return fail("pack failed: " + err.Error())
	}

	if packPath != "" {
		// Pack, then index, then CONFIRM BOTH before publishing the ref.
		for _, f := range []string{packPath, idxPath} {
			dst := root + "/packs/" + filepathBase(f)
			out, err := t.CreateExclusive(dst, f)
			if err != nil {
				return fail("upload failed: " + err.Error())
			}
			if out == transport.Ambiguous {
				return fail("upload outcome ambiguous; re-run to reconcile")
			}
			// Refused here means identical content already exists (pack/idx
			// filenames are content-addressed) — that is success after
			// presence verification, not an error.
			if _, ok, _ := t.Stat(dst); !ok {
				return fail("uploaded object is not readable back: " + dst)
			}
		}
	}

	// --- publish ------------------------------------------------------------
	out, err := WriteRef(t, root, u.Dst, newSha, exists)
	if err != nil || out == transport.Ambiguous {
		return fail(fmt.Sprintf("ref publish failed: %v", err))
	}
	if out == transport.Refused {
		// WriteRef (refs.go) returns (Refused, nil) — no error — specifically
		// when this is a create (exists == false) and a concurrent creator
		// won the race; it deliberately did not overwrite. That is not the
		// same as success: our newSha was never published, so reporting
		// OK: true here would make git update its remote-tracking ref to a
		// sha that disagrees with what is actually on the remote, with
		// nothing to signal the mismatch. It must be reported as a failure.
		return fail("ref changed concurrently; refusing to overwrite")
	}
	return Result{Ref: u.Dst, OK: true}
}

func isBranch(ref string) bool { return len(ref) > 11 && ref[:11] == "refs/heads/" }

// resolve turns src (a sha or a ref name/other rev-parse input) into a sha.
// A value already shaped like a sha is returned as-is; anything else goes
// through `git rev-parse` in gitDir.
func resolve(gitDir, src string) (string, error) {
	if shaRe.MatchString(src) {
		return src, nil
	}
	out, code, _ := gitcmd.RevParse(gitDir, src)
	if code != 0 {
		return "", fmt.Errorf("cannot resolve %s", src)
	}
	return out, nil
}

// filepathBase returns the final path component of p, accepting both '/' and
// '\' as separators since gitcmd.WritePack's paths are built with
// filepath.Join and so use the host's native separator.
func filepathBase(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[i+1:]
		}
	}
	return p
}
