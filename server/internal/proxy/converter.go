package proxy

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tidwall/gjson"
)

// scannerBufPool reuses 64KB buffers for SSE line scanning.
var scannerBufPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 64*1024)
		return &buf
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
	Role       string        `json:"role"`
	Content    interface{}   `json:"content"` // string or []ContentPart
	Name       string        `json:"name,omitempty"`
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

type OpenAITool struct {
	Type     string              `json:"type"`
	Function OpenAIToolFunction  `json:"function"`
}

type OpenAIToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

type OpenAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type OpenAIChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      OpenAIMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type OpenAIStreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role      string `json:"role,omitempty"`
			Content   string `json:"content,omitempty"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id,omitempty"`
				Type     string `json:"type,omitempty"`
				Function struct {
					Name      string `json:"name,omitempty"`
					Arguments string `json:"arguments,omitempty"`
				} `json:"function,omitempty"`
			} `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

// Anthropic request/response types
type AnthropicRequest struct {
	Model         string             `json:"model"`
	Messages      []AnthropicMessage `json:"messages"`
	System        string             `json:"system,omitempty"`
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
}

type AnthropicTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema interface{} `json:"input_schema"`
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

type AnthropicStreamEvent struct {
	Type         string `json:"type"`
	Index        int    `json:"index,omitempty"`
	ContentBlock *struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
		ID   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
	} `json:"content_block,omitempty"`
	Delta *struct {
		Type        string `json:"type"`
		Text        string `json:"text,omitempty"`
		PartialJSON string `json:"partial_json,omitempty"`
		StopReason  string `json:"stop_reason,omitempty"`
	} `json:"delta,omitempty"`
	Message *AnthropicResponse `json:"message,omitempty"`
	Usage   *struct {
		InputTokens  int `json:"input_tokens,omitempty"`
		OutputTokens int `json:"output_tokens,omitempty"`
	} `json:"usage,omitempty"`
}

// Gemini request/response types
type GeminiRequest struct {
	Contents         []GeminiContent        `json:"contents"`
	SystemInstruction *GeminiContent        `json:"systemInstruction,omitempty"`
	GenerationConfig *GeminiGenerationConfig `json:"generationConfig,omitempty"`
	Tools            []GeminiTool           `json:"tools,omitempty"`
	SafetySettings   []GeminiSafetySetting  `json:"safetySettings,omitempty"`
}

type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text         string            `json:"text,omitempty"`
	InlineData   *GeminiInlineData `json:"inlineData,omitempty"`
	FunctionCall *GeminiFunctionCall `json:"functionCall,omitempty"`
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
	Candidates     []GeminiCandidate `json:"candidates"`
	UsageMetadata  *GeminiUsageMetadata `json:"usageMetadata,omitempty"`
	ModelVersion   string `json:"modelVersion,omitempty"`
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

// ConvertRequest converts OpenAI request to target provider format
func (fc *FormatConverter) ConvertRequest(body []byte, targetType ProviderType) ([]byte, string, error) {
	if targetType == ProviderTypeOpenAI {
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
	var systemContent string
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			// Extract system message
			if content, ok := msg.Content.(string); ok {
				if systemContent != "" {
					systemContent += "\n\n"
				}
				systemContent += content
			}
			continue
		}

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
		}

		// Handle tool calls in assistant messages
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			var blocks []AnthropicContentBlock
			// Add text content if present
			if content, ok := msg.Content.(string); ok && content != "" {
				blocks = append(blocks, AnthropicContentBlock{
					Type: "text",
					Text: content,
				})
			}
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

		// Handle tool results
		if msg.Role == "tool" {
			anthropicMsg.Role = "user"
			anthropicMsg.Content = []AnthropicContentBlock{{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
				Content:   fmt.Sprintf("%v", msg.Content),
			}}
		}

		anthropicReq.Messages = append(anthropicReq.Messages, anthropicMsg)
	}

	anthropicReq.System = systemContent

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

// convertContentPart converts OpenAI content part to Anthropic format
func (fc *FormatConverter) convertContentPart(part map[string]interface{}) *AnthropicContentBlock {
	partType, _ := part["type"].(string)

	switch partType {
	case "text":
		text, _ := part["text"].(string)
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
	if sourceType == ProviderTypeOpenAI {
		return body, nil
	}

	if sourceType == ProviderTypeGemini {
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
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Model:   resp.ModelVersion,
		Created: time.Now().Unix(),
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
					ID:   fmt.Sprintf("call_%d", time.Now().UnixNano()),
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
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
	if sourceType == ProviderTypeOpenAI {
		return body, nil
	}

	if sourceType == ProviderTypeGemini {
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
			"created":  time.Now().Unix(),
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
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
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
		Index        int         `json:"index"`
		Message      OpenAIMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
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
	if sourceType == ProviderTypeOpenAI {
		_, err := io.Copy(writer, reader)
		return err
	}

	if sourceType == ProviderTypeGemini {
		return fc.convertGeminiStream(reader, writer)
	}

	// Anthropic streaming conversion
	return fc.convertAnthropicStream(reader, writer)
}

// convertGeminiStream converts Gemini SSE to OpenAI SSE
func (fc *FormatConverter) convertGeminiStream(reader io.Reader, writer http.ResponseWriter) error {
	flusher, ok := writer.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	scanner, bufPtr := newPooledScanner(reader)
	defer scannerBufPool.Put(bufPtr)
	messageID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())

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

	return &OpenAIStreamChunk{
		ID:     messageID,
		Object: "chat.completion.chunk",
		Choices: []struct {
			Index int `json:"index"`
			Delta struct {
				Role      string `json:"role,omitempty"`
				Content   string `json:"content,omitempty"`
				ToolCalls []struct {
					Index    int    `json:"index"`
					ID       string `json:"id,omitempty"`
					Type     string `json:"type,omitempty"`
					Function struct {
						Name      string `json:"name,omitempty"`
						Arguments string `json:"arguments,omitempty"`
					} `json:"function,omitempty"`
				} `json:"tool_calls,omitempty"`
			} `json:"delta"`
			FinishReason string `json:"finish_reason,omitempty"`
		}{{
			Index: 0,
			Delta: struct {
				Role      string `json:"role,omitempty"`
				Content   string `json:"content,omitempty"`
				ToolCalls []struct {
					Index    int    `json:"index"`
					ID       string `json:"id,omitempty"`
					Type     string `json:"type,omitempty"`
					Function struct {
						Name      string `json:"name,omitempty"`
						Arguments string `json:"arguments,omitempty"`
					} `json:"function,omitempty"`
				} `json:"tool_calls,omitempty"`
			}{Content: content},
			FinishReason: finishReason,
		}},
	}
}

// convertAnthropicStream converts Anthropic SSE to OpenAI SSE.
// Uses gjson for field extraction and template-based JSON output to avoid
// json.Unmarshal + json.Marshal per chunk (the hot path for streaming).
func (fc *FormatConverter) convertAnthropicStream(reader io.Reader, writer http.ResponseWriter) error {
	flusher, ok := writer.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	scanner, bufPtr := newPooledScanner(reader)
	defer scannerBufPool.Put(bufPtr)

	var messageID, model string
	// Pre-allocate write buffer for SSE lines
	var wb bytes.Buffer
	wb.Grow(512)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 || !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}

		data := line[6:] // skip "data: "
		eventType := gjson.GetBytes(data, "type").Str

		switch eventType {
		case "message_start":
			messageID = gjson.GetBytes(data, "message.id").Str
			model = gjson.GetBytes(data, "message.model").Str
			wb.Reset()
			wb.WriteString(`data: {"id":"`)
			wb.WriteString(messageID)
			wb.WriteString(`","object":"chat.completion.chunk","model":"`)
			wb.WriteString(model)
			wb.WriteString(`","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`)
			wb.WriteString("\n\n")
			writer.Write(wb.Bytes())
			flusher.Flush()

		case "content_block_delta":
			deltaType := gjson.GetBytes(data, "delta.type").Str
			if deltaType != "text_delta" {
				continue
			}
			text := gjson.GetBytes(data, "delta.text").Str
			wb.Reset()
			wb.WriteString(`data: {"id":"`)
			wb.WriteString(messageID)
			wb.WriteString(`","object":"chat.completion.chunk","model":"`)
			wb.WriteString(model)
			wb.WriteString(`","choices":[{"index":0,"delta":{"content":`)
			// JSON-encode the text content (handles escaping)
			escapedText, _ := json.Marshal(text)
			wb.Write(escapedText)
			wb.WriteString(`},"finish_reason":null}]}`)
			wb.WriteString("\n\n")
			writer.Write(wb.Bytes())
			flusher.Flush()

		case "message_delta":
			stopReason := gjson.GetBytes(data, "delta.stop_reason").Str
			finishReason := "null"
			switch stopReason {
			case "end_turn":
				finishReason = `"stop"`
			case "max_tokens":
				finishReason = `"length"`
			case "tool_use":
				finishReason = `"tool_calls"`
			}
			wb.Reset()
			wb.WriteString(`data: {"id":"`)
			wb.WriteString(messageID)
			wb.WriteString(`","object":"chat.completion.chunk","model":"`)
			wb.WriteString(model)
			wb.WriteString(`","choices":[{"index":0,"delta":{},"finish_reason":`)
			wb.WriteString(finishReason)
			wb.WriteString(`}]`)
			// Include usage if present
			usage := gjson.GetBytes(data, "usage")
			if usage.Exists() {
				inputTokens := usage.Get("input_tokens").Int()
				outputTokens := usage.Get("output_tokens").Int()
				wb.WriteString(`,"usage":{"prompt_tokens":`)
				wb.Write(appendInt(nil, inputTokens))
				wb.WriteString(`,"completion_tokens":`)
				wb.Write(appendInt(nil, outputTokens))
				wb.WriteString(`,"total_tokens":`)
				wb.Write(appendInt(nil, inputTokens+outputTokens))
				wb.WriteByte('}')
			}
			wb.WriteByte('}')
			wb.WriteString("\n\n")
			writer.Write(wb.Bytes())
			flusher.Flush()

		case "message_stop":
			writer.Write([]byte("data: [DONE]\n\n"))
			flusher.Flush()
			return scanner.Err()
		}
	}
	return scanner.Err()
}

// appendInt appends an int64 as decimal to dst without fmt.Sprintf.
func appendInt(dst []byte, v int64) []byte {
	return strconv.AppendInt(dst, v, 10)
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
			Choices: []struct {
				Index int `json:"index"`
				Delta struct {
					Role      string `json:"role,omitempty"`
					Content   string `json:"content,omitempty"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id,omitempty"`
						Type     string `json:"type,omitempty"`
						Function struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						} `json:"function,omitempty"`
					} `json:"tool_calls,omitempty"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason,omitempty"`
			}{{
				Index: 0,
				Delta: struct {
					Role      string `json:"role,omitempty"`
					Content   string `json:"content,omitempty"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id,omitempty"`
						Type     string `json:"type,omitempty"`
						Function struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						} `json:"function,omitempty"`
					} `json:"tool_calls,omitempty"`
				}{
					Role: "assistant",
				},
			}},
		}

	case "content_block_delta":
		if event.Delta == nil {
			return nil
		}
		content := ""
		if event.Delta.Type == "text_delta" {
			content = event.Delta.Text
		}
		return &OpenAIStreamChunk{
			ID:     *messageID,
			Object: "chat.completion.chunk",
			Model:  *model,
			Choices: []struct {
				Index int `json:"index"`
				Delta struct {
					Role      string `json:"role,omitempty"`
					Content   string `json:"content,omitempty"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id,omitempty"`
						Type     string `json:"type,omitempty"`
						Function struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						} `json:"function,omitempty"`
					} `json:"tool_calls,omitempty"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason,omitempty"`
			}{{
				Index: 0,
				Delta: struct {
					Role      string `json:"role,omitempty"`
					Content   string `json:"content,omitempty"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id,omitempty"`
						Type     string `json:"type,omitempty"`
						Function struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						} `json:"function,omitempty"`
					} `json:"tool_calls,omitempty"`
				}{
					Content: content,
				},
			}},
		}

	case "message_delta":
		finishReason := ""
		if event.Delta != nil {
			// Map stop reason — Claude sends stop_reason in delta, not type
			stopReason := event.Delta.StopReason
			if stopReason == "" {
				stopReason = event.Delta.Type // fallback for older format
			}
			switch stopReason {
			case "end_turn":
				finishReason = "stop"
			case "max_tokens":
				finishReason = "length"
			case "tool_use":
				finishReason = "tool_calls"
			}
		}
		chunk := &OpenAIStreamChunk{
			ID:     *messageID,
			Object: "chat.completion.chunk",
			Model:  *model,
			Choices: []struct {
				Index int `json:"index"`
				Delta struct {
					Role      string `json:"role,omitempty"`
					Content   string `json:"content,omitempty"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id,omitempty"`
						Type     string `json:"type,omitempty"`
						Function struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						} `json:"function,omitempty"`
					} `json:"tool_calls,omitempty"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason,omitempty"`
			}{{
				Index:        0,
				FinishReason: finishReason,
			}},
		}
		if event.Usage != nil {
			chunk.Usage = &struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     event.Usage.InputTokens,
				CompletionTokens: event.Usage.OutputTokens,
				TotalTokens:      event.Usage.InputTokens + event.Usage.OutputTokens,
			}
		}
		return chunk
	}

	return nil
}

// WrapRequestBody wraps the request body with format conversion
func (fc *FormatConverter) WrapRequestBody(body []byte, targetType ProviderType) (io.Reader, string, error) {
	converted, path, err := fc.ConvertRequest(body, targetType)
	if err != nil {
		return nil, "", err
	}
	return bytes.NewReader(converted), path, nil
}
