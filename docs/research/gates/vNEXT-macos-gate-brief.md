# vNEXT macOS live gate brief: the macOS port's one live gate, on Craig's MacBook Air

**Status: brief, not run; one Placeholder left, the version name** (Question 1). This is a set of
instructions for the runners of the one live macOS
gate that spec section 6 requires for the release that carries the macOS port
(`docs/superpowers/specs/2026-10-27-v2-macos-port-design.md`, "The one live macOS gate (MD5(a),
O5)"). It is not a record. It was written on 2026-10-27, and re-anchored on 2026-10-28, on branch
`worktree-macos-port`, from section 6 and the plan (`docs/superpowers/plans/2026-10-27-v2-macos-port.md`,
Task 15). Its code quotes and line numbers were read at **`0faada8`, the anchor**, after plan Tasks
12 and 13 and Amendment MA3; C.2 checks the tag against it.
Section 6 is the binding authority for what this brief contains and in what order; its step
numbers (0 to 7, and 5.1 to 5.10) are used unchanged. It has five parts:

- **Part A**, prep with no account: the v0.10.0 build;
- **Part B**, the pre-tag sign-in probe, steps 1 to 7;
- **Part C**, gate setup;
- **Part D**, gate steps 0 to 7, run from the Mac;
- **Part W**, the PC section, which the paired PC session runs from.

Every command appears exactly as its runner types it, in its shell, with its pass condition, what
counts as BLOCKED, and the expected output, written here before the run.

**`vNEXT` is a placeholder** for the release that carries the macOS port, whose version Craig has
not named. When he names it, this file is renamed to `docs/research/gates/<tag>-macos-gate-brief.md`
and every `vNEXT` in it, in prose and in code alike, is replaced with that tag by one scripted
substitution (see "How this file is edited"). The same script renames
`docs/research/gates/vNEXT-composition-refusal-steps.md` likewise and rewrites the two other files
that name these by path, `docs/releasing.md` and `docs/README.md`; afterwards `git grep vNEXT`
must find the old names only in dated records (the plan), which keep them [inference: as at the
anchor]. Remote root names carry it, so they change with it.
v0.10.0, the current release, keeps its own name throughout: Part A builds it as the "before"
helper that step 5.5 needs.

**Rules incorporated by reference.** Every step runs under `docs/research/gates/brief-checklist.md`
rules 1 to 12 (section 6: "It runs under `brief-checklist.md` rules 1-12"). Where a rule matters
for a step, the step points back to it; these pointers add no new rules. One deliberate deviation,
from rule 6, is stated at C.5, as the v0.10.0 briefs made it.

**The gate's place in the release (section 6, O5), restated so a runner need not look it up:**

- **Order.** The release's Linux gate, then its Windows gate, then this gate, last, after the
  draft Windows helper is installed, so that step 5's refusal from Windows is the new helper's.
  Never concurrently with another gate: the gates share the account's row-set baseline, the trash
  tally and the paused-task window. Within this gate the Mac session and the PC session run
  together, the PC session live through steps 5.5 and 5.8.
- **The held draft.** The release's draft stays unpublished until this gate ends, pass or fail, so
  the draft Windows helper that step 0 checks for is still the release's. W.C.2, W.0 and W.7.6
  check that it is still a draft.
- **A Mac-only failure does not hold the release.** No macOS asset ships. The Windows and Linux
  assets publish on their own gates, and the README keeps macOS "not yet live-gated" [README]
  until a Mac gate passes. A defect in shared code found here is a defect in the release, under
  `docs/releasing.md`'s no-retag rule: its draft is not published, and the fix ships as a new tag.
  The record classes each failure, and Craig rules the class before he publishes ("After the
  gate", "The release").
- **The retry rule.** After a failed Mac gate, the next attempt is all of section 6 again, at a
  later release: its own v0.10.0 build and sign-in probe, its own paired PC session, the same
  order after that release's Windows and Linux gates and its draft Windows helper, and its own
  brief and record. Never the failed steps alone, and never a stand-in. So this brief has exactly
  one gate run, `gate-a1`. Only Part B's probe may be re-run (`probe-a2`, on Craig's word), because
  it is prep and changes nothing in the release. *Brief author's call:* a setup failure in Part C
  that comes before any account write (a sign-in that does not complete, say) is fixed and retried
  inside `gate-a1`, as the v0.10.0 briefs retried keyring setup; anything from step 0 on is not.
  Step 0's git-version STOP comes first in its script, before anything is read or recorded;
  whether step 0 then runs again inside `gate-a1`, after Craig's ruling on that git, is his call
  (Question 6). 6.0's STOP on launchd's environment likewise comes before anything of step 6 is
  made or loaded, and whether 6.0 then runs again is his call too. In-step re-firings Craig orders
  are not retries: 6.1 (the screen was not locked),
  6.2 (permission granted, or the `abandon` agent) and 6.3 ((b) not exercised). Each is recorded
  with its count; none follows a BLOCK.
- **The composition refusal on the other platforms.** The release's Windows and Linux gates carry
  the steps in `docs/research/gates/vNEXT-composition-refusal-steps.md` (O5(d)).

**Rulings this brief rests on.**

- **MD4(b) and O2(b)** (spec section 4): the Mac's scheduled backups are one LaunchAgent per
  repository, pushed from that one machine; the gate's launchd step is made representative.
- **MD5(a) and O5** (spec section 6): this gate, its order, the held draft, the retry rule, the
  paired PC session, and the composition steps on the other platforms.
- **O3(d)** (spec section 3): the Mac refusals at `list` and in the push rule, and the warning on
  every platform.
- **O6(c)** (spec section 5): rule 12.
- **MA3** (spec, "Amendments": Craig's call, option (c), 2026-10-28): the stale-local-ref remedy.
  A bare exact-bytes delete can unlink the other spelling's loose ref on APFS, so the helper's
  refusal messages name only a `--ref-format=reftable` clone and point to the README for the
  in-place route (`pack-refs` with precomposition off, then the exact-bytes delete). Strings 6 and
  7 carry the new wording; step 5.11's remedy is the `reftable` clone; the in-place route is not
  run live (see "Not covered live").
- **D2(a)** (the Linux port spec): case-variant creates are refused on every platform.
- **R1 to R6** (plan, "Refinements the spec allows"): put to Craig on 2026-10-27 and not objected
  to, so unopposed defaults. R1 gets a step of its own here (5.11).
- **R7 and T9-2** (Rulings T9-1 and T9-2), refinements made during implementation. They are
  carried by the code (`internal/repo/macpush.go:113-123, 218-224`) and by commit `5079dc9`'s
  message; neither the spec nor the plan carries them yet [code]. Section 6 has no step that meets
  R7; see "R7 and this gate".

**Brief author's calls.** Where section 6 is silent, this brief makes a choice and marks it *Brief
author's call*, with the reason. Each is a default Craig may overturn, not his ruling. An index of
where they sit is near the end, and the ones that most need his word are "Questions for Craig",
below.

**Basis marks:**

- [spec], [plan]: read from the spec or the plan, by section.
- [code file:line]: read at the anchor, `0faada8`.
- [README]: read from `README.md` at the anchor.
- [record]: a committed gate record or brief, cited as of its own date.
- [doc]: read from the named outside document on 2026-10-27.
- [measured 2026-10-27]: a read-only check on this Mac while writing this brief. No account was
  touched, no `proton-drive` binary was executed, and nothing ran on the PC.
- [inference] and [untested].

**How this file is edited.** The file is ASCII. NFD text is never written raw, because a reader
cannot tell it from NFC; the three spellings this gate uses appear only as byte escapes or in the
escaped view below. Literal backslash-u sequences appear only in the section "Strings the helper
prints", and a common tool fault decodes them into raw characters when a file is saved (the plan's
self-review notes, round 3, record it). So the file is changed only by script (Python, building
each backslash from `chr(92)`), never by an editor tool, and is grepped afterwards: every
backslash-u in it must still be one. The renaming of `vNEXT` is such a script; it renames this
file and the steps file, rewrites `docs/releasing.md` and `docs/README.md`, and is followed by a
`git grep vNEXT`.

**R7 and this gate.** R7 refuses, on a Mac `files` clone, an update whose tracking ref aliases the
tracking ref of a delete in the same push, even when neither tracking ref is local; a create in
the same position is not refused [code `internal/repo/macpush.go:113-123`]. Its case is a one-push
repair of a pair from a Mac `files` clone: a delete of one spelling and an update of the other. On
the Mac the delete then lands and the update is refused with string 12's text [code
`internal/repo/repo_test.go:8470-8496, 8533-8548`], and the update then succeeds in a following
push, the pair being gone [inference]. Section 6
has no such push: its repair (5.10) runs from a `reftable` clone, which the Mac rules do not reach,
and section 3's repair says to avoid a delete push from a Mac `files` clone. So no step here
provokes R7, and it is listed under "Not covered live". If a runner meets string 12 anyway, it is
recorded and checked against `[r7-not-provoked]`; the step is still judged on its own pass
conditions, so string 12 in place of a step's expected output is BLOCKED. *Brief author's call:*
adding an R7 step is Craig's decision (Question 2).

---

## Questions for Craig

Each has this brief's default, which stands unless he rules otherwise, and its reason. A question
that others depend on comes first.

1. **The version name.** Default: none; the files stay `vNEXT` until he names the release, then are
   renamed by one scripted substitution. Reason: the root names, the tag C.3 checks out and every
   `--version` expectation carry it.
2. **R7: add a live step?** Default: no step, since section 6 has none ("R7 and this gate"). If he
   adds one, its shape: a remote of its own, the NFD member laid down beside the NFC one by a CLI
   upload, and a Mac `files` clone that holds neither `cafe` tracking ref pushes an update of the
   NFC member and a delete of the NFD one in one push; expected, the delete lands, the update is
   refused with `[r7-not-provoked]`'s text, and the update alone then lands. The NFC member must
   sit at a commit the clone holds, or the helper refuses the update with `fetch first` before the
   Mac rule speaks [code `internal/repo/push.go:275-278`]. Reason: R7 is a refinement made during
   implementation, and gate content is his to add.
3. **May a Part C setup failure before any account write be retried inside `gate-a1`?** Default:
   yes, once (a sign-in that does not complete at C.4, say). Reason: nothing has reached the
   account and no gate state is held, which is what section 6's "never the failed steps alone"
   protects; anything from step 0 on falls under the retry rule.
4. **Is the 1 s bound acceptable as the stand-in for a hang, and must 6.3 show the helper or the CLI
   reaped?** Default: yes to both; 6.3 passes only if a push killed at its bound had a
   `git-remote-proton` or `proton-drive` process in its tree, and otherwise ends "(b) not
   exercised: STOP for Craig". Reason: a real hang (a Keychain prompt on a locked screen) cannot be
   made on purpose, and a check that never saw the helper or the CLI killed shows nothing about
   reaping them.
5. **git configuration.** Default: the Mac's gate scripts switch git's system and global
   configuration off; step 6's launchd runs keep his (so his `core.hooksPath` hooks run on the
   scheduled pushes, as in real use); the PC's blocks keep his PC configuration, as the v0.10.0 PC
   blocks did. Reason: the refusal texts the Mac scripts check must not depend on his config, the
   launchd step tests the README's environment, and the PC's git is not what is under test. Is
   that mixed state what he intends?
6. **Hold Command Line Tools and Xcode updates until the gate has run?** Default: yes; step 0 STOPs
   on any git but `git version 2.54.0 (Apple Git-157)`. Reason: M24, PM1 and R7 were measured on
   that git.
7. **`caffeinate -is` for the whole gate, and who holds it?** Default: yes, held from the end of
   C.0 to 7.0, not only for step 6, which still needs mains power; Craig holds it, running
   `kit/awake.sh` in a Terminal window of his own, and `gx.sh` refuses (a STOP) any gate script from
   C.2 to step 6 while it is not running. Reason: rule 9's harm, a push killed mid-flight leaving an
   orphaned lock, applies to a Mac that sleeps mid-push at any step; and a runner's background
   command is killed at the harness's bound (two hours at most on Craig's machines), well short of
   the gate, so the runner cannot hold it.
8. **Notification permission (6.2).** Default: if macOS asks, allowing it is his decision, made at
   the prompt; the brief records the prompt and what the run did before it. Reason: it is a system
   setting.
9. **Pause `ProtonCliDriftCheck` along with `ProjectsBackupDaily`?** Default: yes. Reason: both run
   the Windows CLI, two CLI starts can collide on its cache (`cli-0.9.0-certification.md` 4.10), and
   the v0.10.0 gates paused both.
10. **Make 5.6's `reftable` clone before 5.5's Windows move?** Default: yes. Reason: its fetch then
    carries a real change to one member; after the move there would be nothing to fetch.
11. **Keep the installed binaries and `~/gpbmac` after the gate?** Default: keep the helper and CLI
    in `~/.local/bin` (section 6's cleanup does not remove them, and real use starts from them), and
    keep `~/gpbmac` until the record is written, then delete it on his word. Reason: the run folders
    are the record's evidence (step 7, "What stays", says what they hold).
12. **C.3: the README's commands or section 1's recipe?** Default: the README's, as section 6
    requires ("with the README's exact commands"). C.3 and step 1 read `README.md` at the tag and
    run its macOS install lines, with the tag in place of `vX.Y.Z`, only after `kit readme` finds
    them equal to the lines C.3 quotes from the README at the anchor; a difference is a STOP for
    adjudication. Reason: the gate tests the install the README gives, and no runner runs README
    lines this brief has not quoted.
13. **A C.2 STOP: hold, or run step 7?** Default: hold. Nothing is cleaned and the PC's tasks stay
    paused until Craig rules whether the tag's brief stands as written for the files C.2 lists. If
    it stands, the gate goes on at C.3; if not, step 7 runs (C.2 says what it does then) and the
    Mac gate is retried at a later release. Reason: nothing has reached the account at C.2; a brief
    at a tag cannot be re-anchored, since the gate runs the tag's brief and tags never move; and
    step 7 deletes Part A's build, which a gate that goes on still needs. C.3's and step 1's STOPs
    (exit 3) hold the same way, for the same reasons.

---

## Names and values

| | |
|---|---|
| Release | tag `vNEXT`, a **DRAFT** release of `craigstoller/git-proton-backup`, unpublished until this gate ends |
| No macOS asset | the helper is built from source at the tag (spec section 1, MD2(d)) |
| Anchor | `0faada8` (`0faada84151fde79376bac180cda32fa8277e31f`), after plan Tasks 12 and 13 and MA3; C.2 diffs it against the tag |
| v0.10.0 | tag commit `e5c76af3bfc78ee90efcf60db8b11d8d0ab76099` [measured 2026-10-27] |
| Certified CLI | `cli-drive@0.9.0+75612299` (`transport.CertifiedCLI`, `internal/transport/cli.go:572`), unchanged since v0.10.0 [measured 2026-10-27: `cli.go` has no diff from v0.10.0] |
| Certified macos/arm64 SHA-512 | M2's value (spec): `3f4125de5e41dcc7f4e0e07e8d60737aa18f35c6ecc1ba3fce47ea97b331857eefaf88e74671135af57122d4cb5439b817ec528f8df383550ac7810aeaf0217b`. Read by script from `$CertifiedMacArm64Sha512` in `scripts/Test-ProtonCliDrift.ps1` and required to equal this quote. At the anchor it is this value, on a line the scripts' pattern matches [measured 2026-10-28: `scripts/Test-ProtonCliDrift.ps1:53`] |
| CLI download | `https://proton.me/download/drive/cli/0.9.0/darwin-arm64/proton-drive` (M1) |
| Mac | MacBook Air, Apple Silicon, macOS 27.2 build 26B123, APFS case-insensitive; `/usr/bin/git` is `git version 2.54.0 (Apple Git-157)`, exec-path `/Applications/Xcode.app/Contents/Developer/usr/libexec/git-core` (176 entries, no `git-remote-proton*`); go1.27.0 at `~/sdk/go1.27.0/bin/go`, not on PATH; `/bin/bash` 3.2.57; `/usr/bin/python3` 3.9.6; `hostname` prints `mac.lan` [measured 2026-10-27] |
| Mac state today | `~/.local/bin` holds `agy` and `claude` only; no `git-remote-proton` or `proton-drive` in it, in `/opt/homebrew/bin` or in `/usr/local/bin`; M7's three default folders absent; `~/gpbmac` absent; on battery [measured 2026-10-27] |
| Mac global git config | sets `init.defaultBranch`, `core.excludesFile` and `core.hooksPath` (Craig's chaining hooks), and no `fetch.*`, `push.*`, `remote.*` or `url.*` key [measured 2026-10-27, names only] |
| PC | Windows, PowerShell 7.6.7, Git for Windows 2.55.0.windows.3, `proton-drive` at `C:\Users\craig\Tools\proton-drive.exe`, the helper at `%LOCALAPPDATA%\Programs\git-proton-backup\git-remote-proton.exe`, tasks `ProjectsBackupDaily` and `ProtonCliDriftCheck` at `\` [record `v0.10.0-linux-gate-brief.md`, measured there 2026-10-16]; `gh`, signed in to GitHub (W.C.2's draft check needs it). W.B.0 re-checks them, `gh auth status` included |
| Main checkouts | Mac `~/Projects/_Tools/git-proton-backup`; PC `C:\Users\craig\Projects\_Tools\git-proton-backup` |
| Runs | `probe-a1` (a re-run, on Craig's word only, is `probe-a2`), and `gate-a1`, the only gate run (the retry rule). SUF is the attempt, `a1` |
| Mac scratch | `~/gpbmac/`: `extract.py`, `kit/`, `scripts/`, `cli/` (the feed-verified CLI), `v010-build/` (Part A), `run.txt`, and one folder per run holding `env.sh`, `cache/`, `log/`, `out/`, `snap/`, `refsets/`, `relay/`, `envprobe/` (B.0's and 6.0's probe agent) and the run's repositories; the gate run also holds the tag's clone `git-proton-backup-vNEXT` (`$SRC`, C.3) and the keep-awake's `caffeinate.pid` |
| PC scratch | `C:\gpbmac\`: `extract.ps1`, `kit\`, `run.txt`, and one folder per run holding `log\`, `refsets\`, `relay\`, `stage\` and `w\` (repositories) |
| Gate cache folder | `~/gpbmac/gate-a1/cache`, the gate's `PROTON_DRIVE_CACHE_DIR`; the probe's is `~/gpbmac/probe-a1/cache` |
| Installed binaries | `~/.local/bin/git-remote-proton` (built at the tag) and `~/.local/bin/proton-drive` (the certified CLI) |
| Spellings | NFC `caf` U+00E9 = bytes `63 61 66 c3 a9`; NFD `cafe` U+0301 = `63 61 66 65 cc 81`; the third, CAP, `Caf` U+00E9 = `43 61 66 c3 a9`. Scripts hold them as `$'caf\xc3\xa9'`, `$'cafe\xcc\x81'` and `$'Caf\xc3\xa9'` (section 6's form), the PC as `"caf$([char]0x00E9)"` and `"cafe$([char]0x0301)"` |
| Remote repo roots | `/my-files/GitRemotes/vNEXT-mac-<role>-a1`, roles `smoke` (step 3), `xm` and `xw` (step 4), `nfd` (5.1, 5.2, 5.4 to 5.7), `case` (5.3), `ren` (5.8, 5.9.2), `mp1` (5.9.1, 5.10), `mp3` (5.9.3), `r1` (5.11), `launchd` (step 6) |
| Contract live root | `GPB_CONTRACT_LIVE_ROOT=/my-files/_cas-probe/contract-mac` (section 6 step 2, rule 8) |
| Never created | `/my-files/GitRemotes/vNEXT-mac-absent-a1`, the missing parent step 6's failing pushes name |

## Confinement (rules 2, 3, 7 and 8)

- **Writes go only to this run's eleven roots:** the ten repo roots above, and the contract live
  root `/my-files/_cas-probe/contract-mac`, which section 6 names and which is part of this list
  (rule 8). `GPB_CONTRACT_LIVE_ROOT` is set for step 2's one command only. The table's
  compiled-in default, `/my-files/_cas-probe/contract` [code
  `internal/transport/contract_test.go:21`], must never be reached; 2.2's evidence is that
  `contract-mac` exists after the table ran.
- **Parents.** The table's `EnsureDir(base)` creates `contract-mac` and no parent [code
  `internal/transport/contract_test.go:627`], so `/my-files/_cas-probe` must exist; the v0.10.0
  record lists it with one child, `contract` [record `v0.10.0-linux-gate.md:318, 322`]. Step 0
  BLOCKs if it is missing: creating it is not authorised. `/my-files/GitRemotes` is created only
  at step 2.0, only if step 0 found it absent, with `filesystem create-folder /my-files
  GitRemotes`, as v0.10.0 did. `GPB_CREATE_PARENTS` is never set [code
  `cmd/git-remote-proton/main.go:740`], so no push creates a parent; step 6's failing pushes rely
  on that, and B.0 and 6.0 check that launchd's own environment does not set it either (step 6,
  "Nor may launchd's own environment set it").
- **What this run owns, and so may trash, is recorded, never inferred from names.** Step 0 starts
  the ownership record `~/gpbmac/gate-a1/roots.tsv`, and only when its checks pass. Each root is
  added the first time it is seen live, with its uid (`kit roots update`). Step 7.1 trashes only
  recorded roots, each after re-reading its uid and enumerating its subtree. A run BLOCKED at step
  0 owns nothing and trashes nothing. *Brief author's call, the v0.10.0 Linux brief's rule.*
- **`/my-files/GitRemotes` itself:** absent at step 0, it is created at 2.0 and trashed at 7.1 only
  if 2.0's script exited 0 and it holds nothing live; present at step 0, it is not this gate's,
  never trashed, and every child but this run's roots is read-only. A child already carrying one
  of this run's root names, or `vNEXT-mac-absent-a1`, is a stale attempt: BLOCKED, no record
  written.
- **`/my-files/_cas-probe` and its `contract` child are read-only,** as the six untouchable
  top-level folders are: `GitBackups`, `Sensitive Project Sources`, `Project Repo Bundles`,
  `ChatGPT Export Text Backup`, `Tax Documents` and `Archive` [code
  `internal/transport/contract_test.go:41-48`]. They appear only as rows in listings. Only
  `contract-mac` is written beneath `_cas-probe`.
- **Verify before trashing, with full subtree enumeration, every time (rule 3).** The only trash
  commands this gate makes are 7.1's.
- **Which helper invocations take the repo lock.** `list for-push`, which every `git push` sends
  [code `cmd/git-remote-proton/main.go:754`], and `--set-head`. `list`, which `git ls-remote`,
  `git clone` and `git fetch` send, takes none. A lock session ends by trashing its `.lock` [code
  `internal/repo/lock.go:135`], including a session the Mac refusal at `list for-push` ends: that
  refusal comes after `AcquireLock`, and the deferred release runs [code
  `cmd/git-remote-proton/main.go:773-779, 593-614`]. A push that fails in `EnsureParents`
  (step 6's failing pushes) takes no lock: it comes before `Bootstrap` and `AcquireLock` [code
  `cmd/git-remote-proton/main.go:740-759`].
- **The trash tally (rule 7)**, taken at 7.2, counts folders as well as files. The expected rows,
  by class [inference from the code cited and the steps below]:
  - **One `.lock` file per lock session.** The gate's pushes, Mac and PC, and its one
    `--set-head` make **39** before step 6: step 3 makes 4, step 4 makes 4, 5.0 makes 2, 5.1 2,
    5.2 1, 5.3 7, 5.4 2, 5.5 1, 5.8 4, 5.9.1 3, 5.9.2 1, 5.9.3 2, 5.10 2 and 5.11 4. Step 6 adds
    2 for each firing of its succeeding agent (one expected; a re-fire, or a second firing a
    `FIRINGS` line counts, adds two more each, 6.4) and none for its failing ones.
  - **Four ref files from deletes:** `topic-m` and `topic-w` (step 4), the NFC `cafe` (5.8, from
    the PC) and the NFD `cafe` (5.10) [code `internal/repo/push.go`, phase 4].
  - **19 folders from step 2's contract table:** its 17 per-test roots, plus `trash-empty` and
    `trash-subtree` [code `internal/transport/contract_test.go:413-452, 626-637`; 17 cases counted
    at the anchor].
  - **The eleven roots 7.1 trashes,** plus `GitRemotes` if this run created it; each root's
    enumeration at 7.1 is the record of what went with it.
  - **A trashed folder's contents** are in none of these counts, the contract folders' and the
    roots' alike: whether the trash lists them as rows of their own is not known (C.5), so any
    that appear are recorded with the folder they came from, not predicted.
  - **No prune folders:** no nested branch is deleted.

  Whether the CLI can list the trash at all is uncertain (C.5). So a count that differs from these
  numbers, or a trash the CLI cannot list, is recorded with its rows and is not a BLOCK by itself.
  A row naming an untouchable, or any node this gate did not create, is a STOP for Craig.

## Two sessions, relays, the kits, timeouts and BLOCKED

**Two machines, two runners.** The **Mac session** runs Parts A to D. The **PC session** runs
Part W, from its own section, as section 6 says. Both are agent sessions; Craig is in both rooms.
The two share no files.

- **Relays.** A value one session needs from the other is printed on a line that starts
  `RELAY `. Craig copies that line, verbatim, into the other session, whose runner stores it
  before the next step: on the Mac `/bin/bash ~/gpbmac/kit/kit.sh relayin '<the line>'`, on the PC
  `Save-Relay '<the line>'`. The quoted line is the only thing a runner fills in, and only with
  what Craig relays. Scripts and blocks then read it (`kit relaycmp` and `kit getval`;
  `Compare-Relay` and `Get-RelayValue`). A relay line holds only ASCII: ref-set digests, commit
  IDs, uids. A second relay under the same name that differs from the first is BLOCKED by the kit,
  and so is a value relay of a name this gate does not relay, or of the wrong shape: `pcroot` is a
  uid, and `W3` a commit ID and `brief` a blob, both 40 lower-case hex digits (`kit relayin`
  checks it, and `kit getval` again on reading it).
  *Brief author's call:* digests
  rather than whole ref sets, so a relay is one short line, and a mismatch is then diagnosed from
  both sides' logs.
- **Handoffs.** Each Mac step that needs the PC ends with `HANDOFF -> PC: W.<step>`, and each PC
  block that the Mac waits on ends with `HANDOFF -> MAC: <step>`. The runner whose turn it is not
  waits; neither side runs ahead. The table in Part W lists every handoff in order.
- **The same brief on both sides.** Each extractor prints `RELAY value brief <blob>`, the git blob
  of this file as it extracted it; the other side compares it with its own (`kit relaycmp` is not
  needed: the runner compares the two 40-hex values). Different blobs: STOP.

**The Mac shell.** Mac blocks are `bash` and run under `/bin/bash` 3.2, whatever shell the runner
types into. Scripts carry a header line `# file: mac:scripts/<name>.sh` and are never typed: the
extractor writes them from this file (below), so no escape in them can be decoded on the way. The
runner runs one as `/bin/bash ~/gpbmac/kit/gx.sh <name>`, which hands it the current run's name,
shows its output (stderr merged) and appends it to `~/gpbmac/<run>/log/<name>.txt` as it arrives,
with STARTED and EXIT lines in `log/runs.txt`. Inside a script, `logged <name> <command...>` keeps
one command's output in `out/<name>.txt` and prints its escaped view. A block marked **Craig
types** belongs in Craig's own Terminal (zsh).

**The PC shell.** PC blocks are `pwsh` (PowerShell 7.5 or newer) and are typed by the PC runner.
Each starts by dot-sourcing `C:\gpbmac\kit\pc-kit.ps1`. PowerShell variable names ignore case, so
no block assigns a name that folds to one of the kit's (`$Root`, `$MainRepo`, `$Tag`, `$Run`, `$G`,
`$Suf`, `$GR`, `$XM`, `$XW`, `$NFDR`, `$RENR`, `$NFC`, `$NFD`).

**The escaped view.** NFC and NFD spellings display identically, so every check reads output
through one transformation, the same in both kits: each character outside printable ASCII is
written `<U+XXXX>` (four or more upper-case hex digits), a backslash is written `<BS>`, and tab and
printable ASCII stay as they are. A raw NFC `cafe` with its accent therefore reads
`caf<U+00E9>`, a raw NFD one `cafe<U+0301>`, and the helper's own ASCII escape of the NFC name
reads `"heads/caf<BS>u00e9"`. *Brief author's call:* no check ever needs a backslash or a
non-ASCII character typed, so neither can be mangled by a tool on the way into a check.

**Expected-text checks.** `kit expect <log> <label>` (Mac) and `Test-Expect <log> <label>` (PC)
read the entry `[<label>]` of `expected.txt`, in "Strings the helper prints" below. Each line of
an entry must be a substring of one line of the log's escaped view, consecutive entry lines of
consecutive log lines. In an entry, `<OID>` matches any 40 lower-case hex digits and
`<ENV:NAME>` the value of the environment variable NAME, which the script sets first. Each prints
`EXPECT <label>: FOUND at line <n>` or `EXPECT <label>: MISSING`, with the entry. `kit absent` and
`Test-Absent` are the inverse: `ABSENT` passes.

**Timeouts (rule 9).**

- **The harness timeout.** Every `gx.sh` call that runs `git push`, `git clone` or `git fetch`, and
  every PC block that does, runs in the background with a harness timeout of at least 30 minutes;
  the runner waits for its `EXIT` line or exit code. Step 2 and step 6's waits run with at least
  90 minutes. Every other call that reaches the account (a listing, snapshot, Windows check, root
  record, logout or trash) runs with at least 10 minutes: a snapshot alone can wait twice 60 s
  for trash lag. A snapshot the harness kills leaves no file under its label.
- **go test's own deadline.** Step 2's `go test` passes `-timeout 40m` and `-count=1` (rule 5).
- **The Mac stays awake.** Craig holds the keep-awake, `kit/awake.sh`, in a Terminal window of
  his own from the end of C.0 to 7.0 (Question 7): not a runner's background command, which a
  harness kills at its bound. It records its pid and start time in the run folder and then becomes
  `/usr/bin/caffeinate -is -t 172800` under that pid (48 hours, longer than the gate [inference]),
  so the Mac cannot idle-sleep mid-push at any step. `gx.sh` checks that one process by its
  recorded pid (`awake_ok`), never any other `caffeinate` on the Mac, before every gate script from
  C.2 to step 6, and refuses to start one while it is down (exit 3, a STOP: nothing ran; Craig
  restarts it and the script is run); step 0, 6.0 and 6.1 check it again. Step 6 also needs mains
  power, where `-s` takes effect (spec, "Facts": `man caffeinate`) [measured 2026-10-28: a bash
  script that writes `$$` and then `exec`s `caffeinate` keeps that pid, which is the starting
  command's `$!`; `ps -o comm=` prints `/usr/bin/caffeinate` for it, and `pmset -g assertions`
  lists it as `pid <n>(caffeinate)`].
- **No hermetic test run is part of this gate.** From C.3 on, a real, signed-in `proton-drive` is
  on the gate's PATH, so a hermetic `go test` or Pester run on this Mac would be rule 12's case: it
  must not resolve the real CLI, which means a PATH without `~/.local/bin` (rule 12). Step 2 is the
  live contract half under `GPB_LIVE_ACCOUNT=1`, which uses the real CLI on purpose and installs no
  trap [spec section 5]; it is not a hermetic run.

**Nothing else writes to the account or runs the Windows CLI meanwhile.** `ProjectsBackupDaily` and
`ProtonCliDriftCheck` are paused on the PC for the probe and for the whole gate (W.B.1, W.C.1).
Section 6 names only the first; *brief author's call*, the v0.10.0 briefs' reason: two Windows CLI
starts can collide on its SQLite cache (`cli-0.9.0-certification.md` 4.10), and a gate's length
is not known in advance. Re-enabling them is Craig's, only when the PC's unpause condition holds.

**BLOCKED (rule 4).** A step is BLOCKED when any of these happens:

- a pass condition fails, or an `EXPECT` prints `MISSING`;
- an expected line is missing, or differs from the text quoted here;
- a line starting `git-remote-proton:` appears where none is expected, other than the known lines
  below;
- a command is killed.

A script's `EXIT 0` is not a pass by itself: every listed pass condition is checked, by the script
where it says so and otherwise by the runner from the printed lines (counts of `ADDED` and
`CHANGED` rows, say). A push a step needs refused that exits 0 has written, so its script records
the remote and stops at once (`unrefused` in `lib.sh`), before any later write in the step builds on
it; a refusal whose text differs is a failed check (`bad`), and the script goes on to record the
rest of the step's evidence.

On a BLOCK the runner reports the verbatim output (its escaped view where it holds non-ASCII), and
patches nothing: no code, config, account or machine state, no re-run of a push, no new sign-in.
It runs step 7 (cleanup) and then stops for Craig. Under the retry rule there is no second gate
run at this release.

**Known `git-remote-proton:` lines, not a BLOCK by themselves.** *Brief author's call, the v0.10.0
briefs' rule.*

- **A pipe held past exit** (string 17): recorded verbatim, and the step is judged on its other
  evidence.
- **The pair warning** (string 4), wherever a step below expects it. Where a step does not
  expect it, it is BLOCKED.
- **The restore-floor warnings** (string 16) are expected absent: Apple's git 2.54.0 passes the
  floor probe (M11). The first form is BLOCKED. The second is recorded the first time (most likely
  the probe's 10 s timeout), and a second occurrence in the run is BLOCKED: `d5_check` in `lib.sh`
  applies that rule to every push `logged` runs and to 6.1's scheduled run, the v0.10.0 briefs'
  rule.

**Counting them.** A pass condition that says "no `git-remote-proton:` line", or counts them, means
none but these known lines, on the Mac and on the PC. `kit helperlines` lists a log's
`git-remote-proton:` lines, marks string 17's and the "could not check" warning's `KNOWN:`, and
prints `others: <n>`, the count such a condition judges. `d5_check` judges the "could not check"
warning itself, and each step that expects the pair warning judges it there.

**A trashed row that outlives the re-takes** is printed by `kit compare` as `TRASHED, STILL
LISTED`, is not counted as a difference, and is recorded; Craig decides whether the listing is
re-taken under `<label>-r`, which later comparisons then read. This holds only for a node this
gate trashed (its `.lock` files, its deletes' ref files, its roots, the contract table's folders,
and a `GitRemotes` step 2.0 made). Any other trashed row is BLOCKED, and so is a `LAG:` line naming
anything but those. The lag on record is at most 112 s (`stage6-gate.md` 5.4), and `kit snapshot`
re-takes twice, 60 s apart.

**What the terminal shows.** git prints its push and fetch summaries to stderr, and the helper its
`gpb:` and `git-remote-proton:` lines; every block merges stderr. The helper's protocol `ok` and
`error` lines go to git on stdout and are not shown; a per-ref refusal is read from git's
`[remote rejected]` line, which carries the helper's reason in parentheses. git's column padding
varies, so its own words are matched as substrings; the helper's messages are matched exactly
(through `expect`). git prints a ref's short name raw, so a refused NFD create reads
`main -> cafe<U+0301> (...)` in the escaped view.

---

## Strings the helper prints, from the code

Each was copied from the code at the anchor, `0faada8`, and checked against it by grep. The 16
entries of `expected.txt` for strings 1, 4 to 10 and 12 were also compared with the text the code
at the anchor produces for each entry's inputs, through the Mac kit's own `expect` [measured
2026-10-28: a scratch test in a scratch copy of the module at `0faada8`, with the Fake transport,
never committed; the three list refusals composed from `main.go`'s own `fmt.Errorf` format strings
around the lines the repo package builds; all 16 matched, and two controls did not: a wrong object
ID, and string 6 with its pre-MA3 tail]. C.2 checks that no file they come from changed between
the anchor and the tag (STOP for adjudication if one did), so a live string that differs from its
entry by even one character is the product's behaviour: BLOCKED.

**How the helper writes a spelling.** Every conflict a refusal names is written by `formatItem`:
the conflicting ref raw, then its prefix pair raw, the relation, and the pair again in Go's
ASCII-only quoted form, `%+q` (`strconv.QuoteToASCII`) [code `internal/repo/casefold.go:185-193`].
So the raw parts show the accented name as it is, and the quoted parts show it escaped: the NFC
prefix as `"heads/caf\u00e9"` and the NFD one as `"heads/cafe\u0301"`, as M29 measured for
whole names (`"refs/heads/caf\u00e9"`, `"refs/heads/cafe\u0301"`). The two list lines, `PairLines`
and `DestLines`, write each whole ref raw and then quoted [code `internal/repo/readcheck.go:57-68,
453-481`]. In the escaped view the raw NFC name reads `caf<U+00E9>`, the raw NFD name
`cafe<U+0301>`, and their quoted forms `caf<BS>u00e9` and `cafe<BS>u0301`.

**The relation's wording** is `letter case` or `Unicode normalisation` [code
`internal/repo/casefold.go:90-98`]; a pair that matches both is reported as `letter case`
(`casefold.go:73-75`).

**Ordering.** Conflicts are sorted by ref, as bytes [code `casefold.go:178`], and a pair is printed
smaller first [code `readcheck.go:23-55`]. The NFD name sorts before the NFC one (`e`, 0x65, comes
before 0xc3).

**The numbered strings.** The entries of `expected.txt` that each string's steps use are named in
brackets.

1. **The create refusal** [code `internal/repo/casefold.go:200-211`, called from
   `internal/repo/push.go:444`]: `<dst> conflicts with <items>: <harm> (README, Honest limits), so
   this create is refused on every platform`, where `<harm>` is `a Mac clone would mishandle the
   pair` when every item is a normalisation match, and `a Windows or Mac clone would mishandle the
   pair` when any is a case match. [`nfd-create`, `third-spelling`, `case-Main`, `case-Featx`,
   `case-MAIN`]
2. **Phase 5's form of string 1** appends ` (the same-batch delete of <ref> did not commit)` [code
   `casefold.go:219-230`, `push.go:720`]. Not provoked live: it needs a phase-4 delete to fail.
3. **The rename-delete refusal** [code `push.go:480-481`]: `<d>: delete refused because it pairs
   with the refused <case-changing|normalisation-changing> rename to <x> (<%+q d> to <%+q x>); the
   rename was not applied`. Not provoked live (section 6 has no rename push in one batch); hermetic
   tests cover it.
4. **The pair warning**, on every platform, at `list` and `list for-push` [code
   `cmd/git-remote-proton/main.go:563-570`, `internal/repo/readcheck.go:57-81`]: the line
   `git-remote-proton: warning: this remote holds refs that differ only by letter case or Unicode
   normalisation; see README, Honest limits:` and then one line per pair. [`warn-case`,
   `warn-nfd`, `warn-head`]
5. **The Mac refusal at `list`, rule (1)**, a pair on the remote [code
   `cmd/git-remote-proton/main.go:477-481`]. [`fetch-pair-5.5`, `fetch-pair-5.7`]
6. **The Mac refusal at `list`, rule (2)**, a destination aliasing a local ref [code
   `cmd/git-remote-proton/main.go:485-496`]. Since MA3 its last line names a `reftable` clone or
   the remote's fetch refspecs, and sends an in-place repair to the README, warning that a plain
   delete can unlink the other spelling's ref. [`fetch-dest-5.8`]
7. **The Mac refusal at `list for-push`, R1** [code `cmd/git-remote-proton/main.go:510-530`].
   Since MA3 its last line names only a push from a `reftable` clone, with the same pointer to the
   README and the same warning. [`push-r1`]
8. **The Mac push rule, part 1(b)**, the remote half [code `internal/repo/macpush.go:218-228`,
   with `macWriteTail` at `:38-39`]. [`mac-remote-half`]
9. **The Mac push rule, part 1(a)**, the local half [code `macpush.go:191-217, 226-228`].
   [`mac-local-half`]
10. **The Mac push rule, part 3**, a delete [code `macpush.go:290-314`]. [`mac-part3`]
11. **Part 2's paired delete, and R3's create:** two messages. Part 2's [code `macpush.go:58-64`]:
    `<d>: delete refused because it pairs with <x>, which this Mac refused; neither was applied`,
    which shares its first words with string 3 but not its end. R3's [code `macpush.go:66-76`, with
    `macWriteTail` at `:38-39`]: `<x>: refused on this Mac: this push's delete of <k> was refused
    here, and <x> would then conflict with <items>`, then `macWriteTail`, `; this repository's
    default (files) ref store cannot keep these apart (README, Honest limits); repair the remote,
    or push from a --ref-format=reftable clone`; with more than one kept delete, `deletes of <k1>,
    ... and <kn> were`. Not provoked live: no section 6 push pairs a delete with a write the Mac
    rule refuses.
12. **R7's form of part 1(a)** adds ` (the tracking ref of <Y>, which this push deletes)` to an
    item [code `macpush.go:198-211`]. Not provoked live (see "R7 and this gate").
    [`r7-not-provoked`]
13. **The lock refusal** [code `internal/repo/lock.go:79-81`]: `repo is locked by nonce <hex> on
    <host> (pid <n>) since <time>; if that process is gone, remove <root>/.lock with the Proton
    CLI`. `<host>` is Go's `os.Hostname()` [code `lock.go:47`], which on this Mac is what
    `hostname` prints, today `mac.lan` [measured 2026-10-27]; it can follow the network the Mac
    is on [inference]. Not expected anywhere; if met, compare `<host>` with `hostname` and `<pid>`
    with the processes that are running, and STOP for Craig.
14. **The missing-parent refusal** [code `internal/repo/parents.go:102-106`], which step 6's
    failing pushes provoke. [`parent-absent`]
15. **`--version`** [code `cmd/git-remote-proton/main.go:111-113`] and `--set-head`'s report
    `HEAD is now <ref>` [code `main.go:142`]. [`version`, `v010-version`, `set-head`]
16. **The restore-floor warnings** [code `cmd/git-remote-proton/main.go:85-95`], expected absent.
    [`d5-too-old`, `d5-unknown`]
17. **A pipe held past exit** [code `internal/transport/cli.go:67-69`,
    `internal/gitcmd/gitcmd.go:55-56`]: `git-remote-proton: <exe> exited but something still held
    its output pipe open after 30s; ...`. Known, and recorded.
18. **The override warning**, only if the release moved the CLI pin and step 5.5 runs v0.10.0's
    helper with `GPB_UNCERTIFIED_CLI=1` [code `internal/transport/cli.go:685-688`, unchanged since
    v0.10.0]: `git-remote-proton: WARNING: proceeding with an UNCERTIFIED Proton CLI because
    GPB_UNCERTIFIED_CLI=1. Version "<found>"; certified: <token>. Behaviour on this build is
    unvalidated.` Recorded where it applies.
19. **The download trace** [code `internal/transport/trace.go:43`]: `gpb: downloaded <path> (<n>
    bytes)`, including the lock read-back after every lock create.
20. **The CLI's own lines:** `You need to login first` signed out (M6, M8); `Node not found` for an
    absent path [code `internal/transport/cli.go:169`]; and `--version`'s first line, `Proton
    Drive CLI cli-drive@0.9.0+75612299` (M4). [`login-first`, `cli-version`]

**The entries.** Every check reads them from this one block, which both extractors write. Each
entry names its source again. They are in the escaped view: `<U+00E9>` is the raw accented `e`,
`<U+0301>` the raw combining accent, and `<BS>` the backslash of the helper's own escape.

```text
# file: both:kit/expected.txt
# The expected texts of the vNEXT macOS gate, in the escaped view (see the brief, "The escaped
# view"). An entry is a [label] line and the lines after it, up to a blank line. Each line must be
# a substring of one log line, consecutive lines of consecutive log lines. <OID> is any 40 hex
# digits; <ENV:NAME> is the value of environment variable NAME. Lines starting with # are notes.

# String 1, NFD create beside NFC (casefold.go:200-211 via push.go:444). Steps 5.1 and 5.2, Mac and PC.
[nfd-create]
-> cafe<U+0301> (refs/heads/cafe<U+0301> conflicts with refs/heads/caf<U+00E9> (heads/cafe<U+0301> vs heads/caf<U+00E9>, by Unicode normalisation: "heads/cafe<BS>u0301" vs "heads/caf<BS>u00e9"): a Mac clone would mishandle the pair (README, Honest limits), so this create is refused on every platform)

# String 1, a third spelling against both (casefold.go:200-211). Step 5.4.
[third-spelling]
-> Caf<U+00E9> (refs/heads/Caf<U+00E9> conflicts with refs/heads/cafe<U+0301> (heads/Caf<U+00E9> vs heads/cafe<U+0301>, by Unicode normalisation: "heads/Caf<BS>u00e9" vs "heads/cafe<BS>u0301"), refs/heads/caf<U+00E9> (heads/Caf<U+00E9> vs heads/caf<U+00E9>, by letter case: "heads/Caf<BS>u00e9" vs "heads/caf<BS>u00e9"): a Windows or Mac clone would mishandle the pair (README, Honest limits), so this create is refused on every platform)

# String 1, D2's case steps rerun from the Mac (casefold.go:200-211). Step 5.3.
[case-Main]
main -> Main (refs/heads/Main conflicts with refs/heads/main (heads/Main vs heads/main, by letter case: "heads/Main" vs "heads/main"): a Windows or Mac clone would mishandle the pair (README, Honest limits), so this create is refused on every platform)

[case-Featx]
main -> Feat/x (refs/heads/Feat/x conflicts with refs/heads/feat (heads/Feat vs heads/feat, by letter case: "heads/Feat" vs "heads/feat"): a Windows or Mac clone would mishandle the pair (README, Honest limits), so this create is refused on every platform)

[case-MAIN]
main -> MAIN (refs/heads/MAIN conflicts with refs/heads/Main (heads/MAIN vs heads/Main, by letter case: "heads/MAIN" vs "heads/Main"), refs/heads/main (heads/MAIN vs heads/main, by letter case: "heads/MAIN" vs "heads/main"): a Windows or Mac clone would mishandle the pair (README, Honest limits), so this create is refused on every platform)

# String 4, the pair warning (main.go:568, readcheck.go:64-65, 71-72). Its first line alone:
[warn-head]
git-remote-proton: warning: this remote holds refs that differ only by letter case or Unicode normalisation; see README, Honest limits:

[warn-case]
git-remote-proton: warning: this remote holds refs that differ only by letter case or Unicode normalisation; see README, Honest limits:
  refs/heads/Main (<OID>, "refs/heads/Main") and refs/heads/main (<OID>, "refs/heads/main"): by letter case

[warn-nfd]
git-remote-proton: warning: this remote holds refs that differ only by letter case or Unicode normalisation; see README, Honest limits:
  refs/heads/cafe<U+0301> (<OID>, "refs/heads/cafe<BS>u0301") and refs/heads/caf<U+00E9> (<OID>, "refs/heads/caf<BS>u00e9"): by Unicode normalisation

# String 5, rule (1) at list (main.go:477-481, readcheck.go:64-65). Step 5.5: NFD at N2, NFC at NC.
[fetch-pair-5.5]
git-remote-proton: refusing to fetch on this Mac: this repository uses git's default (files) ref store, which cannot keep these refs apart (README, Honest limits):
  refs/heads/cafe<U+0301> (<ENV:N2>, "refs/heads/cafe<BS>u0301") and refs/heads/caf<U+00E9> (<ENV:NC>, "refs/heads/caf<BS>u00e9"): by Unicode normalisation
repair the remote, or clone with --ref-format=reftable

# The same at step 5.7, after the PC moved the NFD member to W3.
[fetch-pair-5.7]
git-remote-proton: refusing to fetch on this Mac: this repository uses git's default (files) ref store, which cannot keep these refs apart (README, Honest limits):
  refs/heads/cafe<U+0301> (<ENV:W3>, "refs/heads/cafe<BS>u0301") and refs/heads/caf<U+00E9> (<ENV:NC>, "refs/heads/caf<BS>u00e9"): by Unicode normalisation
repair the remote, or clone with --ref-format=reftable

# String 6, rule (2) at list (main.go:485-496, readcheck.go:463-481). Step 5.8: the NFD branch at RC1.
[fetch-dest-5.8]
git-remote-proton: refusing to fetch on this Mac: refs this fetch would write or prune collide with each other or with refs this repository holds (README, Honest limits):
  refs/remotes/proton-v2/cafe<U+0301> (from refs/heads/cafe<U+0301>, <ENV:RC1>) and refs/remotes/proton-v2/caf<U+00E9> (local) ("refs/remotes/proton-v2/cafe<BS>u0301" vs "refs/remotes/proton-v2/caf<BS>u00e9"): by Unicode normalisation
use a --ref-format=reftable clone, or change the remote's fetch refspecs; to repair a stale local ref in place, follow the README (a plain delete can unlink the other spelling's ref)

# String 7, R1 at list for-push (main.go:510-530, readcheck.go:463-481). Step 5.11: the NFC branch at Q1.
[push-r1]
git-remote-proton: refusing to push on this Mac: git would rewrite tracking refs that collide with refs this repository holds, even for refs that are already up to date (README, Honest limits):
  refs/remotes/proton-v2/caf<U+00E9> (from refs/heads/caf<U+00E9>, <ENV:Q1>) and refs/remotes/proton-v2/cafe<U+0301> (local) ("refs/remotes/proton-v2/caf<BS>u00e9" vs "refs/remotes/proton-v2/cafe<BS>u0301"): by Unicode normalisation
push from a --ref-format=reftable clone; to repair the stale local ref in place, follow the README (a plain delete can unlink the other spelling's ref)

# String 8, part 1(b) (macpush.go:218-228, 38-39). Step 5.9.1.
[mac-remote-half]
caf<U+00E9> -> caf<U+00E9> (refs/heads/caf<U+00E9>: refused on this Mac: the remote also holds refs/heads/cafe<U+0301> (heads/caf<U+00E9> vs heads/cafe<U+0301>, by Unicode normalisation: "heads/caf<BS>u00e9" vs "heads/cafe<BS>u0301"); this repository's default (files) ref store cannot keep these apart (README, Honest limits); repair the remote, or push from a --ref-format=reftable clone)

# String 10, part 3 (macpush.go:290-314). Step 5.9.2.
[mac-part3]
cafe<U+0301> (refs/heads/cafe<U+0301>: delete refused on this Mac: git would then delete its tracking ref refs/remotes/proton-v2/cafe<U+0301>, which this repository's default (files) ref store keeps under the same file name as refs/remotes/proton-v2/caf<U+00E9> (remotes/proton-v2/cafe<U+0301> vs remotes/proton-v2/caf<U+00E9>, by Unicode normalisation: "remotes/proton-v2/cafe<BS>u0301" vs "remotes/proton-v2/caf<BS>u00e9") (README, Honest limits); delete it from a --ref-format=reftable clone)

# String 9, part 1(a) (macpush.go:191-217, 226-228, 38-39). Step 5.9.3.
[mac-local-half]
main -> cafe<U+0301> (refs/heads/cafe<U+0301>: refused on this Mac: its tracking ref refs/remotes/proton-v2/cafe<U+0301> would collide with refs/remotes/proton-v2/caf<U+00E9> (remotes/proton-v2/cafe<U+0301> vs remotes/proton-v2/caf<U+00E9>, by Unicode normalisation: "remotes/proton-v2/cafe<BS>u0301" vs "remotes/proton-v2/caf<BS>u00e9"); this repository's default (files) ref store cannot keep these apart (README, Honest limits); repair the remote, or push from a --ref-format=reftable clone)

# String 12, R7 (macpush.go:198-228). No step provokes it; for an operator who meets it (an
# update of the NFC member beside a delete of the NFD one, from a files clone holding neither
# tracking ref).
[r7-not-provoked]
(refs/heads/caf<U+00E9>: refused on this Mac: its tracking ref refs/remotes/proton-v2/caf<U+00E9> would collide with refs/remotes/proton-v2/cafe<U+0301> (remotes/proton-v2/caf<U+00E9> vs remotes/proton-v2/cafe<U+0301>, by Unicode normalisation: "remotes/proton-v2/caf<BS>u00e9" vs "remotes/proton-v2/cafe<BS>u0301") (the tracking ref of refs/heads/cafe<U+0301>, which this push deletes); this repository's default (files) ref store cannot keep these apart (README, Honest limits); repair the remote, or push from a --ref-format=reftable clone)

# String 14 (parents.go:102-106), in the launchd agent's error log. Step 6.
[parent-absent]
git-remote-proton: parent folder /my-files/GitRemotes/<ENV:TAG>-mac-absent-<ENV:SUF> does not exist; create it first (proton-drive filesystem create-folder /my-files/GitRemotes <ENV:TAG>-mac-absent-<ENV:SUF>, or the web UI), or set GPB_CREATE_PARENTS=1 to let the helper create missing parents

# String 15 (main.go:111-113, 142).
[version]
git-remote-proton <ENV:TAG> (certified CLI: cli-drive@0.9.0+75612299)

[v010-version]
git-remote-proton v0.10.0 (certified CLI: cli-drive@0.9.0+75612299)

[set-head]
HEAD is now refs/heads/side

# String 16 (main.go:85-95): expected ABSENT from every push.
[d5-too-old]
git-remote-proton: WARNING: this git cannot restore from a git-remote-proton remote

[d5-unknown]
git-remote-proton: WARNING: could not check whether this git can restore

# String 20, the CLI's own lines (M4, M6).
[cli-version]
Proton Drive CLI cli-drive@0.9.0+75612299

[login-first]
You need to login first

# The scheduled-push wrapper's own lines (scripts/macos/gpb-v2-push.sh:110-112, 221, 247, 261).
[wrapper-ok]
gpb-v2-push: git push proton-v2 --all: exit 0
gpb-v2-push: git push proton-v2 --tags: exit 0
gpb-v2-push: done: <ENV:LDREPO> -> proton-v2

[marker-lock]
the push was killed; if the next push reports that this repository is locked by this Mac, that lock is the killed push's: remove it as that message says (README, scheduled use)
```

## The extractors and the kits

**Why extraction by script.** The scripts and the expected texts hold byte escapes and, in the
escaped view, sequences a tool can mangle. A runner who retypes them can change them without
seeing it (the plan's self-review notes, round 3). So each machine's runner saves one short
extractor by hand, and it writes every other `# file:` block from this file as git holds it, byte
for byte, and then checks itself against its own block. *Brief author's call.*

**The rev.** Parts A and B run before the tag exists, from the main checkout's
`refs/remotes/origin/main` after a `git fetch origin` (its commit is recorded); C.0 re-extracts
from the tag `vNEXT`. Every later step uses the tag's extraction. Both sessions relay the blob they
extracted, and the two must be equal.

**The Mac's extractor.** The runner saves this block to `~/gpbmac/extract.py` by hand, the one
block saved that way, then runs it:

```python
# file: mac:extract.py
# The Mac's extractor for the vNEXT macOS gate brief. Usage, from any folder:
#   /usr/bin/python3 -I ~/gpbmac/extract.py <rev> docs/research/gates/vNEXT-macos-gate-brief.md
# It reads the brief at <rev> from the main checkout, writes every fenced block whose first line
# is "# file: mac:<path>" or "# file: both:<path>" to ~/gpbmac/<path>, byte for byte, and then
# checks that the copy of itself it was started from equals its own block.
import hashlib
import os
import re
import subprocess
import sys

MAIN = os.path.expanduser("~/Projects/_Tools/git-proton-backup")
ROOT = os.path.expanduser("~/gpbmac")
HEADER = re.compile(r"^# file: (mac|pc|both):([A-Za-z0-9._/-]+)$")


def git(*args):
    return subprocess.run(["/usr/bin/git", "-C", MAIN] + list(args), check=True,
                          stdout=subprocess.PIPE).stdout


def main():
    if len(sys.argv) != 3:
        sys.exit("usage: extract.py <rev> <brief path>")
    rev, path = sys.argv[1], sys.argv[2]
    with open(os.path.abspath(__file__), "rb") as f:
        own = f.read()
    commit = git("rev-parse", "--verify", rev + "^{commit}").decode().strip()
    blob = git("rev-parse", rev + ":" + path).decode().strip()
    data = git("cat-file", "blob", blob)
    print("brief %s at %s (commit %s): blob %s, %d bytes" % (path, rev, commit, blob, len(data)))
    lines = data.decode("utf-8").split("\n")
    files, i = {}, 0
    while i < len(lines):
        fence = lines[i][:3]
        if fence not in ("`" * 3, "~" * 3):
            i += 1
            continue
        j = i + 1
        while j < len(lines) and not lines[j].startswith(fence):
            j += 1
        body = lines[i + 1:j]
        m = HEADER.match(body[0]) if body else None
        if m and m.group(1) in ("mac", "both"):
            rel = m.group(2)
            if rel in files or rel.startswith("/") or ".." in rel.split("/"):
                sys.exit("BLOCKED: bad or repeated file path in the brief: " + rel)
            files[rel] = ("\n".join(body) + "\n").encode("utf-8")
        i = j + 1
    for rel in sorted(files):
        dest = os.path.join(ROOT, rel)
        os.makedirs(os.path.dirname(dest), exist_ok=True)
        with open(dest + ".partial", "wb") as f:
            f.write(files[rel])
        os.replace(dest + ".partial", dest)
        print("  wrote %-34s %6d bytes  sha256 %s" % (rel, len(files[rel]),
                                                      hashlib.sha256(files[rel]).hexdigest()))
    print("files written: %d" % len(files))
    same = files.get("extract.py") == own
    print("SELF-CHECK: the extractor that ran equals its block in the brief: %s"
          % ("YES" if same else "NO (it has now been overwritten with the brief's; run it again)"))
    print("RELAY value brief %s" % blob)
    sys.exit(0 if same else 1)


main()
```

**Pass:** `SELF-CHECK: ... YES`, exit 0, and a `wrote` line for every Mac file this brief names
(`kit/lib.sh`, `kit/gpbkit.py`, `kit/gx.sh`, `kit/kit.sh`, `kit/start-run.sh`, `kit/awake.sh`,
`kit/tree.awk`, `kit/expected.txt` and every `scripts/<name>.sh`). `NO` means the hand-saved copy
differed; it has been replaced by the brief's,
so the runner runs it again, and the second run must print `YES`.

**The Mac kit.**

```bash
# file: mac:kit/lib.sh
# Sourced first by every Mac gate script:  . "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
# $1 is the run's name, which gx.sh passes. It loads the run's env.sh and defines the names and
# helpers the scripts share. Written for /bin/bash 3.2.
set -u
GM="$HOME/gpbmac"
INHERITED=$(env | grep -E '^(GPB_|PROTON_DRIVE_|GIT_DIR=|GIT_COMMON_DIR=|GIT_OBJECT_DIRECTORY=|GIT_CONFIG)' | cut -d= -f1 | tr '\n' ' ')
RUN="${1:-}"
case "$RUN" in probe-a[1-9]|gate-a1) ;; *) echo "BLOCKED: '$RUN' is not a run name (run scripts through gx.sh)"; exit 2 ;; esac
G="$GM/$RUN"
if [ ! -f "$G/env.sh" ]; then echo "BLOCKED: $G/env.sh is missing (start the run with start-run.sh)"; exit 2; fi
. "$G/env.sh"
# The three spellings, as bytes; never typed, since a typed name precomposes (M13).
NFC=$'caf\xc3\xa9'      # 63 61 66 c3 a9
NFD=$'cafe\xcc\x81'     # 63 61 66 65 cc 81
CAP=$'Caf\xc3\xa9'      # 43 61 66 c3 a9
hexof() { printf '%s' "$1" | /usr/bin/od -An -tx1 | /usr/bin/tr -d ' \n'; }
if [ "$(hexof "$NFC")" != 636166c3a9 ] || [ "$(hexof "$NFD")" != 63616665cc81 ] || [ "$(hexof "$CAP")" != 436166c3a9 ]; then
  echo "BLOCKED: the spellings in lib.sh do not hold their bytes"; exit 2
fi
# M2: the certified macos/arm64 CLI's SHA-512, as the spec records it (M2). Every CLI binary, and
# the drift script's $CertifiedMacArm64Sha512, is compared with it.
M2=3f4125de5e41dcc7f4e0e07e8d60737aa18f35c6ecc1ba3fce47ea97b331857eefaf88e74671135af57122d4cb5439b817ec528f8df383550ac7810aeaf0217b
# C.3's quote of README "Install on macOS (v2)": the SHA-256 of the helper's lines and of the CLI's
# lines, as kit readme reads them from the README at the anchor. C.3 and step 1 run the README's
# lines only when the README at the tag gives these two values.
README_HELPER_SHA=5e9dcd5eb538cd14a3fb007b1887db027a5c06bd440bf706d239bd916d7c7b80
README_CLI_SHA=929dfd5a2fb73e5f1cce65f85f1ba8f21dccdc45cab22a5438e5daa2c80c49f0
fail=0
bad() { echo "CHECK FAILED: $*"; fail=1; }
# unrefused <remote root> <label>: a push the step needs refused exited 0, so it wrote. The remote
# is recorded (kit snapshot <label>-unrefused) and the script ends BLOCKED at once, before any later
# write in it builds on that state (rule 4).
unrefused() { echo "BLOCKED: the push was not refused, so it wrote; the remote is recorded and this script stops"; kit snapshot "$2-unrefused" "$1" -r; exit 1; }
kit() { /usr/bin/python3 -I "$GM/kit/gpbkit.py" "$@"; }
have() { local t r=0; for t in "$@"; do command -v "$t" >/dev/null 2>&1 || { echo "missing tool $t"; r=1; }; done; return $r; }
# gate_identity <repo>: the repository-scoped gate identity, for any repository the gate commits in
# (global configuration is off, so there is no other; never git config --global).
gate_identity() { git -C "$1" config user.name 'gpbmac gate' && git -C "$1" config user.email 'gate@gpbmac.invalid'; }
# gate_repo <dir>: git init -b main, with the gate identity.
gate_repo() { git init -q -b main "$1" && gate_identity "$1"; }
# exact_bytes <repo>: section 6 step 5's rule for every step that gives an NFD name by bytes.
exact_bytes() { git -C "$1" config core.precomposeunicode false; }
# commit_in <repo> <file> <line> <message>: appends a line to a file and commits it (real content).
commit_in() { printf '%s\n' "$3" >> "$1/$2" && git -C "$1" add "$2" && git -C "$1" commit -q -m "gate: $4"; }
# logged <name> <command...>: the command's stdout and stderr go to $G/out/<name>.txt, its escaped
# view is printed, and rc (its exit code) and L (the log) are set. A git push's log then goes
# through d5_check (string 16), so every push the gate makes counts toward its sightings.
logged() {
  L="$G/out/$1.txt"; shift; "$@" >"$L" 2>&1; rc=$?; kit esc "$L"
  case " $* " in *" git push "*|*" git "*" push "*) d5_check "$L" ;; esac
  return 0
}
# refs_of <repo> <file>: every local ref, raw bytes, for a before/after comparison with cmp.
refs_of() { git -C "$1" for-each-ref --format='%(objectname) %(refname)' > "$2"; }
# m7: M7's three default folders, one line each; returns 1 if any exists.
m7() {
  local d r=0
  for d in "$HOME/Library/Application Support/proton-drive-cli" "$HOME/Library/Caches/proton-drive-cli" "$HOME/Library/Logs/proton-drive-cli"; do
    if [ -e "$d" ]; then echo "M7 DEFAULT PRESENT: $d"; r=1; else echo "M7 default absent: $d"; fi
  done
  return $r
}
# kc_item: the CLI's Keychain item (M5), queried without printing anything secret (no -w, no -g).
# Status 0: present; 1: absent, security's item-not-found exit, 44 [measured 2026-10-28, a query
# for a name that does not exist]; 2: the query itself failed (its exit printed), which no caller
# may read as absent.
kc_item() {
  local r; /usr/bin/security find-generic-password -s ch.proton.drive/drive-sdk-cli >/dev/null 2>&1; r=$?
  case "$r" in 0) return 0 ;; 44) return 1 ;; *) echo "KEYCHAIN QUERY FAILED: security exit $r"; return 2 ;; esac
}
# kc_expect present|absent <failure text>: prints the item's state, and calls bad with the text
# when the state is not the expected one; a failed query is a failed check either way.
kc_expect() {
  local k; kc_item; k=$?
  case "$k" in
    0) echo "Keychain item ch.proton.drive/drive-sdk-cli: PRESENT" ;;
    1) echo "Keychain item ch.proton.drive/drive-sdk-cli: absent" ;;
    *) bad "the Keychain query failed, so the item's state is unknown ($2)"; return 2 ;;
  esac
  if [ "$1" = present ] && [ "$k" != 0 ]; then bad "$2"; return 1; fi
  if [ "$1" = absent ] && [ "$k" != 1 ]; then bad "$2"; return 1; fi
  return 0
}
# upload_one <local file> <remote folder> <name>: one CLI upload that never overwrites (-f skip),
# its summary read as the helper reads it, the merged output as JSON (cli.go:58, 393-395). Status
# 0 only for (transferred, skipped, failed) = (1, 0, 0) and exit 0; 3 for (0, 1, 0) and exit 0 (the
# name was taken, so the CLI matched it to an existing one); 1 for anything else.
upload_one() {
  local u s
  proton-drive filesystem upload -f skip --json "$1" "$2" > "$G/out/$3.json" 2>&1; u=$?
  kit esc "$G/out/$3.json"; echo "upload: exit $u"
  /usr/bin/python3 -I -c 'import json,sys; s=json.load(open(sys.argv[1])); t=(s.get("transferredItems"), s.get("skippedItems"), s.get("failedItems")); print("upload counts (transferred, skipped, failed):", t); sys.exit(0 if t == (1, 0, 0) else 3 if t == (0, 1, 0) else 1)' "$G/out/$3.json"; s=$?
  [ "$u" = 0 ] || return 1
  return "$s"
}
# d5_check <log>: string 16 in a push's output. The first form (this git cannot restore) is a
# failed check; the second (could not check) is recorded the first time in the run and a failed
# check the second time, the v0.10.0 briefs' rule. Each sighting is noted in $G/d5-unknown.txt.
d5_check() {
  kit absent "$1" d5-too-old || bad "the restore-floor warning that this git cannot restore (string 16)"
  if kit absent "$1" d5-unknown >/dev/null; then echo "ABSENT d5-unknown: YES"; return 0; fi
  if [ -s "$G/d5-unknown.txt" ]; then bad "a second 'could not check' restore-floor warning in this run (string 16)"
  else echo "RECORDED: a 'could not check' restore-floor warning (string 16), the first in this run"; fi
  printf '%s\n' "$1" >> "$G/d5-unknown.txt"
}
# v010 <command...>: one command with v0.10.0's helper first on PATH (step 5.5's commands alone),
# and GPB_UNCERTIFIED_CLI=1 only when C.2 found the CLI pin moved since v0.10.0.
v010() {
  if [ "$(cat "$G/v010-needs-uncertified" 2>/dev/null)" = yes ]; then PATH="$GM/v010-build/bin:$PATH" GPB_UNCERTIFIED_CLI=1 "$@"
  else PATH="$GM/v010-build/bin:$PATH" "$@"; fi
}
# locked: whether the console session's screen is locked, as macOS reports it (step 6).
locked() { /usr/sbin/ioreg -n Root -d1 -a | /usr/bin/plutil -extract IOConsoleUsers.0.CGSSessionScreenIsLocked raw -o - - 2>/dev/null || echo "no (key absent)"; }
# fire <agent> <seconds> [linger]: step 6's firing of one gate agent. Moves its old logs aside,
# bootstraps its plist, waits for its real StartInterval firing (at most 600 s: two intervals),
# then for its run to end (at most <seconds>), waits for launchd to report it not running, waits
# <linger> seconds more if given (6.2: a notification the run started is not cut short), prints
# launchd's state, runs and last exit, and boots it out. The interval counts from the bootstrap,
# not from the run's end (launchd.plist(5): "every N seconds"), so a run ending near a boundary
# can be followed by a second firing before the bootout. `runs =` is printed before the bootout,
# so it can miss a firing that starts between the print and the bootout; the FIRINGS line, counted
# from the out log after the bootout, shows every firing that started and every one that ended.
# Prints both logs in the escaped view. Returns 1 when it did not fire, or did not end, in time, or
# when a firing started and did not end before the bootout (the bootout cut it).
fire() {
  local n="$1" limit="$2" linger="${3:-0}" dom lbl o e t0 t1 k ns ne
  dom="gui/$(id -u)"; lbl="gpbmac.$RUN.$n"; o="$G/launchd/logs/$n.out.log"; e="$G/launchd/logs/$n.err.log"
  [ -f "$G/launchd/$n.plist" ] || { echo "BLOCKED: no plist $G/launchd/$n.plist"; return 1; }
  for k in 1 2 3 4 5 6 7 8 9; do [ -e "$o.$k" ] || break; done
  [ -e "$o" ] && mv "$o" "$o.$k"; [ -e "$e" ] && mv "$e" "$e.$k"
  : > "$G/launchd/$n.stamp"; t0=$(/bin/date +%s)
  echo "screen locked at the bootstrap: $(locked)"
  launchctl bootstrap "$dom" "$G/launchd/$n.plist" || { echo "BLOCKED: launchctl bootstrap of $lbl failed"; return 1; }
  echo "bootstrapped $lbl at $(/bin/date -u '+%H:%M:%SZ'); waiting for its StartInterval firing (at most 600 s)"
  until grep -q 'gpb-v2-push: start:' "$o" 2>/dev/null; do
    if [ $(( $(/bin/date +%s) - t0 )) -ge 600 ]; then
      echo "NOT FIRED within two intervals (600 s)"; launchctl print "$dom/$lbl" 2>&1 | grep -E 'state =|runs =|last exit'
      launchctl bootout "$dom/$lbl"; return 1
    fi
    sleep 10
  done
  t1=$(/bin/date +%s); echo "FIRED $(( t1 - t0 )) s after the bootstrap; screen locked at the firing: $(locked)"
  until grep -q -E 'gpb-v2-push: (done: |git push [^ ]+ --tags: exit )' "$o"; do
    if [ $(( $(/bin/date +%s) - t1 )) -ge "$limit" ]; then
      echo "NOT FINISHED within $limit s"; launchctl print "$dom/$lbl" 2>&1 | grep -E 'state =|runs =|last exit'
      launchctl bootout "$dom/$lbl"; return 1
    fi
    sleep 5
  done
  for k in 1 2 3 4 5 6 7 8 9 10 11 12; do launchctl print "$dom/$lbl" 2>/dev/null | grep -q 'state = running' || break; sleep 5; done
  if [ "$linger" -gt 0 ]; then echo "the run has ended; waiting $linger s before the bootout"; sleep "$linger"; fi
  launchctl print "$dom/$lbl" 2>&1 | grep -E 'state =|runs =|last exit'
  launchctl bootout "$dom/$lbl"; echo "bootout $lbl: exit $?"
  sleep 2
  ns=$(grep -c 'gpb-v2-push: start:' "$o"); ne=$(grep -c -E 'gpb-v2-push: (git push [^ ]+ --tags: exit |could not create a temporary directory)' "$o")
  echo "FIRINGS in $n.out.log, counted after the bootout: $ns started, $ne ended"
  echo "== $n.out.log"; kit esc "$o"; echo "== $n.err.log"; kit esc "$e"
  if [ "$ns" != "$ne" ]; then echo "BLOCKED: a firing started and did not end before the bootout, which cut it"; return 1; fi
  return 0
}
# envprobe <dir>: what launchd's own environment hands an agent of Craig's GUI domain (B.0, 6.0).
# A probe agent, gpbmac.<run>.envprobe, with the gate plists' EnvironmentVariables (their
# PROTON_DRIVE_CACHE_DIR alone) and no account access, runs once at its bootstrap (RunAtLoad):
# bash's compgen -e, which lists exported names and never a value, keeps the GPB_, PROTON_DRIVE_
# and GIT_ ones, then prints its HOME and PROTON_DRIVE_CACHE_DIR, both paths. envprobe waits at
# most 60 s for it, boots it out and prints what it saw. Status 0 only when those names are
# PROTON_DRIVE_CACHE_DIR alone, with the plist's value, and HOME is this runner's; 1 when not; 2
# when the probe could not run. An earlier log in <dir> is moved aside first.
envprobe() {
  local d="$1" dom lbl o k names r=0
  dom="gui/$(id -u)"; lbl="gpbmac.$RUN.envprobe"; o="$d/envprobe.out.log"
  if launchctl print "$dom/$lbl" >/dev/null 2>&1; then echo "BLOCKED: $lbl is already loaded"; return 2; fi
  mkdir -p "$d" || return 2
  for k in 1 2 3 4 5 6 7 8 9; do [ -e "$o.$k" ] || break; done
  [ -e "$o" ] && mv "$o" "$o.$k"
  printf '%s\n' '<?xml version="1.0" encoding="UTF-8"?>' \
    '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">' \
    '<plist version="1.0">' '<dict>' '    <key>Label</key>' "    <string>$lbl</string>" \
    '    <key>ProgramArguments</key>' '    <array>' '        <string>/bin/bash</string>' '        <string>-c</string>' \
    '        <string>compgen -e | /usr/bin/grep -E "^(GPB_|PROTON_DRIVE_|GIT_)" | /usr/bin/sort; echo "HOME=$HOME"; echo "PROTON_DRIVE_CACHE_DIR=${PROTON_DRIVE_CACHE_DIR-}"; echo END</string>' \
    '    </array>' '    <key>RunAtLoad</key>' '    <true/>' \
    '    <key>StandardOutPath</key>' "    <string>$o</string>" '    <key>StandardErrorPath</key>' "    <string>$o</string>" \
    '    <key>EnvironmentVariables</key>' '    <dict>' '        <key>PROTON_DRIVE_CACHE_DIR</key>' \
    "        <string>$PROTON_DRIVE_CACHE_DIR</string>" '    </dict>' '</dict>' '</plist>' > "$d/envprobe.plist" || return 2
  plutil -lint "$d/envprobe.plist" || return 2
  launchctl bootstrap "$dom" "$d/envprobe.plist" || { echo "BLOCKED: launchctl bootstrap of $lbl failed"; return 2; }
  for k in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30; do grep -qx END "$o" 2>/dev/null && break; sleep 2; done
  launchctl bootout "$dom/$lbl"; echo "bootout $lbl: exit $?"
  grep -qx END "$o" 2>/dev/null || { echo "BLOCKED: the probe agent wrote no END line within 60 s"; return 2; }
  names=$(grep -E '^(GPB_|PROTON_DRIVE_|GIT_)[A-Za-z0-9_]*$' "$o" | tr '\n' ' ' | sed 's/ $//')
  echo "launchd's job environment, its GPB_, PROTON_DRIVE_ and GIT_ names (names only): ${names:-none}"
  echo "its HOME: [$(grep '^HOME=' "$o" | cut -d= -f2-)]; this runner's: [$HOME]"
  echo "its PROTON_DRIVE_CACHE_DIR: [$(grep '^PROTON_DRIVE_CACHE_DIR=' "$o" | cut -d= -f2-)]; the plist's: [$PROTON_DRIVE_CACHE_DIR]"
  [ "$names" = PROTON_DRIVE_CACHE_DIR ] || { echo "NOT CLEAN: a name other than the plist's PROTON_DRIVE_CACHE_DIR reaches the agent from launchd"; r=1; }
  grep -qxF "HOME=$HOME" "$o" || { echo "NOT CLEAN: the agent's HOME is not this runner's"; r=1; }
  grep -qxF "PROTON_DRIVE_CACHE_DIR=$PROTON_DRIVE_CACHE_DIR" "$o" || { echo "NOT CLEAN: the agent's PROTON_DRIVE_CACHE_DIR is not the plist's"; r=1; }
  if [ "$r" = 0 ]; then echo "LAUNCHD ENVIRONMENT: CLEAN"; else echo "LAUNCHD ENVIRONMENT: NOT CLEAN"; fi
  return "$r"
}
# awake_ok: the keep-awake Craig started at C.0 (kit/awake.sh) still runs, checked by the pid it
# recorded alone, never by name, so no other caffeinate on the Mac counts. Sets AWAKE_PID.
awake_ok() {
  AWAKE_PID=$(/bin/cat "$G/caffeinate.pid" 2>/dev/null)
  case "$AWAKE_PID" in ''|*[!0-9]*) echo "keep-awake: no pid recorded in $G/caffeinate.pid"; return 1 ;; esac
  local c; c=$(/bin/ps -p "$AWAKE_PID" -o comm= 2>/dev/null)
  if [ "$c" = /usr/bin/caffeinate ]; then echo "keep-awake: pid $AWAKE_PID, /usr/bin/caffeinate, running since $(/bin/cat "$G/caffeinate.started" 2>/dev/null)"; return 0; fi
  echo "keep-awake: pid $AWAKE_PID is not running caffeinate now: [$c]"; return 1
}
# strays <ERE>: for each process whose command line matches, its pid and program path only (never
# its arguments); status 0 when one is printed, 1 when none.
strays() {
  local p; p=$(/usr/bin/pgrep -f "$1" | /usr/bin/tr '\n' ',' | /usr/bin/sed 's/,$//')
  [ -n "$p" ] || return 1
  /bin/ps -o pid=,comm= -p "$p"
}
cd "$G" || exit 2
```

```bash
# file: mac:kit/gx.sh
# gx.sh <script> [names...]: runs ~/gpbmac/scripts/<script>.sh for the current run
# (~/gpbmac/run.txt), handing it the run's name and then any further arguments, which must be plain
# names. Its output, stderr merged, is shown and appended to <run>/log/<script>.txt as it arrives,
# with STARTED and EXIT lines in <run>/log/runs.txt. For the gate run, a script from C.2 to step 6
# is refused (exit 3, a STOP) while the keep-awake is not running. Usage:
#   /bin/bash ~/gpbmac/kit/gx.sh <script>
set -u
GM="$HOME/gpbmac"
n="${1:-}"
case "$n" in ''|*[!A-Za-z0-9-]*) echo "gx: '$n' is not a script name"; exit 2 ;; esac
shift
for a in "$@"; do case "$a" in ''|*[!A-Za-z0-9.-]*) echo "gx: argument '$a' is not a plain name"; exit 2 ;; esac; done
s="$GM/scripts/$n.sh"
if [ ! -f "$s" ]; then echo "gx: no script at $s (nothing ran: extract the brief first)"; exit 2; fi
RUN=$(/bin/cat "$GM/run.txt" 2>/dev/null) || { echo "BLOCKED: no current run ($GM/run.txt)"; exit 2; }
G="$GM/$RUN"
if [ ! -d "$G/log" ]; then echo "BLOCKED: no run folder $G/log"; exit 2; fi
t() { /bin/date -u '+%Y-%m-%dT%H:%M:%SZ'; }
# The gate run's scripts from C.2 to step 6 start only while Craig's keep-awake runs; step 7's
# never wait on it, since cleanup runs on every exit path.
case "$RUN:$n" in
  gate-a1:c[2-5]*|gate-a1:s[0-6]*)
    if ! /bin/bash -c '. "$1/kit/lib.sh" "$2" >/dev/null && awake_ok' gx "$GM" "$RUN"; then
      echo "STOPPED $n at $(t): the keep-awake is not running" >> "$G/log/runs.txt"
      echo "STOP: the keep-awake is not running. Craig restarts it in his own window (/bin/bash ~/gpbmac/kit/awake.sh gate-a1), then this script is run; nothing ran."
      exit 3
    fi ;;
esac
echo "STARTED $n${*:+ $*} at $(t)" >> "$G/log/runs.txt"
echo "=== $n${*:+ $*}, run $RUN, started $(t)" >> "$G/log/$n.txt"
/bin/bash "$s" "$RUN" "$@" 2>&1 | /usr/bin/tee -a "$G/log/$n.txt"
rc=${PIPESTATUS[0]}
echo "EXIT $n = $rc at $(t)" >> "$G/log/runs.txt"
echo "EXIT $n = $rc"
exit "$rc"
```

```bash
# file: mac:kit/kit.sh
# kit.sh <command> [args]: gpbkit.py for the current run, from the runner's own shell, for the
# commands a runner types (relayin, snapshot re-takes). Usage:
#   /bin/bash ~/gpbmac/kit/kit.sh relayin 'RELAY value pcroot <uid>'
set -u
GM="$HOME/gpbmac"
RUN=$(/bin/cat "$GM/run.txt" 2>/dev/null) || { echo "BLOCKED: no current run ($GM/run.txt)"; exit 2; }
. "$GM/$RUN/env.sh" || exit 2
exec /usr/bin/python3 -I "$GM/kit/gpbkit.py" "$@"
```

```bash
# file: mac:kit/start-run.sh
# start-run.sh probe <attempt> | gate 1: makes ~/gpbmac/<run>/ with its folders and env.sh, and
# names it in ~/gpbmac/run.txt. A run folder that exists already is a stale attempt: BLOCKED, and
# nothing is written. The gate has one run (the retry rule), so only "gate 1" is accepted.
set -u
GM="$HOME/gpbmac"
TAG='vNEXT'
kind="${1:-}"; att="${2:-}"
case "$kind $att" in "probe "[1-9]|"gate 1") ;; *) echo "start-run: probe 1-9, or gate 1"; exit 2 ;; esac
RUN="$kind-a$att"; SUF="a$att"; G="$GM/$RUN"; R='/my-files/GitRemotes'
if [ -e "$G" ]; then echo "BLOCKED: $G already exists (that run already started)"; exit 1; fi
mkdir -p "$G/log" "$G/out" "$G/snap" "$G/refsets" "$G/relay" "$G/cache" || exit 1
: > "$G/gitconfig-global" || exit 1
if [ "$kind" = probe ]; then P='/usr/bin:/bin:/usr/sbin:/sbin'; else P="$HOME/.local/bin:/usr/bin:/bin:/usr/sbin:/sbin"; fi
{
  echo "# gpbmac run environment for $RUN: sourced by every script (through lib.sh) and by Craig's Terminal."
  echo "export RUN='$RUN' SUF='$SUF' TAG='$TAG' G='$G' R='$R'"
  echo "export SMOKE='$R/$TAG-mac-smoke-$SUF' XM='$R/$TAG-mac-xm-$SUF' XW='$R/$TAG-mac-xw-$SUF'"
  echo "export CASER='$R/$TAG-mac-case-$SUF' NFDR='$R/$TAG-mac-nfd-$SUF' RENR='$R/$TAG-mac-ren-$SUF'"
  echo "export MP1R='$R/$TAG-mac-mp1-$SUF' MP3R='$R/$TAG-mac-mp3-$SUF' R1R='$R/$TAG-mac-r1-$SUF'"
  echo "export LDR='$R/$TAG-mac-launchd-$SUF' ABSENT='$R/$TAG-mac-absent-$SUF'"
  echo "export CONTRACT='/my-files/_cas-probe/contract-mac' MAIN='$HOME/Projects/_Tools/git-proton-backup'"
  echo "export SRC='$G/git-proton-backup-$TAG'"
  echo "export PATH='$P'"
  echo "export PROTON_DRIVE_CACHE_DIR='$G/cache'"
  echo "export GIT_CONFIG_GLOBAL='$G/gitconfig-global' GIT_CONFIG_NOSYSTEM=1"
  if [ "$kind" = probe ]; then echo "export PD='$GM/cli/proton-drive'"; fi
  echo "unset PROTON_DRIVE_CREDENTIALS_STORE"
} > "$G/env.sh" || exit 1
printf '%s\n' "$RUN" > "$GM/run.txt" || exit 1
echo "RUN STARTED: $RUN ($G) at $(/bin/date -u '+%Y-%m-%dT%H:%M:%SZ')"
```

The keep-awake, which **Craig types** at the end of C.0 in a Terminal window of his own and leaves
open until 7.0, when he presses Ctrl-C in it (Question 7):

```bash
# file: mac:kit/awake.sh
# awake.sh gate-a1: the gate's keep-awake, from the end of C.0 to 7.0, run by Craig in a Terminal
# window of his own (never a runner's background command, which a harness kills at its bound). It
# records its own pid and start time in the run folder, then becomes /usr/bin/caffeinate -is
# -t 172800 under that pid (exec): 48 hours, longer than the gate [inference]; Ctrl-C ends it.
# lib.sh's awake_ok checks that one process by that pid, never any other caffeinate on the Mac.
# Usage:  /bin/bash ~/gpbmac/kit/awake.sh gate-a1
set -u
case "${1:-}" in gate-a1) ;; *) echo "awake: gate-a1 only"; exit 2 ;; esac
G="$HOME/gpbmac/$1"
[ -d "$G/log" ] || { echo "BLOCKED: no run folder $G (start-run.sh first)"; exit 1; }
old=$(/bin/cat "$G/caffeinate.pid" 2>/dev/null)
if [ -n "$old" ] && [ "$(/bin/ps -p "$old" -o comm= 2>/dev/null)" = /usr/bin/caffeinate ]; then
  echo "BLOCKED: the keep-awake already runs, pid $old"; exit 1
fi
[ -z "$old" ] || echo "keep-awake: the earlier pid $old no longer runs caffeinate (recorded)"
echo "$$" > "$G/caffeinate.pid" || exit 1
/bin/date -u '+%Y-%m-%dT%H:%M:%SZ' > "$G/caffeinate.started" || exit 1
echo "keep-awake: pid $$, /usr/bin/caffeinate -is -t 172800, from $(/bin/cat "$G/caffeinate.started"); leave this window open until 7.0"
exec /usr/bin/caffeinate -is -t 172800
```

Step 6.3's sampler filter, which the sampler runs on each `ps` snapshot:

```awk
# file: mac:kit/tree.awk
# Step 6.3's sampler filter. Input: ps -axo pid=,pgid=,ppid=,command=. It prints only the lines of
# the gate wrapper's process trees: each process run by bash or sh whose command line holds the
# tag's wrapper path (from the environment, TREE_WRAP, so that no awk command line holds it), every
# process descended from one by parent, and every process
# in a process group that one of those leads, or led in an earlier sample of the same firing
# (-v known="<pgids>"), so a process whose parent has died is still followed. The groups that tree
# members lead are written to the file -v pgout names, for the next sample. No other process's line
# is ever printed.
BEGIN {
  wrap = ENVIRON["TREE_WRAP"]
  if (wrap == "") { print "tree.awk: TREE_WRAP is not set" > "/dev/stderr"; bad = 1; exit 2 }
  nk = split(known, k, " "); for (i = 1; i <= nk; i++) grp[k[i]] = 1
}
{
  n++; pid[n] = $1; pg[n] = $2; pp[n] = $3; line[n] = $0
  if (index($0, wrap) > 0 && $4 ~ /(^|\/)(ba)?sh$/) keep[$1] = 1
}
END {
  if (bad) exit 2
  ch = 1
  while (ch) {
    ch = 0
    for (i = 1; i <= n; i++) {
      if (pid[i] in keep) {
        if (pg[i] == pid[i] && !(pg[i] in grp)) { grp[pg[i]] = 1; ch = 1 }
      } else if ((pp[i] in keep) || (pg[i] in grp)) { keep[pid[i]] = 1; ch = 1 }
    }
  }
  for (i = 1; i <= n; i++) if (pid[i] in keep) { print line[i]; if (pg[i] == pid[i]) print pg[i] > pgout }
}
```

*Brief author's call:* the sampler follows the push trees rather than every `git`, helper or CLI
process on the Mac, so no other process's command line, which can carry a secret, reaches the run
folder, and another git command, Craig's own scheduled wrapper (another path) or his editor is
never recorded [measured 2026-10-28, macOS's `/usr/bin/awk` and `/bin/bash` 3.2, against a
stand-in wrapper that starts a push-like job and a watchdog-like subshell under `set -m`, as the
wrapper does: the stand-in, its two jobs and the job's child printed, an unrelated process named
`git` not; with the stand-in killed, its orphans still printed through their remembered groups].
A process that is started and then loses its parent within one 0.2 s interval, in a group none of
the tree leads, is missed [inference].

*Brief author's call: git's system and global configuration are switched off for every gate
script* (`GIT_CONFIG_GLOBAL` names an empty file of the run's, and `GIT_CONFIG_NOSYSTEM=1`), as
the spec's probes ran ("with system and global git config disabled", M12 to M17). Craig's global
`core.hooksPath` would otherwise run his hooks on every gate commit and push, and a later change
to his config could change what a refusal sees. Step 6's launchd runs are the exception: they get
the README plist's environment, so his configuration applies there as in real use.

```python
# file: mac:kit/gpbkit.py
# The Mac session's kit for the vNEXT macOS gate:  /usr/bin/python3 -I ~/gpbmac/kit/gpbkit.py <command>
# (lib.sh's `kit`). The run's values come from the environment the run's env.sh exports. Its
# account calls are `filesystem list` and `filesystem info`, both read-only, and `filesystem
# trash` in `trashroot` and `trashgr` alone (step 7.1); all go through $PD when it is set (the
# probe) and otherwise the proton-drive on PATH. Written for the Python 3.9 Apple ships.
#
# The escaped view: every character outside printable ASCII and tab is written <U+XXXX>, and a
# backslash <BS>, so an NFC and an NFD spelling read differently and no check types a backslash.
import hashlib
import json
import os
import re
import subprocess
import sys
import time

KIT = os.path.join(os.path.expanduser("~"), "gpbmac", "kit")
LAG_WAIT = 60      # seconds before each re-take of a listing that shows trash lag
LAG_RETAKES = 2    # stage6-gate.md 5.4 saw a folder's lag at 21 s and none by 112 s
FIELDS = ("uid", "name", "type", "creationTime", "modificationTime", "parentUid")
ROLES = ("smoke", "xm", "xw", "case", "nfd", "ren", "mp1", "mp3", "r1", "launchd")
CONTRACT = "/my-files/_cas-probe/contract-mac"
REPO_OK = [r"gpb-remote\.json", r"HEAD", r"\.lock", r"refs", r"refs/heads", r"refs/tags",
           r"refs/heads/.+", r"refs/tags/.+", r"packs", r"packs/pack-[0-9a-f]+\.(pack|idx)"]


class Blocked(Exception):
    pass


def env(name):
    v = os.environ.get(name, "")
    if not v:
        raise Blocked("%s is not set (source the run's env.sh first)" % name)
    return v


def esc(s):
    out = []
    for ch in s:
        o = ord(ch)
        if o == 92:
            out.append("<BS>")
        elif o == 9 or 32 <= o <= 126:
            out.append(ch)
        else:
            out.append("<U+%04X>" % o)
    return "".join(out)


def read_lines(path):
    with open(path, "rb") as f:
        text = f.read().decode("utf-8", "surrogateescape")
    lines = text.split("\n")
    if lines and lines[-1] == "":
        lines.pop()
    return [l[:-1] if l.endswith("\r") else l for l in lines]


def rdir(sub):
    d = os.path.join(env("G"), sub)
    os.makedirs(d, exist_ok=True)
    return d


def now():
    return time.strftime("%Y-%m-%dT%H:%M:%S%z")


def cli(args):
    try:
        p = subprocess.run([os.environ.get("PD") or "proton-drive"] + args,
                           stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    except FileNotFoundError:
        raise Blocked("no proton-drive to run (%s): no CLI is installed" % (os.environ.get("PD") or "proton-drive on PATH"))
    return (p.returncode, p.stdout.decode("utf-8", "surrogateescape"),
            p.stderr.decode("utf-8", "surrogateescape"))


def rows(path):
    code, out, err = cli(["filesystem", "list", path, "--json"])
    if code != 0:
        raise Blocked("filesystem list %s exited %d: %s" % (esc(path), code, esc((out + err).strip())))
    res = []
    for i in json.loads(out) or []:
        if i is None:
            continue
        name = i["name"].get("value") if isinstance(i.get("name"), dict) else None
        if not i.get("uid") or not name or not i.get("modificationTime"):
            raise Blocked("filesystem list %s: a row lacks uid, name or modificationTime: %s"
                          % (esc(path), esc(json.dumps(i))))
        res.append({"path": path.rstrip("/") + "/" + name, "name": name,
                    "type": str(i.get("type") or ""), "uid": str(i["uid"]),
                    "parentUid": str(i.get("parentUid") or ""),
                    "creationTime": str(i.get("creationTime") or ""),
                    "modificationTime": str(i["modificationTime"]),
                    "trashTime": str(i.get("trashTime") or "")})
    return res


def subtree(path):
    out = []
    for r in rows(path):
        out.append(r)
        if r["type"] == "folder":
            out.extend(subtree(r["path"]))
    return out


def lagging(r):
    return bool(r["trashTime"]) and not r["path"].startswith("/trash/")


def live_named(parent, name):
    return [r for r in rows(parent) if r["name"] == name and not r["trashTime"]]


# ---- listings and row sets (rule 1)
def c_names(path):
    print(" ".join(esc(n) for n in sorted(r["name"] for r in rows(path) if not r["trashTime"])))
    return 0


def c_info(path):
    code, out, err = cli(["filesystem", "info", path, "--json"])
    if code == 0:
        j = json.loads(out)
        print("INFO %s: exit 0, type %s, uid %s" % (esc(path), j.get("type"), j.get("uid")))
    else:
        nf = "Node not found" in out + err
        print("INFO %s: exit %d%s: %s" % (esc(path), code, " (NOT FOUND)" if nf else "",
                                          esc((out + err).strip())))
    return code


def c_absentnode(path):
    code, out, err = cli(["filesystem", "info", path, "--json"])
    if code != 0 and "Node not found" in out + err:
        print("ABSENT NODE: %s (Node not found)" % esc(path))
        return 0
    print("NOT ABSENT: %s: exit %d: %s" % (esc(path), code, esc((out + err).strip())))
    return 1


def c_snapshot(label, path, recurse):
    f = os.path.join(rdir("snap"), label + ".json")
    if os.path.exists(f):
        raise Blocked("snapshot %s already exists in run %s (a label is used once)" % (label, env("RUN")))
    take = (lambda: subtree(path)) if recurse else (lambda: rows(path))
    rs = take()
    for n in range(1, LAG_RETAKES + 1):
        lag = [r for r in rs if lagging(r)]
        if not lag:
            break
        print("LAG: %d live row(s) carry a trashTime (%s); waiting %d s, re-take %d of %d of %s"
              % (len(lag), ", ".join("%s @ %s" % (esc(r["path"]), r["trashTime"]) for r in lag),
                 LAG_WAIT, n, LAG_RETAKES, label), flush=True)
        time.sleep(LAG_WAIT)
        rs = take()
    with open(f + ".partial", "w") as h:
        json.dump(rs, h, ensure_ascii=True, indent=1)
    os.replace(f + ".partial", f)
    print("SNAPSHOT %s = %s%s: %d row(s)" % (label, esc(path), " (whole subtree)" if recurse else "", len(rs)))
    for r in rs:
        print("  %-6s %s  uid %s  mod %s%s" % (r["type"], esc(r["path"]), r["uid"], r["modificationTime"],
                                              ("  trash " + r["trashTime"]) if r["trashTime"] else ""))
    return 0


def load_snap(label):
    d = rdir("snap")
    retake = os.path.join(d, label + "-r.json")
    p = retake if os.path.exists(retake) else os.path.join(d, label + ".json")
    if not os.path.exists(p):
        raise Blocked("no snapshot named %s" % label)
    if p == retake:
        print("  USING RE-TAKE: %s-r for %s" % (label, label))
    with open(p) as h:
        return json.load(h)


def c_compare(before, after, want_none):
    key = lambda r: "\t".join(str(r[k]) for k in FIELDS)
    a = [r for r in load_snap(before) if not lagging(r)]
    b = load_snap(after)
    ka, kb = set(map(key, a)), set(map(key, b))
    gone = [r for r in a if key(r) not in kb]
    new = [r for r in b if key(r) not in ka]
    lines = []
    for n in new:
        if lagging(n):
            continue
        o = [r for r in gone if r["uid"] == n["uid"]]
        if len(o) == 1:
            d = ["%s '%s' -> '%s'" % (k, esc(str(o[0][k])), esc(str(n[k]))) for k in FIELDS if str(o[0][k]) != str(n[k])]
            lines.append("  CHANGED %s: %s" % (esc(n["path"]), "; ".join(d)))
        else:
            lines.append("  ADDED   %s (%s, uid %s)" % (esc(n["path"]), n["type"], n["uid"]))
    for o in gone:
        if not [r for r in new if r["uid"] == o["uid"]]:
            lines.append("  REMOVED %s (%s, uid %s)" % (esc(o["path"]), o["type"], o["uid"]))
    held = ["  TRASHED, STILL LISTED: %s (%s)" % (esc(r["path"]), r["trashTime"]) for r in b if lagging(r)]
    note = ("; %d trashed row(s) still listed" % len(held)) if held else ""
    if not lines:
        print("COMPARE %s -> %s: NO DIFFERENCE (%d row(s))%s" % (before, after, len(a), note))
    else:
        print("COMPARE %s -> %s: %d difference(s)%s" % (before, after, len(lines), note))
        print("\n".join(lines))
    if held:
        print("\n".join(held))
    return 1 if (want_none and lines) else 0


# ---- the escaped view and expected texts
def c_esc(path):
    for l in read_lines(path):
        print(esc(l))
    return 0


def load_expected():
    entries, cur = {}, None
    for raw in read_lines(os.path.join(KIT, "expected.txt")):
        if raw.startswith("#"):
            continue
        m = re.match(r"^\[([A-Za-z0-9.+-]+)\]$", raw)
        if m:
            if m.group(1) in entries:
                raise Blocked("expected.txt repeats [%s]" % m.group(1))
            cur = entries[m.group(1)] = []
        elif raw == "":
            cur = None
        elif cur is None:
            raise Blocked("expected.txt holds a line outside an entry: " + raw)
        else:
            cur.append(raw)
    return entries


def pattern(line):
    out, pos = [], 0
    for m in re.finditer(r"<OID>|<ENV:([A-Za-z0-9_]+)>", line):
        out.append(re.escape(line[pos:m.start()]))
        out.append("[0-9a-f]{40}" if m.group(0) == "<OID>" else re.escape(env(m.group(1))))
        pos = m.end()
    out.append(re.escape(line[pos:]))
    return re.compile("".join(out))


def find(path, label):
    e = load_expected().get(label)
    if not e:
        raise Blocked("expected.txt has no entry [%s]" % label)
    pats = [pattern(x) for x in e]
    lines = [esc(l) for l in read_lines(path)]
    for i in range(len(lines) - len(pats) + 1):
        if all(pats[j].search(lines[i + j]) for j in range(len(pats))):
            return e, i + 1
    return e, 0


def c_expect(path, label):
    e, at = find(path, label)
    if at:
        print("EXPECT %s: FOUND at line %d of %s" % (label, at, os.path.basename(path)))
        return 0
    print("EXPECT %s: MISSING from %s; the entry, in the escaped view:" % (label, os.path.basename(path)))
    for x in e:
        print("  | " + x)
    return 1


def c_absent(path, label):
    e, at = find(path, label)
    if at:
        print("ABSENT %s: NO, FOUND at line %d of %s" % (label, at, os.path.basename(path)))
        return 1
    print("ABSENT %s: YES (in %s)" % (label, os.path.basename(path)))
    return 0


# The lines the brief's known-lines rule tolerates: string 17 (a pipe held past exit), recorded;
# and string 16's "could not check", which d5_check judges (recorded once, BLOCKED the second time).
S17 = re.compile(r"^git-remote-proton: .+ exited but something still held its output pipe open after ")
D5_UNKNOWN = "git-remote-proton: WARNING: could not check whether this git can restore"


def c_helperlines(path):
    hits = [l for l in read_lines(path) if l.startswith("git-remote-proton:")]
    known = [l for l in hits if S17.match(l) or l.startswith(D5_UNKNOWN)]
    print("git-remote-proton: lines in %s: %d; known lines among them: %d; others: %d" % (
        os.path.basename(path), len(hits), len(known), len(hits) - len(known)))
    for h in hits:
        print("  %s%s" % ("KNOWN: " if h in known else "", esc(h)))
    return 0


# ---- ref sets and relays
def read_refset(label):
    p = os.path.join(rdir("refsets"), label + ".txt")
    if not os.path.exists(p):
        raise Blocked("no ref set named %s" % label)
    clone, out = False, []
    for l in read_lines(p):
        if not l.strip():
            continue
        parts = l.split()
        if len(parts) != 2 or not re.match(r"^[0-9a-f]{40}$", parts[0]):
            raise Blocked("ref set %s holds a line that is not '<sha> <ref>': [%s]" % (label, esc(l)))
        ref = parts[1]
        if ref.startswith("refs/remotes/proton-v2/"):
            clone = True
            rest = ref[len("refs/remotes/proton-v2/"):]
            ref = "HEAD" if rest == "HEAD" else "refs/heads/" + rest
        out.append(parts[0] + " " + ref)
    return clone, sorted(set(out))


def digest(rs):
    return hashlib.sha256(("".join(r + "\n" for r in rs)).encode("utf-8", "surrogateescape")).hexdigest()


def nohead(rs):
    return [r for r in rs if not r.endswith(" HEAD")]


def c_refshow(label):
    clone, rs = read_refset(label)
    print("REFSET %s (%s): %d ref(s)" % (label, "clone" if clone else "remote", len(rs)))
    for r in rs:
        print("  " + esc(r))
    return 0


def c_refcmp(x, y):
    cx, rx = read_refset(x)
    cy, ry = read_refset(y)
    drop = cx or cy
    sx, sy = set(nohead(rx) if drop else rx), set(nohead(ry) if drop else ry)
    note = ", HEAD left out" if drop else ""
    if sx == sy:
        print("REFSET %s == %s: EQUAL (%d ref(s)%s)" % (x, y, len(sx), note))
        return 0
    print("REFSET %s vs %s: DIFFER%s" % (x, y, note))
    for r in sorted(sx - sy):
        print("  ONLY IN %s: %s" % (x, esc(r)))
    for r in sorted(sy - sx):
        print("  ONLY IN %s: %s" % (y, esc(r)))
    return 1


def c_relay(label):
    clone, rs = read_refset(label)
    print("RELAY refset %s %s %d %s %d %s" % (label, "clone" if clone else "remote", len(rs), digest(rs),
                                              len(nohead(rs)), digest(nohead(rs))))
    return 0


REFSET_RELAY = r"^RELAY refset ([A-Za-z0-9.-]+) (clone|remote) ([0-9]+) ([0-9a-f]{64}) ([0-9]+) ([0-9a-f]{64})$"
VALUE_RELAY = r"^RELAY value ([A-Za-z0-9_]+) ([A-Za-z0-9~_.-]+)$"
# The values this gate relays, each with its shape: a uid, a commit ID, a blob.
VALUE_SHAPES = {"pcroot": r"[A-Za-z0-9~_.-]+", "W3": r"[0-9a-f]{40}", "brief": r"[0-9a-f]{40}"}


def value_ok(name, value):
    if name not in VALUE_SHAPES:
        raise Blocked("relay value %s: not a value this gate relays (%s)" % (esc(name), ", ".join(sorted(VALUE_SHAPES))))
    if not re.fullmatch(VALUE_SHAPES[name], value):
        raise Blocked("relay value %s: [%s] does not have its shape (%s)" % (name, esc(value), VALUE_SHAPES[name]))


def c_relayin(line):
    line = line.strip()
    m, v = re.match(REFSET_RELAY, line), re.match(VALUE_RELAY, line)
    if not (m or v):
        raise Blocked("not a relay line: [%s]" % esc(line))
    if v:
        value_ok(v.group(1), v.group(2))
    p = os.path.join(rdir("relay"), ("refset-" + m.group(1)) if m else ("value-" + v.group(1)))
    if os.path.exists(p + ".txt") and read_lines(p + ".txt")[0] != line:
        raise Blocked("a different relay of that name was received before: [%s]" % esc(read_lines(p + ".txt")[0]))
    with open(p + ".txt", "w") as h:
        h.write(line + "\n")
    print("RECEIVED " + line)
    return 0


def c_relaycmp(label, theirs):
    p = os.path.join(rdir("relay"), "refset-" + theirs + ".txt")
    if not os.path.exists(p):
        raise Blocked("no relayed ref set %s (pass the PC's RELAY line to kit.sh relayin first)" % theirs)
    m = re.match(REFSET_RELAY, read_lines(p)[0])
    if not m:
        raise Blocked("the stored relay refset-%s is not a ref-set relay line" % theirs)
    clone, rs = read_refset(label)
    drop = clone or m.group(2) == "clone"
    mine = nohead(rs) if drop else rs
    want = (m.group(5), m.group(6)) if drop else (m.group(3), m.group(4))
    same = (str(len(mine)), digest(mine)) == want
    print("RELAYED REFSET %s vs %s: %s (%s ref(s)%s)" % (theirs, label, "EQUAL" if same else "DIFFER",
                                                     want[0], ", HEAD left out" if drop else ""))
    if not same:
        c_refshow(label)
    return 0 if same else 1


def c_getval(name):
    p = os.path.join(rdir("relay"), "value-" + name + ".txt")
    if not os.path.exists(p):
        raise Blocked("no relayed value %s (pass the PC's RELAY line to kit.sh relayin first)" % name)
    v = re.match(VALUE_RELAY, read_lines(p)[0])
    if not v or v.group(1) != name:
        raise Blocked("the stored relay value-%s is not a relay line of that name" % name)
    value_ok(name, v.group(2))
    print(v.group(2))
    return 0


# ---- the ownership record
def root_names():
    return ["%s-mac-%s-%s" % (env("TAG"), r, env("SUF")) for r in ROLES]


def absent_name():
    return "%s-mac-absent-%s" % (env("TAG"), env("SUF"))


def roots_path():
    return os.path.join(env("G"), "roots.tsv")


def read_roots():
    if not os.path.exists(roots_path()):
        return None
    return [dict(zip(("name", "uid", "how", "time"), l.split("\t"))) for l in read_lines(roots_path())]


def c_roots_start():
    if os.path.exists(roots_path()):
        raise Blocked("%s already exists (step 0 already ran in run %s)" % (roots_path(), env("RUN")))
    cas = live_named("/my-files", "_cas-probe")
    if len(cas) != 1 or cas[0]["type"] != "folder":
        raise Blocked("/my-files/_cas-probe is not one folder (%d row(s)): the contract root's parent must exist, "
                      "and creating it is outside this gate's confinement (STOP for Craig); no ownership record was written" % len(cas))
    if live_named("/my-files/_cas-probe", "contract-mac"):
        raise Blocked("%s already exists (a stale root); no ownership record was written, so cleanup trashes nothing" % CONTRACT)
    print("_cas-probe: PRESENT (uid %s), read-only; contract-mac: ABSENT" % cas[0]["uid"])
    top = live_named("/my-files", "GitRemotes")
    if len(top) > 1 or [t for t in top if t["type"] != "folder"]:
        raise Blocked("/my-files/GitRemotes is not one folder; no ownership record was written")
    if not top:
        line = "GitRemotes\t-\tabsent-at-step-0\t" + now()
        print("GitRemotes: ABSENT at step 0 (step 2.0 creates it; 7.1 trashes it only if 2.0 did)")
    else:
        kids = rows(env("R"))
        stale = [k for k in kids if k["name"] in root_names() + [absent_name()]]
        for k in stale:
            print("STALE: %s (uid %s)" % (esc(k["path"]), k["uid"]))
        if stale:
            raise Blocked("GitRemotes already holds a root name of this run; no ownership record was written, so cleanup trashes nothing")
        for k in kids:
            if k["name"].startswith(env("TAG") + "-mac-"):
                print("EARLIER ROOT (read-only; this run never trashes it): %s (uid %s)" % (esc(k["path"]), k["uid"]))
        line = "GitRemotes\t%s\tpresent-at-step-0\t%s" % (top[0]["uid"], now())
        print("GitRemotes: PRESENT at step 0 (uid %s, %d child row(s)); not this gate's, never trashed" % (top[0]["uid"], len(kids)))
    with open(roots_path(), "w") as h:
        h.write(line + "\n")
    return 0


def c_roots_update():
    rec = read_roots()
    if rec is None:
        print("NO ROOT RECORD: step 0 never passed its checks, so this run owns no remote node")
        return 0
    R, problems, add = env("R"), [], []
    top = live_named("/my-files", "GitRemotes")
    gr = [x for x in rec if x["name"] == "GitRemotes"]
    if gr[0]["how"] == "absent-at-step-0" and len(gr) == 1 and len(top) == 1:
        runs = os.path.join(env("G"), "log", "runs.txt")
        made = os.path.exists(runs) and any(l.startswith("EXIT s2-0-gitremotes = 0 ") for l in read_lines(runs))
        if made:
            add.append("GitRemotes\t%s\tcreated-by-2.0\t%s" % (top[0]["uid"], now()))
            print("RECORDED: %s (uid %s), created by step 2.0" % (R, top[0]["uid"]))
        else:
            problems.append("%s exists, but this run's step 2.0 did not create it: not this run's (STOP for Craig)" % R)
    for c in [x for x in gr if x["how"] != "absent-at-step-0"]:
        if len(top) != 1 or top[0]["uid"] != c["uid"]:
            problems.append("%s is gone or has another uid (recorded %s, %s)" % (R, c["uid"], c["how"]))
    kids = [k for k in rows(R) if not k["trashTime"]] if len(top) == 1 else []
    places = [(n, R, [k for k in kids if k["name"] == n]) for n in root_names()]
    places.append(("contract-mac", "/my-files/_cas-probe", live_named("/my-files/_cas-probe", "contract-mac")))
    for name, parent, live in places:
        old = [x for x in rec if x["name"] == name]
        if old:
            if len(live) != 1 or live[0]["uid"] != old[0]["uid"]:
                problems.append("%s/%s is gone or has another uid (recorded %s)" % (parent, name, old[0]["uid"]))
            else:
                print("OK: %s/%s uid %s" % (parent, name, old[0]["uid"]))
        elif len(live) == 1:
            add.append("%s\t%s\tcreated\t%s" % (name, live[0]["uid"], now()))
            print("RECORDED: %s/%s (uid %s)" % (parent, name, live[0]["uid"]))
    if [k for k in kids if k["name"] == absent_name()]:
        problems.append("%s/%s exists, and it must never exist (step 6 names it as a missing parent)" % (R, absent_name()))
    with open(roots_path(), "a") as h:
        for a in add:
            h.write(a + "\n")
    for p in problems:
        print("ROOT RECORD PROBLEM: " + p)
    if problems:
        raise Blocked("the ownership record does not match the account")
    return 0


def c_vet(path):
    base = path.rstrip("/") + "/"
    pats = [r"TestContractCLI_[^/]+(/.*)?"] if path == CONTRACT else REPO_OK
    allr = subtree(path)
    foreign = 0
    for r in allr:
        rel = r["path"][len(base):]
        ok = any(re.fullmatch(p, rel, re.S) for p in pats)
        foreign += 0 if ok else 1
        print("  %s %-6s %s  uid %s%s" % ("ok     " if ok else "FOREIGN", r["type"], esc(rel), r["uid"],
                                         ("  trash " + r["trashTime"]) if r["trashTime"] else ""))
        if rel == ".lock" and not r["trashTime"]:
            print("  LIVE LOCK: %s (this gate's own; record it, and it goes with its root)" % esc(r["path"]))
    if foreign:
        print("VET %s: %d FOREIGN ROW(S) (STOP for Craig; do not trash this root)" % (esc(path), foreign))
        return 1
    print("VET %s: ONLY GATE ARTIFACTS (%d row(s))" % (esc(path), len(allr)))
    return 0


def c_trashroot(name):
    o = [x for x in (read_roots() or []) if x["name"] == name and x["how"] == "created"]
    if not o:
        print("NOT RECORDED: %s is not a root this run created; nothing trashed" % name)
        return 0
    parent = "/my-files/_cas-probe" if name == "contract-mac" else env("R")
    path = parent + "/" + name
    live = live_named(parent, name)
    if not live:
        print("ALREADY GONE: %s (an earlier trash line of this run must show it; if none does, STOP for Craig)" % path)
        return 0
    if len(live) != 1 or live[0]["uid"] != o[0]["uid"]:
        print("NOT TRASHED: %s: its live uid is not the recorded %s (STOP for Craig)" % (path, o[0]["uid"]))
        return 1
    if c_vet(path) != 0:
        print("NOT TRASHED: %s (STOP for Craig)" % path)
        return 1
    code, out, err = cli(["filesystem", "trash", path, "--json"])
    print("trash %s (uid %s): exit %d at %s  %s" % (path, o[0]["uid"], code, now(), esc((out + err).strip())))
    return 0 if code == 0 else 1


def live_subtree(path):
    # Every row beneath path, depth first, not descending into a folder that already carries a
    # trashTime: it has gone to the trash with its contents.
    out = []
    for r in rows(path):
        out.append(r)
        if r["type"] == "folder" and not r["trashTime"]:
            out.extend(live_subtree(r["path"]))
    return out


def c_trashgr():
    made = [x for x in (read_roots() or []) if x["name"] == "GitRemotes" and x["how"] == "created-by-2.0"]
    if not made:
        print("GitRemotes: this run did not create it (no created-by-2.0 record), so it stays")
        return 0
    top = live_named("/my-files", "GitRemotes")
    if len(top) != 1 or top[0]["uid"] != made[0]["uid"]:
        print("GitRemotes: its live uid is not the recorded %s, so it is not trashed (STOP for Craig)" % made[0]["uid"])
        return 1
    # Rule 3: GitRemotes' whole subtree, enumerated just before its trash. Only this run's roots may
    # appear (a trashed one still listed with a trashTime while the lag lasts), and nothing live.
    base, mine = env("R") + "/", set(root_names())
    allr = live_subtree(env("R"))
    foreign = live = 0
    for r in allr:
        rel = r["path"][len(base):]
        ours = rel.split("/")[0] in mine
        foreign += 0 if ours else 1
        live += 0 if r["trashTime"] else 1
        print("  %s %-6s %s  uid %s%s" % ("ok     " if ours else "FOREIGN", r["type"], esc(rel), r["uid"],
                                         ("  trash " + r["trashTime"]) if r["trashTime"] else ""))
    print("GitRemotes subtree: %d row(s), %d live, %d not this run's" % (len(allr), live, foreign))
    if foreign:
        print("GitRemotes: it holds rows that are not this run's roots, so it is not trashed (STOP for Craig)")
        return 1
    if live:
        print("GitRemotes: live rows remain (a root left in place), so it stays")
        return 0
    code, out, err = cli(["filesystem", "trash", env("R"), "--json"])
    print("trash %s: exit %d at %s  %s" % (env("R"), code, now(), esc((out + err).strip())))
    return 0 if code == 0 else 1


def c_pushtree(samples, outlog, errlog):
    # Step 6.3, from the sampler's lines ("<epoch seconds> pid pgid ppid command", every sample of
    # every pid kept) and the hang agent's two logs. A push is a git push to proton-fail the sampler
    # saw. It must lead its own process group (the wrapper's set -m), or BLOCKED. Its tree is every
    # process in that group and every descendant by parent, over all samples. A push the wrapper
    # logs as killed at its bound counts as reaped only when its tree held a git-remote-proton or
    # proton-drive process still seen within 0.5 s of the bound (counted from the push's first
    # sample) or later, and the error log holds string 14 exactly once for each push that was not
    # killed: a killed push that still printed string 14 either ended just before the kill landed
    # or was not stopped by it. Any other git-remote-proton: line in the error log is BLOCKED, so
    # that two anomalies cannot cancel in that count (string 17, a pipe held past exit, is listed
    # and recorded, the brief's known-lines rule); and the out log must hold one firing.
    # Exit 0: a push reaped so, and nothing of any push still runs. Exit 3: (b) not exercised (no
    # push reaped so, string 14 from a killed push, or not one firing). Exit 1: BLOCKED (no push
    # seen, a push not leading its group, a process still running, another git-remote-proton:
    # line, or a push not killed that printed no string 14).
    out = "\n".join(read_lines(outlog))
    killed = set(re.findall(r"gpb-v2-push: git push proton-fail (--all|--tags) ran past [0-9]+s and was killed", out))
    ended = set(re.findall(r"gpb-v2-push: git push proton-fail (--all|--tags): exit [0-9]+", out))
    bounds = [int(b) for b in re.findall(r"gpb-v2-push: git push proton-fail --[a-z]+ ran past ([0-9]+)s and was killed", out)]
    bound = max(bounds) if bounds else 0
    elines = read_lines(errlog)
    n14 = len([l for l in elines if l.startswith("git-remote-proton: parent folder ")])
    s17 = S17
    known = [l for l in elines if s17.match(l)]
    others = [l for l in elines if l.startswith("git-remote-proton:") and not s17.match(l)
              and not l.startswith("git-remote-proton: parent folder ")]
    starts = len(re.findall(r"gpb-v2-push: start: ", out))
    procs = {}
    for l in read_lines(samples):
        p = l.split(None, 4)
        if len(p) == 5 and re.match(r"^[0-9]+([.][0-9]+)?$", p[0]) and p[1].isdigit() and p[2].isdigit() and p[3].isdigit():
            procs.setdefault(p[1], []).append((float(p[0]), p[2], p[3], p[4]))
    root_re = r"(^|/)git -C [^ ]+ push proton-fail (--all|--tags)$"
    tool_re = r"(^|/)(git-remote-proton|proton-drive)( |$)"
    roots, blocked = [], []
    for pid, ss in procs.items():
        hits = [s for s in ss if re.search(root_re, s[3])]
        if hits:
            roots.append((pid, re.search(root_re, hits[0][3]).group(2), min(s[0] for s in hits)))
            if any(s[1] != pid for s in hits):
                blocked.append("the push %s (pid %s) did not lead a process group of its own under launchd" % (
                    roots[-1][1], pid))
    alive, exercised = 0, False
    for root, flag, t0 in sorted(roots, key=lambda x: int(x[0])):
        tree, grew = {root}, True
        while grew:
            grew = False
            for pid, ss in procs.items():
                if pid not in tree and any(s[2] in tree or s[1] == root for s in ss):
                    tree.add(pid)
                    grew = True
        tools = [q for q in tree if any(re.search(tool_re, s[3]) for s in procs[q])]
        clis = [q for q in tools if any(re.search(r"(^|/)proton-drive( |$)", s[3]) for s in procs[q])]
        last = max([max(s[0] for s in procs[q]) for q in tools] or [t0 - 1])
        was_killed = flag in killed
        at_bound = was_killed and bool(tools) and last >= t0 + bound - 0.5
        print("PUSH %s (pid %s, its own group: %s): %s; %d process(es); helper or CLI seen: %d (the CLI itself: %d)" % (
            flag, root, "yes" if all(s[1] == root for s in procs[root] if re.search(root_re, s[3])) else "NO",
            "killed at its bound" if was_killed else "ended inside its bound", len(tree), len(tools), len(clis)))
        if was_killed:
            print("  helper or CLI last seen %+.2f s after the push's first sample (bound %d s; within 0.5 s counts): %s" % (
                last - t0, bound, "ALIVE AT THE BOUND" if at_bound else "NOT ALIVE AT THE BOUND"))
        for pid in sorted(tree, key=int):
            t, g, pp, c = procs[pid][-1]
            now_ = subprocess.run(["/bin/ps", "-p", pid, "-o", "pgid=,command="],
                                  stdout=subprocess.PIPE).stdout.decode("utf-8", "surrogateescape").strip()
            running = bool(now_) and now_.split(None, 1)[0] == g
            alive += 1 if running else 0
            print("  pid %s pgid %s ppid %s%s %s  (%d sample(s), last %+.2f s)  NOW: %s" % (
                pid, g, pp, "" if g == root else " (OTHER GROUP)", esc(c), len(procs[pid]), t - t0,
                ("RUNNING " + esc(now_)) if running else "gone"))
        exercised = exercised or at_bound
    want14 = len(ended) - len(killed & ended)
    print("STRING 14: %d line(s) in %s; pushes ended: %d, killed at the bound: %d, so %d expected" % (
        n14, os.path.basename(errlog), len(ended), len(killed & ended), want14))
    print("OTHER git-remote-proton: LINES in %s: %d (none expected); string 17's, recorded: %d" % (
        os.path.basename(errlog), len(others), len(known)))
    for l in known:
        print("  KNOWN, string 17 (recorded): " + esc(l))
    for l in others:
        print("  OTHER: " + esc(l))
    print("FIRINGS in %s: %d (one expected)" % (os.path.basename(outlog), starts))
    print("PUSH TREE: %d push root(s), %d still running; killed at the bound: %s" % (
        len(roots), alive, ", ".join(sorted(killed)) or "none"))
    for b in blocked:
        print("BLOCKED: " + b)
    if not roots or alive or blocked:
        return 1
    if others:
        print("BLOCKED: the error log holds a git-remote-proton: line that is neither string 14 nor string 17 (OTHER, above)")
        return 1
    if starts != 1:
        print("(b) NOT EXERCISED: the out log holds %d firings, and this check judges one firing's logs" % starts)
        return 3
    if n14 < want14:
        print("BLOCKED: a push that was not killed did not print string 14 (the missing-parent refusal)")
        return 1
    if n14 > want14:
        print("(b) NOT EXERCISED: a push the wrapper logs as killed also printed string 14: its helper ended just before the kill landed, or the kill did not stop it (the logs cannot tell which)")
        return 3
    if not exercised:
        print("(b) NOT EXERCISED: no push killed at its bound had a git-remote-proton or proton-drive process alive at the bound")
        return 3
    print("(b) EXERCISED: a push killed at its bound had the helper or the CLI alive at the bound, it printed no string 14, and nothing is left")
    return 0


def c_readme(path, outdir, want_helper, want_cli):
    # C.3 and step 1: README "Install on macOS (v2)"'s one bash block that clones the tag, split at
    # its CLI download into the helper's lines and the CLI's lines. Comment and blank lines are
    # dropped; every other line is kept as the README holds it. The helper's lines must set TAG
    # exactly once, as TAG=vX.Y.Z. Prints both parts in the escaped view, each with the SHA-256 of
    # its lines as the README holds them, and compares the two with the brief's quote. Exit 0
    # ("README LINES: AS QUOTED") writes <outdir>/helper.sh, with this run's tag for vX.Y.Z, and
    # <outdir>/cli.sh, unchanged. Exit 3: a part differs from the quote, nothing written (STOP for
    # adjudication). BLOCKED: any other shape.
    lines = [l[:-1] if l.endswith("\r") else l for l in open(path, encoding="utf-8").read().split("\n")]
    heads = [i for i, l in enumerate(lines) if re.match(r"^#{2,4} Install on macOS \(v2\)[ \t]*$", l)]
    if len(heads) != 1:
        raise Blocked("README: %d 'Install on macOS (v2)' headings, not 1" % len(heads))
    h = heads[0]
    level = len(lines[h]) - len(lines[h].lstrip("#"))
    # The section ends at the next heading of its level or above, outside a fenced block (a "#"
    # comment line inside a block is not a heading).
    fence, blocks, cur = "`" * 3, [], None
    for l in lines[h + 1:]:
        if l.startswith(fence):
            if cur is None:
                cur = []
            else:
                blocks.append(cur)
                cur = None
        elif cur is not None:
            cur.append(l)
        elif re.match(r"^#{1,%d} " % level, l):
            break
    if cur is not None:
        raise Blocked("README: a fenced block in its macOS section is not closed")
    picked = [b for b in blocks if any(l.startswith("git clone --branch ") for l in b)]
    if len(picked) != 1:
        raise Blocked("README: %d blocks in its macOS section clone the tag, not 1" % len(picked))
    body = [l for l in picked[0] if l.strip() and not l.lstrip().startswith("#")]
    cut = [i for i, l in enumerate(body) if l.startswith("curl -fLo proton-drive ")]
    if len(cut) != 1 or cut[0] == 0:
        raise Blocked("README: the block does not hold one 'curl -fLo proton-drive' line after the helper's lines")
    parts = (("helper", body[:cut[0]], want_helper), ("cli", body[cut[0]:], want_cli))
    tags = [i for i, l in enumerate(parts[0][1]) if l.startswith("TAG=")]
    if len(tags) != 1 or parts[0][1][tags[0]] != "TAG=vX.Y.Z":
        raise Blocked("README: the helper's lines do not set TAG exactly once, as TAG=vX.Y.Z")
    tag = env("TAG")
    if not re.match(r"^v[0-9A-Za-z.-]+$", tag):
        raise Blocked("TAG %r is not a tag name" % tag)
    same = True
    for name, part, want in parts:
        got = hashlib.sha256("".join(l + "\n" for l in part).encode("utf-8")).hexdigest()
        same = same and got == want
        print("README %s lines: %d, SHA-256 %s  (%s)" % (name, len(part), got,
                                                       "as quoted" if got == want else "QUOTED: " + want))
        for l in part:
            print("  | " + esc(l))
    if not same:
        print("README LINES: NOT AS QUOTED (STOP for adjudication; nothing written, nothing run)")
        return 3
    run = list(parts[0][1])
    run[tags[0]] = "TAG=" + tag
    for name, text in (("helper", run), ("cli", parts[1][1])):
        with open(os.path.join(outdir, name + ".sh"), "w", encoding="utf-8") as f:
            f.write("".join(l + "\n" for l in text))
    print("README LINES: AS QUOTED; wrote %s/helper.sh (TAG=%s) and %s/cli.sh" % (outdir, tag, outdir))
    return 0


COMMANDS = {
    "pushtree": (3, c_pushtree), "readme": (4, c_readme),
    "names": (1, c_names), "info": (1, c_info), "absentnode": (1, c_absentnode),
    "esc": (1, c_esc), "expect": (2, c_expect), "absent": (2, c_absent), "helperlines": (1, c_helperlines),
    "refshow": (1, c_refshow), "refcmp": (2, c_refcmp), "relay": (1, c_relay),
    "relayin": (1, c_relayin), "relaycmp": (2, c_relaycmp), "getval": (1, c_getval),
    "roots-start": (0, c_roots_start), "roots-update": (0, c_roots_update),
    "vet": (1, c_vet), "trashroot": (1, c_trashroot), "trashgr": (0, c_trashgr),
}


def main():
    a = sys.argv[1:]
    try:
        if a and a[0] == "snapshot" and len(a) in (3, 4) and (len(a) == 3 or a[3] == "-r"):
            sys.exit(c_snapshot(a[1], a[2], len(a) == 4))
        if a and a[0] == "compare" and len(a) in (3, 4) and (len(a) == 3 or a[3] == "--none"):
            sys.exit(c_compare(a[1], a[2], len(a) == 4))
        if not a or a[0] not in COMMANDS or len(a) - 1 != COMMANDS[a[0]][0]:
            print("usage: gpbkit.py snapshot <label> <path> [-r] | compare <before> <after> [--none] | "
                  + " | ".join(sorted(COMMANDS)))
            sys.exit(2)
        sys.exit(COMMANDS[a[0]][1](*a[1:]))
    except Blocked as e:
        print("BLOCKED: %s" % e)
        sys.exit(1)


main()
```

*Brief author's call:* one kit, on the Mac, for every listing and comparison, the Mac CLI being
the session that writes, so its own listing is fresh (the v0.10.0 Linux brief's point about the
writer's view). The PC adds a second view after its own pushes (W.5.1, W.5.5, W.5.8), from the
session that made them.

---

## Part A: prep with no account, the v0.10.0 build (section 6, "Prep, before tagging, with no account")

**When:** after the merge and the version's renaming, like Part B, and before the sign-in probe and
the tag, so that a failure is found before any gate state is held (section 6).

**What it is for.** Step 5.5 runs v0.10.0's helper on the Mac, and v0.10.0 shipped no macOS
binary. Section 6 says to build it from a fresh clone of the `v0.10.0` tag with section 1's recipe
into a scratch folder outside every clone and never on PATH, and to confirm its `--version`.

**A.0 Save the extractor and extract the brief.** The runner makes `~/gpbmac`, saves the Mac
extractor block (above) to `~/gpbmac/extract.py` by hand, and runs:

```bash
mkdir -p ~/gpbmac
/usr/bin/git -C ~/Projects/_Tools/git-proton-backup fetch origin
/usr/bin/git -C ~/Projects/_Tools/git-proton-backup rev-parse refs/remotes/origin/main
/usr/bin/python3 -I ~/gpbmac/extract.py refs/remotes/origin/main docs/research/gates/vNEXT-macos-gate-brief.md
```

**Pass:** the fetch exits 0, the commit is printed (record it), and the extractor passes (see "The
Mac's extractor"). B.0 fetches and extracts again and relays that blob, which the PC compares with
its own. If `~/gpbmac` already exists with a `v010-build` in it, A.1 BLOCKs.

**A.1 The build:** `/bin/bash ~/gpbmac/kit/gx.sh a1-v010-build` is not possible yet (no run is
started), so this one script runs directly: `/bin/bash ~/gpbmac/scripts/a1-v010-build.sh`.

```bash
# file: mac:scripts/a1-v010-build.sh
# Part A: v0.10.0's helper, built from a fresh clone of its tag with spec section 1's recipe, into
# a scratch folder outside every clone and never on PATH. No account. Run it directly, once.
set -u
GM="$HOME/gpbmac"; B="$GM/v010-build"
if [ -e "$B" ]; then echo "BLOCKED: $B already exists (Craig decides whether that build is deleted)"; exit 1; fi
mkdir -p "$B/bin" || exit 1
: > "$B/gitconfig-global" || exit 1
export PATH="$HOME/sdk/go1.27.0/bin:/usr/bin:/bin:/usr/sbin:/sbin" GOTOOLCHAIN=local
export GIT_CONFIG_GLOBAL="$B/gitconfig-global" GIT_CONFIG_NOSYSTEM=1
echo "go: $(command -v go), $(go version); GOFLAGS=[$(go env GOFLAGS)]"
echo "git: $(command -v git), $(git --version)"
git clone -q --branch v0.10.0 https://github.com/craigstoller/git-proton-backup "$B/src"; rc=$?
echo "clone of v0.10.0: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
c=$(git -C "$B/src" rev-parse HEAD) || exit 1
echo "v0.10.0 commit: $c"
[ "$c" = e5c76af3bfc78ee90efcf60db8b11d8d0ab76099 ] || { echo "BLOCKED: the tag's commit is not the one this brief recorded"; exit 1; }
echo "untracked or modified in the clone: [$(git -C "$B/src" status --porcelain --untracked-files=all)]"
(cd "$B/src" && CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=v0.10.0" -o "$B/bin/git-remote-proton" ./cmd/git-remote-proton); rc=$?
echo "build: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
"$B/bin/git-remote-proton" --version
go version -m "$B/bin/git-remote-proton"
echo "git-remote-proton on this PATH: [$(command -v git-remote-proton || true)]"
```

**Pass:** all of the following hold. Anything else is BLOCKED: STOP for Craig before the probe.

- `clone of v0.10.0: exit 0`, the commit `e5c76af3bfc78ee90efcf60db8b11d8d0ab76099`, and an empty
  `untracked or modified` list.
- `build: exit 0`, then exactly `git-remote-proton v0.10.0 (certified CLI:
  cli-drive@0.9.0+75612299)`.
- `go version -m` shows `-trimpath=true`, `CGO_ENABLED=0`, `GOOS=darwin`, `GOARCH=arm64`,
  `vcs.revision=e5c76af3bfc78ee90efcf60db8b11d8d0ab76099` and `vcs.modified=false`. Record its
  first line (the toolchain) and `GOFLAGS`.
- `git-remote-proton on this PATH: []`. The build is never put on PATH; step 5.5 names it for its
  own commands alone.

The probes found that a darwin build of the current tree compiles and that only three hermetic
tests failed, on a path alias that does not affect the binary (M0) [inference for v0.10.0's tree].
No test runs here. The build stays until step 7.4 deletes it.

---

## Part B: the pre-tag sign-in probe (section 6, "Prep, before tagging: a sign-in probe")

**When:** after Part A, after the merge and the CHANGELOG flip, and before the tag
(`docs/releasing.md` step 3).

**What it is for.** It shows that Craig can sign the Mac's CLI in through the browser (F10) with
`PROTON_DRIVE_CACHE_DIR` set, that every state file lands in that folder (M7), that the Keychain
item appears and goes, and that the PC's session survives all of it. It changes nothing in the
release and ends logged out (section 6).

**Who runs what.** Steps 1, 2, 5, the Windows checks of step 6, and the unpause of step 7 are the
PC session's (Part W, W.B.1 to W.B.7). Steps 3, 4, 6 and 7 run on the Mac. The order, with every
handoff, is Part W's table.

**Which CLI it runs.** The feed-verified binary at `~/gpbmac/cli/proton-drive`, by absolute path
(`$PD`), so nothing lands on PATH. B.0 downloads it and checks it.

**Where its results go.** They open the gate record, `docs/research/gates/vNEXT-macos-gate.md`,
under "Pre-tag sign-in probe". The repository is public, so **a sign-in URL goes into the record
only with its query string and fragment removed**, as in the v0.10.0 briefs.

**Steps 5, 6 and 7 always run once step 1 has run,** each even when a step before it failed, and
each failure is recorded. That is how the probe ends logged out, cleaned up and with the PC's task
decision made on every path.

### B.0 Prep, on both machines

**Mac:** A.0's fetch and extraction again (so the Mac's blob is the one the PC extracts now), then
the probe's run, then the prep script.

```bash
/usr/bin/git -C ~/Projects/_Tools/git-proton-backup fetch origin
/usr/bin/git -C ~/Projects/_Tools/git-proton-backup rev-parse refs/remotes/origin/main
/usr/bin/python3 -I ~/gpbmac/extract.py refs/remotes/origin/main docs/research/gates/vNEXT-macos-gate-brief.md
/bin/bash ~/gpbmac/kit/start-run.sh probe 1   # a re-run of the probe, on Craig's word only, takes 2
/bin/bash ~/gpbmac/kit/gx.sh p0-prep
```

```bash
# file: mac:scripts/p0-prep.sh
# B.0: the probe's CLI, downloaded once and checked against the certified SHA-512, and the
# machine's starting state. No account.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
have curl shasum xattr codesign security cksum od install launchctl plutil || bad "missing tool(s), named above"
CONST=$(git -C "$MAIN" show refs/remotes/origin/main:scripts/Test-ProtonCliDrift.ps1 | tr -d '\r' | grep -oE "^[$]CertifiedMacArm64Sha512[[:space:]]*=[[:space:]]*'[0-9a-f]{128}'[[:space:]]*$" | grep -oE '[0-9a-f]{128}')
echo "certified macos/arm64 SHA-512 (origin/main's drift script): ${CONST:-NONE}"
echo "M2's value, quoted in this brief:                            $M2"
[ "$CONST" = "$M2" ] || bad "the drift script holds no \$CertifiedMacArm64Sha512 equal to M2's (plan Task 12)"
CLI="$GM/cli/proton-drive"
if [ ! -e "$CLI" ]; then
  mkdir -p "$GM/cli/dl" || exit 1
  (cd "$GM/cli/dl" && curl -fLo proton-drive https://proton.me/download/drive/cli/0.9.0/darwin-arm64/proton-drive) || { echo "BLOCKED: the download failed"; exit 1; }
  (cd "$GM/cli/dl" && echo "$M2  proton-drive" | shasum -a 512 -c) && install -m 0755 "$GM/cli/dl/proton-drive" "$CLI" || { echo "BLOCKED: the download is not the certified build"; exit 1; }
  rm -rf "$GM/cli/dl"
fi
GOT=$(shasum -a 512 < "$CLI" | cut -d' ' -f1)
echo "probe CLI SHA-512:                                            $GOT"
[ "$GOT" = "$M2" ] || bad "the probe CLI is not the certified build"
if xattr -p com.apple.quarantine "$CLI" >/dev/null 2>&1; then bad "the probe CLI carries com.apple.quarantine"; else echo "quarantine attribute on the probe CLI: none"; fi
codesign -dv "$CLI" 2>&1 | grep -E '^(Identifier|Authority|TeamIdentifier)='
echo "proton-drive on the probe PATH: [$(command -v proton-drive || true)]"
for x in "$HOME/.local/bin" /opt/homebrew/bin /usr/local/bin; do
  for f in proton-drive git-remote-proton; do [ -e "$x/$f" ] && bad "$x/$f exists (C.3 and step 0 would refuse it later)"; done
done
m7 || bad "an M7 default folder exists"
kc_expect absent "the CLI's Keychain item already exists"
echo "probe cache folder: $PROTON_DRIVE_CACHE_DIR, entries: [$(ls -A "$PROTON_DRIVE_CACHE_DIR")]"
echo "variables inherited by a probe script: ${INHERITED:-none}"
[ -z "$INHERITED" ] || bad "GPB_, PROTON_DRIVE_, GIT_CONFIG or object-placement GIT_ variables reach the scripts"
echo "== what launchd's own environment hands an agent of this GUI domain (a probe agent; no account)"
envprobe "$G/envprobe" || bad "launchd's environment is not the gate's (the lines above say why; names only, never a value)"
if [ "$fail" = 0 ]; then echo "PREP: PASS"; else echo "PREP: BLOCKED"; fi
exit "$fail"
```

`$M2` is `lib.sh`'s constant, M2's value from the spec. **Pass:** the fetch exits 0, the commit is
recorded, the extractor's `SELF-CHECK ... YES` and its `RELAY value brief <blob>` (relayed to the
PC, whose W.B.0 blob must equal it), `RUN STARTED: probe-a1 ...`, then
`PREP: PASS` and `EXIT p0-prep = 0`, which means:

- the drift script on `origin/main` holds `$CertifiedMacArm64Sha512` equal to M2's value, and the
  probe CLI's SHA-512 is that value (M2). A missing constant means plan Task 12 has not landed:
  STOP for Craig. `BLOCKED: the download failed`, or `... not the certified build`, means the
  certified 0.9.0 `darwin-arm64` artifact is gone or re-cut (the README warns its versioned URL
  may stop serving once Proton moves on [README]): a new CLI needs a new pin, M2 and README quote,
  so it is a STOP for Craig before the tag, and the brief is revised first;
- no quarantine attribute (M2: a `curl` download carries none), and `codesign` prints Proton's
  Developer ID (M3); record it;
- no `proton-drive` on the probe's PATH, and neither `proton-drive` nor `git-remote-proton` in
  `~/.local/bin`, `/opt/homebrew/bin` or `/usr/local/bin` (the probe's PATH holds none of them, so
  it checks them by name); no M7 default folder, no Keychain item, an empty probe cache folder, and
  no inherited variable;
- `LAUNCHD ENVIRONMENT: CLEAN`: the probe agent `gpbmac.probe-a1.envprobe`, bootstrapped into
  Craig's GUI domain, listed no `GPB_`, `PROTON_DRIVE_` or `GIT_` name but its plist's own
  `PROTON_DRIVE_CACHE_DIR`, with the plist's value, and its `HOME` is Craig's (step 6, "Nor may
  launchd's own environment set it"). It is the gate's first `launchctl bootstrap`; anything macOS
  shows for it is recorded [untested]. `NOT CLEAN` names each variable, never its value: STOP for
  Craig before the tag. Clearing one from launchd's environment (`launchctl unsetenv <name>`, in
  his own Terminal) is his call; 6.0 checks again before any of step 6's agents exists.

**The screen-lock reading's positive control** (no account):
`/bin/bash ~/gpbmac/kit/gx.sh p0-lockcheck`. Step 6.1 judges a locked screen by `lib.sh`'s
`locked`, whose key is measured absent on an unlocked screen and untested on a locked one; this
tests it before the tag, so a
reading that never works is found while the brief can still change. When the script says so,
Craig locks the screen (Ctrl-Cmd-Q), waits about 30 seconds, and unlocks it.

```bash
# file: mac:scripts/p0-lockcheck.sh
# B.0: the positive control for step 6's screen-lock reading (lib.sh's locked), with no account.
# It reads the key every 5 s for 60 s while Craig locks the screen and unlocks it again.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
echo "before (unlocked): $(locked)"
echo "Craig: lock the screen now (Ctrl-Cmd-Q), wait about 30 s, then unlock it"
seen=0
for k in 1 2 3 4 5 6 7 8 9 10 11 12; do
  sleep 5; v=$(locked); echo "t+$((k * 5)) s: $v"
  if [ "$v" = true ]; then seen=$((seen + 1)); fi
done
echo "after (unlocked again): $(locked)"
echo "readings of true while locked: $seen of 12"
if [ "$seen" -gt 0 ]; then echo "LOCK READING: WORKS"; exit 0; fi
echo "LOCK READING: NEVER TRUE (STOP for Craig before the tag: step 6.1 needs another way to read the lock)"
exit 3
```

**Pass:** `before (unlocked): no (key absent)` [measured 2026-10-27 on an unlocked screen],
some readings `true` while Craig had the screen locked, `after ...: no (key absent)`, and `LOCK
READING: WORKS`. `NEVER TRUE`: STOP for Craig before the tag; the brief's lock reading is
revised first.

**PC:** W.B.0, which extracts the brief on the PC and compares its blob with the Mac's. Then W.B.1
and W.B.2.

### B.1 and B.2, on the PC

W.B.1 pauses `ProjectsBackupDaily` (and `ProtonCliDriftCheck`) and waits until nothing runs, and
W.B.2 takes the Windows baseline and relays its uid: `RELAY value pcroot <uid>`. The Mac runner
stores that line:

```bash
/bin/bash ~/gpbmac/kit/kit.sh relayin 'RELAY value pcroot <the uid the PC printed>'
```

**Pass:** `RECEIVED RELAY value pcroot <uid>`.

### B.3 On the Mac: assert first, then Craig signs in

`/bin/bash ~/gpbmac/kit/gx.sh p3-assert`, then Craig's sign-in.

```bash
# file: mac:scripts/p3-assert.sh
# B.3, first half: no Keychain item and none of M7's default folders, before any sign-in.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
m7 || bad "an M7 default folder exists"
kc_expect absent "the CLI's Keychain item exists before the sign-in"
echo "probe cache folder entries: [$(ls -A "$PROTON_DRIVE_CACHE_DIR")]"
[ -z "$(ls -A "$PROTON_DRIVE_CACHE_DIR")" ] || bad "the probe cache folder is not empty"
if [ "$fail" = 0 ]; then echo "ASSERT: PASS (Craig may sign in)"; else echo "ASSERT: BLOCKED"; fi
exit "$fail"
```

**Pass:** `ASSERT: PASS (Craig may sign in)`. **Craig types,** in a new Terminal window, the first
line, and the second only once the first has named this probe's run:

```bash
source ~/gpbmac/"$(cat ~/gpbmac/run.txt)"/env.sh; echo "run: $RUN; cache folder: $PROTON_DRIVE_CACHE_DIR"
"$PD" auth login; echo "auth login: exit $?"
```

- **The run:** `run: probe-a1; cache folder: /Users/craig/gpbmac/probe-a1/cache`, or `probe-a2`
  in both places on a re-run: the run B.0 started, whose cache folder B.4 checks and B.7 deletes.
  Any other line: STOP, and no sign-in.
- **Expected:** the CLI opens Craig's browser, or prints a URL to open (F10); Craig completes the
  sign-in there, and the CLI reports success. Its exact text is [untested] on the Mac: record it
  verbatim, any URL stripped of its query string and fragment.
- **Pass:** a completed sign-in and `auth login: exit 0`.
- **Not authorised:** storing the password anywhere, or setting
  `PROTON_DRIVE_CREDENTIALS_STORE` (`env.sh` unsets it; `pass` and `unsafe_file` are not gated,
  spec section 4).
- **If a Keychain dialog appears** (none is expected for a logged-in user, M6): Craig records its
  text. Whether to allow it is his decision.
- **If the sign-in cannot complete:** run steps 5 to 7, then STOP for Craig before tagging.
  *Brief author's call:* one retry of the sign-in is allowed first, since nothing has reached the
  account.

### B.4 On the Mac: one read-only command, then where the state landed

`/bin/bash ~/gpbmac/kit/gx.sh p4-readonly`. *Brief author's call:* it runs in the runner's
session, not Craig's Terminal, because every later script runs there; it shows that session sees
the sign-in.

```bash
# file: mac:scripts/p4-readonly.sh
# B.4: one read-only CLI command; the Keychain item now exists; every state file is in the probe
# folder and none in M7's default folders (M7).
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
"$PD" filesystem info /my-files --json > "$G/out/p4-info.json" 2> "$G/out/p4-info.err"; rc=$?
echo "probe read-only info: exit $rc"; kit esc "$G/out/p4-info.err"
[ "$rc" = 0 ] || bad "the read-only info failed"
uid=$(/usr/bin/python3 -I -c 'import json,sys; j=json.load(open(sys.argv[1])); print(j["uid"] if j.get("type") == "folder" else "")' "$G/out/p4-info.json" 2>/dev/null)
pc=$(kit getval pcroot)
echo "Mac root uid: [$uid]   PC baseline uid: [$pc]"
[ -n "$uid" ] && [ "$uid" = "$pc" ] || bad "the Mac's /my-files is not the folder the PC's baseline names"
kc_expect present "no Keychain item after the sign-in"
echo "== the probe cache folder, every file"
find "$PROTON_DRIVE_CACHE_DIR" -type f | sort | sed "s|^$PROTON_DRIVE_CACHE_DIR/|  |"
[ -n "$(find "$PROTON_DRIVE_CACHE_DIR" -type f)" ] || bad "the probe cache folder holds no file"
m7 || bad "an M7 default folder exists: the CLI wrote outside PROTON_DRIVE_CACHE_DIR"
if [ "$fail" = 0 ]; then echo "STEP 4: PASS"; else echo "STEP 4: BLOCKED"; fi
echo "HANDOFF -> PC: W.B.5"
exit "$fail"
```

**Pass:** `STEP 4: PASS`: the info exits 0, the root uid equals the PC's baseline uid, the Keychain
item is present, the probe cache folder holds files (record their names), and M7's three default
folders are absent. **Not a pass:** `You need to login first` (the session did not reach the
runner's process), a Keychain dialog, or a default folder present. For any of these: steps 5 to 7,
then STOP for Craig before tagging.

### B.5 On the PC: the Windows check again (W.B.5)

### B.6 On the Mac: `auth logout`, then the Windows check once more

`/bin/bash ~/gpbmac/kit/gx.sh p6-logout`, then, on the PC, W.B.6.

```bash
# file: mac:scripts/p6-logout.sh
# B.6: auth logout; the CLI then reports "You need to login first", and the Keychain item is gone.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
logged p6-logout "$PD" auth logout; lrc=$rc
echo "auth logout: exit $lrc"
logged p6-after "$PD" filesystem info /my-files --json; prc=$rc
echo "post-logout info: exit $prc (expected nonzero)"
kit expect "$L" login-first || bad "the post-logout info did not say 'You need to login first'"
kc_expect absent "the Keychain item remains (Craig deletes it: see below)"
if [ "$fail" = 0 ] && [ "$lrc" = 0 ] && [ "$prc" != 0 ]; then echo "logged out, item absent, $(date -u '+%Y-%m-%dT%H:%M:%SZ')" > "$G/logout.state"; echo "logout recorded"; else echo "no logout recorded (a check above failed)"; fi
if [ "$fail" = 0 ]; then echo "STEP 6 (Mac): PASS"; else echo "STEP 6 (Mac): NOT CLEAN"; fi
echo "HANDOFF -> PC: W.B.6"
exit "$fail"
```

**Pass:** `auth logout: exit 0` (record its output), the post-logout info nonzero with
`EXPECT login-first: FOUND`, `Keychain item ...: absent` and `logout recorded` (the Linux
probe's step 6 shape [record `v0.10.0-linux-gate.md`]). `logout.state` is written only when every
check passed; a Keychain query that fails (`KEYCHAIN QUERY FAILED`) is a failed check, never an
absent item.

**If the logout failed, or the item remains:** Craig deletes it locally, in his own Terminal
(section 6 step 6), and the runner records that and re-runs the check. **Craig types:**

```bash
/usr/bin/security delete-generic-password -s ch.proton.drive/drive-sdk-cli; echo "delete: exit $?"
```

*Brief author's call:* Craig, not the runner, deletes a Keychain item. Then the runner runs:

```bash
/bin/bash ~/gpbmac/kit/gx.sh p6-recheck
```

```bash
# file: mac:scripts/p6-recheck.sh
# B.6 and 7.3, after Craig's fallback delete: the item is gone and the CLI has no session. The
# probe's CLI is $PD; the gate's is the installed one.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
logged p6-recheck "${PD:-proton-drive}" filesystem info /my-files --json
kit expect "$L" login-first || bad "the CLI still has a session"
kc_expect absent "the Keychain item still exists"
if [ "$fail" = 0 ]; then echo "fallback delete (Craig), item gone, $(date -u '+%Y-%m-%dT%H:%M:%SZ')" > "$G/logout.state"; echo "RECHECK: PASS"; else echo "RECHECK: BLOCKED (STOP for Craig)"; fi
exit "$fail"
```

**If no sign-in ever completed at B.3:** the logout's output is recorded and not judged; Craig says
so, and the runner runs `/bin/bash ~/gpbmac/kit/gx.sh p6-nosignin`, which checks that there is no
session and no item and then records that, in fixed words.

```bash
# file: mac:scripts/p6-nosignin.sh
# B.6, on Craig's word that no sign-in ever completed at B.3: the probe CLI has no session and the
# Keychain item is absent; only then does logout.state record it.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
if [ -s "$G/logout.state" ]; then echo "a logout is already recorded: $(cat "$G/logout.state")"; exit 0; fi
logged p6-nosignin "$PD" filesystem info /my-files --json
kit expect "$L" login-first || bad "the CLI has a session: this is not the no-sign-in case"
kc_expect absent "the Keychain item exists: this is not the no-sign-in case"
if [ "$fail" = 0 ]; then echo "no session: Craig reports that no sign-in completed at B.3; no session, item absent, $(date -u '+%Y-%m-%dT%H:%M:%SZ')" > "$G/logout.state"; echo "NO SIGN-IN: RECORDED"; else echo "NO SIGN-IN: BLOCKED (STOP for Craig)"; fi
exit "$fail"
```

### B.7 On the Mac: delete the probe folder; then the PC decides the unpause

`/bin/bash ~/gpbmac/kit/gx.sh p7-delete`, which runs even if step 6 failed. Then W.B.7.

```bash
# file: mac:scripts/p7-delete.sh
# B.7: delete the probe folder (the probe's PROTON_DRIVE_CACHE_DIR), only after a recorded logout;
# M7's default folders must still be absent. B.0's probe agent is booted out first if it is still
# loaded (envprobe boots it out itself unless it was stopped part way).
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
if launchctl print "gui/$(id -u)/gpbmac.$RUN.envprobe" >/dev/null 2>&1; then launchctl bootout "gui/$(id -u)/gpbmac.$RUN.envprobe"; echo "booted out gpbmac.$RUN.envprobe: exit $?"; fi
if [ ! -s "$G/logout.state" ]; then echo "7 KEPT: no logout is recorded ($G/logout.state), so the probe folder stays (STOP for Craig)"; exit 1; fi
echo "logout: $(cat "$G/logout.state")"
echo "== the probe folder, before deletion"; find "$PROTON_DRIVE_CACHE_DIR" -type f | sort | sed "s|^$PROTON_DRIVE_CACHE_DIR/|  |"
rm -rf -- "$PROTON_DRIVE_CACHE_DIR"
if [ -e "$PROTON_DRIVE_CACHE_DIR" ]; then echo "BLOCKED: $PROTON_DRIVE_CACHE_DIR still exists"; exit 1; fi
echo "probe folder deleted"
m7 || bad "an M7 default folder exists"
kc_expect absent "the Keychain item exists"
if [ "$fail" = 0 ]; then echo "STEP 7 (Mac): PASS"; else echo "STEP 7 (Mac): BLOCKED"; fi
echo "HANDOFF -> PC: W.B.7"
exit "$fail"
```

**Pass:** `probe folder deleted`, the three `M7 default absent` lines, and `STEP 7 (Mac): PASS`.
`7 KEPT` means no logout was recorded: the cache folder holds the session's local state, so it
stays, the PC's tasks stay paused, and Craig decides (the Linux probe's rule, *brief author's call*).

**What the probe leaves.** On the Mac: `~/gpbmac` with the kit, the scripts, the feed-verified CLI
in `cli/`, the v0.10.0 build, and the probe's run folder (`probe-a1/`, or `probe-a2/` after a
re-run) without its cache folder (its logs are the probe's record). On the PC: `C:\gpbmac` with
its kit and `probe-a1\`, whose baseline W.0 compares with the gate's. A probe re-run (Craig's word
only) is `probe-a2`.

---

## Part C: gate setup (section 6, "Gate setup")

**When:** after the tag exists and its Release workflow has built the DRAFT (`docs/releasing.md`
step 4), after this release's Linux and Windows gates have both ended, with their cleanup, and
only if Part B passed. The Windows gate installs the draft Windows helper (its own brief), which
W.C.2 checks.

### C.0 Start the gate's run, from the tag's brief

**Mac:**

```bash
/usr/bin/git -C ~/Projects/_Tools/git-proton-backup fetch origin "refs/tags/vNEXT:refs/tags/vNEXT"
/usr/bin/python3 -I ~/gpbmac/extract.py vNEXT docs/research/gates/vNEXT-macos-gate-brief.md
/bin/bash ~/gpbmac/kit/start-run.sh gate 1
```

**PC:** W.C.0, which extracts from the tag too and starts the PC's `gate-a1`.

**Pass:** the fetch exits 0 (an existing local tag with another value is refused by git: STOP for
Craig), the extractor passes, `RUN STARTED: gate-a1 ...`, and the two sessions' `RELAY value brief`
blobs are equal. From here on every script is the tag's. A brief blob that differs from the
probe's is recorded: the brief may have been revised between the probe and the tag, and the tag's
governs.

**Then Craig starts the keep-awake** (Question 7). **Craig types,** in a new Terminal window of his
own, which he leaves open until 7.0:

```bash
/bin/bash ~/gpbmac/kit/awake.sh gate-a1
```

It prints `keep-awake: pid <n>, ...`, records that pid and its start time in `~/gpbmac/gate-a1/`
and becomes `/usr/bin/caffeinate -is -t 172800` under that pid. It starts here, not earlier: the
run folder it records into, and the tag's `awake.sh`, exist only once C.0 has run. From C.2 to step
6, `gx.sh` starts no gate script while that pid is not running caffeinate (a STOP, exit 3:
nothing ran; Craig runs the line again in his window, and the script is run). The runner never
holds it: a background command is killed at the harness's bound.

### C.1 Pause the PC's tasks for the whole gate, and take the Windows baseline (W.C.1)

W.C.1 pauses both tasks, checks that nothing runs and that no `GPB_*` or `PROTON_DRIVE_*` variable
is set, takes the gate's Windows baseline `gate-c1`, and relays `RELAY value pcroot <uid>`. The
Mac stores it with `kit.sh relayin`. The tasks stay paused until W.7.

### C.2 The tag against the anchor (Mac), the draft and the Windows helper (PC)

`/bin/bash ~/gpbmac/kit/gx.sh c2-anchor`, and on the PC W.C.2.

```bash
# file: mac:scripts/c2-anchor.sh
# C.2: every file this brief quotes or relies on is unchanged between the anchor and the tag;
# whether the release moved the CLI pin since v0.10.0 (section 6 step 5.5); and whether the
# certified CLI's versioned URL, which C.3's README line downloads, still serves it.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
A=0faada84151fde79376bac180cda32fa8277e31f
tagc=$(git -C "$MAIN" rev-parse --verify "$TAG^{commit}") || { echo "BLOCKED: no tag $TAG in the main checkout"; exit 1; }
echo "tag commit: $tagc"
if git -C "$MAIN" cat-file -e "$A^{commit}"; then echo "anchor present: yes"; else bad "the anchor commit is missing from the main checkout"; fi
if git -C "$MAIN" merge-base --is-ancestor "$A" "$tagc"; then echo "anchor is an ancestor of the tag: yes"; else echo "anchor is an ancestor of the tag: no (recorded; a squash merge gives this, and the count below is what counts)"; fi
changed=$(git -C "$MAIN" diff --name-only "$A" "$tagc" -- internal cmd go.mod go.sum scripts/macos scripts/Test-ProtonCliDrift.ps1 .gitattributes .github/workflows/release.yml) || bad "git diff failed"
n=$(printf '%s\n' "$changed" | grep -c .)
echo "files the brief quotes or relies on, changed between the anchor and the tag: $n"
printf '%s\n' "$changed" | grep . | sed 's/^/  CHANGED SINCE THE ANCHOR: /'
[ "$n" = 0 ] || bad "a file this brief quotes or relies on changed: Craig rules whether this tag's brief stands as written (Question 13)"
echo "== the two documents the brief cites by step and rule (read, not counted; a change means re-reading what the brief cites)"
git -C "$MAIN" diff --stat "$A" "$tagc" -- docs/releasing.md docs/research/gates/brief-checklist.md
echo "== every other file changed since the anchor (recorded, not judged)"
git -C "$MAIN" diff --name-only "$A" "$tagc" | sed 's/^/  /'
p10=$(git -C "$MAIN" show v0.10.0:internal/transport/cli.go | grep -E '^const CertifiedCLI = ')
pt=$(git -C "$MAIN" show "$tagc:internal/transport/cli.go" | grep -E '^const CertifiedCLI = ')
echo "v0.10.0: $p10"
echo "$TAG: $pt"
if [ -n "$p10" ] && [ "$p10" = "$pt" ]; then
  echo no > "$G/v010-needs-uncertified"
  echo "CLI PIN MOVED SINCE v0.10.0: no (step 5.5 runs v0.10.0's helper without GPB_UNCERTIFIED_CLI)"
else
  echo yes > "$G/v010-needs-uncertified"
  echo "CLI PIN MOVED SINCE v0.10.0: YES (step 5.5 runs v0.10.0's commands alone with GPB_UNCERTIFIED_CLI=1)"
fi
code=$(/usr/bin/curl -sS -L -r 0-0 -o /dev/null -w '%{http_code}' https://proton.me/download/drive/cli/0.9.0/darwin-arm64/proton-drive) || code="$code (curl exit $?)"
echo "the certified CLI's versioned download (M1; C.3's README line), one byte asked for: HTTP $code"
case "$code" in 200|206) ;; *) bad "the certified CLI's versioned URL does not serve it now, so C.3's README lines would fail: a new CLI needs a new pin, M2 and README quote" ;; esac
if [ "$fail" = 0 ]; then echo "C.2 (Mac): PASS"; exit 0; fi
echo "C.2 (Mac): STOP before any account write; it holds, nothing cleaned, until Craig rules (Question 13)"
exit 3
```

**Pass:** `anchor present: yes`, `files the brief quotes or relies on, changed between the anchor
and the tag: 0` (the count is what is judged), `the certified CLI's versioned download ...: HTTP
200` (or 206, the one byte asked for), and `C.2 (Mac): PASS`. The count covers the code and the
files whose text the brief quotes (the wrapper and plist under `scripts/macos`, the drift script's
`$CertifiedMacArm64Sha512`, the release workflow). The download check is there because the README
warns that this versioned URL may stop serving once Proton moves on [README], and a lapse found
here holds before anything is installed, where at C.3 it would come after the helper's build. Its
`curl` reaches the network but not the account [untested: this brief was written with no network
call to that URL].

**A listed file, or any other failed check:** `C.2 (Mac): STOP` (exit 3), before any account write.
It holds (Question 13): nothing is cleaned, and the PC's tasks stay paused, until Craig rules
whether this tag's brief stands as written. It cannot be re-anchored: the gate runs the tag's
brief (C.0), and tags never move (`docs/releasing.md`). If it stands, the gate goes on at C.3; if
not, step 7 runs and the Mac gate is retried at a later release (the retry rule). Step 7 then has
no CLI and no root record, and does this: 7.0 finds no writer; 7.1 prints `NO ROOT RECORD` and
`GitRemotes: this run did not create it`; 7.2's listings print `BLOCKED: no proton-drive to run`
and say why each comparison is skipped; 7.3 records `no session`; 7.4 deletes Part A's build; 7.5
checks M7; 7.6 runs the Windows check; 7.7 deletes the empty cache folder; 7.8 decides the
unpause.

*Brief author's call:* `docs/releasing.md` and `brief-checklist.md` are not in the count, because
the brief cites them by step and rule rather than quoting them. Their diff is printed, and a
change to either between the anchor and the tag means every step and rule the brief cites in it is
re-read before step 0. Both were re-read at the anchor [measured 2026-10-28]: `brief-checklist.md`
is unchanged from `119227e`, rules 1 to 12; and in `docs/releasing.md`, which plan Task 12 changed,
every place this brief cites still holds: step 3's tag, with the drift check before it; step 4's
draft and its assets; step 5's gates, its macOS gate paragraph (this gate last, the held draft, a
Mac-only failure not holding the release, a retry only at a later release, the composition steps
on the other platforms) and its rule that tags are never moved; steps 6 and 7, Craig's publish
and the closure; and "Between releases", the drift check Craig owns. The other changed files are
recorded; between the anchor and the tag they are expected to be this brief and the steps file
(their re-anchoring commit and the version's renaming) and the version flip's lines in `README.md`
and `CHANGELOG.md` [inference]. `README.md` is not counted either: C.3 and step 1 compare its
macOS install lines with this brief's quote themselves.

**The pin.** At the anchor the pin is v0.10.0's [measured 2026-10-28: no diff in `cli.go` from
`v0.10.0` to `0faada8`], so `CLI PIN MOVED SINCE v0.10.0: no` is expected. A release that moved it would also change
`internal/transport/cli.go`, which the count above lists, so C.2 would STOP for Craig's ruling
(Question 13). The script already carries section 6's rule for that case: v0.10.0's helper
refuses the new certified CLI, so its commands run with `GPB_UNCERTIFIED_CLI=1`, recorded as
such [code `cmd/git-remote-proton/main.go:31` at `v0.10.0`; spec section 6, step 5.5].
`GPB_UNCERTIFIED_CLI=1` is never set for the gated helper. For a moved pin, though, this brief
cannot stand as written: C.3's README quote, M2, `[version]` and `[cli-version]` all name the
0.9.0 CLI, so a ruling that it stands would meet C.3's STOP on the README's lines. That branch of
the script is carried for a brief written for a new pin.

### C.3 Build and install with the README's own lines, read at the tag (Mac)

`/bin/bash ~/gpbmac/kit/gx.sh c3-install`. Section 6 builds the helper "from source at the
release tag with the README's exact commands (section 1), in a fresh clone outside every other
working tree, so the gate tests the install the README gives and the build stamps a clean tree
(M27)". So C.3 reads `README.md` at the tag, from the main checkout (`git show vNEXT:README.md`),
and `kit readme` takes the one bash block of its `Install on macOS (v2)` section that clones the
tag. Its comment and blank lines are dropped, and its other lines run as written, in the run
folder, with the tag in place of `vX.Y.Z` (Question 12): the lines before the CLI download are the
helper's, and they make the tag's clone, `~/gpbmac/gate-a1/git-proton-backup-vNEXT` (`$SRC` in
`env.sh`); the rest are the CLI's, run in a download folder. Step 1 runs the CLI's lines again,
into a temp folder.

**The lines, quoted from the README at the anchor** (`0faada8`, `README.md` lines 291 to 304
[README]), as `kit readme` reads them. The gate reads them at the tag. `kit readme` compares the
SHA-256 of each part with the two below, which `lib.sh` carries (`README_HELPER_SHA` and
`README_CLI_SHA`), and prints `README LINES: AS QUOTED` only when both match. Any difference is a
STOP for adjudication before anything runs: the tag's lines are read against this brief first.

The helper's 7 lines, SHA-256 `5e9dcd5eb538cd14a3fb007b1887db027a5c06bd440bf706d239bd916d7c7b80`:

```text
TAG=vX.Y.Z
git clone --branch "$TAG" https://github.com/craigstoller/git-proton-backup.git "git-proton-backup-$TAG"
cd "git-proton-backup-$TAG"
mkdir -p ~/.local/bin
CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=$TAG" -o ~/.local/bin/git-remote-proton ./cmd/git-remote-proton
~/.local/bin/git-remote-proton --version
cd ..
```

The CLI's 2 lines, one command continued, SHA-256 `929dfd5a2fb73e5f1cce65f85f1ba8f21dccdc45cab22a5438e5daa2c80c49f0`:

```text
curl -fLo proton-drive https://proton.me/download/drive/cli/0.9.0/darwin-arm64/proton-drive \
  && echo "3f4125de5e41dcc7f4e0e07e8d60737aa18f35c6ecc1ba3fce47ea97b331857eefaf88e74671135af57122d4cb5439b817ec528f8df383550ac7810aeaf0217b  proton-drive" | shasum -a 512 -c && install -m 0755 proton-drive ~/.local/bin/proton-drive
```

*Brief author's call:* the helper's lines run under `/bin/bash -e`, so a failing line stops the
rest, as a reader at a prompt would stop; and with go1.27.0 first on PATH for that one command,
with `GOTOOLCHAIN=local`. The README asks for "Go, a current release" and names go1.27.0 as the
release the port was built and tested with on the Mac [README]; `GOTOOLCHAIN=local` keeps `go`
from fetching another toolchain, which `go.mod`'s `go 1.22` line would not ask for anyway
[inference].

```bash
# file: mac:scripts/c3-install.sh
# C.3: README "Install on macOS (v2)"'s own lines, read from README.md at the tag and checked
# against this brief's quote, then run as written in the run folder with the tag for vX.Y.Z: the
# helper built in a fresh clone of the tag, and the certified CLI installed by its checked
# download, both into ~/.local/bin.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
[ "$RUN" = gate-a1 ] || { echo "BLOCKED: $RUN is not the gate run"; exit 1; }
for f in git-remote-proton proton-drive; do
  if [ -e "$HOME/.local/bin/$f" ]; then echo "BLOCKED: ~/.local/bin/$f already exists, and this gate would overwrite it"; exit 1; fi
done
for d in "$SRC" "$G/dl" "$G/readme"; do if [ -e "$d" ]; then echo "BLOCKED: $d already exists"; exit 1; fi; done
if git -C "$G" rev-parse --git-dir >/dev/null 2>&1; then echo "BLOCKED: $G is inside a git working tree"; exit 1; fi
echo "~/.local/bin before the gate: $(if [ -d "$HOME/.local/bin" ]; then echo present; else echo absent; fi)"
echo "== README.md at the tag $TAG (main checkout): its macOS install lines"
git -C "$MAIN" show "$TAG:README.md" > "$G/out/c3-readme-at-tag.md" || { echo "BLOCKED: no README.md at $TAG"; exit 1; }
mkdir "$G/readme" || exit 1
kit readme "$G/out/c3-readme-at-tag.md" "$G/readme" "$README_HELPER_SHA" "$README_CLI_SHA" || { echo "C.3: STOP for adjudication: the README's macOS install lines at the tag are not the ones this brief quotes; nothing written, nothing run, and it holds until Craig rules"; exit 3; }
echo "== the helper: the README's lines, run in $G"
logged c3-helper env PATH="$HOME/sdk/go1.27.0/bin:$PATH" GOTOOLCHAIN=local /bin/bash -e "$G/readme/helper.sh"
echo "README helper lines: exit $rc"
if [ -e "$HOME/.local/bin/git-remote-proton" ]; then echo "$HOME/.local/bin/git-remote-proton" >> "$G/installed.txt"; fi
[ "$rc" = 0 ] || exit "$rc"
kit expect "$L" version || bad "the README's --version line did not print the tag and the certified token"
c=$(git -C "$SRC" rev-parse HEAD); t=$(git -C "$MAIN" rev-parse "$TAG^{commit}")
echo "the tag's clone: $SRC"; echo "clone commit: $c"; echo "tag commit:   $t"
[ "$c" = "$t" ] || { echo "BLOCKED: the clone is not at the tag's commit"; exit 1; }
echo "untracked or modified in the clone: [$(git -C "$SRC" status --porcelain --untracked-files=all)]"
echo "== the CLI: the README's lines, run in $G/dl"
mkdir "$G/dl" && cd "$G/dl" || exit 1
logged c3-cli /bin/bash "$G/readme/cli.sh"
cd "$G" || exit 1
echo "README CLI lines: exit $rc"
if [ -e "$HOME/.local/bin/proton-drive" ]; then echo "$HOME/.local/bin/proton-drive" >> "$G/installed.txt"; fi
[ "$rc" = 0 ] || { echo "C.3: STOP: the README's CLI lines exited $rc; a failed download or check installs nothing. A certified CLI gone or re-cut since C.2 needs a new pin, M2 and README quote; it holds until Craig rules"; exit 3; }
grep -q 'proton-drive: OK' "$L" || bad "no 'proton-drive: OK' from shasum"
s=$(shasum -a 512 < "$HOME/.local/bin/proton-drive" | cut -d' ' -f1); echo "installed CLI SHA-512: $s"
[ "$s" = "$M2" ] || bad "the installed CLI is not M2's bytes"
rm -rf "$G/dl"
for f in git-remote-proton proton-drive; do
  if xattr -p com.apple.quarantine "$HOME/.local/bin/$f" >/dev/null 2>&1; then bad "$f carries com.apple.quarantine"; else echo "quarantine on $f: none"; fi
done
codesign -dv "$HOME/.local/bin/git-remote-proton" 2>&1 | grep -E '^(Identifier|CodeDirectory|Signature)'
PATH="$HOME/sdk/go1.27.0/bin:$PATH" go version -m "$HOME/.local/bin/git-remote-proton"
type -a git-remote-proton proton-drive
if [ "$fail" = 0 ]; then echo "C.3: PASS"; else echo "C.3: BLOCKED"; fi
exit "$fail"
```

**Pass:** `C.3: PASS`, with all of the following.

- Before writing anything, the script BLOCKs if either binary already exists in `~/.local/bin`
  [measured 2026-10-27: neither does], if the tag's clone, `dl` or `readme` already exists in the
  run folder, or if the run folder is inside a git working tree.
- `README helper lines: 7, SHA-256 <the quote's> (as quoted)`, `README cli lines: 2, SHA-256 <the
  quote's> (as quoted)` and `README LINES: AS QUOTED`. `NOT AS QUOTED`, or a `BLOCKED:` line from
  `kit readme`: `C.3: STOP for adjudication` (exit 3), and nothing has run.
- `README helper lines: exit 0` and `EXPECT version: FOUND` (the README's own `--version` line);
  the tag's clone at the tag's commit, and the `untracked or modified` list empty.
- `README CLI lines: exit 0` (any other exit is `C.3: STOP`, exit 3), `proton-drive: OK` from
  `shasum`, and the installed CLI's SHA-512 M2's value; neither binary quarantined (M2, M8, M20);
  and `codesign` showing the helper
  `adhoc,linker-signed` (M9), recorded.
- `go version -m` shows `-trimpath=true`, `CGO_ENABLED=0`, `GOOS=darwin`, `GOARCH=arm64`,
  `vcs.revision=<the tag's commit>` and `vcs.modified=false` (M27). Record its first line, the
  toolchain.
- `type -a` shows exactly `git-remote-proton is /Users/craig/.local/bin/git-remote-proton` and
  `proton-drive is /Users/craig/.local/bin/proton-drive`.

A helper line that fails after the build line (the `--version` line, say) leaves the built helper
installed: `installed.txt` lists each binary that exists, whatever the exit, and C.3 is BLOCKED.

**A C.3 STOP** (exit 3: the README's lines at the tag are not the ones quoted, or its CLI lines
failed) holds as C.2's does (Question 13): nothing is cleaned and the PC's tasks stay paused until
Craig rules, and nothing has reached the account. The tag's brief runs only the README lines it
quotes (Question 12), so with other lines at the tag the gate cannot go on at this release. For a
failed CLI download Craig rules whether it was transient (C.3 then runs again once, Question 3,
after he moves aside what it made, each of which C.3 refuses to find: the binaries `installed.txt`
lists, and `$SRC`, `readme` and `dl` in the run folder) or the certified CLI is gone or re-cut,
which needs a new pin, M2 and README quote. Step 7 runs only on his ruling, as after a C.2 STOP.

**These binaries stay after the gate.** Section 6's cleanup does not remove them, and real use
starts from them ("Afterwards"). *Brief author's call, following section 6 step 7's list.* After a
failed gate, removing them is Craig's call: `rm ~/.local/bin/git-remote-proton
~/.local/bin/proton-drive`.

### C.4 Craig signs in, as in the probe

**Craig types,** in a new Terminal window:

```bash
source ~/gpbmac/gate-a1/env.sh
proton-drive auth login; echo "auth login: exit $?"
```

The same expectations, authorisations and exclusions as B.3 apply, and so does the rule for the
record (a sign-in URL stripped of its query string and fragment). Then the runner runs
`/bin/bash ~/gpbmac/kit/gx.sh c4-check`:

```bash
# file: mac:scripts/c4-check.sh
# C.4: the gate session works from the runner's process, its state is in the gate cache folder,
# and M7's default folders are absent.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
proton-drive filesystem info /my-files --json > "$G/out/c4-info.json" 2> "$G/out/c4-info.err"; rc=$?
echo "read-only info: exit $rc"; kit esc "$G/out/c4-info.err"; [ "$rc" = 0 ] || bad "the read-only info failed"
kc_expect present "no Keychain item"
echo "== the gate cache folder"; find "$PROTON_DRIVE_CACHE_DIR" -type f | sort | sed "s|^$PROTON_DRIVE_CACHE_DIR/|  |"
[ -n "$(find "$PROTON_DRIVE_CACHE_DIR" -type f)" ] || bad "the gate cache folder holds no file"
m7 || bad "an M7 default folder exists"
if [ "$fail" = 0 ]; then echo "C.4: PASS"; else echo "C.4: BLOCKED"; fi
exit "$fail"
```

**Pass:** `C.4: PASS`. A sign-in that does not complete is fixed and retried once inside
`gate-a1` (nothing has reached the account; see the retry rule). A second failure: step 7, then
STOP for Craig.

### C.5 The trash (rule 6, and a deliberate deviation from it)

As in both v0.10.0 briefs, following ruling 4 of `docs/research/gates/v0.9.0-release-gate.md`:
**if the trash holds nothing of Craig's,** Craig empties it in the web UI before step 0, and rule 6
holds as written. **If it holds something of his,** it stays, and so does everything this gate
trashes. *Brief author's call, following ruling 4:* emptying the trash permanently deletes
whatever of Craig's is in it, which no ruling authorises. Permanent deletion afterwards is Craig's
alone, of items he identifies as this gate's; the runner gives him the ownership record and never
deletes anything permanently. Whether the CLI can list the trash at all is uncertain (the Stage 1
probes found no trash enumeration; 0.9.0 listed zero rows from an emptied trash), so every
`/trash` snapshot here is supplementary: one that fails or shows none of the gate's items is
recorded, not BLOCKED.

When C.3 to C.5 are done (the trash decided), the runner tells the PC session `HANDOFF -> PC: W.0`
(row 12 of Part W's table).

---

## Part D: the gate (section 6, "Gate content", steps 0 to 7)

Section 6: "following Linux section 7, with its write confinement and verify-before-trash rules".
Each step below names who runs what. Every Mac script is run as `/bin/bash ~/gpbmac/kit/gx.sh
<name>`, under the timeouts above.

### Step 0: preconditions, asserted

**PC first:** W.0 (the Windows helper is the draft's, the Windows check `gate-0`, the probe-to-gate
boundary, and the draft still a draft), then `HANDOFF -> MAC: step 0`. **Then the Mac:**
`gx.sh s0-pre`.

```bash
# file: mac:scripts/s0-pre.sh
# Step 0: section 6's preconditions, and the rule 1 baseline. The git version comes first: another
# git is a STOP before anything is read or recorded (Question 6).
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
echo "git: $(git --version)"
if [ "$(git --version)" != "git version 2.54.0 (Apple Git-157)" ]; then echo "STOP for adjudication: the git is not the 2.54.0 this brief measured (nothing recorded)"; exit 3; fi
have curl shasum xattr codesign security launchctl osascript pgrep caffeinate pmset plutil cksum od ioreg || bad "missing tool(s), named above"
echo "variables inherited by a gate script: ${INHERITED:-none}"
[ -z "$INHERITED" ] || bad "GPB_, PROTON_DRIVE_, GIT_CONFIG or object-placement GIT_ variables reach gate scripts"
echo "== the gate PATH: $PATH"
type -a git-remote-proton proton-drive git
[ "$(type -ap git-remote-proton)" = "$HOME/.local/bin/git-remote-proton" ] || bad "git-remote-proton does not resolve to exactly one path, the one just built"
[ "$(type -ap proton-drive)" = "$HOME/.local/bin/proton-drive" ] || bad "proton-drive does not resolve to exactly one path, the certified CLI"
ep=$(git --exec-path); echo "git --exec-path: $ep"
if ls "$ep" | grep -q '^git-remote-proton'; then ls -la "$ep"/git-remote-proton*; bad "a git-remote-proton sits in git's exec-path, which git searches before PATH"; fi
GOT=$(shasum -a 512 < "$HOME/.local/bin/proton-drive" | cut -d' ' -f1)
CONST=$(tr -d '\r' < "$SRC/scripts/Test-ProtonCliDrift.ps1" | grep -oE "^[$]CertifiedMacArm64Sha512[[:space:]]*=[[:space:]]*'[0-9a-f]{128}'[[:space:]]*$" | grep -oE '[0-9a-f]{128}')
echo "installed CLI SHA-512:                     $GOT"
echo "the tag's \$CertifiedMacArm64Sha512:        ${CONST:-NONE}"
echo "M2 (spec):                                 $M2"
[ "$GOT" = "$M2" ] && [ "$CONST" = "$M2" ] || bad "the CLI is not the certified build, or the tag's constant is not M2's"
logged 0-cli-version proton-drive --version; kit expect "$L" cli-version || bad "the CLI's --version"
logged 0-version git-remote-proton --version; kit expect "$L" version || bad "the helper's --version"
PATH="$HOME/sdk/go1.27.0/bin:$PATH" go version -m "$HOME/.local/bin/git-remote-proton" > "$G/out/0-buildinfo.txt"; cat "$G/out/0-buildinfo.txt"
grep -q "vcs.revision=$(git -C "$SRC" rev-parse HEAD)" "$G/out/0-buildinfo.txt" || bad "vcs.revision is not the tag's commit"
grep -q 'vcs.modified=false' "$G/out/0-buildinfo.txt" || bad "vcs.modified is not false"
echo "installed helper SHA-256: $(shasum -a 256 < "$HOME/.local/bin/git-remote-proton" | cut -d' ' -f1)"
[ -x "$GM/v010-build/bin/git-remote-proton" ] && echo "v0.10.0 build: present at $GM/v010-build/bin, not on PATH" || bad "the v0.10.0 build is missing (Part A)"
echo "v0.10.0's commands need GPB_UNCERTIFIED_CLI=1: $(cat "$G/v010-needs-uncertified" 2>/dev/null || echo 'UNKNOWN (C.2 did not run)')"
echo "== git configuration in force for gate scripts (system and global switched off)"
git config --show-origin --list | grep -v -E '^file:\.git/config' || echo "  (none outside repositories)"
echo "== the gate cache folder: $PROTON_DRIVE_CACHE_DIR"
proton-drive filesystem info /my-files --json > "$G/out/0-info.json" 2> "$G/out/0-info.err"; rc=$?
echo "Mac read-only info: exit $rc"; kit esc "$G/out/0-info.err"; [ "$rc" = 0 ] || bad "the Mac session cannot read /my-files"
uid=$(/usr/bin/python3 -I -c 'import json,sys; j=json.load(open(sys.argv[1])); print(j["uid"] if j.get("type") == "folder" else "")' "$G/out/0-info.json" 2>/dev/null)
pc=$(kit getval pcroot)
echo "Mac root uid: [$uid]   PC gate baseline uid: [$pc]"
[ -n "$uid" ] && [ "$uid" = "$pc" ] || bad "the Mac's /my-files is not the folder the PC's baseline names"
[ -n "$(find "$PROTON_DRIVE_CACHE_DIR" -type f)" ] || bad "the gate cache folder holds no file"
m7 || bad "an M7 default folder exists"
kc_expect present "no Keychain item"
curl -fsS -o /dev/null -w 'network: Proton feed HTTP %{http_code}\n' https://proton.me/download/drive/cli/version.json || bad "no network: the Proton feed did not answer"
echo "== the tag's contract-test untouchables (six expected)"
grep -E '^[[:space:]]+"(GitBackups|Sensitive Project Sources|Project Repo Bundles|ChatGPT Export Text Backup|Tax Documents|Archive)":[[:space:]]+true,' "$SRC/internal/transport/contract_test.go" | tee "$G/out/0-untouchables.txt"
[ "$(wc -l < "$G/out/0-untouchables.txt" | tr -d ' ')" = 6 ] || bad "the tag's contract test does not list the six untouchables"
echo "== power, recorded here and required at step 6: $(pmset -g batt | head -1)"
awake_ok || bad "the keep-awake is not running (Craig starts it at C.0)"
kit absentnode "$ABSENT" || bad "the never-created parent of step 6 exists"
kit roots-start || bad "the ownership record could not be started"
kit snapshot myfiles-0 /my-files || bad "the /my-files baseline (rule 1)"
kit snapshot casprobe-0 /my-files/_cas-probe || echo "CASPROBE NOT LISTABLE (recorded; but roots-start needs it, see above)"
kit snapshot trash-0 /trash || echo "TRASH NOT LISTABLE (recorded, not a BLOCK; see C.5)"
if grep -q $'^GitRemotes\t[^-]' "$G/roots.tsv" 2>/dev/null; then kit snapshot gitremotes-0 "$R" || bad "the GitRemotes baseline"; fi
if [ "$fail" = 0 ]; then echo "STEP 0 (Mac): PASS"; else echo "STEP 0 (Mac): BLOCKED"; fi
exit "$fail"
```

**Pass (Mac):** `STEP 0 (Mac): PASS`, and every one of the following. Together with W.0 they are
all of section 6's step 0 list.

- `type -a` finds exactly one `git-remote-proton`, the one just built, and exactly one
  `proton-drive`, the certified CLI; git's exec-path holds no `git-remote-proton*` [measured
  2026-10-27: none].
- The installed CLI's SHA-512, the tag's `$CertifiedMacArm64Sha512` and M2's value are one value;
  `EXPECT cli-version: FOUND` and `EXPECT version: FOUND` (`git-remote-proton vNEXT (certified CLI:
  cli-drive@0.9.0+75612299)`).
- `go version -m` shows the tag's commit as `vcs.revision` and `vcs.modified=false` (M27). Record
  the helper's SHA-256.
- `git version 2.54.0 (Apple Git-157)`, the script's first check. Another version stops it at
  once (exit 3, `STOP for adjudication ... (nothing recorded)`), before any account read, root
  record or baseline: M24, PM1 and R7 were measured on 2.54.0. Whether step 0 then runs again is
  Craig's call (the retry rule, Question 6).
- The configuration list holds no `file:` origin outside `.git/config` (system and global are off).
- The gate cache folder is in use (it holds the session's files) and M7's defaults are absent;
  this is re-checked after step 6 (6.5).
- The Mac's root uid equals the PC's gate baseline uid: one account.
- The network answers, and the tag's contract test lists the six untouchables.
- The v0.10.0 build is present and not on PATH, the one second helper the gate uses (step 5.5).
- `keep-awake: pid <n>, /usr/bin/caffeinate, running`: the keep-awake C.0 started, found by the
  pid it recorded, not by name.
- `ABSENT NODE: .../vNEXT-mac-absent-a1`.
- **The ownership record is started:** `_cas-probe: PRESENT ... contract-mac: ABSENT`, and either
  `GitRemotes: ABSENT at step 0 ...` or `GitRemotes: PRESENT at step 0 ...` (then `gitremotes-0`
  is saved). An `EARLIER ROOT` line is recorded and that root left alone. Any `BLOCKED:` line from
  `roots-start` is BLOCKED: no record was written, so 7.1 trashes nothing.
- `myfiles-0` shows the six untouchables and `_cas-probe`, and no row with a `trashTime` (rule 1's
  baseline). `trash-0` is rule 7's baseline, or `TRASH NOT LISTABLE` is recorded.

### Step 1: offline checks

`gx.sh s1-offline`. Section 6: "the helper's `--version` shows the tag and the certified token; the
README's CLI commands, run as written into a temp folder, give bytes equal to the installed CLI's."
The README is read at the tag again, from the tag's clone, with C.3's `kit readme` and C.3's quote.

```bash
# file: mac:scripts/s1-offline.sh
# Step 1: the helper's --version, and README "Install on macOS (v2)"'s CLI lines, read by kit
# readme from the README at the tag (the tag's clone) and checked against C.3's quote, then run as
# written into a temp folder with HOME inside it.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
logged 1-version git-remote-proton --version; kit expect "$L" version || bad "the helper's --version"
T="$G/t1"
if [ -e "$T" ]; then echo "BLOCKED: $T already exists"; exit 1; fi
mkdir -p "$T/home/.local/bin" "$T/readme" || exit 1
kit readme "$SRC/README.md" "$T/readme" "$README_HELPER_SHA" "$README_CLI_SHA" || { echo "STEP 1: STOP for adjudication: the README's macOS install lines at the tag are not the ones C.3 quotes; nothing written, nothing run, and it holds until Craig rules"; exit 3; }
grep -qF 'curl -fLo proton-drive https://proton.me/download/drive/cli/0.9.0/darwin-arm64/proton-drive' "$T/readme/cli.sh" || bad "the README's curl line does not name the versioned darwin-arm64 URL (M1)"
grep -qF "&& echo \"$M2  proton-drive\" | shasum -a 512 -c && install -m 0755 proton-drive ~/.local/bin/proton-drive" "$T/readme/cli.sh" || bad "the README's check line does not carry M2's value, chained to the install"
PRE=$(shasum -a 512 < "$HOME/.local/bin/proton-drive" | cut -d' ' -f1)
cd "$T" || exit 1
logged 1-cli env HOME="$T/home" /bin/bash "$T/readme/cli.sh"
cd "$G" || exit 1
echo "README CLI lines: exit $rc"; [ "$rc" = 0 ] || bad "the README's CLI lines exited $rc"
grep -q 'proton-drive: OK' "$L" || bad "no 'proton-drive: OK' from shasum"
for f in "$T/proton-drive" "$T/home/.local/bin/proton-drive" "$HOME/.local/bin/proton-drive"; do
  s=$( [ -f "$f" ] && shasum -a 512 < "$f" | cut -d' ' -f1 || echo '(no file)')
  echo "SHA-512 $s  $f"; [ "$s" = "$M2" ] || bad "$f is not the certified bytes"
done
[ "$PRE" = "$M2" ] || bad "the installed CLI changed"
echo "== a new login shell's view (recorded, not judged)"
env -i HOME="$HOME" /bin/zsh -lc 'type -a git-remote-proton proton-drive'
if [ "$fail" = 0 ]; then echo "STEP 1: PASS"; else echo "STEP 1: BLOCKED"; fi
exit "$fail"
```

**Pass:** `STEP 1: PASS`, which means:

- `EXPECT version: FOUND`: the tag and the certified token (section 6).
- **The README's CLI lines,** read from the README at the tag (`$SRC/README.md`, the tag's
  clone): `README LINES: AS QUOTED`, both parts equal to C.3's quote, and neither `grep` check
  failing: the `curl` line names the versioned `darwin-arm64` URL (M1), and the check line carries
  M2's value, chained with `&&` to the install (spec section 1, Amendment A3's shape). Run as
  written in a temp folder with `HOME` inside it, they exit 0, print `proton-drive: OK`, and every
  SHA-512 listed (the download, the temp install, the installed CLI) is M2's value. *Brief
  author's call:* the temp `HOME` keeps them from rewriting `~/.local/bin`, which step 0 verified,
  as the Linux brief ran its Block 1; `~/.local/bin` is made in the temp `HOME` first, as the
  README's helper lines make it before its CLI lines run [README].
- **The login-shell view** is recorded, not judged: Craig's `~/.zprofile` adds `~/.local/bin`
  twice [measured 2026-10-27, lines 7 and 11], so a path may print twice. A README finding, if
  any, is reported to Craig.

`STEP 1: STOP for adjudication` (exit 3) holds for Craig's ruling, as a C.3 STOP does: step 0 has
read the account and started the root record, and nothing has been written to it. It cannot follow
a C.3 pass unless the tag's clone changed since [inference], which is itself a finding.

### Step 2: the live contract table from the Mac, under its own root

Section 6: "once, under its own root, `GPB_CONTRACT_LIVE_ROOT=/my-files/_cas-probe/contract-mac`,
named in the brief and the confinement list (rule 8)".

**2.0 Create `/my-files/GitRemotes`, only if step 0 found it absent:** `gx.sh s2-0-gitremotes`.

```bash
# file: mac:scripts/s2-0-gitremotes.sh
# Step 2.0: /my-files/GitRemotes, created with the CLI's two-argument form, only when step 0
# recorded it absent. Its "EXIT s2-0-gitremotes = 0" is what lets 7.1 trash it later.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
if ! grep -q $'^GitRemotes\t-\tabsent-at-step-0\t' "$G/roots.tsv" 2>/dev/null; then echo "2.0 SKIPPED: step 0 did not record GitRemotes absent"; exit 0; fi
kit absentnode "$R" || { echo "BLOCKED: $R exists now, but step 0 found it absent"; exit 1; }
proton-drive filesystem create-folder /my-files GitRemotes; rc=$?
echo "create-folder /my-files GitRemotes: exit $rc"
exit $rc
```

**Pass:** `2.0 SKIPPED ...` when step 0 found `GitRemotes` present; otherwise `create-folder
/my-files GitRemotes: exit 0` and `EXIT s2-0-gitremotes = 0`. The two-argument form is the CLI's
grammar; a one-path form fails with `Expected 2 arguments, got 1` [record `stage6-gate.md` run 2].

**2.1 The table:** `gx.sh s2-1-contract`, in the background with at least 90 minutes (rule 9).

```bash
# file: mac:scripts/s2-1-contract.sh
# Step 2.1: the 17-row live contract table at the tag, from the Mac, under contract-mac.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
kit roots-update || exit 1
if grep -q $'^GitRemotes\t[^\t]*\tcreated-by-2.0\t' "$G/roots.tsv" && [ ! -e "$G/snap/gitremotes-0.json" ]; then kit snapshot gitremotes-0 "$R" || exit 1; fi
cd "$SRC" || exit 1
echo "commit under test: $(git rev-parse HEAD)"
export PATH="$HOME/sdk/go1.27.0/bin:$PATH" GOTOOLCHAIN=local
echo "toolchain for this table: $(go version)"
echo "GPB_CONTRACT_LIVE_ROOT for this one command: $CONTRACT"
env GPB_LIVE_ACCOUNT=1 GPB_CONTRACT_LIVE_ROOT="$CONTRACT" go test ./internal/transport/ -run '^TestContractCLI$' -count=1 -timeout 40m -v; rc=$?
echo "go test: exit $rc"
echo "GPB_ variables left in this shell: $(env | grep '^GPB_' || echo none)"
exit $rc
```

`GPB_LIVE_ACCOUNT` and `GPB_CONTRACT_LIVE_ROOT` are set for this one command only (rules 5, 8 and
9). The test validates the root before any live call: strictly under `/my-files/`, no empty, `.` or
`..` segment, and outside the six untouchables [code `internal/transport/contract_test.go:57-83`];
`/my-files/_cas-probe/contract-mac` passes. With `GPB_LIVE_ACCOUNT=1` the package installs no trap
and uses the real CLI on purpose [spec section 5]: this is not a hermetic run, so rule 12 does not
bar it.

**Pass:** all of the following hold.

- `commit under test` is the tag's commit; the toolchain line reads `go version go1.27.0 ...`
  (record it; it need not equal the build's toolchain).
- 17 lines **contain** `--- PASS: TestContractCLI/` (`go -v` indents them), among them
  `--- PASS: TestContractCLI/download_of_a_directory_recursively_materialises_the_subtree_(F1)`.
- `--- PASS: TestContractCLI (` with a duration of hundreds of seconds (605 to 614 s on Windows
  [record `cli-0.8.0-certification.md:320`]). A sub-second duration is a skip: BLOCKED.
- No line contains `SKIPPING THE LIVE HALF`, `--- FAIL` or `--- SKIP`; a line holds `ok` and
  `github.com/craigstoller/git-proton-backup/internal/transport`; `go test: exit 0`;
  `GPB_ variables left in this shell: none`.

**2.2 Then:** `gx.sh s2-2-after`, and on the PC W.2.

```bash
# file: mac:scripts/s2-2-after.sh
# Step 2.2: the override root was used, and emptied by the table's own cleanups.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
kit roots-update || exit 1
kit snapshot contract-2 "$CONTRACT" || exit 1
kit snapshot casprobe-2 /my-files/_cas-probe && kit compare casprobe-0 casprobe-2 || echo "CASPROBE COMPARISON NOT AVAILABLE (recorded)"
echo "HANDOFF -> PC: W.2"
```

**Pass:** `RECORDED: /my-files/_cas-probe/contract-mac (uid ...)` and `contract-2` listing 0 rows
(the table's `EnsureDir(base)` made the root [code `contract_test.go:627`], and each per-test
root's cleanup trashed it). A row with a `trashTime` is lag, already re-taken; one without is
BLOCKED. `casprobe`: `1 difference(s)` with exactly `ADDED /my-files/_cas-probe/contract-mac`, or
`NO DIFFERENCE` if the listing has not caught up, both recorded. W.2's `WINDOWS CHECK gate-2 ...
-> PASS` shows the PC's session survived the table before step 4 depends on it.

### Step 3: smoke (bootstrap push, clone with `fsck`, incremental fetch, `--set-head`)

All four scripts work on `$SMOKE` (`/my-files/GitRemotes/vNEXT-mac-smoke-a1`). Each commit carries
real file content, so packs hold blobs and `fsck` checks something. *Brief author's call, the
v0.10.0 briefs'.*

**3.1 The bootstrap push:** `gx.sh s3a-bootstrap`.

```bash
# file: mac:scripts/s3a-bootstrap.sh
# Step 3.1: the bootstrap push.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
cd "$G" || exit 1
if [ -e smoke ]; then echo "BLOCKED: $G/smoke already exists"; exit 1; fi
gate_repo smoke && commit_in smoke a.txt 'vNEXT mac smoke 1' 'smoke 1' || exit 1
git -C smoke remote add proton-v2 "proton::$SMOKE" || exit 1
echo "local main: $(git -C smoke rev-parse main)"
logged 3a-push git -C smoke push -u proton-v2 main
echo "push: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
kit helperlines "$L"
kit roots-update || bad "the ownership record"
exit "$fail"
```

**Expected:** `EXIT s3a-bootstrap = 0`, with `To proton::/my-files/GitRemotes/vNEXT-mac-smoke-a1`,
`* [new branch]` and `main -> main`, `branch 'main' set up to track 'proton-v2/main'.`, and
`gpb: downloaded /my-files/GitRemotes/vNEXT-mac-smoke-a1/.lock (<n> bytes)` (the lock's read-back;
other `gpb: downloaded` lines may appear). `ABSENT d5-too-old: YES` and `ABSENT d5-unknown: YES`
(or a first "could not check" recorded instead, `d5_check`); `kit helperlines`' `others: 0` (a
known line, string 17 or that first "could not check", is listed `KNOWN:` and recorded); and
`RECORDED: .../vNEXT-mac-smoke-a1 (uid ...)`.

**3.2 Clone, with `git fsck --full`:** `gx.sh s3b-clone`.

```bash
# file: mac:scripts/s3b-clone.sh
# Step 3.2: a clone of the smoke remote, with git fsck --full.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
cd "$G" || exit 1
if [ -e smoke-clone ]; then echo "BLOCKED: $G/smoke-clone already exists"; exit 1; fi
logged 3b-clone git clone -o proton-v2 "proton::$SMOKE" smoke-clone
echo "clone: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
kit helperlines "$L"
git -C smoke-clone fsck --full; rc=$?
echo "fsck --full: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
b=$(git -C smoke-clone branch --show-current); c=$(git -C smoke-clone rev-parse HEAD); o=$(git -C smoke rev-parse main)
echo "clone's branch: $b"; echo "clone HEAD:     $c"; echo "origin main:    $o"
[ "$b" = main ] && [ "$c" = "$o" ] || { echo "BLOCKED: the clone is not on main at origin main's commit"; exit 1; }
echo "ref format: $(git -C smoke-clone rev-parse --show-ref-format)"
```

**Expected:** `EXIT s3b-clone = 0`: `Cloning into 'smoke-clone'...`, `gpb: downloaded
.../packs/pack-<hex>.pack` and `.idx` lines, `clone: exit 0`, `fsck --full` printing nothing and
`exit 0`, the branch `main` and two equal SHAs, no `git-remote-proton:` line, and `ref format:
files`. This is the Mac `files` clone section 3's rules judge: nothing on this remote collides, so
`list` lets it through.

**3.3 An incremental fetch:** `gx.sh s3c-incremental`.

```bash
# file: mac:scripts/s3c-incremental.sh
# Step 3.3: a second commit pushed, and fetched by the clone.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
commit_in "$G/smoke" a.txt 'vNEXT mac smoke 2' 'smoke 2' || exit 1
logged 3c-push git -C "$G/smoke" push proton-v2 main
echo "push: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
logged 3c-fetch git -C "$G/smoke-clone" fetch proton-v2
echo "fetch: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
c=$(git -C "$G/smoke-clone" rev-parse proton-v2/main); o=$(git -C "$G/smoke" rev-parse main)
echo "clone proton-v2/main: $c"; echo "origin main:          $o"
[ "$c" = "$o" ] || { echo "BLOCKED: proton-v2/main is not origin main"; exit 1; }
git -C "$G/smoke-clone" fsck --full; rc=$?
echo "fsck --full: exit $rc"
exit $rc
```

**Expected:** `EXIT s3c-incremental = 0`: the push prints `..` and `main -> main`, the fetch a line
with `-> proton-v2/main`, the SHAs are equal, and `fsck --full: exit 0`. Which packs the fetch
downloaded is recorded, not asserted.

**3.4 `--set-head`:** `gx.sh s3d-set-head`.

```bash
# file: mac:scripts/s3d-set-head.sh
# Step 3.4: --set-head, then a fresh clone checks out the new default branch.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
if [ -e "$G/smoke-head" ]; then echo "BLOCKED: $G/smoke-head already exists"; exit 1; fi
logged 3d-side git -C "$G/smoke" push proton-v2 main:refs/heads/side
echo "push side: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
logged 3d-set-head git-remote-proton --set-head "proton::$SMOKE" side
echo "set-head: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
kit expect "$L" set-head || bad "the --set-head report"
cd "$G" && logged 3d-clone git clone -o proton-v2 "proton::$SMOKE" smoke-head
echo "clone: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
b=$(git -C "$G/smoke-head" branch --show-current); echo "fresh clone's branch: $b"
[ "$b" = side ] || { echo "BLOCKED: the fresh clone is not on side"; exit 1; }
exit "$fail"
```

**Expected:** `EXIT s3d-set-head = 0`: `* [new branch]` and `main -> side`, `EXPECT set-head:
FOUND` (string 15, `HEAD is now refs/heads/side`), `clone: exit 0`, and `fresh clone's branch:
side`.

### Step 4: cross-platform, Mac and PC

Repo **XM** (`$XM`, `vNEXT-mac-xm-a1`) is pushed from the Mac and cloned on the PC; repo **XW**
(`$XW`, `vNEXT-mac-xw-a1`) is pushed from the PC and cloned on the Mac. Each clone gets `fsck` and a
ref-set comparison; then each origin pushes an update and a branch deletion, and the other side's
existing clone fetches them (section 6). *Brief author's call:* the clones fetch with `--prune`, so
the deletion shows in their refs, as in the v0.10.0 Linux brief.

**The cross-machine ref-set comparison.** Every `git ls-remote` and every clone's
`for-each-ref refs/remotes/proton-v2` is saved under its side's `refsets`; a clone's
`refs/remotes/proton-v2/<b>` is compared as `refs/heads/<b>`, and HEAD is left out whenever either
side is a clone (fetch does not move a clone's `proton-v2/HEAD`). Within one machine the kit
compares two files; across machines it compares a set with the other side's relayed digest
(`RELAY refset <label> <clone|remote> <n> <sha256> <n without HEAD> <sha256 without HEAD>`).
*Brief author's call*, the v0.10.0 Linux brief's comparison, carried across two machines by digest.

**4.1 Push XM from the Mac:** `gx.sh s4a-push-xm`.

```bash
# file: mac:scripts/s4a-push-xm.sh
# Step 4.1: repo XM pushed from the Mac; its remote ref set saved and relayed to the PC.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
cd "$G" || exit 1
if [ -e xm ]; then echo "BLOCKED: $G/xm already exists"; exit 1; fi
gate_repo xm && commit_in xm m.txt 'pushed from the Mac 1' 'xm 1' || exit 1
git -C xm switch -q -c topic-m && commit_in xm t.txt 'topic from the Mac' 'xm topic' && git -C xm switch -q main || exit 1
git -C xm remote add proton-v2 "proton::$XM" || exit 1
logged 4a-push git -C xm push -u proton-v2 main topic-m
echo "push: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
git ls-remote "proton::$XM" > "$G/refsets/xm-4a-mac.txt"; rc=$?
echo "ls-remote: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
kit refshow xm-4a-mac
kit roots-update || exit 1
kit relay xm-4a-mac
echo "HANDOFF -> PC: W.4.2"
```

**Expected:** `EXIT s4a-push-xm = 0`: two `[new branch]` lines (`main -> main`, `topic-m ->
topic-m`), `push: exit 0`, the ref set `HEAD`, `refs/heads/main` and `refs/heads/topic-m` (HEAD at
main's SHA), `RECORDED: .../vNEXT-mac-xm-a1`, and one `RELAY refset xm-4a-mac remote 3 ...` line for
Craig to carry to the PC. The `ls-remote` here runs from the run folder, which no repository
encloses (C.3 checked), so the Mac's `list` only warns, and there is nothing to warn about.

**4.2 and 4.3, on the PC:** W.4.2 clones XM on Windows, with `fsck` and both comparisons; W.4.3
pushes XW from Windows and relays `RELAY refset xw-4c-win remote 3 ...`. The Mac stores it
(`kit.sh relayin '<line>'`).

**4.4 Clone XW on the Mac:** `gx.sh s4d-clone-xw`.

```bash
# file: mac:scripts/s4d-clone-xw.sh
# Step 4.4: repo XW, pushed from Windows, cloned on the Mac; fsck and both comparisons.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
kit roots-update || exit 1
cd "$G" || exit 1
if [ -e xw-clone ]; then echo "BLOCKED: $G/xw-clone already exists"; exit 1; fi
logged 4d-clone git clone -o proton-v2 "proton::$XW" xw-clone
echo "clone: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
kit helperlines "$L"
git -C xw-clone fsck --full; rc=$?
echo "fsck --full: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
git -C xw-clone for-each-ref --format='%(objectname) %(refname)' refs/remotes/proton-v2 > "$G/refsets/xw-4d-macclone.txt" || exit 1
git ls-remote "proton::$XW" > "$G/refsets/xw-4d-mac.txt" || exit 1
kit refcmp xw-4d-macclone xw-4d-mac || bad "the Mac clone's refs are not the remote's"
kit relaycmp xw-4d-mac xw-4c-win || bad "the Mac's view of XW is not the PC's"
exit "$fail"
```

**Pass:** `RECORDED: .../vNEXT-mac-xw-a1 (uid ...)` (the PC created it), `clone: exit 0`, no
`git-remote-proton:` line, `fsck --full` silent with `exit 0`, `REFSET xw-4d-macclone == xw-4d-mac:
EQUAL (2 ref(s), HEAD left out)` and `RELAYED REFSET xw-4c-win vs xw-4d-mac: EQUAL (3 ref(s))`.

**4.5 The Mac pushes an update and a branch deletion to XM:** `gx.sh s4e-update-xm`.

```bash
# file: mac:scripts/s4e-update-xm.sh
# Step 4.5: an update and a branch deletion to XM in one push; the new ref set relayed.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
commit_in "$G/xm" m.txt 'pushed from the Mac 2' 'xm 2' || exit 1
echo "new main: $(git -C "$G/xm" rev-parse main)"
logged 4e-push git -C "$G/xm" push proton-v2 main :refs/heads/topic-m
echo "push: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
git ls-remote "proton::$XM" > "$G/refsets/xm-4e-mac.txt" || exit 1
kit refshow xm-4e-mac
kit relay xm-4e-mac
echo "HANDOFF -> PC: W.4.6"
```

**Expected:** `..` with `main -> main`, `[deleted]` with `topic-m`, `push: exit 0`, the ref set
only `HEAD` and `refs/heads/main`, both at `new main`, and `RELAY refset xm-4e-mac remote 2 ...`.

**4.6 and 4.7, on the PC:** W.4.6 fetches XM into its clone and compares with the relayed set; W.4.7
pushes an update and a deletion to XW and relays `RELAY refset xw-4g-win remote 2 ...`, which the
Mac stores.

**4.8 The Mac's clone of XW fetches:** `gx.sh s4h-fetch-xw`.

```bash
# file: mac:scripts/s4h-fetch-xw.sh
# Step 4.8: the Mac's clone of XW fetches the PC's update and deletion.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
logged 4h-fetch git -C "$G/xw-clone" fetch --prune proton-v2
echo "fetch: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
git -C "$G/xw-clone" fsck --full; rc=$?
echo "fsck --full: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
git -C "$G/xw-clone" for-each-ref --format='%(objectname) %(refname)' refs/remotes/proton-v2 > "$G/refsets/xw-4h-macclone.txt" || exit 1
kit refshow xw-4h-macclone
kit relaycmp xw-4h-macclone xw-4g-win || bad "the Mac clone does not hold the PC's update and deletion"
exit "$fail"
```

**Pass:** a line with `-> proton-v2/main` and a `[deleted]` line naming `proton-v2/topic-w`,
`fetch: exit 0`, `fsck --full: exit 0`, and `RELAYED REFSET xw-4g-win vs xw-4h-macclone: EQUAL (1
ref(s), HEAD left out)`.

### Step 5: case and composition variants, inside the gate root

**Section 6's two setup rules, carried by every step below.**

- **Exact bytes.** Every Mac step that gives an NFD name by exact bytes runs in a repository whose
  config sets `core.precomposeunicode=false` (`exact_bytes` in `lib.sh`), as M24's probe did, with
  the name written as bytes (`$NFD`, `$'cafe\xcc\x81'`), because git on a Mac precomposes its
  arguments otherwise (M13). A clone made by git gets `core.precomposeunicode=true` (M13), so each
  step sets it back to `false` before it names a spelling by bytes.
- **Seeded with `main`.** Each remote of its own is seeded with a push of `main` alone first, so its
  `HEAD` names `main` and deleting a `cafe` spelling is never the `HEAD`-branch delete the helper
  refuses [code `internal/repo/push.go:192-196`].

**Which remote each sub-step uses.** `nfd` for 5.1, 5.2 and 5.4 to 5.7; `case` for 5.3; `ren`
for 5.8 and 5.9.2; `mp1` for 5.9.1 and 5.10; `mp3` for 5.9.3; `r1` for 5.11. *Brief author's
call:* section 6 gives 5.8, 5.9.1 and 5.9.3 a remote of their own; this brief gives 5.3 and 5.11
one too, so the case pair and R1's state never meet the composition steps.

**How a refusal is checked.** After each refused push, a snapshot of the whole repo subtree is
compared with the one before it (rule 1), from the Mac session, whose cache is the gate's own.
`NO DIFFERENCE` shows that no pack was uploaded (the helper never trashes a pack: its only trash
calls are the lock's release, a ref delete and two empty-folder cleanups [code
`internal/repo/lock.go:135`, `internal/repo/push.go:606, 922, 1181`]) and that no ref or folder
was left behind; it cannot
tell "never written" from "written, then trashed" for a ref, and that none was written rests on
the refusal's place, phase 2, before anything moves [code `internal/repo/push.go:503-509`], or on
`list`, before anything is advertised. *Brief author's call, the v0.10.0 Linux brief's stated
strength.* Every refused create or update pushes a commit the remote does not hold, so a refusal
that failed would have to upload a pack, which the snapshot would show. The PC's refused push
(W.5.1) also lists the folders from the PC session, a second view.

**Trash lag** applies to every snapshot: each push trashes its `.lock` last, and `kit snapshot`
waits and re-takes. In step 5 a `LAG:` line may name only a repo root's own `.lock`, or the ref
file a delete just trashed (5.8, 5.10); anything else is BLOCKED.

**5.0 Seed the `nfd` remote:** `gx.sh s5-0-nfd-seed`.

```bash
# file: mac:scripts/s5-0-nfd-seed.sh
# Step 5.0: the nfd remote, seeded with main (N1), then the NFC cafe at NC, a commit only that ref
# holds; N2 is made locally and withheld until 5.2.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
if [ -e nfd ]; then echo "BLOCKED: $G/nfd already exists"; exit 1; fi
gate_repo nfd && exact_bytes nfd && commit_in nfd n.txt 'nfd 1' 'nfd N1' || exit 1
git -C nfd remote add proton-v2 "proton::$NFDR" || exit 1
N1=$(git -C nfd rev-parse main)
logged 5-0-main git -C nfd push -u proton-v2 main
echo "push main: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
git -C nfd switch -q -c nc && commit_in nfd c.txt 'only the NFC cafe holds this' 'nfd NC' || exit 1
NC=$(git -C nfd rev-parse HEAD)
git -C nfd switch -q main && git -C nfd branch -q -D nc || exit 1
logged 5-0-nfc git -C nfd push proton-v2 "$NC:refs/heads/$NFC"
echo "push NC to the NFC cafe: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
commit_in nfd n.txt 'nfd 2, withheld until 5.2' 'nfd N2' || exit 1
N2=$(git -C nfd rev-parse main)
printf 'N1=%s\nNC=%s\nN2=%s\n' "$N1" "$NC" "$N2" > "$G/nfd-commits.sh"
echo "N1 (remote main): $N1"; echo "NC (the remote's NFC cafe): $NC"; echo "N2 (local main, withheld): $N2"
git ls-remote "proton::$NFDR" > "$G/out/5-0-ls.txt" && kit esc "$G/out/5-0-ls.txt"
kit roots-update || exit 1
kit snapshot nfd-0 "$NFDR" -r || exit 1
```

**Expected:** both pushes exit 0 (`* [new branch]`, `main -> main`; then `-> caf<U+00E9>` in the
escaped view), `RECORDED: .../vNEXT-mac-nfd-a1`, `ls-remote` listing `HEAD` and `refs/heads/main`
at N1 and `refs/heads/caf<U+00E9>` at NC, and `nfd-0` holding the frame (`gpb-remote.json`, `HEAD`,
`refs`, `refs/heads`, `refs/tags`, `packs`), the two refs and two pack pairs. *Brief author's
call:* the NFC `cafe` sits on a commit nothing else holds, so step 5.5's measurement of M16 can show
a commit left dangling, as M16's probe did.

**5.1 An NFD create beside the NFC one, refused by the Mac's helper and by the draft Windows
helper, naming both and the relation, with no ref or pack written.** First the Mac:
`gx.sh s5-1-mac`.

```bash
# file: mac:scripts/s5-1-mac.sh
# Step 5.1, Mac: a create of the NFD cafe beside the NFC one, by exact bytes, carrying the
# withheld N2.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
logged 5-1-mac git -C nfd push proton-v2 "main:refs/heads/$NFD"
echo "push main:<the NFD cafe>: exit $rc (expected 1: refused)"; [ "$rc" = 0 ] && unrefused "$NFDR" nfd-5-1-mac; [ "$rc" = 1 ] || bad "the push did not exit 1"
kit expect "$L" nfd-create || bad "the refusal's text"
kit helperlines "$L"
echo "refs/heads (Mac view): $(kit names "$NFDR/refs/heads")"
echo "packs (Mac view): $(kit names "$NFDR/packs")"
kit snapshot nfd-1 "$NFDR" -r || exit 1
kit compare nfd-0 nfd-1 --none || bad "the refused create changed the remote"
echo "HANDOFF -> PC: W.5.1"
exit "$fail"
```

**Pass:** `exit 1`, `EXPECT nfd-create: FOUND` (string 1: both spellings raw and as `%+q`
escapes, `by Unicode normalisation`, `a Mac clone would mishandle the pair`), `kit helperlines`'
`others: 0`, the Mac view `refs/heads (Mac view): caf<U+00E9> main` with 5.0's two pack pairs,
and `COMPARE nfd-0 -> nfd-1: NO DIFFERENCE`. The output also holds the `.lock` read-back line. git
frames the reason as ` ! [remote rejected] main -> cafe<U+0301> (...)`. D2 refuses it before the
Mac push rule sees it (its tracking ref would also collide), so the text is string 1's [code
`push.go:503-509`: the case fixed point runs before `macPass`].

Then the PC: W.5.1 pushes the same create, carrying a commit of its own the remote lacks, through
the draft Windows helper, and checks the same entry. Then the Mac, for the PC's push:
`gx.sh s5-1-after`.

```bash
# file: mac:scripts/s5-1-after.sh
# Step 5.1, after the PC's refused push: nothing written, from the Mac's view as well.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
kit snapshot nfd-1w "$NFDR" -r || exit 1
kit compare nfd-1 nfd-1w --none || bad "the PC's refused create changed the remote"
exit "$fail"
```

**Pass:** `COMPARE nfd-1 -> nfd-1w: NO DIFFERENCE`, agreeing with W.5.1's own view.

**5.2 A mixed push lands its update and refuses the variant create:** `gx.sh s5-2-mixed`.

```bash
# file: mac:scripts/s5-2-mixed.sh
# Step 5.2: main moves N1 to N2 and the NFD create is refused, in one push.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
. "$G/nfd-commits.sh"
logged 5-2-mixed git -C nfd push proton-v2 main "main:refs/heads/$NFD"
echo "push: exit $rc (expected 1: main lands, the NFD create is refused)"; [ "$rc" = 0 ] && unrefused "$NFDR" nfd-5-2-mixed; [ "$rc" = 1 ] || bad "the push did not exit 1"
kit expect "$L" nfd-create || bad "the refusal's text"
git ls-remote "proton::$NFDR" > "$G/out/5-2-ls.txt" && kit esc "$G/out/5-2-ls.txt"
grep -q "^$N2"$'\t'"refs/heads/main$" "$G/out/5-2-ls.txt" || bad "main is not at N2"
kit snapshot nfd-2 "$NFDR" -r || exit 1
kit compare nfd-1w nfd-2
exit "$fail"
```

**Pass:** a line with `..` and `main -> main`, the same refusal as 5.1, character for character,
`main` at N2 with `HEAD`, the NFC `cafe` still at NC, and the comparison showing exactly two
`ADDED` rows (`packs/pack-<h>.pack` and `.idx`, one `<h>`) and at most one `CHANGED` row,
`refs/heads/main`, same uid. No row for the NFD spelling exists anywhere.

**5.3 D2's case-variant steps, rerun from the Mac,** on the `case` remote: the v0.10.0 Linux brief's
step 5 (its 5.0 to 5.4), with the case refusal's text as it now reads. Three scripts, run in
order: `gx.sh s5-3a-case`, then `gx.sh s5-3b-feat`, then `gx.sh s5-3c-pair`.

```bash
# file: mac:scripts/s5-3a-case.sh
# Step 5.3, part 1: seed (C1); Main beside main refused, carrying the withheld C2; a mixed push
# of main and Main; then a fresh clone checks the pack's content.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
if [ -e case ] || [ -e case-clone ]; then echo "BLOCKED: $G/case or $G/case-clone already exists"; exit 1; fi
gate_repo case && commit_in case c.txt 'case 1' 'case C1' && git -C case remote add proton-v2 "proton::$CASER" || exit 1
logged 5-3-seed git -C case push -u proton-v2 main
echo "push main: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
kit roots-update || exit 1
commit_in case c.txt 'case 2, withheld' 'case C2' || exit 1
C2=$(git -C case rev-parse main); echo "C2 (local main, withheld): $C2"
kit snapshot case-0 "$CASER" -r || exit 1
logged 5-3-Main git -C case push proton-v2 main:refs/heads/Main
echo "push main:refs/heads/Main: exit $rc (expected 1)"; [ "$rc" = 0 ] && unrefused "$CASER" case-Main; [ "$rc" = 1 ] || bad "not refused"
kit expect "$L" case-Main || bad "the refusal's text"
echo "refs/heads (Mac view): $(kit names "$CASER/refs/heads")"
kit snapshot case-1 "$CASER" -r || exit 1
kit compare case-0 case-1 --none || bad "the refused create changed the remote"
logged 5-3-mixed git -C case push proton-v2 main main:refs/heads/Main
echo "mixed push: exit $rc (expected 1: main lands, Main is refused)"; [ "$rc" = 0 ] && unrefused "$CASER" case-mixed; [ "$rc" = 1 ] || bad "not 1"
kit expect "$L" case-Main || bad "the refusal's text"
kit snapshot case-2 "$CASER" -r || exit 1
kit compare case-1 case-2
logged 5-3-clone git clone -o proton-v2 "proton::$CASER" case-clone
echo "clone: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
git -C case-clone fsck --full || bad "fsck of the clone"
echo "clone proton-v2/main: $(git -C case-clone rev-parse proton-v2/main)   C2: $C2"
[ "$(git -C case-clone rev-parse proton-v2/main)" = "$C2" ] || bad "the clone's proton-v2/main is not C2"
git -C case-clone for-each-ref --format='%(refname)' refs/remotes/proton-v2
exit "$fail"
```

```bash
# file: mac:scripts/s5-3b-feat.sh
# Step 5.3, part 2: feat created, then Feat/x beside it refused.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
logged 5-3-feat git -C case push proton-v2 main:refs/heads/feat
echo "push feat: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
kit snapshot case-3a "$CASER" -r || exit 1
kit compare case-2 case-3a
logged 5-3-Featx git -C case push proton-v2 main:refs/heads/Feat/x
echo "push Feat/x: exit $rc (expected 1)"; [ "$rc" = 0 ] && unrefused "$CASER" case-Featx; [ "$rc" = 1 ] || bad "not refused"
kit expect "$L" case-Featx || bad "the refusal's text"
echo "refs/heads (Mac view): $(kit names "$CASER/refs/heads")"
kit snapshot case-3b "$CASER" -r || exit 1
kit compare case-3a case-3b --none || bad "the refused create changed the remote"
exit "$fail"
```

```bash
# file: mac:scripts/s5-3c-pair.sh
# Step 5.3, part 3: a pair laid down with a CLI upload (Main beside main), an unrelated ref still
# pushed, and a third spelling (MAIN) refused.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
D="$G/case-upload"
if [ -e "$D" ]; then echo "BLOCKED: $D already exists"; exit 1; fi
mkdir "$D" && git -C case rev-parse main > "$D/Main" || exit 1
echo "fixture: $(wc -c < "$D/Main" | tr -d ' ') bytes: $(cat "$D/Main")"
upload_one "$D/Main" "$CASER/refs/heads" 5-3-upload || { echo "BLOCKED: the upload did not report exactly one item transferred"; exit 1; }
kit snapshot case-4a "$CASER" -r || exit 1
kit compare case-3b case-4a
logged 5-3-side git -C case push proton-v2 main:refs/heads/side
echo "push side: exit $rc"; [ "$rc" = 0 ] || bad "side was not pushed"
kit expect "$L" warn-case || bad "the pair warning"
kit snapshot case-4b "$CASER" -r || exit 1
kit compare case-4a case-4b
logged 5-3-MAIN git -C case push proton-v2 main:refs/heads/MAIN
echo "push MAIN: exit $rc (expected 1)"; [ "$rc" = 0 ] && unrefused "$CASER" case-MAIN; [ "$rc" = 1 ] || bad "not refused"
kit expect "$L" warn-case || bad "the pair warning"
kit expect "$L" case-MAIN || bad "the refusal's text"
echo "refs/heads (Mac view): $(kit names "$CASER/refs/heads")"
kit snapshot case-4c "$CASER" -r || exit 1
kit compare case-4b case-4c --none || bad "the refused create changed the remote"
exit "$fail"
```

`upload_one` (in `lib.sh`, below) runs `proton-drive filesystem upload -f skip --json`, which never
overwrites, and reads its summary the way the helper does, as the CLI's merged output [code
`internal/transport/cli.go:58, 393-395`]; the CLI names the node after the local file's basename
[code `cli.go:456-457`, probe C11].

**Pass (5.3):**

- **5.3a:** the seed exits 0 and `RECORDED: .../vNEXT-mac-case-a1`. `main:refs/heads/Main`: exit 1,
  `EXPECT case-Main: FOUND` (string 1, `by letter case`, `a Windows or Mac clone would mishandle
  the pair`), `refs/heads (Mac view): main`, `COMPARE case-0 -> case-1: NO DIFFERENCE`. The mixed
  push: exit 1, `..` and `main -> main`, the same refusal, and `case-1 -> case-2` exactly one pack
  pair `ADDED` and at most `CHANGED refs/heads/main`. The clone exits 0, `fsck` 0, its
  `proton-v2/main` is C2, and its refs are exactly `refs/remotes/proton-v2/HEAD` and
  `refs/remotes/proton-v2/main`.
- **5.3b:** `feat` exits 0 and the comparison shows exactly `ADDED .../refs/heads/feat`. `Feat/x`:
  exit 1, `EXPECT case-Featx: FOUND`, `refs/heads (Mac view): feat main`, `NO DIFFERENCE`: in
  particular no `Feat` folder (remote parent folders are made only in phase 5, which a create
  refused in phase 2 never reaches [code `internal/repo/push.go:55-57`]).
- **5.3c:** `fixture: 41 bytes: <C2>`, `upload counts (transferred, skipped, failed): (1, 0, 0)`,
  `upload: exit 0`, and exactly `ADDED .../refs/heads/Main`. `side`: exit 0 with `* [new branch]`
  and `main -> side`, `EXPECT warn-case: FOUND` (string 4, now on every platform), and exactly
  `ADDED .../refs/heads/side`. `MAIN`: exit 1, the warning again, `EXPECT case-MAIN: FOUND`
  (`Main` before `main`, sorted as bytes), `refs/heads (Mac view): Main feat main side`, and `NO
  DIFFERENCE`.

On a Mac `files` clone these creates would also meet the Mac push rule's part 1(a), since each
tracking ref collides with `proton-v2/main` in one file; D2 refuses them first, with D2's text,
which is the precedence section 3 sets [spec section 3, "Where it sits in phase 2"].

**5.4 A pair laid down with a CLI upload still lets an unrelated ref be pushed and refuses a third
spelling:** `gx.sh s5-4-pair`, on `nfd`.

```bash
# file: mac:scripts/s5-4-pair.sh
# Step 5.4: the NFD cafe laid down beside the NFC one by a CLI upload, at N2; side still pushed;
# a third spelling, Cafe with a capital C (NFC), refused, carrying the withheld N3.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
. "$G/nfd-commits.sh"
D="$G/nfd-upload"
if [ -e "$D" ]; then echo "BLOCKED: $D already exists"; exit 1; fi
mkdir "$D" && printf '%s\n' "$N2" > "$D/$NFD" || exit 1
for f in "$D"/*; do echo "fixture name bytes: $(hexof "$(basename "$f")")"; done
upload_one "$D/$NFD" "$NFDR/refs/heads" 5-4-upload; u=$?
if [ "$u" = 3 ]; then echo "refs/heads (Mac view): $(kit names "$NFDR/refs/heads")"; echo "STOP for Craig: the CLI skipped the NFD name as already present, so it matched it to the NFC member (it normalised the name [untested until now])"; exit 3; fi
[ "$u" = 0 ] || { echo "BLOCKED: the upload did not report exactly one item transferred"; exit 1; }
echo "refs/heads (Mac view): $(kit names "$NFDR/refs/heads")"
kit snapshot nfd-4a "$NFDR" -r || exit 1
kit compare nfd-2 nfd-4a
logged 5-4-side git -C nfd push proton-v2 main:refs/heads/side
echo "push side: exit $rc"; [ "$rc" = 0 ] || bad "side was not pushed"
kit expect "$L" warn-nfd || bad "the pair warning"
kit snapshot nfd-4b "$NFDR" -r || exit 1
kit compare nfd-4a nfd-4b
commit_in nfd n.txt 'nfd 3, withheld' 'nfd N3' || exit 1
echo "N3 (local main, withheld): $(git -C nfd rev-parse main)"
logged 5-4-third git -C nfd push proton-v2 "main:refs/heads/$CAP"
echo "push main:<Cafe>: exit $rc (expected 1)"; [ "$rc" = 0 ] && unrefused "$NFDR" nfd-third; [ "$rc" = 1 ] || bad "not refused"
kit expect "$L" warn-nfd || bad "the pair warning"
kit expect "$L" third-spelling || bad "the refusal's text"
kit snapshot nfd-4c "$NFDR" -r || exit 1
kit compare nfd-4b nfd-4c --none || bad "the refused create changed the remote"
exit "$fail"
```

**Pass:**

- `fixture name bytes: 63616665cc81` (the local file carries the NFD bytes; APFS keeps a name as
  written, M12), `upload counts ... (1, 0, 0)`, `upload: exit 0`, and `refs/heads (Mac view):
  cafe<U+0301> caf<U+00E9> main`. **If the view shows no `cafe<U+0301>`**, the CLI normalised the
  name on the way up [untested]: STOP for Craig, since 5.5 to 5.7 and 5.9.1 then cannot run as
  written. The likeliest form of that, `upload counts ... (0, 1, 0)` (skipped as already present),
  makes the script print the view and stop with that STOP (exit 3); any other count is BLOCKED.
  The comparison shows exactly `ADDED .../refs/heads/cafe<U+0301>`.
- `side`: exit 0, `EXPECT warn-nfd: FOUND` (the pair, NFD first, both object IDs, both `%+q`
  forms, `by Unicode normalisation`), and exactly `ADDED .../refs/heads/side`. D2's ruling that a
  remote holding a pair still takes pushes to its other refs stands.
- The third spelling: exit 1, the warning, and `EXPECT third-spelling: FOUND`: one refusal naming
  both members, the NFD one `by Unicode normalisation` and the NFC one `by letter case`, so its
  harm reads `a Windows or Mac clone would mishandle the pair`; then `NO DIFFERENCE` (no pack for
  N3). *Brief author's call:* the third spelling is a case variant of the NFC name, the one third
  spelling of `cafe` there is, since NFC and NFD are its only two canonical forms; it also shows
  both relations in one message.

**5.5 A Mac clone into the default ref store is refused at `list`; then M16, measured through a
real helper.** Section 6, in full: the Mac clone "is refused at `list`, with the pair and object
IDs named, and leaves no ref written (O3(d)). Then M16 through a real helper, measured once rather
than inferred: the v0.10.0 helper from prep, used for this step's commands alone through a
per-command `PATH`, clones the remote ...; then the NFD member is moved to a new commit by a push
from the paired Windows session, with the name as exact bytes. ... Then the clone under test
fetches, and what it leaves is recorded."

*Brief author's call: step 5.6's `reftable` clone is made here, before the PC's move,* so that its
fetch in 5.6 carries a real change to one member; otherwise "both spellings survive ... a fetch"
would be a fetch with nothing to do. Each sub-step keeps its own number and checks.

`gx.sh s5-5a-clones`:

```bash
# file: mac:scripts/s5-5a-clones.sh
# Step 5.5, part 1: the files clone refused at list; the v0.10.0 helper's clone; and step 5.6's
# reftable clone through the gated helper, made before the PC's move.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
. "$G/nfd-commits.sh"; export N2 NC
for d in nfd-files nfd-v010 nfd-rt; do if [ -e "$d" ]; then echo "BLOCKED: $G/$d already exists"; exit 1; fi; done
logged 5-5-files git clone -o proton-v2 "proton::$NFDR" nfd-files
echo "files clone through the gated helper: exit $rc (expected nonzero: refused at list)"; [ "$rc" != 0 ] || bad "the clone was not refused"
kit expect "$L" fetch-pair-5.5 || bad "the refusal's text"
if [ -e nfd-files ]; then
  echo "nfd-files was left; its refs: [$(git -C nfd-files for-each-ref 2>&1)]"
  [ -z "$(git -C nfd-files for-each-ref 2>/dev/null)" ] || bad "the refused clone wrote refs"
else
  echo "no clone directory was left, so no ref was written"
fi
[ -s "$G/v010-needs-uncertified" ] || { echo "BLOCKED: C.2 did not record whether the CLI pin moved"; exit 1; }
echo "helper for the v0.10.0 commands (lib.sh's v010): $(PATH="$GM/v010-build/bin:$PATH" command -v git-remote-proton); GPB_UNCERTIFIED_CLI=1: $(cat "$G/v010-needs-uncertified")"
logged 5-5-v010-version v010 git-remote-proton --version; kit expect "$L" v010-version || bad "not v0.10.0's helper"
logged 5-5-v010-clone v010 git clone -o proton-v2 "proton::$NFDR" nfd-v010
echo "v0.10.0 clone: exit $rc"; [ "$rc" = 0 ] || bad "the v0.10.0 clone failed"
exact_bytes nfd-v010
git -C nfd-v010 for-each-ref --format='%(objectname) %(refname)' > "$G/out/5-5-v010-refs-1.txt"; kit esc "$G/out/5-5-v010-refs-1.txt"
git -C nfd-v010 fsck --full > "$G/out/5-5-v010-fsck-1.txt" 2>&1; echo "fsck --full: exit $?"; kit esc "$G/out/5-5-v010-fsck-1.txt"
logged 5-6-rt-clone git clone --ref-format=reftable -o proton-v2 "proton::$NFDR" nfd-rt
echo "reftable clone through the gated helper: exit $rc"; [ "$rc" = 0 ] || bad "the reftable clone failed"
kit expect "$L" warn-nfd || bad "the pair warning"
echo "ref format: $(git -C nfd-rt rev-parse --show-ref-format)"
git -C nfd-rt for-each-ref --format='%(objectname) %(refname)' refs/remotes/proton-v2 > "$G/out/5-6-rt-refs-1.txt"; kit esc "$G/out/5-6-rt-refs-1.txt"
grep -qF "$N2 refs/remotes/proton-v2/$NFD" "$G/out/5-6-rt-refs-1.txt" || bad "the reftable clone lacks the NFD tracking ref at N2"
grep -qF "$NC refs/remotes/proton-v2/$NFC" "$G/out/5-6-rt-refs-1.txt" || bad "the reftable clone lacks the NFC tracking ref at NC"
echo "HANDOFF -> PC: W.5.5"
exit "$fail"
```

**Pass:**

- The `files` clone exits nonzero, and `EXPECT fetch-pair-5.5: FOUND`: string 5, the pair with its
  object IDs (the NFD member at N2, the NFC one at NC), both escape forms, and the remedies. No ref
  was written: either no clone folder was left (git removes the folder a failed clone made
  [inference]) or it holds no ref; record which. The refusal comes at `list`, before git is told of
  any ref (M28: at a clone's `list` the new repository already reads `files`). git's own exit code
  and lines around it are recorded; the code is expected to be 128 [inference from M8], and only
  nonzero is judged.
- The v0.10.0 commands: `EXPECT v010-version: FOUND`, then `v0.10.0 clone: exit 0`, recorded with
  its refs and `fsck`. v0.10.0 has no pair warning and no Mac rule, so it clones the pair; M14
  expects both tracking refs with their own bytes and a clean `fsck` [inference from M14]. If
  `GPB_UNCERTIFIED_CLI=1` applied, string 18 is in its output, recorded as such.
- 5.6's clone: `exit 0`, `EXPECT warn-nfd: FOUND` (the warning only), `ref format: reftable`, and
  both tracking refs at their own commits.

Then the PC: W.5.5 fetches its `nfd` clone and moves the NFD member to a new commit W3 by a push of
the exact bytes, through the draft Windows helper, and relays `RELAY value W3 <sha>`, which the Mac
stores. Not from the Mac, for section 6's four reasons: a Mac `files` clone is refused by the Mac
push rule; a push from the clone under test would update its own tracking ref and leave the fetch
nothing to do; a typed Mac push precomposes to NFC (M13); and a CLI upload would move the ref
without uploading its pack.

Then the Mac: `gx.sh s5-5b-fetches`.

```bash
# file: mac:scripts/s5-5b-fetches.sh
# Step 5.5, part 2: the v0.10.0 clone fetches the moved NFD member, and what it leaves is
# recorded (M16). Step 5.6: the reftable clone fetches through the gated helper.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
. "$G/nfd-commits.sh"; W3=$(kit getval W3) || exit 1; export W3 N2 NC
echo "NC=$NC  N2=$N2  W3=$W3"
logged 5-5-v010-fetch v010 git -C nfd-v010 fetch proton-v2
echo "v0.10.0 fetch: exit $rc (recorded, not judged)"
git -C nfd-v010 for-each-ref --format='%(objectname) %(refname)' > "$G/out/5-5-v010-refs-2.txt"; kit esc "$G/out/5-5-v010-refs-2.txt"
for s in "$NFC" "$NFD"; do printf 'rev-parse refs/remotes/proton-v2/%s: %s\n' "$s" "$(git -C nfd-v010 rev-parse "refs/remotes/proton-v2/$s" 2>&1)"; done > "$G/out/5-5-v010-revparse.txt"
kit esc "$G/out/5-5-v010-revparse.txt"
git -C nfd-v010 fsck --full > "$G/out/5-5-v010-fsck-2.txt" 2>&1; echo "fsck --full: exit $?"; kit esc "$G/out/5-5-v010-fsck-2.txt"
logged 5-6-rt-fetch git -C nfd-rt fetch proton-v2
echo "reftable fetch through the gated helper: exit $rc"; [ "$rc" = 0 ] || bad "the reftable fetch failed"
kit expect "$L" warn-nfd || bad "the pair warning"
git -C nfd-rt for-each-ref --format='%(objectname) %(refname)' refs/remotes/proton-v2 > "$G/out/5-6-rt-refs-2.txt"; kit esc "$G/out/5-6-rt-refs-2.txt"
grep -qF "$W3 refs/remotes/proton-v2/$NFD" "$G/out/5-6-rt-refs-2.txt" || bad "the reftable clone's NFD tracking ref is not at W3"
grep -qF "$NC refs/remotes/proton-v2/$NFC" "$G/out/5-6-rt-refs-2.txt" || bad "the reftable clone's NFC tracking ref is not at NC"
git -C nfd-rt fsck --full || bad "fsck of the reftable clone"
kit snapshot nfd-5 "$NFDR" -r || exit 1
kit compare nfd-4c nfd-5
exit "$fail"
```

**5.5, recorded, not judged:** the v0.10.0 fetch's exit code and output, the clone's refs after it,
what each tracking ref resolves to by its exact bytes, and `fsck`'s output. M16 predicts exit 0,
quietly, the NFC tracking ref then showing W3, the NFD one stale, and `fsck` reporting NC dangling
[inference from M16, measured there with a local remote; this is its measurement through a real
helper]. Whatever the run shows is the finding, and the record states it plainly.

**5.6 Pass:** the `reftable` clone got the warning only (5.5a), and after the fetch (exit 0, the
warning again) both spellings survive with their own commits: the NFD tracking ref at W3 and the
NFC one at NC, and `fsck` exits 0. That is M17's mitigation end to end through the gated helper.
`nfd-4c -> nfd-5` shows the PC's push: one pack pair `ADDED` and `CHANGED .../refs/heads/cafe<U+0301>`.

**5.7 `git ls-remote` of that remote:** from Windows (W.5.7), and on the Mac `gx.sh s5-7-lsremote`.

```bash
# file: mac:scripts/s5-7-lsremote.sh
# Step 5.7: ls-remote from a folder with no repository above it warns and succeeds; from inside a
# Mac files repository, and from a subfolder of one, it is refused as a fetch would be (M25).
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
. "$G/nfd-commits.sh"; W3=$(kit getval W3) || exit 1; export W3 NC
T=$(mktemp -d) || exit 1
cd "$T" || exit 1
if git rev-parse --git-dir >/dev/null 2>&1; then bad "$T is inside a repository"; fi
logged 5-7-outside git ls-remote "proton::$NFDR"
echo "ls-remote from $T (no repository above it): exit $rc"; [ "$rc" = 0 ] || bad "it did not succeed"
kit expect "$L" warn-nfd || bad "the pair warning"
cd "$G/nfd" || exit 1
logged 5-7-inside git ls-remote "proton::$NFDR"
echo "ls-remote from inside the nfd repository (files): exit $rc (expected nonzero)"; [ "$rc" != 0 ] || bad "it was not refused"
kit expect "$L" fetch-pair-5.7 || bad "the refusal's text"
mkdir -p "$G/nfd/sub" && cd "$G/nfd/sub" || exit 1
logged 5-7-subfolder git ls-remote "proton::$NFDR"
echo "ls-remote from a subfolder of it: exit $rc (expected nonzero)"; [ "$rc" != 0 ] || bad "it was not refused"
kit expect "$L" fetch-pair-5.7 || bad "the refusal's text"
cd "$G" && rmdir "$G/nfd/sub" && rm -rf -- "$T"
echo "format of the nfd repository: $(git -C "$G/nfd" rev-parse --show-ref-format)"
exit "$fail"
```

**Pass:** from the temp folder, exit 0 with the ref list (escaped view: `HEAD`, `main` and `side`
at N2, `caf<U+00E9>` at NC, `cafe<U+0301>` at W3) and `EXPECT warn-nfd: FOUND`; from inside the
repository and from its subfolder, nonzero and `EXPECT fetch-pair-5.7: FOUND` (string 5, with W3
and NC), because git sets `GIT_DIR` anywhere inside a repository (M25); the repository's format
`files`. W.5.7: exit 0 and the warning on Windows. This is also the pre-adoption scan's form
(section 3, O3(a)): run from a folder with no repository above it.

**5.8 The rename transition,** on the `ren` remote. First the Mac: `gx.sh s5-8a-seed`.

```bash
# file: mac:scripts/s5-8a-seed.sh
# Step 5.8, part 1: the ren remote holds main and the NFC cafe at RC1; a fresh Mac clone holds
# the NFC cafe's tracking ref.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
if [ -e ren ] || [ -e ren-clone ]; then echo "BLOCKED: $G/ren or $G/ren-clone already exists"; exit 1; fi
gate_repo ren && exact_bytes ren && commit_in ren r.txt 'ren 1' 'ren RC1' && git -C ren remote add proton-v2 "proton::$RENR" || exit 1
logged 5-8-main git -C ren push -u proton-v2 main
echo "push main: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
logged 5-8-nfc git -C ren push proton-v2 "main:refs/heads/$NFC"
echo "push the NFC cafe: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
git -C ren rev-parse main > "$G/ren-RC1.txt"; echo "RC1: $(cat "$G/ren-RC1.txt")"
kit roots-update || exit 1
logged 5-8-clone git clone -o proton-v2 "proton::$RENR" ren-clone
echo "fresh Mac clone: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
git -C ren-clone for-each-ref --format='%(objectname) %(refname)' > "$G/out/5-8-clone-refs.txt"; kit esc "$G/out/5-8-clone-refs.txt"
kit snapshot ren-0 "$RENR" -r || exit 1
echo "HANDOFF -> PC: W.5.8"
```

**Expected:** both pushes and the clone exit 0, `RECORDED: .../vNEXT-mac-ren-a1`, and the clone's
refs include `refs/remotes/proton-v2/caf<U+00E9>` at RC1.

Then the PC: W.5.8 deletes the NFC `cafe` in one push and, in a second push, creates the NFD
spelling by exact bytes at the same commit, so the remote never advertises a pair and git would
send no `fetch` command. Not from the Mac clone under test: its own delete push would remove the
stale tracking ref this step exists to leave, and with it step 5.9.2's precondition (section 6).

Then the Mac: `gx.sh s5-8b-fetch`.

```bash
# file: mac:scripts/s5-8b-fetch.sh
# Step 5.8, part 2: the Mac clone's next fetch is refused at list under rule (2), and nothing is
# written.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
RC1=$(cat "$G/ren-RC1.txt"); export RC1
refs_of ren-clone "$G/out/5-8-before.txt"
logged 5-8-fetch git -C ren-clone fetch proton-v2
echo "fetch: exit $rc (expected nonzero: refused at list)"; [ "$rc" != 0 ] || bad "the fetch was not refused"
kit expect "$L" fetch-dest-5.8 || bad "the refusal's text"
kit absent "$L" warn-head || bad "a pair warning: the remote should hold no pair"
refs_of ren-clone "$G/out/5-8-after.txt"
if cmp -s "$G/out/5-8-before.txt" "$G/out/5-8-after.txt"; then echo "local refs: unchanged, byte for byte"; else bad "the refused fetch changed a local ref"; fi
kit esc "$G/out/5-8-after.txt"
git ls-remote "proton::$RENR" > "$G/out/5-8-ls.txt" && kit esc "$G/out/5-8-ls.txt"
kit snapshot ren-8 "$RENR" -r || exit 1
kit compare ren-0 ren-8
exit "$fail"
```

**Pass:** nonzero, `EXPECT fetch-dest-5.8: FOUND` (string 6: the NFD destination from the remote's
NFD branch at RC1 against the stale NFC tracking ref, `local`, and the remedies), no pair warning,
and `local refs: unchanged, byte for byte`. The remote lists `HEAD`, `main` and `cafe<U+0301>`, all
at RC1, and no NFC `cafe`; `ren-0 -> ren-8` shows the NFC ref `REMOVED` and the NFD ref `ADDED` (a
`TRASHED, STILL LISTED` line for the NFC ref is lag). This is the case section 3's rule (2) was
written for: a rename in two pushes, which never advertises a pair, caught by an unpruned clone's
old tracking ref.

**5.9 The Mac push rule, each part where nothing else would refuse first.**

**5.9.1 Part 1, remote half,** on the `mp1` remote: `gx.sh s5-9-1`.

```bash
# file: mac:scripts/s5-9-1.sh
# Step 5.9.1: a Mac files clone pushes the NFC cafe; the NFD member is laid down at the same
# commit by a CLI upload, as 5.4 lays one; an update of the NFC member is then refused for that
# ref while an unrelated ref in the same push lands.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
for d in mp1 mp1-clone mp1-upload; do if [ -e "$d" ]; then echo "BLOCKED: $G/$d already exists"; exit 1; fi; done
gate_repo mp1 && commit_in mp1 p.txt 'mp1 1' 'mp1 M1' && git -C mp1 remote add proton-v2 "proton::$MP1R" || exit 1
logged 5-9-1-seed git -C mp1 push -u proton-v2 main
echo "seed: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
M1=$(git -C mp1 rev-parse main); echo "M1: $M1"
logged 5-9-1-clone git clone -o proton-v2 "proton::$MP1R" mp1-clone
echo "Mac files clone: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
gate_identity mp1-clone || exit 1
git -C mp1-clone branch "$NFC" main && git -C mp1-clone branch side main || exit 1
logged 5-9-1-nfc git -C mp1-clone push proton-v2 "$NFC"
echo "push the NFC cafe from the clone: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
kit roots-update || exit 1
mkdir mp1-upload && printf '%s\n' "$M1" > "mp1-upload/$NFD" || exit 1
upload_one "$G/mp1-upload/$NFD" "$MP1R/refs/heads" 5-9-1-upload || { echo "BLOCKED: the upload did not report exactly one item transferred"; exit 1; }
echo "refs/heads (Mac view): $(kit names "$MP1R/refs/heads")"
kit snapshot mp1-1 "$MP1R" -r || exit 1
git -C mp1-clone switch -q "$NFC" && commit_in mp1-clone p.txt 'mp1 2, the NFC update, withheld' 'mp1 M2' || exit 1
M2C=$(git -C mp1-clone rev-parse HEAD); echo "M2 (the clone's NFC update, withheld): $M2C"
printf 'M1=%s\nM2C=%s\n' "$M1" "$M2C" > "$G/mp1-commits.sh"
logged 5-9-1-push git -C mp1-clone push proton-v2 "$NFC" side
echo "push the NFC update and side: exit $rc (expected 1: the update refused, side lands)"; [ "$rc" = 0 ] && unrefused "$MP1R" mp1; [ "$rc" = 1 ] || bad "not 1"
kit expect "$L" warn-nfd || bad "the pair warning"
kit expect "$L" mac-remote-half || bad "the refusal's text"
git ls-remote "proton::$MP1R" > "$G/out/5-9-1-ls.txt" && kit esc "$G/out/5-9-1-ls.txt"
kit snapshot mp1-2 "$MP1R" -r || exit 1
kit compare mp1-1 mp1-2
exit "$fail"
```

**Pass:** the seed, the clone and the NFC push exit 0; `RECORDED: .../vNEXT-mac-mp1-a1`; the upload
`(1, 0, 0)` and `refs/heads (Mac view): cafe<U+0301> caf<U+00E9> main`. The last push: exit 1,
`* [new branch]` with `side -> side`, the pair warning, and `EXPECT mac-remote-half: FOUND`
(string 8). The remote then lists `side` at M1 and both `cafe` members still at M1, and
`mp1-1 -> mp1-2` shows exactly `ADDED .../refs/heads/side` (no pack: M2 is withheld). The clone
holds the member it updates, and M1 is its ancestor, so git's fast-forward check passes [code
`internal/repo/push.go:265-292`] and the refusal is the Mac rule's alone. `list for-push`'s R1
check does not fire here: the clone's tracking refs alias no other local ref.

**5.9.2 Part 3,** on `ren` (5.8's remote, which holds the NFD `cafe` only), from 5.8's clone, still
holding its stale NFC tracking ref: `gx.sh s5-9-2`.

```bash
# file: mac:scripts/s5-9-2.sh
# Step 5.9.2: from 5.8's clone, a delete of the NFD cafe by exact bytes is refused (part 3), and
# every local ref is unchanged (M24).
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
exact_bytes ren-clone
refs_of ren-clone "$G/out/5-9-2-before.txt"
logged 5-9-2-push git -C ren-clone push proton-v2 ":refs/heads/$NFD"
echo "push the delete of the NFD cafe: exit $rc (expected 1: refused)"; [ "$rc" = 0 ] && unrefused "$RENR" ren; [ "$rc" = 1 ] || bad "not refused"
kit expect "$L" mac-part3 || bad "the refusal's text"
refs_of ren-clone "$G/out/5-9-2-after.txt"
if cmp -s "$G/out/5-9-2-before.txt" "$G/out/5-9-2-after.txt"; then echo "local refs: unchanged, byte for byte"; else bad "a local ref changed"; fi
kit snapshot ren-9 "$RENR" -r || exit 1
kit compare ren-8 ren-9 --none || bad "the refused delete changed the remote"
exit "$fail"
```

**Pass:** exit 1, `EXPECT mac-part3: FOUND` (string 10), `local refs: unchanged, byte for byte`
(section 6 asks for the comparison by `for-each-ref` before and after), and `NO DIFFERENCE`. Why
nothing else refuses first: the clone holds no branch named `cafe`, so R1 at `list for-push` does
not reach this delete (plan Task 9, "Which parts real git can reach"), and the remote holds no
pair. Section 6 gives this step as it stands, and so does this brief.

**5.9.3 Part 1, local half alone,** on the `mp3` remote, which holds neither spelling:
`gx.sh s5-9-3`.

```bash
# file: mac:scripts/s5-9-3.sh
# Step 5.9.3: a Mac clone carrying a tracking ref for the NFC cafe with no remote counterpart
# (made with git update-ref, as a never-pruned leftover would be) pushes a create of the NFD
# spelling by exact bytes: D2 has nothing to refuse, and the local half of part 1 refuses it.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
for d in mp3 mp3-clone; do if [ -e "$d" ]; then echo "BLOCKED: $G/$d already exists"; exit 1; fi; done
gate_repo mp3 && commit_in mp3 q.txt 'mp3 1' 'mp3 P1' && git -C mp3 remote add proton-v2 "proton::$MP3R" || exit 1
logged 5-9-3-seed git -C mp3 push -u proton-v2 main
echo "seed: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
P1=$(git -C mp3 rev-parse main); echo "P1: $P1"
logged 5-9-3-clone git clone -o proton-v2 "proton::$MP3R" mp3-clone
echo "Mac files clone: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
kit roots-update || exit 1
exact_bytes mp3-clone && gate_identity mp3-clone || exit 1
git -C mp3-clone update-ref "refs/remotes/proton-v2/$NFC" "$P1" || exit 1
commit_in mp3-clone q.txt 'mp3 2, withheld' 'mp3 P2' || exit 1
git -C mp3-clone for-each-ref --format='%(objectname) %(refname)' > "$G/out/5-9-3-refs.txt"; kit esc "$G/out/5-9-3-refs.txt"
kit snapshot mp3-1 "$MP3R" -r || exit 1
logged 5-9-3-push git -C mp3-clone push proton-v2 "main:refs/heads/$NFD"
echo "push the NFD create: exit $rc (expected 1: refused)"; [ "$rc" = 0 ] && unrefused "$MP3R" mp3; [ "$rc" = 1 ] || bad "not refused"
kit expect "$L" mac-local-half || bad "the refusal's text"
kit snapshot mp3-2 "$MP3R" -r || exit 1
kit compare mp3-1 mp3-2 --none || bad "the refused create changed the remote"
exit "$fail"
```

**Pass:** exit 1, `EXPECT mac-local-half: FOUND` (string 9: the create's tracking ref would collide
with the local NFC tracking ref, which no remote ref matches, so neither D2 nor the remote half
says anything), and `NO DIFFERENCE` (no pack for P2).

**5.10 The repair (section 3),** on 5.9.1's remote: `gx.sh s5-10-repair`.

```bash
# file: mac:scripts/s5-10-repair.sh
# Step 5.10: from a temporary reftable clone, the NFD member (which HEAD does not name) is
# deleted by its exact bytes and not refused; a fresh Mac files clone then succeeds with no
# warning; 5.9.1's clone is recovered and re-cloned, as section 3's repair step 4 says.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
. "$G/mp1-commits.sh"
for d in mp1-rt mp1-fresh mp1-reclone; do if [ -e "$d" ]; then echo "BLOCKED: $G/$d already exists"; exit 1; fi; done
logged 5-10-rt-clone git clone --ref-format=reftable -o proton-v2 "proton::$MP1R" mp1-rt
echo "temporary reftable clone: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
kit expect "$L" warn-nfd || bad "the pair warning (step 1 of the repair: both names and object IDs)"
echo "ref format: $(git -C mp1-rt rev-parse --show-ref-format)"
git -C mp1-rt ls-remote --symref proton-v2 HEAD
exact_bytes mp1-rt
logged 5-10-delete git -C mp1-rt push proton-v2 ":refs/heads/$NFD"
echo "delete the NFD member from the reftable clone: exit $rc"; [ "$rc" = 0 ] || bad "the delete failed"
grep -q '\[deleted\]' "$L" || bad "no [deleted] line"
git ls-remote "proton::$MP1R" > "$G/out/5-10-ls.txt" && kit esc "$G/out/5-10-ls.txt"
logged 5-10-fresh git clone -o proton-v2 "proton::$MP1R" mp1-fresh
echo "fresh Mac files clone: exit $rc"; [ "$rc" = 0 ] || bad "the fresh clone failed"
kit absent "$L" warn-head || bad "the fresh clone still warned"
echo "== step 5.9.1's clone: what only it holds, then its unpushed branch pushed, then a re-clone"
git -C mp1-clone fsck --full > "$G/out/5-10-old-fsck.txt" 2>&1; echo "fsck --full: exit $?"; kit esc "$G/out/5-10-old-fsck.txt"
git -C mp1-clone for-each-ref --format='%(objectname) %(refname)' > "$G/out/5-10-old-refs.txt"; kit esc "$G/out/5-10-old-refs.txt"
logged 5-10-recover git -C mp1-clone push proton-v2 "$NFC"
echo "push the clone's unpushed NFC commit: exit $rc"; [ "$rc" = 0 ] || bad "the recovery push failed"
kit absent "$L" warn-head || bad "a pair warning after the repair"
logged 5-10-reclone git clone -o proton-v2 "proton::$MP1R" mp1-reclone
echo "re-clone: exit $rc"; [ "$rc" = 0 ] || bad "the re-clone failed"
kit absent "$L" warn-head || bad "the re-clone warned"
r=$(git -C mp1-reclone rev-parse "refs/remotes/proton-v2/$NFC"); echo "re-clone's NFC cafe: $r   M2: $M2C"
[ "$r" = "$M2C" ] || bad "the re-clone does not hold the recovered commit"
git -C mp1-reclone fsck --full || bad "fsck of the re-clone"
kit snapshot mp1-10 "$MP1R" -r || exit 1
kit compare mp1-2 mp1-10
exit "$fail"
```

**Pass:**

- The temporary clone: exit 0, the warning (the repair's step 1 reads both names and object IDs
  from it), `ref format: reftable`, and `HEAD` naming `refs/heads/main`.
- The delete: exit 0, `[deleted]` with `cafe<U+0301>`, not refused: a `reftable` repository gets
  no Mac rule [code `cmd/git-remote-proton/main.go:911-913`], D2 judges no delete, and `HEAD` does
  not name the member. The remote then lists `HEAD`, `main`, `side` and `caf<U+00E9>`, and no NFD
  ref.
- The fresh `files` clone: exit 0 and `ABSENT warn-head: YES`.
- 5.9.1's clone: `fsck` and its refs recorded; it holds an unpushed commit, M2, on its NFC branch
  (the refused update), so the recovery pushes it; that push exits 0 now that the remote holds no
  pair, with no warning. The re-clone exits 0 with no warning, its NFC tracking ref at M2, and
  `fsck` 0. `mp1-2 -> mp1-10` shows the NFD ref `REMOVED`, one pack pair `ADDED` (M2), and
  `CHANGED .../refs/heads/caf<U+00E9>`.

*Brief author's call:* "recovered" is done by pushing the clone's one unpushed branch, which is
what it alone holds; section 3's step 4 also names a commit `fsck` reports dangling, and the
recorded `fsck` output shows whether there is one. The re-clone is not optional, whatever `fsck`
says: a rewind leaves `git fsck` exiting 0, reporting nothing in M24's run (section 3) and, in the
stale-ref probe's run of M24's layout, listing the lost commit only among the dangling ones
[record `docs/research/probes/macos-port/README.md`, layout L8]. The old clone stays in the run
folder, not used again: step 7 deletes no local repository ("What stays").

**5.11 R1 (added by this brief): an up-to-date push refused at `list for-push`,** on the `r1`
remote. The plan's Task 15 asks for it, in PM1's shape: a Mac `files` clone holds branch `cafe` NFC
at the remote's commit and a stale NFD tracking ref, and its `git push --all` is refused at `list
for-push`. Then the remedy string 7 names (MA3): the same `git push --all proton-v2` from a fresh
`--ref-format=reftable` clone of the same remote, `r1-rt`, a folder in the run folder like the
gate's other clones, with the gate identity. *Brief author's call:* numbered 5.11, after section
6's own steps, so none of theirs moves; it is R1's step, not one of the push rule's three parts.
`gx.sh s5-11-r1`.

```bash
# file: mac:scripts/s5-11-r1.sh
# Step 5.11 (R1, PM1's shape): a files clone holding the NFC cafe branch at the remote's commit
# and a stale NFD tracking ref; git push --all is refused at list for-push, nothing is written.
# Then the refusal's remedy (MA3): the same push, from a fresh reftable clone of the same remote,
# goes through with nothing to write. r1-clone keeps its stale ref: no in-place repair is run.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
for d in r1 r1-clone r1-rt; do if [ -e "$d" ]; then echo "BLOCKED: $G/$d already exists"; exit 1; fi; done
gate_repo r1 && commit_in r1 s.txt 'r1 1' 'r1 Q1' && git -C r1 remote add proton-v2 "proton::$R1R" || exit 1
logged 5-11-seed git -C r1 push -u proton-v2 main
echo "seed: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
Q1=$(git -C r1 rev-parse main); export Q1; echo "Q1: $Q1"
logged 5-11-nfc git -C r1 push proton-v2 "main:refs/heads/$NFC"
echo "push the NFC cafe: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
kit roots-update || exit 1
logged 5-11-clone git clone -o proton-v2 "proton::$R1R" r1-clone
echo "Mac files clone: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
exact_bytes r1-clone && gate_identity r1-clone || exit 1
git -C r1-clone branch "$NFC" "refs/remotes/proton-v2/$NFC" || exit 1
B=$(git -C r1-clone commit-tree -p HEAD -m 'gate: the stale value' 'HEAD^{tree}') || exit 1
git -C r1-clone update-ref "refs/remotes/proton-v2/$NFD" "$B" || exit 1
echo "local NFC cafe branch at: $(git -C r1-clone rev-parse "refs/heads/$NFC")   stale NFD tracking ref at B: $B"
refs_of r1-clone "$G/out/5-11-before.txt"; kit esc "$G/out/5-11-before.txt"
kit snapshot r1-1 "$R1R" -r || exit 1
logged 5-11-push git -C r1-clone push --all proton-v2
echo "push --all: exit $rc (expected nonzero: refused at list for-push)"; [ "$rc" = 0 ] && unrefused "$R1R" r1; [ "$rc" != 0 ] || bad "the push was not refused"
kit expect "$L" push-r1 || bad "the refusal's text"
refs_of r1-clone "$G/out/5-11-after.txt"
if cmp -s "$G/out/5-11-before.txt" "$G/out/5-11-after.txt"; then echo "local refs: unchanged, byte for byte"; else bad "a local ref changed"; fi
kit snapshot r1-2 "$R1R" -r || exit 1
kit compare r1-1 r1-2 --none || bad "the refused push changed the remote"
echo "== the remedy: a reftable clone of the same remote, and the same push from it"
logged 5-11-rt-clone git clone --ref-format=reftable -o proton-v2 "proton::$R1R" r1-rt
echo "reftable clone: exit $rc"; [ "$rc" = 0 ] || { bad "the reftable clone failed"; exit 1; }
kit absent "$L" warn-head || bad "the reftable clone warned"
f=$(git -C r1-rt rev-parse --show-ref-format); echo "ref format: $f"; [ "$f" = reftable ] || bad "r1-rt is not a reftable repository"
gate_identity r1-rt || exit 1
git -C r1-rt branch "$NFC" "refs/remotes/proton-v2/$NFC" || exit 1
echo "r1-rt branches: main at $(git -C r1-rt rev-parse main), the NFC cafe at $(git -C r1-rt rev-parse "refs/heads/$NFC")"
logged 5-11-rt-push git -C r1-rt push --all proton-v2
echo "push --all from the reftable clone: exit $rc"; [ "$rc" = 0 ] || bad "the push from the reftable clone failed"
grep -q 'Everything up-to-date' "$L" || bad "no 'Everything up-to-date'"
kit absent "$L" push-r1 || bad "the reftable clone's push was refused"
kit snapshot r1-3 "$R1R" -r || exit 1
kit compare r1-2 r1-3 --none || bad "the push from the reftable clone changed the remote"
refs_of r1-clone "$G/out/5-11-left.txt"
if cmp -s "$G/out/5-11-before.txt" "$G/out/5-11-left.txt"; then echo "r1-clone: left as it was, its stale NFD tracking ref in place (not repaired in place; MA3)"; else bad "r1-clone's refs changed"; fi
exit "$fail"
```

**Pass:** the seed, the NFC push and the clone exit 0, and `RECORDED: .../vNEXT-mac-r1-a1`. The
branch is made before the stale ref, so it starts at Q1 (after it, a lookup of the NFC tracking ref
would find the NFD file, M15). The refused push: nonzero (128 expected [inference], only nonzero
judged), `EXPECT push-r1: FOUND` (string 7: the NFC tracking ref, from the NFC branch at Q1,
against the stale NFD one, `local`), `local refs: unchanged, byte for byte`, and `NO DIFFERENCE`
(the session took the lock and released it [code `cmd/git-remote-proton/main.go:773-779`]; a
`.lock` lag line is expected). Then the remedy string 7 names: the `reftable` clone exits 0 with
`ABSENT warn-head: YES` (the remote holds no pair) and `ref format: reftable`; its `push --all`,
of the same two branches at Q1, exits 0 with `Everything up-to-date` and `ABSENT push-r1: YES`,
since a `reftable` repository gets no Mac rule [code `cmd/git-remote-proton/main.go:518-520`];
and `NO DIFFERENCE` from `r1-2` to `r1-3` (that push's lock session is its one trace, with the
same lag line); then `r1-clone: left as it was`. PM1 measured the harm R1 prevents: with no
refusal, an "Everything up-to-date" push rewrote the aliasing tracking ref [plan, PM1].

**`r1-clone` keeps its stale ref.** *Brief author's call, following MA3:* string 7 names only the
`reftable` clone; the in-place route (`pack-refs` with precomposition off, then the exact-bytes
delete) is the README's and is not run here ("Not covered live"); and a bare exact-bytes delete is
what MA3 found unsafe. `r1-clone` and `r1-rt` stay in the run folder with the gate's other local
repositories ("What stays"); the remote they share is the `r1` root, which 7.1 trashes with the
others. The lock tally for 5.11 is unchanged at 4: the seed, the NFC push, the refused push and the
`reftable` clone's push, each one lock session; the clones take none.

### Step 6: the launchd push (MD4(b), O2(b))

Section 6: the Mac on mains power with its lid open, and `caffeinate -is` held for the step's span
(this brief holds it from C.0 to 7.0, Question 7),
because launchd misses a `StartInterval` firing while the system sleeps and a sleeping Mac would
trip the bound for the wrong reason (`caffeinate`'s effect on a locked screen is untested). The
agent fires on its 300 s `StartInterval` with the screen locked; both pushes succeed, no Keychain
prompt appears, its log files show the result, and the remote's ref set then equals the
repository's local branches and tags. If it has not fired within two intervals, or its pushes have
not finished within 30 minutes, BLOCK: a Keychain prompt on a locked screen shows up as exactly
that hang.

**What this brief adds to it, from a real scheduled failure under launchd.** The wrapper's own test
ran with stand-ins, never under launchd (`scripts/macos/gpb-v2-push.sh:54-57`: "Not yet observed
under launchd itself; the live macOS gate checks that the notification arrives"). So, after the
succeeding agent, two failing agents fire for real, each by its `StartInterval`:

- **(a) the banner.** The `fail` agent's pushes fail, so the wrapper posts its notification. The
  wrapper starts `osascript` in a process group of its own [code `gpb-v2-push.sh:161-176`], so
  launchd's kill of the job's group at exit should not reach it. If no banner appears, the
  fallback is `AbandonProcessGroup` set to true (the `abandon` agent), and the record says which
  configuration showed the banner.
- **(b) the reaping.** The `hang` agent's pushes run past a 1-second bound, so the wrapper's
  watchdog ends each push's process group, under real launchd, while the helper or the CLI is
  running. A sampler records the processes of the gate wrapper's process trees, nothing else, and
  afterwards a push killed at its bound must have led its own group and had the helper or the CLI
  alive at the bound in its tree, its helper must not have printed string 14 (a kill that did not
  land lets it finish), and none of it may be left. The check is named exactly: `kit pushtree` on
  the sampler's lines and the agent's two logs, then `strays 'proton-drive|git-remote-proton|gpb-v2-push'`, which prints the pid and program of
  each process whose command line matches, and must print nothing. A run that never shows a
  killed push's helper or CLI is "(b) not exercised", a STOP for Craig, never a PASS
  (Question 4).
- **(c) the permission.** Whether macOS asks for notification permission, and what the run did
  before it was granted (its exit code, logs and marker, which the wrapper writes whatever the
  notification does).

**Why the failing agents write nothing to the account.** Their remote is
`proton::/my-files/GitRemotes/vNEXT-mac-absent-a1/repo`, whose parent does not exist, and
`GPB_CREATE_PARENTS` is never set. The helper's `list for-push` refuses in `EnsureParents`, after
read-only `Stat` calls and before `Bootstrap` or the lock [code
`cmd/git-remote-proton/main.go:740-759`, `internal/repo/parents.go:86-106`], with string 14. Each
of their pushes is read-only on the account and takes no lock. For the `hang` agent the 1-second
bound lands during the helper's first CLI calls (its certified-CLI check and those `Stat`s), each
of which takes about a second [inference from M19's "about 1 s"]; if a push ends inside the bound
anyway, it exits with the parent refusal, not 124, and (b) is recorded as not exercised by it.
*Brief author's call:* a bound shorter than the push, on a read-only push, is how a hang is
produced on purpose; a real hang (a Keychain prompt) cannot be.

**Nor may launchd's own environment set it.** A variable set in launchd's environment
(`launchctl setenv`) reaches every agent of Craig's GUI domain whatever its plist leaves out
[measured 2026-10-28: `launchctl help` gives setenv as "Sets the specified environment variables
for all services within the domain"]. So an inherited `GPB_CREATE_PARENTS=1` would let the
failing agents create the parent this gate never creates, and push beneath it; and an inherited
`GPB_PATH`, `GPB_NOTIFY`, `GPB_MARKER_DIR` or `GPB_PUSH_TIMEOUT` would change what step 6 tests.
Step 0's check of inherited variables sees only the runner's own. So B.0, before the tag, and
6.0, before any gate agent exists, bootstrap a probe agent of the same domain (`envprobe` in
`lib.sh`), with the gate plists' `EnvironmentVariables` and no account access. It lists the names
of the `GPB_`, `PROTON_DRIVE_` and `GIT_` variables it was handed, never a value, and prints its
`HOME` and `PROTON_DRIVE_CACHE_DIR`. Any such name but the plist's own `PROTON_DRIVE_CACHE_DIR`,
that variable with another value, or another `HOME`, is `NOT CLEAN`: at B.0 a STOP before the tag,
at 6.0 a STOP before anything of step 6 is made. *Brief author's call:* the job's own view of its
environment, rather than `launchctl getenv` by name, so that a name this brief does not foresee is
seen too.

**The gate's plist** is the README's example (`scripts/macos/gpb-v2-push.plist.example`) with
section 4's two changes, the cache folder and the interval, and the gate's paths. It runs the
tag's wrapper where the tag's clone holds it, not a copy installed in `~/.local/bin` as the README
installs it: the sampler tells the gate's wrapper from Craig's own by its path, and the wrapper
never reads its own path [code: no `$0` or `BASH_SOURCE` in `gpb-v2-push.sh`]. The README's
install line and a plist in `~/Library/LaunchAgents` are listed under "Not covered live".

- `StartInterval` 300 instead of the daily `StartCalendarInterval`;
- `PROTON_DRIVE_CACHE_DIR` set to the gate cache folder, so that the CLI the wrapper starts uses
  the gate's signed-in session, and so that step 0's re-check after this step (6.5) tests
  something: a wrapper that dropped the variable would send the CLI to M7's default folders, where
  it has no session [plan Task 15, review round 2: Kimi];
- `GPB_PATH` is not set, as the example leaves it, so the wrapper uses its default,
  `$HOME/.local/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin` [code
  `gpb-v2-push.sh:68-70`]: the helper and the CLI come from `~/.local/bin`, and git from
  `/usr/bin/git`, since neither `/opt/homebrew/bin` nor `/usr/local/bin` holds a git [measured
  2026-10-27; 6.0 re-checks]. launchd sets `HOME` for a user agent [inference; the probe agent
  of B.0 and 6.0 checks that it is Craig's];
- the `hang` agent alone adds `GPB_PUSH_TIMEOUT` 1, and the `abandon` agent alone
  `AbandonProcessGroup` true;
- `ProgramArguments` runs `/bin/bash` on the tag's wrapper, `$SRC/scripts/macos/gpb-v2-push.sh`
  (in the tag's clone), with the repository and the remote name; logs go to the run folder.

*Brief author's call:* the plists stay in the run folder and are loaded from there, not copied into
`~/Library/LaunchAgents`, so none can load again at a later login if cleanup is interrupted; and
the failure marker stays at its default folder, `~/Library/Logs`, as the README's example leaves
it, and 7.7 removes this gate's markers by name. `GPB_NOTIFY` is not set, in the plists or (6.0's
probe agent shows) in launchd's environment: the default notification is what is tested.

**6.0 Ready.** The keep-awake Craig started at C.0 must still run, checked by its recorded
pid, and Craig confirms the Mac is on mains power with its lid open, where `-s` takes effect. Then
`gx.sh s6-0-ready`. Its first check is launchd's environment, through the probe agent, before
anything of step 6 is made.

```bash
# file: mac:scripts/s6-0-ready.sh
# Step 6.0: launchd's own environment (a probe agent, first: a STOP before anything is made), mains
# power, the keep-awake (caffeinate -is) held, and the launchd step's folder, repository and plists.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
if [ -e "$G/launchd" ] || [ -e ld ]; then echo "BLOCKED: $G/launchd or $G/ld already exists"; exit 1; fi
echo "== what launchd's own environment hands a gate agent, before any agent, folder or plist of step 6 exists"
envprobe "$G/envprobe"; ev=$?
if [ "$ev" != 0 ]; then echo "6.0: STOP: launchd's environment is not the gate's (the lines above say why); nothing of step 6 was made or loaded. Craig rules, and 6.0 runs again on his word"; exit 3; fi
P=$(pmset -g batt | head -1); echo "power: $P"
case "$P" in *"'AC Power'"*) echo "mains power: yes" ;; *) bad "the Mac is not on mains power (section 6 step 6)" ;; esac
if awake_ok; then pmset -g assertions | grep -E "pid $AWAKE_PID[(]caffeinate[)]"; else bad "the keep-awake is not running (Craig started it at C.0)"; fi
mkdir -p "$G/launchd/logs" || exit 1
for x in /opt/homebrew/bin/git /usr/local/bin/git; do [ -e "$x" ] && bad "$x exists: the wrapper's default GPB_PATH would run it, not the git step 0 checked"; done
gate_repo ld && commit_in ld l.txt 'launchd 1' 'ld 1' || exit 1
git -C ld switch -q -c topic-ld && commit_in ld t.txt 'launchd topic' 'ld topic' && git -C ld switch -q main || exit 1
git -C ld tag -a ld-v1 -m 'gate: an annotated tag' || exit 1
git -C ld remote add proton-v2 "proton::$LDR" && git -C ld remote add proton-fail "proton::$ABSENT/repo" || exit 1
LDREPO=$(cd ld && pwd -P); printf '%s\n' "$LDREPO" > "$G/launchd/repo.txt"; echo "repository: $LDREPO"
WRAP="$SRC/scripts/macos/gpb-v2-push.sh"
[ -f "$WRAP" ] || { echo "BLOCKED: no wrapper at $WRAP"; exit 1; }
echo "wrapper (the tag's): $WRAP  sha256 $(shasum -a 256 < "$WRAP" | cut -d' ' -f1)"
mk() {   # mk <agent> <remote> <bound or empty> <abandon: yes|no>
  {
    cat <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>gpbmac.$RUN.$1</string>
    <key>ProgramArguments</key>
    <array>
        <string>/bin/bash</string>
        <string>$WRAP</string>
        <string>$LDREPO</string>
        <string>$2</string>
    </array>
    <key>StartInterval</key>
    <integer>300</integer>
    <key>StandardOutPath</key>
    <string>$G/launchd/logs/$1.out.log</string>
    <key>StandardErrorPath</key>
    <string>$G/launchd/logs/$1.err.log</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PROTON_DRIVE_CACHE_DIR</key>
        <string>$PROTON_DRIVE_CACHE_DIR</string>
EOF
    if [ -n "$3" ]; then printf '        <key>GPB_PUSH_TIMEOUT</key>\n        <string>%s</string>\n' "$3"; fi
    echo '    </dict>'
    if [ "$4" = yes ]; then printf '    <key>AbandonProcessGroup</key>\n    <true/>\n'; fi
    echo '</dict>'
    echo '</plist>'
  } > "$G/launchd/$1.plist" && plutil -lint "$G/launchd/$1.plist"
}
mk ok proton-v2 '' no || bad "the ok plist"
mk fail proton-fail '' no || bad "the fail plist"
mk abandon proton-fail '' yes || bad "the abandon plist"
mk hang proton-fail 1 no || bad "the hang plist"
for r in proton-v2 proton-fail; do
  s=$(printf '%s\n%s' "$LDREPO" "$r" | cksum | sed 's/ .*//')
  m="$HOME/Library/Logs/gpb-v2-push.$s.failed"
  if [ -e "$m" ]; then bad "a marker already exists: $m (not recorded, so 7.7 never removes it)"
  else printf '%s\n' "$m" > "$G/launchd/marker-$r.txt"; echo "marker for $r (absent now, recorded): $m"; fi
done
echo "== the README example's keys, for the record"; plutil -p "$SRC/scripts/macos/gpb-v2-push.plist.example"
echo "== the ok plist"; plutil -p "$G/launchd/ok.plist"
echo "== Craig's global git configuration, which step 6's runs use (Question 5): key names only, each subsection shown as <subsection>, since a url.<URL>.insteadOf or http.<URL>.* name can carry a credential"
env -u GIT_CONFIG_GLOBAL -u GIT_CONFIG_NOSYSTEM git config --global --name-only --list | sed -E 's/^([^.]+)[.].*[.]([^.]+)$/\1.<subsection>.\2/' | sort -u
kit absentnode "$ABSENT" || bad "the never-created parent exists"
for n in ok fail abandon hang; do launchctl print "gui/$(id -u)/gpbmac.$RUN.$n" >/dev/null 2>&1 && bad "gpbmac.$RUN.$n is already loaded"; done
if [ "$fail" = 0 ]; then echo "6.0: READY"; else echo "6.0: BLOCKED"; fi
exit "$fail"
```

**Pass:** `LAUNCHD ENVIRONMENT: CLEAN`, as at B.0; `mains power: yes`; `keep-awake: pid <n>,
/usr/bin/caffeinate, running` with its two
assertions listed under `pid <n>(caffeinate)`, no `git` in
`/opt/homebrew/bin` or `/usr/local/bin`, four `OK` lines from `plutil -lint`, both markers absent, `ABSENT NODE`, no gate agent already loaded, and
`6.0: READY`. The marker's name is the wrapper's own rule, the first field of `printf '%s\n%s'
"<physical path>" "<remote>" | cksum` [code `gpb-v2-push.sh:95-100`], so a step that reads it uses
the repository's physical path (`pwd -P`), as the wrapper does. Craig's global configuration is
recorded by key name with each subsection shown as `<subsection>`, since a `url.<URL>.insteadOf`
or `http.<URL>.*` name can carry a credential.

`6.0: STOP` (exit 3) means launchd's environment is not the gate's, met before anything of step 6
exists: nothing was made or loaded. Craig rules, and whether 6.0 then runs again is his call (the
retry rule).

**6.1 The succeeding agent, fired by its schedule with the screen locked:** Craig says he is about
to lock the screen; the runner starts `gx.sh s6-1-ok` in the background (at least 90 minutes); Craig
locks the screen (Ctrl-Cmd-Q) within the minute the script waits, and leaves it locked until the
script ends.

```bash
# file: mac:scripts/s6-1-ok.sh
# Step 6.1: the ok agent, fired by its 300 s StartInterval with the screen locked; both pushes
# succeed; then the remote's ref set equals the repository's branches and tags. "gx.sh s6-1-ok
# refire" (on Craig's word, 6.1's re-fire) first commits on ld's main and makes one new branch,
# so the re-fire has something to push.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
LDREPO=$(cat "$G/launchd/repo.txt"); export LDREPO
mode="${2:-first}"
case "$mode" in first|refire) ;; *) echo "BLOCKED: mode $mode (first or refire)"; exit 1 ;; esac
awake_ok || { echo "BLOCKED: the keep-awake is not running"; exit 1; }
if [ "$mode" = refire ]; then
  [ -e "$G/launchd/logs/ok.out.log" ] || { echo "BLOCKED: no earlier ok firing to re-fire after"; exit 1; }
  k=1; while git -C ld rev-parse -q --verify "refs/heads/topic-ld-r$k" >/dev/null; do k=$((k + 1)); done
  b="topic-ld-r$k"
  commit_in ld l.txt "launchd re-fire $k" "ld re-fire $k" && git -C ld branch "$b" || exit 1
  echo "re-fire $k: main now at $(git -C ld rev-parse main); new branch $b"
fi
echo "waiting 60 s for Craig to lock the screen"; sleep 60
fire ok 1800 || { echo "BLOCKED: section 6 step 6's bound (two intervals to fire, 30 minutes to finish), or a firing the bootout cut (fire's lines above)"; exit 1; }
echo "== the gate cache folder: files the scheduled run's CLI wrote or changed (nothing else has run the CLI since the bootstrap)"
find "$PROTON_DRIVE_CACHE_DIR" -type f -newer "$G/launchd/ok.stamp" | sed "s|^$PROTON_DRIVE_CACHE_DIR/|  |"
[ -n "$(find "$PROTON_DRIVE_CACHE_DIR" -type f -newer "$G/launchd/ok.stamp")" ] || bad "nothing in the gate cache folder changed: did the wrapper hand PROTON_DRIVE_CACHE_DIR on?"
kit expect "$G/launchd/logs/ok.out.log" wrapper-ok || bad "the wrapper did not log both pushes succeeding"
e="$G/launchd/logs/ok.err.log"
if [ "$mode" = first ]; then
  for r in main topic-ld; do grep -qE "[[]new branch[]] +$r -> $r" "$e" || bad "no [new branch] line for $r"; done
  grep -qE '[[]new tag[]] +ld-v1 -> ld-v1' "$e" || bad "no [new tag] line for ld-v1"
else
  grep -qE '^ +[0-9a-f]+[.][.][0-9a-f]+ +main -> main' "$e" || bad "no fast-forward line for main"
  grep -qE "[[]new branch[]] +$b -> $b" "$e" || bad "no [new branch] line for $b"
fi
kit helperlines "$G/launchd/logs/ok.err.log"
d5_check "$G/launchd/logs/ok.err.log"
m=$(cat "$G/launchd/marker-proton-v2.txt"); if [ -e "$m" ]; then bad "the succeeding run left a marker"; else echo "marker for proton-v2: absent"; fi
git -C ld ls-remote --heads --tags proton-v2 > "$G/refsets/ld-6-remote.txt" || bad "ls-remote failed"
git -C ld for-each-ref --format='%(objectname) %(refname)' refs/heads refs/tags > "$G/refsets/ld-6-local.txt"
kit refcmp ld-6-local ld-6-remote || bad "the remote's ref set is not the repository's branches and tags"
m7 || bad "an M7 default folder exists after the scheduled run"
kit roots-update || bad "the ownership record"
echo "Craig may unlock the screen now, and reports any dialog on it (a Keychain prompt above all)."
if [ "$fail" = 0 ]; then echo "6.1: PASS"; else echo "6.1: BLOCKED"; fi
exit "$fail"
```

**Pass:** `6.1: PASS`, with all of the following.

- `FIRED <n> s after the bootstrap` with `n` at most 600 (the firing came from the schedule: the
  script never kickstarts it), and the screen-lock lines read `true` at the bootstrap and at the
  firing. A `no (key absent)` there means the screen was not locked [measured 2026-10-27: the key
  is absent on an unlocked screen; B.0's positive control reads it on a locked one]: recorded, and
  Craig decides whether 6.1 is re-fired with the screen locked, by `gx.sh s6-1-ok refire`, which
  gives the re-fire a new commit on `main` and a new branch to push.
- The run ended within 30 minutes; `launchctl print` shows `last exit code = 0`; the bootout exits
  0; and `FIRINGS in ok.out.log, counted after the bootout: <n> started, <n> ended`, the two
  counts equal. One firing (`runs = 1`, `1 started, 1 ended`) is expected. Two or more, every one
  ended (a second firing before the bootout, which `runs = 2` also shows when it began before the
  print), pass as well: the later firings push nothing, the record names the count, and each
  firing adds two `.lock` files to the tally (6.4). More started than ended means the bootout cut
  a firing: BLOCKED (`fire` returns 1).
- `EXPECT wrapper-ok: FOUND` (both pushes exit 0 and the `done:` line, the repository named by its
  physical path); the error log holds git's `[new branch]` lines for `main` and `topic-ld` and
  `[new tag]` for `ld-v1` (on a re-fire: a fast-forward line for `main` and one `[new branch]` line
  for its `topic-ld-r<k>`), each checked by the script; `kit helperlines`' `others: 0`, and neither
  D5 warning (a first "could not check" is recorded, `d5_check`); no marker.
- `REFSET ld-6-local == ld-6-remote: EQUAL (3 ref(s))`, one more for each re-fire's branch: the
  remote holds exactly the repository's branches and tags, so a push that sent less than
  everything cannot pass.
- The gate cache folder holds files the scheduled run wrote, listed before the script's own
  `ls-remote` and `roots-update` run the CLI, so no other command can have written them; and M7's
  defaults are absent.
- `RECORDED: .../vNEXT-mac-launchd-a1`.
- **Craig's report**, recorded verbatim: no Keychain prompt and no other dialog on the screen when
  he unlocked it. A prompt is BLOCKED.

`NOT FIRED` or `NOT FINISHED`: BLOCKED, the agent already booted out by `fire`. A second firing
can start before the bootout: the interval counts from the bootstrap (`launchd.plist(5)`: the
key "causes the job to be started every N seconds") [doc], and `fire` boots the agent out up to
about a minute after its run ends, so a run that ends near a 300 s boundary can be followed by
another firing; a run longer than 300 s makes launchd skip the firings it overlaps ("If the job is
running during an interval firing, that interval firing will likewise be missed") [doc]. `fire`
prints `runs =` just before the bootout, so a firing that starts between that print and the
bootout is missed there; its `FIRINGS` line, counted from the out log after the bootout, is the
count. Each firing adds two `.lock` files to the tally.

**A push that fails at 6.1 is BLOCKED, whatever refused it.** Step 6's runs are the only gate
pushes made with Craig's own git configuration (Question 5), and 6.1's are the only ones of those
that get as far as his `core.hooksPath` hooks: 6.2's and 6.3's are refused at `list for-push`,
before git runs `pre-push` [inference from git's push order]. So the error log shows whether one
of his hooks, rather than the helper, refused a push that fails at 6.1, and the record names which.

**6.2 A failing agent: the banner, and the permission.** Craig unlocks the screen and watches it;
the runner runs `gx.sh s6-2-fail` in the background.

```bash
# file: mac:scripts/s6-2-fail.sh
# Step 6.2: the fail agent (a remote whose parent is missing), fired by its StartInterval with the
# screen unlocked: both pushes fail, the wrapper writes its marker and posts its notification.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
LDREPO=$(cat "$G/launchd/repo.txt"); export LDREPO
agent="${2:-fail}"
case "$agent" in fail|abandon) ;; *) echo "BLOCKED: agent $agent"; exit 1 ;; esac
echo "agent: $agent; screen locked now: $(locked) (Craig watches an unlocked screen)"
fire "$agent" 600 30 || { echo "BLOCKED: the $agent agent did not fire or end in time, or the bootout cut a firing"; exit 1; }
o="$G/launchd/logs/$agent.out.log"; e="$G/launchd/logs/$agent.err.log"
grep -E 'gpb-v2-push: git push proton-fail --(all|tags): exit [1-9]' "$o" || bad "the wrapper did not log two failed pushes"
kit expect "$e" parent-absent || bad "the helper's missing-parent refusal"
m=$(cat "$G/launchd/marker-proton-fail.txt")
if [ -f "$m" ]; then echo "== the marker $m"; kit esc "$m"; cp "$m" "$G/launchd/marker-after-$agent.txt"; else bad "no marker at $m"; fi
if [ "$m" -nt "$G/launchd/$agent.stamp" ]; then echo "marker written by this firing: yes"; else bad "the marker was not written by this firing"; fi
grep -q '^failed: git push proton-fail --all (exit ' "$m" && grep -q '^failed: git push proton-fail --tags (exit ' "$m" || bad "the marker does not name both failed pushes"
kit absentnode "$ABSENT" || bad "the missing parent exists now"
echo "Craig: did a banner from git-proton-backup appear, with what text? Did macOS ask to allow notifications? Is it in Notification Center?"
if [ "$fail" = 0 ]; then echo "6.2 ($agent): THE RUN FAILED AS EXPECTED; the banner is Craig's to report"; else echo "6.2 ($agent): BLOCKED"; fi
exit "$fail"
```

**Pass (the run):** the agent fired within 600 s and ended; the out log shows both pushes failing
with a nonzero code (`launchctl print` shows the wrapper's `last exit code`, the first push's
code); `EXPECT parent-absent: FOUND` in the error log (string 14); `marker written by this firing:
yes` (a re-run cannot pass on an earlier run's marker), and the marker names the repository, the
remote and both failed pushes; the missing parent is still absent. `fire` waits 30 s after the run
has ended before the bootout, so a notification the run started is not cut short by it
[inference: untested whether a bootout reaches `osascript`'s own group].

**Craig's observations, recorded verbatim (a, c):** whether a banner titled `git-proton-backup`
appeared, and its text (expected: `git-proton-backup: push failed for <repository> (proton-fail).
failed: git push proton-fail --all (exit <n>) failed: git push proton-fail --tags (exit <n>) See
<marker>` [code `gpb-v2-push.sh:254-257`]); whether macOS asked for permission to send
notifications, naming which app (osascript's notifications are commonly attributed to Script
Editor [inference]), and the prompt's text; and whether the notification is in Notification Center.

- **A permission prompt appeared.** Allowing it is Craig's decision (a system setting). The run
  before it was granted is the one just recorded: its exit code, logs and marker do not depend on
  the notification, which the wrapper never waits on [code `gpb-v2-push.sh:161-176`]. If Craig
  allows it, the runner runs `gx.sh s6-2-fail` again, and the second firing's banner is what (a)
  records.
- **No prompt and no banner.** The fallback: the runner runs `/bin/bash ~/gpbmac/kit/gx.sh
  s6-2-fail abandon`, which fires the `abandon` agent (`AbandonProcessGroup` true), and Craig
  watches again.
- **The record says which configuration showed the banner:** the default one, the
  `AbandonProcessGroup` one, or neither. Neither is a finding for the README and the wrapper,
  reported to Craig; it is not a BLOCK of the release, since a failed run is still visible in its
  log and marker (spec section 4).

**6.3 A hung push, reaped under launchd (b).** The runner starts the sampler in the background
(`gx.sh s6-sampler`, at least 90 minutes) and then runs `gx.sh s6-3-hang`. The sampler records
the gate wrapper's process trees and nothing else (`kit/tree.awk`, with the brief author's call
and the measurement beside it): each `/bin/bash` whose command line holds the tag's wrapper path,
`$SRC/scripts/macos/gpb-v2-push.sh`; every process descended from one; and every process in a
process group one of those leads. The wrapper starts each push and its watchdog under `set -m`,
each in a group of its own [code `gpb-v2-push.sh:184-196`], so a push's helper and CLI are
followed by parent and by group, under any program name, and a process whose parent has died is
still followed through its group while anything of the firing runs. The filter runs before
anything is written. Each sample carries its time to a hundredth of a second, and every sample of
every process is kept.

**What tells a kill that landed from one that did not.** The wrapper logs `ran past 1s and was
killed`, and exit 124, whenever its watchdog's timer ran out, whatever the group kill then did,
and its timing is the same either way [code `gpb-v2-push.sh:186-210`]. The helper has no signal
handling, so a kill that lands stops it before it can print string 14; a kill that does not land
lets it finish, printing string 14 [inference from the code: no `signal.` use in `main.go` or
`cli.go`]. So `kit pushtree` counts a killed push as reaped only when all of these hold: the push
leads its own process group (the watchdog's kill is a group kill, so a push that does not is
BLOCKED); its tree held the helper or the CLI, still seen within 0.5 s of the bound, counted from
the push's first sample, or later; and the error log holds string 14 exactly once for each push
that was not killed, none for a killed one. A killed push that still printed string 14 is "(b) not
exercised": either its helper ended just before the kill landed (a push that ends within a moment
of the bound is still logged as killed, since the wrapper reads the watchdog's mark only after the
push has ended [code `gpb-v2-push.sh:198-204`]), or the kill did not stop it; the logs cannot tell
which, and neither shows a reaping. Any other `git-remote-proton:` line in the error log is
BLOCKED, so that two anomalies cannot cancel in the count (string 17, a pipe held past exit, is
listed and recorded, as everywhere); and the out log must hold one firing, or the run is "(b) not
exercised".

```bash
# file: mac:scripts/s6-sampler.sh
# Step 6.3's sampler: five times a second, the time (epoch seconds, to 0.01 s), pid, pgid, ppid and
# command of each process in the gate wrapper's process trees (kit/tree.awk), and of no other
# process; the filter runs before anything is written. The groups tree members lead are
# remembered while anything of the trees runs, and forgotten once nothing does. An earlier
# firing's samples are moved aside first. It stops when 6.3 leaves its stop file, or after two hours.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
S="$G/launchd/samples.txt"; K="$G/launchd/sample-groups.txt"; W="$SRC/scripts/macos/gpb-v2-push.sh"
for k in 1 2 3 4 5 6 7 8 9; do [ -e "$S.$k" ] || break; done
if [ -e "$S" ]; then mv "$S" "$S.$k" && echo "an earlier firing's samples moved aside: $S.$k"; fi
rm -f "$G/launchd/stop-sampler"; : > "$K"
end=$(( $(/bin/date +%s) + 7200 ))
echo "sampler started; writing $S; following $W"
while [ ! -e "$G/launchd/stop-sampler" ] && [ "$(/bin/date +%s)" -lt "$end" ]; do
  : > "$K.new"
  /bin/ps -axo pid=,pgid=,ppid=,command= | TREE_WRAP="$W" /usr/bin/awk -v known="$(/usr/bin/tr '\n' ' ' < "$K")" -v pgout="$K.new" -f "$HOME/gpbmac/kit/tree.awk" > "$G/launchd/sample.tmp" || { echo "BLOCKED: tree.awk failed"; exit 1; }
  if [ -s "$G/launchd/sample.tmp" ]; then
    t=$(/usr/bin/perl -MTime::HiRes=time -e 'printf "%.2f", time')
    /usr/bin/sed "s/^ */$t /" "$G/launchd/sample.tmp" >> "$S"
    /usr/bin/sort -u "$K" "$K.new" > "$K.tmp" && mv "$K.tmp" "$K"
  else
    : > "$K"
  fi
  sleep 0.2
done
rm -f "$K" "$K.new" "$K.tmp" "$G/launchd/sample.tmp"
if [ -f "$S" ]; then echo "sampler stopped: $(wc -l < "$S" | tr -d ' ') line(s)"; else echo "sampler stopped: 0 line(s), nothing of a gate wrapper seen"; fi
```

```bash
# file: mac:scripts/s6-3-hang.sh
# Step 6.3: the hang agent's pushes run past a 1 s bound, and the wrapper's watchdog ends each
# push's process group under real launchd. PASS needs a push killed at its bound, leading its own
# group, whose tree held the helper or the CLI alive at the bound, with no string 14 from it (the
# kill stopped it), and nothing of any push still running. A run that never shows that is (b) not
# exercised: a STOP for Craig (exit 3), never a PASS.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
LDREPO=$(cat "$G/launchd/repo.txt"); export LDREPO
pgrep -f 's6-sampler[.]sh' >/dev/null || { echo "STOP: the sampler is not running, so nothing was fired. The runner starts it (gx.sh s6-sampler, in the background, at least 90 minutes), then runs gx.sh s6-3-hang again"; exit 3; }
fire hang 600 || { echo "BLOCKED: the hang agent did not fire or end in time, or the bootout cut a firing"; exit 1; }
sleep 15; : > "$G/launchd/stop-sampler"; sleep 2
o="$G/launchd/logs/hang.out.log"; e="$G/launchd/logs/hang.err.log"
grep -E 'gpb-v2-push: git push proton-fail --(all|tags)( ran past|: exit)' "$o"
killed=$(grep -c 'gpb-v2-push: git push proton-fail --[a-z]* ran past 1s and was killed' "$o")
echo "pushes killed at their bound: $killed"
echo "the --all push: 'ran past' lines $(grep -c 'gpb-v2-push: git push proton-fail --all ran past 1s and was killed' "$o"), 'exit 124' lines $(grep -c 'gpb-v2-push: git push proton-fail --all: exit 124' "$o") (recorded)"
echo "string 14 lines in hang.err.log: $(grep -c '^git-remote-proton: parent folder ' "$e") (one for each push not killed, none from a killed push; kit pushtree judges)"
m=$(cat "$G/launchd/marker-proton-fail.txt"); echo "== the marker"; kit esc "$m"
if [ "$m" -nt "$G/launchd/hang.stamp" ]; then echo "marker written by this firing: yes"; else bad "the marker was not written by this firing"; fi
if [ "$killed" -gt 0 ]; then kit expect "$m" marker-lock || bad "a push was killed, and the marker lacks the killed-push sentence"; fi
echo "== every recorded process of the two pushes"
kit pushtree "$G/launchd/samples.txt" "$o" "$e"; t=$?
notex=0
case "$t" in 0) ;; 3) notex=1 ;; *) bad "kit pushtree BLOCKED (its lines above say why)" ;; esac
echo "== stray processes now (pid and program of each command line matching proton-drive|git-remote-proton|gpb-v2-push): none expected"
if strays 'proton-drive|git-remote-proton|gpb-v2-push'; then bad "a stray process is running"; else echo "(none)"; fi
kit absentnode "$ABSENT" || bad "the missing parent exists now"
if [ "$fail" != 0 ]; then echo "6.3: BLOCKED"; exit 1; fi
if [ "$notex" = 1 ]; then echo "6.3: (b) NOT EXERCISED: STOP for Craig"; exit 3; fi
echo "6.3: PASS"
```

**Pass:** `6.3: PASS`, which needs all of the following.

- At least one push killed at its bound (`pushes killed at their bound:` 1 or 2; the wrapper logs
  `ran past 1s and was killed` and exit 124 for it [code `gpb-v2-push.sh:198-210`]), and `kit
  pushtree` showing, for such a push, `its own group: yes`, `helper or CLI seen:` 1 or more and
  `ALIVE AT THE BOUND`. Record every line, and record whether `the CLI itself` was among them (a
  `proton-drive` process).
- `STRING 14: <n> line(s) in hang.err.log; pushes ended: 2, killed at the bound: <k>, so <2-k>
  expected`, with n equal to 2 - k, as the script's own `string 14 lines` count shows. More means
  a push logged as killed still refused, so no reaping is shown: `(b) NOT EXERCISED`. Fewer means
  a push that was not killed ended without the missing-parent refusal: BLOCKED.
- `OTHER git-remote-proton: LINES in hang.err.log: 0`; any other line there is BLOCKED (string
  17's are listed as known and recorded). `FIRINGS in hang.out.log: 1`; another count is `(b) NOT
  EXERCISED`, and `fire`'s own `FIRINGS` line shows it too.
- Then `(b) EXERCISED`, and `PUSH TREE: <n> push root(s), 0 still running`, with n 1 or 2: the
  `--all` push, and the `--tags` push when the sampler saw it; `kit pushtree` needs at least one.
  A process of a tree in another process group is marked `(OTHER GROUP)`: a finding to record
  even when it is gone; one still running is BLOCKED. A push that does not lead its own group is
  BLOCKED.
- `marker written by this firing: yes`, and with a push killed, `EXPECT marker-lock: FOUND` [code
  `gpb-v2-push.sh:221`]. No lock can be left here (the pushes take none), so the sentence's case
  does not arise; it is checked for its text.
- `strays` prints nothing, then `(none)`: no stray `proton-drive`, `git-remote-proton` or wrapper
  process.
- Craig reports whether a second banner appeared (recorded).

**(b) not exercised.** A push that ends inside its bound exits nonzero with string 14; that is not a
BLOCK, and the `--all` push's `ran past` and `exit 124` counts are recorded either way. But when
no push was reaped as above, the script ends `6.3: (b) NOT EXERCISED: STOP for Craig` (exit 3),
never PASS, and Craig decides whether 6.3 is fired again (Question 4). A re-firing starts the
sampler again first (6.3's run stopped it), then `gx.sh s6-3-hang`, which STOPs (exit 3) and fires
nothing while no sampler runs. It is judged on its own evidence alone: the sampler moves the
earlier samples aside (`samples.txt.<k>`), `fire` moves the earlier logs aside, and the marker must
be newer than this firing's stamp.

**6.4 The launchd step's `.lock` count.** Each `ok` firing made two lock sessions, `--all` and
`--tags`: the first, any re-fire (`s6-1-ok refire`), and any second firing a `FIRINGS` line shows.
Count them from `fire`'s `FIRINGS ...: <n> started` lines in `log/s6-1-ok.txt`, one for each
bootstrap; its `runs =` lines are printed before the bootout and can miss a firing. The failing
agents made none. A re-fire is a fresh bootstrap after a bootout, with its logs moved aside first,
so its `runs` and its `FIRINGS` count from 1 again [inference].

**6.5 Step 0's re-check, after step 6:** `gx.sh s6-5-after`.

```bash
# file: mac:scripts/s6-5-after.sh
# Step 6.5: no gate agent is loaded, the gate session still works from the gate cache folder,
# which still holds its files, M7's defaults are absent, and the missing parent still is (section 6
# step 0, "re-checked after step 6").
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
for n in ok fail abandon hang envprobe; do
  if launchctl print "gui/$(id -u)/gpbmac.$RUN.$n" >/dev/null 2>&1; then launchctl bootout "gui/$(id -u)/gpbmac.$RUN.$n"; echo "booted out gpbmac.$RUN.$n: exit $?"; else echo "gpbmac.$RUN.$n: not loaded"; fi
done
proton-drive filesystem info /my-files --json > "$G/out/6-5-info.json" 2> "$G/out/6-5-info.err"; irc=$?
echo "read-only info: exit $irc"; [ "$irc" = 0 ] || { kit esc "$G/out/6-5-info.err"; bad "the read-only info failed"; }
echo "gate cache folder: $(find "$PROTON_DRIVE_CACHE_DIR" -type f | wc -l | tr -d ' ') file(s); changed since 6.1 began: $(find "$PROTON_DRIVE_CACHE_DIR" -type f -newer "$G/launchd/ok.stamp" | wc -l | tr -d ' ') (recorded, not judged: the runner's own CLI calls count there too)"
[ -n "$(find "$PROTON_DRIVE_CACHE_DIR" -type f)" ] || bad "the gate cache folder holds no file"
m7 || bad "an M7 default folder exists"
kit absentnode "$ABSENT" || bad "the missing parent exists"
if [ "$fail" = 0 ]; then echo "6.5: PASS"; else echo "6.5: BLOCKED"; fi
exit "$fail"
```

**Pass:** every agent `not loaded` or booted out, the info exits 0 (the session in the gate cache
folder still works), the folder still holds files (step 0's check; the count changed since 6.1 is
recorded, not judged, since the runner's own CLI calls touch the folder too, and 6.1's list is the
scheduled run's alone), the three `M7 default absent` lines, `ABSENT NODE`, and `6.5: PASS`. A
failed info is BLOCKED.

### Step 7: cleanup, in order, on every exit path, including after a BLOCK

Section 6: "first unload the LaunchAgent (`launchctl bootout`) and wait for any push it started to
finish, so no scheduled write can land during cleanup; then trash the gate's subtrees,
verify-before-trash (rule 3); check the row set and tally trash (rules 1, 7); `auth logout`,
confirm the Keychain item is gone, and fall back to `security delete-generic-password` as in the
probe; delete the scratch v0.10.0 build; confirm M7's default folders are still absent; the Windows
check; delete the gate cache folder; unpause the task only if the Windows check passed."

*Brief author's call:* each sub-step checks for what its work depends on and, when that is missing,
prints why it is skipped and changes nothing; a sub-step that fails is recorded verbatim and the
runner goes on to the next, except that nothing is trashed or deleted while a writer may still run
(7.0), and the cache folder is deleted only after a recorded logout (7.3).

| Sub-step | Does its work only if | Otherwise |
|---|---|---|
| 7.0 writers | always | none |
| 7.1 trash | `roots.tsv` exists (step 0's checks passed), and each root passes `vet` and its uid check | `NO ROOT RECORD`, `NOT RECORDED`, `ALREADY GONE` or `NOT TRASHED` |
| 7.2 row set and tally | always lists; compares only against a baseline that exists | each missing comparison says why |
| 7.3 logout | always; with no CLI installed, or no sign-in ever completed, it records `no session` after checking there is none | no record (a check failed): B.6's fallback |
| 7.4 v0.10.0 build, 7.5 M7 | always | none |
| 7.6 Windows check | always (W.7.6) | none |
| 7.7 cache folder and markers | 7.3 recorded a logout | `7.7 KEPT`, and the PC's tasks stay paused until a logout is recorded |
| 7.8 unpause | always decided, by the PC's ledger of Windows checks (W.7.8) | none |

**7.0 No writer is still running.** `gx.sh s7-0-quiet`, then, on the PC, W.7.0.

```bash
# file: mac:scripts/s7-0-quiet.sh
# Step 7.0: every gate agent unloaded, and no push, helper, CLI or wrapper process still running,
# nor a go test (step 2.1's launcher, which can still start the live table) or its test binary.
# strays prints only the pid and program of each matching process, never its arguments.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
for n in ok fail abandon hang envprobe; do
  if launchctl print "gui/$(id -u)/gpbmac.$RUN.$n" >/dev/null 2>&1; then launchctl bootout "gui/$(id -u)/gpbmac.$RUN.$n"; echo "booted out gpbmac.$RUN.$n: exit $?"; else echo "gpbmac.$RUN.$n: not loaded"; fi
done
if [ -d "$G/launchd" ]; then : > "$G/launchd/stop-sampler"; fi
WP='proton-drive|git-remote-proton|gpb-v2-push|git( -C [^ ]+)? (push|fetch|clone) |[.]test( |$)|(^|/)go test( |$)'
for k in 1 2 3 4 5 6 7 8 9 10 11 12; do
  strays "$WP" >/dev/null || break
  echo "waiting for a writer to finish (check $k of 12):"; strays "$WP"; sleep 10
done
if strays "$WP"; then echo "MAC WRITERS: RUNNING (wait, then run 7.0 again; trash and delete nothing meanwhile)"; exit 1; fi
echo "MAC WRITERS: NONE"
echo "HANDOFF -> PC: W.7.0"
```

**Pass:** `MAC WRITERS: NONE`, and W.7.0's `WINDOWS WRITERS: NONE`. Neither: wait and run 7.0
again; nothing is trashed or deleted meanwhile. A writer is listed by pid and program only. A
`go test` counts as one, as its test binary does: after an interrupted step 2.1, its launcher can
still be compiling, and then start the live table.
Then Craig presses Ctrl-C in the keep-awake's window (held since C.0), and the runner checks that
one pid: `/bin/ps -p "$(/bin/cat ~/gpbmac/gate-a1/caffeinate.pid)" -o pid=,comm=; echo "ps: exit
$?"`. It must print only `ps: exit 1`. A line naming `/usr/bin/caffeinate` means it still runs:
Craig ends it in its window (or with `kill <pid>`) and the runner checks again. Step 7's scripts
never wait on it: cleanup runs on every exit path, keep-awake or not.

**7.1 Trash the gate's subtrees, verify-before-trash (rule 3).** First the enumeration, read-only:
`gx.sh s7-1a-enumerate`.

```bash
# file: mac:scripts/s7-1a-enumerate.sh
# Step 7.1, first half: every recorded root's full subtree, checked against what the gate writes.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
kit roots-update || exit 1
for n in $(cut -f1,3 "$G/roots.tsv" 2>/dev/null | grep -E $'\tcreated$' | cut -f1); do
  if [ "$n" = contract-mac ]; then p="$CONTRACT"; else p="$R/$n"; fi
  echo "== $p"; kit vet "$p" || fail=1
done
exit "$fail"
```

**The runner checks each subtree against the gate's frames** (the frame is `gpb-remote.json`,
`HEAD`, `refs`, `refs/heads`, `refs/tags` and `packs`, with `pack-<hex>.pack`/`.idx` pairs; names
in the escaped view):

| Root | `refs/heads` holds | `refs/tags` | Pack pairs |
|---|---|---|---|
| `smoke` | `main`, `side` | none | 2 (3.1, 3.3) |
| `xm` | `main` | none | 2 (4.1, 4.5) |
| `xw` | `main` | none | 2 (4.3, 4.7) |
| `case` | `Main`, `feat`, `main`, `side` | none | 2 (5.3's seed and mixed push) |
| `nfd` | `cafe<U+0301>`, `caf<U+00E9>`, `main`, `side` | none | 4 (N1, NC, N2, W3) |
| `ren` | `cafe<U+0301>`, `main` | none | 1 (5.8's seed) |
| `mp1` | `caf<U+00E9>`, `main`, `side` | none | 2 (M1, M2) |
| `mp3` | `main` | none | 1 |
| `r1` | `caf<U+00E9>`, `main` | none | 1 |
| `launchd` | `main`, `topic-ld`, and each 6.1 re-fire's `topic-ld-r<k>` | `ld-v1` | 2 (the `--all` push's, and the `--tags` push's tag object, which no branch reaches), plus 1 for each re-fire (its new commit); a second firing that a `FIRINGS` line shows adds none, having nothing to push, though it adds two `.lock` files (6.4) |
| `contract-mac` | empty after a step 2 that passed; after a step-2 BLOCK, `TestContractCLI_*` folders and their contents, gate debris that goes with the root | | |

A root missing from the record was never created: nothing to trash. **After a BLOCK,** a root
that passes `vet` and holds a subset of its row (the refs and pack pairs of the steps that ran, by
their logs) is trashed like a complete one; anything else is a STOP. `VET ...: ONLY GATE
ARTIFACTS` is the kit's check that no row lies outside these shapes; the table is the runner's,
by eye. A `LIVE LOCK` line is this gate's own (a push killed mid-flight); record it, and it goes
with its root. **A `FOREIGN` row, or a subtree that differs from its row of the table:** STOP for
Craig, and that root is not trashed (`$skip`, below).

Then the trash, each root re-read and re-enumerated just before it: `gx.sh s7-1b-trash`, followed
by the name of each root that failed 7.1a's check, if any (for example `gx.sh s7-1b-trash
vNEXT-mac-case-a1`); with no name, every recorded root is trashed.

```bash
# file: mac:scripts/s7-1b-trash.sh
# Step 7.1, second half: each recorded root trashed only if its uid is still the recorded one and
# its subtree still holds only gate artifacts; then GitRemotes, only if step 2.0 made it.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
shift; skip="$*"   # the roots that failed 7.1a's check, named on gx.sh's command line
echo "roots skipped (failed 7.1a): ${skip:-none}"
for n in $(cut -f1,3 "$G/roots.tsv" 2>/dev/null | grep -E $'\tcreated$' | cut -f1); do
  case " $skip " in *" $n "*) echo "SKIPPED (failed 7.1a; left for Craig): $n"; continue ;; esac
  kit trashroot "$n" || fail=1
done
kit trashgr || fail=1
if grep -q $'^GitRemotes\t[^\t]*\tpresent-at-step-0\t' "$G/roots.tsv" 2>/dev/null; then
  kit snapshot gitremotes-final "$R" && kit compare gitremotes-0 gitremotes-final || echo "GITREMOTES COMPARISON NOT AVAILABLE"
fi
exit "$fail"
```

**Expected:** each `trash <root> (uid ...): exit 0 ...` with the CLI's `ok:true` answer and its
time; then `GitRemotes:` either trashed (this run made it and nothing live is left) or the line
saying why it stays. If `GitRemotes` was present at step 0, and only then (one this run made has
gone to the trash with `trashgr`): `COMPARE gitremotes-0 ->
gitremotes-final: NO DIFFERENCE`, or differences that are only rows of skipped roots, so every
child that is not this run's is as it was; anything else is a STOP for Craig. `kit trashgr`
enumerates `GitRemotes`' whole live subtree first (rule 3): a row that is not one of this run's
roots is a STOP, and a live root left in place keeps `GitRemotes`. **To resume after a kill inside 7.1b:** run 7.0, then 7.1b
alone (`trashroot` reports a root already trashed as `ALREADY GONE`); a kill inside 7.1a is a STOP
for Craig, since a root whose enumeration did not finish is never trashed (rule 3).

**7.2 The row set (rule 1) and the trash tally (rule 7):** `gx.sh s7-2-rowset`.

```bash
# file: mac:scripts/s7-2-rowset.sh
# Step 7.2: /my-files against step 0's baseline, and the trash against its own.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
kit snapshot myfiles-final /my-files || echo "MYFILES LISTING FAILED"
if [ -e "$G/snap/myfiles-0.json" ]; then kit compare myfiles-0 myfiles-final; else echo "7.2 COMPARISON SKIPPED: no myfiles-0 (the gate stopped before step 0's baseline)"; fi
kit snapshot casprobe-final /my-files/_cas-probe && kit compare casprobe-0 casprobe-final || echo "CASPROBE COMPARISON NOT AVAILABLE"
kit snapshot trash-final /trash && kit compare trash-0 trash-final || echo "TRASH COMPARISON NOT AVAILABLE (recorded, not a BLOCK; see C.5)"
```

**Pass**, where the baselines exist:

- `COMPARE myfiles-0 -> myfiles-final: NO DIFFERENCE`, or, only if `GitRemotes` was present at
  step 0, exactly one `CHANGED /my-files/GitRemotes: modificationTime ...` (the stage6 brief's
  rule; 0.9.0 observed no change at all). A `TRASHED, STILL LISTED` line for a node 7.1 trashed is
  recorded. Anything else is BLOCKED, and cleanup goes on.
- `casprobe`: `NO DIFFERENCE` against step 0 (supplementary: `contract-mac` came and went).
- **The tally:** each `ADDED` row of the trash comparison classified against "Confinement"'s
  classes (39 `.lock` files plus 2 for each `ok` firing that 6.4 counts, four ref files, 19 contract folders, eleven roots
  and their descendants, `GitRemotes` if 2.0 made it), counted by class and type. A `REMOVED` row
  means someone restored or emptied an item during the gate: reported verbatim. If the trash cannot
  be listed, the tally is those classes, checked against what Craig sees in the web UI (C.5).

**7.3 `auth logout`, and the Keychain item:** `gx.sh s7-3-logout`.

```bash
# file: mac:scripts/s7-3-logout.sh
# Step 7.3: auth logout; the CLI then has no session, and the Keychain item is absent. With no CLI
# installed, or no sign-in ever completed (no "EXIT c4-check = 0"), it records "no session".
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
now=$(/bin/date -u '+%Y-%m-%dT%H:%M:%SZ')
if [ -s "$G/logout.state" ]; then echo "a logout is already recorded: $(cat "$G/logout.state")"; exit 0; fi
if [ ! -x "$HOME/.local/bin/proton-drive" ]; then
  kc_expect absent "a Keychain item exists, though no CLI was installed"
  if [ "$fail" = 0 ]; then echo "no session: no CLI was installed (C.3 did not get that far), item absent, $now" > "$G/logout.state"; echo "7.3: no session (recorded)"; fi
  exit "$fail"
fi
logged 7-3-logout proton-drive auth logout; lrc=$rc
echo "auth logout: exit $lrc"
logged 7-3-after proton-drive filesystem info /my-files --json; prc=$rc
echo "post-logout info: exit $prc (expected nonzero)"
kit expect "$L" login-first || bad "the CLI still has a session"
kc_expect absent "the Keychain item remains (Craig deletes it: B.6's fallback)"
if [ "$fail" = 0 ] && [ "$prc" != 0 ] && [ "$lrc" = 0 ]; then echo "logged out, item absent, $now" > "$G/logout.state"; echo "logout recorded"
elif [ "$fail" = 0 ] && [ "$prc" != 0 ] && ! grep -q '^EXIT c4-check = 0 ' "$G/log/runs.txt" 2>/dev/null; then
  echo "no session: no sign-in ever completed (no EXIT c4-check = 0); auth logout exit $lrc, no session, item absent, $now" > "$G/logout.state"; echo "7.3: no session (recorded)"
else echo "no logout recorded (see above)"; fi
exit "$fail"
```

**Pass:** `auth logout: exit 0`, `EXPECT login-first: FOUND`, `... absent` and `logout recorded`;
or, after a stop before C.3's install or before any sign-in completed, `7.3: no session
(recorded)`, which needs the CLI's "log in first" (when one is installed) and no item. Otherwise
B.6's fallback: Craig types the `security delete-generic-password` line, and the runner runs `gx.sh
p6-recheck`, which writes `logout.state` for this run when the item is gone and the CLI has no
session. *Brief author's call:* the cache folder stays until a logout is recorded, since it holds
the session's local state.

**7.4 and 7.5 The scratch v0.10.0 build, and M7:** `gx.sh s7-4-local`.

```bash
# file: mac:scripts/s7-4-local.sh
# Steps 7.4 and 7.5: the scratch v0.10.0 build deleted; M7's default folders still absent.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
B="$GM/v010-build"
if [ -d "$B" ]; then rm -rf -- "$B"; if [ -e "$B" ]; then bad "$B still exists"; else echo "v0.10.0 build deleted: $B"; fi; else echo "7.4: no v0.10.0 build to delete"; fi
m7 || bad "an M7 default folder exists"
echo "HANDOFF -> PC: W.7.6"
exit "$fail"
```

**Pass:** `v0.10.0 build deleted` (or the line that there is none) and the three `M7 default
absent` lines.

**7.6 The Windows check:** W.7.6, on the PC (`gate-7`).

**7.7 The gate cache folder, and the gate's markers:** `gx.sh s7-7-cache`.

```bash
# file: mac:scripts/s7-7-cache.sh
# Step 7.7: after a recorded logout, the gate cache folder deleted, and the markers this gate's
# failing agents wrote removed by name.
. "$HOME/gpbmac/kit/lib.sh" "$1" || exit 2
if [ ! -s "$G/logout.state" ]; then echo "7.7 KEPT: no logout is recorded ($G/logout.state), so the cache folder stays (STOP for Craig; the PC's tasks stay paused)"; exit 1; fi
echo "logout: $(cat "$G/logout.state")"
rm -rf -- "$PROTON_DRIVE_CACHE_DIR"
if [ -e "$PROTON_DRIVE_CACHE_DIR" ]; then bad "$PROTON_DRIVE_CACHE_DIR still exists"; else echo "gate cache folder deleted"; fi
for r in proton-v2 proton-fail; do
  f="$G/launchd/marker-$r.txt"; [ -f "$f" ] || continue
  m=$(cat "$f"); if [ -e "$m" ]; then rm -f -- "$m" && echo "marker removed: $m"; else echo "no marker: $m"; fi
done
echo "HANDOFF -> PC: W.7.8"
exit "$fail"
```

**Pass:** `gate cache folder deleted`, and the `proton-fail` marker removed (its content is already
in `launchd/marker-after-*.txt` and the logs). `7.7 KEPT`: STOP for Craig.

**7.8 The unpause:** W.7.8's `Test-UnpauseCondition`, on the PC. Re-enabling is Craig's.

**What stays.** The installed helper and CLI in `~/.local/bin` (C.3), and `~/gpbmac` (Question 11),
which then holds, by kind: the kit and scripts; the feed-verified CLI copy in `cli/`; and in each
run folder, the logs and command outputs, snapshots, ref sets, relays, ownership record, the gate's
local repositories (`r1-clone` among them, with 5.11's stale ref still in it, and `r1-rt`) and the
tag's clone, the README lines C.3 ran (`readme/`), the launchd step's plists, logs, marker copies
and samples (`samples.txt`, and `samples.txt.<k>` after a re-fire), and small state files: `env.sh`,
`gitconfig-global`, `caffeinate.pid` and `caffeinate.started`, `logout.state`, `installed.txt`, the
`*-commits.sh` commit records, `v010-needs-uncertified`, `d5-unknown.txt`, in `launchd/` the
stamps, `repo.txt`, the `marker-*.txt` path records and `stop-sampler`, and in `envprobe/` the
probe agent's plist and logs (`GPB_`, `PROTON_DRIVE_` and `GIT_` names only, `HOME` and the cache
folder's path; the probe run's folder holds one too). None of the small files
holds a secret (paths, process and commit ids, times). Those hold account metadata: the names, uids and times of `/my-files`'
top-level rows (Craig's own among them), of `/my-files/_cas-probe`, of `/trash` and of the gate's
roots, and the command lines of the processes in the gate wrapper's trees (the sampler keeps
nothing else). They hold no session token, password or Keychain secret: the CLI's session state
is in the gate cache folder, which 7.7 deletes, and the Keychain item goes at 7.3. A listing of
`/my-files` or `/trash` goes into the public record only as its gate rows and counts. Deleting
`~/gpbmac` after the record is written is Craig's call. *Brief author's call.*

---

## Part W: the PC section (the paired PC session runs from here)

Section 6: "The Windows side (the sign-in probe's Windows steps, the task pause, every Windows
check, step 4's and step 5's Windows pushes and clones, step 5.5's NFD move and step 5.8's two
rename pushes, the cross-machine ref-set comparison) is run by a paired agent session on the PC
from its own section of the gate brief, and both machines' results go into one gate record. The
probe's Windows steps are in its scope too, since the probe holds the PC's scheduled task paused."

**What the PC runner needs to know of the rest.** The Mac session runs everything else and keeps
the gate's only listings and snapshots. The PC runs the Windows CLI only to read (`filesystem info`
and `filesystem list`), as it always has (spec, "Facts"); its writes to the account are git
pushes through the draft Windows helper, in W.4.3, W.4.7, W.5.1, W.5.5 and W.5.8. Relays and
handoffs are described in "Two sessions, relays, the kits, timeouts and BLOCKED"; BLOCKED, rule 9's
timeouts (30 minutes for a block that pushes, clones or fetches; 10 for one that reads the
account) and the escaped view apply here unchanged. Every PC block starts by dot-sourcing the kit.

### The order of the two sessions, with every handoff

| # | Mac | PC | Relayed |
|---|---|---|---|
| 1 | Part A; B.0 prep | W.B.0 prep | each side's `RELAY value brief <blob>`, compared by eye |
| 2 | | W.B.1 pause, W.B.2 baseline | PC to Mac: `RELAY value pcroot <uid>` |
| 3 | B.3, B.4 | | `HANDOFF -> PC: W.B.5` |
| 4 | | W.B.5 | `HANDOFF -> MAC: B.6` |
| 5 | B.6 | | `HANDOFF -> PC: W.B.6` |
| 6 | | W.B.6 | `HANDOFF -> MAC: B.7` |
| 7 | B.7 | | `HANDOFF -> PC: W.B.7` |
| 8 | | W.B.7 unpause decision | (the probe ends; the tag is made) |
| 9 | C.0 | W.C.0 | each side's `RELAY value brief <blob>` |
| 10 | | W.C.1 pause, baseline | PC to Mac: `RELAY value pcroot <uid>` |
| 11 | C.2 | W.C.2 | `HANDOFF -> MAC: C.3` |
| 12 | C.3, C.4, C.5 | | `HANDOFF -> PC: W.0` |
| 13 | | W.0 | `HANDOFF -> MAC: step 0` |
| 14 | steps 0, 1, 2.0 to 2.2 | | `HANDOFF -> PC: W.2` |
| 15 | | W.2 | `HANDOFF -> MAC: step 3` |
| 16 | step 3, 4.1 | | Mac to PC: `RELAY refset xm-4a-mac ...`; `HANDOFF -> PC: W.4.2` |
| 17 | | W.4.2, W.4.3 | PC to Mac: `RELAY refset xw-4c-win ...`; `HANDOFF -> MAC: 4.4` |
| 18 | 4.4, 4.5 | | Mac to PC: `RELAY refset xm-4e-mac ...`; `HANDOFF -> PC: W.4.6` |
| 19 | | W.4.6, W.4.7 | PC to Mac: `RELAY refset xw-4g-win ...`; `HANDOFF -> MAC: 4.8` |
| 20 | 4.8, 5.0, 5.1 (Mac) | | `HANDOFF -> PC: W.5.1` |
| 21 | | W.5.1 | `HANDOFF -> MAC: 5.1 after` |
| 22 | 5.1 after, 5.2, 5.3, 5.4, 5.5a | | `HANDOFF -> PC: W.5.5` |
| 23 | | W.5.5 | PC to Mac: `RELAY value W3 <sha>`; `HANDOFF -> MAC: 5.5b` |
| 24 | 5.5b, 5.6, 5.7 (Mac) | W.5.7 (either order) | |
| 25 | 5.8a | | `HANDOFF -> PC: W.5.8` |
| 26 | | W.5.8 | `HANDOFF -> MAC: 5.8b` |
| 27 | 5.8b to 5.11, step 6 | (idle; nothing of the PC's runs the CLI meanwhile) | |
| 28 | 7.0 | W.7.0 | `HANDOFF -> PC: W.7.0`; both `... WRITERS: NONE`; `HANDOFF -> MAC: 7.1` |
| 29 | 7.1 to 7.5 | | `HANDOFF -> PC: W.7.6` |
| 30 | | W.7.6 | `HANDOFF -> MAC: 7.7` |
| 31 | 7.7 | | `HANDOFF -> PC: W.7.8` |
| 32 | | W.7.8 unpause decision | (the gate ends) |

A step 7 after a BLOCK follows the same rows from 28 on. The PC session stays open from row 1 to
row 8, and from row 9 to row 32: section 6 needs it live through 5.5 and 5.8. If either session
ends mid-gate anyway, a new one resumes from the files (dot-source the kit; `run.txt` names
`gate-a1`; the relays and the ledger are in the run folder): its next row is the table's next, and
nothing is re-run. A Mac session's background commands end with it: a push killed that way is a
BLOCK (rule 9).

### The PC's extractor and kit

The PC runner saves this block to `C:\gpbmac\extract.ps1` by hand, the one block saved that way:

```powershell
# file: pc:extract.ps1
# The PC's extractor for the vNEXT macOS gate brief. Usage:
#   pwsh -NoProfile -File C:\gpbmac\extract.ps1 <rev> docs/research/gates/vNEXT-macos-gate-brief.md
# It reads the brief at <rev> from the main checkout, writes every fenced block whose first line is
# "# file: pc:<path>" or "# file: both:<path>" to C:\gpbmac\<path>, byte for byte with LF lines,
# and then checks that the copy of itself it was started from equals its own block.
param([Parameter(Mandatory)][string]$Rev, [Parameter(Mandatory)][string]$Path)
$ErrorActionPreference = 'Stop'
$MainRepo = 'C:\Users\craig\Projects\_Tools\git-proton-backup'
$Root = 'C:\gpbmac'
$own = [IO.File]::ReadAllBytes($PSCommandPath)
$commit = git -C $MainRepo rev-parse --verify "$Rev^{commit}"; if ($LASTEXITCODE) { throw "BLOCKED: no commit for $Rev" }
$blob = git -C $MainRepo rev-parse "${Rev}:$Path"; if ($LASTEXITCODE) { throw "BLOCKED: no $Path at $Rev" }
$psi = [Diagnostics.ProcessStartInfo]::new('git')
foreach ($a in '-C', $MainRepo, 'cat-file', 'blob', $blob) { $psi.ArgumentList.Add($a) }
$psi.RedirectStandardOutput = $true; $psi.UseShellExecute = $false
$p = [Diagnostics.Process]::Start($psi); $ms = [IO.MemoryStream]::new()
$p.StandardOutput.BaseStream.CopyTo($ms); $p.WaitForExit()
if ($p.ExitCode) { throw 'BLOCKED: git cat-file failed' }
$data = $ms.ToArray()
"brief $Path at $Rev (commit $commit): blob $blob, $($data.Length) bytes"
$lines = [Text.UTF8Encoding]::new($false, $true).GetString($data).Split("`n")
$files = [Collections.Generic.Dictionary[string, byte[]]]::new([StringComparer]::Ordinal)
$fences = @(('`' * 3), ('~' * 3))
$i = 0
while ($i -lt $lines.Count) {
    $fence = if ($lines[$i].Length -ge 3) { $lines[$i].Substring(0, 3) } else { '' }
    if ($fences -cnotcontains $fence) { $i++; continue }
    $j = $i + 1
    while ($j -lt $lines.Count -and -not $lines[$j].StartsWith($fence, [StringComparison]::Ordinal)) { $j++ }
    $body = @(if ($j -gt $i + 1) { $lines[($i + 1)..($j - 1)] })
    if ($body.Count -and $body[0] -cmatch '^# file: (pc|both):([A-Za-z0-9._/-]+)$') {
        $rel = $Matches[2]
        if ($files.ContainsKey($rel) -or $rel.StartsWith('/') -or ($rel.Split('/') -ccontains '..')) { throw "BLOCKED: bad or repeated file path in the brief: $rel" }
        $files[$rel] = [Text.UTF8Encoding]::new($false).GetBytes((($body -join "`n") + "`n"))
    }
    $i = $j + 1
}
foreach ($rel in @($files.Keys | Sort-Object)) {
    $dest = Join-Path $Root ($rel -replace '/', '\')
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $dest) | Out-Null
    [IO.File]::WriteAllBytes("$dest.partial", $files[$rel])
    Move-Item -LiteralPath "$dest.partial" -Destination $dest -Force
    $h = [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData($files[$rel])).ToLowerInvariant()
    '  wrote {0,-26} {1,6} bytes  sha256 {2}' -f $rel, $files[$rel].Length, $h
}
"files written: $($files.Count)"
$hash = { param($b) [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([byte[]]$b)) }
$same = $files.ContainsKey('extract.ps1') -and ((& $hash $files['extract.ps1']) -ceq (& $hash $own))
"SELF-CHECK: the extractor that ran equals its block in the brief: $(if ($same) { 'YES' } else { 'NO (it has now been overwritten with the brief''s; run it again)' })"
"RELAY value brief $blob"
if (-not $same) { exit 1 }
```

**Pass:** `SELF-CHECK: ... YES` and `wrote` lines for `extract.ps1`, `kit/expected.txt` and
`kit/pc-kit.ps1`. `NO` means the hand-saved copy differed (line endings, say); it has been replaced
by the brief's, so the runner runs it again, and the second run must print `YES`.

```powershell
# file: pc:kit/pc-kit.ps1
# The PC session's helpers for the vNEXT macOS gate. Dot-source it at the start of every block:
#   . C:\gpbmac\kit\pc-kit.ps1
# Its only account calls are `filesystem info` and `filesystem list`, both read-only, with the
# Windows CLI. Every name and ref comparison is case-sensitive and ordinal. Several functions are
# the v0.10.0 Linux brief's kit, unchanged but for this gate's paths and run names.
$psv = $PSVersionTable.PSVersion
if ([version]"$($psv.Major).$($psv.Minor)" -lt [version]'7.5') { throw "pc-kit: needs PowerShell 7.5 or newer; this is $psv" }
[Console]::OutputEncoding = [Text.UTF8Encoding]::new()
$OutputEncoding = [Text.UTF8Encoding]::new()
$Root = 'C:\gpbmac'
$MainRepo = 'C:\Users\craig\Projects\_Tools\git-proton-backup'
$Tag = 'vNEXT'
$NFC = "caf$([char]0x00E9)"; $NFD = "cafe$([char]0x0301)"
$Run = $null; $G = $null; $Suf = $null
if (Test-Path -LiteralPath (Join-Path $Root 'run.txt')) {
    $Run = (Get-Content -LiteralPath (Join-Path $Root 'run.txt') -Raw).Trim()
    if ($Run -cnotmatch '^(probe-a[1-9]|gate-a1)$') { throw "pc-kit: run.txt names '$Run', which is not a run name" }
    $G = Join-Path $Root $Run
    $Suf = $Run -creplace '^(probe|gate)-', ''
}
$GR = '/my-files/GitRemotes'
$XM = "$GR/$Tag-mac-xm-$Suf"; $XW = "$GR/$Tag-mac-xw-$Suf"; $NFDR = "$GR/$Tag-mac-nfd-$Suf"; $RENR = "$GR/$Tag-mac-ren-$Suf"

function Assert-Run { if (-not $G -or -not (Test-Path -LiteralPath $G)) { throw 'BLOCKED: no current run (C:\gpbmac\run.txt names none, or its folder is missing)' } }

# Starts a run: C:\gpbmac\<kind>-a<attempt> and its folders, named in run.txt. The gate has one run.
function Start-GateRun([string]$Kind, [int]$Attempt) {
    if (-not (($Kind -ceq 'probe' -and $Attempt -ge 1 -and $Attempt -le 9) -or ($Kind -ceq 'gate' -and $Attempt -eq 1))) { throw 'Start-GateRun: probe 1-9, or gate 1 (the retry rule)' }
    $name = "$Kind-a$Attempt"; $dir = Join-Path $Root $name
    if (Test-Path -LiteralPath $dir) { throw "BLOCKED: $dir already exists (that run already started)" }
    foreach ($d in $dir, "$dir\log", "$dir\refsets", "$dir\relay", "$dir\stage", "$dir\w") { New-Item -ItemType Directory -Path $d -ErrorAction Stop | Out-Null }
    Set-Content -LiteralPath (Join-Path $Root 'run.txt') -Value $name -NoNewline
    "RUN STARTED: $name ($dir) at $(Get-Date -Format o). Dot-source the kit again before the next command."
}

# After a native command: prints "<What>: exit <code>" and throws on a nonzero code. The code is
# $LASTEXITCODE, or -Code when a block saved it before another command ran.
function Assert-Exit([string]$What, [int]$Code = $LASTEXITCODE) { "${What}: exit $Code ($(Get-Date -Format o))"; if ($Code -ne 0) { throw "BLOCKED: $What exited $Code" } }

# The escaped view, the same as the Mac kit's: outside printable ASCII and tab, <U+XXXX>; a backslash, <BS>.
function ConvertTo-Esc([string]$s) {
    $b = [Text.StringBuilder]::new()
    foreach ($r in $s.EnumerateRunes()) {
        $v = $r.Value
        if ($v -eq 92) { [void]$b.Append('<BS>') }
        elseif ($v -eq 9 -or ($v -ge 32 -and $v -le 126)) { [void]$b.Append([char]$v) }
        else { [void]$b.Append(('<U+{0:X4}>' -f $v)) }
    }
    $b.ToString()
}

# A command's output lines into <run>\log\<Name>.txt (UTF-8), and their escaped view printed.
function Save-Log([string]$Name, [object[]]$Lines) {
    Assert-Run
    [IO.File]::WriteAllLines((Join-Path $G "log\$Name.txt"), [string[]]@($Lines | ForEach-Object { "$_" }), [Text.UTF8Encoding]::new($false))
    @($Lines | ForEach-Object { ConvertTo-Esc "$_" })
}

# expected.txt's entry [Label] (labels are case-sensitive: case-Main and case-MAIN differ).
function Get-Expected([string]$Label) {
    $entries = [Collections.Generic.Dictionary[string, object]]::new([StringComparer]::Ordinal); $cur = $null
    foreach ($raw in [IO.File]::ReadAllLines((Join-Path $Root 'kit\expected.txt'), [Text.UTF8Encoding]::new($false))) {
        if ($raw.StartsWith('#')) { continue }
        if ($raw -cmatch '^\[([A-Za-z0-9.+-]+)\]$') { $cur = [Collections.Generic.List[string]]::new(); $entries[$Matches[1]] = $cur; continue }
        if ($raw -ceq '') { $cur = $null; continue }
        if ($null -eq $cur) { throw "expected.txt holds a line outside an entry: $raw" }
        $cur.Add($raw)
    }
    if (-not $entries.ContainsKey($Label)) { throw "expected.txt has no entry [$Label]" }
    , [string[]]$entries[$Label].ToArray()
}
function ConvertTo-Pattern([string]$Line) {
    $out = [Text.StringBuilder]::new(); $pos = 0
    foreach ($m in [regex]::Matches($Line, '<OID>|<ENV:([A-Za-z0-9_]+)>')) {
        [void]$out.Append([regex]::Escape($Line.Substring($pos, $m.Index - $pos)))
        if ($m.Value -ceq '<OID>') { [void]$out.Append('[0-9a-f]{40}') }
        else {
            $v = [Environment]::GetEnvironmentVariable($m.Groups[1].Value)
            if (-not $v) { throw "Test-Expect: `$env:$($m.Groups[1].Value) is not set" }
            [void]$out.Append([regex]::Escape($v))
        }
        $pos = $m.Index + $m.Length
    }
    [void]$out.Append([regex]::Escape($Line.Substring($pos)))
    [regex]::new($out.ToString())
}
function Find-Expected([string]$Log, [string]$Label) {
    $e = Get-Expected $Label
    $pats = @($e | ForEach-Object { ConvertTo-Pattern $_ })
    $lines = @([IO.File]::ReadAllLines((Join-Path $G "log\$Log.txt"), [Text.UTF8Encoding]::new($false)) | ForEach-Object { ConvertTo-Esc $_ })
    for ($i = 0; $i -le $lines.Count - $pats.Count; $i++) {
        $ok = $true
        for ($j = 0; $j -lt $pats.Count; $j++) { if (-not $pats[$j].IsMatch($lines[$i + $j])) { $ok = $false; break } }
        if ($ok) { return [pscustomobject]@{ Entry = $e; At = $i + 1 } }
    }
    [pscustomobject]@{ Entry = $e; At = 0 }
}
# Throws when the entry is missing, so the block stops there.
function Test-Expect([string]$Log, [string]$Label) {
    $r = Find-Expected $Log $Label
    if ($r.At) { "EXPECT ${Label}: FOUND at line $($r.At) of $Log"; return }
    "EXPECT ${Label}: MISSING from $Log; the entry, in the escaped view:"
    $r.Entry | ForEach-Object { "  | $_" }
    throw "BLOCKED: EXPECT $Label is MISSING"
}
function Test-Absent([string]$Log, [string]$Label) {
    $r = Find-Expected $Log $Label
    if ($r.At) { throw "BLOCKED: ABSENT ${Label}: NO, FOUND at line $($r.At) of $Log" }
    "ABSENT ${Label}: YES (in $Log)"
}

# One `filesystem list --json` as rows (the Linux kit's Get-Rows); a row without uid, name or
# modificationTime throws.
function Get-Rows([string]$Path) {
    Assert-Run
    $raw = & proton-drive filesystem list $Path --json 2>> (Join-Path $G 'log\cli-stderr.txt')
    $code = $LASTEXITCODE
    if ($code -ne 0) { throw "filesystem list $(ConvertTo-Esc $Path) exited $code`n$($raw | Out-String)" }
    foreach ($i in @(($raw | Out-String) | ConvertFrom-Json -DateKind String)) {
        if ($null -eq $i) { continue }
        if (-not $i.uid -or -not $i.name.value -or -not $i.modificationTime) { throw "filesystem list: a row lacks uid, name or modificationTime" }
        [pscustomobject]@{ path = "$Path/$($i.name.value)"; name = [string]$i.name.value; type = [string]$i.type; uid = [string]$i.uid; trashTime = [string]$i.trashTime }
    }
}
# The live names in a remote folder, sorted ordinally, in the escaped view: the PC session's own view.
function Get-Names([string]$Path) {
    [string[]]$n = @(Get-Rows $Path | Where-Object { -not $_.trashTime } | ForEach-Object { $_.name })
    [Array]::Sort($n, [StringComparer]::Ordinal)
    ($n | ForEach-Object { ConvertTo-Esc $_ }) -join ' '
}

# Section 6's read-only Windows check: `filesystem info /my-files` with the Windows CLI (the Linux
# kit's Invoke-WindowsCheck). PASS means exit 0, a folder, and the uid of this run's baseline.
function Invoke-WindowsCheck([string]$Label, [switch]$Baseline) {
    Assert-Run
    $file = Join-Path $G 'log\wincheck-baseline-uid.txt'
    if ($Baseline -and (Test-Path -LiteralPath $file)) { throw "Invoke-WindowsCheck: run $Run already has a baseline" }
    $ef = Join-Path $G "log\wincheck-$Label.stderr.txt"
    $raw = & proton-drive filesystem info /my-files --json 2> $ef
    $code = $LASTEXITCODE; $text = $raw | Out-String
    $text | Set-Content -LiteralPath (Join-Path $G "log\wincheck-$Label.txt")
    $uid = ''
    try { $j = $text | ConvertFrom-Json -DateKind String; if ([string]$j.type -ceq 'folder') { $uid = [string]$j.uid } } catch { }
    if ($Baseline -and $code -eq 0 -and $uid) { Set-Content -LiteralPath $file -Value $uid }
    $want = if (Test-Path -LiteralPath $file) { (Get-Content -LiteralPath $file -Raw).Trim() } else { '' }
    $pass = ($code -eq 0) -and [bool]$uid -and ($uid -ceq $want)
    $verdict = if ($pass) { 'PASS' } elseif (-not $want -and $code -eq 0 -and $uid) { 'NO-BASELINE' } else { 'FAIL' }
    "$Label $verdict $(Get-Date -Format o)" | Add-Content -LiteralPath (Join-Path $G 'log\wincheck-ledger.txt')
    "WINDOWS CHECK ${Label}: exit=$code uid=$uid baseline=$want -> $verdict"
    if ($Baseline -and $pass) { "RELAY value pcroot $uid" }
    if (-not $pass) { $text; Get-Content -LiteralPath $ef -Raw }
}
# The probe-to-gate boundary: each probe run's baseline uid must equal this gate run's.
function Compare-ProbeBaseline {
    Assert-Run
    $want = (Get-Content -LiteralPath (Join-Path $G 'log\wincheck-baseline-uid.txt') -Raw).Trim()
    $found = @(Get-ChildItem -LiteralPath $Root -Directory | Where-Object { $_.Name -cmatch '^probe-a[1-9]$' } | ForEach-Object { Join-Path $_.FullName 'log\wincheck-baseline-uid.txt' } | Where-Object { Test-Path -LiteralPath $_ })
    if ($found.Count -eq 0) { 'PROBE BASELINE: none under C:\gpbmac (recorded, not a BLOCK; the boundary is not compared)'; return }
    $same = $true
    foreach ($f in $found) { $u = (Get-Content -LiteralPath $f -Raw).Trim(); "probe baseline ${f}: $u"; if ($u -cne $want) { $same = $false } }
    "PROBE BASELINE == GATE BASELINE ($want): $(if ($same) { 'YES' } else { 'NO' })"
}
# Section 6's condition for unpausing: every Windows check this run made passed, its final one
# ($Final) included. A run with only NO-BASELINE lines stopped before its baseline: Craig decides.
function Test-UnpauseCondition([string]$Final) {
    Assert-Run
    $f = Join-Path $G 'log\wincheck-ledger.txt'
    $lines = @(if (Test-Path -LiteralPath $f) { Get-Content -LiteralPath $f })
    $lines | ForEach-Object { "  $_" }
    $verdicts = @($lines | ForEach-Object { ($_ -split ' ')[1] })
    $finals = @($lines | Where-Object { ($_ -split ' ')[0] -ceq $Final } | ForEach-Object { ($_ -split ' ')[1] })
    if ($lines.Count -and @($verdicts | Where-Object { $_ -cne 'NO-BASELINE' }).Count -eq 0) { 'UNPAUSE CONDITION: NO BASELINE WAS TAKEN (Craig decides)'; return }
    $holds = ($lines.Count -gt 0) -and (@($verdicts | Where-Object { $_ -cne 'PASS' }).Count -eq 0) -and (@($finals | Where-Object { $_ -ceq 'PASS' }).Count -ge 1)
    "UNPAUSE CONDITION: $(if ($holds) { 'HOLDS' } else { 'DOES NOT HOLD (STOP for Craig: recovery comes first)' })"
}
# The Task Scheduler's running instances, as task paths (flag 1 adds hidden tasks).
function Get-RunningTaskPaths { $svc = New-Object -ComObject Schedule.Service; $svc.Connect(); @($svc.GetRunningTasks(1) | ForEach-Object { [string]$_.Path }) }
# Both tasks registered, Disabled and not running; no backup or drift worker; no proton-drive process.
# The running tasks and the process list must be read: an error reading either gives QUIET: UNKNOWN,
# never YES.
function Test-Quiet {
    $ok = $true
    try {
        $running = @(Get-RunningTaskPaths)
        $workers = @(Get-CimInstance Win32_Process -ErrorAction Stop | Where-Object { $_.CommandLine -match 'backup-projects|ProjectsBackup\.psm1|Invoke-ProtonBackup|Test-ProtonCliDrift' })
        $cli = @(Get-Process -ErrorAction Stop | Where-Object { $_.ProcessName -eq 'proton-drive' })
    } catch {
        "QUIET: UNKNOWN (the running tasks or the process list could not be read: $($_.Exception.Message)); never proceed on it"
        return
    }
    foreach ($name in 'ProjectsBackupDaily', 'ProtonCliDriftCheck') {
        $t = Get-ScheduledTask -TaskName $name -ErrorAction SilentlyContinue
        if (-not $t) { "${name}: NOT REGISTERED (STOP for Craig)"; $ok = $false; continue }
        $i = $t | Get-ScheduledTaskInfo
        $now = @($running | Where-Object { $_ -ceq "$($t.TaskPath)$name" }).Count -gt 0
        "${name}: State=$($t.State) RunningNow=$now LastRun=$($i.LastRunTime) LastResult=$($i.LastTaskResult) NextRun=$($i.NextRunTime)"
        if ([string]$t.State -cne 'Disabled' -or $now) { $ok = $false }
    }
    "backup or drift workers: $($workers.Count); proton-drive processes: $($cli.Count)"
    if ($workers.Count -ne 0 -or $cli.Count -ne 0) { $ok = $false }
    "QUIET: $(if ($ok) { 'YES' } else { 'NO (a run in flight: wait, then run Test-Quiet again; NOT REGISTERED: STOP for Craig)' })"
}
function Show-TaskTimes {
    foreach ($name in 'ProjectsBackupDaily', 'ProtonCliDriftCheck') {
        $t = Get-ScheduledTask -TaskName $name -ErrorAction SilentlyContinue
        if (-not $t) { "${name}: not registered"; continue }
        $i = $t | Get-ScheduledTaskInfo
        "${name}: State=$($t.State) LastRun=$($i.LastRunTime) NextRun=$($i.NextRunTime) LastResult=$($i.LastTaskResult)"
    }
    'REMINDER: if a Monday 09:00 ProtonCliDriftCheck fell inside the pause, Craig runs scripts\Test-ProtonCliDrift.ps1 once by hand (docs/releasing.md, "Between releases").'
}
# Step 7.0: no Windows process that could still write to the account. Both process lists must be
# read: an error reading either gives WINDOWS WRITERS: UNKNOWN, never NONE, so W.7.0 hands nothing
# off on it.
function Test-NoWriters {
    try {
        $w = @(Get-Process -ErrorAction Stop | Where-Object { $_.ProcessName -in 'git-remote-proton', 'proton-drive' })
        # A git push, fetch or clone that has not reached the helper yet; its pid alone is printed.
        $g = @(Get-CimInstance Win32_Process -Filter "Name = 'git.exe'" -ErrorAction Stop | Where-Object { $_.CommandLine -match ' (push|fetch|clone)( |$)' })
    } catch {
        "WINDOWS WRITERS: UNKNOWN (a process list could not be read: $($_.Exception.Message))"
        return
    }
    $w | ForEach-Object { "  $($_.ProcessName) $($_.Id) started $($_.StartTime)" }
    $g | ForEach-Object { "  git.exe $($_.ProcessId) (a push, fetch or clone)" }
    "WINDOWS WRITERS: $(if ($w.Count -or $g.Count) { 'RUNNING (wait, then check again)' } else { 'NONE' })"
}
# Section 6 step 0: the helper git resolves on Windows is the draft exe's bytes (the Linux kit's
# Test-WindowsHelper). It SETS this process's PATH to a fresh shell's; -Require throws on NO.
function Test-WindowsHelper([switch]$Require) {
    Assert-Run
    $env:Path = @(([Environment]::GetEnvironmentVariable('Path', 'Machine') + ';' + [Environment]::GetEnvironmentVariable('Path', 'User')) -split ';' | Where-Object { $_ }) -join ';'
    $want = (Get-FileHash -LiteralPath (Join-Path $G 'stage\git-remote-proton.exe') -Algorithm SHA256).Hash.ToLower()
    $exec = [string](git --exec-path)
    $inExec = @(if ($exec) { Get-ChildItem -LiteralPath $exec -Filter 'git-remote-proton*' -File -ErrorAction SilentlyContinue })
    "git --exec-path: $exec ($($inExec.Count) git-remote-proton* there)"
    $all = @(Get-Command git-remote-proton -All -ErrorAction SilentlyContinue)
    "git-remote-proton on a fresh PATH: $($all.Count) hit(s); the draft exe's SHA-256 is $want"
    $ok = $false
    foreach ($c in $all) {
        $h = (Get-FileHash -LiteralPath $c.Source -Algorithm SHA256).Hash.ToLower()
        "  $($c.Source)  sha256 $h  $(& $c.Source --version)"
        if ($all.Count -eq 1 -and $inExec.Count -eq 0 -and $exec -and $h -ceq $want) { $ok = $true }
    }
    "WINDOWS HELPER IS THE DRAFT: $(if ($ok) { 'YES' } else { 'NO' })"
    if ($Require -and -not $ok) { throw 'BLOCKED: the draft exe is not the one and only git-remote-proton git can find' }
}
# No GPB_* or PROTON_DRIVE_* variable in any scope, and no object-placement GIT_ variable.
function Test-GpbEnv {
    $n = 0
    foreach ($s in 'Process', 'User', 'Machine') {
        $vars = [Environment]::GetEnvironmentVariables($s)
        foreach ($k in @($vars.Keys | Sort-Object)) { if ($k -match '^(GPB_|PROTON_DRIVE_)' -or $k -in 'GIT_DIR', 'GIT_COMMON_DIR', 'GIT_OBJECT_DIRECTORY') { "SET: $k ($s)"; $n++ } }
    }
    "ENV check done: $n SET line(s)."
}
# The release is still a draft (section 6: it stays unpublished until this gate ends).
function Test-Draft {
    $j = @(gh release view $Tag --repo craigstoller/git-proton-backup --json isDraft,tagName --jq '.isDraft, .tagName' 2>&1 | ForEach-Object { "$_" }); $c = $LASTEXITCODE
    "release ${Tag}: $($j -join ' ') (gh exit $c)"
    if ($c -ne 0) { throw "BLOCKED: gh release view failed (exit $c); is gh signed in (gh auth status)?" }
    if ($j.Count -lt 1 -or $j[0] -cne 'true') { throw "BLOCKED: release $Tag is not a draft (section 6 holds it unpublished until this gate ends)" }
}

# Ref sets. A file in <run>\refsets holds `git ls-remote` output or a clone's for-each-ref of
# refs/remotes/proton-v2; a clone's refs/remotes/proton-v2/<b> reads as refs/heads/<b>.
function Save-RefSet([string]$Label, [object[]]$Lines) { Assert-Run; [IO.File]::WriteAllLines((Join-Path $G "refsets\$Label.txt"), [string[]]@($Lines | ForEach-Object { "$_" }), [Text.UTF8Encoding]::new($false)) }
function Read-RefSet([string]$Label) {
    Assert-Run
    $f = Join-Path $G "refsets\$Label.txt"
    if (-not (Test-Path -LiteralPath $f)) { throw "no ref set named $Label" }
    $clone = $false; $set = [Collections.Generic.SortedSet[string]]::new([StringComparer]::Ordinal)
    foreach ($l in [IO.File]::ReadAllLines($f, [Text.UTF8Encoding]::new($false))) {
        if (-not $l.Trim()) { continue }
        $p = -split $l
        if ($p.Count -ne 2 -or $p[0] -cnotmatch '^[0-9a-f]{40}$') { throw "ref set $Label holds a line that is not '<sha> <ref>': [$(ConvertTo-Esc $l)]" }
        $ref = $p[1]
        if ($ref.StartsWith('refs/remotes/proton-v2/', [StringComparison]::Ordinal)) { $clone = $true; $rest = $ref.Substring(23); $ref = if ($rest -ceq 'HEAD') { 'HEAD' } else { "refs/heads/$rest" } }
        [void]$set.Add("$($p[0]) $ref")
    }
    [pscustomobject]@{ Clone = $clone; Rows = [string[]]@($set) }
}
function Get-RefDigest([string[]]$Rows) { [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.UTF8Encoding]::new($false).GetBytes((($Rows | ForEach-Object { "$_`n" }) -join '')))).ToLowerInvariant() }
function Get-NoHead([string[]]$Rows) { [string[]]@($Rows | Where-Object { -not $_.EndsWith(' HEAD', [StringComparison]::Ordinal) }) }
function Show-RefSet([string]$Label) { $x = Read-RefSet $Label; "REFSET $Label ($(if ($x.Clone) { 'clone' } else { 'remote' })): $($x.Rows.Count) ref(s)"; $x.Rows | ForEach-Object { "  $(ConvertTo-Esc $_)" } }
function Get-Relay([string]$Label) {
    $x = Read-RefSet $Label; $nh = Get-NoHead $x.Rows
    "RELAY refset $Label $(if ($x.Clone) { 'clone' } else { 'remote' }) $($x.Rows.Count) $(Get-RefDigest $x.Rows) $($nh.Count) $(Get-RefDigest $nh)"
}
function Compare-RefSet([string]$A, [string]$B) {
    $x = Read-RefSet $A; $y = Read-RefSet $B; $drop = $x.Clone -or $y.Clone
    $sa = if ($drop) { Get-NoHead $x.Rows } else { $x.Rows }; $sb = if ($drop) { Get-NoHead $y.Rows } else { $y.Rows }
    $onlyA = @($sa | Where-Object { $sb -cnotcontains $_ }); $onlyB = @($sb | Where-Object { $sa -cnotcontains $_ })
    $note = if ($drop) { ', HEAD left out' } else { '' }
    if ($onlyA.Count -eq 0 -and $onlyB.Count -eq 0) { "REFSET $A == ${B}: EQUAL ($($sa.Count) ref(s)$note)" }
    else { "REFSET $A vs ${B}: DIFFER$note"; $onlyA | ForEach-Object { "  ONLY IN ${A}: $_" }; $onlyB | ForEach-Object { "  ONLY IN ${B}: $_" }; throw 'BLOCKED: the ref sets differ' }
}
# Relays: Save-Relay stores a RELAY line Craig carried from the Mac; Compare-Relay compares a local
# ref set with a stored relayed digest; Get-RelayValue reads a stored value.
function Save-Relay([string]$Line) {
    Assert-Run
    $Line = $Line.Trim()
    if ($Line -cmatch '^RELAY refset ([A-Za-z0-9.-]+) (clone|remote) [0-9]+ [0-9a-f]{64} [0-9]+ [0-9a-f]{64}$') { $n = "refset-$($Matches[1])" }
    elseif ($Line -cmatch '^RELAY value ([A-Za-z0-9_]+) [A-Za-z0-9~_.-]+$') { $n = "value-$($Matches[1])" }
    else { throw "not a relay line: [$(ConvertTo-Esc $Line)]" }
    $f = Join-Path $G "relay\$n.txt"
    if ((Test-Path -LiteralPath $f) -and (Get-Content -LiteralPath $f -Raw).Trim() -cne $Line) { throw "BLOCKED: a different relay named $n was received before" }
    Set-Content -LiteralPath $f -Value $Line
    "RECEIVED $Line"
}
function Compare-Relay([string]$Mine, [string]$Theirs) {
    Assert-Run
    $f = Join-Path $G "relay\refset-$Theirs.txt"
    if (-not (Test-Path -LiteralPath $f)) { throw "no relayed ref set $Theirs (Save-Relay the Mac's line first)" }
    $p = -split (Get-Content -LiteralPath $f -Raw).Trim()
    $x = Read-RefSet $Mine; $drop = $x.Clone -or $p[3] -ceq 'clone'
    $rows = if ($drop) { Get-NoHead $x.Rows } else { $x.Rows }
    $want = if ($drop) { "$($p[6]) $($p[7])" } else { "$($p[4]) $($p[5])" }
    $have = "$($rows.Count) $(Get-RefDigest $rows)"
    "RELAYED REFSET $Theirs vs ${Mine}: $(if ($have -ceq $want) { 'EQUAL' } else { 'DIFFER' }) ($(($want -split ' ')[0]) ref(s)$(if ($drop) { ', HEAD left out' }))"
    if ($have -cne $want) { Show-RefSet $Mine; throw 'BLOCKED: the PC and the Mac see different ref sets' }
}
function Get-RelayValue([string]$Name) {
    $f = Join-Path $G "relay\value-$Name.txt"
    if (-not (Test-Path -LiteralPath $f)) { throw "no relayed value $Name" }
    (-split (Get-Content -LiteralPath $f -Raw).Trim())[3]
}
```

*Brief author's call:* the PC kit is self-contained rather than loading the v0.10.0 kit from
`C:\gpb010`, which may no longer exist; the functions it shares with that kit keep their names and
their tested bodies. Passing NFD bytes to `git.exe` from PowerShell: the strings are UTF-16, and
Git for Windows reads its arguments as wide strings and converts them to UTF-8, so the bytes
arrive as written [inference; no v0.10.0 gate passed a non-ASCII name]. W.5.1's refusal text shows
it, since the helper echoes the destination git sent: a precomposed name would be refused
differently, or not at all.

### W.B Part B on the PC

**W.B.0 Prep.** Save the extractor block by hand to `C:\gpbmac\extract.ps1`, then:

```powershell
New-Item -ItemType Directory -Force -Path C:\gpbmac | Out-Null
git -C C:\Users\craig\Projects\_Tools\git-proton-backup fetch origin; "fetch: exit $LASTEXITCODE"
pwsh -NoProfile -File C:\gpbmac\extract.ps1 refs/remotes/origin/main docs/research/gates/vNEXT-macos-gate-brief.md
. C:\gpbmac\kit\pc-kit.ps1
Start-GateRun probe 1   # a re-run of the probe, on Craig's word only, takes 2
. C:\gpbmac\kit\pc-kit.ps1
Test-GpbEnv
& proton-drive --version; git --version; Get-Command proton-drive, git, gh | Format-Table Name, Source
gh auth status; "gh auth: exit $LASTEXITCODE"
```

**Pass:** the extractor passes; its `RELAY value brief <blob>` equals the Mac's (Craig compares the
two); `RUN STARTED: probe-a1 ...`; `ENV check done: 0 SET line(s).`; the CLI prints
`Proton Drive CLI cli-drive@0.9.0+75612299`, git `git version 2.55.0.windows.3` [record, measured
2026-10-16; another version is recorded]; the three commands resolve; `gh auth: exit 0` (a draft
release is visible only to a signed-in collaborator [inference], so W.C.2's `Test-Draft` needs it).

**W.B.1 Pause both tasks and wait for any run in flight** (section 6 probe step 1):

```powershell
. C:\gpbmac\kit\pc-kit.ps1
Disable-ScheduledTask -TaskName ProjectsBackupDaily | Format-Table TaskName, State
Disable-ScheduledTask -TaskName ProtonCliDriftCheck | Format-Table TaskName, State
Test-Quiet
```

**Pass:** `QUIET: YES`. If a run is in flight: wait, then `Test-Quiet` again; never proceed while
`QUIET: NO` or `QUIET: UNKNOWN` (the running tasks or the process list could not be read: run it
again, and one that repeats is a STOP for Craig). `NOT REGISTERED`: STOP for Craig. If the runner
may not disable tasks, Craig runs the two `Disable` lines himself.

**W.B.2 The read-only Windows check, as a baseline** (probe step 2):

```powershell
. C:\gpbmac\kit\pc-kit.ps1
Invoke-WindowsCheck -Label probe-2 -Baseline
```

**Pass:** `WINDOWS CHECK probe-2: exit=0 uid=<uid> baseline=<the same uid> -> PASS` and the line
`RELAY value pcroot <uid>`, which Craig carries to the Mac. A FAIL: the PC session was broken
before the Mac did anything; STOP for Craig, tasks left paused.

**W.B.5 The Windows check again** (probe step 5), after `HANDOFF -> PC: W.B.5`:

```powershell
. C:\gpbmac\kit\pc-kit.ps1
Invoke-WindowsCheck -Label probe-5
```

**Pass:** `-> PASS`. Then `HANDOFF -> MAC: B.6`.

**W.B.6 The Windows check once more** (probe step 6), after the Mac's logout: the same block with
`-Label probe-6`. **Pass:** `-> PASS`. Then `HANDOFF -> MAC: B.7`.

**W.B.7 The unpause decision** (probe step 7), after the Mac's `STEP 7 (Mac): PASS`:

```powershell
. C:\gpbmac\kit\pc-kit.ps1
Test-UnpauseCondition probe-6
```

**If it prints `UNPAUSE CONDITION: HOLDS`,** the runner tells Craig, and Craig re-enables the tasks
in his own PowerShell; the runner then confirms read-only:

```powershell
Enable-ScheduledTask -TaskName ProjectsBackupDaily | Format-Table TaskName, State
Enable-ScheduledTask -TaskName ProtonCliDriftCheck | Format-Table TaskName, State
```

```powershell
. C:\gpbmac\kit\pc-kit.ps1
Get-ScheduledTask -TaskName ProjectsBackupDaily, ProtonCliDriftCheck | Format-Table TaskName, State
Show-TaskTimes
```

Pass: both `Ready`, and the drift reminder read. **`DOES NOT HOLD`,** or the Mac's `7 KEPT`: STOP
for Craig with both tasks disabled. Recovery is Craig signing in again on Windows and confirming
with a fleet verify from a fresh session with no `GPB_*` overrides (`proton-drive auth login`, then
`pwsh -NoProfile -Command "Import-Module GitProtonBackup; Invoke-ProtonBackupVerify"`), exit 0 and
`Complete=True`, before he re-enables them, as the v0.10.0 briefs set out. *Brief author's call,
following those briefs:* re-enabling is Craig's (`v0.7.0-release-gate.md:72, 521`;
`cli-0.8.0-certification.md:626`).

### W.C Part C on the PC

**W.C.0 Start the gate's run, from the tag's brief:**

```powershell
git -C C:\Users\craig\Projects\_Tools\git-proton-backup fetch origin "refs/tags/vNEXT:refs/tags/vNEXT"; "fetch tag: exit $LASTEXITCODE"
pwsh -NoProfile -File C:\gpbmac\extract.ps1 vNEXT docs/research/gates/vNEXT-macos-gate-brief.md
. C:\gpbmac\kit\pc-kit.ps1
Start-GateRun gate 1
```

**Pass:** the extractor passes with the same blob as the Mac's C.0, and `RUN STARTED: gate-a1 ...`.

**W.C.1 Pause both tasks for the whole gate, and take its baseline:**

```powershell
. C:\gpbmac\kit\pc-kit.ps1
Disable-ScheduledTask -TaskName ProjectsBackupDaily | Format-Table TaskName, State
Disable-ScheduledTask -TaskName ProtonCliDriftCheck | Format-Table TaskName, State
Test-Quiet
Test-GpbEnv
```

**Pass:** `QUIET: YES` (never `NO` or `UNKNOWN`, as at W.B.1) and `ENV check done: 0 SET line(s).`
Then, once quiet:

```powershell
. C:\gpbmac\kit\pc-kit.ps1
Invoke-WindowsCheck -Label gate-c1 -Baseline
```

**Pass:** `WINDOWS CHECK gate-c1 ... -> PASS` and `RELAY value pcroot <uid>`, carried to the Mac.
Every later check is compared with this baseline. A FAIL: STOP for Craig, tasks paused.

**W.C.2 The draft, its Windows helper's bytes, and the installed helper:**

```powershell
. C:\gpbmac\kit\pc-kit.ps1
Test-Draft
gh release download vNEXT --repo craigstoller/git-proton-backup --pattern 'git-remote-proton.exe' --pattern 'git-remote-proton.exe.sha256' --dir "$G\stage"; Assert-Exit 'gh release download'
$exe = (Get-FileHash -LiteralPath "$G\stage\git-remote-proton.exe" -Algorithm SHA256).Hash.ToLower()
$side = Get-Content -LiteralPath "$G\stage\git-remote-proton.exe.sha256" -Raw
"draft exe SHA-256: $exe"
$sideOk = $side -ceq "$exe  git-remote-proton.exe"
"sidecar == '<sha256>  git-remote-proton.exe': $sideOk"
$wh = @(Test-WindowsHelper); $wh
if ($sideOk -and ($wh -ccontains 'WINDOWS HELPER IS THE DRAFT: YES')) { 'HANDOFF -> MAC: C.3' } else { 'NO HANDOFF: the sidecar or the Windows helper check above failed (STOP for Craig; the Mac waits)' }
```

**Pass:** `release vNEXT: true vNEXT (gh exit 0)`; the download exits 0; the sidecar line is `True` (the sidecar
has no trailing newline [code `.github/workflows/release.yml`, Checksum step]); and `WINDOWS HELPER
IS THE DRAFT: YES`, its one line showing `git-remote-proton vNEXT (certified CLI:
cli-drive@0.9.0+75612299)`. Only then does the block print `HANDOFF -> MAC: C.3`; otherwise it
prints `NO HANDOFF: ...` and the Mac waits. **`NO`:** this release's Windows gate installs the
draft helper, and
this gate runs after it (section 6, O5), so a `NO` is a STOP for Craig; this brief installs
nothing on the PC. The exe's SHA-256 must also equal the STAGED BASELINE of this release's Windows
gate record; the runner compares the two and records both.

### W.0 Step 0 on the PC

```powershell
. C:\gpbmac\kit\pc-kit.ps1
Test-GpbEnv
Test-WindowsHelper -Require
& proton-drive --version
git --version
Invoke-WindowsCheck -Label gate-0
Compare-ProbeBaseline
Test-Draft
'HANDOFF -> MAC: step 0'
```

**Pass:** `ENV check done: 0 SET line(s).`; `WINDOWS HELPER IS THE DRAFT: YES` after `git
--exec-path: ... (0 git-remote-proton* there)` (record its path and hash: section 6 step 0 needs the
draft's bytes, because step 5.1 needs its refusal); the CLI and git versions as at W.B.0; `WINDOWS
CHECK gate-0 ... -> PASS`; `PROBE BASELINE == GATE BASELINE (<uid>): YES` (`none` is recorded,
`NO` is BLOCKED); the draft still a draft.

### W.2 After the contract table

```powershell
. C:\gpbmac\kit\pc-kit.ps1
Invoke-WindowsCheck -Label gate-2
'HANDOFF -> MAC: step 3'
```

**Pass:** `-> PASS`. A FAIL is BLOCKED, and the ledger keeps it, so W.7.8 then needs Craig's
recovery whatever `gate-7` says.

### W.4 Step 4's Windows clones and pushes

Every block that reaches a remote starts with `Test-WindowsHelper -Require`, so its PATH is the one
W.0 checked, and every git command is followed by `Assert-Exit`.

**W.4.2 Clone XM on Windows,** after `Save-Relay '<the Mac's RELAY refset xm-4a-mac ... line>'`:

```powershell
. C:\gpbmac\kit\pc-kit.ps1
Test-WindowsHelper -Require
$x = Join-Path $G 'w' 'xm'
if (Test-Path -LiteralPath $x) { throw "BLOCKED: $x already exists" }
$o = @(git clone -o proton-v2 "proton::$XM" $x 2>&1); $c = $LASTEXITCODE; Save-Log w-4b-clone $o; Assert-Exit clone $c
git -C $x fsck --full 2>&1; Assert-Exit 'fsck --full'
Save-RefSet xm-4b-winclone @(git -C $x for-each-ref --format='%(objectname) %(refname)' refs/remotes/proton-v2); Assert-Exit for-each-ref
Save-RefSet xm-4b-win @(git ls-remote "proton::$XM"); Assert-Exit ls-remote
Compare-RefSet xm-4b-winclone xm-4b-win
Compare-Relay xm-4b-win xm-4a-mac
```

**Pass:** `clone: exit 0`, `fsck --full` silent with `exit 0`, `REFSET xm-4b-winclone == xm-4b-win:
EQUAL (2 ref(s), HEAD left out)` and `RELAYED REFSET xm-4a-mac vs xm-4b-win: EQUAL (3 ref(s))`. No
`git-remote-proton:` line in the clone's output.

**W.4.3 Push XW from Windows:**

```powershell
. C:\gpbmac\kit\pc-kit.ps1
Test-WindowsHelper -Require
$x = Join-Path $G 'w' 'xw'
if (Test-Path -LiteralPath $x) { throw "BLOCKED: $x already exists" }
git init -q -b main $x; Assert-Exit 'git init'
git -C $x config user.name 'gpbmac gate'; Assert-Exit 'config user.name'
git -C $x config user.email 'gate@gpbmac.invalid'; Assert-Exit 'config user.email'
Set-Content -LiteralPath (Join-Path $x 'w.txt') -Value 'pushed from Windows 1'
git -C $x add w.txt; Assert-Exit 'add w.txt'
git -C $x commit -q -m 'gate: xw 1'; Assert-Exit 'commit xw 1'
git -C $x switch -q -c topic-w; Assert-Exit 'switch -c topic-w'
Set-Content -LiteralPath (Join-Path $x 'topic.txt') -Value 'topic from Windows'
git -C $x add topic.txt; Assert-Exit 'add topic.txt'
git -C $x commit -q -m 'gate: xw topic'; Assert-Exit 'commit xw topic'
git -C $x switch -q main; Assert-Exit 'switch main'
git -C $x remote add proton-v2 "proton::$XW"; Assert-Exit 'remote add'
$o = @(git -C $x push -u proton-v2 main topic-w 2>&1); $c = $LASTEXITCODE; Save-Log w-4c-push $o; Assert-Exit push $c
Save-RefSet xw-4c-win @(git ls-remote "proton::$XW"); Assert-Exit ls-remote
Show-RefSet xw-4c-win
Get-Relay xw-4c-win
'HANDOFF -> MAC: 4.4'
```

**Pass:** two `[new branch]` lines, `push: exit 0`, no restore-floor warning (Git for Windows
2.55.0 passes the floor probe), the ref set `HEAD`, `refs/heads/main` and `refs/heads/topic-w`, and
the `RELAY refset xw-4c-win remote 3 ...` line, carried to the Mac.

**W.4.6 The Windows clone of XM fetches,** after `Save-Relay '<the Mac's RELAY refset xm-4e-mac ...>'`:

```powershell
. C:\gpbmac\kit\pc-kit.ps1
Test-WindowsHelper -Require
$x = Join-Path $G 'w' 'xm'
$o = @(git -C $x fetch --prune proton-v2 2>&1); $c = $LASTEXITCODE; Save-Log w-4f-fetch $o; Assert-Exit fetch $c
git -C $x fsck --full 2>&1; Assert-Exit 'fsck --full'
Save-RefSet xm-4f-winclone @(git -C $x for-each-ref --format='%(objectname) %(refname)' refs/remotes/proton-v2); Assert-Exit for-each-ref
Compare-Relay xm-4f-winclone xm-4e-mac
```

**Pass:** a line with `-> proton-v2/main`, a `[deleted]` line naming `proton-v2/topic-m`, both exits
0, and `RELAYED REFSET xm-4e-mac vs xm-4f-winclone: EQUAL (1 ref(s), HEAD left out)`.

**W.4.7 Windows pushes an update and a branch deletion to XW:**

```powershell
. C:\gpbmac\kit\pc-kit.ps1
Test-WindowsHelper -Require
$x = Join-Path $G 'w' 'xw'
Add-Content -LiteralPath (Join-Path $x 'w.txt') -Value 'pushed from Windows 2'
git -C $x commit -q -am 'gate: xw 2'; Assert-Exit commit
"new main: $(git -C $x rev-parse main)"
$o = @(git -C $x push proton-v2 main :refs/heads/topic-w 2>&1); $c = $LASTEXITCODE; Save-Log w-4g-push $o; Assert-Exit push $c
Save-RefSet xw-4g-win @(git ls-remote "proton::$XW"); Assert-Exit ls-remote
Show-RefSet xw-4g-win
Get-Relay xw-4g-win
'HANDOFF -> MAC: 4.8'
```

**Pass:** `..` with `main -> main`, `[deleted]` with `topic-w`, `push: exit 0`, the ref set only
`HEAD` and `refs/heads/main` at `new main`, and the relay line, carried to the Mac.

### W.5 Step 5's Windows pushes

**W.5.1 The NFD create, refused by the draft Windows helper** (section 6 step 5.1), after
`HANDOFF -> PC: W.5.1`. The PC clones `nfd` (it holds `main` and the NFC `cafe`, no pair), makes a
commit the remote lacks, and pushes it to the NFD spelling.

```powershell
. C:\gpbmac\kit\pc-kit.ps1
Test-WindowsHelper -Require
$x = Join-Path $G 'w' 'pnfd'
if (Test-Path -LiteralPath $x) { throw "BLOCKED: $x already exists" }
$o = @(git clone -o proton-v2 "proton::$NFDR" $x 2>&1); $c = $LASTEXITCODE; Save-Log w-5-1-clone $o; Assert-Exit clone $c
git -C $x config user.name 'gpbmac gate'; Assert-Exit 'config user.name'
git -C $x config user.email 'gate@gpbmac.invalid'; Assert-Exit 'config user.email'
Add-Content -LiteralPath (Join-Path $x 'n.txt') -Value 'nfd W2, from Windows, withheld'
git -C $x commit -q -am 'gate: nfd W2, withheld'; Assert-Exit 'commit W2'
"W2 (local main, not on the remote): $(git -C $x rev-parse main)"
"dst as PowerShell encodes it (UTF-8): $(([Text.Encoding]::UTF8.GetBytes("refs/heads/$NFD") | ForEach-Object { $_.ToString('x2') }) -join '')"
$o = @(git -C $x push proton-v2 "main:refs/heads/$NFD" 2>&1); $rc = $LASTEXITCODE; Save-Log w-5-1-push $o
"push main:<the NFD cafe>: exit $rc (expected 1: refused) ($(Get-Date -Format o))"
if ($rc -ne 1) { throw "BLOCKED: the push exited $rc, not 1" }
Test-Expect w-5-1-push nfd-create
"refs/heads (PC view): $(Get-Names "$NFDR/refs/heads")"
"packs (PC view): $(Get-Names "$NFDR/packs")"
'HANDOFF -> MAC: 5.1 after'
```

**Pass:** the clone exits 0 with no `git-remote-proton:` line; `dst as PowerShell encodes it` ends
`63616665cc81` (PowerShell's own bytes, no more); the push exits 1 with `EXPECT nfd-create:
FOUND`, the same text as the Mac's (string 1: both spellings and `by Unicode normalisation`). The
refusal is what shows the bytes git.exe passed on: the helper names the destination git sent it,
and the escaped view reads `cafe<U+0301>` only if those bytes arrived as NFD; the PC's view `refs/heads (PC view):
caf<U+00E9> main` and 5.0's two pack pairs. So the draft Windows helper refuses the composition
pair, on its own binary.

**W.5.5 The NFD member moved to a new commit, by exact bytes** (section 6 step 5.5), after
`HANDOFF -> PC: W.5.5`. The remote now holds the pair (5.4), so the fetch and the push both print
the pair warning on Windows; neither is refused off a Mac.

```powershell
. C:\gpbmac\kit\pc-kit.ps1
Test-WindowsHelper -Require
$x = Join-Path $G 'w' 'pnfd'
$o = @(git -C $x fetch proton-v2 2>&1); $c = $LASTEXITCODE; Save-Log w-5-5-fetch $o; Assert-Exit fetch $c
Test-Expect w-5-5-fetch warn-nfd
git -C $x switch -q -c nfdmove proton-v2/main; Assert-Exit 'switch -c nfdmove'
"start (proton-v2/main, N2): $(git -C $x rev-parse HEAD)"
"remote NFD member before: $(git -C $x rev-parse "refs/remotes/proton-v2/$NFD")"
if ((git -C $x rev-parse "refs/remotes/proton-v2/$NFD") -cne (git -C $x rev-parse HEAD)) { throw 'BLOCKED: the NFD member was not at the start commit' }
Add-Content -LiteralPath (Join-Path $x 'n.txt') -Value 'nfd W3, the NFD member moved from Windows'
git -C $x commit -q -am 'gate: nfd W3'; Assert-Exit 'commit W3'
$w3 = git -C $x rev-parse HEAD; "W3: $w3"
$o = @(git -C $x push proton-v2 "nfdmove:refs/heads/$NFD" 2>&1); $c = $LASTEXITCODE; Save-Log w-5-5-push $o; Assert-Exit 'push the NFD member to W3' $c
Test-Expect w-5-5-push warn-nfd
"refs/heads (PC view): $(Get-Names "$NFDR/refs/heads")"
"RELAY value W3 $w3"
'HANDOFF -> MAC: 5.5b'
```

**Pass:** the fetch exits 0 with the warning; the NFD member was at the start commit (N2, the
upload's content); the push exits 0, a fast-forward (`..` with `nfdmove -> cafe<U+0301>`), with the
warning; the PC view lists `cafe<U+0301> caf<U+00E9> main side`; and `RELAY value W3 <sha>`,
carried to the Mac.

**W.5.7 `git ls-remote` of that remote from Windows** (section 6 step 5.7):

```powershell
. C:\gpbmac\kit\pc-kit.ps1
Test-WindowsHelper -Require
Set-Location -LiteralPath $G
$o = @(git ls-remote "proton::$NFDR" 2>&1); $c = $LASTEXITCODE; Save-Log w-5-7-ls $o; Assert-Exit ls-remote $c
Test-Expect w-5-7-ls warn-nfd
```

**Pass:** exit 0, the five refs listed (`HEAD`, `main`, `side`, both `cafe` spellings), and the
warning: off a Mac it never blocks anything (spec section 3).

**W.5.8 The rename transition's two pushes** (section 6 step 5.8), after `HANDOFF -> PC: W.5.8`:
delete the NFC `cafe`, then, in a second push, create the NFD spelling by exact bytes at the same
commit.

```powershell
. C:\gpbmac\kit\pc-kit.ps1
Test-WindowsHelper -Require
$x = Join-Path $G 'w' 'pren'
if (Test-Path -LiteralPath $x) { throw "BLOCKED: $x already exists" }
$o = @(git clone -o proton-v2 "proton::$RENR" $x 2>&1); $c = $LASTEXITCODE; Save-Log w-5-8-clone $o; Assert-Exit clone $c
$rc1 = git -C $x rev-parse "refs/remotes/proton-v2/$NFC"; Assert-Exit 'rev-parse the NFC cafe'
"RC1 (the NFC cafe's commit): $rc1"
$o = @(git -C $x push proton-v2 ":refs/heads/$NFC" 2>&1); $c = $LASTEXITCODE; Save-Log w-5-8-delete $o; Assert-Exit 'push the delete of the NFC cafe' $c
Save-Log w-5-8-ls1 @(git ls-remote "proton::$RENR" 2>&1)
$o = @(git -C $x push proton-v2 "${rc1}:refs/heads/$NFD" 2>&1); $c = $LASTEXITCODE; Save-Log w-5-8-create $o; Assert-Exit 'push the NFD create at RC1' $c
Save-Log w-5-8-ls2 @(git ls-remote "proton::$RENR" 2>&1)
Test-Absent w-5-8-create warn-head
"refs/heads (PC view): $(Get-Names "$RENR/refs/heads")"
'HANDOFF -> MAC: 5.8b'
```

**Pass:** the clone exits 0; the delete exits 0 with `[deleted]` and `caf<U+00E9>`, after which
`ls-remote` lists `HEAD` and `main` only; the create exits 0 with `* [new branch]` and
`-> cafe<U+0301>`, at RC1, and no pair warning (the remote never held both); the PC view lists
`cafe<U+0301> main`. Neither push is refused: the delete is not of the branch `HEAD` names, and the
create meets no other spelling.

### W.7 Step 7 on the PC

**W.7.0 No Windows writer:**

```powershell
. C:\gpbmac\kit\pc-kit.ps1
$nw = @(Test-NoWriters); $nw
if ($nw -ccontains 'WINDOWS WRITERS: NONE') { 'HANDOFF -> MAC: 7.1' } else { 'NO HANDOFF: a Windows writer still runs, or a process list could not be read (above); wait, then run W.7.0 again' }
```

**Pass:** `WINDOWS WRITERS: NONE`, then `HANDOFF -> MAC: 7.1`; otherwise `NO HANDOFF: ...`: wait,
and run W.7.0 again. `WINDOWS WRITERS: UNKNOWN` (a process list could not be read) is never NONE,
so it hands nothing off: run W.7.0 again, and one that repeats is a STOP for Craig, with nothing
trashed or deleted meanwhile.

**W.7.6 The Windows check** (section 6 step 7), after `HANDOFF -> PC: W.7.6`:

```powershell
. C:\gpbmac\kit\pc-kit.ps1
Invoke-WindowsCheck -Label gate-7
Test-Draft
'HANDOFF -> MAC: 7.7'
```

**Pass:** `-> PASS`, and the draft still a draft (the gate is ending; Craig publishes only after it
ends, `docs/releasing.md` step 6).

**W.7.8 The unpause decision,** after the Mac's 7.7:

```powershell
. C:\gpbmac\kit\pc-kit.ps1
Test-UnpauseCondition gate-7
```

As W.B.7: `HOLDS`, then Craig re-enables both tasks and the runner confirms them `Ready` with
`Show-TaskTimes`; `DOES NOT HOLD`, or the Mac's `7.7 KEPT`: STOP for Craig, both tasks disabled,
recovery first. *Brief author's call, the v0.10.0 briefs' stricter reading:* every Windows check
of the run must have passed, not only the last.

---

## After the gate

**The gate record.** The runners write `docs/research/gates/vNEXT-macos-gate.md` (renamed with the
version), opened by the probe's results. It holds: Part A's build lines; C.2's anchor check; C.3's
build information and the installed binaries' SHA-512 and SHA-256 (copied by script from the run's
`out/` files, never typed); each step's key output, verbatim, in the escaped view where it holds
non-ASCII; both machines' ref-set relays; 5.5's M16 measurement, stated plainly; step 6's firing
times, logs, the banner and permission observations and the configuration that showed the banner;
the ownership record (`roots.tsv`) and every trash line with its time; the trash tally, and what
Craig deleted or kept (C.5); the PC's ledger of Windows checks; and the final state. The repository
is public: a sign-in URL keeps only its scheme, host and path.

**The release.** This gate ran against the tag's source and the draft's Windows exe. Tags never
move. Once this gate has ended, and only if the Windows and Linux gates passed, Craig may publish
the draft under `docs/releasing.md` steps 6 and 7: when this gate passed, or when it failed and
every failure is Mac-only (section 6, O5(b): a Mac-only failure does not hold the release). A
failure is Mac-only when it lies in what no Windows or Linux asset carries: the helper's Mac
rules, which act only on darwin; the wrapper and plist under `scripts/macos`; the Mac CLI build,
launchd or the Keychain; or the gate's own setup. A failure in shared code, which the Windows and
Linux assets carry, is a defect in the release: the draft is not published, and the fix ships as
a new tag under the no-retag rule (section 6; `docs/releasing.md` step 5). The record classes each
failure with its evidence. *Brief author's call:* the runners propose the class, and the class is
Craig's ruling, made before he publishes.

**After a pass:** the README's second-pass status line ("live-gated once, at <version>; later
releases' Mac builds and later CLI versions on the Mac are not live-gated", spec section 1) and the
other documents that say macOS is not yet gated are updated after the gate, as v0.10.0's were after
its Linux gate: each edit proposed to Craig with its before and after text and made only on his
word, then a re-grep for every carrier of the old state. That is not this brief's work (plan, "Not
tasks").

**After a fail:** the next attempt is all of section 6 again at a later release, with its own
brief (the retry rule).

**Afterwards (MD5(a), O3).** Real use starts with a fresh sign-in into the default folders
(without `PROTON_DRIVE_CACHE_DIR`), after the writers-first step and the pre-adoption scan of every
remote the Mac will push: `git ls-remote proton::<address>` once with the new helper, from a folder
with no repository above it (step 5.7's form), the warning being the scan's report (spec section
3). Every later hermetic test run on this Mac falls under rule 12, since a signed-in
`proton-drive` stays on PATH.

## Brief author's calls: an index

Each is marked *Brief author's call* where it applies, with its reason, and each is a default
Craig may overturn, not his ruling (the weightiest are "Questions for Craig"). They sit in: the
gate's place (a setup retry); "R7 and this gate"; the extractors; the escaped view; `start-run.sh`
(git configuration); the Mac kit; relays; W.B.1 and W.B.7 (the task pause, re-enabling); C.2 (the
two documents; the hold, Question 13); C.3 (`-e` and go1.27.0 for the README's lines, the kept
binaries); B.0 (the lock reading's positive control); B.3 and B.6 (a sign-in retry, Craig's
fallback delete); step 5's remotes, 5.0 (NC), 5.4 (the third spelling), 5.5 (5.6's clone first),
5.10 (the recovery) and 5.11 (its place, `r1-clone` left with its stale ref); step 6 (the
failing agents, the probe agent for launchd's environment, which B.0 runs too, the plists'
folder, 6.2's 30 s before the bootout, 6.3's evidence of a landed kill) and its sampler's filter
(`kit/tree.awk`); step 7 (sub-step preconditions, `vet`, what
stays); C.5 (rule 6); the PC kit; and step 0 (git 2.54.0).

## Not covered live by this gate

- **R7 (string 12),** the update refused beside an aliasing same-batch delete: no section 6 step
  pushes a one-push repair from a Mac `files` clone. Hermetic tests cover it
  (`TestPushWithLocalClearsTheRemoteHalfWhenTheOtherMemberIsDeleted`,
  `TestPushWithLocalRefusesAnUpdateWhoseTrackingRefAliasesASameBatchDelete`,
  `TestPushWithLocalLocalHalfIsNotClearedBySameBatchDelete`).
- **Part 2's paired delete and R3's create (string 11), R2's phase-5 re-check, and phase 5's form
  of string 1 (string 2):** they need a same-batch delete paired with a refused write, or a
  phase-4 delete that fails; hermetic only.
- **The rename-delete refusal (string 3):** section 6's rename is two pushes; hermetic only.
- **The lock refusal (string 13):** no step leaves a lock; if one is met, it is compared with
  `hostname` and STOPs.
- **What rule (2) cannot see:** a refspec on the command line, `--refmap`, and a command-line
  `--prune` never reach the helper (spec section 3; plan R6).
- **The in-place repair of a stale local ref** (MA3): `git -c core.precomposeunicode=false
  pack-refs --all`, then the exact-bytes delete through `git update-ref --stdin`. It is the
  README's route, measured in scratch repositories on this Mac [record
  `docs/research/probes/macos-port/README.md`, layouts L10 to L12]. Strings 6 and 7 name only a
  `reftable` clone, which 5.11 runs, so this gate does not demonstrate the in-place route, and no
  step deletes a stale ref in place.
- **Not gated, as section 4 says:** the `pass` and `unsafe_file` stores; SSH sessions; a
  LaunchDaemon; runs during sleep or dark wake; a run after a fresh login; and a Keychain prompt
  after a CLI upgrade. The README's daily `StartCalendarInterval` is not exercised: the gate uses a
  300 s `StartInterval` so the wait is bounded.
- **A real hang:** step 6.3 produces one with a short bound on a read-only push; a Keychain prompt
  on a locked screen is caught only as 6.1's bound.
- **The wrapper as the README installs it:** its `install -m 0755` line into `~/.local/bin`, and a
  plist loaded from `~/Library/LaunchAgents`; step 6 runs the tag's wrapper from the tag's clone
  and loads its plists from the run folder (step 6, "The gate's plist").
- **A successful run removing an earlier failure marker:** the `ok` agent's marker never exists
  before its run, and 7.7 removes the failing agents' markers by hand.
- **`caffeinate` on a locked screen** (section 6: untested): recorded, not judged.
- **Intel Macs, darwin/amd64, and any Mac but this one** (MD1, MD8).
- **Two machines pushing one repository** (O2(b) excludes it): step 4's clones never push back.

## Review record

Multi-model review, round 1, 2026-10-28 (Codex, Gemini, DeepSeek, Kimi), on `7d27ae3`. Applied:

- B1: 6.3 needs no string 14 from a killed push, its push leading its own group, its helper or CLI
  alive at the bound; every sample kept, timed to 0.01 s; `samples.txt` rotated per firing.
- Found in verifying B1: a re-fired 6.3 is judged on its own firing's samples, logs and marker.
- B2 (R-b): Craig holds the keep-awake in his own window; `gx.sh` STOPs a gate script while it is
  down; `-t 172800`; Question 7.
- B3: `logout.state` only after every check passed; `kc_item` three-valued, `kc_expect`.
- B4: no-session records at B.6 (`p6-nosignin`) and 7.3; `cli()` BLOCKED without a CLI; `trashgr`
  reads its record first; C.2 says what step 7 does after it.
- Found in verifying B4: a C.2 STOP holds for Craig's ruling (Question 13); a brief at a tag
  cannot be re-anchored.
- B5: B.0's lock-reading positive control; `s6-1-ok refire`; the tally counts every `ok` firing.
- B6: what `EXIT 0` means, in BLOCKED; W.5.5 checks its start commit.
- B7: `unrefused` stops a script whose push was not refused.
- B8: the writer checks see a PC `git` push and a Mac test binary.
- B9: a marker path is recorded only when no marker is there.
- B10, B27: the retry rule names the in-step re-firings; step 0's git check comes first, a STOP.
- B11: string 12 met in place of a step's output is BLOCKED.
- B12: 7.1 trashes a BLOCKed gate's partial roots.
- B13: a second firing can start before the bootout; `runs = 1` expected.
- B15, B16: the wrapper's path, and two uncovered items, under "Not covered live".
- B20: the renaming's other files named.
- B21: C.2 counts the drift script.
- B25: the probe checks the install folders by name.
- B26: `gh` signed in, checked at W.B.0; `Test-Draft` BLOCKs on a `gh` failure.
- B31: how a session resumes mid-gate.
- B34: Part A after the renaming; B.0 extracts again.
- B35: a skipped NFD upload at 5.4 is a STOP.
- B36: each failing firing's marker must be its own; 30 s before the bootout.
- B38: `d5_check` on every logged push; three handoffs added.
- B39: 6.3 counts the `--all` push's lines instead of `EXPECT`.
- B40: the `launchd` root holds 2 pack pairs.
- R-a: the sampler removes its group file; "What stays" by kind.

Already a question, left as Craig's: B37 (Question 5; 6.0 now records the global config's key
names only), B43 (Question 8; the default stands).

Rejected, so not to be raised again:

- B14: the sampled pushes are refused before git runs `pre-push`, so no hook starts in them.
- B17: string 3's quote is `push.go:480-481`'s text; string 11 is listed apart.
- B18: recorded Proton uids fit the relay charset.
- B19: a wrapper that dies early makes `fire` time out and BLOCK, as it should.
- B22: the tag's brief governs by design, and C.2 records its changes.
- B23: `-f skip --json` and its keys are the helper's own upload, used from step 3.
- B24: 19 is the table's folder trash calls, counted in the code.
- B28: the helper never advertises a peeled tag; one would make `refcmp` fail closed.
- B29: any change to the pin line changes `cli.go`, which C.2 counts.
- B30: the PC kit's entries hold no `<ENV:>`.
- B32: both cases are handled (5.5a's partial clone; a live lock).
- B33: `TRASHED, STILL LISTED` is the tested rule, which 5.8 accepts.
- B41: those checks are read-only or fail closed.
- B42: no macOS asset ships; the README's download carries no quarantine.

Multi-model review, round 2, 2026-10-28 (Codex, Gemini, DeepSeek, Kimi), on `6bf53aa`. Gemini
raised nothing new. Applied, with the round-3 re-review's five minors (R3-a to R3-e):

- B2-1 (Codex): launchd's own environment is checked by a probe agent (`envprobe`) at B.0 and
  6.0; 6.0 STOPs before anything of step 6 exists.
- B2-2 (Codex): 7.0's writer check counts a `go test` launcher.
- B2-3 (Codex): B.3's sign-in sources the current run's `env.sh` and names the run.
- B2-4 (Kimi): string 11's two messages quoted apart, with their code sites.
- B2-5 (Kimi): `kit relayin` checks a value relay's name and shape; `getval` and `relaycmp`
  re-check what is stored.
- B2-6 (Kimi, in part): 6.1 says a hook's refusal is BLOCKED and named in the record; Question 5
  stays Craig's, its default failing closed.
- B2-7 (Kimi): the `launchd` row: a second firing adds two locks but no pack.
- B2-8 (Kimi): `fire` counts firings from the out log after the bootout, a cut firing BLOCKED;
  `kit pushtree` judges one firing.
- B2-9 (DeepSeek): 6.1 lists the cache files before its own CLI calls; 6.5 re-checks step 0's
  condition, the changed-file count recorded.
- B2-10 (DeepSeek): "(b) not exercised" by string 14 names both possible causes.
- B2-11 (DeepSeek): 7.1b compares `GitRemotes` only when it was present at step 0.
- B2-12 (DeepSeek): "The pin": a moved pin cannot stand as written.
- B2-13 (DeepSeek): the tally treats a trashed folder's contents alike in every class.
- R3-a: `kit pushtree` BLOCKs on any other `git-remote-proton:` line in `hang.err.log`; string 17
  is recorded.
- R3-b: W.C.2 and W.7.0 print their handoff only inside an `if` on YES or NONE.
- R3-c: `logged` sees a bare `git push`.
- R3-d: 6.0 prints config key names with any subsection shown as `<subsection>`.
- R3-e: 6.3 STOPs, firing nothing, while no sampler runs; a re-fire restarts it first.

Rejected, so not to be raised again:

- B2-14 (Kimi): string 3 and B17's reason match `push.go:480-481`; the kind precedes "rename".
- B2-15 (Kimi): 6.3's pushes are refused before `pre-push`, so no hook runs in them.
- B2-16 (DeepSeek): M14 measured both tracking refs after a Mac `files` clone.

Multi-model review, round 3, 2026-10-28 (Codex, Gemini, DeepSeek, Kimi), on `3098dc2`. Gemini
raised nothing new. Applied:

- B3-1 (Codex): `Test-NoWriters` fails closed: a process list it cannot read is `WINDOWS WRITERS:
  UNKNOWN`, which W.7.0 never hands off on.
- B3-2 (found verifying B3-1): `Test-Quiet` had the same gap; it is `QUIET: UNKNOWN` now.
- B3-3 (Codex): publication needs every failure classed; a shared-code failure holds the draft
  (section 6, O5(b); `docs/releasing.md` step 5); the class is Craig's ruling.
- B3-4 (Kimi): C.3's and step 1's README STOPs exit 3 and hold as C.2's does; so does C.3's
  failed CLI download.
- B3-5 (Kimi): 6.1 passes two or more firings that all ended; B13's `runs = 1` superseded.
- B3-6 (Kimi): `kit helperlines` counts the known lines apart; "no `git-remote-proton:` line"
  means none but known ones.
- B3-7 (DeepSeek): C.2 checks that the certified CLI's versioned URL still serves; B.0 and C.3 name
  a gone or re-cut CLI.

Rejected, so not to be raised again:

- B3-8 (DeepSeek): 6.0's redaction leaves one-dot names alone (`init.defaultbranch`, tested).
- B3-9 (DeepSeek): `docs/README.md` exists; its line 28 names the steps file.
