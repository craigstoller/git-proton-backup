<#
.SYNOPSIS
A6 - is `filesystem upload -f skip` an atomic create-exclusive usable as a mutex?

.DESCRIPTION
Raised by peer review of docs/v2-remote-helper-design.md (2026-07-31). The v2 design needs a
create-exclusive primitive: a write that succeeds only if the name does not already exist, with
the refusal DETECTABLE by the loser. If the CLI cannot express that, the lock is decorative.

The counter-hypothesis to "this is a check-then-act race": uniqueness is enforced SERVER-side,
not by any client check (prior-art memo §3b item 1 - creating a node whose name exists is
rejected with code 2500). If the CLI's `skip` strategy surfaces that refusal distinguishably,
then "attempt the write, read the outcome" is a correct mutual-exclusion primitive.

## v2 of this probe - why it was rewritten

The first version reported "CREATE-EXCLUSIVE WORKS" based ONLY on the final file contents. It
never asserted that writer B actually ran and was actually skipped. Any failure of B - bad
path, network, auth, malformed arguments - would leave writer A's content in place and produce
exactly the same "works" verdict. That is the same defect as the false-COLLIDED A5 run this
project already documented: a probe that cannot distinguish "the remote refused" from "my call
failed" is not evidence.

This version therefore:
  - runs BOTH uploads with --json and records the raw summaries
  - asserts writer A was actually transferred (transferred >= 1)
  - asserts writer B was actually SKIPPED (skipped >= 1 AND transferred == 0)
  - asserts no failures were reported for either
  - verifies the surviving content is A's, by exact bytes
  - reports the CLI version, so the result is pinned to a build

Any of those failing yields ERROR, never a substantive verdict.

## What this still does NOT prove

This is SEQUENTIAL. It shows the CLI refuses an existing name and that the refusal is
detectable. It does NOT prove atomicity under two genuinely simultaneous processes - that
needs a barrier-synchronised concurrent run, and is Stage 1 work in the v2 design.

.NOTES
Writes only under /my-files/_cas-probe (enforced by probe-lib.ps1, no override).
#>
[CmdletBinding()]
param([switch]$SkipSweepGate, [switch]$NoCleanup)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'probe-lib.ps1')

'=== A6 - create-exclusive via the CLI (v2: asserts its own preconditions) ==='
Assert-FleetGreen -SkipSweepGate:$SkipSweepGate

$ver = Invoke-ProbeCli -CliArgs @('--version')
"CLI version: $($ver.Output.Trim())   (results are pinned to this build)"

[void](Initialize-ProbeRoot)

$ws   = New-ProbeWorkspace
$name = 'repo.lock'

function Get-TransferCounts {
    <#  Parse the --json transfer summary. Returns nulls if the shape is not what we expect,
        so the caller can fail loudly rather than silently reading 0 for everything. #>
    param([Parameter(Mandatory)][object]$Result)
    $t = $null; $s = $null; $f = $null; $parsed = $null
    try { $parsed = $Result.Output | ConvertFrom-Json } catch { }
    if ($parsed) {
        foreach ($n in 'transferredItems','transferred','uploadedItems') { if ($null -eq $t -and $parsed.PSObject.Properties[$n]) { $t = [int]$parsed.$n } }
        foreach ($n in 'skippedItems','skipped')                          { if ($null -eq $s -and $parsed.PSObject.Properties[$n]) { $s = [int]$parsed.$n } }
        foreach ($n in 'failedItems','failed')                            { if ($null -eq $f -and $parsed.PSObject.Properties[$n]) { $f = [int]$parsed.$n } }
    }
    [pscustomobject]@{ Transferred=$t; Skipped=$s; Failed=$f; Parsed=$parsed; Raw=$Result.Output; ExitCode=$Result.ExitCode }
}

try {
    $sub    = New-ProbeFolder -Parent $script:ProbeRoot -Name 'a6'
    $remote = "$sub/$name"

    $dirA = Join-Path $ws 'A'; New-Item -ItemType Directory -Path $dirA -Force | Out-Null
    $dirB = Join-Path $ws 'B'; New-Item -ItemType Directory -Path $dirB -Force | Out-Null
    [System.IO.File]::WriteAllText((Join-Path $dirA $name), 'WRITER-A')
    [System.IO.File]::WriteAllText((Join-Path $dirB $name), 'WRITER-B-MUST-NOT-WIN')

    # --- writer A: must genuinely transfer -------------------------------------------
    $rA = Invoke-ProbeCli -CliArgs @('filesystem','upload','-f','skip','--json',(Join-Path $dirA $name),$sub) -RemotePaths @($sub)
    $a  = Get-TransferCounts -Result $rA
    "1. writer A -> exit $($a.ExitCode)  transferred=$($a.Transferred) skipped=$($a.Skipped) failed=$($a.Failed)"
    "   raw: $($a.Raw)"

    # --- writer B: must genuinely be SKIPPED ------------------------------------------
    $rB = Invoke-ProbeCli -CliArgs @('filesystem','upload','-f','skip','--json',(Join-Path $dirB $name),$sub) -RemotePaths @($sub)
    $b  = Get-TransferCounts -Result $rB
    "2. writer B -> exit $($b.ExitCode)  transferred=$($b.Transferred) skipped=$($b.Skipped) failed=$($b.Failed)"
    "   raw: $($b.Raw)"

    # --- surviving content -------------------------------------------------------------
    $dl = Join-Path $ws 'readback'; New-Item -ItemType Directory -Path $dl -Force | Out-Null
    $rD = Invoke-ProbeCli -CliArgs @('filesystem','download',$remote,$dl) -RemotePaths @($remote)
    $got = $null
    $file = Get-ChildItem $dl -File -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($file) { $got = [System.IO.File]::ReadAllText($file.FullName) }
    "3. surviving content: '$got'"

    # --- verdict: every precondition must hold, else ERROR -----------------------------
    $problems = @()
    if ($null -eq $a.Transferred -or $null -eq $b.Skipped) {
        $problems += 'could not parse transfer counts from --json (shape changed?) - see raw output above'
    } else {
        if ($a.Transferred -lt 1)          { $problems += "writer A did not transfer (transferred=$($a.Transferred)) - setup failed, nothing was tested" }
        if ($a.Failed -gt 0)               { $problems += "writer A reported failures ($($a.Failed))" }
        if ($b.Failed -gt 0)               { $problems += "writer B reported failures ($($b.Failed)) - B errored rather than being refused" }
        if ($b.Transferred -ne 0)          { $problems += "writer B TRANSFERRED ($($b.Transferred)) - the name was NOT protected" }
        if ($b.Skipped -lt 1)              { $problems += "writer B was not skipped (skipped=$($b.Skipped)) - refusal not observed" }
    }
    if ($got -ne 'WRITER-A')               { $problems += "surviving content is '$got', expected 'WRITER-A'" }

    ''
    if ($problems.Count) {
        'VERDICT: ERROR - preconditions not met, NO conclusion drawn:'
        $problems | ForEach-Object { "  - $_" }
    } else {
        'VERDICT: CREATE-EXCLUSIVE CONFIRMED (sequential)'
        "  writer A transferred=1, writer B skipped=1 transferred=0, content intact."
        "  Detection MUST read these counts: both invocations exited $($a.ExitCode)/$($b.ExitCode)."
    }
    ''
    'SCOPE: sequential only. Atomicity under two simultaneous processes is NOT established here.'
}
finally {
    Remove-Item $ws -Recurse -Force -ErrorAction SilentlyContinue
    if (-not $NoCleanup) { "`n"; Remove-ProbeTree } else { "`n(cleanup skipped: -NoCleanup)" }
}
