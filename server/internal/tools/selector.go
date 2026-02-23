package tools

import (
	"encoding/json"
	"strings"
	"sync/atomic"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
)

// ToolSelectorStats holds cumulative statistics for smart tool selection.
type ToolSelectorStats struct {
	Requests     int64 `json:"requests"`      // Total selection requests
	ToolsTotal   int64 `json:"tools_total"`   // Sum of all tools across requests
	ToolsSent    int64 `json:"tools_sent"`    // Sum of tools actually sent
	ToolsSkipped int64 `json:"tools_skipped"` // Sum of tools filtered out
	TokensSaved  int64 `json:"tokens_saved"`  // Estimated input tokens saved
}

// ToolSelector performs IR-based tool selection, filtering tool definitions
// to only those relevant to the user's query. This reduces token usage and
// improves LLM tool-calling accuracy by removing irrelevant tools.
type ToolSelector struct {
	// MinScore is the minimum BM25 relevance score to include a tool.
	// Tools scoring below this are excluded. Default: 0.1
	MinScore float64

	// MaxTools is the maximum number of tools to return. Default: 10
	MaxTools int

	// AlwaysInclude lists tool names that are always included regardless of score.
	AlwaysInclude []string

	// Atomic counters for stats
	requests     int64
	toolsTotal   int64
	toolsSent    int64
	toolsSkipped int64
	tokensSaved  int64
}

// DefaultToolSelector returns a ToolSelector with sensible defaults.
func DefaultToolSelector() *ToolSelector {
	return &ToolSelector{
		MinScore: 0.1,
		MaxTools: 10,
	}
}

// Stats returns a snapshot of cumulative selection statistics.
func (ts *ToolSelector) Stats() ToolSelectorStats {
	return ToolSelectorStats{
		Requests:     atomic.LoadInt64(&ts.requests),
		ToolsTotal:   atomic.LoadInt64(&ts.toolsTotal),
		ToolsSent:    atomic.LoadInt64(&ts.toolsSent),
		ToolsSkipped: atomic.LoadInt64(&ts.toolsSkipped),
		TokensSaved:  atomic.LoadInt64(&ts.tokensSaved),
	}
}

// estimateToolTokens estimates the token count for a tool definition.
// Each tool has name, description, and JSON schema parameters.
func estimateToolTokens(def ToolDefinition) int {
	// Base: name + description (~tokens ≈ words * 1.3)
	words := len(strings.Fields(def.Name + " " + def.Description))
	tokens := int(float64(words) * 1.3)
	if tokens < 10 {
		tokens = 10
	}
	// Parameters schema adds significant tokens
	if len(def.Parameters) > 0 {
		b, _ := json.Marshal(def.Parameters)
		// JSON schema: ~1 token per 4 chars
		tokens += len(b) / 4
	}
	return tokens
}

// toolKeywords maps tool names to additional bilingual keywords for matching.
// This bridges the gap between Chinese queries and English tool descriptions.
var toolKeywords = map[string]string{
	"calculator":     "计算 算术 数学 math calculate compute arithmetic tip percentage",
	"weather":        "天气 气温 温度 湿度 forecast climate",
	"datetime":       "时间 日期 日历 时区 date time timezone clock",
	"notes":          "笔记 备忘 记录 note memo write",
	"push_notification": "推送 通知 提醒 闹钟 提示 叫我 提醒我 别忘了 记得 到时候 定时 起床 push notify notification remind alarm alert wake schedule",
	"tasks":          "任务 待办 todo task priority status",
	"translate":      "翻译 语言 translate language",
	"search":         "搜索 查找 查询 search find lookup",
	"browser":        "浏览器 网页 网站 打开 访问 browse web page navigate url screenshot",
	"ui_reviewer":    "UI UX 界面 设计 评估 评价 审查 review evaluate assess quality design visual accessibility website",
	"memory":         "记忆 回忆 记住 忘记 memory recall remember forget store",
	"files":          "文件 读取 写入 目录 file read write directory folder",
	"docker":         "容器 镜像 docker container image volume",
	"network":        "网络 连接 DNS ping 诊断 network connectivity diagnostics",
	"sandbox":        "沙箱 执行 运行 sandbox execute run command",
	"scheduler":      "定时 计划 cron 调度 schedule cron job periodic",
	"system_info":    "系统 硬件 CPU GPU 内存 磁盘 system hardware info",
	"notifications":  "通知 消息 notify notification message alert",
	"unit_converter": "单位 转换 convert unit measurement length weight temperature",
	"workflows":      "工作流 自动化 流程 workflow automation pipeline",
}

// toolDoc combines a tool's name, description, and bilingual keywords
// into a single searchable document.
func toolDoc(def ToolDefinition) string {
	doc := def.Name + " " + def.Description
	if kw, ok := toolKeywords[def.Name]; ok {
		doc += " " + kw
	}
	return doc
}

// Select filters tool definitions to those relevant to the user query.
// Returns all tools if query is empty or if fewer than MaxTools match.
func (ts *ToolSelector) Select(query string, allDefs []ToolDefinition) []ToolDefinition {
	if len(allDefs) == 0 || query == "" {
		return allDefs
	}

	maxTools := ts.MaxTools
	if maxTools <= 0 {
		maxTools = 10
	}

	// If we have fewer tools than max, return all (no point filtering)
	if len(allDefs) <= maxTools {
		return allDefs
	}

	// Build always-include set
	alwaysSet := make(map[string]bool, len(ts.AlwaysInclude))
	for _, name := range ts.AlwaysInclude {
		alwaysSet[name] = true
	}

	// Tokenize query
	queryTokens := pruner.TextTokenize(query)
	if len(queryTokens) == 0 {
		return allDefs
	}

	// Build BM25 scorer with tool documents as corpus
	scorer := pruner.NewBM25Scorer(1.2, 0.75)
	segments := make([]pruner.Segment, len(allDefs))
	for i, def := range allDefs {
		doc := toolDoc(def)
		segments[i] = pruner.Segment{
			Content:   doc,
			Tokens:    pruner.TextTokenize(doc),
			StartLine: i,
			EndLine:   i,
		}
	}

	scored := scorer.Score(query, segments)

	// Also do direct keyword matching as a fallback for short/Chinese queries
	keywordHits := make(map[int]float64)
	queryLower := strings.ToLower(query)
	for i, def := range allDefs {
		kw, ok := toolKeywords[def.Name]
		if !ok {
			continue
		}
		// Check if any keyword appears in the query
		for _, word := range strings.Fields(kw) {
			if strings.Contains(queryLower, strings.ToLower(word)) {
				keywordHits[i] += 1.0
			}
		}
		// Also check if tool name appears in query
		if strings.Contains(queryLower, strings.ToLower(def.Name)) {
			keywordHits[i] += 2.0
		}
	}

	// Merge BM25 scores with keyword hits
	type candidate struct {
		idx   int
		def   ToolDefinition
		score float64
	}

	scoreMap := make(map[int]*candidate)

	// Add BM25 scores
	for _, ss := range scored {
		idx := ss.Segment.StartLine
		if idx < 0 || idx >= len(allDefs) {
			continue
		}
		scoreMap[idx] = &candidate{idx: idx, def: allDefs[idx], score: ss.Score}
	}

	// Boost with keyword hits
	for idx, hits := range keywordHits {
		if c, ok := scoreMap[idx]; ok {
			c.score += hits
		} else {
			scoreMap[idx] = &candidate{idx: idx, def: allDefs[idx], score: hits}
		}
	}

	// Sort by score descending
	sorted := make([]*candidate, 0, len(scoreMap))
	for _, c := range scoreMap {
		sorted = append(sorted, c)
	}
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].score > sorted[i].score {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Collect results
	var results []ToolDefinition
	included := make(map[string]bool)

	// Always-include tools first
	for _, def := range allDefs {
		if alwaysSet[def.Name] {
			results = append(results, def)
			included[def.Name] = true
		}
	}

	// Add scored candidates
	minScore := ts.MinScore
	if minScore <= 0 {
		minScore = 0.1
	}

	for _, c := range sorted {
		if len(results) >= maxTools {
			break
		}
		if included[c.def.Name] {
			continue
		}
		if c.score >= minScore {
			results = append(results, c.def)
			included[c.def.Name] = true
		}
	}

	// Fallback: if we got very few results, include top candidates
	if len(results) < 3 && len(sorted) > 0 {
		for _, c := range sorted {
			if len(results) >= 3 {
				break
			}
			if !included[c.def.Name] {
				results = append(results, c.def)
				included[c.def.Name] = true
			}
		}
	}

	// Record stats
	total := int64(len(allDefs))
	sent := int64(len(results))
	skipped := total - sent
	atomic.AddInt64(&ts.requests, 1)
	atomic.AddInt64(&ts.toolsTotal, total)
	atomic.AddInt64(&ts.toolsSent, sent)
	atomic.AddInt64(&ts.toolsSkipped, skipped)
	// Estimate tokens saved from skipped tools
	var savedTokens int64
	for i, def := range allDefs {
		if !included[def.Name] {
			_ = i
			savedTokens += int64(estimateToolTokens(def))
		}
	}
	atomic.AddInt64(&ts.tokensSaved, savedTokens)

	return results
}

// SelectFromRegistry is a convenience method that gets definitions from a registry
// and filters them based on the query.
func (ts *ToolSelector) SelectFromRegistry(query string, registry *Registry) []ToolDefinition {
	return ts.Select(query, registry.Definitions())
}
