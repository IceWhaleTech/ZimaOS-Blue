# Installation

## Manual Installation

### Download Binary

Download the appropriate binary for your platform from [GitHub Releases](https://github.com/IceWhaleTech/ZimaOS-Echo/server/releases).

| Platform | Architecture | Download |
|----------|--------------|----------|
| Linux | amd64 | `echo-linux-amd64.tar.gz` |
| Linux | arm64 | `echo-linux-arm64.tar.gz` |
| macOS | amd64 | `echo-darwin-amd64.tar.gz` |
| macOS | arm64 | `echo-darwin-arm64.tar.gz` |
| Windows | amd64 | `echo-windows-amd64.zip` |

### Extract and Install

```bash
# Linux/macOS
tar -xzf echo-linux-amd64.tar.gz
sudo mv echo /usr/local/bin/
sudo chmod +x /usr/local/bin/echo

# Create config directory
sudo mkdir -p /etc/zimaos-echo
```

### Create Configuration

```bash
sudo cat > /etc/zimaos-echo/config.yaml << 'EOF'
server:
  host: "0.0.0.0"
  port: 8080

log:
  level: "info"
  format: "json"

worker:
  pool_size: 10
EOF
```

### Create Systemd Service

```bash
sudo cat > /etc/systemd/system/zimaos-echo.service << 'EOF'
[Unit]
Description=ZimaOS Echo
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/echo --config /etc/zimaos-echo/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable zimaos-echo
sudo systemctl start zimaos-echo
```

## Docker Installation

```bash
docker run -d \
  --name zimaos-echo \
  -p 8080:8080 \
  -v /path/to/config:/etc/zimaos-echo \
  zimaos/echo:latest
```

### Docker Compose

```yaml
version: '3.8'
services:
  echo:
    image: zimaos/echo:latest
    ports:
      - "8080:8080"
    volumes:
      - ./config:/etc/zimaos-echo
      - ./data:/var/lib/zimaos-echo
    restart: unless-stopped
```

## Uninstallation

### Linux

```bash
sudo systemctl stop zimaos-echo
sudo systemctl disable zimaos-echo
sudo rm /etc/systemd/system/zimaos-echo.service
sudo rm -rf /opt/zimaos-echo
sudo userdel zimaos-echo
```

### Windows

```powershell
Stop-Service ZimaOS-Echo
sc.exe delete ZimaOS-Echo
Remove-Item -Recurse "$env:ProgramFiles\ZimaOS-Echo"
```
