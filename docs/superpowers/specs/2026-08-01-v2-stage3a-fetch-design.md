# git-remote-proton Stage 3a — `git fetch` and `git clone`

**Status:** design, 2026-08-01. Scoped in conversation, not yet implemented.
**Extends:** [`docs/v2-remote-helper-design.md`](../../v2-remote-helper-design.md) at v6.2 — normative, settled across six peer-review rounds. This document does not restate it; it records where Stage 3a **narrows**, **defers**, or **is more specific than** that design, and why.
**Depends on:** Stage 2 (`e0a99c0`) and Stage 2.1 — a working `git push`, the per-member `Refused` reconciliation, and the pack invariants.
**Transport contract:** [`docs/research/probes/stage1-results.json`](../../research/probes/stage1-results.json), normative, certified on `cli-drive@0.7.0` only.

---

## Goal

`git clone -o proton-v2 proton::/my-files/…` produces a working checkout, and `git fetch proton-v2` updates one.

## The split, and why "Stage 3" became two

The parent design frames Stage 3 as "a real `git clone` and `git fetch`", ending at a working checkout. Working through it, **clone is not separable from fetch**: `git clone` is `list` → `fetch` → checkout, so a stage that advertises refs with a `HEAD` symref and implements `fetch` delivers clone with nothing left over. The genuine seam is not capability but **efficiency**:

- **Stage 3a (this document)** — fetch and clone both work. Downloads **every** pack, verifies each, consolidates the closure into one installed pack, verifies connectivity. Correct for any v2 repo by construction, because every pack this helper writes is non-thin and self-contained, so the union of all packs is a superset of any closure.
- **Stage 3b** — fetch only what is needed. The `.idx` cache, the object-to-pack map, iterative discovery, and the no-progress termination rule. A pure optimisation, landing against a baseline already proven live.

**Why this ordering rather than building discovery first.** Every *new* transport surface appears in 3a — bulk download, listing a populated `packs/`, large-file streaming. Stage 2 found three fake/real divergences, all in transport, none visible to the deterministic suite. Meeting that surface with a small blast radius beats meeting it inside a discovery loop, where a failure is ambiguous between the loop and the transport under it. 3b then diffs against a known-good reference: a loop bug reads as "fetched too little", which is a comparison, not a mystery.

**The cost, stated plainly.** A 3a fetch transfers the whole repository every time, regardless of what changed. That is acceptable only because these repos are small today and because 3b removes it. It is not a shipping end state.

## Decisions taken here

| Decision | Choice | Consequence |
|---|---|---|
| Stage scope | Split 3a/3b as above | Full user-visible capability one stage earlier |
| 3a fetch strategy | Download every pack | No discovery loop, no termination rule, no object-to-pack map in 3a |
| Live contract tests | Opt-in `GPB_LIVE_ACCOUNT=1`, confined root, **loud** skip | CI can never touch the account |
| `HEAD` on push | Write on first push **and backfill when absent**; never rewrite | Repos pushed before 3a stay clonable |

### `HEAD` backfill is a completion, not a rewrite

The parent design writes `HEAD` only when initialising, and says it "is never rewritten implicitly afterwards". Stage 3a keeps the second half exactly — an existing `HEAD` is never touched, so changing a default branch remains an explicit operation out of scope for v2 — and extends the first: **a push that finds `HEAD` absent writes it**, by the same deterministic rules.

This is consistent with the design rather than a deviation from it. That document already rules that "a folder with a valid marker but missing `refs/` or `packs/` is a partial initialisation and is completed, not rejected." A missing `HEAD` is the same condition.

**What actually happens without it, stated accurately** — an earlier draft of this section said such a repository would be "permanently unclonable", and that is wrong. Both review engines independently corrected it. `git clone` of a headless remote **succeeds**: it downloads the objects, sets up `.git`, then skips the checkout and warns that remote `HEAD` cannot be checked out, leaving an empty working tree. The user recovers with `git checkout <branch>` or by cloning with `-b`.

So the cost is a broken-looking clone rather than an impossible one. That is still worth fixing — this stage's gate is *a working checkout*, and "clone appears to succeed and leaves you with nothing" is a bad first experience — but it is a usability failure, not data loss, and the spec should not have claimed otherwise.

---

## Components

Six units, each with one responsibility.

### 1. `internal/transport/contract_test.go` — the shared contract table

The Stage 2 review's named systemic fix. The `Transport` interface documented behaviour but nothing enforced it, so `*Fake` and `*CLI` were free to drift and only real code discovered it — three times.

One table of scenarios written against the interface, run by two harnesses. `*Fake` always. `*CLI` only when `GPB_LIVE_ACCOUNT=1` is set, confining every write to a single root it creates and trashes itself.

**The skip must be loud** — naming what was skipped and how to enable it. Stage 2.1 had a `t.Skipf` guard rot into a no-op; a silent skip in the table that exists to prevent silent drift would be self-defeating.

Seeded with the three divergences that actually bit — C11 upload-names-by-local-basename, `ReadTo`'s folder destination, `List` seeing `EnsureDir`'d empty directories — plus the contract findings nothing currently pins across *both* implementations: `Trash` on a missing target (C4), `CreateExclusive`'s refusal (A6), the identical-content skip (C2).

### 2. `internal/repo/head.go` — `HEAD`

- `DeriveHEAD(candidates []string, clientHEAD string) (string, bool)` — a **pure function** holding the deterministic rules, testable with no transport: one candidate wins outright; among several, the one matching the client's own `HEAD` if present, else lexicographically first, so the result never depends on batch ordering; no branch candidates means nothing is written and the repo stays headless.

  **`candidates` is every branch on the remote after this push, not just the ones this push published.** On a first push those are the same set. On a **backfill** they are not, and taking only the published set would be wrong: a repo whose `main` was pushed last week, receiving a push of `bugfix` today, would get `HEAD` pointed at `bugfix`. The candidate set is therefore the post-push ref listing filtered to `refs/heads/`, intersected with branches whose writes actually succeeded plus those already present — selecting a branch that failed to publish would leave `HEAD` dangling.
- `WriteHEAD(t, root, branch)` — writes `ref: refs/heads/<branch>\n`, staged under the leaf name `HEAD`, read-back verified.

  **It cannot reuse `WriteRef`.** That function validates its payload against `^[0-9a-f]{40}$` and refuses anything else, which is correct for a ref file and fatal for a symref. `WriteHEAD` needs its own write-and-verify path with a symref-shaped validator, sharing `stagedFile` but not `WriteRef`.
- `ReadHEAD(t, root)` — serves the advertisement.

The client's own `HEAD` comes from `git symbolic-ref HEAD` locally; git never sends it to a helper. Push wires this in **after** refs are published and only from branches that actually succeeded — selecting a branch that then failed would leave `HEAD` dangling.

### 3. `internal/repo/fetch.go` — orchestration

`Fetch(t, root, gitDir string, wants []string) (keepPath string, err error)`:

1. List `packs/`; download every `.pack`/`.idx` pair into `<tmp>/objects/pack/`.
2. Verify each pair, **per member — the two are not checked the same way**:
   - the **`.pack`** by recomputing its content checksum against its basename, and
   - the **pair** by `index-pack --verify`.

   **The `.idx` is never checksummed against its basename.** Its basename is the *pack's* checksum, not its own, so that comparison is not merely wrong but guaranteed to fail — it would abort every fetch. This mirrors exactly the per-member asymmetry Stage 2.1 established on the push side in `publishPack`/`publishIdx`, and reuses those helpers. Getting it wrong here would re-break the single most peer-reviewed decision in the parent design.
3. `rev-list --objects <wants> --not --all` against the temp alternate.
4. Verify connectivity, rooted at the exact wants. **Before installing anything.**
5. `pack-objects` the result; install `pack-<hash>.{pack,idx,keep}`.
6. Return the `.keep` path for the caller's `lock` response.

**`--not --all` is load-bearing and easy to lose.** Without it, an incremental fetch consolidates the entire history into a fresh pack and installs it, silently doubling local disk every time. It works perfectly on a two-commit test repo.

### 4. `internal/gitcmd` additions

`SymbolicRef`; a `rev-list` wrapper accepting an alternate object directory; and a connectivity check that is **missing-object-fatal rooted at the wants**, not a generic `fsck` — the wants are not yet referenced by any ref, so `fsck` would not reach them.

The check is a `rev-list` traversal with `GIT_ALTERNATE_OBJECT_DIRECTORIES` pointed at `<tmp>/objects`, the want OIDs fed in explicitly rather than discovered from refs, `--not --all` to stop at what the repository already has, and a zero exit as the pass condition. Both engines confirmed refs are unnecessary for this — an explicit OID root is enough.

**The exact flag set is deliberately not pinned here.** One reviewer proposed a specific invocation; I have not run it against the installed git, and a design document asserting an unverified command line is how a plan inherits a wrong one. The first implementation task pins it with a test that fails when an object is genuinely missing — which also proves the check can fail, the standard this project applies to every guard.

### 5. `cmd/git-remote-proton/main.go`

Advertise `fetch`; handle plain `list`; buffer the blank-line-terminated `fetch` batch; emit the single `lock <keep-path>`, then **`connectivity-ok` if the option was accepted**, then the terminating blank line. Git retains only the **first** `lock`, which is why the closure is consolidated into one pack.

**`check-connectivity` is a capability as well as an option, and both halves are needed.** It must appear in the advertised capability list — git recognises `connectivity-ok` only from a helper that advertised it. Then git sends `option check-connectivity true`, whose boolean must be *tracked*, and `connectivity-ok` is emitted only when it was `true` **and** the check passed. Today the option handler replies `unsupported` to everything, so this needs changing.

Being precise about the stakes, because an earlier draft of this section overstated them: replying `unsupported` does **not** break the fetch. Git falls back to running its own connectivity check. The reason to do this properly is that the parent design puts closure ownership on the helper, and a helper that silently leaves the check to git is not honouring the contract it claims — not that anything visibly fails.

`check-connectivity` is **not** a poison option and must not be added to that list. The poison flag exists for options git *ignores our rejection of*; this is the opposite case, where git honours the answer.

### 6. The temp object directory

Must carry git's own layout — `<tmp>/objects/pack/pack-<hash>.{pack,idx}`. Dropped flat, git silently does not see the packs, `rev-list` reports every object missing, and the failure presents as a corrupt index mapping. Normative, not an implementation detail.

---

## Two boundaries

**The fetch path is strictly read-only on the remote.** It never calls `Bootstrap`. A missing or unrecognised marker is a hard refusal, not something to initialise — you must not be able to create a repository by fetching from one.

**Plain `list` and `fetch` do not take the lock.** The design's acquire-before-advertise rule is written for `list for-push`, and exists to stop a stale view overwriting a concurrent writer. A fetch writes nothing to the remote, so a lock buys no safety and costs real harm: with no takeover in v2, a crashed fetch would wedge the repository for everyone until a human cleared it by hand, and it would serialise concurrent readers for nothing.

---

## Error handling

**Fetch can corrupt the user's local repository.** Push can only damage the remote; a fetch that reports success on an incomplete closure leaves refs pointing at absent objects, in a repository the user trusts and may hold other work in. That is the worst outcome available anywhere in this project, and it sets the posture below.

| Condition | Behaviour |
|---|---|
| Marker absent or unrecognised | Hard refusal; never initialise |
| A downloaded `.pack` fails checksum-vs-basename, or a pair fails `index-pack --verify` | Fatal, before install. Note the `.idx` is **not** checksummed against its basename — see Orchestration step 2 |
| Closure incomplete against the wants | Fatal, before install; never report success |
| Refs advertised but `packs/` empty | Fatal — the remote is corrupt, and "up to date" would be a lie |
| Any download, integrity, or connectivity failure | **Local object store unchanged** — every one of these happens before install |
| A failure *after* install (protocol write error, crash) | The pack and index remain. Best-effort rollback, and the residue is named below rather than pretended away |

**Verify before install, not after.** Installing and then verifying leaks: a failure after install has already written a pack and a `.keep` into the user's repository, and the `.keep` protects a pack git was never told about — so `gc` will never reclaim it and nothing will ever remove it. Verifying in the temp area first means every *download, integrity and connectivity* failure leaves the local repository byte-for-byte untouched.

**The residual window, stated rather than papered over.** An earlier draft promised an unchanged object store after *any* failure. That is not achievable and the claim has been withdrawn. Between install and git's ref update there is an irreducible gap: if the helper dies after installing but before git receives the `lock` line, the pack, index and `.keep` all remain, and because git was never told about the `.keep` nothing will ever remove it. Best-effort rollback on a post-install error narrows it; it cannot close it. This is the same shape as push's orphan-pack window, and the same answer applies — the residue is inert and a retry reconciles — except that here the residue also carries a `.keep`, so it costs disk that `gc` will not reclaim until a human removes the file.

**One scope limit on `connectivity-ok`.** The check runs against the temp alternate plus the existing local object database. For an ordinary clone into an empty repository those are the same thing. Under `git clone --reference`, borrowed alternates can satisfy links the fetched packs do not contain, so a passing check would establish that *this repository* is whole without establishing that the pack is self-contained — which is what `connectivity-ok` claims. 3a does not support `--reference`; the limit is recorded here so the claim is not later assumed to be stronger than it is.

---

## Testing

**Deterministic — real git on both ends, the Fake in between.** Build a real repository, `WritePack` it, plant those bytes in `Fake.Files` under `/r/packs/`, fetch into a second real repository, and assert the objects arrived and the refs resolve. Genuine pack and index handling, no account.

**Live** — the contract table's `*CLI` half, and the stage gate.

**RED/GUARD labelling carries forward.** Every test is one or the other, and a RED must be observed failing before its fix. This is not ceremony: the Stage 2.1 plan's first draft was rejected in peer review for containing tests that passed against unpatched code.

## The gate

`git clone -o proton-v2 proton::<path>` of a repository pushed by this helper produces a **working checkout on the right branch**, and a subsequent `git fetch` after a further push brings the new commits down. Verified against the real account, with the local repository inspected independently — `git log`, `git status`, and `git fsck` clean.

A clone that fetches objects but checks out nothing is a failure, not a partial success.

## Out of scope for 3a

The `.idx` cache, the object-to-pack map, iterative discovery and its termination rule, selective pack download (all 3b); shallow and partial clone beyond the existing poison flag; tag transitions beyond create; `refs/notes` and other namespaces; hierarchical ref names, which remain the documented Stage 2 limitation; the CLI version allowlist and the size ceilings, both Stage 4.
