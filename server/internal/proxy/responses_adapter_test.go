package proxy

import (
	gojson "encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tidwall/gjson"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

func TestConvertOpenAIChatCompletionsToResponses(t *testing.T) {
	body := []byte(`{
		"model":"o3",
		"stream":true,
		"max_tokens":77,
		"previous_response_id":"resp_prev_123",
		"temperature":0.3,
		"messages":[
			{"role":"system","content":"system prompt"},
			{"role":"user","content":"hello"},
			{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"exec","arguments":"{\"cmd\":\"ls\"}"}}]},
			{"role":"tool","tool_call_id":"call_1","content":{"ok":true}},
			{"role":"user","content":[{"type":"text","text":"next"}]}
		],
		"tools":[
			{"type":"function","function":{"name":"exec","description":"run command","parameters":{"type":"object","properties":{"cmd":{"type":"string"}}}}}
		]
	}`)

	converted, err := convertOpenAIChatCompletionsToResponses(body)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if got := gjson.GetBytes(converted, "instructions").String(); got != "system prompt" {
		t.Fatalf("instructions = %q, want %q", got, "system prompt")
	}
	if got := gjson.GetBytes(converted, "input.0.role").String(); got != "user" {
		t.Fatalf("input.0.role = %q, want %q", got, "user")
	}
	if got := gjson.GetBytes(converted, "input.1.type").String(); got != "function_call" {
		t.Fatalf("input.1.type = %q, want %q", got, "function_call")
	}
	if got := gjson.GetBytes(converted, "input.1.call_id").String(); got != "call_1" {
		t.Fatalf("input.1.call_id = %q, want %q", got, "call_1")
	}
	if got := gjson.GetBytes(converted, "input.1.name").String(); got != "exec" {
		t.Fatalf("input.1.name = %q, want %q", got, "exec")
	}
	if got := gjson.GetBytes(converted, "input.1.arguments").String(); got != `{"cmd":"ls"}` {
		t.Fatalf("input.1.arguments = %q, want %q", got, `{"cmd":"ls"}`)
	}
	if got := gjson.GetBytes(converted, "input.2.type").String(); got != "function_call_output" {
		t.Fatalf("input.2.type = %q, want %q", got, "function_call_output")
	}
	if got := gjson.GetBytes(converted, "input.2.call_id").String(); got != "call_1" {
		t.Fatalf("input.2.call_id = %q, want %q", got, "call_1")
	}
	if got := gjson.GetBytes(converted, "input.2.output").String(); got != `{"ok":true}` {
		t.Fatalf("input.2.output = %q, want %q", got, `{"ok":true}`)
	}
	if got := gjson.GetBytes(converted, "tools.0.type").String(); got != "function" {
		t.Fatalf("tools.0.type = %q, want %q", got, "function")
	}
	if got := gjson.GetBytes(converted, "tools.0.name").String(); got != "exec" {
		t.Fatalf("tools.0.name = %q, want %q", got, "exec")
	}
	if got := gjson.GetBytes(converted, "max_output_tokens").Int(); got != 77 {
		t.Fatalf("max_output_tokens = %d, want 77", got)
	}
	if got := gjson.GetBytes(converted, "stream").Bool(); !got {
		t.Fatal("stream = false, want true")
	}
	if got := gjson.GetBytes(converted, "previous_response_id").String(); got != "resp_prev_123" {
		t.Fatalf("previous_response_id = %q, want %q", got, "resp_prev_123")
	}
}

func TestBuildUpstreamRequestWithFormat_ResponsesEndpointConvertsBody(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "p1",
			BaseURL:   "https://chatgpt.com/backend-api/codex/responses",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := []byte(`{"model":"o3","messages":[{"role":"user","content":"hi"}],"max_tokens":5}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatOpenAI)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	if upstreamReq.URL.Path != "/backend-api/codex/responses" {
		t.Fatalf("upstream path = %q, want %q", upstreamReq.URL.Path, "/backend-api/codex/responses")
	}

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if !gjson.GetBytes(convertedBody, "input").Exists() {
		t.Fatalf("converted body missing input: %s", string(convertedBody))
	}
	if gjson.GetBytes(convertedBody, "messages").Exists() {
		t.Fatalf("converted body still contains messages: %s", string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "max_output_tokens").Int(); got != 5 {
		t.Fatalf("max_output_tokens = %d, want 5", got)
	}
}

func TestBuildUpstreamRequestWithFormat_AnthropicEndpointConvertsBody(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "minimax-like",
			BaseURL:   "https://api.minimaxi.com/anthropic",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := []byte(`{"model":"MiniMax-M1","messages":[{"role":"system","content":"You are helpful."},{"role":"user","content":"hi"}],"max_tokens":16}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatOpenAI)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	if upstreamReq.URL.Path != "/anthropic/v1/messages" {
		t.Fatalf("upstream path = %q, want %q", upstreamReq.URL.Path, "/anthropic/v1/messages")
	}

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	var anthropicReq AnthropicRequest
	if err := gojson.Unmarshal(convertedBody, &anthropicReq); err != nil {
		t.Fatalf("converted body is not anthropic request: %v, body=%s", err, string(convertedBody))
	}
	if anthropicReq.System == nil {
		t.Fatalf("expected system prompt in anthropic request, got nil")
	}
	if len(anthropicReq.Messages) != 1 {
		t.Fatalf("anthropic messages len = %d, want 1", len(anthropicReq.Messages))
	}
}

func TestBuildUpstreamRequestWithFormat_CodexModelUsesResponsesEndpoint(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := []byte(`{"model":"gpt-5.3-codex-spark","messages":[{"role":"user","content":"hi"}],"max_tokens":9}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatOpenAI)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	if upstreamReq.URL.Path != "/v1/responses" {
		t.Fatalf("upstream path = %q, want %q", upstreamReq.URL.Path, "/v1/responses")
	}

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if !gjson.GetBytes(convertedBody, "input").Exists() {
		t.Fatalf("converted body missing input: %s", string(convertedBody))
	}
	if gjson.GetBytes(convertedBody, "messages").Exists() {
		t.Fatalf("converted body still contains messages: %s", string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "max_output_tokens").Int(); got != 9 {
		t.Fatalf("max_output_tokens = %d, want 9", got)
	}
}

func TestBuildUpstreamRequestWithFormat_ResponsesPathConvertsMessagesEvenWhenFormatIsResponses(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body := []byte(`{"model":"gpt-5.3-codex-spark","messages":[{"role":"user","content":"hi"}],"max_tokens":9}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	if upstreamReq.URL.Path != "/v1/responses" {
		t.Fatalf("upstream path = %q, want %q", upstreamReq.URL.Path, "/v1/responses")
	}

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if !gjson.GetBytes(convertedBody, "input").Exists() {
		t.Fatalf("converted body missing input: %s", string(convertedBody))
	}
	if gjson.GetBytes(convertedBody, "messages").Exists() {
		t.Fatalf("converted body still contains messages: %s", string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "max_output_tokens").Int(); got != 9 {
		t.Fatalf("max_output_tokens = %d, want 9", got)
	}
}

func TestConvertResponsesToOpenAIChatCompletions(t *testing.T) {
	body := []byte(`{
		"id":"resp_123",
		"object":"response",
		"created_at":1730000000,
		"model":"gpt-5.3-codex",
		"output":[
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello "} ,{"type":"output_text","text":"world"}]},
			{"type":"function_call","id":"fc_1","call_id":"call_1","name":"web_search","arguments":"{\"query\":\"OpenClaw\"}"}
		],
		"usage":{"input_tokens":12,"output_tokens":5,"total_tokens":17}
	}`)

	converted, err := convertResponsesToOpenAIChatCompletions(body)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if got := gjson.GetBytes(converted, "object").String(); got != "chat.completion" {
		t.Fatalf("object = %q, want %q", got, "chat.completion")
	}
	if got := gjson.GetBytes(converted, "choices.0.message.content").String(); got != "hello world" {
		t.Fatalf("content = %q, want %q", got, "hello world")
	}
	if got := gjson.GetBytes(converted, "choices.0.message.tool_calls.0.id").String(); got != "call_1" {
		t.Fatalf("tool call id = %q, want %q", got, "call_1")
	}
	if got := gjson.GetBytes(converted, "choices.0.message.tool_calls.0.function.name").String(); got != "web_search" {
		t.Fatalf("tool call function name = %q, want %q", got, "web_search")
	}
	if got := gjson.GetBytes(converted, "choices.0.message.tool_calls.0.function.arguments").String(); got != `{"query":"OpenClaw"}` {
		t.Fatalf("tool call arguments = %q, want %q", got, `{"query":"OpenClaw"}`)
	}
	if got := gjson.GetBytes(converted, "choices.0.finish_reason").String(); got != "tool_calls" {
		t.Fatalf("finish_reason = %q, want %q", got, "tool_calls")
	}
	if got := gjson.GetBytes(converted, "usage.prompt_tokens").Int(); got != 12 {
		t.Fatalf("usage.prompt_tokens = %d, want 12", got)
	}
}
