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

$golangci = Get-Command golangci-lint -ErrorAction SilentlyContinue
if ($golangci) {
    Write-CfstStep "Running golangci-lint (errcheck/staticcheck/ineffassign/unused/revive/goimports)"
    Push-Location $script:CfstRoot
    try {
        & $golangci.Source run
        Assert-CfstLastExit "golangci-lint"
    }
    finally {
        Pop-Location
    }
}
elseif ($env:CFST_REQUIRE_GOLANGCI -eq "1") {
    throw "golangci-lint is required because CFST_REQUIRE_GOLANGCI=1"
}
else {
    Write-CfstWarning "golangci-lint not found; skipping Go lint"
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

$actionlint = Get-Command actionlint -ErrorAction SilentlyContinue
if ($actionlint) {
    Write-CfstStep "Running actionlint (GitHub Actions workflows)"
    Push-Location $script:CfstRoot
    try {
        & $actionlint.Source -color
        Assert-CfstLastExit "actionlint"
    }
    finally {
        Pop-Location
    }
}
elseif ($env:CFST_REQUIRE_ACTIONLINT -eq "1") {
    throw "actionlint is required because CFST_REQUIRE_ACTIONLINT=1"
}
else {
    Write-CfstWarning "actionlint not found; skipping workflow lint"
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

Write-CfstStep "Running frontend stylelint"
Push-Location $script:CfstFrontend
try {
    pnpm exec stylelint "src/**/*.css"
    Assert-CfstLastExit "stylelint"
}
finally {
    Pop-Location
}

Write-CfstStep "Running markdownlint"
Push-Location $script:CfstRoot
try {
    pnpm exec markdownlint-cli2
    Assert-CfstLastExit "markdownlint"
}
finally {
    Pop-Location
}

Write-CfstStep "Running root ESLint (Playwright config and E2E tests)"
Push-Location $script:CfstRoot
try {
    pnpm exec eslint playwright.config.ts "tests/**/*.ts"
    Assert-CfstLastExit "root eslint"
}
finally {
    Pop-Location
}
Write-CfstStep "Running Android ktlint (main source set) and detekt"
Push-Location $script:CfstRoot
try {
    $androidDir = Join-Path $script:CfstRoot "mobile\android"
    if (Test-Path (Join-Path $androidDir "gradlew.bat")) {
        Push-Location $androidDir
        try {
            & ".\gradlew.bat" ktlintMainSourceSetCheck detekt --console=plain
            Assert-CfstLastExit "android ktlint + detekt"
        }
        finally {
            Pop-Location
        }
    }
    else {
        Write-CfstWarn "Android Gradle wrapper not found; skipping Android lint"
    }
}
finally {
    Pop-Location
}
Write-CfstStep "Lint checks completed"
