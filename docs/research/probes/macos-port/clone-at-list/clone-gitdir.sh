#!/bin/bash
# Does git set GIT_DIR, and has it created the new repository, when a clone's
# helper receives `list`? Local only, logging stand-in helper. Also records the
# new repository's ref format at that moment, for a default and a reftable clone.
set -u
H="$1"; W="$2"; rm -rf "$W"; mkdir -p "$W/plain"
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null PATH="$H:$PATH"
export LOGTEST_SHA=0123456789012345678901234567890123456789
for fmt in default reftable; do
  export LOGTEST_LOG="$W/clone-$fmt.log"; : > "$LOGTEST_LOG"
  extra=(); [ "$fmt" = reftable ] && extra=(--ref-format=reftable)
  (cd "$W/plain" && timeout 30 git clone ${extra[@]+"${extra[@]}"} logtest::x "new-$fmt" >/dev/null 2>&1)
  echo "clone ($fmt): $(grep '^ARGS:' "$LOGTEST_LOG" | sed "s#$W#\$W#g")"
  grep -E '^(IN: list|LISTFMT:|LISTFETCH:|LISTCONFIG:)' "$LOGTEST_LOG" | head -3 | sed 's/^/    /'
done
