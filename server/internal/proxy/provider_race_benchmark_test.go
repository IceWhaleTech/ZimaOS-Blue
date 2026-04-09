package proxy

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	proxytestutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy/testutil"
)

type providerRaceBenchmarkHarness struct {
	handler      *ProxyHandler
	slowServer   *proxytestutil.MockServer
	fastServer   *proxytestutil.MockServer
	cleanupFuncs []func()
}

func newProviderRaceBenchmarkHarness(tb testing.TB, raceEnabled bool) *providerRaceBenchmarkHarness {
	tb.Helper()

	slowServer, err := proxytestutil.NewMockServer(0)
	if err != nil {
		tb.Fatalf("create slow mock server: %v", err)
	}
	slowServer.SetResponseDelay(180 * time.Millisecond)
	slowServer.SetCustomContent("slow")
	if err := slowServer.Start(); err != nil {
		tb.Fatalf("start slow mock server: %v", err)
	}

	fastServer, err := proxytestutil.NewMockServer(0)
	if err != nil {
		tb.Fatalf("create fast mock server: %v", err)
	}
	fastServer.SetResponseDelay(35 * time.Millisecond)
	fastServer.SetCustomContent("fast")
	if err := fastServer.Start(); err != nil {
		tb.Fatalf("start fast mock server: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "provider-race-bench-*")
	if err != nil {
		tb.Fatalf("create temp dir: %v", err)
	}

	storage, err := providerpool.NewFileStorage(tmpDir)
	if err != nil {
		tb.Fatalf("create file storage: %v", err)
	}
	registry, err := providerpool.NewRegistry(storage)
	if err != nil {
		tb.Fatalf("create registry: %v", err)
	}
	discovery := providerpool.NewModelDiscovery(registry, storage, time.Hour)
	router := providerpool.NewRouter(registry, discovery, providerpool.RoutingStrategyPriority)

	slowProvider := &providerpool.Provider{
		ID:        "p-slow",
		Name:      "slow-provider",
		Type:      providerpool.ProviderTypeCustom,
		BaseURL:   slowServer.URL(),
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
		BaseURL:   fastServer.URL(),
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		Priority:  10,
		APIFormat: providerpool.APIFormatOpenAI,
		APIKeys:   []providerpool.APIKey{{ID: "k-fast", Key: "fast-key", Enabled: true}},
	}
	if err := registry.Register(slowProvider); err != nil {
		tb.Fatalf("register slow provider: %v", err)
	}
	if err := registry.Register(fastProvider); err != nil {
		tb.Fatalf("register fast provider: %v", err)
	}

	slowModels := []*providerpool.Model{
		{
			ID:           "race-model",
			Name:         "race-model",
			ProviderID:   slowProvider.ID,
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, Streaming: true},
		},
	}
	fastModels := []*providerpool.Model{
		{
			ID:           "race-model",
			Name:         "race-model",
			ProviderID:   fastProvider.ID,
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, Streaming: true},
		},
	}
	if err := storage.SaveModels(slowProvider.ID, slowModels); err != nil {
		tb.Fatalf("save slow models: %v", err)
	}
	if err := storage.SaveModels(fastProvider.ID, fastModels); err != nil {
		tb.Fatalf("save fast models: %v", err)
	}
	router.RebuildCandidates()

	handler := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	handler.SetProviderPool(&providerpool.Pool{
		Registry:  registry,
		Discovery: discovery,
		Router:    router,
	})
	handler.SetProviderRaceConfig(ProviderRaceConfig{
		Enabled:                    raceEnabled,
		MaxParallel:                2,
		MinProviders:               2,
		EmptyRateMinSamples:        10,
		EmptyRateCooldownThreshold: 0.3,
		EmptyRateSinkThreshold:     0.5,
		EmptyRateExcludeThreshold:  0.8,
		EmptyRateCooldown:          2 * time.Minute,
	})

	return &providerRaceBenchmarkHarness{
		handler:    handler,
		slowServer: slowServer,
		fastServer: fastServer,
		cleanupFuncs: []func(){
			func() { _ = slowServer.Stop() },
			func() { _ = fastServer.Stop() },
			func() { handler.connPool.Close() },
			func() { _ = os.RemoveAll(tmpDir) },
		},
	}
}

func (h *providerRaceBenchmarkHarness) Cleanup() {
	for i := len(h.cleanupFuncs) - 1; i >= 0; i-- {
		h.cleanupFuncs[i]()
	}
}

func (h *providerRaceBenchmarkHarness) Execute(tb testing.TB) (provider string, latency time.Duration, statusCode int, body string) {
	tb.Helper()

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"model":"race-model","messages":[{"role":"user","content":"benchmark"}]}`))
	rec := httptest.NewRecorder()

	start := time.Now()
	h.handler.ServeHTTP(rec, req)
	latency = time.Since(start)

	return rec.Header().Get("X-Actual-Provider"), latency, rec.Code, rec.Body.String()
}

func TestProviderRaceWithMockUpstreamsImprovesFirstHitLatency(t *testing.T) {
	serialHarness := newProviderRaceBenchmarkHarness(t, false)
	defer serialHarness.Cleanup()
	time.Sleep(100 * time.Millisecond)
	serialHarness.slowServer.ResetRequestCount()
	serialHarness.fastServer.ResetRequestCount()

	serialProvider, serialLatency, serialCode, serialBody := serialHarness.Execute(t)
	if serialCode != http.StatusOK {
		t.Fatalf("serial status=%d body=%s", serialCode, serialBody)
	}
	if serialProvider != "slow-provider" {
		t.Fatalf("serial provider=%q want slow-provider", serialProvider)
	}

	raceHarness := newProviderRaceBenchmarkHarness(t, true)
	defer raceHarness.Cleanup()
	time.Sleep(100 * time.Millisecond)
	raceHarness.slowServer.ResetRequestCount()
	raceHarness.fastServer.ResetRequestCount()

	raceProvider, raceLatency, raceCode, raceBody := raceHarness.Execute(t)
	if raceCode != http.StatusOK {
		t.Fatalf("race status=%d body=%s", raceCode, raceBody)
	}
	if raceProvider != "fast-provider" {
		t.Fatalf("race provider=%q want fast-provider", raceProvider)
	}

	if raceLatency >= serialLatency-80*time.Millisecond {
		t.Fatalf("race latency=%v did not materially improve over serial=%v", raceLatency, serialLatency)
	}

	t.Logf("serial latency=%v provider=%s", serialLatency, serialProvider)
	t.Logf("serial request counts slow=%d fast=%d", serialHarness.slowServer.GetRequestCount(), serialHarness.fastServer.GetRequestCount())
	t.Logf("race latency=%v provider=%s", raceLatency, raceProvider)
	t.Logf("race request counts slow=%d fast=%d", raceHarness.slowServer.GetRequestCount(), raceHarness.fastServer.GetRequestCount())
	t.Logf("latency improvement=%v", serialLatency-raceLatency)
}

func TestProviderRaceStatsSnapshotTracksHitsAndEstimatedSavings(t *testing.T) {
	harness := newProviderRaceBenchmarkHarness(t, true)
	defer harness.Cleanup()
	time.Sleep(100 * time.Millisecond)

	if harness.handler.providerPool == nil || harness.handler.providerPool.Router == nil {
		t.Fatal("expected provider pool router")
	}
	harness.handler.providerPool.Router.UpdateLatency("p-slow", 180*time.Millisecond)

	provider, latency, statusCode, body := harness.Execute(t)
	if statusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", statusCode, body)
	}
	if provider != "fast-provider" {
		t.Fatalf("provider=%q want fast-provider", provider)
	}
	if latency <= 0 {
		t.Fatalf("latency=%v want > 0", latency)
	}

	stats := harness.handler.GetProviderRaceStats()
	if stats.RequestsTotal != 1 {
		t.Fatalf("requests_total=%d want 1", stats.RequestsTotal)
	}
	if stats.SuccessfulRaces != 1 {
		t.Fatalf("successful_races=%d want 1", stats.SuccessfulRaces)
	}
	if stats.Hits != 1 {
		t.Fatalf("hits=%d want 1", stats.Hits)
	}
	if stats.HitRate != 1 {
		t.Fatalf("hit_rate=%v want 1", stats.HitRate)
	}
	if stats.AvgWinnerLatencyMs <= 0 {
		t.Fatalf("avg_winner_latency_ms=%v want > 0", stats.AvgWinnerLatencyMs)
	}
	if stats.EstimatedLatencySavedMs <= 0 {
		t.Fatalf("estimated_latency_saved_ms=%v want > 0", stats.EstimatedLatencySavedMs)
	}
	if stats.EstimatedSavingsSamples != 1 {
		t.Fatalf("estimated_savings_samples=%d want 1", stats.EstimatedSavingsSamples)
	}
}

func BenchmarkProviderRaceWithMockUpstreams(b *testing.B) {
	cases := []struct {
		name        string
		raceEnabled bool
		wantWinner  string
	}{
		{
			name:        "race_disabled",
			raceEnabled: false,
			wantWinner:  "slow-provider",
		},
		{
			name:        "race_enabled",
			raceEnabled: true,
			wantWinner:  "fast-provider",
		},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			harness := newProviderRaceBenchmarkHarness(b, tc.raceEnabled)
			defer harness.Cleanup()

			harness.slowServer.ResetRequestCount()
			harness.fastServer.ResetRequestCount()

			requestBody := `{"model":"race-model","messages":[{"role":"user","content":"benchmark"}]}`
			winCount := 0

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(requestBody))
				rec := httptest.NewRecorder()

				harness.handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusOK {
					b.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
				}
				if got := rec.Header().Get("X-Actual-Provider"); got != tc.wantWinner {
					b.Fatalf("winner=%q want=%q", got, tc.wantWinner)
				}
				winCount++
			}
			b.StopTimer()

			totalUpstream := harness.slowServer.GetRequestCount() + harness.fastServer.GetRequestCount()
			b.ReportMetric(float64(totalUpstream)/float64(b.N), "upstream_req/op")
			b.ReportMetric(float64(winCount)*100/float64(b.N), "winner_match_pct")
		})
	}
}
