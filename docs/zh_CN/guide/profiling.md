# 性能剖析指南

ZimaOS Echo 内置基于 Go pprof 的性能剖析能力。本指南说明如何用这些工具进行调试与优化。

## 启用 pprof

pprof 默认启用。可在 `config.yaml` 中配置：

```yaml
performance:
  profiling:
    pprof_enabled: true
    pprof_path: /debug/pprof
    metrics_enabled: true
    benchmark_enabled: false
```

## 访问 pprof 端点

启用后可使用以下端点：

| 端点 | 说明 |
|----------|-------------|
| `/debug/pprof/` | 索引页，链接到所有剖析 |
| `/debug/pprof/heap` | 堆内存剖析 |
| `/debug/pprof/goroutine` | Goroutine 栈跟踪 |
| `/debug/pprof/allocs` | 内存分配剖析 |
| `/debug/pprof/block` | 阻塞剖析 |
| `/debug/pprof/mutex` | 互斥锁争用剖析 |
| `/debug/pprof/profile` | CPU 剖析（默认 30 秒） |
| `/debug/pprof/trace` | 执行跟踪 |
| `/debug/pprof/threadcreate` | 线程创建剖析 |

## 常用剖析任务

### CPU 剖析

采集 30 秒 CPU 剖析：

```bash
# 使用 curl
curl -o cpu.prof http://localhost:23456/debug/pprof/profile?seconds=30

# 使用 go tool
go tool pprof http://localhost:23456/debug/pprof/profile?seconds=30
```

分析剖析文件：

```bash
go tool pprof cpu.prof

# 交互命令：
(pprof) top10          # 按 CPU 时间显示前 10 个函数
(pprof) list funcName  # 显示函数源码
(pprof) web            # 在浏览器中打开可视化
```

### 内存剖析

采集堆剖析：

```bash
curl -o heap.prof http://localhost:23456/debug/pprof/heap

# 或直接分析
go tool pprof http://localhost:23456/debug/pprof/heap
```

常用分析命令：

```bash
(pprof) top10 -cum     # 按累计分配排序
(pprof) list funcName  # 显示函数内分配
(pprof) inuse_space    # 当前已分配内存
(pprof) alloc_space    # 总分配量
```

### Goroutine 分析

检查 goroutine 泄漏：

```bash
# 获取 goroutine 转储
curl http://localhost:23456/debug/pprof/goroutine?debug=1

# 完整栈跟踪
curl http://localhost:23456/debug/pprof/goroutine?debug=2

# 用 pprof 分析
go tool pprof http://localhost:23456/debug/pprof/goroutine
```

### 阻塞剖析

定位阻塞操作：

```bash
# 需先启用阻塞剖析（在代码或配置中）
# runtime.SetBlockProfileRate(1)

curl -o block.prof http://localhost:23456/debug/pprof/block
go tool pprof block.prof
```

### 互斥锁争用

查找互斥锁瓶颈：

```bash
# 需先启用互斥剖析
# runtime.SetMutexProfileFraction(1)

curl -o mutex.prof http://localhost:23456/debug/pprof/mutex
go tool pprof mutex.prof
```

## 可视化

### Web 界面

生成 SVG 可视化：

```bash
go tool pprof -http=:8081 cpu.prof
```

会打开 Web 界面，包含：
- 火焰图
- 调用图
- 源码视图
- Top 函数

### 火焰图

生成火焰图：

```bash
# 安装 go-torch（可选）
go install github.com/uber/go-torch@latest

# 生成火焰图
go-torch -u http://localhost:23456/debug/pprof/profile
```

## 持续剖析

生产环境监控可考虑：

### 1. 定期快照

```bash
#!/bin/bash
# 每小时保存一次剖析
while true; do
    timestamp=$(date +%Y%m%d_%H%M%S)
    curl -o "heap_${timestamp}.prof" http://localhost:23456/debug/pprof/heap
    curl -o "goroutine_${timestamp}.prof" http://localhost:23456/debug/pprof/goroutine
    sleep 3600
done
```

### 2. 指标告警

监控关键指标：

```bash
# 当前 goroutine 数量
curl -s http://localhost:23456/debug/pprof/goroutine?debug=1 | head -1

# 堆统计
curl -s http://localhost:23456/debug/pprof/heap?debug=1 | grep -E "^#"
```

## 最佳实践

### 1. 安全

**重要**：pprof 端点会暴露敏感信息。生产环境建议：

```yaml
# 限制访问
performance:
  profiling:
    pprof_enabled: true
    # 使用认证中间件
    # 或仅绑定 localhost
```

### 2. 性能影响

- CPU 剖析约 5% 开销
- 内存剖析开销较小
- 阻塞/互斥剖析可能有明显开销

### 3. 基线对比

始终与基线对比：

```bash
# 对比基线
go tool pprof -base=baseline.prof current.prof
```

## 常见问题排查

### 内存占用高

1. 采集堆剖析
2. 查找大块分配：
   ```
   (pprof) top10 -cum
   ```
3. 检查内存泄漏：
   ```
   (pprof) inuse_objects
   ```

### Goroutine 泄漏

1. 随时间观察 goroutine 数量
2. 找出卡住的 goroutine：
   ```bash
   curl http://localhost:23456/debug/pprof/goroutine?debug=2 | grep -A 10 "goroutine"
   ```

### CPU 占用高

1. 在高负载时采集 CPU 剖析
2. 找出热点函数：
   ```
   (pprof) top20
   ```
3. 检查低效循环或算法

### 请求慢

1. 启用跟踪：
   ```bash
   curl -o trace.out http://localhost:23456/debug/pprof/trace?seconds=5
   go tool trace trace.out
   ```
2. 在跟踪查看器中分析请求延迟

## 与监控集成

### Prometheus 指标

ZimaOS Echo 导出与 pprof 相关的指标：

```
# HELP go_goroutines Goroutine 数量
# TYPE go_goroutines gauge
go_goroutines 42

# HELP go_memstats_heap_alloc_bytes 已分配堆字节数
# TYPE go_memstats_heap_alloc_bytes gauge
go_memstats_heap_alloc_bytes 1234567
```

### Grafana 仪表盘

可导入提供的 Grafana 仪表盘，用于查看：
- Goroutine 数量随时间变化
- 内存使用趋势
- GC 暂停时间
- 请求延迟

## 延伸阅读

- [Go pprof 文档](https://pkg.go.dev/net/http/pprof)
- [剖析 Go 程序](https://go.dev/blog/pprof)
- [Go 内存管理](https://go.dev/doc/gc-guide)
