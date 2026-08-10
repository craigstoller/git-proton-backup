package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/protocol"
	"github.com/craigstoller/git-proton-backup/internal/repo"
	"github.com/craigstoller/git-proton-backup/internal/transport"
)

// version is stamped by the release build via
//
//	-ldflags "-X main.version=<tag>"
//
// and stays "dev" for a plain `go build`.
var version = "dev"

// uncertifiedCLIEnv is the allowlist-override variable, read at BOTH
// transport.EnforceCertified call sites (run and runSetHead). One const, not
// two literals: a typo in either would silently disable the override on that
// path alone — fails closed, but with the two entry points disagreeing about
// what the documented variable is.
const uncertifiedCLIEnv = "GPB_UNCERTIFIED_CLI"

// createParentsEnv is Task 11's opt-in for repo.EnsureParents: unset (the
// default), a missing parent above a push's repo root is an actionable
// refusal; "1" lets the helper create missing parents itself, one at a time,
// with a loud stderr line per folder. Read fresh from the environment on
// EVERY invocation, same convention as uncertifiedCLIEnv — never cached.
//
// Read ONLY at the "list for-push" arm in loop, below, immediately before
// repo.Bootstrap. Never at a read path (a fetch or plain "list" must never
// bring anything into existence, parents included) and never by runSetHead:
// a repo cannot exist below a missing parent, so honouring this var there
// could only manufacture folder trees and then fail on the marker check
// anyway (repo.SetHead's RequireMarker call, which runs first and refuses
// regardless — see internal/repo/parents.go's doc comment and
// repo_test.go's TestSetHeadNeverCreatesParents, which pins that SetHead
// never references EnsureParents at all).
const createParentsEnv = "GPB_CREATE_PARENTS"

// main only chooses the process exit code. All cleanup lives in run's defers:
// calling os.Exit directly from deep inside the command loop would skip every
// deferred func (Go does not run defers on os.Exit), which is exactly the lock
// leak the "release on every exit path" rule exists to prevent.
func main() {
	os.Exit(run())
}

// runSetHeadFn exists ONLY as a test seam pinning the dispatchUtility→
// runSetHead wiring (argv routing, argument order, exit-code propagation,
// writer plumbing) for hermetic tests that cannot reach the real
// runSetHead (it constructs a live *transport.CLI). Nothing else may
// reassign it — production always calls through the real runSetHead via
// this variable's initializer.
var runSetHeadFn = runSetHead

// dispatchUtility handles the CLOSED set of direct-invocation modes. Only
// exact matches dispatch: a prefix match would misroute a remote whose
// configured name begins with "--" (git passes the remote NAME as argv[1]),
// so only remotes literally named --version or --set-head can collide, and
// those two strings are documented as reserved. Utility stdout is permitted:
// git never invokes the helper with these argv shapes, so it cannot
// interleave with the protocol stream.
func dispatchUtility(args []string, stdout, stderr io.Writer) (bool, int) {
	if len(args) < 2 {
		return false, 0
	}
	switch args[1] {
	case "--version":
		fmt.Fprintf(stdout, "git-remote-proton %s (certified CLI: %s)\n",
			version, transport.CertifiedCLI)
		return true, 0
	case "--set-head":
		if len(args) != 4 {
			fmt.Fprintln(stderr, "usage: git-remote-proton --set-head <proton::address> <branch>")
			return true, 1
		}
		return true, runSetHeadFn(args[2], args[3], stdout, stderr)
	}
	return false, 0
}

func runSetHead(addr, branch string, stdout, stderr io.Writer) int {
	root, err := repo.CanonicalRoot(addr)
	if err != nil {
		fmt.Fprintf(stderr, "git-remote-proton: %v\n", err)
		return 1
	}
	cli := transport.NewCLI("")
	if err := transport.EnforceCertified(cli,
		os.Getenv(uncertifiedCLIEnv) == "1", stderr); err != nil {
		fmt.Fprintf(stderr, "git-remote-proton: %v\n", err)
		return 1
	}
	set, err := repo.SetHead(transport.NewTraced(cli, stderr), root, branch)
	if err != nil {
		fmt.Fprintf(stderr, "git-remote-proton: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "HEAD is now %s\n", set)
	return 0
}

func run() int {
	if handled, code := dispatchUtility(os.Args, os.Stdout, os.Stderr); handled {
		return code
	}
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "git-remote-proton: must be run by git as a remote helper")
		return 1
	}
	// Canonicalise BEFORE anything else touches the address. Every remote path
	// in this process is built by concatenation onto root, so a trailing slash
	// produced "//refs/heads" and an empty address produced "/refs/heads" —
	// both sent to the Proton CLI verbatim. The canonical form is also the
	// lock identity, so two spellings of one repo would otherwise be two locks
	// over one repo. repo.CanonicalRoot additionally refuses /shared-with-me,
	// which is a safety rule rather than a tidiness one: a repo in a folder
	// another person can write to is the one failure mode the single-writer
	// model cannot survive.
	root, err := repo.CanonicalRoot(os.Args[2])
	if err != nil {
		warn(err)
		return 1
	}
	gitDir, err := resolveGitDir()
	if err != nil {
		warn(err)
		return 1
	}
	// The rest of GIT_DIR's class, resolved at the same point and for the same
	// reason. See resolveGitDir's doc for which variables are covered, which
	// is deliberately left alone, and why unsetting them would be fail-open
	// rather than safe.
	if err := absolutizeInheritedGitPaths(); err != nil {
		warn(err)
		return 1
	}
	// git sets GIT_DIR (commonly a RELATIVE path, e.g. ".git") in this
	// process's own environment before spawning the helper. internal/gitcmd
	// invokes `git -C <gitDir> ...` as a subprocess, and Go's exec.Command
	// inherits the current process's environment by default — so without
	// this, the child git ALSO sees GIT_DIR=".git" and resolves it relative
	// to the working directory -C already changed to, producing ".git/.git"
	// and failing every gitcmd call with "not a git repository". gitDir is
	// already captured above (and resolved to an absolute path by
	// resolveGitDir — see its doc) and passed explicitly to every gitcmd
	// call, so clearing the inherited env var here is safe and necessary.
	os.Unsetenv("GIT_DIR")

	cli := transport.NewCLI("")
	// The Stage 4 allowlist: refuse-by-default against the certified build,
	// with GPB_UNCERTIFIED_CLI=1 as the explicit, loud escape hatch. This
	// closed the design/code contradiction open since Stage 2 — the advisory
	// warn that used to live here is now transport.EnforceCertified.
	if err := transport.EnforceCertified(cli,
		os.Getenv(uncertifiedCLIEnv) == "1", os.Stderr); err != nil {
		warn(err)
		return 1
	}
	in := bufio.NewScanner(os.Stdin)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	return loop(transport.NewTraced(cli, os.Stderr), root, gitDir, in, out)
}

// resolveGitDir captures GIT_DIR from the environment and resolves it to an
// ABSOLUTE path immediately, before anything else can see it.
//
// Not optional hygiene: git commonly sets GIT_DIR to a RELATIVE path — ".git"
// is git's own default — relative to THIS PROCESS's cwd. internal/repo's
// fetch install path (consolidateAndInstall) shells out to git with
// `-C gitDir` more than once while installing a new pack, and asks git
// itself where the pack belongs via `rev-parse --git-path objects/pack` — an
// answer that, for a relative gitDir, comes back relative too. That answer
// then gets handed as an argument to a SECOND `-C gitDir`-scoped subprocess
// (pack-objects), which resolves relative ARGUMENTS again, relative to
// gitDir this time — a path built for one resolution context, resolved a
// second time in another, doubling the relative prefix. Observed live on
// Windows exactly this way (Stage 3a gate, task 7): every ordinary
// incremental `git fetch` failed with "unable to write file
// .git\objects\pack\pack-....pack: No such file or directory", because git
// invokes this helper with GIT_DIR=.git by default — the common case, not an
// edge case.
//
// Resolving here, once, at the source, means every downstream `-C gitDir`
// subprocess and every relative path git hands back through one of them gets
// resolved exactly once, unambiguously. filepath.Abs resolves relative to
// this process's own cwd, which is exactly what a relative GIT_DIR is
// relative to. (internal/repo/fetch.go's consolidateAndInstall also resolves
// its own realPack absolutely as a second, independent line of defence —
// fetch.go is a library entry point in its own right, and its own tests call
// Fetch directly, so the fix there must not silently depend on this
// function's hygiene either.)
//
// GIT_DIR IS NOT THE ONLY ENVIRONMENT VARIABLE IN THIS CLASS, and the class is
// NOT closed by this function. Three others can be relative and can change
// where objects live:
//
//   - GIT_COMMON_DIR and GIT_OBJECT_DIRECTORY are handled — see
//     absolutizeInheritedGitPaths below, which resolves them at capture and
//     re-exports the absolute forms. A relative GIT_COMMON_DIR is the sharpest
//     case: re-resolved inside a `-C`-scoped child it yields an install path
//     that PASSES validateObjectsPackPath (it still ends in "/objects/pack")
//     yet points somewhere git never reads — fail-OPEN, since the objects are
//     invisible and connectivity-ok is reported anyway.
//   - GIT_WORK_TREE is deliberately NOT handled. It names the working tree,
//     not the object store, so it cannot move where a pack is installed or
//     where git looks for one; this helper never reads or writes worktree
//     files. Absolutising it would be churn with no failure mode behind it.
//
// Blanket-unsetting the object-placement pair would be WRONG, which is why
// none of this is "just unset them like GIT_DIR". GIT_DIR can be unset for the
// children because gitDir is passed explicitly to every gitcmd call, so the
// child is told the same answer by another route. GIT_OBJECT_DIRECTORY has no
// such route: unsetting it would make this helper pack into the DEFAULT object
// database while the calling git reads the one it nominated — objects written
// where nobody looks, reported as a successful fetch. That is the same
// fail-open shape the relative-path bug produces, arrived at deliberately.
// Absolutising preserves the caller's meaning and only removes the ambiguity.
//
// Reachability is narrow and stated plainly: git does not export any of these
// to a spawned remote helper, so they arrive only if the USER exported them in
// their own shell. That is why this is hardening, not a fix for an observed
// failure — unlike the relative GIT_DIR above, which git sets on every
// ordinary invocation and which broke the live gate.
func resolveGitDir() (string, error) {
	gitDir := os.Getenv("GIT_DIR")
	if gitDir == "" {
		gitDir = "."
	}
	abs, err := filepath.Abs(gitDir)
	if err != nil {
		return "", fmt.Errorf("cannot resolve GIT_DIR %q to an absolute path: %w", gitDir, err)
	}
	return abs, nil
}

// objectPlacementEnv are the inherited variables, other than GIT_DIR, that can
// relocate where git reads and writes objects. Listed as data so the set is
// one thing to audit rather than two open-coded blocks. GIT_WORK_TREE is
// absent on purpose — see resolveGitDir's doc for why it does not belong here.
var objectPlacementEnv = []string{"GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY"}

// absolutizeInheritedGitPaths re-exports each of objectPlacementEnv as an
// ABSOLUTE path, resolved against THIS process's cwd — the same treatment, for
// the same reason, that resolveGitDir gives GIT_DIR.
//
// A relative value here means one thing to this process and another to every
// `git -C <gitDir> ...` child the helper spawns, because -C changes the
// directory relative paths resolve against. Resolving once at the source
// cannot change what the value MEANS (filepath.Abs anchors it to exactly the
// cwd it was already implicitly relative to); it only makes that meaning
// survive being read in a different context.
//
// Unset and empty values are left alone: an empty GIT_OBJECT_DIRECTORY is not
// a relative path, and turning it into the cwd would invent an object store
// the caller never asked for. A value that cannot be resolved is fatal rather
// than passed through, because passing it through is the fail-open case this
// exists to prevent.
func absolutizeInheritedGitPaths() error {
	for _, name := range objectPlacementEnv {
		v := os.Getenv(name)
		if v == "" || filepath.IsAbs(v) {
			continue
		}
		abs, err := filepath.Abs(v)
		if err != nil {
			return fmt.Errorf("cannot resolve %s %q to an absolute path: %w", name, v, err)
		}
		if err := os.Setenv(name, abs); err != nil {
			return fmt.Errorf("cannot re-export %s as an absolute path: %w", name, err)
		}
	}
	return nil
}

// loop runs the git-remote-helper command exchange over in/out. Split out
// from run so the fail-closed exit-code paths can be exercised directly with
// an in-memory transport.Fake and buffered io.Reader/io.Writer in tests — no
// live account, no real git remote.
func loop(t transport.Transport, root, gitDir string, in *bufio.Scanner, out *bufio.Writer) int {
	var opts protocol.Options
	var lock *repo.Lock
	var checkConnectivity bool
	defer func() {
		if lock != nil {
			// Release on EVERY exit path; a leak wedges the repo. Its error is
			// REPORTED but deliberately does NOT change the exit code.
			//
			// Release returns two fully-formed operator messages — "lock at %s
			// is present but its contents are unreadable" and "lock release
			// for %s is unconfirmed (Trash reported %s)" — and discarding them
			// meant that when a release failed, the push exited 0, stderr said
			// nothing, .lock stayed on the remote, and, because v2 has no
			// takeover, every subsequent push failed until a human cleared it
			// by hand with no clue why.
			//
			// The exit code stays as it was because the push itself succeeded:
			// reporting failure for a completed push would make git discard
			// its remote-tracking update for refs that really did land, which
			// is its own wrong answer. Do not "fix" this into a `return 1`.
			if err := lock.Release(); err != nil {
				warn(err)
			}
		}
	}()

	for in.Scan() {
		line := in.Text()
		switch {
		case line == "capabilities":
			// check-connectivity is advertised as well as accepted: git only
			// recognises a connectivity-ok response from a helper that
			// advertised the capability.
			fmt.Fprint(out, "option\npush\nfetch\ncheck-connectivity\n\n")
			out.Flush()

		case strings.HasPrefix(line, "option "):
			// check-connectivity is HONOURED, not poisoned: poison exists for
			// options git ignores our rejection of, and this is the opposite.
			if v, ok := strings.CutPrefix(line, "option check-connectivity "); ok {
				checkConnectivity = strings.TrimSpace(v) == "true"
				fmt.Fprint(out, "ok\n")
				out.Flush()
				continue
			}
			opts.Observe(line)
			fmt.Fprint(out, "unsupported\n") // advisory only; poison flag is the real defence
			out.Flush()

		case line == "list":
			// The FETCH-side advertisement. Read-only: no Bootstrap, no lock.
			// A fetch must never bring a repository into existence, and a
			// lock here would wedge the repo for every reader if we crashed.
			//
			// RequireMarker is the read-only half of Bootstrap's check, and it
			// runs FIRST. Without it, `git ls-remote proton::/my-files/anything`
			// listed a folder that is not one of our repos and reported an
			// empty ref set: ListRefs on a folder with no refs/ namespace is
			// not necessarily an error, so "this is not a git-remote-proton
			// repo" presented as "a repo with no refs". repo.Fetch has always
			// applied the same check; the advertisement is the other read-side
			// entry point and had nothing.
			if err := repo.RequireMarker(t, root); err != nil {
				warn(err)
				return 1
			}
			scan, err := repo.ScanRefs(t, root)
			if err != nil {
				warn(err)
				return 1
			}
			if cs := scan.ContentSkips(); len(cs) > 0 {
				// STRICT fetch survey (design spec, component 1): a well-named
				// ref file whose CONTENT does not parse could be a damaged
				// real ref, so its silent absence would be a false-success
				// restore — a clone that succeeds while quietly lacking a
				// branch. Complete-or-loudly-incomplete: the whole "list" call
				// fails with one error enumerating every content-skipped
				// path, rather than advertising a subset and hoping the
				// operator notices what is missing.
				var b strings.Builder
				fmt.Fprintf(&b, "cannot serve a fetch: %d file(s) under refs/ are not valid refs "+
					"and a restore would silently lack them:\n", len(cs))
				for _, s := range cs {
					fmt.Fprintf(&b, "  %s/%s: %s\n", root, s.Path, s.Reason)
				}
				fmt.Fprintf(&b, "delete these files first (proton-drive filesystem trash <path>, or the "+
					"web UI; Proton trash keeps them restorable), then retry")
				warn(errors.New(b.String()))
				return 1
			}
			refs := scan.Refs
			for name, sha := range refs {
				fmt.Fprintf(out, "%s %s\n", sha, name)
			}
			if branch, ok, err := repo.ReadHEAD(t, root); err != nil {
				warn(err)
				return 1
			} else if ok {
				if _, listed := refs[branch]; listed {
					// The symref line is what lets clone check something out — and
					// it is emitted ONLY when the branch it names is in the ref
					// list we just advertised.
					//
					// A remote can hold a HEAD pointing at a branch that no longer
					// exists: v2 never rewrites an existing HEAD, so any remote
					// that lost its default branch before the delete refusal in
					// repo.pushOne shipped is stuck that way permanently. Advertised
					// verbatim, that symref makes clone fetch every object and then
					// check out nothing — which the design names as a failure.
					// Suppressing the line instead presents the remote as HEADLESS,
					// which IS a defined state: clone reports that no default branch
					// exists rather than silently producing an empty worktree.
					//
					// Checked against the just-listed refs rather than by a second
					// remote read, so the advertisement is internally consistent by
					// construction: the symref cannot name something the same
					// response failed to advertise.
					fmt.Fprintf(out, "@%s HEAD\n", branch)
				} else if sk, matched := scanSkipMatch(scan, branch); matched {
					// HEAD names something the scan skipped rather than
					// something simply absent (the pre-Task-2 case above
					// still covers that silently, unchanged). Only NAME-skips
					// can reach here in the fetch direction: a content-skip
					// would already have failed the whole command above, so
					// this branch is unreachable for Kind==SkipContent by
					// construction, not by a check here.
					headSkipNote(os.Stderr, branch, sk)
				}
			}
			fmt.Fprint(out, "\n")
			out.Flush()

		case line == "list for-push":
			// EnsureParents runs BEFORE Bootstrap, and ONLY here — see
			// createParentsEnv's doc above for why this is the one and only
			// call site. Its stderr argument is the real os.Stderr, not the
			// buffered `out` this loop writes protocol lines to: created-
			// folder notes are advisory operator output, the same channel
			// warn() already uses, never part of the git-remote-helper
			// protocol stream itself.
			if err := repo.EnsureParents(t, root, os.Getenv(createParentsEnv) == "1", os.Stderr); err != nil {
				warn(err)
				return 1
			}
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
			scan, err := repo.ScanRefs(t, root)
			if err != nil {
				warn(err)
				return 1
			}
			refs := scan.Refs // Task 2 wires scan.Skipped into the push-side occupancy preflight
			for name, sha := range refs {
				fmt.Fprintf(out, "%s %s\n", sha, name)
			}
			fmt.Fprint(out, "\n")
			out.Flush()

			// The push-survey HEAD diagnostic: ADVISORY, and COST-GATED on
			// something already having been skipped. This is deliberately
			// AFTER the advertisement above — it can never change what was
			// just advertised, only add a note about it. Gating on
			// len(scan.Skipped) > 0 means a clean repo's push survey stays
			// exactly as read-free as it was before this task: Stage 5's fix
			// round removed an unconditional per-push HEAD read for cost, and
			// this must not reintroduce it for the common case.
			if len(scan.Skipped) > 0 {
				branch, ok, herr := repo.ReadHEAD(t, root)
				if herr != nil {
					// A failing diagnostic read must never fail the push
					// advertisement itself — that would reintroduce, from a
					// different angle, exactly the backup-stopping wedge the
					// tolerant policy exists to remove.
					warn(fmt.Errorf("HEAD unreadable during skip diagnostics: %v", herr))
				} else if ok {
					if sk, matched := scanSkipMatch(scan, branch); matched {
						headSkipNote(os.Stderr, branch, sk)
					}
				}
			}

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
			scan, err := repo.ScanRefs(t, root)
			if err != nil {
				warn(err)
				return 1
			}
			remote := scan.Refs // Task 2 wires scan.Skipped into the push-side occupancy preflight
			for _, r := range repo.Push(t, root, gitDir, ups, remote) {
				if r.OK {
					fmt.Fprintf(out, "ok %s\n", r.Ref)
				} else {
					fmt.Fprintf(out, "error %s %s\n", r.Ref, r.Err)
				}
			}
			fmt.Fprint(out, "\n")
			out.Flush()

		case strings.HasPrefix(line, "fetch "):
			var wants []string
			for l := line; ; {
				sha, perr := parseFetchLine(l)
				if perr != nil {
					// Fail closed rather than silently drop or misparse the
					// line: a malformed line that got silently dropped could
					// leave wants empty, and repo.Fetch reports an empty
					// wants list as ("", nil) — its legitimate "up to date"
					// signal — which would then surface as a FALSE
					// connectivity-ok, vouching for a closure nothing ever
					// verified.
					warn(perr)
					return 1
				}
				wants = append(wants, sha)
				if !in.Scan() {
					break
				}
				l = in.Text()
				if l == "" {
					break
				}
			}
			if len(wants) == 0 {
				// Defence in depth: parseFetchLine above already fails
				// closed on every line it sees, so this should be
				// unreachable. It is asserted explicitly anyway, because the
				// invariant it protects — an empty batch must never reach
				// Fetch, whose own ("", nil) "up to date" signal would then
				// be trusted on the strength of a closure nothing verified —
				// is exactly the one this fix exists for.
				warn(fmt.Errorf("fetch batch contained no wants"))
				return 1
			}
			cacheDir, cerr := repo.ResolveIdxCacheDir(gitDir, root)
			if cerr != nil {
				warn(fmt.Errorf("sidecar cache unavailable (%v); pack indexes will be "+
					"re-downloaded this fetch", cerr))
				cacheDir = ""
			}
			keep, err := repo.Fetch(t, root, gitDir, cacheDir, wants)
			if err != nil {
				warn(err)
				return 1
			}
			if keep != "" {
				// Git retains only the FIRST lock, which is why the closure
				// is consolidated into one pack.
				fmt.Fprintf(out, "lock %s\n", keep)
			}
			if checkConnectivity {
				// Only after Fetch verified it. Fetch returns an error
				// otherwise, so reaching here means the closure is complete.
				fmt.Fprint(out, "connectivity-ok\n")
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

// scanSkipMatch reports whether branch matches an entry in scan's skipped
// set, and which one — the one matcher both the fetch-direction and
// push-direction HEAD arms above call, so the two directions can never drift
// into different notions of "HEAD names a skipped ref" (design spec,
// "Degraded states").
//
// An exact Path match works for ANY Kind. A SkipInvalidNameFolder entry
// additionally matches any path strictly BENEATH it (branch has s.Path+"/"
// as a prefix): ScanRefs' walk never enters an invalid folder's subtree, so
// the folder itself is the only occupancy it can ever record — HEAD naming
// something inside it (e.g. "refs/heads/.hidden/topic" when only
// "refs/heads/.hidden" was skipped) is still naming something the scan
// skipped, and must not be reported as ordinary, unremarkable absence. This
// prefix rule does NOT apply to SkipInvalidName or SkipContent, both of
// which are leaf FILES with no subtree to be a descendant of.
func scanSkipMatch(scan *repo.RefScan, branch string) (repo.SkippedRef, bool) {
	for _, s := range scan.Skipped {
		if s.Path == branch {
			return s, true
		}
		if s.Kind == repo.SkipInvalidNameFolder && strings.HasPrefix(branch, s.Path+"/") {
			return s, true
		}
	}
	return repo.SkippedRef{}, false
}

// headSkipNote writes the stderr note both HEAD arms emit when HEAD names a
// path the scan skipped — one wording shared by both call sites, so the
// fetch- and push-direction surveys can never report the same degraded state
// in different words.
func headSkipNote(w io.Writer, branch string, sk repo.SkippedRef) {
	fmt.Fprintf(w, "git-remote-proton: HEAD names %s, which was skipped (%s); "+
		"advertising no default branch\n", branch, sk.Reason)
}

// fetchShaRe matches a fetch batch line's <sha> field: 40 lowercase hex, the
// same grammar internal/repo enforces on every ref it writes. Duplicated
// rather than exported from repo — validating protocol INPUT is main's job,
// not repo's.
var fetchShaRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// parseFetchLine validates one line of a "fetch" batch against exactly
// "fetch <sha> <name>" — not merely "starts with fetch and has a plausible
// second field". A batch git never sends malformed cannot be relied on to
// stay that way (a hand-crafted or corrupted stream is exactly the case
// fail-closed exists for): the previous, permissive scan silently dropped
// any line with fewer than two fields and accepted whatever token sat second
// on any line with two or more, so a batch of nothing but malformed lines
// left `wants` empty without ever reporting an error. repo.Fetch reports an
// empty wants list as ("", nil) — its legitimate "up to date" signal — and
// with checkConnectivity on, that reached the caller as connectivity-ok for
// a closure nothing had ever verified. This mirrors the push side's
// protocol.ParsePushBatch: one strict parser per line, not a permissive scan
// with the lock/connectivity decision bolted on after.
func parseFetchLine(l string) (sha string, err error) {
	sp := strings.Fields(l)
	if len(sp) != 3 || sp[0] != "fetch" || !fetchShaRe.MatchString(sp[1]) {
		return "", fmt.Errorf("malformed fetch batch line %q: want \"fetch <sha> <name>\" "+
			"with a 40-lowercase-hex sha", l)
	}
	return sp[1], nil
}

func warn(err error) {
	fmt.Fprintf(os.Stderr, "git-remote-proton: %v\n", err)
}
