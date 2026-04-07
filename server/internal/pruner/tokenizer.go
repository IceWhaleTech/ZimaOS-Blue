package pruner

import (
	"strings"
	"unicode"
)

// Stopwords is a set of common English words to filter from non-code content.
var Stopwords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
	"be": true, "by": true, "do": true, "for": true, "from": true,
	"has": true, "have": true, "he": true, "her": true, "his": true,
	"how": true, "if": true, "in": true, "is": true, "it": true, "its": true,
	"me": true, "my": true, "no": true, "not": true, "of": true, "on": true,
	"or": true, "our": true, "she": true, "so": true, "that": true,
	"the": true, "their": true, "them": true, "then": true, "there": true,
	"these": true, "they": true, "this": true, "to": true, "up": true,
	"us": true, "was": true, "we": true, "were": true, "what": true,
	"when": true, "which": true, "who": true, "will": true, "with": true,
	"you": true, "your": true,
	// Chinese stopwords
	"的": true, "了": true, "在": true, "是": true, "我": true, "有": true,
	"和": true, "就": true, "不": true, "人": true, "都": true, "一": true,
	"一个": true, "上": true, "也": true, "很": true, "到": true, "说": true,
	"要": true, "去": true, "你": true, "会": true, "着": true, "没有": true,
	"看": true, "好": true, "自己": true, "这": true,
}

// DefaultMemorySignals are words/phrases that indicate memorable content.
var DefaultMemorySignals = []string{
	// Preferences & decisions
	"prefer", "like", "dislike", "hate", "love", "favorite", "choose", "decided",
	"always", "never", "usually", "want", "need",
	// Personal info
	"my name", "i am", "i'm", "i live", "i work", "my job", "my email",
	"birthday", "age", "family",
	// Action items
	"todo", "remember", "don't forget", "remind", "deadline", "schedule",
	"important", "critical", "urgent",
	// Technical
	"api key", "password", "token", "config", "setting", "version",
	"install", "setup", "deploy",
	// Chinese equivalents
	"喜欢", "不喜欢", "偏好", "选择", "决定", "总是", "从不", "通常",
	"我叫", "我是", "住在", "工作", "邮箱", "生日",
	"记住", "别忘了", "提醒", "截止", "重要", "紧急",
	"密码", "密钥", "配置", "设置", "版本", "安装", "部署",
}

// TextTokenize splits text into lowercase tokens with stopword removal.
// Designed for non-code content (docs, logs, prose). Handles English and Chinese.
func TextTokenize(text string) []string {
	if text == "" {
		return nil
	}

	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	result := make([]string, 0, len(words))
	for _, w := range words {
		if len(w) >= 2 && !Stopwords[w] {
			result = append(result, w)
		}
	}
	return result
}

// SplitSentences splits text into sentences, handling both English and Chinese.
// Requires whitespace after punctuation to avoid breaking emails/URLs.
func SplitSentences(text string) []string {
	ensureSentenceSplitPattern()
	raw := sentenceSplitPattern.Split(text, -1)
	var result []string
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

// SignalWordScore returns a boost score (0-1) based on signal words found in text.
// Uses diminishing returns: 1 match = 0.5, 2 = 0.75, 3+ = ~1.0.
func SignalWordScore(text string, signals []string) float64 {
	lower := strings.ToLower(text)
	matches := 0
	for _, signal := range signals {
		if strings.Contains(lower, signal) {
			matches++
		}
	}
	if matches == 0 {
		return 0
	}
	return 1.0 - 1.0/float64(1+matches)
}

// UnifiedTokenize dispatches to the appropriate tokenizer based on content type.
func UnifiedTokenize(text string, ct ContentType) []string {
	switch ct {
	case ContentCode:
		return codeTokenize(text)
	default:
		return TextTokenize(text)
	}
}
