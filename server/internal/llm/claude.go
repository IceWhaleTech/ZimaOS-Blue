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
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const (
	defaultClaudeBaseURL = "https://api.anthropic.com"
	claudeAPIVersion     = "2023-06-01"
	claudeTimeout        = 120 * time.Second
)

// ToolCapLevel represents the tool capability level of a provider endpoint.
type ToolCapLevel int

const (
	ToolCapNative   ToolCapLevel = iota // Native tool_use supported
	ToolCapPrompt                       // Tools via system prompt injection
	ToolCapNone                         // No tools at all
	ToolCapUnknown  ToolCapLevel = -1   // Not yet probed
)

// toolCapCache stores the remembered tool capability level per baseURL.
// Key: baseURL string, Value: ToolCapLevel
var toolCapCache sync.Map

// RecallToolCap returns the cached tool capability level for a baseURL.
func RecallToolCap(baseURL string) ToolCapLevel {
	if v, ok := toolCapCache.Load(baseURL); ok {
		return v.(ToolCapLevel)
	}
	return ToolCapUnknown
}

// RememberToolCap caches the tool capability level for a baseURL.
func RememberToolCap(baseURL string, level ToolCapLevel) {
	toolCapCache.Store(baseURL, level)
}

// ClaudeProvider implements the Provider interface for Anthropic Claude.
type ClaudeProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewClaudeProvider creates a new Claude provider.
func NewClaudeProvider(apiKey, baseURL string) *ClaudeProvider {
	if baseURL == "" {
		baseURL = defaultClaudeBaseURL
	}
	// Remove trailing slash to avoid double slashes in URL
	baseURL = strings.TrimSuffix(baseURL, "/")
	// Remove trailing /v1 as it will be added when making requests
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	return &ClaudeProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: claudeTimeout,
			Transport: &http.Transport{
				// Disable response buffering for streaming
				DisableCompression: true,
			},
		},
	}
}

// Name returns the provider name.
func (p *ClaudeProvider) Name() string {
	return "claude"
}

// Models returns the list of available models.
func (p *ClaudeProvider) Models() []string {
	return []string{
		// Claude 4 series (latest)
		"claude-opus-4-5-20251101",
		"claude-sonnet-4-20250514",
		// Claude 3.5 series
		"claude-3-5-sonnet-20241022",
		"claude-3-5-haiku-20241022",
		// Claude 3 series
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
	}
}

// claudeRequest represents the Claude API request format.
type claudeRequest struct {
	Model       string          `json:"model"`
	Messages    []claudeMessage `json:"messages"`
	System      string          `json:"system,omitempty"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature float64         `json:"temperature,omitempty"`
	Tools       []claudeTool    `json:"tools,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type claudeMessage struct {
	Role    string         `json:"role"`
	Content []claudeContent `json:"content"`
}

type claudeContent struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text,omitempty"`
	ID        string                 `json:"id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	ToolUseID string                 `json:"tool_use_id,omitempty"`
	Content   string                 `json:"content,omitempty"`
	// Image support
	Source *claudeImageSource `json:"source,omitempty"`
}

type claudeImageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // e.g., "image/jpeg"
	Data      string `json:"data"`       // base64 encoded image data
}

type claudeTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// claudeResponse represents the Claude API response format.
type claudeResponse struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Role       string          `json:"role"`
	Model      string          `json:"model"`
	Content    []claudeContent `json:"content"`
	StopReason string          `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Chat sends a chat completion request to Claude.
func (p *ClaudeProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Convert to Claude format
	claudeReq := p.convertRequest(req)

	// Marshal request
	body, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := p.baseURL + "/v1/messages"
	// Log request URL and masked API key for debugging
	maskedKey := p.apiKey
	if len(maskedKey) > 8 {
		maskedKey = maskedKey[:4] + "..." + maskedKey[len(maskedKey)-4:]
	}
	fmt.Printf("[Claude Chat] URL: %s, APIKey: %s\n", apiURL, maskedKey)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", claudeAPIVersion)

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
	var claudeResp claudeResponse
	if err := json.Unmarshal(respBody, &claudeResp); err != nil {
		// Log raw response for debugging (truncate if too long)
		rawResp := string(respBody)
		if len(rawResp) > 500 {
			rawResp = rawResp[:500] + "...(truncated)"
		}
		return nil, fmt.Errorf("failed to parse response: %w, raw response: %s", err, rawResp)
	}

	// Check for API error
	if claudeResp.Error != nil {
		return nil, fmt.Errorf("Claude API error: %s", claudeResp.Error.Message)
	}

	// Check for HTTP error
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Claude API returned status %d", resp.StatusCode)
	}

	// Convert response
	return p.convertResponse(claudeResp), nil
}

// ChatStream sends a streaming chat completion request to Claude.
func (p *ClaudeProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	// Set stream flag
	req.Stream = true
	claudeReq := p.convertRequest(req)

	// Marshal request
	body, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := p.baseURL + "/v1/messages"
	// Log request URL and masked API key for debugging
	maskedKey := p.apiKey
	if len(maskedKey) > 8 {
		maskedKey = maskedKey[:4] + "..." + maskedKey[len(maskedKey)-4:]
	}
	fmt.Printf("[Claude ChatStream] URL: %s, APIKey: %s\n", apiURL, maskedKey)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", claudeAPIVersion)
	httpReq.Header.Set("Accept", "text/event-stream")

	// Send request
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Check for HTTP error
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("Claude API returned status %d", resp.StatusCode)
	}

	// Create channel for streaming (unbuffered for immediate delivery)
	ch := make(chan StreamChunk)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		p.parseSSEStream(ctx, resp.Body, ch, req.Model)
	}()

	return ch, nil
}

// ChatStreamCallback sends a streaming chat completion request and calls the callback for each chunk.
func (p *ClaudeProvider) ChatStreamCallback(ctx context.Context, req ChatRequest, callback StreamCallback) error {
	// Mask API key for logging (show first 8 and last 4 chars)
	maskedKey := p.apiKey
	if len(p.apiKey) > 12 {
		maskedKey = p.apiKey[:8] + "..." + p.apiKey[len(p.apiKey)-4:]
	}
	fmt.Printf("[ClaudeProvider] ChatStreamCallback: model=%s, messages=%d, baseURL=%s, apiKey=%s\n", req.Model, len(req.Messages), p.baseURL, maskedKey)

	// Set stream flag
	req.Stream = true
	claudeReq := p.convertRequest(req)
	origSystem := claudeReq.System

	// Check cached tool capability level — skip straight to what worked before
	cachedLevel := RecallToolCap(p.baseURL)
	hasTools := len(req.Tools) > 0

	if hasTools && cachedLevel != ToolCapUnknown {
		fmt.Printf("[ClaudeProvider] using cached tool level %d for %s\n", cachedLevel, p.baseURL)
		p.applyToolLevel(&claudeReq, cachedLevel, req.Tools, origSystem)
	}

	// Try the request
	resp, err := p.doClaudeRequest(ctx, claudeReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	fmt.Printf("[ClaudeProvider] response status: %d\n", resp.StatusCode)

	if resp.StatusCode == http.StatusOK {
		// Remember what worked (only if we had tools to degrade)
		if hasTools && cachedLevel == ToolCapUnknown {
			RememberToolCap(p.baseURL, ToolCapNative)
			fmt.Printf("[ClaudeProvider] remembered tool level: native for %s\n", p.baseURL)
		}
		return p.parseSSEStreamCallback(ctx, resp.Body, req.Model, callback)
	}

	// Non-OK: read error body
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("[ClaudeProvider] error response: %s\n", string(bodyBytes))

	// Progressive tool degradation on 400 (only if we have tools and haven't already degraded past this)
	if resp.StatusCode == http.StatusBadRequest && hasTools {
		startLevel := ToolCapNative
		if cachedLevel != ToolCapUnknown {
			startLevel = cachedLevel
		}

		// Try each level below the current one
		for level := startLevel + 1; level <= ToolCapNone; level++ {
			fmt.Printf("[ClaudeProvider] tools degradation: trying level %d for %s\n", level, p.baseURL)
			// Reset to original system prompt before applying new level
			claudeReq.System = origSystem
			claudeReq.Tools = nil // clear before reapply
			p.applyToolLevel(&claudeReq, level, req.Tools, origSystem)

			retryResp, retryErr := p.doClaudeRequest(ctx, claudeReq)
			if retryErr != nil {
				continue
			}
			if retryResp.StatusCode == http.StatusOK {
				RememberToolCap(p.baseURL, level)
				fmt.Printf("[ClaudeProvider] remembered tool level: %d for %s\n", level, p.baseURL)
				defer retryResp.Body.Close()
				return p.parseSSEStreamCallback(ctx, retryResp.Body, req.Model, callback)
			}
			rb, _ := io.ReadAll(retryResp.Body)
			retryResp.Body.Close()
			fmt.Printf("[ClaudeProvider] level %d failed (status %d): %s\n", level, retryResp.StatusCode, string(rb))
			if retryResp.StatusCode != http.StatusBadRequest {
				return fmt.Errorf("Claude API returned status %d: %s", retryResp.StatusCode, string(rb))
			}
		}
	}

	return fmt.Errorf("Claude API returned status %d: %s", resp.StatusCode, string(bodyBytes))
}

// doClaudeRequest marshals and sends a claude request, returning the raw HTTP response.
func (p *ClaudeProvider) doClaudeRequest(ctx context.Context, claudeReq claudeRequest) (*http.Response, error) {
	body, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", claudeAPIVersion)
	httpReq.Header.Set("Accept", "text/event-stream")
	return p.client.Do(httpReq)
}

// applyToolLevel modifies claudeReq in-place to match the given tool capability level.
func (p *ClaudeProvider) applyToolLevel(claudeReq *claudeRequest, level ToolCapLevel, tools []Tool, origSystem string) {
	switch level {
	case ToolCapNative:
		// Already set by convertRequest — nothing to do
	case ToolCapPrompt:
		claudeReq.Tools = nil
		toolPrompt := toolsToSystemPrompt(tools)
		if origSystem != "" {
			claudeReq.System = origSystem + "\n\n" + toolPrompt
		} else {
			claudeReq.System = toolPrompt
		}
	case ToolCapNone:
		claudeReq.Tools = nil
		claudeReq.System = origSystem
	}
}

// parseSSEStreamCallback parses SSE stream and calls callback for each chunk.
func (p *ClaudeProvider) parseSSEStreamCallback(ctx context.Context, reader io.Reader, model string, callback StreamCallback) error {
	// Read byte by byte for immediate streaming without buffering
	var messageID string
	var inputTokens, outputTokens int
	var lineBuffer strings.Builder
	buf := make([]byte, 1)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := reader.Read(buf)
		if err != nil {
			if err == io.EOF {
				// Process any remaining data
				if lineBuffer.Len() > 0 {
					line := strings.TrimSpace(lineBuffer.String())
					if line != "" {
						p.processSSELineCallback(ctx, line, &messageID, &inputTokens, &outputTokens, model, callback)
					}
				}
				return nil
			}
			return err
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

			done, err := p.processSSELineCallback(ctx, line, &messageID, &inputTokens, &outputTokens, model, callback)
			if err != nil {
				return err
			}
			if done {
				return nil
			}
		} else if b != '\r' {
			lineBuffer.WriteByte(b)
		}
	}
}

// processSSELineCallback processes a single SSE line with callback.
func (p *ClaudeProvider) processSSELineCallback(ctx context.Context, line string, messageID *string, inputTokens, outputTokens *int, model string, callback StreamCallback) (bool, error) {
	// Skip non-data lines
	if len(line) < 6 || line[:6] != "data: " {
		return false, nil
	}

	data := line[6:]
	if data == "[DONE]" {
		return true, nil
	}

	var event struct {
		Type    string `json:"type"`
		Index   int    `json:"index"`
		Message struct {
			ID    string `json:"id"`
			Model string `json:"model"`
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		} `json:"message"`
		ContentBlock struct {
			Type  string                 `json:"type"`
			ID    string                 `json:"id"`
			Name  string                 `json:"name"`
			Input map[string]interface{} `json:"input"`
		} `json:"content_block"`
		Delta struct {
			Type         string `json:"type"`
			Text         string `json:"text"`
			PartialJSON  string `json:"partial_json"`
		} `json:"delta"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return false, nil
	}

	switch event.Type {
	case "message_start":
		*messageID = event.Message.ID
		if event.Message.Usage.InputTokens > 0 {
			*inputTokens = event.Message.Usage.InputTokens
		}

	case "content_block_start":
		// Handle tool_use blocks — emit as ToolCall when block starts
		if event.ContentBlock.Type == "tool_use" {
			var args string
			if event.ContentBlock.Input != nil {
				inputJSON, _ := json.Marshal(event.ContentBlock.Input)
				args = string(inputJSON)
			}
			if err := callback(StreamChunk{
				ToolCalls: []ToolCall{{
					ID:        event.ContentBlock.ID,
					Name:      event.ContentBlock.Name,
					Arguments: args,
				}},
				Done: false,
			}); err != nil {
				return true, err
			}
		}

	case "content_block_delta":
		if event.Delta.Text != "" {
			// Split text into small chunks for typewriter effect
			if err := p.sendTextChunksCallback(ctx, event.Delta.Text, callback); err != nil {
				return true, err
			}
		}
		// Accumulate partial JSON for tool_use input
		if event.Delta.Type == "input_json_delta" && event.Delta.PartialJSON != "" {
			// Tool input arrives incrementally — emit as partial tool call update
			if err := callback(StreamChunk{
				ToolCalls: []ToolCall{{Arguments: event.Delta.PartialJSON}},
				Done:      false,
			}); err != nil {
				return true, err
			}
		}

	case "message_delta":
		if event.Usage.OutputTokens > 0 {
			*outputTokens = event.Usage.OutputTokens
		}

	case "message_stop":
		if err := callback(StreamChunk{
			ID:    *messageID,
			Model: model,
			Done:  true,
			Usage: &Usage{
				PromptTokens:     *inputTokens,
				CompletionTokens: *outputTokens,
				TotalTokens:      *inputTokens + *outputTokens,
			},
		}); err != nil {
			return true, err
		}
		return true, nil
	}

	return false, nil
}

// parseSSEStream parses SSE stream from Claude API and sends chunks.
// Uses byte-level reading for immediate response when newlines are detected.
func (p *ClaudeProvider) parseSSEStream(ctx context.Context, reader io.Reader, ch chan<- StreamChunk, model string) {
	bufReader := bufio.NewReaderSize(reader, 4096)
	var messageID string
	var inputTokens, outputTokens int
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
				// Process any remaining data
				if lineBuffer.Len() > 0 {
					p.processSSELine(ctx, ch, lineBuffer.String(), &messageID, &inputTokens, &outputTokens, model)
				}
				return
			}
			continue
		}

		if b == '\n' {
			// Process the complete line immediately
			line := strings.TrimSpace(lineBuffer.String())
			lineBuffer.Reset()

			if line == "" {
				continue
			}

			done := p.processSSELine(ctx, ch, line, &messageID, &inputTokens, &outputTokens, model)
			if done {
				return
			}
		} else if b != '\r' {
			lineBuffer.WriteByte(b)
		}
	}
}

// processSSELine processes a single SSE line and returns true if stream is done.
func (p *ClaudeProvider) processSSELine(ctx context.Context, ch chan<- StreamChunk, line string, messageID *string, inputTokens, outputTokens *int, model string) bool {
	// Skip non-data lines
	if len(line) < 6 || line[:6] != "data: " {
		return false
	}

	data := line[6:]
	if data == "[DONE]" {
		return true
	}

	var event struct {
		Type    string `json:"type"`
		Index   int    `json:"index"`
		Message struct {
			ID    string `json:"id"`
			Model string `json:"model"`
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		} `json:"message"`
		ContentBlock struct {
			Type  string                 `json:"type"`
			ID    string                 `json:"id"`
			Name  string                 `json:"name"`
			Input map[string]interface{} `json:"input"`
		} `json:"content_block"`
		Delta struct {
			Type        string `json:"type"`
			Text        string `json:"text"`
			PartialJSON string `json:"partial_json"`
		} `json:"delta"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return false
	}

	switch event.Type {
	case "message_start":
		*messageID = event.Message.ID
		if event.Message.Usage.InputTokens > 0 {
			*inputTokens = event.Message.Usage.InputTokens
		}

	case "content_block_start":
		if event.ContentBlock.Type == "tool_use" {
			var args string
			if event.ContentBlock.Input != nil {
				inputJSON, _ := json.Marshal(event.ContentBlock.Input)
				args = string(inputJSON)
			}
			select {
			case <-ctx.Done():
				return true
			case ch <- StreamChunk{
				ToolCalls: []ToolCall{{
					ID:        event.ContentBlock.ID,
					Name:      event.ContentBlock.Name,
					Arguments: args,
				}},
				Done: false,
			}:
			}
		}

	case "content_block_delta":
		if event.Delta.Text != "" {
			// Send text in small chunks for typewriter effect
			p.sendTextChunks(ctx, ch, event.Delta.Text)
		}
		if event.Delta.Type == "input_json_delta" && event.Delta.PartialJSON != "" {
			select {
			case <-ctx.Done():
				return true
			case ch <- StreamChunk{
				ToolCalls: []ToolCall{{Arguments: event.Delta.PartialJSON}},
				Done:      false,
			}:
			}
		}

	case "message_delta":
		if event.Usage.OutputTokens > 0 {
			*outputTokens = event.Usage.OutputTokens
		}

	case "message_stop":
		select {
		case <-ctx.Done():
			return true
		case ch <- StreamChunk{
			ID:    *messageID,
			Model: model,
			Done:  true,
			Usage: &Usage{
				PromptTokens:     *inputTokens,
				CompletionTokens: *outputTokens,
				TotalTokens:      *inputTokens + *outputTokens,
			},
		}:
		}
		return true
	}

	return false
}

// sendTextChunks splits text into small chunks and sends them for typewriter effect.
func (p *ClaudeProvider) sendTextChunks(ctx context.Context, ch chan<- StreamChunk, text string) {
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

// sendTextChunksCallback splits text into small chunks and calls callback for typewriter effect.
func (p *ClaudeProvider) sendTextChunksCallback(ctx context.Context, text string, callback StreamCallback) error {
	// For very short text (1-2 chars), send directly with a small delay
	runes := []rune(text)
	if len(runes) <= 2 {
		if err := callback(StreamChunk{
			Delta: text,
			Done:  false,
		}); err != nil {
			return err
		}
		// Small delay for single character chunks (15-25ms)
		delay := time.Duration(15+timeutil.NowNano()%10) * time.Millisecond
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		return nil
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

		if err := callback(StreamChunk{
			Delta: chunk,
			Done:  false,
		}); err != nil {
			return err
		}

		// Add small delay between chunks for typewriter effect (10-30ms)
		if pos < len(runes) {
			delay := time.Duration(10+timeutil.NowNano()%20) * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return nil
}

// convertRequest converts a ChatRequest to Claude format.
func (p *ClaudeProvider) convertRequest(req ChatRequest) claudeRequest {
	var systemPrompt string
	var messages []claudeMessage

	for _, msg := range req.Messages {
		if msg.Role == RoleSystem {
			systemPrompt = msg.Content
			continue
		}

		claudeMsg := claudeMessage{
			Role: string(msg.Role),
		}

		// Handle tool response
		if msg.Role == RoleTool {
			claudeMsg.Role = "user"
			claudeMsg.Content = []claudeContent{
				{
					Type:      "tool_result",
					ToolUseID: msg.ToolCallID,
					Content:   msg.Content,
				},
			}
		} else if len(msg.ToolCalls) > 0 {
			// Handle assistant message with tool calls
			claudeMsg.Content = make([]claudeContent, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				var input map[string]interface{}
				json.Unmarshal([]byte(tc.Arguments), &input)
				claudeMsg.Content[i] = claudeContent{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: input,
				}
			}
		} else {
			// Handle regular message or multimodal message
			if len(msg.ContentParts) > 0 {
				// Multimodal message with content parts
				claudeMsg.Content = make([]claudeContent, 0, len(msg.ContentParts))
				for _, part := range msg.ContentParts {
					switch part.Type {
					case "text":
						claudeMsg.Content = append(claudeMsg.Content, claudeContent{
							Type: "text",
							Text: part.Text,
						})
					case "image":
						claudeMsg.Content = append(claudeMsg.Content, claudeContent{
							Type: "image",
							Source: &claudeImageSource{
								Type:      "base64",
								MediaType: part.MediaType,
								Data:      part.Data,
							},
						})
					}
				}
			} else {
				// Simple text message
				claudeMsg.Content = []claudeContent{
					{Type: "text", Text: msg.Content},
				}
			}
		}

		messages = append(messages, claudeMsg)
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096 // Default max tokens for Claude
	}

	claudeReq := claudeRequest{
		Model:       req.Model,
		Messages:    messages,
		System:      systemPrompt,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
	}

	if len(req.Tools) > 0 {
		claudeReq.Tools = make([]claudeTool, len(req.Tools))
		for i, tool := range req.Tools {
			inputSchema := tool.Parameters
			if inputSchema == nil {
				inputSchema = map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				}
			}
			claudeReq.Tools[i] = claudeTool{
				Name:        tool.Name,
				Description: tool.Description,
				InputSchema: inputSchema,
			}
		}
	}

	return claudeReq
}

// toolsToSystemPrompt converts tool definitions into a system prompt instruction.
func toolsToSystemPrompt(tools []Tool) string {
	var sb strings.Builder
	sb.WriteString("You have access to the following tools. When you need to use a tool, respond with a JSON block in this exact format:\n")
	sb.WriteString("```tool_call\n{\"name\": \"tool_name\", \"arguments\": {\"param\": \"value\"}}\n```\n\n")
	sb.WriteString("Available tools:\n")
	for _, t := range tools {
		sb.WriteString("- **" + t.Name + "**: " + t.Description + "\n")
		if t.Parameters != nil {
			if paramJSON, err := json.Marshal(t.Parameters); err == nil {
				sb.WriteString("  Parameters: " + string(paramJSON) + "\n")
			}
		}
	}
	sb.WriteString("\nOnly use a tool if it is clearly needed. Otherwise respond normally.")
	return sb.String()
}

// convertResponse converts a Claude response to ChatResponse.
func (p *ClaudeProvider) convertResponse(resp claudeResponse) *ChatResponse {
	message := Message{
		Role: RoleAssistant,
	}

	var textContent string
	var toolCalls []ToolCall

	for _, content := range resp.Content {
		switch content.Type {
		case "text":
			textContent += content.Text
		case "tool_use":
			inputJSON, _ := json.Marshal(content.Input)
			toolCalls = append(toolCalls, ToolCall{
				ID:        content.ID,
				Name:      content.Name,
				Arguments: string(inputJSON),
			})
		}
	}

	message.Content = textContent
	message.ToolCalls = toolCalls

	return &ChatResponse{
		ID:      resp.ID,
		Model:   resp.Model,
		Message: message,
		Usage: Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
}
