package server

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

type bootstrapCapture struct {
	mu    sync.Mutex
	paths []string
	bodies []string
}

func (c *bootstrapCapture) add(path, body string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.paths = append(c.paths, path)
	c.bodies = append(c.bodies, body)
}

func (c *bootstrapCapture) snapshot() ([]string, []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p := make([]string, len(c.paths))
	b := make([]string, len(c.bodies))
	copy(p, c.paths)
	copy(b, c.bodies)
	return p, b
}

func TestStreamMessageBootstrapInjection_RealCodex(t *testing.T) {
	if strings.TrimSpace(os.Getenv("ZIMA_RUN_REAL_CODEX")) != "1" {
		t.Skip("set ZIMA_RUN_REAL_CODEX=1 to run bootstrap injection test against real codex provider")
	}

	baseURL := strings.TrimSpace(os.Getenv("ZIMA_REAL_CODEX_BASE_URL"))
	apiKey := strings.TrimSpace(os.Getenv("ZIMA_REAL_CODEX_API_KEY"))
	modelID := strings.TrimSpace(os.Getenv("ZIMA_REAL_CODEX_MODEL"))
	if modelID == "" {
		modelID = "gpt-5.3-codex-spark"
	}
	if baseURL == "" || apiKey == "" {
		t.Skip("missing ZIMA_REAL_CODEX_BASE_URL or ZIMA_REAL_CODEX_API_KEY")
	}
	baseURL = strings.TrimRight(baseURL, "/")

	capture := &bootstrapCapture{}
	client := &http.Client{Timeout: 90 * time.Second}

	forward := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		capture.add(r.URL.Path, string(bodyBytes))

		targetURL := baseURL + r.URL.RequestURI()
		req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, bytes.NewReader(bodyBytes))
		if err != nil {
			http.Error(w, "failed to build upstream request: "+err.Error(), http.StatusBadGateway)
			return
		}
		req.Header = r.Header.Clone()

		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "upstream request failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		for k, vals := range resp.Header {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}))
	defer forward.Close()

	proxyHandler := newSingleModelOpenAIProxyHandlerWithAPIKey(t, forward.URL, "real-codex-bootstrap", modelID, apiKey)
	fixture := newReminderE2EFixture(t, llm.NewProviderRegistry(), proxybridge.NewBridge(proxyHandler))

	workspaceDir := t.TempDir()
	mgr := workspace.NewManager(workspaceDir)
	if err := mgr.EnsureWorkspace(); err != nil {
		t.Fatalf("failed to ensure workspace: %v", err)
	}

	builder := claudecode.NewSystemPromptBuilder(&claudecode.ClaudeCodeConfig{WorkspaceDir: workspaceDir})
	builder.SetWorkspace(mgr)
	fixture.handler.SetSystemPromptBuilder(builder)

	turn1 := runStreamTurn(t, fixture.handler, fixture.convID, buildStreamRequestBody("你好", modelID, ""))
	if strings.Contains(turn1, `"error":"STREAM_ERROR"`) {
		t.Fatalf("first stream failed: %s", turn1)
	}

	paths1, bodies1 := capture.snapshot()
	if len(paths1) == 0 {
		t.Fatal("no upstream request captured for turn1")
	}

	containsBootstrapQuestions := func(body string) bool {
		lower := strings.ToLower(body)
		return strings.Contains(lower, "bootstrap.md") &&
			strings.Contains(lower, "what should i call you") &&
			strings.Contains(lower, "what timezone are you in") &&
			strings.Contains(lower, "what language do you prefer")
	}

	seenTurn1 := false
	for _, body := range bodies1 {
		if containsBootstrapQuestions(body) {
			seenTurn1 = true
			break
		}
	}
	if !seenTurn1 {
		t.Fatalf("turn1 upstream request did not include BOOTSTRAP.md questions; captured %d requests", len(bodies1))
	}

	beforeCount := len(bodies1)
	conv2, err := fixture.store.CreateConversation(context.Background(), "Reminder E2E 2")
	if err != nil {
		t.Fatalf("failed to create second conversation: %v", err)
	}
	turn2 := runStreamTurn(t, fixture.handler, conv2.ID, buildStreamRequestBody("你好", modelID, ""))
	if strings.Contains(turn2, `"error":"STREAM_ERROR"`) {
		t.Fatalf("second independent stream failed: %s", turn2)
	}

	_, bodies2 := capture.snapshot()
	if len(bodies2) <= beforeCount {
		t.Fatalf("expected additional upstream request for second conversation, before=%d after=%d", beforeCount, len(bodies2))
	}

	seenTurn2 := false
	for _, body := range bodies2[beforeCount:] {
		if containsBootstrapQuestions(body) {
			seenTurn2 = true
			break
		}
	}
	if !seenTurn2 {
		t.Fatalf("second conversation upstream request did not include BOOTSTRAP.md questions; new requests=%d", len(bodies2)-beforeCount)
	}
}
