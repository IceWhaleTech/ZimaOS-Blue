# Getting Started

This guide will help you get ZimaOS Echo up and running quickly.

## Prerequisites

- Linux, macOS, or Windows
- 512MB RAM minimum (1GB recommended)
- 100MB disk space

## Quick Installation

### Linux / macOS

```bash
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash
```

### Windows

Open PowerShell as Administrator and run:

```powershell
irm https://echo.zimaos.com/install.ps1 | iex
```

## Verify Installation

After installation, verify the service is running:

```bash
# Linux
systemctl status zimaos-echo

# Windows
Get-Service ZimaOS-Echo
```

Access the dashboard at `http://localhost:8080`

## Health Check

```bash
curl http://localhost:8080/health
```

Expected response:

```json
{
  "status": "ok",
  "version": "0.1.0",
  "uptime": "1h30m",
  "goroutines": 10,
  "mem_alloc_bytes": 5242880
}
```

## Next Steps

- [Installation Options](/guide/installation) - Manual installation and Docker
- [Configuration](/guide/configuration) - Customize your setup
- [Architecture](/guide/architecture) - Understand how it works
