package querylang

// The inference format and signed-byte FNV hashing follow fastText v0.9.2.
// Upstream code attribution and model license are in data/README.md.

import (
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"sync"
	"unicode/utf8"
)

//go:embed data/lid.176.ftz
var languageIDModelData []byte

var builtinLanguageID = sync.OnceValues(func() (*languageIDModel, error) {
	return loadLanguageIDModel(languageIDModelData)
})

type languageIDPrediction struct {
	Language    string  `json:"language"`
	Probability float64 `json:"probability"`
}

type languageIDNode struct {
	left, right int
	count       int64
}

type languageIDModel struct {
	words                    map[string]int
	labels                   []string
	pruned                   map[uint32]int
	codes, normCodes         []byte
	centroids, norms, output []float32
	tree                     []languageIDNode
}

// This cursor is used only after checking the entire immutable model's SHA256.
// It intentionally supports that pinned format, not arbitrary fastText files.
type languageIDCursor struct {
	data   []byte
	offset int
}

func (c *languageIDCursor) take(n int) []byte {
	v := c.data[c.offset : c.offset+n]
	c.offset += n
	return v
}
func (c *languageIDCursor) i32() int   { return int(int32(binary.LittleEndian.Uint32(c.take(4)))) }
func (c *languageIDCursor) i64() int64 { return int64(binary.LittleEndian.Uint64(c.take(8))) }
func (c *languageIDCursor) floats(n int) []float32 {
	v := make([]float32, n)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(c.take(4)))
	}
	return v
}

func loadLanguageIDModel(data []byte) (*languageIDModel, error) {
	const digest = "8f3472cfe8738a7b6099e8e999c3cbfae0dcd15696aac7d7738a8039db603e83"
	if len(data) != 938013 || fmt.Sprintf("%x", sha256.Sum256(data)) != digest {
		return nil, fmt.Errorf("language detector model integrity failure")
	}
	c := languageIDCursor{data: data}
	if c.i32() != 793712314 || c.i32() != 12 {
		return nil, fmt.Errorf("unsupported language detector format")
	}
	var args [12]int
	for i := range args {
		args[i] = c.i32()
	}
	c.take(8) // training sampling threshold
	if args[0] != 16 || args[5] != 1 || args[6] != 1 || args[7] != 3 || args[8] != 2000000 || args[9] != 2 || args[10] != 4 {
		return nil, fmt.Errorf("unsupported language detector parameters")
	}
	size, nwords, nlabels := c.i32(), c.i32(), c.i32()
	c.i64() // training token count
	pruned := int(c.i64())
	m := &languageIDModel{words: make(map[string]int, nwords), pruned: make(map[uint32]int, pruned)}
	counts := make([]int64, 0, nlabels)
	for i := 0; i < size; i++ {
		start := c.offset
		for c.take(1)[0] != 0 {
		}
		word := string(data[start : c.offset-1])
		count := c.i64()
		kind := c.take(1)[0]
		if kind == 0 {
			m.words[word] = i
		} else {
			m.labels = append(m.labels, strings.TrimPrefix(word, "__label__"))
			counts = append(counts, count)
		}
	}
	for i := 0; i < pruned; i++ {
		hash, id := c.i32(), c.i32()
		m.pruned[uint32(hash)] = id + nwords
	}
	if c.take(1)[0] != 1 || c.take(1)[0] != 1 {
		return nil, fmt.Errorf("language detector requires quantized normalized input")
	}
	rows, dim := int(c.i64()), int(c.i64())
	m.codes = c.take(c.i32())
	if dim != 16 || len(m.codes) != rows*8 {
		return nil, fmt.Errorf("invalid language detector input")
	}
	for _, want := range []int{16, 8, 2, 2} {
		if c.i32() != want {
			return nil, fmt.Errorf("invalid language detector quantizer")
		}
	}
	m.centroids = c.floats(16 * 256)
	m.normCodes = c.take(rows)
	for range 4 {
		if c.i32() != 1 {
			return nil, fmt.Errorf("invalid language detector norms")
		}
	}
	m.norms = c.floats(256)
	if c.take(1)[0] != 0 || c.i64() != int64(nlabels) || c.i64() != 16 {
		return nil, fmt.Errorf("invalid language detector output")
	}
	m.output = c.floats(nlabels * 16)
	if c.offset != len(data) {
		return nil, fmt.Errorf("unexpected language detector payload")
	}
	m.tree = make([]languageIDNode, 2*nlabels-1)
	for i := range m.tree {
		m.tree[i] = languageIDNode{left: -1, right: -1, count: 1000000000000000}
	}
	for i, count := range counts {
		m.tree[i].count = count
	}
	leaf, node := nlabels-1, nlabels
	for i := nlabels; i < len(m.tree); i++ {
		var children [2]int
		for j := range children {
			if leaf >= 0 && m.tree[leaf].count < m.tree[node].count {
				children[j] = leaf
				leaf--
			} else {
				children[j] = node
				node++
			}
		}
		m.tree[i] = languageIDNode{left: children[0], right: children[1], count: m.tree[children[0]].count + m.tree[children[1]].count}
	}
	return m, nil
}

func languageIDHash(word string) uint32 {
	h := uint32(2166136261)
	for i := range len(word) {
		h = (h ^ uint32(int8(word[i]))) * 16777619
	}
	return h
}

func (m *languageIDModel) predict(text string) (best [3]languageIDPrediction) {
	var hidden [16]float32
	nfeatures := 0
	addRow := func(row int) {
		nfeatures++
		norm := m.norms[m.normCodes[row]]
		for sub := 0; sub < 8; sub++ {
			base := (sub*256 + int(m.codes[row*8+sub])) * 2
			hidden[sub*2] += float32(norm * m.centroids[base])
			hidden[sub*2+1] += float32(norm * m.centroids[base+1])
		}
	}
	// Match native tokenization, including a single end-of-line feature. The
	// public query gate normalizes multiline/Unicode whitespace before inference.
	line, _, _ := strings.Cut(text, "\n")
	words := strings.FieldsFunc(line, func(r rune) bool { return strings.ContainsRune(" \r\t\v\f\x00", r) })
	words = append(words, "</s>")
	for _, word := range words {
		if strings.HasPrefix(word, "__label__") {
			continue
		}
		if id, ok := m.words[word]; ok {
			addRow(id)
		}
		if word == "</s>" {
			continue
		}
		word = "<" + word + ">"
		for start := range word {
			end := start
			for n := 1; end < len(word) && n <= 4; n++ {
				_, size := utf8.DecodeRuneInString(word[end:])
				end += size
				if n >= 2 {
					if row, ok := m.pruned[languageIDHash(word[start:end])%2000000]; ok {
						addRow(row)
					}
				}
			}
		}
	}
	if nfeatures == 0 {
		return best
	}
	for i := range hidden {
		hidden[i] *= float32(1 / float64(nfeatures))
	}
	type visit struct {
		node  int
		score float32
	}
	var stack [351]visit
	stack[0] = visit{node: len(m.tree) - 1}
	next := 1
	for next > 0 {
		next--
		v := stack[next]
		node := m.tree[v.node]
		if node.left < 0 {
			p := languageIDPrediction{Language: m.labels[v.node], Probability: float64(float32(math.Exp(float64(v.score))))}
			for i := range best {
				if p.Probability > best[i].Probability {
					for j := len(best) - 1; j > i; j-- {
						best[j] = best[j-1]
					}
					best[i] = p
					break
				}
			}
			continue
		}
		var logit float32
		for i, value := range hidden {
			logit += float32(value * m.output[(v.node-len(m.labels))*16+i])
		}
		f := float32(1 / float64(1+float32(math.Exp(float64(-logit)))))
		stack[next] = visit{node: node.left, score: v.score + float32(math.Log(float64(1-f)+0.00001))}
		stack[next+1] = visit{node: node.right, score: v.score + float32(math.Log(float64(f)+0.00001))}
		next += 2
	}
	return best
}
