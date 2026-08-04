# Probe C17b — direct provocation of the trashed-homonym name-conflict mechanism

**Started:** (see S1 timestamp below)
**Runner:** live probe runner (Claude Code background job)
**Account:** real Proton Drive account, `proton-drive` CLI
**Confinement:** writes only under `/my-files/_cas-probe` (work in `/my-files/_cas-probe/c17b`)
**Cleanup discipline:** trash only, never delete/empty-trash/restore; verify-before-trashing on every trash

Hypothesis under test: a name that has trashed homonym(s) — of the node itself or of an
ancestor — can produce a split-brain where fresh-process `filesystem info` reports the node
ABSENT while `filesystem create-folder` for the same name reports ALREADY EXISTS.

Classification per trial: CLEAN / REPRODUCTION / OTHER.

---

## Step S — Setup and ambient-state capture

### S1 — `proton-drive --version`

- Timestamp: 2026-08-04T13:55:01.7188577-07:00 (local, UTC-07:00)
- Exit code: 0

```
Proton Drive CLI cli-drive@0.7.0+5174900c
Proton Drive SDK js@0.20.0+5174900c
```

Certified build matched. Proceeding.

### S2 — `filesystem list /my-files --json`

- Timestamp: 2026-08-04T13:55:24.8868475-07:00
- Exit code: 0

```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~5QY-B6t9Eui6VKsXvJPmuA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g","name":{"ok":true,"value":"Project Repo Bundles"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"creationTime":"2026-06-22T16:25:42.000Z","modificationTime":"2026-06-22T16:25:42.000Z","treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~ThDi8B_92zkL76UXhjqFng","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g","name":{"ok":true,"value":"ChatGPT Export Text Backup"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"creationTime":"2026-05-18T02:36:23.000Z","modificationTime":"2026-05-18T02:36:23.000Z","treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Vho9hzKVaqnBLf7UnwC64w","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g","name":{"ok":true,"value":"GitBackups"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"creationTime":"2026-07-21T00:29:59.000Z","modificationTime":"2026-07-21T00:29:59.000Z","treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~n94X-kyGohebE_FBtxA5Sg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g","name":{"ok":true,"value":"Sensitive Project Sources"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"creationTime":"2026-05-22T15:12:18.000Z","modificationTime":"2026-05-22T15:12:18.000Z","treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```

### S3 — `filesystem --help` (read-only capability survey)

- Timestamp: 2026-08-04T13:55:42.3148865-07:00
- Exit code: 1

```
Usage:
    auth login
    auth logout
    filesystem list [-t TYPE] path
    filesystem info path
    filesystem create-folder parentPath name
    filesystem upload [-c STRATEGY] [-f STRATEGY] [-d STRATEGY] [-t] localPath... parentPath
    filesystem download [-c STRATEGY] [-f STRATEGY] [-d STRATEGY] path... localFolder
    filesystem rename path newName
    filesystem copy [-n NAME] sourcePath... targetParentPath
    filesystem move sourcePath... targetParentPath
    filesystem trash path...
    filesystem restore path...
    filesystem delete path...
    filesystem empty-trash
    sharing status path
    sharing invite [-u USER...] [-r ROLE] [-m MESSAGE] [-n] path
    sharing leave path
    sharing remove [-e EMAIL...] [-a] path
    sharing set-url [--role ROLE] [--password PASSWORD] [--expiration EXPIRATION] path
    sharing remove-url path
    invitation list
    invitation accept invitationUid
    invitation reject invitationUid
    album list
    album create name
    album update [-n NAME] [-c UID] albumPath
    album delete [-f] [-s] albumPath
    album photos [-d] albumPath
    album add-photo albumPath photoPath...
    album remove-photo albumPath photoPath...
    photo timeline [-d]
    photo upload [-c STRATEGY] localPath...
    photo download [-c STRATEGY] path... localFolder

General options:
    -h|--help: Show extended help for a command
    -j|--json: Output in JSON format
    -v|--verbose: Enable verbose output
Command not found: filesystem --help
```

### S3b — `filesystem list -h` (does -t TYPE include trashed?)

- Timestamp: 2026-08-04T13:55:55.8034179-07:00
- Exit code: 0

```
Usage:
    filesystem list [-t TYPE] path

Use / to list top-level sections. Your root folder is /my-files.

Options:
    -t, --type TYPE          Type of the node to filter by. Values: file,
                             folder, album, photo.

Remote paths:
    Posix paths are always used, regardless of the host OS.
    Node names are used for paths if available. When name cannot be decrypted
    or conflicts with other node(s), node UIDs can be used instead.
    Escape / in node names with a backslash.

    Examples:
    - /my-files/folder/file.txt
    - /my-files/folder/foo\/bar
    - /shared-with-me/NODE-UID/file.txt
```

**S3 finding:** no trash subcommand and no include-trashed flag exists. `list -t TYPE` accepts only file/folder/album/photo. The pre-existing trashed homonyms therefore CANNOT be enumerated by this CLI. Note the documented conflict rule in the help text: `When name cannot be decrypted or conflicts with other node(s), node UIDs can be used instead.`

### S4 — fresh-process observations of `/my-files/_cas-probe` BEFORE creating it

S4a `filesystem info /my-files/_cas-probe` — 2026-08-04T13:56:09.4810622-07:00 — exit 1
```
Node not found: _cas-probe
```
S4b `filesystem list /my-files/_cas-probe --json` — 2026-08-04T13:56:12.5669978-07:00 — exit 1
```
Node not found: _cas-probe
```

### S5 — OBSERVATION P0 (headline): `create-folder /my-files _cas-probe` with a trashed `_cas-probe` homonym present

- Timestamp: 2026-08-04T13:56:26.8996077-07:00
- Exit code: 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~L_5KcYiEE3tt59SOGUMbjg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g',
  name: { ok: true, value: '_cas-probe' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T20:56:31.870Z,
  modificationTime: 2026-08-04T20:56:31.870Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

**P0 classification: CLEAN.** S4 said absent, create-folder succeeded exit 0, new uid `tU-Ot1Sq63NwBcxlnl7IcA~L_5KcYiEE3tt59SOGUMbjg` (the pre-existing trashed `_cas-probe` was uid ending `~yzxv1W0vPJuYL0Wu_dYXKA`). A single trashed same-name homonym under the same parent did NOT block creation and did NOT produce the split-brain.

### S6 — OBSERVATION P1: `create-folder /my-files/_cas-probe c17b`

- Timestamp: 2026-08-04T13:56:46.0440220-07:00
- Exit code: 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~L_5KcYiEE3tt59SOGUMbjg',
  name: { ok: true, value: 'c17b' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T20:56:48.788Z,
  modificationTime: 2026-08-04T20:56:48.788Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

**P1 classification: CLEAN.** Workspace uid `tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A`.

---

## Variant 1 - Core (trashed homonym is the node ITSELF)

Per trial: create -> list(gate) -> trash -> recreate -> fresh-process info -> list parent -> create child.

#### Trial core-01 (Variant 1 - Core)

Step 1 `create-folder /my-files/_cas-probe/c17b core-01` - 2026-08-04T13:57:33.3623093-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-01' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T20:57:39.636Z,
  modificationTime: 2026-08-04T20:57:39.636Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 2 `list /my-files/_cas-probe/c17b/core-01 --json` (trash gate) - 2026-08-04T13:57:39.8444299-07:00 - exit 0
```
[

]
```
Step 3 SKIPPED - trash gate FAILED (list exit 0, output not an empty array). No trash issued.

**Note on trial `core-01`:** aborted at step 3 by the runner script's own trash gate. The gate compared the listing to the literal string `[]` but an empty folder lists as `[` newline newline `]`, so the gate failed closed and NO trash was issued. This is a runner-script bug, not Drive behavior. Drive itself behaved normally (create exit 0, list exit 0, empty). The folder `/my-files/_cas-probe/c17b/core-01` remains ACTIVE and EMPTY and is cleaned up in Step Z. The gate was fixed to be whitespace-insensitive and the ten real Variant-1 trials are named `core-02` .. `core-11`.


#### Trial core-02 (Variant 1 - Core)

Step 1 `create-folder /my-files/_cas-probe/c17b core-02` - 2026-08-04T13:58:13.3067792-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~-YJOC705ez6zVInxvV6ODw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-02' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T20:58:18.225Z,
  modificationTime: 2026-08-04T20:58:18.225Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 2 `list /my-files/_cas-probe/c17b/core-02 --json` (trash gate) - 2026-08-04T13:58:18.4507016-07:00 - exit 0
```
[

]
```
Step 3 `trash /my-files/_cas-probe/c17b/core-02` - 2026-08-04T13:58:21.5274900-07:00 - exit 0
```
✅ core-02
```
Step 4 RECREATE `create-folder /my-files/_cas-probe/c17b core-02` - 2026-08-04T13:58:25.7476358-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-02' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T20:58:29.971Z,
  modificationTime: 2026-08-04T20:58:29.971Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5 `info /my-files/_cas-probe/c17b/core-02` (fresh process) - 2026-08-04T13:58:30.1602159-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-02' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T20:58:29.000Z,
  modificationTime: 2026-08-04T20:58:29.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 6 `list /my-files/_cas-probe/c17b --json` - 2026-08-04T13:58:34.4610111-07:00 - exit 0 - rows named core-02 : 1 - trashTime occurrences in whole listing: 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:57:39.000Z","modificationTime":"2026-08-04T20:57:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:58:29.000Z","modificationTime":"2026-08-04T20:58:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 7 `create-folder /my-files/_cas-probe/c17b/core-02 child` (gate-signature check) - 2026-08-04T13:58:36.9872280-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~IeB56Zxo7JnMko1EkWbC3Q',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg',
  name: { ok: true, value: 'child' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T20:58:39.816Z,
  modificationTime: 2026-08-04T20:58:39.816Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

**Trial core-02 classification: CLEAN** (recreate exit 0, info exit 0, child-create exit 0)

#### Trial core-03 (Variant 1 - Core)

Step 1 `create-folder /my-files/_cas-probe/c17b core-03` - 2026-08-04T13:58:50.0724337-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~UeWf_CQLZWHGKpXKV2Pp1w',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-03' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T20:58:55.653Z,
  modificationTime: 2026-08-04T20:58:55.653Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 2 `list /my-files/_cas-probe/c17b/core-03 --json` (trash gate) - 2026-08-04T13:58:55.8717020-07:00 - exit 0
```
[

]
```
Step 3 `trash /my-files/_cas-probe/c17b/core-03` - 2026-08-04T13:59:01.7809372-07:00 - exit 0
```
✅ core-03
```
Step 4 RECREATE `create-folder /my-files/_cas-probe/c17b core-03` - 2026-08-04T13:59:05.4186588-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-03' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T20:59:08.978Z,
  modificationTime: 2026-08-04T20:59:08.978Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5 `info /my-files/_cas-probe/c17b/core-03` (fresh process) - 2026-08-04T13:59:09.1615824-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-03' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T20:59:08.000Z,
  modificationTime: 2026-08-04T20:59:08.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 6 `list /my-files/_cas-probe/c17b --json` - 2026-08-04T13:59:12.9498399-07:00 - exit 0 - rows named core-03 : 1 - trashTime occurrences in whole listing: 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:57:39.000Z","modificationTime":"2026-08-04T20:57:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:58:29.000Z","modificationTime":"2026-08-04T20:58:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:08.000Z","modificationTime":"2026-08-04T20:59:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 7 `create-folder /my-files/_cas-probe/c17b/core-03 child` (gate-signature check) - 2026-08-04T13:59:15.8901233-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~ittNua1w0hnNf22b2JS7ug',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg',
  name: { ok: true, value: 'child' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T20:59:18.680Z,
  modificationTime: 2026-08-04T20:59:18.680Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

**Trial core-03 classification: CLEAN** (recreate exit 0, info exit 0, child-create exit 0)

#### Trial core-04 (Variant 1 - Core)

Step 1 `create-folder /my-files/_cas-probe/c17b core-04` - 2026-08-04T13:59:25.7640185-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~1CNNvVfUetPgVbs-U_w60Q',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-04' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T20:59:28.525Z,
  modificationTime: 2026-08-04T20:59:28.525Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 2 `list /my-files/_cas-probe/c17b/core-04 --json` (trash gate) - 2026-08-04T13:59:28.7247585-07:00 - exit 0
```
[

]
```
Step 3 `trash /my-files/_cas-probe/c17b/core-04` - 2026-08-04T13:59:31.9974548-07:00 - exit 0
```
✅ core-04
```
Step 4 RECREATE `create-folder /my-files/_cas-probe/c17b core-04` - 2026-08-04T13:59:36.2657066-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-04' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T20:59:40.358Z,
  modificationTime: 2026-08-04T20:59:40.358Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5 `info /my-files/_cas-probe/c17b/core-04` (fresh process) - 2026-08-04T13:59:40.5571475-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-04' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T20:59:39.000Z,
  modificationTime: 2026-08-04T20:59:39.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 6 `list /my-files/_cas-probe/c17b --json` - 2026-08-04T13:59:44.5681661-07:00 - exit 0 - rows named core-04 : 1 - trashTime occurrences in whole listing: 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:57:39.000Z","modificationTime":"2026-08-04T20:57:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:58:29.000Z","modificationTime":"2026-08-04T20:58:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:08.000Z","modificationTime":"2026-08-04T20:59:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:39.000Z","modificationTime":"2026-08-04T20:59:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 7 `create-folder /my-files/_cas-probe/c17b/core-04 child` (gate-signature check) - 2026-08-04T13:59:47.8608359-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~ceL-Hrj2Iz1UG6hZbPkGhg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg',
  name: { ok: true, value: 'child' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T20:59:52.114Z,
  modificationTime: 2026-08-04T20:59:52.114Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

**Trial core-04 classification: CLEAN** (recreate exit 0, info exit 0, child-create exit 0)

#### Trial core-05 (Variant 1 - Core)

Step 1 `create-folder /my-files/_cas-probe/c17b core-05` - 2026-08-04T14:00:00.4701823-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~3-hy70Qi9ChDCOV3pmvnng',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-05' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:00:05.484Z,
  modificationTime: 2026-08-04T21:00:05.484Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 2 `list /my-files/_cas-probe/c17b/core-05 --json` (trash gate) - 2026-08-04T14:00:05.6767597-07:00 - exit 0
```
[

]
```
Step 3 `trash /my-files/_cas-probe/c17b/core-05` - 2026-08-04T14:00:08.8077750-07:00 - exit 0
```
✅ core-05
```
Step 4 RECREATE `create-folder /my-files/_cas-probe/c17b core-05` - 2026-08-04T14:00:12.5181827-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~bGWhebPsVOLM0E43JuNSEg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-05' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:00:16.219Z,
  modificationTime: 2026-08-04T21:00:16.219Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5 `info /my-files/_cas-probe/c17b/core-05` (fresh process) - 2026-08-04T14:00:16.4182331-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~bGWhebPsVOLM0E43JuNSEg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-05' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:00:15.000Z,
  modificationTime: 2026-08-04T21:00:15.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 6 `list /my-files/_cas-probe/c17b --json` - 2026-08-04T14:00:22.2327902-07:00 - exit 0 - rows named core-05 : 1 - trashTime occurrences in whole listing: 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:57:39.000Z","modificationTime":"2026-08-04T20:57:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:58:29.000Z","modificationTime":"2026-08-04T20:58:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:08.000Z","modificationTime":"2026-08-04T20:59:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:39.000Z","modificationTime":"2026-08-04T20:59:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~bGWhebPsVOLM0E43JuNSEg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:15.000Z","modificationTime":"2026-08-04T21:00:15.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 7 `create-folder /my-files/_cas-probe/c17b/core-05 child` (gate-signature check) - 2026-08-04T14:00:26.0879937-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~DUU2BBoCLqHf5sCQ72wM6A',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~bGWhebPsVOLM0E43JuNSEg',
  name: { ok: true, value: 'child' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:00:29.756Z,
  modificationTime: 2026-08-04T21:00:29.756Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

**Trial core-05 classification: CLEAN** (recreate exit 0, info exit 0, child-create exit 0)

#### Trial core-06 (Variant 1 - Core)

Step 1 `create-folder /my-files/_cas-probe/c17b core-06` - 2026-08-04T14:00:38.5654058-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~-fHO1is5d9-UTZ8n_cnFjQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-06' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:00:42.212Z,
  modificationTime: 2026-08-04T21:00:42.212Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 2 `list /my-files/_cas-probe/c17b/core-06 --json` (trash gate) - 2026-08-04T14:00:42.4372595-07:00 - exit 0
```
[

]
```
Step 3 `trash /my-files/_cas-probe/c17b/core-06` - 2026-08-04T14:00:46.8150345-07:00 - exit 0
```
✅ core-06
```
Step 4 RECREATE `create-folder /my-files/_cas-probe/c17b core-06` - 2026-08-04T14:00:49.5518487-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~k2ltsJ374d3nDXuylx1LHg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-06' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:00:52.346Z,
  modificationTime: 2026-08-04T21:00:52.346Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5 `info /my-files/_cas-probe/c17b/core-06` (fresh process) - 2026-08-04T14:00:52.5503102-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~k2ltsJ374d3nDXuylx1LHg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-06' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:00:51.000Z,
  modificationTime: 2026-08-04T21:00:51.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 6 `list /my-files/_cas-probe/c17b --json` - 2026-08-04T14:00:56.9835607-07:00 - exit 0 - rows named core-06 : 1 - trashTime occurrences in whole listing: 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:57:39.000Z","modificationTime":"2026-08-04T20:57:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:58:29.000Z","modificationTime":"2026-08-04T20:58:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:08.000Z","modificationTime":"2026-08-04T20:59:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:39.000Z","modificationTime":"2026-08-04T20:59:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~bGWhebPsVOLM0E43JuNSEg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:15.000Z","modificationTime":"2026-08-04T21:00:15.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~k2ltsJ374d3nDXuylx1LHg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:51.000Z","modificationTime":"2026-08-04T21:00:51.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 7 `create-folder /my-files/_cas-probe/c17b/core-06 child` (gate-signature check) - 2026-08-04T14:00:59.6936398-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~LpeZ97LMs-K5hVjQMhYHMA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~k2ltsJ374d3nDXuylx1LHg',
  name: { ok: true, value: 'child' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:01:04.325Z,
  modificationTime: 2026-08-04T21:01:04.325Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

**Trial core-06 classification: CLEAN** (recreate exit 0, info exit 0, child-create exit 0)

#### Trial core-07 (Variant 1 - Core)

Step 1 `create-folder /my-files/_cas-probe/c17b core-07` - 2026-08-04T14:01:13.6463958-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~nMZ3alyHZXU1Brd0_E8Bxw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-07' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:01:19.116Z,
  modificationTime: 2026-08-04T21:01:19.116Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 2 `list /my-files/_cas-probe/c17b/core-07 --json` (trash gate) - 2026-08-04T14:01:19.3071155-07:00 - exit 0
```
[

]
```
Step 3 `trash /my-files/_cas-probe/c17b/core-07` - 2026-08-04T14:01:22.6043052-07:00 - exit 0
```
✅ core-07
```
Step 4 RECREATE `create-folder /my-files/_cas-probe/c17b core-07` - 2026-08-04T14:01:27.3050445-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~v_M-faPpHavHoZAgQYT16A',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-07' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:01:30.229Z,
  modificationTime: 2026-08-04T21:01:30.229Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5 `info /my-files/_cas-probe/c17b/core-07` (fresh process) - 2026-08-04T14:01:30.3994940-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~v_M-faPpHavHoZAgQYT16A',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-07' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:01:29.000Z,
  modificationTime: 2026-08-04T21:01:29.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 6 `list /my-files/_cas-probe/c17b --json` - 2026-08-04T14:01:33.3401779-07:00 - exit 0 - rows named core-07 : 1 - trashTime occurrences in whole listing: 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:57:39.000Z","modificationTime":"2026-08-04T20:57:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:58:29.000Z","modificationTime":"2026-08-04T20:58:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:08.000Z","modificationTime":"2026-08-04T20:59:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:39.000Z","modificationTime":"2026-08-04T20:59:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~bGWhebPsVOLM0E43JuNSEg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:15.000Z","modificationTime":"2026-08-04T21:00:15.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~k2ltsJ374d3nDXuylx1LHg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:51.000Z","modificationTime":"2026-08-04T21:00:51.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~v_M-faPpHavHoZAgQYT16A","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:01:29.000Z","modificationTime":"2026-08-04T21:01:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 7 `create-folder /my-files/_cas-probe/c17b/core-07 child` (gate-signature check) - 2026-08-04T14:01:37.0422669-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~qZJB9AE09Uwetrx1zQeO1w',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~v_M-faPpHavHoZAgQYT16A',
  name: { ok: true, value: 'child' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:01:40.083Z,
  modificationTime: 2026-08-04T21:01:40.083Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

**Trial core-07 classification: CLEAN** (recreate exit 0, info exit 0, child-create exit 0)

#### Trial core-08 (Variant 1 - Core)

Step 1 `create-folder /my-files/_cas-probe/c17b core-08` - 2026-08-04T14:01:49.0102167-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~Q_F3rLnaJON8LsQTiw9hkA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-08' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:01:52.281Z,
  modificationTime: 2026-08-04T21:01:52.281Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 2 `list /my-files/_cas-probe/c17b/core-08 --json` (trash gate) - 2026-08-04T14:01:52.4500829-07:00 - exit 0
```
[

]
```
Step 3 `trash /my-files/_cas-probe/c17b/core-08` - 2026-08-04T14:01:56.3406864-07:00 - exit 0
```
✅ core-08
```
Step 4 RECREATE `create-folder /my-files/_cas-probe/c17b core-08` - 2026-08-04T14:01:59.9720603-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~FJ3e7KVTX6L-KR8eRoYaCw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-08' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:02:02.406Z,
  modificationTime: 2026-08-04T21:02:02.406Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5 `info /my-files/_cas-probe/c17b/core-08` (fresh process) - 2026-08-04T14:02:02.5762436-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~FJ3e7KVTX6L-KR8eRoYaCw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-08' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:02:01.000Z,
  modificationTime: 2026-08-04T21:02:01.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 6 `list /my-files/_cas-probe/c17b --json` - 2026-08-04T14:02:06.3310116-07:00 - exit 0 - rows named core-08 : 1 - trashTime occurrences in whole listing: 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:57:39.000Z","modificationTime":"2026-08-04T20:57:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:58:29.000Z","modificationTime":"2026-08-04T20:58:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:08.000Z","modificationTime":"2026-08-04T20:59:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:39.000Z","modificationTime":"2026-08-04T20:59:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~bGWhebPsVOLM0E43JuNSEg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:15.000Z","modificationTime":"2026-08-04T21:00:15.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~k2ltsJ374d3nDXuylx1LHg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:51.000Z","modificationTime":"2026-08-04T21:00:51.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~v_M-faPpHavHoZAgQYT16A","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:01:29.000Z","modificationTime":"2026-08-04T21:01:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~FJ3e7KVTX6L-KR8eRoYaCw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:01.000Z","modificationTime":"2026-08-04T21:02:01.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 7 `create-folder /my-files/_cas-probe/c17b/core-08 child` (gate-signature check) - 2026-08-04T14:02:08.6233451-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~KJrhJWxY_e5wY1-Bu2ffcQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~FJ3e7KVTX6L-KR8eRoYaCw',
  name: { ok: true, value: 'child' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:02:11.977Z,
  modificationTime: 2026-08-04T21:02:11.977Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

**Trial core-08 classification: CLEAN** (recreate exit 0, info exit 0, child-create exit 0)

#### Trial core-09 (Variant 1 - Core)

Step 1 `create-folder /my-files/_cas-probe/c17b core-09` - 2026-08-04T14:02:19.1511515-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~QhfoNGiksdvJby5fRO3r8w',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-09' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:02:23.602Z,
  modificationTime: 2026-08-04T21:02:23.602Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 2 `list /my-files/_cas-probe/c17b/core-09 --json` (trash gate) - 2026-08-04T14:02:23.8038258-07:00 - exit 0
```
[

]
```
Step 3 `trash /my-files/_cas-probe/c17b/core-09` - 2026-08-04T14:02:27.4765606-07:00 - exit 0
```
✅ core-09
```
Step 4 RECREATE `create-folder /my-files/_cas-probe/c17b core-09` - 2026-08-04T14:02:30.2903869-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~1jszxKEh7pipkm11xfv7oQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-09' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:02:33.990Z,
  modificationTime: 2026-08-04T21:02:33.990Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5 `info /my-files/_cas-probe/c17b/core-09` (fresh process) - 2026-08-04T14:02:34.1991693-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~1jszxKEh7pipkm11xfv7oQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-09' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:02:33.000Z,
  modificationTime: 2026-08-04T21:02:33.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 6 `list /my-files/_cas-probe/c17b --json` - 2026-08-04T14:02:37.2728822-07:00 - exit 0 - rows named core-09 : 1 - trashTime occurrences in whole listing: 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:57:39.000Z","modificationTime":"2026-08-04T20:57:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:58:29.000Z","modificationTime":"2026-08-04T20:58:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:08.000Z","modificationTime":"2026-08-04T20:59:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:39.000Z","modificationTime":"2026-08-04T20:59:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~bGWhebPsVOLM0E43JuNSEg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:15.000Z","modificationTime":"2026-08-04T21:00:15.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~k2ltsJ374d3nDXuylx1LHg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:51.000Z","modificationTime":"2026-08-04T21:00:51.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~v_M-faPpHavHoZAgQYT16A","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:01:29.000Z","modificationTime":"2026-08-04T21:01:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~FJ3e7KVTX6L-KR8eRoYaCw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:01.000Z","modificationTime":"2026-08-04T21:02:01.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~1jszxKEh7pipkm11xfv7oQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:33.000Z","modificationTime":"2026-08-04T21:02:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 7 `create-folder /my-files/_cas-probe/c17b/core-09 child` (gate-signature check) - 2026-08-04T14:02:39.6589119-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~N861OUR4EjoYY0PwJTekqg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~1jszxKEh7pipkm11xfv7oQ',
  name: { ok: true, value: 'child' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:02:42.623Z,
  modificationTime: 2026-08-04T21:02:42.623Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

**Trial core-09 classification: CLEAN** (recreate exit 0, info exit 0, child-create exit 0)

#### Trial core-10 (Variant 1 - Core)

Step 1 `create-folder /my-files/_cas-probe/c17b core-10` - 2026-08-04T14:02:49.8593335-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~A9GIusfwzAx5uHoFeHDbCw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-10' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:02:52.555Z,
  modificationTime: 2026-08-04T21:02:52.555Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 2 `list /my-files/_cas-probe/c17b/core-10 --json` (trash gate) - 2026-08-04T14:02:52.7527488-07:00 - exit 0
```
[

]
```
Step 3 `trash /my-files/_cas-probe/c17b/core-10` - 2026-08-04T14:02:56.4230302-07:00 - exit 0
```
✅ core-10
```
Step 4 RECREATE `create-folder /my-files/_cas-probe/c17b core-10` - 2026-08-04T14:02:59.2094174-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~wKOeh9MS6iGmXEiVci9H1w',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-10' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:03:02.800Z,
  modificationTime: 2026-08-04T21:03:02.800Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5 `info /my-files/_cas-probe/c17b/core-10` (fresh process) - 2026-08-04T14:03:02.9727724-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~wKOeh9MS6iGmXEiVci9H1w',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-10' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:03:02.000Z,
  modificationTime: 2026-08-04T21:03:02.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 6 `list /my-files/_cas-probe/c17b --json` - 2026-08-04T14:03:05.5254225-07:00 - exit 0 - rows named core-10 : 1 - trashTime occurrences in whole listing: 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:57:39.000Z","modificationTime":"2026-08-04T20:57:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:58:29.000Z","modificationTime":"2026-08-04T20:58:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:08.000Z","modificationTime":"2026-08-04T20:59:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:39.000Z","modificationTime":"2026-08-04T20:59:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~bGWhebPsVOLM0E43JuNSEg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:15.000Z","modificationTime":"2026-08-04T21:00:15.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~k2ltsJ374d3nDXuylx1LHg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:51.000Z","modificationTime":"2026-08-04T21:00:51.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~v_M-faPpHavHoZAgQYT16A","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:01:29.000Z","modificationTime":"2026-08-04T21:01:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~FJ3e7KVTX6L-KR8eRoYaCw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:01.000Z","modificationTime":"2026-08-04T21:02:01.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~1jszxKEh7pipkm11xfv7oQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:33.000Z","modificationTime":"2026-08-04T21:02:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~wKOeh9MS6iGmXEiVci9H1w","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:02.000Z","modificationTime":"2026-08-04T21:03:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 7 `create-folder /my-files/_cas-probe/c17b/core-10 child` (gate-signature check) - 2026-08-04T14:03:08.0220713-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~fRf2wrA_B471e6Nv3PBaYQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~wKOeh9MS6iGmXEiVci9H1w',
  name: { ok: true, value: 'child' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:03:12.004Z,
  modificationTime: 2026-08-04T21:03:12.004Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

**Trial core-10 classification: CLEAN** (recreate exit 0, info exit 0, child-create exit 0)

#### Trial core-11 (Variant 1 - Core)

Step 1 `create-folder /my-files/_cas-probe/c17b core-11` - 2026-08-04T14:03:19.2397362-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~MJVAVELc9D9l2TZ9Vv4Ogw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-11' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:03:23.975Z,
  modificationTime: 2026-08-04T21:03:23.975Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 2 `list /my-files/_cas-probe/c17b/core-11 --json` (trash gate) - 2026-08-04T14:03:24.1782663-07:00 - exit 0
```
[

]
```
Step 3 `trash /my-files/_cas-probe/c17b/core-11` - 2026-08-04T14:03:29.8420672-07:00 - exit 0
```
✅ core-11
```
Step 4 RECREATE `create-folder /my-files/_cas-probe/c17b core-11` - 2026-08-04T14:03:34.4779160-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~XU3VjNpul9sR9HcQR60UkA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-11' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:03:37.688Z,
  modificationTime: 2026-08-04T21:03:37.688Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5 `info /my-files/_cas-probe/c17b/core-11` (fresh process) - 2026-08-04T14:03:37.8745444-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~XU3VjNpul9sR9HcQR60UkA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'core-11' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:03:37.000Z,
  modificationTime: 2026-08-04T21:03:37.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 6 `list /my-files/_cas-probe/c17b --json` - 2026-08-04T14:03:42.1842848-07:00 - exit 0 - rows named core-11 : 1 - trashTime occurrences in whole listing: 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:57:39.000Z","modificationTime":"2026-08-04T20:57:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:58:29.000Z","modificationTime":"2026-08-04T20:58:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:08.000Z","modificationTime":"2026-08-04T20:59:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:39.000Z","modificationTime":"2026-08-04T20:59:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~bGWhebPsVOLM0E43JuNSEg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:15.000Z","modificationTime":"2026-08-04T21:00:15.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~k2ltsJ374d3nDXuylx1LHg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:51.000Z","modificationTime":"2026-08-04T21:00:51.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~v_M-faPpHavHoZAgQYT16A","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:01:29.000Z","modificationTime":"2026-08-04T21:01:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~FJ3e7KVTX6L-KR8eRoYaCw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:01.000Z","modificationTime":"2026-08-04T21:02:01.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~1jszxKEh7pipkm11xfv7oQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:33.000Z","modificationTime":"2026-08-04T21:02:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~wKOeh9MS6iGmXEiVci9H1w","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:02.000Z","modificationTime":"2026-08-04T21:03:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~XU3VjNpul9sR9HcQR60UkA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-11"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:37.000Z","modificationTime":"2026-08-04T21:03:37.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 7 `create-folder /my-files/_cas-probe/c17b/core-11 child` (gate-signature check) - 2026-08-04T14:03:46.0484231-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~MK4jr1KV-h0SKneHmv7VWw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~XU3VjNpul9sR9HcQR60UkA',
  name: { ok: true, value: 'child' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:03:48.493Z,
  modificationTime: 2026-08-04T21:03:48.493Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

**Trial core-11 classification: CLEAN** (recreate exit 0, info exit 0, child-create exit 0)

**Variant 1 tally: 10 trials run (core-02 .. core-11), 10 CLEAN, 0 REPRODUCTION, 0 OTHER.** (core-01 aborted on the runner-script gate bug; not counted.)

---

## Variant 2 - Depth (trashed homonym is an ANCESTOR; the actual gate topology)


#### Trial depth-01 (Variant 2 - Depth / ancestor homonym)

Step 1 `create-folder /my-files/_cas-probe/c17b depth-01` - 2026-08-04T14:04:28.7770399-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~f6onybu3c_USu5XvhEqc9Q',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'depth-01' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:04:31.197Z,
  modificationTime: 2026-08-04T21:04:31.197Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 2a `list /my-files/_cas-probe/c17b/depth-01 --json` (trash gate) - 2026-08-04T14:04:31.4005725-07:00 - exit 0
```
[

]
```
Step 2b `trash /my-files/_cas-probe/c17b/depth-01` - 2026-08-04T14:04:36.6915056-07:00 - exit 0
```
✅ depth-01
```
Step 3 RECREATE `create-folder /my-files/_cas-probe/c17b depth-01` - 2026-08-04T14:04:40.0340021-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~zo8FDPjVnwgfwvZhmcOHJQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'depth-01' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:04:44.320Z,
  modificationTime: 2026-08-04T21:04:44.320Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4a `create-folder /my-files/_cas-probe/c17b/depth-01 repo` - 2026-08-04T14:04:44.5075131-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~qu2zzgcMna-ntO_XHHIq3w',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~zo8FDPjVnwgfwvZhmcOHJQ',
  name: { ok: true, value: 'repo' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:04:49.066Z,
  modificationTime: 2026-08-04T21:04:49.066Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4b `create-folder /my-files/_cas-probe/c17b/depth-01/repo refs` - 2026-08-04T14:04:49.2785586-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~_JwuJRg-3VTsayKRPJ3xbw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~qu2zzgcMna-ntO_XHHIq3w',
  name: { ok: true, value: 'refs' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:04:55.161Z,
  modificationTime: 2026-08-04T21:04:55.161Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4c `create-folder /my-files/_cas-probe/c17b/depth-01/repo/refs heads` - 2026-08-04T14:04:55.3488842-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~tMopSmrGTBrZAVJIWQL7qA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~_JwuJRg-3VTsayKRPJ3xbw',
  name: { ok: true, value: 'heads' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:05:01.048Z,
  modificationTime: 2026-08-04T21:05:01.048Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4d `create-folder /my-files/_cas-probe/c17b/depth-01/repo/refs tags` - 2026-08-04T14:05:01.2294150-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~pwtIx_Kj21WA8lqwvDpUCw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~_JwuJRg-3VTsayKRPJ3xbw',
  name: { ok: true, value: 'tags' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:05:05.745Z,
  modificationTime: 2026-08-04T21:05:05.745Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5a `info /my-files/_cas-probe/c17b/depth-01/repo/refs/tags` (fresh process) - 2026-08-04T14:05:05.9357680-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~pwtIx_Kj21WA8lqwvDpUCw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~_JwuJRg-3VTsayKRPJ3xbw',
  name: { ok: true, value: 'tags' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:05:05.000Z,
  modificationTime: 2026-08-04T21:05:05.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5b `info /my-files/_cas-probe/c17b/depth-01/repo/refs/heads` (fresh process) - 2026-08-04T14:05:12.0731269-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~tMopSmrGTBrZAVJIWQL7qA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~_JwuJRg-3VTsayKRPJ3xbw',
  name: { ok: true, value: 'heads' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:05:00.000Z,
  modificationTime: 2026-08-04T21:05:00.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false, claimedModificationTime: undefined },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5c `list /my-files/_cas-probe/c17b/depth-01/repo/refs --json` - 2026-08-04T14:05:16.8767230-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~tMopSmrGTBrZAVJIWQL7qA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~_JwuJRg-3VTsayKRPJ3xbw","name":{"ok":true,"value":"heads"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:05:00.000Z","modificationTime":"2026-08-04T21:05:00.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~pwtIx_Kj21WA8lqwvDpUCw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~_JwuJRg-3VTsayKRPJ3xbw","name":{"ok":true,"value":"tags"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:05:05.000Z","modificationTime":"2026-08-04T21:05:05.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 6 `create-folder /my-files/_cas-probe/c17b/depth-01/repo/refs tags` (2nd time; already-exists is correct) - 2026-08-04T14:05:20.7719574-07:00 - exit 1
```
A file or folder with that name already exists
```

**Trial depth-01 classification: CLEAN** (recreate 0; creates repo/refs/heads/tags 0/0/0/0; info tags/heads 0/0; list refs 0; 2nd tags create 1)

#### Trial depth-02 (Variant 2 - Depth / ancestor homonym)

Step 1 `create-folder /my-files/_cas-probe/c17b depth-02` - 2026-08-04T14:05:46.1150964-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~ORC3XgeiTlKIVRfOv51DHw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'depth-02' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:05:48.826Z,
  modificationTime: 2026-08-04T21:05:48.826Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 2a `list /my-files/_cas-probe/c17b/depth-02 --json` (trash gate) - 2026-08-04T14:05:49.0220910-07:00 - exit 0
```
[

]
```
Step 2b `trash /my-files/_cas-probe/c17b/depth-02` - 2026-08-04T14:05:52.0573591-07:00 - exit 0
```
✅ depth-02
```
Step 3 RECREATE `create-folder /my-files/_cas-probe/c17b depth-02` - 2026-08-04T14:05:55.8030236-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~so1Xnd4CFVTmnSkGYJc1QQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'depth-02' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:05:59.657Z,
  modificationTime: 2026-08-04T21:05:59.657Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4a `create-folder /my-files/_cas-probe/c17b/depth-02 repo` - 2026-08-04T14:05:59.8639439-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~QlYoUbd0lxj4VFZ7Y5dYMw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~so1Xnd4CFVTmnSkGYJc1QQ',
  name: { ok: true, value: 'repo' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:06:04.272Z,
  modificationTime: 2026-08-04T21:06:04.272Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4b `create-folder /my-files/_cas-probe/c17b/depth-02/repo refs` - 2026-08-04T14:06:04.4410747-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~6z5hFVnVCK1UUnaoegBJOA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~QlYoUbd0lxj4VFZ7Y5dYMw',
  name: { ok: true, value: 'refs' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:06:09.810Z,
  modificationTime: 2026-08-04T21:06:09.810Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4c `create-folder /my-files/_cas-probe/c17b/depth-02/repo/refs heads` - 2026-08-04T14:06:09.9846431-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~Hi9tfZSlg6yOkxDwtV5myQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~6z5hFVnVCK1UUnaoegBJOA',
  name: { ok: true, value: 'heads' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:06:15.090Z,
  modificationTime: 2026-08-04T21:06:15.090Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4d `create-folder /my-files/_cas-probe/c17b/depth-02/repo/refs tags` - 2026-08-04T14:06:15.2876455-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~X9D6YqXKH0eJtWlmqd-HBw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~6z5hFVnVCK1UUnaoegBJOA',
  name: { ok: true, value: 'tags' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:06:18.716Z,
  modificationTime: 2026-08-04T21:06:18.716Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5a `info /my-files/_cas-probe/c17b/depth-02/repo/refs/tags` (fresh process) - 2026-08-04T14:06:18.9212404-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~X9D6YqXKH0eJtWlmqd-HBw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~6z5hFVnVCK1UUnaoegBJOA',
  name: { ok: true, value: 'tags' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:06:18.000Z,
  modificationTime: 2026-08-04T21:06:18.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5b `info /my-files/_cas-probe/c17b/depth-02/repo/refs/heads` (fresh process) - 2026-08-04T14:06:23.8232186-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~Hi9tfZSlg6yOkxDwtV5myQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~6z5hFVnVCK1UUnaoegBJOA',
  name: { ok: true, value: 'heads' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:06:14.000Z,
  modificationTime: 2026-08-04T21:06:14.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false, claimedModificationTime: undefined },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5c `list /my-files/_cas-probe/c17b/depth-02/repo/refs --json` - 2026-08-04T14:06:26.8094951-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Hi9tfZSlg6yOkxDwtV5myQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~6z5hFVnVCK1UUnaoegBJOA","name":{"ok":true,"value":"heads"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:06:14.000Z","modificationTime":"2026-08-04T21:06:14.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~X9D6YqXKH0eJtWlmqd-HBw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~6z5hFVnVCK1UUnaoegBJOA","name":{"ok":true,"value":"tags"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:06:18.000Z","modificationTime":"2026-08-04T21:06:18.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 6 `create-folder /my-files/_cas-probe/c17b/depth-02/repo/refs tags` (2nd time; already-exists is correct) - 2026-08-04T14:06:30.6728648-07:00 - exit 1
```
A file or folder with that name already exists
```

**Trial depth-02 classification: CLEAN** (recreate 0; creates repo/refs/heads/tags 0/0/0/0; info tags/heads 0/0; list refs 0; 2nd tags create 1)

#### Trial depth-03 (Variant 2 - Depth / ancestor homonym)

Step 1 `create-folder /my-files/_cas-probe/c17b depth-03` - 2026-08-04T14:06:48.9574549-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~5yP5GpAPKfMV18MtqFY6HA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'depth-03' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:06:52.285Z,
  modificationTime: 2026-08-04T21:06:52.285Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 2a `list /my-files/_cas-probe/c17b/depth-03 --json` (trash gate) - 2026-08-04T14:06:52.5412765-07:00 - exit 0
```
[

]
```
Step 2b `trash /my-files/_cas-probe/c17b/depth-03` - 2026-08-04T14:06:56.9536871-07:00 - exit 0
```
✅ depth-03
```
Step 3 RECREATE `create-folder /my-files/_cas-probe/c17b depth-03` - 2026-08-04T14:07:00.0149767-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~gnyUjXCeWGdaTaD9bGOjNg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'depth-03' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:07:04.167Z,
  modificationTime: 2026-08-04T21:07:04.167Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4a `create-folder /my-files/_cas-probe/c17b/depth-03 repo` - 2026-08-04T14:07:04.3475958-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~eLCf-Sz3BpIJ4anOUqEXCw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~gnyUjXCeWGdaTaD9bGOjNg',
  name: { ok: true, value: 'repo' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:07:07.227Z,
  modificationTime: 2026-08-04T21:07:07.227Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4b `create-folder /my-files/_cas-probe/c17b/depth-03/repo refs` - 2026-08-04T14:07:07.4115944-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~sI-tv88xh7e9unS4ZhXxIA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~eLCf-Sz3BpIJ4anOUqEXCw',
  name: { ok: true, value: 'refs' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:07:13.321Z,
  modificationTime: 2026-08-04T21:07:13.321Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4c `create-folder /my-files/_cas-probe/c17b/depth-03/repo/refs heads` - 2026-08-04T14:07:13.4933155-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~wBeuLR_kPshIo3GSWEnwVw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~sI-tv88xh7e9unS4ZhXxIA',
  name: { ok: true, value: 'heads' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:07:18.311Z,
  modificationTime: 2026-08-04T21:07:18.311Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4d `create-folder /my-files/_cas-probe/c17b/depth-03/repo/refs tags` - 2026-08-04T14:07:18.5409871-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~0EFr9X3hIPiNs4Sp-H62ig',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~sI-tv88xh7e9unS4ZhXxIA',
  name: { ok: true, value: 'tags' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:07:23.217Z,
  modificationTime: 2026-08-04T21:07:23.217Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5a `info /my-files/_cas-probe/c17b/depth-03/repo/refs/tags` (fresh process) - 2026-08-04T14:07:23.3792560-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~0EFr9X3hIPiNs4Sp-H62ig',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~sI-tv88xh7e9unS4ZhXxIA',
  name: { ok: true, value: 'tags' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:07:22.000Z,
  modificationTime: 2026-08-04T21:07:22.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5b `info /my-files/_cas-probe/c17b/depth-03/repo/refs/heads` (fresh process) - 2026-08-04T14:07:26.9932655-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~wBeuLR_kPshIo3GSWEnwVw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~sI-tv88xh7e9unS4ZhXxIA',
  name: { ok: true, value: 'heads' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:07:17.000Z,
  modificationTime: 2026-08-04T21:07:17.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false, claimedModificationTime: undefined },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5c `list /my-files/_cas-probe/c17b/depth-03/repo/refs --json` - 2026-08-04T14:07:31.3404393-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~wBeuLR_kPshIo3GSWEnwVw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~sI-tv88xh7e9unS4ZhXxIA","name":{"ok":true,"value":"heads"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:07:17.000Z","modificationTime":"2026-08-04T21:07:17.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~0EFr9X3hIPiNs4Sp-H62ig","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~sI-tv88xh7e9unS4ZhXxIA","name":{"ok":true,"value":"tags"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:07:22.000Z","modificationTime":"2026-08-04T21:07:22.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 6 `create-folder /my-files/_cas-probe/c17b/depth-03/repo/refs tags` (2nd time; already-exists is correct) - 2026-08-04T14:07:35.5795717-07:00 - exit 1
```
A file or folder with that name already exists
```

**Trial depth-03 classification: CLEAN** (recreate 0; creates repo/refs/heads/tags 0/0/0/0; info tags/heads 0/0; list refs 0; 2nd tags create 1)

#### Trial depth-04 (Variant 2 - Depth / ancestor homonym)

Step 1 `create-folder /my-files/_cas-probe/c17b depth-04` - 2026-08-04T14:07:48.0275404-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~cxbxtD-wyWcXQ3c3D3xH_Q',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'depth-04' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:07:50.927Z,
  modificationTime: 2026-08-04T21:07:50.927Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 2a `list /my-files/_cas-probe/c17b/depth-04 --json` (trash gate) - 2026-08-04T14:07:51.1412575-07:00 - exit 0
```
[

]
```
Step 2b `trash /my-files/_cas-probe/c17b/depth-04` - 2026-08-04T14:07:55.6995550-07:00 - exit 0
```
✅ depth-04
```
Step 3 RECREATE `create-folder /my-files/_cas-probe/c17b depth-04` - 2026-08-04T14:08:00.6992961-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~eht2Lam-dW-anbYO-KBi0g',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'depth-04' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:08:03.352Z,
  modificationTime: 2026-08-04T21:08:03.352Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4a `create-folder /my-files/_cas-probe/c17b/depth-04 repo` - 2026-08-04T14:08:03.5578209-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~NAKENC51ZcIk4ZhectN1zg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~eht2Lam-dW-anbYO-KBi0g',
  name: { ok: true, value: 'repo' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:08:08.046Z,
  modificationTime: 2026-08-04T21:08:08.046Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4b `create-folder /my-files/_cas-probe/c17b/depth-04/repo refs` - 2026-08-04T14:08:08.2380050-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~ONNrhVNpaTMTNzYCMDQCJg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~NAKENC51ZcIk4ZhectN1zg',
  name: { ok: true, value: 'refs' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:08:13.891Z,
  modificationTime: 2026-08-04T21:08:13.891Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4c `create-folder /my-files/_cas-probe/c17b/depth-04/repo/refs heads` - 2026-08-04T14:08:14.0874062-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~xxbbF-6bcxm_8KrhlJPRjQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~ONNrhVNpaTMTNzYCMDQCJg',
  name: { ok: true, value: 'heads' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:08:17.612Z,
  modificationTime: 2026-08-04T21:08:17.612Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4d `create-folder /my-files/_cas-probe/c17b/depth-04/repo/refs tags` - 2026-08-04T14:08:17.7896826-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~xmv6Olq70oWG-DMFndTzUw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~ONNrhVNpaTMTNzYCMDQCJg',
  name: { ok: true, value: 'tags' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:08:21.834Z,
  modificationTime: 2026-08-04T21:08:21.834Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5a `info /my-files/_cas-probe/c17b/depth-04/repo/refs/tags` (fresh process) - 2026-08-04T14:08:22.0116750-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~xmv6Olq70oWG-DMFndTzUw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~ONNrhVNpaTMTNzYCMDQCJg',
  name: { ok: true, value: 'tags' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:08:21.000Z,
  modificationTime: 2026-08-04T21:08:21.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5b `info /my-files/_cas-probe/c17b/depth-04/repo/refs/heads` (fresh process) - 2026-08-04T14:08:27.6211457-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~xxbbF-6bcxm_8KrhlJPRjQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~ONNrhVNpaTMTNzYCMDQCJg',
  name: { ok: true, value: 'heads' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:08:17.000Z,
  modificationTime: 2026-08-04T21:08:17.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false, claimedModificationTime: undefined },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5c `list /my-files/_cas-probe/c17b/depth-04/repo/refs --json` - 2026-08-04T14:08:32.9642091-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~xxbbF-6bcxm_8KrhlJPRjQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~ONNrhVNpaTMTNzYCMDQCJg","name":{"ok":true,"value":"heads"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:08:17.000Z","modificationTime":"2026-08-04T21:08:17.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~xmv6Olq70oWG-DMFndTzUw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~ONNrhVNpaTMTNzYCMDQCJg","name":{"ok":true,"value":"tags"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:08:21.000Z","modificationTime":"2026-08-04T21:08:21.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 6 `create-folder /my-files/_cas-probe/c17b/depth-04/repo/refs tags` (2nd time; already-exists is correct) - 2026-08-04T14:08:36.1789044-07:00 - exit 1
```
A file or folder with that name already exists
```

**Trial depth-04 classification: CLEAN** (recreate 0; creates repo/refs/heads/tags 0/0/0/0; info tags/heads 0/0; list refs 0; 2nd tags create 1)

#### Trial depth-05 (Variant 2 - Depth / ancestor homonym)

Step 1 `create-folder /my-files/_cas-probe/c17b depth-05` - 2026-08-04T14:08:49.3159719-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~0BPrkSkRdtDctGyzrEjATw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'depth-05' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:08:52.548Z,
  modificationTime: 2026-08-04T21:08:52.548Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 2a `list /my-files/_cas-probe/c17b/depth-05 --json` (trash gate) - 2026-08-04T14:08:52.7519099-07:00 - exit 0
```
[

]
```
Step 2b `trash /my-files/_cas-probe/c17b/depth-05` - 2026-08-04T14:08:56.8194888-07:00 - exit 0
```
✅ depth-05
```
Step 3 RECREATE `create-folder /my-files/_cas-probe/c17b depth-05` - 2026-08-04T14:08:59.6979895-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~zsKfPMvKbCckIDqyzJnsLw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'depth-05' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:09:03.396Z,
  modificationTime: 2026-08-04T21:09:03.396Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4a `create-folder /my-files/_cas-probe/c17b/depth-05 repo` - 2026-08-04T14:09:03.5915813-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~PWRquVi0-E2NEL8cBCnuUw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~zsKfPMvKbCckIDqyzJnsLw',
  name: { ok: true, value: 'repo' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:09:07.854Z,
  modificationTime: 2026-08-04T21:09:07.854Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4b `create-folder /my-files/_cas-probe/c17b/depth-05/repo refs` - 2026-08-04T14:09:08.0647525-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~A6X-b6i7R99Hk5wJF9O9Vw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~PWRquVi0-E2NEL8cBCnuUw',
  name: { ok: true, value: 'refs' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:09:11.608Z,
  modificationTime: 2026-08-04T21:09:11.608Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4c `create-folder /my-files/_cas-probe/c17b/depth-05/repo/refs heads` - 2026-08-04T14:09:11.8157406-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~CDb9YuPZH-ZZVE0wy0VvFw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~A6X-b6i7R99Hk5wJF9O9Vw',
  name: { ok: true, value: 'heads' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:09:18.631Z,
  modificationTime: 2026-08-04T21:09:18.631Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4d `create-folder /my-files/_cas-probe/c17b/depth-05/repo/refs tags` - 2026-08-04T14:09:18.8239908-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~5o0mHqZrPO0gEpcB7wnw2g',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~A6X-b6i7R99Hk5wJF9O9Vw',
  name: { ok: true, value: 'tags' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:09:22.355Z,
  modificationTime: 2026-08-04T21:09:22.355Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5a `info /my-files/_cas-probe/c17b/depth-05/repo/refs/tags` (fresh process) - 2026-08-04T14:09:22.5602650-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~5o0mHqZrPO0gEpcB7wnw2g',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~A6X-b6i7R99Hk5wJF9O9Vw',
  name: { ok: true, value: 'tags' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:09:21.000Z,
  modificationTime: 2026-08-04T21:09:21.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5b `info /my-files/_cas-probe/c17b/depth-05/repo/refs/heads` (fresh process) - 2026-08-04T14:09:27.4201924-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~CDb9YuPZH-ZZVE0wy0VvFw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~A6X-b6i7R99Hk5wJF9O9Vw',
  name: { ok: true, value: 'heads' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:09:18.000Z,
  modificationTime: 2026-08-04T21:09:18.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false, claimedModificationTime: undefined },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5c `list /my-files/_cas-probe/c17b/depth-05/repo/refs --json` - 2026-08-04T14:09:30.3705399-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~CDb9YuPZH-ZZVE0wy0VvFw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~A6X-b6i7R99Hk5wJF9O9Vw","name":{"ok":true,"value":"heads"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:09:18.000Z","modificationTime":"2026-08-04T21:09:18.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~5o0mHqZrPO0gEpcB7wnw2g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~A6X-b6i7R99Hk5wJF9O9Vw","name":{"ok":true,"value":"tags"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:09:21.000Z","modificationTime":"2026-08-04T21:09:21.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 6 `create-folder /my-files/_cas-probe/c17b/depth-05/repo/refs tags` (2nd time; already-exists is correct) - 2026-08-04T14:09:34.5523015-07:00 - exit 1
```
A file or folder with that name already exists
```

**Trial depth-05 classification: CLEAN** (recreate 0; creates repo/refs/heads/tags 0/0/0/0; info tags/heads 0/0; list refs 0; 2nd tags create 1)

#### Trial depth-06 (Variant 2 - Depth / ancestor homonym)

Step 1 `create-folder /my-files/_cas-probe/c17b depth-06` - 2026-08-04T14:09:46.6430545-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~YK07E2OJNMWa_XhYsUYENg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'depth-06' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:09:52.028Z,
  modificationTime: 2026-08-04T21:09:52.028Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 2a `list /my-files/_cas-probe/c17b/depth-06 --json` (trash gate) - 2026-08-04T14:09:52.2404083-07:00 - exit 0
```
[

]
```
Step 2b `trash /my-files/_cas-probe/c17b/depth-06` - 2026-08-04T14:09:55.3881343-07:00 - exit 0
```
✅ depth-06
```
Step 3 RECREATE `create-folder /my-files/_cas-probe/c17b depth-06` - 2026-08-04T14:09:57.9627128-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~IZ3yph7JUU1us1o11tj1Eg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'depth-06' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:10:00.454Z,
  modificationTime: 2026-08-04T21:10:00.454Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4a `create-folder /my-files/_cas-probe/c17b/depth-06 repo` - 2026-08-04T14:10:00.6362274-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~DskXjeAF8mR_JXLAp9yInQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~IZ3yph7JUU1us1o11tj1Eg',
  name: { ok: true, value: 'repo' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:10:04.853Z,
  modificationTime: 2026-08-04T21:10:04.853Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4b `create-folder /my-files/_cas-probe/c17b/depth-06/repo refs` - 2026-08-04T14:10:05.0457247-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~Bx_F56vZ-L8bmH939Ksqrg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~DskXjeAF8mR_JXLAp9yInQ',
  name: { ok: true, value: 'refs' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:10:08.636Z,
  modificationTime: 2026-08-04T21:10:08.636Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4c `create-folder /my-files/_cas-probe/c17b/depth-06/repo/refs heads` - 2026-08-04T14:10:08.8362072-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~m5YgodhCysB8gNuimcVq5g',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~Bx_F56vZ-L8bmH939Ksqrg',
  name: { ok: true, value: 'heads' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:10:14.958Z,
  modificationTime: 2026-08-04T21:10:14.958Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4d `create-folder /my-files/_cas-probe/c17b/depth-06/repo/refs tags` - 2026-08-04T14:10:15.1554080-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~8u4pfxWkNLZdxwoHRIpOXw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~Bx_F56vZ-L8bmH939Ksqrg',
  name: { ok: true, value: 'tags' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:10:18.298Z,
  modificationTime: 2026-08-04T21:10:18.298Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5a `info /my-files/_cas-probe/c17b/depth-06/repo/refs/tags` (fresh process) - 2026-08-04T14:10:18.4589519-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~8u4pfxWkNLZdxwoHRIpOXw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~Bx_F56vZ-L8bmH939Ksqrg',
  name: { ok: true, value: 'tags' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:10:17.000Z,
  modificationTime: 2026-08-04T21:10:17.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5b `info /my-files/_cas-probe/c17b/depth-06/repo/refs/heads` (fresh process) - 2026-08-04T14:10:23.7995124-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~m5YgodhCysB8gNuimcVq5g',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~Bx_F56vZ-L8bmH939Ksqrg',
  name: { ok: true, value: 'heads' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:10:14.000Z,
  modificationTime: 2026-08-04T21:10:14.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false, claimedModificationTime: undefined },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5c `list /my-files/_cas-probe/c17b/depth-06/repo/refs --json` - 2026-08-04T14:10:29.1789909-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~m5YgodhCysB8gNuimcVq5g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~Bx_F56vZ-L8bmH939Ksqrg","name":{"ok":true,"value":"heads"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:10:14.000Z","modificationTime":"2026-08-04T21:10:14.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~8u4pfxWkNLZdxwoHRIpOXw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~Bx_F56vZ-L8bmH939Ksqrg","name":{"ok":true,"value":"tags"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:10:17.000Z","modificationTime":"2026-08-04T21:10:17.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 6 `create-folder /my-files/_cas-probe/c17b/depth-06/repo/refs tags` (2nd time; already-exists is correct) - 2026-08-04T14:10:33.5053952-07:00 - exit 1
```
A file or folder with that name already exists
```

**Trial depth-06 classification: CLEAN** (recreate 0; creates repo/refs/heads/tags 0/0/0/0; info tags/heads 0/0; list refs 0; 2nd tags create 1)

#### Trial depth-07 (Variant 2 - Depth / ancestor homonym)

Step 1 `create-folder /my-files/_cas-probe/c17b depth-07` - 2026-08-04T14:10:45.9761575-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~70Kc6effEh2nrFixX7m_uA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'depth-07' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:10:49.709Z,
  modificationTime: 2026-08-04T21:10:49.709Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 2a `list /my-files/_cas-probe/c17b/depth-07 --json` (trash gate) - 2026-08-04T14:10:49.9211569-07:00 - exit 0
```
[

]
```
Step 2b `trash /my-files/_cas-probe/c17b/depth-07` - 2026-08-04T14:10:54.3002236-07:00 - exit 0
```
✅ depth-07
```
Step 3 RECREATE `create-folder /my-files/_cas-probe/c17b depth-07` - 2026-08-04T14:10:59.7256721-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~hiP8Mx7BiCFQsr_c-n6Yaw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'depth-07' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:11:03.564Z,
  modificationTime: 2026-08-04T21:11:03.564Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4a `create-folder /my-files/_cas-probe/c17b/depth-07 repo` - 2026-08-04T14:11:03.7643938-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~P9Wrljx-E0xLd-VOro1OoQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~hiP8Mx7BiCFQsr_c-n6Yaw',
  name: { ok: true, value: 'repo' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:11:06.832Z,
  modificationTime: 2026-08-04T21:11:06.832Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4b `create-folder /my-files/_cas-probe/c17b/depth-07/repo refs` - 2026-08-04T14:11:07.0173342-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~nrQDEM_opPG77lkDXXErew',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~P9Wrljx-E0xLd-VOro1OoQ',
  name: { ok: true, value: 'refs' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:11:13.244Z,
  modificationTime: 2026-08-04T21:11:13.244Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4c `create-folder /my-files/_cas-probe/c17b/depth-07/repo/refs heads` - 2026-08-04T14:11:13.4676272-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~Ui5QlpAtfKq92nQnQuA52w',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~nrQDEM_opPG77lkDXXErew',
  name: { ok: true, value: 'heads' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:11:18.391Z,
  modificationTime: 2026-08-04T21:11:18.391Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4d `create-folder /my-files/_cas-probe/c17b/depth-07/repo/refs tags` - 2026-08-04T14:11:18.6217291-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~lirt6B1YCjsTKEEDEqEXbg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~nrQDEM_opPG77lkDXXErew',
  name: { ok: true, value: 'tags' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:11:24.564Z,
  modificationTime: 2026-08-04T21:11:24.564Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5a `info /my-files/_cas-probe/c17b/depth-07/repo/refs/tags` (fresh process) - 2026-08-04T14:11:24.7587721-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~lirt6B1YCjsTKEEDEqEXbg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~nrQDEM_opPG77lkDXXErew',
  name: { ok: true, value: 'tags' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:11:24.000Z,
  modificationTime: 2026-08-04T21:11:24.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5b `info /my-files/_cas-probe/c17b/depth-07/repo/refs/heads` (fresh process) - 2026-08-04T14:11:28.0454420-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~Ui5QlpAtfKq92nQnQuA52w',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~nrQDEM_opPG77lkDXXErew',
  name: { ok: true, value: 'heads' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:11:17.000Z,
  modificationTime: 2026-08-04T21:11:17.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false, claimedModificationTime: undefined },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5c `list /my-files/_cas-probe/c17b/depth-07/repo/refs --json` - 2026-08-04T14:11:31.8841348-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Ui5QlpAtfKq92nQnQuA52w","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~nrQDEM_opPG77lkDXXErew","name":{"ok":true,"value":"heads"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:11:17.000Z","modificationTime":"2026-08-04T21:11:17.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~lirt6B1YCjsTKEEDEqEXbg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~nrQDEM_opPG77lkDXXErew","name":{"ok":true,"value":"tags"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:11:24.000Z","modificationTime":"2026-08-04T21:11:24.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 6 `create-folder /my-files/_cas-probe/c17b/depth-07/repo/refs tags` (2nd time; already-exists is correct) - 2026-08-04T14:11:36.0742494-07:00 - exit 1
```
A file or folder with that name already exists
```

**Trial depth-07 classification: CLEAN** (recreate 0; creates repo/refs/heads/tags 0/0/0/0; info tags/heads 0/0; list refs 0; 2nd tags create 1)

#### Trial depth-08 (Variant 2 - Depth / ancestor homonym)

Step 1 `create-folder /my-files/_cas-probe/c17b depth-08` - 2026-08-04T14:11:46.7184608-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~t8N0kk-JEQ7ji1JlgrUBYA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'depth-08' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:11:51.619Z,
  modificationTime: 2026-08-04T21:11:51.619Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 2a `list /my-files/_cas-probe/c17b/depth-08 --json` (trash gate) - 2026-08-04T14:11:51.8512585-07:00 - exit 0
```
[

]
```
Step 2b `trash /my-files/_cas-probe/c17b/depth-08` - 2026-08-04T14:11:56.8787501-07:00 - exit 0
```
✅ depth-08
```
Step 3 RECREATE `create-folder /my-files/_cas-probe/c17b depth-08` - 2026-08-04T14:12:00.8282564-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~zp0bn7M_uJWhzRN6wMiVNQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'depth-08' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:12:06.209Z,
  modificationTime: 2026-08-04T21:12:06.209Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4a `create-folder /my-files/_cas-probe/c17b/depth-08 repo` - 2026-08-04T14:12:06.4100922-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~lxn4lU38WGnZ8Qki-nsPkA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~zp0bn7M_uJWhzRN6wMiVNQ',
  name: { ok: true, value: 'repo' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:12:12.722Z,
  modificationTime: 2026-08-04T21:12:12.722Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4b `create-folder /my-files/_cas-probe/c17b/depth-08/repo refs` - 2026-08-04T14:12:12.9151337-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~Ho71_M0bQ3TwdNGqD6UFfw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~lxn4lU38WGnZ8Qki-nsPkA',
  name: { ok: true, value: 'refs' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:12:16.381Z,
  modificationTime: 2026-08-04T21:12:16.381Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4c `create-folder /my-files/_cas-probe/c17b/depth-08/repo/refs heads` - 2026-08-04T14:12:16.5834426-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~6-d2iaooJJDNcpyDGur8aQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~Ho71_M0bQ3TwdNGqD6UFfw',
  name: { ok: true, value: 'heads' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:12:23.140Z,
  modificationTime: 2026-08-04T21:12:23.140Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4d `create-folder /my-files/_cas-probe/c17b/depth-08/repo/refs tags` - 2026-08-04T14:12:23.3249420-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~6UPyQ42zNscqcmo7y3D4mA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~Ho71_M0bQ3TwdNGqD6UFfw',
  name: { ok: true, value: 'tags' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:12:28.979Z,
  modificationTime: 2026-08-04T21:12:28.979Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5a `info /my-files/_cas-probe/c17b/depth-08/repo/refs/tags` (fresh process) - 2026-08-04T14:12:29.1668783-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~6UPyQ42zNscqcmo7y3D4mA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~Ho71_M0bQ3TwdNGqD6UFfw',
  name: { ok: true, value: 'tags' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:12:28.000Z,
  modificationTime: 2026-08-04T21:12:28.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5b `info /my-files/_cas-probe/c17b/depth-08/repo/refs/heads` (fresh process) - 2026-08-04T14:12:33.7784850-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~6-d2iaooJJDNcpyDGur8aQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~Ho71_M0bQ3TwdNGqD6UFfw',
  name: { ok: true, value: 'heads' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:12:22.000Z,
  modificationTime: 2026-08-04T21:12:22.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false, claimedModificationTime: undefined },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5c `list /my-files/_cas-probe/c17b/depth-08/repo/refs --json` - 2026-08-04T14:12:38.0994547-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6-d2iaooJJDNcpyDGur8aQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~Ho71_M0bQ3TwdNGqD6UFfw","name":{"ok":true,"value":"heads"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:12:22.000Z","modificationTime":"2026-08-04T21:12:22.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6UPyQ42zNscqcmo7y3D4mA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~Ho71_M0bQ3TwdNGqD6UFfw","name":{"ok":true,"value":"tags"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:12:28.000Z","modificationTime":"2026-08-04T21:12:28.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 6 `create-folder /my-files/_cas-probe/c17b/depth-08/repo/refs tags` (2nd time; already-exists is correct) - 2026-08-04T14:12:41.3846290-07:00 - exit 1
```
A file or folder with that name already exists
```

**Trial depth-08 classification: CLEAN** (recreate 0; creates repo/refs/heads/tags 0/0/0/0; info tags/heads 0/0; list refs 0; 2nd tags create 1)

#### Trial depth-09 (Variant 2 - Depth / ancestor homonym)

Step 1 `create-folder /my-files/_cas-probe/c17b depth-09` - 2026-08-04T14:12:54.9103576-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~E0K8QR3fgGV6m4ZpKl6w0A',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'depth-09' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:12:57.547Z,
  modificationTime: 2026-08-04T21:12:57.547Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 2a `list /my-files/_cas-probe/c17b/depth-09 --json` (trash gate) - 2026-08-04T14:12:57.7400157-07:00 - exit 0
```
[

]
```
Step 2b `trash /my-files/_cas-probe/c17b/depth-09` - 2026-08-04T14:13:03.3927460-07:00 - exit 0
```
✅ depth-09
```
Step 3 RECREATE `create-folder /my-files/_cas-probe/c17b depth-09` - 2026-08-04T14:13:07.3407067-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~72iZp0eSHsi5_GlXpukoQw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'depth-09' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:13:09.755Z,
  modificationTime: 2026-08-04T21:13:09.755Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4a `create-folder /my-files/_cas-probe/c17b/depth-09 repo` - 2026-08-04T14:13:09.9362594-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~0JZWfuFjGpPRdHhQTyvxwg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~72iZp0eSHsi5_GlXpukoQw',
  name: { ok: true, value: 'repo' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:13:14.383Z,
  modificationTime: 2026-08-04T21:13:14.383Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4b `create-folder /my-files/_cas-probe/c17b/depth-09/repo refs` - 2026-08-04T14:13:14.5659226-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~ewxT5rUyghT410EhzErSQg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~0JZWfuFjGpPRdHhQTyvxwg',
  name: { ok: true, value: 'refs' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:13:18.207Z,
  modificationTime: 2026-08-04T21:13:18.207Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4c `create-folder /my-files/_cas-probe/c17b/depth-09/repo/refs heads` - 2026-08-04T14:13:18.3745699-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~l5lTR8Qnei5eCpU8-i5WBw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~ewxT5rUyghT410EhzErSQg',
  name: { ok: true, value: 'heads' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:13:22.819Z,
  modificationTime: 2026-08-04T21:13:22.819Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4d `create-folder /my-files/_cas-probe/c17b/depth-09/repo/refs tags` - 2026-08-04T14:13:22.9939171-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~U6to0SZraR67mqmCWJFZTA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~ewxT5rUyghT410EhzErSQg',
  name: { ok: true, value: 'tags' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:13:25.953Z,
  modificationTime: 2026-08-04T21:13:25.953Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5a `info /my-files/_cas-probe/c17b/depth-09/repo/refs/tags` (fresh process) - 2026-08-04T14:13:26.1634819-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~U6to0SZraR67mqmCWJFZTA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~ewxT5rUyghT410EhzErSQg',
  name: { ok: true, value: 'tags' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:13:25.000Z,
  modificationTime: 2026-08-04T21:13:25.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5b `info /my-files/_cas-probe/c17b/depth-09/repo/refs/heads` (fresh process) - 2026-08-04T14:13:30.7726883-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~l5lTR8Qnei5eCpU8-i5WBw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~ewxT5rUyghT410EhzErSQg',
  name: { ok: true, value: 'heads' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:13:22.000Z,
  modificationTime: 2026-08-04T21:13:22.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false, claimedModificationTime: undefined },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5c `list /my-files/_cas-probe/c17b/depth-09/repo/refs --json` - 2026-08-04T14:13:35.9996491-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~l5lTR8Qnei5eCpU8-i5WBw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~ewxT5rUyghT410EhzErSQg","name":{"ok":true,"value":"heads"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:13:22.000Z","modificationTime":"2026-08-04T21:13:22.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~U6to0SZraR67mqmCWJFZTA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~ewxT5rUyghT410EhzErSQg","name":{"ok":true,"value":"tags"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:13:25.000Z","modificationTime":"2026-08-04T21:13:25.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 6 `create-folder /my-files/_cas-probe/c17b/depth-09/repo/refs tags` (2nd time; already-exists is correct) - 2026-08-04T14:13:39.0185658-07:00 - exit 1
```
A file or folder with that name already exists
```

**Trial depth-09 classification: CLEAN** (recreate 0; creates repo/refs/heads/tags 0/0/0/0; info tags/heads 0/0; list refs 0; 2nd tags create 1)

#### Trial depth-10 (Variant 2 - Depth / ancestor homonym)

Step 1 `create-folder /my-files/_cas-probe/c17b depth-10` - 2026-08-04T14:13:52.1811639-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~t0f-34wHBTqdKK6VMlCThA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'depth-10' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:13:54.663Z,
  modificationTime: 2026-08-04T21:13:54.663Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 2a `list /my-files/_cas-probe/c17b/depth-10 --json` (trash gate) - 2026-08-04T14:13:54.8640375-07:00 - exit 0
```
[

]
```
Step 2b `trash /my-files/_cas-probe/c17b/depth-10` - 2026-08-04T14:13:58.9425347-07:00 - exit 0
```
✅ depth-10
```
Step 3 RECREATE `create-folder /my-files/_cas-probe/c17b depth-10` - 2026-08-04T14:14:03.3989492-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~rAjSTiPpATCEhhUBujaEJg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'depth-10' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:14:08.622Z,
  modificationTime: 2026-08-04T21:14:08.622Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4a `create-folder /my-files/_cas-probe/c17b/depth-10 repo` - 2026-08-04T14:14:08.8024734-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~CJWTlv0CDNo9iC5e2tR-PA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~rAjSTiPpATCEhhUBujaEJg',
  name: { ok: true, value: 'repo' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:14:14.945Z,
  modificationTime: 2026-08-04T21:14:14.945Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4b `create-folder /my-files/_cas-probe/c17b/depth-10/repo refs` - 2026-08-04T14:14:15.1675511-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~EZ3YbbhWvMWnsWSbxK9oGQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~CJWTlv0CDNo9iC5e2tR-PA',
  name: { ok: true, value: 'refs' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:14:20.990Z,
  modificationTime: 2026-08-04T21:14:20.990Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4c `create-folder /my-files/_cas-probe/c17b/depth-10/repo/refs heads` - 2026-08-04T14:14:21.1597592-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~gF9z4g6KiTGK_aV9sALkSQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~EZ3YbbhWvMWnsWSbxK9oGQ',
  name: { ok: true, value: 'heads' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:14:25.964Z,
  modificationTime: 2026-08-04T21:14:25.964Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 4d `create-folder /my-files/_cas-probe/c17b/depth-10/repo/refs tags` - 2026-08-04T14:14:26.1573951-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~4EDW32EY1V0pvacq-WT5EQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~EZ3YbbhWvMWnsWSbxK9oGQ',
  name: { ok: true, value: 'tags' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:14:29.671Z,
  modificationTime: 2026-08-04T21:14:29.671Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5a `info /my-files/_cas-probe/c17b/depth-10/repo/refs/tags` (fresh process) - 2026-08-04T14:14:29.8121212-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~4EDW32EY1V0pvacq-WT5EQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~EZ3YbbhWvMWnsWSbxK9oGQ',
  name: { ok: true, value: 'tags' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:14:29.000Z,
  modificationTime: 2026-08-04T21:14:29.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5b `info /my-files/_cas-probe/c17b/depth-10/repo/refs/heads` (fresh process) - 2026-08-04T14:14:35.7243024-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~gF9z4g6KiTGK_aV9sALkSQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~EZ3YbbhWvMWnsWSbxK9oGQ',
  name: { ok: true, value: 'heads' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:14:25.000Z,
  modificationTime: 2026-08-04T21:14:25.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false, claimedModificationTime: undefined },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 5c `list /my-files/_cas-probe/c17b/depth-10/repo/refs --json` - 2026-08-04T14:14:39.9162756-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~gF9z4g6KiTGK_aV9sALkSQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~EZ3YbbhWvMWnsWSbxK9oGQ","name":{"ok":true,"value":"heads"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:14:25.000Z","modificationTime":"2026-08-04T21:14:25.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~4EDW32EY1V0pvacq-WT5EQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~EZ3YbbhWvMWnsWSbxK9oGQ","name":{"ok":true,"value":"tags"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:14:29.000Z","modificationTime":"2026-08-04T21:14:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 6 `create-folder /my-files/_cas-probe/c17b/depth-10/repo/refs tags` (2nd time; already-exists is correct) - 2026-08-04T14:14:43.6945884-07:00 - exit 1
```
A file or folder with that name already exists
```

**Trial depth-10 classification: CLEAN** (recreate 0; creates repo/refs/heads/tags 0/0/0/0; info tags/heads 0/0; list refs 0; 2nd tags create 1)

**Variant 2 tally: 10 trials run (depth-01 .. depth-10), 10 CLEAN, 0 REPRODUCTION, 0 OTHER.**

---

## Variant 3 - Generations (THREE trashed homonyms of the same name before recreating)


#### Trial gen-01 (Variant 3 - Generations / three trashed homonyms)

Step 1.1-a `create-folder /my-files/_cas-probe/c17b gen-01` (generation 1) - 2026-08-04T14:15:27.1762140-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~iWPiagrVuXicmjkToLgtAA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-01' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:15:31.163Z,
  modificationTime: 2026-08-04T21:15:31.163Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.1-b `list /my-files/_cas-probe/c17b/gen-01 --json` (trash gate) - 2026-08-04T14:15:31.3630265-07:00 - exit 0
```
[

]
```
Step 1.1-c `trash /my-files/_cas-probe/c17b/gen-01` - 2026-08-04T14:15:36.0230448-07:00 - exit 0
```
✅ gen-01
```
Step 1.2-pre `info /my-files/_cas-probe/c17b/gen-01` (fresh process, before recreate; expect absent) - 2026-08-04T14:15:39.9514148-07:00 - exit 1
```
Node not found: gen-01
```
Step 1.2-a `create-folder /my-files/_cas-probe/c17b gen-01` (generation 2) - 2026-08-04T14:15:42.7017605-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~EJc2m56JMcohaoqwHwnqlQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-01' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:15:46.992Z,
  modificationTime: 2026-08-04T21:15:46.992Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.2-b `list /my-files/_cas-probe/c17b/gen-01 --json` (trash gate) - 2026-08-04T14:15:47.1817004-07:00 - exit 0
```
[

]
```
Step 1.2-c `trash /my-files/_cas-probe/c17b/gen-01` - 2026-08-04T14:15:51.7497464-07:00 - exit 0
```
✅ gen-01
```
Step 1.3-pre `info /my-files/_cas-probe/c17b/gen-01` (fresh process, before recreate; expect absent) - 2026-08-04T14:15:54.2166382-07:00 - exit 1
```
Node not found: gen-01
```
Step 1.3-a `create-folder /my-files/_cas-probe/c17b gen-01` (generation 3) - 2026-08-04T14:15:57.8981000-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~ncJ2FduQM1ThnjmIgH6Uqg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-01' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:16:01.965Z,
  modificationTime: 2026-08-04T21:16:01.965Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.3-b `list /my-files/_cas-probe/c17b/gen-01 --json` (trash gate) - 2026-08-04T14:16:02.1443588-07:00 - exit 0
```
[

]
```
Step 1.3-c `trash /my-files/_cas-probe/c17b/gen-01` - 2026-08-04T14:16:05.1265016-07:00 - exit 0
```
✅ gen-01
```
Step 2a `info /my-files/_cas-probe/c17b/gen-01` (fresh process, 3 trashed generations present; expect absent) - 2026-08-04T14:16:09.8815856-07:00 - exit 1
```
Node not found: gen-01
```
Step 2b RECREATE `create-folder /my-files/_cas-probe/c17b gen-01` - 2026-08-04T14:16:12.4427379-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~Fh1ZdDLBW1zUvD-jBenl4g',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-01' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:16:15.116Z,
  modificationTime: 2026-08-04T21:16:15.116Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3a `info /my-files/_cas-probe/c17b/gen-01` (fresh process) - 2026-08-04T14:16:15.3179176-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~Fh1ZdDLBW1zUvD-jBenl4g',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-01' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:16:14.000Z,
  modificationTime: 2026-08-04T21:16:14.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3b `list /my-files/_cas-probe/c17b --json` - 2026-08-04T14:16:19.0413253-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:57:39.000Z","modificationTime":"2026-08-04T20:57:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:58:29.000Z","modificationTime":"2026-08-04T20:58:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:08.000Z","modificationTime":"2026-08-04T20:59:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:39.000Z","modificationTime":"2026-08-04T20:59:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~bGWhebPsVOLM0E43JuNSEg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:15.000Z","modificationTime":"2026-08-04T21:00:15.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~k2ltsJ374d3nDXuylx1LHg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:51.000Z","modificationTime":"2026-08-04T21:00:51.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~v_M-faPpHavHoZAgQYT16A","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:01:29.000Z","modificationTime":"2026-08-04T21:01:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~FJ3e7KVTX6L-KR8eRoYaCw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:01.000Z","modificationTime":"2026-08-04T21:02:01.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~1jszxKEh7pipkm11xfv7oQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:33.000Z","modificationTime":"2026-08-04T21:02:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~wKOeh9MS6iGmXEiVci9H1w","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:02.000Z","modificationTime":"2026-08-04T21:03:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~XU3VjNpul9sR9HcQR60UkA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-11"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:37.000Z","modificationTime":"2026-08-04T21:03:37.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zo8FDPjVnwgfwvZhmcOHJQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:04:43.000Z","modificationTime":"2026-08-04T21:04:43.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~so1Xnd4CFVTmnSkGYJc1QQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:05:59.000Z","modificationTime":"2026-08-04T21:05:59.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~gnyUjXCeWGdaTaD9bGOjNg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:07:03.000Z","modificationTime":"2026-08-04T21:07:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~eht2Lam-dW-anbYO-KBi0g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:08:02.000Z","modificationTime":"2026-08-04T21:08:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zsKfPMvKbCckIDqyzJnsLw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:09:02.000Z","modificationTime":"2026-08-04T21:09:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~IZ3yph7JUU1us1o11tj1Eg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:10:00.000Z","modificationTime":"2026-08-04T21:10:00.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~hiP8Mx7BiCFQsr_c-n6Yaw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:11:03.000Z","modificationTime":"2026-08-04T21:11:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zp0bn7M_uJWhzRN6wMiVNQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:12:05.000Z","modificationTime":"2026-08-04T21:12:05.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~72iZp0eSHsi5_GlXpukoQw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:13:09.000Z","modificationTime":"2026-08-04T21:13:09.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~rAjSTiPpATCEhhUBujaEJg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:14:08.000Z","modificationTime":"2026-08-04T21:14:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Fh1ZdDLBW1zUvD-jBenl4g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:16:14.000Z","modificationTime":"2026-08-04T21:16:14.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 3b row count for name `gen-01`: 1 ; trashTime occurrences in listing: 0
Step 3c `create-folder /my-files/_cas-probe/c17b/gen-01 child` - 2026-08-04T14:16:22.7747952-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~DtmC-hkevKXhaqDFJGb1_Q',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~Fh1ZdDLBW1zUvD-jBenl4g',
  name: { ok: true, value: 'child' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:16:26.847Z,
  modificationTime: 2026-08-04T21:16:26.847Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3d `create-folder /my-files/_cas-probe/c17b/gen-01/child sub` - 2026-08-04T14:16:27.0628862-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~P43XMplFsZ8xk2R0itDb4g',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~DtmC-hkevKXhaqDFJGb1_Q',
  name: { ok: true, value: 'sub' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:16:30.320Z,
  modificationTime: 2026-08-04T21:16:30.320Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3e `info /my-files/_cas-probe/c17b/gen-01/child/sub` (fresh process) - 2026-08-04T14:16:30.5179302-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~P43XMplFsZ8xk2R0itDb4g',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~DtmC-hkevKXhaqDFJGb1_Q',
  name: { ok: true, value: 'sub' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:16:29.000Z,
  modificationTime: 2026-08-04T21:16:29.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

**Trial gen-01 classification: CLEAN** (cycle creates 0/0/0; pre-recreate infos 1/1, final pre-info 1; recreate 0; info 0; rows 1; child 0; sub 0; info sub 0)

#### Trial gen-02 (Variant 3 - Generations / three trashed homonyms)

Step 1.1-a `create-folder /my-files/_cas-probe/c17b gen-02` (generation 1) - 2026-08-04T14:16:42.2733761-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~Rb_h87o4bKYiDpyX80dFKQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-02' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:16:47.110Z,
  modificationTime: 2026-08-04T21:16:47.110Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.1-b `list /my-files/_cas-probe/c17b/gen-02 --json` (trash gate) - 2026-08-04T14:16:47.3427269-07:00 - exit 0
```
[

]
```
Step 1.1-c `trash /my-files/_cas-probe/c17b/gen-02` - 2026-08-04T14:16:50.3649867-07:00 - exit 0
```
✅ gen-02
```
Step 1.2-pre `info /my-files/_cas-probe/c17b/gen-02` (fresh process, before recreate; expect absent) - 2026-08-04T14:16:55.1178457-07:00 - exit 1
```
Node not found: gen-02
```
Step 1.2-a `create-folder /my-files/_cas-probe/c17b gen-02` (generation 2) - 2026-08-04T14:16:59.4416315-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~mQ4qMJR8s_pY6nPLJzDb9w',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-02' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:17:03.564Z,
  modificationTime: 2026-08-04T21:17:03.564Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.2-b `list /my-files/_cas-probe/c17b/gen-02 --json` (trash gate) - 2026-08-04T14:17:03.7691323-07:00 - exit 0
```
[

]
```
Step 1.2-c `trash /my-files/_cas-probe/c17b/gen-02` - 2026-08-04T14:17:08.0738720-07:00 - exit 0
```
✅ gen-02
```
Step 1.3-pre `info /my-files/_cas-probe/c17b/gen-02` (fresh process, before recreate; expect absent) - 2026-08-04T14:17:12.4409962-07:00 - exit 1
```
Node not found: gen-02
```
Step 1.3-a `create-folder /my-files/_cas-probe/c17b gen-02` (generation 3) - 2026-08-04T14:17:15.9987280-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~j4NfB2rP6gSb5mxBjGjBnA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-02' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:17:20.416Z,
  modificationTime: 2026-08-04T21:17:20.416Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.3-b `list /my-files/_cas-probe/c17b/gen-02 --json` (trash gate) - 2026-08-04T14:17:20.6109394-07:00 - exit 0
```
[

]
```
Step 1.3-c `trash /my-files/_cas-probe/c17b/gen-02` - 2026-08-04T14:17:23.8921030-07:00 - exit 0
```
✅ gen-02
```
Step 2a `info /my-files/_cas-probe/c17b/gen-02` (fresh process, 3 trashed generations present; expect absent) - 2026-08-04T14:17:26.5528357-07:00 - exit 1
```
Node not found: gen-02
```
Step 2b RECREATE `create-folder /my-files/_cas-probe/c17b gen-02` - 2026-08-04T14:17:30.0546147-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~Tn5s58NB8uM2yZLf7rt0rw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-02' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:17:33.969Z,
  modificationTime: 2026-08-04T21:17:33.969Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3a `info /my-files/_cas-probe/c17b/gen-02` (fresh process) - 2026-08-04T14:17:34.1814114-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~Tn5s58NB8uM2yZLf7rt0rw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-02' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:17:33.000Z,
  modificationTime: 2026-08-04T21:17:33.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3b `list /my-files/_cas-probe/c17b --json` - 2026-08-04T14:17:38.3881970-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:57:39.000Z","modificationTime":"2026-08-04T20:57:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:58:29.000Z","modificationTime":"2026-08-04T20:58:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:08.000Z","modificationTime":"2026-08-04T20:59:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:39.000Z","modificationTime":"2026-08-04T20:59:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~bGWhebPsVOLM0E43JuNSEg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:15.000Z","modificationTime":"2026-08-04T21:00:15.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~k2ltsJ374d3nDXuylx1LHg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:51.000Z","modificationTime":"2026-08-04T21:00:51.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~v_M-faPpHavHoZAgQYT16A","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:01:29.000Z","modificationTime":"2026-08-04T21:01:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~FJ3e7KVTX6L-KR8eRoYaCw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:01.000Z","modificationTime":"2026-08-04T21:02:01.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~1jszxKEh7pipkm11xfv7oQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:33.000Z","modificationTime":"2026-08-04T21:02:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~wKOeh9MS6iGmXEiVci9H1w","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:02.000Z","modificationTime":"2026-08-04T21:03:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~XU3VjNpul9sR9HcQR60UkA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-11"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:37.000Z","modificationTime":"2026-08-04T21:03:37.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zo8FDPjVnwgfwvZhmcOHJQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:04:43.000Z","modificationTime":"2026-08-04T21:04:43.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~so1Xnd4CFVTmnSkGYJc1QQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:05:59.000Z","modificationTime":"2026-08-04T21:05:59.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~gnyUjXCeWGdaTaD9bGOjNg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:07:03.000Z","modificationTime":"2026-08-04T21:07:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~eht2Lam-dW-anbYO-KBi0g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:08:02.000Z","modificationTime":"2026-08-04T21:08:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zsKfPMvKbCckIDqyzJnsLw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:09:02.000Z","modificationTime":"2026-08-04T21:09:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~IZ3yph7JUU1us1o11tj1Eg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:10:00.000Z","modificationTime":"2026-08-04T21:10:00.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~hiP8Mx7BiCFQsr_c-n6Yaw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:11:03.000Z","modificationTime":"2026-08-04T21:11:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zp0bn7M_uJWhzRN6wMiVNQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:12:05.000Z","modificationTime":"2026-08-04T21:12:05.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~72iZp0eSHsi5_GlXpukoQw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:13:09.000Z","modificationTime":"2026-08-04T21:13:09.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~rAjSTiPpATCEhhUBujaEJg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:14:08.000Z","modificationTime":"2026-08-04T21:14:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Fh1ZdDLBW1zUvD-jBenl4g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:16:14.000Z","modificationTime":"2026-08-04T21:16:14.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Tn5s58NB8uM2yZLf7rt0rw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:17:33.000Z","modificationTime":"2026-08-04T21:17:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 3b row count for name `gen-02`: 1 ; trashTime occurrences in listing: 0
Step 3c `create-folder /my-files/_cas-probe/c17b/gen-02 child` - 2026-08-04T14:17:43.4643346-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~KSyxPPeTUGdv4PSWe-cFCQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~Tn5s58NB8uM2yZLf7rt0rw',
  name: { ok: true, value: 'child' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:17:47.446Z,
  modificationTime: 2026-08-04T21:17:47.446Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3d `create-folder /my-files/_cas-probe/c17b/gen-02/child sub` - 2026-08-04T14:17:47.6347140-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~m9fUDmApTARAPX63LXR_uA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~KSyxPPeTUGdv4PSWe-cFCQ',
  name: { ok: true, value: 'sub' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:17:50.603Z,
  modificationTime: 2026-08-04T21:17:50.603Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3e `info /my-files/_cas-probe/c17b/gen-02/child/sub` (fresh process) - 2026-08-04T14:17:50.8132647-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~m9fUDmApTARAPX63LXR_uA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~KSyxPPeTUGdv4PSWe-cFCQ',
  name: { ok: true, value: 'sub' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:17:50.000Z,
  modificationTime: 2026-08-04T21:17:50.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

**Trial gen-02 classification: CLEAN** (cycle creates 0/0/0; pre-recreate infos 1/1, final pre-info 1; recreate 0; info 0; rows 1; child 0; sub 0; info sub 0)

#### Trial gen-03 (Variant 3 - Generations / three trashed homonyms)

Step 1.1-a `create-folder /my-files/_cas-probe/c17b gen-03` (generation 1) - 2026-08-04T14:18:02.3972911-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~satuVHsVNY-uyItLrMNUJA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-03' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:18:07.489Z,
  modificationTime: 2026-08-04T21:18:07.489Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.1-b `list /my-files/_cas-probe/c17b/gen-03 --json` (trash gate) - 2026-08-04T14:18:07.7178896-07:00 - exit 0
```
[

]
```
Step 1.1-c `trash /my-files/_cas-probe/c17b/gen-03` - 2026-08-04T14:18:12.8760150-07:00 - exit 0
```
✅ gen-03
```
Step 1.2-pre `info /my-files/_cas-probe/c17b/gen-03` (fresh process, before recreate; expect absent) - 2026-08-04T14:18:17.6476713-07:00 - exit 1
```
Node not found: gen-03
```
Step 1.2-a `create-folder /my-files/_cas-probe/c17b gen-03` (generation 2) - 2026-08-04T14:18:20.6274093-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~clf097du5ouRxxfA7F2tUQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-03' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:18:23.053Z,
  modificationTime: 2026-08-04T21:18:23.053Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.2-b `list /my-files/_cas-probe/c17b/gen-03 --json` (trash gate) - 2026-08-04T14:18:23.2467789-07:00 - exit 0
```
[

]
```
Step 1.2-c `trash /my-files/_cas-probe/c17b/gen-03` - 2026-08-04T14:18:28.4519567-07:00 - exit 0
```
✅ gen-03
```
Step 1.3-pre `info /my-files/_cas-probe/c17b/gen-03` (fresh process, before recreate; expect absent) - 2026-08-04T14:18:32.3622023-07:00 - exit 1
```
Node not found: gen-03
```
Step 1.3-a `create-folder /my-files/_cas-probe/c17b gen-03` (generation 3) - 2026-08-04T14:18:37.0806957-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~9poccuxCiBSFozque9Jwdw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-03' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:18:40.039Z,
  modificationTime: 2026-08-04T21:18:40.039Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.3-b `list /my-files/_cas-probe/c17b/gen-03 --json` (trash gate) - 2026-08-04T14:18:40.2216074-07:00 - exit 0
```
[

]
```
Step 1.3-c `trash /my-files/_cas-probe/c17b/gen-03` - 2026-08-04T14:18:44.3031075-07:00 - exit 0
```
✅ gen-03
```
Step 2a `info /my-files/_cas-probe/c17b/gen-03` (fresh process, 3 trashed generations present; expect absent) - 2026-08-04T14:18:49.0702571-07:00 - exit 1
```
Node not found: gen-03
```
Step 2b RECREATE `create-folder /my-files/_cas-probe/c17b gen-03` - 2026-08-04T14:18:52.5077993-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~r83Wqy01urJprPp8deKMlg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-03' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:18:57.686Z,
  modificationTime: 2026-08-04T21:18:57.686Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3a `info /my-files/_cas-probe/c17b/gen-03` (fresh process) - 2026-08-04T14:18:57.8710967-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~r83Wqy01urJprPp8deKMlg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-03' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:18:57.000Z,
  modificationTime: 2026-08-04T21:18:57.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3b `list /my-files/_cas-probe/c17b --json` - 2026-08-04T14:19:01.5838586-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:57:39.000Z","modificationTime":"2026-08-04T20:57:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:58:29.000Z","modificationTime":"2026-08-04T20:58:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:08.000Z","modificationTime":"2026-08-04T20:59:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:39.000Z","modificationTime":"2026-08-04T20:59:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~bGWhebPsVOLM0E43JuNSEg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:15.000Z","modificationTime":"2026-08-04T21:00:15.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~k2ltsJ374d3nDXuylx1LHg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:51.000Z","modificationTime":"2026-08-04T21:00:51.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~v_M-faPpHavHoZAgQYT16A","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:01:29.000Z","modificationTime":"2026-08-04T21:01:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~FJ3e7KVTX6L-KR8eRoYaCw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:01.000Z","modificationTime":"2026-08-04T21:02:01.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~1jszxKEh7pipkm11xfv7oQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:33.000Z","modificationTime":"2026-08-04T21:02:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~wKOeh9MS6iGmXEiVci9H1w","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:02.000Z","modificationTime":"2026-08-04T21:03:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~XU3VjNpul9sR9HcQR60UkA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-11"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:37.000Z","modificationTime":"2026-08-04T21:03:37.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zo8FDPjVnwgfwvZhmcOHJQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:04:43.000Z","modificationTime":"2026-08-04T21:04:43.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~so1Xnd4CFVTmnSkGYJc1QQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:05:59.000Z","modificationTime":"2026-08-04T21:05:59.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~gnyUjXCeWGdaTaD9bGOjNg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:07:03.000Z","modificationTime":"2026-08-04T21:07:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~eht2Lam-dW-anbYO-KBi0g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:08:02.000Z","modificationTime":"2026-08-04T21:08:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zsKfPMvKbCckIDqyzJnsLw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:09:02.000Z","modificationTime":"2026-08-04T21:09:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~IZ3yph7JUU1us1o11tj1Eg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:10:00.000Z","modificationTime":"2026-08-04T21:10:00.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~hiP8Mx7BiCFQsr_c-n6Yaw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:11:03.000Z","modificationTime":"2026-08-04T21:11:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zp0bn7M_uJWhzRN6wMiVNQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:12:05.000Z","modificationTime":"2026-08-04T21:12:05.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~72iZp0eSHsi5_GlXpukoQw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:13:09.000Z","modificationTime":"2026-08-04T21:13:09.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~rAjSTiPpATCEhhUBujaEJg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:14:08.000Z","modificationTime":"2026-08-04T21:14:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Fh1ZdDLBW1zUvD-jBenl4g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:16:14.000Z","modificationTime":"2026-08-04T21:16:14.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Tn5s58NB8uM2yZLf7rt0rw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:17:33.000Z","modificationTime":"2026-08-04T21:17:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~r83Wqy01urJprPp8deKMlg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:18:57.000Z","modificationTime":"2026-08-04T21:18:57.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 3b row count for name `gen-03`: 1 ; trashTime occurrences in listing: 0
Step 3c `create-folder /my-files/_cas-probe/c17b/gen-03 child` - 2026-08-04T14:19:05.0749990-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~n842EYQvihskrWNuQ6XunA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~r83Wqy01urJprPp8deKMlg',
  name: { ok: true, value: 'child' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:19:09.907Z,
  modificationTime: 2026-08-04T21:19:09.907Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3d `create-folder /my-files/_cas-probe/c17b/gen-03/child sub` - 2026-08-04T14:19:10.0871239-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~iJbv_8v813tWhEkr86xxDg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~n842EYQvihskrWNuQ6XunA',
  name: { ok: true, value: 'sub' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:19:14.123Z,
  modificationTime: 2026-08-04T21:19:14.123Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3e `info /my-files/_cas-probe/c17b/gen-03/child/sub` (fresh process) - 2026-08-04T14:19:14.3236608-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~iJbv_8v813tWhEkr86xxDg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~n842EYQvihskrWNuQ6XunA',
  name: { ok: true, value: 'sub' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:19:13.000Z,
  modificationTime: 2026-08-04T21:19:13.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

**Trial gen-03 classification: CLEAN** (cycle creates 0/0/0; pre-recreate infos 1/1, final pre-info 1; recreate 0; info 0; rows 1; child 0; sub 0; info sub 0)

#### Trial gen-04 (Variant 3 - Generations / three trashed homonyms)

Step 1.1-a `create-folder /my-files/_cas-probe/c17b gen-04` (generation 1) - 2026-08-04T14:19:24.6168720-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~S0cB-zJnvOGXciI7ZjMghQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-04' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:19:27.131Z,
  modificationTime: 2026-08-04T21:19:27.131Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.1-b `list /my-files/_cas-probe/c17b/gen-04 --json` (trash gate) - 2026-08-04T14:19:27.3352185-07:00 - exit 0
```
[

]
```
Step 1.1-c `trash /my-files/_cas-probe/c17b/gen-04` - 2026-08-04T14:19:32.3063200-07:00 - exit 0
```
✅ gen-04
```
Step 1.2-pre `info /my-files/_cas-probe/c17b/gen-04` (fresh process, before recreate; expect absent) - 2026-08-04T14:19:36.2174247-07:00 - exit 1
```
Node not found: gen-04
```
Step 1.2-a `create-folder /my-files/_cas-probe/c17b gen-04` (generation 2) - 2026-08-04T14:19:38.8947607-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~g8crr0UOSW5E0OPrws0SpA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-04' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:19:42.317Z,
  modificationTime: 2026-08-04T21:19:42.317Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.2-b `list /my-files/_cas-probe/c17b/gen-04 --json` (trash gate) - 2026-08-04T14:19:42.5034780-07:00 - exit 0
```
[

]
```
Step 1.2-c `trash /my-files/_cas-probe/c17b/gen-04` - 2026-08-04T14:19:46.7384347-07:00 - exit 0
```
✅ gen-04
```
Step 1.3-pre `info /my-files/_cas-probe/c17b/gen-04` (fresh process, before recreate; expect absent) - 2026-08-04T14:19:51.8588295-07:00 - exit 1
```
Node not found: gen-04
```
Step 1.3-a `create-folder /my-files/_cas-probe/c17b gen-04` (generation 3) - 2026-08-04T14:19:54.2893745-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~eTdE3BG8Zp7A5xAj_nf_1w',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-04' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:19:58.188Z,
  modificationTime: 2026-08-04T21:19:58.188Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.3-b `list /my-files/_cas-probe/c17b/gen-04 --json` (trash gate) - 2026-08-04T14:19:58.3535805-07:00 - exit 0
```
[

]
```
Step 1.3-c `trash /my-files/_cas-probe/c17b/gen-04` - 2026-08-04T14:20:02.3303386-07:00 - exit 0
```
✅ gen-04
```
Step 2a `info /my-files/_cas-probe/c17b/gen-04` (fresh process, 3 trashed generations present; expect absent) - 2026-08-04T14:20:04.9567158-07:00 - exit 1
```
Node not found: gen-04
```
Step 2b RECREATE `create-folder /my-files/_cas-probe/c17b gen-04` - 2026-08-04T14:20:08.8603861-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~Yrixgx1Cgm9QwQIYGf4Cwg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-04' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:20:12.846Z,
  modificationTime: 2026-08-04T21:20:12.846Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3a `info /my-files/_cas-probe/c17b/gen-04` (fresh process) - 2026-08-04T14:20:13.0465876-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~Yrixgx1Cgm9QwQIYGf4Cwg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-04' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:20:12.000Z,
  modificationTime: 2026-08-04T21:20:12.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3b `list /my-files/_cas-probe/c17b --json` - 2026-08-04T14:20:15.6439773-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:57:39.000Z","modificationTime":"2026-08-04T20:57:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:58:29.000Z","modificationTime":"2026-08-04T20:58:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:08.000Z","modificationTime":"2026-08-04T20:59:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:39.000Z","modificationTime":"2026-08-04T20:59:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~bGWhebPsVOLM0E43JuNSEg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:15.000Z","modificationTime":"2026-08-04T21:00:15.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~k2ltsJ374d3nDXuylx1LHg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:51.000Z","modificationTime":"2026-08-04T21:00:51.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~v_M-faPpHavHoZAgQYT16A","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:01:29.000Z","modificationTime":"2026-08-04T21:01:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~FJ3e7KVTX6L-KR8eRoYaCw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:01.000Z","modificationTime":"2026-08-04T21:02:01.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~1jszxKEh7pipkm11xfv7oQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:33.000Z","modificationTime":"2026-08-04T21:02:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~wKOeh9MS6iGmXEiVci9H1w","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:02.000Z","modificationTime":"2026-08-04T21:03:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~XU3VjNpul9sR9HcQR60UkA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-11"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:37.000Z","modificationTime":"2026-08-04T21:03:37.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zo8FDPjVnwgfwvZhmcOHJQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:04:43.000Z","modificationTime":"2026-08-04T21:04:43.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~so1Xnd4CFVTmnSkGYJc1QQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:05:59.000Z","modificationTime":"2026-08-04T21:05:59.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~gnyUjXCeWGdaTaD9bGOjNg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:07:03.000Z","modificationTime":"2026-08-04T21:07:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~eht2Lam-dW-anbYO-KBi0g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:08:02.000Z","modificationTime":"2026-08-04T21:08:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zsKfPMvKbCckIDqyzJnsLw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:09:02.000Z","modificationTime":"2026-08-04T21:09:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~IZ3yph7JUU1us1o11tj1Eg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:10:00.000Z","modificationTime":"2026-08-04T21:10:00.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~hiP8Mx7BiCFQsr_c-n6Yaw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:11:03.000Z","modificationTime":"2026-08-04T21:11:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zp0bn7M_uJWhzRN6wMiVNQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:12:05.000Z","modificationTime":"2026-08-04T21:12:05.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~72iZp0eSHsi5_GlXpukoQw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:13:09.000Z","modificationTime":"2026-08-04T21:13:09.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~rAjSTiPpATCEhhUBujaEJg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:14:08.000Z","modificationTime":"2026-08-04T21:14:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Fh1ZdDLBW1zUvD-jBenl4g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:16:14.000Z","modificationTime":"2026-08-04T21:16:14.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Tn5s58NB8uM2yZLf7rt0rw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:17:33.000Z","modificationTime":"2026-08-04T21:17:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~r83Wqy01urJprPp8deKMlg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:18:57.000Z","modificationTime":"2026-08-04T21:18:57.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Yrixgx1Cgm9QwQIYGf4Cwg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:20:12.000Z","modificationTime":"2026-08-04T21:20:12.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 3b row count for name `gen-04`: 1 ; trashTime occurrences in listing: 0
Step 3c `create-folder /my-files/_cas-probe/c17b/gen-04 child` - 2026-08-04T14:20:19.2019383-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~INNrX9MUVo9hRxm_fvINCg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~Yrixgx1Cgm9QwQIYGf4Cwg',
  name: { ok: true, value: 'child' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:20:21.910Z,
  modificationTime: 2026-08-04T21:20:21.910Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3d `create-folder /my-files/_cas-probe/c17b/gen-04/child sub` - 2026-08-04T14:20:22.0885237-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~nA8RVK5RT1c-BMmYmAIwbw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~INNrX9MUVo9hRxm_fvINCg',
  name: { ok: true, value: 'sub' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:20:26.615Z,
  modificationTime: 2026-08-04T21:20:26.615Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3e `info /my-files/_cas-probe/c17b/gen-04/child/sub` (fresh process) - 2026-08-04T14:20:26.8028118-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~nA8RVK5RT1c-BMmYmAIwbw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~INNrX9MUVo9hRxm_fvINCg',
  name: { ok: true, value: 'sub' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:20:26.000Z,
  modificationTime: 2026-08-04T21:20:26.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

**Trial gen-04 classification: CLEAN** (cycle creates 0/0/0; pre-recreate infos 1/1, final pre-info 1; recreate 0; info 0; rows 1; child 0; sub 0; info sub 0)

#### Trial gen-05 (Variant 3 - Generations / three trashed homonyms)

Step 1.1-a `create-folder /my-files/_cas-probe/c17b gen-05` (generation 1) - 2026-08-04T14:20:37.9706697-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~QoGxH8NZQo3KgWkdAabpEg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-05' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:20:42.320Z,
  modificationTime: 2026-08-04T21:20:42.320Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.1-b `list /my-files/_cas-probe/c17b/gen-05 --json` (trash gate) - 2026-08-04T14:20:42.5282033-07:00 - exit 0
```
[

]
```
Step 1.1-c `trash /my-files/_cas-probe/c17b/gen-05` - 2026-08-04T14:20:45.3472650-07:00 - exit 0
```
✅ gen-05
```
Step 1.2-pre `info /my-files/_cas-probe/c17b/gen-05` (fresh process, before recreate; expect absent) - 2026-08-04T14:20:47.9810986-07:00 - exit 1
```
Node not found: gen-05
```
Step 1.2-a `create-folder /my-files/_cas-probe/c17b gen-05` (generation 2) - 2026-08-04T14:20:52.1065115-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~4BKB-XLp7nN4TB2-r51JDQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-05' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:20:56.462Z,
  modificationTime: 2026-08-04T21:20:56.462Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.2-b `list /my-files/_cas-probe/c17b/gen-05 --json` (trash gate) - 2026-08-04T14:20:56.6628381-07:00 - exit 0
```
[

]
```
Step 1.2-c `trash /my-files/_cas-probe/c17b/gen-05` - 2026-08-04T14:20:59.8559628-07:00 - exit 0
```
✅ gen-05
```
Step 1.3-pre `info /my-files/_cas-probe/c17b/gen-05` (fresh process, before recreate; expect absent) - 2026-08-04T14:21:02.8453978-07:00 - exit 1
```
Node not found: gen-05
```
Step 1.3-a `create-folder /my-files/_cas-probe/c17b gen-05` (generation 3) - 2026-08-04T14:21:07.4848230-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~faEO3vS_-7sgtqn27dikOg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-05' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:21:11.914Z,
  modificationTime: 2026-08-04T21:21:11.914Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.3-b `list /my-files/_cas-probe/c17b/gen-05 --json` (trash gate) - 2026-08-04T14:21:12.1046559-07:00 - exit 0
```
[

]
```
Step 1.3-c `trash /my-files/_cas-probe/c17b/gen-05` - 2026-08-04T14:21:15.7413194-07:00 - exit 0
```
✅ gen-05
```
Step 2a `info /my-files/_cas-probe/c17b/gen-05` (fresh process, 3 trashed generations present; expect absent) - 2026-08-04T14:21:19.6780009-07:00 - exit 1
```
Node not found: gen-05
```
Step 2b RECREATE `create-folder /my-files/_cas-probe/c17b gen-05` - 2026-08-04T14:21:22.3237261-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~6Ba5GiPvCL7c5hyGLYmknA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-05' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:21:26.117Z,
  modificationTime: 2026-08-04T21:21:26.117Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3a `info /my-files/_cas-probe/c17b/gen-05` (fresh process) - 2026-08-04T14:21:26.3462716-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~6Ba5GiPvCL7c5hyGLYmknA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-05' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:21:25.000Z,
  modificationTime: 2026-08-04T21:21:25.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3b `list /my-files/_cas-probe/c17b --json` - 2026-08-04T14:21:31.7068370-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:57:39.000Z","modificationTime":"2026-08-04T20:57:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:58:29.000Z","modificationTime":"2026-08-04T20:58:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:08.000Z","modificationTime":"2026-08-04T20:59:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:39.000Z","modificationTime":"2026-08-04T20:59:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~bGWhebPsVOLM0E43JuNSEg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:15.000Z","modificationTime":"2026-08-04T21:00:15.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~k2ltsJ374d3nDXuylx1LHg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:51.000Z","modificationTime":"2026-08-04T21:00:51.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~v_M-faPpHavHoZAgQYT16A","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:01:29.000Z","modificationTime":"2026-08-04T21:01:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~FJ3e7KVTX6L-KR8eRoYaCw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:01.000Z","modificationTime":"2026-08-04T21:02:01.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~1jszxKEh7pipkm11xfv7oQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:33.000Z","modificationTime":"2026-08-04T21:02:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~wKOeh9MS6iGmXEiVci9H1w","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:02.000Z","modificationTime":"2026-08-04T21:03:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~XU3VjNpul9sR9HcQR60UkA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-11"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:37.000Z","modificationTime":"2026-08-04T21:03:37.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zo8FDPjVnwgfwvZhmcOHJQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:04:43.000Z","modificationTime":"2026-08-04T21:04:43.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~so1Xnd4CFVTmnSkGYJc1QQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:05:59.000Z","modificationTime":"2026-08-04T21:05:59.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~gnyUjXCeWGdaTaD9bGOjNg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:07:03.000Z","modificationTime":"2026-08-04T21:07:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~eht2Lam-dW-anbYO-KBi0g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:08:02.000Z","modificationTime":"2026-08-04T21:08:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zsKfPMvKbCckIDqyzJnsLw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:09:02.000Z","modificationTime":"2026-08-04T21:09:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~IZ3yph7JUU1us1o11tj1Eg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:10:00.000Z","modificationTime":"2026-08-04T21:10:00.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~hiP8Mx7BiCFQsr_c-n6Yaw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:11:03.000Z","modificationTime":"2026-08-04T21:11:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zp0bn7M_uJWhzRN6wMiVNQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:12:05.000Z","modificationTime":"2026-08-04T21:12:05.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~72iZp0eSHsi5_GlXpukoQw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:13:09.000Z","modificationTime":"2026-08-04T21:13:09.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~rAjSTiPpATCEhhUBujaEJg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:14:08.000Z","modificationTime":"2026-08-04T21:14:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Fh1ZdDLBW1zUvD-jBenl4g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:16:14.000Z","modificationTime":"2026-08-04T21:16:14.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Tn5s58NB8uM2yZLf7rt0rw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:17:33.000Z","modificationTime":"2026-08-04T21:17:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~r83Wqy01urJprPp8deKMlg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:18:57.000Z","modificationTime":"2026-08-04T21:18:57.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Yrixgx1Cgm9QwQIYGf4Cwg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:20:12.000Z","modificationTime":"2026-08-04T21:20:12.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6Ba5GiPvCL7c5hyGLYmknA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:21:25.000Z","modificationTime":"2026-08-04T21:21:25.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 3b row count for name `gen-05`: 1 ; trashTime occurrences in listing: 0
Step 3c `create-folder /my-files/_cas-probe/c17b/gen-05 child` - 2026-08-04T14:21:35.3792811-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~BnnwpIZcrY75SGnTVLRU0g',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~6Ba5GiPvCL7c5hyGLYmknA',
  name: { ok: true, value: 'child' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:21:38.042Z,
  modificationTime: 2026-08-04T21:21:38.042Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3d `create-folder /my-files/_cas-probe/c17b/gen-05/child sub` - 2026-08-04T14:21:38.2275534-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~PZRZptTGq980Igt0Jna75w',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~BnnwpIZcrY75SGnTVLRU0g',
  name: { ok: true, value: 'sub' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:21:44.406Z,
  modificationTime: 2026-08-04T21:21:44.406Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3e `info /my-files/_cas-probe/c17b/gen-05/child/sub` (fresh process) - 2026-08-04T14:21:44.5899762-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~PZRZptTGq980Igt0Jna75w',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~BnnwpIZcrY75SGnTVLRU0g',
  name: { ok: true, value: 'sub' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:21:43.000Z,
  modificationTime: 2026-08-04T21:21:43.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

**Trial gen-05 classification: CLEAN** (cycle creates 0/0/0; pre-recreate infos 1/1, final pre-info 1; recreate 0; info 0; rows 1; child 0; sub 0; info sub 0)

#### Trial gen-06 (Variant 3 - Generations / three trashed homonyms)

Step 1.1-a `create-folder /my-files/_cas-probe/c17b gen-06` (generation 1) - 2026-08-04T14:21:55.4439378-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~A22r_Un_JRvS6_opXLz7dg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-06' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:21:57.858Z,
  modificationTime: 2026-08-04T21:21:57.858Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.1-b `list /my-files/_cas-probe/c17b/gen-06 --json` (trash gate) - 2026-08-04T14:21:58.0796200-07:00 - exit 0
```
[

]
```
Step 1.1-c `trash /my-files/_cas-probe/c17b/gen-06` - 2026-08-04T14:22:02.2031758-07:00 - exit 0
```
✅ gen-06
```
Step 1.2-pre `info /my-files/_cas-probe/c17b/gen-06` (fresh process, before recreate; expect absent) - 2026-08-04T14:22:04.9600157-07:00 - exit 1
```
Node not found: gen-06
```
Step 1.2-a `create-folder /my-files/_cas-probe/c17b gen-06` (generation 2) - 2026-08-04T14:22:07.8828288-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~kuj-Vv23KfWZbDnrLhaTmA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-06' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:22:12.268Z,
  modificationTime: 2026-08-04T21:22:12.268Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.2-b `list /my-files/_cas-probe/c17b/gen-06 --json` (trash gate) - 2026-08-04T14:22:12.4660931-07:00 - exit 0
```
[

]
```
Step 1.2-c `trash /my-files/_cas-probe/c17b/gen-06` - 2026-08-04T14:22:15.5530071-07:00 - exit 0
```
✅ gen-06
```
Step 1.3-pre `info /my-files/_cas-probe/c17b/gen-06` (fresh process, before recreate; expect absent) - 2026-08-04T14:22:18.4177317-07:00 - exit 1
```
Node not found: gen-06
```
Step 1.3-a `create-folder /my-files/_cas-probe/c17b gen-06` (generation 3) - 2026-08-04T14:22:21.3980059-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~vV_aqeSVTaw05Xrdofo70Q',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-06' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:22:24.635Z,
  modificationTime: 2026-08-04T21:22:24.635Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.3-b `list /my-files/_cas-probe/c17b/gen-06 --json` (trash gate) - 2026-08-04T14:22:24.8229109-07:00 - exit 0
```
[

]
```
Step 1.3-c `trash /my-files/_cas-probe/c17b/gen-06` - 2026-08-04T14:22:27.8360729-07:00 - exit 0
```
✅ gen-06
```
Step 2a `info /my-files/_cas-probe/c17b/gen-06` (fresh process, 3 trashed generations present; expect absent) - 2026-08-04T14:22:31.1902378-07:00 - exit 1
```
Node not found: gen-06
```
Step 2b RECREATE `create-folder /my-files/_cas-probe/c17b gen-06` - 2026-08-04T14:22:33.9915588-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~XMYWGvVn7H7Zu-xE7d1ORg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-06' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:22:36.403Z,
  modificationTime: 2026-08-04T21:22:36.403Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3a `info /my-files/_cas-probe/c17b/gen-06` (fresh process) - 2026-08-04T14:22:36.5972727-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~XMYWGvVn7H7Zu-xE7d1ORg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-06' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:22:35.000Z,
  modificationTime: 2026-08-04T21:22:35.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3b `list /my-files/_cas-probe/c17b --json` - 2026-08-04T14:22:41.9977971-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:57:39.000Z","modificationTime":"2026-08-04T20:57:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:58:29.000Z","modificationTime":"2026-08-04T20:58:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:08.000Z","modificationTime":"2026-08-04T20:59:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:39.000Z","modificationTime":"2026-08-04T20:59:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~bGWhebPsVOLM0E43JuNSEg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:15.000Z","modificationTime":"2026-08-04T21:00:15.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~k2ltsJ374d3nDXuylx1LHg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:51.000Z","modificationTime":"2026-08-04T21:00:51.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~v_M-faPpHavHoZAgQYT16A","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:01:29.000Z","modificationTime":"2026-08-04T21:01:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~FJ3e7KVTX6L-KR8eRoYaCw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:01.000Z","modificationTime":"2026-08-04T21:02:01.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~1jszxKEh7pipkm11xfv7oQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:33.000Z","modificationTime":"2026-08-04T21:02:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~wKOeh9MS6iGmXEiVci9H1w","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:02.000Z","modificationTime":"2026-08-04T21:03:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~XU3VjNpul9sR9HcQR60UkA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-11"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:37.000Z","modificationTime":"2026-08-04T21:03:37.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zo8FDPjVnwgfwvZhmcOHJQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:04:43.000Z","modificationTime":"2026-08-04T21:04:43.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~so1Xnd4CFVTmnSkGYJc1QQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:05:59.000Z","modificationTime":"2026-08-04T21:05:59.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~gnyUjXCeWGdaTaD9bGOjNg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:07:03.000Z","modificationTime":"2026-08-04T21:07:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~eht2Lam-dW-anbYO-KBi0g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:08:02.000Z","modificationTime":"2026-08-04T21:08:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zsKfPMvKbCckIDqyzJnsLw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:09:02.000Z","modificationTime":"2026-08-04T21:09:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~IZ3yph7JUU1us1o11tj1Eg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:10:00.000Z","modificationTime":"2026-08-04T21:10:00.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~hiP8Mx7BiCFQsr_c-n6Yaw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:11:03.000Z","modificationTime":"2026-08-04T21:11:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zp0bn7M_uJWhzRN6wMiVNQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:12:05.000Z","modificationTime":"2026-08-04T21:12:05.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~72iZp0eSHsi5_GlXpukoQw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:13:09.000Z","modificationTime":"2026-08-04T21:13:09.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~rAjSTiPpATCEhhUBujaEJg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:14:08.000Z","modificationTime":"2026-08-04T21:14:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Fh1ZdDLBW1zUvD-jBenl4g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:16:14.000Z","modificationTime":"2026-08-04T21:16:14.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Tn5s58NB8uM2yZLf7rt0rw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:17:33.000Z","modificationTime":"2026-08-04T21:17:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~r83Wqy01urJprPp8deKMlg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:18:57.000Z","modificationTime":"2026-08-04T21:18:57.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Yrixgx1Cgm9QwQIYGf4Cwg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:20:12.000Z","modificationTime":"2026-08-04T21:20:12.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6Ba5GiPvCL7c5hyGLYmknA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:21:25.000Z","modificationTime":"2026-08-04T21:21:25.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~XMYWGvVn7H7Zu-xE7d1ORg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:22:35.000Z","modificationTime":"2026-08-04T21:22:35.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 3b row count for name `gen-06`: 1 ; trashTime occurrences in listing: 0
Step 3c `create-folder /my-files/_cas-probe/c17b/gen-06 child` - 2026-08-04T14:22:46.3391561-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~FZ7Kc8g1FRi08R5BLVrDuA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~XMYWGvVn7H7Zu-xE7d1ORg',
  name: { ok: true, value: 'child' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:22:49.246Z,
  modificationTime: 2026-08-04T21:22:49.246Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3d `create-folder /my-files/_cas-probe/c17b/gen-06/child sub` - 2026-08-04T14:22:49.4328072-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~5CsNlQySyBN_5Wn6gsqYMg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~FZ7Kc8g1FRi08R5BLVrDuA',
  name: { ok: true, value: 'sub' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:22:53.646Z,
  modificationTime: 2026-08-04T21:22:53.646Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3e `info /my-files/_cas-probe/c17b/gen-06/child/sub` (fresh process) - 2026-08-04T14:22:53.8370904-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~5CsNlQySyBN_5Wn6gsqYMg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~FZ7Kc8g1FRi08R5BLVrDuA',
  name: { ok: true, value: 'sub' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:22:53.000Z,
  modificationTime: 2026-08-04T21:22:53.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

**Trial gen-06 classification: CLEAN** (cycle creates 0/0/0; pre-recreate infos 1/1, final pre-info 1; recreate 0; info 0; rows 1; child 0; sub 0; info sub 0)

#### Trial gen-07 (Variant 3 - Generations / three trashed homonyms)

Step 1.1-a `create-folder /my-files/_cas-probe/c17b gen-07` (generation 1) - 2026-08-04T14:23:04.6657088-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~UR9BfFKhMhWpX9Rk2qxnCw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-07' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:23:08.611Z,
  modificationTime: 2026-08-04T21:23:08.611Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.1-b `list /my-files/_cas-probe/c17b/gen-07 --json` (trash gate) - 2026-08-04T14:23:08.8169731-07:00 - exit 0
```
[

]
```
Step 1.1-c `trash /my-files/_cas-probe/c17b/gen-07` - 2026-08-04T14:23:12.9878335-07:00 - exit 0
```
✅ gen-07
```
Step 1.2-pre `info /my-files/_cas-probe/c17b/gen-07` (fresh process, before recreate; expect absent) - 2026-08-04T14:23:16.0177470-07:00 - exit 1
```
Node not found: gen-07
```
Step 1.2-a `create-folder /my-files/_cas-probe/c17b gen-07` (generation 2) - 2026-08-04T14:23:19.2962980-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~oIB_BnEcOxhR7Vis85fMoQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-07' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:23:23.991Z,
  modificationTime: 2026-08-04T21:23:23.991Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.2-b `list /my-files/_cas-probe/c17b/gen-07 --json` (trash gate) - 2026-08-04T14:23:24.1737619-07:00 - exit 0
```
[

]
```
Step 1.2-c `trash /my-files/_cas-probe/c17b/gen-07` - 2026-08-04T14:23:27.3980951-07:00 - exit 0
```
✅ gen-07
```
Step 1.3-pre `info /my-files/_cas-probe/c17b/gen-07` (fresh process, before recreate; expect absent) - 2026-08-04T14:23:30.3928368-07:00 - exit 1
```
Node not found: gen-07
```
Step 1.3-a `create-folder /my-files/_cas-probe/c17b gen-07` (generation 3) - 2026-08-04T14:23:33.9466950-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~QunoTXs914c4FYHa2IRoiA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-07' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:23:38.702Z,
  modificationTime: 2026-08-04T21:23:38.702Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.3-b `list /my-files/_cas-probe/c17b/gen-07 --json` (trash gate) - 2026-08-04T14:23:38.9120801-07:00 - exit 0
```
[

]
```
Step 1.3-c `trash /my-files/_cas-probe/c17b/gen-07` - 2026-08-04T14:23:44.4229017-07:00 - exit 0
```
✅ gen-07
```
Step 2a `info /my-files/_cas-probe/c17b/gen-07` (fresh process, 3 trashed generations present; expect absent) - 2026-08-04T14:23:47.7654537-07:00 - exit 1
```
Node not found: gen-07
```
Step 2b RECREATE `create-folder /my-files/_cas-probe/c17b gen-07` - 2026-08-04T14:23:51.5534178-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~rOG9BHYlbZNWG0KIW7JcDQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-07' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:23:55.290Z,
  modificationTime: 2026-08-04T21:23:55.290Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3a `info /my-files/_cas-probe/c17b/gen-07` (fresh process) - 2026-08-04T14:23:55.4905745-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~rOG9BHYlbZNWG0KIW7JcDQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-07' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:23:54.000Z,
  modificationTime: 2026-08-04T21:23:54.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3b `list /my-files/_cas-probe/c17b --json` - 2026-08-04T14:24:00.6147153-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:57:39.000Z","modificationTime":"2026-08-04T20:57:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:58:29.000Z","modificationTime":"2026-08-04T20:58:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:08.000Z","modificationTime":"2026-08-04T20:59:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:39.000Z","modificationTime":"2026-08-04T20:59:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~bGWhebPsVOLM0E43JuNSEg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:15.000Z","modificationTime":"2026-08-04T21:00:15.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~k2ltsJ374d3nDXuylx1LHg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:51.000Z","modificationTime":"2026-08-04T21:00:51.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~v_M-faPpHavHoZAgQYT16A","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:01:29.000Z","modificationTime":"2026-08-04T21:01:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~FJ3e7KVTX6L-KR8eRoYaCw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:01.000Z","modificationTime":"2026-08-04T21:02:01.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~1jszxKEh7pipkm11xfv7oQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:33.000Z","modificationTime":"2026-08-04T21:02:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~wKOeh9MS6iGmXEiVci9H1w","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:02.000Z","modificationTime":"2026-08-04T21:03:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~XU3VjNpul9sR9HcQR60UkA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-11"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:37.000Z","modificationTime":"2026-08-04T21:03:37.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zo8FDPjVnwgfwvZhmcOHJQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:04:43.000Z","modificationTime":"2026-08-04T21:04:43.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~so1Xnd4CFVTmnSkGYJc1QQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:05:59.000Z","modificationTime":"2026-08-04T21:05:59.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~gnyUjXCeWGdaTaD9bGOjNg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:07:03.000Z","modificationTime":"2026-08-04T21:07:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~eht2Lam-dW-anbYO-KBi0g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:08:02.000Z","modificationTime":"2026-08-04T21:08:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zsKfPMvKbCckIDqyzJnsLw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:09:02.000Z","modificationTime":"2026-08-04T21:09:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~IZ3yph7JUU1us1o11tj1Eg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:10:00.000Z","modificationTime":"2026-08-04T21:10:00.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~hiP8Mx7BiCFQsr_c-n6Yaw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:11:03.000Z","modificationTime":"2026-08-04T21:11:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zp0bn7M_uJWhzRN6wMiVNQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:12:05.000Z","modificationTime":"2026-08-04T21:12:05.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~72iZp0eSHsi5_GlXpukoQw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:13:09.000Z","modificationTime":"2026-08-04T21:13:09.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~rAjSTiPpATCEhhUBujaEJg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:14:08.000Z","modificationTime":"2026-08-04T21:14:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Fh1ZdDLBW1zUvD-jBenl4g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:16:14.000Z","modificationTime":"2026-08-04T21:16:14.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Tn5s58NB8uM2yZLf7rt0rw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:17:33.000Z","modificationTime":"2026-08-04T21:17:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~r83Wqy01urJprPp8deKMlg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:18:57.000Z","modificationTime":"2026-08-04T21:18:57.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Yrixgx1Cgm9QwQIYGf4Cwg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:20:12.000Z","modificationTime":"2026-08-04T21:20:12.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6Ba5GiPvCL7c5hyGLYmknA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:21:25.000Z","modificationTime":"2026-08-04T21:21:25.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~XMYWGvVn7H7Zu-xE7d1ORg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:22:35.000Z","modificationTime":"2026-08-04T21:22:35.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~rOG9BHYlbZNWG0KIW7JcDQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:23:54.000Z","modificationTime":"2026-08-04T21:23:54.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 3b row count for name `gen-07`: 1 ; trashTime occurrences in listing: 0
Step 3c `create-folder /my-files/_cas-probe/c17b/gen-07 child` - 2026-08-04T14:24:03.8370711-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~J3VVm2uTtn9jT959eg8_5Q',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~rOG9BHYlbZNWG0KIW7JcDQ',
  name: { ok: true, value: 'child' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:24:06.468Z,
  modificationTime: 2026-08-04T21:24:06.468Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3d `create-folder /my-files/_cas-probe/c17b/gen-07/child sub` - 2026-08-04T14:24:06.6614197-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~gl6t1Xi--PsoRW_boK-K8A',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~J3VVm2uTtn9jT959eg8_5Q',
  name: { ok: true, value: 'sub' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:24:11.299Z,
  modificationTime: 2026-08-04T21:24:11.299Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3e `info /my-files/_cas-probe/c17b/gen-07/child/sub` (fresh process) - 2026-08-04T14:24:11.4790856-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~gl6t1Xi--PsoRW_boK-K8A',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~J3VVm2uTtn9jT959eg8_5Q',
  name: { ok: true, value: 'sub' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:24:10.000Z,
  modificationTime: 2026-08-04T21:24:10.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

**Trial gen-07 classification: CLEAN** (cycle creates 0/0/0; pre-recreate infos 1/1, final pre-info 1; recreate 0; info 0; rows 1; child 0; sub 0; info sub 0)

#### Trial gen-08 (Variant 3 - Generations / three trashed homonyms)

Step 1.1-a `create-folder /my-files/_cas-probe/c17b gen-08` (generation 1) - 2026-08-04T14:24:21.5847352-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~F_7rmfG_OCCq_U7m0NK2GQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-08' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:24:24.128Z,
  modificationTime: 2026-08-04T21:24:24.128Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.1-b `list /my-files/_cas-probe/c17b/gen-08 --json` (trash gate) - 2026-08-04T14:24:24.3338130-07:00 - exit 0
```
[

]
```
Step 1.1-c `trash /my-files/_cas-probe/c17b/gen-08` - 2026-08-04T14:24:27.5477064-07:00 - exit 0
```
✅ gen-08
```
Step 1.2-pre `info /my-files/_cas-probe/c17b/gen-08` (fresh process, before recreate; expect absent) - 2026-08-04T14:24:31.4826900-07:00 - exit 1
```
Node not found: gen-08
```
Step 1.2-a `create-folder /my-files/_cas-probe/c17b gen-08` (generation 2) - 2026-08-04T14:24:35.1667318-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~xLetay505lEW6KnhqOhxjg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-08' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:24:38.911Z,
  modificationTime: 2026-08-04T21:24:38.911Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.2-b `list /my-files/_cas-probe/c17b/gen-08 --json` (trash gate) - 2026-08-04T14:24:39.0904262-07:00 - exit 0
```
[

]
```
Step 1.2-c `trash /my-files/_cas-probe/c17b/gen-08` - 2026-08-04T14:24:42.9965984-07:00 - exit 0
```
✅ gen-08
```
Step 1.3-pre `info /my-files/_cas-probe/c17b/gen-08` (fresh process, before recreate; expect absent) - 2026-08-04T14:24:48.3236423-07:00 - exit 1
```
Node not found: gen-08
```
Step 1.3-a `create-folder /my-files/_cas-probe/c17b gen-08` (generation 3) - 2026-08-04T14:24:52.0022599-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~6fhZW5EvsuL6KVfteWLjmQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-08' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:24:57.288Z,
  modificationTime: 2026-08-04T21:24:57.288Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.3-b `list /my-files/_cas-probe/c17b/gen-08 --json` (trash gate) - 2026-08-04T14:24:57.4860724-07:00 - exit 0
```
[

]
```
Step 1.3-c `trash /my-files/_cas-probe/c17b/gen-08` - 2026-08-04T14:25:00.3446550-07:00 - exit 0
```
✅ gen-08
```
Step 2a `info /my-files/_cas-probe/c17b/gen-08` (fresh process, 3 trashed generations present; expect absent) - 2026-08-04T14:25:02.9562482-07:00 - exit 1
```
Node not found: gen-08
```
Step 2b RECREATE `create-folder /my-files/_cas-probe/c17b gen-08` - 2026-08-04T14:25:05.8973064-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~YuJg_83yIxaQES8GY10umg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-08' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:25:09.800Z,
  modificationTime: 2026-08-04T21:25:09.800Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3a `info /my-files/_cas-probe/c17b/gen-08` (fresh process) - 2026-08-04T14:25:10.0100929-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~YuJg_83yIxaQES8GY10umg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-08' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:25:09.000Z,
  modificationTime: 2026-08-04T21:25:09.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3b `list /my-files/_cas-probe/c17b --json` - 2026-08-04T14:25:14.1512233-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:57:39.000Z","modificationTime":"2026-08-04T20:57:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:58:29.000Z","modificationTime":"2026-08-04T20:58:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:08.000Z","modificationTime":"2026-08-04T20:59:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:39.000Z","modificationTime":"2026-08-04T20:59:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~bGWhebPsVOLM0E43JuNSEg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:15.000Z","modificationTime":"2026-08-04T21:00:15.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~k2ltsJ374d3nDXuylx1LHg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:51.000Z","modificationTime":"2026-08-04T21:00:51.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~v_M-faPpHavHoZAgQYT16A","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:01:29.000Z","modificationTime":"2026-08-04T21:01:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~FJ3e7KVTX6L-KR8eRoYaCw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:01.000Z","modificationTime":"2026-08-04T21:02:01.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~1jszxKEh7pipkm11xfv7oQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:33.000Z","modificationTime":"2026-08-04T21:02:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~wKOeh9MS6iGmXEiVci9H1w","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:02.000Z","modificationTime":"2026-08-04T21:03:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~XU3VjNpul9sR9HcQR60UkA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-11"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:37.000Z","modificationTime":"2026-08-04T21:03:37.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zo8FDPjVnwgfwvZhmcOHJQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:04:43.000Z","modificationTime":"2026-08-04T21:04:43.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~so1Xnd4CFVTmnSkGYJc1QQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:05:59.000Z","modificationTime":"2026-08-04T21:05:59.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~gnyUjXCeWGdaTaD9bGOjNg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:07:03.000Z","modificationTime":"2026-08-04T21:07:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~eht2Lam-dW-anbYO-KBi0g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:08:02.000Z","modificationTime":"2026-08-04T21:08:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zsKfPMvKbCckIDqyzJnsLw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:09:02.000Z","modificationTime":"2026-08-04T21:09:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~IZ3yph7JUU1us1o11tj1Eg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:10:00.000Z","modificationTime":"2026-08-04T21:10:00.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~hiP8Mx7BiCFQsr_c-n6Yaw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:11:03.000Z","modificationTime":"2026-08-04T21:11:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zp0bn7M_uJWhzRN6wMiVNQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:12:05.000Z","modificationTime":"2026-08-04T21:12:05.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~72iZp0eSHsi5_GlXpukoQw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:13:09.000Z","modificationTime":"2026-08-04T21:13:09.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~rAjSTiPpATCEhhUBujaEJg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:14:08.000Z","modificationTime":"2026-08-04T21:14:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Fh1ZdDLBW1zUvD-jBenl4g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:16:14.000Z","modificationTime":"2026-08-04T21:16:14.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Tn5s58NB8uM2yZLf7rt0rw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:17:33.000Z","modificationTime":"2026-08-04T21:17:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~r83Wqy01urJprPp8deKMlg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:18:57.000Z","modificationTime":"2026-08-04T21:18:57.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Yrixgx1Cgm9QwQIYGf4Cwg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:20:12.000Z","modificationTime":"2026-08-04T21:20:12.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6Ba5GiPvCL7c5hyGLYmknA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:21:25.000Z","modificationTime":"2026-08-04T21:21:25.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~XMYWGvVn7H7Zu-xE7d1ORg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:22:35.000Z","modificationTime":"2026-08-04T21:22:35.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~rOG9BHYlbZNWG0KIW7JcDQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:23:54.000Z","modificationTime":"2026-08-04T21:23:54.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~YuJg_83yIxaQES8GY10umg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:25:09.000Z","modificationTime":"2026-08-04T21:25:09.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 3b row count for name `gen-08`: 1 ; trashTime occurrences in listing: 0
Step 3c `create-folder /my-files/_cas-probe/c17b/gen-08 child` - 2026-08-04T14:25:17.6261351-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~WxwB_fWGPUVK688wxp8eCQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~YuJg_83yIxaQES8GY10umg',
  name: { ok: true, value: 'child' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:25:22.918Z,
  modificationTime: 2026-08-04T21:25:22.918Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3d `create-folder /my-files/_cas-probe/c17b/gen-08/child sub` - 2026-08-04T14:25:23.0916267-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~E5RNEvM8Cgr3zI63aF6clw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~WxwB_fWGPUVK688wxp8eCQ',
  name: { ok: true, value: 'sub' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:25:26.362Z,
  modificationTime: 2026-08-04T21:25:26.362Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3e `info /my-files/_cas-probe/c17b/gen-08/child/sub` (fresh process) - 2026-08-04T14:25:26.5456351-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~E5RNEvM8Cgr3zI63aF6clw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~WxwB_fWGPUVK688wxp8eCQ',
  name: { ok: true, value: 'sub' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:25:25.000Z,
  modificationTime: 2026-08-04T21:25:25.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

**Trial gen-08 classification: CLEAN** (cycle creates 0/0/0; pre-recreate infos 1/1, final pre-info 1; recreate 0; info 0; rows 1; child 0; sub 0; info sub 0)

#### Trial gen-09 (Variant 3 - Generations / three trashed homonyms)

Step 1.1-a `create-folder /my-files/_cas-probe/c17b gen-09` (generation 1) - 2026-08-04T14:25:36.4566939-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~knXYSKt_JvhJprq_LuRlLw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-09' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:25:40.280Z,
  modificationTime: 2026-08-04T21:25:40.280Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.1-b `list /my-files/_cas-probe/c17b/gen-09 --json` (trash gate) - 2026-08-04T14:25:40.4710584-07:00 - exit 0
```
[

]
```
Step 1.1-c `trash /my-files/_cas-probe/c17b/gen-09` - 2026-08-04T14:25:43.6110092-07:00 - exit 0
```
✅ gen-09
```
Step 1.2-pre `info /my-files/_cas-probe/c17b/gen-09` (fresh process, before recreate; expect absent) - 2026-08-04T14:25:47.6467370-07:00 - exit 1
```
Node not found: gen-09
```
Step 1.2-a `create-folder /my-files/_cas-probe/c17b gen-09` (generation 2) - 2026-08-04T14:25:50.2914588-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~jHYH6p81QgVdaBmj1-aafg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-09' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:25:54.741Z,
  modificationTime: 2026-08-04T21:25:54.741Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.2-b `list /my-files/_cas-probe/c17b/gen-09 --json` (trash gate) - 2026-08-04T14:25:54.9662308-07:00 - exit 0
```
[

]
```
Step 1.2-c `trash /my-files/_cas-probe/c17b/gen-09` - 2026-08-04T14:25:59.1689581-07:00 - exit 0
```
✅ gen-09
```
Step 1.3-pre `info /my-files/_cas-probe/c17b/gen-09` (fresh process, before recreate; expect absent) - 2026-08-04T14:26:02.5924067-07:00 - exit 1
```
Node not found: gen-09
```
Step 1.3-a `create-folder /my-files/_cas-probe/c17b gen-09` (generation 3) - 2026-08-04T14:26:05.9769582-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~WAiiOtErbej-xByMi3hCJg',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-09' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:26:08.436Z,
  modificationTime: 2026-08-04T21:26:08.436Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.3-b `list /my-files/_cas-probe/c17b/gen-09 --json` (trash gate) - 2026-08-04T14:26:08.6267399-07:00 - exit 0
```
[

]
```
Step 1.3-c `trash /my-files/_cas-probe/c17b/gen-09` - 2026-08-04T14:26:11.8400837-07:00 - exit 0
```
✅ gen-09
```
Step 2a `info /my-files/_cas-probe/c17b/gen-09` (fresh process, 3 trashed generations present; expect absent) - 2026-08-04T14:26:14.4424431-07:00 - exit 1
```
Request timed out: GET https://drive-api.proton.me/core/v4/addresses?Page=0&PageSize=50
```
Step 2b RECREATE `create-folder /my-files/_cas-probe/c17b gen-09` - 2026-08-04T14:31:25.4162471-07:00 - exit 1
```
===============================================
error: Was there a typo in the url or port?
  path: "https://drive-api.proton.me/drive/v2/shares/my-files",
 errno: 0,
  code: "FailedToOpenSocket"


      at dY1 (src/cli/run.ts:77:13)
      at F20 (src/cli/run.ts:25:9)
      at async L20 (src/cli/index.ts:29:15)
Error details:
{
  code: 'FailedToOpenSocket',
  path: 'https://drive-api.proton.me/drive/v2/shares/my-files',
  errno: 0
}
```
Step 3a `info /my-files/_cas-probe/c17b/gen-09` (fresh process) - 2026-08-04T14:31:29.1311997-07:00 - exit 1
```
Node not found: gen-09
```
Step 3b `list /my-files/_cas-probe/c17b --json` - 2026-08-04T14:39:24.8299942-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~1jszxKEh7pipkm11xfv7oQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:33.000Z","modificationTime":"2026-08-04T21:02:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6Ba5GiPvCL7c5hyGLYmknA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:21:25.000Z","modificationTime":"2026-08-04T21:21:25.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:39.000Z","modificationTime":"2026-08-04T20:59:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~72iZp0eSHsi5_GlXpukoQw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:13:09.000Z","modificationTime":"2026-08-04T21:13:09.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~FJ3e7KVTX6L-KR8eRoYaCw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:01.000Z","modificationTime":"2026-08-04T21:02:01.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Fh1ZdDLBW1zUvD-jBenl4g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:16:14.000Z","modificationTime":"2026-08-04T21:16:14.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:08.000Z","modificationTime":"2026-08-04T20:59:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~IZ3yph7JUU1us1o11tj1Eg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:10:00.000Z","modificationTime":"2026-08-04T21:10:00.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:58:29.000Z","modificationTime":"2026-08-04T20:58:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:57:39.000Z","modificationTime":"2026-08-04T20:57:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Tn5s58NB8uM2yZLf7rt0rw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:17:33.000Z","modificationTime":"2026-08-04T21:17:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~XMYWGvVn7H7Zu-xE7d1ORg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:22:35.000Z","modificationTime":"2026-08-04T21:22:35.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~XU3VjNpul9sR9HcQR60UkA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-11"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:37.000Z","modificationTime":"2026-08-04T21:03:37.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Yrixgx1Cgm9QwQIYGf4Cwg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:20:12.000Z","modificationTime":"2026-08-04T21:20:12.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~YuJg_83yIxaQES8GY10umg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:25:09.000Z","modificationTime":"2026-08-04T21:25:09.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~bGWhebPsVOLM0E43JuNSEg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:15.000Z","modificationTime":"2026-08-04T21:00:15.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~eht2Lam-dW-anbYO-KBi0g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:08:02.000Z","modificationTime":"2026-08-04T21:08:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~gnyUjXCeWGdaTaD9bGOjNg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:07:03.000Z","modificationTime":"2026-08-04T21:07:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~hiP8Mx7BiCFQsr_c-n6Yaw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:11:03.000Z","modificationTime":"2026-08-04T21:11:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~k2ltsJ374d3nDXuylx1LHg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:51.000Z","modificationTime":"2026-08-04T21:00:51.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~r83Wqy01urJprPp8deKMlg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:18:57.000Z","modificationTime":"2026-08-04T21:18:57.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~rAjSTiPpATCEhhUBujaEJg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:14:08.000Z","modificationTime":"2026-08-04T21:14:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~rOG9BHYlbZNWG0KIW7JcDQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:23:54.000Z","modificationTime":"2026-08-04T21:23:54.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~so1Xnd4CFVTmnSkGYJc1QQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:05:59.000Z","modificationTime":"2026-08-04T21:05:59.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~v_M-faPpHavHoZAgQYT16A","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:01:29.000Z","modificationTime":"2026-08-04T21:01:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~wKOeh9MS6iGmXEiVci9H1w","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:02.000Z","modificationTime":"2026-08-04T21:03:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zo8FDPjVnwgfwvZhmcOHJQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:04:43.000Z","modificationTime":"2026-08-04T21:04:43.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zp0bn7M_uJWhzRN6wMiVNQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:12:05.000Z","modificationTime":"2026-08-04T21:12:05.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zsKfPMvKbCckIDqyzJnsLw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:09:02.000Z","modificationTime":"2026-08-04T21:09:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 3b row count for name `gen-09`: 0 ; trashTime occurrences in listing: 0
Step 3c `create-folder /my-files/_cas-probe/c17b/gen-09 child` - 2026-08-04T14:39:29.3751841-07:00 - exit 1
```
Node not found: gen-09
```
Step 3d `create-folder /my-files/_cas-probe/c17b/gen-09/child sub` - 2026-08-04T14:39:33.2107513-07:00 - exit 1
```
Node not found: gen-09
```
Step 3e `info /my-files/_cas-probe/c17b/gen-09/child/sub` (fresh process) - 2026-08-04T14:39:36.5139890-07:00 - exit 1
```
Node not found: gen-09
```

**Trial gen-09 classification: REPRODUCTION** [AUTO-CLASSIFIER OUTPUT - WRONG. OVERRIDDEN TO **VOID / OTHER (network outage)** - see the CORRECTION block immediately below. The exit-1s were `Request timed out` and `FailedToOpenSocket`, NOT `Node not found` + `already exists`. This is NOT a reproduction.] (cycle creates 0/0/0; pre-recreate infos 1/1, final pre-info 1; recreate 1; info 1; rows 0; child 1; sub 1; info sub 1)

**CORRECTION - trial gen-09 auto-classification is WRONG and is hereby overridden to OTHER (VOID: transient network outage).**

The runner script classifies from exit codes alone, so it emitted REPRODUCTION. Reading the verbatim output shows the exit-1s were NOT the gate signature:

- Step 2a `info` failed with `Request timed out: GET https://drive-api.proton.me/core/v4/addresses?Page=0&PageSize=50` - a NETWORK TIMEOUT, not `Node not found`.
- Step 2b RECREATE failed with `FailedToOpenSocket` against `https://drive-api.proton.me/drive/v2/shares/my-files` - a NETWORK failure, not `A file or folder with that name already exists`.
- The outage spans 14:26:14 to ~14:31:25 local (~5 minutes), which also explains this trial exceeding its 500 s wall clock.
- Steps 3a/3c/3d then correctly report `Node not found: gen-09`, and Step 3b lists ZERO rows named gen-09. That is CONSISTENT, not split-brain: all three generations were trashed and the recreate never landed because the network was down.

gen-09 is therefore VOID (no probe signal), NOT a reproduction. Per rule 4 the failed create-folder was NOT retried. Residual state: the name `gen-09` under c17b has 3 trashed generations and NO active node.

Connectivity re-check `info /my-files/_cas-probe/c17b` - 2026-08-04T14:40:39.4451512-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~L_5KcYiEE3tt59SOGUMbjg',
  name: { ok: true, value: 'c17b' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T20:56:48.000Z,
  modificationTime: 2026-08-04T20:56:48.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false, claimedModificationTime: undefined },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

#### Trial gen-10 (Variant 3 - Generations / three trashed homonyms)

Step 1.1-a `create-folder /my-files/_cas-probe/c17b gen-10` (generation 1) - 2026-08-04T14:40:52.7310387-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~46bNb3MVYO1whqD4CamB1Q',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-10' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:40:55.274Z,
  modificationTime: 2026-08-04T21:40:55.274Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.1-b `list /my-files/_cas-probe/c17b/gen-10 --json` (trash gate) - 2026-08-04T14:40:55.4949237-07:00 - exit 0
```
[

]
```
Step 1.1-c `trash /my-files/_cas-probe/c17b/gen-10` - 2026-08-04T14:40:59.1759403-07:00 - exit 0
```
✅ gen-10
```
Step 1.2-pre `info /my-files/_cas-probe/c17b/gen-10` (fresh process, before recreate; expect absent) - 2026-08-04T14:41:03.3351575-07:00 - exit 1
```
Node not found: gen-10
```
Step 1.2-a `create-folder /my-files/_cas-probe/c17b gen-10` (generation 2) - 2026-08-04T14:41:07.0386737-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~2z2lRjcmVlzwS-k8hJ-lpA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-10' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:41:10.534Z,
  modificationTime: 2026-08-04T21:41:10.534Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.2-b `list /my-files/_cas-probe/c17b/gen-10 --json` (trash gate) - 2026-08-04T14:41:10.7576147-07:00 - exit 0
```
[

]
```
Step 1.2-c `trash /my-files/_cas-probe/c17b/gen-10` - 2026-08-04T14:41:14.4662359-07:00 - exit 0
```
✅ gen-10
```
Step 1.3-pre `info /my-files/_cas-probe/c17b/gen-10` (fresh process, before recreate; expect absent) - 2026-08-04T14:41:18.0222850-07:00 - exit 1
```
Node not found: gen-10
```
Step 1.3-a `create-folder /my-files/_cas-probe/c17b gen-10` (generation 3) - 2026-08-04T14:41:20.9030072-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~hS_2xGlcNz_YDVVF61aUNA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-10' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:41:26.007Z,
  modificationTime: 2026-08-04T21:41:26.007Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.3-b `list /my-files/_cas-probe/c17b/gen-10 --json` (trash gate) - 2026-08-04T14:41:26.2505378-07:00 - exit 0
```
[

]
```
Step 1.3-c `trash /my-files/_cas-probe/c17b/gen-10` - 2026-08-04T14:41:30.7340045-07:00 - exit 0
```
✅ gen-10
```
Step 2a `info /my-files/_cas-probe/c17b/gen-10` (fresh process, 3 trashed generations present; expect absent) - 2026-08-04T14:41:34.6615712-07:00 - exit 1
```
Node not found: gen-10
```
Step 2b RECREATE `create-folder /my-files/_cas-probe/c17b gen-10` - 2026-08-04T14:41:37.2391225-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~SR5yKXGTV72_VD5E9FsX0Q',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-10' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:41:41.293Z,
  modificationTime: 2026-08-04T21:41:41.293Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3a `info /my-files/_cas-probe/c17b/gen-10` (fresh process) - 2026-08-04T14:41:41.4953689-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~SR5yKXGTV72_VD5E9FsX0Q',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-10' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:41:40.000Z,
  modificationTime: 2026-08-04T21:41:40.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3b `list /my-files/_cas-probe/c17b --json` - 2026-08-04T14:41:44.9738430-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:57:39.000Z","modificationTime":"2026-08-04T20:57:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:58:29.000Z","modificationTime":"2026-08-04T20:58:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:08.000Z","modificationTime":"2026-08-04T20:59:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:39.000Z","modificationTime":"2026-08-04T20:59:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~bGWhebPsVOLM0E43JuNSEg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:15.000Z","modificationTime":"2026-08-04T21:00:15.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~k2ltsJ374d3nDXuylx1LHg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:51.000Z","modificationTime":"2026-08-04T21:00:51.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~v_M-faPpHavHoZAgQYT16A","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:01:29.000Z","modificationTime":"2026-08-04T21:01:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~FJ3e7KVTX6L-KR8eRoYaCw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:01.000Z","modificationTime":"2026-08-04T21:02:01.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~1jszxKEh7pipkm11xfv7oQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:33.000Z","modificationTime":"2026-08-04T21:02:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~wKOeh9MS6iGmXEiVci9H1w","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:02.000Z","modificationTime":"2026-08-04T21:03:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~XU3VjNpul9sR9HcQR60UkA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-11"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:37.000Z","modificationTime":"2026-08-04T21:03:37.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zo8FDPjVnwgfwvZhmcOHJQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:04:43.000Z","modificationTime":"2026-08-04T21:04:43.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~so1Xnd4CFVTmnSkGYJc1QQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:05:59.000Z","modificationTime":"2026-08-04T21:05:59.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~gnyUjXCeWGdaTaD9bGOjNg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:07:03.000Z","modificationTime":"2026-08-04T21:07:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~eht2Lam-dW-anbYO-KBi0g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:08:02.000Z","modificationTime":"2026-08-04T21:08:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zsKfPMvKbCckIDqyzJnsLw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:09:02.000Z","modificationTime":"2026-08-04T21:09:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~IZ3yph7JUU1us1o11tj1Eg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:10:00.000Z","modificationTime":"2026-08-04T21:10:00.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~hiP8Mx7BiCFQsr_c-n6Yaw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:11:03.000Z","modificationTime":"2026-08-04T21:11:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zp0bn7M_uJWhzRN6wMiVNQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:12:05.000Z","modificationTime":"2026-08-04T21:12:05.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~72iZp0eSHsi5_GlXpukoQw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:13:09.000Z","modificationTime":"2026-08-04T21:13:09.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~rAjSTiPpATCEhhUBujaEJg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:14:08.000Z","modificationTime":"2026-08-04T21:14:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Fh1ZdDLBW1zUvD-jBenl4g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:16:14.000Z","modificationTime":"2026-08-04T21:16:14.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Tn5s58NB8uM2yZLf7rt0rw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:17:33.000Z","modificationTime":"2026-08-04T21:17:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~r83Wqy01urJprPp8deKMlg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:18:57.000Z","modificationTime":"2026-08-04T21:18:57.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Yrixgx1Cgm9QwQIYGf4Cwg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:20:12.000Z","modificationTime":"2026-08-04T21:20:12.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6Ba5GiPvCL7c5hyGLYmknA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:21:25.000Z","modificationTime":"2026-08-04T21:21:25.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~XMYWGvVn7H7Zu-xE7d1ORg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:22:35.000Z","modificationTime":"2026-08-04T21:22:35.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~rOG9BHYlbZNWG0KIW7JcDQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:23:54.000Z","modificationTime":"2026-08-04T21:23:54.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~YuJg_83yIxaQES8GY10umg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:25:09.000Z","modificationTime":"2026-08-04T21:25:09.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~SR5yKXGTV72_VD5E9FsX0Q","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:41:40.000Z","modificationTime":"2026-08-04T21:41:40.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 3b row count for name `gen-10`: 1 ; trashTime occurrences in listing: 0
Step 3c `create-folder /my-files/_cas-probe/c17b/gen-10 child` - 2026-08-04T14:41:47.6403340-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~qBG4i4Mz1bTRw1imivp_2w',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~SR5yKXGTV72_VD5E9FsX0Q',
  name: { ok: true, value: 'child' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:41:50.467Z,
  modificationTime: 2026-08-04T21:41:50.467Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3d `create-folder /my-files/_cas-probe/c17b/gen-10/child sub` - 2026-08-04T14:41:50.6443196-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~Q_oTkxNl9cGYYTd6lBmvag',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~qBG4i4Mz1bTRw1imivp_2w',
  name: { ok: true, value: 'sub' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:41:55.150Z,
  modificationTime: 2026-08-04T21:41:55.150Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3e `info /my-files/_cas-probe/c17b/gen-10/child/sub` (fresh process) - 2026-08-04T14:41:55.3680036-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~Q_oTkxNl9cGYYTd6lBmvag',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~qBG4i4Mz1bTRw1imivp_2w',
  name: { ok: true, value: 'sub' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:41:54.000Z,
  modificationTime: 2026-08-04T21:41:54.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

**Trial gen-10 classification: CLEAN** (cycle creates 0/0/0; pre-recreate infos 1/1, final pre-info 1; recreate 0; info 0; rows 1; child 0; sub 0; info sub 0)

#### Trial gen-11 (Variant 3 - Generations / three trashed homonyms)

Step 1.1-a `create-folder /my-files/_cas-probe/c17b gen-11` (generation 1) - 2026-08-04T14:42:06.8615784-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~vekv1ObR2h5tA5hy1zZ5QQ',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-11' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:42:09.634Z,
  modificationTime: 2026-08-04T21:42:09.634Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.1-b `list /my-files/_cas-probe/c17b/gen-11 --json` (trash gate) - 2026-08-04T14:42:09.8548047-07:00 - exit 0
```
[

]
```
Step 1.1-c `trash /my-files/_cas-probe/c17b/gen-11` - 2026-08-04T14:42:12.7984193-07:00 - exit 0
```
✅ gen-11
```
Step 1.2-pre `info /my-files/_cas-probe/c17b/gen-11` (fresh process, before recreate; expect absent) - 2026-08-04T14:42:15.9372499-07:00 - exit 1
```
Node not found: gen-11
```
Step 1.2-a `create-folder /my-files/_cas-probe/c17b gen-11` (generation 2) - 2026-08-04T14:42:18.5302942-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~joSK6hxFKtyUaZaEcEJ4hA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-11' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:42:21.600Z,
  modificationTime: 2026-08-04T21:42:21.600Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.2-b `list /my-files/_cas-probe/c17b/gen-11 --json` (trash gate) - 2026-08-04T14:42:21.8705969-07:00 - exit 0
```
[

]
```
Step 1.2-c `trash /my-files/_cas-probe/c17b/gen-11` - 2026-08-04T14:42:26.4761852-07:00 - exit 0
```
✅ gen-11
```
Step 1.3-pre `info /my-files/_cas-probe/c17b/gen-11` (fresh process, before recreate; expect absent) - 2026-08-04T14:42:29.3139760-07:00 - exit 1
```
Node not found: gen-11
```
Step 1.3-a `create-folder /my-files/_cas-probe/c17b gen-11` (generation 3) - 2026-08-04T14:42:32.7458228-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~b-b-JmROtq1i2NyudJGSKA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-11' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:42:37.519Z,
  modificationTime: 2026-08-04T21:42:37.519Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 1.3-b `list /my-files/_cas-probe/c17b/gen-11 --json` (trash gate) - 2026-08-04T14:42:37.7146658-07:00 - exit 0
```
[

]
```
Step 1.3-c `trash /my-files/_cas-probe/c17b/gen-11` - 2026-08-04T14:42:41.9418021-07:00 - exit 0
```
✅ gen-11
```
Step 2a `info /my-files/_cas-probe/c17b/gen-11` (fresh process, 3 trashed generations present; expect absent) - 2026-08-04T14:42:44.5165487-07:00 - exit 1
```
Node not found: gen-11
```
Step 2b RECREATE `create-folder /my-files/_cas-probe/c17b gen-11` - 2026-08-04T14:42:47.5976011-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~_AASZgdazRghP4YR5DLL3Q',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-11' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:42:51.743Z,
  modificationTime: 2026-08-04T21:42:51.743Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3a `info /my-files/_cas-probe/c17b/gen-11` (fresh process) - 2026-08-04T14:42:51.9332443-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~_AASZgdazRghP4YR5DLL3Q',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A',
  name: { ok: true, value: 'gen-11' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:42:51.000Z,
  modificationTime: 2026-08-04T21:42:51.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3b `list /my-files/_cas-probe/c17b --json` - 2026-08-04T14:42:54.6987400-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:57:39.000Z","modificationTime":"2026-08-04T20:57:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:58:29.000Z","modificationTime":"2026-08-04T20:58:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:08.000Z","modificationTime":"2026-08-04T20:59:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:39.000Z","modificationTime":"2026-08-04T20:59:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~bGWhebPsVOLM0E43JuNSEg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:15.000Z","modificationTime":"2026-08-04T21:00:15.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~k2ltsJ374d3nDXuylx1LHg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:51.000Z","modificationTime":"2026-08-04T21:00:51.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~v_M-faPpHavHoZAgQYT16A","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:01:29.000Z","modificationTime":"2026-08-04T21:01:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~FJ3e7KVTX6L-KR8eRoYaCw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:01.000Z","modificationTime":"2026-08-04T21:02:01.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~1jszxKEh7pipkm11xfv7oQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:33.000Z","modificationTime":"2026-08-04T21:02:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~wKOeh9MS6iGmXEiVci9H1w","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:02.000Z","modificationTime":"2026-08-04T21:03:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~XU3VjNpul9sR9HcQR60UkA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-11"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:37.000Z","modificationTime":"2026-08-04T21:03:37.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zo8FDPjVnwgfwvZhmcOHJQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:04:43.000Z","modificationTime":"2026-08-04T21:04:43.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~so1Xnd4CFVTmnSkGYJc1QQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:05:59.000Z","modificationTime":"2026-08-04T21:05:59.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~gnyUjXCeWGdaTaD9bGOjNg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:07:03.000Z","modificationTime":"2026-08-04T21:07:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~eht2Lam-dW-anbYO-KBi0g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:08:02.000Z","modificationTime":"2026-08-04T21:08:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zsKfPMvKbCckIDqyzJnsLw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:09:02.000Z","modificationTime":"2026-08-04T21:09:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~IZ3yph7JUU1us1o11tj1Eg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:10:00.000Z","modificationTime":"2026-08-04T21:10:00.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~hiP8Mx7BiCFQsr_c-n6Yaw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:11:03.000Z","modificationTime":"2026-08-04T21:11:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zp0bn7M_uJWhzRN6wMiVNQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:12:05.000Z","modificationTime":"2026-08-04T21:12:05.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~72iZp0eSHsi5_GlXpukoQw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:13:09.000Z","modificationTime":"2026-08-04T21:13:09.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~rAjSTiPpATCEhhUBujaEJg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:14:08.000Z","modificationTime":"2026-08-04T21:14:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Fh1ZdDLBW1zUvD-jBenl4g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:16:14.000Z","modificationTime":"2026-08-04T21:16:14.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Tn5s58NB8uM2yZLf7rt0rw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:17:33.000Z","modificationTime":"2026-08-04T21:17:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~r83Wqy01urJprPp8deKMlg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:18:57.000Z","modificationTime":"2026-08-04T21:18:57.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Yrixgx1Cgm9QwQIYGf4Cwg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:20:12.000Z","modificationTime":"2026-08-04T21:20:12.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6Ba5GiPvCL7c5hyGLYmknA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:21:25.000Z","modificationTime":"2026-08-04T21:21:25.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~XMYWGvVn7H7Zu-xE7d1ORg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:22:35.000Z","modificationTime":"2026-08-04T21:22:35.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~rOG9BHYlbZNWG0KIW7JcDQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:23:54.000Z","modificationTime":"2026-08-04T21:23:54.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~YuJg_83yIxaQES8GY10umg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:25:09.000Z","modificationTime":"2026-08-04T21:25:09.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~SR5yKXGTV72_VD5E9FsX0Q","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:41:40.000Z","modificationTime":"2026-08-04T21:41:40.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~_AASZgdazRghP4YR5DLL3Q","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-11"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:42:51.000Z","modificationTime":"2026-08-04T21:42:51.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Step 3b row count for name `gen-11`: 1 ; trashTime occurrences in listing: 0
Step 3c `create-folder /my-files/_cas-probe/c17b/gen-11 child` - 2026-08-04T14:42:58.7049425-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~Z-AQ0NhpJ_kr9Vp2bbylbw',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~_AASZgdazRghP4YR5DLL3Q',
  name: { ok: true, value: 'child' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:43:03.278Z,
  modificationTime: 2026-08-04T21:43:03.278Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3d `create-folder /my-files/_cas-probe/c17b/gen-11/child sub` - 2026-08-04T14:43:03.4605552-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~m80ulRCC8b_kdykFWnDaQA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~Z-AQ0NhpJ_kr9Vp2bbylbw',
  name: { ok: true, value: 'sub' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'inherited',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me' },
  type: 'folder',
  mediaType: 'Folder',
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:43:06.460Z,
  modificationTime: 2026-08-04T21:43:06.460Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```
Step 3e `info /my-files/_cas-probe/c17b/gen-11/child/sub` (fresh process) - 2026-08-04T14:43:06.6794764-07:00 - exit 0
```
{
  uid: 'tU-Ot1Sq63NwBcxlnl7IcA~m80ulRCC8b_kdykFWnDaQA',
  parentUid: 'tU-Ot1Sq63NwBcxlnl7IcA~Z-AQ0NhpJ_kr9Vp2bbylbw',
  name: { ok: true, value: 'sub' },
  keyAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  nameAuthor: { ok: true, value: 'hgp64ua8ny2o@pm.me' },
  directRole: 'admin',
  membership: undefined,
  ownedBy: { email: 'hgp64ua8ny2o@pm.me', organization: undefined },
  type: 'folder',
  mediaType: undefined,
  isShared: false,
  isSharedByUrl: false,
  creationTime: 2026-08-04T21:43:06.000Z,
  modificationTime: 2026-08-04T21:43:06.000Z,
  trashTime: undefined,
  totalStorageSize: undefined,
  activeRevision: undefined,
  folder: { isImported: false },
  deprecatedShareId: undefined,
  treeEventScopeId: 'tU-Ot1Sq63NwBcxlnl7IcA',
  errors: undefined
}
```

**Trial gen-11 classification: CLEAN** (cycle creates 0/0/0; pre-recreate infos 1/1, final pre-info 1; recreate 0; info 0; rows 1; child 0; sub 0; info sub 0)

**Variant 3 tally: 10 VALID trials (gen-01..gen-08, gen-10, gen-11), 10 CLEAN, 0 REPRODUCTION, 0 OTHER. Plus gen-09 VOID (network outage, no signal).**

---

## Step Z - Cleanup (verify-before-trashing)

Z1a `list /my-files/_cas-probe/c17b --json` (pre-trash verification) - 2026-08-04T14:43:28.2489396-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~1jszxKEh7pipkm11xfv7oQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:33.000Z","modificationTime":"2026-08-04T21:02:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6Ba5GiPvCL7c5hyGLYmknA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:21:25.000Z","modificationTime":"2026-08-04T21:21:25.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~6iamm8o4AL65wlMe84eTYg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:39.000Z","modificationTime":"2026-08-04T20:59:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~72iZp0eSHsi5_GlXpukoQw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-09"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:13:09.000Z","modificationTime":"2026-08-04T21:13:09.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~FJ3e7KVTX6L-KR8eRoYaCw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:02:01.000Z","modificationTime":"2026-08-04T21:02:01.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Fh1ZdDLBW1zUvD-jBenl4g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:16:14.000Z","modificationTime":"2026-08-04T21:16:14.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~HPWEfcY68KqvF58ttpCjTg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:59:08.000Z","modificationTime":"2026-08-04T20:59:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~IZ3yph7JUU1us1o11tj1Eg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:10:00.000Z","modificationTime":"2026-08-04T21:10:00.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Jc2_j4C5uARjqXvROD3thg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:58:29.000Z","modificationTime":"2026-08-04T20:58:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~QUg-ynOt40gbB2eqG2kCzw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:57:39.000Z","modificationTime":"2026-08-04T20:57:39.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~SR5yKXGTV72_VD5E9FsX0Q","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:41:40.000Z","modificationTime":"2026-08-04T21:41:40.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Tn5s58NB8uM2yZLf7rt0rw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:17:33.000Z","modificationTime":"2026-08-04T21:17:33.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~XMYWGvVn7H7Zu-xE7d1ORg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:22:35.000Z","modificationTime":"2026-08-04T21:22:35.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~XU3VjNpul9sR9HcQR60UkA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-11"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:37.000Z","modificationTime":"2026-08-04T21:03:37.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Yrixgx1Cgm9QwQIYGf4Cwg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:20:12.000Z","modificationTime":"2026-08-04T21:20:12.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~YuJg_83yIxaQES8GY10umg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:25:09.000Z","modificationTime":"2026-08-04T21:25:09.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~_AASZgdazRghP4YR5DLL3Q","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-11"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:42:51.000Z","modificationTime":"2026-08-04T21:42:51.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~bGWhebPsVOLM0E43JuNSEg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:15.000Z","modificationTime":"2026-08-04T21:00:15.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~eht2Lam-dW-anbYO-KBi0g","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-04"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:08:02.000Z","modificationTime":"2026-08-04T21:08:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~gnyUjXCeWGdaTaD9bGOjNg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:07:03.000Z","modificationTime":"2026-08-04T21:07:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~hiP8Mx7BiCFQsr_c-n6Yaw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:11:03.000Z","modificationTime":"2026-08-04T21:11:03.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~k2ltsJ374d3nDXuylx1LHg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-06"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:00:51.000Z","modificationTime":"2026-08-04T21:00:51.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~r83Wqy01urJprPp8deKMlg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-03"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:18:57.000Z","modificationTime":"2026-08-04T21:18:57.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~rAjSTiPpATCEhhUBujaEJg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:14:08.000Z","modificationTime":"2026-08-04T21:14:08.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~rOG9BHYlbZNWG0KIW7JcDQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"gen-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:23:54.000Z","modificationTime":"2026-08-04T21:23:54.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~so1Xnd4CFVTmnSkGYJc1QQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-02"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:05:59.000Z","modificationTime":"2026-08-04T21:05:59.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~v_M-faPpHavHoZAgQYT16A","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-07"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:01:29.000Z","modificationTime":"2026-08-04T21:01:29.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~wKOeh9MS6iGmXEiVci9H1w","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"core-10"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:03:02.000Z","modificationTime":"2026-08-04T21:03:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zo8FDPjVnwgfwvZhmcOHJQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-01"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:04:43.000Z","modificationTime":"2026-08-04T21:04:43.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zp0bn7M_uJWhzRN6wMiVNQ","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-08"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:12:05.000Z","modificationTime":"2026-08-04T21:12:05.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~zsKfPMvKbCckIDqyzJnsLw","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A","name":{"ok":true,"value":"depth-05"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T21:09:02.000Z","modificationTime":"2026-08-04T21:09:02.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```
Z1a verification: 31 children, ALL probe-created (core-01..core-11, depth-01..depth-10, gen-01..gen-08, gen-10, gen-11). No unexpected names. No gen-09 active node (expected). Gate PASSED - proceeding to trash.
Z1b `trash /my-files/_cas-probe/c17b` - 2026-08-04T14:43:43.6415621-07:00 - exit 0
```
✅ c17b
```
Z2a `list /my-files/_cas-probe --json` (pre-trash verification) - 2026-08-04T14:43:59.0967820-07:00 - exit 0
```
[

]
```
Z2a verification: EMPTY array - no active children remain. Gate PASSED - proceeding to trash.
Z2b `trash /my-files/_cas-probe` - 2026-08-04T14:44:13.1497752-07:00 - exit 0
```
✅ _cas-probe
```
Z3 FINAL `list /my-files --json` - 2026-08-04T14:44:23.9703468-07:00 - exit 0
```
[
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~5QY-B6t9Eui6VKsXvJPmuA","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g","name":{"ok":true,"value":"Project Repo Bundles"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"creationTime":"2026-06-22T16:25:42.000Z","modificationTime":"2026-06-22T16:25:42.000Z","treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~ThDi8B_92zkL76UXhjqFng","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g","name":{"ok":true,"value":"ChatGPT Export Text Backup"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"creationTime":"2026-05-18T02:36:23.000Z","modificationTime":"2026-05-18T02:36:23.000Z","treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~Vho9hzKVaqnBLf7UnwC64w","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g","name":{"ok":true,"value":"GitBackups"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"creationTime":"2026-07-21T00:29:59.000Z","modificationTime":"2026-07-21T00:29:59.000Z","treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~n94X-kyGohebE_FBtxA5Sg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g","name":{"ok":true,"value":"Sensitive Project Sources"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"creationTime":"2026-05-22T15:12:18.000Z","modificationTime":"2026-05-22T15:12:18.000Z","treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"},
{"uid":"tU-Ot1Sq63NwBcxlnl7IcA~L_5KcYiEE3tt59SOGUMbjg","parentUid":"tU-Ot1Sq63NwBcxlnl7IcA~iMF_ohkUGz7d0J77g_Lb4g","name":{"ok":true,"value":"_cas-probe"},"keyAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"nameAuthor":{"ok":true,"value":"hgp64ua8ny2o@pm.me"},"directRole":"admin","ownedBy":{"email":"hgp64ua8ny2o@pm.me"},"type":"folder","isShared":false,"isSharedByUrl":false,"creationTime":"2026-08-04T20:56:31.000Z","modificationTime":"2026-08-04T20:56:31.000Z","trashTime":"2026-08-04T21:44:14.000Z","folder":{"isImported":false},"treeEventScopeId":"tU-Ot1Sq63NwBcxlnl7IcA"}
]
```

**Z3 assertion:** the four standing folders (Project Repo Bundles, ChatGPT Export Text Backup, GitBackups, Sensitive Project Sources) are BYTE-IDENTICAL to S2 - same uids, same creationTime, same modificationTime, no trashTime. No surprise.

**Z3 incidental finding (trashed-row visibility in `list`):** the just-trashed `_cas-probe` row DOES appear in `list /my-files --json` carrying `trashTime: 2026-08-04T21:44:14.000Z`. At S2 (6 h after the gate-run-2 cleanup) the trashed `_cas-probe`/`GitRemotes` rows did NOT appear at all. So `list` appears to surface recently-trashed rows and stop surfacing them later. Repeat fresh-process observations below.

Z3-repeat-1 `list /my-files --json` - 2026-08-04T14:44:48.5346526-07:00 - exit 0 - rows total 4 - trashTime occurrences 0 - _cas-probe row present: NO
Z3-repeat-2 `list /my-files --json` - 2026-08-04T14:44:52.0172689-07:00 - exit 0 - rows total 4 - trashTime occurrences 0 - _cas-probe row present: NO

Repeat observations: both subsequent fresh-process listings returned 4 rows, 0 trashTime occurrences, no `_cas-probe` row. The trashed row is therefore visible in `list` only within a short window (~10 s) after the trash, then disappears. This is a cache-coherency window on the list path and it explains the run-1 vs run-2 inconsistency noted in the C17 background.

---

# DELIVERABLES

## 1. Per-variant tallies and verdicts

| Variant | Topology | Trials run | CLEAN | REPRODUCTION | OTHER |
|---|---|---|---|---|---|
| P0 / P1 (ambient) | trashed homonym of `_cas-probe` from gate run 2 present at top level | 2 | 2 | 0 | 0 |
| 1 - Core | trashed homonym is the NODE ITSELF (1 generation) | 10 (core-02..core-11) | 10 | 0 | 0 |
| 2 - Depth | trashed homonym is an ANCESTOR (the actual gate topology) | 10 (depth-01..depth-10) | 10 | 0 | 0 |
| 3 - Generations | THREE trashed homonyms of the same name | 10 valid (gen-01..gen-08, gen-10, gen-11) | 10 | 0 | 0 |
| **TOTAL** | | **30 valid trials + P0/P1** | **32** | **0** | **0** (+1 VOID) |

**P0/P1 verdict.** The headline pre-trial observation is CLEAN. A trashed `_cas-probe` homonym created by gate run 2's cleanup (uid ending `~yzxv1W0vPJuYL0Wu_dYXKA`) was sitting in the trash, un-purged, when `create-folder /my-files _cas-probe` ran. Fresh-process `info` said `Node not found: _cas-probe` and the create nonetheless succeeded exit 0 with a new uid `…~L_5KcYiEE3tt59SOGUMbjg`. A same-name trashed sibling under the same parent did not block creation and did not produce a split-brain.

**Variant 1 verdict.** Ten create -> trash -> recreate -> probe cycles, all CLEAN. In every trial the recreate succeeded, fresh-process `info` on the recreated node returned exit 0 (found), the parent listing showed exactly one row for the name with zero `trashTime` fields, and `create-folder <node> child` through the recreated node succeeded. A single trashed homonym of the node itself never desynchronised `info` from `create-folder`.

**Variant 2 verdict.** Ten trials reproducing the gate's own topology - trash an ancestor, recreate it, then build `repo/refs/{heads,tags}` through the recreated ancestor. All CLEAN. Every descendant create returned exit 0; fresh-process `info` on the deep nodes `…/repo/refs/tags` and `…/repo/refs/heads` returned exit 0 (found) in all ten trials; `list …/repo/refs` always showed both children. The deliberate second `create-folder …/repo/refs tags` returned the exact gate string `A file or folder with that name already exists` in all ten trials - but CORRECTLY, paired with an `info` that FOUND the node. The gate signature requires that error paired with an ABSENT `info`, and that pairing never occurred.

**Variant 3 verdict.** Ten valid trials with THREE trashed generations of the same name manufactured before the final recreate, all CLEAN. Notably each intermediate recreate (generations 2 and 3) also succeeded exit 0 while the immediately preceding fresh-process `info` correctly reported `Node not found` - i.e. absent-then-create-succeeds, the coherent outcome, thirty times over. After the final recreate, `info` found the node, the parent listing showed exactly ONE row for the name (the three trashed generations are invisible to `list`), and both `child` and `child/sub` created and resolved cleanly. Multiple trashed generations did not trigger the mechanism.

## 2. Overall verdict

**NOT REPRODUCED.**

30 valid trials across three topologies (node-itself homonym, ancestor homonym, three-generation homonym), plus the ambient P0/P1 pair executed against a genuinely un-purged trashed homonym, produced ZERO instances of the gate signature. The signature hunted was: fresh-process `info` reports ABSENT while `create-folder` for that same name reports ALREADY EXISTS. Across the run, `info` and `create-folder` agreed with each other in every single observation:

- `info` absent + `create-folder` succeeds - observed ~40 times (all recreates), coherent.
- `info` found + `create-folder` says already-exists - observed 10 times (Variant 2 step 6), coherent.
- `info` absent + `create-folder` says already-exists - **observed ZERO times.**

The trashed-homonym hypothesis for the Stage 3b gate run 1 BLOCK is **not supported** by direct provocation. Trashed nodes were shown to be fully invisible to `list` (0 `trashTime` occurrences in every in-trial parent listing) and non-blocking to `create-folder`. Note this is a negative result on the specific mechanism, not proof that the run-1 failure cannot recur by some other mechanism.

**One VOID trial (gen-09), no signal.** Its exit codes superficially resembled the signature, but the verbatim output shows a ~5-minute network outage (`Request timed out: GET https://drive-api.proton.me/core/v4/addresses…` then `FailedToOpenSocket`), not `Node not found` / `already exists`. Per rule 4 the failed write was NOT retried; two replacement trials (gen-10, gen-11) were run instead to reach ten valid Variant 3 trials.

## 3. Purge list for the user (the user empties trash - the probe never does)

### 3a. Created AND trashed BY THIS PROBE (safe to purge)

Everything below was created by this probe today and trashed by it. 55 trash operations, all exit 0, all confined to `/my-files/_cas-probe`. Purging the single top-level entry `/my-files/_cas-probe` covers the whole subtree; the intermediate generations are listed because each trash is its own trash entry.

| Path | uid (where the CLI reported one) | Trash time |
|---|---|---|
| `/my-files/_cas-probe` | `tU-Ot1Sq63NwBcxlnl7IcA~L_5KcYiEE3tt59SOGUMbjg` | 2026-08-04T21:44:14.000Z |
| `/my-files/_cas-probe/c17b` | `tU-Ot1Sq63NwBcxlnl7IcA~aHPaZy9lDj5hiAQmUhvQ0A` | 2026-08-04T21:43:xx Z |
| `/my-files/_cas-probe/c17b/core-01` | (never individually trashed; trashed as a descendant of c17b) | with c17b |
| `/my-files/_cas-probe/c17b/core-02` .. `core-11` | see per-trial step 1 entries above | 21:00-21:04 Z, 1 generation each |
| `/my-files/_cas-probe/c17b/depth-01` .. `depth-10` | see per-trial step 1 entries above | 21:04-21:15 Z, 1 generation each |
| `/my-files/_cas-probe/c17b/gen-01` .. `gen-08`, `gen-10`, `gen-11` | see per-trial step 1.N entries above | 21:16-21:43 Z, **3 generations each** |
| `/my-files/_cas-probe/c17b/gen-09` | 3 generations: `…~knXYSKt_JvhJprq_LuRlLw`, `…~jHYH6p81QgVdaBmj1-aafg`, `…~WAiiOtErbej-xByMi3hCJg` | 21:25-21:26 Z, 3 generations, NO active node |

Total probe-created trash entries: 55 (1 `_cas-probe` + 1 `c17b` + 10 core + 10 depth + 33 gen generations). `core-01` was created but never individually trashed - it went to trash as part of `c17b`.

### 3b. PRE-EXISTING trash - NOT created by this probe (the user's gate leavings)

Listed for completeness only. This probe did not create, touch, restore, or purge any of these.

| Path | uid | Origin |
|---|---|---|
| `/my-files/GitRemotes` (containing `stage3b-gate-r2`) | uid ending `~IkPH9Xzj_QCEvxF3MFAd8g` | gate run 2 cleanup, 2026-08-04 ~15:38 UTC |
| `/my-files/_cas-probe` (containing `contract`) | uid ending `~yzxv1W0vPJuYL0Wu_dYXKA` | gate run 2 cleanup, 2026-08-04 ~15:38 UTC |
| possible additional earlier probe leavings | not enumerable | the CLI has NO trash-listing capability (see S3) |

**Caveat:** the CLI exposes no way to enumerate trash, so 3b cannot be verified from the command line - it is transcribed from the task briefing. The user should confirm in the Proton Drive web UI before emptying.

## 4. Surprises and unclassifiable observations

No BLOCKED-class surprise occurred. The run completed all planned steps. Four things are worth recording:

1. **Transient network outage (gen-09), verbatim in section above.** ~5 minutes of connectivity loss to `drive-api.proton.me`. Classifiable as an environment fault, not Drive behavior; trial voided, write not retried.
2. **Runner-script gate bug on `core-01`.** The trash gate compared the listing to the literal `[]`, but an empty folder lists as `[` newline newline `]`, so the gate FAILED CLOSED and no trash was issued. Drive behaved normally. The gate was made whitespace-insensitive; Variant 1 trials were renamed `core-02..core-11`. Worth noting that the gate erred safe.
3. **Trashed rows are transiently visible in `list`.** Immediately after `trash /my-files/_cas-probe`, the final `list /my-files --json` included the `_cas-probe` row carrying `trashTime`. Two fresh-process listings seconds later did not. This is a real cache-coherency window on the list path, and it explains why run 1 saw trashed rows in `list` and run 2 did not - the difference was elapsed time since the trash, not trash contents.
4. **`list` never shows trashed nodes in steady state.** Every in-trial parent listing across all 30 trials showed 0 `trashTime` occurrences and exactly 1 row per live name, even with 3 trashed homonyms of that name present.

## 5. Confinement and integrity attestation

- Writes were confined to `/my-files/_cas-probe` and its descendants. 0 of 55 trash commands targeted anything outside that subtree.
- `GitBackups`, `Sensitive Project Sources`, `Project Repo Bundles`, `ChatGPT Export Text Backup` were named ONLY in read-only `filesystem list /my-files --json` output. Their Z3 rows are byte-identical to their S2 rows (same uids, creationTime, modificationTime, no trashTime).
- `/my-files/GitRemotes` was never touched, created, or named in any command.
- No `delete`, no `empty-trash`, no `restore` was ever invoked.
- No failed write was retried.
- No git writes; no repo file modified except this log.
