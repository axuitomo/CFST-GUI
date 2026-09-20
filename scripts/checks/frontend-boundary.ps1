param()

# Frontend platform-convergence boundary (Windows counterpart of
# frontend-boundary.sh). Self-contained so it runs standalone, from check.ps1,
# or from any working directory.
#
# Platform switching is only allowed under frontend/src/lib/, per
# docs/dev/architecture-constraints.md:
#
#   1) platform runtime references: window.wails / _wails (the host runtime marker) /
#      wailsjs/ / @capacitor/ / Capacitor
#   2) platform channel references: '/api/...' literals or templates, Wails host address
#      (wails.localhost, protocol === "wails:")
#
# Comments in views/components/composables must avoid these names too (the grep does not
# distinguish comments from code); refer to lib/wailsRuntime.ts helpers instead.

#
# Views and components must not pick the channel themselves: a desktop page sent to
# /api/command/{command} gets a 404 (the bottom-right "WebUI 请求失败 (404)" toast) and a
# WebUI page sent to /wails/runtime gets a 405. Route the decision through
# lib/bridge.ts resolveBridgeMode.

$ErrorActionPreference = 'Stop'
$cfstRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path

Write-Host "==> Checking frontend platform-convergence boundary"

$runtimePattern = 'window\.wails|window\[[''"]wails[''"]\]|_wails|wailsjs/|@capacitor/|Capacitor'
$channelPattern = '[''"`]/api/|wails\.localhost|protocol\s*===\s*[''"]wails:'

$violations = @(Get-ChildItem (Join-Path $cfstRoot 'frontend\src') -Recurse -File -Include *.ts,*.vue,*.js |
    Where-Object { $_.FullName -notlike '*\src\lib\*' } |
    Select-String -Pattern $runtimePattern, $channelPattern)

if ($violations.Count -gt 0) {
    $details = ($violations | ForEach-Object {
        $rel = $_.Path.Substring($cfstRoot.Length + 1) -replace '\\', '/'
        "$rel`:$($_.LineNumber): $($_.Line.Trim())"
    }) -join "`n"
    Write-Host "Frontend boundary check failed: platform switching must stay inside frontend/src/lib/." -ForegroundColor Red
    Write-Host "  - runtime references: window.wails / _wails / wailsjs/ / @capacitor / Capacitor" -ForegroundColor Red
    Write-Host "  - channel references: /api/... literals or templates / wails.localhost / protocol === ""wails:""" -ForegroundColor Red
    Write-Host $details -ForegroundColor Red
    Write-Host "Route channel decisions through lib/bridge.ts resolveBridgeMode. See docs/dev/architecture-constraints.md." -ForegroundColor Red
    exit 1
}

Write-Host "==> Frontend boundary check completed"
