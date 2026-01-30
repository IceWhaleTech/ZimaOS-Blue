# PRD: Go CGO 嵌入方案

## 概述

将 Go Echo 服务器编译为 C 共享库（.dylib/.dll/.so），嵌入到 Tauri Rust 应用中，通过 FFI 调用，实现单一进程架构，消除进程间通信开销，加快启动速度。

## 目标

1. **保持向后兼容**: 原有的 `go build ./cmd/echo` 仍然可以编译出独立的 CLI 服务器
2. **新增库模式**: 支持 `go build -buildmode=c-shared` 编译为共享库
3. **统一代码库**: 核心逻辑只维护一份，CLI 和库模式共享
4. **启动时间优化**: 消除进程启动和健康检查轮询的开销

## 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                    Tauri Application                         │
│  ┌─────────────────────────────────────────────────────────┐│
│  │                    Rust Layer                            ││
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐ ││
│  │  │   lib.rs    │  │  server.rs  │  │   echolib.rs    │ ││
│  │  │  (Tauri)    │  │  (Sidecar)  │  │   (FFI 绑定)    │ ││
│  │  └─────────────┘  └─────────────┘  └────────┬────────┘ ││
│  └─────────────────────────────────────────────┼───────────┘│
│                                                │             │
│  ┌─────────────────────────────────────────────▼───────────┐│
│  │              libecho.dylib / echo.dll / libecho.so       ││
│  │  ┌─────────────────────────────────────────────────────┐││
│  │  │                  CGO Export Layer                    │││
│  │  │  EchoServerStart() | EchoServerStop() | ...         │││
│  │  └─────────────────────────────────────────────────────┘││
│  │  ┌─────────────────────────────────────────────────────┐││
│  │  │                   echocore Package                   │││
│  │  │  Server | Config | Lifecycle | Services             │││
│  │  └─────────────────────────────────────────────────────┘││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                    CLI Mode (独立运行)                        │
│  ┌─────────────────────────────────────────────────────────┐│
│  │                    cmd/echo/main.go                      ││
│  │  main() → echocore.Run() → 信号处理 → 优雅关闭           ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

## 代码结构

```
server/
├── cmd/
│   ├── echo/           # CLI 入口 (保持不变)
│   │   ├── main.go     # 调用 echocore.Run()
│   │   └── service_*.go
│   └── echolib/        # CGO 库入口 (新增)
│       └── exports.go  # CGO 导出函数
├── internal/
│   ├── echocore/       # 核心逻辑 (从 main.go 提取)
│   │   ├── server.go   # Server 结构体和生命周期
│   │   ├── config.go   # 配置加载
│   │   ├── services.go # 服务初始化
│   │   └── routes.go   # 路由注册
│   └── ...             # 其他现有包
```

## 实现阶段

### Phase 1: 代码重构 (不改变功能)

**目标**: 将 `cmd/echo/main.go` 中的核心逻辑提取到 `internal/echocore` 包

**步骤**:
1. 创建 `internal/echocore/server.go` - Server 结构体
2. 创建 `internal/echocore/config.go` - 配置相关
3. 创建 `internal/echocore/services.go` - 服务初始化
4. 创建 `internal/echocore/routes.go` - 路由注册
5. 修改 `cmd/echo/main.go` 调用 echocore
6. 验证 CLI 模式正常工作

**验收标准**:
- `go build ./cmd/echo` 编译成功
- 服务器功能与重构前完全一致
- 所有测试通过

### Phase 2: CGO 导出层

**目标**: 创建 C 兼容的导出函数

**步骤**:
1. 创建 `cmd/echolib/exports.go` - CGO 导出
2. 定义 C 头文件接口
3. 实现回调机制
4. 添加构建脚本

**导出函数**:
```c
// 生命周期
int EchoServerStart(uint16_t port, const char* config_path);
int EchoServerStop(void);
int EchoServerIsRunning(void);

// 状态查询
uint16_t EchoServerGetPort(void);
const char* EchoServerGetVersion(void);

// 回调设置
void EchoServerSetCallbacks(
    void (*ready)(uint16_t port),
    void (*error)(const char* msg),
    void (*stopped)(void)
);

// 内存管理
void EchoServerFreeString(char* s);
```

**验收标准**:
- `go build -buildmode=c-shared` 编译成功
- 生成 .h 头文件
- 简单 C 程序可以调用

### Phase 3: Rust FFI 绑定

**目标**: 在 Tauri 中创建 Rust 绑定

**步骤**:
1. 创建 `tauri-app/src-tauri/src/echolib.rs`
2. 使用 `extern "C"` 声明外部函数
3. 创建安全的 Rust 包装器
4. 处理回调和线程安全

**Rust 接口**:
```rust
pub struct EchoServer {
    // ...
}

impl EchoServer {
    pub fn start(port: u16, config_path: Option<&str>) -> Result<Self, Error>;
    pub fn stop(&mut self) -> Result<(), Error>;
    pub fn is_running(&self) -> bool;
    pub fn port(&self) -> u16;
    pub fn version() -> String;
}
```

**验收标准**:
- Rust 代码编译成功
- 可以启动/停止服务器
- 回调正常工作

### Phase 4: Tauri 集成

**目标**: 修改 Tauri 应用使用嵌入库

**步骤**:
1. 修改 `lib.rs` 使用 echolib
2. 更新 `server.rs` 支持两种模式
3. 添加功能开关 (feature flag)
4. 更新构建脚本

**功能开关**:
```toml
[features]
default = ["embedded-server"]
embedded-server = []  # 使用嵌入库
sidecar-server = []   # 使用独立进程
```

**验收标准**:
- 两种模式都可以正常工作
- 嵌入模式启动更快
- 无功能回归

### Phase 5: 构建系统

**目标**: 更新 Makefile 支持新的构建模式

**新增目标**:
```makefile
# 构建共享库
build-lib-darwin-arm64:
    cd $(SERVER_DIR) && CGO_ENABLED=1 go build -buildmode=c-shared \
        -o $(DIST_DIR)/libecho-darwin-arm64.dylib ./cmd/echolib

# Tauri 嵌入模式构建
tauri-build-embedded: build-lib-$(PLATFORM)
    cd $(TAURI_DIR) && cargo build --release --features embedded-server
```

## 风险和缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| CGO 跨平台编译复杂 | 高 | 使用 CI 矩阵构建，每个平台独立编译 |
| 内存泄漏 | 中 | 严格的内存管理，提供 Free 函数 |
| 回调线程安全 | 中 | 使用 channel 或 mutex 保护 |
| 调试困难 | 中 | 保留 sidecar 模式作为调试选项 |
| 二进制体积增大 | 低 | 使用 LTO 和 strip 优化 |

## 时间估算

| 阶段 | 预计工作量 |
|------|-----------|
| Phase 1: 代码重构 | 2-3 小时 |
| Phase 2: CGO 导出 | 1-2 小时 |
| Phase 3: Rust FFI | 2-3 小时 |
| Phase 4: Tauri 集成 | 1-2 小时 |
| Phase 5: 构建系统 | 1 小时 |
| 测试和调试 | 2-3 小时 |
| **总计** | **9-14 小时** |

## 成功指标

1. **启动时间**: 从 5-17 秒减少到 1-3 秒
2. **兼容性**: CLI 模式 100% 功能保持
3. **稳定性**: 无内存泄漏，无崩溃
4. **可维护性**: 核心代码只维护一份

## 下一步

确认此 PRD 后，开始 Phase 1 代码重构。
