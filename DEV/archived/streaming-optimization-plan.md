# Chat 流式响应优化方案

## 问题分析

当前流式响应不是逐字返回，可能原因：
1. **缓冲问题** - 数据被缓冲而不是立即发送
2. **Proxy 缓冲** - CC CLI 或 Proxy 层缓冲数据
3. **网络缓冲** - HTTP 客户端缓冲

## 解决方案

### 1. 立即 Flush 每个 Chunk
```go
// 在 StreamMessage 中，每个 chunk 后立即 flush
fmt.Fprintf(c.Response().Writer, "data: %s\n\n", jsonData)
flusher.Flush()
```

### 2. 禁用所有缓冲
- ✓ 已设置 `X-Accel-Buffering: no`
- ✓ 已设置 `Content-Encoding: identity`
- ✓ 已调用 `flusher.Flush()`

### 3. 检查 Proxy 层
- 确保 CC CLI 不缓冲响应
- 检查 Proxy Handler 是否缓冲

### 4. 优化 Chunk 大小
- 减小 chunk 大小以加快响应
- 避免等待完整句子

## 实施步骤

1. **验证当前流式是否工作**
   - 检查浏览器网络标签
   - 查看是否收到 SSE 事件

2. **添加调试日志**
   - 记录每个 chunk 发送时间
   - 监控 flush 调用

3. **优化 Chunk 处理**
   - 立即发送每个 token
   - 避免批处理

4. **测试端到端流式**
   - 从 Chat → CC CLI → Proxy → 客户端
   - 验证延迟

## 性能指标

- **TTFT (Time To First Token)**: < 100ms
- **Token 延迟**: < 50ms
- **吞吐量**: > 50 tokens/sec
