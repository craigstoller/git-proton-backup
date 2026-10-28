#!/bin/bash
# What does git send a remote helper for fetch (objects local / absent), clone and ls-remote?
set -u
H="$(cd "$(dirname "$0")" && pwd)"; W="$1"
mkdir "$W" && cd "$W" || exit 1
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_AUTHOR_NAME=p GIT_AUTHOR_EMAIL=p@example.invalid \
  GIT_COMMITTER_NAME=p GIT_COMMITTER_EMAIL=p@example.invalid PATH="$H:$PATH"
git init -q -b main src && git -C src commit -q --allow-empty -m one
present=$(git -C src rev-parse HEAD)
git clone -q src work
git -C work remote add lt logtest::x
absent=1234567890123456789012345678901234567890

show() { echo "--- $1 (exit $2)"; sed 's/^/    /' "$LOGTEST_LOG"; }

export LOGTEST_LOG="$W/log-fetch-present" LOGTEST_SHA="$present"; : > "$LOGTEST_LOG"
git -C work fetch -q lt 2>/dev/null; rc=$?; show "git fetch lt, advertised tip already local" $rc
echo "    tracking ref written: $(git -C work rev-parse --verify -q refs/remotes/lt/main || echo none)"

export LOGTEST_LOG="$W/log-fetch-absent" LOGTEST_SHA="$absent"; : > "$LOGTEST_LOG"
git -C work fetch -q lt 2>/dev/null; rc=$?; show "git fetch lt, advertised tip absent" $rc

export LOGTEST_LOG="$W/log-lsremote-inrepo" LOGTEST_SHA="$present"; : > "$LOGTEST_LOG"
git -C work ls-remote lt >/dev/null 2>&1; rc=$?; show "git ls-remote lt, inside the repository" $rc

export LOGTEST_LOG="$W/log-lsremote-outside" LOGTEST_SHA="$present"; : > "$LOGTEST_LOG"
(cd "$W" && git ls-remote logtest::x >/dev/null 2>&1); rc=$?; show "git ls-remote logtest::x, outside any repository" $rc

export LOGTEST_LOG="$W/log-clone" LOGTEST_SHA="$absent"; : > "$LOGTEST_LOG"
git clone -q logtest::x newclone >/dev/null 2>&1; rc=$?; show "git clone logtest::x" $rc
