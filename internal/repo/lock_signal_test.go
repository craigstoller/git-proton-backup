package repo

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// Tests for the in-doubt lock handle, the process-wide register and the typed
// release (spec §5.4, §5.5). Every test that looks at the register calls
// cleanRegister FIRST as well as at cleanup: earlier tests in this package
// can leave a lock registered (one whose release failed, say), and package
// tests run in file order, so a register assertion that trusted the previous
// test to have tidied up would fail for reasons that have nothing to do with
// what it checks.

const (
	sigRoot = "/my-files/r"
	sigLock = sigRoot + "/" + LockName
)

// errKilled stands in for the transport error a process killed mid-upload
// would report: the upload may or may not have landed.
var errKilled = errors.New("killed")

func cleanRegister(t *testing.T) {
	t.Helper()
	ResetRegister()
	t.Cleanup(ResetRegister)
}

// replaceSettle swaps the settle seam for the test. Release and the exported
// Settle both go through it, so one swap covers both paths.
func replaceSettle(t *testing.T, fn func()) {
	t.Helper()
	old := settle
	settle = fn
	t.Cleanup(func() { settle = old })
}

// foreignLockBody is a healthy lock that belongs to some other process.
func foreignLockBody() []byte {
	return []byte(`{"nonce":"someone-else","host":"h","pid":1,"acquiredAt":"2026-01-01T00:00:00Z"}`)
}

// ourLockBody is the body l's own upload would have written: what a late
// landing puts on the remote.
func ourLockBody(t *testing.T, l *Lock) []byte {
	t.Helper()
	b, err := json.Marshal(lockBody{Nonce: l.Nonce(), Host: "h", PID: 1, AcquiredAt: "2026-01-01T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// callLogTransport records the name of every call made through it, in order.
// repo_test.go's countingTransport counts only the write methods per path;
// the refusal test needs the whole sequence, reads included.
type callLogTransport struct {
	transport.Transport
	calls []string
}

func (c *callLogTransport) EnsureDir(p string) error {
	c.calls = append(c.calls, "EnsureDir")
	return c.Transport.EnsureDir(p)
}

func (c *callLogTransport) List(p string) ([]transport.Node, error) {
	c.calls = append(c.calls, "List")
	return c.Transport.List(p)
}

func (c *callLogTransport) Stat(p string) (transport.Node, bool, error) {
	c.calls = append(c.calls, "Stat")
	return c.Transport.Stat(p)
}

func (c *callLogTransport) ReadTo(p, local string) error {
	c.calls = append(c.calls, "ReadTo")
	return c.Transport.ReadTo(p, local)
}

func (c *callLogTransport) CreateExclusive(p, local string) (transport.Outcome, error) {
	c.calls = append(c.calls, "CreateExclusive")
	return c.Transport.CreateExclusive(p, local)
}

func (c *callLogTransport) UpdateRevision(p, local string) (transport.Outcome, error) {
	c.calls = append(c.calls, "UpdateRevision")
	return c.Transport.UpdateRevision(p, local)
}

func (c *callLogTransport) Trash(p string) (transport.Outcome, error) {
	c.calls = append(c.calls, "Trash")
	return c.Transport.Trash(p)
}

// errOnCreateTransport is a process killed mid-upload: the body reaches the
// remote, and the caller gets an error rather than an outcome.
type errOnCreateTransport struct {
	*transport.Fake
}

func (e errOnCreateTransport) CreateExclusive(p, local string) (transport.Outcome, error) {
	e.Fake.CreateExclusive(p, local)
	return transport.Ambiguous, errKilled
}

// ambiguousNilTransport reports Ambiguous with a nil error and writes nothing:
// the upload may still be in flight and land later.
type ambiguousNilTransport struct {
	*transport.Fake
}

func (a ambiguousNilTransport) CreateExclusive(p, local string) (transport.Outcome, error) {
	return transport.Ambiguous, nil
}

// refusedCreateTransport reports Refused with a nil error and writes nothing:
// the name was taken and has vanished again by the time anyone looks.
type refusedCreateTransport struct {
	*transport.Fake
}

func (r refusedCreateTransport) CreateExclusive(p, local string) (transport.Outcome, error) {
	return transport.Refused, nil
}

// hookCreateTransport runs hook at the start of CreateExclusive, then writes.
type hookCreateTransport struct {
	*transport.Fake
	hook func()
}

func (h hookCreateTransport) CreateExclusive(p, local string) (transport.Outcome, error) {
	h.hook()
	return h.Fake.CreateExclusive(p, local)
}

// readBackErrorTransport lets the upload land and then fails the read-back
// download with an I/O error.
type readBackErrorTransport struct {
	*transport.Fake
}

func (r readBackErrorTransport) ReadTo(p, local string) error {
	return errors.New("read-back disk failure")
}

// rewriteAfterCreateTransport lets the upload report Committed and then leaves
// body at the path instead of what was uploaded: a read-back that finds
// something other than our own lock.
type rewriteAfterCreateTransport struct {
	*transport.Fake
	body []byte
}

func (r rewriteAfterCreateTransport) CreateExclusive(p, local string) (transport.Outcome, error) {
	out, err := r.Fake.CreateExclusive(p, local)
	if err != nil || out != transport.Committed {
		return out, err
	}
	r.Fake.Files[p] = r.body
	return transport.Committed, nil
}

// refusedTrashTransport answers Trash with Refused and a nil error: not an
// error, and not proof the lock is gone.
type refusedTrashTransport struct {
	*transport.Fake
}

func (r refusedTrashTransport) Trash(p string) (transport.Outcome, error) {
	return transport.Refused, nil
}

func registeredHas(l *Lock) bool {
	for _, r := range RegisteredLocks() {
		if r == l {
			return true
		}
	}
	return false
}

// GUARD of the ordering, RED for the register: a signal can land at any point
// in the upload, so the lock must already be findable when CreateExclusive
// starts, not after it returns.
func TestAcquireLockRegistersBeforeTheUpload(t *testing.T) {
	cleanRegister(t)
	f := transport.NewFake()
	var seen []*Lock
	tr := hookCreateTransport{Fake: f, hook: func() { seen = RegisteredLocks() }}

	l, err := AcquireLock(tr, sigRoot)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if len(seen) != 1 {
		t.Fatalf("the register held %d locks when the upload began, want 1", len(seen))
	}
	if seen[0].Path() != sigLock {
		t.Errorf("registered path = %q, want %q", seen[0].Path(), sigLock)
	}
	if seen[0] != l {
		t.Error("the lock registered before the upload must be the one AcquireLock returns")
	}
	if l.InDoubt() {
		t.Error("a Committed upload that read back as ours is a held lock, not an in-doubt one")
	}
	if !registeredHas(l) {
		t.Error("a held lock stays registered until it is released")
	}
}

func TestAcquireLockRegistersNothingWhenStagingFails(t *testing.T) {
	cleanRegister(t)
	old := stageFile
	stageFile = func(content []byte, leafName string) (string, func(), error) {
		return "", func() {}, errors.New("staging failed")
	}
	t.Cleanup(func() { stageFile = old })

	c := &callLogTransport{Transport: transport.NewFake()}
	l, err := AcquireLock(c, sigRoot)
	if err == nil {
		t.Fatal("a staging failure must be an error")
	}
	if l != nil {
		t.Error("no handle may be returned when nothing was ever uploaded")
	}
	if n := len(RegisteredLocks()); n != 0 {
		t.Errorf("the register holds %d locks after a staging failure, want 0", n)
	}
	if len(c.calls) != 0 {
		t.Errorf("a staging failure must not touch the transport, calls = %v", c.calls)
	}
}

// TestAcquireLockRefusedMakesTodaysCallsAndNoHandle: a refusal is not in
// doubt (nothing of ours was written), so it hands back no handle, leaves the
// register empty, and costs exactly the calls it cost before this change. The
// sequences below were measured on the unchanged code (6e8ebf7, 2026-10-25) by
// logging every Transport call through AcquireLock's Refused path: 3 calls
// for a healthy or an unreadable holder, 2 when the lock vanished again.
func TestAcquireLockRefusedMakesTodaysCallsAndNoHandle(t *testing.T) {
	cases := []struct {
		name      string
		seed      []byte // lock already on the remote; nil for none
		wrap      func(f *transport.Fake) transport.Transport
		wantCalls []string
		wantErr   string
	}{
		{"held by another", foreignLockBody(),
			func(f *transport.Fake) transport.Transport { return f },
			[]string{"CreateExclusive", "Stat", "ReadTo"}, "repo is locked by nonce someone-else"},
		{"unreadable holder", []byte("not json"),
			func(f *transport.Fake) transport.Transport { return f },
			[]string{"CreateExclusive", "Stat", "ReadTo"}, "unreadable"},
		{"vanished again", nil,
			func(f *transport.Fake) transport.Transport { return refusedCreateTransport{f} },
			[]string{"CreateExclusive", "Stat"}, "repo is locked; remove"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cleanRegister(t)
			f := transport.NewFake()
			if tc.seed != nil {
				f.Files[sigLock] = tc.seed
			}
			c := &callLogTransport{Transport: tc.wrap(f)}

			l, err := AcquireLock(c, sigRoot)
			if err == nil {
				t.Fatal("a refused acquisition must be an error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %v, want it to contain %q", err, tc.wantErr)
			}
			if l != nil {
				t.Error("a refusal must not hand back a handle: nothing of ours is on the remote")
			}
			if n := len(RegisteredLocks()); n != 0 {
				t.Errorf("the register holds %d locks after a refusal, want 0", n)
			}
			if !reflect.DeepEqual(c.calls, tc.wantCalls) {
				t.Errorf("calls = %v, want exactly today's %v", c.calls, tc.wantCalls)
			}
			if tc.seed != nil && string(f.Files[sigLock]) != string(tc.seed) {
				t.Error("the other holder's lock must be left exactly as it was")
			}
		})
	}
}

func TestAcquireLockInDoubtReturnsHandleAndError(t *testing.T) {
	cases := []struct {
		name    string
		wrap    func(f *transport.Fake) transport.Transport
		wantErr string
	}{
		{"error from the upload",
			func(f *transport.Fake) transport.Transport { return errOnCreateTransport{f} }, "killed"},
		{"ambiguous with a nil error",
			func(f *transport.Fake) transport.Transport { return ambiguousNilTransport{f} },
			"lock acquisition ambiguous; re-run to reconcile"},
		{"unrecognised outcome",
			func(f *transport.Fake) transport.Transport {
				return unknownOutcomeTransport{Fake: f, forPath: sigLock}
			}, "outcome(42)"},
		{"read-back I/O error",
			func(f *transport.Fake) transport.Transport { return readBackErrorTransport{f} },
			"read-back disk failure"},
		{"unparseable read-back",
			func(f *transport.Fake) transport.Transport {
				return rewriteAfterCreateTransport{Fake: f, body: []byte("not json")}
			}, "lock read-back mismatch"},
		{"foreign read-back",
			func(f *transport.Fake) transport.Transport {
				return rewriteAfterCreateTransport{Fake: f, body: foreignLockBody()}
			}, "lock read-back mismatch"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cleanRegister(t)
			l, err := AcquireLock(tc.wrap(transport.NewFake()), sigRoot)
			if err == nil {
				t.Fatal("an acquisition that is not a clean hold must be an error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %v, want it to contain %q", err, tc.wantErr)
			}
			if l == nil {
				t.Fatal("the lock may be on the remote: a handle is needed to release it")
			}
			if !l.InDoubt() {
				t.Error("the handle must be marked in doubt")
			}
			if l.Path() != sigLock || l.Nonce() == "" {
				t.Errorf("handle = (%q, %q), want the lock path and a nonce", l.Path(), l.Nonce())
			}
			if reg := RegisteredLocks(); len(reg) != 1 || reg[0] != l {
				t.Errorf("the register must hold exactly the in-doubt handle, got %d locks", len(reg))
			}
		})
	}
	t.Run("the error is the transport's own", func(t *testing.T) {
		cleanRegister(t)
		_, err := AcquireLock(errOnCreateTransport{transport.NewFake()}, sigRoot)
		if !errors.Is(err, errKilled) {
			t.Errorf("error = %v, want it to wrap the transport error unchanged", err)
		}
	})
}

func TestReleaseViaOutcomes(t *testing.T) {
	cases := []struct {
		name       string
		mutate     func(f *transport.Fake) // the remote state, after our lock is held
		release    func(f *transport.Fake) transport.Transport
		want       ReleaseOutcome
		wantErr    string // "" means a nil error
		wantGone   bool
		wantInReg  bool
		retryFixes bool // a Failed outcome: a retry on a healthy transport must finish the job
	}{
		{"removed", func(f *transport.Fake) {},
			func(f *transport.Fake) transport.Transport { return f },
			Removed, "", true, false, false},
		{"absent", func(f *transport.Fake) { delete(f.Files, sigLock) },
			func(f *transport.Fake) transport.Transport { return f },
			Absent, "", true, false, false},
		{"foreign", func(f *transport.Fake) { f.Files[sigLock] = foreignLockBody() },
			func(f *transport.Fake) transport.Transport { return f },
			Foreign, "", false, false, false},
		{"unreadable", func(f *transport.Fake) { f.Files[sigLock] = []byte("not json") },
			func(f *transport.Fake) transport.Transport { return f },
			Unreadable, "present but its contents are unreadable", false, false, false},
		{"failed: ambiguous trash", func(f *transport.Fake) {},
			func(f *transport.Fake) transport.Transport { return ambiguousTrashTransport{f} },
			Failed, "is unconfirmed (Trash reported ambiguous, not committed)", false, true, true},
		{"failed: refused trash", func(f *transport.Fake) {},
			func(f *transport.Fake) transport.Transport { return refusedTrashTransport{f} },
			Failed, "is unconfirmed (Trash reported refused, not committed)", false, true, true},
		{"failed: stat error", func(f *transport.Fake) {},
			func(f *transport.Fake) transport.Transport { return statErrorTransport{f} },
			Failed, "simulated transport failure statting", false, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cleanRegister(t)
			f := transport.NewFake()
			acq := &callLogTransport{Transport: f}
			l, err := AcquireLock(acq, sigRoot)
			if err != nil {
				t.Fatalf("acquire: %v", err)
			}
			tc.mutate(f)
			acquireCalls := len(acq.calls)
			before, hadFile := f.Files[sigLock]

			out, err := l.ReleaseVia(tc.release(f))

			if out != tc.want {
				t.Errorf("outcome = %s, want %s", out, tc.want)
			}
			if tc.wantErr == "" && err != nil {
				t.Errorf("error = %v, want nil", err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Errorf("error = %v, want it to contain %q", err, tc.wantErr)
			}
			if out.Final() == (tc.want == Failed) {
				t.Errorf("Final() = %v for %s", out.Final(), out)
			}
			got, hasFile := f.Files[sigLock]
			if tc.wantGone && hasFile {
				t.Error("the lock must be gone")
			}
			if !tc.wantGone && (!hasFile || string(got) != string(before) || !hadFile) {
				t.Error("a lock that was not removed must be left exactly as it was")
			}
			if registeredHas(l) != tc.wantInReg {
				t.Errorf("registered = %v, want %v", registeredHas(l), tc.wantInReg)
			}
			if len(acq.calls) != acquireCalls {
				t.Errorf("ReleaseVia used the lock's own transport (%d extra calls); it must use the one it was given",
					len(acq.calls)-acquireCalls)
			}
			if tc.retryFixes {
				out2, err2 := l.ReleaseVia(f)
				if out2 != Removed || err2 != nil {
					t.Errorf("retry on a healthy transport = (%s, %v), want (removed, nil)", out2, err2)
				}
				if registeredHas(l) {
					t.Error("the final outcome of the retry must unregister the lock")
				}
			}
		})
	}
}

func TestReleaseOutcomeStringAndFinal(t *testing.T) {
	cases := []struct {
		o     ReleaseOutcome
		s     string
		final bool
	}{
		{Removed, "removed", true},
		{Absent, "absent", true},
		{Foreign, "foreign", true},
		{Unreadable, "unreadable", true},
		{Failed, "failed", false},
	}
	for _, c := range cases {
		if c.o.String() != c.s || c.o.Final() != c.final {
			t.Errorf("outcome %d = (%q, final %v), want (%q, final %v)",
				int(c.o), c.o.String(), c.o.Final(), c.s, c.final)
		}
	}
	// The zero value and any out-of-range value must never read as a release
	// that happened.
	var zero ReleaseOutcome
	if zero.Final() || zero.String() == "removed" {
		t.Errorf("the zero ReleaseOutcome = (%q, final %v); it must not claim a release", zero.String(), zero.Final())
	}
	if o := ReleaseOutcome(99); o.Final() || o.String() == "removed" {
		t.Errorf("an unknown ReleaseOutcome = (%q, final %v); it must not claim a release", o.String(), o.Final())
	}
}

// The late landing: an upload that was in flight when the process died can
// still arrive after the first look. Release of an in-doubt handle waits first,
// then reads, so the lock that arrived is found and removed. The seam below
// lands our body DURING the settle and nowhere else, so the lock is gone at
// the end only if Release settled before it read.
func TestReleaseOfAnInDoubtHandleSettlesFirst(t *testing.T) {
	cleanRegister(t)
	f := transport.NewFake()
	l, err := AcquireLock(ambiguousNilTransport{f}, sigRoot)
	if err == nil || l == nil || !l.InDoubt() {
		t.Fatalf("setup: want an in-doubt handle and an error, got (%v, %v)", l, err)
	}
	settled := 0
	replaceSettle(t, func() {
		settled++
		f.Files[l.Path()] = ourLockBody(t, l)
	})

	if err := l.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}
	if settled != 1 {
		t.Errorf("settle ran %d times, want 1", settled)
	}
	if _, ok := f.Files[sigLock]; ok {
		t.Error("the late-landing lock must be gone: Release must settle before it reads")
	}
	if registeredHas(l) {
		t.Error("a removed lock must leave the register")
	}
}

func TestReleaseOfAHeldHandleDoesNotSettle(t *testing.T) {
	cleanRegister(t)
	f := transport.NewFake()
	l, err := AcquireLock(f, sigRoot)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	replaceSettle(t, func() { t.Error("a held handle has nothing in flight; Release must not wait") })

	if err := l.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, ok := f.Files[sigLock]; ok {
		t.Error("lock file must be gone after release")
	}
}

func TestSettleRunsTheSeam(t *testing.T) {
	n := 0
	replaceSettle(t, func() { n++ })
	Settle()
	if n != 1 {
		t.Errorf("Settle() ran the seam %d times, want 1", n)
	}
}

// An in-doubt handle whose read-back found someone else's lock (or something
// unparseable) must never be the licence to trash it: the nonce-first rule
// holds for the settled release exactly as it does for a held one.
func TestReleaseOfAnInDoubtHandleNeverTrashesWhatIsNotOurs(t *testing.T) {
	cases := []struct {
		name    string
		body    []byte
		wantErr string
	}{
		{"foreign", foreignLockBody(), ""},
		{"unreadable", []byte("not json"), "unreadable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cleanRegister(t)
			f := transport.NewFake()
			l, err := AcquireLock(rewriteAfterCreateTransport{Fake: f, body: tc.body}, sigRoot)
			if err == nil || l == nil || !l.InDoubt() {
				t.Fatalf("setup: want an in-doubt handle and an error, got (%v, %v)", l, err)
			}

			rerr := l.Release()

			if tc.wantErr == "" && rerr != nil {
				t.Errorf("release = %v, want nil for a lock that is not ours", rerr)
			}
			if tc.wantErr != "" && (rerr == nil || !strings.Contains(rerr.Error(), tc.wantErr)) {
				t.Errorf("release = %v, want an error containing %q", rerr, tc.wantErr)
			}
			if string(f.Files[sigLock]) != string(tc.body) {
				t.Error("a lock that is not ours must be left exactly as it was")
			}
		})
	}
}

// A signal handler reads the register from its own goroutine while the main
// one acquires and releases. Meaningful under -race; without it, it still
// proves the snapshot is never torn by a concurrent writer.
func TestRegisterIsSafeForConcurrentUse(t *testing.T) {
	cleanRegister(t)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			for _, l := range RegisteredLocks() {
				_ = l.Path()
				_ = l.Nonce()
				_ = l.InDoubt()
			}
		}
	}()
	for i := 0; i < 25; i++ {
		l, err := AcquireLock(transport.NewFake(), sigRoot)
		if err != nil {
			t.Fatalf("acquire: %v", err)
		}
		if err := l.Release(); err != nil {
			t.Fatalf("release: %v", err)
		}
	}
	close(stop)
	wg.Wait()
	if n := len(RegisteredLocks()); n != 0 {
		t.Errorf("the register holds %d locks after every one was released, want 0", n)
	}
}

func TestRegisteredLocksIsASnapshotCopy(t *testing.T) {
	cleanRegister(t)
	l, err := AcquireLock(transport.NewFake(), sigRoot)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	snap := RegisteredLocks()
	if len(snap) != 1 || snap[0] != l {
		t.Fatalf("snapshot = %v, want exactly the held lock", snap)
	}
	snap[0] = nil // a caller scribbling on its copy must not reach the register
	if reg := RegisteredLocks(); len(reg) != 1 || reg[0] != l {
		t.Error("RegisteredLocks must return a copy; writing to it changed the register")
	}
	ResetRegister()
	if n := len(RegisteredLocks()); n != 0 {
		t.Errorf("ResetRegister left %d locks", n)
	}
}

// SetHead acquires its own lock and releases it on every exit path. An
// acquisition that comes back in doubt (the process may have died mid-upload
// and the lock landed anyway) must still be released by that same defer,
// while the operation's own error is what SetHead reports.
func TestSetHeadReleasesAnInDoubtHandle(t *testing.T) {
	cleanRegister(t)
	f := transport.NewFake()
	f.Dirs["/r"] = true
	if err := Bootstrap(f, "/r"); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteRef(f, "/r", "refs/heads/main", "1111111111111111111111111111111111111111", false); err != nil {
		t.Fatal(err)
	}

	_, err := SetHead(errOnCreateTransport{f}, "/r", "main")

	if !errors.Is(err, errKilled) {
		t.Fatalf("SetHead error = %v, want the acquisition's own error", err)
	}
	if _, ok := f.Files["/r/"+LockName]; ok {
		t.Error("SetHead must release the in-doubt lock it never confirmed")
	}
	if n := len(RegisteredLocks()); n != 0 {
		t.Errorf("the register holds %d locks after SetHead returned, want 0", n)
	}
}
