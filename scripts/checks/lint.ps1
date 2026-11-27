param([switch]$SkipInstall)

. (Join-Path $PSScriptRoot "../lib/common.ps1")

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
Write-CfstStep "Running frontend Oxlint"
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

Write-CfstStep "Running root Oxlint (Playwright config and E2E tests)"
Push-Location $script:CfstRoot
try {
    pnpm exec oxlint --deny-warnings playwright.config.ts tests
    Assert-CfstLastExit "root oxlint"
}
finally {
    Pop-Location
}
Write-CfstStep "Running Android ktlint (main source set) and detekt"
Push-Location $script:CfstRoot
try {
    $androidDir = Join-Path $script:CfstRoot "apps\android"
    $gradleBat = Join-Path $androidDir "gradlew.bat"
    $androidAar = Join-Path $androidDir "app\libs\mobileapi.aar"
    $androidSdkCandidates = @(
        [Environment]::GetEnvironmentVariable("ANDROID_HOME"),
        [Environment]::GetEnvironmentVariable("ANDROID_SDK_ROOT"),
        (Join-Path $HOME "Library\Android\sdk"),
        $(if ($env:LOCALAPPDATA) { Join-Path $env:LOCALAPPDATA "Android\Sdk" }),
        (Join-Path $HOME ".cache\cfst-gui\android-toolchain\android-sdk")
    ) | Where-Object { $_ -and (Test-Path -LiteralPath $_ -PathType Container) }
    $androidSdkHome = $androidSdkCandidates | Select-Object -First 1
    $requireAndroidLint = [Environment]::GetEnvironmentVariable("CFST_REQUIRE_ANDROID_LINT") -eq "1"

    if (-not (Test-Path -LiteralPath $gradleBat -PathType Leaf)) {
        Write-CfstWarning "Android Gradle wrapper not found; skipping Android lint"
    }
    elseif (-not $androidSdkHome) {
        if ($requireAndroidLint) {
            throw "Android lint is required because CFST_REQUIRE_ANDROID_LINT=1, but the Android SDK was not found"
        }
        Write-CfstWarning "Android SDK not found; skipping Android lint"
    }
    elseif (-not (Test-Path -LiteralPath $androidAar -PathType Leaf)) {
        if ($requireAndroidLint) {
            throw "Android lint is required because CFST_REQUIRE_ANDROID_LINT=1, but apps/android/app/libs/mobileapi.aar is missing (run scripts/build/build-android-mobile.sh)"
        }
        Write-CfstWarning "mobileapi.aar not found; skipping Android lint (run scripts/build/build-android-mobile.sh first)"
    }
    else {
        $pluginsDir = Join-Path $androidDir "capacitor-cordova-android-plugins"
        if (-not (Test-Path -LiteralPath $pluginsDir -PathType Container)) {
            Write-CfstStep "Generating Capacitor Android plugins"
            Invoke-CfstWailsGenerate
            Push-Location $script:CfstFrontend
            try {
                pnpm run build
                Assert-CfstLastExit "pnpm run build"
                pnpm exec cap sync android
                Assert-CfstLastExit "pnpm exec cap sync android"
            }
            finally {
                Pop-Location
            }
            node (Join-Path $script:CfstRoot "scripts\build\patch-android-gradle-warnings.mjs")
            Assert-CfstLastExit "patch-android-gradle-warnings"
        }
        Push-Location $androidDir
        try {
            & ".\gradlew.bat" ktlintMainSourceSetCheck detektDebug detektDebugUnitTest --console=plain
            Assert-CfstLastExit "android ktlint + detekt"
        }
        finally {
            Pop-Location
        }
    }
}
finally {
    Pop-Location
}
Write-CfstStep "Lint checks completed"
