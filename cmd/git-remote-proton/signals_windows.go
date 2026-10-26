package main

import (
	"os"
	"syscall"
)

// shutdownSignals is the set that starts shutdown on Windows (S2): os.Interrupt,
// which Go delivers for both Ctrl-C and Ctrl-Break in a console (spec P4), so
// both exit 130; and syscall.SIGTERM, which Go delivers for a console close.
func shutdownSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}

// notifySIGPIPE does nothing: Windows has no SIGPIPE. A write to a dead git
// just returns an error (P5), which loop records (S16).
func notifySIGPIPE(chan os.Signal) {}
