package embedding

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nlpodyssey/cybertron/pkg/models/bert"
	"github.com/nlpodyssey/cybertron/pkg/tasks"
	"github.com/nlpodyssey/cybertron/pkg/tasks/textencoding"
)

// CybertronProvider implements Provider using cybertron (pure Go, local inference).
// Model is lazily downloaded and loaded on first Embed() call.
type CybertronProvider struct {
	cfg        CybertronConfig
	encoder    textencoding.Interface
	dimensions int
	once       sync.Once
	initErr    error
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
// The model is NOT downloaded here — it's lazily loaded on first Embed() call.
func NewCybertronProvider(cfg CybertronConfig) *CybertronProvider {
	if cfg.Model == "" {
		cfg.Model = DefaultCybertronModel
	}
	if cfg.ModelsDir == "" {
		cfg.ModelsDir = "models"
	}
	return &CybertronProvider{cfg: cfg}
}

// ensureLoaded lazily loads the model on first use.
func (p *CybertronProvider) ensureLoaded() error {
	p.once.Do(func() {
		encoder, err := tasks.Load[textencoding.Interface](&tasks.Config{
			ModelsDir: p.cfg.ModelsDir,
			ModelName: p.cfg.Model,
		})
		if err != nil {
			p.initErr = fmt.Errorf("load cybertron model %q: %w", p.cfg.Model, err)
			return
		}
		p.encoder = encoder

		// Auto-detect dimensions
		dims := p.cfg.Dimensions
		if dims == 0 {
			probe, err := encoder.Encode(context.Background(), "hello", int(bert.MeanPooling))
			if err != nil {
				p.initErr = fmt.Errorf("probe embedding for dimension detection: %w", err)
				return
			}
			dims = len(probe.Vector.Data().F32())
		}
		p.dimensions = dims
	})
	return p.initErr
}

func (p *CybertronProvider) Name() string  { return "cybertron" }
func (p *CybertronProvider) Model() string { return p.cfg.Model }
func (p *CybertronProvider) Dimensions() int {
	if p.dimensions == 0 {
		_ = p.ensureLoaded()
	}
	return p.dimensions
}

// Embed generates an embedding for a single text.
func (p *CybertronProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	if err := p.ensureLoaded(); err != nil {
		return nil, err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()

	result, err := p.encoder.Encode(ctx, text, int(bert.MeanPooling))
	if err != nil {
		return nil, fmt.Errorf("cybertron encode: %w", err)
	}
	return result.Vector.Data().F32(), nil
}

// EmbedBatch generates embeddings for multiple texts.
func (p *CybertronProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if err := p.ensureLoaded(); err != nil {
		return nil, err
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
