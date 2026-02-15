package pruner

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode/utf8"
)

// BPETokenizer implements byte-level BPE tokenization compatible with HuggingFace tokenizers.
// It loads vocab.json + merges.txt (Qwen3/GPT-style BPE).
type BPETokenizer struct {
	vocab    map[string]int    // token string → token id
	ivocab   map[int]string    // token id → token string
	merges   []mergePair       // ordered merge rules
	mergeMap map[mergePair]int // merge pair → priority (lower = higher priority)

	// Special token IDs
	padID int
	eosID int
	bosID int
}

type mergePair struct {
	a, b string
}

// LoadBPETokenizer loads a BPE tokenizer from vocab.json and merges.txt files.
func LoadBPETokenizer(vocabPath, mergesPath string) (*BPETokenizer, error) {
	vocabData, err := os.ReadFile(vocabPath)
	if err != nil {
		return nil, fmt.Errorf("read vocab.json: %w", err)
	}
	var vocab map[string]int
	if err := json.Unmarshal(vocabData, &vocab); err != nil {
		return nil, fmt.Errorf("parse vocab.json: %w", err)
	}

	ivocab := make(map[int]string, len(vocab))
	for k, v := range vocab {
		ivocab[v] = k
	}

	mergesData, err := os.ReadFile(mergesPath)
	if err != nil {
		return nil, fmt.Errorf("read merges.txt: %w", err)
	}

	lines := strings.Split(string(mergesData), "\n")
	var merges []mergePair
	mergeMap := make(map[mergePair]int)

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		mp := mergePair{a: parts[0], b: parts[1]}
		merges = append(merges, mp)
		mergeMap[mp] = i
	}

	t := &BPETokenizer{
		vocab:    vocab,
		ivocab:   ivocab,
		merges:   merges,
		mergeMap: mergeMap,
		padID:    lookupSpecial(vocab, "<|endoftext|>", "<pad>"),
		eosID:    lookupSpecial(vocab, "<|endoftext|>", "<|im_end|>", "</s>"),
		bosID:    lookupSpecial(vocab, "<|im_start|>", "<s>"),
	}

	return t, nil
}

func lookupSpecial(vocab map[string]int, candidates ...string) int {
	for _, c := range candidates {
		if id, ok := vocab[c]; ok {
			return id
		}
	}
	return 0
}

// Encode tokenizes text into token IDs.
func (t *BPETokenizer) Encode(text string) []int {
	if text == "" {
		return nil
	}
	words := t.pretokenize(text)
	var ids []int
	for _, word := range words {
		tokens := t.bpe(word)
		for _, tok := range tokens {
			if id, ok := t.vocab[tok]; ok {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// PadID returns the padding token ID.
func (t *BPETokenizer) PadID() int { return t.padID }

// VocabSize returns the vocabulary size.
func (t *BPETokenizer) VocabSize() int { return len(t.vocab) }

// pretokenize splits text into words for BPE processing using byte-level encoding.
func (t *BPETokenizer) pretokenize(text string) [][]string {
	var result [][]string
	chunks := splitOnWhitespace(text)
	for _, chunk := range chunks {
		var symbols []string
		for i := 0; i < len(chunk); i++ {
			symbols = append(symbols, string(byteToUnicode[chunk[i]]))
		}
		if len(symbols) > 0 {
			result = append(result, symbols)
		}
	}
	return result
}

func splitOnWhitespace(text string) []string {
	var result []string
	var current strings.Builder

	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
			current.WriteRune(r)
		} else {
			current.WriteRune(r)
		}
		i += size
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}

// bpe applies BPE merges to a sequence of symbols.
func (t *BPETokenizer) bpe(symbols []string) []string {
	if len(symbols) <= 1 {
		return symbols
	}

	word := make([]string, len(symbols))
	copy(word, symbols)

	for {
		if len(word) < 2 {
			break
		}
		bestIdx := -1
		bestRank := len(t.merges)

		for i := 0; i < len(word)-1; i++ {
			mp := mergePair{a: word[i], b: word[i+1]}
			if rank, ok := t.mergeMap[mp]; ok && rank < bestRank {
				bestRank = rank
				bestIdx = i
			}
		}
		if bestIdx < 0 {
			break
		}

		merged := word[bestIdx] + word[bestIdx+1]
		newWord := make([]string, 0, len(word)-1)
		newWord = append(newWord, word[:bestIdx]...)
		newWord = append(newWord, merged)
		if bestIdx+2 < len(word) {
			newWord = append(newWord, word[bestIdx+2:]...)
		}
		word = newWord
	}
	return word
}

// BuildPrunerInput constructs input tensors for the pruner model.
// Returns input_ids, attention_mask, and the start/end offsets of code tokens.
func (t *BPETokenizer) BuildPrunerInput(query, code string, maxLength int) (inputIDs, attentionMask []int64, codeStart, codeEnd int) {
	prefix := "<|im_start|>system\nYou are a code relevance evaluator.<|im_end|>\n<|im_start|>user\n"
	suffix := "<|im_end|>\n"

	var queryPart string
	if query != "" {
		queryPart = "Goal: " + query + "\nCode:\n"
	} else {
		queryPart = "Code:\n"
	}

	prefixIDs := t.Encode(prefix + queryPart)
	codeIDs := t.Encode(code)
	suffixIDs := t.Encode(suffix)

	overhead := len(prefixIDs) + len(suffixIDs)
	maxCodeTokens := maxLength - overhead
	if maxCodeTokens < 0 {
		maxCodeTokens = 0
	}
	if len(codeIDs) > maxCodeTokens {
		codeIDs = codeIDs[:maxCodeTokens]
	}

	inputIDs = make([]int64, maxLength)
	attentionMask = make([]int64, maxLength)

	for i := range inputIDs {
		inputIDs[i] = int64(t.padID)
	}

	idx := 0
	for _, id := range prefixIDs {
		inputIDs[idx] = int64(id)
		attentionMask[idx] = 1
		idx++
	}
	codeStart = idx
	for _, id := range codeIDs {
		inputIDs[idx] = int64(id)
		attentionMask[idx] = 1
		idx++
	}
	codeEnd = idx
	for _, id := range suffixIDs {
		if idx >= maxLength {
			break
		}
		inputIDs[idx] = int64(id)
		attentionMask[idx] = 1
		idx++
	}
	return
}

// byteToUnicode maps each byte (0-255) to a printable unicode character.
// Standard GPT-2 / Qwen byte-level BPE encoding.
var byteToUnicode [256]rune

func init() {
	bs := make([]int, 0, 256)
	cs := make([]int, 0, 256)

	for i := int('!'); i <= int('~'); i++ {
		bs = append(bs, i)
		cs = append(cs, i)
	}
	for i := int('¡'); i <= int('¬'); i++ {
		bs = append(bs, i)
		cs = append(cs, i)
	}
	for i := int('®'); i <= int('ÿ'); i++ {
		bs = append(bs, i)
		cs = append(cs, i)
	}

	n := 0
	for b := 0; b < 256; b++ {
		found := false
		for _, existing := range bs {
			if existing == b {
				found = true
				break
			}
		}
		if !found {
			bs = append(bs, b)
			cs = append(cs, 256+n)
			n++
		}
	}

	type pair struct{ b, c int }
	pairs := make([]pair, len(bs))
	for i := range bs {
		pairs[i] = pair{bs[i], cs[i]}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].b < pairs[j].b })

	for _, p := range pairs {
		byteToUnicode[p.b] = rune(p.c)
	}
}
