# Rename matrix (private → public port)

Apply mechanically. Anything not listed keeps its name and behavior.

## Git config keys        ## Env vars                    ## State files
pos.workrepo  → gpb.workrepo      POS_LOCK_PATH → GPB_LOCK_PATH    projects-backup.lock → gpb.lock
pos.hookscript → (DELETED — shim imports the module by name)       push-pending\*.json  → unchanged name, new home
pos.verifyseconds → gpb.verifyseconds   POS_HOOK_DISABLED → GPB_HOOK_DISABLED   .<name>.lastdigest → unchanged
                                  POS_MIRROR_ROOT → (DELETED — mirrors always under the state dir)
                                  (NEW) GPB_CONFIG_DIR → overrides the state root for tests

## Functions (ported, renamed)
Invoke-LocalRepoBackup   → Invoke-RepoBundleBackup   (params -Tier/-WhatIf DELETED; auto-commit block DELETED;
                                                      -RetentionKeep/-RetentionCheckpoints ADDED, threaded to prune)
Get-BackupLockPath       → Get-GpbLockPath           (GPB_LOCK_PATH override; default <state>\gpb.lock)
Wait-BackupLock          → Wait-GpbLock              (behavior identical incl. non-contention warning)
Get-RepoMarkerSlug       → Get-GpbSlug               (same derivation; single slug fn for markers/mirrors/bundles)
Get-PushMarkerDir        → Get-GpbMarkerDir          (no -OutDir param; derives from state root)
Install-ProtonPushMirror → Install-GpbMirror         (internal; deltas in Task 5)
Remove-ProtonPushMirror  → Remove-GpbMirror          (internal; ownership-equality delta in Task 5)
Test-ProtonPushMirror    → Test-GpbMirror            (internal; unchanged checks minus hookscript)
Get-ProtonMirrorPath     → Get-GpbMirrorPath         (slug-keyed: <state>\mirrors\<slug>.git; no ProjectsRoot)
Invoke-PushBackupFlow    → (internal, same name)     (deltas in Task 6)
[wrapper script]         → Invoke-ProtonBackupHook   (exported function, not a script; Task 6)

## Functions (ported, same name): Get-RepoRefDigest, Get-BundleObjects, Get-BundleToPrune,
Get-RepoPreflight (identity check DELETED, detached-HEAD rejection DELETED), Get-RepoStatusEntry,
ConvertFrom-PorcelainLine, Test-RepoHazard, Get-CloudBundlePath (containment check ADDED),
Get-CloudFileSyncState, ConvertFrom-CfPlaceholderState, Initialize-CfType,
Confirm-BundleUploaded, Test-ProtonCliReady (Get-Command resolution ADDED),
Write-PushPendingMarker (atomic write ADDED), Remove-PushPendingMarker.

## Functions NOT ported (private-system only): Get-RepoPolicy, Get-RepoRemote, Get-RepoRoute,
Test-RemoteAllowed, Test-IsHub, Get-AutoCommitDecision, Get-ProjectRepo, Invoke-BackupRun,
Write-CoverageReport, Send-CoverageNotification, Get-CoverageAudit, Get-InboxBacklog,
Get-OutboxStaging, Get-IgnoredFileAudit, Get-RegisteredOriginal, Test-HostedRepoCoverage,
Get-AttentionSignature, Test-ShouldNotify, Invoke-PushMarkerSweep (superseded by Verify).

## Messages
'daily sweep will cover this' / 'the sweep will verify' / 'daily sweep will verify'
  → 'run Invoke-ProtonBackupVerify (or the scheduled task) to confirm'
'backup deferred — another backup run is active; the sweep will verify'
  → 'backup deferred — another backup operation is active; run Invoke-ProtonBackupVerify (or the scheduled task) to confirm'
All other push-output phrases (incl. 'confirmed on Proton', 'staged; in-sync per Cloud Files
(CLI verification unavailable)', 'staged, not yet confirmed') are IDENTICAL.
