package smallmodel

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
)

const defaultPreTokenizePattern = `(?i:'s|'t|'re|'ve|'m|'ll|'d)|[^\r\n\p{L}\p{N}]?[\p{L}\p{M}]+|\p{N}| ?[^\s\p{L}\p{M}\p{N}]+[\r\n]*|\s*[\r\n]+|\s+(?!\S)|\s+`

type mergePair struct {
	a string
	b string
}

type tokenizerJSON struct {
	Model struct {
		Type   string         `json:"type"`
		Vocab  map[string]int `json:"vocab"`
		Merges []any          `json:"merges"`
	} `json:"model"`
	AddedTokens []struct {
		ID      int    `json:"id"`
		Content string `json:"content"`
		Special bool   `json:"special"`
	} `json:"added_tokens"`
	PreTokenizer struct {
		Type          string `json:"type"`
		Pretokenizers []struct {
			Type    string `json:"type"`
			Pattern struct {
				Regex string `json:"Regex"`
			} `json:"pattern"`
		} `json:"pretokenizers"`
	} `json:"pre_tokenizer"`
}

// bpeTokenizer provides lightweight byte-level BPE encode/decode for Qwen tokenizer.json.
type bpeTokenizer struct {
	vocab          map[string]int64
	ivocab         map[int64]string
	mergeMap       map[mergePair]int
	specialByID    map[int64]string
	specialByToken map[string]int64
	pretokRegex    *regexp2.Regexp
}

func loadTokenizer(tokenizerPath string) (*bpeTokenizer, error) {
	data, err := os.ReadFile(tokenizerPath)
	if err != nil {
		return nil, fmt.Errorf("read tokenizer.json: %w", err)
	}

	var tj tokenizerJSON
	if err := json.Unmarshal(data, &tj); err != nil {
		return nil, fmt.Errorf("parse tokenizer.json: %w", err)
	}
	if strings.TrimSpace(strings.ToUpper(tj.Model.Type)) != "BPE" {
		return nil, fmt.Errorf("unsupported tokenizer model type: %s", tj.Model.Type)
	}

	vocab := make(map[string]int64, len(tj.Model.Vocab)+len(tj.AddedTokens))
	ivocab := make(map[int64]string, len(tj.Model.Vocab)+len(tj.AddedTokens))
	for tok, id := range tj.Model.Vocab {
		i := int64(id)
		vocab[tok] = i
		ivocab[i] = tok
	}

	specialByID := make(map[int64]string, len(tj.AddedTokens))
	specialByToken := make(map[string]int64, len(tj.AddedTokens))
	for _, item := range tj.AddedTokens {
		id := int64(item.ID)
		vocab[item.Content] = id
		ivocab[id] = item.Content
		if item.Special {
			specialByID[id] = item.Content
			specialByToken[item.Content] = id
		}
	}

	mergeMap := make(map[mergePair]int, len(tj.Model.Merges))
	for i, raw := range tj.Model.Merges {
		var a, b string
		switch v := raw.(type) {
		case string:
			line := strings.TrimSpace(v)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, " ", 2)
			if len(parts) != 2 {
				continue
			}
			a, b = parts[0], parts[1]
		case []any:
			if len(v) != 2 {
				continue
			}
			left, lok := v[0].(string)
			right, rok := v[1].(string)
			if !lok || !rok {
				continue
			}
			a, b = left, right
		default:
			continue
		}
		mergeMap[mergePair{a: a, b: b}] = i
	}

	pattern := defaultPreTokenizePattern
	for _, p := range tj.PreTokenizer.Pretokenizers {
		if strings.EqualFold(strings.TrimSpace(p.Type), "Split") && strings.TrimSpace(p.Pattern.Regex) != "" {
			pattern = p.Pattern.Regex
			break
		}
	}
	re, err := regexp2.Compile(pattern, 0)
	if err != nil {
		// Use a robust fallback so runtime still works if regex compatibility drifts.
		re, _ = regexp2.Compile(defaultPreTokenizePattern, 0)
	}

	return &bpeTokenizer{
		vocab:          vocab,
		ivocab:         ivocab,
		mergeMap:       mergeMap,
		specialByID:    specialByID,
		specialByToken: specialByToken,
		pretokRegex:    re,
	}, nil
}

func (t *bpeTokenizer) specialID(token string) (int64, bool) {
	if t == nil {
		return 0, false
	}
	id, ok := t.specialByToken[token]
	return id, ok
}

func (t *bpeTokenizer) encode(text string) []int64 {
	if t == nil || text == "" {
		return nil
	}
	parts := t.pretokenize(text)
	if len(parts) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(parts)*3)
	for _, part := range parts {
		symbols := bytesToUnicodeSymbols([]byte(part))
		if len(symbols) == 0 {
			continue
		}
		for _, tok := range t.bpe(symbols) {
			if id, ok := t.vocab[tok]; ok {
				ids = append(ids, id)
				continue
			}
			// Fallback to raw symbol-level lookup when merged token is unknown.
			for _, r := range tok {
				if id, ok := t.vocab[string(r)]; ok {
					ids = append(ids, id)
				}
			}
		}
	}
	return ids
}

func (t *bpeTokenizer) decode(ids []int64) string {
	if t == nil || len(ids) == 0 {
		return ""
	}
	out := make([]byte, 0, len(ids)*4)
	for _, id := range ids {
		token, ok := t.ivocab[id]
		if !ok {
			continue
		}
		if _, isSpecial := t.specialByID[id]; isSpecial {
			continue
		}
		for _, r := range token {
			if b, ok := unicodeToByte[r]; ok {
				out = append(out, b)
			} else {
				out = append(out, []byte(string(r))...)
			}
		}
	}
	return string(out)
}

func (t *bpeTokenizer) pretokenize(text string) []string {
	if t == nil || strings.TrimSpace(text) == "" {
		return nil
	}
	if t.pretokRegex == nil {
		return splitOnWhitespace(text)
	}
	var out []string
	m, err := t.pretokRegex.FindStringMatch(text)
	if err != nil {
		return splitOnWhitespace(text)
	}
	for m != nil {
		out = append(out, m.String())
		m, err = t.pretokRegex.FindNextMatch(m)
		if err != nil {
			break
		}
	}
	if len(out) == 0 {
		return splitOnWhitespace(text)
	}
	return out
}

func (t *bpeTokenizer) bpe(symbols []string) []string {
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
		bestRank := int(^uint(0) >> 1)
		for i := 0; i < len(word)-1; i++ {
			rank, ok := t.mergeMap[mergePair{a: word[i], b: word[i+1]}]
			if ok && rank < bestRank {
				bestRank = rank
				bestIdx = i
			}
		}
		if bestIdx < 0 {
			break
		}
		merged := word[bestIdx] + word[bestIdx+1]
		next := make([]string, 0, len(word)-1)
		next = append(next, word[:bestIdx]...)
		next = append(next, merged)
		if bestIdx+2 < len(word) {
			next = append(next, word[bestIdx+2:]...)
		}
		word = next
	}
	return word
}

func bytesToUnicodeSymbols(bs []byte) []string {
	if len(bs) == 0 {
		return nil
	}
	out := make([]string, 0, len(bs))
	for _, b := range bs {
		out = append(out, string(byteToUnicode[b]))
	}
	return out
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

var (
	byteToUnicode [256]rune
	unicodeToByte map[rune]byte
	byteMapOnce   sync.Once
)

func ensureByteMappings() {
	byteMapOnce.Do(func() {
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

		type pair struct {
			b int
			c int
		}
		pairs := make([]pair, len(bs))
		for i := range bs {
			pairs[i] = pair{b: bs[i], c: cs[i]}
		}
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].b < pairs[j].b })

		unicodeToByte = make(map[rune]byte, 256)
		for _, p := range pairs {
			r := rune(p.c)
			byteToUnicode[p.b] = r
			unicodeToByte[r] = byte(p.b)
		}
	})
}

func init() {
	ensureByteMappings()
}
