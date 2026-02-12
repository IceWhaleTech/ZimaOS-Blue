$ErrorActionPreference = "Stop"
$basePath = "C:\Users\Administrator\AppData\Local\nvm\v20.20.0;C:\Users\Administrator\.cargo\bin;C:\Program Files\Go\bin;" + $env:Path
$env:Path = $basePath

Write-Host "=== Checking tools ==="
Write-Host "Node: $(node --version)"
Write-Host "npm: $(npm --version)"
Write-Host "Go: $(go version)"
Write-Host "Cargo: $(cargo --version)"
Write-Host ""

# Step 1: Build frontend
Write-Host "[STEP 1] Building frontend..."
Set-Location "g:\GitHub\ZimaOS-Echo\web"
if (Test-Path dist) { Remove-Item -Recurse -Force dist }
npm install
if ($LASTEXITCODE -ne 0) { throw "npm install failed" }
npm run build
if ($LASTEXITCODE -ne 0) { throw "npm run build failed" }
Write-Host "[OK] Frontend built"

# Step 2: Copy to server embed dir
Write-Host "[STEP 2] Copying frontend to server/internal/web/dist..."
$embedDir = "g:\GitHub\ZimaOS-Echo\server\internal\web\dist"
if (Test-Path $embedDir) { Remove-Item -Recurse -Force $embedDir }
New-Item -ItemType Directory -Path $embedDir -Force | Out-Null
Copy-Item -Recurse -Force "g:\GitHub\ZimaOS-Echo\web\dist\*" $embedDir
Get-ChildItem -Recurse -Filter "*.map" $embedDir | Remove-Item -Force
Write-Host "[OK] Frontend copied"

# Step 3: Build Go sidecar (needs MinGW in PATH for CGO)
Write-Host "[STEP 3] Building Go sidecar (CGO enabled)..."
Set-Location "g:\GitHub\ZimaOS-Echo\server"
$tauriDir = "g:\GitHub\ZimaOS-Echo\tauri-app\src-tauri"
$sidecarName = "echo-server-x86_64-pc-windows-msvc.exe"
if (!(Test-Path "$tauriDir\binaries")) { New-Item -ItemType Directory "$tauriDir\binaries" -Force | Out-Null }
if (!(Test-Path "$tauriDir\bin")) { New-Item -ItemType Directory "$tauriDir\bin" -Force | Out-Null }
# Temporarily add MinGW to PATH for CGO
$env:Path = "C:\mingw64\bin;" + $basePath
$env:CGO_ENABLED = "1"
$env:CC = "gcc"
$env:CXX = "g++"
go build -ldflags="-s -w" -o "$tauriDir\binaries\$sidecarName" ./cmd/blue/
if ($LASTEXITCODE -ne 0) { throw "Go build failed" }
Copy-Item -Force "$tauriDir\binaries\$sidecarName" "$tauriDir\bin\"
Write-Host "[OK] Sidecar built: $sidecarName"
# Restore PATH without MinGW (avoid link.exe conflict with MSVC)
$env:Path = $basePath
Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue
Remove-Item Env:\CC -ErrorAction SilentlyContinue
Remove-Item Env:\CXX -ErrorAction SilentlyContinue

# Step 4: Clean data dir
Write-Host "[STEP 4] Cleaning data directory..."
if (Test-Path "$tauriDir\data") { Remove-Item -Recurse -Force "$tauriDir\data" }
New-Item -ItemType Directory "$tauriDir\data" -Force | Out-Null

# Step 5: Build Tauri (MSVC toolchain, no MinGW)
Write-Host "[STEP 5] Building Tauri application..."
Set-Location "g:\GitHub\ZimaOS-Echo\tauri-app"
npm install
if ($LASTEXITCODE -ne 0) { throw "tauri npm install failed" }
npm run build
if ($LASTEXITCODE -ne 0) { throw "tauri build failed" }
Write-Host "[OK] Tauri build complete"

# Step 6: Copy to FilesToInstall
Write-Host "[STEP 6] Preparing NSIS files..."
$skinDir = "$tauriDir\nsis\skin-installer"
$filesDir = "$skinDir\FilesToInstall"
$releaseDirs = @("$tauriDir\target\release", "$tauriDir\target\x86_64-pc-windows-msvc\release", "$tauriDir\target\x86_64-pc-windows-gnu\release")
foreach ($rd in $releaseDirs) {
    if (Test-Path "$rd\zimaos-blue.exe") {
        Copy-Item -Force "$rd\zimaos-blue.exe" $filesDir
        Copy-Item -Force "$rd\WebView2Loader.dll" $filesDir
        Copy-Item -Force "$tauriDir\bin\$sidecarName" "$filesDir\echo-server.exe"
        Write-Host "[OK] Files copied from $rd"
        break
    }
}

# Step 7: Build NSIS
Write-Host "[STEP 7] Building NSIS installer..."
Set-Location "$tauriDir\nsis\skin-installer"

# app.7z
if (Test-Path "SetupScripts\app.7z") { Remove-Item "SetupScripts\app.7z" }
& .\7z.exe a "SetupScripts\app.7z" "$filesDir\*.*"

# skin.zip
Push-Location "SetupScripts\zimaos\skin"
if (Test-Path "..\skin.zip") { Remove-Item "..\skin.zip" }
& ..\..\..\7z.exe a "..\skin.zip" ".\*" -r
Pop-Location

# NSIS
if (!(Test-Path "Output")) { New-Item -ItemType Directory "Output" -Force | Out-Null }
& .\NSIS\makensis.exe "SetupScripts\zimaos\zimaos_setup.nsi"
if ($LASTEXITCODE -ne 0) { throw "NSIS build failed" }

Write-Host ""
Write-Host "=========================================="
Write-Host "Build Complete!"
Write-Host "=========================================="
Get-ChildItem "Output\ZimaOS-*_*.exe" | ForEach-Object { Write-Host "Output: $($_.FullName) ($([math]::Round($_.Length/1MB, 1)) MB)" }
