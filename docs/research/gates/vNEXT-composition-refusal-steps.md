# vNEXT composition-refusal steps, for this release's Windows and Linux gate briefs (O5(d))

**Status: steps to paste, not a brief, and not run.** Section 6 of the macOS port spec
(`docs/superpowers/specs/2026-10-27-v2-macos-port-design.md`, "The one live macOS gate") says:
"The release's Windows and Linux gates also test the composition refusal on their own binaries, as
v0.10.0's tested the case refusal per platform." These are the steps those two briefs carry. The
briefs themselves are written at release time from the v0.10.0 briefs plus this file (plan
`docs/superpowers/plans/2026-10-27-v2-macos-port.md`, Task 15 and "Not tasks"). Written on
2026-10-27 on branch `worktree-macos-port`, and re-anchored on 2026-10-28; code read at
**`0faada8`**, the macOS brief's anchor. The code it cites is unchanged from `119227e` there but
for two lines of `cmd/git-remote-proton/main.go` [measured 2026-10-28: among the code paths, `git
diff 119227e 0faada8` changes only that file's lines 493-495 and 526-527, the Mac list refusals'
MA3 tails, which these steps never meet, and its test]. The host brief's anchor check must cover
every code file these steps cite or rely on: `internal/repo/casefold.go`, `internal/repo/push.go`,
`internal/repo/lock.go`, `internal/repo/readcheck.go`, `internal/gitcmd/gitcmd.go`,
`internal/transport/trace.go` and `cmd/git-remote-proton/main.go`. The macOS brief's C.2 requires
the same files unchanged from `0faada8` to the tag (its pathspec holds `internal` and `cmd`), so no
change is expected; but the Linux and Windows gates run before it, so their own check must catch
one. A change to any of them re-derives, before the gate, the expected lines (the refusal, the
case forms, the warning, the `.lock` read-back), N.1's one pack pair and N.2's "nothing written"
argument.

**`vNEXT` is a placeholder** for the release that carries the macOS port, whose version Craig has
not named. When he names it, this file is renamed with the version and every `vNEXT` replaced, by
script. **The file is ASCII and holds no backslash-u sequence.** Its backslashes are the bash byte
escapes and the Windows paths; the helper's own backslash, in the expected line, is written `<BS>`
(the escaped view, below), so an editor tool cannot mangle it. **`gpb010` in the blocks** (`$HOME/gpb010`
in WSL, `C:\gpb010` on Windows) stands for the host brief's own scratch folder, v0.10.0's name for
it: the host brief author replaces it with that brief's folder when pasting.

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

It is the macOS brief's `[nfd-create]` entry, character for character. It has one item, the NFC
ref, because the root is fresh: the host briefs refuse a root whose name already exists [record
`v0.10.0-linux-gate-brief.md:563-565`; `v0.10.0-windows-gate-brief.md:262`, which the next section
extends to the new root]. A leftover ref there would change the line, a BLOCK, never a false pass.

## What the host brief adds for these steps

*Brief author's call:* a remote of its own, so the pair warning a case pair now prints on every
platform (spec section 3, "The warning") cannot appear in these steps, and so these steps do not
depend on the host brief's other steps. (The alternative not taken: seed the NFC name on the host
brief's smoke root after its case step, keeping the Windows kit single-root, at the cost of
changing smoke's frame.)

- **A root:** `/my-files/GitRemotes/vNEXT-<platform>-nfd-<SUF>` (`vNEXT-linux-nfd-a1` and
  `vNEXT-win-nfd-a1` for the first attempts), named in the host brief's root list, its
  confinement (rule 2) and its ownership record, as the v0.10.0 briefs named theirs. **Linux:** add
  the role `nfd` to `$RootNames` in `kit.ps1` (v0.10.0's is built from the roles `contract`,
  `smoke`, `xl`, `xw`, `case` [record `v0.10.0-linux-gate-brief.md:193`]), which `Update-RootRecord`
  reads, and set `$NFDL` beside `$CASE`. **Windows:** define `$NfdwName` and `$NFDW` beside
  `$SmokeName` and `$SMOKE` [record `v0.10.0-windows-gate-brief.md:212`], and extend
  `Start-SmokeRecord`, `Test-SmokeRecord`, 5.1's enumeration and 5.2's trash, which name the smoke
  root alone [record `v0.10.0-windows-gate-brief.md:254-290, 1197, 1223-1227`], to the second root.
- **Its restore-probe diagnosis:** add `nfd` to the WSL repositories v0.10.0's diagnosis loops over
  and `pnfd` to its Windows list [record `v0.10.0-linux-gate-brief.md:1805, 1820`], so a D5
  sighting in N.1 or N.2 is diagnosed there too.
- **Its trash tally (rule 7):** three `.lock` files (three pushes), and at cleanup the root with
  its descendants. Its frame at cleanup: `gpb-remote.json`, `HEAD`, `refs`, `refs/heads` (holding
  `main` and `caf<U+00E9>`), `refs/tags`, `packs` with one pack pair: the NFC push carries C1, which
  the remote already holds, so its pack would be empty and none is sent [code
  `internal/repo/push.go:511-520`, `internal/gitcmd/gitcmd.go:314-315`].
- **Two kit functions and a constant,** added to the host brief's pwsh kit (`kit.ps1` for Linux,
  `win-kit.ps1` for Windows, as v0.10.0 named them). `ConvertTo-Esc` is the macOS brief's PC kit's,
  unchanged; `Test-EscapedLine` is new, a single-entry form of that kit's `Test-Expect`; and
  `$NfdRefusal` is the macOS brief's `[nfd-create]` entry. All three go into the host kit:

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

**What else changes in the host brief at this release** (it is written from the v0.10.0 briefs, whose
case steps predate both changes):

- **String 1's case form.** v0.10.0's case steps expect `... conflicts by letter case with <ref>
  (<ours> vs <theirs>): a Windows clone would mishandle the pair ...` [record
  `v0.10.0-linux-gate-brief.md:2566, 2706, 2801`; `v0.10.0-windows-gate-brief.md:1069`]. The code
  now writes the macOS brief's `[case-Main]`, `[case-Featx]` and `[case-MAIN]` form [code
  `internal/repo/casefold.go:200-211`], each item naming every member the remote holds, which the
  host brief author checks against its own remote's refs:

```text
main -> Main (refs/heads/Main conflicts with refs/heads/main (heads/Main vs heads/main, by letter case: "heads/Main" vs "heads/main"): a Windows or Mac clone would mishandle the pair (README, Honest limits), so this create is refused on every platform)
main -> Feat/x (refs/heads/Feat/x conflicts with refs/heads/feat (heads/Feat vs heads/feat, by letter case: "heads/Feat" vs "heads/feat"): a Windows or Mac clone would mishandle the pair (README, Honest limits), so this create is refused on every platform)
main -> MAIN (refs/heads/MAIN conflicts with refs/heads/Main (heads/MAIN vs heads/Main, by letter case: "heads/MAIN" vs "heads/Main"), refs/heads/main (heads/MAIN vs heads/main, by letter case: "heads/MAIN" vs "heads/main"): a Windows or Mac clone would mishandle the pair (README, Honest limits), so this create is refused on every platform)
```

- **The pair warning on every platform** (spec section 3, "The warning") [code
  `cmd/git-remote-proton/main.go:563-570`, `internal/repo/readcheck.go:57-81`]. Wherever the host's
  remote holds a case pair, every helper `list` and `list for-push` of it (a push, fetch, clone or
  `ls-remote`) prints the macOS brief's `[warn-case]` lines. In v0.10.0's briefs that is Linux 5.4
  alone, after `s5g-upload-Main` laid `Main` down beside `main`: `s5h-side` prints them twice, for
  its push and for its `ls-remote`, and `s5i-MAIN` once [record
  `v0.10.0-linux-gate-brief.md:2718-2795`]. No later Linux step reaches that remote [measured
  2026-10-28: no line of that brief after 2771 names `$CASE`], and no Windows remote holds a pair
  (its case create, 4.4, is refused) [record `v0.10.0-windows-gate-brief.md:1069`]. v0.10.0 treats
  an unexpected `git-remote-proton:` line as BLOCKED [record `v0.10.0-linux-gate-brief.md:715,
  2326`], so the host brief expects these two lines at each of those three places:

```text
git-remote-proton: warning: this remote holds refs that differ only by letter case or Unicode normalisation; see README, Honest limits:
  refs/heads/Main (<OID>, "refs/heads/Main") and refs/heads/main (<OID>, "refs/heads/main"): by letter case
```

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
(Get-Content -LiteralPath (Join-Path $G 'snap' 'nfd-0.json') -Raw | ConvertFrom-Json) | ForEach-Object { "  $(ConvertTo-Esc $_.path)" }
```

**Expected:** both pushes exit 0, `RECORDED: .../vNEXT-linux-nfd-<SUF>`, and `nfd-0` holding the
frame, `refs/heads/main` and `refs/heads/caf<U+00E9>` (in the escaped paths the block prints), and
one pack pair [code `internal/repo/push.go:511-520`, `internal/gitcmd/gitcmd.go:314-315`: the NFC
push's pack would be empty]. A `LAG:` line may name only `.../vNEXT-linux-nfd-<SUF>/.lock`.

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
h=$(names "$NFDL/refs/heads") || { echo "WSL listing of refs/heads failed"; exit 99; }
p=$(names "$NFDL/packs") || { echo "WSL listing of packs failed"; exit 99; }
echo "refs/heads (WSL view): $h"
echo "packs (WSL view): $p"
exit "$rc"
```

```powershell
. C:\gpb010\kit.ps1
Save-Snapshot nfd-1 $NFDL -Recurse
Compare-Snapshot nfd-0 nfd-1
Test-EscapedLine (Join-Path $G 'log' 'sN2-nfd-create.txt') $NfdRefusal
```

**Pass:** all of the following hold.

- `dst as the script passes it:` ends `63616665cc81` (the bytes bash hands git, no more), and
  `EXIT sN2-nfd-create = 1` (99 means a WSL listing failed: BLOCKED). The block takes the
  after-snapshot before it judges the text, so a BLOCK still leaves the remote's state recorded.
- `EXPECTED LINE: FOUND`: the refusal names both spellings and the relation, exactly as above. It
  is also what shows the bytes git sent the helper: the helper names the destination it
  received, and the escaped view reads `cafe<U+0301>` only if it arrived as NFD.
- **Nothing written, the v0.10.0 shape:** `COMPARE nfd-0 -> nfd-1: NO DIFFERENCE`, and the WSL
  view, from the session that pushed, agrees: `refs/heads (WSL view): caf<U+00E9> main` in the
  escaped print of the log above (one name of the two, never both), and `packs (WSL view)` N.1's
  one pair.
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
$log = Join-Path $G 'log' 'N1-nfd-seed.txt'
try { git -C $x push -u proton-v2 main 2>&1 | Tee-Object -FilePath $log; Assert-Exit 'push main' }
finally { if (@(Get-Rows $GR | Where-Object { $_.name -ceq $NfdwName -and -not $_.trashTime }).Count) { Save-Creation $NfdwName $GR created-by-N.1 } else { 'N.1: no root exists after the push, so none is recorded' } }
git -C $x push proton-v2 "main:refs/heads/$NFC" 2>&1 | Tee-Object -FilePath $log -Append; Assert-Exit 'push the NFC name'
Add-Content -LiteralPath (Join-Path $x 'n.txt') -Value 'nfd 2, withheld'
git -C $x commit -q -am 'gate: nfd 2, withheld'; Assert-Exit 'commit 2'
"C2 (local main, not on the remote): $(git -C $x rev-parse main)"
Save-Snapshot nfd-0 $NFDW -Recurse
(Get-Content -LiteralPath (Join-Path $G 'snap' 'nfd-0.json') -Raw | ConvertFrom-Json) | ForEach-Object { "  $(ConvertTo-Esc $_.path)" }
```

The first push is wrapped as v0.10.0's Windows brief wraps its root's first push [record
`v0.10.0-windows-gate-brief.md:933-934`]: `Bootstrap` creates the root before anything else can
fail, so the root is recorded whatever happens after it.

**Expected:** both pushes `exit 0` (their output in `log\N1-nfd-seed.txt`, with no
`git-remote-proton:` line), the root recorded `created-by-N.1`, and `nfd-0` holding the frame,
`refs/heads/main` and `refs/heads/caf<U+00E9>` (in the escaped paths), and one pack pair, for the
reason given on Linux. A `LAG:` line may name only `.../vNEXT-win-nfd-<SUF>/.lock`.

**N.2 The NFD create, by exact bytes, carrying the withheld C2:**

```powershell
. C:\gpb010\win-kit.ps1
Assert-Run
Test-WindowsHelper -Require
$x = Join-Path $G 'w' 'pnfd'
$NFD = "cafe$([char]0x0301)"
"dst as PowerShell encodes it (UTF-8): $(([Text.Encoding]::UTF8.GetBytes("refs/heads/$NFD") | ForEach-Object { $_.ToString('x2') }) -join '')"
$out = @(git -C $x push proton-v2 "main:refs/heads/$NFD" 2>&1 | ForEach-Object { "$_" }); $rc = $LASTEXITCODE
[IO.File]::WriteAllLines((Join-Path $G 'log' 'N2-nfd-create.txt'), [string[]]$out, [Text.UTF8Encoding]::new($false))
"push main:<the NFD name>: exit $rc (expected 1: refused) ($(Get-Date -Format o))"
Save-Snapshot nfd-1 $NFDW -Recurse
Compare-Snapshot nfd-0 nfd-1
$ls = @(git -C $x ls-remote proton-v2 2>&1 | ForEach-Object { "$_" }); $lc = $LASTEXITCODE
$ls | ForEach-Object { "  ls-remote | $(ConvertTo-Esc $_)" }
if ($lc -ne 0) { throw "BLOCKED: ls-remote exited $lc" }
Test-EscapedLine (Join-Path $G 'log' 'N2-nfd-create.txt') $NfdRefusal
if ($rc -ne 1) { throw "BLOCKED: the push exited $rc, not 1" }
```

**Pass:** all of the following hold.

- `dst as PowerShell encodes it` ends `63616665cc81`. That is PowerShell's own encoding, no more;
  that Git for Windows passed those bytes on unchanged is [inference] (it reads its arguments as
  wide strings and converts them to UTF-8), and the refusal text below is what shows it.
- `exit 1` and `EXPECTED LINE: FOUND`, as on Linux, character for character: the same rule on the
  Windows binary. The push's output (in `log\N2-nfd-create.txt`, printed escaped by
  `Test-EscapedLine`) also holds the `.lock` read-back line, `gpb: downloaded .../.lock (<n>
  bytes)` [code `internal/transport/trace.go:43`], as v0.10.0's 4.4 expected [record
  `v0.10.0-windows-gate-brief.md:1071-1078`]. The
  block takes the after-snapshot and `ls-remote` before it judges the exit and the text, so a BLOCK
  still leaves both recorded and the output printed.
- **Nothing written:** `COMPARE nfd-0 -> nfd-1: NO DIFFERENCE`, and `ls-remote` exits 0 listing
  `HEAD`, `refs/heads/main` and `refs/heads/caf<U+00E9>`, all at N.1's commit, and no NFD name.
  The Windows session is the one that pushed, so its view is the writer's own. No pack (C2
  withheld), no ref (phase 2), as on Linux.
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
- A mixed push off the Mac (an update that lands beside a refused NFD create, in one push): only
  the macOS gate's 5.2 runs one, from the Mac.

## Review record

Multi-model review, round 1, 2026-10-28 (Codex, Gemini, DeepSeek, Kimi), on `7d27ae3`. Applied:

- S1: the separate root no longer claims the host's other steps keep their expectations; the case
  form and the pair warning that change are listed.
- S2: the new root wired into the host kits' ownership record and cleanup, Linux and Windows.
- S3: Windows N.1 records its root in a `finally` around the first push.
- S4: both N.2 blocks take the after-state before judging; Windows N.2 adds `ls-remote` and the
  `.lock` read-back.
- S5: the host brief's anchor check lists the three cited files.
- S8: why one pack pair, cited.
- S9: the mixed push off the Mac, under "Not covered".
- S10: the `LAG:` allowance in both N.1 blocks.
- S11: which kit pieces are unchanged and which are new.
- S13: the backslash sentence.
- S14: the names read in the escaped view, printed by the blocks.
- S16: Windows N.1 logs its pushes; the logs are named after their steps.
- S21: `gpb010` stands for the host brief's scratch folder.
- S23: a failed WSL listing exits 99.
- S24: `nfd` and `pnfd` in the restore-probe diagnosis lists.

Not applied here: S22, a line for this file in `docs/README.md`, which is outside this file.

Rejected, so not to be raised again:

- S6: the whole expected line names the NFC ref and the relation, which a blanket rejection would not.
- S7: the pushing session's own view is v0.10.0's freshness rule.
- S12: the expected line equals the macOS brief's entry byte for byte (checked by script).
- S15: the host briefs forbid a re-run of a push.
- S17: the cited trash calls are all of the non-test ones.
- S18: `push.go:503-509` is phase 2's loop, which ends before phase 3.
- S19: the host kits are pwsh 7; four-digit escapes are allowed; `$NFDL` is consistent.
- S20: the whole-line match is all three briefs' rule; O5(d) is Craig's ruling.
- S25: the cited guardrails do not exist in the v0.10.0 Windows brief.
- S26: the pre-adoption scan is outside these steps and fails closed.
- S27: its concrete parts are S1 and S2; its alternative is noted under S2's fix.

Multi-model review, round 2, 2026-10-28 (Codex, Gemini, DeepSeek, Kimi), on `6bf53aa`. Codex and
Gemini raised nothing new. Applied:

- S2-1 (Kimi, DeepSeek): the anchor check covers every code file cited or relied on, adding
  `readcheck.go`, `gitcmd.go`, `trace.go` and `main.go`; the note on `main.go`'s MA3 lines
  corrected.
- S2-2 (DeepSeek, Kimi): the warning's and the `.lock` read-back's code cited; the warning lines
  equal the macOS brief's `[warn-case]`, which its expected-texts check matched to the code.
- S2-3 (DeepSeek): every helper `list` of a pair-holding remote warns; `s5h-side` warns twice.
- S2-4 (DeepSeek): why a fresh root keeps the expected line to its one item.
- S2-5: S22 has since been applied, in `docs/README.md` (`7f1ffd5`).

Rejected, so not to be raised again:

- S2-6 (DeepSeek): listing `main.go` is not "permanently red": C.2 holds `cmd` unchanged.
- S2-7 (DeepSeek): v0.10.0's `names` sorts, so the WSL view's order is fixed.
