#!/bin/bash
# Local probe, no account: what a delete push of one NFC/NFD pair member from a
# Mac `files` clone does to the surviving member's tracking ref.
# Case A: both tracking refs packed (fresh clone). Case B: the surviving
# member's tracking ref loose (after a fetch moved it).
set -u
W="$1"; rm -rf "$W"; mkdir -p "$W"; cd "$W" || exit 1
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
export GIT_AUTHOR_NAME=p GIT_AUTHOR_EMAIL=p@x GIT_COMMITTER_NAME=p GIT_COMMITTER_EMAIL=p@x
NFC=$(printf 'caf\xc3\xa9'); NFD=$(printf 'cafe\xcc\x81')
hexrefs() { git -C "$1" for-each-ref --format='%(objectname:short) %(refname)' | while read -r o r; do printf '%s %s\n' "$o" "$(printf '%s' "$r" | xxd -p | tr -d '\n')"; done; }

git init -q --ref-format=reftable --bare remote.git
git init -q --ref-format=reftable src
git -C src commit -q --allow-empty -m A; A=$(git -C src rev-parse HEAD)
git -C src commit -q --allow-empty -m B; B=$(git -C src rev-parse HEAD)
git -C src commit -q --allow-empty -m C; C=$(git -C src rev-parse HEAD)
git -C src push -q ../remote.git "$C:refs/heads/main"
printf 'create refs/heads/%s %s\ncreate refs/heads/%s %s\n' "$NFC" "$A" "$NFD" "$B" | git -C remote.git update-ref --stdin
echo "remote refs (hex):"; hexrefs remote.git

for case in A B; do
  rm -rf "clone$case"
  git clone -q remote.git "clone$case" 2>&1 | sed 's/^/  clone: /'
  git -C "clone$case" config core.precomposeunicode false
  echo "== case $case: ref format $(git -C clone$case rev-parse --show-ref-format)"
  if [ "$case" = B ]; then
    # Move the NFC member on the remote, then fetch, so its tracking ref is written loose.
    printf 'update refs/heads/%s %s\n' "$NFC" "$C" | git -C remote.git update-ref --stdin
    git -C "clone$case" fetch -q origin 2>&1 | sed 's/^/  fetch: /'
  fi
  echo "  loose files under refs/remotes/origin:"; (cd "clone$case/.git/refs/remotes/origin" 2>/dev/null && ls | while read -r f; do printf '    %s\n' "$(printf '%s' "$f" | xxd -p)"; done)
  echo "  local refs before (hex):"; hexrefs "clone$case" | sed 's/^/    /'
  git -C "clone$case" push origin ":refs/heads/$NFD" 2>&1 | sed 's/^/  push: /'; echo "  push exit: ${PIPESTATUS[0]}"
  echo "  local refs after (hex):"; hexrefs "clone$case" | sed 's/^/    /'
  echo "  remote refs after (hex):"; hexrefs remote.git | sed 's/^/    /'
  git -C "clone$case" fsck --no-progress 2>&1 | sed 's/^/  fsck: /'
  # restore the remote's NFD member for the next case
  printf 'create refs/heads/%s %s\n' "$NFD" "$B" | git -C remote.git update-ref --stdin 2>/dev/null
done
echo "hex key: NFC=$(printf '%s' "$NFC" | xxd -p) NFD=$(printf '%s' "$NFD" | xxd -p)"
