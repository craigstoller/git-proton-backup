#!/bin/bash
# Task 13 review, risk 2, second pass. probe.sh found that `git pack-refs` under git init's
# core.precomposeunicode=true re-spells a loose NFD ref as NFC in packed-refs, so its layouts
# with an NFD *packed* entry were never built. A clone writes its refs to packed-refs by their
# raw bytes, so those layouts are real. This pass builds them two ways and runs the README's
# command,
#     printf 'delete refs/remotes/proton-v2/cafe\314\201\n' | git update-ref --stdin
# verbatim. Local repositories only (local-path clone and fetch); no network, no Proton.
#
# Usage: /bin/bash probe2.sh <existing empty root dir> <dir holding emit.py and show.py>
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
upd() {  # upd <repo> <emit args...>
    local target=$1
    shift
    emit "$@" | git -C "$target" update-ref --stdin
    echo "    update-ref --stdin in $(basename "$target") ($*): exit $?"
}
state() {  # state <repo> <namespace> <label>
    echo "  [$3]"
    python3 -I "$here/show.py" "$1" "$2"
    echo "    git -c core.precomposeunicode=false for-each-ref $2:"
    git -C "$1" -c core.precomposeunicode=false for-each-ref --format='%(objectname:short) %(refname)' "$2" | reprlines
    echo "    ls .git/$2 | od -c:"
    if [ -d "$1/.git/$2" ]; then ls "$1/.git/$2" | od -c | sed 's/^/      /'; else echo "      (no such directory)"; fi
}
fsck() {
    echo "    git fsck:"
    git -C "$1" fsck 2>&1 | sed 's/^/      /'
    echo "      (fsck exit ${PIPESTATUS[0]})"
}
readme_delete_remotes() {  # the README's command, verbatim, run in <repo>
    echo "  RUN: printf 'delete refs/remotes/proton-v2/cafe\\314\\201\\n' | git update-ref --stdin"
    (cd "$1" && printf 'delete refs/remotes/proton-v2/cafe\314\201\n' | git update-ref --stdin)
    echo "    exit $?"
}
commits() {  # commits <repo>: A, B (child of A), C, D
    tree=$(git -C "$1" hash-object -w -t tree /dev/null)
    A=$(git -C "$1" commit-tree "$tree" -m A)
    B=$(git -C "$1" commit-tree "$tree" -p "$A" -m B)
    C=$(git -C "$1" commit-tree "$tree" -m C)
    D=$(git -C "$1" commit-tree "$tree" -p "$C" -m D)
    echo "  commits: A=${A:0:12} B=${B:0:12} C=${C:0:12} D=${D:0:12}"
}

echo "== environment: $(git --version); $(sw_vers -productVersion 2>/dev/null)"

echo
echo "== L7: hand-built. NFD packed (C) by pack-refs with precomposition off, then NFC loose (B)"
r=$root/L7
git init -q "$r"
commits "$r"
upd "$r" create refs/remotes/proton-v2 nfd "$C"
git -C "$r" -c core.precomposeunicode=false pack-refs --all
upd "$r" create refs/remotes/proton-v2 nfc "$B"
state "$r" refs/remotes/proton-v2 before
readme_delete_remotes "$r"
state "$r" refs/remotes/proton-v2 after
fsck "$r"

echo
echo "== L8: real clone and fetch, M24's layout. The remote (reftable) holds NFC (A) and NFD (C);"
echo "   a files clone packs both; the remote moves NFC to B; a fetch writes NFC loose."
s=$root/L8
mkdir "$s"
git init -q --bare --ref-format=reftable "$s/remote.git"
commits "$s/remote.git"
upd "$s/remote.git" create refs/heads nfc "$A"
upd "$s/remote.git" create refs/heads nfd "$C"
git -C "$s" clone -q --ref-format=files -o proton-v2 remote.git clone 2>&1 | sed 's/^/    clone: /'
r=$s/clone
echo "  clone: ref format $(git -C "$r" rev-parse --show-ref-format); core.precomposeunicode=$(git -C "$r" config core.precomposeunicode)"
state "$r" refs/remotes/proton-v2 "after clone"
upd "$s/remote.git" update refs/heads nfc "$B" "$A"
git -C "$r" fetch -q proton-v2 2>&1 | sed 's/^/    fetch: /'
echo "    fetch exit ${PIPESTATUS[0]}"
state "$r" refs/remotes/proton-v2 "before the README command (after fetch)"
readme_delete_remotes "$r"
state "$r" refs/remotes/proton-v2 after
fsck "$r"

echo
echo "== L9: real clone and fetch, the R1 / stale-ref case. At clone time the remote holds only NFD (C);"
echo "   then NFC (B) appears on the remote and a fetch writes its tracking ref loose."
s=$root/L9
mkdir "$s"
git init -q --bare --ref-format=reftable "$s/remote.git"
commits "$s/remote.git"
upd "$s/remote.git" create refs/heads nfd "$C"
git -C "$s" clone -q --ref-format=files -o proton-v2 remote.git clone 2>&1 | sed 's/^/    clone: /'
r=$s/clone
state "$r" refs/remotes/proton-v2 "after clone"
upd "$s/remote.git" create refs/heads nfc "$B"
git -C "$r" fetch -q proton-v2 2>&1 | sed 's/^/    fetch: /'
echo "    fetch exit ${PIPESTATUS[0]}"
state "$r" refs/remotes/proton-v2 "before the README command (after fetch)"
readme_delete_remotes "$r"
state "$r" refs/remotes/proton-v2 after
fsck "$r"

echo
echo "== done"
