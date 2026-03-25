package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type browserCompatBackend struct {
	navigateURL               string
	navigateTargetID          string
	lastRouteHint             BrowserRouteHint
	recipeName                string
	recipeParams              map[string]string
	screenshotData            string
	screenshotTabData         string
	lastScreenshotURL         string
	lastScreenshotTabTargetID string
	lastActMode               string
	lastActTargetID           string
	lastActRef                int
	lastActAction             string
	lastActValue              string
	usesRelay                 bool
}

func (b *browserCompatBackend) Start(context.Context) error { return nil }
func (b *browserCompatBackend) UsesRelay(context.Context, string) bool {
	return b.usesRelay
}
func (b *browserCompatBackend) Navigate(ctx context.Context, url string, targetID string) (BrowserNavResult, error) {
	if ctxHint := GetBrowserRouteHint(ctx); ctxHint.Action != "" {
		b.lastRouteHint = ctxHint
	}
	b.navigateURL = url
	b.navigateTargetID = targetID
	if targetID == "" {
		targetID = "tab-1"
	}
	return BrowserNavResult{URL: url, Title: "Example", TargetID: targetID}, nil
}
func (b *browserCompatBackend) CookieHeader(context.Context, string, string) (string, error) {
	return "", nil
}
func (b *browserCompatBackend) ObserveNetwork(context.Context, string, int, bool) (BrowserObservedNetworkResult, error) {
	return BrowserObservedNetworkResult{}, nil
}
func (b *browserCompatBackend) WaitNetworkIdle(context.Context, string, int, int) error {
	return nil
}
func (b *browserCompatBackend) AccessibilityTree(context.Context, string, int) (BrowserA11yTreeResult, error) {
	return BrowserA11yTreeResult{}, nil
}
func (b *browserCompatBackend) InteractiveElements(context.Context, string) (BrowserInteractiveResult, error) {
	return BrowserInteractiveResult{Tree: "[@1] button \"OK\"", URL: "https://example.com", Title: "Example", TargetID: "tab-9", RefMap: map[int]string{1: "button"}, Count: 1}, nil
}
func (b *browserCompatBackend) CountInteractiveElements(context.Context, string) (int, error) {
	return 1, nil
}
func (b *browserCompatBackend) ActByRef(_ context.Context, targetID string, ref int, _ map[int]int, action string, value string) error {
	b.lastActMode = "a11y"
	b.lastActTargetID = targetID
	b.lastActRef = ref
	b.lastActAction = action
	b.lastActValue = value
	return nil
}
func (b *browserCompatBackend) ActByInteractiveRef(_ context.Context, targetID string, ref int, _ map[int]string, action string, value string) error {
	b.lastActMode = "interactive"
	b.lastActTargetID = targetID
	b.lastActRef = ref
	b.lastActAction = action
	b.lastActValue = value
	return nil
}
func (b *browserCompatBackend) Screenshot(_ context.Context, url string) (string, error) {
	b.lastScreenshotURL = url
	if b.screenshotData != "" {
		return b.screenshotData, nil
	}
	return "", nil
}
func (b *browserCompatBackend) ScreenshotTab(_ context.Context, targetID string) (string, error) {
	b.lastScreenshotTabTargetID = targetID
	if b.screenshotTabData != "" {
		return b.screenshotTabData, nil
	}
	return "", nil
}
func (b *browserCompatBackend) CloseTab(context.Context, string) error           { return nil }
func (b *browserCompatBackend) Tabs(context.Context) ([]BrowserTabResult, error) { return nil, nil }
func (b *browserCompatBackend) ExecuteRecipe(_ context.Context, recipe string, params map[string]string) (BrowserRecipeResult, error) {
	b.recipeName = recipe
	b.recipeParams = params
	return BrowserRecipeResult{Success: true, Message: "ok", Data: map[string]interface{}{"recipe": recipe}}, nil
}
func (b *browserCompatBackend) ListRecipes(context.Context) []BrowserRecipeInfo { return nil }

func TestBrowserToolExecuteSupportsNestedCamelCaseArgs(t *testing.T) {
	backend := &browserCompatBackend{}
	tool := NewBrowserTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"action":   "navigate",
			"href":     "https://example.com/page",
			"targetId": "tab-9",
			"vision":   true,
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if backend.navigateURL != "https://example.com/page" {
		t.Fatalf("navigateURL = %q, want https://example.com/page", backend.navigateURL)
	}
	if backend.navigateTargetID != "tab-9" {
		t.Fatalf("navigateTargetID = %q, want tab-9", backend.navigateTargetID)
	}
	if backend.lastRouteHint.Action != "navigate" || backend.lastRouteHint.FollowupAction != "snapshot_auto" {
		t.Fatalf("lastRouteHint = %#v, want navigate/snapshot_auto", backend.lastRouteHint)
	}
	if !backend.lastRouteHint.Vision || !backend.lastRouteHint.RequiresImage {
		t.Fatalf("lastRouteHint = %#v, want vision/image hint", backend.lastRouteHint)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["target_id"] != "tab-9" {
		t.Fatalf("target_id = %v, want tab-9", out["target_id"])
	}
}

func TestBrowserToolRecipeSupportsNestedCamelCaseArgs(t *testing.T) {
	backend := &browserCompatBackend{}
	tool := NewBrowserTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"action":     "recipe",
			"recipeName": "search",
			"params": map[string]interface{}{
				"query": "zima",
				"page":  2,
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if backend.recipeName != "search" {
		t.Fatalf("recipeName = %q, want search", backend.recipeName)
	}
	if backend.recipeParams["query"] != "zima" || backend.recipeParams["page"] != "2" {
		t.Fatalf("recipeParams = %#v", backend.recipeParams)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if got := out["recipe"]; got != "search" {
		t.Fatalf("recipe = %v, want search", got)
	}
}

func TestBrowserToolExecuteCanonicalizesLegacyTopLevelActAction(t *testing.T) {
	backend := &browserCompatBackend{}
	tool := NewBrowserTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "snapshot_interactive",
	}); err != nil {
		t.Fatalf("snapshot_interactive error = %v", err)
	}

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "scroll",
		"ref":    1,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if backend.lastActMode != "interactive" {
		t.Fatalf("lastActMode = %q, want interactive", backend.lastActMode)
	}
	if backend.lastActAction != "scroll" {
		t.Fatalf("lastActAction = %q, want scroll", backend.lastActAction)
	}
	if backend.lastActRef != 1 {
		t.Fatalf("lastActRef = %d, want 1", backend.lastActRef)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if got := out["success"]; got != true {
		t.Fatalf("success = %v, want true", got)
	}
}

func TestBrowserToolExecuteAcceptsAtPrefixedRef(t *testing.T) {
	backend := &browserCompatBackend{}
	tool := NewBrowserTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "snapshot_interactive",
	}); err != nil {
		t.Fatalf("snapshot_interactive error = %v", err)
	}

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "act",
		"ref":      "@1",
		"act_type": "click",
	}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if backend.lastActRef != 1 {
		t.Fatalf("lastActRef = %d, want 1", backend.lastActRef)
	}
}

func TestBrowserToolExecuteLegacyTopLevelActActionRequiresRef(t *testing.T) {
	backend := &browserCompatBackend{}
	tool := NewBrowserTool()
	tool.SetBackend(backend)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "scroll",
	})
	if err == nil {
		t.Fatal("expected missing ref error")
	}
	if !strings.Contains(err.Error(), "ref is required for act") {
		t.Fatalf("error = %q, want missing ref message", err.Error())
	}
}

func TestBrowserToolScreenshotSavesMediaURLWhenMediaDirConfigured(t *testing.T) {
	const pngBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII="

	backend := &browserCompatBackend{screenshotData: pngBase64}
	tool := NewBrowserTool()
	tool.SetBackend(backend)
	mediaDir := t.TempDir()
	tool.SetMediaDir(mediaDir)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "screenshot",
		"url":    "https://example.com",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	got, _ := out["screenshot"].(string)
	if got == "" {
		t.Fatal("expected screenshot file path")
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("screenshot = %q, want absolute path", got)
	}
	if !strings.HasPrefix(got, filepath.Join(mediaDir, "browser")+string(os.PathSeparator)) {
		t.Fatalf("screenshot = %q, want under media dir %q", got, filepath.Join(mediaDir, "browser"))
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("expected saved screenshot at %s: %v", got, err)
	}
}

func TestBrowserToolScreenshotSavesTempFileWhenMediaDirMissing(t *testing.T) {
	const pngBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII="

	backend := &browserCompatBackend{screenshotData: pngBase64}
	tool := NewBrowserTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "screenshot",
		"url":    "https://example.com",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	got, _ := out["screenshot"].(string)
	if got == "" {
		t.Fatal("expected screenshot file path")
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("screenshot = %q, want absolute path", got)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("expected saved screenshot at %s: %v", got, err)
	}
}

func TestBrowserToolScreenshotUsesTargetIDWithoutURL(t *testing.T) {
	const pngBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII="

	backend := &browserCompatBackend{screenshotTabData: pngBase64}
	tool := NewBrowserTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "screenshot",
		"target_id": "tab-42",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if backend.lastScreenshotURL != "" {
		t.Fatalf("lastScreenshotURL = %q, want empty", backend.lastScreenshotURL)
	}
	if backend.lastScreenshotTabTargetID != "tab-42" {
		t.Fatalf("lastScreenshotTabTargetID = %q, want tab-42", backend.lastScreenshotTabTargetID)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if got := out["target_id"]; got != "tab-42" {
		t.Fatalf("target_id = %v, want tab-42", got)
	}
	if got := out["message"]; got != "Screenshot captured for tab tab-42" {
		t.Fatalf("message = %v, want tab message", got)
	}
}

func TestBrowserToolScreenshotUsesActiveTabWhenURLAndTargetMissing(t *testing.T) {
	const pngBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII="

	backend := &browserCompatBackend{screenshotTabData: pngBase64}
	tool := NewBrowserTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "screenshot",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if backend.lastScreenshotURL != "" {
		t.Fatalf("lastScreenshotURL = %q, want empty", backend.lastScreenshotURL)
	}
	if backend.lastScreenshotTabTargetID != "" {
		t.Fatalf("lastScreenshotTabTargetID = %q, want empty for active-tab fallback", backend.lastScreenshotTabTargetID)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if got := out["message"]; got != "Screenshot captured for active tab" {
		t.Fatalf("message = %v, want active-tab message", got)
	}
}

func TestBrowserToolRelayModeRequiresCheckpoint(t *testing.T) {
	backend := &browserCompatBackend{usesRelay: true}
	tool := NewBrowserTool()
	tool.SetBackend(backend)

	var seen int
	ctx := WithSessionID(context.Background(), "conv-relay")
	ctx = WithUserID(ctx, "user-relay")
	ctx = WithBrowserCheckpointRequester(ctx, func(_ context.Context, req BrowserCheckpointRequest) (BrowserCheckpointResult, error) {
		seen++
		if req.Step != "relay" {
			t.Fatalf("checkpoint step = %q, want relay", req.Step)
		}
		if req.Action != "list_connected_tabs" {
			t.Fatalf("checkpoint action = %q, want list_connected_tabs", req.Action)
		}
		return BrowserCheckpointResult{Decision: BrowserCheckpointApprove, CheckpointID: "cp-1"}, nil
	})

	raw, err := tool.Execute(ctx, map[string]interface{}{"action": "tabs"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if seen != 1 {
		t.Fatalf("checkpoint seen = %d, want 1", seen)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if got := out["count"]; got != float64(0) {
		t.Fatalf("count = %v, want 0", got)
	}

	_, err = tool.Execute(ctx, map[string]interface{}{"action": "tabs"})
	if err != nil {
		t.Fatalf("second Execute() error = %v", err)
	}
	if seen != 1 {
		t.Fatalf("checkpoint seen after cached approval = %d, want 1", seen)
	}
}

func TestBrowserToolRelayModePendingCheckpointShortCircuits(t *testing.T) {
	backend := &browserCompatBackend{usesRelay: true}
	tool := NewBrowserTool()
	tool.SetBackend(backend)

	ctx := WithSessionID(context.Background(), "conv-relay-pending")
	ctx = WithBrowserCheckpointRequester(ctx, func(_ context.Context, req BrowserCheckpointRequest) (BrowserCheckpointResult, error) {
		return BrowserCheckpointResult{
			Pending:      true,
			CheckpointID: "cp-pending",
			Message:      "need confirmation",
		}, nil
	})

	raw, err := tool.Execute(ctx, map[string]interface{}{
		"action": "navigate",
		"url":    "https://example.com/account",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if backend.navigateURL != "" {
		t.Fatalf("navigateURL = %q, want empty because execution should pause", backend.navigateURL)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if got := out["checkpoint_pending"]; got != true {
		t.Fatalf("checkpoint_pending = %v, want true", got)
	}
	if got := out["checkpoint_id"]; got != "cp-pending" {
		t.Fatalf("checkpoint_id = %v, want cp-pending", got)
	}
}
