<#
    Hermetic coverage for install.ps1's PATH read-modify-write block (Stage 4 polish wave:
    REG_EXPAND_SZ vs REG_SZ value-kind preservation). Before this file, that logic had NO
    automated regression coverage, because the project's isolation rule forbids real registry
    writes in tests — GitProtonBackup.Tests.ps1's 'install.ps1 helper block' Describe always
    passes -SkipPathUpdate for exactly that reason.

    Stage 4 incident (why this is careful): tests that "sandboxed" via $env:LOCALAPPDATA
    overrides still permanently polluted the REAL user PATH, because
    [Environment]::SetEnvironmentVariable(...,'User') writes HKCU directly — env-var overrides
    do not contain registry writes. install.ps1 also does NOT use SetEnvironmentVariable (see
    its own comment at the top of the PATH block); it writes HKCU\Environment via
    Microsoft.Win32.Registry directly, which is a channel no env-var override touches. This
    file therefore routes the registry through an injected mock OBJECT (-EnvironmentKey) — the
    entire point of Task 2's seam — and never opens the real key for writing.

    Containment, in order:
      1. install.ps1 is copied into a fresh TestDrive: directory with NO GitProtonBackup
         payload beside it, so the module block always takes its skip branch (a payload dir
         DOES exist beside the real repo root, and would write into the real Documents
         modules dir if this script were ever run from there).
      2. $env:LOCALAPPDATA is redirected into TestDrive: for the whole file (restored in
         AfterAll) so the helper Copy-Item lands in TestDrive, not the real
         %LOCALAPPDATA%\Programs\git-proton-backup.
      3. -EnvironmentKey is always a mock; the real HKCU\Environment key is opened read-only,
         only by the AfterAll guard below, and never written.
      4. -EffectivePath is always passed explicitly (never the default, which reads the real
         Machine/User registry PATH — see install.ps1's own param comment).

    AfterAll then re-reads the real user PATH (raw, DoNotExpandEnvironmentNames), the real
    %LOCALAPPDATA%\Programs\git-proton-backup fingerprint, and the real Documents module dir
    fingerprint, and asserts all three are byte-for-byte unchanged from the snapshot taken
    before any test ran. This machine's real user PATH value kind is already REG_SZ (verified
    pre-existing, not caused by any of this work) — the guard captures-before/compares-after
    and never assumes a kind.
#>

BeforeAll {
    # ---- Snapshot REAL state before anything in this file touches env vars or runs the
    # script. This must be the first thing that happens. ----
    $script:originalLocalAppData = $env:LOCALAPPDATA

    function Get-RealUserPathState {
        $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $false)
        try {
            $exists = $null -ne $key.GetValue('Path')
            if ($exists) {
                $raw  = [string]$key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
                $kind = $key.GetValueKind('Path')
            } else {
                $raw  = $null
                $kind = $null
            }
            [pscustomobject]@{ Exists = $exists; Raw = $raw; Kind = $kind }
        } finally {
            $key.Close()
        }
    }

    function Get-DirFingerprint {
        param([string]$Path)
        if (-not (Test-Path -LiteralPath $Path)) { return @() }
        @(Get-ChildItem -LiteralPath $Path -Recurse -File -Force | Sort-Object FullName | ForEach-Object {
            [pscustomobject]@{
                RelativePath = $_.FullName.Substring($Path.Length).TrimStart('\')
                Hash         = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash
            }
        })
    }

    $script:realPathStateBefore = Get-RealUserPathState
    $script:realHelperDirPath   = Join-Path $script:originalLocalAppData 'Programs\git-proton-backup'
    $script:realHelperDirFingerprintBefore = Get-DirFingerprint -Path $script:realHelperDirPath
    $script:realModuleDirPath  = Join-Path ([Environment]::GetFolderPath('MyDocuments')) 'PowerShell\Modules\GitProtonBackup'
    $script:realModuleDirFingerprintBefore = Get-DirFingerprint -Path $script:realModuleDirPath

    # ---- Redirect LOCALAPPDATA into TestDrive: for the rest of this file. ----
    $env:LOCALAPPDATA = Join-Path $TestDrive 'localappdata-root'
    New-Item -ItemType Directory -Path $env:LOCALAPPDATA -Force | Out-Null

    # ---- Test doubles ----

    # Builds a fresh COPY of install.ps1 under TestDrive:, with NO GitProtonBackup payload
    # beside it (module block always skips) and a valid TestDrive helper exe + sidecar (so the
    # checksum block always passes). Each call gets its own scriptDir and LOCALAPPDATA subdir
    # so tests never share mutable state.
    function New-InstallerCopy {
        $id = [guid]::NewGuid().ToString('N').Substring(0, 8)
        $scriptDir = Join-Path $TestDrive "install-$id"
        New-Item -ItemType Directory -Path $scriptDir -Force | Out-Null
        Copy-Item -Path "$PSScriptRoot/../install.ps1" -Destination (Join-Path $scriptDir 'install.ps1') -Force

        $exePath = Join-Path $scriptDir 'git-remote-proton.exe'
        Set-Content -Path $exePath -Value 'fake exe bytes' -NoNewline
        $hash = (Get-FileHash $exePath -Algorithm SHA256).Hash.ToLower()
        Set-Content -Path "$exePath.sha256" -Value "$hash  git-remote-proton.exe" -NoNewline

        $localAppData = Join-Path $TestDrive "localappdata-$id"
        New-Item -ItemType Directory -Path $localAppData -Force | Out-Null

        [pscustomobject]@{
            InstallScript = Join-Path $scriptDir 'install.ps1'
            HelperExe     = $exePath
            LocalAppData  = $localAppData
            HelperDir     = Join-Path $localAppData 'Programs\git-proton-backup'
        }
    }

    # Mock HKCU\Environment key. Speaks exactly the surface install.ps1 uses:
    # GetValue($name, $default, $options), GetValueKind($name), SetValue($name, $value, $kind),
    # Close(). Backed by a hashtable ($state) plus a SetCalls recorder (List), a GetCalls
    # recorder (List, Task 7: records every GetValue $options argument so tests can pin which
    # RegistryValueOptions flags install.ps1 actually passes), and a CloseCalls counter ([ref]).
    #
    # IMPORTANT: these are captured via .GetNewClosure() on each scriptblock, NOT via $script:
    # scope. Verified interactively that Add-Member ScriptMethod blocks referencing $script:
    # resolve against whatever script is EXECUTING at invocation time — since these methods are
    # invoked from deep inside install.ps1 (a different .ps1 file, loaded via `&`), a $script:
    # variable set here would resolve to install.ps1's OWN script scope at call time (empty),
    # not this test file's, and silently read/write $null. GetNewClosure() captures the actual
    # local variables below by reference, which works correctly across that script boundary.
    # The recorders are exposed as NoteProperties on the mock object itself so each test reads
    # $mock.SetCalls / $mock.GetCalls / $mock.CloseCalls.Value rather than any shared scope.
    function New-MockEnvironmentKey {
        param(
            [AllowNull()][string]$InitialPath = $null,
            [Microsoft.Win32.RegistryValueKind]$InitialKind = [Microsoft.Win32.RegistryValueKind]::ExpandString,
            [bool]$PathExists = $true,
            [switch]$ThrowOnSetValue
        )
        $state = @{ Path = $InitialPath; Kind = $InitialKind; PathExists = $PathExists }
        $setCalls = New-Object System.Collections.Generic.List[object]
        $getCalls = New-Object System.Collections.Generic.List[object]
        $closeCalls = [ref]0
        $throwOnSetValue = [bool]$ThrowOnSetValue

        $mock = [pscustomobject]@{}
        $mock | Add-Member -MemberType ScriptMethod -Name GetValue -Value ({
            param($name, $default, $options)
            $getCalls.Add([pscustomobject]@{ Name = $name; Options = $options })
            if ($name -ne 'Path' -or -not $state.PathExists) { return $default }
            return $state.Path
        }.GetNewClosure())
        $mock | Add-Member -MemberType ScriptMethod -Name GetValueKind -Value ({
            param($name)
            return $state.Kind
        }.GetNewClosure())
        $mock | Add-Member -MemberType ScriptMethod -Name SetValue -Value ({
            param($name, $value, $kind)
            $setCalls.Add([pscustomobject]@{ Name = $name; Value = $value; Kind = $kind })
            if ($throwOnSetValue) { throw 'Simulated registry write failure (test double)' }
            $state.Path = $value
            $state.Kind = $kind
            $state.PathExists = $true
        }.GetNewClosure())
        $mock | Add-Member -MemberType ScriptMethod -Name Close -Value ({
            $closeCalls.Value++
        }.GetNewClosure())

        $mock | Add-Member -MemberType NoteProperty -Name SetCalls -Value $setCalls
        $mock | Add-Member -MemberType NoteProperty -Name GetCalls -Value $getCalls
        $mock | Add-Member -MemberType NoteProperty -Name CloseCalls -Value $closeCalls
        return $mock
    }
}

AfterAll {
    # Restore the real LOCALAPPDATA regardless of pass/fail before asserting anything.
    $env:LOCALAPPDATA = $script:originalLocalAppData

    # ---- Guard: real user PATH byte-unchanged (kind included; no kind assumed). ----
    $realPathStateAfter = Get-RealUserPathState
    $realPathStateAfter.Exists | Should -Be $script:realPathStateBefore.Exists -Because 'no test may create/delete the real Path value'
    $realPathStateAfter.Raw    | Should -BeExactly $script:realPathStateBefore.Raw -Because 'no test may mutate the real Path raw value'
    $realPathStateAfter.Kind   | Should -Be $script:realPathStateBefore.Kind -Because 'no test may change the real Path value kind'

    # ---- Guard: real %LOCALAPPDATA%\Programs\git-proton-backup byte-unchanged. ----
    $realHelperDirFingerprintAfter = Get-DirFingerprint -Path $script:realHelperDirPath
    (@($realHelperDirFingerprintAfter) | ConvertTo-Json -Depth 5 -Compress) |
        Should -BeExactly (@($script:realHelperDirFingerprintBefore) | ConvertTo-Json -Depth 5 -Compress) `
        -Because 'no test may touch the real helper install directory'

    # ---- Guard: real Documents PowerShell module dir byte-unchanged. ----
    $realModuleDirFingerprintAfter = Get-DirFingerprint -Path $script:realModuleDirPath
    (@($realModuleDirFingerprintAfter) | ConvertTo-Json -Depth 5 -Compress) |
        Should -BeExactly (@($script:realModuleDirFingerprintBefore) | ConvertTo-Json -Depth 5 -Compress) `
        -Because 'no test may touch the real Documents module directory'
}

Describe 'install.ps1 PATH registry seam (-EnvironmentKey)' {

    It 'RED: preserves REG_EXPAND_SZ kind on append' {
        $ctx = New-InstallerCopy
        $env:LOCALAPPDATA = $ctx.LocalAppData
        $oldRaw = '%USERPROFILE%\bin'
        $mock = New-MockEnvironmentKey -InitialPath $oldRaw -InitialKind ([Microsoft.Win32.RegistryValueKind]::ExpandString)

        & $ctx.InstallScript -SkipPathUpdate:$false -EnvironmentKey $mock -HelperExe $ctx.HelperExe -EffectivePath '' | Out-Null

        $mock.SetCalls.Count    | Should -Be 1
        $mock.SetCalls[0].Name  | Should -Be 'Path'
        $mock.SetCalls[0].Kind  | Should -Be ([Microsoft.Win32.RegistryValueKind]::ExpandString)
        $mock.SetCalls[0].Value | Should -BeExactly "$oldRaw;$($ctx.HelperDir)"
        $mock.CloseCalls.Value  | Should -Be 1
    }

    It 'RED: preserves REG_SZ kind on append' {
        $ctx = New-InstallerCopy
        $env:LOCALAPPDATA = $ctx.LocalAppData
        $oldRaw = 'C:\Existing\Path'
        $mock = New-MockEnvironmentKey -InitialPath $oldRaw -InitialKind ([Microsoft.Win32.RegistryValueKind]::String)

        & $ctx.InstallScript -SkipPathUpdate:$false -EnvironmentKey $mock -HelperExe $ctx.HelperExe -EffectivePath '' | Out-Null

        $mock.SetCalls.Count    | Should -Be 1
        $mock.SetCalls[0].Name  | Should -Be 'Path'
        $mock.SetCalls[0].Kind  | Should -Be ([Microsoft.Win32.RegistryValueKind]::String)
        $mock.SetCalls[0].Value | Should -BeExactly "$oldRaw;$($ctx.HelperDir)"
        $mock.CloseCalls.Value  | Should -Be 1
    }

    It 'RED: no-op when helperDir already present (literal entry and %VAR%-spelled entry)' {
        # Literal entry, already present among others.
        $ctx1 = New-InstallerCopy
        $env:LOCALAPPDATA = $ctx1.LocalAppData
        $mock1 = New-MockEnvironmentKey -InitialPath "C:\Other\Dir;$($ctx1.HelperDir)" -InitialKind ([Microsoft.Win32.RegistryValueKind]::String)

        & $ctx1.InstallScript -SkipPathUpdate:$false -EnvironmentKey $mock1 -HelperExe $ctx1.HelperExe -EffectivePath '' | Out-Null

        $mock1.SetCalls.Count  | Should -Be 0 -Because 'helperDir is already a literal entry'
        $mock1.CloseCalls.Value | Should -Be 1

        # %VAR%-spelled entry that EXPANDS to helperDir (raw stored value never matches
        # literally, only after [Environment]::ExpandEnvironmentVariables).
        $ctx2 = New-InstallerCopy
        $env:LOCALAPPDATA = $ctx2.LocalAppData
        $mock2 = New-MockEnvironmentKey -InitialPath '%LOCALAPPDATA%\Programs\git-proton-backup' -InitialKind ([Microsoft.Win32.RegistryValueKind]::ExpandString)

        & $ctx2.InstallScript -SkipPathUpdate:$false -EnvironmentKey $mock2 -HelperExe $ctx2.HelperExe -EffectivePath '' | Out-Null

        $mock2.SetCalls.Count  | Should -Be 0 -Because 'the %LOCALAPPDATA%-spelled entry expands to helperDir'
        $mock2.CloseCalls.Value | Should -Be 1
    }

    It 'RED: fresh empty PATH writes ExpandString' {
        $ctx = New-InstallerCopy
        $env:LOCALAPPDATA = $ctx.LocalAppData
        $mock = New-MockEnvironmentKey -InitialPath $null -PathExists $false

        & $ctx.InstallScript -SkipPathUpdate:$false -EnvironmentKey $mock -HelperExe $ctx.HelperExe -EffectivePath '' | Out-Null

        $mock.SetCalls.Count    | Should -Be 1
        $mock.SetCalls[0].Kind  | Should -Be ([Microsoft.Win32.RegistryValueKind]::ExpandString)
        $mock.SetCalls[0].Value | Should -BeExactly $ctx.HelperDir
        $mock.CloseCalls.Value  | Should -Be 1
    }

    It 'GUARD: Close is called on every path, including when SetValue throws' {
        $ctx = New-InstallerCopy
        $env:LOCALAPPDATA = $ctx.LocalAppData
        $mock = New-MockEnvironmentKey -InitialPath $null -PathExists $false -ThrowOnSetValue

        { & $ctx.InstallScript -SkipPathUpdate:$false -EnvironmentKey $mock -HelperExe $ctx.HelperExe -EffectivePath '' } |
            Should -Throw '*Simulated registry write failure*'

        $mock.SetCalls.Count   | Should -Be 1 -Because 'SetValue was attempted before it threw'
        $mock.CloseCalls.Value | Should -Be 1 -Because 'finally must run Close() even when the try body throws'
    }

    It 'RED: install.ps1 passes DoNotExpandEnvironmentNames on its PATH read' {
        # Task 7: pins install.ps1's own comment/behaviour (line ~52: "DoNotExpandEnvironmentNames
        # keeps the raw value") against regression. A PathExists=$true mock is required: install.ps1
        # only reaches the flagged GetValue call (line 61-62) after its existence check (line 60,
        # $envKey.GetValue('Path') with no $options) finds a non-null value.
        $ctx = New-InstallerCopy
        $env:LOCALAPPDATA = $ctx.LocalAppData
        $mock = New-MockEnvironmentKey -InitialPath 'C:\Existing\Path' -InitialKind ([Microsoft.Win32.RegistryValueKind]::String)

        & $ctx.InstallScript -SkipPathUpdate:$false -EnvironmentKey $mock -HelperExe $ctx.HelperExe -EffectivePath '' | Out-Null

        $flagged = [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames
        ($mock.GetCalls | Where-Object { $_.Options -eq $flagged }).Count |
            Should -BeGreaterThan 0 -Because 'install.ps1 must read the raw (unexpanded) Path value before comparing entries'
    }

    It 'GUARD: the recorder distinguishes a flagless call' {
        # Pins the spy itself (round-1 Gemini): a call made WITHOUT the flag must not be
        # confused with a flagged one, or the RED test above would be worthless.
        $mock = New-MockEnvironmentKey -InitialPath 'C:\Whatever' -InitialKind ([Microsoft.Win32.RegistryValueKind]::String)

        $mock.GetValue('Path', '', $null) | Out-Null

        $flagged = [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames
        $mock.GetCalls.Count       | Should -Be 1
        $mock.GetCalls[0].Options  | Should -Not -Be $flagged -Because 'a flagless call must be recorded as distinguishable from a flagged one'
    }
}
