BeforeAll { Import-Module "$PSScriptRoot/../GitProtonBackup/GitProtonBackup.psd1" -Force }

Describe 'State foundation' {
    BeforeEach {
        $env:GPB_CONFIG_DIR = Join-Path $TestDrive "gpb-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        $env:GPB_LOCK_PATH  = Join-Path $TestDrive "lk-$([guid]::NewGuid().ToString('N').Substring(0,8)).lock"
    }
    AfterEach { Remove-Item Env:GPB_CONFIG_DIR, Env:GPB_LOCK_PATH -ErrorAction SilentlyContinue }

    It 'Get-GpbRoot honors GPB_CONFIG_DIR and creates it' {
        $r = Get-GpbRoot
        $r | Should -Be $env:GPB_CONFIG_DIR
        Test-Path $r | Should -BeTrue
    }
    It 'Write-GpbJsonAtomic leaves no temp files and round-trips' {
        $p = Join-Path (Get-GpbRoot) 'x.json'
        Write-GpbJsonAtomic -Path $p -Object ([pscustomobject]@{ a = 1 })
        (Get-Content $p -Raw | ConvertFrom-Json).a | Should -Be 1
        @(Get-ChildItem (Get-GpbRoot) -Filter '*.tmp-*').Count | Should -Be 0
    }
    It 'Read-GpbConfig throws with Initialize guidance when config is missing' {
        { Read-GpbConfig } | Should -Throw '*Initialize-ProtonBackup*'
    }
    It 'Read-GpbConfig throws when ProtonDriveRoot does not exist on disk' {
        $cfg = [pscustomobject]@{ ProtonDriveRoot = 'C:\no\such\dir'; BackupSubdir='GitBackups'; ProtonCli='';
            VerifySeconds=60; RetentionKeep=5; RetentionCheckpoints=24; MaxUnconfirmedAgeDays=7; HeartbeatUrl=''; Repos=@() }
        Write-GpbJsonAtomic -Path (Get-GpbConfigPath) -Object $cfg
        { Read-GpbConfig } | Should -Throw '*ProtonDriveRoot*'
    }
    It 'config round-trips through Write-GpbConfig/Read-GpbConfig' {
        $root = Join-Path $TestDrive 'drive'; New-Item -ItemType Directory $root -Force | Out-Null
        $cfg = [pscustomobject]@{ ProtonDriveRoot=$root; BackupSubdir='GitBackups'; ProtonCli='';
            VerifySeconds=60; RetentionKeep=5; RetentionCheckpoints=24; MaxUnconfirmedAgeDays=7; HeartbeatUrl=''; Repos=@() }
        Write-GpbConfig -Config $cfg
        (Read-GpbConfig).ProtonDriveRoot | Should -Be $root
        @((Read-GpbConfig).Repos).Count | Should -Be 0
    }
    It 'Get-GpbSlug: leaf + 10-hex hash, collision-proof across same-leaf paths' {
        $a = Get-GpbSlug -RepoPath 'C:\P\alpha\hub'
        $b = Get-GpbSlug -RepoPath 'C:\P\beta\hub'
        $a | Should -Match '^hub-[0-9a-f]{10}$'
        $a | Should -Not -Be $b
    }
    It 'Wait-GpbLock: acquires, blocks a second waiter, warns on non-contention failure only' {
        $fs = Wait-GpbLock -TimeoutSeconds 1
        try { Wait-GpbLock -TimeoutSeconds 1 -PollMs 100 -WarningVariable wv -WarningAction SilentlyContinue | Should -BeNullOrEmpty
              @($wv).Count | Should -Be 0 } finally { $fs.Close() }
        $env:GPB_LOCK_PATH = Join-Path $TestDrive 'no-such-dir\nested\x.lock'
        Wait-GpbLock -TimeoutSeconds 0 -WarningVariable wv2 -WarningAction SilentlyContinue | Should -BeNullOrEmpty
        @($wv2).Count | Should -Be 1
    }
    It 'derived paths: marker dir, mirror path, bundle dir are slug-keyed under the right roots' {
        $root = Join-Path $TestDrive 'drive'; New-Item -ItemType Directory $root -Force | Out-Null
        $cfg = [pscustomobject]@{ ProtonDriveRoot=$root; BackupSubdir='GitBackups'; ProtonCli='';
            VerifySeconds=60; RetentionKeep=5; RetentionCheckpoints=24; MaxUnconfirmedAgeDays=7; HeartbeatUrl=''; Repos=@() }
        $repo = 'C:\P\alpha\hub'; $slug = Get-GpbSlug -RepoPath $repo
        Get-GpbMarkerDir            | Should -Be (Join-Path (Get-GpbRoot) 'push-pending')
        Get-GpbMirrorPath -RepoPath $repo | Should -Be (Join-Path (Get-GpbRoot) "mirrors\$slug.git")
        Get-GpbBundleDir -Config $cfg -RepoPath $repo | Should -Be (Join-Path $root "GitBackups\$slug")
    }
}
