BeforeAll {
    Import-Module "$PSScriptRoot/../GitProtonBackup/GitProtonBackup.psd1" -Force
    function New-Sandbox {
        $env:GPB_CONFIG_DIR = Join-Path $TestDrive "cfg-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        $env:GPB_LOCK_PATH  = Join-Path $TestDrive "lk-$([guid]::NewGuid().ToString('N').Substring(0,8)).lock"
        $env:GPB_HOOK_DISABLED = '1'
        $drive = Join-Path $TestDrive "drive-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        New-Item -ItemType Directory $drive -Force | Out-Null
        $cfg = Get-GpbDefaultConfig; $cfg.ProtonDriveRoot = $drive
        Write-GpbConfig -Config $cfg
        $drive
    }
    function Clear-Sandbox { Remove-Item Env:GPB_CONFIG_DIR, Env:GPB_LOCK_PATH, Env:GPB_HOOK_DISABLED -ErrorAction SilentlyContinue }
    function New-TestRepo {
        $r = Join-Path $TestDrive "repo-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        New-Item -ItemType Directory $r -Force | Out-Null
        git -C $r init -qb main; git -C $r config user.email 't@t'; git -C $r config user.name 't'
        Set-Content "$r/a.txt" 'one'; git -C $r add .; git -C $r commit -qm c1
        $r
    }

    # The production-install (hook ENABLED) test's shim spawns a child pwsh that does
    # `Import-Module GitProtonBackup` by bare name — the module's PARENT directory (the repo
    # root) must be on PSModulePath for that resolution to succeed (same pattern as
    # tests/PushBackupFlow.Tests.ps1's 'Invoke-ProtonBackupHook contract' BeforeAll).
    $script:repoRoot = (Resolve-Path "$PSScriptRoot/..").Path
    $script:priorPSModulePath = $env:PSModulePath
    $env:PSModulePath = "$script:repoRoot;$env:PSModulePath"
}

AfterAll {
    $env:PSModulePath = $script:priorPSModulePath
}

Describe 'Install/Uninstall/Repair' {
    BeforeEach { $script:drive = New-Sandbox; $script:repo = New-TestRepo }
    AfterEach  { Clear-Sandbox }

    It 'install wires the repo and registers it' {
        Install-ProtonBackup -RepoPath $script:repo
        (git -C $script:repo remote get-url proton) | Should -Be (Get-GpbMirrorPath -RepoPath $script:repo)
        @((Read-GpbConfig).Repos) | Should -Contain (Resolve-Path $script:repo).Path
    }
    It 'install refuses a shallow clone with an explanation' {
        $src = New-TestRepo
        Set-Content "$src/b.txt" 'two'; git -C $src add .; git -C $src commit -qm c2
        $shallow = Join-Path $TestDrive 'shallow'
        git clone -q --depth 1 "file:///$($src -replace '\\','/')" $shallow
        { Install-ProtonBackup -RepoPath $shallow } | Should -Throw '*shallow*'
    }
    It 'install warns (not refuses) on LFS/submodule hazards' {
        Set-Content "$script:repo/.gitattributes" '*.bin filter=lfs diff=lfs merge=lfs'
        git -C $script:repo add .; git -C $script:repo commit -qm lfs
        Install-ProtonBackup -RepoPath $script:repo -WarningVariable wv -WarningAction SilentlyContinue
        (@($wv) -join "`n") | Should -Match '(?i)lfs'
        @((Read-GpbConfig).Repos).Count | Should -Be 1
    }
    It 'uninstall reverses wiring, deregisters, and drops the pending marker' {
        Install-ProtonBackup -RepoPath $script:repo
        Write-PushPendingMarker -RepoPath $script:repo -Reason verify_timeout -BundleDir 'C:\B' -BundleBaseName 'x'
        Uninstall-ProtonBackup -RepoPath $script:repo
        git -C $script:repo remote get-url proton *> $null
        $LASTEXITCODE | Should -Not -Be 0
        @((Read-GpbConfig).Repos).Count | Should -Be 0
        @(Get-ChildItem (Get-GpbMarkerDir) -Filter '*.json' -ErrorAction SilentlyContinue).Count | Should -Be 0
    }
    It 'relative-path uninstall of an existing repo removes the absolute registry entry (consistency)' {
        Install-ProtonBackup -RepoPath $script:repo
        Push-Location (Split-Path $script:repo -Parent)
        try { Uninstall-ProtonBackup -RepoPath ".\$(Split-Path $script:repo -Leaf)" } finally { Pop-Location }
        git -C $script:repo remote get-url proton *> $null
        $LASTEXITCODE | Should -Not -Be 0
        @((Read-GpbConfig).Repos).Count | Should -Be 0
    }
    It 'deleted-repo uninstall (absolute path) removes mirror + registry entry' {
        Install-ProtonBackup -RepoPath $script:repo
        $mirror = Get-GpbMirrorPath -RepoPath $script:repo
        Remove-Item $script:repo -Recurse -Force
        Uninstall-ProtonBackup -RepoPath $script:repo
        Test-Path $mirror | Should -BeFalse
        @((Read-GpbConfig).Repos).Count | Should -Be 0
    }
    It 'deleted-repo uninstall via RELATIVE path also works' {
        Install-ProtonBackup -RepoPath $script:repo
        $mirror = Get-GpbMirrorPath -RepoPath $script:repo
        $leaf = Split-Path $script:repo -Leaf
        Remove-Item $script:repo -Recurse -Force
        Push-Location $TestDrive
        try { Uninstall-ProtonBackup -RepoPath ".\$leaf" } finally { Pop-Location }
        Test-Path $mirror | Should -BeFalse
        @((Read-GpbConfig).Repos).Count | Should -Be 0
    }
    It 'repair re-creates a deleted mirror' {
        Install-ProtonBackup -RepoPath $script:repo
        Remove-Item (Get-GpbMirrorPath -RepoPath $script:repo) -Recurse -Force
        Repair-ProtonBackup -RepoPath $script:repo
        Test-Path (Join-Path (Get-GpbMirrorPath -RepoPath $script:repo) 'HEAD') | Should -BeTrue
    }
    It 'install without config points at Initialize' {
        Remove-Item (Get-GpbConfigPath) -Force
        { Install-ProtonBackup -RepoPath $script:repo } | Should -Throw '*Initialize-ProtonBackup*'
    }
    It '-Force replaces a foreign proton remote wiring without touching its target' {
        $foreign = Join-Path $TestDrive 'foreign.git'; git init --bare -q $foreign
        git -C $script:repo remote add proton $foreign
        { Install-ProtonBackup -RepoPath $script:repo } | Should -Throw '*-Force*'
        Install-ProtonBackup -RepoPath $script:repo -Force
        (git -C $script:repo remote get-url proton) | Should -Be (Get-GpbMirrorPath -RepoPath $script:repo)
        Test-Path (Join-Path $foreign 'HEAD') | Should -BeTrue   # foreign target untouched
    }
    It 'repair recovers a MOVED repo (old-slug mirror repointed, old mirror cleaned)' {
        Install-ProtonBackup -RepoPath $script:repo
        $oldMirror = Get-GpbMirrorPath -RepoPath $script:repo
        $moved = Join-Path $TestDrive "moved-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        Move-Item $script:repo $moved
        Repair-ProtonBackup -RepoPath $moved
        (git -C $moved remote get-url proton) | Should -Be (Get-GpbMirrorPath -RepoPath $moved)
        Test-Path $oldMirror | Should -BeFalse   # delete-safe cleanup of the old-slug mirror
    }
    It 'relative-path install slugs identically to the absolute path (canonical identity)' {
        Push-Location (Split-Path $script:repo -Parent)
        try { Install-ProtonBackup -RepoPath ".\$(Split-Path $script:repo -Leaf)" } finally { Pop-Location }
        (git -C $script:repo remote get-url proton) | Should -Be (Get-GpbMirrorPath -RepoPath $script:repo)
    }
    It 'production install (hook ENABLED) does not self-contend on the lock' {
        Remove-Item Env:GPB_HOOK_DISABLED -ErrorAction SilentlyContinue
        # module must be resolvable by the shim's Import-Module: PSModulePath prepend per Task 6's pattern
        $out = Install-ProtonBackup -RepoPath $script:repo 6>&1
        ($out -join "`n") | Should -Not -Match 'deferred'
        @(Get-ChildItem (Get-GpbMarkerDir) -Filter '*.json' -ErrorAction SilentlyContinue |
            Where-Object { (Read-GpbMarker -File $_)?.Reason -eq 'deferred_lock' }).Count | Should -Be 0
    }
}

Describe 'Initialize + Config' {
    BeforeEach {
        $env:GPB_CONFIG_DIR = Join-Path $TestDrive "init-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        $env:GPB_LOCK_PATH  = Join-Path $TestDrive "init-lk-$([guid]::NewGuid().ToString('N').Substring(0,8)).lock"
        $script:drive = Join-Path $TestDrive "pd-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        New-Item -ItemType Directory $script:drive -Force | Out-Null
    }
    AfterEach { Remove-Item Env:GPB_CONFIG_DIR, Env:GPB_LOCK_PATH -ErrorAction SilentlyContinue }

    It 'initialize writes a valid config and creates the backup subdir' {
        $out = Initialize-ProtonBackup -ProtonDriveRoot $script:drive -AuthProbe { 0 } -InfoProbe { 0 } 6>&1
        ($out -join "`n") | Should -Match 'COMMITTED work only'
        (Read-GpbConfig).ProtonDriveRoot | Should -Be $script:drive
        Test-Path (Join-Path $script:drive 'GitBackups') | Should -BeTrue
    }
    It 'initialize preserves the registry on re-run' {
        Initialize-ProtonBackup -ProtonDriveRoot $script:drive -AuthProbe { 0 } -InfoProbe { 0 } | Out-Null
        $cfg = Read-GpbConfig; $cfg.Repos = @('C:\P\x'); Write-GpbConfig -Config $cfg
        Initialize-ProtonBackup -ProtonDriveRoot $script:drive -AuthProbe { 0 } -InfoProbe { 0 } | Out-Null
        @((Read-GpbConfig).Repos) | Should -Contain 'C:\P\x'
    }
    It 'initialize warns (not throws) when the CLI is absent or unauthenticated' {
        $out = Initialize-ProtonBackup -ProtonDriveRoot $script:drive -ProtonCli 'no-such-cli-anywhere' -WarningVariable wv -WarningAction SilentlyContinue
        (@($wv) -join "`n") | Should -Match '(?i)cli'
        (Read-GpbConfig).ProtonDriveRoot | Should -Be $script:drive
    }
    It 'initialize without a discoverable root throws asking for the parameter' {
        # The real %USERPROFILE%\Proton Drive exists on the dev machine (it does not on CI
        # runners) — point discovery at an empty $TestDrive for the scope of this test only.
        $priorUserProfile = $env:USERPROFILE
        $env:USERPROFILE = $TestDrive
        try {
            { Initialize-ProtonBackup } | Should -Throw '*-ProtonDriveRoot*'
        } finally {
            $env:USERPROFILE = $priorUserProfile
        }
    }
    It 'Set-ProtonBackupConfig validates values' {
        Initialize-ProtonBackup -ProtonDriveRoot $script:drive -AuthProbe { 0 } -InfoProbe { 0 } | Out-Null
        { Set-ProtonBackupConfig -Key ProtonDriveRoot -Value 'C:\no\such' } | Should -Throw
        { Set-ProtonBackupConfig -Key VerifySeconds -Value -5 } | Should -Throw
        Set-ProtonBackupConfig -Key HeartbeatUrl -Value 'https://hc-ping.com/abc'
        (Get-ProtonBackupConfig).HeartbeatUrl | Should -Be 'https://hc-ping.com/abc'
    }
    It 'BackupSubdir containment: a ..\ escape is rejected' {
        Initialize-ProtonBackup -ProtonDriveRoot $script:drive -AuthProbe { 0 } -InfoProbe { 0 } | Out-Null
        { Set-ProtonBackupConfig -Key BackupSubdir -Value '..\outside' } | Should -Throw '*under*'
    }
    It 'initialize write-probe fails loud on an unwritable sync folder' {
        $ro = Join-Path $TestDrive 'ro-drive'; New-Item -ItemType Directory $ro -Force | Out-Null
        # Simulate unwritability via an injected probe failure rather than real ACLs (CI-safe):
        { Initialize-ProtonBackup -ProtonDriveRoot $ro -AuthProbe { 0 } -WriteProbe { throw 'denied' } } | Should -Throw '*denied*'
    }
    It 'discovery: a one-level subdir with My files wins, and ProtonDriveRoot is set to that folder' {
        # Pins the inferred discovery semantics: ProtonDriveRoot ends up as the 'My files' folder
        # ITSELF (not its parent) — that's what Get-CloudBundlePath/BackupSubdir need, since Proton
        # Drive's desktop client syncs "Proton Drive\<account>\My files" 1:1 with cloud '/my-files'.
        $up = Join-Path $TestDrive 'up'
        $acctMyFiles = Join-Path $up 'Proton Drive\acct123\My files'
        New-Item -ItemType Directory $acctMyFiles -Force | Out-Null
        $priorUserProfile = $env:USERPROFILE
        $env:USERPROFILE = $up
        try {
            Initialize-ProtonBackup -AuthProbe { 0 } -InfoProbe { 0 } | Out-Null
        } finally {
            $env:USERPROFILE = $priorUserProfile
        }
        (Read-GpbConfig).ProtonDriveRoot | Should -Match ([regex]::Escape('\My files') + '$')
    }
    It 'corrupt config is quarantined, warned about, and re-initialized' {
        Set-Content -LiteralPath (Get-GpbConfigPath) -Value '{broken' -NoNewline
        Initialize-ProtonBackup -ProtonDriveRoot $script:drive -AuthProbe { 0 } -InfoProbe { 0 } -WarningVariable wv -WarningAction SilentlyContinue | Out-Null
        (@($wv) -join "`n") | Should -Match '(?i)quarantin'
        @(Get-ChildItem -LiteralPath (Split-Path (Get-GpbConfigPath) -Parent) -Filter 'config.json.*.bad').Count | Should -Be 1
        (Read-GpbConfig).ProtonDriveRoot | Should -Be $script:drive
    }
    It 're-init preserves a customized BackupSubdir and probes it' {
        $script:writeProbeCalls = 0
        $wp = {
            param($dir)
            $script:writeProbeCalls++
            $f = Join-Path $dir '.gpb-probe'
            Set-Content -LiteralPath $f -Value 'x' -NoNewline -ErrorAction Stop
            Remove-Item -LiteralPath $f -Force -ErrorAction Stop
        }
        Initialize-ProtonBackup -ProtonDriveRoot $script:drive -AuthProbe { 0 } -InfoProbe { 0 } -WriteProbe $wp | Out-Null
        Set-ProtonBackupConfig -Key BackupSubdir -Value 'MyBundles'
        Initialize-ProtonBackup -ProtonDriveRoot $script:drive -AuthProbe { 0 } -InfoProbe { 0 } -WriteProbe $wp | Out-Null
        (Read-GpbConfig).BackupSubdir | Should -Be 'MyBundles'
        Test-Path (Join-Path $script:drive 'MyBundles') | Should -BeTrue
        # 1st Initialize (default dir) + 2nd Initialize's pre-lock default-dir probe + 2nd
        # Initialize's under-lock re-probe of the preserved custom subdir = 3 invocations.
        $script:writeProbeCalls | Should -Be 3
    }
}

Describe 'Invoke-ProtonBackupVerify (reconciliation)' {
    BeforeEach { $script:drive = New-Sandbox; $script:repo = New-TestRepo; Install-ProtonBackup -RepoPath $script:repo }
    AfterEach  { Clear-Sandbox }

    It 'broken-hook case: stale coverage is re-cut with NO marker present' {
        # New commit; hook disabled (sandbox), so no bundle covers c2 and no marker exists.
        Set-Content "$script:repo/a.txt" 'two'; git -C $script:repo add .; git -C $script:repo commit -qm c2
        $r = Invoke-ProtonBackupVerify -SyncCheck { param($p) $true } -CliReadyRunner { $false }
        $r.ExitCode | Should -Be 0
        $bd = Get-GpbBundleDir -Config (Read-GpbConfig) -RepoPath $script:repo
        # The CURRENT digest must be covered — a bundle merely existing is not enough
        # (review finding: an install-time bundle would false-pass a weaker assertion).
        $digest8 = (Get-RepoRefDigest -RepoPath $script:repo).Substring(0,8).ToLowerInvariant()
        $newest = Get-ChildItem $bd -Filter '*.bundle' | Sort-Object LastWriteTime | Select-Object -Last 1
        $newest.Name | Should -Match "-$digest8\.bundle$"
        (Get-Content (Join-Path (Get-GpbRoot) 'last-verify.json') -Raw | ConvertFrom-Json).ExitCode | Should -Be 0
    }
    It 'config failure (exit 2) still writes the report and pings /fail' {
        Set-ProtonBackupConfig -Key HeartbeatUrl -Value 'https://hc.example/uuid'
        $cfgRaw = Get-Content (Get-GpbConfigPath) -Raw | ConvertFrom-Json
        $cfgRaw.ProtonDriveRoot = 'C:\no\such\dir'
        Write-GpbJsonAtomic -Path (Get-GpbConfigPath) -Object $cfgRaw   # invalid root, parseable JSON
        $script:pinged = @()
        $r = Invoke-ProtonBackupVerify -WebRunner { param($u) $script:pinged += $u } -WarningAction SilentlyContinue
        $r.ExitCode | Should -Be 2
        (Get-Content (Join-Path (Get-GpbRoot) 'last-verify.json') -Raw | ConvertFrom-Json).ExitCode | Should -Be 2
        $script:pinged[-1] | Should -Be 'https://hc.example/uuid/fail'
    }
    It 'unconfirmed newest bundle → exit 1 + finding; marker kept' {
        Write-PushPendingMarker -RepoPath $script:repo -Reason verify_timeout `
            -BundleDir (Get-GpbBundleDir -Config (Read-GpbConfig) -RepoPath $script:repo) -BundleBaseName (Split-Path $script:repo -Leaf)
        $r = Invoke-ProtonBackupVerify -SyncCheck { param($p) $false } -CliReadyRunner { $false }
        $r.ExitCode | Should -Be 1
        @(Get-ChildItem (Get-GpbMarkerDir) -Filter '*.json').Count | Should -Be 1
    }
    It 'confirmed + digest-current clears the marker' {
        git -C $script:repo push proton 2>&1 | Out-Null   # hook disabled: refs move, no bundle
        Invoke-ProtonBackupVerify -SyncCheck { param($p) $true } -CliReadyRunner { $false } | Out-Null   # reconciles + confirms
        Write-PushPendingMarker -RepoPath $script:repo -Reason verify_timeout `
            -BundleDir (Get-GpbBundleDir -Config (Read-GpbConfig) -RepoPath $script:repo) -BundleBaseName (Split-Path $script:repo -Leaf)
        $r = Invoke-ProtonBackupVerify -SyncCheck { param($p) $true } -CliReadyRunner { $false }
        $r.ExitCode | Should -Be 0
        @(Get-ChildItem (Get-GpbMarkerDir) -Filter '*.json').Count | Should -Be 0
    }
    It 'orphaned marker (repo gone + not registered) is evicted with a finding' {
        $gone = New-TestRepo
        Write-PushPendingMarker -RepoPath $gone -Reason cli_unready -BundleDir 'C:\B' -BundleBaseName 'g'
        Remove-Item $gone -Recurse -Force
        $r = Invoke-ProtonBackupVerify -SyncCheck { param($p) $true } -CliReadyRunner { $false }
        ($r.Findings -join "`n") | Should -Match '(?i)evicted|orphan'
        @(Get-ChildItem (Get-GpbMarkerDir) -Filter '*.json').Count | Should -Be 0
    }
    It 'heartbeat: success pings url, attention pings url/fail, network failure never alters exit code' {
        Set-ProtonBackupConfig -Key HeartbeatUrl -Value 'https://hc.example/uuid'
        $script:pinged = @()
        Invoke-ProtonBackupVerify -SyncCheck { param($p) $true } -CliReadyRunner { $false } -WebRunner { param($u) $script:pinged += $u } | Out-Null
        $script:pinged[-1] | Should -Be 'https://hc.example/uuid'
        Write-PushPendingMarker -RepoPath $script:repo -Reason auth_error `
            -BundleDir (Get-GpbBundleDir -Config (Read-GpbConfig) -RepoPath $script:repo) -BundleBaseName (Split-Path $script:repo -Leaf)
        Set-Content "$script:repo/a.txt" 'x'; git -C $script:repo add .; git -C $script:repo commit -qm cX
        $r = Invoke-ProtonBackupVerify -SyncCheck { param($p) $false } -CliReadyRunner { $false } -WebRunner { param($u) $script:pinged += $u }
        $script:pinged[-1] | Should -Be 'https://hc.example/uuid/fail'
        $r2 = Invoke-ProtonBackupVerify -SyncCheck { param($p) $false } -CliReadyRunner { $false } -WebRunner { param($u) throw 'net down' } -WarningAction SilentlyContinue
        $r2.ExitCode | Should -Be $r.ExitCode
    }
    It 'no heartbeat configured → no web call' {
        $script:pinged = @()
        Invoke-ProtonBackupVerify -SyncCheck { param($p) $true } -CliReadyRunner { $false } -WebRunner { param($u) $script:pinged += $u } | Out-Null
        @($script:pinged).Count | Should -Be 0
    }
    It 'spool guard: old unconfirmed bundle raises an age finding' {
        Invoke-ProtonBackupVerify -SyncCheck { param($p) $false } -CliReadyRunner { $false } | Out-Null
        $bd = Get-GpbBundleDir -Config (Read-GpbConfig) -RepoPath $script:repo
        Get-ChildItem $bd -Filter '*.bundle' | ForEach-Object { $_.LastWriteTime = (Get-Date).AddDays(-10) }
        $r = Invoke-ProtonBackupVerify -SyncCheck { param($p) $false } -CliReadyRunner { $false }
        ($r.Findings -join "`n") | Should -Match '(?i)unconfirmed for'
    }
    It 'lock unavailable → finding + exit 1 + report and /fail heartbeat still fire' {
        Set-ProtonBackupConfig -Key HeartbeatUrl -Value 'https://hc.example/uuid'
        $holder = Wait-GpbLock -TimeoutSeconds 1
        try {
            $script:pinged = @()
            $r = Invoke-ProtonBackupVerify -LockTimeoutSeconds 1 -WebRunner { param($u) $script:pinged += $u }
            $r.ExitCode | Should -Be 1
            ($r.Findings -join "`n") | Should -Match 'lock unavailable'
            (Get-Content (Join-Path (Get-GpbRoot) 'last-verify.json') -Raw | ConvertFrom-Json).ExitCode | Should -Be 1
            $script:pinged[-1] | Should -Match '/fail$'
        } finally { $holder.Close() }
    }
}
