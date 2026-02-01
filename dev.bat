@echo off
setlocal enabledelayedexpansion

:: ZimaOS-Echo Development Script
:: Usage: dev.bat [command]
:: Commands: start (default), server, web, build, clean, prd

set "PROJECT_ROOT=%~dp0"
set "COMMAND=%~1"

if "%COMMAND%"=="" set "COMMAND=start"

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

:run
goto :%COMMAND% 2>nul || (
    echo Unknown command: %COMMAND%
    echo Usage: dev.bat [start^|server^|web^|build^|clean^|prd]
    exit /b 1
)

:start
echo.
echo ========================================
echo   ZimaOS-Echo Development Environment
echo ========================================
echo.
echo   Backend:  http://localhost:23456
echo   Frontend: http://localhost:3000
echo.

:: Check prerequisites
call :check_prereqs
if errorlevel 1 exit /b 1

:: Install dependencies
call :install_deps
if errorlevel 1 exit /b 1

:: Start Vite dev server in background FIRST (Go server proxies to it)
echo [INFO] Starting Vite dev server...
start "ZimaOS-Echo Web" cmd /c "cd /d "%PROJECT_ROOT%web" && npm run dev"

:: Wait for Vite to start
timeout /t 3 /nobreak >nul

:: Start Go server in foreground (dev mode - proxies to Vite)
echo [INFO] Starting Go server (dev mode)...
cd /d "%PROJECT_ROOT%server"
go run -tags dev ./cmd/echo
goto :eof

:server
call :check_prereqs
if errorlevel 1 exit /b 1

echo [INFO] Starting Go server...
cd /d "%PROJECT_ROOT%server"
go run  -tags dev ./cmd/echo
goto :eof

:web
call :check_prereqs
if errorlevel 1 exit /b 1

echo [INFO] Starting web dev server...
cd /d "%PROJECT_ROOT%web"
if not exist "node_modules" (
    echo [INFO] Installing Node dependencies...
    call npm install
)
call npm run dev
goto :eof

:build
call :check_prereqs
if errorlevel 1 exit /b 1

echo [INFO] Building for production...

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

:: Build server (embeds the copied frontend)
echo [INFO] Building Go server...
cd /d "%PROJECT_ROOT%server"
go build -ldflags="-s -w" -o echo.exe ./cmd/echo
if errorlevel 1 (
    echo [ERROR] Failed to build server
    exit /b 1
)
echo [OK] Server built: server\echo.exe

echo [OK] Production build complete!
goto :eof

:prd
call :check_prereqs
if errorlevel 1 exit /b 1

echo [INFO] Production run: build web, copy to server/internal/web, start server...

:: Build web
echo [INFO] Building web frontend...
cd /d "%PROJECT_ROOT%web"
if not exist "node_modules" call npm install
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

:: Start server (production mode, no -tags dev, serves embedded frontend)
echo [INFO] Starting Go server (production mode, http://localhost:23456)...
cd /d "%PROJECT_ROOT%server"
go run ./cmd/echo
goto :eof

:clean
echo [INFO] Cleaning build artifacts...

if exist "%PROJECT_ROOT%server\echo.exe" del /f "%PROJECT_ROOT%server\echo.exe"
if exist "%PROJECT_ROOT%server\data" rmdir /s /q "%PROJECT_ROOT%server\data"
if exist "%PROJECT_ROOT%web\dist" rmdir /s /q "%PROJECT_ROOT%web\dist"
if exist "%PROJECT_ROOT%server\internal\web\dist" rmdir /s /q "%PROJECT_ROOT%server\internal\web\dist"

echo [OK] Clean complete!
goto :eof
