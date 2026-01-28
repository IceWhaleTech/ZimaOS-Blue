## [0.9.2]

### New
- Added OTA update system with automatic version checking
- Added update notification badge (red dot) in settings
- Added cross-platform hot update (Linux/macOS/Windows) with uptime continuity
- Added automatic rollback on update failure

### Features
- Manual and automatic update check options
- Download in background, apply when no active sessions
- Session-aware update: waits for active conversations to complete
- Preserves uptime counter across hot updates

### Configuration
- `update.enabled`: Enable/disable OTA updates
- `update.check_interval`: Auto-check frequency (default: 24h)
- `update.auto_download`: Auto-download when update available
- `update.release_channel`: stable / beta / alpha

### API
- `GET /api/v1/system/update/check` - Check for updates
- `POST /api/v1/system/update/download` - Download update
- `POST /api/v1/system/update/apply` - Apply update
- `POST /api/v1/system/update/rollback` - Rollback to previous

### Tips
- Updates are applied only when no active chat sessions
- Linux/macOS: zero-downtime hot update via syscall.Exec
- Windows: brief reconnection (~1s) during update via exe rename
- If you encounter any issues, feel free to join our community!
- <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/issues" target="_blank" style="color:blue">https://github.com/IceWhaleTech/ZimaOS-Echo/issues</a>
