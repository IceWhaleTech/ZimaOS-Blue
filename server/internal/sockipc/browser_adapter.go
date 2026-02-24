package sockipc

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// ToolBrowserIPCAdapter adapts tools.BrowserBackend to the sockipc.BrowserBackend interface.
type ToolBrowserIPCAdapter struct {
	backend tools.BrowserBackend
}

// NewToolBrowserIPCAdapter creates a new adapter from a tools.BrowserBackend.
func NewToolBrowserIPCAdapter(backend tools.BrowserBackend) *ToolBrowserIPCAdapter {
	return &ToolBrowserIPCAdapter{backend: backend}
}

func (a *ToolBrowserIPCAdapter) Start(ctx context.Context) error {
	return a.backend.Start(ctx)
}

func (a *ToolBrowserIPCAdapter) Navigate(ctx context.Context, url string, targetID string) (map[string]string, error) {
	resp, err := a.backend.Navigate(ctx, url, targetID)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"url":       resp.URL,
		"title":     resp.Title,
		"target_id": resp.TargetID,
	}, nil
}

func (a *ToolBrowserIPCAdapter) AccessibilityTree(ctx context.Context, targetID string, maxDepth int) (map[string]string, error) {
	resp, err := a.backend.AccessibilityTree(ctx, targetID, maxDepth)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"tree":      resp.Tree,
		"url":       resp.URL,
		"title":     resp.Title,
		"target_id": resp.TargetID,
	}, nil
}

func (a *ToolBrowserIPCAdapter) InteractiveElements(ctx context.Context, targetID string) (map[string]string, error) {
	resp, err := a.backend.InteractiveElements(ctx, targetID)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"tree":      resp.Tree,
		"url":       resp.URL,
		"title":     resp.Title,
		"target_id": resp.TargetID,
		"count":     strconv.Itoa(resp.Count),
	}, nil
}

func (a *ToolBrowserIPCAdapter) Screenshot(ctx context.Context, url string) (string, error) {
	return a.backend.Screenshot(ctx, url)
}

func (a *ToolBrowserIPCAdapter) ScreenshotTab(ctx context.Context, targetID string) (string, error) {
	return a.backend.ScreenshotTab(ctx, targetID)
}

func (a *ToolBrowserIPCAdapter) Act(ctx context.Context, targetID string, ref int, actType, value string) error {
	// IPC doesn't carry refMap — assume ref IS the backend DOM node ID (direct mapping).
	refMap := map[int]int{ref: ref}
	return a.backend.ActByRef(ctx, targetID, ref, refMap, actType, value)
}

func (a *ToolBrowserIPCAdapter) Tabs(ctx context.Context) (string, error) {
	tabs, err := a.backend.Tabs(ctx)
	if err != nil {
		return "", err
	}
	type tabJSON struct {
		TargetID string `json:"target_id"`
		URL      string `json:"url"`
		Title    string `json:"title"`
		Active   bool   `json:"active"`
	}
	out := make([]tabJSON, len(tabs))
	for i, t := range tabs {
		out[i] = tabJSON{TargetID: t.TargetID, URL: t.URL, Title: t.Title, Active: t.Active}
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}

func (a *ToolBrowserIPCAdapter) CloseTab(ctx context.Context, targetID string) error {
	return a.backend.CloseTab(ctx, targetID)
}
