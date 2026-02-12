# Chat Interaction Flow

This document describes the complete chat interaction flow in ZimaOS-Blue, particularly focusing on the Claude Code CLI integration.

## Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Frontend (Vue/Pinia)                            │
├─────────────────────────────────────────────────────────────────────────────┤
│  User Input → sendMessage() → SSEClient.connect()                            │
│                                    │                                         │
│                                    ▼                                         │
│  POST /api/v1/conversations/{id}/messages/stream                            │
│  Headers: Accept: text/event-stream                                          │
│  Body: { message, provider, model, temperature, max_tokens }                │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Backend (chat.go)                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│  StreamMessage()                                                             │
│    ├─ Store user message to SQLite                                           │
│    ├─ Get message history (50 messages)                                      │
│    ├─ Get provider (claude-code)                                             │
│    ├─ Call provider.ChatStream(ctx, chatReq)                                 │
│    └─ Set SSE Headers: Content-Type: text/event-stream                       │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                      Claude Code Provider (provider.go)                      │
├─────────────────────────────────────────────────────────────────────────────┤
│  ChatStream()                                                                │
│    ├─ buildRunParams() - Build CLI parameters                                │
│    ├─ sandboxedRunner.RunStream(ctx, params)                                 │
│    └─ Convert CliStreamChunk → llm.StreamChunk                               │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Runner (runner.go)                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│  RunStream()                                                                 │
│    ├─ Build command: claude -p --output-format stream-json ...               │
│    ├─ exec.CommandContext() start process                                    │
│    ├─ cmd.StdoutPipe() get output pipe                                       │
│    └─ parser.ParseStream(stdout, "jsonl")                                    │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         OutputParser (parser.go)                             │
├─────────────────────────────────────────────────────────────────────────────┤
│  ParseStream() → parseStreamJSONL()                                          │
│    ├─ bufio.Reader read line by line                                         │
│    ├─ json.Unmarshal parse each JSON line                                    │
│    ├─ extractText() - Extract text content                                   │
│    └─ Send CliStreamChunk to channel                                         │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           SSE Response Format                                │
├─────────────────────────────────────────────────────────────────────────────┤
│  data: {"delta":"Hello","done":false,"stream_id":"xxx"}\n\n                 │
│  data: {"delta":" World","done":false,"stream_id":"xxx"}\n\n                │
│  data: {"delta":"!","done":true,"usage":{...},"stream_id":"xxx"}\n\n        │
│  data: [DONE]\n\n                                                            │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Frontend Processing (sse.ts)                       │
├─────────────────────────────────────────────────────────────────────────────┤
│  ReadableStream.read() read chunks                                           │
│    ├─ Parse "data: " prefix                                                  │
│    ├─ JSON.parse() parse chunk                                               │
│    ├─ onMessage(chunk) - Accumulate streamingContent                         │
│    └─ Update last message content in messages array                          │
└─────────────────────────────────────────────────────────────────────────────┘
```

## CLI Command

The Claude Code CLI is executed with the following command:

```bash
claude -p \
  --output-format stream-json \
  --dangerously-skip-permissions \
  --model sonnet \
  --session-id <session-id> \
  --append-system-prompt "<system-prompt>" \
  "<user-prompt>"
```

### Command Arguments

| Argument | Description |
|----------|-------------|
| `-p` | Print mode (non-interactive) |
| `--output-format stream-json` | Output in streaming JSON format (JSONL) |
| `--dangerously-skip-permissions` | Skip permission prompts |
| `--model <model>` | Model to use (opus, sonnet, haiku) |
| `--session-id <id>` | Session ID for context continuity |
| `--append-system-prompt <prompt>` | System prompt to append |
| `--resume <session-id>` | Resume an existing session |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `ANTHROPIC_AUTH_TOKEN` | API authentication token |
| `ANTHROPIC_BASE_URL` | Custom API base URL (optional) |

## Key Code Locations

| Component | File | Key Function |
|-----------|------|--------------|
| Frontend Store | `web/src/stores/chat.ts:139` | `sendMessage()` |
| SSE Client | `web/src/utils/sse.ts:13` | `connect()` |
| Backend Handler | `server/internal/server/chat.go:356` | `StreamMessage()` |
| Provider | `server/internal/claudecode/provider.go:97` | `ChatStream()` |
| Runner | `server/internal/claudecode/runner.go:224` | `RunStream()` |
| Parser | `server/internal/claudecode/parser.go:132` | `parseStreamJSONL()` |

## Streaming Implementation

### Backend Streaming (chat.go:455-588)

```go
// Set SSE headers
c.Response().Header().Set("Content-Type", "text/event-stream")
c.Response().Header().Set("Cache-Control", "no-cache")
c.Response().Header().Set("Connection", "keep-alive")

// Stream loop
for {
    select {
    case <-ctx.Done():
        // Handle cancellation
    case chunk, ok := <-chunks:
        if !ok {
            // Channel closed, send [DONE]
            c.Response().Write([]byte("data: [DONE]\n\n"))
            c.Response().Flush()
            return nil
        }

        // Write SSE data
        data := map[string]interface{}{
            "delta":     chunk.Delta,
            "done":      chunk.Done,
            "stream_id": streamID,
        }
        jsonData, _ := json.Marshal(data)
        c.Response().Write([]byte("data: " + string(jsonData) + "\n\n"))
        c.Response().Flush()
    }
}
```

### Frontend Streaming (sse.ts:56-91)

```typescript
const reader = response.body?.getReader()
const decoder = new TextDecoder()
let buffer = ''

while (this.isConnected) {
    const { done, value } = await reader.read()
    if (done) break

    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() || ''

    for (const line of lines) {
        if (line.startsWith('data: ')) {
            const data = line.slice(6).trim()
            if (data === '[DONE]') {
                options.onComplete?.()
                break
            }
            const chunk: StreamChunk = JSON.parse(data)
            options.onMessage(chunk)
        }
    }
}
```

## Sandbox Mode

When sandbox mode is enabled, CLI execution is isolated:

```go
// SandboxedRunner wraps Runner with sandbox execution
type SandboxedRunner struct {
    runner         *Runner
    sandboxManager *sandbox.Manager
    config         *ClaudeCodeConfig
    enabled        bool
}

// Execute in sandbox
func (sr *SandboxedRunner) Run(ctx context.Context, params *RunParams) (*RunResult, error) {
    if !sr.enabled || sr.sandboxManager == nil {
        return sr.runner.Run(ctx, params)
    }
    return sr.runSandboxed(ctx, params)
}
```

### Sandbox Configuration

```yaml
claudecode:
  sandbox:
    enabled: true
    memory_limit: 536870912  # 512 MB
    cpu_limit: 1.0
    process_limit: 50
    network_enabled: true
    allowed_paths:
      - /data/workspace
    denied_paths:
      - /etc/passwd
      - /etc/shadow
      - /root
      - /home
```

## Cancellation Flow

```
Frontend: cancelStreaming()
    └─ sseClient.disconnect()
        └─ abortController.abort()
            └─ Fetch aborted

Backend: ctx.Done() triggered
    ├─ Record metrics (cancelled)
    ├─ Store partial message + "[Response interrupted]"
    ├─ Send cancellation event
    └─ Unregister stream
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/conversations` | Create conversation |
| GET | `/api/v1/conversations` | List conversations |
| GET | `/api/v1/conversations/{id}` | Get conversation |
| DELETE | `/api/v1/conversations/{id}` | Delete conversation |
| GET | `/api/v1/conversations/{id}/messages` | Get messages |
| POST | `/api/v1/conversations/{id}/messages` | Send message (non-streaming) |
| POST | `/api/v1/conversations/{id}/messages/stream` | Send message (streaming) |
| POST | `/api/v1/conversations/{id}/messages/cancel` | Cancel stream |
| GET | `/api/v1/streams/active` | List active streams |
| POST | `/api/v1/streams/cancel-all` | Cancel all streams |

## Data Flow Summary

```
User Input
    ↓
Frontend Store (Pinia)
    ↓
SSE Client (Fetch API)
    ↓
Backend Chat Handler
    ↓
LLM Provider Registry
    ↓
Claude Code Provider / Other Providers
    ↓
CLI Runner / API Calls
    ↓
Stream Chunks
    ↓
Backend: Store Message + Metrics
    ↓
Frontend: Update UI in Real-time
    ↓
Display Complete Response
```
