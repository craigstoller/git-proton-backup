<#
.SYNOPSIS
A4 - trash and the namespace. Does a trashed file still occupy its name?

.DESCRIPTION
Memo: docs/research/remote-helper-prior-art.md §3b, assumption A4.

WHY IT MATTERS. In the v2 design a ref is a file: refs/heads/<branch>. Deleting a branch trashes
that file; recreating the branch re-creates it. Proton's delete is two-stage (trash, then empty),
which the memo calls "arguably safer" - unless a trashed node keeps holding its name for
create-exclusive purposes. If it does, then

    git push proton :feature      # delete branch
    git push proton feature       # recreate it

fails on the second command until the user empties their Proton trash by hand. That is
unacceptable UX, and it would force storage names to be decoupled from ref names via an
indirection layer.

WHAT IS TESTED
  1. upload v1 (content "AAAA", 4 bytes)  -> assert present, record node uid
  2. trash it                              -> assert the path no longer resolves
  3. upload v2, SAME name, different size  -> exit code is data, not a preconditio
  4. read the path back                    -> uid and size decide the verdict

Sizes differ deliberately (4 vs 8 bytes). If the second upload were silently skipped on
conflict, an identical file would make failure look like success.

READING THE RESULT
  new uid, size 8   -> A4 HOLDS: trash freed the name
  path won't resolve-> A4 FAILS: the name could not be recreated
  old uid returned  -> AMBIGUOUS: the trashed node may have been restored, not replaced
  size 4            -> A4 FAILS: the old node still owns the name
  precondition fail -> ERROR, no conclusion drawn

.NOTES
Writes only under /my-files/_cas-probe (enforced by probe-lib.ps1, no override).
#>
[CmdletBinding()]
param([switch]$SkipSweepGate, [switch]$NoCleanup)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'probe-lib.ps1')

'=== A4 - trash and the namespace ==='
Assert-FleetGreen -SkipSweepGate:$SkipSweepGate
[void](Initialize-ProbeRoot)
"probe root ready: $script:ProbeRoot"

$ws   = New-ProbeWorkspace
$name = 'a4-ref-probe.txt'

function Get-NodeFacts {
    param([Parameter(Mandatory)][string]$Path)
    [void](Assert-ProbePath -Path $Path)
    $r = Invoke-ProbeCli -CliArgs @('filesystem','info',$Path,'--json') -RemotePaths @($Path)
    $uid = $null; $size = $null; $sha1 = $null; $obj = $null
    if ($r.ExitCode -eq 0) {
        try {
            $obj = $r.Output | ConvertFrom-Json
            $uid = $obj.uid
            # The plaintext length is claimedSize. NOT ".size" (absent) and NOT storageSize /
            # totalStorageSize, which include E2EE overhead - an 8-byte file reports 86.
            #
            # The CLI dropped the {ok,value} Result wrapper between 0.4.6 and 0.7.0, so BOTH
            # shapes must be read. This probe originally handled only the wrapped form and, on
            # 0.7.0, reported "size unreadable" for a perfectly good result - the same defect
            # that made the shipped module report every bundle unconfirmed (fixed in 0.2.4).
            $rev = $null
            if ($obj -and $obj.PSObject.Properties['activeRevision']) {
                $ar = $obj.activeRevision
                $rev = if ($ar.PSObject.Properties['value']) { $ar.value } else { $ar }
            }
            if ($rev) {
                try { $size = $rev.claimedSize } catch { }
                try { $sha1 = $rev.claimedDigests.sha1 } catch { }
            }
        } catch { }
    }
    [pscustomobject]@{ Exists = ($r.ExitCode -eq 0); ExitCode = $r.ExitCode; Uid = $uid; Size = $size; Sha1 = $sha1; Raw = $r.Output; Obj = $obj }
}

try {
    $sub    = New-ProbeFolder -Parent $script:ProbeRoot -Name 'a4'
    $remote = "$sub/$name"

    # --- 1. create -------------------------------------------------------------------
    $d1 = Join-Path $ws 'v1'; New-Item -ItemType Directory -Path $d1 -Force | Out-Null
    [System.IO.File]::WriteAllText((Join-Path $d1 $name), 'AAAA')
    $u1 = Invoke-ProbeCli -CliArgs @('filesystem','upload','-f','skip',(Join-Path $d1 $name),$sub) -RemotePaths @($sub)
    "1. upload v1 (4 B) -> exit $($u1.ExitCode)"; if ($u1.Output) { "   $($u1.Output)" }
    [void](Assert-CliOk -Result $u1 -What 'uploading the initial probe file')

    $f1 = Get-NodeFacts -Path $remote
    "   info -> exists=$($f1.Exists) uid=$($f1.Uid) size=$($f1.Size)"
    if (-not $f1.Exists) { throw "PROBE PRECONDITION FAILED - could not read back the file just uploaded. Raw: $($f1.Raw)" }

    # --- 2. trash --------------------------------------------------------------------
    $t = Invoke-ProbeCli -CliArgs @('filesystem','trash',$remote) -RemotePaths @($remote)
    "2. trash -> exit $($t.ExitCode)"; if ($t.Output) { "   $($t.Output)" }
    [void](Assert-CliOk -Result $t -What 'trashing the probe file')

    $f2 = Get-NodeFacts -Path $remote
    "   info after trash -> exists=$($f2.Exists) (false expected: the node should leave its path)"
    $after = Get-ProbeNodes -Path $sub
    "   folder now contains $($after.Count) node(s): $(($after | ForEach-Object { $_.name.value }) -join ', ')"

    # --- 3. recreate the same name ---------------------------------------------------
    $d2 = Join-Path $ws 'v2'; New-Item -ItemType Directory -Path $d2 -Force | Out-Null
    [System.IO.File]::WriteAllText((Join-Path $d2 $name), 'BBBBBBBB')
    $u2 = Invoke-ProbeCli -CliArgs @('filesystem','upload','-f','skip',(Join-Path $d2 $name),$sub) -RemotePaths @($sub)
    "3. upload v2 (8 B, same name) -> exit $($u2.ExitCode)"; if ($u2.Output) { "   $($u2.Output)" }

    # --- 4. read back ----------------------------------------------------------------
    $f3 = Get-NodeFacts -Path $remote
    "4. info after recreate -> exists=$($f3.Exists) uid=$($f3.Uid) claimedSize=$($f3.Size) sha1=$($f3.Sha1)"
    # Independent content check: sha1 of the v2 bytes must match what Proton reports back.
    $expect = (([System.Security.Cryptography.SHA1]::Create().ComputeHash([Text.Encoding]::UTF8.GetBytes('BBBBBBBB')) | ForEach-Object { $_.ToString('x2') }) -join '')
    "   expected sha1 of v2 content: $expect  -> match=$($f3.Sha1 -eq $expect)"
    if (-not $f3.Exists) { "   raw: $($f3.Raw)" }
    $final = Get-ProbeNodes -Path $sub
    "   folder now contains $($final.Count) node(s): $(($final | ForEach-Object { "$($_.name.value) [$($_.uid)]" }) -join ', ')"

    $verdict =
        if (-not $f3.Exists)                { 'A4 FAILS - the name could not be recreated after trash' }
        elseif ($f3.Uid -eq $f1.Uid)        { 'A4 AMBIGUOUS - same node uid returned; trashed node may have been restored rather than replaced' }
        elseif ($f3.Size -eq 8)             { 'A4 HOLDS - trash freed the name; a NEW node owns it' }
        elseif ($f3.Size -eq 4)             { 'A4 FAILS - the old 4-byte node still occupies the name (second upload was skipped)' }
        elseif ($null -eq $f3.Size)         { 'A4 LIKELY HOLDS - new uid, but size unreadable from the info payload; check the raw JSON below' }
        else                                { "A4 INCONCLUSIVE - new uid but size reads '$($f3.Size)'" }

    "`nVERDICT: $verdict"
    "  uid before trash  : $($f1.Uid)"
    "  uid after recreate: $($f3.Uid)"
    if ($null -eq $f3.Size -and $f3.Obj) { "`n  raw info JSON after recreate:`n$($f3.Raw)" }
}
finally {
    Remove-Item $ws -Recurse -Force -ErrorAction SilentlyContinue
    if (-not $NoCleanup) { "`n"; Remove-ProbeTree } else { "`n(cleanup skipped: -NoCleanup)" }
}

""
"Memo A4 asserts a trashed ref file does NOT still occupy its name."
"If this FAILS, deleting and recreating a branch breaks until the trash is emptied, and v2"
"cannot name storage files after refs without an indirection layer."
