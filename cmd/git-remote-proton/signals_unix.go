//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

// shutdownSignals is the set that starts shutdown on Unix (S2): SIGINT, SIGTERM
// and SIGHUP, each left out if it was already ignored when the helper started.
// Notify would un-ignore it, and `nohup git push` and background jobs rely on
// the inherited disposition. SIGQUIT keeps Go's default on purpose: the
// goroutine dump is the debugging exit, and most useful on a stuck push.
func shutdownSignals() []os.Signal {
	var sigs []os.Signal
	for _, s := range []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP} {
		if !signal.Ignored(s) {
			sigs = append(sigs, s)
		}
	}
	return sigs
}

// notifySIGPIPE catches SIGPIPE, so that a write to a dead git fails with EPIPE,
// which loop records (S16), instead of killing the helper with the lock held
// (§5.2). Notify, not signal.Ignore: an ignored disposition carries across exec
// to every git and CLI child, while a caught one is reset to the default there.
func notifySIGPIPE(ch chan os.Signal) {
	signal.Notify(ch, syscall.SIGPIPE)
}
