#!/bin/bash
# Offline test for gpb-v2-push.sh. Run it as: /bin/bash scripts/macos/test-gpb-v2-push.sh
#
# Hermetic by construction:
#   * the wrapper runs under `env -i` with stand-in `git`, `osascript` and `sleep` first on its
#     GPB_PATH, so no real git, remote, Proton CLI, notification or ~/Library/Logs is reached;
#   * every case gets a fresh temp dir for GPB_MARKER_DIR, TMPDIR and HOME;
#   * the stand-ins write the PIDs they start to files, and every "is it dead" check looks only
#     at those PIDs, never at a machine-wide process listing;
#   * a trap removes the temp dir and kills any recorded process still alive, even on failure.
#
# GPB_PATH uses the system directories only (not the caller's PATH) so the wrapper is exercised
# with the BSD tools launchd would give it, not the GNU ones a login shell may put first.
#
# Prints one PASS or FAIL line per case, and exits 0 only if all seven pass.

set -u

HERE=$(cd "$(dirname "$0")" && pwd -P) || exit 2
WRAPPER=$HERE/gpb-v2-push.sh
SYSPATH=/usr/bin:/bin:/usr/sbin:/sbin

BASE=${TMPDIR:-/tmp}
BASE=${BASE%/}
ROOT=$(mktemp -d "$BASE/gpb-v2-push-test.XXXXXX") || exit 2
STUB=$ROOT/stubs
PIDS=$ROOT/pids
mkdir -p "$STUB" || exit 2
: >"$PIDS"

PASSED=0
FAILED=0
CASE=
T=
FAILS=
DETAIL=
WRC=

# ---------------------------------------------------------------------------------------------
# Cleanup: kill whatever the stand-ins recorded and left running, then remove the temp dir.
# A recorded PID is only killed if its command line still looks like ours.
# ---------------------------------------------------------------------------------------------
cleanup() {
    local p cmd
    trap '' EXIT INT TERM HUP
    if [ -f "$PIDS" ]; then
        while read -r p; do
            [ -n "$p" ] || continue
            kill -0 "$p" 2>/dev/null || continue
            cmd=$(ps -o command= -p "$p" 2>/dev/null)
            case $cmd in
                *sleep*|*"$ROOT"*) kill -KILL "$p" 2>/dev/null ;;
            esac
        done <"$PIDS"
    fi
    case $ROOT in
        */gpb-v2-push-test.*) rm -rf "$ROOT" ;;
    esac
}
trap cleanup EXIT
trap 'exit 130' INT TERM HUP

# ---------------------------------------------------------------------------------------------
# Stand-ins
# ---------------------------------------------------------------------------------------------
cat >"$STUB/git" <<'EOF'
#!/bin/bash
# Stand-in git: records the call, its environment and its PIDs. Touches no repository.
last=
for a in "$@"; do last=$a; done
case $last in
    --all) mode=all ;;
    --tags) mode=tags ;;
    *) mode=other ;;
esac
echo "$$" >>"$GPB_TEST_PIDS"
echo "$*" >>"$GPB_TEST_GITLOG"
echo "PATH=$PATH" >>"$GPB_TEST_ENVLOG"
echo "PROTON_DRIVE_CACHE_DIR=${PROTON_DRIVE_CACHE_DIR-unset}" >>"$GPB_TEST_ENVLOG"
echo "$$" >"$GPB_TEST_DIR/pid.git.$mode"
if [ -n "${STUB_AWAIT_TIMER:-}" ]; then
    # Do not return before this push's watchdog has started its timer (the stand-in sleep logs
    # it), so a push that returns at once cannot beat the watchdog into existence.
    calls=$(wc -l <"$GPB_TEST_GITLOG" | tr -d ' ')
    i=0
    while [ "$i" -lt 50 ]; do
        n=$(awk '$3 == 1800' "$GPB_TEST_SLEEPS" | wc -l | tr -d ' ')
        [ "$n" -ge "$calls" ] && break
        /bin/sleep 0.1
        i=$((i + 1))
    done
fi
if [ "${STUB_HANG:-}" = "$mode" ]; then
    # A grandchild that ignores TERM (ignored signals survive exec), in the push's process group.
    ( trap '' TERM; exec /bin/sleep 600 ) &
    gc=$!
    echo "$gc" >>"$GPB_TEST_PIDS"
    echo "$gc" >"$GPB_TEST_DIR/pid.grandchild.$mode"
    /bin/sleep 600
    exit 0
fi
case $mode in
    all) exit "${STUB_RC_ALL:-0}" ;;
    tags) exit "${STUB_RC_TAGS:-0}" ;;
esac
exit 0
EOF

cat >"$STUB/sleep" <<'EOF'
#!/bin/bash
# Stand-in sleep: records its own PID, its parent's PID and its arguments, then really sleeps.
echo "$$ $PPID $*" >>"$GPB_TEST_SLEEPS"
echo "$$" >>"$GPB_TEST_PIDS"
exec /bin/sleep "$@"
EOF

cat >"$STUB/osascript" <<'EOF'
#!/bin/bash
# Stand-in osascript: records its argv, one argument per line, then the argument count.
f=${GPB_TEST_OSA:?}
{
    n=0
    for a in "$@"; do
        n=$((n + 1))
        printf 'arg%d=%s\n' "$n" "$a"
    done
    echo "argc=$n"
} >"$f.tmp" && mv "$f.tmp" "$f"
EOF

cat >"$STUB/notify-touch" <<'EOF'
#!/bin/bash
# Stand-in GPB_NOTIFY command: records its argv and its own process group.
f=${GPB_TEST_NOTIFY:?}
{
    echo "argc=$#"
    n=0
    for a in "$@"; do
        n=$((n + 1))
        printf 'arg%d=%s\n' "$n" "$a"
    done
    echo "pgid=$(ps -o pgid= -p $$ | tr -d ' ')"
} >"$f.tmp" && mv "$f.tmp" "$f"
EOF

cat >"$STUB/notify-hang" <<'EOF'
#!/bin/bash
# Stand-in GPB_NOTIFY command that never finishes.
echo "$$" >>"$GPB_TEST_PIDS"
exec /bin/sleep 600
EOF

chmod +x "$STUB/git" "$STUB/sleep" "$STUB/osascript" "$STUB/notify-touch" "$STUB/notify-hang"

# ---------------------------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------------------------
new_case() {
    CASE=$1
    T=$ROOT/$1
    FAILS=
    DETAIL=
    mkdir -p "$T/home" "$T/tmp" "$T/markers" "$T/repos"
    : >"$T/gitlog"
    : >"$T/envlog"
    : >"$T/sleeps"
}

bad() {
    FAILS="$FAILS
    - $*"
}

end_case() {
    local m
    if [ -n "${GPB_TEST_VERBOSE:-}" ]; then
        # Show what the wrapper logged and which markers it left, for diagnosing a case.
        [ -f "$T/out" ] && sed 's/^/    log: /' "$T/out"
        for m in "$T"/markers/*.failed; do
            [ -f "$m" ] && sed 's/^/    marker: /' "$m"
        done
    fi
    if [ -z "$FAILS" ]; then
        echo "PASS: $CASE${DETAIL:+ ($DETAIL)}"
        PASSED=$((PASSED + 1))
    else
        echo "FAIL: $CASE$FAILS"
        FAILED=$((FAILED + 1))
    fi
}

# make_repo <name>: an empty directory standing in for a repository; prints its logical path.
make_repo() {
    mkdir -p "$T/repos/$1" && echo "$T/repos/$1"
}

# phys <dir>: the physical (symlink-free) path of a directory.
phys() {
    (cd "$1" && pwd -P)
}

# marker_for <physical repo> <remote>: where the wrapper must put its failure marker.
marker_for() {
    local sum
    sum=$(printf '%s\n%s' "$1" "$2" | cksum | awk '{print $1}')
    echo "$T/markers/gpb-v2-push.$sum.failed"
}

# run_wrapper <repo> <remote> [VAR=value ...]
# Runs the wrapper under /bin/bash in a clean environment, with the caller's redirections, and
# sets WRC to its exit status. A wrapper still running after $RUN_LIMIT seconds (default 60) is
# killed and WRC is "timeout".
run_wrapper() {
    local repo=$1 remote=$2 wpid end
    shift 2
    /usr/bin/env -i \
        HOME="$T/home" PATH="$SYSPATH" TMPDIR="$T/tmp" \
        GPB_PATH="$STUB:$SYSPATH" GPB_MARKER_DIR="$T/markers" \
        GPB_TEST_DIR="$T" GPB_TEST_PIDS="$PIDS" GPB_TEST_GITLOG="$T/gitlog" \
        GPB_TEST_ENVLOG="$T/envlog" GPB_TEST_SLEEPS="$T/sleeps" \
        GPB_TEST_OSA="$T/osa" GPB_TEST_NOTIFY="$T/notify" \
        "$@" \
        /bin/bash "$WRAPPER" "$repo" "$remote" </dev/null &
    wpid=$!
    end=$((SECONDS + ${RUN_LIMIT:-60}))
    while kill -0 "$wpid" 2>/dev/null; do
        if [ "$SECONDS" -ge "$end" ]; then
            kill -KILL "$wpid" 2>/dev/null
            wait "$wpid" 2>/dev/null
            WRC=timeout
            return 0
        fi
        /bin/sleep 0.1
    done
    wait "$wpid"
    WRC=$?
}

# await <seconds> <command...>: poll until the command succeeds; fail after the time is up.
await() {
    local limit=$1 end
    shift
    end=$((SECONDS + limit))
    while ! "$@"; do
        [ "$SECONDS" -lt "$end" ] || return 1
        /bin/sleep 0.1
    done
}
exists() { [ -e "$1" ]; }
gone() { ! kill -0 "$1" 2>/dev/null; }

# markers_in <dir>: the failure markers in a directory, one per line.
markers_in() {
    ls -A "$1" 2>/dev/null | grep -F '.failed' || true
}

expect_calls() {   # expect_calls <physical repo> <remote> : both pushes, in order
    local want
    want="-C $1 push $2 --all
-C $1 push $2 --tags"
    [ "$(cat "$T/gitlog")" = "$want" ] || bad "git calls were: $(tr '\n' '|' <"$T/gitlog"), want --all then --tags"
}

expect_tmp_clean() {
    [ -z "$(ls -A "$T/tmp")" ] || bad "the wrapper left a temp dir behind: $(ls -A "$T/tmp")"
}

# ---------------------------------------------------------------------------------------------
# Cases
# ---------------------------------------------------------------------------------------------
LOCK_SENTENCE="the push was killed; if the next push reports that this repository is locked by this Mac, that lock is the killed push's: remove it as that message says (README, scheduled use)"

case_success() {
    local repo rp
    new_case success
    repo=$(make_repo r); rp=$(phys "$repo")
    run_wrapper "$repo" proton-v2 GPB_NOTIFY="$STUB/notify-touch" PROTON_DRIVE_CACHE_DIR="$T/cache" \
        >"$T/out" 2>&1
    [ "$WRC" = 0 ] || bad "exit $WRC, want 0"
    expect_calls "$rp" proton-v2
    [ -z "$(markers_in "$T/markers")" ] || bad "a marker exists after a successful run"
    exists "$T/notify" && bad "a notification was posted after a successful run"
    exists "$T/osa" && bad "osascript was called after a successful run"
    grep -Fxq "PATH=$STUB:$SYSPATH" "$T/envlog" || bad "git did not run with PATH=GPB_PATH"
    grep -Fxq "PROTON_DRIVE_CACHE_DIR=$T/cache" "$T/envlog" || bad "git did not inherit PROTON_DRIVE_CACHE_DIR"
    expect_tmp_clean
    end_case
}

case_failure() {
    local repo rp marker mine
    new_case failure
    repo=$(make_repo r); rp=$(phys "$repo")
    run_wrapper "$repo" proton-v2 GPB_NOTIFY="$STUB/notify-touch" STUB_RC_ALL=1 >"$T/out" 2>&1
    [ "$WRC" = 1 ] || bad "exit $WRC, want 1"
    expect_calls "$rp" proton-v2   # --tags still ran after --all failed
    marker=$(marker_for "$rp" proton-v2)
    if [ -f "$marker" ]; then
        grep -Fq -- "$rp" "$marker" || bad "marker does not name the repository"
        grep -Fq -- "proton-v2" "$marker" || bad "marker does not name the remote"
        grep -Fq -- "--all" "$marker" || bad "marker does not name the --all push"
        grep -Eq -- '--all.*exit 1|exit 1.*--all' "$marker" || bad "marker does not give --all's exit code 1"
        grep -Fq -- "--tags" "$marker" && bad "marker names --tags, which succeeded"
        grep -Fq -- "the push was killed" "$marker" && bad "marker carries the lock sentence for a plain failure"
    else
        bad "no marker at $(basename "$marker"); markers: $(markers_in "$T/markers")"
    fi
    [ "$(markers_in "$T/markers" | wc -l | tr -d ' ')" = 1 ] || bad "want exactly one marker"
    if await 3 exists "$T/notify"; then
        grep -Fxq 'argc=1' "$T/notify" || bad "notify command did not get exactly one argument"
        grep -Fq -- "$rp" "$T/notify" || bad "notification message does not name the repository"
        # The notification must not share the wrapper's process group: launchd kills the job's
        # leftover group members when the job exits, which would cancel a still-starting osascript.
        mine=$(ps -o pgid= -p $$ | tr -d ' ')
        if grep -Fxq "pgid=$mine" "$T/notify"; then
            bad "the notification process is in the wrapper's process group"
        fi
    else
        bad "no notification after a failed run"
    fi
    expect_tmp_clean
    end_case
}

case_two_repositories() {
    local a b c ap bp cp ma mb mc
    new_case two-repositories
    a=$(make_repo a); ap=$(phys "$a")
    b=$(make_repo b); bp=$(phys "$b")
    c=$(make_repo c); cp=$(phys "$c")
    ma=$(marker_for "$ap" proton-v2); mb=$(marker_for "$bp" proton-v2); mc=$(marker_for "$cp" proton-v2)
    [ "$ma" != "$mb" ] || bad "setup: two repositories share a marker name"

    run_wrapper "$a" proton-v2 GPB_NOTIFY="$STUB/notify-touch" STUB_RC_ALL=1 >"$T/out" 2>&1
    [ "$WRC" = 1 ] || bad "A: exit $WRC, want 1"
    [ -f "$ma" ] || bad "A failed but left no marker"

    run_wrapper "$b" proton-v2 GPB_NOTIFY="$STUB/notify-touch" >>"$T/out" 2>&1
    [ "$WRC" = 0 ] || bad "B: exit $WRC, want 0"
    [ -f "$ma" ] || bad "A's marker did not survive B's successful run"
    [ -e "$mb" ] && bad "B succeeded but has a marker"

    # A "succeeds" removes its own marker, and only its own.
    run_wrapper "$c" proton-v2 GPB_NOTIFY="$STUB/notify-touch" STUB_RC_TAGS=1 >>"$T/out" 2>&1
    [ "$WRC" = 1 ] || bad "C: exit $WRC, want 1"
    [ -f "$mc" ] || bad "C failed on --tags but left no marker"
    run_wrapper "$a" proton-v2 GPB_NOTIFY="$STUB/notify-touch" >>"$T/out" 2>&1
    [ "$WRC" = 0 ] || bad "A again: exit $WRC, want 0"
    [ -e "$ma" ] && bad "A succeeded but its own marker is still there"
    [ -f "$mc" ] || bad "A's success removed C's marker"
    expect_tmp_clean
    end_case
}

case_hang() {
    local repo rp marker canary gitpid gcpid start elapsed
    new_case hang
    repo=$(make_repo r); rp=$(phys "$repo")

    # Setup check: the stand-in's grandchild construct really survives TERM, so that a later
    # "dead" can only be KILL's doing.
    ( trap '' TERM; exec /bin/sleep 600 ) </dev/null >/dev/null 2>&1 &
    canary=$!
    echo "$canary" >>"$PIDS"
    /bin/sleep 0.5
    kill -TERM "$canary" 2>/dev/null
    /bin/sleep 1
    kill -0 "$canary" 2>/dev/null || bad "setup: a TERM-ignoring process died on TERM, so this case cannot prove KILL"
    kill -KILL "$canary" 2>/dev/null
    wait "$canary" 2>/dev/null   # consume bash's "Killed: 9" notice here, not in a later log

    start=$SECONDS
    RUN_LIMIT=20 run_wrapper "$repo" proton-v2 GPB_NOTIFY="$STUB/notify-touch" \
        GPB_PUSH_TIMEOUT=2 STUB_HANG=all >"$T/out" 2>&1
    elapsed=$((SECONDS - start))
    DETAIL="exit $WRC after ${elapsed}s"
    [ "$WRC" = 124 ] || bad "exit $WRC after ${elapsed}s, want 124 within 20s"
    [ "$elapsed" -le 20 ] || bad "took ${elapsed}s, want at most 20s"

    marker=$(marker_for "$rp" proton-v2)
    if [ -f "$marker" ]; then
        grep -Fq -- "$LOCK_SENTENCE" "$marker" || bad "marker lacks the lock sentence"
        grep -Eq -- '--all.*exit 124|exit 124.*--all' "$marker" || bad "marker does not give --all's exit code 124"
    else
        bad "no marker after a timeout"
    fi

    gitpid=$(cat "$T/pid.git.all" 2>/dev/null)
    gcpid=$(cat "$T/pid.grandchild.all" 2>/dev/null)
    if [ -z "$gitpid" ] || [ -z "$gcpid" ]; then
        bad "the stand-in did not record its PIDs"
    else
        await 3 gone "$gitpid" || bad "the push's git process $gitpid is still alive"
        await 3 gone "$gcpid" || bad "the TERM-ignoring grandchild $gcpid is still alive: KILL did not reach it"
    fi
    # --tags still ran after the --all push was killed.
    grep -Fq -- "push proton-v2 --tags" "$T/gitlog" || bad "--tags did not run after the timeout"
    expect_tmp_clean
    end_case
}

case_notify_hangs() {
    local repo rp start elapsed
    new_case notify-hangs
    repo=$(make_repo r); rp=$(phys "$repo")
    start=$SECONDS
    # Through a pipe, as a caller would capture the log: a notification that held the pipe open
    # would keep `cat` running for 600 s.
    (
        run_wrapper "$repo" proton-v2 GPB_NOTIFY="$STUB/notify-hang" STUB_RC_ALL=1 2>&1 | cat >"$T/out"
        : >"$T/done"
    ) &
    if await 5 exists "$T/done"; then
        elapsed=$((SECONDS - start))
        DETAIL="exited and closed the pipe in ${elapsed}s"
    else
        bad "the wrapper and its pipe were still open after 5s: the hung notification was waited on or held the pipe"
    fi
    # Release the pipe if the wrapper held it, so the subshell above can finish.
    if [ -f "$PIDS" ] && ! exists "$T/done"; then
        while read -r p; do
            kill -KILL "$p" 2>/dev/null
        done <"$PIDS"
    fi
    wait
    [ -f "$(marker_for "$rp" proton-v2)" ] || bad "the failed run left no marker"
    end_case
}

case_default_notification() {
    local repo rp
    new_case default-notification
    # A repository directory whose name needs AppleScript escaping: a double quote and a backslash.
    repo=$(make_repo 'q"b\s'); rp=$(phys "$repo")
    run_wrapper "$repo" proton-v2 STUB_RC_ALL=1 >"$T/out" 2>&1   # GPB_NOTIFY is unset
    [ "$WRC" = 1 ] || bad "exit $WRC, want 1"
    if await 3 exists "$T/osa"; then
        grep -Fxq 'argc=2' "$T/osa" || bad "osascript did not get exactly two arguments: $(tr '\n' '|' <"$T/osa")"
        grep -Fxq 'arg1=-e' "$T/osa" || bad "osascript's first argument is not -e"
        grep -Fq -- 'arg2=display notification "' "$T/osa" || bad "the script is not a display notification"
        grep -Fq -- 'with title "git-proton-backup"' "$T/osa" || bad "the script has no git-proton-backup title"
        grep -Fq -- 'repos/q\"b\\s' "$T/osa" || bad "the script does not name the repository, escaped for AppleScript"
    else
        bad "osascript was not called"
    fi
    end_case
}

case_watchdog_reaped() {
    local repo start elapsed n pid ppid
    new_case watchdog-reaped
    repo=$(make_repo r)
    start=$SECONDS
    RUN_LIMIT=5 run_wrapper "$repo" proton-v2 GPB_NOTIFY="$STUB/notify-touch" STUB_AWAIT_TIMER=1 \
        >"$T/out" 2>&1
    elapsed=$((SECONDS - start))
    DETAIL="exit $WRC after ${elapsed}s"
    [ "$WRC" = 0 ] || bad "exit $WRC within 5s, want 0 (a lingering watchdog would hold the wrapper)"
    [ "$elapsed" -le 5 ] || bad "took ${elapsed}s, want at most 5s"
    # One watchdog per push, each a `sleep 1800` started through GPB_PATH by the watchdog subshell.
    n=$(awk '$3 == 1800' "$T/sleeps" | wc -l | tr -d ' ')
    [ "$n" = 2 ] || bad "saw $n watchdog timers of 1800s, want 2 (one per push)"
    awk '$3 == 1800 {print $1, $2}' "$T/sleeps" | while read -r pid ppid; do
        await 3 gone "$pid" || echo "timer $pid still alive" >>"$T/zombies"
        await 3 gone "$ppid" || echo "watchdog $ppid still alive" >>"$T/zombies"
    done
    [ ! -s "$T/zombies" ] || bad "$(tr '\n' ';' <"$T/zombies")"
    expect_tmp_clean
    end_case
}

if [ ! -f "$WRAPPER" ]; then
    echo "note: $WRAPPER does not exist" >&2
fi

case_success
case_failure
case_two_repositories
case_hang
case_notify_hangs
case_default_notification
case_watchdog_reaped

echo "$PASSED passed, $FAILED failed"
[ "$FAILED" -eq 0 ] && [ "$PASSED" -eq 7 ]
