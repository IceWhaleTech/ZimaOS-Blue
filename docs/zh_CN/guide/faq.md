# 常见问题（FAQ）

## 一般问题

### 什么是 ZimaOS Blue？

ZimaOS Blue 是面向家庭服务器和 NAS 设备的 AI 助手，提供：
- 自然语言聊天界面
- 通过 Home Assistant 的智能家居控制
- 多通道消息（Telegram、Discord 等）
- 语音助手能力
- 浏览器自动化
- 可扩展的插件系统

### 支持哪些 LLM 提供商？

ZimaOS Blue 支持多种 LLM 提供商：
- **OpenAI**：GPT-4o、GPT-4o-mini、GPT-4-turbo、GPT-3.5-turbo
- **Anthropic**：Claude 3.5 Sonnet、Claude 3 Opus、Claude 3 Haiku
- **Ollama**：本地模型（Llama 3.2、Mistral、CodeLlama 等）
- **自定义**：任意 OpenAI 兼容 API

### 系统要求是什么？

**最低配置：**
- CPU：2 核
- 内存：512MB
- 存储：1GB
- 系统：Linux (amd64/arm64)、Windows、macOS

**推荐配置：**
- CPU：4 核
- 内存：2GB
- 存储：10GB
- 数据库使用 SSD

### ZimaOS Blue 免费吗？

是的，ZimaOS Blue 在 Apache 2.0 许可下开源。但您可能产生以下费用：
- 云 LLM API 使用（OpenAI、Anthropic）
- 云托管（若非自托管）

使用 Ollama 本地模型完全免费。

---

## 安装

### 如何安装 ZimaOS Blue？

**Docker（推荐）：**
```bash
docker run -d \
  --name zimaos-blue \
  -p 23456:23456 \
  -v echo-data:/app/data \
  icewhale/zimaos-blue:latest
```

**二进制：**
```bash
# 从发布页下载
wget https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download/zimaos-blue-linux-amd64
chmod +x zimaos-blue-linux-amd64
./zimaos-blue-linux-amd64
```

### 如何更新 ZimaOS Blue？

**Docker：**
```bash
docker pull icewhale/zimaos-blue:latest
docker stop zimaos-blue
docker rm zimaos-blue
# 使用相同卷挂载重新运行
```

**二进制：**
下载新版本并替换二进制文件。数据目录中的数据会保留。

### 能在树莓派上运行 ZimaOS Blue 吗？

可以。ZimaOS Blue 支持 ARM64 架构。建议：
- 使用 4GB+ 内存的树莓派 4
- 使用 Ollama 搭配较小模型（Phi-3、TinyLlama）
- 如需要可启用 swap

---

## 配置

### 配置文件在哪里？

默认位置：
- `/etc/zimaos-blue/config.yaml`
- `./config.yaml`（当前目录）
- `~/.config/zimaos-blue/config.yaml`

或通过参数指定：`zimaos-blue --config /path/to/config.yaml`

### 如何配置多个 LLM 提供商？

```yaml
llm:
  default_provider: openai
  providers:
    openai:
      api_key: sk-...
      model: gpt-4o-mini
    anthropic:
      api_key: sk-ant-...
      model: claude-3-5-sonnet-20241022
    ollama:
      base_url: http://localhost:11434
      model: llama3.2
```

### 如何启用 HTTPS？

方式一：使用反向代理（推荐）
```nginx
server {
    listen 443 ssl;
    server_name echo.example.com;

    ssl_certificate /etc/letsencrypt/live/echo.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/echo.example.com/privkey.pem;

    location / {
        proxy_pass http://localhost:23456;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

方式二：直接 TLS
```yaml
server:
  tls:
    enabled: true
    cert_file: /path/to/cert.pem
    key_file: /path/to/key.pem
```

### 如何修改默认端口？

```yaml
server:
  port: 8081
```

或通过环境变量：
```bash
export BLUE_SERVER_PORT=8081
```

---

## LLM 与聊天

### 该用哪个模型？

| 场景 | 推荐模型 |
|----------|------------------|
| 日常聊天 | GPT-4o-mini、Claude 3 Haiku |
| 复杂任务 | GPT-4o、Claude 3.5 Sonnet |
| 注重隐私 | Ollama（本地） |
| 低延迟 | GPT-3.5-turbo、本地模型 |
| 成本优先 | Ollama、GPT-4o-mini |

### 如何用 Ollama 使用本地模型？

1. 安装 Ollama：https://ollama.ai
2. 拉取模型：`ollama pull llama3.2`
3. 配置 Echo：
```yaml
llm:
  default_provider: ollama
  providers:
    ollama:
      base_url: http://localhost:11434
      model: llama3.2
```

### 为什么回复很慢？

常见原因：
1. 访问云 API 的**网络延迟**
2. **大模型**（GPT-4 比 GPT-3.5 慢）
3. **长上下文**（token 越多越慢）
4. 提供商的**限流**

建议：
- 使用流式输出获得实时反馈
- 选用更快模型（GPT-4o-mini、本地模型）
- 降低 max_tokens
- 使用本地 Ollama 降低延迟

### 如何限制 token 使用？

```yaml
llm:
  max_tokens: 1000
  max_context_tokens: 4000
```

### 能用自己的微调模型吗？

可以，只要兼容 OpenAI API：
```yaml
llm:
  providers:
    custom:
      base_url: https://your-api.com/v1
      api_key: your-key
      model: your-fine-tuned-model
```

---

## 智能家居

### 如何连接 Home Assistant？

1. 在 Home Assistant 创建长期访问令牌：
   - 个人资料 → 长期访问令牌 → 创建令牌

2. 在 Echo 中配置：
```yaml
homeassistant:
  enabled: true
  url: http://homeassistant.local:8123
  token: your-long-lived-token
```

### 语音/聊天能控制什么？

- 灯光（开关、亮度、颜色）
- 开关与插座
- 温控器
- 门锁
- 窗帘、车库门
- 媒体播放器
- 场景与自动化
- 任意 Home Assistant 实体

### 示例命令？

- 「打开客厅灯」
- 「把卧室温度设为 72」
- 「锁上前门」
- 「厨房温度多少？」
- 「执行电影夜场景」

---

## 通道与消息

### 如何配置 Telegram？

1. 用 @BotFather 创建机器人
2. 获取机器人令牌
3. 配置：
```yaml
channels:
  telegram:
    enabled: true
    token: "123456789:ABCdefGHIjklMNOpqrsTUVwxyz"
```

### 如何配置 Discord？

1. 在 https://discord.com/developers 创建应用
2. 创建 Bot 并获取令牌
3. 配置：
```yaml
channels:
  discord:
    enabled: true
    token: "your-discord-bot-token"
```

### 能同时用多个通道吗？

可以，按需启用：
```yaml
channels:
  telegram:
    enabled: true
  discord:
    enabled: true
  slack:
    enabled: true
```

---

## 安全

### 数据安全吗？

- 数据存储在您自己的服务器
- API 密钥加密存储
- 除 LLM 提供商外不向第三方发送数据
- 支持 HTTPS

### 如何启用认证？

```yaml
security:
  users:
    allow_registration: false
  password:
    min_length: 12
    require_uppercase: true
    require_number: true
```

### 如何启用 MFA？

```yaml
security:
  mfa:
    enabled: true
    required: false  # 设为 true 则强制 MFA
```

### 如何重置管理员密码？

```bash
zimaos-blue reset-password --username admin
```

---

## 备份与恢复

### 如何备份数据？

**自动备份：**
```yaml
backup:
  enabled: true
  schedule: "0 2 * * *"  # 每天凌晨 2 点
  retention_days: 7
```

**手动备份：**
```bash
curl -X POST http://localhost:23456/api/backup/create \
  -H "Authorization: Bearer <token>" \
  -d '{"type": "full"}'
```

### 如何从备份恢复？

```bash
curl -X POST http://localhost:23456/api/backup/restore/<backup-id> \
  -H "Authorization: Bearer <token>"
```

### 备份存在哪里？

默认：`/var/lib/zimaos-blue/backups/`

配置：
```yaml
backup:
  path: /custom/backup/path
```

---

## 故障排除

### 如何开启调试日志？

```yaml
log:
  level: debug
```

### 如何检查服务状态？

```bash
curl http://localhost:23456/health
```

### 哪里可以获得帮助？

- 文档：https://docs.zimaspace.com/echo
- GitHub Issues：https://github.com/IceWhaleTech/ZimaOS-Blue/issues
- Discord：https://discord.gg/zimaos

---

## 开发

### 如何从源码构建？

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue/server
go build -o zimaos-blue ./cmd/server
```

### 如何运行测试？

```bash
cd server
go test ./... -v
```

### 如何参与贡献？

1. Fork 仓库
2. 创建功能分支
3. 修改代码
4. 运行测试
5. 提交 Pull Request

详见 [CONTRIBUTING.md](../CONTRIBUTING.md)。
