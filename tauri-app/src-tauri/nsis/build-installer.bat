@echo off
REM ZimaOS Blue - Build Beautified NSIS Installer (NiuniuSkin)
REM Run this after `cargo tauri build` or with pre-built binaries in FilesToInstall

setlocal enabledelayedexpansion

set "SCRIPT_DIR=%~dp0"
set "SKIN_DIR=%SCRIPT_DIR%skin-installer"
set "FILES_DIR=%SKIN_DIR%\FilesToInstall"
set "SETUP_DIR=%SKIN_DIR%\SetupScripts"
set "ZIMAOS_DIR=%SETUP_DIR%\zimaos"
set "OUTPUT_DIR=%SKIN_DIR%\Output"
set "TAURI_DIR=%SCRIPT_DIR%.."

echo === ZimaOS Blue Beautified Installer Builder ===
echo.

REM Step 1: Try to copy from Tauri build if available
set "RELEASE_DIR=%TAURI_DIR%\target\release"
if not exist "%RELEASE_DIR%\zimaos-blue.exe" (
    set "RELEASE_DIR=%TAURI_DIR%\target\x86_64-pc-windows-gnu\release"
)
if not exist "%RELEASE_DIR%\zimaos-blue.exe" (
    set "RELEASE_DIR=%TAURI_DIR%\target\x86_64-pc-windows-msvc\release"
)

if exist "%RELEASE_DIR%\zimaos-blue.exe" (
    echo Copying files from Tauri build: %RELEASE_DIR%
    copy /y "%RELEASE_DIR%\zimaos-blue.exe" "%FILES_DIR%\"
    copy /y "%RELEASE_DIR%\WebView2Loader.dll" "%FILES_DIR%\"
    if exist "%TAURI_DIR%\bin\blue-server-x86_64-pc-windows-gnu.exe" (
        copy /y "%TAURI_DIR%\bin\blue-server-x86_64-pc-windows-gnu.exe" "%FILES_DIR%\blue-server.exe"
    )
    if exist "%TAURI_DIR%\bin\blue-server-x86_64-pc-windows-msvc.exe" (
        copy /y "%TAURI_DIR%\bin\blue-server-x86_64-pc-windows-msvc.exe" "%FILES_DIR%\blue-server.exe"
    )
) else (
    echo No Tauri build found, using existing files in FilesToInstall...
)

REM Verify required files exist
if not exist "%FILES_DIR%\zimaos-blue.exe" (
    echo ERROR: zimaos-blue.exe not found in %FILES_DIR%
    echo Please run 'cargo tauri build' first or copy binaries manually.
    exit /b 1
)

echo.
echo Files to install:
dir /b "%FILES_DIR%"
echo.

REM Step 2: Create app.7z
echo Creating app.7z...
cd /d "%SKIN_DIR%"
if exist "%SETUP_DIR%\app.7z" del "%SETUP_DIR%\app.7z"
"%SKIN_DIR%\7z.exe" a "%SETUP_DIR%\app.7z" "%FILES_DIR%\*.*"

REM Also compress subdirectories if any
for /f "delims=" %%a in ('dir /ad /b "%FILES_DIR%" 2^>nul') do (
    "%SKIN_DIR%\7z.exe" a "%SETUP_DIR%\app.7z" "%FILES_DIR%\%%a"
    echo Compressing: %%a
)

if errorlevel 1 (
    echo ERROR: Failed to create app.7z
    exit /b 1
)
echo app.7z created successfully.

REM Step 3: Create skin.zip for zimaos theme (must cd into skin dir for correct relative paths)
echo Creating skin.zip...
if exist "%ZIMAOS_DIR%\skin.zip" del "%ZIMAOS_DIR%\skin.zip"
pushd "%ZIMAOS_DIR%\skin"
"%SKIN_DIR%\7z.exe" a "..\skin.zip" ".\*" -r
popd

if errorlevel 1 (
    echo ERROR: Failed to create skin.zip
    exit /b 1
)
echo skin.zip created successfully.

REM Step 4: Ensure Output directory exists
if not exist "%OUTPUT_DIR%" mkdir "%OUTPUT_DIR%"

REM Step 5: Build with NSIS
echo.
echo Building NSIS installer...
cd /d "%ZIMAOS_DIR%"
"%SKIN_DIR%\NSIS\makensis.exe" "%ZIMAOS_DIR%\zimaos_setup.nsi"

if errorlevel 1 (
    echo.
    echo ERROR: NSIS build failed.
    exit /b 1
)

echo.
echo === Build Complete ===
if exist "%OUTPUT_DIR%\ZimaOS-Blue_*.exe" (
    for %%f in ("%OUTPUT_DIR%\ZimaOS-Blue_*.exe") do (
        echo Output: %%f
    )
) else (
    echo WARNING: Output file not found in %OUTPUT_DIR%
)

endlocal
