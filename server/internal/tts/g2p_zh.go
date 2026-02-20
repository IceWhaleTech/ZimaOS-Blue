//go:build kokoro

package tts

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/mozillazg/go-pinyin"
)

// ChineseG2P converts Chinese text to Zhuyin (Bopomofo) phonemes for Kokoro v1.1-zh.
// Pipeline: text → go-pinyin (initials+finals+tone3) → ZH_MAP lookup → Zhuyin string.
// Matches misaki's ZHFrontend output format.
type ChineseG2P struct {
	pinyinArgs pinyin.Args
	enFallback *EnglishG2P
}

// NewChineseG2P creates a Chinese G2P with English fallback for mixed text.
func NewChineseG2P(en *EnglishG2P) *ChineseG2P {
	a := pinyin.NewArgs()
	a.Style = pinyin.Tone3
	a.Heteronym = false
	return &ChineseG2P{pinyinArgs: a, enFallback: en}
}

var zhEnSplitRe = regexp.MustCompile(`([A-Za-z][A-Za-z' -]*[A-Za-z]|[A-Za-z])`)

// Phonemize converts Chinese text (possibly mixed with English) to Zhuyin phonemes.
func (g *ChineseG2P) Phonemize(text string) string {
	indices := zhEnSplitRe.FindAllStringIndex(text, -1)
	if len(indices) == 0 {
		return g.phonemizeChinese(text)
	}

	var parts []string
	prev := 0
	for _, idx := range indices {
		if idx[0] > prev {
			zh := strings.TrimSpace(text[prev:idx[0]])
			if zh != "" {
				parts = append(parts, g.phonemizeChinese(zh))
			}
		}
		en := strings.TrimSpace(text[idx[0]:idx[1]])
		if en != "" && g.enFallback != nil {
			parts = append(parts, g.enFallback.Phonemize(en))
		}
		prev = idx[1]
	}
	if prev < len(text) {
		zh := strings.TrimSpace(text[prev:])
		if zh != "" {
			parts = append(parts, g.phonemizeChinese(zh))
		}
	}
	return strings.Join(parts, " ")
}

// zhPunctMap maps Chinese punctuation to ASCII equivalents.
var zhPunctMap = map[rune]string{
	'。': ". ", '．': ". ", '！': "! ", '？': "? ",
	'，': ", ", '、': ", ", '；': "; ", '：': ": ",
	'—': "— ", '…': "… ", '～': "— ",
	'《': " \"", '》': "\" ", '「': " \"", '」': "\" ",
	'【': " \"", '】': "\" ", '（': " (", '）': ") ",
	'\u201c': "\"", '\u201d': "\"", '\u2018': "'", '\u2019': "'",
	'«': " \"", '»': "\" ",
}

// phonemizeChinese converts pure Chinese text to Zhuyin phonemes.
// Uses go-pinyin for initials/finals, then maps through ZH_MAP to Zhuyin characters.
// Word boundaries are marked with '/' between words (every 2 syllables as approximation).
func (g *ChineseG2P) phonemizeChinese(text string) string {
	var result strings.Builder
	var hanBuf []rune

	flushHan := func() {
		if len(hanBuf) == 0 {
			return
		}
		segment := string(hanBuf)
		chars := make([]rune, len(hanBuf))
		copy(chars, hanBuf)
		hanBuf = hanBuf[:0]

		// Get initials and finals separately (matching ZHFrontend._get_initials_finals)
		initArgs := pinyin.NewArgs()
		initArgs.Style = pinyin.Initials
		finalArgs := pinyin.NewArgs()
		finalArgs.Style = pinyin.FinalsTone3

		initials := pinyin.Pinyin(segment, initArgs)
		finals := pinyin.Pinyin(segment, finalArgs)

		var syllables []zhSyllable
		for i := 0; i < len(initials) && i < len(finals); i++ {
			init := ""
			if len(initials[i]) > 0 {
				init = initials[i][0]
			}
			fin := ""
			if len(finals[i]) > 0 {
				fin = finals[i][0]
			}
			// Handle 嗯: when initial and final are both empty-ish
			if fin == "" && init == "" {
				continue
			}
			// Discriminate i variants: zi/ci/si → ii, zhi/chi/shi/ri → iii
			fin = discriminateI(init, fin)
			tone, base := extractTone(fin)
			syllables = append(syllables, zhSyllable{
				initial: init,
				final:   base,
				tone:    tone,
			})
		}
		applyToneSandhi(syllables)
		applyErhua(chars, syllables)

		// Convert to Zhuyin and emit with word boundaries
		emitted := 0
		for _, s := range syllables {
			if s.silent {
				continue
			}
			if emitted > 0 && emitted%2 == 0 {
				result.WriteRune('/')
			}
			result.WriteString(syllableToZhuyin(s))
			emitted++
		}
	}

	for _, r := range text {
		if isHanChar(r) {
			hanBuf = append(hanBuf, r)
			continue
		}
		flushHan()

		if mapped, ok := zhPunctMap[r]; ok {
			result.WriteString(mapped)
		} else if r == ' ' || r == '\t' {
			result.WriteRune(' ')
		} else if (r >= '0' && r <= '9') || r == '.' || r == ',' || r == '!' || r == '?' || r == ';' || r == ':' || r == '"' || r == '(' || r == ')' || r == '/' {
			result.WriteRune(r)
		}
	}
	flushHan()

	return strings.TrimSpace(result.String())
}

// discriminateI handles special i finals: zi/ci/si → ii, zhi/chi/shi/ri → iii
func discriminateI(initial, final string) string {
	if len(final) == 0 {
		return final
	}
	// Check if final starts with 'i' followed by a tone digit
	if final[0] != 'i' {
		return final
	}
	rest := final[1:]
	if len(rest) == 0 || (rest[0] >= '1' && rest[0] <= '5') {
		// final is "i" or "i[1-5]"
		switch initial {
		case "z", "c", "s":
			return "ii" + rest
		case "zh", "ch", "sh", "r":
			return "iii" + rest
		}
	}
	return final
}

// syllableToZhuyin converts a syllable to Zhuyin string using ZH_MAP.
func syllableToZhuyin(s zhSyllable) string {
	var b strings.Builder

	// Map initial
	if s.initial != "" {
		if mapped, ok := zhMap[s.initial]; ok {
			b.WriteString(mapped)
		}
	}

	// Map final — may need splitting for compound finals
	if s.final != "" {
		if mapped, ok := zhMap[s.final]; ok {
			b.WriteString(mapped)
		} else {
			// Fallback: try character by character (shouldn't happen with complete map)
			b.WriteString(s.final)
		}
	}

	// Erhua marker
	if s.erhua {
		b.WriteRune('R')
	}

	// Tone (as digit character, matching v1.1-zh tokenizer)
	if s.tone >= 1 && s.tone <= 4 {
		b.WriteByte(byte('0' + s.tone))
	}
	// tone 5 (neutral) = no marker

	return b.String()
}

// extractTone extracts tone number (1-5) and base final from FinalsTone3 format.
func extractTone(py string) (int, string) {
	if len(py) == 0 {
		return 5, py
	}
	last := py[len(py)-1]
	if last >= '1' && last <= '5' {
		return int(last - '0'), py[:len(py)-1]
	}
	return 5, py // neutral tone
}

// applyToneSandhi applies Mandarin tone sandhi rules.
func applyToneSandhi(syllables []zhSyllable) {
	for i := 0; i < len(syllables)-1; i++ {
		// Third tone sandhi: 3+3 → 2+3
		if syllables[i].tone == 3 && syllables[i+1].tone == 3 {
			syllables[i].tone = 2
		}
	}
	// 一 (yi) sandhi
	for i, s := range syllables {
		if s.initial != "" || s.final != "i" {
			continue
		}
		// Check if this is actually yi (no initial, final=i)
		if i+1 < len(syllables) {
			if syllables[i+1].tone == 4 {
				syllables[i].tone = 2
			} else if syllables[i+1].tone >= 1 && syllables[i+1].tone <= 3 {
				syllables[i].tone = 4
			}
		}
	}
	// 不 (bu) sandhi
	for i, s := range syllables {
		if s.initial != "b" || s.final != "u" {
			continue
		}
		if i+1 < len(syllables) && syllables[i+1].tone == 4 {
			syllables[i].tone = 2
		}
	}
}

type zhSyllable struct {
	initial string
	final   string
	tone    int
	erhua   bool
	silent  bool
}

// notErhua is a set of words where 儿 is a standalone character, not a suffix.
var notErhua = map[string]bool{
	"女儿": true, "儿子": true, "儿童": true, "幼儿": true, "少儿": true,
	"婴儿": true, "孤儿": true, "男儿": true, "侄儿": true, "孙儿": true,
	"患儿": true, "婴幼儿": true, "混血儿": true,
}

// applyErhua handles 儿化 (erhua/rhotacization) in Chinese.
func applyErhua(chars []rune, syllables []zhSyllable) {
	if len(chars) != len(syllables) || len(syllables) < 2 {
		return
	}
	for i := len(syllables) - 1; i >= 1; i-- {
		if chars[i] != '儿' {
			continue
		}
		if syllables[i].initial != "" || syllables[i].final != "er" {
			continue
		}
		skip := false
		for wlen := 2; wlen <= 3 && i-wlen+1 >= 0; wlen++ {
			word := string(chars[i-wlen+1 : i+1])
			if notErhua[word] {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		syllables[i-1].erhua = true
		syllables[i].silent = true
	}
}

// zhMap maps pinyin initials and finals to Zhuyin characters.
// Matches misaki's ZH_MAP from zh_frontend.py exactly.
var zhMap = map[string]string{
	// Initials
	"b": "ㄅ", "p": "ㄆ", "m": "ㄇ", "f": "ㄈ",
	"d": "ㄉ", "t": "ㄊ", "n": "ㄋ", "l": "ㄌ",
	"g": "ㄍ", "k": "ㄎ", "h": "ㄏ",
	"j": "ㄐ", "q": "ㄑ", "x": "ㄒ",
	"zh": "ㄓ", "ch": "ㄔ", "sh": "ㄕ", "r": "ㄖ",
	"z": "ㄗ", "c": "ㄘ", "s": "ㄙ",
	// Simple finals
	"a": "ㄚ", "o": "ㄛ", "e": "ㄜ",
	"i": "ㄧ", "u": "ㄨ", "v": "ㄩ",
	// Compound finals
	"ie": "ㄝ", "ai": "ㄞ", "ei": "ㄟ", "ao": "ㄠ", "ou": "ㄡ",
	"an": "ㄢ", "en": "ㄣ", "ang": "ㄤ", "eng": "ㄥ", "er": "ㄦ",
	// Special finals
	"ii":   "ㄭ",  // zi/ci/si
	"iii":  "十",   // zhi/chi/shi/ri
	"ve":   "月",   // üe
	"ia":   "压",
	"ian":  "言",
	"iang": "阳",
	"iao":  "要",
	"in":   "阴",
	"ing":  "应",
	"iong": "用",
	"iou":  "又",
	"ong":  "中",
	"ua":   "穵",
	"uai":  "外",
	"uan":  "万",
	"uang": "王",
	"uei":  "为",
	"uen":  "文",
	"ueng": "瓮",
	"uo":   "我",
	"van":  "元",
	"vn":   "云",
}

// isHanChar checks if a rune is a CJK Unified Ideograph.
func isHanChar(r rune) bool {
	return unicode.Is(unicode.Han, r)
}
