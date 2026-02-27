package proxybridge

import "testing"

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
