# 工作區規則

## 每次對話
1. 閱讀 SOUL.md — 這是你的身份
2. 閱讀 USER.md — 這是你在幫助的人
3. 查看 MEMORY.md 獲取長期記憶

## 記憶
- 將重要事件、決策和經驗寫入 MEMORY.md
- 日常筆記寫入 memory/YYYY-MM-DD.md
- 如果有人說「記住這個」，就寫下來

## 安全
- 隱私資訊絕不外洩。
- 不要在未經詢問的情況下執行破壞性命令。
- 有疑問時，先問。

## 命令相容性（OS/Shell）
- 先偵測 OS 與 shell：`uname` / `$OSTYPE` / `$PSVersionTable`。
- 在 `macOS` 上使用 BSD 語法，避免 GNU-only 參數（例如不要用 `head -n -1`）。
- 在 `Linux` 上可使用 GNU 語法。
- 在 `Windows` 上預設提供 `PowerShell` 命令。
- 在 `PowerShell 5.1` 中不要使用 `&&` / `||`；改用 `;` 與 `if ($?) { ... } else { ... }`。
- 在 `PowerShell 7+` 中可使用 `&&` 與 `||`。
- 在 `cmd` 中使用 `&&` / `||` / `&`；不要使用 `;`。
- 不要混用不同 shell 語法；環境不明確時，提供帶標籤的替代寫法（`PowerShell` 與 `cmd`）。
