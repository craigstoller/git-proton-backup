<#
.SYNOPSIS
Trash everything under /my-files/_cas-probe. Safe to run at any time.

.DESCRIPTION
The probes clean up after themselves unless run with -NoCleanup. This is the manual escape
hatch for an interrupted probe, and a way to confirm the probe area is empty before or after a
session.

It can only touch /my-files/_cas-probe, enforced by probe-lib.ps1's path assertion. There is no
parameter to point it somewhere else - that is deliberate, per the guardrails recorded in
probe-lib.ps1.

Note this TRASHES rather than permanently deletes, so the bytes remain in your Proton trash
until you empty it by hand. That is intentional: an automated permanent delete against a real
account is exactly the capability these guardrails exist to withhold.
#>
[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'probe-lib.ps1')

'=== probe cleanup ==='
Remove-ProbeTree
