; ZimaOS Echo - Modern One-Click Installer
; Custom UI without nsis_tauri_utils.dll

!include "MUI2.nsh"
!include "nsDialogs.nsh"
!include "LogicLib.nsh"
!include "FileFunc.nsh"
!include "x64.nsh"
!include "WinVer.nsh"

Unicode true
ManifestDPIAware true

; Product Info
!define PRODUCT_NAME "ZimaOS Echo"
!define PRODUCT_VERSION "0.10.17"
!define PRODUCT_PUBLISHER "ZimaOS Team"
!define PRODUCT_DIR_REGKEY "Software\Microsoft\Windows\CurrentVersion\App Paths\zimaos-echo.exe"
!define PRODUCT_UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}"

; Installer Settings
Name "${PRODUCT_NAME} ${PRODUCT_VERSION}"
OutFile "ZimaOS-Echo_${PRODUCT_VERSION}_x64-setup.exe"
InstallDir "$LOCALAPPDATA\${PRODUCT_NAME}"
RequestExecutionLevel user
SetCompressor /SOLID lzma

; Custom UI Variables
Var Dialog
Var LogoBitmap
Var LogoHandle
Var TitleLabel
Var DescLabel
Var VersionLabel
Var InstallBtn
Var ProgressBar
Var StatusLabel
Var CloseBtn
Var IsInstalling

; MUI Settings
!define MUI_ABORTWARNING
!define MUI_ICON "icons\icon.ico"
!define MUI_UNICON "icons\icon.ico"

; Skip default pages, use custom
Page custom CustomMainPage CustomMainPageLeave
Page instfiles "" "" InstFilesLeave

; Uninstaller pages
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

; Languages
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "SimpChinese"
!insertmacro MUI_LANGUAGE "TradChinese"
!insertmacro MUI_LANGUAGE "Japanese"
!insertmacro MUI_LANGUAGE "Korean"
!insertmacro MUI_LANGUAGE "German"
!insertmacro MUI_LANGUAGE "French"
!insertmacro MUI_LANGUAGE "Spanish"
!insertmacro MUI_LANGUAGE "Portuguese"
!insertmacro MUI_LANGUAGE "Russian"
!insertmacro MUI_LANGUAGE "Italian"
!insertmacro MUI_LANGUAGE "Dutch"
!insertmacro MUI_LANGUAGE "Polish"
!insertmacro MUI_LANGUAGE "Ukrainian"
!insertmacro MUI_LANGUAGE "Turkish"
!insertmacro MUI_LANGUAGE "Arabic"
!insertmacro MUI_LANGUAGE "Thai"
!insertmacro MUI_LANGUAGE "Vietnamese"
!insertmacro MUI_LANGUAGE "Indonesian"

; Language Strings
LangString TITLE_TEXT ${LANG_ENGLISH} "ZimaOS Echo"
LangString TITLE_TEXT ${LANG_SIMPCHINESE} "ZimaOS Echo"
LangString DESC_TEXT ${LANG_ENGLISH} "AI Gateway for Local LLM Integration"
LangString DESC_TEXT ${LANG_SIMPCHINESE} "本地 LLM 集成的 AI 网关"
LangString INSTALL_BTN ${LANG_ENGLISH} "Install"
LangString INSTALL_BTN ${LANG_SIMPCHINESE} "安装"
LangString INSTALLING ${LANG_ENGLISH} "Installing..."
LangString INSTALLING ${LANG_SIMPCHINESE} "正在安装..."
LangString COMPLETE ${LANG_ENGLISH} "Installation Complete!"
LangString COMPLETE ${LANG_SIMPCHINESE} "安装完成！"
LangString LAUNCH_BTN ${LANG_ENGLISH} "Launch"
LangString LAUNCH_BTN ${LANG_SIMPCHINESE} "启动"
LangString CLOSE_BTN ${LANG_ENGLISH} "Close"
LangString CLOSE_BTN ${LANG_SIMPCHINESE} "关闭"

Function .onInit
  StrCpy $IsInstalling 0
  !insertmacro MUI_LANGDLL_DISPLAY

  ; Extract logo
  InitPluginsDir
  SetOutPath $PLUGINSDIR
  File "assets\logo.png"
FunctionEnd

; Custom Main Page
Function CustomMainPage
  nsDialogs::Create 1018
  Pop $Dialog
  ${If} $Dialog == error
    Abort
  ${EndIf}

  ; White background
  SetCtlColors $Dialog "" "FFFFFF"

  ; Logo (centered)
  ${NSD_CreateBitmap} 175 30 150 150 ""
  Pop $LogoBitmap

  ; Title
  ${NSD_CreateLabel} 0 200 500 30 "$(TITLE_TEXT)"
  Pop $TitleLabel
  SetCtlColors $TitleLabel "1E293B" "FFFFFF"
  ${NSD_AddStyle} $TitleLabel ${SS_CENTER}
  CreateFont $0 "Segoe UI" 24 700
  SendMessage $TitleLabel ${WM_SETFONT} $0 1

  ; Description
  ${NSD_CreateLabel} 0 240 500 20 "$(DESC_TEXT)"
  Pop $DescLabel
  SetCtlColors $DescLabel "64748B" "FFFFFF"
  ${NSD_AddStyle} $DescLabel ${SS_CENTER}
  CreateFont $0 "Segoe UI" 11 400
  SendMessage $DescLabel ${WM_SETFONT} $0 1

  ; Version
  ${NSD_CreateLabel} 0 265 500 15 "v${PRODUCT_VERSION}"
  Pop $VersionLabel
  SetCtlColors $VersionLabel "94A3B8" "FFFFFF"
  ${NSD_AddStyle} $VersionLabel ${SS_CENTER}
  CreateFont $0 "Segoe UI" 9 400
  SendMessage $VersionLabel ${WM_SETFONT} $0 1

  ; Install Button
  ${NSD_CreateButton} 175 310 150 45 "$(INSTALL_BTN)"
  Pop $InstallBtn
  ${NSD_OnClick} $InstallBtn OnInstallClick
  CreateFont $0 "Segoe UI" 12 600
  SendMessage $InstallBtn ${WM_SETFONT} $0 1

  ; Progress Bar (hidden initially)
  ${NSD_CreateProgressBar} 50 320 400 25 ""
  Pop $ProgressBar
  ShowWindow $ProgressBar ${SW_HIDE}

  ; Status Label (hidden initially)
  ${NSD_CreateLabel} 0 350 500 20 ""
  Pop $StatusLabel
  SetCtlColors $StatusLabel "64748B" "FFFFFF"
  ${NSD_AddStyle} $StatusLabel ${SS_CENTER}
  ShowWindow $StatusLabel ${SW_HIDE}

  nsDialogs::Show
FunctionEnd

Function OnInstallClick
  ; Hide install button, show progress
  ShowWindow $InstallBtn ${SW_HIDE}
  ShowWindow $ProgressBar ${SW_SHOW}
  ShowWindow $StatusLabel ${SW_SHOW}

  ${NSD_SetText} $StatusLabel "$(INSTALLING)"
  SendMessage $ProgressBar ${PBM_SETRANGE32} 0 100
  SendMessage $ProgressBar ${PBM_SETPOS} 0 0

  StrCpy $IsInstalling 1

  ; Trigger next page (instfiles)
  SendMessage $HWNDPARENT ${WM_COMMAND} 1 0
FunctionEnd

Function CustomMainPageLeave
  ${If} $IsInstalling == 0
    Abort
  ${EndIf}
FunctionEnd

Function InstFilesLeave
  ; Installation complete
FunctionEnd

Section "Install"
  SetOutPath $INSTDIR

  ; Copy files from Tauri build
  File /r "release\*.*"

  ; Create shortcuts
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

  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  IntFmt $0 "0x%08X" $0
  WriteRegDWORD HKCU "${PRODUCT_UNINST_KEY}" "EstimatedSize" "$0"
SectionEnd

Section "Uninstall"
  ; Remove files
  RMDir /r "$INSTDIR"

  ; Remove shortcuts
  Delete "$DESKTOP\${PRODUCT_NAME}.lnk"
  RMDir /r "$SMPROGRAMS\${PRODUCT_NAME}"

  ; Remove registry
  DeleteRegKey HKCU "${PRODUCT_UNINST_KEY}"
  DeleteRegKey HKCU "${PRODUCT_DIR_REGKEY}"
SectionEnd

Function .onInstSuccess
  ; Launch app
  Exec "$INSTDIR\zimaos-echo.exe"
FunctionEnd
