# 快速开始

本指南将帮助您快速启动和运行 ZimaOS Echo。

## 前置要求

- Linux、macOS 或 Windows
- 最低 512MB 内存（推荐 1GB）
- 100MB 磁盘空间

## 快速安装

### Linux / macOS

```bash
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash
```

### Windows

以管理员身份打开 PowerShell 并运行：

```powershell
irm https://echo.zimaos.com/install.ps1 | iex
```

## 验证安装

安装后，验证服务是否正在运行：

```bash
# Linux
systemctl status zimaos-echo

# Windows
Get-Service ZimaOS-Echo
```

访问仪表板：`http://localhost:8080`

## 健康检查

```bash
curl http://localhost:8080/health
```

预期响应：

```json
{
  "status": "ok",
  "version": "0.1.0",
  "uptime": "1h30m",
  "goroutines": 10,
  "mem_alloc_bytes": 5242880
}
```

## 下一步

- [安装选项](/zh/guide/installation) - 手动安装和 Docker
- [配置](/zh/guide/configuration) - 自定义您的设置
- [架构](/zh/guide/architecture) - 了解工作原理
