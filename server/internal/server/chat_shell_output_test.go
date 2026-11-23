package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestShellOutputBudget(t *testing.T) {
	for _, tool := range []string{"bash", "exec"} {
		t.Run(tool, func(t *testing.T) {
			stdout := strings.Repeat("context\n", 400) + "answer=2100.20\n"
			raw, _ := json.Marshal(map[string]interface{}{"session_id": "sample", "stdout": stdout, "stderr": "末尾证据\n", "exit_code": 0})
			var out map[string]interface{}
			if err := json.Unmarshal([]byte(compactToolResultContentForLLM(tool, string(raw))), &out); err != nil {
				t.Fatal(err)
			}
			if out["stdout"] != stdout || out["stderr"] != "末尾证据\n" {
				t.Error("small output must survive byte-for-byte when the envelope fits")
			}
			long, _ := json.Marshal(map[string]interface{}{"session_id": "sample", "stdout": strings.Repeat("证据\n\"", 16000), "truncated": false, "exit_code": 0})
			content := compactToolResultContentForLLM(tool, string(long))
			if len(content) > maxLLMToolOutputBytes || !utf8.ValidString(content) {
				t.Fatal("invalid envelope budget/UTF-8")
			}
			if err := json.Unmarshal([]byte(content), &out); err != nil {
				t.Fatal("long shell result must remain valid JSON:", err)
			}
			if out["llm_output_truncated"] != true || out["truncated"] != false {
				t.Error("preview truncation must be distinct from buffer loss")
			}
			if _, ok := out["output_retrieval"].(map[string]interface{}); !ok {
				t.Error("missing actionable session recovery")
			}
			calls := []llm.ToolCall{{ID: "one", Name: tool}, {ID: "two", Name: tool}}
			results := []llm.Message{{ToolCallID: "one", Content: string(long)}, {ToolCallID: "two", Content: string(long)}}
			compacted := compactToolResultsForLLM(calls, results)
			for _, r := range compacted {
				var page map[string]interface{}
				if err := json.Unmarshal([]byte(r.Content), &page); err != nil {
					t.Fatal("round budget broke JSON:", err)
				}
				if page["llm_output_truncated"] != true || page["session_id"] != "sample" {
					t.Error("round budget dropped recovery state")
				}
			}
		})
	}
}

func TestShellLogPagingThroughGatewayAndRoundBudget(t *testing.T) {
	sessions := tools.NewSessionRegistry()
	defer sessions.Cleanup()
	want := strings.Repeat("证据\n\"", 9000) + "answer=3582.70\n"
	s := &tools.ProcessSession{ID: "recover", Status: tools.ProcessRunning, Stdout: tools.NewOutputBuffer(200000), Stderr: tools.NewOutputBuffer(200000)}
	s.Stdout.Append(want)
	sessions.Add(s)
	code := 0
	sessions.MarkExited(s.ID, &code, "", tools.ProcessCompleted)
	registry := tools.NewRegistry()
	registry.Register(tools.NewExecTool(tools.ExecConfig{}, sessions, nil, nil, nil))
	gateway := tools.NewToolGateway(registry, tools.NewExecutor(registry))
	var collected strings.Builder
	offset := 0
	for round := 0; round < 300; round++ {
		args, _ := json.Marshal(map[string]interface{}{"action": "log", "session_id": s.ID, "offset": offset, "limit": 16384})
		result, err := gateway.Execute(context.Background(), tools.ToolGatewayRequest{ToolName: "bash", Arguments: string(args), RouteKind: tools.ToolRouteKindChat})
		if err != nil {
			t.Fatal(err)
		}
		content := compactToolResultContentForLLM("bash", result.CompactLLMContent)
		content = recompactToolResultForLLMBudget("bash", content, 2048)
		var page map[string]interface{}
		if err := json.Unmarshal([]byte(content), &page); err != nil {
			t.Fatal(err)
		}
		part, _ := page["output"].(string)
		collected.WriteString(part)
		next, ok := page["next_offset"].(float64)
		if !ok || int(next) != offset+len(part) {
			t.Fatalf("round budget skipped output: %s", content)
		}
		if page["has_more"] == false {
			break
		}
		if int(next) <= offset {
			t.Fatal("page cannot advance")
		}
		offset = int(next)
	}
	if collected.String() != want {
		t.Fatalf("recovered %d bytes, want %d", collected.Len(), len(want))
	}
}
