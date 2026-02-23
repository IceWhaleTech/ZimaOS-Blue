$ErrorActionPreference = "Stop"

# Function to ensure PATH is correct
function Ensure-Path {
    # Keep original PATH and prepend our required paths
    $requiredPaths = @(
        "C:\Users\Administrator\AppData\Local\nvm\v20.20.0",
        "C:\Users\Administrator\.cargo\bin",
        "C:\Program Files\Go\bin"
    )

    # Only add paths that aren't already in PATH
    $currentPath = $env:Path
    foreach ($path in $requiredPaths) {
        if ($currentPath -notlike "*$path*") {
            $currentPath = "$path;$currentPath"
        }
    }
    $env:Path = $currentPath
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
    Write-Host "[WARN] Production dependencies have vulnerabilities, but continuing build..." -ForegroundColor Yellow
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

# Step 1.6: Copy canonical skills to server/internal/skill/embedded/skills/ for go:embed
Write-Host "[STEP 1.6] Copying skills for go:embed..."
$skillsSrc = "g:\GitHub\ZimaOS-Blue\assets\skills"
$skillsEmbed = "g:\GitHub\ZimaOS-Blue\server\internal\skill\embedded\skills"
if (Test-Path $skillsEmbed) { Remove-Item -Recurse -Force $skillsEmbed }
New-Item -ItemType Directory $skillsEmbed -Force | Out-Null
Copy-Item -Recurse -Force "$skillsSrc\*" $skillsEmbed
Write-Host "[OK] Skills copied to $skillsEmbed ($(((Get-ChildItem -Directory $skillsEmbed).Count)) skills)"

# Step 2: Build Go library (c-archive, whisper via FFI)
Write-Host "[STEP 2] Building Go library..."
Set-Location "g:\GitHub\ZimaOS-Blue\server"
$tauriDir = "g:\GitHub\ZimaOS-Blue\tauri-app\src-tauri"
if (!(Test-Path "$tauriDir\lib")) { New-Item -ItemType Directory "$tauriDir\lib" -Force | Out-Null }

# Setup CGO with MinGW-w64 and static linking
Write-Host "[STEP 2.1] Setting up CGO with MinGW-w64 (static linking)..."

# Use MinGW-w64 with static linking to avoid libstdc++ runtime dependency
# This statically links libstdc++, libgcc, and winpthread into the binary
$env:CGO_ENABLED = "1"
$env:CC = "gcc"
$env:CXX = "g++"
# Use explicit static linking for C++ runtime libraries
# Note: We link libstdc++ statically but keep system libraries dynamic
$env:CGO_LDFLAGS = "-static-libgcc -static-libstdc++"
$env:CGO_CFLAGS = "-O2"
$env:CGO_CXXFLAGS = "-O2"

Write-Host "[OK] CGO configured with static linking"
Write-Host "[INFO] This will statically link libstdc++ to avoid runtime dependencies"
Write-Host "[INFO] CGO will automatically compile C++ files in speech/windows directory"

$goLdflags = "-s -w"
if ($env:ZIMAOS_TRIAL_LICENSE) {
    $goLdflags += " -X github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool.trialLicense=$($env:ZIMAOS_TRIAL_LICENSE)"
}
# Build without espeak, kokoro, and whisper tags (Windows native only)
Write-Host "[STEP 2.2] Running Go build with MSVC..."
Write-Host "[DEBUG] CC=$env:CC"
Write-Host "[DEBUG] CXX=$env:CXX"
Write-Host "[DEBUG] CGO_CFLAGS=$env:CGO_CFLAGS"
go build -v -buildmode=c-archive -ldflags="$goLdflags" -o "$tauriDir\lib\libblue.a" ./cmd/bluelib/
if ($LASTEXITCODE -ne 0) {
    Write-Host "[ERROR] Go build failed with exit code $LASTEXITCODE" -ForegroundColor Red
    throw "Go build failed"
}
Write-Host "[OK] libblue.a built (Windows native TTS/ASR only, MSVC)"

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

        # Check if blue.exe has dynamic dependencies on MinGW DLLs
        # If static linking failed, copy required DLLs as fallback
        $depsCheck = & objdump -p "$filesDir\blue.exe" 2>&1 | Select-String "libstdc\+\+|libgcc|libwinpthread"
        if ($depsCheck) {
            Write-Host "[WARN] Detected dynamic MinGW dependencies, copying runtime DLLs as fallback..."
            # Try multiple possible MinGW locations
            $mingwLocations = @("C:\mingw64\bin", "C:\msys64\mingw64\bin")
            $mingwBin = $null
            foreach ($loc in $mingwLocations) {
                if (Test-Path $loc) {
                    $mingwBin = $loc
                    Write-Host "[OK] Found MinGW at $mingwBin"
                    break
                }
            }
            if ($mingwBin) {
                $requiredDlls = @("libstdc++-6.dll", "libgcc_s_seh-1.dll", "libwinpthread-1.dll")
                foreach ($dll in $requiredDlls) {
                    $dllPath = Join-Path $mingwBin $dll
                    if (Test-Path $dllPath) {
                        Copy-Item -Force $dllPath $filesDir
                        Write-Host "[OK] Copied $dll"
                    }
                }
            } else {
                Write-Host "[WARN] MinGW bin directory not found in any known location"
            }
        } else {
            Write-Host "[OK] blue.exe is statically linked (no MinGW DLL dependencies)"
        }
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

# Copy canonical skills to FilesToInstall/.claude/skills/ for NSIS packaging
$skillsSrc = "g:\GitHub\ZimaOS-Blue\assets\skills"
$skillsDest = "$filesDir\.claude\skills"
if (Test-Path $skillsSrc) {
    if (Test-Path $skillsDest) { Remove-Item -Recurse -Force $skillsDest }
    New-Item -ItemType Directory $skillsDest -Force | Out-Null
    Copy-Item -Recurse -Force "$skillsSrc\*" $skillsDest
    Write-Host "[OK] Skills copied to FilesToInstall\.claude\skills\ ($(((Get-ChildItem -Directory $skillsDest).Count)) skills)"
} else {
    Write-Host "[WARN] Skills not found at $skillsSrc"
}

# Clean up build artifacts from FilesToInstall
Write-Host "[STEP 6] Cleaning build artifacts..."
Remove-Item -Force "$filesDir\dist\stats.html" -ErrorAction SilentlyContinue
Get-ChildItem -Recurse -Path "$filesDir\dist" -Filter "*.gz" | Remove-Item -Force
Get-ChildItem -Recurse -Path "$filesDir\dist" -Filter "*.br" | Remove-Item -Force
Get-ChildItem -Recurse -Path "$filesDir\dist" -Filter "*.map" | Remove-Item -Force
Write-Host "[OK] Build artifacts cleaned"

# Step 6.1: Sign executables before packaging (optional)
Write-Host "[STEP 6.1] Signing executables..."
try {
    & $signtool sign /tr http://timestamp.digicert.com /td sha256 /fd sha256 /a "$filesDir\blue.exe" 2>&1 | Out-Null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "[OK] blue.exe signed"
    } else {
        Write-Host "[WARN] Failed to sign blue.exe (no certificate or signing failed)"
    }
} catch {
    Write-Host "[WARN] Signing skipped (signtool not available or no certificate)"
}

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

# Copy icon and license files for NSIS
Copy-Item -Force "..\icons\icon.ico" "SetupScripts\zimaos\logo.ico"
Copy-Item -Force "g:\GitHub\ZimaOS-Blue\LICENSE" "SetupScripts\zimaos\license.txt"

# NSIS
if (!(Test-Path "Output")) { New-Item -ItemType Directory "Output" -Force | Out-Null }
& .\NSIS\makensis.exe "SetupScripts\zimaos\zimaos_setup.nsi"
if ($LASTEXITCODE -ne 0) { throw "NSIS build failed" }

# Clean up temporary files after NSIS build
Remove-Item -Force "SetupScripts\zimaos\logo.ico" -ErrorAction SilentlyContinue
Remove-Item -Force "SetupScripts\zimaos\license.txt" -ErrorAction SilentlyContinue
Remove-Item -Force "SetupScripts\zimaos\skin.zip" -ErrorAction SilentlyContinue
Remove-Item -Force "SetupScripts\app.7z" -ErrorAction SilentlyContinue
Write-Host "[OK] NSIS installer built and temporary files cleaned"

# Step 8: Sign the final installer (optional)
Write-Host "[STEP 8] Signing installer..."
$signed = $false
# Output directory is now at nsis/Output (one level up from skin-installer)
$outputDir = "$tauriDir\nsis\Output"
Get-ChildItem "$outputDir\ZimaOS-*_*.exe" | ForEach-Object {
    try {
        & $signtool sign /tr http://timestamp.digicert.com /td sha256 /fd sha256 /a $_.FullName 2>&1 | Out-Null
        if ($LASTEXITCODE -eq 0) {
            Write-Host "[OK] $($_.Name) signed"
            $signed = $true
        } else {
            Write-Host "[WARN] Failed to sign $($_.Name) (no certificate or signing failed)"
        }
    } catch {
        Write-Host "[WARN] Signing skipped for $($_.Name) (signtool not available or no certificate)"
    }
}
if (-not $signed) {
    Write-Host "[WARN] No installers were signed - code signing certificate not found"
}

# Clean up embedded skills (build artifact)
$skillsEmbed = "g:\GitHub\ZimaOS-Blue\server\internal\skill\embedded\skills"
if (Test-Path $skillsEmbed) {
    Remove-Item -Recurse -Force $skillsEmbed
    Write-Host "[OK] Cleaned server\internal\skill\embedded\skills (build artifact)"
}

Write-Host ""
Write-Host "=========================================="
Write-Host "Build Complete!"
Write-Host "=========================================="
Get-ChildItem "$outputDir\ZimaOS-*_*.exe" | ForEach-Object { Write-Host "Output: $($_.FullName) ($([math]::Round($_.Length/1MB, 1)) MB)" }
