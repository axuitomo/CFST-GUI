param(
    [string]$Base = ""
)

# 秒级报错闭环：按变更区域跑最小检查集（计划 docs/dev/refactor-plan.md §2.3）。
# 前端三件套直调 node 二进制并行执行，规避 pnpm 壳开销；Go 侧 -short。
# 全量门禁仍用 check.ps1 / CI Quality。

. (Join-Path $PSScriptRoot "../lib/common.ps1")

Assert-CfstCommand "go"
Assert-CfstCommand "node"
Assert-CfstCommand "git"

$script:FastTimings = [ordered]@{}
$script:FastFailures = @()
$script:FastTotal = [System.Diagnostics.Stopwatch]::StartNew()
$script:FastTemp = Join-Path ([System.IO.Path]::GetTempPath()) "cfst-fast-$PID"
New-Item -ItemType Directory -Force -Path $script:FastTemp | Out-Null

function Add-FastTiming([string]$Name, [double]$Seconds) {
    $script:FastTimings[$Name] = $Seconds
}

function Add-FastFailure([string]$Name, [string]$Detail) {
    $script:FastFailures += $Name
    Write-Host "`n[fast] FAILED: $Name" -ForegroundColor Red
    if ($Detail) { Write-Host $Detail }
}

function Invoke-FastGo([string]$Name, [string[]]$GoArgs) {
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    Push-Location $script:CfstRoot
    try {
        $output = & go @GoArgs 2>&1 | Out-String
        $code = $LASTEXITCODE
    }
    finally { Pop-Location }
    $sw.Stop()
    Add-FastTiming $Name $sw.Elapsed.TotalSeconds
    if ($code -ne 0) { Add-FastFailure $Name $output }
    elseif ($output -match 'no tests to run') { Add-FastFailure $Name "boundary gate ran nothing: $output" }
    else { Write-Host ("[fast] {0} ok ({1:N1}s)" -f $Name, $sw.Elapsed.TotalSeconds) }
    return $code
}

# 前端三件套并行：Start-Process 直调 node，避免 pwsh job/pnpm 壳开销。
function Start-FastNode([string]$Name, [string[]]$NodeArgs, [string]$WorkDir) {
    $outFile = Join-Path $script:FastTemp "$Name.out"
    $errFile = Join-Path $script:FastTemp "$Name.err"
    $proc = Start-Process -FilePath "node" -ArgumentList $NodeArgs -WorkingDirectory $WorkDir `
        -RedirectStandardOutput $outFile -RedirectStandardError $errFile -NoNewWindow -PassThru
    return [pscustomobject]@{ Name = $Name; Proc = $proc; Out = $outFile; Err = $errFile }
}

function Wait-FastNode([object[]]$Jobs, [string]$TimingName) {
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    foreach ($j in $Jobs) { $j.Proc.WaitForExit() }
    $sw.Stop()
    Add-FastTiming $TimingName $sw.Elapsed.TotalSeconds
    foreach ($j in $Jobs) {
        if ($j.Proc.ExitCode -ne 0) {
            $detail = (Get-Content -LiteralPath $j.Out -Raw -ErrorAction SilentlyContinue) + "`n" + (Get-Content -LiteralPath $j.Err -Raw -ErrorAction SilentlyContinue)
            Add-FastFailure $j.Name $detail
        }
        else { Write-Host ("[fast] {0} ok" -f $j.Name) }
    }
}

function Invoke-FastFrontend {
    if (-not (Test-Path -LiteralPath (Join-Path $script:CfstFrontend "node_modules/vue-tsc"))) {
        throw "frontend/node_modules missing: run pnpm install --frozen-lockfile in frontend first"
    }
    $jobs = @(
        Start-FastNode "oxlint" @("node_modules/oxlint/bin/oxlint", "--deny-warnings", "src", "vite.config.ts", "capacitor.config.ts", "vitest.config.ts", "vitest.browser.config.ts") $script:CfstFrontend
        Start-FastNode "vitest" @("node_modules/vitest/vitest.mjs", "run") $script:CfstFrontend
        Start-FastNode "vue-tsc" @("node_modules/vue-tsc/bin/vue-tsc.js", "--noEmit") $script:CfstFrontend
    )
    return $jobs
}

function Complete-FastFrontend([object[]]$Jobs) {
    Wait-FastNode $Jobs "frontend(lint+test+tsc parallel)"
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    Push-Location $script:CfstRoot
    try { node scripts/checks/vapor-mode.mjs } finally { Pop-Location }
    if ($LASTEXITCODE -ne 0) { Add-FastFailure "vapor-mode" "see output above" }
    & (Join-Path $PSScriptRoot "frontend-boundary.ps1")
    if ($LASTEXITCODE -ne 0) { Add-FastFailure "frontend-boundary" "see output above" }
    $sw.Stop()
    Add-FastTiming "vapor+boundary" $sw.Elapsed.TotalSeconds
}

function Start-FastBoundary {
    $outFile = Join-Path $script:FastTemp "appcore-boundary.out"
    $errFile = Join-Path $script:FastTemp "appcore-boundary.err"
    $proc = Start-Process -FilePath "go" -ArgumentList @(
        "test", "./internal/appcore/", "-count=1", "-run",
        "TestSharedCorePackagesDoNotTransitivelyImportPlatform|TestSharedInternalPackagesDoNotImportPlatformAdapters|TestPlatformInvokeDoesNotReimplementSharedCommands"
    ) -WorkingDirectory $script:CfstRoot -RedirectStandardOutput $outFile -RedirectStandardError $errFile -NoNewWindow -PassThru
    return [pscustomobject]@{ Proc = $proc; Out = $outFile; Err = $errFile }
}

function Complete-FastBoundary([object]$Job) {
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $Job.Proc.WaitForExit()
    $sw.Stop()
    Add-FastTiming "appcore-boundary(parallel)" $sw.Elapsed.TotalSeconds
    $output = (Get-Content -LiteralPath $Job.Out -Raw -ErrorAction SilentlyContinue) + "`n" + (Get-Content -LiteralPath $Job.Err -Raw -ErrorAction SilentlyContinue)
    if ($Job.Proc.ExitCode -ne 0) { Add-FastFailure "appcore-boundary" $output }
    elseif ($output -match 'no tests to run') { Add-FastFailure "appcore-boundary" "boundary gate ran nothing: $output" }
    else { Write-Host "[fast] appcore-boundary ok" }
}

# ---- 变更集 ----
$swChanged = [System.Diagnostics.Stopwatch]::StartNew()
Push-Location $script:CfstRoot
try {
    $changed = @()
    if ($Base) { $changed += @(git diff --name-only --diff-filter=ACMR "$Base...HEAD") }
    $changed += @(git diff --name-only --diff-filter=ACMR HEAD)
    $changed += @(git ls-files --others --exclude-standard)
    $changed = @($changed | Where-Object { $_ } | Sort-Object -Unique)
}
finally { Pop-Location }
$swChanged.Stop()
Add-FastTiming "changed-set" $swChanged.Elapsed.TotalSeconds

$goChanged = @($changed | Where-Object { $_ -match '\.go$' -or $_ -eq 'go.mod' -or $_ -eq 'go.sum' })
$frontendChanged = @($changed | Where-Object { $_ -like 'frontend/*' }).Count -gt 0
$scriptChanged = @($changed | Where-Object { $_ -like 'scripts/*' -and $_ -match '\.sh$' })

$frontendJobs = $null
$boundaryJob = $null
if ($changed.Count -eq 0) {
    Write-CfstStep "No changed files: minimal full sweep (go -short all + frontend trio)"
    $frontendJobs = Invoke-FastFrontend
    $pkgs = Get-CfstGoPackages
    Invoke-FastGo "go-test-short(all)" (@("test", "-short", "-count=1") + $pkgs) | Out-Null
    Complete-FastFrontend $frontendJobs
}
else {
    Write-Host ("[fast] changed files: {0}" -f $changed.Count)

    # 前端三件套先启动（非阻塞），与 Go 阶段完全并行。
    if ($frontendChanged) { $frontendJobs = Invoke-FastFrontend }

    if ($goChanged.Count -gt 0) {
        $pkgs = @()
        if (($changed -contains 'go.mod') -or ($changed -contains 'go.sum')) {
            $pkgs = Get-CfstGoPackages
        }
        else {
            # 直接按目录映射包路径（省去 go list 全量扫描的 1-2s）；testdata 不是包，跳过。
            $dirs = @($goChanged | ForEach-Object { (Split-Path $_ -Parent) -replace '\\', '/' } | Sort-Object -Unique)
            foreach ($d in $dirs) {
                if (($d -split '/') -contains 'testdata') { continue }
                $pkgs += if ($d -eq '' -or $d -eq '.') { '.' } else { "./$d" }
            }
            $pkgs = @($pkgs | Sort-Object -Unique)
            if ($pkgs.Count -eq 0) { Write-CfstWarning "changed Go files map to no testable package; skipping go test" }
        }
        if ($pkgs.Count -gt 0) {
            $appcoreHit = @($pkgs | Where-Object { $_ -match 'internal/appcore/?$' }).Count -gt 0
            if (-not $appcoreHit) { $boundaryJob = Start-FastBoundary }
            Invoke-FastGo "go-test-short" (@("test", "-short", "-count=1") + $pkgs) | Out-Null
            if ($boundaryJob) { Complete-FastBoundary $boundaryJob }
        }
    }

    if ($frontendJobs) { Complete-FastFrontend $frontendJobs }

    if ($scriptChanged.Count -gt 0) {
        $bashCmd = Get-Command bash -ErrorAction SilentlyContinue
        $bashExe = if ($bashCmd) { $bashCmd.Source } else { $null }
        if (-not $bashExe) {
            $gitBash = Join-Path $env:ProgramFiles "Git\bin\bash.exe"
            if (Test-Path -LiteralPath $gitBash) { $bashExe = $gitBash }
        }
        if ($bashExe) {
            $sw = [System.Diagnostics.Stopwatch]::StartNew()
            foreach ($s in $scriptChanged) { & $bashExe -n (Join-Path $script:CfstRoot $s); if ($LASTEXITCODE -ne 0) { Add-FastFailure "bash -n $s" "" } }
            $sw.Stop(); Add-FastTiming "bash -n" $sw.Elapsed.TotalSeconds
        }
        else { Write-CfstWarning "bash not found; shell syntax check skipped (CI covers it)" }
    }
}

# ---- no-string-code 守卫（refactor-plan §3.6，词法级全仓扫描） ----
$swGuard = [System.Diagnostics.Stopwatch]::StartNew()
Push-Location $script:CfstRoot
try { node scripts/checks/no-string-code.mjs } finally { Pop-Location }
if ($LASTEXITCODE -ne 0) { Add-FastFailure "no-string-code" "see output above" }
$swGuard.Stop()
Add-FastTiming "no-string-code" $swGuard.Elapsed.TotalSeconds

# ---- 拆分红线警告（不阻断） ----
foreach ($f in $changed) {
    if ($f -match '(_test\.go|\.test\.ts|\.spec\.ts|\.browser\.test\.ts)$') { continue }
    if ($f -notmatch '\.(go|ts|vue|css|kt)$') { continue }
    $p = Join-Path $script:CfstRoot $f
    if (Test-Path -LiteralPath $p) {
        $n = @(Get-Content -LiteralPath $p).Count
        if ($n -gt 400) { Write-CfstWarning ("{0}: {1} lines (>400 拆分红线)" -f $f, $n) }
    }
}

# ---- 汇总 ----
$script:FastTotal.Stop()
Write-Host "`n==> fast check timings"
foreach ($k in $script:FastTimings.Keys) { Write-Host ("{0,-34} {1,6:N1}s" -f $k, $script:FastTimings[$k]) }
Write-Host ("{0,-34} {1,6:N1}s" -f "TOTAL", $script:FastTotal.Elapsed.TotalSeconds)
Remove-Item -LiteralPath $script:FastTemp -Recurse -Force -ErrorAction SilentlyContinue
if ($script:FastFailures.Count -gt 0) {
    Write-Host ("`n[fast] FAILED steps: {0}" -f ($script:FastFailures -join ", ")) -ForegroundColor Red
    exit 1
}
Write-Host "[fast] all checks passed" -ForegroundColor Green
exit 0