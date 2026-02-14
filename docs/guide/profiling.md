# Performance Profiling Guide

ZimaOS Blue includes built-in performance profiling capabilities using Go's pprof package. This guide explains how to use these tools for debugging and optimization.

## Enabling pprof

pprof is enabled by default. You can configure it in your `config.yaml`:

```yaml
performance:
  profiling:
    pprof_enabled: true
    pprof_path: /debug/pprof
    metrics_enabled: true
    benchmark_enabled: false
```

## Accessing pprof Endpoints

When enabled, the following endpoints are available:

| Endpoint | Description |
|----------|-------------|
| `/debug/pprof/` | Index page with links to all profiles |
| `/debug/pprof/heap` | Heap memory profile |
| `/debug/pprof/goroutine` | Goroutine stack traces |
| `/debug/pprof/allocs` | Memory allocation profile |
| `/debug/pprof/block` | Blocking profile |
| `/debug/pprof/mutex` | Mutex contention profile |
| `/debug/pprof/profile` | CPU profile (30s by default) |
| `/debug/pprof/trace` | Execution trace |
| `/debug/pprof/threadcreate` | Thread creation profile |

## Common Profiling Tasks

### CPU Profiling

Capture a 30-second CPU profile:

```bash
# Using curl
curl -o cpu.prof http://localhost:23456/debug/pprof/profile?seconds=30

# Using go tool
go tool pprof http://localhost:23456/debug/pprof/profile?seconds=30
```

Analyze the profile:

```bash
go tool pprof cpu.prof

# Interactive commands:
(pprof) top10          # Show top 10 functions by CPU time
(pprof) list funcName  # Show source code for a function
(pprof) web            # Open visualization in browser
```

### Memory Profiling

Capture heap profile:

```bash
curl -o heap.prof http://localhost:23456/debug/pprof/heap

# Or directly analyze
go tool pprof http://localhost:23456/debug/pprof/heap
```

Common analysis commands:

```bash
(pprof) top10 -cum     # Top by cumulative allocation
(pprof) list funcName  # Show allocations in function
(pprof) inuse_space    # Show currently allocated memory
(pprof) alloc_space    # Show total allocations
```

### Goroutine Analysis

Check for goroutine leaks:

```bash
# Get goroutine dump
curl http://localhost:23456/debug/pprof/goroutine?debug=1

# Full stack traces
curl http://localhost:23456/debug/pprof/goroutine?debug=2

# Analyze with pprof
go tool pprof http://localhost:23456/debug/pprof/goroutine
```

### Blocking Profile

Identify blocking operations:

```bash
# Enable block profiling first (in code or config)
# runtime.SetBlockProfileRate(1)

curl -o block.prof http://localhost:23456/debug/pprof/block
go tool pprof block.prof
```

### Mutex Contention

Find mutex bottlenecks:

```bash
# Enable mutex profiling first
# runtime.SetMutexProfileFraction(1)

curl -o mutex.prof http://localhost:23456/debug/pprof/mutex
go tool pprof mutex.prof
```

## Visualization

### Web Interface

Generate SVG visualization:

```bash
go tool pprof -http=:8081 cpu.prof
```

This opens a web interface with:
- Flame graphs
- Call graphs
- Source code view
- Top functions

### Flame Graphs

Generate flame graph:

```bash
# Install go-torch (optional)
go install github.com/uber/go-torch@latest

# Generate flame graph
go-torch -u http://localhost:23456/debug/pprof/profile
```

## Continuous Profiling

For production monitoring, consider:

### 1. Periodic Snapshots

```bash
#!/bin/bash
# Save profiles every hour
while true; do
    timestamp=$(date +%Y%m%d_%H%M%S)
    curl -o "heap_${timestamp}.prof" http://localhost:23456/debug/pprof/heap
    curl -o "goroutine_${timestamp}.prof" http://localhost:23456/debug/pprof/goroutine
    sleep 3600
done
```

### 2. Alerting on Metrics

Monitor key metrics:

```bash
# Get current goroutine count
curl -s http://localhost:23456/debug/pprof/goroutine?debug=1 | head -1

# Get heap stats
curl -s http://localhost:23456/debug/pprof/heap?debug=1 | grep -E "^#"
```

## Best Practices

### 1. Security

**Important**: pprof endpoints expose sensitive information. In production:

```yaml
# Restrict access
performance:
  profiling:
    pprof_enabled: true
    # Use authentication middleware
    # Or bind to localhost only
```

### 2. Performance Impact

- CPU profiling has ~5% overhead
- Memory profiling has minimal overhead
- Block/mutex profiling can have significant overhead

### 3. Baseline Comparison

Always compare against a baseline:

```bash
# Capture baseline
go tool pprof -base=baseline.prof current.prof
```

## Troubleshooting Common Issues

### High Memory Usage

1. Capture heap profile
2. Look for large allocations:
   ```
   (pprof) top10 -cum
   ```
3. Check for memory leaks:
   ```
   (pprof) inuse_objects
   ```

### Goroutine Leaks

1. Get goroutine count over time
2. Identify stuck goroutines:
   ```bash
   curl http://localhost:23456/debug/pprof/goroutine?debug=2 | grep -A 10 "goroutine"
   ```

### High CPU Usage

1. Capture CPU profile during high load
2. Identify hot functions:
   ```
   (pprof) top20
   ```
3. Look for inefficient loops or algorithms

### Slow Requests

1. Enable tracing:
   ```bash
   curl -o trace.out http://localhost:23456/debug/pprof/trace?seconds=5
   go tool trace trace.out
   ```
2. Analyze request latency in trace viewer

## Integration with Monitoring

### Prometheus Metrics

ZimaOS Blue exports pprof-related metrics:

```
# HELP go_goroutines Number of goroutines
# TYPE go_goroutines gauge
go_goroutines 42

# HELP go_memstats_heap_alloc_bytes Heap bytes allocated
# TYPE go_memstats_heap_alloc_bytes gauge
go_memstats_heap_alloc_bytes 1234567
```

### Grafana Dashboard

Import the included Grafana dashboard for visualizing:
- Goroutine count over time
- Memory usage trends
- GC pause times
- Request latencies

## Additional Resources

- [Go pprof documentation](https://pkg.go.dev/net/http/pprof)
- [Profiling Go Programs](https://go.dev/blog/pprof)
- [Go Memory Management](https://go.dev/doc/gc-guide)
