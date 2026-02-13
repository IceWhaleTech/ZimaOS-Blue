# ZimaOS-Blue CLI Tutorial

ZimaOS-Blue provides a comprehensive command-line interface (CLI) for managing and interacting with the A Local-first Agent Runtime for Builders with Bolder Mind.

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
curl -LO https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download/blue-windows-amd64.exe
mv blue-windows-amd64.exe blue.exe

# Linux
curl -LO https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download/blue-linux-amd64
chmod +x blue-linux-amd64
sudo mv blue-linux-amd64 /usr/local/bin/blue

# macOS
curl -LO https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download/blue-darwin-amd64
chmod +x blue-darwin-amd64
sudo mv blue-darwin-amd64 /usr/local/bin/blue
```

### From Source

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue/server
go build -o blue ./cmd/blue/
```

---

## Quick Start

### 1. Check System Health

```bash
# Run diagnostic checks
blue doctor

# Auto-fix common issues
blue doctor --fix
```

### 2. Start the Service

```bash
# Run in foreground (for testing)
blue gateway run

# Or install as system service
blue gateway install
blue gateway start
```

### 3. Check Status

```bash
# Quick status check
blue status

# Detailed status with all info
blue status --all
```

### 4. View Logs

```bash
# Show recent logs
blue logs

# Follow logs in real-time
blue logs -f
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
blue --dev status

# Use a custom profile
blue --profile testing config list

# Get JSON output for scripting
blue --json models list

# Disable colors for piping
blue --no-color logs | grep error
```

---

## Commands Reference

### Service Management

#### `gateway` - Service Control

Manage the ZimaOS-Blue service.

```bash
# Run service in foreground
blue gateway run [--port 23456] [--bind 0.0.0.0]

# Check service status
blue gateway status

# Start service (background)
blue gateway start

# Stop service
blue gateway stop

# Restart service
blue gateway restart

# Install as system service
blue gateway install

# Uninstall system service
blue gateway uninstall
```

**Options:**
- `--port <port>` - HTTP server port (default: 23456)
- `--bind <address>` - Bind address (default: 0.0.0.0)
- `--verbose` - Enable verbose logging

#### `status` - Service Status

Display service health and recent activity.

```bash
# Basic status
blue status

# Full status with all details
blue status --all

# Deep health check
blue status --deep

# JSON output
blue status --json
```

#### `health` - Health Check

Quick health check of the running service.

```bash
# Basic health check
blue health

# With custom timeout
blue health --timeout 5s

# JSON output
blue health --json
```

#### `doctor` - System Diagnostics

Run diagnostic checks and auto-fix issues.

```bash
# Run all checks
blue doctor

# Auto-fix issues
blue doctor --fix
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
blue config list

# Get a specific value
blue config get server.port

# Set a value
blue config set server.port 23456

# Remove a value
blue config unset server.debug
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
blue models list

# List models from specific provider
blue models list --provider openai

# Check model availability
blue models list --check

# Show model status
blue models status

# Set default model
blue models set gpt-4

# Scan for available models
blue models scan
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
blue sessions list

# List only active sessions
blue sessions list --active

# Show session details
blue sessions show <session-id>

# Delete a session
blue sessions delete <session-id>

# Clear all sessions
blue sessions clear
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
blue cron list

# Show cron service status
blue cron status

# Add a new cron job
blue cron add --name "Daily Backup" --cron "0 2 * * *" --handler http --payload '{"url":"http://localhost/backup"}'

# Remove a cron job
blue cron rm <job-id>

# Enable/disable a job
blue cron enable <job-id>
blue cron disable <job-id>

# Run a job immediately
blue cron run <job-id>

# View job execution history
blue cron runs <job-id>
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
blue plugins list

# Show plugin details
blue plugins info <plugin-id>

# Enable/disable a plugin
blue plugins enable <plugin-id>
blue plugins disable <plugin-id>

# Run plugin diagnostics
blue plugins doctor
```

**Options:**
- `--json` - JSON output

---

### Skills

#### `skills` - Skill Management

Manage agent skills.

```bash
# List all skills
blue skills list

# List only eligible skills
blue skills list --eligible

# Show skill details
blue skills info <skill-id>

# Check skill availability
blue skills check
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
blue logs

# Show last N lines
blue logs -n 100

# Follow logs in real-time
blue logs -f

# Filter by log level
blue logs --level error

# Combine options
blue logs -f --level warn -n 200
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
blue --json models list | jq '.models[].name'

# Check if service is healthy
if blue --json health | jq -e '.healthy' > /dev/null; then
    echo "Service is healthy"
else
    echo "Service is unhealthy"
fi

# Get session count
blue --json sessions list | jq '.conversations | length'
```

### Development Workflow

```bash
# Start in dev mode
blue --dev gateway run

# In another terminal, check status
blue --dev status

# View dev logs
blue --dev logs -f
```

### Profile Isolation

```bash
# Create a testing profile
blue --profile testing config set server.port 9090

# Run with testing profile
blue --profile testing gateway run

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
    if ! blue health --timeout 5s > /dev/null 2>&1; then
        echo "$(date): Service unhealthy, restarting..."
        blue gateway restart
    fi
    sleep 60
done
```

---

## Troubleshooting

### Service Won't Start

1. **Check if port is in use:**
   ```bash
   blue doctor
   # Look for "Port available" check
   ```

2. **Check logs for errors:**
   ```bash
   blue logs --level error
   ```

3. **Try running in foreground:**
   ```bash
   blue gateway run --verbose
   ```

### Configuration Issues

1. **Verify config file:**
   ```bash
   blue config list
   ```

2. **Reset to defaults:**
   ```bash
   blue config unset <problematic-key>
   ```

3. **Run doctor with fix:**
   ```bash
   blue doctor --fix
   ```

### Connection Refused

1. **Check if service is running:**
   ```bash
   blue gateway status
   ```

2. **Verify port configuration:**
   ```bash
   blue config get server.port
   ```

3. **Check firewall settings** (platform-specific)

### Permission Errors

1. **Run doctor to check permissions:**
   ```bash
   blue doctor
   ```

2. **Fix permissions automatically:**
   ```bash
   blue doctor --fix
   ```

### Log File Not Found

The CLI looks for logs in these locations:
1. `~/.zimaos-blue/logs/blue.log`
2. `./logs/blue.log`
3. `./blue.log`

Ensure the service has been started at least once to create log files.

---

## Getting Help

```bash
# General help
blue --help

# Command-specific help
blue gateway --help
blue config --help
blue models --help
```

For more information, visit:
- GitHub: https://github.com/IceWhaleTech/ZimaOS-Blue
- Documentation: https://docs.zimaos.com/blue
