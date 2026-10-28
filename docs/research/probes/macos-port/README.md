# macOS port probes

**Pinned results, run locally on 2026-10-27.** These are the probes the macOS port's design rests
on: scripts, stand-in helpers, small Go and Python programs, and the logs they printed. They ran on
one MacBook Air (Apple Silicon, macOS 27.2, APFS case-insensitive) with Apple git 2.54.0, go1.27.0
and `/bin/bash` 3.2.57. All are local: no network and no Proton account. The three
`gatekeeper-*.out` files come from a different kind of run, the real `git-remote-proton` build
started by git, offline. Nothing here was re-run when it was committed; each log is as the run left
it.

The finding numbers are the design's own. **M12–M29** are rows of "What the probes established" in
[the spec](../../../superpowers/specs/2026-10-27-v2-macos-port-design.md). **PM1–PM9** are the
measurements listed in [the plan](../../../superpowers/plans/2026-10-27-v2-macos-port.md). M0–M11
(the 2026-10-23 runs against Proton's CLI and the hermetic suite) have no files here.

This is a subfolder, unlike the flat probes beside it, because the port's probes come to 34 files.
Eight more, run on 2026-10-28 for Amendment MA3, sit in `stale-ref-remedy/`; the last section
below covers them.

## Reading the files

- `$SP` in a log stands for the scratch folder the probe was run in; `$W`, for the work folder
  passed to the script. Each script's header comment says what it asks. Its arguments are named in
  the first lines of its body: `W`, the work folder, and where it uses one, `H`, the folder holding
  the stand-in helper. Two scripts differ: `helper-protocol/run.sh` takes only `W` and uses its own
  folder for `H`, and `nfd-probe2.sh` takes the folder `nfd-probe.sh` made (`cd "$1"`; its header
  comment gives the usage).
- The probe scripts carry no executable bit; run them with `bash`. The three `git-remote-logtest*`
  stand-in helpers do, because git starts them. A stand-in must be on `PATH` under the plain name
  `git-remote-logtest`, which is what a `logtest::` URL makes git look for, so
  `clone-at-list/git-remote-logtest-v2` has to be installed under that name.
- The stand-ins differ in what they record. `helper-protocol/`'s logs the arguments and every line
  git sends. `gitdir-argv/`'s adds `GIT_DIR` and the working directory. `clone-at-list/`'s adds, at
  `list`, the new repository's ref format and its `remote.*` configuration.
- `fold-classes/` runs from inside its own folder. `check.py` reads the orbits file named by its
  first argument: `python3 check.py orbits.txt`. The files do not record how `orbits.txt` was made;
  `go run main.go > orbits.txt` is inferred, because it is the form `main.go` prints (a
  `# unicode.Version` line, then one orbit per line with a trailing space). `iota-minfold.go` takes
  no argument.

## Which file shows which finding

| Finding | Files | What they show |
|---|---|---|
| M12, M13, M14, M15, M17 | `nfd-probe.sh`, `nfd-run1.log` | Step 0: APFS finds an NFD name at an NFC file, and one entry results (M12). Step 3 and S1: `core.precomposeunicode=true`, and `git branch <NFD>` stores the NFC bytes (M13). S1: a clone of the bundle holding both spellings keeps both tracking refs, and `fsck` is clean (M14). S1 to S4: typed names land on the NFC branch; exact NFD bytes put HEAD on the NFD name at the NFC branch's commit; with refs packed first, the NFC branch is repointed to the NFD commit; the directory pair is re-normalised (M15). S5: a `reftable` clone keeps both (M17). |
| M15, M16, M17 | `nfd-probe2.sh`, `nfd-run1-fetch.log` | S3b: typing the NFC name of a directory branch the remote holds as NFD fails with `fatal: invalid reference` (M15). S6: the remote moves only the NFD branch, the fetch exits 0, and the NFC tracking ref shows the NFD commit while `fsck` reports the NFC commit dangling. S7: with both moved, the fetch exits 1 with `cannot lock ref` (M16). S8: the same two fetches into a `reftable` clone keep both (M17). |
| M18 | `gatekeeper-run1-quarantined-helper.out` | git's stderr when a quarantined helper copy is started: `died of signal 9`, `remote helper 'proton' aborted session`. |
| M19 | `gatekeeper-run2-quarantined-cli.out` | A quarantined copy of Proton's CLI run through an unquarantined helper: the CLI's own `You need to login first`, relayed by the helper. |
| M20 | `gatekeeper-run3-xattr-removed.out` | The same text, byte for byte, from the helper copy after `xattr -d com.apple.quarantine`. This run went through a stand-in CLI behind a fresh rebuild of the helper (recorded when the probe ran; the output file does not show it, and the spec's M20 row does not say it; see below). |
| M21 | `mnprobe.go` | The twelve samples (NFC and NFD `café`, Vietnamese, Hindi, Thai, Korean, Japanese) tested against "refuse any name containing `unicode.Mn`". |
| M23 | `fold-classes/` | `main.go` printed every multi-member `unicode.SimpleFold` orbit, smallest first, into `orbits.txt` (first line: Unicode 17.0.0; 1,482 orbits). `check.py` checks them against Python's `unicodedata`: U+1FBE's decomposition, the orbits that mix combining classes, and the orbits whose smallest member decomposes while another does not. `iota-minfold.go` prints where `ι`, `Ι`, U+1FBE and U+0345 fold to. |
| M24 | `delete-aliasing/run.sh`, `run1.log`, `run2.log` | Case A (both tracking refs packed) and case B (the NFC tracking ref loose after a fetch) for a delete push of the NFD name from a `files` clone with `core.precomposeunicode=false`. In case B the NFC tracking ref goes back to its packed value. `run2.log` repeats `run1.log` with fresh commit IDs, except for one line, case B's listing of the remote after the push (line 36 of each); see "What a file does not show". |
| M25 | `gitdir-argv/gitdir.sh`, `git-remote-logtest`, `gitdir.log` | `GIT_DIR` and arguments as git passes them to a helper: `ls-remote` at a repository's top level and in a subfolder (`GIT_DIR=.git`), in a folder no repository encloses (`GIT_DIR` unset), and `fetch` by remote name and by anonymous URL. |
| M25 (earlier measurement) | `helper-protocol/run.sh`, `git-remote-logtest` | What git sends a helper for `fetch` with the advertised tip already local and absent, `ls-remote` inside and outside a repository, and `clone`. `gitdir-argv/gitdir.sh` opens "Re-measure GIT_DIR and argv"; reading that as a re-measurement of this run is this README's inference, and the stand-in used there, `gitdir-argv/git-remote-logtest`, adds `GIT_DIR`. No log of this run was kept. |
| M26, M27 | `gitdir-argv/m26-m27.md` | The two one-line probes, as commands and results in text. They were not scripts. |
| M28 | `clone-at-list/clone-gitdir.sh`, `git-remote-logtest-v2`, `clone-default.log`, `clone-reftable.log`, `clone-gitdir.log` | A clone's helper at `list`: first argument `origin`, `GIT_DIR` set to the new `.git`, ref format `files` or `reftable`, `remote.origin.url` written, no `remote.origin.fetch`, no refs; `option cloning true` arrives after `list`. The first two logs are the stand-in's full logs; `clone-gitdir.log` is the script's own summary. |
| M29 | `quoting/main.go` | Prints NFC and NFD `refs/heads/café` with `%q` and `%+q`. |
| PM1 | `delete-aliasing/uptodate.sh`, `uptodate.log` | Case 2: with a stale loose NFD tracking ref at B beside an NFC branch already equal to the remote at A, `git push` prints `Everything up-to-date` and the NFD tracking ref then reads A. |
| PM2, PM7, PM8, PM9 | `plan-probes/inline-probes.md` | Commands and results as text. These ran as inline commands and have no script: fetch refspec shapes (PM2), `go vet`'s `stdversion` check (PM7), the Hangul count in `NormalizationTest.txt` Part 1 (PM8), a remote with no fetch refspec (PM9). |
| PM3, PM4 | `plan-probes/colon-name.sh`, `colon-name.log` | `git remote add 'a:b'` is refused; `git config --get` and `--get-all` on an anonymous URL's key both exit 1. |
| PM5, PM6 | `plan-probes/worktree-pgrp.sh`, `worktree-pgrp.log` | A linked worktree's git directory gives the same ref format, `remote.origin.url` and ref names as the main one. Under bash 3.2 with `set -m`, `kill -TERM -- -<pgid>` ends a background job and its grandchild while the script goes on. |

## What a file does not show

- **No output kept.** `mnprobe.go` (M21), `quoting/main.go` (M29), `fold-classes/check.py` and
  `iota-minfold.go` (M23) are programs whose printed output was not saved; the spec's rows record
  it. The same holds for the `helper-protocol/` run.
- **M22** (the `sandbox-exec` check) has no file here.
- **Gatekeeper (M18–M20)** was run as inline commands, so only the three outputs exist; no script is
  claimed. They hold git's and the helper's stderr only. The 21 seconds of M18 and the absence of a
  dialog are in the spec, not in these files. `gatekeeper-run2-quarantined-cli.out` and
  `gatekeeper-run3-xattr-removed.out` are byte-identical, so the files cannot tell the two setups
  apart. Run2 went through Proton's CLI: that rests on the spec's M19 row, not on the file. Run3
  went through a stand-in CLI behind a fresh rebuild of the helper: that was recorded when the probe
  ran; the output files do not show it, and the spec's M20 row does not say it.
- **M24, run1 against run2.** M24 says the remote still holds C after the case-B push, and
  `run2.log:36` shows that: the remote's `café` is at the same commit as `main`. `run1.log:36` lists
  the remote's `café` at the case-A commit instead. With run1's commit IDs mapped onto run2's roles
  (NFD is B, NFC is A, `main` is C), that is the only line in which the two logs differ.
  [inference] Nothing in the committed `run.sh` between the case-B fetch and that listing writes the
  remote except the delete of the NFD ref, so run1's line cannot be reproduced from this script and
  probably came from an earlier revision of it. M24's local claims (the NFC tracking ref reverts to
  its packed A, `[deleted] café`, exit 0, `fsck` silent) hold in both logs; its remote clause rests
  on `run2.log`.
- **PM1, case 1.** It failed (`src refspec main does not match any`, the clone having no local
  `main`) and shows nothing. Case 2 is the evidence.
- **PM3, the second sentence.** The log shows git reading a hand-written `[remote "a:b"]`: `git
  ls-remote a:b` exits 128 on that section's own `fetch` refspec (`invalid refspec`), before any
  helper starts. No helper is shown receiving `a:b`.
- **M25 and named remotes.** `gitdir.log` has `ls-remote` only with an anonymous URL. The
  named-remote `ls-remote` call is in `helper-protocol/run.sh`, which has no log here.

## Changes made when these were committed

The files are copies of the originals with these edits only:

- The four Go files (`mnprobe.go`, `quoting/main.go`, `fold-classes/main.go`,
  `fold-classes/iota-minfold.go`) start with `//go:build ignore` and a blank line. Without it,
  `go build ./...`, `go vet ./...` and `go test ./...` would pick them up, and the two programs in
  `fold-classes/` would both declare `main`. Run one with `go run <file>`.
- Five lines in `nfd-run1-fetch.log`, `delete-aliasing/run1.log` and `run2.log` wrote the run's
  scratch folder out in full; they now say `$SP`, as the other logs do.

One note on a file, not an edit: `mnprobe.go` says its samples are Go escapes "so this file stays
ASCII". The file holds the characters themselves (NFC `café` is `63 61 66 c3 a9`, NFD
`63 61 66 65 cc 81`), so it is not ASCII and the comment is out of date. The comment was left as it
was.

## The stale-ref remedy (MA3): `stale-ref-remedy/`

**Pinned results, run locally on 2026-10-28** by the review of the plan's Task 13, on the same
Mac (Apple git 2.54.0, `/bin/bash` 3.2.57, macOS 27.2; pass 1's first lines show APFS opening an
NFC-named file by its NFD name). They test the in-place repair of a stale local ref that the
README's macOS honest limits give, and they are the measurement behind Amendment MA3 in
[the spec](../../../superpowers/specs/2026-10-27-v2-macos-port-design.md). Local repositories
only: no network, no Proton account. Every layout uses the two spellings of `café`, NFC
(`63 61 66 c3 a9`) and NFD (`63 61 66 65 cc 81`), and deletes the NFD one.

Each script takes an existing empty folder for its repositories and the folder holding `emit.py`
and `show.py`: `/bin/bash probe.sh <empty folder> <this folder>`, and the same for `probe2.sh` and
`probe3.sh`. `emit.py` writes one `git update-ref --stdin` line with exact bytes; `show.py` lists a
namespace's loose ref files and the packed-refs entries, both with raw name bytes. git runs with
no system or global config and fixed dates, so the commit IDs are the same in every log
(A=`cdeb7feb82d3`, B=`ea92338cb941`, C=`578d65cf39ea`).

| Layouts | Files | What they show |
|---|---|---|
| L1–L6 (pass 1) | `probe.sh`, `t13-probe-transcript.txt` | A bare exact-bytes delete under git's Mac default, `core.precomposeunicode=true`, which `git init` sets. **L1** (`refs/heads`: NFC packed at A, NFD loose at C) and **L5** (the same in `refs/remotes/proton-v2`, NFC at B): only the NFD ref goes. **L6** (only NFC, loose at B, and the delete naming NFD): the NFC ref is gone, and `git fsck` exits 0. **L2, L3 and L4 never built** the layout they name: `pack-refs` under the default setting wrote the NFD ref into `packed-refs` spelled in NFC, so the next step failed (`reference already exists` in L2 and L4). **L3** records the overwrite this causes: NFC was packed at A, and after `pack-refs` the one packed entry, spelled in NFC, holds C, the NFD ref's commit (`is at 578d65cf… but expected cdeb7feb…`). That is why the in-place route's `pack-refs` needs `-c core.precomposeunicode=false`. |
| L7–L9 (pass 2) | `probe2.sh`, `t13-probe-transcript3.txt` | The layouts pass 1 could not build, made two ways, then the bare delete. **L7** (NFD packed at C by `pack-refs` with precomposition off, NFC loose at B): both refs are gone. **L8** (a real `files` clone of a `reftable` remote holding both, then a fetch that writes NFC loose at B; M24's layout): NFD is gone and NFC falls back to its stale packed A, with `git fsck` exiting 0 and listing B, NFC's lost commit, among the dangling commits. **L9** (the clone held only NFD, packed at C; a fetch then wrote NFC loose at B): both refs are gone. |
| L10–L12 (pass 3) | `probe3.sh`, `t13-probe-transcript4.txt` | The same delete after `git -c core.precomposeunicode=false pack-refs --all`, in L8's layout (**L10**), L9's (**L11**) and L1's (**L12**). In all three only the NFD ref goes, and NFC keeps its current value (B, B and A). These are the "three ref layouts" the README and design doc cite. |

**What these files do not show.**

- **Pass 2's first run is not here.** `probe2.sh` first ran with a bug: in L8 and L9 the fetch, the
  listings and the delete ran against the bare remote instead of the clone, and its log shows
  `fatal: 'proton-v2' does not appear to be a git repository` and `fetch exit 128`. That log was
  discarded; `t13-probe-transcript3.txt` is the corrected run. The `probe2.sh` here keeps each
  repository in a local variable in its `upd` helper (`local target=$1`), where `probe.sh`'s
  helper assigns the global `r` that L8 and L9 use for the clone, so this copy matches the
  corrected run [inference from the code and both logs: the files do not record whether the
  script was edited again after that run].
- **Pass 3 runs no `git fsck`,** and none of the passes tests letter-case pairs, a `reftable`
  clone, or the helper itself.

**Changes made when these were committed.** The files are copies. In `t13-probe-transcript.txt`
only, the run's scratch folder is written `$SP` (the repositories' root was `$SP/t13-probe`), and
the local account name in the `ls -la` lines is written `$USER`. Two of those lines (32 and 202)
show the NFD file name raw, as `ls` printed it; the `od -c` lines below them give its bytes. The
scripts take their folders as arguments and hold no path.
