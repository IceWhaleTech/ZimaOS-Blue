; ZimaOS Echo - Custom Modern Installer
; One-click simple installation with modern UI

Unicode true
ManifestDPIAware true
ManifestDPIAwareness PerMonitorV2

!include "MUI2.nsh"
!include "FileFunc.nsh"
!include "x64.nsh"

; Compression
SetCompressor /SOLID lzma

; Modern UI Configuration
!define MUI_ABORTWARNING
!define MUI_ICON "${NSISDIR}\Contrib\Graphics\Icons\modern-install.ico"
!define MUI_UNICON "${NSISDIR}\Contrib\Graphics\Icons\modern-uninstall.ico"

; Welcome page configuration
!define MUI_WELCOMEFINISHPAGE_BITMAP "${NSISDIR}\Contrib\Graphics\Wizard\win.bmp"
!define MUI_WELCOMEPAGE_TITLE_3LINES

; Finish page configuration
!define MUI_FINISHPAGE_NOAUTOCLOSE
!define MUI_FINISHPAGE_RUN "$INSTDIR\${MAINBINARYNAME}.exe"
!define MUI_FINISHPAGE_RUN_TEXT "$(LaunchApp)"

; Pages - Simplified flow
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

; 27 Languages
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "SimpChinese"
!insertmacro MUI_LANGUAGE "TradChinese"
!insertmacro MUI_LANGUAGE "Japanese"
!insertmacro MUI_LANGUAGE "Korean"
!insertmacro MUI_LANGUAGE "German"
!insertmacro MUI_LANGUAGE "French"
!insertmacro MUI_LANGUAGE "Spanish"
!insertmacro MUI_LANGUAGE "SpanishInternational"
!insertmacro MUI_LANGUAGE "Portuguese"
!insertmacro MUI_LANGUAGE "PortugueseBR"
!insertmacro MUI_LANGUAGE "Italian"
!insertmacro MUI_LANGUAGE "Dutch"
!insertmacro MUI_LANGUAGE "Russian"
!insertmacro MUI_LANGUAGE "Polish"
!insertmacro MUI_LANGUAGE "Ukrainian"
!insertmacro MUI_LANGUAGE "Czech"
!insertmacro MUI_LANGUAGE "Slovak"
!insertmacro MUI_LANGUAGE "Hungarian"
!insertmacro MUI_LANGUAGE "Romanian"
!insertmacro MUI_LANGUAGE "Bulgarian"
!insertmacro MUI_LANGUAGE "Turkish"
!insertmacro MUI_LANGUAGE "Arabic"
!insertmacro MUI_LANGUAGE "Hebrew"
!insertmacro MUI_LANGUAGE "Thai"
!insertmacro MUI_LANGUAGE "Vietnamese"
!insertmacro MUI_LANGUAGE "Indonesian"

; Custom language strings
LangString LaunchApp ${LANG_ENGLISH} "Launch ZimaOS Echo"
LangString LaunchApp ${LANG_SIMPCHINESE} "启动 ZimaOS Echo"
LangString LaunchApp ${LANG_TRADCHINESE} "啟動 ZimaOS Echo"
LangString LaunchApp ${LANG_JAPANESE} "ZimaOS Echo を起動"
LangString LaunchApp ${LANG_KOREAN} "ZimaOS Echo 실행"
LangString LaunchApp ${LANG_GERMAN} "ZimaOS Echo starten"
LangString LaunchApp ${LANG_FRENCH} "Lancer ZimaOS Echo"
LangString LaunchApp ${LANG_SPANISH} "Iniciar ZimaOS Echo"
LangString LaunchApp ${LANG_SPANISHINTERNATIONAL} "Iniciar ZimaOS Echo"
LangString LaunchApp ${LANG_PORTUGUESE} "Iniciar ZimaOS Echo"
LangString LaunchApp ${LANG_PORTUGUESEBR} "Iniciar ZimaOS Echo"
LangString LaunchApp ${LANG_ITALIAN} "Avvia ZimaOS Echo"
LangString LaunchApp ${LANG_DUTCH} "ZimaOS Echo starten"
LangString LaunchApp ${LANG_RUSSIAN} "Запустить ZimaOS Echo"
LangString LaunchApp ${LANG_POLISH} "Uruchom ZimaOS Echo"
LangString LaunchApp ${LANG_UKRAINIAN} "Запустити ZimaOS Echo"
LangString LaunchApp ${LANG_CZECH} "Spustit ZimaOS Echo"
LangString LaunchApp ${LANG_SLOVAK} "Spustiť ZimaOS Echo"
LangString LaunchApp ${LANG_HUNGARIAN} "ZimaOS Echo indítása"
LangString LaunchApp ${LANG_ROMANIAN} "Lansați ZimaOS Echo"
LangString LaunchApp ${LANG_BULGARIAN} "Стартирайте ZimaOS Echo"
LangString LaunchApp ${LANG_TURKISH} "ZimaOS Echo'yu Başlat"
LangString LaunchApp ${LANG_ARABIC} "تشغيل ZimaOS Echo"
LangString LaunchApp ${LANG_HEBREW} "הפעל את ZimaOS Echo"
LangString LaunchApp ${LANG_THAI} "เปิด ZimaOS Echo"
LangString LaunchApp ${LANG_VIETNAMESE} "Khởi chạy ZimaOS Echo"
LangString LaunchApp ${LANG_INDONESIAN} "Jalankan ZimaOS Echo"

Function .onInit
  !insertmacro MUI_LANGDLL_DISPLAY
FunctionEnd
