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
		if held, ok, _ := readLock(t, p); ok {
			return nil, fmt.Errorf("repo is locked by %s (pid %d) since %s; "+
				"if that process is gone, remove %s with the Proton CLI",
				held.Host, held.PID, held.AcquiredAt, p)
		}
		return nil, fmt.Errorf("repo is locked; remove %s with the Proton CLI if stale", p)
	case transport.Ambiguous:
		return nil, fmt.Errorf("lock acquisition ambiguous; re-run to reconcile")
	}

	// Verify by read-back: a byte-identical write is silently skipped by the
	// CLI, so Committed alone does not prove OUR body landed.
	if held, ok, err := readLock(t, p); err != nil {
		return nil, err
	} else if !ok || held.Nonce != nonce {
		return nil, fmt.Errorf("lock read-back mismatch; another writer holds %s", p)
	}
	return &Lock{t: t, path: p, nonce: nonce}, nil
}

// Release is check-then-act and cannot be made atomic on this transport. If a
// human clears a stale lock and another process acquires it in the gap, this
// can still delete the newcomer's lock. Narrow, documented, and unavoidable
// without a conditional delete.
func (l *Lock) Release() error {
	held, ok, err := readLock(l.t, l.path)
	if err != nil {
		return err
	}
	if !ok || held.Nonce != l.nonce {
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
func readLock(t transport.Transport, p string) (lockBody, bool, error) {
	if _, ok, err := t.Stat(p); err != nil || !ok {
		return lockBody{}, false, err
	}
	dir, err := os.MkdirTemp("", "gpb-lock-*")
	if err != nil {
		return lockBody{}, false, err
	}
	defer os.RemoveAll(dir)
	if err := t.ReadTo(p, dir); err != nil {
		return lockBody{}, false, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return lockBody{}, false, err
	}
	raw, err := os.ReadFile(dir + string(os.PathSeparator) + entries[0].Name())
	if err != nil {
		return lockBody{}, false, err
	}
	var lb lockBody
	if err := json.Unmarshal(raw, &lb); err != nil {
		return lockBody{}, false, nil // unreadable lock is treated as held by someone
	}
	return lb, true, nil
}
