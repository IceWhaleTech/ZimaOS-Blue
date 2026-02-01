# NAS 集成指南

本指南介绍在各种 NAS 平台上安装和配置 ZimaOS Echo 的方法。

## 目录

- [ZimaOS](#zimaos)
- [群晖 DSM](#synology-dsm)
- [威联通 QTS](#qnap-qts)
- [TrueNAS](#truenas)
- [Unraid](#unraid)
- [通用 Linux](#generic-linux)

---

## ZimaOS

ZimaOS Echo 专为 ZimaOS 设计，提供最佳集成体验。

### 通过应用商店安装

1. 打开 ZimaOS 控制台
2. 进入 **应用商店**
3. 搜索「Echo」
4. 点击 **安装**

### 手动安装

```bash
# SSH 登录 ZimaOS 设备
ssh root@zimaos.local

# 拉取 Docker 镜像
docker pull icewhale/zimaos-echo:latest

# 运行容器
docker run -d \
  --name zimaos-echo \
  --restart unless-stopped \
  -p 8765:23456 \
  -v /DATA/AppData/zimaos-echo/data:/app/data \
  -v /DATA/AppData/zimaos-echo/config:/app/config \
  -e TZ=$(cat /etc/timezone) \
  icewhale/zimaos-echo:latest
```

### ZimaOS 特有功能

- **自动发现**：自动发现局域网内的 Home Assistant
- **文件访问**：直接访问 ZimaOS 文件共享
- **应用集成**：与其他 ZimaOS 应用交互
- **控制台组件**：从 ZimaOS 控制台快速进入

### 配置

编辑 `/DATA/AppData/zimaos-echo/config/config.yaml`：

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

## 群晖 DSM

### 前置要求

- DSM 7.0 或更高
- 已安装 Docker（容器管理器）
- 至少 1GB 可用内存

### 通过容器管理器安装

1. 打开 **容器管理器**（原 Docker）
2. 进入 **注册表** → 搜索「icewhale/zimaos-echo」
3. 下载 `latest` 标签
4. 进入 **映像** → 选择映像 → **运行**
5. 配置：
   - **容器名称**：zimaos-echo
   - **端口设置**：本地 8765 → 容器 23456
   - **卷**：`/docker/zimaos-echo` → `/app/data`
   - **环境**：`TZ=您的时区`

### 使用 Docker Compose

创建 `/volume1/docker/zimaos-echo/docker-compose.yml`：

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

运行：
```bash
cd /volume1/docker/zimaos-echo
docker-compose up -d
```

### 反向代理设置

1. 打开 **控制面板** → **登录门户** → **高级**
2. 点击 **反向代理** → **新建**
3. 配置：
   - **来源**：HTTPS、您的域名、端口 443
   - **目标**：HTTP、localhost、端口 8765
4. 在 **自定义头** 中启用 WebSocket

### 群晖相关建议

- 使用群晖内置 Let's Encrypt 申请 SSL
- 为 Echo 数据创建专用共享文件夹
- 用群晖计划任务做备份
- 通过资源监控查看资源使用

---

## 威联通 QTS

### 前置要求

- QTS 5.0 或更高
- 已安装 Container Station
- 至少 1GB 可用内存

### 通过 Container Station 安装

1. 打开 **Container Station**
2. 进入 **创建** → **创建应用**
3. 粘贴以下 YAML：

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

4. 点击 **创建**

### 手动 Docker 安装

```bash
# SSH 登录威联通
ssh admin@qnap.local

# 创建目录
mkdir -p /share/Container/zimaos-echo/{data,config}

# 运行容器
docker run -d \
  --name zimaos-echo \
  --restart always \
  -p 8765:23456 \
  -v /share/Container/zimaos-echo/data:/app/data \
  -v /share/Container/zimaos-echo/config:/app/config \
  -e TZ=America/New_York \
  icewhale/zimaos-echo:latest
```

### 威联通反向代理

1. 从 App Center 安装 **Nginx**（或使用内置反向代理）
2. 为 Echo 配置虚拟主机
3. 启用 WebSocket 支持

---

## TrueNAS

### TrueNAS SCALE（推荐）

TrueNAS SCALE 基于 Kubernetes，部署应用较简单。

#### 通过 TrueCharts

1. 添加 TrueCharts 目录：
   - **应用** → **管理目录** → **添加目录**
   - 名称：`truecharts`
   - 仓库：`https://github.com/truecharts/catalog`

2. 安装 ZimaOS Echo：
   - **应用** → **可用应用**
   - 搜索「zimaos-echo」（若可用）或使用自定义应用

#### 自定义应用安装

1. 进入 **应用** → **可用应用** → **自定义应用**
2. 配置：
   - **应用名称**：zimaos-echo
   - **镜像仓库**：icewhale/zimaos-echo
   - **镜像标签**：latest
   - **容器端口**：23456
   - **节点端口**：8765

3. 添加存储：
   - **主机路径**：`/mnt/pool/apps/zimaos-echo/data`
   - **挂载路径**：`/app/data`

### TrueNAS CORE（FreeBSD Jail）

TrueNAS CORE 使用 FreeBSD Jail，需在 Linux Jail 或 VM 中运行：

1. 在 **虚拟机** 中创建 Ubuntu VM
2. 在 VM 中安装 Docker
3. 按 [通用 Linux](#generic-linux) 步骤操作

---

## Unraid

### 通过 Community Apps 安装

1. 打开 **Apps** 标签
2. 搜索「zimaos-echo」（若 CA 中有）
3. 点击 **安装**

### 手动 Docker 安装

1. 打开 **Docker** 标签
2. 点击 **添加容器**
3. 配置：
   - **名称**：zimaos-echo
   - **仓库**：icewhale/zimaos-echo:latest
   - **端口映射**：8765 → 23456
   - **路径映射**：`/mnt/user/appdata/zimaos-echo` → `/app/data`

### Docker Compose（通过 Compose Manager）

1. 安装 **Compose Manager** 插件
2. 创建栈 `zimaos-echo`：

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

### Unraid 相关建议

- 用 User Scripts 插件做备份
- 在 Unraid 仪表盘监控
- 使用内置反向代理（SWAG/NPM）

---

## 通用 Linux

### 使用 Docker

```bash
# 安装 Docker（若未安装）
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# 创建目录
sudo mkdir -p /opt/zimaos-echo/{data,config}
sudo chown -R $USER:$USER /opt/zimaos-echo

# 运行容器
docker run -d \
  --name zimaos-echo \
  --restart unless-stopped \
  -p 8765:23456 \
  -v /opt/zimaos-echo/data:/app/data \
  -v /opt/zimaos-echo/config:/app/config \
  -e TZ=$(cat /etc/timezone) \
  icewhale/zimaos-echo:latest
```

### 使用 Docker Compose

创建 `/opt/zimaos-echo/docker-compose.yml`：

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

运行：
```bash
cd /opt/zimaos-echo
docker-compose up -d
```

### 使用 Systemd（原生二进制）

1. 下载二进制：
```bash
wget https://github.com/IceWhaleTech/ZimaOS-Echo/releases/latest/download/zimaos-echo-linux-amd64
sudo mv zimaos-echo-linux-amd64 /usr/local/bin/zimaos-echo
sudo chmod +x /usr/local/bin/zimaos-echo
```

2. 创建 systemd 服务 `/etc/systemd/system/zimaos-echo.service`：
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

3. 启用并启动：
```bash
sudo systemctl daemon-reload
sudo systemctl enable zimaos-echo
sudo systemctl start zimaos-echo
```

---

## 通用配置

### Home Assistant 集成

各平台均可集成 Home Assistant：

```yaml
homeassistant:
  enabled: true
  url: http://homeassistant.local:8123
  token: your-long-lived-access-token
```

### Ollama 集成（本地 LLM）

与 Echo 一起运行 Ollama：

```yaml
# docker-compose.yml
version: '3.8'
services:
  zimaos-echo:
    image: icewhale/zimaos-echo:latest
    # ... 其他配置 ...
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

### Nginx 反向代理

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

## 故障排除

### 容器无法启动

```bash
# 查看日志
docker logs zimaos-echo

# 检查端口占用
netstat -tlnp | grep 8765

# 检查卷权限
ls -la /path/to/data
```

### 无法访问 Web 界面

1. 确认容器在运行：`docker ps`
2. 检查防火墙规则
3. 确认端口映射
4. 尝试用 IP 而非主机名访问

### 性能问题

- 保证足够内存（至少 512MB）
- 数据目录使用 SSD
- 考虑本地 LLM（Ollama）降低延迟
- 在配置中启用缓存

### 数据库错误

```bash
# 备份并重建数据库
docker exec zimaos-echo cp /app/data/echo.db /app/data/echo.db.bak
docker restart zimaos-echo
```

---

## 支持

- 文档：https://docs.zimaspace.com/echo
- GitHub Issues：https://github.com/IceWhaleTech/ZimaOS-Echo/issues
- Discord：https://discord.gg/zimaos
