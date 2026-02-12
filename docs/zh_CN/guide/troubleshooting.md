# 故障排除指南

[English Version](../../../DEV/troubleshooting.md)

本指南帮助你诊断和解决 ZimaOS-Blue 的常见问题。

## 快速诊断

### 检查服务状态

```bash
# Systemd 服务
sudo systemctl status echo

# Docker 容器
docker ps | grep echo
docker logs zimaos-blue --tail 100

# 直接进程
ps aux | grep echo
```

### 检查日志

```bash
# Systemd 日志
sudo journalctl -u echo -f

# 日志文件（如果已配置）
tail -f /var/log/echo/echo.log

# Docker 日志
docker logs -f zimaos-blue
```

### 健康检查

```bash
curl http://localhost:23456/health
```

预期响应：
```json
{"status":"ok","version":"0.5.0"}
```

---

## 常见问题

### 服务无法启动

#### 端口已被占用

**症状：** 错误消息 "address already in use"

**解决方案：**
```bash
# 查找使用 23456 端口的进程
sudo lsof -i :23456
# 或
sudo netstat -tlnp | grep 23456

# 终止该进程或更改 Echo 的端口
# 在 config.yaml 中：
server:
  port: 8081
```

#### 缺少配置

**症状：** "config file not found" 或 "missing required field"

**解决方案：**
```bash
# 创建最小配置
cat > /etc/echo/config.yaml << 'EOF'
server:
  port: 23456
llm:
  default_provider: "ollama"
  providers:
    ollama:
      base_url: "http://localhost:11434"
      model: "llama2"
auth:
  enabled: false
EOF
```

#### 缺少 JWT 密钥

**症状：** "JWT secret is required" 错误

**解决方案：**
```bash
# 生成并设置 JWT 密钥
export JWT_SECRET=$(openssl rand -base64 32)

# 或添加到配置中
auth:
  jwt:
    secret: "your-32-character-secret-here"
```

#### 权限被拒绝

**症状：** "permission denied" 错误

**解决方案：**
```bash
# 修复文件权限
sudo chown -R echo:echo /var/lib/echo
sudo chmod 755 /var/lib/echo

# 修复二进制文件权限
sudo chmod +x /usr/local/bin/echo
```

---

### 认证问题

#### 无效令牌

**症状：** 401 Unauthorized，显示 "invalid token"

**原因：**
1. 令牌已过期
2. 令牌已被撤销
3. JWT 密钥已更改

**解决方案：**
```bash
# 获取新令牌
curl -X POST http://localhost:23456/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}'

# 或使用刷新令牌
curl -X POST http://localhost:23456/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"your-refresh-token"}'
```

#### API 密钥不工作

**症状：** 使用 API 密钥认证时返回 401

**检查：**
```bash
# 验证 API 密钥格式
# 应该是：ek_xxxxx...

# 检查密钥是否具有所需的权限范围
curl http://localhost:23456/api/v1/apikeys \
  -H "Authorization: Bearer <admin-token>"
```

#### RBAC 权限被拒绝

**症状：** 403 Forbidden

**解决方案：**
```bash
# 检查用户的角色和权限
curl http://localhost:23456/api/v1/auth/me \
  -H "Authorization: Bearer <token>"

# 更新用户角色（仅管理员）
# 或调整 RBAC 配置：
rbac:
  roles:
    - name: user
      permissions: ["chat", "skills.execute", "plugins.list"]
```

---

### LLM 连接问题

#### Ollama 无响应

**症状：** 连接 Ollama 时 "connection refused"

**解决方案：**
```bash
# 检查 Ollama 是否正在运行
curl http://localhost:11434/api/tags

# 启动 Ollama
ollama serve

# 检查模型是否可用
ollama list

# 如果缺少模型则拉取
ollama pull llama2
```

#### OpenAI API 错误

**症状：** OpenAI 返回 401 或 429

**常见原因：**
1. API 密钥无效
2. 超出速率限制
3. 配额不足

**解决方案：**
```bash
# 验证 API 密钥
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"

# 在 https://platform.openai.com/usage 检查使用情况
```

#### Anthropic API 错误

**症状：** 认证或速率限制错误

**解决方案：**
```bash
# 验证 API 密钥格式（应以 sk-ant- 开头）
echo $ANTHROPIC_API_KEY

# 测试 API 密钥
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model":"claude-3-opus-20240229","max_tokens":10,"messages":[{"role":"user","content":"Hi"}]}'
```

#### LLM 响应慢

**症状：** 响应时间长

**解决方案：**
1. 使用更小/更快的模型
2. 减少 max_tokens
3. 使用本地 Ollama 而不是云 API
4. 检查网络延迟

```yaml
llm:
  providers:
    openai:
      model: "gpt-3.5-turbo"  # 比 gpt-4 更快
      max_tokens: 500          # 减少令牌限制
```

---

### 插件问题

#### 插件无法加载

**症状：** 插件未出现在列表中

**检查：**
```bash
# 验证插件目录
ls -la ./plugins/

# 检查插件清单
cat ./plugins/my-plugin/manifest.json

# 检查日志中的加载错误
grep "plugin" /var/log/echo/echo.log
```

**常见问题：**
1. manifest.json 无效
2. 缺少必填字段
3. JavaScript 语法错误

#### 插件执行错误

**症状：** 插件在执行过程中失败

**调试：**
```bash
# 启用调试日志
logging:
  level: "debug"

# 检查插件特定日志
grep "my-plugin" /var/log/echo/echo.log
```

#### JavaScript 插件超时

**症状：** "execution timeout" 错误

**解决方案：**
```yaml
plugins:
  execution_timeout: "30s"  # 增加超时时间
```

---

### 数据库问题

#### 数据库锁定

**症状：** "database is locked" 错误（SQLite）

**解决方案：**
```bash
# 检查是否有多个进程
fuser /var/lib/echo/echo.db

# 在配置中增加忙等待超时
database:
  busy_timeout: "10s"

# 或切换到其他数据库
database:
  type: "postgres"
  dsn: "postgres://user:pass@localhost/echo"
```

#### 数据库损坏

**症状：** "database disk image is malformed"

**解决方案：**
```bash
# 备份当前数据库
cp /var/lib/echo/echo.db /var/lib/echo/echo.db.bak

# 尝试恢复
sqlite3 /var/lib/echo/echo.db ".recover" | sqlite3 /var/lib/echo/echo_new.db

# 或从备份恢复
cp /var/lib/echo/backups/latest/echo.db /var/lib/echo/echo.db
```

---

### 性能问题

#### 内存使用过高

**症状：** 内存消耗持续增长

**解决方案：**
1. 限制并发连接数
2. 减少对话历史大小
3. 启用内存限制

```yaml
server:
  max_connections: 100

chat:
  max_history_messages: 50
```

#### CPU 使用过高

**症状：** CPU 持续 100%

**调试：**
```bash
# 启用性能分析
profiling:
  enabled: true
  endpoint_prefix: "/debug/pprof"

# 获取 CPU 分析
curl http://localhost:23456/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof
```

#### API 响应慢

**症状：** API 延迟 > 1s（不包括 LLM 时间）

**解决方案：**
1. 启用响应缓存
2. 优化数据库查询
3. 检查磁盘 I/O

```bash
# 检查磁盘 I/O
iostat -x 1

# 检查数据库查询时间
logging:
  level: "debug"
  include_sql: true
```

---

### 网络问题

#### CORS 错误

**症状：** 浏览器控制台显示 CORS 错误

**解决方案：**
```yaml
server:
  cors:
    enabled: true
    allowed_origins:
      - "http://localhost:3000"
      - "https://your-domain.com"
    allowed_methods:
      - "GET"
      - "POST"
      - "PUT"
      - "DELETE"
```

#### WebSocket 连接失败

**症状：** WebSocket 升级失败

**检查：**
1. 反向代理配置
2. 防火墙规则
3. WebSocket 支持是否启用

**Nginx 配置：**
```nginx
location /api/v1/ws/ {
    proxy_pass http://localhost:23456;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
}
```

#### SSL/TLS 问题

**症状：** 证书错误

**解决方案：**
```yaml
server:
  tls:
    enabled: true
    cert_file: "/etc/echo/cert.pem"
    key_file: "/etc/echo/key.pem"
```

或使用反向代理（推荐）：
```bash
# 使用 certbot 获取 Let's Encrypt 证书
sudo certbot --nginx -d echo.yourdomain.com
```

---

### 备份/恢复问题

#### 备份失败

**症状：** 备份任务失败

**检查：**
```bash
# 验证备份目录存在且可写
ls -la /var/lib/echo/backups/
mkdir -p /var/lib/echo/backups
chown echo:echo /var/lib/echo/backups

# 检查磁盘空间
df -h /var/lib/echo/
```

#### 恢复失败

**症状：** 无法从备份恢复

**解决方案：**
```bash
# 首先停止服务
sudo systemctl stop echo

# 恢复数据库
cp /var/lib/echo/backups/2024-01-15/echo.db /var/lib/echo/echo.db

# 恢复配置
cp /var/lib/echo/backups/2024-01-15/config.yaml /etc/echo/config.yaml

# 修复权限
chown echo:echo /var/lib/echo/echo.db

# 启动服务
sudo systemctl start echo
```

---

## 诊断命令

### 系统信息

```bash
# Echo 版本
echo --version

# Go 版本（如果从源码构建）
go version

# 系统信息
uname -a
cat /etc/os-release

# 内存
free -h

# 磁盘
df -h
```

### 网络诊断

```bash
# 检查监听端口
sudo netstat -tlnp | grep echo

# 测试连接
curl -v http://localhost:23456/health

# DNS 解析
nslookup api.openai.com
```

### 日志分析

```bash
# 统计最近一小时的错误数
journalctl -u echo --since "1 hour ago" | grep -c "error"

# 查找最常见的错误
journalctl -u echo --since "1 day ago" | grep "error" | sort | uniq -c | sort -rn | head

# 监控特定问题
journalctl -u echo -f | grep -E "(error|panic|fatal)"
```

---

## 获取帮助

如果你无法解决问题：

1. **查看现有问题：** [GitHub Issues](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)

2. **收集诊断信息：**
   ```bash
   echo --version
   cat /etc/echo/config.yaml
   journalctl -u echo --since "1 hour ago" > echo-logs.txt
   ```

3. **提交新问题**，包含：
   - Echo 版本
   - 操作系统和版本
   - 复现步骤
   - 错误消息
   - 相关日志

4. **社区支持：**
   - [Discord](https://discord.gg/zimaos)
   - [论坛](https://forum.zimaos.com)
