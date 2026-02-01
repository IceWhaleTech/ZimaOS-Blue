# NAS Integration Guide

This guide covers how to install and configure ZimaOS Echo on various NAS platforms.

## Table of Contents

- [ZimaOS](#zimaos)
- [Synology DSM](#synology-dsm)
- [QNAP QTS](#qnap-qts)
- [TrueNAS](#truenas)
- [Unraid](#unraid)
- [Generic Linux](#generic-linux)

---

## ZimaOS

ZimaOS Echo is designed specifically for ZimaOS and provides the best integration experience.

### Installation via App Store

1. Open ZimaOS Dashboard
2. Navigate to **App Store**
3. Search for "Echo"
4. Click **Install**

### Manual Installation

```bash
# SSH into your ZimaOS device
ssh root@zimaos.local

# Pull the Docker image
docker pull icewhale/zimaos-echo:latest

# Run the container
docker run -d \
  --name zimaos-echo \
  --restart unless-stopped \
  -p 8765:23456 \
  -v /DATA/AppData/zimaos-echo/data:/app/data \
  -v /DATA/AppData/zimaos-echo/config:/app/config \
  -e TZ=$(cat /etc/timezone) \
  icewhale/zimaos-echo:latest
```

### ZimaOS-Specific Features

- **Auto-discovery**: Automatically discovers Home Assistant on your network
- **File Access**: Direct access to ZimaOS file shares
- **App Integration**: Interact with other ZimaOS apps
- **Dashboard Widget**: Quick access from ZimaOS dashboard

### Configuration

Edit `/DATA/AppData/zimaos-echo/config/config.yaml`:

```yaml
server:
  port: 23456

zimaos:
  enabled: true
  api_endpoint: http://localhost:80
  data_path: /DATA/AppData/zimaos-echo

llm:
  default_provider: ollama
  providers:
    ollama:
      base_url: http://localhost:11434
      model: llama3.2
```

---

## Synology DSM

### Prerequisites

- DSM 7.0 or later
- Docker package installed
- At least 1GB free RAM

### Installation via Container Manager

1. Open **Container Manager** (formerly Docker)
2. Go to **Registry** → Search "icewhale/zimaos-echo"
3. Download the `latest` tag
4. Go to **Image** → Select the image → **Run**
5. Configure:
   - **Container Name**: zimaos-echo
   - **Port Settings**: Local 8765 → Container 23456
   - **Volume**: `/docker/zimaos-echo` → `/app/data`
   - **Environment**: `TZ=Your/Timezone`

### Using Docker Compose

Create `/volume1/docker/zimaos-echo/docker-compose.yml`:

```yaml
version: '3.8'
services:
  zimaos-echo:
    image: icewhale/zimaos-echo:latest
    container_name: zimaos-echo
    restart: unless-stopped
    ports:
      - "8765:23456"
    volumes:
      - ./data:/app/data
      - ./config:/app/config
    environment:
      - TZ=America/New_York
      - ECHO_LOG_LEVEL=info
```

Run:
```bash
cd /volume1/docker/zimaos-echo
docker-compose up -d
```

### Reverse Proxy Setup

1. Open **Control Panel** → **Login Portal** → **Advanced**
2. Click **Reverse Proxy** → **Create**
3. Configure:
   - **Source**: HTTPS, your domain, port 443
   - **Destination**: HTTP, localhost, port 8765
4. Enable WebSocket under **Custom Header**

### Synology-Specific Tips

- Use Synology's built-in Let's Encrypt for SSL
- Create a dedicated shared folder for Echo data
- Use Synology's scheduled tasks for backups
- Monitor resources via Resource Monitor

---

## QNAP QTS

### Prerequisites

- QTS 5.0 or later
- Container Station installed
- At least 1GB free RAM

### Installation via Container Station

1. Open **Container Station**
2. Go to **Create** → **Create Application**
3. Paste the following YAML:

```yaml
version: '3'
services:
  zimaos-echo:
    image: icewhale/zimaos-echo:latest
    container_name: zimaos-echo
    restart: always
    ports:
      - "8765:23456"
    volumes:
      - /share/Container/zimaos-echo/data:/app/data
      - /share/Container/zimaos-echo/config:/app/config
    environment:
      - TZ=America/New_York
```

4. Click **Create**

### Manual Docker Installation

```bash
# SSH into QNAP
ssh admin@qnap.local

# Create directories
mkdir -p /share/Container/zimaos-echo/{data,config}

# Run container
docker run -d \
  --name zimaos-echo \
  --restart always \
  -p 8765:23456 \
  -v /share/Container/zimaos-echo/data:/app/data \
  -v /share/Container/zimaos-echo/config:/app/config \
  -e TZ=America/New_York \
  icewhale/zimaos-echo:latest
```

### Reverse Proxy with QNAP

1. Install **Nginx** from App Center (or use built-in reverse proxy)
2. Configure virtual host for Echo
3. Enable WebSocket support

---

## TrueNAS

### TrueNAS SCALE (Recommended)

TrueNAS SCALE uses Kubernetes, making app deployment straightforward.

#### Via TrueCharts

1. Add TrueCharts catalog:
   - **Apps** → **Manage Catalogs** → **Add Catalog**
   - Name: `truecharts`
   - Repository: `https://github.com/truecharts/catalog`

2. Install ZimaOS Echo:
   - **Apps** → **Available Applications**
   - Search "zimaos-echo" (if available) or use Custom App

#### Custom App Installation

1. Go to **Apps** → **Available Applications** → **Custom App**
2. Configure:
   - **Application Name**: zimaos-echo
   - **Image Repository**: icewhale/zimaos-echo
   - **Image Tag**: latest
   - **Container Port**: 23456
   - **Node Port**: 8765

3. Add Storage:
   - **Host Path**: `/mnt/pool/apps/zimaos-echo/data`
   - **Mount Path**: `/app/data`

### TrueNAS CORE (FreeBSD Jail)

TrueNAS CORE uses FreeBSD jails. Use a Linux jail or VM:

1. Create a Ubuntu VM via **Virtual Machines**
2. Install Docker in the VM
3. Follow [Generic Linux](#generic-linux) instructions

---

## Unraid

### Installation via Community Apps

1. Go to **Apps** tab
2. Search "zimaos-echo" (if available in CA)
3. Click **Install**

### Manual Docker Installation

1. Go to **Docker** tab
2. Click **Add Container**
3. Configure:
   - **Name**: zimaos-echo
   - **Repository**: icewhale/zimaos-echo:latest
   - **Port Mapping**: 8765 → 23456
   - **Path Mapping**: `/mnt/user/appdata/zimaos-echo` → `/app/data`

### Docker Compose (via Compose Manager)

1. Install **Compose Manager** plugin
2. Create stack `zimaos-echo`:

```yaml
version: '3.8'
services:
  zimaos-echo:
    image: icewhale/zimaos-echo:latest
    container_name: zimaos-echo
    restart: unless-stopped
    ports:
      - "8765:23456"
    volumes:
      - /mnt/user/appdata/zimaos-echo/data:/app/data
      - /mnt/user/appdata/zimaos-echo/config:/app/config
    environment:
      - TZ=America/New_York
      - PUID=99
      - PGID=100
```

### Unraid-Specific Tips

- Use User Scripts plugin for backup automation
- Monitor via Unraid dashboard
- Use Unraid's built-in reverse proxy (SWAG/NPM)

---

## Generic Linux

### Using Docker

```bash
# Install Docker (if not installed)
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# Create directories
sudo mkdir -p /opt/zimaos-echo/{data,config}
sudo chown -R $USER:$USER /opt/zimaos-echo

# Run container
docker run -d \
  --name zimaos-echo \
  --restart unless-stopped \
  -p 8765:23456 \
  -v /opt/zimaos-echo/data:/app/data \
  -v /opt/zimaos-echo/config:/app/config \
  -e TZ=$(cat /etc/timezone) \
  icewhale/zimaos-echo:latest
```

### Using Docker Compose

Create `/opt/zimaos-echo/docker-compose.yml`:

```yaml
version: '3.8'
services:
  zimaos-echo:
    image: icewhale/zimaos-echo:latest
    container_name: zimaos-echo
    restart: unless-stopped
    ports:
      - "8765:23456"
    volumes:
      - ./data:/app/data
      - ./config:/app/config
    environment:
      - TZ=${TZ:-UTC}
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:23456/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

Run:
```bash
cd /opt/zimaos-echo
docker-compose up -d
```

### Using Systemd (Native Binary)

1. Download the binary:
```bash
wget https://github.com/IceWhaleTech/ZimaOS-Echo/releases/latest/download/zimaos-echo-linux-amd64
sudo mv zimaos-echo-linux-amd64 /usr/local/bin/zimaos-echo
sudo chmod +x /usr/local/bin/zimaos-echo
```

2. Create systemd service `/etc/systemd/system/zimaos-echo.service`:
```ini
[Unit]
Description=ZimaOS Echo AI Assistant
After=network.target

[Service]
Type=simple
User=zimaos-echo
Group=zimaos-echo
WorkingDirectory=/opt/zimaos-echo
ExecStart=/usr/local/bin/zimaos-echo --config /etc/zimaos-echo/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

3. Enable and start:
```bash
sudo systemctl daemon-reload
sudo systemctl enable zimaos-echo
sudo systemctl start zimaos-echo
```

---

## Common Configuration

### Home Assistant Integration

All platforms can integrate with Home Assistant:

```yaml
homeassistant:
  enabled: true
  url: http://homeassistant.local:8123
  token: your-long-lived-access-token
```

### Ollama Integration (Local LLM)

Run Ollama alongside Echo:

```yaml
# docker-compose.yml
version: '3.8'
services:
  zimaos-echo:
    image: icewhale/zimaos-echo:latest
    # ... other config ...
    environment:
      - ECHO_LLM_DEFAULT_PROVIDER=ollama
      - ECHO_LLM_OLLAMA_BASE_URL=http://ollama:11434

  ollama:
    image: ollama/ollama:latest
    volumes:
      - ollama-data:/root/.ollama
    ports:
      - "11434:11434"

volumes:
  ollama-data:
```

### Reverse Proxy with Nginx

```nginx
server {
    listen 443 ssl http2;
    server_name echo.yourdomain.com;

    ssl_certificate /etc/letsencrypt/live/echo.yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/echo.yourdomain.com/privkey.pem;

    location / {
        proxy_pass http://localhost:8765;
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

---

## Troubleshooting

### Container Won't Start

```bash
# Check logs
docker logs zimaos-echo

# Check if port is in use
netstat -tlnp | grep 8765

# Verify volume permissions
ls -la /path/to/data
```

### Can't Access Web UI

1. Check container is running: `docker ps`
2. Check firewall rules
3. Verify port mapping
4. Try accessing via IP instead of hostname

### Performance Issues

- Ensure adequate RAM (minimum 512MB)
- Use SSD for data directory
- Consider local LLM (Ollama) for lower latency
- Enable caching in config

### Database Errors

```bash
# Backup and recreate database
docker exec zimaos-echo cp /app/data/echo.db /app/data/echo.db.bak
docker restart zimaos-echo
```

---

## Support

- Documentation: https://docs.zimaspace.com/echo
- GitHub Issues: https://github.com/IceWhaleTech/ZimaOS-Echo/issues
- Discord: https://discord.gg/zimaos
