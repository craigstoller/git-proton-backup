#!/bin/bash
# NFC/NFD ref-name probe (MD6). Local only: no network, no account, no helper.
# The macOS equivalent of the Linux spec's F13 case probe: refs that differ only
# by Unicode normalisation, carried in a bundle, cloned onto APFS.
#
# Usage: nfd-probe.sh <new work dir>
set -u
W="$1"
mkdir "$W" && cd "$W" || exit 1
# No system or global config: the user's hooksPath must not run on these throwaway commits,
# and the result should depend only on git's own per-repo defaults.
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_AUTHOR_NAME=probe GIT_AUTHOR_EMAIL=probe@example.invalid \
  GIT_COMMITTER_NAME=probe GIT_COMMITTER_EMAIL=probe@example.invalid

# Octal escapes keep this file ASCII. e-acute: NFC U+00E9 = c3 a9; NFD e + U+0301 = 65 cc 81.
NFC=$(printf 'caf\303\251')
NFD=$(printf 'cafe\314\201')
DNFC=$(printf 'grp-\303\251')
DNFD=$(printf 'grp-e\314\201')

hex() { printf '%s' "$1" | xxd -p | tr -d '\n'; }
refs() {
  git -C "$1" for-each-ref --format='%(objectname:short) %(refname)' |
    while IFS=' ' read -r s r; do printf '    %s %s\n' "$s" "$(hex "$r")"; done
}
state() {  # <repo> : HEAD's refname bytes, the checked-out file, and every ref
  printf '  HEAD -> %s, which.txt = %s\n' "$(hex "$(git -C "$1" symbolic-ref HEAD 2>/dev/null)")" \
    "$(cat "$1/which.txt" 2>/dev/null || echo '(none)')"
  refs "$1"
}
step() { echo; echo "--- $*"; }
run() { "$@"; echo "  exit=$?"; }

echo "names: NFC=$(hex "$NFC") NFD=$(hex "$NFD") DNFC=$(hex "$DNFC") DNFD=$(hex "$DNFD")"

step "0. APFS: does an NFD name reach an NFC file?"
mkdir fs && printf one > "fs/$NFC"
if [ -e "fs/$NFD" ]; then echo "  yes: lookup is normalisation-insensitive"; else echo "  no"; fi
printf two > "fs/$NFD"
echo "  entries after writing both: $(ls fs | wc -l | tr -d ' '); stored name: $(hex "$(ls fs)"); content: $(cat "fs/$NFC")"

step "1. source repo (reftable, so the two spellings cannot collide on disk)"
run git init -q --ref-format=reftable -b main src
cd src
printf base > which.txt && git add which.txt && git commit -q -m base
base=$(git rev-parse HEAD)
mk() { git checkout -q --detach "$base"; printf '%s' "$1" > which.txt; git commit -q -am "$1"; git rev-parse HEAD; }
c_nfc=$(mk nfc); c_nfd=$(mk nfd); c_dnfc=$(mk dir-nfc); c_dnfd=$(mk dir-nfd)
git checkout -q main
echo "  commits: base=${base:0:7} nfc=${c_nfc:0:7} nfd=${c_nfd:0:7} dir-nfc=${c_dnfc:0:7} dir-nfd=${c_dnfd:0:7}"
# stdin, not argv: git precomposes argv on macOS, and this must keep the bytes exact.
printf 'create refs/heads/%s %s\n' "$NFC" "$c_nfc" "$NFD" "$c_nfd" "$DNFC/a" "$c_dnfc" "$DNFD/b" "$c_dnfd" |
  git update-ref --stdin
echo "  update-ref exit=$?"
refs .
cd ..

step "2. bundle"
run git -C src bundle create ../all.bundle --all
git bundle list-heads all.bundle | while read -r s r; do printf '    %s %s\n' "${s:0:7}" "$(hex "$r")"; done

step "3. a branch a macOS user creates from an NFD argument (files backend, defaults)"
git init -q -b main userrepo && git -C userrepo commit -q --allow-empty -m x
git -C userrepo config core.precomposeunicode
run git -C userrepo branch "$NFD"
refs userrepo

step "S1. clone (defaults), switch NFC, then switch NFD as typed (argv)"
run git clone -q all.bundle s1
echo "  core.precomposeunicode=$(git -C s1 config core.precomposeunicode)  core.ignorecase=$(git -C s1 config core.ignorecase)"
echo "  refs straight after clone:"; refs s1
run git -C s1 switch -q "$NFC"; state s1
run git -C s1 switch -q "$NFD"; state s1
run git -C s1 fsck --no-progress

step "S2. clone, switch NFC, then switch NFD with precomposeunicode off (raw bytes)"
git clone -q all.bundle s2
run git -C s2 switch -q "$NFC"; state s2
run git -C s2 -c core.precomposeunicode=false switch -q "$NFD"; state s2
echo "  loose files under refs/heads: $(ls s2/.git/refs/heads | while read -r f; do printf '%s ' "$(hex "$f")"; done)"

step "S3. clone, directory prefixes: switch DNFC/a, then DNFD/b with precomposeunicode off"
git clone -q all.bundle s3
run git -C s3 switch -q "$DNFC/a"; state s3
run git -C s3 -c core.precomposeunicode=false switch -q "$DNFD/b"; state s3

step "S4. as S2, but with refs packed before the second switch"
git clone -q all.bundle s4
git -C s4 switch -q "$NFC"; git -C s4 switch -q main; run git -C s4 pack-refs --all
run git -C s4 -c core.precomposeunicode=false switch -q "$NFD"; state s4

step "S5. clone with the reftable ref format, then the S2 sequence"
run git clone -q --ref-format=reftable all.bundle s5
run git -C s5 switch -q "$NFC"; state s5
run git -C s5 -c core.precomposeunicode=false switch -q "$NFD"; state s5
run git -C s5 switch -q "$NFD"; state s5

echo; echo "done"
