# 快速开始

本指南将帮助您快速启动和运行 ZimaOS Echo。

## 前置要求

- Linux、macOS 或 Windows
- 最低 512MB 内存（推荐 1GB）
- 100MB 磁盘空间

## 快速安装

从源码构建并运行：

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo server
```

更多方式见 [安装](installation.md)。

## 验证安装

安装后，验证服务是否正在运行：

```bash
# Linux
systemctl status zimaos-echo

# Windows
Get-Service ZimaOS-Echo
```

访问仪表板：`http://localhost:23456`

## 健康检查

```bash
curl http://localhost:23456/health
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
