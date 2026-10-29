#!/bin/bash
# I1 probe: a lone NFD branch, cloned into a Mac `files` repository at git's Mac default
# (core.precomposeunicode=true, which git init sets on macOS). Local repositories only:
# a reftable bare remote reached by path, so no remote helper and no account are involved.
# What it measures: what `git for-each-ref` (the read LocalRefs makes) lists after the clone
# and after each incremental fetch, with the repository's own config and with precomposition
# forced off, beside the loose file names on disk.
set -u
W="$1"
rm -rf "$W"; mkdir -p "$W" || exit 1
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
export GIT_AUTHOR_NAME=probe GIT_AUTHOR_EMAIL=probe@example.invalid
export GIT_COMMITTER_NAME=probe GIT_COMMITTER_EMAIL=probe@example.invalid
NFD=$(printf 'cafe\314\201')
echo "== $(git --version); $(sw_vers -productVersion 2>/dev/null)"

git init -q --bare --ref-format=reftable "$W/remote.git" || exit 1
git init -q "$W/seed" || exit 1
git -C "$W/seed" commit -q --allow-empty -m A || exit 1
A=$(git -C "$W/seed" rev-parse HEAD)
git -C "$W/seed" commit -q --allow-empty -m B || exit 1
B=$(git -C "$W/seed" rev-parse HEAD)
git -C "$W/seed" commit -q --allow-empty -m C || exit 1
C=$(git -C "$W/seed" rev-parse HEAD)
git -C "$W/seed" push -q "$W/remote.git" "$C:refs/heads/main" || exit 1
printf 'create refs/heads/%s %s\n' "$NFD" "$A" | git -C "$W/remote.git" update-ref --stdin || exit 1
echo "commits: A=${A:0:12} B=${B:0:12} C=${C:0:12}"

show() {
  echo "  [$1]"
  echo "    loose files under refs/remotes/origin:"
  if [ -d "$W/clone/.git/refs/remotes/origin" ]; then
    ls "$W/clone/.git/refs/remotes/origin" | od -c | sed 's/^/      /'
  else
    echo "      (none)"
  fi
  echo "    packed-refs entries for origin:"
  grep 'refs/remotes/origin/' "$W/clone/.git/packed-refs" 2>/dev/null | od -c | sed 's/^/      /' | head -20
  echo "    for-each-ref, the repository's own config (core.precomposeunicode=$(git -C "$W/clone" config core.precomposeunicode)):"
  git -C "$W/clone" for-each-ref --format='%(objectname:short) %(refname)' refs/remotes/origin/ | od -c | sed 's/^/      /'
  echo "    for-each-ref with -c core.precomposeunicode=false:"
  git -C "$W/clone" -c core.precomposeunicode=false for-each-ref --format='%(objectname:short) %(refname)' refs/remotes/origin/ | od -c | sed 's/^/      /'
}

git clone -q "$W/remote.git" "$W/clone" 2>&1 | sed 's/^/  clone: /'
echo "  clone ref format: $(git -C "$W/clone" rev-parse --show-ref-format)"
show "after clone"

printf 'update refs/heads/%s %s %s\n' "$NFD" "$B" "$A" | git -C "$W/remote.git" update-ref --stdin || exit 1
git -C "$W/clone" fetch -q origin 2>&1 | sed 's/^/  fetch 1: /'; echo "  fetch 1 exit: ${PIPESTATUS[0]}"
show "after fetch 1 (remote NFD moved A -> B)"

printf 'update refs/heads/%s %s %s\n' "$NFD" "$C" "$B" | git -C "$W/remote.git" update-ref --stdin || exit 1
git -C "$W/clone" fetch -q origin 2>&1 | sed 's/^/  fetch 2: /'; echo "  fetch 2 exit: ${PIPESTATUS[0]}"
show "after fetch 2 (remote NFD moved B -> C)"

echo "  git fsck:"; git -C "$W/clone" fsck 2>&1 | sed 's/^/    /'; echo "  (fsck exit ${PIPESTATUS[0]})"
echo "== done"
