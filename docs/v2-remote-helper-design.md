# git-remote-proton — v2 design

**Status:** design v5, revised 2026-08-01 after four rounds of Codex + Gemini peer review. Not implemented.
**Relates to:** [issue #3](https://github.com/craigstoller/git-proton-backup/issues/3).
**Depends on:** [`docs/research/remote-helper-prior-art.md`](research/remote-helper-prior-art.md) — read first; assumed, not repeated.
**Pinned to:** Proton Drive CLI **`cli-drive@0.7.0`** (SDK `js@0.20.0`) — Stage 1 was certified
against this build; results in `docs/research/probes/stage1-results.json`. Support is an
**exact-version allowlist**, not a minimum. That is not theoretical caution: 0.4.6 and 0.7.0
differ in the `activeRevision` payload shape, in whether a byte-identical rewrite is skipped,
and in concurrent-startup behaviour. A version floor would have admitted 0.7.0 and broken
verification silently — which is exactly what happened to v1 before it was fixed in 0.2.4.

---

## What this is

A `git-remote-proton` binary that makes Proton Drive a real git remote.

```
git clone -o proton-v2 proton::/my-files/GitRemotes/myrepo
git push proton-v2 main
git fetch proton-v2
```

**URL form:** everything after `proton::` is a Proton Drive POSIX path, matching the CLI's convention. Canonicalisation rules are normative — see "Storage layout".

**Remote name:** `-o proton-v2` is required on clone, because `git clone` names its remote `origin` by default and **v1 already owns the remote named `proton`** (it rewrites that remote's URL to the local bookkeeping mirror and sets its push refspecs). This is a coexistence requirement, and the clone example must carry `-o` or the guidance contradicts itself.

**v1 is unchanged by this.** With distinct remote names, both operate on one repo.

## Goals and non-goals

**Goals:** a self-contained tool; proper git semantics; cross-platform (a consequence of using neither `cldapi.dll` nor the sync app); fail-closed behaviour.

**Non-goals:** multi-machine concurrent pushes; eliminating the CLI dependency; implementing Proton auth; shallow/partial clone.

**Deployment surface, honestly:** `git-remote-proton` **and** `proton-drive` on PATH, an authenticated CLI session, and on Linux a working OS secret store. One prerequisite, installed once — not literally one file.

## Substrate decision

Scoping v2 to a single writer removes the need for the SDK, for vendoring its internals, and for implementing auth — because the conditional-write primitive those reach exists only to make *concurrent* writers safe.

**This is an explicit override of the memo's Avoid #3** ("v2's writes must go through the SDK"), which was written for a multi-writer v2. Recorded here rather than left implicit; it is the first thing to revisit if multi-writer becomes a goal.

**Verified, not assumed:** the CLI runs on macOS, Windows and Linux (Goal 3 depends on it), and is standalone — no desktop sync app.

**Language: Go.** No *external* language runtime (Go embeds its own; full static linking additionally requires avoiding cgo); one cross-compilation matrix; `go-proton-api` exists as a distant fallback, though rclone calls that bridge unofficial and error-prone, so it is weak evidence rather than a plan. Rust was rejected narrowly — its advantages here are smaller than they look for an I/O-bound tool, though "purely CPU-bound" was too strong a dismissal. Python was rejected on binary size, startup, and PyInstaller's inability to cross-compile.

## Storage layout (normative)

```
<repo-root>/
  gpb-remote.json             format marker: {"format":"git-remote-proton","version":1}
  HEAD                        "ref: refs/heads/<branch>\n"
  refs/heads/<branch>         "<40-hex sha>\n"   one file per ref
  refs/tags/<tag>
  refs/<other-namespace>/…
  packs/<sha256>.pack         immutable, content-named, never rewritten
  packs/<sha256>.idx
  .lock                       advisory single-writer lock (JSON, see Concurrency)
```

**Format marker.** `gpb-remote.json` is written on initialisation and read before any other operation. An unrecognised or absent marker on a non-empty folder is a hard refusal — the helper never guesses whether a folder is one of its repos.

**Initialisation ordering is normative, because two independently-correct rules deadlocked
without it.** v3 required the lock to be acquired before `list for-push`, and separately treated
an absent marker in a *non-empty* folder as a hard refusal. Acquiring the lock creates `.lock`,
which makes an empty folder non-empty - so a first push would create the lock, then refuse
itself, permanently bricking the remote on its first command. The bootstrap sequence is:

1. `Stat` the marker. If present, proceed normally.
2. If absent, `List` the folder. **`.lock` is excluded from the emptiness test** - it is helper
   scaffolding, not repo content.
3. If the folder is otherwise non-empty, hard-refuse: it is someone else's data.
4. If empty, `CreateExclusive` the marker **before** acquiring the lock. A concurrent
   initialiser loses the race, re-reads, and proceeds.
5. Only then acquire the lock and continue.

A folder with a valid marker but missing `refs/` or `packs/` is a partial initialisation and is
completed, not rejected.

**Ref-file grammar:** exactly 40 lowercase hex characters plus `\n`. Anything else is corruption and is fatal, never coerced.

**Ref names map to remote paths directly, but NOT to local ones.** A5 (2026-07-31) verified Proton's *remote* name uniqueness is exact-byte, with no case folding or Unicode normalisation — so no escaping is needed on the remote side. That is not the whole problem. The CLI takes a **local file path** as its upload source, and CLI 0.4.6 treats a local path containing `{` as a glob and expands it, while git happily accepts `refs/heads/a{b,c}`. Windows additionally reserves names like `con`, which git also accepts.

So v2 **stages every ref write through a neutral temporary local filename** (a content-independent name in the helper's temp directory) and uploads it to the desired remote name. The ref name never appears in a local path, which sidesteps globbing, reserved names, and case-insensitive local filesystems in one move. Ref names are still validated with `git check-ref-format`, and any name the CLI cannot express as a *remote* path is rejected explicitly rather than silently mangled.

**Pack naming:** `<sha256>` is the SHA-256 of the pack file's bytes, deliberately distinct from git's object hash so the two are never confused.

**No generation field in v2.** An earlier draft reserved one for future compaction while also
calling it unused, then referenced it from the fetch cache and the error table - reserved and
relied upon at once. Removed. The marker's `version` field is the forward-compatibility seam:
compaction will bump it and define its own ordering scheme at that point. The v2 fetch cache is
keyed on pack name plus existence only.

**Path canonicalisation:** the address is normalised before use — duplicate and trailing slashes collapsed, `.` and `..` rejected outright rather than resolved, empty path rejected, and the root must lie under `/my-files` or `/devices`. The CLI also permits creation under
`/shared-with-me`, which v2 **refuses**: a repo in a folder another person can write to is
precisely the concurrent-writer case this design has no defence against, so allowing it would
invite the one failure mode single-writer cannot survive. The canonical form is the cache and lock identity.

## Architecture

```
git  ──spawns──▶  git-remote-proton  ──subprocess──▶  proton-drive CLI  ──▶  Proton Drive
     stdin/stdout                     --json
```

**Protocol layer** speaks git's helper protocol; knows nothing about Proton. **Repository layer** decides what moves. **Transport layer** is the only Proton-aware code.

### Transport contract

A generic `Upload` is unsafe: the CLI's `replace` trashes the existing node *before* creating the new one, so a crash can destroy a ref. Semantics must be explicit, and large objects must stream rather than materialise in memory.

```go
type Outcome int
const (
    Committed Outcome = iota  // definitely applied
    Refused                   // name existed; nothing changed
    Ambiguous                 // outcome unknown - MUST be reconciled by reading remote state
)

type Transport interface {
    EnsureDir(path string) error                       // Stat-then-create; create-folder fails if it exists
    List(path string, opts ListOpts) ([]Node, error)   // non-recursive; paginates internally; unordered
    Stat(path string) (node Node, exists bool, err error)   // absence is (.., false, nil), never an error
    ReadTo(path string, localPath string) error        // streams to disk
    CreateExclusive(path, localPath string) (Outcome, error)
    UpdateRevision(path, localPath string) (Outcome, error)
    Trash(path string) (Outcome, error)                // NOT idempotent: Stat first (see below)
}
```

**Verified CLI mappings (Stage 1, CLI 0.7.0 — `docs/research/probes/stage1-results.json`).** Three of these contradict what this document previously asserted:

| Operation | CLI | Result shape | Notes |
|---|---|---|---|
| `CreateExclusive` | `upload -f skip --json` | transfer summary | `(1,0,0)`=Committed, `(0,1,0)`=Refused |
| `UpdateRevision` | `upload -f merge --json` | transfer summary | **Verified revises in place** — node uid stable, revision uid changes |
| `Trash` | `trash --json` | **`{uid, ok}` — NOT a transfer summary** | needs its own parser; the upload parser would silently read nulls |
| `EnsureDir` | `create-folder parent name` | text | **fails on an existing folder** |
| `List` | `list path --json` | JSON array | empty folder → exit 0, zero elements. Distinguishable from failure |

**`Trash` on a missing target FAILS with exit 1** — this document previously declared it idempotent and `Committed`, which is wrong. Since the desired end state is still "not there", the helper must `Stat` first, or treat this specific absent-target error as success. It cannot simply call `Trash` and trust the exit code, which is what concurrent branch deletion and lock cleanup would otherwise do.

**`EnsureDir` is NOT idempotent** — `create-folder` on an existing folder exits 1. It must be implemented as `Stat`-then-create, never create-and-ignore-error, because a generic error swallow would also hide a real permission or path failure.

**A byte-identical rewrite is silently skipped.** Verified: re-uploading content whose sha1 matches the existing node returns `skippedItems=1` with no write, before any conflict strategy applies. A no-op is therefore indistinguishable from a successful write by exit code or counts alone — which is exactly why every mutable write is followed by read-back verification. This is the single strongest justification for that rule.

**Every non-idempotent mutation returns `Outcome`**, including `Trash`. A plain `error` cannot distinguish a delete that failed from one that committed before the response was lost, and the error table requires that distinction.

**`CreateExclusive` maps to `upload -f skip --json`.** The outcome is read from the transfer summary, whose shape is pinned by probe A6 on CLI 0.4.6:

```json
{"transferredItems":1,"transferredBytes":8,"skippedItems":0,"failedItems":0,"failures":[]}
```

**Count tuples are exact and mutually exclusive** for a single-file operation - the CLI's transfer queue records exactly one of success, skip, or failure per item, so `(0,0,2)` is contradictory and is `Ambiguous`. Any `>=` comparison is too permissive — a `transferred=1, skipped=1` response is contradictory for one file and must not read as success:

| `transferred` | `skipped` | `failed` | Outcome |
|---|---|---|---|
| 1 | 0 | 0 | `Committed` |
| 0 | 1 | 0 | `Refused` |
| 0 | 0 | 1 | error (**exit code may still be 0**) |
| anything else, or unparseable JSON, or a missing field | | | `Ambiguous` |

A missing count field is `Ambiguous`, never defaulted to zero — a renamed field in a future CLI must fail loudly rather than silently read as "nothing happened."

**Exit codes cannot distinguish these.** A6 confirms both success and refusal exit 0. Any implementation branching on exit status is wrong.

**Mutable writes require post-write verification, because the CLI may skip silently.** **Version-specific, and previously misattributed here.** CLI **0.7.0** auto-skips when the
existing node''s *claimed* SHA-1 matches the local file''s, **before** any conflict strategy is
applied. CLI 0.4.6 - the build the probes ran on - does *not*; it resolves the strategy
directly. An earlier revision of this document attributed the 0.7.0 behaviour to 0.4.6. Since
0.7.0 is the current release, the hazard is real for the version users would install, which
makes read-back verification more important rather than less. So `UpdateRevision` is not guaranteed to create a revision, and the decision is made from a digest Proton itself flags `sha1Verified: false`. Every mutable write is therefore followed by `ReadTo` and a byte comparison; equality by claimed digest is never accepted as proof. This is the one place the design cannot trust its transport, and it is why `Ambiguous` exists as a first-class outcome.

**Immutable objects** (packs, indexes) use `CreateExclusive`. `Refused` there means identical content already exists — success *after* byte verification, not an error.

## Concurrency posture

**Single writer per repo is a product precondition, weakly enforced.**

**Lifecycle is normative, because a lock without one is useless.** A helper that acquires the lock *after* advertising can still overwrite: A advertises, B acquires and moves a ref, A acquires and writes from its stale view.

1. **Acquire before `list for-push`** — always, even for a read-only-looking batch.
2. **Verify by read-back**: after `CreateExclusive` returns `Committed`, read `.lock` and confirm the nonce matches. A `Refused` means someone holds it; report and stop.
3. **Read refs under the lock.** The advertisement git receives must be derived after acquisition.
4. **Hold across the entire push batch**, including all per-ref status responses.
5. **Release on every exit path** - normal completion, an up-to-date push that sends no batch,
   a poisoned or rejected batch, git aborting after `list for-push`, EOF, cancellation, or
   signal. With takeover removed, a leaked lock wedges the repo until a human clears it, so
   release is a `defer`, not a happy-path step.
6. **Release verifies the nonce first, and this remains check-then-act.** No conditional delete
   exists, so if an operator manually clears a stale lock and another process acquires it in
   the gap, this process can still delete the newcomer''s lock. The window is narrow and
   requires manual intervention mid-operation, but it is **not** eliminated - only takeover was
   removed, not the underlying race.

**Lock contents:** `{"nonce":"<uuid>","host":"…","pid":123,"acquiredAt":"<rfc3339>"}`. The **nonce**, not the hostname, is identity — two processes on one machine are otherwise indistinguishable. A local OS file lock keyed by the **canonical remote address** (not the working copy) serialises clones on the same machine before the remote attempt.

> ### STAGE 1 FINDING (RESOLVED) — the CLI cannot be STARTED concurrently (CLI 0.7.0)
>
> Discovered while re-running the probes against 0.7.0 on 2026-08-01. **Two `proton-drive`
> processes cannot run at the same time on one machine.** The second dies during startup:
>
> ```
> SQLiteError: database is locked   errno: 261   code: "SQLITE_BUSY_RECOVERY"
>   at new SQLiteCache (src/cache/sqliteCache.ts:12)
>   at LO (src/init.ts:49)
> ```
>
> This is the CLI's **local encrypted SQLite session cache**, not a Proton response. The loser
> never issues a network request.
>
> **Three consequences, and they are not small:**
>
> 1. **A7's 0.7.0 run does not establish server atomicity.** "Exactly one winner" there is an
>    artifact of local cache locking, not server arbitration. The **0.4.6** A7 run remains
>    genuine evidence — its losers exited 0 with `skippedItems=1`, meaning they reached the
>    server and were refused by it.
> 2. **The v1/v2 coexistence claim in this document is wrong as written.** Using distinct remote
>    names avoids a *git* collision, but it does not let a v1 sweep and a v2 push run
>    simultaneously: both shell out to the same CLI and would collide on this cache. Any
>    concurrent `proton-drive` use on the machine is affected, including a user running the CLI
>    by hand during a scheduled sweep.
> 3. **v2 must serialise every CLI invocation machine-wide**, via a local named mutex keyed to
>    the CLI's cache path rather than to the repo. That is a different lock from the remote
>    `.lock`, with a different scope, and the design previously had no concept of it.
>
> **RESOLVED by probe A8 (2026-08-01): the lock is TRANSIENT, and covers CLI STARTUP only.**
>
> A holder process ran a real 8.4-second upload (exit 0). A second process issued a cheap
> read 3.1 seconds in — **while the holder was still running** — and succeeded first try, with
> zero `SQLITE_BUSY` failures. The stack trace corroborates: the failure occurs at
> `src/init.ts:49 → new SQLiteCache`, during initialisation, not during the operation.
>
> So processes collide **only when their startups overlap**, which is exactly what A7 forced by
> firing four workers within 0.3 ms. Once a process is past init, another can start freely.
>
> **Revised consequences:**
>
> - **Machine-wide serialisation is NOT required.** The helper needs **retry with backoff on the
>   SQLite signature at startup**, which is a far cheaper obligation than a global mutex.
> - **v1/v2 coexistence is back on**, provided both retry. A v1 sweep and a v2 push can overlap;
>   what they cannot do is *start* at the same instant without one retrying.
> - **A7's 0.4.6 result stands as genuine evidence of server-side atomicity** — its losers reached
>   the server and were refused with `skippedItems=1`. A7 on 0.7.0 remains uninformative about
>   the server, because its losers died locally.
>
> **Not measured:** the width of the startup window. A8 sampled at 3.1 s and found it clear; it
> did not bisect for the boundary. Retry policy should therefore be derived from observation
> during Stage 2 rather than from a guessed constant.
>
> **No per-process cache option exists** — the CLI's only global flags are `--help`, `--json`,
> and `--verbose` — so retry is the mechanism, not isolation.

**What is verified.** A6 confirms `CreateExclusive` refuses a second writer sequentially, on both 0.4.6 and 0.7.0. **A7 on CLI 0.4.6 (2026-08-01) confirms it under genuine concurrency** — the Stage 1 gate that could have invalidated this design. The 0.7.0 re-run does *not* corroborate it, for the reason in the blocker above. Four worker processes, barrier-aligned to fire spreads of 0.3–2.8 ms, three rounds, on CLI 0.4.6: every round produced **exactly one `transferred=1` and three `skipped=1`**, zero failures, with the surviving remote content always matching the winner. The **winner rotated between rounds** (W1, W4, W3), which is what distinguishes a genuine race arbitrated by the server from a harness that accidentally serialised its workers.

**What that does and does not establish.** The experiment is asymmetric by construction: observing two winners would have *disproved* atomicity outright, while observing none only *supports* it. Three rounds at four workers is not proof of a race-free server, and this result should be re-run whenever the pinned CLI version changes. But the specific failure that would have sunk the lock — two writers both told they succeeded — did not occur under conditions designed to provoke it.

**The lock is therefore buildable.** Acquisition rests on a server-arbitrated primitive rather than a client-side check. What remains unprotected is unchanged and stated above: ref *updates* are still last-write-wins, because create-exclusive protects names, not revisions.

**What is not protected at all:** ref *updates* are last-write-wins. Without a conditional write there is no defence if two writers do run concurrently. New refs are safer, since `CreateExclusive` genuinely refuses. **The lock lowers the chance of an accident; it does not make concurrent use safe.**

**Release cannot be conditional, so v2 has no takeover at all.** An earlier draft promised release would "not delete a replacement lock." That guarantee is impossible with this transport: verifying the nonce and then calling `Trash` is check-then-act, and no conditional delete exists. If A verifies its nonce, B takes over, and A then trashes by path, A deletes B's lock.

Rather than ship an impossible promise, **v2 removes takeover**. A stale lock is reported — holder nonce, host, age — with instructions to remove it manually via the CLI once the operator has established the holder is dead. There is no override flag. This is less convenient and it is honest; a fencing token or conditional delete would be required to do better, and neither exists without the SDK.

**Weaker than v1's lock.** v1 holds an OS file handle with exclusive sharing — a kernel mutex. A file on cloud storage is a convention.

## Push

### Unsupported-option state machine (safety-critical)

**Git does not honour the helper's rejection of every option, so "reply `unsupported`" is not a defence.** This is the single most dangerous subtlety in the protocol.

- **`--atomic`** → git sends `option atomic true`, **checks the response, and aborts** on rejection. Replying `unsupported` is sufficient and correct.
- **`--force-with-lease`** → git sends `option cas <ref>:<expected>`, **ignores the response, and sends the forced push batch anyway** (`transport-helper.c:1029-1046`). Replying `unsupported` accomplishes nothing: the push proceeds as a plain force, silently discarding the exact safety property the user asked for.
- **Shallow and partial** → git likewise ignores responses to `depth`, `deepen-since`, `deepen-not`, `deepen-relative`, `update-shallow`, `filter`, `from-promisor`, `no-dependents`, and `refetch` (`transport-helper.c:709-724`, `751-767`). Git's transport layer reports success when *either* its internal layer or the helper accepts an option, which is why rejection alone protects nothing. (`no-dependents` is documented by git but no sender was located in the inspected commit - its ignore-behaviour is **unverified**). A fresh `git clone --depth` sends `depth`, not `update-shallow`, so watching for the latter alone misses the common case.

**Therefore the helper maintains session state, not per-option replies.** Any unsupported safety option seen during the option phase sets a poison flag naming it. The flag is checked **after the complete blank-line-terminated batch has been buffered**, not when the first `push` line arrives - the protocol permits further `option` lines after the last `push` and before the terminator, so checking early can miss one. Enforcement happens, and the batch is failed with a message naming the option — **before any pack is uploaded, any ref is written, or any object is installed.** Replying `unsupported` to the option itself is still done, but it is treated as advisory, never as protection.

This conformance is **Stage 2/3 work**, not Stage 4 — it is a correctness property of the first working push, not a polish item.

**Ref transitions** — enumerated, because one ancestry rule applied to every ref repeats a flaw the memo found in git-remote-dropbox:

| Transition | Rule |
|---|---|
| Create branch | `CreateExclusive`; `Refused` means a concurrent creator — report, never overwrite |
| Fast-forward branch | `merge-base --is-ancestor old new`, then `UpdateRevision` + verify |
| Non-fast-forward, unforced | `error <ref> non-fast-forward` |
| Old sha absent locally, unforced | `error <ref> fetch first` |
| Forced branch update | Skip ancestry; still `UpdateRevision` + verify |
| Branch target not a commit | **Reject** — git never allows a branch to point at a tree, blob, or tag object |
| Delete (`push :dst`) | `Trash`; refuse to delete the branch `HEAD` points at |
| Tag create | `CreateExclusive` |
| Tag update | Requires force, matching git's rule; no ancestry check |
| `refs/notes/*`, `refs/replace/*`, other valid namespaces | Create-exclusive on create; **force required to move** — an intentional conservative deviation, see below |
| Pseudorefs and unsupported destinations | Explicit rejection with a named reason |
| First push to empty remote | Write the marker, then `HEAD` per the deterministic rules below |

**The other-namespace rule is deliberately stricter than git.** Git's push rules are object-type aware outside `refs/heads/*` and `refs/tags/*`: some commit and tag fast-forwards are permitted without force, while tree and blob updates are refused. v2 requires force for any move in those namespaces. That is a **knowing deviation from the "proper git semantics" goal**, taken because the conservative rule cannot silently lose data and the permissive one needs object-type inspection this design has not specified. It is listed as future work rather than presented as equivalent to git.

**Initial `HEAD` is deterministic**, because it decides what a later clone checks out:

- `HEAD` is chosen from branches whose ref writes **succeeded**, and is written **after** them - selecting a requested branch that then fails would leave `HEAD` dangling, and writing it first would violate the pack→idx→ref ordering.
- Single successfully published branch → `HEAD` points at it.
- Multiple branches in one first push → the branch matching the client's own `HEAD` if it is among them; otherwise the lexicographically first, so the result never depends on batch ordering.
- Tag-only or non-branch first push → **no `HEAD` is written**, and the repo is left headless. A later branch push writes it. A clone before then fetches objects and reports that no default branch exists, rather than silently checking out nothing.
- `HEAD` is never rewritten implicitly afterwards. Changing the default branch is an explicit operation, out of scope for v2.

**Protocol mechanics the helper must implement** — referenced throughout but previously never stated, which alone blocked implementation:

- **`list` / `list for-push` output:** one `<40-hex-sha> SP <refname>` line per ref, plus `@<refname> SP HEAD` for the symref on plain `list`, terminated by a blank line.
- **Push commands arrive as `push <src>:<dst>`**, batched, blank-line terminated. **A leading `+` on `<src>` is how a forced push is signalled** — `push +refs/heads/main:refs/heads/main`. There is no separate force option; the helper parses the prefix. Nothing else distinguishes forced from unforced.
- **Push responses:** `ok <dst>` or `error <dst> <reason>`, one per ref, then a blank line. Every ref in the batch receives exactly one status, including refs rejected during validation.
- **Object type is resolved locally with `git cat-file -t <sha>`** before packing. The helper receives only hashes from git, so the "branch target must be a commit" rule is unenforceable without this — it was specified as a rule with no mechanism.

**Object transfer per batch:** compute `git rev-list --objects <new> ^<remote-tip>…`, excluding only advertised tips that also exist locally (unknown tips are simply not excluded — larger pack, never wrong); build **one non-thin pack** (`--no-thin`); `CreateExclusive` the pack, then the `.idx`; **`Stat` both to confirm presence before any ref is written**. A ref whose index is missing is not fetch-discoverable.

**Ordering: pack → idx → confirm → ref.** Failures before publication leave orphan packs, which are inert.

**Multi-ref batches are not atomic.** Each ref reports its own `ok`/`error`; partial success is expected.

**Shallow and promisor repositories are refused**, and — because a fresh `git clone --depth` is not yet shallow when the helper starts — the helper must poison on the full set - `depth`, `deepen-since`, `deepen-not`, `deepen-relative`, `update-shallow`, `filter`, `from-promisor`, `no-dependents`, `refetch`. A startup check alone is insufficient.

## Fetch

The helper owns the closure. Git's `fetch` capability is defined as transferring "objects **reachable from**" the named ones, and under `check-connectivity` "**the helper** must output `connectivity-ok`". Git does not iteratively request missing objects.

1. Advertise refs plus `@refs/heads/<branch> HEAD`.
2. Download `.idx` sidecars not already cached; build an object-to-pack map.
3. Download packs containing wanted objects into a temp area.
4. **Traverse.** An `.idx` maps object IDs to byte offsets and carries no connectivity data, so the object graph must be walked. Index each downloaded pack into a **temporary object directory** and expose it via `GIT_ALTERNATE_OBJECT_DIRECTORIES`, keeping unverified remote objects out of the real database until closure is proven.

   **The walk uses git plumbing, not a hand-written parser.** `git cat-file --batch` decompresses and identifies objects but does *not* enumerate commit parents, tag targets, or tree entries — an earlier draft claimed it removed the need for a parser, which was wrong. Instead run `git rev-list --objects --missing=print <wants>` against the temp directory: git performs the traversal and reports missing OIDs prefixed with `?`. Feed those back through the object-to-pack map, download, repeat. This also handles gitlinks and malformed objects with git's own semantics rather than reimplemented ones.
5. **Consolidate the closure into ONE pack, install it, and emit git's single `lock`.** Git's `transport-helper.c` retains only the **first** `lock <path>` response, so a multi-pack install cannot be protected by multiple protocol locks.

   **This decision reversed twice and is now settled on the merits.** v2 used consolidation; v3 replaced it with helper-managed `.keep` files swept at the start of the next fetch; v4 restores consolidation. The helper-managed variant is **not crash-safe**: if a fetch dies after installing a pack but before git updates refs, the next fetch removes that pack's protection *before* refs exist, and a concurrent `git gc` can delete it between connectivity verification and git's ref write — leaving refs pointing at missing objects. It also leaks permanently if the user never fetches from that remote again. Git's protocol lock exists precisely to cover the install-to-ref-update interval and is released only after both complete. Corruption beats a repacking pipeline as a thing to avoid.

   **Naming is normative:** git recognises a `.keep` only when its stem exactly matches the adjacent pack (`packfile.c:368-384`). The installed files are `pack-<git-pack-hash>.{pack,idx,keep}` using git's own naming from `index-pack`, not the remote's SHA-256 content name.
6. **Verify connectivity against the exact requested wants**, after all imports and before reporting success — an explicit missing-object-fatal traversal rooted at the wants, not a generic `fsck`, since the wants are not yet referenced by any ref.

**Termination is explicit.** The loop maintains a set of already-downloaded packs and a set of still-missing OIDs. Each round must either download a pack not previously downloaded, or resolve at least one missing OID. **A round that does neither is fatal** — that is the signature of a stale or corrupt index mapping a missing OID into a pack already held, and without this check the loop runs forever.

**Resume-safety:** prune the walk at objects already present locally *with complete history*, so an interrupted fetch resumes.

**Caching** is valid only while a pack exists. v2 never deletes a pack, so an entry cannot go stale; the cache is keyed on pack name and existence. Compaction will invalidate that assumption, and will bump the marker's `version` when it does.

**Discovery cost grows linearly with pack count.** Every fetch enumerates `packs/`; a new client downloads every `.idx`. This is the design's main scaling weakness and the reason compaction is a real milestone.

## Error handling

**Fail closed.** Anything unconfirmed is a failure.

| Class | Behaviour |
|---|---|
| CLI missing, logged out, session expired mid-operation | Startup probe plus per-call detection; actionable message |
| CLI version not on the certified allowlist | Refuse to run - exact versions, not a floor |
| `failedItems > 0` with exit code 0 | Treated as failure — never inferred from exit status |
| Unparseable or unexpected `--json` shape | `Ambiguous`; reconcile against remote state before retry |
| Mutation timed out after the remote may have committed | `Ambiguous`; read back before any retry |
| Upload refused where creation was required | Contention, not success |
| Pack present, `.idx` missing | Ref not published; orphan reported |
| Crash between pack and ref | Orphan pack, inert; retry reconciles first |
| Ref already at desired sha on retry | Success, not conflict |
| Missing or unrecognised format marker on a non-empty folder | Hard refusal; never guess |
| Missing versus empty remote | Distinguished; empty is initialisable, missing is an error |
| File/folder collision at an expected path | Fatal with the specific path |
| Malformed ref name or ref-file contents | Fatal; never coerced |
| Corrupt, mismatched, or orphaned `.pack`/`.idx` | Byte/hash verify on read; mismatch fatal |
| Fetch made no progress in a round | Fatal — see Termination |
| Incomplete closure after import | Fatal; fetch never reports success |
| Cached pack no longer present remotely | Entry discarded; cache rebuilt from the pack listing |
| List pagination failure | Fatal; a truncated listing must never look like a complete one |
| Unsupported git option (`--atomic`, `--force-with-lease`, shallow, filter) | Explicit rejection naming the option |
| Quota, rate limit, oversized pack | Distinct messages; no silent truncation |
| Local disk full, permissions, cancellation, timeout | Reported distinctly |
| Network failure mid-transfer | Partial discarded, never indexed; retry re-downloads whole |

**Never trust `claimedDigests.sha1`** — Proton flags it `sha1Verified: false`, and the CLI's own auto-skip depends on it, which is exactly why mutable writes are verified by read-back.

**Recovery is provisional, not promised.** Immutable packs are recoverable by construction. Recovering an *overwritten or deleted ref value* depends on Proton's revision and trash retention, which is **unverified** — and on knowing the old OID. Until retention is established, the documented promise covers object data, not ref history.

## Testing

**Two layers, deliberately.**

- **Deterministic**: in-memory fake transport plus fault injection covers the protocol and repository layers, every ref transition, every error-table row, ordering, termination, and lock lifecycle — including cases impossible to provoke live (quota exhaustion, mid-request death, disk full).
- **Live compatibility**: a smaller suite against a real account, run at every stage gate, pinned to a CLI version, using the A4/A5/A6 guardrail pattern.

**Seams alone are insufficient and v1 proved it** — the `GetNewClosure` bug was invisible to 92 seamed tests and surfaced only live, because every test mocked past the branch containing it.

**Tests must assert their preconditions.** Both the false-`COLLIDED` A5 run and the first version of A6 drew conclusions from runs that had not actually exercised what they claimed. A6 v2 asserts `transferredItems`/`skippedItems`/`failedItems` explicitly and reports ERROR when preconditions fail.

## Implementation staging

**Vertical after Stage 1** — each later stage ends with something a user can actually do through git, so integration problems surface early rather than at the end.

**Stage 1 — pinned transport contract. COMPLETE 2026-08-01 on CLI 0.7.0.** Results are committed and normative: `docs/research/probes/stage1-results.json`, produced by `probe-stage1-transport-contract.ps1` alongside probes A4-A8. It found three assertions in this document to be wrong (Trash idempotency, EnsureDir idempotency, Trash output shape), confirmed two (merge revises in place, empty list distinguishable), and established the claimed-SHA-1 no-op that makes read-back verification mandatory. Original scope:
Executable, not prose. Produces a committed results file that becomes normative. Covers, on a pinned CLI version:
~~`CreateExclusive` under two barrier-synchronised concurrent processes~~ **DONE — probe A7, 2026-08-01: 4 workers x 3 rounds, 0.3-2.8 ms spread, exactly one winner every round, winner rotating. Re-run on any CLI version change.**; whether `merge` preserves the prior revision readable until commit; whether the claimed-SHA-1 auto-skip can be defeated or must be worked around; exact `--json` shapes for success, skip, failure, and error; ambiguous-outcome boundaries (kill mid-upload, then read back); `EnsureDir` behaviour on existing and partial paths; `List` pagination and ordering; streaming limits and oversized-file behaviour; and what "durable" means — when a write becomes visible to a second client.
**The single item most likely to invalidate the design is now settled** (A7). What remains is contract detail: `merge` revision preservation, ambiguous-outcome boundaries, `Trash` output shape, pagination, streaming, and durability.

**Stage 2 — a real `git push`.** Format marker, initialisation, lock lifecycle, ref transitions, pack build, ordering guarantee, `list for-push`. Ends when `git push proton-v2 main` works end to end against a real account.

**Stage 3 — a real `git clone` and `git fetch`.** Idx cache, iterative discovery with termination, single-pack consolidation, `.keep`, connectivity verification. Ends when `git clone -o proton-v2 proton::…` produces a working checkout.

**Stage 4 — productionisation.** Cross-compiled release artefacts, v1 coexistence testing, recovery documentation and its test, broader protocol conformance.

Compaction and retention remain a separately approved milestone. v2 reserves nothing for them beyond the marker's `version` field, which is the seam a future layout change goes through.

## Open questions

1. **Pack compaction** — generation-tagged, reader-safe, avoiding gcrypt's full-reupload cliff.
2. **Retention** — what Proton keeps, for how long, and therefore what recovery may honestly promise.
3. **Git object hash** — SHA-1 repositories only at first, with explicit early refusal of SHA-256.

## What would change this design

- **Stage 1 shows `CreateExclusive` is not atomic under contention** → the lock is decorative; single-writer becomes purely social, or the project waits for the SDK.
- **Stage 1 shows the claimed-SHA-1 auto-skip cannot be worked around** → mutable ref updates are unreliable on this transport, which is close to fatal for the CLI approach.
- **The Account SDK ships third-party auth** → A1–A3 become verifiable and multi-writer reachable; note this changes the **repository** layer too, not only transport, since revision tokens must be captured at `list` time and carried to push.
- **The CLI gains a conditional-write flag** → multi-writer without the SDK; worth an upstream request.

---

## Revision history

**v5, 2026-08-01 — fourth peer-review round. Severity did not decay, and both engines said so explicitly.**

- **A v3 fix created an initialisation deadlock.** Acquiring the lock before `list for-push` (correct) plus hard-refusing an absent marker in a non-empty folder (correct) meant a first push created `.lock`, making the folder non-empty, then refused itself — bricking the remote on its first command. Bootstrap order is now normative and `.lock` is excluded from the emptiness test.
- **The v4 `.keep` change was a crash-safety regression; consolidation is restored.** Helper-managed keeps swept at the next fetch cannot cover a crash between pack install and git's ref update — the sweep removes protection *before* refs exist, so a concurrent `gc` can delete the pack and leave refs pointing at missing objects. They also leak permanently if the user never fetches again. This flip-flopped across three revisions, chasing whichever reviewer spoke last; it is now settled on merits, with keep/pack stem naming made normative.
- **A factual error I introduced by trusting a review finding.** The pre-strategy SHA-1 auto-skip is **0.7.0** behaviour; **0.4.6** resolves the conflict strategy directly. A round-2 finding read `main` and I attributed it to the pinned build without checking the tag. Verified against both. The hazard remains real, since 0.7.0 is what users install.
- **Poison-flag timing was wrong.** The protocol permits `option` lines *after* the last `push` and before the terminator, so checking at the first `push` can miss one. The full batch is buffered first. `refetch` added; `no-dependents` marked unverified.
- **Ref names do not map safely to LOCAL paths.** A5 proved *remote* equivalence only. CLI 0.4.6 glob-expands local paths containing `{`, and git accepts `refs/heads/a{b,c}`; Windows reserves `con`. Ref writes now stage through a neutral temporary local filename.
- **`cat-file --batch` does not enumerate parents or tree entries**, so claiming it removed the need for a parser was wrong. Traversal now uses `rev-list --objects --missing=print`.
- **Protocol mechanics were never stated** — `list` grammar, the `+` prefix as the *only* forced-push signal, `ok`/`error` response format, and `cat-file -t` for the commit-target rule, which had been a rule with no mechanism. This alone blocked implementation.
- Lock release is now a `defer` covering EOF, cancellation and no-batch pushes (a leak wedges the repo now takeover is gone), and the residual check-then-act race on release is stated rather than implied away. Failure tuple tightened to exactly `(0,0,1)`. Initial `HEAD` is chosen from *successfully published* branches, after their ref writes.

**v4, 2026-07-31 — third peer-review round.** One finding was a live safety bug:

- **`--force-with-lease` was not actually rejected.** Git converts it to `option cas`, then **ignores the helper's response and sends the forced push anyway** (`transport-helper.c:1029-1046`). "Reply `unsupported`" — what v3 specified — does nothing, so the push would have proceeded as a plain force, discarding the precise safety property the user asked for. The same applies to `depth`, `deepen-*`, `update-shallow`, `filter`, `from-promisor` and `no-dependents`; only `--atomic` is genuinely honoured at the option response. Replaced with a session-level poison-flag state machine enforced at the start of the batch, before any mutation. Moved from Stage 4 to Stage 2/3.
- **The lock's release guarantee was impossible.** v3 promised release would not delete a replacement lock, but `Trash` is unconditional and verify-then-trash is check-then-act. **Takeover is removed from v2 entirely** rather than shipping a promise the transport cannot keep.
- **Typed outcomes were applied only partially.** `Trash` still returned a plain `error`, and `transferredItems >= 1` was too permissive — a contradictory `transferred=1, skipped=1` would have read as success. Exact mutually-exclusive count tuples now defined; a missing field is `Ambiguous`, never zero.
- **Generations were reserved and relied upon simultaneously** — absent from the marker schema, yet used by the fetch cache and error table. Removed from v2; the marker's `version` field is the forward-compatibility seam.
- **Probe A5 had the same defect A6 did** — writer B was never validated, so a failed second upload would still have produced one node and read as `COLLIDED`. Both probes now share one strict parser in `probe-lib.ps1` that requires exact field names and returns `$null` when absent. Re-run: A5 both cases DISTINCT, A6 confirmed.
- **The pinned CLI is stale.** Probes ran on `cli-drive@0.4.6`; Proton's current published CLI is **0.7.0**. Support is now an exact-version allowlist rather than a minimum, and Stage 1 must re-certify.
- Fetch traversal gained a normative mechanism (temp object dir + `GIT_ALTERNATE_OBJECT_DIRECTORIES` + `cat-file --batch`); pack consolidation was replaced with helper-managed `.keep` files and no `lock` response, as the simpler path to the same guarantee; initial `HEAD` gained deterministic multi-branch and tag-only rules; the other-namespace ref rule is now labelled a knowing conservative deviation rather than presented as git-equivalent; `Trash` on a missing target is `Committed`; and `/shared-with-me` is refused as a repo root, since a folder others can write to is exactly the concurrent-writer case this design cannot survive.

**v3, 2026-07-31 — second peer-review round.** Corrections, including two self-inflicted:

- **The normative storage layout had been accidentally deleted in the v2 rewrite** and its absence went unmentioned in the revision history, while later sections still depended on it. Restored and expanded with the decisions it had always lacked: format marker, initialisation and partial-init recovery, ref-file grammar, pack naming, generation reservation, path canonicalisation.
- **Probe A6 did not assert what the spec claimed it proved.** Writer B ran without `--json` and without any assertion, so any failure of B would have produced the same "works" verdict — the identical defect this project had already documented after the false-`COLLIDED` A5 run. A6 v2 parses and asserts the counts, records the CLI version, and reports ERROR on unmet preconditions. Re-run on `cli-drive@0.4.6`: A `transferredItems:1`, B `skippedItems:1 transferredItems:0`.
- **The CLI auto-skips when the existing node's claimed SHA-1 matches**, before any conflict strategy — so `UpdateRevision` may not create a revision, and the decision rests on a digest Proton flags unverified. Mutable writes now require read-back verification, and `Ambiguous` became a first-class outcome.
- **Git records only the first `lock <path>`**, so the multi-`.keep` fetch design was unimplementable. The closure is now consolidated into one installed pack.
- **The lock had no lifecycle**, so a stale advertisement could overwrite a concurrent writer even with both respecting it. Acquisition now precedes `list for-push` and is verified by nonce read-back.
- **Iterative fetch had no termination condition.** A no-progress round is now fatal.
- Ref table gained non-commit targets, other namespaces, pseudorefs, and explicit rejection of `--atomic` and `--force-with-lease`. Shallow/filter rejection moved to the protocol-option boundary. Transport interface gained `EnsureDir`, streaming, typed outcomes, and defined `List`/`Stat` semantics. `git clone` example gained `-o proton-v2`. Recovery downgraded to provisional. Staging became vertical.

**v2, 2026-07-31 — first peer-review round.** The `.lock` had claimed to prevent silent corruption; a generic `Upload` hid that `replace` trashes before creating; fetch was claimed to work from `.idx` alone; the upgrade path was claimed to touch only transport; v1 and v2 collided on the remote name `proton`; the push section covered only branch fast-forwards.

### Findings considered and rejected

- **"Fetch traversal is architecturally backwards — git drives the iteration."** Rejected. Git's documentation defines the `fetch` capability as transferring "objects **reachable from**" the named ones, says the command writes "the **necessary objects**", and places `connectivity-ok` on **the helper**. git-remote-dropbox implements the same client-side walk (memo §1b). Accepting this would have produced a fetch that silently returns incomplete history.
- **"Abandon the CLI for the SDK because the lock is racy."** Partly accepted — the lock *was* overstated and is now described accurately — but the conclusion did not follow: uniqueness is server-enforced, so acquisition is not check-then-act, and A6 confirms the refusal is real and detectable.
