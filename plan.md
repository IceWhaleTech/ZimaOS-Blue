# Fix: Analyze Tool Data Truncation

## Problem

Analyze tool 采集的页面数据严重不足，有 4 层截断：

1. **axTreeBuilder.build()** — 8192 字符硬截断 (service.go:1135)
2. **scrapeURL** — 15000 字符截断 (analyze.go:296) — 但因为第1层已经截到8K，这层形同虚设
3. **llmExtractAndAnalyze** — 30000 字符截断 (analyze.go:336)
4. **AccessibilityTree depth=8** vs browser tool 的 depth=10

核心问题：CDP `AccessibilityGetFullAXTree` 返回的是完整 DOM 树（不受滚动位置影响），但 8K 的 DSL 截断导致大量内容丢失。对于论坛/Reddit 等内容密集页面，8K 只能覆盖页面顶部一小部分。

## Solution

### Approach: 为 analyze 提供专用的大容量文本提取

不修改 `axTreeBuilder` 的 8K 上限（那是给 LLM 对话用的，需要省 token），而是：

1. **扩展 `BrowserBackend` 接口**，添加 `ExtractText` 方法 — 通过 JS `document.body.innerText` 提取纯文本内容，不受 a11y tree DSL 格式和 8K 限制
2. **在 `RodService` 实现 `ExtractText`** — 用 JS eval 提取 `document.body.innerText`，上限 50K 字符
3. **在 `RodBrowserBackend` adapter 暴露** `ExtractText`
4. **修改 `scrapeURL`** — 同时获取 a11y tree（结构信息）+ `ExtractText`（完整文本），合并后提供给 LLM
5. **提高 `llmExtractAndAnalyze` 的截断上限** — 从 30K → 50K

### Changes

#### 1. `server/internal/browser/service.go` — 新增 `ExtractText`
```go
func (s *RodService) ExtractText(ctx context.Context, targetID string, maxLen int) (string, error)
```
- 用 `document.body.innerText` 提取纯文本
- 默认 maxLen=50000

#### 2. `server/internal/tools/browser.go` — 扩展 `BrowserBackend` 接口
```go
ExtractText(ctx context.Context, targetID string, maxLen int) (string, error)
```

#### 3. `server/internal/tools/browser_adapter.go` — 添加 adapter 方法

#### 4. `server/internal/tools/analyze.go` — 改进 `scrapeURL`
- 调用 `ExtractText` 获取完整文本（最多 50K）
- a11y tree 作为补充结构信息（保持 depth=8，但不再是主要数据源）
- 提高 `llmExtractAndAnalyze` 截断上限到 50K
- 如果 `ExtractText` 失败，fallback 到现有的 a11y tree 方式
