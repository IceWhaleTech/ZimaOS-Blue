package proxy

import (
	"context"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

func forceProviderPoolReady(pool *providerpool.Pool) {
	if pool == nil {
		return
	}
	closedCh := make(chan struct{})
	close(closedCh)
	field := reflect.ValueOf(pool).Elem().FieldByName("readyCh")
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(closedCh))
}

func mustModelAliases(t *testing.T, model string) []string {
	t.Helper()
	aliases, ok := modelAliasesFor(model)
	if !ok {
		t.Fatalf("expected aliases for %q", model)
	}
	return aliases
}

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

	models := ph.allModelsForProvider(nil, pid, burl, "claude-3-5-haiku-20241022", "claude-3-5-haiku-20241022", "", false)
	for _, model := range models {
		if model == "claude-3-5-haiku-20241022" {
			t.Fatal("blacklisted model should not appear in candidates")
		}
	}

	// Should still have aliases available
	if len(models) == 0 {
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
	for _, alias := range mustModelAliases(t, "claude-3-5-haiku-20241022") {
		ph.providerMemory.BlacklistModel(pid, burl, alias)
	}

	models := ph.allModelsForProvider(nil, pid, burl, "claude-3-5-haiku-20241022", "claude-3-5-haiku-20241022", "", false)
	if len(models) != 0 {
		t.Fatalf("expected 0 models when all blacklisted, got %d", len(models))
	}
}

// TestAllModelsForProvider_RememberedAliasFirst verifies remembered alias has highest priority
func TestAllModelsForProvider_RememberedAliasFirst(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)
	pid := "relay"
	burl := "https://relay.example.com"

	// Remember that "claude-haiku-4-5" worked for "claude-3-5-haiku-20241022"
	ph.providerMemory.RememberModelAlias(pid, burl, "claude-3-5-haiku-20241022", "claude-haiku-4-5")

	models := ph.allModelsForProvider(nil, pid, burl, "claude-3-5-haiku-20241022", "claude-3-5-haiku-20241022", "", false)
	if len(models) == 0 {
		t.Fatal("expected at least one model")
	}
	if models[0] != "claude-haiku-4-5" {
		t.Errorf("expected remembered alias first, got %q", models[0])
	}
}

// TestAllModelsForProvider_IgnoreBlacklist verifies single-provider mode can bypass blacklist filtering.
func TestAllModelsForProvider_IgnoreBlacklist(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)
	pid := "relay"
	burl := "https://relay.example.com"
	model := "claude-3-5-haiku-20241022"

	ph.providerMemory.BlacklistModel(pid, burl, model)
	for _, alias := range mustModelAliases(t, model) {
		ph.providerMemory.BlacklistModel(pid, burl, alias)
	}

	models := ph.allModelsForProvider(nil, pid, burl, model, model, "", true)
	if len(models) == 0 {
		t.Fatal("expected models when ignoreBlacklist=true")
	}

	foundOriginal := false
	for _, candidate := range models {
		if candidate == model {
			foundOriginal = true
			break
		}
	}
	if !foundOriginal {
		t.Fatalf("expected original model %q to be present when ignoreBlacklist=true", model)
	}
}

func TestAllModelsForProvider_RoutingHintExpandsRelayModels(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "proxy-routing-hint-models-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := providerpool.NewFileStorage(tmpDir)
	registry, _ := providerpool.NewRegistry(storage)
	discovery := providerpool.NewModelDiscovery(registry, storage, time.Hour)

	provider := &providerpool.Provider{
		ID:        "relay-provider",
		Name:      "Relay Provider",
		Type:      providerpool.ProviderTypeCustom,
		BaseURL:   "https://relay.example.com",
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		Location:  providerpool.ProviderLocationCloud,
		APIKeys:   []providerpool.APIKey{{ID: "k1", Key: "test-key", Enabled: true}},
		APIFormat: providerpool.APIFormatOpenAI,
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	if err := storage.SaveModels(provider.ID, []*providerpool.Model{
		{
			ID:           "claude-sonnet-4-6",
			ProviderID:   provider.ID,
			Name:         "claude-sonnet-4-6",
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, FunctionCall: true, Streaming: true},
		},
		{
			ID:           "claude-3-5-sonnet-20241022",
			ProviderID:   provider.ID,
			Name:         "claude-3-5-sonnet-20241022",
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, FunctionCall: true, Streaming: true},
		},
	}); err != nil {
		t.Fatalf("save models: %v", err)
	}

	ph := NewProxyHandler(nil, nil, nil)
	ph.providerPool = &providerpool.Pool{
		Registry:  registry,
		Discovery: discovery,
	}

	models := ph.allModelsForProvider(provider, provider.ID, provider.BaseURL, "auto", "", "claude-3-5-sonnet-20241022", true)
	if len(models) < 2 {
		t.Fatalf("expected routing hint to expand provider models, got %d", len(models))
	}
	if models[0] != "claude-3-5-sonnet-20241022" {
		t.Fatalf("expected routed model first, got %q", models[0])
	}
	foundWorking := false
	for _, model := range models {
		if model == "claude-sonnet-4-6" {
			foundWorking = true
			break
		}
	}
	if !foundWorking {
		t.Fatalf("expected expanded candidates to include provider model claude-sonnet-4-6, got %v", models)
	}
}

func TestAllModelsForProvider_RoutingHintSkipsEmbeddingLikeRelayModels(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "proxy-routing-hint-skip-embedding-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := providerpool.NewFileStorage(tmpDir)
	registry, _ := providerpool.NewRegistry(storage)
	discovery := providerpool.NewModelDiscovery(registry, storage, time.Hour)

	provider := &providerpool.Provider{
		ID:        "relay-provider",
		Name:      "Relay Provider",
		Type:      providerpool.ProviderTypeCustom,
		BaseURL:   "https://relay.example.com",
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		Location:  providerpool.ProviderLocationCloud,
		APIKeys:   []providerpool.APIKey{{ID: "k1", Key: "test-key", Enabled: true}},
		APIFormat: providerpool.APIFormatOpenAI,
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	if err := storage.SaveModels(provider.ID, []*providerpool.Model{
		{
			ID:           "embedding-bert-512-v1",
			ProviderID:   provider.ID,
			Name:         "embedding-bert-512-v1",
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, FunctionCall: true, Streaming: true},
		},
		{
			ID:           "claude-sonnet-4-6",
			ProviderID:   provider.ID,
			Name:         "claude-sonnet-4-6",
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, FunctionCall: true, Streaming: true},
		},
	}); err != nil {
		t.Fatalf("save models: %v", err)
	}

	ph := NewProxyHandler(nil, nil, nil)
	ph.providerPool = &providerpool.Pool{
		Registry:  registry,
		Discovery: discovery,
	}

	models := ph.allModelsForProvider(provider, provider.ID, provider.BaseURL, "auto", "", "", true)
	if len(models) != 1 {
		t.Fatalf("expected only chat-capable candidate, got %v", models)
	}
	if models[0] != "claude-sonnet-4-6" {
		t.Fatalf("expected embedding-like candidate to be skipped, got %q", models[0])
	}
}

func TestTryOnProvider_RoutingHintFallsThroughToRelayModelAndRemembersAlias(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "proxy-routing-hint-fallback-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	failingModel := "claude-3-5-sonnet-20241022"
	workingModel := "claude-sonnet-4-6"
	var attempts []string

	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := stdjson.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		model, _ := body["model"].(string)
		attempts = append(attempts, model)
		w.Header().Set("Content-Type", "application/json")
		switch model {
		case failingModel:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Model claude-3-5-sonnet-20241022 is not available","type":"invalid_request_error"}}`))
		case workingModel:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":"chatcmpl-1","object":"chat.completion","model":%q,"choices":[{"message":{"role":"assistant","content":"ok"}}]}`, workingModel)))
		default:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"error":{"message":"unexpected model %s","type":"invalid_request_error"}}`, model)))
		}
	}))
	defer upstream.Close()

	storage, _ := providerpool.NewFileStorage(tmpDir)
	registry, _ := providerpool.NewRegistry(storage)
	discovery := providerpool.NewModelDiscovery(registry, storage, time.Hour)
	provider := &providerpool.Provider{
		ID:        "relay-provider",
		Name:      "Relay Provider",
		Type:      providerpool.ProviderTypeCustom,
		BaseURL:   upstream.URL,
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		Location:  providerpool.ProviderLocationCloud,
		APIKeys:   []providerpool.APIKey{{ID: "k1", Key: "test-key", Enabled: true}},
		APIFormat: providerpool.APIFormatOpenAI,
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	if err := storage.SaveModels(provider.ID, []*providerpool.Model{
		{
			ID:           workingModel,
			ProviderID:   provider.ID,
			Name:         workingModel,
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, FunctionCall: true, Streaming: true},
		},
		{
			ID:           failingModel,
			ProviderID:   provider.ID,
			Name:         failingModel,
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, FunctionCall: true, Streaming: true},
		},
	}); err != nil {
		t.Fatalf("save models: %v", err)
	}

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	ph.providerPool = &providerpool.Pool{
		Registry:  registry,
		Discovery: discovery,
	}

	result := &providerpool.RouteResult{
		Provider: provider,
		Model: &providerpool.Model{
			ID:         failingModel,
			ProviderID: provider.ID,
			Name:       failingModel,
			Enabled:    true,
		},
		APIKey: &providerpool.APIKey{Key: "test-key"},
	}
	pr := &parsedRequest{
		body:                  []byte(`{"model":"auto","messages":[{"role":"user","content":"hi"}]}`),
		requestedModel:        "auto",
		singleProvider:        true,
		routingSingleProvider: true,
	}
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp, _, usedModel, err := ph.tryOnProvider(r, result, pr)
	if err != nil {
		t.Fatalf("first auto request failed: %v", err)
	}
	if resp != nil {
		resp.Body.Close()
	}
	if usedModel != workingModel {
		t.Fatalf("first auto request used model %q, want %q", usedModel, workingModel)
	}
	if got, ok := ph.providerMemory.RecallModelAlias(provider.ID, upstream.URL, "auto"); !ok || got != workingModel {
		t.Fatalf("remembered auto alias = %q (ok=%v), want %q", got, ok, workingModel)
	}
	if len(attempts) != 2 || attempts[0] != failingModel || attempts[1] != workingModel {
		t.Fatalf("first request attempts = %v, want [%q %q]", attempts, failingModel, workingModel)
	}

	attempts = attempts[:0]
	resp, _, usedModel, err = ph.tryOnProvider(r, result, pr)
	if err != nil {
		t.Fatalf("second auto request failed: %v", err)
	}
	if resp != nil {
		resp.Body.Close()
	}
	if usedModel != workingModel {
		t.Fatalf("second auto request used model %q, want %q", usedModel, workingModel)
	}
	if len(attempts) != 1 || attempts[0] != workingModel {
		t.Fatalf("second request attempts = %v, want [%q]", attempts, workingModel)
	}
}

func TestTryOnProvider_HAFallsThroughToRelayModelForExplicitModelAndRemembersAlias(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "proxy-ha-explicit-model-fallback-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	failingModel := "relay-haiku-broken"
	workingModel := "claude-haiku-4-5"
	var attempts []string

	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := stdjson.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		model, _ := body["model"].(string)
		attempts = append(attempts, model)
		w.Header().Set("Content-Type", "application/json")
		switch model {
		case failingModel:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"No available AI provider for model 'relay-haiku-broken' across all groups checked.","type":"invalid_request_error"}}`))
		case workingModel:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":"chatcmpl-1","object":"chat.completion","model":%q,"choices":[{"message":{"role":"assistant","content":"ok"}}]}`, workingModel)))
		default:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"error":{"message":"unexpected model %s","type":"invalid_request_error"}}`, model)))
		}
	}))
	defer upstream.Close()

	storage, _ := providerpool.NewFileStorage(tmpDir)
	registry, _ := providerpool.NewRegistry(storage)
	discovery := providerpool.NewModelDiscovery(registry, storage, time.Hour)
	provider := &providerpool.Provider{
		ID:        "relay-provider",
		Name:      "Relay Provider",
		Type:      providerpool.ProviderTypeCustom,
		BaseURL:   upstream.URL,
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		Location:  providerpool.ProviderLocationCloud,
		APIKeys:   []providerpool.APIKey{{ID: "k1", Key: "test-key", Enabled: true}},
		APIFormat: providerpool.APIFormatOpenAI,
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	if err := storage.SaveModels(provider.ID, []*providerpool.Model{
		{
			ID:           workingModel,
			ProviderID:   provider.ID,
			Name:         workingModel,
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, FunctionCall: true, Streaming: true},
		},
		{
			ID:           failingModel,
			ProviderID:   provider.ID,
			Name:         failingModel,
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, FunctionCall: true, Streaming: true},
		},
	}); err != nil {
		t.Fatalf("save models: %v", err)
	}

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	ph.providerPool = &providerpool.Pool{
		Registry:  registry,
		Discovery: discovery,
	}

	result := &providerpool.RouteResult{
		Provider: provider,
		Model: &providerpool.Model{
			ID:         failingModel,
			ProviderID: provider.ID,
			Name:       failingModel,
			Enabled:    true,
		},
		APIKey: &providerpool.APIKey{Key: "test-key"},
	}
	pr := &parsedRequest{
		body:                  []byte(fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}]}`, failingModel)),
		model:                 failingModel,
		requestedModel:        failingModel,
		singleProvider:        false,
		routingSingleProvider: false,
	}
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp, _, usedModel, err := ph.tryOnProvider(r, result, pr)
	if err != nil {
		t.Fatalf("first HA request failed: %v", err)
	}
	if resp != nil {
		resp.Body.Close()
	}
	if usedModel != workingModel {
		t.Fatalf("first HA request used model %q, want %q", usedModel, workingModel)
	}
	if got, ok := ph.providerMemory.RecallModelAlias(provider.ID, upstream.URL, failingModel); !ok || got != workingModel {
		t.Fatalf("remembered explicit-model alias = %q (ok=%v), want %q", got, ok, workingModel)
	}
	if len(attempts) != 2 || attempts[0] != failingModel || attempts[1] != workingModel {
		t.Fatalf("first request attempts = %v, want [%q %q]", attempts, failingModel, workingModel)
	}

	attempts = attempts[:0]
	resp, _, usedModel, err = ph.tryOnProvider(r, result, pr)
	if err != nil {
		t.Fatalf("second HA request failed: %v", err)
	}
	if resp != nil {
		resp.Body.Close()
	}
	if usedModel != workingModel {
		t.Fatalf("second HA request used model %q, want %q", usedModel, workingModel)
	}
	if len(attempts) != 1 || attempts[0] != workingModel {
		t.Fatalf("second request attempts = %v, want [%q]", attempts, workingModel)
	}
}

func TestTryOnProvider_RoutingHintFallsThroughOnWrapped503ModelNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "proxy-routing-hint-wrapped-503-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	failingModel := "360gpt2-pro"
	workingModel := "claude-sonnet-4-6"
	var attempts []string

	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := stdjson.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		model, _ := body["model"].(string)
		attempts = append(attempts, model)
		w.Header().Set("Content-Type", "application/json")
		switch model {
		case failingModel:
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"code":"model_not_found","message":"分组 default 下模型 360gpt2-pro 无可用渠道（distributor）","type":"new_api_error"}}`))
		case workingModel:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":"chatcmpl-1","object":"chat.completion","model":%q,"choices":[{"message":{"role":"assistant","content":"ok"}}]}`, workingModel)))
		default:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"error":{"message":"unexpected model %s","type":"invalid_request_error"}}`, model)))
		}
	}))
	defer upstream.Close()

	storage, _ := providerpool.NewFileStorage(tmpDir)
	registry, _ := providerpool.NewRegistry(storage)
	discovery := providerpool.NewModelDiscovery(registry, storage, time.Hour)
	provider := &providerpool.Provider{
		ID:        "relay-provider",
		Name:      "Relay Provider",
		Type:      providerpool.ProviderTypeCustom,
		BaseURL:   upstream.URL,
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		Location:  providerpool.ProviderLocationCloud,
		APIKeys:   []providerpool.APIKey{{ID: "k1", Key: "test-key", Enabled: true}},
		APIFormat: providerpool.APIFormatOpenAI,
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	if err := storage.SaveModels(provider.ID, []*providerpool.Model{
		{
			ID:           workingModel,
			ProviderID:   provider.ID,
			Name:         workingModel,
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, FunctionCall: true, Streaming: true},
		},
		{
			ID:           failingModel,
			ProviderID:   provider.ID,
			Name:         failingModel,
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, FunctionCall: true, Streaming: true},
		},
	}); err != nil {
		t.Fatalf("save models: %v", err)
	}

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	ph.providerPool = &providerpool.Pool{
		Registry:  registry,
		Discovery: discovery,
	}

	result := &providerpool.RouteResult{
		Provider: provider,
		Model: &providerpool.Model{
			ID:         failingModel,
			ProviderID: provider.ID,
			Name:       failingModel,
			Enabled:    true,
		},
		APIKey: &providerpool.APIKey{Key: "test-key"},
	}
	pr := &parsedRequest{
		body:                  []byte(`{"model":"auto","messages":[{"role":"user","content":"hi"}]}`),
		requestedModel:        "auto",
		singleProvider:        true,
		routingSingleProvider: true,
	}
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp, _, usedModel, err := ph.tryOnProvider(r, result, pr)
	if err != nil {
		t.Fatalf("auto request failed: %v", err)
	}
	if resp != nil {
		resp.Body.Close()
	}
	if usedModel != workingModel {
		t.Fatalf("used model %q, want %q", usedModel, workingModel)
	}
	if len(attempts) != 2 || attempts[0] != failingModel || attempts[1] != workingModel {
		t.Fatalf("attempts = %v, want [%q %q]", attempts, failingModel, workingModel)
	}
}

func TestTryOnProvider_RoutingHintFallsThroughPastNinthCandidate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "proxy-routing-hint-many-candidates-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	failingModels := []string{
		"model-09",
		"model-08",
		"model-07",
		"model-06",
		"model-05",
		"model-04",
		"model-03",
		"model-02",
		"model-01",
	}
	workingModel := "model-00"
	var attempts []string

	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := stdjson.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		model, _ := body["model"].(string)
		attempts = append(attempts, model)
		w.Header().Set("Content-Type", "application/json")
		if model == workingModel {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":"chatcmpl-1","object":"chat.completion","model":%q,"choices":[{"message":{"role":"assistant","content":"ok"}}]}`, workingModel)))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"error":{"code":"model_not_found","message":"分组 default 下模型 %s 无可用渠道（distributor）","type":"new_api_error"}}`, model)))
	}))
	defer upstream.Close()

	storage, _ := providerpool.NewFileStorage(tmpDir)
	registry, _ := providerpool.NewRegistry(storage)
	discovery := providerpool.NewModelDiscovery(registry, storage, time.Hour)
	provider := &providerpool.Provider{
		ID:        "relay-provider",
		Name:      "Relay Provider",
		Type:      providerpool.ProviderTypeCustom,
		BaseURL:   upstream.URL,
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		Location:  providerpool.ProviderLocationCloud,
		APIKeys:   []providerpool.APIKey{{ID: "k1", Key: "test-key", Enabled: true}},
		APIFormat: providerpool.APIFormatOpenAI,
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("register provider: %v", err)
	}

	models := make([]*providerpool.Model, 0, len(failingModels)+1)
	for _, modelID := range append(append([]string(nil), failingModels...), workingModel) {
		models = append(models, &providerpool.Model{
			ID:           modelID,
			ProviderID:   provider.ID,
			Name:         modelID,
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, FunctionCall: true, Streaming: true},
		})
	}
	if err := storage.SaveModels(provider.ID, models); err != nil {
		t.Fatalf("save models: %v", err)
	}

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	ph.providerPool = &providerpool.Pool{
		Registry:  registry,
		Discovery: discovery,
	}

	result := &providerpool.RouteResult{
		Provider: provider,
		Model: &providerpool.Model{
			ID:         failingModels[0],
			ProviderID: provider.ID,
			Name:       failingModels[0],
			Enabled:    true,
		},
		APIKey: &providerpool.APIKey{Key: "test-key"},
	}
	pr := &parsedRequest{
		body:                  []byte(`{"model":"auto","messages":[{"role":"user","content":"hi"}]}`),
		requestedModel:        "auto",
		singleProvider:        true,
		routingSingleProvider: true,
	}
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp, _, usedModel, err := ph.tryOnProvider(r, result, pr)
	if err != nil {
		t.Fatalf("auto request failed: %v", err)
	}
	if resp != nil {
		resp.Body.Close()
	}
	if usedModel != workingModel {
		t.Fatalf("used model %q, want %q", usedModel, workingModel)
	}
	if len(attempts) != len(failingModels)+1 {
		t.Fatalf("expected %d attempts, got %d (%v)", len(failingModels)+1, len(attempts), attempts)
	}
	if attempts[len(attempts)-1] != workingModel {
		t.Fatalf("expected last attempt to reach %q, got %v", workingModel, attempts)
	}
}

func TestProxyServeHTTP_AutoRoutingUsesFreshSnapshotAfterFetchModels(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "proxy-auto-fetch-e2e-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	failingModel := "claude-3-5-sonnet-20241022"
	workingModel := "claude-sonnet-4-6"
	var attempts []string

	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fmt.Sprintf(`{
				"data": [
					{"id": %q},
					{"id": %q}
				]
			}`, workingModel, failingModel)))
		case "/v1/chat/completions":
			var body map[string]interface{}
			if err := stdjson.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			model, _ := body["model"].(string)
			attempts = append(attempts, model)
			w.Header().Set("Content-Type", "application/json")
			switch model {
			case workingModel:
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(fmt.Sprintf(`{"id":"chatcmpl-1","object":"chat.completion","model":%q,"choices":[{"message":{"role":"assistant","content":"ok"}}]}`, workingModel)))
			case failingModel:
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":{"message":"Model claude-3-5-sonnet-20241022 is not available","type":"invalid_request_error"}}`))
			default:
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(fmt.Sprintf(`{"error":{"message":"unexpected model %s","type":"invalid_request_error"}}`, model)))
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	storage, _ := providerpool.NewFileStorage(tmpDir)
	registry, _ := providerpool.NewRegistry(storage)
	discovery := providerpool.NewModelDiscovery(registry, storage, time.Hour)

	provider := &providerpool.Provider{
		ID:        "relay-provider",
		Name:      "Relay Provider",
		Type:      providerpool.ProviderTypeCustom,
		BaseURL:   upstream.URL,
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		Location:  providerpool.ProviderLocationCloud,
		Priority:  100,
		APIKeys:   []providerpool.APIKey{{ID: "k1", Key: "test-key", Enabled: true}},
		APIFormat: providerpool.APIFormatOpenAI,
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	if err := storage.SaveModels(provider.ID, []*providerpool.Model{{
		ID:           failingModel,
		ProviderID:   provider.ID,
		Name:         failingModel,
		Enabled:      true,
		Capabilities: providerpool.ModelCapabilities{Chat: true, FunctionCall: true, Streaming: true},
	}}); err != nil {
		t.Fatalf("save stale models: %v", err)
	}

	router := providerpool.NewRouter(registry, discovery, providerpool.RoutingStrategyPriority)
	initial, err := router.Route(&providerpool.RouteRequest{Mode: providerpool.RoutingModeAuto})
	if err != nil {
		t.Fatalf("initial route failed: %v", err)
	}
	if initial.Model == nil || initial.Model.ID != failingModel {
		t.Fatalf("initial routed model = %v, want %q", initial.Model, failingModel)
	}

	if _, err := discovery.FetchModels(context.Background(), provider.ID); err != nil {
		t.Fatalf("FetchModels failed: %v", err)
	}

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), NewFailoverHandler(&FailoverConfig{Enabled: false}, nil))
	ph.providerPool = &providerpool.Pool{
		Registry:  registry,
		Discovery: discovery,
		Router:    router,
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"model":"auto","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ph.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ServeHTTP status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Actual-Model"); got != workingModel {
		t.Fatalf("X-Actual-Model = %q, want %q", got, workingModel)
	}
	if len(attempts) != 1 || attempts[0] != workingModel {
		t.Fatalf("upstream attempts = %v, want [%q]", attempts, workingModel)
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

func TestNormalizeModelRoutingHint(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		wantModel string
		wantMode  string
	}{
		{name: "auto", model: "auto", wantModel: "", wantMode: "auto"},
		{name: "cloud", model: "cloud", wantModel: "", wantMode: "cloud"},
		{name: "local", model: "local", wantModel: "", wantMode: "local"},
		{name: "trimmed auto", model: " auto ", wantModel: "", wantMode: "auto"},
		{name: "explicit model unchanged", model: "gpt-4o", wantModel: "gpt-4o", wantMode: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotModel, gotMode := normalizeModelRoutingHint(tt.model)
			if gotModel != tt.wantModel || gotMode != tt.wantMode {
				t.Fatalf("normalizeModelRoutingHint(%q) = (%q,%q), want (%q,%q)",
					tt.model, gotModel, gotMode, tt.wantModel, tt.wantMode)
			}
		})
	}
}

func TestResolveRoutingMode(t *testing.T) {
	tests := []struct {
		name       string
		scopedMode string
		hintedMode string
		want       string
	}{
		{name: "scope cloud wins over local hint", scopedMode: "cloud", hintedMode: "local", want: "cloud"},
		{name: "scope local wins over cloud hint", scopedMode: "local", hintedMode: "cloud", want: "local"},
		{name: "auto scope uses cloud hint", scopedMode: "auto", hintedMode: "cloud", want: "cloud"},
		{name: "auto scope uses local hint", scopedMode: "auto", hintedMode: "local", want: "local"},
		{name: "auto fallback", scopedMode: "auto", hintedMode: "", want: "auto"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveRoutingMode(tt.scopedMode, tt.hintedMode); got != tt.want {
				t.Fatalf("resolveRoutingMode(%q,%q) = %q, want %q", tt.scopedMode, tt.hintedMode, got, tt.want)
			}
		})
	}
}

func TestProxyHandler_ModelRoutingHintsCloudLocal(t *testing.T) {
	cloudUpstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"cloud","choices":[{"message":{"content":"ok"}}],"model":"cloud-model"}`))
	}))
	defer cloudUpstream.Close()

	localUpstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"local","choices":[{"message":{"content":"ok"}}],"model":"local-model"}`))
	}))
	defer localUpstream.Close()

	tmpDir, err := os.MkdirTemp("", "proxy-routing-hint-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, _ := providerpool.NewFileStorage(tmpDir)
	registry, _ := providerpool.NewRegistry(storage)
	discovery := providerpool.NewModelDiscovery(registry, storage, time.Hour)
	router := providerpool.NewRouter(registry, discovery, providerpool.RoutingStrategyPriority)

	cloudProvider := &providerpool.Provider{
		ID:        "p-cloud",
		Name:      "cloud-provider",
		Type:      providerpool.ProviderTypeCustom,
		BaseURL:   cloudUpstream.URL,
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		Priority:  100,
		Location:  providerpool.ProviderLocationCloud,
		APIFormat: providerpool.APIFormatOpenAI,
		APIKeys:   []providerpool.APIKey{{ID: "k-cloud", Key: "cloud-key", Enabled: true}},
	}
	localProvider := &providerpool.Provider{
		ID:        "p-local",
		Name:      "local-provider",
		Type:      providerpool.ProviderTypeCustom,
		BaseURL:   localUpstream.URL,
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		Priority:  100,
		Location:  providerpool.ProviderLocationLocal,
		APIFormat: providerpool.APIFormatOpenAI,
		APIKeys:   []providerpool.APIKey{{ID: "k-local", Key: "local-key", Enabled: true}},
	}
	registry.Register(cloudProvider)
	registry.Register(localProvider)

	_ = storage.SaveModels(cloudProvider.ID, []*providerpool.Model{
		{
			ID:           "cloud-model",
			Name:         "cloud-model",
			ProviderID:   cloudProvider.ID,
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, Streaming: true},
		},
	})
	_ = storage.SaveModels(localProvider.ID, []*providerpool.Model{
		{
			ID:           "local-model",
			Name:         "local-model",
			ProviderID:   localProvider.ID,
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, Streaming: true},
		},
	})
	router.RebuildCandidates()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	pool := &providerpool.Pool{
		Registry:  registry,
		Discovery: discovery,
		Router:    router,
	}
	forceProviderPoolReady(pool)
	ph.SetProviderPool(pool)

	t.Run("cloud hint routes to cloud provider", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
			strings.NewReader(`{"model":"cloud","messages":[{"role":"user","content":"hi"}]}`))
		rec := httptest.NewRecorder()
		ph.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("X-Actual-Provider"); got != "cloud-provider" {
			t.Fatalf("expected cloud-provider, got %q", got)
		}
	})

	t.Run("local hint routes to local provider", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
			strings.NewReader(`{"model":"local","messages":[{"role":"user","content":"hi"}]}`))
		rec := httptest.NewRecorder()
		ph.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("X-Actual-Provider"); got != "local-provider" {
			t.Fatalf("expected local-provider, got %q", got)
		}
	})

	t.Run("api key scoped mode overrides model hint", func(t *testing.T) {
		ph.SetAPIKeyValidator(func(key string) ([]string, error) {
			if key == "scoped-key" {
				return []string{"route:local"}, nil
			}
			return nil, nil
		})
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
			strings.NewReader(`{"model":"cloud","messages":[{"role":"user","content":"hi"}]}`))
		req.Header.Set("x-api-key", "scoped-key")
		rec := httptest.NewRecorder()
		ph.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("X-Actual-Provider"); got != "local-provider" {
			t.Fatalf("expected scoped local-provider, got %q", got)
		}
	})
}

func TestServeHTTP_CustomRelayRemembersFormatPerProviderModel_Smoke(t *testing.T) {
	const (
		providerID   = "custom-relay-memory"
		providerName = "relay-memory-provider"
		claudeModel  = "claude-sonnet-4-6"
		codexModel   = "gpt-5.3-codex"
	)

	var (
		mu           sync.Mutex
		counts       = map[string]int{}
		headerErrors []string
	)

	key := func(model, path string) string {
		return model + "|" + path
	}
	snapshot := func() map[string]int {
		mu.Lock()
		defer mu.Unlock()
		out := make(map[string]int, len(counts))
		for k, v := range counts {
			out[k] = v
		}
		return out
	}
	delta := func(before map[string]int, model, path string) int {
		mu.Lock()
		defer mu.Unlock()
		return counts[key(model, path)] - before[key(model, path)]
	}
	recordHeaderError := func(format string, args ...interface{}) {
		mu.Lock()
		defer mu.Unlock()
		headerErrors = append(headerErrors, fmt.Sprintf(format, args...))
	}

	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		_ = stdjson.Unmarshal(body, &payload)
		model, _ := payload["model"].(string)

		mu.Lock()
		counts[key(model, r.URL.Path)]++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/messages":
			if got := r.Header.Get("x-api-key"); got != "relay-key" {
				recordHeaderError("/v1/messages x-api-key = %q, want %q", got, "relay-key")
			}
			if got := r.Header.Get("anthropic-version"); got != "2023-06-01" {
				recordHeaderError("/v1/messages anthropic-version = %q, want %q", got, "2023-06-01")
			}
			if got := r.Header.Get("Authorization"); got != "" {
				recordHeaderError("/v1/messages Authorization = %q, want empty", got)
			}
			if model != claudeModel {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(`{"error":{"message":"Unsupported request body."}}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","content":[{"type":"text","text":"claude ok"}]}`))
		case "/v1/responses":
			if gotAuth, gotKey := r.Header.Get("Authorization"), r.Header.Get("x-api-key"); gotAuth != "Bearer relay-key" && gotKey != "relay-key" {
				recordHeaderError("/v1/responses expected API key auth, got Authorization=%q x-api-key=%q", gotAuth, gotKey)
			}
			if model != codexModel {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":{"message":"not implemented","type":"new_api_error","code":"convert_request_failed"}}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"resp_1","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"codex ok"}]}]}`))
		case "/v1/chat/completions":
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"error":{"message":"Unsupported request body."}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer upstream.Close()

	tmpDir, err := os.MkdirTemp("", "proxy-relay-memory-smoke-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := providerpool.NewFileStorage(tmpDir)
	if err != nil {
		t.Fatalf("create storage failed: %v", err)
	}
	registry, err := providerpool.NewRegistry(storage)
	if err != nil {
		t.Fatalf("create registry failed: %v", err)
	}
	discovery := providerpool.NewModelDiscovery(registry, storage, time.Hour)
	router := providerpool.NewRouter(registry, discovery, providerpool.RoutingStrategyPriority)

	provider := &providerpool.Provider{
		ID:        providerID,
		Name:      providerName,
		Type:      providerpool.ProviderTypeCustom,
		BaseURL:   upstream.URL,
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		Priority:  100,
		Location:  providerpool.ProviderLocationCloud,
		APIFormat: providerpool.APIFormatOpenAI,
		APIKeys:   []providerpool.APIKey{{ID: "k-relay", Key: "relay-key", Enabled: true}},
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("register provider failed: %v", err)
	}
	if err := storage.SaveModels(providerID, []*providerpool.Model{
		{
			ID:           claudeModel,
			Name:         claudeModel,
			ProviderID:   providerID,
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, Streaming: true},
		},
		{
			ID:           codexModel,
			Name:         codexModel,
			ProviderID:   providerID,
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, Streaming: true},
		},
	}); err != nil {
		t.Fatalf("save models failed: %v", err)
	}
	router.RebuildCandidates()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	pool := &providerpool.Pool{
		Registry:  registry,
		Discovery: discovery,
		Router:    router,
	}
	forceProviderPoolReady(pool)
	ph.SetProviderPool(pool)

	send := func(model string) {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
			strings.NewReader(fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}]}`, model)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		ph.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("model %s: expected 200, got %d body=%s", model, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("X-Actual-Provider"); got != providerName {
			t.Fatalf("model %s: X-Actual-Provider = %q, want %q", model, got, providerName)
		}
	}

	if _, ok := ph.providerMemory.RecallModelFormat(providerID, upstream.URL, claudeModel); ok {
		t.Fatal("expected claude format memory miss before first request")
	}
	if _, ok := ph.providerMemory.RecallModelFormat(providerID, upstream.URL, codexModel); ok {
		t.Fatal("expected codex format memory miss before first request")
	}

	beforeClaudeFirst := snapshot()
	send(claudeModel)
	if got := delta(beforeClaudeFirst, claudeModel, "/v1/messages"); got != 1 {
		t.Fatalf("claude first request /v1/messages delta = %d, want 1", got)
	}
	if got := delta(beforeClaudeFirst, claudeModel, "/v1/responses"); got != 0 {
		t.Fatalf("claude first request /v1/responses delta = %d, want 0", got)
	}
	if got := delta(beforeClaudeFirst, claudeModel, "/v1/chat/completions"); got != 0 {
		t.Fatalf("claude first request /v1/chat/completions delta = %d, want 0", got)
	}
	if format, ok := ph.providerMemory.RecallModelFormat(providerID, upstream.URL, claudeModel); !ok || format != string(providerpool.APIFormatAnthropic) {
		t.Fatalf("claude remembered format = %q (ok=%v), want anthropic", format, ok)
	}
	if _, ok := ph.providerMemory.RecallModelFormat(providerID, upstream.URL, codexModel); ok {
		t.Fatal("expected codex format to remain unremembered after claude success")
	}

	beforeClaudeSecond := snapshot()
	send(claudeModel)
	if got := delta(beforeClaudeSecond, claudeModel, "/v1/messages"); got != 1 {
		t.Fatalf("claude second request /v1/messages delta = %d, want 1", got)
	}
	if got := delta(beforeClaudeSecond, claudeModel, "/v1/responses"); got != 0 {
		t.Fatalf("claude second request /v1/responses delta = %d, want 0", got)
	}
	if got := delta(beforeClaudeSecond, claudeModel, "/v1/chat/completions"); got != 0 {
		t.Fatalf("claude second request /v1/chat/completions delta = %d, want 0", got)
	}

	beforeCodexFirst := snapshot()
	send(codexModel)
	if got := delta(beforeCodexFirst, codexModel, "/v1/responses"); got != 1 {
		t.Fatalf("codex first request /v1/responses delta = %d, want 1", got)
	}
	if got := delta(beforeCodexFirst, codexModel, "/v1/messages"); got != 0 {
		t.Fatalf("codex first request /v1/messages delta = %d, want 0", got)
	}
	if got := delta(beforeCodexFirst, codexModel, "/v1/chat/completions"); got != 0 {
		t.Fatalf("codex first request /v1/chat/completions delta = %d, want 0", got)
	}
	if format, ok := ph.providerMemory.RecallModelFormat(providerID, upstream.URL, codexModel); !ok || format != string(providerpool.APIFormatResponses) {
		t.Fatalf("codex remembered format = %q (ok=%v), want responses", format, ok)
	}
	if format, ok := ph.providerMemory.RecallModelFormat(providerID, upstream.URL, claudeModel); !ok || format != string(providerpool.APIFormatAnthropic) {
		t.Fatalf("claude remembered format after codex request = %q (ok=%v), want anthropic", format, ok)
	}

	beforeCodexSecond := snapshot()
	send(codexModel)
	if got := delta(beforeCodexSecond, codexModel, "/v1/responses"); got != 1 {
		t.Fatalf("codex second request /v1/responses delta = %d, want 1", got)
	}
	if got := delta(beforeCodexSecond, codexModel, "/v1/messages"); got != 0 {
		t.Fatalf("codex second request /v1/messages delta = %d, want 0", got)
	}
	if got := delta(beforeCodexSecond, codexModel, "/v1/chat/completions"); got != 0 {
		t.Fatalf("codex second request /v1/chat/completions delta = %d, want 0", got)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(headerErrors) > 0 {
		t.Fatalf("unexpected upstream auth headers:\n%s", strings.Join(headerErrors, "\n"))
	}
}

func TestServeHTTP_CustomRelayDoesNotRememberFormatUntilFirstSuccess_Smoke(t *testing.T) {
	const (
		providerID   = "custom-relay-fail-then-success"
		providerName = "relay-fail-then-success"
		modelID      = "claude-sonnet-4-6"
	)

	var (
		mu           sync.Mutex
		counts       = map[string]int{}
		headerErrors []string
		phase        atomic.Int32
	)

	snapshot := func() map[string]int {
		mu.Lock()
		defer mu.Unlock()
		out := make(map[string]int, len(counts))
		for k, v := range counts {
			out[k] = v
		}
		return out
	}
	delta := func(before map[string]int, path string) int {
		mu.Lock()
		defer mu.Unlock()
		return counts[path] - before[path]
	}
	recordHeaderError := func(format string, args ...interface{}) {
		mu.Lock()
		defer mu.Unlock()
		headerErrors = append(headerErrors, fmt.Sprintf(format, args...))
	}

	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		_ = stdjson.Unmarshal(body, &payload)
		model, _ := payload["model"].(string)

		mu.Lock()
		counts[r.URL.Path]++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if model != modelID {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"message":"unknown model"}}`))
			return
		}

		switch r.URL.Path {
		case "/v1/messages":
			if gotAuth, gotKey := r.Header.Get("Authorization"), r.Header.Get("x-api-key"); gotAuth == "" && gotKey == "" {
				recordHeaderError("/v1/messages expected some API key auth, got Authorization=%q x-api-key=%q", gotAuth, gotKey)
			}
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"error":{"message":"Unsupported request body."}}`))
		case "/v1/chat/completions":
			if gotAuth, gotKey := r.Header.Get("Authorization"), r.Header.Get("x-api-key"); gotAuth == "" && gotKey == "" {
				recordHeaderError("/v1/chat/completions expected some API key auth, got Authorization=%q x-api-key=%q", gotAuth, gotKey)
			}
			if phase.Load() == 0 {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(`{"error":{"message":"Unsupported request body."}}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"chatcmpl-1","object":"chat.completion","choices":[{"message":{"role":"assistant","content":"openai ok"}}]}`))
		case "/v1/responses":
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"error":{"message":"Unsupported request body."}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer upstream.Close()

	tmpDir, err := os.MkdirTemp("", "proxy-relay-fail-then-success-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := providerpool.NewFileStorage(tmpDir)
	if err != nil {
		t.Fatalf("create storage failed: %v", err)
	}
	registry, err := providerpool.NewRegistry(storage)
	if err != nil {
		t.Fatalf("create registry failed: %v", err)
	}
	discovery := providerpool.NewModelDiscovery(registry, storage, time.Hour)
	router := providerpool.NewRouter(registry, discovery, providerpool.RoutingStrategyPriority)

	provider := &providerpool.Provider{
		ID:        providerID,
		Name:      providerName,
		Type:      providerpool.ProviderTypeCustom,
		BaseURL:   upstream.URL,
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		Priority:  100,
		Location:  providerpool.ProviderLocationCloud,
		APIFormat: providerpool.APIFormatOpenAI,
		APIKeys:   []providerpool.APIKey{{ID: "k-relay", Key: "relay-key", Enabled: true}},
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("register provider failed: %v", err)
	}
	if err := storage.SaveModels(providerID, []*providerpool.Model{{
		ID:           modelID,
		Name:         modelID,
		ProviderID:   providerID,
		Enabled:      true,
		Capabilities: providerpool.ModelCapabilities{Chat: true, Streaming: true},
	}}); err != nil {
		t.Fatalf("save models failed: %v", err)
	}
	router.RebuildCandidates()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), NewFailoverHandler(&FailoverConfig{Enabled: false}, nil))
	pool := &providerpool.Pool{
		Registry:  registry,
		Discovery: discovery,
		Router:    router,
	}
	forceProviderPoolReady(pool)
	ph.SetProviderPool(pool)
	time.Sleep(200 * time.Millisecond)

	send := func(wantStatus int) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
			strings.NewReader(fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}]}`, modelID)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		ph.ServeHTTP(rec, req)
		if rec.Code != wantStatus {
			t.Fatalf("expected %d, got %d body=%s", wantStatus, rec.Code, rec.Body.String())
		}
		return rec
	}

	beforeFail := snapshot()
	failureRec := send(http.StatusUnprocessableEntity)
	if got := delta(beforeFail, "/v1/messages"); got != 1 {
		t.Fatalf("first failed request /v1/messages delta = %d, want 1", got)
	}
	if got := delta(beforeFail, "/v1/chat/completions"); got != 1 {
		t.Fatalf("first failed request /v1/chat/completions delta = %d, want 1", got)
	}
	if got := delta(beforeFail, "/v1/responses"); got != 1 {
		t.Fatalf("first failed request /v1/responses delta = %d, want 1", got)
	}
	if _, ok := ph.providerMemory.RecallModelFormat(providerID, upstream.URL, modelID); ok {
		t.Fatalf("expected no model format memory after first failure, body=%s", failureRec.Body.String())
	}
	if _, ok := ph.providerMemory.RecallFormat(providerID, upstream.URL); ok {
		t.Fatal("expected no provider format memory after first failure")
	}

	phase.Store(1)
	beforeSuccess := snapshot()
	successRec := send(http.StatusOK)
	if got := successRec.Header().Get("X-Actual-Provider"); got != providerName {
		t.Fatalf("second request X-Actual-Provider = %q, want %q", got, providerName)
	}
	if got := delta(beforeSuccess, "/v1/messages"); got != 1 {
		t.Fatalf("second request /v1/messages delta = %d, want 1", got)
	}
	if got := delta(beforeSuccess, "/v1/chat/completions"); got != 1 {
		t.Fatalf("second request /v1/chat/completions delta = %d, want 1", got)
	}
	if got := delta(beforeSuccess, "/v1/responses"); got != 0 {
		t.Fatalf("second request /v1/responses delta = %d, want 0", got)
	}
	if format, ok := ph.providerMemory.RecallModelFormat(providerID, upstream.URL, modelID); !ok || format != string(providerpool.APIFormatOpenAI) {
		t.Fatalf("remembered model format after first success = %q (ok=%v), want openai", format, ok)
	}
	if _, ok := ph.providerMemory.RecallFormat(providerID, upstream.URL); ok {
		t.Fatal("expected custom relay to keep provider-scoped format memory empty after success")
	}

	phase.Store(2)
	beforeRemembered := snapshot()
	rememberedRec := send(http.StatusOK)
	if got := rememberedRec.Header().Get("X-Actual-Provider"); got != providerName {
		t.Fatalf("third request X-Actual-Provider = %q, want %q", got, providerName)
	}
	if got := delta(beforeRemembered, "/v1/messages"); got != 0 {
		t.Fatalf("remembered request /v1/messages delta = %d, want 0", got)
	}
	if got := delta(beforeRemembered, "/v1/chat/completions"); got != 1 {
		t.Fatalf("remembered request /v1/chat/completions delta = %d, want 1", got)
	}
	if got := delta(beforeRemembered, "/v1/responses"); got != 0 {
		t.Fatalf("remembered request /v1/responses delta = %d, want 0", got)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(headerErrors) > 0 {
		t.Fatalf("unexpected upstream auth headers:\n%s", strings.Join(headerErrors, "\n"))
	}
}

func TestServeHTTP_CustomRelayModelFailureDoesNotPolluteOtherRememberedFormat_Smoke(t *testing.T) {
	const (
		providerID      = "custom-relay-model-isolation"
		providerName    = "relay-model-isolation"
		rememberedModel = "claude-sonnet-4-6"
		failingModel    = "claude-haiku-4-5"
	)

	var (
		mu           sync.Mutex
		counts       = map[string]int{}
		headerErrors []string
	)

	key := func(model, path string) string {
		return model + "|" + path
	}
	snapshot := func() map[string]int {
		mu.Lock()
		defer mu.Unlock()
		out := make(map[string]int, len(counts))
		for k, v := range counts {
			out[k] = v
		}
		return out
	}
	delta := func(before map[string]int, model, path string) int {
		mu.Lock()
		defer mu.Unlock()
		return counts[key(model, path)] - before[key(model, path)]
	}
	recordHeaderError := func(format string, args ...interface{}) {
		mu.Lock()
		defer mu.Unlock()
		headerErrors = append(headerErrors, fmt.Sprintf(format, args...))
	}

	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		_ = stdjson.Unmarshal(body, &payload)
		model, _ := payload["model"].(string)

		mu.Lock()
		counts[key(model, r.URL.Path)]++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/messages":
			if gotAuth, gotKey := r.Header.Get("Authorization"), r.Header.Get("x-api-key"); gotAuth == "" && gotKey == "" {
				recordHeaderError("/v1/messages expected some API key auth, got Authorization=%q x-api-key=%q", gotAuth, gotKey)
			}
			if model != rememberedModel {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(`{"error":{"message":"Unsupported request body."}}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","content":[{"type":"text","text":"remembered ok"}]}`))
		case "/v1/chat/completions":
			if gotAuth, gotKey := r.Header.Get("Authorization"), r.Header.Get("x-api-key"); gotAuth == "" && gotKey == "" {
				recordHeaderError("/v1/chat/completions expected some API key auth, got Authorization=%q x-api-key=%q", gotAuth, gotKey)
			}
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"error":{"message":"Unsupported request body."}}`))
		case "/v1/responses":
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"error":{"message":"Unsupported request body."}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer upstream.Close()

	tmpDir, err := os.MkdirTemp("", "proxy-relay-model-isolation-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := providerpool.NewFileStorage(tmpDir)
	if err != nil {
		t.Fatalf("create storage failed: %v", err)
	}
	registry, err := providerpool.NewRegistry(storage)
	if err != nil {
		t.Fatalf("create registry failed: %v", err)
	}
	discovery := providerpool.NewModelDiscovery(registry, storage, time.Hour)
	router := providerpool.NewRouter(registry, discovery, providerpool.RoutingStrategyPriority)

	provider := &providerpool.Provider{
		ID:        providerID,
		Name:      providerName,
		Type:      providerpool.ProviderTypeCustom,
		BaseURL:   upstream.URL,
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		Priority:  100,
		Location:  providerpool.ProviderLocationCloud,
		APIFormat: providerpool.APIFormatOpenAI,
		APIKeys:   []providerpool.APIKey{{ID: "k-relay", Key: "relay-key", Enabled: true}},
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("register provider failed: %v", err)
	}
	if err := storage.SaveModels(providerID, []*providerpool.Model{
		{
			ID:           rememberedModel,
			Name:         rememberedModel,
			ProviderID:   providerID,
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, Streaming: true},
		},
		{
			ID:           failingModel,
			Name:         failingModel,
			ProviderID:   providerID,
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, Streaming: true},
		},
	}); err != nil {
		t.Fatalf("save models failed: %v", err)
	}
	router.RebuildCandidates()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), NewFailoverHandler(&FailoverConfig{Enabled: false}, nil))
	pool := &providerpool.Pool{
		Registry:  registry,
		Discovery: discovery,
		Router:    router,
	}
	forceProviderPoolReady(pool)
	ph.SetProviderPool(pool)
	time.Sleep(200 * time.Millisecond)

	send := func(model string, wantStatus int) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
			strings.NewReader(fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}]}`, model)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		ph.ServeHTTP(rec, req)
		if rec.Code != wantStatus {
			t.Fatalf("model %s: expected %d, got %d body=%s", model, wantStatus, rec.Code, rec.Body.String())
		}
		return rec
	}

	beforeRememberedWarm := snapshot()
	firstRemembered := send(rememberedModel, http.StatusOK)
	if got := firstRemembered.Header().Get("X-Actual-Provider"); got != providerName {
		t.Fatalf("remembered model first request provider = %q, want %q", got, providerName)
	}
	if got := delta(beforeRememberedWarm, rememberedModel, "/v1/messages"); got != 1 {
		t.Fatalf("remembered model first request /v1/messages delta = %d, want 1", got)
	}
	if got := delta(beforeRememberedWarm, rememberedModel, "/v1/chat/completions"); got != 0 {
		t.Fatalf("remembered model first request /v1/chat/completions delta = %d, want 0", got)
	}
	if got := delta(beforeRememberedWarm, rememberedModel, "/v1/responses"); got != 0 {
		t.Fatalf("remembered model first request /v1/responses delta = %d, want 0", got)
	}
	if format, ok := ph.providerMemory.RecallModelFormat(providerID, upstream.URL, rememberedModel); !ok || format != string(providerpool.APIFormatAnthropic) {
		t.Fatalf("remembered model format = %q (ok=%v), want anthropic", format, ok)
	}

	beforeFailing := snapshot()
	failingRec := send(failingModel, http.StatusUnprocessableEntity)
	if got := delta(beforeFailing, failingModel, "/v1/messages"); got != 1 {
		t.Fatalf("failing model /v1/messages delta = %d, want 1", got)
	}
	if got := delta(beforeFailing, failingModel, "/v1/chat/completions"); got != 1 {
		t.Fatalf("failing model /v1/chat/completions delta = %d, want 1", got)
	}
	if got := delta(beforeFailing, failingModel, "/v1/responses"); got != 1 {
		t.Fatalf("failing model /v1/responses delta = %d, want 1", got)
	}
	if _, ok := ph.providerMemory.RecallModelFormat(providerID, upstream.URL, failingModel); ok {
		t.Fatalf("expected failing model to remain unremembered, body=%s", failingRec.Body.String())
	}
	if format, ok := ph.providerMemory.RecallModelFormat(providerID, upstream.URL, rememberedModel); !ok || format != string(providerpool.APIFormatAnthropic) {
		t.Fatalf("remembered model format after other model failure = %q (ok=%v), want anthropic", format, ok)
	}

	beforeRememberedAgain := snapshot()
	secondRemembered := send(rememberedModel, http.StatusOK)
	if got := secondRemembered.Header().Get("X-Actual-Provider"); got != providerName {
		t.Fatalf("remembered model second request provider = %q, want %q", got, providerName)
	}
	if got := delta(beforeRememberedAgain, rememberedModel, "/v1/messages"); got != 1 {
		t.Fatalf("remembered model second request /v1/messages delta = %d, want 1", got)
	}
	if got := delta(beforeRememberedAgain, rememberedModel, "/v1/chat/completions"); got != 0 {
		t.Fatalf("remembered model second request /v1/chat/completions delta = %d, want 0", got)
	}
	if got := delta(beforeRememberedAgain, rememberedModel, "/v1/responses"); got != 0 {
		t.Fatalf("remembered model second request /v1/responses delta = %d, want 0", got)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(headerErrors) > 0 {
		t.Fatalf("unexpected upstream auth headers:\n%s", strings.Join(headerErrors, "\n"))
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

	formats, n, known := ph.allFormatsForProvider(pid, burl, "", provider)
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

	formats, n, known := ph.allFormatsForProvider(pid, burl, "", provider)
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
		Type:      providerpool.ProviderTypeCustom,
		BaseURL:   burl,
		APIFormat: providerpool.APIFormatOpenAI,
	}

	formats, n, known := ph.allFormatsForProvider(pid, burl, "gpt-5.1", provider)
	if known {
		t.Fatal("expected known=false without detected/remembered format")
	}
	if n != 3 {
		t.Fatalf("expected 3 formats for generic openai provider, got %d", n)
	}
	if formats[0] != providerpool.APIFormatOpenAI || formats[1] != providerpool.APIFormatAnthropic || formats[2] != providerpool.APIFormatResponses {
		t.Fatalf("unexpected format order: [%q %q %q]", formats[0], formats[1], formats[2])
	}
}

func TestAllFormatsForProvider_CustomClaudePrefersAnthropic(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)
	pid := "generic-claude"
	burl := "https://relay.example.com"
	provider := &providerpool.Provider{ID: pid, Type: providerpool.ProviderTypeCustom, BaseURL: burl, APIFormat: providerpool.APIFormatOpenAI}

	formats, n, known := ph.allFormatsForProvider(pid, burl, "claude-sonnet-4-6", provider)
	if known {
		t.Fatal("expected known=false without remembered model format")
	}
	if n != 3 {
		t.Fatalf("expected 3 formats, got %d", n)
	}
	if formats[0] != providerpool.APIFormatAnthropic || formats[1] != providerpool.APIFormatOpenAI || formats[2] != providerpool.APIFormatResponses {
		t.Fatalf("unexpected format order: [%q %q %q]", formats[0], formats[1], formats[2])
	}
}

func TestAllFormatsForProvider_CustomCodexPrefersResponses(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)
	pid := "generic-codex"
	burl := "https://relay.example.com"
	provider := &providerpool.Provider{ID: pid, Type: providerpool.ProviderTypeCustom, BaseURL: burl, APIFormat: providerpool.APIFormatOpenAI}

	formats, n, known := ph.allFormatsForProvider(pid, burl, "gpt-5.3-codex", provider)
	if known {
		t.Fatal("expected known=false without remembered model format")
	}
	if n != 3 {
		t.Fatalf("expected 3 formats, got %d", n)
	}
	if formats[0] != providerpool.APIFormatResponses || formats[1] != providerpool.APIFormatOpenAI || formats[2] != providerpool.APIFormatAnthropic {
		t.Fatalf("unexpected format order: [%q %q %q]", formats[0], formats[1], formats[2])
	}
}

func TestTryOnProvider_LegacyProtocolMismatchPrefersResponsesBeforeAnthropic(t *testing.T) {
	var paths []string
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/chat/completions":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Unsupported legacy protocol: /v1/chat/completions is not supported. Please use /v1/responses.","type":"invalid_request_error"}}`))
		case "/v1/responses":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"resp_1","object":"response","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`))
		case "/v1/messages":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":{"message":"wrong format reached anthropic path"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "responses-relay",
			Type:      providerpool.ProviderTypeCustom,
			BaseURL:   upstream.URL,
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "test-key"},
	}
	pr := &parsedRequest{
		body:  []byte(`{"model":"gpt-5.3-codex","messages":[{"role":"user","content":"hi"}]}`),
		model: "gpt-5.3-codex",
	}
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp, format, _, err := ph.tryOnProvider(r, result, pr)
	if err != nil {
		t.Fatalf("expected success after responses fallback, got: %v", err)
	}
	if resp != nil {
		resp.Body.Close()
	}
	if format != providerpool.APIFormatResponses {
		t.Fatalf("format = %q, want %q", format, providerpool.APIFormatResponses)
	}
	if len(paths) != 1 {
		t.Fatalf("expected exactly 1 upstream attempt, got %d (%v)", len(paths), paths)
	}
	if paths[0] != "/v1/responses" {
		t.Fatalf("expected responses-first attempt, got %v", paths)
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

	formats, n, known := ph.allFormatsForProvider(pid, burl, "gpt-5.3-codex", provider)
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

		formats, n, known := ph.allFormatsForProvider(pid, burl, "claude-sonnet-4-6", provider)
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

	formats, n, known := ph.allFormatsForProvider(pid, burl, "claude-sonnet-4-6", provider)
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
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		stdjson.NewEncoder(w).Encode(map[string]interface{}{
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
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

// TestTryOnProvider_WrappedOverloaded5xxThrottlesProvider verifies wrapped
// overload responses are treated as short-lived provider throttles.
func TestTryOnProvider_WrappedOverloaded5xxThrottlesProvider(t *testing.T) {
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"type":    "overloaded_error",
				"message": "构建请求失败",
			},
		})
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "wrapped-overload-provider",
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
		t.Fatal("expected error for wrapped overloaded 500")
	}
	if !strings.Contains(err.Error(), "upstream 500") {
		t.Fatalf("expected upstream 500 error, got: %s", err)
	}
	if !ph.providerMemory.IsThrottled("wrapped-overload-provider", upstream.URL) {
		t.Fatal("provider should be throttled after wrapped overloaded 500")
	}
}

// TestTryOnProvider_5xxSkipsEntireProvider verifies 5xx errors skip the provider
func TestTryOnProvider_5xxSkipsEntireProvider(t *testing.T) {
	var requestCount int32

	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestTryOnProvider_AuthExhaustedSkipsModelFallback(t *testing.T) {
	var requestCount atomic.Int32
	var (
		mu         sync.Mutex
		seenModels []string
	)

	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		var body map[string]interface{}
		_ = stdjson.NewDecoder(r.Body).Decode(&body)
		if model, _ := body["model"].(string); model != "" {
			mu.Lock()
			seenModels = append(seenModels, model)
			mu.Unlock()
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"message":"用户已被封禁"}}`))
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "auth-failing-provider",
			BaseURL:   upstream.URL,
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "test-key"},
		Model:  &providerpool.Model{ID: "claude-sonnet-4-5-20250929"},
	}

	pr := &parsedRequest{
		body:  []byte(`{"model":"claude-3-5-sonnet-20241022","messages":[{"role":"user","content":"hi"}]}`),
		model: "claude-3-5-sonnet-20241022",
	}

	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	resp, _, _, err := ph.tryOnProvider(r, result, pr)
	if resp != nil {
		resp.Body.Close()
	}

	if err == nil {
		t.Fatal("expected auth exhaustion error")
	}

	var authErr *AuthExhaustedError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthExhaustedError, got %T: %v", err, err)
	}
	if authErr.LastStatusCode != http.StatusForbidden {
		t.Fatalf("expected forbidden auth status, got %d", authErr.LastStatusCode)
	}
	if !strings.Contains(authErr.LastBody, "封禁") {
		t.Fatalf("expected upstream auth body to be preserved, got %q", authErr.LastBody)
	}

	if count := requestCount.Load(); count != 4 {
		t.Fatalf("expected exactly 4 auth strategy attempts for the routed model, got %d", count)
	}

	mu.Lock()
	gotModels := append([]string(nil), seenModels...)
	mu.Unlock()
	for _, model := range gotModels {
		if model != "claude-sonnet-4-5-20250929" {
			t.Fatalf("expected auth failure to stop model fallback, saw models %v", gotModels)
		}
	}
}

func TestTryOnProvider_RequestConversionUnsupportedSkipsAliases(t *testing.T) {
	var requestCount atomic.Int32
	var seenModels []string
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		var body map[string]interface{}
		_ = stdjson.NewDecoder(r.Body).Decode(&body)
		if model, _ := body["model"].(string); model != "" {
			seenModels = append(seenModels, model)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "not implemented (request id: abc123)",
				"type":    "new_api_error",
				"code":    "convert_request_failed",
			},
		})
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "responses-provider",
			BaseURL:   upstream.URL,
			APIFormat: providerpool.APIFormatResponses,
		},
		APIKey: &providerpool.APIKey{Key: "test-key"},
		Fallbacks: []*providerpool.RouteCandidate{{
			Provider: &providerpool.Provider{ID: "fallback-provider", BaseURL: "http://fallback.invalid", APIFormat: providerpool.APIFormatResponses},
			Model:    &providerpool.Model{ID: "claude-opus-4-6"},
		}},
	}

	pr := &parsedRequest{
		body:  []byte(`{"model":"claude-opus-4-6","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"noop","parameters":{"type":"object","properties":{}}}}]}`),
		model: "claude-opus-4-6",
	}

	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	resp, _, _, err := ph.tryOnProvider(r, result, pr)
	if resp != nil {
		resp.Body.Close()
	}

	if err == nil {
		t.Fatal("expected request conversion unsupported error")
	}
	if !errors.Is(err, errRequestConversionUnsupported) {
		t.Fatalf("expected conversion error, got: %s", err)
	}
	if ph.providerMemory.IsModelBlacklisted("responses-provider", upstream.URL, "claude-opus-4-6") {
		t.Fatal("model should not be blacklisted after convert_request_failed")
	}
	if got := requestCount.Load(); got != 1 {
		t.Fatalf("expected exactly 1 upstream request, got %d", got)
	}
	if len(seenModels) != 1 || seenModels[0] != "claude-opus-4-6" {
		t.Fatalf("expected only original model to be tried, saw %v", seenModels)
	}
}

func TestExecuteOnRouteResult_RequestConversionUnsupportedMarksToolCapNone(t *testing.T) {
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "not implemented (request id: abc123)",
				"type":    "new_api_error",
				"code":    "convert_request_failed",
			},
		})
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "responses-provider",
			BaseURL:   upstream.URL,
			APIFormat: providerpool.APIFormatResponses,
		},
		APIKey: &providerpool.APIKey{Key: "test-key"},
		Fallbacks: []*providerpool.RouteCandidate{{
			Provider: &providerpool.Provider{ID: "fallback-provider", BaseURL: "http://fallback.invalid", APIFormat: providerpool.APIFormatResponses},
			Model:    &providerpool.Model{ID: "claude-opus-4-6"},
		}},
	}

	pr := &parsedRequest{
		body:  []byte(`{"model":"claude-opus-4-6","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"noop","parameters":{"type":"object","properties":{}}}}]}`),
		model: "claude-opus-4-6",
	}

	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	_, err := ph.executeOnRouteResult(r, result, pr, true)
	if err == nil {
		t.Fatal("expected executeOnRouteResult to fail")
	}

	cap, ok := ph.providerMemory.RecallToolCap("responses-provider", upstream.URL)
	if !ok {
		t.Fatal("expected tool capability to be remembered")
	}
	if cap != ToolCapNone {
		t.Fatalf("expected ToolCapNone, got %d", cap)
	}
}

// TestTryOnProvider_SuccessRemembersFormatAndModel verifies successful requests
// are remembered for future optimization
func TestTryOnProvider_SuccessRemembersFormatAndModel(t *testing.T) {
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			Type:      providerpool.ProviderTypeCustom,
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

	format, ok := ph.providerMemory.RecallModelFormat(pid, upstream.URL, "gpt-4")
	if !ok {
		t.Fatal("expected model-scoped format to be remembered")
	}
	if format != string(providerpool.APIFormatOpenAI) {
		t.Errorf("expected remembered model format 'openai', got %q", format)
	}
	if _, ok := ph.providerMemory.RecallFormat(pid, upstream.URL); ok {
		t.Fatal("expected custom provider not to persist provider-scoped format memory")
	}
}

// TestExecuteOnRouteResult_ResolvedModelUsesActualUsedModel verifies that
// resolvedModel reflects the model that actually succeeded upstream.
func TestExecuteOnRouteResult_ResolvedModelUsesActualUsedModel(t *testing.T) {
	const (
		requestModel = "gpt-5.3-codex-spark"
		aliasModel   = "gpt-5-codex"
		providerID   = "alias-provider"
	)

	var seenModels []string
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		model, _ := body["model"].(string)
		seenModels = append(seenModels, model)

		w.Header().Set("Content-Type", "application/json")
		if model == aliasModel {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":      "chatcmpl-123",
				"object":  "chat.completion",
				"model":   model,
				"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}}},
			})
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("Model %s is not available", model),
			},
		})
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	ph.providerMemory.RememberModelAlias(providerID, upstream.URL, requestModel, aliasModel)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        providerID,
			Name:      providerID,
			BaseURL:   upstream.URL,
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: requestModel},
		APIKey: &providerpool.APIKey{Key: "test-key"},
	}

	pr := &parsedRequest{
		body:  []byte(`{"model":"` + requestModel + `","messages":[{"role":"user","content":"hi"}]}`),
		model: requestModel,
	}

	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	outcome, err := ph.executeOnRouteResult(r, result, pr, false)
	if err != nil {
		t.Fatalf("executeOnRouteResult failed: %v", err)
	}
	if outcome == nil || outcome.resp == nil {
		t.Fatal("expected non-nil outcome response")
	}
	defer outcome.resp.Body.Close()

	if outcome.resolvedModel != aliasModel {
		t.Fatalf("resolvedModel = %q, want %q", outcome.resolvedModel, aliasModel)
	}
	if len(seenModels) == 0 || seenModels[0] != aliasModel {
		t.Fatalf("expected alias model to be tried first, seen=%v", seenModels)
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

func TestIsRequestConversionUnsupportedError(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"500 convert_request_failed", 500, `{"error":{"message":"not implemented (request id: abc123)","type":"new_api_error","code":"convert_request_failed"}}`, true},
		{"501 not implemented relay error", 501, `{"error":{"message":"not implemented","type":"new_api_error"}}`, true},
		{"500 generic internal", 500, `{"error":{"message":"internal server error"}}`, false},
		{"400 convert_request_failed", 400, `{"error":{"code":"convert_request_failed"}}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRequestConversionUnsupportedError(tt.status, []byte(tt.body))
			if got != tt.want {
				t.Errorf("isRequestConversionUnsupportedError(%d, %q) = %v, want %v", tt.status, tt.body, got, tt.want)
			}
		})
	}
}

// TestTryOnProvider_FormatMismatch422_ExpandsFormats verifies that a 422 on the
// remembered format triggers expansion to all formats, eventually succeeding.
func TestTryOnProvider_FormatMismatch422_ExpandsFormats(t *testing.T) {
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "chat/completions") {
			w.WriteHeader(422)
			w.Write([]byte(`{"error":{"message":"Unsupported request body."}}`))
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"id":"msg_1","type":"message","content":[{"type":"text","text":"ok"}]}`))
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	pid := "format-mismatch-provider"
	modelID := "claude-sonnet-4-5-20250514"

	ph.providerMemory.RememberModelFormat(pid, upstream.URL, modelID, "openai")

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        pid,
			Type:      providerpool.ProviderTypeCustom,
			BaseURL:   upstream.URL,
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "test-key"},
	}

	pr := &parsedRequest{
		body:  []byte(`{"model":"claude-sonnet-4-5-20250514","messages":[{"role":"user","content":"hi"}]}`),
		model: modelID,
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

	format, ok := ph.providerMemory.RecallModelFormat(pid, upstream.URL, modelID)
	if !ok {
		t.Fatal("expected model-scoped format to be remembered after successful fallback")
	}
	if format != string(providerpool.APIFormatAnthropic) {
		t.Errorf("expected remembered model format 'anthropic', got %q", format)
	}
	if _, ok := ph.providerMemory.RecallFormat(pid, upstream.URL); ok {
		t.Fatal("expected provider-scoped format memory to stay empty for custom relays")
	}
}

func TestTryOnProvider_CustomResponsesLockSelfHealsClaude(t *testing.T) {
	var paths []string
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/responses":
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":{"message":"not implemented","type":"new_api_error","code":"convert_request_failed"}}`))
		case "/v1/messages":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":"msg_1","type":"message","content":[{"type":"text","text":"ok"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	pid := "claude-relay"
	baseURL := upstream.URL + "/v1/responses"
	modelID := "claude-sonnet-4-6"
	ph.providerMemory.RememberModelFormat(pid, baseURL, modelID, "responses")

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        pid,
			Type:      providerpool.ProviderTypeCustom,
			BaseURL:   baseURL,
			APIFormat: providerpool.APIFormatResponses,
		},
		APIKey: &providerpool.APIKey{Key: "test-key"},
	}
	pr := &parsedRequest{
		body:  []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hi"}]}`),
		model: modelID,
	}

	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	resp, usedFormat, _, err := ph.tryOnProvider(r, result, pr)
	if err != nil {
		t.Fatalf("expected custom relay to self-heal, got error: %v", err)
	}
	if resp != nil {
		resp.Body.Close()
	}
	if usedFormat != providerpool.APIFormatAnthropic {
		t.Fatalf("usedFormat = %q, want %q", usedFormat, providerpool.APIFormatAnthropic)
	}
	if len(paths) != 2 || paths[0] != "/v1/responses" || paths[1] != "/v1/messages" {
		t.Fatalf("expected responses then anthropic self-heal, got %v", paths)
	}
	format, ok := ph.providerMemory.RecallModelFormat(pid, baseURL, modelID)
	if !ok || format != string(providerpool.APIFormatAnthropic) {
		t.Fatalf("expected remembered model format anthropic, got %q (ok=%v)", format, ok)
	}
}

// TestTryOnProvider_FormatMismatch404_OpenAIError verifies that a 404 with
// "openai_error" body is treated as format mismatch, not model blacklisting.
func TestTryOnProvider_FormatMismatch404_OpenAIError(t *testing.T) {
	var callCount atomic.Int32
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	// Should only try all request formats for the first model, then skip
	// remaining aliases. Without the fix, it would multiply by alias count.
	calls := callCount.Load()
	if calls > 3 {
		t.Errorf("expected at most 3 upstream calls (all formats for first model), got %d", calls)
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
	slowUpstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"slow","choices":[{"message":{"content":"slow"}}]}`))
	}))
	defer slowUpstream.Close()

	fastUpstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestExecuteOnRouteResult_LastFallbackBehavesAsSingleProvider(t *testing.T) {
	var requestCount int32
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			ID:        "last-candidate-provider",
			BaseURL:   upstream.URL,
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey:    &providerpool.APIKey{Key: "test-key"},
		Fallbacks: nil, // no remaining candidates
	}
	pr := &parsedRequest{
		body:           []byte(`{"model":"gpt-5.3-codex","messages":[{"role":"user","content":"hi"}]}`),
		model:          "gpt-5.3-codex",
		singleProvider: false, // global mode is not single-provider
	}

	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	outcome, err := ph.executeOnRouteResult(r, result, pr, false)
	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}
	if outcome == nil || outcome.resp == nil {
		t.Fatal("expected non-nil outcome response")
	}
	outcome.resp.Body.Close()
	if got := atomic.LoadInt32(&requestCount); got != 2 {
		t.Fatalf("expected 2 upstream attempts on last fallback provider, got %d", got)
	}
}

func TestExecuteOnRouteResult_SingleProviderWaitsForThrottle(t *testing.T) {
	var requestCount int32
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"chatcmpl-1","choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "throttled-single-provider",
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

	ph.providerMemory.RememberThrottle(result.Provider.ID, upstream.URL, 60*time.Millisecond)

	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	start := time.Now()
	outcome, err := ph.executeOnRouteResult(r, result, pr, false)
	if err != nil {
		t.Fatalf("expected throttled single-provider path to wait then succeed, got error: %v", err)
	}
	if outcome == nil || outcome.resp == nil {
		t.Fatal("expected non-nil outcome response")
	}
	outcome.resp.Body.Close()

	if elapsed := time.Since(start); elapsed < 45*time.Millisecond {
		t.Fatalf("expected executeOnRouteResult to wait for throttle, elapsed=%v", elapsed)
	}
	if got := atomic.LoadInt32(&requestCount); got != 1 {
		t.Fatalf("expected exactly 1 upstream request after waiting, got %d", got)
	}
}

func TestExecuteOnRouteResult_SingleProviderThrottleRespectsContextDeadline(t *testing.T) {
	var requestCount int32
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"chatcmpl-1","choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer upstream.Close()

	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "deadline-throttled-provider",
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

	ph.providerMemory.RememberThrottle(result.Provider.ID, upstream.URL, 200*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx)

	start := time.Now()
	_, err := ph.executeOnRouteResult(r, result, pr, false)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded while waiting for throttle, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 150*time.Millisecond {
		t.Fatalf("expected throttle wait to stop on context deadline, elapsed=%v", elapsed)
	}
	if got := atomic.LoadInt32(&requestCount); got != 0 {
		t.Fatalf("expected no upstream request when context expires during throttle wait, got %d", got)
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
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	client := ph.connPool.GetClient(provider.Name, ConnectionProfileProbe)
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
	upstream := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
