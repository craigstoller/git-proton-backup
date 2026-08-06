<# Installs the GitProtonBackup module (v1) and, when an exe is present, the
   git-remote-proton helper (v2). Helper first: the module's already-installed
   throw must never block a helper-only run. #>
[CmdletBinding()]
param(
    [switch]$Force,
    [switch]$SkipPathUpdate,
    [string]$HelperExe = (Join-Path $PSScriptRoot 'git-remote-proton.exe'),
    # The PATH a FRESH shell would resolve, Machine then User in order — read-only,
    # used only for shadow detection below, never mutated. Overridable so tests can
    # supply a synthetic PATH built from TestDrive dirs instead of touching the real
    # registry (Stage 4 Task 6 review: an earlier draft's tests leaked GUID-temp
    # entries into the real user PATH doing exactly that).
    [string]$EffectivePath = (
        [Environment]::GetEnvironmentVariable('Path', 'Machine') + ';' +
        [Environment]::GetEnvironmentVariable('Path', 'User')
    )
)
$ErrorActionPreference = 'Stop'

# ---- helper (v2) ----
if (Test-Path $HelperExe) {
    $sidecar = "$HelperExe.sha256"
    if (Test-Path $sidecar) {
        $want = ((Get-Content $sidecar -Raw) -split '\s+')[0].Trim().ToLower()
        $got  = (Get-FileHash $HelperExe -Algorithm SHA256).Hash.ToLower()
        if ($want -ne $got) {
            throw "Checksum mismatch for $HelperExe — expected $want, got $got. Refusing to install the helper."
        }
    }
    $helperDir = Join-Path $env:LOCALAPPDATA 'Programs\git-proton-backup'
    New-Item -ItemType Directory -Force -Path $helperDir | Out-Null
    try {
        Copy-Item $HelperExe (Join-Path $helperDir 'git-remote-proton.exe') -Force
    } catch {
        throw "Cannot replace $helperDir\git-remote-proton.exe (a git process may be using it). Close running git commands and re-run. $_"
    }
    if ($SkipPathUpdate) {
        Write-Host "Skipping user PATH update (-SkipPathUpdate)."
    } else {
        # PATH check tolerates empty user PATHs (no leading semicolon) and
        # trailing-backslash variants (no duplicate entries) — peer-reviewed.
        $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
        $entries = @(($userPath -split ';') | Where-Object { $_ } | ForEach-Object { $_.TrimEnd('\') })
        if ($entries -notcontains $helperDir.TrimEnd('\')) {
            $newPath = if ([string]::IsNullOrEmpty($userPath)) { $helperDir } else { "$userPath;$helperDir" }
            [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
            Write-Host "Added $helperDir to your user PATH. Open a NEW terminal for it to take effect (this script cannot change its caller's session)."
        }
    }
    Write-Host "Helper installed: $helperDir\git-remote-proton.exe"

    # Stage 4 gate run 1 (BLOCKED at step 1.5): a stale git-remote-proton.exe already
    # earlier on PATH silently outran this fresh install — the script printed success
    # and exited 0 while git kept running the old binary. Detect that: walk the PATH a
    # fresh shell would resolve (Machine then User, in order) for the first
    # git-remote-proton.exe, and compare it to the one just installed. This check is
    # read-only and runs regardless of -SkipPathUpdate — that switch controls whether
    # PATH gets MUTATED, not whether shadowing gets checked.
    $installedExe = Join-Path $helperDir 'git-remote-proton.exe'
    $pathDirs = @(($EffectivePath -split ';') | Where-Object { $_ } | ForEach-Object { $_.TrimEnd('\') })
    $shadow = $null
    foreach ($dir in $pathDirs) {
        $candidate = Join-Path $dir 'git-remote-proton.exe'
        if (Test-Path -LiteralPath $candidate -PathType Leaf) {
            $shadow = $candidate
            break
        }
    }
    if ($shadow -and ($shadow.TrimEnd('\') -ne $installedExe.TrimEnd('\'))) {
        Write-Warning ("Another git-remote-proton.exe is earlier on PATH and shadows the copy " +
            "just installed: a fresh shell will run $shadow instead of $installedExe. Remove or " +
            "rename the shadowing copy, or reorder PATH so $helperDir comes first — until you " +
            "do, git keeps running the old binary.")
    }
} else {
    Write-Host "No git-remote-proton.exe found at $HelperExe — skipping helper install (module only)."
}

# ---- module (v1), semantics unchanged when the payload is present ----
# A release download is exe + sha256 + this script, with NO module payload
# beside it — that layout must install the helper and SKIP the module, not
# throw (peer-review blocker: the gate's install step is exactly this case).
$payload = Join-Path $PSScriptRoot 'GitProtonBackup'
if (Test-Path $payload) {
    $dest = Join-Path ([Environment]::GetFolderPath('MyDocuments')) 'PowerShell\Modules\GitProtonBackup'
    if ((Test-Path $dest) -and -not $Force) { throw "Already installed at $dest — re-run with -Force to overwrite." }
    New-Item -ItemType Directory -Path $dest -Force | Out-Null
    Copy-Item -Path (Join-Path $payload '*') -Destination $dest -Recurse -Force
    Write-Host "Installed. Start with: Import-Module GitProtonBackup; Initialize-ProtonBackup"
} else {
    Write-Host "No GitProtonBackup module payload beside the script — helper-only install. For the module, clone the repository and re-run."
}
