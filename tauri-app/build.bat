@echo off
REM ZimaOS Blue - Windows Build Script
REM Builds the complete Tauri application + beautified NSIS installer
REM
REM Build Strategy: Sidecar approach (Go binary as separate process)
REM Compression: LZMA in NSIS installer (no UPX on binary)

setlocal enabledelayedexpansion

set "SCRIPT_DIR=%~dp0"
set "PROJECT_ROOT=%SCRIPT_DIR%.."
set "TAURI_DIR=%SCRIPT_DIR%src-tauri"
set "WEB_DIR=%PROJECT_ROOT%\web"
set "SERVER_DIR=%PROJECT_ROOT%\server"
set "NSIS_DIR=%TAURI_DIR%\nsis"
set "SKIN_DIR=%NSIS_DIR%\skin-installer"
set "EMBED_DIR=%SERVER_DIR%\internal\web\dist"

echo ==========================================
echo ZimaOS Blue - Windows Build Script
echo ==========================================
echo.

REM Check dependencies
echo [STEP] Checking dependencies...
where node >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Node.js not installed
    exit /b 1
)
where cargo >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Rust/Cargo not installed
    exit /b 1
)
where go >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Go not installed
    exit /b 1
)
for /f "tokens=*" %%i in ('node --version') do echo   Node.js: %%i
for /f "tokens=*" %%i in ('cargo --version') do echo   Cargo: %%i
for /f "tokens=*" %%i in ('go version') do echo   Go: %%i
echo.

REM Step 1: Build frontend (fresh build)
echo [STEP] Building frontend (fresh build)...
cd /d "%WEB_DIR%"
if exist dist rmdir /s /q dist
if exist node_modules\.vite rmdir /s /q node_modules\.vite
call npm install
if errorlevel 1 (
    echo [ERROR] npm install failed
    exit /b 1
)
call npm run build
if errorlevel 1 (
    echo [ERROR] Frontend build failed
    exit /b 1
)
if not exist "%WEB_DIR%\dist\index.html" (
    echo [ERROR] Frontend build failed - dist directory is empty or missing
    exit /b 1
)
echo [OK] Frontend built
echo.

REM Step 2: Copy frontend to server/internal/web/dist for Go embedding
echo [STEP] Copying frontend to server/internal/web/dist for Go embedding...
if exist "%EMBED_DIR%" rmdir /s /q "%EMBED_DIR%"
mkdir "%EMBED_DIR%"
REM Copy all files except .map files
xcopy /s /e /y /q "%WEB_DIR%\dist\*" "%EMBED_DIR%\" >nul
REM Remove source maps
del /s /q "%EMBED_DIR%\*.map" >nul 2>&1
if not exist "%EMBED_DIR%\index.html" (
    echo [ERROR] Failed to copy frontend
    exit /b 1
)
echo [OK] Frontend copied to server/internal/web/dist
echo.

REM Step 3: Build Go sidecar (with embedded frontend)
echo [STEP] Building Go sidecar (with embedded frontend)...
cd /d "%SERVER_DIR%"
set "TARGET=x86_64-pc-windows-msvc"
set "SIDECAR_NAME=blue-server-%TARGET%.exe"

if not exist "%TAURI_DIR%\binaries" mkdir "%TAURI_DIR%\binaries"
if not exist "%TAURI_DIR%\bin" mkdir "%TAURI_DIR%\bin"

set CGO_ENABLED=0
go build -ldflags="-s -w" -o "%TAURI_DIR%\binaries\%SIDECAR_NAME%" ./cmd/blue/
if errorlevel 1 (
    echo [ERROR] Go sidecar build failed
    exit /b 1
)
copy /y "%TAURI_DIR%\binaries\%SIDECAR_NAME%" "%TAURI_DIR%\bin\" >nul
echo [OK] Sidecar built: %SIDECAR_NAME% (no UPX compression)
echo.

REM Step 4: Clean up data directory before build
echo [STEP] Cleaning up data directory...
if exist "%TAURI_DIR%\data" rmdir /s /q "%TAURI_DIR%\data"
mkdir "%TAURI_DIR%\data"
echo [OK] Data directory cleaned
echo.

REM Step 5: Build Tauri app
echo [STEP] Building Tauri application...
cd /d "%SCRIPT_DIR%"
call npm install
if errorlevel 1 (
    echo [ERROR] npm install failed
    exit /b 1
)
call npm run build
if errorlevel 1 (
    echo [ERROR] Tauri build failed
    exit /b 1
)
echo [OK] Tauri build complete
echo.

REM Step 6: Copy Tauri output to NSIS FilesToInstall
echo [STEP] Preparing files for NSIS installer...
set "FILES_DIR=%SKIN_DIR%\FilesToInstall"

REM Try multiple possible release paths
set "RELEASE_DIR=%TAURI_DIR%\target\release"
if not exist "!RELEASE_DIR!\zimaos-blue.exe" set "RELEASE_DIR=%TAURI_DIR%\target\x86_64-pc-windows-msvc\release"
if not exist "!RELEASE_DIR!\zimaos-blue.exe" set "RELEASE_DIR=%TAURI_DIR%\target\x86_64-pc-windows-gnu\release"

if exist "!RELEASE_DIR!\zimaos-blue.exe" (
    copy /y "!RELEASE_DIR!\zimaos-blue.exe" "%FILES_DIR%\" >nul
    copy /y "!RELEASE_DIR!\WebView2Loader.dll" "%FILES_DIR%\" >nul
    copy /y "%TAURI_DIR%\bin\%SIDECAR_NAME%" "%FILES_DIR%\blue-server.exe" >nul
    echo [OK] Files copied from Tauri build
) else (
    echo [WARN] Tauri release not found, using existing FilesToInstall
)
echo.

REM Step 7: Build beautified NSIS installer
echo [STEP] Building beautified NSIS installer...
cd /d "%NSIS_DIR%"
call build-installer.bat
if errorlevel 1 (
    echo [ERROR] NSIS installer build failed
    exit /b 1
)

echo.
echo ==========================================
echo Build Complete!
echo ==========================================
echo.
echo Build Strategy: Sidecar Process
echo Compression: None on binary (installer handles compression)
echo.
echo Output:
if exist "%SKIN_DIR%\Output\ZimaOS-Blue_*.exe" (
    for %%f in ("%SKIN_DIR%\Output\ZimaOS-Blue_*.exe") do (
        echo   %%f
        for %%A in ("%%f") do (
            set "sz=%%~zA"
            set /a "mb=!sz! / 1048576"
            echo   Size: !mb! MB
        )
    )
) else (
    echo   WARNING: Output file not found in %SKIN_DIR%\Output
)
echo.
echo Done!

endlocal
