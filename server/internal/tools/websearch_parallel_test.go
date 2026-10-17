package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type parallelTestTransport func(*http.Request) (*http.Response, error)

func (f parallelTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type parallelFixture struct {
	t             *testing.T
	mu            sync.Mutex
	methods       []string
	sessions      []string
	callResult    interface{}
	failure       string
	sse           bool
	authorization string
}

func (f *parallelFixture) roundTrip(r *http.Request) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.String() != "https://search.parallel.ai/mcp" {
		f.t.Errorf("destination = %s", r.URL)
	}
	if r.Method != "POST" {
		f.t.Errorf("method = %s", r.Method)
	}
	if !strings.Contains(r.UserAgent(), "ZimaOS-Blue") {
		f.t.Errorf("User-Agent = %q", r.UserAgent())
	}
	if r.Header.Get("Authorization") != f.authorization {
		f.t.Errorf("unexpected authorization header")
	}
	if r.Header.Get("Accept") != "application/json, text/event-stream" {
		f.t.Errorf("Accept = %q", r.Header.Get("Accept"))
	}
	var req struct {
		ID     int             `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	f.methods = append(f.methods, req.Method)
	h := make(http.Header)
	h.Set("Content-Type", "application/json")
	response := map[string]interface{}{"jsonrpc": "2.0", "id": req.ID}
	switch req.Method {
	case "initialize":
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
			ClientInfo      struct {
				Name string `json:"name"`
			} `json:"clientInfo"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, err
		}
		if params.ClientInfo.Name != "ZimaOS-Blue" {
			f.t.Errorf("client name = %q", params.ClientInfo.Name)
		}
		version := params.ProtocolVersion
		if f.failure == "protocol" {
			version = "unsupported-version"
		}
		response["result"] = map[string]interface{}{"protocolVersion": version, "capabilities": map[string]interface{}{"tools": map[string]interface{}{}}}
		h.Set("Mcp-Session-Id", "fixture-session")
	case "notifications/initialized":
		if r.Header.Get("Mcp-Session-Id") != "fixture-session" {
			f.t.Error("missing session on initialized notification")
		}
		return &http.Response{StatusCode: 202, Header: h, Body: io.NopCloser(strings.NewReader(""))}, nil
	case "tools/list":
		if r.Header.Get("Mcp-Session-Id") != "fixture-session" || r.Header.Get("MCP-Protocol-Version") == "" {
			f.t.Error("missing negotiated session/protocol")
		}
		name := "web_search"
		if f.failure == "missing-tool" {
			name = "other"
		}
		response["result"] = map[string]interface{}{"tools": []interface{}{map[string]interface{}{"name": name}}}
	case "tools/call":
		var p struct {
			Name      string `json:"name"`
			Arguments struct {
				Objective     string   `json:"objective"`
				SearchQueries []string `json:"search_queries"`
				SessionID     string   `json:"session_id"`
			} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, err
		}
		if p.Name != "web_search" || p.Arguments.Objective == "" || len(p.Arguments.SearchQueries) != 1 {
			f.t.Errorf("invalid search call %s", req.Params)
		}
		if len(p.Arguments.SessionID) < 32 || len(p.Arguments.SessionID) > 100 {
			f.t.Errorf("invalid stable session %q", p.Arguments.SessionID)
		}
		f.sessions = append(f.sessions, p.Arguments.SessionID)
		switch f.failure {
		case "quota":
			return &http.Response{StatusCode: 429, Header: h, Body: io.NopCloser(strings.NewReader(`{"error":"quota reached"}`))}, nil
		case "rpc":
			response["error"] = map[string]interface{}{"code": -32603, "message": "failed"}
		case "tool":
			response["result"] = map[string]interface{}{"isError": true, "content": []interface{}{map[string]interface{}{"type": "text", "text": "failed"}}}
		case "malformed":
			return &http.Response{StatusCode: 200, Header: h, Body: io.NopCloser(strings.NewReader("not json"))}, nil
		case "id":
			response["id"] = 99
			response["result"] = f.callResult
		default:
			response["result"] = f.callResult
		}
	default:
		return nil, fmt.Errorf("unexpected method %q", req.Method)
	}
	raw, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	if f.sse {
		h.Set("Content-Type", "text/event-stream")
		raw = []byte(": heartbeat\n\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/message\",\"params\":{}}\n\nevent: message\ndata: " + string(raw) + "\n\n")
	}
	return &http.Response{StatusCode: 200, Header: h, Body: io.NopCloser(strings.NewReader(string(raw)))}, nil
}

func parallelFixtureResults() map[string]interface{} {
	return map[string]interface{}{"results": []interface{}{
		map[string]interface{}{"title": "Blue docs", "url": "https://docs.example.com/blue?a=1&b=2", "excerpts": []string{"Focused source excerpt", "More evidence"}},
		map[string]interface{}{"title": "Unsafe", "url": "javascript:alert(1)", "excerpts": []string{"unsafe"}},
		map[string]interface{}{"title": "Other", "url": "https://example.com/other", "excerpts": []string{"Other source"}},
	}}
}

func TestParallelSearchThroughDispatcher(t *testing.T) {
	for _, sse := range []bool{false, true} {
		t.Run(fmt.Sprint(sse), func(t *testing.T) {
			t.Setenv("PARALLEL_API_KEY", "ambient-key-must-not-be-used")
			payload := parallelFixtureResults()
			result := map[string]interface{}{"structuredContent": payload}
			if sse {
				raw, _ := json.Marshal(payload)
				result = map[string]interface{}{"content": []interface{}{map[string]interface{}{"type": "text", "text": string(raw)}}}
			}
			f := &parallelFixture{t: t, sse: sse, callResult: result}
			search := NewWebSearchTool(WebSearchConfig{Provider: "parallel", APIKey: "another-provider-key", BaseURL: "https://other-provider.invalid", CacheTTL: -1})
			search.SetHTTPClientForTest(&http.Client{Transport: parallelTestTransport(f.roundTrip)})
			raw, err := search.Execute(WithSessionID(context.Background(), "conversation-one"), map[string]interface{}{"query": "Blue docs", "max_results": 1, "format": "json"})
			if err != nil {
				t.Fatal(err)
			}
			var response WebSearchResponse
			if err := json.Unmarshal([]byte(raw.(string)), &response); err != nil {
				t.Fatal(err)
			}
			if response.Provider != "parallel" || len(response.Results) != 1 || response.Results[0].URL != "https://docs.example.com/blue?a=1&b=2" || !strings.Contains(response.Results[0].Description, "Focused source excerpt") {
				t.Fatalf("response = %+v", response)
			}
			if !reflect.DeepEqual(f.methods, []string{"initialize", "notifications/initialized", "tools/list", "tools/call"}) {
				t.Fatalf("methods = %#v", f.methods)
			}
			// The normal web_query path must consume the provider output and retain sources.
			reader := &scriptedWebTool{exec: func(args map[string]interface{}) (interface{}, error) {
				return `{"url":"https://docs.example.com/blue?a=1&b=2","final_url":"https://docs.example.com/blue?a=1&b=2","title":"Blue docs","content":"Blue is a local-first agent runtime with a maintained web-search dispatcher. This source contains enough concrete detail to be useful for answering a documentation question and confirms that search results can be consumed by the normal reader.","format":"text","source":"http"}`, nil
			}}
			web := NewWebQueryTool(search, nil, reader, nil, nil)
			raw, err = web.Execute(WithSessionID(context.Background(), "conversation-one"), map[string]interface{}{"input": "Blue docs", "depth": "quick", "max_results": 1})
			if err != nil {
				t.Fatal(err)
			}
			var envelope webQueryEnvelope
			if err := json.Unmarshal([]byte(raw.(string)), &envelope); err != nil {
				t.Fatal(err)
			}
			if len(envelope.Sources) != 1 || envelope.Provider != "parallel" || !strings.Contains(envelope.Sources[0].Snippet, "Focused source excerpt") || envelope.TargetURL != "https://docs.example.com/blue?a=1&b=2" {
				t.Fatalf("envelope = %+v", envelope)
			}
			if len(f.sessions) != 2 || f.sessions[0] != f.sessions[1] {
				t.Fatalf("session not stable: %#v", f.sessions)
			}
		})
	}
}

func TestParallelSearchErrors(t *testing.T) {
	for _, failure := range []string{"quota", "rpc", "tool", "malformed", "id", "protocol", "missing-tool"} {
		t.Run(failure, func(t *testing.T) {
			f := &parallelFixture{t: t, failure: failure, callResult: map[string]interface{}{"structuredContent": parallelFixtureResults()}}
			search := NewWebSearchTool(WebSearchConfig{Provider: "parallel", CacheTTL: -1})
			search.SetHTTPClientForTest(&http.Client{Transport: parallelTestTransport(f.roundTrip)})
			_, err := search.Execute(context.Background(), map[string]interface{}{"query": "Blue docs"})
			if err == nil {
				t.Fatal("expected error")
			}
			if len(f.sessions) > 1 {
				t.Fatal("unexpected quota/error retries")
			}
		})
	}
}

func TestParallelSearchCancellationAndTimeout(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelled), func(t *testing.T) {
			search := NewWebSearchTool(WebSearchConfig{Provider: "parallel", CacheTTL: -1, Timeout: 20 * time.Millisecond})
			search.SetHTTPClientForTest(&http.Client{Transport: parallelTestTransport(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if cancelled {
				cancel()
			}
			_, err := search.Execute(ctx, map[string]interface{}{"query": "Blue docs"})
			want := context.DeadlineExceeded
			if cancelled {
				want = context.Canceled
			}
			if !errors.Is(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
		})
	}
}

func TestParallelSearchPreservesChoices(t *testing.T) {
	search := NewWebSearchTool(WebSearchConfig{})
	if !reflect.DeepEqual(search.providerChain(nil), []string{"duckduckgo", "bing"}) {
		t.Fatal("changed defaults")
	}
	search = NewWebSearchTool(WebSearchConfig{Providers: []string{"brave"}, APIKey: "saved-key"})
	if !reflect.DeepEqual(search.providerChain(nil), []string{"brave"}) || search.apiKeyForProvider("brave") != "saved-key" {
		t.Fatal("changed saved choice")
	}
	if !reflect.DeepEqual(search.providerChain("parallel"), []string{"parallel"}) {
		t.Fatal("lost explicit override")
	}
	disabled := false
	search = NewWebSearchTool(WebSearchConfig{Provider: "parallel", ProviderSettings: map[string]WebSearchProviderSetting{"parallel": {Enabled: &disabled}}})
	search.SetHTTPClientForTest(&http.Client{Transport: parallelTestTransport(func(r *http.Request) (*http.Response, error) {
		t.Error("disabled provider made request")
		return nil, errors.New("unexpected request")
	})})
	if _, err := search.Execute(context.Background(), map[string]interface{}{"query": "Blue docs"}); err == nil {
		t.Fatal("disabled search should fail")
	}
}

func TestParallelSearchProviderKeyAndStableFallbackSession(t *testing.T) {
	f := &parallelFixture{t: t, authorization: "Bearer parallel-key", callResult: map[string]interface{}{"structuredContent": parallelFixtureResults()}}
	search := NewWebSearchTool(WebSearchConfig{Provider: "parallel", CacheTTL: -1, ProviderSettings: map[string]WebSearchProviderSetting{"parallel": {APIKey: "parallel-key"}}})
	search.SetHTTPClientForTest(&http.Client{Transport: parallelTestTransport(f.roundTrip)})
	for i := 0; i < 2; i++ {
		if _, err := search.Execute(context.Background(), map[string]interface{}{"query": "Blue docs"}); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.sessions) != 2 || f.sessions[0] != f.sessions[1] {
		t.Fatalf("unstable fallback session: %#v", f.sessions)
	}
}

func TestParallelSearchRejectsMissingResults(t *testing.T) {
	f := &parallelFixture{t: t, callResult: map[string]interface{}{"structuredContent": map[string]interface{}{}}}
	search := NewWebSearchTool(WebSearchConfig{Provider: "parallel", CacheTTL: -1})
	search.SetHTTPClientForTest(&http.Client{Transport: parallelTestTransport(f.roundTrip)})
	if _, err := search.Execute(context.Background(), map[string]interface{}{"query": "Blue docs"}); err == nil {
		t.Fatal("missing results should be a schema error")
	}
}

func TestParallelSearchHonorsGatewayPermissions(t *testing.T) {
	search := NewWebSearchTool(WebSearchConfig{Provider: "parallel"})
	search.SetHTTPClientForTest(&http.Client{Transport: parallelTestTransport(func(r *http.Request) (*http.Response, error) {
		t.Error("denied tool made network request")
		return nil, errors.New("unexpected network")
	})})
	registry := NewRegistry()
	registry.Register(NewWebQueryTool(search, nil, nil, nil, nil))
	gateway := NewToolGateway(registry, NewExecutor(registry))
	approver := &approvalStub{decision: ToolApprovalDecision{Allowed: false, Approval: ToolApprovalEnvelope{Required: true, Mode: "deny"}}}
	gateway.SetApprover(approver)
	if _, err := gateway.Execute(context.Background(), ToolGatewayRequest{ToolName: "web_query", Arguments: `{"action":"search","query":"Blue docs"}`, RouteKind: ToolRouteKindChat}); err == nil {
		t.Fatal("permission denial should fail")
	}
	if approver.calls != 1 {
		t.Fatal("approval policy was bypassed")
	}
	registry.Disable("web_query")
	if _, err := gateway.Execute(context.Background(), ToolGatewayRequest{ToolName: "web_query", Arguments: `{"action":"search","query":"Blue docs"}`, RouteKind: ToolRouteKindChat}); err == nil {
		t.Fatal("disabled tool should fail")
	}
}

// This opt-in check uses the actual maintained dispatcher with no saved credentials.
func TestParallelSearchLive(t *testing.T) {
	if os.Getenv("BLUE_PARALLEL_LIVE") != "1" {
		t.Skip("set BLUE_PARALLEL_LIVE=1 for anonymous MCP validation")
	}
	quota := false
	search := NewWebSearchTool(WebSearchConfig{Provider: "parallel", CacheTTL: -1, Timeout: 30 * time.Second})
	search.SetHTTPClientForTest(&http.Client{Transport: parallelTestTransport(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://search.parallel.ai/mcp" || !strings.Contains(req.UserAgent(), "ZimaOS-Blue") || req.Header.Get("Authorization") != "" {
			t.Error("unexpected destination, User-Agent or credentials")
		}
		resp, err := http.DefaultTransport.RoundTrip(req)
		if err == nil && resp.StatusCode == 429 {
			body, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				return nil, readErr
			}
			resp.Body = io.NopCloser(strings.NewReader(string(body)))
			quota = true
			t.Logf("Anonymous MCP quota at %s: HTTP %d Retry-After=%q body=%s", time.Now().UTC().Format(time.RFC3339), resp.StatusCode, resp.Header.Get("Retry-After"), body)
		}
		return resp, err
	})})
	registry := NewRegistry()
	registry.Register(NewWebQueryTool(search, nil, nil, nil, nil))
	gateway := NewToolGateway(registry, NewExecutor(registry))
	result, err := gateway.Execute(WithSessionID(context.Background(), "blue-parallel-live-validation"), ToolGatewayRequest{ToolName: "web_query", Arguments: `{"action":"search","query":"Go context cancellation documentation","provider":"parallel","max_results":2,"format":"json"}`, RouteKind: ToolRouteKindChat})
	if err != nil {
		if quota {
			t.Skipf("live search was quota limited: %v", err)
		}
		t.Fatal(err)
	}
	var response WebSearchResponse
	if err := decodeToolJSONResult(result.ExecutionResult, &response); err != nil {
		t.Fatal(err)
	}
	if response.Provider != "parallel" || len(response.Results) == 0 || response.Results[0].Description == "" {
		t.Fatalf("no useful source results: %+v", response)
	}
	t.Logf("Anonymous runtime search returned %d useful results; first=%s", len(response.Results), response.Results[0].URL)
}
