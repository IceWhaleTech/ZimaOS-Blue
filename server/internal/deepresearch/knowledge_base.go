package deepresearch

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func normalizeReportStyle(raw string) string {
	style := strings.ToLower(strings.TrimSpace(raw))
	switch style {
	case "knowledge-base", "knowledge base", "kb":
		return "knowledge_base"
	case "summary", "timeline", "knowledge_base":
		return style
	default:
		return style
	}
}

func isKnowledgeBaseReportStyle(style string) bool {
	return normalizeReportStyle(style) == "knowledge_base"
}

func queryWantsKnowledgeBaseStyle(query string) bool {
	lower := strings.ToLower(strings.TrimSpace(query))
	if lower == "" {
		return false
	}
	return knowledgeBaseStyleCueMatcher.Contains(lower)
}

func localizedKnowledgeBaseHeading(lang, key string) string {
	if normalizeResearchLang(lang, key) == researchLangZH {
		switch key {
		case "executive_summary":
			return "## 执行摘要"
		case "scope_method":
			return "## 范围与方法"
		case "object_map":
			return "## 研究对象地图"
		case "source_inventory":
			return "## 来源清单"
		case "verification":
			return "## 核验与冲突"
		case "open_questions":
			return "## 信息缺口与待核实问题"
		case "references":
			return "## 参考来源"
		}
	}
	switch key {
	case "executive_summary":
		return "## Executive Summary"
	case "scope_method":
		return "## Scope and Method"
	case "object_map":
		return "## Object Map"
	case "source_inventory":
		return "## Source Inventory"
	case "verification":
		return "## Verification and Conflicts"
	case "open_questions":
		return "## Gaps and Open Questions"
	case "references":
		return "## References"
	default:
		return "## Section"
	}
}

func buildKnowledgeBaseAnswer(query, lang string, evidence []Evidence, citations []Citation, timelineSections []TimelineSection, openQuestions []string, verificationSummary *VerificationSummary, citationIdx map[string]int) string {
	lines := []string{localizedSummaryTitle(query, lang), "", localizedKnowledgeBaseHeading(lang, "executive_summary")}

	top := evidence
	if len(top) > 3 {
		top = top[:3]
	}
	for _, ev := range top {
		summary := firstNonEmpty(strings.TrimSpace(ev.Snippet), strings.TrimSpace(ev.Title), strings.TrimSpace(ev.URL))
		line := "- " + summary
		if refs := formatCitationRefs([]string{ev.ID}, citationIdx, lang, query); refs != "" {
			line += " " + refs
		}
		lines = append(lines, line)
	}
	if len(top) == 0 {
		lines = append(lines, "- "+localizedNoEvidence(lang, query))
	}

	domains := make(map[string]struct{})
	for _, ev := range evidence {
		if d := strings.TrimSpace(ev.Domain); d != "" {
			domains[d] = struct{}{}
		}
	}
	lines = append(lines, "", localizedKnowledgeBaseHeading(lang, "scope_method"))
	if normalizeResearchLang(lang, query) == researchLangZH {
		lines = append(lines,
			fmt.Sprintf("- 证据条数：%d", len(evidence)),
			fmt.Sprintf("- 去重来源域名：%d", len(domains)),
			"- 工作流：范围界定 → 来源收集 → 事实抽取 → 冲突核验 → 综合成稿",
		)
	} else {
		lines = append(lines,
			fmt.Sprintf("- Evidence items collected: %d", len(evidence)),
			fmt.Sprintf("- Distinct source domains: %d", len(domains)),
			"- Workflow: scope → sources → extraction → audit → synthesis",
		)
	}

	if len(timelineSections) > 0 {
		lines = append(lines, "", localizedTimelineHeading(lang, query))
		for _, section := range timelineSections {
			if len(section.Highlights) == 0 {
				continue
			}
			lines = append(lines, fmt.Sprintf("### %s", section.Label))
			for _, hl := range section.Highlights {
				lines = append(lines, "- "+hl)
			}
		}
	}

	lines = append(lines, "", localizedKnowledgeBaseHeading(lang, "source_inventory"))
	for i, ev := range evidence {
		label := firstNonEmpty(strings.TrimSpace(ev.Title), strings.TrimSpace(ev.URL))
		meta := firstNonEmpty(strings.TrimSpace(ev.Domain), strings.TrimSpace(ev.Source), "web")
		line := fmt.Sprintf("%d. %s (%s)", i+1, label, meta)
		if refs := formatCitationRefs([]string{ev.ID}, citationIdx, lang, query); refs != "" {
			line += " " + refs
		}
		lines = append(lines, line)
	}

	lines = append(lines, "", localizedKnowledgeBaseHeading(lang, "verification"))
	if verificationSummary != nil && len(verificationSummary.Items) > 0 {
		for _, item := range verificationSummary.Items {
			line := fmt.Sprintf("- %s: %s", localizedVerificationStatusLabel(lang, item.Status), strings.TrimSpace(item.Summary))
			if refs := formatCitationRefs(item.EvidenceIDs, citationIdx, lang, query); refs != "" {
				line += " " + refs
			}
			lines = append(lines, line)
		}
	} else if normalizeResearchLang(lang, query) == researchLangZH {
		lines = append(lines, "- 当前无额外冲突核验项。")
	} else {
		lines = append(lines, "- No additional verification conflicts were recorded.")
	}

	lines = append(lines, "", localizedKnowledgeBaseHeading(lang, "open_questions"))
	for _, q := range openQuestions {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		lines = append(lines, "- "+q)
		if len(lines) > 120 {
			break
		}
	}

	lines = append(lines, "", localizedKnowledgeBaseHeading(lang, "references"))
	for i, c := range citations {
		label := firstNonEmpty(strings.TrimSpace(c.Title), strings.TrimSpace(c.URL))
		lines = append(lines, fmt.Sprintf("- [%s#%d] %s — %s", localizedSourceLabel(lang, query), i+1, label, c.URL))
	}

	return strings.Join(lines, "\n")
}

func buildKnowledgeBaseObjectMap(query, lang string, tasks []Task) []map[string]interface{} {
	if len(tasks) == 0 {
		return []map[string]interface{}{{
			"id":         "research",
			"label":      localizedAxisLabel(lang, "research", "Research"),
			"task_count": 0,
			"questions":  []string{strings.TrimSpace(query)},
		}}
	}

	type group struct {
		priority    int
		label       string
		questions   []string
		timeWindows []string
		statusCount map[string]int
	}
	groups := make(map[string]*group)
	order := make([]string, 0)
	for _, task := range tasks {
		axisID := normalizeAxisID(task.Axis)
		if axisID == "" {
			axisID = "research"
		}
		g := groups[axisID]
		if g == nil {
			g = &group{
				priority:    task.Priority,
				label:       localizedAxisLabel(lang, axisID, humanizeAxisID(axisID)),
				statusCount: make(map[string]int),
			}
			groups[axisID] = g
			order = append(order, axisID)
		}
		if task.Priority > 0 && (g.priority == 0 || task.Priority < g.priority) {
			g.priority = task.Priority
		}
		if q := strings.TrimSpace(task.Question); q != "" && !containsString(g.questions, q) {
			g.questions = append(g.questions, q)
		}
		if tw := strings.TrimSpace(task.TimeWindow); tw != "" && !containsString(g.timeWindows, tw) {
			g.timeWindows = append(g.timeWindows, tw)
		}
		g.statusCount[strings.TrimSpace(task.Status)]++
	}
	sort.SliceStable(order, func(i, j int) bool {
		left := groups[order[i]]
		right := groups[order[j]]
		if left.priority == right.priority {
			return left.label < right.label
		}
		if left.priority == 0 {
			return false
		}
		if right.priority == 0 {
			return true
		}
		return left.priority < right.priority
	})
	out := make([]map[string]interface{}, 0, len(order))
	for _, axisID := range order {
		g := groups[axisID]
		entry := map[string]interface{}{
			"id":            axisID,
			"label":         g.label,
			"task_count":    len(g.questions),
			"questions":     append([]string(nil), g.questions...),
			"status_counts": g.statusCount,
		}
		if len(g.timeWindows) > 0 {
			entry["time_windows"] = append([]string(nil), g.timeWindows...)
		}
		out = append(out, entry)
	}
	return out
}

func buildKnowledgeBaseSourceInventory(evidence []Evidence) []map[string]interface{} {
	if len(evidence) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(evidence))
	out := make([]map[string]interface{}, 0, len(evidence))
	for _, ev := range evidence {
		key := firstNonEmpty(strings.TrimSpace(ev.URL), strings.TrimSpace(ev.ID))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		entry := map[string]interface{}{
			"source_id":         ev.ID,
			"title":             firstNonEmpty(strings.TrimSpace(ev.Title), strings.TrimSpace(ev.URL)),
			"url":               ev.URL,
			"source_type":       firstNonEmpty(strings.TrimSpace(ev.Source), "web"),
			"domain":            ev.Domain,
			"fetched_at":        ev.FetchedAt.Format(time.RFC3339),
			"relevance_score":   ev.RelevanceScore,
			"credibility_score": ev.CredibilityScore,
		}
		if ev.PublishedAt != nil {
			entry["published_at"] = ev.PublishedAt.Format(time.RFC3339)
		}
		if strings.TrimSpace(ev.ClaimKey) != "" {
			entry["claim_key"] = ev.ClaimKey
		}
		if strings.TrimSpace(ev.TimeLabel) != "" {
			entry["time_label"] = ev.TimeLabel
		}
		out = append(out, entry)
	}
	return out
}

func buildKnowledgeBaseCoverage(current *Job) map[string]interface{} {
	if current == nil {
		return nil
	}
	domains := make(map[string]struct{})
	for _, ev := range current.Evidence {
		if d := strings.TrimSpace(ev.Domain); d != "" {
			domains[d] = struct{}{}
		}
	}
	coverage := map[string]interface{}{
		"task_count":            len(current.Tasks),
		"evidence_count":        len(current.Evidence),
		"distinct_domain_count": len(domains),
	}
	if current.Report != nil {
		coverage["citation_count"] = len(current.Report.Citations)
		coverage["open_question_count"] = len(current.Report.OpenQuestions)
		coverage["citation_coverage"] = current.Report.CitationCoverage
		if current.Report.VerificationSummary != nil {
			coverage["resolved_count"] = current.Report.VerificationSummary.ResolvedCount
			coverage["conflicted_count"] = current.Report.VerificationSummary.ConflictedCount
			coverage["insufficient_count"] = current.Report.VerificationSummary.InsufficientCount
		}
	}
	return coverage
}

func deepResearchWorkflowPhases(status JobStatus, stage string) []map[string]interface{} {
	stage = strings.ToLower(strings.TrimSpace(stage))
	currentIndex := 0
	switch stage {
	case "planning":
		currentIndex = 0
	case "retrieve":
		currentIndex = 2
	case "verify":
		currentIndex = 3
	case "synthesize":
		currentIndex = 4
	default:
		if status == JobStatusCompleted {
			currentIndex = 5
		}
	}
	phases := []struct {
		id    string
		label string
	}{
		{"scope", "scope"},
		{"sources", "sources"},
		{"extraction", "extraction"},
		{"audit", "audit"},
		{"synthesis", "synthesis"},
	}
	out := make([]map[string]interface{}, 0, len(phases))
	for idx, phase := range phases {
		phaseStatus := "pending"
		switch {
		case status == JobStatusCompleted:
			phaseStatus = "completed"
		case idx < currentIndex:
			phaseStatus = "completed"
		case idx == currentIndex:
			phaseStatus = "current"
		}
		out = append(out, map[string]interface{}{
			"id":     phase.id,
			"label":  phase.label,
			"status": phaseStatus,
		})
	}
	return out
}

func humanizeAxisID(axisID string) string {
	axisID = strings.TrimSpace(strings.ReplaceAll(axisID, "_", " "))
	if axisID == "" {
		return "Research"
	}
	parts := strings.Fields(axisID)
	for i, p := range parts {
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
