# 部署

本指南介绍在各种环境中部署 ZimaOS Blue。

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

### 1. 手动安装

#### 下载二进制文件

```bash
# Linux AMD64
wget https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download/zimaos-blue-linux-amd64.tar.gz
tar -xzf zimaos-blue-linux-amd64.tar.gz

# Linux ARM64
wget https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download/zimaos-blue-linux-arm64.tar.gz
tar -xzf zimaos-blue-linux-arm64.tar.gz

# macOS
wget https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download/zimaos-blue-darwin-amd64.tar.gz
tar -xzf zimaos-blue-darwin-amd64.tar.gz
```

#### 安装为服务

```bash
# 复制二进制文件
sudo cp zimaos-blue /usr/local/bin/

# 创建 systemd 服务
sudo tee /etc/systemd/system/zimaos-blue.service > /dev/null <<EOF
[Unit]
Description=ZimaOS Blue Agent Runtime
After=network.target

[Service]
Type=simple
User=zimaos-blue
ExecStart=/usr/local/bin/zimaos-blue server
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 启用并启动
sudo systemctl daemon-reload
sudo systemctl enable zimaos-blue
sudo systemctl start zimaos-blue
```

### 3. Docker 部署

```bash
# 拉取镜像
docker pull icewhaletech/zimaos-blue:latest

# 运行容器
docker run -d \
  --name zimaos-blue \
  -p 23456:23456 \
  -v /path/to/config:/etc/zimaos-blue \
  -v /path/to/data:/var/lib/zimaos-blue \
  icewhaletech/zimaos-blue:latest
```

#### Docker Compose

```yaml
version: '3.8'

services:
  zimaos-blue:
    image: icewhaletech/zimaos-blue:latest
    container_name: zimaos-blue
    restart: unless-stopped
    ports:
      - "23456:23456"
    volumes:
      - ./config:/etc/zimaos-blue
      - ./data:/var/lib/zimaos-blue
    environment:
      - BLUE_LOG_LEVEL=info
      - BLUE_API_KEY=${API_KEY}
```

### 4. ZimaOS 应用商店

即将推出 - 从 ZimaOS 应用商店一键安装。

## 配置

### 环境变量

| 变量 | 描述 | 默认值 |
|----------|-------------|---------|
| `BLUE_CONFIG_PATH` | 配置文件路径 | `/etc/zimaos-blue/config.yaml` |
| `BLUE_DATA_PATH` | 数据目录 | `/var/lib/zimaos-blue` |
| `BLUE_LOG_LEVEL` | 日志级别 | `info` |
| `BLUE_HTTP_PORT` | HTTP 端口 | `23456` |
| `BLUE_API_KEY` | LLM 提供商的 API 密钥 | - |

### 配置文件

```yaml
# /etc/zimaos-blue/config.yaml

server:
  host: "0.0.0.0"
  port: 23456
  read_timeout: 30s
  write_timeout: 30s

llm:
  provider: "openai"
  model: "gpt-4"
  api_key: "${BLUE_API_KEY}"
  max_tokens: 4096

database:
  path: "/var/lib/zimaos-blue/data.db"
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
    server_name blue.example.com;

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
blue.example.com {
    reverse_proxy localhost:23456
}
```

## SSL/TLS

### 使用 Certbot 获取 Let's Encrypt 证书

```bash
sudo certbot --nginx -d blue.example.com
```

### 自签名证书

```bash
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout /etc/ssl/private/zimaos-blue.key \
  -out /etc/ssl/certs/zimaos-blue.crt
```

## 监控

### 健康检查

```bash
curl http://localhost:23456/api/v1/health
```

### Prometheus 指标

```bash
curl http://localhost:23456/metrics
```

### 日志

```bash
# Systemd
journalctl -u zimaos-blue -f

# Docker
docker logs -f zimaos-blue
```

## 备份与恢复

### 创建备份

```bash
curl -X POST http://localhost:23456/api/v1/backup
```

### 从备份恢复

```bash
curl -X POST http://localhost:23456/api/v1/backup/{id}/restore
```

## 故障排除

### 服务无法启动

1. 检查日志：`journalctl -u zimaos-blue -n 50`
2. 验证配置：`zimaos-blue config validate`
3. 检查数据目录权限

### 内存使用过高

1. 检查配置中的缓存大小
2. 查看 goroutine 数量：`curl http://localhost:23456/debug/pprof/goroutine?debug=1`
3. 启用内存分析

### 连接问题

1. 验证防火墙规则
2. 检查端口可用性：`netstat -tlnp | grep 23456`
3. 测试连接：`curl -v http://localhost:23456/api/v1/health`

## 升级

### 手动升级

```bash
# 停止服务
sudo systemctl stop zimaos-blue

# 备份当前二进制文件
sudo cp /usr/local/bin/zimaos-blue /usr/local/bin/zimaos-blue.bak

# 下载新版本
wget https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download/zimaos-blue-linux-amd64.tar.gz
tar -xzf zimaos-blue-linux-amd64.tar.gz
sudo cp zimaos-blue /usr/local/bin/

# 启动服务
sudo systemctl start zimaos-blue
```
