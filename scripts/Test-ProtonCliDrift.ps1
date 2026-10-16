# Proton CLI drift check for git-proton-backup (docs/releasing.md: before tagging, before
# publishing, and weekly between releases).
#
# Four legs, each named in the verdict line:
#   1. the proton-drive the helper would spawn (first on PATH unless -Cli is given) has the
#      bytes Proton's feed advertises for windows/x64 -- drift of what is installed;
#   2. that CLI's own identity token equals the helper's compiled-in certified token -- the
#      check a hash alone cannot make when Proton re-cuts a version under the same number;
#   3. the feed's version equals the certified token's semver -- the feed has moved past the pin;
#   4. the feed's linux/x64 SHA-512 for that SAME release equals $CertifiedLinuxX64Sha512 below --
#      a Linux re-cut under the same version number passes the helper's token check, so only the
#      feed's hash can show it. This leg compares the feed with a recorded value: it cannot see
#      an installed Linux CLI (nothing on Windows hashes one). A release with no linux entry is a
#      MISMATCH (Proton dropped or delayed Linux: worth a certification look); a linux entry
#      whose SHA-512 is not 128 hex characters is a STOP.
#
# -Helper: the exe whose certified token is authoritative. Use the frozen release CANDIDATE
#   before tagging or publishing a pin-moving release (the installed helper still carries the
#   old token until the release is installed); use the installed helper (the default, on PATH)
#   between releases and after this release is installed.
# -Cli: defaults to the proton-drive first on PATH, which is the one the helper spawns.
#
# Exit codes: 0 MATCH, 1 MISMATCH, 2 STOP (the check itself could not run: the feed was not
# reachable within 30 s or carries no usable windows/x64 or linux/x64 SHA-512, a parse failed,
# PATH).
# The one network call is bounded because this script also runs unattended from a weekly
# scheduled task (docs/releasing.md, "Between releases").
param(
    [string]$Helper = 'git-remote-proton',
    [string]$Cli = ''
)
$ErrorActionPreference = 'Stop'

# The linux/x64 SHA-512 of the certified Linux CLI (0.9.0, the semver of the helper's CertifiedCLI).
# Recorded 2026-10-16: copied from Proton's live feed for 0.9.0 linux/x64, and equal to
# `sha512sum` of the binary the Linux probes and gate ran (~/opt/proton-drive-cli/0.9.0/proton-drive).
# Every certification updates this AND the README's Linux CLI URL version and SHA-512
# (docs/releasing.md, "Between releases"). Keep it on one line in single quotes: the Pester
# tests read it with a regex.
$CertifiedLinuxX64Sha512 = '3533025ba69ae112b64e3e01fbcc1ad0688136a4043f6cf6a72886967d85fdcd9ec235479c2e113171614be5225bfba93427509a05ca5aa6071d924fa7e91ca8'

function Stop-Drift([string]$why) { "DRIFT CHECK: STOP ($why)"; exit 2 }

$cliLabel = 'cli (-Cli)         '
if (-not $Cli) {
    $cliLabel = 'cli (first on PATH)'
    $c = Get-Command proton-drive -ErrorAction SilentlyContinue
    if (-not $c) { Stop-Drift 'no proton-drive on PATH' }
    $Cli = $c.Source
}
if (-not (Test-Path -LiteralPath $Cli)) { Stop-Drift "CLI not found: $Cli" }

try { $m = Invoke-RestMethod 'https://proton.me/download/drive/cli/version.json' -TimeoutSec 30 } catch { Stop-Drift "feed unreachable: $_" }
$w = @($m.Releases | ForEach-Object {
    $r = $_
    # The release's own linux/x64 entries, carried on each windows/x64 record so the linux leg
    # reads the SAME release the windows leg picked as latest, never another release's entry.
    # Exact platform names: linux/arm64 and linux/x64-musl are other builds.
    $linux = @($r.Files | Where-Object { $_.Platform -in @('linux/x64', 'linux-x64') } | ForEach-Object { [string]$_.Sha512CheckSum })
    $r.Files | Where-Object { $_.Platform -in @('windows/x64', 'windows-x64') } | ForEach-Object {
        [pscustomobject]@{ Version = $r.Version; Date = $r.ReleaseDate; Sha512 = ([string]$_.Sha512CheckSum).ToLower(); LinuxSha512 = $linux }
    }
})
if ($w.Count -lt 1) { Stop-Drift 'feed malformed or no windows/x64 entry (fail closed)' }
$w | Format-Table Version, Date, Sha512 -AutoSize | Out-String | ForEach-Object { $_.TrimEnd() }

# [semver], not [version]: a prerelease tag such as 0.10.0-beta must sort, not throw.
try { $latest = $w | Sort-Object { [semver]$_.Version } -Descending | Select-Object -First 1 } catch { Stop-Drift "unparseable feed version: $_" }
# An entry without a usable hash is a feed the check cannot use, not a build that differs:
# STOP, never a MISMATCH verdict built on an empty comparison.
if ($latest.Sha512 -notmatch '^[0-9a-f]{128}$') { Stop-Drift "feed windows/x64 entry for $($latest.Version) has no usable SHA-512 (fail closed)" }
# The same rule for the latest release's linux/x64 entry. No entry at all is not a STOP: it is
# the linux leg failing (a MISMATCH below), because Proton dropping or delaying Linux is drift.
foreach ($l in $latest.LinuxSha512) {
    if ($l -notmatch '^[0-9a-f]{128}$') { Stop-Drift "feed linux/x64 entry for $($latest.Version) has no usable SHA-512 (fail closed)" }
}

$installedSha = (Get-FileHash -LiteralPath $Cli -Algorithm SHA512).Hash.ToLower()

try { $helperOut = (& $Helper --version 2>&1) -join ' ' } catch { Stop-Drift "could not run the helper '$Helper': $_" }
if ($LASTEXITCODE -ne 0) { Stop-Drift "the helper '$Helper' --version exited $LASTEXITCODE" }
if ($helperOut -notmatch 'certified CLI: (cli-drive@[0-9]+\.[0-9]+\.[0-9]+\+[0-9a-f]+)') { Stop-Drift "could not parse the helper's certified token from: $helperOut" }
$certified = $Matches[1]
$certifiedSemver = $certified -replace '^cli-drive@([0-9]+\.[0-9]+\.[0-9]+)\+.*$', '$1'

# The CLI's identity line, captured in this scope (never through a pipeline, where $Matches
# need not survive): the FIRST line of the shape "Proton Drive CLI <token>".
$cliToken = $null
try { $cliLines = @(& $Cli --version 2>&1) } catch { Stop-Drift "could not run the CLI '$Cli': $_" }
# The helper's own gate is exit 0 AND identity token equal (Version() errors on a non-zero exit,
# EnforceCertified then refuses); a CLI that prints its identity line and exits non-zero must
# never read as MATCH here, because this script is the first triage step for that refusal.
if ($LASTEXITCODE -ne 0) { Stop-Drift "the CLI '$Cli' --version exited $LASTEXITCODE" }
foreach ($line in $cliLines) {
    if ([string]$line -match '^Proton Drive CLI (cli-drive@\S+)$') { $cliToken = $Matches[1]; break }
}
if (-not $cliToken) { Stop-Drift "could not parse the installed CLI's identity line from: $($cliLines -join ' | ')" }

"${cliLabel}: $Cli"
"feed latest        : $($latest.Version) $($latest.Date)"
"installed CLI      : $cliToken  sha512 $installedSha"
"helper certifies   : $certified  (semver $certifiedSemver; helper $Helper)"
$feedLinux = @($latest.LinuxSha512)
$feedLinuxShown = if ($feedLinux.Count -gt 0) { $feedLinux -join ', ' } else { '(no linux/x64 entry in that release)' }
"feed linux/x64     : $feedLinuxShown"
"certified linux/x64: $CertifiedLinuxX64Sha512"

# Case-sensitive, as the helper's own Go == compare is: a token differing only in case is a
# different token to the helper, so it must be a MISMATCH here too. The linux leg is the same:
# the feed's text is compared as it is, against the recorded constant, so a difference in case
# is a MISMATCH and never a silent match. No linux entry, or any linux entry that differs, is False.
$hashOk  = $latest.Sha512 -ceq $installedSha
$tokenOk = $cliToken -ceq $certified
$verOk   = $latest.Version -ceq $certifiedSemver
$linuxOk = ($feedLinux.Count -gt 0) -and (@($feedLinux | Where-Object { $_ -cne $CertifiedLinuxX64Sha512 }).Count -eq 0)
if ($hashOk -and $tokenOk -and $verOk -and $linuxOk) { 'DRIFT CHECK: MATCH'; exit 0 }
"DRIFT CHECK: MISMATCH (installed CLI == feed: $hashOk; installed CLI == helper's certified token: $tokenOk; feed version == certified semver: $verOk; feed linux/x64 == certified linux/x64: $linuxOk) -- STOP"
exit 1
