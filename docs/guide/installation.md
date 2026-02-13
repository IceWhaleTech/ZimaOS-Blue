# Installation

## Manual Installation

### Download Binary

Download the appropriate binary for your platform from [GitHub Releases](https://github.com/IceWhaleTech/ZimaOS-Blue/server/releases).

| Platform | Architecture | Download |
|----------|--------------|----------|
| Linux | amd64 | `blue-linux-amd64.tar.gz` |
| Linux | arm64 | `blue-linux-arm64.tar.gz` |
| macOS | amd64 | `blue-darwin-amd64.tar.gz` |
| macOS | arm64 | `blue-darwin-arm64.tar.gz` |
| Windows | amd64 | `blue-windows-amd64.zip` |

### Extract and Install

```bash
# Linux/macOS
tar -xzf blue-linux-amd64.tar.gz
sudo mv blue /usr/local/bin/
sudo chmod +x /usr/local/bin/blue

# Create config directory
sudo mkdir -p /etc/zimaos-blue
```

### Create Configuration

```bash
sudo cat > /etc/zimaos-blue/config.yaml << 'EOF'
server:
  host: "0.0.0.0"
  port: 23456

log:
  level: "info"
  format: "json"

worker:
  pool_size: 10
EOF
```

### Create Systemd Service

```bash
sudo cat > /etc/systemd/system/zimaos-blue.service << 'EOF'
[Unit]
Description=ZimaOS Blue
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/blue --config /etc/zimaos-blue/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable zimaos-blue
sudo systemctl start zimaos-blue
```

## Docker Installation

```bash
docker run -d \
  --name zimaos-blue \
  -p 23456:23456 \
  -v /path/to/config:/etc/zimaos-blue \
  zimaos/blue:latest
```

### Docker Compose

```yaml
version: '3.8'
services:
  blue:
    image: zimaos/blue:latest
    ports:
      - "23456:23456"
    volumes:
      - ./config:/etc/zimaos-blue
      - ./data:/var/lib/zimaos-blue
    restart: unless-stopped
```

## Uninstallation

### Linux

```bash
sudo systemctl stop zimaos-blue
sudo systemctl disable zimaos-blue
sudo rm /etc/systemd/system/zimaos-blue.service
sudo rm -rf /opt/zimaos-blue
sudo userdel zimaos-blue
```

### Windows

```powershell
Stop-Service ZimaOS-Blue
sc.exe delete ZimaOS-Blue
Remove-Item -Recurse "$env:ProgramFiles\ZimaOS-Blue"
```
