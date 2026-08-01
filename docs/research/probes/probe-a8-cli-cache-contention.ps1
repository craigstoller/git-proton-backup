<#
.SYNOPSIS
A8 - is the CLI's SQLite cache lock transient or a hard serialisation limit?

.DESCRIPTION
Stage 1 follow-up to the A7 blocker (docs/v2-remote-helper-design.md). On CLI 0.7.0 a second
concurrent `proton-drive` process dies at startup with:

    SQLiteError: database is locked   errno: 261   SQLITE_BUSY_RECOVERY
      at new SQLiteCache (src/cache/sqliteCache.ts:12)  at LO (src/init.ts:49)

That is the local encrypted session cache, not a Proton response - the loser never reaches the
network. The design's answer differs sharply depending on which this is:

  TRANSIENT  -> the helper retries with backoff. Inconvenient, survivable, and v1/v2 can
                coexist on one machine as long as both retry.
  HARD       -> every CLI invocation on the machine must be serialised behind a named mutex,
                and a v1 sweep running concurrently with a v2 push is simply unsupported.

## What is measured

  1. A holder process starts a real CLI operation and keeps the cache busy.
  2. A probe process repeatedly attempts a CHEAP read-only call (`filesystem info`), recording
     every attempt: timestamp, exit code, and whether the failure was the SQLite signature.
  3. Reported: did the probe ever succeed while the holder ran; how many attempts it took; how
     long the lock persisted; and whether failures were all SQLITE_BUSY or something else.

A cheap read is used deliberately - we are timing CACHE contention, not upload duration.

## Reading the result

  probe succeeds while holder is still running        -> lock is brief (startup only)
  probe succeeds only after holder exits              -> lock spans the whole invocation
  probe succeeds on retry, consistently               -> TRANSIENT, retry is a valid strategy
  probe never succeeds until the holder is gone       -> HARD for the holder's lifetime

.NOTES
Read-only against Proton apart from one small upload by the holder, under /my-files/_cas-probe.
#>
[CmdletBinding()]
param([switch]$SkipSweepGate, [switch]$NoCleanup, [int]$MaxAttempts = 40, [int]$DelayMs = 250)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'probe-lib.ps1')

'=== A8 - CLI cache contention: transient or hard? ==='
Assert-FleetGreen -SkipSweepGate:$SkipSweepGate
$ver = Invoke-ProbeCli -CliArgs @('--version')
"CLI: $(($ver.Output -split "`n")[0].Trim())"

[void](Initialize-ProbeRoot)
$ws = New-ProbeWorkspace

try {
    $sub = New-ProbeFolder -Parent $script:ProbeRoot -Name 'a8'

    # a payload big enough that the holder runs for a few seconds
    $big = Join-Path $ws 'holder-payload.bin'
    $bytes = New-Object byte[] (6MB)
    (New-Object Random 42).NextBytes($bytes)
    [System.IO.File]::WriteAllBytes($big, $bytes)
    "holder payload: $([math]::Round((Get-Item $big).Length/1MB,1)) MB"

    # --- start the holder ---------------------------------------------------------
    $holder = Start-Job -ScriptBlock {
        param($lib,$file,$dest)
        . $lib
        $t0 = [datetime]::UtcNow
        $r = Invoke-ProbeCli -CliArgs @('filesystem','upload','-f','replace','--json',$file,$dest) -RemotePaths @($dest)
        [pscustomobject]@{ Start=$t0.ToString('o'); End=([datetime]::UtcNow).ToString('o'); Exit=$r.ExitCode; Out=$r.Output } | ConvertTo-Json -Compress
    } -ArgumentList (Join-Path $PSScriptRoot 'probe-lib.ps1'), $big, $sub

    # Probe as soon as the holder has plausibly opened its cache. The first version waited
    # 400 ms plus job-startup overhead, so the first attempt landed ~3.1 s in - by which time a
    # holder that died at 0.7 s was long gone, and the probe measured NO contention while
    # reporting "TRANSIENT". Short wait, and the holder's own success is asserted below.
    Start-Sleep -Milliseconds 150

    # --- hammer with cheap reads --------------------------------------------------
    $attempts = [System.Collections.Generic.List[object]]@()
    $t0 = [datetime]::UtcNow
    for ($i = 1; $i -le $MaxAttempts; $i++) {
        $holderDone = ($holder.State -ne 'Running')
        $r = Invoke-ProbeCli -CliArgs @('filesystem','info',$script:ProbeRoot,'--json') -RemotePaths @($script:ProbeRoot)
        $isBusy = $r.Output -match 'SQLITE_BUSY|database is locked'
        $attempts.Add([pscustomobject]@{
            N=$i; MsSinceStart=[math]::Round(([datetime]::UtcNow - $t0).TotalMilliseconds)
            Exit=$r.ExitCode; Busy=$isBusy; HolderDone=$holderDone
            Other = ($r.ExitCode -ne 0 -and -not $isBusy)
        })
        if ($r.ExitCode -eq 0) { break }
        Start-Sleep -Milliseconds $DelayMs
    }

    $h = $holder | Wait-Job -Timeout 300 | Receive-Job
    $holder | Remove-Job -Force -ErrorAction SilentlyContinue
    $hj = try { $h | ConvertFrom-Json } catch { $null }

    ''
    '=== attempts ==='
    $attempts | Format-Table -AutoSize N, MsSinceStart, Exit, Busy, HolderDone, Other

    $first = $attempts | Where-Object { $_.Exit -eq 0 } | Select-Object -First 1
    $busyCount = @($attempts | Where-Object Busy).Count
    $otherFail = @($attempts | Where-Object Other).Count

    $holderSecs = if ($hj) { [math]::Round(([datetime]$hj.End - [datetime]$hj.Start).TotalSeconds,1) } else { $null }
    "holder: exit $($hj.Exit)   ran ${holderSecs}s"
    "attempts: $($attempts.Count)   SQLITE_BUSY failures: $busyCount   other failures: $otherFail"

    # --- PRECONDITIONS: without these the verdict is meaningless -------------------
    $void = @()
    if (-not $hj)                    { $void += 'holder produced no result' }
    elseif ($hj.Exit -ne 0)          { $void += "holder FAILED (exit $($hj.Exit)) - it never held the cache, so there was nothing to contend with" }
    $probedWhileHolding = @($attempts | Where-Object { -not $_.HolderDone }).Count
    if ($probedWhileHolding -eq 0)   { $void += 'every attempt happened after the holder had already exited - no contention was sampled' }

    if ($void.Count) {
        ''
        'VERDICT: ERROR - experiment void, NO conclusion drawn:'
        $void | ForEach-Object { "  - $_" }
        if ($hj -and $hj.Exit -ne 0) {
            ''
            '  holder output:'
            (("$($hj.Out)" -split "`n") | Select-Object -First 12) | ForEach-Object { "    $_" }
        }
        return
    }

    ''
    if (-not $first) {
        'VERDICT: probe NEVER succeeded within the attempt budget.'
        'Treat as HARD for the holder lifetime - serialise CLI invocations machine-wide.'
    } elseif ($first.HolderDone) {
        "VERDICT: HARD FOR THE HOLDER'S LIFETIME."
        "  First success at attempt $($first.N) ($($first.MsSinceStart) ms), and only AFTER the holder exited."
        '  Retry alone does not help while another CLI process is running.'
        '  => v2 must serialise every CLI invocation machine-wide (named mutex on the cache path).'
        '  => a v1 sweep and a v2 push cannot overlap on one machine.'
    } else {
        "VERDICT: TRANSIENT."
        "  First success at attempt $($first.N) ($($first.MsSinceStart) ms) while the holder was STILL RUNNING."
        '  The lock covers CLI startup only, so retry-with-backoff is a valid strategy.'
    }
}
finally {
    Remove-Item $ws -Recurse -Force -ErrorAction SilentlyContinue
    if (-not $NoCleanup) { "`n"; Remove-ProbeTree } else { "`n(cleanup skipped: -NoCleanup)" }
}
