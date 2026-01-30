# UI Restructure - Findings & Research

## Session: 2026-01-30

---

## Component Analysis

### HomeView Components
| Component | Location | Keep/Modify |
|-----------|----------|-------------|
| Hero Section | inline | Keep |
| Auto-refresh | useIntervalFn | Keep |
| Stats Grid | inline | Replace with Dashboard |

### SystemView Components to Migrate

#### To HomeView (Dashboard)
| Component | Source | Purpose |
|-----------|--------|---------|
| ConfigurableDashboard | `components/dashboard/` | 可配置仪表板 |
| DashboardCustomizer | `components/dashboard/` | 仪表板自定义 |
| System Info Display | inline in SystemView | OS/CPU/Memory/Disk/GPU/Network |
| MetricsOverview | `components/` | 指标概览 |
| TokenUsageChart | `components/` | Token 使用图表 |
| LatencyChart | `components/` | 延迟图表 |

#### To SettingsView (System Tab)
| Component | Source | Purpose |
|-----------|--------|---------|
| Config Editor | inline in SystemView | JSON 配置编辑 |
| Backup Management | inline in SystemView | 备份管理 |
| ServiceManagement | `components/` | 服务管理 |
| Logs Viewer | inline in SystemView | 日志查看 |

---

## Store Dependencies

### HomeView (After Merge)
```typescript
import { useSystemStore } from '@/stores/system'
import { useMetricsStore } from '@/stores/metrics'
import { useDashboardStore } from '@/stores/dashboard' // if exists
```

### SettingsView (After Restructure)
```typescript
import { useSettingsStore } from '@/stores/settings'
import { useLocaleStore } from '@/stores/locale'
import { useThemeStore } from '@/stores/theme'
import { useProviderPoolStore } from '@/stores/providerPool'
import { useSystemStore } from '@/stores/system' // for config/backup
```

---

## API Dependencies

### SystemView APIs to Redistribute
```typescript
// To HomeView
systemApi.getHealth()
systemApi.getSystemInfo()
metricsApi.getMetrics()

// To SettingsView
systemApi.getConfig()
systemApi.updateConfig()
backupApi.listBackups()
backupApi.createBackup()
backupApi.restoreBackup()
backupApi.deleteBackup()
systemApi.getLogs()
```

---

## i18n Key Categories

### Keys to Keep
- `home.*` - HomeView 相关
- `settings.*` - Settings 相关
- `providerPool.*` - LLM Provider 相关
- `common.*` - 通用词汇

### Keys to Review
- `system.*` - 部分迁移到 settings
- `metrics.*` - 迁移到 home
- `dashboard.*` - 迁移到 home

### Potentially Unused Keys
需要通过代码扫描确认:
- 旧版功能的 key
- 已删除组件的 key

---

## Router Changes

### Current Routes
```typescript
{ path: '/', component: HomeView }
{ path: '/settings', component: SettingsView }
{ path: '/system', component: SystemView }
```

### Target Routes
```typescript
{ path: '/', component: HomeView }
{ path: '/settings', component: SettingsView }
{ path: '/settings/:tab?', component: SettingsView } // optional tab param
{ path: '/system', redirect: '/' } // redirect old links
```

---

## UI/UX Considerations

### HomeView Layout
1. **Hero** - 顶部欢迎区域，简洁
2. **Quick Stats** - 关键指标快速查看
3. **Dashboard** - 可配置的详细仪表板
4. **System Details** - 可折叠的详细信息

### SettingsView Tabs
1. **General** - 最常用设置
2. **LLM** - AI 模型配置 (独立 Tab 提高可见性)
3. **System** - 系统管理 (高级功能)

---

## Migration Checklist

### Phase 1: HomeView
- [ ] 导入 ConfigurableDashboard
- [ ] 导入 MetricsOverview, Charts
- [ ] 添加系统详细信息展示
- [ ] 保持 5秒刷新逻辑
- [ ] 调整布局和样式

### Phase 2: SettingsView
- [ ] 添加 Tab 组件
- [ ] 创建 General Tab
- [ ] 创建 LLM Tab
- [ ] 创建 System Tab
- [ ] 迁移 Config Editor
- [ ] 迁移 Backup Management
- [ ] 迁移 Service Management
- [ ] 迁移 Logs Viewer

### Phase 3: Cleanup
- [ ] 删除 SystemView.vue
- [ ] 更新路由
- [ ] 更新导航菜单
- [ ] 清理 i18n

---

## Notes

- ConfigurableDashboard 已经是独立组件，迁移应该比较简单
- Logs Viewer 功能较复杂，需要完整迁移搜索/过滤/导出功能
- 考虑添加 Settings Tab 的 URL 参数支持 (如 /settings/llm)
