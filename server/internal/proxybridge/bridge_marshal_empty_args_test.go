package proxybridge

import (
	"encoding/json"
	"testing"
)

func TestNormalizeInboundBridgeToolCall_EmptyArgumentsStayEmpty(t *testing.T) {
	_, _, arguments := normalizeInboundBridgeToolCall("call_1", "file_write", json.RawMessage(""), false, nil)
	if arguments != "" {
		t.Fatalf("arguments = %q, want empty string", arguments)
	}
}

func TestParseSSEChunk_ResponsesFunctionCallDoneKeepsEmptyArguments(t *testing.T) {
	payload := `{"type":"response.output_item.done","response_id":"resp_empty_done","item":{"type":"function_call","call_id":"call_1","name":"file_write","arguments":""}}`

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
	if chunk.ToolCalls[0].Arguments != "" {
		t.Fatalf("tool call arguments = %q, want empty string", chunk.ToolCalls[0].Arguments)
	}
}
