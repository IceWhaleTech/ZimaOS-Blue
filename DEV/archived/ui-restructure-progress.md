# UI Restructure - Progress Log

## Session: 2026-01-30

### Phase 1: HomeView Enhancement ✅ COMPLETED

**Changes Made:**
- 移除了 4 个快速统计卡片 (Status, Version, Uptime, Memory)
- 保留 Hero Section (欢迎信息)
- 保留 5秒自动刷新功能
- 添加 ConfigurableDashboard 组件
- 添加详细系统信息展示 (长条展开式)
- 移除 Metrics 部分 (根据用户反馈)

### Phase 2: Settings Page Restructure ✅ COMPLETED

**Changes Made:**
- 重构为扁平化 Tab 结构 (8 个顶级 Tab):
  - General: 语言、时区、主题
  - LLM: Provider Pool、Claude Code Settings、Model Parameters
  - Metrics: 用量统计、图表、模型统计
  - Config: 系统配置 JSON 编辑
  - Backup: 备份管理
  - Logs: 日志查看
  - Service: 服务管理
  - Retention: 数据保留设置 (从 Security 页面迁移)
- 支持 URL 参数 (?tab=llm)
- 从 SystemView 迁移了所有系统管理功能
- 从 SecurityView 迁移了 Data Retention Settings

### Phase 3: SystemView Cleanup ✅ COMPLETED

**Changes Made:**
- 路由 /system 重定向到 /
- 导航菜单移除 System 入口
- SystemView.vue 保留但不再使用 (可后续删除)

### Phase 4: i18n Cleanup ✅ COMPLETED

**Changes Made:**
- 添加 settings.tab.* keys (general, llm, metrics, config, backup, logs, service, retention)
- 添加 security.settings.description, security.settings.cleanupSuccess keys
- 添加 common.copy, common.copied keys
- 添加 system.backupCreated, system.backupDeleted keys

### Phase 5: Testing & Verification ⏳ PENDING

**To Verify:**
- [ ] HomeView 所有功能正常
- [ ] Settings 所有 Tab 功能正常
- [ ] 路由跳转正确
- [ ] i18n 无缺失 key
- [ ] 响应式布局测试

---

## Files Modified

### Views
- `web/src/views/HomeView.vue` - 重构
- `web/src/views/SettingsView.vue` - 重构 (扁平化 Tab + Data Retention)

### Router
- `web/src/router/index.ts` - /system 重定向

### Navigation
- `web/src/components/AppSidebar.vue` - 移除 System 入口

### i18n
- `web/src/i18n/locales/en-US.ts` - 添加/清理 keys

---

## Summary

界面调整已完成:
1. HomeView: 欢迎信息 + Dashboard + 详细系统信息 (展开式)
2. SettingsView: 8 个扁平化 Tab (General | LLM | Metrics | Config | Backup | Logs | Service | Retention)
3. /system 路由重定向到首页
4. 导航菜单已更新
5. Data Retention Settings 从 Security 页面迁移到 Settings 页面
