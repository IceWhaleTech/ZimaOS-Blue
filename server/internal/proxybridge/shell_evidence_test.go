package proxybridge

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func TestBridgePreservesReadableShellEvidence(t *testing.T) {
	source := "Project\tCategory\tAmount\nLEDGER_Q7\tHardware\t217.35\nLEDGER_Q7\tService\t82.65\n"
	raw, _ := json.Marshal(map[string]interface{}{"stdout": source, "stderr": "partial\n", "exit_code": 3, "session_id": "page-7", "has_more": true})
	for name, marshal := range map[string]func(llm.ChatRequest) ([]byte, error){"chat": MarshalChatRequest, "responses": MarshalResponsesRequest} {
		t.Run(name, func(t *testing.T) {
			request := llm.ChatRequest{Model: "test", Messages: []llm.Message{{Role: llm.RoleTool, ToolName: "exec", ToolCallID: "tc", Content: string(raw)}}}
			wire, err := marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			var body map[string]interface{}
			if err = json.Unmarshal(wire, &body); err != nil {
				t.Fatal(err)
			}
			var output string
			if name == "chat" {
				output = body["messages"].([]interface{})[0].(map[string]interface{})["content"].(string)
			} else {
				output = body["input"].([]interface{})[0].(map[string]interface{})["output"].(string)
			}
			if !strings.Contains(output, source) {
				t.Fatal("source rows are still escaped at provider boundary")
			}
			if !strings.Contains(output, `"exit_code":3`) || !strings.Contains(output, `"session_id":"page-7"`) {
				t.Fatal("execution state lost")
			}
			if len(output) > len(raw) {
				t.Fatal("rendering exceeded budgeted envelope")
			}
		})
	}
}
