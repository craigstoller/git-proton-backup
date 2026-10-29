#!/bin/bash
# Re-measure GIT_DIR and argv as git passes them to a remote helper, for
# ls-remote in three places, plus a fetch by anonymous URL. Local only.
set -u
H="$1"; W="$2"; rm -rf "$W"; mkdir -p "$W"
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null PATH="$H:$PATH"
export LOGTEST_SHA=0123456789012345678901234567890123456789
git init -q "$W/repo"; mkdir -p "$W/repo/sub/deeper"
mkdir -p "$W/plain/a"
case_run() { # label dir args...
  local label="$1" dir="$2"; shift 2
  export LOGTEST_LOG="$W/$label.log"; : > "$LOGTEST_LOG"
  (cd "$dir" && git "$@" >/dev/null 2>&1)
  printf '%-34s %s\n' "$label:" "$(grep '^ARGS:' "$LOGTEST_LOG")"
}
case_run ls-remote-repo-top        "$W/repo"              ls-remote logtest::x
case_run ls-remote-repo-subfolder  "$W/repo/sub/deeper"   ls-remote logtest::x
case_run ls-remote-plain-folder    "$W/plain/a"           ls-remote logtest::x
git -C "$W/repo" remote add lt logtest::x
case_run fetch-named-remote        "$W/repo"              fetch lt
case_run fetch-anonymous-url       "$W/repo"              fetch logtest::x
echo "is a repository above the plain folder? $(cd "$W/plain/a" && git rev-parse --git-dir 2>&1 | head -1)"
