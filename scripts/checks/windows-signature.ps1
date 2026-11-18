# Copyright (C) 2026 axuitomo
# SPDX-License-Identifier: GPL-3.0-only

# Shared Authenticode verification for release assets and local installer tests.
# A signature must always cover the current file contents, keep its timestamp,
# and use the expected signing identity. The certificate chain may terminate in
# an untrusted root: self-signed and private-PKI certificates are accepted by
# default, while CFST_WINDOWS_ALLOW_SELF_SIGNED=0 restores the trusted-chain rule.

Set-StrictMode -Version Latest

$script:CfstAllowedChainFlags = [int](
    [System.Security.Cryptography.X509Certificates.X509ChainStatusFlags]::UntrustedRoot -bor
    [System.Security.Cryptography.X509Certificates.X509ChainStatusFlags]::PartialChain
)

function Test-CfstUntrustedCertificateChain {
    [CmdletBinding()]
    [OutputType([bool])]
    param(
        [Parameter(Mandatory = $true)]
        [System.Security.Cryptography.X509Certificates.X509Certificate2]$Certificate
    )

    $chain = [System.Security.Cryptography.X509Certificates.X509Chain]::new()
    try {
        $chain.ChainPolicy.RevocationMode = [System.Security.Cryptography.X509Certificates.X509RevocationMode]::NoCheck
        $null = $chain.Build($Certificate)
        $statuses = @($chain.ChainStatus)
        if ($statuses.Count -eq 0) {
            return $false
        }
        foreach ($status in $statuses) {
            if (([int]$status.Status -band (-bnot $script:CfstAllowedChainFlags)) -ne 0) {
                return $false
            }
        }
        return $true
    }
    finally {
        $chain.Dispose()
    }
}

function Get-CfstWindowsSignature {
    <#
    .SYNOPSIS
    Verifies an Authenticode signature and returns its trust mode.
    .DESCRIPTION
    Throws unless the file is signed, the signature hash matches, the optional
    timestamp exists, and the signer matches ExpectedThumbprint when provided.
    Untrusted self-signed/private-PKI chains are accepted only when
    AllowSelfSigned is true and the chain has no other error.
    #>
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path,
        [string]$ExpectedThumbprint = '',
        [switch]$RequireTimestamp,
        [bool]$AllowSelfSigned = $true
    )

    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "Signed file not found: $Path"
    }
    $resolvedPath = (Resolve-Path -LiteralPath $Path -ErrorAction Stop).Path
    $signature = Get-AuthenticodeSignature -LiteralPath $resolvedPath
    if ($null -eq $signature.SignerCertificate) {
        throw "File is not code-signed: $resolvedPath (status: $($signature.Status))"
    }
    if ($signature.Status -eq [System.Management.Automation.SignatureStatus]::HashMismatch) {
        throw "Signature hash does not match file contents: $resolvedPath"
    }
    if ($ExpectedThumbprint -and ($signature.SignerCertificate.Thumbprint -ne $ExpectedThumbprint)) {
        throw "Unexpected signing identity for ${resolvedPath}: expected $ExpectedThumbprint, got $($signature.SignerCertificate.Thumbprint)"
    }
    if ($RequireTimestamp -and ($null -eq $signature.TimeStamperCertificate)) {
        throw "Signature has no timestamp: $resolvedPath"
    }

    $trust = 'self-signed'
    if ($signature.Status -eq [System.Management.Automation.SignatureStatus]::Valid) {
        $trust = 'trusted'
    }
    elseif (-not $AllowSelfSigned) {
        throw "Signature is not trusted and self-signed signing is disabled: $resolvedPath (status: $($signature.Status); $($signature.StatusMessage))"
    }
    elseif (-not (Test-CfstUntrustedCertificateChain -Certificate $signature.SignerCertificate)) {
        throw "Signature certificate chain is invalid: $resolvedPath (status: $($signature.Status); $($signature.StatusMessage))"
    }

    return [pscustomobject]@{
        Path = $resolvedPath
        Status = $signature.Status
        Trust = $trust
        Signature = $signature
        SignerThumbprint = $signature.SignerCertificate.Thumbprint
        SignerSubject = $signature.SignerCertificate.Subject
        HasTimestamp = $null -ne $signature.TimeStamperCertificate
    }
}
