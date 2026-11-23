package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestExecSessionLogPagination(t *testing.T) {
	for _, finished := range []bool{false, true} {
		sessions := NewSessionRegistry()
		s := &ProcessSession{ID: "paged", Status: ProcessRunning, Stdout: NewOutputBuffer(4096), Stderr: NewOutputBuffer(4096)}
		want := strings.Repeat("证据\n", 40) + "answer=3582.70\n"
		s.Stdout.Append(want)
		s.Stderr.Append("stderr证据\n")
		sessions.Add(s)
		if finished {
			code := 0
			sessions.MarkExited(s.ID, &code, "", ProcessCompleted)
		}
		tool := NewProcessTool(sessions)
		offset := 0
		var collected strings.Builder
		for round := 0; round < 100; round++ {
			raw, err := tool.Execute(context.Background(), map[string]interface{}{"action": "log", "session_id": s.ID, "offset": offset, "limit": 17})
			if err != nil {
				t.Fatal(err)
			}
			var page map[string]interface{}
			if err := json.Unmarshal([]byte(raw.(string)), &page); err != nil {
				t.Fatal(err)
			}
			part, _ := page["output"].(string)
			if !utf8.ValidString(part) || len(part) > 17 {
				t.Fatalf("invalid page: %q", part)
			}
			collected.WriteString(part)
			next, ok := page["next_offset"].(float64)
			if !ok || int(next) != offset+len(part) {
				t.Fatalf("invalid next offset: %#v", page)
			}
			if page["has_more"] == false {
				break
			}
			if int(next) <= offset {
				t.Fatal("pagination must advance")
			}
			offset = int(next)
		}
		if collected.String() != want+"stderr证据\n" {
			t.Fatal("pagination lost or duplicated evidence")
		}
		for _, bad := range []map[string]interface{}{{"offset": -1}, {"offset": 1}, {"limit": -1}, {"offset": "bad"}} {
			bad["action"] = "log"
			bad["session_id"] = s.ID
			if _, err := tool.Execute(context.Background(), bad); err == nil {
				t.Errorf("invalid page arguments accepted: %#v", bad)
			}
		}
		sessions.Cleanup()
	}
}

func TestGatewayShellOutputIsRecoverableJSON(t *testing.T) {
	raw, _ := json.Marshal(map[string]interface{}{"session_id": "sample", "stdout": strings.Repeat("证据\n", 18000), "truncated": false})
	for _, name := range []string{"bash", "exec"} {
		out := serializeToolPayload(compactToolPayloadForLLM(context.Background(), ToolGatewayRequest{}, name, string(raw), string(raw)))
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(out), &payload); err != nil {
			t.Fatal("gateway broke shell JSON:", err)
		}
		if payload["llm_output_truncated"] != true || payload["session_id"] != "sample" {
			t.Error("gateway dropped recovery state")
		}
	}
}

func TestShellOutputTinyBudgetsKeepSession(t *testing.T) {
	for _, budget := range []int{64, 128, 320, 1024, 8192} {
		content := CompactExecOutputForLLM(map[string]interface{}{"session_id": "sample", "exit_code": 0, "truncated": false, "stdout": strings.Repeat("证据", 9000)}, budget)
		var out map[string]interface{}
		if len(content) > budget || !utf8.ValidString(content) {
			t.Fatal("invalid budget/encoding")
		}
		if err := json.Unmarshal([]byte(content), &out); err != nil {
			t.Fatal(err)
		}
		if out["session_id"] != "sample" || out["llm_output_truncated"] != true {
			t.Errorf("budget %d lost session or incomplete state: %s", budget, content)
		}
	}
}

func TestExecPreviewNamesRecoveryTool(t *testing.T) {
	content := CompactExecOutputForLLM(map[string]interface{}{"stdout": strings.Repeat("context", 2000), "session_id": "retained"}, 8192)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		t.Fatal(err)
	}
	recovery, _ := result["output_retrieval"].(map[string]interface{})
	if recovery["tool"] != "exec" {
		t.Fatalf("recovery must name callable exec, got %v", recovery)
	}
}

func TestExecSkillDuplicateDataKeepsSourceAndRecoverableOutput(t *testing.T) {
	sessions := NewSessionRegistry()
	defer sessions.Cleanup()
	tool := NewExecTool(DefaultExecConfig(), sessions, nil, nil, nil)
	const headline = "测试要闻甲：发布前完成双人复核。"
	source := headline + "\n" + strings.Repeat("来源正文证据内容\n", 1200) + "FINAL_FINDING=证据末尾\n"
	tool.SetSkillExecutor(func(context.Context, string, map[string]any) (map[string]string, error) {
		return map[string]string{"content": source, "url": "https://www.xinhuanet.com/test-news.html"}, nil
	})
	raw, err := tool.Execute(context.Background(), map[string]interface{}{"command": "blue web_query input=\"https://www.xinhuanet.com/\""})
	if err != nil {
		t.Fatal(err)
	}
	preview := serializeToolPayload(compactToolPayloadForLLM(context.Background(), ToolGatewayRequest{}, "exec", raw, raw.(string)))
	if !strings.Contains(preview, headline) {
		t.Errorf("gateway discarded source text: %s", preview)
	}
	var envelope map[string]interface{}
	if err := json.Unmarshal([]byte(preview), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(preview) > maxToolGatewayLLMStringBytes || !utf8.ValidString(preview) {
		t.Fatal("invalid preview budget or encoding")
	}
	id, _ := envelope["session_id"].(string)
	var collected strings.Builder
	offset := 0
	for i := 0; i < 20; i++ {
		page, err := tool.Execute(context.Background(), map[string]interface{}{"action": "log", "session_id": id, "offset": offset, "limit": 16384})
		if err != nil {
			t.Fatalf("omitted skill output must be retrievable: %v", err)
		}
		var recovered map[string]interface{}
		if err := json.Unmarshal([]byte(page.(string)), &recovered); err != nil {
			t.Fatal(err)
		}
		collected.WriteString(recovered["output"].(string))
		if recovered["has_more"] == false {
			break
		}
		next := int(recovered["next_offset"].(float64))
		if next <= offset {
			t.Fatal("retrieval did not advance")
		}
		offset = next
	}
	if !strings.Contains(collected.String(), "FINAL_FINDING=证据末尾") {
		t.Fatal("retained skill output lost source tail")
	}
}
