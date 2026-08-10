# Stage 6 live gate brief — v0.5.0 foreign-data availability, occupancy-aware push, contract-root/F1 debts

This is a **brief** (prospective instructions for the gate runner), not a record. It implements
design spec component 5's "Live gate (one, small)"
(`docs/superpowers/specs/2026-08-09-v2-stage6-foreign-data-and-debts-design.md`, Component 5)
**verbatim**, in the same numbered order (five steps), fleshed out with the exact commands,
expected wording, and BLOCK conditions the Stage 6 implementation tasks actually produced.
Structure follows `docs/research/gates/stage5-gate-brief.md`'s house style (Preconditions →
numbered outline steps → Release integration → Confinement rules).

**This brief incorporates `docs/research/gates/brief-checklist.md` BY REFERENCE in full,
including its two Stage-6 lines** (item 8: state the contract table's live root and fold it into
the confinement list; item 9: run pushes with a long tool timeout or in background). Every rule
in that checklist applies to every step below without being re-derived; where a rule is
load-bearing for a specific step it is also called out inline, as a pointer back to the
checklist, not a new rule.

**Every `git push` in this brief runs with a long tool timeout (10 minutes or more) or in the
background** (checklist item 9, the Stage 5 S1 lesson: a 2-minute harness timeout killed a
bootstrap push mid-flight and orphaned the repo lock, which then had to be diagnosed and cleared
by hand — see `docs/research/gates/stage5-gate.md`'s S1). This is stated once, here, and applies
to every push step below without repeating it. The one `go test` invocation in this brief
(outline step 3) has historically run 5–10+ minutes live (17 contract cases; the Stage 3b
9-case table took 270s) and gets the same long-timeout-or-background treatment — **and (run-1
amendment, 2026-08-10, adjudicated): `go test`'s OWN default 10-minute deadline must be raised
explicitly with `-timeout 40m`; a harness-side timeout does not cover it. Run 1 of this gate
BLOCKED at exactly this — the live table panicked at `test timed out after 10m0s` with all 17
subtests started and none failed: a command-validity defect in this brief's original text, not
a product defect.**

Release integration follows `docs/releasing.md` step 5: **this gate runs against the v0.5.0
DRAFT's bytes** (downloaded from the draft, digests staged before any account write), never
against a locally-built exe — see "Release integration" near the end of this brief.

**Remote naming convention used throughout this brief — do not conflate the two.** `proton::<path>`
is the URL **scheme** `git-remote-proton` registers with git (`README.md:97`). Use the literal
scheme for every command that talks to the helper before any local git remote exists for that
path: `git ls-remote proton::...`, and the first `git clone -o proton-v2 proton::...` in a given
local repo. `proton-v2` is a **local remote alias**, not a URL scheme — it exists only after
`git clone -o proton-v2 proton::...` or `git remote add proton-v2 "proton::..."` has run in that
specific local repo, and every subsequent git command in that repo (`git push proton-v2 ...`,
`git fetch proton-v2 ...`) uses the alias. **`proton-v2::` is never valid anywhere** — git would
look for a nonexistent `git-remote-proton-v2` binary and fail outright. Every command below has
been checked against this distinction.

---

## Preconditions

1. **CLI / git versions.** Record `proton-drive --version` (must be the certified
   `cli-drive@0.7.0+5174900c` build, unchanged since Stage 4/5 — Stage 6 makes no change to
   `internal/transport/cli.go`) and `git --version`.
2. **PATH shadow check** (Stage 4 lesson, still binding): in a genuinely fresh shell
   (`$env:Path = [Environment]::GetEnvironmentVariable('Path','Machine') + ';' +
   [Environment]::GetEnvironmentVariable('Path','User')`), confirm `(Get-Command
   git-remote-proton -All).Source`'s FIRST hit is the draft-installed helper. Any shadowing =
   BLOCKED.
3. **Empty the trash before the gate** (checklist item 6) — cheap insurance, taken on report
   from Craig's web-UI action unless the CLI can verify it directly.
4. **Pre-run `/my-files` listing**, recorded before anything else runs, row-set form (checklist
   item 1): `uid`, `name`, `type`, `creationTime`, `modificationTime`, `parentUid` for every row.
   This is the baseline the cleanup step's post-run listing must match, row-set-for-row-set.
5. **Write confinement, stated explicitly (checklist item 2):**
   - `/my-files/GitRemotes/stage6-gate` — the demo repo, plus the gate-authorized creation of
     `/my-files/GitRemotes` itself if absent. (It is very likely absent: Stage 5's own cleanup
     trashed the entire `/my-files/GitRemotes` folder and its subtree —
     `docs/research/gates/stage5-gate.md:532`.) **This authorization is symmetric: this gate also
     trashes `/my-files/GitRemotes` itself in outline step 5's cleanup**, after verifying it
     holds nothing but this gate's own subtree — the same close every prior gate took
     (`docs/research/gates/stage3b-gate.md:270`, `stage4-gate.md:747`, `stage5-gate.md:512`).
     Creating it without ever authorizing its own removal would leave the Preconditions-step-4
     baseline permanently unmatchable by the post-run listing (one extra row, forever) — see
     outline step 5 below.
   - `/my-files/GitRemotes/stage6-gate-contract` — **the contract table's live root for this
     gate (checklist item 8), named explicitly here and nowhere implicit.** Outline step 3 below
     points `GPB_CONTRACT_LIVE_ROOT` at this path so the live contract half never needs to write
     outside this brief's own confinement — the exact defect Stage 5's S2 recorded
     (`docs/research/gates/stage5-gate.md`'s S2: a brief silent on the contract table's root
     forced the runner to refuse the pinned write and improvise a workaround). This brief does
     not repeat that shape: the root is named here, before any step needs it.
   - Untouchables (`GitBackups`, `Sensitive Project Sources`, `Project Repo Bundles`, `ChatGPT
     Export Text Backup`) are read-only: they may appear only as rows in listings, never named
     in any write command.

**Allowlist and Stage 5's signature-constant pins are not re-provoked live this stage.** Stage 6
touches `internal/repo/refs.go`, `internal/repo/refscan.go`, `internal/repo/push.go`,
`cmd/git-remote-proton/main.go`'s protocol loop, and `internal/transport/contract_test.go` — it
does not touch `internal/transport/cli.go` at all (confirmed: the certified-CLI allowlist and
`notFoundSignature`/`alreadyExistsSignature` were last changed in a Stage 5 commit, `6a43ed1`,
2026-08-07, Task 9b's review round). Both were already proven live at the Stage 4 and Stage 5
gates and are covered hermetically ever since (`internal/testcli`, the contract table's Fake
half). Re-provoking them here would reprove something already proven, for no new information.

---

## Runner-asserted strings — reference and BLOCK rule

**Every string quoted below is copied verbatim from the source, character for character,
including two things that read like typos but are not: a doubled "not a ref" phrase, and a
literal, unsubstituted `<path>` placeholder. Do not "fix" either — a mismatch against the
literal shipped text is a BLOCK; a match against a "corrected" version you improvised is not
evidence of anything.**

**Stream note, applies to every quote in this section:** `git-remote-proton` writes every one of
these strings directly to its own process's `os.Stderr` (`warn()`, `skipNote()`, and the
occupancy `fail()` path all use it) or, for push results specifically, through the
remote-helper *protocol* `error` line, which git itself renders on the terminal. Neither path
goes through a "remote:" prefix — that prefix belongs to the smart-HTTP protocol's server-side
output; `git-remote-proton` runs as a **local subprocess** git launches directly for the
`proton::` scheme, and its stderr is inherited straight through to the terminal, exactly as
Stage 5's stale-lock message and heal note were observed directly
(`docs/research/gates/stage5-gate.md`, S1 and outline step 2). Every quote below is therefore
TERMINAL-visible without `GIT_TRACE` — unlike a helper's protocol `ok` lines, which are not.

**One more legitimate stderr source, not itself a runner-asserted string but expected
throughout:** `main.go` wraps the CLI transport in `transport.NewTraced` before the protocol
loop ever runs (`cmd/git-remote-proton/main.go:173`), and `Traced.ReadTo` prints one line per
SUCCESSFUL download — `gpb: downloaded <remote path> (<n> bytes)\n` — to the same `os.Stderr`
(`internal/transport/trace.go:38-48`; its own doc comment calls the `"gpb: downloaded "` prefix
NORMATIVE). Every `readRef` call goes through `t.ReadTo`, so this line fires on every in-band
candidate `ScanRefs` actually downloads — including a junk file, before its grammar check fails.
**`Traced.ReadTo` is the ONE handle wrapping every download this helper makes, not only ref
reads** — fetch and clone route pack/index downloads through the same transport, so expect
`gpb: downloaded .../packs/pack-*.pack` and matching `.idx` lines during outline step 2's clone
and outline step 4's fetches too; these are ordinary, unrelated to the junk fixtures, and not
something to cross-check against anything in this brief. Seeing `gpb: downloaded ...` lines
interleaved with the strings below is expected, not an anomaly; outline step 1 below asserts the
specific positive/negative pair this stage's size gate predicts, scoped by the exact junk
filenames — `gate-junk-a`/`gate-junk-b` — never by count or by conflating them with a
pack/idx line that happens to appear nearby.

1. **The strict-fetch enumerated error** (`cmd/git-remote-proton/main.go`, the `case line ==
   "list":` arm, ~line 372). Surfaced by `git ls-remote`, `git fetch`, or `git clone` against
   this remote's URL scheme whenever `scan.ContentSkips()` is nonempty; written via `warn()` to
   stderr, then the helper exits 1. Template (`%d` = count, then one `  <root>/<path>: <reason>`
   line per content-skipped file, in **walk order — not guaranteed stable, per F2**,
   `docs/research/gates/stage5-gate.md`'s F2 finding that `filesystem list` row order is
   unstable):
   ```
   git-remote-proton: cannot serve a fetch: <N> file(s) under refs/ are not valid refs and a restore would silently lack them:
     <root>/<path>: <reason>
   delete these files first (proton-drive filesystem trash <path>, or the web UI; Proton trash keeps them restorable), then retry
   ```
   **The final line's `<path>` is a literal, unsubstituted placeholder in the shipped code** —
   not a per-file computed value (contrast the enumerated lines above it, which DO substitute a
   real path per file). Quote it exactly as `<path>`, angle brackets included; this is not this
   brief's error.
2. **`OccupancyMessage`'s three kinds** (`internal/repo/refscan.go:70-88`). Exact templates,
   `%s` in the order the code substitutes them:
   - `SkipContent` — `"a file occupies %s and its contents are not a ref (%s); delete it first
     (proton-drive filesystem trash %s, or the web UI)"`, args `(s.Path, s.Reason, root+"/"+s.Path)`.
     **When `s.Reason` itself begins "not a ref: ..." (the ordinary case for generic junk —
     `classifyRefContent`'s non-noncanonical branch, `internal/repo/refs.go:312`), the rendered
     message reads "...are not a ref (not a ref: ...)" — a doubled phrase. This IS the shipped
     text**, not a transcription error: the template's own "are not a ref (%s)" wraps around a
     `Reason` that already starts with the same three words for that class of file. Outline step
     2 below hits exactly this case and quotes the doubled form in full.
   - `SkipInvalidNameFolder` — `"a folder with an invalid name occupies %s; its contents were
     never examined - inspect it before removing anything (the web UI, or a CLI listing of %s,
     will show what's inside before you decide)"`, args `(s.Path, root+"/"+s.Path)`. Note the
     literal ASCII hyphen surrounded by spaces (" - "), not an em dash.
   - `SkipInvalidName` (default) — `"a file with an invalid ref name occupies %s (contents never
     examined); delete or rename it first (proton-drive filesystem trash %s, or the web UI)"`,
     args `(s.Path, root+"/"+s.Path)`.
   - **Only the `SkipContent` kind is provoked live by this brief** (outline step 2). The other
     two require a NAME-invalid file or folder, which is a different scan outcome
     (`SkipInvalidName`/`SkipInvalidNameFolder`) that spec component 5 keeps hermetic-only for
     this stage's live gate — the same posture Stage 5's brief took for its own foreign-file
     heal variant ("the foreign-file variant stays hermetic-only",
     `docs/research/gates/stage5-gate-brief.md` outline step 2 item 3). Quoted here for
     completeness and for the next stage that needs them live, not asserted by any step below.
3. **The heal-arm race diagnosis** (`internal/repo/push.go`'s `createRefHealingCollision`,
   ~lines 807–840) reuses `OccupancyMessage` **verbatim** — but only ever with `Kind:
   SkipContent` (both `SkippedRef` literals it builds, lines 818 and 839, hardcode that kind; the
   occupant it diagnoses is reached via `Stat` on an already-`checkDst`-validated destination
   name, so a NAME-invalid occupant can never arise here — the other two `OccupancyMessage`
   kinds are structurally unreachable from this call site, not merely unexercised) — from a
   DIFFERENT call site than item 2 above: a `Stat` performed AFTER a create's `WriteRef` was
   refused, diagnosing an occupant that appeared in the race window between the batch's
   advertisement scan and its own write attempt. **This brief's outline step 2 does NOT reach
   this code path.**
   The junk files are uploaded and visible to `ScanRefs` well before the colliding push is ever
   sent, so `occupied[u.Dst]` in `Push`'s phase-2 preflight (`internal/repo/push.go:222`) catches
   the collision first, by construction — the same "preflight wins, race-window arm
   unreached" relationship Stage 5's brief documented between rule 2b and
   `createRefHealingCollision`'s folder-heal ("Why not `describeBlockers`'... wording",
   `docs/research/gates/stage5-gate-brief.md` outline step 2 item 3). Provoking the race-window
   arm live would require a foreign write landing in that exact window, which is not
   deterministically provokable — it stays hermetic
   (`internal/repo/repo_test.go`'s `TestHealArmDiagnosesUnderBandOccupantWithoutDownload`,
   `TestHealArmDiagnosesOversizedOccupantWithoutDownload`,
   `TestHealArmDiagnosesInBandJunkOccupant`, and siblings). Quoted here for completeness only.
4. **`skipNote`** (`internal/repo/refs.go:186-188`): `"git-remote-proton: skipping %s/%s: %v\n"`,
   args `(root, full, reason)`. Fires on **every** `ScanRefs` walk that finds a skipped path —
   both junk files, in outline step 1 (tolerant push) and again inside outline step 1's
   `ls-remote` failure path's own scan, and a THIRD and FOURTH time across outline step 2's two
   `ScanRefs` calls (see the note under outline step 1 below on why the count is not "once per
   push"). Directly terminal-visible per the stream note above.
5. **`headSkipNote`** (`cmd/git-remote-proton/main.go:673-676`): `"git-remote-proton: HEAD names
   %s, which was skipped (%s); advertising no default branch\n"`. **Not exercised live by this
   brief** — none of this gate's steps ever point HEAD at a skipped name (HEAD is set once, to
   `main`, by the first successful push in outline step 1, and never moved). Quoted here for
   completeness; the degraded-state coverage for this is entirely hermetic per spec component 5.

**BLOCK rule, restated with extra force for this section (checklist item 4):** if any live
observation of strings 1, 2 (`SkipContent` kind), or 4 above differs from the shipped text —
including whitespace, the doubled phrase, or the literal `<path>` placeholder — the gate reports
BLOCKED with the verbatim output and does NOT patch, retry, or "close enough" the mismatch.

---

## Outline step 1 — Junk manufacture, tolerant push, strict `ls-remote`

Per spec component 5's live-gate outline item 1.

0. **Bootstrap the demo repo**, run from a fresh local working directory:
   ```
   git init -b main stage6-gate
   cd stage6-gate
   git commit --allow-empty -m "gate: stage6 initial commit"
   git remote add proton-v2 "proton::/my-files/GitRemotes/stage6-gate"
   proton-drive filesystem create-folder /my-files/GitRemotes
   git push -u proton-v2 main
   ```
   (`-b main` is explicit, not `init.defaultBranch` — every later command in this brief names
   `main` literally.) **Run-1 amendment (2026-08-10, adjudicated):** the original text claimed
   the bootstrap push itself "creates `/my-files/GitRemotes` if absent" — wrong: the helper
   refuses missing parents unless `GPB_CREATE_PARENTS=1` is set, which this brief never sets.
   The explicit `create-folder` above (authorized by Preconditions step 5) is the correct
   bootstrap when the folder is absent; run 1's runner performed exactly this as an
   adjudicated-accepted in-spirit deviation. The push then creates the repo marker and
   `refs/heads/main`.
1. **Manufacture two junk files** at valid ref names under `refs/heads` — CLI upload of a
   non-ref file, the web-UI-equivalent action. One lands IN the 40–42-byte candidate band (gets
   downloaded and grammar-checked); one is oversized (classified by size alone, never
   downloaded) — the two `readRefClassified` arms (`internal/repo/refs.go:143-172`). Stage the
   local files OUTSIDE the git repo's own working tree (a separate scratch folder), so they
   never show up as untracked files in `git status` inside the demo repo and cannot be mistaken
   for something that needs `git add`ing:
   ```
   New-Item -ItemType Directory -Force -Path C:\gpb6-junk | Out-Null
   [System.IO.File]::WriteAllText('C:\gpb6-junk\gate-junk-a', ('x' * 41), [System.Text.Encoding]::ASCII)
   [System.IO.File]::WriteAllText('C:\gpb6-junk\gate-junk-b', ('y' * 100), [System.Text.Encoding]::ASCII)
   proton-drive filesystem upload C:\gpb6-junk\gate-junk-a /my-files/GitRemotes/stage6-gate/refs/heads
   proton-drive filesystem upload C:\gpb6-junk\gate-junk-b /my-files/GitRemotes/stage6-gate/refs/heads
   ```
   `[System.Text.Encoding]::ASCII` guarantees no BOM and exactly one byte per character, so
   `gate-junk-a` is exactly 41 bytes and `gate-junk-b` is exactly 100 bytes with no trailing
   newline — `filesystem upload` names each node after its local basename (probe C11), so these
   land as `refs/heads/gate-junk-a` and `refs/heads/gate-junk-b`, both otherwise-valid branch
   names.
   **Verify before relying on the strings below:**
   `proton-drive filesystem list /my-files/GitRemotes/stage6-gate/refs/heads --json` and confirm
   the reported sizes are exactly 41 and 100. **If they differ, recompute the expected note text
   from the templates in "Runner-asserted strings" above using the actually-observed size or
   content, rather than treating a mismatch against the numbers below as a BLOCK** — only a
   mismatch against the recomputed, actually-applicable text is a BLOCK. Given the construction
   above, the expected classified reasons are:
   - `gate-junk-a` (in-band, downloaded and grammar-checked): `not a ref:
     "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"` (41 literal `x` characters inside the quotes —
     `classifyRefContent`'s generic-junk branch, `internal/repo/refs.go:301-312`; not the
     noncanonical damaged-ref branch, since the bytes are not 40-hex).
   - `gate-junk-b` (out-of-band, never downloaded): `not a ref: size 100 outside the 40-42
     candidate band` — `readRefClassified`'s pre-download size gate,
     `internal/repo/refs.go:144-158`. **The absence of a download for this file IS
     independently observable** — see item 2 below for the exact trace-line pair this predicts;
     do not rely on the note's wording alone as the only evidence (an earlier draft of this
     brief claimed the wording was the only available evidence — it was wrong: the helper's
     download tracer makes the absence directly visible on stderr).
2. **Push of an unrelated ref succeeds** (tolerant direction — long timeout/background, per the
   standing note above):
   ```
   git push proton-v2 main:push-ok
   ```
   Expect exit 0 and git's own `* [new branch] main -> push-ok` line. **Expect the classified
   `skipNote` lines for BOTH junk files on stderr — possibly each appearing TWICE.** `git push`
   always sends `list for-push` before the batch (`cmd/git-remote-proton/main.go:425`, whose
   handler calls `repo.ScanRefs`), and the `push ` handler
   (`cmd/git-remote-proton/main.go:540`) calls `ScanRefs` a SECOND time to build the occupancy
   set for phase 2 — `ScanRefs` itself calls `skipNote` as a side effect of the walk
   (`internal/repo/refs.go:156,170`), unconditionally, every time it runs. Seeing each junk
   file's note line twice in one `git push` invocation is correct, not a defect; do not BLOCK on
   the duplication, and do not BLOCK if a given implementation detail causes only one scan to
   run instead — either count is consistent with the code, since nothing in the design commits
   to exactly one `ScanRefs` call per push. The content of each note (once you have it) is what
   this brief asserts, from "Runner-asserted strings" item 4 above.

   **Also expect the positive/negative `gpb: downloaded` trace pair — this IS the spec's
   "observed via the absence of a transfer" requirement, made concrete:**
   ```
   gpb: downloaded /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-a (41 bytes)
   ```
   must appear on stderr at least once (the in-band candidate — `readRefClassified` downloads it
   via `readRef`/`t.ReadTo` before the grammar check fails; `Traced.ReadTo`,
   `internal/transport/trace.go:38-48`, logs every successful download regardless of what
   happens to the bytes afterward) — it may appear more than once, for the same reason
   `skipNote` can. **No line naming `gate-junk-b` may EVER appear** — not once, not with any
   byte count — for the whole rest of this brief up to and including its deletion in outline
   step 2: the size gate refuses it before `ReadTo` is ever called, so its absence from every
   `gpb: downloaded` line in this gate's entire transcript is the live, terminal-visible evidence
   that no read happened for it. A `gpb: downloaded .../gate-junk-b ...` line appearing ANYWHERE
   is a BLOCK — it would mean the pre-download size gate did not fire.
3. **`git ls-remote` FAILS with the enumerated error** (strict direction):
   ```
   git ls-remote proton::/my-files/GitRemotes/stage6-gate
   ```
   Expect nonzero exit and stderr containing, verbatim (item 1 in "Runner-asserted strings"
   above), with `<N>` = 2, `<root>` = `/my-files/GitRemotes/stage6-gate`, and the two enumerated
   lines (order not guaranteed — F2) naming `refs/heads/gate-junk-a` with its reason and
   `refs/heads/gate-junk-b` with its reason, exactly as recomputed in step 1 above.
4. **The junk files stay in place for outline step 2** — do not trash them here. (An earlier
   draft of the Stage 5 brief made this mistake for its own analogous step; the round-5 fix that
   caught it there applies with equal force here: step 2's occupancy-collision push needs
   something to collide with.)

## Outline step 2 — Push onto the junk name, then delete and recover

Per spec component 5's live-gate outline item 2. Same repo/alias as outline step 1 throughout.

1. **Row-set baseline before the refused push**:
   `proton-drive filesystem list /my-files/GitRemotes/stage6-gate/packs --json` (row-set form,
   checklist item 1) — record it; this is what the post-refusal listing below must match
   exactly, proving no pack was uploaded by the refused batch.
2. **Create a withheld object, THEN push onto the occupied name** (long timeout/background):
   ```
   git commit --allow-empty -m "gate: stage6 withheld object"
   git push proton-v2 main:gate-junk-a
   ```
   **The `commit --allow-empty` here is load-bearing, not filler.** Without it, `main`'s local
   tip is still the SAME commit that both `refs/heads/main` and `refs/heads/push-ok` already
   advertise on the remote — `gitcmd.WritePack`'s `haves` set (built from every currently-
   advertised remote ref this local repo already holds, `internal/repo/push.go:389-394`) would
   already cover that object, so `wants` minus `haves` is empty and NO pack would be uploaded
   for this push **even if the occupancy refusal had a bug and let it through** — making item 3
   below ("row-set proof no pack was uploaded") pass regardless of whether the refusal actually
   fired. The new commit is a genuinely unpublished object: if the phase-2 preflight refuses this
   push (the expected, correct outcome), the new commit is never packed or uploaded, because
   `valid[i]` stays `false` and `newShas[i]` is never populated by the occupancy `continue`
   (`internal/repo/push.go:222-233`) — it never reaches the pack engine's own phase 3
   `wants`-building loop at all (`internal/repo/push.go:356-361`, which only appends `newShas[i]`
   for entries where `valid[i]` is true — note this "phase 3" is `Push`'s internal five-phase
   engine, unrelated to this brief's own outline steps). If a bug let the create through instead,
   THIS object — new, not already on the remote via any other ref — would force a real,
   non-empty pack upload, which item 3 below's row-set comparison would then catch. This restores
   the check's discriminating power.

   **Local-only side effect, carried forward:** local `main` now sits one commit ahead of remote
   `refs/heads/main` (which is never touched by this refused push and stays at the original
   bootstrap commit). Outline step 4 branches `feature/x` from whatever `main` currently is, so
   `feature/x` will include this "stage6 withheld object" commit as an ancestor — harmless, and
   means outline step 4's own push of `feature/x` (unlike `push-ok`'s push in outline step 1)
   WILL upload a real, non-empty pack, since that withheld object has still never been published
   to any remote ref by the time step 4 pushes it. This is expected, not a regression signal.

   `gate-junk-a` has no local or remote ref, so git's own push-refspec DWIM resolves the bare
   name to `refs/heads/gate-junk-a` — the same resolution Stage 5's brief relied on for
   `main:heal-empty` and `main:feature`. `refs/heads/gate-junk-a` is an EXACT-NAME occupancy hit
   in `Push`'s phase-2 preflight (`internal/repo/push.go:222`), refused before any pack is
   built. Expect exit 1 and, on the terminal, git's own rejection wrapper around the
   `SkipContent` `OccupancyMessage` computed for `gate-junk-a` above:
   ```
    ! [remote rejected] main -> gate-junk-a (a file occupies refs/heads/gate-junk-a and its contents are not a ref (not a ref: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"); delete it first (proton-drive filesystem trash /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-a, or the web UI))
   error: failed to push some refs to 'proton::/my-files/GitRemotes/stage6-gate'
   ```
   (Format matches `docs/research/gates/stage5-gate.md:282`'s recorded rendering of the same
   git rejection wrapper for a different refusal reason — the wrapper itself is git's, stable
   across reasons; the parenthetical is the code-sourced text this brief pins.) Note the doubled
   "not a ref (not a ref: ...)" — see "Runner-asserted strings" item 2 above; this is the
   shipped text.
3. **Row-set proof no pack was uploaded** (checklist item 1, BLOCK on mismatch — the Stage 5
   signature-pin discipline applied to this stage's own load-bearing assertion):
   `proton-drive filesystem list /my-files/GitRemotes/stage6-gate/packs --json` again; compare
   field-by-field against step 1's baseline. Any new row is a BLOCK.
4. **Verify before trash** (checklist item 3): `proton-drive filesystem list
   /my-files/GitRemotes/stage6-gate/refs/heads --json` and confirm exactly four entries —
   `main`, `push-ok`, `gate-junk-a`, `gate-junk-b` — before targeting the two junk names.
5. **Delete both junk files via the CLI**:
   ```
   proton-drive filesystem trash /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-a
   proton-drive filesystem trash /my-files/GitRemotes/stage6-gate/refs/heads/gate-junk-b
   ```
   Re-list `refs/heads` and confirm exactly `main` and `push-ok` remain.
6. **`git ls-remote` and a clone now succeed with all real refs** (complete-restore recovery
   observed live):
   ```
   git ls-remote proton::/my-files/GitRemotes/stage6-gate
   ```
   Expect exit 0, two rows for `refs/heads/main` and `refs/heads/push-ok` (their SHAs), plus a
   `HEAD` row carrying the SAME sha as `refs/heads/main`. **Do not expect the literal text
   `@refs/heads/main HEAD`** — that is the wire-protocol symref line
   (`cmd/git-remote-proton/main.go:410`, `fmt.Fprintf(out, "@%s HEAD\n", branch)`) git itself
   consumes and resolves before printing; plain `git ls-remote` renders it as an ordinary
   `<sha>\tHEAD` row, exactly as recorded live in `docs/research/gates/stage5-gate.md:222`.
   ```
   git clone -o proton-v2 proton::/my-files/GitRemotes/stage6-gate C:\gpb6\clone1
   ```
   into a short local path (Stage 4's R2-2 MAX_PATH lesson — keep every clone destination
   shallow; `C:\gpb6\clone1` is short enough, substitute another equally short path if it's
   already taken on the runner's machine). Expect success, `main` checked out. **Keep this
   clone** — outline step 4 reuses it.

## Outline step 3 — Contract table live, including F1

Per spec component 5's live-gate outline item 3, and checklist item 8.

**Working directory for this step, stated explicitly: the git-proton-backup SOURCE checkout this
gate is being run from — its repository root (where `go.mod` lives, and `./internal/transport/`
resolves as a package path).** This is NOT the demo repo (`stage6-gate`) and NOT either of its
clones from outline steps 1–2 — every prior command in this brief has been inside one of those,
so change back to the source checkout's root first (`cd` there, using wherever this gate's own
`git-proton-backup` clone actually lives on the runner's machine) before running the block below,
the same anchoring Stage 3b/4/5's briefs assumed for their own `go test ./internal/transport/
...` invocations (`docs/research/gates/stage3b-gate.md:69,389`):
```
$env:GPB_LIVE_ACCOUNT = "1"
$env:GPB_CONTRACT_LIVE_ROOT = "/my-files/GitRemotes/stage6-gate-contract"
go test ./internal/transport/ -run TestContractCLI -count=1 -timeout 40m -v
Remove-Item Env:GPB_LIVE_ACCOUNT
Remove-Item Env:GPB_CONTRACT_LIVE_ROOT
```
(`-count=1` mandated — checklist item 5, the Stage 3b lesson that a cached run must never stand
in for a live one; confirm the run duration is NOT the sub-second signature of a skip, the same
tell Stage 3b's own gate used.) `TestContractCLI` validates `GPB_CONTRACT_LIVE_ROOT`
(`internal/transport/contract_test.go`'s `validateContractLiveRoot`) BEFORE any live call — the
value above is a plain path strictly under `/my-files/`, not `/my-files` itself, not prefixed by
an untouchable folder, with no `.`/`..` segments, so it passes. The test then `EnsureDir`s this
base once and, **for every one of the 17 subtests, creates and auto-trashes its own per-test
subfolder** (`t.Cleanup`) — the base folder itself is NOT auto-trashed and is this brief's own
job in outline step 5.

Confirm in the `-v` output: all 17 subtests PASS, including
`TestContractCLI/download_of_a_directory_recursively_materialises_the_subtree_(F1)` — the new F1
row (spec component 3), which downloads a live two-level directory via `filesystem download` and
confirms both files land under `<dest>/<leaf>/...` with the relative layout preserved. Any
non-PASS result anywhere in this run is a BLOCK, verbatim output, no patch, no retry.

## Outline step 4 — Hierarchical smoke: nested branch push/fetch round-trip

Per spec component 5's live-gate outline item 4 — no regression, kept small. Reuses the
outline-step-1 gate repo (main checked out) and the outline-step-2 clone.

1. In the gate repo (long timeout/background):
   ```
   git switch -c feature/x
   git commit --allow-empty -m "gate: stage6 nested branch commit"
   git push proton-v2 feature/x
   ```
2. In the outline-step-2 clone: `git fetch proton-v2`. Confirm `refs/remotes/proton-v2/feature/x`
   now exists and `git rev-parse proton-v2/feature/x` matches `git rev-parse feature/x` run in
   the gate repo.
3. Back in the gate repo (still on `feature/x`, long timeout/background):
   ```
   git commit --allow-empty -m "gate: stage6 incremental commit"
   git push proton-v2 feature/x
   ```
4. In the clone: `git fetch proton-v2` again. Confirm `proton-v2/feature/x` now matches the new
   commit. Since both junk files were deleted in outline step 2, this fetch's own `ScanRefs`
   walk has nothing to skip — a clean run here is also live confirmation that the new strict
   `list` policy (outline step 1 item 3) imposes no regression on an ordinary fetch once the
   remote is clean.

## Outline step 5 — Cleanup, row-set comparisons, trash accounting

Per spec component 5's live-gate outline item 5, and checklist items 1/3/7 inline.

- **First, check the Preconditions-step-4 baseline: did `/my-files/GitRemotes` already have a
  row there BEFORE this gate ran?** Do this before any of the listing/trash steps below, since it
  determines whether they apply at all. The expected case (per Preconditions step 5) is NO —
  this gate creates it fresh, and everything below (subtree verification, trashing both children,
  then trashing `GitRemotes` itself) applies as written. **If the baseline already shows a
  `GitRemotes` row, this gate did NOT create it, and the folder itself is NOT this gate's to
  trash** — trash only its two children (the subtree-verification and both-children-trash steps
  below still apply unchanged), then STOP: skip the "verify and trash `/my-files/GitRemotes`
  itself" step entirely. In that branch, `/my-files/GitRemotes` reappears in the post-run
  comparison with the SAME `uid`/`name`/`type`/`creationTime` as the baseline, but its
  `modificationTime` MAY legitimately differ — this gate wrote children into it and then removed
  them, and Proton is free to bump a folder's own modification time for that. A differing
  `modificationTime` on `/my-files/GitRemotes` alone, in this branch only, is NOT a BLOCK; every
  other field (and every other row, including the four untouchables) still must match exactly.
- **Verify-before-trash with full subtree enumeration** (checklist item 3): `filesystem list`
  the full subtree of `/my-files/GitRemotes/stage6-gate` (refs/heads holding `main` and
  `push-ok` as FILES, plus a `feature` FOLDER containing `x` — outline step 4's nested branch is
  a folder, not a third flat file; refs/tags: empty; `packs/`; the repo marker) and confirm it
  contains only this gate's own artifacts. Separately, `filesystem list
  /my-files/GitRemotes/stage6-gate-contract` and confirm it is now EMPTY (every per-test
  subfolder already auto-trashed by outline step 3's `t.Cleanup`) before trashing the base
  folder itself. **Apply the same transient-lag tolerance here as the `/my-files/GitRemotes`
  check below**: if this listing still shows a just-trashed per-test subfolder carrying a
  `trashTime` field, wait briefly and re-list before concluding it is non-empty — only a
  persistent non-empty listing after a re-list means it is not actually empty (and is itself a
  BLOCK, since it would mean a subfolder `t.Cleanup` was supposed to trash did not commit).
- **Trash both children, THEN — fresh-creation case only — verify and trash
  `/my-files/GitRemotes` itself too** — the demo repo and the contract root are `GitRemotes`'
  only two children (both gate-created; Preconditions step 5 authorizes this symmetrically with
  authorizing `GitRemotes`' own creation, same close every prior gate took:
  `docs/research/gates/stage3b-gate.md:268-270`, `stage4-gate.md:743-749`,
  `stage5-gate.md:508-514`):
  ```
  proton-drive filesystem trash /my-files/GitRemotes/stage6-gate
  proton-drive filesystem trash /my-files/GitRemotes/stage6-gate-contract
  proton-drive filesystem list /my-files/GitRemotes --json
  ```
  Confirm the listing is empty (no rows) — if a just-trashed row still appears carrying a
  `trashTime` field, this is the **transient post-trash listing lag** prior gates recorded
  live (`docs/research/gates/stage5-gate.md:536-538`: the first post-trash listing still showed
  the just-trashed row for several seconds; `stage4-gate.md` noted the same window). Wait briefly
  and re-list; only a listing that STILL shows a non-trashed row after a re-list is a BLOCK.
  Once genuinely empty:
  ```
  proton-drive filesystem trash /my-files/GitRemotes
  ```
- **All listing comparisons as row sets** (checklist item 1), not raw JSON byte equality, for
  every pre/post comparison in this gate — not only this final one.
- **Trash accounting** (checklist item 7's spirit, though this gate has nothing of item 7's own
  specific class to count — see below): this gate never runs `git push --delete` and never hits
  a self-heal folder collision, so `push.go`'s own prune/heal folder-trashing (the thing
  checklist item 7 exists to keep from being undercounted) never fires this stage, unlike Stage
  5. For the record, tally: the two junk FILES trashed by hand in outline step 2
  (`gate-junk-a`, `gate-junk-b`); the 17 per-test SCRATCH FOLDERS `TestContractCLI`'s own
  `t.Cleanup` auto-trashed during outline step 3 (test-harness housekeeping, not a v2 prune
  event — do not conflate the two classes); and the gate ROOT folders trashed just above — THREE
  in the expected fresh-creation case (the demo repo subtree, the contract base folder, and
  `/my-files/GitRemotes` itself), only TWO if the baseline check above found `GitRemotes`
  pre-existing (the demo repo subtree and the contract base folder only).
- **Post-run `/my-files` listing** must be row-set-identical to the pre-run listing from
  Preconditions step 4: no `trashTime` on any row, same uids, same creation/modification times —
  **except** `/my-files/GitRemotes`'s own `modificationTime` in the pre-existing-`GitRemotes`
  branch above (first bullet), where a differing `modificationTime` on that ONE row alone is not
  a BLOCK, exactly as that bullet already states; every other row and every other field is held
  to the strict identity rule. **This listing is taken immediately after the cleanup trash(es)
  above (`/my-files/GitRemotes` itself in the fresh-creation case; its two children only in the
  pre-existing case), so the same transient post-trash listing lag applies here** — if a
  just-trashed node still appears in this listing carrying a `trashTime` field, that is the same
  lag `docs/research/gates/stage5-gate.md:534-538` recorded live under its own "Post-run
  `/my-files` listing" heading (the first post-trash listing there still showed the just-trashed
  `GitRemotes` row). Wait briefly and re-list; only a listing that STILL shows a non-trashed or
  mismatched row after a re-list is a BLOCK.

---

## Release integration

Per spec component 5's "Release" line and `docs/releasing.md` steps 4–7. This brief's own
companion commit performs `docs/releasing.md` step 2 (the CHANGELOG flip, `## Unreleased` →
`## 0.5.0 — 2026-08-09`) alongside writing this file — steps 3 onward are Craig-directed:

- **This gate runs against the v0.5.0 DRAFT's bytes**, not a local build. Download the draft's
  three assets (`git-remote-proton.exe`, `git-remote-proton.exe.sha256`, `install.ps1`) into a
  fresh, empty gate directory; record SHA-256 for all three as the **staged baseline** before any
  account write, exactly as `stage4-gate.md`/`stage5-gate.md` did for their releases. Verify the
  sidecar independently (recompute, don't trust the installer's own check). Run the installer;
  confirm a fresh shell resolves the installed helper first (Preconditions step 2) and
  `--version` reports `git-remote-proton v0.5.0 (certified CLI: cli-drive@0.7.0+5174900c)`.
- **Tags are never moved after artifacts have been built from them.** If this gate finds a
  defect, the fix ships as a new tag (`v0.5.1` or later), never a retag of `v0.5.0`.
- **Publication digest closure happens AFTER Craig publishes**, as a separate, later, read-only
  pass: re-download every published asset into a new empty directory, hash, and compare against
  the staged digests recorded above (three-way: recorded-vs-published, staging-dir-rehash-vs-
  recorded, byte-for-byte) — same structure as prior stages' closures. Only after that closure
  PASSes is the release final. This gate brief's own verdict (from outline steps 1–5) remains
  **provisional** until the closure runs.

---

## Confinement rules (restated per checklist item 2)

- Writes only under `/my-files/GitRemotes/stage6-gate` and
  `/my-files/GitRemotes/stage6-gate-contract` (the contract table's live root — checklist item 8,
  named explicitly per Preconditions step 5) — plus the gate-authorized creation of
  `/my-files/GitRemotes` itself if absent, AND the symmetric authorization to trash
  `/my-files/GitRemotes` itself in cleanup once both children are gone (outline step 5) — a
  gate-created folder is a gate-owned folder, torn down same as its contents, same close every
  prior gate took.
- Untouchables (`GitBackups`, `Sensitive Project Sources`, `Project Repo Bundles`, `ChatGPT
  Export Text Backup`) read-only: rows in listings only, never named in a write command.
- Verify-before-trash, full subtree enumeration, every time (checklist item 3).
- **Report BLOCKED with verbatim output, never patch** (checklist item 4) — applies with extra
  force to the "Runner-asserted strings" section above: a mismatch there is not a "fix the
  message and continue" situation, it is a BLOCK.
- `-count=1` on the one `go test` invocation this brief calls for (checklist item 5).
- Every `git push` runs with a long tool timeout or in the background (checklist item 9).
- `GPB_LIVE_ACCOUNT` and `GPB_CONTRACT_LIVE_ROOT` are set only for outline step 3's one command,
  never left set across shells — verify both absent in a fresh shell afterward.
  `GPB_UNCERTIFIED_CLI` is never set at all in this gate (the draft is gated against the
  certified CLI only). `GPB_CREATE_PARENTS` is not exercised by this brief — Stage 6 adds only a
  three-line hermetic test for `EnsureParents`' mount-is-a-file branch (component 3's leftover
  item), no behaviour change, so no live provocation is called for here.
