@echo off
REM ZimaOS Echo - Build Custom Installer
REM Run this after `cargo tauri build`

setlocal enabledelayedexpansion

set "SCRIPT_DIR=%~dp0"
set "TAURI_DIR=%SCRIPT_DIR%.."
set "RELEASE_DIR=%TAURI_DIR%\target\x86_64-pc-windows-gnu\release"
set "NSIS_DIR=%SCRIPT_DIR%"
set "OUTPUT_DIR=%RELEASE_DIR%\bundle\nsis"

echo === ZimaOS Echo Custom Installer Builder ===
echo.

REM Check if Tauri build exists
if not exist "%RELEASE_DIR%\zimaos-echo.exe" (
    echo ERROR: Tauri build not found. Run 'cargo tauri build' first.
    exit /b 1
)

REM Create release folder for NSIS
echo Creating release folder...
if exist "%NSIS_DIR%\release" rmdir /s /q "%NSIS_DIR%\release"
mkdir "%NSIS_DIR%\release"

REM Copy files from Tauri build
echo Copying files...
copy "%RELEASE_DIR%\zimaos-echo.exe" "%NSIS_DIR%\release\"
copy "%RELEASE_DIR%\WebView2Loader.dll" "%NSIS_DIR%\release\"
copy "%TAURI_DIR%\bin\echo-server-x86_64-pc-windows-gnu.exe" "%NSIS_DIR%\release\echo-server.exe"

REM Copy icons
echo Copying icons...
xcopy /s /i /y "%TAURI_DIR%\icons" "%NSIS_DIR%\icons"

REM Build with NSIS
echo.
echo Building installer with NSIS...
cd /d "%NSIS_DIR%"

REM Find NSIS
set "NSIS_PATH="
if exist "C:\Program Files (x86)\NSIS\makensis.exe" set "NSIS_PATH=C:\Program Files (x86)\NSIS\makensis.exe"
if exist "C:\Program Files\NSIS\makensis.exe" set "NSIS_PATH=C:\Program Files\NSIS\makensis.exe"
if exist "%LOCALAPPDATA%\tauri\NSIS\makensis.exe" set "NSIS_PATH=%LOCALAPPDATA%\tauri\NSIS\makensis.exe"

if "!NSIS_PATH!"=="" (
    echo ERROR: NSIS not found. Please install NSIS.
    exit /b 1
)

echo Using NSIS: !NSIS_PATH!
"!NSIS_PATH!" /V4 modern-installer.nsi

if errorlevel 1 (
    echo ERROR: NSIS build failed.
    exit /b 1
)

REM Move output
if exist "ZimaOS-Echo_*.exe" (
    move /y "ZimaOS-Echo_*.exe" "%OUTPUT_DIR%\"
    echo.
    echo === Build Complete ===
    echo Output: %OUTPUT_DIR%\ZimaOS-Echo_0.10.17_x64-setup.exe
) else (
    echo ERROR: Output file not found.
    exit /b 1
)

endlocal
