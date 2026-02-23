// Package i18n provides internationalization support for server-side messages.
package i18n

import (
	"fmt"
	"strings"
	"sync"
)

// Language represents a supported language.
type Language string

const (
	// LangEnUS is English (US).
	LangEnUS Language = "en-US"
	// LangZhCN is Simplified Chinese.
	LangZhCN Language = "zh-CN"
)

// DefaultLanguage is the fallback language.
const DefaultLanguage = LangEnUS

// Message keys for error messages.
const (
	MsgProcessingError     = "error.processing"
	MsgChannelNotConnected = "error.channel_not_connected"
	MsgTimeout             = "error.timeout"
	MsgRateLimited         = "error.rate_limited"
	MsgServiceUnavailable  = "error.service_unavailable"
	MsgInvalidRequest      = "error.invalid_request"
	MsgUnauthorized        = "error.unauthorized"
	MsgInternalError       = "error.internal"
	MsgNoProviderAvailable = "error.no_provider_available"
	MsgProvidersInCooldown = "error.providers_in_cooldown"
)

// Message keys for iMessage channel.
const (
	MsgIMNotSetUp          = "imessage.not_set_up"
	MsgIMNotSignedIn       = "imessage.not_signed_in"
	MsgIMFullDiskAccess    = "imessage.full_disk_access"
	MsgIMAutomationDenied  = "imessage.automation_denied"
	MsgIMUnavailablePlatform = "imessage.unavailable_platform"
)

// Message keys for UI review report.
const (
	MsgUIReviewTitle         = "ui_review.title"
	MsgUIReviewOverall       = "ui_review.overall"
	MsgUIReviewPass          = "ui_review.pass"
	MsgUIReviewFail          = "ui_review.fail"
	MsgUIReviewVisual        = "ui_review.visual"
	MsgUIReviewFunctional    = "ui_review.functional"
	MsgUIReviewAccessibility = "ui_review.accessibility"
	MsgUIReviewIssues        = "ui_review.issues"
	MsgUIReviewSuggestions   = "ui_review.suggestions"
	MsgUIReviewCritical      = "ui_review.critical"
	MsgUIReviewMajor         = "ui_review.major"
	MsgUIReviewMinor         = "ui_review.minor"
)

// Message keys for UI review step names.
const (
	MsgStepPageLoad      = "ui_review.step.page_load"
	MsgStepFunctional    = "ui_review.step.functional"
	MsgStepAccessibility = "ui_review.step.accessibility"
	MsgStepViewport      = "ui_review.step.viewport"
	MsgStepScroll        = "ui_review.step.scroll"
	MsgStepScreenshot    = "ui_review.step.screenshot"
	MsgStepVisualReview  = "ui_review.step.visual_review"
	MsgStepVLMSkipped    = "ui_review.step.vlm_skipped"
	MsgStepVLMFailed     = "ui_review.step.vlm_failed"
	MsgStepStructural    = "ui_review.step.structural"
	MsgStepStructuralMsg = "ui_review.step.structural_msg"
	MsgSuggestionNoVLM   = "ui_review.suggestion.no_vlm"

	// Structural analysis issue descriptions
	MsgIssueNoHeading     = "ui_review.issue.no_heading"
	MsgIssueNoNavigation  = "ui_review.issue.no_navigation"
	MsgIssueSparseContent = "ui_review.issue.sparse_content"
	MsgIssueNoA11yTree    = "ui_review.issue.no_a11y_tree"
	MsgIssueA11yTreeFail  = "ui_review.issue.a11y_tree_fail"
	MsgIssueNoHeadingA11y = "ui_review.issue.no_heading_a11y"
	MsgIssueNoNavA11y     = "ui_review.issue.no_nav_a11y"
	MsgIssueVLMFailed     = "ui_review.issue.vlm_failed"

	// Card action messages (sent as user message when clicking card buttons)
	MsgActionRecheck     = "ui_review.action.recheck"
	MsgActionRecheckURL  = "ui_review.action.recheck_url"
	MsgActionA11yOnly    = "ui_review.action.a11y_only"
	MsgActionA11yOnlyURL = "ui_review.action.a11y_only_url"
	MsgActionFullReport  = "ui_review.action.full_report"
	MsgActionFullReportURL = "ui_review.action.full_report_url"
)

var (
	translations = map[Language]map[string]string{
		LangEnUS: {
			MsgProcessingError:     "Sorry, an error occurred while processing your message: %v",
			MsgChannelNotConnected: "The channel is not connected. Please try again later.",
			MsgTimeout:             "The request timed out. Please try again.",
			MsgRateLimited:         "Too many requests. Please wait a moment and try again.",
			MsgServiceUnavailable:  "The service is temporarily unavailable. Please try again later.",
			MsgInvalidRequest:      "Invalid request. Please check your input and try again.",
			MsgUnauthorized:        "You are not authorized to perform this action.",
			MsgInternalError:       "An internal error occurred. Please try again later.",
			MsgNoProviderAvailable: "No AI service provider is available. Please check the configuration or contact the administrator.",
			MsgProvidersInCooldown: "No AI service provider is currently available (%d providers are in cooldown). Please try again later.",
			// iMessage
			MsgIMNotSetUp:            "iMessage is not set up. Please open Messages.app and sign in with your Apple ID first.",
			MsgIMNotSignedIn:         "iMessage account is not signed in. Please open Messages.app and sign in with your Apple ID.",
			MsgIMFullDiskAccess:      "Messages database access denied. Grant Full Disk Access to \"%s\" in System Settings > Privacy & Security > Full Disk Access, then restart.",
			MsgIMAutomationDenied:    "Cannot send messages. Grant Automation permission for Messages.app to \"%s\" in System Settings > Privacy & Security > Automation.",
			MsgIMUnavailablePlatform: "iMessage is only available on macOS.",
			// UI Review
			MsgUIReviewTitle:         "📋 UI Review Report",
			MsgUIReviewOverall:       "Overall Score",
			MsgUIReviewPass:          "✅ PASS",
			MsgUIReviewFail:          "❌ FAIL",
			MsgUIReviewVisual:        "👁️ Visual",
			MsgUIReviewFunctional:    "⚙️ Functional",
			MsgUIReviewAccessibility: "♿ Accessibility",
			MsgUIReviewIssues:        "🔍 Issues (%d)",
			MsgUIReviewSuggestions:   "💡 Suggestions",
			MsgUIReviewCritical:      "critical",
			MsgUIReviewMajor:         "major",
			MsgUIReviewMinor:         "minor",
			// UI Review Steps
			MsgStepPageLoad:      "Page Load",
			MsgStepFunctional:    "Functional Check",
			MsgStepAccessibility: "Accessibility Check",
			MsgStepViewport:      "Viewport",
			MsgStepScroll:        "Scroll",
			MsgStepScreenshot:    "Screenshot",
			MsgStepVisualReview:  "Visual Review",
			MsgStepVLMSkipped:    "No LLM provider configured",
			MsgStepVLMFailed:     "VLM returned no result",
			MsgStepStructural:    "Structural Analysis",
			MsgStepStructuralMsg: "Estimated from page structure (no vision model)",
			MsgSuggestionNoVLM:   "Visual score is estimated from page structure (no vision model available). For accurate visual review, configure a vision-capable model.",
			// Structural analysis issues
			MsgIssueNoHeading:     "No heading structure detected — visual hierarchy may be unclear",
			MsgIssueNoNavigation:  "No navigation landmark — layout structure may be unclear",
			MsgIssueSparseContent: "Very few elements detected — page may be sparse or not fully loaded",
			MsgIssueNoA11yTree:    "Page accessibility tree unavailable — page may not have loaded correctly",
			MsgIssueA11yTreeFail:  "Could not retrieve accessibility tree",
			MsgIssueNoHeadingA11y: "No heading elements found in accessibility tree",
			MsgIssueNoNavA11y:     "No navigation landmark found",
			MsgIssueVLMFailed:     "VLM review failed — no visual score available",
			// Card action messages
			MsgActionRecheck:       "Please retry the UI review",
			MsgActionRecheckURL:    "Please re-run the UI review for %s",
			MsgActionA11yOnly:      "Run accessibility check",
			MsgActionA11yOnlyURL:   "Run accessibility check only for %s",
			MsgActionFullReport:    "Show full report",
			MsgActionFullReportURL: "Show full human-readable UI review report for %s",
		},
		LangZhCN: {
			MsgProcessingError:     "抱歉，处理您的消息时发生错误：%v",
			MsgChannelNotConnected: "频道未连接，请稍后重试。",
			MsgTimeout:             "请求超时，请重试。",
			MsgRateLimited:         "请求过于频繁，请稍等片刻后重试。",
			MsgServiceUnavailable:  "服务暂时不可用，请稍后重试。",
			MsgInvalidRequest:      "无效的请求，请检查您的输入后重试。",
			MsgUnauthorized:        "您没有权限执行此操作。",
			MsgInternalError:       "发生内部错误，请稍后重试。",
			MsgNoProviderAvailable: "没有可用的AI服务提供商，请检查配置或联系管理员。",
			MsgProvidersInCooldown: "暂时没有可用的AI服务提供商（有 %d 个提供商正在冷却中），请稍后重试。",
			// iMessage
			MsgIMNotSetUp:            "iMessage 尚未设置。请先打开「信息」应用并使用 Apple ID 登录。",
			MsgIMNotSignedIn:         "iMessage 账户未登录。请打开「信息」应用并使用 Apple ID 登录。",
			MsgIMFullDiskAccess:      "无法访问信息数据库。请在「系统设置 > 隐私与安全性 > 完全磁盘访问权限」中授权「%s」，然后重启。",
			MsgIMAutomationDenied:    "无法发送消息。请在「系统设置 > 隐私与安全性 > 自动化」中授权「%s」控制「信息」应用。",
			MsgIMUnavailablePlatform: "iMessage 仅在 macOS 上可用。",
			// UI Review
			MsgUIReviewTitle:         "📋 UI 评审报告",
			MsgUIReviewOverall:       "综合评分",
			MsgUIReviewPass:          "✅ 通过",
			MsgUIReviewFail:          "❌ 未通过",
			MsgUIReviewVisual:        "👁️ 视觉",
			MsgUIReviewFunctional:    "⚙️ 功能",
			MsgUIReviewAccessibility: "♿ 可访问性",
			MsgUIReviewIssues:        "🔍 问题 (%d)",
			MsgUIReviewSuggestions:   "💡 建议",
			MsgUIReviewCritical:      "严重",
			MsgUIReviewMajor:         "重要",
			MsgUIReviewMinor:         "轻微",
			// UI Review Steps
			MsgStepPageLoad:      "页面加载",
			MsgStepFunctional:    "功能检查",
			MsgStepAccessibility: "可访问性检查",
			MsgStepViewport:      "窗口尺寸",
			MsgStepScroll:        "滚动",
			MsgStepScreenshot:    "截图",
			MsgStepVisualReview:  "视觉评审",
			MsgStepVLMSkipped:    "未配置 LLM 提供商",
			MsgStepVLMFailed:     "VLM 未返回结果",
			MsgStepStructural:    "结构分析",
			MsgStepStructuralMsg: "基于页面结构估算（无视觉模型）",
			MsgSuggestionNoVLM:   "视觉评分基于页面结构估算（无视觉模型）。如需精确的视觉评审，请配置支持视觉的模型。",
			// Structural analysis issues
			MsgIssueNoHeading:     "未检测到标题结构——视觉层次可能不清晰",
			MsgIssueNoNavigation:  "未检测到导航区域——布局结构可能不清晰",
			MsgIssueSparseContent: "检测到的元素很少——页面可能内容稀疏或未完全加载",
			MsgIssueNoA11yTree:    "无法获取页面可访问性树——页面可能未正确加载",
			MsgIssueA11yTreeFail:  "无法获取可访问性树",
			MsgIssueNoHeadingA11y: "可访问性树中未找到标题元素",
			MsgIssueNoNavA11y:     "未找到导航区域",
			MsgIssueVLMFailed:     "VLM 评审失败——无法获取视觉评分",
			// Card action messages
			MsgActionRecheck:       "请重新进行 UI 评审",
			MsgActionRecheckURL:    "请重新评审 %s",
			MsgActionA11yOnly:      "仅进行可访问性检查",
			MsgActionA11yOnlyURL:   "仅对 %s 进行可访问性检查",
			MsgActionFullReport:    "显示完整报告",
			MsgActionFullReportURL: "显示 %s 的完整 UI 评审报告",
		},
	}
	mu sync.RWMutex
)

// T translates a message key to the specified language.
// If the key is not found, it returns the key itself.
func T(lang Language, key string, args ...interface{}) string {
	mu.RLock()
	defer mu.RUnlock()

	// Try the requested language
	if msgs, ok := translations[lang]; ok {
		if msg, ok := msgs[key]; ok {
			if len(args) > 0 {
				return fmt.Sprintf(msg, args...)
			}
			return msg
		}
	}

	// Fallback to default language
	if msgs, ok := translations[DefaultLanguage]; ok {
		if msg, ok := msgs[key]; ok {
			if len(args) > 0 {
				return fmt.Sprintf(msg, args...)
			}
			return msg
		}
	}

	// Return the key if not found
	return key
}

// ParseLanguage parses a language string and returns the corresponding Language.
// It handles common formats like "en", "en-US", "zh", "zh-CN", "zh-Hans".
func ParseLanguage(lang string) Language {
	if lang == "" {
		return DefaultLanguage
	}

	lang = strings.ToLower(strings.TrimSpace(lang))

	// Handle exact matches first
	switch lang {
	case "en-us", "en_us", "en":
		return LangEnUS
	case "zh-cn", "zh_cn", "zh", "zh-hans", "zh-sg":
		return LangZhCN
	}

	// Handle prefix matches
	if strings.HasPrefix(lang, "zh") {
		return LangZhCN
	}
	if strings.HasPrefix(lang, "en") {
		return LangEnUS
	}

	return DefaultLanguage
}

// AddTranslation adds or updates a translation for a specific language and key.
func AddTranslation(lang Language, key, value string) {
	mu.Lock()
	defer mu.Unlock()

	if _, ok := translations[lang]; !ok {
		translations[lang] = make(map[string]string)
	}
	translations[lang][key] = value
}
