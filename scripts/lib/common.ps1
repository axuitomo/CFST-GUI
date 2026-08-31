Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$script:CfstRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$script:CfstFrontend = Join-Path $script:CfstRoot "frontend"

function Write-CfstStep([string]$Message) {
    Write-Host "`n==> $Message"
}

function Write-CfstWarning([string]$Message) {
    Write-Warning $Message
}

function Assert-CfstCommand([string]$Name) {
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "required command not found: $Name"
    }
}

function Assert-CfstLastExit([string]$CommandName) {
    if ($LASTEXITCODE -ne 0) {
        throw "$CommandName failed with exit code $LASTEXITCODE"
    }
}

function Get-CfstGoPackages {
    Push-Location $script:CfstRoot
    try {
        $packages = @(go list ./... | Where-Object { $_ -notmatch '/frontend/node_modules(?:/|$)' })
        Assert-CfstLastExit "go list"
        return $packages
    }
    finally {
        Pop-Location
    }
}

function Install-CfstFrontend([switch]$Skip) {
    if ($Skip -or $env:CFST_SKIP_PNPM_INSTALL -eq "1") {
        Write-CfstStep "Skipping frontend pnpm install"
        return
    }
    Write-CfstStep "Installing frontend dependencies with pnpm"
    Push-Location $script:CfstFrontend
    try {
        pnpm install --frozen-lockfile
        Assert-CfstLastExit "pnpm install"
    }
    finally {
        Pop-Location
    }
}

function Invoke-CfstWailsGenerate([switch]$Skip) {
    if ($Skip -or $env:CFST_SKIP_WAILS_GENERATE -eq "1") {
        Write-CfstStep "Skipping Wails module generation"
        return
    }
    if (Get-Command wails -ErrorAction SilentlyContinue) {
        Write-CfstStep "Generating Wails frontend bridge"
        Push-Location $script:CfstRoot
        try {
            wails3 generate bindings
            Assert-CfstLastExit "wails3 generate bindings"
        }
        finally {
            Pop-Location
        }
        return
    }
    if (Test-Path (Join-Path $script:CfstFrontend "wailsjs")) {
        Write-CfstWarning "frontend/bindings missing: wails3 command not found; using existing frontend/bindings"
        return
    }
    throw "frontend/bindings missing: wails3 command not found and frontend/bindings is missing"
}
