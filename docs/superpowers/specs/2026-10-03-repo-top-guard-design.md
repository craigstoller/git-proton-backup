# Wire a repository only by its own top folder — design

**Date:** 2026-10-03. **Issue:** [#10](https://github.com/craigstoller/git-proton-backup/issues/10).
**Scope:** the `GitProtonBackup` PowerShell module (v1 bundles) only; the v2 remote helper is
untouched. **Shape:** a bounded fix to existing commands, so this document carries its own task list
instead of a separate plan file (the 2026-09-05 digest-stamp spec set the precedent). Steps that
live in other repos or in Craig's hands are labelled *external*.

**Status (2026-10-03):** Tasks 1–5 implemented on branch `claude/nice-mclean-3f37e0`, test-first.
The Issue #10 Describe in `tests/Commands.Tests.ps1` holds 24 tests. 21 of them failed on the
code they target before it changed, for the intended reasons, in five stages: 10 against 0.8.0;
4 (D5) against the round-1 code; 2 (first-probe) against the round-2 code; 3 (D3's amendment and
gate, and the unnamed-repository wording) against the round-3 code; 2 (form-divergent git dirs,
a bare repository's worktree git dir) during the verification round. 10 + 4 + 2 + 3 + 2 = 21 —
counting tests, not failures: the 2 existing Verify tests whose expected wording D3's amendment
changed failed a second time then, and are not counted again. The other 3 are guards that passed
throughout (bare top, submodule top, a main checkout with a linked worktree). The full Pester suite (all four files) and
PSScriptAnalyzer with the repo settings ran clean on each committed tree — counts in the commit
messages. Task 6 is *external*.

Basis marks: **[sandbox]** = run on 2026-10-03 against `aa067cf` (v0.8.0) in an isolated state root
(`GPB_CONFIG_DIR`, `GPB_LOCK_PATH`, `GPB_HOOK_DISABLED=1`); **[code]** = read from
`GitProtonBackup.psm1` at `aa067cf`; **[inference]** = reasoned from the two, not run.

## Problem

Every git call the module makes for a repo is `git -C <RepoPath> …`, and git resolves `-C` to
whatever repository *contains* that folder. The module's own bookkeeping — registry entry, mirror
slug, bundle-directory slug, the mirror's `gpb.workrepo` — is keyed on the path string it was given.
When the path is not the repository's own root, the two disagree: the engine records one identity
and acts on another.

`Install-ProtonBackup` checks only `git rev-parse --git-dir`, which succeeds anywhere inside a
repository **[code]**. So a subfolder passes, and `Install-GpbMirror` then reads the *containing*
repository's `proton` remote, sees it pointing at a different mirror under the mirrors root, takes
the moved-repo branch, deletes the containing repository's own mirror, repoints its remote at a new
mirror named after the subfolder, pushes all its branches and tags there, and stamps the subfolder
into that mirror's `gpb.workrepo`. The registry gains the subfolder; the containing repository's
entry stays **[code, sandbox]**. `Uninstall-ProtonBackup` on a subfolder has the same reach in
reverse: `Remove-GpbMirror` removes the containing repository's `proton` remote, because its URL is
under the mirrors root **[code; issue repro step 4]**.

## Evidence beyond the issue

- The issue's repro reproduces exactly **[sandbox]**.
- In that state, `Invoke-ProtonBackupVerify` reports the containing repository as "wiring broken —
  run Repair-ProtonBackup", reports the subfolder entry `ok`, and cuts a second bundle directory
  under the subfolder's slug holding the containing repository's full history **[sandbox]**.
  `Get-ProtonBackupStatus` shows the subfolder entry `WiringOk = true` **[sandbox]**.
- Following Verify's advice — `Repair-ProtonBackup <containing repo>` — heals that repository and
  deletes the subfolder's mirror, after which Verify reports the *subfolder* entry as "wiring broken
  — run Repair-ProtonBackup"; on 0.8.0 that Repair re-runs the bug. The advice loops **[sandbox]**.
- `git rev-parse --show-prefix` is empty not only at the top of a work tree, a linked worktree and a
  submodule, but also inside a repository's `.git` folder and in any subfolder of a bare repository
  **[sandbox]**. On 0.8.0, `Install-ProtonBackup <repo>\.git` and `Install-ProtonBackup <bare>\refs`
  each rewire their repository the same way, deleting its own mirror **[sandbox]**. So the issue's
  rule as written ("`--show-prefix` prints nothing") does not close the class.
- `Install-ProtonBackup` on a bare repository's own folder succeeds today **[sandbox]**. The README
  neither promises nor excludes it.

## Decision

Rulings are **Craig's call, 2026-10-03**: he took the recommended default on each decision of the
brief in a single reply ("defaults"). The options not taken are kept with each.

### D1 — what counts as a repository's own root

A path is a repository's own root when either:

- it is inside a work tree and `git rev-parse --show-prefix` prints nothing (the top of a repo or
  a submodule; the top of a *linked* worktree passes this test but is excluded by D5); or
- the repository is bare and the path is its git directory itself (`git rev-parse --git-dir`
  prints `.`, and that directory is the repository's common dir — see below).

Any other path inside a repository is refused — a work-tree subfolder, a non-bare repository's
`.git` folder (or anything under it), a subfolder of a bare repository, including the private
git dir of one of its linked worktrees (`<bare>\worktrees\<wt>`). That last one passes the
`--git-dir` test alone — git says bare there and prints `--git-dir` as `.` — and was accepted as
a root until the verification round; only its common dir, the bare repository, tells them apart
**[sandbox]**. The `.git` case never
reaches the first bullet: inside `.git`, git answers `--is-inside-work-tree` with `false` (and
`--is-bare-repository` with `false`) although `--show-prefix` is empty **[sandbox]** — so it is
neither a work-tree root nor a bare root. A path inside no repository
at all is outside this rule (Install already refuses it as "not a git repository"). A folder that
has its own `.git` is its own repository's root wherever it sits — including inside another
repository's work tree, which is where following the refusal's "run 'git init'" advice leaves it.

This applies to `Install-ProtonBackup`, and to `Repair-ProtonBackup` through it. Refusal messages:

- work-tree subfolder (the issue's wording): `'<path>' is inside the repository at '<top>', not its
  top folder. Pass '<top>', or run 'git init' in '<path>' first.`
- inside a `.git` folder: `'<path>' is inside a repository's git directory, not its work tree. Pass
  the repository's top folder.`
- bare-repository subfolder: `'<path>' is inside the bare repository at '<gitdir>', not its top
  folder. Pass '<gitdir>'.`

Not taken: **(a)** the issue's rule as written — misses the `.git` and bare-subfolder cases above;
**(c)** accept only work-tree tops and refuse bare repositories outright — removes something that
works today and is not needed to close the bug. Also not taken, as the issue argued: **resolving
the path to its top and carrying on** — a caller who passed a subfolder most likely meant a
different repository, and silently wiring the containing one is the surprise this fix removes.

### D2 — `Uninstall-ProtonBackup` on an existing path that is not a root

Uninstall never touches the containing repository: it neither reads nor removes that repository's
`proton` remote. It still removes the path's **own** bookkeeping — registry entry, push-pending
marker, digest stamp (both locations), and the mirror at the path's own slug, under the existing
delete-safe rule (bare, and `gpb.workrepo` canonically equal to this path). It warns, naming the
containing repository, that the repository was left untouched. A path with no bookkeeping of its
own (a mistyped path) ends at the existing "nothing to uninstall" warning plus that one, having
changed nothing.

A path that no longer exists is unchanged: `git -C` on a missing folder fails, so no git state is
reached, exactly as today. That is the documented fix for a moved repository's old registry entry
and must keep working.

Not taken: **(a)** refuse, like Install (the issue's proposal) — a registered non-root entry, left by
the 0.8.0 bug or by a nested repository whose own `.git` was later removed, could then never be
deregistered: Repair refuses (D1), Uninstall refuses, and `Set-ProtonBackupConfig` will not edit
`Repos` **[code]**; only hand-editing `config.json` would clear it **[inference]**. **(c)** refuse
when the path is unregistered, behave as above when it is — more branches for little gain, since
the mistyped-path case above already changes nothing.

Recovery for anyone already hit by the bug: `Repair-ProtonBackup <containing repo>`, then
`Uninstall-ProtonBackup <subfolder>`. Repair repoints the repository at its own mirror and, through
the existing moved-repo branch, deletes the subfolder's mirror; Uninstall then removes the
subfolder's registration and marker. The other order ends in the same state, but in between the
repository's remote points at the subfolder mirror Uninstall just deleted, so `git push proton`
fails loudly until the Repair (Verify reports the wiring broken, which is the right advice there).
Recommending Repair first avoids that window. The mirror itself is disposable bookkeeping (README,
"How it works"); the history lives in the repository and its bundles, so nothing is lost either way.

### D3 — `Invoke-ProtonBackupVerify` recognises a registered non-root entry

For a registered path that exists but is not a repository's own root, Verify records a finding,
sets the entry to `attention`, and skips its wiring check and bundling step. Skipping the bundle
step stops *Verify's* duplicate bundles; skipping the wiring check stops the "run Repair" advice
that would now only meet D1's refusal.

**Amended after round 3 — Craig's call, 2026-10-03** (took the recommended default, third brief).
The finding prescribes the repository first and the deregistration second, the order the CHANGELOG
recommends; as first ruled it said only "Uninstall-ProtonBackup '<path>' to deregister it", which
for a subfolder-only setup (0.8.0 registered the subfolder, never its repository) ends the backup
[Codex, DeepSeek]. Wordings, the first clause shared with D1/D5:

- subfolder (bare and linked alike, with their own first clause): `registered path '<path>' is
  inside the repository at '<top>', not its top folder — Install-ProtonBackup '<top>' (it also
  repairs existing wiring), then Uninstall-ProtonBackup '<path>' to deregister it`
- `.git`, or any kind whose repository git did not name: `… — Repair-ProtonBackup the
  repository's top folder, then Uninstall-ProtonBackup '<path>' to deregister it`

**Also gated on a positive `root` — Craig's call, 2026-10-03, same brief.** For a registered path
that exists but for which git answers `none` (no repository there, or no answer), Verify records
`registered path '<path>': git found no repository there (or did not answer) — fix the path or
run 'git init' there and Install-ProtonBackup '<path>', or Uninstall-ProtonBackup '<path>' to
deregister it`, sets `attention`, and skips the wiring check and bundling step too. As first built,
`none` kept the normal pass, so a transient first-probe failure on a registered non-root path
bundled the containing repository under that path's name — permanently, since Uninstall never
deletes bundles — and in the 0.8.0 leftover state could even read `ok` [Codex, DeepSeek]. The cost,
accepted with the ruling: a momentary git failure on a healthy repository skips that repository's
bundle for one run, with an `attention` finding; the next run retries. This also closes the loop
round 1 had named out of scope (a registered folder whose `.git` was deleted was told "run Repair",
which then said "not a git repository").

Not taken for the amendment: keep the first-ruled wording. Not taken for the gate: keep the normal
pass on `none` — non-destructive, but a permanent duplicate bundle and a possible false `ok`.

`Get-ProtonBackupStatus` and the push hook are unchanged. Status is a read-only display, and in the
fresh-bug state it will keep showing the subfolder entry as wired until the user recovers (it shows
the containing repository's wiring as broken in that state **[sandbox]**); Verify is the alarm
surface. The hook must never fail a push, and after this fix no new mirror can carry a non-root
`gpb.workrepo`. The residual, stated: a mirror the 0.8.0 bug already made still carries the
subfolder as its `gpb.workrepo`, so until the user recovers, each push from the containing
repository still cuts a bundle of its history under the subfolder's slug, and any push-pending
marker that push leaves is reported by Verify's marker pass as a second finding for the subfolder.
Nothing is lost (the repository's history is what gets bundled), and both stop at recovery, which
the D3 finding prescribes.

Not taken: **(b)** leave Verify alone — users keep being sent to Repair, meet the refusal, and must
work out Uninstall themselves, while the duplicate bundles continue.

### D4 — release

Ship as **0.8.1**, a module-only patch: the helper is rebuilt only so its `--version` moves with the
tag, as in 0.8.0. Precedent: 0.2.1–0.2.3 were fix-only patch releases. It does refuse an input
0.8.0 accepted, deliberately: that input only ever produced a mis-keyed backup of a different
repository, and the CHANGELOG says so. This branch adds an `Unreleased` CHANGELOG entry only; the
`ModuleVersion` bump, tag, draft, live gate and publication follow `docs/releasing.md` in full, on
Craig's word (*external*). The pin
bump in `project-operating-standards` (`config/backup-engine.psd1`) follows the release as its own
step, on Craig's word (*external*); that repo already guards this case itself
(`Test-OwnRepository`, `scripts/hublib/HubDiscovery.ps1`, `285672c`) and needs no other change.

Not taken: **(b)** hold the fix for the next minor release with the Stage 7 work.

### D5 — linked worktrees

**Craig's call, 2026-10-03** (took the recommended default, second brief): refuse a linked
worktree's top folder too, naming the main checkout; the general guard (option c) goes on the
Stage 7 slate.

Found after D1–D4 were ruled, while writing the guard tests, and raised independently by two
reviewers in round 1: a linked worktree shares its repository's config, `proton` remote included.
On 0.8.0, installing a linked worktree's top folder while the main checkout was already wired
repointed the shared remote at a mirror named after the worktree and deleted the main checkout's own
mirror; the registry held both paths; Verify then reported the main checkout's wiring broken and
advised Repair **[sandbox]**. Each wired worktree would also carry a full copy of the shared history
in its own bundle folder **[inference]**. The downstream consumer already treats the main checkout
as the unit: `project-operating-standards` maps a worktree path to its main checkout before wiring,
citing this same collision (`scripts/hublib/HubDiscovery.ps1`) **[code, other repo]**, and the
README's Limits section already says a bundle carries every branch and tag of the repository.

So D1 narrows: the top of a linked worktree (its `--absolute-git-dir` differs from its
`--git-common-dir`) is not a root of its own. The helper reports it as kind `linked`, with the main
checkout as the repository it belongs to — the first entry of `git worktree list --porcelain`,
which is the bare directory itself for a worktree of a bare repository. That ordering is git's
documented behaviour, not just observed: the `git worktree` manual says the main worktree is
listed first **[git docs, git-worktree.html shipped with Git for Windows 2.55]**. Install's message:
`'<path>' is a linked worktree of the repository at '<main>'. Pass '<main>'; its bundles carry
every worktree's branches and tags.` Uninstall leaves the shared remote alone, as for every other
non-root (D2). Verify's finding names the step a worktree-only setup needs, in the order D3 (as
amended) uses for every kind: `registered path '<path>' is a linked worktree of the repository at
'<main>' — Install-ProtonBackup '<main>' (it also repairs existing wiring), then
Uninstall-ProtonBackup '<path>' to deregister it`. A *subfolder* of
a linked worktree keeps the `worktree` wording naming the worktree's top; installing that top then
names the main checkout (two refusals, each true).

The cost, accepted with the ruling: someone who wired only a linked worktree — which works on 0.8.0,
since the refs are shared — is refused on Repair and flagged by Verify, and moves to the main
checkout: Install the main checkout, then Uninstall the worktree. The other order works too but,
as in D2, leaves the shared remote pointing at the just-deleted worktree mirror in between, so the
CHANGELOG recommends Install first. Uninstall leaves the
shared remote alone, as D2 requires, so a user who uninstalls the worktree and never installs the
main checkout is left with a `proton` remote pointing at the deleted worktree mirror: `git push
proton` then fails loudly until they install the main checkout or remove the remote. That is the
D2 trade, stated: never touching a remote that may not be the path's own outranks tidying one.

Not taken: **(b)** keep accepting linked worktrees and track the collision separately — ships a
reproduced way to break a wired repository. **(c)** a general guard in Install's moved-repo branch
— refuse when the existing remote points at a mirror whose `gpb.workrepo` is a different path that
still exists (a live second identity, not a moved repository); it would also cover path aliases
(junctions, `subst`) and plausibly a copied repository **[inference]**, but it changes the branch
Repair relies on and needs its own design. **Deferred to the Stage 7 slate** rather than rejected.

### Smaller defaults (accepted in the same reply)

- The rulings are recorded here, with a Decision section, rather than in the v2 design doc's
  revision history or a gate record — AGENTS.md names those two for durable decisions, but neither
  fits a v1 module fix; the 2026-09-05 spec is the precedent. Peer-reviewed before implementation
  is presented as complete.
- README gains one sentence after the `Install-ProtonBackup C:\code\myrepo` example. Amended with
  D5 (Craig's call, 2026-10-03; the first wording was false for a submodule's top [DeepSeek]):
  "Pass the repository's own top folder (a submodule's counts); a plain folder inside a repository
  is refused rather than wired, and a linked worktree is refused in favour of its main checkout."
- The Uninstall warning reads "… — that repository was left untouched." rather than naming the
  `proton` remote (round 1 [Gemini]: the first wording read as an alarm on a mistyped path);
  Craig confirmed the change, 2026-10-03.
- Tests go in `tests/Commands.Tests.ps1` with its existing sandbox helpers, written red first.
- Commits stay local on the worktree branch; push and PR on Craig's word.

## Design

- **One helper, `Get-GpbRepoRole -RepoPath`.** Always returns a kind: `root` when the path is a
  repository's own root (D1, as narrowed by D5), `none` when git found no repository there or did
  not answer, and otherwise one of `worktree`, `linked`, `gitdir`, `bare` with the repository it
  belongs to
  (`--show-toplevel` for a work-tree subfolder, the first `git worktree list --porcelain` entry for
  a linked worktree, the common dir for a bare repository, `--absolute-git-dir` otherwise),
  normalised to a native path. The linked test — and, since the verification round, the bare test
  too — compares the directories git names for `--git-dir` and `--git-common-dir`, each resolved
  to an absolute path, not git's spelling of them: they are the same directory for a main checkout,
  a submodule and a bare repository's own top, and different only for a linked worktree's top or
  private git dir **[sandbox, git 2.55]**. Comparing spellings worked on this git (both print
  `.git` in a main checkout) but would refuse a real root wherever git printed one relative and
  the other absolute; a test now forces that divergence. It needs no `--path-format`, so no
  minimum git version beyond what the module already uses. (`--git-common-dir` dates from git 2.5 and `--absolute-git-dir`
  from 2.13; Install's existing `--is-shallow-repository` probe already needs 2.15.) A second helper formats the
  shared first sentence of every message ("`'<path>' is inside …`"), so Install, Uninstall and
  Verify cannot drift apart in wording. The test is made with git's own answers (`--show-prefix`,
  `--is-inside-work-tree`, `--is-bare-repository`, `--git-dir`), never by comparing the given path
  with a path git prints, so case, slash direction and 8.3 names cannot cause a false refusal
  **[sandbox for case and slashes; 8.3 names and junctions inference — git makes its own
  cwd-versus-top comparison for `--show-prefix`]** (the bare test does compare git's output to `.`, which is how git reports "this
  directory is the git directory"). Probe order: the linked test runs only once `--show-prefix`
  has come back empty, so a subfolder of a linked worktree is a `worktree`, not a `linked`.
  **Failure contract — positive classification:** `root` requires an affirmative answer at every
  step; a probe that fails anywhere yields a non-root kind or `none`, never `root`. That is a
  safety guarantee, not a diagnostic one: after a failure part-way through, the non-root kind
  reported can be the wrong one (a subfolder whose work-tree probe failed reads as `gitdir`), so
  the message may misname the case, but it can never permit a rewire. Every step that acts on a
  repository is gated on `root` alone: Install proceeds only on `root` (`none`, after its own "is
  a git repository" check passed, is a contradiction it refuses: "git did not confirm … nothing
  was changed; retry"); Uninstall probes and removes the `proton` remote only on `root` (on
  `none` it skips the remote, which a path git found no repository for cannot own, and says so);
  and Verify wiring-checks and bundles only on `root` (D3 as amended). When a later probe fails
  and git names no containing repository, messages describe it ("inside a repository", "Pass
  the repository's top folder") rather than print `''`. Messages carry the canonical path
  (Install's `Resolve-Path` at entry), not the caller's spelling.
- **Assumed, not changed:** the probes inherit the caller's environment, so a session with
  `GIT_DIR`/`GIT_WORK_TREE` set gets those repositories' answers — for these probes exactly as for
  every other git call the public commands already make. The push hook strips those variables; the
  public commands never have. Pre-existing and out of scope.
- **Install:** the check runs in `Install-ProtonBackup` right after the existing "not a git
  repository" check — before the shallow probe and the hazard warnings, both of which would
  otherwise describe the containing repository — and throws the D1 message. `Install-GpbMirror`
  is internal and its only production caller is `Install-ProtonBackup`, so it is not separately
  guarded.
- **Uninstall:** for an existing path, the helper runs once at entry, before the lock (it only
  reads); a path that no longer exists is `none` without asking git. Unless the answer is `root`,
  Uninstall skips the `hadRemote` probe (so no remote can count towards "Unwired.") and calls
  `Remove-GpbMirror` with a new `-SkipRemote` switch, under which it neither reads nor removes the
  `proton` remote and falls back to the slug mirror under its existing delete-safe rule — exactly
  what a deleted path already got, since its remote probe failed. It warns, naming the containing
  repository, for the non-root kinds; for an existing path git answered `none` for, it warns that
  no `proton` remote was touched. That makes the fail-closed side visible once, as a console
  warning, and no more: Uninstall still drops the registry entry, so afterwards nothing monitors
  that repository, and a real repository whose probe failed transiently keeps a remote pointing at
  its now-removed mirror until a re-run of Install or `git remote remove proton` — the only later
  signal being a failed `git push proton`. Accepted, rather than risk Uninstall stripping a remote
  that belongs to the repository around the path. git does not tell "no repository here"
  from "did not answer" except in localised error text, so the two are not told apart. The marker, stamp and registry steps after that are
  unchanged, and `didSomething` reflects only this path's own mirror and registry entry.
- **Verify:** two new branches between "registered repo missing on disk" and the normal pass —
  one for `none`, one for the non-root kinds — so only a `root` reaches the normal pass. Phases B,
  B2 and C already skip records without a bundle result (as they do for a missing repo), so none of
  them changes. The marker pass is not one of those phases: it works from marker files, not from
  the per-repo records, which is why a marker the hook left for a non-root path is still reported
  (the D3 residual).
- **Closed by D3's gate (was named out of scope in round 1):** a registered path that exists but
  is inside *no* repository (its `.git` removed, nothing around it) used to get "wiring broken —
  run Repair-ProtonBackup", and Repair then said "not a git repository". It now gets the `none`
  finding, which names the two ways out.

## Tests (TDD)

**RED** = fails on 0.8.0 for the reason the fix addresses. **GUARD** = passes on 0.8.0 and pins
what must not change. All in `tests/Commands.Tests.ps1`.

- RED: Install of a work-tree subfolder throws the D1 message naming the top; the containing
  repository's remote, its mirror and the registry are unchanged.
- RED: Repair of a work-tree subfolder throws the same, with the same unchanged-state assertions.
- RED: Install of `<repo>\.git` throws the git-directory message; the repository's wiring is
  unchanged.
- RED: Install of `<bare>\refs` throws the bare message naming the bare repository; its wiring is
  unchanged.
- GUARD: Install still accepts the top of a bare repository and of a submodule (whose superproject
  is not rewired), and the main checkout of a repository that has a linked worktree.
- RED (D5): Install of a linked worktree's top throws the `linked` message naming the main
  checkout, and the main checkout's wiring and the registry are unchanged; the same for a worktree
  of a bare repository, naming the bare directory.
- RED (D5): Uninstall of a linked worktree's top leaves the shared remote and the main checkout's
  mirror in place; Verify on a registered linked worktree reports the `linked` finding and does
  not bundle it.
- RED: Uninstall of an unregistered existing subfolder of a wired repository leaves that
  repository's remote and mirror in place and warns naming it.
- RED: Uninstall of `<repo>\.git` and of `<bare>\refs` leave their repository wired and warn with
  the matching D1 wording.
- RED: from the fabricated 0.8.0 leftover state (registry holds the subfolder; the containing
  repository's remote points at the subfolder's mirror, whose `gpb.workrepo` is the subfolder),
  `Uninstall-ProtonBackup <subfolder>` then `Repair-ProtonBackup <repo>` ends with one registry
  entry, the repository wired to its own mirror, and the subfolder's mirror gone.
- RED: the same recovery in the other order (Repair first, then Uninstall).
- RED (round 2): with only the first classification probe failing (a Pester mock of `git`
  scoped to that call), Install of a subfolder refuses with "git did not confirm" and rewires
  nothing, and Uninstall of it leaves the containing repository's remote in place.
- RED (D3 as amended): the Verify tests' expected findings name the Install step before the
  Uninstall (both failed against the first-ruled wording, then passed); Verify on a registered
  subfolder whose first probe fails (scoped mock) reports the `none` finding and bundles nothing;
  Verify on a registered folder with no repository at all reports the `none` finding, not "run
  Repair", and bundles nothing; an Install refusal for a subfolder whose `--show-toplevel` fails
  (scoped mock) describes the repository instead of printing `''`.
- RED (verification round): Install of a main checkout whose `--git-dir` git prints absolute
  while `--git-common-dir` stays relative (scoped mock) still accepts it as a root; Install of
  `<bare>\worktrees\<wt>`, a bare repository's worktree's private git dir, refuses naming the bare
  repository and leaves its wiring intact.
- RED: Verify on the leftover state reports the D3 finding for the subfolder entry, marks it
  `attention`, and creates no bundle directory under the subfolder's slug. One Verify test, not
  one per kind: the branch does not depend on the kind, and the kind's wording comes from the
  shared formatter the Install and Uninstall tests already pin.
- GUARD (existing): deleted-repo Uninstall by absolute and by relative path; relative-path Install;
  moved-repo Repair; foreign-remote Uninstall.

The leftover state is fabricated by hand (bare mirror at the subfolder's slug, `gpb.workrepo`,
`remote set-url`, registry entry), not by calling the internal `Install-GpbMirror` on the subfolder,
so the fixture stays valid if that function is ever guarded too.

## Docs

- `CHANGELOG.md`: an `Unreleased` entry — what was refused and why, the `.git`/bare cases, the
  linked-worktree refusal and the move for a worktree-only setup, Uninstall's new behaviour on a
  non-root path, Verify's new finding, and the two-command recovery (Repair first).
- `README.md`: after the `Install-ProtonBackup C:\code\myrepo` example, the sentence as amended
  under "Smaller defaults".
- `docs/design.md`: one qualifier in "Markers + the reconciliation backstop" — Verify's per-run
  recompute applies to registered repos that are present and that git confirms as a repository's
  own top folder; any other entry gets a finding instead. (Before D3's gate the unqualified
  sentence was already loose about the missing-on-disk skip; the gate made it wrong [DeepSeek,
  verification round].) The doc lists no Verify findings, and none of its mechanisms changes.

## Revisions

- **Round 1 (2026-10-03; panel: Codex, Gemini (agy), DeepSeek and Kimi (repo-aware) all
  reported).** The two repo-aware engines read the tree mid-session, after the tests and README
  sentence had landed and before the module change, and reported the docs as ahead of the code;
  that was the TDD order in progress (RED observed 10 failing on 0.8.0, then GREEN), not a stale
  document. Applied: D5 opened for the linked-worktree collision, reproduced in the sandbox before
  the round and raised independently [Codex, DeepSeek]; probe-failure contract made explicit and
  fail-closed in code [Codex, Gemini]; recovery recommends Repair first, with the in-between
  window stated [Codex, Gemini]; D3 states the pre-existing-mirror residual (hook bundles and a
  marker finding until recovery) instead of claiming one finding [Kimi, DeepSeek]; Uninstall tests
  for the `.git` and bare-subfolder kinds, and the Repair test's unchanged-state assertions listed
  [Codex]; Uninstall's `hadRemote`/`didSomething` delta and pre-lock placement stated [DeepSeek];
  D4 says the release refuses a previously accepted input and runs the full release procedure
  [Kimi, DeepSeek]; `GIT_*` environment named as an assumption, and the inside-no-repository loop
  named out of scope [DeepSeek]; the Uninstall warning reworded to "that repository was left
  untouched" so a mistyped path does not read as an alarm [Gemini]. Rejected: "a `$null` result
  makes Uninstall throw on a non-repository" — its git calls redirect errors and check exit codes,
  and the deleted-repo tests exercise exactly that path [Gemini]; "Status hides data loss" —
  nothing is lost in the leftover state (the repository's history is bundled, under the wrong
  name) and Status shows the containing repository's wiring broken [Gemini]; "archive the deleted
  mirror during recovery" — mirrors are disposable bookkeeping [Codex]; "the `git init` advice
  steers users into a coverage gap" — once initialised and wired, the folder gets bundles of its
  own, and the parent's bundles never carried untracked content [Kimi]; "the foreign-remote
  Uninstall guard does not exist" — it does, in the Issue #2 Describe [Kimi; DeepSeek confirmed];
  "the `.git` message should name a top" — inside a linked worktree's private git directory there
  is no single work tree to name [Gemini]; registry path aliasing — pre-existing, and canonicalised
  at entry by Install and Uninstall alike [Gemini, Codex]. Put to Craig: D5, and the README
  sentence's wording [DeepSeek: as written it is false for a submodule's top].
- **Between rounds 1 and 2 (2026-10-03).** Craig ruled D5 (refuse a linked worktree, naming its
  main checkout; option c to the Stage 7 slate), approved the amended README sentence and confirmed
  the reworded Uninstall warning. D5 implemented test-first: four new tests failed on the round-1
  code for the intended reasons, then passed, with a new guard for a main checkout that has a
  linked worktree. Verify's linked-worktree finding also names the Install a worktree-only setup
  needs, so deregistering never silently ends a backup.
- **Round 2 (2026-10-03; panel: Codex, Gemini (agy), DeepSeek and Kimi (repo-aware) all
  reported; Codex, Gemini and Kimi said "Blockers: none").** Applied: round 1's failure-contract
  fix was itself defective — a failing *first* probe returned the same `$null` as a root, so
  Install could still rewire and Uninstall still strip the containing remote [Codex, DeepSeek;
  DeepSeek read the module minutes before the correction landed]. The helper became
  `Get-GpbRepoRole`, which always answers a kind, and every destructive step is gated on a
  positive `root`; two new tests make only that probe fail through a scoped `git` mock (both
  failed on the round-1 code, then passed). Also applied: the sandbox answers that keep `.git`
  out of D1's first bullet, plus a `.git\refs` assertion [DeepSeek]; probe order stated (the
  linked test runs only after an empty prefix) [Gemini]; the marker pass named as working from
  marker files, which is why the D3 residual exists [Gemini]; a nested repository with its own
  `.git` stated to be its own root [Kimi]; the worktree-only move recommends Install first, and
  the dangling shared remote after an Uninstall-only move is stated as the D2 trade [Gemini,
  DeepSeek]; a Status line recording what has landed and the gate results [Kimi]; a duplicated
  `Test-Path` in Verify removed [Kimi]. Rejected: "the recovery's second command crashes on an
  already-deleted mirror" — `Remove-GpbMirror` returns early on a missing mirror and the
  moved-repo branch tests the path before deleting, and both recovery-order tests pass [Gemini];
  "`docs/design.md` must gain the D3 finding" — it lists no Verify findings at all, and none of
  the mechanisms it describes changes [DeepSeek]. Noted, not changed: round 1's registry-aliasing
  rejection covers registry identity only; deleting a live mirror through a second path to the
  same repository (junction, `subst`, a copy) is exactly the Stage 7 option (c) guard [DeepSeek].
- **Round 3 (2026-10-03; panel: Codex, Gemini (agy) and Kimi (repo-aware) reported — Codex, Kimi
  "Blockers: none"; DeepSeek (repo-aware) INCOMPLETE on its first attempt, its synthesis turn cut
  off by the run's clock with no review text, then reported on its single re-run, "No
  blockers").** Two small doc notes
  landed after the round's baseline, so the later readers may have seen them: the git 2.15
  floor and the Uninstall warning below. Applied: when git answers `none` for an existing path,
  Uninstall now says so ("no 'proton' remote was touched"), making the fail-closed side visible
  rather than silent; a test asserts it, red first [Gemini]; the failure contract narrowed to
  what it guarantees — safety, not which non-root kind is reported after a mid-sequence failure
  [Kimi]; D5's first-entry rule cited to git's own manual [Kimi: flagged as observed only].
  Rejected: "the new probes raise the module's git floor" — `--git-common-dir` (2.5) and
  `--absolute-git-dir` (2.13) are older than the `--is-shallow-repository` (2.15) Install already
  needs [Gemini]; "`extensions.worktreeConfig` makes per-worktree remotes safe" — Install adds
  the remote with `git remote add`, which writes the shared config either way [Gemini]. Also
  applied from DeepSeek's re-run: the Status line now gives the staged red counts instead of
  "19 failed", since the 3 guards never failed; the path-comparison claim is labelled by basis.
  Rejected: "`docs/README.md` needs a line for this spec" — the index covers specs by folder,
  not one by one [DeepSeek]. Put to Craig, both raised by Codex and DeepSeek independently:
  **(A)** Verify's D3 finding prescribes Uninstall alone — for a subfolder-only setup that ends
  the backup (the D5 finding already names the Install step), and in every case it gives the
  order the CHANGELOG recommends against (Uninstall before Repair/Install); **(B)** Verify keeps
  its normal pass on `none`, so a transient first-probe failure on a registered non-root path
  bundles the containing repository under that path's name — and in the 0.8.0 leftover state
  can even read `ok` — instead of gating on a positive `root` as Install and Uninstall do.
- **After round 3 (2026-10-03).** Craig ruled both (recommended defaults): (A) D3's finding
  amended to name the repository's Install (or, where git names none, a Repair of its top
  folder) before the Uninstall, for every kind; (B) Verify gated on a positive `root`, with a
  `none` finding of its own. Also folded in, as recommended in the same brief: a message whose
  repository git did not name describes it instead of printing `''`. Implemented test-first:
  three new tests and the two Verify tests whose expected wording changed failed against the
  round-3 code, then passed. B closes the inside-no-repository loop that round 1 had named out
  of scope. Each ruling's text and the options not taken are under D3.
- **Verification round (2026-10-03, after the A/B rulings; panel: Codex and Gemini (agy)
  reported, Codex "Blockers: none"; Kimi (repo-aware) reported, "Blockers: none"; DeepSeek
  (repo-aware) INCOMPLETE — a whole review, `finish_reason=stop`, ending without the run's canary
  token — its findings checked by hand).** Applied: a bare repository's worktree's private git
  dir (`<bare>\worktrees\<wt>`) classified as a root and would have been wired — reproduced in
  the sandbox, then fixed with the common-dir test and a red-first test [DeepSeek]; the linked
  (and now bare) test compares directories, not git's spelling of them, after a test forcing
  one absolute and one relative spelling refused a real main checkout [Kimi]; `docs/design.md`'s
  per-run-recompute sentence qualified [DeepSeek]; the Uninstall fail-closed warning described as
  the one-shot signal it is [DeepSeek]; the Status ledger made to add up [Kimi, DeepSeek]; the
  CHANGELOG names Install's "git did not confirm" refusal [Kimi]. Rejected, with evidence: "a
  junction or other alias spelling of a repository's top is now refused" — a junction to a top
  classifies as `root` **[sandbox]** [DeepSeek]; "non-ASCII worktree paths come back C-quoted
  from `worktree list --porcelain`" — printed unquoted, main checkout named correctly **[sandbox,
  git 2.55]** [DeepSeek]; "Repair rejects an unregistered top folder, so the `.git` finding's
  Repair step fails" — `Repair-ProtonBackup` is `Install-ProtonBackup`, which registers [code]
  [Gemini]; "Status's `EMPTY` digest sentinel can collide with a stamp" — pre-existing and needs a
  hand-made stamp [Kimi]. Noted, not changed: a submodule checked out inside a linked worktree
  is a `root`; whether its git dir is shared with the main checkout's copy of that submodule is
  unverified and outside #10 [DeepSeek, inference]. No further round: the loop's three rounds and
  this verification round are spent; the two code changes it produced are each pinned by a test
  that failed first.

## Tasks

1. RED: the Install/Repair tests (subfolder, `.git`, bare subfolder) and the GUARD acceptance
   tests; GREEN: `Get-GpbRepoRole`, the message helper, the Install check.
2. RED: the Uninstall tests (unregistered subfolder, both recovery orders); GREEN: Uninstall's
   entry check and `Remove-GpbMirror -SkipRemote`.
3. RED: the Verify test; GREEN: Verify's non-root branch.
4. Docs: CHANGELOG `Unreleased` entry, README sentence.
5. Full Pester run (all four suites) and PSScriptAnalyzer with the repo settings; commit.
6. *External.* Release 0.8.1 per `docs/releasing.md` on Craig's word; then the pos pin bump, on
   Craig's word.
