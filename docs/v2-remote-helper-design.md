# git-remote-proton — v2 design

**Status:** design v2, revised 2026-07-31 after Codex + Gemini peer review. Not yet implemented.
**Relates to:** [issue #3](https://github.com/craigstoller/git-proton-backup/issues/3).
**Depends on:** [`docs/research/remote-helper-prior-art.md`](research/remote-helper-prior-art.md) — read that first; this document assumes its findings rather than repeating them.

---

## What this is

A `git-remote-proton` binary that makes Proton Drive a real git remote:

```
git clone proton::/my-files/GitRemotes/myrepo
git push proton-v2 main
git fetch proton-v2
```

Git's remote-helper mechanism spawns the binary automatically when it sees a `proton::` URL.

**URL form:** everything after `proton::` is a Proton Drive POSIX path identifying the repo folder, matching the CLI's own convention (`/my-files/…`). Taken literally — no implicit root is prepended. The folder is created on first push if absent.

**Remote name:** examples use **`proton-v2`**, not `proton`. v1 already owns the remote named `proton` — it rewrites that remote's URL to the local bookkeeping mirror and configures its push refspecs. Using the same name would break v1 on any repo running both. This is a hard naming constraint, not a stylistic choice.

**This is a different product from v1, not a replacement.** v1 (`GitProtonBackup`) is a Windows PowerShell module that rides the Proton Drive sync app and produces verified `git bundle` backups. It is unchanged by this. With the distinct remote name above, both can operate on the same repo.

## Goals, and non-goals

**Goals**

1. **A self-contained tool.** One binary to install; git drives it.
2. **Proper git semantics.** `clone`, `fetch`, `push` as first-class operations.
3. **Cross-platform.** Linux, macOS, Windows — a consequence of the architecture, since v2 uses neither `cldapi.dll` nor the sync app.
4. **Behavioural robustness.** Fail closed: anything short of confirmed success is reported as failure, in git's idiom.

**Non-goals for v2**

- **Multi-machine concurrent pushes.** Single writer is a *product precondition*; see "Concurrency posture", which states precisely how weakly it is enforced.
- **Eliminating the Proton CLI dependency.**
- **Implementing Proton authentication.**
- **Shallow or partial clone.** Explicitly refused, not silently mishandled.

### Deployment surface, stated honestly

"One binary" describes the helper, not the install. A working setup needs `git-remote-proton` **and** `proton-drive` on PATH, the CLI's authenticated session, and on Linux a working OS secret store for that session. That is still a reasonable dependency story — one prerequisite, installed once — but it is not literally a single file.

## Substrate decision

The prior-art memo left three coupled questions: which SDK, whether to vendor its internal upload layer, and where sessions come from. **Scoping v2 to a single writer removes the need for all three**, because the conditional-write primitive they exist to reach is required only to make *concurrent* writers safe.

| Memo question | Answer | Why |
|---|---|---|
| Which SDK? | **None** | Transport is the CLI |
| Vendor internal upload layer? | **No** | Only needed for CAS, only needed for multi-writer |
| Where do sessions come from? | **The CLI's own login** | Delegated, not solved |

**This is an explicit override of the memo's Avoid #3**, which states "v2's writes must go through the SDK." That instruction was written for a multi-writer v2. Under a single-writer scope the memo's own reasoning — that a subprocess helper "caps out at gcrypt-grade safety" — describes a ceiling this product does not need to exceed. The override is recorded here rather than left implicit, and it is the first thing to revisit if multi-writer becomes a goal.

**Two facts verified against Proton's documentation, not assumed:**

- **The CLI runs on macOS, Windows, and Linux.** Goal 3 depends entirely on this; a Windows-only CLI would have silently reintroduced the constraint v2 exists to escape.
- **The CLI is standalone** — no desktop sync app required. v2 sheds the sync app completely.

**Language: Go.** A single executable with **no external language runtime** (Go embeds its own; full static linking additionally requires avoiding cgo); one cross-compilation matrix for all three platforms; and `go-proton-api` exists as a future path if the CLI dependency is ever dropped — though rclone describes that bridge as unofficial and potentially error-prone, so it is weak evidence, not a plan.

Rust was rejected narrowly. Its advantages here are smaller than they look — this tool is I/O-bound, dominated by subprocess and network latency — but the claim "Rust's advantages are purely CPU-bound" was too strong: predictable latency and no GC are real properties. Go wins on cross-build simplicity and existing Proton libraries, not on performance. Python was rejected on binary size, startup cost, and the fact that PyInstaller cannot cross-compile.

## Architecture

```
git  ──spawns──▶  git-remote-proton  ──subprocess──▶  proton-drive CLI  ──▶  Proton Drive
     stdin/stdout                     --json
```

**Protocol layer.** Speaks git's helper protocol on stdin/stdout. Knows nothing about Proton.
**Repository layer.** Decides what moves: incremental object sets, packs, ref transitions.
**Transport layer.** The only code aware Proton exists.

### The transport contract

A generic `Upload(path, data)` is **not** sufficient, because the CLI's conflict strategies differ in ways that are safety-critical. `replace` trashes the existing node *before* creating the new one, so a crash mid-operation can destroy a ref. `merge` creates a new revision of the existing node. `skip` refuses when the name exists. The interface must therefore expose semantics, not bytes:

```go
type Transport interface {
    List(path string) ([]Node, error)
    Stat(path string) (Node, bool, error)      // existence + active revision facts
    Read(path string) ([]byte, error)
    CreateExclusive(path string, data []byte) (created bool, err error)
    UpdateRevision(path string, data []byte) error   // never trashes-then-creates
    Trash(path string) error
}
```

**`CreateExclusive` must not be implemented by checking then uploading.** It maps to `upload -f skip --json`, and the result is read from the transfer summary: `transferredItems == 1` means created, `skippedItems == 1` means refused. **Exit code cannot distinguish these — both are 0.** That is verified, not assumed (probe A6, 2026-07-31).

Immutable objects — packs and their indexes — are written with `CreateExclusive`. A name collision there means the identical content already exists, which is success after byte verification, not an error.

## Concurrency posture

**Single writer per repo is a product precondition, not an enforced guarantee.** This section says exactly what is and is not protected, because an earlier draft of this document overstated it.

**What the lock does.** Acquisition is `CreateExclusive` on `.lock`, carrying a unique operation nonce plus machine identity and timestamp. Because uniqueness is enforced *server-side*, this is not a check-then-act race in the ordinary sense: the loser's write is refused by the server.

**What is verified, and what is not.** Probe A6 confirms the refusal happens and is detectable — **sequentially**. Atomicity under two genuinely simultaneous CLI processes is **unverified**, and belongs to the same class of claim as memo assumptions A1–A3. Until it is tested with two concurrent clients, the lock is *best-effort*.

**What is not protected at all.** Ref *updates* are last-write-wins. There is no conditional write without the SDK, so if two writers ever do run concurrently, one can silently overwrite the other's ref — gcrypt's failure mode. New refs are safer, since `CreateExclusive` genuinely refuses a second creator. **The lock reduces the likelihood of an accident; it does not make concurrent use safe.**

**Takeover and release are the sharp edges.**

- An override can delete A's lock while A is merely slow, after which A may publish behind B, and A may trash B's lock when releasing by path. There is no fencing token to prevent this.
- Machine identity alone cannot distinguish two processes on the *same* machine; the nonce plus local file-lock serialization is required for that.
- **Default: never auto-steal.** Report holder, nonce, and age; require an explicit override flag; and document that the override is only safe once the operator has independently established the holder is dead.

**This is weaker than v1's lock, and the earlier claim of equivalence was wrong.** v1 uses an OS file handle held open with exclusive sharing — a real mutex provided by the kernel. A file on cloud storage written through a CLI is a convention, not a mutex.

## Push

Git calls `list for-push` first; the helper's answer becomes git's entire view of the remote.

**Per-ref transition table.** A single ancestry rule applied to every ref repeats a flaw the memo identified in git-remote-dropbox, so transitions are enumerated:

| Transition | Rule |
|---|---|
| Create branch (no remote ref) | `CreateExclusive`; a refusal means a concurrent creator — report, do not overwrite |
| Fast-forward branch | Require `merge-base --is-ancestor old new`; then `UpdateRevision` |
| Non-fast-forward, unforced | `error <ref> non-fast-forward` |
| Old sha absent locally, unforced | `error <ref> fetch first` — we cannot verify ancestry we do not have |
| Forced update | Skip the ancestry check; still `UpdateRevision`. No lease is honoured; document this |
| Delete (`push :dst`) | `Trash` the ref file. Refuse to delete the branch `HEAD` points at |
| Tag create | `CreateExclusive` |
| Tag update | Requires force, matching git's own rule; no ancestry check applies |
| First push to empty remote | Create `HEAD` deterministically from the pushed branch, else later clones fetch objects but check out nothing |

**Object transfer**, per push batch:

1. `git rev-list --objects <new> ^<remote-tip>…`, excluding only advertised tips that also exist locally. Unknown tips are simply not excluded — the pack is larger, never wrong. A first push to an empty remote therefore packs the full closure and works.
2. Build **one non-thin pack** (`--no-thin`), so it is self-contained with respect to delta bases.
3. `CreateExclusive` the pack, then its `.idx`. **Both must be confirmed present via `Stat` before any ref is written.** A ref whose pack exists but whose index is missing is not fetch-discoverable.
4. Publish refs per the table above.

**Ordering is pack → idx → confirm → ref.** Failures before publication leave orphan packs, which are inert: immutable, content-named, unreferenced.

**Multi-ref batches are not atomic.** Each ref reports its own `ok`/`error`. Partial success is possible and expected; git renders it per-ref.

**Shallow and promisor repositories are refused at startup.** Possessing a tip object does not prove its closure is available locally.

## Fetch

**Sidecars alone are not sufficient.** An `.idx` maps object IDs to the pack containing them; it carries no commit/tree dependency graph. Discovery is therefore iterative:

1. Advertise refs, plus `@refs/heads/<branch> HEAD` for the symref.
2. Download `.idx` sidecars not already cached; build an object-to-pack map.
3. Download the pack containing each wanted object; install with `git index-pack`, writing a **`.keep` file** while the pack is not yet referenced, so a concurrent local repack cannot reap it.
4. Traverse the now-readable objects, collect missing parents/trees/blobs/tags, look them up in the map, and repeat until closed.
5. **Verify connectivity explicitly** — `git index-pack` validates the pack it reads but can succeed while a needed parent lives in a pack we never fetched. Closure must be proven, not inferred.
6. Remove `.keep` once refs reference the objects.

**Resume-safety:** the walk prunes at objects already present locally *with complete history*, so an interrupted fetch resumes rather than restarting.

**Caching is safe only under stated conditions:** packs are content-named and written create-exclusively, so an entry is valid *while the pack exists*. Compaction deletes packs and therefore invalidates cache entries — so the cache needs a generation tag from the outset, even though compaction itself is deferred. The content-hash algorithm is **SHA-256 over the pack bytes**, chosen independently of git's object hash so the two never get confused.

**Pack discovery cost grows linearly with pack count.** Every fetch enumerates the packs folder; new clients download every `.idx`. This is the design's main scaling weakness and the reason compaction is a real milestone rather than a nicety.

## Error handling

**Fail closed.** Anything not confirmed is a failure.

| Class | Behaviour |
|---|---|
| CLI missing / not logged in / session expired mid-operation | Startup probe plus per-call detection; actionable message (`run proton-drive auth login`) |
| Upload refused (`skippedItems == 1`) where creation was required | Treated as contention, not success — never inferred from exit code |
| Upload succeeded, `.idx` missing | Ref is not published; orphan pack left; reported |
| Crash between pack and ref | Orphan pack, inert; retry reconciles against remote state first |
| Ref already at desired sha on retry | Success, not conflict |
| Non-fast-forward / missing old sha | `error <ref> non-fast-forward` / `fetch first` |
| Lock held / stale / ambiguous release | Report holder, nonce, age; explicit override only |
| Corrupt or dangling `HEAD`, invalid ref contents | Refuse with a specific message; never guess |
| Orphaned, corrupt, or mismatched `.pack`/`.idx` | Byte/hash verify on read; treat mismatch as fatal |
| Incomplete closure after `index-pack` | Fatal; fetch does not report success |
| Pack exceeds size limits, quota exhausted, rate limited | Distinct messages; no silent truncation |
| CLI version or JSON-schema drift | Minimum CLI version pinned; fail clearly below it |
| Local disk full, permissions, cancellation, timeout | Reported distinctly |

**Never trust `claimedDigests.sha1`.** Proton flags it `sha1Verified: false` — it is client-asserted (found during A4 verification). Integrity comes from re-hashing.

**Recoverability is a product promise.** Plain packs and ref files mean a repo is reconstructible without the helper. Must be documented *and tested*.

## Testing

Transport is an interface, so an in-memory fake covers the protocol and repository layers without a network or an account. The protocol layer is line-oriented text, so it is table-testable.

**Seams are not sufficient, and v1 proved it** — the `GetNewClosure` bug was invisible to 92 seamed tests and surfaced only in a live run, because every test mocked past the branch containing it. v2 needs a **gated e2e suite** against a real account, skipped by default, using the A4/A5/A6 guardrail pattern.

**Tests must assert preconditions and fail loudly.** The first A5 probe run reported a design-sinking result when nothing had been uploaded, because it could not distinguish "the remote did X" from "my call was malformed."

**Required coverage:** every row of the ref-transition table; every row of the error table; the pack→idx→confirm→ref ordering guarantee; iterative fetch closure including the multi-pack case; connectivity verification catching a deliberately incomplete fetch; `.keep` lifecycle; lock acquisition, contention, and refusal-detection via summary counts rather than exit code; recoverability without the helper; and v1 coexistence on one repo.

## Implementation staging

This is too large for one plan. Four gated stages, each independently valuable:

1. **CLI transport-contract spike.** Prove the semantics the rest depends on: `CreateExclusive` under *two concurrent processes*; whether `merge` keeps the prior revision readable until commit; `--json` shapes for success, skip, and error; upload durability. **Nothing else should be built until this passes** — it is the stage that can still invalidate the design.
2. **Push engine.** Full ref-transition table, pack building, ordering guarantee, lock.
3. **Fetch engine.** Iterative discovery, cache with generation tags, `.keep`, connectivity verification.
4. **Productionisation.** Protocol conformance, packaging and cross-compilation, v1 coexistence, recovery documentation, real-account e2e.

Compaction and retention are a separately approved milestone, but the on-disk generation and cache contracts must reserve for them now.

## Open questions

1. **Pack compaction.** Generation-tagged and reader-safe, to avoid gcrypt's full-reupload cliff and avoid deleting a pack a concurrent fetch is reading.
2. **Retention.** "Recoverable" is not "retained". Force pushes, deletions, and compaction all discard history; a backup-positioned tool must state what is kept.
3. **Hash algorithm for git objects.** SHA-1 repositories only at first, with explicit early refusal for SHA-256 rather than silent misbehaviour.

## What would change this design

- **Stage 1 shows `CreateExclusive` is not atomic under contention.** Then the lock is decorative, and single-writer must be enforced socially or the project waits for the SDK.
- **The Account SDK ships with third-party auth.** Then A1–A3 become verifiable, conditional ref writes become possible, and multi-writer is reachable — but note this changes the **repository** layer too, not only transport: `list` must retain revision tokens and carry them to push time. The earlier claim that only transport would change was wrong.
- **The CLI gains a conditional-write flag.** Multi-writer without the SDK; worth filing as an upstream request.

---

## Revision history

**v2, 2026-07-31 — after Codex + Gemini peer review.** The first draft's core held (single writer, CLI transport, Go) but it overstated its safety in ways worth recording:

- The `.lock` was described as preventing silent corruption. It does not. It is best-effort, verified only sequentially, and ref *updates* remain last-write-wins. The claim that it matched v1's lock was wrong — v1 holds a real OS mutex.
- The transport interface was a generic `Upload`, which hides that `replace` trashes before creating and can destroy a ref on a crash. Replaced with semantic operations.
- Fetch was claimed to work from `.idx` sidecars "alone". It cannot; discovery is iterative, and `git index-pack` does not prove closure. `.keep`, resume-safety, and connectivity checks — all required by the memo — had been omitted.
- The upgrade path was claimed to touch only the transport layer. It touches the repository layer too.
- v1 and v2 would have collided on the remote name `proton`.
- The push section defined branch fast-forwards only, omitting deletions, tags, `HEAD` creation, and multi-ref partial success.
- Several Go arguments were imprecise, and the antivirus claim was unverified and has been dropped.

Probe **A6** was written in response to this review and confirms `CreateExclusive` works sequentially — and that **both success and refusal exit 0**, so detection must read the transfer summary. That trap was predicted by review and is now verified.
