package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
)

type prunerBackendMock struct {
	calls atomic.Int32
}

func (m *prunerBackendMock) Prune(_ context.Context, req pruner.PruneRequest) (*pruner.PruneResponse, error) {
	m.calls.Add(1)
	return &pruner.PruneResponse{
		PrunedContent:  req.GetContent(),
		PrunedCode:     req.GetContent(),
		OriginalTokens: 100,
		PrunedTokens:   60,
	}, nil
}

func (m *prunerBackendMock) Health(context.Context) error { return nil }
func (m *prunerBackendMock) Close() error                 { return nil }

func TestProxyHandler_UsesPrunerByDefault(t *testing.T) {
	backend := &prunerBackendMock{}
	cfg := pruner.DefaultConfig()
	cfg.Enabled = true
	cfg.MinLines = 2
	mw := pruner.NewMiddleware(backend, cfg, pruner.NewStats())

	handler := NewProxyHandler(nil, nil, nil)
	handler.SetPruner(mw)

	reqBody := `{"model":"auto","messages":[{"role":"user","content":"# A\n## B\ncontent"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if backend.calls.Load() == 0 {
		t.Fatal("expected pruner backend to be called")
	}
}

func TestProxyHandler_SkipsPrunerWhenContextDisabled(t *testing.T) {
	backend := &prunerBackendMock{}
	cfg := pruner.DefaultConfig()
	cfg.Enabled = true
	cfg.MinLines = 2
	mw := pruner.NewMiddleware(backend, cfg, pruner.NewStats())

	handler := NewProxyHandler(nil, nil, nil)
	handler.SetPruner(mw)

	reqBody := `{"model":"auto","messages":[{"role":"user","content":"# A\n## B\ncontent"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req = req.WithContext(pruner.WithPrunerDisabled(req.Context(), true))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if got := backend.calls.Load(); got != 0 {
		t.Fatalf("pruner backend calls = %d, want 0", got)
	}
}
