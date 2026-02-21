$ErrorActionPreference = "Stop"

# Function to ensure PATH is correct
function Ensure-Path {
    $basePath = "C:\Users\Administrator\AppData\Local\nvm\v20.20.0;C:\Users\Administrator\.cargo\bin;C:\Program Files\Go\bin;" + $env:Path
    $env:Path = $basePath
}

# Set PATH initially
Ensure-Path

Write-Host "=== Checking tools ==="
Write-Host "Node: $(node --version)"
Write-Host "npm: $(npm --version)"
Write-Host "Go: $(go version)"
Write-Host "Cargo: $(cargo --version)"
Write-Host ""

# Step 0: Clean old builds
Write-Host "[STEP 0] Cleaning old builds..."
$tauriDir = "g:\GitHub\ZimaOS-Blue\tauri-app\src-tauri"
if (Test-Path "$tauriDir\target") {
    Write-Host "Cleaning Rust target directory..."
    Remove-Item -Recurse -Force "$tauriDir\target\release" -ErrorAction SilentlyContinue
    Remove-Item -Recurse -Force "$tauriDir\target\x86_64-pc-windows-msvc\release" -ErrorAction SilentlyContinue
    Remove-Item -Recurse -Force "$tauriDir\target\x86_64-pc-windows-gnu\release" -ErrorAction SilentlyContinue
}
if (Test-Path "$tauriDir\lib\libblue.a") {
    Write-Host "Removing old libblue.a..."
    Remove-Item -Force "$tauriDir\lib\libblue.a"
}
$skinDir = "$tauriDir\nsis\skin-installer"
if (Test-Path "$skinDir\FilesToInstall") {
    Write-Host "Cleaning FilesToInstall directory..."
    Remove-Item -Recurse -Force "$skinDir\FilesToInstall"
    New-Item -ItemType Directory "$skinDir\FilesToInstall" -Force | Out-Null
}
if (Test-Path "$skinDir\Output") {
    Write-Host "Cleaning Output directory..."
    Remove-Item -Recurse -Force "$skinDir\Output"
}
Write-Host "[OK] Old builds cleaned"
Write-Host ""

# Step 1: Build frontend
Write-Host "[STEP 1] Building frontend..."
Set-Location "g:\GitHub\ZimaOS-Blue\web"
if (Test-Path dist) { Remove-Item -Recurse -Force dist }

# Ensure PATH is correct before npm operations
Ensure-Path

# Install all dependencies first
Write-Host "[STEP 1.1] Installing all dependencies..."
npm install
if ($LASTEXITCODE -ne 0) { throw "npm install failed" }

# Check production dependencies for vulnerabilities (after all deps are installed)
Write-Host "[STEP 1.2] Checking production dependencies for vulnerabilities..."
npm audit --omit=dev
if ($LASTEXITCODE -ne 0) {
    Write-Host "[ERROR] Production dependencies have vulnerabilities. Please fix them before building." -ForegroundColor Red
    throw "Production dependencies have vulnerabilities"
}

# Build using npm run build (which uses vite from node_modules/.bin)
Write-Host "[STEP 1.3] Building frontend..."
npm run build
if ($LASTEXITCODE -ne 0) { throw "npm run build failed" }
Write-Host "[OK] Frontend built"

# Step 1.5: Clean up server embed dir (not needed for Tauri)
Write-Host "[STEP 1.5] Cleaning server embed dir..."
$embedDir = "g:\GitHub\ZimaOS-Blue\server\internal\web\dist"
if (Test-Path $embedDir) {
    Remove-Item -Recurse -Force $embedDir
    Write-Host "[OK] Removed $embedDir"
} else {
    Write-Host "[OK] $embedDir does not exist"
}

# Step 2: Build Go library (c-archive, whisper via FFI)
Write-Host "[STEP 2] Building Go library..."
Set-Location "g:\GitHub\ZimaOS-Blue\server"
$tauriDir = "g:\GitHub\ZimaOS-Blue\tauri-app\src-tauri"
if (!(Test-Path "$tauriDir\lib")) { New-Item -ItemType Directory "$tauriDir\lib" -Force | Out-Null }
$env:CGO_ENABLED = "1"
$goLdflags = "-s -w"
if ($env:ZIMAOS_TRIAL_LICENSE) {
    $goLdflags += " -X github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool.trialLicense=$($env:ZIMAOS_TRIAL_LICENSE)"
}
# Build without espeak, kokoro, and whisper tags (Windows native only)
go build -tags "fts5" -buildmode=c-archive -ldflags="$goLdflags" -o "$tauriDir\lib\libblue.a" ./cmd/bluelib/
if ($LASTEXITCODE -ne 0) { throw "Go build failed" }
Write-Host "[OK] libblue.a built (with fts5 support, Windows native TTS/ASR only)"

# Step 3: Build Tauri application
Write-Host "[STEP 3] Building Tauri application..."
Set-Location $tauriDir
cargo build --release
if ($LASTEXITCODE -ne 0) { throw "Cargo build failed" }
Write-Host "[OK] Tauri application built"

# Step 4: Verify executable exists
Write-Host "[STEP 4] Verifying executable..."
$releaseDirs = @("$tauriDir\target\release", "$tauriDir\target\x86_64-pc-windows-msvc\release", "$tauriDir\target\x86_64-pc-windows-gnu\release")
$foundExe = $false
foreach ($rd in $releaseDirs) {
    if (Test-Path "$rd\blue.exe") {
        Write-Host "[OK] Found blue.exe in $rd"
        $foundExe = $true
        break
    } elseif (Test-Path "$rd\zimaos-blue.exe") {
        Write-Host "[OK] Found zimaos-blue.exe in $rd"
        $foundExe = $true
        break
    }
}
if (!$foundExe) {
    throw "Executable not found in any release directory"
}

# Step 5: Copy to FilesToInstall
Write-Host "[STEP 5] Preparing NSIS files..."
$skinDir = "$tauriDir\nsis\skin-installer"
$filesDir = "$skinDir\FilesToInstall"
$signtool = "C:\Program Files (x86)\Windows Kits\10\bin\10.0.19041.0\x64\signtool.exe"
$releaseDirs = @("$tauriDir\target\release", "$tauriDir\target\x86_64-pc-windows-msvc\release", "$tauriDir\target\x86_64-pc-windows-gnu\release")
foreach ($rd in $releaseDirs) {
    $exeName = if (Test-Path "$rd\blue.exe") { "blue.exe" } elseif (Test-Path "$rd\zimaos-blue.exe") { "zimaos-blue.exe" } else { $null }
    if ($exeName) {
        Copy-Item -Force "$rd\$exeName" "$filesDir\blue.exe"
        Copy-Item -Force "$rd\WebView2Loader.dll" $filesDir
        Write-Host "[OK] Files copied from $rd"
        break
    }
}

# Copy frontend dist directory directly from web/dist
$distSrc = "g:\GitHub\ZimaOS-Blue\web\dist"
if (Test-Path $distSrc) {
    Get-ChildItem -Recurse -File $distSrc | ForEach-Object {
        $relativePath = $_.FullName.Substring($distSrc.Length + 1)
        $destPath = Join-Path $filesDir "dist\$relativePath"
        $destDir = Split-Path $destPath -Parent
        if (!(Test-Path $destDir)) { New-Item -ItemType Directory -Path $destDir -Force | Out-Null }
        Copy-Item -Force $_.FullName $destPath
    }
    Write-Host "[OK] Frontend dist copied ($(((Get-ChildItem -Recurse -File "$filesDir\dist").Count)) files)"
} else {
    Write-Host "[WARN] Frontend dist not found at $distSrc"
}

# Clean up build artifacts from FilesToInstall
Write-Host "[STEP 6] Cleaning build artifacts..."
Remove-Item -Force "$filesDir\dist\stats.html" -ErrorAction SilentlyContinue
Get-ChildItem -Recurse -Path "$filesDir\dist" -Filter "*.gz" | Remove-Item -Force
Get-ChildItem -Recurse -Path "$filesDir\dist" -Filter "*.br" | Remove-Item -Force
Get-ChildItem -Recurse -Path "$filesDir\dist" -Filter "*.map" | Remove-Item -Force
Write-Host "[OK] Build artifacts cleaned"

# Step 6.1: Sign executables before packaging
Write-Host "[STEP 6.1] Signing executables..."
& $signtool sign /tr http://timestamp.digicert.com /td sha256 /fd sha256 /a "$filesDir\blue.exe"
if ($LASTEXITCODE -ne 0) { throw "Failed to sign blue.exe" }
Write-Host "[OK] blue.exe signed"

# Note: uninst.exe is 32-bit and may not be compatible with signing tool
if (Test-Path "$filesDir\uninst.exe") {
    Write-Host "[INFO] uninst.exe found (skipping signature - 32-bit compatibility)"
}
Write-Host "[OK] Executables prepared"

# Step 7: Build NSIS
Write-Host "[STEP 7] Building NSIS installer..."
Set-Location "$tauriDir\nsis\skin-installer"

# app.7z (all files recursively)
if (Test-Path "SetupScripts\app.7z") { Remove-Item "SetupScripts\app.7z" }
& .\7z.exe a "SetupScripts\app.7z" "$filesDir\*" -r

# skin.zip
Push-Location "SetupScripts\zimaos\skin"
if (Test-Path "..\skin.zip") { Remove-Item "..\skin.zip" }
& ..\..\..\7z.exe a "..\skin.zip" ".\*" -r
Pop-Location

# NSIS
if (!(Test-Path "Output")) { New-Item -ItemType Directory "Output" -Force | Out-Null }
& .\NSIS\makensis.exe "SetupScripts\zimaos\zimaos_setup.nsi"
if ($LASTEXITCODE -ne 0) { throw "NSIS build failed" }

# Step 8: Sign the final installer
Write-Host "[STEP 8] Signing installer..."
Get-ChildItem "Output\ZimaOS-*_*.exe" | ForEach-Object {
    & $signtool sign /tr http://timestamp.digicert.com /td sha256 /fd sha256 /a $_.FullName
    if ($LASTEXITCODE -ne 0) { throw "Failed to sign $($_.Name)" }
}
Write-Host "[OK] Installer signed"

Write-Host ""
Write-Host "=========================================="
Write-Host "Build Complete!"
Write-Host "=========================================="
Get-ChildItem "Output\ZimaOS-*_*.exe" | ForEach-Object { Write-Host "Output: $($_.FullName) ($([math]::Round($_.Length/1MB, 1)) MB)" }
