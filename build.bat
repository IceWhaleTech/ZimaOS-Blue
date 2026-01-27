@echo off
REM Build script for ZimaOS-Echo (Windows)
REM This script builds the frontend and embeds it into the Go binary

setlocal enabledelayedexpansion

REM Get script directory
set "SCRIPT_DIR=%~dp0"
set "PROJECT_ROOT=%SCRIPT_DIR:~0,-1%"

REM Version info
if "%VERSION%"=="" set "VERSION=0.9.0"
for /f "tokens=*" %%i in ('powershell -command "Get-Date -Format 'yyyy-MM-dd_HH:mm:ss'"') do set "BUILD_TIME=%%i"
for /f "tokens=*" %%i in ('git rev-parse --short HEAD 2^>nul') do set "GIT_COMMIT=%%i"
if "%GIT_COMMIT%"=="" set "GIT_COMMIT=unknown"

echo Building ZimaOS-Echo v%VERSION%
echo Build time: %BUILD_TIME%
echo Git commit: %GIT_COMMIT%
echo.

REM Step 1: Build frontend
echo Step 1: Building frontend...
cd /d "%PROJECT_ROOT%\web"

if not exist "node_modules" (
    echo Installing npm dependencies...
    call npm install
    if errorlevel 1 (
        echo Failed to install npm dependencies
        exit /b 1
    )
)

echo Building production frontend...
call npm run build
if errorlevel 1 (
    echo Failed to build frontend
    exit /b 1
)

REM Step 2: Copy frontend build to server embed directory
echo Step 2: Copying frontend build to server...
set "EMBED_DIR=%PROJECT_ROOT%\server\internal\web\dist"
if exist "%EMBED_DIR%" rmdir /s /q "%EMBED_DIR%"
mkdir "%EMBED_DIR%"
xcopy /s /e /q "dist\*" "%EMBED_DIR%\"
echo Frontend copied to %EMBED_DIR%

REM Step 3: Build Go binary
echo Step 3: Building Go binary...
cd /d "%PROJECT_ROOT%\server"

REM Build flags
set "LDFLAGS=-s -w"
set "LDFLAGS=%LDFLAGS% -X main.version=%VERSION%"
set "LDFLAGS=%LDFLAGS% -X main.buildTime=%BUILD_TIME%"
set "LDFLAGS=%LDFLAGS% -X main.gitCommit=%GIT_COMMIT%"

REM Output directory
set "OUTPUT_DIR=%PROJECT_ROOT%\dist"
if not exist "%OUTPUT_DIR%" mkdir "%OUTPUT_DIR%"

REM Binary name
set "BINARY_NAME=zimaos-echo.exe"
set "OUTPUT_PATH=%OUTPUT_DIR%\%BINARY_NAME%"

echo Building for windows/amd64...
set CGO_ENABLED=0
go build -ldflags "%LDFLAGS%" -o "%OUTPUT_PATH%" .\cmd\echo
if errorlevel 1 (
    echo Failed to build Go binary
    exit /b 1
)

REM Get file size
for %%A in ("%OUTPUT_PATH%") do set "SIZE=%%~zA"
set /a "SIZE_MB=%SIZE% / 1048576"
echo Binary size: approximately %SIZE_MB% MB

echo.
echo Build complete!
echo Output: %OUTPUT_PATH%
echo.
echo To run the server:
echo   %OUTPUT_PATH%
echo.
echo To run with custom config:
echo   %OUTPUT_PATH% -config path\to\config.yaml

endlocal
