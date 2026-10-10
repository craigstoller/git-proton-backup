# Proton CLI drift check for git-proton-backup (docs/releasing.md: before tagging, before
# publishing, and weekly between releases).
#
# Three legs, each named in the verdict line:
#   1. the proton-drive the helper would spawn (first on PATH unless -Cli is given) has the
#      bytes Proton's feed advertises for windows/x64 -- drift of what is installed;
#   2. that CLI's own identity token equals the helper's compiled-in certified token -- the
#      check a hash alone cannot make when Proton re-cuts a version under the same number;
#   3. the feed's version equals the certified token's semver -- the feed has moved past the pin.
#
# -Helper: the exe whose certified token is authoritative. Use the frozen release CANDIDATE
#   before tagging or publishing a pin-moving release (the installed helper still carries the
#   old token until the release is installed); use the installed helper (the default, on PATH)
#   between releases and after this release is installed.
# -Cli: defaults to the proton-drive first on PATH, which is the one the helper spawns.
#
# Exit codes: 0 MATCH, 1 MISMATCH, 2 STOP (the check itself could not run: feed, parse, PATH).
param(
    [string]$Helper = 'git-remote-proton',
    [string]$Cli = ''
)
$ErrorActionPreference = 'Stop'

function Stop-Drift([string]$why) { "DRIFT CHECK: STOP ($why)"; exit 2 }

if (-not $Cli) {
    $c = Get-Command proton-drive -ErrorAction SilentlyContinue
    if (-not $c) { Stop-Drift 'no proton-drive on PATH' }
    $Cli = $c.Source
}
if (-not (Test-Path -LiteralPath $Cli)) { Stop-Drift "CLI not found: $Cli" }

try { $m = Invoke-RestMethod 'https://proton.me/download/drive/cli/version.json' } catch { Stop-Drift "feed unreachable: $_" }
$w = @($m.Releases | ForEach-Object {
    $r = $_
    $r.Files | Where-Object { $_.Platform -in @('windows/x64', 'windows-x64') } | ForEach-Object {
        [pscustomobject]@{ Version = $r.Version; Date = $r.ReleaseDate; Sha512 = ([string]$_.Sha512CheckSum).ToLower() }
    }
})
if ($w.Count -lt 1) { Stop-Drift 'feed malformed or no windows/x64 entry (fail closed)' }
$w | Format-Table -AutoSize | Out-String | ForEach-Object { $_.TrimEnd() }

# [semver], not [version]: a prerelease tag such as 0.10.0-beta must sort, not throw.
try { $latest = $w | Sort-Object { [semver]$_.Version } -Descending | Select-Object -First 1 } catch { Stop-Drift "unparseable feed version: $_" }

$installedSha = (Get-FileHash -LiteralPath $Cli -Algorithm SHA512).Hash.ToLower()

try { $helperOut = (& $Helper --version 2>&1) -join ' ' } catch { Stop-Drift "could not run the helper '$Helper': $_" }
if ($helperOut -notmatch 'certified CLI: (cli-drive@[0-9]+\.[0-9]+\.[0-9]+\+[0-9a-f]+)') { Stop-Drift "could not parse the helper's certified token from: $helperOut" }
$certified = $Matches[1]
$certifiedSemver = $certified -replace '^cli-drive@([0-9]+\.[0-9]+\.[0-9]+)\+.*$', '$1'

# The CLI's identity line, captured in this scope (never through a pipeline, where $Matches
# need not survive): the FIRST line of the shape "Proton Drive CLI <token>".
$cliToken = $null
try { $cliLines = @(& $Cli --version 2>&1) } catch { Stop-Drift "could not run the CLI '$Cli': $_" }
foreach ($line in $cliLines) {
    if ([string]$line -match '^Proton Drive CLI (cli-drive@\S+)$') { $cliToken = $Matches[1]; break }
}
if (-not $cliToken) { Stop-Drift "could not parse the installed CLI's identity line from: $($cliLines -join ' | ')" }

"cli on PATH        : $Cli"
"feed latest        : $($latest.Version) $($latest.Date)"
"installed CLI      : $cliToken  sha512 $installedSha"
"helper certifies   : $certified  (semver $certifiedSemver; helper $Helper)"

$hashOk  = $latest.Sha512 -eq $installedSha
$tokenOk = $cliToken -eq $certified
$verOk   = $latest.Version -eq $certifiedSemver
if ($hashOk -and $tokenOk -and $verOk) { 'DRIFT CHECK: MATCH'; exit 0 }
"DRIFT CHECK: MISMATCH (installed CLI == feed: $hashOk; installed CLI == helper's certified token: $tokenOk; feed version == certified semver: $verOk) -- STOP"
exit 1
