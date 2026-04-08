@echo off
setlocal enabledelayedexpansion

:: ZimaOS-Blue Development Script
:: Usage: build.bat [command]
:: Commands: start (default), server, web, build, clean, prd
:: Optional signing env vars:
::   WINDOWS_CERTIFICATE_THUMBPRINT=certificate SHA1 thumbprint (required to sign)
::   WINDOWS_TIMESTAMP_URL=RFC3161 timestamp URL (default: http://timestamp.digicert.com)
::   SIGNTOOL_PATH=full path to signtool.exe (optional)
::   WINDOWS_SIGN_CSP=SafeNet CSP/KSP provider name (optional)
::   WINDOWS_SIGN_KC=SafeNet key container name (optional)

set "PROJECT_ROOT=%~dp0"
set "COMMAND=%~1"

:: Enable CGO for Windows native TTS/ASR (SAPI)
set "CGO_ENABLED=1"
set "CC=gcc"
set "CXX=g++"
set "CGO_LDFLAGS=-static-libgcc -static-libstdc++"
set "CGO_CFLAGS=-O2"
set "CGO_CXXFLAGS=-O2"

if "%COMMAND%"=="" set "COMMAND=prd"

:: Subroutines must be defined before goto (Windows batch quirk)
goto :run

:check_prereqs
echo [INFO] Checking prerequisites...
where go >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Go is not installed. Download from https://golang.org/dl/
    exit /b 1
)
where node >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Node.js is not installed. Download from https://nodejs.org/
    exit /b 1
)
where npm >nul 2>&1
if errorlevel 1 (
    echo [ERROR] npm is not installed.
    exit /b 1
)
echo [OK] All prerequisites found
exit /b 0

:install_deps
echo [INFO] Installing dependencies...
cd /d "%PROJECT_ROOT%server"
go mod download
if errorlevel 1 (
    echo [ERROR] Failed to install Go dependencies
    exit /b 1
)
cd /d "%PROJECT_ROOT%web"
if not exist "node_modules" (
    echo [INFO] Installing Node dependencies...
    call npm install
    if errorlevel 1 (
        echo [ERROR] Failed to install Node dependencies
        exit /b 1
    )
) else (
    echo [INFO] Node modules already installed, skipping...
)
echo [OK] Dependencies installed
exit /b 0

:build_third_party
echo [INFO] Skipping third_party native libraries (Windows uses native TTS/ASR only)...

:: Windows uses native SAPI for TTS/ASR, no need to build:
:: - espeak-ng (replaced by Windows native TTS)
:: - whisper.cpp (replaced by Windows native ASR)
:: - opus (not needed for Windows native)

echo [OK] Third_party libraries not needed on Windows
exit /b 0

:run_web_audit
if exist "%PROJECT_ROOT%web\scripts\audit-ci.mjs" (
    if exist "%PROJECT_ROOT%web\audit-allowlist.json" (
        call node scripts\audit-ci.mjs --omit=dev
    ) else (
        call node scripts\audit-ci.mjs --no-allowlist --omit=dev
    )
) else (
    call npm audit --omit=dev
)
exit /b %errorlevel%

:handle_web_audit_failure
set "AUDIT_STATUS=%~1"
if "%AUDIT_STATUS%"=="1" (
    echo [ERROR] Production dependencies have vulnerabilities. Please fix them before building.
) else (
    if "%AUDIT_STATUS%"=="2" (
        echo [ERROR] Unable to complete npm audit because the npm registry request failed. Check network access and retry.
    ) else (
        echo [ERROR] npm audit failed unexpectedly ^(exit code: %AUDIT_STATUS%^).
    )
)
exit /b 0

:pack_dist_into_binary
set "BIN_PATH=%~1"
set "DIST_DIR=%~2"
set "ARCHIVE_PATH=%TEMP%\zimaos-dist.tar.gz"

if "%BIN_PATH%"=="" (
    echo [ERROR] Missing binary path for pack step.
    exit /b 1
)
if "%DIST_DIR%"=="" (
    echo [ERROR] Missing dist directory for pack step.
    exit /b 1
)

if exist "%ARCHIVE_PATH%" del /f /q "%ARCHIVE_PATH%" >nul 2>&1

echo [INFO] Packing dist into binary...
tar czf "%ARCHIVE_PATH%" -C "%DIST_DIR%" .
if errorlevel 1 (
    echo [ERROR] Failed to create dist archive.
    exit /b 1
)

for %%F in ("%BIN_PATH%") do set "OFFSET=%%~zF"
if not defined OFFSET (
    echo [ERROR] Failed to determine binary size before packing.
    if exist "%ARCHIVE_PATH%" del /f /q "%ARCHIVE_PATH%" >nul 2>&1
    exit /b 1
)

copy /b "%BIN_PATH%" + "%ARCHIVE_PATH%" "%BIN_PATH%" >nul
if errorlevel 1 (
    echo [ERROR] Failed to append dist archive to binary.
    if exist "%ARCHIVE_PATH%" del /f /q "%ARCHIVE_PATH%" >nul 2>&1
    exit /b 1
)

powershell -NoProfile -ExecutionPolicy Bypass -Command "$offset=[Int64]$env:OFFSET; $bytes=[System.BitConverter]::GetBytes($offset); $fs=[System.IO.File]::Open($env:BIN_PATH,[System.IO.FileMode]::Append,[System.IO.FileAccess]::Write,[System.IO.FileShare]::Read); try { $fs.Write($bytes,0,$bytes.Length) } finally { $fs.Dispose() }"
if errorlevel 1 (
    echo [ERROR] Failed to append binary trailer.
    if exist "%ARCHIVE_PATH%" del /f /q "%ARCHIVE_PATH%" >nul 2>&1
    exit /b 1
)

del /f /q "%ARCHIVE_PATH%" >nul 2>&1
if exist "%ARCHIVE_PATH%" (
    echo [ERROR] Failed to remove temporary dist archive: %ARCHIVE_PATH%
    exit /b 1
)

set "OFFSET="
set "ARCHIVE_PATH="
set "DIST_DIR="
set "BIN_PATH="
exit /b 0

:: Windows signing is handled by scripts\sign-windows.cmd

:run
goto :%COMMAND% 2>nul || (
    echo Unknown command: %COMMAND%
    echo Usage: build.bat [start^|server^|web^|build^|clean^|prd]
    exit /b 1
)

:start
echo.
echo ========================================
echo   ZimaOS-Blue Development Environment
echo ========================================
echo.
echo   Backend:  http://localhost
echo   Frontend: http://localhost:3000 (background)
echo.
echo   Press Ctrl+C to stop backend server
echo.

:: Check prerequisites
call :check_prereqs
if errorlevel 1 exit /b 1

:: Install dependencies
call :install_deps
if errorlevel 1 exit /b 1

:: Start Vite dev server in background FIRST (Go server proxies to it)
echo [INFO] Starting Vite dev server in background...
start "ZimaOS-Blue Web" /min cmd /c "cd /d "%PROJECT_ROOT%web" && npm run dev"

:: Wait for Vite to start
timeout /t 3 /nobreak >nul

:: Start Go server in foreground (dev mode - proxies to Vite)
echo [INFO] Starting Go server (dev mode)...
cd /d "%PROJECT_ROOT%server"
go run -tags dev ./cmd/blue
goto :eof

:server
call :check_prereqs
if errorlevel 1 exit /b 1

echo [INFO] Starting Go server...
cd /d "%PROJECT_ROOT%server"
go run  -tags dev ./cmd/blue
goto :eof

:web
call :check_prereqs
if errorlevel 1 exit /b 1

echo [INFO] Starting web dev server...
cd /d "%PROJECT_ROOT%web"
echo [INFO] Installing Node dependencies...
call npm install
call npm run dev
goto :eof

:build
call :check_prereqs
if errorlevel 1 exit /b 1

echo [INFO] Building for production...

:: Check production dependencies for vulnerabilities
echo [INFO] Checking production dependencies for vulnerabilities...
cd /d "%PROJECT_ROOT%web"
call :run_web_audit
set "AUDIT_STATUS=%errorlevel%"
if not "%AUDIT_STATUS%"=="0" (
    call :handle_web_audit_failure %AUDIT_STATUS%
    exit /b 1
)
echo.

:: Build third_party native libraries
call :build_third_party
if errorlevel 1 exit /b 1

:: Build web FIRST (Go embeds frontend at compile time)
echo [INFO] Building web frontend...
cd /d "%PROJECT_ROOT%web"
if not exist "node_modules" call npm install
call npm run build
if errorlevel 1 (
    echo [ERROR] Failed to build web
    exit /b 1
)
echo [OK] Web built: web\dist\

:: Copy web\dist to server\internal\web\dist (required for go:embed)
echo [INFO] Copying web build to server\internal\web\dist...
if exist "%PROJECT_ROOT%server\internal\web\dist" rmdir /s /q "%PROJECT_ROOT%server\internal\web\dist"
xcopy /E /I /Y "%PROJECT_ROOT%web\dist" "%PROJECT_ROOT%server\internal\web\dist" >nul
if errorlevel 1 (
    echo [ERROR] Failed to copy web dist
    exit /b 1
)
echo [OK] Web assets copied to server\internal\web\dist

:: Copy canonical skills from assets\skills to server\internal\skill\embedded\skills (required for go:embed)
echo [INFO] Copying skills to server\internal\skill\embedded\skills...
if exist "%PROJECT_ROOT%server\internal\skill\embedded\skills" rmdir /s /q "%PROJECT_ROOT%server\internal\skill\embedded\skills"
xcopy /E /I /Y "%PROJECT_ROOT%assets\skills" "%PROJECT_ROOT%server\internal\skill\embedded\skills" >nul
if errorlevel 1 (
    echo [ERROR] Failed to copy skills
    exit /b 1
)
echo [OK] Skills copied to server\internal\skill\embedded\skills

:: Build server with pack-dist (appends web assets to binary)
echo [INFO] Building Go server...
cd /d "%PROJECT_ROOT%server"
go build -ldflags="-s -w" -o bin\blue.exe ./cmd/blue
if errorlevel 1 (
    echo [ERROR] Failed to build server
    exit /b 1
)
:: Pack dist into binary (tar.gz + 8-byte LE offset trailer)
call :pack_dist_into_binary "bin\blue.exe" "internal\web\dist"
if errorlevel 1 exit /b 1
call "%PROJECT_ROOT%scripts\sign-windows.cmd" "%PROJECT_ROOT%server\bin\blue.exe"
if errorlevel 1 exit /b 1
echo [OK] Server built: server\bin\blue.exe

echo [OK] Production build complete!
goto :eof

:prd
call :check_prereqs
if errorlevel 1 exit /b 1

echo [INFO] Production run: build web, copy to server/internal/web, start server...

:: Check production dependencies for vulnerabilities
echo [INFO] Checking production dependencies for vulnerabilities...
cd /d "%PROJECT_ROOT%web"
call npm install
call :run_web_audit
set "AUDIT_STATUS=%errorlevel%"
if not "%AUDIT_STATUS%"=="0" (
    call :handle_web_audit_failure %AUDIT_STATUS%
    exit /b 1
)
echo.

:: Build web
echo [INFO] Building web frontend...
cd /d "%PROJECT_ROOT%web"
call npm run build
if errorlevel 1 (
    echo [ERROR] Failed to build web
    exit /b 1
)
echo [OK] Web built: web\dist\

:: Copy web\dist to server\internal\web\dist
echo [INFO] Copying web build to server\internal\web\dist...
if exist "%PROJECT_ROOT%server\internal\web\dist" rmdir /s /q "%PROJECT_ROOT%server\internal\web\dist"
xcopy /E /I /Y "%PROJECT_ROOT%web\dist" "%PROJECT_ROOT%server\internal\web\dist" >nul
if errorlevel 1 (
    echo [ERROR] Failed to copy web dist
    exit /b 1
)
echo [OK] Web assets copied to server\internal\web\dist

:: Copy canonical skills from assets\skills to server\internal\skill\embedded\skills (required for go:embed)
echo [INFO] Copying skills to server\internal\skill\embedded\skills...
if exist "%PROJECT_ROOT%server\internal\skill\embedded\skills" rmdir /s /q "%PROJECT_ROOT%server\internal\skill\embedded\skills"
xcopy /E /I /Y "%PROJECT_ROOT%assets\skills" "%PROJECT_ROOT%server\internal\skill\embedded\skills" >nul
if errorlevel 1 (
    echo [ERROR] Failed to copy skills
    exit /b 1
)
echo [OK] Skills copied to server\internal\skill\embedded\skills

:: Build server with pack-dist (appends web assets to binary)
echo [INFO] Building Go server (production mode)...
cd /d "%PROJECT_ROOT%server"
go build -ldflags="-s -w" -o bin\blue.exe ./cmd/blue
if errorlevel 1 (
    echo [ERROR] Failed to build server
    exit /b 1
)
:: Pack dist into binary (tar.gz + 8-byte LE offset trailer)
call :pack_dist_into_binary "bin\blue.exe" "internal\web\dist"
if errorlevel 1 exit /b 1
call "%PROJECT_ROOT%scripts\sign-windows.cmd" "%PROJECT_ROOT%server\bin\blue.exe"
if errorlevel 1 exit /b 1
echo [OK] Server built: server\bin\blue.exe

:: Run the built binary
echo [INFO] Starting server (production mode, http://localhost)...
bin\blue.exe
goto :eof

:clean
echo [INFO] Cleaning build artifacts...

if exist "%PROJECT_ROOT%server\blue.exe" del /f "%PROJECT_ROOT%server\blue.exe"
if exist "%PROJECT_ROOT%server\data" rmdir /s /q "%PROJECT_ROOT%server\data"
if exist "%PROJECT_ROOT%web\dist" rmdir /s /q "%PROJECT_ROOT%web\dist"
if exist "%PROJECT_ROOT%server\internal\web\dist" rmdir /s /q "%PROJECT_ROOT%server\internal\web\dist"
if exist "%PROJECT_ROOT%server\internal\skill\embedded\skills" rmdir /s /q "%PROJECT_ROOT%server\internal\skill\embedded\skills"

echo [OK] Clean complete!
goto :eof
