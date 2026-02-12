# ZimaOS Blue - Tauri Desktop App

Tauri v2 桌面应用，将 ZimaOS Blue 打包为原生桌面应用。

## 架构

```
┌─────────────────────────────────────────┐
│           ZimaOS Blue.app               │
│  ┌───────────────────────────────────┐  │
│  │     zimaos-blue (Tauri/Rust)      │  │
│  │  - 窗口管理 (WebView)              │  │
│  │  - 系统托盘                        │  │
│  │  - 进程生命周期管理                 │  │
│  └───────────────┬───────────────────┘  │
│                  │ 启动/管理            │
│  ┌───────────────▼───────────────────┐  │
│  │     echo-server (Go sidecar)      │  │
│  │  - HTTP API 服务 (:23456)           │  │
│  │  - LLM 代理                        │  │
│  │  - 嵌入式前端                      │  │
│  │  - 所有业务逻辑                    │  │
│  └───────────────────────────────────┘  │
└─────────────────────────────────────────┘
```

### 组件说明

| 组件 | 技术栈 | 作用 |
|------|--------|------|
| **zimaos-blue** | Rust/Tauri | 桌面壳，负责窗口、托盘、sidecar 管理 |
| **echo-server** | Go | 后端服务，处理所有 API 请求和业务逻辑 |

## 开发

### 前置要求

- Node.js 20+
- Rust (stable)
- Go 1.23+

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
│   │   ├── lib.rs      # 主入口，窗口和托盘设置
│   │   ├── server.rs   # Sidecar 管理（启动/停止/状态）
│   │   └── tray.rs     # 托盘相关
│   ├── bin/            # Go sidecar 二进制文件
│   │   └── echo-server-{target}
│   ├── icons/          # 应用图标
│   ├── Cargo.toml      # Rust 依赖
│   └── tauri.conf.json # Tauri 配置
├── build.sh            # 完整构建脚本
└── package.json        # Node.js 依赖
```

## Sidecar 启动逻辑

应用启动时会执行以下逻辑（见 `src-tauri/src/server.rs`）：

1. **检查现有服务** - 如果 23456 端口已有健康的 echo-server，直接复用
2. **清理僵尸进程** - 如果端口被占用但服务不健康，杀掉旧进程
3. **启动新服务** - 找到可用端口，启动 sidecar
4. **等待就绪** - 轮询健康检查接口直到服务就绪

## 跨平台构建

本地只能构建当前平台的包。跨平台构建请使用 GitHub Actions：

```bash
# 推送代码后，在 GitHub Actions 手动触发
# Actions → Build Tauri App → Run workflow → 选择平台
```

支持的平台：
- **macOS**: arm64 (Apple Silicon) + x64 (Intel)
- **Windows**: x64
- **Linux**: x64 (AppImage, deb)

## 配置

主要配置文件：`src-tauri/tauri.conf.json`

- `bundle.externalBin`: sidecar 二进制文件路径
- `app.windows`: 窗口配置（大小、标题等）
- `app.trayIcon`: 托盘图标配置

## 故障排除

### 服务未启动

1. 检查日志：应用启动时会输出到 stderr
2. 检查端口：`lsof -i :23456`
3. 手动测试：`curl http://localhost:23456/api/v1/health`

### 窗口不显示

- 点击系统托盘图标
- 右键托盘 → Show Window

### 构建失败

```bash
# 清理后重新构建
make tauri-clean
make tauri-package
```
