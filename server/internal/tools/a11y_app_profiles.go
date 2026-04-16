package tools

import "strings"

type a11yConversationAppProfile struct {
	ID                              string
	Aliases                         []string
	EnableSearchFallback            bool
	EnableVisualFastPath            bool
	EnableSearchResultRoleFallback  bool
	SendVerificationSuccessStates   []string
	SendVerificationTransientStates []string
	SearchPlans                     func(hostOS string) []a11yConversationSearchPlan
}

var a11yConversationAppProfiles = []a11yConversationAppProfile{
	{
		ID:                              "feishu_lark",
		Aliases:                         []string{"feishu", "飞书", "lark"},
		EnableSearchFallback:            true,
		EnableVisualFastPath:            true,
		EnableSearchResultRoleFallback:  true,
		SendVerificationSuccessStates:   []string{"sent", "delivered"},
		SendVerificationTransientStates: []string{"pending", "sending"},
		SearchPlans:                     a11yConversationSearchPlans,
	},
	{
		ID:                              "slack",
		Aliases:                         []string{"slack"},
		EnableSearchFallback:            true,
		EnableVisualFastPath:            false,
		EnableSearchResultRoleFallback:  true,
		SendVerificationSuccessStates:   []string{"sent", "posted"},
		SendVerificationTransientStates: []string{"pending", "sending"},
		SearchPlans:                     a11yConversationQuickSwitcherSearchPlans,
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

func lookupA11yConversationAppProfileByID(id string) *a11yConversationAppProfile {
	normalizedID := strings.TrimSpace(strings.ToLower(id))
	if normalizedID == "" {
		return nil
	}
	for idx := range a11yConversationAppProfiles {
		profile := &a11yConversationAppProfiles[idx]
		if strings.TrimSpace(strings.ToLower(profile.ID)) == normalizedID {
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

func (p *a11yConversationAppProfile) matchesSendVerificationSuccessState(status string) bool {
	return p.matchesSendVerificationState(status, p.SendVerificationSuccessStates)
}

func (p *a11yConversationAppProfile) matchesSendVerificationTransientState(status string) bool {
	return p.matchesSendVerificationState(status, p.SendVerificationTransientStates)
}

func (p *a11yConversationAppProfile) matchesSendVerificationState(status string, accepted []string) bool {
	if p == nil {
		return false
	}
	normalizedStatus := strings.TrimSpace(strings.ToLower(status))
	if normalizedStatus == "" {
		return false
	}
	for _, candidate := range accepted {
		if strings.TrimSpace(strings.ToLower(candidate)) == normalizedStatus {
			return true
		}
	}
	return false
}

func a11yChatSendVerificationDisposition(profileID string, status string) string {
	normalizedStatus := strings.TrimSpace(strings.ToLower(status))
	if normalizedStatus == "" {
		return ""
	}
	if profile := lookupA11yConversationAppProfileByID(profileID); profile != nil {
		if profile.matchesSendVerificationSuccessState(normalizedStatus) {
			return "success"
		}
		if profile.matchesSendVerificationTransientState(normalizedStatus) {
			return "retry"
		}
	}
	switch normalizedStatus {
	case "sent":
		return "success"
	case "pending":
		return "retry"
	default:
		return "fail"
	}
}
