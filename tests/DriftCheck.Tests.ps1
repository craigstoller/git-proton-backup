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
    must be at least as strict as the helper's own gate (exit 0 AND identity token equal), and
    it must fail closed on a feed it cannot read.
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

    # Runs the drift script in a child pwsh with Invoke-RestMethod shadowed by a function that
    # returns a feed whose windows/x64 entry carries $Sha512 (or no windows/x64 entry at all
    # when $Sha512 is $null). Returns the output lines and the child's exit code.
    function Invoke-Drift {
        param([string]$Helper, [string]$Cli, $Sha512)
        if ($null -eq $Sha512) {
            $files = "@([pscustomobject]@{ Platform = 'linux/x64'; Sha512CheckSum = 'ab' })"
        } else {
            $files = "@([pscustomobject]@{ Platform = 'windows/x64'; Sha512CheckSum = '$Sha512' })"
        }
        $feed = "[pscustomobject]@{ Releases = @([pscustomobject]@{ Version = '0.9.0'; ReleaseDate = '2026-10-05'; Files = $files }) }"
        # `exit $LASTEXITCODE` at the end is load-bearing: pwsh -Command converts any script exit
        # code other than 0 or 1 to 1 unless the command string re-exits with it explicitly.
        $cmd = "function Invoke-RestMethod { param(`$Uri) $feed }; & '$script:driftScript' -Helper '$Helper' -Cli '$Cli'; exit `$LASTEXITCODE"
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
}
