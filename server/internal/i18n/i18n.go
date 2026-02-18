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
