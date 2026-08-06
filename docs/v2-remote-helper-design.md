# git-remote-proton — v2 design

**Status:** design v6.4, revised 2026-08-06 during Stage 4 implementation. v5 was settled after four rounds of Codex + Gemini peer review; v6, v6.1, v6.2 and v6.3 each change exactly one mechanism, and each only because implementation proved the original unimplementable, unenforceable, or worse than what shipped. v6.4 is different in kind: it documents Stage 4's shipped additions — the CLI version allowlist and the `--set-head` operation — rather than a single corrected mechanism. See the revision history.
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
  packs/pack-<git-pack-hash>.pack   immutable, content-named, never rewritten
  packs/pack-<git-pack-hash>.idx
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

**Ref names map to remote paths directly, but NOT to local ones.** A5 (2026-07-31) verified Proton's *remote* name uniqueness is exact-byte, with no case folding or Unicode normalisation — so no escaping is needed on the remote side. That is not the whole problem. The CLI takes a **local file path** as its upload source, and treats a local path containing `{` as a glob and expands it, while git happily accepts `refs/heads/a{b,c}`. Windows additionally reserves names like `con`, which git also accepts. **C13 (2026-08-01) confirms the globbing hazard is live on the pinned 0.7.0 build**, not only on 0.4.6 as previously recorded: a local file literally named `a{b,c}` fails with `No paths matched`.

> **SUPERSEDED 2026-08-01 — neutral local staging is not expressible on this CLI.**
>
> This section previously specified that v2 **stages every ref write through a neutral
> temporary local filename** and "uploads it to the desired remote name," so that the ref name
> never appears in a local path. **Stage 2 implementation proved that impossible.** `filesystem
> upload localPath... parentPath` takes a **parent** path, not a target file path, and has no
> `--name` flag — the remote node is *always* named after the **local** basename (**C11**). The
> desired remote name is simply not an input the command accepts.
>
> Upload-then-`rename` was evaluated and rejected (**C12**). `rename` does exist and does refuse
> an already-taken name with exit 1, so it could stand in for `CreateExclusive` — but it cannot
> serve `UpdateRevision`, because renaming onto an existing ref requires trashing that ref
> first. That is precisely the destructive `replace` pattern this document rejects for being
> able to destroy a ref on a crash. A hybrid would also still need the name validation below,
> since a ref created that way still could not be *updated* later.
>
> **The two goals — never put a ref name in a local path, and write to a chosen remote name —
> are not jointly achievable on this transport.** Writing to the intended name wins, because
> without it nothing works at all.

**So v2 stages every ref write under the target's own leaf name** in a private temporary
directory, and uploads that to the target's parent folder. The local basename therefore equals
the remote leaf name. This is the only mechanism that serves **both** `CreateExclusive` — whose
server-arbitrated name refusal is what the A6/A7 lock guarantee rests on — and `UpdateRevision`,
which merges in place when the basenames match (**C14**).

**The cost is paid explicitly, never silently.** A leaf name that cannot be expressed as a local
staging path is **rejected with a named reason**, exactly as this document already requires for
names the CLI cannot express as a *remote* path. The rejected set is small, because git itself
already forbids space, control characters, `?`, `*`, `[`, `~`, `^`, and `:` in ref names. What
remains is leaf names containing `{` or `}`, and Windows device names (`con`, `nul`, `com1`…).
Refusing those at creation is also the *consistent* choice: such a ref could never be updated on
this transport anyway, so accepting the create would promise something the update path cannot
keep. Ref names are still validated with `git check-ref-format` first.

**This is a transport-imposed constraint, not a preference.** If the CLI ever gains a `--name`
flag on `upload`, neutral staging becomes possible again and this restriction should be lifted.

**Pack naming: git's own pack name, remotely and locally.** `<git-pack-hash>` is what
`git pack-objects` prints, which is **the checksum of the pack's contents** — precisely, of
every byte of the header and object entries, which git then appends as the pack's trailing
checksum, computed with the repository's hash algorithm (SHA-1 for the repositories v2 supports
today). The trailer is not part of its own input, so a file differing *only* in its trailer
recomputes to the same name; `index-pack --verify` is what rejects that, which is one reason the
two checks below are both required. The remote name is byte-identical to the name `index-pack`
gives the same pack locally.

> **REVISED after Stage 2 (see v6.2).** This previously specified `<sha256>.pack`, the SHA-256
> of the pack file's bytes, "deliberately distinct from git's object hash so the two are never
> confused." Stage 2 shipped git's naming; the two are reconciled here in favour of the code.
> **Both schemes are byte-content-addressed** — the choice is which digest, not whether the
> name is a checksum.

Verified, because an earlier draft of this section got it wrong: packing the same 42 objects at
`pack.compression=1` and `=9` produces two different names, and each name equals the last 20
bytes of its own `.pack`. So the name tracks the bytes, not the object set.

Two reasons, and neither is dedup:

1. **It is free and already correct.** `pack-objects` prints the name; nothing else has to be
   computed, and Stage 2 ships it. A SHA-256 name is not expensive — it can be computed while
   the pack is streamed rather than by re-reading it — but it is strictly more code for a
   different digest of the same bytes.
2. **A downloaded pack keeps its name.** `index-pack` recomputes the identical checksum from
   the identical bytes, so a pack fetched from the remote lands locally under the name it had
   remotely. That makes the fetch cache, the `.keep` stem rule, and every log line agree
   without translation.

**What this does NOT buy, stated because the first draft claimed it:** deduplication of packs
built from the same objects. Two packs with identical object sets but different compression,
git version, or delta choices have different bytes and therefore different names under **either**
scheme. They will both be uploaded and both kept forever, since packs are never rewritten.
`Refused` on a pack upload therefore means a file with **the same content checksum** is already
present — which is the strongest claim a name can make, and it is not the same as proven byte
identity, both because the trailer is outside the checksum's input and because SHA-1 collisions
exist at all. Byte identity is established by Push's explicit comparison against the candidate
this push built — not by the refusal, and not by the two checks below, which validate a file
against its own name and its own structure rather than against what we meant to send. And the
refusal will fire less often than a naive reading suggests.

**The verification consequence, corrected.** Because the name *is* a checksum, it stays usable
as one, and the two available checks are complementary rather than redundant:

- Recomputing the pack checksum and comparing it to the basename proves **the file is the one
  the name claims**. `index-pack --verify` does not establish that on its own.
- `index-pack --verify` proves the pack is **internally well-formed** — trailer, object
  decoding, and agreement with its adjacent index. A basename comparison does not establish
  that.

Both are required on read; neither substitutes for the other.

**Honest limit on the digest.** Git's pack checksum is SHA-1 for a SHA-1 repository, so it is
strong against accidental corruption and weak against a deliberate collision. That is the
right trade only under this design's stated threat model — a single writer's own Drive, where
the adversary would have to be Proton or a compromised account, both of which defeat far more
than the pack name. If that assumption ever stops holding, the fix is **not** a SHA-256 sidecar
file: the adversaries named above — Proton, or a compromised account — can replace a sidecar
along with the pack it describes. Defending against them needs the digest bound to something
they cannot rewrite, such as a signature the client verifies with a key never stored on the
Drive. That is a different design, not a stronger hash, and it is out of scope here.

**The original rule's stated concern is met by the `pack-` prefix.** It guarded against
confusing a remote pack name with a git object hash. `pack-<hash>` is unambiguous, and it is
the name a reader already recognises from `.git/objects/pack/`.

**The `.idx` is named after the pack, not after itself, and that has a consequence.** It
borrows the pack's checksum for its stem because git requires the stems to match — but its own
bytes are not what the name commits to, and git can write more than one valid index for a given
pack (v1 and v2 encodings differ). So two byte-different indexes can legitimately claim the same
name, and `Refused` on an `.idx` upload does **not** by itself prove the remote copy is the one
this push would have written. Two normative consequences:

- **v2 pins the index format by passing `--index-version=2` explicitly**, never by relying on
  the default. `pack.indexVersion` is user-configurable, and this design pins the Proton CLI but
  inherits whatever git the user has — so "we write version 2" is a rule with no mechanism
  unless the flag is on the command line. Note also that `index-pack --verify` validates an
  index *in whatever version it already is*; it does not require v2, so it cannot be the
  enforcement point.
- **A `Refused` `.idx` is validated by the pair check**, not accepted on the strength of its
  name. See the reconciliation rule under Push.

The pack half needs no index-version caveat: its name is a checksum of its own contents.

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
    ReadTo(path string, localDir string) error         // streams INTO an existing dir, named after the node's remote basename
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

**Immutable objects** (packs, indexes) use `CreateExclusive`. `Refused` there means only that **the name is already taken** — it is not evidence about the bytes behind it, and it is never success on the strength of the refusal alone. It becomes success only after Push's reconciliation rule, which differs by member: a `.pack` is compared byte-for-byte against the candidate, because its name is its content checksum; a `.idx` is not, because its name is borrowed from its pack and more than one valid index can carry it. See Push for the full rule.

## CLI version allowlist

**Shipped Stage 4.** The seam is CLI-transport construction: before the first filesystem command
on either the protocol path (`run()`) or `--set-head` (`runSetHead()`), the helper runs
`proton-drive --version` once per process and matches the CLI's identity token against a
compiled-in certified list — today exactly one build, `transport.CertifiedCLI =
"cli-drive@0.7.0+5174900c"` (`internal/transport/cli.go`). Matching is exact-token, not
containment or a prefix: `IsCertified` requires the first whitespace-delimited field starting
`cli-drive@` to equal `CertifiedCLI` exactly, so `cli-drive@0.7.0+5174900c-extra` is **not**
certified. The SDK line in `--version`'s output is recorded in diagnostics but never matched —
only the CLI line's token gates anything.

**Refuse by default, override loudly.** `transport.EnforceCertified` is the enforcement point:

- **Mismatch, unparseable `--version` output, or a nonzero `--version` exit → refuse**, before
  any filesystem command, naming what was found (or that it could not be determined) and what is
  certified.
- **`GPB_UNCERTIFIED_CLI=1` → proceed**, printing a loud stderr warning naming the untested (or
  undetermined) version on every helper invocation. The override is never remembered or cached
  across processes — it is read fresh from the environment each run.
- **A binary that never started refuses regardless of the override.** "Never started" is
  detected structurally — `verr` non-nil and not an `*exec.ExitError` — rather than narrowly as
  `exec.ErrNotFound`, because the narrow check let permission-denied, a bad executable format, or
  `CLI.Exe` pointing at a directory all fall through to the generic (overridable) branch and
  proceed, which is exactly the binary-synthesis the override must never do (fix round 1). The
  override covers a CLI that ran and reported something uncertified; it does not conjure a CLI
  that never ran at all.

**Where it does and does not run.** The check fires wherever a CLI transport is constructed: the
protocol path and `--set-head`, both before touching the repo. `--version` constructs no
transport and is therefore never gated — the diagnostic for "your CLI isn't certified" must not
itself require a certified CLI to print. A CLI that never starts at all still fails exactly as it
always has (spawn error, fail closed) — the allowlist adds no new failure path there, only a new
refusal for a CLI that DOES start but is not the certified build.

**Threat-model framing: a compatibility gate, not a provenance check.** The allowlist defends
against accidental drift — an auto-updated or hand-upgraded CLI silently changing semantics
under the helper, exactly the failure class this document already establishes between 0.4.6 and
0.7.0: the `activeRevision` payload shape, the byte-identical-rewrite auto-skip, and
concurrent-startup behaviour (all above), plus the local-path globbing hazard confirmed live on
0.7.0 (probe C13, above). It does **not** defend against a hostile binary on PATH: a spoofed
`--version` defeats the check trivially — the gate's own shim test proves it — and that is
accepted as out of scope, because a helper that already trusts the CLI with every byte of repo
data has no meaningful defence against a malicious one regardless of what `--version` claims.

**Defense in depth, behind the front door — the design/code contradiction this closes.**
`parseNodeJSON` (`cli.go`) tolerates two `activeRevision` shapes: 0.7.0's unwrapped form and
0.4.6's `{ok, value}` wrapper. That tolerance is KEPT, not removed by the allowlist — but its
role changes. Read on its own, a parser that accepts two CLI versions' output reads as a floor,
contradicting this document's stated rule, unchanged since v4: "exact versions, not a floor or
prefix." The allowlist resolves that reading: it is the front door, gating on the exact certified
build before any filesystem command runs; the parser's version tolerance is defense in depth
*behind* that door. The two are not in tension once ordered this way, and this document now says
so explicitly rather than leaving the contradiction implicit — see also the two updated rows in
the error table, below, which previously recorded this as deliberately unimplemented.

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
| Delete (`push :dst`) | `Trash`; refuse to delete the branch `HEAD` points at, naming `git-remote-proton --set-head <url> <branch>` as the remedy |
| Tag create | `CreateExclusive` |
| Tag update | Requires force, matching git's rule; no ancestry check |
| `refs/notes/*`, `refs/replace/*`, other valid namespaces | **NOT IMPLEMENTED IN STAGE 2 — rejected outright with a named reason.** See the v6.1 note below; the rule as written here is unimplementable while `ListRefs` is non-recursive |
| Pseudorefs and unsupported destinations | Explicit rejection with a named reason |
| First push to empty remote | **The marker is already written** — bootstrap creates it at `list for-push`, before the lock, per the normative initialisation ordering. This row previously said the push writes it, which contradicted that ordering. The push's only first-time responsibility is `HEAD`, per the deterministic rules below (deferred past Stage 2) |

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

**Object transfer per batch:** compute `git rev-list --objects <new> ^<remote-tip>…`, excluding only advertised tips that also exist locally (unknown tips are simply not excluded — larger pack, never wrong); build **one non-thin pack** (`--no-thin --index-version=2`); `CreateExclusive` the pack, then the `.idx`; **confirm both before any ref is written**. A ref whose index is missing is not fetch-discoverable.

**`Stat` is sufficient confirmation only for a `Committed` upload. A `Refused` one must be reconciled, and this is where the design previously contradicted itself.** The transport contract says a refusal on an immutable object is success "after byte verification" — but the publication path specified only `CreateExclusive` then `Stat`, and a `Stat` proves a node exists at that path, nothing about what is in it. A remote `.idx` that is corrupt, truncated, or a legitimately different encoding of the same pack would satisfy it, and v2 would then publish a ref that no future client can fetch. That is precisely the outcome the pack→idx→confirm→ref ordering exists to prevent, so the gap is closed here rather than left to the reader:

**The two members are checked differently, and conflating them produces rules that cannot be satisfied.** A `.pack` is named by its own content checksum; a `.idx` borrows that name and is not addressed by its own bytes. So:

- **`Committed`, either member** — `Stat` to confirm presence, and proceed. The bytes are the ones this push just sent.
- **`Refused` on the `.pack`** — download it and require **byte equality with the candidate this push built**, plus its recomputed content checksum equal to its basename. Byte equality is the right bar here precisely because the name *is* the content checksum: a matching name that does not match byte-for-byte means corruption or a tampered trailer, which is exactly what this catches.
- **`Refused` on the `.idx`** — download it and require that it is a **valid index for the already-validated remote pack**, by `index-pack --verify` on the pair. **Do not require byte equality, and do not require version 2.** The v2 pin governs what this helper *writes*, so its own uploads are deterministic; it cannot govern what some other writer put there. Since this document already establishes that two byte-different indexes can legitimately index the same pack, demanding byte equality would make a legitimate remote index fatal — and unrecoverably so, because the object is immutable and cannot be replaced. The property that actually matters is that the remote index correctly indexes the remote pack, not that it matches ours.
- **`Refused` on the `.pack` with no remote `.idx` present** — not an error. That is an orphan from a writer that crashed between the two uploads, and this push repairs it: the pack is verified as above, then this push's own `.idx` uploads normally and completes the pair. Worth stating because it is easy to lose: a future cleanup that deletes orphan packs would delete the thing this path repairs.
- **`Ambiguous`, either member** — as everywhere else: unknown, reconcile by reading remote state, never assumed either way.

Any failure of the checks above is fatal for that ref and reported as a corrupt remote object naming the path — **never** worked around by overwriting, because packs are immutable and an overwrite would invalidate every ref already pointing into that pack.

**"One pack" is an assumption about the user's git config, not a guarantee, and v2 must not inherit it silently.** `pack.packSizeLimit` makes `pack-objects` split its output and print *several* names; `pack.compression`, delta and reuse settings change the bytes and therefore the name. Only the Proton CLI is version-pinned by this design — git is whatever the user has, configured however they configured it. So the helper **overrides the settings that would break the invariant on the `pack-objects` command line — `-c pack.packSizeLimit=0` and `--index-version=2` — and treats more than one emitted name as a hard error**. The size override must be the config form: `--max-pack-size=0` is read as *unset*, at which point git falls back to the very `pack.packSizeLimit` being overridden rather than parsing the first line and proceeding. Failing loudly here is cheap; a second pack silently dropped would publish a ref whose objects are half-uploaded, which is the exact failure the pack→idx→confirm→ref ordering exists to prevent.

**Forcing a single unlimited pack is a deliberate trade with a ceiling, not a free win.** It buys the one-pack invariant the whole publication path is built on, and it costs an upper bound. **The two directions bind differently and must not be conflated:** a push is capped by whichever is smaller of local temp-disk capacity and Proton's largest accepted single *upload*, because the whole closure goes up as one file; a clone is capped by local disk alone, since fetch downloads many remote packs — each already small enough to have been uploaded — and consolidates them locally, never sending anything. Both bounds are currently **unmeasured**. Establishing them is a Stage 4 productionisation item: certify the push-pack and clone-local ceilings separately against the pinned CLI, publish them as product limits, and make each failure message name the limit it hit rather than surfacing a raw transfer error.

**Two costs the reconciliation path adds, which the ceiling measurement must account for.** Neither is a defect — both are the price of the correct rule — but a bound measured on the happy path would be wrong by a multiple. First, **peak temp-disk on a `Refused` upload is 2–3× pack size, not 1×**: the push holds its own pack and index, `publishPack` downloads the full remote pack to compare, and `publishIdx` needs the pack adjacent to the index (a hard link where the filesystem allows one, a full copy where it does not). Since the size pin guarantees a single unbounded pack, that multiplier applies to the whole repository. Second, **a multi-ref batch pointing at the same commit takes the reconciliation path for every ref after the first**: `haves` is deliberately not updated between refs, so `git push proton-v2 main:main main:backup` rebuilds a byte-identical pack for the second ref, collides on the name, and pays a full download plus a streaming compare where a naive implementation would have done a `Stat`.

**Listing is immediately consistent for creation, which is what the Fetch cache depends on** (probe **C15**). A node is visible to a `list` from a fresh CLI process as soon as its upload or `create-folder` returns — 11 trials across files and folders, no misses. The Fetch cache below is keyed on pack name and existence, so it rests on exactly this property, and it holds.

> **A withdrawn claim, kept visible on purpose.** An earlier revision of this paragraph asserted that `list` can be stale after a `trash` and that `info` is therefore authoritative over it. That came from **one** observation during a cleanup check and **did not reproduce** — 0 stale results in 6 follow-up trials across an empty folder and a populated tree. The anomaly is recorded in C15 as seen rather than characterised, with its candidate explanations unresolved. Do not design defensive re-listing around it without reproducing it first. Recorded rather than deleted because the reasoning matters more than the conclusion: a normative contract asserting a property from a single observation would have sent Stage 3 hardening against a hazard that may not exist.

**Ordering: pack → idx → confirm → ref.** Failures before publication leave orphan packs, which are inert.

**Multi-ref batches are not atomic.** Each ref reports its own `ok`/`error`; partial success is expected.

**Shallow and promisor repositories are refused**, and — because a fresh `git clone --depth` is not yet shallow when the helper starts — the helper must poison on the full set - `depth`, `deepen-since`, `deepen-not`, `deepen-relative`, `update-shallow`, `filter`, `from-promisor`, `no-dependents`, `refetch`. A startup check alone is insufficient.

## Fetch

The helper owns the closure. Git's `fetch` capability is defined as transferring "objects **reachable from**" the named ones, and under `check-connectivity` "**the helper** must output `connectivity-ok`". Git does not iteratively request missing objects.

1. Advertise refs plus `@refs/heads/<branch> HEAD`.
2. Download `.idx` sidecars not already cached; build an object-to-pack map.

   **The map builder must read every index version Push is willing to accept.** Push deliberately accepts a valid remote `.idx` it did not write, which may be version 1 even though this helper only ever writes version 2 — so a v2-only parser here would accept an index at push time and then be unable to fetch the repository it just wrote to. Build the map with git plumbing rather than a hand-rolled version-specific parser, so the accepted set is git's and cannot drift from it.
3. Download packs containing wanted objects into a temp area.
4. **Traverse.** An `.idx` maps object IDs to byte offsets and carries no connectivity data, so the object graph must be walked. Index each downloaded pack into a **temporary object directory** and expose it via `GIT_ALTERNATE_OBJECT_DIRECTORIES`, keeping unverified remote objects out of the real database until closure is proven.

   **The temp directory must have git's own object layout, not a flat one.** An alternate is an *objects* directory, so packs are only found at `<tmp-objects>/pack/pack-<hash>.{pack,idx}`. Dropped flat into `<tmp-objects>/`, git silently does not see them — `rev-list` then reports every object missing, the loop downloads packs it already holds, and the no-progress rule below fires on what looks like a corrupt index mapping. The failure is loud but the cause is not, so the layout is normative rather than an implementation detail.

   **The walk uses git plumbing, not a hand-written parser.** `git cat-file --batch` decompresses and identifies objects but does *not* enumerate commit parents, tag targets, or tree entries — an earlier draft claimed it removed the need for a parser, which was wrong. Instead run `git rev-list --objects --missing=print <wants>` against the temp directory: git performs the traversal and reports missing OIDs prefixed with `?`. Feed those back through the object-to-pack map, download, repeat. This also handles gitlinks and malformed objects with git's own semantics rather than reimplemented ones.
5. **Consolidate the closure into ONE pack, install it, and emit git's single `lock`.** Git's `transport-helper.c` retains only the **first** `lock <path>` response, so a multi-pack install cannot be protected by multiple protocol locks.

   **This decision reversed twice and is now settled on the merits.** v2 used consolidation; v3 replaced it with helper-managed `.keep` files swept at the start of the next fetch; v4 restores consolidation. The helper-managed variant is **not crash-safe**: if a fetch dies after installing a pack but before git updates refs, the next fetch removes that pack's protection *before* refs exist, and a concurrent `git gc` can delete it between connectivity verification and git's ref write — leaving refs pointing at missing objects. It also leaks permanently if the user never fetches from that remote again. Git's protocol lock exists precisely to cover the install-to-ref-update interval and is released only after both complete. Corruption beats a repacking pipeline as a thing to avoid.

   **Naming is normative:** git recognises a `.keep` only when its stem exactly matches the adjacent pack (`packfile.c:368-384`). The installed files are `pack-<git-pack-hash>.{pack,idx,keep}` using git's own naming from `index-pack`.

   **What v6.2's shared naming does and does not remove.** A pack *downloaded unchanged* keeps its name locally, because `index-pack` recomputes the same checksum from the same bytes — so no rename or translation is needed for it. It does **not** remove the object-to-pack map: that is built from the downloaded `.idx` sidecars and is what discovery runs on regardless of naming. Nor does it apply to the **consolidated** pack this step installs, which is a new pack built from many, whose checksum legitimately matches none of the remote names it was derived from. Shared naming is a convenience at the edges, not an end-to-end identity.
6. **Verify connectivity against the exact requested wants**, against the downloaded packs in the temp area **BEFORE the consolidated pack of step 5 is installed**, and before reporting success — an explicit missing-object-fatal traversal rooted at the wants, not a generic `fsck`, since the wants are not yet referenced by any ref. **Verifying before the install, rather than after it, is deliberate:** a failure discovered after the install has already written the pack and its `.keep` leaves that pack protected by a `.keep` no git operation will ever remove — an unreclaimable residue costing disk until a human deletes the file — whereas a failure in the temp area leaves the local object store untouched. This reverses the ordering an earlier draft of this step specified; see the v6.3 entry.

**Termination is explicit.** The loop maintains a set of already-downloaded packs and a set of still-missing OIDs. Each round must either download a pack not previously downloaded, or resolve at least one missing OID. **A round that does neither is fatal** — that is the signature of a stale or corrupt index mapping a missing OID into a pack already held, and without this check the loop runs forever.

**Resume-safety:** prune the walk at objects already present locally *with complete history*, so an interrupted fetch resumes.

**Caching** is valid only while a pack exists. v2 never deletes a pack, so an entry cannot go stale; the cache is keyed on pack name and existence. Compaction will invalidate that assumption, and will bump the marker's `version` when it does.

**Discovery cost grows linearly with pack count.** Every fetch enumerates `packs/`; a new client downloads every `.idx`. This is the design's main scaling weakness and the reason compaction is a real milestone.

## Utility modes: `--version` and `--set-head`

**Shipped Stage 4.** Beyond the protocol path git invokes (`git-remote-proton <name> <url>`), the
helper also serves two direct-invocation "utility modes," dispatched from `os.Args[1]` before any
protocol I/O begins (`dispatchUtility`, `cmd/git-remote-proton/main.go`): `--version` and
`--set-head <address> <branch>`.

**`--set-head` ships as a direct-invocation utility mode, not a protocol extension.** Remote
helpers have no protocol verb for changing a remote's default branch, and the user — not git —
is the natural invoker of that change.

**Dispatch matches a closed set, not a prefix.** `os.Args[1]` must EQUAL `--version` or
`--set-head` exactly; every other value takes the protocol path unchanged, even one that begins
with `--`. A prefix match (`--*`) would misroute a remote whose *configured name* begins with
`--` — git passes the remote's name as `os.Args[1]` on the protocol path — but the closed set
narrows that collision to exactly two strings. **`--version` and `--set-head` are documented here
as reserved: a remote cannot be named either of those two strings and used with this helper**,
because those exact `os.Args[1]` values are claimed by utility-mode dispatch instead of being
passed through as a remote name.

**Utility-mode stdout is safe because the argv shapes are disjoint.** Every other command in this
helper writes protocol data to stdout and diagnostics to stderr; utility mode writes its result
to stdout instead — `--version` prints `git-remote-proton <version> (certified CLI:
<CertifiedCLI>)`, `--set-head` confirms with `HEAD is now <ref>` — because git never invokes the
helper with either of these two `os.Args[1]` values during a real remote-helper session, so
utility-mode stdout can never interleave with the protocol stream.

**Arity is checked before anything else runs.** `--set-head` takes exactly two further arguments
(`<proton::address> <branch>`); the wrong count refuses with a usage line on stderr (`usage:
git-remote-proton --set-head <proton::address> <branch>`) before `CanonicalRoot`, the allowlist,
or any repo access. A bare invocation with no arguments at all falls through `dispatchUtility` (it
requires at least `os.Args[1]`) and keeps the helper's existing "must be run by git as a remote
helper" usage error.

**`--version` constructs no CLI transport and is therefore never allowlist-gated** (see CLI
version allowlist, above) — the diagnostic for an uncertified CLI must not itself require a
certified CLI to run. `--set-head` constructs one and is gated like everything else.

### `--set-head` grammar

`<address>` is anything `CanonicalRoot` accepts, with or without the `proton::` prefix.
`<branch>` is a short name (normalised to `refs/heads/<branch>`) or a full `refs/heads/…` ref;
anything outside `refs/heads/` is refused — HEAD points at branches only, the same rule
`WriteHEAD` already enforces. The hierarchical-name refusal is checked first, ahead of the
general staging-path check, so it gets its own named reason: a leaf containing `/` or `\` is
refused naming Stage 5, not yet supported. The remaining leaf must pass the same
`checkStageableLeaf` rules the push path enforces — no braces, no Windows reserved device names,
not empty/`.`/`..`. Matching against remote branches is exact byte comparison, no case folding.

### Flow, lock, and outcome semantics

Order: dispatch (arity-checked) → `CanonicalRoot` → allowlist check (transport construction) →
`RequireMarker` → `AcquireLock` → **verify `refs/heads/<branch>` exists remotely, on every
invocation, before any short-circuit** → read `HEAD` under the lock → if it already names the
verified target, report success and stop (idempotent, no upload) → `UpdateHEAD` → `Release`.

The existence check runs before the idempotence short-circuit deliberately — a round-3
peer-review finding both engines caught independently: reading `HEAD` first and short-circuiting
on a match, without first confirming the branch still exists, would let a `HEAD` that already
names a since-deleted branch report success instead of refusing. Verifying first closes that. The
same read-`HEAD`-under-lock step is also what makes a re-run after an `Ambiguous` outcome
self-reconciling: a re-run either finds the write already landed (reports success) or makes a
fresh attempt — no special retry mode is needed.

The lock is released via `defer` on every exit path, the same "release on every exit path" rule
and the same stale-lock/holder-nonce semantics as every other lock-holding operation in this
design (see Concurrency posture, above). Within the lock, verify-then-write is serialised against
every other v2 writer — branch deletion takes the same lock. Non-v2 actors (the web UI, other
Proton clients) can still mutate the repo at any time; that is the same accepted model every
existing v2 operation lives with, not a new exposure `--set-head` introduces.

**`UpdateHEAD` carries the overwrite; `WriteHEAD` is untouched.** `WriteHEAD` is backfill-only by
pinned contract (`TestWriteHEADNeverOverwritesExistingHEAD`) and never overwrites an existing
symref. `UpdateHEAD` (`internal/repo/head.go`) is the new function that does, and it branches on
whether `HEAD` currently exists: it `Stat`s the path first, then calls `UpdateRevision` (`upload
-f merge`, per probe C14) when `HEAD` is present — the ordinary overwrite case — or
`CreateExclusive` when it is absent, which is the headless-repo rescue this document already
defines elsewhere: a repo with no `HEAD` file at all, either because its first push was
tags-only (the ref-transition table's own headless case) or because an operator deleted the
`HEAD` file in the web UI to repair a corrupt one (`SetHead`'s own doc comment names this path).
Either way it stages the same local basename `HEAD` used everywhere else, and both
branches feed the same closed-set outcome handling as the marker and ref paths: `Committed` falls
through to read-back verification; `Refused` is impossible from another v2 writer under the lock,
so it is read as a non-v2 actor and refused rather than adopted — the user asked for THIS branch;
`Ambiguous` likewise refuses, both with "re-run to reconcile"; and any unrecognised outcome fails
closed rather than guessing.

**The branch-delete refusal names this as the remedy.** The ref-transition table's delete row, in
Push above, and `push.go`'s refusal text now both name `git-remote-proton --set-head <url>
<branch>` as the fix for "refusing to delete the branch HEAD points at" — previously the refusal
named the problem with no way out.

## Error handling

**Fail closed.** Anything unconfirmed is a failure.

| Class | Behaviour |
|---|---|
| CLI missing, logged out, session expired mid-operation | The CLI-version allowlist gate (above) refuses outright, before any filesystem command, for a CLI that never starts at all (missing binary, permission denied, not an executable) — that case is no longer a warning followed by an opaque bootstrap failure. A session that is valid at startup but expires mid-operation is not covered by that gate and is detected per-call, where the CLI's own error surfaces through the affected transport method's actionable message |
| CLI version not on the certified allowlist | **Shipped Stage 4** (see CLI version allowlist, above): refuse to run before any filesystem command, naming what was found versus what is certified — exact versions, not a floor. `GPB_UNCERTIFIED_CLI=1` overrides with a loud per-invocation stderr warning, never cached across processes. **This row previously said NOT IMPLEMENTED as of Stage 2.1, deliberately** — Stage 2.1 shipped the version probe as an advisory warning only, deferring a hard refusal to this stage as a policy call, since a silent behaviour change across CLI versions is what broke v1 in the first place. That deferral is now resolved |
| `proton-drive --version` exits 0 but its output does not match a certified token | Same refusal as above, naming the found output verbatim (quoted, bounded to 200 chars) — treated as uncertified, not as version-undetermined, because the process ran and its output is trustworthy text even though it does not match. The override still applies, and its warning names that same found text |
| `proton-drive --version` exits nonzero (the binary started and ran) | Refusal treated as version-undetermined: the process ran, but a nonzero exit forfeits trust in anything it printed, so nothing is named as "found." The override still applies; its warning says the version could not be determined rather than naming a wrong one |
| `failedItems > 0` with exit code 0 | Treated as failure — never inferred from exit status |
| Unparseable or unexpected `--json` shape | `Ambiguous`; reconcile against remote state before retry |
| Mutation timed out after the remote may have committed | `Ambiguous`; read back before any retry |
| Upload refused where creation was required | **Depends on the object.** For a *mutable* name — a ref, the marker, the lock — contention, not success. For an *immutable* one — a pack or index — not a verdict at all: it becomes success or failure only after Push's per-member reconciliation |
| Crash between pack and ref | Orphan pack, inert; retry reconciles first |
| Ref already at desired sha on retry | Success, not conflict |
| Missing or unrecognised format marker on a non-empty folder | Hard refusal; never guess |
| Missing versus empty remote | Distinguished; empty is initialisable, missing is an error |
| File/folder collision at an expected path | Fatal with the specific path |
| Malformed ref name or ref-file contents | Fatal; never coerced |
| Corrupt or mismatched `.pack`/`.idx` | **Two checks, each run as early as it can be and always before the data it validates is trusted; either failure is fatal.** (1) **Checksum against basename**, immediately on downloading a `.pack` — it needs nothing else, and it is the only check proving the file is the one the name claims. (2) **`git index-pack --verify` on the pair**, as soon as both members are local — it is the only check proving the pack is well-formed and agrees with its index, and it necessarily cannot run on a member that arrives alone. Note fetch downloads `.idx` sidecars first, before their packs: that is permitted, because the lone index is used only to *plan* which packs to fetch, never as a source of truth about objects. **No object from a pack may be trusted until both checks have passed for that pack.** "Before install" would be the wrong gate — fetch never installs the downloaded packs, it consolidates them into a new one |
| Pack present, `.idx` missing | Incomplete pair — the ref is not published. **Not necessarily an error to be reported and left:** a push that finds its own pack already there with no index completes the pair, repairing the orphan (see Push). Reported only when this push is not the one that can repair it |
| `.idx` present with no pack | Incomplete pair, and the unrepairable direction — an index cannot be validated without its pack, and v2 never overwrites an immutable name. Reported, ref not published |
| Valid `.pack`/`.idx` pair no ref points at | Orphan from a crash between upload and publication. Inert and left in place — v2 never deletes a pack, and collection waits for the compaction milestone |
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

**v6.4, 2026-08-06 — Stage 4 ships: CLI version allowlist, `--set-head`, and the docs update that closes a standing design/code contradiction.**

- **The CLI version allowlist is now enforced, not advisory.** The error table's "CLI version not
  on the certified allowlist" row said, as of Stage 2.1, "NOT IMPLEMENTED... deliberately... The
  helper warns on stderr and continues." `transport.EnforceCertified` (`internal/transport/cli.go`)
  now refuses to run before any filesystem command unless the CLI reports exactly `CertifiedCLI`,
  with `GPB_UNCERTIFIED_CLI=1` as the documented override. Both existing error-table rows are
  updated to match, and two rows are added distinguishing an unparseable-but-clean-exit
  `--version` (refused, naming the found text verbatim) from a nonzero exit (refused as
  version-undetermined — nothing printed is trustworthy) — the two are NOT the same refusal
  wording, which an earlier draft of this revision collapsed into one row and a fix round
  corrected (see the amendment note below). A new "CLI version allowlist" section states the
  mechanism and its threat-model framing.
- **This closes the `parseNodeJSON` contradiction that had stood since Stage 2.** `parseNodeJSON`
  (`cli.go`) tolerates both the 0.4.6 wrapped `{ok,value}` shape and the 0.7.0 unwrapped shape —
  version-tolerant parsing that read as a floor, while this document's stated rule (unchanged
  since v4) is "exact versions, not a floor or prefix." The allowlist is now that front door; the
  parser's tolerance is defense in depth behind it, not a substitute for it. Both are kept, and
  the design/code disagreement is resolved in favour of the shipped front-door enforcement.
- **`--set-head` closes the one user-facing gap Stage 3 left**: a v2 remote's default branch could
  not be changed in-tool. It ships as a direct-invocation utility mode, dispatched from a closed
  `os.Args[1]` set alongside `--version` — not a protocol extension, since remote helpers have no
  protocol verb for it and the user, not git, is the natural invoker. The new "Utility modes"
  section states the dispatch rationale, the branch-name grammar, the lock and
  verify-before-short-circuit ordering (branch existence checked on every run, before the
  idempotence short-circuit — a round-3 peer-review finding, both engines), and the
  `UpdateHEAD`/`WriteHEAD` split: `UpdateHEAD` carries the overwrite, branching on whether `HEAD`
  currently exists (`UpdateRevision` when present, `CreateExclusive` for the headless-repo rescue
  when absent), while `WriteHEAD` stays untouched and backfill-only, pinned by
  `TestWriteHEADNeverOverwritesExistingHEAD`.
- **The branch-delete refusal now names its own remedy.** The ref-transition table's delete row,
  and `internal/repo/push.go`'s refusal message, both name `git-remote-proton --set-head <url>
  <branch>` as the fix for "refusing to delete the branch HEAD points at" — previously the
  refusal named the problem but not the way out.
- **`--version` and `--set-head` are documented as reserved.** Because dispatch matches
  `os.Args[1]` against a closed set rather than a prefix, only a remote literally named
  `--version` or `--set-head` could collide with utility-mode dispatch — and this document now
  states plainly that those two strings cannot be used as a remote name with this helper.
- README gained a v1/v2 coexistence section, per the Stage 4 spec's component 4: the side-by-side
  table, the dedicated-root guidance (never inside `GitBackups`), the restore-contract honesty
  (v1: git only; v2: git + helper + certified CLI), and the trash note scoped to homonym cleanup
  with an evidence-capture step, not a blanket "empty your trash."

**Amendment (fix round 1, 2026-08-06) — three accuracy defects found on review, all confirmed against code and corrected in place rather than superseded as v6.5, since v6.4 had not yet been treated as settled.**

- **README's restore-needs table cell contradicted its own prose.** The table said v1 restore
  needs "git only, from any device signed into your Proton account"; the prose two paragraphs
  down (correctly) said no account is needed at restore time, because the bundle itself is the
  backup. The table cell was wrong, not the prose — fixed to "git only, from any machine... no
  account needed."
- **`UpdateHEAD` was described as unconditionally using `upload -f merge`.** The shipped function
  (`internal/repo/head.go:116-126`) `Stat`s `HEAD` first and branches: `UpdateRevision` (merge)
  only when `HEAD` already exists, `CreateExclusive` when it is absent — the headless-repo rescue
  this document defines elsewhere. The original v6.4 text described only the merge branch, which
  is also not this document's own cross-check table's wording ("CAS via
  `CreateExclusive`/`UpdateRevision`," Component 2 of the Stage 4 spec). Both branches are now
  described, here and in the "Utility modes" section itself.
- **The error table collapsed two distinct refusals into one row with the wrong shared wording.**
  Per `EnforceCertified` (`cli.go:453-477`), "could not be determined (%v)" fires only when
  `Version()` returned an error from a process that STARTED and exited nonzero (`*exec.ExitError`);
  a start failure is refused earlier, by the separate never-started check, with its own "Proton
  CLI could not be started" message, and never reaches this wording at all. A clean exit whose
  output does not match a certified token takes a third path: `found` stays the actual quoted,
  bounded output, never "could not be determined." The merged row claimed the first two cases got
  the same wording; split back into two rows matching the Stage 4 spec's own error table.

**Amendment (fix round 2, 2026-08-06) — the fix-round-1 amendment above introduced its own
inaccuracy, caught on re-review.** Its third bullet originally said "could not be determined
(%v)" fires when `Version()` "returned an error — a nonzero exit or a start failure," as if both
reached the same code path. They do not: a start failure is caught EARLIER, by
`EnforceCertified`'s own never-started check (`verr != nil && !errors.As(verr, &exitErr)`), which
returns "Proton CLI could not be started: %w" immediately and never falls through to the
`found`/"could not be determined" logic at all. Only a process that STARTED and exited nonzero
(`verr` wraps an `*exec.ExitError`) reaches that wording. The split error-table rows above
(`--version exits 0 but...` / `--version exits nonzero...`) already stated this correctly; only
this prose sentence conflicted with them, and it is corrected in place above rather than
appending yet another amendment layer for a two-word phrase.

**v6.3, 2026-08-02 — Fetch verifies connectivity BEFORE the install, not after it.**

- Fetch step 6 said the connectivity check runs "after all imports and before reporting
  success". Stage 3a's shipped `internal/repo/fetch.go` runs it **before** `consolidateAndInstall`
  writes anything into the local object store, and does so deliberately. The step now says so.
- The reason is the `.keep`. Step 5's install writes `pack-<hash>.{pack,idx,keep}`, and git removes
  that `.keep` only after it has updated refs on the strength of a successful fetch. A verification
  failure discovered *after* the install therefore leaves a pack nothing references and a `.keep`
  nothing will ever remove — an unreclaimable residue costing disk until a human deletes the file.
  Verified first, the same failure leaves the local store untouched, which is what the step's own
  "keeping unverified remote objects out of the real database until closure is proven" (step 4)
  was always aiming at.
- Nothing else moves: the steps keep their numbers, the traversal is still rooted at the exact
  wants rather than being a generic `fsck`, and the install is still one consolidated pack with
  one protocol `lock`. Recorded because a normative document that disagrees with shipped, gated
  code is how the *next* implementer reintroduces the defect — the same reason v6.2 exists.

**v6.2, 2026-08-01 — pack naming is git's; and the first draft of this entry was wrong.**

- The storage layout specified `packs/<sha256>.pack`, the SHA-256 of the pack file's bytes,
  "deliberately distinct from git's object hash so the two are never confused." Stage 2 shipped
  `pack-<git-pack-hash>` instead — the plan baked it in and the live gate confirmed it on a real
  account. A **normative** layout rule and shipped behaviour disagreed, and the disagreement was
  found by the final whole-branch review rather than by anything in the build.
- **A first draft of this revision justified keeping git's name on a false premise** — that
  `pack-objects` names a pack by the hash of its sorted object list, and therefore that two
  packs with the same objects would collide and deduplicate. Peer review challenged it and a
  direct experiment settled it: the same 42 objects packed at `pack.compression=1` and `=9`
  produce two different names, and each name equals the last 20 bytes of its own `.pack`. **The
  name is the checksum of the pack's bytes.** Both schemes are byte-content-addressed; the
  choice was only ever which digest. Recorded here rather than quietly corrected, because the
  wrong version was written into this document and briefly argued for.
- Resolved in favour of the code on the reasons that survive: `pack-objects` prints the name for
  free, and a pack downloaded unchanged keeps its name locally because `index-pack` recomputes
  the same checksum from the same bytes. The "no mapping layer" argument is also narrower than
  first claimed — the object-to-pack map comes from the `.idx` sidecars regardless, and the
  consolidated fetch pack matches no remote name at all.
- **The verification rule moved in the opposite direction from the first draft.** Since the name
  *is* a checksum, it stays usable as one. The error table now requires **both** checks — the
  basename comparison, which alone proves the file is the one the name claims, and
  `index-pack --verify`, which alone proves the pack is well-formed. Neither substitutes for the
  other, and the first draft's claim that `index-pack` was "always the stronger check" was wrong.
- **The digest is weaker and that is now stated:** git's pack checksum is SHA-1 for a SHA-1
  repository, sound against accidental corruption and not against a deliberate collision. It is
  the right trade only under this design's single-writer, own-Drive threat model.
- **A second review round found the publication path contradicted the transport contract**, and
  that is the most consequential change in v6.2. The contract has always said a refusal on an
  immutable object is success "after byte verification", but Push specified only
  `CreateExclusive` then `Stat` — and a `Stat` proves a node exists, not what is in it. A
  corrupt or differently-encoded remote `.idx` would have satisfied it and v2 would have
  published a ref no client could fetch. Push now specifies a per-outcome reconciliation rule,
  with the `Refused` branch requiring byte equality, basename agreement, index version 2, and
  `index-pack --verify` on the pair.
- Two adjacent gaps are **specified** here, and deliberately not claimed as done: the `.idx`
  borrows the pack's stem but is not addressed by its own bytes, so v2 must pass
  `--index-version=2` explicitly rather than trusting a user-configurable default; and
  `pack.packSizeLimit` can make `pack-objects` emit several packs, so the helper must override
  it and treat multiple emitted names as a hard error. **Stage 2's shipped code does neither**,
  and neither does it reconcile a `Refused` upload — it `Stat`s. Those three are Stage 3 work,
  recorded so they are not mistaken for shipped behaviour.
- Forcing one unlimited pack is now stated as a trade with an **unmeasured ceiling**, and the
  push and clone directions bind differently — a push is capped by Proton's largest accepted
  upload as well as local disk, a clone by local disk alone. Certifying both bounds is a Stage 4
  item.
- **A second engine (Gemini) then found what three rounds of the first had not**, including one
  contradiction those rounds had *caused*: the `Refused` reconciliation demanded byte equality on
  the `.idx`, while the storage layout says in as many words that two byte-different indexes can
  legitimately index the same pack. A legitimate remote index would therefore have been fatal —
  and unrecoverably so, since the object is immutable. The rule is now split by member: byte
  equality for the `.pack`, whose name *is* its content checksum, and validity-against-that-pack
  for the `.idx`, whose name is borrowed. The same round found the check list was unsatisfiable
  as written (index-version and pair checks applied to a refused *pack*, which may have no remote
  index yet), that a refused pack with no remote index is an **orphan this push repairs** rather
  than an error, that the error table's "before install" was wrong because fetch never installs
  downloaded packs, that the ref-transition table still had the push writing the marker after v5
  moved that to bootstrap, and that an alternate object directory must carry git's `pack/`
  layout or the traversal silently sees nothing. Recorded because it is the clearest evidence on
  this project for why the second engine exists: the two engines' misses were not the same
  misses.
- **Timing kept this cheap.** No Stage 3 code had been written against the old rule. A pack in
  the shipped naming did exist on the account during the Stage 2 gate, but that demo remote was
  trashed and no repository retains the old layout — so this is a correction, not a migration.
  One stage later it would have been a migration.

**v6.1, 2026-08-01 — Stage 2 rejects the other namespaces rather than supporting them conservatively.**

- The ref-transition table promised `refs/notes/*`, `refs/replace/*` and "other valid
  namespaces" would be create-exclusive on create with force required to move. **Stage 2 rejects
  them outright instead**, with a named reason, at the top of `pushOne` before anything is
  packed. Two reasons. First, the rule is unimplementable as written while `ListRefs` is
  non-recursive: those namespaces are nested, so the advertisement cannot see them, `exists`
  is always false, the ancestry rule never applies, and `WriteRef` would upload into a parent
  folder that does not exist. Second, without the guard the failure was expensive and
  misleading — a full pack was built and uploaded to paid storage before `WriteRef` failed with
  a message naming neither the ref shape nor the limitation, leaving an orphan pack that Stage 2
  has no way to collect. `refs/stash` was worse: written, reported `ok`, and then permanently
  invisible to the advertisement.
- This also implements the error table's "Pseudorefs and unsupported destinations → Explicit
  rejection with a named reason," which had no mechanism before.
- The deviation is in the fail-closed direction and is a narrowing, not a change of intent.
  Stage 3 owns recursive listing and parent creation; when it lands, this row becomes
  implementable and should be revisited against the conservative rule originally stated here.

**v6, 2026-08-01 — first revision forced by implementation rather than by review.**

- **Neutral local staging for ref writes is not expressible on the pinned CLI, and v5 specified
  it as normative.** `filesystem upload` takes a *parent* path and has no `--name` flag, so the
  remote node is always named after the local basename (probe C11). Every ref write, the marker,
  and the lock would have landed under their temp-file names — and the in-memory fake would not
  have caught it, because the fake keys on the full target path. Upload-then-`rename` was
  evaluated and rejected: it can replace `CreateExclusive` but not `UpdateRevision`, which would
  need the old ref trashed first (probe C12). Ref writes now stage under the target's own leaf
  name, and leaf names that cannot be expressed as a local staging path are rejected with a named
  reason rather than mangled. Probes C11–C14 are recorded in `stage1-results.json`.
- **The globbing hazard that motivated neutral staging is confirmed live on 0.7.0** (probe C13),
  not merely inherited from 0.4.6 as v5 recorded. The mitigation changed; the hazard did not.
- This revision was **not** peer-reviewed. It is a single mechanism replacement compelled by
  probe evidence, with the alternative enumerated and rejected on the record.

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
