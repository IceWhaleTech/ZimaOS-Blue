package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/metrics"
)

type stubCacheFootprintProvider struct {
	footprint ChatCacheFootprint
}

func (s stubCacheFootprintProvider) ChatCacheFootprint() ChatCacheFootprint {
	return s.footprint
}

func TestMetricsHandlerGetCurrentMetrics_EmbedsChatCacheFootprint(t *testing.T) {
	collector := metrics.NewCollector(time.Minute, 4)
	collector.Start()
	defer collector.Stop()

	handler := NewMetricsHandler(collector, stubCacheFootprintProvider{
		footprint: ChatCacheFootprint{
			ConversationCacheEntries:      3,
			ConversationCacheBytes:        4096,
			WarmupCacheEntries:            2,
			WarmupCacheBytes:              2048,
			PromptToolSurfaceRefs:         5,
			PromptToolSurfaceSharedEntries: 1,
			PromptToolSurfaceSharedBytes:  1024,
			ProviderAffinityEntries:       4,
		},
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/system/metrics", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.GetCurrentMetrics(c); err != nil {
		t.Fatalf("GetCurrentMetrics() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("GetCurrentMetrics() status=%d, want 200", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	expect := map[string]float64{
		"conversation_cache_entries":         3,
		"conversation_cache_bytes":           4096,
		"warmup_cache_entries":               2,
		"warmup_cache_bytes":                 2048,
		"prompt_tool_surface_refs":           5,
		"prompt_tool_surface_shared_entries": 1,
		"prompt_tool_surface_shared_bytes":   1024,
		"provider_affinity_entries":          4,
	}
	for key, want := range expect {
		got, ok := body[key].(float64)
		if !ok {
			t.Fatalf("body[%q]=%T, want float64 payload field", key, body[key])
		}
		if got != want {
			t.Fatalf("body[%q]=%v, want %v", key, got, want)
		}
	}
}
