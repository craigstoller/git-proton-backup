# Probes run inline while writing the plan, 2026-10-27 (git 2.54.0, go1.27.0)

PM2 (fetch refspec shapes): in a scratch repo with remote.o.url = a local bare repo
holding refs/heads/main and refs/heads/café (NFC), for each value of remote.o.fetch:
  refs/heads/main:                 -> exit 0, no refs written
  refs/heads/main                  -> exit 0, no refs written
  heads/main:refs/remotes/o/main   -> exit 0, wrote refs/remotes/o/main
  refs/heads/main:heads/x          -> exit 0, wrote refs/heads/x

PM7: ~/sdk/go1.27.0/bin/go tool vet help | grep stdversion
  -> "stdversion   report uses of too-new standard library symbols"

PM8: Hangul syllables U+AC00..U+D7A3 listed in NormalizationTest.txt 17.0.0 Part 1: 11172 of 11172
  (awk and Python counts agree); file parts: @Part0..@Part5.

PM9: remote.o.url set, no remote.o.fetch, `git fetch o`:
  non-bare repo with one commit -> refs unchanged (refs/heads/master only), no FETCH_HEAD line
  bare repo                      -> no refs
  (the probe remote's HEAD named a missing branch, so git also printed
   "fatal: couldn't find remote ref HEAD")

Gatekeeper (M18-M20): run as inline commands; their outputs are gatekeeper-run*.out
one level up. No script file exists for them.

Unicode 17.0.0 inputs: fetched from https://www.unicode.org/Public/17.0.0/ucd/,
sizes and SHA-256 as in the plan's Global Constraints table; no checksum file in the
ucd/ listing, none mentioned in either ReadMe.
