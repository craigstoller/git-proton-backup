//go:build !windows

package main

import (
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/craigstoller/git-proton-backup/internal/repo"
)

// The Unix end-to-end signal cases (spec §6, layer 3). Each drives the real
// helper binary through signal_harness_test.go and sends it a real signal.

// processAlive reports whether pid names a live process. kill(pid, 0) succeeds
// for a zombie too, so a killed shim reads as alive until its parent reaps it;
// assertNoShimLeft polls to absorb that. A shim whose parent has gone is
// reparented and reaped, and then reads as gone.
func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// setOwnProcessGroup starts the helper as the leader of a new process group
// (helperOpts.OwnGroup), so a test can signal the group without signalling
// itself.
func setOwnProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// TestSignal_E1_ReleasesWhenSignalledDuringThePackUpload is E1: SIGTERM, SIGINT
// and SIGHUP, each sent to the helper alone while its push is blocked in the
// pack upload with the lock held. The helper stops the shim, settles, releases
// the lock through its own CLI instance, says so, and exits 128 + the signal.
func TestSignal_E1_ReleasesWhenSignalledDuringThePackUpload(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP} {
		t.Run(sig.String(), func(t *testing.T) {
			h := startHelper(t, helperOpts{Block: "upload:*.pack:once:before"})
			h.pushMain(t)
			waitReady(t, h.Ready, "ready-upload-", 20*time.Second)
			if err := h.Cmd.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			code := h.wait(t, 20*time.Second)
			stderr := h.Stderr.String()
			if want := 128 + int(sig); code != want {
				t.Errorf("exit code = %d, want %d (128 + %s); stderr:\n%s", code, want, sig, stderr)
			}
			if lockPresent(h.Tree, harnessRoot) {
				t.Errorf("%s/%s is still in the shim tree after the release; stderr:\n%s",
					harnessRoot, repo.LockName, stderr)
			}
			assertNoShimLeft(t, h.Tree)

			name := map[syscall.Signal]string{
				syscall.SIGTERM: "SIGTERM", syscall.SIGINT: "SIGINT", syscall.SIGHUP: "SIGHUP",
			}[sig]
			if catch := linePrefix + "received " + name + ": stopping the push and checking the remote lock"; !strings.Contains(stderr, catch) {
				t.Errorf("stderr lacks the catch line %q:\n%s", catch, stderr)
			}
			removed := regexp.MustCompile(regexp.QuoteMeta(linePrefix+"lock "+harnessRoot+"/"+repo.LockName) +
				` \(nonce [0-9a-f]{32}\): removed: the lock is gone\n`)
			if !removed.MatchString(stderr) {
				t.Errorf("stderr lacks the removed result line for %s/%s:\n%s", harnessRoot, repo.LockName, stderr)
			}
		})
	}
}
