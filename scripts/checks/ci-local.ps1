param([switch]$SkipAudit)

. (Join-Path $PSScriptRoot "lib/common.ps1")

Install-CfstFrontend
$previousSkipInstall = $env:CFST_SKIP_PNPM_INSTALL
$env:CFST_SKIP_PNPM_INSTALL = "1"
try {
    & (Join-Path $PSScriptRoot "format-check.ps1") -SkipInstall
    & (Join-Path $PSScriptRoot "lint.ps1") -SkipInstall
    & (Join-Path $PSScriptRoot "check.ps1") -SkipInstall
    & (Join-Path $PSScriptRoot "verify-generated.ps1") -SkipInstall
    if (-not $SkipAudit -and $env:CFST_SKIP_AUDIT -ne "1") {
        & (Join-Path $PSScriptRoot "audit.ps1") -SkipInstall
    }
}
finally {
    $env:CFST_SKIP_PNPM_INSTALL = $previousSkipInstall
}

Write-CfstStep "Local CI completed"
