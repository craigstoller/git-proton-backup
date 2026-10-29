package gitcmd

// Three reads of the local repository that the helper makes before it decides
// whether a fetch or a push is safe on a Mac: the ref storage format, the
// remote's configuration, and the names of the refs it already holds.
//
// All three run on gitStdout rather than on git(). git() mixes stderr into the
// output it returns and trims the result: a warning git writes on stderr would
// become a bogus ref name, and TrimSpace could strip trailing bytes from a real
// one. These reads are about exact bytes, so they take stdout alone, untrimmed.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// gitStdout runs `git -C gitDir <args>` like git() does, with the same
// GIT_ALTERNATE_OBJECT_DIRECTORIES scrub and the same waitDelay, but captures
// stdout and stderr SEPARATELY and trims neither. stdout is exactly the bytes
// git wrote to it.
//
// The return shape follows git(): code is the exit code, or -1 when the process
// never started (git missing from PATH, permission denied), and err is whatever
// os/exec reported, so a non-zero exit comes back as a non-nil *exec.ExitError
// beside its code. Callers that branch on a particular exit code (RemoteConfig
// on 1) look at code first.
//
// exec.ErrWaitDelay is the one case that cannot be passed through. os/exec
// substitutes it for a nil error when git itself exited 0 but a grandchild (an
// fsmonitor daemon, say) still held an output pipe after waitDelay: the pipe
// was abandoned, so stdout may be cut short. Every caller here treats stdout as
// a complete list, and a truncated ref list or config dump would fail open, so
// the output is withheld, the code is -1 (a caller that checks only the code
// fails closed too), and err wraps exec.ErrWaitDelay. revListWithAlt and
// ShowIndex refuse a possibly truncated list the same way; this one keeps the
// sentinel reachable through errors.Is.
func gitStdout(gitDir string, args ...string) (stdout []byte, stderr string, code int, err error) {
	cmd := exec.Command("git", append([]string{"-C", gitDir}, args...)...)
	// Scrubbed for the reason git() scrubs it: an inherited alternate must not
	// make these reads see a world the rest of the package does not.
	cmd.Env = append(os.Environ(), "GIT_ALTERNATE_OBJECT_DIRECTORIES=")
	// See waitDelay: DO NOT DELETE as unnecessary.
	cmd.WaitDelay = waitDelay
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	if errors.Is(err, exec.ErrWaitDelay) {
		return nil, errBuf.String(), -1, fmt.Errorf("git %s exited but something still held its "+
			"output pipe open after %s, so its output cannot be trusted to be complete; refusing "+
			"to act on a possibly truncated answer: %w", strings.Join(args, " "), waitDelay, err)
	}
	// ProcessState is nil when the process never started; ExitCode() on it would
	// panic. See git() for the same guard.
	if cmd.ProcessState == nil {
		return outBuf.Bytes(), errBuf.String(), -1, err
	}
	return outBuf.Bytes(), errBuf.String(), cmd.ProcessState.ExitCode(), err
}

// gitStdoutError describes a gitStdout call that did not succeed: the command,
// git's stderr, and the exit code (or the start error when the process never
// ran).
func gitStdoutError(args []string, stderr string, code int, err error) error {
	if errors.Is(err, exec.ErrWaitDelay) {
		return err // gitStdout's own message already names the command
	}
	what := "git " + strings.Join(args, " ")
	detail := strings.TrimSpace(stderr)
	if detail == "" {
		detail = "no message on stderr"
	}
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		return fmt.Errorf("%s: %s: %w", what, detail, err)
	}
	return fmt.Errorf("%s: %s (exit %d)", what, detail, code)
}

// RefFormat returns the reference storage format of the repository at gitDir,
// "files" or "reftable", as `git rev-parse --show-ref-format` reports it.
// Stdout minus ONE trailing newline must be exactly one of those two words.
// Anything else, or a non-zero exit, is an error naming the command and git's
// stderr: the format decides whether refs can be treated as files, so it is
// never guessed.
func RefFormat(gitDir string) (string, error) {
	args := []string{"rev-parse", "--show-ref-format"}
	out, stderr, code, err := gitStdout(gitDir, args...)
	if err != nil || code != 0 {
		return "", gitStdoutError(args, stderr, code, err)
	}
	format := strings.TrimSuffix(string(out), "\n")
	if format != "files" && format != "reftable" {
		return "", fmt.Errorf("git %s: unexpected output %q, want files or reftable",
			strings.Join(args, " "), string(out))
	}
	return format, nil
}

// RemoteCfg is what the repository's configuration says about one remote.
type RemoteCfg struct {
	// URL is the remote's first remote.<name>.url, the one git fetches from; ""
	// when none is set. An empty value clears the values before it, as in git,
	// so this is the first value after the last empty one, and "" when the
	// last value is empty.
	URL string
	// PushURL is the remote's first remote.<name>.pushurl, read as URL is; ""
	// when none is set.
	PushURL string
	// Fetch holds every remote.<name>.fetch refspec in configuration order,
	// negative ones ("^refs/heads/x") included and unparsed.
	Fetch []string
	// Prune is remote.<name>.prune when that is set, else fetch.prune, read as
	// git reads a boolean.
	Prune bool
}

// RemoteConfig reads the configuration of the remote called name from the
// repository at gitDir. name is any string git accepts as a remote name, a
// dotted one such as back.up included, and may be the anonymous URL the helper
// is handed ("proton::/my-files/a.b"), which has no configuration at all.
//
// It makes ONE call, `git config -z --get-regexp
// '^(remote\.<name>\.(url|pushurl|fetch|prune)|fetch\.prune)$'`, with <name>
// regex-escaped, and parses the NUL-separated "key\nvalue" records it prints. A
// record with no newline is a key with no value. Exit 1 means none of the keys
// is set: the zero RemoteCfg and no error. Any other non-zero exit (a broken
// config file is 128) is an error, because it must never read as "unset".
//
// A url, pushurl or fetch with no value, and a prune that is not a boolean, are
// errors, as git itself refuses them. For url and pushurl the FIRST value is
// kept (git fetches from the first url), except that an empty value clears
// the list before it, which git's documentation says of both keys (git help
// config, git 2.54.0): the first value after the last empty one is kept. For
// prune the LAST wins, as for any single-valued key.
//
// remote.<name>.mirror is deliberately not read (MA1).
func RemoteConfig(gitDir, name string) (RemoteCfg, error) {
	pattern := `^(remote\.` + regexp.QuoteMeta(name) + `\.(url|pushurl|fetch|prune)|fetch\.prune)$`
	args := []string{"config", "-z", "--get-regexp", pattern}
	out, stderr, code, err := gitStdout(gitDir, args...)
	if code == 1 {
		return RemoteCfg{}, nil // none of the keys is set
	}
	if err != nil || code != 0 {
		return RemoteCfg{}, gitStdoutError(args, stderr, code, err)
	}
	// Every record ends in NUL. Output that does not was cut short, or is not
	// what -z promises; either way it cannot be trusted.
	if len(out) == 0 || out[len(out)-1] != 0 {
		return RemoteCfg{}, fmt.Errorf("git config -z --get-regexp exited 0 with output that is "+
			"empty or not NUL-terminated (%q); refusing to read it", out)
	}

	prefix := "remote." + name + "."
	var cfg RemoteCfg
	var haveURL, havePushURL, haveRemotePrune, haveFetchPrune bool
	var remotePrune, fetchPrune bool
	for _, rec := range bytes.Split(out[:len(out)-1], []byte{0}) {
		key, value, hasValue := strings.Cut(string(rec), "\n")
		switch key {
		case prefix + "url":
			if !hasValue {
				return RemoteCfg{}, fmt.Errorf("%s has no value", key)
			}
			switch {
			case value == "": // clears the list, as in git
				cfg.URL, haveURL = "", false
			case !haveURL:
				cfg.URL, haveURL = value, true
			}
		case prefix + "pushurl":
			if !hasValue {
				return RemoteCfg{}, fmt.Errorf("%s has no value", key)
			}
			switch {
			case value == "": // clears the list, as in git
				cfg.PushURL, havePushURL = "", false
			case !havePushURL:
				cfg.PushURL, havePushURL = value, true
			}
		case prefix + "fetch":
			if !hasValue {
				return RemoteCfg{}, fmt.Errorf("%s has no value", key)
			}
			cfg.Fetch = append(cfg.Fetch, value)
		case prefix + "prune":
			b, perr := parseGitBool(value, hasValue)
			if perr != nil {
				return RemoteCfg{}, fmt.Errorf("%s: %w", key, perr)
			}
			remotePrune, haveRemotePrune = b, true
		case "fetch.prune":
			b, perr := parseGitBool(value, hasValue)
			if perr != nil {
				return RemoteCfg{}, fmt.Errorf("%s: %w", key, perr)
			}
			fetchPrune, haveFetchPrune = b, true
		default:
			return RemoteCfg{}, fmt.Errorf("git config -z --get-regexp returned the unexpected key %q", key)
		}
	}
	switch {
	case haveRemotePrune:
		cfg.Prune = remotePrune
	case haveFetchPrune:
		cfg.Prune = fetchPrune
	}
	return cfg, nil
}

// gitIntRe is the shape of an integer git_parse_int accepts: an optional sign,
// decimal, octal (leading 0) or hexadecimal digits, and one optional unit
// suffix. Anything else, underscores and Go's 0b prefix included, is not one.
var gitIntRe = regexp.MustCompile(`^[+-]?(0[xX][0-9a-fA-F]+|[0-9]+)[kKmMgG]?$`)

// parseGitBool reads a config value the way git_parse_maybe_bool does. A key
// with no value (hasValue false) is true; "" is false; true, yes and on, and
// false, no and off, in any case, are what they say; any other integer is true
// unless it is zero. Everything else is an error, as git dies with "bad boolean
// config value" on it.
func parseGitBool(value string, hasValue bool) (bool, error) {
	if !hasValue {
		return true, nil
	}
	switch strings.ToLower(value) {
	case "":
		return false, nil
	case "true", "yes", "on":
		return true, nil
	case "false", "no", "off":
		return false, nil
	}
	if m := gitIntRe.FindStringSubmatch(value); m != nil {
		n, err := strconv.ParseUint(m[1], 0, 64)
		if err == nil {
			return n != 0, nil
		}
	}
	return false, fmt.Errorf("bad boolean config value %q", value)
}

// LocalRefs returns the name of every ref in the repository at gitDir, mapped to
// the ref it points at when it is symbolic ("refs/remotes/o/HEAD" to
// "refs/remotes/o/main") and to "" when it is direct. It reads `git for-each-ref
// --format=%(refname)%00%(symref)` and splits stdout on "\n" only, each line at
// its NUL, so every name keeps the bytes git printed: this code neither trims
// nor normalises a name.
//
// What git prints is git's view under the repository's config (and whatever
// config the caller's environment passes it), not the bytes on disk. With
// core.precomposeunicode on, which git init and git clone set on a Mac, git
// reads the name of a loose ref file precomposed to NFC and a packed-refs
// entry by its bytes. So a loose NFD ref is reported under its NFC name, and
// beside a packed entry of the same NFD name git lists both, the packed one at
// its stale value (measured 2026-10-28, git 2.54.0;
// docs/research/probes/macos-port/lone-nfd-default/, and
// TestLocalRefsAtTheMacDefaultReportsALooseNFDNamePrecomposed). With it off,
// each name is listed by the bytes it is stored under.
//
// Stdout alone is parsed, and anything git says on stderr stops the read. git
// skips a ref file it cannot parse, exits 0, and warns on stderr
// ("ignoring broken ref"); that ref is content the helper cannot read, so the
// design stops rather than skips (spec section 3) and a clean exit with a
// non-empty stderr is an error carrying git's words. The test is "stderr is not
// empty", never a match on the message, which is localised.
//
// Known limit (controller ruling T6-2): a dangling symbolic ref, for example a
// refs/remotes/<r>/HEAD left behind by a prune, is not listed and produces no
// warning. Every git enumerator leaves it out silently (only fsck sees it), so
// the caller's collision check cannot see it either.
func LocalRefs(gitDir string) (map[string]string, error) {
	args := []string{"for-each-ref", "--format=%(refname)%00%(symref)"}
	out, stderr, code, err := gitStdout(gitDir, args...)
	if err != nil || code != 0 {
		return nil, gitStdoutError(args, stderr, code, err)
	}
	if strings.TrimSpace(stderr) != "" {
		return nil, fmt.Errorf("git %s exited 0 but wrote to stderr, so it may have skipped a ref; "+
			"refusing to act on a possibly incomplete ref list: %s",
			strings.Join(args, " "), strings.TrimSpace(stderr))
	}
	refs := map[string]string{}
	if len(out) == 0 {
		return refs, nil
	}
	// Every line ends in "\n". Output that does not was cut short.
	if out[len(out)-1] != '\n' {
		return nil, fmt.Errorf("git %s: output does not end in a newline (%d bytes); refusing to "+
			"read a possibly truncated ref list", strings.Join(args, " "), len(out))
	}
	for _, line := range bytes.Split(out[:len(out)-1], []byte{'\n'}) {
		name, target, ok := bytes.Cut(line, []byte{0})
		if !ok || len(name) == 0 {
			return nil, fmt.Errorf("git %s: unparseable line %q", strings.Join(args, " "), line)
		}
		refs[string(name)] = string(target)
	}
	return refs, nil
}
