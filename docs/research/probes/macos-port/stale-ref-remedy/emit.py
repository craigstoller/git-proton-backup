# Writes one `git update-ref --stdin` command line with exact bytes, so no shell
# argument or terminal can normalise the name.
# Usage: python3 -I emit.py <op> <prefix> <nfc|nfd> [<oid> ...]
#   emit.py create refs/heads nfd <oid>  ->  b"create refs/heads/cafe\xcc\x81 <oid>\n"
import sys

op, prefix, form = sys.argv[1], sys.argv[2], sys.argv[3]
name = {"nfc": b"caf\xc3\xa9", "nfd": b"cafe\xcc\x81"}[form]
line = op.encode() + b" " + prefix.encode() + b"/" + name
for oid in sys.argv[4:]:
    line += b" " + oid.encode()
sys.stdout.buffer.write(line + b"\n")
