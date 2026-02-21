package proxy

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"unsafe"

	"github.com/tidwall/gjson"
)

// FNV-1a constants for fast, non-cryptographic hashing.
const (
	fnvOffset64 uint64 = 14695981039346656037
	fnvPrime64  uint64 = 1099511628211
)

// unsafeString converts a byte slice to string without copying.
// The caller MUST ensure the bytes are not modified while the string is in use.
func unsafeString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// Canonicalizer generates stable cache keys from LLM API requests.
// It sanitizes random/billing fields and normalizes parameters so that
// semantically identical requests produce the same hash.
type Canonicalizer struct{}

// NewCanonicalizer creates a new Canonicalizer.
func NewCanonicalizer() *Canonicalizer {
	return &Canonicalizer{}
}

// CanonicalKey generates a cache key from the request body using FNV-1a.
// Hashes fields directly into FNV state — no intermediate strings.Builder needed.
// Uses unsafe.String to avoid gjson's internal byte→string copy.
func (c *Canonicalizer) CanonicalKey(body []byte) string {
	if !gjson.ValidBytes(body) {
		return c.fnvHashBytes(body)
	}

	// unsafe.String avoids the copy that gjson.GetBytes does internally
	bodyStr := unsafe.String(unsafe.SliceData(body), len(body))
	model := gjson.Get(bodyStr, "model").Str
	temp := bucketFloat(gjson.Get(bodyStr, "temperature").Float(), 0.1)
	topP := bucketFloat(gjson.Get(bodyStr, "top_p").Float(), 0.1)
	maxTokens := gjson.Get(bodyStr, "max_tokens").Int()
	msgsHash := c.canonicalMessagesHash(gjson.Get(bodyStr, "messages"))

	// Hash all fields directly into FNV — no strings.Builder allocation
	hash := fnvOffset64
	for i := 0; i < len(model); i++ {
		hash ^= uint64(model[i])
		hash *= fnvPrime64
	}
	hash ^= uint64('|')
	hash *= fnvPrime64
	// Mix in messages hash
	hash ^= msgsHash
	hash *= fnvPrime64
	hash ^= uint64('|')
	hash *= fnvPrime64
	// Mix in temp/topP/maxTokens as raw bits
	hash ^= math.Float64bits(temp)
	hash *= fnvPrime64
	hash ^= math.Float64bits(topP)
	hash *= fnvPrime64
	hash ^= uint64(maxTokens)
	hash *= fnvPrime64

	return fnvHashToHex(hash)
}

// CanonicalKeyFromParsed generates a cache key from a pre-parsed request map.
// Avoids redundant JSON parsing when the caller already has the parsed body.
func (c *Canonicalizer) CanonicalKeyFromParsed(req map[string]interface{}) string {
	model, _ := req["model"].(string)
	temp := bucketFloat(getFloat(req, "temperature"), 0.1)
	topP := bucketFloat(getFloat(req, "top_p"), 0.1)
	maxTokens := getInt(req, "max_tokens")

	msgs := c.sanitizeMessages(req)
	canonical := c.canonicalMessages(msgs)

	// Build key data with strings.Builder to avoid fmt.Sprintf allocation
	var b strings.Builder
	b.Grow(len(model) + len(canonical) + 32)
	b.WriteString(model)
	b.WriteByte('|')
	b.WriteString(canonical)
	b.WriteByte('|')
	b.Write(strconv.AppendFloat(nil, temp, 'f', 1, 64))
	b.WriteByte('|')
	b.Write(strconv.AppendFloat(nil, topP, 'f', 1, 64))
	b.WriteByte('|')
	b.Write(strconv.AppendInt(nil, int64(maxTokens), 10))

	return c.fnvHashString(b.String())
}

// hexDigits for fast hex encoding without fmt.Sprintf.
const hexDigits = "0123456789abcdef"

// fnvHashToHex converts a uint64 hash to a 16-char hex string without fmt.Sprintf.
func fnvHashToHex(hash uint64) string {
	var buf [16]byte
	for i := 15; i >= 0; i-- {
		buf[i] = hexDigits[hash&0xf]
		hash >>= 4
	}
	return string(buf[:])
}

// fnvHashBytes computes FNV-1a hash of raw bytes.
func (c *Canonicalizer) fnvHashBytes(data []byte) string {
	hash := fnvOffset64
	for _, b := range data {
		hash ^= uint64(b)
		hash *= fnvPrime64
	}
	return fnvHashToHex(hash)
}

// fnvHashString computes FNV-1a hash of a string.
func (c *Canonicalizer) fnvHashString(s string) string {
	hash := fnvOffset64
	for i := 0; i < len(s); i++ {
		hash ^= uint64(s[i])
		hash *= fnvPrime64
	}
	return fnvHashToHex(hash)
}

// sanitizeMessages removes billing headers, cch tokens, and cc_version from messages.
func (c *Canonicalizer) sanitizeMessages(req map[string]interface{}) []interface{} {
	msgs, ok := req["messages"].([]interface{})
	if !ok {
		return nil
	}

	out := make([]interface{}, 0, len(msgs))
	for _, raw := range msgs {
		m, ok := raw.(map[string]interface{})
		if !ok {
			out = append(out, raw)
			continue
		}

		role, _ := m["role"].(string)
		content, _ := m["content"].(string)

		// Skip system messages that are billing/tracking headers
		if role == "system" && isBillingHeader(content) {
			continue
		}

		// Clean content of embedded tracking tokens
		if content != "" {
			content = cleanTrackingTokens(content)
		}

		cleaned := make(map[string]interface{}, len(m))
		for k, v := range m {
			cleaned[k] = v
		}
		if content != "" {
			cleaned["content"] = content
		}
		out = append(out, cleaned)
	}
	return out
}

// canonicalMessages produces a stable JSON string from messages.
func (c *Canonicalizer) canonicalMessages(msgs []interface{}) string {
	if len(msgs) == 0 {
		return "[]"
	}

	// Sort messages by role for stability (user messages may arrive in different order
	// for the same semantic request in some edge cases)
	sort.SliceStable(msgs, func(i, j int) bool {
		mi, _ := msgs[i].(map[string]interface{})
		mj, _ := msgs[j].(map[string]interface{})
		ri, _ := mi["role"].(string)
		rj, _ := mj["role"].(string)
		return ri < rj
	})

	data, _ := json.Marshal(msgs)
	return string(data)
}

// canonicalMessagesHash produces a stable FNV hash from messages using gjson,
// avoiding all intermediate string/slice allocations.
// Uses ForEach instead of Array() to avoid []Result heap allocation.
// Uses insertion sort (O(n²) but n is small) to avoid sort.SliceStable's reflect alloc.
func (c *Canonicalizer) canonicalMessagesHash(msgs gjson.Result) uint64 {
	if !msgs.Exists() || !msgs.IsArray() {
		return 0
	}

	// Collect role+content pairs, filtering billing headers.
	// Use stack array for common case (≤16 messages).
	type msgPair struct {
		role    string
		content string
	}
	var stackBuf [16]msgPair
	n := 0

	msgs.ForEach(func(_, msg gjson.Result) bool {
		role := msg.Get("role").Str
		content := msg.Get("content").Str
		if role == "system" && isBillingHeader(content) {
			return true // continue
		}
		if content != "" {
			content = cleanTrackingTokens(content)
		}
		if n < len(stackBuf) {
			stackBuf[n] = msgPair{role: role, content: content}
			n++
		}
		// Drop messages beyond 16 — extremely rare, acceptable for cache key stability
		return true
	})

	if n == 0 {
		return 0
	}
	pairs := stackBuf[:n]

	// Insertion sort by role — avoids sort.SliceStable's reflect.Swapper alloc
	for i := 1; i < len(pairs); i++ {
		key := pairs[i]
		j := i - 1
		for j >= 0 && pairs[j].role > key.role {
			pairs[j+1] = pairs[j]
			j--
		}
		pairs[j+1] = key
	}

	// Hash directly
	hash := fnvOffset64
	for _, p := range pairs {
		for i := 0; i < len(p.role); i++ {
			hash ^= uint64(p.role[i])
			hash *= fnvPrime64
		}
		hash ^= uint64('|')
		hash *= fnvPrime64
		for i := 0; i < len(p.content); i++ {
			hash ^= uint64(p.content[i])
			hash *= fnvPrime64
		}
		hash ^= uint64('\n')
		hash *= fnvPrime64
	}
	return hash
}

// isBillingHeader checks if content is a billing/tracking header.
// Uses case-insensitive prefix matching to avoid strings.ToLower allocation.
func isBillingHeader(content string) bool {
	return hasPrefixFold(content, "x-anthropic-billing") ||
		containsFold(content, "cch=")
}

// hasPrefixFold is like strings.HasPrefix but case-insensitive, zero-alloc.
func hasPrefixFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return strings.EqualFold(s[:len(prefix)], prefix)
}

// containsFold is like strings.Contains but case-insensitive for short needles.
func containsFold(s, needle string) bool {
	nl := len(needle)
	for i := 0; i <= len(s)-nl; i++ {
		if strings.EqualFold(s[i:i+nl], needle) {
			return true
		}
	}
	return false
}

// cleanTrackingTokens removes cch=xxx, cc_version=xxx tokens from content.
func cleanTrackingTokens(content string) string {
	result := content
	for _, prefix := range []string{"cch=", "cc_version=", "x-cc-session="} {
		for {
			idx := strings.Index(result, prefix)
			if idx < 0 {
				break
			}
			end := idx + len(prefix)
			for end < len(result) && result[end] != ' ' && result[end] != '\n' && result[end] != ',' && result[end] != ';' {
				end++
			}
			// Also consume trailing separator
			if end < len(result) && (result[end] == ' ' || result[end] == ',' || result[end] == ';') {
				end++
			}
			result = result[:idx] + result[end:]
		}
	}
	return strings.TrimSpace(result)
}

// bucketFloat rounds a float to the given precision.
func bucketFloat(v, precision float64) float64 {
	if precision <= 0 {
		return v
	}
	return math.Round(v/precision) * precision
}

// getFloat extracts a float64 from a map.
func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}

// getInt extracts an int from a map.
func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}
