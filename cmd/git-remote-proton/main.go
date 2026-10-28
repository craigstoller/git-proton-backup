package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/craigstoller/git-proton-backup/internal/gitcmd"
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

// probeRestoreFloor is the restore-floor probe (spec D5(d)): whether this
// host's git is new enough to RESTORE from a git-remote-proton remote. It is a
// test seam only, there so the package's tests spawn no git for it (TestMain
// stubs it for the whole package) and can pin both warnings. Nothing else may
// reassign it; production always runs the real probe via this initializer.
var probeRestoreFloor = gitcmd.ProbeMissingTips

// warnRestoreFloor runs the restore-floor probe and writes at most one WARNING
// line to stderr. It never fails and never blocks: pushing needs no particular
// git, restoring needs git 2.45.0 or newer, and the point is to say so while the
// operator can still do something about it. stderr only, because stdout is the
// git-remote-helper protocol stream.
func warnRestoreFloor(gitDir string) {
	res, detail := probeRestoreFloor(gitDir)
	switch res {
	case gitcmd.ProbeOK:
		// This git can restore: nothing to say.
	case gitcmd.ProbeTooOld:
		fmt.Fprintf(os.Stderr, "git-remote-proton: WARNING: this git cannot restore from a "+
			"git-remote-proton remote: fetch and clone need git 2.45.0 or newer (%s). "+
			"This check does not block the push.\n", detail)
	default:
		// gitcmd.ProbeUnknown, and any result this switch does not know: an
		// unexpected answer is not proof this git can restore, so it reads as
		// "could not check", never as a silent pass.
		fmt.Fprintf(os.Stderr, "git-remote-proton: WARNING: could not check whether this git can "+
			"restore from a git-remote-proton remote (fetch and clone need git 2.45.0 or newer): %s. "+
			"This check does not block the push.\n", detail)
	}
}

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
	// Whether git set GIT_DIR, read HERE: resolveGitDir below defaults an
	// unset GIT_DIR to the working directory, and the Unsetenv after it
	// clears it, so no later read can tell (M25; see helperEnvFrom).
	env := helperEnvFrom(os.Args, os.Getenv, runtime.GOOS)
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

	return loopEnv(transport.NewTraced(cli, os.Stderr), root, gitDir, env, in, out)
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

// helperEnv is what the helper knows about how git started it, for the Mac
// read rules (spec 2026-10-27, section 3, O3(d)):
//
//   - goos is the operating system, runtime.GOOS in production. It is a field,
//     not a call, so the Mac rules are tested on every CI platform.
//   - remoteName is argv[1]: the remote's configured name, or, for an
//     anonymous URL, the URL itself (M25).
//   - gitDirSet is whether git set GIT_DIR. git sets it for a command run
//     anywhere inside a repository and leaves it unset only where no
//     repository encloses the working directory (M25).
//
// The zero value means "not a Mac": no local read, no refusal, the pair
// warning only.
type helperEnv struct {
	goos, remoteName string
	gitDirSet        bool
}

// macRules reports whether the Mac checks apply at all: on darwin, inside a
// repository. Whether that repository's ref store is files is the third
// condition, and needs a local read (readLocalState).
func (e helperEnv) macRules() bool { return e.goos == "darwin" && e.gitDirSet }

// helperEnvFrom builds the helperEnv from argv, the environment and the OS.
// run() calls it BEFORE resolveGitDir: resolveGitDir answers an absolute path
// whether or not GIT_DIR was set (an unset one defaults to the working
// directory), and run() then unsets GIT_DIR for the children, so only a read
// made first sees what git said. An empty GIT_DIR counts as unset, as
// resolveGitDir treats it.
func helperEnvFrom(args []string, getenv func(string) string, goos string) helperEnv {
	env := helperEnv{goos: goos, gitDirSet: getenv("GIT_DIR") != ""}
	if len(args) > 1 {
		env.remoteName = args[1]
	}
	return env
}

// readRefFormat, readRemoteConfig and readLocalRefs are the Mac checks' three
// local reads. Like probeRestoreFloor they are test seams only, so the tests
// can make each read fail and can pin that no read happens off a Mac. Nothing
// else may reassign them; production always calls the gitcmd functions
// through these initializers.
var (
	readRefFormat    = gitcmd.RefFormat
	readRemoteConfig = gitcmd.RemoteConfig
	readLocalRefs    = gitcmd.LocalRefs
)

// localState is what the Mac checks know about the local repository.
//
//   - format is "files" or "reftable". For reftable nothing else is read and
//     every other field is zero.
//   - specs is the remote's fetch mapping (see readLocalState for which).
//   - anonymous is a remote with neither url nor pushurl configured (R5).
//   - prune is remote.<name>.prune, else fetch.prune (R6).
//   - refs holds the name of every local ref; symrefs maps each symbolic one
//     to its target.
type localState struct {
	format           string
	specs            []repo.Refspec
	anonymous, prune bool
	refs             map[string]bool
	symrefs          map[string]string
}

// readLocalState makes the Mac checks' local reads, in this order, stopping at
// the first that fails: the ref format; for files only, the remote's config
// and then the local refs. Each error is wrapped with the read's name ("ref
// format", "remote config" or "local refs"), so the refusal says which read
// failed (spec section 3, "Its local reads fail closed").
//
// The fetch mapping (R5, R6, PM9):
//   - a remote whose url and pushurl are both unset or empty is anonymous and
//     has no specs, whatever its name looks like (PM3, PM4); a push-only
//     remote, with pushurl and fetch set, keeps its mapping;
//   - configured fetch refspecs are parsed, and one the helper does not
//     model fails the read closed (R4), under "remote config";
//   - a repository that holds no refs is a clone at "list" (M28), and gets
//     git's default mapping for remoteName ON TOP of the configured specs,
//     if any. A clone given `-c remote.<name>.fetch=...` shows the helper
//     only that spec at "list", yet git adds its default mapping and writes
//     both (Ruling T8-2; measured 2026-10-27, git 2.54.0). An empty
//     repository that is not a clone but has custom specs is over-refused
//     by this, which is the safe direction;
//   - a repository that holds refs gets its configured specs alone, or none
//     when none are configured, because git fetch then writes no ref (PM9).
func readLocalState(gitDir, remoteName string) (localState, error) {
	format, err := readRefFormat(gitDir)
	if err != nil {
		return localState{}, fmt.Errorf("ref format: %w", err)
	}
	if format == "reftable" {
		return localState{format: format}, nil
	}
	cfg, err := readRemoteConfig(gitDir, remoteName)
	if err != nil {
		return localState{}, fmt.Errorf("remote config: %w", err)
	}
	local, err := readLocalRefs(gitDir)
	if err != nil {
		return localState{}, fmt.Errorf("local refs: %w", err)
	}
	st := localState{
		format:  format,
		prune:   cfg.Prune,
		refs:    make(map[string]bool, len(local)),
		symrefs: map[string]string{},
	}
	for name, target := range local {
		st.refs[name] = true
		if target != "" {
			st.symrefs[name] = target
		}
	}
	if cfg.URL == "" && cfg.PushURL == "" {
		st.anonymous = true
		return st, nil
	}
	if len(cfg.Fetch) > 0 {
		specs, err := repo.ParseFetchRefspecs(cfg.Fetch)
		if err != nil {
			return localState{}, fmt.Errorf("remote config: %w", err)
		}
		st.specs = specs
	}
	if len(local) == 0 {
		st.specs = append(st.specs, repo.DefaultFetchRefspecs(remoteName)...)
	}
	return st, nil
}

// macFetchCheck is the Mac refusal at "list" (spec section 3, rules (1) and
// (2)). It returns nil when the fetch may go ahead: off a Mac, outside a
// repository, in a reftable one, or when nothing collides. Otherwise it
// returns the error the arm reports before advertising nothing, so git writes
// no ref. refs is the advertised set and pairs its AdvertisedPairs.
//
// git ls-remote inside such a repository is refused the same way: nothing
// before "list" tells it from a fetch.
func macFetchCheck(gitDir string, env helperEnv, refs map[string]string, pairs []repo.RefPair) error {
	if !env.macRules() {
		return nil
	}
	local, err := readLocalState(gitDir, env.remoteName)
	if err != nil {
		return fmt.Errorf("cannot check this Mac repository before fetching: %w", err)
	}
	if local.format != "files" {
		return nil
	}
	// Rule (1): a pair on the remote lands as two refs in one file, whatever
	// the mapping (bare and mirror clones included).
	if len(pairs) > 0 {
		return fmt.Errorf("refusing to fetch on this Mac: this repository uses git's default (files) "+
			"ref store, which cannot keep these refs apart (README, Honest limits):\n%s\n"+
			"repair the remote, or clone with --ref-format=reftable", repo.PairLines(pairs, refs))
	}
	// Rule (2): what this fetch could write, followed through local symbolic
	// refs, against itself and the local refs; and, when pruning is
	// configured (R6), what it could delete.
	d := repo.ExpandSymrefs(repo.FetchDestinations(refs, local.specs), local.symrefs)
	cs := repo.DestinationConflicts(d, local.refs)
	if local.prune {
		cs = repo.MergeConflicts(cs, repo.LocalConflicts(repo.PruneCandidates(local.specs, d, local.refs), local.refs))
	}
	if len(cs) > 0 {
		return fmt.Errorf("refusing to fetch on this Mac: refs this fetch would write or prune collide "+
			"with each other or with refs this repository holds (README, Honest limits):\n%s\n"+
			"delete the stale local ref by its exact bytes (git update-ref --stdin), change the remote's "+
			"fetch refspecs, or use a --ref-format=reftable clone", repo.DestLines(cs, refs))
	}
	return nil
}

// macPushCheck is the Mac refusal at "list for-push" (R1, PM1). An
// "Everything up-to-date" push still rewrites the tracking refs of the refs it
// matches, and sends the helper no push command for them, so the push arm
// never sees them. The check is the tracking refs of each advertised ref the
// local repository holds by its exact name, followed through local symbolic
// refs, against the other local refs. It returns nil when the push may go
// ahead, as macFetchCheck does.
//
// It does not refuse a remote pair: D2's ruling that a pair still takes pushes
// to its other refs stands, and the push arm decides per ref.
func macPushCheck(gitDir string, env helperEnv, refs map[string]string) error {
	if !env.macRules() {
		return nil
	}
	local, err := readLocalState(gitDir, env.remoteName)
	if err != nil {
		return fmt.Errorf("cannot check this Mac repository before pushing: %w", err)
	}
	if local.format != "files" {
		return nil
	}
	cs := repo.LocalConflicts(repo.ExpandSymrefs(repo.UpToDateTracking(refs, local.specs, local.refs),
		local.symrefs), local.refs)
	if len(cs) > 0 {
		return fmt.Errorf("refusing to push on this Mac: git would rewrite tracking refs that collide "+
			"with refs this repository holds, even for refs that are already up to date (README, Honest "+
			"limits):\n%s\ndelete the stale local ref by its exact bytes (git update-ref --stdin), or push "+
			"from a --ref-format=reftable clone", repo.DestLines(cs, refs))
	}
	return nil
}

// macLocalSide is the local side the Mac push rule judges a batch against
// (repo.LocalSide): the local ref names, and for each destination in ups its
// tracking refs, the positive mappings of the remote's fetch refspecs (R6,
// repo.PushTrackingRefs ignores negatives) followed through local symbolic
// refs, since git follows a symbolic tracking ref when it updates one. An
// anonymous URL has no remote config, so git writes no tracking ref and no
// destination gets one; nor does a destination no refspec maps.
func macLocalSide(st localState, ups []protocol.RefUpdate) *repo.LocalSide {
	side := &repo.LocalSide{Refs: st.refs, Tracking: map[string][]string{}}
	if st.anonymous {
		return side
	}
	for _, u := range ups {
		dests := map[string]string{}
		for _, tr := range repo.PushTrackingRefs(st.specs, u.Dst) {
			dests[tr] = u.Dst
		}
		if len(dests) == 0 {
			continue
		}
		expanded := repo.ExpandSymrefs(dests, st.symrefs)
		ts := make([]string, 0, len(expanded))
		for tr := range expanded {
			ts = append(ts, tr)
		}
		sort.Strings(ts)
		side.Tracking[u.Dst] = ts
	}
	return side
}

// warnPairs writes the pair warning (spec section 3, "The warning") to stderr
// when pairs is not empty. Each list arm calls it at most once; the push arm's
// own scan never does.
func warnPairs(pairs []repo.RefPair, refs map[string]string) {
	if len(pairs) > 0 {
		fmt.Fprintf(os.Stderr, "git-remote-proton: %s\n", repo.PairWarning(pairs, refs))
	}
}

// loop runs the git-remote-helper command exchange over in/out. Split out
// from run so the fail-closed exit-code paths can be exercised directly with
// an in-memory transport.Fake and buffered io.Reader/io.Writer in tests — no
// live account, no real git remote.
//
// It is loopEnv with the zero helperEnv ("not a Mac"), kept so the tests
// written before Task 8 run unchanged.
func loop(t transport.Transport, root, gitDir string, in *bufio.Scanner, out *bufio.Writer) int {
	return loopEnv(t, root, gitDir, helperEnv{}, in, out)
}

// loopEnv is the command exchange, with env saying how git started the helper:
// the "list" and "list for-push" arms warn about remote pairs everywhere and,
// on a Mac files-store repository, refuse what the filesystem would confuse
// (macFetchCheck, macPushCheck), and there the push arm hands repo.Push the
// local side its per-ref Mac rule judges (macLocalSide). run() is its only
// production caller.
func loopEnv(t transport.Transport, root, gitDir string, env helperEnv, in *bufio.Scanner, out *bufio.Writer) int {
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
			// O3(d): on a Mac files-store repository, refuse before advertising
			// anything, so git writes no ref (macFetchCheck). Everywhere else a
			// remote pair is a warning, written once, and the fetch goes on.
			pairs := repo.AdvertisedPairs(refs)
			if err := macFetchCheck(gitDir, env, refs, pairs); err != nil {
				warn(err)
				return 1
			}
			warnPairs(pairs, refs)
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
			// D5(d): say so if this git could not RESTORE from what it is
			// about to back up. After Bootstrap, so a push that is refused for
			// any earlier reason says nothing of the kind, and BEFORE the lock,
			// so the probe's few seconds (bounded near 12 s) are never spent
			// holding the single-writer lock. It only ever warns.
			warnRestoreFloor(gitDir)
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
			// scan.Skipped is not consulted here: this is the ADVERTISEMENT
			// (what refs a client is told exist before it builds a push
			// batch), not the execution arm. Occupancy enforcement against a
			// skipped path happens where the batch is actually applied —
			// repo.Push's phase-2 preflight and delete arm (component 2,
			// below at the "push " case) — this call site's only job is to
			// hand scan.Refs to the client unchanged.
			refs := scan.Refs
			// R1: on a Mac files-store repository, refuse before advertising
			// when an up-to-date push would rewrite an aliasing tracking ref
			// (macPushCheck). After AcquireLock, so the deferred release runs.
			if err := macPushCheck(gitDir, env, refs); err != nil {
				warn(err)
				return 1
			}
			for name, sha := range refs {
				fmt.Fprintf(out, "%s %s\n", sha, name)
			}
			fmt.Fprint(out, "\n")
			out.Flush()
			// The pair warning, on every platform, once, after the refs are
			// printed. Only here: the push arm's own ScanRefs below never
			// warns, so a push session says it once.
			warnPairs(repo.AdvertisedPairs(refs), refs)

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
			terminated := false
			for in.Scan() {
				l := in.Text()
				if l == "" {
					terminated = true
					break
				}
				if strings.HasPrefix(l, "option ") {
					opts.Observe(l) // options may appear after the last push line
					continue
				}
				batch = append(batch, l)
			}
			// The loop above also stops on EOF or a read error: git died, or the
			// read failed, before the whole batch arrived. What arrived parses as
			// a complete batch, so applying it would be a partial set of remote
			// writes nobody asked for in that shape. Fail closed before any
			// remote read or write; the defer above still releases the lock.
			if !terminated {
				err := in.Err()
				if err == nil {
					err = io.ErrUnexpectedEOF
				}
				warn(fmt.Errorf("push batch ended without its terminating blank line, "+
					"so none of it was applied: %w", err))
				return 1
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
			// scan.Skipped is handed straight to repo.Push (component 2): its
			// phase-2 preflight and delete arm are what actually enforce
			// occupancy against it — a create/update/delete landing on,
			// above, or beneath a skipped path is refused there, kind-aware,
			// before any pack is built. This call site's job is only to pass
			// through what ScanRefs just found.
			remote := scan.Refs
			// The Mac push rule (spec 2026-10-27, section 3): on a Mac files
			// repository, Push also judges each ref against the local refs
			// and its tracking refs, which only this arm can read. A failed
			// read refuses every update in the batch, naming the read, like
			// the poisoned path above: nothing has been written yet. The
			// reason goes through repo.OneLine because git's stderr is
			// multi-line, and a newline would desynchronise the protocol.
			var local *repo.LocalSide
			if env.macRules() {
				state, err := readLocalState(gitDir, env.remoteName)
				if err != nil {
					reason := repo.OneLine("cannot check this Mac repository before pushing: " + err.Error())
					for _, u := range ups {
						fmt.Fprintf(out, "error %s %s\n", u.Dst, reason)
					}
					fmt.Fprint(out, "\n")
					out.Flush()
					continue
				}
				if state.format == "files" {
					local = macLocalSide(state, ups)
				}
			}
			for _, r := range repo.PushWithLocal(t, root, gitDir, ups, remote, scan.Skipped, local) {
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
