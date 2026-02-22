# Check if blue.exe has dynamic dependencies on MinGW DLLs
# This script helps verify if static linking was successful

param(
    [string]$ExePath = "target\x86_64-pc-windows-gnu\release\blue.exe"
)

if (!(Test-Path $ExePath)) {
    Write-Host "[ERROR] Executable not found: $ExePath" -ForegroundColor Red
    exit 1
}

Write-Host "Checking dependencies for: $ExePath" -ForegroundColor Cyan
Write-Host ""

# Use objdump to check DLL dependencies
$objdumpOutput = & objdump -p $ExePath 2>&1 | Select-String "DLL Name:"

Write-Host "=== DLL Dependencies ===" -ForegroundColor Yellow
$objdumpOutput | ForEach-Object { Write-Host $_ }
Write-Host ""

# Check for MinGW runtime DLLs
$mingwDlls = $objdumpOutput | Select-String "libstdc\+\+|libgcc|libwinpthread"

if ($mingwDlls) {
    Write-Host "[FAIL] Found MinGW runtime dependencies:" -ForegroundColor Red
    $mingwDlls | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
    Write-Host ""
    Write-Host "Static linking FAILED. The executable requires these DLLs to run." -ForegroundColor Red
    Write-Host "These DLLs will be automatically copied during NSIS packaging." -ForegroundColor Yellow
    exit 1
} else {
    Write-Host "[PASS] No MinGW runtime dependencies found!" -ForegroundColor Green
    Write-Host "Static linking SUCCESSFUL. The executable is self-contained." -ForegroundColor Green
    exit 0
}
