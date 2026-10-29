# Prints a repository's loose ref files under one namespace, with raw name bytes,
# and its packed-refs entries, with raw name bytes. Reads only.
# Usage: python3 -I show.py <repo-dir> <namespace, e.g. refs/heads>
import os
import sys

repo = os.fsencode(sys.argv[1])
ns = os.fsencode(sys.argv[2])
d = os.path.join(repo, b".git", ns)
print("    loose files in", sys.argv[2] + ":")
if os.path.isdir(d):
    names = sorted(os.listdir(d))
    if not names:
        print("      (none)")
    for n in names:
        p = os.path.join(d, n)
        if os.path.isfile(p):
            with open(p, "rb") as f:
                print("     ", repr(n), "->", f.read().strip()[:12].decode())
        else:
            print("     ", repr(n), "(directory)")
else:
    print("      (no such directory)")
pr = os.path.join(repo, b".git", b"packed-refs")
print("    packed-refs entries:")
found = False
if os.path.exists(pr):
    with open(pr, "rb") as f:
        for line in f:
            if line.startswith(b"#") or line.startswith(b"^"):
                continue
            oid, _, name = line.rstrip(b"\n").partition(b" ")
            print("     ", repr(name), "->", oid[:12].decode())
            found = True
if not found:
    print("      (none)")
