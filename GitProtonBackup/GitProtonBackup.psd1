@{
    RootModule        = 'GitProtonBackup.psm1'
    ModuleVersion     = '0.8.1'
    GUID              = 'f3e7e2c7-df15-445e-8957-5e8aad6cfeb4'
    Author            = 'Craig Stoller'
    Description       = 'Git-native backups: git push your repos to Proton Drive.'
    PowerShellVersion = '7.4'
    FunctionsToExport = @(
        'Initialize-ProtonBackup',
        'Install-ProtonBackup',
        'Uninstall-ProtonBackup',
        'Repair-ProtonBackup',
        'Get-ProtonBackupStatus',
        'Invoke-ProtonBackupVerify',
        'Install-ProtonBackupTask',
        'Get-ProtonBackupConfig',
        'Set-ProtonBackupConfig',
        'Invoke-ProtonBackupHook'
    )
    CmdletsToExport = @()
    AliasesToExport = @()
    Copyright   = '(c) 2026 Craig Stoller. MIT.'
    PrivateData = @{
        PSData = @{
            Tags       = @('git','backup','proton','proton-drive','bundle','Windows')
            LicenseUri = 'https://github.com/craigstoller/git-proton-backup/blob/main/LICENSE'
            ProjectUri = 'https://github.com/craigstoller/git-proton-backup'
            ReleaseNotes = @'
Windows only (it rides the Proton Drive sync app). PowerShell 7.4+. Backs up
committed history only, never your working tree. The module performs no
encryption of its own: bundles are ordinary git bundles and the E2EE is
Proton's. Not affiliated with Proton.

What changed in each version is in the changelog:
https://github.com/craigstoller/git-proton-backup/blob/main/CHANGELOG.md
'@
        }
    }
}
