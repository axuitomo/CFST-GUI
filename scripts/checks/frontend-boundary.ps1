param()

# Frontend platform-convergence boundary (Windows counterpart of
# frontend-boundary.sh). Self-contained so it runs standalone, from check.ps1,
# or from any working directory.
#
# Platform switching is only allowed under packages/frontend/src/lib/adapters/, per
# docs/dev/architecture-constraints.md:
#
#   1) platform runtime references: @wailsio / window.wails / _wails /
#      wailsjs/ / @capacitor/ / Capacitor
#   2) platform channel references: '/api/...' literals or templates, Wails host address
#      (wails.localhost, protocol === "wails:")
#
# Comments outside packages/frontend/src/lib/adapters/ (and unit tests) must avoid these tokens too.
# Views, components, composables, and shared bridge core must route through lib/bridge.ts.

$ErrorActionPreference = 'Stop'
$cfstRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path

Write-Host "==> Checking frontend platform-convergence boundary"

$runtimePattern = '@wailsio|window\.wails|window\[[''"]wails[''"]\]|_wails|wailsjs/|@capacitor/|Capacitor'
$channelPattern = '[''"`]/api/|wails\.localhost|protocol\s*===\s*[''"]wails:'

$violations = @(Get-ChildItem (Join-Path $cfstRoot 'packages\frontend\src') -Recurse -File -Include *.ts,*.vue,*.js |
    Where-Object {
        $_.FullName -notlike '*\src\lib\adapters\*' -and
        $_.Name -notmatch '\.(?:test|browser\.test)\.ts$'
    } |
    Select-String -Pattern $runtimePattern, $channelPattern)

if ($violations.Count -gt 0) {
    $details = ($violations | ForEach-Object {
        $rel = $_.Path.Substring($cfstRoot.Length + 1) -replace '\\', '/'
        "$rel`:$($_.LineNumber): $($_.Line.Trim())"
    }) -join "`n"
    Write-Host "Frontend boundary check failed: platform switching must stay inside packages/frontend/src/lib/adapters/." -ForegroundColor Red
    Write-Host "  - runtime references: @wailsio / window.wails / _wails / wailsjs/ / @capacitor / Capacitor" -ForegroundColor Red
    Write-Host "  - channel references: /api/... literals or templates / wails.localhost / protocol === ""wails:""" -ForegroundColor Red
    Write-Host $details -ForegroundColor Red
    Write-Host "Route platform capabilities through lib/bridge.ts and lib/adapters/. See docs/dev/architecture-constraints.md." -ForegroundColor Red
    exit 1
}

node (Join-Path $PSScriptRoot "frontend-architecture.mjs")
if ($LASTEXITCODE -ne 0) { exit 1 }

Write-Host "==> Frontend boundary check completed"
