param([switch]$SkipInstall)

. (Join-Path $PSScriptRoot "lib/common.ps1")

Assert-CfstCommand "go"
Assert-CfstCommand "pnpm"

Write-CfstStep "Running go vet"
$goPackages = @(Get-CfstGoPackages)
Push-Location $script:CfstRoot
try {
    go vet @goPackages
    Assert-CfstLastExit "go vet"
}
finally {
    Pop-Location
}

$shellcheck = Get-Command shellcheck -ErrorAction SilentlyContinue
if ($shellcheck) {
    Write-CfstStep "Running shellcheck"
    $shellFiles = @(Get-ChildItem (Join-Path $script:CfstRoot "scripts") -Recurse -File -Filter "*.sh" | Sort-Object FullName | ForEach-Object { $_.FullName })
    if ($shellFiles.Count -gt 0) {
        & $shellcheck.Source @shellFiles
        Assert-CfstLastExit "shellcheck"
    }
}
elseif ($env:CFST_REQUIRE_SHELLCHECK -eq "1") {
    throw "shellcheck is required because CFST_REQUIRE_SHELLCHECK=1"
}
else {
    Write-CfstWarning "shellcheck not found; skipping shell lint"
}

Install-CfstFrontend -Skip:$SkipInstall
Write-CfstStep "Running frontend ESLint"
Push-Location $script:CfstFrontend
try {
    pnpm run lint
    Assert-CfstLastExit "pnpm lint"
}
finally {
    Pop-Location
}

Write-CfstStep "Lint checks completed"
