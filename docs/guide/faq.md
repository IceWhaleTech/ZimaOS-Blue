# Frequently Asked Questions (FAQ)

## General Questions

### What is ZimaOS Blue?

ZimaOS Blue is an AI assistant designed for home servers and NAS devices. It provides:
- Natural language chat interface
- Smart home control via Home Assistant
- Multi-channel messaging (Telegram, Discord, etc.)
- Voice assistant capabilities
- Browser automation
- Plugin system for extensibility

### What LLM providers are supported?

ZimaOS Blue supports multiple LLM providers:
- **OpenAI**: GPT-4o, GPT-4o-mini, GPT-4-turbo, GPT-3.5-turbo
- **Anthropic**: Claude 3.5 Sonnet, Claude 3 Opus, Claude 3 Haiku
- **Ollama**: Local models (Llama 3.2, Mistral, CodeLlama, etc.)
- **Custom**: Any OpenAI-compatible API

### What are the system requirements?

**Minimum:**
- CPU: 2 cores
- RAM: 512MB
- Storage: 1GB
- OS: Linux (amd64/arm64), Windows, macOS

**Recommended:**
- CPU: 4 cores
- RAM: 2GB
- Storage: 10GB
- SSD for database

### Is ZimaOS Blue free?

Yes, ZimaOS Blue is open source under the Apache 2.0 license. However, you may incur costs for:
- Cloud LLM API usage (OpenAI, Anthropic)
- Cloud hosting (if not self-hosted)

Using local models with Ollama is completely free.

---

## Installation

### How do I install ZimaOS Blue?

**Docker (Recommended):**
```bash
docker run -d \
  --name zimaos-blue \
  -p 23456:23456 \
  -v echo-data:/app/data \
  icewhale/zimaos-blue:latest
```

**Binary:**
```bash
# Download from releases
wget https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download/zimaos-blue-linux-amd64
chmod +x zimaos-blue-linux-amd64
./zimaos-blue-linux-amd64
```

### How do I update ZimaOS Blue?

**Docker:**
```bash
docker pull icewhale/zimaos-blue:latest
docker stop zimaos-blue
docker rm zimaos-blue
# Re-run with same volume mounts
```

**Binary:**
Download the new version and replace the binary. Your data is preserved in the data directory.

### Can I run ZimaOS Blue on a Raspberry Pi?

Yes! ZimaOS Blue supports ARM64 architecture. For best performance:
- Use Raspberry Pi 4 with 4GB+ RAM
- Use Ollama with smaller models (Phi-3, TinyLlama)
- Enable swap if needed

---

## Configuration

### Where is the configuration file?

Default locations:
- `/etc/zimaos-blue/config.yaml`
- `./config.yaml` (current directory)
- `~/.config/zimaos-blue/config.yaml`

Or specify with: `zimaos-blue --config /path/to/config.yaml`

### How do I configure multiple LLM providers?

```yaml
llm:
  default_provider: openai
  providers:
    openai:
      api_key: sk-...
      model: gpt-4o-mini
    anthropic:
      api_key: sk-ant-...
      model: claude-3-5-sonnet-20241022
    ollama:
      base_url: http://localhost:11434
      model: llama3.2
```

### How do I enable HTTPS?

Option 1: Use a reverse proxy (recommended)
```nginx
server {
    listen 443 ssl;
    server_name echo.example.com;

    ssl_certificate /etc/letsencrypt/live/echo.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/echo.example.com/privkey.pem;

    location / {
        proxy_pass http://localhost:23456;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

Option 2: Direct TLS
```yaml
server:
  tls:
    enabled: true
    cert_file: /path/to/cert.pem
    key_file: /path/to/key.pem
```

### How do I change the default port?

```yaml
server:
  port: 8081
```

Or via environment variable:
```bash
export BLUE_SERVER_PORT=8081
```

---

## LLM & Chat

### Which model should I use?

| Use Case | Recommended Model |
|----------|-------------------|
| General chat | GPT-4o-mini, Claude 3 Haiku |
| Complex tasks | GPT-4o, Claude 3.5 Sonnet |
| Privacy-focused | Ollama (local) |
| Low latency | GPT-3.5-turbo, local models |
| Cost-effective | Ollama, GPT-4o-mini |

### How do I use local models with Ollama?

1. Install Ollama: https://ollama.ai
2. Pull a model: `ollama pull llama3.2`
3. Configure Echo:
```yaml
llm:
  default_provider: ollama
  providers:
    ollama:
      base_url: http://localhost:11434
      model: llama3.2
```

### Why are responses slow?

Common causes:
1. **Network latency** to cloud APIs
2. **Large model** (GPT-4 is slower than GPT-3.5)
3. **Long context** (more tokens = slower)
4. **Rate limiting** from provider

Solutions:
- Use streaming for real-time feedback
- Use faster models (GPT-4o-mini, local models)
- Reduce max_tokens setting
- Use local Ollama for lowest latency

### How do I limit token usage?

```yaml
llm:
  max_tokens: 1000
  max_context_tokens: 4000
```

### Can I use my own fine-tuned model?

Yes, if it's OpenAI-compatible:
```yaml
llm:
  providers:
    custom:
      base_url: https://your-api.com/v1
      api_key: your-key
      model: your-fine-tuned-model
```

---

## Smart Home

### How do I connect to Home Assistant?

1. Get a long-lived access token from Home Assistant:
   - Profile → Long-Lived Access Tokens → Create Token

2. Configure Echo:
```yaml
homeassistant:
  enabled: true
  url: http://homeassistant.local:8123
  token: your-long-lived-token
```

### What can I control with voice/chat?

- Lights (on/off, brightness, color)
- Switches and outlets
- Thermostats
- Locks
- Covers (blinds, garage doors)
- Media players
- Scenes and automations
- Any Home Assistant entity

### Example commands?

- "Turn on the living room lights"
- "Set bedroom temperature to 72"
- "Lock the front door"
- "What's the temperature in the kitchen?"
- "Run the movie night scene"

---

## Channels & Messaging

### How do I set up Telegram?

1. Create a bot with @BotFather
2. Get the bot token
3. Configure:
```yaml
channels:
  telegram:
    enabled: true
    token: "123456789:ABCdefGHIjklMNOpqrsTUVwxyz"
```

### How do I set up Discord?

1. Create a Discord application at https://discord.com/developers
2. Create a bot and get the token
3. Configure:
```yaml
channels:
  discord:
    enabled: true
    token: "your-discord-bot-token"
```

### Can I use multiple channels simultaneously?

Yes! Enable as many channels as you need:
```yaml
channels:
  telegram:
    enabled: true
  discord:
    enabled: true
  slack:
    enabled: true
```

---

## Security

### Is my data secure?

- All data is stored locally on your server
- API keys are stored encrypted
- No data is sent to third parties (except LLM providers)
- HTTPS is supported for secure connections

### How do I enable authentication?

```yaml
security:
  users:
    allow_registration: false
  password:
    min_length: 12
    require_uppercase: true
    require_number: true
```

### How do I enable MFA?

```yaml
security:
  mfa:
    enabled: true
    required: false  # Set to true to require MFA
```

### How do I reset the admin password?

```bash
zimaos-blue reset-password --username admin
```

---

## Backup & Recovery

### How do I backup my data?

**Automatic backups:**
```yaml
backup:
  enabled: true
  schedule: "0 2 * * *"  # Daily at 2 AM
  retention_days: 7
```

**Manual backup:**
```bash
curl -X POST http://localhost:23456/api/backup/create \
  -H "Authorization: Bearer <token>" \
  -d '{"type": "full"}'
```

### How do I restore from backup?

```bash
curl -X POST http://localhost:23456/api/backup/restore/<backup-id> \
  -H "Authorization: Bearer <token>"
```

### Where are backups stored?

Default: `/var/lib/zimaos-blue/backups/`

Configure:
```yaml
backup:
  path: /custom/backup/path
```

---

## Troubleshooting

### How do I enable debug logging?

```yaml
log:
  level: debug
```

### How do I check the service status?

```bash
curl http://localhost:23456/health
```

### Where can I get help?

- Documentation: https://docs.zimaspace.com/echo
- GitHub Issues: https://github.com/IceWhaleTech/ZimaOS-Blue/issues
- Discord: https://discord.gg/zimaos

---

## Development

### How do I build from source?

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue/server
go build -o zimaos-blue ./cmd/server
```

### How do I run tests?

```bash
cd server
go test ./... -v
```

### How do I contribute?

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests
5. Submit a pull request

See [CONTRIBUTING.md](../CONTRIBUTING.md) for details.
