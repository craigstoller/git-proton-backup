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
