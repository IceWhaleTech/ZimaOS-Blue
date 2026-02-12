# ZimaOS-Blue CLI Tutorial

ZimaOS-Blue provides a comprehensive command-line interface (CLI) for managing and interacting with the NAS-Native Agent Runtime.

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Global Flags](#global-flags)
- [Commands Reference](#commands-reference)
  - [Service Management](#service-management)
  - [Configuration](#configuration)
  - [Models](#models)
  - [Sessions](#sessions)
  - [Cron Jobs](#cron-jobs)
  - [Plugins](#plugins)
  - [Skills](#skills)
  - [Logs](#logs)
- [Examples](#examples)
- [Troubleshooting](#troubleshooting)

---

## Installation

### From Binary

Download the latest release for your platform:

```bash
# Windows
curl -LO https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download/echo-windows-amd64.exe
mv echo-windows-amd64.exe echo.exe

# Linux
curl -LO https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download/echo-linux-amd64
chmod +x echo-linux-amd64
sudo mv echo-linux-amd64 /usr/local/bin/echo

# macOS
curl -LO https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download/echo-darwin-amd64
chmod +x echo-darwin-amd64
sudo mv echo-darwin-amd64 /usr/local/bin/echo
```

### From Source

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue/server
go build -o echo ./cmd/blue/
```

---

## Quick Start

### 1. Check System Health

```bash
# Run diagnostic checks
echo doctor

# Auto-fix common issues
echo doctor --fix
```

### 2. Start the Service

```bash
# Run in foreground (for testing)
echo gateway run

# Or install as system service
echo gateway install
echo gateway start
```

### 3. Check Status

```bash
# Quick status check
echo status

# Detailed status with all info
echo status --all
```

### 4. View Logs

```bash
# Show recent logs
echo logs

# Follow logs in real-time
echo logs -f
```

---

## Global Flags

These flags can be used with any command:

| Flag | Description |
|------|-------------|
| `--config <path>` | Specify config file path |
| `--dev` | Development mode (uses port 8081, isolated state) |
| `--profile <name>` | Use named profile for state isolation |
| `--no-color` | Disable ANSI color output |
| `--json` | Output in JSON format (machine-readable) |
| `-v, --verbose` | Enable verbose output |
| `-h, --help` | Show help for any command |

### Examples

```bash
# Use development mode
echo --dev status

# Use a custom profile
echo --profile testing config list

# Get JSON output for scripting
echo --json models list

# Disable colors for piping
echo --no-color logs | grep error
```

---

## Commands Reference

### Service Management

#### `gateway` - Service Control

Manage the ZimaOS-Blue service.

```bash
# Run service in foreground
echo gateway run [--port 23456] [--bind 0.0.0.0]

# Check service status
echo gateway status

# Start service (background)
echo gateway start

# Stop service
echo gateway stop

# Restart service
echo gateway restart

# Install as system service
echo gateway install

# Uninstall system service
echo gateway uninstall
```

**Options:**
- `--port <port>` - HTTP server port (default: 23456)
- `--bind <address>` - Bind address (default: 0.0.0.0)
- `--verbose` - Enable verbose logging

#### `status` - Service Status

Display service health and recent activity.

```bash
# Basic status
echo status

# Full status with all details
echo status --all

# Deep health check
echo status --deep

# JSON output
echo status --json
```

#### `health` - Health Check

Quick health check of the running service.

```bash
# Basic health check
echo health

# With custom timeout
echo health --timeout 5s

# JSON output
echo health --json
```

#### `doctor` - System Diagnostics

Run diagnostic checks and auto-fix issues.

```bash
# Run all checks
echo doctor

# Auto-fix issues
echo doctor --fix
```

**Checks performed:**
- Configuration directory exists
- Configuration file is valid
- Data directory exists
- Logs directory exists
- Service is running
- Port is available
- Dependencies are installed
- File permissions are correct

---

### Configuration

#### `config` - Configuration Management

Manage ZimaOS-Blue configuration.

```bash
# List all configuration
echo config list

# Get a specific value
echo config get server.port

# Set a value
echo config set server.port 23456

# Remove a value
echo config unset server.debug
```

**Common configuration keys:**
- `server.port` - HTTP server port
- `server.bind` - Bind address
- `server.debug` - Debug mode
- `providers.default` - Default AI provider
- `models.default` - Default model

---

### Models

#### `models` - Model Management

Manage AI models and providers.

```bash
# List available models
echo models list

# List models from specific provider
echo models list --provider openai

# Check model availability
echo models list --check

# Show model status
echo models status

# Set default model
echo models set gpt-4

# Scan for available models
echo models scan
```

**Options:**
- `--provider <name>` - Filter by provider
- `--check` - Verify model availability
- `--json` - JSON output

---

### Sessions

#### `sessions` - Session Management

Manage conversation sessions.

```bash
# List all sessions
echo sessions list

# List only active sessions
echo sessions list --active

# Show session details
echo sessions show <session-id>

# Delete a session
echo sessions delete <session-id>

# Clear all sessions
echo sessions clear
```

**Options:**
- `--active` - Show only active sessions
- `--json` - JSON output

---

### Cron Jobs

#### `cron` - Scheduled Task Management

Manage scheduled cron jobs.

```bash
# List all cron jobs
echo cron list

# Show cron service status
echo cron status

# Add a new cron job
echo cron add --name "Daily Backup" --cron "0 2 * * *" --handler http --payload '{"url":"http://localhost/backup"}'

# Remove a cron job
echo cron rm <job-id>

# Enable/disable a job
echo cron enable <job-id>
echo cron disable <job-id>

# Run a job immediately
echo cron run <job-id>

# View job execution history
echo cron runs <job-id>
```

**Add Options:**
- `--name <name>` - Job name (required)
- `--cron <expr>` - Cron expression (required)
- `--handler <type>` - Handler type: http, command
- `--payload <json>` - Job payload as JSON

**Cron Expression Format:**
```
┌───────────── minute (0 - 59)
│ ┌───────────── hour (0 - 23)
│ │ ┌───────────── day of month (1 - 31)
│ │ │ ┌───────────── month (1 - 12)
│ │ │ │ ┌───────────── day of week (0 - 6) (Sunday = 0)
│ │ │ │ │
* * * * *
```

**Examples:**
- `0 * * * *` - Every hour
- `0 2 * * *` - Daily at 2 AM
- `0 0 * * 0` - Weekly on Sunday
- `*/15 * * * *` - Every 15 minutes

---

### Plugins

#### `plugins` - Plugin Management

Manage ZimaOS-Blue plugins.

```bash
# List all plugins
echo plugins list

# Show plugin details
echo plugins info <plugin-id>

# Enable/disable a plugin
echo plugins enable <plugin-id>
echo plugins disable <plugin-id>

# Run plugin diagnostics
echo plugins doctor
```

**Options:**
- `--json` - JSON output

---

### Skills

#### `skills` - Skill Management

Manage agent skills.

```bash
# List all skills
echo skills list

# List only eligible skills
echo skills list --eligible

# Show skill details
echo skills info <skill-id>

# Check skill availability
echo skills check
```

**Options:**
- `--eligible` - Show only eligible skills
- `--json` - JSON output

---

### Logs

#### `logs` - View Service Logs

View and follow service logs.

```bash
# Show recent logs (last 50 lines)
echo logs

# Show last N lines
echo logs -n 100

# Follow logs in real-time
echo logs -f

# Filter by log level
echo logs --level error

# Combine options
echo logs -f --level warn -n 200
```

**Options:**
- `-f, --follow` - Follow logs in real-time
- `-n, --lines <num>` - Number of lines to show (default: 50)
- `--level <level>` - Filter by level: debug, info, warn, error
- `--json` - JSON output

---

## Examples

### Scripting with JSON Output

```bash
# Get model list as JSON and process with jq
echo --json models list | jq '.models[].name'

# Check if service is healthy
if echo --json health | jq -e '.healthy' > /dev/null; then
    echo "Service is healthy"
else
    echo "Service is unhealthy"
fi

# Get session count
echo --json sessions list | jq '.conversations | length'
```

### Development Workflow

```bash
# Start in dev mode
echo --dev gateway run

# In another terminal, check status
echo --dev status

# View dev logs
echo --dev logs -f
```

### Profile Isolation

```bash
# Create a testing profile
echo --profile testing config set server.port 9090

# Run with testing profile
echo --profile testing gateway run

# Each profile has isolated:
# - Configuration: ~/.zimaos-blue-testing/config.yaml
# - Data: ~/.zimaos-blue-testing/data/
# - Logs: ~/.zimaos-blue-testing/logs/
```

### Automated Health Monitoring

```bash
#!/bin/bash
# health-check.sh

while true; do
    if ! echo health --timeout 5s > /dev/null 2>&1; then
        echo "$(date): Service unhealthy, restarting..."
        echo gateway restart
    fi
    sleep 60
done
```

---

## Troubleshooting

### Service Won't Start

1. **Check if port is in use:**
   ```bash
   echo doctor
   # Look for "Port available" check
   ```

2. **Check logs for errors:**
   ```bash
   echo logs --level error
   ```

3. **Try running in foreground:**
   ```bash
   echo gateway run --verbose
   ```

### Configuration Issues

1. **Verify config file:**
   ```bash
   echo config list
   ```

2. **Reset to defaults:**
   ```bash
   echo config unset <problematic-key>
   ```

3. **Run doctor with fix:**
   ```bash
   echo doctor --fix
   ```

### Connection Refused

1. **Check if service is running:**
   ```bash
   echo gateway status
   ```

2. **Verify port configuration:**
   ```bash
   echo config get server.port
   ```

3. **Check firewall settings** (platform-specific)

### Permission Errors

1. **Run doctor to check permissions:**
   ```bash
   echo doctor
   ```

2. **Fix permissions automatically:**
   ```bash
   echo doctor --fix
   ```

### Log File Not Found

The CLI looks for logs in these locations:
1. `~/.zimaos-blue/logs/echo.log`
2. `./logs/echo.log`
3. `./echo.log`

Ensure the service has been started at least once to create log files.

---

## Getting Help

```bash
# General help
echo --help

# Command-specific help
echo gateway --help
echo config --help
echo models --help
```

For more information, visit:
- GitHub: https://github.com/IceWhaleTech/ZimaOS-Blue
- Documentation: https://docs.zimaos.com/echo
