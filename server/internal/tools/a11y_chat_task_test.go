package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
