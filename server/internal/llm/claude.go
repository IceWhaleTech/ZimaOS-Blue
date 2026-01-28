package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultClaudeBaseURL = "https://api.anthropic.com"
	claudeAPIVersion     = "2023-06-01"
	claudeTimeout        = 120 * time.Second
)

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
	return &ClaudeProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: claudeTimeout,
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

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/messages", bytes.NewReader(body))
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
		return nil, fmt.Errorf("failed to parse response: %w", err)
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

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/messages", bytes.NewReader(body))
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

			var event struct {
				Type  string `json:"type"`
				Index int    `json:"index"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
				Message struct {
					ID    string `json:"id"`
					Model string `json:"model"`
					Usage struct {
						InputTokens  int `json:"input_tokens"`
						OutputTokens int `json:"output_tokens"`
					} `json:"usage"`
				} `json:"message"`
			}

			if err := decoder.Decode(&event); err != nil {
				if err == io.EOF {
					return
				}
				continue
			}

			switch event.Type {
			case "content_block_delta":
				select {
				case <-ctx.Done():
					return
				case ch <- StreamChunk{
					Delta: event.Delta.Text,
					Done:  false,
				}:
				}
			case "message_stop":
				select {
				case <-ctx.Done():
					return
				case ch <- StreamChunk{
					ID:    event.Message.ID,
					Model: event.Message.Model,
					Done:  true,
					Usage: &Usage{
						PromptTokens:     event.Message.Usage.InputTokens,
						CompletionTokens: event.Message.Usage.OutputTokens,
						TotalTokens:      event.Message.Usage.InputTokens + event.Message.Usage.OutputTokens,
					},
				}:
				}
				return
			}
		}
	}()

	return ch, nil
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
			claudeMsg.Content = []claudeContent{
				{Type: "text", Text: msg.Content},
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
