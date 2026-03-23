package proxybridge

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func TestParseSSEChunk_ResponsesOutputTextDelta(t *testing.T) {
	payload := `{"type":"response.output_text.delta","response_id":"resp_1","delta":"Hello"}`

	chunk, done, err := ParseSSEChunk(payload)
	if err != nil {
		t.Fatalf("ParseSSEChunk failed: %v", err)
	}
	if done {
		t.Fatal("done = true, want false")
	}
	if chunk.ID != "resp_1" {
		t.Fatalf("chunk.ID = %q, want %q", chunk.ID, "resp_1")
	}
	if chunk.Delta != "Hello" {
		t.Fatalf("chunk.Delta = %q, want %q", chunk.Delta, "Hello")
	}
}

func TestParseSSEChunk_ResponsesFunctionCallDone(t *testing.T) {
	payload := `{"type":"response.output_item.done","response_id":"resp_2","item":{"type":"function_call","call_id":"call_1","name":"exec","arguments":"{\"cmd\":\"pwd\"}"}}`

	chunk, done, err := ParseSSEChunk(payload)
	if err != nil {
		t.Fatalf("ParseSSEChunk failed: %v", err)
	}
	if done {
		t.Fatal("done = true, want false")
	}
	if len(chunk.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(chunk.ToolCalls))
	}
	tc := chunk.ToolCalls[0]
	if tc.ID != "call_1" {
		t.Fatalf("tool call id = %q, want %q", tc.ID, "call_1")
	}
	if tc.Name != "exec" {
		t.Fatalf("tool call name = %q, want %q", tc.Name, "exec")
	}
	if tc.Arguments != `{"cmd":"pwd"}` {
		t.Fatalf("tool call arguments = %q, want %q", tc.Arguments, `{"cmd":"pwd"}`)
	}
}

func TestParseSSEChunk_ResponsesFunctionCallAdded(t *testing.T) {
	payload := `{"type":"response.output_item.added","response_id":"resp_added","item":{"type":"function_call","id":"fc_1","call_id":"call_added_1","name":"exec","arguments":""}}`

	chunk, done, err := ParseSSEChunk(payload)
	if err != nil {
		t.Fatalf("ParseSSEChunk failed: %v", err)
	}
	if done {
		t.Fatal("done = true, want false")
	}
	if len(chunk.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(chunk.ToolCalls))
	}
	tc := chunk.ToolCalls[0]
	if tc.ID != "call_added_1" {
		t.Fatalf("tool call id = %q, want %q", tc.ID, "call_added_1")
	}
	if tc.Name != "exec" {
		t.Fatalf("tool call name = %q, want %q", tc.Name, "exec")
	}
}

func TestParseSSEChunk_ResponsesFunctionCallArgumentsDelta(t *testing.T) {
	payload := `{"type":"response.function_call_arguments.delta","response_id":"resp_args_1","delta":"{\"cmd\":\"pw"}`

	chunk, done, err := ParseSSEChunk(payload)
	if err != nil {
		t.Fatalf("ParseSSEChunk failed: %v", err)
	}
	if done {
		t.Fatal("done = true, want false")
	}
	if len(chunk.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(chunk.ToolCalls))
	}
	if chunk.ToolCalls[0].Arguments != `{"cmd":"pw` {
		t.Fatalf("tool call arguments = %q, want %q", chunk.ToolCalls[0].Arguments, `{"cmd":"pw`)
	}
}

func TestParseSSEChunk_ResponsesFunctionCallArgumentsDone(t *testing.T) {
	payload := `{"type":"response.function_call_arguments.done","response_id":"resp_args_done_1","arguments":"{\"cmd\":\"pwd\"}"}`

	chunk, done, err := ParseSSEChunk(payload)
	if err != nil {
		t.Fatalf("ParseSSEChunk failed: %v", err)
	}
	if done {
		t.Fatal("done = true, want false")
	}
	if len(chunk.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(chunk.ToolCalls))
	}
	if chunk.ToolCalls[0].Arguments != `{"cmd":"pwd"}` {
		t.Fatalf("tool call arguments = %q, want %q", chunk.ToolCalls[0].Arguments, `{"cmd":"pwd"}`)
	}
}

func TestParseSSEChunk_ResponsesCompleted(t *testing.T) {
	payload := `{"type":"response.completed","response":{"id":"resp_3","model":"o3","usage":{"input_tokens":10,"output_tokens":4,"total_tokens":14}}}`

	chunk, done, err := ParseSSEChunk(payload)
	if err != nil {
		t.Fatalf("ParseSSEChunk failed: %v", err)
	}
	if !done || !chunk.Done {
		t.Fatal("expected done=true on response.completed")
	}
	if chunk.ID != "resp_3" {
		t.Fatalf("chunk.ID = %q, want %q", chunk.ID, "resp_3")
	}
	if chunk.Model != "o3" {
		t.Fatalf("chunk.Model = %q, want %q", chunk.Model, "o3")
	}
	if chunk.Usage == nil {
		t.Fatal("usage is nil")
	}
	if chunk.Usage.TotalTokens != 14 {
		t.Fatalf("total_tokens = %d, want 14", chunk.Usage.TotalTokens)
	}
}

func TestParseSSEChunk_ResponsesCompletedExtractsFinalText(t *testing.T) {
	payload := `{"type":"response.completed","response":{"id":"resp_3b","model":"o3","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"A"}]}]}}`

	chunk, done, err := ParseSSEChunk(payload)
	if err != nil {
		t.Fatalf("ParseSSEChunk failed: %v", err)
	}
	if !done || !chunk.Done {
		t.Fatal("expected done=true on response.completed")
	}
	if chunk.Delta != "A" {
		t.Fatalf("chunk.Delta = %q, want %q", chunk.Delta, "A")
	}
}

func TestParseSSEChunk_ResponsesCompletedExtractsFunctionCallFallback(t *testing.T) {
	payload := `{"type":"response.completed","response":{"id":"resp_fc_done","model":"o3","output":[{"type":"function_call","id":"fc_9","call_id":"call_fc_done","name":"exec","arguments":"{\"cmd\":\"pwd\"}"}]}}`

	chunk, done, err := ParseSSEChunk(payload)
	if err != nil {
		t.Fatalf("ParseSSEChunk failed: %v", err)
	}
	if !done || !chunk.Done {
		t.Fatal("expected done=true on response.completed")
	}
	if len(chunk.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(chunk.ToolCalls))
	}
	tc := chunk.ToolCalls[0]
	if tc.ID != "call_fc_done" {
		t.Fatalf("tool call id = %q, want %q", tc.ID, "call_fc_done")
	}
	if tc.Name != "exec" {
		t.Fatalf("tool call name = %q, want %q", tc.Name, "exec")
	}
	if tc.Arguments != `{"cmd":"pwd"}` {
		t.Fatalf("tool call arguments = %q, want %q", tc.Arguments, `{"cmd":"pwd"}`)
	}
}

func TestParseSSEChunk_ResponsesFailed(t *testing.T) {
	payload := `{"type":"response.failed","response":{"id":"resp_4"},"error":{"message":"boom","type":"invalid_request_error"}}`

	chunk, done, err := ParseSSEChunk(payload)
	if err != nil {
		t.Fatalf("ParseSSEChunk failed: %v", err)
	}
	if !done || !chunk.Done {
		t.Fatal("expected done=true on response.failed")
	}
	if chunk.Error != "boom" {
		t.Fatalf("chunk.Error = %q, want %q", chunk.Error, "boom")
	}
}

func TestParseSSEChunk_ResponsesMetadataEventAsProgress(t *testing.T) {
	payload := `{"type":"response.created","response":{"id":"resp_meta","model":"o3"}}`

	chunk, done, err := ParseSSEChunk(payload)
	if err != nil {
		t.Fatalf("ParseSSEChunk failed: %v", err)
	}
	if done {
		t.Fatal("done = true, want false")
	}
	if chunk.Progress != "response.created" {
		t.Fatalf("chunk.Progress = %q, want %q", chunk.Progress, "response.created")
	}
	if chunk.ID != "resp_meta" {
		t.Fatalf("chunk.ID = %q, want %q", chunk.ID, "resp_meta")
	}
}

func TestParseSSEChunk_OpenAIErrorPayload(t *testing.T) {
	payload := `{"error":{"message":"upstream request failed","type":"upstream_error"}}`

	chunk, done, err := ParseSSEChunk(payload)
	if err != nil {
		t.Fatalf("ParseSSEChunk failed: %v", err)
	}
	if !done || !chunk.Done {
		t.Fatal("expected done=true on top-level error payload")
	}
	if chunk.Error != "upstream request failed" {
		t.Fatalf("chunk.Error = %q, want %q", chunk.Error, "upstream request failed")
	}
}

type bridgeMetricsStub struct {
	counts map[string]int64
}

func (m *bridgeMetricsStub) RecordCounter(name string, value int64, _ map[string]string) {
	if m.counts == nil {
		m.counts = make(map[string]int64)
	}
	m.counts[name] += value
}

func TestParseChatResponseWithMetricsRepairsStringifiedToolArguments(t *testing.T) {
	payload := []byte(`{
		"id":"resp_repair",
		"model":"gpt-4o",
		"choices":[
			{
				"index":0,
				"message":{
					"role":"assistant",
					"tool_calls":[
						{
							"id":"call_1",
							"type":"function",
							"function":{"name":"exec","arguments":"{'cmd':'pwd'}"}
						}
					]
				}
			}
		]
	}`)

	metrics := &bridgeMetricsStub{}
	resp, err := parseChatResponseWithMetrics(payload, metrics)
	if err != nil {
		t.Fatalf("parseChatResponseWithMetrics() error = %v", err)
	}
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(resp.Message.ToolCalls))
	}
	if got := resp.Message.ToolCalls[0].Arguments; got != `{"cmd":"pwd"}` {
		t.Fatalf("arguments = %q, want canonical JSON object", got)
	}
	if got := metrics.counts["provider_tool_normalization_repair_total"]; got != 1 {
		t.Fatalf("repair metric = %d, want 1", got)
	}
}

func TestNormalizeInboundBridgeToolCall_RepairsMissingStringClosureAfterEscapedQuotes(t *testing.T) {
	metrics := &bridgeMetricsStub{}

	_, _, arguments := normalizeInboundBridgeToolCall(
		"call_exec_1",
		"exec",
		json.RawMessage(`"{\"command\": \"echo \\\"Hello from exec\\\"}"`),
		false,
		metrics,
	)

	if arguments != `{"command":"echo \"Hello from exec\""}` {
		t.Fatalf("arguments = %q, want repaired canonical JSON", arguments)
	}
	if got := metrics.counts["provider_tool_normalization_repair_total"]; got != 1 {
		t.Fatalf("repair metric = %d, want 1", got)
	}
}

func TestParseSSEChunk_ResponsesFunctionCallDoneRepairsMissingStringClosureAfterEscapedQuotes(t *testing.T) {
	payload := `{"type":"response.output_item.done","response_id":"resp_quote_fix","item":{"type":"function_call","call_id":"call_exec_1","name":"exec","arguments":"{\"command\": \"echo \\\"Hello from exec\\\"}"}}`

	chunk, done, err := ParseSSEChunk(payload)
	if err != nil {
		t.Fatalf("ParseSSEChunk failed: %v", err)
	}
	if done {
		t.Fatal("done = true, want false")
	}
	if len(chunk.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(chunk.ToolCalls))
	}
	if got := chunk.ToolCalls[0].Arguments; got != `{"command":"echo \"Hello from exec\""}` {
		t.Fatalf("tool call arguments = %q, want repaired canonical JSON", got)
	}
}

func TestParseResponsesChatResponseWithMetricsRecordsNormalizationFailure(t *testing.T) {
	payload := []byte(`{
		"id":"resp_fail",
		"object":"response",
		"model":"o3",
		"output":[
			{"type":"function_call","call_id":"call_bad","name":"exec","arguments":"oops"}
		]
	}`)

	metrics := &bridgeMetricsStub{}
	resp, handled, err := parseResponsesChatResponseWithMetrics(payload, metrics)
	if err != nil {
		t.Fatalf("parseResponsesChatResponseWithMetrics() error = %v", err)
	}
	if !handled {
		t.Fatal("handled = false, want true")
	}
	if resp == nil || len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("tool calls = %+v, want one normalized tool call", resp)
	}
	if got := resp.Message.ToolCalls[0].Arguments; got != "oops" {
		t.Fatalf("arguments = %q, want original irreparable payload", got)
	}
	if got := metrics.counts["provider_tool_normalization_fail_total"]; got != 1 {
		t.Fatalf("failure metric = %d, want 1", got)
	}
}

func TestParseChatResponse_ResponsesObject(t *testing.T) {
	body := []byte(`{
		"id":"resp_5",
		"object":"response",
		"model":"o3",
		"output":[
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"result:"}]},
			{"type":"function_call","call_id":"call_5","name":"exec","arguments":"{\"cmd\":\"ls\"}"}
		],
		"usage":{"input_tokens":11,"output_tokens":3,"total_tokens":14}
	}`)

	resp, err := ParseChatResponse(body)
	if err != nil {
		t.Fatalf("ParseChatResponse failed: %v", err)
	}
	if resp.Model != "o3" {
		t.Fatalf("model = %q, want %q", resp.Model, "o3")
	}
	if resp.Message.Content != "result:" {
		t.Fatalf("content = %q, want %q", resp.Message.Content, "result:")
	}
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(resp.Message.ToolCalls))
	}
	tc := resp.Message.ToolCalls[0]
	if tc.ID != "call_5" {
		t.Fatalf("tool call id = %q, want %q", tc.ID, "call_5")
	}
	if tc.Name != "exec" {
		t.Fatalf("tool call name = %q, want %q", tc.Name, "exec")
	}
	if tc.Arguments != `{"cmd":"ls"}` {
		t.Fatalf("tool call arguments = %q, want %q", tc.Arguments, `{"cmd":"ls"}`)
	}
	if resp.Usage.TotalTokens != 14 {
		t.Fatalf("total_tokens = %d, want 14", resp.Usage.TotalTokens)
	}
}

func TestMarshalChatRequest_ToolMessageContentIsJSONString(t *testing.T) {
	req := llm.ChatRequest{
		Model: "gpt-5.3-codex-spark",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "帮我查今天战况"},
			{
				Role:    llm.RoleAssistant,
				Content: "",
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "web_search", Arguments: `{"query":"Iran latest updates"}`},
				},
			},
			{Role: llm.RoleTool, ToolCallID: "call_1", Content: `{"query":"Iran latest updates","results":[{"title":"x"}]}`},
		},
	}

	raw, err := MarshalChatRequest(req)
	if err != nil {
		t.Fatalf("MarshalChatRequest failed: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	msgs, ok := payload["messages"].([]any)
	if !ok || len(msgs) != 3 {
		t.Fatalf("messages shape mismatch: %#v", payload["messages"])
	}
	assistantMsg, ok := msgs[1].(map[string]any)
	if !ok {
		t.Fatalf("assistant message type mismatch: %#v", msgs[1])
	}
	toolCalls, ok := assistantMsg["tool_calls"].([]any)
	if !ok || len(toolCalls) != 1 {
		t.Fatalf("assistant tool_calls shape mismatch: %#v", assistantMsg["tool_calls"])
	}
	tc, ok := toolCalls[0].(map[string]any)
	if !ok {
		t.Fatalf("tool_call type mismatch: %#v", toolCalls[0])
	}
	fn, ok := tc["function"].(map[string]any)
	if !ok {
		t.Fatalf("tool_call.function type mismatch: %#v", tc["function"])
	}
	args, ok := fn["arguments"].(string)
	if !ok {
		t.Fatalf("tool_call.function.arguments should be string, got type=%T value=%#v", fn["arguments"], fn["arguments"])
	}
	if args != `{"query":"Iran latest updates"}` {
		t.Fatalf("tool_call.function.arguments = %q", args)
	}
	toolMsg, ok := msgs[2].(map[string]any)
	if !ok {
		t.Fatalf("tool message type mismatch: %#v", msgs[2])
	}
	content, ok := toolMsg["content"].(string)
	if !ok {
		t.Fatalf("tool content should be string, got type=%T value=%#v", toolMsg["content"], toolMsg["content"])
	}
	if content != `{"query":"Iran latest updates","results":[{"title":"x"}]}` {
		t.Fatalf("tool content = %q", content)
	}
}

func TestMarshalResponsesRequest_UsesNativeResponsesShape(t *testing.T) {
	req := llm.ChatRequest{
		Model:              "gpt-5.3-codex-spark",
		PreviousResponseID: "resp_prev_1",
		Stream:             true,
		MaxTokens:          128,
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "continue"},
		},
	}

	raw, err := MarshalResponsesRequest(req)
	if err != nil {
		t.Fatalf("MarshalResponsesRequest failed: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if got["store"] != true {
		t.Fatalf("store = %#v, want true", got["store"])
	}
	if got["previous_response_id"] != "resp_prev_1" {
		t.Fatalf("previous_response_id = %#v, want %q", got["previous_response_id"], "resp_prev_1")
	}
	if _, ok := got["messages"]; ok {
		t.Fatalf("native responses payload should not include messages: %s", string(raw))
	}
}

func TestMarshalResponsesRequest_FirstTurnExtractsInstructions(t *testing.T) {
	req := llm.ChatRequest{
		Model: "gpt-5.3-codex-spark",
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "system prompt"},
			{Role: llm.RoleUser, Content: "hello"},
		},
	}

	raw, err := MarshalResponsesRequest(req)
	if err != nil {
		t.Fatalf("MarshalResponsesRequest failed: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if got["instructions"] != "system prompt" {
		t.Fatalf("instructions = %#v, want %q", got["instructions"], "system prompt")
	}
}

func TestMarshalResponsesRequest_ContinuationKeepsLastAssistantContext(t *testing.T) {
	req := llm.ChatRequest{
		Model:              "gpt-5.3-codex-spark",
		PreviousResponseID: "resp_prev_ctx_1",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "旧问题"},
			{Role: llm.RoleAssistant, Content: "A) 选项1 B) 选项2"},
			{Role: llm.RoleUser, Content: "B"},
		},
	}

	raw, err := MarshalResponsesRequest(req)
	if err != nil {
		t.Fatalf("MarshalResponsesRequest failed: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	input, ok := got["input"].([]any)
	if !ok {
		t.Fatalf("input missing or wrong type: %#v", got["input"])
	}
	if len(input) != 2 {
		t.Fatalf("input length = %d, want 2", len(input))
	}

	first, _ := input[0].(map[string]any)
	if first["role"] != "assistant" {
		t.Fatalf("first role = %#v, want assistant", first["role"])
	}
}

func TestMarshalResponsesRequest_ContinuationSkipsAssistantToolCallEcho(t *testing.T) {
	req := llm.ChatRequest{
		Model:              "gpt-5.3-codex-spark",
		PreviousResponseID: "resp_prev_tool_1",
		Messages: []llm.Message{
			{
				Role:    llm.RoleAssistant,
				Content: "tool call summary",
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "exec", Arguments: `{"command":"blue web_search query=\"ZimaOS Blue latest news\""}`},
				},
			},
			{Role: llm.RoleTool, ToolCallID: "call_1", Content: `{"status":"completed","stdout":"ok"}`},
		},
	}

	raw, err := MarshalResponsesRequest(req)
	if err != nil {
		t.Fatalf("MarshalResponsesRequest failed: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	input, ok := got["input"].([]any)
	if !ok {
		t.Fatalf("input missing or wrong type: %#v", got["input"])
	}
	if len(input) != 1 {
		t.Fatalf("input length = %d, want 1", len(input))
	}
	first, _ := input[0].(map[string]any)
	if first["type"] != "function_call_output" {
		t.Fatalf("first type = %#v, want function_call_output", first["type"])
	}
	if first["call_id"] != "call_1" {
		t.Fatalf("call_id = %#v, want call_1", first["call_id"])
	}
}

func TestMarshalResponsesRequest_ContinuationSizeGuard(t *testing.T) {
	huge := `{"status":"completed","stdout":"` + strings.Repeat("x", 24000) + `"}`
	req := llm.ChatRequest{
		Model:              "gpt-5.3-codex-spark",
		PreviousResponseID: "resp_prev_size_1",
		Messages: []llm.Message{
			{
				Role: llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "exec", Arguments: `{"command":"blue web_search query=\"a\""}`},
					{ID: "call_2", Name: "exec", Arguments: `{"command":"blue web_search query=\"b\""}`},
					{ID: "call_3", Name: "exec", Arguments: `{"command":"blue web_search query=\"c\""}`},
				},
			},
			{Role: llm.RoleTool, ToolCallID: "call_1", Content: huge},
			{Role: llm.RoleTool, ToolCallID: "call_2", Content: huge},
			{Role: llm.RoleTool, ToolCallID: "call_3", Content: huge},
			{Role: llm.RoleUser, Content: strings.Repeat("progress ", 2000)},
		},
		Tools: []llm.Tool{
			{Name: "exec", Description: strings.Repeat("desc ", 1000)},
		},
	}

	raw, err := MarshalResponsesRequest(req)
	if err != nil {
		t.Fatalf("MarshalResponsesRequest failed: %v", err)
	}
	if len(raw) > maxResponsesRequestBytes {
		t.Fatalf("payload size = %d, want <= %d", len(raw), maxResponsesRequestBytes)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	input, ok := got["input"].([]any)
	if !ok {
		t.Fatalf("input missing or wrong type: %#v", got["input"])
	}
	if len(input) == 0 {
		t.Fatal("input should not be empty after size guard")
	}

	foundLatestCall := false
	for _, item := range input {
		m, _ := item.(map[string]any)
		if m["type"] == "function_call_output" && m["call_id"] == "call_3" {
			foundLatestCall = true
			if output, _ := m["output"].(string); !strings.Contains(output, "[truncated]") {
				t.Fatalf("latest tool output should be truncated, got: %q", output)
			}
		}
	}
	if !foundLatestCall {
		t.Fatal("latest function_call_output (call_3) should be preserved")
	}
}

func TestMarshalChatRequest_EmitsPromptCacheKey(t *testing.T) {
	req := llm.ChatRequest{
		Model:          "gpt-5",
		Messages:       []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
		PromptCacheKey: "pcache-key-1",
	}
	raw, err := MarshalChatRequest(req)
	if err != nil {
		t.Fatalf("MarshalChatRequest failed: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got, _ := body["prompt_cache_key"].(string); got != "pcache-key-1" {
		t.Fatalf("prompt_cache_key = %q, want %q", got, "pcache-key-1")
	}
}

func TestMarshalResponsesRequest_EmitsPromptCacheKey(t *testing.T) {
	req := llm.ChatRequest{
		Model:          "gpt-5",
		Messages:       []llm.Message{{Role: llm.RoleSystem, Content: "sys"}, {Role: llm.RoleUser, Content: "hello"}},
		PromptCacheKey: "pcache-key-2",
	}
	raw, err := MarshalResponsesRequest(req)
	if err != nil {
		t.Fatalf("MarshalResponsesRequest failed: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got, _ := body["prompt_cache_key"].(string); got != "pcache-key-2" {
		t.Fatalf("prompt_cache_key = %q, want %q", got, "pcache-key-2")
	}
}
