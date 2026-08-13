BeforeAll {
    Import-Module "$PSScriptRoot/../GitProtonBackup/GitProtonBackup.psm1" -Force
    function New-Sandbox {
        $env:GPB_CONFIG_DIR = Join-Path $TestDrive "cfg-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        $env:GPB_LOCK_PATH  = Join-Path $TestDrive "lk-$([guid]::NewGuid().ToString('N').Substring(0,8)).lock"
        $env:GPB_HOOK_DISABLED = '1'
        $drive = Join-Path $TestDrive "drive-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        New-Item -ItemType Directory $drive -Force | Out-Null
        $cfg = Get-GpbDefaultConfig; $cfg.ProtonDriveRoot = $drive
        # VerifySeconds doubles as the verify-path upload-lag grace budget; 1s keeps every
        # always-unconfirmed verify call in this suite fast (same pattern as PushBackupFlow's
        # New-GpbTestConfig, which pins it to 1s for the push-path poll loop).
        $cfg.VerifySeconds = 1
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

Describe 'Invoke-ProtonBackupVerify upload-lag grace' {
    # A machine-off gap makes EVERY registered repo's digest stale at once, so the next verify run
    # re-cuts the whole fleet and then asks Proton about files that have existed for only seconds —
    # before the sync app can possibly have uploaded them. Without a grace, that first
    # post-downtime run false-alarms fleet-wide ("newest bundle not confirmed on Proton" for every
    # repo; observed live 2026-08-12: 15/16 repos flagged, all clean on a re-run 10 minutes later).
    # The grace is the verify-path analogue of the push flow's VerifySeconds poll loop, but
    # fleet-shaped: ONE shared deadline sweeps every freshly-cut still-unconfirmed repo in rounds —
    # never a per-repo window (16 repos must never cost 16 × VerifySeconds).
    BeforeEach {
        $script:drive = New-Sandbox; $script:repo = New-TestRepo
        Install-ProtonBackup -RepoPath $script:repo
        # The grace budget defaults to cfg.VerifySeconds (the same knob as the push path's poll
        # window); 1s keeps every bounded-wait assertion below fast.
        Set-ProtonBackupConfig -Key VerifySeconds -Value 1
    }
    AfterEach { Clear-Sandbox }

    It 'first run after a downtime gap: freshly cut fleet confirms during the grace window (no false fleet alarm)' {
        # RED: reproduces the 2026-08-12 incident in CLI mode. The first info call per bundle
        # answers "Node not found" (sync app hasn't uploaded yet); any later call answers active.
        # Pre-grace code checked each repo exactly once and the whole fleet exited 1.
        # No -GraceSeconds passed: this also pins the cfg.VerifySeconds default fallback.
        $repo2 = New-TestRepo; Install-ProtonBackup -RepoPath $repo2
        $script:infoCalls = @{}
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $true } -InfoRunner {
            param($cp, $cli)
            $script:infoCalls[$cp] = 1 + [int]$script:infoCalls[$cp]
            if ($script:infoCalls[$cp] -eq 1) { [pscustomobject]@{ ExitCode = 1; Output = 'Node not found' } }
            else { [pscustomobject]@{ ExitCode = 0; Output = "state: 'active'" } }
        }
        $r.ExitCode | Should -Be 0
        @($r.Findings) | Should -BeNullOrEmpty
        @($r.Repos).Count | Should -Be 2
        @($r.Repos | Where-Object State -ne 'ok').Count | Should -Be 0
        # both repos actually got the second look
        $script:infoCalls.Count | Should -Be 2
        @($script:infoCalls.Values | Where-Object { $_ -lt 2 }).Count | Should -Be 0
    }

    It 'a verify_timeout push marker clears when its bundle confirms during the grace (degraded Cloud-Files mode)' {
        # RED. Also pins -GraceSeconds/-GracePollSeconds as explicit overrides of the config
        # default, and that the grace works through the degraded SyncCheck branch too (upload lag
        # is the same phenomenon under either verifier).
        Write-PushPendingMarker -RepoPath $script:repo -Reason verify_timeout `
            -BundleDir (Get-GpbBundleDir -Config (Read-GpbConfig) -RepoPath $script:repo) -BundleBaseName (Split-Path $script:repo -Leaf)
        $script:syncCalls = @{}
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $false } -WarningAction SilentlyContinue `
            -GraceSeconds 30 -GracePollSeconds 1 -SyncCheck {
                param($p)
                $script:syncCalls[$p] = 1 + [int]$script:syncCalls[$p]
                $script:syncCalls[$p] -ge 2
            }
        $r.ExitCode | Should -Be 0
        @(Get-ChildItem (Get-GpbMarkerDir) -Filter '*.json' -ErrorAction SilentlyContinue).Count | Should -Be 0
        ($r.Findings -join "`n") | Should -Not -Match 'pending backup not confirmed'
    }

    It 'a bundle left unconfirmed by a PREVIOUS run is not grace-polled (the grace is for fresh cuts only)' {
        # GUARD: budget protection — an old stuck spool is not suffering upload lag and can never
        # confirm within the window; polling it would burn the whole shared budget and delay the
        # report without changing the verdict. It must fail fast (MaxUnconfirmedAgeDays is its
        # escalation path). The bundle is aged below because eligibility is fresh-cut-by-age OR
        # cut-this-run: a bundle a previous run cut only seconds ago legitimately still gets the
        # grace (see the hook-cut test below); a genuinely stuck spool is hours old.
        Invoke-ProtonBackupVerify -CliReadyRunner { $false } -WarningAction SilentlyContinue `
            -SyncCheck { param($p) $false } | Out-Null            # cut once; never confirms
        $bd = Get-GpbBundleDir -Config (Read-GpbConfig) -RepoPath $script:repo
        Get-ChildItem $bd -Filter '*.bundle' | ForEach-Object { $_.LastWriteTime = (Get-Date).AddHours(-1) }
        $script:syncCalls = @{}
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $false } -WarningAction SilentlyContinue `
            -GraceSeconds 5 -GracePollSeconds 1 -SyncCheck {
                param($p); $script:syncCalls[$p] = 1 + [int]$script:syncCalls[$p]; $false
            }
        $r.ExitCode | Should -Be 1
        ($r.Findings -join "`n") | Should -Match 'not confirmed on Proton'
        # exactly the one in-pass check — zero grace re-checks for a bundle this run did not cut
        @($script:syncCalls.Values) | Should -Be @(1)
    }

    It '-GraceSeconds 0 disables the grace: single check, immediate verdict' {
        # GUARD: the escape hatch — with the grace off, behavior is exactly the pre-grace single
        # check, even for a seam that WOULD have confirmed on a second call.
        $script:syncCalls = @{}
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $false } -WarningAction SilentlyContinue `
            -GraceSeconds 0 -SyncCheck {
                param($p)
                $script:syncCalls[$p] = 1 + [int]$script:syncCalls[$p]
                $script:syncCalls[$p] -ge 2
            }
        $r.ExitCode | Should -Be 1
        ($r.Findings -join "`n") | Should -Match 'not confirmed on Proton'
        @($script:syncCalls.Values) | Should -Be @(1)
    }

    It 'the grace is ONE shared deadline: re-checks sweep the pending fleet in rounds, wall-clock bounded' {
        # RED (re-checks happen at all) + GUARD (round shape): a per-repo serial poll would burn
        # repos × VerifySeconds, and its call trace would repeat one path to exhaustion before
        # touching the next. The trace must instead sweep all pending repos each round.
        $repo2 = New-TestRepo; Install-ProtonBackup -RepoPath $repo2
        $repo3 = New-TestRepo; Install-ProtonBackup -RepoPath $repo3
        $script:log = [System.Collections.Generic.List[string]]::new()
        $elapsed = Measure-Command {
            $script:r = Invoke-ProtonBackupVerify -CliReadyRunner { $false } -WarningAction SilentlyContinue `
                -SyncCheck { param($p) $script:log.Add($p); $false }
        }
        $script:r.ExitCode | Should -Be 1
        # Bounded: three pending repos share ONE 1s window. The generous ceiling stays far below
        # both a serial-per-repo pathology and a hardcoded-60s default while tolerating slow CI.
        $elapsed.TotalSeconds | Should -BeLessThan 20
        $script:log.Count | Should -BeGreaterThan 3            # grace re-checks actually happened
        $grace = @($script:log)[3..($script:log.Count - 1)]
        ($grace.Count % 3) | Should -Be 0
        for ($i = 0; $i -lt $grace.Count; $i += 3) {
            @($grace[$i..($i + 2)] | Sort-Object -Unique).Count | Should -Be 3
        }
    }

    It 'mixed fleet: the repo confirming during grace goes quiet; the never-confirming one still alarms — in registration order' {
        # RED. Also guards report ordering: the Repos array must keep registration order even
        # though grace-deferred repos are finalized after the others.
        $repoB = New-TestRepo; Install-ProtonBackup -RepoPath $repoB
        $slugA = Get-GpbSlug -RepoPath $script:repo
        $script:aCalls = 0
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $false } -WarningAction SilentlyContinue -SyncCheck {
            param($p)
            if ($p -like "*$slugA*") { $script:aCalls++; $script:aCalls -ge 2 } else { $false }
        }
        $r.ExitCode | Should -Be 1
        $resolvedA = (Resolve-Path $script:repo).Path
        $resolvedB = (Resolve-Path $repoB).Path
        @($r.Repos)[0].RepoPath | Should -Be $resolvedA
        @($r.Repos)[1].RepoPath | Should -Be $resolvedB
        @($r.Repos)[0].State | Should -Be 'ok'
        @($r.Repos)[1].State | Should -Be 'attention'
        ($r.Findings -join "`n") | Should -Match ([regex]::Escape("not confirmed on Proton for $resolvedB"))
        ($r.Findings -join "`n") | Should -Not -Match ([regex]::Escape("not confirmed on Proton for $resolvedA"))
    }

    It 'a fresh bundle cut moments BEFORE the run (push hook) is grace-eligible: file age matters, not who cut it' {
        # RED (round-1 peer review, Gemini High + Codex Medium-high): eligibility keyed ONLY on
        # Created missed the sibling false alarm — a push hook cuts a bundle seconds before the
        # scheduled verify fires; verify cache-hits (Created=$false) and used to alarm instantly
        # on a file still uploading. Upload lag is a property of the file's age, not of which
        # process cut it. Run 1 below stands in for the hook's cut: bundle on disk, digest
        # current, unconfirmed, seconds old.
        Invoke-ProtonBackupVerify -CliReadyRunner { $false } -WarningAction SilentlyContinue `
            -GraceSeconds 0 -SyncCheck { param($p) $false } | Out-Null
        $script:syncCalls = @{}
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $false } -WarningAction SilentlyContinue `
            -GraceSeconds 30 -GracePollSeconds 1 -SyncCheck {
                param($p)
                $script:syncCalls[$p] = 1 + [int]$script:syncCalls[$p]
                $script:syncCalls[$p] -ge 2
            }
        $r.ExitCode | Should -Be 0
        @($r.Findings) | Should -BeNullOrEmpty
    }

    It 'a marker written while the run is mid-survey (concurrent push) survives the confirm-time clear and is reported pending' {
        # RED (round-1 peer review, Codex Critical; boundary tightened by round 2, Codex+Gemini
        # Blocker): phase C used to clear the repo''s marker on confirm regardless of WHEN it was
        # written — deleting a deferred_lock marker a push wrote while deferring on verify''s held
        # lock, silencing the one signal that that push''s (possibly newer) coverage had a gap.
        # The clear must be stale-safe against a cutoff captured BEFORE the digest snapshot: the
        # marker below is written from inside the bundling step itself (the in-pass SyncCheck
        # call), the tightest interleaving — a cutoff captured after the bundle call misclassifies
        # it as old and wrongly clears it.
        $bd = Get-GpbBundleDir -Config (Read-GpbConfig) -RepoPath $script:repo
        $baseName = Split-Path $script:repo -Leaf
        $script:calls = 0
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $false } -WarningAction SilentlyContinue `
            -GraceSeconds 30 -GracePollSeconds 1 -SyncCheck {
                param($p)
                $script:calls++
                if ($script:calls -eq 1) {
                    # the concurrent push, deferring against the held lock while this repo's
                    # bundling step is still running
                    Write-PushPendingMarker -RepoPath $script:repo -Reason deferred_lock -BundleDir $bd -BundleBaseName $baseName
                }
                $script:calls -ge 2
            }
        $r.ExitCode | Should -Be 1
        @(Get-ChildItem (Get-GpbMarkerDir) -Filter '*.json').Count | Should -Be 1
        ($r.Findings -join "`n") | Should -Match 'pending backup not confirmed \(reason: deferred_lock\)'
        # the repo row itself still reports ok — the marker is the deferred push's story, not this bundle's
        (@($r.Repos) | Where-Object RepoPath -eq (Resolve-Path $script:repo).Path).State | Should -Be 'ok'
    }

    It 'freshness is judged against the RUN START, not against when the fleet pass happens to reach the repo' {
        # RED (round-2 peer review, Codex Major): the mtime cutoff was compared against "now" at
        # eligibility time — a hook-cut bundle fresh when the run started could age out while
        # earlier fleet entries or a slow probe ran, then got the instant verdict anyway. The
        # first probe below stalls 3s; with GraceSeconds 10 and a bundle aged 8s, a now-anchored
        # cutoff sees ~11s (ineligible) while the run-start anchor correctly sees 8s.
        Invoke-ProtonBackupVerify -CliReadyRunner { $false } -WarningAction SilentlyContinue `
            -GraceSeconds 0 -SyncCheck { param($p) $false } | Out-Null
        $bd = Get-GpbBundleDir -Config (Read-GpbConfig) -RepoPath $script:repo
        Get-ChildItem $bd -Filter '*.bundle' | ForEach-Object { $_.LastWriteTime = (Get-Date).AddSeconds(-8) }
        $script:syncCalls = @{}
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $false } -WarningAction SilentlyContinue `
            -GraceSeconds 10 -GracePollSeconds 1 -SyncCheck {
                param($p)
                $script:syncCalls[$p] = 1 + [int]$script:syncCalls[$p]
                if ($script:syncCalls[$p] -eq 1) { Start-Sleep -Seconds 3; return $false }   # slow fleet-pass stand-in
                $true
            }
        $r.ExitCode | Should -Be 0
        @($r.Findings) | Should -BeNullOrEmpty
    }

    It 'a hand-edited non-numeric VerifySeconds cannot crash verify past its durable report' {
        # RED (round-3 peer review, Codex Major): the config-default clamp cast cfg.VerifySeconds
        # to [int] OUTSIDE the never-go-silent protections — a hand-edited config value like
        # "sixty" killed the run before last-verify.json and the heartbeat, exactly the silence
        # this function exists to prevent. Garbage now falls back to the stock default.
        $raw = Get-Content (Get-GpbConfigPath) -Raw | ConvertFrom-Json
        $raw.VerifySeconds = 'sixty'
        Write-GpbJsonAtomic -Path (Get-GpbConfigPath) -Object $raw
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $false } -WarningAction SilentlyContinue -SyncCheck { param($p) $true }
        $r.ExitCode | Should -Be 0
        (Get-Content (Join-Path (Get-GpbRoot) 'last-verify.json') -Raw | ConvertFrom-Json).ExitCode | Should -Be 0
    }

    It 'a repo that only ever confirms during the grace still gets retention pruning' {
        # RED (round-3 peer review, Codex Major): the bundling step's own retention prune runs
        # only when the in-pass check confirms — a busy repo whose refs change before every run
        # and whose uploads always outlast the in-pass moment would accumulate bundles unboundedly
        # AND have its spool warning suppressed (Confirmed=true). Grace-confirmed repos now prune
        # in phase C. Three bundles because the oldest is a monthly checkpoint (never pruned);
        # with RetentionKeep=1 the middle one is the prunable excess.
        Set-ProtonBackupConfig -Key RetentionKeep -Value 1
        Set-Content "$script:repo/a.txt" 'two'; git -C $script:repo add .; git -C $script:repo commit -qm c2
        Invoke-ProtonBackupVerify -CliReadyRunner { $false } -WarningAction SilentlyContinue -GraceSeconds 0 -SyncCheck { param($p) $false } | Out-Null
        Set-Content "$script:repo/a.txt" 'three'; git -C $script:repo add .; git -C $script:repo commit -qm c3
        Invoke-ProtonBackupVerify -CliReadyRunner { $false } -WarningAction SilentlyContinue -GraceSeconds 0 -SyncCheck { param($p) $false } | Out-Null
        $bd = Get-GpbBundleDir -Config (Read-GpbConfig) -RepoPath $script:repo
        $script:preexisting = @(Get-ChildItem $bd -Filter '*.bundle' | Sort-Object LastWriteTime | ForEach-Object FullName)
        $script:preexisting.Count | Should -Be 2
        Set-Content "$script:repo/a.txt" 'four'; git -C $script:repo add .; git -C $script:repo commit -qm c4
        $script:calls = 0
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $false } -WarningAction SilentlyContinue `
            -GraceSeconds 30 -GracePollSeconds 1 -SyncCheck {
                param($p)
                if ($p -in $script:preexisting) { return $true }   # older spool: long since uploaded
                $script:calls++
                $script:calls -ge 2                                 # the new cut: confirms during grace
            }
        $r.ExitCode | Should -Be 0
        $after = @(Get-ChildItem $bd -Filter '*.bundle').FullName
        # newest kept + oldest kept (monthly checkpoint); the middle bundle pruned
        $after.Count | Should -Be 2
        $after | Should -Contain $script:preexisting[0]
        $after | Should -Not -Contain $script:preexisting[1]
    }

    It 'parameter ranges: a zero poll interval (spin loop) and a beyond-1h grace are refused at binding' {
        # RED (round-2 peer review, Gemini+Codex Major / DeepSeek Major): -GracePollSeconds 0
        # produced an unpaced spin loop under the held lock, and -GraceSeconds up to 86400 turned
        # the freshness line into "anything cut today" plus a potential day-long lock hold. The
        # poll floor is 1s — the parameter itself is the test seam, and no test needs a true 0.
        { Invoke-ProtonBackupVerify -GracePollSeconds 0 } | Should -Throw
        { Invoke-ProtonBackupVerify -GraceSeconds 86400 } | Should -Throw
    }

    It 'a throwing grace re-check faults only its own repo: others still confirm, polling it stops, the report survives' {
        # RED (the healthy repo must go quiet) + GUARD (fault isolation): one repo's bad probe
        # must never abort the fleet's grace, keep being re-polled, or skip the durable report.
        $repoB = New-TestRepo; Install-ProtonBackup -RepoPath $repoB
        $repoC = New-TestRepo; Install-ProtonBackup -RepoPath $repoC
        $slugA = Get-GpbSlug -RepoPath $script:repo
        $slugB = Get-GpbSlug -RepoPath $repoB
        $script:aCalls = 0; $script:bCalls = 0
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $false } -WarningAction SilentlyContinue -SyncCheck {
            param($p)
            if ($p -like "*$slugA*") {
                $script:aCalls++
                if ($script:aCalls -ge 2) { throw 'probe exploded' }
                return $false
            }
            if ($p -like "*$slugB*") { $script:bCalls++; return ($script:bCalls -ge 2) }
            $false   # repo C: never confirms — keeps the grace loop alive past A's fault
        }
        $r.ExitCode | Should -Be 1
        $resolvedA = (Resolve-Path $script:repo).Path
        $resolvedB = (Resolve-Path $repoB).Path
        (@($r.Repos) | Where-Object RepoPath -eq $resolvedA).State | Should -Be 'attention'
        (@($r.Repos) | Where-Object RepoPath -eq $resolvedB).State | Should -Be 'ok'
        ($r.Findings -join "`n") | Should -Match ([regex]::Escape("not confirmed on Proton for $resolvedA"))
        # the faulted repo is dropped from later rounds, not re-polled to the deadline
        $script:aCalls | Should -Be 2
        (Get-Content (Join-Path (Get-GpbRoot) 'last-verify.json') -Raw | ConvertFrom-Json).ExitCode | Should -Be 1
    }

    It 'a hand-edited NEGATIVE VerifySeconds cannot crash verify past its durable report (grace degrades to disabled)' {
        # RED (post-branch review fix pass): [int]::TryParse accepts "-1", and assigning it into
        # the [ValidateRange(0,3600)] $GraceSeconds parameter re-fires validation OUTSIDE every
        # try — killing the run before last-verify.json and the heartbeat, exactly the silence the
        # non-numeric test above exists to prevent (that test's 'sixty' is rejected by TryParse
        # and never reaches the assignment). A negative wait means "don't wait": it must clamp to
        # 0 (grace disabled, the push path's effective behavior for a negative window), not crash
        # and not inherit the 60s stock default.
        $raw = Get-Content (Get-GpbConfigPath) -Raw | ConvertFrom-Json
        $raw.VerifySeconds = -1
        Write-GpbJsonAtomic -Path (Get-GpbConfigPath) -Object $raw
        $script:syncCalls = @{}
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $false } -WarningAction SilentlyContinue -SyncCheck {
            param($p)
            $script:syncCalls[$p] = 1 + [int]$script:syncCalls[$p]
            $script:syncCalls[$p] -ge 2   # would confirm on a second look — the grace must not grant one
        }
        $r.ExitCode | Should -Be 1
        (Get-Content (Join-Path (Get-GpbRoot) 'last-verify.json') -Raw | ConvertFrom-Json).ExitCode | Should -Be 1
        # clamped to 0, not defaulted to 60: exactly the one in-pass check, no grace rounds
        @($script:syncCalls.Values) | Should -Be @(1)
    }

    It 'a mid-run verify_timeout marker naming the SAME bundle this run confirms is cleared (identity trumps age)' {
        # RED (post-branch review fix pass): the push hook cuts its bundle, RELEASES the lock, and
        # polls lock-free — so its poll can time out and write a verify_timeout marker naming that
        # bundle WHILE verify holds the lock and is confirming the very same file. That marker's
        # mtime postdates SurveyedAtUtc, so the age-only stale-safe guard refused to clear it and
        # the marker pass reported a false 'pending backup not confirmed' + exit 1 for coverage
        # this run's verdict fully vouches for. Every push-poll/verify overlap produced one false
        # alarm. Identity must trump age: a marker whose BundlePath IS res.BundlePath is cleared.
        $bd = Get-GpbBundleDir -Config (Read-GpbConfig) -RepoPath $script:repo
        $baseName = Split-Path $script:repo -Leaf
        $script:calls = 0
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $false } -WarningAction SilentlyContinue `
            -GraceSeconds 30 -GracePollSeconds 1 -SyncCheck {
                param($p)
                $script:calls++
                if ($script:calls -eq 1) {
                    # the push's lock-free confirm poll timing out mid-run, naming the same bundle
                    Write-PushPendingMarker -RepoPath $script:repo -Reason verify_timeout `
                        -BundleDir $bd -BundleBaseName $baseName -BundlePath $p
                }
                $script:calls -ge 2
            }
        $r.ExitCode | Should -Be 0
        @(Get-ChildItem (Get-GpbMarkerDir) -Filter '*.json' -ErrorAction SilentlyContinue).Count | Should -Be 0
        ($r.Findings -join "`n") | Should -Not -Match 'pending backup not confirmed'
    }

    It 'a mid-run marker naming a DIFFERENT bundle still survives the confirm-time clear (no blanket clearing)' {
        # GUARD for the identity exception above: it must not regress the round-1 stale-safe
        # guard. A marker naming coverage this run did NOT confirm — a push racing ahead with a
        # newer cut — stays pending until a run that actually confirms it, exactly like the
        # deferred_lock (no-BundlePath) case the original guard was built for.
        $bd = Get-GpbBundleDir -Config (Read-GpbConfig) -RepoPath $script:repo
        $baseName = Split-Path $script:repo -Leaf
        $script:calls = 0
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $false } -WarningAction SilentlyContinue `
            -GraceSeconds 30 -GracePollSeconds 1 -SyncCheck {
                param($p)
                $script:calls++
                if ($script:calls -eq 1) {
                    Write-PushPendingMarker -RepoPath $script:repo -Reason verify_timeout `
                        -BundleDir $bd -BundleBaseName $baseName -BundlePath "$p.raced-ahead"
                }
                $script:calls -ge 2
            }
        $r.ExitCode | Should -Be 1
        @(Get-ChildItem (Get-GpbMarkerDir) -Filter '*.json').Count | Should -Be 1
        ($r.Findings -join "`n") | Should -Match 'pending backup not confirmed \(reason: verify_timeout\)'
        # the repo row itself still reports ok — the marker is the racing push's story
        (@($r.Repos) | Where-Object RepoPath -eq (Resolve-Path $script:repo).Path).State | Should -Be 'ok'
    }
}

Describe 'Invoke-ProtonBackupVerify sync-app stall detection' {
    # 2026-08-12 ~18:50 PT incident (same day as the upload-lag one above, hours later): the
    # Proton Drive sync app silently stopped uploading (last successful upload 13:51 PT) while its
    # Cloud Files provider kept marking freshly cut bundles InSync — locally "in sync", yet
    # `proton-drive filesystem info` said Node not found for the same files, and a direct CLI
    # upload succeeded instantly, proving the CLI/API channel live and the sync app stalled.
    # Waiting doesn't help this class (it is NOT upload lag), and the only watchdog was
    # MaxUnconfirmedAgeDays (7 days). When CLI verification is available and a bundle stays
    # unconfirmed because the server says the node is ABSENT, verify now cross-checks the local
    # Cloud Files state: CF-InSync while the server still says absent at the same instant is the
    # stall signature (a signature, not a proof — docs/design.md names the transients and the
    # debounce), surfaced as its own finding with the remedy (restart the app) instead of the
    # generic — and wait-and-see — "newest bundle not confirmed on Proton".
    BeforeEach {
        $script:drive = New-Sandbox; $script:repo = New-TestRepo
        Install-ProtonBackup -RepoPath $script:repo
        # Grace budget (defaults from cfg.VerifySeconds): 1s keeps never-confirming runs fast.
        Set-ProtonBackupConfig -Key VerifySeconds -Value 1
    }
    AfterEach { Clear-Sandbox }

    It 'CF-InSync bundles absent on Proton across the fleet → distinct stall finding, not the generic unconfirmed one' {
        # RED: reproduces the incident. Two repos, CLI live, every info call answers Node not
        # found, while the local Cloud Files probe reports InSync. Pre-detector code reported the
        # generic "newest bundle not confirmed on Proton" — indistinguishable from upload lag,
        # inviting exactly the wrong response (wait), while nothing uploads for up to 7 days.
        $repo2 = New-TestRepo; Install-ProtonBackup -RepoPath $repo2
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $true } `
            -InfoRunner { param($cp, $cli) [pscustomobject]@{ ExitCode = 1; Output = 'Node not found' } } `
            -SyncCheck { param($p) $true }
        $r.ExitCode | Should -Be 1
        @($r.Repos | Where-Object State -ne 'attention').Count | Should -Be 0
        $joined = $r.Findings -join "`n"
        $joined | Should -Match 'sync app appears stalled'
        $joined | Should -Match 'restart the Proton Drive app'
        $joined | Should -Not -Match 'not confirmed on Proton'
        # per-repo rows carry the stall finding too — the report says WHICH repos are affected
        @($r.Repos | Where-Object { ($_.Findings -join ' ') -match 'stalled' }).Count | Should -Be 2
        # and the durable report — what Get-ProtonBackupStatus and post-mortems actually read —
        # carries the diagnosis, not just the in-memory return (round-1 peer review, Kimi)
        $lv = Get-Content (Join-Path (Get-GpbRoot) 'last-verify.json') -Raw | ConvertFrom-Json
        @($lv.Repos | Where-Object { ($_.Findings -join ' ') -match 'sync app appears stalled' }).Count | Should -Be 2
    }

    It 'TWO contradicting repos with the grace disabled are still diagnosed (fleet corroboration needs no window)' {
        # GUARD (mutation-checked: gating every diagnosis on a nonzero grace fails this): the
        # debounce arms are independent by design — corroboration across repos stands on its own
        # because one machine-wide app owns every upload. Known residual, documented in
        # design.md: with the grace explicitly off, correlated server-side metadata lag across
        # simultaneously-landing uploads could in principle satisfy both halves at once; the
        # same-instant re-probe narrows that window but no instant observation can close it.
        # Accepted because grace-off is an explicit operator override, never the default.
        $repo2 = New-TestRepo; Install-ProtonBackup -RepoPath $repo2
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $true } -GraceSeconds 0 `
            -InfoRunner { param($cp, $cli) [pscustomobject]@{ ExitCode = 1; Output = 'Node not found' } } `
            -SyncCheck { param($p) $true }
        $r.ExitCode | Should -Be 1
        ($r.Findings -join "`n") | Should -Match 'sync app appears stalled'
        ($r.Findings -join "`n") | Should -Not -Match 'not confirmed on Proton'
    }

    It 'a SINGLE contradictory repo that out-waited the grace window is a stall; a repo confirming during grace never counts' {
        # RED: debounce arm 2. One contradictory repo could be transient metadata lag (the upload
        # landed; the info endpoint hasn't caught up) — but a repo still server-absent after a
        # full nonzero grace window of polling has persistence, not a moment's lag. Mixed fleet:
        # repo A confirms on its second info call (ordinary upload lag, resolved in-window) and
        # must NOT count as a suspect; repo B never confirms and its CF state claims InSync.
        $repoB = New-TestRepo; Install-ProtonBackup -RepoPath $repoB
        $slugB = Get-GpbSlug -RepoPath $repoB
        $script:infoCalls = @{}
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $true } -GraceSeconds 1 -GracePollSeconds 1 `
            -InfoRunner {
                param($cp, $cli)
                $script:infoCalls[$cp] = 1 + [int]$script:infoCalls[$cp]
                if ($cp -like "*$slugB*" -or $script:infoCalls[$cp] -lt 2) {
                    [pscustomobject]@{ ExitCode = 1; Output = 'Node not found' }
                } else { [pscustomobject]@{ ExitCode = 0; Output = "state: 'active'" } }
            } `
            -SyncCheck { param($p) $true }
        $r.ExitCode | Should -Be 1
        $resolvedA = (Resolve-Path $script:repo).Path
        $resolvedB = (Resolve-Path $repoB).Path
        (@($r.Repos) | Where-Object RepoPath -eq $resolvedA).State | Should -Be 'ok'
        (@($r.Repos) | Where-Object RepoPath -eq $resolvedB).State | Should -Be 'attention'
        $joined = $r.Findings -join "`n"
        $joined | Should -Match ([regex]::Escape("sync app appears stalled: bundle marked in-sync locally but absent on Proton for $resolvedB"))
        $joined | Should -Not -Match ([regex]::Escape($resolvedA))
        $joined | Should -Not -Match 'not confirmed on Proton'
    }

    It 'a throwing Cloud Files probe degrades to the generic finding: the pass completes and the report survives' {
        # RED (fault isolation, same rule as every probe in this function): the cross-check is a
        # diagnostic refinement — its own failure must neither abort the pass (Complete=$false,
        # skipping the marker pass) nor mask the underlying unconfirmed verdict. It leaves its
        # own breadcrumb finding so the broken probe doesn't hide silently either.
        $repo2 = New-TestRepo; Install-ProtonBackup -RepoPath $repo2
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $true } `
            -InfoRunner { param($cp, $cli) [pscustomobject]@{ ExitCode = 1; Output = 'Node not found' } } `
            -SyncCheck { param($p) throw 'CF probe exploded' }
        $r.ExitCode | Should -Be 1
        $r.Complete | Should -BeTrue
        $joined = $r.Findings -join "`n"
        $joined | Should -Match 'not confirmed on Proton'
        $joined | Should -Match 'stall cross-check failed'
        $joined | Should -Not -Match 'sync app appears stalled'
        (Get-Content (Join-Path (Get-GpbRoot) 'last-verify.json') -Raw | ConvertFrom-Json).ExitCode | Should -Be 1
    }

    It 'an upload landing between the cached CLI verdict and the cross-check is a late confirmation, not a stall (same-instant re-probe)' {
        # RED (round-1 peer review — the one finding all four engines raised): the contradiction
        # used to be assembled from observations taken at different times — the CLI's "node
        # absent" cached during the in-pass check or grace rounds, the Cloud Files probe run
        # after the grace wait. A healthy upload completing in between yields CF-InSync against
        # a STALE absent verdict — the exact stall signature — and diagnosed a working sync app,
        # telling the user to restart it. The cross-check must re-ask the server at the same
        # moment and take a fresh 'active' as the late confirmation it is. Ordering is
        # deterministic without timing games: in CLI mode -SyncCheck is invoked ONLY by the
        # cross-check, so its first call IS the moment the upload "lands" (it drops the flag) —
        # every info call before it answers absent, every one after answers active.
        $script:flag = Join-Path $TestDrive "uploaded-$([guid]::NewGuid().ToString('N').Substring(0,8)).flag"
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $true } -GraceSeconds 1 -GracePollSeconds 1 `
            -InfoRunner {
                param($cp, $cli)
                if (Test-Path -LiteralPath $script:flag) { [pscustomobject]@{ ExitCode = 0; Output = "state: 'active'" } }
                else { [pscustomobject]@{ ExitCode = 1; Output = 'Node not found' } }
            } `
            -SyncCheck { param($p) New-Item -ItemType File -Path $script:flag -Force | Out-Null; $true }
        $r.ExitCode | Should -Be 0
        @($r.Findings) | Should -BeNullOrEmpty
        (@($r.Repos) | Where-Object RepoPath -eq (Resolve-Path $script:repo).Path).State | Should -Be 'ok'
    }

    It 'a SINGLE contradictory repo with the grace disabled stays generic (debounce: one instant observation could be transient)' {
        # GUARD (mutation-checked: diagnosing lone suspects unconditionally fails this): with
        # -GraceSeconds 0 the one contradictory observation happened an instant after the cut —
        # transient metadata lag (upload landed, info endpoint behind) can't be ruled out, and a
        # false "restart the app" costs the user a manual intervention. Fleet corroboration (the
        # first test) or an out-waited window (the mixed-fleet test) is what upgrades it.
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $true } -GraceSeconds 0 `
            -InfoRunner { param($cp, $cli) [pscustomobject]@{ ExitCode = 1; Output = 'Node not found' } } `
            -SyncCheck { param($p) $true }
        $r.ExitCode | Should -Be 1
        ($r.Findings -join "`n") | Should -Match 'not confirmed on Proton'
        ($r.Findings -join "`n") | Should -Not -Match 'sync app appears stalled'
    }

    It 'CLI-absent with the local CF state still pending is plain upload lag: generic finding, no stall claim' {
        # GUARD (mutation-checked: suspecting every CLI-absent repo without the CF half fails
        # this): a genuinely still-uploading bundle is absent on the server AND not yet marked
        # InSync locally — both channels agree, no contradiction. This is the ordinary
        # upload-outlives-the-window case; the verdict must stay "wait/re-run", not "restart".
        $repo2 = New-TestRepo; Install-ProtonBackup -RepoPath $repo2
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $true } `
            -InfoRunner { param($cp, $cli) [pscustomobject]@{ ExitCode = 1; Output = 'Node not found' } } `
            -SyncCheck { param($p) $false }
        $r.ExitCode | Should -Be 1
        ($r.Findings -join "`n") | Should -Match 'not confirmed on Proton'
        ($r.Findings -join "`n") | Should -Not -Match 'sync app appears stalled'
    }

    It 'a CLI auth failure with CF-InSync is NOT a stall: the contradiction requires the server saying the node is absent' {
        # GUARD (mutation-checked: keying on "unconfirmed" instead of the not_in_cloud reason
        # fails this): an auth_error verdict means the CLI couldn't answer whether the node
        # exists — there is no server-truth half to contradict. Diagnosing a stall from it would
        # send the user to restart an app that may be fine when the real fix is the CLI session.
        $repo2 = New-TestRepo; Install-ProtonBackup -RepoPath $repo2
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $true } `
            -InfoRunner { param($cp, $cli) [pscustomobject]@{ ExitCode = 1; Output = 'you need to login' } } `
            -SyncCheck { param($p) $true }
        $r.ExitCode | Should -Be 1
        ($r.Findings -join "`n") | Should -Match 'not confirmed on Proton'
        ($r.Findings -join "`n") | Should -Not -Match 'sync app appears stalled'
    }

    It 'degraded mode (CLI unavailable) is untouched: an unconfirmed fleet reports the generic finding, never a stall' {
        # GUARD: without the CLI there is no server-truth channel — the local Cloud Files state
        # IS the verifier, so the contradiction is unobservable (an InSync answer would simply
        # have confirmed the bundle) and no stall diagnosis must be attempted. Structurally
        # immune twice over: phase B2 is gated on cliReady AND keys on CLI verdict reasons that
        # degraded mode never records.
        $repo2 = New-TestRepo; Install-ProtonBackup -RepoPath $repo2
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $false } -WarningAction SilentlyContinue `
            -SyncCheck { param($p) $false }
        $r.ExitCode | Should -Be 1
        ($r.Findings -join "`n") | Should -Match 'not confirmed on Proton'
        ($r.Findings -join "`n") | Should -Not -Match 'sync app appears stalled'
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

Describe 'Issue #2 hardening (v0.2.1)' {
    BeforeEach {
        $script:dir = Join-Path ([IO.Path]::GetTempPath()) ("gpb21-" + [guid]::NewGuid().ToString('N'))
        New-Item -ItemType Directory -Force -Path $script:dir | Out-Null
        $env:GPB_CONFIG_DIR = $script:dir
        $env:GPB_LOCK_PATH  = Join-Path $script:dir 'test.lock'
        $script:drive = Join-Path $script:dir 'drive'
        New-Item -ItemType Directory -Force -Path $script:drive | Out-Null
        Initialize-ProtonBackup -ProtonDriveRoot $script:drive -WarningAction SilentlyContinue
    }
    AfterEach {
        Remove-Item Env:GPB_CONFIG_DIR -ErrorAction SilentlyContinue
        Remove-Item Env:GPB_LOCK_PATH -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath $script:dir -Recurse -Force -ErrorAction SilentlyContinue
    }

    It 'Verify reports and returns instead of going silent when the pass throws unexpectedly' {
        # Get-GpbMarkerDir is called in the marker pass OUTSIDE the inner fault isolations —
        # a stand-in for any unexpected throw escaping those islands.
        Mock Get-GpbMarkerDir { throw 'boom' } -ModuleName GitProtonBackup
        $r = Invoke-ProtonBackupVerify -CliReadyRunner { $false } -WarningAction SilentlyContinue
        $r.ExitCode | Should -Be 1
        $r.Complete | Should -BeFalse
        $r.IncompleteReason | Should -Be 'error'
        (@($r.Findings) -join ' ') | Should -Match 'verify pass failed unexpectedly'
        # The whole point: the durable report still gets written.
        $lv = Get-Content (Join-Path $script:dir 'last-verify.json') -Raw | ConvertFrom-Json
        $lv.Complete | Should -BeFalse
        $lv.IncompleteReason | Should -Be 'error'
    }

    It 'Uninstall leaves a foreign proton remote in place (with a warning)' {
        $repo = Join-Path $script:dir 'repo'; New-Item -ItemType Directory -Force -Path $repo | Out-Null
        git -C $repo init -q; git -C $repo config user.email t@t; git -C $repo config user.name t
        Set-Content (Join-Path $repo 'a.txt') 'x'; git -C $repo add .; git -C $repo commit -qm init
        $foreign = Join-Path $script:dir 'foreign.git'
        git init --bare -q $foreign
        git -C $repo remote add proton $foreign
        Uninstall-ProtonBackup -RepoPath $repo -WarningVariable wv -WarningAction SilentlyContinue
        (git -C $repo remote get-url proton) | Should -Be $foreign
        (@($wv) -join ' ') | Should -Match 'not a GitProtonBackup mirror'
        Test-Path -LiteralPath $foreign | Should -BeTrue
    }
}
