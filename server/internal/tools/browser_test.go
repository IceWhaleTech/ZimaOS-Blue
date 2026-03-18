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
	navigateURL      string
	navigateTargetID string
	recipeName       string
	recipeParams     map[string]string
	screenshotData   string
}

func (b *browserCompatBackend) Start(context.Context) error { return nil }
func (b *browserCompatBackend) Navigate(_ context.Context, url string, targetID string) (BrowserNavResult, error) {
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
func (b *browserCompatBackend) AccessibilityTree(context.Context, string, int) (BrowserA11yTreeResult, error) {
	return BrowserA11yTreeResult{}, nil
}
func (b *browserCompatBackend) InteractiveElements(context.Context, string) (BrowserInteractiveResult, error) {
	return BrowserInteractiveResult{Tree: "[@1] button \"OK\"", URL: "https://example.com", Title: "Example", TargetID: "tab-9", RefMap: map[int]string{1: "button"}, Count: 1}, nil
}
func (b *browserCompatBackend) CountInteractiveElements(context.Context, string) (int, error) {
	return 1, nil
}
func (b *browserCompatBackend) ActByRef(context.Context, string, int, map[int]int, string, string) error {
	return nil
}
func (b *browserCompatBackend) ActByInteractiveRef(context.Context, string, int, map[int]string, string, string) error {
	return nil
}
func (b *browserCompatBackend) Screenshot(context.Context, string) (string, error) {
	if b.screenshotData != "" {
		return b.screenshotData, nil
	}
	return "", nil
}
func (b *browserCompatBackend) ScreenshotTab(context.Context, string) (string, error) { return "", nil }
func (b *browserCompatBackend) CloseTab(context.Context, string) error                { return nil }
func (b *browserCompatBackend) Tabs(context.Context) ([]BrowserTabResult, error)      { return nil, nil }
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
