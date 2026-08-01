<#
Shared guardrails for the A1-A5 verification probes.
Memo: docs/research/remote-helper-prior-art.md (§3b, "The load-bearing assumptions").

WHY THIS FILE EXISTS. The memo says these tests should run against a throwaway Proton
account. The author decided to run them against his REAL account instead, accepting the risk
on condition of hard guardrails. This file IS those guardrails, and they are mechanical rather
than procedural on purpose:

  1. Every remote path is asserted to sit under /my-files/_cas-probe before any CLI call.
     A path outside it throws. There is no override switch.
  2. No probe takes a remote path as a parameter. Names are constants in the scripts.
     A path that cannot be passed in cannot be passed in wrongly.
  3. Volumes are trivial (bytes, not megabytes) and every probe cleans up after itself.
  4. Probes refuse to run unless the backup fleet is in a known-good state.

FAIL-LOUD DISCIPLINE (added after the first A5 run, 2026-07-31). That run reported
"COLLIDED - remote folded the two names", a design-sinking result, when in fact nothing had
been tested: create-folder had been called with one argument instead of two, the probe root
was never created, every call returned "Node not found", and a text-parsing heuristic counted
that single error line as one directory entry. A probe that cannot tell "the remote did X"
from "my call was malformed" is worse than no probe. Therefore:

  - Every CLI call goes through Invoke-ProbeCli, and anything that must succeed goes through
    Assert-CliOk, which throws on a non-zero exit.
  - Listings are parsed from --json into real objects. No text heuristics, ever.
  - Verdict logic in each probe must establish its preconditions actually held before
    interpreting a result, and must emit ERROR (not a substantive verdict) otherwise.

Dot-source this; do not run it directly.
#>

Set-StrictMode -Version Latest

# --- constants, not parameters ---------------------------------------------------------
$script:ProbeRootParent = '/my-files'
$script:ProbeRootName   = '_cas-probe'
$script:ProbeRoot       = "$script:ProbeRootParent/$script:ProbeRootName"
$script:Cli             = 'proton-drive'

function Assert-ProbePath {
    <#  The single choke point. Every remote path used by any probe passes through here. #>
    [CmdletBinding()]
    param([Parameter(Mandatory)][string]$Path)

    if ([string]::IsNullOrWhiteSpace($Path)) { throw 'GUARDRAIL: empty remote path.' }
    if ($Path -ne $script:ProbeRoot -and -not $Path.StartsWith("$script:ProbeRoot/", [StringComparison]::Ordinal)) {
        throw "GUARDRAIL VIOLATION: refusing to touch '$Path' - outside $script:ProbeRoot. No override exists."
    }
    if ($Path -match '\.\.') { throw "GUARDRAIL VIOLATION: '$Path' contains '..'" }
    $Path
}

function Invoke-ProbeCli {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string[]]$CliArgs,
        [string[]]$RemotePaths = @()
    )
    foreach ($p in $RemotePaths) { [void](Assert-ProbePath -Path $p) }
    $out = & $script:Cli @CliArgs 2>&1 | Out-String
    [pscustomobject]@{
        Args     = ($CliArgs -join ' ')
        ExitCode = $LASTEXITCODE
        Output   = $out.TrimEnd()
    }
}

function Assert-CliOk {
    <#  Use for any call whose failure invalidates the experiment. #>
    [CmdletBinding()]
    param([Parameter(Mandatory)][object]$Result, [Parameter(Mandatory)][string]$What)
    if ($Result.ExitCode -ne 0) {
        throw "PROBE PRECONDITION FAILED - $What`n  cmd : $($Result.Args)`n  exit: $($Result.ExitCode)`n  out : $($Result.Output)"
    }
    $Result
}

function Get-UploadCounts {
    <#  Parse a `filesystem upload --json` transfer summary into asserted counts.

        Shape observed on cli-drive@0.4.6 (probe A6, 2026-07-31):
          {"transferredItems":1,"transferredBytes":8,"skippedItems":0,"failedItems":0,"failures":[]}

        Returns $null for a count that is ABSENT rather than defaulting it to 0 - a missing
        field must fail an assertion, not silently read as "nothing happened". Exit code is
        deliberately not interpreted here: A6 established that success and refusal both exit 0.
    #>
    [CmdletBinding()]
    param([Parameter(Mandatory)][object]$Result)
    $parsed = $null
    try { $parsed = $Result.Output | ConvertFrom-Json } catch { }
    $get = {
        param($obj, $name)
        if ($obj -and $obj.PSObject.Properties[$name]) { [int]$obj.$name } else { $null }
    }
    [pscustomobject]@{
        Transferred = & $get $parsed 'transferredItems'
        Skipped     = & $get $parsed 'skippedItems'
        Failed      = & $get $parsed 'failedItems'
        Parsed      = $parsed
        Raw         = $Result.Output
        ExitCode    = $Result.ExitCode
    }
}

function Test-FleetGreen {
    [CmdletBinding()]
    param()
    $lv = Join-Path $env:LOCALAPPDATA 'GitProtonBackup\last-verify.json'
    if (-not (Test-Path $lv)) {
        return [pscustomobject]@{ Green=$false; Reason='no last-verify.json - engine has never verified'; Attention=$null; Total=$null; AsOf=$null }
    }
    $j = Get-Content $lv -Raw | ConvertFrom-Json
    $att = @($j.Repos | Where-Object State -ne 'ok')
    [pscustomobject]@{
        Green     = ($att.Count -eq 0)
        Reason    = if ($att.Count) { "$($att.Count) repo(s) in attention: $(($att | ForEach-Object { Split-Path $_.RepoPath -Leaf }) -join ', ')" } else { 'all repos ok' }
        Attention = $att.Count
        Total     = @($j.Repos).Count
        AsOf      = (Get-Item $lv).LastWriteTime
    }
}

function Assert-FleetGreen {
    [CmdletBinding()]
    param([switch]$SkipSweepGate)
    $v = Test-FleetGreen
    "fleet: $($v.Attention) of $($v.Total) in attention as of $($v.AsOf) - $($v.Reason)"
    if ($v.Green) { return }
    if ($SkipSweepGate) {
        Write-Warning "Sweep gate BYPASSED (-SkipSweepGate). Only legitimate when the attention item has been individually confirmed benign."
        return
    }
    throw "GUARDRAIL: fleet is not green ($($v.Reason)). Diagnose it, or re-run with -SkipSweepGate only if you have confirmed the attention item is benign."
}

function New-ProbeWorkspace {
    [CmdletBinding()]
    param()
    $d = Join-Path ([System.IO.Path]::GetTempPath()) ("cas-probe-" + [guid]::NewGuid().ToString('N').Substring(0,8))
    New-Item -ItemType Directory -Path $d -Force | Out-Null
    $d
}

function Get-ProbeNodes {
    <#  Parsed listing. Returns @() for an empty folder, throws if the listing itself failed.
        JSON only - the text table is not a contract. #>
    [CmdletBinding()]
    param([string]$Path = $script:ProbeRoot)
    [void](Assert-ProbePath -Path $Path)
    $r = Invoke-ProbeCli -CliArgs @('filesystem','list',$Path,'--json') -RemotePaths @($Path)
    if ($r.ExitCode -ne 0) {
        throw "PROBE: listing '$Path' failed (exit $($r.ExitCode)): $($r.Output)"
    }
    # Return with the unary comma so an EMPTY result stays an array. `return @()` unrolls to
    # zero pipeline objects, so the caller's variable becomes $null and `.Count` throws under
    # StrictMode - which is exactly how the first A4 run died on an empty folder. (Same
    # PowerShell gotcha that produced the DelegatedSet JSON-null bug in the private sweep.)
    if ([string]::IsNullOrWhiteSpace($r.Output)) { return ,@() }
    try { return ,@($r.Output | ConvertFrom-Json) }
    catch { throw "PROBE: listing '$Path' returned unparseable JSON: $($r.Output)" }
}

function Test-ProbeNodeExists {
    [CmdletBinding()]
    param([Parameter(Mandatory)][string]$Path)
    [void](Assert-ProbePath -Path $Path)
    $r = Invoke-ProbeCli -CliArgs @('filesystem','info',$Path,'--json') -RemotePaths @($Path)
    ($r.ExitCode -eq 0)
}

function New-ProbeFolder {
    <#  create-folder takes PARENT and NAME as two arguments, not one path.
        Getting this wrong is what invalidated the first A5 run. #>
    [CmdletBinding()]
    param([Parameter(Mandatory)][string]$Parent, [Parameter(Mandatory)][string]$Name)
    $full = "$Parent/$Name"
    [void](Assert-ProbePath -Path $full)
    $r = Invoke-ProbeCli -CliArgs @('filesystem','create-folder',$Parent,$Name)
    if ($r.ExitCode -ne 0 -and $r.Output -notmatch 'exist') {
        throw "PROBE: create-folder '$Parent' '$Name' failed (exit $($r.ExitCode)): $($r.Output)"
    }
    if (-not (Test-ProbeNodeExists -Path $full)) {
        throw "PROBE: created '$full' but it does not read back. Refusing to continue."
    }
    $full
}

function Initialize-ProbeRoot {
    <#  Ensure /my-files/_cas-probe exists, and PROVE it before any probe proceeds. #>
    [CmdletBinding()]
    param()
    if (Test-ProbeNodeExists -Path $script:ProbeRoot) { return $script:ProbeRoot }
    # parent is /my-files, which is deliberately outside the assertion (we create INTO it,
    # never modify it), so call the CLI directly here rather than through the path guard.
    $r = Invoke-ProbeCli -CliArgs @('filesystem','create-folder',$script:ProbeRootParent,$script:ProbeRootName)
    if ($r.ExitCode -ne 0 -and $r.Output -notmatch 'exist') {
        throw "PROBE: could not create $script:ProbeRoot (exit $($r.ExitCode)): $($r.Output)"
    }
    if (-not (Test-ProbeNodeExists -Path $script:ProbeRoot)) {
        throw "PROBE: $script:ProbeRoot does not exist after create-folder. Refusing to continue."
    }
    $script:ProbeRoot
}

function Remove-ProbeTree {
    [CmdletBinding()]
    param()
    if (-not (Test-ProbeNodeExists -Path $script:ProbeRoot)) { 'cleanup: nothing to remove'; return }
    $r = Invoke-ProbeCli -CliArgs @('filesystem','trash',$script:ProbeRoot) -RemotePaths @($script:ProbeRoot)
    "cleanup: trash $script:ProbeRoot -> exit $($r.ExitCode)"
    if ($r.Output) { "  $($r.Output)" }
    'NOTE: trashed, not permanently deleted. Empty the Proton trash by hand if you want the bytes gone.'
}
