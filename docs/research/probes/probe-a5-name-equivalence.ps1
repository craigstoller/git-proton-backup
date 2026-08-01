<#
.SYNOPSIS
A5 - name equivalence. Is Proton's server-side name uniqueness exact-byte on the plaintext?

.DESCRIPTION
Memo: docs/research/remote-helper-prior-art.md §3b, assumption A5.

WHY IT MATTERS. Proton enforces name uniqueness over *hashed* names. If the plaintext is
case-folded or Unicode-normalised before hashing, two genuinely distinct git refs collide on the
remote. git-remote-dropbox has exactly this bug (§1c): Dropbox paths are case-insensitive, so
refs/heads/Foo and refs/heads/foo are the same file, and the helper papers over it by lowercasing
the repo path. A git-remote-proton would have to escape ref names into the storage namespace
rather than using them directly - a design change, not a detail.

WHAT IS TESTED
  Case:    "Foo.txt" vs "foo.txt"
  Unicode: "café.txt" NFD (e + U+0301) vs NFC (U+00E9) - different bytes, identical rendering

METHOD. Each candidate uploads from its OWN local directory, because Windows' filesystem is
itself case-insensitive and would refuse to hold both side by side - the local FS must not be
allowed to answer a question about the remote. Upload uses -f skip so a name collision resolves
by skipping rather than replacing or auto-renaming. Evidence is the parsed --json listing.

READING THE RESULT
  2 nodes -> DISTINCT: exact-byte uniqueness. Ref names usable directly. A5 holds.
  1 node  -> COLLIDED: the remote folded them. Ref names need escaping. A5 fails.
  anything else, or a failed precondition -> ERROR, and no conclusion is drawn.

The ERROR path matters: the first run of this probe reported COLLIDED when in fact nothing had
been uploaded at all (create-folder was called with the wrong arity, so every call failed and a
text heuristic counted the error line as a directory entry). Preconditions are now asserted and
a failure is loud rather than silently interpreted.

.NOTES
Writes only under /my-files/_cas-probe (enforced by probe-lib.ps1, no override).
#>
[CmdletBinding()]
param([switch]$SkipSweepGate, [switch]$NoCleanup)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'probe-lib.ps1')

'=== A5 - name equivalence ==='
Assert-FleetGreen -SkipSweepGate:$SkipSweepGate
[void](Initialize-ProbeRoot)
"probe root ready: $script:ProbeRoot"

$ws = New-ProbeWorkspace
$results = [System.Collections.Generic.List[object]]@()

function Invoke-NamePair {
    param([string]$Label, [string]$NameA, [string]$NameB)

    "`n--- $Label ---"
    "  A = $NameA"
    "      bytes: $(([System.Text.Encoding]::UTF8.GetBytes($NameA) | ForEach-Object { $_.ToString('x2') }) -join ' ')"
    "  B = $NameB"
    "      bytes: $(([System.Text.Encoding]::UTF8.GetBytes($NameB) | ForEach-Object { $_.ToString('x2') }) -join ' ')"

    $dirA = Join-Path $ws "$Label-a"; $dirB = Join-Path $ws "$Label-b"
    New-Item -ItemType Directory -Path $dirA, $dirB -Force | Out-Null
    [System.IO.File]::WriteAllText((Join-Path $dirA $NameA), 'AAAA')
    [System.IO.File]::WriteAllText((Join-Path $dirB $NameB), 'BBBBBBBB')

    $sub = New-ProbeFolder -Parent $script:ProbeRoot -Name "a5-$Label"

    # Both uploads run with --json so their outcomes are DATA, not guesses.
    # (Round-3 review catch: an earlier version validated only A. If B failed for any reason -
    #  network, auth, bad path - the folder would hold one node and the probe would report
    #  COLLIDED, a design-sinking result, from a call that never reached the server. Same
    #  defect as the original A6. B's counts are now asserted.)
    $u1 = Invoke-ProbeCli -CliArgs @('filesystem','upload','-f','skip','--json',(Join-Path $dirA $NameA),$sub) -RemotePaths @($sub)
    $c1 = Get-UploadCounts -Result $u1
    "  upload A -> exit $($u1.ExitCode) transferred=$($c1.Transferred) skipped=$($c1.Skipped) failed=$($c1.Failed)"

    $u2 = Invoke-ProbeCli -CliArgs @('filesystem','upload','-f','skip','--json',(Join-Path $dirB $NameB),$sub) -RemotePaths @($sub)
    $c2 = Get-UploadCounts -Result $u2
    "  upload B -> exit $($u2.ExitCode) transferred=$($c2.Transferred) skipped=$($c2.Skipped) failed=$($c2.Failed)"

    # Preconditions: A must have landed, and B must have either landed (distinct names) or
    # been skipped (collision). B FAILING means the experiment did not run.
    $bad = @()
    if ($null -eq $c1.Transferred -or $null -eq $c2.Transferred) { $bad += 'could not parse --json counts' }
    else {
        if ($c1.Transferred -lt 1) { $bad += "upload A did not transfer (transferred=$($c1.Transferred))" }
        if ($c1.Failed -gt 0)      { $bad += "upload A reported $($c1.Failed) failure(s)" }
        if ($c2.Failed -gt 0)      { $bad += "upload B reported $($c2.Failed) failure(s) - B errored rather than landing or being skipped" }
        if ($c2.Transferred -lt 1 -and $c2.Skipped -lt 1) { $bad += 'upload B neither transferred nor was skipped - outcome unknown' }
    }
    if ($bad.Count) {
        "  VERDICT: ERROR - preconditions not met, NO conclusion drawn:"
        $bad | ForEach-Object { "    - $_" }
        $results.Add([pscustomobject]@{ Case=$Label; NameA=$NameA; NameB=$NameB
            UploadAExit=$u1.ExitCode; UploadBExit=$u2.ExitCode; Nodes=$null
            NamesSeen=''; Verdict='ERROR - preconditions not met' })
        return
    }

    $nodes = Get-ProbeNodes -Path $sub
    $names = @($nodes | ForEach-Object { $_.name.value })
    "  nodes present: $($nodes.Count)"
    foreach ($n in $nodes) {
        "    name='$($n.name.value)'  uid=$($n.uid)"
        "      bytes: $(([System.Text.Encoding]::UTF8.GetBytes([string]$n.name.value) | ForEach-Object { $_.ToString('x2') }) -join ' ')"
    }

    $verdict = switch ($nodes.Count) {
        2       { 'DISTINCT - exact-byte uniqueness (ref names usable directly)' }
        1       { 'COLLIDED - remote folded the two names (ref names need escaping)' }
        0       { 'ERROR - no nodes after a successful upload; experiment invalid' }
        default { "ERROR - unexpected node count $($nodes.Count); experiment invalid" }
    }
    "  VERDICT: $verdict"

    $results.Add([pscustomobject]@{
        Case = $Label; NameA = $NameA; NameB = $NameB
        UploadAExit = $u1.ExitCode; UploadBExit = $u2.ExitCode
        Nodes = $nodes.Count; NamesSeen = ($names -join ' | '); Verdict = $verdict
    })
}

try {
    Invoke-NamePair -Label 'case'    -NameA 'Foo.txt' -NameB 'foo.txt'

    $nfd = "cafe" + [char]0x0301 + ".txt"
    $nfc = "caf"  + [char]0x00E9 + ".txt"
    Invoke-NamePair -Label 'unicode' -NameA $nfd -NameB $nfc
}
finally {
    Remove-Item $ws -Recurse -Force -ErrorAction SilentlyContinue
    if (-not $NoCleanup) { "`n"; Remove-ProbeTree } else { "`n(cleanup skipped: -NoCleanup)" }
}

"`n=== A5 SUMMARY ==="
$results | Select-Object Case, UploadAExit, UploadBExit, Nodes, Verdict | Format-Table -AutoSize
$results | ForEach-Object { "  $($_.Case): names seen -> $($_.NamesSeen)" }
""
"Memo A5 asserts exact-byte uniqueness with no case folding or Unicode normalisation."
"DISTINCT on both cases confirms A5. Any COLLIDED means §3b A5 fails and v2 must escape ref names."
"Any ERROR means the experiment did not run - fix it before drawing any conclusion."
