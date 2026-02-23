package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
)

// BrowserBackend defines the browser interface needed by the BrowserTool.
type BrowserBackend interface {
	Start(ctx context.Context) error
	Navigate(ctx context.Context, url string, targetID string) (BrowserNavResult, error)
	AccessibilityTree(ctx context.Context, targetID string, maxDepth int) (BrowserA11yTreeResult, error)
	InteractiveElements(ctx context.Context, targetID string) (BrowserInteractiveResult, error)
	CountInteractiveElements(ctx context.Context, targetID string) (int, error)
	ActByRef(ctx context.Context, targetID string, ref int, refMap map[int]int, action string, value string) error
	ActByInteractiveRef(ctx context.Context, targetID string, ref int, refMap map[int]string, action string, value string) error
	Screenshot(ctx context.Context, url string) (string, error)
	ScreenshotTab(ctx context.Context, targetID string) (string, error)
	CloseTab(ctx context.Context, targetID string) error
	Tabs(ctx context.Context) ([]BrowserTabResult, error)
}

// BrowserNavResult represents a navigation result.
type BrowserNavResult struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	TargetID string `json:"target_id"`
}

// BrowserA11yTreeResult represents an accessibility tree result.
type BrowserA11yTreeResult struct {
	Tree     string      `json:"tree"`
	URL      string      `json:"url"`
	Title    string      `json:"title"`
	TargetID string      `json:"target_id"`
	RefMap   map[int]int `json:"ref_map,omitempty"`
}

// BrowserInteractiveResult represents interactive elements.
type BrowserInteractiveResult struct {
	Tree     string         `json:"tree"`
	URL      string         `json:"url"`
	Title    string         `json:"title"`
	TargetID string         `json:"target_id"`
	RefMap   map[int]string `json:"ref_map,omitempty"`
	Count    int            `json:"count"`
}

// BrowserTabResult represents a browser tab.
type BrowserTabResult struct {
	TargetID string `json:"target_id"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Active   bool   `json:"active"`
}

// BrowserTool provides headless browser automation as a native tool.
type BrowserTool struct {
	mu                    sync.RWMutex
	backend               BrowserBackend
	lastRefMap            map[int]int
	lastInteractiveRefMap map[int]string
	lastRefMode           string // "a11y" or "interactive"
	lastTarget            string
}

// NewBrowserTool creates a new browser tool.
func NewBrowserTool() *BrowserTool {
	return &BrowserTool{}
}

// SetBackend injects the browser backend.
func (t *BrowserTool) SetBackend(b BrowserBackend) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.backend = b
}

func (t *BrowserTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "browser",
		Description: "Browse the web with a headless browser. Navigate to URLs, read page content via accessibility tree DSL, interact with elements using @ref references, take screenshots. Token-efficient: uses accessibility tree instead of raw HTML.",
		Icon:        "browser",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "Action: navigate (open URL + auto snapshot), snapshot (full CDP accessibility tree), snapshot_interactive (JS interactive elements only), snapshot_auto (auto-pick best strategy), act (interact with @ref element), screenshot (capture page image), tabs (list open tabs), close (close tab)",
				},
				"url": map[string]interface{}{
					"type":        "string",
					"description": "URL to navigate to (required for navigate, screenshot)",
				},
				"ref": map[string]interface{}{
					"type":        "number",
					"description": "Element @ref from accessibility tree DSL (required for act)",
				},
				"act_type": map[string]interface{}{
					"type":        "string",
					"description": "Action type for act: click, type, focus, hover, scroll, select",
				},
				"value": map[string]interface{}{
					"type":        "string",
					"description": "Value for type/select actions",
				},
				"target_id": map[string]interface{}{
					"type":        "string",
					"description": "Tab target ID (optional, defaults to active tab)",
				},
				"vision": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the calling model supports vision/images (used by snapshot_auto to decide strategy)",
				},
			},
			"required": []string{"action"},
		},
	}
}

func (t *BrowserTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	t.mu.RLock()
	backend := t.backend
	t.mu.RUnlock()

	if backend == nil {
		return jsonErr("browser service not available"), nil
	}

	action, _ := args["action"].(string)
	if action == "" {
		return nil, errors.New("action is required")
	}
	targetID, _ := args["target_id"].(string)
	vision, _ := args["vision"].(bool)

	switch action {
	case "navigate":
		return t.doNavigate(ctx, backend, args, vision)
	case "snapshot":
		return t.doSnapshot(ctx, backend, targetID)
	case "snapshot_interactive":
		return t.doSnapshotInteractive(ctx, backend, targetID)
	case "snapshot_auto":
		return t.doAutoSnapshot(ctx, backend, targetID, vision)
	case "act":
		return t.doAct(ctx, backend, args, targetID)
	case "screenshot":
		return t.doScreenshot(ctx, backend, args)
	case "tabs":
		return t.doTabs(ctx, backend)
	case "close":
		return t.doClose(ctx, backend, targetID)
	default:
		return nil, fmt.Errorf("invalid action: %s", action)
	}
}

// --- Actions ---

func (t *BrowserTool) doNavigate(ctx context.Context, b BrowserBackend, args map[string]interface{}, vision bool) (interface{}, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return nil, errors.New("url is required for navigate")
	}
	targetID, _ := args["target_id"].(string)

	_ = b.Start(ctx)
	nav, err := b.Navigate(ctx, url, targetID)
	if err != nil {
		return jsonErr(fmt.Sprintf("navigation failed: %s", err)), nil
	}
	return t.doAutoSnapshot(ctx, b, nav.TargetID, vision)
}

func (t *BrowserTool) doSnapshot(ctx context.Context, b BrowserBackend, targetID string) (interface{}, error) {
	a11y, err := b.AccessibilityTree(ctx, targetID, 10)
	if err != nil {
		return jsonErr(err.Error()), nil
	}
	t.cacheRefs(a11y.TargetID, a11y.RefMap, nil)
	return jsonResult(map[string]interface{}{
		"tree":      a11y.Tree,
		"url":       a11y.URL,
		"title":     a11y.Title,
		"target_id": a11y.TargetID,
		"message":   browserPageMsg(a11y.Title, a11y.URL, a11y.Tree, -1),
	}), nil
}

func (t *BrowserTool) doSnapshotInteractive(ctx context.Context, b BrowserBackend, targetID string) (interface{}, error) {
	result, err := b.InteractiveElements(ctx, targetID)
	if err != nil {
		return jsonErr(err.Error()), nil
	}
	t.cacheRefs(result.TargetID, nil, result.RefMap)
	return jsonResult(map[string]interface{}{
		"tree":      result.Tree,
		"url":       result.URL,
		"title":     result.Title,
		"target_id": result.TargetID,
		"count":     result.Count,
		"strategy":  "interactive",
		"message":   browserPageMsg(result.Title, result.URL, result.Tree, result.Count),
	}), nil
}

const (
	browserInteractiveThreshold = 30
	browserA11yTreeMaxLen       = 6000
)

func (t *BrowserTool) doAutoSnapshot(ctx context.Context, b BrowserBackend, targetID string, vision bool) (interface{}, error) {
	count, err := b.CountInteractiveElements(ctx, targetID)
	if err != nil {
		return t.doSnapshotInteractive(ctx, b, targetID)
	}

	if count <= browserInteractiveThreshold {
		return t.doSnapshotInteractive(ctx, b, targetID)
	}

	if vision {
		return t.doScreenshotWithInteractive(ctx, b, targetID)
	}

	a11y, err := b.AccessibilityTree(ctx, targetID, 10)
	if err != nil {
		return t.doSnapshotInteractive(ctx, b, targetID)
	}

	if len(a11y.Tree) >= browserA11yTreeMaxLen {
		result, err := t.doSnapshotInteractive(ctx, b, targetID)
		if err != nil {
			return result, err
		}
		// Try to append note about complexity
		if s, ok := result.(string); ok {
			var m map[string]interface{}
			if json.Unmarshal([]byte(s), &m) == nil {
				m["note"] = fmt.Sprintf("Page has %d interactive elements and a large DOM. Using interactive elements list. Use 'screenshot' for visual layout.", count)
				b2, _ := json.Marshal(m)
				return string(b2), nil
			}
		}
		return result, nil
	}

	t.cacheA11y(a11y.TargetID, a11y.RefMap)
	return jsonResult(map[string]interface{}{
		"tree":      a11y.Tree,
		"url":       a11y.URL,
		"title":     a11y.Title,
		"target_id": a11y.TargetID,
		"strategy":  "a11y",
		"message":   fmt.Sprintf("Page: %s (%s)\n\n%s", a11y.Title, a11y.URL, a11y.Tree),
	}), nil
}

func (t *BrowserTool) doScreenshotWithInteractive(ctx context.Context, b BrowserBackend, targetID string) (interface{}, error) {
	interactive, err := b.InteractiveElements(ctx, targetID)
	if err != nil {
		data, sErr := b.ScreenshotTab(ctx, targetID)
		if sErr != nil {
			return jsonErr(sErr.Error()), nil
		}
		return jsonResult(map[string]interface{}{
			"screenshot": data,
			"strategy":   "screenshot",
			"message":    "Screenshot captured (interactive elements unavailable)",
		}), nil
	}

	t.cacheRefs(interactive.TargetID, nil, interactive.RefMap)

	data, err := b.ScreenshotTab(ctx, targetID)
	if err != nil {
		return jsonResult(map[string]interface{}{
			"tree":      interactive.Tree,
			"url":       interactive.URL,
			"title":     interactive.Title,
			"target_id": interactive.TargetID,
			"count":     interactive.Count,
			"strategy":  "interactive",
			"message":   browserPageMsg(interactive.Title, interactive.URL, interactive.Tree, interactive.Count),
		}), nil
	}

	return jsonResult(map[string]interface{}{
		"screenshot": data,
		"tree":       interactive.Tree,
		"url":        interactive.URL,
		"title":      interactive.Title,
		"target_id":  interactive.TargetID,
		"count":      interactive.Count,
		"strategy":   "screenshot+interactive",
		"message":    "Page: " + interactive.Title + " (" + interactive.URL + ") — screenshot + " + strconv.Itoa(interactive.Count) + " interactive elements\n\n" + interactive.Tree,
	}), nil
}

func (t *BrowserTool) doAct(ctx context.Context, b BrowserBackend, args map[string]interface{}, targetID string) (interface{}, error) {
	ref := 0
	switch r := args["ref"].(type) {
	case float64:
		ref = int(r)
	case int:
		ref = r
	}
	actType, _ := args["act_type"].(string)
	if actType == "" {
		return nil, errors.New("act_type is required for act")
	}
	value, _ := args["value"].(string)

	t.mu.RLock()
	refMode := t.lastRefMode
	a11yRefMap := t.lastRefMap
	interactiveRefMap := t.lastInteractiveRefMap
	cachedTarget := t.lastTarget
	t.mu.RUnlock()

	if a11yRefMap == nil && interactiveRefMap == nil {
		return jsonErr("no page snapshot loaded — use navigate, snapshot, or snapshot_interactive first"), nil
	}

	if targetID == "" {
		targetID = cachedTarget
	}

	var actErr error
	if refMode == "interactive" && interactiveRefMap != nil {
		actErr = b.ActByInteractiveRef(ctx, targetID, ref, interactiveRefMap, actType, value)
	} else {
		actErr = b.ActByRef(ctx, targetID, ref, a11yRefMap, actType, value)
	}
	if actErr != nil {
		return jsonErr(fmt.Sprintf("act @%d %s failed: %s", ref, actType, actErr)), nil
	}

	return jsonResult(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Performed %s on @%d", actType, ref),
	}), nil
}

func (t *BrowserTool) doScreenshot(ctx context.Context, b BrowserBackend, args map[string]interface{}) (interface{}, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return nil, errors.New("url is required for screenshot")
	}
	_ = b.Start(ctx)
	data, err := b.Screenshot(ctx, url)
	if err != nil {
		return jsonErr(err.Error()), nil
	}
	return jsonResult(map[string]interface{}{
		"screenshot": data,
		"message":    fmt.Sprintf("Screenshot captured for %s", url),
	}), nil
}

func (t *BrowserTool) doTabs(ctx context.Context, b BrowserBackend) (interface{}, error) {
	tabs, err := b.Tabs(ctx)
	if err != nil {
		return jsonErr(err.Error()), nil
	}
	return jsonResult(map[string]interface{}{
		"tabs":    tabs,
		"count":   len(tabs),
		"message": fmt.Sprintf("%d open tabs", len(tabs)),
	}), nil
}

func (t *BrowserTool) doClose(ctx context.Context, b BrowserBackend, targetID string) (interface{}, error) {
	if targetID == "" {
		t.mu.RLock()
		targetID = t.lastTarget
		t.mu.RUnlock()
	}
	if targetID == "" {
		return jsonErr("no tab to close — specify target_id"), nil
	}
	if err := b.CloseTab(ctx, targetID); err != nil {
		return jsonErr(err.Error()), nil
	}
	return jsonResult(map[string]interface{}{
		"closed":  true,
		"message": fmt.Sprintf("Tab %s closed", targetID),
	}), nil
}

// --- Ref caching ---

func (t *BrowserTool) cacheRefs(targetID string, a11yRefs map[int]int, interactiveRefs map[int]string) {
	t.mu.Lock()
	t.lastRefMap = a11yRefs
	t.lastInteractiveRefMap = interactiveRefs
	if a11yRefs != nil {
		t.lastRefMode = "a11y"
	} else {
		t.lastRefMode = "interactive"
	}
	t.lastTarget = targetID
	t.mu.Unlock()
}

func (t *BrowserTool) cacheA11y(targetID string, refMap map[int]int) {
	t.mu.Lock()
	t.lastRefMap = refMap
	t.lastInteractiveRefMap = nil
	t.lastRefMode = "a11y"
	t.lastTarget = targetID
	t.mu.Unlock()
}

// --- Helpers ---

func jsonResult(data map[string]interface{}) string {
	b, _ := json.Marshal(data)
	return string(b)
}

func jsonErr(msg string) string {
	b, _ := json.Marshal(map[string]interface{}{"error": msg})
	return string(b)
}

// RegisterBrowserTool registers the browser tool with the registry.
func RegisterBrowserTool(registry *Registry, backend BrowserBackend) {
	t := NewBrowserTool()
	if backend != nil {
		t.SetBackend(backend)
	}
	registry.Register(t)
}

// GetBrowserTool retrieves the BrowserTool from the registry for dependency injection.
func GetBrowserTool(registry *Registry) *BrowserTool {
	tool := registry.Get("browser")
	if tool == nil {
		return nil
	}
	if t, ok := tool.(*BrowserTool); ok {
		return t
	}
	return nil
}

func browserPageMsg(title, url, tree string, count int) string {
	n := len("Page: ") + len(title) + len(" (") + len(url) + len(")")
	if count >= 0 {
		n += len(" — ") + 10 + len(" interactive elements")
	}
	n += 2 + len(tree)
	buf := make([]byte, 0, n)
	buf = append(buf, "Page: "...)
	buf = append(buf, title...)
	buf = append(buf, " ("...)
	buf = append(buf, url...)
	buf = append(buf, ')')
	if count >= 0 {
		buf = append(buf, " — "...)
		buf = append(buf, strconv.Itoa(count)...)
		buf = append(buf, " interactive elements"...)
	}
	buf = append(buf, "\n\n"...)
	buf = append(buf, tree...)
	return string(buf)
}
