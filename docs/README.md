# git-proton-backup Docs

**Living reference. Last updated: 2026-10-29.** Update this index when a doc is added, renamed, retired, or superseded — not on a review calendar.

This repo ships two tools that share one version line; each has its own design doc.

## Designs (the "why")

- [design.md](design.md) - v1: why bundles instead of a synced live repo. The design behind the `GitProtonBackup` module.
- [v2-remote-helper-design.md](v2-remote-helper-design.md) - v2: the `git-remote-proton` remote helper. A living design with a strict revision history (one mechanism per revision); read the prior-art survey first — the design assumes it.

## Operating

- [releasing.md](releasing.md) - The canonical release procedure for both tools. Follow it in order; its steps exist because skipping them has already gone wrong once.
- [../scripts/Test-ProtonCliDrift.ps1](../scripts/Test-ProtonCliDrift.ps1) - The Proton CLI drift check `releasing.md` calls for before tagging, before publishing, and weekly between releases: the installed CLI against Proton's feed, its identity token against the helper's certified token, the feed's version against the pin, and the feed's `linux/x64` and `macos/arm64` SHA-512s against the values it records.

## Process And Provenance

- [how-it-was-built.md](how-it-was-built.md) - The build process, including the parts it got wrong before it got them right.

## Research And Evidence

- [research/remote-helper-prior-art.md](research/remote-helper-prior-art.md) - Prior-art survey with a dated verification-status block and pinned sources.
- [research/gates/](research/gates/) - Stage gate briefs (what will be tested) and gate records (what happened). [brief-checklist.md](research/gates/brief-checklist.md) holds the standing rules every gate brief incorporates by reference.
- [research/probes/](research/probes/) - Runnable probes against the real Proton CLI, and local probes of git, APFS and Go behaviour, with their pinned results.
- [research/probes/macos-port/README.md](research/probes/macos-port/README.md) - The macOS port's local probes (no network, no Proton account): which file shows which finding (M12–M29, PM1–PM9), and what each file does not show.
- [research/gates/v0.11.0-macos-gate-brief.md](research/gates/v0.11.0-macos-gate-brief.md) - The brief for the macOS port's one live gate, run once, for v0.11.0, the release that carries the port. Not yet run.
- [research/gates/v0.11.0-linux-gate-brief.md](research/gates/v0.11.0-linux-gate-brief.md) - The brief for v0.11.0's Linux live gate, written from v0.10.0's and its record; it runs first and installs the draft Windows helper. Not yet run.
- [research/gates/v0.11.0-windows-gate-brief.md](research/gates/v0.11.0-windows-gate-brief.md) - The brief for v0.11.0's Windows live gate, written from v0.10.0's and its record; it runs second, asserts the draft helper's install, and adopts the release. Not yet run.
- [research/gates/v0.11.0-composition-refusal-steps.md](research/gates/v0.11.0-composition-refusal-steps.md) - The steps the same release's Windows and Linux gate briefs carry: an NFD create refused on each platform's own binary.

## Specs And Plans

Feature work is specified and planned in dated pairs under [superpowers/](superpowers/): `YYYY-MM-DD-<slug>-design.md` (spec) in `specs/`, `YYYY-MM-DD-<slug>.md` (plan) in `plans/`; the plan links its spec. Superseded planning docs stay in place with a `Status: Superseded` line naming their successor — see [v2-stage2-plan.md](v2-stage2-plan.md).

- [superpowers/specs/2026-10-15-v2-linux-port-design.md](superpowers/specs/2026-10-15-v2-linux-port-design.md) - The Linux port of `git-remote-proton`: the findings behind it (F1–F14), the case-variant ref rule, the install and gate design, and Craig's rulings D1–D8. Approved 2026-10-16; its design record is v6.9 of the v2 design doc.
- [superpowers/plans/2026-10-16-v2-linux-port.md](superpowers/plans/2026-10-16-v2-linux-port.md) - The 12-task implementation plan for that spec.
- [superpowers/specs/2026-10-27-v2-macos-port-design.md](superpowers/specs/2026-10-27-v2-macos-port-design.md) - The macOS port of `git-remote-proton`: the findings behind it (M0–M29), the canonical-equivalence rule, the Mac refusals and the repair, scheduled use, the one live macOS gate, and Craig's rulings MD1–MD8 and O1–O6. Approved 2026-10-27; its design record is v6.10 of the v2 design doc.
- [superpowers/plans/2026-10-27-v2-macos-port.md](superpowers/plans/2026-10-27-v2-macos-port.md) - The 15-task implementation plan for that spec, with its refinements R1–R6, and its execution notes, which add R7 and record every ruling made while it ran.

## Conventions

- Durable decisions live in the v2 design doc's revision history and the gate records; this repo does not use a separate decisions folder.
- `.superpowers/` and `.claude/` hold untracked agent working files; they are not canonical documentation.
- Everything durable should be reachable from this index — add a link when you add a doc.

<!-- repo-docs-bootstrap:index:start -->
<!-- generated by repo-docs-bootstrap - edits between these markers are overwritten -->

## Installed Docs

(No template files installed yet.)

<!-- repo-docs-bootstrap:index:end -->
