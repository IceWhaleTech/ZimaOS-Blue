package tools

import (
	"fmt"
	"html"
	"strings"
	"time"
)

type analyzeRenderedSection struct {
	ID       string
	Title    string
	Icon     string
	IconBG   string
	Subtitle string
	Body     string
}

type analyzeReportPayload struct {
	Title                   string
	Summary                 string
	ResearchQuestion        string
	Style                   string
	Stats                   []analyzeDisplayStat
	Themes                  []analyzeDisplayTheme
	Quotes                  []analyzeDisplayQuote
	Insights                []analyzeDisplayInsight
	Recommendations         []analyzeDisplayRecommendation
	ComparisonItems         []analyzeDisplayComparisonItem
	ScenarioRecommendations []analyzeDisplayScenarioRecommendation
	References              []analyzeDisplayReference
}

type analyzeDisplayStat struct {
	Label string
	Value string
	Color string
}

type analyzeDisplayTheme struct {
	Name        string
	Count       int
	Sentiment   string
	Description string
}

type analyzeDisplayQuote struct {
	Text      string
	Author    string
	Sentiment string
}

type analyzeDisplayInsight struct {
	Type        string
	Title       string
	Description string
}

type analyzeDisplayRecommendation struct {
	Priority    string
	Title       string
	Description string
}

type analyzeDisplayMetric struct {
	Label string
	Value string
}

type analyzeDisplayComparisonItem struct {
	Name      string
	Summary   string
	BestFor   string
	Caution   string
	Metrics   []analyzeDisplayMetric
	Strengths []string
	Tradeoffs []string
}

type analyzeDisplayScenarioRecommendation struct {
	Scenario    string
	Recommended string
	Reason      string
}

type analyzeDisplayReference struct {
	Label       string
	URL         string
	Description string
	Source      string
}

func buildAnalyzeReportBody(topic string, analysisData map[string]interface{}, sourceRefs []analyzeSourceReference, lang, reportStyle string) string {
	payload := normalizeAnalyzeReportPayload(topic, analysisData, sourceRefs, lang, reportStyle)
	if payload.Style == analyzeReportStyleBriefing {
		return buildAnalyzeBriefingBody(payload, lang)
	}
	return buildAnalyzeDashboardBody(payload, lang)
}

func normalizeAnalyzeReportPayload(topic string, analysisData map[string]interface{}, sourceRefs []analyzeSourceReference, lang, reportStyle string) analyzeReportPayload {
	if analysisData == nil {
		analysisData = map[string]interface{}{}
	}

	payload := analyzeReportPayload{
		Title:            strings.TrimSpace(firstAnalyzeStringValue(analysisData["refined_title"], analysisData["title"], topic)),
		Summary:          strings.TrimSpace(firstAnalyzeStringValue(analysisData["summary"])),
		ResearchQuestion: strings.TrimSpace(firstAnalyzeStringValue(analysisData["research_question"], topic)),
		Style:            normalizeAnalyzeReportStyle(reportStyle),
	}
	if payload.Style == analyzeReportStyleAuto {
		payload.Style = analyzeReportStyleDashboard
	}
	if payload.Title == "" {
		payload.Title = analyzeLocalized(lang, "Analysis Report", "分析报告")
	}
	if payload.Summary == "" {
		payload.Summary = analyzeLocalized(lang, "Structured analysis generated from the collected sources.", "已基于收集到的来源生成结构化分析。")
	}
	if payload.ResearchQuestion == "" {
		payload.ResearchQuestion = payload.Title
	}

	payload.Stats = normalizeAnalyzeStats(analysisData["stats"])
	payload.Themes = normalizeAnalyzeThemes(analysisData["themes"])
	payload.Quotes = normalizeAnalyzeQuotes(analysisData["quotes"], lang)
	payload.Insights = normalizeAnalyzeInsights(analysisData["insights"])
	payload.Recommendations = normalizeAnalyzeRecommendations(analysisData["recommendations"])
	payload.ComparisonItems = normalizeAnalyzeComparisonItems(analysisData["comparison_items"], lang)
	payload.ScenarioRecommendations = normalizeAnalyzeScenarioRecommendations(analysisData["scenario_recommendations"], lang)
	payload.References = mergeAnalyzeReferences(normalizeAnalyzeReferences(analysisData["references"]), sourceRefs, lang)

	if len(payload.Stats) == 0 {
		payload.Stats = fallbackAnalyzeStats(payload, lang)
	}
	if len(payload.ComparisonItems) == 0 {
		payload.ComparisonItems = fallbackAnalyzeComparisonItems(payload, lang)
	}
	if len(payload.ScenarioRecommendations) == 0 {
		payload.ScenarioRecommendations = fallbackAnalyzeScenarioRecommendations(payload, lang)
	}
	if len(payload.References) == 0 {
		payload.References = []analyzeDisplayReference{{
			Label:       analyzeLocalized(lang, "Collected analysis inputs", "分析输入"),
			Description: analyzeLocalized(lang, "No explicit reference list was extracted from the collected content.", "未从收集内容中提取到显式参考列表。"),
		}}
	}

	return payload
}

func buildAnalyzeDashboardBody(payload analyzeReportPayload, lang string) string {
	sections := make([]analyzeRenderedSection, 0, 7)
	addSection := func(id, title, icon, iconBG, subtitle, body string) {
		if strings.TrimSpace(body) == "" {
			return
		}
		sections = append(sections, analyzeRenderedSection{
			ID:       id,
			Title:    title,
			Icon:     icon,
			IconBG:   iconBG,
			Subtitle: subtitle,
			Body:     body,
		})
	}

	addSection(
		"overview",
		analyzeLocalized(lang, "Overview", "概览"),
		"📊",
		"bg-blue",
		analyzeLocalized(lang, "Headline metrics and framing", "核心指标与结论框架"),
		renderDashboardOverview(payload, lang),
	)
	addSection(
		"themes",
		analyzeLocalized(lang, "Themes", "主题"),
		"📈",
		"bg-purple",
		analyzeLocalized(lang, "Recurring topics across the collected material", "来源内容中的高频主题"),
		renderDashboardThemes(payload, lang),
	)
	addSection(
		"findings",
		analyzeLocalized(lang, "Key Findings", "关键发现"),
		"💡",
		"bg-orange",
		analyzeLocalized(lang, "What stands out most", "最值得关注的洞察"),
		renderDashboardInsights(payload),
	)
	addSection(
		"voices",
		analyzeLocalized(lang, "Voices", "代表性观点"),
		"🗣️",
		"bg-green",
		analyzeLocalized(lang, "Representative excerpts and attribution", "代表性摘录与归属"),
		renderDashboardQuotes(payload, lang),
	)
	addSection(
		"recommendations",
		analyzeLocalized(lang, "Recommendations", "建议"),
		"🎯",
		"bg-red",
		analyzeLocalized(lang, "Prioritized next moves", "优先级排序后的下一步"),
		renderDashboardRecommendations(payload, lang),
	)
	addSection(
		"summary",
		analyzeLocalized(lang, "Summary", "摘要表"),
		"📋",
		"bg-blue",
		analyzeLocalized(lang, "Condensed report snapshot", "报告摘要快照"),
		renderDashboardSummary(payload, lang),
	)
	addSection(
		"references",
		analyzeLocalized(lang, "References", "参考资源"),
		"🔍",
		"bg-purple",
		analyzeLocalized(lang, "Collected sources and evidence trail", "收集到的来源与证据轨迹"),
		renderDashboardReferences(payload, lang),
	)

	var b strings.Builder
	b.WriteString(`<div class="hero">`)
	fmt.Fprintf(&b, `<div class="badge">%s</div>`, esc(analyzeLocalized(lang, "Structured Analysis", "结构化分析")))
	fmt.Fprintf(&b, `<h1>%s</h1>`, esc(payload.Title))
	fmt.Fprintf(&b, `<div class="subtitle">%s</div>`, esc(payload.Summary))
	b.WriteString(`<div class="meta-row">`)
	writeDashboardMetaItem(&b, fmt.Sprintf("%d", len(payload.References)), analyzeLocalized(lang, "Sources", "来源"))
	writeDashboardMetaItem(&b, fmt.Sprintf("%d", len(payload.Insights)), analyzeLocalized(lang, "Insights", "洞察"))
	writeDashboardMetaItem(&b, fmt.Sprintf("%d", len(payload.Recommendations)), analyzeLocalized(lang, "Actions", "建议"))
	writeDashboardMetaItem(&b, analyzeLocalized(lang, "Dashboard", "仪表板"), analyzeLocalized(lang, "Template", "模板"))
	b.WriteString(`</div></div>`)

	b.WriteString(`<div class="container">`)
	fmt.Fprintf(&b, `<div class="research-question"><strong>%s</strong> %s</div>`,
		esc(analyzeLocalized(lang, "Research question:", "研究问题：")),
		esc(payload.ResearchQuestion),
	)
	b.WriteString(`</div>`)

	if len(sections) > 0 {
		b.WriteString(`<div class="toc"><div class="toc-inner">`)
		for _, section := range sections {
			fmt.Fprintf(&b, `<a href="#%s">%s</a>`, esc(section.ID), esc(section.Title))
		}
		b.WriteString(`</div></div>`)
	}

	for _, section := range sections {
		fmt.Fprintf(&b, `<div class="section %s" id="%s"><div class="container">`, esc(section.ID), esc(section.ID))
		b.WriteString(`<div class="section-header">`)
		fmt.Fprintf(&b, `<div class="section-icon %s">%s</div>`, esc(section.IconBG), esc(section.Icon))
		b.WriteString(`<div>`)
		fmt.Fprintf(&b, `<h2>%s</h2>`, esc(section.Title))
		if strings.TrimSpace(section.Subtitle) != "" {
			fmt.Fprintf(&b, `<p>%s</p>`, esc(section.Subtitle))
		}
		b.WriteString(`</div></div>`)
		b.WriteString(section.Body)
		b.WriteString(`</div></div>`)
	}

	return b.String()
}

func buildAnalyzeBriefingBody(payload analyzeReportPayload, lang string) string {
	var b strings.Builder
	dateText := time.Now().Format("2006-01-02")

	b.WriteString(`<div class="briefing-root">`)
	b.WriteString(`<div class="briefing-header"><div class="briefing-header-content">`)
	fmt.Fprintf(&b, `<h1>%s</h1>`, esc(payload.Title))
	fmt.Fprintf(&b, `<div class="subtitle">%s</div>`, esc(payload.Summary))
	fmt.Fprintf(&b, `<div class="research-question"><strong>%s</strong> %s</div>`,
		esc(analyzeLocalized(lang, "Research question:", "研究问题：")),
		esc(payload.ResearchQuestion),
	)
	b.WriteString(`<div class="briefing-meta">`)
	fmt.Fprintf(&b, `<div class="meta-item"><span>%s</span><strong>%s</strong></div>`,
		esc(analyzeLocalized(lang, "Generated:", "生成日期：")),
		esc(dateText),
	)
	fmt.Fprintf(&b, `<div class="meta-item"><span class="briefing-badge">%s</span></div>`,
		esc(analyzeLocalized(lang, "Blue Analyze Briefing", "Blue Analyze 简报")),
	)
	b.WriteString(`</div></div></div>`)

	b.WriteString(`<div class="briefing-container">`)
	sectionNo := 1
	writeBriefingSection(&b, &sectionNo, analyzeLocalized(lang, "Core Conclusions and Recommendation", "核心结论与推荐"), renderBriefingConclusions(payload, lang))
	writeBriefingSection(&b, &sectionNo, analyzeLocalized(lang, "Evidence and Comparison", "证据与对比"), renderBriefingComparison(payload, lang))
	writeBriefingSection(&b, &sectionNo, analyzeLocalized(lang, "Scenario Guidance", "场景建议"), renderBriefingScenarios(payload, lang))
	writeBriefingSection(&b, &sectionNo, analyzeLocalized(lang, "Supporting Observations", "补充观察"), renderBriefingObservations(payload, lang))
	writeBriefingSection(&b, &sectionNo, analyzeLocalized(lang, "References", "参考资源"), renderBriefingReferences(payload, lang))
	b.WriteString(`</div></div>`)

	return b.String()
}

func renderDashboardOverview(payload analyzeReportPayload, lang string) string {
	var b strings.Builder
	if len(payload.Stats) > 0 {
		b.WriteString(`<div class="stat-grid">`)
		for i, stat := range payload.Stats {
			colorClass := normalizeAnalyzeColor(stat.Color, i)
			classAttr := "stat-box"
			if colorClass != "" {
				classAttr += " " + colorClass
			}
			fmt.Fprintf(&b, `<div class="%s"><span class="number">%s</span><div class="label">%s</div></div>`,
				esc(classAttr),
				esc(stat.Value),
				esc(stat.Label),
			)
		}
		b.WriteString(`</div>`)
	}
	fmt.Fprintf(&b, `<div class="highlight-strip">%s</div>`, esc(payload.Summary))
	return b.String()
}

func renderDashboardThemes(payload analyzeReportPayload, lang string) string {
	if len(payload.Themes) == 0 {
		return ""
	}
	maxCount := 1
	for _, theme := range payload.Themes {
		if theme.Count > maxCount {
			maxCount = theme.Count
		}
	}

	var b strings.Builder
	b.WriteString(`<div class="grid-2">`)
	fmt.Fprintf(&b, `<div class="card"><h3>%s</h3><div class="tag-cloud">`, esc(analyzeLocalized(lang, "Themes", "主题")))
	for i, theme := range payload.Themes {
		fmt.Fprintf(&b, `<div class="tag %s">%s <span class="count">%d</span></div>`,
			esc(tagColorClass(i)),
			esc(theme.Name),
			theme.Count,
		)
	}
	b.WriteString(`</div></div>`)

	fmt.Fprintf(&b, `<div class="card"><h3>%s</h3>`, esc(analyzeLocalized(lang, "Relative Weight", "相对权重")))
	for i, theme := range payload.Themes {
		width := 100
		if maxCount > 0 && theme.Count > 0 {
			width = theme.Count * 100 / maxCount
		}
		if width < 12 {
			width = 12
		}
		fmt.Fprintf(&b, `<div class="progress-item"><div class="progress-label"><span>%s</span><span>%d</span></div><div class="progress-bar"><div class="progress-fill %s" style="width:%d%%"></div></div></div>`,
			esc(theme.Name),
			theme.Count,
			esc(progressColorClass(i)),
			width,
		)
	}
	b.WriteString(`</div></div>`)
	return b.String()
}

func renderDashboardInsights(payload analyzeReportPayload) string {
	if len(payload.Insights) == 0 {
		return ""
	}
	var b strings.Builder
	for _, insight := range payload.Insights {
		className := insightColorClass(insight.Type)
		if className == "" {
			className = "insight-box"
		}
		fmt.Fprintf(&b, `<div class="%s"><div class="insight-title">%s</div><p>%s</p></div>`,
			esc(className),
			esc(insight.Title),
			esc(insight.Description),
		)
	}
	return b.String()
}

func renderDashboardQuotes(payload analyzeReportPayload, lang string) string {
	if len(payload.Quotes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div class="quote-grid">`)
	for _, quote := range payload.Quotes {
		className := quoteColorClass(quote.Sentiment)
		tagText := quoteTagText(quote.Sentiment, lang)
		fmt.Fprintf(&b, `<div class="%s"><div class="quote-tag">%s</div><div class="quote-text">%s</div><div class="quote-author">%s</div></div>`,
			esc(className),
			esc(tagText),
			esc(quote.Text),
			esc(quote.Author),
		)
	}
	b.WriteString(`</div>`)
	return b.String()
}

func renderDashboardRecommendations(payload analyzeReportPayload, lang string) string {
	items := payload.Recommendations
	if len(items) == 0 && len(payload.ScenarioRecommendations) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div class="grid-2">`)
	for _, rec := range items {
		b.WriteString(`<div class="card">`)
		fmt.Fprintf(&b, `<h3>%s</h3>`, esc(rec.Title))
		if strings.TrimSpace(rec.Priority) != "" {
			fmt.Fprintf(&b, `<p><strong>%s</strong> %s</p>`,
				esc(analyzeLocalized(lang, "Priority:", "优先级：")),
				esc(priorityLabel(rec.Priority, lang)),
			)
		}
		fmt.Fprintf(&b, `<p>%s</p></div>`, esc(rec.Description))
	}
	for _, item := range payload.ScenarioRecommendations {
		b.WriteString(`<div class="card">`)
		fmt.Fprintf(&b, `<h3>%s</h3>`, esc(item.Scenario))
		fmt.Fprintf(&b, `<p><strong>%s</strong> %s</p>`,
			esc(analyzeLocalized(lang, "Recommended:", "建议：")),
			esc(item.Recommended),
		)
		fmt.Fprintf(&b, `<p>%s</p></div>`, esc(item.Reason))
	}
	b.WriteString(`</div>`)
	return b.String()
}

func renderDashboardSummary(payload analyzeReportPayload, lang string) string {
	rows := []analyzeDisplayMetric{
		{Label: analyzeLocalized(lang, "Report title", "报告标题"), Value: payload.Title},
		{Label: analyzeLocalized(lang, "Research question", "研究问题"), Value: payload.ResearchQuestion},
		{Label: analyzeLocalized(lang, "Themes", "主题数"), Value: fmt.Sprintf("%d", len(payload.Themes))},
		{Label: analyzeLocalized(lang, "Insights", "洞察数"), Value: fmt.Sprintf("%d", len(payload.Insights))},
		{Label: analyzeLocalized(lang, "Recommendations", "建议数"), Value: fmt.Sprintf("%d", len(payload.Recommendations))},
		{Label: analyzeLocalized(lang, "References", "参考数"), Value: fmt.Sprintf("%d", len(payload.References))},
	}

	var b strings.Builder
	b.WriteString(`<div class="card"><table class="data-table"><thead><tr>`)
	fmt.Fprintf(&b, `<th>%s</th><th>%s</th>`,
		esc(analyzeLocalized(lang, "Metric", "指标")),
		esc(analyzeLocalized(lang, "Value", "数值")),
	)
	b.WriteString(`</tr></thead><tbody>`)
	for _, row := range rows {
		fmt.Fprintf(&b, `<tr><td>%s</td><td>%s</td></tr>`, esc(row.Label), esc(row.Value))
	}
	b.WriteString(`</tbody></table></div>`)
	return b.String()
}

func renderDashboardReferences(payload analyzeReportPayload, lang string) string {
	var b strings.Builder
	b.WriteString(`<div class="card references"><table class="data-table"><thead><tr>`)
	fmt.Fprintf(&b, `<th>%s</th><th>%s</th><th>%s</th></tr>`,
		esc(analyzeLocalized(lang, "Source", "来源")),
		esc(analyzeLocalized(lang, "Location", "链接/位置")),
		esc(analyzeLocalized(lang, "Notes", "说明")),
	)
	b.WriteString(`</tr></thead><tbody>`)
	for _, ref := range payload.References {
		b.WriteString(`<tr>`)
		fmt.Fprintf(&b, `<td>%s</td>`, esc(ref.Label))
		if strings.TrimSpace(ref.URL) != "" {
			fmt.Fprintf(&b, `<td><a href="%s" target="_blank" rel="noreferrer">%s</a></td>`,
				esc(ref.URL),
				esc(ref.URL),
			)
		} else {
			fmt.Fprintf(&b, `<td>%s</td>`, esc(analyzeLocalized(lang, "Local / inline input", "本地/直接输入")))
		}
		note := strings.TrimSpace(strings.Join(filterNonEmpty([]string{ref.Description, ref.Source}), " · "))
		fmt.Fprintf(&b, `<td>%s</td>`, esc(note))
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table></div>`)
	return b.String()
}

func renderBriefingConclusions(payload analyzeReportPayload, lang string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<div class="callout primary"><strong>%s</strong><br>%s</div>`,
		esc(analyzeLocalized(lang, "Executive summary", "执行摘要")),
		esc(payload.Summary),
	)

	callouts := payload.Recommendations
	if len(callouts) > 3 {
		callouts = callouts[:3]
	}
	if len(callouts) == 0 && len(payload.Insights) > 0 {
		for _, insight := range payload.Insights {
			callouts = append(callouts, analyzeDisplayRecommendation{
				Priority:    insight.Type,
				Title:       insight.Title,
				Description: insight.Description,
			})
			if len(callouts) >= 3 {
				break
			}
		}
	}

	for _, item := range callouts {
		className := briefingCalloutClass(item.Priority)
		fmt.Fprintf(&b, `<div class="%s"><strong>%s</strong><br>%s</div>`,
			esc(className),
			esc(item.Title),
			esc(item.Description),
		)
	}
	return b.String()
}

func renderBriefingComparison(payload analyzeReportPayload, lang string) string {
	var b strings.Builder
	for _, item := range payload.ComparisonItems {
		b.WriteString(`<div class="subsection">`)
		fmt.Fprintf(&b, `<h3>%s</h3>`, esc(item.Name))
		if strings.TrimSpace(item.Summary) != "" {
			fmt.Fprintf(&b, `<p>%s</p>`, esc(item.Summary))
		}
		if strings.TrimSpace(item.BestFor) != "" {
			fmt.Fprintf(&b, `<h4>%s</h4><p>%s</p>`,
				esc(analyzeLocalized(lang, "Best for", "适合场景")),
				esc(item.BestFor),
			)
		}
		if len(item.Metrics) > 0 {
			b.WriteString(`<table><thead><tr>`)
			fmt.Fprintf(&b, `<th>%s</th><th>%s</th>`,
				esc(analyzeLocalized(lang, "Metric", "指标")),
				esc(analyzeLocalized(lang, "Value", "数值")),
			)
			b.WriteString(`</tr></thead><tbody>`)
			for _, metric := range item.Metrics {
				fmt.Fprintf(&b, `<tr><td>%s</td><td>%s</td></tr>`, esc(metric.Label), esc(metric.Value))
			}
			b.WriteString(`</tbody></table>`)
		}
		if len(item.Strengths) > 0 {
			fmt.Fprintf(&b, `<h4>%s</h4>`, esc(analyzeLocalized(lang, "Strengths", "优点")))
			b.WriteString(`<ul>`)
			for _, text := range item.Strengths {
				fmt.Fprintf(&b, `<li>%s</li>`, esc(text))
			}
			b.WriteString(`</ul>`)
		}
		if len(item.Tradeoffs) > 0 || strings.TrimSpace(item.Caution) != "" {
			fmt.Fprintf(&b, `<h4>%s</h4>`, esc(analyzeLocalized(lang, "Tradeoffs", "权衡点")))
			b.WriteString(`<ul>`)
			for _, text := range item.Tradeoffs {
				fmt.Fprintf(&b, `<li>%s</li>`, esc(text))
			}
			if strings.TrimSpace(item.Caution) != "" {
				fmt.Fprintf(&b, `<li>%s</li>`, esc(item.Caution))
			}
			b.WriteString(`</ul>`)
		}
		b.WriteString(`</div>`)
	}
	return b.String()
}

func renderBriefingScenarios(payload analyzeReportPayload, lang string) string {
	if len(payload.ScenarioRecommendations) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div class="comparison-grid">`)
	for _, item := range payload.ScenarioRecommendations {
		b.WriteString(`<div class="comparison-card">`)
		fmt.Fprintf(&b, `<h4>%s</h4>`, esc(item.Scenario))
		fmt.Fprintf(&b, `<p><strong>%s</strong> %s</p>`,
			esc(analyzeLocalized(lang, "Recommended:", "建议：")),
			esc(item.Recommended),
		)
		if strings.TrimSpace(item.Reason) != "" {
			fmt.Fprintf(&b, `<p>%s</p>`, esc(item.Reason))
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

func renderBriefingObservations(payload analyzeReportPayload, lang string) string {
	if len(payload.Insights) == 0 && len(payload.Quotes) == 0 {
		return ""
	}
	var b strings.Builder
	if len(payload.Insights) > 0 {
		b.WriteString(`<div class="subsection">`)
		fmt.Fprintf(&b, `<h3>%s</h3>`, esc(analyzeLocalized(lang, "Key observations", "关键观察")))
		b.WriteString(`<ul>`)
		for _, insight := range payload.Insights {
			text := strings.TrimSpace(strings.Join(filterNonEmpty([]string{insight.Title, insight.Description}), ": "))
			fmt.Fprintf(&b, `<li>%s</li>`, esc(text))
		}
		b.WriteString(`</ul></div>`)
	}
	if len(payload.Quotes) > 0 {
		b.WriteString(`<div class="subsection">`)
		fmt.Fprintf(&b, `<h3>%s</h3>`, esc(analyzeLocalized(lang, "Representative excerpts", "代表性摘录")))
		b.WriteString(`<ul>`)
		for _, quote := range payload.Quotes {
			fmt.Fprintf(&b, `<li>%s (%s)</li>`, esc(quote.Text), esc(quote.Author))
		}
		b.WriteString(`</ul></div>`)
	}
	return b.String()
}

func renderBriefingReferences(payload analyzeReportPayload, lang string) string {
	var b strings.Builder
	b.WriteString(`<div class="subsection references"><ol class="references-list">`)
	for _, ref := range payload.References {
		b.WriteString(`<li>`)
		fmt.Fprintf(&b, `<div class="ref-label">%s</div>`, esc(ref.Label))
		if strings.TrimSpace(ref.URL) != "" {
			fmt.Fprintf(&b, `<div><a href="%s" target="_blank" rel="noreferrer">%s</a></div>`,
				esc(ref.URL),
				esc(ref.URL),
			)
		} else {
			fmt.Fprintf(&b, `<div>%s</div>`, esc(analyzeLocalized(lang, "Local / inline input", "本地/直接输入")))
		}
		meta := strings.TrimSpace(strings.Join(filterNonEmpty([]string{ref.Description, ref.Source}), " · "))
		if meta != "" {
			fmt.Fprintf(&b, `<div class="ref-meta">%s</div>`, esc(meta))
		}
		b.WriteString(`</li>`)
	}
	b.WriteString(`</ol></div>`)
	return b.String()
}

func writeDashboardMetaItem(b *strings.Builder, value, label string) {
	b.WriteString(`<div class="meta-item">`)
	fmt.Fprintf(b, `<span class="num">%s</span><span class="label">%s</span>`, esc(value), esc(label))
	b.WriteString(`</div>`)
}

func writeBriefingSection(b *strings.Builder, sectionNo *int, title, body string) {
	if strings.TrimSpace(body) == "" {
		return
	}
	fmt.Fprintf(b, `<div class="briefing-section"><div class="briefing-section-header"><span class="section-number">%d</span><h2>%s</h2></div>%s</div>`,
		*sectionNo,
		esc(title),
		body,
	)
	*sectionNo = *sectionNo + 1
}

func normalizeAnalyzeStats(raw interface{}) []analyzeDisplayStat {
	items := analyzeObjectSlice(raw)
	out := make([]analyzeDisplayStat, 0, len(items))
	for idx, item := range items {
		label := strings.TrimSpace(firstAnalyzeStringValue(item["label"], item["name"], item["title"]))
		value := strings.TrimSpace(firstAnalyzeStringValue(item["value"], item["summary"], item["description"]))
		if label == "" || value == "" {
			continue
		}
		out = append(out, analyzeDisplayStat{
			Label: label,
			Value: value,
			Color: normalizeAnalyzeColor(firstAnalyzeStringValue(item["color"]), idx),
		})
	}
	return out
}

func normalizeAnalyzeThemes(raw interface{}) []analyzeDisplayTheme {
	items := analyzeObjectSlice(raw)
	out := make([]analyzeDisplayTheme, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(firstAnalyzeStringValue(item["name"], item["label"], item["title"]))
		if name == "" {
			continue
		}
		out = append(out, analyzeDisplayTheme{
			Name:        name,
			Count:       analyzeIntValue(item["count"]),
			Sentiment:   strings.TrimSpace(firstAnalyzeStringValue(item["sentiment"])),
			Description: strings.TrimSpace(firstAnalyzeStringValue(item["description"], item["summary"])),
		})
	}
	return out
}

func normalizeAnalyzeQuotes(raw interface{}, lang string) []analyzeDisplayQuote {
	items := analyzeObjectSlice(raw)
	out := make([]analyzeDisplayQuote, 0, len(items))
	for _, item := range items {
		text := strings.TrimSpace(firstAnalyzeStringValue(item["text"], item["summary"], item["description"]))
		if text == "" {
			continue
		}
		author := strings.TrimSpace(firstAnalyzeStringValue(item["author"], item["source"], item["label"]))
		if author == "" {
			author = analyzeLocalized(lang, "Collected source", "收集来源")
		}
		out = append(out, analyzeDisplayQuote{
			Text:      text,
			Author:    author,
			Sentiment: strings.TrimSpace(firstAnalyzeStringValue(item["sentiment"])),
		})
	}
	return out
}

func normalizeAnalyzeInsights(raw interface{}) []analyzeDisplayInsight {
	items := analyzeObjectSlice(raw)
	out := make([]analyzeDisplayInsight, 0, len(items))
	for _, item := range items {
		title := strings.TrimSpace(firstAnalyzeStringValue(item["title"], item["label"], item["name"]))
		description := strings.TrimSpace(firstAnalyzeStringValue(item["description"], item["summary"], item["text"]))
		if title == "" && description == "" {
			continue
		}
		if title == "" {
			title = description
		}
		out = append(out, analyzeDisplayInsight{
			Type:        strings.TrimSpace(firstAnalyzeStringValue(item["type"])),
			Title:       title,
			Description: description,
		})
	}
	return out
}

func normalizeAnalyzeRecommendations(raw interface{}) []analyzeDisplayRecommendation {
	items := analyzeObjectSlice(raw)
	out := make([]analyzeDisplayRecommendation, 0, len(items))
	for _, item := range items {
		title := strings.TrimSpace(firstAnalyzeStringValue(item["title"], item["label"], item["name"]))
		description := strings.TrimSpace(firstAnalyzeStringValue(item["description"], item["summary"], item["text"]))
		if title == "" && description == "" {
			continue
		}
		if title == "" {
			title = description
		}
		out = append(out, analyzeDisplayRecommendation{
			Priority:    strings.TrimSpace(firstAnalyzeStringValue(item["priority"], item["type"])),
			Title:       title,
			Description: description,
		})
	}
	return out
}

func normalizeAnalyzeComparisonItems(raw interface{}, lang string) []analyzeDisplayComparisonItem {
	items := analyzeObjectSlice(raw)
	out := make([]analyzeDisplayComparisonItem, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(firstAnalyzeStringValue(item["name"], item["title"], item["label"]))
		summary := strings.TrimSpace(firstAnalyzeStringValue(item["summary"], item["description"]))
		if name == "" && summary == "" {
			continue
		}
		if name == "" {
			name = analyzeLocalized(lang, "Comparison item", "对比项")
		}
		metricsRaw := analyzeObjectSlice(item["metrics"])
		metrics := make([]analyzeDisplayMetric, 0, len(metricsRaw))
		for _, metric := range metricsRaw {
			label := strings.TrimSpace(firstAnalyzeStringValue(metric["label"], metric["name"], metric["title"]))
			value := strings.TrimSpace(firstAnalyzeStringValue(metric["value"], metric["summary"], metric["description"]))
			if label == "" || value == "" {
				continue
			}
			metrics = append(metrics, analyzeDisplayMetric{Label: label, Value: value})
		}
		out = append(out, analyzeDisplayComparisonItem{
			Name:      name,
			Summary:   summary,
			BestFor:   strings.TrimSpace(firstAnalyzeStringValue(item["best_for"], item["recommended_for"], item["scenario"])),
			Caution:   strings.TrimSpace(firstAnalyzeStringValue(item["caution"], item["risk"])),
			Metrics:   metrics,
			Strengths: analyzeStringSlice(item["strengths"]),
			Tradeoffs: analyzeStringSlice(item["tradeoffs"]),
		})
	}
	return out
}

func normalizeAnalyzeScenarioRecommendations(raw interface{}, lang string) []analyzeDisplayScenarioRecommendation {
	items := analyzeObjectSlice(raw)
	out := make([]analyzeDisplayScenarioRecommendation, 0, len(items))
	for _, item := range items {
		scenario := strings.TrimSpace(firstAnalyzeStringValue(item["scenario"], item["title"], item["label"]))
		recommended := strings.TrimSpace(firstAnalyzeStringValue(item["recommended"], item["title"], item["name"]))
		reason := strings.TrimSpace(firstAnalyzeStringValue(item["reason"], item["description"], item["summary"]))
		if scenario == "" && recommended == "" && reason == "" {
			continue
		}
		if scenario == "" {
			scenario = analyzeLocalized(lang, "Suggested scenario", "建议场景")
		}
		out = append(out, analyzeDisplayScenarioRecommendation{
			Scenario:    scenario,
			Recommended: recommended,
			Reason:      reason,
		})
	}
	return out
}

func normalizeAnalyzeReferences(raw interface{}) []analyzeDisplayReference {
	items := analyzeObjectSlice(raw)
	out := make([]analyzeDisplayReference, 0, len(items))
	for _, item := range items {
		label := strings.TrimSpace(firstAnalyzeStringValue(item["label"], item["title"], item["name"], item["url"]))
		url := strings.TrimSpace(firstAnalyzeStringValue(item["url"], item["href"]))
		description := strings.TrimSpace(firstAnalyzeStringValue(item["description"], item["summary"]))
		source := strings.TrimSpace(firstAnalyzeStringValue(item["source"], item["kind"], item["type"]))
		if label == "" && url == "" && description == "" {
			continue
		}
		out = append(out, analyzeDisplayReference{
			Label:       label,
			URL:         url,
			Description: description,
			Source:      source,
		})
	}
	return out
}

func fallbackAnalyzeStats(payload analyzeReportPayload, lang string) []analyzeDisplayStat {
	return []analyzeDisplayStat{
		{Label: analyzeLocalized(lang, "Sources", "来源"), Value: fmt.Sprintf("%d", len(payload.References)), Color: "blue"},
		{Label: analyzeLocalized(lang, "Themes", "主题"), Value: fmt.Sprintf("%d", len(payload.Themes)), Color: "green"},
		{Label: analyzeLocalized(lang, "Insights", "洞察"), Value: fmt.Sprintf("%d", len(payload.Insights)), Color: "orange"},
		{Label: analyzeLocalized(lang, "Recommendations", "建议"), Value: fmt.Sprintf("%d", len(payload.Recommendations)), Color: "red"},
	}
}

func fallbackAnalyzeComparisonItems(payload analyzeReportPayload, lang string) []analyzeDisplayComparisonItem {
	if len(payload.Stats) == 0 {
		return nil
	}
	metrics := make([]analyzeDisplayMetric, 0, len(payload.Stats))
	for _, stat := range payload.Stats {
		metrics = append(metrics, analyzeDisplayMetric{Label: stat.Label, Value: stat.Value})
	}
	return []analyzeDisplayComparisonItem{{
		Name:    analyzeLocalized(lang, "Key evidence snapshot", "关键证据快照"),
		Summary: payload.Summary,
		Metrics: metrics,
	}}
}

func fallbackAnalyzeScenarioRecommendations(payload analyzeReportPayload, lang string) []analyzeDisplayScenarioRecommendation {
	if len(payload.Recommendations) == 0 {
		return nil
	}
	out := make([]analyzeDisplayScenarioRecommendation, 0, len(payload.Recommendations))
	for _, rec := range payload.Recommendations {
		out = append(out, analyzeDisplayScenarioRecommendation{
			Scenario:    priorityLabel(rec.Priority, lang),
			Recommended: rec.Title,
			Reason:      rec.Description,
		})
	}
	return out
}

func mergeAnalyzeReferences(existing []analyzeDisplayReference, sourceRefs []analyzeSourceReference, lang string) []analyzeDisplayReference {
	out := make([]analyzeDisplayReference, 0, len(existing)+len(sourceRefs))
	seen := make(map[string]struct{}, len(existing)+len(sourceRefs))
	appendRef := func(ref analyzeDisplayReference) {
		key := strings.ToLower(strings.TrimSpace(ref.URL))
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(ref.Label + "|" + ref.Description + "|" + ref.Source))
		}
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, ref)
	}

	for _, ref := range existing {
		appendRef(ref)
	}
	for _, ref := range sourceRefs {
		label := strings.TrimSpace(ref.Label)
		if label == "" {
			label = strings.TrimSpace(ref.URL)
		}
		if label == "" {
			label = analyzeLocalized(lang, "Collected source", "收集来源")
		}
		appendRef(analyzeDisplayReference{
			Label:       label,
			URL:         strings.TrimSpace(ref.URL),
			Description: strings.TrimSpace(ref.Description),
			Source:      strings.TrimSpace(firstAnalyzeStringValue(ref.Source, ref.Kind)),
		})
	}

	return out
}

func analyzeObjectSlice(raw interface{}) []map[string]interface{} {
	switch typed := raw.(type) {
	case []map[string]interface{}:
		return typed
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(typed))
		for _, item := range typed {
			if m, ok := item.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

func analyzeStringSlice(raw interface{}) []string {
	switch typed := raw.(type) {
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(firstAnalyzeStringValue(item)); text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		if text := strings.TrimSpace(firstAnalyzeStringValue(raw)); text != "" {
			return []string{text}
		}
		return nil
	}
}

func analyzeIntValue(raw interface{}) int {
	switch typed := raw.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func normalizeAnalyzeColor(raw string, idx int) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "blue":
		return ""
	case "":
		return []string{"", "green", "orange", "red"}[idx%4]
	case "green":
		return "green"
	case "orange", "amber", "yellow":
		return "orange"
	case "red":
		return "red"
	default:
		return []string{"", "green", "orange", "red"}[idx%4]
	}
}

func insightColorClass(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "opportunity":
		return "insight-box green"
	case "challenge":
		return "insight-box orange"
	case "risk":
		return "insight-box red"
	default:
		return "insight-box"
	}
}

func quoteColorClass(sentiment string) string {
	switch strings.ToLower(strings.TrimSpace(sentiment)) {
	case "positive":
		return "quote-card positive"
	case "negative":
		return "quote-card negative"
	case "mixed":
		return "quote-card orange"
	default:
		return "quote-card neutral"
	}
}

func quoteTagText(sentiment, lang string) string {
	switch strings.ToLower(strings.TrimSpace(sentiment)) {
	case "positive":
		return analyzeLocalized(lang, "Positive", "正向")
	case "negative":
		return analyzeLocalized(lang, "Negative", "负向")
	case "mixed":
		return analyzeLocalized(lang, "Mixed", "混合")
	default:
		return analyzeLocalized(lang, "Neutral", "中性")
	}
}

func tagColorClass(idx int) string {
	classes := []string{"tag-blue", "tag-green", "tag-orange", "tag-purple", "tag-red"}
	return classes[idx%len(classes)]
}

func progressColorClass(idx int) string {
	classes := []string{"fill-blue", "fill-green", "fill-orange", "fill-purple", "fill-red"}
	return classes[idx%len(classes)]
}

func briefingCalloutClass(priority string) string {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "high", "risk":
		return "callout warning"
	case "low", "opportunity":
		return "callout success"
	case "challenge", "medium":
		return "callout primary"
	default:
		return "callout primary"
	}
}

func priorityLabel(priority, lang string) string {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "high":
		return analyzeLocalized(lang, "High priority", "高优先级")
	case "medium":
		return analyzeLocalized(lang, "Medium priority", "中优先级")
	case "low":
		return analyzeLocalized(lang, "Low priority", "低优先级")
	case "opportunity":
		return analyzeLocalized(lang, "Opportunity", "机会")
	case "challenge":
		return analyzeLocalized(lang, "Challenge", "挑战")
	case "risk":
		return analyzeLocalized(lang, "Risk", "风险")
	default:
		return analyzeLocalized(lang, "Suggested next step", "建议下一步")
	}
}

func filterNonEmpty(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func esc(text string) string {
	return html.EscapeString(strings.TrimSpace(text))
}
