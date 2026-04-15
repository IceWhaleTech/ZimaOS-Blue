package tools

import (
	"context"
	"encoding/json"
	"testing"

	a11yruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

type a11yQueryCompatBackend struct {
	a11yCompatBackend
	resolveResult a11yruntime.TargetResolution
	resolveErr    error
	resolveCalls  int
}

func (b *a11yQueryCompatBackend) ResolveTarget(_ context.Context, windowID string, selector a11yruntime.TargetSelector) (a11yruntime.TargetResolution, error) {
	b.resolveCalls++
	if b.resolveErr != nil {
		return a11yruntime.TargetResolution{}, b.resolveErr
	}
	result := b.resolveResult
	if result.WindowID == "" {
		result.WindowID = windowID
	}
	return result, nil
}

func TestA11yToolExecute_ActUsesCachedQueryResolverWithoutInteractiveSnapshot(t *testing.T) {
	backend := &a11yQueryCompatBackend{
		a11yCompatBackend: a11yCompatBackend{
			hostOS: "darwin",
			windows: []a11yruntime.WindowInfo{
				{ID: "win-1", Title: "Feishu", AppName: "Feishu", Focused: true},
			},
			actResultSet: true,
			actResult: a11yruntime.ActionResult{
				WindowID:           "win-1",
				ExecutionMode:      "semantic",
				VerificationPassed: true,
				VerificationMethod: "ocr",
				ActionTelemetry: a11yruntime.ActionTelemetry{
					ActionMS:       11,
					VerificationMS: 4,
					EndToEndMS:     15,
				},
			},
		},
		resolveResult: a11yruntime.TargetResolution{
			WindowID:         "win-1",
			Ref:              7,
			RefMap:           map[int]string{7: "token-message"},
			Tree:             `@7 [text_field] "Type a message"`,
			Token:            "token-message",
			SnapshotRevision: 9,
			CacheHit:         true,
			NodeCount:        42,
			CandidateCount:   2,
			QueryMS:          3,
		},
	}

	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "act",
		"window_id": "win-1",
		"act_type":  "type",
		"value":     "hello",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if backend.resolveCalls != 1 {
		t.Fatalf("resolveCalls = %d, want 1", backend.resolveCalls)
	}
	if backend.interactiveCalls != 0 {
		t.Fatalf("interactiveCalls = %d, want 0", backend.interactiveCalls)
	}
	if backend.lastActRef != 7 {
		t.Fatalf("lastActRef = %d, want 7", backend.lastActRef)
	}

	payload := map[string]interface{}{}
	if err := json.Unmarshal([]byte(raw.(string)), &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got := payload["cache_hit"]; got != true {
		t.Fatalf("cache_hit = %#v, want true", got)
	}
	if got := payload["snapshot_revision"]; got != float64(9) {
		t.Fatalf("snapshot_revision = %#v, want 9", got)
	}
	if got := payload["candidate_count"]; got != float64(2) {
		t.Fatalf("candidate_count = %#v, want 2", got)
	}
	if got := payload["query_ms"]; got != float64(3) {
		t.Fatalf("query_ms = %#v, want 3", got)
	}
	if got := payload["action_ms"]; got != float64(11) {
		t.Fatalf("action_ms = %#v, want 11", got)
	}
}

func TestA11yToolExecute_SnapshotExposesTelemetryFields(t *testing.T) {
	backend := &a11yCompatBackend{
		hostOS: "windows",
		windows: []a11yruntime.WindowInfo{
			{ID: "win-1", Title: "Settings", Focused: true},
		},
		snapshotResult: a11yruntime.SnapshotResult{
			HostOS:   "windows",
			WindowID: "win-1",
			Title:    "Settings",
			Tree:     `@1 [switch] "Notifications"`,
			RefMap:   map[int]string{1: "token-toggle"},
			ActionTelemetry: a11yruntime.ActionTelemetry{
				SnapshotRevision: 5,
				CacheHit:         true,
				NodeCount:        17,
				TreeFetchMS:      12,
				TreeSerializeMS:  3,
				EndToEndMS:       15,
				Fallbacks:        []string{"watch_unavailable"},
			},
		},
	}

	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "snapshot",
		"window_id": "win-1",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	payload := map[string]interface{}{}
	if err := json.Unmarshal([]byte(raw.(string)), &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got := payload["snapshot_revision"]; got != float64(5) {
		t.Fatalf("snapshot_revision = %#v, want 5", got)
	}
	if got := payload["cache_hit"]; got != true {
		t.Fatalf("cache_hit = %#v, want true", got)
	}
	if got := payload["node_count"]; got != float64(17) {
		t.Fatalf("node_count = %#v, want 17", got)
	}
	if got := payload["tree_fetch_ms"]; got != float64(12) {
		t.Fatalf("tree_fetch_ms = %#v, want 12", got)
	}
	if got := payload["fallbacks"]; got == nil {
		t.Fatal("fallbacks missing from snapshot payload")
	}
}
