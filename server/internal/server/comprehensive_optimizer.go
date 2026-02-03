package server

import (
	"sync"
)

// ComprehensiveOptimizer manages all optimization strategies
type ComprehensiveOptimizer struct {
	// Core optimizers
	chatOptimizer    *ChatCallChainOptimizer
	pipeline         *RequestPipeline
	cacheStrategy    *SmartCacheStrategy
	circuitBreakers  map[string]*CircuitBreaker

	// Metrics
	totalOptimized   int64
	totalDeduped     int64
	totalPrefetched  int64

	mu sync.RWMutex
}

// NewComprehensiveOptimizer creates a new comprehensive optimizer
func NewComprehensiveOptimizer() *ComprehensiveOptimizer {
	return &ComprehensiveOptimizer{
		chatOptimizer:   NewChatCallChainOptimizer(),
		cacheStrategy:   NewSmartCacheStrategy(),
		circuitBreakers: make(map[string]*CircuitBreaker),
	}
}

// GetOrCreateCircuitBreaker gets or creates a circuit breaker for a provider
func (co *ComprehensiveOptimizer) GetOrCreateCircuitBreaker(provider string) *CircuitBreaker {
	co.mu.Lock()
	defer co.mu.Unlock()

	if cb, exists := co.circuitBreakers[provider]; exists {
		return cb
	}

	cb := NewCircuitBreaker(provider, 5, 30*1000000000) // 30 seconds
	co.circuitBreakers[provider] = cb
	return cb
}

// GetAllMetrics returns comprehensive metrics
func (co *ComprehensiveOptimizer) GetAllMetrics() map[string]interface{} {
	co.mu.RLock()
	defer co.mu.RUnlock()

	metrics := map[string]interface{}{
		"chat_optimization": co.chatOptimizer.GetOptimizationStats(),
		"performance":       co.chatOptimizer.GetMetrics(),
		"usage_stats":       co.chatOptimizer.GetAllUsageStats(),
		"circuit_breakers":  co.getCircuitBreakerStates(),
	}

	return metrics
}

// getCircuitBreakerStates returns all circuit breaker states
func (co *ComprehensiveOptimizer) getCircuitBreakerStates() map[string]interface{} {
	states := make(map[string]interface{})
	for provider, cb := range co.circuitBreakers {
		states[provider] = cb.GetState()
	}
	return states
}

// Close closes all resources
func (co *ComprehensiveOptimizer) Close() {
	co.chatOptimizer.Close()
	if co.pipeline != nil {
		co.pipeline.Stop()
	}
}
