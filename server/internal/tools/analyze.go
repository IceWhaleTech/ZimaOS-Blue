package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
	"github.com/google/uuid"
)

const (
	analyzeMaxURLs           = 5
	analyzeMaxSearches       = 3
	analyzeMaxTextLen        = 50000
	analyzeWebReadMaxChars   = 15000
	analyzeScrapeMaxAttempts = 3
	analyzeScrapeIdleMS      = 400
	analyzeScrapeTimeoutMS   = 2000

	// analyzeLLMRequestTimeout keeps long-form analysis/report calls from being
	// cut off by bridge fallback timeouts when the parent context has no deadline.
	analyzeLLMRequestTimeout = 5 * time.Minute
	analyzeDocExtractTimeout = 5 * time.Minute

	analyzeDocExtractAutoRollbackMinAttempts = 40
	analyzeDocExtractAutoRollbackMaxFailRate = 0.15
	analyzeFallbackReasonAutoRollbackDoc     = "auto_rollback_doc_extract_fallback_rate"
)

var analyzeTopicURLPattern = regexp.MustCompile(`https?://[^\s<>"']+`)

// AnalyzeTool performs deep-dive content analysis and can return either an
// inline structured answer or an explicit HTML report.
type AnalyzeTool struct {
	mu       sync.RWMutex
	bridge   LLMBridge
	browser  BrowserBackend
	executor *Executor
	mediaDir string

	smallModel             smallmodel.Runtime
	smallModelEnabledFn    func() bool
	smallModelDocExtractFn func() bool
	smallModelSetDocFn     func(bool) (bool, error)
	smallModelStats        SmallModelStatsRecorder
	docExtractAttempts     int64
	docExtractSuccess      int64
	docExtractGateAttempts int64
	docExtractGateSuccess  int64
}

type analyzeGatherStats struct {
	RequestedSources int
	CollectedSources int
	TextSources      int
	TextChars        int
	URLSources       int
	URLCollected     int
	SearchSources    int
	SearchCollected  int
}

type analyzeDocExtractResult struct {
	content string
	status  string
	detail  string
}

// SmallModelStatsRecorder records small-model scene-level observability counters.
type SmallModelStatsRecorder interface {
	RecordDocExtractAttempt()
	RecordDocExtractSuccess()
	RecordLatency(time.Duration)
	RecordLatencyWithScene(string, time.Duration)
	RecordFallback(reason string)
	RecordAutoRollback()
}

// NewAnalyzeTool creates a new analyze tool.
func NewAnalyzeTool() *AnalyzeTool {
	return &AnalyzeTool{}
}

// SetLLMBridge injects the LLM bridge for analysis.
func (t *AnalyzeTool) SetLLMBridge(bridge LLMBridge) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.bridge = bridge
}

// SetBrowser injects the browser backend for URL scraping.
func (t *AnalyzeTool) SetBrowser(browser BrowserBackend) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.browser = browser
}

// SetExecutor injects the tool executor for calling web_search.
func (t *AnalyzeTool) SetExecutor(executor *Executor) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.executor = executor
}

// SetMediaDir sets the directory for persisting HTML reports.
func (t *AnalyzeTool) SetMediaDir(dir string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.mediaDir = dir
}

// SetSmallModelRuntime injects optional small-model runtime used for doc extraction pre-pass.
func (t *AnalyzeTool) SetSmallModelRuntime(rt smallmodel.Runtime) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.smallModel = rt
}

// SetSmallModelSwitchFuncs injects runtime switch readers.
func (t *AnalyzeTool) SetSmallModelSwitchFuncs(enabledFn, docExtractFn func() bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.smallModelEnabledFn = enabledFn
	t.smallModelDocExtractFn = docExtractFn
}

// SetSmallModelDocExtractToggle wires a persistence setter used by auto-rollback gate.
func (t *AnalyzeTool) SetSmallModelDocExtractToggle(setter func(bool) (bool, error)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.smallModelSetDocFn = setter
}

// SetSmallModelStatsRecorder injects cross-module small-model counters.
func (t *AnalyzeTool) SetSmallModelStatsRecorder(recorder SmallModelStatsRecorder) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.smallModelStats = recorder
}

// Definition returns the tool's definition.
func (t *AnalyzeTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "analyze",
		Description: "Deep-dive analysis tool. Gathers data from URLs, search queries, and direct text, then returns an inline structured answer by default. Only generate an HTML report when the user explicitly asks for a report or dashboard.",
		Icon:        "analyze",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"topic": map[string]interface{}{
					"type":        "string",
					"description": "Analysis topic / report title. Must accurately reflect the user's request, in the user's language.",
				},
				"urls": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "URLs to scrape for content (max 5)",
				},
				"text": map[string]interface{}{
					"type":        "string",
					"description": "Direct text content to include in analysis",
				},
				"search_queries": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Web search queries to gather additional data (max 3)",
				},
				"lang": map[string]interface{}{
					"type":        "string",
					"description": "Output language (default: zh-CN). Examples: zh-CN, en-US",
				},
				"output_mode": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"inline", "report"},
					"description": "Return mode. Defaults to inline. Use report only when an HTML report or dashboard is explicitly requested.",
				},
				"report": map[string]interface{}{
					"type":        "boolean",
					"description": "Compatibility alias for output_mode=report.",
				},
			},
			"required": []string{"topic"},
		},
	}
}

// Execute runs the analysis pipeline.
func (t *AnalyzeTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	normalizeAnalyzeToolArgs(args)
	topic := firstCompatString(args, "topic", "subject")
	if topic == "" {
		return nil, errors.New("topic is required")
	}
	promoteAnalyzeTopicSources(args, topic)
	if normalizedTopic := strings.TrimSpace(firstCompatString(args, "topic", "subject")); normalizedTopic != "" {
		topic = normalizedTopic
	}
	lang := firstCompatString(args, "lang", "language")
	if lang == "" {
		lang = GetLang(ctx)
	}

	t.mu.RLock()
	bridge := t.bridge
	browser := t.browser
	executor := t.executor
	mediaDir := t.mediaDir
	t.mu.RUnlock()

	if bridge == nil {
		return nil, errors.New("LLM bridge not available — cannot analyze")
	}

	return t.runFullAnalysis(ctx, topic, args, lang, resolveAnalyzeOutputMode(args), bridge, browser, executor, mediaDir)
}

func normalizeAnalyzeToolArgs(args map[string]interface{}) {
	if len(args) == 0 {
		return
	}

	if topic := strings.TrimSpace(firstCompatString(args, "topic", "subject")); topic != "" {
		args["topic"] = topic
	}

	if len(analyzeCollectStringInputs(args["urls"], analyzeMaxURLs)) == 0 {
		if url := strings.TrimSpace(firstCompatString(args, "url", "href", "link", "source")); url != "" {
			args["urls"] = []interface{}{url}
		}
	}

	if len(analyzeCollectStringInputs(args["search_queries"], analyzeMaxSearches)) == 0 {
		if raw, ok := compatArgValue(args, "searchQueries", "queries"); ok {
			args["search_queries"] = raw
		} else if query := strings.TrimSpace(firstCompatString(args, "query")); query != "" {
			args["search_queries"] = []interface{}{query}
		}
	}

	if strings.TrimSpace(firstCompatString(args, "text")) == "" {
		if text := strings.TrimSpace(firstCompatString(args, "content")); text != "" {
			args["text"] = text
		}
	}
}

func promoteAnalyzeTopicSources(args map[string]interface{}, topic string) {
	if len(args) == 0 {
		return
	}
	if len(analyzeCollectStringInputs(args["urls"], analyzeMaxURLs)) > 0 {
		return
	}

	urls := extractAnalyzeTopicURLs(topic, analyzeMaxURLs)
	if len(urls) == 0 {
		return
	}

	promoted := make([]interface{}, 0, len(urls))
	for _, url := range urls {
		promoted = append(promoted, url)
	}
	args["urls"] = promoted

	if cleaned := stripAnalyzeTopicURLs(topic); cleaned != "" && cleaned != topic {
		args["topic"] = cleaned
	}
}

func extractAnalyzeTopicURLs(topic string, limit int) []string {
	matches := analyzeTopicURLPattern.FindAllString(topic, -1)
	if len(matches) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = len(matches)
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		candidate := strings.TrimSpace(match)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func stripAnalyzeTopicURLs(topic string) string {
	cleaned := analyzeTopicURLPattern.ReplaceAllString(topic, " ")
	return strings.TrimSpace(strings.Join(strings.Fields(cleaned), " "))
}

// runFullAnalysis gathers data from URLs/search, then generates a report.
func (t *AnalyzeTool) runFullAnalysis(ctx context.Context, topic string, args map[string]interface{}, lang, outputMode string, bridge LLMBridge, browser BrowserBackend, executor *Executor, mediaDir string) (interface{}, error) {
	rawContent, stats := t.gatherData(ctx, args, lang, browser, executor)
	if rawContent == "" {
		return nil, errors.New("no data collected — provide URLs, search queries, or text")
	}
	emitAnalyzeCollectionCard(ctx, lang, stats)

	return t.analyzeAndGenerate(ctx, topic, rawContent, lang, outputMode, bridge, mediaDir)
}

// analyzeAndGenerate runs the LLM analysis pipeline and optionally generates HTML.
func (t *AnalyzeTool) analyzeAndGenerate(ctx context.Context, topic, rawContent, lang, outputMode string, bridge LLMBridge, mediaDir string) (interface{}, error) {
	emitAnalyzeProgress(ctx, "doc_extract", analyzeProgressLabel(lang, "doc_extract", 0, 0), "running")
	docExtract := t.smallModelDocExtract(ctx, topic, rawContent, lang)
	if docExtract.content != "" {
		rawContent = docExtract.content
	}
	emitAnalyzeProgress(ctx, "doc_extract", analyzeProgressLabel(lang, "doc_extract", 0, 0), docExtract.status, map[string]interface{}{"detail": docExtract.detail})

	// 2. LLM analysis — extract structured data
	emitAnalyzeProgress(ctx, "analysis", analyzeProgressLabel(lang, "analysis", 0, 0), "running")
	analysisJSON, err := t.llmExtractAndAnalyze(ctx, bridge, topic, rawContent, lang)
	if err != nil {
		emitAnalyzeProgress(ctx, "analysis", analyzeProgressLabel(lang, "analysis", 0, 0), "failed")
		return nil, fmt.Errorf("analysis failed: %w", err)
	}
	emitAnalyzeProgress(ctx, "analysis", analyzeProgressLabel(lang, "analysis", 0, 0), "success")

	// Extract refined_title from analysis JSON if present
	reportTitle := topic
	var analysisData map[string]interface{}
	if json.Unmarshal([]byte(analysisJSON), &analysisData) == nil {
		if rt, ok := analysisData["refined_title"].(string); ok && rt != "" {
			reportTitle = rt
		}
	}

	if outputMode != "report" {
		emitAnalyzeProgress(ctx, "report", analyzeProgressLabel(lang, "report", 0, 0), "skipped", map[string]interface{}{
			"detail": analyzeLocalized(lang, "Inline answer requested; skipping HTML report generation", "已请求内联回答，跳过 HTML 报告生成"),
		})
		emitAnalyzeProgress(ctx, "save_report", analyzeProgressLabel(lang, "save_report", 0, 0), "skipped", map[string]interface{}{
			"detail": analyzeLocalized(lang, "No report file needed for inline mode", "内联模式无需生成报告文件"),
		})
		return buildAnalyzeInlineResult(reportTitle, analysisJSON, analysisData, lang), nil
	}

	// 3. Generate HTML report
	emitAnalyzeProgress(ctx, "report", analyzeProgressLabel(lang, "report", 0, 0), "running")
	htmlBody, err := t.llmGenerateHTML(ctx, bridge, reportTitle, analysisJSON, lang)
	if err != nil {
		emitAnalyzeProgress(ctx, "report", analyzeProgressLabel(lang, "report", 0, 0), "failed")
		return nil, fmt.Errorf("report generation failed: %w", err)
	}

	fullHTML := buildAnalyzeHTML(reportTitle, lang, htmlBody)
	emitAnalyzeProgress(ctx, "report", analyzeProgressLabel(lang, "report", 0, 0), "success")

	// 4. Save to disk
	emitAnalyzeProgress(ctx, "save_report", analyzeProgressLabel(lang, "save_report", 0, 0), "running")
	reportURL, saveErr := t.saveReport(fullHTML, mediaDir)
	switch {
	case saveErr != nil:
		emitAnalyzeProgress(ctx, "save_report", analyzeProgressLabel(lang, "save_report", 0, 0), "failed", map[string]interface{}{"detail": analyzeLocalized(lang, "Could not save HTML report", "无法保存 HTML 报告")})
	case reportURL != "":
		emitAnalyzeProgress(ctx, "save_report", analyzeProgressLabel(lang, "save_report", 0, 0), "success", map[string]interface{}{"detail": analyzeLocalized(lang, "HTML report saved", "HTML 报告已保存")})
	default:
		emitAnalyzeProgress(ctx, "save_report", analyzeProgressLabel(lang, "save_report", 0, 0), "skipped", map[string]interface{}{"detail": analyzeLocalized(lang, "Media directory unavailable; returning the result inline", "媒体目录不可用，仅通过结果返回")})
	}

	result := map[string]interface{}{
		"success": true,
		"topic":   reportTitle,
		"message": fmt.Sprintf("Analysis report generated: %s", reportTitle),
	}
	if reportURL != "" {
		result["report_url"] = reportURL
		result["message"] = fmt.Sprintf("Analysis report generated: %s\nReport link: %s\nThe URL is a relative path — do NOT add file:// or any host prefix. Do NOT open this URL with the browser tool. Just present the link to the user.", reportTitle, reportURL)
	}

	b, _ := json.Marshal(result)
	return string(b), nil
}

func resolveAnalyzeOutputMode(args map[string]interface{}) string {
	mode := strings.ToLower(strings.TrimSpace(firstCompatString(args, "output_mode", "outputMode")))
	switch mode {
	case "report", "html", "dashboard", "visual", "visual_report":
		return "report"
	case "inline", "answer", "structured", "":
	default:
		return "inline"
	}
	if mode == "inline" {
		return "inline"
	}
	if raw, ok := compatArgValue(args, "report", "generate_report", "generateReport", "html_report", "htmlReport"); ok && compatBoolValue(raw, false) {
		return "report"
	}
	return "inline"
}

func buildAnalyzeInlineResult(topic, analysisJSON string, analysisData map[string]interface{}, lang string) string {
	result := map[string]interface{}{
		"success":     true,
		"topic":       topic,
		"output_mode": "inline",
		"message":     analyzeLocalized(lang, fmt.Sprintf("Analysis ready: %s", topic), fmt.Sprintf("分析已完成：%s", topic)),
		"answer":      buildAnalyzeInlineAnswer(topic, analysisData, analysisJSON, lang),
	}
	if len(analysisData) > 0 {
		result["analysis"] = analysisData
		if summary := strings.TrimSpace(firstAnalyzeStringValue(analysisData["summary"])); summary != "" {
			result["summary"] = summary
		}
	}
	b, _ := json.Marshal(result)
	return string(b)
}

func buildAnalyzeInlineAnswer(topic string, analysisData map[string]interface{}, fallbackJSON, lang string) string {
	lines := make([]string, 0, 5)
	if summary := strings.TrimSpace(firstAnalyzeStringValue(analysisData["summary"])); summary != "" {
		lines = append(lines, summary)
	} else if strings.TrimSpace(topic) != "" {
		lines = append(lines, analyzeLocalized(lang, fmt.Sprintf("Analysis completed for %s.", topic), fmt.Sprintf("已完成对 %s 的分析。", topic)))
	}
	if stats := analyzeInlineSection(analysisData["stats"], 3); len(stats) > 0 {
		lines = append(lines, analyzeLocalized(lang, "Key stats: ", "关键数据：")+strings.Join(stats, "; "))
	}
	if insights := analyzeInlineSection(analysisData["insights"], 3); len(insights) > 0 {
		lines = append(lines, analyzeLocalized(lang, "Key insights: ", "关键洞察：")+strings.Join(insights, "; "))
	}
	if recommendations := analyzeInlineSection(analysisData["recommendations"], 3); len(recommendations) > 0 {
		lines = append(lines, analyzeLocalized(lang, "Recommended next steps: ", "建议下一步：")+strings.Join(recommendations, "; "))
	}
	if len(lines) == 0 {
		return strings.TrimSpace(fallbackJSON)
	}
	return strings.Join(lines, "\n")
}

func analyzeInlineSection(raw interface{}, limit int) []string {
	items := make([]string, 0, limit)
	appendItem := func(text string) {
		text = strings.TrimSpace(text)
		if text == "" || len(items) >= limit {
			return
		}
		items = append(items, text)
	}
	switch typed := raw.(type) {
	case string:
		appendItem(typed)
	case []string:
		for _, item := range typed {
			appendItem(item)
		}
	case []interface{}:
		for _, item := range typed {
			if len(items) >= limit {
				break
			}
			switch entry := item.(type) {
			case string:
				appendItem(entry)
			case map[string]interface{}:
				appendItem(firstAnalyzeStringValue(entry["value"], entry["label"], entry["name"], entry["title"], entry["summary"], entry["text"]))
			default:
				appendItem(fmt.Sprintf("%v", entry))
			}
		}
	case map[string]interface{}:
		appendItem(firstAnalyzeStringValue(typed["value"], typed["label"], typed["name"], typed["title"], typed["summary"], typed["text"]))
	}
	return items
}

func firstAnalyzeStringValue(values ...interface{}) string {
	for _, value := range values {
		switch typed := value.(type) {
		case string:
			if trimmed := strings.TrimSpace(typed); trimmed != "" {
				return trimmed
			}
		case fmt.Stringer:
			if trimmed := strings.TrimSpace(typed.String()); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

// gatherData collects content from URLs, search queries, and direct text.
func (t *AnalyzeTool) gatherData(ctx context.Context, args map[string]interface{}, lang string, browser BrowserBackend, executor *Executor) (string, analyzeGatherStats) {
	var parts []string
	stats := analyzeGatherStats{}
	urlsValue, _ := compatArgValue(args, "urls")
	queriesValue, _ := compatArgValue(args, "search_queries", "searchQueries", "queries")
	text := strings.TrimSpace(firstCompatString(args, "text", "content"))
	urls := analyzeCollectStringInputs(urlsValue, analyzeMaxURLs)
	queries := analyzeCollectStringInputs(queriesValue, analyzeMaxSearches)

	if text != "" {
		stats.RequestedSources++
		stats.TextSources = 1
	}
	stats.URLSources = len(urls)
	stats.SearchSources = len(queries)
	stats.RequestedSources += stats.URLSources + stats.SearchSources

	emitAnalyzeProgress(ctx, "data_collection", analyzeProgressLabel(lang, "data_collection", 0, 0), "running", map[string]interface{}{
		"detail":  analyzeCollectionPlanDetail(lang, stats),
		"current": 0,
		"total":   stats.RequestedSources,
	})

	// Direct text
	if text != "" {
		emitAnalyzeProgress(ctx, "text_input", analyzeProgressLabel(lang, "text_input", 0, 0), "running", map[string]interface{}{
			"detail": analyzeLocalized(lang, "Preparing direct text input", "整理直接输入文本"),
		})
		if len(text) > analyzeMaxTextLen {
			text = text[:analyzeMaxTextLen]
		}
		stats.TextChars = utf8.RuneCountInString(text)
		stats.CollectedSources++
		parts = append(parts, "=== Direct Input ===\n"+text)
		emitAnalyzeProgress(ctx, "text_input", analyzeProgressLabel(lang, "text_input", 0, 0), "success", map[string]interface{}{
			"detail":     analyzeLocalized(lang, fmt.Sprintf("%d chars", stats.TextChars), fmt.Sprintf("%d 个字符", stats.TextChars)),
			"char_count": stats.TextChars,
		})
	}

	// URLs — scrape via browser
	if len(urls) > 0 {
		if executor == nil && browser == nil {
			emitAnalyzeProgress(ctx, "url_fetch_backend", analyzeLocalized(lang, "Webpage collection unavailable", "网页采集不可用"), "failed", map[string]interface{}{
				"detail": analyzeLocalized(lang, "No page-reading backend available", "缺少网页读取后端"),
			})
		} else {
			for i, url := range urls {
				stepID := fmt.Sprintf("url_fetch_%d", i+1)
				emitAnalyzeProgress(ctx, stepID, analyzeProgressLabel(lang, "url_fetch", i+1, len(urls)), "running", map[string]interface{}{
					"current":      i + 1,
					"total":        len(urls),
					"source_kind":  "url",
					"source_label": url,
				})

				content, requiresBrowser := t.readURL(ctx, executor, url)
				if content == "" || requiresBrowser {
					if browserContent := t.scrapeURL(ctx, browser, url); browserContent != "" {
						content = browserContent
					}
				}

				if content != "" {
					stats.CollectedSources++
					stats.URLCollected++
					parts = append(parts, fmt.Sprintf("=== URL: %s ===\n%s", url, content))
					emitAnalyzeProgress(ctx, stepID, analyzeProgressLabel(lang, "url_fetch", i+1, len(urls)), "success", map[string]interface{}{
						"current":      i + 1,
						"total":        len(urls),
						"source_kind":  "url",
						"source_label": url,
						"char_count":   utf8.RuneCountInString(content),
						"detail":       analyzeLocalized(lang, "Page content extracted", "网页内容已提取"),
					})
					continue
				}

				emitAnalyzeProgress(ctx, stepID, analyzeProgressLabel(lang, "url_fetch", i+1, len(urls)), "failed", map[string]interface{}{
					"current":      i + 1,
					"total":        len(urls),
					"source_kind":  "url",
					"source_label": url,
					"detail":       analyzeLocalized(lang, "Could not extract page content", "无法提取网页内容"),
				})
			}
		}
	}

	// Search queries — via web_search tool
	if len(queries) > 0 {
		if executor == nil {
			emitAnalyzeProgress(ctx, "search_backend", analyzeLocalized(lang, "Web search unavailable", "网页搜索不可用"), "failed", map[string]interface{}{
				"detail": analyzeLocalized(lang, "Search executor unavailable", "搜索执行器不可用"),
			})
		} else {
			for i, query := range queries {
				stepID := fmt.Sprintf("search_%d", i+1)
				emitAnalyzeProgress(ctx, stepID, analyzeProgressLabel(lang, "search", i+1, len(queries)), "running", map[string]interface{}{
					"current":      i + 1,
					"total":        len(queries),
					"source_kind":  "search",
					"source_label": query,
				})
				content, resultCount := t.webSearch(ctx, executor, query)
				if content != "" {
					stats.CollectedSources++
					stats.SearchCollected++
					parts = append(parts, fmt.Sprintf("=== Search: %s ===\n%s", query, content))
					emitAnalyzeProgress(ctx, stepID, analyzeProgressLabel(lang, "search", i+1, len(queries)), "success", map[string]interface{}{
						"current":      i + 1,
						"total":        len(queries),
						"source_kind":  "search",
						"source_label": query,
						"result_count": resultCount,
						"detail":       analyzeLocalized(lang, fmt.Sprintf("%d results", resultCount), fmt.Sprintf("%d 条结果", resultCount)),
					})
					continue
				}
				emitAnalyzeProgress(ctx, stepID, analyzeProgressLabel(lang, "search", i+1, len(queries)), "failed", map[string]interface{}{
					"current":      i + 1,
					"total":        len(queries),
					"source_kind":  "search",
					"source_label": query,
					"detail":       analyzeLocalized(lang, "No search results collected", "未收集到搜索结果"),
				})
			}
		}
	}

	rawContent := strings.Join(parts, "\n\n")
	if rawContent == "" {
		emitAnalyzeProgress(ctx, "data_collection", analyzeProgressLabel(lang, "data_collection", 0, 0), "failed", map[string]interface{}{
			"detail":  analyzeCollectionResultDetail(lang, stats),
			"current": stats.CollectedSources,
			"total":   stats.RequestedSources,
		})
		return "", stats
	}
	emitAnalyzeProgress(ctx, "data_collection", analyzeProgressLabel(lang, "data_collection", 0, 0), "success", map[string]interface{}{
		"detail":  analyzeCollectionResultDetail(lang, stats),
		"current": stats.CollectedSources,
		"total":   stats.RequestedSources,
	})
	return rawContent, stats
}

func (t *AnalyzeTool) shouldUseSmallModelDocExtract() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.smallModel == nil || t.smallModelEnabledFn == nil || t.smallModelDocExtractFn == nil {
		return false
	}
	return t.smallModelEnabledFn() && t.smallModelDocExtractFn()
}

// smallModelDocExtract performs a lightweight structured extraction pre-pass.
// It returns augmented raw content for the main analysis step, or empty when skipped.
func (t *AnalyzeTool) smallModelDocExtract(ctx context.Context, topic, rawContent, lang string) analyzeDocExtractResult {
	if !t.shouldUseSmallModelDocExtract() {
		return analyzeDocExtractResult{status: "skipped", detail: analyzeLocalized(lang, "Small-model preprocessing skipped", "已跳过小模型预处理")}
	}
	raw := strings.TrimSpace(rawContent)
	if len(raw) < 80 {
		return analyzeDocExtractResult{status: "skipped", detail: analyzeLocalized(lang, "Source content is already concise", "原始内容已较为精简")}
	}
	if len(raw) > 12000 {
		raw = raw[:12000] + "\n... (truncated)"
	}

	t.mu.RLock()
	rt := t.smallModel
	stats := t.smallModelStats
	t.mu.RUnlock()
	if rt == nil {
		return analyzeDocExtractResult{status: "skipped", detail: analyzeLocalized(lang, "Small-model runtime unavailable", "小模型运行时不可用")}
	}

	prompt := fmt.Sprintf(`Extract a concise structured workflow summary for topic "%s".
Return plain text with exactly these sections:
Objective:
Inputs:
Steps:
Outputs:
Risks:

Language: %s

Source:
---
%s
---`, topic, lang, raw)

	smCtx, cancel := context.WithTimeout(ctx, analyzeDocExtractTimeout)
	defer cancel()

	if stats != nil {
		stats.RecordDocExtractAttempt()
	}
	t.mu.Lock()
	t.docExtractAttempts++
	t.mu.Unlock()
	defer t.maybeAutoRollbackDocExtract()
	started := time.Now()
	resp, err := rt.Generate(smCtx, smallmodel.GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   320,
		Temperature: 0.1,
	})
	if stats != nil {
		stats.RecordLatencyWithScene("doc_extract", time.Since(started))
	}
	if err != nil {
		if stats != nil {
			stats.RecordFallback(analyzeSmallModelFallbackReason(err))
		}
		return analyzeDocExtractResult{status: "skipped", detail: analyzeLocalized(lang, "Small-model preprocessing unavailable; continuing with the main model", "小模型预处理不可用，继续使用主模型")}
	}
	if resp == nil {
		if stats != nil {
			stats.RecordFallback(smallmodel.FallbackReasonResourceGuard)
		}
		return analyzeDocExtractResult{status: "skipped", detail: analyzeLocalized(lang, "Small-model preprocessing unavailable; continuing with the main model", "小模型预处理不可用，继续使用主模型")}
	}
	extracted := strings.TrimSpace(resp.Text)
	if extracted == "" {
		if stats != nil {
			stats.RecordFallback(smallmodel.FallbackReasonLowConfidence)
		}
		return analyzeDocExtractResult{status: "skipped", detail: analyzeLocalized(lang, "Small-model preprocessing was not confident enough", "小模型预处理置信度不足")}
	}
	if len(extracted) > 6000 {
		extracted = extracted[:6000] + "\n... (truncated)"
	}
	if stats != nil {
		stats.RecordDocExtractSuccess()
	}
	t.mu.Lock()
	t.docExtractSuccess++
	t.mu.Unlock()

	return analyzeDocExtractResult{
		content: "=== Small-model structured extraction ===\n" + extracted + "\n\n" + rawContent,
		status:  "success",
		detail:  analyzeLocalized(lang, "Added structured preprocessing context", "已补充结构化预处理上下文"),
	}
}

func (t *AnalyzeTool) maybeAutoRollbackDocExtract() {
	t.mu.RLock()
	docEnabledFn := t.smallModelDocExtractFn
	setDocFn := t.smallModelSetDocFn
	stats := t.smallModelStats
	attempts := t.docExtractAttempts
	success := t.docExtractSuccess
	gateAttempts := t.docExtractGateAttempts
	gateSuccess := t.docExtractGateSuccess
	t.mu.RUnlock()

	if docEnabledFn == nil || setDocFn == nil || !docEnabledFn() {
		return
	}
	windowAttempts := attempts - gateAttempts
	windowSuccess := success - gateSuccess
	if windowAttempts < analyzeDocExtractAutoRollbackMinAttempts {
		return
	}
	failures := windowAttempts - windowSuccess
	if failures <= 0 {
		t.mu.Lock()
		t.docExtractGateAttempts = attempts
		t.docExtractGateSuccess = success
		t.mu.Unlock()
		return
	}
	failRate := float64(failures) / float64(windowAttempts)
	if failRate < analyzeDocExtractAutoRollbackMaxFailRate {
		t.mu.Lock()
		t.docExtractGateAttempts = attempts
		t.docExtractGateSuccess = success
		t.mu.Unlock()
		return
	}

	changed, err := setDocFn(false)
	if err != nil || !changed {
		return
	}
	if stats != nil {
		stats.RecordAutoRollback()
		stats.RecordFallback(analyzeFallbackReasonAutoRollbackDoc)
	}
	t.mu.Lock()
	t.docExtractGateAttempts = attempts
	t.docExtractGateSuccess = success
	t.mu.Unlock()
}

func analyzeSmallModelFallbackReason(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, smallmodel.ErrCircuitOpen):
		return smallmodel.FallbackReasonCircuitOpen
	case errors.Is(err, smallmodel.ErrNotReady):
		return smallmodel.FallbackReasonModelUnready
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return smallmodel.FallbackReasonTimeout
	default:
		return smallmodel.FallbackReasonResourceGuard
	}
}

// scrapeURL uses the browser to navigate and extract page content.
func (t *AnalyzeTool) scrapeURL(ctx context.Context, browser BrowserBackend, url string) string {
	if err := browser.Start(ctx); err != nil {
		return ""
	}

	nav, err := browser.Navigate(ctx, url, "")
	if err != nil {
		return ""
	}
	defer func() { _ = browser.CloseTab(ctx, nav.TargetID) }()

	for attempt := 0; attempt < analyzeScrapeMaxAttempts; attempt++ {
		_ = browser.WaitNetworkIdle(ctx, nav.TargetID, analyzeScrapeIdleMS, analyzeScrapeTimeoutMS)
		a11y, err := browser.AccessibilityTree(ctx, nav.TargetID, 8)
		if err == nil {
			tree := strings.TrimSpace(a11y.Tree)
			if tree != "" {
				if len(tree) > 15000 {
					tree = tree[:15000] + "\n... (truncated)"
				}
				title := strings.TrimSpace(a11y.Title)
				if title == "" {
					title = strings.TrimSpace(nav.Title)
				}
				finalURL := strings.TrimSpace(a11y.URL)
				if finalURL == "" {
					finalURL = strings.TrimSpace(nav.URL)
				}
				if finalURL == "" {
					finalURL = url
				}
				return fmt.Sprintf("Title: %s\nURL: %s\n\n%s", title, finalURL, tree)
			}
		}
		if attempt == analyzeScrapeMaxAttempts-1 {
			break
		}
		if sleepErr := sleepWithContext(ctx, time.Duration(attempt+1)*250*time.Millisecond); sleepErr != nil {
			return ""
		}
	}
	return ""
}

func (t *AnalyzeTool) readURL(ctx context.Context, executor *Executor, url string) (string, bool) {
	if executor == nil {
		return "", false
	}

	result, err := executor.Execute(ctx, "web_read", map[string]interface{}{
		"url":       url,
		"format":    webReadFormatText,
		"max_chars": analyzeWebReadMaxChars,
	})
	if err != nil {
		return "", false
	}

	resultStr, ok := result.(string)
	if !ok {
		return "", false
	}

	var resp webReadResponse
	if err := json.Unmarshal([]byte(resultStr), &resp); err != nil {
		return "", false
	}

	content := strings.TrimSpace(resp.Content)
	if content == "" {
		return "", analyzeWebReadRequiresBrowser(resp)
	}

	details := make([]string, 0, 3)
	if title := strings.TrimSpace(resp.Title); title != "" {
		details = append(details, fmt.Sprintf("Title: %s", title))
	}
	finalURL := strings.TrimSpace(resp.FinalURL)
	if finalURL == "" {
		finalURL = strings.TrimSpace(resp.URL)
	}
	if finalURL == "" {
		finalURL = strings.TrimSpace(url)
	}
	if finalURL != "" {
		details = append(details, fmt.Sprintf("URL: %s", finalURL))
	}
	if source := strings.TrimSpace(resp.Source); source != "" {
		details = append(details, fmt.Sprintf("Source: %s", source))
	}

	if len(details) == 0 {
		return content, analyzeWebReadRequiresBrowser(resp)
	}
	return strings.Join(details, "\n") + "\n\n" + content, analyzeWebReadRequiresBrowser(resp)
}

func analyzeWebReadRequiresBrowser(resp webReadResponse) bool {
	if resp.InteractiveRequired {
		return true
	}
	for _, code := range resp.WarningCodes {
		switch strings.ToLower(strings.TrimSpace(code)) {
		case webFetchWarningCodeLoginWall, webFetchWarningCodeChallenge, webFetchWarningCodeBrowserRequired:
			return true
		}
	}
	return false
}

// webSearch calls the web_search tool via executor.
func (t *AnalyzeTool) webSearch(ctx context.Context, executor *Executor, query string) (string, int) {
	result, err := executor.Execute(ctx, "web_search", map[string]interface{}{
		"query":       query,
		"max_results": 5,
	})
	if err != nil {
		return "", 0
	}

	// Result is JSON string from WebSearchTool
	resultStr, ok := result.(string)
	if !ok {
		return "", 0
	}

	var searchResp WebSearchResponse
	if json.Unmarshal([]byte(resultStr), &searchResp) != nil {
		return resultStr, 0
	}

	var b strings.Builder
	for _, r := range searchResp.Results {
		fmt.Fprintf(&b, "- %s\n  %s\n  %s\n\n", r.Title, r.URL, r.Description)
	}
	count := searchResp.TotalCount
	if count <= 0 {
		count = len(searchResp.Results)
	}
	return b.String(), count
}

// llmExtractAndAnalyze calls LLM to extract structured data and generate insights.
func (t *AnalyzeTool) llmExtractAndAnalyze(ctx context.Context, bridge LLMBridge, topic, rawContent, lang string) (string, error) {
	langName := langDisplayName(lang)

	// Truncate raw content to fit context
	if len(rawContent) > 30000 {
		rawContent = rawContent[:30000] + "\n... (truncated)"
	}

	prompt := fmt.Sprintf(`You are a senior data analyst. Analyze the following raw content about "%s" and produce a comprehensive structured analysis.

Raw Content:
---
%s
---

Return ONLY valid JSON (no markdown fences, no explanation) with this structure:
{
  "refined_title": "A more accurate, concise report title based on the actual content (same language as content)",
  "summary": "2-3 sentence executive summary",
  "stats": [
    {"label": "...", "value": "...", "color": "blue|green|orange|red"}
  ],
  "themes": [
    {"name": "...", "count": N, "sentiment": "positive|negative|neutral|mixed", "description": "..."}
  ],
  "quotes": [
    {"text": "exact quote...", "author": "username/source", "sentiment": "positive|negative|neutral"}
  ],
  "insights": [
    {"type": "strength|opportunity|challenge|risk", "title": "...", "description": "..."}
  ],
  "recommendations": [
    {"priority": "high|medium|low", "title": "...", "description": "..."}
  ]
}

Requirements:
- Extract 4-8 key statistics, distribute colors: 1st blue, 2nd green, 3rd orange, 4th red, then cycle
- Identify 4-8 major themes with frequency counts and sentiment
- Select 6-10 representative quotes with accurate attribution
- Generate 4-8 insights: mix of strength, opportunity, challenge, risk types
- Provide 3-6 prioritized recommendations
- All text in %s`, topic, rawContent, langName)

	callCtx, cancel := ensureAnalyzeLLMTimeout(ctx)
	defer cancel()

	return bridge.Chat(callCtx, prompt, 8000)
}

// llmGenerateHTML calls LLM to generate the HTML body content.
func (t *AnalyzeTool) llmGenerateHTML(ctx context.Context, bridge LLMBridge, topic, analysisJSON, lang string) (string, error) {
	langName := langDisplayName(lang)

	prompt := fmt.Sprintf(`You are an expert HTML report designer. Generate the HTML body content for an analysis report about "%s".

Analysis Data (JSON):
---
%s
---

IMPORTANT RULES:
1. Return ONLY raw HTML — no markdown fences, no <html>/<head>/<style> tags, no inline styles
2. Use ONLY these pre-defined CSS classes (the stylesheet is already included):

LAYOUT:
  .hero, .hero .badge, .hero h1, .hero .subtitle, .hero .meta-row, .hero .meta-item (.num, .label)
  .container, .toc, .toc-inner, .toc a
  .section, .section-header, .section-icon, .card, .grid-2, .grid-3

SECTION ICON BACKGROUNDS (use class, NOT inline style):
  .section-icon.bg-blue   → light blue  (for data/stats sections)
  .section-icon.bg-green  → light green (for positive/success sections)
  .section-icon.bg-orange → light amber (for recommendations/opportunities)
  .section-icon.bg-red    → light red   (for warnings/challenges)
  .section-icon.bg-purple → light purple (for showcase/community sections)

STAT BOXES (use color class for variety):
  .stat-grid, .stat-box (default blue), .stat-box.green, .stat-box.orange, .stat-box.red

INSIGHT BOXES (map insight type → color):
  .insight-box (default blue = strength), .insight-box.green (opportunity), .insight-box.orange (challenge), .insight-box.red (risk)

QUOTE CARDS (map sentiment → color):
  .quote-grid, .quote-card.positive (green), .quote-card.negative (red), .quote-card.neutral (blue), .quote-card.orange
  Inside: .quote-tag, .quote-text, .quote-author

TAG CLOUD (alternate colors for visual variety):
  .tag-cloud, .tag + color: .tag-blue, .tag-green, .tag-red, .tag-orange, .tag-purple
  Inside: .tag .count

PROGRESS BARS:
  .progress-item, .progress-label, .progress-bar, .progress-fill + color: .fill-blue, .fill-green, .fill-orange, .fill-red, .fill-purple

TABLES:
  .data-table OR .homelab-table (identical styling)

SHOWCASE CARDS:
  .homelab-card (.user, .setup-text, .tags), .mini-tag + color: .mt-blue, .mt-green, .mt-orange, .mt-purple, .mt-gray

OTHER:
  .highlight-strip, .divider, .chart-wrap, .chart-caption

3. COLOR MAPPING RULES — follow these strictly:
   - Stat boxes: distribute colors evenly (1st=blue, 2nd=green, 3rd=orange, 4th=red or vary)
   - Insight boxes: strength→default(blue), opportunity→green, challenge→orange, risk→red
   - Quote cards: positive→.positive, negative→.negative, neutral→.neutral, mixed→.orange
   - Tags: cycle through blue→green→orange→purple→red for visual variety
   - Progress bars: cycle through fill-blue→fill-green→fill-orange→fill-purple→fill-red
   - Section icons: each section should use a DIFFERENT bg-* color

4. STRUCTURE — include these sections in order:
   a. Hero: badge, h1 title, subtitle summary, meta-row with 3-4 key stats
   b. TOC: sticky nav with links to each section
   c. Overview: stat-grid with 4-8 colored stat-boxes + highlight-strip
   d. Themes: tag-cloud + progress bars showing theme frequency
   e. Key Findings: insight-boxes with proper color mapping per type
   f. Voices: quote-grid with sentiment-colored quote-cards
   g. Recommendations: cards in grid-2 layout
   h. Summary: data-table with key metrics

5. Use emoji icons in section headers (📊 📈 💡 🗣️ ✅ 📋 🎯 🔍)
6. All text content in %s
7. Make the report visually rich — use ALL available color variants, avoid monotone sections`, topic, analysisJSON, langName)

	callCtx, cancel := ensureAnalyzeLLMTimeout(ctx)
	defer cancel()

	return bridge.Chat(callCtx, prompt, 12000)
}

func ensureAnalyzeLLMTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, analyzeLLMRequestTimeout)
}

// saveReport writes the HTML to disk and returns the media URL.
func (t *AnalyzeTool) saveReport(html, mediaDir string) (string, error) {
	if mediaDir == "" || len(html) == 0 {
		return "", nil
	}
	dir := filepath.Join(mediaDir, "analyze")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", err
	}
	id := uuid.New().String()
	filename := id + ".html"
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(html), 0644); err != nil {
		return "", err
	}
	return "/api/v1/media/analyze/" + filename, nil
}

// langDisplayName returns a human-readable language name.
func langDisplayName(lang string) string {
	switch {
	case strings.HasPrefix(lang, "zh"):
		return "Chinese"
	case strings.HasPrefix(lang, "ja"):
		return "Japanese"
	case strings.HasPrefix(lang, "ko"):
		return "Korean"
	case strings.HasPrefix(lang, "fr"):
		return "French"
	case strings.HasPrefix(lang, "de"):
		return "German"
	case strings.HasPrefix(lang, "es"):
		return "Spanish"
	default:
		return "English"
	}
}

// emitAnalyzeProgress pushes a streaming progress card.
func emitAnalyzeProgress(ctx context.Context, stepID, stepName, status string, extras ...map[string]interface{}) {
	card := map[string]interface{}{
		"type":   "analyze-progress",
		"step":   stepID,
		"name":   stepName,
		"status": status,
	}
	if len(extras) > 0 {
		for key, value := range extras[0] {
			card[key] = value
		}
	}
	EmitCard(ctx, card)
}

func analyzeCollectStringInputs(raw interface{}, limit int) []string {
	if limit <= 0 || raw == nil {
		return nil
	}

	appendText := func(result []string, text string) []string {
		text = strings.TrimSpace(text)
		if text == "" {
			return result
		}
		result = append(result, text)
		if len(result) > limit {
			return result[:limit]
		}
		return result
	}

	result := make([]string, 0, 4)
	switch items := raw.(type) {
	case []interface{}:
		for _, item := range items {
			text, ok := item.(string)
			if !ok {
				continue
			}
			result = appendText(result, text)
			if len(result) >= limit {
				break
			}
		}
	case []string:
		for _, text := range items {
			result = appendText(result, text)
			if len(result) >= limit {
				break
			}
		}
	case string:
		fields := strings.FieldsFunc(items, func(r rune) bool { return r == '\n' || r == ',' })
		if len(fields) == 0 {
			fields = []string{items}
		}
		for _, text := range fields {
			result = appendText(result, text)
			if len(result) >= limit {
				break
			}
		}
	}
	return result
}

func analyzeLocalized(lang, en, zh string) string {
	if strings.HasPrefix(strings.ToLower(lang), "zh") {
		return zh
	}
	return en
}

func analyzeProgressLabel(lang, key string, current, total int) string {
	switch key {
	case "data_collection":
		return analyzeLocalized(lang, "Gathering data", "收集数据")
	case "text_input":
		return analyzeLocalized(lang, "Preparing input text", "整理输入文本")
	case "url_fetch":
		if current > 0 && total > 0 {
			return analyzeLocalized(lang, fmt.Sprintf("Fetching URL %d/%d", current, total), fmt.Sprintf("抓取网页 %d/%d", current, total))
		}
		return analyzeLocalized(lang, "Fetching URLs", "抓取网页")
	case "search":
		if current > 0 && total > 0 {
			return analyzeLocalized(lang, fmt.Sprintf("Searching web %d/%d", current, total), fmt.Sprintf("搜索资料 %d/%d", current, total))
		}
		return analyzeLocalized(lang, "Searching the web", "搜索资料")
	case "doc_extract":
		return analyzeLocalized(lang, "Structuring key points", "提炼结构化要点")
	case "analysis":
		return analyzeLocalized(lang, "Analyzing content", "分析内容")
	case "report":
		return analyzeLocalized(lang, "Generating report", "生成报告")
	case "save_report":
		return analyzeLocalized(lang, "Saving report", "保存报告")
	default:
		return key
	}
}

func analyzeCollectionPlanDetail(lang string, stats analyzeGatherStats) string {
	parts := make([]string, 0, 3)
	if stats.TextSources > 0 {
		parts = append(parts, analyzeLocalized(lang, "text input", "文本输入"))
	}
	if stats.URLSources > 0 {
		parts = append(parts, analyzeLocalized(lang, fmt.Sprintf("%d URLs", stats.URLSources), fmt.Sprintf("%d 个网页", stats.URLSources)))
	}
	if stats.SearchSources > 0 {
		parts = append(parts, analyzeLocalized(lang, fmt.Sprintf("%d searches", stats.SearchSources), fmt.Sprintf("%d 个搜索", stats.SearchSources)))
	}
	if len(parts) == 0 {
		return analyzeLocalized(lang, "No input sources", "没有输入来源")
	}
	return strings.Join(parts, " · ")
}

func analyzeCollectionResultDetail(lang string, stats analyzeGatherStats) string {
	if stats.RequestedSources == 0 {
		return analyzeLocalized(lang, "No data collected", "未收集到数据")
	}
	return analyzeLocalized(lang, fmt.Sprintf("%d/%d sources ready", stats.CollectedSources, stats.RequestedSources), fmt.Sprintf("已收集 %d/%d 个来源", stats.CollectedSources, stats.RequestedSources))
}

func emitAnalyzeCollectionCard(ctx context.Context, lang string, stats analyzeGatherStats) {
	details := []map[string]interface{}{
		{"label": "sources", "value": fmt.Sprintf("%d/%d", stats.CollectedSources, stats.RequestedSources)},
	}
	if stats.URLSources > 0 {
		details = append(details, map[string]interface{}{"label": "urls", "value": fmt.Sprintf("%d/%d", stats.URLCollected, stats.URLSources)})
	}
	if stats.SearchSources > 0 {
		details = append(details, map[string]interface{}{"label": "searches", "value": fmt.Sprintf("%d/%d", stats.SearchCollected, stats.SearchSources)})
	}
	if stats.TextSources > 0 {
		details = append(details, map[string]interface{}{"label": "text_chars", "value": stats.TextChars})
	}
	EmitCard(ctx, map[string]interface{}{
		"type":    "result",
		"title":   "analyze",
		"status":  "info",
		"message": analyzeLocalized(lang, fmt.Sprintf("Collected %d source(s)", stats.CollectedSources), fmt.Sprintf("已收集 %d 个来源", stats.CollectedSources)),
		"details": details,
	})
}
