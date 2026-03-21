package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const (
	defaultOpenAIBaseURL = "https://api.openai.com"
	openAITimeout        = 5 * time.Minute
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
		client: network.NewPooledHTTPClientWithOptions(network.HTTPClientOptions{
			Timeout:            openAITimeout,
			DisableCompression: true,
		}),
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
	// Return default fallback list for official OpenAI API
	// For third-party APIs, CustomProvider overrides this with its own fallback
	return p.getDefaultModels()
}

// getDefaultModels returns the default model list for OpenAI.
// This can be overridden by embedded providers.
func (p *OpenAIProvider) getDefaultModels() []string {
	return []string{
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-4-turbo",
		"gpt-4",
		"gpt-3.5-turbo",
		"o1",
		"o1-mini",
		"o1-preview",
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
	url := p.getAPIPath("/models")
	models := p.tryFetchModels(ctx, url)
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
// For third-party OpenAI-compatible APIs, we use a permissive approach:
// exclude known non-chat models rather than requiring known chat patterns.
func (p *OpenAIProvider) isChatModel(modelID string) bool {
	modelLower := strings.ToLower(modelID)

	// Exclude known non-chat models (embeddings, audio, image, moderation)
	excludePatterns := []string{
		// Embedding models
		"embedding", "embed-", "text-embedding",
		// Audio models
		"whisper", "tts-", "audio",
		// Image models
		"dall-e", "stable-diffusion", "midjourney", "image",
		// Legacy completion models (not chat)
		"davinci", "babbage", "ada", "curie",
		// Moderation and other utility models
		"moderation", "content-filter",
		// Rerank models
		"rerank",
	}
	for _, pattern := range excludePatterns {
		if strings.Contains(modelLower, pattern) {
			return false
		}
	}

	// Include all other models by default
	// This is more permissive for third-party APIs that may have custom model names
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
	return p.getDefaultModels()
}

// openAIRequest represents the OpenAI API request format.
type openAIRequest struct {
	Model         string               `json:"model"`
	Messages      []openAIMessage      `json:"messages"`
	Temperature   float64              `json:"temperature,omitempty"`
	MaxTokens     int                  `json:"max_tokens,omitempty"`
	Tools         []openAITool         `json:"tools,omitempty"`
	Stream        bool                 `json:"stream,omitempty"`
	StreamOptions *openAIStreamOptions `json:"stream_options,omitempty"`
}

// openAIStreamOptions represents streaming options for OpenAI API.
type openAIStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    interface{}      `json:"content"` // string or []openAIContentPart for vision
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

// openAIContentPart represents a content part for vision API
type openAIContentPart struct {
	Type       string            `json:"type"` // "text" or "image_url"
	Text       string            `json:"text,omitempty"`
	ImageURL   *openAIImageURL   `json:"image_url,omitempty"`
	InputAudio *openAIInputAudio `json:"input_audio,omitempty"`
}

type openAIImageURL struct {
	URL    string `json:"url"`              // data:image/jpeg;base64,... or URL
	Detail string `json:"detail,omitempty"` // "low", "high", or "auto"
}

type openAIInputAudio struct {
	Data   string `json:"data"`
	Format string `json:"format,omitempty"` // wav | mp3
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
			Role      string           `json:"role"`
			Content   string           `json:"content"`
			ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
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

	apiURL := p.getAPIPath("/chat/completions")
	// Log request URL and masked API key for debugging
	maskedKey := p.apiKey
	if len(maskedKey) > 8 {
		maskedKey = maskedKey[:4] + "..." + maskedKey[len(maskedKey)-4:]
	}
	fmt.Printf("[OpenAI Chat] URL: %s, APIKey: %s\n", apiURL, maskedKey)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
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

	apiURL := p.getAPIPath("/chat/completions")
	// Log request URL and masked API key for debugging
	maskedKey := p.apiKey
	if len(maskedKey) > 8 {
		maskedKey = maskedKey[:4] + "..." + maskedKey[len(maskedKey)-4:]
	}
	fmt.Printf("[OpenAI ChatStream] URL: %s, APIKey: %s\n", apiURL, maskedKey)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
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

	// Create channel for streaming (unbuffered for immediate delivery)
	ch := make(chan StreamChunk)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		p.parseOpenAISSEStream(ctx, resp.Body, ch, req.Model)
	}()

	return ch, nil
}

// ChatStreamCallback sends a streaming chat completion request and calls the callback for each chunk.
func (p *OpenAIProvider) ChatStreamCallback(ctx context.Context, req ChatRequest, callback StreamCallback) error {
	// Set stream flag
	req.Stream = true
	openAIReq := p.convertRequest(req)

	// Marshal request
	body, err := json.Marshal(openAIReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.getAPIPath("/chat/completions"), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	// Send request
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check for HTTP error
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("OpenAI API returned status %d", resp.StatusCode)
	}

	// Parse SSE stream directly with callback
	return p.parseOpenAISSEStreamCallback(ctx, resp.Body, req.Model, callback)
}

// parseOpenAISSEStreamCallback parses SSE stream and calls callback for each chunk.
func (p *OpenAIProvider) parseOpenAISSEStreamCallback(ctx context.Context, reader io.Reader, model string, callback StreamCallback) error {
	// Read directly without buffering for immediate response
	var messageID string
	var actualModel = model // Track actual model from response
	var promptTokens, completionTokens int
	var lineBuffer strings.Builder
	buf := make([]byte, 1)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Read byte by byte directly from reader (no buffering)
		n, err := reader.Read(buf)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			continue
		}
		if n == 0 {
			continue
		}

		b := buf[0]
		if b == '\n' {
			line := strings.TrimSpace(lineBuffer.String())
			lineBuffer.Reset()

			if line == "" {
				continue
			}

			// Parse data lines
			if len(line) < 6 || line[:6] != "data: " {
				continue
			}

			data := line[6:]
			if data == "[DONE]" {
				// Send final chunk
				return callback(StreamChunk{
					ID:    messageID,
					Model: actualModel,
					Done:  true,
					Usage: &Usage{
						PromptTokens:     promptTokens,
						CompletionTokens: completionTokens,
						TotalTokens:      promptTokens + completionTokens,
					},
				})
			}

			var chunk struct {
				ID      string `json:"id"`
				Model   string `json:"model"`
				Choices []struct {
					Delta struct {
						Content   string           `json:"content"`
						ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
				Usage *struct {
					PromptTokens     int `json:"prompt_tokens"`
					CompletionTokens int `json:"completion_tokens"`
					TotalTokens      int `json:"total_tokens"`
				} `json:"usage,omitempty"`
			}

			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if chunk.ID != "" {
				messageID = chunk.ID
			}

			// Capture actual model from response
			if chunk.Model != "" {
				actualModel = chunk.Model
			}

			if chunk.Usage != nil {
				promptTokens = chunk.Usage.PromptTokens
				completionTokens = chunk.Usage.CompletionTokens
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			content := chunk.Choices[0].Delta.Content
			if content != "" {
				if err := callback(StreamChunk{
					Delta: content,
					Done:  false,
				}); err != nil {
					return err
				}
			}

			// Forward tool calls from stream delta
			if len(chunk.Choices[0].Delta.ToolCalls) > 0 {
				var tcs []ToolCall
				for _, tc := range chunk.Choices[0].Delta.ToolCalls {
					tcs = append(tcs, ToolCall{
						ID:        tc.ID,
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					})
				}
				if err := callback(StreamChunk{
					ToolCalls: tcs,
					Done:      false,
				}); err != nil {
					return err
				}
			}

			if chunk.Choices[0].FinishReason == "stop" || chunk.Choices[0].FinishReason == "tool_calls" {
				return callback(StreamChunk{
					ID:    messageID,
					Model: actualModel,
					Done:  true,
					Usage: &Usage{
						PromptTokens:     promptTokens,
						CompletionTokens: completionTokens,
						TotalTokens:      promptTokens + completionTokens,
					},
				})
			}
		} else if b != '\r' {
			lineBuffer.WriteByte(b)
		}
	}
}

// parseOpenAISSEStream parses SSE stream from OpenAI API and sends chunks.
func (p *OpenAIProvider) parseOpenAISSEStream(ctx context.Context, reader io.Reader, ch chan<- StreamChunk, model string) {
	bufReader := bufio.NewReaderSize(reader, 4096)
	var messageID string
	var actualModel = model // Track actual model from response
	var promptTokens, completionTokens int
	var lineBuffer strings.Builder

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Read byte by byte for immediate newline detection
		b, err := bufReader.ReadByte()
		if err != nil {
			if err == io.EOF {
				return
			}
			continue
		}

		if b == '\n' {
			line := strings.TrimSpace(lineBuffer.String())
			lineBuffer.Reset()

			if line == "" {
				continue
			}

			// Parse data lines
			if len(line) < 6 || line[:6] != "data: " {
				continue
			}

			data := line[6:]
			if data == "[DONE]" {
				// Send final chunk
				select {
				case <-ctx.Done():
					return
				case ch <- StreamChunk{
					ID:    messageID,
					Model: actualModel,
					Done:  true,
					Usage: &Usage{
						PromptTokens:     promptTokens,
						CompletionTokens: completionTokens,
						TotalTokens:      promptTokens + completionTokens,
					},
				}:
				}
				return
			}

			var chunk struct {
				ID      string `json:"id"`
				Model   string `json:"model"`
				Choices []struct {
					Delta struct {
						Content   string           `json:"content"`
						ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
				Usage *struct {
					PromptTokens     int `json:"prompt_tokens"`
					CompletionTokens int `json:"completion_tokens"`
					TotalTokens      int `json:"total_tokens"`
				} `json:"usage,omitempty"`
			}

			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if chunk.ID != "" {
				messageID = chunk.ID
			}

			// Capture actual model from response
			if chunk.Model != "" {
				actualModel = chunk.Model
			}

			if chunk.Usage != nil {
				promptTokens = chunk.Usage.PromptTokens
				completionTokens = chunk.Usage.CompletionTokens
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			content := chunk.Choices[0].Delta.Content
			if content != "" {
				// Send content directly - OpenAI API already returns token by token
				select {
				case <-ctx.Done():
					return
				case ch <- StreamChunk{
					Delta: content,
					Done:  false,
				}:
				}
			}

			// Forward tool calls from stream delta
			if len(chunk.Choices[0].Delta.ToolCalls) > 0 {
				var tcs []ToolCall
				for _, tc := range chunk.Choices[0].Delta.ToolCalls {
					tcs = append(tcs, ToolCall{
						ID:        tc.ID,
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					})
				}
				select {
				case <-ctx.Done():
					return
				case ch <- StreamChunk{
					ToolCalls: tcs,
					Done:      false,
				}:
				}
			}

			if chunk.Choices[0].FinishReason == "stop" || chunk.Choices[0].FinishReason == "tool_calls" {
				select {
				case <-ctx.Done():
					return
				case ch <- StreamChunk{
					ID:    messageID,
					Model: actualModel,
					Done:  true,
					Usage: &Usage{
						PromptTokens:     promptTokens,
						CompletionTokens: completionTokens,
						TotalTokens:      promptTokens + completionTokens,
					},
				}:
				}
				return
			}
		} else if b != '\r' {
			lineBuffer.WriteByte(b)
		}
	}
}

// sendOpenAITextChunks splits text into small chunks and sends them for typewriter effect.
func (p *OpenAIProvider) sendOpenAITextChunks(ctx context.Context, ch chan<- StreamChunk, text string) {
	// For very short text (1-2 chars), send directly with a small delay
	runes := []rune(text)
	if len(runes) <= 2 {
		select {
		case <-ctx.Done():
			return
		case ch <- StreamChunk{
			Delta: text,
			Done:  false,
		}:
		}
		// Small delay for single character chunks (15-25ms)
		delay := time.Duration(15+timeutil.NowNano()%10) * time.Millisecond
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		return
	}

	// Split into small chunks (3-6 characters for better typewriter effect)
	pos := 0

	for pos < len(runes) {
		// Random chunk size between 3-6 characters
		chunkSize := 3 + int(timeutil.NowNano()%4)
		if pos+chunkSize > len(runes) {
			chunkSize = len(runes) - pos
		}

		chunk := string(runes[pos : pos+chunkSize])
		pos += chunkSize

		select {
		case <-ctx.Done():
			return
		case ch <- StreamChunk{
			Delta: chunk,
			Done:  false,
		}:
		}

		// Add small delay between chunks for typewriter effect (10-30ms)
		if pos < len(runes) {
			delay := time.Duration(10+timeutil.NowNano()%20) * time.Millisecond
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}
	}
}

// convertRequest converts a ChatRequest to OpenAI format.
func (p *OpenAIProvider) convertRequest(req ChatRequest) openAIRequest {
	messages := make([]openAIMessage, len(req.Messages))
	for i, msg := range req.Messages {
		oaiMsg := openAIMessage{
			Role:       string(msg.Role),
			ToolCallID: msg.ToolCallID,
		}

		// Handle multimodal content
		if len(msg.ContentParts) > 0 {
			// Build content parts array for vision API
			contentParts := make([]openAIContentPart, 0, len(msg.ContentParts))
			for _, part := range msg.ContentParts {
				switch part.Type {
				case "text":
					contentParts = append(contentParts, openAIContentPart{
						Type: "text",
						Text: part.Text,
					})
				case "image":
					// Convert to data URL format
					dataURL := "data:" + part.MediaType + ";base64," + part.Data
					contentParts = append(contentParts, openAIContentPart{
						Type: "image_url",
						ImageURL: &openAIImageURL{
							URL:    dataURL,
							Detail: "auto",
						},
					})
				case "audio":
					contentParts = append(contentParts, openAIContentPart{
						Type: "input_audio",
						InputAudio: &openAIInputAudio{
							Data:   part.Data,
							Format: openAIAudioFormatFromMediaType(part.MediaType),
						},
					})
				}
			}
			oaiMsg.Content = contentParts
		} else {
			// Simple text content
			oaiMsg.Content = msg.Content
		}

		if len(msg.ToolCalls) > 0 {
			oaiMsg.ToolCalls = make([]openAIToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				oaiMsg.ToolCalls[j] = openAIToolCall{
					ID:   tc.ID,
					Type: "function",
				}
				oaiMsg.ToolCalls[j].Function.Name = tc.Name
				oaiMsg.ToolCalls[j].Function.Arguments = tc.Arguments
			}
		}

		messages[i] = oaiMsg
	}

	openAIReq := openAIRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
	}

	// Request usage data in streaming mode
	if req.Stream {
		openAIReq.StreamOptions = &openAIStreamOptions{
			IncludeUsage: true,
		}
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

func openAIAudioFormatFromMediaType(mediaType string) string {
	mt := strings.ToLower(strings.TrimSpace(mediaType))
	if strings.Contains(mt, "wav") {
		return "wav"
	}
	// OpenAI input_audio supports wav/mp3 in chat payloads; use mp3 as fallback
	// for non-wav formats (webm/ogg/m4a may still fail provider-side).
	return "mp3"
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
