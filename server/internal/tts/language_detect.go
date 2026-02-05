package tts

import (
	"unicode"
)

// LanguageStats holds language detection statistics
type LanguageStats struct {
	Chinese  int     // 中文字符数
	Japanese int     // 日文假名数
	Korean   int     // 韩文字符数
	Latin    int     // 拉丁字符数 (英文等)
	Other    int     // 其他字符数
	Total    int     // 总有效字符数
}

// DetectLanguage detects the primary language of text based on Unicode ranges
// Returns the detected language code and confidence ratio (0.0-1.0)
func DetectLanguage(text string) (lang string, ratio float64) {
	stats := AnalyzeText(text)
	return stats.PrimaryLanguage()
}

// AnalyzeText analyzes text and returns language statistics
func AnalyzeText(text string) *LanguageStats {
	stats := &LanguageStats{}

	for _, r := range text {
		// Skip whitespace and punctuation
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}

		// Chinese characters (CJK Unified Ideographs)
		// Note: Japanese Kanji also falls in this range
		if isChinese(r) {
			stats.Chinese++
			stats.Total++
			continue
		}

		// Japanese Hiragana and Katakana
		if isJapanese(r) {
			stats.Japanese++
			stats.Total++
			continue
		}

		// Korean Hangul
		if isKorean(r) {
			stats.Korean++
			stats.Total++
			continue
		}

		// Latin characters (English, etc.)
		if isLatin(r) {
			stats.Latin++
			stats.Total++
			continue
		}

		// Other characters (numbers, etc.)
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			stats.Other++
			stats.Total++
		}
	}

	return stats
}

// PrimaryLanguage returns the primary language and its ratio
func (s *LanguageStats) PrimaryLanguage() (lang string, ratio float64) {
	if s.Total == 0 {
		return "en", 0.0
	}

	// Find the dominant language
	max := s.Latin
	lang = "en"

	// If Japanese kana exists, it's likely Japanese (even with Kanji)
	if s.Japanese > 0 {
		// Japanese text often mixes Kanji (Chinese chars) with Kana
		japaneseTotal := s.Japanese + s.Chinese
		if japaneseTotal > max {
			max = japaneseTotal
			lang = "ja"
			ratio = float64(japaneseTotal) / float64(s.Total)
			return lang, ratio
		}
	}

	if s.Chinese > max {
		max = s.Chinese
		lang = "cmn" // eSpeak-NG uses "cmn" for Mandarin
	}

	if s.Korean > max {
		max = s.Korean
		lang = "ko"
	}

	if s.Total > 0 {
		ratio = float64(max) / float64(s.Total)
	}

	return lang, ratio
}

// GetLanguageRatios returns all language ratios
func (s *LanguageStats) GetLanguageRatios() map[string]float64 {
	ratios := make(map[string]float64)
	if s.Total == 0 {
		return ratios
	}

	if s.Chinese > 0 {
		ratios["zh"] = float64(s.Chinese) / float64(s.Total)
	}
	if s.Japanese > 0 {
		ratios["ja"] = float64(s.Japanese) / float64(s.Total)
	}
	if s.Korean > 0 {
		ratios["ko"] = float64(s.Korean) / float64(s.Total)
	}
	if s.Latin > 0 {
		ratios["en"] = float64(s.Latin) / float64(s.Total)
	}

	return ratios
}

// IsMixedLanguage returns true if text contains multiple languages
func (s *LanguageStats) IsMixedLanguage() bool {
	count := 0
	if s.Chinese > 0 {
		count++
	}
	if s.Japanese > 0 {
		count++
	}
	if s.Korean > 0 {
		count++
	}
	if s.Latin > 0 {
		count++
	}
	return count > 1
}

// isChinese checks if a rune is a Chinese character
func isChinese(r rune) bool {
	return unicode.Is(unicode.Han, r)
}

// isJapanese checks if a rune is Japanese Hiragana or Katakana
func isJapanese(r rune) bool {
	return unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r)
}

// isKorean checks if a rune is Korean Hangul
func isKorean(r rune) bool {
	return unicode.Is(unicode.Hangul, r)
}

// isLatin checks if a rune is a Latin character
func isLatin(r rune) bool {
	return unicode.Is(unicode.Latin, r)
}
