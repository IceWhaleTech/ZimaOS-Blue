package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	a11yruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

type a11yChatGrounderStub struct {
	results         map[string]a11yChatGroundingResult
	resultSequences map[string][]a11yChatGroundingResult
	errs            map[string]error
	calls           []a11yChatGroundingRequest
}

func (s *a11yChatGrounderStub) Ground(ctx context.Context, req a11yChatGroundingRequest) (a11yChatGroundingResult, error) {
	s.calls = append(s.calls, req)
	if err := s.errs[string(req.TaskHint)]; err != nil {
		return a11yChatGroundingResult{}, err
	}
	if sequence := s.resultSequences[string(req.TaskHint)]; len(sequence) > 0 {
		result := sequence[0]
		s.resultSequences[string(req.TaskHint)] = sequence[1:]
		return result, nil
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
	if out["stage"] != "type_or_send" {
		t.Fatalf("stage = %v, want type_or_send when submit evidence already confirms success", out["stage"])
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
	if out["app_profile"] != "feishu_lark" {
		t.Fatalf("app_profile = %v, want feishu_lark", out["app_profile"])
	}
	if _, ok := out["verification"].(map[string]interface{}); !ok {
		t.Fatalf("verification = %#v, want object", out["verification"])
	}
	if stages, ok := out["task_stages"].([]interface{}); !ok || len(stages) < 4 {
		t.Fatalf("task_stages = %#v, want >= 4 entries", out["task_stages"])
	}
}

func TestA11yToolExecute_MessageDraftSkipsOutcomeGroundingWhenSubmitFalse(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"Message\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
		structuredSnapshot: a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
			WindowID: "win-feishu",
			Title:    "Feishu",
			Mode:     "ax",
		}, &a11yruntime.Node{
			Role: "window",
			Name: "Feishu",
			Children: []*a11yruntime.Node{
				{
					Token:       "token-editor",
					Role:        "document",
					Name:        "Message",
					Description: "focused editable",
					Interactive: true,
				},
				{
					Token:       "token-send",
					Role:        "button",
					Name:        "Send",
					Interactive: true,
				},
			},
		}),
		screenshotGroundingBytes: []byte("verify-draft-grounding"),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	grounder := &a11yChatGrounderStub{}
	tool.SetChatGrounder(grounder)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "message",
		"app_name": "Feishu",
		"value":    "hello",
		"submit":   false,
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["stage"] != "type_or_send" {
		t.Fatalf("stage = %v, want type_or_send when draft confirmation completes in type_or_send", out["stage"])
	}
	if out["strategy"] != "draft_confirmation" {
		t.Fatalf("strategy = %v, want draft_confirmation when submit is false", out["strategy"])
	}
	verification, ok := out["verification"].(map[string]interface{})
	if !ok {
		t.Fatalf("verification = %#v, want object", out["verification"])
	}
	if verification["status"] != "drafted" {
		t.Fatalf("verification.status = %v, want drafted", verification["status"])
	}
	if out["failure_code"] != nil {
		t.Fatalf("failure_code = %v, want nil", out["failure_code"])
	}
	if backend.lastGroundingScreenshotWindow != "" {
		t.Fatalf("lastGroundingScreenshotWindow = %q, want empty for draft flow without outcome grounding", backend.lastGroundingScreenshotWindow)
	}
	if len(grounder.calls) != 0 {
		t.Fatalf("grounder.calls = %#v, want no verify_draft grounding calls", grounder.calls)
	}
	stages, ok := out["task_stages"].([]interface{})
	if !ok || len(stages) == 0 {
		t.Fatalf("task_stages = %#v, want chat stage trace", out["task_stages"])
	}
	for _, rawStage := range stages {
		stage, _ := rawStage.(map[string]interface{})
		if stage["stage"] == "verify_outcome" {
			t.Fatalf("task_stages = %#v, do not want verify_outcome stage for draft flow", out["task_stages"])
		}
	}
}

func TestA11yToolExecute_MessageWaitsForSendVerificationWhenVisualStatusAppearsLate(t *testing.T) {
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
		screenshotGroundingBytes: []byte("verify-send-grounding"),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	grounder := &a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskLocateComposer): {
				Source: "vision_model",
				Candidates: []a11yChatGroundingCandidate{
					{Role: "composer", Label: "Message", Confidence: 0.98},
				},
			},
		},
		resultSequences: map[string][]a11yChatGroundingResult{
			string(a11yChatGroundingTaskVerifySend): {
				{
					Source: "vision_model",
					Verification: map[string]interface{}{
						"status": "pending",
					},
				},
				{
					Source: "vision_model",
					Verification: map[string]interface{}{
						"status": "sent",
					},
				},
			},
		},
	}
	tool.SetChatGrounder(grounder)

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
	if out["failure_code"] != nil {
		t.Fatalf("failure_code = %v, want nil", out["failure_code"])
	}
	verification, ok := out["verification"].(map[string]interface{})
	if !ok {
		t.Fatalf("verification = %#v, want object", out["verification"])
	}
	if verification["status"] != "sent" {
		t.Fatalf("verification.status = %v, want sent", verification["status"])
	}
	if len(grounder.calls) < 3 {
		t.Fatalf("grounder.calls = %#v, want repeated verify_send attempts", grounder.calls)
	}
}

func TestA11yToolExecute_MessageAcceptsAppAwareDeliveredSendVerificationStatus(t *testing.T) {
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
		screenshotGroundingBytes: []byte("verify-send-grounding"),
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
					"status": "delivered",
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
	if out["failure_code"] != nil {
		t.Fatalf("failure_code = %v, want nil", out["failure_code"])
	}
	verification, ok := out["verification"].(map[string]interface{})
	if !ok {
		t.Fatalf("verification = %#v, want object", out["verification"])
	}
	if verification["status"] != "delivered" {
		t.Fatalf("verification.status = %v, want delivered", verification["status"])
	}
}

func TestA11yToolExecute_MessageAcceptsComposerClearedSendVerificationCue(t *testing.T) {
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
		screenshotGroundingBytes: []byte("verify-send-grounding"),
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
					"composer_cleared": true,
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
	if out["failure_code"] != nil {
		t.Fatalf("failure_code = %v, want nil", out["failure_code"])
	}
	verification, ok := out["verification"].(map[string]interface{})
	if !ok {
		t.Fatalf("verification = %#v, want object", out["verification"])
	}
	if verification["composer_cleared"] != true {
		t.Fatalf("verification.composer_cleared = %#v, want true", verification["composer_cleared"])
	}
	if verification["status"] != "sent" {
		t.Fatalf("verification.status = %v, want sent default after positive cue", verification["status"])
	}
}

func TestA11yToolExecute_MessageFailsClosedWhenSendVerificationHasNoPositiveCue(t *testing.T) {
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
		screenshotGroundingBytes: []byte("verify-send-grounding"),
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
				Source:       "vision_model",
				Verification: map[string]interface{}{},
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
	if out["failure_code"] != "send_not_verified" {
		t.Fatalf("failure_code = %v, want send_not_verified", out["failure_code"])
	}
	if out["phase"] != "submit" {
		t.Fatalf("phase = %v, want submit", out["phase"])
	}
}

func TestA11yToolExecute_MessageUsesStructuredPostSubmitConfirmationBeforeGrounding(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"Message\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [document] \"Type a message\"\n@2 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-editor",
				2: "token-send",
			},
		},
		structuredSnapshot: a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
			WindowID: "win-feishu",
			Title:    "Feishu",
			Mode:     "ax",
		}, &a11yruntime.Node{
			Role: "window",
			Name: "Feishu",
			Children: []*a11yruntime.Node{
				{
					Token:       "token-editor",
					Role:        "document",
					Name:        "Message",
					Description: "focused editable",
					Interactive: true,
				},
				{
					Token:       "token-send",
					Role:        "button",
					Name:        "Send",
					Interactive: true,
				},
			},
		}),
		structuredSnapshotAfterSubmit: a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
			WindowID: "win-feishu",
			Title:    "Feishu",
			Mode:     "ax",
		}, &a11yruntime.Node{
			Role: "window",
			Name: "Feishu",
			Children: []*a11yruntime.Node{
				{
					Token:       "token-editor",
					Role:        "document",
					Name:        "Type a message",
					Description: "focused editable",
					Interactive: true,
				},
				{
					Token:       "token-send",
					Role:        "button",
					Name:        "Send",
					Interactive: true,
				},
			},
		}),
		screenshotGroundingBytes: []byte("verify-send-grounding"),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "message",
		"app_name": "Feishu",
		"value":    "hello",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["failure_code"] != nil {
		t.Fatalf("failure_code = %v, want nil", out["failure_code"])
	}
	verification, ok := out["verification"].(map[string]interface{})
	if !ok {
		t.Fatalf("verification = %#v, want object", out["verification"])
	}
	if verification["status"] != "sent" {
		t.Fatalf("verification.status = %v, want sent", verification["status"])
	}
	if verification["composer_cleared"] != true {
		t.Fatalf("verification.composer_cleared = %#v, want true", verification["composer_cleared"])
	}
	if backend.lastGroundingScreenshotWindow != "" {
		t.Fatalf("lastGroundingScreenshotWindow = %q, want empty when AX post-submit confirmation is enough", backend.lastGroundingScreenshotWindow)
	}
	if len(backend.runtimeUpdateHistory) < 2 {
		t.Fatalf("runtimeUpdateHistory = %#v, want type + submit updates before completion", backend.runtimeUpdateHistory)
	}
	if backend.runtimeUpdateHistory[0].ActType != "type" {
		t.Fatalf("runtimeUpdateHistory[0] = %#v, want first update for type", backend.runtimeUpdateHistory[0])
	}
	if backend.runtimeUpdateHistory[1].ActType != "submit" {
		t.Fatalf("runtimeUpdateHistory[1] = %#v, want second update for submit", backend.runtimeUpdateHistory[1])
	}
}

func TestA11yToolExecute_MessageUsesStructuredPostSubmitComposerAnchorBeforeGrounding(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"Message\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-send",
			},
		},
		structuredSnapshot: a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
			WindowID: "win-feishu",
			Title:    "Feishu",
			Mode:     "ax",
		}, &a11yruntime.Node{
			Role: "window",
			Name: "Feishu",
			Children: []*a11yruntime.Node{
				{
					Token:       "token-editor",
					Role:        "document",
					Name:        "Message",
					Description: "focused editable",
					Interactive: true,
				},
				{
					Token:       "token-send",
					Role:        "button",
					Name:        "Send",
					Interactive: true,
				},
			},
		}),
		structuredSnapshotAfterSubmit: a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
			WindowID: "win-feishu",
			Title:    "Feishu",
			Mode:     "ax",
		}, &a11yruntime.Node{
			Role: "window",
			Name: "Feishu",
			Children: []*a11yruntime.Node{
				{
					Token:       "token-editor",
					Role:        "document",
					Name:        "Type a message",
					Description: "focused editable",
					Interactive: true,
				},
				{
					Token:       "token-send",
					Role:        "button",
					Name:        "Send",
					Interactive: true,
				},
			},
		}),
		screenshotGroundingBytes: []byte("verify-send-grounding"),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "message",
		"app_name": "Feishu",
		"value":    "hello",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["failure_code"] != nil {
		t.Fatalf("failure_code = %v, want nil", out["failure_code"])
	}
	verification, ok := out["verification"].(map[string]interface{})
	if !ok {
		t.Fatalf("verification = %#v, want object", out["verification"])
	}
	if verification["status"] != "sent" {
		t.Fatalf("verification.status = %v, want sent", verification["status"])
	}
	if verification["composer_cleared"] != true {
		t.Fatalf("verification.composer_cleared = %#v, want true", verification["composer_cleared"])
	}
	if verification["confirmation"] != "structured_post_submit_anchor_confirmation" {
		t.Fatalf("verification.confirmation = %v, want structured_post_submit_anchor_confirmation", verification["confirmation"])
	}
	if verification["element_stable_id"] == nil || strings.TrimSpace(verification["element_stable_id"].(string)) == "" {
		t.Fatalf("verification.element_stable_id = %#v, want remembered composer anchor", verification["element_stable_id"])
	}
	if backend.interactiveCallsAfterSubmit != 0 {
		t.Fatalf("interactiveCallsAfterSubmit = %d, want 0 when current structured snapshot is enough for post-submit anchor confirmation", backend.interactiveCallsAfterSubmit)
	}
	if backend.lastGroundingScreenshotWindow != "" {
		t.Fatalf("lastGroundingScreenshotWindow = %q, want empty when AX post-submit anchor confirmation is enough", backend.lastGroundingScreenshotWindow)
	}
}

func TestVerifyA11yChatOutcome_UsesRememberedWindowForStructuredPostSubmitConfirmation(t *testing.T) {
	backend := &a11yCompatBackend{
		structuredSnapshot: a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
			WindowID: "win-feishu-current",
			Title:    "Feishu",
			Mode:     "ax",
		}, &a11yruntime.Node{
			Role: "window",
			Name: "Feishu",
			Children: []*a11yruntime.Node{
				{
					Token:       "token-editor",
					Role:        "document",
					Name:        "Type a message",
					Description: "focused editable",
					Interactive: true,
				},
				{
					Token:       "token-send",
					Role:        "button",
					Name:        "Send",
					Interactive: true,
				},
			},
		}),
		screenshotGroundingBytes: []byte("verify-send-grounding"),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.lastWindow = "win-feishu-current"
	grounder := &a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskVerifySend): {
				Source: "vision_model",
				Verification: map[string]interface{}{
					"status": "sent",
				},
			},
		},
	}
	tool.SetChatGrounder(grounder)
	state := newA11yChatExecutionState("darwin", "feishu_lark", "message", "Orca")

	err := tool.verifyA11yChatOutcome(
		withA11yChatExecutionState(context.Background(), state),
		backend,
		"win-feishu-stale",
		true,
		"hello",
	)
	if err != nil {
		t.Fatalf("verifyA11yChatOutcome() error = %v", err)
	}
	if state.verification["confirmation"] != "structured_post_submit_confirmation" {
		t.Fatalf("verification.confirmation = %v, want structured_post_submit_confirmation", state.verification["confirmation"])
	}
	if state.verification["composer_cleared"] != true {
		t.Fatalf("verification.composer_cleared = %#v, want true", state.verification["composer_cleared"])
	}
	if backend.interactiveCalls != 0 {
		t.Fatalf("interactiveCalls = %d, want 0 when remembered structured post-submit confirmation is enough", backend.interactiveCalls)
	}
	if backend.lastGroundingScreenshotWindow != "" {
		t.Fatalf("lastGroundingScreenshotWindow = %q, want empty when remembered structured post-submit confirmation is enough", backend.lastGroundingScreenshotWindow)
	}
	if len(grounder.calls) != 0 {
		t.Fatalf("grounder.calls = %#v, want none when remembered structured post-submit confirmation is enough", grounder.calls)
	}
}

func TestVerifyA11yChatOutcome_UsesRememberedWindowForInteractivePostSubmitCheckWhenStructuredStateIsPending(t *testing.T) {
	backend := &a11yCompatBackend{
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu-current",
			Title:    "Feishu",
			Tree:     "@1 [document] \"Type a message\"\n@2 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-editor",
				2: "token-send",
			},
		},
		structuredSnapshot: a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
			WindowID: "win-feishu-current",
			Title:    "Feishu",
			Mode:     "ax",
		}, &a11yruntime.Node{
			Role: "window",
			Name: "Feishu",
			Children: []*a11yruntime.Node{
				{
					Token:       "token-editor",
					Role:        "document",
					Name:        "hello",
					Description: "focused editable",
					Interactive: true,
				},
				{
					Token:       "token-send",
					Role:        "button",
					Name:        "Send",
					Interactive: true,
				},
			},
		}),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.lastWindow = "win-feishu-current"
	state := newA11yChatExecutionState("darwin", "feishu_lark", "message", "Orca")

	err := tool.verifyA11yChatOutcome(
		withA11yChatExecutionState(context.Background(), state),
		backend,
		"win-feishu-stale",
		true,
		"hello",
	)
	if err != nil {
		t.Fatalf("verifyA11yChatOutcome() error = %v", err)
	}
	if state.verification["confirmation"] != "structured_post_submit_confirmation" {
		t.Fatalf("verification.confirmation = %v, want structured_post_submit_confirmation", state.verification["confirmation"])
	}
	if backend.lastSnapshotWindowID != "win-feishu-current" {
		t.Fatalf("lastSnapshotWindowID = %q, want remembered current window for interactive post-submit check", backend.lastSnapshotWindowID)
	}
	if backend.interactiveCalls != 1 {
		t.Fatalf("interactiveCalls = %d, want 1 interactive post-submit check on remembered current window", backend.interactiveCalls)
	}
}

func TestVerifyA11yChatOutcome_UsesStructuredUniqueComposerAfterSubmitWithoutFocusedAnchor(t *testing.T) {
	backend := &a11yCompatBackend{
		structuredSnapshot: a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
			WindowID: "win-feishu",
			Title:    "Feishu",
			Mode:     "ax",
		}, &a11yruntime.Node{
			Role: "window",
			Name: "Feishu",
			Children: []*a11yruntime.Node{
				{
					Token:       "token-editor",
					Role:        "document",
					Name:        "Type a message",
					Interactive: true,
				},
				{
					Token:       "token-send",
					Role:        "button",
					Name:        "Send",
					Interactive: true,
				},
			},
		}),
		screenshotGroundingBytes: []byte("verify-send-grounding"),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	grounder := &a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskVerifySend): {
				Source: "vision_model",
				Verification: map[string]interface{}{
					"status": "sent",
				},
			},
		},
	}
	tool.SetChatGrounder(grounder)
	state := newA11yChatExecutionState("darwin", "feishu_lark", "message", "Orca")

	err := tool.verifyA11yChatOutcome(
		withA11yChatExecutionState(context.Background(), state),
		backend,
		"win-feishu",
		true,
		"hello",
	)
	if err != nil {
		t.Fatalf("verifyA11yChatOutcome() error = %v", err)
	}
	if state.verification["confirmation"] != "structured_post_submit_confirmation" {
		t.Fatalf("verification.confirmation = %v, want structured_post_submit_confirmation", state.verification["confirmation"])
	}
	if state.verification["composer_cleared"] != true {
		t.Fatalf("verification.composer_cleared = %#v, want true", state.verification["composer_cleared"])
	}
	if backend.interactiveCalls != 0 {
		t.Fatalf("interactiveCalls = %d, want 0 when unique structured composer confirmation is enough", backend.interactiveCalls)
	}
	if backend.lastGroundingScreenshotWindow != "" {
		t.Fatalf("lastGroundingScreenshotWindow = %q, want empty when unique structured composer confirmation is enough", backend.lastGroundingScreenshotWindow)
	}
	if len(grounder.calls) != 0 {
		t.Fatalf("grounder.calls = %#v, want none when unique structured composer confirmation is enough", grounder.calls)
	}
}

func TestVerifyA11yChatOutcome_WaitsForStructuredPostSubmitCueBeforeGrounding(t *testing.T) {
	prevTimeout := a11yChatVerifyOutcomeTimeout
	prevPoll := a11yChatVerifyOutcomePollInterval
	a11yChatVerifyOutcomeTimeout = 25 * time.Millisecond
	a11yChatVerifyOutcomePollInterval = 0
	defer func() {
		a11yChatVerifyOutcomeTimeout = prevTimeout
		a11yChatVerifyOutcomePollInterval = prevPoll
	}()

	backend := &a11yCompatBackend{
		structuredSnapshots: []*a11yruntime.Snapshot{
			a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
				WindowID: "win-feishu",
				Title:    "Feishu",
				Mode:     "ax",
			}, &a11yruntime.Node{
				Role: "window",
				Name: "Feishu",
				Children: []*a11yruntime.Node{
					{
						Token:       "token-editor",
						Role:        "document",
						Name:        "hello",
						Interactive: true,
					},
					{
						Token:       "token-send",
						Role:        "button",
						Name:        "Send",
						Interactive: true,
					},
				},
			}),
			a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
				WindowID: "win-feishu",
				Title:    "Feishu",
				Mode:     "ax",
			}, &a11yruntime.Node{
				Role: "window",
				Name: "Feishu",
				Children: []*a11yruntime.Node{
					{
						Token:       "token-editor",
						Role:        "document",
						Name:        "Type a message",
						Interactive: true,
					},
					{
						Token:       "token-send",
						Role:        "button",
						Name:        "Send",
						Interactive: true,
					},
				},
			}),
		},
		screenshotGroundingBytes: []byte("verify-send-grounding"),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	grounder := &a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskVerifySend): {
				Source: "vision_model",
				Verification: map[string]interface{}{
					"status": "sent",
				},
			},
		},
	}
	tool.SetChatGrounder(grounder)
	state := newA11yChatExecutionState("darwin", "feishu_lark", "message", "Orca")

	err := tool.verifyA11yChatOutcome(
		withA11yChatExecutionState(context.Background(), state),
		backend,
		"win-feishu",
		true,
		"hello",
	)
	if err != nil {
		t.Fatalf("verifyA11yChatOutcome() error = %v", err)
	}
	if state.verification["confirmation"] != "structured_post_submit_confirmation" {
		t.Fatalf("verification.confirmation = %v, want structured_post_submit_confirmation", state.verification["confirmation"])
	}
	if backend.interactiveCalls != 0 {
		t.Fatalf("interactiveCalls = %d, want 0 when late structured cue arrives before grounding", backend.interactiveCalls)
	}
	if backend.lastGroundingScreenshotWindow != "" {
		t.Fatalf("lastGroundingScreenshotWindow = %q, want empty when late structured cue arrives before grounding", backend.lastGroundingScreenshotWindow)
	}
	if len(grounder.calls) != 0 {
		t.Fatalf("grounder.calls = %#v, want none when late structured cue arrives before grounding", grounder.calls)
	}
	if backend.structuredSnapshotCalls < 2 {
		t.Fatalf("structuredSnapshotCalls = %d, want >= 2 while waiting for late structured cue", backend.structuredSnapshotCalls)
	}
}

func TestA11yToolExecute_MessageReusesSubmitPhaseInteractiveConfirmationBeforeVerifyOutcome(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"Message\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
		interactiveResultAfterSubmit: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [document] \"Type a message\"\n@2 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-editor",
				2: "token-send",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "message",
		"app_name": "Feishu",
		"value":    "hello",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["failure_code"] != nil {
		t.Fatalf("failure_code = %v, want nil", out["failure_code"])
	}
	verification, ok := out["verification"].(map[string]interface{})
	if !ok {
		t.Fatalf("verification = %#v, want object", out["verification"])
	}
	if verification["status"] != "sent" {
		t.Fatalf("verification.status = %v, want sent", verification["status"])
	}
	if verification["composer_cleared"] != true {
		t.Fatalf("verification.composer_cleared = %#v, want true", verification["composer_cleared"])
	}
	if verification["confirmation"] != "interactive_post_submit_confirmation" {
		t.Fatalf("verification.confirmation = %v, want interactive_post_submit_confirmation", verification["confirmation"])
	}
	if backend.interactiveCallsAfterSubmit != 1 {
		t.Fatalf("interactiveCallsAfterSubmit = %d, want 1 when verify_outcome reuses submit-phase interactive confirmation", backend.interactiveCallsAfterSubmit)
	}
	if backend.lastGroundingScreenshotWindow != "" {
		t.Fatalf("lastGroundingScreenshotWindow = %q, want empty when submit-phase interactive confirmation is reused", backend.lastGroundingScreenshotWindow)
	}
}

func TestA11yToolExecute_MessageWithConversationStillRunsLateStructuredVerifyWithoutGrounder(t *testing.T) {
	prevPoll := a11yChatVerifyOutcomePollInterval
	a11yChatVerifyOutcomePollInterval = 0
	defer func() {
		a11yChatVerifyOutcomePollInterval = prevPoll
	}()

	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [list_item] \"Orca\"\n@2 [document] \"Message\"\n@3 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-orca",
					2: "token-editor",
					3: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"Message\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
		interactiveResultAfterSubmit: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [group] \"Sending\"",
			RefMap: map[int]string{
				1: "token-sending",
			},
		},
		structuredSnapshots: []*a11yruntime.Snapshot{
			a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
				WindowID: "win-feishu",
				Title:    "Feishu",
				Mode:     "ax",
			}, &a11yruntime.Node{
				Role: "window",
				Name: "Feishu",
				Children: []*a11yruntime.Node{
					{
						Token:       "token-status",
						Role:        "group",
						Name:        "Sending",
						Interactive: false,
					},
				},
			}),
			a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
				WindowID: "win-feishu",
				Title:    "Feishu",
				Mode:     "ax",
			}, &a11yruntime.Node{
				Role: "window",
				Name: "Feishu",
				Children: []*a11yruntime.Node{
					{
						Token:       "token-message-hello",
						Role:        "static_text",
						Name:        "hello",
						Interactive: false,
					},
				},
			}),
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
	if out["failure_code"] != nil {
		t.Fatalf("failure_code = %v, want nil", out["failure_code"])
	}
	verification, ok := out["verification"].(map[string]interface{})
	if !ok {
		t.Fatalf("verification = %#v, want object", out["verification"])
	}
	if verification["status"] != "sent" {
		t.Fatalf("verification.status = %v, want sent", verification["status"])
	}
	if verification["message_list_matched"] != true {
		t.Fatalf("verification.message_list_matched = %#v, want true", verification["message_list_matched"])
	}
	if verification["confirmation"] != "structured_sent_message_visible" {
		t.Fatalf("verification.confirmation = %v, want structured_sent_message_visible", verification["confirmation"])
	}
	if backend.lastGroundingScreenshotWindow != "" {
		t.Fatalf("lastGroundingScreenshotWindow = %q, want empty when structured verify completes without grounder", backend.lastGroundingScreenshotWindow)
	}
	if backend.structuredSnapshotCalls < 2 {
		t.Fatalf("structuredSnapshotCalls = %d, want >= 2 so verify_outcome waits for the late structured cue", backend.structuredSnapshotCalls)
	}
}

func TestA11yToolExecute_MessageUsesFocusedClipboardFallbackAfterComposerConfirmation(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [list_item] \"Orca\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-orca",
					2: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-send",
				},
			},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-send",
			},
		},
		structuredSnapshot: a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
			WindowID: "win-feishu",
			Title:    "Feishu",
			Mode:     "ax",
		}, &a11yruntime.Node{
			Role: "window",
			Name: "Feishu",
			Children: []*a11yruntime.Node{
				{
					Token:       "token-orca",
					Role:        "list_item",
					Name:        "Orca",
					Description: "selected",
					Interactive: true,
				},
				{
					Token:       "token-editor",
					Role:        "document",
					Name:        "Message",
					Description: "focused editable",
					Interactive: true,
				},
				{
					Token:       "token-send",
					Role:        "button",
					Name:        "Send",
					Interactive: true,
				},
			},
		}),
		structuredSnapshotAfterSubmit: a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
			WindowID: "win-feishu",
			Title:    "Feishu",
			Mode:     "ax",
		}, &a11yruntime.Node{
			Role: "window",
			Name: "Feishu",
			Children: []*a11yruntime.Node{
				{
					Token:       "token-orca",
					Role:        "list_item",
					Name:        "Orca",
					Description: "selected",
					Interactive: true,
				},
				{
					Token:       "token-editor",
					Role:        "document",
					Name:        "Type a message",
					Description: "focused editable",
					Interactive: true,
				},
				{
					Token:       "token-send",
					Role:        "button",
					Name:        "Send",
					Interactive: true,
				},
			},
		}),
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
	if got := len(backend.focusedTypeHistory); got != 1 {
		t.Fatalf("focusedTypeHistory len = %d, want 1 focused clipboard fallback after composer confirmation", got)
	}
	if backend.lastFocusedTypeValue != "hello" {
		t.Fatalf("lastFocusedTypeValue = %q, want hello", backend.lastFocusedTypeValue)
	}
	for _, actType := range backend.actTypeHistory {
		if actType == "type" {
			t.Fatalf("actTypeHistory = %#v, want no semantic type act when focused clipboard fallback is available", backend.actTypeHistory)
		}
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != nil {
		t.Fatalf("error_code = %v, want nil after focused clipboard fallback", out["error_code"])
	}
	if out["input_method"] != "clipboard" {
		t.Fatalf("input_method = %v, want clipboard", out["input_method"])
	}
	if out["verification_method"] != "focused_text" {
		t.Fatalf("verification_method = %v, want focused_text", out["verification_method"])
	}
}

func TestA11yToolExecute_MessageStopsAtTypeOrSendWhenSubmitEvidenceAlreadyConfirmsSuccess(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"Message\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
		interactiveResultAfterSubmit: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [document] \"Type a message\"\n@2 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-editor",
				2: "token-send",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	var events []ToolEvent
	ctx := WithEventEmitter(context.Background(), func(_ context.Context, event ToolEvent) error {
		events = append(events, event)
		return nil
	})

	raw, err := tool.Execute(ctx, map[string]interface{}{
		"action":   "message",
		"app_name": "Feishu",
		"value":    "hello",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["stage"] != "type_or_send" {
		t.Fatalf("stage = %v, want type_or_send when submit evidence already confirms success", out["stage"])
	}
	if out["strategy"] != "submit_phase_confirmation" {
		t.Fatalf("strategy = %v, want submit_phase_confirmation when submit evidence already confirms success", out["strategy"])
	}
	verification, ok := out["verification"].(map[string]interface{})
	if !ok {
		t.Fatalf("verification = %#v, want object", out["verification"])
	}
	if verification["confirmation"] != "interactive_post_submit_confirmation" {
		t.Fatalf("verification.confirmation = %v, want interactive_post_submit_confirmation", verification["confirmation"])
	}
	for _, event := range events {
		stage, _ := event.Payload["stage"].(string)
		if stage == "computer_use_chat.verify_outcome" {
			t.Fatalf("events = %#v, do not want standalone verify_outcome event when submit evidence already confirms success", events)
		}
	}
}

func TestA11yToolExecute_MessageUsesStructuredSentMessageCueBeforeGrounding(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"Message\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-send",
			},
		},
		structuredSnapshot: a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
			WindowID: "win-feishu",
			Title:    "Feishu",
			Mode:     "ax",
		}, &a11yruntime.Node{
			Role: "window",
			Name: "Feishu",
			Children: []*a11yruntime.Node{
				{
					Token:       "token-editor",
					Role:        "document",
					Name:        "Message",
					Description: "focused editable",
					Interactive: true,
				},
				{
					Token:       "token-send",
					Role:        "button",
					Name:        "Send",
					Interactive: true,
				},
			},
		}),
		structuredSnapshotAfterSubmit: a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
			WindowID: "win-feishu",
			Title:    "Feishu",
			Mode:     "ax",
		}, &a11yruntime.Node{
			Role: "window",
			Name: "Feishu",
			Children: []*a11yruntime.Node{
				{
					Role: "group",
					Name: "Conversation Body",
					Children: []*a11yruntime.Node{
						{
							Role: "static_text",
							Name: "hello",
						},
					},
				},
				{
					Token:       "token-editor",
					Role:        "document",
					Name:        "Type a message",
					Description: "focused editable",
					Interactive: true,
				},
				{
					Token:       "token-send",
					Role:        "button",
					Name:        "Send",
					Interactive: true,
				},
			},
		}),
		screenshotGroundingBytes: []byte("verify-send-grounding"),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "message",
		"app_name": "Feishu",
		"value":    "hello",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["failure_code"] != nil {
		t.Fatalf("failure_code = %v, want nil", out["failure_code"])
	}
	verification, ok := out["verification"].(map[string]interface{})
	if !ok {
		t.Fatalf("verification = %#v, want object", out["verification"])
	}
	if verification["status"] != "sent" {
		t.Fatalf("verification.status = %v, want sent", verification["status"])
	}
	if verification["message_list_matched"] != true {
		t.Fatalf("verification.message_list_matched = %#v, want true", verification["message_list_matched"])
	}
	if backend.lastGroundingScreenshotWindow != "" {
		t.Fatalf("lastGroundingScreenshotWindow = %q, want empty when structured sent-message cue is enough", backend.lastGroundingScreenshotWindow)
	}
}

func TestA11yToolExecute_MessageFailsClosedWhenSendVerificationGroundingIsUnavailable(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"Message\"\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [group] \"Conversation body\"",
				RefMap: map[int]string{
					1: "token-body",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "message",
		"app_name": "Feishu",
		"value":    "hello",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["failure_code"] != "send_not_verified" {
		t.Fatalf("failure_code = %v, want send_not_verified", out["failure_code"])
	}
	if out["phase"] != "submit" {
		t.Fatalf("phase = %v, want submit", out["phase"])
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

func TestA11yToolExecute_MessageWaitsForComposerVisualConfirmationWhenGroundingMismatchIsTransient(t *testing.T) {
	prevTimeout := a11yChatComposerConfirmationTimeout
	prevPoll := a11yChatComposerConfirmationPollInterval
	a11yChatComposerConfirmationTimeout = 100 * time.Millisecond
	a11yChatComposerConfirmationPollInterval = time.Millisecond
	defer func() {
		a11yChatComposerConfirmationTimeout = prevTimeout
		a11yChatComposerConfirmationPollInterval = prevPoll
	}()

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
		screenshotGroundingBytes: []byte("composer-grounding"),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	grounder := &a11yChatGrounderStub{
		resultSequences: map[string][]a11yChatGroundingResult{
			string(a11yChatGroundingTaskLocateComposer): {
				{
					Source: "vision_model",
					Candidates: []a11yChatGroundingCandidate{
						{Role: "search_field", Label: "Search", Confidence: 0.99, RationaleTags: []string{"search_field"}},
					},
				},
				{
					Source: "vision_model",
					Candidates: []a11yChatGroundingCandidate{
						{Role: "composer", Label: "Message", Confidence: 0.98},
					},
				},
			},
			string(a11yChatGroundingTaskVerifySend): {
				{
					Source: "vision_model",
					Verification: map[string]interface{}{
						"status": "sent",
					},
				},
			},
		},
	}
	tool.SetChatGrounder(grounder)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "message",
		"app_name":     "Feishu",
		"conversation": "Orca",
		"value":        "hello",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 3 || backend.actTypeHistory[0] != "click" || backend.actTypeHistory[1] != "type" || backend.actTypeHistory[2] != "submit" {
		t.Fatalf("actTypeHistory = %#v, want [click type submit]", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["failure_code"] != nil {
		t.Fatalf("failure_code = %v, want nil", out["failure_code"])
	}
	locateComposerCalls := 0
	for _, call := range grounder.calls {
		if call.TaskHint == a11yChatGroundingTaskLocateComposer {
			locateComposerCalls++
		}
	}
	if locateComposerCalls < 2 {
		t.Fatalf("grounder.calls = %#v, want repeated locate_composer attempts", grounder.calls)
	}
}

func TestA11yToolExecute_MessageDoesNotFailWhenComposerAndSearchFieldAreBothGrounded(t *testing.T) {
	prevTimeout := a11yChatComposerConfirmationTimeout
	prevPoll := a11yChatComposerConfirmationPollInterval
	a11yChatComposerConfirmationTimeout = 0
	a11yChatComposerConfirmationPollInterval = time.Millisecond
	defer func() {
		a11yChatComposerConfirmationTimeout = prevTimeout
		a11yChatComposerConfirmationPollInterval = prevPoll
	}()

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
		screenshotGroundingBytes: []byte("composer-grounding"),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	grounder := &a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskLocateComposer): {
				Source: "vision_model",
				Candidates: []a11yChatGroundingCandidate{
					{Role: "search_field", Label: "Search", Confidence: 0.99, RationaleTags: []string{"search_field"}},
					{Role: "composer", Label: "Message", Confidence: 0.98, RationaleTags: []string{"current", "selected"}},
				},
			},
			string(a11yChatGroundingTaskVerifySend): {
				Source: "vision_model",
				Verification: map[string]interface{}{
					"status": "sent",
				},
			},
		},
	}
	tool.SetChatGrounder(grounder)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "message",
		"app_name":     "Feishu",
		"conversation": "Orca",
		"value":        "hello",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 3 || backend.actTypeHistory[0] != "click" || backend.actTypeHistory[1] != "type" || backend.actTypeHistory[2] != "submit" {
		t.Fatalf("actTypeHistory = %#v, want [click type submit]", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["failure_code"] != nil {
		t.Fatalf("failure_code = %v, want nil", out["failure_code"])
	}
	locateComposerCalls := 0
	for _, call := range grounder.calls {
		if call.TaskHint == a11yChatGroundingTaskLocateComposer {
			locateComposerCalls++
		}
	}
	if locateComposerCalls != 1 {
		t.Fatalf("grounder.calls = %#v, want single locate_composer grounding attempt", grounder.calls)
	}
}

func TestConfirmA11yChatComposerReady_SkipsGroundingWhenStructuredSnapshotShowsFocusedComposer(t *testing.T) {
	backend := &a11yCompatBackend{
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"Type a message\"\n@2 [button] \"Send\"",
				RefMap:   map[int]string{1: "token-editor", 2: "token-send"},
			},
		},
		screenshotGroundingBytes: []byte("composer-grounding"),
		structuredSnapshot: a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
			WindowID: "win-feishu",
			Title:    "Feishu",
			Mode:     "ax",
		}, &a11yruntime.Node{
			Role: "window",
			Name: "Feishu",
			Children: []*a11yruntime.Node{
				{
					Token:       "token-search",
					Role:        "search_field",
					Name:        "Search",
					Description: "search conversations",
					Interactive: true,
				},
				{
					Token:       "token-editor",
					Role:        "document",
					Name:        "Type a message",
					Description: "focused editable",
					Interactive: true,
				},
				{
					Token:       "token-send",
					Role:        "button",
					Name:        "Send",
					Interactive: true,
				},
			},
		}),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	grounder := &a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskLocateComposer): {
				Source: "vision_model",
				Candidates: []a11yChatGroundingCandidate{
					{Role: "composer", Label: "Message", Confidence: 0.98},
				},
			},
		},
	}
	tool.SetChatGrounder(grounder)
	ctx := withA11yChatExecutionState(context.Background(), newA11yChatExecutionState("darwin", "feishu_lark", "message", "Orca"))

	windowID, err := tool.confirmA11yChatComposerReady(ctx, backend, "win-feishu")
	if err != nil {
		t.Fatalf("confirmA11yChatComposerReady() error = %v", err)
	}
	if windowID != "win-feishu" {
		t.Fatalf("windowID = %q, want win-feishu", windowID)
	}
	if backend.lastGroundingScreenshotWindow != "" {
		t.Fatalf("lastGroundingScreenshotWindow = %q, want empty when structured composer confirmation is enough", backend.lastGroundingScreenshotWindow)
	}
	if backend.interactiveCalls != 0 {
		t.Fatalf("interactiveCalls = %d, want 0 when current structured snapshot already proves a focused composer", backend.interactiveCalls)
	}
	if len(grounder.calls) != 0 {
		t.Fatalf("grounder.calls = %#v, want none when structured composer confirmation is enough", grounder.calls)
	}
}

func TestConfirmA11yChatComposerReady_UsesRememberedWindowForStructuredFocusedComposer(t *testing.T) {
	backend := &a11yCompatBackend{
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu-current",
				Title:    "Feishu",
				Tree:     "@1 [document] \"Type a message\"\n@2 [button] \"Send\"",
				RefMap:   map[int]string{1: "token-editor", 2: "token-send"},
			},
		},
		screenshotGroundingBytes: []byte("composer-grounding"),
		structuredSnapshot: a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
			WindowID: "win-feishu-current",
			Title:    "Feishu",
			Mode:     "ax",
		}, &a11yruntime.Node{
			Role: "window",
			Name: "Feishu",
			Children: []*a11yruntime.Node{
				{
					Token:       "token-editor",
					Role:        "document",
					Name:        "Type a message",
					Description: "focused editable",
					Interactive: true,
				},
				{
					Token:       "token-send",
					Role:        "button",
					Name:        "Send",
					Interactive: true,
				},
			},
		}),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.lastWindow = "win-feishu-current"
	grounder := &a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskLocateComposer): {
				Source: "vision_model",
				Candidates: []a11yChatGroundingCandidate{
					{Role: "composer", Label: "Message", Confidence: 0.98},
				},
			},
		},
	}
	tool.SetChatGrounder(grounder)
	ctx := withA11yChatExecutionState(context.Background(), newA11yChatExecutionState("darwin", "feishu_lark", "message", "Orca"))

	windowID, err := tool.confirmA11yChatComposerReady(ctx, backend, "win-feishu-stale")
	if err != nil {
		t.Fatalf("confirmA11yChatComposerReady() error = %v", err)
	}
	if windowID != "win-feishu-current" {
		t.Fatalf("windowID = %q, want win-feishu-current from remembered structured window", windowID)
	}
	if backend.lastGroundingScreenshotWindow != "" {
		t.Fatalf("lastGroundingScreenshotWindow = %q, want empty when remembered structured composer confirmation is enough", backend.lastGroundingScreenshotWindow)
	}
	if backend.interactiveCalls != 0 {
		t.Fatalf("interactiveCalls = %d, want 0 when remembered structured snapshot already proves a focused composer", backend.interactiveCalls)
	}
	if len(grounder.calls) != 0 {
		t.Fatalf("grounder.calls = %#v, want none when remembered structured composer confirmation is enough", grounder.calls)
	}
}

func TestConfirmA11yChatComposerReady_DoesNotSkipGroundingWhenStructuredSnapshotShowsFocusedSearchField(t *testing.T) {
	backend := &a11yCompatBackend{
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document] \"Type a message\"\n@2 [button] \"Send\"",
				RefMap:   map[int]string{1: "token-editor", 2: "token-send"},
			},
		},
		screenshotGroundingBytes: []byte("composer-grounding"),
		structuredSnapshot: a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
			WindowID: "win-feishu",
			Title:    "Feishu",
			Mode:     "ax",
		}, &a11yruntime.Node{
			Role: "window",
			Name: "Feishu",
			Children: []*a11yruntime.Node{
				{
					Token:       "token-search",
					Role:        "search_field",
					Name:        "Search",
					Description: "focused active",
					Interactive: true,
				},
				{
					Token:       "token-editor",
					Role:        "document",
					Name:        "Type a message",
					Description: "editable",
					Interactive: true,
				},
				{
					Token:       "token-send",
					Role:        "button",
					Name:        "Send",
					Interactive: true,
				},
			},
		}),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	grounder := &a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskLocateComposer): {
				Source: "vision_model",
				Candidates: []a11yChatGroundingCandidate{
					{Role: "composer", Label: "Message", Confidence: 0.98},
				},
			},
		},
	}
	tool.SetChatGrounder(grounder)
	ctx := withA11yChatExecutionState(context.Background(), newA11yChatExecutionState("darwin", "feishu_lark", "message", "Orca"))

	windowID, err := tool.confirmA11yChatComposerReady(ctx, backend, "win-feishu")
	if err != nil {
		t.Fatalf("confirmA11yChatComposerReady() error = %v", err)
	}
	if windowID != "win-feishu" {
		t.Fatalf("windowID = %q, want win-feishu", windowID)
	}
	if backend.lastGroundingScreenshotWindow != "win-feishu" {
		t.Fatalf("lastGroundingScreenshotWindow = %q, want win-feishu when focused search field blocks the structured shortcut", backend.lastGroundingScreenshotWindow)
	}
	if len(grounder.calls) != 1 {
		t.Fatalf("grounder.calls = %#v, want one locate_composer grounding call", grounder.calls)
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

func TestA11yToolExecute_MessageIgnoresStaleComposerCacheFromDifferentWindow(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [document]\n@2 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-editor",
				2: "token-send",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.SetChatGrounder(&a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskVerifySend): {
				Source: "vision_model",
				Verification: map[string]interface{}{
					"status": "sent",
				},
			},
		},
	})
	tool.lastWindow = "win-slack"
	tool.lastRefs = parseA11ySnapshotEntries("@1 [search_field] \"Search\"")
	tool.lastRefMap = map[int]string{1: "token-stale-search"}

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "message",
		"app_name": "Feishu",
		"value":    "hello",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 2 || backend.actTypeHistory[0] != "type" || backend.actTypeHistory[1] != "submit" {
		t.Fatalf("actTypeHistory = %#v, want [type submit]", backend.actTypeHistory)
	}
	if backend.lastActWindowID != "win-feishu" {
		t.Fatalf("lastActWindowID = %q, want win-feishu", backend.lastActWindowID)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["failure_code"] != nil {
		t.Fatalf("failure_code = %v, want nil", out["failure_code"])
	}
}

func TestA11yToolExecute_MessageDoesNotTrustComposerCacheFromDifferentWindow(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [search_field] \"Search\"",
			RefMap: map[int]string{
				1: "token-search",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.lastWindow = "win-slack"
	tool.lastRefs = parseA11ySnapshotEntries("@1 [document]\n@2 [button] \"Send\"")
	tool.lastRefMap = map[int]string{
		1: "token-stale-editor",
		2: "token-stale-send",
	}

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "message",
		"app_name": "Feishu",
		"value":    "hello",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 0 {
		t.Fatalf("actTypeHistory = %#v, want fail-closed before typing", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["phase"] != "composer" {
		t.Fatalf("phase = %v, want composer", out["phase"])
	}
	if out["failure_code"] != "composer_not_found" {
		t.Fatalf("failure_code = %v, want composer_not_found", out["failure_code"])
	}
}

func TestA11yToolExecute_MessageWaitsForComposerWhenCurrentChatInputAppearsLate(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [search_field] \"Search\"",
				RefMap: map[int]string{
					1: "token-search",
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
			string(a11yChatGroundingTaskVerifySend): {
				Source: "vision_model",
				Verification: map[string]interface{}{
					"status": "sent",
				},
			},
		},
	})

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "message",
		"app_name": "Feishu",
		"value":    "hello",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if len(backend.actTypeHistory) != 2 || backend.actTypeHistory[0] != "type" || backend.actTypeHistory[1] != "submit" {
		t.Fatalf("actTypeHistory = %#v, want [type submit]", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["failure_code"] != nil {
		t.Fatalf("failure_code = %v, want nil", out["failure_code"])
	}
}

func TestA11yToolExecute_MessageRecoversComposerViaVisualPointClickWhenAXMisses(t *testing.T) {
	prevTimeout := a11yChatComposerConfirmationTimeout
	prevPoll := a11yChatComposerConfirmationPollInterval
	prevSettle := a11yMessageConversationSettleDelay
	a11yChatComposerConfirmationTimeout = 0
	a11yChatComposerConfirmationPollInterval = time.Millisecond
	a11yMessageConversationSettleDelay = 0
	defer func() {
		a11yChatComposerConfirmationTimeout = prevTimeout
		a11yChatComposerConfirmationPollInterval = prevPoll
		a11yMessageConversationSettleDelay = prevSettle
	}()

	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [group] \"Conversation body\"",
				RefMap: map[int]string{
					1: "token-body",
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
		screenshotGroundingBytes: []byte("composer-grounding"),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	grounder := &a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskLocateComposer): {
				Source: "vision_model",
				Candidates: []a11yChatGroundingCandidate{
					{
						Role:       "composer",
						Label:      "Message",
						Confidence: 0.98,
						Bounds: a11yruntime.NormalizedRect{
							X:      0.20,
							Y:      0.70,
							Width:  0.50,
							Height: 0.10,
						},
					},
				},
			},
			string(a11yChatGroundingTaskVerifySend): {
				Source: "vision_model",
				Verification: map[string]interface{}{
					"status": "sent",
				},
			},
		},
	}
	tool.SetChatGrounder(grounder)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "message",
		"app_name": "Feishu",
		"value":    "hello",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if len(backend.pointClickHistory) != 1 {
		t.Fatalf("pointClickHistory = %#v, want one visual composer recovery click", backend.pointClickHistory)
	}
	if backend.pointClickHistory[0].X != 0.45 || backend.pointClickHistory[0].Y != 0.75 {
		t.Fatalf("pointClick = %#v, want composer center", backend.pointClickHistory[0])
	}
	if len(backend.actTypeHistory) != 2 || backend.actTypeHistory[0] != "type" || backend.actTypeHistory[1] != "submit" {
		t.Fatalf("actTypeHistory = %#v, want [type submit] after composer recovery", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["failure_code"] != nil {
		t.Fatalf("failure_code = %v, want nil", out["failure_code"])
	}
	if out["grounding_source"] != "vision_model" {
		t.Fatalf("grounding_source = %v, want vision_model", out["grounding_source"])
	}
}

func TestA11yToolExecute_MessageRecoversComposerViaStructuredPointClickWhenAXMisses(t *testing.T) {
	prevTimeout := a11yChatComposerConfirmationTimeout
	prevPoll := a11yChatComposerConfirmationPollInterval
	prevSettle := a11yMessageConversationSettleDelay
	a11yChatComposerConfirmationTimeout = 0
	a11yChatComposerConfirmationPollInterval = time.Millisecond
	a11yMessageConversationSettleDelay = 0
	defer func() {
		a11yChatComposerConfirmationTimeout = prevTimeout
		a11yChatComposerConfirmationPollInterval = prevPoll
		a11yMessageConversationSettleDelay = prevSettle
	}()

	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [group] \"Conversation body\"",
				RefMap: map[int]string{
					1: "token-body",
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
		structuredSnapshot: &a11yruntime.Snapshot{
			WindowID: "win-feishu",
			Nodes: []a11yruntime.FlatNode{
				{
					NodeID:      1,
					Role:        "group",
					Name:        "Conversation body",
					Bounds:      a11yruntime.NormalizedRect{X: 0, Y: 0, Width: 1, Height: 1},
					Visible:     true,
					Enabled:     true,
					Interactive: false,
				},
				{
					NodeID:      2,
					ParentID:    1,
					Role:        "document",
					Name:        "Type a message",
					Bounds:      a11yruntime.NormalizedRect{X: 0.20, Y: 0.70, Width: 0.50, Height: 0.10},
					Visible:     true,
					Enabled:     true,
					Interactive: false,
				},
			},
		},
		structuredSnapshotAfterSubmit: a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
			WindowID: "win-feishu",
			Title:    "Feishu",
			Mode:     "ax",
		}, &a11yruntime.Node{
			Role: "window",
			Name: "Feishu",
			Children: []*a11yruntime.Node{
				{
					Role: "group",
					Name: "Conversation Body",
					Children: []*a11yruntime.Node{
						{
							Role: "static_text",
							Name: "hello",
						},
					},
				},
				{
					Token:       "token-editor",
					Role:        "document",
					Name:        "Type a message",
					Description: "focused editable",
					Interactive: true,
				},
				{
					Token:       "token-send",
					Role:        "button",
					Name:        "Send",
					Interactive: true,
				},
			},
		}),
		interactiveResultAfterSubmit: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-send",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "message",
		"app_name": "Feishu",
		"value":    "hello",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if len(backend.pointClickHistory) != 1 {
		t.Fatalf("pointClickHistory = %#v, want one structured composer recovery click", backend.pointClickHistory)
	}
	if backend.pointClickHistory[0].X != 0.45 || backend.pointClickHistory[0].Y != 0.75 {
		t.Fatalf("pointClick = %#v, want composer center", backend.pointClickHistory[0])
	}
	if backend.lastGroundingScreenshotWindow != "" {
		t.Fatalf("lastGroundingScreenshotWindow = %q, want empty when structured composer recovery is enough", backend.lastGroundingScreenshotWindow)
	}
	if len(backend.actTypeHistory) != 2 || backend.actTypeHistory[0] != "type" || backend.actTypeHistory[1] != "submit" {
		t.Fatalf("actTypeHistory = %#v, want [type submit] after structured composer recovery", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["failure_code"] != nil {
		t.Fatalf("failure_code = %v, want nil", out["failure_code"])
	}
	if out["grounding_source"] != nil {
		t.Fatalf("grounding_source = %v, want nil when structured composer recovery is enough", out["grounding_source"])
	}
}

func TestA11yToolExecute_MessageFallsBackToVisualComposerRecoveryWhenStructuredBoundsAreAmbiguous(t *testing.T) {
	prevTimeout := a11yChatComposerConfirmationTimeout
	prevPoll := a11yChatComposerConfirmationPollInterval
	prevSettle := a11yMessageConversationSettleDelay
	a11yChatComposerConfirmationTimeout = 0
	a11yChatComposerConfirmationPollInterval = time.Millisecond
	a11yMessageConversationSettleDelay = 0
	defer func() {
		a11yChatComposerConfirmationTimeout = prevTimeout
		a11yChatComposerConfirmationPollInterval = prevPoll
		a11yMessageConversationSettleDelay = prevSettle
	}()

	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [group] \"Conversation body\"",
				RefMap: map[int]string{
					1: "token-body",
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
		structuredSnapshot: &a11yruntime.Snapshot{
			WindowID: "win-feishu",
			Nodes: []a11yruntime.FlatNode{
				{
					NodeID:      1,
					Role:        "document",
					Name:        "Type a message",
					Bounds:      a11yruntime.NormalizedRect{X: 0.20, Y: 0.70, Width: 0.50, Height: 0.10},
					Visible:     true,
					Enabled:     true,
					Interactive: false,
				},
				{
					NodeID:      2,
					Role:        "text_field",
					Name:        "Reply",
					Bounds:      a11yruntime.NormalizedRect{X: 0.18, Y: 0.58, Width: 0.52, Height: 0.10},
					Visible:     true,
					Enabled:     true,
					Interactive: false,
				},
			},
		},
		structuredSnapshotAfterSubmit: a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
			WindowID: "win-feishu",
			Title:    "Feishu",
			Mode:     "ax",
		}, &a11yruntime.Node{
			Role: "window",
			Name: "Feishu",
			Children: []*a11yruntime.Node{
				{
					Role: "group",
					Name: "Conversation Body",
					Children: []*a11yruntime.Node{
						{
							Role: "static_text",
							Name: "hello",
						},
					},
				},
				{
					Token:       "token-editor",
					Role:        "document",
					Name:        "Type a message",
					Description: "focused editable",
					Interactive: true,
				},
				{
					Token:       "token-send",
					Role:        "button",
					Name:        "Send",
					Interactive: true,
				},
			},
		}),
		interactiveResultAfterSubmit: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-send",
			},
		},
		screenshotGroundingBytes: []byte("composer-grounding"),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	grounder := &a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskLocateComposer): {
				Source: "vision_model",
				Candidates: []a11yChatGroundingCandidate{
					{
						Role:       "composer",
						Label:      "Message",
						Confidence: 0.98,
						Bounds: a11yruntime.NormalizedRect{
							X:      0.20,
							Y:      0.70,
							Width:  0.50,
							Height: 0.10,
						},
					},
				},
			},
		},
	}
	tool.SetChatGrounder(grounder)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "message",
		"app_name": "Feishu",
		"value":    "hello",
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if backend.lastGroundingScreenshotWindow != "win-feishu" {
		t.Fatalf("lastGroundingScreenshotWindow = %q, want win-feishu when structured composer bounds are ambiguous", backend.lastGroundingScreenshotWindow)
	}
	if len(grounder.calls) == 0 {
		t.Fatal("grounder.calls = nil, want locate_composer grounding fallback when structured composer bounds are ambiguous")
	}
	for _, call := range grounder.calls {
		if call.TaskHint != a11yChatGroundingTaskLocateComposer {
			t.Fatalf("grounder.calls = %#v, want only locate_composer grounding calls in this path", grounder.calls)
		}
	}
	if len(backend.pointClickHistory) != 1 {
		t.Fatalf("pointClickHistory = %#v, want one composer recovery click", backend.pointClickHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["failure_code"] != nil {
		t.Fatalf("failure_code = %v, want nil", out["failure_code"])
	}
	if out["grounding_source"] != "vision_model" {
		t.Fatalf("grounding_source = %v, want vision_model after visual fallback", out["grounding_source"])
	}
}

func TestTryA11yChatComposerStructuredRecovery_PrefersRememberedStableIDAnchor(t *testing.T) {
	prevSettle := a11yMessageConversationSettleDelay
	a11yMessageConversationSettleDelay = 0
	defer func() {
		a11yMessageConversationSettleDelay = prevSettle
	}()

	snapshot := a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
		WindowID: "win-feishu",
		Title:    "Feishu",
		Mode:     "ax",
	}, &a11yruntime.Node{
		Role: "window",
		Name: "Feishu",
		Children: []*a11yruntime.Node{
			{
				Token:       "token-editor-primary",
				Role:        "document",
				Name:        "Type a message",
				Bounds:      a11yruntime.NormalizedRect{X: 0.18, Y: 0.58, Width: 0.52, Height: 0.10},
				Interactive: true,
			},
			{
				Token:       "token-editor-remembered",
				Role:        "document",
				Name:        "Type a message",
				Bounds:      a11yruntime.NormalizedRect{X: 0.12, Y: 0.74, Width: 0.64, Height: 0.12},
				Interactive: true,
			},
		},
	})
	if snapshot == nil {
		t.Fatal("BuildStructuredSnapshot() = nil")
	}

	rememberedStableID := ""
	for _, node := range snapshot.Nodes {
		if node.BackendToken == "token-editor-remembered" {
			rememberedStableID = node.StableID
			break
		}
	}
	if rememberedStableID == "" {
		t.Fatal("rememberedStableID = empty, want stable id for remembered composer")
	}

	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		structuredSnapshot: snapshot,
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.chatMemory.RememberAnchor("darwin", "feishu_lark", "message", string(a11yChatStageLocateComposer), rememberedStableID)

	state := newA11yChatExecutionState("darwin", "feishu_lark", "message", "Orca")
	resolvedWindow, attempted, recovered, err := tool.tryA11yChatComposerStructuredRecovery(
		withA11yChatExecutionState(context.Background(), state),
		backend,
		"win-feishu",
	)
	if err != nil {
		t.Fatalf("tryA11yChatComposerStructuredRecovery() error = %v", err)
	}
	if !attempted || !recovered {
		t.Fatalf("attempted/recovered = (%v, %v), want both true", attempted, recovered)
	}
	if resolvedWindow != "win-feishu" {
		t.Fatalf("resolvedWindow = %q, want win-feishu", resolvedWindow)
	}
	if len(backend.pointClickHistory) != 1 {
		t.Fatalf("pointClickHistory = %#v, want one remembered-anchor click", backend.pointClickHistory)
	}
	if backend.pointClickHistory[0].X != 0.44 || backend.pointClickHistory[0].Y != 0.80 {
		t.Fatalf("pointClick = %#v, want remembered composer center", backend.pointClickHistory[0])
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

func TestTryA11yChatComposerStructuredRecovery_UsesComposerConfirmationInsteadOfFixedDelay(t *testing.T) {
	prevDelay := a11yMessageConversationSettleDelay
	a11yMessageConversationSettleDelay = time.Second
	defer func() { a11yMessageConversationSettleDelay = prevDelay }()

	snapshot := a11yruntime.BuildStructuredSnapshot(a11yruntime.BuildStructuredSnapshotOptions{
		WindowID: "win-feishu",
		Title:    "Feishu",
		Mode:     "ax",
	}, &a11yruntime.Node{
		Role: "window",
		Name: "Feishu",
		Children: []*a11yruntime.Node{
			{
				Token:       "token-editor",
				Role:        "document",
				Name:        "Type a message",
				Description: "focused editable",
				Interactive: true,
				Bounds:      a11yruntime.NormalizedRect{X: 0.38, Y: 0.73, Width: 0.12, Height: 0.08},
			},
			{
				Token:       "token-send",
				Role:        "button",
				Name:        "Send",
				Interactive: true,
			},
		},
	})
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		structuredSnapshot: snapshot,
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	state := newA11yChatExecutionState("darwin", "feishu_lark", "message", "Orca")
	ctx, cancel := context.WithTimeout(withA11yChatExecutionState(context.Background(), state), 80*time.Millisecond)
	defer cancel()

	resolvedWindow, attempted, recovered, err := tool.tryA11yChatComposerStructuredRecovery(ctx, backend, "win-feishu")
	if err != nil {
		t.Fatalf("tryA11yChatComposerStructuredRecovery() error = %v", err)
	}
	if !attempted || !recovered {
		t.Fatalf("attempted/recovered = (%v, %v), want both true", attempted, recovered)
	}
	if resolvedWindow != "win-feishu" {
		t.Fatalf("resolvedWindow = %q, want win-feishu", resolvedWindow)
	}
	if len(backend.pointClickHistory) != 1 {
		t.Fatalf("pointClickHistory = %#v, want one composer recovery click", backend.pointClickHistory)
	}
	if backend.interactiveCalls != 0 {
		t.Fatalf("interactiveCalls = %d, want 0 when structured composer confirmation is already available", backend.interactiveCalls)
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

	foundSubmitCompletion := false
	for _, event := range events {
		if event.Type != "stage_changed" {
			t.Fatalf("event.Type = %q, want stage_changed", event.Type)
		}
		if event.ToolName != "computer_use" {
			t.Fatalf("event.ToolName = %q, want computer_use", event.ToolName)
		}
		stage, _ := event.Payload["stage"].(string)
		if stage == "computer_use_chat.verify_outcome" {
			t.Fatalf("events = %#v, do not want standalone verify_outcome stage when submit evidence already confirms success", events)
		}
		if stage == "computer_use_chat.type_or_send" {
			if strategy, _ := event.Payload["strategy"].(string); strategy == "submit_phase_confirmation" {
				foundSubmitCompletion = true
				if status, _ := event.Payload["status"].(string); status != "ok" {
					t.Fatalf("type_or_send submit completion status = %q, want ok", status)
				}
			}
		}
	}
	if !foundSubmitCompletion {
		t.Fatalf("events = %#v, want submit_phase_confirmation on type_or_send stage", events)
	}
}

func TestA11yToolExecute_SelectConversationStopsAfterConfirmConversation(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [list_item] \"Orca\"",
				RefMap: map[int]string{
					1: "token-orca-conversation",
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

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "select",
		"app_name":     "Feishu",
		"conversation": "Orca",
	})
	if err != nil {
		t.Fatalf("select Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["stage"] != "confirm_conversation" {
		t.Fatalf("stage = %v, want confirm_conversation", out["stage"])
	}
	stages, ok := out["task_stages"].([]interface{})
	if !ok || len(stages) < 2 {
		t.Fatalf("task_stages = %#v, want select chat stage trace", out["task_stages"])
	}
	for _, rawStage := range stages {
		stage, _ := rawStage.(map[string]interface{})
		if stage["stage"] == "type_or_send" || stage["stage"] == "verify_outcome" {
			t.Fatalf("task_stages = %#v, do not want submit stages for select conversation flow", out["task_stages"])
		}
	}
}

func TestA11yToolExecute_SelectConversationFailsClosedWhenGrounderRejectsConversationConfirmation(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [list_item] \"Orca\"",
				RefMap: map[int]string{
					1: "token-orca-conversation",
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
		screenshotGroundingBytes: []byte("conversation-grounding"),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.SetChatGrounder(&a11yChatGrounderStub{
		results: map[string]a11yChatGroundingResult{
			string(a11yChatGroundingTaskLocateConversation): {
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
		"action":       "select",
		"app_name":     "Feishu",
		"conversation": "Orca",
	})
	if err != nil {
		t.Fatalf("select Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["phase"] != "conversation" {
		t.Fatalf("phase = %v, want conversation", out["phase"])
	}
	if out["failure_code"] != "search_box_still_active" {
		t.Fatalf("failure_code = %v, want search_box_still_active", out["failure_code"])
	}
	if out["grounding_source"] != "vision_model" {
		t.Fatalf("grounding_source = %v, want vision_model", out["grounding_source"])
	}
	if out["stage"] != "confirm_conversation" {
		t.Fatalf("stage = %v, want confirm_conversation", out["stage"])
	}
}
