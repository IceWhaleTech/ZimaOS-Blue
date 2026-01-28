package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultOpenAIBaseURL = "https://api.openai.com"
	openAITimeout        = 120 * time.Second
)

// OpenAIProvider implements the Provider interface for OpenAI.
type OpenAIProvider struct {
	apiKey       string
	baseURL      string
	client       *http.Client
	cachedModels []string
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(apiKey, baseURL string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	return &OpenAIProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: openAITimeout,
		},
	}
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// Models returns the list of available models.
func (p *OpenAIProvider) Models() []string {
	// Try to fetch models from API
	models := p.fetchModels()
	if len(models) > 0 {
		return models
	}
	// Fallback to default list if API is not available
	return []string{
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-4-turbo",
		"gpt-4",
		"gpt-3.5-turbo",
	}
}

// normalizeBaseURL ensures the base URL is properly formatted.
// It handles cases where the URL may or may not include /v1.
func (p *OpenAIProvider) normalizeBaseURL() string {
	baseURL := strings.TrimSuffix(p.baseURL, "/")
	return baseURL
}

// getAPIPath returns the full API path, handling /v1 compatibility.
// It tries with /v1 first, and if that fails, tries without.
func (p *OpenAIProvider) getAPIPath(endpoint string) string {
	baseURL := p.normalizeBaseURL()
	// If baseURL already ends with /v1, don't add it again
	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL + endpoint
	}
	return baseURL + "/v1" + endpoint
}

// fetchModels fetches the list of available models from the API.
func (p *OpenAIProvider) fetchModels() []string {
	// Use cached models if available
	if len(p.cachedModels) > 0 {
		return p.cachedModels
	}

	// Don't fetch if no API key
	if p.apiKey == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try to fetch models, handling /v1 compatibility
	models := p.tryFetchModels(ctx, p.getAPIPath("/models"))
	if models == nil {
		// If failed, try without /v1 (for endpoints that don't use it)
		baseURL := p.normalizeBaseURL()
		if !strings.HasSuffix(baseURL, "/v1") {
			models = p.tryFetchModels(ctx, baseURL+"/models")
		}
	}

	if len(models) > 0 {
		p.cachedModels = models
	}
	return models
}

// tryFetchModels attempts to fetch models from a specific URL.
func (p *OpenAIProvider) tryFetchModels(ctx context.Context, url string) []string {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}

	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		// Filter to only include chat models (exclude embeddings, etc.)
		if p.isChatModel(m.ID) {
			models = append(models, m.ID)
		}
	}

	return models
}

// isChatModel checks if a model ID is likely a chat model.
func (p *OpenAIProvider) isChatModel(modelID string) bool {
	// Include common chat model patterns
	chatPatterns := []string{
		"gpt-", "chatgpt-", "o1-", "o3-",
		"claude-", "llama", "mistral", "mixtral",
		"qwen", "deepseek", "gemma", "phi",
	}
	modelLower := strings.ToLower(modelID)
	for _, pattern := range chatPatterns {
		if strings.Contains(modelLower, pattern) {
			return true
		}
	}
	// Exclude known non-chat models
	excludePatterns := []string{
		"embedding", "embed-", "whisper", "tts-",
		"dall-e", "davinci", "babbage", "ada",
		"moderation", "text-",
	}
	for _, pattern := range excludePatterns {
		if strings.Contains(modelLower, pattern) {
			return false
		}
	}
	// Default to including unknown models
	return true
}

// RefreshModels clears the cached models and fetches fresh list.
// Falls back to default models if API fetch fails.
func (p *OpenAIProvider) RefreshModels() []string {
	p.cachedModels = nil
	models := p.fetchModels()
	if len(models) > 0 {
		return models
	}
	// Fallback to default list if API is not available
	return []string{
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-4-turbo",
		"gpt-4",
		"gpt-3.5-turbo",
	}
}

// openAIRequest represents the OpenAI API request format.
type openAIRequest struct {
	Model       string              `json:"model"`
	Messages    []openAIMessage     `json:"messages"`
	Temperature float64             `json:"temperature,omitempty"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Tools       []openAITool        `json:"tools,omitempty"`
	Stream      bool                `json:"stream,omitempty"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// openAIResponse represents the OpenAI API response format.
type openAIResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role       string           `json:"role"`
			Content    string           `json:"content"`
			ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
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

// Chat sends a chat completion request to OpenAI.
func (p *OpenAIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Convert to OpenAI format
	openAIReq := p.convertRequest(req)

	// Marshal request
	body, err := json.Marshal(openAIReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.getAPIPath("/chat/completions"), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	// Send request
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	var openAIResp openAIResponse
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for API error
	if openAIResp.Error != nil {
		return nil, fmt.Errorf("OpenAI API error: %s", openAIResp.Error.Message)
	}

	// Check for HTTP error
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI API returned status %d", resp.StatusCode)
	}

	// Convert response
	return p.convertResponse(openAIResp), nil
}

// ChatStream sends a streaming chat completion request to OpenAI.
func (p *OpenAIProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	// Set stream flag
	req.Stream = true
	openAIReq := p.convertRequest(req)

	// Marshal request
	body, err := json.Marshal(openAIReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.getAPIPath("/chat/completions"), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	// Send request
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Check for HTTP error
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("OpenAI API returned status %d", resp.StatusCode)
	}

	// Create channel for streaming
	ch := make(chan StreamChunk, 100)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		decoder := json.NewDecoder(resp.Body)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Read SSE data
			var chunk struct {
				ID      string `json:"id"`
				Model   string `json:"model"`
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
				Usage *struct {
					PromptTokens     int `json:"prompt_tokens"`
					CompletionTokens int `json:"completion_tokens"`
					TotalTokens      int `json:"total_tokens"`
				} `json:"usage,omitempty"`
			}

			if err := decoder.Decode(&chunk); err != nil {
				if err == io.EOF {
					return
				}
				// Skip invalid JSON (like "data: [DONE]")
				continue
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			streamChunk := StreamChunk{
				ID:    chunk.ID,
				Model: chunk.Model,
				Delta: chunk.Choices[0].Delta.Content,
				Done:  chunk.Choices[0].FinishReason == "stop",
			}

			if chunk.Usage != nil {
				streamChunk.Usage = &Usage{
					PromptTokens:     chunk.Usage.PromptTokens,
					CompletionTokens: chunk.Usage.CompletionTokens,
					TotalTokens:      chunk.Usage.TotalTokens,
				}
			}

			select {
			case <-ctx.Done():
				return
			case ch <- streamChunk:
			}

			if streamChunk.Done {
				return
			}
		}
	}()

	return ch, nil
}

// convertRequest converts a ChatRequest to OpenAI format.
func (p *OpenAIProvider) convertRequest(req ChatRequest) openAIRequest {
	messages := make([]openAIMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = openAIMessage{
			Role:       string(msg.Role),
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}
		if len(msg.ToolCalls) > 0 {
			messages[i].ToolCalls = make([]openAIToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				messages[i].ToolCalls[j] = openAIToolCall{
					ID:   tc.ID,
					Type: "function",
				}
				messages[i].ToolCalls[j].Function.Name = tc.Name
				messages[i].ToolCalls[j].Function.Arguments = tc.Arguments
			}
		}
	}

	openAIReq := openAIRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
	}

	if len(req.Tools) > 0 {
		openAIReq.Tools = make([]openAITool, len(req.Tools))
		for i, tool := range req.Tools {
			openAIReq.Tools[i] = openAITool{
				Type: "function",
				Function: openAIFunction{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.Parameters,
				},
			}
		}
	}

	return openAIReq
}

// convertResponse converts an OpenAI response to ChatResponse.
func (p *OpenAIProvider) convertResponse(resp openAIResponse) *ChatResponse {
	if len(resp.Choices) == 0 {
		return &ChatResponse{
			ID:    resp.ID,
			Model: resp.Model,
		}
	}

	choice := resp.Choices[0]
	message := Message{
		Role:    Role(choice.Message.Role),
		Content: choice.Message.Content,
	}

	if len(choice.Message.ToolCalls) > 0 {
		message.ToolCalls = make([]ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			message.ToolCalls[i] = ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			}
		}
	}

	return &ChatResponse{
		ID:      resp.ID,
		Model:   resp.Model,
		Message: message,
		Usage: Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}
}
