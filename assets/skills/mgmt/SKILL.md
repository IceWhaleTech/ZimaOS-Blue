# Mgmt

System management tool. Manages LLM providers, settings, channels, skills, tools, users, API keys, and system info.

## How to Send

Use the `blue` CLI with `mgmt.{domain}.{action}` format:

```bash
blue mgmt.providers.list
```

Add `--json` for JSON output.

## Domains & Actions

### providers — LLM Provider Management

| Action | Required Params | Description |
|--------|----------------|-------------|
| `providers.list` | — | List all providers with status and models |
| `providers.add` | `name`, `provider_type` | Add provider. Optional: `base_url`, `api_key`. Types: openai, anthropic, google, ollama |
| `providers.remove` | `id` | Remove a provider |
| `providers.enable` | `id` | Enable a disabled provider |
| `providers.disable` | `id` | Disable a provider |
| `providers.test` | `id` | Test provider connectivity |
| `providers.models` | — | List all available models across providers |

### settings — System Settings

| Action | Required Params | Description |
|--------|----------------|-------------|
| `settings.get` | — | Get all current settings (locale, agent_mode, etc.) |
| `settings.set` | `key`, `value` | Update a setting. Keys: locale, timezone, smart_tool_selection, agent_mode, agent_auto_confirm |

### channels — Channel Management

| Action | Required Params | Description |
|--------|----------------|-------------|
| `channels.list` | — | List all channels (iMessage, DingTalk, etc.) with status |
| `channels.status` | `name` | Get status of a specific channel |

### skills — Skill Management

| Action | Required Params | Description |
|--------|----------------|-------------|
| `skills.list` | — | List all registered skills (enabled/disabled, builtin/custom) |
| `skills.enable` | `id` | Enable a skill |
| `skills.disable` | `id` | Disable a skill |

### tools — Tool Management

| Action | Required Params | Description |
|--------|----------------|-------------|
| `tools.list` | — | List all registered tools (enabled/disabled) |
| `tools.enable` | `name` | Enable a tool |
| `tools.disable` | `name` | Disable a tool |

### system — System Info

| Action | Required Params | Description |
|--------|----------------|-------------|
| `system.health` | — | Health check: version, uptime, Go version, CPU count, goroutines, memory |
| `system.version` | — | Get current version |

### proxy — Proxy & Cache Stats

| Action | Required Params | Description |
|--------|----------------|-------------|
| `proxy.stats` | — | Get proxy pipeline statistics |
| `proxy.cache_stats` | — | Get prompt cache statistics |

### users — User Management

| Action | Required Params | Description |
|--------|----------------|-------------|
| `users.list` | — | List all users with role and lock status |
| `users.lock` | `id` | Lock a user account |
| `users.unlock` | `id` | Unlock a user account |

### apikeys — API Key Management

| Action | Required Params | Description |
|--------|----------------|-------------|
| `apikeys.list` | — | List API keys (prefix only, keys are masked) |
| `apikeys.create` | `name` | Create a new API key |
| `apikeys.revoke` | `id` | Revoke an API key |

## Example Triggers

- "List all providers" / "列出所有提供商"
- "Add an OpenAI provider" / "添加一个OpenAI提供商"
- "Show system health" / "显示系统状态"
- "List all skills" / "列出所有技能"
- "Disable the browser tool" / "禁用浏览器工具"
- "Show current settings" / "显示当前设置"
- "List all users" / "列出所有用户"
- "Create an API key" / "创建一个API密钥"
- "What models are available?" / "有哪些可用的模型？"
