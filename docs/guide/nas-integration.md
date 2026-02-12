# NAS Integration Guide

This guide covers how to install and configure ZimaOS Blue on various NAS platforms.

## Table of Contents

- [ZimaOS](#zimaos)
- [Synology DSM](#synology-dsm)
- [QNAP QTS](#qnap-qts)
- [TrueNAS](#truenas)
- [Unraid](#unraid)
- [Generic Linux](#generic-linux)

---

## ZimaOS

ZimaOS Blue is designed specifically for ZimaOS and provides the best integration experience.

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
docker pull icewhale/zimaos-blue:latest

# Run the container
docker run -d \
  --name zimaos-blue \
  --restart unless-stopped \
  -p 8765:23456 \
  -v /DATA/AppData/zimaos-blue/data:/app/data \
  -v /DATA/AppData/zimaos-blue/config:/app/config \
  -e TZ=$(cat /etc/timezone) \
  icewhale/zimaos-blue:latest
```

### ZimaOS-Specific Features

- **Auto-discovery**: Automatically discovers Home Assistant on your network
- **File Access**: Direct access to ZimaOS file shares
- **App Integration**: Interact with other ZimaOS apps
- **Dashboard Widget**: Quick access from ZimaOS dashboard

### Configuration

Edit `/DATA/AppData/zimaos-blue/config/config.yaml`:

```yaml
server:
  port: 23456

zimaos:
  enabled: true
  api_endpoint: http://localhost:80
  data_path: /DATA/AppData/zimaos-blue

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
2. Go to **Registry** → Search "icewhale/zimaos-blue"
3. Download the `latest` tag
4. Go to **Image** → Select the image → **Run**
5. Configure:
   - **Container Name**: zimaos-blue
   - **Port Settings**: Local 8765 → Container 23456
   - **Volume**: `/docker/zimaos-blue` → `/app/data`
   - **Environment**: `TZ=Your/Timezone`

### Using Docker Compose

Create `/volume1/docker/zimaos-blue/docker-compose.yml`:

```yaml
version: '3.8'
services:
  zimaos-blue:
    image: icewhale/zimaos-blue:latest
    container_name: zimaos-blue
    restart: unless-stopped
    ports:
      - "8765:23456"
    volumes:
      - ./data:/app/data
      - ./config:/app/config
    environment:
      - TZ=America/New_York
      - BLUE_LOG_LEVEL=info
```

Run:
```bash
cd /volume1/docker/zimaos-blue
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
  zimaos-blue:
    image: icewhale/zimaos-blue:latest
    container_name: zimaos-blue
    restart: always
    ports:
      - "8765:23456"
    volumes:
      - /share/Container/zimaos-blue/data:/app/data
      - /share/Container/zimaos-blue/config:/app/config
    environment:
      - TZ=America/New_York
```

4. Click **Create**

### Manual Docker Installation

```bash
# SSH into QNAP
ssh admin@qnap.local

# Create directories
mkdir -p /share/Container/zimaos-blue/{data,config}

# Run container
docker run -d \
  --name zimaos-blue \
  --restart always \
  -p 8765:23456 \
  -v /share/Container/zimaos-blue/data:/app/data \
  -v /share/Container/zimaos-blue/config:/app/config \
  -e TZ=America/New_York \
  icewhale/zimaos-blue:latest
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

2. Install ZimaOS Blue:
   - **Apps** → **Available Applications**
   - Search "zimaos-blue" (if available) or use Custom App

#### Custom App Installation

1. Go to **Apps** → **Available Applications** → **Custom App**
2. Configure:
   - **Application Name**: zimaos-blue
   - **Image Repository**: icewhale/zimaos-blue
   - **Image Tag**: latest
   - **Container Port**: 23456
   - **Node Port**: 8765

3. Add Storage:
   - **Host Path**: `/mnt/pool/apps/zimaos-blue/data`
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
2. Search "zimaos-blue" (if available in CA)
3. Click **Install**

### Manual Docker Installation

1. Go to **Docker** tab
2. Click **Add Container**
3. Configure:
   - **Name**: zimaos-blue
   - **Repository**: icewhale/zimaos-blue:latest
   - **Port Mapping**: 8765 → 23456
   - **Path Mapping**: `/mnt/user/appdata/zimaos-blue` → `/app/data`

### Docker Compose (via Compose Manager)

1. Install **Compose Manager** plugin
2. Create stack `zimaos-blue`:

```yaml
version: '3.8'
services:
  zimaos-blue:
    image: icewhale/zimaos-blue:latest
    container_name: zimaos-blue
    restart: unless-stopped
    ports:
      - "8765:23456"
    volumes:
      - /mnt/user/appdata/zimaos-blue/data:/app/data
      - /mnt/user/appdata/zimaos-blue/config:/app/config
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
sudo mkdir -p /opt/zimaos-blue/{data,config}
sudo chown -R $USER:$USER /opt/zimaos-blue

# Run container
docker run -d \
  --name zimaos-blue \
  --restart unless-stopped \
  -p 8765:23456 \
  -v /opt/zimaos-blue/data:/app/data \
  -v /opt/zimaos-blue/config:/app/config \
  -e TZ=$(cat /etc/timezone) \
  icewhale/zimaos-blue:latest
```

### Using Docker Compose

Create `/opt/zimaos-blue/docker-compose.yml`:

```yaml
version: '3.8'
services:
  zimaos-blue:
    image: icewhale/zimaos-blue:latest
    container_name: zimaos-blue
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
cd /opt/zimaos-blue
docker-compose up -d
```

### Using Systemd (Native Binary)

1. Download the binary:
```bash
wget https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download/zimaos-blue-linux-amd64
sudo mv zimaos-blue-linux-amd64 /usr/local/bin/zimaos-blue
sudo chmod +x /usr/local/bin/zimaos-blue
```

2. Create systemd service `/etc/systemd/system/zimaos-blue.service`:
```ini
[Unit]
Description=ZimaOS Blue AI Assistant
After=network.target

[Service]
Type=simple
User=zimaos-blue
Group=zimaos-blue
WorkingDirectory=/opt/zimaos-blue
ExecStart=/usr/local/bin/zimaos-blue --config /etc/zimaos-blue/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

3. Enable and start:
```bash
sudo systemctl daemon-reload
sudo systemctl enable zimaos-blue
sudo systemctl start zimaos-blue
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
  zimaos-blue:
    image: icewhale/zimaos-blue:latest
    # ... other config ...
    environment:
      - BLUE_LLM_DEFAULT_PROVIDER=ollama
      - BLUE_LLM_OLLAMA_BASE_URL=http://ollama:11434

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
docker logs zimaos-blue

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
docker exec zimaos-blue cp /app/data/echo.db /app/data/echo.db.bak
docker restart zimaos-blue
```

---

## Support

- Documentation: https://docs.zimaspace.com/echo
- GitHub Issues: https://github.com/IceWhaleTech/ZimaOS-Blue/issues
- Discord: https://discord.gg/zimaos
