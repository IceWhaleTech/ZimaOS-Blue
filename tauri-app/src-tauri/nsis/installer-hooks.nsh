; ZimaOS Echo - Custom Installer Hooks
; Modern one-click installer with logo display

!include "nsDialogs.nsh"
!include "LogicLib.nsh"

; Variables for custom page
Var CustomDialog
Var LogoImage
Var TitleLabel
Var SubtitleLabel
Var InstallButton
Var ProgressBar
Var StatusLabel

; Custom welcome page with logo
Page custom CustomWelcomePage CustomWelcomePageLeave

Function CustomWelcomePage
  ; Skip if passive mode
  ${If} $PassiveMode = 1
    Abort
  ${EndIf}

  nsDialogs::Create 1018
  Pop $CustomDialog
  ${If} $CustomDialog == error
    Abort
  ${EndIf}

  ; Set dialog background to white
  SetCtlColors $CustomDialog 0x333333 0xFFFFFF

  ; Logo image (centered at top)
  ${NSD_CreateBitmap} 200 20 100 100 ""
  Pop $LogoImage
  ${NSD_SetImage} $LogoImage "$PLUGINSDIR\logo.bmp" $0

  ; Title
  ${NSD_CreateLabel} 0 140 100% 30 "ZimaOS Echo"
  Pop $TitleLabel
  SetCtlColors $TitleLabel 0x333333 0xFFFFFF
  CreateFont $0 "Segoe UI" 18 700
  SendMessage $TitleLabel ${WM_SETFONT} $0 1

  ; Subtitle
  ${NSD_CreateLabel} 0 175 100% 20 "AI Gateway for Local LLM Integration"
  Pop $SubtitleLabel
  SetCtlColors $SubtitleLabel 0x666666 0xFFFFFF
  CreateFont $0 "Segoe UI" 10 400
  SendMessage $SubtitleLabel ${WM_SETFONT} $0 1

  ; Version info
  ${NSD_CreateLabel} 0 200 100% 15 "Version ${VERSION}"
  Pop $0
  SetCtlColors $0 0x999999 0xFFFFFF
  CreateFont $1 "Segoe UI" 9 400
  SendMessage $0 ${WM_SETFONT} $1 1

  ; Install button (large, centered)
  ${NSD_CreateButton} 150 250 200 40 "Install"
  Pop $InstallButton
  SetCtlColors $InstallButton 0xFFFFFF 0x0078D4
  CreateFont $0 "Segoe UI" 12 600
  SendMessage $InstallButton ${WM_SETFONT} $0 1

  nsDialogs::Show
FunctionEnd

Function CustomWelcomePageLeave
  ; Continue to installation
FunctionEnd

; Pre-install hook - extract logo
!macro NSIS_HOOK_PREINSTALL
  ; Show installation progress
  DetailPrint "Installing ZimaOS Echo..."
!macroend

; Post-install hook
!macro NSIS_HOOK_POSTINSTALL
  DetailPrint "Installation complete!"
!macroend
