package gitcmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// isolateGitConfig keeps every git process the test starts, including the ones
// RefFormat, RemoteConfig and LocalRefs start (they inherit this process's
// environment), away from the host's system and global configuration, so a
// developer's own fetch.prune or init.defaultBranch cannot change a result. It
// returns the path of the empty global config file, for a test that wants a
// global value. GIT_DIR and its relatives are cleared too, so a run inside a git
// hook cannot be redirected at the hook's repository. LC_ALL=C keeps git's
// messages in English.
func isolateGitConfig(t *testing.T) string {
	t.Helper()
	global := filepath.Join(t.TempDir(), "global.gitconfig")
	if err := os.WriteFile(global, nil, 0o644); err != nil {
		t.Fatalf("write empty global config: %v", err)
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("LC_ALL", "C")
	for _, k := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE"} {
		t.Setenv(k, "") // registers the restore; the next line is the real change
		os.Unsetenv(k)
	}
	return global
}

// localGit runs `git -C dir args...`, fails the test if it fails, and returns
// its stdout.
func localGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		var stderr []byte
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			stderr = ee.Stderr
		}
		t.Fatalf("git %v: %v: %s", args, err, stderr)
	}
	return string(out)
}

// localGitCode runs `git -C dir args...` directly, WITHOUT gitStdout, and returns
// its exit code and stderr. Fixture checks use it so that a precondition never
// depends on the code the test is about to judge. A command that never starts
// fails the test.
func localGitCode(t *testing.T, dir string, args ...string) (code int, stderr string) {
	t.Helper()
	var errBuf strings.Builder
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if cmd.ProcessState == nil {
		t.Fatalf("git %v never started: %v", args, err)
	}
	return cmd.ProcessState.ExitCode(), errBuf.String()
}

// localRepo makes a throwaway repository under t.TempDir() with one empty commit
// on main, isolated from the host's git configuration (see isolateGitConfig).
// initArgs go to `git init`, for --ref-format=reftable. The identity is set
// per repository, never with `git config --global`.
func localRepo(t *testing.T, initArgs ...string) string {
	t.Helper()
	isolateGitConfig(t)
	d := t.TempDir()
	localGit(t, d, append([]string{"init", "-q", "-b", "main"}, initArgs...)...)
	localGit(t, d, "config", "user.email", "t@t")
	localGit(t, d, "config", "user.name", "t")
	localGit(t, d, "commit", "-q", "--allow-empty", "-m", "c1")
	return d
}

// localCreateRefs creates each named ref at HEAD through `update-ref --stdin`,
// so the names reach git as stdin bytes and never as argv (on macOS, git
// precomposes non-ASCII argv).
func localCreateRefs(t *testing.T, dir string, names ...string) {
	t.Helper()
	oid := strings.TrimSpace(localGit(t, dir, "rev-parse", "HEAD"))
	var in strings.Builder
	for _, n := range names {
		in.WriteString("create " + n + " " + oid + "\n")
	}
	cmd := exec.Command("git", "-C", dir, "update-ref", "--stdin")
	cmd.Stdin = strings.NewReader(in.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git update-ref --stdin (%q): %v: %s", names, err, out)
	}
}

// appendFile appends text to the file at path, creating it if need be.
func appendFile(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	if _, err := f.WriteString(text); err != nil {
		f.Close()
		t.Fatalf("write %s: %v", path, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close %s: %v", path, err)
	}
}

// RED. The ref format decides whether the helper may treat refs as files, so
// both values git reports must come back as git spells them. reftable needs a
// git that can create one (the suite's floor, 2.45.0).
func TestRefFormatReadsFilesAndReftable(t *testing.T) {
	for _, want := range []string{"files", "reftable"} {
		t.Run(want, func(t *testing.T) {
			d := localRepo(t, "--ref-format="+want)
			got, err := RefFormat(filepath.Join(d, ".git"))
			if err != nil {
				t.Fatalf("RefFormat on a %s repository: %v", want, err)
			}
			if got != want {
				t.Errorf("RefFormat = %q, want %q", got, want)
			}
		})
	}
}

// RED. A directory that is not a repository is an error that names the command
// and carries git's own stderr, never a guess at "files". GIT_CEILING_DIRECTORIES
// stops discovery at the directory's parent, so a temp directory that happens to
// sit inside a repository cannot rescue the call.
func TestRefFormatFailsOnANonRepository(t *testing.T) {
	isolateGitConfig(t)
	d := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(d))

	code, gitErr := localGitCode(t, d, "rev-parse", "--show-ref-format")
	if code == 0 || strings.TrimSpace(gitErr) == "" {
		t.Fatalf("fixture: git rev-parse --show-ref-format in a non-repository = exit %d, stderr %q; want a failure with a message", code, gitErr)
	}

	got, err := RefFormat(d)
	if err == nil {
		t.Fatalf("RefFormat(non-repository) = %q, nil; want an error", got)
	}
	if got != "" {
		t.Errorf("RefFormat(non-repository) = %q alongside the error, want an empty format", got)
	}
	if !strings.Contains(err.Error(), "rev-parse --show-ref-format") {
		t.Errorf("error %q does not name the command", err)
	}
	if !strings.Contains(err.Error(), strings.TrimSpace(gitErr)) {
		t.Errorf("error %q does not carry git's stderr %q", err, strings.TrimSpace(gitErr))
	}
}

// RED. Stdout minus ONE trailing newline must be exactly files or reftable.
// Anything else is an error, and a warning git writes on stderr is not part of
// the answer. A POSIX shell script stands in for git so each output is
// deterministic (skipped on Windows, like the other fake-git tests).
func TestRefFormatRefusesAnythingButFilesOrReftable(t *testing.T) {
	cases := []struct {
		name, script string
		want         string // "" means an error is expected
	}{
		{"files", `printf 'files\n'`, "files"},
		{"reftable", `printf 'reftable\n'`, "reftable"},
		{"no newline at all", `printf 'files'`, "files"},
		{"stderr noise is not the answer", `echo 'warning: noise' >&2; printf 'files\n'`, "files"},
		{"two newlines", `printf 'files\n\n'`, ""},
		{"CRLF", `printf 'files\r\n'`, ""},
		{"wrong case", `printf 'Files\n'`, ""},
		{"an unknown format", `printf 'sha1\n'`, ""},
		{"the flag echoed back", `printf -- '--show-ref-format\n'`, ""},
		{"nothing", `printf ''`, ""},
		{"a non-zero exit with a plausible answer", `printf 'files\n'; echo 'fatal: boom' >&2; exit 3`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeGitOnPath(t, c.script)
			got, err := RefFormat(t.TempDir())
			if c.want == "" {
				if err == nil {
					t.Fatalf("RefFormat = %q, nil; want an error", got)
				}
				if got != "" {
					t.Errorf("RefFormat = %q alongside the error, want empty", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("RefFormat: %v", err)
			}
			if got != c.want {
				t.Errorf("RefFormat = %q, want %q", got, c.want)
			}
		})
	}
}

// RED (Review Focus 2). Git splits a config key at its FIRST and LAST dot, so a
// remote named back.up has the keys remote.back.up.url and
// remote.back.up.fetch. The refspecs come back exactly, in order, the negative
// one included. The decoys prove the name is regex-escaped (an unescaped dot
// would match backXup) and that a shorter remote name does not leak in.
func TestRemoteConfigReadsADottedName(t *testing.T) {
	d := localRepo(t)
	for _, kv := range [][2]string{
		{"remote.back.up.url", "/srv/back-up"},
		{"remote.back.up.fetch", "+refs/heads/*:refs/remotes/back.up/*"},
		{"remote.back.up.fetch", "+refs/tags/*:refs/tags/*"},
		{"remote.back.up.fetch", "^refs/heads/skip"},
		{"remote.backXup.url", "/srv/decoy-x"},
		{"remote.backXup.fetch", "+refs/heads/*:refs/remotes/backXup/*"},
		{"remote.back.url", "/srv/decoy-back"},
		{"remote.back.fetch", "+refs/heads/*:refs/remotes/back/*"},
	} {
		localGit(t, d, "config", "--add", kv[0], kv[1])
	}
	want := RemoteCfg{
		URL: "/srv/back-up",
		Fetch: []string{
			"+refs/heads/*:refs/remotes/back.up/*",
			"+refs/tags/*:refs/tags/*",
			"^refs/heads/skip",
		},
	}
	got, err := RemoteConfig(filepath.Join(d, ".git"), "back.up")
	if err != nil {
		t.Fatalf("RemoteConfig(back.up): %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RemoteConfig(back.up) = %+v, want %+v", got, want)
	}
}

// GUARD (it passes against a do-nothing stub, so it cannot be RED; it fails
// as soon as the exit-1 branch is removed). A name with nothing set is the zero
// value, not an error: exit 1 from git config means "none of the keys is set".
// That covers the anonymous URL's would-be key (PM4), a remote that does not
// exist, and a url that is set but empty (which exits 0 and must read back as
// ""). The preconditions run git directly so the test cannot pass through the
// wrong branch.
func TestRemoteConfigUnsetIsNotAnError(t *testing.T) {
	d := localRepo(t)
	gitDir := filepath.Join(d, ".git")
	localGit(t, d, "config", "remote.origin.url", "/srv/origin")
	localGit(t, d, "config", "remote.empty.url", "")

	const anon = "proton::/my-files/a.b"
	code, _ := localGitCode(t, d, "config", "--get", "remote."+anon+".url")
	if code != 1 {
		t.Fatalf("fixture: git config --get remote.%s.url exited %d, want 1 (unset)", anon, code)
	}
	code, _ = localGitCode(t, d, "config", "--get-all", "remote."+anon+".fetch")
	if code != 1 {
		t.Fatalf("fixture: git config --get-all remote.%s.fetch exited %d, want 1 (unset)", anon, code)
	}
	code, _ = localGitCode(t, d, "config", "-z", "--get-regexp", `^remote\.empty\.url$`)
	if code != 0 {
		t.Fatalf("fixture: a remote whose url is set but empty exited %d, want 0 (set)", code)
	}

	for _, name := range []string{anon, "no-such-remote", "empty"} {
		got, err := RemoteConfig(gitDir, name)
		if err != nil {
			t.Errorf("RemoteConfig(%q): %v, want no error", name, err)
		}
		if !reflect.DeepEqual(got, RemoteCfg{}) {
			t.Errorf("RemoteConfig(%q) = %+v, want the zero value", name, got)
		}
	}
}

// RED. When git exits 0 but a grandchild still holds stdout open, the output may
// be cut short, and here the output is a ref list or a config dump that a caller
// would trust. gitStdout hands back exec.ErrWaitDelay as an error, no stdout,
// and a non-zero code, so a caller that checks only the code fails closed too.
//
// The fake git prints one name, leaves `sleep 20` holding the pipes, and exits 0.
// waitDelay is shortened to 250 ms; the sleep outlasts the 10 s bound, so the
// bound holds only while gitStdout sets cmd.WaitDelay. With no WaitDelay, Wait
// blocks until the sleep ends and returns a nil error. Skipped on Windows, like
// the other fake-git tests.
func TestGitStdoutTreatsWaitDelayAsAnError(t *testing.T) {
	old := waitDelay
	waitDelay = 250 * time.Millisecond
	t.Cleanup(func() { waitDelay = old })
	fakeGitOnPath(t, "echo refs/heads/partial\nsleep 20 &\nexit 0")

	start := time.Now()
	out, _, code, err := gitStdout(t.TempDir(), "for-each-ref")
	took := time.Since(start)

	if !errors.Is(err, exec.ErrWaitDelay) {
		t.Fatalf("gitStdout err = %v, want one wrapping exec.ErrWaitDelay (stdout %q, code %d)", err, out, code)
	}
	if len(out) != 0 {
		t.Errorf("gitStdout stdout = %q alongside the error, want none: it may be truncated", out)
	}
	if code != -1 {
		t.Errorf("gitStdout code = %d, want -1 so a code-only check fails closed", code)
	}
	if took > 10*time.Second {
		t.Errorf("gitStdout took %s, want it bounded near waitDelay (250ms), not the sleep", took)
	}
}

// RED. gitStdout keeps stdout and stderr apart and trims nothing, which git()
// does not: the trailing newline survives, a failing command's message lands in
// stderr and not in stdout, and a non-zero exit reports its code and an
// *exec.ExitError the way git() does.
func TestGitStdoutKeepsStdoutAndStderrApartAndTrimsNothing(t *testing.T) {
	d := localRepo(t)

	out, stderr, code, err := gitStdout(d, "rev-parse", "--show-ref-format")
	if err != nil || code != 0 {
		t.Fatalf("gitStdout(rev-parse --show-ref-format) = code %d, err %v", code, err)
	}
	if string(out) != "files\n" {
		t.Errorf("stdout = %q, want %q untrimmed", out, "files\n")
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}

	out, stderr, code, err = gitStdout(d, "cat-file", "-t", "0123456789abcdef0123456789abcdef01234567")
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Errorf("err = %v, want an *exec.ExitError for a non-zero exit", err)
	}
	if code == 0 {
		t.Errorf("code = 0, want git's non-zero exit")
	}
	if len(out) != 0 {
		t.Errorf("stdout = %q, want none: git's complaint belongs on stderr", out)
	}
	if strings.TrimSpace(stderr) == "" {
		t.Errorf("stderr is empty, want git's complaint")
	}
}

// RED. A git that never starts reports code -1 and the start error, as git()
// does, and does not panic on the missing ProcessState.
func TestGitStdoutReportsAGitThatNeverStarted(t *testing.T) {
	d := localRepo(t)
	t.Setenv("PATH", t.TempDir()) // nothing on it

	out, _, code, err := gitStdout(d, "rev-parse", "--show-ref-format")
	if err == nil {
		t.Fatalf("gitStdout with no git on PATH = %q, nil; want the start error", out)
	}
	if code != -1 {
		t.Errorf("code = %d, want -1 for a process that never started", code)
	}
}

// RED. gitStdout scrubs GIT_ALTERNATE_OBJECT_DIRECTORIES as git() does: a value
// in this process's environment must not reach the child. A fake git echoes the
// variable (skipped on Windows, like the other fake-git tests).
func TestGitStdoutScrubsTheAlternateObjectDirectories(t *testing.T) {
	t.Setenv("GIT_ALTERNATE_OBJECT_DIRECTORIES", "/leaked/alternate")
	fakeGitOnPath(t, `printf '[%s]' "$GIT_ALTERNATE_OBJECT_DIRECTORIES"`)

	out, _, code, err := gitStdout(t.TempDir(), "for-each-ref")
	if err != nil || code != 0 {
		t.Fatalf("gitStdout = code %d, err %v", code, err)
	}
	if string(out) != "[]" {
		t.Errorf("the child saw GIT_ALTERNATE_OBJECT_DIRECTORIES as %q, want it set and empty", out)
	}
}

// RED. A broken config file makes git config exit 128, and that is an error: it
// must never read as "nothing is set", which would let a caller proceed as if
// the remote had no refspecs.
func TestRemoteConfigFailsOnABrokenConfigFile(t *testing.T) {
	d := localRepo(t)
	gitDir := filepath.Join(d, ".git")
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("this is not a config file\n"), 0o644); err != nil {
		t.Fatalf("break the config file: %v", err)
	}
	code, gitErr := localGitCode(t, gitDir, "config", "-z", "--get-regexp", "^remote")
	if code == 0 || code == 1 || strings.TrimSpace(gitErr) == "" {
		t.Fatalf("fixture: git config on a broken file = exit %d, stderr %q; want a failure other than 1", code, gitErr)
	}

	got, err := RemoteConfig(gitDir, "origin")
	if err == nil {
		t.Fatalf("RemoteConfig on a broken config file = %+v, nil; want an error", got)
	}
	if !strings.Contains(err.Error(), "git config") {
		t.Errorf("error %q does not name the command", err)
	}
	if !strings.Contains(err.Error(), strings.TrimSpace(gitErr)) {
		t.Errorf("error %q does not carry git's stderr %q", err, strings.TrimSpace(gitErr))
	}
}

// RED. pushurl is read with no url beside it, and Prune is remote.<name>.prune
// when that is set, else fetch.prune, read as git reads a boolean: a key with no
// value is true, a later value wins, and fetch.prune counts from the global file
// too (git config reads every scope).
func TestRemoteConfigReadsPushURLAndPrune(t *testing.T) {
	cases := []struct {
		name   string
		global string // appended to the global config
		local  string // appended to the repository's config
		want   RemoteCfg
	}{
		{"pushurl with no url", "", "[remote \"p\"]\n\tpushurl = /srv/push\n",
			RemoteCfg{PushURL: "/srv/push"}},
		{"url and pushurl are told apart", "", "[remote \"p\"]\n\turl = /srv/fetch\n\tpushurl = /srv/push\n",
			RemoteCfg{URL: "/srv/fetch", PushURL: "/srv/push"}},
		{"no prune anywhere", "", "[remote \"p\"]\n\turl = /srv/u\n",
			RemoteCfg{URL: "/srv/u"}},
		{"fetch.prune alone", "", "[remote \"p\"]\n\turl = /srv/u\n[fetch]\n\tprune = true\n",
			RemoteCfg{URL: "/srv/u", Prune: true}},
		{"fetch.prune false alone", "", "[remote \"p\"]\n\turl = /srv/u\n[fetch]\n\tprune = false\n",
			RemoteCfg{URL: "/srv/u"}},
		{"remote true over fetch false", "", "[remote \"p\"]\n\tprune = true\n[fetch]\n\tprune = false\n",
			RemoteCfg{Prune: true}},
		{"remote false over fetch true", "", "[remote \"p\"]\n\tprune = false\n[fetch]\n\tprune = true\n",
			RemoteCfg{}},
		{"fetch.prune from the global file", "[fetch]\n\tprune = true\n", "[remote \"p\"]\n\turl = /srv/u\n",
			RemoteCfg{URL: "/srv/u", Prune: true}},
		{"remote false over a global fetch.prune", "[fetch]\n\tprune = true\n", "[remote \"p\"]\n\tprune = no\n",
			RemoteCfg{}},
		{"a key with no value is true", "", "[remote \"p\"]\n\tprune\n",
			RemoteCfg{Prune: true}},
		{"a later value wins", "", "[remote \"p\"]\n\tprune = true\n\tprune = off\n",
			RemoteCfg{}},
		{"yes", "", "[remote \"p\"]\n\tprune = Yes\n",
			RemoteCfg{Prune: true}},
		{"a non-zero integer", "", "[remote \"p\"]\n\tprune = 2\n",
			RemoteCfg{Prune: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := localRepo(t)
			gitDir := filepath.Join(d, ".git")
			appendFile(t, os.Getenv("GIT_CONFIG_GLOBAL"), c.global)
			appendFile(t, filepath.Join(gitDir, "config"), c.local)
			got, err := RemoteConfig(gitDir, "p")
			if err != nil {
				t.Fatalf("RemoteConfig(p): %v", err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("RemoteConfig(p) = %+v, want %+v", got, c.want)
			}
		})
	}
}

// RED. What git itself would refuse, RemoteConfig refuses rather than guess at:
// a prune that is not a boolean, and a url, pushurl or fetch with no value.
func TestRemoteConfigRefusesWhatGitWouldRefuse(t *testing.T) {
	cases := []struct{ name, local, mention string }{
		{"a prune that is not a boolean", "[remote \"p\"]\n\tprune = maybe\n", "maybe"},
		{"fetch.prune that is not a boolean", "[fetch]\n\tprune = sometimes\n", "sometimes"},
		{"a url with no value", "[remote \"p\"]\n\turl\n", "remote.p.url"},
		{"a pushurl with no value", "[remote \"p\"]\n\tpushurl\n", "remote.p.pushurl"},
		{"a fetch with no value", "[remote \"p\"]\n\tfetch\n", "remote.p.fetch"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := localRepo(t)
			gitDir := filepath.Join(d, ".git")
			appendFile(t, filepath.Join(gitDir, "config"), c.local)
			got, err := RemoteConfig(gitDir, "p")
			if err == nil {
				t.Fatalf("RemoteConfig(p) = %+v, nil; want an error", got)
			}
			if !strings.Contains(err.Error(), c.mention) {
				t.Errorf("error %q does not mention %q", err, c.mention)
			}
		})
	}
}

// RED. A remote may carry several url or pushurl values. RemoteCfg has room
// for one of each, so it is the FIRST, which is the one git fetches from.
func TestRemoteConfigTakesTheFirstOfSeveralURLs(t *testing.T) {
	d := localRepo(t)
	gitDir := filepath.Join(d, ".git")
	for _, kv := range [][2]string{
		{"remote.m.url", "/srv/first"}, {"remote.m.url", "/srv/second"},
		{"remote.m.pushurl", "/srv/push-first"}, {"remote.m.pushurl", "/srv/push-second"},
	} {
		localGit(t, d, "config", "--add", kv[0], kv[1])
	}
	got, err := RemoteConfig(gitDir, "m")
	if err != nil {
		t.Fatalf("RemoteConfig(m): %v", err)
	}
	want := RemoteCfg{URL: "/srv/first", PushURL: "/srv/push-first"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RemoteConfig(m) = %+v, want %+v", got, want)
	}
}

// RED. parseGitBool reads a value as git_parse_maybe_bool does: true, yes, on in
// any case are true; false, no, off and the empty string are false; any other
// integer (a unit suffix allowed) is true unless it is zero; a key with no value
// is true; everything else is an error.
func TestParseGitBool(t *testing.T) {
	cases := []struct {
		value    string
		hasValue bool
		want     bool
		wantErr  bool
	}{
		{"", false, true, false}, // the key is present with no "="
		{"", true, false, false}, // "prune =" with nothing after it
		{"true", true, true, false}, {"TRUE", true, true, false}, {"Yes", true, true, false}, {"on", true, true, false},
		{"false", true, false, false}, {"False", true, false, false}, {"no", true, false, false}, {"OFF", true, false, false},
		{"1", true, true, false}, {"2", true, true, false}, {"-1", true, true, false}, {"+3", true, true, false},
		{"0", true, false, false}, {"-0", true, false, false}, {"00", true, false, false},
		{"0x10", true, true, false}, {"0x0", true, false, false}, {"010", true, true, false},
		{"1k", true, true, false}, {"2M", true, true, false}, {"0g", true, false, false},
		{"maybe", true, false, true}, {"y", true, false, true}, {"tru", true, false, true},
		{"09", true, false, true}, {"0b1", true, false, true}, {"1_0", true, false, true},
		{"1.5", true, false, true}, {" true", true, false, true}, {"1 ", true, false, true},
		{"99999999999999999999999", true, false, true},
	}
	for _, c := range cases {
		got, err := parseGitBool(c.value, c.hasValue)
		if (err != nil) != c.wantErr {
			t.Errorf("parseGitBool(%q, %v) error = %v, want error %v", c.value, c.hasValue, err, c.wantErr)
			continue
		}
		if err == nil && got != c.want {
			t.Errorf("parseGitBool(%q, %v) = %v, want %v", c.value, c.hasValue, got, c.want)
		}
	}
}

// RED. Both spellings of "cafe, accented" coexist in a reftable repository (the files
// backend cannot hold them on a normalising filesystem), and LocalRefs returns
// each one's bytes untouched: no normalisation, no trimming, two keys. The
// names reach git on stdin, and core.precomposeunicode is off, so neither macOS
// argv precomposition nor a checkout can fold one spelling into the other.
func TestLocalRefsKeepsNFDBytes(t *testing.T) {
	d := localRepo(t, "--ref-format=reftable")
	localGit(t, d, "config", "core.precomposeunicode", "false")
	const nfc = "refs/heads/caf\u00e9"  // the accented e as one code point
	const nfd = "refs/heads/cafe\u0301" // e, then a combining acute accent
	if nfc == nfd {
		t.Fatalf("fixture: the two spellings are equal")
	}
	localCreateRefs(t, d, nfc, nfd)

	got, err := LocalRefs(filepath.Join(d, ".git"))
	if err != nil {
		t.Fatalf("LocalRefs: %v", err)
	}
	want := map[string]string{"refs/heads/main": "", nfc: "", nfd: ""}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LocalRefs = %q, want %q", got, want)
	}
}

// RED. A symbolic ref maps to its target and a direct ref to "". Git does not
// list HEAD itself here, only refs/ (the remote-tracking HEAD is under refs/).
func TestLocalRefsReportsSymbolicTargets(t *testing.T) {
	d := localRepo(t)
	localCreateRefs(t, d, "refs/remotes/o/main")
	localGit(t, d, "symbolic-ref", "refs/remotes/o/HEAD", "refs/remotes/o/main")

	got, err := LocalRefs(filepath.Join(d, ".git"))
	if err != nil {
		t.Fatalf("LocalRefs: %v", err)
	}
	want := map[string]string{
		"refs/heads/main":     "",
		"refs/remotes/o/main": "",
		"refs/remotes/o/HEAD": "refs/remotes/o/main",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LocalRefs = %q, want %q", got, want)
	}
}

// RED (fix round 1, finding I1). When git exits 0 but says something on stderr
// while listing refs, LocalRefs fails closed: it names the command, carries
// git's own words, and returns no map. The fixture holds a dangling symbolic ref
// and a ref file with garbage in it. The garbage file is what makes git 2.54.0
// print "warning: ignoring broken ref" on stderr and leave that ref out of the
// listing (measured 2026-10-27); the dangling symbolic ref alone is skipped
// silently. The ref is a file on disk that the helper cannot read, so the design
// (spec section 3) stops rather than skips. The precondition asserts the warning
// exists, so the test cannot pass vacuously on a git that stays quiet.
func TestLocalRefsFailsClosedOnAWarning(t *testing.T) {
	d := localRepo(t)
	localGit(t, d, "symbolic-ref", "refs/heads/dangling", "refs/heads/nope")
	if err := os.WriteFile(filepath.Join(d, ".git", "refs", "heads", "garbage"), []byte("garbage\n"), 0o644); err != nil {
		t.Fatalf("write the broken ref file: %v", err)
	}
	code, warn := localGitCode(t, d, "for-each-ref", "--format=%(refname)%00%(symref)")
	if code != 0 || !strings.Contains(warn, "garbage") {
		t.Fatalf("fixture: for-each-ref = exit %d, stderr %q; want a clean exit with a warning naming the broken ref", code, warn)
	}

	got, err := LocalRefs(filepath.Join(d, ".git"))
	if err == nil {
		t.Fatalf("LocalRefs = %q, nil; want an error: git skipped a ref and said so on stderr", got)
	}
	if got != nil {
		t.Errorf("LocalRefs = %q alongside the error, want no map", got)
	}
	if !strings.Contains(err.Error(), "for-each-ref") {
		t.Errorf("error %q does not name the command", err)
	}
	if !strings.Contains(err.Error(), "garbage") {
		t.Errorf("error %q does not carry git's warning, which names the ref", err)
	}
	if !strings.Contains(err.Error(), strings.TrimSpace(warn)) {
		t.Errorf("error %q does not carry git's stderr %q", err, strings.TrimSpace(warn))
	}
}

// RED (fix round 1, finding I1). The decision turns on stderr being non-empty,
// never on what it says: git's messages are localised and the helper runs in the
// user's locale. A fake git exits 0, lists one ref, and complains in French.
// Skipped on Windows, like the other fake-git tests.
func TestLocalRefsFailsClosedOnAnyStderrText(t *testing.T) {
	fakeGitOnPath(t, "printf 'refs/heads/main\\000\\n'\necho 'avertissement : reference ignoree' >&2")
	got, err := LocalRefs(t.TempDir())
	if err == nil {
		t.Fatalf("LocalRefs = %q, nil; want an error for any text on stderr", got)
	}
	if !strings.Contains(err.Error(), "avertissement") {
		t.Errorf("error %q does not carry git's stderr", err)
	}
}

// RED. A directory that is not a repository is an error here too.
func TestLocalRefsFailsOnANonRepository(t *testing.T) {
	isolateGitConfig(t)
	d := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(d))
	got, err := LocalRefs(d)
	if err == nil {
		t.Fatalf("LocalRefs(non-repository) = %q, nil; want an error", got)
	}
	if !strings.Contains(err.Error(), "for-each-ref") {
		t.Errorf("error %q does not name the command", err)
	}
}

// RED (Review Focus 4, PM5). The helper is handed the per-worktree git dir of a
// linked worktree, .git/worktrees/<w>. All three reads give what the main
// repository's .git gives (measured on git 2.54.0, 2026-10-27). The fixture
// carries non-trivial answers, so two empty results cannot agree by accident.
func TestLocalReadsInALinkedWorktree(t *testing.T) {
	main := localRepo(t)
	mainGit := filepath.Join(main, ".git")
	for _, kv := range [][2]string{
		{"remote.origin.url", "/srv/origin"},
		{"remote.origin.pushurl", "/srv/origin-push"},
		{"remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"},
		{"remote.origin.fetch", "^refs/heads/skip"},
		{"remote.origin.prune", "true"},
	} {
		localGit(t, main, "config", "--add", kv[0], kv[1])
	}
	localCreateRefs(t, main, "refs/remotes/origin/main")
	localGit(t, main, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")

	wt := filepath.Join(t.TempDir(), "wt")
	localGit(t, main, "worktree", "add", "-q", wt, "-b", "wt-branch")
	linkedGit := strings.TrimSpace(localGit(t, wt, "rev-parse", "--absolute-git-dir"))
	if !strings.Contains(filepath.ToSlash(linkedGit), "/worktrees/") {
		t.Fatalf("absolute git dir %q is not a linked worktree's: the fixture is wrong", linkedGit)
	}

	wantFormat, err := RefFormat(mainGit)
	if err != nil {
		t.Fatalf("RefFormat(main): %v", err)
	}
	gotFormat, err := RefFormat(linkedGit)
	if err != nil {
		t.Fatalf("RefFormat(linked worktree): %v", err)
	}
	if gotFormat != wantFormat || gotFormat != "files" {
		t.Errorf("RefFormat(linked worktree) = %q, main = %q, want both files", gotFormat, wantFormat)
	}

	wantCfg, err := RemoteConfig(mainGit, "origin")
	if err != nil {
		t.Fatalf("RemoteConfig(main): %v", err)
	}
	gotCfg, err := RemoteConfig(linkedGit, "origin")
	if err != nil {
		t.Fatalf("RemoteConfig(linked worktree): %v", err)
	}
	if wantCfg.URL != "/srv/origin" || len(wantCfg.Fetch) != 2 || !wantCfg.Prune {
		t.Fatalf("fixture: RemoteConfig(main) = %+v, want a populated answer to compare against", wantCfg)
	}
	if !reflect.DeepEqual(gotCfg, wantCfg) {
		t.Errorf("RemoteConfig(linked worktree) = %+v, main = %+v", gotCfg, wantCfg)
	}

	wantRefs, err := LocalRefs(mainGit)
	if err != nil {
		t.Fatalf("LocalRefs(main): %v", err)
	}
	gotRefs, err := LocalRefs(linkedGit)
	if err != nil {
		t.Fatalf("LocalRefs(linked worktree): %v", err)
	}
	if _, ok := wantRefs["refs/heads/wt-branch"]; !ok || wantRefs["refs/remotes/origin/HEAD"] != "refs/remotes/origin/main" {
		t.Fatalf("fixture: LocalRefs(main) = %q, want the worktree branch and the symbolic ref in it", wantRefs)
	}
	if !reflect.DeepEqual(gotRefs, wantRefs) {
		t.Errorf("LocalRefs(linked worktree) = %q, main = %q", gotRefs, wantRefs)
	}
}
