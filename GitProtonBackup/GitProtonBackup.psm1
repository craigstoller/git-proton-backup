Set-StrictMode -Version Latest
# GitProtonBackup — git-native backups to Proton Drive.
# Design rationale: docs/design.md. Origin: extracted (hard fork) from the author's
# private backup tooling, 2026-07.

# --- State foundation -------------------------------------------------------

function Get-GpbRoot {
    $r = if ($env:GPB_CONFIG_DIR) { $env:GPB_CONFIG_DIR } else { Join-Path $env:LOCALAPPDATA 'GitProtonBackup' }
    if (-not (Test-Path -LiteralPath $r)) { New-Item -ItemType Directory -Path $r -Force | Out-Null }
    $r
}
function Get-GpbConfigPath { Join-Path (Get-GpbRoot) 'config.json' }
function Get-GpbMarkerDir  { Join-Path (Get-GpbRoot) 'push-pending' }

function Write-GpbJsonAtomic {
    [CmdletBinding()] param([Parameter(Mandatory)][string]$Path, [Parameter(Mandatory)][object]$Object)
    $dir = Split-Path $Path -Parent
    if (-not (Test-Path -LiteralPath $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
    $tmp = Join-Path $dir ".tmp-$([guid]::NewGuid().ToString('N'))"
    $Object | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath $tmp -NoNewline
    Move-Item -LiteralPath $tmp -Destination $Path -Force
}

function Get-GpbDefaultConfig {
    [pscustomobject]@{ ProtonDriveRoot=''; BackupSubdir='GitBackups'; ProtonCli=''; VerifySeconds=60
        RetentionKeep=5; RetentionCheckpoints=24; MaxUnconfirmedAgeDays=7; HeartbeatUrl=''; Repos=@() }
}

function Read-GpbConfig {
    $p = Get-GpbConfigPath
    if (-not (Test-Path -LiteralPath $p)) { throw "No configuration found at $p — run Initialize-ProtonBackup first." }
    $cfg = Get-Content -LiteralPath $p -Raw | ConvertFrom-Json
    foreach ($k in (Get-GpbDefaultConfig).PSObject.Properties.Name) {
        if (-not $cfg.PSObject.Properties[$k]) { throw "config.json is missing '$k' — run Initialize-ProtonBackup to repair." }
    }
    if (-not $cfg.ProtonDriveRoot -or -not (Test-Path -LiteralPath $cfg.ProtonDriveRoot)) {
        throw "ProtonDriveRoot '$($cfg.ProtonDriveRoot)' does not exist — run Initialize-ProtonBackup."
    }
    $cfg
}
function Write-GpbConfig { [CmdletBinding()] param([Parameter(Mandatory)][object]$Config)
    Write-GpbJsonAtomic -Path (Get-GpbConfigPath) -Object $Config }

function Get-GpbSlug {
    [CmdletBinding()] param([Parameter(Mandatory)][string]$RepoPath)
    # Canonicalize first: '.', relative paths, and aliases must slug identically to the
    # absolute path the hook stores in gpb.workrepo (public commands also Resolve-Path at entry).
    $resolved = (Resolve-Path -LiteralPath $RepoPath -ErrorAction SilentlyContinue)?.Path
    if ($resolved) { $RepoPath = $resolved }
    $full = $RepoPath.TrimEnd('\','/').ToLowerInvariant()
    $sha = [System.Security.Cryptography.SHA256]::Create()
    $hex = -join ($sha.ComputeHash([System.Text.Encoding]::UTF8.GetBytes($full)) | ForEach-Object { $_.ToString('x2') })
    $leaf = (Split-Path $full -Leaf) -replace '[^a-z0-9-]', '-'
    "$leaf-$($hex.Substring(0,10))"
}
function Get-GpbMirrorPath { [CmdletBinding()] param([Parameter(Mandatory)][string]$RepoPath)
    Join-Path (Get-GpbRoot) "mirrors\$(Get-GpbSlug -RepoPath $RepoPath).git" }
function Get-GpbBundleDir { [CmdletBinding()] param([Parameter(Mandatory)][object]$Config, [Parameter(Mandatory)][string]$RepoPath)
    Join-Path $Config.ProtonDriveRoot "$($Config.BackupSubdir)\$(Get-GpbSlug -RepoPath $RepoPath)" }

function Get-GpbLockPath {
    # GPB_LOCK_PATH override exists for test isolation (never collide with a real scheduled sweep).
    if ($env:GPB_LOCK_PATH) { return $env:GPB_LOCK_PATH }
    Join-Path (Get-GpbRoot) 'gpb.lock'
}

function Wait-GpbLock {
    # Bounded, poll-based acquisition of the single backup lock shared by the sweep and the
    # push hook. Returns the open FileStream (caller closes) or $null on timeout. Never throws.
    # On timeout, a NON-contention failure (bad lock path, ACLs) emits a warning — otherwise the
    # caller's "another run holds the lock" message would misdiagnose a config problem as
    # contention, day after day, with no toast.
    [CmdletBinding()]
    param([int]$TimeoutSeconds = 0, [int]$PollMs = 1000)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        try { return [System.IO.File]::Open((Get-GpbLockPath), 'OpenOrCreate', 'ReadWrite', 'None') }
        catch {
            # ERROR_SHARING_VIOLATION / ERROR_LOCK_VIOLATION = genuine contention (stay quiet).
            # GetBaseException: PowerShell wraps the IOException in a MethodInvocationException.
            $ex = $_.Exception.GetBaseException()
            $contention = $ex.HResult -in -2147024864, -2147024863
            if ((Get-Date) -ge $deadline) {
                if (-not $contention) { Write-Warning "backup lock: open failed for a non-contention reason: $($ex.Message)" }
                return $null
            }
            Start-Sleep -Milliseconds $PollMs
        }
    } while ($true)
}

# --- Bundle core: digest, fail-closed publication, retention ----------------

function Get-RepoRefDigest {
    [CmdletBinding()]
    param([Parameter(Mandatory)][string]$RepoPath)
    # Canonical ref set: heads + tags only. Remote-tracking refs (e.g. the proton
    # bookkeeping remote) are not content — including them would re-bundle on every push.
    $refs = @(git -C $RepoPath show-ref 2>$null) | Where-Object { $_ -match ' refs/(heads|tags)/' }
    if (-not $refs) { return 'EMPTY' }
    $joined = ($refs | Sort-Object) -join "`n"
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($joined)
    $sha = [System.Security.Cryptography.SHA256]::Create()
    -join ($sha.ComputeHash($bytes) | ForEach-Object { $_.ToString('x2') })
}

function ConvertFrom-CfPlaceholderState {
    [CmdletBinding()]
    param([Parameter(Mandatory)][int]$State)
    [pscustomobject]@{
        IsPlaceholder = ($State -band 0x1) -ne 0
        InSync        = ($State -band 0x8) -ne 0
        Raw           = $State
    }
}

$script:CfTypeAdded = $false
function Initialize-CfType {
    if ($script:CfTypeAdded) { return }
    Add-Type -Language CSharp -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class CfProbe {
  [StructLayout(LayoutKind.Sequential, CharSet=CharSet.Unicode, Pack=4)]
  public struct WFD {
    public uint attr; public uint cLo; public uint cHi; public uint aLo; public uint aHi;
    public uint wLo; public uint wHi; public uint sizeHi; public uint sizeLo; public uint r0; public uint r1;
    [MarshalAs(UnmanagedType.ByValTStr, SizeConst=260)] public string name;
    [MarshalAs(UnmanagedType.ByValTStr, SizeConst=14)] public string alt;
  }
  [DllImport("kernel32.dll", CharSet=CharSet.Unicode, SetLastError=true)]
  public static extern IntPtr FindFirstFileW(string p, out WFD d);
  [DllImport("kernel32.dll", SetLastError=true)] public static extern bool FindClose(IntPtr h);
  [DllImport("cldapi.dll")] public static extern int CfGetPlaceholderStateFromAttributeTag(uint attr, uint tag);
  public static int State(string path){
    WFD d; IntPtr h = FindFirstFileW(path, out d);
    if ((long)h == -1) return 0;
    try {
      uint tag = ((d.attr & 0x400u)!=0) ? d.r0 : 0u;
      return CfGetPlaceholderStateFromAttributeTag(d.attr, tag);
    } finally { FindClose(h); }
  }
}
'@
    $script:CfTypeAdded = $true
}

function Get-CloudFileSyncState {
    [CmdletBinding()]
    param([Parameter(Mandatory)][string]$Path)
    Initialize-CfType
    $state = [CfProbe]::State($Path)
    ConvertFrom-CfPlaceholderState -State $state
}

function Get-RepoPreflight {
    [CmdletBinding()]
    param([Parameter(Mandatory)][string]$RepoPath)
    $gitDir = git -C $RepoPath rev-parse --git-dir 2>$null
    if (-not $gitDir) { return [pscustomobject]@{ Safe=$false; Reason='not a git repo' } }
    $abs = if ([System.IO.Path]::IsPathRooted($gitDir)) { $gitDir } else { Join-Path $RepoPath $gitDir }
    foreach ($m in 'MERGE_HEAD','rebase-merge','rebase-apply','CHERRY_PICK_HEAD') {
        if (Test-Path -LiteralPath (Join-Path $abs $m)) { return [pscustomobject]@{ Safe=$false; Reason="in-progress: $m" } }
    }
    $head = git -C $RepoPath symbolic-ref -q HEAD 2>$null
    # A detached commit changes no head/tag, so it is invisible to both the push trigger and
    # Verify's digest reconciliation — allowing it would let commits escape silently. Deferring
    # WITH a finding (below) makes Verify surface it as attention instead.
    if (-not $head) { return [pscustomobject]@{ Safe=$false; Reason='detached HEAD — coverage identity is branches+tags; return to a branch to back this up' } }
    [pscustomobject]@{ Safe=$true; Reason='' }
}

function ConvertFrom-PorcelainLine {
    [CmdletBinding()] param([Parameter(Mandatory)][string]$Line)
    if ($Line.Length -lt 4) { return $null }
    $x = $Line.Substring(0,1); $y = $Line.Substring(1,1)
    $rest = $Line.Substring(3)
    $untracked = ($x -eq '?' -and $y -eq '?')
    # rename/copy: "ORIG -> NEW" — take NEW
    if (($x -eq 'R' -or $x -eq 'C') -and $rest -match '^(.*?)\s->\s(.*)$') { $rest = $Matches[2] }
    # unquote C-style quoted path (strip surrounding quotes + basic unescape)
    if ($rest.StartsWith('"') -and $rest.EndsWith('"')) {
        $rest = $rest.Substring(1, $rest.Length - 2) -replace '\\"','"' -replace '\\\\','\'
    }
    [pscustomobject]@{ X=$x; Y=$y; Path=$rest; Untracked=$untracked }
}

function Get-RepoStatusEntry {
    param([string]$RepoPath)
    foreach ($line in (git -C $RepoPath status --porcelain=v1 2>$null)) {
        $parsed = ConvertFrom-PorcelainLine -Line $line
        if (-not $parsed) { continue }
        $full = Join-Path $RepoPath $parsed.Path
        $size = if (Test-Path -LiteralPath $full -PathType Leaf) { (Get-Item -LiteralPath $full).Length } else { 0 }
        [pscustomobject]@{ X=$parsed.X; Y=$parsed.Y; Path=$parsed.Path; Untracked=$parsed.Untracked; SizeBytes=$size }
    }
}

function Test-RepoHazard {
    param([string]$RepoPath)
    $h = @()
    if (Test-Path -LiteralPath (Join-Path $RepoPath '.gitattributes')) {
        if (Select-String -LiteralPath (Join-Path $RepoPath '.gitattributes') -Pattern 'filter=lfs' -Quiet -ErrorAction SilentlyContinue) { $h += 'git-lfs' }
    }
    if (Test-Path -LiteralPath (Join-Path $RepoPath '.gitmodules')) { $h += 'submodules' }
    $gitDir = git -C $RepoPath rev-parse --git-dir 2>$null
    if ($gitDir) {
        $abs = if ([System.IO.Path]::IsPathRooted($gitDir)) { $gitDir } else { Join-Path $RepoPath $gitDir }
        $worktreesDir = Join-Path $abs 'worktrees'
        if ((Test-Path -LiteralPath $worktreesDir -PathType Container) -and
            (@(Get-ChildItem -LiteralPath $worktreesDir -Force -ErrorAction SilentlyContinue).Count -gt 0)) {
            $h += 'worktree'
        }
    }
    $h
}

function Get-BundleObjects {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$BundleDir,
        [Parameter(Mandatory)][string]$BundleBaseName,
        [scriptblock]$SyncCheck = { param($p) $false }
    )
    $bundles = @(Get-ChildItem -LiteralPath $BundleDir -Filter "$BundleBaseName-*.bundle" -ErrorAction SilentlyContinue | Sort-Object LastWriteTime)
    if (-not $bundles.Count) { return @() }

    # Find the first bundle per calendar month (monthly checkpoint)
    $monthSeen = @{}
    $result = foreach ($b in ($bundles | Sort-Object LastWriteTime)) {
        $monthKey = $b.LastWriteTime.ToString('yyyy-MM')
        $isCheckpoint = -not $monthSeen.ContainsKey($monthKey)
        $monthSeen[$monthKey] = $true
        $confirmed = & $SyncCheck $b.FullName
        [pscustomobject]@{
            Name                = $b.Name
            FullName            = $b.FullName
            Created             = $b.LastWriteTime
            Confirmed           = [bool]$confirmed
            IsMonthlyCheckpoint = $isCheckpoint
        }
    }
    $result
}

function Get-BundleToPrune {
    [CmdletBinding()]
    param([Parameter(Mandatory)][object[]]$Bundles, [int]$Keep = 5, [int]$KeepCheckpoints = 24)
    $sorted = $Bundles | Sort-Object Created -Descending
    $kept = 0
    $keptCheckpoints = 0
    foreach ($b in $sorted) {
        if (-not $b.Confirmed) { continue }
        if ($b.IsMonthlyCheckpoint) {
            $keptCheckpoints++
            if ($keptCheckpoints -gt $KeepCheckpoints) { $b }
            continue
        }
        $kept++
        if ($kept -gt $Keep) { $b }
    }
}

function Invoke-RepoBundleBackup {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$RepoPath,
        [Parameter(Mandatory)][string]$BundleDir,
        [Parameter(Mandatory)][string]$BundleBaseName,
        [scriptblock]$SyncCheck = { param($p) (Get-CloudFileSyncState -Path $p).InSync },
        [string]$Stamp = ((Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ')),
        [int]$RetentionKeep = 5,
        [int]$RetentionCheckpoints = 24
    )
    $findings = New-Object System.Collections.Generic.List[object]
    $pre = Get-RepoPreflight -RepoPath $RepoPath
    if (-not $pre.Safe) {
        return [pscustomobject]@{ RepoPath=$RepoPath; State='detected_not_backed_up'; Findings=@([pscustomobject]@{ Severity='warn'; Kind='preflight'; Detail=$pre.Reason }) }
    }
    $digest = Get-RepoRefDigest -RepoPath $RepoPath
    $stateFile = Join-Path $BundleDir ".$BundleBaseName.lastdigest"

    $last = (Test-Path -LiteralPath $stateFile) ? (Get-Content -LiteralPath $stateFile -Raw).Trim() : ''
    $digest8 = if ($digest.Length -ge 8) { $digest.Substring(0, 8).ToLowerInvariant() } else { $digest.ToLowerInvariant() }
    # Cache hit requires digest match AND a newest bundle carrying the CURRENT digest fragment
    # (an older retained bundle must not satisfy it — self-heal a deleted current bundle).
    $newest = (Get-ChildItem -LiteralPath $BundleDir -Filter "$BundleBaseName-*.bundle" -ErrorAction SilentlyContinue | Sort-Object LastWriteTime | Select-Object -Last 1)?.FullName
    $newestIsCurrent = [bool]($newest -and $newest -match "-$([regex]::Escape($digest8))\.bundle$")
    $bundlePath = $null
    $createdNewBundle = $false
    if ($digest -ne $last -or -not $newestIsCurrent) {
        if (-not (Test-Path -LiteralPath $BundleDir)) { New-Item -ItemType Directory -Path $BundleDir -Force | Out-Null }
        $target  = Join-Path $BundleDir "$BundleBaseName-$Stamp-$digest8.bundle"
        # .partial in the SAME directory: publish is a same-volume rename (atomic), and the
        # retention/lookup glob (*-*.bundle) never matches the in-progress file.
        $partial = "$target.partial"
        $failedStage = $null
        git -C $RepoPath bundle create $partial HEAD --branches --tags 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) { $failedStage = 'create' }
        if (-not $failedStage) {
            git -C $RepoPath bundle verify $partial 2>&1 | Out-Null
            if ($LASTEXITCODE -ne 0) { $failedStage = 'verify' }
        }
        if (-not $failedStage) {
            # Move-Item does not throw when $target already exists as a directory — it silently
            # nests the source inside it instead of overwriting. Guard that case explicitly so a
            # blocked target path (e.g. something else occupying the name) fails closed as 'publish'.
            if (Test-Path -LiteralPath $target -PathType Container) {
                $failedStage = 'publish'
            } else {
                try { Move-Item -LiteralPath $partial -Destination $target -Force -ErrorAction Stop }
                catch { $failedStage = 'publish' }
            }
        }
        if ($failedStage) {
            Remove-Item -LiteralPath $partial -Force -ErrorAction SilentlyContinue
            $findings.Add([pscustomobject]@{ Severity='high'; Kind='bundle_failed'; Detail="bundle $failedStage failed for $RepoPath" })
            return [pscustomobject]@{ RepoPath=$RepoPath; State='detected_not_backed_up'; BundlePath=$null; Findings=$findings.ToArray() }
        }
        # Fail-closed: the digest is stamped only after the bundle is verified and in place.
        # A stamp failure is non-fatal (the bundle exists); the next run just re-creates.
        try { Set-Content -LiteralPath $stateFile -Value $digest -NoNewline -ErrorAction Stop }
        catch { $findings.Add([pscustomobject]@{ Severity='warn'; Kind='bundle_failed'; Detail="digest stamp failed for $RepoPath (bundle published; next run re-bundles)" }) }
        $bundlePath = $target
        $createdNewBundle = $true
    } else {
        $bundlePath = $newest
    }

    $state = 'pending_sync'
    if ($bundlePath -and (& $SyncCheck $bundlePath)) { $state = 'backed_up' }
    if ($findings.Count -and $state -eq 'backed_up') { $state = 'detected_not_backed_up' }

    # Retention: prune whenever the newest bundle is observed confirmed (not only when created
    # this call — asynchronous confirmation is the hook's normal path). The count gate keeps
    # per-bundle SyncCheck probes off the hot path when nothing could be pruned (Keep default 5).
    if ($state -eq 'backed_up' -and
        @(Get-ChildItem -LiteralPath $BundleDir -Filter "$BundleBaseName-*.bundle" -ErrorAction SilentlyContinue).Count -gt $RetentionKeep) {
        $bundleObjects = @(Get-BundleObjects -BundleDir $BundleDir -BundleBaseName $BundleBaseName -SyncCheck $SyncCheck)
        $toPrune = @(Get-BundleToPrune -Bundles $bundleObjects -Keep $RetentionKeep -KeepCheckpoints $RetentionCheckpoints)
        foreach ($b in $toPrune) {
            Remove-Item -LiteralPath $b.FullName -Force -ErrorAction SilentlyContinue
        }
    }

    [pscustomobject]@{ RepoPath=$RepoPath; State=$state; BundlePath=$bundlePath; Findings=$findings.ToArray() }
}
