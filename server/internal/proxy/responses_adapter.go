package proxy

import (
	gojson "encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// openAIChatRequestForResponses captures the subset of OpenAI chat-completions
// fields we need to map into the Responses API.
type openAIChatRequestForResponses struct {
	Model              string                          `json:"model"`
	Messages           []openAIChatMessageForResponses `json:"messages"`
	Tools              []openAIChatToolForResponses    `json:"tools,omitempty"`
	Stream             bool                            `json:"stream,omitempty"`
	MaxTokens          int                             `json:"max_tokens,omitempty"`
	Temperature        *float64                        `json:"temperature,omitempty"`
	TopP               *float64                        `json:"top_p,omitempty"`
	PreviousResponseID string                          `json:"previous_response_id,omitempty"`
}

type openAIChatMessageForResponses struct {
	Role       string                           `json:"role"`
	Content    gojson.RawMessage                `json:"content"`
	ToolCalls  []openAIChatToolCallForResponses `json:"tool_calls,omitempty"`
	ToolCallID string                           `json:"tool_call_id,omitempty"`
}

type openAIChatToolCallForResponses struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string            `json:"name"`
		Arguments gojson.RawMessage `json:"arguments"`
	} `json:"function"`
}

type openAIChatToolForResponses struct {
	Type     string `json:"type"`
	Function struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description,omitempty"`
		Parameters  map[string]interface{} `json:"parameters,omitempty"`
		Strict      bool                   `json:"strict,omitempty"`
	} `json:"function"`
}

type responsesRequestForOpenAI struct {
	Model              string          `json:"model,omitempty"`
	Input              []interface{}   `json:"input,omitempty"`
	Instructions       string          `json:"instructions,omitempty"`
	Tools              []responsesTool `json:"tools,omitempty"`
	Stream             bool            `json:"stream,omitempty"`
	MaxOutputTokens    int             `json:"max_output_tokens,omitempty"`
	Temperature        *float64        `json:"temperature,omitempty"`
	TopP               *float64        `json:"top_p,omitempty"`
	PreviousResponseID string          `json:"previous_response_id,omitempty"`
}

type responsesInputMessage struct {
	Role    string                      `json:"role"`
	Content []responsesInputContentPart `json:"content"`
}

type responsesInputContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type responsesFunctionCallItem struct {
	Type      string `json:"type"`
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type responsesFunctionCallOutputItem struct {
	Type   string `json:"type"`
	CallID string `json:"call_id,omitempty"`
	Output string `json:"output,omitempty"`
}

type responsesTool struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Strict      bool                   `json:"strict,omitempty"`
}

type responsesAPIResponse struct {
	ID        string                   `json:"id"`
	Object    string                   `json:"object"`
	CreatedAt int64                    `json:"created_at"`
	Model     string                   `json:"model"`
	Output    []responsesAPIOutputItem `json:"output,omitempty"`
	Usage     struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

type responsesAPIOutputItem struct {
	ID        string                     `json:"id,omitempty"`
	Type      string                     `json:"type"`
	Role      string                     `json:"role,omitempty"`
	Name      string                     `json:"name,omitempty"`
	CallID    string                     `json:"call_id,omitempty"`
	Arguments gojson.RawMessage          `json:"arguments,omitempty"`
	Content   []responsesAPIContentBlock `json:"content,omitempty"`
}

type responsesAPIContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// convertOpenAIChatCompletionsToResponses converts an OpenAI chat-completions
// request body into an OpenAI Responses API request body.
func convertOpenAIChatCompletionsToResponses(body []byte) ([]byte, error) {
	var in openAIChatRequestForResponses
	if err := gojson.Unmarshal(body, &in); err != nil {
		return nil, fmt.Errorf("parse openai chat request: %w", err)
	}

	out := responsesRequestForOpenAI{
		Model:  in.Model,
		Stream: in.Stream,
	}
	if in.MaxTokens > 0 {
		out.MaxOutputTokens = in.MaxTokens
	}
	if in.Temperature != nil {
		out.Temperature = in.Temperature
	}
	if in.TopP != nil {
		out.TopP = in.TopP
	}
	if in.PreviousResponseID != "" {
		out.PreviousResponseID = in.PreviousResponseID
	}

	if len(in.Tools) > 0 {
		out.Tools = make([]responsesTool, 0, len(in.Tools))
		for _, t := range in.Tools {
			if t.Type != "" && t.Type != "function" {
				continue
			}
			if t.Function.Name == "" {
				continue
			}
			out.Tools = append(out.Tools, responsesTool{
				Type:        "function",
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
				Strict:      t.Function.Strict,
			})
		}
	}

	var instructions []string
	out.Input = make([]interface{}, 0, len(in.Messages)+2)
	for msgIdx, m := range in.Messages {
		role := strings.ToLower(strings.TrimSpace(m.Role))
		switch role {
		case "system", "developer":
			text := extractTextFromChatContent(m.Content)
			if text != "" {
				instructions = append(instructions, text)
			}
		case "tool":
			if m.ToolCallID == "" {
				continue
			}
			out.Input = append(out.Input, responsesFunctionCallOutputItem{
				Type:   "function_call_output",
				CallID: m.ToolCallID,
				Output: contentRawToString(m.Content),
			})
		default:
			parts := convertChatContentToResponsesParts(m.Content)
			if len(parts) > 0 {
				out.Input = append(out.Input, responsesInputMessage{
					Role:    normalizeInputRole(role),
					Content: parts,
				})
			}
			if role == "assistant" && len(m.ToolCalls) > 0 {
				for callIdx, tc := range m.ToolCalls {
					callID := strings.TrimSpace(tc.ID)
					if callID == "" {
						callID = "call_" + strconv.Itoa(msgIdx) + "_" + strconv.Itoa(callIdx)
					}
					out.Input = append(out.Input, responsesFunctionCallItem{
						Type:      "function_call",
						CallID:    callID,
						Name:      tc.Function.Name,
						Arguments: rawJSONToString(tc.Function.Arguments),
					})
				}
			}
		}
	}

	if len(instructions) > 0 {
		out.Instructions = strings.Join(instructions, "\n\n")
	}

	converted, err := gojson.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshal responses request: %w", err)
	}
	return converted, nil
}

// convertResponsesToOpenAIChatCompletions converts an OpenAI Responses API
// response body into an OpenAI chat-completions response body.
func convertResponsesToOpenAIChatCompletions(body []byte) ([]byte, error) {
	var in responsesAPIResponse
	if err := gojson.Unmarshal(body, &in); err != nil {
		return nil, fmt.Errorf("parse responses api response: %w", err)
	}
	if in.Object != "response" {
		return nil, fmt.Errorf("unexpected object: %s", in.Object)
	}

	var content strings.Builder
	toolCalls := make([]OpenAIToolCall, 0, 2)
	for idx, item := range in.Output {
		switch item.Type {
		case "message":
			if item.Role != "" && item.Role != "assistant" {
				continue
			}
			for _, part := range item.Content {
				switch part.Type {
				case "", "output_text", "text":
					if part.Text != "" {
						content.WriteString(part.Text)
					}
				}
			}
		case "function_call":
			callID := strings.TrimSpace(item.CallID)
			if callID == "" {
				callID = strings.TrimSpace(item.ID)
			}
			if callID == "" {
				callID = "call_resp_" + strconv.Itoa(idx)
			}
			toolCalls = append(toolCalls, OpenAIToolCall{
				ID:   callID,
				Type: "function",
				Function: OpenAIToolCallFunc{
					Name:      item.Name,
					Arguments: rawJSONToString(item.Arguments),
				},
			})
		}
	}

	finishReason := "stop"
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	out := OpenAIChatResponse{
		ID:      in.ID,
		Object:  "chat.completion",
		Created: in.CreatedAt,
		Model:   in.Model,
		Choices: []struct {
			Index        int           `json:"index"`
			Message      OpenAIMessage `json:"message"`
			FinishReason string        `json:"finish_reason"`
		}{{
			Index: 0,
			Message: OpenAIMessage{
				Role:      "assistant",
				Content:   content.String(),
				ToolCalls: toolCalls,
			},
			FinishReason: finishReason,
		}},
	}
	out.Usage.PromptTokens = in.Usage.InputTokens
	out.Usage.CompletionTokens = in.Usage.OutputTokens
	if in.Usage.TotalTokens > 0 {
		out.Usage.TotalTokens = in.Usage.TotalTokens
	} else {
		out.Usage.TotalTokens = in.Usage.InputTokens + in.Usage.OutputTokens
	}

	converted, err := gojson.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshal chat completion response: %w", err)
	}
	return converted, nil
}

func normalizeInputRole(role string) string {
	switch role {
	case "assistant":
		return "assistant"
	case "tool":
		return "assistant"
	default:
		return "user"
	}
}

func convertChatContentToResponsesParts(raw gojson.RawMessage) []responsesInputContentPart {
	raw = gojson.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}

	var asString string
	if err := gojson.Unmarshal(raw, &asString); err == nil {
		if asString == "" {
			return nil
		}
		return []responsesInputContentPart{{Type: "input_text", Text: asString}}
	}

	var asArray []map[string]interface{}
	if err := gojson.Unmarshal(raw, &asArray); err == nil {
		parts := make([]responsesInputContentPart, 0, len(asArray))
		for _, part := range asArray {
			if p, ok := convertChatContentPart(part); ok {
				parts = append(parts, p)
			}
		}
		return parts
	}

	var asObject map[string]interface{}
	if err := gojson.Unmarshal(raw, &asObject); err == nil {
		if p, ok := convertChatContentPart(asObject); ok {
			return []responsesInputContentPart{p}
		}
	}

	return []responsesInputContentPart{{Type: "input_text", Text: string(raw)}}
}

func convertChatContentPart(part map[string]interface{}) (responsesInputContentPart, bool) {
	partType, _ := part["type"].(string)
	switch partType {
	case "", "text", "input_text", "output_text":
		text := anyToString(part["text"])
		if text == "" {
			return responsesInputContentPart{}, false
		}
		return responsesInputContentPart{Type: "input_text", Text: text}, true
	case "image_url", "input_image":
		imageURL := ""
		switch v := part["image_url"].(type) {
		case string:
			imageURL = v
		case map[string]interface{}:
			imageURL = anyToString(v["url"])
		}
		if imageURL == "" {
			imageURL = anyToString(part["url"])
		}
		if imageURL == "" {
			return responsesInputContentPart{}, false
		}
		return responsesInputContentPart{Type: "input_image", ImageURL: imageURL}, true
	default:
		text := anyToString(part["text"])
		if text == "" {
			return responsesInputContentPart{}, false
		}
		return responsesInputContentPart{Type: "input_text", Text: text}, true
	}
}

func extractTextFromChatContent(raw gojson.RawMessage) string {
	parts := convertChatContentToResponsesParts(raw)
	if len(parts) == 0 {
		return ""
	}
	texts := make([]string, 0, len(parts))
	for _, p := range parts {
		if p.Type == "input_text" && p.Text != "" {
			texts = append(texts, p.Text)
		}
	}
	return strings.Join(texts, "\n")
}

func contentRawToString(raw gojson.RawMessage) string {
	rawStr := strings.TrimSpace(string(raw))
	if rawStr == "" || rawStr == "null" {
		return ""
	}
	var plain string
	if err := gojson.Unmarshal(raw, &plain); err == nil {
		return plain
	}
	return rawStr
}

func rawJSONToString(raw gojson.RawMessage) string {
	rawStr := strings.TrimSpace(string(raw))
	if rawStr == "" || rawStr == "null" {
		return ""
	}
	var plain string
	if err := gojson.Unmarshal(raw, &plain); err == nil {
		return plain
	}
	return rawStr
}

func anyToString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case gojson.Number:
		return t.String()
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32)
	case int:
		return strconv.Itoa(t)
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return ""
	}
}
