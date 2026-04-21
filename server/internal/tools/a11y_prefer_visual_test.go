package tools

import (
	"context"
	"encoding/json"
	"testing"

	a11yruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

func TestA11yToolExecute_MessagePrefersVisualWhenRequested(t *testing.T) {
	prevLocate := a11yLocateConversationVisualHitFromPNG
	a11yLocateConversationVisualHitFromPNG = func(context.Context, []byte, string) (a11yConversationVisualHit, error) {
		return a11yConversationVisualHit{
			Point:      a11yruntime.NormalizedPoint{X: 0.20, Y: 0.25},
			Confidence: 0.95,
		}, nil
	}
	defer func() { a11yLocateConversationVisualHitFromPNG = prevLocate }()

	backend := &a11yCompatBackend{
		hostOS: "darwin",
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu", Focused: true},
		},
		screenshotGroundingBytes: []byte("grounding-png"),
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
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":        "message",
		"app_name":      "Feishu,飞书,Lark",
		"conversation":  "test_group",
		"value":         "你们好，我是 blue 发的",
		"prefer_visual": true,
		"submit":        false,
	})
	if err != nil {
		t.Fatalf("message Execute() error = %v", err)
	}
	if len(backend.keyHistory) != 0 {
		t.Fatalf("keyHistory = %#v, want none when prefer_visual is enabled and visual locate hits", backend.keyHistory)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["conversation_locate_strategy"] == "" {
		t.Fatalf("conversation_locate_strategy = %#v, want non-empty", out["conversation_locate_strategy"])
	}
}
