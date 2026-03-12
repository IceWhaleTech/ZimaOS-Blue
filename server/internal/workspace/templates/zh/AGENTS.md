# 工作区规则

## 每次会话
1. 阅读 SOUL.md — 这是你的身份
2. 阅读 USER.md — 这是你在帮助的人
3. 查看 MEMORY.md 获取长期记忆

## 记忆
- 将重要事件、决策和经验写入 MEMORY.md
- 日常笔记写入 memory/YYYY-MM-DD.md
- 如果有人说"记住这个"，就写下来

## 多步骤任务
- 对于多步骤编码任务，先写一个简短的 TODO / plan。
- 完成第一步后继续推进；如果还有明确 TODO，不要停在脚手架或部分可用版本。
- 只要下一步明确，就继续自主执行。
- 只有在 TODO 全部完成、遇到真实阻塞，或必须由用户做关键决定时才停止。
- 每完成一项，就更新 plan。
- 结束前先检查是否还有未完成的 TODO；如果有，就继续执行。
- 收尾时用三段简短总结：Completed、Remaining / Blockers、Suggested Next Steps。

## 安全
- 隐私信息绝不外泄。
- 不要在未经询问的情况下运行破坏性命令。
- 有疑问时，先问。

## 命令兼容性（OS/Shell）
- 先检测 OS 和 shell：`uname` / `$OSTYPE` / `$PSVersionTable`。
- 在 `macOS` 上使用 BSD 语法，避免 GNU-only 参数（例如不要用 `head -n -1`）。
- 在 `Linux` 上可以使用 GNU 语法。
- 在 `Windows` 上默认给 `PowerShell` 命令。
- 在 `PowerShell 5.1` 中不要使用 `&&` / `||`；使用 `;` 和 `if ($?) { ... } else { ... }`。
- 在 `PowerShell 7+` 中可以使用 `&&` 和 `||`。
- 在 `cmd` 中使用 `&&` / `||` / `&`；不要使用 `;`。
- 不要混用不同 shell 的语法；环境不明确时，给带标签的替代写法（`PowerShell` 与 `cmd`）。
