# 安装指南

[English Version](../../../DEV/installation.md)

本指南涵盖 ZimaOS-Echo 的所有安装方法。

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

验证安装：

```bash
echo --version
```

### 方法 2：Docker

使用 Docker Compose（推荐）：

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
  --name zimaos-echo \
  -p 8080:8080 \
  -v $(pwd)/config:/app/config \
  -v $(pwd)/data:/app/data \
  -e JWT_SECRET=your-secret-key \
  icewhaletech/zimaos-echo:latest
```

### 方法 3：从源码构建

克隆并构建：

```bash
# 克隆仓库
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo

# 构建后端
cd server
go build -o echo ./cmd/echo

# 构建前端（可选）
cd ../web
npm install
npm run build
```

运行服务器：

```bash
./server/echo --config ./config.yaml
```

### 方法 4：一键安装脚本

适用于 Linux 系统：

```bash
curl -fsSL https://raw.githubusercontent.com/IceWhaleTech/ZimaOS-Echo/main/scripts/install.sh | bash
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
3. 搜索 "Echo"
4. 点击 "安装"
5. 在应用面板中配置设置

## 初始配置

### 1. 创建配置文件

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

### 2. 设置环境变量

```bash
# 生成安全的 JWT 密钥
export JWT_SECRET=$(openssl rand -base64 32)

# 可选：设置 LLM API 密钥
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."
```

持久化配置，添加到 `/etc/environment` 或创建 `/etc/echo/env`：

```bash
JWT_SECRET=your-generated-secret
OPENAI_API_KEY=sk-...
```

### 3. 启动服务

```bash
# 直接执行
echo --config /etc/echo/config.yaml

# 或使用 systemd
sudo systemctl start echo
sudo systemctl enable echo
```

## Systemd 服务设置

创建服务文件 `/etc/systemd/system/echo.service`：

```ini
[Unit]
Description=ZimaOS Echo AI 助手
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

# 安全加固
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/echo /var/log/echo

[Install]
WantedBy=multi-user.target
```

启用并启动：

```bash
# 创建用户
sudo useradd -r -s /bin/false echo

# 创建目录
sudo mkdir -p /opt/echo /var/lib/echo /var/log/echo
sudo chown echo:echo /var/lib/echo /var/log/echo

# 启用服务
sudo systemctl daemon-reload
sudo systemctl enable echo
sudo systemctl start echo

# 检查状态
sudo systemctl status echo
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

配置 Echo：

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
curl http://localhost:8080/health

# 预期响应
{"status":"ok","version":"0.5.0"}
```

### 访问 Web UI

打开浏览器并导航到：

```
http://localhost:8080
```

### 测试 API

```bash
# 获取认证令牌（如果启用了认证）
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' | jq -r '.token')

# 测试聊天端点
curl -X POST http://localhost:8080/api/v1/chat \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message":"你好，Echo！"}'
```

## 升级

### 二进制安装

```bash
# 停止服务
sudo systemctl stop echo

# 下载新版本
curl -LO https://github.com/IceWhaleTech/ZimaOS-Echo/releases/latest/download/echo-linux-amd64
sudo mv echo-linux-amd64 /usr/local/bin/echo
sudo chmod +x /usr/local/bin/echo

# 启动服务
sudo systemctl start echo
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
sudo systemctl stop echo
sudo systemctl disable echo

# 删除文件
sudo rm /usr/local/bin/echo
sudo rm -rf /etc/echo
sudo rm -rf /var/lib/echo
sudo rm /etc/systemd/system/echo.service
sudo systemctl daemon-reload

# 删除用户
sudo userdel echo
```

### Docker

```bash
# 停止并删除容器
docker-compose down

# 删除卷（可选，将删除数据）
docker-compose down -v

# 删除镜像
docker rmi icewhaletech/zimaos-echo
```

## 故障排除

### 服务无法启动

检查日志：

```bash
sudo journalctl -u echo -f
```

常见问题：
- 端口 8080 已被占用：在配置中更改端口
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
