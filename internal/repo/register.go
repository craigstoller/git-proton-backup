package repo

import "sync"

// The register is this process's list of remote locks it has put there, or may
// have: AcquireLock enters one BEFORE its upload begins, and a final release
// outcome removes it. A shutdown coordinator reads it from the signal path to
// find what a killed process would otherwise leave behind. The normal path
// never reads it: loop and SetHead release the handle they were given and
// nothing else, so one lock held by another caller in the same process is not
// theirs to touch.
//
// The mutex guards the slice only and is never held across a transport call,
// so a slow or stuck call can never block the signal path from reading the
// register.
var (
	regMu    sync.Mutex
	regLocks []*Lock
)

func register(l *Lock) {
	regMu.Lock()
	defer regMu.Unlock()
	regLocks = append(regLocks, l)
}

// unregister drops l by identity. A lock that is not registered is not an
// error: a final outcome may arrive from the normal path and the coordinator
// both.
func unregister(l *Lock) {
	regMu.Lock()
	defer regMu.Unlock()
	for i, r := range regLocks {
		if r == l {
			regLocks = append(regLocks[:i:i], regLocks[i+1:]...)
			return
		}
	}
}

// RegisteredLocks returns a snapshot of the register. The slice is the
// caller's to keep and change; the locks in it are shared.
func RegisteredLocks() []*Lock {
	regMu.Lock()
	defer regMu.Unlock()
	return append([]*Lock(nil), regLocks...)
}

// ResetRegister empties the register. The coordinator calls it on a clean
// exit; tests call it to start from a known state.
func ResetRegister() {
	regMu.Lock()
	defer regMu.Unlock()
	regLocks = nil
}
