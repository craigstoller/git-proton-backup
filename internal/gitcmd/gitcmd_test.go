package gitcmd

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// newRepo builds a real, throwaway git repo under t.TempDir() with one
// commit. Every step's error is checked (the brief's original version
// ignored errors from the setup commands and from rev-parse, then sliced
// the rev-parse output to [:40] unconditionally — which panics on an empty
// result instead of failing with a useful message). A broken test helper
// must report itself via t.Fatal, not panic.
func newRepo(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	for _, a := range [][]string{
		{"init", "-qb", "main"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"},
	} {
		if err := exec.Command("git", append([]string{"-C", d}, a...)...).Run(); err != nil {
			t.Fatalf("git %v: %v", a, err)
		}
	}
	if err := os.WriteFile(d+"/a.txt", []byte("one"), 0o644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	if err := exec.Command("git", "-C", d, "add", ".").Run(); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := exec.Command("git", "-C", d, "commit", "-qm", "c1").Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	return d
}

// headOf returns the full HEAD sha of the repo at d, failing the test
// loudly if rev-parse errors or returns something other than a 40-char sha
// (rather than panicking on a short slice, as the brief's inline version
// would).
func headOf(t *testing.T, d string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", d, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	sha := strings.TrimSpace(string(out))
	if len(sha) != 40 {
		t.Fatalf("rev-parse HEAD returned %q, want a 40-char sha", sha)
	}
	return sha
}

func TestObjectTypeAndAncestry(t *testing.T) {
	d := newRepo(t)
	head := headOf(t, d)

	if got, err := ObjectType(d, head); err != nil || got != "commit" {
		t.Fatalf("ObjectType = %q, %v; want commit", got, err)
	}
	if !HasObject(d, head) {
		t.Error("HasObject must find HEAD")
	}
	if ok, err := IsAncestor(d, head, head); !ok || err != nil {
		t.Errorf("IsAncestor(head, head) = %v, %v; want true, nil (a commit is its own ancestor)", ok, err)
	}
}

// TestHasObjectMissing covers the negative path Task 10 depends on: a sha
// that is well-formed but does not exist in the repo must report false, not
// merely be left untested alongside the positive "finds HEAD" case above.
func TestHasObjectMissing(t *testing.T) {
	d := newRepo(t)
	missing := "0000000000000000000000000000000000000000"
	if HasObject(d, missing) {
		t.Error("HasObject must not report a nonexistent object as present")
	}
}

// TestIsAncestorNonAncestor covers the genuine negative answer using two
// commits that actually diverge, not just the trivial self-ancestor case:
// a commit on branchA and a commit on branchB, both built on the same base
// commit from newRepo, are ancestors of neither each other.
func TestIsAncestorNonAncestor(t *testing.T) {
	d := newRepo(t)

	commitOn := func(branch, file, content string) string {
		if err := exec.Command("git", "-C", d, "checkout", "-qb", branch).Run(); err != nil {
			t.Fatalf("checkout %s: %v", branch, err)
		}
		if err := os.WriteFile(d+"/"+file, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
		if err := exec.Command("git", "-C", d, "add", ".").Run(); err != nil {
			t.Fatalf("add: %v", err)
		}
		if err := exec.Command("git", "-C", d, "commit", "-qm", branch).Run(); err != nil {
			t.Fatalf("commit %s: %v", branch, err)
		}
		return headOf(t, d)
	}

	commitA := commitOn("branchA", "b.txt", "A")
	if err := exec.Command("git", "-C", d, "checkout", "-q", "main").Run(); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	commitB := commitOn("branchB", "c.txt", "B")

	if ok, err := IsAncestor(d, commitA, commitB); ok || err != nil {
		t.Fatalf("IsAncestor(A, B) = %v, %v; want false, nil for genuinely divergent commits", ok, err)
	}
	if ok, err := IsAncestor(d, commitB, commitA); ok || err != nil {
		t.Fatalf("IsAncestor(B, A) = %v, %v; want false, nil for genuinely divergent commits", ok, err)
	}
}

// TestIsAncestorMalformedObjectIsError proves the new error path added in
// this fix round is actually reachable: merge-base --is-ancestor exits 128
// (verified against a real git invocation), not 1, for a malformed object
// name, so this must come back as a non-nil error rather than a confirmed
// (false, nil). If IsAncestor were reverted to testing only `code == 0`,
// this would observe (false, nil) and fail — that is what makes this a real
// regression test rather than one that could never fail.
func TestIsAncestorMalformedObjectIsError(t *testing.T) {
	d := newRepo(t)
	head := headOf(t, d)

	ok, err := IsAncestor(d, "not-a-real-sha-0000000000000000000000", head)
	if err == nil {
		t.Fatalf("want a non-nil error for a malformed object, got ok=%v err=nil", ok)
	}
	if ok {
		t.Fatalf("want ok=false alongside the error, got true")
	}
}

// TestWritePack covers the function the dispatch flagged as having no
// coverage at all in the brief: the real-pack path (asserting the returned
// .pack and .idx actually exist on disk) and the "nothing to send" path,
// which must stay a legitimate, distinct, non-error outcome.
func TestWritePack(t *testing.T) {
	d := newRepo(t)
	head := headOf(t, d)

	t.Run("real pack", func(t *testing.T) {
		outDir := t.TempDir()
		packPath, idxPath, err := WritePack(d, head, nil, outDir)
		if err != nil {
			t.Fatalf("WritePack: %v", err)
		}
		if packPath == "" || idxPath == "" {
			t.Fatalf("want non-empty paths for a non-empty range, got pack=%q idx=%q", packPath, idxPath)
		}
		if _, err := os.Stat(packPath); err != nil {
			t.Errorf("returned pack path does not exist on disk: %v", err)
		}
		if _, err := os.Stat(idxPath); err != nil {
			t.Errorf("returned idx path does not exist on disk: %v", err)
		}
	})

	t.Run("nothing to send", func(t *testing.T) {
		outDir := t.TempDir()
		// want == the only have: rev-list --objects head ^head is empty.
		packPath, idxPath, err := WritePack(d, head, []string{head}, outDir)
		if err != nil {
			t.Fatalf("WritePack: %v", err)
		}
		if packPath != "" || idxPath != "" {
			t.Fatalf("want empty paths when there is nothing to send, got pack=%q idx=%q", packPath, idxPath)
		}
	})
}
