package proxy

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// MockConfig mock endpoint configuration
type MockConfig struct {
	Enabled   bool            `json:"enabled"`
	Endpoints []*MockEndpoint `json:"endpoints"`
}

// DefaultMockConfig returns default mock configuration
func DefaultMockConfig() *MockConfig {
	return &MockConfig{
		Enabled:   false,
		Endpoints: make([]*MockEndpoint, 0),
	}
}

// MockEndpoint defines a mock endpoint
type MockEndpoint struct {
	Path       string        `json:"path"`
	Method     string        `json:"method"`
	Response   interface{}   `json:"response"`
	StatusCode int           `json:"status_code"`
	Delay      time.Duration `json:"delay"`
	Enabled    bool          `json:"enabled"`
	Headers    http.Header   `json:"headers,omitempty"`
}

// MockHandler handles mock endpoints
type MockHandler struct {
	config    *MockConfig
	endpoints map[string]*MockEndpoint // path+method -> endpoint
	mu        sync.RWMutex

	// Stats
	hitCount map[string]int64
}

// NewMockHandler creates a new mock handler
func NewMockHandler(config *MockConfig) *MockHandler {
	if config == nil {
		config = DefaultMockConfig()
	}

	mh := &MockHandler{
		config:    config,
		endpoints: make(map[string]*MockEndpoint),
		hitCount:  make(map[string]int64),
	}
	mh.loadDefaultEndpoints()
	mh.indexEndpoints()
	return mh
}

// loadDefaultEndpoints loads built-in mock endpoints
func (mh *MockHandler) loadDefaultEndpoints() {
	defaults := []*MockEndpoint{
		{
			Path:       "/v1/models",
			Method:     "GET",
			StatusCode: 200,
			Response: map[string]interface{}{
				"object": "list",
				"data": []map[string]interface{}{
					{"id": "claude-opus-4-5", "object": "model", "owned_by": "anthropic"},
					{"id": "claude-sonnet-4-5", "object": "model", "owned_by": "anthropic"},
					{"id": "gpt-4o", "object": "model", "owned_by": "openai"},
					{"id": "gpt-4-turbo", "object": "model", "owned_by": "openai"},
					{"id": "llama3", "object": "model", "owned_by": "meta"},
				},
			},
			Enabled: true,
		},
		{
			Path:       "/health",
			Method:     "GET",
			StatusCode: 200,
			Response:   map[string]string{"status": "healthy"},
			Enabled:    true,
		},
		{
			Path:       "/ready",
			Method:     "GET",
			StatusCode: 200,
			Response:   map[string]string{"status": "ready"},
			Enabled:    true,
		},
		{
			Path:       "/v1/chat/completions/mock",
			Method:     "POST",
			StatusCode: 200,
			Response: map[string]interface{}{
				"id":      "mock-response-001",
				"object":  "chat.completion",
				"created": time.Now().Unix(),
				"model":   "mock-model",
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"message": map[string]string{
							"role":    "assistant",
							"content": "This is a mock response for testing purposes.",
						},
						"finish_reason": "stop",
					},
				},
				"usage": map[string]int{
					"prompt_tokens":     10,
					"completion_tokens": 15,
					"total_tokens":      25,
				},
			},
			Enabled: true,
		},
	}

	// Prepend defaults to config endpoints
	mh.config.Endpoints = append(defaults, mh.config.Endpoints...)
}

// indexEndpoints indexes endpoints for fast lookup
func (mh *MockHandler) indexEndpoints() {
	mh.mu.Lock()
	defer mh.mu.Unlock()

	for _, ep := range mh.config.Endpoints {
		if ep.Enabled {
			key := mh.makeKey(ep.Method, ep.Path)
			mh.endpoints[key] = ep
		}
	}
}

// makeKey creates a lookup key from method and path
func (mh *MockHandler) makeKey(method, path string) string {
	return method + ":" + path
}

// Handle checks if request matches a mock endpoint
func (mh *MockHandler) Handle(w http.ResponseWriter, r *http.Request) bool {
	if !mh.config.Enabled {
		return false
	}

	mh.mu.RLock()
	key := mh.makeKey(r.Method, r.URL.Path)
	ep, ok := mh.endpoints[key]
	mh.mu.RUnlock()

	if !ok {
		return false
	}

	// Record hit
	mh.mu.Lock()
	mh.hitCount[key]++
	mh.mu.Unlock()

	// Apply delay
	if ep.Delay > 0 {
		time.Sleep(ep.Delay)
	}

	// Set custom headers
	for key, values := range ep.Headers {
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Mock-Response", "true")
	w.WriteHeader(ep.StatusCode)
	json.NewEncoder(w).Encode(ep.Response)

	return true
}

// AddEndpoint adds a mock endpoint
func (mh *MockHandler) AddEndpoint(ep *MockEndpoint) {
	mh.mu.Lock()
	defer mh.mu.Unlock()

	// Set defaults
	if ep.StatusCode == 0 {
		ep.StatusCode = 200
	}
	if ep.Method == "" {
		ep.Method = "GET"
	}

	mh.config.Endpoints = append(mh.config.Endpoints, ep)
	if ep.Enabled {
		key := mh.makeKey(ep.Method, ep.Path)
		mh.endpoints[key] = ep
	}
}

// RemoveEndpoint removes a mock endpoint
func (mh *MockHandler) RemoveEndpoint(path, method string) bool {
	mh.mu.Lock()
	defer mh.mu.Unlock()

	key := mh.makeKey(method, path)
	if _, ok := mh.endpoints[key]; !ok {
		return false
	}

	delete(mh.endpoints, key)
	delete(mh.hitCount, key)

	// Remove from config
	for i, ep := range mh.config.Endpoints {
		if ep.Path == path && ep.Method == method {
			mh.config.Endpoints = append(
				mh.config.Endpoints[:i],
				mh.config.Endpoints[i+1:]...,
			)
			break
		}
	}

	return true
}

// SetEnabled enables or disables a mock endpoint
func (mh *MockHandler) SetEnabled(path, method string, enabled bool) bool {
	mh.mu.Lock()
	defer mh.mu.Unlock()

	key := mh.makeKey(method, path)

	// Find in config
	for _, ep := range mh.config.Endpoints {
		if ep.Path == path && ep.Method == method {
			ep.Enabled = enabled
			if enabled {
				mh.endpoints[key] = ep
			} else {
				delete(mh.endpoints, key)
			}
			return true
		}
	}

	return false
}

// GetEndpoint returns a mock endpoint
func (mh *MockHandler) GetEndpoint(path, method string) (*MockEndpoint, bool) {
	mh.mu.RLock()
	defer mh.mu.RUnlock()

	key := mh.makeKey(method, path)
	ep, ok := mh.endpoints[key]
	if !ok {
		return nil, false
	}

	// Return a copy
	copy := *ep
	return &copy, true
}

// ListEndpoints returns all mock endpoints
func (mh *MockHandler) ListEndpoints() []*MockEndpoint {
	mh.mu.RLock()
	defer mh.mu.RUnlock()

	endpoints := make([]*MockEndpoint, 0, len(mh.config.Endpoints))
	for _, ep := range mh.config.Endpoints {
		copy := *ep
		endpoints = append(endpoints, &copy)
	}
	return endpoints
}

// SetGlobalEnabled enables or disables all mock endpoints
func (mh *MockHandler) SetGlobalEnabled(enabled bool) {
	mh.mu.Lock()
	defer mh.mu.Unlock()

	mh.config.Enabled = enabled
}

// IsEnabled returns whether mock handling is enabled
func (mh *MockHandler) IsEnabled() bool {
	mh.mu.RLock()
	defer mh.mu.RUnlock()

	return mh.config.Enabled
}

// Stats returns mock handler statistics
func (mh *MockHandler) Stats() map[string]interface{} {
	mh.mu.RLock()
	defer mh.mu.RUnlock()

	totalHits := int64(0)
	for _, count := range mh.hitCount {
		totalHits += count
	}

	return map[string]interface{}{
		"enabled":        mh.config.Enabled,
		"endpoint_count": len(mh.endpoints),
		"total_hits":     totalHits,
		"hit_counts":     mh.hitCount,
	}
}

// ResetStats resets hit counters
func (mh *MockHandler) ResetStats() {
	mh.mu.Lock()
	defer mh.mu.Unlock()

	mh.hitCount = make(map[string]int64)
}
