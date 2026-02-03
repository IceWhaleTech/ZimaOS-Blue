; ZimaOS Echo - Beautiful Custom UI Installer
; Full custom window with background image

!include "MUI2.nsh"
!include "nsDialogs.nsh"
!include "LogicLib.nsh"
!include "FileFunc.nsh"
!include "x64.nsh"

Unicode true
ManifestDPIAware true

!define PRODUCT_NAME "ZimaOS Echo"
!define PRODUCT_VERSION "0.10.17"
!define PRODUCT_PUBLISHER "ZimaOS Team"
!define PRODUCT_UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}"

Name "${PRODUCT_NAME}"
OutFile "ZimaOS-Echo_${PRODUCT_VERSION}_x64-setup.exe"
InstallDir "$LOCALAPPDATA\${PRODUCT_NAME}"
RequestExecutionLevel user
SetCompressor /SOLID lzma

!define MUI_ICON "icons\icon.ico"
!define MUI_UNICON "icons\icon.ico"

; Custom variables
Var Dialog
Var BgImage
Var BgImageHandle
Var InstallBtn
Var CancelBtn
Var ProgressBar
Var StatusText
Var Installing

; Custom page
Page custom CustomPage CustomPageLeave
Page instfiles "" "" InstFilesShow

!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "SimpChinese"

Function .onInit
  StrCpy $Installing 0
  InitPluginsDir

  ; Extract background
  SetOutPath $PLUGINSDIR
  File "assets\background_with_logo.png"
  File "assets\logo.png"
FunctionEnd

Function .onGUIInit
  ; Set window size to match background (scaled down)
  System::Call "user32::SetWindowPos(p $HWNDPARENT, p 0, i 200, i 100, i 600, i 400, i 0)"
FunctionEnd

Function CustomPage
  nsDialogs::Create 1018
  Pop $Dialog
  ${If} $Dialog == error
    Abort
  ${EndIf}

  ; Set background color
  SetCtlColors $Dialog "" "1a1a2e"

  ; Install button - centered, modern style
  ${NSD_CreateButton} 220 280 160 45 "Install"
  Pop $InstallBtn
  ${NSD_OnClick} $InstallBtn OnInstallClick

  ; Cancel button
  ${NSD_CreateButton} 400 280 80 45 "Cancel"
  Pop $CancelBtn
  ${NSD_OnClick} $CancelBtn OnCancelClick

  ; Progress bar (hidden)
  ${NSD_CreateProgressBar} 100 290 400 20 ""
  Pop $ProgressBar
  ShowWindow $ProgressBar ${SW_HIDE}

  ; Status text (hidden)
  ${NSD_CreateLabel} 100 320 400 20 ""
  Pop $StatusText
  SetCtlColors $StatusText "FFFFFF" "1a1a2e"
  ShowWindow $StatusText ${SW_HIDE}

  nsDialogs::Show
FunctionEnd

Function OnInstallClick
  ShowWindow $InstallBtn ${SW_HIDE}
  ShowWindow $CancelBtn ${SW_HIDE}
  ShowWindow $ProgressBar ${SW_SHOW}
  ShowWindow $StatusText ${SW_SHOW}

  ${NSD_SetText} $StatusText "Installing..."
  SendMessage $ProgressBar ${PBM_SETRANGE32} 0 100
  SendMessage $ProgressBar ${PBM_SETPOS} 10 0

  StrCpy $Installing 1
  SendMessage $HWNDPARENT ${WM_COMMAND} 1 0
FunctionEnd

Function OnCancelClick
  Quit
FunctionEnd

Function CustomPageLeave
  ${If} $Installing == 0
    Abort
  ${EndIf}
FunctionEnd

Function InstFilesShow
FunctionEnd

Section "Install"
  SetOutPath $INSTDIR

  ; Copy files
  File "release\zimaos-echo.exe"
  File "release\WebView2Loader.dll"
  File "release\echo-server.exe"

  ; Shortcuts
  CreateDirectory "$SMPROGRAMS\${PRODUCT_NAME}"
  CreateShortcut "$SMPROGRAMS\${PRODUCT_NAME}\${PRODUCT_NAME}.lnk" "$INSTDIR\zimaos-echo.exe"
  CreateShortcut "$DESKTOP\${PRODUCT_NAME}.lnk" "$INSTDIR\zimaos-echo.exe"

  ; Uninstaller
  WriteUninstaller "$INSTDIR\uninstall.exe"

  ; Registry
  WriteRegStr HKCU "${PRODUCT_UNINST_KEY}" "DisplayName" "${PRODUCT_NAME}"
  WriteRegStr HKCU "${PRODUCT_UNINST_KEY}" "UninstallString" "$INSTDIR\uninstall.exe"
  WriteRegStr HKCU "${PRODUCT_UNINST_KEY}" "DisplayIcon" "$INSTDIR\zimaos-echo.exe"
  WriteRegStr HKCU "${PRODUCT_UNINST_KEY}" "DisplayVersion" "${PRODUCT_VERSION}"
  WriteRegStr HKCU "${PRODUCT_UNINST_KEY}" "Publisher" "${PRODUCT_PUBLISHER}"
SectionEnd

Section "Uninstall"
  RMDir /r "$INSTDIR"
  Delete "$DESKTOP\${PRODUCT_NAME}.lnk"
  RMDir /r "$SMPROGRAMS\${PRODUCT_NAME}"
  DeleteRegKey HKCU "${PRODUCT_UNINST_KEY}"
SectionEnd

Function .onInstSuccess
  Exec "$INSTDIR\zimaos-echo.exe"
FunctionEnd
