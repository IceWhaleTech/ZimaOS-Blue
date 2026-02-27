package proxy

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// ---------- Request conversion: tool calls + tool results with special chars ----------

func TestConvertRequest_ToolCallsWithSpecialChars(t *testing.T) {
	fc := NewFormatConverter()

	// OpenAI request with:
	// - system prompt containing quotes and newlines
	// - assistant message with tool_calls whose arguments contain special chars
	// - tool result containing JSON with special chars
	openaiReq := `{
		"model": "claude-3-5-sonnet",
		"messages": [
			{"role": "system", "content": "You are helpful.\nDon't say \"bad\" things.\nUse path\\to\\file."},
			{"role": "user", "content": "Search for \"hello world\" in C:\\Users\\test"},
			{"role": "assistant", "content": "I'll search for that.", "tool_calls": [
				{
					"id": "call_abc123",
					"type": "function",
					"function": {
						"name": "search_files",
						"arguments": "{\"query\":\"hello \\\"world\\\"\",\"path\":\"C:\\\\Users\\\\test\",\"note\":\"line1\\nline2\\ttab\"}"
					}
				}
			]},
			{"role": "tool", "tool_call_id": "call_abc123", "content": "Found 3 results:\n1. C:\\Users\\test\\file1.txt: \"hello world\"\n2. C:\\Users\\test\\file2.txt: contains\ttabs\nDone."}
		],
		"max_tokens": 1024,
		"stream": false
	}`

	converted, path, err := fc.ConvertRequest([]byte(openaiReq), ProviderTypeAnthropic)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}
	if path != "/v1/messages" {
		t.Errorf("path = %s, want /v1/messages", path)
	}

	// Verify the output is valid JSON by round-tripping
	var anthropicReq AnthropicRequest
	if err := json.Unmarshal(converted, &anthropicReq); err != nil {
		t.Fatalf("output is not valid JSON: %v\n  body: %s", err, converted)
	}

	// Verify system prompt preserved special chars
	sysStr := ""
	switch s := anthropicReq.System.(type) {
	case string:
		sysStr = s
	case []interface{}:
		for _, b := range s {
			if block, ok := b.(map[string]interface{}); ok {
				if text, ok := block["text"].(string); ok {
					if sysStr != "" {
						sysStr += "\n"
					}
					sysStr += text
				}
			}
		}
	default:
		t.Fatalf("unexpected System type: %T", anthropicReq.System)
	}
	if !strings.Contains(sysStr, "Don't say \"bad\" things") {
		t.Errorf("system prompt lost quotes: %s", sysStr)
	}
	if !strings.Contains(sysStr, "path\\to\\file") {
		t.Errorf("system prompt lost backslash: %s", sysStr)
	}

	// Should have 3 messages: user, assistant (with tool_use), user (tool_result)
	if len(anthropicReq.Messages) != 3 {
		t.Fatalf("Messages count = %d, want 3", len(anthropicReq.Messages))
	}

	// Verify assistant message has tool_use block
	assistantMsg := anthropicReq.Messages[1]
	blocks, ok := assistantMsg.Content.([]interface{})
	if !ok {
		t.Fatalf("assistant content is not array: %T", assistantMsg.Content)
	}
	if len(blocks) < 2 {
		t.Fatalf("assistant blocks count = %d, want >= 2", len(blocks))
	}

	// Verify tool_result message
	toolResultMsg := anthropicReq.Messages[2]
	if toolResultMsg.Role != "user" {
		t.Errorf("tool result role = %s, want user", toolResultMsg.Role)
	}
	resultBlocks, ok := toolResultMsg.Content.([]interface{})
	if !ok {
		t.Fatalf("tool result content is not array: %T", toolResultMsg.Content)
	}
	if len(resultBlocks) == 0 {
		t.Fatal("tool result has no blocks")
	}
	firstBlock, ok := resultBlocks[0].(map[string]interface{})
	if !ok {
		t.Fatalf("tool result block is not map: %T", resultBlocks[0])
	}
	if firstBlock["type"] != "tool_result" {
		t.Errorf("block type = %v, want tool_result", firstBlock["type"])
	}
	content, _ := firstBlock["content"].(string)
	if !strings.Contains(content, "C:\\Users\\test\\file1.txt") {
		t.Errorf("tool result lost backslashes: %s", content)
	}
	if !strings.Contains(content, "\"hello world\"") {
		t.Errorf("tool result lost quotes: %s", content)
	}
	if !strings.Contains(content, "contains\ttabs") {
		t.Errorf("tool result lost tabs: %s", content)
	}

	// Final check: re-marshal and re-unmarshal to verify full round-trip
	remarshaled, err := json.Marshal(anthropicReq)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	var roundTrip AnthropicRequest
	if err := json.Unmarshal(remarshaled, &roundTrip); err != nil {
		t.Fatalf("round-trip unmarshal failed: %v\n  body: %s", err, remarshaled)
	}
}

// ---------- Response conversion: Anthropic tool_use → OpenAI tool_calls ----------

func TestConvertResponse_ToolUseWithSpecialChars(t *testing.T) {
	fc := NewFormatConverter()

	// Anthropic response with text + tool_use containing special chars in input
	anthropicResp := `{
		"id": "msg_special",
		"type": "message",
		"role": "assistant",
		"content": [
			{"type": "text", "text": "I'll search for \"hello\\nworld\" now."},
			{
				"type": "tool_use",
				"id": "toolu_abc",
				"name": "search_files",
				"input": {
					"query": "hello \"world\"",
					"path": "C:\\Users\\test\\docs",
					"regex": "line1\nline2\ttab"
				}
			}
		],
		"model": "claude-3-5-sonnet-20241022",
		"stop_reason": "tool_use",
		"usage": {"input_tokens": 100, "output_tokens": 50}
	}`

	converted, err := fc.ConvertResponse([]byte(anthropicResp), ProviderTypeAnthropic)
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}

	// Must be valid JSON
	var openaiResp OpenAIChatResponse
	if err := json.Unmarshal(converted, &openaiResp); err != nil {
		t.Fatalf("output is not valid JSON: %v\n  body: %s", err, converted)
	}

	if len(openaiResp.Choices) != 1 {
		t.Fatalf("Choices count = %d, want 1", len(openaiResp.Choices))
	}

	choice := openaiResp.Choices[0]

	// Text content should contain the special chars
	if !strings.Contains(choice.Message.Content.(string), `"hello\nworld"`) {
		t.Errorf("text content lost special chars: %v", choice.Message.Content)
	}

	// Should have tool_calls
	if len(choice.Message.ToolCalls) != 1 {
		t.Fatalf("ToolCalls count = %d, want 1", len(choice.Message.ToolCalls))
	}

	tc := choice.Message.ToolCalls[0]
	if tc.ID != "toolu_abc" {
		t.Errorf("tool call ID = %s, want toolu_abc", tc.ID)
	}
	if tc.Function.Name != "search_files" {
		t.Errorf("tool call name = %s, want search_files", tc.Function.Name)
	}

	// Arguments should be valid JSON with special chars preserved
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		t.Fatalf("tool call arguments not valid JSON: %v\n  args: %s", err, tc.Function.Arguments)
	}
	if args["query"] != `hello "world"` {
		t.Errorf("query = %v, want hello \"world\"", args["query"])
	}
	if args["path"] != `C:\Users\test\docs` {
		t.Errorf("path = %v, want C:\\Users\\test\\docs", args["path"])
	}

	// finish_reason should be tool_calls
	if choice.FinishReason != "tool_calls" {
		t.Errorf("FinishReason = %s, want tool_calls", choice.FinishReason)
	}
}

// ---------- Streaming: Anthropic SSE → OpenAI SSE with special chars ----------

// flusherRecorder implements http.ResponseWriter + http.Flusher for testing.
type flusherRecorder struct {
	buf bytes.Buffer
	hdr http.Header
}

func (f *flusherRecorder) Header() http.Header {
	if f.hdr == nil {
		f.hdr = make(http.Header)
	}
	return f.hdr
}
func (f *flusherRecorder) Write(b []byte) (int, error) { return f.buf.Write(b) }
func (f *flusherRecorder) WriteHeader(int)             {}
func (f *flusherRecorder) Flush()                      {}

func TestConvertAnthropicStream_SpecialChars(t *testing.T) {
	fc := NewFormatConverter()

	// Text with quotes, backslashes, newlines, tabs, and Chinese — no manual escaping.
	specialText := "He said \"hello\\world\"\nand\ttab 你好"

	sseInput := strings.Join([]string{
		sseEvent(t, AnthropicStreamEvent{
			Type:    "message_start",
			Message: &AnthropicResponse{ID: "msg_test", Model: "claude-3", Role: "assistant"},
		}),
		sseEvent(t, AnthropicStreamEvent{
			Type:         "content_block_start",
			Index:        0,
			ContentBlock: &AnthropicStreamContentBlock{Type: "text"},
		}),
		sseEvent(t, AnthropicStreamEvent{
			Type:  "content_block_delta",
			Index: 0,
			Delta: &AnthropicStreamDelta{Type: "text_delta", Text: specialText},
		}),
		sseEvent(t, AnthropicStreamEvent{Type: "content_block_stop", Index: 0}),
		sseEvent(t, AnthropicStreamEvent{
			Type:  "message_delta",
			Delta: &AnthropicStreamDelta{StopReason: "end_turn"},
			Usage: &AnthropicStreamUsage{OutputTokens: 15},
		}),
		sseEvent(t, AnthropicStreamEvent{Type: "message_stop"}),
		"",
	}, "\n")

	reader := strings.NewReader(sseInput)
	rec := &flusherRecorder{}

	err := fc.convertAnthropicStream(reader, rec)
	if err != nil {
		t.Fatalf("convertAnthropicStream failed: %v", err)
	}

	output := rec.buf.String()

	// Parse each SSE line and verify it's valid JSON
	var contentChunks []string
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			continue
		}

		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			t.Fatalf("SSE chunk is not valid JSON: %v\n  payload: %s", err, payload)
		}

		// Extract content from delta if present
		choices, _ := chunk["choices"].([]interface{})
		if len(choices) > 0 {
			choice, _ := choices[0].(map[string]interface{})
			delta, _ := choice["delta"].(map[string]interface{})
			if content, ok := delta["content"].(string); ok {
				contentChunks = append(contentChunks, content)
			}
		}
	}

	// Reassemble content and verify special chars survived
	fullContent := strings.Join(contentChunks, "")
	if !strings.Contains(fullContent, `"hello\world"`) {
		t.Errorf("content lost quotes/backslash: %q", fullContent)
	}
	if !strings.Contains(fullContent, "\n") {
		t.Errorf("content lost newline: %q", fullContent)
	}
	if !strings.Contains(fullContent, "\t") {
		t.Errorf("content lost tab: %q", fullContent)
	}
	if !strings.Contains(fullContent, "你好") {
		t.Errorf("content lost Chinese: %q", fullContent)
	}
}

// ---------- Streaming: CloudCode SSE → OpenAI SSE with special chars ----------

func TestConvertCloudCodeStream_SpecialChars(t *testing.T) {
	fc := NewFormatConverter()

	// Build Gemini-style SSE events via json.Marshal — no manual escaping.
	chunk1 := GeminiStreamChunk{
		Candidates: []GeminiCandidate{{
			Content: GeminiContent{Parts: []GeminiPart{{Text: "path is C:\\Users\\test and \"quoted\"\nline2"}}},
		}},
	}
	chunk2 := GeminiStreamChunk{
		Candidates: []GeminiCandidate{{
			Content:      GeminiContent{Parts: []GeminiPart{{Text: "done"}}},
			FinishReason: "STOP",
		}},
	}
	data1, _ := json.Marshal(chunk1)
	data2, _ := json.Marshal(chunk2)

	sseInput := strings.Join([]string{
		"data: " + string(data1),
		"data: " + string(data2),
		"",
	}, "\n")

	reader := strings.NewReader(sseInput)
	rec := &flusherRecorder{}

	err := fc.convertCloudCodeStream(reader, rec)
	if err != nil {
		t.Fatalf("convertCloudCodeStream failed: %v", err)
	}

	output := rec.buf.String()

	// Parse each SSE line
	var contentChunks []string
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			continue
		}

		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			t.Fatalf("SSE chunk is not valid JSON: %v\n  payload: %s", err, payload)
		}

		choices, _ := chunk["choices"].([]interface{})
		if len(choices) > 0 {
			choice, _ := choices[0].(map[string]interface{})
			delta, _ := choice["delta"].(map[string]interface{})
			if content, ok := delta["content"].(string); ok {
				contentChunks = append(contentChunks, content)
			}
		}
	}

	fullContent := strings.Join(contentChunks, "")
	if !strings.Contains(fullContent, `C:\Users\test`) {
		t.Errorf("content lost backslashes: %q", fullContent)
	}
	if !strings.Contains(fullContent, `"quoted"`) {
		t.Errorf("content lost quotes: %q", fullContent)
	}
}

// ---------- Multi-turn: interleaved tool calls + results ----------

func TestConvertRequest_MultiTurnToolConversation(t *testing.T) {
	fc := NewFormatConverter()

	// Realistic multi-turn: user → assistant(tool_call) → tool_result → assistant(tool_call) → tool_result → user
	openaiReq := `{
		"model": "claude-3-5-sonnet",
		"messages": [
			{"role": "system", "content": "You are a code assistant.\nAlways escape \"special\" chars properly."},
			{"role": "user", "content": "Read the file C:\\Projects\\src\\main.go"},
			{"role": "assistant", "content": "", "tool_calls": [
				{
					"id": "call_1",
					"type": "function",
					"function": {
						"name": "read_file",
						"arguments": "{\"path\":\"C:\\\\Projects\\\\src\\\\main.go\"}"
					}
				}
			]},
			{"role": "tool", "tool_call_id": "call_1", "content": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello \\\"world\\\"\")\n}"},
			{"role": "assistant", "content": "I see the file. Let me also check the test file.", "tool_calls": [
				{
					"id": "call_2",
					"type": "function",
					"function": {
						"name": "read_file",
						"arguments": "{\"path\":\"C:\\\\Projects\\\\src\\\\main_test.go\"}"
					}
				}
			]},
			{"role": "tool", "tool_call_id": "call_2", "content": "package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {\n\t// TODO: add tests\n}"},
			{"role": "user", "content": "Now modify main.go to add a \"greeting\" function"}
		],
		"max_tokens": 4096,
		"tools": [
			{
				"type": "function",
				"function": {
					"name": "read_file",
					"description": "Read a file from disk. Handles paths with backslashes (e.g. C:\\Users\\file.txt).",
					"parameters": {
						"type": "object",
						"properties": {
							"path": {"type": "string", "description": "File path (supports \\\\ escapes)"}
						},
						"required": ["path"]
					}
				}
			}
		]
	}`

	converted, _, err := fc.ConvertRequest([]byte(openaiReq), ProviderTypeAnthropic)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}

	// Must be valid JSON
	var anthropicReq AnthropicRequest
	if err := json.Unmarshal(converted, &anthropicReq); err != nil {
		t.Fatalf("output is not valid JSON: %v\n  body: %s", err, converted)
	}

	// System prompt
	sysStr := ""
	switch s := anthropicReq.System.(type) {
	case string:
		sysStr = s
	case []interface{}:
		for _, b := range s {
			if block, ok := b.(map[string]interface{}); ok {
				if text, ok := block["text"].(string); ok {
					if sysStr != "" {
						sysStr += "\n"
					}
					sysStr += text
				}
			}
		}
	default:
		t.Fatalf("unexpected System type: %T", anthropicReq.System)
	}
	if !strings.Contains(sysStr, "escape \"special\" chars") {
		t.Errorf("system prompt lost quotes: %s", sysStr)
	}

	// Messages: user, assistant(tool_use), user(tool_result), assistant(text+tool_use), user(tool_result), user
	// Note: tool results become role=user in Anthropic format
	if len(anthropicReq.Messages) != 6 {
		t.Fatalf("Messages count = %d, want 6", len(anthropicReq.Messages))
	}

	// Verify tool definition preserved description with special chars
	if len(anthropicReq.Tools) != 1 {
		t.Fatalf("Tools count = %d, want 1", len(anthropicReq.Tools))
	}
	if !strings.Contains(anthropicReq.Tools[0].Description, `C:\Users\file.txt`) {
		t.Errorf("tool description lost backslashes: %s", anthropicReq.Tools[0].Description)
	}

	// Full round-trip
	remarshaled, err := json.Marshal(anthropicReq)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	var roundTrip AnthropicRequest
	if err := json.Unmarshal(remarshaled, &roundTrip); err != nil {
		t.Fatalf("round-trip failed: %v\n  body: %s", err, remarshaled)
	}
}

// ---------- Anthropic stream: message_delta with usage ----------

func TestConvertAnthropicStream_MessageDeltaUsage(t *testing.T) {
	fc := NewFormatConverter()

	sseInput := strings.Join([]string{
		sseEvent(t, AnthropicStreamEvent{
			Type:    "message_start",
			Message: &AnthropicResponse{ID: "msg_usage", Model: "claude-3", Role: "assistant"},
		}),
		sseEvent(t, AnthropicStreamEvent{
			Type:         "content_block_start",
			Index:        0,
			ContentBlock: &AnthropicStreamContentBlock{Type: "text"},
		}),
		sseEvent(t, AnthropicStreamEvent{
			Type:  "content_block_delta",
			Index: 0,
			Delta: &AnthropicStreamDelta{Type: "text_delta", Text: "done"},
		}),
		sseEvent(t, AnthropicStreamEvent{Type: "content_block_stop", Index: 0}),
		sseEvent(t, AnthropicStreamEvent{
			Type:  "message_delta",
			Delta: &AnthropicStreamDelta{StopReason: "end_turn"},
			Usage: &AnthropicStreamUsage{InputTokens: 50, OutputTokens: 10},
		}),
		sseEvent(t, AnthropicStreamEvent{Type: "message_stop"}),
		"",
	}, "\n")

	rec := &flusherRecorder{}
	err := fc.convertAnthropicStream(strings.NewReader(sseInput), rec)
	if err != nil {
		t.Fatalf("convertAnthropicStream failed: %v", err)
	}

	output := rec.buf.String()

	// Find the message_delta chunk (has finish_reason)
	var foundUsage bool
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			continue
		}
		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			t.Fatalf("SSE chunk not valid JSON: %v\n  payload: %s", err, payload)
		}
		if usage, ok := chunk["usage"].(map[string]interface{}); ok {
			foundUsage = true
			if pt, _ := usage["prompt_tokens"].(float64); int(pt) != 50 {
				t.Errorf("prompt_tokens = %v, want 50", pt)
			}
			if ct, _ := usage["completion_tokens"].(float64); int(ct) != 10 {
				t.Errorf("completion_tokens = %v, want 10", ct)
			}
		}
	}
	if !foundUsage {
		t.Error("no usage found in output SSE")
	}
}

// ---------- JSON repair ----------

func TestRepairJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool // expect repaired output to be valid JSON
	}{
		{"already_valid", `{"key":"value"}`, true},
		{"truncated_object", `{"key":"val`, true},
		{"truncated_array", `[1,2,3`, true},
		{"unclosed_string", `{"key":"val`, true},
		{"nested_truncated", `{"a":{"b":[1,2`, true},
		{"empty_object", `{`, true},
		{"empty_array", `[`, true},
		{"deeply_nested", `{"a":{"b":{"c":[{"d":"e`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repaired := repairJSON([]byte(tt.input))
			if tt.valid && !json.Valid(repaired) {
				t.Errorf("repaired JSON is not valid: %s", repaired)
			}
		})
	}
}

// ---------- Streaming: Anthropic tool_use SSE → OpenAI tool_calls ----------

// mustSSEEvent marshals an AnthropicStreamEvent to an SSE data line.
// Panics on marshal failure (test-only).
func mustSSEEvent(event AnthropicStreamEvent) string {
	data, err := json.Marshal(event)
	if err != nil {
		panic("marshal SSE event: " + err.Error())
	}
	return "data: " + string(data)
}

// sseEvent is the *testing.T variant of mustSSEEvent.
func sseEvent(t *testing.T, event AnthropicStreamEvent) string {
	t.Helper()
	return mustSSEEvent(event)
}

func TestConvertAnthropicStream_ToolUse(t *testing.T) {
	fc := NewFormatConverter()

	// Build SSE events programmatically — no manual escaping needed.
	msgModel := "claude-3"
	sseInput := strings.Join([]string{
		sseEvent(t, AnthropicStreamEvent{
			Type:    "message_start",
			Message: &AnthropicResponse{ID: "msg_tool", Model: msgModel, Role: "assistant"},
		}),
		sseEvent(t, AnthropicStreamEvent{
			Type:         "content_block_start",
			Index:        0,
			ContentBlock: &AnthropicStreamContentBlock{Type: "text"},
		}),
		sseEvent(t, AnthropicStreamEvent{
			Type:  "content_block_delta",
			Index: 0,
			Delta: &AnthropicStreamDelta{Type: "text_delta", Text: "Let me search."},
		}),
		sseEvent(t, AnthropicStreamEvent{Type: "content_block_stop", Index: 0}),
		sseEvent(t, AnthropicStreamEvent{
			Type:         "content_block_start",
			Index:        1,
			ContentBlock: &AnthropicStreamContentBlock{Type: "tool_use", ID: "toolu_abc", Name: "search_files"},
		}),
		// partial_json chunks that assemble to: {"query":"hello \"world\"","path":"C:\\Users"}
		sseEvent(t, AnthropicStreamEvent{
			Type:  "content_block_delta",
			Index: 1,
			Delta: &AnthropicStreamDelta{Type: "input_json_delta", PartialJSON: `{"query":"hello`},
		}),
		sseEvent(t, AnthropicStreamEvent{
			Type:  "content_block_delta",
			Index: 1,
			Delta: &AnthropicStreamDelta{Type: "input_json_delta", PartialJSON: ` \"world\""`},
		}),
		sseEvent(t, AnthropicStreamEvent{
			Type:  "content_block_delta",
			Index: 1,
			Delta: &AnthropicStreamDelta{Type: "input_json_delta", PartialJSON: `,"path":"C:\\`},
		}),
		sseEvent(t, AnthropicStreamEvent{
			Type:  "content_block_delta",
			Index: 1,
			Delta: &AnthropicStreamDelta{Type: "input_json_delta", PartialJSON: `Users"}`},
		}),
		sseEvent(t, AnthropicStreamEvent{Type: "content_block_stop", Index: 1}),
		sseEvent(t, AnthropicStreamEvent{
			Type:  "message_delta",
			Delta: &AnthropicStreamDelta{StopReason: "tool_use"},
			Usage: &AnthropicStreamUsage{InputTokens: 10, OutputTokens: 20},
		}),
		sseEvent(t, AnthropicStreamEvent{Type: "message_stop"}),
		"",
	}, "\n")

	rec := &flusherRecorder{}
	err := fc.convertAnthropicStream(strings.NewReader(sseInput), rec)
	if err != nil {
		t.Fatalf("convertAnthropicStream failed: %v", err)
	}

	output := rec.buf.String()

	var (
		foundToolStart bool
		foundToolDelta bool
		foundFinish    bool
		toolArgs       string
	)

	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			continue
		}

		var chunk OpenAIStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			t.Fatalf("SSE chunk not valid JSON: %v\n  payload: %s", err, payload)
		}

		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta

		if len(delta.ToolCalls) > 0 {
			tc := delta.ToolCalls[0]
			if tc.ID == "toolu_abc" && tc.Function.Name == "search_files" {
				foundToolStart = true
			}
			if tc.Function.Arguments != "" {
				foundToolDelta = true
				toolArgs += tc.Function.Arguments
			}
		}

		if chunk.Choices[0].FinishReason != nil && *chunk.Choices[0].FinishReason == "tool_calls" {
			foundFinish = true
		}
	}

	if !foundToolStart {
		t.Error("missing tool_use start chunk with ID and name")
	}
	if !foundToolDelta {
		t.Error("missing input_json_delta chunks")
	}
	if !foundFinish {
		t.Error("missing finish_reason=tool_calls")
	}

	// Verify assembled arguments are valid JSON with special chars
	var args map[string]string
	if err := json.Unmarshal([]byte(toolArgs), &args); err != nil {
		t.Fatalf("assembled tool args not valid JSON: %v\n  args: %s", err, toolArgs)
	}
	if args["query"] != `hello "world"` {
		t.Errorf("query = %q, want %q", args["query"], `hello "world"`)
	}
	if args["path"] != `C:\Users` {
		t.Errorf("path = %q, want %q", args["path"], `C:\Users`)
	}
}

// ---------- Streaming: truncated SSE event recovery ----------

func TestConvertAnthropicStream_TruncatedEvent(t *testing.T) {
	fc := NewFormatConverter()

	// Second event is truncated JSON — should be repaired or skipped gracefully
	sseInput := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_trunc","model":"claude-3","role":"assistant","usage":{"input_tokens":5,"output_tokens":0}}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hel`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"lo"}}`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":5,"output_tokens":2}}`,
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	rec := &flusherRecorder{}
	err := fc.convertAnthropicStream(strings.NewReader(sseInput), rec)
	if err != nil {
		t.Fatalf("convertAnthropicStream failed: %v", err)
	}

	// Should not crash, and should produce valid SSE output
	output := rec.buf.String()
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			continue
		}
		if !json.Valid([]byte(payload)) {
			t.Errorf("invalid JSON in output SSE: %s", payload)
		}
	}
}

// ---------- Benchmark ----------

func BenchmarkConvertAnthropicStream(b *testing.B) {
	fc := NewFormatConverter()

	// Build a realistic SSE payload via struct marshal — no manual escaping.
	var lines []string
	lines = append(lines, mustSSEEvent(AnthropicStreamEvent{
		Type:    "message_start",
		Message: &AnthropicResponse{ID: "msg_bench", Model: "claude-3-5-sonnet", Role: "assistant"},
	}))
	lines = append(lines, mustSSEEvent(AnthropicStreamEvent{
		Type:         "content_block_start",
		Index:        0,
		ContentBlock: &AnthropicStreamContentBlock{Type: "text"},
	}))
	for i := 0; i < 50; i++ {
		lines = append(lines, mustSSEEvent(AnthropicStreamEvent{
			Type:  "content_block_delta",
			Index: 0,
			Delta: &AnthropicStreamDelta{
				Type: "text_delta",
				Text: fmt.Sprintf("chunk %d with \"quotes\" and \\backslash\n", i),
			},
		}))
	}
	lines = append(lines, mustSSEEvent(AnthropicStreamEvent{Type: "content_block_stop", Index: 0}))
	// Add a tool_use block
	lines = append(lines, mustSSEEvent(AnthropicStreamEvent{
		Type:         "content_block_start",
		Index:        1,
		ContentBlock: &AnthropicStreamContentBlock{Type: "tool_use", ID: "toolu_bench", Name: "search"},
	}))
	lines = append(lines, mustSSEEvent(AnthropicStreamEvent{
		Type:  "content_block_delta",
		Index: 1,
		Delta: &AnthropicStreamDelta{Type: "input_json_delta", PartialJSON: `{"q":"test"}`},
	}))
	lines = append(lines, mustSSEEvent(AnthropicStreamEvent{Type: "content_block_stop", Index: 1}))
	lines = append(lines, mustSSEEvent(AnthropicStreamEvent{
		Type:  "message_delta",
		Delta: &AnthropicStreamDelta{StopReason: "tool_use"},
		Usage: &AnthropicStreamUsage{InputTokens: 100, OutputTokens: 200},
	}))
	lines = append(lines, mustSSEEvent(AnthropicStreamEvent{Type: "message_stop"}))
	lines = append(lines, "")

	payload := strings.Join(lines, "\n")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		rec := &flusherRecorder{}
		_ = fc.convertAnthropicStream(strings.NewReader(payload), rec)
	}
}
