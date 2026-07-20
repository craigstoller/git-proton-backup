<# Installs the GitProtonBackup module for the current user (until PSGallery publish). #>
[CmdletBinding()]
param([switch]$Force)
$dest = Join-Path ([Environment]::GetFolderPath('MyDocuments')) 'PowerShell\Modules\GitProtonBackup'
if ((Test-Path $dest) -and -not $Force) { throw "Already installed at $dest — re-run with -Force to overwrite." }
New-Item -ItemType Directory -Path $dest -Force | Out-Null
Copy-Item -Path (Join-Path $PSScriptRoot 'GitProtonBackup\*') -Destination $dest -Recurse -Force
Write-Host "Installed. Start with: Import-Module GitProtonBackup; Initialize-ProtonBackup"
