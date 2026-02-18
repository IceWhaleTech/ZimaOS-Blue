# 优雅重启实现

## 功能概述

实现了智能端口占用检测和优雅重启功能。当检测到端口被旧的 ZimaOS-Blue 实例占用时，新实例会通知旧实例优雅退出，然后复用端口启动。

## 实现细节

### 1. Server 结构体扩展

```go
type Server struct {
	echo           *echo.Echo
	config         *config.ServerConfig
	shutdownChan   chan struct{}      // 关闭信号通道
	httpServer     *http.Server       // HTTP 服务器引用
	shutdownMu     sync.Mutex         // 关闭互斥锁
	isShuttingDown bool               // 关闭状态标志
}
```

### 2. 优雅关闭方法

```go
func (s *Server) Shutdown(ctx context.Context) error {
	s.shutdownMu.Lock()
	if s.isShuttingDown {
		s.shutdownMu.Unlock()
		return fmt.Errorf("shutdown already in progress")
	}
	s.isShuttingDown = true
	s.shutdownMu.Unlock()

	logger.Info().Msg("Initiating graceful shutdown")

	// Signal shutdown
	close(s.shutdownChan)

	// Shutdown HTTP server if it exists
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			logger.Error().Err(err).Msg("Error during HTTP server shutdown")
			return err
		}
	}

	logger.Info().Msg("Server shutdown complete")
	return nil
}
```

### 3. 关闭 API 端点

```go
func (s *Server) RegisterShutdownRoute() {
	s.echo.POST("/api/v1/shutdown", func(c echo.Context) error {
		logger.Info().Msg("Received shutdown request")

		// Start shutdown in background
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := s.Shutdown(ctx); err != nil {
				logger.Error().Err(err).Msg("Shutdown failed")
			}
		}()

		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":  "ok",
			"message": "Server shutting down gracefully",
		})
	})
}
```

### 4. 智能端口占用处理

```go
func requestGracefulShutdown(host string, port int) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	url := fmt.Sprintf("http://%s:%d/api/v1/shutdown", host, port)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	// Wait a bit for the old server to shutdown
	time.Sleep(1 * time.Second)
	return true
}
```

### 5. 启动流程

```go
ln, err := lc.Listen(context.Background(), "tcp", addr)
if err != nil {
	if isAddrInUse(err) {
		// Check if it's our own previous instance
		if checkExistingServer(s.config.Host, s.config.Port) {
			logger.Info().
				Int("port", s.config.Port).
				Msg("Detected existing ZimaOS-Blue server, requesting graceful shutdown")

			// Request graceful shutdown of the old instance
			if requestGracefulShutdown(s.config.Host, s.config.Port) {
				logger.Info().Msg("Old instance shutdown successfully, reusing port")

				// Retry listening after old instance shutdown
				ln, err = lc.Listen(context.Background(), "tcp", addr)
				if err != nil {
					return fmt.Errorf("failed to listen after graceful shutdown: %w", err)
				}
			} else {
				logger.Warn().Msg("Failed to shutdown old instance, will reuse connection")
				// Fallback: just reuse the existing server
				actualPort.Store(int32(s.config.Port))
				security.SetServerPort(s.config.Port)
				network.SetDynamicPort(s.config.Port)
				return nil
			}
		} else {
			// Not our server — fallback to random port if enabled
			if s.config.PortAutoFallback {
				logger.Warn().
					Int("configured_port", s.config.Port).
					Msg("Port in use by another process, falling back to random port")
				randomAddr := fmt.Sprintf("%s:0", s.config.Host)
				ln, err = lc.Listen(context.Background(), "tcp", randomAddr)
				if err != nil {
					return fmt.Errorf("failed to create listener on random port: %w", err)
				}
			} else {
				return fmt.Errorf("failed to create listener: %w", err)
			}
		}
	} else {
		return fmt.Errorf("failed to create listener: %w", err)
	}
}
```

## 工作流程

1. **新实例启动**：尝试绑定配置的端口（默认 80）
2. **端口占用检测**：如果端口被占用，检查是否为 ZimaOS-Blue
3. **健康检查**：通过 `/api/v1/health` 端点验证服务标识
4. **优雅关闭请求**：向旧实例发送 `POST /api/v1/shutdown`
5. **等待关闭**：等待 1 秒让旧实例完成关闭
6. **重试绑定**：新实例重新尝试绑定端口
7. **启动成功**：新实例成功启动，实现零停机更新

## 优势

1. **零停机更新**：新旧实例平滑切换
2. **优雅关闭**：旧实例有时间完成正在处理的请求
3. **智能回退**：如果优雅关闭失败，回退到复用连接
4. **端口自动切换**：如果是其他进程占用，自动切换到随机端口

## 使用方法

### 注册关闭端点

在 `main.go` 中注册关闭路由：

```go
srv := server.New(&cfg.Server)
srv.RegisterShutdownRoute()  // 注册优雅关闭端点
srv.RegisterHealthRoutes()
```

### 手动触发关闭

```bash
curl -X POST http://localhost/api/v1/shutdown
```

### 自动优雅重启

直接启动新实例，系统会自动处理：

```bash
./blue  # 新实例会自动通知旧实例退出
```

## 配置选项

```yaml
server:
  host: "0.0.0.0"
  port: 80
  port_auto_fallback: true  # 启用端口自动回退
```

## 日志示例

```
INFO Starting HTTP server addr=0.0.0.0:80
INFO Detected existing ZimaOS-Blue server, requesting graceful shutdown port=80
INFO Old instance shutdown successfully, reusing port
INFO Server listening actual_port=80 configured_port=80
```

## 相关文件

- `server/internal/server/server.go` - 核心实现
- `server/internal/config/config.go` - 配置（默认端口 80）
- `server/docs/PORT-CHANGE.md` - 端口变更文档
