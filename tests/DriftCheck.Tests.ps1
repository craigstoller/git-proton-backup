<#
    Hermetic coverage for scripts/Test-ProtonCliDrift.ps1 (the Proton CLI drift check that
    docs/releasing.md calls for before tagging, before publishing, and weekly between releases).

    Containment: the script is run in a CHILD pwsh so its `exit` codes are observable and its
    network call never happens — the child defines a function named Invoke-RestMethod before
    invoking the script, and a function shadows the cmdlet for command resolution, so the feed
    is whatever the test says it is. The helper and the CLI are .cmd stubs under TestDrive:
    that print the lines the real binaries print and exit with the code the case needs. No
    real helper, no real CLI, no Proton account.

    Why these cases: the script is the FIRST triage step for an uncertified-CLI refusal, so it
    must be at least as strict as the helper's own gate (exit 0 AND identity token equal, the
    token compared case-sensitively as Go's == does), it must fail closed on a feed it cannot
    read or whose entry carries no usable hash, and — because it also runs unattended from a
    weekly scheduled task — its one network call must be bounded.
#>

BeforeAll {
    $script:driftScript = Join-Path $PSScriptRoot '..\scripts\Test-ProtonCliDrift.ps1'
    $script:driftScript = (Resolve-Path $script:driftScript).Path

    # Writes a .cmd stub that prints the given lines and exits with the given code.
    function New-Stub {
        param([string]$Path, [string[]]$Lines, [int]$ExitCode)
        $body = @('@echo off') + ($Lines | ForEach-Object { "echo $_" }) + @("exit /b $ExitCode")
        Set-Content -LiteralPath $Path -Value ($body -join "`r`n") -Encoding ascii
        return $Path
    }

    # The certified linux/x64 SHA-512 the script records, read from the script itself so the
    # tests follow the constant (a certification changes it; the tests need no edit).
    $script:certLinux = [regex]::Match((Get-Content -LiteralPath $script:driftScript -Raw), '\$CertifiedLinuxX64Sha512\s*=\s*''([0-9a-f]{128})''').Groups[1].Value

    # Runs the drift script in a child pwsh with Invoke-RestMethod shadowed by a function that
    # returns a feed whose windows/x64 entry carries $Sha512 (or no windows/x64 entry AND no
    # linux entry at all when $Sha512 is $null). The same release also carries a linux entry
    # whose platform spelling is $LinuxPlatform and whose SHA-512 is $LinuxSha512 (default: the
    # script's certified value), or none with -NoLinux. -OlderLinux adds an older 0.8.0 release
    # that has a linux/x64 entry carrying $LinuxSha512, to show that the leg reads the entry of
    # the LATEST release only. -NoWindows makes the latest release's only entry the linux one
    # (whatever $Sha512 says). -ExtraLinuxSha512 adds a second linux/x64 entry, after the first,
    # carrying that hash. Returns the output lines and the child's exit code.
    function Invoke-Drift {
        param([string]$Helper, [string]$Cli, $Sha512, $LinuxSha512 = $script:certLinux, [string]$LinuxPlatform = 'linux/x64', [switch]$NoLinux, [switch]$OlderLinux, [switch]$NoWindows, $ExtraLinuxSha512 = $null)
        $win = "[pscustomobject]@{ Platform = 'windows/x64'; Sha512CheckSum = '$Sha512' }"
        $lin = "[pscustomobject]@{ Platform = '$LinuxPlatform'; Sha512CheckSum = '$LinuxSha512' }"
        $lin2 = "[pscustomobject]@{ Platform = 'linux/x64'; Sha512CheckSum = '$ExtraLinuxSha512' }"
        if ($NoWindows) {
            $files = "@($lin)"
        } elseif ($null -eq $Sha512) {
            $files = '@()'
        } elseif ($NoLinux) {
            $files = "@($win)"
        } elseif ($null -ne $ExtraLinuxSha512) {
            $files = "@($win, $lin, $lin2)"
        } else {
            $files = "@($win, $lin)"
        }
        $releases = "[pscustomobject]@{ Version = '0.9.0'; ReleaseDate = '2026-10-05'; Files = $files }"
        if ($OlderLinux) {
            $releases += ", [pscustomobject]@{ Version = '0.8.0'; ReleaseDate = '2026-09-01'; Files = @($win, [pscustomobject]@{ Platform = 'linux/x64'; Sha512CheckSum = '$LinuxSha512' }) }"
        }
        $feed = "[pscustomobject]@{ Releases = @($releases) }"
        # `exit $LASTEXITCODE` at the end is load-bearing: pwsh -Command converts any script exit
        # code other than 0 or 1 to 1 unless the command string re-exits with it explicitly.
        # The shadow also reports the -TimeoutSec it was given, on stderr (merged into Output), so
        # a test can see whether the script bounded its one network call.
        $cmd = "function Invoke-RestMethod { param(`$Uri, `$TimeoutSec) [Console]::Error.WriteLine('FEED-SHADOW TimeoutSec=' + `$TimeoutSec); $feed }; & '$script:driftScript' -Helper '$Helper' -Cli '$Cli'; exit `$LASTEXITCODE"
        $out = & pwsh -NoProfile -NonInteractive -Command $cmd 2>&1
        return [pscustomobject]@{ Output = @($out | ForEach-Object { [string]$_ }); ExitCode = $LASTEXITCODE }
    }

    $script:certifiedToken = 'cli-drive@0.9.0+75612299'
    $script:helperOk = New-Stub -Path (Join-Path $TestDrive 'helper-ok.cmd') -Lines @("git-remote-proton v9.9.9 (certified CLI: $script:certifiedToken)") -ExitCode 0
    $script:helperOld = New-Stub -Path (Join-Path $TestDrive 'helper-old.cmd') -Lines @('git-remote-proton v9.9.9 (certified CLI: cli-drive@0.8.0+06e8c605)') -ExitCode 0
    $script:helperBad = New-Stub -Path (Join-Path $TestDrive 'helper-bad.cmd') -Lines @("git-remote-proton v9.9.9 (certified CLI: $script:certifiedToken)") -ExitCode 1
    $cliLines = @("Proton Drive CLI $script:certifiedToken", 'Proton Drive SDK js@0.0.0+stub', 'You are running the latest version.')
    $script:cliOk = New-Stub -Path (Join-Path $TestDrive 'cli-ok.cmd') -Lines $cliLines -ExitCode 0
    $script:cliBad = New-Stub -Path (Join-Path $TestDrive 'cli-bad.cmd') -Lines $cliLines -ExitCode 3
    $script:cliOkSha = (Get-FileHash -LiteralPath $script:cliOk -Algorithm SHA512).Hash.ToLower()
    $script:cliBadSha = (Get-FileHash -LiteralPath $script:cliBad -Algorithm SHA512).Hash.ToLower()

    # A helper/CLI pair whose tokens differ ONLY in the case of the build id: the helper's Go
    # compare (==) refuses that pair, so the script must not read it as MATCH.
    $script:helperHex = New-Stub -Path (Join-Path $TestDrive 'helper-hex.cmd') -Lines @('git-remote-proton v9.9.9 (certified CLI: cli-drive@0.9.0+0abc0001)') -ExitCode 0
    $script:cliHexUpper = New-Stub -Path (Join-Path $TestDrive 'cli-hex-upper.cmd') -Lines @('Proton Drive CLI cli-drive@0.9.0+0ABC0001', 'Proton Drive SDK js@0.0.0+stub') -ExitCode 0
    $script:cliHexUpperSha = (Get-FileHash -LiteralPath $script:cliHexUpper -Algorithm SHA512).Hash.ToLower()
}

Describe 'Test-ProtonCliDrift.ps1' {
    It 'reports MATCH (exit 0) when the CLI bytes equal the feed, its token equals the certified token, and the feed version equals the certified semver' {
        $r = Invoke-Drift -Helper $script:helperOk -Cli $script:cliOk -Sha512 $script:cliOkSha
        $r.ExitCode | Should -Be 0
        $r.Output | Should -Contain 'DRIFT CHECK: MATCH'
    }

    It 'reports MISMATCH (exit 1) naming the token leg when the helper certifies a different build' {
        $r = Invoke-Drift -Helper $script:helperOld -Cli $script:cliOk -Sha512 $script:cliOkSha
        $r.ExitCode | Should -Be 1
        ($r.Output -join "`n") | Should -Match "installed CLI == helper's certified token: False"
    }

    It 'STOPs (exit 2) when the feed carries no windows/x64 entry (fail closed)' {
        $r = Invoke-Drift -Helper $script:helperOk -Cli $script:cliOk -Sha512 $null
        $r.ExitCode | Should -Be 2
        ($r.Output -join "`n") | Should -Match 'DRIFT CHECK: STOP'
    }

    It 'STOPs (exit 2) when the CLI prints its identity line but exits non-zero — the helper would refuse, so MATCH is wrong' {
        $r = Invoke-Drift -Helper $script:helperOk -Cli $script:cliBad -Sha512 $script:cliBadSha
        $r.ExitCode | Should -Be 2
        ($r.Output -join "`n") | Should -Match 'DRIFT CHECK: STOP \(.*exited 3'
    }

    It 'STOPs (exit 2) when the helper itself exits non-zero' {
        $r = Invoke-Drift -Helper $script:helperBad -Cli $script:cliOk -Sha512 $script:cliOkSha
        $r.ExitCode | Should -Be 2
        ($r.Output -join "`n") | Should -Match 'DRIFT CHECK: STOP \(.*exited 1'
    }

    It 'bounds its one network call with a positive -TimeoutSec, so an unreachable feed cannot hang a scheduled run' {
        $r = Invoke-Drift -Helper $script:helperOk -Cli $script:cliOk -Sha512 $script:cliOkSha
        $r.ExitCode | Should -Be 0
        ($r.Output -join "`n") | Should -Match 'FEED-SHADOW TimeoutSec=[1-9][0-9]*'
    }

    It 'STOPs (exit 2) when the feed''s windows/x64 entry carries an empty SHA-512 — fail closed, not a MISMATCH verdict' {
        $r = Invoke-Drift -Helper $script:helperOk -Cli $script:cliOk -Sha512 ''
        $r.ExitCode | Should -Be 2
        ($r.Output -join "`n") | Should -Match 'DRIFT CHECK: STOP \(.*SHA-512'
    }

    It 'reports MISMATCH (exit 1) naming the token leg when the CLI token differs from the certified token only in case' {
        $r = Invoke-Drift -Helper $script:helperHex -Cli $script:cliHexUpper -Sha512 $script:cliHexUpperSha
        $r.ExitCode | Should -Be 1
        ($r.Output -join "`n") | Should -Match "installed CLI == helper's certified token: False"
    }

    It 'labels the CLI as given with -Cli rather than as found on PATH' {
        $r = Invoke-Drift -Helper $script:helperOk -Cli $script:cliOk -Sha512 $script:cliOkSha
        ($r.Output -join "`n") | Should -Match '(?m)^cli \(-Cli\)\s+: '
        ($r.Output -join "`n") | Should -Not -Match 'cli on PATH'
    }

    # The linux/x64 leg (D8(a)): the feed's linux/x64 SHA-512 for the SAME release as the
    # windows/x64 entry it picked, against the value the script records. It cannot see an
    # installed Linux CLI; nothing on Windows hashes one.
    It 'records the certified linux/x64 SHA-512 as 128 lowercase hex characters' {
        $script:certLinux | Should -Match '^[0-9a-f]{128}$'
    }

    It 'reports MISMATCH (exit 1) naming the linux leg when the feed''s linux/x64 SHA-512 differs from the certified value' {
        $r = Invoke-Drift -Helper $script:helperOk -Cli $script:cliOk -Sha512 $script:cliOkSha -LinuxSha512 ('0' * 128)
        $r.ExitCode | Should -Be 1
        ($r.Output -join "`n") | Should -Match 'DRIFT CHECK: MISMATCH \(installed CLI == feed: True; installed CLI == helper''s certified token: True; feed version == certified semver: True; feed linux/x64 == certified linux/x64: False\) -- STOP'
    }

    It 'reports MISMATCH (exit 1) naming the linux leg when the latest release has no linux/x64 entry' {
        $r = Invoke-Drift -Helper $script:helperOk -Cli $script:cliOk -Sha512 $script:cliOkSha -NoLinux
        $r.ExitCode | Should -Be 1
        ($r.Output -join "`n") | Should -Match 'feed linux/x64 == certified linux/x64: False'
    }

    It 'STOPs (exit 2) when the feed''s linux/x64 entry carries a non-128-hex SHA-512 (fail closed)' {
        $r = Invoke-Drift -Helper $script:helperOk -Cli $script:cliOk -Sha512 $script:cliOkSha -LinuxSha512 'abc123'
        $r.ExitCode | Should -Be 2
        ($r.Output -join "`n") | Should -Match 'DRIFT CHECK: STOP \(.*linux/x64.*SHA-512'
    }

    It 'accepts the linux-x64 platform spelling as well as linux/x64' {
        # Read as the linux entry, a matching hash is MATCH ...
        $r = Invoke-Drift -Helper $script:helperOk -Cli $script:cliOk -Sha512 $script:cliOkSha -LinuxPlatform 'linux-x64'
        $r.ExitCode | Should -Be 0
        $r.Output | Should -Contain 'DRIFT CHECK: MATCH'
        # ... and a malformed hash is a STOP; were the entry ignored it would be a missing-entry MISMATCH.
        $r = Invoke-Drift -Helper $script:helperOk -Cli $script:cliOk -Sha512 $script:cliOkSha -LinuxPlatform 'linux-x64' -LinuxSha512 'abc123'
        $r.ExitCode | Should -Be 2
    }

    # Guards on the leg's other rules: each fails before the leg exists (the check then says MATCH).
    It 'compares the linux/x64 SHA-512 case-sensitively' {
        $r = Invoke-Drift -Helper $script:helperOk -Cli $script:cliOk -Sha512 $script:cliOkSha -LinuxSha512 $script:certLinux.ToUpper()
        $r.ExitCode | Should -Be 1
        ($r.Output -join "`n") | Should -Match 'feed linux/x64 == certified linux/x64: False'
    }

    It 'reads the linux/x64 entry of the latest release only, not of an older one' {
        $r = Invoke-Drift -Helper $script:helperOk -Cli $script:cliOk -Sha512 $script:cliOkSha -NoLinux -OlderLinux
        $r.ExitCode | Should -Be 1
        ($r.Output -join "`n") | Should -Match 'feed linux/x64 == certified linux/x64: False'
    }

    It 'does not take another linux build (linux/x64-musl) for linux/x64' {
        $r = Invoke-Drift -Helper $script:helperOk -Cli $script:cliOk -Sha512 $script:cliOkSha -LinuxPlatform 'linux/x64-musl'
        $r.ExitCode | Should -Be 1
        ($r.Output -join "`n") | Should -Match 'feed linux/x64 == certified linux/x64: False'
    }

    # GUARDs (final-review fix C11): both shapes already behave this way; these pin it.
    It 'STOPs (exit 2) when the latest release has a lone linux/x64 entry carrying the certified hash and no windows/x64 entry' {
        $r = Invoke-Drift -Helper $script:helperOk -Cli $script:cliOk -Sha512 $null -NoWindows
        $r.ExitCode | Should -Be 2
        ($r.Output -join "`n") | Should -Match 'DRIFT CHECK: STOP \(.*no windows/x64'
    }

    It 'reports MISMATCH (exit 1) naming the linux leg when the latest release has two linux/x64 entries, one certified and one not' {
        $r = Invoke-Drift -Helper $script:helperOk -Cli $script:cliOk -Sha512 $script:cliOkSha -ExtraLinuxSha512 ('0' * 128)
        $r.ExitCode | Should -Be 1
        ($r.Output -join "`n") | Should -Match 'DRIFT CHECK: MISMATCH \(installed CLI == feed: True; installed CLI == helper''s certified token: True; feed version == certified semver: True; feed linux/x64 == certified linux/x64: False\) -- STOP'
    }
}
