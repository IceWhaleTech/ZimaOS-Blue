//go:build kokoro

package tts

import (
	"embed"
	"encoding/json"
	"strings"
	"sync"
	"unicode"
)

//go:embed g2pdata/us_gold.json
var usGoldFS embed.FS

// EnglishG2P converts English text to IPA phonemes using dictionary lookup.
type EnglishG2P struct {
	dict     map[string]interface{} // word -> string or map[string]string
	initOnce sync.Once
}

// NewEnglishG2P creates an English G2P with embedded dictionary.
func NewEnglishG2P() *EnglishG2P {
	return &EnglishG2P{}
}

func (g *EnglishG2P) init() {
	g.initOnce.Do(func() {
		data, err := usGoldFS.ReadFile("g2pdata/us_gold.json")
		if err != nil {
			g.dict = make(map[string]interface{})
			return
		}
		g.dict = make(map[string]interface{}, 100000)
		json.Unmarshal(data, &g.dict)
		// Generate case variants
		growDictionary(g.dict)
	})
}

// growDictionary adds case variants (lowercase ↔ capitalized).
func growDictionary(d map[string]interface{}) {
	additions := make(map[string]interface{})
	for k, v := range d {
		if len(k) == 0 {
			continue
		}
		first := rune(k[0])
		if unicode.IsLower(first) {
			cap := strings.ToUpper(k[:1]) + k[1:]
			if _, exists := d[cap]; !exists {
				additions[cap] = v
			}
		} else if unicode.IsUpper(first) {
			low := strings.ToLower(k[:1]) + k[1:]
			if _, exists := d[low]; !exists {
				additions[low] = v
			}
		}
	}
	for k, v := range additions {
		d[k] = v
	}
}

// Phonemize converts English text to IPA phonemes.
func (g *EnglishG2P) Phonemize(text string) string {
	g.init()

	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	words := tokenizeEnglish(text)
	var parts []string
	for _, w := range words {
		if w == " " {
			parts = append(parts, " ")
			continue
		}
		// Punctuation passthrough
		if len(w) == 1 && !unicode.IsLetter(rune(w[0])) {
			parts = append(parts, w)
			continue
		}
		ph := g.lookupWord(w)
		if ph != "" {
			parts = append(parts, ph)
		}
	}
	return strings.Join(parts, "")
}

// lookupWord looks up a word in the dictionary with fallback cascade.
func (g *EnglishG2P) lookupWord(word string) string {
	// Direct lookup
	if ph := g.dictLookup(word); ph != "" {
		return ph
	}
	// Lowercase lookup
	low := strings.ToLower(word)
	if ph := g.dictLookup(low); ph != "" {
		return ph
	}
	// Morphological stemming: -s
	if ph := g.tryStemS(low); ph != "" {
		return ph
	}
	// Morphological stemming: -ed
	if ph := g.tryStemEd(low); ph != "" {
		return ph
	}
	// Morphological stemming: -ing
	if ph := g.tryStemIng(low); ph != "" {
		return ph
	}
	// eSpeak fallback (build-tag dependent)
	if ph := espeakFallback(word); ph != "" {
		return ph
	}
	// Last resort: return lowercased (tokenizer will map valid IPA chars)
	return low
}

// dictLookup looks up a word in the gold dictionary.
func (g *EnglishG2P) dictLookup(word string) string {
	v, ok := g.dict[word]
	if !ok {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case map[string]interface{}:
		// POS-tagged entry: use DEFAULT
		if def, ok := val["DEFAULT"]; ok {
			if s, ok := def.(string); ok {
				return s
			}
		}
	}
	return ""
}

// tryStemS tries to find the stem of a word ending in -s/-es/-ies.
func (g *EnglishG2P) tryStemS(word string) string {
	if !strings.HasSuffix(word, "s") || len(word) < 3 {
		return ""
	}
	// Try -ies → -y
	if strings.HasSuffix(word, "ies") {
		stem := word[:len(word)-3] + "y"
		if ph := g.dictLookup(stem); ph != "" {
			return ph + suffixS(ph)
		}
	}
	// Try -es
	if strings.HasSuffix(word, "es") {
		stem := word[:len(word)-2]
		if ph := g.dictLookup(stem); ph != "" {
			return ph + suffixS(ph)
		}
	}
	// Try -s
	stem := word[:len(word)-1]
	if ph := g.dictLookup(stem); ph != "" {
		return ph + suffixS(ph)
	}
	return ""
}

// tryStemEd tries to find the stem of a word ending in -ed/-d.
func (g *EnglishG2P) tryStemEd(word string) string {
	if !strings.HasSuffix(word, "ed") || len(word) < 4 {
		return ""
	}
	// Try -ed
	stem := word[:len(word)-2]
	if ph := g.dictLookup(stem); ph != "" {
		return ph + suffixEd(ph)
	}
	// Try -d (e.g., "baked" → "bake")
	stem = word[:len(word)-1]
	if ph := g.dictLookup(stem); ph != "" {
		return ph + suffixEd(ph)
	}
	return ""
}

// tryStemIng tries to find the stem of a word ending in -ing.
func (g *EnglishG2P) tryStemIng(word string) string {
	if !strings.HasSuffix(word, "ing") || len(word) < 5 {
		return ""
	}
	stem := word[:len(word)-3]
	// Try stem directly (e.g., "running" → "runn" → "run" with double consonant)
	if ph := g.dictLookup(stem); ph != "" {
		return ph + "ɪŋ"
	}
	// Try stem + e (e.g., "making" → "make")
	if ph := g.dictLookup(stem + "e"); ph != "" {
		return ph + "ɪŋ"
	}
	// Try removing double consonant (e.g., "running" → "run")
	if len(stem) >= 2 && stem[len(stem)-1] == stem[len(stem)-2] {
		if ph := g.dictLookup(stem[:len(stem)-1]); ph != "" {
			return ph + "ɪŋ"
		}
	}
	return ""
}

// suffixS returns the phoneme suffix for plural/3rd person -s.
func suffixS(stem string) string {
	if len(stem) == 0 {
		return "z"
	}
	last := rune(stem[len(stem)-1])
	switch last {
	case 'p', 't', 'k', 'f', 'θ':
		return "s"
	case 's', 'z', 'ʃ', 'ʒ', 'ʧ', 'ʤ':
		return "ᵻz"
	default:
		return "z"
	}
}

// suffixEd returns the phoneme suffix for past tense -ed.
func suffixEd(stem string) string {
	if len(stem) == 0 {
		return "d"
	}
	last := rune(stem[len(stem)-1])
	switch last {
	case 'p', 'k', 'f', 'θ', 'ʃ', 's', 'ʧ':
		return "t"
	case 'd', 't':
		return "ᵻd"
	default:
		return "d"
	}
}

// tokenizeEnglish splits text into words and punctuation tokens.
func tokenizeEnglish(text string) []string {
	var tokens []string
	var cur strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || r == '\'' || r == '-' {
			cur.WriteRune(r)
		} else {
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
			if unicode.IsSpace(r) {
				tokens = append(tokens, " ")
			} else {
				tokens = append(tokens, string(r))
			}
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}
