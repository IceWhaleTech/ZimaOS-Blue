---
layout: home

hero:
  name: ZimaOS Echo
  text: NAS 原生 Agent 运行时
  tagline: 轻量、高性能的 AI Agent 运行时，专为低功耗 NAS 设备优化
  actions:
    - theme: brand
      text: 快速开始
      link: /zh/guide/getting-started
    - theme: alt
      text: GitHub
      link: https://github.com/IceWhaleTech/ZimaOS-Echo/server

features:
  - icon: 🚀
    title: 轻量级
    details: 单一二进制文件 < 15MB，内存占用 < 80MB，专为 NAS 设备优化
  - icon: ⚡
    title: 高性能
    details: 使用 Go 构建，利用 goroutine 实现高吞吐低延迟
  - icon: 🔌
    title: 可扩展
    details: 插件化架构，支持 Go 模块或 WASM 插件
  - icon: 🛡️
    title: 稳定可靠
    details: 专为 24/7 运行设计，支持优雅关闭和自动恢复
  - icon: 🎯
    title: 一键部署
    details: 简单的安装脚本，支持 Linux、macOS 和 Windows
  - icon: 🌐
    title: 多通道
    details: 支持 Web、Telegram、Discord、Slack 等（即将推出）
---

## 从源码安装

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo server
```

更多方式见 [安装指南](guide/installation.md)。

## 什么是 ZimaOS Echo？

ZimaOS Echo 是一个使用 Go 构建的 **NAS 原生 Agent 运行时**，灵感来源于 [clawdbot](https://github.com/clawdbot/clawdbot)。它专为低功耗 NAS 和边缘设备设计，提供：

- **极低资源占用**：在 CPU 和内存有限的设备上高效运行
- **长期稳定性**：专为 24/7 不间断运行设计
- **简单部署**：一键安装，集成 systemd
- **现代前端**：Vue 3 仪表板用于监控和管理
