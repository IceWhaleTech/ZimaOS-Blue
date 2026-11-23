package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProvidersPreserveReadableShellEvidence(t *testing.T) {
	converters := map[string]func(ChatRequest) string{
		"openai": func(r ChatRequest) string { return (&OpenAIProvider{}).convertRequest(r).Messages[0].Content.(string) },
		"claude": func(r ChatRequest) string {
			return (&ClaudeProvider{}).convertRequest(r).Messages[0].Content[0].Content
		},
		"ollama": func(r ChatRequest) string { return (&OllamaProvider{}).convertRequest(r).Messages[0].Content },
	}
	for name, convert := range converters {
		t.Run(name, func(t *testing.T) {
			for _, stream := range []string{"stdout", "stderr", "output"} {
				source := "Project\tCategory\tAmount\nLEDGER_Q7\tHardware\t217.35\nLEDGER_Q7\tService\t82.65\n预算: 325.50\nLiteral JSON: {\"x\":\"a\\tb\"}\n"
				metadata := map[string]interface{}{"exit_code": 3, "status": "completed", "session_id": "page-7", "truncated": false, "llm_output_truncated": true, "offset": 12, "next_offset": 57, "has_more": true, "output_retrieval": map[string]interface{}{"tool": "exec", "action": "log", "session_id": "page-7", "offset": 57}}
				payload := make(map[string]interface{}, len(metadata)+1)
				for k, v := range metadata {
					payload[k] = v
				}
				payload[stream] = source
				raw, _ := json.Marshal(payload)
				request := ChatRequest{Messages: []Message{{Role: RoleTool, ToolName: "bash", ToolCallID: "tc", Content: string(raw)}}}
				got := convert(request)
				if !strings.Contains(got, source) {
					t.Errorf("%s source text remains escaped", stream)
					continue
				}
				if len(got) > len(raw) {
					t.Error("rendering expanded the budgeted envelope")
				}
				var retained map[string]interface{}
				if err := json.Unmarshal([]byte(strings.SplitN(got, "\n", 2)[0]), &retained); err != nil {
					t.Fatal(err)
				}
				want, _ := json.Marshal(metadata)
				actual, _ := json.Marshal(retained)
				if string(actual) != string(want) {
					t.Errorf("execution/recovery metadata changed: %s", actual)
				}
				if request.Messages[0].Content != string(raw) {
					t.Error("rendering mutated audit input")
				}
			}
			for _, msg := range []Message{
				{Role: RoleTool, ToolName: "search", Content: `{"stdout":"a\nb","exit_code":0}`},
				{Role: RoleTool, ToolName: "bash", Content: `{"stdout":`},
				{Role: RoleTool, ToolName: "bash", Content: `{"exit_code":7,"error":"failed"}`},
			} {
				if got := convert(ChatRequest{Messages: []Message{msg}}); got != msg.Content {
					t.Errorf("unrelated/malformed content changed: %q", got)
				}
			}
		})
	}
}

func TestProviderContentLeavesUserJSONUntouched(t *testing.T) {
	m := Message{Role: RoleUser, ToolName: "bash", Content: `{"stdout":"a\nb","exit_code":0}`}
	if m.ProviderContent() != m.Content {
		t.Fatal("user JSON was interpreted as shell output")
	}
}
