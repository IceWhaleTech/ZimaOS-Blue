package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func TestPseudoXMLToolTagCandidates_UsesNativeDocumentToolsByDefault(t *testing.T) {
	candidates := pseudoXMLToolTagCandidates(nil)
	set := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		set[candidate] = struct{}{}
	}

	for _, required := range []string{"docx", "xlsx", "pptx"} {
		if _, ok := set[required]; !ok {
			t.Fatalf("expected %q in default pseudo tool candidates, got=%v", required, candidates)
		}
	}
	if _, ok := set["office"]; ok {
		t.Fatalf("did not expect legacy office pseudo tool candidate, got=%v", candidates)
	}
}

func TestRecoverPseudoToolCallsFromContent_RecoversTypelessPDFArgumentBlock(t *testing.T) {
	content := "继续用 PDF 工具创建：\n\n```typeless\n" +
		"{\"action\":\"create\",\"output_path\":\"reports/qwen36_growth.pdf\",\"title\":\"Qwen3.6 Benchmark 大幅增长分析\",\"markdown\":\"# Qwen3.6 Benchmark 大幅增长分析\\n\\n- SkillsBench Avg5: +552%\\n- NL2Repo: +43%\"}\n" +
		"```\n\n这次直接生成杂志版。"

	calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "pdf"}})
	if !ok {
		t.Fatal("expected typeless pdf argument block recovery to succeed")
	}
	if len(calls) != 1 {
		t.Fatalf("recovered calls = %d, want 1", len(calls))
	}
	if calls[0].Name != "pdf" {
		t.Fatalf("call name = %q, want pdf", calls[0].Name)
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
		t.Fatalf("unmarshal arguments: %v", err)
	}
	if got, _ := args["action"].(string); got != "create" {
		t.Fatalf("action = %q, want create", got)
	}
	if got, _ := args["output_path"].(string); got != "reports/qwen36_growth.pdf" {
		t.Fatalf("output_path = %q, want reports/qwen36_growth.pdf", got)
	}
	if got, _ := args["title"].(string); got != "Qwen3.6 Benchmark 大幅增长分析" {
		t.Fatalf("title = %q, want Qwen3.6 Benchmark 大幅增长分析", got)
	}
	if got, _ := args["markdown"].(string); got == "" {
		t.Fatal("expected markdown payload to be preserved")
	}
}

func TestPseudoJSONToolCallStartIndex_RecoversTypelessPDFArgumentBlock(t *testing.T) {
	delta := "继续用 PDF 工具创建：\n\n```typeless\n" +
		"{\"action\":\"create\",\"output_path\":\"reports/qwen36_growth.pdf\",\"title\":\"Qwen3.6 Benchmark 大幅增长分析\",\"markdown\":\"# Qwen3.6 Benchmark 大幅增长分析\\n\\n- SkillsBench Avg5: +552%\"}\n" +
		"```\n\n这次直接生成杂志版。"
	want := strings.Index(delta, `{"action":"create"`)
	if got := pseudoJSONToolCallStartIndex(delta, []llm.Tool{{Name: "pdf"}}); got != want {
		t.Fatalf("pseudoJSONToolCallStartIndex() = %d, want %d", got, want)
	}
}

func TestRecoverPseudoToolCallsFromContent_RecoversExactTypelessPDFMagazinePayload(t *testing.T) {
	content := "现在用 PDF 原生工具生成：\n```typeless\n" +
		"{\"action\":\"create\",\"outputPath\":\"qwen36_ppt/Qwen3.6_Benchmark大幅增长分析_杂志版.pdf\",\"styleHint\":\"杂志版PDF样式，双栏排版，强调大幅增长数据，高亮关键数字，用深紫色标题栏，引用来源脚注\",\"title\":\"Qwen3.6 Benchmark 大幅增长分析\",\"theme\":\"magazine\"}\n" +
		"```\n这次我用 pdf 工具直接创建杂志版 PDF。"

	calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "pdf"}})
	if !ok {
		t.Fatal("expected exact typeless pdf magazine payload recovery to succeed")
	}
	if len(calls) != 1 {
		t.Fatalf("recovered calls = %d, want 1", len(calls))
	}
	if calls[0].Name != "pdf" {
		t.Fatalf("call name = %q, want pdf", calls[0].Name)
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
		t.Fatalf("unmarshal arguments: %v", err)
	}
	if got, _ := args["action"].(string); got != "create" {
		t.Fatalf("action = %q, want create", got)
	}
	if got, _ := args["outputPath"].(string); got != "qwen36_ppt/Qwen3.6_Benchmark大幅增长分析_杂志版.pdf" {
		t.Fatalf("outputPath = %q, want exact camelCase payload", got)
	}
	if got, _ := args["title"].(string); got != "Qwen3.6 Benchmark 大幅增长分析" {
		t.Fatalf("title = %q, want Qwen3.6 Benchmark 大幅增长分析", got)
	}
	if got, _ := args["styleHint"].(string); !strings.Contains(got, "杂志版PDF样式") {
		t.Fatalf("styleHint = %q, want original typeless payload preserved", got)
	}
}

func TestPseudoJSONToolCallStartIndex_RecoversExactTypelessPDFMagazinePayload(t *testing.T) {
	delta := "现在用 PDF 原生工具生成：\n```typeless\n" +
		"{\"action\":\"create\",\"outputPath\":\"qwen36_ppt/Qwen3.6_Benchmark大幅增长分析_杂志版.pdf\",\"styleHint\":\"杂志版PDF样式，双栏排版，强调大幅增长数据，高亮关键数字，用深紫色标题栏，引用来源脚注\",\"title\":\"Qwen3.6 Benchmark 大幅增长分析\",\"theme\":\"magazine\"}\n" +
		"```\n这次我用 pdf 工具直接创建杂志版 PDF。"
	want := strings.Index(delta, `{"action":"create"`)
	if got := pseudoJSONToolCallStartIndex(delta, []llm.Tool{{Name: "pdf"}}); got != want {
		t.Fatalf("pseudoJSONToolCallStartIndex() = %d, want %d", got, want)
	}
}

func TestRecoverPseudoToolCallsFromContent_RecoversTypelessSingleInputNativeDocumentCreate(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		seedPath string
	}{
		{name: "docx", toolName: "docx", seedPath: "reports/seed.md"},
		{name: "xlsx", toolName: "xlsx", seedPath: "reports/seed.md"},
		{name: "pptx", toolName: "pptx", seedPath: "decks/seed.md"},
		{name: "pdf", toolName: "pdf", seedPath: "reports/seed.md"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			content := "继续创建文档：\n\n```typeless\n" +
				"{\"action\":\"create\",\"path\":\"" + tc.seedPath + "\",\"theme\":\"editorial\"}\n" +
				"```\n"

			calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: tc.toolName}})
			if !ok {
				t.Fatalf("expected typeless single-input %s recovery to succeed", tc.toolName)
			}
			if len(calls) != 1 {
				t.Fatalf("recovered calls = %d, want 1", len(calls))
			}
			if calls[0].Name != tc.toolName {
				t.Fatalf("call name = %q, want %s", calls[0].Name, tc.toolName)
			}

			var args map[string]interface{}
			if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
				t.Fatalf("unmarshal arguments: %v", err)
			}
			if got, _ := args["action"].(string); got != "create" {
				t.Fatalf("action = %q, want create", got)
			}
			if got, _ := args["path"].(string); got != tc.seedPath {
				t.Fatalf("path = %q, want %s", got, tc.seedPath)
			}
		})
	}
}

func TestRecoverPseudoToolCallsFromContent_RecoversRequestToolEnvelopeForComputerUse(t *testing.T) {
	content := `request tool=computer_use args="\"{\\\"action\\\":\\\"message\\\",\\\"app_name\\\":\\\"飞书\\\",\\\"conversation\\\":\\\"后端之家\\\",\\\"intent\\\":\\\"send_greeting\\\",\\\"value\\\":\\\"嗨！我是 blue，这是从我的 ZimaOS Blue 助手发来的招呼信息\\\",\\\"submit\\\":true}\"" count=1 msg_index=65 total_msgs=67`

	calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "computer_use"}})
	if !ok {
		t.Fatal("expected request tool envelope recovery to succeed")
	}
	if len(calls) != 1 {
		t.Fatalf("recovered calls = %d, want 1", len(calls))
	}
	if calls[0].Name != "computer_use" {
		t.Fatalf("call name = %q, want computer_use", calls[0].Name)
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
		t.Fatalf("unmarshal arguments: %v", err)
	}
	if got, _ := args["action"].(string); got != "message" {
		t.Fatalf("action = %q, want message", got)
	}
	if got, _ := args["app_name"].(string); got != "飞书" {
		t.Fatalf("app_name = %q, want 飞书", got)
	}
	if got, _ := args["conversation"].(string); got != "后端之家" {
		t.Fatalf("conversation = %q, want 后端之家", got)
	}
	if got, _ := args["intent"].(string); got != "send_greeting" {
		t.Fatalf("intent = %q, want send_greeting", got)
	}
	if got, _ := args["value"].(string); !strings.Contains(got, "ZimaOS Blue 助手发来的招呼信息") {
		t.Fatalf("value = %q, want greeting payload preserved", got)
	}
	if got, _ := args["submit"].(bool); !got {
		t.Fatalf("submit = %v, want true", args["submit"])
	}
}

func TestPseudoJSONToolCallStartIndex_RecoversTypelessSingleInputNativeDocumentCreate(t *testing.T) {
	delta := "继续创建 DOCX：\n\n```typeless\n" +
		"{\"action\":\"create\",\"path\":\"reports/seed.md\",\"theme\":\"editorial\"}\n" +
		"```\n"
	want := strings.Index(delta, `{"action":"create"`)
	if got := pseudoJSONToolCallStartIndex(delta, []llm.Tool{{Name: "docx"}}); got != want {
		t.Fatalf("pseudoJSONToolCallStartIndex() = %d, want %d", got, want)
	}
}
