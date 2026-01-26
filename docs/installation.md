# Installation Guide

[中文版本](./zh/installation.md)

This guide covers all installation methods for ZimaOS-Echo.

## Prerequisites

### System Requirements

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| CPU | 2 cores | 4+ cores |
| RAM | 512 MB | 2 GB+ |
| Disk | 100 MB | 1 GB+ |
| OS | Linux/macOS/Windows | Linux |

### Software Requirements

- Go 1.21+ (for building from source)
- Node.js 18+ (for frontend development)
- Docker (optional, for containerized deployment)

## Installation Methods

### Method 1: Pre-built Binary (Recommended)

Download the latest release for your platform:

```bash
# Linux (amd64)
curl -LO https://github.com/IceWhaleTech/ZimaOS-Echo/releases/latest/download/echo-linux-amd64
chmod +x echo-linux-amd64
sudo mv echo-linux-amd64 /usr/local/bin/echo

# Linux (arm64)
curl -LO https://github.com/IceWhaleTech/ZimaOS-Echo/releases/latest/download/echo-linux-arm64
chmod +x echo-linux-arm64
sudo mv echo-linux-arm64 /usr/local/bin/echo

# macOS (amd64)
curl -LO https://github.com/IceWhaleTech/ZimaOS-Echo/releases/latest/download/echo-darwin-amd64
chmod +x echo-darwin-amd64
sudo mv echo-darwin-amd64 /usr/local/bin/echo

# macOS (arm64 / Apple Silicon)
curl -LO https://github.com/IceWhaleTech/ZimaOS-Echo/releases/latest/download/echo-darwin-arm64
chmod +x echo-darwin-arm64
sudo mv echo-darwin-arm64 /usr/local/bin/echo
```

Verify installation:

```bash
echo --version
```

### Method 2: Docker

Using Docker Compose (recommended):

```yaml
# docker-compose.yml
version: '3.8'

services:
  echo:
    image: icewhaletech/zimaos-echo:latest
    container_name: zimaos-echo
    ports:
      - "8080:8080"
    volumes:
      - ./config:/app/config
      - ./data:/app/data
      - ./plugins:/app/plugins
    environment:
      - JWT_SECRET=${JWT_SECRET}
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    restart: unless-stopped
```

Start the service:

```bash
# Create required directories
mkdir -p config data plugins

# Start with Docker Compose
docker-compose up -d

# View logs
docker-compose logs -f
```

Using Docker directly:

```bash
docker run -d \
  --name zimaos-echo \
  -p 8080:8080 \
  -v $(pwd)/config:/app/config \
  -v $(pwd)/data:/app/data \
  -e JWT_SECRET=your-secret-key \
  icewhaletech/zimaos-echo:latest
```

### Method 3: Build from Source

Clone and build:

```bash
# Clone repository
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo

# Build backend
cd server
go build -o echo ./cmd/echo

# Build frontend (optional)
cd ../web
npm install
npm run build
```

Run the server:

```bash
./server/echo --config ./config.yaml
```

### Method 4: One-Click Install Script

For Linux systems:

```bash
curl -fsSL https://raw.githubusercontent.com/IceWhaleTech/ZimaOS-Echo/main/scripts/install.sh | bash
```

The script will:
1. Detect your system architecture
2. Download the appropriate binary
3. Create configuration directory
4. Set up systemd service
5. Start the service

### Method 5: ZimaOS App Store

For ZimaOS users:

1. Open ZimaOS Dashboard
2. Navigate to App Store
3. Search for "Echo"
4. Click "Install"
5. Configure settings in the app panel

## Initial Configuration

### 1. Create Configuration File

```bash
mkdir -p /etc/echo
cat > /etc/echo/config.yaml << 'EOF'
server:
  host: "0.0.0.0"
  port: 8080

llm:
  default_provider: "ollama"
  providers:
    ollama:
      base_url: "http://localhost:11434"
      model: "llama2"

auth:
  enabled: true
  jwt:
    secret: "${JWT_SECRET}"
    expiration: "24h"

logging:
  level: "info"
  format: "json"
EOF
```

### 2. Set Environment Variables

```bash
# Generate a secure JWT secret
export JWT_SECRET=$(openssl rand -base64 32)

# Optional: Set LLM API keys
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."
```

For persistent configuration, add to `/etc/environment` or create `/etc/echo/env`:

```bash
JWT_SECRET=your-generated-secret
OPENAI_API_KEY=sk-...
```

### 3. Start the Service

```bash
# Direct execution
echo --config /etc/echo/config.yaml

# Or with systemd
sudo systemctl start echo
sudo systemctl enable echo
```

## Systemd Service Setup

Create service file `/etc/systemd/system/echo.service`:

```ini
[Unit]
Description=ZimaOS Echo AI Assistant
After=network.target

[Service]
Type=simple
User=echo
Group=echo
WorkingDirectory=/opt/echo
ExecStart=/usr/local/bin/echo --config /etc/echo/config.yaml
Restart=always
RestartSec=5
EnvironmentFile=/etc/echo/env

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/echo /var/log/echo

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
# Create user
sudo useradd -r -s /bin/false echo

# Create directories
sudo mkdir -p /opt/echo /var/lib/echo /var/log/echo
sudo chown echo:echo /var/lib/echo /var/log/echo

# Enable service
sudo systemctl daemon-reload
sudo systemctl enable echo
sudo systemctl start echo

# Check status
sudo systemctl status echo
```

## LLM Provider Setup

### Ollama (Local, Recommended for Privacy)

Install Ollama:

```bash
curl -fsSL https://ollama.ai/install.sh | sh
```

Pull a model:

```bash
ollama pull llama2
# Or for better performance
ollama pull llama2:13b
```

Configure Echo:

```yaml
llm:
  default_provider: "ollama"
  providers:
    ollama:
      base_url: "http://localhost:11434"
      model: "llama2"
```

### OpenAI

1. Get API key from [OpenAI Platform](https://platform.openai.com/api-keys)
2. Configure:

```yaml
llm:
  default_provider: "openai"
  providers:
    openai:
      api_key: "${OPENAI_API_KEY}"
      model: "gpt-4"
```

### Anthropic

1. Get API key from [Anthropic Console](https://console.anthropic.com/)
2. Configure:

```yaml
llm:
  default_provider: "anthropic"
  providers:
    anthropic:
      api_key: "${ANTHROPIC_API_KEY}"
      model: "claude-3-opus-20240229"
```

## Verification

### Check Service Status

```bash
# Check if running
curl http://localhost:8080/health

# Expected response
{"status":"ok","version":"0.5.0"}
```

### Access Web UI

Open your browser and navigate to:

```
http://localhost:8080
```

### Test API

```bash
# Get auth token (if auth enabled)
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' | jq -r '.token')

# Test chat endpoint
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message":"Hello, Echo!"}'
```

## Upgrading

### Binary Installation

```bash
# Stop service
sudo systemctl stop echo

# Download new version
curl -LO https://github.com/IceWhaleTech/ZimaOS-Echo/releases/latest/download/echo-linux-amd64
sudo mv echo-linux-amd64 /usr/local/bin/echo
sudo chmod +x /usr/local/bin/echo

# Start service
sudo systemctl start echo
```

### Docker

```bash
# Pull latest image
docker-compose pull

# Restart with new image
docker-compose up -d
```

## Uninstallation

### Binary Installation

```bash
# Stop and disable service
sudo systemctl stop echo
sudo systemctl disable echo

# Remove files
sudo rm /usr/local/bin/echo
sudo rm -rf /etc/echo
sudo rm -rf /var/lib/echo
sudo rm /etc/systemd/system/echo.service
sudo systemctl daemon-reload

# Remove user
sudo userdel echo
```

### Docker

```bash
# Stop and remove container
docker-compose down

# Remove volumes (optional, will delete data)
docker-compose down -v

# Remove image
docker rmi icewhaletech/zimaos-echo
```

## Troubleshooting

### Service Won't Start

Check logs:

```bash
sudo journalctl -u echo -f
```

Common issues:
- Port 8080 already in use: Change port in config
- Missing JWT_SECRET: Set environment variable
- Permission denied: Check file permissions

### Can't Connect to LLM

For Ollama:
```bash
# Check if Ollama is running
curl http://localhost:11434/api/tags

# Check if model is available
ollama list
```

For cloud providers:
- Verify API key is correct
- Check network connectivity
- Verify API quota/billing

See [Troubleshooting Guide](./troubleshooting.md) for more solutions.

## Next Steps

- [Configuration Guide](./configuration.md) - Customize your installation
- [API Reference](./api-reference.md) - Integrate with your applications
- [NAS Integration](./nas-integration.md) - Set up NAS-specific features
