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
	defaultOllamaBaseURL = "http://localhost:11434"
	ollamaTimeout        = 300 * time.Second // Ollama can be slow for large models
)

// OllamaProvider implements the Provider interface for Ollama.
type OllamaProvider struct {
	baseURL string
	client  *http.Client
}

// NewOllamaProvider creates a new Ollama provider.
func NewOllamaProvider(baseURL string) *OllamaProvider {
	if baseURL == "" {
		baseURL = defaultOllamaBaseURL
	}
	return &OllamaProvider{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: ollamaTimeout,
		},
	}
}

// Name returns the provider name.
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// Models returns the list of commonly available models.
// In production, this could query /api/tags for installed models.
func (p *OllamaProvider) Models() []string {
	return []string{
		"llama3.2",
		"llama3.1",
		"llama3",
		"mistral",
		"mixtral",
		"codellama",
		"phi3",
		"gemma2",
		"qwen2.5",
	}
}

// ollamaRequest represents the Ollama API request format.
type ollamaRequest struct {
	Model    string           `json:"model"`
	Messages []ollamaMessage  `json:"messages"`
	Stream   bool             `json:"stream"`
	Options  *ollamaOptions   `json:"options,omitempty"`
	Tools    []ollamaTool     `json:"tools,omitempty"`
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

type ollamaTool struct {
	Type     string         `json:"type"`
	Function ollamaFunction `json:"function"`
}

type ollamaFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type ollamaToolCall struct {
	Function struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	} `json:"function"`
}

// ollamaResponse represents the Ollama API response format.
type ollamaResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Message   struct {
		Role      string           `json:"role"`
		Content   string           `json:"content"`
		ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
	} `json:"message"`
	Done              bool   `json:"done"`
	TotalDuration     int64  `json:"total_duration"`
	PromptEvalCount   int    `json:"prompt_eval_count"`
	EvalCount         int    `json:"eval_count"`
	Error             string `json:"error,omitempty"`
}

// Chat sends a chat completion request to Ollama.
func (p *OllamaProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Convert to Ollama format
	ollamaReq := p.convertRequest(req)
	ollamaReq.Stream = false

	// Marshal request
	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

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
	var ollamaResp ollamaResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for API error
	if ollamaResp.Error != "" {
		return nil, fmt.Errorf("Ollama API error: %s", ollamaResp.Error)
	}

	// Check for HTTP error
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama API returned status %d", resp.StatusCode)
	}

	// Convert response
	return p.convertResponse(ollamaResp), nil
}

// ChatStream sends a streaming chat completion request to Ollama.
func (p *OllamaProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	// Convert to Ollama format
	ollamaReq := p.convertRequest(req)
	ollamaReq.Stream = true

	// Marshal request
	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Check for HTTP error
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("Ollama API returned status %d", resp.StatusCode)
	}

	// Create channel for streaming
	ch := make(chan StreamChunk, 100)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		decoder := json.NewDecoder(resp.Body)
		var totalPromptTokens, totalCompletionTokens int

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			var chunk ollamaResponse
			if err := decoder.Decode(&chunk); err != nil {
				if err == io.EOF {
					return
				}
				continue
			}

			if chunk.Error != "" {
				return
			}

			totalPromptTokens = chunk.PromptEvalCount
			totalCompletionTokens = chunk.EvalCount

			streamChunk := StreamChunk{
				Model: chunk.Model,
				Delta: chunk.Message.Content,
				Done:  chunk.Done,
			}

			if chunk.Done {
				streamChunk.Usage = &Usage{
					PromptTokens:     totalPromptTokens,
					CompletionTokens: totalCompletionTokens,
					TotalTokens:      totalPromptTokens + totalCompletionTokens,
				}
			}

			select {
			case <-ctx.Done():
				return
			case ch <- streamChunk:
			}

			if chunk.Done {
				return
			}
		}
	}()

	return ch, nil
}

// convertRequest converts a ChatRequest to Ollama format.
func (p *OllamaProvider) convertRequest(req ChatRequest) ollamaRequest {
	messages := make([]ollamaMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = ollamaMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	ollamaReq := ollamaRequest{
		Model:    req.Model,
		Messages: messages,
	}

	// Add options if specified
	if req.Temperature > 0 || req.MaxTokens > 0 {
		ollamaReq.Options = &ollamaOptions{
			Temperature: req.Temperature,
			NumPredict:  req.MaxTokens,
		}
	}

	// Add tools if specified
	if len(req.Tools) > 0 {
		ollamaReq.Tools = make([]ollamaTool, len(req.Tools))
		for i, tool := range req.Tools {
			ollamaReq.Tools[i] = ollamaTool{
				Type: "function",
				Function: ollamaFunction{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.Parameters,
				},
			}
		}
	}

	return ollamaReq
}

// convertResponse converts an Ollama response to ChatResponse.
func (p *OllamaProvider) convertResponse(resp ollamaResponse) *ChatResponse {
	message := Message{
		Role:    RoleAssistant,
		Content: resp.Message.Content,
	}

	// Convert tool calls
	if len(resp.Message.ToolCalls) > 0 {
		message.ToolCalls = make([]ToolCall, len(resp.Message.ToolCalls))
		for i, tc := range resp.Message.ToolCalls {
			argsJSON, _ := json.Marshal(tc.Function.Arguments)
			message.ToolCalls[i] = ToolCall{
				ID:        fmt.Sprintf("call_%d", i),
				Name:      tc.Function.Name,
				Arguments: string(argsJSON),
			}
		}
	}

	return &ChatResponse{
		ID:      resp.CreatedAt, // Ollama doesn't have a unique ID, use timestamp
		Model:   resp.Model,
		Message: message,
		Usage: Usage{
			PromptTokens:     resp.PromptEvalCount,
			CompletionTokens: resp.EvalCount,
			TotalTokens:      resp.PromptEvalCount + resp.EvalCount,
		},
	}
}
