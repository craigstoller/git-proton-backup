#!/bin/bash
# Task 13 review, risk 2, candidate fix. Does packing every ref first, with precomposition off,
#     git -c core.precomposeunicode=false pack-refs --all
# make the README's exact-bytes delete touch only the named ref? Same layouts as L8 and L9
# (real local clone and fetch), plus L1's (NFD loose, NFC packed). Local only.
#
# Usage: /bin/bash probe3.sh <existing empty root dir> <dir holding emit.py and show.py>
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
upd() {
    local target=$1
    shift
    emit "$@" | git -C "$target" update-ref --stdin
    echo "    update-ref --stdin in $(basename "$target") ($*): exit $?"
}
state() {
    echo "  [$3]"
    python3 -I "$here/show.py" "$1" "$2"
}
commits() {
    tree=$(git -C "$1" hash-object -w -t tree /dev/null)
    A=$(git -C "$1" commit-tree "$tree" -m A)
    B=$(git -C "$1" commit-tree "$tree" -p "$A" -m B)
    C=$(git -C "$1" commit-tree "$tree" -m C)
    echo "  commits: A=${A:0:12} B=${B:0:12} C=${C:0:12}"
}
fix_then_delete() {  # fix_then_delete <repo> <namespace>
    echo "  RUN: git -c core.precomposeunicode=false pack-refs --all"
    git -C "$1" -c core.precomposeunicode=false pack-refs --all
    echo "    exit $?"
    state "$1" "$2" "after pack-refs"
    echo "  RUN: printf 'delete $2/cafe\\314\\201\\n' | git update-ref --stdin"
    (cd "$1" && printf "delete $2/cafe\\314\\201\\n" | git update-ref --stdin)
    echo "    exit $?"
    state "$1" "$2" "after delete"
}

echo "== L10: L8's layout (clone packs NFC=A and NFD=C; fetch writes NFC=B loose), packed first"
s=$root/L10
mkdir "$s"
git init -q --bare --ref-format=reftable "$s/remote.git"
commits "$s/remote.git"
upd "$s/remote.git" create refs/heads nfc "$A"
upd "$s/remote.git" create refs/heads nfd "$C"
git -C "$s" clone -q --ref-format=files -o proton-v2 remote.git clone 2>/dev/null
upd "$s/remote.git" update refs/heads nfc "$B" "$A"
git -C "$s/clone" fetch -q proton-v2
echo "    fetch exit $?"
state "$s/clone" refs/remotes/proton-v2 "before"
fix_then_delete "$s/clone" refs/remotes/proton-v2

echo
echo "== L11: L9's layout (clone packs NFD=C; fetch writes NFC=B loose), packed first"
s=$root/L11
mkdir "$s"
git init -q --bare --ref-format=reftable "$s/remote.git"
commits "$s/remote.git"
upd "$s/remote.git" create refs/heads nfd "$C"
git -C "$s" clone -q --ref-format=files -o proton-v2 remote.git clone 2>/dev/null
upd "$s/remote.git" create refs/heads nfc "$B"
git -C "$s/clone" fetch -q proton-v2
echo "    fetch exit $?"
state "$s/clone" refs/remotes/proton-v2 "before"
fix_then_delete "$s/clone" refs/remotes/proton-v2

echo
echo "== L12: L1's layout (NFC packed A, NFD loose C), packed first"
r=$root/L12
git init -q "$r"
commits "$r"
upd "$r" create refs/heads nfc "$A"
git -C "$r" pack-refs --all
upd "$r" create refs/heads nfd "$C"
state "$r" refs/heads "before"
fix_then_delete "$r" refs/heads

echo
echo "== done"
