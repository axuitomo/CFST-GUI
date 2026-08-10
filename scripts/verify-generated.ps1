param(
    [switch]$SkipInstall,
    [switch]$SkipFrontendBuild
)

. (Join-Path $PSScriptRoot "lib/common.ps1")

Assert-CfstCommand "git"
Assert-CfstCommand "wails"
if (-not (Test-Path (Join-Path $script:CfstRoot ".git"))) {
    throw "verify-generated requires Git metadata"
}

function Get-GeneratedState {
    Push-Location $script:CfstRoot
    try {
        return (@(
            git status --porcelain -- frontend/dist frontend_assets.go
            git diff --binary -- frontend/dist frontend_assets.go
            git diff --cached --binary -- frontend/dist frontend_assets.go
        ) -join "`n")
    }
    finally {
        Pop-Location
    }
}

$before = Get-GeneratedState
Write-CfstStep "Regenerating Wails frontend bridge"
Push-Location $script:CfstRoot
try {
    wails generate module
    Assert-CfstLastExit "wails generate module"
}
finally {
    Pop-Location
}

if (-not $SkipFrontendBuild -and $env:CFST_SKIP_FRONTEND_BUILD -ne "1") {
    Install-CfstFrontend -Skip:$SkipInstall
    Write-CfstStep "Rebuilding embedded frontend assets"
    Push-Location $script:CfstFrontend
    try {
        pnpm run build
        Assert-CfstLastExit "pnpm build"
    }
    finally {
        Pop-Location
    }
}

$after = Get-GeneratedState
if ($after -ne $before) {
    throw "Generated artifacts changed during regeneration"
}
Write-CfstStep "Generated artifacts are stable"
