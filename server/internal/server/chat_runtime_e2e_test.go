package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

const chatRuntimeE2EDefaultCCCLIModel = "gpt-5.3-codex-spark"

type chatRuntimeE2ECLIInvocation struct {
	Model       string `json:"model"`
	Prompt      string `json:"prompt"`
	BaseURL     string `json:"base_url"`
	AuthPresent bool   `json:"auth_present"`
}

type chatRuntimeE2EProxyRequest struct {
	Path          string
	Model         string
	Stream        bool
	Authorization string
}

type chatRuntimeE2EProxyCapture struct {
	mu       sync.Mutex
	requests []chatRuntimeE2EProxyRequest
}

func (c *chatRuntimeE2EProxyCapture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bodyBytes, _ := io.ReadAll(r.Body)
	defer r.Body.Close()

	var payload struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	_ = json.Unmarshal(bodyBytes, &payload)

	c.mu.Lock()
	c.requests = append(c.requests, chatRuntimeE2EProxyRequest{
		Path:          r.URL.Path,
		Model:         strings.TrimSpace(payload.Model),
		Stream:        payload.Stream,
		Authorization: r.Header.Get("Authorization"),
	})
	c.mu.Unlock()

	if payload.Stream || strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream") {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			fmt.Fprint(w, "data: "+`{"id":"resp_e2e","choices":[{"delta":{"content":"via"},"finish_reason":null}],"model":"proxy-e2e-model"}`+"\n\n")
			flusher.Flush()
			fmt.Fprint(w, "data: "+`{"id":"resp_e2e","choices":[{"delta":{"content":" proxy"},"finish_reason":"stop"}],"model":"proxy-e2e-model"}`+"\n\n")
			flusher.Flush()
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"id":"resp_e2e","model":"proxy-e2e-model","choices":[{"message":{"role":"assistant","content":"via cli proxy"},"finish_reason":"stop"}]}`))
}

func (c *chatRuntimeE2EProxyCapture) Snapshot() []chatRuntimeE2EProxyRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]chatRuntimeE2EProxyRequest, len(c.requests))
	copy(out, c.requests)
	return out
}

type chatRuntimeE2EBridgeProvider struct {
	bridge *proxybridge.Bridge
}

func (p *chatRuntimeE2EBridgeProvider) Name() string {
	return "proxy"
}

func (p *chatRuntimeE2EBridgeProvider) Models() []string {
	return nil
}

func (p *chatRuntimeE2EBridgeProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return p.bridge.Chat(ctx, req)
}

func (p *chatRuntimeE2EBridgeProvider) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 64)
	go func() {
		defer close(ch)
		_ = p.bridge.ChatStream(ctx, req, func(chunk llm.StreamChunk) error {
			ch <- chunk
			return nil
		})
	}()
	return ch, nil
}

func (p *chatRuntimeE2EBridgeProvider) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, cb llm.StreamCallback) error {
	return p.bridge.ChatStream(ctx, req, cb)
}

type chatRuntimeE2EToggleProvider struct {
	ccEnabled bool
	cc        llm.Provider
	proxy     llm.Provider
}

func (p *chatRuntimeE2EToggleProvider) Name() string {
	if p.ccEnabled {
		return p.cc.Name()
	}
	return p.proxy.Name()
}

func (p *chatRuntimeE2EToggleProvider) Models() []string {
	if p.ccEnabled {
		return p.cc.Models()
	}
	return p.proxy.Models()
}

func (p *chatRuntimeE2EToggleProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	req = p.normalize(req)
	if p.ccEnabled {
		return p.cc.Chat(ctx, req)
	}
	return p.proxy.Chat(ctx, req)
}

func (p *chatRuntimeE2EToggleProvider) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	req = p.normalize(req)
	if p.ccEnabled {
		return p.cc.ChatStream(ctx, req)
	}
	return p.proxy.ChatStream(ctx, req)
}

func (p *chatRuntimeE2EToggleProvider) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, cb llm.StreamCallback) error {
	req = p.normalize(req)
	if p.ccEnabled {
		return p.cc.ChatStreamCallback(ctx, req, cb)
	}
	return p.proxy.ChatStreamCallback(ctx, req, cb)
}

func (p *chatRuntimeE2EToggleProvider) normalize(req llm.ChatRequest) llm.ChatRequest {
	if !p.ccEnabled {
		return req
	}
	model := strings.TrimSpace(req.Model)
	if model == "" || strings.EqualFold(model, "auto") || strings.EqualFold(model, chatRuntimeE2EDefaultCCCLIModel) {
		req.Model = ""
	}
	return req
}

func installFakeChatRuntimeCCCLIBinary(t *testing.T, homeDir string) string {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skipf("python3 not available for fake CC CLI backend: %v", err)
	}
	if runtime.GOOS == "windows" {
		t.Skip("fake CC CLI e2e test is unix-only")
	}

	binaryDir := filepath.Join(homeDir, ".local", "share", "zimaos-blue", "claude-code")
	if err := os.MkdirAll(binaryDir, 0o755); err != nil {
		t.Fatalf("failed to create fake CC CLI binary dir: %v", err)
	}
	scriptPath := filepath.Join(binaryDir, "claude")
	script := `#!/usr/bin/env python3
import json
import os
import sys
import urllib.request

args = sys.argv[1:]
model = ""
prompt = args[-1] if args else ""
for i, arg in enumerate(args):
    if arg == "--model" and i + 1 < len(args):
        model = args[i + 1]

base_url = os.environ.get("ANTHROPIC_BASE_URL", "").rstrip("/")
token = os.environ.get("ANTHROPIC_AUTH_TOKEN", "")
log_path = os.environ.get("ZIMA_FAKE_CC_LOG", "")

if log_path:
    with open(log_path, "a", encoding="utf-8") as fh:
        fh.write(json.dumps({
            "model": model,
            "prompt": prompt,
            "base_url": base_url,
            "auth_present": bool(token),
        }, ensure_ascii=False) + "\n")

payload = json.dumps({
    "model": model,
    "messages": [{"role": "user", "content": prompt}],
    "stream": False,
}).encode("utf-8")

req = urllib.request.Request(
    base_url + "/v1/chat/completions",
    data=payload,
    headers={"Content-Type": "application/json"},
    method="POST",
)
if token:
    req.add_header("Authorization", "Bearer " + token)

with urllib.request.urlopen(req, timeout=30) as resp:
    body = json.loads(resp.read().decode("utf-8"))

text = ""
choices = body.get("choices") or []
if choices:
    message = choices[0].get("message") or {}
    text = message.get("content", "")

sys.stdout.write(text)
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake CC CLI backend: %v", err)
	}
	return scriptPath
}

func readChatRuntimeCLIInvocations(t *testing.T, logPath string) []chatRuntimeE2ECLIInvocation {
	t.Helper()

	f, err := os.Open(logPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("failed to open fake CC CLI log: %v", err)
	}
	defer f.Close()

	var invocations []chatRuntimeE2ECLIInvocation
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var invocation chatRuntimeE2ECLIInvocation
		if err := json.Unmarshal([]byte(line), &invocation); err != nil {
			t.Fatalf("failed to decode fake CC CLI log line %q: %v", line, err)
		}
		invocations = append(invocations, invocation)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("failed to scan fake CC CLI log: %v", err)
	}
	return invocations
}

func TestChatRuntime_EndToEnd_ClaudeCodeToggle(t *testing.T) {
	tests := []struct {
		name               string
		ccEnabled          bool
		wantCLIInvocations int
		wantProxyModel     string
		wantProxyStream    bool
	}{
		{
			name:               "cccli_disabled_routes_directly_to_proxy",
			ccEnabled:          false,
			wantCLIInvocations: 0,
			wantProxyModel:     "auto",
			wantProxyStream:    true,
		},
		{
			name:               "cccli_enabled_routes_through_cli_then_proxy",
			ccEnabled:          true,
			wantCLIInvocations: 1,
			wantProxyModel:     "fake-cc-default",
			wantProxyStream:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			homeDir := t.TempDir()
			t.Setenv("HOME", homeDir)
			scriptPath := installFakeChatRuntimeCCCLIBinary(t, homeDir)

			logPath := filepath.Join(t.TempDir(), "fake-cc.log")
			t.Setenv("ZIMA_FAKE_CC_LOG", logPath)

			proxyCapture := &chatRuntimeE2EProxyCapture{}
			proxyServer := httptest.NewServer(proxyCapture)
			defer proxyServer.Close()

			ccKV := kvstore.NewMemoryStore()
			if err := ccKV.SetJSON(context.Background(), "config:claudecode", &claudecode.ClaudeCodePersistentConfig{
				Enabled:        tt.ccEnabled,
				DefaultModel:   "fake-cc-default",
				SandboxEnabled: false,
				NetworkEnabled: true,
			}, 0); err != nil {
				t.Fatalf("seed claudecode config: %v", err)
			}
			ccHandler := claudecode.NewHandlerWithDataDir(nil, "", ccKV)

			ccProvider := claudecode.NewProvider(&claudecode.ClaudeCodeConfig{
				Enabled:        true,
				Command:        scriptPath,
				DefaultModel:   "fake-cc-default",
				BaseURL:        proxyServer.URL,
				APIKey:         "blue-internal-claudecode",
				ActualProvider: "proxy",
				Backend: claudecode.CliBackendConfig{
					Command: scriptPath,
					Output:  "text",
					Input:   "arg",
				},
				Sandbox: claudecode.SandboxConfig{
					Enabled: false,
				},
			})
			proxyProvider := &chatRuntimeE2EBridgeProvider{bridge: proxybridge.NewBridge(proxyCapture)}
			runtimeProvider := &chatRuntimeE2EToggleProvider{
				ccEnabled: tt.ccEnabled,
				cc:        ccProvider,
				proxy:     proxyProvider,
			}

			store, err := memory.NewStore(":memory:")
			if err != nil {
				t.Fatalf("create memory store: %v", err)
			}
			defer store.Close()

			handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
			defer handler.Close()
			handler.SetClaudeCodeHandler(ccHandler)
			handler.SetRuntimeProvider(runtimeProvider)

			conv, err := store.CreateConversation(context.Background(), "Chat Runtime E2E")
			if err != nil {
				t.Fatalf("create conversation: %v", err)
			}

			body := runStreamTurn(t, handler, conv.ID, `{"message":"ping","model":"auto","max_tokens":32}`)
			if strings.Contains(body, `"error":"STREAM_ERROR"`) {
				t.Fatalf("stream failed unexpectedly: %s", body)
			}
			if !strings.Contains(body, `"done":true`) {
				t.Fatalf("expected done marker in stream body, got: %s", body)
			}

			invocations := readChatRuntimeCLIInvocations(t, logPath)
			if len(invocations) != tt.wantCLIInvocations {
				t.Fatalf("fake CC CLI invocations = %d, want %d (%+v), body=%s", len(invocations), tt.wantCLIInvocations, invocations, body)
			}
			if tt.wantCLIInvocations > 0 {
				if invocations[0].Model != "fake-cc-default" {
					t.Fatalf("fake CC CLI model = %q, want fake-cc-default", invocations[0].Model)
				}
				if !strings.Contains(invocations[0].BaseURL, proxyServer.URL) {
					t.Fatalf("fake CC CLI base_url = %q, want %q", invocations[0].BaseURL, proxyServer.URL)
				}
				if !invocations[0].AuthPresent {
					t.Fatal("expected fake CC CLI to receive auth token for local proxy")
				}
			}

			requests := proxyCapture.Snapshot()
			if len(requests) == 0 {
				t.Fatalf("expected local proxy to receive at least one request, body=%s", body)
			}
			last := requests[len(requests)-1]
			if last.Path != "/v1/chat/completions" {
				t.Fatalf("proxy path = %q, want /v1/chat/completions", last.Path)
			}
			if last.Model != tt.wantProxyModel {
				t.Fatalf("proxy model = %q, want %q", last.Model, tt.wantProxyModel)
			}
			if last.Stream != tt.wantProxyStream {
				t.Fatalf("proxy stream = %v, want %v", last.Stream, tt.wantProxyStream)
			}
			if tt.ccEnabled && !strings.HasPrefix(last.Authorization, "Bearer ") {
				t.Fatalf("expected CLI->proxy request to carry bearer auth, got %q", last.Authorization)
			}
			if !tt.ccEnabled && last.Authorization != "" {
				t.Fatalf("expected direct proxy bridge request to omit bearer auth, got %q", last.Authorization)
			}
		})
	}
}
