package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/protocol"
	"github.com/craigstoller/git-proton-backup/internal/repo"
	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// main only chooses the process exit code. All cleanup lives in run's defers:
// calling os.Exit directly from deep inside the command loop would skip every
// deferred func (Go does not run defers on os.Exit), which is exactly the lock
// leak the "release on every exit path" rule exists to prevent.
func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "git-remote-proton: must be run by git as a remote helper")
		return 1
	}
	root := strings.TrimPrefix(os.Args[2], "proton::")
	gitDir := os.Getenv("GIT_DIR")
	if gitDir == "" {
		gitDir = "."
	}
	// git sets GIT_DIR (commonly a RELATIVE path, e.g. ".git") in this
	// process's own environment before spawning the helper. internal/gitcmd
	// invokes `git -C <gitDir> ...` as a subprocess, and Go's exec.Command
	// inherits the current process's environment by default — so without
	// this, the child git ALSO sees GIT_DIR=".git" and resolves it relative
	// to the working directory -C already changed to, producing ".git/.git"
	// and failing every gitcmd call with "not a git repository". gitDir is
	// already captured above and passed explicitly to every gitcmd call, so
	// clearing the inherited env var here is safe and necessary.
	os.Unsetenv("GIT_DIR")

	t := transport.NewCLI("")
	in := bufio.NewScanner(os.Stdin)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	return loop(t, root, gitDir, in, out)
}

// loop runs the git-remote-helper command exchange over in/out. Split out
// from run so the fail-closed exit-code paths can be exercised directly with
// an in-memory transport.Fake and buffered io.Reader/io.Writer in tests — no
// live account, no real git remote.
func loop(t transport.Transport, root, gitDir string, in *bufio.Scanner, out *bufio.Writer) int {
	var opts protocol.Options
	var lock *repo.Lock
	defer func() {
		if lock != nil {
			_ = lock.Release() // release on EVERY exit path; a leak wedges the repo
		}
	}()

	for in.Scan() {
		line := in.Text()
		switch {
		case line == "capabilities":
			fmt.Fprint(out, "option\npush\n\n")
			out.Flush()

		case strings.HasPrefix(line, "option "):
			opts.Observe(line)
			fmt.Fprint(out, "unsupported\n") // advisory only; poison flag is the real defence
			out.Flush()

		case line == "list for-push":
			if err := repo.Bootstrap(t, root); err != nil {
				warn(err)
				return 1
			}
			l, err := repo.AcquireLock(t, root) // BEFORE advertising
			if err != nil {
				warn(err)
				return 1
			}
			lock = l
			refs, err := repo.ListRefs(t, root)
			if err != nil {
				warn(err)
				return 1
			}
			for name, sha := range refs {
				fmt.Fprintf(out, "%s %s\n", sha, name)
			}
			fmt.Fprint(out, "\n")
			out.Flush()

		case strings.HasPrefix(line, "push "):
			// The lock is only ever set by a prior successful "list for-push",
			// which git always sends first to learn the ref list before it can
			// build a push batch. A push processed with no lock held would
			// write packs and refs with no mutual exclusion — precisely the
			// hazard the lock exists to prevent. Real git will not send push
			// without list-for-push first, but trusting that implicitly is
			// weaker than asserting it.
			if lock == nil {
				warn(fmt.Errorf("push received before list for-push"))
				return 1
			}
			batch := []string{line}
			for in.Scan() {
				l := in.Text()
				if l == "" {
					break
				}
				if strings.HasPrefix(l, "option ") {
					opts.Observe(l) // options may appear after the last push line
					continue
				}
				batch = append(batch, l)
			}
			// One parser for both the poisoned and non-poisoned paths: a
			// hand-rolled `dst := b[strings.Index(b, ":")+1:]` on the poison
			// path previously trusted every buffered line to already look
			// like "push <src>:<dst>" with no validation, so a line with no
			// colon produced dst == the whole raw line (including the
			// "push " prefix), corrupting the ref field of the "error <ref>
			// <reason>" status line git expects as one whitespace-free
			// token. Parsing once here means both paths agree on what a
			// line means.
			ups, err := protocol.ParsePushBatch(batch)
			if err != nil {
				// A batch that is both poisoned and malformed still fails
				// closed — poisoned or not, an unparseable batch was never
				// going to be applied.
				warn(err)
				return 1
			}
			// Check poison AFTER the whole batch is buffered and parsed,
			// before any remote read or mutation.
			if opts.Poisoned != "" {
				for _, u := range ups {
					fmt.Fprintf(out, "error %s unsupported option %s\n", u.Dst, opts.Poisoned)
				}
				fmt.Fprint(out, "\n")
				out.Flush()
				continue
			}
			remote, err := repo.ListRefs(t, root)
			if err != nil {
				warn(err)
				return 1
			}
			for _, r := range repo.Push(t, root, gitDir, ups, remote) {
				if r.OK {
					fmt.Fprintf(out, "ok %s\n", r.Ref)
				} else {
					fmt.Fprintf(out, "error %s %s\n", r.Ref, r.Err)
				}
			}
			fmt.Fprint(out, "\n")
			out.Flush()

		case line == "":
			return 0
		default:
			warn(fmt.Errorf("unsupported command: %q", line))
			return 1
		}
	}
	// in.Scan() returning false means either a clean EOF or a genuine read
	// error (a broken pipe, git crashing mid-session). Falling through to
	// "return 0" either way would report success for a session that ended
	// abnormally, with git never having sent the terminating blank line —
	// a literal violation of fail-closed. This does not affect the lock:
	// the defer above fires on every return from loop, so only the exit
	// code was ever at risk here.
	if err := in.Err(); err != nil {
		warn(err)
		return 1
	}
	return 0
}

func warn(err error) {
	fmt.Fprintf(os.Stderr, "git-remote-proton: %v\n", err)
}
