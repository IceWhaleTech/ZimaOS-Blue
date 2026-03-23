package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

type chatPerfScenario struct {
	name                string
	durability          string
	readLite            bool
	persistAsync        bool
	externalAttachments bool
}

type chatPerfMetricsStub struct {
	mu       sync.Mutex
	counters map[string]int64
}

func (m *chatPerfMetricsStub) RecordAPICall(string, bool, float64, int64, int64, int64, int64, string) {
}

func (m *chatPerfMetricsStub) RecordAPICallForUser(string, string, bool, float64, int64, int64, int64, int64, string) {
}

func (m *chatPerfMetricsStub) RecordSpeed(string, float64, float64, float64) {}

func (m *chatPerfMetricsStub) RecordCounter(name string, value int64, _ map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.counters == nil {
		m.counters = make(map[string]int64)
	}
	m.counters[name] += value
}

func (m *chatPerfMetricsStub) Counter(name string) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.counters[name]
}

type timingResponseRecorder struct {
	*httptest.ResponseRecorder
	firstWriteAt time.Time
}

func newTimingResponseRecorder() *timingResponseRecorder {
	return &timingResponseRecorder{ResponseRecorder: httptest.NewRecorder()}
}

func (r *timingResponseRecorder) Write(p []byte) (int, error) {
	if r.firstWriteAt.IsZero() && len(p) > 0 {
		r.firstWriteAt = time.Now()
	}
	return r.ResponseRecorder.Write(p)
}

func (r *timingResponseRecorder) WriteString(s string) (int, error) {
	if r.firstWriteAt.IsZero() && len(s) > 0 {
		r.firstWriteAt = time.Now()
	}
	return r.ResponseRecorder.WriteString(s)
}

type chatPerfFixture struct {
	handler *ChatHandler
	store   *memory.Store
	convID  string
	metrics *chatPerfMetricsStub
}

func BenchmarkChatHandlerSendMessageLargeHistory(b *testing.B) {
	scenarios := []chatPerfScenario{
		{
			name:                "legacy_full_sync",
			durability:          "full",
			readLite:            false,
			persistAsync:        false,
			externalAttachments: false,
		},
		{
			name:                "optimized_normal_lite_external",
			durability:          "normal",
			readLite:            true,
			persistAsync:        false,
			externalAttachments: true,
		},
	}

	for _, scenario := range scenarios {
		scenario := scenario
		b.Run(scenario.name, func(b *testing.B) {
			runSendMessageLargeHistoryBenchmark(b, scenario)
		})
	}
}

func BenchmarkChatHandlerStreamMessageLargeHistory(b *testing.B) {
	scenarios := []chatPerfScenario{
		{
			name:                "legacy_full_sync",
			durability:          "full",
			readLite:            false,
			persistAsync:        false,
			externalAttachments: false,
		},
		{
			name:                "optimized_normal_lite_external_sync",
			durability:          "normal",
			readLite:            true,
			persistAsync:        false,
			externalAttachments: true,
		},
		{
			name:                "optimized_normal_lite_external_async",
			durability:          "normal",
			readLite:            true,
			persistAsync:        true,
			externalAttachments: true,
		},
	}

	for _, scenario := range scenarios {
		scenario := scenario
		b.Run(scenario.name, func(b *testing.B) {
			runStreamMessageLargeHistoryBenchmark(b, scenario)
		})
	}
}

func runSendMessageLargeHistoryBenchmark(b *testing.B, scenario chatPerfScenario) {
	b.Helper()
	b.ReportAllocs()

	baseDir := b.TempDir()
	samples := make([]time.Duration, 0, b.N)

	for i := 0; i < b.N; i++ {
		fixture := newChatPerfFixture(b, filepath.Join(baseDir, fmt.Sprintf("send-%03d", i)), scenario, false)
		e := echo.New()
		body := `{"message":"Continue from the earlier debugging thread and restate the root cause, exact file path, and request id.","provider":"mock","model":"mock-model"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+fixture.convID+"/messages", bytes.NewBufferString(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(fixture.convID)

		b.StartTimer()
		start := time.Now()
		err := fixture.handler.SendMessage(c)
		elapsed := time.Since(start)
		b.StopTimer()

		if err != nil {
			fixture.close()
			b.Fatalf("SendMessage(%s): %v", scenario.name, err)
		}
		if rec.Code != http.StatusOK {
			fixture.close()
			b.Fatalf("SendMessage(%s) status=%d body=%s", scenario.name, rec.Code, rec.Body.String())
		}

		samples = append(samples, elapsed)
		fixture.close()
	}

	reportDurationMetrics(b, samples, "handler")
}

func runStreamMessageLargeHistoryBenchmark(b *testing.B, scenario chatPerfScenario) {
	b.Helper()
	b.ReportAllocs()

	baseDir := b.TempDir()
	totalSamples := make([]time.Duration, 0, b.N)
	ttftSamples := make([]time.Duration, 0, b.N)
	var totalDraftUpdates float64
	var totalAssistantWrites float64

	for i := 0; i < b.N; i++ {
		fixture := newChatPerfFixture(b, filepath.Join(baseDir, fmt.Sprintf("stream-%03d", i)), scenario, true)
		e := echo.New()
		body := `{"message":"Continue from the earlier debugging thread and give a concise status update with the exact file path and request id.","provider":"streaming-mock","model":"streaming-mock-model"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+fixture.convID+"/messages/stream", bytes.NewBufferString(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := newTimingResponseRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(fixture.convID)

		persistedOpsBefore := fixture.metrics.Counter("chat_persist_batch_size")

		b.StartTimer()
		start := time.Now()
		err := fixture.handler.StreamMessage(c)
		elapsed := time.Since(start)
		b.StopTimer()

		if err != nil {
			fixture.close()
			b.Fatalf("StreamMessage(%s): %v", scenario.name, err)
		}
		if rec.Code != http.StatusOK {
			fixture.close()
			b.Fatalf("StreamMessage(%s) status=%d body=%s", scenario.name, rec.Code, rec.Body.String())
		}

		draftUpdates, ok := extractStreamDraftDBUpdates(rec.Body.String())
		if !ok {
			fixture.close()
			b.Fatalf("StreamMessage(%s) missing draft_db_updates in SSE payload", scenario.name)
		}

		totalSamples = append(totalSamples, elapsed)
		if rec.firstWriteAt.IsZero() {
			ttftSamples = append(ttftSamples, elapsed)
		} else {
			ttftSamples = append(ttftSamples, rec.firstWriteAt.Sub(start))
		}
		totalDraftUpdates += float64(draftUpdates)

		if scenario.persistAsync {
			totalAssistantWrites += float64(fixture.metrics.Counter("chat_persist_batch_size") - persistedOpsBefore)
		} else {
			totalAssistantWrites += float64(nonNegativeInt(draftUpdates) + 1)
		}

		fixture.close()
	}

	reportDurationMetrics(b, totalSamples, "total")
	reportDurationMetrics(b, ttftSamples, "ttft")
	if b.N > 0 {
		b.ReportMetric(totalDraftUpdates/float64(b.N), "draft_updates/op")
		b.ReportMetric(totalAssistantWrites/float64(b.N), "assistant_writes/op")
	}
}

func newChatPerfFixture(b *testing.B, dir string, scenario chatPerfScenario, streaming bool) *chatPerfFixture {
	b.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		b.Fatalf("MkdirAll(%s): %v", dir, err)
	}

	dbPath := filepath.Join(dir, "chat.db")
	opts := memory.DefaultChatStoreOptions(dbPath)
	opts.Durability = scenario.durability
	opts.CheckpointInterval = 0
	opts.RuntimeStateCleanupInterval = 0
	opts.AttachmentExternalStore = scenario.externalAttachments
	if scenario.externalAttachments {
		opts.AttachmentDir = filepath.Join(dir, "attachments")
	}

	store, err := memory.NewStoreWithOptions(dbPath, opts)
	if err != nil {
		b.Fatalf("NewStoreWithOptions(%s): %v", scenario.name, err)
	}

	registry := llm.NewProviderRegistry()
	if streaming {
		registry.Register(NewStreamingMockProvider(benchmarkStreamingResponse()))
	} else {
		mockProvider := llm.NewMockProvider()
		mockProvider.SetResponse(llm.ChatResponse{
			ID:    "bench-response",
			Model: "mock-model",
			Message: llm.Message{
				Role:    llm.RoleAssistant,
				Content: "The root cause was API key rotation on 2026-03-18 breaking refresh handling in auth/middleware.go for request req_0099.",
			},
			Usage: llm.Usage{PromptTokens: 256, CompletionTokens: 48, TotalTokens: 304},
		})
		registry.Register(mockProvider)
	}

	handler := NewChatHandler(store, registry, tools.NewRegistry())
	metrics := &chatPerfMetricsStub{counters: make(map[string]int64)}
	handler.SetMetricsRecorder(metrics)
	handler.SetPersistenceOptions(scenario.persistAsync, scenario.readLite)

	conv, err := store.CreateConversation(context.Background(), "Benchmark conversation")
	if err != nil {
		handler.Shutdown()
		_ = store.Close()
		b.Fatalf("CreateConversation(%s): %v", scenario.name, err)
	}

	populateChatPerfConversation(b, store, conv.ID)

	return &chatPerfFixture{
		handler: handler,
		store:   store,
		convID:  conv.ID,
		metrics: metrics,
	}
}

func (f *chatPerfFixture) close() {
	if f == nil {
		return
	}
	if f.handler != nil {
		f.handler.Shutdown()
	}
	if f.store != nil {
		_ = f.store.Close()
	}
}

func populateChatPerfConversation(b *testing.B, store *memory.Store, convID string) {
	b.Helper()

	ctx := context.Background()
	stats := &memory.MessageStats{
		InputTokens:     1024,
		OutputTokens:    512,
		TotalTokens:     1536,
		LatencyMs:       2500,
		TTFTMs:          450,
		TokensPerSecond: 72.1,
	}
	attachmentPayload := strings.Repeat("A", 24*1024)
	attachments := []memory.MessageAttachment{
		{
			Type:     "file",
			Name:     "artifact.txt",
			MimeType: "text/plain",
			Data:     attachmentPayload,
		},
	}

	for i := 0; i < 100; i++ {
		if _, err := store.AddMessageTrusted(ctx, convID, memory.Message{
			Role:    "user",
			Content: fmt.Sprintf("Round %03d: keep the exact root cause, file path auth/middleware.go, and request id req_%04d available for later follow-ups.", i, i),
		}); err != nil {
			b.Fatalf("AddMessageTrusted(user %d): %v", i, err)
		}

		msg := memory.Message{
			Role:     "assistant",
			Content:  fmt.Sprintf("Round %03d findings: API key rotation on 2026-03-18 broke refresh handling in auth/middleware.go and impacted request req_%04d. Keep the remediation notes concise.", i, i),
			Provider: "bench",
			Model:    "bench-model",
			Stats:    stats,
		}
		if i%4 == 0 {
			msg.Attachments = attachments
		}
		if _, err := store.AddMessageTrusted(ctx, convID, msg); err != nil {
			b.Fatalf("AddMessageTrusted(assistant %d): %v", i, err)
		}
	}
}

func benchmarkStreamingResponse() string {
	part := strings.Repeat("delta", 24)
	parts := make([]string, 0, 100)
	for i := 0; i < 100; i++ {
		parts = append(parts, fmt.Sprintf("%s-%03d", part, i))
	}
	return strings.Join(parts, " ")
}

func extractStreamDraftDBUpdates(body string) (int, bool) {
	lines := strings.Split(body, "\n")
	lastDone := -1
	for _, line := range lines {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		raw := strings.TrimPrefix(line, "data: ")
		if raw == "[DONE]" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			continue
		}
		done, _ := payload["done"].(bool)
		if !done {
			continue
		}
		switch value := payload["draft_db_updates"].(type) {
		case float64:
			lastDone = int(value)
		case int:
			lastDone = value
		}
	}
	if lastDone < 0 {
		return 0, false
	}
	return lastDone, true
}

func reportDurationMetrics(b *testing.B, samples []time.Duration, prefix string) {
	b.Helper()
	if len(samples) == 0 {
		return
	}
	b.ReportMetric(float64(percentileDuration(samples, 0.50).Microseconds())/1000.0, prefix+"_p50_ms")
	b.ReportMetric(float64(percentileDuration(samples, 0.95).Microseconds())/1000.0, prefix+"_p95_ms")
}

func percentileDuration(samples []time.Duration, percentile float64) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	if percentile <= 0 {
		return sorted[0]
	}
	if percentile >= 1 {
		return sorted[len(sorted)-1]
	}

	idx := int(math.Ceil(percentile*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func nonNegativeInt(value int) int {
	if value < 0 {
		return 0
	}
	return value
}
