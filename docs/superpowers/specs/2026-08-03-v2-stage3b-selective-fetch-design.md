# git-remote-proton Stage 3b — selective fetch

**Status:** design, 2026-08-03. Brainstormed with Craig; scope, cache home, and gate mechanism decided in conversation. Revised through three peer-review rounds, Codex + Gemini (see Revisions).
**Extends:** [`docs/v2-remote-helper-design.md`](../../v2-remote-helper-design.md) at v6.3 (normative — Fetch steps 2–4, the termination rule, resume-safety, and the caching validity rule are that document's; this one records where 3b narrows or is more specific) and [`2026-08-01-v2-stage3a-fetch-design.md`](2026-08-01-v2-stage3a-fetch-design.md).
**Depends on:** Stage 3a (merge `180aaaf`) — a live-proven download-everything fetch, clone with clean fsck, and the contract table 9/9 live.
**Transport contract:** [`docs/research/probes/stage1-results.json`](../../research/probes/stage1-results.json), certified on `cli-drive@0.7.0` only.

---

## Goal

An incremental `git fetch proton-v2` against a remote holding several packs downloads **only the pack(s) it actually needs — measured, not assumed** — with everything 3a's gate proved still passing. "Needs" is greedy, not proven-optimal, and conditional on truthful metadata: with a truthful sidecar cache — the common case — the loop downloads no pack that contributes nothing to the current missing frontier; a lying cache causes at most bounded over-download before the self-heal round rebuilds it, and never a wrong install (the verification chain is unchanged). Exact minimal set cover is not attempted.

3b is a pure optimisation against a live-proven baseline. That framing is load-bearing for debugging: a discovery bug reads as "fetched too little" against known-good behaviour — a comparison, not a mystery.

## Decisions taken here

| Decision | Choice | Consequence |
|---|---|---|
| Hierarchical refs | **Deferred past 3b** (own stage, 3c or part of 4) | 3b's gate compares against an unchanged ref surface; no new push/list transport surface enters the discovery stage |
| HEAD-update (default-branch change) | **Stage 4** | Push-side policy/UX operation; needs its own command-surface design |
| Cache home | **Per-repo**: `<git-common-dir>/proton-v2/idx-cache/<remote-key>/` | Lifecycle tied to the repo; no eviction policy, no cross-repo races. A fresh clone re-downloads all sidecars — acceptable, it downloads all packs anyway |
| Cache shape | **Raw `.idx` files exactly as downloaded**, keyed by pack basename | No derived on-disk format to version or invalidate; the map is rebuilt per fetch by git plumbing |
| Discovery loop | **One uniform iteration** — the wants are just the first frontier | No special bootstrap round; probed `--missing=print` reports missing tips (below) |
| Pack choice for a round's missing OIDs | **Deterministic greedy**: packs that are an OID's *only* candidate are forced first; remaining uncovered OIDs are covered by the candidate pack holding most of them, ties broken lexicographically | Never downloads a pack contributing nothing to the frontier; cheap; deterministic and testable. Exact set-cover remains out of scope |
| Minimum git for fetch | `rev-list --missing=print` must report missing tips and parents as `?` lines (present in the installed 2.53; absent in older gits) | On an older git, round 0 fails closed with an **actionable** wrapped error naming this requirement — never a loop, never a silent wrong answer. No version *allowlist* is introduced (that remains Stage 4, CLI-side); this is a documented behavioural floor |
| Gate measurement | **Transport-boundary trace decorator**, always on | Every `ReadTo` from any code path is counted; identical mechanism hermetic and live; doubles as progress output |
| Cache failure | **Degradation, never failure** — any cache I/O failure (create, write, rename, read) warns on stderr and falls back to the run's temp dir for the affected sidecar(s) | Correctness never depends on the cache existing, and — after this revision — never depends on it being *right* either (see the self-heal round) |

## Probe: `rev-list --missing=print`, pinned on the installed git

Run 2026-08-03 on `git version 2.53.0.windows.2` (local repos only, no account). Two commits A←B packed disjointly; a destination repo saw selected packs via a `pack/`-shaped alternate:

- **Missing parent commit** (tip B present, parent A absent): `rev-list --objects --missing=print B` printed B's objects **and `?<A>`, exit 0**. This is the loop's normal driving case — pack N+1 holds the new commits, their parent lives in pack N — and it works.
- **Missing tip** (A absent everywhere): printed `?<A>`, exit 0. This is why no special round-0 exists: the wants enter the same rev-list as every later frontier.
- **Missing tree, and the frontier deepens** (a pack holding only commit B, its tree and blob absent): printed `?<tree>` — and did **not** print the blob beneath that tree. An absent object conceals everything below it, so missingness is discovered incrementally as objects arrive: rounds are bounded by graph depth, and the loop's iteration is essential, not defensive.
- **`?`-line shape:** on this git, `?` lines carry a bare OID with no path suffix, and interleave with `<oid> <path>` object lines. Parsing is normative anyway (component 4): by prefix, first whitespace-delimited token, 40-hex validated — never by position or line shape.

**The behaviour is version-dependent** — older gits die on a missing tip instead of printing it. The helper pins no git version, so: the probe result is recorded here; a GUARD test pins it against whatever git runs the suite; and the minimum-git decision row above turns the older-git failure into a diagnosed refusal rather than an opaque one.

## Components

### 1. `internal/transport/trace.go` — the measurement boundary

A decorator wrapping `Transport`, installed at construction in `main.go`, always on. One stderr line per **successful `ReadTo`**, stable prefix normative:

```
gpb: downloaded <remote-path> (<n> bytes)
```

The prefix `gpb: downloaded ` is what the gate parses; the size is informative. Wrapping the only handle to the remote is the honesty argument: fetch orchestration cannot forget to log, and a future bug that adds a download shows up in the count. The Fake is wrapped by the same decorator in hermetic tests, so the deliberate-regression twin and the live gate assert against identical output.

### 2. `internal/repo/idxcache.go` — the sidecar cache

- **Location:** `<git-common-dir>/proton-v2/idx-cache/<remote-key>/pack-<hash>.idx`. The common dir comes from `rev-parse --git-common-dir` — shared across linked worktrees, unlike `--git-path <arbitrary-name>`, which resolves into the per-worktree admin dir — and is **absolutised before use** (the 3a relative-`GIT_DIR` lesson, applied before it can bite: git commonly spawns helpers with `GIT_DIR=.git`, and `--git-common-dir` answers relative in an ordinary repo).
- **`<remote-key>`** is a short SHA-256 of the remote root path — sidestepping every Windows-filename hazard in paths like `/my-files/GitRemotes/demo` — with a `remote` breadcrumb file inside holding the plain path for humans.
- **Writes are temp-file-then-rename** in the same directory (on Windows, Go's `os.Rename` replaces an existing file). A crashed fetch cannot leave a half-written sidecar under a valid name. Concurrency needs no protocol beyond the degradation rule: a fetch that loses a rename race, or fails any cache read or write for any reason, uses its own temp copy for that sidecar and warns — cache trouble is never allowed to become fetch trouble.
- **Validity is the parent design's rule verbatim:** an entry is valid while its pack name appears in the remote `packs/` listing. Each fetch **that enters discovery** prunes entries whose pack has vanished (an up-to-date fetch short-circuits before listing and prunes nothing — the earlier "each fetch prunes" wording contradicted the preserved short-circuit). v2 never deletes a pack, so pruning is defensive, not expected.
- **A `.idx` is never checksummed against its basename** — the basename is the *pack's* checksum. Its structural validity is checked by `show-index` (component 3) and its truth by pair verification once its pack is local (component 5).

### 3. The object-to-pack map — built fresh each fetch, by plumbing

Input is the remote `packs/` listing, filtered twice, both filters normative:

- **Name grammar before any filesystem use.** Only file (non-directory) nodes whose names match `^pack-[0-9a-f]{40}\.(pack|idx)$` exactly participate in discovery, caching, or mapping. Anything else is skipped with a stderr note — the existing stray-node stance — and its name is **never joined into a local path**. Listed names are remote-controlled input; without this rule a hostile or corrupt name (path separators, `..`, drive syntax) reaches `ReadTo` destinations and cache paths.
- **Only complete pairs enter the map.** A listed `.idx` with no `.pack` is never a download target (the unrepairable direction); a listed `.pack` with no `.idx` is the normal signature of a concurrent or crashed push observed mid-publication (Push uploads the pack before the index) and is skipped with a stderr note, not an error. If a skipped pack later turns out to be needed, the missing-OID path below reports it — after the self-heal round re-lists and retries, which also covers the race where the pair completed moments after the first listing.

For each surviving pack, ensure the sidecar is cached, then run `git show-index` over it (stdin-fed) and accumulate OID → []pack-name.

- `show-index` speaks index **v1 and v2**. This is the parent design's trap closed the way it prescribes: Push deliberately accepts a valid remote v1 `.idx` it did not write, so a v2-only parser would accept an index at push time it cannot fetch later. Using git's own reader means the accepted set is git's and cannot drift from it.
- A **cached** sidecar `show-index` rejects is discarded and re-downloaded once — a structurally corrupt cache entry self-heals as a cache miss. A **freshly downloaded** sidecar that still fails is fatal naming the file: the remote's sidecar is bad.
- **The map is invalidated whenever a sidecar it was built from is refreshed** — by the self-heal round or by a pair-verification re-download (component 5). Entries derived from the old bytes are rebuilt from the new ones before the loop consults the map again; a refreshed file with a stale in-memory map is the desynchronisation both reviewers flagged, and it presents as the no-progress fatal with a wrong diagnosis.
- **Scaling note:** building the map spawns one `show-index` process per pack, so map construction costs linear process-spawn overhead on top of the parent design's recorded linear discovery cost. Both are the same debt with the same creditor: compaction. Recorded, not redesigned here.

### 4. `internal/gitcmd` additions

- `RevListMissing(gitDir, altObjects string, wants []string) (missing []string, err error)` — the `--objects --missing=print <wants> --not --all` invocation, alternate spliced via `GIT_ALTERNATE_OBJECT_DIRECTORIES`. **Parsing is normative:** a line beginning `?` yields its first whitespace-delimited token after the `?`, which must match `^[0-9a-f]{40}$` or the line is a hard parse error — never trimmed-and-trusted, never assumed path-free, even though the probed git prints bare OIDs. On a git whose `rev-list` dies at a missing tip (the pre-floor behaviour), the wrapper returns an error that names the minimum-git requirement from the decisions table — the diagnosed refusal, not git's bare fatal.
- `ShowIndex(idxPath string)` — entries or error.
- Exact flag spellings are pinned by the first implementation task with tests that fail when the behaviour is absent, the standard this project applies to every guard; the *semantics* are pinned by the probe above.

### 5. `internal/repo/fetch.go` — the loop replacing `downloadAllPacks`

Everything around it is untouched: marker check, presence short-circuit, `ConnectivityOK` before install, `RevListNewObjects`, `consolidateAndInstall`, read-only/no-lock. The diff is confined to *which packs land in the temp area*.

```
downloaded = {}
healed = false
round:
    missing = RevListMissing(gitDir, altObjects, wants)
    if missing is empty: break                      # discovery complete
    packs = greedyCover(missing, map)               # forced singles, then most-covering, ties lexicographic

    # CACHE-SUSPECT failures — each heals at most once per fetch, then is fatal:
    #   an OID in missing has no candidate pack        → heal, else fatal: remote incomplete
    #   packs − downloaded is empty                    → heal, else fatal: no-progress rule
    #   a selected pack fails download                 → heal, else fatal: remote/transport trouble
    #   a selected pack fails checksum-vs-basename     → heal, else fatal: remote corruption
    # heal = re-list packs/, discard every cached sidecar for listed packs,
    #        re-download fresh, rebuild the map, RESTART the round
    # every failed download or checksum attempt DELETES its target file(s) from
    # tmp/objects/pack/ before the restart — see the residue rule below

    for pack in packs − downloaded:
        download .pack into tmp/objects/pack/       # failure → heal-or-fatal, above
        checksum-vs-basename on the .pack           # the .pack ALONE — never the .idx; failure → heal-or-fatal
        copy cached .idx beside it                  # git discovers packs in an alternate via the .idx,
                                                    # so this copy is required for traversal anyway
        index-pack --verify on the pair             # both members local: the only time pair-truth is checkable
        downloaded += pack
        on pair failure: re-download the sidecar once, rebuild its map entries, re-verify;
                         still failing → fatal (corrupt pair, member undetermined);
                         now passing   → RESTART the round (the plan predates the rebuild)
```

- **Termination is structural.** Every restart is paid for: the self-heal runs at most once per fetch, and a pair-refresh restart is preceded by a newly downloaded, verified pack, so restarts are bounded by pack count plus one. Between restarts, every surviving round downloads at least one pack not previously downloaded; packs are finite; the loop ends.
- **The self-heal round is what makes the cache-degradation promise true, and its trigger set is every cache-suspect failure, not just the two fatal diagnoses.** A cached sidecar that parses but lies — a bit-flipped OID entry, a cache-key collision, tampering — can present as "remote incomplete", as "no progress", **or as a download or checksum failure on a pack a truthful map would never have selected**; in each case the actual fault may be local and user-invisible, so each gets one fresh rebuild before its fatal. After healing, every terminal diagnosis was reached on sidecars downloaded *this run*: a fatal then genuinely indicts the remote or the transport. The fatal messages state that the cache was already refreshed, so none ever advises cache-clearing as a remedy.
- **Any map rebuild restarts the round's planning.** The missing set and the greedy plan are recomputed from the fresh map before any further download — a plan computed from the old bytes may name packs the new map knows are irrelevant. Packs already downloaded and verified are kept (they are genuine, content-addressed data; at worst over-downloaded), only the *plan* is discarded.
- **A failed attempt cleans up its residue before any retry.** A failed download can leave a partial file, and a checksum failure necessarily leaves a corrupt one, at the pack's own basename in `tmp/objects/pack/` — and a healed plan may legitimately re-select that same pack. `ReadTo`'s behaviour onto an existing file is deliberately unpinned by the transport contract (only directory-must-exist is), and C2 warns that identical-content writes can be silently skipped, so retry correctness must never depend on overwrite semantics: the failed member file(s) are deleted before the round restarts. The corrupt residue is invisible to git either way — traversal discovers packs only through the `.idx`, which is copied beside the pack strictly *after* its checksum passes — so this rule protects the retry, not the graph.
- **Per-pack verification happens inside the loop, before the next `rev-list`** — no object from a pack may be trusted until both checks pass for that pack, and the next round's traversal *reads* those objects.
- **Pair-failure handling is a heuristic, not a verdict.** Checksum-vs-basename proves the pack matches its *name*, not that it is well-formed — a malformed pack can be self-consistently named. On `index-pack --verify` failure the cached sidecar is merely the *cheaper* suspect: re-download it once, rebuild its map entries, re-verify. A pair that still fails is fatal as a **corrupt pair, member undetermined** — the message names both files and does not pretend to know which is bad.
- **Resume-safety is presence-based and needs no new code:** `--missing=print` reports objects absent from gitDir ∪ alternate; anything already local is never missing, so an interrupted-then-retried fetch re-downloads only what it still lacks. The mixed-wants case (one want up to date, one behind) also falls out: present wants contribute no missing OIDs and therefore no downloads.

### 6. Wiring

`main.go` wraps the transport in the trace decorator and resolves the cache directory: `rev-parse --git-common-dir`, whose answer is combined stdout+stderr from the same `git()` runner `RevParse` uses and gets the same treatment `validateObjectsPackPath` established — refuse any answer containing a newline (a merged git warning), require a plausible shape (non-empty, no `\r`), absolutise before use. `Fetch`'s signature grows the cache path; tests that want no cache pass a temp dir.

## Error handling — new rows

| Condition | Behaviour |
|---|---|
| Listed name failing the pack-name grammar, or a directory | Skipped with a stderr note; never joined into a local path |
| Listed `.pack` with no `.idx` (concurrent/crashed push window) | Skipped with a stderr note; not an error. Becomes one only if discovery needs it after the self-heal round's fresh listing |
| Listed `.idx` with no `.pack` | Excluded from the map; never a download target |
| Cached `.idx` rejected by `show-index` | Discard, re-download once; fresh copy failing is fatal naming the file |
| Missing OID with no candidate pack | Self-heal round (once): fresh listing, fresh sidecars, rebuilt map; recurring after it is fatal naming the OID — the remote does not hold a closure for the wants |
| Round resolves only already-downloaded packs | Same self-heal round; recurring after it is fatal (no-progress rule). Both messages state the cache was already refreshed this run |
| A selected pack fails download or checksum-vs-basename | The failed file(s) are deleted from the temp area, then the same self-heal round — the selection may rest on a lying cache, and a truthful map might never have touched this pack; recurring after it is fatal as genuine remote or transport trouble |
| Pair verification fails | Re-download the sidecar once, rebuild its map entries, re-verify; passing resumes with the round's plan recomputed (the old plan predates the rebuild); still failing is fatal naming a **corrupt pair, member undetermined** |
| Any cache I/O failure (create, write, rename, read) | stderr warning; the affected sidecar(s) live in the run's temp dir; fetch proceeds |
| Older git: `rev-list` dies at a missing tip | Wrapped, actionable error naming the minimum-git requirement (decisions table); fail-closed at round 0 |

All discovery failures happen **before install** — the 3a posture (a failed fetch leaves the local object store byte-for-byte untouched) is unchanged, because the install path is unchanged.

## Testing

**Deterministic — real git on both ends, the Fake in between**, as in 3a. RED tests: the loop fetches a two-pack history from the pack holding only the tip (drives one real iteration); a frontier that deepens through a missing tree (probe 3's shape — the blob is only discoverable after the tree arrives); the no-progress fatal *after* a failed self-heal; the self-heal succeeding (a parseable-but-lying cached sidecar — a bit-flipped OID entry — fixed by the rebuild, fetch completes); the self-heal reached from a checksum failure (a lying cache selects a pack whose Fake-hosted bytes are corrupt; the healed map routes around it and the fetch completes without that pack); the same-pack retry (a transiently failed download leaves residue at the destination basename; after healing the plan re-selects that pack and the retry succeeds cleanly); map rebuild after a pair-verification sidecar re-download, with the round's plan recomputed rather than resumed; greedy selection (x in packs {A,B}, y in {B} → downloads B alone); a listed `.pack` without its `.idx` skipped, fetch succeeds when unneeded; grammar-violating listed names never reaching a filesystem join (asserted via the Fake's listing). GUARD tests: `--missing=print` prints missing tips and missing parents on the suite's git; `?`-token extraction and 40-hex validation; `show-index` reads a v1 index (`index-pack --index-version=1` builds the fixture); corrupt-cached-sidecar self-heal; selectivity itself — a fetch needing one of three Fake-hosted packs downloads exactly one, asserted on trace output; cache-write failure degrading to temp with the fetch still succeeding.

**Untestable on the suite's git, recorded rather than pretended:** the older-git missing-tip fatal (the suite's git supports `--missing=print` on tips, so the wrapper's diagnosed-refusal path for that case is exercised only by unit-testing the error translation, not end-to-end).

**The deliberate-regression twin, hermetic:** the selectivity assertion demonstrably *fails* against download-everything behaviour (run the same scenario through a loop seeded with every pack), proving the measurement can detect over-fetching before the live gate trusts it.

**Live** — the contract table's `*CLI` half, and the gate below. Live writes confined to `/my-files/GitRemotes/<demo>`; gate runners report BLOCKED with verbatim output and never patch.

## The gate

Against the real account, stderr captured and parsed for `gpb: downloaded ` lines:

1. Three pushes from a source repo → remote holds 3 packs (verified via CLI listing).
2. **Fresh clone** → downloads exactly the 3 `.pack`s and 3 `.idx`s under `packs/`; working checkout on the right branch; `git log`/`status`/`fsck` clean — 3a's gate preserved, cache now populated. Every download assertion in this gate is scoped to `packs/` paths: ref and marker reads also emit trace lines, legitimately, and are excluded the same way throughout.
3. Push #4 → **incremental fetch downloads exactly `pack-4.idx` + `pack-4.pack`** — cache hits on the other three sidecars, selectivity on the pack. Ref advanced; fsck clean.
4. **Up-to-date re-fetch → zero `packs/` downloads.** Scoped to `packs/` deliberately, per step 2.
5. Everything asserted on measured trace output, never on timing or assumption.

## Parked flags, disposed

- **`RevListNewObjects` output held in memory** — deferred again, with rationale: its size is bounded by *new-object* count, which 3b does not change (consolidation input is identical to 3a's). It becomes real at compaction scale, and compaction is the milestone that revisits it.
- **Delete-then-recreate-in-one-batch pinned against the Fake only**, and **`ReadHEAD` once per deleted ref in a batch** — push-side, untouched by 3b, re-parked for Stage 4 and recorded here so they cannot silently evaporate.

## Out of scope

Hierarchical ref names (own stage); HEAD-update (Stage 4); exact set-cover pack selection (greedy is the 3b bar); streaming `RevListNewObjects`; shallow/partial clone beyond the poison flag; the CLI version allowlist and size ceilings (Stage 4); compaction and retention (separately approved milestone).

## Revisions

**Round 1 (2026-08-03, Codex + Gemini):** Applied — self-heal round before the missing-OID and no-progress fatals, and map rebuild on every sidecar refresh (both engines; the cache-can't-fail-the-fetch principle now holds for *wrong* caches, not just absent ones); greedy pack selection replacing lexicographic-first (both engines; hash-ordered names made the old rule effectively random); complete-pairs-only map admission covering the concurrent-push window (both engines); normative pack-name grammar before any filesystem join (Codex); pair-verify fatal reworded to corrupt-pair-member-undetermined (Codex — checksum-vs-basename proves naming, not well-formedness); cache degradation generalised to all cache I/O incl. rename races (Codex); "each fetch prunes" scoped to fetches that enter discovery (Codex); normative `?`-line tokenization with 40-hex validation (Gemini), plus probe 3 pinning missing-tree line shape and the deepening frontier; `--git-common-dir` answer validated like `validateObjectsPackPath` (Gemini); minimum-git behavioural floor with diagnosed refusal (both engines raised the older-git regression); show-index spawn cost recorded as compaction debt (Gemini). Rejected — a permanent fallback to 3a's download-everything path (Codex's highest-impact proposal): a second live code path would mask selectivity regressions behind silent fallback, doubling the maintenance surface to hedge a failure the self-heal round plus diagnosed refusals already convert from opaque to actionable.

**Round 2 (2026-08-03, Codex + Gemini):** Gemini: blockers none. Codex: one major, accepted — the round-1 self-heal was incomplete: it triggered only on the no-candidate and no-progress fatals, so a lying cached index could still fail a fetch by selecting a pack whose download or checksum fails (trouble a truthful map would never touch), and a pair-verify sidecar refresh left the round's already-computed plan resting on the old bytes. Fixed by widening the heal's trigger set to every cache-suspect failure (no-candidate, no-progress, selected-pack download failure, selected-pack checksum failure — each once, then fatal) and by the rule that any map rebuild restarts the round's planning; the Goal's no-noncontributing-pack claim is now explicitly conditional on a truthful cache. Termination argument updated: restarts are bounded by pack count plus one.

**Round 3 (2026-08-03, Codex + Gemini, final under the three-round cap):** Gemini: blockers none, round-2 fixes explicitly verified. Codex: one major in the round-2 fix itself, accepted — the heal-and-retry path could re-select a pack whose earlier failed attempt left partial or corrupt bytes at the destination basename, and `ReadTo`'s behaviour onto an existing file is unpinned (C2's identical-content skip makes reuse of stale bytes a real hazard). Fixed with the residue rule: every failed download/checksum attempt deletes its target file(s) before the round restarts, plus a same-pack-retry RED test. Noted for honesty: the corrupt residue was never reachable by git traversal (the `.idx` is copied beside a pack only after its checksum passes); the rule protects the retry, not the graph.
