# Deployment

This guide covers deploying ZimaOS Echo in various environments.

## System Requirements

### Minimum Requirements

| Resource | Requirement |
|----------|-------------|
| CPU | 1 core (ARM or x86_64) |
| RAM | 256MB |
| Disk | 100MB |
| OS | Linux, macOS, Windows |

### Recommended Requirements

| Resource | Requirement |
|----------|-------------|
| CPU | 2+ cores |
| RAM | 512MB+ |
| Disk | 1GB+ |
| OS | Linux (Debian/Ubuntu) |

## Deployment Options

### 1. Manual Installation

#### Download Binary

```bash
# Linux AMD64
wget https://github.com/IceWhaleTech/ZimaOS-Echo/releases/latest/download/zimaos-echo-linux-amd64.tar.gz
tar -xzf zimaos-echo-linux-amd64.tar.gz

# Linux ARM64
wget https://github.com/IceWhaleTech/ZimaOS-Echo/releases/latest/download/zimaos-echo-linux-arm64.tar.gz
tar -xzf zimaos-echo-linux-arm64.tar.gz

# macOS
wget https://github.com/IceWhaleTech/ZimaOS-Echo/releases/latest/download/zimaos-echo-darwin-amd64.tar.gz
tar -xzf zimaos-echo-darwin-amd64.tar.gz
```

#### Install as Service

```bash
# Copy binary
sudo cp zimaos-echo /usr/local/bin/

# Create systemd service
sudo tee /etc/systemd/system/zimaos-echo.service > /dev/null <<EOF
[Unit]
Description=ZimaOS Echo Agent Runtime
After=network.target

[Service]
Type=simple
User=zimaos-echo
ExecStart=/usr/local/bin/zimaos-echo server
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable zimaos-echo
sudo systemctl start zimaos-echo
```

### 3. Docker Deployment

```bash
# Pull image
docker pull icewhaletech/zimaos-echo:latest

# Run container
docker run -d \
  --name zimaos-echo \
  -p 23456:23456 \
  -v /path/to/config:/etc/zimaos-echo \
  -v /path/to/data:/var/lib/zimaos-echo \
  icewhaletech/zimaos-echo:latest
```

#### Docker Compose

```yaml
version: '3.8'

services:
  zimaos-echo:
    image: icewhaletech/zimaos-echo:latest
    container_name: zimaos-echo
    restart: unless-stopped
    ports:
      - "23456:23456"
    volumes:
      - ./config:/etc/zimaos-echo
      - ./data:/var/lib/zimaos-echo
    environment:
      - ECHO_LOG_LEVEL=info
      - ECHO_API_KEY=${API_KEY}
```

### 4. ZimaOS App Store

Coming soon - one-click installation from ZimaOS App Store.

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ECHO_CONFIG_PATH` | Config file path | `/etc/zimaos-echo/config.yaml` |
| `ECHO_DATA_PATH` | Data directory | `/var/lib/zimaos-echo` |
| `ECHO_LOG_LEVEL` | Log level | `info` |
| `ECHO_HTTP_PORT` | HTTP port | `23456` |
| `ECHO_API_KEY` | API key for LLM provider | - |

### Config File

```yaml
# /etc/zimaos-echo/config.yaml

server:
  host: "0.0.0.0"
  port: 23456
  read_timeout: 30s
  write_timeout: 30s

llm:
  provider: "openai"
  model: "gpt-4"
  api_key: "${ECHO_API_KEY}"
  max_tokens: 4096

database:
  path: "/var/lib/zimaos-echo/data.db"
  max_open_conns: 10

cache:
  max_size: 1000
  default_ttl: 5m

logging:
  level: "info"
  format: "json"
  output: "stdout"
```

## Reverse Proxy Setup

### Nginx

```nginx
server {
    listen 80;
    server_name echo.example.com;

    location / {
        proxy_pass http://127.0.0.1:23456;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Caddy

```caddyfile
echo.example.com {
    reverse_proxy localhost:23456
}
```

## SSL/TLS

### Let's Encrypt with Certbot

```bash
sudo certbot --nginx -d echo.example.com
```

### Self-Signed Certificate

```bash
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout /etc/ssl/private/zimaos-echo.key \
  -out /etc/ssl/certs/zimaos-echo.crt
```

## Monitoring

### Health Check

```bash
curl http://localhost:23456/api/v1/health
```

### Prometheus Metrics

```bash
curl http://localhost:23456/metrics
```

### Logs

```bash
# Systemd
journalctl -u zimaos-echo -f

# Docker
docker logs -f zimaos-echo
```

## Backup & Restore

### Create Backup

```bash
curl -X POST http://localhost:23456/api/v1/backup
```

### Restore from Backup

```bash
curl -X POST http://localhost:23456/api/v1/backup/{id}/restore
```

## Troubleshooting

### Service Won't Start

1. Check logs: `journalctl -u zimaos-echo -n 50`
2. Verify config: `zimaos-echo config validate`
3. Check permissions on data directory

### High Memory Usage

1. Check cache size in config
2. Review goroutine count: `curl http://localhost:23456/debug/pprof/goroutine?debug=1`
3. Enable memory profiling

### Connection Issues

1. Verify firewall rules
2. Check port availability: `netstat -tlnp | grep 23456`
3. Test connectivity: `curl -v http://localhost:23456/api/v1/health`

## Upgrading

### Manual Upgrade

```bash
# Stop service
sudo systemctl stop zimaos-echo

# Backup current binary
sudo cp /usr/local/bin/zimaos-echo /usr/local/bin/zimaos-echo.bak

# Download new version
wget https://github.com/IceWhaleTech/ZimaOS-Echo/releases/latest/download/zimaos-echo-linux-amd64.tar.gz
tar -xzf zimaos-echo-linux-amd64.tar.gz
sudo cp zimaos-echo /usr/local/bin/

# Start service
sudo systemctl start zimaos-echo
```
