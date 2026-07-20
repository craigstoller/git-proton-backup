@{
    RootModule        = 'GitProtonBackup.psm1'
    ModuleVersion     = '0.1.0'
    GUID              = '00000000-0000-0000-0000-000000000000'   # replaced by New-Guid in Task 11
    Author            = 'Craig Stoller'
    Description       = 'Git-native backups: git push your repos to Proton Drive.'
    PowerShellVersion = '7.4'
    FunctionsToExport = @('*')                                    # narrowed in Task 11
}
