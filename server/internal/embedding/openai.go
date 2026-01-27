package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIProvider implements Provider for OpenAI embeddings.
type OpenAIProvider struct {
	client     *http.Client
	apiKey     string
	baseURL    string
	model      string
	dimensions int
}

// OpenAI embedding models and their dimensions.
var openAIEmbeddingModels = map[string]int{
	"text-embedding-3-small": 1536,
	"text-embedding-3-large": 3072,
	"text-embedding-ada-002": 1536,
}

// OpenAIConfig holds OpenAI embedding configuration.
type OpenAIConfig struct {
	APIKey     string
	BaseURL    string
	Model      string
	Dimensions int
	Timeout    time.Duration
}

// NewOpenAIProvider creates a new OpenAIProvider.
func NewOpenAIProvider(cfg OpenAIConfig) (*OpenAIProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}

	model := cfg.Model
	if model == "" {
		model = "text-embedding-3-small"
	}

	dimensions := cfg.Dimensions
	if dimensions == 0 {
		if d, ok := openAIEmbeddingModels[model]; ok {
			dimensions = d
		} else {
			dimensions = 1536
		}
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &OpenAIProvider{
		client: &http.Client{
			Timeout: timeout,
		},
		apiKey:     cfg.APIKey,
		baseURL:    baseURL,
		model:      model,
		dimensions: dimensions,
	}, nil
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// Model returns the model name.
func (p *OpenAIProvider) Model() string {
	return p.model
}

// Dimensions returns the embedding dimensions.
func (p *OpenAIProvider) Dimensions() int {
	return p.dimensions
}

// openAIEmbeddingRequest represents an OpenAI embedding request.
type openAIEmbeddingRequest struct {
	Input          interface{} `json:"input"`
	Model          string      `json:"model"`
	EncodingFormat string      `json:"encoding_format,omitempty"`
	Dimensions     int         `json:"dimensions,omitempty"`
}

// openAIEmbeddingResponse represents an OpenAI embedding response.
type openAIEmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// openAIErrorResponse represents an OpenAI error response.
type openAIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// Embed generates an embedding for a single text.
func (p *OpenAIProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts.
func (p *OpenAIProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	reqBody := openAIEmbeddingRequest{
		Input: texts,
		Model: p.model,
	}

	// For text-embedding-3 models, we can specify dimensions
	if p.model == "text-embedding-3-small" || p.model == "text-embedding-3-large" {
		reqBody.Dimensions = p.dimensions
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp openAIErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("OpenAI API error: %s", errResp.Error.Message)
		}
		return nil, fmt.Errorf("OpenAI API error: status %d", resp.StatusCode)
	}

	var embResp openAIEmbeddingResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Sort by index and extract embeddings
	embeddings := make([][]float32, len(texts))
	for _, data := range embResp.Data {
		if data.Index < len(embeddings) {
			embeddings[data.Index] = data.Embedding
		}
	}

	return embeddings, nil
}
