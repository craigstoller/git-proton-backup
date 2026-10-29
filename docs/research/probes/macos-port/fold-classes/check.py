"""Check the properties key(s) = caseKey(nfd(s)) relies on, against Python's
Unicode data, for every SimpleFold orbit Go printed.

1. Is U+1FBE canonically decomposable (agy's claim: it is not)?
2. Which orbits mix combining classes? (key correctness needs: only orbits whose
   nonzero-ccc member is the highest ccc in use, so folding it never needs a
   reorder.)
3. Which orbits have a canonically decomposable smallest member while some
   other member is not decomposable? (Such an orbit would let caseKey put a
   decomposable character into a key built from NFD input.)
"""
import sys
import unicodedata as ud


def canon(c):
    d = ud.decomposition(c)
    return d if d and not d.startswith("<") else ""


print("python unicodedata", ud.unidata_version)
for cp in (0x1FBE, 0x0345, 0x03B9, 0x1FB4, 0x1FB3):
    c = chr(cp)
    print(f"U+{cp:04X} ccc={ud.combining(c)} canonical-decomposition={canon(c) or '-'}"
          f" NFD={' '.join(f'{ord(x):04X}' for x in ud.normalize('NFD', c))}")

max_ccc = max(ud.combining(chr(i)) for i in range(sys.maxunicode + 1))
print("highest combining class in use:", max_ccc)

mixed, bad_min = [], []
for line in open(sys.argv[1]):
    if line.startswith("#") or not line.strip():
        continue
    orbit = [int(h, 16) for h in line.split()]
    cccs = {ud.combining(chr(m)) for m in orbit}
    if len(cccs) > 1:
        mixed.append(orbit)
    mn = orbit[0]
    if canon(chr(mn)) and any(not canon(chr(m)) for m in orbit[1:]):
        bad_min.append(orbit)

print("orbits mixing combining classes:")
for o in mixed:
    print("  ", " ".join(f"U+{m:04X}(ccc {ud.combining(chr(m))})" for m in o))
print("orbits whose smallest member is decomposable but another is not:", len(bad_min))
for o in bad_min:
    print("  ", " ".join(f"U+{m:04X}" for m in o))
