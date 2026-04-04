; ZimaOS Blue Setup Script
!define PRODUCT_NAME                    "ZimaOS Blue"
!define PRODUCT_PATHNAME                "ZimaOS_Blue"
!define INSTALL_APPEND_PATH             "ZimaOS Blue"
!define INSTALL_DEFALT_SETUPPATH        ""
!define EXE_NAME                        "blue.exe"
!define PRODUCT_VERSION                 "0.10.38.0"
!define PRODUCT_PUBLISHER               "ZimaOS Team"
!define PRODUCT_LEGAL                   "ZimaOS Team Copyright 2024"
!define INSTALL_OUTPUT_NAME             "ZimaOS-Blue_0.10.38_x64-setup.exe"

!define INSTALL_7Z_PATH                 "..\app.7z"
!define INSTALL_7Z_NAME                 "app.7z"
!define INSTALL_RES_PATH                "skin.zip"
!define INSTALL_LICENCE_FILENAME        "license.txt"
!define INSTALL_ICO                     "logo.ico"

; ==================== NSIS 3.x Modern Features ====================
; Set maximum compression for smaller installer size
; IMPORTANT: Must be set BEFORE any !include statements
SetCompressor /SOLID /FINAL lzma
SetCompressorDictSize 96
SetDatablockOptimize on

!include "ui_zimaos_setup.nsh"

; Enable DPI awareness for high-resolution displays (NSIS 3.03+)
ManifestDPIAware true

; Declare supported operating systems (NSIS 3.0+)
; Only Windows 10 and later are supported (Win11 uses Win10 GUID)
ManifestSupportedOS Win10

; Enable long path support for Windows 10 1607+ (NSIS 3.06+)
ManifestLongPathAware true

RequestExecutionLevel user
Name "${PRODUCT_NAME}"
OutFile "..\..\..\Output\${INSTALL_OUTPUT_NAME}"
InstallDir "$LOCALAPPDATA\${INSTALL_APPEND_PATH}"
Icon "${INSTALL_ICO}"
UninstallIcon "${INSTALL_ICO}"

Section "None"
SectionEnd
