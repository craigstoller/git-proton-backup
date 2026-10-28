# vNEXT composition-refusal steps, for this release's Windows and Linux gate briefs (O5(d))

**Status: steps to paste, not a brief, and not run.** Section 6 of the macOS port spec
(`docs/superpowers/specs/2026-10-27-v2-macos-port-design.md`, "The one live macOS gate") says:
"The release's Windows and Linux gates also test the composition refusal on their own binaries, as
v0.10.0's tested the case refusal per platform." These are the steps those two briefs carry. The
briefs themselves are written at release time from the v0.10.0 briefs plus this file (plan
`docs/superpowers/plans/2026-10-27-v2-macos-port.md`, Task 15 and "Not tasks"). Written on
2026-10-27 on branch `worktree-macos-port`, and re-anchored on 2026-10-28; code read at
**`0faada8`**, the macOS brief's anchor. The code it cites is unchanged from `119227e` there
[measured 2026-10-28: among the code paths, `git diff 119227e 0faada8` changes only
`cmd/git-remote-proton/main.go` and its test, neither cited here].

**`vNEXT` is a placeholder** for the release that carries the macOS port, whose version Craig has
not named. When he names it, this file is renamed with the version and every `vNEXT` replaced, by
script. **The file is ASCII and holds no backslash-u sequence;** the one literal backslash it needs
is written `<BS>` (the escaped view, below), so an editor tool cannot mangle it.

**What the steps show, on each platform's own binary** (the draft Linux asset in WSL; the draft
Windows exe on the PC):

1. a create of `refs/heads/cafe` with its accent in **NFD** (`cafe` + U+0301, bytes
   `63 61 66 65 cc 81`) beside the same name in **NFC** (`caf` + U+00E9, bytes `63 61 66 c3 a9`),
   pushed by exact bytes, is refused;
2. the refusal names both spellings, raw and in Go's ASCII-only quoted form (`%+q`), and the
   relation, `Unicode normalisation`, and says the harm, `a Mac clone would mishandle the pair`;
3. no ref and no pack are written.

**Why off a Mac at all.** The create rule is on every platform: a pair pushed from Linux or Windows
is what a Mac clone would later mishandle (spec section 3; M13: git on Linux and Windows does not
normalise names, so a pair comes from those clients) [inference for the two platforms; M13
measured the Mac].

## The refusal, from the code

[code `internal/repo/casefold.go:200-211`, called from `internal/repo/push.go:444`; the item form
`casefold.go:185-193`; the relation's name `casefold.go:90-98`]. Its template is
`<dst> conflicts with <items>: <harm> (README, Honest limits), so this create is refused on every
platform`, each item `<ref> (<ours> vs <theirs>, by <relation>: <%+q ours> vs <%+q theirs>)`. With
only the NFC `cafe` beside it, there is one item, and the relation is `Unicode normalisation`
(the two names share the canonical key but not the case key [code `casefold.go:73-87`]), so the
harm is `a Mac clone would mishandle the pair`.

git prints a ref's short name raw, and the helper prints both ref names raw, so on screen the two
spellings look the same. The check therefore reads the output in the **escaped view**: each
character outside printable ASCII and tab written `<U+XXXX>`, and a backslash written `<BS>`. The
raw NFD name reads `cafe<U+0301>`, the raw NFC name `caf<U+00E9>`, and the helper's quoted forms
`"heads/cafe<BS>u0301"` and `"heads/caf<BS>u00e9"`. git frames the refusal as
` ! [remote rejected] main -> <dst short name> (<reason>)` (the v0.10.0 briefs' case refusal).

**The expected line, in the escaped view, matched as a substring of one line of the push's output:**

```text
-> cafe<U+0301> (refs/heads/cafe<U+0301> conflicts with refs/heads/caf<U+00E9> (heads/cafe<U+0301> vs heads/caf<U+00E9>, by Unicode normalisation: "heads/cafe<BS>u0301" vs "heads/caf<BS>u00e9"): a Mac clone would mishandle the pair (README, Honest limits), so this create is refused on every platform)
```

It is the macOS brief's `[nfd-create]` entry, character for character.

## What the host brief adds for these steps

*Brief author's call:* a remote of its own, so the pair warning a case pair now prints on every
platform (spec section 3, "The warning") cannot appear in these steps, and the host brief's other
steps keep their expectations.

- **A root:** `/my-files/GitRemotes/vNEXT-<platform>-nfd-<SUF>` (`vNEXT-linux-nfd-a1` and
  `vNEXT-win-nfd-a1` for the first attempts), named in the host brief's root list, its
  confinement (rule 2) and its ownership record, as the v0.10.0 briefs named theirs.
- **Its trash tally (rule 7):** three `.lock` files (three pushes), and at cleanup the root with
  its descendants. Its frame at cleanup: `gpb-remote.json`, `HEAD`, `refs`, `refs/heads` (holding
  `main` and `caf<U+00E9>`), `refs/tags`, `packs` with one pack pair.
- **Two kit functions,** added to the host brief's pwsh kit (`kit.ps1` for Linux, `win-kit.ps1` for
  Windows, as v0.10.0 named them). They are the macOS brief's PC kit's escaped view, unchanged:

```powershell
# The escaped view: outside printable ASCII and tab, <U+XXXX>; a backslash, <BS>.
function ConvertTo-Esc([string]$s) {
    $b = [Text.StringBuilder]::new()
    foreach ($r in $s.EnumerateRunes()) {
        $v = $r.Value
        if ($v -eq 92) { [void]$b.Append('<BS>') }
        elseif ($v -eq 9 -or ($v -ge 32 -and $v -le 126)) { [void]$b.Append([char]$v) }
        else { [void]$b.Append(('<U+{0:X4}>' -f $v)) }
    }
    $b.ToString()
}
# Prints a file's escaped view, then FOUND or MISSING for an expected substring of one line;
# MISSING throws, so the block stops there.
function Test-EscapedLine([string]$File, [string]$Expected) {
    $lines = @([IO.File]::ReadAllLines($File, [Text.UTF8Encoding]::new($false)) | ForEach-Object { ConvertTo-Esc $_ })
    $lines | ForEach-Object { "  | $_" }
    $at = 0; for ($i = 0; $i -lt $lines.Count; $i++) { if ($lines[$i].Contains($Expected, [StringComparison]::Ordinal)) { $at = $i + 1; break } }
    if ($at) { "EXPECTED LINE: FOUND at line $at" } else { "EXPECTED LINE: MISSING"; throw 'BLOCKED: the refusal text differs' }
}
$NfdRefusal = '-> cafe<U+0301> (refs/heads/cafe<U+0301> conflicts with refs/heads/caf<U+00E9> (heads/cafe<U+0301> vs heads/caf<U+00E9>, by Unicode normalisation: "heads/cafe<BS>u0301" vs "heads/caf<BS>u00e9"): a Mac clone would mishandle the pair (README, Honest limits), so this create is refused on every platform)'
```

`[Console]::OutputEncoding` is already UTF-8 in both v0.10.0 kits, so git's UTF-8 output reaches
the log intact.

## The Linux steps (WSL scripts and Windows snapshots, in the v0.10.0 Linux brief's shape)

Run after the host brief's case step. `$NFDL` is the new root, set in the WSL `env.sh` beside
`$CASE`, and `$NFDL` in the pwsh kit beside `$CASE`.

**N.1 Seed, then the NFC name:** `Invoke-GateWsl sN1-nfd-seed`, then the pwsh block.

```bash
# script: sN1-nfd-seed
set -u
source "$HOME/gpb010/${1:?no run named}/env.sh" || exit 1
cd "$G" || exit 1
if [ -e nfd ]; then echo "BLOCKED: $G/nfd already exists"; exit 1; fi
NFC=$'caf\xc3\xa9'
gate_repo nfd && cd nfd || exit 1
echo 'nfd 1' > n.txt && git add n.txt && git commit -q -m 'gate: nfd 1' || exit 1
git remote add proton-v2 "proton::$NFDL" || exit 1
git push -u proton-v2 main; rc=$?
echo "push main: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
git push proton-v2 "main:refs/heads/$NFC"; rc=$?
echo "push the NFC name: exit $rc"; [ "$rc" = 0 ] || exit "$rc"
echo 'nfd 2, withheld' >> n.txt && git commit -q -am 'gate: nfd 2, withheld' || exit 1
echo "C2 (local main, not on the remote): $(git rev-parse main)"
```

```powershell
. C:\gpb010\kit.ps1
Update-RootRecord
Save-Snapshot nfd-0 $NFDL -Recurse
```

**Expected:** both pushes exit 0, `RECORDED: .../vNEXT-linux-nfd-<SUF>`, and `nfd-0` holding the
frame, `refs/heads/main` and `refs/heads/caf<U+00E9>` (the snapshot's path column is raw; read it
through `ConvertTo-Esc` if in doubt), and one pack pair.

**N.2 The NFD create, by exact bytes, carrying the withheld C2:** `Invoke-GateWsl sN2-nfd-create`,
then the pwsh block.

```bash
# script: sN2-nfd-create
set -u
source "$HOME/gpb010/${1:?no run named}/env.sh" || exit 1
cd "$G/nfd" || exit 1
NFD=$'cafe\xcc\x81'
echo "dst as the script passes it: $(printf '%s' "refs/heads/$NFD" | od -An -tx1 | tr -d '[:space:]')"
git push proton-v2 "main:refs/heads/$NFD"; rc=$?
echo "push main:<the NFD name>: exit $rc (expected 1: refused)"
echo "refs/heads (WSL view): $(names "$NFDL/refs/heads")"
echo "packs (WSL view): $(names "$NFDL/packs")"
exit "$rc"
```

```powershell
. C:\gpb010\kit.ps1
Test-EscapedLine (Join-Path $G 'log' 'sN2-nfd-create.txt') $NfdRefusal
Save-Snapshot nfd-1 $NFDL -Recurse
Compare-Snapshot nfd-0 nfd-1
```

**Pass:** all of the following hold.

- `dst as the script passes it:` ends `63616665cc81` (the bytes bash hands git, no more), and
  `EXIT sN2-nfd-create = 1`.
- `EXPECTED LINE: FOUND`: the refusal names both spellings and the relation, exactly as above. It
  is also what shows the bytes git sent the helper: the helper names the destination it
  received, and the escaped view reads `cafe<U+0301>` only if it arrived as NFD.
- **Nothing written, the v0.10.0 shape:** `COMPARE nfd-0 -> nfd-1: NO DIFFERENCE`, and the WSL
  view, from the session that pushed, agrees: `refs/heads (WSL view)` lists `main` and the NFC name
  (`names` prints it raw: one name of the two, never both) and `packs (WSL view)` N.1's one pair.
  No pack was uploaded, because C2 was withheld and the helper never trashes a pack (its only
  trash calls are the lock's release, a ref delete and two empty-folder cleanups [code
  `internal/repo/lock.go:135`, `internal/repo/push.go:606, 922, 1181`]); that no ref was written
  rests on the refusal being phase 2's, before anything moves [code
  `internal/repo/push.go:503-509`], the v0.10.0 briefs' stated strength.
- A `LAG:` line may name only `.../vNEXT-linux-nfd-<SUF>/.lock`. No unexpected
  `git-remote-proton:` line.

## The Windows steps (in the v0.10.0 Windows brief's shape)

Run after the host brief's case refusal (v0.10.0's 4.4). `$NFDW` is the new root in `win-kit.ps1`.

**N.1 Seed, then the NFC name:**

```powershell
. C:\gpb010\win-kit.ps1
Assert-Run
Test-WindowsHelper -Require
$x = Join-Path $G 'w' 'pnfd'
if (Test-Path -LiteralPath $x) { throw "BLOCKED: $x already exists" }
$NFC = "caf$([char]0x00E9)"
git init -q -b main $x; Assert-Exit 'git init'
git -C $x config user.name 'gpb gate'; Assert-Exit 'config user.name'
git -C $x config user.email 'gate@gpb.invalid'; Assert-Exit 'config user.email'
Set-Content -LiteralPath (Join-Path $x 'n.txt') -Value 'nfd 1'
git -C $x add n.txt; Assert-Exit 'add n.txt'
git -C $x commit -q -m 'gate: nfd 1'; Assert-Exit 'commit 1'
git -C $x remote add proton-v2 "proton::$NFDW"; Assert-Exit 'remote add'
git -C $x push -u proton-v2 main 2>&1; Assert-Exit 'push main'
git -C $x push proton-v2 "main:refs/heads/$NFC" 2>&1; Assert-Exit 'push the NFC name'
Add-Content -LiteralPath (Join-Path $x 'n.txt') -Value 'nfd 2, withheld'
git -C $x commit -q -am 'gate: nfd 2, withheld'; Assert-Exit 'commit 2'
"C2 (local main, not on the remote): $(git -C $x rev-parse main)"
Save-Snapshot nfd-0 $NFDW -Recurse
```

Then the host brief's own ownership record line for the new root (v0.10.0's Windows brief records
a creation in the creating block, `Save-Creation`).

**Expected:** both pushes `exit 0`, and `nfd-0` holding the frame, `refs/heads/main` and
`refs/heads/caf<U+00E9>`, and one pack pair.

**N.2 The NFD create, by exact bytes, carrying the withheld C2:**

```powershell
. C:\gpb010\win-kit.ps1
Assert-Run
Test-WindowsHelper -Require
$x = Join-Path $G 'w' 'pnfd'
$NFD = "cafe$([char]0x0301)"
"dst as PowerShell encodes it (UTF-8): $(([Text.Encoding]::UTF8.GetBytes("refs/heads/$NFD") | ForEach-Object { $_.ToString('x2') }) -join '')"
$out = @(git -C $x push proton-v2 "main:refs/heads/$NFD" 2>&1 | ForEach-Object { "$_" }); $rc = $LASTEXITCODE
[IO.File]::WriteAllLines((Join-Path $G 'log' 'composition-create.txt'), [string[]]$out, [Text.UTF8Encoding]::new($false))
"push main:<the NFD name>: exit $rc (expected 1: refused) ($(Get-Date -Format o))"
if ($rc -ne 1) { throw "BLOCKED: the push exited $rc, not 1" }
Test-EscapedLine (Join-Path $G 'log' 'composition-create.txt') $NfdRefusal
Save-Snapshot nfd-1 $NFDW -Recurse
Compare-Snapshot nfd-0 nfd-1
```

**Pass:** all of the following hold.

- `dst as PowerShell encodes it` ends `63616665cc81`. That is PowerShell's own encoding, no more;
  that Git for Windows passed those bytes on unchanged is [inference] (it reads its arguments as
  wide strings and converts them to UTF-8), and the refusal text below is what shows it.
- `exit 1` and `EXPECTED LINE: FOUND`, as on Linux, character for character: the same rule on the
  Windows binary.
- **Nothing written:** `COMPARE nfd-0 -> nfd-1: NO DIFFERENCE`. The Windows session is the one that
  pushed, so its view is the writer's own. No pack (C2 withheld), no ref (phase 2), as on Linux.
- A `LAG:` line may name only `.../vNEXT-win-nfd-<SUF>/.lock`. No unexpected `git-remote-proton:`
  line.

## Cleanup

The new root is trashed with the host brief's other roots, verify-before-trash (rule 3), under that
brief's ownership record; its frame is above.

## Not covered by these steps

- A Mac clone's handling of such a pair, the Mac refusals and the Mac push rule: the macOS gate
  (its steps 5.1 to 5.11).
- A pair matching both relations (a case and a composition variant at once): the macOS gate's 5.4
  refuses one; hermetic tests cover the rest.
