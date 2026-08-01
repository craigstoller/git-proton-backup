<#
.SYNOPSIS
A6 - is `filesystem upload -f skip` an atomic create-exclusive usable as a mutex?

.DESCRIPTION
Raised by peer review of docs/v2-remote-helper-design.md (2026-07-31). The v2 design uses an
advisory .lock file to enforce one writer per repo. A reviewer objected that writing that lock
through the CLI is a time-of-check-to-time-of-use race: two machines both list, both see no
lock, both upload, both proceed - reintroducing exactly the silent-clobber hazard the design
claims to avoid. If true, the reviewer's conclusion follows: the CLI cannot carry the lock and
the design needs the SDK.

The counter-hypothesis: the race may not exist, because uniqueness is enforced SERVER-side, not
by the client's check. The prior-art memo (§3b item 1) records that Proton rejects creating a
file whose name already exists with code 2500 - "atomic create-exclusive", equivalent to
Dropbox's WriteMode.add. If the CLI's `skip` conflict strategy surfaces that refusal, then
"upload my lock, read it back, check whose content is there" is a correct mutual-exclusion
primitive, and no client-side check is involved at all.

WHAT IS TESTED
  1. upload lock-A (content identifying writer A)
  2. upload lock-B - SAME name, different content, -f skip
  3. read the file back

READING THE RESULT
  content is still A -> the second write was refused. Create-exclusive works through the CLI,
                        the loser can detect it by reading back, and the lock is sound.
  content is now B   -> skip did not protect the file. The reviewer is right: no mutex via CLI.
  file missing/error -> ERROR, no conclusion.

NOTE ON WHAT THIS DOES AND DOES NOT PROVE. This is a SEQUENTIAL test: it shows the CLI refuses
to overwrite an existing name and that the refusal is detectable. It does NOT prove atomicity
under genuine simultaneous contention - two uploads racing in the same instant. That would need
two clients firing together, and is the same class of claim as A1-A3. Sequential refusal is
necessary but not sufficient; it is reported as such.

.NOTES
Writes only under /my-files/_cas-probe (enforced by probe-lib.ps1, no override).
#>
[CmdletBinding()]
param([switch]$SkipSweepGate, [switch]$NoCleanup)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'probe-lib.ps1')

'=== A6 - create-exclusive via the CLI ==='
Assert-FleetGreen -SkipSweepGate:$SkipSweepGate
[void](Initialize-ProbeRoot)

$ws   = New-ProbeWorkspace
$name = 'repo.lock'

try {
    $sub    = New-ProbeFolder -Parent $script:ProbeRoot -Name 'a6'
    $remote = "$sub/$name"

    $dirA = Join-Path $ws 'A'; New-Item -ItemType Directory -Path $dirA -Force | Out-Null
    $dirB = Join-Path $ws 'B'; New-Item -ItemType Directory -Path $dirB -Force | Out-Null
    [System.IO.File]::WriteAllText((Join-Path $dirA $name), 'WRITER-A')
    [System.IO.File]::WriteAllText((Join-Path $dirB $name), 'WRITER-B-SHOULD-NOT-WIN')

    $u1 = Invoke-ProbeCli -CliArgs @('filesystem','upload','-f','skip',(Join-Path $dirA $name),$sub) -RemotePaths @($sub)
    "1. writer A takes the lock -> exit $($u1.ExitCode)"; if ($u1.Output) { "   $($u1.Output)" }
    [void](Assert-CliOk -Result $u1 -What 'writer A creating the lock')

    $u2 = Invoke-ProbeCli -CliArgs @('filesystem','upload','-f','skip',(Join-Path $dirB $name),$sub) -RemotePaths @($sub)
    "2. writer B attempts the same name -> exit $($u2.ExitCode)"; if ($u2.Output) { "   $($u2.Output)" }

    # read back: download to a local file and inspect the bytes
    $dl = Join-Path $ws 'readback'; New-Item -ItemType Directory -Path $dl -Force | Out-Null
    $d = Invoke-ProbeCli -CliArgs @('filesystem','download',$remote,$dl) -RemotePaths @($remote)
    "3. read back -> exit $($d.ExitCode)"; if ($d.Output) { "   $($d.Output)" }

    $got = $null
    $f = Get-ChildItem $dl -File -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($f) { $got = [System.IO.File]::ReadAllText($f.FullName) }
    "   content on the remote: '$got'"

    $verdict =
        if ($null -eq $got)            { 'ERROR - could not read the lock back; no conclusion' }
        elseif ($got -eq 'WRITER-A')   { 'CREATE-EXCLUSIVE WORKS (sequential) - the second write was refused and the loser can detect it by reading back' }
        elseif ($got -like 'WRITER-B*'){ 'NO MUTEX - skip did not protect the existing file; the reviewer is right' }
        else                           { "ERROR - unexpected content '$got'" }

    "`nVERDICT: $verdict"
    "  writer B exit code was $($u2.ExitCode) - note whether a refusal is distinguishable from success by exit code alone"
    "`nSCOPE: sequential refusal only. This does NOT prove atomicity under simultaneous contention."
}
finally {
    Remove-Item $ws -Recurse -Force -ErrorAction SilentlyContinue
    if (-not $NoCleanup) { "`n"; Remove-ProbeTree } else { "`n(cleanup skipped: -NoCleanup)" }
}
