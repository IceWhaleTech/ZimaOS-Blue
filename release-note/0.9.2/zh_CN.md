## [0.9.2]

### 新增
- 新增 OTA 自动更新系统
- 新增设置页面更新提示小红点
- 新增跨平台热更新支持（Linux/macOS/Windows），uptime 不中断
- 新增更新失败自动回滚机制

### 功能
- 支持手动检查更新和自动检查更新
- 后台下载，无活跃会话时自动更新
- 会话感知：等待活跃对话完成后再更新
- 热更新时保持 uptime 计数器连续

### 配置项
- `update.enabled`: 启用/禁用 OTA 更新
- `update.check_interval`: 自动检查频率（默认: 24h）
- `update.auto_download`: 有更新时自动下载
- `update.release_channel`: stable / beta / alpha

### API
- `GET /api/v1/system/update/check` - 检查更新
- `POST /api/v1/system/update/download` - 下载更新
- `POST /api/v1/system/update/apply` - 应用更新
- `POST /api/v1/system/update/rollback` - 回滚版本

### 提示
- 仅在无活跃聊天会话时应用更新
- Linux/macOS: 通过 syscall.Exec 实现零停机热更新
- Windows: 通过 exe 重命名实现更新，短暂重连 (~1s)
- 如遇问题，欢迎加入社区反馈！
- <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/issues" target="_blank" style="color:blue">https://github.com/IceWhaleTech/ZimaOS-Echo/issues</a>
