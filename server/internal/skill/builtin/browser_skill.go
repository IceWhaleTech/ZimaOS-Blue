package builtin

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

// BrowserServiceInterface defines the interface for browser service used by the skill.
type BrowserServiceInterface interface {
	Start(ctx context.Context) error
	Navigate(ctx context.Context, url string, targetID string) (BrowserNavResult, error)
	AccessibilityTree(ctx context.Context, targetID string, maxDepth int) (BrowserA11yResult, error)
	InteractiveElements(ctx context.Context, targetID string) (BrowserInteractiveResult, error)
	CountInteractiveElements(ctx context.Context, targetID string) (int, error)
	ActByRef(ctx context.Context, targetID string, ref int, refMap map[int]int, action string, value string) error
	ActByInteractiveRef(ctx context.Context, targetID string, ref int, refMap map[int]string, action string, value string) error
	Screenshot(ctx context.Context, url string) (string, error) // returns base64
	ScreenshotTab(ctx context.Context, targetID string) (string, error)
	CloseTab(ctx context.Context, targetID string) error
	Tabs(ctx context.Context) ([]BrowserTabInfo, error)
}

// BrowserNavResult represents a navigation result.
type BrowserNavResult struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	TargetID string `json:"target_id"`
}

// BrowserA11yResult represents an accessibility tree result.
type BrowserA11yResult struct {
	Tree     string         `json:"tree"`
	URL      string         `json:"url"`
	Title    string         `json:"title"`
	TargetID string         `json:"target_id"`
	RefMap   map[int]int    `json:"ref_map,omitempty"`
}

// BrowserInteractiveResult represents a JS-extracted interactive elements result.
type BrowserInteractiveResult struct {
	Tree     string         `json:"tree"`
	URL      string         `json:"url"`
	Title    string         `json:"title"`
	TargetID string         `json:"target_id"`
	RefMap   map[int]string `json:"ref_map,omitempty"`
	Count    int            `json:"count"`
}

// BrowserTabInfo represents a browser tab.
type BrowserTabInfo struct {
	TargetID string `json:"target_id"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Active   bool   `json:"active"`
}

// Browser is a built-in skill for browser automation.
// It uses an accessibility tree DSL for token-efficient page understanding.
type Browser struct {
	manifest *skill.Manifest
	mu       sync.RWMutex
	svc      BrowserServiceInterface
	// Per-session ref map cache (last snapshot's refs)
	lastRefMap            map[int]int
	lastInteractiveRefMap map[int]string
	lastRefMode           string // "a11y" or "interactive"
	lastTarget            string
}

// NewBrowser creates a new browser skill.
func NewBrowser() *Browser {
	return &Browser{
		manifest: &skill.Manifest{
			ID:          "browser",
			Name:        "Browser",
			Version:     "1.0.0",
			Description: "Browse the web with a headless browser. Navigate to URLs, read page content via accessibility tree DSL, interact with elements using @ref references, take screenshots. Token-efficient: uses accessibility tree instead of raw HTML.",
			Category:    "system",
			Icon:        "browser",
			Tags:        []string{"browser", "web", "scrape", "automate", "navigate", "accessibility"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action: navigate (open URL + auto snapshot), snapshot (full CDP accessibility tree), snapshot_interactive (JS interactive elements only), snapshot_auto (auto-pick best strategy), act (interact with @ref element), screenshot (capture page image), tabs (list open tabs), close (close tab)",
					Required:    true,
				},
				{
					Name:        "url",
					Type:        "string",
					Description: "URL to navigate to (required for navigate, screenshot)",
					Required:    false,
				},
				{
					Name:        "ref",
					Type:        "number",
					Description: "Element @ref from accessibility tree DSL (required for act)",
					Required:    false,
				},
				{
					Name:        "act_type",
					Type:        "string",
					Description: "Action type for act: click, type, focus, hover, scroll, select",
					Required:    false,
				},
				{
					Name:        "value",
					Type:        "string",
					Description: "Value for type/select actions",
					Required:    false,
				},
				{
					Name:        "target_id",
					Type:        "string",
					Description: "Tab target ID (optional, defaults to active tab)",
					Required:    false,
				},
				{
					Name:        "vision",
					Type:        "boolean",
					Description: "Whether the calling model supports vision/images (used by snapshot_auto to decide strategy)",
					Required:    false,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "tree",
					Type:        "string",
					Description: "Accessibility tree DSL of the page",
				},
				{
					Name:        "screenshot",
					Type:        "string",
					Description: "Base64-encoded screenshot",
				},
			},
		},
	}
}

// SetBrowserService injects the browser service.
func (b *Browser) SetBrowserService(svc BrowserServiceInterface) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.svc = svc
}

// cacheRefs stores the latest snapshot's ref map and target for subsequent act calls.
func (b *Browser) cacheRefs(targetID string, a11yRefs map[int]int, interactiveRefs map[int]string) {
	b.mu.Lock()
	b.lastRefMap = a11yRefs
	b.lastInteractiveRefMap = interactiveRefs
	if a11yRefs != nil {
		b.lastRefMode = "a11y"
	} else {
		b.lastRefMode = "interactive"
	}
	b.lastTarget = targetID
	b.mu.Unlock()
}

func (b *Browser) Manifest() *skill.Manifest { return b.manifest }

func (b *Browser) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}
	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}
	switch actionStr {
	case "navigate", "snapshot", "snapshot_interactive", "snapshot_auto", "act", "screenshot", "tabs", "close":
		// valid
	default:
		return fmt.Errorf("invalid action: %s", actionStr)
	}
	switch actionStr {
	case "navigate", "screenshot":
		if _, ok := input["url"]; !ok {
			return fmt.Errorf("url is required for %s", actionStr)
		}
	case "act":
		if _, ok := input["ref"]; !ok {
			return fmt.Errorf("ref is required for act (use @N from the accessibility tree)")
		}
		if _, ok := input["act_type"]; !ok {
			return fmt.Errorf("act_type is required for act (click, type, focus, hover, scroll, select)")
		}
	}
	return nil
}

func (b *Browser) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	b.mu.RLock()
	svc := b.svc
	b.mu.RUnlock()

	if svc == nil {
		return skill.NewErrorResult(fmt.Errorf("browser service not available")), nil
	}

	action := input["action"].(string)
	targetID, _ := input["target_id"].(string)
	vision, _ := input["vision"].(bool)

	switch action {
	case "navigate":
		url := input["url"].(string)
		_ = svc.Start(ctx)

		nav, err := svc.Navigate(ctx, url, targetID)
		if err != nil {
			return skill.NewErrorResult(fmt.Errorf("navigation failed: %w", err)), nil
		}

		// Auto-snapshot after navigation
		return b.autoSnapshot(ctx, svc, nav.TargetID, vision)

	case "snapshot":
		a11y, err := svc.AccessibilityTree(ctx, targetID, 10)
		if err != nil {
			return skill.NewErrorResult(err), nil
		}

		b.cacheRefs(a11y.TargetID, a11y.RefMap, nil)

		return skill.NewResult(map[string]any{
			"tree":      a11y.Tree,
			"url":       a11y.URL,
			"title":     a11y.Title,
			"target_id": a11y.TargetID,
			"message":   pageMessage(a11y.Title, a11y.URL, a11y.Tree, -1),
		}), nil

	case "snapshot_interactive":
		return b.doSnapshotInteractive(ctx, svc, targetID)

	case "snapshot_auto":
		return b.autoSnapshot(ctx, svc, targetID, vision)

	case "act":
		ref := 0
		switch r := input["ref"].(type) {
		case float64:
			ref = int(r)
		case int:
			ref = r
		}
		actType := input["act_type"].(string)
		value, _ := input["value"].(string)

		b.mu.RLock()
		refMode := b.lastRefMode
		a11yRefMap := b.lastRefMap
		interactiveRefMap := b.lastInteractiveRefMap
		cachedTarget := b.lastTarget
		b.mu.RUnlock()

		if a11yRefMap == nil && interactiveRefMap == nil {
			return skill.NewErrorResult(fmt.Errorf("no page snapshot loaded — use navigate, snapshot, or snapshot_interactive first")), nil
		}

		if targetID == "" {
			targetID = cachedTarget
		}

		var actErr error
		if refMode == "interactive" && interactiveRefMap != nil {
			actErr = svc.ActByInteractiveRef(ctx, targetID, ref, interactiveRefMap, actType, value)
		} else {
			actErr = svc.ActByRef(ctx, targetID, ref, a11yRefMap, actType, value)
		}
		if actErr != nil {
			return skill.NewErrorResult(fmt.Errorf("act @%d %s failed: %w", ref, actType, actErr)), nil
		}

		return skill.NewResult(map[string]any{
			"success": true,
			"message": fmt.Sprintf("Performed %s on @%d", actType, ref),
		}), nil

	case "screenshot":
		url := input["url"].(string)
		_ = svc.Start(ctx)

		data, err := svc.Screenshot(ctx, url)
		if err != nil {
			return skill.NewErrorResult(err), nil
		}

		return skill.NewResult(map[string]any{
			"screenshot": data,
			"message":    fmt.Sprintf("Screenshot captured for %s", url),
		}), nil

	case "tabs":
		tabs, err := svc.Tabs(ctx)
		if err != nil {
			return skill.NewErrorResult(err), nil
		}
		return skill.NewResult(map[string]any{
			"tabs":    tabs,
			"count":   len(tabs),
			"message": fmt.Sprintf("%d open tabs", len(tabs)),
		}), nil

	case "close":
		if targetID == "" {
			b.mu.RLock()
			targetID = b.lastTarget
			b.mu.RUnlock()
		}
		if targetID == "" {
			return skill.NewErrorResult(fmt.Errorf("no tab to close — specify target_id")), nil
		}
		if err := svc.CloseTab(ctx, targetID); err != nil {
			return skill.NewErrorResult(err), nil
		}
		return skill.NewResult(map[string]any{
			"closed":  true,
			"message": fmt.Sprintf("Tab %s closed", targetID),
		}), nil
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

// Adaptive snapshot thresholds
const (
	// Pages with ≤ this many interactive elements use snapshot_interactive
	interactiveThreshold = 30
	// A11y tree DSL longer than this is considered "too large" → fall back
	a11yTreeMaxLen = 6000
)

// pageMessage builds the human-readable message for snapshot results.
func pageMessage(title, url, tree string, count int) string {
	// "Page: Title (url) — N interactive elements\n\nTree"
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

// autoSnapshot picks the best snapshot strategy based on page complexity and model capabilities.
//
// Strategy:
//  1. Count interactive elements (cheap JS call)
//  2. If ≤ 30 → snapshot_interactive (compact, all elements are actionable)
//  3. If > 30 and vision → screenshot + interactive elements (visual layout + action refs)
//  4. If > 30 and no vision → try a11y tree; if truncated → fall back to interactive + warning
func (b *Browser) autoSnapshot(ctx context.Context, svc BrowserServiceInterface, targetID string, vision bool) (*skill.Result, error) {
	count, err := svc.CountInteractiveElements(ctx, targetID)
	if err != nil {
		// Can't count — fall back to interactive snapshot
		return b.doSnapshotInteractive(ctx, svc, targetID)
	}

	// Simple page: interactive elements list is enough
	if count <= interactiveThreshold {
		return b.doSnapshotInteractive(ctx, svc, targetID)
	}

	// Complex page with vision: screenshot + interactive list
	if vision {
		return b.doScreenshotWithInteractive(ctx, svc, targetID)
	}

	// Complex page without vision: try a11y tree, fall back if too large
	a11y, err := svc.AccessibilityTree(ctx, targetID, 10)
	if err != nil {
		// A11y failed — fall back to interactive
		return b.doSnapshotInteractive(ctx, svc, targetID)
	}

	// Check if tree was truncated (too large for token budget)
	if len(a11y.Tree) >= a11yTreeMaxLen {
		// Tree too large — use interactive list instead, attach truncation note
		result, err := b.doSnapshotInteractive(ctx, svc, targetID)
		if err != nil {
			return result, err
		}
		// Append note about complexity
		if data, ok := result.Data.(map[string]any); ok {
			data["note"] = fmt.Sprintf("Page has %d interactive elements and a large DOM. Using interactive elements list. Use 'screenshot' for visual layout.", count)
		}
		return result, nil
	}

	// A11y tree fits — use it
	b.mu.Lock()
	b.lastRefMap = a11y.RefMap
	b.lastInteractiveRefMap = nil
	b.lastRefMode = "a11y"
	b.lastTarget = a11y.TargetID
	b.mu.Unlock()

	return skill.NewResult(map[string]any{
		"tree":      a11y.Tree,
		"url":       a11y.URL,
		"title":     a11y.Title,
		"target_id": a11y.TargetID,
		"strategy":  "a11y",
		"message":   fmt.Sprintf("Page: %s (%s)\n\n%s", a11y.Title, a11y.URL, a11y.Tree),
	}), nil
}

// doSnapshotInteractive extracts interactive elements and caches the ref map.
func (b *Browser) doSnapshotInteractive(ctx context.Context, svc BrowserServiceInterface, targetID string) (*skill.Result, error) {
	result, err := svc.InteractiveElements(ctx, targetID)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	b.cacheRefs(result.TargetID, nil, result.RefMap)

	return skill.NewResult(map[string]any{
		"tree":      result.Tree,
		"url":       result.URL,
		"title":     result.Title,
		"target_id": result.TargetID,
		"count":     result.Count,
		"strategy":  "interactive",
		"message":   pageMessage(result.Title, result.URL, result.Tree, result.Count),
	}), nil
}

// doScreenshotWithInteractive returns a screenshot + interactive elements list.
// Best for vision-capable models on complex pages.
func (b *Browser) doScreenshotWithInteractive(ctx context.Context, svc BrowserServiceInterface, targetID string) (*skill.Result, error) {
	// Get interactive elements for action refs
	interactive, err := svc.InteractiveElements(ctx, targetID)
	if err != nil {
		// Fall back to screenshot only
		data, sErr := svc.ScreenshotTab(ctx, targetID)
		if sErr != nil {
			return skill.NewErrorResult(sErr), nil
		}
		return skill.NewResult(map[string]any{
			"screenshot": data,
			"strategy":   "screenshot",
			"message":    "Screenshot captured (interactive elements unavailable)",
		}), nil
	}

	b.cacheRefs(interactive.TargetID, nil, interactive.RefMap)

	// Take screenshot
	data, err := svc.ScreenshotTab(ctx, targetID)
	if err != nil {
		// Screenshot failed — return interactive only
		return skill.NewResult(map[string]any{
			"tree":      interactive.Tree,
			"url":       interactive.URL,
			"title":     interactive.Title,
			"target_id": interactive.TargetID,
			"count":     interactive.Count,
			"strategy":  "interactive",
			"message":   pageMessage(interactive.Title, interactive.URL, interactive.Tree, interactive.Count),
		}), nil
	}

	return skill.NewResult(map[string]any{
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
