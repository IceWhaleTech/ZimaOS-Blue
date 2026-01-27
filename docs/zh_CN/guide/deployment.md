# 部署

本指南介绍在各种环境中部署 ZimaOS Echo。

## 系统要求

### 最低要求

| 资源 | 要求 |
|----------|-------------|
| CPU | 1 核（ARM 或 x86_64） |
| 内存 | 256MB |
| 磁盘 | 100MB |
| 操作系统 | Linux、macOS、Windows |

### 推荐配置

| 资源 | 要求 |
|----------|-------------|
| CPU | 2+ 核 |
| 内存 | 512MB+ |
| 磁盘 | 1GB+ |
| 操作系统 | Linux（Debian/Ubuntu） |

## 部署方式

### 1. 一键安装（推荐）

#### Linux / macOS

```bash
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash
```

#### Windows（以管理员身份运行 PowerShell）

```powershell
irm https://echo.zimaos.com/install.ps1 | iex
```

### 2. 手动安装

#### 下载二进制文件

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

#### 安装为服务

```bash
# 复制二进制文件
sudo cp zimaos-echo /usr/local/bin/

# 创建 systemd 服务
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

# 启用并启动
sudo systemctl daemon-reload
sudo systemctl enable zimaos-echo
sudo systemctl start zimaos-echo
```

### 3. Docker 部署

```bash
# 拉取镜像
docker pull icewhaletech/zimaos-echo:latest

# 运行容器
docker run -d \
  --name zimaos-echo \
  -p 8080:8080 \
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
      - "8080:8080"
    volumes:
      - ./config:/etc/zimaos-echo
      - ./data:/var/lib/zimaos-echo
    environment:
      - ECHO_LOG_LEVEL=info
      - ECHO_API_KEY=${API_KEY}
```

### 4. ZimaOS 应用商店

即将推出 - 从 ZimaOS 应用商店一键安装。

## 配置

### 环境变量

| 变量 | 描述 | 默认值 |
|----------|-------------|---------|
| `ECHO_CONFIG_PATH` | 配置文件路径 | `/etc/zimaos-echo/config.yaml` |
| `ECHO_DATA_PATH` | 数据目录 | `/var/lib/zimaos-echo` |
| `ECHO_LOG_LEVEL` | 日志级别 | `info` |
| `ECHO_HTTP_PORT` | HTTP 端口 | `8080` |
| `ECHO_API_KEY` | LLM 提供商的 API 密钥 | - |

### 配置文件

```yaml
# /etc/zimaos-echo/config.yaml

server:
  host: "0.0.0.0"
  port: 8080
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

## 反向代理设置

### Nginx

```nginx
server {
    listen 80;
    server_name echo.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
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
    reverse_proxy localhost:8080
}
```

## SSL/TLS

### 使用 Certbot 获取 Let's Encrypt 证书

```bash
sudo certbot --nginx -d echo.example.com
```

### 自签名证书

```bash
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout /etc/ssl/private/zimaos-echo.key \
  -out /etc/ssl/certs/zimaos-echo.crt
```

## 监控

### 健康检查

```bash
curl http://localhost:8080/api/v1/health
```

### Prometheus 指标

```bash
curl http://localhost:8080/metrics
```

### 日志

```bash
# Systemd
journalctl -u zimaos-echo -f

# Docker
docker logs -f zimaos-echo
```

## 备份与恢复

### 创建备份

```bash
curl -X POST http://localhost:8080/api/v1/backup
```

### 从备份恢复

```bash
curl -X POST http://localhost:8080/api/v1/backup/{id}/restore
```

## 故障排除

### 服务无法启动

1. 检查日志：`journalctl -u zimaos-echo -n 50`
2. 验证配置：`zimaos-echo config validate`
3. 检查数据目录权限

### 内存使用过高

1. 检查配置中的缓存大小
2. 查看 goroutine 数量：`curl http://localhost:8080/debug/pprof/goroutine?debug=1`
3. 启用内存分析

### 连接问题

1. 验证防火墙规则
2. 检查端口可用性：`netstat -tlnp | grep 8080`
3. 测试连接：`curl -v http://localhost:8080/api/v1/health`

## 升级

### 自动升级

```bash
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash
```

### 手动升级

```bash
# 停止服务
sudo systemctl stop zimaos-echo

# 备份当前二进制文件
sudo cp /usr/local/bin/zimaos-echo /usr/local/bin/zimaos-echo.bak

# 下载新版本
wget https://github.com/IceWhaleTech/ZimaOS-Echo/releases/latest/download/zimaos-echo-linux-amd64.tar.gz
tar -xzf zimaos-echo-linux-amd64.tar.gz
sudo cp zimaos-echo /usr/local/bin/

# 启动服务
sudo systemctl start zimaos-echo
```
