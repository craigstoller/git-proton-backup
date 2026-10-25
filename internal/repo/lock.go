package repo

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/craigstoller/git-proton-backup/internal/transport"
)

type lockBody struct {
	Nonce      string `json:"nonce"`
	Host       string `json:"host"`
	PID        int    `json:"pid"`
	AcquiredAt string `json:"acquiredAt"`
}

type Lock struct {
	t     transport.Transport
	path  string
	nonce string
	// inDoubt: the upload may have landed without us knowing. Atomic because
	// AcquireLock clears it on the main goroutine while the shutdown
	// coordinator may already be reading the registered lock.
	inDoubt atomic.Bool
}

func (l *Lock) Path() string  { return l.path }
func (l *Lock) Nonce() string { return l.nonce }

// InDoubt reports that this handle was returned without proof the lock is
// held: the upload's result was lost, unrecognised, or read back as something
// other than our own body. The lock may or may not be on the remote.
func (l *Lock) InDoubt() bool { return l.inDoubt.Load() }

// SettleDelay is how long an in-doubt handle waits before its release reads
// the remote. A request a killed or failed child had already sent can still be
// applied by the server after the child ends; the wait lets it land first, so
// a late lock upload is seen and removed rather than read as absent and left
// to leak. A bounded wait, not a proof that nothing more will land.
var SettleDelay = 5 * time.Second

// settle is the seam tests replace; Settle is the one wait both the normal
// path (Release) and the shutdown coordinator go through.
var settle = func() { time.Sleep(SettleDelay) }

// Settle waits out SettleDelay.
func Settle() { settle() }

// stageFile is the seam for AcquireLock's staging step.
var stageFile = stagedFile

// ReleaseOutcome is what a release attempt is known to have done. The zero
// value is Failed, deliberately: a ReleaseOutcome that was never assigned must
// not read as a release that happened.
type ReleaseOutcome int

const (
	Failed     ReleaseOutcome = iota // a transport error, or a Trash that did not commit; not final
	Removed                          // .lock held our nonce and Trash committed
	Absent                           // no .lock: never landed, or already removed
	Foreign                          // .lock holds another nonce; left alone
	Unreadable                       // .lock is present but did not parse; left alone
)

func (o ReleaseOutcome) String() string {
	switch o {
	case Failed:
		return "failed"
	case Removed:
		return "removed"
	case Absent:
		return "absent"
	case Foreign:
		return "foreign"
	case Unreadable:
		return "unreadable"
	default:
		return fmt.Sprintf("release-outcome(%d)", int(o))
	}
}

// Final reports that the attempt reached an answer, so there is nothing to
// retry. Only the four known final outcomes qualify: Failed, and any value
// nobody recognises, are not final.
func (o ReleaseOutcome) Final() bool {
	switch o {
	case Removed, Absent, Foreign, Unreadable:
		return true
	}
	return false
}

// lockPresence distinguishes "nothing at path" from "something at path we
// could not parse". Conflating the two is exactly the bug this type exists
// to prevent: a present-but-corrupt .lock must never be treated as absent,
// because that lets Release report success without deleting anything, and
// lets AcquireLock treat someone else's damaged lock as free to take.
type lockPresence int

const (
	lockAbsent     lockPresence = iota // Stat says nothing is there
	lockPresent                        // Stat says present, and it parsed
	lockUnreadable                     // Stat says present, but the body did not parse as JSON
)

// AcquireLock returns one of three things. A held lock and a nil error: the
// upload committed and the read-back shows our nonce. No handle and an error:
// nothing of ours reached the remote (staging failed, or another holder's lock
// refused us). Or a handle marked in doubt TOGETHER WITH the error: the lock
// may or may not be on the remote, and the caller must still release the
// handle (after the settle wait, which Release applies). The lock is entered in
// the register before the upload starts. A refusal takes it out again at once;
// anything else keeps it there until a release reaches a final outcome.
func AcquireLock(t transport.Transport, root string) (*Lock, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	nonce := hex.EncodeToString(b)
	host, _ := os.Hostname()
	body, _ := json.Marshal(lockBody{nonce, host, os.Getpid(), time.Now().UTC().Format(time.RFC3339)})

	// Staged under the lock's own leaf name: the CLI names the uploaded node
	// after the LOCAL basename (probe C11), so it must match.
	staged, cleanup, err := stageFile(body, LockName)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	p := root + "/" + LockName
	// Registered, and in doubt, BEFORE the upload starts: a signal can arrive at
	// any point inside CreateExclusive, and the coordinator can only release
	// what it can find. The flag clears only once the read-back below proves
	// the lock ours.
	l := &Lock{t: t, path: p, nonce: nonce}
	l.inDoubt.Store(true)
	register(l)

	out, err := t.CreateExclusive(p, staged)
	if err != nil {
		// A killed upload lands here, as Ambiguous with an error: it may still
		// have been applied. In doubt, not failed.
		return l, err
	}
	switch out {
	case transport.Committed:
		// Falls out of the switch into the read-back verification below.
		// Named rather than left implicit so the default arm can exist at all:
		// without it, an unrecognised Outcome reached that verification and,
		// if the remote happened to hold a lock with our nonce, would be
		// reported as a lock we hold on the strength of a value nobody
		// recognised.
	case transport.Refused:
		// Coherent per presence, not a fallthrough: a corrupt lock is not the
		// same situation as a healthy one or a lock that vanished mid-race,
		// and the operator needs to know which they are looking at before
		// touching anything by hand.
		//
		// A refusal means the name was taken and nothing of ours was written:
		// the entry leaves the register and no handle comes back, before the
		// diagnostic read so a signal during it cannot find a lock that is not
		// ours.
		unregister(l)
		held, presence, _ := readLock(t, p)
		switch presence {
		case lockPresent:
			return nil, fmt.Errorf("repo is locked by nonce %s on %s (pid %d) since %s; "+
				"if that process is gone, remove %s with the Proton CLI",
				held.Nonce, held.Host, held.PID, held.AcquiredAt, p)
		case lockUnreadable:
			return nil, fmt.Errorf("repo is locked by an unreadable %s; "+
				"inspect it with the Proton CLI and remove it manually if stale", p)
		default: // lockAbsent: Refused but nothing there now — a concurrent release raced us
			return nil, fmt.Errorf("repo is locked; remove %s with the Proton CLI if stale", p)
		}
	case transport.Ambiguous:
		return l, fmt.Errorf("lock acquisition ambiguous; re-run to reconcile")
	default:
		return l, fmt.Errorf("lock acquisition for %s returned an unrecognised outcome %s; "+
			"refusing to guess whether the lock is held", p, out)
	}

	// Verify by read-back: a byte-identical write is silently skipped by the
	// CLI, so Committed alone does not prove OUR body landed.
	if held, presence, err := readLock(t, p); err != nil {
		return l, err
	} else if presence != lockPresent || held.Nonce != nonce {
		return l, fmt.Errorf("lock read-back mismatch; another writer holds %s", p)
	}
	l.inDoubt.Store(false)
	return l, nil
}

// Release is the normal-path release of the handle a caller was given: one
// attempt, after the settle wait when the handle is in doubt, with a failure
// reported as the error and every final outcome except Unreadable reported as
// success.
func (l *Lock) Release() error {
	if l.InDoubt() {
		settle()
	}
	out, err := l.ReleaseVia(l.t)
	switch out {
	case Removed, Absent, Foreign:
		return nil
	}
	if err == nil {
		// ReleaseVia always explains a non-release; if that ever stops being
		// true, the gap must not read as success.
		err = fmt.Errorf("lock release for %s did not complete (%s)", l.path, out)
	}
	return err
}

// ReleaseVia runs the nonce-first release on t, which need not be the
// transport the lock was acquired on: the shutdown coordinator passes its own
// instance. A final outcome takes the lock out of the register; Failed leaves
// it there so a later attempt can find it.
func (l *Lock) ReleaseVia(t transport.Transport) (ReleaseOutcome, error) {
	out, err := l.releaseOn(t)
	if out.Final() {
		unregister(l)
	}
	return out, err
}

// releaseOn is check-then-act and cannot be made atomic on this transport. If
// a human clears a stale lock and another process acquires it in the gap, this
// can still delete the newcomer's lock. Narrow, documented, and unavoidable
// without a conditional delete.
func (l *Lock) releaseOn(t transport.Transport) (ReleaseOutcome, error) {
	held, presence, err := readLock(t, l.path)
	if err != nil {
		return Failed, err
	}
	switch presence {
	case lockAbsent:
		return Absent, nil // already gone; nothing to do
	case lockUnreadable:
		// Present, but we cannot parse it, so we cannot prove it is ours.
		// Trashing it anyway would be exactly the takeover v2 does not have:
		// it could delete a healthy lock some other process just wrote.
		return Unreadable, fmt.Errorf("lock at %s is present but its contents are unreadable; "+
			"not removed because it cannot be confirmed as ours — "+
			"inspect it with the Proton CLI and clear it manually if it is stale", l.path)
	}
	if held.Nonce != l.nonce {
		return Foreign, nil // not ours any more; leave it alone
	}
	// Committed is the only outcome that proves the delete happened. A plain
	// error cannot distinguish a Trash that failed from one that committed
	// before the response was lost, so a non-Committed Outcome (Refused or
	// Ambiguous) must be reported as failure, not swallowed: with no
	// takeover mechanism in v2, an operator who believes the repo is
	// unlocked while .lock is still sitting on the remote wedges the repo
	// until someone notices.
	out, err := t.Trash(l.path)
	if err != nil {
		return Failed, err
	}
	if out != transport.Committed {
		return Failed, fmt.Errorf("lock release for %s is unconfirmed (Trash reported %s, not committed); "+
			"check with the Proton CLI whether %s still exists before assuming the repo is unlocked",
			l.path, out, l.path)
	}
	return Removed, nil
}

// readLock downloads to a temp DIRECTORY, not a file path: the CLI's
// `download` takes a destination folder, so the lock body lands under
// whatever name it already has on the remote.
//
// The returned lockPresence MUST be checked before trusting lockBody: a
// non-nil error is a genuine I/O failure (network, disk, ...) and callers
// return it as-is. A nil error with lockUnreadable means the read itself
// succeeded but the content did not parse — that is presence, not absence,
// and callers must not treat it as "nothing here."
func readLock(t transport.Transport, p string) (lockBody, lockPresence, error) {
	if _, ok, err := t.Stat(p); err != nil {
		return lockBody{}, lockAbsent, err
	} else if !ok {
		return lockBody{}, lockAbsent, nil
	}
	dir, err := os.MkdirTemp("", "gpb-lock-*")
	if err != nil {
		return lockBody{}, lockUnreadable, err
	}
	defer os.RemoveAll(dir)
	if err := t.ReadTo(p, dir); err != nil {
		return lockBody{}, lockUnreadable, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return lockBody{}, lockUnreadable, err
	}
	if len(entries) == 0 {
		return lockBody{}, lockUnreadable, fmt.Errorf("lock at %s downloaded no content", p)
	}
	raw, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		return lockBody{}, lockUnreadable, err
	}
	var lb lockBody
	if err := json.Unmarshal(raw, &lb); err != nil {
		// Present, but not valid JSON: presence, never absence — a corrupt
		// lock must not be treated as free to acquire or safe to skip on
		// release.
		return lockBody{}, lockUnreadable, nil
	}
	return lb, lockPresent, nil
}
