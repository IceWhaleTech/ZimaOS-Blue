#Requires -RunAsAdministrator
<#
.SYNOPSIS
    ZimaOS-Echo One-Click Installation Script for Windows
.DESCRIPTION
    Downloads and installs ZimaOS-Echo on Windows systems
.PARAMETER Version
    Version to install (default: latest)
.PARAMETER InstallDir
    Installation directory (default: C:\Program Files\ZimaOS-Echo)
.EXAMPLE
    irm https://echo.zimaos.com/install.ps1 | iex
.EXAMPLE
    .\install.ps1 -Version v0.1.0
#>

param(
    [string]$Version = "latest",
    [string]$InstallDir = "$env:ProgramFiles\ZimaOS-Echo"
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$GitHubRepo = "zimaos/echo"
$BaseUrl = "https://github.com/$GitHubRepo/releases"
$ServiceName = "ZimaOS-Echo"

function Write-Banner {
    Write-Host ""
    Write-Host "╔═══════════════════════════════════════════╗" -ForegroundColor Cyan
    Write-Host "║         ZimaOS-Echo Installer             ║" -ForegroundColor Cyan
    Write-Host "║     NAS-Native Agent Runtime              ║" -ForegroundColor Cyan
    Write-Host "╚═══════════════════════════════════════════╝" -ForegroundColor Cyan
    Write-Host ""
}

function Write-Info {
    param([string]$Message)
    Write-Host "[INFO] " -ForegroundColor Green -NoNewline
    Write-Host $Message
}

function Write-Warn {
    param([string]$Message)
    Write-Host "[WARN] " -ForegroundColor Yellow -NoNewline
    Write-Host $Message
}

function Write-Error {
    param([string]$Message)
    Write-Host "[ERROR] " -ForegroundColor Red -NoNewline
    Write-Host $Message
}

function Test-Administrator {
    $currentUser = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($currentUser)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-Architecture {
    $arch = [System.Environment]::GetEnvironmentVariable("PROCESSOR_ARCHITECTURE")
    switch ($arch) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        default {
            throw "Unsupported architecture: $arch"
        }
    }
}

function Get-LatestVersion {
    if ($Version -eq "latest") {
        try {
            $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$GitHubRepo/releases/latest"
            $script:Version = $release.tag_name
        } catch {
            $script:Version = "v0.1.0"
            Write-Warn "Could not fetch latest version, using $Version"
        }
    }
    Write-Info "Installing version: $Version"
}

function Install-Binary {
    $arch = Get-Architecture
    $binaryName = "echo-windows-$arch.zip"
    $downloadUrl = "$BaseUrl/download/$Version/$binaryName"

    Write-Info "Downloading from: $downloadUrl"

    # Create directories
    $binDir = Join-Path $InstallDir "bin"
    $configDir = Join-Path $InstallDir "config"
    $dataDir = Join-Path $InstallDir "data"
    $logsDir = Join-Path $InstallDir "logs"

    New-Item -ItemType Directory -Force -Path $binDir, $configDir, $dataDir, $logsDir | Out-Null

    # Download and extract
    $tempFile = Join-Path $env:TEMP "zimaos-echo.zip"
    Invoke-WebRequest -Uri $downloadUrl -OutFile $tempFile
    Expand-Archive -Path $tempFile -DestinationPath $binDir -Force
    Remove-Item $tempFile -Force

    Write-Info "Installed to $InstallDir"
}

function New-DefaultConfig {
    $configPath = Join-Path $InstallDir "config\config.yaml"

    if (-not (Test-Path $configPath)) {
        Write-Info "Creating default configuration..."

        $config = @"
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: "30s"
  write_timeout: "30s"
  idle_timeout: "120s"

log:
  level: "info"
  format: "json"
  output: "stdout"

worker:
  pool_size: 10
  max_queue_len: 100
"@
        Set-Content -Path $configPath -Value $config
    }
}

function Install-WindowsService {
    Write-Info "Setting up Windows service..."

    $binaryPath = Join-Path $InstallDir "bin\echo.exe"
    $configPath = Join-Path $InstallDir "config\config.yaml"

    # Check if service exists
    $service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue

    if ($service) {
        Write-Info "Stopping existing service..."
        Stop-Service -Name $ServiceName -Force
        sc.exe delete $ServiceName | Out-Null
        Start-Sleep -Seconds 2
    }

    # Create service using sc.exe
    $binPathEscaped = "`"$binaryPath`" --config `"$configPath`""
    sc.exe create $ServiceName binPath= $binPathEscaped start= auto DisplayName= "ZimaOS Echo" | Out-Null
    sc.exe description $ServiceName "ZimaOS Echo - NAS-Native Agent Runtime" | Out-Null

    # Configure recovery options
    sc.exe failure $ServiceName reset= 86400 actions= restart/5000/restart/10000/restart/30000 | Out-Null

    Write-Info "Starting service..."
    Start-Service -Name $ServiceName

    Start-Sleep -Seconds 2
    $service = Get-Service -Name $ServiceName
    if ($service.Status -eq "Running") {
        Write-Info "Service started successfully!"
    } else {
        Write-Warn "Service may not have started. Check Event Viewer for details."
    }
}

function Add-FirewallRule {
    Write-Info "Adding firewall rule..."

    $ruleName = "ZimaOS-Echo"
    $existingRule = Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue

    if (-not $existingRule) {
        New-NetFirewallRule -DisplayName $ruleName `
            -Direction Inbound `
            -Protocol TCP `
            -LocalPort 8080 `
            -Action Allow `
            -Profile Any | Out-Null
    }
}

function Write-Success {
    $ip = (Get-NetIPAddress -AddressFamily IPv4 | Where-Object { $_.InterfaceAlias -notlike "*Loopback*" } | Select-Object -First 1).IPAddress
    if (-not $ip) { $ip = "localhost" }

    Write-Host ""
    Write-Host "╔═══════════════════════════════════════════╗" -ForegroundColor Green
    Write-Host "║       Installation Complete!              ║" -ForegroundColor Green
    Write-Host "╚═══════════════════════════════════════════╝" -ForegroundColor Green
    Write-Host ""
    Write-Host "  Dashboard: http://${ip}:8080"
    Write-Host "  Health:    http://${ip}:8080/health"
    Write-Host ""
    Write-Host "  Commands:"
    Write-Host "    Get-Service ZimaOS-Echo              - Check status"
    Write-Host "    Restart-Service ZimaOS-Echo          - Restart"
    Write-Host "    Get-EventLog -LogName Application    - View logs"
    Write-Host ""
    Write-Host "  Config: $InstallDir\config\config.yaml"
    Write-Host ""
}

function Main {
    Write-Banner

    if (-not (Test-Administrator)) {
        Write-Error "Please run as Administrator"
        Write-Host "  Right-click PowerShell and select 'Run as Administrator'"
        exit 1
    }

    Get-LatestVersion
    Install-Binary
    New-DefaultConfig
    Install-WindowsService
    Add-FirewallRule
    Write-Success
}

Main
