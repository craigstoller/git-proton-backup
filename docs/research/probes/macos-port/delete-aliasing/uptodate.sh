#!/bin/bash
# Does git rewrite a remote-tracking ref after a push where the ref was already
# up to date (no push command sent)? Case 1: plain names. Case 2: a stale NFD
# tracking ref beside an up-to-date NFC branch, on a Mac `files` clone. Local only.
set -u
W="$1"; rm -rf "$W"; mkdir -p "$W"; cd "$W" || exit 1
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
export GIT_AUTHOR_NAME=p GIT_AUTHOR_EMAIL=p@x GIT_COMMITTER_NAME=p GIT_COMMITTER_EMAIL=p@x
NFC=$(printf 'caf\xc3\xa9'); NFD=$(printf 'cafe\xcc\x81')
hexrefs() { git -C "$1" for-each-ref --format='%(objectname:short) %(refname)' refs/remotes | while read -r o r; do printf '    %s %s\n' "$o" "$(printf '%s' "$r" | xxd -p | tr -d '\n')"; done; }
git init -q --bare remote.git
git init -q src
git -C src commit -q --allow-empty -m A; A=$(git -C src rev-parse HEAD)
git -C src commit -q --allow-empty -m B; B=$(git -C src rev-parse HEAD)
git -C src push -q ../remote.git "$B:refs/heads/main" "$A:refs/heads/$NFC"
git clone -q remote.git clone; cd clone || exit 1
git config core.precomposeunicode false
echo "A=$(git rev-parse --short "$A") B=$(git rev-parse --short "$B")"

echo "== case 1: tracking ref main deliberately stale at A; local main == remote main == B"
git update-ref refs/remotes/origin/main "$A"
git push origin main 2>&1 | sed 's/^/  push: /'
echo "  refs/remotes/origin/main now: $(git rev-parse --short refs/remotes/origin/main)"

echo "== case 2: local branch NFC == remote NFC == A; a stale loose NFD tracking ref at B"
git branch -f "$NFC" "$A"
git pack-refs --all
printf 'create refs/remotes/origin/%s %s\n' "$NFD" "$B" | git update-ref --stdin 2>&1 | sed 's/^/  update-ref: /'
echo "  before:"; hexrefs .
git push origin "refs/heads/$NFC:refs/heads/$NFC" 2>&1 | sed 's/^/  push: /'
echo "  after:"; hexrefs .
echo "hex: NFC=$(printf '%s' "$NFC" | xxd -p) NFD=$(printf '%s' "$NFD" | xxd -p)"
