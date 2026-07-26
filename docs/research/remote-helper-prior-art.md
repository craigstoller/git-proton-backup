# Remote-helper prior art: git-remote-dropbox, and what Proton Drive actually exposes

**Research memo for [issue #3](https://github.com/craigstoller/git-proton-backup/issues/3) (v2 direction: `git-remote-proton`).**
Date: 2026-07-23. Status: research only — no implementation decisions are made here.

Sources examined (all read in full or in relevant part, pinned to the versions studied):

- [git-remote-dropbox](https://github.com/anishathalye/git-remote-dropbox) @ `a18031a` (2026-06-26) — complete source (~1,100 lines of Python), `DESIGN.md`, README, and the [author's project page](https://anishathalye.com/git-remote-dropbox/)
- [gitremote-helpers(7)](https://git-scm.com/docs/gitremote-helpers) — git's own remote-helper protocol documentation (read from the local git 2.x install)
- [git-remote-gcrypt](https://github.com/spwhitton/git-remote-gcrypt) @ `72f93b3` (2024-12-29) — README/design skim for contrast
- [Proton Drive SDK](https://github.com/ProtonDriveApps/sdk) @ `e0ad37e` (2026-07-22) — the open-source (MIT) SDK that Proton's first-party clients are being migrated onto; includes the source of the `proton-drive` CLI itself. API semantics below are read from its OpenAPI-derived type definitions, upload manager, and tests
- Proton Drive CLI (`proton-drive.exe`, local install) — full help text for every subcommand. **Live probing was not possible**: the CLI reported "You need to login first" in this session, and per the research ground rules (strictly read-only against a real account) no login or write of any kind was performed. Nothing in this memo depends on live behavior; where a claim rests only on SDK type definitions rather than observed behavior, it is flagged.
- [rclone's Proton Drive backend docs](https://rclone.org/protondrive/) and Proton's SDK blog posts ([July 2025 preview](https://proton.me/blog/proton-drive-sdk-preview), [January 2026 update](https://proton.me/blog/drive-sdk-january-2026))

---

## Part 1 — How git-remote-dropbox works

### 1a. The remote-helper protocol, as actually implemented

Git's remote-helper mechanism is a line-oriented stdin/stdout protocol: git spawns `git-remote-<scheme>` as a child process, sends commands one per line, and reads responses. The helper never touches the git wire protocol — for dumb storage you implement the `push` and `fetch` capabilities and git does all negotiation-adjacent work itself via the helper's answers.

git-remote-dropbox's entire protocol surface is one loop (`helper.py::Helper.run`):

| Command from git | Helper's response |
|---|---|
| `capabilities` | `option`, `push`, `fetch`, blank line |
| `option verbosity N` | `ok` (all other options: `unsupported`) |
| `list` / `list for-push` | one `<sha> <refname>` line per remote ref; on plain `list` also `@refs/heads/X HEAD` for the symref; blank line terminates |
| `push +src:dst` (batched, blank-line-terminated) | after attempting each: `ok <dst>` or `error <dst> <reason>` per ref, then blank line |
| `fetch <sha> <name>` (batched) | downloads objects into the local odb, then a blank line |
| (blank line) | exit |

Points that matter for a reimplementation:

- **`list` is the foundation of everything.** Git calls it before both fetch and push (`list for-push`), and the helper's answer is git's entire view of the remote. Crucially, git-remote-dropbox *also* uses its own `list` pass to capture concurrency metadata (see 1c).
- **The helper writes objects directly into the local object database** during fetch (via `git hash-object -w`) and reads them out during push (via `git cat-file`). Git doesn't hand the helper packfiles in `push`/`fetch` mode; the helper shells out to git plumbing for everything object-shaped (`rev-list --objects`, `cat-file`, `merge-base --is-ancestor`, `hash-object -w`). `GIT_DIR` is set by git so the plumbing targets the right repo.
- **Per-ref error reporting is the safety valve.** A failed CAS surfaces as `error <dst> fetch first` — git prints it exactly like a rejected push from a real server, and the user retries after fetching. No helper-side retry loop exists for contention, and none is needed.
- **stderr is free-form** (progress, `error:`/`info:`/`debug:` messages); stdout is protocol-only. On Windows stdout must be put in binary mode (there is a dedicated `stdout_to_binary()` for exactly this).
- Unsupported: shallow clone (`option depth` is never accepted), push options, atomic multi-ref pushes (`option atomic`), `check-connectivity`.

### 1b. Storage layout and incremental transfer

The Dropbox folder is laid out as a **bare repository with loose objects only**:

```
<repo-root>/
  HEAD                          "ref: refs/heads/main\n"
  refs/heads/<branch>           "<40-hex sha>\n"       (one file per ref)
  objects/<2-hex>/<38-hex>      zlib(type SP size NUL payload)  — byte-compatible with git's loose format
```

- **No packs, no deltas, no GC ever.** Every object is an individual file. The layout is deliberately recoverable without the helper: copy `refs/` and `objects/` into a fresh `.git/` and it just works (the README documents this as a feature).
- **Incremental push** is computed entirely client-side: `git rev-list --objects <src> ^<each-remote-ref-sha-we-have-locally>`. The "what does the server have" set is approximated by *the refs the server advertises whose objects exist locally* — no server-side smarts, no per-object existence probing. Objects are content-addressed, so concurrent writers uploading the same object are harmless (`WriteMode.overwrite` is used for objects, correctly).
- **Incremental fetch** is a client-side recursive walk: start from the wanted sha, download the object, parse it (`commit` → tree+parents, `tree` → entries, `tag` → target), recurse, and **prune the walk at any object that already exists locally with complete history** (`git cat-file -e` + `git rev-list --objects` as a history-completeness check — the latter exists to make aborted-then-resumed fetches safe). Submodule gitlinks are explicitly skipped during tree parsing.
- The costs of this layout are the tool's main known complaints: object-count-bound performance (one API round-trip per object, mitigated by 20 parallel threads), loose-object bloat in the local repo after clone (README: run `git gc --aggressive`), no shallow clone, and impracticality for large-history repos.

### 1c. The concurrency mechanics, in precise detail

This is the part issue #3's comment asks about, and the source confirms it exactly — with one refinement worth stealing: **there are two distinct atomic write modes, not one.**

The revision capture happens during `list`. `get_refs()` lists the `refs/` folder recursively, downloads every ref file, and records `self._refs[name] = (rev, sha)` — **the Dropbox revision id of each ref file, paired with the sha it contained, at advertisement time**. This is the "observed state" that the subsequent push is conditioned on.

Ref writes then go through `_write_ref(new_sha, dst, force)` (helper.py:431), which picks a write mode:

1. **Force push** → `WriteMode.overwrite`. Unconditional; force means force.
2. **Ref exists on the remote** → first a *local* fast-forward check (`git merge-base --is-ancestor <old-sha> <new-sha>`; if the old sha's object doesn't even exist locally, fail early with `fetch first`), then upload with **`WriteMode.update(rev)`** — Dropbox's compare-and-swap: "write this file only if its current revision is still `rev`". If any other client updated the ref since our `list`, the API rejects the write.
3. **Ref doesn't exist on the remote** → **`WriteMode.add`** — an *atomic create-exclusive*: fails if a concurrent writer created the file first. This closes the race two machines pushing the same *new* branch would otherwise have. (The symbolic ref `HEAD` is written the same way: `add` on creation, `update(rev)` for changes.)

Every conditional upload passes **`strict_conflict=True`**. This flag is load-bearing: without it, Dropbox "helpfully" resolves some conflicts by auto-renaming (`file (conflicted copy).txt`) instead of failing, which would corrupt the scheme. The helper needs *hard failures*, not hosting-provider conflict UX.

The failure path: any `UploadError` from the conditional write is translated to `error <dst> fetch first` back to git; the user fetches (sees the concurrent update), and retries. The retry is *the human's* (or the porcelain's) — the helper never spins.

**The invariants this protects:**

- A ref file only ever transitions *from a state a pusher has actually observed and fast-forward-validated*. Two concurrent non-force pushes cannot silently clobber each other: exactly one CAS wins; the loser gets a clean, git-idiomatic rejection.
- Object uploads need no coordination at all (content-addressing), so the only serialization point is the tiny ref file — contention is minimal and the window is one HTTP request.
- What it does **not** protect — and this list is longer than the DESIGN.md admits: force pushes (unconditional `overwrite` by design — which also means `--force-with-lease` is not atomically honored: git checks the lease against the helper's `list for-push` advertisement, then the helper overwrites unconditionally, leaving a race window a real server's receive-pack doesn't have); delete-vs-update races (`files_delete` is unconditioned; a concurrent delete error is deliberately tolerated); and every-ref-is-a-branch semantics (the fast-forward rule is applied to all refs uniformly — tags and other namespaces get no special handling). Ref deletion refuses only one thing: deleting the branch `HEAD` points at. One further inherited hazard: Dropbox paths are case-insensitive (the helper canonicalizes the repo path to lowercase), so `refs/heads/Foo` and `refs/heads/foo` collide on the remote — a ref-name/provider-filename equivalence mismatch any reimplementation must check for its own provider.

### 1d. Error handling, auth, performance

**Error handling.** Transport retries exist only for Dropbox `InternalServerError` (3 attempts) and chunked-upload offset desync (resynced from the server's `correct_offset`). Everything else is fail-fast: the helper prints `error: ... (run with -v for details)` and exits non-zero; git reports the operation failed. On fetch, every downloaded object is **verified by re-hashing** (`git hash-object` recomputes the sha; mismatch is fatal) — dumb storage is not trusted to return what was stored. Worker threads communicate failures via poison-pill sentinels so a single bad download aborts the whole fetch deterministically.

**Auth.** OAuth2: `git dropbox login` runs the flow and stores a refresh token in `~/.config/git/git-remote-dropbox.json` (with config-format versioning and migration in the parser). Multiple named accounts are supported (`dropbox://work@/path`), and a legacy token-in-URL form exists but is discouraged. The helper validates the token with a whoami call at startup and produces actionable errors ("log in first with ..."). Connections are per-thread and lazily created.

**Performance profile.** 20 parallel transfer threads; uploads ≤50 MB are single-shot, larger ones use Dropbox upload sessions in 50 MB chunks. The known complaints follow directly from the loose-object design rather than from bugs: clones of large repos are slow (round-trip per object), fetch of deep new history is walk-latency-bound (an object can only be discovered after its referrer is downloaded and parsed), and there is no shallow clone to bail out with. The author's framing — "maintains all guarantees of a traditional Git remote" — is accurate for the case that matters most (contended non-force pushes to existing refs) but should be read with the exclusions in 1c above; and the implementation is SHA-1-only (40-hex parsing, `2/38` object paths, no `object-format` capability), a limitation a reimplementation should either lift or reject explicitly at startup.

---

## Part 2 — Contrast: git-remote-gcrypt (what happens without a CAS)

git-remote-gcrypt solves a different problem (GPG-encrypted repos over rsync/sftp/arbitrary git remotes) but is the cautionary tale for the concurrency question. Its remote state is a single **manifest file** (encrypted+signed) listing all refs and all encrypted packfiles. Pushing means: pack the delta, upload the pack, regenerate the *entire* manifest, and **PUT it unconditionally** — rsync/sftp offer no conditional write, so the last writer wins.

The documented consequence (README, "Known issues"): *"Every git push effectively has `--force`."* Two machines pushing concurrently silently drop one machine's refs — exactly the failure mode issue #3's one-machine-per-repo limit exists to prevent. The project's own mitigation is a config flag that refuses non-explicit force pushes; the underlying lost-update problem is unsolved a decade in, because the storage layer beneath it has no CAS. Secondary lessons: its packfile-stacking design (each push = one new encrypted pack listed in the manifest) is the natural *efficient* layout for dumb storage, but it forces periodic repacks whose cost is a full-history re-upload — a UX cliff git-remote-dropbox avoids by never packing at all.

The two projects mark the two ends of the design space: **dropbox = per-ref files + CAS, safe contended pushes, object-granular traffic; gcrypt = manifest + packs, efficient transfer, unsafe refs.** A `git-remote-proton` wants the safety of the first with the transfer economics of the second — and Part 4 notes the one piece of design work that combination genuinely requires (pack discovery without a gcrypt-style mutable manifest).

---

## Part 3 — What Proton Drive actually exposes

### 3a. The CLI surface (v1's transport)

The full command inventory (from `proton-drive.exe` help; the CLI's TypeScript source lives in the SDK repo under `cli/`): `auth login/logout`; `filesystem list/info/create-folder/upload/download/rename/copy/move/trash/restore/delete/empty-trash`; `sharing` and `invitation` groups. `--json` output exists; paths are POSIX-style with node-UID escape hatches; `filesystem info` returns "full node metadata including latest revision details."

The load-bearing observation: **upload's conflict handling is a policy knob, not a primitive.** `filesystem upload -f merge|keep-both|replace|skip` decides what to do *when the name already exists* — nothing is conditioned on a revision the caller observed. `rename`/`move`/`copy` fail if the target name exists (no rename-over). `auth login` is a browser-based flow; the session is cached locally (encrypted SQLite, per the CLI source). So the CLI as-is can express "create if absent" and "replace whatever is there," but **not** "replace only if unchanged since I looked" — building a multi-writer-safe helper on CLI subprocess calls is not viable. (Caveat: conflict-strategy exit codes and `--json` error shapes were not live-verified this session.)

### 3b. The API/SDK surface — the conditional-write answer

**The answer to issue #3's load-bearing question is yes at the interface level.** A conditional-write primitive is documented in the API surface — potentially stronger than Dropbox's — and it is visible in the open-source SDK that Proton's own clients use. (What the interface documents vs. what contention testing must still confirm is separated out below; the confidence note and assumptions A1–A5 at the end of this section are part of the claim.) Evidence, from the SDK's OpenAPI-derived types (`client/js/src/internal/apiService/driveTypes.ts`) and upload manager:

1. **Atomic create-exclusive (= Dropbox `WriteMode.add`).** Creating a file draft in a folder is rejected server-side with code 2500 — "file or folder with same name already exists" — including when the occupant is only another client's *draft*. The conflict payload identifies the occupant (`ConflictLinkID`, `ConflictDraftRevisionID`, `ConflictDraftClientUID`), and the SDK surfaces it as `NodeAlreadyExistsValidationError` with the existing node's UID. Name uniqueness is enforced on the server against *hashed* names, so E2EE doesn't weaken the guarantee.

2. **Revision-conditioned write (= Dropbox `WriteMode.update(rev)`).** Updating a file's content means creating a **draft revision** and committing it — and `CreateRevisionRequestDto` carries **`CurrentRevisionID`**. The documented failure modes are exact quotes from the API types:
   - draft creation: HTTP 409 — *"Conflict, the submitted revision is no longer up to date or another draft is open."*
   - the single-shot small-file revision endpoint (`POST .../files/{linkID}/revisions/small`), whose metadata *requires* `CurrentRevisionID`: HTTP 409 — *"Conflict, the passed CurrentRevisionID is no longer up to date."*

3. **The draft plausibly functions as an exclusive write lock, which would be *more* than Dropbox offers.** Only one draft revision can be open per file; a competing writer's draft creation fails until the draft is committed or deleted. Dropbox's CAS is a single atomic instant; Proton's would be a *critical section* — between draft-create (conditioned on the current revision) and commit, no other writer can even begin. `ClientUID` lets a client mark drafts so conflicts identify their owner (`ConflictDraftClientUID`). Note the SDK's own conflict handling carefully, because it is sharper-edged than "cleanup": on a draft conflict it **immediately deletes the existing draft and retries** whenever the draft carries the *same* ClientUID — no age or liveness check — and does the same for *foreign* drafts when the caller opts in (`overrideExistingDraftByOtherClient`, a per-upload metadata flag in the JS SDK, mirrored in the Swift layer). Two concurrent operations sharing a ClientUID will therefore stomp each other's live drafts; a helper must give each push operation a distinct ClientUID (or serialize pushes per machine) and treat the UID as part of the concurrency design, not a label. What the evidence establishes is exclusion *against other drafts*; whether the draft also fences the other operations that can mutate a node (trash, move, rename, delete) is not established and is on the verification list below.

**One real gap vs. Dropbox, and two candidate closures.** The SDK's *public facade* cannot express the conditioned write at all: `getFileRevisionUploader(nodeUid, …)` returns an opaque uploader whose single `uploadFromStream` call performs draft-create (with a freshly-read `CurrentRevisionID`), block upload, and commit internally — there is no public seam to pass the revision observed at `list` time *or* to insert any step between draft acquisition and commit. (Worse for our use case: files under 128 KiB — every ref file — can take a feature-flagged single-shot "small file" path with no draft lifecycle exposed at all.) So both closures currently live **below the public facade**, in the internal upload manager whose seams are real but unstable across SDK releases — and both are **unverified constructions at this point, not proven recipes**:

- *Literal CAS at the internal layer:* the internal upload API accepts `currentRevisionUid` explicitly (`createDraftRevision`), so a helper vendoring that layer can do a Dropbox-style conditioned write directly. Conceptually the cleanest mapping.
- *The lock, not the compare (the more interesting candidate):* use the internal `createDraftRevision` / `commitDraft` seam to open the draft revision (acquire the exclusive lock, CAS'd against *whatever is current*), then **download and verify the ref file's content under the lock** — if it still contains the sha the fast-forward check was based on, commit; otherwise delete the draft and answer `error <dst> fetch first`. *If* draft exclusivity holds and *if* the committed content stays readable under an open draft, verify-then-commit closes the observation gap — the same invariant git-remote-dropbox protects, achieved with a lock instead of a compare.

Either way, the durable fix is upstream: a public way to pass an expected `CurrentRevisionID` (or a public draft-lifecycle handle) is a small, well-motivated SDK feature request — and exactly the kind of concrete third-party need Proton has invited feedback on.

**The load-bearing assumptions, stated as test cases.** The lock construction (and parts of both) rests on behaviors that SDK types cannot prove — type definitions document intended request/response shapes, not backend linearizability. Before any spec commits to this design, a ~50-line script against a **throwaway Proton account** (never the real one) must confirm:

- **A1 — read-under-draft:** downloading a file's *committed* content while another client holds an open draft returns the committed revision (not a block, not the draft). If reads block or go inconsistent under a draft, the verify step collapses.
- **A2 — fenced commit:** after a draft is deleted by another client — whether via foreign override or the SDK's *immediate same-ClientUID* delete-and-retry — the original uploader's commit of that draft **fails** — i.e., commit validates the specific draft-revision UID, not just the node. If commit isn't fenced, takeover enables a silent lost update, which is worse than the liveness problem it solves. (`commitDraftRevision` addresses a `nodeRevisionUid`, which suggests fencing — suggestion is not proof.) Test both the foreign-override case and the same-UID case explicitly.
- **A3 — lock scope:** an open draft actually excludes, or at least is not silently invalidated by, concurrent trash/move/rename/delete of the same node.
- **A4 — trash and the namespace:** a *trashed* ref file does not still occupy its name for create-exclusive purposes — otherwise deleting a branch and recreating one with the same name breaks until the trash is emptied.
- **A5 — name equivalence:** Proton's server-side name-uniqueness (computed over hashed names) is exact-byte on the plaintext — no case folding or Unicode normalization that would make two distinct git refs collide (the Dropbox helper genuinely has this problem; see 1c).

Liveness is the flip side of the lock: an abandoned draft blocks all writers until cleaned up. A helper needs an explicit stale-draft takeover policy — and the safe default is *conservative*: surface the block to the user (whose draft, how old) rather than auto-deleting, with automatic takeover only past a generous age threshold and only once A2 is proven, since a slow-but-alive writer misclassified as dead is exactly the split-brain case A2 exists to catch. Dropbox's design never needed any of this; it is the one genuinely new protocol element `git-remote-proton` must specify.

One edge case the create-exclusive path adds: a draft *reserves* a new file's name before any content is committed. Two machines pushing the same new branch concurrently means the loser gets a name conflict while the winner's push is still in flight — at which point there is nothing to fetch yet. The helper's conflict message must therefore distinguish "fetch first" from "another push is in progress; retry shortly."

For completeness: rename/move are fail-if-target-exists (no rename-over-CAS available), which is fine — the draft mechanism makes rename tricks unnecessary.

*Confidence note:* items 1–3 rest on the SDK's API type definitions (OpenAPI-derived, i.e., generated from Proton's own server spec), its tests (which exercise the "Draft already exists" conflict paths), and corroboration from rclone's independently reverse-engineered backend ("a draft exist — usually this means a file is being uploaded at another client"). That is strong evidence that the *interface* exists and behaves as documented in the common case; it is not evidence about concurrency semantics under contention, which is what A1–A5 exist to establish. Nothing was runtime-verified in this session (read-only constraint; CLI session unavailable).

### 3c. Platform facts that shape the design

- **The SDK is MIT-licensed and personal projects are explicitly permitted**, with published operational requirements: identify honestly via `x-pm-appversion: external-drive-{name}@{semver}-{channel}`, use event-based sync rather than polling/recursive traversal, respect the same rate limits as first-party clients, no Proton branding, and a third-party disclosure when prompting for credentials. A `git-remote-proton` fits these rules comfortably — and the header format even gives it a citable identity (`external-drive-git_remote_proton@…`).
- **Auth is the missing module.** The SDK explicitly excludes authentication, login flows, and session management ("Official Proton Drive clients wire these pieces into the SDK"). The CLI has a working browser-login + encrypted-session implementation, and an `incubating/account` module exists in the repo, but today a third-party tool must bring its own session plumbing. This is the largest engineering unknown — not Drive semantics.
- **A breaking cryptographic migration is scheduled** ("end of 2026/early 2027"): clients on the old crypto model will stop interoperating until upgraded. Anything built now signs up for tracking SDK releases through that break.
- **Languages:** JS/TS and C# are the primary SDK clients (Swift/Kotlin incubating). The CLI itself is TypeScript on the JS SDK — a useful, Proton-maintained reference implementation of exactly the plumbing (session cache, node addressing, transfer queue) a helper needs.
- **Rate limits and traffic shape matter more than on Dropbox.** Every node carries E2EE overhead (per-node keys, signatures, manifest), and Proton's guidelines explicitly discourage many-small-requests patterns. git-remote-dropbox's one-file-per-loose-object layout, ported naively, would be slower and less welcome here than it is on Dropbox.

### 3d. Mechanism-by-mechanism mapping

| git-remote-dropbox mechanism | Proton equivalent | Status |
|---|---|---|
| `files_list_folder(recursive)` over `refs/` at `list` time | SDK folder iteration (small, single folder — fine); event-stream sync for caches | ✅ exists |
| `files_download` of ref files + objects | SDK file download | ✅ exists |
| `WriteMode.add` (create-exclusive) for new refs | Server-enforced name uniqueness on draft creation (code 2500) | ✅ exists, atomic |
| `WriteMode.update(rev)` (CAS) for existing refs | `CurrentRevisionID` on revision creation (409 when stale) **plus** exclusive draft lock; verify-under-lock or internal-layer CAS closes the observation gap | ⚠️ candidate — primitive documented in SDK/API types, concurrency contract (A1–A5) needs runtime verification |
| `strict_conflict=True` (defeat provider conflict UX) | Don't use CLI conflict strategies; SDK surfaces conflicts as typed errors by default | ✅ default behavior |
| Revision id captured per ref during `list` | `activeRevision.uid` in node metadata (listing / `filesystem info`) | ✅ exists |
| Unconditional overwrite for content-addressed objects | `replace` semantics via revision upload (or trash + re-create) | ✅ exists |
| Chunked upload sessions (50 MB) | SDK block-based upload (handled internally, resumable drafts) | ✅ exists |
| OAuth refresh token + `git dropbox login` + config file | **Gap:** no third-party auth module yet; CLI's session code is the reference; `incubating/account` in progress | ⚠️ largest gap |
| Loose-object layout (round-trip per object) | Portable but ill-fitting: per-node crypto overhead + rate-limit guidelines punish many small files | ⚠️ layout should change (packs) |
| Ref deletion (unconditioned delete) | `trash`/`delete` (two-stage) | ✅ exists (two-stage is arguably safer — but see A4: a trashed ref file must not block recreating the name) |
| Shared-folder collaboration | `sharing`/`invitation` command groups + shared-with-me paths | ✅ exists (future option) |

---

## Part 4 — Copy this / avoid this, for a hypothetical `git-remote-proton`

**Copy:**

1. **The two-mode atomic ref write** — create-exclusive for new refs, revision-conditioned update for existing ones — and the *capture-revision-at-`list`-time* discipline that makes the condition meaningful (adapted to the draft-lock construction on Proton, once verified). And go one better than the original: **condition *forced* writes and deletions on the observed revision too** (force should bypass the ancestry check, not the observed-state check). That closes the `--force-with-lease` race and the delete-vs-update race that git-remote-dropbox knowingly leaves open — the CAS costs nothing extra once the machinery exists.
2. **`error <dst> fetch first` as the primary contention story.** No helper-side retry loops; surface the conflict in git's native idiom and let the user fetch. It keeps the helper stateless. Extend it with the two states dumb storage adds: "another push is in flight" (name reserved by an uncommitted draft — retry, don't fetch) and ambiguous outcomes (commit response lost, crash between pack upload and ref publication) — the retry path must reconcile against current remote state first and treat "ref already at the desired sha" as success, not conflict.
3. **Client-side incremental computation via git plumbing** (`rev-list --objects <want> ^<have>`): zero server smarts needed, and v1's bundle engine already trades on the same idea.
4. **Verify-on-fetch** (re-hash every object downloaded; trust nothing dumb storage returns) and **resume-safe fetch** (history-completeness check before pruning the walk).
5. **The recoverability property**: document exactly how to reconstruct a working repo from the raw remote files without the helper installed. For a backup-positioned tool this is not a nicety, it's the product promise. (A pack-based layout can keep it: `git index-pack` + a documented layout is still a five-line recovery.) Recoverability is *not* retention, though — force pushes, ref deletions, and pack compaction all discard history, so a backup-positioned spec must separately state what is retained and for how long (immutable packs kept N days? rely on Drive's revision history? tombstones?) rather than letting "recoverable" imply "nothing is ever lost."
6. **Protocol hygiene details**: binary-mode stdout on Windows, stderr-only diagnostics, per-ref `ok`/`error` reporting, config-file versioning with migration, actionable auth errors, whoami preflight. If the layout is pack-based, also adopt the fetch-side hygiene git's protocol provides for exactly that case: emit `lock <path>.keep` while a fetched pack is not yet referenced (so a concurrent local repack can't reap it), verify with `git index-pack`, and honor `check-connectivity`.
7. From gcrypt (the one thing worth taking): **packfile stacking as the transfer unit** — each push uploads one pack. But this drags in a design obligation gcrypt solved badly: **pack discovery.** `fetch <sha>` must locate which pack(s) contain the closure without downloading everything. Two schemes that don't reintroduce the mutable-manifest problem: (a) *manifest-free* — content-named immutable packs each uploaded with a small `.idx` sidecar; clients list the pack folder and pull only the idx files to build the object→pack map locally; or (b) a manifest that is itself updated through the same CAS'd ref-write protocol. Failed pushes leave orphan packs — harmless because immutable and unreferenced, but compaction must be specified deliberately (generation-tagged, reader-safe, delayed reclamation) to avoid gcrypt's full-reupload cliff and to avoid deleting packs a concurrent fetch is reading.

**Avoid:**

1. **gcrypt's single unconditioned manifest.** Any design where one file describes all refs, written last-write-wins, silently reintroduces "every push is a force push." If a manifest exists at all, it must be per-ref or CAS-protected by the draft-lock recipe, and ref pointers should live in individually-CAS'd files exactly as in git-remote-dropbox.
2. **The loose-object layout on Proton.** It is the right call on Dropbox and the wrong call here: per-node E2EE overhead, per-session rate limits, and the explicit "don't make many small requests / don't recursively traverse" guideline all punish object-granular storage. Store packs (append-only, content-named, immutable), keep a per-ref pointer file, and maintain a local negotiation cache. Accept gcrypt's repack problem as the price and design the repack path deliberately (size-tiered compaction rather than all-or-nothing).
3. **Building the concurrency story on the CLI.** The CLI's conflict strategies (`replace`/`skip`/…) are policy, not primitives; a subprocess-driven helper caps out at gcrypt-grade safety. The CLI remains fine for v1's verify path; v2's writes must go through the SDK.
4. **Unbounded trust in the draft lock's liveness — in either direction.** No takeover policy means one crashed push on machine A wedges machine B indefinitely; an *aggressive* takeover policy means a slow-but-alive machine A gets its draft deleted and (absent proof of A2, commit fencing) can silently overwrite machine B afterward. Default conservative: report the blocking draft (owner ClientUID, age) and require explicit user action or a generous age threshold; enable automatic takeover only after A2 is verified.
5. **Dropbox's silence on shallow/partial clone.** Fine for them; for a backup tool whose repos include large-ish real projects, at minimum *document* the limitation prominently as git-remote-dropbox does — or reserve protocol room for `option depth` later.
6. **Auto-retrying failed uploads aggressively.** The SDK's own guidance is that interrupted uploads should be re-initiated by the user, with the SDK handling transient network retries internally; mirror git-remote-dropbox's fail-fast posture rather than fighting the platform.

---

## Verdict and recommendations

**Feasibility verdict: engineering project, conditional on a short verification campaign** — per the framing in issue #3's comment thread. The single question the thread called load-bearing ("does Proton's CLI/SDK expose any conditional-write or equivalent atomicity guarantee?") has a positive, specific answer at the interface level: *revision-conditioned draft creation (`CurrentRevisionID`, 409 on staleness) plus exclusive draft locks plus server-enforced name uniqueness*, all visible in the MIT-licensed SDK Proton's own clients use, all within the published third-party usage guidelines. That is the primitive git-remote-dropbox's safety argument needs, in a form that may even be stronger (a critical section rather than a single-shot compare).

The conditionality is real, though: interface evidence is not concurrency evidence. Assumptions A1–A5 (§3b) — read-under-draft, commit fencing, lock scope, trash/namespace behavior, name equivalence — are each individually checkable with a ~50-line script and a throwaway account, and each is individually capable of sinking the lock-based design if it fails (fallbacks exist: the internal-layer literal CAS, or a lock-node scheme). So the precise verdict: **the concurrency question has moved from "open research question" to "specific, cheaply testable claims."** If the verification passes, everything that remains — auth/session plumbing, pack layout and discovery, SDK churn through the announced crypto migration — is engineering. The residual risk is *platform-tracking*, not computer science.

**The top three design decisions a v2 spec must make:**

1. **Substrate and auth.** Which SDK (JS/TS is where the CLI and most Proton momentum is; C# is the other first-class citizen); whether to vendor the SDK's internal upload layer — both concurrency constructions currently require seams the public facade doesn't expose (§3b) — or to first pursue the upstream feature request for a public conditioned write; and above all, where sessions come from: implement Proton auth against the `incubating/account` module, piggyback on the CLI's stored session (undocumented, fragile — note it wasn't even readable this session), or gate v2 on Proton shipping a third-party auth story. This decision dominates both effort and fragility, and nothing else can be prototyped honestly without it.
2. **Remote layout.** Loose objects (maximum simplicity, proven by git-remote-dropbox, poor fit for Proton's rate-limit and crypto economics) vs. append-only pack stacking with individually-CAS'd ref files (efficient, but requires a pack-discovery scheme — idx sidecars or a CAS-protected manifest, see Copy #7 — a local negotiation cache, and a deliberate compaction design that avoids gcrypt's full-reupload cliff). Includes retention policy and recoverability documentation for whichever layout wins, and a position on hash algorithms (SHA-1-only with an explicit early refusal, or `object-format` support).
3. **The concurrency protocol spec — a full ref-transition table, not just the happy path.** Run the A1–A5 verification first, then pin down every transition: create, fast-forward update, forced update and deletion (both revision-conditioned — see Copy #1), stale-draft takeover (conservative default; see Avoid #4), HEAD/default-branch handling and partial-success reporting across multi-ref pushes, ref-namespace scoping (branches vs. tags), ambiguous-outcome reconciliation, and the exact `fetch first` / "push in flight" UX — plus how v2 coexists with v1 (second remote alongside the local mirror vs. replacement, and whether v1's instant-local-push property is preserved by keeping the helper as a *backup* remote rather than the working remote).

**Short form of the above**, as posted to issue #3:

> Answered the load-bearing question by reading the source: Proton's API does document a conditional-write primitive. It's visible in the open-source SDK ([ProtonDriveApps/sdk](https://github.com/ProtonDriveApps/sdk), MIT — the same code Proton's own clients run): updating a file means opening a *draft revision* whose creation takes a `CurrentRevisionID` and fails with 409 if that revision is no longer current *or* another draft is open — and only one draft can be open per file. On paper that's compare-and-swap plus a short-lived exclusive lock, i.e. more than the Dropbox `WriteMode.update(rev)` mechanism git-remote-dropbox relies on. Server-enforced name uniqueness additionally gives an atomic create-if-absent for new refs.
>
> "On paper" is doing work in that sentence: this is read from the SDK's API types and tests, not verified under live contention. Before a spec leans on it, a throwaway-account script needs to confirm a handful of behaviors (can you read a file's committed content while another client holds a draft; does deleting someone's stale draft actually fence out their in-flight commit; does a trashed file free up its name). Each is cheap to test and each has a fallback design if it fails.
>
> Other caveats that keep this from being a weekend project: the CLI doesn't expose any of this (its conflict strategies are policy, not primitives), so a helper builds on the SDK directly; the SDK ships no third-party auth/session module yet; and Proton has a breaking crypto-model migration scheduled for ~end of 2026. But per this issue's framing, the concurrency question has moved from open research question to a short list of testable claims — if they check out, `git-remote-proton` is an engineering project. Notes in `docs/research/remote-helper-prior-art.md`.

---

*Written as read-only research: no code in this repository was modified and nothing was uploaded to Proton Drive while producing it. Findings were peer-reviewed by two independent models before publication.*
