// Package i18n provides internationalization support for server-side messages.
package i18n

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Language represents a supported language.
type Language string

const (
	LangEnUS Language = "en-US"
	LangEnGB Language = "en-GB"
	LangZhCN Language = "zh-CN"
	LangZhTW Language = "zh-TW"
	LangJaJP Language = "ja-JP"
	LangKoKR Language = "ko-KR"
	LangDeDE Language = "de-DE"
	LangFrFR Language = "fr-FR"
	LangEsES Language = "es-ES"
	LangItIT Language = "it-IT"
	LangPtBR Language = "pt-BR"
	LangPtPT Language = "pt-PT"
	LangRuRU Language = "ru-RU"
	LangPlPL Language = "pl-PL"
	LangNlNL Language = "nl-NL"
	LangSvSE Language = "sv-SE"
	LangDaDK Language = "da-DK"
	LangNbNO Language = "nb-NO"
	LangCsCZ Language = "cs-CZ"
	LangSkSK Language = "sk-SK"
	LangHuHU Language = "hu-HU"
	LangRoRO Language = "ro-RO"
	LangHrHR Language = "hr-HR"
	LangElGR Language = "el-GR"
	LangCaES Language = "ca-ES"
	LangGaIE Language = "ga-IE"
	LangMlIN Language = "ml-IN"
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

// Message keys for media generation (channel-facing).
const (
	MsgMediaGenerating     = "media.generating"
	MsgMediaGenFailed      = "media.gen_failed"
	MsgMediaGenCancelled   = "media.gen_cancelled"
	MsgMediaGenNoOutput    = "media.gen_no_output"
	MsgMediaImageGenerated = "media.image_generated"
	MsgMediaVideoGenerated = "media.video_generated"
	MsgMediaGenerated      = "media.generated"
	MsgMediaGenTimeout     = "media.gen_timeout"

	// Common error reasons (used as %s in MsgMediaGenFailed)
	MsgErrRateLimit    = "media.err.rate_limit"
	MsgErrContentBlock = "media.err.content_block"
	MsgErrAPIFailed    = "media.err.api_failed"
	MsgErrUnknown      = "media.err.unknown"

	// Duration units
	MsgDurationSec = "duration.sec" // e.g. "5s" / "5秒"
	MsgDurationMin = "duration.min" // e.g. "2m" / "2分"
	MsgDurationHr  = "duration.hr"  // e.g. "1h" / "1时"
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
			// Media generation
			MsgMediaGenerating:     "🎨 Generating media... I'll send the result when it's ready.",
			MsgMediaGenFailed:      "❌ Media generation failed: %s",
			MsgMediaGenCancelled:   "🚫 Media generation was cancelled.",
			MsgMediaGenNoOutput:    "✅ Generation complete, but no output was returned.",
			MsgMediaImageGenerated: "✅ Image generated (%s)",
			MsgMediaVideoGenerated: "✅ Video generated (%s)",
			MsgMediaGenerated:      "✅ Media generated (%s)",
			MsgMediaGenTimeout:     "⏰ Media generation timed out. Please try again.",
			// Error reasons for media generation
			MsgErrRateLimit:    "rate limited, please try again later",
			MsgErrContentBlock: "content was blocked by safety filters",
			MsgErrAPIFailed:    "service temporarily unavailable",
			MsgErrUnknown:      "an unexpected error occurred",
			MsgDurationSec:     "%ds",
			MsgDurationMin:     "%dm",
			MsgDurationHr:      "%dh",
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
			// Media generation
			MsgMediaGenerating:     "🎨 正在生成媒体，完成后会发送给你。",
			MsgMediaGenFailed:      "❌ 媒体生成失败：%s",
			MsgMediaGenCancelled:   "🚫 媒体生成已取消。",
			MsgMediaGenNoOutput:    "✅ 生成完成，但没有返回结果。",
			MsgMediaImageGenerated: "✅ 图片已生成（%s）",
			MsgMediaVideoGenerated: "✅ 视频已生成（%s）",
			MsgMediaGenerated:      "✅ 媒体已生成（%s）",
			MsgMediaGenTimeout:     "⏰ 媒体生成超时，请重试。",
			// Error reasons for media generation
			MsgErrRateLimit:    "请求过于频繁，请稍后重试",
			MsgErrContentBlock: "内容被安全过滤器拦截",
			MsgErrAPIFailed:    "服务暂时不可用",
			MsgErrUnknown:      "发生了意外错误",
			MsgDurationSec:     "%d秒",
			MsgDurationMin:     "%d分",
			MsgDurationHr:      "%d时",
		},
		LangEnGB: {
			MsgMediaGenerating:     "🎨 Generating media… I'll send the result when it's ready.",
			MsgMediaGenFailed:      "❌ Media generation failed: %s",
			MsgMediaGenCancelled:   "🚫 Media generation was cancelled.",
			MsgMediaGenNoOutput:    "✅ Generation complete, but no output was returned.",
			MsgMediaImageGenerated: "✅ Image generated (%s)",
			MsgMediaVideoGenerated: "✅ Video generated (%s)",
			MsgMediaGenerated:      "✅ Media generated (%s)",
			MsgMediaGenTimeout:     "⏰ Media generation timed out. Please try again.",
			MsgErrRateLimit:        "rate limited, please try again later",
			MsgErrContentBlock:     "content was blocked by safety filters",
			MsgErrAPIFailed:        "service temporarily unavailable",
			MsgErrUnknown:          "an unexpected error occurred",
			MsgDurationSec:         "%ds",
			MsgDurationMin:         "%dm",
			MsgDurationHr:          "%dh",
		},
		LangZhTW: {
			MsgMediaGenerating:     "🎨 正在生成媒體，完成後會傳送給你。",
			MsgMediaGenFailed:      "❌ 媒體生成失敗：%s",
			MsgMediaGenCancelled:   "🚫 媒體生成已取消。",
			MsgMediaGenNoOutput:    "✅ 生成完成，但沒有回傳結果。",
			MsgMediaImageGenerated: "✅ 圖片已生成（%s）",
			MsgMediaVideoGenerated: "✅ 影片已生成（%s）",
			MsgMediaGenerated:      "✅ 媒體已生成（%s）",
			MsgMediaGenTimeout:     "⏰ 媒體生成逾時，請重試。",
			MsgErrRateLimit:        "請求過於頻繁，請稍後重試",
			MsgErrContentBlock:     "內容被安全過濾器攔截",
			MsgErrAPIFailed:        "服務暫時不可用",
			MsgErrUnknown:          "發生了意外錯誤",
			MsgDurationSec:         "%d秒",
			MsgDurationMin:         "%d分",
			MsgDurationHr:          "%d時",
		},
		LangJaJP: {
			MsgMediaGenerating:     "🎨 メディアを生成中です。完了したらお送りします。",
			MsgMediaGenFailed:      "❌ メディア生成に失敗しました：%s",
			MsgMediaGenCancelled:   "🚫 メディア生成がキャンセルされました。",
			MsgMediaGenNoOutput:    "✅ 生成は完了しましたが、出力がありませんでした。",
			MsgMediaImageGenerated: "✅ 画像を生成しました（%s）",
			MsgMediaVideoGenerated: "✅ 動画を生成しました（%s）",
			MsgMediaGenerated:      "✅ メディアを生成しました（%s）",
			MsgMediaGenTimeout:     "⏰ メディア生成がタイムアウトしました。もう一度お試しください。",
			MsgErrRateLimit:        "リクエストが多すぎます。しばらくしてから再試行してください",
			MsgErrContentBlock:     "コンテンツが安全フィルターによりブロックされました",
			MsgErrAPIFailed:        "サービスが一時的に利用できません",
			MsgErrUnknown:          "予期しないエラーが発生しました",
			MsgDurationSec:         "%d秒",
			MsgDurationMin:         "%d分",
			MsgDurationHr:          "%d時間",
		},
		LangKoKR: {
			MsgMediaGenerating:     "🎨 미디어를 생성 중입니다. 완료되면 보내드리겠습니다.",
			MsgMediaGenFailed:      "❌ 미디어 생성 실패: %s",
			MsgMediaGenCancelled:   "🚫 미디어 생성이 취소되었습니다.",
			MsgMediaGenNoOutput:    "✅ 생성이 완료되었지만 출력이 없습니다.",
			MsgMediaImageGenerated: "✅ 이미지 생성 완료 (%s)",
			MsgMediaVideoGenerated: "✅ 동영상 생성 완료 (%s)",
			MsgMediaGenerated:      "✅ 미디어 생성 완료 (%s)",
			MsgMediaGenTimeout:     "⏰ 미디어 생성 시간이 초과되었습니다. 다시 시도해 주세요.",
			MsgErrRateLimit:        "요청이 너무 많습니다. 잠시 후 다시 시도해 주세요",
			MsgErrContentBlock:     "콘텐츠가 안전 필터에 의해 차단되었습니다",
			MsgErrAPIFailed:        "서비스를 일시적으로 사용할 수 없습니다",
			MsgErrUnknown:          "예기치 않은 오류가 발생했습니다",
			MsgDurationSec:         "%d초",
			MsgDurationMin:         "%d분",
			MsgDurationHr:          "%d시간",
		},
		LangDeDE: {
			MsgMediaGenerating:     "🎨 Medien werden generiert… Ich sende das Ergebnis, sobald es fertig ist.",
			MsgMediaGenFailed:      "❌ Mediengenerierung fehlgeschlagen: %s",
			MsgMediaGenCancelled:   "🚫 Mediengenerierung wurde abgebrochen.",
			MsgMediaGenNoOutput:    "✅ Generierung abgeschlossen, aber keine Ausgabe erhalten.",
			MsgMediaImageGenerated: "✅ Bild generiert (%s)",
			MsgMediaVideoGenerated: "✅ Video generiert (%s)",
			MsgMediaGenerated:      "✅ Medien generiert (%s)",
			MsgMediaGenTimeout:     "⏰ Mediengenerierung hat das Zeitlimit überschritten. Bitte erneut versuchen.",
			MsgErrRateLimit:        "Zu viele Anfragen, bitte später erneut versuchen",
			MsgErrContentBlock:     "Inhalt wurde durch Sicherheitsfilter blockiert",
			MsgErrAPIFailed:        "Dienst vorübergehend nicht verfügbar",
			MsgErrUnknown:          "Ein unerwarteter Fehler ist aufgetreten",
			MsgDurationSec:         "%ds",
			MsgDurationMin:         "%dmin",
			MsgDurationHr:          "%dh",
		},
		LangFrFR: {
			MsgMediaGenerating:     "🎨 Génération du média en cours… Je vous enverrai le résultat dès qu'il sera prêt.",
			MsgMediaGenFailed:      "❌ Échec de la génération : %s",
			MsgMediaGenCancelled:   "🚫 La génération a été annulée.",
			MsgMediaGenNoOutput:    "✅ Génération terminée, mais aucun résultat retourné.",
			MsgMediaImageGenerated: "✅ Image générée (%s)",
			MsgMediaVideoGenerated: "✅ Vidéo générée (%s)",
			MsgMediaGenerated:      "✅ Média généré (%s)",
			MsgMediaGenTimeout:     "⏰ Délai de génération dépassé. Veuillez réessayer.",
			MsgErrRateLimit:        "trop de requêtes, veuillez réessayer plus tard",
			MsgErrContentBlock:     "contenu bloqué par les filtres de sécurité",
			MsgErrAPIFailed:        "service temporairement indisponible",
			MsgErrUnknown:          "une erreur inattendue s'est produite",
			MsgDurationSec:         "%ds",
			MsgDurationMin:         "%dmin",
			MsgDurationHr:          "%dh",
		},
		LangEsES: {
			MsgMediaGenerating:     "🎨 Generando contenido multimedia… Te enviaré el resultado cuando esté listo.",
			MsgMediaGenFailed:      "❌ Error en la generación: %s",
			MsgMediaGenCancelled:   "🚫 La generación fue cancelada.",
			MsgMediaGenNoOutput:    "✅ Generación completada, pero no se obtuvo resultado.",
			MsgMediaImageGenerated: "✅ Imagen generada (%s)",
			MsgMediaVideoGenerated: "✅ Vídeo generado (%s)",
			MsgMediaGenerated:      "✅ Contenido generado (%s)",
			MsgMediaGenTimeout:     "⏰ Tiempo de generación agotado. Por favor, inténtalo de nuevo.",
			MsgErrRateLimit:        "demasiadas solicitudes, inténtalo más tarde",
			MsgErrContentBlock:     "contenido bloqueado por filtros de seguridad",
			MsgErrAPIFailed:        "servicio temporalmente no disponible",
			MsgErrUnknown:          "se produjo un error inesperado",
			MsgDurationSec:         "%ds",
			MsgDurationMin:         "%dmin",
			MsgDurationHr:          "%dh",
		},
		LangItIT: {
			MsgMediaGenerating:     "🎨 Generazione in corso… Ti invierò il risultato quando sarà pronto.",
			MsgMediaGenFailed:      "❌ Generazione fallita: %s",
			MsgMediaGenCancelled:   "🚫 La generazione è stata annullata.",
			MsgMediaGenNoOutput:    "✅ Generazione completata, ma nessun risultato restituito.",
			MsgMediaImageGenerated: "✅ Immagine generata (%s)",
			MsgMediaVideoGenerated: "✅ Video generato (%s)",
			MsgMediaGenerated:      "✅ Media generato (%s)",
			MsgMediaGenTimeout:     "⏰ Timeout della generazione. Riprova.",
			MsgErrRateLimit:        "troppe richieste, riprova più tardi",
			MsgErrContentBlock:     "contenuto bloccato dai filtri di sicurezza",
			MsgErrAPIFailed:        "servizio temporaneamente non disponibile",
			MsgErrUnknown:          "si è verificato un errore imprevisto",
			MsgDurationSec:         "%ds",
			MsgDurationMin:         "%dmin",
			MsgDurationHr:          "%dh",
		},
		LangPtBR: {
			MsgMediaGenerating:     "🎨 Gerando mídia… Enviarei o resultado quando estiver pronto.",
			MsgMediaGenFailed:      "❌ Falha na geração: %s",
			MsgMediaGenCancelled:   "🚫 A geração foi cancelada.",
			MsgMediaGenNoOutput:    "✅ Geração concluída, mas nenhum resultado retornado.",
			MsgMediaImageGenerated: "✅ Imagem gerada (%s)",
			MsgMediaVideoGenerated: "✅ Vídeo gerado (%s)",
			MsgMediaGenerated:      "✅ Mídia gerada (%s)",
			MsgMediaGenTimeout:     "⏰ Tempo de geração esgotado. Tente novamente.",
			MsgErrRateLimit:        "muitas solicitações, tente novamente mais tarde",
			MsgErrContentBlock:     "conteúdo bloqueado pelos filtros de segurança",
			MsgErrAPIFailed:        "serviço temporariamente indisponível",
			MsgErrUnknown:          "ocorreu um erro inesperado",
			MsgDurationSec:         "%ds",
			MsgDurationMin:         "%dmin",
			MsgDurationHr:          "%dh",
		},
		LangPtPT: {
			MsgMediaGenerating:     "🎨 A gerar conteúdo… Enviarei o resultado quando estiver pronto.",
			MsgMediaGenFailed:      "❌ Falha na geração: %s",
			MsgMediaGenCancelled:   "🚫 A geração foi cancelada.",
			MsgMediaGenNoOutput:    "✅ Geração concluída, mas sem resultado devolvido.",
			MsgMediaImageGenerated: "✅ Imagem gerada (%s)",
			MsgMediaVideoGenerated: "✅ Vídeo gerado (%s)",
			MsgMediaGenerated:      "✅ Conteúdo gerado (%s)",
			MsgMediaGenTimeout:     "⏰ Tempo de geração excedido. Tente novamente.",
			MsgErrRateLimit:        "demasiados pedidos, tente novamente mais tarde",
			MsgErrContentBlock:     "conteúdo bloqueado pelos filtros de segurança",
			MsgErrAPIFailed:        "serviço temporariamente indisponível",
			MsgErrUnknown:          "ocorreu um erro inesperado",
			MsgDurationSec:         "%ds",
			MsgDurationMin:         "%dmin",
			MsgDurationHr:          "%dh",
		},
		LangRuRU: {
			MsgMediaGenerating:     "🎨 Генерация медиа… Отправлю результат, когда будет готово.",
			MsgMediaGenFailed:      "❌ Ошибка генерации: %s",
			MsgMediaGenCancelled:   "🚫 Генерация отменена.",
			MsgMediaGenNoOutput:    "✅ Генерация завершена, но результат не получен.",
			MsgMediaImageGenerated: "✅ Изображение создано (%s)",
			MsgMediaVideoGenerated: "✅ Видео создано (%s)",
			MsgMediaGenerated:      "✅ Медиа создано (%s)",
			MsgMediaGenTimeout:     "⏰ Время генерации истекло. Попробуйте снова.",
			MsgErrRateLimit:        "слишком много запросов, попробуйте позже",
			MsgErrContentBlock:     "контент заблокирован фильтрами безопасности",
			MsgErrAPIFailed:        "сервис временно недоступен",
			MsgErrUnknown:          "произошла непредвиденная ошибка",
			MsgDurationSec:         "%dс",
			MsgDurationMin:         "%dмин",
			MsgDurationHr:          "%dч",
		},
		LangPlPL: {
			MsgMediaGenerating:     "🎨 Generowanie… Wyślę wynik, gdy będzie gotowy.",
			MsgMediaGenFailed:      "❌ Generowanie nie powiodło się: %s",
			MsgMediaGenCancelled:   "🚫 Generowanie zostało anulowane.",
			MsgMediaGenNoOutput:    "✅ Generowanie zakończone, ale brak wyniku.",
			MsgMediaImageGenerated: "✅ Obraz wygenerowany (%s)",
			MsgMediaVideoGenerated: "✅ Wideo wygenerowane (%s)",
			MsgMediaGenerated:      "✅ Media wygenerowane (%s)",
			MsgMediaGenTimeout:     "⏰ Przekroczono limit czasu. Spróbuj ponownie.",
			MsgErrRateLimit:        "zbyt wiele żądań, spróbuj ponownie później",
			MsgErrContentBlock:     "treść zablokowana przez filtry bezpieczeństwa",
			MsgErrAPIFailed:        "usługa tymczasowo niedostępna",
			MsgErrUnknown:          "wystąpił nieoczekiwany błąd",
			MsgDurationSec:         "%ds",
			MsgDurationMin:         "%dmin",
			MsgDurationHr:          "%dh",
		},
		LangNlNL: {
			MsgMediaGenerating:     "🎨 Media genereren… Ik stuur het resultaat zodra het klaar is.",
			MsgMediaGenFailed:      "❌ Genereren mislukt: %s",
			MsgMediaGenCancelled:   "🚫 Genereren is geannuleerd.",
			MsgMediaGenNoOutput:    "✅ Genereren voltooid, maar geen resultaat.",
			MsgMediaImageGenerated: "✅ Afbeelding gegenereerd (%s)",
			MsgMediaVideoGenerated: "✅ Video gegenereerd (%s)",
			MsgMediaGenerated:      "✅ Media gegenereerd (%s)",
			MsgMediaGenTimeout:     "⏰ Time-out bij genereren. Probeer het opnieuw.",
			MsgErrRateLimit:        "te veel verzoeken, probeer het later opnieuw",
			MsgErrContentBlock:     "inhoud geblokkeerd door veiligheidsfilters",
			MsgErrAPIFailed:        "service tijdelijk niet beschikbaar",
			MsgErrUnknown:          "er is een onverwachte fout opgetreden",
			MsgDurationSec:         "%ds",
			MsgDurationMin:         "%dmin",
			MsgDurationHr:          "%du",
		},
		LangSvSE: {
			MsgMediaGenerating:     "🎨 Genererar media… Jag skickar resultatet när det är klart.",
			MsgMediaGenFailed:      "❌ Generering misslyckades: %s",
			MsgMediaGenCancelled:   "🚫 Genereringen avbröts.",
			MsgMediaGenNoOutput:    "✅ Generering klar, men inget resultat.",
			MsgMediaImageGenerated: "✅ Bild genererad (%s)",
			MsgMediaVideoGenerated: "✅ Video genererad (%s)",
			MsgMediaGenerated:      "✅ Media genererad (%s)",
			MsgMediaGenTimeout:     "⏰ Tidsgränsen för generering överskreds. Försök igen.",
			MsgErrRateLimit:        "för många förfrågningar, försök igen senare",
			MsgErrContentBlock:     "innehåll blockerat av säkerhetsfilter",
			MsgErrAPIFailed:        "tjänsten är tillfälligt otillgänglig",
			MsgErrUnknown:          "ett oväntat fel inträffade",
			MsgDurationSec:         "%ds",
			MsgDurationMin:         "%dmin",
			MsgDurationHr:          "%dt",
		},
		LangDaDK: {
			MsgMediaGenerating:     "🎨 Genererer medie… Jeg sender resultatet, når det er klar.",
			MsgMediaGenFailed:      "❌ Generering mislykkedes: %s",
			MsgMediaGenCancelled:   "🚫 Generering blev annulleret.",
			MsgMediaGenNoOutput:    "✅ Generering fuldført, men intet resultat.",
			MsgMediaImageGenerated: "✅ Billede genereret (%s)",
			MsgMediaVideoGenerated: "✅ Video genereret (%s)",
			MsgMediaGenerated:      "✅ Medie genereret (%s)",
			MsgMediaGenTimeout:     "⏰ Generering tog for lang tid. Prøv igen.",
			MsgErrRateLimit:        "for mange forespørgsler, prøv igen senere",
			MsgErrContentBlock:     "indhold blokeret af sikkerhedsfiltre",
			MsgErrAPIFailed:        "tjenesten er midlertidigt utilgængelig",
			MsgErrUnknown:          "der opstod en uventet fejl",
			MsgDurationSec:         "%ds",
			MsgDurationMin:         "%dmin",
			MsgDurationHr:          "%dt",
		},
		LangNbNO: {
			MsgMediaGenerating:     "🎨 Genererer media… Jeg sender resultatet når det er klart.",
			MsgMediaGenFailed:      "❌ Generering mislyktes: %s",
			MsgMediaGenCancelled:   "🚫 Generering ble avbrutt.",
			MsgMediaGenNoOutput:    "✅ Generering fullført, men ingen resultat.",
			MsgMediaImageGenerated: "✅ Bilde generert (%s)",
			MsgMediaVideoGenerated: "✅ Video generert (%s)",
			MsgMediaGenerated:      "✅ Media generert (%s)",
			MsgMediaGenTimeout:     "⏰ Generering tok for lang tid. Prøv igjen.",
			MsgErrRateLimit:        "for mange forespørsler, prøv igjen senere",
			MsgErrContentBlock:     "innhold blokkert av sikkerhetsfiltre",
			MsgErrAPIFailed:        "tjenesten er midlertidig utilgjengelig",
			MsgErrUnknown:          "det oppstod en uventet feil",
			MsgDurationSec:         "%ds",
			MsgDurationMin:         "%dmin",
			MsgDurationHr:          "%dt",
		},
		LangCsCZ: {
			MsgMediaGenerating:     "🎨 Generuji média… Výsledek pošlu, až bude hotový.",
			MsgMediaGenFailed:      "❌ Generování selhalo: %s",
			MsgMediaGenCancelled:   "🚫 Generování bylo zrušeno.",
			MsgMediaGenNoOutput:    "✅ Generování dokončeno, ale bez výsledku.",
			MsgMediaImageGenerated: "✅ Obrázek vygenerován (%s)",
			MsgMediaVideoGenerated: "✅ Video vygenerováno (%s)",
			MsgMediaGenerated:      "✅ Médium vygenerováno (%s)",
			MsgMediaGenTimeout:     "⏰ Časový limit generování vypršel. Zkuste to znovu.",
			MsgErrRateLimit:        "příliš mnoho požadavků, zkuste to později",
			MsgErrContentBlock:     "obsah zablokován bezpečnostními filtry",
			MsgErrAPIFailed:        "služba je dočasně nedostupná",
			MsgErrUnknown:          "došlo k neočekávané chybě",
			MsgDurationSec:         "%ds",
			MsgDurationMin:         "%dmin",
			MsgDurationHr:          "%dh",
		},
		LangSkSK: {
			MsgMediaGenerating:     "🎨 Generujem médiá… Výsledok pošlem, keď bude hotový.",
			MsgMediaGenFailed:      "❌ Generovanie zlyhalo: %s",
			MsgMediaGenCancelled:   "🚫 Generovanie bolo zrušené.",
			MsgMediaGenNoOutput:    "✅ Generovanie dokončené, ale bez výsledku.",
			MsgMediaImageGenerated: "✅ Obrázok vygenerovaný (%s)",
			MsgMediaVideoGenerated: "✅ Video vygenerované (%s)",
			MsgMediaGenerated:      "✅ Médium vygenerované (%s)",
			MsgMediaGenTimeout:     "⏰ Časový limit generovania vypršal. Skúste to znova.",
			MsgErrRateLimit:        "príliš veľa požiadaviek, skúste to neskôr",
			MsgErrContentBlock:     "obsah zablokovaný bezpečnostnými filtrami",
			MsgErrAPIFailed:        "služba je dočasne nedostupná",
			MsgErrUnknown:          "vyskytla sa neočakávaná chyba",
			MsgDurationSec:         "%ds",
			MsgDurationMin:         "%dmin",
			MsgDurationHr:          "%dh",
		},
		LangHuHU: {
			MsgMediaGenerating:     "🎨 Média generálása… Az eredményt elküldöm, ha kész.",
			MsgMediaGenFailed:      "❌ Generálás sikertelen: %s",
			MsgMediaGenCancelled:   "🚫 A generálás megszakítva.",
			MsgMediaGenNoOutput:    "✅ Generálás kész, de nincs eredmény.",
			MsgMediaImageGenerated: "✅ Kép generálva (%s)",
			MsgMediaVideoGenerated: "✅ Videó generálva (%s)",
			MsgMediaGenerated:      "✅ Média generálva (%s)",
			MsgMediaGenTimeout:     "⏰ Generálási időtúllépés. Próbálja újra.",
			MsgErrRateLimit:        "túl sok kérés, próbálja később",
			MsgErrContentBlock:     "tartalom biztonsági szűrők által blokkolva",
			MsgErrAPIFailed:        "a szolgáltatás átmenetileg nem elérhető",
			MsgErrUnknown:          "váratlan hiba történt",
			MsgDurationSec:         "%dmp",
			MsgDurationMin:         "%dp",
			MsgDurationHr:          "%dó",
		},
		LangRoRO: {
			MsgMediaGenerating:     "🎨 Se generează media… Voi trimite rezultatul când este gata.",
			MsgMediaGenFailed:      "❌ Generarea a eșuat: %s",
			MsgMediaGenCancelled:   "🚫 Generarea a fost anulată.",
			MsgMediaGenNoOutput:    "✅ Generare completă, dar fără rezultat.",
			MsgMediaImageGenerated: "✅ Imagine generată (%s)",
			MsgMediaVideoGenerated: "✅ Video generat (%s)",
			MsgMediaGenerated:      "✅ Media generată (%s)",
			MsgMediaGenTimeout:     "⏰ Timpul de generare a expirat. Încercați din nou.",
			MsgErrRateLimit:        "prea multe cereri, încercați mai târziu",
			MsgErrContentBlock:     "conținut blocat de filtrele de securitate",
			MsgErrAPIFailed:        "serviciu temporar indisponibil",
			MsgErrUnknown:          "a apărut o eroare neașteptată",
			MsgDurationSec:         "%ds",
			MsgDurationMin:         "%dmin",
			MsgDurationHr:          "%dh",
		},
		LangHrHR: {
			MsgMediaGenerating:     "🎨 Generiranje medija… Poslat ću rezultat kad bude gotov.",
			MsgMediaGenFailed:      "❌ Generiranje nije uspjelo: %s",
			MsgMediaGenCancelled:   "🚫 Generiranje je otkazano.",
			MsgMediaGenNoOutput:    "✅ Generiranje završeno, ali nema rezultata.",
			MsgMediaImageGenerated: "✅ Slika generirana (%s)",
			MsgMediaVideoGenerated: "✅ Video generiran (%s)",
			MsgMediaGenerated:      "✅ Medij generiran (%s)",
			MsgMediaGenTimeout:     "⏰ Isteklo je vrijeme generiranja. Pokušajte ponovo.",
			MsgErrRateLimit:        "previše zahtjeva, pokušajte kasnije",
			MsgErrContentBlock:     "sadržaj blokiran sigurnosnim filtrima",
			MsgErrAPIFailed:        "usluga privremeno nedostupna",
			MsgErrUnknown:          "došlo je do neočekivane pogreške",
			MsgDurationSec:         "%ds",
			MsgDurationMin:         "%dmin",
			MsgDurationHr:          "%dh",
		},
		LangElGR: {
			MsgMediaGenerating:     "🎨 Δημιουργία πολυμέσου… Θα στείλω το αποτέλεσμα όταν είναι έτοιμο.",
			MsgMediaGenFailed:      "❌ Η δημιουργία απέτυχε: %s",
			MsgMediaGenCancelled:   "🚫 Η δημιουργία ακυρώθηκε.",
			MsgMediaGenNoOutput:    "✅ Η δημιουργία ολοκληρώθηκε, αλλά χωρίς αποτέλεσμα.",
			MsgMediaImageGenerated: "✅ Εικόνα δημιουργήθηκε (%s)",
			MsgMediaVideoGenerated: "✅ Βίντεο δημιουργήθηκε (%s)",
			MsgMediaGenerated:      "✅ Πολυμέσο δημιουργήθηκε (%s)",
			MsgMediaGenTimeout:     "⏰ Λήξη χρονικού ορίου δημιουργίας. Δοκιμάστε ξανά.",
			MsgErrRateLimit:        "πάρα πολλά αιτήματα, δοκιμάστε αργότερα",
			MsgErrContentBlock:     "το περιεχόμενο αποκλείστηκε από φίλτρα ασφαλείας",
			MsgErrAPIFailed:        "η υπηρεσία είναι προσωρινά μη διαθέσιμη",
			MsgErrUnknown:          "παρουσιάστηκε μη αναμενόμενο σφάλμα",
			MsgDurationSec:         "%dδ",
			MsgDurationMin:         "%dλ",
			MsgDurationHr:          "%dω",
		},
		LangCaES: {
			MsgMediaGenerating:     "🎨 Generant contingut… T'enviaré el resultat quan estigui llest.",
			MsgMediaGenFailed:      "❌ Error en la generació: %s",
			MsgMediaGenCancelled:   "🚫 La generació s'ha cancel·lat.",
			MsgMediaGenNoOutput:    "✅ Generació completada, però sense resultat.",
			MsgMediaImageGenerated: "✅ Imatge generada (%s)",
			MsgMediaVideoGenerated: "✅ Vídeo generat (%s)",
			MsgMediaGenerated:      "✅ Contingut generat (%s)",
			MsgMediaGenTimeout:     "⏰ Temps de generació esgotat. Torna-ho a provar.",
			MsgErrRateLimit:        "massa sol·licituds, torna-ho a provar més tard",
			MsgErrContentBlock:     "contingut bloquejat pels filtres de seguretat",
			MsgErrAPIFailed:        "servei temporalment no disponible",
			MsgErrUnknown:          "s'ha produït un error inesperat",
			MsgDurationSec:         "%ds",
			MsgDurationMin:         "%dmin",
			MsgDurationHr:          "%dh",
		},
		LangGaIE: {
			MsgMediaGenerating:     "🎨 Ag giniúint meán… Seolfaidh mé an toradh nuair a bheidh sé réidh.",
			MsgMediaGenFailed:      "❌ Theip ar an nginiúint: %s",
			MsgMediaGenCancelled:   "🚫 Cuireadh an ghiniúint ar ceal.",
			MsgMediaGenNoOutput:    "✅ Giniúint críochnaithe, ach gan toradh.",
			MsgMediaImageGenerated: "✅ Íomhá ginte (%s)",
			MsgMediaVideoGenerated: "✅ Físeán ginte (%s)",
			MsgMediaGenerated:      "✅ Meán ginte (%s)",
			MsgMediaGenTimeout:     "⏰ Teorainn ama na giniúna caite. Bain triail eile as.",
			MsgErrRateLimit:        "an iomarca iarratas, bain triail eile as ar ball",
			MsgErrContentBlock:     "ábhar blocáilte ag scagairí sábháilteachta",
			MsgErrAPIFailed:        "seirbhís gan fáil go sealadach",
			MsgErrUnknown:          "tharla earráid gan choinne",
			MsgDurationSec:         "%ds",
			MsgDurationMin:         "%dn",
			MsgDurationHr:          "%du",
		},
		LangMlIN: {
			MsgMediaGenerating:     "🎨 മീഡിയ ജനറേറ്റ് ചെയ്യുന്നു… തയ്യാറാകുമ്പോൾ ഫലം അയയ്ക്കാം.",
			MsgMediaGenFailed:      "❌ ജനറേഷൻ പരാജയപ്പെട്ടു: %s",
			MsgMediaGenCancelled:   "🚫 ജനറേഷൻ റദ്ദാക്കി.",
			MsgMediaGenNoOutput:    "✅ ജനറേഷൻ പൂർത്തിയായി, പക്ഷേ ഫലമില്ല.",
			MsgMediaImageGenerated: "✅ ചിത്രം ജനറേറ്റ് ചെയ്തു (%s)",
			MsgMediaVideoGenerated: "✅ വീഡിയോ ജനറേറ്റ് ചെയ്തു (%s)",
			MsgMediaGenerated:      "✅ മീഡിയ ജനറേറ്റ് ചെയ്തു (%s)",
			MsgMediaGenTimeout:     "⏰ ജനറേഷൻ സമയപരിധി കഴിഞ്ഞു. വീണ്ടും ശ്രമിക്കുക.",
			MsgErrRateLimit:        "അധികം അഭ്യർത്ഥനകൾ, പിന്നീട് ശ്രമിക്കുക",
			MsgErrContentBlock:     "സുരക്ഷാ ഫിൽട്ടറുകൾ ഉള്ളടക്കം തടഞ്ഞു",
			MsgErrAPIFailed:        "സേവനം താൽക്കാലികമായി ലഭ്യമല്ല",
			MsgErrUnknown:          "അപ്രതീക്ഷിത പിശക് സംഭവിച്ചു",
			MsgDurationSec:         "%dസെ",
			MsgDurationMin:         "%dമി",
			MsgDurationHr:          "%dമ",
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

// FormatDuration formats a time.Duration as a localized string like "1h2m3s" / "1时2分3秒".
func FormatDuration(lang Language, d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalSec := int(d.Seconds())
	h := totalSec / 3600
	m := (totalSec % 3600) / 60
	s := totalSec % 60

	var parts []string
	if h > 0 {
		parts = append(parts, T(lang, MsgDurationHr, h))
	}
	if m > 0 {
		parts = append(parts, T(lang, MsgDurationMin, m))
	}
	// Always show seconds if no hours/minutes, or if there are remaining seconds
	if s > 0 || len(parts) == 0 {
		parts = append(parts, T(lang, MsgDurationSec, s))
	}
	return strings.Join(parts, "")
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
	case "en-gb", "en_gb":
		return LangEnGB
	case "zh-cn", "zh_cn", "zh", "zh-hans", "zh-sg":
		return LangZhCN
	case "zh-tw", "zh_tw", "zh-hant", "zh-hk":
		return LangZhTW
	case "ja-jp", "ja_jp", "ja":
		return LangJaJP
	case "ko-kr", "ko_kr", "ko":
		return LangKoKR
	case "de-de", "de_de", "de":
		return LangDeDE
	case "fr-fr", "fr_fr", "fr":
		return LangFrFR
	case "es-es", "es_es", "es":
		return LangEsES
	case "it-it", "it_it", "it":
		return LangItIT
	case "pt-br", "pt_br":
		return LangPtBR
	case "pt-pt", "pt_pt", "pt":
		return LangPtPT
	case "ru-ru", "ru_ru", "ru":
		return LangRuRU
	case "pl-pl", "pl_pl", "pl":
		return LangPlPL
	case "nl-nl", "nl_nl", "nl":
		return LangNlNL
	case "sv-se", "sv_se", "sv":
		return LangSvSE
	case "da-dk", "da_dk", "da":
		return LangDaDK
	case "nb-no", "nb_no", "nb", "no":
		return LangNbNO
	case "cs-cz", "cs_cz", "cs":
		return LangCsCZ
	case "sk-sk", "sk_sk", "sk":
		return LangSkSK
	case "hu-hu", "hu_hu", "hu":
		return LangHuHU
	case "ro-ro", "ro_ro", "ro":
		return LangRoRO
	case "hr-hr", "hr_hr", "hr":
		return LangHrHR
	case "el-gr", "el_gr", "el":
		return LangElGR
	case "ca-es", "ca_es", "ca":
		return LangCaES
	case "ga-ie", "ga_ie", "ga":
		return LangGaIE
	case "ml-in", "ml_in", "ml":
		return LangMlIN
	}

	// Handle prefix matches for Chinese and English variants
	if strings.HasPrefix(lang, "zh") {
		return LangZhCN
	}
	if strings.HasPrefix(lang, "en") {
		return LangEnUS
	}
	if strings.HasPrefix(lang, "pt") {
		return LangPtPT
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
