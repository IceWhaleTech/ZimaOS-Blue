package selector

import "strings"

// CompactTerms lowercases, trims, and de-duplicates selector terms while
// preserving their first-seen order.
func CompactTerms(terms ...string) []string {
	seen := make(map[string]struct{}, len(terms))
	out := make([]string, 0, len(terms))
	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" {
			continue
		}
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		out = append(out, term)
	}
	return out
}

// ApplyCommonProfilePreset merges shared selector routing cues for tool and
// skill profiles. Callers can layer their own package-specific additions after
// applying the preset.
func ApplyCommonProfilePreset(profile SelectorProfile, name string) (SelectorProfile, bool) {
	preset, ok := commonProfilePreset(name)
	if !ok {
		return profile, false
	}

	profile.ExactAliases = CompactTerms(append(profile.ExactAliases, preset.ExactAliases...)...)
	profile.Actions = CompactTerms(append(profile.Actions, preset.Actions...)...)
	profile.Objects = CompactTerms(append(profile.Objects, preset.Objects...)...)
	profile.ContextCues = CompactTerms(append(profile.ContextCues, preset.ContextCues...)...)
	profile.NegativeCues = CompactTerms(append(profile.NegativeCues, preset.NegativeCues...)...)
	profile.PreferredDomains = CompactTerms(append(profile.PreferredDomains, preset.PreferredDomains...)...)
	profile.RequireAnyDomains = CompactTerms(append(profile.RequireAnyDomains, preset.RequireAnyDomains...)...)
	profile.ConflictDomains = CompactTerms(append(profile.ConflictDomains, preset.ConflictDomains...)...)

	return profile, true
}

func commonProfilePreset(name string) (SelectorProfile, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "ask":
		return SelectorProfile{
			Actions: []string{"ask", "clarify", "confirm", "question", "询问", "澄清", "确认", "提问"},
			Objects: []string{"question", "choice", "clarification", "问题", "选项", "澄清"},
		}, true
	case "browser":
		return SelectorProfile{
			Actions:          []string{"open", "visit", "navigate", "browse", "打开", "访问", "跳转", "浏览"},
			Objects:          []string{"url", "web", "website", "site", "web page", "webpage", "网页", "网站", "网址"},
			ContextCues:      []string{"http://", "https://", "www."},
			PreferredDomains: []string{DomainLiveWeb, DomainURLPresent},
		}, true
	case "ui_reviewer":
		return SelectorProfile{
			Actions:           []string{"review", "audit", "inspect", "evaluate", "critique", "score", "rate", "assess", "accessibility check", "评审", "审查", "检查", "点评", "打分", "评分", "无障碍检查"},
			Objects:           []string{"ui", "ux", "screen", "screenshot", "design", "mockup", "layout", "component", "website", "site", "webpage", "landing page", "app", "accessibility", "a11y", "visual", "界面", "截图", "设计稿", "布局", "组件", "网站", "网页", "落地页", "应用", "无障碍", "可访问性", "视觉"},
			RequireAnyDomains: []string{DomainUIArtifact},
			PreferredDomains:  []string{DomainUIArtifact},
			ConflictDomains:   []string{DomainLocalWorkspace, DomainProductivity},
		}, true
	case "reminder":
		return SelectorProfile{
			Actions:          []string{"remind", "notify", "提醒", "通知"},
			Objects:          []string{"reminder", "notification", "task", "提醒", "通知", "任务"},
			PreferredDomains: []string{DomainProductivity},
		}, true
	case "scheduler":
		return SelectorProfile{
			Actions:          []string{"schedule", "cron", "run every", "every hour", "定时", "调度", "计划任务"},
			Objects:          []string{"schedule", "job", "workflow", "日程", "工作流"},
			PreferredDomains: []string{DomainProductivity},
		}, true
	case "datetime":
		return SelectorProfile{
			Actions: []string{"time", "date", "today", "tomorrow", "timezone", "时间", "日期", "今天", "明天", "时区"},
			Objects: []string{"time", "date", "timezone", "clock", "时间", "日期", "时区"},
		}, true
	case "tasks":
		return SelectorProfile{
			Actions:          []string{"task", "todo", "track", "manage", "任务", "待办", "跟踪", "管理"},
			Objects:          []string{"task", "todo", "priority", "status", "任务", "待办", "优先级", "状态"},
			PreferredDomains: []string{DomainProductivity},
		}, true
	default:
		return SelectorProfile{}, false
	}
}
