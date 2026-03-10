package deepresearch

import (
	"fmt"
	"sort"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const deepResearchActiveEvidenceLimit = 6

const (
	verificationStatusResolved     = "resolved"
	verificationStatusConflicted   = "conflicted"
	verificationStatusInsufficient = "insufficient"
)

type researchBrief struct {
	Query            string
	Entity           string
	Goal             string
	TimeWindows      []string
	PreferPrimary    bool
	MustVerifyClaims []string
	Axes             []researchAxis
	WantsLatest      bool
	PersonTimeline   bool
}

type researchAxis struct {
	ID         string
	Label      string
	Priority   int
	TimeWindow string
	QueryHint  string
}

type researchGap struct {
	ID          string
	Focus       string
	Gap         string
	Status      string
	Priority    int
	Axis        string
	QueryHints  []string
	EvidenceIDs []string
}

func buildResearchBrief(query, lang string, timeWindows []string, reportStyle string) researchBrief {
	q := strings.TrimSpace(query)
	brief := researchBrief{
		Query:         q,
		TimeWindows:   normalizeTimeWindows(timeWindows),
		PreferPrimary: true,
		WantsLatest:   queryWantsLatest(q),
	}

	if looksLikePersonTimelineResearch(q, lang) || strings.EqualFold(strings.TrimSpace(reportStyle), "timeline") {
		entity := extractPrimaryEntityName(q)
		if entity == "" {
			entity = q
		}
		brief.Entity = entity
		brief.Goal = "timeline_analysis"
		brief.PersonTimeline = true
		if len(brief.TimeWindows) == 0 {
			brief.TimeWindows = localizedTimelineDefaultLabels(lang)
		}
		labels := brief.TimeWindows
		if len(labels) == 0 {
			labels = []string{"earlier", "middle", "recent"}
		}
		brief.Axes = []researchAxis{{
			ID:        "identity",
			Label:     localizedAxisLabel(lang, "identity_validation", "Identity"),
			Priority:  1,
			QueryHint: personIdentityQuery(entity, lang),
		}}
		for idx, label := range labels {
			brief.Axes = append(brief.Axes, researchAxis{
				ID:         timelineAxisID(idx),
				Label:      label,
				Priority:   idx + 2,
				TimeWindow: label,
				QueryHint:  personPeriodQuery(entity, label, lang),
			})
			if idx >= 2 {
				break
			}
		}
		brief.Axes = append(brief.Axes, researchAxis{
			ID:        "footprint",
			Label:     localizedAxisLabel(lang, "internet_footprint", "Internet footprint"),
			Priority:  5,
			QueryHint: personFootprintQuery(entity, lang),
		})
		brief.MustVerifyClaims = dedupeStrings([]string{
			fmt.Sprintf("%s identity/background", entity),
			fmt.Sprintf("%s viewpoint evolution", entity),
		})
		return brief
	}

	brief.Goal = "topic_summary"
	brief.Axes = []researchAxis{
		{ID: "overview", Label: localizedAxisLabel(lang, "overview", "Overview"), Priority: 1, QueryHint: q},
		{ID: "latest", Label: localizedAxisLabel(lang, "latest", "Latest"), Priority: 2, QueryHint: buildLatestQuery(q, lang)},
		{ID: "official", Label: localizedAxisLabel(lang, "official", "Official sources"), Priority: 2, QueryHint: buildOfficialQuery(q, lang)},
		{ID: "comparison", Label: localizedAxisLabel(lang, "comparison", "Comparison"), Priority: 3, QueryHint: buildComparisonQuery(q, lang)},
		{ID: "best_practices", Label: localizedAxisLabel(lang, "best_practices", "Best practices"), Priority: 4, QueryHint: buildBestPracticeQuery(q, lang)},
	}
	if looksLikeClaimValidation(q) {
		brief.MustVerifyClaims = []string{q}
	}
	return brief
}

func annotateTasksWithBrief(tasks []Task, brief researchBrief) []Task {
	if len(tasks) == 0 {
		return nil
	}
	out := make([]Task, 0, len(tasks))
	for idx, task := range tasks {
		cp := task
		if cp.Axis == "" {
			cp.Axis = inferTaskAxis(cp, brief, idx)
		}
		if cp.TimeWindow == "" {
			if axis := briefAxisByID(brief, cp.Axis); axis != nil {
				cp.TimeWindow = axis.TimeWindow
			}
		}
		out = append(out, cp)
	}
	return out
}

func inferTaskAxis(task Task, brief researchBrief, idx int) string {
	if task.Axis != "" {
		return task.Axis
	}
	switch {
	case task.Category == "identity_validation" || strings.Contains(strings.ToLower(task.ID), "identity"):
		return "identity"
	case task.Category == "internet_footprint" || strings.Contains(strings.ToLower(task.ID), "footprint"):
		return "footprint"
	case task.TimeWindow != "":
		if strings.Contains(strings.ToLower(task.ID), "early") {
			return timelineAxisID(0)
		}
		if strings.Contains(strings.ToLower(task.ID), "mid") {
			return timelineAxisID(1)
		}
		if strings.Contains(strings.ToLower(task.ID), "recent") {
			return timelineAxisID(2)
		}
		return normalizeAxisID(task.TimeWindow)
	case task.Category != "":
		return normalizeAxisID(task.Category)
	default:
		_ = idx
		return "research"
	}
}

func briefAxisByID(brief researchBrief, axisID string) *researchAxis {
	axisID = normalizeAxisID(axisID)
	for i := range brief.Axes {
		if brief.Axes[i].ID == axisID {
			return &brief.Axes[i]
		}
	}
	return nil
}

func maxFollowUpRounds(mode Mode) int {
	switch mode {
	case ModeDeep:
		return 3
	case ModeStandard:
		return 1
	default:
		return 0
	}
}

func buildVerificationSummary(query, lang string, brief researchBrief, evidence []Evidence, tasks []Task) (*VerificationSummary, []researchGap) {
	_ = query
	summary := &VerificationSummary{}
	if len(tasks) == 0 {
		return summary, nil
	}
	itemByKey := make(map[string]int)
	addItem := func(item VerificationItem) {
		key := strings.ToLower(strings.TrimSpace(item.Focus + "|" + item.Gap + "|" + item.Status))
		if key == "||" {
			return
		}
		if idx, ok := itemByKey[key]; ok {
			cur := summary.Items[idx]
			cur.EvidenceIDs = dedupeStrings(append(cur.EvidenceIDs, item.EvidenceIDs...))
			if cur.Summary == "" {
				cur.Summary = item.Summary
			}
			summary.Items[idx] = cur
			return
		}
		item.EvidenceIDs = dedupeStrings(item.EvidenceIDs)
		summary.Items = append(summary.Items, item)
		itemByKey[key] = len(summary.Items) - 1
		switch item.Status {
		case verificationStatusResolved:
			summary.ResolvedCount++
		case verificationStatusConflicted:
			summary.ConflictedCount++
		case verificationStatusInsufficient:
			summary.InsufficientCount++
		}
	}

	taskByID := make(map[string]Task, len(tasks))
	orderedAxes := make([]researchAxis, 0, len(tasks))
	seenAxes := map[string]struct{}{}
	for _, task := range tasks {
		taskByID[task.ID] = task
		axisID := normalizeAxisID(task.Axis)
		if axisID == "" {
			axisID = inferTaskAxis(task, brief, len(orderedAxes))
		}
		if _, ok := seenAxes[axisID]; ok {
			continue
		}
		seenAxes[axisID] = struct{}{}
		axis := briefAxisByID(brief, axisID)
		if axis != nil {
			orderedAxes = append(orderedAxes, *axis)
			continue
		}
		orderedAxes = append(orderedAxes, researchAxis{
			ID:         axisID,
			Label:      taskAxisLabel(task, lang),
			Priority:   maxInt(1, task.Priority),
			TimeWindow: task.TimeWindow,
			QueryHint:  task.Question,
		})
	}

	axisEvidence := map[string][]Evidence{}
	for _, ev := range evidence {
		task := taskByID[ev.TaskID]
		axisID := normalizeAxisID(task.Axis)
		if axisID == "" {
			axisID = inferTaskAxis(task, brief, 0)
		}
		axisEvidence[axisID] = append(axisEvidence[axisID], ev)
	}

	gaps := make([]researchGap, 0)
	for _, axis := range orderedAxes {
		evs := axisEvidence[axis.ID]
		item := VerificationItem{
			Focus:       axis.Label,
			Status:      verificationStatusResolved,
			EvidenceIDs: evidenceIDs(evs),
		}
		gap := researchGap{
			ID:          "axis:" + axis.ID,
			Focus:       axis.Label,
			Priority:    maxInt(1, axis.Priority),
			Axis:        axis.ID,
			EvidenceIDs: evidenceIDs(evs),
		}
		domains := uniqueDomains(evs)
		switch {
		case len(evs) == 0:
			item.Status = verificationStatusInsufficient
			item.Gap = localizedGapText(lang, axis.Label, "Need evidence coverage")
			item.Summary = localizedGapSummary(lang, axis.Label, 0, 0)
			gap.Status = item.Status
			gap.Gap = item.Gap
			gap.QueryHints = axisGapQueries(brief, axis, lang)
			gaps = append(gaps, gap)
		case axis.ID == "official" && !hasOfficialLikeEvidence(evs):
			item.Status = verificationStatusInsufficient
			item.Gap = localizedGapText(lang, axis.Label, "Need primary or official sources")
			item.Summary = localizedOfficialGapSummary(lang, axis.Label)
			gap.Status = item.Status
			gap.Gap = item.Gap
			gap.QueryHints = axisGapQueries(brief, axis, lang)
			gaps = append(gaps, gap)
		case len(evs) == 1 && axis.ID != "official" && brief.PersonTimeline:
			item.Status = verificationStatusInsufficient
			item.Gap = localizedGapText(lang, axis.Label, "Need broader evidence coverage")
			item.Summary = localizedGapSummary(lang, axis.Label, len(evs), len(domains))
			gap.Status = item.Status
			gap.Gap = item.Gap
			gap.QueryHints = axisGapQueries(brief, axis, lang)
			gaps = append(gaps, gap)
		default:
			item.Summary = localizedResolvedSummary(lang, axis.Label, len(evs), len(domains))
		}
		addItem(item)
	}

	if len(evidence) > 1 {
		domains := uniqueDomains(evidence)
		item := VerificationItem{
			Focus:       localizedFocusLabel(lang, "source_diversity", "Source diversity"),
			Status:      verificationStatusResolved,
			EvidenceIDs: evidenceIDs(evidence),
		}
		if len(domains) < 2 {
			item.Status = verificationStatusInsufficient
			item.Gap = localizedGapText(lang, item.Focus, "Need broader source diversity")
			item.Summary = localizedDiversityGapSummary(lang, len(domains))
			gaps = append(gaps, researchGap{
				ID:          "global:source_diversity",
				Focus:       item.Focus,
				Gap:         item.Gap,
				Status:      item.Status,
				Priority:    2,
				Axis:        "official",
				QueryHints:  diversityGapQueries(brief.Query, lang),
				EvidenceIDs: evidenceIDs(evidence),
			})
		} else {
			item.Summary = localizedDiversityResolvedSummary(lang, len(domains))
		}
		addItem(item)
	}

	if brief.WantsLatest || hasAxis(tasks, "latest") {
		freshYear := newestEvidenceYear(evidence)
		item := VerificationItem{
			Focus:       localizedFocusLabel(lang, "freshness", "Freshness"),
			Status:      verificationStatusResolved,
			EvidenceIDs: evidenceIDs(newestEvidenceSlice(evidence)),
		}
		currentYear := timeutil.NowTime().Year()
		if freshYear == 0 || freshYear < currentYear-2 {
			item.Status = verificationStatusInsufficient
			item.Gap = localizedGapText(lang, item.Focus, "Need fresher sources")
			item.Summary = localizedFreshnessGapSummary(lang, freshYear)
			gaps = append(gaps, researchGap{
				ID:          "global:freshness",
				Focus:       item.Focus,
				Gap:         item.Gap,
				Status:      item.Status,
				Priority:    1,
				Axis:        "latest",
				QueryHints:  freshnessGapQueries(brief.Query, lang),
				EvidenceIDs: item.EvidenceIDs,
			})
		} else {
			item.Summary = localizedFreshnessResolvedSummary(lang, freshYear)
		}
		addItem(item)
	}

	supportCount, conflictCount, hasConflict := analyzeEvidenceConsistencyByClaim(evidence)
	conflictItem := VerificationItem{
		Focus:       localizedFocusLabel(lang, "claim_validation", "Claim validation"),
		Status:      verificationStatusResolved,
		EvidenceIDs: evidenceIDs(evidence),
	}
	if hasConflict {
		conflictItem.Status = verificationStatusConflicted
		conflictItem.Gap = localizedGapText(lang, conflictItem.Focus, "Resolve conflicting claims")
		conflictItem.Summary = localizedConflictGapSummary(lang, supportCount, conflictCount)
		gaps = append(gaps, researchGap{
			ID:          "global:conflict",
			Focus:       conflictItem.Focus,
			Gap:         conflictItem.Gap,
			Status:      conflictItem.Status,
			Priority:    1,
			Axis:        "official",
			QueryHints:  conflictGapQueries(brief.Query, lang),
			EvidenceIDs: evidenceIDs(evidence),
		})
	} else {
		conflictItem.Summary = localizedConflictResolvedSummary(lang, supportCount)
	}
	addItem(conflictItem)

	return summary, dedupeResearchGaps(gaps)
}

func buildFollowUpTasks(brief researchBrief, gaps []researchGap, lang string, iteration, remainingSteps, nextIndex int) []Task {
	if remainingSteps <= 0 || len(gaps) == 0 {
		return nil
	}
	ordered := append([]researchGap(nil), gaps...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Priority == ordered[j].Priority {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].Priority < ordered[j].Priority
	})

	seenQueries := map[string]struct{}{}
	tasks := make([]Task, 0, remainingSteps)
	for _, gap := range ordered {
		queries := gap.QueryHints
		if len(queries) == 0 {
			queries = []string{fallbackFollowUpQuery(brief, gap, lang)}
		}
		perGap := 0
		for _, query := range queries {
			query = strings.TrimSpace(query)
			if query == "" {
				continue
			}
			key := strings.ToLower(query)
			if _, ok := seenQueries[key]; ok {
				continue
			}
			seenQueries[key] = struct{}{}
			tasks = append(tasks, Task{
				ID:          fmt.Sprintf("task_followup_%d_%d", iteration, nextIndex),
				Question:    query,
				Priority:    maxInt(1, gap.Priority),
				Depth:       maxInt(1, iteration),
				Status:      "pending",
				Axis:        normalizeAxisID(gap.Axis),
				Category:    "follow_up",
				TimeWindow:  followUpTimeWindow(brief, gap.Axis),
				FollowUpOf:  gap.ID,
				NegKeywords: followUpNegKeywords(brief, gap),
			})
			nextIndex++
			perGap++
			if len(tasks) >= remainingSteps || perGap >= 2 {
				break
			}
		}
		if len(tasks) >= remainingSteps {
			break
		}
	}
	return tasks
}

func topEvidenceWorkset(evidence []Evidence, limit int) []Evidence {
	if limit <= 0 || len(evidence) == 0 {
		return nil
	}
	cp := append([]Evidence(nil), evidence...)
	sort.SliceStable(cp, func(i, j int) bool {
		iScore := evidenceWorksetScore(cp[i])
		jScore := evidenceWorksetScore(cp[j])
		if iScore == jScore {
			return cp[i].RelevanceScore > cp[j].RelevanceScore
		}
		return iScore > jScore
	})
	if len(cp) > limit {
		cp = cp[:limit]
	}
	return cp
}

func evidenceWorksetScore(ev Evidence) float64 {
	freshness := 0.2
	if year := extractEvidenceYear(ev); year > 0 {
		age := maxInt(0, timeutil.NowTime().Year()-year)
		switch {
		case age == 0:
			freshness = 1.0
		case age <= 1:
			freshness = 0.85
		case age <= 2:
			freshness = 0.7
		case age <= 4:
			freshness = 0.5
		default:
			freshness = 0.3
		}
	}
	return ev.RelevanceScore*0.5 + ev.CredibilityScore*0.3 + ev.NoveltyScore*0.1 + freshness*0.1
}

func verificationOutcome(summary *VerificationSummary) string {
	if summary == nil {
		return verificationStatusInsufficient
	}
	switch {
	case summary.ConflictedCount > 0:
		return verificationStatusConflicted
	case summary.InsufficientCount > 0:
		return verificationStatusInsufficient
	default:
		return verificationStatusResolved
	}
}

func unresolvedResearchGaps(gaps []researchGap) []researchGap {
	out := make([]researchGap, 0, len(gaps))
	for _, gap := range gaps {
		if gap.Status == verificationStatusResolved {
			continue
		}
		out = append(out, gap)
	}
	return out
}

func hasHighPriorityGap(gaps []researchGap) bool {
	for _, gap := range gaps {
		if gap.Priority <= 2 {
			return true
		}
	}
	return false
}

func primaryResearchGap(gaps []researchGap) researchGap {
	if len(gaps) == 0 {
		return researchGap{}
	}
	best := gaps[0]
	for _, gap := range gaps[1:] {
		if gap.Priority < best.Priority {
			best = gap
		}
	}
	return best
}

func taskAxisLabel(task Task, lang string) string {
	if task.TimeWindow != "" {
		return task.TimeWindow
	}
	if task.Axis != "" {
		return task.Axis
	}
	if task.Category != "" {
		return task.Category
	}
	return localizedAxisLabel(lang, "research", "Research")
}

func hasAxis(tasks []Task, axisID string) bool {
	axisID = normalizeAxisID(axisID)
	for _, task := range tasks {
		if normalizeAxisID(task.Axis) == axisID {
			return true
		}
	}
	return false
}

func uniqueDomains(evidence []Evidence) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(evidence))
	for _, ev := range evidence {
		domain := strings.TrimSpace(strings.ToLower(ev.Domain))
		if domain == "" {
			continue
		}
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		out = append(out, domain)
	}
	sort.Strings(out)
	return out
}

func evidenceIDs(evidence []Evidence) []string {
	out := make([]string, 0, len(evidence))
	for _, ev := range evidence {
		if strings.TrimSpace(ev.ID) != "" {
			out = append(out, ev.ID)
		}
	}
	return dedupeStrings(out)
}

func newestEvidenceYear(evidence []Evidence) int {
	best := 0
	for _, ev := range evidence {
		if year := extractEvidenceYear(ev); year > best {
			best = year
		}
	}
	return best
}

func newestEvidenceSlice(evidence []Evidence) []Evidence {
	bestYear := newestEvidenceYear(evidence)
	if bestYear == 0 {
		return nil
	}
	out := make([]Evidence, 0, len(evidence))
	for _, ev := range evidence {
		if extractEvidenceYear(ev) == bestYear {
			out = append(out, ev)
		}
	}
	return out
}

func hasOfficialLikeEvidence(evidence []Evidence) bool {
	for _, ev := range evidence {
		domain := strings.ToLower(strings.TrimSpace(ev.Domain))
		text := strings.ToLower(strings.Join([]string{ev.Title, ev.URL, ev.Snippet, domain}, " "))
		switch {
		case strings.HasSuffix(domain, ".gov"), strings.HasSuffix(domain, ".edu"):
			return true
		case strings.Contains(text, "official"), strings.Contains(text, "官方"), strings.Contains(text, "documentation"), strings.Contains(text, "文档"):
			return true
		}
	}
	return false
}

func dedupeResearchGaps(gaps []researchGap) []researchGap {
	seen := map[string]struct{}{}
	out := make([]researchGap, 0, len(gaps))
	for _, gap := range gaps {
		key := strings.ToLower(strings.TrimSpace(gap.ID + "|" + gap.Gap + "|" + gap.Status))
		if key == "||" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		gap.QueryHints = dedupeStrings(gap.QueryHints)
		gap.EvidenceIDs = dedupeStrings(gap.EvidenceIDs)
		out = append(out, gap)
	}
	return out
}

func followUpTimeWindow(brief researchBrief, axisID string) string {
	if axis := briefAxisByID(brief, axisID); axis != nil {
		return axis.TimeWindow
	}
	return ""
}

func followUpNegKeywords(brief researchBrief, gap researchGap) []string {
	if !brief.PersonTimeline {
		return nil
	}
	base := []string{"同名", "other person", "无关"}
	if gap.Axis == "identity" {
		base = append(base, "非目标人物", "different person")
	}
	return dedupeStrings(base)
}

func axisGapQueries(brief researchBrief, axis researchAxis, lang string) []string {
	if brief.PersonTimeline {
		entity := firstNonEmpty(brief.Entity, brief.Query)
		switch axis.ID {
		case "identity":
			return []string{personIdentityQuery(entity, lang)}
		case "footprint":
			return []string{personFootprintQuery(entity, lang)}
		default:
			if axis.TimeWindow != "" {
				return []string{
					personPeriodQuery(entity, axis.TimeWindow, lang),
					buildOfficialPersonPeriodQuery(entity, axis.TimeWindow, lang),
				}
			}
		}
	}
	base := firstNonEmpty(axis.QueryHint, brief.Query)
	switch axis.ID {
	case "latest":
		return freshnessGapQueries(base, lang)
	case "official":
		return []string{buildOfficialQuery(base, lang), buildPrimarySourceQuery(base, lang)}
	case "comparison":
		return []string{buildComparisonQuery(base, lang), buildIndependentAnalysisQuery(base, lang)}
	case "best_practices":
		return []string{buildBestPracticeQuery(base, lang), buildCaseStudyQuery(base, lang)}
	default:
		return []string{base, buildOfficialQuery(base, lang)}
	}
}

func diversityGapQueries(query, lang string) []string {
	return []string{buildIndependentAnalysisQuery(query, lang), buildOfficialQuery(query, lang)}
}

func freshnessGapQueries(query, lang string) []string {
	currentYear := timeutil.NowTime().Year()
	if normalizeResearchLang(lang, query) == researchLangZH {
		return []string{
			fmt.Sprintf("%s 最新进展 %d 官方 公告", query, currentYear),
			fmt.Sprintf("%s 最新动态 一手来源", query),
		}
	}
	return []string{
		fmt.Sprintf("%s latest updates %d official announcement", query, currentYear),
		fmt.Sprintf("%s latest primary source verification", query),
	}
}

func conflictGapQueries(query, lang string) []string {
	if normalizeResearchLang(lang, query) == researchLangZH {
		return []string{
			fmt.Sprintf("%s 官方回应 一手来源", query),
			fmt.Sprintf("%s 发布时间 核验 官方", query),
		}
	}
	return []string{
		fmt.Sprintf("%s official statement primary source", query),
		fmt.Sprintf("%s publication date confirmation", query),
	}
}

func fallbackFollowUpQuery(brief researchBrief, gap researchGap, lang string) string {
	if len(gap.QueryHints) > 0 {
		return gap.QueryHints[0]
	}
	focus := firstNonEmpty(gap.Focus, gap.Axis, brief.Query)
	if normalizeResearchLang(lang, brief.Query) == researchLangZH {
		return fmt.Sprintf("%s 核验 一手来源", focus)
	}
	return fmt.Sprintf("%s primary source verification", focus)
}

func buildLatestQuery(query, lang string) string {
	if normalizeResearchLang(lang, query) == researchLangZH {
		return fmt.Sprintf("%s 最新进展", query)
	}
	return fmt.Sprintf("%s latest updates", query)
}

func buildOfficialQuery(query, lang string) string {
	if normalizeResearchLang(lang, query) == researchLangZH {
		return fmt.Sprintf("%s 官方文档 官方公告 一手来源", query)
	}
	return fmt.Sprintf("%s official documentation primary source", query)
}

func buildPrimarySourceQuery(query, lang string) string {
	if normalizeResearchLang(lang, query) == researchLangZH {
		return fmt.Sprintf("%s 一手来源 核验", query)
	}
	return fmt.Sprintf("%s primary source verification", query)
}

func buildComparisonQuery(query, lang string) string {
	if normalizeResearchLang(lang, query) == researchLangZH {
		return fmt.Sprintf("%s 对比 基准 评测", query)
	}
	return fmt.Sprintf("%s benchmark comparison", query)
}

func buildIndependentAnalysisQuery(query, lang string) string {
	if normalizeResearchLang(lang, query) == researchLangZH {
		return fmt.Sprintf("%s 独立分析 报道", query)
	}
	return fmt.Sprintf("%s independent analysis", query)
}

func buildBestPracticeQuery(query, lang string) string {
	if normalizeResearchLang(lang, query) == researchLangZH {
		return fmt.Sprintf("%s 最佳实践", query)
	}
	return fmt.Sprintf("%s best practices", query)
}

func buildCaseStudyQuery(query, lang string) string {
	if normalizeResearchLang(lang, query) == researchLangZH {
		return fmt.Sprintf("%s 案例研究", query)
	}
	return fmt.Sprintf("%s case study", query)
}

func buildOfficialPersonPeriodQuery(entity, period, lang string) string {
	if normalizeResearchLang(lang, entity) == researchLangZH {
		return fmt.Sprintf("%s %s 官方 访谈 一手来源", entity, period)
	}
	return fmt.Sprintf("%s %s official interview primary source", entity, period)
}

func queryWantsLatest(query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return false
	}
	for _, marker := range []string{"latest", "recent", "today", "current", "最新", "近期", "最近", "今年"} {
		if strings.Contains(q, marker) {
			return true
		}
	}
	return false
}

func looksLikeClaimValidation(query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return false
	}
	markers := []string{"是否", "是不是", "真的假的", "confirm", "confirmed", "verify", "verification", "did", "does", "is "}
	for _, marker := range markers {
		if strings.Contains(q, marker) {
			return true
		}
	}
	return false
}

func normalizeAxisID(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	replacer := strings.NewReplacer(" ", "_", "-", "_", "/", "_")
	raw = replacer.Replace(raw)
	return raw
}

func timelineAxisID(idx int) string {
	switch idx {
	case 0:
		return "earlier"
	case 1:
		return "middle"
	default:
		return "recent"
	}
}

func localizedAxisLabel(lang, axisID, fallback string) string {
	if normalizeResearchLang(lang, axisID) == researchLangZH {
		switch axisID {
		case "identity_validation":
			return "身份核验"
		case "internet_footprint":
			return "互联网足迹"
		case "overview":
			return "概览"
		case "latest":
			return "最新动态"
		case "official":
			return "官方来源"
		case "comparison":
			return "对比信息"
		case "best_practices":
			return "最佳实践"
		case "research":
			return "研究"
		}
	}
	return fallback
}

func localizedFocusLabel(lang, key, fallback string) string {
	if normalizeResearchLang(lang, key) == researchLangZH {
		switch key {
		case "source_diversity":
			return "来源多样性"
		case "freshness":
			return "时效性"
		case "claim_validation":
			return "结论核验"
		}
	}
	return fallback
}

func localizedGapText(lang, focus, fallback string) string {
	if normalizeResearchLang(lang, focus) == researchLangZH {
		switch fallback {
		case "Need evidence coverage":
			return fmt.Sprintf("需要补足%s证据", focus)
		case "Need primary or official sources":
			return fmt.Sprintf("需要补充%s的一手/官方来源", focus)
		case "Need broader evidence coverage":
			return fmt.Sprintf("需要扩展%s证据覆盖", focus)
		case "Need broader source diversity":
			return "需要扩展来源多样性"
		case "Need fresher sources":
			return "需要更新近的来源"
		case "Resolve conflicting claims":
			return "需要消解冲突结论"
		}
	}
	return fallback
}

func localizedGapSummary(lang, focus string, evidenceCount, domainCount int) string {
	if normalizeResearchLang(lang, focus) == researchLangZH {
		return fmt.Sprintf("%s仅覆盖 %d 条证据 / %d 个来源域名，需要继续深挖。", focus, evidenceCount, domainCount)
	}
	return fmt.Sprintf("%s only covers %d evidence item(s) across %d domain(s); follow-up research is needed.", focus, evidenceCount, domainCount)
}

func localizedOfficialGapSummary(lang, focus string) string {
	if normalizeResearchLang(lang, focus) == researchLangZH {
		return fmt.Sprintf("%s尚未拿到稳定的一手/官方来源支撑。", focus)
	}
	return fmt.Sprintf("%s still lacks stable primary or official sources.", focus)
}

func localizedResolvedSummary(lang, focus string, evidenceCount, domainCount int) string {
	if normalizeResearchLang(lang, focus) == researchLangZH {
		return fmt.Sprintf("%s已覆盖 %d 条证据 / %d 个来源域名。", focus, evidenceCount, domainCount)
	}
	return fmt.Sprintf("%s is covered by %d evidence item(s) across %d domain(s).", focus, evidenceCount, domainCount)
}

func localizedDiversityGapSummary(lang string, domainCount int) string {
	if isResearchLangZH(lang, "") {
		return fmt.Sprintf("当前仅覆盖 %d 个来源域名，需要扩展来源多样性。", domainCount)
	}
	return fmt.Sprintf("Only %d unique domain(s) were collected; broaden source diversity.", domainCount)
}

func localizedDiversityResolvedSummary(lang string, domainCount int) string {
	if isResearchLangZH(lang, "") {
		return fmt.Sprintf("当前已覆盖 %d 个来源域名。", domainCount)
	}
	return fmt.Sprintf("Coverage spans %d unique domain(s).", domainCount)
}

func localizedFreshnessGapSummary(lang string, year int) string {
	if isResearchLangZH(lang, "") {
		if year > 0 {
			return fmt.Sprintf("最新可确认年份为 %d，时效性不足。", year)
		}
		return "未能确认足够新的来源。"
	}
	if year > 0 {
		return fmt.Sprintf("The newest confirmed evidence is from %d, which is not fresh enough.", year)
	}
	return "No fresh evidence could be confirmed."
}

func localizedFreshnessResolvedSummary(lang string, year int) string {
	if isResearchLangZH(lang, "") {
		return fmt.Sprintf("已覆盖到 %d 年的较新来源。", year)
	}
	return fmt.Sprintf("Fresh evidence reaches %d.", year)
}

func localizedConflictGapSummary(lang string, supportCount, conflictCount int) string {
	if isResearchLangZH(lang, "") {
		return fmt.Sprintf("检测到 %d 组支持信号和 %d 组冲突信号，需要回查官方/一手来源。", supportCount, conflictCount)
	}
	return fmt.Sprintf("Detected %d supporting claim group(s) and %d conflicting group(s); verify against primary or official sources.", supportCount, conflictCount)
}

func localizedConflictResolvedSummary(lang string, supportCount int) string {
	if isResearchLangZH(lang, "") {
		return fmt.Sprintf("主要结论已被 %d 组支持信号覆盖。", supportCount)
	}
	return fmt.Sprintf("Core conclusions are supported across %d claim group(s).", supportCount)
}

func isResearchLangZH(lang, query string) bool {
	return normalizeResearchLang(lang, query) == researchLangZH
}
