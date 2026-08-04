// Package gitcmd wraps git plumbing commands (cat-file, merge-base,
// rev-list, pack-objects) needed by git-remote-proton. All object work is
// done by shelling out to git rather than reimplementing pack formats.
//
// This package is self-contained: it imports nothing from internal/transport
// or internal/repo.
package gitcmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// oidRe validates a full SHA-1 object name. gitcmd is self-contained (it
// imports nothing from internal/repo), so it holds its own copy rather than
// sharing repo's shaRe. SHA-256 repositories are refused elsewhere by design.
var oidRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// waitDelay bounds how long Wait may block draining a git child's output pipes
// AFTER git itself has already exited. It is not a command timeout.
//
// MECHANISM (documented os/exec behaviour). Every exec site in this package
// redirects Stdout/Stderr to something that is not an *os.File — a
// *bytes.Buffer via CombinedOutput, or a *strings.Builder in WritePack — so
// os/exec creates an os.Pipe(), hands the write end to git, and copies from
// the read end on a goroutine. Wait does not return until that goroutine sees
// EOF, and EOF requires EVERY holder of the write end to close it, not just
// the direct child. A git invoked with core.fsmonitor enabled spawns a daemon
// that inherits the pipe and outlives the command, so without this the helper
// blocks forever (go.dev/issue/23019). Identical shape to the hang observed
// against the Proton CLI during the Stage 2.1 live gate; see
// internal/transport/cli.go.
//
// LIMIT: with no Context set, the timer starts when Wait observes the child
// has exited. A git that never exits at all is NOT bounded by this.
//
// A var, not a const, purely for symmetry with the transport-side knob;
// nothing in production writes to it.
var waitDelay = 30 * time.Second

// warnWaitDelay reports an abandoned output pipe on stderr — never stdout,
// which is protocol-only. Without it a recurrence of the grandchild-holds-the-
// pipe case is completely invisible.
func warnWaitDelay(what string) {
	fmt.Fprintf(os.Stderr, "git-remote-proton: %s exited but something still held its output "+
		"pipe open after %s; the pipe was abandoned (output may be truncated)\n", what, waitDelay)
}

// git runs `git -C gitDir <args>` and returns its combined stdout+stderr
// (trimmed), its exit code, and any error starting/running the process.
//
// cmd.ProcessState is nil when the process never starts at all (git missing
// from PATH, permission denied, ...), and ExitCode() on a nil ProcessState
// panics. A panic is not failing closed, so that case is guarded explicitly
// and reported as code -1 with the underlying start error preserved — the
// same shape internal/transport/cli.go's run helper uses, for the same
// reason: callers must be able to distinguish "ran and failed" from "never
// ran", and the cause must survive even when combined output is empty.
func git(gitDir string, args ...string) (string, int, error) {
	cmd := exec.Command("git", append([]string{"-C", gitDir}, args...)...)
	// Scrubbed for the same reason main.go unsets the inherited GIT_DIR: Go's
	// exec.Command inherits this process's environment by default, so a
	// GIT_ALTERNATE_OBJECT_DIRECTORIES set here would otherwise leak into
	// every git() call — including WritePack's rev-list step, which would
	// then enumerate objects reachable only through an alternate that
	// PackObjectsFromList's own explicit override (altObjects="" for
	// WritePack) cannot read, producing a pack request for objects it cannot
	// actually pack. Fail-closed either way, but this makes rev-list and
	// pack-objects agree on the same (alternate-free) world instead of
	// disagreeing on it.
	cmd.Env = append(os.Environ(), "GIT_ALTERNATE_OBJECT_DIRECTORIES=")
	// Bounds the post-exit pipe drain so an fsmonitor daemon (or any other
	// grandchild) holding the inherited write end cannot hang the helper
	// forever. See waitDelay: DO NOT DELETE as unnecessary.
	cmd.WaitDelay = waitDelay
	out, err := cmd.CombinedOutput()
	if errors.Is(err, exec.ErrWaitDelay) {
		// git itself SUCCEEDED — os/exec substitutes ErrWaitDelay for a nil
		// error only on a successful exit — so ProcessState is set, the exit
		// code below is the real one, and callers that branch on it first are
		// unaffected. Truncated output is still handled correctly by every
		// caller: ObjectType would see a type that is not "commit",
		// IsAncestor switches on the exit code, and WritePack guards its own
		// rev-list output explicitly (below), so no path silently narrows.
		warnWaitDelay("git " + strings.Join(args, " "))
	}
	if cmd.ProcessState == nil {
		return strings.TrimSpace(string(out)), -1, err
	}
	return strings.TrimSpace(string(out)), cmd.ProcessState.ExitCode(), err
}

// ObjectType resolves the type of sha locally. git hands the helper only
// hashes, so the design's "a branch target must be a commit" rule is
// unenforceable without this: there is no other way to tell a commit from a
// tag or blob before deciding whether to accept a ref update.
func ObjectType(gitDir, sha string) (string, error) {
	out, code, err := git(gitDir, "cat-file", "-t", sha)
	if code != 0 {
		if err != nil {
			return "", fmt.Errorf("cat-file -t %s: %s: %w", sha, out, err)
		}
		return "", fmt.Errorf("cat-file -t %s: %s", sha, out)
	}
	return out, nil
}

// HasObject reports whether sha exists in gitDir's object store.
//
// A false return means "not confirmed present", not "confirmed absent": the
// signature (fixed by the plan; Task 10 is written against it) has no error
// channel, so a tooling failure (git missing from PATH, a corrupt repo, an
// unexpected exit code) is indistinguishable here from a genuine miss. That
// is tolerable specifically because of how HasObject is used: the design's
// object-transfer rule for advertised tips is that objects that cannot be
// confirmed locally are simply not excluded from the pack — collapsing
// "not confirmed" into "false" only makes the pack larger, never wrong.
func HasObject(gitDir, sha string) bool {
	_, code, _ := git(gitDir, "cat-file", "-e", sha+"^{object}")
	return code == 0
}

// IsAncestor reports whether old is an ancestor of new (a commit is its own
// ancestor). Only exit code 1 from merge-base --is-ancestor is the genuine,
// confirmed negative answer ("not an ancestor"); any other outcome — a
// higher exit code from a malformed or missing object, or the -1 sentinel
// git() returns when the git binary itself never started — means the
// question could not actually be answered, and is surfaced as (false, err)
// rather than silently folded into the same "not an ancestor" result. Task
// 10 gates non-fast-forward rejection on this return value, so a genuine
// tooling failure must not present as "confirmed safe to reject" with no
// diagnostic trail.
func IsAncestor(gitDir, old, new string) (bool, error) {
	out, code, err := git(gitDir, "merge-base", "--is-ancestor", old, new)
	switch code {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		if err != nil {
			return false, fmt.Errorf("merge-base --is-ancestor %s %s: %s: %w", old, new, out, err)
		}
		return false, fmt.Errorf("merge-base --is-ancestor %s %s: %s (exit %d)", old, new, out, code)
	}
}

// RevParse runs `git rev-parse <args...>` and returns the trimmed output,
// the raw exit code, and any start/run error, the same three-value shape
// git() itself returns: callers (Task 10's resolve(), repo.consolidateAndInstall)
// decide what counts as success themselves, the same way IsAncestor and
// WritePack above interpret git's exit codes rather than collapsing them
// into a single bool.
//
// Variadic (fix round 2) rather than a single `rev` string: `--git-path
// <path>` needs the path as a SEPARATE argv element — confirmed empirically
// that rev-parse does not accept the `--flag=value` form for it, so
// "--git-path=objects/pack" is not recognised as a flag at all and is
// echoed back as a literal string rather than resolved. The single-rev
// shape every existing caller (resolve()'s RevParse(gitDir, src)) already
// uses still compiles unchanged: a lone variadic argument is the same call.
func RevParse(gitDir string, args ...string) (string, int, error) {
	out, code, err := git(gitDir, append([]string{"rev-parse"}, args...)...)
	return out, code, err
}

// WritePack builds a NON-THIN pack containing the objects reachable from
// want but not from any of haves, and writes it into outDir. --no-thin is
// deliberate: a thin pack would depend on delta bases the remote may not
// hold, and the remote here is Proton Drive, not another git repo that can
// fill in bases on the fly.
//
// Ordering guarantee: pack, then idx, then confirm both exist, then (by the
// caller) the ref. WritePack itself only does the first three steps — it
// must not report success, or hand back a path, it has not confirmed. A ref
// pointing at a missing index would not be fetch-discoverable.
//
// When rev-list finds nothing to send (want is already covered by haves),
// WritePack returns ("", "", nil): a legitimate, distinct outcome, not an
// error.
//
// outDir is resolved to an ABSOLUTE path first, and the returned paths are
// absolute as a result. This is the untreated twin of the path-doubling bug
// the Stage 3a live gate found in repo.consolidateAndInstall: outDir becomes
// PackObjectsFromList's outStem, which is an ARGUMENT to a `git -C gitDir ...`
// subprocess, and -C changes the directory relative arguments resolve against
// too — so a relative outDir would be resolved against gitDir by the child
// while this function's own os.Stat guards below resolve it against the
// process cwd. Every current caller happens to pass an absolute temp dir, so
// the hazard is latent and fails closed (pack-objects errors on a directory
// that does not exist) rather than silently misplacing a pack. It is fixed
// anyway because PackObjectsFromList is now a SHARED exec site with two
// callers holding different path disciplines, and "latent" is a property of
// today's callers, not of the function.
func WritePack(gitDir, want string, haves []string, outDir string) (string, string, error) {
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return "", "", fmt.Errorf("cannot resolve pack output directory %q to an absolute "+
			"path: %w", outDir, err)
	}
	// Assigned only AFTER the error check: on failure filepath.Abs returns "",
	// and overwriting outDir first would make the message above name an empty
	// path instead of the one the caller actually passed.
	outDir = absOut

	revArgs := []string{"rev-list", "--objects", want}
	for _, h := range haves {
		revArgs = append(revArgs, "^"+h)
	}
	objs, code, err := git(gitDir, revArgs...)
	if code != 0 {
		if err != nil {
			return "", "", fmt.Errorf("rev-list failed: %s: %w", objs, err)
		}
		return "", "", fmt.Errorf("rev-list failed: %s", objs)
	}
	// The one caller of git() where truncated output would be silently WRONG
	// rather than merely noisy: this list IS the pack's contents, so a short
	// read produces a smaller pack that is missing objects, gets published,
	// and is unrecoverable. Every other git() caller fails closed on its own
	// under truncation; this one has to say so. Fail closed here rather than
	// pack what we happen to have read.
	if errors.Is(err, exec.ErrWaitDelay) {
		return "", "", fmt.Errorf("rev-list exited but something still held its output pipe "+
			"open after %s, so its object list cannot be trusted to be complete; refusing to "+
			"build a pack that may be missing objects", waitDelay)
	}
	if strings.TrimSpace(objs) == "" {
		return "", "", nil // nothing to send
	}

	// No alternate: WritePack packs purely from gitDir's own store, which is
	// exactly what an empty altObjects means to PackObjectsFromList below.
	name, err := PackObjectsFromList(gitDir, "", objs, filepath.Join(outDir, "pack"))
	if err != nil {
		return "", "", err
	}
	base := filepath.Join(outDir, "pack-"+name)
	packPath, idxPath := base+".pack", base+".idx"

	// Fail closed: confirm both files actually exist before returning them.
	// The design's ordering guarantee is pack -> idx -> confirm both -> ref;
	// a caller that trusts an unconfirmed path could write a ref pointing at
	// a pack or index that was never actually written, which would not be
	// fetch-discoverable.
	if _, err := os.Stat(packPath); err != nil {
		return "", "", fmt.Errorf("pack-objects reported %s but the pack file is missing: %w", packPath, err)
	}
	if _, err := os.Stat(idxPath); err != nil {
		return "", "", fmt.Errorf("pack-objects reported %s but the idx file is missing: %w", idxPath, err)
	}
	return packPath, idxPath, nil
}

// PackObjectsFromList runs `git pack-objects` against objs — a newline-
// separated object list, as rev-list --objects produces — writing the result
// under outStem (pack-objects appends "-<hash>.{pack,idx}" itself) and
// returning that hash.
//
// outStem MUST BE ABSOLUTE. It is passed as an argument to `git -C gitDir
// pack-objects`, and -C changes the directory relative arguments resolve
// against, so a relative stem is resolved against gitDir by this subprocess —
// not against the cwd the caller built it from. Both callers absolutise
// before calling (WritePack's outDir, consolidateAndInstall's realPack), each
// for a reason recorded at its own site; this is the shared contract those two
// independently satisfy. Not enforced here, because the failure is a
// caller-side path-discipline bug and a check here would only rediscover it
// one frame later, with less context about which discipline was violated.
//
// altObjects, when non-empty, is spliced in as an alternate object store via
// GIT_ALTERNATE_OBJECT_DIRECTORIES — the same mechanism revListWithAlt above
// uses. An empty altObjects means gitDir's own store only (the same contract
// ConnectivityOK's doc states), and the variable is set to that empty value
// explicitly rather than left unset, so a value already present in the
// calling process's own environment can never leak in unnoticed.
//
// This is WritePack's own exec site, extracted (fix round 1, I1) so a second
// caller — repo.consolidateAndInstall, which packs the alt-objects closure a
// fetch downloads — gets the same WaitDelay / ErrWaitDelay /
// multi-pack-rejection guards WritePack already had, rather than a second,
// easily-drifting copy of them missing all three (which is exactly what
// consolidateAndInstall had before this extraction: a hang after the pack was
// already written into the user's live object store, unkept and unknown to
// git, plus no defence against a truncated pack name). See waitDelay: DO NOT
// DELETE as unnecessary.
//
// pack-objects writes the pack hash to stdout, which is then parsed into a
// name below. stdout must not be mixed with stderr here (unlike the git()
// helper above) — any warning git writes to stderr would corrupt the parsed
// name and produce a path that does not exist. stdout is captured alone for
// parsing; stderr is kept separately and folded into the error message on
// failure so diagnostics still surface. (This is a distinct hazard from I2's
// RevParse fix, restored here after round 1's extraction dropped it: the
// same class of bug — untrusted subprocess output trusted as data without
// being kept clean of stderr — should not have to be rediscovered twice.)
//
// Both pins are normative (design v6.2). packSizeLimit must be overridden as
// CONFIG, not as --max-pack-size=0: git reads 0 there as "unset" and falls
// back to the very config being overridden. The index-version pin must be on
// the command line too, because pack.indexVersion is user-configurable and
// `index-pack --verify` validates an index in whatever version it already is.
func PackObjectsFromList(gitDir, altObjects, objs, outStem string) (string, error) {
	cmd := exec.Command("git", "-C", gitDir,
		"-c", "pack.packSizeLimit=0",
		"pack-objects", "--no-thin", "--index-version=2", "-q", outStem)
	cmd.Env = append(os.Environ(), "GIT_ALTERNATE_OBJECT_DIRECTORIES="+altObjects)
	// A trailing newline is required, and at most one is wanted: rev-list's
	// own untrimmed output (RevListNewObjects, fed here unmodified by
	// repo.consolidateAndInstall) already ends with one, while git()'s
	// trimmed output (fed here by WritePack above) does not. TrimRight then
	// re-adding exactly one normalises either caller's input the same way.
	cmd.Stdin = strings.NewReader(strings.TrimRight(objs, "\r\n") + "\n")
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// Same exposure as git() above, for the same reason: *strings.Builder is
	// not an *os.File, so these are pipes a grandchild can hold open past the
	// child's exit. See waitDelay: DO NOT DELETE as unnecessary.
	cmd.WaitDelay = waitDelay
	if err := cmd.Run(); err != nil {
		// ErrWaitDelay means pack-objects itself exited 0 but the pipe was
		// abandoned, so stdout — which is the pack NAME this function parses a
		// filename out of — may be short. A truncated hash yields a path that
		// does not exist, and a caller's os.Stat guard would report that as a
		// missing pack, blaming the wrong thing. Name the real cause instead.
		if errors.Is(err, exec.ErrWaitDelay) {
			return "", fmt.Errorf("pack-objects exited but something still held its output "+
				"pipe open after %s, so the pack name it printed cannot be trusted to be "+
				"complete; refusing to guess which pack it wrote", waitDelay)
		}
		return "", fmt.Errorf("pack-objects: %s: %w", strings.TrimSpace(stderr.String()), err)
	}
	name := strings.TrimSpace(stdout.String())

	// One pack is an invariant every caller's ordering guarantee rests on: a
	// second pack silently dropped would mean a caller trusts a name it never
	// actually got — a ref published against half-uploaded objects on the
	// push side, or an untracked pack in the live object store on the fetch
	// side. The size pin above should make this unreachable, so treat it as a
	// hard error rather than parsing the first line.
	if strings.ContainsAny(name, " \t\r\n") {
		return "", fmt.Errorf("pack-objects emitted more than one pack (%q); "+
			"the one-pack invariant is broken", name)
	}
	return name, nil
}

// SymbolicRef returns the ref a symbolic ref points at — "refs/heads/main" for
// HEAD in an ordinary checkout. It exists because git never tells a remote
// helper what the client's own HEAD is, and the deterministic HEAD rules need
// it to break a multi-branch tie.
//
// A detached HEAD is not a failure: `symbolic-ref --quiet` exits 1 for "this
// is not a symbolic ref", and that is reported as ("", nil).
func SymbolicRef(gitDir, name string) (string, error) {
	out, code, err := git(gitDir, "symbolic-ref", "--quiet", name)
	switch code {
	case 0:
		return out, nil
	case 1:
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("symbolic-ref %s: %s: %w", name, out, err)
	}
	return "", fmt.Errorf("symbolic-ref %s: %s", name, out)
}

// revListWithAlt runs rev-list against gitDir with altObjects spliced in as an
// alternate object store, feeding the wants on stdin. The wants are passed
// explicitly rather than discovered from refs because at fetch time nothing
// references them yet.
//
// altObjects is an OBJECTS directory: git looks for packs at
// <altObjects>/pack/pack-<hash>.{pack,idx} and silently ignores them anywhere
// else, which presents downstream as "every object is missing".
func revListWithAlt(gitDir, altObjects string, wants []string, args ...string) (string, int, error) {
	cmd := exec.Command("git", append([]string{"-C", gitDir, "rev-list"}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_ALTERNATE_OBJECT_DIRECTORIES="+altObjects)
	cmd.Stdin = strings.NewReader(strings.Join(wants, "\n") + "\n")
	// See waitDelay: DO NOT DELETE as unnecessary.
	cmd.WaitDelay = waitDelay
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := -1
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	if errors.Is(err, exec.ErrWaitDelay) {
		// rev-list itself exited 0 but a grandchild held the pipe. stdout may
		// be truncated, and here stdout IS the object list, so refusing to
		// guess is the only safe answer.
		return "", -1, fmt.Errorf("rev-list output was abandoned after %s; refusing to "+
			"act on a possibly truncated object list", waitDelay)
	}
	if code != 0 {
		if err != nil {
			return "", code, fmt.Errorf("rev-list %s: %s: %w", strings.Join(args, " "),
				strings.TrimSpace(stderr.String()), err)
		}
		return "", code, fmt.Errorf("rev-list %s: %s", strings.Join(args, " "),
			strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), 0, nil
}

// ConnectivityOK reports whether every object reachable from wants is present,
// counting gitDir's own store plus altObjects. A nil return means the closure
// is complete. An empty altObjects means no alternate is spliced in at all —
// gitDir's own store only — which is the contract repo.Fetch's up-to-date
// short-circuit relies on: calling this with altObjects="" asks purely
// whether gitDir already has the closure, with nothing downloaded yet.
//
// This is a traversal, not an fsck: the wants are not referenced by any ref, so
// fsck would never reach them. --quiet suppresses the object list; the exit
// code is the answer.
func ConnectivityOK(gitDir, altObjects string, wants []string) error {
	if len(wants) == 0 {
		return nil
	}
	if _, _, err := revListWithAlt(gitDir, altObjects, wants,
		"--objects", "--stdin", "--not", "--all", "--quiet"); err != nil {
		return fmt.Errorf("closure is incomplete for the requested objects: %w", err)
	}
	return nil
}

// RevListNewObjects returns the objects reachable from wants that gitDir does
// not already have, one per line, ready to feed to pack-objects.
//
// --not --all is load-bearing: without it an incremental fetch reconsolidates
// the entire history into a fresh pack and installs it, silently doubling
// local disk every time.
func RevListNewObjects(gitDir, altObjects string, wants []string) (string, error) {
	if len(wants) == 0 {
		return "", nil
	}
	out, _, err := revListWithAlt(gitDir, altObjects, wants,
		"--objects", "--stdin", "--not", "--all")
	if err != nil {
		return "", err
	}
	return out, nil
}

// IndexPackVerify runs `git index-pack --verify` on packPath, whose .idx must
// sit beside it under the same stem. It is the only check that proves a pack
// is internally well-formed and agrees with its index; a basename/checksum
// comparison is the only check that proves the file is the one the name
// claims. Neither substitutes for the other (design v6.2, error table).
//
// It verifies an index in whatever version that index already is, which is
// why it cannot be the enforcement point for the index-version pin.
func IndexPackVerify(packPath string) error {
	cmd := exec.Command("git", "index-pack", "--verify", packPath)
	// Same exposure as git() above. See waitDelay: DO NOT DELETE.
	cmd.WaitDelay = waitDelay
	out, err := cmd.CombinedOutput()
	// ErrWaitDelay is substituted for a nil error ONLY on a successful exit,
	// so the verification itself passed and only the pipe drain was abandoned.
	// Returning it as an error would report a passing verification as
	// "the remote index does not verify against its pack" and refuse a
	// legitimate push — a false failure, not a fail-closed one. Warn instead.
	if errors.Is(err, exec.ErrWaitDelay) {
		warnWaitDelay("git index-pack --verify " + packPath)
		return nil
	}
	if err != nil {
		return fmt.Errorf("index-pack --verify %s: %s: %w",
			packPath, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// RevListMissing returns the OIDs of objects reachable from wants but present
// neither in gitDir's store nor in altObjects — the missing frontier the
// selective-fetch loop resolves through the object-to-pack map. Deduplicated;
// empty means discovery is complete.
//
// Requires a git whose `rev-list --missing=print` reports missing tips and
// parents as ?-lines (the Stage 3b minimum-git floor; probed on 2.53, pinned
// by this package's tests). On an older git the traversal dies at the first
// missing tip; the error wrap below names the floor so the failure is a
// diagnosed refusal rather than git's bare fatal.
func RevListMissing(gitDir, altObjects string, wants []string) ([]string, error) {
	if len(wants) == 0 {
		return nil, nil
	}
	out, _, err := revListWithAlt(gitDir, altObjects, wants,
		"--objects", "--missing=print", "--stdin", "--not", "--all")
	if err != nil {
		return nil, fmt.Errorf("%w (note: selective fetch requires a git whose rev-list "+
			"--missing=print reports absent objects as ?-lines; a git older than that floor "+
			"dies here with its own 'bad object' error instead)", err)
	}
	return parseMissingOIDs(out)
}

// parseMissingOIDs extracts ?-prefixed OIDs from rev-list --missing=print
// output. Normative (spec, component 4): the OID is the first whitespace-
// delimited token after the '?', validated as 40-hex — never assumed
// path-free, even though the probed git prints bare OIDs — and a ?-line that
// does not yield one is a hard parse error, never skipped.
func parseMissingOIDs(out string) ([]string, error) {
	seen := map[string]bool{}
	var missing []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if !strings.HasPrefix(line, "?") {
			continue
		}
		fields := strings.Fields(line[1:])
		if len(fields) == 0 || !oidRe.MatchString(fields[0]) {
			return nil, fmt.Errorf("unparseable missing-object line %q from rev-list", line)
		}
		if !seen[fields[0]] {
			seen[fields[0]] = true
			missing = append(missing, fields[0])
		}
	}
	return missing, nil
}

// ShowIndex returns every object ID recorded in the pack index at idxPath,
// via `git show-index` — git's own reader, which speaks index v1 AND v2.
// That is the parent design's trap closed the way it prescribes: Push
// deliberately accepts a valid remote v1 .idx it did not write, so a v2-only
// parser here would accept an index at push time it cannot fetch later.
//
// show-index reads the index on stdin and needs no repository; output lines
// are "<offset> <oid>" (v1) or "<offset> <oid> (<crc32>)" (v2), so the OID is
// always the second field. Any line that does not parse that way is a hard
// error — a truncated or garbled answer must never become a smaller map.
func ShowIndex(idxPath string) ([]string, error) {
	f, err := os.Open(idxPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cmd := exec.Command("git", "show-index")
	cmd.Stdin = f
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// stdout here IS the object list, so an abandoned pipe means a possibly
	// truncated list; fail closed like WritePack's rev-list guard.
	// See waitDelay: DO NOT DELETE as unnecessary.
	cmd.WaitDelay = waitDelay
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrWaitDelay) {
			return nil, fmt.Errorf("show-index output was abandoned after %s; refusing to "+
				"act on a possibly truncated object list", waitDelay)
		}
		return nil, fmt.Errorf("show-index %s: %s: %w", idxPath,
			strings.TrimSpace(stderr.String()), err)
	}
	var oids []string
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || !oidRe.MatchString(fields[1]) {
			return nil, fmt.Errorf("show-index %s: unparseable line %q", idxPath, line)
		}
		oids = append(oids, fields[1])
	}
	return oids, nil
}
