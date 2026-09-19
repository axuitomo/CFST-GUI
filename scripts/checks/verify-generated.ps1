param(
    [switch]$SkipInstall,
    [switch]$SkipFrontendBuild
)

. (Join-Path $PSScriptRoot "../lib/common.ps1")

Assert-CfstCommand "git"
Assert-CfstCommand "wails3"
if (-not (Test-Path (Join-Path $script:CfstRoot ".git"))) {
    throw "verify-generated requires Git metadata"
}

# frontend/dist 的构建产物按 .gitignore 约定不入库（只保留 .gitkeep 占位），因此不再对 dist
# 做 git 漂移检测；每次都重新生成 bindings、重新构建前端并断言产物存在，才是「旧前端」的防线。
# 只有静态的 frontend_assets.go（embed 指令）需要保证不会在重新生成时被意外改动。
$trackedPaths = @("frontend_assets.go")

function Get-TrackedState {
    Push-Location $script:CfstRoot
    try {
        # 2>$null：git 的 autocrlf 等提示走 stderr，绝不能混入状态快照导致前后误判为漂移。
        return (@(
            git status --porcelain -- @trackedPaths 2>$null
            git diff --binary -- @trackedPaths 2>$null
            git diff --cached --binary -- @trackedPaths 2>$null
        ) -join "`n")
    }
    finally {
        Pop-Location
    }
}

function Assert-FrontendBuildOutput {
    $index = Join-Path $script:CfstFrontend "dist/index.html"
    if (-not (Test-Path -LiteralPath $index -PathType Leaf)) {
        throw "frontend build produced no dist/index.html; frontend/dist must be built before embedding"
    }
    $chunk = Get-ChildItem -LiteralPath (Join-Path $script:CfstFrontend "dist/assets") -Filter *.js -ErrorAction SilentlyContinue |
        Select-Object -First 1
    if (-not $chunk) {
        throw "frontend build produced no dist/assets/*.js chunk; check the Vite build output"
    }
}

$before = Get-TrackedState
Write-CfstStep "Regenerating Wails frontend bridge"
Push-Location $script:CfstRoot
try {
    wails3 generate bindings
    Assert-CfstLastExit "wails3 generate bindings"
    Assert-CfstWailsBindings
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
        Assert-FrontendBuildOutput
    }
    finally {
        Pop-Location
    }
}

$after = Get-TrackedState
if ($after -ne $before) {
    throw "Tracked embed wiring changed during regeneration"
}
Write-CfstStep "Generated artifacts are present and embed wiring is stable"
