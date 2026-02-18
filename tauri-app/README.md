# ZimaOS Blue - Tauri Desktop App

Tauri v2 桌面应用，通过 FFI 直接链接 Go 静态库（libblue.a），无需 sidecar 进程。

## 架构

```
┌─────────────────────────────────────────┐
│           ZimaOS Blue.app               │
│  ┌───────────────────────────────────┐  │
│  │     Tauri Shell (Rust)            │  │
│  │  - 窗口管理 (WebView)             │  │
│  │  - 系统托盘                       │  │
│  │  - 生命周期管理                    │  │
│  └───────────────┬───────────────────┘  │
│                  │ FFI (C ABI)          │
│  ┌───────────────▼───────────────────┐  │
│  │     libblue.a (Go c-archive)      │  │
│  │  - HTTP API 服务 (:80)          │  │
│  │  - LLM 代理 / 会话管理             │  │
│  │  - STT (Whisper) / TTS (eSpeak)   │  │
│  │  - 安全 / 插件 / 工作流            │  │
│  └───────────────────────────────────┘  │
│  ┌───────────────────────────────────┐  │
│  │     Vue 3 Frontend (WebView)      │  │
│  └───────────────────────────────────┘  │
└─────────────────────────────────────────┘
```

### 组件说明

| 组件 | 技术栈 | 作用 |
|------|--------|------|
| **Tauri Shell** | Rust/Tauri v2 | 桌面壳，窗口、托盘、FFI 调用 |
| **libblue.a** | Go (c-archive) | 后端服务，通过 FFI 嵌入，处理所有 API 和业务逻辑 |
| **Frontend** | Vue 3 + Tailwind | WebView 中的前端界面 |

### FFI 接口

Go 静态库导出以下 C 函数（见 `server/cmd/bluelib/main.go`）：

| 函数 | 说明 |
|------|------|
| `BlueServerStartWithArgs(port, dataDir, args)` | 启动服务（带参数） |
| `BlueServerStart(port, dataDir)` | 启动服务 |
| `BlueServerStop()` | 停止服务 |
| `BlueServerIsRunning()` | 检查运行状态 |
| `BlueServerGetVersion()` | 获取版本号 |
| `BlueServerFreeString(s)` | 释放 Go 分配的字符串 |

Rust 侧绑定见 `src-tauri/src/echo_ffi.rs`。

## 开发

### 前置要求

- Node.js 20+
- Rust (stable)
- Go 1.23+ (CGO_ENABLED=1)
- CMake 3.16+（构建 whisper.cpp、opus、espeak-ng）

### 构建 libblue.a

```bash
# 从项目根目录构建第三方库 + Go 静态库
./build.sh build

# 或手动构建
cd server
CGO_ENABLED=1 go build -buildmode=c-archive -o ../tauri-app/src-tauri/lib/libblue.a ./cmd/bluelib/
```

### 开发模式

```bash
# 从项目根目录
make tauri-dev

# 或直接在 tauri-app 目录
npm install
npm run dev
```

### 构建生产版本

```bash
# 推荐方式（完整构建脚本）
make tauri-package

# 或标准构建
make tauri-build
```

### 清理构建产物

```bash
make tauri-clean
```

## 目录结构

```
tauri-app/
├── src-tauri/
│   ├── src/
│   │   ├── lib.rs       # 主入口，窗口和托盘设置
│   │   ├── echo_ffi.rs  # FFI 绑定（Rust → Go 静态库）
│   │   ├── server.rs    # 服务管理（启动/停止/状态）
│   │   └── tray.rs      # 系统托盘
│   ├── lib/             # libblue.a 存放目录
│   ├── build.rs         # 链接配置（静态库 + 系统框架）
│   ├── icons/           # 应用图标
│   ├── Cargo.toml       # Rust 依赖
│   └── tauri.conf.json  # Tauri 配置
├── build.sh             # 完整构建脚本
└── package.json         # Node.js 依赖
```

## 链接依赖

`build.rs` 负责链接以下静态库和系统框架：

| 库 | 来源 | 用途 |
|----|------|------|
| libblue.a | Go c-archive | 后端服务 |
| libwhisper.a + libggml*.a | third_party/whisper.cpp | 语音识别 |
| libopus.a | third_party/opus-src | 音频编解码 |
| libespeak-ng.a | third_party/espeak-ng | 语音合成 |
| Accelerate, Metal | macOS 系统 | GPU 加速 |
| CoreFoundation, Security | macOS 系统 | 系统服务 |

## 跨平台构建

本地只能构建当前平台的包。跨平台构建请使用 GitHub Actions：

```bash
# 推送代码后，在 GitHub Actions 手动触发
# Actions → Build Tauri App → Run workflow → 选择平台
```

支持的平台：
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: arm64 (Apple Silicon) + x64 (Intel)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: x64
- **Linux**: x64 (AppImage, deb)

## 配置

主要配置文件：`src-tauri/tauri.conf.json`

- `app.windows`: 窗口配置（大小、标题等）
- `app.trayIcon`: 托盘图标配置

## 故障排除

### 服务未启动

1. 检查日志：应用启动时会输出到 stderr
2. 检查端口：`lsof -i :80`
3. 手动测试：`curl http://localhost/api/v1/health`

### 构建失败：libblue.a 找不到

```bash
# 确保先构建 Go 静态库
./build.sh build
# 检查文件是否存在
ls -la tauri-app/src-tauri/lib/libblue.a
```

### 链接错误：undefined symbols

```bash
# 确保第三方库已构建
ls third_party/whisper.cpp/build/src/libwhisper.a
ls third_party/opus-src/build/libopus.a
ls third_party/espeak-ng/build/src/libespeak-ng/libespeak-ng.a

# 如果缺失，重新构建
./build.sh build
```

### 窗口不显示

- 点击系统托盘图标
- 右键托盘 → Show Window
