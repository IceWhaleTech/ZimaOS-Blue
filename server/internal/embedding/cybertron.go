package embedding

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nlpodyssey/cybertron/pkg/models/bert"
	"github.com/nlpodyssey/cybertron/pkg/tasks"
	"github.com/nlpodyssey/cybertron/pkg/tasks/textencoding"
)

// ErrNotReady is returned when the embedding model is still loading.
// Callers should skip vector search and fall back to FTS or no-op.
var ErrNotReady = errors.New("embedding model not ready")

// CybertronProvider implements Provider using cybertron (pure Go, local inference).
// Model loading is started asynchronously; Embed() returns ErrNotReady until loaded.
type CybertronProvider struct {
	cfg        CybertronConfig
	encoder    textencoding.Interface
	dimensions int
	ready      atomic.Bool
	initErr    error
	startOnce  sync.Once
	mu         sync.RWMutex
}

// CybertronConfig holds cybertron embedding configuration.
type CybertronConfig struct {
	ModelsDir  string        // Directory to store downloaded models
	Model      string        // HuggingFace model name (e.g. "BAAI/bge-small-zh-v1.5")
	Dimensions int           // Expected embedding dimensions (0 = auto-detect)
	Timeout    time.Duration // unused for local, kept for interface compat
}

// Default cybertron models.
const (
	DefaultCybertronModel = "BAAI/bge-small-zh-v1.5"
	CybertronModelLaBSE   = "sentence-transformers/LaBSE"
	CybertronModelMiniLM  = "sentence-transformers/all-MiniLM-L6-v2"
)

// NewCybertronProvider creates a new local embedding provider.
// The model is NOT downloaded here — call StartAsync() or it auto-starts on first Embed().
func NewCybertronProvider(cfg CybertronConfig) *CybertronProvider {
	if cfg.Model == "" {
		cfg.Model = DefaultCybertronModel
	}
	if cfg.ModelsDir == "" {
		cfg.ModelsDir = "models"
	}
	return &CybertronProvider{cfg: cfg}
}

// StartAsync begins model loading in the background.
// Embed() calls return ErrNotReady until loading completes.
func (p *CybertronProvider) StartAsync() {
	p.startOnce.Do(func() {
		go p.loadModel()
	})
}

// IsReady returns true if the model is loaded and ready for inference.
func (p *CybertronProvider) IsReady() bool {
	return p.ready.Load()
}

// loadModel downloads and loads the model synchronously (called from goroutine).
func (p *CybertronProvider) loadModel() {
	encoder, err := tasks.Load[textencoding.Interface](&tasks.Config{
		ModelsDir: p.cfg.ModelsDir,
		ModelName: p.cfg.Model,
	})
	if err != nil {
		p.mu.Lock()
		p.initErr = fmt.Errorf("load cybertron model %q: %w", p.cfg.Model, err)
		p.mu.Unlock()
		return
	}

	// Auto-detect dimensions
	dims := p.cfg.Dimensions
	if dims == 0 {
		probe, err := encoder.Encode(context.Background(), "hello", int(bert.MeanPooling))
		if err != nil {
			p.mu.Lock()
			p.initErr = fmt.Errorf("probe embedding for dimension detection: %w", err)
			p.mu.Unlock()
			return
		}
		dims = len(probe.Vector.Data().F32())
	}

	p.mu.Lock()
	p.encoder = encoder
	p.dimensions = dims
	p.mu.Unlock()
	p.ready.Store(true)
}

func (p *CybertronProvider) Name() string  { return "cybertron" }
func (p *CybertronProvider) Model() string { return p.cfg.Model }
func (p *CybertronProvider) Dimensions() int {
	if p.dimensions == 0 && p.cfg.Dimensions > 0 {
		return p.cfg.Dimensions
	}
	return p.dimensions
}

// Embed generates an embedding for a single text.
// Returns ErrNotReady if the model is still loading.
func (p *CybertronProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	// Auto-start on first call if not already started
	p.StartAsync()

	if !p.ready.Load() {
		return nil, ErrNotReady
	}
	p.mu.RLock()
	if p.initErr != nil {
		p.mu.RUnlock()
		return nil, p.initErr
	}
	encoder := p.encoder
	p.mu.RUnlock()

	result, err := encoder.Encode(ctx, text, int(bert.MeanPooling))
	if err != nil {
		return nil, fmt.Errorf("cybertron encode: %w", err)
	}
	return result.Vector.Data().F32(), nil
}

// EmbedBatch generates embeddings for multiple texts.
// Returns ErrNotReady if the model is still loading.
func (p *CybertronProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	p.StartAsync()
	if !p.ready.Load() {
		return nil, ErrNotReady
	}
	results := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := p.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("embed text %d: %w", i, err)
		}
		results[i] = emb
	}
	return results, nil
}
