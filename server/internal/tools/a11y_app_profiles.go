package tools

import "strings"

type a11yConversationAppProfile struct {
	ID                   string
	Aliases              []string
	EnableSearchFallback bool
	EnableVisualFastPath bool
	SearchPlans          func(hostOS string) []a11yConversationSearchPlan
}

var a11yConversationAppProfiles = []a11yConversationAppProfile{
	{
		ID:                   "feishu_lark",
		Aliases:              []string{"feishu", "飞书", "lark"},
		EnableSearchFallback: true,
		EnableVisualFastPath: true,
		SearchPlans:          a11yConversationSearchPlans,
	},
	{
		ID:                   "slack",
		Aliases:              []string{"slack"},
		EnableSearchFallback: true,
		EnableVisualFastPath: false,
		SearchPlans:          a11yConversationQuickSwitcherSearchPlans,
	},
}

func lookupA11yConversationAppProfile(args map[string]interface{}) *a11yConversationAppProfile {
	return lookupA11yConversationAppProfileValues(
		firstCompatString(args, "app_name", "appName", "application", "app"),
		firstCompatString(args, "window_title", "windowTitle", "title"),
	)
}

func lookupA11yConversationAppProfileValues(values ...string) *a11yConversationAppProfile {
	for idx := range a11yConversationAppProfiles {
		profile := &a11yConversationAppProfiles[idx]
		if profile.matchesAnyValue(values...) {
			return profile
		}
	}
	return nil
}

func (p *a11yConversationAppProfile) matchesAnyValue(values ...string) bool {
	if p == nil {
		return false
	}
	for _, value := range values {
		if p.matchesValue(value) {
			return true
		}
	}
	return false
}

func (p *a11yConversationAppProfile) matchesValue(value string) bool {
	if p == nil {
		return false
	}
	normalizedValue := normalizeA11yWindowMatchValue(value)
	if normalizedValue == "" {
		return false
	}
	for _, alias := range p.Aliases {
		normalizedAlias := normalizeA11yWindowMatchValue(alias)
		if normalizedAlias == "" {
			continue
		}
		if strings.Contains(normalizedValue, normalizedAlias) {
			return true
		}
	}
	return false
}

func a11yConversationSearchPlansForArgs(args map[string]interface{}, hostOS string) []a11yConversationSearchPlan {
	return a11yConversationSearchPlansForProfile(lookupA11yConversationAppProfile(args), hostOS)
}

func a11yConversationSearchPlansForProfile(profile *a11yConversationAppProfile, hostOS string) []a11yConversationSearchPlan {
	if profile == nil || profile.SearchPlans == nil {
		return nil
	}
	return profile.SearchPlans(hostOS)
}

func a11yConversationShortcutModifier(hostOS string) string {
	if strings.EqualFold(strings.TrimSpace(hostOS), "windows") {
		return "ctrl"
	}
	return "command"
}

func a11yConversationQuickSwitcherSearchPlans(hostOS string) []a11yConversationSearchPlan {
	modifier := a11yConversationShortcutModifier(hostOS)
	return []a11yConversationSearchPlan{
		{Name: "quick_switcher", Open: [][]string{{modifier, "k"}}},
	}
}
