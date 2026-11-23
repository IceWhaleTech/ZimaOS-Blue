package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestSkillExecInitialSurfaceBudget(t *testing.T) {
	h := newClarifyNoneSelectionChatHandler(t, nil, nil, tools.NewRegistry())
	h.toolRegistry.Register(tools.NewExecTool(tools.DefaultExecConfig(), tools.NewSessionRegistry(), nil, nil, nil))
	for _, query := range []string{"Search the latest OpenAI Responses API documentation.", "Review this screenshot and audit the UI and accessibility.", "Read release-notes.txt in the local directory and answer with citations from that file."} {
		for _, preview := range []bool{false, true} {
			req := tools.ToolPolicyRequest{RouteKind: tools.ToolRouteKindChat}
			var selection chatToolSurfaceSelection
			if preview {
				selection = h.previewChatToolSurfacesForRequest(context.Background(), query, req, nil, nil)
			} else {
				selection = h.selectChatToolSurfacesForRequest(context.Background(), query, req, nil, nil)
			}
			if names := selectedToolNames(selection.NativeDefs); len(names) != 1 || names[0] != "exec" {
				t.Errorf("query=%q preview=%v: expected exec-only, got %v", query, preview, names)
				continue
			}
			schema, _ := json.Marshal(selection.NativeDefs[0].Parameters)
			if len(schema) > 200 {
				t.Errorf("query=%q preview=%v: command schema is %d bytes, budget 200", query, preview, len(schema))
			}
			props, _ := selection.NativeDefs[0].Parameters["properties"].(map[string]interface{})
			for _, key := range []string{"command", "workdir", "timeout"} {
				if props[key] == nil {
					t.Errorf("missing initial command parameter %s", key)
				}
			}
		}
	}
}

func TestClarificationInitialSurfaceHasNoExecutionTools(t *testing.T) {
	h := newClarifyNoneSelectionChatHandler(t, nil, nil, nil)
	s := h.selectChatToolSurfacesForRequest(context.Background(), clarifyNoneMixedPrompt, tools.ToolPolicyRequest{RouteKind: tools.ToolRouteKindChat}, nil, nil)
	if len(s.NativeDefs) != 0 {
		t.Fatalf("unresolved user choice must clarify without tools: %v", selectedToolNames(s.NativeDefs))
	}
}

func TestRunningExecExposesSessionSchema(t *testing.T) {
	h := newClarifyNoneSelectionChatHandler(t, nil, nil, nil)
	h.toolRegistry.Register(tools.NewExecTool(tools.DefaultExecConfig(), tools.NewSessionRegistry(), nil, nil, nil))
	req := llm.ChatRequest{Tools: []llm.Tool{{Name: "exec", Parameters: map[string]interface{}{"type": "object"}}}}
	got := h.refreshDiscoveredTools(context.Background(), req, []llm.ToolCall{{ID: "run", Name: "exec"}}, []llm.Message{{Role: llm.RoleTool, ToolCallID: "run", Content: `{"status":"running","session_id":"exec-1"}`}}, "running-session", nil, nil)
	for _, tool := range got.Tools {
		if tool.Name == "exec" {
			props, _ := tool.Parameters["properties"].(map[string]interface{})
			if props["action"] == nil || props["session_id"] == nil {
				t.Fatal("running command needs full session schema")
			}
			return
		}
	}
	t.Fatal("missing exec session recovery tool")
}

func TestLocalCitationSourceIsNotWriteTarget(t *testing.T) {
	query := "Read release-notes.txt in the local directory and answer with citations from that file."
	if target := extractRequestedArtifactWriteTarget(query); target != "" {
		t.Fatalf("source was mistaken for output: %q", target)
	}
}

func TestUnknownCapabilityUsesDiscoverySurface(t *testing.T) {
	h := newClarifyNoneSelectionChatHandler(t, nil, nil, nil)
	s := h.selectChatToolSurfacesForRequest(context.Background(), "Handle the frobnicator.", tools.ToolPolicyRequest{RouteKind: tools.ToolRouteKindChat}, nil, nil)
	if names := selectedToolNames(s.NativeDefs); len(names) != 1 || names[0] != "tool_search" {
		t.Fatalf("unknown capability needs discovery before execution, got %v, decision %+v", names, s.SkillDecision)
	}
}

func TestSandboxSkillExecKeepsHostChoiceWithinBudget(t *testing.T) {
	h := newClarifyNoneSelectionChatHandler(t, nil, nil, nil)
	def := tools.NewExecTool(tools.DefaultExecConfig(), tools.NewSessionRegistry(), nil, nil, nil).Definition()
	def.Parameters["properties"].(map[string]interface{})["host"] = map[string]interface{}{"type": "string", "enum": []string{"local", "sandbox"}}
	h.toolRegistry.ExposeDefinition(def)
	for _, preview := range []bool{false, true} {
		req := tools.ToolPolicyRequest{RouteKind: tools.ToolRouteKindChat}
		query := "Search the latest OpenAI Responses API documentation."
		var s chatToolSurfaceSelection
		if preview {
			s = h.previewChatToolSurfacesForRequest(context.Background(), query, req, nil, nil)
		} else {
			s = h.selectChatToolSurfacesForRequest(context.Background(), query, req, nil, nil)
		}
		metrics := tools.MeasureLLMToolSurface(s.NativeDefs)
		if metrics.SchemaBytes > 219 {
			t.Errorf("preview=%v: sandbox command schema %d exceeds 20%% of 1096-byte baseline", preview, metrics.SchemaBytes)
		}
		if len(s.NativeDefs) != 1 {
			t.Fatal("expected exec only")
		}
		props := s.NativeDefs[0].Parameters["properties"].(map[string]interface{})
		host := props["host"].(map[string]interface{})
		choices, _ := json.Marshal(host["enum"])
		if string(choices) != `["local","sandbox"]` {
			t.Fatalf("sandbox host choices lost: %s", choices)
		}
	}
}

func installedFiLensShellQuery(t *testing.T, h *ChatHandler) string {
	t.Helper()
	raw, err := os.ReadFile("../harness/testdata/shell-evidence.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Items []struct {
			ID    string `json:"id"`
			Input struct {
				Goal string `json:"goal"`
			} `json:"input"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	dir := filepath.Join(workspace, ".agents", "skills", "filens")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	const skill = `---
name: filens
description: Use FiLens CLI for local document search. Search a supplied directory directly without listing files; for unknown scanned pages, search concise source-language page markers and use the returned OCR excerpt.
version: "9.0.0"
category: external_cli
invocation: "blue exec command='filens search /path/documents --query budget'"
capability_tags: [read, search, summarize, answer, document, documents, report, pdf, docx, xlsx, pptx, archive, ocr, 回答, 读取, 搜索, 总结, 文件, 文档, 报告, 扫描, 合同, 预算, 金额]
interaction_mode: stateless
card_support: none
---
Use installed filens for local document search.
`
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skill), 0o644); err != nil {
		t.Fatal(err)
	}
	h.SetSkillSelector(agentcore.NewSkillSelector(workspace, nil))
	for _, item := range data.Items {
		if item.ID == "shell-json-query-with-installed-filens" {
			return item.Input.Goal
		}
	}
	t.Fatal("missing original shell query")
	return ""
}

func TestWeakInstalledSkillMatchPreservesTools(t *testing.T) {
	h := newClarifyNoneSelectionChatHandler(t, nil, nil, nil)
	query := installedFiLensShellQuery(t, h)
	decision, ok := h.resolveSkillDecision(context.Background(), query)
	if !ok || decision.Reason != "ir_ranked" || !decision.NeedClarify || len(decision.ConflictFlags) != 0 {
		t.Fatalf("fixture must reproduce weak, conflict-free skill match: %+v", decision)
	}
	for _, preview := range []bool{false, true} {
		req := tools.ToolPolicyRequest{RouteKind: tools.ToolRouteKindChat}
		var selection chatToolSurfaceSelection
		if preview {
			selection = h.previewChatToolSurfacesForRequest(context.Background(), query, req, nil, nil)
		} else {
			selection = h.selectChatToolSurfacesForRequest(context.Background(), query, req, nil, nil)
		}
		if len(selection.NativeDefs) == 0 || selection.NativeMode == chatNativeToolSurfaceModeClarifyNone {
			t.Errorf("preview=%v: weak skill match suppressed all tools: %+v", preview, selection)
		}
	}
}

func TestDiscoverFirstKeepsUserAndRiskClarifications(t *testing.T) {
	for _, decision := range []agentcore.Decision{
		{SelectedSkill: "filens", NeedClarify: true, Reason: "ir_ranked", ConflictFlags: []string{"mixed_intent"}},
		{SelectedSkill: "filens", Query: "Delete all production data", NeedClarify: true, Reason: "ir_ranked"},
		{SelectedSkill: "filens", NeedClarify: true, Reason: "reranker_requires_choice"},
	} {
		if !shouldUseSkillDecisionForDiscoverFirst(decision) {
			t.Errorf("discarded explicit or high-risk clarification: %+v", decision)
		}
	}
}
