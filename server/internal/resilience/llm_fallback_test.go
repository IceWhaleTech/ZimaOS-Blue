package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockLLMProvider is a mock LLM provider for testing
type MockLLMProvider struct {
	name        string
	shouldFail  bool
	failCount   int
	maxFails    int
	latency     time.Duration
	healthError error
}

func NewMockLLMProvider(name string) *MockLLMProvider {
	return &MockLLMProvider{name: name}
}

func (m *MockLLMProvider) Name() string {
	return m.name
}

func (m *MockLLMProvider) Execute(ctx context.Context, request interface{}) (interface{}, error) {
	if m.latency > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(m.latency):
		}
	}

	if m.shouldFail {
		if m.maxFails > 0 && m.failCount >= m.maxFails {
			m.shouldFail = false
			return map[string]string{"result": "success"}, nil
		}
		m.failCount++
		return nil, errors.New("provider error")
	}

	return map[string]string{"result": "success"}, nil
}

func (m *MockLLMProvider) HealthCheck(ctx context.Context) error {
	return m.healthError
}

func (m *MockLLMProvider) SetShouldFail(fail bool) {
	m.shouldFail = fail
	m.failCount = 0
}

func (m *MockLLMProvider) SetMaxFails(max int) {
	m.maxFails = max
}

func (m *MockLLMProvider) SetLatency(latency time.Duration) {
	m.latency = latency
}

func (m *MockLLMProvider) SetHealthError(err error) {
	m.healthError = err
}

func TestLLMFallbackChain(t *testing.T) {
	t.Run("executes with primary provider", func(t *testing.T) {
		chain := NewLLMFallbackChain(LLMFallbackConfig{})

		provider := NewMockLLMProvider("primary")
		chain.AddProvider(provider, 10, 100)

		result, err := chain.Execute(context.Background(), "test request")

		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("falls back to secondary provider", func(t *testing.T) {
		chain := NewLLMFallbackChain(LLMFallbackConfig{
			MaxRetries: 0,
		})

		primary := NewMockLLMProvider("primary")
		primary.SetShouldFail(true)
		chain.AddProvider(primary, 10, 100)

		secondary := NewMockLLMProvider("secondary")
		chain.AddProvider(secondary, 5, 50)

		result, err := chain.Execute(context.Background(), "test request")

		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("returns error when all providers fail", func(t *testing.T) {
		chain := NewLLMFallbackChain(LLMFallbackConfig{
			MaxRetries: 0,
		})

		primary := NewMockLLMProvider("primary")
		primary.SetShouldFail(true)
		chain.AddProvider(primary, 10, 100)

		secondary := NewMockLLMProvider("secondary")
		secondary.SetShouldFail(true)
		chain.AddProvider(secondary, 5, 50)

		_, err := chain.Execute(context.Background(), "test request")

		assert.ErrorIs(t, err, ErrAllProvidersFailed)
	})

	t.Run("retries on failure", func(t *testing.T) {
		chain := NewLLMFallbackChain(LLMFallbackConfig{
			MaxRetries: 2,
			RetryDelay: 10 * time.Millisecond,
		})

		provider := NewMockLLMProvider("primary")
		provider.SetShouldFail(true)
		provider.SetMaxFails(2) // Fail twice, then succeed
		chain.AddProvider(provider, 10, 100)

		result, err := chain.Execute(context.Background(), "test request")

		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		chain := NewLLMFallbackChain(LLMFallbackConfig{
			MaxRetries: 5,
			RetryDelay: 100 * time.Millisecond,
		})

		provider := NewMockLLMProvider("primary")
		provider.SetShouldFail(true)
		chain.AddProvider(provider, 10, 100)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := chain.Execute(ctx, "test request")

		assert.Error(t, err)
	})

	t.Run("calls OnProviderSwitch callback", func(t *testing.T) {
		called := false
		chain := NewLLMFallbackChain(LLMFallbackConfig{
			MaxRetries: 0,
			OnProviderSwitch: func(from, to string) {
				called = true
				assert.Equal(t, "primary", from)
				assert.Equal(t, "secondary", to)
			},
		})

		primary := NewMockLLMProvider("primary")
		primary.SetShouldFail(true)
		chain.AddProvider(primary, 10, 100)

		secondary := NewMockLLMProvider("secondary")
		chain.AddProvider(secondary, 5, 50)

		chain.Execute(context.Background(), "test request")

		// Wait for callback
		time.Sleep(50 * time.Millisecond)
		assert.True(t, called)
	})

	t.Run("calls OnAllFailed callback", func(t *testing.T) {
		called := false
		chain := NewLLMFallbackChain(LLMFallbackConfig{
			MaxRetries: 0,
			OnAllFailed: func(err error) {
				called = true
			},
		})

		provider := NewMockLLMProvider("primary")
		provider.SetShouldFail(true)
		chain.AddProvider(provider, 10, 100)

		chain.Execute(context.Background(), "test request")

		// Wait for callback
		time.Sleep(50 * time.Millisecond)
		assert.True(t, called)
	})

	t.Run("removes provider", func(t *testing.T) {
		chain := NewLLMFallbackChain(LLMFallbackConfig{})

		chain.AddProvider(NewMockLLMProvider("primary"), 10, 100)
		chain.AddProvider(NewMockLLMProvider("secondary"), 5, 50)

		assert.Equal(t, 2, chain.ProviderCount())

		chain.RemoveProvider("primary")

		assert.Equal(t, 1, chain.ProviderCount())
	})

	t.Run("set and get provider status", func(t *testing.T) {
		chain := NewLLMFallbackChain(LLMFallbackConfig{})

		chain.AddProvider(NewMockLLMProvider("primary"), 10, 100)

		chain.SetProviderStatus("primary", ProviderDegraded)

		status, ok := chain.GetProviderStatus("primary")
		assert.True(t, ok)
		assert.Equal(t, ProviderDegraded, status)
	})

	t.Run("get non-existent provider status", func(t *testing.T) {
		chain := NewLLMFallbackChain(LLMFallbackConfig{})

		_, ok := chain.GetProviderStatus("nonexistent")
		assert.False(t, ok)
	})

	t.Run("stats returns all providers", func(t *testing.T) {
		chain := NewLLMFallbackChain(LLMFallbackConfig{})

		chain.AddProvider(NewMockLLMProvider("primary"), 10, 100)
		chain.AddProvider(NewMockLLMProvider("secondary"), 5, 50)

		stats := chain.Stats()
		assert.Len(t, stats, 2)
		assert.Contains(t, stats, "primary")
		assert.Contains(t, stats, "secondary")
	})

	t.Run("reset clears all statistics", func(t *testing.T) {
		chain := NewLLMFallbackChain(LLMFallbackConfig{})

		provider := NewMockLLMProvider("primary")
		chain.AddProvider(provider, 10, 100)

		// Execute some requests
		chain.Execute(context.Background(), "test")

		chain.Reset()

		stats := chain.Stats()
		providerStats := stats["primary"].(map[string]interface{})
		assert.Equal(t, int64(0), providerStats["success_count"])
	})

	t.Run("get active provider", func(t *testing.T) {
		chain := NewLLMFallbackChain(LLMFallbackConfig{})

		chain.AddProvider(NewMockLLMProvider("primary"), 10, 100)
		chain.AddProvider(NewMockLLMProvider("secondary"), 5, 50)

		active := chain.GetActiveProvider()
		assert.Equal(t, "primary", active)
	})

	t.Run("healthy provider count", func(t *testing.T) {
		chain := NewLLMFallbackChain(LLMFallbackConfig{})

		chain.AddProvider(NewMockLLMProvider("primary"), 10, 100)
		chain.AddProvider(NewMockLLMProvider("secondary"), 5, 50)

		assert.Equal(t, 2, chain.HealthyProviderCount())

		chain.SetProviderStatus("primary", ProviderUnhealthy)

		assert.Equal(t, 1, chain.HealthyProviderCount())
	})

	t.Run("skips unhealthy providers", func(t *testing.T) {
		chain := NewLLMFallbackChain(LLMFallbackConfig{})

		primary := NewMockLLMProvider("primary")
		chain.AddProvider(primary, 10, 100)

		secondary := NewMockLLMProvider("secondary")
		chain.AddProvider(secondary, 5, 50)

		chain.SetProviderStatus("primary", ProviderUnhealthy)

		result, err := chain.Execute(context.Background(), "test")

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "secondary", chain.GetActiveProvider())
	})

	t.Run("health check updates status", func(t *testing.T) {
		chain := NewLLMFallbackChain(LLMFallbackConfig{})

		provider := NewMockLLMProvider("primary")
		chain.AddProvider(provider, 10, 100)

		chain.HealthCheck(context.Background())

		// Wait for health check to complete
		time.Sleep(50 * time.Millisecond)

		status, _ := chain.GetProviderStatus("primary")
		assert.Equal(t, ProviderHealthy, status)
	})

	t.Run("empty chain returns error", func(t *testing.T) {
		chain := NewLLMFallbackChain(LLMFallbackConfig{})

		_, err := chain.Execute(context.Background(), "test")

		assert.ErrorIs(t, err, ErrAllProvidersFailed)
	})
}

func TestLLMProviderStatus(t *testing.T) {
	t.Run("status string representation", func(t *testing.T) {
		assert.Equal(t, "healthy", ProviderHealthy.String())
		assert.Equal(t, "degraded", ProviderDegraded.String())
		assert.Equal(t, "unhealthy", ProviderUnhealthy.String())
		assert.Equal(t, "unknown", LLMProviderStatus(99).String())
	})
}
