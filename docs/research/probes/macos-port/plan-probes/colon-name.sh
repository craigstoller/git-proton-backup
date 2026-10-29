#!/bin/bash
# Can a git remote name contain ':'? And what does `git config --get` return for
# remote.<anonymous URL>.url? Local only, logging stand-in helper.
set -u
H="$1"; W="$2"; rm -rf "$W"; mkdir -p "$W"
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null PATH="$H:$PATH"
export LOGTEST_SHA=0123456789012345678901234567890123456789
git init -q "$W/repo"; cd "$W/repo" || exit 1
echo "--- git remote add 'a:b':"
git remote add 'a:b' logtest::x 2>&1; echo "exit $?"
echo "--- hand-written [remote \"a:b\"] in .git/config, then ls-remote a:b:"
printf '[remote "a:b"]\n\turl = logtest::y\n\tfetch = +refs/heads/*:refs/remotes/a:b/*\n' >> .git/config
export LOGTEST_LOG="$W/colon.log"; : > "$LOGTEST_LOG"
git ls-remote a:b >/dev/null 2>"$W/colon.err"; echo "exit $?"; sed 's/^/  stderr: /' "$W/colon.err" | head -3
grep '^ARGS:' "$LOGTEST_LOG" | sed "s#$W#\$W#g; s/^/  /"
echo "--- git config --get on an anonymous URL's would-be key:"
git config --get 'remote.proton::/my-files/a.b.url'; echo "exit $?"
git config --get-all 'remote.proton::/my-files/a.b.fetch'; echo "exit $?"
