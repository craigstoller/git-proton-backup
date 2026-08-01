<#
.SYNOPSIS
A7 - does CreateExclusive hold under GENUINELY CONCURRENT processes?

.DESCRIPTION
Stage 1 gate for docs/v2-remote-helper-design.md. This is the probe that can still invalidate
the design.

A6 established that `filesystem upload -f skip --json` refuses an existing name and that the
refusal is detectable (transferredItems=0, skippedItems=1). But A6 is SEQUENTIAL: writer A
finished long before writer B started. The v2 lock rests on something stronger - that when N
writers attempt the same new name at the same instant, the SERVER admits exactly one.

If two or more writers can both be told "transferred", the lock is decorative, ref updates are
unprotected, and single-writer becomes a purely social convention. That is the finding that
would send v2 back to waiting for the SDK.

## Method

  - N worker processes are spawned, each a separate pwsh running this script with -Worker.
  - Every worker is given the SAME absolute start timestamp and spins until it passes, so the
    CLI invocations fire together rather than in sequence.
  - Each worker attempts CreateExclusive on ONE shared name, with content identifying itself.
  - The orchestrator collects every worker's parsed counts and the surviving remote content.
  - Repeated over several rounds, each with a fresh name, because a race that fails rarely is
    still a race.

## Reading the result

  exactly 1 transferred, N-1 skipped, every round  -> CONSISTENT WITH ATOMICITY. Supportive,
                                                      not proof; absence of an observed race is
                                                      weaker evidence than an observed one.
  2 or more transferred in ANY round               -> NOT ATOMIC. Definitive negative. The v2
                                                      lock cannot be built on this.
  0 transferred, or failures                       -> ERROR. Nothing was tested.

The asymmetry is deliberate and worth stating: this experiment can DISPROVE atomicity outright,
but can only support it. The design should say so rather than claiming proof.

.NOTES
Writes only under /my-files/_cas-probe (enforced by probe-lib.ps1, no override).
Each round writes one tiny file; workers that lose write nothing.
#>
[CmdletBinding()]
param(
    [switch]$SkipSweepGate,
    [switch]$NoCleanup,
    [int]$Workers = 4,
    [int]$Rounds  = 3,

    # --- worker mode (internal) ---
    [switch]$Worker,
    [string]$WorkerName,
    [string]$RemoteDir,
    [string]$LocalFile,
    [string]$StartAtUtc
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'probe-lib.ps1')

# ---------------------------------------------------------------- worker ----
if ($Worker) {
    $start = [datetime]::Parse($StartAtUtc, $null, [System.Globalization.DateTimeStyles]::RoundtripKind)
    # Spin rather than sleep for the final approach: Start-Sleep granularity is ~15ms and we
    # want the CLI invocations as close together as the OS will allow.
    while ([datetime]::UtcNow -lt $start.AddMilliseconds(-50)) { Start-Sleep -Milliseconds 5 }
    while ([datetime]::UtcNow -lt $start) { }

    $fired = [datetime]::UtcNow
    $r = Invoke-ProbeCli -CliArgs @('filesystem','upload','-f','skip','--json',$LocalFile,$RemoteDir) -RemotePaths @($RemoteDir)
    $c = Get-UploadCounts -Result $r
    # single-line JSON on stdout so the orchestrator can parse it unambiguously
    [pscustomobject]@{
        Worker      = $WorkerName
        FiredAtUtc  = $fired.ToString('o')
        ExitCode    = $c.ExitCode
        Transferred = $c.Transferred
        Skipped     = $c.Skipped
        Failed      = $c.Failed
        Raw         = $c.Raw
    } | ConvertTo-Json -Compress
    return
}

# ---------------------------------------------------------- orchestrator ----
'=== A7 - concurrent create-exclusive (Stage 1 gate) ==='
Assert-FleetGreen -SkipSweepGate:$SkipSweepGate

$ver = Invoke-ProbeCli -CliArgs @('--version')
"CLI: $(($ver.Output -split "`n")[0].Trim())"
"workers per round: $Workers   rounds: $Rounds"

[void](Initialize-ProbeRoot)
$ws = New-ProbeWorkspace
$self = $PSCommandPath
$roundResults = [System.Collections.Generic.List[object]]@()

try {
    $sub = New-ProbeFolder -Parent $script:ProbeRoot -Name 'a7'

    for ($round = 1; $round -le $Rounds; $round++) {
        $name = "race-$round.lock"
        "`n--- round $round : $name ---"

        # each worker gets its own local file (same target name, distinct content)
        $jobs = @()
        $startAt = [datetime]::UtcNow.AddSeconds(6)   # generous: pwsh + module load per worker
        for ($w = 1; $w -le $Workers; $w++) {
            $wname = "W$w"
            $dir = Join-Path $ws "r$round-$wname"
            New-Item -ItemType Directory -Path $dir -Force | Out-Null
            $lf = Join-Path $dir $name
            [System.IO.File]::WriteAllText($lf, $wname)

            $jobs += Start-Job -ScriptBlock {
                param($script, $wname, $remoteDir, $localFile, $startAt)
                & pwsh -NoProfile -File $script -Worker -WorkerName $wname `
                    -RemoteDir $remoteDir -LocalFile $localFile -StartAtUtc $startAt 2>&1
            } -ArgumentList $self, $wname, $sub, $lf, $startAt.ToString('o')
        }

        $raw = $jobs | Wait-Job -Timeout 180 | Receive-Job
        $jobs | Remove-Job -Force -ErrorAction SilentlyContinue

        $parsed = @()
        foreach ($line in @($raw)) {
            $s = "$line".Trim()
            if ($s.StartsWith('{')) { try { $parsed += ($s | ConvertFrom-Json) } catch { } }
        }

        foreach ($p in ($parsed | Sort-Object FiredAtUtc)) {
            "  {0}  fired {1}  exit {2}  transferred={3} skipped={4} failed={5}" -f `
                $p.Worker, ([datetime]$p.FiredAtUtc).ToString('HH:mm:ss.fff'), $p.ExitCode, $p.Transferred, $p.Skipped, $p.Failed
        }

        $won  = @($parsed | Where-Object { $_.Transferred -eq 1 })
        $lost = @($parsed | Where-Object { $_.Skipped -eq 1 -and $_.Transferred -eq 0 })
        $bad  = @($parsed | Where-Object { $_.Failed -gt 0 })

        # what actually survived on the remote
        $dl = Join-Path $ws "r$round-readback"; New-Item -ItemType Directory -Path $dl -Force | Out-Null
        [void](Invoke-ProbeCli -CliArgs @('filesystem','download',"$sub/$name",$dl) -RemotePaths @("$sub/$name"))
        $survivor = $null
        $sf = Get-ChildItem $dl -File -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($sf) { $survivor = [System.IO.File]::ReadAllText($sf.FullName).Trim() }

        $spread = if ($parsed.Count -ge 2) {
            $times = $parsed | ForEach-Object { [datetime]$_.FiredAtUtc }
            [math]::Round((($times | Measure-Object -Maximum).Maximum - ($times | Measure-Object -Minimum).Minimum).TotalMilliseconds, 1)
        } else { $null }

        "  responded: $($parsed.Count)/$Workers   won: $($won.Count)   refused: $($lost.Count)   failed: $($bad.Count)"
        "  fire spread: ${spread}ms   surviving content: '$survivor'"

        $verdict =
            if ($parsed.Count -lt $Workers)      { 'ERROR - not every worker reported' }
            elseif ($bad.Count -gt 0)            { 'ERROR - a worker reported failedItems' }
            elseif ($won.Count -gt 1)            { 'NOT ATOMIC - MORE THAN ONE WRITER WON' }
            elseif ($won.Count -eq 0)            { 'ERROR - nobody won' }
            elseif ($won[0].Worker -ne $survivor){ "INCONSISTENT - winner $($won[0].Worker) but remote holds '$survivor'" }
            elseif ($lost.Count -ne ($Workers-1)){ 'ERROR - losers did not all report a clean skip' }
            else                                 { 'OK - exactly one winner, all others refused' }
        "  ROUND VERDICT: $verdict"

        $roundResults.Add([pscustomobject]@{
            Round=$round; Responded=$parsed.Count; Won=$won.Count; Refused=$lost.Count
            Failed=$bad.Count; SpreadMs=$spread; Survivor=$survivor; Verdict=$verdict })
    }
}
finally {
    Remove-Item $ws -Recurse -Force -ErrorAction SilentlyContinue
    if (-not $NoCleanup) { "`n"; Remove-ProbeTree } else { "`n(cleanup skipped: -NoCleanup)" }
}

"`n=== A7 SUMMARY ==="
$roundResults | Format-Table -AutoSize Round, Responded, Won, Refused, Failed, SpreadMs, Survivor, Verdict

$ok      = @($roundResults | Where-Object { $_.Verdict -like 'OK*' })
$notAtom = @($roundResults | Where-Object { $_.Verdict -like 'NOT ATOMIC*' })
''
if ($notAtom.Count) {
    'OVERALL: NOT ATOMIC. More than one writer was told it transferred.'
    'The v2 advisory lock cannot be built on this transport. Design must change.'
} elseif ($ok.Count -eq $roundResults.Count -and $roundResults.Count -gt 0) {
    "OVERALL: CONSISTENT WITH ATOMICITY across $($ok.Count) round(s)."
    'Supportive, NOT proof - no race was observed, which is weaker evidence than observing one.'
    'Record the round count, worker count and fire spread alongside this result.'
} else {
    'OVERALL: INCONCLUSIVE - see per-round verdicts above. Fix the harness before concluding.'
}
