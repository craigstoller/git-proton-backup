package repo

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
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
	staged, cleanup, err := stagedFile(body, LockName)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	p := root + "/" + LockName
	out, err := t.CreateExclusive(p, staged)
	if err != nil {
		return nil, err
	}
	switch out {
	case transport.Refused:
		// Coherent per presence, not a fallthrough: a corrupt lock is not the
		// same situation as a healthy one or a lock that vanished mid-race,
		// and the operator needs to know which they are looking at before
		// touching anything by hand.
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
		return nil, fmt.Errorf("lock acquisition ambiguous; re-run to reconcile")
	}

	// Verify by read-back: a byte-identical write is silently skipped by the
	// CLI, so Committed alone does not prove OUR body landed.
	if held, presence, err := readLock(t, p); err != nil {
		return nil, err
	} else if presence != lockPresent || held.Nonce != nonce {
		return nil, fmt.Errorf("lock read-back mismatch; another writer holds %s", p)
	}
	return &Lock{t: t, path: p, nonce: nonce}, nil
}

// Release is check-then-act and cannot be made atomic on this transport. If a
// human clears a stale lock and another process acquires it in the gap, this
// can still delete the newcomer's lock. Narrow, documented, and unavoidable
// without a conditional delete.
func (l *Lock) Release() error {
	held, presence, err := readLock(l.t, l.path)
	if err != nil {
		return err
	}
	switch presence {
	case lockAbsent:
		return nil // already gone; nothing to do
	case lockUnreadable:
		// Present, but we cannot parse it, so we cannot prove it is ours.
		// Trashing it anyway would be exactly the takeover v2 does not have:
		// it could delete a healthy lock some other process just wrote.
		return fmt.Errorf("lock at %s is present but its contents are unreadable; "+
			"not removed because it cannot be confirmed as ours — "+
			"inspect it with the Proton CLI and clear it manually if it is stale", l.path)
	}
	if held.Nonce != l.nonce {
		return nil // not ours any more; leave it alone
	}
	// Committed is the only outcome that proves the delete happened. A plain
	// error cannot distinguish a Trash that failed from one that committed
	// before the response was lost, so a non-Committed Outcome (Refused or
	// Ambiguous) must be reported as failure, not swallowed: with no
	// takeover mechanism in v2, an operator who believes the repo is
	// unlocked while .lock is still sitting on the remote wedges the repo
	// until someone notices.
	out, err := l.t.Trash(l.path)
	if err != nil {
		return err
	}
	if out != transport.Committed {
		return fmt.Errorf("lock release for %s is unconfirmed (Trash reported %s, not committed); "+
			"check with the Proton CLI whether %s still exists before assuming the repo is unlocked",
			l.path, out, l.path)
	}
	return nil
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
	raw, err := os.ReadFile(dir + string(os.PathSeparator) + entries[0].Name())
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
