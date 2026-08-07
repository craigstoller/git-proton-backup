BeforeAll { Import-Module "$PSScriptRoot/../GitProtonBackup/GitProtonBackup.psm1" -Force }

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
    It 'confirms from the --json payload (activeRevision.value.state), not prose' {
        $r = Confirm-BundleUploaded -CloudPath '/my-files/x.bundle' -CliPath 'cli' -InfoRunner {
            param($cp, $cli)
            [pscustomobject]@{ ExitCode = 0; Output = '{"type":"file","activeRevision":{"ok":true,"value":{"uid":"a~b~c","state":"active","storageSize":120350}}}' }
        }
        $r.Confirmed | Should -BeTrue
        $r.Reason | Should -Be 'cli_confirmed'
    }
    It 'does NOT confirm when the json payload reports a non-active revision' {
        $r = Confirm-BundleUploaded -CloudPath '/my-files/x.bundle' -CliPath 'cli' -InfoRunner {
            param($cp, $cli)
            [pscustomobject]@{ ExitCode = 0; Output = '{"type":"file","activeRevision":{"ok":true,"value":{"uid":"a~b~c","state":"draft"}}}' }
        }
        $r.Confirmed | Should -BeFalse
        $r.Reason | Should -Be 'unverified'
    }
    It 'falls back to the human-readable match when the payload is not json (older CLI)' {
        $r = Confirm-BundleUploaded -CloudPath '/my-files/x.bundle' -CliPath 'cli' -InfoRunner {
            param($cp, $cli)
            [pscustomobject]@{ ExitCode = 0; Output = "uid: 'a~b~c',`n  state: 'active'," }
        }
        $r.Confirmed | Should -BeTrue
        $r.Reason | Should -Be 'cli_confirmed'
    }
    It 'keeps not_in_cloud and auth_error detection on the error paths' {
        (Confirm-BundleUploaded -CloudPath '/x' -CliPath 'cli' -InfoRunner {
            param($cp, $cli) [pscustomobject]@{ ExitCode = 1; Output = 'Node not found: x.bundle' } }).Reason |
            Should -Be 'not_in_cloud'
        (Confirm-BundleUploaded -CloudPath '/x' -CliPath 'cli' -InfoRunner {
            param($cp, $cli) [pscustomobject]@{ ExitCode = 1; Output = 'You need to login first' } }).Reason |
            Should -Be 'auth_error'
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

Describe 'GpbMirror lifecycle' {
    BeforeEach {
        $env:GPB_CONFIG_DIR = Join-Path $TestDrive "gpb-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        $env:GPB_HOOK_DISABLED = '1'
        $script:proot = Join-Path $TestDrive "pr-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        $script:prepo = Join-Path $script:proot 'repo'
        New-Item -ItemType Directory -Path $script:prepo -Force | Out-Null
        git -C $script:prepo init -qb main
        git -C $script:prepo config user.email 't@t'; git -C $script:prepo config user.name 't'
        Set-Content "$script:prepo/a.txt" 'one'; git -C $script:prepo add .; git -C $script:prepo commit -qm 'c1'
    }
    AfterEach { Remove-Item Env:GPB_CONFIG_DIR, Env:GPB_HOOK_DISABLED -ErrorAction SilentlyContinue }

    It 'install wires mirror, remote, refspecs, shim, config, initial push, upstream' {
        $r = Install-GpbMirror -RepoPath $script:prepo
        Test-Path (Join-Path $r.MirrorPath 'HEAD') | Should -BeTrue
        (git -C $script:prepo remote get-url proton) | Should -Be $r.MirrorPath
        @(git -C $script:prepo config --get-all remote.proton.push) | Should -Contain '+refs/heads/*:refs/heads/*'
        @(git -C $script:prepo config --get-all remote.proton.push) | Should -Contain '+refs/tags/*:refs/tags/*'
        (git -C $r.MirrorPath config gpb.workrepo) | Should -Be $script:prepo
        $shim = Get-Content (Join-Path $r.MirrorPath 'hooks/post-receive') -Raw
        $shim | Should -Match '^#!/bin/sh'
        $shim | Should -Not -Match "`r"                        # LF-only
        $shim | Should -Match 'Import-Module GitProtonBackup'
        $shim | Should -Match '(?m)^exit 0\s*$'                 # unconditional exit 0
        $shim | Should -Not -Match 'exec'                       # no exec
        $shim | Should -Match ([regex]::Escape('|| vs=0'))      # verifyseconds fallback pin — a vs=60 regression must not slip through
        (git -C $r.MirrorPath rev-parse refs/heads/main) | Should -Be (git -C $script:prepo rev-parse main)
        (git -C $script:prepo config branch.main.remote) | Should -Be 'proton'
        $h = Test-GpbMirror -RepoPath $script:prepo
        $h.HasRemote | Should -BeTrue; $h.MirrorExists | Should -BeTrue
        $h.HookExists | Should -BeTrue; $h.WorkRepoOk | Should -BeTrue
    }
    It 'install throws on a non-git RepoPath (side-effects-first: repo untouched)' {
        $plain = Join-Path $script:proot 'plain'
        New-Item -ItemType Directory -Path $plain -Force | Out-Null
        { Install-GpbMirror -RepoPath $plain } | Should -Throw '*not a git repository*'
    }
    It 'install is idempotent and repairs a deleted mirror' {
        $r1 = Install-GpbMirror -RepoPath $script:prepo
        Remove-Item -LiteralPath $r1.MirrorPath -Recurse -Force
        $r2 = Install-GpbMirror -RepoPath $script:prepo
        $r2.MirrorPath | Should -Be $r1.MirrorPath
        Test-Path (Join-Path $r2.MirrorPath 'HEAD') | Should -BeTrue
        @(git -C $script:prepo config --get-all remote.proton.push).Count | Should -Be 2
    }
    It 'install on a repo with no commits skips push/upstream but wires everything else' {
        $bare = Join-Path $script:proot 'empty-repo'
        New-Item -ItemType Directory -Path $bare -Force | Out-Null
        git -C $bare init -qb main
        $r = Install-GpbMirror -RepoPath $bare
        Test-Path (Join-Path $r.MirrorPath 'HEAD') | Should -BeTrue
        (git -C $bare remote get-url proton) | Should -Be $r.MirrorPath
    }
    It 'remove drops remote and mirror; health reports absent' {
        $r = Install-GpbMirror -RepoPath $script:prepo
        Remove-GpbMirror -RepoPath $script:prepo
        git -C $script:prepo remote get-url proton *> $null
        $LASTEXITCODE | Should -Not -Be 0
        Test-Path $r.MirrorPath | Should -BeFalse
        (Test-GpbMirror -RepoPath $script:prepo).HasRemote | Should -BeFalse
    }
    It 'remove refuses to delete a directory that is not our mirror' {
        $victim = Join-Path $TestDrive 'victim'
        New-Item -ItemType Directory -Path $victim -Force | Out-Null
        Set-Content (Join-Path $victim 'precious.txt') 'data'
        git -C $script:prepo remote add proton $victim
        Remove-GpbMirror -RepoPath $script:prepo -WarningVariable wv -WarningAction SilentlyContinue
        Test-Path (Join-Path $victim 'precious.txt') | Should -BeTrue   # untouched
        # v0.2.1 (issue #2): a FOREIGN remote is now left in place too — ownership-symmetric
        # with Install, which refuses foreign remotes rather than silently adopting them.
        (git -C $script:prepo remote get-url proton) | Should -Be $victim
        (@($wv) -join ' ') | Should -Match 'not a GitProtonBackup mirror'
    }
    It 'health flags a mirror wired to a DIFFERENT repo' {
        $r = Install-GpbMirror -RepoPath $script:prepo
        git -C $r.MirrorPath config gpb.workrepo (Join-Path $TestDrive 'somewhere-else')
        (Test-GpbMirror -RepoPath $script:prepo).WorkRepoOk | Should -BeFalse
    }

    # --- Task 5 deltas: foreign-remote refusal, upstream policy, ownership-equality removal ---

    It 'refuses a foreign proton remote' {
        git -C $script:prepo remote add proton (Join-Path $TestDrive 'foreign.git')
        { Install-GpbMirror -RepoPath $script:prepo } | Should -Throw '*not owned by GitProtonBackup*'
    }
    It 'never overwrites an existing upstream unless -SetUpstream' {
        git -C $script:prepo remote add origin (Join-Path $TestDrive 'origin.git')
        git -C $script:prepo config branch.main.remote origin
        git -C $script:prepo config branch.main.merge refs/heads/main
        Install-GpbMirror -RepoPath $script:prepo | Out-Null
        (git -C $script:prepo config branch.main.remote) | Should -Be 'origin'
        Install-GpbMirror -RepoPath $script:prepo -SetUpstream | Out-Null
        (git -C $script:prepo config branch.main.remote) | Should -Be 'proton'
    }
    It 'sets upstream when the branch has none' {
        Install-GpbMirror -RepoPath $script:prepo | Out-Null
        (git -C $script:prepo config branch.main.remote) | Should -Be 'proton'
    }
    It 'removal requires canonical ownership equality' {
        $r = Install-GpbMirror -RepoPath $script:prepo
        git -C $r.MirrorPath config gpb.workrepo 'C:\somewhere\else'
        Remove-GpbMirror -RepoPath $script:prepo -WarningAction SilentlyContinue
        Test-Path $r.MirrorPath | Should -BeTrue      # left in place
        git -C $script:prepo remote get-url proton *> $null
        $LASTEXITCODE | Should -Not -Be 0             # remote still removed
    }
}

Describe 'Confirm-BundleUploaded across CLI payload shapes' {
    # Regression: the CLI dropped the {ok,value} Result wrapper around activeRevision between
    # 0.4.6 and 0.7.0. Reading only the wrapped form made every healthy bundle read as
    # unconfirmed on 0.7.0 - and, because the success payload contains keyAuthor/nameAuthor,
    # the fall-through matched the bare substring 'auth' and reported auth_error, pointing the
    # user at a login problem that did not exist.

    # Pester 5: a Describe body runs at DISCOVERY, so fixtures declared there are $null when the
    # Its execute. They must live in BeforeAll. This is the same discovery/run split that has
    # bitten this project before, and it fails silently - the closure just captures nothing.
    BeforeAll {
        $script:wrapped   = '{"uid":"u1","name":{"ok":true,"value":"x.bundle"},"keyAuthor":{"ok":true,"value":"me@pm.me"},"nameAuthor":{"ok":true,"value":"me@pm.me"},"activeRevision":{"ok":true,"value":{"uid":"r1","state":"active","claimedSize":8}}}'
        $script:unwrapped = '{"uid":"u1","name":{"ok":true,"value":"x.bundle"},"keyAuthor":{"ok":true,"value":"me@pm.me"},"nameAuthor":{"ok":true,"value":"me@pm.me"},"contentAuthor":{"ok":true,"value":"me@pm.me"},"activeRevision":{"uid":"r1","state":"active","claimedSize":8}}'
    }

    It 'confirms the 0.4.6 wrapped shape' {
        $w = $script:wrapped; $runner = { param($cp,$cli) [pscustomobject]@{ ExitCode=0; Output=$w } }.GetNewClosure()
        (Confirm-BundleUploaded -CloudPath '/x' -CliPath 'p' -InfoRunner $runner).Confirmed | Should -BeTrue
    }
    It 'confirms the 0.7.0 unwrapped shape' {
        $u = $script:unwrapped; $runner = { param($cp,$cli) [pscustomobject]@{ ExitCode=0; Output=$u } }.GetNewClosure()
        $r = Confirm-BundleUploaded -CloudPath '/x' -CliPath 'p' -InfoRunner $runner
        $r.Confirmed | Should -BeTrue
        $r.Reason    | Should -Be 'cli_confirmed'
    }
    It 'does NOT report auth_error just because the payload contains keyAuthor/nameAuthor' {
        # exit 0 with a non-active state: must be 'unverified', never 'auth_error'
        $inactive = $script:unwrapped -replace '"state":"active"','"state":"draft"'
        $runner = { param($cp,$cli) [pscustomobject]@{ ExitCode=0; Output=$inactive } }.GetNewClosure()
        $r = Confirm-BundleUploaded -CloudPath '/x' -CliPath 'p' -InfoRunner $runner
        $r.Confirmed | Should -BeFalse
        $r.Reason    | Should -Not -Be 'auth_error'
    }
    It 'still reports auth_error on a genuine failed call' {
        $runner = { param($cp,$cli) [pscustomobject]@{ ExitCode=1; Output='Error: You need to login first' } }.GetNewClosure()
        (Confirm-BundleUploaded -CloudPath '/x' -CliPath 'p' -InfoRunner $runner).Reason | Should -Be 'auth_error'
    }
    It 'honours ok=false in the wrapped shape as no usable revision' {
        $bad = $script:wrapped -replace '"ok":true,"value":\{"uid":"r1","state":"active","claimedSize":8\}','"ok":false'
        $runner = { param($cp,$cli) [pscustomobject]@{ ExitCode=0; Output=$bad } }.GetNewClosure()
        (Confirm-BundleUploaded -CloudPath '/x' -CliPath 'p' -InfoRunner $runner).Confirmed | Should -BeFalse
    }
}

Describe 'Cloud Files placeholder state decoding' {
    # CF_PLACEHOLDER_STATE values per cfapi.h:
    #   NO_STATES 0x0  PLACEHOLDER 0x1  SYNC_ROOT 0x2  ESSENTIAL_PROP 0x4
    #   IN_SYNC   0x8  PARTIAL     0x10 PARTIALLY_ON_DISK 0x20  INVALID 0xffffffff

    It 'INVALID (0xffffffff -> -1 through the int P/Invoke) fails CLOSED' {
        # Regression: masking bits straight off -1 (every bit set) reported BOTH
        # IsPlaceholder and InSync as true, so a file whose state the API could not parse
        # read as "safely in sync" — and InSync alone drives backed_up state, marker
        # clearing, and retention pruning in the verify pass.
        $s = ConvertFrom-CfPlaceholderState -State ([int]0xffffffff)
        $s.IsPlaceholder | Should -BeFalse
        $s.InSync        | Should -BeFalse
        $s.Raw           | Should -Be -1      # original preserved for diagnosis
    }
    It 'any negative state fails closed, not just the documented sentinel' {
        foreach ($v in -2, -12345, [int]::MinValue) {
            $s = ConvertFrom-CfPlaceholderState -State $v
            $s.IsPlaceholder | Should -BeFalse
            $s.InSync        | Should -BeFalse
        }
    }
    It 'NO_STATES reports neither placeholder nor in-sync' {
        $s = ConvertFrom-CfPlaceholderState -State 0
        $s.IsPlaceholder | Should -BeFalse
        $s.InSync        | Should -BeFalse
    }
    It 'PLACEHOLDER without IN_SYNC is a placeholder that is not synced' {
        $s = ConvertFrom-CfPlaceholderState -State 0x1
        $s.IsPlaceholder | Should -BeTrue
        $s.InSync        | Should -BeFalse
    }
    It 'PLACEHOLDER + IN_SYNC is the confirmed-locally case' {
        $s = ConvertFrom-CfPlaceholderState -State (0x1 -bor 0x8)
        $s.IsPlaceholder | Should -BeTrue
        $s.InSync        | Should -BeTrue
    }
    It 'an undocumented future bit does not suppress a real IN_SYNC (fix must not over-reject)' {
        # Guards the fix itself: failing closed on negatives must not become
        # failing closed on any unrecognised positive bit.
        $s = ConvertFrom-CfPlaceholderState -State (0x1 -bor 0x8 -bor 0x40)
        $s.InSync | Should -BeTrue
    }
}

Describe 'install.ps1 helper block' {
    # Isolation rule (Stage 4 Task 6, fix round 1): every test here copies install.ps1 into a
    # TestDrive: directory WITHOUT a GitProtonBackup payload folder — so the module block always
    # takes its skip path — and runs it in a CHILD pwsh -NoProfile process with $env:LOCALAPPDATA
    # pointed at a TestDrive: subdir. No test may ever write to the real Documents modules
    # directory or the real LOCALAPPDATA (an earlier draft had exactly that contamination bug).
    # $env:LOCALAPPDATA only sandboxes the filesystem side — install.ps1's PATH persistence is a
    # direct HKCU\Environment registry write (Microsoft.Win32.Registry SetValue) that no
    # process-env override contains. Every invocation below therefore also passes
    # -SkipPathUpdate so no test run ever touches the real user PATH (SUPERSEDED banner,
    # Task 6 review).
    BeforeAll {
        function New-InstallerSandbox {
            $id = [guid]::NewGuid().ToString('N').Substring(0, 8)
            $scriptDir = Join-Path $TestDrive "install-$id"
            $localAppData = Join-Path $TestDrive "localappdata-$id"
            New-Item -ItemType Directory -Path $scriptDir -Force | Out-Null
            New-Item -ItemType Directory -Path $localAppData -Force | Out-Null
            Copy-Item -Path "$PSScriptRoot/../install.ps1" -Destination (Join-Path $scriptDir 'install.ps1') -Force
            [pscustomobject]@{ ScriptDir = $scriptDir; LocalAppData = $localAppData }
        }

        function Invoke-InstallerSandbox {
            param($ScriptDir, $LocalAppData, [string]$EffectivePath = '')
            $script = Join-Path $ScriptDir 'install.ps1'
            # -EffectivePath is always passed explicitly (default '' = no directories to
            # search, i.e. no shadow) so no invocation here ever falls back to the real
            # Machine/User registry PATH — see the shadow-detection tests below for the
            # two outcomes exercised with a real synthetic value.
            $output = & pwsh -NoProfile -Command "`$env:LOCALAPPDATA = '$LocalAppData'; & '$script' -SkipPathUpdate -EffectivePath '$EffectivePath'" 2>&1 | Out-String
            $exitCode = $LASTEXITCODE
            [pscustomobject]@{ ExitCode = $exitCode; Output = $output }
        }
    }

    It 'release-asset layout (exe + matching .sha256, no module payload): helper installs, module block skips' {
        $sb = New-InstallerSandbox
        $exePath = Join-Path $sb.ScriptDir 'git-remote-proton.exe'
        Set-Content -Path $exePath -Value 'fake exe bytes' -NoNewline
        $hash = (Get-FileHash $exePath -Algorithm SHA256).Hash.ToLower()
        Set-Content -Path "$exePath.sha256" -Value "$hash  git-remote-proton.exe" -NoNewline

        $r = Invoke-InstallerSandbox -ScriptDir $sb.ScriptDir -LocalAppData $sb.LocalAppData
        $r.ExitCode | Should -Be 0
        Test-Path (Join-Path $sb.LocalAppData 'Programs\git-proton-backup\git-remote-proton.exe') | Should -BeTrue
        $r.Output | Should -Match 'helper-only install'
        # Proves -SkipPathUpdate actually took the skip branch and never reached the real-PATH
        # mutation path (the registry-channel gap the fix round closed).
        $r.Output | Should -Not -Match 'Added .*to your user PATH'
    }

    It 'checksum mismatch refuses to install the helper and copies nothing' {
        $sb = New-InstallerSandbox
        $exePath = Join-Path $sb.ScriptDir 'git-remote-proton.exe'
        Set-Content -Path $exePath -Value 'fake exe bytes' -NoNewline
        Set-Content -Path "$exePath.sha256" -Value ('0' * 64 + '  git-remote-proton.exe') -NoNewline

        $r = Invoke-InstallerSandbox -ScriptDir $sb.ScriptDir -LocalAppData $sb.LocalAppData
        $r.ExitCode | Should -Not -Be 0
        Test-Path (Join-Path $sb.LocalAppData 'Programs\git-proton-backup\git-remote-proton.exe') | Should -BeFalse
    }

    It 'missing exe skips the helper install cleanly' {
        $sb = New-InstallerSandbox
        $r = Invoke-InstallerSandbox -ScriptDir $sb.ScriptDir -LocalAppData $sb.LocalAppData
        $r.ExitCode | Should -Be 0
        $r.Output | Should -Match 'skipping helper install'
    }

    # Stage 4 gate run 1 (BLOCKED): a stale git-remote-proton.exe earlier on PATH
    # shadowed the freshly installed helper — install.ps1 printed success and exited 0
    # while git kept resolving the old binary. These two tests exercise both outcomes of
    # the -EffectivePath shadow check WITHOUT ever reading the real registry PATH: a
    # synthetic PATH built from TestDrive dirs, passed explicitly, is the only input.
    It 'a git-remote-proton.exe earlier on the synthetic PATH triggers a loud warning naming both paths' {
        $sb = New-InstallerSandbox
        $exePath = Join-Path $sb.ScriptDir 'git-remote-proton.exe'
        Set-Content -Path $exePath -Value 'fake exe bytes' -NoNewline
        $hash = (Get-FileHash $exePath -Algorithm SHA256).Hash.ToLower()
        Set-Content -Path "$exePath.sha256" -Value "$hash  git-remote-proton.exe" -NoNewline

        $shadowDir = Join-Path $TestDrive "shadow-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        New-Item -ItemType Directory -Path $shadowDir -Force | Out-Null
        $shadowExe = Join-Path $shadowDir 'git-remote-proton.exe'
        Set-Content -Path $shadowExe -Value 'stale exe bytes' -NoNewline

        $installedExe = Join-Path $sb.LocalAppData 'Programs\git-proton-backup\git-remote-proton.exe'
        # Shadow dir listed FIRST — earlier on PATH than where the helper would resolve.
        $effectivePath = "$shadowDir;$(Split-Path $installedExe -Parent)"

        $r = Invoke-InstallerSandbox -ScriptDir $sb.ScriptDir -LocalAppData $sb.LocalAppData -EffectivePath $effectivePath
        $r.ExitCode | Should -Be 0                                   # remedy is environmental, not a failure
        $r.Output | Should -Match 'WARNING'
        $r.Output | Should -Match ([regex]::Escape($shadowExe))      # names the shadowing exe
        $r.Output | Should -Match ([regex]::Escape($installedExe))   # names the installed exe
    }

    It 'no earlier match on the synthetic PATH: no warning (behaviour unchanged)' {
        $sb = New-InstallerSandbox
        $exePath = Join-Path $sb.ScriptDir 'git-remote-proton.exe'
        Set-Content -Path $exePath -Value 'fake exe bytes' -NoNewline
        $hash = (Get-FileHash $exePath -Algorithm SHA256).Hash.ToLower()
        Set-Content -Path "$exePath.sha256" -Value "$hash  git-remote-proton.exe" -NoNewline

        $decoyDir = Join-Path $TestDrive "decoy-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        New-Item -ItemType Directory -Path $decoyDir -Force | Out-Null   # no exe inside — never a match

        $installedDir = Join-Path $sb.LocalAppData 'Programs\git-proton-backup'
        # Decoy first (no match), then the dir the helper installs into (the real first
        # hit once the copy has happened) — proves the check doesn't false-positive when
        # the installed copy is genuinely the winner.
        $effectivePath = "$decoyDir;$installedDir"

        $r = Invoke-InstallerSandbox -ScriptDir $sb.ScriptDir -LocalAppData $sb.LocalAppData -EffectivePath $effectivePath
        $r.ExitCode | Should -Be 0
        $r.Output | Should -Not -Match 'WARNING'
        $r.Output | Should -Not -Match 'shadow'
    }
}
