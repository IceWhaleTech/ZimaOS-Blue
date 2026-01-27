package embedding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
)

// Provider generates embeddings for text.
type Provider interface {
	// Name returns the provider name.
	Name() string

	// Model returns the model name.
	Model() string

	// Dimensions returns the embedding dimensions.
	Dimensions() int

	// Embed generates an embedding for a single text.
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// Embedding represents a text embedding.
type Embedding struct {
	Text       string    `json:"text"`
	Vector     []float32 `json:"vector"`
	Dimensions int       `json:"dimensions"`
	Provider   string    `json:"provider"`
	Model      string    `json:"model"`
}

// TextHash returns a hash of the text for caching.
func TextHash(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}

// CosineSimilarity calculates the cosine similarity between two vectors.
func CosineSimilarity(a, b []float32) (float32, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("vectors must have the same length: %d != %d", len(a), len(b))
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0, nil
	}

	return float32(dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))), nil
}

// DotProduct calculates the dot product of two vectors.
func DotProduct(a, b []float32) (float32, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("vectors must have the same length: %d != %d", len(a), len(b))
	}

	var result float64
	for i := range a {
		result += float64(a[i]) * float64(b[i])
	}

	return float32(result), nil
}

// EuclideanDistance calculates the Euclidean distance between two vectors.
func EuclideanDistance(a, b []float32) (float32, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("vectors must have the same length: %d != %d", len(a), len(b))
	}

	var sum float64
	for i := range a {
		diff := float64(a[i]) - float64(b[i])
		sum += diff * diff
	}

	return float32(math.Sqrt(sum)), nil
}

// Normalize normalizes a vector to unit length.
func Normalize(v []float32) []float32 {
	var norm float64
	for _, x := range v {
		norm += float64(x) * float64(x)
	}
	norm = math.Sqrt(norm)

	if norm == 0 {
		return v
	}

	result := make([]float32, len(v))
	for i, x := range v {
		result[i] = float32(float64(x) / norm)
	}
	return result
}

// ProviderRegistry manages embedding providers.
type ProviderRegistry struct {
	providers map[string]Provider
}

// NewProviderRegistry creates a new ProviderRegistry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]Provider),
	}
}

// Register registers a provider.
func (r *ProviderRegistry) Register(provider Provider) {
	r.providers[provider.Name()] = provider
}

// Get returns a provider by name.
func (r *ProviderRegistry) Get(name string) (Provider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// List returns all provider names.
func (r *ProviderRegistry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}
