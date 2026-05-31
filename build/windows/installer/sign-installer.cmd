@echo off
setlocal

if "%~1"=="" (
  echo Missing target file for SignTool. 1>&2
  exit /b 2
)

set "CERT=%CFST_WINDOWS_SIGNING_CERT_NATIVE%"
if "%CERT%"=="" set "CERT=%CFST_WINDOWS_SIGNING_CERT%"
if "%CERT%"=="" exit /b 0
set "SIGN_TOOL=%CFST_WINDOWS_SIGNING_TOOL%"
if "%SIGN_TOOL%"=="" set "SIGN_TOOL=SignTool.exe"

if not exist "%CERT%" (
  echo Windows signing certificate not found: %CERT% 1>&2
  exit /b 1
)

if "%CFST_WINDOWS_SIGNING_PASSWORD%"=="" (
  "%SIGN_TOOL%" sign /fd SHA256 /f "%CERT%" "%~1"
) else (
  "%SIGN_TOOL%" sign /fd SHA256 /f "%CERT%" /p "%CFST_WINDOWS_SIGNING_PASSWORD%" "%~1"
)
