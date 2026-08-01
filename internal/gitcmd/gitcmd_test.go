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
	if ok, _ := IsAncestor(d, head, head); !ok {
		t.Error("a commit is its own ancestor")
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
