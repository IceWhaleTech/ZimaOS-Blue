package tools

import (
	"context"
	"testing"

	a11yruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

func TestTryActivateHostAppForWindowResolve_RetriesUntilFocusedWindowResolved(t *testing.T) {
	backend := &a11yCompatBackend{
		windowsResults: [][]a11yruntime.WindowInfo{
			{
				{ID: "win-1", Title: "Feishu", AppName: "飞书"},
				{ID: "win-2", Title: "Feishu", AppName: "飞书"},
			},
			{
				{ID: "win-1", Title: "Feishu", AppName: "飞书"},
				{ID: "win-2", Title: "Feishu", AppName: "飞书", Focused: true},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	args := map[string]interface{}{
		"action":   "act",
		"app_name": "飞书",
	}
	resolveErr := a11yruntime.NewError("backend_unavailable", "target window is ambiguous", map[string]interface{}{
		"matches": 2,
	})

	resolved, match, _, err, handled := tool.tryActivateHostAppForWindowResolve(context.Background(), backend, args, "", resolveErr)
	if !handled {
		t.Fatalf("handled=false, want true")
	}
	if err != nil {
		t.Fatalf("err=%v, want nil", err)
	}
	if resolved != "win-2" {
		t.Fatalf("resolved=%q, want win-2", resolved)
	}
	if match == nil || match.ResolvedID != "win-2" {
		t.Fatalf("match=%+v, want resolved win-2", match)
	}
	if len(backend.activateAppCalls) != 1 || backend.activateAppCalls[0] != "飞书" {
		t.Fatalf("activateAppCalls=%v, want [飞书]", backend.activateAppCalls)
	}
}

