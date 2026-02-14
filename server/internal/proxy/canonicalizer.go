package proxy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Canonicalizer generates stable cache keys from LLM API requests.
// It sanitizes random/billing fields and normalizes parameters so that
// semantically identical requests produce the same hash.
type Canonicalizer struct{}

// NewCanonicalizer creates a new Canonicalizer.
func NewCanonicalizer() *Canonicalizer {
	return &Canonicalizer{}
}

// CanonicalKey generates a SHA256 cache key from the request body.
// Key = SHA256(model + canonical(messages) + temp_bucket + topP_bucket + maxTokens)
func (c *Canonicalizer) CanonicalKey(body []byte) string {
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		// Fallback: hash raw body
		sum := sha256.Sum256(body)
		return hex.EncodeToString(sum[:])
	}

	model, _ := req["model"].(string)
	temp := bucketFloat(getFloat(req, "temperature"), 0.1)
	topP := bucketFloat(getFloat(req, "top_p"), 0.1)
	maxTokens := getInt(req, "max_tokens")

	// Sanitize and canonicalize messages
	msgs := c.sanitizeMessages(req)
	canonical := c.canonicalMessages(msgs)

	keyData := fmt.Sprintf("%s|%s|%.1f|%.1f|%d", model, canonical, temp, topP, maxTokens)
	sum := sha256.Sum256([]byte(keyData))
	return hex.EncodeToString(sum[:])
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
	// Remove cch=<hex> patterns
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
