#!/bin/bash
# Task 13 review, risk 2. What does the README's exact-bytes delete,
#     printf 'delete refs/remotes/proton-v2/cafe\314\201\n' | git update-ref --stdin
# do in a files-store repository on APFS that holds both spellings of "café"?
# Local repositories only; no remote, no network, no Proton.
#
# Usage: /bin/bash probe.sh <existing empty root dir> <dir holding emit.py and show.py>
set -u
root=$1
here=$2
if [ ! -d "$root" ] || [ -n "$(ls -A "$root")" ]; then
    echo "root must be an existing, empty directory" >&2
    exit 2
fi
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
export GIT_AUTHOR_NAME=probe GIT_AUTHOR_EMAIL=probe@example.invalid
export GIT_COMMITTER_NAME=probe GIT_COMMITTER_EMAIL=probe@example.invalid
export GIT_AUTHOR_DATE='2026-10-28T00:00:00+0000' GIT_COMMITTER_DATE='2026-10-28T00:00:00+0000'
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_COMMON_DIR

emit() { python3 -I "$here/emit.py" "$@"; }
reprlines() {
    python3 -I -c 'import sys
lines = sys.stdin.buffer.read().splitlines()
print("      (none)") if not lines else [print("     ", repr(l)) for l in lines]'
}

echo "== environment"
date -u '+  %Y-%m-%dT%H:%M:%SZ'
echo "  $(git --version); $(/bin/bash --version | head -1)"
echo "  sw_vers: $(sw_vers -productVersion 2>/dev/null)"
mkdir "$root/fscheck"
python3 -I -c 'import os, sys
d = os.fsencode(sys.argv[1])
open(os.path.join(d, b"caf\xc3\xa9"), "w").close()
print("  filesystem: an NFC-named file was created; does the NFD name open it?",
      os.path.exists(os.path.join(d, b"cafe\xcc\x81")))
print("  stored name bytes:", os.listdir(d))
os.remove(os.path.join(d, b"caf\xc3\xa9"))' "$root/fscheck"
rmdir "$root/fscheck"
echo "  the README command's bytes:"
printf 'delete refs/remotes/proton-v2/cafe\314\201\n' | od -c | sed 's/^/    /'

mkrepo() {  # mkrepo <dir>: a files-store repo with commits A, B (child of A) and C
    git init -q "$1" || exit 1
    tree=$(git -C "$1" hash-object -w -t tree /dev/null)
    A=$(git -C "$1" commit-tree "$tree" -m A)
    B=$(git -C "$1" commit-tree "$tree" -p "$A" -m B)
    C=$(git -C "$1" commit-tree "$tree" -m C)
    echo "  ref format: $(git -C "$1" rev-parse --show-ref-format);" \
        "core.precomposeunicode (set by git init): $(git -C "$1" config core.precomposeunicode)"
    echo "  commits: A=${A:0:12} B=${B:0:12} C=${C:0:12}"
}

upd() {  # upd <repo> <emit args...>
    r=$1
    shift
    emit "$@" | git -C "$r" update-ref --stdin
    echo "    update-ref --stdin ($*): exit $?"
}

state() {  # state <repo> <namespace> <label>
    echo "  [$3]"
    python3 -I "$here/show.py" "$1" "$2"
    echo "    git for-each-ref (repo config as git init left it):"
    git -C "$1" for-each-ref --format='%(objectname:short) %(refname)' | reprlines
    echo "    git -c core.precomposeunicode=false for-each-ref:"
    git -C "$1" -c core.precomposeunicode=false for-each-ref --format='%(objectname:short) %(refname)' | reprlines
    echo "    ls -la .git/$2:"
    ls -la "$1/.git/$2" | sed 's/^/      /'
    echo "    ls .git/$2 | od -c:"
    ls "$1/.git/$2" | od -c | sed 's/^/      /'
}

fsck() {  # fsck <repo>
    echo "    git fsck:"
    git -C "$1" fsck 2>&1 | sed 's/^/      /'
    echo "      (fsck exit ${PIPESTATUS[0]})"
}

readme_delete_remotes() {  # the README's command, verbatim, run in <repo>
    echo "  RUN: printf 'delete refs/remotes/proton-v2/cafe\\314\\201\\n' | git update-ref --stdin"
    (cd "$1" && printf 'delete refs/remotes/proton-v2/cafe\314\201\n' | git update-ref --stdin)
    echo "    exit $?"
}

readme_delete_heads() {  # the README's command with refs/heads in place of refs/remotes/proton-v2
    echo "  RUN: printf 'delete refs/heads/cafe\\314\\201\\n' | git update-ref --stdin"
    (cd "$1" && printf 'delete refs/heads/cafe\314\201\n' | git update-ref --stdin)
    echo "    exit $?"
}

echo
echo "== L1: refs/heads, NFC packed (A), NFD loose (C); delete NFD by exact bytes"
r=$root/L1
mkrepo "$r"
upd "$r" create refs/heads nfc "$A"
git -C "$r" pack-refs --all
upd "$r" create refs/heads nfd "$C"
state "$r" refs/heads before
readme_delete_heads "$r"
state "$r" refs/heads after
fsck "$r"

echo
echo "== L2: refs/heads, NFD packed (C), NFC loose (B); delete NFD by exact bytes"
r=$root/L2
mkrepo "$r"
upd "$r" create refs/heads nfd "$C"
git -C "$r" pack-refs --all
upd "$r" create refs/heads nfc "$B"
state "$r" refs/heads before
readme_delete_heads "$r"
state "$r" refs/heads after
fsck "$r"

echo
echo "== L3: refs/heads, NFC packed (A) and loose (B), NFD packed (C); delete NFD by exact bytes"
r=$root/L3
mkrepo "$r"
upd "$r" create refs/heads nfc "$A"
git -C "$r" pack-refs --all
upd "$r" create refs/heads nfd "$C"
git -C "$r" pack-refs --all
upd "$r" update refs/heads nfc "$B" "$A"
state "$r" refs/heads before
readme_delete_heads "$r"
state "$r" refs/heads after
fsck "$r"

echo
echo "== L4: refs/remotes/proton-v2, NFD packed (C), NFC loose (B); the README's command verbatim"
r=$root/L4
mkrepo "$r"
upd "$r" create refs/remotes/proton-v2 nfd "$C"
git -C "$r" pack-refs --all
upd "$r" create refs/remotes/proton-v2 nfc "$B"
state "$r" refs/remotes/proton-v2 before
readme_delete_remotes "$r"
state "$r" refs/remotes/proton-v2 after
fsck "$r"

echo
echo "== L5: refs/remotes/proton-v2, NFD loose (C) only, NFC packed (B); the README's command verbatim"
r=$root/L5
mkrepo "$r"
upd "$r" create refs/remotes/proton-v2 nfc "$B"
git -C "$r" pack-refs --all
upd "$r" create refs/remotes/proton-v2 nfd "$C"
state "$r" refs/remotes/proton-v2 before
readme_delete_remotes "$r"
state "$r" refs/remotes/proton-v2 after
fsck "$r"

echo
echo "== L6: refs/remotes/proton-v2, only NFC, loose (B); the README's command verbatim (NFD named)"
r=$root/L6
mkrepo "$r"
upd "$r" create refs/remotes/proton-v2 nfc "$B"
state "$r" refs/remotes/proton-v2 before
readme_delete_remotes "$r"
state "$r" refs/remotes/proton-v2 after
fsck "$r"

echo
echo "== done"
