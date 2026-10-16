<#
    Guard that the README's Linux CLI install commands follow the code.

    The README tells a Linux user to download Proton's CLI from a versioned URL and to check the
    download against a SHA-512. Both are certification touch points (docs/releasing.md, "Between
    releases"): the version is the semver of transport.CertifiedCLI, and the SHA-512 is the
    constant scripts/Test-ProtonCliDrift.ps1 records for the certified linux/x64 build. A
    certification that moves one and forgets the README would leave the README telling a Linux
    user to install a build the helper refuses, or to verify against a hash the gate never ran.
    These two tests make that a red build instead of a user's first-run failure.

    Hermetic: reads three files from the repo, runs nothing, touches no account.
#>

BeforeAll {
    $script:repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path

    $cliGo   = Get-Content -LiteralPath (Join-Path $script:repoRoot 'internal/transport/cli.go') -Raw
    $drift   = Get-Content -LiteralPath (Join-Path $script:repoRoot 'scripts/Test-ProtonCliDrift.ps1') -Raw
    $script:readme = Get-Content -LiteralPath (Join-Path $script:repoRoot 'README.md') -Raw

    # CertifiedCLI is the exact pinned token, e.g. cli-drive@0.9.0+75612299; its semver is the
    # download URL's version segment.
    $tokenMatch = [regex]::Match($cliGo, 'const\s+CertifiedCLI\s*=\s*"(cli-drive@(\d+\.\d+\.\d+)\+[0-9a-f]+)"')
    if (-not $tokenMatch.Success) { throw 'ReadmePins: could not read the CertifiedCLI constant from internal/transport/cli.go' }
    $script:certifiedSemver = $tokenMatch.Groups[2].Value

    $shaMatch = [regex]::Match($drift, '\$CertifiedLinuxX64Sha512\s*=\s*''([0-9a-f]{128})''')
    if (-not $shaMatch.Success) { throw 'ReadmePins: could not read $CertifiedLinuxX64Sha512 from scripts/Test-ProtonCliDrift.ps1' }
    $script:certifiedLinuxSha512 = $shaMatch.Groups[1].Value
}

Describe 'README Linux CLI pins' {
    # RED: both tests below were written before the README had a Linux install block, and failed
    # on their Should -BeTrue assertions until it did (Task 11 report). They now guard the pins.
    It 'pins the README''s Linux CLI download to the certified version' {
        $url = "https://proton.me/download/drive/cli/$($script:certifiedSemver)/linux-x64/proton-drive"
        $script:readme.Contains($url) | Should -BeTrue -Because "README.md must contain $url (the semver of transport.CertifiedCLI)"
    }

    It 'pins the README''s Linux CLI SHA-512 to the drift script''s certified value' {
        # `sha512sum -c` takes "<hash>  <name>" with two spaces.
        $line = "$($script:certifiedLinuxSha512)  proton-drive"
        $script:readme.Contains($line) | Should -BeTrue -Because 'README.md must contain the drift script''s $CertifiedLinuxX64Sha512, then two spaces, then proton-drive'
    }
}
