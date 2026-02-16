$ErrorActionPreference = "Stop"
Write-Host "=== ZimaOS Blue - Full Build with Signing ==="
& "$PSScriptRoot\src-tauri\nsis\build-all.ps1"
