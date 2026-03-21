package deepresearch

import (
	"fmt"
	"regexp"
	"strings"
)

var personNamePattern = regexp.MustCompile(`[\p{Han}]{2,4}|[A-Za-z][A-Za-z\.\- ]{2,40}`)

func looksLikePersonTimelineResearch(query, lang string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return false
	}

	if personTimelinePrimaryCueMatcher.Contains(q) ||
		(personTimelineViewCueMatcher.Contains(q) && personTimelineArticleCueMatcher.Contains(q)) {
		return true
	}
	if personTimelineResearchCueMatcher.Contains(q) && personTimelinePartnerCueMatcher.Contains(q) {
		return true
	}

	return false
}

func planPersonResearchTasks(query string, mode Mode, lang string) []Task {
	entity := extractPrimaryEntityName(query)
	if entity == "" {
		entity = strings.TrimSpace(query)
	}

	timeLabels := []string{"earlier", "middle", "recent"}
	if normalizeResearchLang(lang, query) == researchLangZH {
		timeLabels = []string{"早期", "中期", "近期"}
	}

	base := []Task{
		{
			ID:       "task_identity",
			Question: personIdentityQuery(entity, lang),
			Priority: 1,
			Depth:    1,
			Status:   "pending",
			Axis:     "identity",
			Category: "identity_validation",
			NegKeywords: []string{
				"同名", "另一位", "爱驰", "车联", "滴滴高管", "非蓝驰", "different person",
			},
		},
		{
			ID:          "task_early",
			Question:    personPeriodQuery(entity, timeLabels[0], lang),
			Priority:    2,
			Depth:       1,
			Status:      "pending",
			Axis:        timelineAxisID(0),
			Category:    "viewpoint_period",
			TimeWindow:  timeLabels[0],
			NegKeywords: []string{"无关", "同名", "other person"},
		},
		{
			ID:          "task_mid",
			Question:    personPeriodQuery(entity, timeLabels[1], lang),
			Priority:    3,
			Depth:       1,
			Status:      "pending",
			Axis:        timelineAxisID(1),
			Category:    "viewpoint_period",
			TimeWindow:  timeLabels[1],
			NegKeywords: []string{"无关", "同名", "other person"},
		},
		{
			ID:          "task_recent",
			Question:    personPeriodQuery(entity, timeLabels[2], lang),
			Priority:    4,
			Depth:       1,
			Status:      "pending",
			Axis:        timelineAxisID(2),
			Category:    "viewpoint_period",
			TimeWindow:  timeLabels[2],
			NegKeywords: []string{"无关", "同名", "other person"},
		},
		{
			ID:          "task_footprint",
			Question:    personFootprintQuery(entity, lang),
			Priority:    5,
			Depth:       1,
			Status:      "pending",
			Axis:        "footprint",
			Category:    "internet_footprint",
			NegKeywords: []string{"广告", "软文", "转载", "spam"},
		},
	}

	switch mode {
	case ModeFast:
		return base[:3]
	case ModeStandard:
		return base[:4]
	default:
		return base
	}
}

func extractPrimaryEntityName(query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return ""
	}

	// Prefer explicit "针对 X" style.
	if idx := strings.Index(q, "针对"); idx >= 0 {
		raw := strings.TrimSpace(q[idx+len("针对"):])
		if cut := strings.IndexAny(raw, "，,。.;；:：\n"); cut > 0 {
			raw = raw[:cut]
		}
		raw = strings.Trim(raw, " \"'“”‘’")
		if len(raw) >= 2 {
			return raw
		}
	}

	candidates := personNamePattern.FindAllString(q, -1)
	longest := ""
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if len(c) > len(longest) {
			longest = c
		}
	}
	return longest
}

func personIdentityQuery(entity, lang string) string {
	switch normalizeResearchLang(lang, entity) {
	case researchLangZH:
		return fmt.Sprintf("%s 身份背景 蓝驰 合伙人 资料 来源核验", entity)
	default:
		return fmt.Sprintf("%s profile identity background sources verification", entity)
	}
}

func personPeriodQuery(entity, period, lang string) string {
	switch normalizeResearchLang(lang, entity) {
	case researchLangZH:
		return fmt.Sprintf("%s %s 文章 观点 洞察 访谈 演讲", entity, period)
	default:
		return fmt.Sprintf("%s %s period articles viewpoints interviews talks", entity, period)
	}
}

func personFootprintQuery(entity, lang string) string {
	switch normalizeResearchLang(lang, entity) {
	case researchLangZH:
		return fmt.Sprintf("%s 互联网分享 平台 足迹 公众号 知乎 播客", entity)
	default:
		return fmt.Sprintf("%s internet footprint platforms posts podcast", entity)
	}
}
