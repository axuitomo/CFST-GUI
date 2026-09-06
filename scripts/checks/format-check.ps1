param([switch]$SkipInstall)

. (Join-Path $PSScriptRoot "lib/common.ps1")

Assert-CfstCommand "gofmt"
Assert-CfstCommand "pnpm"

Write-CfstStep "Checking Go formatting"
$excluded = '[\\/](build|frontend[\\/]node_modules|mobile[\\/]android[\\/](\.gradle|app[\\/]build|build))[\\/]'
$goFiles = @(Get-ChildItem $script:CfstRoot -Recurse -File -Filter "*.go" | Where-Object { $_.FullName -notmatch $excluded } | ForEach-Object { $_.FullName })
if ($goFiles.Count -gt 0) {
    $unformatted = @(gofmt -l @goFiles)
    Assert-CfstLastExit "gofmt"
    if ($unformatted.Count -gt 0) {
        throw "Go files require gofmt:`n$($unformatted -join "`n")"
    }
}

Install-CfstFrontend -Skip:$SkipInstall
$frontendFiles = @()
if ($env:CFST_FORMAT_SCOPE -eq "all") {
    $frontendFiles = @(Get-ChildItem $script:CfstFrontend -Recurse -File | Where-Object {
        $_.FullName -notmatch '[\\/](node_modules|dist|wailsjs)[\\/]' -and $_.Extension -in @(".ts", ".vue", ".css", ".json")
    } | ForEach-Object { $_.FullName.Substring($script:CfstFrontend.Length + 1) })
}
elseif ((Get-Command git -ErrorAction SilentlyContinue) -and (Test-Path (Join-Path $script:CfstRoot ".git"))) {
    Push-Location $script:CfstRoot
    try {
        $changed = @(
            git diff --name-only --diff-filter=ACMR HEAD -- frontend
            git ls-files --others --exclude-standard -- frontend
        ) | Sort-Object -Unique
        $frontendFiles = @($changed | Where-Object { $_ -match '^frontend/.*\.(ts|vue|css|json)$' -and $_ -notmatch '^frontend/(node_modules|dist|wailsjs)/' } | ForEach-Object { $_.Substring(9) })
    }
    finally {
        Pop-Location
    }
}
else {
    Write-CfstWarning "Git metadata is unavailable; skipping changed-scope frontend format check. Set CFST_FORMAT_SCOPE=all to check all frontend files."
}

if ($frontendFiles.Count -eq 0) {
    Write-CfstStep "No frontend files selected for Prettier check"
}
else {
    Write-CfstStep "Checking frontend formatting"
    Push-Location $script:CfstFrontend
    try {
        pnpm exec prettier --check @frontendFiles
        Assert-CfstLastExit "prettier --check"
    }
    finally {
        Pop-Location
    }
}

Write-CfstStep "Formatting checks completed"
