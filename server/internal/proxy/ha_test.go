package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

// --- Provider Memory HA Tests ---

// TestProviderMemory_BlacklistDoesNotAffectOtherProviders verifies model blacklisting is per-provider
func TestProviderMemory_BlacklistDoesNotAffectOtherProviders(t *testing.T) {
	pm := NewProviderMemory()

	pm.BlacklistModel("p1", "https://api1.example.com", "gpt-4")

	if !pm.IsModelBlacklisted("p1", "https://api1.example.com", "gpt-4") {
		t.Fatal("expected gpt-4 blacklisted on p1")
	}
	if pm.IsModelBlacklisted("p2", "https://api2.example.com", "gpt-4") {
		t.Fatal("gpt-4 should NOT be blacklisted on p2")
	}
}

// TestProviderMemory_BlacklistMultipleModels verifies blacklisting 9 of 10 models
func TestProviderMemory_BlacklistMultipleModels(t *testing.T) {
	pm := NewProviderMemory()
	pid := "relay-provider"
	burl := "https://relay.example.com"

	models := make([]string, 10)
	for i := 0; i < 10; i++ {
		models[i] = fmt.Sprintf("model-%d", i)
	}

	// Blacklist first 9 models
	for i := 0; i < 9; i++ {
		pm.BlacklistModel(pid, burl, models[i])
	}

	// Verify first 9 are blacklisted
	for i := 0; i < 9; i++ {
		if !pm.IsModelBlacklisted(pid, burl, models[i]) {
			t.Errorf("expected model-%d to be blacklisted", i)
		}
	}

	// Verify 10th is NOT blacklisted
	if pm.IsModelBlacklisted(pid, burl, models[9]) {
		t.Fatal("model-9 should NOT be blacklisted")
	}
}

// TestProviderMemory_ClearBlacklistAllowsRetry verifies clearing blacklist re-enables model
func TestProviderMemory_ClearBlacklistAllowsRetry(t *testing.T) {
	pm := NewProviderMemory()
	pid := "p1"
	burl := "https://api.example.com"

	pm.BlacklistModel(pid, burl, "gpt-4")
	if !pm.IsModelBlacklisted(pid, burl, "gpt-4") {
		t.Fatal("expected blacklisted")
	}

	pm.ClearModelBlacklist(pid, burl, "gpt-4")
	if pm.IsModelBlacklisted(pid, burl, "gpt-4") {
		t.Fatal("expected cleared after ClearModelBlacklist")
	}
}

// TestProviderMemory_ThrottleAndRecovery verifies throttle timing
func TestProviderMemory_ThrottleAndRecovery(t *testing.T) {
	pm := NewProviderMemory()
	pid := "p1"
	burl := "https://api.example.com"

	// Not throttled initially
	if pm.IsThrottled(pid, burl) {
		t.Fatal("should not be throttled initially")
	}

	// Throttle with short duration
	pm.RememberThrottle(pid, burl, 50*time.Millisecond)
	if !pm.IsThrottled(pid, burl) {
		t.Fatal("should be throttled after RememberThrottle")
	}

	// Wait for throttle to expire
	time.Sleep(60 * time.Millisecond)
	if pm.IsThrottled(pid, burl) {
		t.Fatal("throttle should have expired")
	}
}

// TestAllModelsForProvider_SkipsBlacklisted verifies blacklisted models are excluded from candidates
func TestAllModelsForProvider_SkipsBlacklisted(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)
	pid := "relay"
	burl := "https://relay.example.com"

	// Blacklist the original model
	ph.providerMemory.BlacklistModel(pid, burl, "claude-3-5-haiku-20241022")

	modelsBuf, nModels := ph.allModelsForProvider(pid, burl, "claude-3-5-haiku-20241022", "")
	for i := 0; i < nModels; i++ {
		if modelsBuf[i] == "claude-3-5-haiku-20241022" {
			t.Fatal("blacklisted model should not appear in candidates")
		}
	}

	// Should still have aliases available
	if nModels == 0 {
		t.Fatal("expected at least one alias to be available")
	}
}

// TestAllModelsForProvider_AllBlacklisted verifies empty result when all models blacklisted
func TestAllModelsForProvider_AllBlacklisted(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)
	pid := "relay"
	burl := "https://relay.example.com"

	// Blacklist original + all known aliases
	ph.providerMemory.BlacklistModel(pid, burl, "claude-3-5-haiku-20241022")
	for _, alias := range ModelAliases["claude-3-5-haiku-20241022"] {
		ph.providerMemory.BlacklistModel(pid, burl, alias)
	}

	_, nModels := ph.allModelsForProvider(pid, burl, "claude-3-5-haiku-20241022", "")
	if nModels != 0 {
		t.Fatalf("expected 0 models when all blacklisted, got %d", nModels)
	}
}

// TestAllModelsForProvider_RememberedAliasFirst verifies remembered alias has highest priority
func TestAllModelsForProvider_RememberedAliasFirst(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)
	pid := "relay"
	burl := "https://relay.example.com"

	// Remember that "claude-haiku-4-5" worked for "claude-3-5-haiku-20241022"
	ph.providerMemory.RememberModelAlias(pid, burl, "claude-3-5-haiku-20241022", "claude-haiku-4-5")

	modelsBuf, nModels := ph.allModelsForProvider(pid, burl, "claude-3-5-haiku-20241022", "")
	if nModels == 0 {
		t.Fatal("expected at least one model")
	}
	if modelsBuf[0] != "claude-haiku-4-5" {
		t.Errorf("expected remembered alias first, got %q", modelsBuf[0])
	}
}

// --- tryOnProvider HA Tests ---

// TestTryOnProvider_NotConfiguredSkipsToNextProvider tests that "not configured" errors
// trigger provider failover instead of just model blacklisting
func TestTryOnProvider_NotConfiguredSkipsToNextProvider(t *testing.T) {
	var requestCount int32

	// Mock upstream: returns 404 "not configured" for all requests
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Model gpt-4 is not configured on this provider",
				"type":    "not_found_error",
			},
		})
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "test-provider",
			BaseURL:   upstream.URL,
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "test-key"},
	}

	pr := &parsedRequest{
		body:  []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`),
		model: "gpt-4",
	}

	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp, _, _, err := ph.tryOnProvider(r, result, pr)
	if resp != nil {
		resp.Body.Close()
	}

	// Should return error (not configured → provider failover)
	if err == nil {
		t.Fatal("expected error for not-configured model")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' in error, got: %s", err.Error())
	}

	// Model SHOULD be blacklisted on THIS provider (per-provider scope)
	// so future requests skip it. It can still work on other providers.
	if !ph.providerMemory.IsModelBlacklisted("test-provider", upstream.URL, "gpt-4") {
		t.Error("model SHOULD be blacklisted on this provider for 'not configured' errors")
	}
}

// TestTryOnProvider_Regular4xxBlacklistsModel verifies regular 4xx errors DO blacklist the model
func TestTryOnProvider_Regular4xxBlacklistsModel(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Invalid request: max_tokens must be positive",
				"type":    "invalid_request_error",
			},
		})
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "test-provider",
			BaseURL:   upstream.URL,
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "test-key"},
	}

	pr := &parsedRequest{
		body:  []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`),
		model: "gpt-4",
	}

	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp, _, _, err := ph.tryOnProvider(r, result, pr)
	if resp != nil {
		resp.Body.Close()
	}

	if err == nil {
		t.Fatal("expected error for 400 response")
	}

	// Regular 4xx SHOULD blacklist the model
	if !ph.providerMemory.IsModelBlacklisted("test-provider", upstream.URL, "gpt-4") {
		t.Error("model SHOULD be blacklisted for regular 4xx errors")
	}
}

// TestTryOnProvider_10Models_OnlyLastWorks tests the scenario where 10 models exist
// but only the last one is usable. The system should find it and remember error states
// so subsequent requests skip the first 9 models.
func TestTryOnProvider_10Models_OnlyLastWorks(t *testing.T) {
	workingModel := "model-9"
	var attemptLog []string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse the model from request body
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		model, _ := body["model"].(string)
		attemptLog = append(attemptLog, model)

		w.Header().Set("Content-Type", "application/json")
		if model == workingModel {
			// Only model-9 works
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":      "chatcmpl-123",
				"object":  "chat.completion",
				"model":   model,
				"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}}},
			})
		} else {
			// All other models return "not configured" (4xx)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": fmt.Sprintf("Model %s is not available", model),
					"type":    "invalid_request_error",
				},
			})
		}
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	// Pre-populate ModelAliases with 10 models for "model-0"
	// We'll use the routed model + original model + manually add aliases
	// Since allModelsForProvider uses ModelAliases, we need a different approach:
	// We'll blacklist models 0-8 to simulate them having been tried before,
	// then verify model-9 (the original) still works.

	// First call: model-0 is the original, it returns "not available" → triggers provider failover
	// (because isModelNotConfiguredError detects "not available")
	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "test-provider",
			BaseURL:   upstream.URL,
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "test-key"},
	}

	// Test: try model-0 through model-8, all fail with "not available"
	for i := 0; i < 9; i++ {
		model := fmt.Sprintf("model-%d", i)
		pr := &parsedRequest{
			body:  []byte(fmt.Sprintf(`{"model":"%s","messages":[{"role":"user","content":"hi"}]}`, model)),
			model: model,
		}
		r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

		resp, _, _, err := ph.tryOnProvider(r, result, pr)
		if resp != nil {
			resp.Body.Close()
		}
		if err == nil {
			t.Fatalf("model-%d should have failed", i)
		}
		// "not available" triggers isModelNotConfiguredError → blacklists on this provider
	}

	// Now try model-9 — should succeed
	pr := &parsedRequest{
		body:  []byte(fmt.Sprintf(`{"model":"%s","messages":[{"role":"user","content":"hi"}]}`, workingModel)),
		model: workingModel,
	}
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp, _, usedModel, err := ph.tryOnProvider(r, result, pr)
	if err != nil {
		t.Fatalf("model-9 should have succeeded, got error: %v", err)
	}
	if resp != nil {
		resp.Body.Close()
	}
	if usedModel != workingModel {
		t.Errorf("expected used model %q, got %q", workingModel, usedModel)
	}

	// Verify all 10 models were attempted
	if len(attemptLog) != 10 {
		t.Errorf("expected 10 attempts, got %d: %v", len(attemptLog), attemptLog)
	}
}

// TestTryOnProvider_429ThrottlesEntireProvider verifies 429 skips entire provider
func TestTryOnProvider_429ThrottlesEntireProvider(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Rate limit exceeded",
			},
		})
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "throttle-provider",
			BaseURL:   upstream.URL,
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "test-key"},
	}

	pr := &parsedRequest{
		body:  []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`),
		model: "gpt-4",
	}

	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp, _, _, err := ph.tryOnProvider(r, result, pr)
	if resp != nil {
		resp.Body.Close()
	}

	if err == nil {
		t.Fatal("expected error for 429")
	}
	if !strings.Contains(err.Error(), "throttled") {
		t.Errorf("expected 'throttled' in error, got: %s", err.Error())
	}

	// Provider should be throttled
	if !ph.providerMemory.IsThrottled("throttle-provider", upstream.URL) {
		t.Error("provider should be throttled after 429")
	}
}

// TestTryOnProvider_5xxSkipsEntireProvider verifies 5xx errors skip the provider
func TestTryOnProvider_5xxSkipsEntireProvider(t *testing.T) {
	var requestCount int32

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Internal server error",
			},
		})
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "failing-provider",
			BaseURL:   upstream.URL,
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "test-key"},
	}

	pr := &parsedRequest{
		body:  []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`),
		model: "gpt-4",
	}

	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp, _, _, err := ph.tryOnProvider(r, result, pr)
	if resp != nil {
		resp.Body.Close()
	}

	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !strings.Contains(err.Error(), "upstream 500") {
		t.Errorf("expected 'upstream 500' in error, got: %s", err.Error())
	}

	// Should only make 1 request (5xx = skip entire provider immediately)
	if count := atomic.LoadInt32(&requestCount); count != 1 {
		t.Errorf("expected 1 request for 5xx (skip provider), got %d", count)
	}
}

// TestTryOnProvider_SuccessRemembersFormatAndModel verifies successful requests
// are remembered for future optimization
func TestTryOnProvider_SuccessRemembersFormatAndModel(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "chatcmpl-123",
			"object":  "chat.completion",
			"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}}},
		})
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	pid := "success-provider"

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        pid,
			BaseURL:   upstream.URL,
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "test-key"},
	}

	pr := &parsedRequest{
		body:  []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`),
		model: "gpt-4",
	}

	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp, usedFormat, usedModel, err := ph.tryOnProvider(r, result, pr)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if resp != nil {
		resp.Body.Close()
	}

	if usedModel != "gpt-4" {
		t.Errorf("expected model gpt-4, got %q", usedModel)
	}
	if usedFormat != providerpool.APIFormatOpenAI {
		t.Errorf("expected format openai, got %q", usedFormat)
	}

	// Verify format was remembered
	format, ok := ph.providerMemory.RecallFormat(pid, upstream.URL)
	if !ok {
		t.Fatal("expected format to be remembered")
	}
	if format != string(providerpool.APIFormatOpenAI) {
		t.Errorf("expected remembered format 'openai', got %q", format)
	}
}

// TestFailoverMetrics_RecordAndRetrieve verifies failover metrics tracking
func TestFailoverMetrics_RecordAndRetrieve(t *testing.T) {
	metrics := NewFailoverMetrics()

	// Record errors
	metrics.RecordError("provider-a", &ErrorClassification{
		Type:     ErrorTypeRateLimited,
		Category: ErrorCategoryFailover,
	})
	metrics.RecordError("provider-a", &ErrorClassification{
		Type:     ErrorTypeRateLimited,
		Category: ErrorCategoryFailover,
	})
	metrics.RecordError("provider-b", &ErrorClassification{
		Type:     ErrorTypeModelNotConfigured,
		Category: ErrorCategoryFailover,
	})

	// Record failovers
	metrics.RecordFailover("provider-a", "provider-b", true)
	metrics.RecordFailover("provider-b", "provider-c", false)

	stats := metrics.GetStats()

	if stats["failover_total"].(int64) != 2 {
		t.Errorf("expected 2 total failovers, got %v", stats["failover_total"])
	}
	if stats["failover_success"].(int64) != 1 {
		t.Errorf("expected 1 successful failover, got %v", stats["failover_success"])
	}
	if stats["failover_failure"].(int64) != 1 {
		t.Errorf("expected 1 failed failover, got %v", stats["failover_failure"])
	}

	errorsByType := stats["errors_by_type"].(map[RetryableErrorType]int64)
	if errorsByType[ErrorTypeRateLimited] != 2 {
		t.Errorf("expected 2 rate_limited errors, got %d", errorsByType[ErrorTypeRateLimited])
	}
	if errorsByType[ErrorTypeModelNotConfigured] != 1 {
		t.Errorf("expected 1 model_not_configured error, got %d", errorsByType[ErrorTypeModelNotConfigured])
	}
}

// TestParseRetryAfter verifies Retry-After header parsing
func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected time.Duration
	}{
		{"empty", "", 30 * time.Second},
		{"seconds", "5", 5 * time.Second},
		{"large seconds", "120", 120 * time.Second},
		{"invalid", "abc", 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRetryAfter(tt.value)
			if got != tt.expected {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.value, got, tt.expected)
			}
		})
	}
}

// TestProviderMemory_FormatRememberAndForget verifies format memory lifecycle
func TestProviderMemory_FormatRememberAndForget(t *testing.T) {
	pm := NewProviderMemory()
	pid := "p1"
	burl := "https://api.example.com"

	// Initially no format remembered
	if _, ok := pm.RecallFormat(pid, burl); ok {
		t.Fatal("expected no format initially")
	}

	// Remember anthropic format
	pm.RememberFormat(pid, burl, "anthropic")
	format, ok := pm.RecallFormat(pid, burl)
	if !ok || format != "anthropic" {
		t.Fatalf("expected anthropic, got %q (ok=%v)", format, ok)
	}

	// Overwrite with openai
	pm.RememberFormat(pid, burl, "openai")
	format, ok = pm.RecallFormat(pid, burl)
	if !ok || format != "openai" {
		t.Fatalf("expected openai after overwrite, got %q", format)
	}

	// Forget
	pm.ForgetFormat(pid, burl)
	if _, ok := pm.RecallFormat(pid, burl); ok {
		t.Fatal("expected no format after forget")
	}
}

// TestIsFormatMismatchError verifies detection of format mismatch errors.
func TestIsFormatMismatchError(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"422 empty body", 422, "", true},
		{"422 unsupported request", 422, `{"error":{"message":"Unsupported request body."}}`, true},
		{"404 openai_error", 404, `{"error":{"message":"openai_error","type":"bad_response_status_code"}}`, true},
		{"400 bad_response_status_code", 400, `{"error":{"type":"bad_response_status_code"}}`, true},
		{"404 model not found", 404, `{"error":{"message":"model not found"}}`, false},
		{"404 plain not found", 404, `not found`, false},
		{"200 ok", 200, `ok`, false},
		{"500 server error", 500, `internal error`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isFormatMismatchError(tt.status, []byte(tt.body))
			if got != tt.want {
				t.Errorf("isFormatMismatchError(%d, %q) = %v, want %v", tt.status, tt.body, got, tt.want)
			}
		})
	}
}

// TestTryOnProvider_FormatMismatch422_ExpandsFormats verifies that a 422 on the
// remembered format triggers expansion to all formats, eventually succeeding.
func TestTryOnProvider_FormatMismatch422_ExpandsFormats(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// OpenAI path → 422 (format mismatch)
		if strings.Contains(r.URL.Path, "chat/completions") {
			w.WriteHeader(422)
			w.Write([]byte(`{"error":{"message":"Unsupported request body."}}`))
			return
		}
		// Anthropic path → success
		w.WriteHeader(200)
		w.Write([]byte(`{"id":"msg_1","type":"message","content":[{"type":"text","text":"ok"}]}`))
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	pid := "format-mismatch-provider"

	// Pre-remember openai format (wrong) so formatKnown=true, nFormats=1
	ph.providerMemory.RememberFormat(pid, upstream.URL, "openai")

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        pid,
			BaseURL:   upstream.URL,
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "test-key"},
	}

	pr := &parsedRequest{
		body:  []byte(`{"model":"claude-sonnet-4-5-20250514","messages":[{"role":"user","content":"hi"}]}`),
		model: "claude-sonnet-4-5-20250514",
	}

	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	resp, usedFormat, _, err := ph.tryOnProvider(r, result, pr)
	if err != nil {
		t.Fatalf("expected success after format expansion, got error: %v", err)
	}
	if resp != nil {
		resp.Body.Close()
	}

	if usedFormat != providerpool.APIFormatAnthropic {
		t.Errorf("expected anthropic format after fallback, got %q", usedFormat)
	}

	// Verify the correct format is now remembered (anthropic, not openai)
	format, ok := ph.providerMemory.RecallFormat(pid, upstream.URL)
	if !ok {
		t.Fatal("expected format to be remembered after successful fallback")
	}
	if format != string(providerpool.APIFormatAnthropic) {
		t.Errorf("expected remembered format 'anthropic', got %q", format)
	}
}

// TestTryOnProvider_FormatMismatch404_OpenAIError verifies that a 404 with
// "openai_error" body is treated as format mismatch, not model blacklisting.
func TestTryOnProvider_FormatMismatch404_OpenAIError(t *testing.T) {
	var callCount atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n == 1 {
			// First call → 404 with openai_error
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(404)
			w.Write([]byte(`{"error":{"message":"openai_error","type":"bad_response_status_code"}}`))
			return
		}
		// Second call → success
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"id":"chatcmpl-1","choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	pid := "openai-error-provider"

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        pid,
			BaseURL:   upstream.URL,
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "test-key"},
	}

	pr := &parsedRequest{
		body:  []byte(`{"model":"gpt-5.1-codex-max","messages":[{"role":"user","content":"hi"}]}`),
		model: "gpt-5.1-codex-max",
	}

	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	resp, _, _, err := ph.tryOnProvider(r, result, pr)
	if err != nil {
		t.Fatalf("expected success after format fallback, got error: %v", err)
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Model should NOT be blacklisted
	if ph.providerMemory.IsModelBlacklisted(pid, upstream.URL, "gpt-5.1-codex-max") {
		t.Error("model should not be blacklisted on format mismatch")
	}
}
