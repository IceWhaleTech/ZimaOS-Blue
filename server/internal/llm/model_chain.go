package llm

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

// ModelChain manages a chain of models for failover.
type ModelChain struct {
	name        string
	description string
	models      []*ChainedModel
	mu          sync.RWMutex
}

// ChainedModel represents a model in the chain with health tracking.
type ChainedModel struct {
	Provider   string
	Model      string
	Priority   int
	Weight     int
	MaxRetries int
	Timeout    time.Duration

	// Health tracking
	mu           sync.RWMutex
	healthy      bool
	successCount int64
	failureCount int64
	lastSuccess  time.Time
	lastFailure  time.Time
	lastError    error
	avgLatency   time.Duration
	latencySum   time.Duration
	latencyCount int64
}

// NewModelChain creates a new ModelChain from configuration.
func NewModelChain(cfg config.ModelChainConfig) *ModelChain {
	chain := &ModelChain{
		name:        cfg.Name,
		description: cfg.Description,
		models:      make([]*ChainedModel, 0, len(cfg.Models)),
	}

	for _, m := range cfg.Models {
		chain.models = append(chain.models, &ChainedModel{
			Provider:   m.Provider,
			Model:      m.Model,
			Priority:   m.Priority,
			Weight:     m.Weight,
			MaxRetries: m.MaxRetries,
			Timeout:    m.Timeout,
			healthy:    true, // Start as healthy
		})
	}

	// Sort by priority (higher first)
	sort.Slice(chain.models, func(i, j int) bool {
		return chain.models[i].Priority > chain.models[j].Priority
	})

	return chain
}

// Name returns the chain name.
func (c *ModelChain) Name() string {
	return c.name
}

// Description returns the chain description.
func (c *ModelChain) Description() string {
	return c.description
}

// GetNextModel returns the next healthy model to try.
func (c *ModelChain) GetNextModel() *ChainedModel {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, m := range c.models {
		if m.IsHealthy() {
			return m
		}
	}

	// If no healthy models, return the first one (might recover)
	if len(c.models) > 0 {
		return c.models[0]
	}
	return nil
}

// GetModels returns all models in the chain.
func (c *ModelChain) GetModels() []*ChainedModel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.models
}

// GetHealthyModels returns only healthy models.
func (c *ModelChain) GetHealthyModels() []*ChainedModel {
	c.mu.RLock()
	defer c.mu.RUnlock()

	healthy := make([]*ChainedModel, 0)
	for _, m := range c.models {
		if m.IsHealthy() {
			healthy = append(healthy, m)
		}
	}
	return healthy
}

// IsHealthy returns true if the model is healthy.
func (m *ChainedModel) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.healthy
}

// SetHealthy sets the health status.
func (m *ChainedModel) SetHealthy(healthy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthy = healthy
}

// RecordSuccess records a successful request.
func (m *ChainedModel) RecordSuccess(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.successCount++
	m.lastSuccess = time.Now()
	m.healthy = true
	m.latencySum += latency
	m.latencyCount++
	if m.latencyCount > 0 {
		m.avgLatency = m.latencySum / time.Duration(m.latencyCount)
	}
}

// RecordFailure records a failed request.
func (m *ChainedModel) RecordFailure(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.failureCount++
	m.lastFailure = time.Now()
	m.lastError = err
}

// Stats returns the model statistics.
func (m *ChainedModel) Stats() ModelStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return ModelStats{
		Provider:     m.Provider,
		Model:        m.Model,
		Priority:     m.Priority,
		Healthy:      m.healthy,
		SuccessCount: m.successCount,
		FailureCount: m.failureCount,
		LastSuccess:  m.lastSuccess,
		LastFailure:  m.lastFailure,
		AvgLatency:   m.avgLatency,
	}
}

// ModelStats holds model statistics.
type ModelStats struct {
	Provider     string        `json:"provider"`
	Model        string        `json:"model"`
	Priority     int           `json:"priority"`
	Healthy      bool          `json:"healthy"`
	SuccessCount int64         `json:"success_count"`
	FailureCount int64         `json:"failure_count"`
	LastSuccess  time.Time     `json:"last_success,omitempty"`
	LastFailure  time.Time     `json:"last_failure,omitempty"`
	AvgLatency   time.Duration `json:"avg_latency"`
}

// ModelChainManager manages multiple model chains.
type ModelChainManager struct {
	chains     map[string]*ModelChain
	default_   *ModelChain
	registry   *ProviderRegistry
	classifier ErrorClassifier
	mu         sync.RWMutex
}

// NewModelChainManager creates a new ModelChainManager.
func NewModelChainManager(cfg config.LLMConfig, registry *ProviderRegistry) *ModelChainManager {
	mgr := &ModelChainManager{
		chains:     make(map[string]*ModelChain),
		registry:   registry,
		classifier: NewDefaultErrorClassifier(),
	}

	for _, chainCfg := range cfg.Chains {
		chain := NewModelChain(chainCfg)
		mgr.chains[chain.Name()] = chain
		if chainCfg.Default {
			mgr.default_ = chain
		}
	}

	// If no default set, use the first chain
	if mgr.default_ == nil && len(mgr.chains) > 0 {
		for _, chain := range mgr.chains {
			mgr.default_ = chain
			break
		}
	}

	return mgr
}

// GetChain returns a chain by name.
func (m *ModelChainManager) GetChain(name string) (*ModelChain, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	chain, ok := m.chains[name]
	return chain, ok
}

// GetDefaultChain returns the default chain.
func (m *ModelChainManager) GetDefaultChain() *ModelChain {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.default_
}

// ListChains returns all chain names.
func (m *ModelChainManager) ListChains() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.chains))
	for name := range m.chains {
		names = append(names, name)
	}
	return names
}

// ExecuteWithFailover executes a request with automatic failover.
func (m *ModelChainManager) ExecuteWithFailover(
	ctx context.Context,
	chainName string,
	req ChatRequest,
) (*ChatResponse, error) {
	chain, ok := m.GetChain(chainName)
	if !ok {
		chain = m.GetDefaultChain()
	}
	if chain == nil {
		return nil, fmt.Errorf("no model chain available")
	}

	var lastErr error
	models := chain.GetModels()

	for _, model := range models {
		// Skip unhealthy models unless it's the last one
		if !model.IsHealthy() && model != models[len(models)-1] {
			continue
		}

		// Get provider
		provider := m.registry.Get(model.Provider)
		if provider == nil {
			continue
		}

		// Set model in request
		req.Model = model.Model

		// Execute with retries
		for attempt := 0; attempt <= model.MaxRetries; attempt++ {
			start := time.Now()

			// Create context with timeout
			execCtx := ctx
			if model.Timeout > 0 {
				var cancel context.CancelFunc
				execCtx, cancel = context.WithTimeout(ctx, model.Timeout)
				defer cancel()
			}

			resp, err := provider.Chat(execCtx, req)
			latency := time.Since(start)

			if err == nil {
				model.RecordSuccess(latency)
				return resp, nil
			}

			model.RecordFailure(err)
			lastErr = err

			// Check if we should retry
			if !m.classifier.ShouldRetry(err) {
				break
			}

			// Wait before retry
			if attempt < model.MaxRetries {
				delay := m.classifier.GetRetryDelay(err, attempt)
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
				}
			}
		}

		// Check if we should failover
		if !m.classifier.ShouldFailover(lastErr) {
			break
		}

		// Mark model as unhealthy for failover
		model.SetHealthy(false)
	}

	return nil, fmt.Errorf("all models failed: %w", lastErr)
}

// StreamWithFailover executes a streaming request with automatic failover.
func (m *ModelChainManager) StreamWithFailover(
	ctx context.Context,
	chainName string,
	req ChatRequest,
) (<-chan StreamChunk, error) {
	chain, ok := m.GetChain(chainName)
	if !ok {
		chain = m.GetDefaultChain()
	}
	if chain == nil {
		return nil, fmt.Errorf("no model chain available")
	}

	var lastErr error
	models := chain.GetModels()

	for _, model := range models {
		if !model.IsHealthy() && model != models[len(models)-1] {
			continue
		}

		provider := m.registry.Get(model.Provider)
		if provider == nil {
			continue
		}

		req.Model = model.Model

		execCtx := ctx
		if model.Timeout > 0 {
			var cancel context.CancelFunc
			execCtx, cancel = context.WithTimeout(ctx, model.Timeout)
			defer cancel()
		}

		start := time.Now()
		stream, err := provider.ChatStream(execCtx, req)
		if err == nil {
			// Wrap stream to record success on completion
			return m.wrapStream(stream, model, start), nil
		}

		model.RecordFailure(err)
		lastErr = err

		if !m.classifier.ShouldFailover(err) {
			break
		}
		model.SetHealthy(false)
	}

	return nil, fmt.Errorf("all models failed: %w", lastErr)
}

// wrapStream wraps a stream channel to record success on completion.
func (m *ModelChainManager) wrapStream(
	in <-chan StreamChunk,
	model *ChainedModel,
	start time.Time,
) <-chan StreamChunk {
	out := make(chan StreamChunk)
	go func() {
		defer close(out)
		for chunk := range in {
			out <- chunk
			if chunk.Done {
				model.RecordSuccess(time.Since(start))
			}
		}
	}()
	return out
}

// HealthCheck performs health checks on all models.
func (m *ModelChainManager) HealthCheck(ctx context.Context) map[string][]ModelStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]ModelStats)
	for name, chain := range m.chains {
		stats := make([]ModelStats, 0)
		for _, model := range chain.GetModels() {
			stats = append(stats, model.Stats())
		}
		result[name] = stats
	}
	return result
}
