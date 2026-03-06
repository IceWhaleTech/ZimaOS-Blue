package proxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/tidwall/gjson"
)

func TestCopyResponse_CloudCode_NonStreamingConvertedToOpenAI(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)

	body := `{"candidates":[{"content":{"parts":[{"text":"hello from cloudcode"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":2,"totalTokenCount":5}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	rec := httptest.NewRecorder()
	pr := &parsedRequest{upstreamFormat: ProviderTypeCloudCode}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ph.copyResponse(rec, resp, pr, req)

	gotBody := rec.Body.Bytes()
	if got := gjson.GetBytes(gotBody, "choices.0.message.content").String(); got != "hello from cloudcode" {
		t.Fatalf("converted content = %q, want %q, body=%s", got, "hello from cloudcode", string(gotBody))
	}
	if got := gjson.GetBytes(gotBody, "usage.total_tokens").Int(); got != 5 {
		t.Fatalf("converted usage.total_tokens = %d, want 5, body=%s", got, string(gotBody))
	}
}

func TestCopyResponse_CloudCode_StreamingConvertedToOpenAISSE(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)

	sse := "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"hello\"}]}}]}\n\n" +
		"data: [DONE]\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(sse)),
	}

	rec := httptest.NewRecorder()
	pr := &parsedRequest{
		upstreamFormat: ProviderTypeCloudCode,
		streaming:      true,
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ph.copyResponse(rec, resp, pr, req)

	got := rec.Body.String()
	if !strings.Contains(got, "chat.completion.chunk") {
		t.Fatalf("expected converted OpenAI SSE chunk, got: %s", got)
	}
	if !strings.Contains(got, "data: [DONE]") {
		t.Fatalf("expected DONE marker, got: %s", got)
	}
}

func TestCopyResponse_OpenAIResponsesEndpoint_NonStreamingConvertedToChatCompletions(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)

	body := `{
		"id":"resp_abc",
		"object":"response",
		"created_at":1730000010,
		"model":"gpt-5.3-codex",
		"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"normalized output"}]}],
		"usage":{"input_tokens":7,"output_tokens":3,"total_tokens":10}
	}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    httptest.NewRequest(http.MethodPost, "https://relay.example.com/v1/responses", nil),
	}

	rec := httptest.NewRecorder()
	pr := &parsedRequest{}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ph.copyResponse(rec, resp, pr, req)

	gotBody := rec.Body.Bytes()
	if got := gjson.GetBytes(gotBody, "object").String(); got != "chat.completion" {
		t.Fatalf("object = %q, want %q; body=%s", got, "chat.completion", string(gotBody))
	}
	if got := gjson.GetBytes(gotBody, "choices.0.message.content").String(); got != "normalized output" {
		t.Fatalf("content = %q, want %q; body=%s", got, "normalized output", string(gotBody))
	}
	if got := gjson.GetBytes(gotBody, "usage.total_tokens").Int(); got != 10 {
		t.Fatalf("usage.total_tokens = %d, want 10; body=%s", got, string(gotBody))
	}
}

func TestCopyResponse_OpenAIResponsesEndpoint_StreamingConvertedToChatCompletionsSSE(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)

	sse := "event: response.created\n" +
		"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_stream_1\",\"model\":\"gpt-5.3-codex\",\"status\":\"in_progress\"}}\n\n" +
		"event: response.output_text.delta\n" +
		"data: {\"type\":\"response.output_text.delta\",\"response_id\":\"resp_stream_1\",\"delta\":\"hello \"}\n\n" +
		"event: response.output_text.delta\n" +
		"data: {\"type\":\"response.output_text.delta\",\"response_id\":\"resp_stream_1\",\"delta\":\"world\"}\n\n" +
		"event: response.completed\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_stream_1\",\"model\":\"gpt-5.3-codex\",\"status\":\"completed\",\"usage\":{\"input_tokens\":7,\"output_tokens\":3,\"total_tokens\":10},\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"hello world\"}]}]}}\n\n"

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(sse)),
		Request:    httptest.NewRequest(http.MethodPost, "https://relay.example.com/v1/responses", nil),
	}

	rec := httptest.NewRecorder()
	pr := &parsedRequest{streaming: true}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req = req.WithContext(WithSessionID(context.Background(), "sess-copy-stream-responses-1"))
	ph.copyResponse(rec, resp, pr, req)

	got := rec.Body.String()
	if !strings.Contains(got, "chat.completion.chunk") {
		t.Fatalf("expected OpenAI SSE chunk, got: %s", got)
	}
	if strings.Contains(got, "response.output_text.delta") {
		t.Fatalf("responses SSE event should be converted, got raw payload: %s", got)
	}
	if !strings.Contains(got, "\"content\":\"hello \"") || !strings.Contains(got, "\"content\":\"world\"") {
		t.Fatalf("expected converted text deltas, got: %s", got)
	}
	if !strings.Contains(got, "\"finish_reason\":\"stop\"") {
		t.Fatalf("expected finish_reason stop in final chunk, got: %s", got)
	}
	if !strings.Contains(got, "\"prompt_tokens\":7") || !strings.Contains(got, "\"completion_tokens\":3") {
		t.Fatalf("expected usage in final chunk, got: %s", got)
	}
	if !strings.Contains(got, "data: [DONE]") {
		t.Fatalf("expected DONE marker, got: %s", got)
	}
	if gotPrev := rec.Header().Get(ResponsesPreviousIDHeader); gotPrev != "resp_stream_1" {
		t.Fatalf("%s = %q, want %q", ResponsesPreviousIDHeader, gotPrev, "resp_stream_1")
	}
}

func TestCopyResponse_OpenAIResponsesEndpoint_StreamingToolArgumentsNotDuplicated(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)

	sse := "event: response.created\n" +
		"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_stream_tool_1\",\"model\":\"gpt-5.3-codex\",\"status\":\"in_progress\"}}\n\n" +
		"event: response.output_item.added\n" +
		"data: {\"type\":\"response.output_item.added\",\"response_id\":\"resp_stream_tool_1\",\"output_index\":0,\"item\":{\"type\":\"function_call\",\"id\":\"fc_1\",\"call_id\":\"call_1\",\"name\":\"exec\",\"arguments\":\"\"}}\n\n" +
		"event: response.function_call_arguments.delta\n" +
		"data: {\"type\":\"response.function_call_arguments.delta\",\"response_id\":\"resp_stream_tool_1\",\"output_index\":0,\"call_id\":\"call_1\",\"delta\":\"{\\\"cmd\\\":\\\"ls\\\"}\"}\n\n" +
		"event: response.function_call_arguments.done\n" +
		"data: {\"type\":\"response.function_call_arguments.done\",\"response_id\":\"resp_stream_tool_1\",\"output_index\":0,\"call_id\":\"call_1\",\"arguments\":\"{\\\"cmd\\\":\\\"ls\\\"}\"}\n\n" +
		"event: response.output_item.done\n" +
		"data: {\"type\":\"response.output_item.done\",\"response_id\":\"resp_stream_tool_1\",\"output_index\":0,\"item\":{\"type\":\"function_call\",\"id\":\"fc_1\",\"call_id\":\"call_1\",\"name\":\"exec\",\"arguments\":\"{\\\"cmd\\\":\\\"ls\\\"}\"}}\n\n" +
		"event: response.completed\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_stream_tool_1\",\"model\":\"gpt-5.3-codex\",\"status\":\"completed\",\"output\":[{\"type\":\"function_call\",\"call_id\":\"call_1\",\"name\":\"exec\",\"arguments\":\"{\\\"cmd\\\":\\\"ls\\\"}\"}]}}\n\n"

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(sse)),
		Request:    httptest.NewRequest(http.MethodPost, "https://relay.example.com/v1/responses", nil),
	}

	rec := httptest.NewRecorder()
	pr := &parsedRequest{streaming: true}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req = req.WithContext(WithSessionID(context.Background(), "sess-copy-stream-responses-tool-1"))
	ph.copyResponse(rec, resp, pr, req)

	got := rec.Body.String()
	if !strings.Contains(got, "chat.completion.chunk") {
		t.Fatalf("expected OpenAI SSE chunk, got: %s", got)
	}
	if count := strings.Count(got, "\"arguments\":\"{\\\"cmd\\\":\\\"ls\\\"}\""); count != 1 {
		t.Fatalf("tool arguments emitted %d times, want 1; body=%s", count, got)
	}
	if !strings.Contains(got, "data: [DONE]") {
		t.Fatalf("expected DONE marker, got: %s", got)
	}
}

func TestCopyResponse_OpenAIResponsesEndpoint_StreamingToolArgumentsNotDuplicated_ItemIDFallback(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)

	sse := "event: response.created\n" +
		"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_stream_tool_2\",\"model\":\"gpt-5.3-codex\",\"status\":\"in_progress\"}}\n\n" +
		"event: response.output_item.added\n" +
		"data: {\"type\":\"response.output_item.added\",\"response_id\":\"resp_stream_tool_2\",\"output_index\":0,\"item\":{\"type\":\"function_call\",\"id\":\"fc_2\",\"name\":\"exec\",\"arguments\":\"\"}}\n\n" +
		"event: response.function_call_arguments.delta\n" +
		"data: {\"type\":\"response.function_call_arguments.delta\",\"response_id\":\"resp_stream_tool_2\",\"output_index\":0,\"item_id\":\"fc_2\",\"delta\":\"{\\\"cmd\\\":\\\"pwd\\\"}\"}\n\n" +
		"event: response.output_item.done\n" +
		"data: {\"type\":\"response.output_item.done\",\"response_id\":\"resp_stream_tool_2\",\"output_index\":0,\"item\":{\"type\":\"function_call\",\"id\":\"fc_2\",\"name\":\"exec\",\"arguments\":\"{\\\"cmd\\\":\\\"pwd\\\"}\"}}\n\n" +
		"event: response.completed\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_stream_tool_2\",\"model\":\"gpt-5.3-codex\",\"status\":\"completed\",\"output\":[{\"type\":\"function_call\",\"id\":\"fc_2\",\"name\":\"exec\",\"arguments\":\"{\\\"cmd\\\":\\\"pwd\\\"}\"}]}}\n\n"

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(sse)),
		Request:    httptest.NewRequest(http.MethodPost, "https://relay.example.com/v1/responses", nil),
	}

	rec := httptest.NewRecorder()
	pr := &parsedRequest{streaming: true}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req = req.WithContext(WithSessionID(context.Background(), "sess-copy-stream-responses-tool-2"))
	ph.copyResponse(rec, resp, pr, req)

	got := rec.Body.String()
	if count := strings.Count(got, "\"arguments\":\"{\\\"cmd\\\":\\\"pwd\\\"}\""); count != 1 {
		t.Fatalf("tool arguments emitted %d times, want 1; body=%s", count, got)
	}
	if !strings.Contains(got, "\"id\":\"fc_2\"") {
		t.Fatalf("expected tool id from item_id fallback, got: %s", got)
	}
	if !strings.Contains(got, "data: [DONE]") {
		t.Fatalf("expected DONE marker, got: %s", got)
	}
}

func TestCopyResponse_OpenAIResponsesEndpoint_StreamingAbruptCloseDoesNotInjectDone(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)

	// No response.completed / [DONE], simulating upstream abrupt stream close.
	sse := "event: response.created\n" +
		"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_stream_cut_1\",\"model\":\"gpt-5.3-codex\",\"status\":\"in_progress\"}}\n\n" +
		"event: response.output_text.delta\n" +
		"data: {\"type\":\"response.output_text.delta\",\"response_id\":\"resp_stream_cut_1\",\"delta\":\"partial\"}\n\n"

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(sse)),
		Request:    httptest.NewRequest(http.MethodPost, "https://relay.example.com/v1/responses", nil),
	}

	rec := httptest.NewRecorder()
	pr := &parsedRequest{streaming: true}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req = req.WithContext(WithSessionID(context.Background(), "sess-copy-stream-responses-cut-1"))
	ph.copyResponse(rec, resp, pr, req)

	got := rec.Body.String()
	if !strings.Contains(got, "\"content\":\"partial\"") {
		t.Fatalf("expected converted partial chunk, got: %s", got)
	}
	if strings.Contains(got, "data: [DONE]") {
		t.Fatalf("unexpected synthetic DONE for abrupt close: %s", got)
	}
}

func TestCopyResponse_OpenAIResponsesEndpoint_InvalidPayloadReturnsBadGateway(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":     []string{"application/json"},
			"Content-Encoding": []string{"br"},
		},
		Body:    io.NopCloser(strings.NewReader("【invalid-non-json-payload】")),
		Request: httptest.NewRequest(http.MethodPost, "https://relay.example.com/v1/responses", nil),
	}

	rec := httptest.NewRecorder()
	pr := &parsedRequest{
		resolvedProviderID: "p-resp-invalid",
		resolvedModel:      "gpt-5.3-codex-spark",
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ph.copyResponse(rec, resp, pr, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("content-encoding = %q, want empty", got)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("content-type = %q, want application/json", got)
	}
	if !strings.Contains(rec.Body.String(), "invalid responses payload from upstream") {
		t.Fatalf("unexpected error body: %s", rec.Body.String())
	}
}

func TestParseUsageTokensFromSSE_ResponsesCompletedNestedUsage(t *testing.T) {
	data := []byte("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"usage\":{\"input_tokens\":11,\"output_tokens\":4,\"total_tokens\":15}}}\n\n")
	in, out := parseUsageTokensFromSSE(data)
	if in != 11 || out != 4 {
		t.Fatalf("parseUsageTokensFromSSE = (%d,%d), want (11,4)", in, out)
	}
}

func TestParseUsageTokensFromSSE_AnthropicSplitUsageAcrossEvents(t *testing.T) {
	data := []byte(strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_1","model":"claude-3"},"usage":{"input_tokens":13}}`,
		"",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`,
		"",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n"))
	in, out := parseUsageTokensFromSSE(data)
	if in != 13 || out != 5 {
		t.Fatalf("parseUsageTokensFromSSE = (%d,%d), want (13,5)", in, out)
	}
}

func TestRecordPromptCacheFromSSE_ResponsesCompletedNestedUsage(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)
	pr := &parsedRequest{
		model:            "gpt-5.3-codex",
		resolvedModel:    "gpt-5.3-codex",
		resolvedProvider: "openai",
	}
	data := []byte("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"usage\":{\"input_tokens\":120,\"output_tokens\":20,\"cache_read_input_tokens\":90,\"cache_creation_input_tokens\":15}}}\n\n")

	ph.recordPromptCacheFromSSE(pr, data)

	stats := ph.GetPromptCacheStats()
	if stats.Requests != 1 {
		t.Fatalf("requests = %d, want 1", stats.Requests)
	}
	if stats.TotalInputTokens != 120 {
		t.Fatalf("total_input_tokens = %d, want 120", stats.TotalInputTokens)
	}
	if stats.TotalCacheReadTokens != 90 {
		t.Fatalf("total_cache_read_tokens = %d, want 90", stats.TotalCacheReadTokens)
	}
	if stats.TotalCacheCreation != 15 {
		t.Fatalf("total_cache_creation_tokens = %d, want 15", stats.TotalCacheCreation)
	}
}

func TestRecordPromptCacheFromSSE_AnthropicSplitUsageFallsBackToEarlierChunk(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)
	pr := &parsedRequest{
		model:            "claude-3",
		resolvedModel:    "claude-3",
		resolvedProvider: "anthropic",
	}
	data := []byte(strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_1","model":"claude-3"},"usage":{"input_tokens":80,"cache_read_input_tokens":40,"cache_creation_input_tokens":8}}`,
		"",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":12}}`,
		"",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n"))

	ph.recordPromptCacheFromSSE(pr, data)

	stats := ph.GetPromptCacheStats()
	if stats.Requests != 1 {
		t.Fatalf("requests = %d, want 1", stats.Requests)
	}
	if stats.TotalInputTokens != 80 {
		t.Fatalf("total_input_tokens = %d, want 80", stats.TotalInputTokens)
	}
	if stats.TotalCacheReadTokens != 40 {
		t.Fatalf("total_cache_read_tokens = %d, want 40", stats.TotalCacheReadTokens)
	}
	if stats.TotalCacheCreation != 8 {
		t.Fatalf("total_cache_creation_tokens = %d, want 8", stats.TotalCacheCreation)
	}
}

func TestCopyResponse_OpenAIResponsesEndpoint_SetsResponsesHeadersAndCachesPrevID(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)

	body := `{
		"id":"resp_cache_1",
		"object":"response",
		"model":"gpt-5.3-codex",
		"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]
	}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    httptest.NewRequest(http.MethodPost, "https://relay.example.com/v1/responses", nil),
	}

	rec := httptest.NewRecorder()
	pr := &parsedRequest{}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req = req.WithContext(WithSessionID(context.Background(), "sess-copy-1"))
	ph.copyResponse(rec, resp, pr, req)

	if got := rec.Header().Get(ResponsesUsedHeader); got != "1" {
		t.Fatalf("%s = %q, want %q", ResponsesUsedHeader, got, "1")
	}
	if got := rec.Header().Get(ResponsesPreviousIDHeader); got != "resp_cache_1" {
		t.Fatalf("%s = %q, want %q", ResponsesPreviousIDHeader, got, "resp_cache_1")
	}
	if got := ph.getCachedResponsesPreviousID(req); got != "resp_cache_1" {
		t.Fatalf("cached previous_response_id = %q, want %q", got, "resp_cache_1")
	}
}

func TestCopyResponse_OpenAIResponsesEndpoint_DisablesContinuationWhenUpstreamDropsPreviousResponseID(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)

	body := `{
		"id":"resp_followup_1",
		"object":"response",
		"model":"gpt-5.3-codex",
		"previous_response_id":null,
		"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"I do not remember previous context"}]}]
	}`
	upstreamReq := httptest.NewRequest(http.MethodPost, "https://relay.example.com/v1/responses", nil)
	upstreamReq = upstreamReq.WithContext(withUpstreamResponsesPreviousID(upstreamReq.Context(), "resp_prev_1"))
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    upstreamReq,
	}

	rec := httptest.NewRecorder()
	pr := &parsedRequest{
		resolvedProviderID: "provider-prev-drop",
		resolvedModel:      "gpt-5.3-codex",
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req = req.WithContext(WithSessionID(context.Background(), "sess-copy-disable-1"))
	ph.setCachedResponsesPreviousID(req, "resp_prev_1")
	ph.copyResponse(rec, resp, pr, req)

	if got := rec.Header().Get(ResponsesContinuationDisabledHeader); got != "1" {
		t.Fatalf("%s = %q, want %q", ResponsesContinuationDisabledHeader, got, "1")
	}
	if got := ph.getCachedResponsesPreviousID(req); got != "" {
		t.Fatalf("cached previous_response_id should be cleared, got %q", got)
	}

	route := &providerpool.RouteResult{
		Provider: &providerpool.Provider{ID: "provider-prev-drop"},
		Model:    &providerpool.Model{ID: "gpt-5.3-codex"},
	}
	if !ph.isResponsesContinuationDisabledForRoute(route) {
		t.Fatal("expected continuation to be disabled for provider route")
	}
}

func TestCopyResponse_RecordsProviderUsageWithoutSession(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "proxy-usage-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := providerpool.NewFileStorage(tmpDir)
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}
	tracker := providerpool.NewUsageTracker(storage)
	tracker.Start()
	defer tracker.Stop()

	ph := NewProxyHandler(nil, nil, nil)
	ph.SetProviderPool(&providerpool.Pool{UsageTracker: tracker})

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"usage":{"prompt_tokens":11,"completion_tokens":7}}`,
		)),
	}

	rec := httptest.NewRecorder()
	pr := &parsedRequest{
		model:              "gpt-4o-mini",
		resolvedModel:      "gpt-4o-mini",
		resolvedProviderID: "openai",
		resolvedProvider:   "OpenAI",
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ph.copyResponse(rec, resp, pr, req)

	time.Sleep(20 * time.Millisecond)

	stats := tracker.GetCurrentStats()
	s, ok := stats["openai:gpt-4o-mini"]
	if !ok {
		t.Fatalf("expected usage stats for openai:gpt-4o-mini, got keys=%v", mapsKeys(stats))
	}
	if s.TotalRequests != 1 {
		t.Fatalf("expected 1 request, got %d", s.TotalRequests)
	}
	if s.TotalInputTokens != 11 || s.TotalOutputTokens != 7 {
		t.Fatalf("unexpected tokens in=%d out=%d", s.TotalInputTokens, s.TotalOutputTokens)
	}
}

func TestCopyResponse_RecordsProviderUsageWithoutSession_StreamingAnthropic(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "proxy-usage-stream-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := providerpool.NewFileStorage(tmpDir)
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}
	tracker := providerpool.NewUsageTracker(storage)
	tracker.Start()
	defer tracker.Stop()

	ph := NewProxyHandler(nil, nil, nil)
	ph.SetProviderPool(&providerpool.Pool{UsageTracker: tracker})

	sse := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_stream_1","model":"claude-3","role":"assistant"},"usage":{"input_tokens":9}}`,
		"",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":4}}`,
		"",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(sse)),
	}

	rec := httptest.NewRecorder()
	pr := &parsedRequest{
		model:              "claude-3",
		resolvedModel:      "claude-3",
		resolvedProviderID: "anthropic",
		resolvedProvider:   "Anthropic",
		upstreamFormat:     ProviderTypeAnthropic,
		streaming:          true,
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ph.copyResponse(rec, resp, pr, req)

	time.Sleep(20 * time.Millisecond)

	stats := tracker.GetCurrentStats()
	s, ok := stats["anthropic:claude-3"]
	if !ok {
		t.Fatalf("expected usage stats for anthropic:claude-3, got keys=%v", mapsKeys(stats))
	}
	if s.TotalRequests != 1 {
		t.Fatalf("expected 1 request, got %d", s.TotalRequests)
	}
	if s.TotalInputTokens != 9 || s.TotalOutputTokens != 4 {
		t.Fatalf("unexpected tokens in=%d out=%d", s.TotalInputTokens, s.TotalOutputTokens)
	}
}

func mapsKeys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestCopyResponse_RecordsPromptCacheStats_NonStreamingAnthropic(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"msg_1","model":"claude-sonnet-4-5","usage":{"input_tokens":120,"output_tokens":20,"cache_read_input_tokens":90,"cache_creation_input_tokens":15}}`,
		)),
	}

	rec := httptest.NewRecorder()
	pr := &parsedRequest{
		model:            "claude-sonnet-4-5",
		resolvedModel:    "claude-sonnet-4-5",
		resolvedProvider: "anthropic",
		upstreamFormat:   ProviderTypeAnthropic,
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ph.copyResponse(rec, resp, pr, req)

	stats := ph.GetPromptCacheStats()
	if stats.Requests != 1 {
		t.Fatalf("requests = %d, want 1", stats.Requests)
	}
	if stats.CacheHits != 1 || stats.CacheMisses != 0 {
		t.Fatalf("hits/misses = %d/%d, want 1/0", stats.CacheHits, stats.CacheMisses)
	}
	if stats.TotalInputTokens != 120 {
		t.Fatalf("total_input_tokens = %d, want 120", stats.TotalInputTokens)
	}
	if stats.TotalCacheReadTokens != 90 {
		t.Fatalf("total_cache_read_tokens = %d, want 90", stats.TotalCacheReadTokens)
	}
	if stats.TotalCacheCreation != 15 {
		t.Fatalf("total_cache_creation_tokens = %d, want 15", stats.TotalCacheCreation)
	}
}

func TestCopyResponse_RecordsPromptCacheStats_StreamingAnthropic(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)
	sse := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_2","model":"claude-3","role":"assistant"}}`,
		"",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":80,"output_tokens":12,"cache_read_input_tokens":40,"cache_creation_input_tokens":8}}`,
		"",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(sse)),
	}
	rec := httptest.NewRecorder()
	pr := &parsedRequest{
		model:            "claude-3",
		resolvedModel:    "claude-3",
		resolvedProvider: "anthropic",
		upstreamFormat:   ProviderTypeAnthropic,
		streaming:        true,
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ph.copyResponse(rec, resp, pr, req)

	stats := ph.GetPromptCacheStats()
	if stats.Requests != 1 {
		t.Fatalf("requests = %d, want 1", stats.Requests)
	}
	if stats.TotalInputTokens != 80 {
		t.Fatalf("total_input_tokens = %d, want 80", stats.TotalInputTokens)
	}
	if stats.TotalCacheReadTokens != 40 {
		t.Fatalf("total_cache_read_tokens = %d, want 40", stats.TotalCacheReadTokens)
	}
	if stats.TotalCacheCreation != 8 {
		t.Fatalf("total_cache_creation_tokens = %d, want 8", stats.TotalCacheCreation)
	}
}
