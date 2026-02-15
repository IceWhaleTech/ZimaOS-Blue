package proxy

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

// FNV-1a constants for fast, non-cryptographic hashing.
const (
	fnvOffset64 uint64 = 14695981039346656037
	fnvPrime64  uint64 = 1099511628211
)

// Canonicalizer generates stable cache keys from LLM API requests.
// It sanitizes random/billing fields and normalizes parameters so that
// semantically identical requests produce the same hash.
type Canonicalizer struct{}

// NewCanonicalizer creates a new Canonicalizer.
func NewCanonicalizer() *Canonicalizer {
	return &Canonicalizer{}
}

// CanonicalKey generates a cache key from the request body using FNV-1a.
// Key = FNV-1a(model + canonical(messages) + temp_bucket + topP_bucket + maxTokens)
func (c *Canonicalizer) CanonicalKey(body []byte) string {
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return c.fnvHashBytes(body)
	}
	return c.CanonicalKeyFromParsed(req)
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
	fmt.Fprintf(&b, "%.1f|%.1f|%d", temp, topP, maxTokens)

	return c.fnvHashString(b.String())
}

// fnvHashBytes computes FNV-1a hash of raw bytes.
func (c *Canonicalizer) fnvHashBytes(data []byte) string {
	hash := fnvOffset64
	for _, b := range data {
		hash ^= uint64(b)
		hash *= fnvPrime64
	}
	return fmt.Sprintf("%016x", hash)
}

// fnvHashString computes FNV-1a hash of a string.
func (c *Canonicalizer) fnvHashString(s string) string {
	hash := fnvOffset64
	for i := 0; i < len(s); i++ {
		hash ^= uint64(s[i])
		hash *= fnvPrime64
	}
	return fmt.Sprintf("%016x", hash)
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

// isBillingHeader checks if content is a billing/tracking header.
func isBillingHeader(content string) bool {
	lower := strings.ToLower(content)
	return strings.HasPrefix(lower, "x-anthropic-billing-header:") ||
		strings.HasPrefix(lower, "x-anthropic-billing") ||
		strings.Contains(lower, "cch=")
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
