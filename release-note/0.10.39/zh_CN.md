# ZimaOS Blue v0.10.39

发布日期：2026-04-09

本次更新统一了 Research 工作流，新增 Knowledge 和 Evolution 工作区，并进一步增强了会话恢复能力以及文档提取和执行可靠性。

## 亮点

- 为 research、knowledge、evolution 任务带来更统一的工作流
- 返回进行中或等待中的会话时，恢复体验更好
- 文档提取和受控执行的可靠性更强

## 新增

- 新增统一的 `Research` 入口，同时保留不同模式下的专属输出
- 新增 `Knowledge` 工作区，用于管理编译页面、lint 状态和 ask-and-archive 任务
- 新增 `Evolution` 控制台和本地 `blue audit` 工具，用于审查和诊断

## 改进

- 改进会话 bootstrap，使重新打开的聊天可以恢复更多活动状态和待处理审批
- 改进 Harness 数据集工作流，使评估运行更具可重复性
- 改进上下文压缩、failover 处理和本地化一致性

## 修复

- 修复提供商目录校验中的刷新循环问题
- 通过 artifact recovery 处理修复重复读取循环场景
- 修复 PDF 文本提取过程中异常字符修复的问题

## 安全

- 通过 `blue exec` 和分层 sandbox 路由强化命令执行安全性

## 说明

如果你遇到任何问题，欢迎加入 Zima Discord 社区，获得超过 43,000 名成员的支持：

https://zimaboard.com/discord
