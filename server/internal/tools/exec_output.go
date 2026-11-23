package tools

import (
	"encoding/json"
	"unicode/utf8"
)

// CompactExecOutputForLLM budgets the JSON envelope, keeping session recovery
// available whenever a preview omits output. Source buffer loss is separate.
func CompactExecOutputForLLM(payload map[string]interface{}, budget int) string {
	out := sanitizeToolPayload(payload, maxToolGatewayLLMStringBytes).(map[string]interface{})
	for _, key := range []string{"stdout", "stderr", "output"} {
		if value, ok := payload[key].(string); ok {
			out[key] = sanitizeToolString(value, 0)
		}
	}
	encode := func() string { data, _ := json.Marshal(out); return string(data) }
	if content := encode(); len(content) <= budget {
		return content
	}
	out["llm_output_truncated"] = true
	updateRecovery := func() {
		offset := 0
		if value, ok := out["output"].(string); ok {
			start, _ := firstCompatIntDeep(out, "offset")
			offset = start + len(value)
			out["next_offset"] = offset
			out["has_more"] = true
		}
		if id, _ := out["session_id"].(string); id != "" {
			out["output_retrieval"] = map[string]interface{}{"tool": "exec", "action": "log", "session_id": id, "offset": offset, "limit": 4096}
			out["output_notice"] = "Incomplete preview; call exec with action=log and the retained session_id before concluding evidence is absent."
		}
	}
	for {
		updateRecovery()
		if content := encode(); len(content) <= budget {
			return content
		}
		longest := ""
		for _, key := range []string{"stdout", "stderr", "output"} {
			value, _ := out[key].(string)
			previous, _ := out[longest].(string)
			if len(value) > len(previous) {
				longest = key
			}
		}
		if longest == "" {
			break
		}
		value := out[longest].(string)
		limit := len(value) / 2
		for limit > 0 && !utf8.ValidString(value[:limit]) {
			limit--
		}
		out[longest] = value[:limit]
	}
	// Tiny round budgets still need valid JSON and a session handle, not a
	// sliced JSON prefix that looks like a complete tool result.
	for key := range out {
		switch key {
		case "session_id", "exit_code", "truncated", "llm_output_truncated", "offset", "next_offset", "has_more":
		default:
			delete(out, key)
		}
	}
	if content := encode(); len(content) <= budget {
		return content
	}
	for _, key := range []string{"exit_code", "truncated", "offset", "next_offset", "has_more", "session_id"} {
		delete(out, key)
		if content := encode(); len(content) <= budget {
			return content
		}
	}
	return "{}"
}
