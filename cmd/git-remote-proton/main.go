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
			// Check poison AFTER the whole batch is buffered, before mutating.
			if opts.Poisoned != "" {
				for _, b := range batch {
					dst := b[strings.Index(b, ":")+1:]
					fmt.Fprintf(out, "error %s unsupported option %s\n", dst, opts.Poisoned)
				}
				fmt.Fprint(out, "\n")
				out.Flush()
				continue
			}
			ups, err := protocol.ParsePushBatch(batch)
			if err != nil {
				warn(err)
				return 1
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
	return 0
}

func warn(err error) {
	fmt.Fprintf(os.Stderr, "git-remote-proton: %v\n", err)
}
