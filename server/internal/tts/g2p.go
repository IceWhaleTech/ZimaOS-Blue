//go:build kokoro

package tts

import (
	"regexp"
	"strings"
	"unicode"
)

// G2PDispatcher routes text to language-specific G2P backends.
type G2PDispatcher struct {
	en *EnglishG2P
	zh *ChineseG2P
	ja *JapaneseG2P
}

// NewG2PDispatcher creates a dispatcher with all language backends.
func NewG2PDispatcher() *G2PDispatcher {
	en := NewEnglishG2P()
	return &G2PDispatcher{
		en: en,
		zh: NewChineseG2P(en),
		ja: NewJapaneseG2P(),
	}
}

// Warmup pre-loads dictionaries so the first Synthesize call is fast.
func (d *G2PDispatcher) Warmup() {
	d.en.init() // loads us_gold.json (~3MB)
}

// cjkRe matches CJK Unified Ideographs and common CJK punctuation.
var cjkRe = regexp.MustCompile(`[\x{4E00}-\x{9FFF}\x{3000}-\x{303F}\x{FF00}-\x{FFEF}]+`)

// japaneseRe matches Hiragana, Katakana, and CJK characters.
var japaneseRe = regexp.MustCompile(`[\x{3040}-\x{309F}\x{30A0}-\x{30FF}]`)

// Phonemize converts text to IPA phonemes using the appropriate language backend.
func (d *G2PDispatcher) Phonemize(text, lang string) string {
	switch lang {
	case "zh-CN":
		return d.zh.Phonemize(text)
	case "ja-JP":
		return d.ja.Phonemize(text)
	default:
		// For English and other languages, handle mixed content
		return d.phonemizeMixed(text, lang)
	}
}

// phonemizeMixed handles text that may contain mixed languages.
func (d *G2PDispatcher) phonemizeMixed(text, lang string) string {
	// Check if text contains CJK characters
	hasCJK := false
	for _, r := range text {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			hasCJK = true
			break
		}
	}
	if !hasCJK {
		return d.en.Phonemize(text)
	}

	// Split into segments by script
	var parts []string
	var cur strings.Builder
	var curIsLatin bool
	first := true

	for _, r := range text {
		isLatin := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '\'' || r == '-'
		if first {
			curIsLatin = isLatin
			first = false
		}
		if isLatin != curIsLatin && cur.Len() > 0 {
			seg := strings.TrimSpace(cur.String())
			if seg != "" {
				if curIsLatin {
					parts = append(parts, d.en.Phonemize(seg))
				} else if japaneseRe.MatchString(seg) {
					parts = append(parts, d.ja.Phonemize(seg))
				} else {
					parts = append(parts, d.zh.Phonemize(seg))
				}
			}
			cur.Reset()
			curIsLatin = isLatin
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		seg := strings.TrimSpace(cur.String())
		if seg != "" {
			if curIsLatin {
				parts = append(parts, d.en.Phonemize(seg))
			} else if japaneseRe.MatchString(seg) {
				parts = append(parts, d.ja.Phonemize(seg))
			} else {
				parts = append(parts, d.zh.Phonemize(seg))
			}
		}
	}

	return strings.Join(parts, " ")
}

// mapPunctuation converts CJK punctuation to ASCII equivalents.
func mapPunctuation(text string) string {
	r := strings.NewReplacer(
		"\u3002", ".", // 。
		"\uff0c", ",", // ，
		"\uff01", "!", // ！
		"\uff1f", "?", // ？
		"\uff1b", ";", // ；
		"\uff1a", ":", // ：
		"\u201c", "\"", // "
		"\u201d", "\"", // "
		"\u2018", "'", // '
		"\u2019", "'", // '
		"\u3001", ",", // 、
		"\u2014", "—", // —
		"\u2026", "…", // …
		"\uff08", "(", // （
		"\uff09", ")", // ）
	)
	return r.Replace(text)
}
