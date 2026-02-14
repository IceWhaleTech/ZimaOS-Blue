# 安装指南

[English Version](../../../DEV/installation.md)

本指南涵盖 ZimaOS-Blue 的所有安装方法。

## 前置要求

### 系统要求

| 组件 | 最低配置 | 推荐配置 |
|------|----------|----------|
| CPU | 2 核 | 4+ 核 |
| 内存 | 512 MB | 2 GB+ |
| 磁盘 | 100 MB | 1 GB+ |
| 操作系统 | Linux/macOS/Windows | Linux |

### 软件要求

- Go 1.21+（从源码构建时需要）
- Node.js 18+（前端开发时需要）
- Docker（可选，用于容器化部署）

## 安装方法

### 方法 1：预编译二进制文件（推荐）

下载适合您平台的最新版本：

```bash
# Linux (amd64)
curl -LO https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download/blue-linux-amd64
chmod +x blue-linux-amd64
sudo mv blue-linux-amd64 /usr/local/bin/blue

# Linux (arm64)
curl -LO https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download/blue-linux-arm64
chmod +x blue-linux-arm64
sudo mv blue-linux-arm64 /usr/local/bin/blue

# macOS (amd64)
curl -LO https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download/blue-darwin-amd64
chmod +x blue-darwin-amd64
sudo mv blue-darwin-amd64 /usr/local/bin/blue

# macOS (arm64 / Apple Silicon)
curl -LO https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download/blue-darwin-arm64
chmod +x blue-darwin-arm64
sudo mv blue-darwin-arm64 /usr/local/bin/blue
```

验证安装：

```bash
blue --version
```

### 方法 2：Docker

使用 Docker Compose（推荐）：

```yaml
# docker-compose.yml
version: '3.8'

services:
  blue:
    image: icewhaletech/zimaos-blue:latest
    container_name: zimaos-blue
    ports:
      - "23456:23456"
    volumes:
      - ./config:/app/config
      - ./data:/app/data
      - ./plugins:/app/plugins
    environment:
      - JWT_SECRET=${JWT_SECRET}
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    restart: unless-stopped
```

启动服务：

```bash
# 创建必要目录
mkdir -p config data plugins

# 使用 Docker Compose 启动
docker-compose up -d

# 查看日志
docker-compose logs -f
```

直接使用 Docker：

```bash
docker run -d \
  --name zimaos-blue \
  -p 23456:23456 \
  -v $(pwd)/config:/app/config \
  -v $(pwd)/data:/app/data \
  -e JWT_SECRET=your-secret-key \
  icewhaletech/zimaos-blue:latest
```

### 方法 3：从源码构建

克隆并构建：

```bash
# 克隆仓库
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue

# 构建后端
cd server
go build -o blue ./cmd/blue

# 构建前端（可选）
cd ../web
npm install
npm run build
```

运行服务器：

```bash
./server/blue --config ./config.yaml
```

### 方法 4：一键安装脚本

适用于 Linux 系统：

```bash
curl -fsSL https://raw.githubusercontent.com/IceWhaleTech/ZimaOS-Blue/main/scripts/install.sh | bash
```

脚本将：
1. 检测系统架构
2. 下载适当的二进制文件
3. 创建配置目录
4. 设置 systemd 服务
5. 启动服务

### 方法 5：ZimaOS 应用商店

ZimaOS 用户：

1. 打开 ZimaOS 控制面板
2. 导航到应用商店
3. 搜索 "Blue"
4. 点击 "安装"
5. 在应用面板中配置设置

## 初始配置

### 1. 创建配置文件

```bash
mkdir -p /etc/blue
cat > /etc/blue/config.yaml << 'EOF'
server:
  host: "0.0.0.0"
  port: 23456

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

### 2. 设置环境变量

```bash
# 生成安全的 JWT 密钥
export JWT_SECRET=$(openssl rand -base64 32)

# 可选：设置 LLM API 密钥
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."
```

持久化配置，添加到 `/etc/environment` 或创建 `/etc/blue/env`：

```bash
JWT_SECRET=your-generated-secret
OPENAI_API_KEY=sk-...
```

### 3. 启动服务

```bash
# 直接执行
blue --config /etc/blue/config.yaml

# 或使用 systemd
sudo systemctl start blue
sudo systemctl enable blue
```

## Systemd 服务设置

创建服务文件 `/etc/systemd/system/blue.service`：

```ini
[Unit]
Description=ZimaOS Blue AI 助手
After=network.target

[Service]
Type=simple
User=blue
Group=blue
WorkingDirectory=/opt/blue
ExecStart=/usr/local/bin/blue --config /etc/blue/config.yaml
Restart=always
RestartSec=5
EnvironmentFile=/etc/blue/env

# 安全加固
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/blue /var/log/blue

[Install]
WantedBy=multi-user.target
```

启用并启动：

```bash
# 创建用户
sudo useradd -r -s /bin/false blue

# 创建目录
sudo mkdir -p /opt/blue /var/lib/blue /var/log/blue
sudo chown blue:blue /var/lib/blue /var/log/blue

# 启用服务
sudo systemctl daemon-reload
sudo systemctl enable blue
sudo systemctl start blue

# 检查状态
sudo systemctl status blue
```

## LLM 提供商设置

### Ollama（本地，推荐用于隐私保护）

安装 Ollama：

```bash
curl -fsSL https://ollama.ai/install.sh | sh
```

拉取模型：

```bash
ollama pull llama2
# 或获得更好性能
ollama pull llama2:13b
```

配置 Blue：

```yaml
llm:
  default_provider: "ollama"
  providers:
    ollama:
      base_url: "http://localhost:11434"
      model: "llama2"
```

### OpenAI

1. 从 [OpenAI 平台](https://platform.openai.com/api-keys) 获取 API 密钥
2. 配置：

```yaml
llm:
  default_provider: "openai"
  providers:
    openai:
      api_key: "${OPENAI_API_KEY}"
      model: "gpt-4"
```

### Anthropic

1. 从 [Anthropic 控制台](https://console.anthropic.com/) 获取 API 密钥
2. 配置：

```yaml
llm:
  default_provider: "anthropic"
  providers:
    anthropic:
      api_key: "${ANTHROPIC_API_KEY}"
      model: "claude-3-opus-20240229"
```

## 验证

### 检查服务状态

```bash
# 检查是否运行
curl http://localhost:23456/health

# 预期响应
{"status":"ok","version":"0.5.0"}
```

### 访问 Web UI

打开浏览器并导航到：

```
http://localhost:23456
```

### 测试 API

```bash
# 获取认证令牌（如果启用了认证）
TOKEN=$(curl -s -X POST http://localhost:23456/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' | jq -r '.token')

# 测试聊天端点
curl -X POST http://localhost:23456/api/v1/chat \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message":"你好，Blue！"}'
```

## 升级

### 二进制安装

```bash
# 停止服务
sudo systemctl stop blue

# 下载新版本
curl -LO https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download/blue-linux-amd64
sudo mv blue-linux-amd64 /usr/local/bin/blue
sudo chmod +x /usr/local/bin/blue

# 启动服务
sudo systemctl start blue
```

### Docker

```bash
# 拉取最新镜像
docker-compose pull

# 使用新镜像重启
docker-compose up -d
```

## 卸载

### 二进制安装

```bash
# 停止并禁用服务
sudo systemctl stop blue
sudo systemctl disable blue

# 删除文件
sudo rm /usr/local/bin/blue
sudo rm -rf /etc/blue
sudo rm -rf /var/lib/blue
sudo rm /etc/systemd/system/blue.service
sudo systemctl daemon-reload

# 删除用户
sudo userdel blue
```

### Docker

```bash
# 停止并删除容器
docker-compose down

# 删除卷（可选，将删除数据）
docker-compose down -v

# 删除镜像
docker rmi icewhaletech/zimaos-blue
```

## 故障排除

### 服务无法启动

检查日志：

```bash
sudo journalctl -u blue -f
```

常见问题：
- 端口 23456 已被占用：在配置中更改端口
- 缺少 JWT_SECRET：设置环境变量
- 权限被拒绝：检查文件权限

### 无法连接到 LLM

对于 Ollama：
```bash
# 检查 Ollama 是否运行
curl http://localhost:11434/api/tags

# 检查模型是否可用
ollama list
```

对于云提供商：
- 验证 API 密钥是否正确
- 检查网络连接
- 验证 API 配额/计费

更多解决方案请参阅 [故障排除指南](./troubleshooting.md)。

## 下一步

- [配置指南](./configuration.md) - 自定义您的安装
- [API 参考](./api-reference.md) - 与您的应用程序集成
- [NAS 集成](./nas-integration.md) - 设置 NAS 特定功能
