# UI Restructure Plan - Dashboard & Settings Consolidation

## Overview

将系统页面的 Dashboard 合并到 HomeView，系统配置功能合并到设置页面，LLM 设置独立为单独 Tab，清理无效 i18n 资源。

## Current State Analysis

### HomeView (`web/src/views/HomeView.vue`)
- 欢迎信息 Hero section
- 5秒自动刷新功能
- 6个统计卡片 (Status, Version, Uptime, Memory, Goroutines, CPU)

### SystemView (`web/src/views/SystemView.vue`)
- **Overview Tab**: ConfigurableDashboard, 详细系统信息 (OS, CPU, Memory, Disk, GPU, Network, Runtime)
- **Metrics Tab**: MetricsOverview, TokenUsageChart, LatencyChart
- **Logs Tab**: 日志搜索、过滤、导出
- **Config Tab**: JSON 配置编辑器
- **Backup Tab**: 备份管理
- **Service Tab**: 服务管理

### SettingsView (`web/src/views/SettingsView.vue`)
- General Settings (Language, Timezone, Theme)
- Provider Pool Section (LLM 设置)
- Claude Code CLI Settings
- Model Parameters

---

## Task Plan

### Phase 1: HomeView Enhancement (Dashboard Merge)
**Priority: HIGH**

| Task | Description | Status |
|------|-------------|--------|
| 1.1 | 保留 HomeView 欢迎信息和 5秒刷新功能 | ⬜ Pending |
| 1.2 | 从 SystemView 移植 ConfigurableDashboard 组件到 HomeView | ⬜ Pending |
| 1.3 | 移植详细系统信息展示 (OS, CPU, Memory, Disk, GPU, Network) | ⬜ Pending |
| 1.4 | 移植 Metrics 相关组件 (MetricsOverview, Charts) | ⬜ Pending |
| 1.5 | 调整 HomeView 布局，整合所有 Dashboard 内容 | ⬜ Pending |
| 1.6 | 更新 HomeView 的 store 依赖 | ⬜ Pending |

### Phase 2: Settings Page Restructure
**Priority: HIGH**

| Task | Description | Status |
|------|-------------|--------|
| 2.1 | 创建 Settings 页面 Tab 结构 | ⬜ Pending |
| 2.2 | Tab 1: General - 保留现有通用设置 | ⬜ Pending |
| 2.3 | Tab 2: LLM - 独立 ProviderPoolSection 和 Model Parameters | ⬜ Pending |
| 2.4 | Tab 3: System - 从 SystemView 移植 Config, Backup, Service, Logs | ⬜ Pending |
| 2.5 | 更新 Settings 页面导航和路由 | ⬜ Pending |

### Phase 3: SystemView Cleanup
**Priority: MEDIUM**

| Task | Description | Status |
|------|-------------|--------|
| 3.1 | 移除或重定向 /system 路由 | ⬜ Pending |
| 3.2 | 删除 SystemView.vue 文件 | ⬜ Pending |
| 3.3 | 更新导航菜单，移除 System 入口 | ⬜ Pending |
| 3.4 | 清理相关的 store 和 API 引用 | ⬜ Pending |

### Phase 4: i18n Cleanup
**Priority: LOW**

| Task | Description | Status |
|------|-------------|--------|
| 4.1 | 分析所有 i18n key 的使用情况 | ⬜ Pending |
| 4.2 | 识别未使用的 i18n 资源 | ⬜ Pending |
| 4.3 | 清理 en-US.ts 中的无效 key | ⬜ Pending |
| 4.4 | 同步清理其他语言文件 | ⬜ Pending |
| 4.5 | 添加新的 i18n key (如需要) | ⬜ Pending |

### Phase 5: Testing & Verification
**Priority: HIGH**

| Task | Description | Status |
|------|-------------|--------|
| 5.1 | 验证 HomeView 所有功能正常 | ⬜ Pending |
| 5.2 | 验证 Settings 所有 Tab 功能正常 | ⬜ Pending |
| 5.3 | 验证路由跳转正确 | ⬜ Pending |
| 5.4 | 验证 i18n 无缺失 key | ⬜ Pending |
| 5.5 | 响应式布局测试 | ⬜ Pending |

---

## Detailed Implementation

### New HomeView Structure

```
HomeView.vue
├── Hero Section (保留)
│   ├── Welcome message
│   └── Auto-refresh toggle (5s)
├── Dashboard Section (新增)
│   ├── ConfigurableDashboard
│   └── Dashboard Customizer
├── System Info Section (从 SystemView 移植)
│   ├── OS Info
│   ├── CPU Info (with chart)
│   ├── Memory Info (RAM + Swap)
│   ├── Disk Info
│   ├── GPU Info
│   ├── Network Info
│   └── Runtime Info
└── Metrics Section (从 SystemView 移植)
    ├── MetricsOverview
    ├── TokenUsageChart
    └── LatencyChart
```

### New SettingsView Structure

```
SettingsView.vue
├── Tab: General (通用设置)
│   ├── Language Selector
│   ├── Timezone Selector
│   └── Theme Selector
├── Tab: LLM (AI 模型设置) [NEW TAB]
│   ├── ProviderPoolSection
│   ├── Model Parameters
│   └── Claude Code Settings
└── Tab: System (系统管理) [从 SystemView 移植]
    ├── Config Editor
    ├── Backup Management
    ├── Service Management
    └── Logs Viewer
```

---

## Files to Modify

### Create/Modify
- `web/src/views/HomeView.vue` - 增强，添加 Dashboard 内容
- `web/src/views/SettingsView.vue` - 重构为 Tab 结构

### Delete
- `web/src/views/SystemView.vue` - 功能已合并

### Update
- `web/src/router/index.ts` - 移除 /system 路由
- `web/src/components/layout/Sidebar.vue` 或导航组件 - 更新菜单
- `web/src/i18n/locales/en-US.ts` - 清理无效 key

---

## Dependencies

### Components to Reuse
- `ConfigurableDashboard.vue`
- `DashboardCustomizer.vue`
- `DashboardCard.vue`
- `MetricsOverview.vue`
- `TokenUsageChart.vue`
- `LatencyChart.vue`
- `ProviderPoolSection.vue`
- `ClaudeCodeSettings.vue`

### Stores
- `useSystemStore()`
- `useMetricsStore()`
- `useSettingsStore()`
- `useProviderPoolStore()`

---

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| 组件依赖丢失 | HIGH | 仔细检查所有 import |
| i18n key 缺失 | MEDIUM | 运行时检查，添加 fallback |
| 路由重定向问题 | MEDIUM | 添加 redirect 规则 |
| 响应式布局问题 | LOW | 充分测试各屏幕尺寸 |

---

## Estimated Scope

- **Files to modify**: ~8-10 files
- **New components**: 0 (复用现有)
- **Deleted files**: 1 (SystemView.vue)
- **i18n changes**: TBD after analysis

---

## Notes

1. 保持 5秒刷新功能在 HomeView 中
2. LLM 设置作为独立 Tab 提高可访问性
3. 系统管理功能集中到 Settings 便于统一管理
4. i18n 清理需要全面扫描，避免遗漏
