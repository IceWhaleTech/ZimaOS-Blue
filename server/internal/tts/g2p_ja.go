package tts

import (
	"strings"
	"unicode/utf8"

	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"
)

// JapaneseG2P converts Japanese text to IPA phonemes for Kokoro.
// Pipeline: text → kagome morphology → katakana reading → M2P table → IPA.
type JapaneseG2P struct {
	tok *tokenizer.Tokenizer
}

// NewJapaneseG2P creates a Japanese G2P using kagome IPA dictionary.
func NewJapaneseG2P() *JapaneseG2P {
	t, err := tokenizer.New(ipa.Dict(), tokenizer.OmitBosEos())
	if err != nil {
		return &JapaneseG2P{}
	}
	return &JapaneseG2P{tok: t}
}

// Phonemize converts Japanese text to IPA phonemes.
func (g *JapaneseG2P) Phonemize(text string) string {
	text = mapPunctuation(text)

	if g.tok == nil {
		return text
	}

	tokens := g.tok.Tokenize(text)
	var parts []string
	for _, t := range tokens {
		if t.Class == tokenizer.DUMMY {
			continue
		}
		// Get katakana reading from morphological analysis
		features := t.Features()
		reading := ""
		if len(features) >= 8 {
			reading = features[7] // Reading field in IPA dict
		}
		if reading == "" || reading == "*" {
			// No reading available — use surface form
			reading = t.Surface
		}

		ipa := katakanaToIPA(reading)
		if ipa != "" {
			parts = append(parts, ipa)
		}
	}
	return strings.Join(parts, "")
}

// katakanaToIPA converts a katakana string to IPA using the M2P table.
func katakanaToIPA(kana string) string {
	var result strings.Builder
	runes := []rune(kana)
	i := 0
	for i < len(runes) {
		// Try two-character digraph first
		if i+1 < len(runes) {
			digraph := string(runes[i : i+2])
			if ipa, ok := jaM2P[digraph]; ok {
				result.WriteString(ipa)
				i += 2
				continue
			}
		}
		// Single character
		ch := string(runes[i])
		if ipa, ok := jaM2P[ch]; ok {
			result.WriteString(ipa)
		} else {
			// Passthrough for punctuation and unknown chars
			r := runes[i]
			if r == ' ' || r == '、' || r == '。' {
				result.WriteRune(' ')
			} else if r < 0x3000 { // ASCII/Latin
				result.WriteRune(r)
			}
			// Skip unknown katakana/hiragana
		}
		i++
	}
	return result.String()
}

// applyNasalContext applies context-dependent ン rules.
// This is called as a post-processing step on the IPA output.
func applyNasalContext(ipa string) string {
	runes := []rune(ipa)
	var result strings.Builder
	for i := 0; i < len(runes); i++ {
		if runes[i] == 'ɴ' && i+1 < len(runes) {
			next := runes[i+1]
			switch {
			case next == 'm' || next == 'p' || next == 'b':
				result.WriteRune('m')
			case next == 'k' || next == 'ɡ':
				result.WriteRune('ŋ')
			case next == 'ɲ' || next == 'ʨ' || next == 'ʥ':
				result.WriteRune('ɲ')
			case next == 'n' || next == 't' || next == 'd' || next == 'ɾ' || next == 'z':
				result.WriteRune('n')
			default:
				result.WriteRune('ɴ')
			}
		} else {
			result.WriteRune(runes[i])
		}
	}
	return result.String()
}

// Ensure utf8 import is used
var _ = utf8.RuneLen

// jaM2P maps katakana characters/digraphs to IPA phonemes (Kokoro M2P table).
var jaM2P = map[string]string{
	// Single katakana (vowels)
	"ア": "a", "イ": "i", "ウ": "u", "エ": "e", "オ": "o",
	"ァ": "a", "ィ": "i", "ゥ": "u", "ェ": "e", "ォ": "o",
	// Ka-row
	"カ": "ka", "キ": "ki", "ク": "ku", "ケ": "ke", "コ": "ko",
	"ガ": "ga", "ギ": "gi", "グ": "gu", "ゲ": "ge", "ゴ": "go",
	// Sa-row
	"サ": "sa", "シ": "ɕi", "ス": "su", "セ": "se", "ソ": "so",
	"ザ": "za", "ジ": "ʥi", "ズ": "zu", "ゼ": "ze", "ゾ": "zo",
	// Ta-row
	"タ": "ta", "チ": "ʨi", "ツ": "ʦu", "テ": "te", "ト": "to",
	"ダ": "da", "ヂ": "ʥi", "ヅ": "zu", "デ": "de", "ド": "do",
	// Na-row
	"ナ": "na", "ニ": "ni", "ヌ": "nu", "ネ": "ne", "ノ": "no",
	// Ha-row
	"ハ": "ha", "ヒ": "hi", "フ": "fu", "ヘ": "he", "ホ": "ho",
	"バ": "ba", "ビ": "bi", "ブ": "bu", "ベ": "be", "ボ": "bo",
	"パ": "pa", "ピ": "pi", "プ": "pu", "ペ": "pe", "ポ": "po",
	// Ma-row
	"マ": "ma", "ミ": "mi", "ム": "mu", "メ": "me", "モ": "mo",
	// Ya-row
	"ヤ": "ja", "ユ": "ju", "ヨ": "jo",
	"ャ": "ja", "ュ": "ju", "ョ": "jo",
	// Ra-row
	"ラ": "ra", "リ": "ri", "ル": "ru", "レ": "re", "ロ": "ro",
	// Wa-row
	"ワ": "wa", "ヲ": "o", "ヰ": "i", "ヱ": "e",
	// Special
	"ン": "ɴ", "ッ": "ʔ", "ー": "ː",
	"ヴ": "vu",
	// Digraphs: Ki-row
	"キャ": "ᶄa", "キュ": "ᶄu", "キョ": "ᶄo", "キィ": "ᶄi", "キェ": "ᶄe",
	"ギャ": "ᶃa", "ギュ": "ᶃu", "ギョ": "ᶃo", "ギィ": "ᶃi", "ギェ": "ᶃe",
	// Shi-row
	"シャ": "ɕa", "シュ": "ɕu", "ショ": "ɕo", "シェ": "ɕe",
	"ジャ": "ʥa", "ジュ": "ʥu", "ジョ": "ʥo", "ジェ": "ʥe",
	// Chi-row
	"チャ": "ʨa", "チュ": "ʨu", "チョ": "ʨo", "チェ": "ʨe",
	// Ni-row
	"ニャ": "ɲa", "ニュ": "ɲu", "ニョ": "ɲo", "ニィ": "ɲi", "ニェ": "ɲe",
	// Hi-row
	"ヒャ": "ça", "ヒュ": "çu", "ヒョ": "ço", "ヒィ": "çi", "ヒェ": "çe",
	"ビャ": "ᶀa", "ビュ": "ᶀu", "ビョ": "ᶀo",
	"ピャ": "ᶈa", "ピュ": "ᶈu", "ピョ": "ᶈo",
	// Mi-row
	"ミャ": "ᶆa", "ミュ": "ᶆu", "ミョ": "ᶆo",
	// Ri-row
	"リャ": "ᶉa", "リュ": "ᶉu", "リョ": "ᶉo",
	// Foreign sounds
	"ファ": "fa", "フィ": "fi", "フェ": "fe", "フォ": "fo",
	"ティ": "ti", "ディ": "di", "トゥ": "tu", "ドゥ": "du",
	"ウィ": "wi", "ウェ": "we", "ウォ": "wo", "ウゥ": "wu",
	"イェ": "je",
	"ツァ": "ʦa", "ツィ": "ʦi", "ツェ": "ʦe", "ツォ": "ʦo",
}
