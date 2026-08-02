package transport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// liveEnv gates the *CLI half. CI never sets it, so `go test ./...` on a
// runner exercises only the Fake and can never touch a real account.
const liveEnv = "GPB_LIVE_ACCOUNT"

// liveRoot is the only remote path the live half may write to.
const liveRoot = "/my-files/_cas-probe/contract"

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
		root := liveRoot + "/" + strings.ReplaceAll(t.Name(), "/", "_")
		if err := c.EnsureDir(liveRoot); err != nil {
			t.Fatalf("EnsureDir %s: %v", liveRoot, err)
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
