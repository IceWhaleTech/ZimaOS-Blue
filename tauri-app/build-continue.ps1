$ErrorActionPreference = "Stop"
Write-Host "=== ZimaOS Blue - Continue Build (skip frontend) with Signing ==="
& "$PSScriptRoot\src-tauri\nsis\build-continue.ps1"
