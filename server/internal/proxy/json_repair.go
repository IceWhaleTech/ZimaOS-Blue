package proxy

// repairJSON attempts to fix truncated JSON by closing unclosed brackets,
// braces, and strings. This handles SSE lines truncated by network issues.
// If the input is already valid JSON, it is returned as-is.
//
// IMPORTANT: Repaired JSON may contain truncated string values. Callers must
// validate critical fields after unmarshaling repaired data. Only use repaired
// results for loss-tolerant fields (e.g. streaming text deltas).
func repairJSON(data []byte) []byte {
	if json.Valid(data) {
		return data
	}

	// Track state: string context, escape, and bracket stack.
	inString := false
	escaped := false
	var stack []byte // tracks '{' and '['

	for _, b := range data {
		if escaped {
			escaped = false
			continue
		}
		if inString {
			switch b {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch b {
		case '"':
			inString = true
		case '{':
			stack = append(stack, '}')
		case '[':
			stack = append(stack, ']')
		case '}', ']':
			if len(stack) > 0 && stack[len(stack)-1] == b {
				stack = stack[:len(stack)-1]
			}
		}
	}

	if !inString && len(stack) == 0 {
		return data // nothing to fix
	}

	// Build repaired output.
	out := make([]byte, 0, len(data)+len(stack)+1)
	out = append(out, data...)
	if inString {
		out = append(out, '"')
	}
	// Close brackets in reverse order.
	for i := len(stack) - 1; i >= 0; i-- {
		out = append(out, stack[i])
	}
	return out
}

// isRepairedEventSafe checks whether a repaired (truncated) SSE event is safe
// to forward. Structural events (message_start, content_block_start,
// message_delta) carry IDs, names, and stop reasons that MUST NOT be truncated
// — forwarding corrupted values causes downstream failures (wrong message ID,
// broken tool call matching, lost finish signal). Only text_delta is
// loss-tolerant: a truncated text chunk just means the client sees a partial
// word, which the next chunk will complete.
//
// input_json_delta is NOT safe: truncated partial_json corrupts the assembled
// tool arguments on the client side.
func isRepairedEventSafe(event *AnthropicStreamEvent) bool {
	switch event.Type {
	case "content_block_delta":
		if event.Delta != nil && event.Delta.Type == "text_delta" {
			return true // text truncation is acceptable in streaming
		}
		return false // input_json_delta or unknown delta type — not safe
	default:
		// message_start, content_block_start, message_delta, message_stop,
		// content_block_stop — all carry structural data, skip if truncated.
		return false
	}
}
