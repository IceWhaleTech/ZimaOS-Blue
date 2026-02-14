# Memory System Enhancement Checklist

> 目标: 对齐并超越 Clawdbot 的记忆管理系统
> 创建日期: 2026-02-03
> 最后更新: 2026-02-03

## Phase 1: 基础对齐 (v0.10.18) ✅ 已完成

### 1.1 Memory Tools for AI Agent
- [x] 实现 `memory_search` 工具 - 语义搜索记忆
  - [x] 支持 query, maxResults, minScore 参数
  - [x] 返回 path, startLine, endLine, score, snippet
- [x] 实现 `memory_get` 工具 - 读取特定记忆内容
  - [x] 支持 path, from, lines 参数
- [x] 实现 `memory_stats` 工具 - 获取记忆统计
- [x] 注册到 tools registry (server/internal/tools/memory.go)

### 1.2 Markdown 导出/导入
- [x] `GET /api/v1/memory/export` - 导出为 Markdown
- [x] `POST /api/v1/memory/import` - 从 Markdown 导入
- [x] 前端 UI 支持 (UserDataExport.vue)
- [x] 支持按日期范围导出
- [x] 支持按标签过滤导出

### 1.3 搜索增强
- [x] 验证混合搜索权重配置 (70/30)
- [x] 添加搜索结果高亮
- [x] 支持日期范围过滤

---

## Phase 2: 双层记忆架构 (v0.11.0) ✅ 已完成

### 2.1 每日日志层 (Daily Logs)
- [x] 实现 Markdown 文件存储 (server/internal/memory/layered_memory.go)
- [x] 自动按日期分组存储 (daily/YYYY-MM-DD.md)
- [x] API: `GET /api/v1/memory/daily/:date`
- [x] API: `POST /api/v1/memory/daily` (AppendToDaily)
- [x] API: `GET /api/v1/memory/daily` (ListDailyLogs)
- [x] API: `POST /api/v1/memory/daily/prune` (PruneDailyLogs)

### 2.2 长期记忆层 (Long-term Memory)
- [x] 实现 MEMORY.md 文件存储
- [x] 支持分类 (category 参数)
- [x] API: `GET /api/v1/memory/longterm`
- [x] API: `POST /api/v1/memory/longterm` (PromoteToLongTerm)

### 2.3 自动分层
- [x] 会话结束时自动写入每日日志 (SessionMemoryHook)
- [x] 定期任务: 从日志提炼长期记忆 (MemoryExtractor)
- [x] 重要性评分算法 (ImportanceScorer)

---

## Phase 3: 压缩与刷新机制 (v0.11.1) ✅ 已完成

### 3.1 上下文压缩
- [x] 检测上下文接近限制 (75% 阈值)
- [x] 实现对话历史压缩 (SessionCompactor)
- [x] 保留最近 N 轮原始消息 (PreserveRecent)
- [x] 压缩摘要持久化

### 3.2 压缩前记忆刷新
- [x] 触发条件: 软阈值 (SoftThresholdRatio = 0.6)
- [x] CompactorMemoryIntegration 实现
- [x] 自动提取重要信息写入记忆 (LayeredMemoryRefresher)
- [x] 配置项: enabled, softThresholdRatio, systemPrompt, maxExtractTokens

### 3.3 会话钩子
- [x] 会话结束钩子 (SessionHook interface)
- [x] HookManager 管理多个钩子
- [x] 自动保存会话摘要到每日日志 (SessionMemoryHook)
- [x] 支持 Archive/Reset/Delete 触发
- [x] `/new` 命令触发保存 (EndReasonNew, SessionManager.NewSession)

---

## Phase 4: 多智能体支持 (v0.11.2) ✅ 已完成

### 4.1 记忆隔离
- [x] 按 agentId 隔离记忆存储 (MultiAgentMemoryManager)
- [x] 独立的向量索引 (每个 agent 独立 VectorStore)
- [x] 独立的 FTS 索引
- [x] 数据库路径: `memory/agents/<agentId>/memory.db`

### 4.2 智能体配置
- [x] 每个智能体独立的记忆配置 (AgentMemory)
- [x] 支持共享记忆池 (EnableSharedPool)
- [x] 跨智能体记忆搜索 (CrossAgentSearch)

---

## Phase 5: 透明存储模式 (v0.12.0) 🔄 部分完成

### 5.1 Markdown 文件存储
- [x] 可选的 Markdown 文件存储模式 (LayeredMemoryService)
- [x] 目录结构:
  ```
  ~/.zimaos-blue/memory/
  ├── MEMORY.md           # 长期记忆
  └── daily/
      ├── 2026-02-03.md   # 每日日志
      └── ...
  ```
- [ ] 文件监听器 (chokidar 风格)
- [ ] 自动重新索引

### 5.2 双向同步
- [ ] 文件修改 → 数据库更新
- [ ] 数据库修改 → 文件更新
- [ ] 冲突解决策略

---

## Phase 6: 超越 Clawdbot (v0.12.x)

### 6.1 智能记忆管理
- [ ] 记忆重要性自动评分
- [ ] 过期记忆自动归档
- [ ] 记忆关联图谱 (知识图谱)
- [ ] 记忆版本历史

### 6.2 高级搜索
- [ ] 时间感知搜索 ("上周讨论的...")
- [ ] 关系搜索 ("关于 Alice 的所有记忆")
- [ ] 多模态记忆 (图片、文件附件)

### 6.3 隐私与安全
- [ ] 端到端加密存储
- [ ] 敏感信息自动检测
- [ ] 记忆访问审计日志
- [ ] 选择性记忆删除 (GDPR 合规)

### 6.4 云端增强
- [ ] Supermemory 深度集成
- [ ] 跨设备记忆同步
- [ ] 离线优先 + 在线同步

---

## 验收标准

### 功能对齐
- [x] 混合搜索 (向量 + 关键词)
- [x] FTS5 全文搜索
- [x] 双层记忆架构
- [x] 压缩前记忆刷新
- [x] 会话结束钩子
- [x] 多智能体隔离
- [x] Markdown 透明存储 (部分)

### 超越 Clawdbot
- [x] 后端切换 (本地 ↔ 云端)
- [x] 自动清理策略
- [x] 记忆重要性评分 (ImportanceScorer)
- [ ] 知识图谱关联
- [ ] 端到端加密
- [ ] 跨设备同步

---

## 实现文件清单

### 新增文件
- `server/internal/tools/memory.go` - Memory tools for AI agent
- `server/internal/memory/tools_adapter.go` - Tools interface adapter
- `server/internal/memory/layered_memory.go` - Dual-layer memory service
- `server/internal/memory/layered_memory_test.go` - Tests
- `server/internal/memory/session_hook.go` - Session memory hook
- `server/internal/memory/session_hook_test.go` - Tests
- `server/internal/memory/memory_refresher.go` - Memory refresher
- `server/internal/memory/multi_agent.go` - Multi-agent memory manager
- `server/internal/memory/multi_agent_test.go` - Tests
- `server/internal/memory/importance.go` - Memory importance scoring
- `server/internal/memory/importance_test.go` - Tests
- `server/internal/memory/markdown_store.go` - Markdown file search
- `server/internal/memory/markdown_backend.go` - Pure Markdown backend
- `server/internal/session/hooks.go` - Session hook interface
- `server/internal/session/compactor_memory.go` - Compactor with memory integration
- `server/internal/session/compactor_memory_test.go` - Tests

### 修改文件
- `server/internal/providerpool/builtin.go` - RegisterMemoryTools
- `server/internal/server/memory_handler.go` - Layered memory API endpoints, date range export, highlighting
- `server/internal/session/manager.go` - Hook support
- `server/internal/memory/vector_store.go` - sqlite-vec support for fast KNN search
- `server/internal/memory/hybrid_search.go` - Search highlighting and date range filter
- `web/src/api/memory.ts` - Export/import API
- `web/src/components/UserDataExport.vue` - Memory UI

---

## 参考资料

- [Clawdbot Memory System Analysis](./memory-system-comparison-clawdbot.md)
- [原文](https://manthanguptaa.in/posts/clawdbot_memory/)
