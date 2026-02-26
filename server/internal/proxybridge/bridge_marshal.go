package proxybridge

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

// bridgeRequest mirrors the OpenAI chat completion request format.
type bridgeRequest struct {
	Model         string              `json:"model"`
	Messages      []bridgeMessage     `json:"messages"`
	Temperature   float64             `json:"temperature,omitempty"`
	MaxTokens     int                 `json:"max_tokens,omitempty"`
	Tools         []bridgeTool        `json:"tools,omitempty"`
	Stream        bool                `json:"stream,omitempty"`
	StreamOptions *bridgeStreamOpts   `json:"stream_options,omitempty"`
}

type bridgeStreamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

type bridgeMessage struct {
	Role       string          `json:"role"`
	Content    interface{}     `json:"content"` // string or []bridgeContentPart
	ToolCalls  []bridgeToolCall `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

type bridgeContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *bridgeImageURL `json:"image_url,omitempty"`
}

type bridgeImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type bridgeTool struct {
	Type     string         `json:"type"`
	Function bridgeFunction `json:"function"`
}

type bridgeFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type bridgeToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"`
}

// bridgeResponse mirrors the OpenAI chat completion response format.
type bridgeResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role      string           `json:"role"`
			Content   string           `json:"content"`
			ToolCalls []bridgeToolCall  `json:"tool_calls,omitempty"`
		} `json:"message"`
		Delta struct {
			Role      string           `json:"role,omitempty"`
			Content   string           `json:"content,omitempty"`
			ToolCalls []bridgeToolCall  `json:"tool_calls,omitempty"`
		} `json:"delta,omitempty"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// MarshalChatRequest converts llm.ChatRequest to OpenAI-format JSON bytes.
func MarshalChatRequest(req llm.ChatRequest) ([]byte, error) {
	msgs := make([]bridgeMessage, len(req.Messages))
	for i, m := range req.Messages {
		bm := bridgeMessage{
			Role:       string(m.Role),
			ToolCallID: m.ToolCallID,
		}
		if len(m.ContentParts) > 0 {
			parts := make([]bridgeContentPart, 0, len(m.ContentParts))
			for _, p := range m.ContentParts {
				switch p.Type {
				case "text":
					parts = append(parts, bridgeContentPart{Type: "text", Text: p.Text})
				case "image":
					dataURL := "data:" + p.MediaType + ";base64," + p.Data
					parts = append(parts, bridgeContentPart{
						Type:     "image_url",
						ImageURL: &bridgeImageURL{URL: dataURL, Detail: "auto"},
					})
				}
			}
			bm.Content = parts
		} else if len(m.ToolCalls) > 0 {
			// When tool_calls are present, set content to empty string "".
			// Many OpenAI→Anthropic relays (e.g. tribios) fail to convert
			// tool_calls into Anthropic tool_use blocks when content is null,
			// causing "tool_result has no corresponding tool_use" errors.
			// Empty string is valid per OpenAI spec and gives relays a
			// parseable value during format conversion.
			bm.Content = ""
		} else if m.Content != "" {
			// For tool results, if content is already valid JSON, embed it
			// directly as json.RawMessage to avoid double-encoding.
			if m.Role == llm.RoleTool && json.Valid([]byte(m.Content)) {
				bm.Content = json.RawMessage(m.Content)
			} else {
				bm.Content = m.Content
			}
		}
		if len(m.ToolCalls) > 0 {
			bm.ToolCalls = make([]bridgeToolCall, len(m.ToolCalls))
			for j, tc := range m.ToolCalls {
				bm.ToolCalls[j] = bridgeToolCall{ID: tc.ID, Type: "function"}
				bm.ToolCalls[j].Function.Name = tc.Name
				bm.ToolCalls[j].Function.Arguments = toRawJSON(tc.Arguments)
			}
		}
		msgs[i] = bm
	}

	br := bridgeRequest{
		Model:       req.Model,
		Messages:    msgs,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
	}
	if req.Stream {
		// Note: stream_options is not set — many relay/proxy services
		// reject unknown fields with 400 "Improperly formed request"
	}
	if len(req.Tools) > 0 {
		br.Tools = make([]bridgeTool, len(req.Tools))
		for i, t := range req.Tools {
			br.Tools[i] = bridgeTool{
				Type: "function",
				Function: bridgeFunction{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.Parameters,
				},
			}
		}
	}
	result, err := json.Marshal(br)
	if err == nil {
		// Log only the last assistant+tool_calls message (current round)
		for i := len(msgs) - 1; i >= 0; i-- {
			if len(msgs[i].ToolCalls) > 0 {
				tc := msgs[i].ToolCalls[0]
				args := string(tc.Function.Arguments)
				if len(args) > 200 {
					args = args[:200] + "..."
				}
				slog.Info("[bridge] tool_calls in request",
					"tool", tc.Function.Name,
					"args", args,
					"count", len(msgs[i].ToolCalls),
					"msg_index", i,
					"total_msgs", len(msgs))
				break
			}
		}
	}
	return result, err
}

// ParseChatResponse parses OpenAI-format JSON into llm.ChatResponse.
func ParseChatResponse(body []byte) (*llm.ChatResponse, error) {
	var resp bridgeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if resp.Error != nil {
		// Sanitize: only expose error type, not full upstream message which may leak internal details
		errType := resp.Error.Type
		if errType == "" {
			errType = "unknown"
		}
		return nil, fmt.Errorf("upstream error: type=%s", errType)
	}
	cr := &llm.ChatResponse{
		ID:    resp.ID,
		Model: resp.Model,
		Usage: llm.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}
	if len(resp.Choices) > 0 {
		c := resp.Choices[0]
		cr.Message = llm.Message{
			Role:    llm.Role(c.Message.Role),
			Content: c.Message.Content,
		}
		if len(c.Message.ToolCalls) > 0 {
			cr.Message.ToolCalls = make([]llm.ToolCall, len(c.Message.ToolCalls))
			for i, tc := range c.Message.ToolCalls {
				cr.Message.ToolCalls[i] = llm.ToolCall{
					ID:        tc.ID,
					Name:      tc.Function.Name,
					Arguments: rawToString(tc.Function.Arguments),
				}
			}
		}
	}
	return cr, nil
}

// ParseSSEChunk parses a single SSE data line into an llm.StreamChunk.
// Returns (chunk, done, error). done=true on [DONE] sentinel.
func ParseSSEChunk(dataPayload string) (llm.StreamChunk, bool, error) {
	if dataPayload == "[DONE]" {
		return llm.StreamChunk{Done: true}, true, nil
	}
	var resp bridgeResponse
	if err := json.Unmarshal([]byte(dataPayload), &resp); err != nil {
		return llm.StreamChunk{}, false, err
	}

	chunk := llm.StreamChunk{
		ID:    resp.ID,
		Model: resp.Model,
	}

	if len(resp.Choices) > 0 {
		c := resp.Choices[0]
		chunk.Delta = c.Delta.Content
		if c.FinishReason == "stop" || c.FinishReason == "end_turn" || c.FinishReason == "tool_calls" {
			chunk.Done = true
		}
		if len(c.Delta.ToolCalls) > 0 {
			chunk.ToolCalls = make([]llm.ToolCall, len(c.Delta.ToolCalls))
			for i, tc := range c.Delta.ToolCalls {
				chunk.ToolCalls[i] = llm.ToolCall{
					ID:        tc.ID,
					Name:      tc.Function.Name,
					Arguments: rawToString(tc.Function.Arguments),
				}
			}
		}
	}

	if resp.Usage.TotalTokens > 0 {
		chunk.Usage = &llm.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}

	return chunk, chunk.Done, nil
}

// toRawJSON converts a string to json.RawMessage.
// If the string is already valid JSON, it's used directly.
// Otherwise it's JSON-encoded as a string value.
func toRawJSON(s string) json.RawMessage {
	if len(s) > 0 && json.Valid([]byte(s)) {
		return json.RawMessage(s)
	}
	b, _ := json.Marshal(s)
	return b
}

// rawToString extracts a Go string from json.RawMessage.
// If the raw value is a JSON string (starts with "), it unquotes it.
// Otherwise returns the raw bytes as-is (e.g. for streaming deltas
// which are partial JSON fragments).
func rawToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// If it's a JSON string, unquote it to get the inner value
	if raw[0] == '"' {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return s
		}
	}
	return string(raw)
}
