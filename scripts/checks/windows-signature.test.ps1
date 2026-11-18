# Copyright (C) 2026 axuitomo
# SPDX-License-Identifier: GPL-3.0-only

# Regression test for the shared Authenticode helper. It signs a throwaway
# binary with a temporary self-signed certificate, verifies that self-signed
# chains are accepted by default, rejected in strict mode, and that any
# post-signing modification invalidates the result. No timestamp is requested
# so the test does not depend on a timestamp server.

Set-StrictMode -Version Latest

. (Join-Path $PSScriptRoot 'windows-signature.ps1')

$signToolCommand = Get-Command SignTool.exe -ErrorAction SilentlyContinue
$signToolPath = if ($null -ne $signToolCommand) { $signToolCommand.Source } else { $null }
if (-not $signToolPath) {
    $sdkRoot = Join-Path ${env:ProgramFiles(x86)} 'Windows Kits\10\bin'
    if (Test-Path -LiteralPath $sdkRoot) {
        $candidate = Get-ChildItem $sdkRoot -Recurse -Filter SignTool.exe |
            Where-Object { $_.FullName -match '\\x64\\SignTool.exe$' } |
            Sort-Object FullName -Descending |
            Select-Object -First 1
        if ($null -ne $candidate) {
            $signToolPath = $candidate.FullName
        }
    }
}
if (-not $signToolPath) {
    throw 'SignTool.exe not found on PATH or in the Windows SDK; install the Windows SDK first.'
}
$go = Get-Command go -ErrorAction SilentlyContinue
if (-not $go) {
    throw 'go not found on PATH; the signature test builds a throwaway binary.'
}

$testDirectory = Join-Path ([IO.Path]::GetTempPath()) ('cfst-signature-test-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $testDirectory | Out-Null
$certificate = $null
try {
    $sourcePath = Join-Path $testDirectory 'main.go'
    [IO.File]::WriteAllText($sourcePath, "package main`nfunc main() {}`n", (New-Object Text.UTF8Encoding $false))
    $exePath = Join-Path $testDirectory 'unsigned.exe'
    Push-Location $testDirectory
    try {
        & $go.Source build -o $exePath main.go
        if ($LASTEXITCODE -ne 0) {
            throw 'go build failed while preparing the signature test binary.'
        }
    }
    finally {
        Pop-Location
    }

    $certificate = New-SelfSignedCertificate -Type CodeSigningCert -Subject 'CN=CFST-GUI Signature Test' `
        -CertStoreLocation 'Cert:\CurrentUser\My' -KeyExportPolicy Exportable -NotAfter (Get-Date).AddDays(1)
    $pfxPath = Join-Path $testDirectory 'test.pfx'
    $passwordText = 'CfstSignatureTest!2026'
    $password = ConvertTo-SecureString -String $passwordText -AsPlainText -Force
    Export-PfxCertificate -Cert $certificate.PSPath -FilePath $pfxPath -Password $password | Out-Null

    & $signToolPath sign /fd SHA256 /f $pfxPath /p $passwordText $exePath | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw 'SignTool failed to sign the test binary.'
    }

    $signature = Get-CfstWindowsSignature -Path $exePath -AllowSelfSigned $true
    if ($signature.Trust -ne 'self-signed') {
        throw "Expected a self-signed signature, got $($signature.Trust)."
    }

    $strictRejected = $false
    try {
        $null = Get-CfstWindowsSignature -Path $exePath -AllowSelfSigned $false
    }
    catch {
        if ($_.Exception.Message -notlike '*not trusted*') {
            throw
        }
        $strictRejected = $true
    }
    if (-not $strictRejected) {
        throw 'Strict verification accepted a self-signed signature.'
    }

    $stream = [IO.File]::Open($exePath, [IO.FileMode]::Append, [IO.FileAccess]::Write)
    try {
        $stream.Write([byte[]](1, 2, 3), 0, 3)
    }
    finally {
        $stream.Dispose()
    }
    $tamperedRejected = $false
    try {
        $null = Get-CfstWindowsSignature -Path $exePath -AllowSelfSigned $true
    }
    catch {
        if ($_.Exception.Message -notmatch 'not code-signed|hash does not match') {
            throw
        }
        $tamperedRejected = $true
    }
    if (-not $tamperedRejected) {
        throw 'Verification accepted a file modified after signing.'
    }

    Write-Output 'Verified self-signed acceptance, strict rejection, and tamper rejection.'
}
finally {
    if ($null -ne $certificate) {
        Remove-Item -LiteralPath $certificate.PSPath -Force -ErrorAction SilentlyContinue
    }
    Remove-Item -LiteralPath $testDirectory -Recurse -Force -ErrorAction SilentlyContinue
}
