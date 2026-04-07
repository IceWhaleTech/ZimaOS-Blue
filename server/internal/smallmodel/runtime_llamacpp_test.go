package smallmodel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeLlamaCppDirectBackend struct {
	ensureErr     error
	generateErr   error
	text          string
	ensureCalls   int
	generateCalls int
	lastReq       llamaCppDirectGenerateRequest
}

func (b *fakeLlamaCppDirectBackend) EnsureLoaded() error {
	b.ensureCalls++
	return b.ensureErr
}

func (b *fakeLlamaCppDirectBackend) Generate(_ context.Context, req llamaCppDirectGenerateRequest) (string, error) {
	b.generateCalls++
	b.lastReq = req
	if b.generateErr != nil {
		return "", b.generateErr
	}
	return b.text, nil
}

func TestLlamaCppRuntimeReadinessReasonRuntimeNil(t *testing.T) {
	var rt *LlamaCppRuntime
	if got := rt.ReadinessReason(); got != "runtime_nil" {
		t.Fatalf("ReadinessReason() = %q, want runtime_nil", got)
	}
	if got := rt.ReadinessDetail(); !strings.Contains(got, "runtime") {
		t.Fatalf("ReadinessDetail() = %q, want runtime-related detail", got)
	}
}

func TestLlamaCppRuntimeReadinessReasonManagerNil(t *testing.T) {
	rt := NewLlamaCppRuntime(nil)
	if got := rt.ReadinessReason(); got != "manager_nil" {
		t.Fatalf("ReadinessReason() = %q, want manager_nil", got)
	}
}

func TestLlamaCppRuntimeReadinessReasonModelFilesMissing(t *testing.T) {
	m := NewManager(t.TempDir())
	rt := NewLlamaCppRuntime(m)
	if got := rt.ReadinessReason(); got != "model_files_missing_or_incomplete" {
		t.Fatalf("ReadinessReason() = %q, want model_files_missing_or_incomplete", got)
	}
	if rt.Ready() {
		t.Fatal("Ready() = true, want false")
	}
}

func TestLlamaCppRuntimeReadinessReasonCGOModeUnavailable(t *testing.T) {
	m := NewManager(t.TempDir())
	createReadyModelFiles(t, m)
	rt := NewLlamaCppRuntime(m, LlamaCppRuntimeOptions{Mode: "cgo"})
	if got := rt.ReadinessReason(); got != "llama_cpp_cgo_unavailable" {
		t.Fatalf("ReadinessReason() = %q, want llama_cpp_cgo_unavailable", got)
	}
}

func TestLlamaCppRuntimeReadinessReasonFFIModeUnavailable(t *testing.T) {
	m := NewManager(t.TempDir())
	createReadyModelFiles(t, m)
	rt := NewLlamaCppRuntime(m, LlamaCppRuntimeOptions{Mode: "ffi"})
	if got := rt.ReadinessReason(); got != "llama_cpp_ffi_unavailable" {
		t.Fatalf("ReadinessReason() = %q, want llama_cpp_ffi_unavailable", got)
	}
}

func TestLlamaCppRuntimeGenerateNotReadyWhenModelMissing(t *testing.T) {
	m := NewManager(t.TempDir())
	rt := NewLlamaCppRuntime(m)

	_, err := rt.Generate(context.Background(), GenerateRequest{Prompt: "hello"})
	if !errors.Is(err, ErrNotReady) {
		t.Fatalf("expected ErrNotReady, got %v", err)
	}
}

func TestLlamaCppRuntimeGenerateUsesDirectBackendInFFIMode(t *testing.T) {
	m := NewManager(t.TempDir())
	createReadyModelFiles(t, m)

	rt := NewLlamaCppRuntime(m, LlamaCppRuntimeOptions{
		Mode:      "ffi",
		ServerURL: "http://127.0.0.1:1",
		CLIPath:   "/definitely/missing/llama-cli",
	})
	fake := &fakeLlamaCppDirectBackend{text: "ffi answer"}
	rt.ffiBackend = fake

	resp, err := rt.Generate(context.Background(), GenerateRequest{
		Prompt:      "hello from ffi",
		MaxTokens:   17,
		Temperature: 0.5,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp == nil || resp.Text != "ffi answer" {
		t.Fatalf("Generate() = %#v, want ffi answer", resp)
	}
	if fake.ensureCalls != 1 {
		t.Fatalf("EnsureLoaded() calls = %d, want 1", fake.ensureCalls)
	}
	if fake.generateCalls != 1 {
		t.Fatalf("Generate() backend calls = %d, want 1", fake.generateCalls)
	}
	if fake.lastReq.ModelPath != m.ModelPath() {
		t.Fatalf("backend model path = %q, want %q", fake.lastReq.ModelPath, m.ModelPath())
	}
	if fake.lastReq.Prompt != "hello from ffi" {
		t.Fatalf("backend prompt = %q, want hello from ffi", fake.lastReq.Prompt)
	}
	if fake.lastReq.MaxTokens != 17 {
		t.Fatalf("backend max tokens = %d, want 17", fake.lastReq.MaxTokens)
	}
}

func TestLlamaCppRuntimePrefixCachingStub(t *testing.T) {
	m := NewManager(t.TempDir())
	createReadyModelFiles(t, m)

	var (
		mu       sync.Mutex
		requests []map[string]interface{}
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health", "/v1/models":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		case "/completion":
			defer r.Body.Close()
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode completion payload: %v", err)
			}
			mu.Lock()
			requests = append(requests, payload)
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"content":"answer"}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	rt := NewLlamaCppRuntime(m, LlamaCppRuntimeOptions{ServerURL: server.URL})

	prefixRt, ok := interface{}(rt).(PrefixCachingRuntime)
	if !ok {
		t.Fatal("LlamaCppRuntime should implement PrefixCachingRuntime")
	}

	stats := prefixRt.PrefixCacheStats()
	if !stats.Supported {
		t.Fatalf("PrefixCacheStats().Supported = false, want true")
	}

	result, err := prefixRt.Prefill(context.Background(), PrefillRequest{Prefix: "hello world", TTL: time.Minute})
	if err != nil {
		t.Fatalf("Prefill() error = %v", err)
	}
	if result == nil || result.PrefixID == "" {
		t.Fatalf("Prefill() result = %#v, want prefix metadata", result)
	}
	if result.CacheHit {
		t.Fatal("Prefill() CacheHit = true, want false on first prefill")
	}

	mu.Lock()
	if len(requests) != 1 {
		t.Fatalf("completion requests = %d, want 1", len(requests))
	}
	first := requests[0]
	mu.Unlock()
	if got := first["prompt"]; got != "hello world" {
		t.Fatalf("completion prompt = %#v, want %q", got, "hello world")
	}
	if got := first["cache_prompt"]; got != true {
		t.Fatalf("completion cache_prompt = %#v, want true", got)
	}
	if got := int(first["n_predict"].(float64)); got != 0 {
		t.Fatalf("completion n_predict = %d, want 0", got)
	}
	if got := int(first["id_slot"].(float64)); got <= 0 {
		t.Fatalf("completion id_slot = %d, want > 0", got)
	}

	resp, err := prefixRt.GenerateFromPrefix(context.Background(), GenerateFromPrefixRequest{
		PrefixID:    result.PrefixID,
		Suffix:      "\nAssistant:",
		MaxTokens:   24,
		Temperature: 0.2,
	})
	if err != nil {
		t.Fatalf("GenerateFromPrefix() error = %v", err)
	}
	if resp == nil || resp.Text != "answer" {
		t.Fatalf("GenerateFromPrefix() = %#v, want answer", resp)
	}

	mu.Lock()
	if len(requests) != 2 {
		t.Fatalf("completion requests = %d, want 2", len(requests))
	}
	second := requests[1]
	mu.Unlock()
	if got := second["prompt"]; got != "hello world\nAssistant:" {
		t.Fatalf("second completion prompt = %#v, want concatenated prompt", got)
	}
	if got := second["cache_prompt"]; got != true {
		t.Fatalf("second completion cache_prompt = %#v, want true", got)
	}
	if int(second["id_slot"].(float64)) != int(first["id_slot"].(float64)) {
		t.Fatalf("second completion id_slot = %#v, want reuse %#v", second["id_slot"], first["id_slot"])
	}

	stats = prefixRt.PrefixCacheStats()
	if stats.Entries != 1 {
		t.Fatalf("PrefixCacheStats().Entries = %d, want 1", stats.Entries)
	}
	if stats.Hits == 0 {
		t.Fatalf("PrefixCacheStats().Hits = %d, want > 0", stats.Hits)
	}

	if !prefixRt.EvictPrefix(result.PrefixID) {
		t.Fatal("EvictPrefix() = false, want true")
	}
	if prefixRt.EvictPrefix(result.PrefixID) {
		t.Fatal("EvictPrefix() second call = true, want false")
	}
}

func TestLlamaCppRuntimePrefillRejectsImages(t *testing.T) {
	m := NewManager(t.TempDir())
	createReadyModelFiles(t, m)
	rt := NewLlamaCppRuntime(m, LlamaCppRuntimeOptions{ServerURL: "http://127.0.0.1:18080"})
	prefixRt := interface{}(rt).(PrefixCachingRuntime)

	_, err := prefixRt.Prefill(context.Background(), PrefillRequest{
		Prefix: "hello",
		Images: []ImageInput{{MimeType: "image/png", Data: "abc"}},
	})
	if !errors.Is(err, ErrPrefixCachingUnsupported) {
		t.Fatalf("Prefill(images) error = %v, want ErrPrefixCachingUnsupported", err)
	}
}

func TestLlamaCppRuntimeBatchWindowGroupsRequests(t *testing.T) {
	m := NewManager(t.TempDir())
	createReadyModelFiles(t, m)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health", "/v1/models":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	rt := NewLlamaCppRuntime(m, LlamaCppRuntimeOptions{
		ServerURL:    server.URL,
		BatchWindow:  20 * time.Millisecond,
		BatchMaxSize: 8,
	})
	batchingRt, ok := interface{}(rt).(BatchingRuntime)
	if !ok {
		t.Fatal("LlamaCppRuntime should implement BatchingRuntime")
	}

	start := make(chan struct{})
	errCh := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			<-start
			_, err := rt.Generate(context.Background(), GenerateRequest{Prompt: fmt.Sprintf("hello %d", i), MaxTokens: 8})
			errCh <- err
		}(i)
	}
	close(start)
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
	}

	stats := batchingRt.BatchingStats()
	if !stats.Supported {
		t.Fatal("BatchingStats().Supported = false, want true")
	}
	if stats.Submitted != 2 {
		t.Fatalf("BatchingStats().Submitted = %d, want 2", stats.Submitted)
	}
	if stats.Executed != 2 {
		t.Fatalf("BatchingStats().Executed = %d, want 2", stats.Executed)
	}
	if stats.BatchCount != 1 {
		t.Fatalf("BatchingStats().BatchCount = %d, want 1", stats.BatchCount)
	}
	if stats.LargestBatch < 2 {
		t.Fatalf("BatchingStats().LargestBatch = %d, want >= 2", stats.LargestBatch)
	}
}
