package tts

import (
	"unicode"
)

// LanguageStats holds language detection statistics.
// CJKHan counts are shared across Chinese/Japanese/Korean since Han ideographs
// are used by all three languages. Disambiguating scripts (Kana, Hangul) break ties.
type LanguageStats struct {
	CJKHan   int // CJK Unified Ideographs (shared: zh, ja, ko)
	Kana     int // Hiragana + Katakana (unique to Japanese)
	Hangul   int // Korean Hangul (unique to Korean)
	Latin    int // Latin script (en, fr, de, es, pt, etc.)
	Cyrillic int // Cyrillic script (ru, uk, bg, etc.)
	Arabic   int // Arabic script (ar, fa, ur, etc.)
	Thai     int // Thai script
	Devanag  int // Devanagari script (hi, mr, ne, etc.)
	Other    int // Other letters/numbers
	Total    int // Total scored characters
}

// DetectLanguage detects the primary language of text based on Unicode ranges.
// Returns the detected language code and confidence ratio (0.0-1.0).
func DetectLanguage(text string) (lang string, ratio float64) {
	stats := AnalyzeText(text)
	return stats.PrimaryLanguage()
}

// AnalyzeText analyzes text and returns language statistics.
func AnalyzeText(text string) *LanguageStats {
	stats := &LanguageStats{}

	for _, r := range text {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}

		// Japanese Kana — check before Han so we don't double-count
		if unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			stats.Kana++
			stats.Total++
			continue
		}

		// Korean Hangul
		if unicode.Is(unicode.Hangul, r) {
			stats.Hangul++
			stats.Total++
			continue
		}

		// CJK Han ideographs — shared across zh/ja/ko
		if unicode.Is(unicode.Han, r) {
			stats.CJKHan++
			stats.Total++
			continue
		}

		// Latin
		if unicode.Is(unicode.Latin, r) {
			stats.Latin++
			stats.Total++
			continue
		}

		// Cyrillic
		if unicode.Is(unicode.Cyrillic, r) {
			stats.Cyrillic++
			stats.Total++
			continue
		}

		// Arabic
		if unicode.Is(unicode.Arabic, r) {
			stats.Arabic++
			stats.Total++
			continue
		}

		// Thai
		if unicode.Is(unicode.Thai, r) {
			stats.Thai++
			stats.Total++
			continue
		}

		// Devanagari
		if unicode.Is(unicode.Devanagari, r) {
			stats.Devanag++
			stats.Total++
			continue
		}

		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			stats.Other++
			stats.Total++
		}
	}

	return stats
}

// PrimaryLanguage returns the primary language and its confidence ratio.
// Han ideographs are distributed to CJK languages based on disambiguating scripts:
//   - If Kana present → Han counts toward Japanese
//   - If Hangul present → Han counts toward Korean
//   - If neither → Han counts toward Chinese
//   - If both Kana and Hangul → Han split proportionally
func (s *LanguageStats) PrimaryLanguage() (lang string, ratio float64) {
	if s.Total == 0 {
		return "en", 0.0
	}

	// Build effective scores for each language
	scores := s.effectiveScores()

	// Find the dominant language
	best := "en"
	bestScore := 0
	for l, sc := range scores {
		if sc > bestScore {
			bestScore = sc
			best = l
		}
	}

	if s.Total > 0 {
		ratio = float64(bestScore) / float64(s.Total)
	}
	return best, ratio
}

// effectiveScores distributes CJKHan to the appropriate CJK languages
// and returns per-language scores.
func (s *LanguageStats) effectiveScores() map[string]int {
	scores := make(map[string]int)

	// Unique-script languages
	if s.Latin > 0 {
		scores["en"] = s.Latin
	}
	if s.Cyrillic > 0 {
		scores["ru"] = s.Cyrillic
	}
	if s.Arabic > 0 {
		scores["ar"] = s.Arabic
	}
	if s.Thai > 0 {
		scores["th"] = s.Thai
	}
	if s.Devanag > 0 {
		scores["hi"] = s.Devanag
	}

	// Distribute CJK Han ideographs
	if s.CJKHan > 0 {
		kana := s.Kana
		hangul := s.Hangul
		disambig := kana + hangul

		if disambig == 0 {
			// No Kana or Hangul → pure Han → Chinese
			scores["cmn"] = s.CJKHan
		} else {
			// Distribute Han proportionally to disambiguating scripts
			if kana > 0 {
				jaHan := s.CJKHan * kana / disambig
				scores["ja"] = kana + jaHan
			}
			if hangul > 0 {
				koHan := s.CJKHan * hangul / disambig
				scores["ko"] = hangul + koHan
			}
			// Any remainder goes to Chinese if there's leftover
			distributed := 0
			if kana > 0 {
				distributed += s.CJKHan * kana / disambig
			}
			if hangul > 0 {
				distributed += s.CJKHan * hangul / disambig
			}
			if remainder := s.CJKHan - distributed; remainder > 0 {
				scores["cmn"] += remainder
			}
		}
	} else {
		// No Han at all — Kana/Hangul stand alone
		if s.Kana > 0 {
			scores["ja"] = s.Kana
		}
		if s.Hangul > 0 {
			scores["ko"] = s.Hangul
		}
	}

	return scores
}

// GetLanguageRatios returns all language ratios.
func (s *LanguageStats) GetLanguageRatios() map[string]float64 {
	ratios := make(map[string]float64)
	if s.Total == 0 {
		return ratios
	}
	scores := s.effectiveScores()
	for l, sc := range scores {
		if sc > 0 {
			ratios[l] = float64(sc) / float64(s.Total)
		}
	}
	return ratios
}

// IsMixedLanguage returns true if text contains multiple language scripts.
func (s *LanguageStats) IsMixedLanguage() bool {
	count := 0
	if s.CJKHan > 0 || s.Kana > 0 || s.Hangul > 0 {
		count++ // CJK family counts as one group
	}
	if s.Latin > 0 {
		count++
	}
	if s.Cyrillic > 0 {
		count++
	}
	if s.Arabic > 0 {
		count++
	}
	if s.Thai > 0 {
		count++
	}
	if s.Devanag > 0 {
		count++
	}
	// Also check if multiple CJK sub-scripts are present
	cjkSubs := 0
	if s.Kana > 0 {
		cjkSubs++
	}
	if s.Hangul > 0 {
		cjkSubs++
	}
	if s.CJKHan > 0 && s.Kana == 0 && s.Hangul == 0 {
		cjkSubs++ // pure Han = Chinese
	}
	if cjkSubs > 1 {
		count++ // multiple CJK languages
	}
	return count > 1
}

