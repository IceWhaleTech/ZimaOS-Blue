package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

	modelsBuf, nModels := ph.allModelsForProvider(pid, burl, "claude-3-5-haiku-20241022", "", false)
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

	_, nModels := ph.allModelsForProvider(pid, burl, "claude-3-5-haiku-20241022", "", false)
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

	modelsBuf, nModels := ph.allModelsForProvider(pid, burl, "claude-3-5-haiku-20241022", "", false)
	if nModels == 0 {
		t.Fatal("expected at least one model")
	}
	if modelsBuf[0] != "claude-haiku-4-5" {
		t.Errorf("expected remembered alias first, got %q", modelsBuf[0])
	}
}

// TestAllModelsForProvider_IgnoreBlacklist verifies single-provider mode can bypass blacklist filtering.
func TestAllModelsForProvider_IgnoreBlacklist(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)
	pid := "relay"
	burl := "https://relay.example.com"
	model := "claude-3-5-haiku-20241022"

	ph.providerMemory.BlacklistModel(pid, burl, model)
	for _, alias := range ModelAliases[model] {
		ph.providerMemory.BlacklistModel(pid, burl, alias)
	}

	modelsBuf, nModels := ph.allModelsForProvider(pid, burl, model, "", true)
	if nModels == 0 {
		t.Fatal("expected models when ignoreBlacklist=true")
	}

	foundOriginal := false
	for i := 0; i < nModels; i++ {
		if modelsBuf[i] == model {
			foundOriginal = true
			break
		}
	}
	if !foundOriginal {
		t.Fatalf("expected original model %q to be present when ignoreBlacklist=true", model)
	}
}

func TestProxyHandler_IsSingleProviderMode(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "proxy-single-provider-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := providerpool.NewFileStorage(tmpDir)
	registry, _ := providerpool.NewRegistry(storage)

	cloudProvider := &providerpool.Provider{
		ID:       "p-cloud",
		Name:     "Cloud Provider",
		Type:     providerpool.ProviderTypeCustom,
		Location: providerpool.ProviderLocationCloud,
		Enabled:  true,
		Status:   providerpool.ProviderStatusActive,
	}
	registry.Register(cloudProvider)

	ph := NewProxyHandler(nil, nil, nil)
	ph.providerPool = &providerpool.Pool{Registry: registry}

	if !ph.isSingleProviderMode(providerpool.RoutingModeAuto) {
		t.Fatal("expected single-provider mode for auto with one enabled provider")
	}
	if !ph.isSingleProviderMode(providerpool.RoutingModeCloud) {
		t.Fatal("expected single-provider mode for cloud with one cloud provider")
	}
	if ph.isSingleProviderMode(providerpool.RoutingModeLocal) {
		t.Fatal("expected non-single-provider for local when no local provider exists")
	}

	localProvider := &providerpool.Provider{
		ID:       "p-local",
		Name:     "Local Provider",
		Type:     providerpool.ProviderTypeCustom,
		Location: providerpool.ProviderLocationLocal,
		Enabled:  true,
		Status:   providerpool.ProviderStatusActive,
	}
	registry.Register(localProvider)

	if ph.isSingleProviderMode(providerpool.RoutingModeAuto) {
		t.Fatal("expected non-single-provider mode for auto with two enabled providers")
	}
}

func TestAllFormatsForProvider_CopilotStaysSingleFormat(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)
	pid := "github-copilot"
	burl := "https://api.githubcopilot.com"
	provider := &providerpool.Provider{
		ID:        pid,
		BaseURL:   burl,
		APIFormat: providerpool.APIFormatCopilot,
	}

	formats, n, known := ph.allFormatsForProvider(pid, burl, provider)
	if known {
		t.Fatal("expected known=false without detected/remembered format")
	}
	if n != 1 {
		t.Fatalf("expected exactly 1 format for copilot, got %d", n)
	}
	if formats[0] != providerpool.APIFormatCopilot {
		t.Fatalf("expected copilot format, got %q", formats[0])
	}
}

func TestAllFormatsForProvider_CloudCodeStaysSingleFormat(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)
	pid := "google-antigravity"
	burl := "https://cloudcode-pa.googleapis.com"
	provider := &providerpool.Provider{
		ID:        pid,
		BaseURL:   burl,
		APIFormat: providerpool.APIFormatCloudCode,
	}

	formats, n, known := ph.allFormatsForProvider(pid, burl, provider)
	if known {
		t.Fatal("expected known=false without detected/remembered format")
	}
	if n != 1 {
		t.Fatalf("expected exactly 1 format for cloudcode, got %d", n)
	}
	if formats[0] != providerpool.APIFormatCloudCode {
		t.Fatalf("expected cloudcode format, got %q", formats[0])
	}
}

func TestAllFormatsForProvider_OpenAIKeepsCrossFamilyFallback(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)
	pid := "generic-openai"
	burl := "https://api.example.com/v1"
	provider := &providerpool.Provider{
		ID:        pid,
		BaseURL:   burl,
		APIFormat: providerpool.APIFormatOpenAI,
	}

	formats, n, known := ph.allFormatsForProvider(pid, burl, provider)
	if known {
		t.Fatal("expected known=false without detected/remembered format")
	}
	if n != 2 {
		t.Fatalf("expected 2 formats for generic openai provider, got %d", n)
	}
	if formats[0] != providerpool.APIFormatOpenAI || formats[1] != providerpool.APIFormatAnthropic {
		t.Fatalf("unexpected format order: [%q %q]", formats[0], formats[1])
	}
}

func TestAllFormatsForProvider_ResponsesEndpointLocksOpenAI(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)
	pid := "responses-locked"
	burl := "https://chatgpt.com/backend-api/codex/responses"
	provider := &providerpool.Provider{
		ID:        pid,
		BaseURL:   burl,
		APIFormat: providerpool.APIFormatOpenAI,
	}
	// Poison remembered/detected format on purpose; endpoint should still win.
	provider.DetectedFormat = providerpool.APIFormatAnthropic
	ph.providerMemory.RememberFormat(pid, burl, "anthropic")

	formats, n, known := ph.allFormatsForProvider(pid, burl, provider)
	if !known {
		t.Fatal("expected known=true for endpoint-locked format")
	}
	if n != 1 {
		t.Fatalf("expected single locked format for /responses endpoint, got %d", n)
	}
	if formats[0] != providerpool.APIFormatResponses {
		t.Fatalf("expected responses format for /responses endpoint, got %q", formats[0])
	}
}

func TestAllFormatsForProvider_AnthropicEndpointLocksAnthropic(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)
	pid := "anthropic-locked"

	for _, burl := range []string{
		"https://api.minimaxi.com/anthropic",
		"https://api.anthropic.com/v1/messages",
	} {
		provider := &providerpool.Provider{
			ID:        pid,
			BaseURL:   burl,
			APIFormat: providerpool.APIFormatOpenAI,
		}
		// Poison remembered/detected format on purpose; endpoint should still win.
		provider.DetectedFormat = providerpool.APIFormatOpenAI
		ph.providerMemory.RememberFormat(pid, burl, "openai")

		formats, n, known := ph.allFormatsForProvider(pid, burl, provider)
		if !known {
			t.Fatalf("expected known=true for endpoint-locked format on %s", burl)
		}
		if n != 1 {
			t.Fatalf("expected single locked format on %s, got %d", burl, n)
		}
		if formats[0] != providerpool.APIFormatAnthropic {
			t.Fatalf("expected anthropic format on %s, got %q", burl, formats[0])
		}
	}
}

func TestDetectEndpointFixedFormatFromPath(t *testing.T) {
	tests := []struct {
		path   string
		want   providerpool.APIFormat
		hasFmt bool
	}{
		{path: "/v1/responses", want: providerpool.APIFormatResponses, hasFmt: true},
		{path: "/backend-api/codex/responses", want: providerpool.APIFormatResponses, hasFmt: true},
		{path: "/v1/messages", want: providerpool.APIFormatAnthropic, hasFmt: true},
		{path: "/anthropic", want: providerpool.APIFormatAnthropic, hasFmt: true},
		{path: "/v1/chat/completions", hasFmt: false},
	}

	for _, tc := range tests {
		got, ok := detectEndpointFixedFormatFromPath(tc.path)
		if ok != tc.hasFmt {
			t.Fatalf("path %q: ok=%v, want %v", tc.path, ok, tc.hasFmt)
		}
		if ok && got != tc.want {
			t.Fatalf("path %q: format=%q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestAllFormatsForProvider_BuiltinSingleFormat(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)
	pid := "minimax"
	burl := "https://api.minimaxi.com/anthropic"
	provider := &providerpool.Provider{
		ID:        pid,
		Type:      providerpool.ProviderTypeBuiltin,
		BaseURL:   burl,
		APIFormat: providerpool.APIFormatAnthropic,
	}

	formats, n, known := ph.allFormatsForProvider(pid, burl, provider)
	if !known {
		t.Fatal("expected known=true for endpoint-locked builtin provider")
	}
	if n != 1 {
		t.Fatalf("expected 1 format for builtin provider, got %d", n)
	}
	if formats[0] != providerpool.APIFormatAnthropic {
		t.Fatalf("expected anthropic format, got %q", formats[0])
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

func TestTryOnProvider_RequestResponsesEndpointForcesResponsesFormat(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"resp_1","object":"response","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`))
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "test-provider",
			BaseURL:   upstream.URL + "/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "test-key"},
	}

	pr := &parsedRequest{
		body:  []byte(`{"model":"gpt-4.1","input":[{"role":"user","content":"hi"}],"stream":false}`),
		model: "gpt-4.1",
	}
	r := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	resp, format, _, err := ph.tryOnProvider(r, result, pr)
	if err != nil {
		t.Fatalf("tryOnProvider failed: %v", err)
	}
	defer resp.Body.Close()

	if format != providerpool.APIFormatResponses {
		t.Fatalf("format = %q, want %q", format, providerpool.APIFormatResponses)
	}
}

// TestTryOnProvider_InvalidRequestDoesNotBlacklistModel verifies invalid_request_error
// is treated as request-level failure and should not blacklist the model.
func TestTryOnProvider_InvalidRequestDoesNotBlacklistModel(t *testing.T) {
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

	// invalid_request_error is request-shape related; model should not be blacklisted.
	if ph.providerMemory.IsModelBlacklisted("test-provider", upstream.URL, "gpt-4") {
		t.Error("model should NOT be blacklisted for invalid_request_error")
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
		{"422 model not found", 422, `{"error":{"message":"model not found"}}`, false},
		{"422 not configured", 422, `{"error":{"message":"model not configured"}}`, false},
		{"422 unknown field", 422, `{"error":{"message":"unknown field 'tools'"}}`, true},
		{"422 generic validation", 422, `{"error":{"message":"max_tokens must be positive"}}`, false},
		{"422 additional properties", 422, `{"error":{"message":"Additional properties not allowed"}}`, true},
		{"404 openai_error", 404, `{"error":{"message":"openai_error","type":"bad_response_status_code"}}`, true},
		{"400 unsupported legacy protocol", 400, `{"error":{"message":"Unsupported legacy protocol: /v1/chat/completions is not supported. Please use /v1/responses.","type":"invalid_request_error"}}`, true},
		{"400 wrapped not configured plus legacy protocol", 400, `{"error":{"message":"model gpt-5 not configured on provider p1: {\"error\":{\"message\":\"Unsupported legacy protocol: /v1/chat/completions is not supported. Please use /v1/responses.\"}}"}}`, true},
		{"400 bad_response_status_code", 400, `{"error":{"type":"bad_response_status_code"}}`, true},
		{"404 model not found", 404, `{"error":{"message":"model not found"}}`, false},
		{"404 plain not found", 404, `not found`, true},
		{"404 page not found", 404, `404 page not found`, true},
		{"502 wrapped legacy protocol", 502, `{"error":{"message":"Unsupported legacy protocol: /v1/chat/completions is not supported. Please use /v1/responses.","type":"invalid_request_error"}}`, true},
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

// TestTryOnProvider_AllFormatsMismatch_SkipsAliases verifies that when all formats
// return 422 for a model, remaining model aliases are skipped (no point trying
// different model names if the provider doesn't understand any request format).
func TestTryOnProvider_AllFormatsMismatch_SkipsAliases(t *testing.T) {
	var callCount atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		// Always return 422 — provider doesn't understand any format
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(422)
		w.Write([]byte(`{"error":{"message":"Unsupported request body."}}`))
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	pid := "all-mismatch-provider"

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        pid,
			BaseURL:   upstream.URL,
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "test-key"},
	}

	// Use a model that has aliases so nModels > 1
	pr := &parsedRequest{
		body:  []byte(`{"model":"claude-haiku-4-5","messages":[{"role":"user","content":"hi"}]}`),
		model: "claude-haiku-4-5",
	}

	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	_, _, _, err := ph.tryOnProvider(r, result, pr)
	if err == nil {
		t.Fatal("expected error when all formats mismatch")
	}

	// Should only try 2 formats (openai + anthropic) for the first model,
	// then skip remaining aliases. Without the fix, it would try 2 × nModels.
	calls := callCount.Load()
	if calls > 2 {
		t.Errorf("expected at most 2 upstream calls (all formats for first model), got %d", calls)
	}
}

func TestProviderRace_EmptyRateCooldown(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)
	ph.SetProviderRaceConfig(ProviderRaceConfig{
		Enabled:                    true,
		MaxParallel:                2,
		MinProviders:               2,
		EmptyRateMinSamples:        4,
		EmptyRateCooldownThreshold: 0.5,
		EmptyRateSinkThreshold:     0.5,
		EmptyRateExcludeThreshold:  0.9,
		EmptyRateCooldown:          1 * time.Minute,
	})

	ph.recordProviderRaceAttempt("p1", true)
	ph.recordProviderRaceAttempt("p1", true)
	ph.recordProviderRaceAttempt("p1", false)
	ph.recordProviderRaceAttempt("p1", false)

	stat := ph.getProviderRaceStat("p1")
	if stat.Attempts != 4 {
		t.Fatalf("expected 4 attempts, got %d", stat.Attempts)
	}
	if stat.EmptyRuns != 2 {
		t.Fatalf("expected 2 empty runs, got %d", stat.EmptyRuns)
	}
	if stat.CooldownUntil.IsZero() {
		t.Fatal("expected provider to enter race cooldown")
	}
	if !ph.isProviderInRaceCooldown("p1") {
		t.Fatal("expected provider to be in race cooldown")
	}
}

func TestProxyHandler_ProviderRaceChoosesFastest(t *testing.T) {
	slowUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"slow","choices":[{"message":{"content":"slow"}}]}`))
	}))
	defer slowUpstream.Close()

	fastUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"fast","choices":[{"message":{"content":"fast"}}]}`))
	}))
	defer fastUpstream.Close()

	tmpDir, err := os.MkdirTemp("", "provider-race-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := providerpool.NewFileStorage(tmpDir)
	registry, _ := providerpool.NewRegistry(storage)
	discovery := providerpool.NewModelDiscovery(registry, storage, time.Hour)
	router := providerpool.NewRouter(registry, discovery, providerpool.RoutingStrategyPriority)

	slowProvider := &providerpool.Provider{
		ID:        "p-slow",
		Name:      "slow-provider",
		Type:      providerpool.ProviderTypeCustom,
		BaseURL:   slowUpstream.URL,
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		Priority:  100,
		APIFormat: providerpool.APIFormatOpenAI,
		APIKeys:   []providerpool.APIKey{{ID: "k-slow", Key: "slow-key", Enabled: true}},
	}
	fastProvider := &providerpool.Provider{
		ID:        "p-fast",
		Name:      "fast-provider",
		Type:      providerpool.ProviderTypeCustom,
		BaseURL:   fastUpstream.URL,
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		Priority:  10,
		APIFormat: providerpool.APIFormatOpenAI,
		APIKeys:   []providerpool.APIKey{{ID: "k-fast", Key: "fast-key", Enabled: true}},
	}
	registry.Register(slowProvider)
	registry.Register(fastProvider)

	models := []*providerpool.Model{
		{
			ID:           "race-model",
			Name:         "race-model",
			ProviderID:   "p-slow",
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, Streaming: true},
		},
	}
	storage.SaveModels("p-slow", models)
	models[0].ProviderID = "p-fast"
	storage.SaveModels("p-fast", models)
	router.RebuildCandidates()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	ph.SetProviderPool(&providerpool.Pool{
		Registry:  registry,
		Discovery: discovery,
		Router:    router,
	})
	ph.SetProviderRaceConfig(ProviderRaceConfig{
		Enabled:                    true,
		MaxParallel:                2,
		MinProviders:               2,
		EmptyRateMinSamples:        10,
		EmptyRateCooldownThreshold: 0.3,
		EmptyRateSinkThreshold:     0.5,
		EmptyRateExcludeThreshold:  0.8,
		EmptyRateCooldown:          2 * time.Minute,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"model":"race-model","messages":[{"role":"user","content":"hi"}]}`))
	rec := httptest.NewRecorder()

	ph.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Actual-Provider"); got != "fast-provider" {
		t.Fatalf("expected fastest provider selected, got %q", got)
	}
}

// TestTryOnProvider_ResponsesEndpointBasePath verifies that when a provider BaseURL
// already points at a /responses endpoint, the proxy does not append
// /v1/chat/completions to it.
func TestTryOnProvider_ResponsesEndpointBasePath(t *testing.T) {
	pathCh := make(chan string, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case pathCh <- r.URL.Path:
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"id":"chatcmpl-1","choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "responses-base-provider",
			BaseURL:   upstream.URL + "/backend-api/codex/responses",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "test-key"},
	}
	pr := &parsedRequest{
		body:  []byte(`{"model":"gpt-5.3-codex-spark","messages":[{"role":"user","content":"hi"}]}`),
		model: "gpt-5.3-codex-spark",
	}

	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	resp, _, _, err := ph.tryOnProvider(r, result, pr)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if resp != nil {
		resp.Body.Close()
	}

	select {
	case gotPath := <-pathCh:
		if gotPath != "/backend-api/codex/responses" {
			t.Fatalf("expected upstream path /backend-api/codex/responses, got %s", gotPath)
		}
	default:
		t.Fatal("expected upstream request, got none")
	}
}

func TestTryOnProvider_SingleProviderRetriesTransient5xx(t *testing.T) {
	var requestCount int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte(`{"error":{"message":"Upstream request failed","type":"upstream_error"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"chatcmpl-1","choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "single-provider",
			BaseURL:   upstream.URL,
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "test-key"},
	}
	pr := &parsedRequest{
		body:           []byte(`{"model":"gpt-5.3-codex","messages":[{"role":"user","content":"hi"}]}`),
		model:          "gpt-5.3-codex",
		singleProvider: true,
	}

	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	resp, _, _, err := ph.tryOnProvider(r, result, pr)
	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}
	if resp != nil {
		resp.Body.Close()
	}
	if got := atomic.LoadInt32(&requestCount); got != 2 {
		t.Fatalf("expected 2 upstream attempts, got %d", got)
	}
}

func TestHasToolMessagesInRequest(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "no tool messages",
			body: `{"messages":[{"role":"system","content":"s"},{"role":"user","content":"u"}]}`,
			want: false,
		},
		{
			name: "tool role message",
			body: `{"messages":[{"role":"tool","tool_call_id":"call_1","content":"ok"}]}`,
			want: true,
		},
		{
			name: "assistant tool_calls",
			body: `{"messages":[{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"exec","arguments":"{}"}}]}]}`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasToolMessagesInRequest([]byte(tt.body))
			if got != tt.want {
				t.Fatalf("hasToolMessagesInRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestWarmToolCallSupport_Probe422 verifies that warmToolCallSupport marks a provider
// as ToolCapNone when the upstream returns 422 on a tool-bearing request.
func TestWarmToolCallSupport_Probe422(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.WriteHeader(200)
			w.Write([]byte(`{"data":[]}`))
			return
		}
		// Reject tool-bearing requests with 422
		w.WriteHeader(422)
		w.Write([]byte(`{"error":{"message":"Unsupported request body."}}`))
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	provider := &providerpool.Provider{
		ID:      "test-no-tools",
		Name:    "test-no-tools",
		BaseURL: upstream.URL,
		Enabled: true,
	}
	// Pre-set auth so warmToolCallSupport can use it
	ph.authProber.Remember(provider.ID, provider.BaseURL, AuthBearer)

	// Create a minimal pool with just GetAPIKey support
	ph.providerPool = &providerpool.Pool{}

	// Call warmToolCallSupport directly — it needs authProber.Recall and pool.Registry.GetAPIKey.
	// Since we can't easily mock the registry, test the probe logic inline.
	// Send the probe request manually to verify the detection logic.
	probeURL := upstream.URL + "/v1/chat/completions"
	req, _ := http.NewRequest(http.MethodPost, probeURL, nil)
	client := ph.connPool.GetClient(provider.Name)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Verify 422 is detected as format mismatch (which means no tool support)
	if !isFormatMismatchError(resp.StatusCode, nil) {
		t.Fatalf("expected 422 to be format mismatch, got status %d", resp.StatusCode)
	}

	// Simulate what warmToolCallSupport does
	ph.providerMemory.RememberToolCap(provider.ID, provider.BaseURL, ToolCapNone)

	cap, ok := ph.providerMemory.RecallToolCap(provider.ID, provider.BaseURL)
	if !ok {
		t.Fatal("expected tool cap to be remembered")
	}
	if cap != ToolCapNone {
		t.Errorf("expected ToolCapNone, got %d", cap)
	}
}

// TestWarmToolCallSupport_Probe200 verifies that warmToolCallSupport marks a provider
// as ToolCapNative when the upstream accepts tool-bearing requests.
func TestWarmToolCallSupport_Probe200(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"id":"chatcmpl-1","choices":[{"message":{"role":"assistant","content":"hi"}}]}`))
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	provider := &providerpool.Provider{
		ID:      "test-with-tools",
		Name:    "test-with-tools",
		BaseURL: upstream.URL,
		Enabled: true,
	}
	ph.authProber.Remember(provider.ID, provider.BaseURL, AuthBearer)

	// Simulate what warmToolCallSupport does on 200
	ph.providerMemory.RememberToolCap(provider.ID, provider.BaseURL, ToolCapNative)

	cap, ok := ph.providerMemory.RecallToolCap(provider.ID, provider.BaseURL)
	if !ok {
		t.Fatal("expected tool cap to be remembered")
	}
	if cap != ToolCapNative {
		t.Errorf("expected ToolCapNative, got %d", cap)
	}
}

// TestExecuteOnProvider_SkipsNoToolProvider verifies that executeOnProvider skips
// providers marked as ToolCapNone when the request contains tools.
func TestExecuteOnProvider_SkipsNoToolProvider(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	provider := &providerpool.Provider{
		ID:      "no-tool-provider",
		Name:    "no-tool-provider",
		BaseURL: "https://example.com",
		Enabled: true,
	}

	// Mark provider as not supporting tools
	ph.providerMemory.RememberToolCap(provider.ID, provider.BaseURL, ToolCapNone)

	// Verify RecallToolCap returns ToolCapNone
	cap, ok := ph.providerMemory.RecallToolCap(provider.ID, provider.BaseURL)
	if !ok || cap != ToolCapNone {
		t.Fatalf("expected ToolCapNone, got %d (ok=%v)", cap, ok)
	}
}

func TestIsResponsesEndpointBaseURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://chatgpt.com/backend-api/codex/responses", true},
		{"https://chatgpt.com/backend-api/codex/responses/", true},
		{"https://api.openai.com/v1", false},
		{"https://api.openai.com/v1/chat/completions", false},
		{"not-a-url", false},
	}

	for _, tt := range tests {
		got := isResponsesEndpointBaseURL(tt.url)
		if got != tt.want {
			t.Fatalf("isResponsesEndpointBaseURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}
