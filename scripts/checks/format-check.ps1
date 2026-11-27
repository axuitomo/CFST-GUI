param([switch]$SkipInstall)

. (Join-Path $PSScriptRoot "../lib/common.ps1")

Assert-CfstCommand "gofmt"
Assert-CfstCommand "pnpm"
Assert-CfstCommand "git"

Write-CfstStep "Checking Go formatting"
$excluded = '[\\/](node_modules|\.pnpm-store|\.codegraph|\.git|\.tmp|\.agents|build|apps[\\/]android[\\/](\.gradle|app[\\/]build|build))[\\/]'
$goFiles = @(Get-ChildItem $script:CfstRoot -Recurse -File -Filter "*.go" | Where-Object { $_.FullName -notmatch $excluded } | ForEach-Object { $_.FullName })
if ($goFiles.Count -gt 0) {
    $unformatted = @(gofmt -l @goFiles)
    Assert-CfstLastExit "gofmt"
    if ($unformatted.Count -gt 0) {
        throw "Go files require gofmt:`n$($unformatted -join "`n")"
    }
}

Install-CfstFrontend -Skip:$SkipInstall
# 全量扫描：检查范围随改动集变化，会让格式漂移只在某个工作流里才暴露——发布就因此红过一次。
# 用 git 列文件而不是遍历目录，让 .gitignore（node_modules、dist、bindings、.vite 缓存）自动生效。
$frontendFiles = @(
    git -C $script:CfstRoot ls-files --cached --others --exclude-standard -- packages/frontend |
        ForEach-Object { $_ -replace '^packages/frontend/', '' } |
        Where-Object { $_ -match '\.(ts|vue|css|json)$' }
)

if ($frontendFiles.Count -eq 0) {
    Write-CfstStep "No frontend files selected for oxfmt check"
}
else {
    Write-CfstStep "Checking frontend formatting"
    Push-Location $script:CfstFrontend
    try {
        pnpm exec oxfmt --check @frontendFiles
        Assert-CfstLastExit "oxfmt --check"
    }
    finally {
        Pop-Location
    }
}

Write-CfstStep "Formatting checks completed"
