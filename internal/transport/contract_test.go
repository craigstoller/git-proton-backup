package transport

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// liveEnv gates the *CLI half. CI never sets it, so `go test ./...` on a
// runner exercises only the Fake and can never touch a real account.
const liveEnv = "GPB_LIVE_ACCOUNT"

// liveRoot is the default remote path the live half writes to.
// GPB_CONTRACT_LIVE_ROOT (see contractLiveRoot) may override it — the
// Stage 5 gate hit a real incident (S2, docs/research/gates/stage5-gate.md)
// where a gate's authorised confinement did not include this hardcoded
// value, so the table refused to run under the gate's own root. An
// override still goes through validateContractLiveRoot, fail-closed.
const liveRoot = "/my-files/_cas-probe/contract"

// contractLiveRootEnv overrides liveRoot for one test run. Read fresh on
// every call (see contractLiveRoot) — never cached in package state.
const contractLiveRootEnv = "GPB_CONTRACT_LIVE_ROOT"

// untouchableTopLevelFolders are the four pre-existing top-level folders in
// the live Proton account that no gate or contract run may ever write to or
// trash. These are account facts, not inventions — verified verbatim
// against docs/research/gates/stage5-gate.md:669-670 ("Untouchable folders
// read-only. `GitBackups`, `Sensitive Project Sources`, `Project Repo
// Bundles`, `ChatGPT Export Text Backup` appeared only as rows in
// `filesystem list /my-files` output.") and cross-checked against the same
// four names in docs/research/gates/stage4-gate.md:271-272,830-831 and
// docs/research/gates/stage5-gate-brief.md:60,487.
var untouchableTopLevelFolders = map[string]bool{
	"GitBackups":                 true,
	"Sensitive Project Sources":  true,
	"Project Repo Bundles":       true,
	"ChatGPT Export Text Backup": true,
}

// validateContractLiveRoot is the PURE, fail-closed gate on the contract
// table's live root, directly unit-testable with no environment and no
// live account (TestValidateContractLiveRoot below). It requires:
//   - an absolute path strictly below /my-files/ — so /my-files itself is
//     refused, and any path outside /my-files is refused;
//   - no empty, ".", or ".." segment ANYWHERE, checked on the raw split of
//     v (not a cleaned path): a prefix check alone would still admit
//     /my-files/x/../../outside-style traversal if the CLI resolves dot
//     segments (round-3 Codex, spec Component 3);
//   - the first path segment under /my-files/ is not one of the four
//     untouchable top-level folders above.
//
// Every rejection's error names the offending value v.
func validateContractLiveRoot(v string) error {
	if !strings.HasPrefix(v, "/my-files/") {
		return fmt.Errorf("GPB_CONTRACT_LIVE_ROOT must be an absolute path strictly below /my-files/, got %q", v)
	}
	// TrimPrefix first, THEN split — round-1 Gemini (plan review): splitting
	// the raw absolute path (leading "/") yields an empty first element that
	// a no-empty-segments rule would then reject for every valid path.
	parts := strings.Split(strings.TrimPrefix(v, "/"), "/")
	if len(parts) < 2 || parts[0] != "my-files" {
		return fmt.Errorf("GPB_CONTRACT_LIVE_ROOT must be an absolute path strictly below /my-files/, got %q", v)
	}
	for _, p := range parts {
		if p == "" || p == "." || p == ".." {
			return fmt.Errorf("GPB_CONTRACT_LIVE_ROOT must not contain an empty, \".\", or \"..\" path segment, got %q", v)
		}
	}
	if untouchableTopLevelFolders[parts[1]] {
		return fmt.Errorf("GPB_CONTRACT_LIVE_ROOT must not be inside the untouchable folder %q, got %q", parts[1], v)
	}
	return nil
}

// contractLiveRoot resolves the live root TestContractCLI must use: the
// env override if set, else liveRoot — always passed through
// validateContractLiveRoot before any live call is made. An invalid value
// fails the live run loudly (t.Fatalf) rather than silently falling back to
// the default or proceeding unvalidated.
func contractLiveRoot(t *testing.T) string {
	t.Helper()
	v := os.Getenv(contractLiveRootEnv)
	if v == "" {
		v = liveRoot
	}
	if err := validateContractLiveRoot(v); err != nil {
		t.Fatalf("%s: %v", contractLiveRootEnv, err)
	}
	return v
}

// TestValidateContractLiveRoot exercises the PURE validator directly — no
// env, no live call, no t.Fatalf-only surface (round-1 Codex: a
// t.Fatalf-only surface cannot have rejection subtests, since an expected
// Fatalf would still fail the suite).
func TestValidateContractLiveRoot(t *testing.T) {
	reject := []struct{ name, v string }{
		{"root itself", "/my-files"},
		{"GitBackups prefix", "/my-files/GitBackups/x"},
		{"Sensitive Project Sources prefix", "/my-files/Sensitive Project Sources/x"},
		{"Project Repo Bundles prefix", "/my-files/Project Repo Bundles/x"},
		{"ChatGPT Export Text Backup prefix", "/my-files/ChatGPT Export Text Backup/x"},
		{"outside /my-files", "/other/x"},
		{"dot-dot segments climbing out", "/my-files/x/../../etc"},
		{"dot-dot segment reaching an untouchable", "/my-files/x/../GitBackups"},
		{"relative path", "relative/path"},
		{"empty segment (doubled slash)", "/my-files//x"},
	}
	for _, c := range reject {
		t.Run(c.name, func(t *testing.T) {
			err := validateContractLiveRoot(c.v)
			if err == nil {
				t.Fatalf("validateContractLiveRoot(%q): want a rejection, got nil", c.v)
			}
			if !strings.Contains(err.Error(), c.v) {
				t.Errorf("validateContractLiveRoot(%q): error must name the offending value, got %q", c.v, err.Error())
			}
		})
	}

	accept := []struct{ name, v string }{
		{"the default liveRoot", liveRoot},
		{"a sibling under the cas-probe root", "/my-files/_cas-probe/other"},
	}
	for _, c := range accept {
		t.Run(c.name, func(t *testing.T) {
			if err := validateContractLiveRoot(c.v); err != nil {
				t.Errorf("validateContractLiveRoot(%q): want acceptance, got %v", c.v, err)
			}
		})
	}
}

// contractCase is one scenario, expressed against the interface alone.
type contractCase struct {
	name string
	run  func(t *testing.T, tr Transport, root string, stage func(name, content string) string)
}

var contractCases = []contractCase{
	// No probe ID: this is the Transport INTERFACE's own contract line
	// ("absence is (_, false, nil), never an error"), not an observed CLI
	// behaviour. The two implementations reach it differently — *CLI absorbs
	// `filesystem info`'s non-zero exit for a missing node while carefully
	// NOT absorbing a CLI that never started (see CLI.Stat's code == -1
	// guard), and the Fake reports a map miss — so the case pins the shape
	// both must present, which is exactly what the shared table is for.
	{"stat absence is not an error", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		_, ok, err := tr.Stat(root + "/definitely-absent")
		if err != nil {
			t.Fatalf("absence must be (_, false, nil), got err %v", err)
		}
		if ok {
			t.Error("a node that was never created must not exist")
		}
	}},

	// Task 4: pins the not-found/error split itself, not just the interface-
	// level "absence is not an error" the row above already covers. Every
	// implementation must still report a missing path as (_, false, nil),
	// and — live half only — this ALSO captures the CLI's verbatim
	// `filesystem info` failure text and asserts it still contains
	// notFoundSignature (cli.go), so a future CLI build that changes its
	// not-found wording is caught here at the gate rather than silently
	// making every Stat failure read as "the CLI is broken" (the Stage 4
	// gate 2b masquerade this task fixed). The complementary half of the
	// split — some OTHER failure must be an error, never absence — is
	// deliberately NOT exercised through this shared table: the Fake has no
	// notion of a Stat failure at all (fake.go: a map miss is always
	// confirmed absence), and provoking an arbitrary non-not-found failure
	// against the LIVE account is not safe to do routinely. That half is
	// covered hermetically instead, by cli_test.go's role-based
	// TestCLIStatNonNotFoundFailureIsAnErrorNotAbsence (a stand-in CLI, no
	// live account) and by repo_test.go's
	// TestRequireMarkerSurfacesStatFailureDistinctlyFromNoMarker (a stub
	// Transport) — both hermetic and both required by Step 6's full suite.
	{"stat not-found is pinned against the certified CLI's own signature (Task 4)", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		missing := root + "/definitely-absent-t4-notfound-signature"
		if c, ok := tr.(*CLI); ok {
			out, code, runErr := c.run("filesystem", "info", missing, "--json")
			if code == 0 {
				t.Fatalf("expected a nonzero exit for a missing node, got 0 (out=%q, err=%v)", out, runErr)
			}
			t.Logf("live not-found output (must contain notFoundSignature %q): %q", notFoundSignature, out)
			if !strings.Contains(out, notFoundSignature) {
				t.Errorf("the live CLI's not-found text no longer contains notFoundSignature %q — "+
					"got %q; update the constant in cli.go before trusting the not-found/error split",
					notFoundSignature, out)
			}
		}
		_, ok, err := tr.Stat(missing)
		if err != nil {
			t.Fatalf("a missing node must be (_, false, nil), got err %v", err)
		}
		if ok {
			t.Error("a node that was never created must not exist")
		}
	}},

	// C11 (Task 2 review round 1): upload names the node after the LOCAL
	// basename, so the caller contract is that the local basename equals the
	// target leaf. Staging under a DIFFERENT name than the target leaf is
	// what actually exercises the guard — the review found that the original
	// version of this case staged under the SAME name as the leaf, so it
	// could not distinguish "names after the target leaf" from "names after
	// the local basename" and stayed green even with checkUploadBasename
	// deleted entirely. This version must go red if either implementation's
	// guard is removed.
	{"create refuses a local basename that mismatches the target leaf (C11)", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		local := stage("wrong-name.txt", "hello")
		out, err := tr.CreateExclusive(root+"/target-leaf.txt", local)
		if err == nil {
			t.Fatalf("a basename mismatch must be refused with a non-nil error, got outcome=%v err=nil", out)
		}
		if out == Committed {
			t.Errorf("a basename mismatch must not be Committed, got %v", out)
		}
		if _, ok, statErr := tr.Stat(root + "/target-leaf.txt"); statErr != nil || ok {
			t.Errorf("a refused mismatch must not create the node at the target leaf: ok=%v err=%v", ok, statErr)
		}
	}},

	// The honest happy path, split out from the case above: this one CAN
	// fail (e.g. if create silently landed at the wrong leaf), unlike its
	// predecessor which could not fail no matter what checkUploadBasename did.
	{"create lands at the target leaf when basenames agree", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		local := stage("leafname.txt", "hello")
		if out, err := tr.CreateExclusive(root+"/leafname.txt", local); err != nil || out != Committed {
			t.Fatalf("create: %v %v", out, err)
		}
		if _, ok, err := tr.Stat(root + "/leafname.txt"); err != nil || !ok {
			t.Fatalf("the node must exist under the target leaf: %v %v", ok, err)
		}
	}},

	// A6: a second create of the same name is refused, not overwritten.
	{"create refuses a name already taken", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		p := root + "/taken.txt"
		if out, err := tr.CreateExclusive(p, stage("taken.txt", "first")); err != nil || out != Committed {
			t.Fatalf("first create: %v %v", out, err)
		}
		out, err := tr.CreateExclusive(p, stage("taken.txt", "second"))
		if err != nil {
			t.Fatalf("second create errored: %v", err)
		}
		if out != Refused {
			t.Errorf("a taken name must be Refused, got %v", out)
		}
	}},

	// ReadTo's destination is a DIRECTORY, and the file lands under the
	// node's own remote basename. The Fake and the CLI disagreed on this.
	{"readTo lands under the remote basename in an existing dir", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		p := root + "/readback.txt"
		if out, err := tr.CreateExclusive(p, stage("readback.txt", "payload")); err != nil || out != Committed {
			t.Fatalf("create: %v %v", out, err)
		}
		dir := t.TempDir()
		if err := tr.ReadTo(p, dir); err != nil {
			t.Fatalf("ReadTo: %v", err)
		}
		got, err := os.ReadFile(filepath.Join(dir, "readback.txt"))
		if err != nil {
			t.Fatalf("the download must land under the remote basename: %v", err)
		}
		if string(got) != "payload" {
			t.Errorf("content = %q, want payload", got)
		}
	}},

	{"readTo into a missing directory errors and creates nothing", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		p := root + "/nodir.txt"
		if out, err := tr.CreateExclusive(p, stage("nodir.txt", "x")); err != nil || out != Committed {
			t.Fatalf("create: %v %v", out, err)
		}
		missing := filepath.Join(t.TempDir(), "not-created")
		if err := tr.ReadTo(p, missing); err == nil {
			t.Error("ReadTo must not create its destination directory")
		}
		if _, err := os.Stat(missing); !os.IsNotExist(err) {
			t.Error("the destination directory must still not exist")
		}
	}},

	// F1 (Stage 5 gate): `filesystem download` on a DIRECTORY exits 0 and
	// recursively downloads the whole subtree. sethead.go's Stat-IsDir-first
	// guard comment still calls this "NO verified contract at all" — that
	// comment names the live CLI's directory-download behaviour, not this
	// row, and is out of this task's file list, so it is left as is; this row
	// is what now actually pins the shape. Fake.ReadTo had no directory
	// behaviour at all before this row: a directory path missed on f.Files
	// and was misreported as an absent file. Fixture is the pinned F1 shape:
	// one folder containing one file and one subfolder with a file — both
	// must land, relative layout preserved under <dest>/<leaf>/.... Live half
	// records the verbatim output of the raw download call; both halves then
	// assert the same resulting layout.
	{"download of a directory recursively materialises the subtree (F1)", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		d := root + "/d"
		if err := tr.EnsureDir(d); err != nil {
			t.Fatalf("EnsureDir: %v", err)
		}
		if out, err := tr.CreateExclusive(d+"/a", stage("a", "A")); err != nil || out != Committed {
			t.Fatalf("seed create a: %v %v", out, err)
		}
		sub := d + "/sub"
		if err := tr.EnsureDir(sub); err != nil {
			t.Fatalf("EnsureDir: %v", err)
		}
		if out, err := tr.CreateExclusive(sub+"/b", stage("b", "B")); err != nil || out != Committed {
			t.Fatalf("seed create b: %v %v", out, err)
		}

		dest := t.TempDir()
		if c, ok := tr.(*CLI); ok {
			out, code, runErr := c.run("filesystem", "download", d, dest)
			if code != 0 {
				t.Fatalf("expected exit 0 downloading a directory (F1), got %d (out=%q, err=%v)", code, out, runErr)
			}
			t.Logf("live directory-download output (F1): %q", out)
		} else if err := tr.ReadTo(d, dest); err != nil {
			t.Fatalf("ReadTo on a directory: %v", err)
		}

		gotA, err := os.ReadFile(filepath.Join(dest, "d", "a"))
		if err != nil {
			t.Fatalf("expected downloaded file at dest/d/a: %v", err)
		}
		if string(gotA) != "A" {
			t.Errorf("dest/d/a = %q, want %q", gotA, "A")
		}
		gotB, err := os.ReadFile(filepath.Join(dest, "d", "sub", "b"))
		if err != nil {
			t.Fatalf("expected downloaded file at dest/d/sub/b: %v", err)
		}
		if string(gotB) != "B" {
			t.Errorf("dest/d/sub/b = %q, want %q", gotB, "B")
		}
	}},

	// C4: trash on a missing target is Committed — the desired end state is
	// "not there", and the CLI's own exit 1 is absorbed by the implementation.
	{"trash on a missing target is committed", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		out, err := tr.Trash(root + "/never-existed.txt")
		if err != nil {
			t.Fatalf("trash of an absent node errored: %v", err)
		}
		if out != Committed {
			t.Errorf("an already-absent node must be Committed, got %v", out)
		}
	}},

	// C5 + the Fake's own gap: an EnsureDir'd empty folder must be visible
	// to List. The Fake ignored f.Dirs and did not show it.
	{"ensureDir is idempotent and its result is listable", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		d := root + "/emptydir"
		if err := tr.EnsureDir(d); err != nil {
			t.Fatalf("EnsureDir: %v", err)
		}
		if err := tr.EnsureDir(d); err != nil {
			t.Fatalf("EnsureDir must be idempotent: %v", err)
		}
		nodes, err := tr.List(root)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, n := range nodes {
			if n.Name == "emptydir" {
				if !n.IsDir {
					t.Error("an EnsureDir'd node must list as a directory")
				}
				return
			}
		}
		t.Error("an EnsureDir'd empty directory must appear in its parent listing")
	}},

	// C6 + C10: an empty listing is exit 0 with zero parsed elements (C6), and
	// C10 pinned the literal bytes behind that — `[\r\n\r\n]\r\n`, 8 bytes,
	// which survives TrimSpace and parses as a zero-element array on the
	// normal path. So "empty" must present as an empty slice and a nil error,
	// never as a failure and never via CLI.List's truly-empty-output fallback,
	// which C10 showed is a defensive branch the real CLI does not reach.
	{"list of an empty directory is empty, not an error", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		d := root + "/emptylist"
		if err := tr.EnsureDir(d); err != nil {
			t.Fatalf("EnsureDir: %v", err)
		}
		nodes, err := tr.List(d)
		if err != nil {
			t.Fatalf("an empty listing must not be an error: %v", err)
		}
		if len(nodes) != 0 {
			t.Errorf("want an empty listing, got %d nodes", len(nodes))
		}
	}},

	// Task 7: folder fidelity. Fake half asserted now (fake.go's Trash and
	// EnsureDir were both bare, permissive implementations before this task);
	// the live half runs only at the gate (GPB_LIVE_ACCOUNT), never here.

	{"trash on an empty folder is committed and the folder is gone", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		d := root + "/trash-empty"
		if err := tr.EnsureDir(d); err != nil {
			t.Fatalf("EnsureDir: %v", err)
		}
		out, err := tr.Trash(d)
		if err != nil {
			t.Fatalf("Trash: %v", err)
		}
		if out != Committed {
			t.Errorf("Trash of an empty folder: want Committed, got %v", out)
		}
		if _, ok, statErr := tr.Stat(d); statErr != nil || ok {
			t.Errorf("the folder must be gone after Trash: ok=%v err=%v", ok, statErr)
		}
	}},

	{"trash on a folder with children removes the whole subtree", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		d := root + "/trash-subtree"
		if err := tr.EnsureDir(d); err != nil {
			t.Fatalf("EnsureDir: %v", err)
		}
		child := d + "/child.txt"
		if out, err := tr.CreateExclusive(child, stage("child.txt", "x")); err != nil || out != Committed {
			t.Fatalf("seed create: %v %v", out, err)
		}
		out, err := tr.Trash(d)
		if err != nil {
			t.Fatalf("Trash: %v", err)
		}
		if out != Committed {
			t.Errorf("Trash of a non-empty folder: want Committed, got %v", out)
		}
		if _, ok, statErr := tr.Stat(d); statErr != nil || ok {
			t.Errorf("the folder must be gone after Trash: ok=%v err=%v", ok, statErr)
		}
		if _, ok, statErr := tr.Stat(child); statErr != nil || ok {
			t.Errorf("a child under the trashed folder must be gone too (subtree removal): ok=%v err=%v", ok, statErr)
		}
	}},

	{"create-folder refuses a name already taken by a file", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		p := root + "/file-vs-folder"
		if out, err := tr.CreateExclusive(p, stage("file-vs-folder", "hello")); err != nil || out != Committed {
			t.Fatalf("seed create: %v %v", out, err)
		}
		if err := tr.EnsureDir(p); err == nil {
			t.Error("EnsureDir onto a path already occupied by a file must error")
		}
		if node, ok, statErr := tr.Stat(p); statErr != nil || !ok || node.IsDir {
			t.Errorf("the file must survive a refused EnsureDir, unchanged: node=%+v ok=%v err=%v", node, ok, statErr)
		}
	}},

	// Task 9b: pins the REAL text behind alreadyExistsSignature (cli.go) —
	// the C17b hypothesis constant EnsureDir's contradiction re-observation
	// keys on. This is NOT a reproduction of the C17 race itself (observed
	// once live under genuine check-then-create timing, never reproduced
	// under deliberate provocation — docs/research/probes/
	// c17b-provocation-log.md); it only confirms that create-folder's own
	// ordinary already-exists wording, genuinely provoked here by calling
	// create-folder on a folder EnsureDir already made, still contains
	// "already exists" on the certified build. If this ever stops matching,
	// EnsureDir's contradiction re-observation keys on a constant the live
	// CLI no longer uses, and this row is what catches that before a real
	// contradiction is ever misdiagnosed.
	{"create-folder on an existing folder reports the already-exists signature (Task 9b, C17b)",
		func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
			d := root + "/already-there"
			if err := tr.EnsureDir(d); err != nil {
				t.Fatalf("EnsureDir: %v", err)
			}
			if c, ok := tr.(*CLI); ok {
				out, code, runErr := c.run("filesystem", "create-folder", root, "already-there")
				if code == 0 {
					t.Fatalf("expected a nonzero exit for create-folder on an existing folder, "+
						"got 0 (out=%q, err=%v)", out, runErr)
				}
				t.Logf("live already-exists output (must contain alreadyExistsSignature %q): %q",
					alreadyExistsSignature, out)
				if !strings.Contains(out, alreadyExistsSignature) {
					t.Errorf("the live CLI's already-exists text no longer contains "+
						"alreadyExistsSignature %q — got %q; update the constant in cli.go "+
						"before trusting EnsureDir's contradiction re-observation",
						alreadyExistsSignature, out)
				}
			}
			// Hermetic half, both implementations: EnsureDir itself must stay
			// idempotent regardless of what the raw create-folder call above did.
			if err := tr.EnsureDir(d); err != nil {
				t.Errorf("EnsureDir must remain idempotent on an existing folder: %v", err)
			}
		}},

	// Upload of a file colliding with an existing folder name. UNVERIFIED ON
	// THE REAL CLI UNTIL THE GATE (Task 7 brief) — the Fake models the
	// conservative reading, Refused with no error, mirroring the D/F
	// collision rule CreateExclusive already applies to a name taken by a
	// FILE. The assertion below only pins the invariant every reading must
	// satisfy regardless of the live CLI's exact outcome/error shape: the
	// upload must not silently succeed as Committed, and the folder must
	// survive.
	{"upload of a file colliding with an existing folder name does not silently succeed", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		d := root + "/folder-vs-file"
		if err := tr.EnsureDir(d); err != nil {
			t.Fatalf("EnsureDir: %v", err)
		}
		out, err := tr.CreateExclusive(d, stage("folder-vs-file", "hello"))
		if err == nil && out == Committed {
			t.Errorf("a name already taken by a folder must not silently succeed as Committed")
		}
		if node, ok, statErr := tr.Stat(d); statErr != nil || !ok || !node.IsDir {
			t.Errorf("the folder must survive an upload attempt onto its name: node=%+v ok=%v err=%v", node, ok, statErr)
		}
	}},

	{"nested list reports folders and files with the correct IsDir", func(t *testing.T, tr Transport, root string, stage func(string, string) string) {
		d := root + "/nested"
		if err := tr.EnsureDir(d); err != nil {
			t.Fatalf("EnsureDir: %v", err)
		}
		sub := d + "/sub"
		if err := tr.EnsureDir(sub); err != nil {
			t.Fatalf("EnsureDir: %v", err)
		}
		if out, err := tr.CreateExclusive(d+"/leaf.txt", stage("leaf.txt", "x")); err != nil || out != Committed {
			t.Fatalf("seed create: %v %v", out, err)
		}
		nodes, err := tr.List(d)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		var gotDir, gotFile bool
		for _, n := range nodes {
			switch n.Name {
			case "sub":
				if !n.IsDir {
					t.Errorf("sub must list as a directory, got %+v", n)
				}
				gotDir = true
			case "leaf.txt":
				if n.IsDir {
					t.Errorf("leaf.txt must list as a file, got %+v", n)
				}
				gotFile = true
			}
		}
		if !gotDir || !gotFile {
			t.Errorf("want both a folder entry (sub) and a file entry (leaf.txt), got %+v", nodes)
		}
	}},
}

// runContract executes the table against one implementation. root must already
// exist and be empty; stage writes a local file under a given basename.
func runContract(t *testing.T, newTransport func(t *testing.T) (Transport, string)) {
	for _, c := range contractCases {
		t.Run(c.name, func(t *testing.T) {
			tr, root := newTransport(t)
			dir := t.TempDir()
			stage := func(name, content string) string {
				p := filepath.Join(dir, name)
				if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
				return p
			}
			c.run(t, tr, root, stage)
		})
	}
}

func TestContractFake(t *testing.T) {
	runContract(t, func(t *testing.T) (Transport, string) {
		f := NewFake()
		root := "/r"
		// "/r"'s parent, "/", is not a mount root (only "/my-files" and
		// "/devices" are — Task 7), so the stricter EnsureDir needs it seeded
		// as already-existing; the subsequent EnsureDir call below is then an
		// idempotent no-op, same as it would be against a root a real
		// Bootstrap had already created.
		f.Dirs[root] = true
		if err := f.EnsureDir(root); err != nil {
			t.Fatal(err)
		}
		return f, root
	})
}

func TestContractCLI(t *testing.T) {
	if os.Getenv(liveEnv) == "" {
		// LOUD, never silent. Stage 2.1 had a guard rot into a no-op skip,
		// and a silent skip in the table that exists to prevent silent drift
		// would be self-defeating.
		t.Skipf("SKIPPING THE LIVE HALF of the Transport contract table. "+
			"The *Fake half ran; *CLI did not, so fake/real drift is NOT covered by this run. "+
			"Set %s=1 to run it. It writes only under %s and trashes that root afterwards.",
			liveEnv, liveRoot)
	}
	// Validated fail-closed BEFORE any live call — including before the
	// Version() probe below — so an invalid GPB_CONTRACT_LIVE_ROOT never
	// reaches the account at all.
	base := contractLiveRoot(t)
	runContract(t, func(t *testing.T) (Transport, string) {
		c := NewCLI("")
		v, err := c.Version()
		if err != nil {
			t.Fatalf("the live half needs a working proton-drive: %v", err)
		}
		if !IsCertified(v) {
			t.Fatalf("live half refuses an uncertified CLI: got %q, certified is %s", v, CertifiedCLI)
		}
		// A per-test root so cases cannot see each other's nodes.
		root := base + "/" + strings.ReplaceAll(t.Name(), "/", "_")
		if err := c.EnsureDir(base); err != nil {
			t.Fatalf("EnsureDir %s: %v", base, err)
		}
		if err := c.EnsureDir(root); err != nil {
			t.Fatalf("EnsureDir %s: %v", root, err)
		}
		t.Cleanup(func() {
			if out, err := c.Trash(root); err != nil {
				t.Errorf("CLEANUP FAILED for %s (outcome %s): %v — remove it by hand", root, out, err)
			}
		})
		return c, root
	})
}
