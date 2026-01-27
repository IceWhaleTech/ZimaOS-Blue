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

// OllamaProvider implements Provider for Ollama embeddings.
type OllamaProvider struct {
	client     *http.Client
	baseURL    string
	model      string
	dimensions int
}

// OllamaConfig holds Ollama embedding configuration.
type OllamaConfig struct {
	BaseURL    string
	Model      string
	Dimensions int
	Timeout    time.Duration
}

// NewOllamaProvider creates a new OllamaProvider.
func NewOllamaProvider(cfg OllamaConfig) (*OllamaProvider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	model := cfg.Model
	if model == "" {
		model = "nomic-embed-text"
	}

	dimensions := cfg.Dimensions
	if dimensions == 0 {
		// Default dimensions for common models
		switch model {
		case "nomic-embed-text":
			dimensions = 768
		case "mxbai-embed-large":
			dimensions = 1024
		case "all-minilm":
			dimensions = 384
		default:
			dimensions = 768
		}
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	return &OllamaProvider{
		client: &http.Client{
			Timeout: timeout,
		},
		baseURL:    baseURL,
		model:      model,
		dimensions: dimensions,
	}, nil
}

// Name returns the provider name.
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// Model returns the model name.
func (p *OllamaProvider) Model() string {
	return p.model
}

// Dimensions returns the embedding dimensions.
func (p *OllamaProvider) Dimensions() int {
	return p.dimensions
}

// ollamaEmbeddingRequest represents an Ollama embedding request.
type ollamaEmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// ollamaEmbeddingResponse represents an Ollama embedding response.
type ollamaEmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

// ollamaErrorResponse represents an Ollama error response.
type ollamaErrorResponse struct {
	Error string `json:"error"`
}

// Embed generates an embedding for a single text.
func (p *OllamaProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := ollamaEmbeddingRequest{
		Model:  p.model,
		Prompt: text,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

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
		var errResp ollamaErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error != "" {
			return nil, fmt.Errorf("Ollama API error: %s", errResp.Error)
		}
		return nil, fmt.Errorf("Ollama API error: status %d", resp.StatusCode)
	}

	var embResp ollamaEmbeddingResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return embResp.Embedding, nil
}

// EmbedBatch generates embeddings for multiple texts.
// Note: Ollama doesn't support batch embeddings natively, so we call Embed for each text.
func (p *OllamaProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))

	for i, text := range texts {
		embedding, err := p.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to embed text %d: %w", i, err)
		}
		embeddings[i] = embedding
	}

	return embeddings, nil
}
