# 默认端口变更说明

## 变更内容

将 ZimaOS-Blue 的默认 HTTP 端口从 `80` 改为 `80`。

## 端口占用处理逻辑

当端口 80 被占用时，系统会自动执行以下检测流程：

### 1. 健康检查
向 `http://0.0.0.0:80/api/v1/health` 发送 GET 请求，检查响应内容。

### 2. 服务识别
检查健康端点返回的 JSON 中是否包含：
```json
{
  "status": "ok",
  "service": "zimaos-blue",
  ...
}
```

### 3. 处理策略

- **如果是 ZimaOS-Blue**：
  - 日志输出：`Existing ZimaOS-Blue server already running on port, reusing`
  - 复用现有服务，不启动新实例

- **如果不是 ZimaOS-Blue**：
  - 日志输出：`Port in use by another process, falling back to random port`
  - 自动切换到随机可用端口（需要 `port_auto_fallback: true`）

## 配置选项

### 环境变量
```bash
# 覆盖默认端口
export BLUE_SERVER_PORT=8080
```

### 配置文件 (config.yaml)
```yaml
server:
  host: "0.0.0.0"
  port: 80
  port_auto_fallback: true  # 启用端口自动回退
```

## 相关文件

- `server/internal/config/config.go` - 默认配置（第 497 行）
- `server/internal/server/server.go` - 端口占用检测逻辑（第 93-123 行）
- `server/internal/config/config_test.go` - 测试用例更新

## 测试

```bash
# 测试默认配置
go test -v ./internal/config -run TestLoad_Defaults

# 构建验证
go build -o /dev/null ./cmd/blue
```

## 注意事项

1. 端口 80 需要 root 权限（Linux/macOS）
2. 如果无法绑定 80 端口且 `port_auto_fallback` 为 false，服务将启动失败
3. 建议在生产环境启用 `port_auto_fallback: true` 以提高可用性
