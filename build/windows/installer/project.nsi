Unicode true

####
## Please note: Template replacements don't work in this file. They are provided with default defines like
## mentioned underneath.
## If the keyword is not defined, "wails_tools.nsh" will populate them with the values from ProjectInfo.
## If they are defined here, "wails_tools.nsh" will not touch them. This allows to use this project.nsi manually
## from outside of Wails for debugging and development of the installer.
##
## For development first make a wails nsis build to populate the "wails_tools.nsh":
## > wails build --target windows/amd64 --nsis
## Then you can call makensis on this file with specifying the path to your binary:
## For a AMD64 only installer:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app.exe
## For a ARM64 only installer:
## > makensis -DARG_WAILS_ARM64_BINARY=..\..\bin\app.exe
## For a installer with both architectures:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app-amd64.exe -DARG_WAILS_ARM64_BINARY=..\..\bin\app-arm64.exe
####
## The following information is taken from the ProjectInfo file, but they can be overwritten here.
####
## !define INFO_PROJECTNAME    "MyProject" # Default "{{.Name}}"
## !define INFO_COMPANYNAME    "MyCompany" # Default "{{.Info.CompanyName}}"
## !define INFO_PRODUCTNAME    "MyProduct" # Default "{{.Info.ProductName}}"
## !define INFO_PRODUCTVERSION "1.0.0"     # Default "{{.Info.ProductVersion}}"
## !define INFO_COPYRIGHT      "Copyright" # Default "{{.Info.Copyright}}"
###
## !define PRODUCT_EXECUTABLE  "Application.exe"      # Default "${INFO_PROJECTNAME}.exe"
## !define UNINST_KEY_NAME     "UninstKeyInRegistry"  # Default "${INFO_COMPANYNAME}${INFO_PRODUCTNAME}"
####
## !define REQUEST_EXECUTION_LEVEL "admin"            # Default "admin"  see also https://nsis.sourceforge.io/Docs/Chapter4.html
####
## Include the wails tools
####
!include "wails_tools.nsh"

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

ManifestDPIAware true

!include "MUI.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_FINISHPAGE_NOAUTOCLOSE
!define MUI_ABORTWARNING
!define WEBVIEW2_RUNTIME_GUID "{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"
!define WEBVIEW2_RUNTIME_URL "https://developer.microsoft.com/microsoft-edge/webview2/consumer/"
!define WEBVIEW2_RUNTIME_MISSING_MESSAGE "CFST-GUI 需要 Microsoft Edge WebView2 Runtime.$\r$\n$\r$\n当前系统未检测到该运行时。请先安装 WebView2 Runtime，然后重新运行 CFST-GUI 安装程序。"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

!uninstfinalize 'cmd /D /C sign-installer.cmd "%1"'
!finalize 'cmd /D /C sign-installer.cmd "%1"'

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\release\desktop\cfst-gui-windows-amd64.exe"
InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
ShowInstDetails show

Function .onInit
   !insertmacro wails.checkArchitecture
   Call CheckWebView2Runtime
FunctionEnd

Function CheckWebView2Runtime
    SetRegView 64

    StrCpy $0 ""
    ClearErrors
    ReadRegStr $0 HKLM "SOFTWARE\Microsoft\EdgeUpdate\Clients\${WEBVIEW2_RUNTIME_GUID}" "pv"
    StrCmp $0 "" webview2_check_hklm_wow6432
    StrCmp $0 "0.0.0.0" webview2_check_hklm_wow6432 webview2_found

webview2_check_hklm_wow6432:
    StrCpy $0 ""
    ClearErrors
    ReadRegStr $0 HKLM "SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\${WEBVIEW2_RUNTIME_GUID}" "pv"
    StrCmp $0 "" webview2_check_hkcu
    StrCmp $0 "0.0.0.0" webview2_check_hkcu webview2_found

webview2_check_hkcu:
    StrCpy $0 ""
    ClearErrors
    ReadRegStr $0 HKCU "SOFTWARE\Microsoft\EdgeUpdate\Clients\${WEBVIEW2_RUNTIME_GUID}" "pv"
    StrCmp $0 "" webview2_missing
    StrCmp $0 "0.0.0.0" webview2_missing webview2_found

webview2_missing:
    IfSilent webview2_missing_silent webview2_missing_interactive

webview2_missing_silent:
    SetErrorLevel 66
    Quit

webview2_missing_interactive:
    MessageBox MB_ICONEXCLAMATION|MB_OK "${WEBVIEW2_RUNTIME_MISSING_MESSAGE}" IDOK webview2_open_download

webview2_open_download:
    ExecShell "open" "${WEBVIEW2_RUNTIME_URL}"
    SetErrorLevel 66
    Quit

webview2_found:
    Return
FunctionEnd

Section
    !insertmacro wails.setShellContext

    SetOutPath $INSTDIR

    !insertmacro wails.files
    File "/oname=icon.ico" "..\icon.ico"

    CreateShortCut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}" "" "$INSTDIR\icon.ico" 0
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}" "" "$INSTDIR\icon.ico" 0

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    !insertmacro wails.writeUninstaller
    WriteRegStr HKLM "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\icon.ico"
SectionEnd

Section "uninstall"
    !insertmacro wails.setShellContext

    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}"

    Delete "$INSTDIR\icon.ico"
    RMDir /r $INSTDIR

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller
SectionEnd
