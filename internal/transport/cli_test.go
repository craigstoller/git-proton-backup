package transport

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// *CLI satisfies Transport as of Task 5's write side (CreateExclusive,
// UpdateRevision, Trash), mirroring the same assertion for *Fake in
// fake_test.go.
var _ Transport = (*CLI)(nil)

const unwrapped = `{"uid":"u1","name":{"ok":true,"value":"x.bundle"},"type":"file",
 "activeRevision":{"uid":"r1","state":"active","claimedSize":8}}`

const wrapped = `{"uid":"u1","name":{"ok":true,"value":"x.bundle"},"type":"file",
 "activeRevision":{"ok":true,"value":{"uid":"r1","state":"active","claimedSize":8}}}`

func TestParseNodeJSONBothShapes(t *testing.T) {
	for name, payload := range map[string]string{"0.7.0": unwrapped, "0.4.6": wrapped} {
		n, err := parseNodeJSON([]byte(payload))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if n.Name != "x.bundle" || n.Size != 8 {
			t.Errorf("%s: got %+v", name, n)
		}
	}
}

// TestCLIStatStartFailureIsNotConfirmedAbsence locks in the fix from Task 4
// review round 1: when the CLI executable itself cannot start (missing from
// PATH, permission denied, ...), run's nil-ProcessState guard must not let
// Stat fold that into "this path does not exist". A start failure is a
// transport error and must come back as a non-nil error, not as a confirmed
// (_, false, nil) absence, and it must not panic getting there.
func TestCLIStatStartFailureIsNotConfirmedAbsence(t *testing.T) {
	c := NewCLI("nonexistent-xyz-binary-git-proton-backup-test")

	_, ok, err := c.Stat("/whatever")

	if err == nil {
		t.Fatalf("want a non-nil error when the CLI binary cannot start, got ok=%v err=nil (reads as confirmed absence)", ok)
	}
	if ok {
		t.Fatalf("want ok=false alongside the error, got ok=true")
	}
}

func TestClassifyUpload(t *testing.T) {
	cases := []struct {
		t, s, f int
		want    Outcome
	}{
		{1, 0, 0, Committed},
		{0, 1, 0, Refused},
		{0, 0, 1, Ambiguous}, // a reported failure needs reconciliation
		{1, 1, 0, Ambiguous}, // contradictory for one file
		{0, 0, 0, Ambiguous}, // nothing happened; unknown
		{0, 0, 2, Ambiguous},
	}
	for _, c := range cases {
		if got := classifyUpload(c.t, c.s, c.f); got != c.want {
			t.Errorf("(%d,%d,%d): got %v want %v", c.t, c.s, c.f, got, c.want)
		}
	}
}

// TestParseTransferSummaryMissingFieldIsAmbiguous locks in the design rule
// that a missing count field decodes as Ambiguous, never a defaulted zero.
// Without this, a renamed or dropped `failedItems` combined with
// `skippedItems:1` would silently read as a confident Refused for a write
// that may well have landed.
func TestParseTransferSummaryMissingFieldIsAmbiguous(t *testing.T) {
	cases := []struct {
		name string
		json string
		want Outcome
		// wantErr: only decoding-level problems (unparseable JSON, a missing
		// count field) produce an error. A well-formed body that decodes to
		// a genuinely-ambiguous tuple, e.g. (0,0,0), is still Ambiguous per
		// classifyUpload's own truth table, but that is not a decode error.
		wantErr bool
	}{
		{"all fields present, committed", `{"transferredItems":1,"skippedItems":0,"failedItems":0}`, Committed, false},
		{"all fields present, refused", `{"transferredItems":0,"skippedItems":1,"failedItems":0}`, Refused, false},
		{"all fields present but genuinely zero", `{"transferredItems":0,"skippedItems":0,"failedItems":0}`, Ambiguous, false},
		{"failedItems missing entirely", `{"transferredItems":0,"skippedItems":1}`, Ambiguous, true},
		{"transferredItems missing entirely", `{"skippedItems":0,"failedItems":0}`, Ambiguous, true},
		{"empty object, every field missing", `{}`, Ambiguous, true},
		{"unparseable", `not json`, Ambiguous, true},
		// C3's own {uid,ok} shape has none of the count fields: this is the
		// exact case the design flags as "would silently read nulls" if the
		// fields were plain ints instead of pointers.
		{"trash shape fed to the upload parser", `{"uid":"x","ok":true}`, Ambiguous, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseTransferSummary(c.json)
			if got != c.want {
				t.Errorf("got %v want %v (err=%v)", got, c.want, err)
			}
			if c.wantErr && err == nil {
				t.Errorf("want a non-nil error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Errorf("want a nil error, got %v", err)
			}
		})
	}
}

// TestParseTrashResult locks in the design rule that a delete is Committed
// only when affirmatively confirmed by the C3 {uid,ok} array shape, and
// never inferred from exit status alone.
func TestParseTrashResult(t *testing.T) {
	cases := []struct {
		name string
		json string
		want Outcome
	}{
		// Stage 1 C3's literal evidence payload.
		{"C3 evidence payload, ok true", "[\r\n{\"uid\":\"tU-Ot1Sq63NwBcxlnl7IcA~-Rqa_crVUqB4keyIbsgkQw\",\"ok\":true}\r\n]", Committed},
		{"ok explicitly false", `[{"uid":"x","ok":false}]`, Ambiguous},
		{"empty array", `[]`, Ambiguous},
		{"more than one item", `[{"uid":"a","ok":true},{"uid":"b","ok":true}]`, Ambiguous},
		{"unparseable", `not json`, Ambiguous},
		// The upload transfer-summary shape (an object) is not the trash
		// array shape: applying the wrong parser must fail loudly, not read
		// a false success.
		{"upload transfer-summary shape fed to the trash parser", `{"transferredItems":1,"skippedItems":0,"failedItems":0}`, Ambiguous},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseTrashResult(c.json)
			if got != c.want {
				t.Errorf("got %v want %v (err=%v)", got, c.want, err)
			}
			if c.want == Ambiguous && err == nil {
				t.Errorf("want a non-nil error alongside Ambiguous, got nil")
			}
			if c.want != Ambiguous && err != nil {
				t.Errorf("want a nil error for %v, got %v", c.want, err)
			}
		})
	}
}

// TestCLITrashStartFailureIsAmbiguous mirrors
// TestCLIStatStartFailureIsNotConfirmedAbsence: when the CLI binary cannot
// start at all, Trash's Stat-first check must surface that as Ambiguous
// plus a non-nil error, never fold it into the absent-target Committed
// path. No real proton-drive CLI is invoked — the binary never starts.
func TestCLITrashStartFailureIsAmbiguous(t *testing.T) {
	c := NewCLI("nonexistent-xyz-binary-git-proton-backup-test")

	got, err := c.Trash("/whatever")

	if err == nil {
		t.Fatalf("want a non-nil error when the CLI binary cannot start, got outcome=%v err=nil", got)
	}
	if got != Ambiguous {
		t.Fatalf("want Ambiguous when the CLI binary cannot start, got %v", got)
	}
}

// TestCLIUploadStartFailureIsAmbiguous exercises CreateExclusive and
// UpdateRevision end to end against a binary that cannot start, without
// invoking the real proton-drive CLI: c.run's CombinedOutput on an
// unstartable process yields empty output, which parseTransferSummary must
// treat as unparseable — Ambiguous, not a false Committed or Refused.
//
// It also pins the fix for the one CLI method that discarded both the exit
// code and the start error (`out, _, _ := c.run(...)`). With empty output the
// message degraded to a bare "unparseable upload summary: " and the actual
// cause was lost, which is the worst possible diagnostic for the case an
// operator is most likely to hit — the CLI not being installed. The start
// error must survive into the returned error.
func TestCLIUploadStartFailureIsAmbiguous(t *testing.T) {
	const exe = "nonexistent-xyz-binary-git-proton-backup-test"
	c := NewCLI(exe)

	fns := map[string]func(string, string) (Outcome, error){
		"CreateExclusive": c.CreateExclusive,
		"UpdateRevision":  c.UpdateRevision,
	}
	for name, fn := range fns {
		t.Run(name, func(t *testing.T) {
			got, err := fn("/some/path.txt", "local.txt")
			if err == nil {
				t.Fatalf("want a non-nil error, got outcome=%v err=nil", got)
			}
			if got != Ambiguous {
				t.Fatalf("want Ambiguous, got %v", got)
			}
			// exec's start error names the executable it could not run; that
			// naming is the whole diagnostic value being preserved here.
			if !strings.Contains(err.Error(), exe) {
				t.Errorf("error must preserve the start failure's cause, got %q", err)
			}
			if !errors.Is(err, exec.ErrNotFound) && !strings.Contains(err.Error(), "exited -1") {
				t.Errorf("error must preserve the exit code or the wrapped exec error, got %q", err)
			}
		})
	}
}

// TestDirOf pins the real behaviour of dirOf, the function the Task 5
// review round 1 finding (C11) turned on: CreateExclusive/UpdateRevision
// pass only dirOf(p) to the CLI, so the CLI process's target directory
// depends entirely on this function being correct. Asserts actual observed
// behaviour, not an assumed one.
func TestDirOf(t *testing.T) {
	cases := []struct {
		name string
		p    string
		want string
	}{
		{"nested path", "/a/b/c", "/a/b"},
		{"parent is root", "/a", "/"},
		{"root itself", "/", "/"},
		{"no slash at all", "filename.txt", "/"},
		{"empty string", "", "/"},
		{"relative nested path, no leading slash", "refs/heads/master", "refs/heads"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := dirOf(c.p); got != c.want {
				t.Errorf("dirOf(%q) = %q, want %q", c.p, got, c.want)
			}
		})
	}
}
