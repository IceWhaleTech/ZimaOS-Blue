package proxy

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// parseSSEChunks extracts JSON objects from SSE data lines.
func parseSSEChunks(data []byte) []map[string]interface{} {
	var chunks []map[string]interface{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}
		var chunk map[string]interface{}
		if json.Unmarshal([]byte(payload), &chunk) == nil {
			chunks = append(chunks, chunk)
		}
	}
	return chunks
}

// assembleNonStreamingResponse converts SSE chunks into a
// non-streaming chat completion JSON for caching.
func assembleNonStreamingResponse(chunks []map[string]interface{}) []byte {
	if len(chunks) == 0 {
		return nil
	}

	first := chunks[0]
	id, _ := first["id"].(string)
	model, _ := first["model"].(string)

	type choiceAcc struct {
		role         string
		content      strings.Builder
		finishReason string
	}
	choices := make(map[int]*choiceAcc)

	for _, chunk := range chunks {
		rawChoices, _ := chunk["choices"].([]interface{})
		for _, rc := range rawChoices {
			c, _ := rc.(map[string]interface{})
			if c == nil {
				continue
			}
			idx := int(getFloat(c, "index"))
			acc, ok := choices[idx]
			if !ok {
				acc = &choiceAcc{}
				choices[idx] = acc
			}
			delta, _ := c["delta"].(map[string]interface{})
			if delta == nil {
				continue
			}
			if r, ok := delta["role"].(string); ok && r != "" {
				acc.role = r
			}
			if ct, ok := delta["content"].(string); ok {
				acc.content.WriteString(ct)
			}
			if fr, ok := c["finish_reason"].(string); ok && fr != "" {
				acc.finishReason = fr
			}
		}
	}

	var outChoices []map[string]interface{}
	for idx := 0; idx < len(choices); idx++ {
		acc, ok := choices[idx]
		if !ok {
			continue
		}
		role := acc.role
		if role == "" {
			role = "assistant"
		}
		outChoices = append(outChoices, map[string]interface{}{
			"index": idx,
			"message": map[string]interface{}{
				"role":    role,
				"content": acc.content.String(),
			},
			"finish_reason": acc.finishReason,
		})
	}

	// Extract usage from last chunk if present
	var usage interface{}
	for i := len(chunks) - 1; i >= 0; i-- {
		if u, ok := chunks[i]["usage"]; ok && u != nil {
			usage = u
			break
		}
	}

	resp := map[string]interface{}{
		"id":      id,
		"object":  "chat.completion",
		"created": timeutil.Now(),
		"model":   model,
		"choices": outChoices,
	}
	if usage != nil {
		resp["usage"] = usage
	}

	data, _ := json.Marshal(resp)
	return data
}

// convertToSSE converts a non-streaming chat completion JSON
// into SSE format for streaming clients.
func convertToSSE(body []byte) []byte {
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		var buf bytes.Buffer
		fmt.Fprintf(&buf, "data: %s\n\ndata: [DONE]\n\n", body)
		return buf.Bytes()
	}

	id, _ := resp["id"].(string)
	model, _ := resp["model"].(string)
	created, _ := resp["created"].(float64)
	rawChoices, _ := resp["choices"].([]interface{})

	var buf bytes.Buffer

	for _, rc := range rawChoices {
		c, _ := rc.(map[string]interface{})
		if c == nil {
			continue
		}
		msg, _ := c["message"].(map[string]interface{})
		if msg == nil {
			continue
		}
		idx := int(getFloat(c, "index"))
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)
		finishReason, _ := c["finish_reason"].(string)

		chunk := map[string]interface{}{
			"id":      id,
			"object":  "chat.completion.chunk",
			"created": int64(created),
			"model":   model,
			"choices": []map[string]interface{}{
				{
					"index": idx,
					"delta": map[string]interface{}{
						"role":    role,
						"content": content,
					},
					"finish_reason": finishReason,
				},
			},
		}
		if usage, ok := resp["usage"]; ok && usage != nil {
			chunk["usage"] = usage
		}

		data, _ := json.Marshal(chunk)
		fmt.Fprintf(&buf, "data: %s\n\n", data)
	}

	buf.WriteString("data: [DONE]\n\n")
	return buf.Bytes()
}
