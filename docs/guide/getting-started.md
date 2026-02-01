# Getting Started

This guide will help you get ZimaOS Echo up and running quickly.

## Prerequisites

- Linux, macOS, or Windows
- 512MB RAM minimum (1GB recommended)
- 100MB disk space

## Quick Installation

Build and run from source:

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo server
```

See [Installation](installation.md) for binary download and other options.

## Verify Installation

After installation, verify the service is running:

```bash
# Linux
systemctl status zimaos-echo

# Windows
Get-Service ZimaOS-Echo
```

Access the dashboard at `http://localhost:23456`

## Health Check

```bash
curl http://localhost:23456/health
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
