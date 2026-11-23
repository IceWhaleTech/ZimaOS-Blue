# ZimaOS Blue v0.11.0

改进技能发现、文档处理和任务完成判断。

## 下载

| 平台 | 安装包 | 下载大小 |
| --- | --- | ---: |
| macOS 桌面版 · Apple Silicon | [ZimaOS-Blue-0.11.0-macos-arm64.dmg](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/download/0.11.0/ZimaOS-Blue-0.11.0-macos-arm64.dmg) | 31.94 MiB |
| macOS 桌面版 · Intel | [ZimaOS-Blue-0.11.0-macos-amd64.dmg](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/download/0.11.0/ZimaOS-Blue-0.11.0-macos-amd64.dmg) | 35.01 MiB |
| macOS CLI · Apple Silicon | [blue-0.11.0-darwin-arm64.tar.gz](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/download/0.11.0/blue-0.11.0-darwin-arm64.tar.gz) | 31.22 MiB |
| macOS CLI · Intel | [blue-0.11.0-darwin-amd64.tar.gz](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/download/0.11.0/blue-0.11.0-darwin-amd64.tar.gz) | 33.56 MiB |
| Linux · x86_64 | [blue-0.11.0-linux-amd64.tar.gz](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/download/0.11.0/blue-0.11.0-linux-amd64.tar.gz) | 46.52 MiB |
| Linux · arm64 | [blue-0.11.0-linux-arm64.tar.gz](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/download/0.11.0/blue-0.11.0-linux-arm64.tar.gz) | 44.28 MiB |
| ZimaOS 模块 | [zimaos-blue.raw](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/download/0.11.0/zimaos-blue.raw) | 46.66 MiB |

## 改进

- 根据任务自动选择已安装的 FiLens 等文档技能，覆盖表格与跨文件计算场景。
- 保留后续推理需要的 shell 执行证据，并在技能搜索和推荐接口中返回请求 token 用量。
- 结束前检查用户要求的结果是否已交付，交付完成后及时结束。
- 将口语化的技能查找请求改写成有效搜索词，增加多语言查询规划、缓存与失败回退。
- 复用 Photos 的本地语言识别，模型大小为 938,013 字节，无需附带翻译运行时或语言包。

## 修复

- 使用 PDFKit 实际的注释键和字段类型，修复 macOS PDF 表单识别与填写。
- 更新前端依赖，处理本次发布依赖审计发现的漏洞。

## 打包

- 将 CLI、网页、Rust 和桌面端版本统一为 0.11.0。
- 为 Apple Silicon 和 Intel 构建 macOS CLI 与桌面安装包，检查架构和签名，要求 Apple 公证通过，并验证 DMG 装订的公证票据。
- 签名凭据不可用时提前明确失败。手动验证构建不会上传公开发行附件。

[发布验证构建](https://github.com/IceWhaleTech/ZimaOS-Blue/actions/runs/45145923323)已通过：macOS 两种架构的 Developer ID 签名、Apple 公证、DMG 票据装订与 Gatekeeper 检查均成功。

## 在 ZimaOS 上运行 Blue

<p align="center"><a href="https://www.zimaspace.com/cn/zimaos?utm_source=github&amp;utm_medium=referral&amp;utm_campaign=blue_zimaos&amp;utm_content=release_banner_zh"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS-Blue/main/docs/assets/release/zimaos-banner.zh-CN.png" alt="ZimaOS：个人云，更简单，支持 Blue 本地 AI 智能体" width="640" /></a></p>

用 ZimaOS 构建自己的个人云：统一管理文件、备份数据，并使用 Blue 运行本地 AI 智能体。
