# git-remote-proton — v2 design

**Status:** design approved 2026-07-31, not yet implemented.
**Supersedes:** the open questions in [issue #3](https://github.com/craigstoller/git-proton-backup/issues/3).
**Depends on:** [`docs/research/remote-helper-prior-art.md`](research/remote-helper-prior-art.md) — read that first; this document assumes its findings rather than repeating them.

---

## What this is

A `git-remote-proton` binary that makes Proton Drive a real git remote:

```
git clone proton::/my-files/GitRemotes/myrepo
git push proton main
git fetch proton
```

Git's remote-helper mechanism spawns the binary automatically when it sees a `proton::` URL. No wrapper commands, no bundle files to unpack by hand.

**URL form:** everything after `proton::` is a Proton Drive POSIX path identifying the repo folder, matching the CLI's own path convention (`/my-files/…`). It is taken literally — no implicit root is prepended, because a hidden default is exactly the kind of surprise that makes a tool feel unpredictable. The folder is created on first push if absent.

**This is a different product from v1, not a replacement.** v1 (`GitProtonBackup`) is a Windows PowerShell module that rides the Proton Drive sync app and produces verified `git bundle` backups. It stays as it is. v2 is a cross-platform git remote. They can coexist on the same repo — v1 as the backup path, v2 as a remote — and nothing here changes v1.

## Goals, and non-goals

**Goals**

1. **A real, self-contained tool.** One binary on PATH. Install it, use git normally.
2. **Proper git semantics.** `clone`, `fetch`, `push` as first-class operations, not a bundle workflow.
3. **Cross-platform.** Linux, macOS, Windows. This falls out of the architecture rather than being engineered: v1 is Windows-only because it depends on `cldapi.dll` and the sync app, and v2 uses neither.
4. **Behavioural robustness.** Fail closed. Anything short of a confirmed success is reported as a failure, in git's own idiom.

**Explicit non-goals for v2**

- **Multi-machine concurrent pushes.** This is the single biggest scope decision here; see "Concurrency posture" below.
- **Eliminating the Proton CLI dependency.** Deferred deliberately; see "Why the CLI".
- **Implementing Proton authentication.** Not attempted. The Account SDK will make it unnecessary.
- **Shallow or partial clone.** Documented as unsupported, as git-remote-dropbox does.

## Decision #1, settled

The prior-art memo left three coupled questions: which SDK, whether to vendor its internal upload layer, and where sessions come from. **All three dissolve under the scope decision above.**

The SDK is needed only for the conditional-write primitive (draft revisions, `CurrentRevisionID`), and that primitive exists only to make *concurrent writers* safe. With a single writer, it isn't needed. The CLI's published surface — `list`, `info`, `download`, `upload`, `create-folder`, `trash` — is sufficient for everything a single-writer remote helper does.

So:

| Memo question | Answer | Why |
|---|---|---|
| Which SDK? | **None, for now** | Transport is the CLI |
| Vendor internal upload layer? | **No** | Only needed for CAS, which is only needed for multi-writer |
| Where do sessions come from? | **The CLI's own login** | Proton supports this usage; we implement no auth |

**Language: Go.** Reasons, in order of weight: a single static binary with no runtime is the strongest form of "a real app"; cross-compilation to all three platforms is one build matrix; startup cost matters because git spawns the helper per operation; and `go-proton-api` (a fork of an official ProtonMail repository, used by rclone's Proton backend) exists as a future path if the CLI dependency is ever dropped.

Rust was considered and rejected narrowly — its advantages are CPU-bound and this tool is I/O-bound, so they would be invisible here, while Go wins on cross-compilation simplicity and existing Proton libraries. Python was considered and rejected: it *can* be shipped as a binary via PyInstaller or Nuitka, but the result is larger, slower to start, cannot be cross-compiled, and frequently trips antivirus.

### Why the CLI, and what it costs

**Two facts this rests on, both verified 2026-07-31 against Proton's own documentation:**

- **The CLI runs on macOS, Windows, and Linux.** Goal 3 depends entirely on this. Had the CLI been Windows-only, choosing it as transport would have silently reintroduced the exact platform constraint v2 exists to escape.
- **The CLI is standalone.** It does not require the Proton Drive desktop sync app. So v2 sheds the sync app completely rather than merely avoiding it on the transport path — which was the actual criterion behind "a real, self-contained tool."

**It costs one line in the README:** users install the Proton CLI and run `proton-drive auth login` once.

**It buys:** no dependence on anything Proton has labelled unstable. As of 2026-07-31 the Drive SDK's public interface "may still change"; the only module providing login is `incubating/account`, which Proton states is "not planned to be published or promoted outside of the incubating directory" and exists "only for convenience… until the Account SDK is available"; and a breaking crypto-model migration is targeted for end of 2026 / early 2027. Building v2 on those would mean building on three pre-announced moving parts, and the auth piece is circular — verifying the concurrency assumptions (A1–A3) requires the very session plumbing that is missing.

**Independent corroboration:** rclone's Proton Drive backend, a mature and widely used client, states its cache "is currently built for the case when rclone is the only instance performing operations." A second serious implementation arrived at the same single-writer posture. That is what the platform currently affords.

## Architecture

```
git  ──spawns──▶  git-remote-proton  ──subprocess──▶  proton-drive CLI  ──▶  Proton Drive
     stdin/stdout                     --json
```

Three layers with deliberately separate responsibilities:

**Protocol layer.** Speaks git's line-oriented helper protocol on stdin/stdout: `capabilities`, `list`, `list for-push`, `push`, `fetch`. Knows nothing about Proton. Depends on nothing below it except the repository layer's interface.

**Repository layer.** Decides what moves. Computes incremental object sets via git plumbing, builds and reads packs, reads and writes ref pointer files. Knows git and the storage layout; knows nothing about transport.

**Transport layer.** The only code aware Proton exists. Today it shells out to `proton-drive`. This is the seam that gets replaced if the CLI dependency is ever dropped.

That middle boundary is the load-bearing one: it makes the CLI a swappable detail rather than an architectural commitment.

### Storage layout

One folder per repo:

```
<repo>/
  HEAD                        "ref: refs/heads/main"
  refs/heads/<branch>         one file per ref, containing a 40-hex sha
  refs/tags/<tag>
  packs/<content-hash>.pack   append-only, immutable
  packs/<content-hash>.idx    sidecar index, for pack discovery
  .lock                       advisory single-writer lock
```

**Packs, not loose objects.** The memo's Avoid #2 established that per-object storage is punished on Proton by per-node E2EE overhead and explicit rate-limit guidance. Through subprocess transport that penalty compounds. With packs, a push is one pack upload plus one small ref write regardless of object count.

**Ref files are named directly after refs.** A5 verification (2026-07-31) confirmed Proton's name uniqueness is exact-byte on the plaintext, with no case folding and no Unicode normalisation, so `refs/heads/Foo` and `refs/heads/foo` are distinct. **No escaping layer is required** — where git-remote-dropbox must lowercase entire repo paths to survive Dropbox's case-insensitivity.

**Ref deletion is a plain trash.** A4 verification (2026-07-31) confirmed a trashed node stops occupying its name, so deleting a branch and recreating it works without the user emptying their Proton trash.

## Push

Git calls `list for-push` first, and the helper's answer becomes git's entire view of the remote.

1. **Advertise.** List `refs/` via the CLI, download each ref file, emit `<sha> <refname>` per ref.
2. **Compute the delta.** `git rev-list --objects <new> ^<remote-sha>…`, excluding every advertised sha we also hold locally. Entirely client-side; the remote needs no intelligence.
3. **Validate.** `git merge-base --is-ancestor <old> <new>` unless the push is forced. If the old sha isn't in the local object database, fail early with `fetch first` — we cannot reason about history we don't have.
4. **Upload** one pack plus its `.idx` sidecar, both content-named.
5. **Publish** by writing the ref file.
6. **Report** `ok <ref>` or `error <ref> <reason>` per ref.

Ordering matters: **pack first, ref last.** A ref must never point at objects that aren't fully uploaded.

## Fetch

The problem is locating a wanted object without downloading every pack.

1. **Advertise** as above, plus `@refs/heads/<branch> HEAD` for the symref.
2. **Discover.** Download the `.idx` sidecars and build an object-to-pack map locally. Sidecars are small relative to packs.
3. **Download** only the packs needed to close over the wanted shas.
4. **Install** with `git index-pack`, which re-hashes and verifies everything.

**Idx sidecars rather than a manifest file.** A single mutable manifest listing all packs would be simpler, and under a single writer it would even be safe — but it is precisely git-remote-gcrypt's mistake, and it would become a liability the moment multi-machine arrives. Sidecars remain correct under concurrency, so this choice never has to be revisited.

**Caching is trivially safe** because packs are immutable and content-named: a cached object-to-pack entry can never go stale. Only a fresh clone pays full idx download.

## Concurrency posture

**One writer per repo, enforced by an advisory `.lock` file** carrying machine identity and timestamp. A second machine gets a clear "another machine holds this repo" message rather than silent corruption.

This is the deliberate scope boundary of v2, and it is honest rather than accidental: v1 already ships the same one-machine-per-repo limit, and rclone's Proton backend assumes the same exclusivity.

**Stale locks are reported, never auto-stolen.** The helper prints the holder and the age and requires an explicit override. Memo Avoid #4 gives the reasoning: an aggressive takeover policy is *worse* than a blocking one, because a slow-but-alive writer misclassified as dead can silently overwrite the machine that stole from it — and the commit-fencing guarantee that would make takeover safe (A2) is still unverified.

**The upgrade path is real and does not invalidate this design.** When the Account SDK ships, verify A1–A3, add revision-conditioned ref writes in the transport layer, and lift the single-writer limit. The protocol and repository layers do not change.

## Error handling

**Fail closed.** Anything that is not a confirmed success is a failure to git.

| Situation | Behaviour |
|---|---|
| CLI missing / not logged in | Startup probe, actionable message (`run proton-drive auth login`) |
| Upload fails | `error <ref> …`; no ref written; no partial success claimed |
| Died between pack and ref write | Orphan pack — harmless by construction: immutable, content-named, unreferenced |
| Retry after ambiguous outcome | Reconcile against current remote state; "ref already at desired sha" is success, not conflict |
| Non-fast-forward | `error <ref> fetch first` |
| Lock held by another machine | Report holder and age; explicit override required |

**No retry loops.** Fail fast and let the user retry, matching git-remote-dropbox's posture and Proton's own guidance that interrupted uploads be user-reinitiated. v1 learned this from the stuck-upload incident recorded in issue #3.

**Never trust `claimedDigests.sha1`.** Node metadata carries a client-asserted digest that Proton explicitly flags `sha1Verified: false` (found during A4 verification). Integrity comes from `git index-pack` re-hashing what it installs — never from remote metadata.

**Recoverability is a product promise, not a nicety.** The layout is plain packs and ref files, so a repo is reconstructible without the helper installed: fetch the packs, `git index-pack` them, create refs from the ref files. This must be documented, and it must be tested.

## Testing

**Transport is an interface**, which is what makes everything above it testable:

```go
type Transport interface {
    List(path string) ([]Node, error)
    Download(path string) ([]byte, error)
    Upload(path string, data []byte) error
    Remove(path string) error
}
```

An in-memory fake covers the protocol and repository layers with no network and no Proton account.

**The protocol layer is unusually testable** — git's helper protocol is lines of text on stdin and stdout. Table-driven tests feed commands and assert responses.

**Seams are not sufficient, and v1 proved it.** The `GetNewClosure` SessionState bug was invisible to all 92 seamed tests and surfaced only in a real end-to-end run, because every test mocked past the branch containing it. v2 therefore needs a **gated e2e suite** against a real account in a dedicated folder, skipped by default, using the guardrail pattern established by the A4/A5 probes (path assertion with no override, no remote path as a parameter, automatic cleanup).

**Tests must assert preconditions and fail loudly.** The first A5 probe run reported a design-sinking `COLLIDED` result when nothing had been uploaded at all, because it could not distinguish "the remote did X" from "my call was malformed." Any test that interprets an error as data is worse than no test.

**Required coverage:** protocol conformance; incremental push correctness; pack discovery and fetch closure; the ordering guarantee (ref never precedes pack); lock behaviour including the stale-lock path; recoverability without the helper; and every error-table row above.

## Open questions

These are deliberately unresolved and do not block implementation:

1. **Pack compaction.** Append-only packs accumulate. Compaction must be generation-tagged and reader-safe to avoid gcrypt's full-reupload cliff and to avoid deleting a pack a concurrent fetch is reading. Not needed for a first working version; needed before the tool is recommended for large repos.
2. **Retention.** "Recoverable" is not "retained". Force pushes, ref deletions, and compaction all discard history. A backup-positioned tool must state what is kept and for how long.
3. **Hash algorithm.** SHA-1 only initially, with an explicit early refusal for SHA-256 repositories rather than silent misbehaviour.
4. **CLI output stability.** The `--json` shape is a dependency. Worth pinning a minimum CLI version and failing clearly below it.

## What would change this design

Stated so the decision can be revisited honestly rather than defended:

- **The Account SDK ships with a usable third-party auth story.** Then dropping the CLI dependency becomes attractive, and A1–A3 become verifiable, and multi-machine becomes reachable.
- **The CLI gains a conditional-write flag.** That would make multi-machine possible without the SDK at all, and is a small, well-motivated upstream feature request worth filing.
- **A4/A5 turn out to be wrong under conditions we did not test.** Both were verified with single-file, single-client probes. Neither was tested under contention — which is fine for single-writer, and would need revisiting for multi-writer.
