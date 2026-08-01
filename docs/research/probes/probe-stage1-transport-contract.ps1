<#
.SYNOPSIS
Stage 1 transport contract - the remaining CLI behaviours the v2 design asserts.

.DESCRIPTION
Completes the Stage 1 gate in docs/v2-remote-helper-design.md. A4-A8 settled naming, trash,
create-exclusive (sequential and concurrent) and cache contention. This covers what is left,
and every item here is something the design currently ASSERTS without evidence:

  C1  UpdateRevision - does `upload -f merge` update in place (same node uid, new revision) or
      replace the node? The design maps UpdateRevision to merge and needs the uid to be stable,
      because a changing uid means the ref file was recreated rather than revised.
  C2  Claimed-SHA-1 auto-skip - 0.7.0 skips when the existing node's claimed sha1 matches the
      local file BEFORE applying a conflict strategy. If so, writing byte-identical content is
      a silent no-op, and the design's mandatory read-back verification is what catches it.
  C3  Trash output shape - Codex found trash emits node results, not the upload transfer
      summary. The design gives Trash a typed Outcome and must know what to parse.
  C4  Trash on a missing target - the design declares this Committed (idempotent). Verify.
  C5  EnsureDir - create-folder on an EXISTING folder: error or benign? The design calls
      EnsureDir idempotent.
  C6  List shape and empty-folder behaviour - the design's Get-ProbeNodes assumes JSON array,
      and empty must be distinguishable from failure.
  C7  Durability - is a write visible to a subsequent independent CLI process immediately?

Writes a normative results file next to this script so the answers are committed evidence
rather than prose.

.NOTES
Writes only under /my-files/_cas-probe (enforced by probe-lib.ps1, no override).
#>
[CmdletBinding()]
param([switch]$SkipSweepGate, [switch]$NoCleanup)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'probe-lib.ps1')

'=== Stage 1 transport contract ==='
Assert-FleetGreen -SkipSweepGate:$SkipSweepGate
$ver = (Invoke-ProbeCli -CliArgs @('--version')).Output
$cliVer = (($ver -split "`n")[0]).Trim()
"CLI: $cliVer"
[void](Initialize-ProbeRoot)

$ws = New-ProbeWorkspace
$findings = [ordered]@{}

function Add-Finding { param($Id,$Question,$Answer,$Evidence)
    $findings[$Id] = [ordered]@{ question=$Question; answer=$Answer; evidence=$Evidence }
    "`n[$Id] $Question"
    "     -> $Answer"
}

function New-LocalFile { param($Name,$Content)
    $d = Join-Path $ws ([guid]::NewGuid().ToString('N').Substring(0,8))
    New-Item -ItemType Directory -Path $d -Force | Out-Null
    $p = Join-Path $d $Name
    [System.IO.File]::WriteAllText($p, $Content)
    $p
}
function Get-Node { param($Path)
    $r = Invoke-ProbeCli -CliArgs @('filesystem','info',$Path,'--json') -RemotePaths @($Path)
    if ($r.ExitCode -ne 0) { return $null }
    try { $r.Output | ConvertFrom-Json } catch { $null }
}
function Get-Rev { param($Node)   # 0.4.6 wrapped / 0.7.0 unwrapped
    if (-not $Node -or -not $Node.PSObject.Properties['activeRevision']) { return $null }
    $ar = $Node.activeRevision
    if ($ar.PSObject.Properties['value']) { $ar.value } else { $ar }
}

try {
    $sub = New-ProbeFolder -Parent $script:ProbeRoot -Name 'contract'

    # ---- C1 / C2 : UpdateRevision -------------------------------------------------
    $f1 = New-LocalFile 'rev.txt' 'VERSION-ONE'
    [void](Assert-CliOk -Result (Invoke-ProbeCli -CliArgs @('filesystem','upload','-f','skip','--json',$f1,$sub) -RemotePaths @($sub)) -What 'C1 seed')
    $n1 = Get-Node "$sub/rev.txt"; $r1 = Get-Rev $n1

    $f2 = New-LocalFile 'rev.txt' 'VERSION-TWO-DIFFERENT'
    $u2 = Invoke-ProbeCli -CliArgs @('filesystem','upload','-f','merge','--json',$f2,$sub) -RemotePaths @($sub)
    $c2 = Get-UploadCounts -Result $u2
    $n2 = Get-Node "$sub/rev.txt"; $r2 = Get-Rev $n2

    $sameNode = ($n1.uid -eq $n2.uid)
    $newRev   = ($r1.uid -ne $r2.uid)
    Add-Finding 'C1' 'Does `upload -f merge` revise in place, or replace the node?' `
        $(if ($sameNode -and $newRev) { 'REVISES IN PLACE - node uid stable, revision uid changed. UpdateRevision->merge is sound.' }
          elseif (-not $sameNode)     { 'REPLACES THE NODE - node uid changed. UpdateRevision cannot map to merge as designed.' }
          else                        { 'AMBIGUOUS - node uid stable but revision uid unchanged; content may not have been written.' }) `
        @{ counts=@{t=$c2.Transferred;s=$c2.Skipped;f=$c2.Failed}; nodeUidStable=$sameNode; revisionUidChanged=$newRev; claimedSize=$r2.claimedSize }

    # C2: byte-identical rewrite
    $f3 = New-LocalFile 'rev.txt' 'VERSION-TWO-DIFFERENT'    # identical to current remote
    $u3 = Invoke-ProbeCli -CliArgs @('filesystem','upload','-f','merge','--json',$f3,$sub) -RemotePaths @($sub)
    $c3 = Get-UploadCounts -Result $u3
    $r3 = Get-Rev (Get-Node "$sub/rev.txt")
    Add-Finding 'C2' 'Is a byte-identical rewrite silently skipped (claimed-SHA-1 auto-skip)?' `
        $(if ($c3.Skipped -ge 1) { 'YES - skipped without writing. A no-op write is indistinguishable from a real one by exit code; read-back verification is REQUIRED.' }
          elseif ($c3.Transferred -ge 1 -and $r3.uid -ne $r2.uid) { 'NO - a new revision was created even for identical content.' }
          else { 'INCONCLUSIVE - see evidence.' }) `
        @{ counts=@{t=$c3.Transferred;s=$c3.Skipped;f=$c3.Failed}; revisionUidChanged=($r3.uid -ne $r2.uid) }

    # ---- C3 / C4 : Trash ----------------------------------------------------------
    $f4 = New-LocalFile 'doomed.txt' 'x'
    [void](Assert-CliOk -Result (Invoke-ProbeCli -CliArgs @('filesystem','upload','-f','skip','--json',$f4,$sub) -RemotePaths @($sub)) -What 'C3 seed')
    $t1 = Invoke-ProbeCli -CliArgs @('filesystem','trash',"$sub/doomed.txt",'--json') -RemotePaths @("$sub/doomed.txt")
    $trashParsed = $null; try { $trashParsed = $t1.Output | ConvertFrom-Json } catch { }
    Add-Finding 'C3' 'What does `trash --json` emit? (design needs a typed Outcome from it)' `
        $(if ($trashParsed -and $trashParsed.PSObject.Properties['transferredItems']) { 'transfer-summary shaped - same parser as upload' }
          elseif ($trashParsed) { "DIFFERENT SHAPE - keys: $(($trashParsed.PSObject.Properties.Name) -join ',')" }
          else { 'NOT JSON - Trash needs its own outcome mapping, not the upload parser' }) `
        @{ exit=$t1.ExitCode; raw=("$($t1.Output)".Trim()) }

    $t2 = Invoke-ProbeCli -CliArgs @('filesystem','trash',"$sub/never-existed.txt",'--json') -RemotePaths @("$sub/never-existed.txt")
    Add-Finding 'C4' 'Is trash on a MISSING target benign? (design declares it Committed)' `
        $(if ($t2.ExitCode -eq 0) { 'YES - exit 0, idempotent as designed' } else { "NO - exit $($t2.ExitCode). The design must treat absent-target as a distinct, tolerated error." }) `
        @{ exit=$t2.ExitCode; raw=(("$($t2.Output)" -split "`n") | Select-Object -First 3) -join ' | ' }

    # ---- C5 : EnsureDir on an existing folder --------------------------------------
    $e1 = Invoke-ProbeCli -CliArgs @('filesystem','create-folder',$script:ProbeRoot,'contract')
    Add-Finding 'C5' 'Is create-folder on an EXISTING folder benign? (design calls EnsureDir idempotent)' `
        $(if ($e1.ExitCode -eq 0) { 'YES - exit 0' } else { "NO - exit $($e1.ExitCode); EnsureDir must Stat-then-create, or tolerate this specific error" }) `
        @{ exit=$e1.ExitCode; raw=(("$($e1.Output)" -split "`n") | Select-Object -First 2) -join ' | ' }

    # ---- C6 : List shape, incl. empty ----------------------------------------------
    $empty = New-ProbeFolder -Parent $script:ProbeRoot -Name 'emptydir'
    $l1 = Invoke-ProbeCli -CliArgs @('filesystem','list',$empty,'--json') -RemotePaths @($empty)
    $l2 = Invoke-ProbeCli -CliArgs @('filesystem','list',$sub,'--json') -RemotePaths @($sub)
    $emptyParsed = $null; try { $emptyParsed = @($l1.Output | ConvertFrom-Json) } catch { }
    Add-Finding 'C6' 'Is an EMPTY listing distinguishable from a failed one?' `
        $(if ($l1.ExitCode -eq 0 -and [string]::IsNullOrWhiteSpace($l1.Output)) { 'EMPTY OUTPUT with exit 0 - caller must treat blank as empty, not as an error' }
          elseif ($l1.ExitCode -eq 0) { "exit 0 with $(@($emptyParsed).Count) parsed element(s)" }
          else { "exit $($l1.ExitCode) - empty folder listing FAILS; that must not be read as 'no nodes'" }) `
        @{ emptyExit=$l1.ExitCode; emptyRawLen=("$($l1.Output)".Trim()).Length; populatedExit=$l2.ExitCode }

    # ---- C7 : durability to a subsequent process -----------------------------------
    $f5 = New-LocalFile 'durable.txt' 'D'
    $sw = [Diagnostics.Stopwatch]::StartNew()
    [void](Assert-CliOk -Result (Invoke-ProbeCli -CliArgs @('filesystem','upload','-f','skip','--json',$f5,$sub) -RemotePaths @($sub)) -What 'C7 seed')
    $seen = Get-Node "$sub/durable.txt"
    $sw.Stop()
    Add-Finding 'C7' 'Is a completed write immediately visible to the next CLI invocation?' `
        $(if ($seen) { "YES - readable by a fresh process $([math]::Round($sw.Elapsed.TotalMilliseconds))ms after upload returned" } else { 'NO - not visible immediately; the design needs a visibility-delay policy' }) `
        @{ visible=[bool]$seen; msFromUploadStart=[math]::Round($sw.Elapsed.TotalMilliseconds) }
}
finally {
    Remove-Item $ws -Recurse -Force -ErrorAction SilentlyContinue
    if (-not $NoCleanup) { "`n"; Remove-ProbeTree } else { "`n(cleanup skipped: -NoCleanup)" }
}

$out = Join-Path $PSScriptRoot 'stage1-results.json'
[ordered]@{ cli=$cliVer; runUtc=([datetime]::UtcNow.ToString('o')); findings=$findings } |
    ConvertTo-Json -Depth 8 | Set-Content $out -Encoding utf8
"`n=== results written: $out ==="
