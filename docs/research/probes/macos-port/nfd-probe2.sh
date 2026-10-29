#!/bin/bash
# Follow-up to nfd-probe.sh, in the same work dir: what an incremental fetch does when
# the remote moves one of an NFC/NFD pair, and the typed-name case for directories.
# Source-side ref names go only through stdin or are resolved from object IDs: git on
# macOS precomposes argv, so an NFD name on the command line would hit the NFC ref.
#
# Usage: nfd-probe2.sh <work dir that nfd-probe.sh made>
set -u
cd "$1" || exit 1
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_AUTHOR_NAME=probe GIT_AUTHOR_EMAIL=probe@example.invalid \
  GIT_COMMITTER_NAME=probe GIT_COMMITTER_EMAIL=probe@example.invalid
NFC=$(printf 'caf\303\251'); NFD=$(printf 'cafe\314\201')
DNFC=$(printf 'grp-\303\251')
hex() { printf '%s' "$1" | xxd -p | tr -d '\n'; }
refs() {
  git -C "$1" for-each-ref --format='%(objectname:short) %(refname)' "${2:-refs/}" |
    while IFS=' ' read -r s r; do printf '    %s %s\n' "$s" "$(hex "$r")"; done
}
# The object a source ref points at, found by exact bytes in for-each-ref output.
srcsha() {
  local want; want=$(hex "refs/heads/$1")
  git -C src for-each-ref --format='%(objectname) %(refname)' refs/heads/ |
    while IFS=' ' read -r s r; do [ "$(hex "$r")" = "$want" ] && echo "$s"; done
}
# Commit "label" on top of commit "parent" without touching any ref; prints its ID.
child() { git -C src commit-tree -p "$1" -m "$2" "$(printf '%s' "$2" | git -C src hash-object -w --stdin | {
  read -r blob; printf '100644 blob %s\twhich.txt\n' "$blob" | git -C src mktree; })"; }
setref() { printf 'update refs/heads/%s %s\n' "$1" "$2" | git -C src update-ref --stdin; }
step() { echo; echo "--- $*"; }
run() { "$@"; echo "  exit=$?"; }

c_nfc1=$(srcsha "$NFC"); c_nfd1=$(srcsha "$NFD")
echo "source: NFC -> ${c_nfc1:0:7}, NFD -> ${c_nfd1:0:7}"

step "S3b. directory branch typed as the user would (NFC), when the remote holds it as NFD"
git clone -q all.bundle s3b
run git -C s3b switch -q "$DNFC/b"
echo "  HEAD -> $(hex "$(git -C s3b symbolic-ref HEAD)")"

step "S6. fetch after the remote moves the NFD branch (clone first, so tracking refs start packed)"
git clone -q all.bundle s6
echo "  before:"; refs s6 refs/remotes/
c_nfd2=$(child "$c_nfd1" nfd-2); setref "$NFD" "$c_nfd2"
echo "  remote now: NFC -> $(srcsha "$NFC" | cut -c1-7), NFD -> $(srcsha "$NFD" | cut -c1-7)"
git -C src bundle create ../all.bundle --all >/dev/null 2>&1
run git -C s6 fetch -q origin
echo "  after fetch:"; refs s6 refs/remotes/
echo "  loose files under refs/remotes/origin: $(ls s6/.git/refs/remotes/origin 2>/dev/null | while read -r f; do printf '%s ' "$(hex "$f")"; done)"
run git -C s6 fsck --no-progress

step "S7. a second fetch after the remote moves the NFC branch too"
c_nfc2=$(child "$c_nfc1" nfc-2); setref "$NFC" "$c_nfc2"
echo "  remote now: NFC -> $(srcsha "$NFC" | cut -c1-7), NFD -> $(srcsha "$NFD" | cut -c1-7)"
git -C src bundle create ../all.bundle --all >/dev/null 2>&1
run git -C s6 fetch -q origin
echo "  after fetch:"; refs s6 refs/remotes/
echo "  loose files under refs/remotes/origin: $(ls s6/.git/refs/remotes/origin 2>/dev/null | while read -r f; do printf '%s ' "$(hex "$f")"; done)"

step "S8. the same two moves fetched into a reftable clone"
setref "$NFC" "$c_nfc1"; setref "$NFD" "$c_nfd1"
git -C src bundle create ../all.bundle --all >/dev/null 2>&1
git clone -q --ref-format=reftable all.bundle s8
setref "$NFD" "$c_nfd2"; git -C src bundle create ../all.bundle --all >/dev/null 2>&1; run git -C s8 fetch -q origin
setref "$NFC" "$c_nfc2"; git -C src bundle create ../all.bundle --all >/dev/null 2>&1; run git -C s8 fetch -q origin
echo "  after both fetches:"; refs s8 refs/remotes/

echo; echo "done (remote ends at NFC -> ${c_nfc2:0:7}, NFD -> ${c_nfd2:0:7})"
