; ZimaOS Echo Setup Script
!define PRODUCT_NAME                    "ZimaOS Echo"
!define PRODUCT_PATHNAME                "ZimaOS_Echo"
!define INSTALL_APPEND_PATH             "ZimaOS Echo"
!define INSTALL_DEFALT_SETUPPATH        ""
!define EXE_NAME                        "zimaos-echo.exe"
!define PRODUCT_VERSION                 "0.10.17.0"
!define PRODUCT_PUBLISHER               "ZimaOS Team"
!define PRODUCT_LEGAL                   "ZimaOS Team Copyright 2024"
!define INSTALL_OUTPUT_NAME             "ZimaOS-Echo_0.10.17_x64-setup.exe"

!define INSTALL_7Z_PATH                 "..\app.7z"
!define INSTALL_7Z_NAME                 "app.7z"
!define INSTALL_RES_PATH                "skin.zip"
!define INSTALL_LICENCE_FILENAME        "license.txt"
!define INSTALL_ICO                     "logo.ico"

!include "ui_zimaos_setup.nsh"

RequestExecutionLevel user
Name "${PRODUCT_NAME}"
OutFile "..\..\Output\${INSTALL_OUTPUT_NAME}"
InstallDir "$LOCALAPPDATA\${INSTALL_APPEND_PATH}"
Icon "${INSTALL_ICO}"
UninstallIcon "${INSTALL_ICO}"

Section "None"
SectionEnd
