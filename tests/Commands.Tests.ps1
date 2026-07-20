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
}
