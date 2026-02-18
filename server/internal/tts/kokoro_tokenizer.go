package tts

import (
	"strings"
	"unicode"
)

// KokoroTokenizer tokenizes text for Kokoro ONNX model
type KokoroTokenizer struct {
	// Kokoro uses phoneme-based tokenization
	// Token IDs: 0-127 for ASCII, 128+ for special tokens
	vocab map[rune]int64
}

// NewKokoroTokenizer creates a new tokenizer
func NewKokoroTokenizer() *KokoroTokenizer {
	return &KokoroTokenizer{
		vocab: buildVocab(),
	}
}

// Tokenize converts text to token IDs
func (t *KokoroTokenizer) Tokenize(text string) []int64 {
	// Normalize text
	text = strings.ToLower(text)
	text = strings.TrimSpace(text)

	tokens := []int64{}
	for _, r := range text {
		if id, ok := t.vocab[r]; ok {
			tokens = append(tokens, id)
		} else if unicode.IsSpace(r) {
			tokens = append(tokens, 32) // space token
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			// Fallback: use ASCII value
			tokens = append(tokens, int64(r))
		}
	}

	// Add EOS token
	tokens = append(tokens, 0) // end-of-sequence

	return tokens
}

// GetVoiceID returns voice embedding ID for Kokoro
func (t *KokoroTokenizer) GetVoiceID(voiceName string) int64 {
	// af_heart is the only supported voice
	if voiceName == "af_heart" {
		return 0
	}
	return 0 // default to af_heart
}

// buildVocab builds character to token ID mapping
func buildVocab() map[rune]int64 {
	vocab := make(map[rune]int64)

	// ASCII characters
	for i := 32; i < 127; i++ {
		vocab[rune(i)] = int64(i)
	}

	// Common punctuation
	vocab['!'] = 33
	vocab['"'] = 34
	vocab['#'] = 35
	vocab['$'] = 36
	vocab['%'] = 37
	vocab['&'] = 38
	vocab['\''] = 39
	vocab['('] = 40
	vocab[')'] = 41
	vocab['*'] = 42
	vocab['+'] = 43
	vocab[','] = 44
	vocab['-'] = 45
	vocab['.'] = 46
	vocab['/'] = 47

	return vocab
}
