BeforeAll {
    Import-Module "$PSScriptRoot/../GitProtonBackup/GitProtonBackup.psm1" -Force
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
        $null = Initialize-ProtonBackup -ProtonDriveRoot $script:drive -ProtonCli 'no-such-cli-anywhere' -WarningVariable wv -WarningAction SilentlyContinue
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
    It 'Set-ProtonBackupConfig refuses to set Repos directly (owned by Install/Uninstall-ProtonBackup)' {
        Initialize-ProtonBackup -ProtonDriveRoot $script:drive -AuthProbe { 0 } -InfoProbe { 0 } | Out-Null
        { Set-ProtonBackupConfig -Key Repos -Value @() } | Should -Throw '*Install-ProtonBackup*'
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
    It 'CLI-ready path confirms and clears the marker with no -SyncCheck (behavioral coverage only — NOT the closure regression pin)' {
        # NOTE: this Describe's tests import the .psm1 directly (this file's BeforeAll:
        # `Import-Module ...GitProtonBackup.psm1 -Force`), which — because the module never calls
        # Export-ModuleMember — auto-exports EVERY function into this session's global command
        # table, private helpers included. That means a bareword call inside the GetNewClosure()'d
        # $effectiveCheck scriptblock in Invoke-ProtonBackupVerify (see psm1 ~1046) can still
        # resolve Get-CloudBundlePath/Confirm-BundleUploaded via THIS session's global scope even
        # if the closure's own SessionState is detached and broken — so this test cannot actually
        # catch a regression of that bareword-resolution bug (mutation-tested: reintroducing the
        # bug still passes this test). It's kept as ordinary behavioral coverage of the CLI-ready
        # branch (confirm + clear marker, no -SyncCheck seam). The real regression pin is the
        # manifest-import test below, in its own Describe.
        Write-PushPendingMarker -RepoPath $script:repo -Reason verify_timeout `
            -BundleDir (Get-GpbBundleDir -Config (Read-GpbConfig) -RepoPath $script:repo) -BundleBaseName (Split-Path $script:repo -Leaf)
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $true } -InfoRunner { param($cp,$cli) [pscustomobject]@{ ExitCode=0; Output="state: 'active'" } }
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

Describe 'Status + scheduled task' {
    BeforeEach { $script:drive = New-Sandbox; $script:repo = New-TestRepo; Install-ProtonBackup -RepoPath $script:repo }
    AfterEach  { Clear-Sandbox }

    It 'status reports wiring, local coverage, confirmation, dirt, and last-verify age' {
        Set-Content "$script:repo/b.txt" 'dirty'
        Invoke-ProtonBackupVerify -SyncCheck { param($p) $true } -CliReadyRunner { $false } | Out-Null
        $s = Get-ProtonBackupStatus -Json | ConvertFrom-Json
        @($s).Count | Should -Be 1
        $s[0].WiringOk | Should -BeTrue
        $s[0].CurrentBundled | Should -BeTrue
        $s[0].ConfirmedAtLastVerify | Should -Be 'ok'
        $s[0].DirtyCount | Should -Be 1
        $s[0].LastVerifyAgeHours | Should -BeLessThan 1
        $s[0].LastVerifyExitCode | Should -Be 0
    }
    It 'CurrentBundled is local-only: an unconfirmed spool still shows bundled but NOT confirmed' {
        Invoke-ProtonBackupVerify -SyncCheck { param($p) $false } -CliReadyRunner { $false } | Out-Null
        $s = (Get-ProtonBackupStatus -Json | ConvertFrom-Json)[0]
        $s.CurrentBundled | Should -BeTrue
        $s.ConfirmedAtLastVerify | Should -Be 'attention'
    }
    It 'status flags staleness after a new commit' {
        Invoke-ProtonBackupVerify -SyncCheck { param($p) $true } -CliReadyRunner { $false } | Out-Null
        Set-Content "$script:repo/a.txt" 'two'; git -C $script:repo add .; git -C $script:repo commit -qm c2
        ((Get-ProtonBackupStatus -Json | ConvertFrom-Json))[0].CurrentBundled | Should -BeFalse
    }
    It 'CurrentBundled: the NEWEST bundle governs, not any bundle carrying the current digest8' {
        # Fabricate two bundles in the bundle dir directly (no real Invoke-RepoBundleBackup call):
        # an OLDER one whose filename carries the CURRENT digest8, and a NEWER one carrying a
        # stale digest8. The .lastdigest stamp is set to the current digest so the digest-equality
        # half of the check passes — only the newest-bundle-carries-it half can fail here.
        $cfg = Read-GpbConfig
        $bd = Get-GpbBundleDir -Config $cfg -RepoPath $script:repo
        New-Item -ItemType Directory -Path $bd -Force | Out-Null
        $baseName = Split-Path $script:repo -Leaf
        $digest = Get-RepoRefDigest -RepoPath $script:repo
        $digest8 = $digest.Substring(0, 8).ToLowerInvariant()
        $staleDigest8 = 'deadbeef'
        Set-Content -LiteralPath (Join-Path $bd ".$baseName.lastdigest") -Value $digest -NoNewline

        $older = Join-Path $bd "$baseName-20260101T000000Z-$digest8.bundle"
        Set-Content -LiteralPath $older -Value 'x' -NoNewline
        (Get-Item -LiteralPath $older).LastWriteTime = (Get-Date).AddDays(-2)

        $newer = Join-Path $bd "$baseName-20260102T000000Z-$staleDigest8.bundle"
        Set-Content -LiteralPath $newer -Value 'y' -NoNewline
        (Get-Item -LiteralPath $newer).LastWriteTime = (Get-Date)

        (Get-ProtonBackupStatus -Json | ConvertFrom-Json)[0].CurrentBundled | Should -BeFalse
    }
    It 'default output (no -Json) renders as a table, not a vertical property list' {
        Invoke-ProtonBackupVerify -SyncCheck { param($p) $true } -CliReadyRunner { $false } | Out-Null
        $out = Get-ProtonBackupStatus | Out-String
        $out | Should -Match '(?m)^\s*RepoPath\s+WiringOk'
    }
    It 'task install passes plain data: interactive logon, StartWhenAvailable, daily; -Uninstall unregisters' {
        $script:reg = $null; $script:unreg = $null
        Install-ProtonBackupTask -Register { param($p) $script:reg = $p } -Unregister { param($n) $script:unreg = $n }
        $script:reg.TaskName | Should -Be 'GitProtonBackup Verify'
        $script:reg.LogonType | Should -Be 'Interactive'
        $script:reg.StartWhenAvailable | Should -BeTrue
        $script:reg.Execute | Should -Be 'pwsh'
        $script:reg.Arguments | Should -Match 'Invoke-ProtonBackupVerify'
        Install-ProtonBackupTask -Uninstall -Register { param($p) } -Unregister { param($n) $script:unreg = $n }
        $script:unreg | Should -Be 'GitProtonBackup Verify'
    }
}

Describe 'Manifest-import regression pins' {
    BeforeEach {
        # GPB_CONFIG_DIR/GPB_LOCK_PATH/GPB_HOOK_DISABLED are set here (in THIS process) so the
        # child pwsh process spawned below inherits them automatically — same mechanism as every
        # other real-child-process test in this suite (e.g. PushBackupFlow.Tests.ps1's
        # Invoke-RealShimPush).
        $env:GPB_CONFIG_DIR = Join-Path $TestDrive "mi-cfg-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        $env:GPB_LOCK_PATH  = Join-Path $TestDrive "mi-lk-$([guid]::NewGuid().ToString('N').Substring(0,8)).lock"
        $env:GPB_HOOK_DISABLED = '1'
        New-Item -ItemType Directory $env:GPB_CONFIG_DIR -Force | Out-Null
    }
    AfterEach { Remove-Item Env:GPB_CONFIG_DIR, Env:GPB_LOCK_PATH, Env:GPB_HOOK_DISABLED -ErrorAction SilentlyContinue }

    It 'manifest-import verify resolves internal helpers (GetNewClosure regression pin)' {
        # THE actual regression pin for the GetNewClosure bareword-resolution bug in
        # Invoke-ProtonBackupVerify's $effectiveCheck (psm1, ~line 1046: "GetNewClosure() below
        # detaches the resulting scriptblock's SessionState from this module..."). Every other
        # test in this suite imports the .psm1 file DIRECTLY (this file's own BeforeAll does
        # `Import-Module .../GitProtonBackup.psm1 -Force`) — and because this module never calls
        # Export-ModuleMember, a direct .psm1 import auto-exports EVERY function, private helpers
        # included, into the importing session's global command table. That means a bareword call
        # inside the detached closure can still resolve Get-CloudBundlePath/Confirm-BundleUploaded
        # via the test session's own global scope even when the closure's OWN scope resolution is
        # broken — so an in-process test of this branch is vacuous and cannot catch a regression
        # (confirmed by mutation testing: reintroducing the bareword-call bug still passes an
        # in-process test). Production imports the module via the psd1 MANIFEST
        # (`Import-Module GitProtonBackup` by bare name resolves through the manifest, which
        # exports only the 10 public functions) — a real manifest import, in a real child process,
        # is the only way this failure mode can actually be exercised and caught.
        $drive = Join-Path $TestDrive "mi-drive-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        New-Item -ItemType Directory $drive -Force | Out-Null
        # Config written as plain JSON (not via Write-GpbConfig/Get-GpbDefaultConfig) — the child
        # process resolves the module fresh via the manifest; nothing here should lean on this
        # (in-process, direct-import) session's copy of those private helpers.
        $cfg = [pscustomobject]@{
            ProtonDriveRoot = $drive; BackupSubdir = 'GitBackups'; ProtonCli = 'stub'
            VerifySeconds = 1; RetentionKeep = 5; RetentionCheckpoints = 24
            MaxUnconfirmedAgeDays = 7; HeartbeatUrl = ''; Repos = @()
        }
        $cfg | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $env:GPB_CONFIG_DIR 'config.json') -NoNewline

        $repo = Join-Path $TestDrive "mi-repo-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        New-Item -ItemType Directory $repo -Force | Out-Null
        git -C $repo init -qb main
        git -C $repo config user.email 't@t'; git -C $repo config user.name 't'
        Set-Content "$repo/a.txt" 'one'; git -C $repo add .; git -C $repo commit -qm 'c1'

        # Written to a temp .ps1 and run via -File (rather than a single -Command string) so the
        # InfoRunner's nested quoting ("state: 'active'") never has to survive two layers of
        # command-line requoting.
        $childScriptPath = Join-Path $TestDrive "mi-pin-child-$([guid]::NewGuid().ToString('N').Substring(0,8)).ps1"
        @'
param([Parameter(Mandatory)][string]$RepoPath)
Import-Module GitProtonBackup   # by bare name -> psd1 manifest -> only the 10 public functions
$exported = (Get-Module GitProtonBackup).ExportedFunctions.Count
Write-Host "child: manifest import exported $exported function(s)"
Install-ProtonBackup -RepoPath $RepoPath | Out-Null
$r = Invoke-ProtonBackupVerify -CliReadyRunner { $true } -InfoRunner {
    param($cp, $cli)
    [pscustomobject]@{ ExitCode = 0; Output = "state: 'active'" }
}
foreach ($f in @($r.Findings)) { Write-Host "FINDING: $f" }
Write-Host "EXIT=$($r.ExitCode)"
exit $r.ExitCode
'@ | Set-Content -LiteralPath $childScriptPath -NoNewline

        $out = & pwsh -NoProfile -File $childScriptPath -RepoPath $repo 2>&1
        $childExit = $LASTEXITCODE

        ($out -join "`n") | Should -Not -Match 'is not recognized'
        ($out -join "`n") | Should -Match 'exported 10 function'
        $childExit | Should -Be 0
    }
}

Describe 'Invoke-ProtonBackupVerify return contract (v0.2.0)' {
    BeforeEach {
        $script:dir = Join-Path ([IO.Path]::GetTempPath()) ("gpbv2-" + [guid]::NewGuid().ToString('N'))
        New-Item -ItemType Directory -Force -Path $script:dir | Out-Null
        $env:GPB_CONFIG_DIR = $script:dir
        $env:GPB_LOCK_PATH  = Join-Path $script:dir 'test.lock'
    }
    AfterEach {
        Remove-Item Env:GPB_CONFIG_DIR -ErrorAction SilentlyContinue
        Remove-Item Env:GPB_LOCK_PATH -ErrorAction SilentlyContinue
        # -Force (not the raw [IO.Directory]::Delete the brief's snippet used) so the repo's
        # .git objects — Windows marks loose objects read-only — don't throw
        # UnauthorizedAccessException here and fail an otherwise-passing test.
        Remove-Item -LiteralPath $script:dir -Recurse -Force -ErrorAction SilentlyContinue
    }

    It 'returns Complete=$false, IncompleteReason=config, Repos=@() on missing config (exit 2)' {
        $r = Invoke-ProtonBackupVerify
        $r.ExitCode | Should -Be 2
        $r.Complete | Should -BeFalse
        $r.IncompleteReason | Should -Be 'config'
        @($r.Repos).Count | Should -Be 0
        $lv = Get-Content (Join-Path $script:dir 'last-verify.json') -Raw | ConvertFrom-Json
        $lv.Complete | Should -BeFalse
        $lv.IncompleteReason | Should -Be 'config'
    }

    It 'returns Complete=$false, IncompleteReason=lock on lock contention (exit 1)' {
        # Valid config so the config read succeeds, then hold the lock exclusively.
        $drive = Join-Path $script:dir 'drive'; New-Item -ItemType Directory -Force -Path $drive | Out-Null
        Initialize-ProtonBackup -ProtonDriveRoot $drive -WarningAction SilentlyContinue
        $fs = [System.IO.File]::Open($env:GPB_LOCK_PATH, 'OpenOrCreate', 'ReadWrite', 'None')
        try { $r = Invoke-ProtonBackupVerify -LockTimeoutSeconds 1 } finally { $fs.Close() }
        $r.ExitCode | Should -Be 1
        $r.Complete | Should -BeFalse
        $r.IncompleteReason | Should -Be 'lock'
        $lv = Get-Content (Join-Path $script:dir 'last-verify.json') -Raw | ConvertFrom-Json
        $lv.Complete | Should -BeFalse
    }

    It 'returns Complete=$true, IncompleteReason empty, and per-repo Repos on a normal pass' {
        $drive = Join-Path $script:dir 'drive'; New-Item -ItemType Directory -Force -Path $drive | Out-Null
        Initialize-ProtonBackup -ProtonDriveRoot $drive -WarningAction SilentlyContinue
        # Empty registry: still a complete pass.
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $false } -WarningAction SilentlyContinue
        $r.ExitCode | Should -Be 0
        $r.Complete | Should -BeTrue
        $r.IncompleteReason | Should -Be ''
        @($r.Repos).Count | Should -Be 0
        # Register a real repo; its row must appear in the RETURN value, not only the file.
        $repo = Join-Path $script:dir 'repo'; New-Item -ItemType Directory -Force -Path $repo | Out-Null
        git -C $repo init -q; git -C $repo config user.email t@t; git -C $repo config user.name t
        Set-Content (Join-Path $repo 'a.txt') 'x'; git -C $repo add .; git -C $repo commit -qm init
        $env:GPB_HOOK_DISABLED = '1'
        try { Install-ProtonBackup -RepoPath $repo | Out-Null } finally { Remove-Item Env:GPB_HOOK_DISABLED }
        $r2 = Invoke-ProtonBackupVerify -CliReadyRunner { $false } -SyncCheck { param($p) $true } -WarningAction SilentlyContinue
        $r2.Complete | Should -BeTrue
        @($r2.Repos).Count | Should -Be 1
        $r2.Repos[0].RepoPath | Should -Be (Resolve-Path $repo).Path
        $r2.Repos[0].State | Should -BeIn @('ok','attention')
    }
}
