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
    # GPB_LOCK_PATH override exists for test isolation (never collide with a real scheduled task run).
    if ($env:GPB_LOCK_PATH) { return $env:GPB_LOCK_PATH }
    Join-Path (Get-GpbRoot) 'gpb.lock'
}

function Wait-GpbLock {
    # Bounded, poll-based acquisition of the single backup lock shared by Invoke-ProtonBackupVerify
    # (or the scheduled task) and the push hook. Returns the open FileStream (caller closes) or
    # $null on timeout. Never throws.
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

# --- Option A: Proton Drive CLI verification ---------------------------------

function Get-CloudBundlePath {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$LocalPath,
        [Parameter(Mandatory)][string]$DriveLocalRoot,
        [string]$DriveCloudRoot = '/my-files'
    )
    $full = (Resolve-Path -LiteralPath $LocalPath -ErrorAction SilentlyContinue)?.Path
    if (-not $full) { $full = [System.IO.Path]::GetFullPath($LocalPath) }   # file may not exist yet in tests
    $rootFull = (Resolve-Path -LiteralPath $DriveLocalRoot -ErrorAction SilentlyContinue)?.Path
    if (-not $rootFull) { $rootFull = [System.IO.Path]::GetFullPath($DriveLocalRoot) }
    $root = $rootFull.TrimEnd('\','/')
    # Canonical containment check — a blind Substring would silently produce a garbage
    # relative path (or throw an unhelpful index error) for anything outside the root.
    $under = $full.Equals($root, [System.StringComparison]::OrdinalIgnoreCase) -or
             $full.StartsWith("$root\", [System.StringComparison]::OrdinalIgnoreCase) -or
             $full.StartsWith("$root/", [System.StringComparison]::OrdinalIgnoreCase)
    if (-not $under) { throw "Get-CloudBundlePath: '$LocalPath' is not under DriveLocalRoot '$DriveLocalRoot'" }
    $rel = $full.Substring($root.Length).TrimStart('\','/').Replace('\','/')
    "$DriveCloudRoot/$rel"
}

function Confirm-BundleUploaded {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$CloudPath,
        [Parameter(Mandatory)][string]$CliPath,
        # InfoRunner returns @{ ExitCode=<int>; Output=<string> }; default invokes the real CLI
        [scriptblock]$InfoRunner
    )
    if (-not $InfoRunner) {
        $InfoRunner = {
            param($cp, $cli)
            $out = & $cli filesystem info $cp 2>&1 | Out-String
            [pscustomobject]@{ ExitCode = $LASTEXITCODE; Output = $out }
        }
    }
    $r = & $InfoRunner $CloudPath $CliPath
    if ($r.ExitCode -eq 0 -and $r.Output -match "state:\s*'active'") {
        return [pscustomobject]@{ Confirmed=$true;  Reason='cli_confirmed' }
    }
    if ($r.Output -match 'Node not found') {
        return [pscustomobject]@{ Confirmed=$false; Reason='not_in_cloud' }
    }
    if ($r.Output -match 'auth|login|unauthor|session') {
        return [pscustomobject]@{ Confirmed=$false; Reason='auth_error' }
    }
    [pscustomobject]@{ Confirmed=$false; Reason='unverified' }
}

function Test-ProtonCliReady {
    [CmdletBinding()]
    param([string]$CliPath, [scriptblock]$Runner)
    if (-not $CliPath) { return $false }
    if (-not [System.IO.Path]::IsPathRooted($CliPath)) {
        # A bare command name (e.g. 'proton-drive') must resolve via PATH before the
        # Test-Path existence check below, which only understands literal filesystem paths.
        $cmd = Get-Command -Name $CliPath -ErrorAction SilentlyContinue
        if ($cmd) { $CliPath = $cmd.Source }
    }
    if (-not (Test-Path -LiteralPath $CliPath)) { return $false }
    if (-not $Runner) { $Runner = { param($cli) & $cli filesystem list '/my-files' 2>&1 | Out-Null; $LASTEXITCODE } }
    (& $Runner $CliPath) -eq 0
}

# --- Push pending markers (pessimistic backstop for git-push backups) --------

function Write-PushPendingMarker {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$RepoPath,
        [Parameter(Mandatory)][ValidateSet('in_progress','deferred_lock','no_bundle','verify_timeout','auth_error','cli_unready')][string]$Reason,
        [Parameter(Mandatory)][string]$BundleDir,
        [Parameter(Mandatory)][string]$BundleBaseName,
        [string]$BundlePath,
        [string]$Stamp = ((Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ'))
    )
    $path = Join-Path (Get-GpbMarkerDir) "$(Get-GpbSlug -RepoPath $RepoPath).json"
    Write-GpbJsonAtomic -Path $path -Object ([pscustomobject]@{
        RepoPath = $RepoPath; Reason = $Reason; BundleDir = $BundleDir
        BundleBaseName = $BundleBaseName; BundlePath = $BundlePath; Stamp = $Stamp
    })
}

function Remove-PushPendingMarker {
    [CmdletBinding()] param([Parameter(Mandatory)][string]$RepoPath)
    Remove-Item -LiteralPath (Join-Path (Get-GpbMarkerDir) "$(Get-GpbSlug -RepoPath $RepoPath).json") -Force -ErrorAction SilentlyContinue
}

function Read-GpbMarker {
    # Returns the parsed marker or $null. Never deletes: unreadable/incomplete markers are
    # quarantined (renamed .bad) so evidence survives for inspection.
    [CmdletBinding()] param([Parameter(Mandatory)][object]$File)
    try {
        $m = Get-Content -LiteralPath $File.FullName -Raw | ConvertFrom-Json -ErrorAction Stop
        foreach ($k in 'RepoPath','Reason','BundleDir','BundleBaseName') {
            if (-not $m.PSObject.Properties[$k] -or -not $m.$k) { throw "missing $k" }
        }
        $m
    } catch {
        # Unique quarantine name: repeated corruptions must not overwrite earlier evidence.
        $dest = "$($File.FullName).$([guid]::NewGuid().ToString('N').Substring(0,6)).bad"
        Move-Item -LiteralPath $File.FullName -Destination $dest -Force -ErrorAction SilentlyContinue
        $null
    }
}

# --- Push mirror lifecycle (install/remove/health) --------------------------

function Install-GpbMirror {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$RepoPath,
        [switch]$Force,
        [switch]$SetUpstream
    )
    git -C $RepoPath rev-parse --git-dir *> $null
    if ($LASTEXITCODE -ne 0) { throw "Install-GpbMirror: '$RepoPath' is not a git repository — run 'git init' first." }

    $mirror = Get-GpbMirrorPath -RepoPath $RepoPath

    # Ownership by LOCATION, not by config, so Repair genuinely repairs: a deleted mirror has no
    # gpb.workrepo to check, and a moved repo's remote points at the OLD slug path — both are ours
    # as long as the url lives under our mirrors root. Anything else is a foreign remote and is
    # refused unless -Force (which repoints it — the foreign remote's target is never touched).
    $existingUrl = git -C $RepoPath remote get-url proton 2>$null
    if ($LASTEXITCODE -ne 0) { $existingUrl = $null }
    $oursRoot = (Join-Path (Get-GpbRoot) 'mirrors')
    if ($existingUrl -and -not $Force) {
        $isOurs = $existingUrl.StartsWith($oursRoot, [System.StringComparison]::OrdinalIgnoreCase)
        if (-not $isOurs) { throw "A 'proton' remote already exists and is not owned by GitProtonBackup (url: $existingUrl) — remove/rename it, or re-run with -Force to replace the wiring (the foreign remote's target is never touched)." }
        if ($existingUrl -ne $mirror) {
            # Moved repo: old-slug mirror. Repoint, and delete the old mirror only under the
            # standard delete-safe rule (bare + carries a gpb.workrepo).
            if ((Test-Path -LiteralPath $existingUrl) -and ((git -C $existingUrl rev-parse --is-bare-repository 2>$null) -eq 'true') -and (git -C $existingUrl config gpb.workrepo 2>$null)) {
                Remove-Item -LiteralPath $existingUrl -Recurse -Force
            }
        }
    }

    if (-not (Test-Path -LiteralPath (Join-Path $mirror 'HEAD'))) {
        New-Item -ItemType Directory -Path $mirror -Force | Out-Null
        git init --bare -q $mirror
        if ($LASTEXITCODE -ne 0) { throw "git init --bare failed for $mirror" }
    }
    git -C $mirror config gpb.workrepo $RepoPath

    # Quoting-safe, version-independent shim: the work-repo path travels via an environment
    # variable (GPB_WORKREPO) — NEVER inline in the -Command string. A path containing an
    # apostrophe (e.g. C:\Craig's Code) would break any sh-side quote embedding. `\$env:` is
    # escaped so sh does NOT expand it, leaving the literal `$env:GPB_WORKREPO` for pwsh to read
    # as its own environment-variable syntax; `$vs` is unescaped so sh substitutes the (int)
    # verifyseconds value inline before pwsh ever sees it. LF-only; unconditional exit 0 (no
    # exec) so the hook never reports failure even if pwsh fails to start, Import-Module fails,
    # or the target function doesn't exist yet. Post-receive stdin is intentionally NOT
    # forwarded (see Task 6's upstream-healing delta).
    $shim = @(
        '#!/bin/sh',
        'GPB_WORKREPO=$(git config gpb.workrepo); export GPB_WORKREPO',
        'vs=$(git config --int gpb.verifyseconds); [ -n "$vs" ] || vs=60',
        'pwsh -NoProfile -Command "Import-Module GitProtonBackup; Invoke-ProtonBackupHook -WorkRepo \$env:GPB_WORKREPO -VerifySeconds $vs"',
        'exit 0'
    ) -join "`n"
    $hooksDir = Join-Path $mirror 'hooks'
    if (-not (Test-Path -LiteralPath $hooksDir)) { New-Item -ItemType Directory -Path $hooksDir -Force | Out-Null }
    [System.IO.File]::WriteAllText((Join-Path $hooksDir 'post-receive'), $shim + "`n")

    git -C $RepoPath remote get-url proton *> $null
    if ($LASTEXITCODE -ne 0) { git -C $RepoPath remote add proton $mirror }
    else { git -C $RepoPath remote set-url proton $mirror }
    $existingPush = @(git -C $RepoPath config --get-all remote.proton.push 2>$null)
    if ($existingPush -notcontains '+refs/heads/*:refs/heads/*') { git -C $RepoPath config --add remote.proton.push '+refs/heads/*:refs/heads/*' }
    if ($existingPush -notcontains '+refs/tags/*:refs/tags/*')   { git -C $RepoPath config --add remote.proton.push '+refs/tags/*:refs/tags/*' }

    # Initial push + upstream — only when the repo has a born HEAD. A failed push must fail the
    # install (side-effects-first: the module manifest is written only after this succeeds).
    git -C $RepoPath rev-parse -q --verify HEAD *> $null
    if ($LASTEXITCODE -eq 0) {
        git -C $RepoPath push -q proton 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "initial 'git push proton' failed for $RepoPath (mirror: $mirror)" }
        $cur = git -C $RepoPath branch --show-current
        if ($cur) {
            # Upstream policy: never clobber an existing upstream unless -SetUpstream was passed.
            $hasUpstream = [bool](git -C $RepoPath config "branch.$cur.remote" 2>$null)
            if ($SetUpstream -or -not $hasUpstream) {
                git -C $RepoPath config "branch.$cur.remote" proton
                git -C $RepoPath config "branch.$cur.merge" "refs/heads/$cur"
            }
        }
    }
    [pscustomobject]@{ MirrorPath = $mirror }
}

function Remove-GpbMirror {
    [CmdletBinding()]
    param([Parameter(Mandatory)][string]$RepoPath)
    $mirror = git -C $RepoPath remote get-url proton 2>$null
    if ($LASTEXITCODE -ne 0) { $mirror = $null }
    if (-not $mirror) { $mirror = Get-GpbMirrorPath -RepoPath $RepoPath }
    git -C $RepoPath remote remove proton 2>&1 | Out-Null
    if (-not $mirror -or -not (Test-Path -LiteralPath $mirror)) { return }
    # Safety: only delete a directory that is verifiably OUR mirror for THIS repo — a bare repo
    # whose gpb.workrepo canonically equals $RepoPath (Resolve-Path both sides, case-insensitive).
    # Never Remove-Item -Recurse a path inferred from config otherwise.
    $isBare = (git -C $mirror rev-parse --is-bare-repository 2>$null) -eq 'true'
    $workrepo = git -C $mirror config gpb.workrepo 2>$null
    if ($LASTEXITCODE -ne 0) { $workrepo = $null }
    $ownsIt = $false
    if ($isBare -and $workrepo -and (Test-Path -LiteralPath $workrepo)) {
        $a = (Resolve-Path -LiteralPath $workrepo).Path.TrimEnd('\','/')
        $b = (Resolve-Path -LiteralPath $RepoPath).Path.TrimEnd('\','/')
        $ownsIt = [string]::Equals($a, $b, [System.StringComparison]::OrdinalIgnoreCase)
    }
    if ($ownsIt) { Remove-Item -LiteralPath $mirror -Recurse -Force }
    else { Write-Warning "proton remote pointed at '$mirror', which is not verifiably owned by this repo — left in place (remote removed)." }
}

function Test-GpbMirror {
    [CmdletBinding()] param([Parameter(Mandatory)][string]$RepoPath)
    $mirror = git -C $RepoPath remote get-url proton 2>$null
    if ($LASTEXITCODE -ne 0 -or -not $mirror) {
        return [pscustomobject]@{ HasRemote=$false; MirrorPath=$null; MirrorExists=$false; HookExists=$false; WorkRepoOk=$false }
    }
    $mirrorExists = Test-Path -LiteralPath (Join-Path $mirror 'HEAD')
    $hookExists   = $mirrorExists -and (Test-Path -LiteralPath (Join-Path $mirror 'hooks/post-receive'))
    $workrepo = if ($mirrorExists) { git -C $mirror config gpb.workrepo 2>$null } else { $null }
    # WorkRepoOk: gpb.workrepo must canonically resolve to THIS repo — a mirror wired elsewhere
    # (or pointed at a path that no longer exists) must not pass.
    $workRepoOk = $false
    if ($workrepo -and (Test-Path -LiteralPath $workrepo)) {
        $a = (Resolve-Path -LiteralPath $workrepo).Path.TrimEnd('\','/')
        $b = (Resolve-Path -LiteralPath $RepoPath).Path.TrimEnd('\','/')
        $workRepoOk = [string]::Equals($a, $b, [System.StringComparison]::OrdinalIgnoreCase)
    }
    [pscustomobject]@{
        HasRemote    = $true
        MirrorPath   = $mirror
        MirrorExists = [bool]$mirrorExists
        HookExists   = [bool]$hookExists
        WorkRepoOk   = $workRepoOk
    }
}

# --- Push flow + hook entry point --------------------------------------------

function Invoke-PushBackupFlow {
    # post-receive flow (spec: docs/design.md — hook flow + verification table).
    # A normal function: no `exit`, messages via Write-Host (git relays them as remote: lines).
    # Throws only on unexpected errors; Invoke-ProtonBackupHook catches and still returns.
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$WorkRepo,
        [object]$OutConfig,
        [int]$VerifySeconds,
        [int]$PollSeconds = 5,
        [int]$LockTimeoutSeconds = 15,
        # Test seams — production leaves these unset.
        [scriptblock]$SyncCheck, [scriptblock]$InfoRunner, [scriptblock]$CliReadyRunner
    )
    $cfg = if ($OutConfig) { $OutConfig } else { Read-GpbConfig }
    if (-not $PSBoundParameters.ContainsKey('VerifySeconds')) { $VerifySeconds = $cfg.VerifySeconds }

    $bundleDir = Get-GpbBundleDir -Config $cfg -RepoPath $WorkRepo
    $baseName  = Split-Path $WorkRepo -Leaf

    # 1. Pessimistic marker BEFORE any long-running work (Ctrl+C / crash safe).
    Write-PushPendingMarker -RepoPath $WorkRepo -Reason in_progress -BundleDir $bundleDir -BundleBaseName $baseName

    # 2. Lock for the bundle step only (bounded).
    $lock = Wait-GpbLock -TimeoutSeconds $LockTimeoutSeconds
    if (-not $lock) {
        Write-PushPendingMarker -RepoPath $WorkRepo -Reason deferred_lock -BundleDir $bundleDir -BundleBaseName $baseName
        Write-Host 'backup deferred — another backup operation is active; run Invoke-ProtonBackupVerify (or the scheduled task) to confirm'
        return
    }

    # 3. Bundle (never auto-commit on a deliberate push); release the lock immediately after.
    try {
        $res = Invoke-RepoBundleBackup -RepoPath $WorkRepo -BundleDir $bundleDir -BundleBaseName $baseName `
            -SyncCheck { param($p) $false } -RetentionKeep $cfg.RetentionKeep -RetentionCheckpoints $cfg.RetentionCheckpoints
    } finally {
        $lock.Close()
        Remove-Item -LiteralPath (Get-GpbLockPath) -Force -ErrorAction SilentlyContinue
    }
    if (-not $res.PSObject.Properties['BundlePath'] -or -not $res.BundlePath) {
        Write-PushPendingMarker -RepoPath $WorkRepo -Reason no_bundle -BundleDir $bundleDir -BundleBaseName $baseName
        $why = (@($res.Findings) | Select-Object -First 1)?.Detail
        Write-Host "no bundle produced ($why) — run Invoke-ProtonBackupVerify (or the scheduled task) to confirm"
        return
    }

    # 4. Dirty-tree note: uncommitted changes are never included in a bundle (HEAD-only content).
    $dirty = @(git -C $WorkRepo status --porcelain=v1 2>$null).Count
    if ($dirty -gt 0) { Write-Host "note: $dirty uncommitted change(s) not included — commit to back them up" }

    # 5. Upstream healing for the CURRENT branch only, only when it lacks one (existing upstreams
    #    untouched). Enumerating all local branches would set upstreams on dormant branches the
    #    user never pushed — too intrusive for a public tool; the current branch is what a push
    #    almost always means. Enumeration/config failure skips healing with a warning — it must
    #    never fail the hook.
    try {
        $cur = git -C $WorkRepo branch --show-current
        if ($LASTEXITCODE -eq 0 -and $cur -and -not (git -C $WorkRepo config "branch.$cur.remote" 2>$null)) {
            git -C $WorkRepo config "branch.$cur.remote" proton
            git -C $WorkRepo config "branch.$cur.merge" "refs/heads/$cur"
            Write-Host "tracking set: $cur -> proton/$cur"
        }
    } catch {
        Write-Warning "upstream healing skipped: $($_.Exception.Message)"
    }

    # 6. Bounded verify, lock-free. Wording table lives in the spec; "confirmed on Proton" is
    #    reserved for CLI confirmation.
    $cliPath = if ($cfg.ProtonCli) { $cfg.ProtonCli } else { 'proton-drive' }
    if (-not $CliReadyRunner) { $CliReadyRunner = { Test-ProtonCliReady -CliPath $cliPath } }
    if (-not $SyncCheck)      { $SyncCheck      = { param($p) (Get-CloudFileSyncState -Path $p).InSync } }
    $cloudPath = Get-CloudBundlePath -LocalPath $res.BundlePath -DriveLocalRoot $cfg.ProtonDriveRoot

    if (& $CliReadyRunner) {
        $deadline = (Get-Date).AddSeconds($VerifySeconds)
        do {
            $c = if ($InfoRunner) { Confirm-BundleUploaded -CloudPath $cloudPath -CliPath $cliPath -InfoRunner $InfoRunner }
                 else             { Confirm-BundleUploaded -CloudPath $cloudPath -CliPath $cliPath }
            if ($c.Confirmed) {
                Remove-PushPendingMarker -RepoPath $WorkRepo
                Write-Host 'confirmed on Proton'
                return
            }
            if ($c.Reason -eq 'auth_error') {
                Write-PushPendingMarker -RepoPath $WorkRepo -Reason auth_error -BundleDir $bundleDir -BundleBaseName $baseName -BundlePath $res.BundlePath
                Write-Host 'staged, not yet confirmed — Proton CLI session expired; run Invoke-ProtonBackupVerify (or the scheduled task) to confirm'
                return
            }
            if ((Get-Date) -lt $deadline) { Start-Sleep -Seconds $PollSeconds }
        } while ((Get-Date) -lt $deadline)
        Write-PushPendingMarker -RepoPath $WorkRepo -Reason verify_timeout -BundleDir $bundleDir -BundleBaseName $baseName -BundlePath $res.BundlePath
        Write-Host 'staged, not yet confirmed — run Invoke-ProtonBackupVerify (or the scheduled task) to confirm'
        return
    }

    # CLI unavailable: IN_SYNC is the degraded verifier (never claims "confirmed on Proton").
    if (& $SyncCheck $res.BundlePath) {
        Remove-PushPendingMarker -RepoPath $WorkRepo
        Write-Host 'staged; in-sync per Cloud Files (CLI verification unavailable)'
    } else {
        Write-PushPendingMarker -RepoPath $WorkRepo -Reason cli_unready -BundleDir $bundleDir -BundleBaseName $baseName -BundlePath $res.BundlePath
        Write-Host 'staged, not yet confirmed — Proton CLI unavailable; run Invoke-ProtonBackupVerify (or the scheduled task) to confirm'
    }
}

function Invoke-ProtonBackupHook {
    # post-receive entry point. Always returns; never throws — a push must never appear to fail
    # on backup bookkeeping. Invoked by the mirror shim via Import-Module (version-independent).
    [CmdletBinding()] param([Parameter(Mandatory)][string]$WorkRepo, [int]$VerifySeconds = 0)
    try {
        if ($env:GPB_HOOK_DISABLED -eq '1') { return }
        foreach ($v in 'GIT_DIR','GIT_WORK_TREE','GIT_INDEX_FILE','GIT_OBJECT_DIRECTORY','GIT_ALTERNATE_OBJECT_DIRECTORIES','GIT_QUARANTINE_PATH') {
            Remove-Item "Env:$v" -ErrorAction SilentlyContinue
        }
        if ($VerifySeconds -gt 0) { Invoke-PushBackupFlow -WorkRepo $WorkRepo -VerifySeconds $VerifySeconds }
        else { Invoke-PushBackupFlow -WorkRepo $WorkRepo }
    } catch {
        Write-Host "backup hook error: $($_.Exception.Message) — run Invoke-ProtonBackupVerify (or the scheduled task) to confirm"
    }
}
