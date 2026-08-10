param([switch]$SkipInstall)

. (Join-Path $PSScriptRoot "lib/common.ps1")

Assert-CfstCommand "go"
Assert-CfstCommand "pnpm"
Push-Location $script:CfstRoot
try {
    Write-CfstStep "Verifying Go module checksums"
    go mod verify
    Assert-CfstLastExit "go mod verify"
    Write-CfstStep "Listing available Go module updates"
    go list -m -u all
    Assert-CfstLastExit "go list -m -u all"
}
finally {
    Pop-Location
}

Install-CfstFrontend -Skip:$SkipInstall
Push-Location $script:CfstFrontend
try {
    Write-CfstStep "Running pnpm audit"
    $auditLevel = if ($env:CFST_PNPM_AUDIT_LEVEL) { $env:CFST_PNPM_AUDIT_LEVEL } else { "moderate" }
    pnpm audit "--audit-level=$auditLevel"
    Assert-CfstLastExit "pnpm audit"
    Write-CfstStep "Listing available pnpm package updates"
    pnpm outdated
}
finally {
    Pop-Location
}

Write-CfstStep "Dependency audit completed"
