package proxy

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	gojson "github.com/goccy/go-json"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// scannerBufPool reuses 64KB buffers for SSE line scanning.
var scannerBufPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 64*1024)
		return &buf
	},
}

// sseWriteBufPool reuses bytes.Buffer for SSE chunk assembly.
var sseWriteBufPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 512))
	},
}

// newPooledScanner creates a bufio.Scanner with a pooled buffer.
// Caller must return the buffer via scannerBufPool.Put after scanning is done.
func newPooledScanner(r io.Reader) (*bufio.Scanner, *[]byte) {
	bufPtr := scannerBufPool.Get().(*[]byte)
	s := bufio.NewScanner(r)
	s.Buffer(*bufPtr, 256*1024) // 64KB initial, 256KB max for large tool_call JSON
	return s, bufPtr
}

// ProviderType identifies the API format type
type ProviderType string

const (
	ProviderTypeOpenAI    ProviderType = "openai"
	ProviderTypeAnthropic ProviderType = "anthropic"
	ProviderTypeGemini    ProviderType = "gemini"
)

// FormatConverter handles conversion between OpenAI and Anthropic API formats
type FormatConverter struct{}

// NewFormatConverter creates a new format converter
func NewFormatConverter() *FormatConverter {
	return &FormatConverter{}
}

// sharedConverter is a package-level singleton — FormatConverter is stateless.
var sharedConverter = &FormatConverter{}

// DetectProviderType detects the provider type from endpoint URL
func (fc *FormatConverter) DetectProviderType(endpoint string) ProviderType {
	if strings.Contains(endpoint, "anthropic") {
		return ProviderTypeAnthropic
	}
	if strings.Contains(endpoint, "generativelanguage.googleapis.com") || strings.Contains(endpoint, "aiplatform.googleapis.com") {
		return ProviderTypeGemini
	}
	return ProviderTypeOpenAI
}

// OpenAI request/response types
type OpenAIChatRequest struct {
	Model            string                 `json:"model"`
	Messages         []OpenAIMessage        `json:"messages"`
	MaxTokens        int                    `json:"max_tokens,omitempty"`
	Temperature      float64                `json:"temperature,omitempty"`
	TopP             float64                `json:"top_p,omitempty"`
	Stream           bool                   `json:"stream,omitempty"`
	Stop             []string               `json:"stop,omitempty"`
	PresencePenalty  float64                `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64                `json:"frequency_penalty,omitempty"`
	Tools            []OpenAITool           `json:"tools,omitempty"`
	ToolChoice       interface{}            `json:"tool_choice,omitempty"`
	ResponseFormat   map[string]interface{} `json:"response_format,omitempty"`
}

type OpenAIMessage struct {
	Role       string           `json:"role"`
	Content    interface{}      `json:"content"` // string or []ContentPart
	Name       string           `json:"name,omitempty"`
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type OpenAITool struct {
	Type     string             `json:"type"`
	Function OpenAIToolFunction `json:"function"`
}

type OpenAIToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

type OpenAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function OpenAIToolCallFunc `json:"function"`
}

// OpenAIToolCallFunc is the function part of an OpenAI tool call.
// Arguments is normally a JSON string, but some providers send it as a JSON object.
type OpenAIToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// UnmarshalJSON handles Arguments being either a JSON string or a JSON object.
func (f *OpenAIToolCallFunc) UnmarshalJSON(data []byte) error {
	// Use an alias to avoid infinite recursion.
	type alias struct {
		Name      string            `json:"name"`
		Arguments gojson.RawMessage `json:"arguments"`
	}
	var raw alias
	if err := gojson.Unmarshal(data, &raw); err != nil {
		return err
	}
	f.Name = raw.Name
	if len(raw.Arguments) > 0 && raw.Arguments[0] == '"' {
		// It's a JSON string — unmarshal to get the actual string value.
		return gojson.Unmarshal(raw.Arguments, &f.Arguments)
	}
	// It's a JSON object (or other non-string) — keep as raw JSON string.
	f.Arguments = string(raw.Arguments)
	return nil
}

type OpenAIChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int           `json:"index"`
		Message      OpenAIMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// StreamChunkDelta is the delta object inside an OpenAI streaming chunk choice.
type StreamChunkDelta struct {
	Role      string                `json:"role,omitempty"`
	Content   string                `json:"content,omitempty"`
	ToolCalls []StreamChunkToolCall `json:"tool_calls,omitempty"`
}

// StreamChunkToolCall is a single tool call inside a streaming delta.
type StreamChunkToolCall struct {
	Index    int                     `json:"index"`
	ID       string                  `json:"id,omitempty"`
	Type     string                  `json:"type,omitempty"`
	Function StreamChunkToolCallFunc `json:"function,omitempty"`
}

// StreamChunkToolCallFunc is the function part of a streaming tool call.
type StreamChunkToolCallFunc struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// StreamChunkChoice is a single choice in an OpenAI streaming chunk.
type StreamChunkChoice struct {
	Index        int              `json:"index"`
	Delta        StreamChunkDelta `json:"delta"`
	FinishReason *string          `json:"finish_reason"`
}

// StreamChunkUsage is the usage object in an OpenAI streaming chunk.
type StreamChunkUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAIStreamChunk struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Created int64               `json:"created,omitempty"`
	Model   string              `json:"model,omitempty"`
	Choices []StreamChunkChoice `json:"choices"`
	Usage   *StreamChunkUsage   `json:"usage,omitempty"`
}

// Anthropic request/response types
type AnthropicRequest struct {
	Model         string             `json:"model"`
	Messages      []AnthropicMessage `json:"messages"`
	System        interface{}        `json:"system,omitempty"` // string or []AnthropicSystemBlock
	MaxTokens     int                `json:"max_tokens"`
	Temperature   float64            `json:"temperature,omitempty"`
	TopP          float64            `json:"top_p,omitempty"`
	TopK          int                `json:"top_k,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	Tools         []AnthropicTool    `json:"tools,omitempty"`
	ToolChoice    interface{}        `json:"tool_choice,omitempty"`
}

type AnthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []ContentBlock
}

type AnthropicContentBlock struct {
	Type      string      `json:"type"`
	Text      string      `json:"text,omitempty"`
	ID        string      `json:"id,omitempty"`
	Name      string      `json:"name,omitempty"`
	Input     interface{} `json:"input,omitempty"`
	ToolUseID string      `json:"tool_use_id,omitempty"`
	Content   string      `json:"content,omitempty"`
	Source    *struct {
		Type      string `json:"type"`
		MediaType string `json:"media_type"`
		Data      string `json:"data"`
	} `json:"source,omitempty"`
	CacheControl *AnthropicCacheControl `json:"cache_control,omitempty"`
}

type AnthropicTool struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`
	InputSchema  interface{}            `json:"input_schema"`
	CacheControl *AnthropicCacheControl `json:"cache_control,omitempty"`
}

// AnthropicCacheControl is the cache_control block for Anthropic prompt caching.
type AnthropicCacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

// AnthropicSystemBlock is a content block in the system prompt array (for prompt caching).
type AnthropicSystemBlock struct {
	Type         string                 `json:"type"`
	Text         string                 `json:"text"`
	CacheControl *AnthropicCacheControl `json:"cache_control,omitempty"`
}

type AnthropicResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Content      []AnthropicContentBlock `json:"content"`
	Model        string                  `json:"model"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence string                  `json:"stop_sequence,omitempty"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// AnthropicStreamContentBlock is the content_block in a content_block_start event.
type AnthropicStreamContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// AnthropicStreamDelta is the delta in content_block_delta / message_delta events.
type AnthropicStreamDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

// AnthropicStreamUsage is the usage in message_start / message_delta events.
type AnthropicStreamUsage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
}

type AnthropicStreamEvent struct {
	Type         string                       `json:"type"`
	Index        int                          `json:"index,omitempty"`
	ContentBlock *AnthropicStreamContentBlock `json:"content_block,omitempty"`
	Delta        *AnthropicStreamDelta        `json:"delta,omitempty"`
	Message      *AnthropicResponse           `json:"message,omitempty"`
	Usage        *AnthropicStreamUsage        `json:"usage,omitempty"`
}

// Gemini request/response types
type GeminiRequest struct {
	Contents          []GeminiContent         `json:"contents"`
	SystemInstruction *GeminiContent          `json:"systemInstruction,omitempty"`
	GenerationConfig  *GeminiGenerationConfig `json:"generationConfig,omitempty"`
	Tools             []GeminiTool            `json:"tools,omitempty"`
	SafetySettings    []GeminiSafetySetting   `json:"safetySettings,omitempty"`
}

type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text             string                  `json:"text,omitempty"`
	InlineData       *GeminiInlineData       `json:"inlineData,omitempty"`
	FunctionCall     *GeminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *GeminiFunctionResponse `json:"functionResponse,omitempty"`
}

type GeminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type GeminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

type GeminiFunctionResponse struct {
	Name     string                 `json:"name"`
	Response map[string]interface{} `json:"response"`
}

type GeminiGenerationConfig struct {
	Temperature     float64  `json:"temperature,omitempty"`
	TopP            float64  `json:"topP,omitempty"`
	TopK            int      `json:"topK,omitempty"`
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type GeminiTool struct {
	FunctionDeclarations []GeminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

type GeminiFunctionDeclaration struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

type GeminiSafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

type GeminiResponse struct {
	Candidates    []GeminiCandidate    `json:"candidates"`
	UsageMetadata *GeminiUsageMetadata `json:"usageMetadata,omitempty"`
	ModelVersion  string               `json:"modelVersion,omitempty"`
}

type GeminiCandidate struct {
	Content       GeminiContent `json:"content"`
	FinishReason  string        `json:"finishReason,omitempty"`
	SafetyRatings []struct {
		Category    string `json:"category"`
		Probability string `json:"probability"`
	} `json:"safetyRatings,omitempty"`
}

type GeminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type GeminiStreamChunk struct {
	Candidates    []GeminiCandidate    `json:"candidates,omitempty"`
	UsageMetadata *GeminiUsageMetadata `json:"usageMetadata,omitempty"`
}

// GeminiModelsResponse represents Gemini models list response
type GeminiModelsResponse struct {
	Models []GeminiModel `json:"models"`
}

type GeminiModel struct {
	Name                       string   `json:"name"`
	Version                    string   `json:"version,omitempty"`
	DisplayName                string   `json:"displayName,omitempty"`
	Description                string   `json:"description,omitempty"`
	InputTokenLimit            int      `json:"inputTokenLimit,omitempty"`
	OutputTokenLimit           int      `json:"outputTokenLimit,omitempty"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods,omitempty"`
}

// ProviderTypeCloudCode identifies Google Cloud Code Assist API format
const ProviderTypeCloudCode ProviderType = "cloudcode"

// ProviderTypeCopilot identifies GitHub Copilot API format
const ProviderTypeCopilot ProviderType = "copilot"

// ConvertRequest converts OpenAI request to target provider format
func (fc *FormatConverter) ConvertRequest(body []byte, targetType ProviderType) ([]byte, string, error) {
	if targetType == ProviderTypeOpenAI || targetType == ProviderTypeCopilot {
		// Copilot uses OpenAI-compatible format
		return body, "/v1/chat/completions", nil
	}

	// Parse OpenAI request
	var openaiReq OpenAIChatRequest
	if err := json.Unmarshal(body, &openaiReq); err != nil {
		return nil, "", fmt.Errorf("failed to parse OpenAI request: %w", err)
	}

	if targetType == ProviderTypeGemini {
		return fc.convertToGemini(openaiReq)
	}

	// Convert to Anthropic format
	anthropicReq := fc.openAIToAnthropic(openaiReq)
	converted, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal Anthropic request: %w", err)
	}
	return converted, "/v1/messages", nil
}

// ConvertRequestWithCaching converts OpenAI request to target provider format and
// optionally applies prompt caching in a single unmarshal/marshal pass.
// This eliminates the double unmarshal/marshal that ConvertRequest + InjectPromptCaching does.
func (fc *FormatConverter) ConvertRequestWithCaching(body []byte, targetType ProviderType, promptCacheEnabled bool) ([]byte, string, error) {
	if targetType == ProviderTypeOpenAI || targetType == ProviderTypeCopilot {
		return body, "/v1/chat/completions", nil
	}

	var openaiReq OpenAIChatRequest
	if err := json.Unmarshal(body, &openaiReq); err != nil {
		return nil, "", fmt.Errorf("failed to parse OpenAI request: %w", err)
	}

	if targetType == ProviderTypeGemini {
		return fc.convertToGemini(openaiReq)
	}

	// Convert to Anthropic struct (no marshal yet)
	anthropicReq := fc.openAIToAnthropic(openaiReq)

	// Apply prompt caching on the struct directly — avoids second unmarshal/marshal
	if promptCacheEnabled {
		ApplyPromptCaching(&anthropicReq)
	}

	converted, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal Anthropic request: %w", err)
	}
	return converted, "/v1/messages", nil
}

// InjectPromptCaching unmarshals an Anthropic request body, applies cache breakpoints, and re-marshals.
func InjectPromptCaching(body []byte) ([]byte, error) {
	var req AnthropicRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return body, err
	}
	ApplyPromptCaching(&req)
	return json.Marshal(req)
}

// convertToGemini converts OpenAI request to Gemini format
func (fc *FormatConverter) convertToGemini(req OpenAIChatRequest) ([]byte, string, error) {
	geminiReq := GeminiRequest{
		GenerationConfig: &GeminiGenerationConfig{
			Temperature:     req.Temperature,
			TopP:            req.TopP,
			MaxOutputTokens: req.MaxTokens,
			StopSequences:   req.Stop,
		},
	}

	// Convert messages
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			// System instruction
			if content, ok := msg.Content.(string); ok {
				geminiReq.SystemInstruction = &GeminiContent{
					Parts: []GeminiPart{{Text: content}},
				}
			}
			continue
		}

		geminiContent := GeminiContent{
			Role: fc.convertRoleToGemini(msg.Role),
		}

		// Convert content
		switch content := msg.Content.(type) {
		case string:
			geminiContent.Parts = []GeminiPart{{Text: content}}
		case []interface{}:
			for _, part := range content {
				if partMap, ok := part.(map[string]interface{}); ok {
					geminiPart := fc.convertContentPartToGemini(partMap)
					if geminiPart != nil {
						geminiContent.Parts = append(geminiContent.Parts, *geminiPart)
					}
				}
			}
		}

		// Handle tool calls
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				var args map[string]interface{}
				json.Unmarshal([]byte(tc.Function.Arguments), &args)
				geminiContent.Parts = append(geminiContent.Parts, GeminiPart{
					FunctionCall: &GeminiFunctionCall{
						Name: tc.Function.Name,
						Args: args,
					},
				})
			}
		}

		// Handle tool results
		if msg.Role == "tool" {
			geminiContent.Role = "function"
			geminiContent.Parts = []GeminiPart{{
				FunctionResponse: &GeminiFunctionResponse{
					Name:     msg.Name,
					Response: map[string]interface{}{"result": msg.Content},
				},
			}}
		}

		geminiReq.Contents = append(geminiReq.Contents, geminiContent)
	}

	// Convert tools
	if len(req.Tools) > 0 {
		var funcDecls []GeminiFunctionDeclaration
		for _, tool := range req.Tools {
			if tool.Type == "function" {
				funcDecls = append(funcDecls, GeminiFunctionDeclaration{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  tool.Function.Parameters,
				})
			}
		}
		if len(funcDecls) > 0 {
			geminiReq.Tools = []GeminiTool{{FunctionDeclarations: funcDecls}}
		}
	}

	converted, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal Gemini request: %w", err)
	}

	// Build path with model name
	model := fc.convertModelToGemini(req.Model)
	action := "generateContent"
	if req.Stream {
		action = "streamGenerateContent?alt=sse"
	}
	path := fmt.Sprintf("/v1beta/models/%s:%s", model, action)

	return converted, path, nil
}

func (fc *FormatConverter) convertRoleToGemini(role string) string {
	switch role {
	case "assistant":
		return "model"
	case "user":
		return "user"
	default:
		return role
	}
}

func (fc *FormatConverter) convertContentPartToGemini(part map[string]interface{}) *GeminiPart {
	partType, _ := part["type"].(string)
	switch partType {
	case "text":
		text, _ := part["text"].(string)
		return &GeminiPart{Text: text}
	case "image_url":
		if imageURL, ok := part["image_url"].(map[string]interface{}); ok {
			url, _ := imageURL["url"].(string)
			if strings.HasPrefix(url, "data:") {
				parts := strings.SplitN(url, ",", 2)
				if len(parts) == 2 {
					mimeType := strings.TrimPrefix(strings.Split(parts[0], ";")[0], "data:")
					return &GeminiPart{
						InlineData: &GeminiInlineData{
							MimeType: mimeType,
							Data:     parts[1],
						},
					}
				}
			}
		}
	}
	return nil
}

func (fc *FormatConverter) convertModelToGemini(model string) string {
	modelMap := map[string]string{
		"gpt-4":         "gemini-1.5-pro",
		"gpt-4-turbo":   "gemini-1.5-pro",
		"gpt-4o":        "gemini-1.5-pro",
		"gpt-4o-mini":   "gemini-1.5-flash",
		"gpt-3.5-turbo": "gemini-1.5-flash",
	}
	if mapped, ok := modelMap[model]; ok {
		return mapped
	}
	// Remove "models/" prefix if present
	return strings.TrimPrefix(model, "models/")
}

// openAIToAnthropic converts OpenAI request to Anthropic format
func (fc *FormatConverter) openAIToAnthropic(req OpenAIChatRequest) AnthropicRequest {
	anthropicReq := AnthropicRequest{
		Model:       fc.convertModel(req.Model),
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
	}

	// Default max_tokens if not set (required by Anthropic)
	if anthropicReq.MaxTokens == 0 {
		anthropicReq.MaxTokens = 4096
	}

	// Convert stop sequences
	if len(req.Stop) > 0 {
		anthropicReq.StopSequences = req.Stop
	}

	// Convert messages
	var systemBlocks []AnthropicSystemBlock
	var pendingToolResults []AnthropicContentBlock
	flushPendingToolResults := func() {
		if len(pendingToolResults) == 0 {
			return
		}
		blocks := make([]AnthropicContentBlock, len(pendingToolResults))
		copy(blocks, pendingToolResults)
		anthropicReq.Messages = append(anthropicReq.Messages, AnthropicMessage{
			Role:    "user",
			Content: blocks,
		})
		pendingToolResults = pendingToolResults[:0]
	}
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			flushPendingToolResults()
			// Preserve individual system blocks to keep static/config/dynamic boundaries.
			switch content := msg.Content.(type) {
			case string:
				if content != "" {
					systemBlocks = append(systemBlocks, AnthropicSystemBlock{
						Type: "text",
						Text: content,
					})
				}
			case []interface{}:
				for _, part := range content {
					partMap, ok := part.(map[string]interface{})
					if !ok {
						continue
					}
					if partType, _ := partMap["type"].(string); partType != "" && partType != "text" {
						continue
					}
					text, _ := partMap["text"].(string)
					if text == "" {
						continue
					}
					systemBlocks = append(systemBlocks, AnthropicSystemBlock{
						Type: "text",
						Text: text,
					})
				}
			}
			continue
		}

		if msg.Role == "tool" {
			pendingToolResults = append(pendingToolResults, AnthropicContentBlock{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
				Content:   anthropicToolResultString(msg.Content),
			})
			continue
		}

		flushPendingToolResults()

		anthropicMsg := AnthropicMessage{
			Role: msg.Role,
		}

		// Convert content
		switch content := msg.Content.(type) {
		case string:
			anthropicMsg.Content = content
		case []interface{}:
			// Multi-part content (e.g., with images)
			var blocks []AnthropicContentBlock
			for _, part := range content {
				if partMap, ok := part.(map[string]interface{}); ok {
					block := fc.convertContentPart(partMap)
					if block != nil {
						blocks = append(blocks, *block)
					}
				}
			}
			if len(blocks) > 0 {
				anthropicMsg.Content = blocks
			}
		default:
			anthropicMsg.Content = anthropicToolResultString(content)
		}

		// Handle tool calls in assistant messages
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			blocks := anthropicContentBlocksFromMessageContent(anthropicMsg.Content)
			// Add tool use blocks
			for _, tc := range msg.ToolCalls {
				var input interface{}
				json.Unmarshal([]byte(tc.Function.Arguments), &input)
				blocks = append(blocks, AnthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				})
			}
			anthropicMsg.Content = blocks
		}
		anthropicMsg.Content = normalizeAnthropicMessageContent(anthropicMsg.Content)

		anthropicReq.Messages = append(anthropicReq.Messages, anthropicMsg)
	}
	flushPendingToolResults()

	if len(systemBlocks) > 0 {
		anthropicReq.System = systemBlocks
	}

	// Convert tools (native Anthropic format)
	for _, tool := range req.Tools {
		if tool.Type == "function" {
			anthropicReq.Tools = append(anthropicReq.Tools, AnthropicTool{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				InputSchema: tool.Function.Parameters,
			})
		}
	}

	return anthropicReq
}

func anthropicContentBlocksFromMessageContent(content interface{}) []AnthropicContentBlock {
	switch c := content.(type) {
	case string:
		if strings.TrimSpace(c) == "" {
			return nil
		}
		return []AnthropicContentBlock{{
			Type: "text",
			Text: c,
		}}
	case []AnthropicContentBlock:
		if len(c) == 0 {
			return nil
		}
		out := make([]AnthropicContentBlock, 0, len(c))
		out = append(out, c...)
		return out
	case []interface{}:
		if len(c) == 0 {
			return nil
		}
		out := make([]AnthropicContentBlock, 0, len(c))
		for _, raw := range c {
			block, ok := raw.(map[string]interface{})
			if !ok || block == nil {
				continue
			}
			blockType, _ := block["type"].(string)
			switch blockType {
			case "text":
				text, _ := block["text"].(string)
				if strings.TrimSpace(text) == "" {
					continue
				}
				out = append(out, AnthropicContentBlock{
					Type: "text",
					Text: text,
				})
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	default:
		return nil
	}
}

func anthropicToolResultString(content interface{}) string {
	switch v := content.(type) {
	case nil:
		return ""
	case string:
		return v
	case gojson.RawMessage:
		raw := strings.TrimSpace(string(v))
		if raw == "" || raw == "null" {
			return ""
		}
		return raw
	default:
		if encoded, err := json.Marshal(v); err == nil {
			return string(encoded)
		}
		return fmt.Sprintf("%v", v)
	}
}

func normalizeAnthropicMessageContent(content interface{}) interface{} {
	switch c := content.(type) {
	case nil:
		return ""
	case []AnthropicContentBlock:
		if len(c) == 0 {
			return ""
		}
		return c
	case []interface{}:
		if len(c) == 0 {
			return ""
		}
		return c
	default:
		return content
	}
}

// ApplyPromptCaching adds cache_control breakpoints to an Anthropic request.
// Breakpoints are placed on:
//
//	(1) the system prompt static block(s) — up to 2 blocks for static+config
//	(2) the last tool definition
//	(3) a turn-boundary message (4th-from-last) for long conversations
//
// This enables Anthropic's prompt caching, which can save up to 90% on input token costs.
// Anthropic allows up to 4 cache breakpoints per request.
func ApplyPromptCaching(req *AnthropicRequest) {
	ephemeral := &AnthropicCacheControl{Type: "ephemeral"}

	// Handle system prompt: convert string to array, or annotate existing array blocks.
	switch s := req.System.(type) {
	case string:
		if s != "" {
			req.System = []AnthropicSystemBlock{{
				Type:         "text",
				Text:         s,
				CacheControl: ephemeral,
			}}
		}
	case []AnthropicSystemBlock:
		// Multi-block system prompt (from BuildStructured).
		// Place cache_control on each non-dynamic block.
		// Convention: last block is TURN_DYNAMIC (no caching), earlier blocks are static/config.
		if len(s) == 1 {
			s[0].CacheControl = ephemeral
		} else if len(s) >= 2 {
			// Cache all blocks except the last (dynamic) one.
			// Anthropic caches the prefix up to each breakpoint.
			for i := 0; i < len(s)-1; i++ {
				s[i].CacheControl = ephemeral
			}
			// Last block (TURN_DYNAMIC) gets no cache_control.
		}
		req.System = s
	}

	// Add cache_control to the last tool (tools are sorted alphabetically for stability)
	if n := len(req.Tools); n > 0 {
		req.Tools[n-1].CacheControl = ephemeral
	}

	// Turn-boundary breakpoint: cache older conversation messages.
	// Place a breakpoint on the 4th-from-last message for conversations with 6+ messages.
	// This caches the conversation prefix while keeping recent turns uncached.
	if n := len(req.Messages); n >= 6 {
		idx := n - 4
		msg := &req.Messages[idx]
		// Ensure content is an array so we can attach cache_control.
		switch c := msg.Content.(type) {
		case string:
			if c != "" {
				msg.Content = []AnthropicContentBlock{{
					Type:         "text",
					Text:         c,
					CacheControl: ephemeral,
				}}
			}
		case []AnthropicContentBlock:
			if len(c) > 0 {
				c[len(c)-1].CacheControl = ephemeral
			}
		case []interface{}:
			// JSON-unmarshaled array — add cache_control to the last block via map.
			if len(c) > 0 {
				if lastBlock, ok := c[len(c)-1].(map[string]interface{}); ok {
					lastBlock["cache_control"] = map[string]string{"type": "ephemeral"}
				}
			}
		}
	}
}

// convertContentPart converts OpenAI content part to Anthropic format
func (fc *FormatConverter) convertContentPart(part map[string]interface{}) *AnthropicContentBlock {
	partType, _ := part["type"].(string)

	switch partType {
	case "text":
		rawText, ok := part["text"]
		if !ok || rawText == nil {
			return nil
		}
		text, ok := rawText.(string)
		if !ok {
			text = fmt.Sprintf("%v", rawText)
		}
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return &AnthropicContentBlock{
			Type: "text",
			Text: text,
		}
	case "image_url":
		if imageURL, ok := part["image_url"].(map[string]interface{}); ok {
			url, _ := imageURL["url"].(string)
			// Handle base64 data URLs
			if strings.HasPrefix(url, "data:") {
				parts := strings.SplitN(url, ",", 2)
				if len(parts) == 2 {
					mediaType := strings.TrimPrefix(strings.Split(parts[0], ";")[0], "data:")
					return &AnthropicContentBlock{
						Type: "image",
						Source: &struct {
							Type      string `json:"type"`
							MediaType string `json:"media_type"`
							Data      string `json:"data"`
						}{
							Type:      "base64",
							MediaType: mediaType,
							Data:      parts[1],
						},
					}
				}
			}
		}
	}
	return nil
}

// convertModel converts OpenAI model name to Anthropic model name
func (fc *FormatConverter) convertModel(model string) string {
	// Map common model names
	// Only map cross-family names (GPT→Claude). Claude model names are passed
	// through as-is so relay/provider config controls the actual model used.
	modelMap := map[string]string{
		"gpt-4":         "claude-3-opus-20240229",
		"gpt-4-turbo":   "claude-3-opus-20240229",
		"gpt-4o":        "claude-3-5-sonnet-20241022",
		"gpt-4o-mini":   "claude-3-5-haiku-20241022",
		"gpt-3.5-turbo": "claude-3-haiku-20240307",
	}

	if mapped, ok := modelMap[model]; ok {
		return mapped
	}
	return model
}

// ConvertResponse converts provider response to OpenAI format
func (fc *FormatConverter) ConvertResponse(body []byte, sourceType ProviderType) ([]byte, error) {
	if sourceType == ProviderTypeOpenAI || sourceType == ProviderTypeCopilot {
		return body, nil
	}

	if sourceType == ProviderTypeCloudCode || sourceType == ProviderTypeGemini {
		return fc.convertGeminiResponse(body)
	}

	var anthropicResp AnthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to parse Anthropic response: %w", err)
	}

	openaiResp := fc.anthropicToOpenAI(anthropicResp)
	return json.Marshal(openaiResp)
}

// convertGeminiResponse converts Gemini response to OpenAI format
func (fc *FormatConverter) convertGeminiResponse(body []byte) ([]byte, error) {
	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini response: %w", err)
	}

	openaiResp := fc.geminiToOpenAI(geminiResp)
	return json.Marshal(openaiResp)
}

func (fc *FormatConverter) geminiToOpenAI(resp GeminiResponse) OpenAIChatResponse {
	openaiResp := OpenAIChatResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", timeutil.NowNano()),
		Object:  "chat.completion",
		Model:   resp.ModelVersion,
		Created: timeutil.Now(),
	}

	if len(resp.Candidates) > 0 {
		candidate := resp.Candidates[0]
		var content string
		var toolCalls []OpenAIToolCall

		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				content += part.Text
			}
			if part.FunctionCall != nil {
				argsJSON, _ := json.Marshal(part.FunctionCall.Args)
				toolCalls = append(toolCalls, OpenAIToolCall{
					ID:   fmt.Sprintf("call_%d", timeutil.NowNano()),
					Type: "function",
					Function: OpenAIToolCallFunc{
						Name:      part.FunctionCall.Name,
						Arguments: string(argsJSON),
					},
				})
			}
		}

		finishReason := "stop"
		switch candidate.FinishReason {
		case "STOP":
			finishReason = "stop"
		case "MAX_TOKENS":
			finishReason = "length"
		case "SAFETY":
			finishReason = "content_filter"
		case "FUNCTION_CALL":
			finishReason = "tool_calls"
		}

		openaiResp.Choices = []struct {
			Index        int           `json:"index"`
			Message      OpenAIMessage `json:"message"`
			FinishReason string        `json:"finish_reason"`
		}{{
			Index: 0,
			Message: OpenAIMessage{
				Role:      "assistant",
				Content:   content,
				ToolCalls: toolCalls,
			},
			FinishReason: finishReason,
		}}
	}

	if resp.UsageMetadata != nil {
		openaiResp.Usage.PromptTokens = resp.UsageMetadata.PromptTokenCount
		openaiResp.Usage.CompletionTokens = resp.UsageMetadata.CandidatesTokenCount
		openaiResp.Usage.TotalTokens = resp.UsageMetadata.TotalTokenCount
	}

	return openaiResp
}

// ConvertModelsResponse converts provider models list to OpenAI format
func (fc *FormatConverter) ConvertModelsResponse(body []byte, sourceType ProviderType) ([]byte, error) {
	if sourceType == ProviderTypeOpenAI || sourceType == ProviderTypeCopilot {
		return body, nil
	}

	if sourceType == ProviderTypeGemini || sourceType == ProviderTypeCloudCode {
		return fc.convertGeminiModels(body)
	}

	// Anthropic doesn't have a models endpoint, return empty list
	return json.Marshal(map[string]interface{}{
		"object": "list",
		"data":   []interface{}{},
	})
}

func (fc *FormatConverter) convertGeminiModels(body []byte) ([]byte, error) {
	var geminiModels GeminiModelsResponse
	if err := json.Unmarshal(body, &geminiModels); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini models: %w", err)
	}

	var models []map[string]interface{}
	for _, m := range geminiModels.Models {
		// Extract model ID from name (e.g., "models/gemini-1.5-pro" -> "gemini-1.5-pro")
		modelID := strings.TrimPrefix(m.Name, "models/")
		models = append(models, map[string]interface{}{
			"id":       modelID,
			"object":   "model",
			"created":  timeutil.Now(),
			"owned_by": "google",
		})
	}

	return json.Marshal(map[string]interface{}{
		"object": "list",
		"data":   models,
	})
}

// anthropicToOpenAI converts Anthropic response to OpenAI format
func (fc *FormatConverter) anthropicToOpenAI(resp AnthropicResponse) OpenAIChatResponse {
	openaiResp := OpenAIChatResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Model:   resp.Model,
		Created: 0, // Anthropic doesn't provide this
	}

	// Convert content
	var content string
	var toolCalls []OpenAIToolCall
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			content += block.Text
		case "tool_use":
			inputJSON, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, OpenAIToolCall{
				ID:   block.ID,
				Type: "function",
				Function: OpenAIToolCallFunc{
					Name:      block.Name,
					Arguments: string(inputJSON),
				},
			})
		}
	}

	// Convert finish reason
	finishReason := "stop"
	switch resp.StopReason {
	case "end_turn":
		finishReason = "stop"
	case "max_tokens":
		finishReason = "length"
	case "tool_use":
		finishReason = "tool_calls"
	}

	openaiResp.Choices = []struct {
		Index        int           `json:"index"`
		Message      OpenAIMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	}{{
		Index: 0,
		Message: OpenAIMessage{
			Role:      "assistant",
			Content:   content,
			ToolCalls: toolCalls,
		},
		FinishReason: finishReason,
	}}

	openaiResp.Usage.PromptTokens = resp.Usage.InputTokens
	openaiResp.Usage.CompletionTokens = resp.Usage.OutputTokens
	openaiResp.Usage.TotalTokens = resp.Usage.InputTokens + resp.Usage.OutputTokens

	return openaiResp
}

// ConvertStreamingResponse creates a streaming response converter
func (fc *FormatConverter) ConvertStreamingResponse(reader io.Reader, sourceType ProviderType, writer http.ResponseWriter) error {
	if sourceType == ProviderTypeOpenAI || sourceType == ProviderTypeCopilot {
		_, err := io.Copy(writer, reader)
		return err
	}

	if sourceType == ProviderTypeGemini {
		return fc.convertGeminiStream(reader, writer)
	}

	if sourceType == ProviderTypeCloudCode {
		return fc.convertCloudCodeStream(reader, writer)
	}

	// Anthropic streaming conversion
	return fc.convertAnthropicStream(reader, writer)
}

// convertCloudCodeStream converts Cloud Code SSE to OpenAI SSE.
// Cloud Code wraps Gemini-style responses in SSE format.
func (fc *FormatConverter) convertCloudCodeStream(reader io.Reader, writer http.ResponseWriter) error {
	flusher, ok := writer.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	scanner, bufPtr := newPooledScanner(reader)
	defer scannerBufPool.Put(bufPtr)
	messageID := fmt.Sprintf("chatcmpl-%d", timeutil.NowNano())

	wb := sseWriteBufPool.Get().(*bytes.Buffer)
	defer func() {
		wb.Reset()
		sseWriteBufPool.Put(wb)
	}()

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 || !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}

		data := line[6:]
		if bytes.Equal(data, []byte("[DONE]")) {
			writer.Write([]byte("data: [DONE]\n\n"))
			flusher.Flush()
			break
		}

		// Cloud Code returns Gemini-style candidates
		var chunk GeminiStreamChunk
		wasRepaired := false
		if err := json.Unmarshal(data, &chunk); err != nil {
			fixed := repairJSON(data)
			if err2 := json.Unmarshal(fixed, &chunk); err2 != nil {
				continue
			}
			wasRepaired = true
		}

		openaiChunk := fc.convertGeminiStreamChunk(chunk, messageID)
		if openaiChunk == nil {
			continue
		}

		chunkJSON, err := json.Marshal(openaiChunk)
		if err != nil {
			continue
		}

		wb.Reset()
		wb.WriteString("data: ")
		wb.Write(chunkJSON)
		wb.WriteString("\n\n")
		writer.Write(wb.Bytes())
		flusher.Flush()

		// Don't trust finishReason from repaired chunks — truncated values
		// like "STO" would cause premature stream termination.
		if !wasRepaired && len(chunk.Candidates) > 0 && chunk.Candidates[0].FinishReason != "" {
			writer.Write([]byte("data: [DONE]\n\n"))
			flusher.Flush()
			break
		}
	}
	return scanner.Err()
}

// convertGeminiStream converts Gemini SSE to OpenAI SSE
func (fc *FormatConverter) convertGeminiStream(reader io.Reader, writer http.ResponseWriter) error {
	flusher, ok := writer.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	scanner, bufPtr := newPooledScanner(reader)
	defer scannerBufPool.Put(bufPtr)
	messageID := fmt.Sprintf("chatcmpl-%d", timeutil.NowNano())

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		var chunk GeminiStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		openaiChunk := fc.convertGeminiStreamChunk(chunk, messageID)
		if openaiChunk == nil {
			continue
		}

		chunkJSON, _ := json.Marshal(openaiChunk)
		writer.Write([]byte("data: " + string(chunkJSON) + "\n\n"))
		flusher.Flush()

		// Check for finish
		if len(chunk.Candidates) > 0 && chunk.Candidates[0].FinishReason != "" {
			writer.Write([]byte("data: [DONE]\n\n"))
			flusher.Flush()
			break
		}
	}
	return scanner.Err()
}

func (fc *FormatConverter) convertGeminiStreamChunk(chunk GeminiStreamChunk, messageID string) *OpenAIStreamChunk {
	if len(chunk.Candidates) == 0 {
		return nil
	}

	candidate := chunk.Candidates[0]
	var content string
	for _, part := range candidate.Content.Parts {
		content += part.Text
	}

	finishReason := ""
	if candidate.FinishReason != "" {
		switch candidate.FinishReason {
		case "STOP":
			finishReason = "stop"
		case "MAX_TOKENS":
			finishReason = "length"
		}
	}

	var fr *string
	if finishReason != "" {
		fr = &finishReason
	}

	return &OpenAIStreamChunk{
		ID:     messageID,
		Object: "chat.completion.chunk",
		Choices: []StreamChunkChoice{{
			Index:        0,
			Delta:        StreamChunkDelta{Content: content},
			FinishReason: fr,
		}},
	}
}

// convertAnthropicStream converts Anthropic SSE to OpenAI SSE.
// Deserializes each event into AnthropicStreamEvent, converts via convertStreamEvent,
// then serializes the OpenAI chunk. Truncated JSON is repaired before retry.
func (fc *FormatConverter) convertAnthropicStream(reader io.Reader, writer http.ResponseWriter) error {
	flusher, ok := writer.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	scanner, bufPtr := newPooledScanner(reader)
	defer scannerBufPool.Put(bufPtr)

	var messageID, model string
	// Pooled write buffer for SSE lines
	wb := sseWriteBufPool.Get().(*bytes.Buffer)
	defer func() {
		wb.Reset()
		sseWriteBufPool.Put(wb)
	}()

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 || !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}

		data := line[6:] // skip "data: "

		var event AnthropicStreamEvent
		if err := json.Unmarshal(data, &event); err != nil {
			repaired := repairJSON(data)
			if err2 := json.Unmarshal(repaired, &event); err2 != nil {
				continue // unfixable, skip
			}
			// Repaired JSON may have truncated values — only forward safe events.
			if !isRepairedEventSafe(&event) {
				continue
			}
		}

		if event.Type == "message_stop" {
			writer.Write([]byte("data: [DONE]\n\n"))
			flusher.Flush()
			return scanner.Err()
		}

		chunk := fc.convertStreamEvent(event, &messageID, &model)
		if chunk == nil {
			continue
		}

		chunkJSON, err := json.Marshal(chunk)
		if err != nil {
			continue
		}

		wb.Reset()
		wb.WriteString("data: ")
		wb.Write(chunkJSON)
		wb.WriteString("\n\n")
		writer.Write(wb.Bytes())
		flusher.Flush()
	}
	return scanner.Err()
}

// convertStreamEvent converts Anthropic stream event to OpenAI chunk
func (fc *FormatConverter) convertStreamEvent(event AnthropicStreamEvent, messageID, model *string) *OpenAIStreamChunk {
	switch event.Type {
	case "message_start":
		if event.Message != nil {
			*messageID = event.Message.ID
			*model = event.Message.Model
		}
		return &OpenAIStreamChunk{
			ID:     *messageID,
			Object: "chat.completion.chunk",
			Model:  *model,
			Choices: []StreamChunkChoice{{
				Index: 0,
				Delta: StreamChunkDelta{Role: "assistant"},
			}},
		}

	case "content_block_start":
		// tool_use block: emit chunk with tool call ID, type, and function name
		if event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" {
			return &OpenAIStreamChunk{
				ID:     *messageID,
				Object: "chat.completion.chunk",
				Model:  *model,
				Choices: []StreamChunkChoice{{
					Index: 0,
					Delta: StreamChunkDelta{
						ToolCalls: []StreamChunkToolCall{{
							Index: event.Index,
							ID:    event.ContentBlock.ID,
							Type:  "function",
							Function: StreamChunkToolCallFunc{
								Name: event.ContentBlock.Name,
							},
						}},
					},
				}},
			}
		}
		return nil

	case "content_block_delta":
		if event.Delta == nil {
			return nil
		}
		switch event.Delta.Type {
		case "text_delta":
			return &OpenAIStreamChunk{
				ID:     *messageID,
				Object: "chat.completion.chunk",
				Model:  *model,
				Choices: []StreamChunkChoice{{
					Index: 0,
					Delta: StreamChunkDelta{Content: event.Delta.Text},
				}},
			}
		case "input_json_delta":
			return &OpenAIStreamChunk{
				ID:     *messageID,
				Object: "chat.completion.chunk",
				Model:  *model,
				Choices: []StreamChunkChoice{{
					Index: 0,
					Delta: StreamChunkDelta{
						ToolCalls: []StreamChunkToolCall{{
							Index: event.Index,
							Function: StreamChunkToolCallFunc{
								Arguments: event.Delta.PartialJSON,
							},
						}},
					},
				}},
			}
		}
		return nil

	case "message_delta":
		var finishReason *string
		if event.Delta != nil {
			stopReason := event.Delta.StopReason
			if stopReason == "" {
				stopReason = event.Delta.Type // fallback for older format
			}
			var fr string
			switch stopReason {
			case "end_turn":
				fr = "stop"
			case "max_tokens":
				fr = "length"
			case "tool_use":
				fr = "tool_calls"
			}
			if fr != "" {
				finishReason = &fr
			}
		}
		chunk := &OpenAIStreamChunk{
			ID:     *messageID,
			Object: "chat.completion.chunk",
			Model:  *model,
			Choices: []StreamChunkChoice{{
				Index:        0,
				FinishReason: finishReason,
			}},
		}
		if event.Usage != nil {
			chunk.Usage = &StreamChunkUsage{
				PromptTokens:     event.Usage.InputTokens,
				CompletionTokens: event.Usage.OutputTokens,
				TotalTokens:      event.Usage.InputTokens + event.Usage.OutputTokens,
			}
		}
		return chunk
	}

	return nil
}
