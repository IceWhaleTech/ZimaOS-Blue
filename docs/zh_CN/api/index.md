# API 参考

ZimaOS Blue 提供 RESTful API 用于监控和管理。

## 基础 URL

```
http://localhost:23456
```

## 端点

| 方法 | 路径 | 描述 |
|--------|------|-------------|
| GET | `/health` | 完整健康状态 |
| GET | `/health/live` | 存活探针 |
| GET | `/health/ready` | 就绪探针 |
| GET | `/api/v1/workers/stats` | 工作池统计 |

## 认证

目前 API 不需要认证。认证功能将在 v0.4.0 版本中添加。

## 响应格式

所有响应均为 JSON 格式：

```json
{
  "status": "ok",
  "data": { ... }
}
```

## 错误响应

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "可读的错误信息"
  }
}
```

## 速率限制

目前未实现速率限制。此功能将在未来版本中添加。
