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
    # A path that no longer exists on disk (e.g. Uninstall of an already-deleted repo) can't be
    # Resolve-Path'd — GetUnresolvedProviderPathFromPSPath still normalizes it (relative segments
    # resolved against PowerShell's actual current location, separators/'.'/'..' collapsed)
    # WITHOUT requiring the target to exist, so a deleted repo slugs identically to how it slugged
    # while it still existed. Falling back to the raw string here was the root cause of
    # deleted-repo mirrors becoming unreachable by slug. NOTE: [System.IO.Path]::GetFullPath is
    # NOT safe for this — it resolves against [Environment]::CurrentDirectory, which does not
    # track PowerShell's Push-Location/$PWD in this host, so a relative-path deleted-repo
    # Uninstall would silently normalize against the wrong directory.
    $resolved = (Resolve-Path -LiteralPath $RepoPath -ErrorAction SilentlyContinue)?.Path
    $RepoPath = if ($resolved) { $resolved } else { $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($RepoPath) }
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
    # Unlike Get-GpbSlug's note on why [System.IO.Path]::GetFullPath is unsafe for a possibly-
    # relative, possibly-deleted REPO path, both inputs here are always already-rooted (absolute)
    # by the time any caller reaches this function — $res.BundlePath and $cfg.ProtonDriveRoot are
    # both produced upstream via Resolve-Path/New-Item on real paths — so GetFullPath below is just
    # a no-op normalization fallback for a not-yet-created bundle file, never a real relative-path
    # resolution against the wrong current directory.
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
        'vs=$(git config --int gpb.verifyseconds); [ -n "$vs" ] || vs=0',
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
    # Ownership-symmetric with Install (issue #2): Install refuses a foreign 'proton' remote,
    # so Uninstall must not strip one either. The remote is removed only when its url points
    # into OUR mirrors root; a foreign remote is warned about and left in place — while any
    # GitProtonBackup mirror for this repo is still cleaned up under the delete-safe rule below.
    $remoteUrl = git -C $RepoPath remote get-url proton 2>$null
    if ($LASTEXITCODE -ne 0) { $remoteUrl = $null }
    $oursRoot = Join-Path (Get-GpbRoot) 'mirrors'
    $remoteIsOurs = [bool]($remoteUrl -and $remoteUrl.StartsWith($oursRoot, [System.StringComparison]::OrdinalIgnoreCase))
    if ($remoteIsOurs) { git -C $RepoPath remote remove proton 2>&1 | Out-Null }
    elseif ($remoteUrl) { Write-Warning "the 'proton' remote points at '$remoteUrl', which is not a GitProtonBackup mirror — remote left in place." }
    $mirror = if ($remoteIsOurs) { $remoteUrl } else { Get-GpbMirrorPath -RepoPath $RepoPath }
    if (-not $mirror -or -not (Test-Path -LiteralPath $mirror)) { return }
    # Safety: only delete a directory that is verifiably OUR mirror for THIS repo — a bare repo
    # whose gpb.workrepo canonically equals $RepoPath (Resolve-Path both sides, case-insensitive).
    # Never Remove-Item -Recurse a path inferred from config otherwise.
    $isBare = (git -C $mirror rev-parse --is-bare-repository 2>$null) -eq 'true'
    $workrepo = git -C $mirror config gpb.workrepo 2>$null
    if ($LASTEXITCODE -ne 0) { $workrepo = $null }
    $ownsIt = $false
    if ($isBare -and $workrepo) {
        # Normalize BOTH sides the same way: Resolve-Path when the path still exists (canonical
        # casing/symlinks), else GetUnresolvedProviderPathFromPSPath (honors PowerShell's actual
        # $PWD — see Get-GpbSlug's note on why [System.IO.Path]::GetFullPath is unsafe here).
        # Gating this on Test-Path $workrepo (as before) made a DELETED repo's own mirror
        # permanently "unowned" and un-removable — exactly backwards from what Uninstall/Repair on
        # a deleted repo need to do.
        $a = if (Test-Path -LiteralPath $workrepo) { (Resolve-Path -LiteralPath $workrepo).Path } else { $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($workrepo) }
        $b = if (Test-Path -LiteralPath $RepoPath) { (Resolve-Path -LiteralPath $RepoPath).Path } else { $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($RepoPath) }
        $ownsIt = [string]::Equals($a.TrimEnd('\','/'), $b.TrimEnd('\','/'), [System.StringComparison]::OrdinalIgnoreCase)
    }
    if ($ownsIt) { Remove-Item -LiteralPath $mirror -Recurse -Force }
    else { Write-Warning "mirror at '$mirror' is not verifiably owned by this repo — left in place." }
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

# --- Public commands: Install / Uninstall / Repair ---------------------------

function Install-ProtonBackup {
    [CmdletBinding()] param([Parameter(Mandatory)][string]$RepoPath, [switch]$SetUpstream, [switch]$Force)
    $cfg = Read-GpbConfig
    $RepoPath = (Resolve-Path -LiteralPath $RepoPath -ErrorAction Stop).Path   # canonical identity everywhere
    git -C $RepoPath rev-parse --git-dir *> $null
    if ($LASTEXITCODE -ne 0) { throw "'$RepoPath' is not a git repository." }
    $shallow = git -C $RepoPath rev-parse --is-shallow-repository
    if ($LASTEXITCODE -ne 0) { throw "'$RepoPath': git failed while probing repository shape." }
    if ($shallow -eq 'true') {
        throw "'$RepoPath' is a shallow clone — git bundles from shallow repos are unreliable. Unshallow it first (git fetch --unshallow)."
    }
    foreach ($h in @(Test-RepoHazard -RepoPath $RepoPath)) {
        Write-Warning ("restore-coverage hazard '{0}': bundles won't include {1} — see the Limits section of the docs." -f $h,
            $(switch ($h) { 'git-lfs' { 'LFS objects (pointer files only)' } 'submodules' { 'submodule repos (wire each separately)' } default { 'externally-stored state' } }))
    }
    # Mirror wiring runs OUTSIDE the global lock: the initial push fires the hook, whose flow
    # takes the same lock for its bundle step — holding it here would self-contend (15s stall +
    # a spurious deferred_lock marker on every real install; review finding).
    Install-GpbMirror -RepoPath $RepoPath -SetUpstream:$SetUpstream -Force:$Force | Out-Null
    $lock = Wait-GpbLock -TimeoutSeconds 15
    if (-not $lock) { throw 'Another GitProtonBackup operation holds the lock — the mirror is wired; re-run to finish registering.' }
    try {
        $cfg = Read-GpbConfig
        if (@($cfg.Repos) -notcontains $RepoPath) { $cfg.Repos = @($cfg.Repos) + $RepoPath; Write-GpbConfig -Config $cfg }
    } finally { $lock.Close(); Remove-Item (Get-GpbLockPath) -Force -ErrorAction SilentlyContinue }
    Write-Host "Wired. Back up with: git push proton   (status: Get-ProtonBackupStatus)"
}

function Uninstall-ProtonBackup {
    [CmdletBinding()] param([Parameter(Mandatory)][string]$RepoPath)
    # Canonicalize once at entry (same contract as Install/Repair): an existing repo uninstalled
    # via a relative path must match the absolute path Install registered. A deleted repo
    # (unresolvable via Resolve-Path) still normalizes to the SAME absolute form via
    # GetUnresolvedProviderPathFromPSPath (honors PowerShell's actual $PWD, unlike
    # [System.IO.Path]::GetFullPath — see Get-GpbSlug) — Remove-GpbMirror/Remove-PushPendingMarker/
    # the registry filter all need the identical canonical identity Install stored, even when the
    # working tree is gone (this is the whole point of Uninstall on an already-deleted repo: Verify
    # tells users to do exactly this).
    $resolved = (Resolve-Path -LiteralPath $RepoPath -ErrorAction SilentlyContinue)?.Path
    $RepoPath = if ($resolved) { $resolved } else { $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($RepoPath) }
    $lock = Wait-GpbLock -TimeoutSeconds 15
    if (-not $lock) { throw 'Another GitProtonBackup operation holds the lock — retry shortly.' }
    try {
        $mirror = Get-GpbMirrorPath -RepoPath $RepoPath
        $hadRemote = $false
        git -C $RepoPath remote get-url proton *> $null
        if ($LASTEXITCODE -eq 0) { $hadRemote = $true }
        $mirrorExisted = Test-Path -LiteralPath $mirror

        Remove-GpbMirror -RepoPath $RepoPath
        $mirrorRemoved = $mirrorExisted -and -not (Test-Path -LiteralPath $mirror)

        Remove-PushPendingMarker -RepoPath $RepoPath

        $cfg = Read-GpbConfig
        $hadRegistryEntry = @($cfg.Repos) -contains $RepoPath
        $cfg.Repos = @(@($cfg.Repos) | Where-Object { $_ -ne $RepoPath })
        Write-GpbConfig -Config $cfg

        $didSomething = $hadRemote -or $mirrorRemoved -or $hadRegistryEntry
    } finally { $lock.Close(); Remove-Item (Get-GpbLockPath) -Force -ErrorAction SilentlyContinue }
    if ($didSomething) { Write-Host 'Unwired. Existing bundles on Proton Drive were left in place.' }
    else { Write-Warning "nothing to uninstall for $RepoPath (not wired and not registered)" }
}

function Repair-ProtonBackup {
    [CmdletBinding()] param([Parameter(Mandatory)][string]$RepoPath)
    Install-ProtonBackup -RepoPath $RepoPath
}

# --- Public commands: Initialize + config get/set ----------------------------

function Read-GpbConfigOrDefault {
    # Tolerant variant of Read-GpbConfig for Initialize: unlike Read-GpbConfig, this must succeed
    # even when no config exists yet, AND even when an existing config's ProtonDriveRoot no longer
    # exists on disk (re-running Initialize to point at a NEW location — e.g. a moved Proton Drive
    # folder or a fresh machine — is exactly the scenario this guards). Never throws; missing
    # fields fall back to Get-GpbDefaultConfig's values (same field-completion Read-GpbConfig does).
    [CmdletBinding()] param()
    $p = Get-GpbConfigPath
    $default = Get-GpbDefaultConfig
    if (-not (Test-Path -LiteralPath $p)) { return $default }
    try { $cfg = Get-Content -LiteralPath $p -Raw | ConvertFrom-Json -ErrorAction Stop }
    catch {
        # Fail-visible, not fail-silent: an existing-but-unparseable config.json must never be
        # silently discarded — Initialize's later locked write would otherwise overwrite it with
        # fresh defaults, quietly losing the Repos registry. Quarantine (same pattern as
        # Read-GpbMarker's marker quarantine) preserves the evidence and the old Repos list for
        # manual recovery; the original is left in place (Initialize's own write replaces it).
        $msg = "config.json is unreadable — quarantining it and starting fresh (your repo registry may need re-registering)"
        $quarantine = "$p.$([guid]::NewGuid().ToString('N').Substring(0,6)).bad"
        try {
            Copy-Item -LiteralPath $p -Destination $quarantine -Force -ErrorAction Stop
        } catch {
            # The warning must never lie about evidence existing — if the copy itself failed,
            # say so, rather than silently claiming a '.bad' file is sitting there when it isn't.
            $msg += " (quarantine copy failed: $($_.Exception.Message))"
        }
        Write-Warning $msg
        return $default
    }
    foreach ($k in $default.PSObject.Properties.Name) {
        if (-not $cfg.PSObject.Properties[$k]) { $cfg | Add-Member -NotePropertyName $k -NotePropertyValue $default.$k -Force }
    }
    $cfg
}

function Find-GpbSyncRoot {
    # Discovery per spec: $env:USERPROFILE\Proton Drive, plus its one-level subdirectories,
    # searched for a 'My files' child — that's the actual local folder that mirrors cloud
    # '/my-files' (Proton Drive's desktop client lays out sync roots as
    # "Proton Drive\<account>\My files"), so IT (not its parent) is what ProtonDriveRoot must be
    # for Get-CloudBundlePath's mapping and for BackupSubdir to actually land inside the synced
    # tree. The deepest hit wins (a subdirectory's 'My files' beats the top level's, on the rare
    # chance both exist). When the base folder exists but no 'My files' is found anywhere, degrade
    # to the base folder itself — caller warns that cloud-path verification may be degraded.
    # Returns $null when nothing is discoverable at all.
    [CmdletBinding()] param()
    $base = Join-Path $env:USERPROFILE 'Proton Drive'
    if (-not (Test-Path -LiteralPath $base -PathType Container)) { return $null }

    $hits = New-Object System.Collections.Generic.List[object]
    $topMyFiles = Join-Path $base 'My files'
    if (Test-Path -LiteralPath $topMyFiles -PathType Container) {
        $hits.Add([pscustomobject]@{ Path = $topMyFiles; Depth = 0 })
    }
    foreach ($d in @(Get-ChildItem -LiteralPath $base -Directory -ErrorAction SilentlyContinue)) {
        $sub = Join-Path $d.FullName 'My files'
        if (Test-Path -LiteralPath $sub -PathType Container) {
            $hits.Add([pscustomobject]@{ Path = $sub; Depth = 1 })
        }
    }
    if ($hits.Count -gt 0) {
        $deepest = ($hits | Sort-Object Depth -Descending | Select-Object -First 1).Path
        return [pscustomobject]@{ Root = $deepest; Degraded = $false }
    }
    [pscustomobject]@{ Root = $base; Degraded = $true }
}

function Initialize-ProtonBackup {
    # Guided first-run: discover/accept the Proton Drive sync folder, resolve the CLI (never
    # fatal if it's missing or unauthenticated — that's a degraded mode, not a blocker), prove the
    # sync folder is actually writable (fatal — without that, nothing this tool does will work),
    # and write config while preserving anything already registered (Repos, prior customizations).
    # Design: nothing before the lock reads config — discovery uses the parameter/filesystem, CLI
    # resolution uses the parameter/Get-Command, and all three probes use those locals plus the
    # DEFAULT backup-subdir name. The single tolerant config read (which can quarantine an
    # unparseable config.json — see Read-GpbConfigOrDefault) happens exactly once, immediately
    # before the write, both inside the lock. This keeps the lock narrow (no subprocess/probe time
    # held while it's taken) while closing the lost-update window a pre-lock read would otherwise
    # open across the whole probe sequence (a concurrent Set-ProtonBackupConfig write during CLI
    # probing would get clobbered by a stale pre-lock snapshot).
    [CmdletBinding()]
    param(
        [string]$ProtonDriveRoot,
        [string]$ProtonCli,
        [scriptblock]$AuthProbe,
        [scriptblock]$WriteProbe,
        [scriptblock]$InfoProbe
    )

    if (-not $ProtonDriveRoot) {
        $found = Find-GpbSyncRoot
        if (-not $found) {
            throw "No Proton Drive sync folder found under '$(Join-Path $env:USERPROFILE 'Proton Drive')' — pass -ProtonDriveRoot explicitly."
        }
        $ProtonDriveRoot = $found.Root
        if ($found.Degraded) {
            Write-Warning "Proton Drive folder found, but no 'My files' subfolder was located under it — bundles may not upload to Proton Drive AT ALL until this folder is confirmed to actually be synced (not just present). If you know the real synced folder, pass it explicitly with -ProtonDriveRoot instead of relying on this fallback."
        }
    }
    if (-not (Test-Path -LiteralPath $ProtonDriveRoot -PathType Container)) {
        throw "ProtonDriveRoot '$ProtonDriveRoot' does not exist."
    }
    $ProtonDriveRoot = (Resolve-Path -LiteralPath $ProtonDriveRoot).Path

    # --- CLI resolution: absent is a warning, never fatal ---
    $cliArg  = if ($PSBoundParameters.ContainsKey('ProtonCli')) { $ProtonCli } else { 'proton-drive' }
    $cliPath = $cliArg
    if ($cliPath -and -not [System.IO.Path]::IsPathRooted($cliPath)) {
        $cmd = Get-Command -Name $cliPath -ErrorAction SilentlyContinue
        if ($cmd) { $cliPath = $cmd.Source }
    }
    $cliResolved = [bool]$cliPath -and (Test-Path -LiteralPath $cliPath -ErrorAction SilentlyContinue)
    $cliReady = $false

    if (-not $cliResolved) {
        Write-Warning "Proton Drive CLI not found (looked for '$cliArg') — sign-in and cloud-upload verification will be skipped until it's installed and on PATH."
    } else {
        $probe = if ($PSBoundParameters.ContainsKey('AuthProbe')) { $AuthProbe } else { { param($cli) & $cli filesystem list '/my-files' 2>&1 | Out-Null; $LASTEXITCODE } }
        $authResult = & $probe $cliPath
        if ($authResult -ne 0) {
            Write-Warning 'CLI present but not signed in'
        } else {
            $cliReady = $true
        }
    }

    # --- Backup subdir + write probe (fatal — the sync folder must be genuinely writable) ---
    # No config read here (see the function-level design note) — uses the DEFAULT subdir name.
    # This is what gets recorded when there's no pre-existing customization; if a customized
    # BackupSubdir is preserved below (under the lock), that folder gets its OWN re-probe there —
    # so either way, whatever ends up recorded in config was actually validated writable.
    $backupSubdir = (Get-GpbDefaultConfig).BackupSubdir
    $backupDir = Join-Path $ProtonDriveRoot $backupSubdir
    if (-not (Test-Path -LiteralPath $backupDir)) { New-Item -ItemType Directory -Path $backupDir -Force | Out-Null }

    if ($WriteProbe) {
        & $WriteProbe $backupDir
    } else {
        $probeFile = Join-Path $backupDir '.gpb-probe'
        Set-Content -LiteralPath $probeFile -Value 'gpb-write-probe' -NoNewline -ErrorAction Stop
        Remove-Item -LiteralPath $probeFile -Force -ErrorAction Stop
    }

    # --- Cloud info probe: only when the CLI is ready, and never fatal either way — the sync
    #     app simply may not have uploaded the freshly-created folder yet. -InfoProbe is a test
    #     seam (receives the mapped cloud path, then the resolved CLI path); default invokes the
    #     real CLI's `filesystem info` and treats a nonzero/absent exit code as the same warning.
    if ($cliReady) {
        $cloudPath = Get-CloudBundlePath -LocalPath $backupDir -DriveLocalRoot $ProtonDriveRoot
        try {
            $infoProbeSb = if ($PSBoundParameters.ContainsKey('InfoProbe')) { $InfoProbe } else { { param($cp, $cli) & $cli filesystem info $cp 2>&1 | Out-Null; $LASTEXITCODE } }
            $infoResult = & $infoProbeSb $cloudPath $cliPath
            if ($infoResult -ne 0) {
                Write-Warning 'cloud path not yet visible — the sync app may not have uploaded it yet'
            }
        } catch {
            Write-Warning 'cloud path not yet visible — the sync app may not have uploaded it yet'
        }
    }

    # --- Config: single tolerant read-modify-write, ALL under the lock — the only config access
    #     in this whole function. Preserves everything not touched here (Repos, retention/verify
    #     settings, an existing ProtonCli the user set via Set-...) — INCLUDING a previously
    #     customized BackupSubdir: silently resetting it back to the default would silently move
    #     where future bundles land, which this project treats as an unacceptable silent behavior
    #     change. When the preserved value differs from the default folder already probed above,
    #     re-probe THAT folder here instead (filesystem-only — mkdir + write/delete a probe file —
    #     so it's fine to do under the lock; the CLI info probe is NOT re-run for it, since it's
    #     warning-only and the default path already exercised it above). ---
    $lock = Wait-GpbLock -TimeoutSeconds 15
    if (-not $lock) { throw 'Another GitProtonBackup operation holds the lock — retry shortly.' }
    try {
        $cfg = Read-GpbConfigOrDefault
        $cfg.ProtonDriveRoot = $ProtonDriveRoot
        if ($cfg.BackupSubdir -and $cfg.BackupSubdir -ne $backupSubdir) {
            $customDir = Join-Path $ProtonDriveRoot $cfg.BackupSubdir
            if (-not (Test-Path -LiteralPath $customDir)) { New-Item -ItemType Directory -Path $customDir -Force | Out-Null }
            if ($WriteProbe) {
                & $WriteProbe $customDir
            } else {
                $probeFile2 = Join-Path $customDir '.gpb-probe'
                Set-Content -LiteralPath $probeFile2 -Value 'gpb-write-probe' -NoNewline -ErrorAction Stop
                Remove-Item -LiteralPath $probeFile2 -Force -ErrorAction Stop
            }
        } else {
            $cfg.BackupSubdir = $backupSubdir
        }
        if ($PSBoundParameters.ContainsKey('ProtonCli')) { $cfg.ProtonCli = $ProtonCli }
        Write-GpbConfig -Config $cfg
    } finally { $lock.Close(); Remove-Item -LiteralPath (Get-GpbLockPath) -Force -ErrorAction SilentlyContinue }

    Write-Host 'Note: git push proton backs up COMMITTED work only — uncommitted changes are never included.'
}

function Invoke-ProtonBackupVerify {
    # The reconciliation backstop (spec: docs/design.md — dead-man's-switch). Runs on a schedule
    # (or on demand) independent of any push: re-derives the actual digest per registered repo and
    # re-cuts a bundle whenever coverage is stale — this is what heals a broken/uninstalled hook,
    # not just what clears a marker. Every exit path (config failure, lock contention, or normal
    # completion) writes the durable last-verify.json + verify.log AND best-effort pings the
    # heartbeat URL — the whole point of a dead-man's switch is that it never goes silent.
    [CmdletBinding()]
    param([scriptblock]$SyncCheck, [scriptblock]$InfoRunner, [scriptblock]$CliReadyRunner, [scriptblock]$WebRunner, [int]$LockTimeoutSeconds = 30)
    $findings = [System.Collections.Generic.List[string]]@()
    $repoResults = [System.Collections.Generic.List[object]]@()
    $exit = 0
    $complete = $false
    $incompleteReason = ''
    try { $cfg = Read-GpbConfig } catch {
        # Hard config failure: STILL write the durable report and best-effort /fail heartbeat
        # (review finding: silence on exit 2 defeats the dead-man's switch's purpose).
        Write-Warning $_.Exception.Message
        $report = [pscustomobject]@{ Timestamp = (Get-Date).ToUniversalTime().ToString('o'); ExitCode = 2; Complete = $false; IncompleteReason = 'config'; Repos = @(); Error = $_.Exception.Message }
        Write-GpbJsonAtomic -Path (Join-Path (Get-GpbRoot) 'last-verify.json') -Object $report
        Add-Content -LiteralPath (Join-Path (Get-GpbRoot) 'verify.log') -Value "$($report.Timestamp) exit=2 config-error"
        $rawHb = try { (Get-Content (Get-GpbConfigPath) -Raw -ErrorAction Stop | ConvertFrom-Json -ErrorAction Stop).HeartbeatUrl } catch { $null }
        if ($rawHb) {
            $runner = if ($WebRunner) { $WebRunner } else { { param($u) Invoke-RestMethod -Uri $u -Method Get -TimeoutSec 10 | Out-Null } }
            try { & $runner "$rawHb/fail" } catch { Write-Warning "heartbeat ping failed: $($_.Exception.Message)" }
        }
        return [pscustomobject]@{ ExitCode = 2; Complete = $false; IncompleteReason = 'config'; Findings = @($_.Exception.Message); Repos = @() }
    }

    $lock = Wait-GpbLock -TimeoutSeconds $LockTimeoutSeconds
    if (-not $lock) {
        $findings.Add('lock unavailable — another GitProtonBackup operation is running'); $exit = 1
        $incompleteReason = 'lock'
    } else {
      try {
        $cli = if ($cfg.ProtonCli) { $cfg.ProtonCli } else { 'proton-drive' }
        # Fault-isolated: a throwing -CliReadyRunner (or a misbehaving real probe) must degrade to
        # "CLI unavailable", not escape past the report/heartbeat writes below — this whole
        # function exists to never go silent, including on its own internal failures.
        $cliReady = $false
        try {
            $cliReady = if ($CliReadyRunner) { & $CliReadyRunner } else { Test-ProtonCliReady -CliPath $cli }
        } catch {
            $findings.Add("CLI readiness probe failed: $($_.Exception.Message) — degraded verification (Cloud Files sync state only)")
            $exit = [Math]::Max($exit, 1)
        }
        if (-not $cliReady) { Write-Warning 'Proton CLI unavailable or not signed in — degraded verification (Cloud Files sync state only).' }
        # GetNewClosure() below detaches the resulting scriptblock's SessionState from this
        # module — it captures the *variables* it references, but bareword calls to non-exported
        # module functions (Get-CloudBundlePath, Confirm-BundleUploaded, Get-CloudFileSyncState)
        # then fail to resolve at invocation time ("... is not recognized ..."), because every
        # test seams past this real branch with -SyncCheck/-CliReadyRunner and never exercised it
        # for real. Capture references to those private functions as variables (which DO survive
        # the closure) and invoke through the references instead of by name.
        $getCloudBundlePathFn    = ${function:Get-CloudBundlePath}
        $confirmBundleUploadedFn = ${function:Confirm-BundleUploaded}
        $getCloudFileSyncStateFn = ${function:Get-CloudFileSyncState}
        $effectiveCheck = {
            param($p)
            if ($cliReady) {
                $cloudPath = & $getCloudBundlePathFn -LocalPath $p -DriveLocalRoot $cfg.ProtonDriveRoot
                $c = if ($InfoRunner) { & $confirmBundleUploadedFn -CloudPath $cloudPath -CliPath $cli -InfoRunner $InfoRunner }
                     else             { & $confirmBundleUploadedFn -CloudPath $cloudPath -CliPath $cli }
                $c.Confirmed
            } elseif ($SyncCheck) { & $SyncCheck $p } else { (& $getCloudFileSyncStateFn -Path $p).InSync }
        }.GetNewClosure()

        foreach ($repo in @($cfg.Repos)) {
            $rf = [System.Collections.Generic.List[string]]@(); $state = 'ok'
            try {
                if (-not (Test-Path -LiteralPath $repo)) { $rf.Add("registered repo missing on disk: $repo — Uninstall-ProtonBackup to deregister"); $state = 'attention' }
                else {
                    $m = Test-GpbMirror -RepoPath $repo
                    if (-not ($m.HasRemote -and $m.MirrorExists -and $m.HookExists -and $m.WorkRepoOk)) {
                        $rf.Add("wiring broken for $repo — run Repair-ProtonBackup"); $state = 'attention'
                    }
                    # Digest reconciliation: re-cut when coverage is stale, marker or not.
                    $bd = Get-GpbBundleDir -Config $cfg -RepoPath $repo
                    $res = Invoke-RepoBundleBackup -RepoPath $repo -BundleDir $bd -BundleBaseName (Split-Path $repo -Leaf) `
                        -SyncCheck $effectiveCheck -RetentionKeep $cfg.RetentionKeep -RetentionCheckpoints $cfg.RetentionCheckpoints
                    foreach ($f in @($res.Findings)) { $rf.Add("$($f.Kind): $($f.Detail)"); $state = 'attention' }
                    if ($res.State -ne 'backed_up') { $rf.Add("newest bundle not confirmed on Proton for $repo"); $state = 'attention' }
                    elseif ($res.PSObject.Properties['BundlePath'] -and $res.BundlePath) {
                        # Read BEFORE removing: a malformed marker under this slug must be
                        # quarantined (Read-GpbMarker renames it .bad), never deleted unseen.
                        $mkFile = Get-Item -LiteralPath (Join-Path (Get-GpbMarkerDir) "$(Get-GpbSlug -RepoPath $repo).json") -ErrorAction SilentlyContinue
                        if ($mkFile) {
                            if (Read-GpbMarker -File $mkFile) { Remove-PushPendingMarker -RepoPath $repo }
                            else { $rf.Add("unreadable marker quarantined for $repo (renamed with a .bad suffix)"); $state = 'attention' }
                        }
                    }
                    # Spool guard.
                    $allBundles = @(Get-ChildItem -LiteralPath $bd -Filter '*.bundle' -ErrorAction SilentlyContinue | Sort-Object LastWriteTime)
                    $newest = $allBundles | Select-Object -Last 1
                    if ($newest -and $res.State -ne 'backed_up' -and $newest.LastWriteTime -lt (Get-Date).AddDays(-[int]$cfg.MaxUnconfirmedAgeDays)) {
                        $rf.Add("bundle unconfirmed for over $($cfg.MaxUnconfirmedAgeDays) day(s) — is the Proton Drive app running?"); $state = 'attention'
                    }
                    if ($res.State -ne 'backed_up' -and $allBundles.Count -gt [int]$cfg.RetentionKeep) {
                        $rf.Add("$($allBundles.Count) bundles spooling unconfirmed (retention can't prune until one confirms)"); $state = 'attention'
                    }
                }
            } catch { $rf.Add("verify failed for ${repo}: $($_.Exception.Message)"); $state = 'attention' }
            if ($state -eq 'attention') { $exit = [Math]::Max($exit, 1) }
            $repoResults.Add([pscustomobject]@{ RepoPath = $repo; State = $state; Findings = $rf.ToArray() })
            foreach ($x in $rf) { $findings.Add($x) }
        }

        # Marker pass: anything the repo loop didn't clear. Fault-isolated the same way as the
        # CLI-readiness probe above — neither a bad enumeration nor a single bad marker file may
        # propagate out of the function and skip the report/heartbeat below.
        $registered = @($cfg.Repos)
        $markerFiles = @()
        $markerDir = Get-GpbMarkerDir
        # Get-GpbMarkerDir is created lazily (only when the first marker is written) — its
        # absence is the ordinary "no pending markers" state, not a fault, and must not be
        # reported as one. Only an enumeration failure against an EXISTING directory (permissions,
        # I/O, etc.) is a genuine fault worth a finding.
        if (Test-Path -LiteralPath $markerDir -PathType Container) {
            try {
                $markerFiles = @(Get-ChildItem -LiteralPath $markerDir -Filter '*.json' -ErrorAction Stop)
            } catch {
                $findings.Add("marker pass skipped: enumeration failed: $($_.Exception.Message)")
                $exit = [Math]::Max($exit, 1)
            }
        }
        foreach ($file in $markerFiles) {
            try {
                $mk = Read-GpbMarker -File $file
                # Read-GpbMarker's quarantine name carries a random suffix ("$($file.Name).<hex>.bad")
                # so repeated corruptions never overwrite earlier evidence — naming an exact filename
                # here would name one that doesn't exist, so this only states what happened.
                if (-not $mk) { $findings.Add("quarantined unreadable marker: $($file.Name) (renamed with a .bad suffix)"); $exit = [Math]::Max($exit, 1); continue }
                if (($registered -notcontains $mk.RepoPath) -and -not (Test-Path -LiteralPath $mk.RepoPath)) {
                    Remove-Item -LiteralPath $file.FullName -Force -ErrorAction SilentlyContinue
                    $findings.Add("evicted orphaned marker for $($mk.RepoPath)")
                    continue
                }
                $findings.Add("pending backup not confirmed (reason: $($mk.Reason)) for $($mk.RepoPath)")
                $exit = [Math]::Max($exit, 1)
            } catch {
                $findings.Add("marker check failed ($($file.Name)): $($_.Exception.Message)")
                $exit = [Math]::Max($exit, 1)
            }
        }

        $complete = $true
      } catch {
        # Never-go-silent backstop (issue #2): the inner fault isolations cover the expected
        # failure modes, but a throw OUTSIDE those islands would previously escape this
        # catchless try/finally — releasing the lock, then skipping the report/heartbeat tail
        # entirely, leaving last-verify.json silently stale. Convert it to an incomplete run
        # instead; the tail below still reports and pings.
        $findings.Add("verify pass failed unexpectedly: $($_.Exception.GetBaseException().Message)")
        $exit = [Math]::Max($exit, 1)
        $incompleteReason = 'error'
      } finally { $lock.Close(); Remove-Item (Get-GpbLockPath) -Force -ErrorAction SilentlyContinue }
    }

    $report = [pscustomobject]@{ Timestamp = (Get-Date).ToUniversalTime().ToString('o'); ExitCode = $exit; Complete = $complete; IncompleteReason = $incompleteReason; Repos = $repoResults.ToArray() }
    Write-GpbJsonAtomic -Path (Join-Path (Get-GpbRoot) 'last-verify.json') -Object $report
    Add-Content -LiteralPath (Join-Path (Get-GpbRoot) 'verify.log') -Value "$($report.Timestamp) exit=$exit findings=$($findings.Count)"

    if ($cfg.HeartbeatUrl) {
        $hb = if ($exit -eq 0) { $cfg.HeartbeatUrl } else { "$($cfg.HeartbeatUrl)/fail" }
        $runner = if ($WebRunner) { $WebRunner } else { { param($u) Invoke-RestMethod -Uri $u -Method Get -TimeoutSec 10 | Out-Null } }
        try { & $runner $hb } catch { Write-Warning "heartbeat ping failed: $($_.Exception.Message)" }
    }
    foreach ($x in $findings) { Write-Host "- $x" }
    [pscustomobject]@{ ExitCode = $exit; Complete = $complete; IncompleteReason = $incompleteReason; Findings = $findings.ToArray(); Repos = $repoResults.ToArray() }
}

function Get-ProtonBackupConfig {
    [CmdletBinding()] param()
    Read-GpbConfig
}

function Set-ProtonBackupConfig {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$Key,
        [Parameter(Mandatory)][object]$Value
    )
    $default = Get-GpbDefaultConfig
    if ($default.PSObject.Properties.Name -notcontains $Key) {
        throw "Set-ProtonBackupConfig: unknown config key '$Key'."
    }

    switch ($Key) {
        'Repos' { throw "Set-ProtonBackupConfig: Repos is managed by Install-ProtonBackup/Uninstall-ProtonBackup — not settable directly." }
        'ProtonDriveRoot' {
            if (-not (Test-Path -LiteralPath $Value -PathType Container)) {
                throw "Set-ProtonBackupConfig: ProtonDriveRoot '$Value' does not exist."
            }
        }
        'BackupSubdir' {
            # Containment: the resolved join must remain UNDER the CURRENT ProtonDriveRoot —
            # rejects '..\' escapes that would write backups outside the Proton Drive sync tree.
            $cfgNow  = Read-GpbConfig
            $root    = ([System.IO.Path]::GetFullPath($cfgNow.ProtonDriveRoot)).TrimEnd('\', '/')
            $full    = [System.IO.Path]::GetFullPath((Join-Path $cfgNow.ProtonDriveRoot $Value))
            $isUnder = $full.Equals($root, [System.StringComparison]::OrdinalIgnoreCase) -or
                       $full.StartsWith("$root\", [System.StringComparison]::OrdinalIgnoreCase) -or
                       $full.StartsWith("$root/", [System.StringComparison]::OrdinalIgnoreCase)
            if (-not $isUnder) {
                throw "Set-ProtonBackupConfig: BackupSubdir '$Value' must resolve to a path under ProtonDriveRoot ('$($cfgNow.ProtonDriveRoot)')."
            }
        }
        'ProtonCli' {
            if ($Value) {
                $resolved = [string]$Value
                if (-not [System.IO.Path]::IsPathRooted($resolved)) {
                    $cmd = Get-Command -Name $resolved -ErrorAction SilentlyContinue
                    if ($cmd) { $resolved = $cmd.Source }
                }
                if (-not (Test-Path -LiteralPath $resolved)) {
                    throw "Set-ProtonBackupConfig: ProtonCli '$Value' does not resolve to an executable."
                }
            }
        }
        { $_ -in 'VerifySeconds', 'RetentionKeep', 'RetentionCheckpoints', 'MaxUnconfirmedAgeDays' } {
            $n = 0
            if (-not [int]::TryParse([string]$Value, [ref]$n) -or $n -le 0) {
                throw "Set-ProtonBackupConfig: $Key must be a positive integer."
            }
            $Value = $n
        }
        'HeartbeatUrl' {
            if ($Value -ne '' -and -not ([string]$Value).StartsWith('http')) {
                throw "Set-ProtonBackupConfig: HeartbeatUrl must be '' or start with 'http'."
            }
        }
    }

    $lock = Wait-GpbLock -TimeoutSeconds 15
    if (-not $lock) { throw 'Another GitProtonBackup operation holds the lock — retry shortly.' }
    try {
        $cfg = Read-GpbConfig
        $cfg.$Key = $Value
        Write-GpbConfig -Config $cfg
    } finally { $lock.Close(); Remove-Item -LiteralPath (Get-GpbLockPath) -Force -ErrorAction SilentlyContinue }
}

# --- Public commands: Status + scheduled-task installer ----------------------

function Get-ProtonBackupStatus {
    # Read-only snapshot per registered repo. NEVER calls the Proton CLI, and never bundles —
    # that's Invoke-ProtonBackupVerify's job. CurrentBundled replicates Invoke-RepoBundleBackup's
    # own cache-hit comparison (stamped .lastdigest == current digest AND the newest bundle
    # filename carries the current digest8) purely by reading state already on disk — LOCAL
    # coverage only. ConfirmedAtLastVerify and the LastVerify* fields come from last-verify.json,
    # the durable report Invoke-ProtonBackupVerify writes on every run (including its own
    # failure paths) — never freshly probed here. One caveat to "read-only": reading a pending
    # marker goes through Read-GpbMarker, which quarantines (renames .bad) a marker file it can't
    # parse as a side effect of being read — so a corrupt marker can still be mutated by a Status
    # call, even though nothing here writes config, bundles, or the registry.
    [CmdletBinding()]
    param([switch]$Json)

    $cfg = Read-GpbConfig

    $verifyPath = Join-Path (Get-GpbRoot) 'last-verify.json'
    $lastVerify = $null
    if (Test-Path -LiteralPath $verifyPath) {
        try { $lastVerify = Get-Content -LiteralPath $verifyPath -Raw | ConvertFrom-Json -ErrorAction Stop }
        catch { $lastVerify = $null }
    }
    $lastVerifyExitCode = if ($lastVerify -and $lastVerify.PSObject.Properties['ExitCode']) { $lastVerify.ExitCode } else { $null }
    $lastVerifyAgeHours = $null
    if ($lastVerify -and $lastVerify.PSObject.Properties['Timestamp'] -and $lastVerify.Timestamp) {
        try {
            $ts = [datetime]$lastVerify.Timestamp
            $lastVerifyAgeHours = ((Get-Date).ToUniversalTime() - $ts.ToUniversalTime()).TotalHours
        } catch { $lastVerifyAgeHours = $null }
    }

    $results = [System.Collections.Generic.List[object]]::new()
    foreach ($repo in @($cfg.Repos)) {
        $wiring = Test-GpbMirror -RepoPath $repo
        $wiringOk = [bool]($wiring.HasRemote -and $wiring.MirrorExists -and $wiring.HookExists -and $wiring.WorkRepoOk)

        # Same cache-hit comparison as Invoke-RepoBundleBackup (digest8, newest-bundle-carries-it) —
        # read-only here: no bundling, no cloud probe.
        $baseName  = Split-Path $repo -Leaf
        $bundleDir = Get-GpbBundleDir -Config $cfg -RepoPath $repo
        $digest    = Get-RepoRefDigest -RepoPath $repo
        $stateFile = Join-Path $bundleDir ".$baseName.lastdigest"
        $lastDigest = if (Test-Path -LiteralPath $stateFile) { (Get-Content -LiteralPath $stateFile -Raw).Trim() } else { '' }
        $digest8 = if ($digest.Length -ge 8) { $digest.Substring(0, 8).ToLowerInvariant() } else { $digest.ToLowerInvariant() }
        $newest = (Get-ChildItem -LiteralPath $bundleDir -Filter "$baseName-*.bundle" -ErrorAction SilentlyContinue |
            Sort-Object LastWriteTime | Select-Object -Last 1)
        $newestIsCurrent = [bool]($newest -and $newest.Name -match "-$([regex]::Escape($digest8))\.bundle$")
        $currentBundled = [bool](($digest -eq $lastDigest) -and $newestIsCurrent)

        $confirmedAtLastVerify = 'never-verified'
        if ($lastVerify -and $lastVerify.PSObject.Properties['Repos']) {
            $entry = @($lastVerify.Repos) | Where-Object { $_.RepoPath -eq $repo } | Select-Object -First 1
            if ($entry) { $confirmedAtLastVerify = $entry.State }
        }

        $dirtyCount = @(git -C $repo status --porcelain=v1 2>$null).Count

        $pendingMarker = ''
        $pendingMarkerAgeHours = $null
        $markerFile = Join-Path (Get-GpbMarkerDir) "$(Get-GpbSlug -RepoPath $repo).json"
        $markerItem = Get-Item -LiteralPath $markerFile -ErrorAction SilentlyContinue
        if ($markerItem) {
            $mk = Read-GpbMarker -File $markerItem
            if ($mk) {
                $pendingMarker = $mk.Reason
                $pendingMarkerAgeHours = ((Get-Date).ToUniversalTime() - $markerItem.LastWriteTimeUtc).TotalHours
            }
        }

        $results.Add([pscustomobject]@{
            RepoPath              = $repo
            WiringOk              = $wiringOk
            CurrentBundled        = $currentBundled
            ConfirmedAtLastVerify = $confirmedAtLastVerify
            DirtyCount            = $dirtyCount
            PendingMarker         = $pendingMarker
            PendingMarkerAgeHours = $pendingMarkerAgeHours
            NewestBundle          = if ($newest) { $newest.Name } else { '' }
            LastVerifyAgeHours    = $lastVerifyAgeHours
            LastVerifyExitCode    = $lastVerifyExitCode
        })
    }

    if ($Json) { return ($results.ToArray() | ConvertTo-Json -Depth 6 -AsArray) }
    # Table default: a curated, human-width column set — PowerShell's default formatter falls
    # back to a vertical list view once an object has more than ~4 properties, which would defeat
    # the "table by default" contract for this 10-field object. The omitted fields
    # (PendingMarkerAgeHours, NewestBundle, LastVerifyExitCode) remain fully available via -Json.
    $results.ToArray() | Format-Table -AutoSize RepoPath, WiringOk, CurrentBundled, ConfirmedAtLastVerify, DirtyCount, PendingMarker, LastVerifyAgeHours
}

function Install-ProtonBackupTask {
    # Builds a PLAIN data hashtable first (no CIM/ScheduledTask objects constructed on the path
    # tests exercise — the Task Scheduler provider can be unavailable in sandboxed/CI contexts,
    # and tests must never depend on it). The default -Register seam is where real
    # New-ScheduledTask* objects get built, strictly AFTER the plain data is assembled — swap it
    # out via -Register/-Unregister for testing.
    [CmdletBinding()]
    param(
        [string]$At = '12:30',
        [switch]$Uninstall,
        [scriptblock]$Register,
        [scriptblock]$Unregister
    )
    $taskName = 'GitProtonBackup Verify'

    if ($Uninstall) {
        $unreg = if ($Unregister) { $Unregister } else { { param($n) Unregister-ScheduledTask -TaskName $n -Confirm:$false } }
        & $unreg $taskName
        return
    }

    $data = @{
        TaskName           = $taskName
        At                 = $At
        LogonType          = 'Interactive'
        Execute            = 'pwsh'
        Arguments          = '-NoProfile -WindowStyle Hidden -Command "Import-Module GitProtonBackup; exit (Invoke-ProtonBackupVerify).ExitCode"'
        StartWhenAvailable = $true
    }

    $reg = if ($Register) { $Register } else {
        {
            param($p)
            # Missed runs (machine off at 12:30) fire after the next logon — StartWhenAvailable.
            $principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive
            $trigger   = New-ScheduledTaskTrigger -Daily -At $p.At
            $action    = New-ScheduledTaskAction -Execute $p.Execute -Argument $p.Arguments
            $settings  = if ($p.StartWhenAvailable) { New-ScheduledTaskSettingsSet -StartWhenAvailable } else { New-ScheduledTaskSettingsSet }
            Register-ScheduledTask -TaskName $p.TaskName -Principal $principal -Trigger $trigger -Action $action -Settings $settings -Force | Out-Null
        }
    }
    & $reg $data
}
