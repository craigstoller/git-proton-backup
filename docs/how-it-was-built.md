# How this was built

This tool wasn't written free-hand. It went through a spec, an adversarial review of that spec by
two independent models, a task-by-task build with a review pass after every task, and it inherited
one production incident's scar tissue from the private tool it was extracted from. This page is the
honest version of that process — including the parts it got wrong before it got them right.

## Spec first, reviewed twice before a line of code

Both design documents behind this tool — the original private module's design, and the extraction
design for this public repo — carry the same footnote: "revised same day after Codex + Gemini peer
review." That's not decoration. A design doesn't move to implementation here until two independently
reasoning models have taken a pass at it looking for exactly the kind of thing a single author,
reading their own plan, tends not to see.

Two catches from that pre-build review, both real, both design-changing:

- **A lock-protocol flaw.** The original design had the daily reconciliation job give up immediately
  if its lock was already held. Combine that with a push hook that takes the same lock briefly to
  cut a bundle, and the failure mode is obvious in hindsight: a push landing at the wrong moment
  could make an entire day's scheduled backup pass silently do nothing. The fix was a bounded retry
  instead of an immediate bail — the reconciliation job now waits out a brief hold rather than
  walking away from it.
- **A collision-ordering hole.** Bundles were originally named by repo and timestamp alone. Review
  traced through what happens when two bundles land in the same second: a slow upload could let the
  verifier confirm a *stale* file sitting at a filename a newer bundle was about to overwrite,
  blessing content that was never actually uploaded as current. The fix is why every bundle filename
  now carries a fragment of its own content digest — two different pushes can never collide on a
  name, so there's nothing left for that race to confuse.

Neither of those was a bug in running code. They were holes in a plan, caught while the plan was
still just a document — which is the cheapest possible place to catch them.

## Day one, production: the bug 175 sandboxed tests missed

The private tool's test suite was thorough — 175 Pester tests covering digest logic, publication,
retention, locking, and the push flow itself, all green before the first real install. None of them
caught what happened on the first real production push.

`git` runs a `post-receive` hook with `GIT_DIR` (and several related variables) already set in the
hook's environment, pointing at the bare repo the hook is running inside — not the working repo the
hook needed to operate on. Every `git -C <workrepo>` call the hook script made inherited that
environment and silently ran against the wrong repository instead. The bundle step failed quietly on
every single push, the first day this ran against a real repo outside a test harness.

The sandboxed suite never caught it because none of the 175 tests actually spawned a real,
git-invoked hook process — they called the hook's logic directly, in-process, which never carries
the environment git itself sets. The gap wasn't in what the tests checked; it was in what kind of
process they were checking. Diagnosed and fixed the same day, by scrubbing the inherited git
environment variables at the top of the hook before anything else runs — and it became a permanent
regression test that spawns a real shim through a real git push, specifically so this class of bug
can never hide behind a mocked hook call again. This tool ports that fix and that regression test
unchanged.

## Building this port: three more catches, one per task

This repo's own build followed the same task-by-task discipline: implement one piece, then have it
adversarially reviewed before starting the next. Three real defects were caught and fixed that way,
each turned into its own test in the same task it surfaced in.

- **Deleted-repo uninstall identity.** Path canonicalization tried to resolve the repo path to an
  absolute form and, when that resolution failed, silently fell back to using the raw, as-typed
  string instead. That fallback triggers exactly when a repo no longer exists on disk — which is
  precisely the case uninstalling a deleted repo needs to handle, since nothing else can key its
  mirror, marker, and registry entry to the identity install originally stored. The exact repo you'd
  want to uninstall *because* it's gone couldn't be found by its own bookkeeping. Fixed by
  normalizing the path the same way whether or not it still resolves on disk, so a deleted repo's
  identity always matches the one recorded while it still existed.
- **A lost-update lock window.** An early version of the guided first-run setup read the existing
  configuration before acquiring the write lock, then did several seconds of network and filesystem
  probing, then wrote. Any configuration change landing during that probing window would get quietly
  discarded by the stale read that started before it. Fixed by moving the single configuration
  read-modify-write to happen entirely inside the lock, with no configuration access anywhere
  outside it.
- **A list-vs-table formatting contract break.** The status command's result object grew past
  PowerShell's roughly four-property threshold for its default table formatter, so the promised
  "table by default" output silently became a vertical list per repo instead — technically correct
  data, wrong shape. Fixed with an explicit curated-column table format, with the full object still
  available for anyone piping into something else.

## Why releases are strict: there is a downstream that pins exact versions

This tool has a private downstream consumer — a personal backup system that runs it across a dozen
repositories and imports it with `-RequiredVersion` against an exact pin, not a floor. That single
fact shapes the release discipline here, so it is worth stating plainly rather than leaving as
unexplained fussiness.

Every behaviour change gets a version bump and a CHANGELOG entry, because something out there
declares a version and will get exactly what it asks for. And the pin is *exact* rather than a
minimum, because a floor silently admits whatever ships next.

That is not hypothetical. Version 0.2.4 exists because of it.

The Proton Drive CLI moved from 0.4.6 to 0.7.0 and dropped the `{ok, value}` wrapper around
`activeRevision` in `filesystem info --json`. Before the change, upload confirmation read
`activeRevision.value.state`; after it, the field lives at `activeRevision.state`. Reading only the
wrapped form meant **every healthy bundle reported as unconfirmed** — fleet-wide, with no error,
because the code failed closed exactly as designed.

The misdiagnosis was worse than the break. With the state parse failing, control fell through to
auth detection, which matched the bare substring `auth` — and the *success* payload contains
`keyAuthor`, `nameAuthor` and `contentAuthor`. So a healthy bundle on a valid session reported
`auth_error`, and a user would have gone off to re-authenticate a session that was never broken.

Both are fixed: the parser accepts either payload shape, and auth detection now requires a non-zero
exit code and uses word boundaries. Five regression tests cover both shapes, the `keyAuthor` false
positive, a genuine auth failure, and the `ok: false` case.

Two things generalise from that:

- **A dependency that silently changes an output shape is more dangerous than one that breaks
  loudly.** Nothing crashed. Exit codes stayed 0. The only symptom was a fleet reporting work it had
  actually done as not done.
- **A version floor would have admitted 0.7.0 automatically.** The exact-version allowlist in the v2
  design is not caution for its own sake — it is this incident, written down as a rule.

## The takeaway

None of these were caught by the person who wrote them. The lock and collision issues were caught by
models reviewing a plan before it became code. The production bug was caught by real use exposing a
gap no test happened to be shaped to find. The three build-time catches were caught by a review pass
that ran on every single task, not just the ones that felt risky. The common thread isn't that the
author is careless — it's that a second, independent, adversarial look catches an entire category of
mistake that looking at your own work one more time does not. That's the actual argument for the
process this tool was built with: not that it makes the author better, but that it catches what the
author, by construction, can't see.
