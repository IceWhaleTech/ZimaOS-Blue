//go:build kokoro

package tts

// KokoroTokenizer tokenizes phoneme text for Kokoro ONNX model.
// The v1.1-zh model vocabulary has 178 entries (sparse IDs 0-177) mapping
// IPA symbols (for English) and Zhuyin/Chinese characters (for Chinese) to token IDs.
// Input text must already be in phoneme form; non-vocab characters are silently dropped.
type KokoroTokenizer struct {
	vocab map[rune]int64
}

// NewKokoroTokenizer creates a tokenizer with the Kokoro v1.1-zh phoneme vocabulary.
func NewKokoroTokenizer() *KokoroTokenizer {
	return &KokoroTokenizer{vocab: kokoroVocab()}
}

// Tokenize converts phoneme text to token IDs, wrapped with PAD token (0) at both ends.
func (t *KokoroTokenizer) Tokenize(text string) []int64 {
	tokens := make([]int64, 0, len(text)+2)
	tokens = append(tokens, 0) // $ start
	for _, r := range text {
		if id, ok := t.vocab[r]; ok {
			tokens = append(tokens, id)
		}
		// Characters not in vocab are silently dropped
	}
	tokens = append(tokens, 0) // $ end
	return tokens
}

// kokoroVocab returns the phoneme-to-token-ID mapping from Kokoro v1.1-zh config.json.
// This vocab supports both IPA (for English) and Zhuyin/Chinese characters (for Chinese).
// Sparse IDs: not all values 0-177 are used.
func kokoroVocab() map[rune]int64 {
	return map[rune]int64{
		'$': 0, // PAD / start-end
		// Punctuation (shared)
		';': 1, ':': 2, ',': 3, '.': 4, '!': 5, '?': 6,
		'/':      7,  // word boundary (used in Chinese)
		'—':      9,  // em dash U+2014
		'…':      10, // ellipsis U+2026
		'"':      11,
		'(':      12,
		')':      13,
		'\u201c': 14, // left curly quote
		'\u201d': 15, // right curly quote
		' ':      16, // space / word boundary

		// IPA modifiers (shared)
		'\u0303': 17, // combining tilde (nasalization)
		'ʣ':      18, // dz affricate U+02A3
		'ʥ':      19, // dʑ affricate U+02A5
		'ʦ':      20, // ts affricate U+02A6
		'ʨ':      21, // tɕ affricate U+02A8
		'ᵝ':      22, // bilabial approximant U+1D5D

		// Zhuyin initials
		'ㄓ': 23, // zh
		'ㄅ': 30, // b
		'ㄆ': 32, // p
		'ㄇ': 37, // m
		'ㄈ': 38, // f
		'ㄉ': 40, // d
		'ㄊ': 49, // t
		'ㄋ': 73, // n
		'ㄌ': 74, // l
		'ㄍ': 79, // g
		'ㄎ': 84, // k
		'ㄏ': 88, // h
		'ㄐ': 89, // j
		'ㄑ': 91, // q
		'ㄒ': 93, // x
		'ㄔ': 94, // ch
		'ㄕ': 95, // sh
		'ㄗ': 96, // z
		'ㄘ': 97, // c
		'ㄙ': 98, // s
		'ㄖ': 126, // r

		// Zhuyin simple finals
		'ㄚ': 100, // a
		'ㄛ': 104, // o
		'ㄜ': 140, // e
		'ㄝ': 105, // ie
		'ㄞ': 106, // ai
		'ㄟ': 107, // ei
		'ㄠ': 108, // ao
		'ㄡ': 109, // ou
		'ㄢ': 117, // an
		'ㄣ': 121, // en
		'ㄤ': 122, // ang
		'ㄥ': 124, // eng
		'ㄦ': 85,  // er
		'ㄧ': 127, // i
		'ㄨ': 134, // u
		'ㄩ': 137, // ü
		'ㄭ': 141, // ii (zi/ci/si)

		// Chinese character finals (compound finals)
		'月': 99,  // üe (ve)
		'十': 144, // iii (zhi/chi/shi/ri)
		'压': 145, // ia
		'言': 146, // ian
		'阳': 149, // iang
		'要': 150, // iao
		'阴': 151, // in
		'应': 152, // ing
		'用': 153, // iong
		'又': 154, // iou
		'中': 155, // ong
		'穵': 159, // ua
		'外': 160, // uai
		'万': 161, // uan
		'王': 163, // uang
		'为': 165, // uei
		'文': 166, // uen
		'瓮': 167, // ueng
		'我': 168, // uo
		'元': 175, // üan (van)
		'云': 176, // ün (vn)

		// Tone digits (Chinese)
		'1': 171, // tone 1 (high level)
		'2': 172, // tone 2 (rising)
		'3': 169, // tone 3 (dipping)
		'4': 173, // tone 4 (falling)
		'5': 170, // tone 5 (neutral)
		'R': 34,  // erhua marker

		// English IPA letters
		'A': 24, 'I': 25, 'O': 31, 'Q': 33, 'S': 35, 'T': 36,
		'W': 39, 'Y': 41,
		'ᵊ': 42, // muted schwa U+1D4A
		'a': 43, 'b': 44, 'c': 45, 'd': 46, 'e': 47, 'f': 48,
		'h': 50, 'i': 51, 'j': 52, 'k': 53, 'l': 54, 'm': 55,
		'n': 56, 'o': 57, 'p': 58, 'q': 59, 'r': 60, 's': 61,
		't': 62, 'u': 63, 'v': 64, 'w': 65, 'x': 66, 'y': 67,
		'z': 68,

		// IPA vowels and consonants (for English)
		'ɑ': 69,  // open back unrounded U+0251
		'ɐ': 70,  // near-open central U+0250
		'ɒ': 71,  // open back rounded U+0252
		'æ': 72,  // near-open front U+00E6
		'β': 75,  // voiced bilabial fricative U+03B2
		'ɔ': 76,  // open-mid back rounded U+0254
		'ɕ': 77,  // voiceless alveolopalatal U+0255
		'ç': 78,  // voiceless palatal fricative U+00E7
		'ɖ': 80,  // voiced retroflex stop U+0256
		'ð': 81,  // voiced dental fricative U+00F0
		'ʤ': 82,  // voiced postalveolar affricate U+02A4
		'ə': 83,  // schwa U+0259
		'ɛ': 86,  // open-mid front U+025B
		'ɜ': 87,  // open-mid central U+025C
		'ɟ': 90,  // voiced palatal stop U+025F
		'ɡ': 92,  // voiced velar stop U+0261
		'ɥ': 99,  // labial-palatal approximant U+0265 (shares ID with 月)
		'ɨ': 101, // close central unrounded U+0268
		'ɪ': 102, // near-close near-front U+026A
		'ʝ': 103, // voiced palatal fricative U+029D
		'ɯ': 110, // close back unrounded U+026F
		'ɰ': 111, // velar approximant U+0270
		'ŋ': 112, // velar nasal U+014B
		'ɳ': 113, // retroflex nasal U+0273
		'ɲ': 114, // palatal nasal U+0272
		'ɴ': 115, // uvular nasal U+0274
		'ø': 116, // close-mid front rounded U+00F8
		'ɸ': 118, // voiceless bilabial fricative U+0278
		'θ': 119, // voiceless dental fricative U+03B8
		'œ': 120, // open-mid front rounded U+0153
		'ɹ': 123, // alveolar approximant U+0279
		'ɾ': 125, // alveolar flap U+027E
		'ɻ': 126, // retroflex approximant U+027B (shares ID with ㄖ)
		'ʁ': 128, // voiced uvular fricative U+0281
		'ɽ': 129, // retroflex flap U+027D
		'ʂ': 130, // voiceless retroflex fricative U+0282
		'ʃ': 131, // voiceless postalveolar U+0283
		'ʈ': 132, // voiceless retroflex stop U+0288
		'ʧ': 133, // voiceless postalveolar affricate U+02A7
		'ʊ': 135, // near-close near-back U+028A
		'ʋ': 136, // labiodental approximant U+028B
		'ʌ': 138, // open-mid back unrounded U+028C
		'ɣ': 139, // voiced velar fricative U+0263
		'ɤ': 140, // close-mid back unrounded U+0264 (shares ID with ㄜ)
		'χ': 142, // voiceless uvular fricative U+03C7
		'ʎ': 143, // palatal lateral U+028E
		'ʒ': 147, // voiced postalveolar U+0292
		'ʔ': 148, // glottal stop U+0294
		'ˈ': 156, // primary stress U+02C8
		'ˌ': 157, // secondary stress U+02CC
		'ː': 158, // length mark U+02D0
		'ʰ': 162, // aspiration U+02B0
		'ʲ': 164, // palatalization U+02B2
		// Tone arrows (for English IPA, mapped to same IDs as tone digits)
		'↓': 169, // downstep/falling tone U+2193 = tone 3
		'→': 171, // level tone U+2192 = tone 1
		'↗': 172, // rising tone U+2197 = tone 2
		'↘': 173, // falling tone U+2198 = tone 4
		'ᵻ': 177, // near-close central U+1D7B
	}
}
