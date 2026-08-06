package transport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RED: NewTraced does not exist. A successful ReadTo emits exactly one line
// with the normative prefix, the remote path, and the landed byte count.
func TestTracedReadToLogsOneLine(t *testing.T) {
	f := NewFake()
	f.Files["/r/packs/pack-abc.pack"] = []byte("hello")
	var buf strings.Builder
	tr := NewTraced(f, &buf)
	dir := t.TempDir()
	if err := tr.ReadTo("/r/packs/pack-abc.pack", dir); err != nil {
		t.Fatalf("ReadTo: %v", err)
	}
	want := "gpb: downloaded /r/packs/pack-abc.pack (5 bytes)\n"
	if buf.String() != want {
		t.Errorf("trace = %q, want %q", buf.String(), want)
	}
	if _, err := os.Stat(filepath.Join(dir, "pack-abc.pack")); err != nil {
		t.Errorf("delegation must still land the file: %v", err)
	}
}

// RED: a FAILED ReadTo must not log — the gate counts these lines as
// transfers, and a failed transfer is not a transfer.
func TestTracedReadToFailureLogsNothing(t *testing.T) {
	f := NewFake()
	var buf strings.Builder
	tr := NewTraced(f, &buf)
	if err := tr.ReadTo("/r/absent", t.TempDir()); err == nil {
		t.Fatal("ReadTo of a missing node must fail")
	}
	if buf.String() != "" {
		t.Errorf("failed ReadTo logged %q; want nothing", buf.String())
	}
}

// RED: every other method delegates verbatim and logs nothing.
func TestTracedDelegatesSilently(t *testing.T) {
	f := NewFake()
	var buf strings.Builder
	tr := NewTraced(f, &buf)
	if err := tr.EnsureDir("/r/d"); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.List("/r"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := tr.Stat("/r/d"); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(t.TempDir(), "leaf")
	if err := os.WriteFile(local, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.CreateExclusive("/r/leaf", local); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.UpdateRevision("/r/leaf", local); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.Trash("/r/leaf"); err != nil {
		t.Fatal(err)
	}
	if !f.Dirs["/r/d"] {
		t.Error("EnsureDir did not delegate")
	}
	if buf.String() != "" {
		t.Errorf("non-ReadTo methods logged %q; want nothing", buf.String())
	}
}

// readToWithoutLanding reports ReadTo success without creating the local
// file, which is the only way to reach Traced's size-unknown branch.
type readToWithoutLanding struct{ Transport }

func (readToWithoutLanding) ReadTo(p, local string) error { return nil }

// GUARD (Stage 4): the size-unknown fallback must still emit the normative
// "gpb: downloaded" prefix the gate greps for.
func TestTracedReportsSizeUnknownWhenTheFileDidNotLand(t *testing.T) {
	var buf strings.Builder
	tr := NewTraced(readToWithoutLanding{}, &buf)
	if err := tr.ReadTo("/r/packs/pack-x.idx", t.TempDir()); err != nil {
		t.Fatalf("stub ReadTo must succeed: %v", err)
	}
	want := "gpb: downloaded /r/packs/pack-x.idx (size unknown)\n"
	if buf.String() != want {
		t.Errorf("trace line %q, want %q", buf.String(), want)
	}
}
