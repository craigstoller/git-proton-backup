# git-proton-backup Repo Instructions

Keep this file short; it is loaded into agent context. Routing and hard guardrails only.

## Documentation

- Treat `docs/README.md` as the canonical documentation index.
- Put durable decisions in the v2 design doc's revision history or a gate record under `docs/research/gates/`; this repo does not use a decisions folder.
- Mark unfinished docs as `Placeholder` and state what evidence is missing.
- Do not turn generated briefs, speculative policies, or inferred architecture into source of truth without repo evidence or user confirmation.

## Repo Gotchas

- Proton CLI support is an exact-version allowlist, not a minimum version — see the pin in `docs/v2-remote-helper-design.md`. Do not "upgrade" it casually.
- `docs/releasing.md` is the canonical release procedure. Follow it in order; its steps exist because skipping them has already gone wrong once.
- `.superpowers/` and `.claude/` are untracked agent working files, not canonical docs. Never cite them as source of truth.
- Feature work uses dated spec/plan pairs under `docs/superpowers/`; read the spec before executing its plan.
