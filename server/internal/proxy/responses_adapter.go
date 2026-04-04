package proxy

import (
	gojson "encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/toolschema"
)

// openAIChatRequestForResponses captures the subset of OpenAI chat-completions
// fields we need to map into the Responses API.
type openAIChatRequestForResponses struct {
	Model               string                          `json:"model"`
	Messages            []openAIChatMessageForResponses `json:"messages"`
	Tools               []openAIChatToolForResponses    `json:"tools,omitempty"`
	Stream              bool                            `json:"stream,omitempty"`
	MaxTokens           int                             `json:"max_tokens,omitempty"`
	MaxCompletionTokens int                             `json:"max_completion_tokens,omitempty"`
	Temperature         *float64                        `json:"temperature,omitempty"`
	TopP                *float64                        `json:"top_p,omitempty"`
	ToolChoice          gojson.RawMessage               `json:"tool_choice,omitempty"`
	ParallelToolCalls   *bool                           `json:"parallel_tool_calls,omitempty"`
	ResponseFormat      gojson.RawMessage               `json:"response_format,omitempty"`
	PreviousResponseID  string                          `json:"previous_response_id,omitempty"`
	Instructions        string                          `json:"instructions,omitempty"`
	User                string                          `json:"user,omitempty"`
	SafetyIdentifier    string                          `json:"safety_identifier,omitempty"`
	PromptCacheKey      string                          `json:"prompt_cache_key,omitempty"`
	Metadata            gojson.RawMessage               `json:"metadata,omitempty"`
	ReasoningEffort     string                          `json:"reasoning_effort,omitempty"`
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
	Model              string              `json:"model,omitempty"`
	Store              bool                `json:"store"`
	Input              []interface{}       `json:"input,omitempty"`
	Tools              []responsesTool     `json:"tools,omitempty"`
	Stream             bool                `json:"stream,omitempty"`
	MaxOutputTokens    int                 `json:"max_output_tokens,omitempty"`
	Temperature        *float64            `json:"temperature,omitempty"`
	TopP               *float64            `json:"top_p,omitempty"`
	ToolChoice         interface{}         `json:"tool_choice,omitempty"`
	ParallelToolCalls  *bool               `json:"parallel_tool_calls,omitempty"`
	Text               *responsesText      `json:"text,omitempty"`
	Instructions       string              `json:"instructions,omitempty"`
	PreviousResponseID string              `json:"previous_response_id,omitempty"`
	User               string              `json:"user,omitempty"`
	SafetyIdentifier   string              `json:"safety_identifier,omitempty"`
	PromptCacheKey     string              `json:"prompt_cache_key,omitempty"`
	Metadata           interface{}         `json:"metadata,omitempty"`
	Reasoning          *responsesReasoning `json:"reasoning,omitempty"`
}

type responsesText struct {
	Format interface{} `json:"format,omitempty"`
}

type responsesReasoning struct {
	Effort string `json:"effort,omitempty"`
}

type responsesInputMessage struct {
	Role    string                      `json:"role"`
	Content []responsesInputContentPart `json:"content"`
}

type responsesInputContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Audio    any    `json:"input_audio,omitempty"`
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
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	Refusal string `json:"refusal,omitempty"`
}

type chatAudioTranscriber func(inputAudio any) (string, bool)

// convertOpenAIChatCompletionsToResponses converts an OpenAI chat-completions
// request body into an OpenAI Responses API request body.
func convertOpenAIChatCompletionsToResponses(body []byte) ([]byte, error) {
	return convertOpenAIChatCompletionsToResponsesWithAudioTranscriber(body, nil)
}

func convertOpenAIChatCompletionsToResponsesWithAudioTranscriber(body []byte, audioTranscriber chatAudioTranscriber) ([]byte, error) {
	var in openAIChatRequestForResponses
	if err := gojson.Unmarshal(body, &in); err != nil {
		return nil, fmt.Errorf("parse openai chat request: %w", err)
	}

	out := responsesRequestForOpenAI{
		Model:  in.Model,
		Stream: in.Stream,
	}
	// Preserve explicit store preference if client sent one; otherwise default to true.
	// This lets applyStorePolicy distinguish "converter-set" from "client-set" later.
	out.Store = true // default; will be overridden if client explicitly set store

	// Prefer max_completion_tokens when both fields are present.
	if in.MaxCompletionTokens > 0 {
		out.MaxOutputTokens = in.MaxCompletionTokens
	} else if in.MaxTokens > 0 {
		out.MaxOutputTokens = in.MaxTokens
	}
	if in.Temperature != nil {
		out.Temperature = in.Temperature
	}
	if in.TopP != nil {
		out.TopP = in.TopP
	}
	if parsedToolChoice, ok := decodeRawJSONValue(in.ToolChoice); ok {
		out.ToolChoice = parsedToolChoice
	}
	if in.ParallelToolCalls != nil {
		out.ParallelToolCalls = in.ParallelToolCalls
	}
	if textCfg := convertChatResponseFormatToResponsesText(in.ResponseFormat); textCfg != nil {
		out.Text = textCfg
	}
	if in.PreviousResponseID != "" {
		out.PreviousResponseID = in.PreviousResponseID
	}
	if strings.TrimSpace(in.Instructions) != "" {
		out.Instructions = strings.TrimSpace(in.Instructions)
	}
	if strings.TrimSpace(in.User) != "" {
		out.User = strings.TrimSpace(in.User)
	}
	if strings.TrimSpace(in.SafetyIdentifier) != "" {
		out.SafetyIdentifier = strings.TrimSpace(in.SafetyIdentifier)
	}
	if strings.TrimSpace(in.PromptCacheKey) != "" {
		out.PromptCacheKey = strings.TrimSpace(in.PromptCacheKey)
	}
	if metadata, ok := decodeRawJSONValue(in.Metadata); ok {
		out.Metadata = metadata
	}
	if reasoningEffort := normalizeReasoningEffort(in.ReasoningEffort); reasoningEffort != "" {
		out.Reasoning = &responsesReasoning{Effort: reasoningEffort}
	}

	if len(in.Tools) > 0 {
		out.Tools = make([]responsesTool, 0, len(in.Tools))
		for _, t := range in.Tools {
			// Preserve all tool types (function, image_generation, computer_use, etc.)
			// Only skip anonymous or empty tool definitions
			if t.Type == "" && t.Function.Name == "" {
				continue
			}
			// For function tools, map to Responses API function type
			if t.Type == "function" || t.Type == "" {
				if t.Function.Name == "" {
					continue
				}
				out.Tools = append(out.Tools, responsesTool{
					Type:        "function",
					Name:        t.Function.Name,
					Description: t.Function.Description,
					Parameters:  toolschema.NormalizeForOpenAICompat(t.Function.Parameters),
					Strict:      t.Function.Strict,
				})
			} else {
				// Preserve non-function tool types as-is (e.g., image_generation, computer_use)
				out.Tools = append(out.Tools, responsesTool{
					Type:        t.Type,
					Name:        t.Function.Name,
					Description: t.Function.Description,
					Parameters:  toolschema.NormalizeForOpenAICompat(t.Function.Parameters),
					Strict:      t.Function.Strict,
				})
			}
		}
	}

	trimmedMessages := in.Messages
	if in.PreviousResponseID != "" {
		trimmedMessages = trimMessagesForContinuation(in.Messages)
	}

	out.Input = make([]interface{}, 0, len(trimmedMessages)+2)
	for msgIdx, m := range trimmedMessages {
		role := strings.ToLower(strings.TrimSpace(m.Role))
		switch role {
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
			parts := convertChatContentToResponsesParts(m.Content, audioTranscriber)
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

	out.Input = sanitizeResponsesInput(out.Input)

	converted, err := gojson.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshal responses request: %w", err)
	}
	return converted, nil
}

func clampPositiveIntJSONField(body []byte, field string, maxAllowed int) []byte {
	if maxAllowed <= 0 {
		return body
	}
	if len(body) == 0 {
		return body
	}
	valueResult := gjson.GetBytes(body, field)
	if !valueResult.Exists() {
		return body
	}
	value := int(valueResult.Int())
	if value <= 0 || value <= maxAllowed {
		return body
	}
	out, err := sjson.SetBytes(body, field, maxAllowed)
	if err != nil {
		return body
	}
	return out
}

// clampResponsesMaxOutputTokens caps max_output_tokens only when a positive
// model-level limit is provided. Unknown model limits remain passthrough.
func clampResponsesMaxOutputTokens(body []byte, maxAllowed int) []byte {
	return clampPositiveIntJSONField(body, "max_output_tokens", maxAllowed)
}

// clampChatCompletionsMaxTokens caps chat-completions output token fields when
// a model-level limit is known. Unknown model limits remain passthrough.
// It also ensures max_completion_tokens is set from max_tokens so that both
// legacy models (which use max_tokens) and newer models (which require
// max_completion_tokens) are served correctly.
func clampChatCompletionsMaxTokens(body []byte, maxAllowed int) []byte {
	// Ensure max_completion_tokens is populated from max_tokens for
	// forward compatibility with OpenAI models that reject max_tokens.
	body = ensureMaxCompletionTokens(body)
	body = clampPositiveIntJSONField(body, "max_tokens", maxAllowed)
	body = clampPositiveIntJSONField(body, "max_completion_tokens", maxAllowed)
	return body
}

// ensureMaxCompletionTokens converts max_tokens to max_completion_tokens.
// OpenAI's gpt-5.4 series rejects max_tokens entirely and requires
// max_completion_tokens. Unlike older models that ignore the new field,
// gpt-5.4 fails if max_tokens is present at all — so we must remove it.
func ensureMaxCompletionTokens(body []byte) []byte {
	if len(body) == 0 {
		return body
	}
	maxTokens := gjson.GetBytes(body, "max_tokens")
	maxCompletionTokens := gjson.GetBytes(body, "max_completion_tokens")
	if maxTokens.Exists() && !maxCompletionTokens.Exists() {
		// gpt-5.4 fails if max_tokens is present at all, so we must remove it
		// after copying the value to max_completion_tokens.
		out, _ := sjson.SetBytes(body, "max_completion_tokens", maxTokens.Int())
		out, _ = sjson.DeleteBytes(out, "max_tokens")
		return out
	}
	// If max_tokens exists alongside max_completion_tokens, remove max_tokens
	// as it will cause gpt-5.4 to reject the request.
	if maxTokens.Exists() && maxCompletionTokens.Exists() {
		out, _ := sjson.DeleteBytes(body, "max_tokens")
		return out
	}
	return body
}

const fixedCodexResponsesEndpointPath = "/backend-api/codex/responses"

// applyResponsesStorePolicy applies endpoint-specific store policy and returns
// both transformed body and the policy label for observability.
// It respects the client's explicit store preference when set, and sets the
// appropriate default when not specified. For stateless requests (store:false),
// it also injects include: ["reasoning.encrypted_content"] per OpenAI spec.
// The originalBody param is the request body before conversion, used to detect
// whether the client explicitly set the store field.
func applyResponsesStorePolicy(body []byte, finalPath string, originalBody []byte) ([]byte, string) {
	if normalizeResponsesEndpointPath(finalPath) == fixedCodexResponsesEndpointPath {
		return applyStorePolicy(body, false, originalBody)
	}
	return applyStorePolicy(body, true, originalBody)
}

func normalizeResponsesEndpointPath(path string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(path)), "/")
}

// applyStorePolicy sets the store policy, respecting client preference when explicitly set.
// When store=false (stateless), also injects include: ["reasoning.encrypted_content"].
// originalBody is used to detect whether the client explicitly set store (before conversion
// may have added a default value).
func applyStorePolicy(body []byte, defaultVal bool, originalBody []byte) ([]byte, string) {
	if len(body) == 0 {
		return body, "unchanged"
	}
	// Check original body for explicit client store preference
	clientStoreExplicit := gjson.GetBytes(originalBody, "store")

	if clientStoreExplicit.Exists() {
		// Client explicitly set store - respect their choice
		if clientStoreExplicit.Bool() {
			return body, "client_store_true"
		}
		// Client explicitly set store:false - inject include for stateless mode
		return injectIncludeForStateless(body)
	}
	// No explicit store preference in original request - apply default
	if defaultVal {
		out, err := sjson.SetBytes(body, "store", true)
		if err != nil {
			return body, "error"
		}
		return out, "default_store_true"
	}
	return injectIncludeForStateless(body)
}

// injectIncludeForStateless adds include: ["reasoning.encrypted_content"] for
// stateless (store:false) requests per OpenAI API spec.
func injectIncludeForStateless(body []byte) ([]byte, string) {
	// First set store:false
	out, err := sjson.SetBytes(body, "store", false)
	if err != nil {
		return body, "error"
	}
	// Inject include if not already present
	include := gjson.GetBytes(out, "include")
	if !include.Exists() {
		outStr, err := sjson.SetBytes(out, "include", []string{"reasoning.encrypted_content"})
		if err != nil {
			return body, "error"
		}
		return []byte(outStr), "store_false_with_include"
	}
	return out, "store_false"
}

// ensureResponsesStoreEnabled keeps legacy behavior for generic /responses
// endpoints by forcing store=true.
func ensureResponsesStoreEnabled(body []byte) []byte {
	out, _ := sjson.SetBytes(body, "store", true)
	return out
}

// trimMessagesForContinuation keeps only incremental messages when previous_response_id is set.
func trimMessagesForContinuation(messages []openAIChatMessageForResponses) []openAIChatMessageForResponses {
	if len(messages) == 0 {
		return messages
	}

	lastAssistant := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.EqualFold(strings.TrimSpace(messages[i].Role), "assistant") {
			lastAssistant = i
			break
		}
	}

	if lastAssistant >= 0 {
		// Tool rounds already carry explicit function_call / function_call_output
		// items. Re-sending the assistant tool_call message causes redundant echo.
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

	// No assistant message in payload: keep only the latest turn as incremental input.
	return messages[len(messages)-1:]
}

// convertResponsesToOpenAIChatCompletions converts an OpenAI Responses API
// response body into an OpenAI chat-completions response body.
// preferredModel should come from routing/request metadata, not upstream response payload.
func convertResponsesToOpenAIChatCompletions(body []byte, preferredModel string) ([]byte, error) {
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
		Model:   strings.TrimSpace(preferredModel),
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
	case "system":
		return "system"
	case "developer":
		return "developer"
	case "assistant":
		return "assistant"
	case "tool":
		return "assistant"
	default:
		return "user"
	}
}

func convertChatContentToResponsesParts(raw gojson.RawMessage, audioTranscriber chatAudioTranscriber) []responsesInputContentPart {
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
			if p, ok := convertChatContentPart(part, audioTranscriber); ok {
				parts = append(parts, p)
			}
		}
		return parts
	}

	var asObject map[string]interface{}
	if err := gojson.Unmarshal(raw, &asObject); err == nil {
		if p, ok := convertChatContentPart(asObject, audioTranscriber); ok {
			return []responsesInputContentPart{p}
		}
	}

	return []responsesInputContentPart{{Type: "input_text", Text: string(raw)}}
}

func convertChatContentPart(part map[string]interface{}, audioTranscriber chatAudioTranscriber) (responsesInputContentPart, bool) {
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
		detail := ""
		switch v := part["image_url"].(type) {
		case string:
			imageURL = v
		case map[string]interface{}:
			imageURL = anyToString(v["url"])
			detail = anyToString(v["detail"])
		}
		if imageURL == "" {
			imageURL = anyToString(part["url"])
		}
		// Also check top-level detail field (Responses API native format)
		if detail == "" {
			detail = anyToString(part["detail"])
		}
		if imageURL == "" {
			return responsesInputContentPart{}, false
		}
		return responsesInputContentPart{Type: "input_image", ImageURL: imageURL, Detail: detail}, true
	case "input_audio":
		audio, ok := part["input_audio"]
		if !ok || audio == nil {
			return responsesInputContentPart{}, false
		}
		if audioTranscriber != nil {
			if text, ok := audioTranscriber(audio); ok {
				if strings.TrimSpace(text) == "" {
					return responsesInputContentPart{}, false
				}
				return responsesInputContentPart{Type: "input_text", Text: text}, true
			}
		}
		return responsesInputContentPart{Type: "input_audio", Audio: audio}, true
	default:
		text := anyToString(part["text"])
		if text == "" {
			return responsesInputContentPart{}, false
		}
		return responsesInputContentPart{Type: "input_text", Text: text}, true
	}
}

func extractTextFromChatContent(raw gojson.RawMessage) string {
	parts := convertChatContentToResponsesParts(raw, nil)
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

func decodeRawJSONValue(raw gojson.RawMessage) (interface{}, bool) {
	rawStr := strings.TrimSpace(string(raw))
	if rawStr == "" || rawStr == "null" {
		return nil, false
	}
	var out interface{}
	if err := gojson.Unmarshal(raw, &out); err != nil {
		return nil, false
	}
	return out, true
}

func convertChatResponseFormatToResponsesText(raw gojson.RawMessage) *responsesText {
	parsed, ok := decodeRawJSONValue(raw)
	if !ok {
		return nil
	}
	formatObj, ok := parsed.(map[string]interface{})
	if !ok || formatObj == nil {
		return nil
	}

	formatType := strings.ToLower(strings.TrimSpace(anyToString(formatObj["type"])))
	switch formatType {
	case "text", "json_object":
		return &responsesText{
			Format: map[string]interface{}{"type": formatType},
		}
	case "json_schema":
		// Chat Completions nests schema details under `response_format.json_schema`.
		if nested, ok := formatObj["json_schema"].(map[string]interface{}); ok && nested != nil {
			return buildResponsesJSONSchemaTextConfig(nested)
		}
		return buildResponsesJSONSchemaTextConfig(formatObj)
	default:
		return nil
	}
}

func buildResponsesJSONSchemaTextConfig(obj map[string]interface{}) *responsesText {
	name := strings.TrimSpace(anyToString(obj["name"]))
	schema, hasSchema := obj["schema"]
	if name == "" || !hasSchema || schema == nil {
		return nil
	}

	format := map[string]interface{}{
		"type":   "json_schema",
		"name":   name,
		"schema": schema,
	}
	if description := strings.TrimSpace(anyToString(obj["description"])); description != "" {
		format["description"] = description
	}
	if strict, ok := obj["strict"].(bool); ok {
		format["strict"] = strict
	}
	return &responsesText{Format: format}
}

func normalizeReasoningEffort(effort string) string {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "none", "minimal", "low", "medium", "high", "xhigh":
		return strings.ToLower(strings.TrimSpace(effort))
	case "extrahigh", "extra_high", "extra-high":
		return "xhigh"
	default:
		return ""
	}
}

func sanitizeResponsesInput(items []interface{}) []interface{} {
	if len(items) == 0 {
		return nil
	}
	out := make([]interface{}, 0, len(items))
	for _, item := range items {
		switch v := item.(type) {
		case responsesInputMessage:
			msg := sanitizeResponsesInputMessage(v)
			if len(msg.Content) == 0 {
				continue
			}
			out = append(out, msg)
		case responsesFunctionCallItem:
			if strings.TrimSpace(v.Type) == "" {
				v.Type = "function_call"
			}
			if v.Type != "function_call" || strings.TrimSpace(v.Name) == "" {
				continue
			}
			out = append(out, v)
		case responsesFunctionCallOutputItem:
			if strings.TrimSpace(v.Type) == "" {
				v.Type = "function_call_output"
			}
			if v.Type != "function_call_output" || strings.TrimSpace(v.CallID) == "" {
				continue
			}
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func sanitizeResponsesInputMessage(msg responsesInputMessage) responsesInputMessage {
	msg.Role = normalizeInputRole(strings.ToLower(strings.TrimSpace(msg.Role)))
	msg.Content = sanitizeResponsesContentParts(msg.Content)
	return msg
}

func sanitizeResponsesContentParts(parts []responsesInputContentPart) []responsesInputContentPart {
	if len(parts) == 0 {
		return nil
	}
	out := make([]responsesInputContentPart, 0, len(parts))
	for _, p := range parts {
		partType := strings.TrimSpace(p.Type)
		switch partType {
		case "input_text":
			if strings.TrimSpace(p.Text) == "" {
				continue
			}
			out = append(out, responsesInputContentPart{Type: "input_text", Text: p.Text})
		case "input_image":
			if strings.TrimSpace(p.ImageURL) == "" {
				continue
			}
			out = append(out, responsesInputContentPart{Type: "input_image", ImageURL: p.ImageURL})
		case "input_audio":
			if p.Audio == nil {
				continue
			}
			out = append(out, responsesInputContentPart{Type: "input_audio", Audio: p.Audio})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
