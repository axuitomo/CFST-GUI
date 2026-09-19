param()

# Frontend platform-convergence boundary (Windows counterpart of
# frontend-boundary.sh). Self-contained so it runs standalone, from check.ps1,
# or from any working directory.
#
# Platform runtime references (window.wails / wailsjs / @capacitor / Capacitor)
# are allowed only under frontend/src/lib/, per docs/dev/architecture-constraints.md.

$ErrorActionPreference = 'Stop'
$cfstRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path

Write-Host "==> Checking frontend platform-convergence boundary"

$pattern = 'window\.wails|window\[[''"]wails[''"]\]|wailsjs/|@capacitor/|Capacitor'
$violations = @(Get-ChildItem (Join-Path $cfstRoot 'frontend\src') -Recurse -File -Include *.ts,*.vue,*.js |
    Where-Object { $_.FullName -notlike '*\src\lib\*' } |
    Select-String -Pattern $pattern)

if ($violations.Count -gt 0) {
    $details = ($violations | ForEach-Object {
        $rel = $_.Path.Substring($cfstRoot.Length + 1) -replace '\\', '/'
        "$rel`:$($_.LineNumber): $($_.Line.Trim())"
    }) -join "`n"
    Write-Host "Frontend boundary check failed: platform runtime references (window.wails / wailsjs / @capacitor / Capacitor) are only allowed under frontend/src/lib." -ForegroundColor Red
    Write-Host $details -ForegroundColor Red
    Write-Host "See docs/dev/architecture-constraints.md." -ForegroundColor Red
    exit 1
}

Write-Host "==> Frontend boundary check completed"
