# git-remote-proton Stage 3b — selective fetch

**Status:** design, 2026-08-03. Brainstormed with Craig; scope, cache home, and gate mechanism decided in conversation.
**Extends:** [`docs/v2-remote-helper-design.md`](../../v2-remote-helper-design.md) at v6.3 (normative — Fetch steps 2–4, the termination rule, resume-safety, and the caching validity rule are that document's; this one records where 3b narrows or is more specific) and [`2026-08-01-v2-stage3a-fetch-design.md`](2026-08-01-v2-stage3a-fetch-design.md).
**Depends on:** Stage 3a (merge `180aaaf`) — a live-proven download-everything fetch, clone with clean fsck, and the contract table 9/9 live.
**Transport contract:** [`docs/research/probes/stage1-results.json`](../../research/probes/stage1-results.json), certified on `cli-drive@0.7.0` only.

---

## Goal

An incremental `git fetch proton-v2` against a remote holding several packs downloads **only the pack(s) it actually needs — measured, not assumed** — with everything 3a's gate proved still passing.

3b is a pure optimisation against a live-proven baseline. That framing is load-bearing for debugging: a discovery bug reads as "fetched too little" against known-good behaviour — a comparison, not a mystery.

## Decisions taken here

| Decision | Choice | Consequence |
|---|---|---|
| Hierarchical refs | **Deferred past 3b** (own stage, 3c or part of 4) | 3b's gate compares against an unchanged ref surface; no new push/list transport surface enters the discovery stage |
| HEAD-update (default-branch change) | **Stage 4** | Push-side policy/UX operation; needs its own command-surface design |
| Cache home | **Per-repo**: `<git-common-dir>/proton-v2/idx-cache/<remote-key>/` | Lifecycle tied to the repo; no eviction policy, no cross-repo races. A fresh clone re-downloads all sidecars — acceptable, it downloads all packs anyway |
| Cache shape | **Raw `.idx` files exactly as downloaded**, keyed by pack basename | No derived on-disk format to version or invalidate; the map is rebuilt per fetch by git plumbing |
| Discovery loop | **One uniform iteration** — the wants are just the first frontier | No special bootstrap round; probed `--missing=print` reports missing tips (below) |
| Pack choice for a multi-pack OID | Lexicographically first candidate | Deterministic and testable. Greedy set-cover is a recorded future optimisation, not built |
| Gate measurement | **Transport-boundary trace decorator**, always on | Every `ReadTo` from any code path is counted; identical mechanism hermetic and live; doubles as progress output |
| Cache failure | **Degradation, not failure** — warn on stderr, use the run's temp dir for sidecars | Correctness never depends on the cache existing |

## Probe: `rev-list --missing=print`, pinned on the installed git

Run 2026-08-03 on `git version 2.53.0.windows.2` (local repos only, no account). Two commits A←B packed disjointly; a destination repo saw only B's pack via a `pack/`-shaped alternate:

- **Missing parent commit** (tip B present, parent A absent): `rev-list --objects --missing=print B` printed B's objects **and `?<A>`, exit 0**. This is the loop's normal driving case — pack N+1 holds the new commits, their parent lives in pack N — and it works.
- **Missing tip** (A absent everywhere): printed `?<A>`, exit 0. This is why no special round-0 exists: the wants enter the same rev-list as every later frontier.

`?` lines interleave with object lines; parsing is by prefix, never by position.

**The behaviour is version-dependent** — older gits die on a missing tip instead of printing it. The helper pins no git version, so: the probe result is recorded here; a GUARD test pins it against whatever git runs the suite; and on an older git the failure mode is git's own loud fatal in round 0 — fail-closed, never a silent wrong answer or an unterminated loop.

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
- **Writes are temp-file-then-rename** in the same directory. A crashed fetch cannot leave a half-written sidecar under a valid name; a concurrent fetch loses the rename race harmlessly.
- **Validity is the parent design's rule verbatim:** an entry is valid while its pack name appears in the remote `packs/` listing. Each fetch prunes entries whose pack has vanished. v2 never deletes a pack, so this is defensive, not expected.
- **A `.idx` is never checksummed against its basename** — the basename is the *pack's* checksum. Its structural validity is checked by `show-index` (component 3) and its truth by pair verification once its pack is local (component 5).

### 3. The object-to-pack map — built fresh each fetch, by plumbing

For every listed pack whose **`.pack` member is present in the listing** (an orphan remote `.idx` never becomes a download target), ensure the sidecar is cached, then run `git show-index` over it (stdin-fed) and accumulate OID → []pack-name.

- `show-index` speaks index **v1 and v2**. This is the parent design's trap closed the way it prescribes: Push deliberately accepts a valid remote v1 `.idx` it did not write, so a v2-only parser would accept an index at push time it cannot fetch later. Using git's own reader means the accepted set is git's and cannot drift from it.
- A **cached** sidecar `show-index` rejects is discarded and re-downloaded once — a corrupt cache entry self-heals as a cache miss instead of surviving to masquerade as the no-progress fatal with its misleading diagnosis. A **freshly downloaded** sidecar that still fails is fatal naming the file: the remote's sidecar is bad.
- Rebuilding per fetch costs a linear re-read of small files and buys freedom from a second on-disk format. The map lives only in memory.

### 4. `internal/gitcmd` additions

`RevListMissing(gitDir, altObjects string, wants []string) (missing []string, err error)` — the `--objects --missing=print <wants> --not --all` invocation, `?`-prefix parsing, alternate spliced via `GIT_ALTERNATE_OBJECT_DIRECTORIES`. `ShowIndex(idxPath string)` — entries or error. Exact flag spellings are pinned by the first implementation task with tests that fail when the behaviour is absent, the standard this project applies to every guard; the *semantics* are pinned by the probe above.

### 5. `internal/repo/fetch.go` — the loop replacing `downloadAllPacks`

Everything around it is untouched: marker check, presence short-circuit, `ConnectivityOK` before install, `RevListNewObjects`, `consolidateAndInstall`, read-only/no-lock. The diff is confined to *which packs land in the temp area*.

```
downloaded = {}
loop:
    missing = RevListMissing(gitDir, altObjects, wants)
    if missing is empty: break                      # discovery complete
    packs = { pick(map[oid]) for oid in missing }   # oid not in map → fatal: remote incomplete;
                                                    # pick = lexicographically first candidate
    toGet = packs − downloaded
    if toGet is empty: fatal                        # no-progress rule, see below
    for pack in toGet:
        download .pack into tmp/objects/pack/
        checksum-vs-basename on the .pack           # the .pack ALONE — never the .idx
        copy cached .idx beside it                  # git discovers packs in an alternate via the .idx,
                                                    # so this copy is required for traversal anyway
        index-pack --verify on the pair             # both members local: the only time pair-truth is checkable
        downloaded += pack
```

- **Termination is structural.** Every surviving round downloads at least one pack not previously downloaded; packs are finite; the loop ends. A round whose resolved packs are all already held is the parent design's no-progress signature — "a stale or corrupt index mapping a missing OID into a pack already held" — and is fatal, with a message that names clearing the cache as a known remedy, since a stale cache is one cause a user can fix.
- **Per-pack verification happens inside the loop, before the next `rev-list`** — no object from a pack may be trusted until both checks pass for that pack, and the next round's traversal *reads* those objects.
- **Pair-failure diagnosis exploits content-addressing.** The pack already proved it matches its name, so on `index-pack --verify` failure the *index* is the suspect member: re-download the sidecar once, re-verify, then fatal naming the pair. This is the fetch-side sequel to the push side's per-member asymmetry, with the extra twist that the suspect member here arrived from *cache*, not from this download.
- **Resume-safety is presence-based and needs no new code:** `--missing=print` reports objects absent from gitDir ∪ alternate; anything already local is never missing, so an interrupted-then-retried fetch re-downloads only what it still lacks. The mixed-wants case (one want up to date, one behind) also falls out: present wants contribute no missing OIDs and therefore no downloads.

### 6. Wiring

`main.go` wraps the transport in the trace decorator and passes the cache directory (resolved from `--git-common-dir` + remote root) into `Fetch`. `Fetch`'s signature grows the cache path; tests that want no cache pass a temp dir.

## Error handling — new rows

| Condition | Behaviour |
|---|---|
| Cached `.idx` rejected by `show-index` | Discard, re-download once; fresh copy failing is fatal naming the file |
| Missing OID absent from the map | Fatal naming the OID; the remote does not hold a closure for the wants |
| Round resolves only already-downloaded packs | Fatal (no-progress rule); message names a stale cache as a user-fixable cause |
| Pair verification fails with a cached sidecar | Re-download the sidecar once, re-verify; still failing is fatal naming the pair |
| Remote `.idx` listed with no `.pack` | Excluded from the map; never a download target (unrepairable-direction row in the parent table already covers reporting on push) |
| Cache directory cannot be created or written | stderr warning; sidecars for this run live in the temp dir; fetch proceeds |

All discovery failures happen **before install** — the 3a posture (a failed fetch leaves the local object store byte-for-byte untouched) is unchanged, because the install path is unchanged.

## Testing

**Deterministic — real git on both ends, the Fake in between**, as in 3a. RED tests: the loop fetches a two-pack history from the pack holding only the tip (drives one real iteration); the no-progress fatal (a map poisoned to resolve a missing OID into a held pack); corrupt-cached-sidecar self-heal; pair-failure sidecar re-download. GUARD tests: `--missing=print` prints missing tips and missing parents on the suite's git; `show-index` reads a v1 index (`index-pack --index-version=1` builds the fixture); selectivity itself — a fetch needing one of three Fake-hosted packs downloads exactly one, asserted on trace output.

**The deliberate-regression twin, hermetic:** the selectivity assertion demonstrably *fails* against download-everything behaviour (run the same scenario through a loop seeded with every pack), proving the measurement can detect over-fetching before the live gate trusts it.

**Live** — the contract table's `*CLI` half, and the gate below. Live writes confined to `/my-files/GitRemotes/<demo>`; gate runners report BLOCKED with verbatim output and never patch.

## The gate

Against the real account, stderr captured and parsed for `gpb: downloaded ` lines:

1. Three pushes from a source repo → remote holds 3 packs (verified via CLI listing).
2. **Fresh clone** → downloads exactly the 3 `.pack`s and 3 `.idx`s under `packs/`; working checkout on the right branch; `git log`/`status`/`fsck` clean — 3a's gate preserved, cache now populated. Every download assertion in this gate is scoped to `packs/` paths: ref and marker reads also emit trace lines, legitimately, and are excluded the same way throughout.
3. Push #4 → **incremental fetch downloads exactly `pack-4.idx` + `pack-4.pack`** — cache hits on the other three sidecars, selectivity on the pack. Ref advanced; fsck clean.
4. **Up-to-date re-fetch → zero `packs/` downloads.** Scoped to `packs/` deliberately: ref advertisement legitimately reads ref files, so the assertion is "no pack or sidecar transferred", not "no transport traffic".
5. Everything asserted on measured trace output, never on timing or assumption.

## Parked flags, disposed

- **`RevListNewObjects` output held in memory** — deferred again, with rationale: its size is bounded by *new-object* count, which 3b does not change (consolidation input is identical to 3a's). It becomes real at compaction scale, and compaction is the milestone that revisits it.
- **Delete-then-recreate-in-one-batch pinned against the Fake only**, and **`ReadHEAD` once per deleted ref in a batch** — push-side, untouched by 3b, re-parked for Stage 4 and recorded here so they cannot silently evaporate.

## Out of scope

Hierarchical ref names (own stage); HEAD-update (Stage 4); greedy set-cover pack selection; streaming `RevListNewObjects`; shallow/partial clone beyond the poison flag; the CLI version allowlist and size ceilings (Stage 4); compaction and retention (separately approved milestone).
