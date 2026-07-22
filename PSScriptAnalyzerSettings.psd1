@{
    # PSScriptAnalyzer settings — CI fails on ANY finding not excluded here, so every
    # exclusion below is a considered deviation with its reason, not a mute button.
    # Rule of thumb for future changes: fix first; exclude only when the rule fights a
    # deliberate design choice of this module.

    ExcludeRules = @(
        # The hook flow writes user-facing lines that git relays as "remote: ..." during
        # `git push proton`, and the public commands print short one-line UX ("Wired. ...").
        # Write-Host IS the interface here — this module has no logging framework by design.
        'PSAvoidUsingWriteHost',

        # Seam-heavy codebase: test seams are scriptblock parameters with contractual
        # signatures (e.g. a default `{ param($p) $false }` SyncCheck whose stub body ignores
        # $p). The parameter defines the seam's shape; the rule reads that as waste. Also
        # false-positives on parameters used only inside closures.
        'PSReviewUnusedParameter',

        # v1 API deliberately has no -WhatIf/-Confirm plumbing. Destructive operations are
        # guarded structurally instead (ownership-equality checks before any mirror delete;
        # uninstall never touches bundles). Adding SupportsShouldProcess is a real API
        # decision for a future major version, not a lint fix.
        'PSUseShouldProcessForStateChangingFunctions',

        # Internal functions return ad-hoc [pscustomobject] result shapes documented at the
        # call sites and pinned by the test suite. [OutputType] annotations would add
        # maintenance surface with no consumer benefit pre-1.0.
        'PSUseOutputTypeCorrectly',

        # The module requires PowerShell 7.4+ (see README), where BOM-less UTF-8 is the
        # default and read correctly. The rule protects Windows PowerShell 5.1, which this
        # module does not support.
        'PSUseBOMForUnicodeEncodedFile',

        # Both hits are `2>$null` stderr redirections inside if-conditions (e.g.
        # `if (... (git config gpb.workrepo 2>$null))`) — a known false-positive pattern for
        # this rule; no `>` is ever used as a comparison in this codebase.
        'PSPossibleIncorrectUsageOfRedirectionOperator',

        # One hit: the private (non-exported) helper Get-BundleObjects, named for parity with
        # its twin in the private codebase this module was extracted from. All ten exported
        # commands use singular nouns.
        'PSUseSingularNouns'
    )
}
