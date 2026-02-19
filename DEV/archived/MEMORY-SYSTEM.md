# ZimaOS-Blue 记忆系统

## 概述

ZimaOS-Blue 实现了一个双层记忆架构，灵感来自 clawdbot 的 MEMORY 系统，用于存储和检索对话中的重要信息。

## 架构

### 1. 双层记忆结构

#### 第一层：每日日志 (Daily Logs)
- **位置**: `~/.zimaos-blue/memory/daily/YYYY-MM-DD.md`
- **特点**:
  - 仅追加（append-only）
  - 自动记录每天的对话要点
  - 带时间戳的条目
  - 支持标签分类
- **保留期**: 默认 30 天（可配置）

#### 第二层：长期记忆 (Long-term Memory)
- **位置**: `~/.zimaos-blue/memory/MEMORY.md`
- **特点**:
  - 精选的持久知识
  - 手动或自动提升的重要信息
  - 按类别组织
  - 永久保存

### 2. 工作流程

```
用户对话
    ↓
自动提取重要信息
    ↓
保存到每日日志 (daily/YYYY-MM-DD.md)
    ↓
重要性评分 (ImportanceScorer)
    ↓
自动提升到长期记忆 (MEMORY.md)
    ↓
向量索引 + 关键词索引
    ↓
在后续对话中自动召回
```

## 核心功能

### 1. 自动记忆提取

在每次对话后，系统会：
1. 使用 LLM 分析对话内容
2. 提取重要事实和决策
3. 保存到当天的日志文件
4. 触发 `memory_saved` companion 事件

**触发条件**:
- IM 对话结束
- Web 聊天完成
- 用户明确要求记住某事

### 2. 记忆召回 (Recall)

在用户发送新消息时：
1. 使用混合搜索（向量 + BM25）查找相关记忆
2. 评分并排序结果
3. 将相关记忆作为 system message 注入对话
4. 阈值过滤（score < 0.3 的结果被忽略）

**示例**:
```
用户: "我上次说的那个项目进展如何？"
系统: [自动召回] "相关记忆：用户在 2024-01-15 提到正在开发 ZimaOS-Blue 项目..."
```

### 3. 记忆搜索

支持三种搜索模式：
- **向量搜索**: 语义相似度匹配
- **关键词搜索**: BM25 精确匹配
- **混合搜索**: 结合两者优势（默认）

## API 端点

### 每日日志

#### 列出所有日志
```http
GET /api/memory/daily
```

响应:
```json
{
  "dates": ["2024-01-15", "2024-01-16"],
  "count": 2
}
```

#### 获取特定日期的日志
```http
GET /api/memory/daily/:date
```

响应:
```json
{
  "date": "2024-01-15",
  "content": "# Daily Log - 2024-01-15\n\n## 10:30:00\n\n**Tags:** conversation\n\n用户讨论了..."
}
```

#### 添加到今日日志
```http
POST /api/memory/daily
Content-Type: application/json

{
  "content": "重要决策：采用 Go 语言开发后端",
  "tags": ["decision", "technical"]
}
```

#### 清理旧日志
```http
POST /api/memory/daily/prune
```

### 长期记忆

#### 获取长期记忆
```http
GET /api/memory/longterm
```

响应:
```json
{
  "content": "# Long-Term Memory\n\n## Technical\n\n*Added: 2024-01-15 10:30*\n\n..."
}
```

#### 提升到长期记忆
```http
POST /api/memory/longterm
Content-Type: application/json

{
  "content": "用户偏好使用 Vim 编辑器",
  "category": "Preferences"
}
```

### 搜索

#### 搜索记忆
```http
POST /api/memory/search
Content-Type: application/json

{
  "query": "项目决策",
  "limit": 10
}
```

响应:
```json
{
  "results": [
    {
      "content": "决定使用 Go 语言...",
      "score": 0.85,
      "created_at": "2024-01-15T10:30:00Z"
    }
  ],
  "count": 1
}
```

### 配置

#### 获取配置
```http
GET /api/memory/config
```

响应:
```json
{
  "base_dir": "/Users/user/.zimaos-blue/memory",
  "daily_retention_days": 30,
  "auto_promote_threshold": 0.8
}
```

## 用户数据导出/导入

### 导出

记忆数据现在包含在用户数据导出中：

```http
POST /api/userdata/export
Content-Type: application/json

{
  "password": "your-password",
  "format": "encrypted"
}
```

导出内容包括：
- ✅ 聊天历史
- ✅ 用户设置
- ✅ **每日日志**
- ✅ **长期记忆**

### 导入

```http
POST /api/userdata/import
Content-Type: application/json

{
  "password": "your-password",
  "data": "base64-encoded-data"
}
```

## 与 clawdbot 的对比

| 特性 | clawdbot | ZimaOS-Blue |
|------|----------|-------------|
| 存储方式 | Markdown 文件 | Markdown 文件 + SQLite |
| 记忆层级 | 2 层 (daily + MEMORY.md) | 2 层 (daily + MEMORY.md) |
| 搜索方式 | 向量 + BM25 混合 | 向量 + BM25 混合 |
| 自动提取 | ✅ (memory flush) | ✅ (extractMemory) |
| 自动召回 | ✅ (memory_search tool) | ✅ (recallMemories) |
| 向量索引 | sqlite-vec | 自定义实现 |
| API 访问 | ❌ | ✅ |
| 用户管理 | ❌ | ✅ |

## 配置

在 `config.yaml` 中配置记忆系统：

```yaml
memory:
  enabled: true
  base_dir: "~/.zimaos-blue/memory"
  daily_retention_days: 30
  auto_promote_threshold: 0.8

  # 搜索配置
  search:
    hybrid_enabled: true
    vector_weight: 0.7
    keyword_weight: 0.3
```

## 最佳实践

### 1. 用户明确要求记住

当用户说"记住这个"时：
```go
layeredMemory.AppendToDaily(ctx, content, []string{"user-request"})
```

### 2. 重要决策

自动标记为 "decision" 标签：
```go
layeredMemory.AppendToDaily(ctx, content, []string{"decision"})
```

### 3. 定期清理

设置定时任务清理旧日志：
```go
deleted, _ := layeredMemory.PruneDailyLogs(ctx)
```

### 4. 手动提升

对于特别重要的信息，手动提升到长期记忆：
```go
layeredMemory.PromoteToLongTerm(ctx, content, "Important")
```

## 隐私和安全

1. **本地存储**: 所有记忆数据存储在用户本地
2. **加密导出**: 支持密码加密的数据导出
3. **用户控制**: 用户可以查看、编辑、删除任何记忆
4. **自动清理**: 旧的每日日志会自动删除

## 故障排除

### 记忆未被召回

检查：
1. `layeredMemory` 是否已初始化
2. 搜索阈值是否过高（默认 0.3）
3. 向量索引是否正常工作

### 自动提取失败

检查：
1. LLM provider 是否可用
2. 对话内容是否足够长
3. 日志文件权限是否正确

### 搜索结果不准确

调整混合搜索权重：
```yaml
memory:
  search:
    vector_weight: 0.6  # 降低语义权重
    keyword_weight: 0.4  # 提高关键词权重
```

## 未来改进

- [ ] 支持图片记忆
- [ ] 记忆去重
- [ ] 记忆关联图谱
- [ ] 多用户记忆隔离
- [ ] 记忆分享功能
- [ ] 更智能的重要性评分
- [ ] 记忆统计和可视化

## 参考

- [clawdbot Memory Documentation](https://docs.openclaw.ai/concepts/memory)
- [ZimaOS-Blue Memory Implementation](../server/internal/memory/)
