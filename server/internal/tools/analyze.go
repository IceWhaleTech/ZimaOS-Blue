package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	analyzeMaxURLs     = 5
	analyzeMaxSearches = 3
	analyzeMaxTextLen  = 50000
)

// AnalyzeTool performs deep-dive content analysis and generates HTML reports.
type AnalyzeTool struct {
	mu       sync.RWMutex
	bridge   LLMBridge
	browser  BrowserBackend
	executor *Executor
	mediaDir string
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

// Definition returns the tool's definition.
func (t *AnalyzeTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "analyze",
		Description: "Deep-dive analysis tool. Gathers data from URLs and web searches, then generates a comprehensive HTML report with statistics, insights, and visualizations. Use when asked to analyze, research, or generate a report on a topic.",
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
			},
			"required": []string{"topic"},
		},
	}
}

// Execute runs the analysis pipeline.
func (t *AnalyzeTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	topic, _ := args["topic"].(string)
	if topic == "" {
		return nil, errors.New("topic is required")
	}
	lang, _ := args["lang"].(string)
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

	return t.runFullAnalysis(ctx, topic, args, lang, bridge, browser, executor, mediaDir)
}

// runFullAnalysis gathers data from URLs/search, then generates a report.
func (t *AnalyzeTool) runFullAnalysis(ctx context.Context, topic string, args map[string]interface{}, lang string, bridge LLMBridge, browser BrowserBackend, executor *Executor, mediaDir string) (interface{}, error) {
	// 1. Data collection
	emitAnalyzeProgress(ctx, "data_collection", "Gathering data", "running")
	rawContent := t.gatherData(ctx, args, browser, executor)
	if rawContent == "" {
		emitAnalyzeProgress(ctx, "data_collection", "Gathering data", "failed")
		return nil, errors.New("no data collected — provide URLs, search queries, or text")
	}
	emitAnalyzeProgress(ctx, "data_collection", "Gathering data", "success")

	return t.analyzeAndGenerate(ctx, topic, rawContent, lang, bridge, mediaDir)
}

// analyzeAndGenerate runs the LLM analysis pipeline and generates HTML.
func (t *AnalyzeTool) analyzeAndGenerate(ctx context.Context, topic, rawContent, lang string, bridge LLMBridge, mediaDir string) (interface{}, error) {
	// 2. LLM analysis — extract structured data
	emitAnalyzeProgress(ctx, "analysis", "Analyzing content", "running")
	analysisJSON, err := t.llmExtractAndAnalyze(ctx, bridge, topic, rawContent, lang)
	if err != nil {
		emitAnalyzeProgress(ctx, "analysis", "Analyzing content", "failed")
		return nil, fmt.Errorf("analysis failed: %w", err)
	}
	emitAnalyzeProgress(ctx, "analysis", "Analyzing content", "success")

	// Extract refined_title from analysis JSON if present
	reportTitle := topic
	var analysisData map[string]interface{}
	if json.Unmarshal([]byte(analysisJSON), &analysisData) == nil {
		if rt, ok := analysisData["refined_title"].(string); ok && rt != "" {
			reportTitle = rt
		}
	}

	// 3. Generate HTML report
	emitAnalyzeProgress(ctx, "report", "Generating report", "running")
	htmlBody, err := t.llmGenerateHTML(ctx, bridge, reportTitle, analysisJSON, lang)
	if err != nil {
		emitAnalyzeProgress(ctx, "report", "Generating report", "failed")
		return nil, fmt.Errorf("report generation failed: %w", err)
	}

	fullHTML := buildAnalyzeHTML(reportTitle, lang, htmlBody)

	// 4. Save to disk
	reportURL := t.saveReport(fullHTML, mediaDir)
	emitAnalyzeProgress(ctx, "report", "Generating report", "success")

	result := map[string]interface{}{
		"success": true,
		"topic":   reportTitle,
		"message": fmt.Sprintf("Analysis report generated: %s", reportTitle),
	}
	if reportURL != "" {
		result["report_url"] = reportURL
		result["message"] = fmt.Sprintf("Analysis report generated: %s\nReport link: %s\nThe URL is a relative path — do NOT add file:// or any host prefix. Do NOT open this URL with the browser tool. Just present the link to the user.", reportTitle, reportURL)
		// Emit result card for frontend rendering
		EmitCard(ctx, map[string]interface{}{
			"type":       "result",
			"status":     "success",
			"title":      reportTitle,
			"report_url": reportURL,
		})
	}

	b, _ := json.Marshal(result)
	return string(b), nil
}

// gatherData collects content from URLs, search queries, and direct text.
func (t *AnalyzeTool) gatherData(ctx context.Context, args map[string]interface{}, browser BrowserBackend, executor *Executor) string {
	var parts []string

	// Direct text
	if text, ok := args["text"].(string); ok && text != "" {
		if len(text) > analyzeMaxTextLen {
			text = text[:analyzeMaxTextLen]
		}
		parts = append(parts, "=== Direct Input ===\n"+text)
	}

	// URLs — scrape via browser
	if urls, ok := args["urls"].([]interface{}); ok && browser != nil {
		limit := analyzeMaxURLs
		if len(urls) < limit {
			limit = len(urls)
		}
		for i := 0; i < limit; i++ {
			url, ok := urls[i].(string)
			if !ok || url == "" {
				continue
			}
			content := t.scrapeURL(ctx, browser, url)
			if content != "" {
				parts = append(parts, fmt.Sprintf("=== URL: %s ===\n%s", url, content))
			}
		}
	}

	// Search queries — via web_search tool
	if queries, ok := args["search_queries"].([]interface{}); ok && executor != nil {
		limit := analyzeMaxSearches
		if len(queries) < limit {
			limit = len(queries)
		}
		for i := 0; i < limit; i++ {
			query, ok := queries[i].(string)
			if !ok || query == "" {
				continue
			}
			content := t.webSearch(ctx, executor, query)
			if content != "" {
				parts = append(parts, fmt.Sprintf("=== Search: %s ===\n%s", query, content))
			}
		}
	}

	return strings.Join(parts, "\n\n")
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

	// Wait for page load
	time.Sleep(2 * time.Second)

	a11y, err := browser.AccessibilityTree(ctx, nav.TargetID, 8)
	if err != nil {
		return ""
	}

	// Truncate very large trees
	tree := a11y.Tree
	if len(tree) > 15000 {
		tree = tree[:15000] + "\n... (truncated)"
	}

	return fmt.Sprintf("Title: %s\nURL: %s\n\n%s", nav.Title, nav.URL, tree)
}

// webSearch calls the web_search tool via executor.
func (t *AnalyzeTool) webSearch(ctx context.Context, executor *Executor, query string) string {
	result, err := executor.Execute(ctx, "web_search", map[string]interface{}{
		"query":       query,
		"max_results": 5,
	})
	if err != nil {
		return ""
	}

	// Result is JSON string from WebSearchTool
	resultStr, ok := result.(string)
	if !ok {
		return ""
	}

	var searchResp WebSearchResponse
	if json.Unmarshal([]byte(resultStr), &searchResp) != nil {
		return resultStr // return raw if can't parse
	}

	var b strings.Builder
	for _, r := range searchResp.Results {
		fmt.Fprintf(&b, "- %s\n  %s\n  %s\n\n", r.Title, r.URL, r.Description)
	}
	return b.String()
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

	return bridge.Chat(ctx, prompt, 8000)
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

	return bridge.Chat(ctx, prompt, 12000)
}

// saveReport writes the HTML to disk and returns the media URL.
func (t *AnalyzeTool) saveReport(html, mediaDir string) string {
	if mediaDir == "" || len(html) == 0 {
		return ""
	}
	dir := filepath.Join(mediaDir, "analyze")
	_ = os.MkdirAll(dir, 0750)
	id := uuid.New().String()
	filename := id + ".html"
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(html), 0644); err != nil {
		return ""
	}
	return "/api/v1/media/analyze/" + filename
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
func emitAnalyzeProgress(ctx context.Context, stepID, stepName, status string) {
	EmitCard(ctx, map[string]interface{}{
		"type":   "analyze-progress",
		"step":   stepID,
		"name":   stepName,
		"status": status,
	})
}
