#!/bin/bash
# gpb-v2-push.sh <repository> <remote>
#
# Backs one repository up through the git-remote-proton v2 helper, for a per-repository
# LaunchAgent (see gpb-v2-push.plist.example). It runs two pushes:
#
#   git -C <repository> push <remote> --all
#   git -C <repository> push <remote> --tags
#
# The second runs even if the first fails. There is no --mirror, so a scheduled run never deletes
# a remote ref. The exit status is the first non-zero push status, or 124 for a push that was
# killed for running past its bound.
#
# It is written for the /bin/bash that macOS ships (3.2) and for launchd's environment: a PATH of
# /usr/bin:/bin:/usr/sbin:/sbin, no ~ expansion, a working directory of /, no ~/.zprofile.
#
# Environment (all optional):
#   GPB_PATH          PATH for git, the helper and the Proton CLI. Default:
#                     $HOME/.local/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin
#                     Set it where git lives elsewhere. The wrapper exports PATH="$GPB_PATH".
#   PROTON_DRIVE_CACHE_DIR
#                     Exported to git, the helper and the CLI when set (the plist may set it).
#   GPB_PUSH_TIMEOUT  Seconds each push may run. Default 1800. A push that runs longer has its
#                     whole process group (git, the helper, the CLI) ended and counts as exit 124.
#   GPB_MARKER_DIR    Where the failure marker goes. Default $HOME/Library/Logs.
#   GPB_NOTIFY        A command (one program name or path, no arguments) that is run with the
#                     failure message as its only argument. Unset: osascript posts a macOS
#                     notification. Either way it is fire-and-forget.
#
# The marker is $GPB_MARKER_DIR/gpb-v2-push.<cksum>.failed, where <cksum> is the first field of
#   printf '%s\n%s' "<repository, physical absolute path>" "<remote>" | cksum
# It is written on any failure and removed by the next run of the same repository and remote that
# succeeds. A run never touches another repository's marker.
#
# How a push is bounded
#   Each push runs as a background job under `set -m`, so it is the leader of a process group of
#   its own (its pgid is its pid) and git's children, the helper and the CLI, all join it. A
#   watchdog, also a job under `set -m` and so also in a group of its own, sleeps for the bound.
#   If it wakes, it
#     1. writes a sentinel file in the run's temp dir,
#     2. sends TERM to the push's group,
#     3. waits 10 s and sends KILL to the push's group.
#   When `wait` on the push returns, the wrapper looks for the sentinel:
#     present  The watchdog fired. The wrapper waits for the watchdog to finish step 3 and records
#              exit 124. It does not stop at the push's own exit, because git can die on TERM while
#              a grandchild that ignores TERM is still alive in the group, and only step 3 ends it.
#     absent   The push ended first. The wrapper ends the watchdog's whole group (the subshell and
#              its sleep), with a KILL if TERM has not done it within 2 s, and waits for it, so no
#              timer outlives a push, signals a recycled group later, or holds the wrapper's
#              output open.
#   Bash may print job notices ("Terminated: 15", "Killed: 9") for a killed push; they go to the
#   log, which is useful.
#
# The failure notification is started in a group of its own as well. launchd kills any process
# left in the job's process group when the job exits (launchd.plist(5), AbandonProcessGroup),
# which would cancel a notification still starting. Not yet observed under launchd itself; the
# live macOS gate checks that the notification arrives.
#
# If the wrapper is itself sent TERM, INT or HUP it ends the push and the watchdog (TERM, then KILL
# after 10 s), removes its temp dir and exits 143, 130 or 129. That leaves no marker.
#
# Exit status: 0 both pushes succeeded; the first non-zero push status; 124 a push was killed for
# running past the bound; 2 no temporary directory could be made, so nothing was pushed; 64 wrong
# arguments.

set -u

GPB_PATH=${GPB_PATH:-$HOME/.local/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin}
PATH=$GPB_PATH
export PATH
if [ -n "${PROTON_DRIVE_CACHE_DIR:-}" ]; then
    export PROTON_DRIVE_CACHE_DIR
fi

if [ "$#" -ne 2 ] || [ -z "$1" ] || [ -z "$2" ]; then
    echo "usage: gpb-v2-push.sh <repository> <remote>" >&2
    exit 64
fi

repo=$1
remote=$2

bound=${GPB_PUSH_TIMEOUT:-1800}
case $bound in
    ''|*[!0-9]*) bound_ok=0 ;;
    *) if [ "$bound" -gt 0 ]; then bound_ok=1; else bound_ok=0; fi ;;
esac
if [ "$bound_ok" -ne 1 ]; then
    echo "gpb-v2-push: GPB_PUSH_TIMEOUT='$bound' is not a positive whole number of seconds; using 1800" >&2
    bound=1800
fi

marker_dir=${GPB_MARKER_DIR:-$HOME/Library/Logs}

# The repository's physical absolute path, so the marker name does not depend on how it was spelt.
# A path that cannot be entered is used as given; git will then report it and the run fails.
abs=$(CDPATH= cd -- "$repo" 2>/dev/null && pwd -P) || abs=$repo

sum=$(printf '%s\n%s' "$abs" "$remote" | cksum | sed 's/ .*//')
marker=$marker_dir/gpb-v2-push.$sum.failed

tmp=
push_pid=
wd_pid=
rc=0
first_rc=0
failures=
lock_note=0

log() {
    printf '%s gpb-v2-push: %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

# end_watchdog: ends the watchdog's whole group (the subshell and its sleep) and reaps it. TERM
# comes first, and the watchdog's trap makes it exit quietly. Bash may run a trapped signal only
# when its foreground sleep returns (not measured), so a TERM that arrives just before that sleep
# is forked could leave a timer running for the whole bound; if the group has not gone in 2 s,
# KILL it.
end_watchdog() {
    kill -TERM -- "-$wd_pid" 2>/dev/null
    i=0
    while [ "$i" -lt 20 ] && kill -0 -- "-$wd_pid" 2>/dev/null; do
        sleep 0.1
        i=$((i + 1))
    done
    if kill -0 -- "-$wd_pid" 2>/dev/null; then
        kill -KILL -- "-$wd_pid" 2>/dev/null
    fi
    wait "$wd_pid" 2>/dev/null
}

# Ends whatever a signal, or a normal exit, leaves behind: the watchdog first (so it cannot fire
# into the push's group half way through), then any push still running, then the temp dir.
cleanup() {
    trap '' TERM INT HUP
    if [ -n "$wd_pid" ]; then
        end_watchdog
        wd_pid=
    fi
    if [ -n "$push_pid" ]; then
        kill -TERM -- "-$push_pid" 2>/dev/null
        i=0
        while [ "$i" -lt 10 ] && kill -0 -- "-$push_pid" 2>/dev/null; do
            sleep 1
            i=$((i + 1))
        done
        if kill -0 -- "-$push_pid" 2>/dev/null; then
            kill -KILL -- "-$push_pid" 2>/dev/null
        fi
        wait "$push_pid" 2>/dev/null
        push_pid=
    fi
    if [ -n "$tmp" ]; then
        rm -rf "$tmp"
    fi
}
trap cleanup EXIT
trap 'exit 143' TERM
trap 'exit 130' INT
trap 'exit 129' HUP

# notify <message>: fire and forget. Started in a process group of its own, detached from this
# script's stdin, stdout and stderr, never waited on, its status ignored. A hung or failing
# notification can neither change the outcome nor hold a caller's pipe open.
notify() {
    msg=$1
    set -m
    if [ -n "${GPB_NOTIFY:-}" ]; then
        "$GPB_NOTIFY" "$msg" </dev/null >/dev/null 2>&1 &
    else
        esc=$(printf '%s' "$msg" | tr '\n' ' ' | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g')
        osascript -e "display notification \"$esc\" with title \"git-proton-backup\"" \
            </dev/null >/dev/null 2>&1 &
    fi
    set +m
    return 0
}

# run_push <flag>: runs `git push <remote> <flag>` bounded by $bound seconds; sets rc.
run_push() {
    flag=$1
    sentinel=$tmp/fired.${flag#--}
    rm -f "$sentinel"

    set -m
    git -C "$abs" push "$remote" "$flag" </dev/null &
    push_pid=$!
    (
        trap 'exit 0' TERM
        sleep "$bound"
        : >"$sentinel"
        kill -TERM -- "-$push_pid" 2>/dev/null
        sleep 10
        kill -KILL -- "-$push_pid" 2>/dev/null
    ) </dev/null >/dev/null 2>&1 &
    wd_pid=$!
    set +m

    wait "$push_pid"
    rc=$?

    if [ -e "$sentinel" ]; then
        wait "$wd_pid" 2>/dev/null
        rc=124
        log "git push $remote $flag ran past ${bound}s and was killed"
    else
        end_watchdog
    fi
    wd_pid=
    push_pid=
}

write_marker() {
    mkdir -p "$marker_dir" 2>/dev/null
    {
        echo "git-proton-backup: a scheduled push failed"
        echo "time: $(date '+%Y-%m-%d %H:%M:%S %Z')"
        echo "repository: $abs"
        echo "remote: $remote"
        printf '%s' "$failures"
        if [ "$lock_note" = 1 ]; then
            echo "the push was killed; if the next push reports that this repository is locked by this Mac, that lock is the killed push's: remove it as that message says (README, scheduled use)"
        fi
    } 2>/dev/null >"$marker.tmp" && mv -f "$marker.tmp" "$marker" 2>/dev/null \
        || log "could not write the failure marker $marker"
}

record() {   # record <description> <status>
    failures="${failures}failed: $1 (exit $2)
"
    if [ "$first_rc" -eq 0 ]; then
        first_rc=$2
    fi
    if [ "$2" -eq 124 ]; then
        lock_note=1
    fi
}

log "start: $abs -> $remote (bound ${bound}s)"

tmp=$(mktemp -d "${TMPDIR:-/tmp}/gpb-v2-push.XXXXXX" 2>/dev/null) || tmp=
if [ -z "$tmp" ]; then
    log "could not create a temporary directory; no push was run"
    record "setup: no temporary directory" 2
else
    for flag in --all --tags; do
        run_push "$flag"
        log "git push $remote $flag: exit $rc"
        if [ "$rc" -ne 0 ]; then
            record "git push $remote $flag" "$rc"
        fi
    done
fi

if [ "$first_rc" -ne 0 ]; then
    write_marker
    notify "git-proton-backup: push failed for $abs ($remote). $(printf '%s' "$failures" | tr '\n' ' ')See $marker"
    exit "$first_rc"
fi

rm -f "$marker"
log "done: $abs -> $remote"
exit 0
