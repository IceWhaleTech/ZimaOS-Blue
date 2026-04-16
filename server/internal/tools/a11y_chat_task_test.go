package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	a11yruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

type a11yChatGrounderStub struct {
	results map[string]a11yChatGroundingResult
	errs    map[string]error
	calls   []a11yChatGroundingRequest
}

func (s *a11yChatGrounderStub) Ground(ctx context.Context, req a11yChatGroundingRequest) (a11yChatGroundingResult, error) {
	s.calls = append(s.calls, req)
	if err := s.errs[string(req.TaskHint)]; err != nil {
		return a11yChatGroundingResult{}, err
	}
	if result, ok := s.results[string(req.TaskHint)]; ok {
		return result, nil
	}
	return a11yChatGroundingResult{}, nil
}

func TestA11yToolExecute_MessageIncludesChatTaskMetadata(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [list_item] \"Orca\"\n@2 [document]\n@3 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-orca-conversation",
					2: "token-editor",
					3: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.SetChatGrounder(&a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskLocateComposer): {
				Source: "vision_model",
				Candidates: []a11yChatGroundingCandidate{
					{Role: "composer", Label: "Message", Confidence: 0.98},
				},
			},
			string(a11yChatGroundingTaskVerifySend): {
				Source: "vision_model",
				Verification: map[string]interface{}{
					"status": "sent",
				},
			},
		},
	})

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "message",
		"app_name":     "Feishu",
		"conversation": "Orca",
		"value":        "hello",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["stage"] != "verify_outcome" {
		t.Fatalf("stage = %v, want verify_outcome", out["stage"])
	}
	if _, ok := out["strategy"].(string); !ok {
		t.Fatalf("strategy = %#v, want string", out["strategy"])
	}
	if got := int(out["attempt_count"].(float64)); got < 4 {
		t.Fatalf("attempt_count = %d, want >= 4", got)
	}
	if out["grounding_source"] != "vision_model" {
		t.Fatalf("grounding_source = %v, want vision_model", out["grounding_source"])
	}
	if _, ok := out["verification"].(map[string]interface{}); !ok {
		t.Fatalf("verification = %#v, want object", out["verification"])
	}
	if stages, ok := out["task_stages"].([]interface{}); !ok || len(stages) < 4 {
		t.Fatalf("task_stages = %#v, want >= 4 entries", out["task_stages"])
	}
}

func TestA11yToolExecute_MessageFailsClosedWhenGrounderRejectsComposer(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [list_item] \"Orca\"\n@2 [document]\n@3 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-orca-conversation",
					2: "token-editor",
					3: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.SetChatGrounder(&a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskLocateComposer): {
				Source: "vision_model",
				Candidates: []a11yChatGroundingCandidate{
					{Role: "search_field", Label: "Search", Confidence: 0.99, RationaleTags: []string{"search_field"}},
				},
				Verification: map[string]interface{}{
					"rejected": true,
					"reason":   "search_field_detected",
				},
			},
		},
	})

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "message",
		"app_name":     "Feishu",
		"conversation": "Orca",
		"value":        "hello",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 1 || backend.actTypeHistory[0] != "click" {
		t.Fatalf("actTypeHistory = %#v, want only conversation click before fail-closed", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["phase"] != "composer" {
		t.Fatalf("phase = %v, want composer", out["phase"])
	}
	if out["failure_code"] != "composer_not_confirmed" {
		t.Fatalf("failure_code = %v, want composer_not_confirmed", out["failure_code"])
	}
	if out["grounding_source"] != "vision_model" {
		t.Fatalf("grounding_source = %v, want vision_model", out["grounding_source"])
	}
}

func TestA11yToolExecute_MessageReportsSearchBoxStillActiveFailureCode(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [list_item] \"Orca\"\n@2 [document]\n@3 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-orca-conversation",
					2: "token-editor",
					3: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Orca\"\n@2 [document]\n@3 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-search",
					2: "token-editor",
					3: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "message",
		"app_name":     "Feishu",
		"conversation": "Orca",
		"value":        "hello",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["failure_code"] != "search_box_still_active" {
		t.Fatalf("failure_code = %v, want search_box_still_active", out["failure_code"])
	}
}

func TestA11yToolExecute_MessageVisualFallbackWritesTraceArtifacts(t *testing.T) {
	prev := a11yLocateConversationVisualHitFromPNG
	a11yLocateConversationVisualHitFromPNG = func(ctx context.Context, imagePNG []byte, selectorName string) (a11yConversationVisualHit, error) {
		return a11yConversationVisualHit{
			Point:      a11yruntime.NormalizedPoint{X: 0.61, Y: 0.34},
			Confidence: 0.91,
		}, nil
	}
	defer func() { a11yLocateConversationVisualHitFromPNG = prev }()

	tmpDir := t.TempDir()
	ctx := WithFSScope(context.Background(), []string{tmpDir}, map[string]string{"workspace": tmpDir})

	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [group] \"Sidebar\"",
				RefMap: map[int]string{
					1: "token-sidebar",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
		screenshotGroundingBytes: []byte("fake-grounding-bytes"),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(ctx, map[string]interface{}{
		"action":       "message",
		"app_name":     "Feishu,飞书,Lark",
		"conversation": "Orca",
		"value":        "hello",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	artifactPaths, ok := out["artifact_paths"].([]interface{})
	if !ok || len(artifactPaths) == 0 {
		t.Fatalf("artifact_paths = %#v, want non-empty array", out["artifact_paths"])
	}
	for _, rawPath := range artifactPaths {
		rel := rawPath.(string)
		abs := filepath.Join(tmpDir, rel)
		if _, statErr := os.Stat(abs); statErr != nil {
			t.Fatalf("artifact %q not found: %v", abs, statErr)
		}
	}
}

func TestA11yChatStageMemory_ReordersConversationPlansFromSuccessfulStrategy(t *testing.T) {
	memory := newA11yChatStageMemory()
	memory.Remember("darwin", "feishu_lark", "message", string(a11yChatStageLocateConversation), "structured_search")

	plans := []a11yConversationSearchPlan{
		{Name: "quick_switcher", Open: [][]string{{"command", "k"}}},
		{Name: "structured_search", Open: [][]string{{"command", "f"}, {"command", "f"}}},
	}
	got := memory.OrderConversationPlans("darwin", "feishu_lark", "message", plans)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].Name != "structured_search" {
		t.Fatalf("got[0].Name = %q, want structured_search", got[0].Name)
	}
	if plans[0].Name != "quick_switcher" {
		t.Fatalf("plans mutated = %#v, want original ordering preserved", plans)
	}
}

func TestA11yToolExecute_MessageEmitsChatStageEvents(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [list_item] \"Orca\"\n@2 [document]\n@3 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-orca-conversation",
					2: "token-editor",
					3: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.SetChatGrounder(&a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskLocateComposer): {
				Source: "vision_model",
				Candidates: []a11yChatGroundingCandidate{
					{Role: "composer", Label: "Message", Confidence: 0.98},
				},
			},
			string(a11yChatGroundingTaskVerifySend): {
				Source: "vision_model",
				Verification: map[string]interface{}{
					"status": "sent",
				},
			},
		},
	})

	var events []ToolEvent
	ctx := WithEventEmitter(context.Background(), func(_ context.Context, event ToolEvent) error {
		events = append(events, event)
		return nil
	})

	raw, err := tool.Execute(ctx, map[string]interface{}{
		"action":       "message",
		"app_name":     "Feishu",
		"conversation": "Orca",
		"value":        "hello",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if raw == nil {
		t.Fatal("expected non-nil tool output")
	}
	if len(events) < 4 {
		t.Fatalf("events = %#v, want >= 4 chat stage events", events)
	}

	foundVerify := false
	for _, event := range events {
		if event.Type != "stage_changed" {
			t.Fatalf("event.Type = %q, want stage_changed", event.Type)
		}
		if event.ToolName != "computer_use" {
			t.Fatalf("event.ToolName = %q, want computer_use", event.ToolName)
		}
		stage, _ := event.Payload["stage"].(string)
		if stage == "computer_use_chat.verify_outcome" {
			foundVerify = true
			if status, _ := event.Payload["status"].(string); status != "ok" {
				t.Fatalf("verify_outcome status = %q, want ok", status)
			}
		}
	}
	if !foundVerify {
		t.Fatalf("events = %#v, want verify_outcome stage", events)
	}
}
