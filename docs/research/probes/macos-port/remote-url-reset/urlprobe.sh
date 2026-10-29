#!/bin/bash
# Probe: which remote.<name>.url / pushurl does git hand a remote helper when an
# empty value appears in the list? A stub git-remote-proton logs its argv and exits.
set -u
S="$SP/urlprobe"   # SP: the scratch folder the run used (scrubbed when committed)
rm -rf "$S"
mkdir -p "$S/bin" "$S/home"
LOG="$S/argv.log"
cat > "$S/bin/git-remote-proton" <<EOF
#!/bin/bash
echo "argv: \$1 | \$2" >> "$LOG"
exit 1
EOF
chmod +x "$S/bin/git-remote-proton"
export PATH="$S/bin:/usr/bin:/bin"
export HOME="$S/home" GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
export GIT_TERMINAL_PROMPT=0 GIT_AUTHOR_NAME=p GIT_AUTHOR_EMAIL=p@example.invalid GIT_COMMITTER_NAME=p GIT_COMMITTER_EMAIL=p@example.invalid

run() { # name, then config lines
  local name=$1; shift
  local r="$S/$name"
  git init -q "$r"
  ( cd "$r" && git commit -q --allow-empty -m x )
  for kv in "$@"; do
    git -C "$r" config --add "${kv%%=*}" "${kv#*=}"
  done
  echo "== $name: $*" >> "$LOG"
  echo "-- ls-remote back" >> "$LOG"
  git -C "$r" ls-remote back >> "$LOG" 2>&1; echo "   exit $?" >> "$LOG"
  echo "-- push back HEAD:refs/heads/main" >> "$LOG"
  git -C "$r" push back HEAD:refs/heads/main >> "$LOG" 2>&1; echo "   exit $?" >> "$LOG"
}

run c1 'remote.back.url=' 'remote.back.url=proton::/x'
run c2 'remote.back.url=proton::/x' 'remote.back.url='
run c3 'remote.back.url=proton::/x' 'remote.back.url=' 'remote.back.pushurl=proton::/y'
run c4 'remote.back.url=proton::/x' 'remote.back.pushurl=' 'remote.back.pushurl=proton::/y'
run c5 'remote.back.url=proton::/x' 'remote.back.pushurl=proton::/y' 'remote.back.pushurl='
run c6 'remote.back.url=' 'remote.back.pushurl=proton::/y'
run c7 'remote.back.url=proton::/x' 'remote.back.url=' 'remote.back.url=proton::/z'
cat "$LOG"
