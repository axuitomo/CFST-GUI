param(
    [switch]$SkipInstall,
    [switch]$SkipWailsGenerate
)

. (Join-Path $PSScriptRoot "lib/common.ps1")

Assert-CfstCommand "go"
Assert-CfstCommand "pnpm"
Invoke-CfstWailsGenerate -Skip:$SkipWailsGenerate

Write-CfstStep "Running Go tests"
$goPackages = @(Get-CfstGoPackages)
Push-Location $script:CfstRoot
try {
    go test @goPackages
    Assert-CfstLastExit "go test"
    # WebUI 构建标签不在默认构建中（发布流程用它构建 Linux WebUI 二进制），这里单独跑一次
    # internal/app 的标签测试，避免 webui 路径只在发布时才被编译。
    Write-CfstStep "Running Go tests (webui build tag)"
    go test -tags webui ./internal/app/
    Assert-CfstLastExit "go test -tags webui"
}
finally {
    Pop-Location
}

Install-CfstFrontend -Skip:$SkipInstall
Push-Location $script:CfstFrontend
try {
    Write-CfstStep "Running frontend unit tests"
    pnpm run test
    Assert-CfstLastExit "pnpm test"
    Write-CfstStep "Running frontend typecheck"
    pnpm run typecheck
    Assert-CfstLastExit "pnpm typecheck"
    Write-CfstStep "Running frontend production build"
    pnpm run build
    Assert-CfstLastExit "pnpm build"
}
finally {
    Pop-Location
}

Write-CfstStep "Project checks completed"
