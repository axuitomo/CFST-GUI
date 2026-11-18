# Copyright (C) 2026 axuitomo
# SPDX-License-Identifier: GPL-3.0-only
param(
    [Parameter(Mandatory = $true)]
    [string]$ExpectedVersion
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if (-not $IsWindows -or $env:GITHUB_ACTIONS -ne 'true' -or $env:RUNNER_ENVIRONMENT -ne 'github-hosted') {
    throw 'Run signed installation verification only on a disposable GitHub-hosted Windows runner.'
}
$principal = [Security.Principal.WindowsPrincipal]::new([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw 'Signed installation verification requires an elevated runner.'
}

$root = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$installer = Join-Path $root 'build/artifacts/release/desktop/cfst-gui-windows-amd64.exe'
$portable = Join-Path $root 'build/artifacts/release/desktop/cfst-gui-windows-amd64-portable.exe'
$registryPath = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\axuitomoCFST-GUI'
$shortcutPaths = @('Programs', 'CommonPrograms', 'DesktopDirectory', 'CommonDesktopDirectory') | ForEach-Object {
    Join-Path ([Environment]::GetFolderPath($_)) 'CFST-GUI.lnk'
}
if ((Test-Path $registryPath) -or @($shortcutPaths | Where-Object { Test-Path $_ }).Count -ne 0) {
    throw 'Refusing to replace an existing CFST-GUI installation or shortcut.'
}
$installDir = Join-Path $env:RUNNER_TEMP ('CFST signed install ' + [guid]::NewGuid().ToString('N'))
$uninstaller = Join-Path $installDir 'uninstall.exe'
$legalNames = @('LICENSE', 'THIRD-PARTY-NOTICES.md')
. (Join-Path $PSScriptRoot 'windows-signature.ps1')
$allowSelfSigned = $env:CFST_WINDOWS_ALLOW_SELF_SIGNED -ne '0'
$installerSignature = Get-CfstWindowsSignature -Path $installer -RequireTimestamp -AllowSelfSigned $allowSelfSigned
$signerThumbprint = $installerSignature.SignerThumbprint

function Assert-ReleaseSignature([string]$Path) {
    return Get-CfstWindowsSignature -Path $Path -RequireTimestamp -AllowSelfSigned $allowSelfSigned `
        -ExpectedThumbprint $signerThumbprint
}

function Invoke-Installer([string]$Path, [string[]]$Arguments) {
    $process = Start-Process -FilePath $Path -ArgumentList $Arguments -PassThru
    if (-not $process.WaitForExit(120000)) {
        $process.Kill($true)
        throw 'The installer did not finish within 120 seconds.'
    }
    $process.Refresh()
    if ($process.ExitCode -ne 0) {
        throw "Installer exited with code $($process.ExitCode); code 66 means WebView2 Runtime is missing."
    }
}

function Assert-Installed {
    foreach ($name in $legalNames) {
        if ((Get-FileHash -LiteralPath (Join-Path $installDir $name)).Hash -ne
            (Get-FileHash -LiteralPath (Join-Path $root $name)).Hash) {
            throw "Installed legal text differs from the canonical file: $name"
        }
    }
    foreach ($pair in @(@('cfst-gui.exe', $portable), @('uninstall.exe', $installer))) {
        $installedFile = Join-Path $installDir $pair[0]
        if ((Get-FileHash -LiteralPath $installedFile).Hash -ne (Get-FileHash -LiteralPath $pair[1]).Hash) {
            throw "Installed executable differs from the signed release asset: $($pair[0])"
        }
        $null = Assert-ReleaseSignature $installedFile
    }
    $registration = Get-ItemProperty -LiteralPath $registryPath
    if ($registration.InstallLocation -ne $installDir -or $registration.DisplayVersion -ne $ExpectedVersion -or
        $registration.QuietUninstallString -ne ('"' + $uninstaller + '" --uninstall --silent')) {
        throw 'The installed location, version, or quiet uninstall registration is incorrect.'
    }
    $startMenu = Join-Path ([Environment]::GetFolderPath('CommonPrograms')) 'CFST-GUI.lnk'
    if (-not (Test-Path -LiteralPath $startMenu)) { throw 'The Start Menu shortcut is missing.' }
    $shell = New-Object -ComObject WScript.Shell
    $shortcut = $null
    try {
        $shortcut = $shell.CreateShortcut($startMenu)
        if ($shortcut.TargetPath -ne (Join-Path $installDir 'cfst-gui.exe')) {
            throw 'The Start Menu shortcut targets the wrong executable.'
        }
    } finally {
        if ($null -ne $shortcut) { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($shortcut) }
        [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($shell)
    }
    foreach ($folder in @('DesktopDirectory', 'CommonDesktopDirectory')) {
        if (Test-Path -LiteralPath (Join-Path ([Environment]::GetFolderPath($folder)) 'CFST-GUI.lnk')) {
            throw 'The --no-desktop option was ignored.'
        }
    }
}

$null = Assert-ReleaseSignature $installer
$null = Assert-ReleaseSignature $portable
$installArguments = @('/S', '--no-desktop', '--no-launch', ('--dir="' + $installDir + '"'))
try {
    Invoke-Installer $installer $installArguments
    Assert-Installed
    $userFile = Join-Path $installDir 'user-data.json'
    $userData = '{"preserve":true}'
    [IO.File]::WriteAllText($userFile, $userData)
    foreach ($name in $legalNames) { [IO.File]::WriteAllText((Join-Path $installDir $name), 'stale notice') }
    Invoke-Installer $installer $installArguments
    Assert-Installed
    if ([IO.File]::ReadAllText($userFile) -ne $userData) { throw 'Reinstallation changed user data.' }
    Invoke-Installer $uninstaller @('/S', '--uninstall')
    for ($attempt = 0; $attempt -lt 40 -and (Test-Path -LiteralPath $uninstaller); $attempt++) {
        Start-Sleep -Milliseconds 250
    }
    foreach ($name in @('cfst-gui.exe', 'uninstall.exe') + $legalNames) {
        if (Test-Path -LiteralPath (Join-Path $installDir $name)) { throw "Uninstall left an application file: $name" }
    }
    if ((Test-Path $registryPath) -or @($shortcutPaths | Where-Object { Test-Path $_ }).Count -ne 0) {
        throw 'Uninstall left application registration or shortcuts.'
    }
    if ([IO.File]::ReadAllText($userFile) -ne $userData) { throw 'Uninstall removed or changed user data.' }
    Remove-Item -LiteralPath $userFile
    Remove-Item -LiteralPath $installDir
    Write-Output "Verified signed install, canonical legal files, reinstallation, and uninstall with user data preserved. Signature trust: $($installerSignature.Trust)."
} finally {
    if ((Test-Path -LiteralPath $registryPath) -and (Test-Path -LiteralPath $uninstaller)) {
        $registration = Get-ItemProperty -LiteralPath $registryPath
        if ($registration.InstallLocation -eq $installDir) {
            Invoke-Installer $uninstaller @('/S', '--uninstall')
        }
    }
}
