@{
    RootModule        = 'GitProtonBackup.psm1'
    ModuleVersion     = '0.6.0'
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
0.2.3 - Fail-closed fix in Cloud Files sync-state decoding.

CF_PLACEHOLDER_STATE_INVALID (0xffffffff) arrives through the int-returning
P/Invoke as -1, and the decoder masked bits straight off it, so a file whose
state Windows could not determine reported IN_SYNC as true. Because that alone
drives backed-up state and retention pruning when the Proton CLI is
unavailable, an unreadable state could have permitted a bundle to be pruned.
Negatives now fail closed; unknown positive bits are still ignored so a future
Windows state cannot suppress a real IN_SYNC.

Windows only (it rides the Proton Drive sync app). PowerShell 7.4+. Backs up
committed history only, never your working tree. The module performs no
encryption of its own: bundles are ordinary git bundles and the E2EE is
Proton's. Not affiliated with Proton.

Full changelog: https://github.com/craigstoller/git-proton-backup/blob/main/CHANGELOG.md
'@
        }
    }
}
