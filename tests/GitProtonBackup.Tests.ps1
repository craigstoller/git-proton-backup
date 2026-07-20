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

Describe 'Get-RepoRefDigest' {
    BeforeAll {
        $env:GPB_CONFIG_DIR = Join-Path $TestDrive "gpb-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        $env:GPB_LOCK_PATH  = Join-Path $TestDrive "lk-$([guid]::NewGuid().ToString('N').Substring(0,8)).lock"
        $repo = Join-Path $TestDrive 'digestrepo'
        New-Item -ItemType Directory -Path $repo -Force | Out-Null
        git -C $repo init -q
        git -C $repo config user.email 't@t'; git -C $repo config user.name 't'
        Set-Content "$repo/a.txt" 'one'; git -C $repo add .; git -C $repo commit -qm 'c1'
    }
    AfterAll { Remove-Item Env:GPB_CONFIG_DIR, Env:GPB_LOCK_PATH -ErrorAction SilentlyContinue }

    It 'changes when a new commit is added' {
        $d1 = Get-RepoRefDigest -RepoPath $repo
        Set-Content "$repo/a.txt" 'two'; git -C $repo add .; git -C $repo commit -qm 'c2'
        $d2 = Get-RepoRefDigest -RepoPath $repo
        $d2 | Should -Not -Be $d1
    }
    It 'changes when a tag is added (not just HEAD)' {
        $d1 = Get-RepoRefDigest -RepoPath $repo
        git -C $repo tag v1
        (Get-RepoRefDigest -RepoPath $repo) | Should -Not -Be $d1
    }
    It 'ignores remote-tracking refs (proton bookkeeping)' {
        $d1 = Get-RepoRefDigest -RepoPath $repo
        git -C $repo update-ref refs/remotes/proton/main HEAD
        (Get-RepoRefDigest -RepoPath $repo) | Should -Be $d1
        git -C $repo update-ref -d refs/remotes/proton/main
    }
}

Describe 'Invoke-RepoBundleBackup canonical ref set' {
    BeforeAll {
        $env:GPB_CONFIG_DIR = Join-Path $TestDrive "gpb-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        $env:GPB_LOCK_PATH  = Join-Path $TestDrive "lk-$([guid]::NewGuid().ToString('N').Substring(0,8)).lock"
    }
    AfterAll { Remove-Item Env:GPB_CONFIG_DIR, Env:GPB_LOCK_PATH -ErrorAction SilentlyContinue }

    It 'bundles only HEAD, heads, and tags — never remote-tracking refs' {
        $repo = Join-Path $TestDrive 'refset-repo'
        New-Item -ItemType Directory -Path $repo -Force | Out-Null
        git -C $repo init -qb main
        git -C $repo config user.email 't@t'; git -C $repo config user.name 't'
        Set-Content "$repo/a.txt" 'one'; git -C $repo add .; git -C $repo commit -qm 'c1'
        git -C $repo tag v1
        git -C $repo update-ref refs/remotes/proton/main HEAD
        $bd = Join-Path $TestDrive 'refset-bundles'
        $res = Invoke-RepoBundleBackup -RepoPath $repo -BundleDir $bd -BundleBaseName 'refset-repo' -SyncCheck { param($p) $true }
        $res.BundlePath | Should -Not -BeNullOrEmpty
        $heads = git bundle list-heads $res.BundlePath
        ($heads | Where-Object { $_ -match 'refs/remotes/' }) | Should -BeNullOrEmpty
        ($heads | Where-Object { $_ -match 'refs/heads/' })   | Should -Not -BeNullOrEmpty
        ($heads | Where-Object { $_ -match 'refs/tags/v1' })  | Should -Not -BeNullOrEmpty
    }
}

Describe 'Invoke-RepoBundleBackup fail-closed publication' {
    BeforeEach {
        $env:GPB_CONFIG_DIR = Join-Path $TestDrive "gpb-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        $env:GPB_LOCK_PATH  = Join-Path $TestDrive "lk-$([guid]::NewGuid().ToString('N').Substring(0,8)).lock"
        $script:repo = Join-Path $TestDrive "fc-repo-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        New-Item -ItemType Directory -Path $script:repo -Force | Out-Null
        git -C $script:repo init -qb main
        git -C $script:repo config user.email 't@t'; git -C $script:repo config user.name 't'
        Set-Content "$script:repo/a.txt" 'one'; git -C $script:repo add .; git -C $script:repo commit -qm 'c1'
        $script:bd = Join-Path $TestDrive "fc-bundles-$([guid]::NewGuid().ToString('N').Substring(0,8))"
    }
    AfterEach { Remove-Item Env:GPB_CONFIG_DIR, Env:GPB_LOCK_PATH -ErrorAction SilentlyContinue }

    It 'stamps the digest only after a successful publish' {
        $res = Invoke-RepoBundleBackup -RepoPath $script:repo -BundleDir $script:bd -BundleBaseName 'r' -SyncCheck { param($p) $true }
        $res.State | Should -Be 'backed_up'
        Test-Path (Join-Path $script:bd '.r.lastdigest') | Should -BeTrue
    }
    It 'an empty repo yields bundle_failed and no digest stamp (fail-visible)' {
        $empty = Join-Path $TestDrive 'fc-empty'
        New-Item -ItemType Directory -Path $empty -Force | Out-Null
        git -C $empty init -qb main
        git -C $empty config user.email 't@t'; git -C $empty config user.name 't'
        $res = Invoke-RepoBundleBackup -RepoPath $empty -BundleDir $script:bd -BundleBaseName 'e' -SyncCheck { param($p) $true }
        $res.State | Should -Be 'detected_not_backed_up'
        @($res.Findings | Where-Object { $_.Kind -eq 'bundle_failed' }).Count | Should -BeGreaterThan 0
        Test-Path (Join-Path $script:bd '.e.lastdigest') | Should -BeFalse
    }
    It 'a publish failure (target blocked) yields bundle_failed, no digest stamp, no partials left' {
        $digest = Get-RepoRefDigest -RepoPath $script:repo
        $d8 = $digest.Substring(0,8).ToLowerInvariant()
        $target = Join-Path $script:bd "r-20260719T000000Z-$d8.bundle"
        New-Item -ItemType Directory -Path $target -Force | Out-Null   # a DIRECTORY at the target path blocks Move-Item
        $res = Invoke-RepoBundleBackup -RepoPath $script:repo -BundleDir $script:bd -BundleBaseName 'r' -SyncCheck { param($p) $true } -Stamp '20260719T000000Z'
        $res.State | Should -Be 'detected_not_backed_up'
        @($res.Findings | Where-Object { $_.Kind -eq 'bundle_failed' -and $_.Detail -match 'publish' }).Count | Should -Be 1
        Test-Path (Join-Path $script:bd '.r.lastdigest') | Should -BeFalse
        @(Get-ChildItem $script:bd -Filter '*.partial').Count | Should -Be 0
    }
    It 'bundle filename carries the digest fragment (unique across same-second publishes)' {
        $res = Invoke-RepoBundleBackup -RepoPath $script:repo -BundleDir $script:bd -BundleBaseName 'r' -SyncCheck { param($p) $true } -Stamp '20260719T000000Z'
        Set-Content "$script:repo/a.txt" 'two'; git -C $script:repo add .; git -C $script:repo commit -qm 'c2'
        $res2 = Invoke-RepoBundleBackup -RepoPath $script:repo -BundleDir $script:bd -BundleBaseName 'r' -SyncCheck { param($p) $true } -Stamp '20260719T000000Z'
        $res2.BundlePath | Should -Not -Be $res.BundlePath
        (Split-Path $res.BundlePath -Leaf) | Should -Match '^r-20260719T000000Z-[0-9a-f]{8}\.bundle$'
    }
    It 'cache hit requires the CURRENT digest bundle — an older retained bundle does not satisfy it' {
        $res1 = Invoke-RepoBundleBackup -RepoPath $script:repo -BundleDir $script:bd -BundleBaseName 'r' -SyncCheck { param($p) $true } -Stamp '20260719T000001Z'
        Set-Content "$script:repo/a.txt" 'two'; git -C $script:repo add .; git -C $script:repo commit -qm 'c2'
        $res2 = Invoke-RepoBundleBackup -RepoPath $script:repo -BundleDir $script:bd -BundleBaseName 'r' -SyncCheck { param($p) $true } -Stamp '20260719T000002Z'
        Remove-Item -LiteralPath $res2.BundlePath -Force        # current bundle gone; older $res1 bundle remains
        $res3 = Invoke-RepoBundleBackup -RepoPath $script:repo -BundleDir $script:bd -BundleBaseName 'r' -SyncCheck { param($p) $true } -Stamp '20260719T000003Z'
        $res3.BundlePath | Should -Not -Be $res1.BundlePath      # re-created, not the stale one
        (Split-Path $res3.BundlePath -Leaf) | Should -Match "-$((Get-RepoRefDigest -RepoPath $script:repo).Substring(0,8).ToLowerInvariant())\.bundle$"
        $res3.State | Should -Be 'backed_up'
    }
    It 'prunes on a later call once the newest bundle confirms (delayed confirmation)' {
        for ($i = 1; $i -le 7; $i++) {
            Set-Content "$script:repo/a.txt" "v$i"; git -C $script:repo add .; git -C $script:repo commit -qm "c$i"
            Invoke-RepoBundleBackup -RepoPath $script:repo -BundleDir $script:bd -BundleBaseName 'r' `
                -SyncCheck { param($p) $false } -Stamp ('20260719T00000' + $i + 'Z') | Out-Null
        }
        @(Get-ChildItem $script:bd -Filter 'r-*.bundle').Count | Should -Be 7
        # No new commit: digest matches, nothing created — but now everything confirms → retention must fire
        Invoke-RepoBundleBackup -RepoPath $script:repo -BundleDir $script:bd -BundleBaseName 'r' `
            -SyncCheck { param($p) $true } | Out-Null
        @(Get-ChildItem $script:bd -Filter 'r-*.bundle').Count | Should -BeLessOrEqual 6   # keep-5 + possible monthly checkpoint
    }
    It 'detached HEAD defers with a visible finding (never silent)' {
        git -C $script:repo checkout -q --detach
        Set-Content "$script:repo/orphan.txt" 'x'; git -C $script:repo add .; git -C $script:repo commit -qm orphan
        $res = Invoke-RepoBundleBackup -RepoPath $script:repo -BundleDir $script:bd -BundleBaseName 'r' -SyncCheck { param($p) $true }
        $res.State | Should -Be 'detected_not_backed_up'
        (@($res.Findings) | ForEach-Object Detail) -join ' ' | Should -Match '(?i)detached'
    }
    It 'RetentionKeep is threaded: keep 2 prunes below the default gate' {
        for ($i = 1; $i -le 4; $i++) {
            Set-Content "$script:repo/a.txt" "v$i"; git -C $script:repo add .; git -C $script:repo commit -qm "c$i"
            Invoke-RepoBundleBackup -RepoPath $script:repo -BundleDir $script:bd -BundleBaseName 'r' `
                -SyncCheck { param($p) $true } -Stamp ('20260720T00000' + $i + 'Z') -RetentionKeep 2 | Out-Null
        }
        @(Get-ChildItem $script:bd -Filter 'r-*.bundle').Count | Should -BeLessOrEqual 3   # keep-2 + monthly checkpoint
    }
}

Describe 'Get-CloudBundlePath' {
    BeforeAll {
        $script:driveRoot = Join-Path $TestDrive 'MyFiles'
        New-Item -ItemType Directory -Path $script:driveRoot -Force | Out-Null
    }
    It 'maps a local bundle path under DriveLocalRoot to a cloud path' {
        $local = Join-Path $script:driveRoot 'Project Repo Bundles\CrowdFlow\X.bundle'
        $cloud = Get-CloudBundlePath -LocalPath $local -DriveLocalRoot $script:driveRoot
        $cloud | Should -Be '/my-files/Project Repo Bundles/CrowdFlow/X.bundle'
    }
    It 'uses a custom DriveCloudRoot when provided' {
        $local = Join-Path $script:driveRoot 'Folder\file.bundle'
        $cloud = Get-CloudBundlePath -LocalPath $local -DriveLocalRoot $script:driveRoot -DriveCloudRoot '/custom-root'
        $cloud | Should -Be '/custom-root/Folder/file.bundle'
    }
    It 'handles a path that does not exist on disk (no Resolve-Path error)' {
        $local = Join-Path $script:driveRoot 'Does\Not\Exist.bundle'
        # Should not throw — file may not exist yet
        { Get-CloudBundlePath -LocalPath $local -DriveLocalRoot $script:driveRoot } | Should -Not -Throw
    }
    It 'converts backslashes to forward slashes in the relative segment' {
        $local = Join-Path $script:driveRoot 'A\B\C\repo.bundle'
        $cloud = Get-CloudBundlePath -LocalPath $local -DriveLocalRoot $script:driveRoot
        $cloud | Should -Not -Match '\\'
    }
}

# Option A — Confirm-BundleUploaded
Describe 'Confirm-BundleUploaded -CliPath' {
    It 'passes the explicit CLI path to the runner (not module state)' {
        $script:seenCli = $null
        $r = Confirm-BundleUploaded -CloudPath '/my-files/x.bundle' -CliPath 'C:\custom\proton-drive.exe' -InfoRunner {
            param($cp, $cli)
            $script:seenCli = $cli
            [pscustomobject]@{ ExitCode = 0; Output = "state: 'active'" }
        }
        $r.Confirmed | Should -BeTrue
        $script:seenCli | Should -Be 'C:\custom\proton-drive.exe'
    }
}

Describe 'Push pending markers' {
    BeforeEach {
        $env:GPB_CONFIG_DIR = Join-Path $TestDrive "gpb-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        $script:mrepo = 'C:\P\marker-repo'
        $script:mbd   = Join-Path $TestDrive "marker-bundles-$([guid]::NewGuid().ToString('N').Substring(0,8))"
    }
    AfterEach { Remove-Item Env:GPB_CONFIG_DIR -ErrorAction SilentlyContinue }

    It 'a newer write overwrites the previous marker for the same repo' {
        Write-PushPendingMarker -RepoPath $script:mrepo -Reason cli_unready -BundleDir $script:mbd -BundleBaseName 'r'
        Write-PushPendingMarker -RepoPath $script:mrepo -Reason verify_timeout -BundleDir $script:mbd -BundleBaseName 'r'
        @(Get-ChildItem (Get-GpbMarkerDir) -Filter '*.json').Count | Should -Be 1
        ((Get-Content (Get-ChildItem (Get-GpbMarkerDir) -Filter '*.json').FullName -Raw | ConvertFrom-Json).Reason) | Should -Be 'verify_timeout'
    }
}

Describe 'Marker atomicity + quarantine' {
    BeforeEach { $env:GPB_CONFIG_DIR = Join-Path $TestDrive "mk-$([guid]::NewGuid().ToString('N').Substring(0,8))" }
    AfterEach  { Remove-Item Env:GPB_CONFIG_DIR -ErrorAction SilentlyContinue }
    It 'writes atomically (no .tmp residue) and round-trips' {
        Write-PushPendingMarker -RepoPath 'C:\P\x' -Reason verify_timeout -BundleDir 'C:\B' -BundleBaseName 'x'
        $f = Get-ChildItem (Get-GpbMarkerDir) -Filter '*.json'
        @($f).Count | Should -Be 1
        (Read-GpbMarker -File $f).Reason | Should -Be 'verify_timeout'
        @(Get-ChildItem (Get-GpbMarkerDir) -Filter '.tmp-*').Count | Should -Be 0
    }
    It 'quarantines malformed markers as .bad instead of deleting' {
        New-Item -ItemType Directory (Get-GpbMarkerDir) -Force | Out-Null
        Set-Content (Join-Path (Get-GpbMarkerDir) 'broken.json') '{not json'
        $f = Get-ChildItem (Get-GpbMarkerDir) -Filter 'broken.json'
        Read-GpbMarker -File $f | Should -BeNullOrEmpty
        # Quarantine name is unique (guid-suffixed) per Read-GpbMarker's contract — repeated
        # corruptions must not overwrite earlier evidence — so match via glob, not an exact name.
        @(Get-ChildItem (Get-GpbMarkerDir) -Filter 'broken.json.*.bad').Count | Should -Be 1
    }
    It 'Test-ProtonCliReady resolves a bare command name via Get-Command' {
        Test-ProtonCliReady -CliPath 'this-command-does-not-exist-anywhere' | Should -BeFalse
        Test-ProtonCliReady -CliPath 'git' -Runner { param($cli) 0 } | Should -BeTrue   # resolvable name + injected runner
    }
    It 'Get-CloudBundlePath refuses paths outside the drive root' {
        { Get-CloudBundlePath -LocalPath 'C:\elsewhere\x.bundle' -DriveLocalRoot 'C:\Drive' } | Should -Throw '*not under*'
    }
}
