BeforeAll {
    Import-Module "$PSScriptRoot/../GitProtonBackup/GitProtonBackup.psm1" -Force

    function New-HookRepo {
        $repo = Join-Path $TestDrive "hr-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        New-Item -ItemType Directory -Path $repo -Force | Out-Null
        git -C $repo init -qb main
        git -C $repo config user.email 't@t'; git -C $repo config user.name 't'
        Set-Content "$repo/a.txt" 'one'; git -C $repo add .; git -C $repo commit -qm 'c1'
        $repo
    }

    # Sandboxed config object for -OutConfig: an isolated ProtonDriveRoot under $TestDrive, a
    # short VerifySeconds, and a CLI path that is never resolved for real (every It in the
    # Invoke-PushBackupFlow Describe supplies an explicit -CliReadyRunner).
    function New-GpbTestConfig {
        $root = Join-Path $TestDrive "drive-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        New-Item -ItemType Directory -Path $root -Force | Out-Null
        $cfg = Get-GpbDefaultConfig
        $cfg.ProtonDriveRoot = $root
        $cfg.ProtonCli       = 'stub'
        $cfg.VerifySeconds   = 1
        $cfg
    }
}

Describe 'Invoke-PushBackupFlow' {
    BeforeEach {
        $env:GPB_CONFIG_DIR = Join-Path $TestDrive "cfg-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        $env:GPB_LOCK_PATH  = Join-Path $TestDrive "lk-$([guid]::NewGuid().ToString('N').Substring(0,8)).lock"
        $script:repo = New-HookRepo
        $script:cfg  = New-GpbTestConfig
        $script:mdir = Get-GpbMarkerDir
        $script:bundleDir = Get-GpbBundleDir -Config $script:cfg -RepoPath $script:repo
        $script:base = @{ WorkRepo = $script:repo; OutConfig = $script:cfg
                          PollSeconds = 0; LockTimeoutSeconds = 1 }
    }
    AfterEach { Remove-Item Env:GPB_CONFIG_DIR, Env:GPB_LOCK_PATH -ErrorAction SilentlyContinue }

    It 'confirmed path: bundles, prints confirmed on Proton, leaves no marker' {
        Set-Content (Join-Path $script:repo 'dirty.txt') 'uncommitted'   # dirty tree — note coverage
        $o = Invoke-PushBackupFlow @script:base -CliReadyRunner { $true } `
            -InfoRunner { param($cp,$cli) [pscustomobject]@{ ExitCode=0; Output="state: 'active'" } } 6>&1
        ($o -join "`n") | Should -Match 'confirmed on Proton'
        ($o -join "`n") | Should -Match 'uncommitted change\(s\) not included'
        @(Get-ChildItem $script:mdir -Filter '*.json' -ErrorAction SilentlyContinue).Count | Should -Be 0
        @(Get-ChildItem $script:bundleDir -Recurse -Filter '*.bundle').Count | Should -Be 1
    }
    It 'confirmed-after-poll: second InfoRunner call confirms; marker exists DURING the first call' {
        $script:calls = 0; $script:markerSeenMidPoll = $false
        $p = $script:base.Clone()
        $cfg2 = New-GpbTestConfig; $cfg2.VerifySeconds = 30
        $p.OutConfig = $cfg2
        $o = Invoke-PushBackupFlow @p -CliReadyRunner { $true } -InfoRunner {
            param($cp,$cli)
            $script:calls++
            if ($script:calls -eq 1) {
                $script:markerSeenMidPoll = @(Get-ChildItem $script:mdir -Filter '*.json' -ErrorAction SilentlyContinue).Count -eq 1
                return [pscustomobject]@{ ExitCode=1; Output='Node not found' }
            }
            [pscustomobject]@{ ExitCode=0; Output="state: 'active'" }
        } 6>&1
        ($o -join "`n") | Should -Match 'confirmed on Proton'
        $script:calls | Should -Be 2
        $script:markerSeenMidPoll | Should -BeTrue      # pessimistic marker was on disk while polling
        @(Get-ChildItem $script:mdir -Filter '*.json' -ErrorAction SilentlyContinue).Count | Should -Be 0
    }
    It 'timeout path: staged-not-confirmed + verify_timeout marker' {
        $o = Invoke-PushBackupFlow @script:base -CliReadyRunner { $true } `
            -InfoRunner { param($cp,$cli) [pscustomobject]@{ ExitCode=1; Output='Node not found' } } 6>&1
        ($o -join "`n") | Should -Match 'staged, not yet confirmed'
        (Get-Content (Get-ChildItem $script:mdir -Filter '*.json').FullName -Raw | ConvertFrom-Json).Reason | Should -Be 'verify_timeout'
    }
    It 'honors cfg.VerifySeconds (1s) when -VerifySeconds is not passed — not a stale 60s default (regression pin)' {
        # $script:base's -OutConfig has VerifySeconds=1 (New-GpbTestConfig) and -VerifySeconds is
        # deliberately NOT in $script:base — this pins the fallback path
        # (`if (-not $PSBoundParameters.ContainsKey('VerifySeconds')) { $VerifySeconds = $cfg.VerifySeconds }`)
        # that the shim's `vs=0` fix (was `vs=60`) depends on: a mirror with no per-repo
        # `gpb.verifyseconds` override must defer to config, never spin for a hardcoded 60s.
        $elapsed = Measure-Command {
            Invoke-PushBackupFlow @script:base -CliReadyRunner { $true } `
                -InfoRunner { param($cp,$cli) [pscustomobject]@{ ExitCode=1; Output='Node not found' } } 6>&1 | Out-Null
        }
        $elapsed.TotalSeconds | Should -BeLessThan 10
        (Get-Content (Get-ChildItem $script:mdir -Filter '*.json').FullName -Raw | ConvertFrom-Json).Reason | Should -Be 'verify_timeout'
    }
    It 'auth-error path: session-expired message + auth_error marker, no further polling' {
        $script:calls = 0
        $p = $script:base.Clone()
        $cfg2 = New-GpbTestConfig; $cfg2.VerifySeconds = 30
        $p.OutConfig = $cfg2
        $o = Invoke-PushBackupFlow @p -CliReadyRunner { $true } -InfoRunner {
            param($cp,$cli); $script:calls++
            [pscustomobject]@{ ExitCode=1; Output='session expired: unauthorized' }
        } 6>&1
        ($o -join "`n") | Should -Match 'session expired'
        $script:calls | Should -Be 1
        (Get-Content (Get-ChildItem $script:mdir -Filter '*.json').FullName -Raw | ConvertFrom-Json).Reason | Should -Be 'auth_error'
    }
    It 'CLI-unready + IN_SYNC: degraded phrase, never "confirmed on Proton", no marker' {
        $o = Invoke-PushBackupFlow @script:base -CliReadyRunner { $false } -SyncCheck { param($p) $true } 6>&1
        ($o -join "`n") | Should -Match 'in-sync per Cloud Files \(CLI verification unavailable\)'
        ($o -join "`n") | Should -Not -Match 'confirmed on Proton'
        @(Get-ChildItem $script:mdir -Filter '*.json' -ErrorAction SilentlyContinue).Count | Should -Be 0
    }
    It 'CLI-unready + not-in-sync: cli_unready marker' {
        Invoke-PushBackupFlow @script:base -CliReadyRunner { $false } -SyncCheck { param($p) $false } 6>&1 | Out-Null
        (Get-Content (Get-ChildItem $script:mdir -Filter '*.json').FullName -Raw | ConvertFrom-Json).Reason | Should -Be 'cli_unready'
    }
    It 'lock held: defers with deferred_lock marker, no bundle' {
        $holder = Wait-GpbLock -TimeoutSeconds 1
        try {
            $o = Invoke-PushBackupFlow @script:base 6>&1
            ($o -join "`n") | Should -Match 'backup deferred'
            (Get-Content (Get-ChildItem $script:mdir -Filter '*.json').FullName -Raw | ConvertFrom-Json).Reason | Should -Be 'deferred_lock'
            @(Get-ChildItem $script:bundleDir -Recurse -Filter '*.bundle' -ErrorAction SilentlyContinue).Count | Should -Be 0
        } finally { $holder.Close() }
    }
    It 'heals a missing upstream for the current branch; a different branch is left untouched' {
        git -C $script:repo remote add proton (Join-Path $TestDrive 'nowhere.git')
        git -C $script:repo checkout -qb other
        git -C $script:repo config branch.other.remote origin
        git -C $script:repo config branch.other.merge refs/heads/other
        git -C $script:repo checkout -q main
        Invoke-PushBackupFlow @script:base -CliReadyRunner { $true } `
            -InfoRunner { param($cp,$cli) [pscustomobject]@{ ExitCode=0; Output="state: 'active'" } } 6>&1 | Out-Null
        (git -C $script:repo config branch.main.remote)  | Should -Be 'proton'
        (git -C $script:repo config branch.main.merge)   | Should -Be 'refs/heads/main'
        (git -C $script:repo config branch.other.remote) | Should -Be 'origin'   # untouched — not the current branch
    }
    It 'no-bundle path (preflight declined): no_bundle marker, no throw' {
        git -C $script:repo checkout -q --detach
        $o = Invoke-PushBackupFlow @script:base 6>&1
        ($o -join "`n") | Should -Match 'no bundle produced'
        (Get-Content (Get-ChildItem $script:mdir -Filter '*.json').FullName -Raw | ConvertFrom-Json).Reason | Should -Be 'no_bundle'
    }
}

Describe 'Invoke-ProtonBackupHook contract' {
    BeforeAll {
        # The real-shim Its below spawn `git push` -> sh post-receive shim -> a child `pwsh`
        # process that does `Import-Module GitProtonBackup` by bare name. The module's PARENT
        # directory (the repo root) must be on PSModulePath for that bare-name resolution to
        # find it; child processes inherit env vars from this process, so setting it here is
        # enough to reach the whole chain.
        $script:repoRoot = (Resolve-Path "$PSScriptRoot/..").Path
        $script:priorPSModulePath = $env:PSModulePath
        $env:PSModulePath = "$script:repoRoot;$env:PSModulePath"

        function Invoke-RealShimPush {
            param([Parameter(Mandatory)][string]$RepoPath)
            $env:GPB_CONFIG_DIR = Join-Path $TestDrive "shim-cfg-$([guid]::NewGuid().ToString('N').Substring(0,8))"
            $env:GPB_LOCK_PATH  = Join-Path $TestDrive "shim-lock-$([guid]::NewGuid().ToString('N').Substring(0,8)).lock"
            $driveRoot = Join-Path $TestDrive "shim-drive-$([guid]::NewGuid().ToString('N').Substring(0,8))"
            New-Item -ItemType Directory -Path $driveRoot -Force | Out-Null
            $cfg = Get-GpbDefaultConfig
            $cfg.ProtonDriveRoot = $driveRoot
            $cfg.VerifySeconds   = 1
            # Guaranteed-missing CLI path: Test-ProtonCliReady must resolve $false fast, never
            # touching a real Proton Drive CLI that might happen to be installed on this machine.
            $cfg.ProtonCli       = Join-Path $TestDrive 'nonexistent-proton-cli.exe'
            Write-GpbConfig -Config $cfg

            # Install BEFORE the repo has any commits: Install-GpbMirror's own initial push only
            # fires when the repo already has a HEAD, and it would otherwise invoke the (now
            # hook-enabled) mirror itself and produce a first bundle — leaving exactly one real
            # `git push proton` (below) to exercise for the assertions.
            New-Item -ItemType Directory -Path $RepoPath -Force | Out-Null
            git -C $RepoPath init -qb main
            git -C $RepoPath config user.email 't@t'; git -C $RepoPath config user.name 't'
            Install-GpbMirror -RepoPath $RepoPath | Out-Null   # hook ENABLED — GPB_HOOK_DISABLED not set

            Set-Content (Join-Path $RepoPath 'a.txt') 'one'; git -C $RepoPath add .; git -C $RepoPath commit -qm 'c1'
            $out = git -C $RepoPath push proton 2>&1
            [pscustomobject]@{
                Output    = ($out -join "`n")
                BundleDir = (Get-GpbBundleDir -Config $cfg -RepoPath $RepoPath)
            }
        }
    }
    AfterAll {
        $env:PSModulePath = $script:priorPSModulePath
    }
    BeforeEach {
        Remove-Item Env:GPB_HOOK_DISABLED, Env:GPB_CONFIG_DIR, Env:GPB_LOCK_PATH -ErrorAction SilentlyContinue
    }
    AfterEach {
        Remove-Item Env:GPB_HOOK_DISABLED, Env:GPB_CONFIG_DIR, Env:GPB_LOCK_PATH -ErrorAction SilentlyContinue
    }

    It 'GPB_HOOK_DISABLED short-circuits with no side effects' {
        $env:GPB_HOOK_DISABLED = '1'
        try { Invoke-ProtonBackupHook -WorkRepo (Join-Path $TestDrive 'nonexistent') } finally { Remove-Item Env:GPB_HOOK_DISABLED -EA SilentlyContinue }
    }
    It 'a flow failure is swallowed with the verify-pointer message' {
        # No config exists under this sandboxed GPB_CONFIG_DIR -> Read-GpbConfig throws inside
        # the flow -> the hook's catch must swallow it and print the verify pointer, never
        # re-throw (a push must never appear to fail on backup bookkeeping).
        $env:GPB_CONFIG_DIR = Join-Path $TestDrive "hook-cfg-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        $out = Invoke-ProtonBackupHook -WorkRepo (Join-Path $TestDrive 'nonexistent') 6>&1
        ($out -join "`n") | Should -Match 'backup hook error'
        ($out -join "`n") | Should -Match 'Invoke-ProtonBackupVerify'
    }
    It 'hook-inherited GIT_DIR does not hijack the working-repo git calls (regression)' {
        # git runs post-receive with GIT_DIR set (relative '.', cwd = the mirror). Inherited by
        # the pwsh child, it made every `git -C <workrepo>` call fail with "not a git repo" until
        # Invoke-ProtonBackupHook started scrubbing the hook env.
        $repo = Join-Path $TestDrive "gd-repo-$([guid]::NewGuid().ToString('N').Substring(0,8))"
        $r = Invoke-RealShimPush -RepoPath $repo
        $r.Output | Should -Not -Match 'no bundle produced'
        @(Get-ChildItem -LiteralPath $r.BundleDir -Filter '*.bundle' -ErrorAction SilentlyContinue).Count | Should -Be 1
    }
    It 'hostile path shapes survive the real shim (spaces, &, apostrophe, Unicode)' {
        # Pins the quoting-safe env-var transport (GPB_WORKREPO) used by the mirror shim — an
        # inline-quoted shim command would break on the apostrophe/space/& in this path.
        $repo = Join-Path $TestDrive "sp ace & O'Brien münich\repo"
        $r = Invoke-RealShimPush -RepoPath $repo
        $r.Output | Should -Not -Match 'no bundle produced'
        @(Get-ChildItem -LiteralPath $r.BundleDir -Filter '*.bundle' -ErrorAction SilentlyContinue).Count | Should -Be 1
    }
}
