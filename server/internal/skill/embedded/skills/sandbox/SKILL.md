# Sandbox

Execute commands in a sandboxed environment with limits and isolation.

## How to Send

Direct call:

```bash
sandbox.execute command=echo args='["hello"]'
```

Add `--json` for structured output.

## Commands

### sandbox.execute

Run a command in sandbox.

Required: `command`
Optional: `args`, `stdin`, `timeout` (seconds, default 30, max 300)

```bash
sandbox.execute command=sh args='["-lc","date"]' timeout=20
```

### sandbox.status

Get execution status by ID.

Required: `id`

```bash
sandbox.status id=exec_abc123
```

### sandbox.kill

Stop a running execution.

Required: `id`

```bash
sandbox.kill id=exec_abc123
```

### sandbox.info

Show sandbox availability/capability info.

```bash
sandbox.info
```

## Error Response

All commands return `status=error` with an `error` message on failure.

Common errors:
- `action is required`
- `invalid action: ...`
- `command is required for execute`
- `id is required for status/kill`
- `sandbox service not available`

## Example Triggers

- "Run this shell command safely in sandbox"
- "Check sandbox execution status"
- "停止这个沙箱任务"
