#!/bin/bash
# (1) What do the three local reads return for a linked worktree's git dir, as
#     git would hand it to a helper (GIT_DIR unset, `git -C <gitdir>`)?
# (2) Under /bin/bash 3.2, non-interactive, does `set -m` give a background job its
#     own process group, so `kill -- -PGID` reaches its grandchildren only?
set -u
W="$1"; rm -rf "$W"; mkdir -p "$W"
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
export GIT_AUTHOR_NAME=p GIT_AUTHOR_EMAIL=p@x GIT_COMMITTER_NAME=p GIT_COMMITTER_EMAIL=p@x
git init -q "$W/main" && git -C "$W/main" commit -q --allow-empty -m A
git -C "$W/main" remote add origin /nonexistent && git -C "$W/main" update-ref refs/remotes/origin/x HEAD
git -C "$W/main" worktree add -q "$W/wt" -b wtbranch
WTGIT=$(git -C "$W/wt" rev-parse --absolute-git-dir)
echo "worktree git dir: ${WTGIT#$W/}"
unset GIT_DIR
for d in "$W/main/.git" "$WTGIT"; do
  echo "== -C ${d#$W/}"
  echo "  format: $(git -C "$d" rev-parse --show-ref-format 2>&1) (exit $?)"
  echo "  url:    $(git -C "$d" config --get remote.origin.url 2>&1) (exit $?)"
  echo "  refs:   $(git -C "$d" for-each-ref --format='%(refname)' 2>&1 | tr '\n' ' ')"
done

echo "== bash $BASH_VERSION: set -m and process groups"
set -m
( sleep 300 & echo $! > "$W/grandchild.pid"; sleep 300 ) &
job=$!
sleep 1
own=$(ps -o pgid= -p $$ | tr -d ' '); jobpg=$(ps -o pgid= -p "$job" | tr -d ' '); gc=$(cat "$W/grandchild.pid"); gcpg=$(ps -o pgid= -p "$gc" | tr -d ' ')
echo "  wrapper pgid=$own  job pid=$job pgid=$jobpg  grandchild pgid=$gcpg"
kill -TERM -- "-$jobpg"; sleep 1
for p in "$job" "$gc"; do kill -0 "$p" 2>/dev/null && echo "  pid $p still alive" || echo "  pid $p gone"; done
echo "  wrapper still running: yes (this line printed)"
