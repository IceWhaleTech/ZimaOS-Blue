$ErrorActionPreference = "Stop"
$env:Path = "C:\mingw64\bin;C:\cmake-3.31.4-windows-x86_64\bin;C:\Users\Administrator\AppData\Local\nvm\v20.20.0;C:\Users\Administrator\.cargo\bin;C:\Program Files\Go\bin;" + $env:Path

$tauriDir = "g:\GitHub\ZimaOS-Blue\tauri-app\src-tauri"
$skinDir = "$tauriDir\nsis\skin-installer"

# Step 3: Build Go sidecar (CGO enabled with MinGW)
Write-Host "[STEP 3] Building Go sidecar (CGO enabled)..."
Set-Location "g:\GitHub\ZimaOS-Blue\server"
$sidecarName = "blue-server-x86_64-pc-windows-msvc.exe"
if (!(Test-Path "$tauriDir\binaries")) { New-Item -ItemType Directory "$tauriDir\binaries" -Force | Out-Null }
if (!(Test-Path "$tauriDir\bin")) { New-Item -ItemType Directory "$tauriDir\bin" -Force | Out-Null }
$env:CGO_ENABLED = "1"
$env:CC = "gcc"
$env:CXX = "g++"
# Build ldflags with trial provider config (from environment)
$goLdflags = "-s -w"
if ($env:ZIMAOS_TRIAL_API_KEY) {
    $goLdflags += " -X github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool.trialAPIKey=$($env:ZIMAOS_TRIAL_API_KEY)"
}
if ($env:ZIMAOS_TRIAL_BASE_URL) {
    $goLdflags += " -X github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool.trialBaseURL=$($env:ZIMAOS_TRIAL_BASE_URL)"
}
go build -ldflags="$goLdflags" -o "$tauriDir\binaries\$sidecarName" ./cmd/blue/
if ($LASTEXITCODE -ne 0) { throw "Go build failed" }
Copy-Item -Force "$tauriDir\binaries\$sidecarName" "$tauriDir\bin\"
Write-Host "[OK] Sidecar built: $sidecarName"

# Step 4: Clean data dir
if (Test-Path "$tauriDir\data") { Remove-Item -Recurse -Force "$tauriDir\data" }
New-Item -ItemType Directory "$tauriDir\data" -Force | Out-Null

# Step 5: Build Tauri (Rust GNU toolchain uses MinGW)
Write-Host "[STEP 5] Building Tauri application..."
Set-Location "g:\GitHub\ZimaOS-Blue\tauri-app"
npm install
if ($LASTEXITCODE -ne 0) { throw "tauri npm install failed" }
npx tauri build --no-bundle
if ($LASTEXITCODE -ne 0) { throw "tauri build failed" }
Write-Host "[OK] Tauri build complete (no-bundle, using custom NSIS skin installer)"

# Step 6: Copy to FilesToInstall
Write-Host "[STEP 6] Preparing NSIS files..."
$filesDir = "$skinDir\FilesToInstall"
$signtool = "C:\Program Files (x86)\Windows Kits\10\bin\10.0.19041.0\x64\signtool.exe"
$releaseDirs = @("$tauriDir\target\release", "$tauriDir\target\x86_64-pc-windows-gnu\release", "$tauriDir\target\x86_64-pc-windows-msvc\release")
foreach ($rd in $releaseDirs) {
    $exeName = if (Test-Path "$rd\blue.exe") { "blue.exe" } elseif (Test-Path "$rd\zimaos-blue.exe") { "zimaos-blue.exe" } else { $null }
    if ($exeName) {
        Copy-Item -Force "$rd\$exeName" "$filesDir\zimaos-blue.exe"
        if (Test-Path "$rd\WebView2Loader.dll") { Copy-Item -Force "$rd\WebView2Loader.dll" $filesDir }
        Copy-Item -Force "$tauriDir\bin\$sidecarName" "$filesDir\blue-server.exe"
        Write-Host "[OK] Files copied from $rd"
        break
    }
}

# Step 6.5: Sign executables before packaging
Write-Host "[STEP 6.5] Signing executables..."
& $signtool sign /tr http://timestamp.digicert.com /td sha256 /fd sha256 /a "$filesDir\zimaos-blue.exe"
if ($LASTEXITCODE -ne 0) { throw "Failed to sign zimaos-blue.exe" }
& $signtool sign /tr http://timestamp.digicert.com /td sha256 /fd sha256 /a "$filesDir\blue-server.exe"
if ($LASTEXITCODE -ne 0) { throw "Failed to sign blue-server.exe" }
Write-Host "[OK] Executables signed"

# Step 7: Build NSIS
Write-Host "[STEP 7] Building NSIS installer..."
Set-Location $skinDir
if (Test-Path "SetupScripts\app.7z") { Remove-Item "SetupScripts\app.7z" }
& .\7z.exe a "SetupScripts\app.7z" "$filesDir\*.*"
Push-Location "SetupScripts\zimaos\skin"
if (Test-Path "..\skin.zip") { Remove-Item "..\skin.zip" }
& ..\..\..\7z.exe a "..\skin.zip" ".\*" -r
Pop-Location
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

Write-Host "`n=========================================="
Write-Host "Build Complete!"
Write-Host "=========================================="
Get-ChildItem "Output\ZimaOS-*_*.exe" | ForEach-Object { Write-Host "Output: $($_.FullName) ($([math]::Round($_.Length/1MB, 1)) MB)" }
