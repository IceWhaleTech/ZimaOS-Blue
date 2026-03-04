package proxybridge

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

// bridgeRequest mirrors the OpenAI chat completion request format.
type bridgeRequest struct {
	Model         string            `json:"model"`
	Messages      []bridgeMessage   `json:"messages"`
	Temperature   float64           `json:"temperature,omitempty"`
	MaxTokens     int               `json:"max_tokens,omitempty"`
	Tools         []bridgeTool      `json:"tools,omitempty"`
	Stream        bool              `json:"stream,omitempty"`
	StreamOptions *bridgeStreamOpts `json:"stream_options,omitempty"`
}

type bridgeStreamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

type bridgeMessage struct {
	Role       string           `json:"role"`
	Content    interface{}      `json:"content"` // string or []bridgeContentPart
	ToolCalls  []bridgeToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
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
			ToolCalls []bridgeToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		Delta struct {
			Role      string           `json:"role,omitempty"`
			Content   string           `json:"content,omitempty"`
			ToolCalls []bridgeToolCall `json:"tool_calls,omitempty"`
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

type bridgeResponsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type bridgeResponsesOutputContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type bridgeResponsesOutputItem struct {
	ID        string                         `json:"id,omitempty"`
	Type      string                         `json:"type"`
	Role      string                         `json:"role,omitempty"`
	Name      string                         `json:"name,omitempty"`
	CallID    string                         `json:"call_id,omitempty"`
	Arguments json.RawMessage                `json:"arguments,omitempty"`
	Content   []bridgeResponsesOutputContent `json:"content,omitempty"`
}

type bridgeResponsesResponse struct {
	ID     string                      `json:"id"`
	Object string                      `json:"object"`
	Model  string                      `json:"model"`
	Output []bridgeResponsesOutputItem `json:"output,omitempty"`
	Usage  bridgeResponsesUsage        `json:"usage"`
	Error  *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

type bridgeResponsesEvent struct {
	Type       string                     `json:"type"`
	ResponseID string                     `json:"response_id,omitempty"`
	Delta      string                     `json:"delta,omitempty"`
	Arguments  string                     `json:"arguments,omitempty"`
	Item       *bridgeResponsesOutputItem `json:"item,omitempty"`
	Response   *bridgeResponsesResponse   `json:"response,omitempty"`
	Error      *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

type bridgeResponsesRequest struct {
	Model              string                `json:"model"`
	Store              bool                  `json:"store"`
	Input              []interface{}         `json:"input,omitempty"`
	Tools              []bridgeResponsesTool `json:"tools,omitempty"`
	Stream             bool                  `json:"stream,omitempty"`
	MaxOutputTokens    int                   `json:"max_output_tokens,omitempty"`
	Temperature        *float64              `json:"temperature,omitempty"`
	PreviousResponseID string                `json:"previous_response_id,omitempty"`
	Instructions       string                `json:"instructions,omitempty"`
}

type bridgeResponsesTool struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type bridgeResponsesInputMessage struct {
	Role    string                       `json:"role"`
	Content []bridgeResponsesContentPart `json:"content"`
}

type bridgeResponsesContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type bridgeResponsesFunctionCall struct {
	Type      string `json:"type"`
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type bridgeResponsesFunctionCallOutput struct {
	Type   string `json:"type"`
	CallID string `json:"call_id,omitempty"`
	Output string `json:"output,omitempty"`
}

const (
	maxResponsesRequestBytes            = 28 * 1024
	maxResponsesInputTextBytes          = 1024
	maxResponsesFunctionArgsBytes       = 768
	maxResponsesFunctionOutputBytes     = 3072
	maxResponsesFunctionOutputTight     = 1024
	maxResponsesFunctionOutputEmergency = 384
	maxResponsesContinuationOutputsKeep = 2
)

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

// MarshalResponsesRequest converts llm.ChatRequest to native OpenAI Responses API JSON bytes.
func MarshalResponsesRequest(req llm.ChatRequest) ([]byte, error) {
	out := bridgeResponsesRequest{
		Model:  req.Model,
		Store:  true,
		Stream: req.Stream,
	}
	if req.Store != nil {
		out.Store = *req.Store
	}
	if req.MaxTokens > 0 {
		out.MaxOutputTokens = req.MaxTokens
	}
	if req.Temperature != 0 {
		out.Temperature = &req.Temperature
	}
	if req.PreviousResponseID != "" {
		out.PreviousResponseID = req.PreviousResponseID
	}

	messages := req.Messages
	if out.PreviousResponseID != "" {
		messages = trimMessagesForResponsesContinuation(messages)
	}
	if out.PreviousResponseID == "" {
		instructions, trimmed := extractResponsesInstructions(messages)
		if instructions != "" {
			out.Instructions = instructions
		}
		messages = trimmed
	}

	input := make([]interface{}, 0, len(messages)+2)
	for msgIdx, m := range messages {
		role := strings.ToLower(strings.TrimSpace(string(m.Role)))
		switch role {
		case "tool":
			if strings.TrimSpace(m.ToolCallID) == "" {
				continue
			}
			input = append(input, bridgeResponsesFunctionCallOutput{
				Type:   "function_call_output",
				CallID: strings.TrimSpace(m.ToolCallID),
				Output: m.Content,
			})
		default:
			parts := make([]bridgeResponsesContentPart, 0, len(m.ContentParts)+1)
			if m.Content != "" {
				parts = append(parts, bridgeResponsesContentPart{
					Type: "input_text",
					Text: m.Content,
				})
			}
			for _, p := range m.ContentParts {
				switch p.Type {
				case "text":
					if p.Text == "" {
						continue
					}
					parts = append(parts, bridgeResponsesContentPart{Type: "input_text", Text: p.Text})
				case "image":
					if p.Data == "" || p.MediaType == "" {
						continue
					}
					parts = append(parts, bridgeResponsesContentPart{
						Type:     "input_image",
						ImageURL: "data:" + p.MediaType + ";base64," + p.Data,
					})
				}
			}
			if len(parts) > 0 {
				input = append(input, bridgeResponsesInputMessage{
					Role:    normalizeResponsesInputRole(role),
					Content: parts,
				})
			}
			if role == "assistant" && len(m.ToolCalls) > 0 {
				for callIdx, tc := range m.ToolCalls {
					callID := strings.TrimSpace(tc.ID)
					if callID == "" {
						callID = "call_" + strconv.Itoa(msgIdx) + "_" + strconv.Itoa(callIdx)
					}
					input = append(input, bridgeResponsesFunctionCall{
						Type:      "function_call",
						CallID:    callID,
						Name:      tc.Name,
						Arguments: tc.Arguments,
					})
				}
			}
		}
	}
	if len(input) > 0 {
		out.Input = input
	}

	if len(req.Tools) > 0 {
		out.Tools = make([]bridgeResponsesTool, 0, len(req.Tools))
		for _, t := range req.Tools {
			if strings.TrimSpace(t.Name) == "" {
				continue
			}
			out.Tools = append(out.Tools, bridgeResponsesTool{
				Type:        "function",
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			})
		}
	}

	return marshalResponsesRequestWithSizeGuard(out)
}

func trimMessagesForResponsesContinuation(messages []llm.Message) []llm.Message {
	if len(messages) == 0 {
		return messages
	}
	lastAssistant := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.EqualFold(strings.TrimSpace(string(messages[i].Role)), "assistant") {
			lastAssistant = i
			break
		}
	}
	if lastAssistant >= 0 {
		// With previous_response_id, assistant tool_calls are already tracked by
		// the prior response. Re-sending that assistant message is redundant and
		// can massively inflate request bodies after large tool outputs.
		if len(messages[lastAssistant].ToolCalls) > 0 {
			if lastAssistant+1 >= len(messages) {
				return nil
			}
			return messages[lastAssistant+1:]
		}
		if lastAssistant+1 >= len(messages) {
			return nil
		}
		return messages[lastAssistant:]
	}
	return messages[len(messages)-1:]
}

func marshalResponsesRequestWithSizeGuard(out bridgeResponsesRequest) ([]byte, error) {
	encoded, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	if len(encoded) <= maxResponsesRequestBytes || len(out.Input) == 0 {
		return encoded, nil
	}

	working := out
	working.Input = compactResponsesInputForSize(working.Input, maxResponsesInputTextBytes, maxResponsesFunctionArgsBytes, maxResponsesFunctionOutputBytes)
	encoded, err = json.Marshal(working)
	if err != nil {
		return nil, err
	}
	if len(encoded) <= maxResponsesRequestBytes {
		slog.Warn("[bridge] responses payload compacted for size",
			"bytes", len(encoded),
			"strategy", "truncate_input")
		return encoded, nil
	}

	if strings.TrimSpace(out.PreviousResponseID) != "" {
		working.Input = keepLatestResponsesContinuationInput(working.Input, maxResponsesContinuationOutputsKeep)
		working.Input = compactResponsesInputForSize(working.Input, maxResponsesInputTextBytes, maxResponsesFunctionArgsBytes, maxResponsesFunctionOutputTight)
		encoded, err = json.Marshal(working)
		if err != nil {
			return nil, err
		}
		if len(encoded) <= maxResponsesRequestBytes {
			slog.Warn("[bridge] responses payload compacted for size",
				"bytes", len(encoded),
				"strategy", "keep_latest_continuation")
			return encoded, nil
		}

		working.Input = keepLatestResponsesContinuationInput(working.Input, 1)
		working.Input = compactResponsesInputForSize(working.Input, 320, 320, maxResponsesFunctionOutputEmergency)
		encoded, err = json.Marshal(working)
		if err != nil {
			return nil, err
		}
		if len(encoded) <= maxResponsesRequestBytes {
			slog.Warn("[bridge] responses payload compacted for size",
				"bytes", len(encoded),
				"strategy", "emergency_continuation")
			return encoded, nil
		}
	}

	working.Tools = stripResponsesToolDescriptions(working.Tools)
	encoded, err = json.Marshal(working)
	if err != nil {
		return nil, err
	}
	if len(encoded) > maxResponsesRequestBytes {
		slog.Warn("[bridge] responses payload still large after compaction",
			"bytes", len(encoded),
			"limit", maxResponsesRequestBytes)
	}
	return encoded, nil
}

func compactResponsesInputForSize(input []interface{}, textCap, argsCap, outputCap int) []interface{} {
	if len(input) == 0 {
		return input
	}
	out := make([]interface{}, 0, len(input))
	for _, item := range input {
		switch v := item.(type) {
		case bridgeResponsesInputMessage:
			parts := make([]bridgeResponsesContentPart, 0, len(v.Content))
			for _, p := range v.Content {
				np := p
				if np.Type == "input_text" {
					np.Text = truncateUTF8Bytes(np.Text, textCap)
				}
				parts = append(parts, np)
			}
			v.Content = parts
			out = append(out, v)
		case bridgeResponsesFunctionCall:
			v.Arguments = truncateUTF8Bytes(v.Arguments, argsCap)
			out = append(out, v)
		case bridgeResponsesFunctionCallOutput:
			v.Output = truncateUTF8Bytes(v.Output, outputCap)
			out = append(out, v)
		default:
			out = append(out, item)
		}
	}
	return out
}

func keepLatestResponsesContinuationInput(input []interface{}, maxOutputs int) []interface{} {
	if len(input) == 0 {
		return input
	}
	if maxOutputs < 1 {
		maxOutputs = 1
	}

	outputIdx := make([]int, 0, len(input))
	lastMsgIdx := -1
	for i, item := range input {
		switch item.(type) {
		case bridgeResponsesFunctionCallOutput:
			outputIdx = append(outputIdx, i)
		case bridgeResponsesInputMessage:
			lastMsgIdx = i
		}
	}

	keep := map[int]struct{}{}
	if lastMsgIdx >= 0 {
		keep[lastMsgIdx] = struct{}{}
	}
	start := len(outputIdx) - maxOutputs
	if start < 0 {
		start = 0
	}
	for _, idx := range outputIdx[start:] {
		keep[idx] = struct{}{}
	}

	if len(keep) == 0 {
		return []interface{}{input[len(input)-1]}
	}

	out := make([]interface{}, 0, len(keep))
	for i, item := range input {
		if _, ok := keep[i]; ok {
			out = append(out, item)
		}
	}
	return out
}

func stripResponsesToolDescriptions(tools []bridgeResponsesTool) []bridgeResponsesTool {
	if len(tools) == 0 {
		return tools
	}
	out := make([]bridgeResponsesTool, 0, len(tools))
	for _, t := range tools {
		t.Description = ""
		out = append(out, t)
	}
	return out
}

func truncateUTF8Bytes(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	const suffix = "\n[truncated]"
	if maxBytes <= len(suffix) {
		return suffix[:maxBytes]
	}
	budget := maxBytes - len(suffix)
	cut := 0
	for _, r := range s {
		size := utf8.RuneLen(r)
		if size <= 0 {
			size = 1
		}
		if cut+size > budget {
			break
		}
		cut += size
	}
	if cut <= 0 {
		return suffix[:maxBytes]
	}
	return s[:cut] + suffix
}

func extractResponsesInstructions(messages []llm.Message) (string, []llm.Message) {
	if len(messages) == 0 {
		return "", messages
	}
	var instructions []string
	consumeUntil := 0
	for i, m := range messages {
		role := strings.ToLower(strings.TrimSpace(string(m.Role)))
		if role != "system" && role != "developer" {
			break
		}
		if m.Content != "" {
			instructions = append(instructions, m.Content)
		}
		consumeUntil = i + 1
	}
	if len(instructions) == 0 {
		return "", messages
	}
	return strings.Join(instructions, "\n\n"), messages[consumeUntil:]
}

func normalizeResponsesInputRole(role string) string {
	switch role {
	case "system":
		return "system"
	case "developer":
		return "developer"
	case "assistant":
		return "assistant"
	default:
		return "user"
	}
}

// ParseChatResponse parses OpenAI-format JSON into llm.ChatResponse.
func ParseChatResponse(body []byte) (*llm.ChatResponse, error) {
	if resp, handled, err := parseResponsesChatResponse(body); handled {
		return resp, err
	}

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

	if chunk, done, handled, err := parseResponsesSSEChunk(dataPayload); handled {
		return chunk, done, err
	}

	var resp bridgeResponse
	if err := json.Unmarshal([]byte(dataPayload), &resp); err != nil {
		return llm.StreamChunk{}, false, err
	}
	if resp.Error != nil {
		msg := strings.TrimSpace(resp.Error.Message)
		if msg == "" {
			if typ := strings.TrimSpace(resp.Error.Type); typ != "" {
				msg = "upstream error: type=" + typ
			} else {
				msg = "upstream error"
			}
		}
		return llm.StreamChunk{
			ID:    resp.ID,
			Model: resp.Model,
			Done:  true,
			Error: msg,
		}, true, nil
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

func parseResponsesChatResponse(body []byte) (*llm.ChatResponse, bool, error) {
	var probe struct {
		Object string          `json:"object"`
		Output json.RawMessage `json:"output"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return nil, false, nil
	}
	if probe.Object != "response" && len(probe.Output) == 0 {
		return nil, false, nil
	}

	var resp bridgeResponsesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, true, fmt.Errorf("parse responses response: %w", err)
	}
	if resp.Error != nil {
		errType := resp.Error.Type
		if errType == "" {
			errType = "unknown"
		}
		return nil, true, fmt.Errorf("upstream error: type=%s", errType)
	}

	cr := &llm.ChatResponse{
		ID:    resp.ID,
		Model: resp.Model,
		Message: llm.Message{
			Role: llm.RoleAssistant,
		},
		Usage: llm.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	var content strings.Builder
	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			if item.Role != "" {
				cr.Message.Role = llm.Role(item.Role)
			}
			for _, part := range item.Content {
				if part.Type == "output_text" || part.Type == "text" || part.Type == "input_text" {
					content.WriteString(part.Text)
				}
			}
		case "function_call":
			callID := item.CallID
			if callID == "" {
				callID = item.ID
			}
			cr.Message.ToolCalls = append(cr.Message.ToolCalls, llm.ToolCall{
				ID:        callID,
				Name:      item.Name,
				Arguments: rawToString(item.Arguments),
			})
		}
	}
	if content.Len() == 0 {
		content.WriteString(extractResponsesOutputText(resp.Output))
	}
	cr.Message.Content = content.String()
	return cr, true, nil
}

func parseResponsesSSEChunk(dataPayload string) (llm.StreamChunk, bool, bool, error) {
	var event bridgeResponsesEvent
	if err := json.Unmarshal([]byte(dataPayload), &event); err != nil {
		return llm.StreamChunk{}, false, false, nil
	}
	if event.Type == "" {
		return llm.StreamChunk{}, false, false, nil
	}

	chunk := llm.StreamChunk{}
	if event.ResponseID != "" {
		chunk.ID = event.ResponseID
	}
	if event.Response != nil {
		if chunk.ID == "" {
			chunk.ID = event.Response.ID
		}
		chunk.Model = event.Response.Model
	}

	switch event.Type {
	case "response.output_text.delta":
		chunk.Delta = event.Delta
		return chunk, false, true, nil
	case "response.output_item.added":
		if event.Item != nil && event.Item.Type == "function_call" {
			callID := event.Item.CallID
			if callID == "" {
				callID = event.Item.ID
			}
			chunk.ToolCalls = []llm.ToolCall{
				{
					ID:        callID,
					Name:      event.Item.Name,
					Arguments: rawToString(event.Item.Arguments),
				},
			}
			return chunk, false, true, nil
		}
		chunk.Progress = event.Type
		return chunk, false, true, nil
	case "response.function_call_arguments.delta":
		if event.Delta != "" {
			chunk.ToolCalls = []llm.ToolCall{{Arguments: event.Delta}}
			return chunk, false, true, nil
		}
		chunk.Progress = event.Type
		return chunk, false, true, nil
	case "response.function_call_arguments.done":
		args := strings.TrimSpace(event.Arguments)
		if args == "" && event.Item != nil {
			args = rawToString(event.Item.Arguments)
		}
		if args != "" {
			chunk.ToolCalls = []llm.ToolCall{{Arguments: args}}
			return chunk, false, true, nil
		}
		chunk.Progress = event.Type
		return chunk, false, true, nil
	case "response.output_item.done":
		if event.Item != nil && event.Item.Type == "function_call" {
			callID := event.Item.CallID
			if callID == "" {
				callID = event.Item.ID
			}
			chunk.ToolCalls = []llm.ToolCall{
				{
					ID:        callID,
					Name:      event.Item.Name,
					Arguments: rawToString(event.Item.Arguments),
				},
			}
			return chunk, false, true, nil
		}
		chunk.Progress = event.Type
		return chunk, false, true, nil
	case "response.completed":
		chunk.Done = true
		if event.Response != nil {
			chunk.Delta = extractResponsesOutputText(event.Response.Output)
			chunk.ToolCalls = extractResponsesOutputToolCalls(event.Response.Output)
			u := event.Response.Usage
			if u.InputTokens > 0 || u.OutputTokens > 0 || u.TotalTokens > 0 {
				chunk.Usage = &llm.Usage{
					PromptTokens:     u.InputTokens,
					CompletionTokens: u.OutputTokens,
					TotalTokens:      u.TotalTokens,
				}
			}
		}
		return chunk, true, true, nil
	case "response.failed":
		chunk.Done = true
		if event.Error != nil {
			if event.Error.Message != "" {
				chunk.Error = event.Error.Message
			} else if event.Error.Type != "" {
				chunk.Error = "upstream error: type=" + event.Error.Type
			}
		}
		if chunk.Error == "" {
			chunk.Error = "upstream response failed"
		}
		return chunk, true, true, nil
	default:
		// Preserve metadata events as progress so downstream can surface status and
		// avoid misclassifying metadata-only streams as "zero chunks".
		chunk.Progress = event.Type
		return chunk, false, true, nil
	}
}

func extractResponsesOutputText(output []bridgeResponsesOutputItem) string {
	if len(output) == 0 {
		return ""
	}
	var content strings.Builder
	for _, item := range output {
		if item.Type != "message" {
			continue
		}
		for _, part := range item.Content {
			if part.Type == "output_text" || part.Type == "text" || part.Type == "input_text" {
				content.WriteString(part.Text)
			}
		}
	}
	return content.String()
}

func extractResponsesOutputToolCalls(output []bridgeResponsesOutputItem) []llm.ToolCall {
	if len(output) == 0 {
		return nil
	}
	out := make([]llm.ToolCall, 0, 2)
	for _, item := range output {
		if item.Type != "function_call" {
			continue
		}
		callID := item.CallID
		if callID == "" {
			callID = item.ID
		}
		if callID == "" || item.Name == "" {
			continue
		}
		out = append(out, llm.ToolCall{
			ID:        callID,
			Name:      item.Name,
			Arguments: rawToString(item.Arguments),
		})
	}
	return out
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
