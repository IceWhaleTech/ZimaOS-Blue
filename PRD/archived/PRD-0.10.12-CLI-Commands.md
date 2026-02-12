# PRD: CLI Commands Implementation v0.10.12

## Overview

**Version:** 0.10.12
**Date:** 2026-02-01
**Status:** Draft
**Reference:** ClawdBot CLI Documentation (`clawdbot/docs/cli/index.md`)

## Objective

Following ClawdBot's documentation and code, implement equivalent CLI parameter functionality for ZimaOS-Blue to provide complete CLI management capabilities.

## Current State

ZimaOS-Blue current CLI functionality (`server/cmd/blue/main.go`):
- `--config <path>` - Configuration file path
- `--version` - Display version information
- `--help` - Display help information
- Windows service commands: `install`, `uninstall`, `start`, `stop`, `status`

## Target State

Implement ClawdBot core CLI commands in phases by priority.

---

## Phase 1: Core Commands (P0 - Must Have)

### 1.1 Global Flags

- [x] `--dev` - Isolate state to `~/.zimaos-blue-dev`, use different port
- [x] `--profile <name>` - Isolate state to `~/.zimaos-blue-<name>`
- [x] `--no-color` - Disable ANSI color output
- [x] `-V`, `--version`, `-v` - Print version and exit (existing, needs unification)

### 1.2 Status & Health Commands

- [x] `status` - Display service health status and recent activity
  - [x] `--json` - JSON format output
  - [x] `--all` - Show all details
  - [x] `--deep` - Deep check
  - [x] `--verbose` - Verbose output
- [x] `health` - Get health status of running service
  - [x] `--json` - JSON format output
  - [x] `--timeout` - Timeout duration
  - [x] `--verbose` - Verbose output

### 1.3 Configuration Commands

- [x] `config` - Configuration management
  - [x] `config get <key>` - Get configuration value
  - [x] `config set <key> <value>` - Set configuration value
  - [x] `config unset <key>` - Delete configuration value
  - [x] `config list` - List all configurations
- [x] `doctor` - Health check and quick fix
  - [x] `--fix` - Auto-fix issues

---

## Phase 2: Service Management (P1 - Should Have)

### 2.1 Gateway/Server Commands

- [x] `gateway` (or `server`) - Service control
  - [x] `gateway run` - Run service in foreground
  - [x] `gateway status` - Service status
  - [x] `gateway start` - Start service (background)
  - [x] `gateway stop` - Stop service
  - [x] `gateway restart` - Restart service
  - [x] `gateway install` - Install as system service
  - [x] `gateway uninstall` - Uninstall system service
  - [ ] Options:
    - [x] `--port <port>` - Specify port
    - [x] `--bind <address>` - Bind address
    - [x] `--verbose` - Verbose logging

### 2.2 Logs Command

- [x] `logs` - View service logs
  - [x] `--follow`, `-f` - Follow logs in real-time
  - [x] `--lines`, `-n <num>` - Show last N lines
  - [x] `--level <level>` - Filter by log level
  - [x] `--json` - JSON format output

---

## Phase 3: Model & Provider Management (P1)

### 3.1 Models Commands

- [x] `models` - Model management
  - [x] `models list` - List available models
  - [x] `models status` - Model status
  - [x] `models set <model>` - Set default model
  - [x] `models scan` - Scan available models
  - [ ] Options:
    - [x] `--provider <name>` - Specify provider
    - [x] `--json` - JSON format output
    - [x] `--check` - Check model availability

### 3.2 Auth Commands (for providers)

- [ ] `models auth` - Provider authentication
  - [ ] `models auth add <provider>` - Add provider authentication
  - [ ] `models auth setup-token <provider>` - Set API Token
  - [ ] `models auth list` - List configured authentications

---

## Phase 4: Agent & Chat Commands (P2 - Nice to Have)

### 4.1 Agent Command

- [ ] `agent` - Run single Agent interaction
  - [ ] `--message <msg>` - Send message
  - [ ] `--session-id <id>` - Session ID
  - [ ] `--model <model>` - Model to use
  - [ ] `--thinking` - Show thinking process
  - [ ] `--verbose` - Verbose output
  - [ ] `--json` - JSON format output
  - [ ] `--timeout <seconds>` - Timeout duration

### 4.2 Sessions Command

- [x] `sessions` - Session management
  - [x] `sessions list` - List sessions
  - [x] `sessions show <id>` - Show session details
  - [x] `sessions delete <id>` - Delete session
  - [x] `sessions clear` - Clear all sessions
  - [x] Options:
    - [x] `--json` - JSON format output
    - [x] `--active` - Show only active sessions

---

## Phase 5: Automation Commands (P2)

### 5.1 Cron Commands

- [x] `cron` - Scheduled task management
  - [x] `cron list` - List tasks
  - [x] `cron status` - Task status
  - [x] `cron add` - Add task
  - [x] `cron rm <id>` - Remove task
  - [x] `cron enable <id>` - Enable task
  - [x] `cron disable <id>` - Disable task
  - [x] `cron run <id>` - Run task immediately
  - [x] `cron runs <id>` - View run history
  - [x] Options:
    - [x] `--name <name>` - Task name
    - [x] `--cron <expr>` - Cron expression
    - [x] `--handler <type>` - Handler type
    - [x] `--payload <json>` - Task payload
    - [x] `--json` - JSON format output

---

## Phase 6: Plugin & Skill Management (P2)

### 6.1 Plugins Commands

- [x] `plugins` - Plugin management
  - [x] `plugins list` - List plugins
  - [x] `plugins info <id>` - Plugin details
  - [x] `plugins enable <id>` - Enable plugin
  - [x] `plugins disable <id>` - Disable plugin
  - [x] `plugins doctor` - Plugin diagnostics
  - [x] Options:
    - [x] `--json` - JSON format output

### 6.2 Skills Commands

- [x] `skills` - Skill management
  - [x] `skills list` - List skills
  - [x] `skills info <id>` - Skill details
  - [x] `skills check` - Check skill status
  - [x] Options:
    - [x] `--eligible` - Show only eligible skills
    - [x] `--json` - JSON format output

---

## Phase 7: Security & Maintenance (P2)

### 7.1 Security Commands

- [ ] `security` - Security management
  - [ ] `security audit` - Security audit
    - [ ] `--deep` - Deep audit
    - [ ] `--fix` - Auto-fix

### 7.2 Reset & Uninstall

- [ ] `reset` - Reset configuration/state
  - [ ] `--scope <scope>` - Reset scope (config/state/all)
  - [ ] `--yes` - Skip confirmation
  - [ ] `--dry-run` - Show what would be executed only
- [ ] `uninstall` - Uninstall
  - [ ] `--service` - Uninstall service
  - [ ] `--state` - Delete state data
  - [ ] `--all` - Complete uninstall
  - [ ] `--yes` - Skip confirmation

---

## Phase 8: Advanced Features (P3 - Future)

### 8.1 Browser Commands

- [ ] `browser` - Browser control
  - [ ] `browser status` - Browser status
  - [ ] `browser start` - Start browser
  - [ ] `browser stop` - Stop browser
  - [ ] `browser tabs` - List tabs
  - [ ] `browser screenshot` - Screenshot

### 8.2 Memory Commands

- [ ] `memory` - Vector search
  - [ ] `memory status` - Index status
  - [ ] `memory index` - Rebuild index
  - [ ] `memory search <query>` - Semantic search

### 8.3 TUI Command

- [ ] `tui` - Terminal UI
  - [ ] `--session <id>` - Specify session
  - [ ] `--model <model>` - Specify model

---

## Implementation Checklist

### Phase 1 Tasks (Core)

1. [x] Refactor `main.go`, use cobra or urfave/cli framework
2. [x] Implement global flags parsing
3. [x] Implement `status` command
4. [x] Implement `health` command
5. [x] Implement `config` command group
6. [x] Implement `doctor` command
7. [ ] Add unit tests
8. [x] Update help documentation

### Phase 2 Tasks (Service)

1. [x] Implement `gateway` command group
2. [x] Implement `logs` command
3. [x] Cross-platform service management support
4. [ ] Add integration tests

### Phase 3 Tasks (Models)

1. [x] Implement `models` command group
2. [ ] Implement `models auth` subcommand
3. [x] Integrate with Provider Pool

### Phase 4-8 Tasks

Implement progressively by priority, release minor version after each Phase completion.

---

## Technical Design

### CLI Framework

Recommended: [cobra](https://github.com/spf13/cobra):
- Most popular CLI framework in Go ecosystem
- Supports subcommands, flags, auto-completion
- Good help documentation generation

### Directory Structure

```
server/
├── cmd/
│   └── echo/
│       ├── main.go           # Entry point
│       ├── root.go           # Root command
│       ├── status.go         # status command
│       ├── health.go         # health command
│       ├── config.go         # config command group
│       ├── gateway.go        # gateway command group
│       ├── models.go         # models command group
│       ├── cron.go           # cron command group
│       ├── plugins.go        # plugins command group
│       ├── skills.go         # skills command group
│       ├── security.go       # security command group
│       └── ...
```

### Output Formatting

- Default: Human-readable format with ANSI colors
- `--json`: Machine-readable JSON format
- `--no-color`: Disable colors
- Support TTY detection, auto-disable colors for non-TTY

### Configuration Storage

- Config file: `~/.zimaos-blue/config.yaml`
- State data: `~/.zimaos-blue/data/`
- Log files: `~/.zimaos-blue/logs/`
- Profile isolation: `~/.zimaos-blue-<profile>/`

---

## Success Metrics

1. All P0 commands implemented and tested
2. CLI help documentation complete
3. Feature parity with existing HTTP API
4. Cross-platform support (Windows, Linux, macOS)

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Breaking existing functionality | High | Maintain backward compatibility, incremental refactoring |
| Insufficient test coverage | Medium | Unit tests for each command |
| Documentation out of sync | Medium | Sync code and documentation updates |

---

## Timeline

- **Phase 1**: 1-2 weeks (Core commands)
- **Phase 2**: 1 week (Service management)
- **Phase 3**: 1 week (Models)
- **Phase 4-8**: Implement as needed

---

## References

- ClawdBot CLI Documentation: `clawdbot/docs/cli/index.md`
- ClawdBot CLI Implementation: `clawdbot/src/cli/`
- Cobra Framework: https://github.com/spf13/cobra
