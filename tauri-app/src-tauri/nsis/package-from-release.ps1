$ErrorActionPreference = "Stop"
$originalDir = Get-Location

$tauriDir = "g:\GitHub\ZimaOS-Blue\tauri-app\src-tauri"
$skinDir = "$tauriDir\nsis\skin-installer"
$filesDir = "$skinDir\FilesToInstall"
$releaseDir = "$tauriDir\nsis\release"

Write-Host "=== Packaging NSIS installer from release files ==="

# Step 1: Clean and prepare FilesToInstall
Write-Host "[STEP 1] Preparing NSIS files..."
if (Test-Path $filesDir) { Remove-Item -Recurse -Force $filesDir }
New-Item -ItemType Directory -Path $filesDir -Force | Out-Null

# Step 2: Copy files from release directory
Write-Host "[STEP 2] Copying files from release..."
if (!(Test-Path $releaseDir)) { throw "Release directory not found: $releaseDir" }

Copy-Item -Force "$releaseDir\blue.exe" "$filesDir\blue.exe"
Copy-Item -Force "$releaseDir\WebView2Loader.dll" "$filesDir\WebView2Loader.dll"
if (Test-Path "$releaseDir\uninst.exe") {
    Copy-Item -Force "$releaseDir\uninst.exe" "$filesDir\uninst.exe"
    Write-Host "[OK] uninst.exe copied"
}

# Copy dist directory
if (Test-Path "$releaseDir\dist") {
    Copy-Item -Recurse -Force "$releaseDir\dist" "$filesDir\dist"
    Write-Host "[OK] dist directory copied ($(((Get-ChildItem -Recurse -File "$filesDir\dist").Count)) files)"
} else {
    Write-Host "[WARN] dist directory not found in release"
}

Write-Host "[OK] Files copied from release"

# Step 3: Sign executables before packaging
Write-Host "[STEP 3] Signing executables..."
$signtool = "C:\Program Files (x86)\Windows Kits\10\bin\10.0.19041.0\x64\signtool.exe"

Write-Host "Signing blue.exe..."
& $signtool sign /tr http://timestamp.digicert.com /td sha256 /fd sha256 /a "$filesDir\blue.exe"
if ($LASTEXITCODE -ne 0) { throw "Failed to sign blue.exe" }
Write-Host "[OK] blue.exe signed"

if (Test-Path "$filesDir\uninst.exe") {
    Write-Host "Signing uninst.exe..."
    & $signtool sign /tr http://timestamp.digicert.com /td sha256 /fd sha256 /a "$filesDir\uninst.exe"
    if ($LASTEXITCODE -ne 0) { throw "Failed to sign uninst.exe" }
    Write-Host "[OK] uninst.exe signed"
} else {
    Write-Host "[WARN] uninst.exe not found"
}

Write-Host "[OK] All executables signed"

# Step 4: Build NSIS
Write-Host "[STEP 4] Building NSIS installer..."
Set-Location "$skinDir"

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

# Step 5: Sign the final installer
Write-Host "[STEP 5] Signing installer..."
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

# Return to original directory
Set-Location $originalDir
