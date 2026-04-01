package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// BrowserServiceInterface defines the interface for browser service used by the skill.
type BrowserServiceInterface interface {
	Start(ctx context.Context) error
	Navigate(ctx context.Context, url string, targetID string) (BrowserNavResult, error)
	ExtractText(ctx context.Context, targetID, selector string) (string, error)
	AccessibilityTree(ctx context.Context, targetID string, maxDepth int) (BrowserA11yResult, error)
	InteractiveElements(ctx context.Context, targetID string) (BrowserInteractiveResult, error)
	CountInteractiveElements(ctx context.Context, targetID string) (int, error)
	ActByRef(ctx context.Context, targetID string, ref int, refMap map[int]int, action string, value string) error
	ActByInteractiveRef(ctx context.Context, targetID string, ref int, refMap map[int]string, action string, value string) error
	Screenshot(ctx context.Context, url string) (string, error) // returns base64
	ScreenshotTab(ctx context.Context, targetID string) (string, error)
	CloseTab(ctx context.Context, targetID string) error
	Tabs(ctx context.Context) ([]BrowserTabInfo, error)
	ExecuteRecipe(ctx context.Context, recipe string, params map[string]string) (BrowserRecipeResult, error)
	ListRecipes(ctx context.Context) []BrowserRecipeInfo
}

// BrowserNavResult represents a navigation result.
type BrowserNavResult struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	TargetID string `json:"target_id"`
}

// BrowserA11yResult represents an accessibility tree result.
type BrowserA11yResult struct {
	Tree     string      `json:"tree"`
	URL      string      `json:"url"`
	Title    string      `json:"title"`
	TargetID string      `json:"target_id"`
	RefMap   map[int]int `json:"ref_map,omitempty"`
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

// BrowserRecipeResult represents the result of a recipe execution.
type BrowserRecipeResult struct {
	Success  bool                   `json:"success"`
	Data     map[string]interface{} `json:"data,omitempty"`
	TargetID string                 `json:"target_id,omitempty"`
	Message  string                 `json:"message"`
}

// BrowserRecipeInfo describes an available recipe.
type BrowserRecipeInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	KeepTab     bool   `json:"keep_tab"`
}

// Browser is a built-in skill for browser automation.
// It uses an accessibility tree DSL for token-efficient page understanding.
type Browser struct {
	manifest *skill.Manifest
	mu       sync.RWMutex
	svc      BrowserServiceInterface
	mediaDir string
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
			Description: "Open a URL, read page content (accessibility tree), interact with elements (@ref), take screenshots. For keyword search or public-page reads use web_query; for UI quality scoring use ui_reviewer.",
			Category:    "system",
			Icon:        "browser",
			Tags:        []string{"browser", "web", "scrape", "automate", "navigate", "accessibility"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action: navigate (open URL + auto snapshot), snapshot (full CDP accessibility tree), snapshot_interactive (JS interactive elements only), snapshot_auto (auto-pick best strategy), act (interact with @ref element), screenshot (capture page image), tabs (list open tabs), close (close tab), recipe (run a predefined automation template), recipes (list available recipes)",
					Required:    true,
				},
				{
					Name:        "url",
					Type:        "string",
					Description: "URL to navigate to (required for navigate; optional for screenshot when capturing the current tab or a target_id tab)",
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
				{
					Name:        "locale",
					Type:        "string",
					Description: "Language/locale code for localized responses (e.g., en-US, zh-CN)",
					Required:    false,
				},
				{
					Name:        "recipe",
					Type:        "string",
					Description: "Recipe name for action=recipe (search, fill_form, extract, login)",
					Required:    false,
				},
				{
					Name:        "params",
					Type:        "object",
					Description: "Recipe parameters as key-value pairs (e.g., {\"query\": \"test\", \"engine\": \"google\"})",
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
					Description: "Saved screenshot file path",
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

// SetMediaDir configures the public media directory used for persisted screenshots.
func (b *Browser) SetMediaDir(dir string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.mediaDir = dir
}

func (b *Browser) normalizeScreenshotPayload(data string) string {
	b.mu.RLock()
	mediaDir := b.mediaDir
	b.mu.RUnlock()
	if strings.TrimSpace(data) == "" {
		return data
	}
	savedPath, err := tools.SaveBrowserScreenshotBase64(mediaDir, data)
	if err != nil || savedPath == "" {
		return data
	}
	return savedPath
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
	normalizeBrowserSkillInput(input)

	// Default action to "navigate" when url is provided without action.
	if _, ok := input["action"]; !ok {
		if _, hasURL := input["url"].(string); hasURL {
			input["action"] = "navigate"
		} else {
			return fmt.Errorf("action is required")
		}
	}
	actionStr, ok := input["action"].(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}
	actType, _ := input["act_type"].(string)
	if actType == "" {
		actType, _ = input["actType"].(string)
	}
	actionStr, actType = tools.CanonicalizeBrowserAction(actionStr, actType)
	switch actionStr {
	case "navigate", "snapshot", "snapshot_interactive", "snapshot_auto", "act", "screenshot", "tabs", "close", "recipe", "recipes":
		// valid
	default:
		return fmt.Errorf("invalid action: %s", actionStr)
	}
	switch actionStr {
	case "navigate":
		if _, ok := input["url"]; !ok {
			return fmt.Errorf("url is required for %s", actionStr)
		}
	case "act":
		if _, ok := input["ref"]; !ok {
			return fmt.Errorf("ref is required for act (pass 12 or \"@12\" from the accessibility tree)")
		}
		if actType == "" {
			return fmt.Errorf("act_type is required for act (click, type, focus, hover, scroll, select)")
		}
	case "recipe":
		if _, ok := input["recipe"]; !ok {
			return fmt.Errorf("recipe is required for action=recipe (search, fill_form, extract, login)")
		}
	}
	return nil
}

func normalizeBrowserSkillInput(input map[string]any) {
	normalizeStringAlias(input, "action", "op", "operation", "command")
	normalizeStringAlias(input, "url", "href")
	normalizeStringAlias(input, "target_id", "targetId")
	normalizeStringAlias(input, "act_type", "actType")
	normalizeStringAlias(input, "value", "text")
	normalizeStringAlias(input, "recipe", "recipe_name", "recipeName")
}

func (b *Browser) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	if err := b.Validate(input); err != nil {
		return skill.NewErrorResult(err), nil
	}

	b.mu.RLock()
	svc := b.svc
	b.mu.RUnlock()

	if svc == nil {
		return skill.NewErrorResult(fmt.Errorf("browser service not available")), nil
	}

	action, _ := input["action"].(string)
	actType, _ := input["act_type"].(string)
	action, actType = tools.CanonicalizeBrowserAction(action, actType)
	if action == "" {
		return skill.NewErrorResult(fmt.Errorf("action is required")), nil
	}
	input["action"] = action
	if actType != "" {
		input["act_type"] = actType
	}
	targetID, _ := input["target_id"].(string)
	vision, _ := input["vision"].(bool)

	switch action {
	case "navigate":
		url, _ := input["url"].(string)
		if url == "" {
			return skill.NewErrorResult(fmt.Errorf("url is required for navigate")), nil
		}
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

		payload := map[string]any{
			"tree":      a11y.Tree,
			"url":       a11y.URL,
			"title":     a11y.Title,
			"target_id": a11y.TargetID,
			"message":   pageMessage(a11y.Title, a11y.URL, a11y.Tree, -1),
		}
		b.maybeAugmentSnapshotWithReadableContent(ctx, svc, payload)
		return skill.NewResult(payload), nil

	case "snapshot_interactive":
		return b.doSnapshotInteractive(ctx, svc, targetID)

	case "snapshot_auto":
		return b.autoSnapshot(ctx, svc, targetID, vision)

	case "act":
		rawRef, ok := input["ref"]
		if !ok {
			return skill.NewErrorResult(fmt.Errorf("ref is required for act (pass 12 or \"@12\" from the accessibility tree)")), nil
		}
		ref, ok := tools.CoerceBrowserRef(rawRef)
		if !ok {
			return skill.NewErrorResult(fmt.Errorf("ref must be an integer or @N string for act")), nil
		}
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
		url, _ := input["url"].(string)
		targetID, _ := input["target_id"].(string)
		_ = svc.Start(ctx)

		var (
			data    string
			err     error
			message string
		)
		switch {
		case url != "":
			data, err = svc.Screenshot(ctx, url)
			message = fmt.Sprintf("Screenshot captured for %s", url)
		case targetID != "":
			data, err = svc.ScreenshotTab(ctx, targetID)
			message = fmt.Sprintf("Screenshot captured for tab %s", targetID)
		default:
			data, err = svc.ScreenshotTab(ctx, "")
			message = "Screenshot captured for active tab"
		}
		if err != nil {
			return skill.NewErrorResult(err), nil
		}
		data = b.normalizeScreenshotPayload(data)

		out := map[string]any{
			"screenshot": data,
			"message":    message,
		}
		if targetID != "" {
			out["target_id"] = targetID
		}
		if url != "" {
			out["url"] = url
		}
		return skill.NewResult(out), nil

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

	case "recipe":
		recipeName, _ := input["recipe"].(string)
		params := extractRecipeParams(input)
		_ = svc.Start(ctx)
		result, err := svc.ExecuteRecipe(ctx, recipeName, params)
		if err != nil {
			return skill.NewErrorResult(fmt.Errorf("recipe %s failed: %w", recipeName, err)), nil
		}
		data := map[string]any{
			"success": result.Success,
			"message": result.Message,
		}
		for k, v := range result.Data {
			data[k] = v
		}
		if result.TargetID != "" {
			data["target_id"] = result.TargetID
		}
		return skill.NewResult(data), nil

	case "recipes":
		infos := svc.ListRecipes(ctx)
		return skill.NewResult(map[string]any{
			"recipes": infos,
			"count":   len(infos),
			"message": fmt.Sprintf("%d recipes available", len(infos)),
		}), nil
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

// Adaptive snapshot thresholds
const (
	// Pages with ≤ this many interactive elements use snapshot_interactive
	interactiveThreshold = 30
	// A11y tree DSL longer than this is considered "too large" → fall back
	a11yTreeMaxLen          = 6000
	readableContentMaxLen   = 3200
	readableContentMinChars = 140
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

	payload := map[string]any{
		"tree":      a11y.Tree,
		"url":       a11y.URL,
		"title":     a11y.Title,
		"target_id": a11y.TargetID,
		"strategy":  "a11y",
		"message":   fmt.Sprintf("Page: %s (%s)\n\n%s", a11y.Title, a11y.URL, a11y.Tree),
	}
	b.maybeAugmentSnapshotWithReadableContent(ctx, svc, payload)
	return skill.NewResult(payload), nil
}

// doSnapshotInteractive extracts interactive elements and caches the ref map.
func (b *Browser) doSnapshotInteractive(ctx context.Context, svc BrowserServiceInterface, targetID string) (*skill.Result, error) {
	result, err := svc.InteractiveElements(ctx, targetID)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	b.cacheRefs(result.TargetID, nil, result.RefMap)

	payload := map[string]any{
		"tree":      result.Tree,
		"url":       result.URL,
		"title":     result.Title,
		"target_id": result.TargetID,
		"count":     result.Count,
		"strategy":  "interactive",
		"message":   pageMessage(result.Title, result.URL, result.Tree, result.Count),
	}
	b.maybeAugmentSnapshotWithReadableContent(ctx, svc, payload)
	return skill.NewResult(payload), nil
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
		data = b.normalizeScreenshotPayload(data)
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
		payload := map[string]any{
			"tree":      interactive.Tree,
			"url":       interactive.URL,
			"title":     interactive.Title,
			"target_id": interactive.TargetID,
			"count":     interactive.Count,
			"strategy":  "interactive",
			"message":   pageMessage(interactive.Title, interactive.URL, interactive.Tree, interactive.Count),
		}
		b.maybeAugmentSnapshotWithReadableContent(ctx, svc, payload)
		return skill.NewResult(payload), nil
	}
	data = b.normalizeScreenshotPayload(data)

	payload := map[string]any{
		"screenshot": data,
		"tree":       interactive.Tree,
		"url":        interactive.URL,
		"title":      interactive.Title,
		"target_id":  interactive.TargetID,
		"count":      interactive.Count,
		"strategy":   "screenshot+interactive",
		"message":    "Page: " + interactive.Title + " (" + interactive.URL + ") — screenshot + " + strconv.Itoa(interactive.Count) + " interactive elements\n\n" + interactive.Tree,
	}
	b.maybeAugmentSnapshotWithReadableContent(ctx, svc, payload)
	return skill.NewResult(payload), nil
}

func (b *Browser) maybeAugmentSnapshotWithReadableContent(ctx context.Context, svc BrowserServiceInterface, payload map[string]any) {
	if payload == nil || svc == nil {
		return
	}
	url, _ := payload["url"].(string)
	title, _ := payload["title"].(string)
	tree, _ := payload["tree"].(string)
	targetID, _ := payload["target_id"].(string)
	count := browserCountFromPayload(payload["count"])
	if !browserShouldTryReadableContent(url, title, tree, count) {
		return
	}

	content, err := extractReadableSkillBrowserContent(ctx, svc, targetID, url)
	if err != nil || strings.TrimSpace(content) == "" {
		return
	}

	payload["content"] = content
	payload["content_format"] = "text"
	payload["content_strategy"] = "extract_recipe"
	if tree == "" {
		payload["message"] = fmt.Sprintf("Page: %s (%s)\n\nMain content:\n%s", title, url, content)
		return
	}
	sectionLabel := "Page structure"
	switch strings.TrimSpace(fmt.Sprintf("%v", payload["strategy"])) {
	case "interactive", "screenshot+interactive":
		sectionLabel = "Interactive elements"
	}
	payload["message"] = fmt.Sprintf("Page: %s (%s)\n\nMain content:\n%s\n\n%s:\n%s", title, url, content, sectionLabel, tree)
}

func browserShouldTryReadableContent(url, title, tree string, count int) bool {
	if strings.TrimSpace(url) == "" {
		return false
	}
	if !browserLooksDocumentationPage(url, title) {
		return false
	}
	if strings.TrimSpace(tree) == "" {
		return true
	}
	if count >= interactiveThreshold {
		return true
	}
	if browserTreeLooksNavigationHeavy(tree) {
		return true
	}
	return true
}

func browserLooksDocumentationPage(url, title string) bool {
	lowerURL := strings.ToLower(strings.TrimSpace(url))
	lowerTitle := strings.ToLower(strings.TrimSpace(title))
	switch {
	case strings.Contains(lowerURL, "/docs/"):
		return true
	case strings.Contains(lowerURL, "/reference/"):
		return true
	case strings.Contains(lowerURL, "/api/reference/"):
		return true
	case strings.Contains(lowerURL, "/guides/"):
		return true
	case strings.Contains(lowerTitle, "api reference"):
		return true
	case strings.Contains(lowerTitle, "documentation"):
		return true
	case strings.Contains(lowerTitle, "developer docs"):
		return true
	default:
		return false
	}
}

func browserTreeLooksNavigationHeavy(tree string) bool {
	lines := strings.Split(strings.TrimSpace(tree), "\n")
	total := 0
	navLines := 0
	textLines := 0
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		total++
		lower := strings.ToLower(line)
		if strings.Contains(lower, "[a]") || strings.Contains(lower, "[link]") || strings.Contains(lower, "[menuitem]") || strings.Contains(lower, "[button]") {
			navLines++
		}
		if strings.Contains(line, ". ") || strings.Contains(line, ": ") {
			textLines++
		}
	}
	if total == 0 {
		return false
	}
	return navLines*100/total >= 70 && textLines <= 2
}

func extractReadableSkillBrowserContent(ctx context.Context, svc BrowserServiceInterface, targetID, url string) (string, error) {
	if strings.TrimSpace(targetID) != "" {
		if content, err := extractReadableSkillBrowserContentFromTab(ctx, svc, targetID); err == nil && strings.TrimSpace(content) != "" {
			return content, nil
		}
	}
	return extractReadableSkillBrowserContentViaRecipe(ctx, svc, url)
}

func extractReadableSkillBrowserContentFromTab(ctx context.Context, svc BrowserServiceInterface, targetID string) (string, error) {
	selectors := []string{
		"main",
		"article",
		`[role="main"]`,
		".sl-markdown-content",
		"[data-pagefind-body]",
		".docs-content",
		".documentation",
		".doc-content",
		".content",
		".markdown-body",
		".prose",
		"[data-content]",
		".content-area",
		".docs-page",
	}
	best := ""
	for _, selector := range selectors {
		text, err := svc.ExtractText(ctx, targetID, selector)
		if err != nil {
			continue
		}
		candidate := cleanReadableBrowserContent(text)
		if len([]rune(candidate)) > len([]rune(best)) {
			best = candidate
		}
	}
	heading, _ := svc.ExtractText(ctx, targetID, "h1")
	heading = cleanReadableBrowserContent(heading)
	if heading != "" && !strings.Contains(strings.ToLower(best), strings.ToLower(heading)) {
		best = strings.TrimSpace(heading + "\n\n" + best)
	}
	if len([]rune(best)) < readableContentMinChars {
		return "", fmt.Errorf("readable browser content too short")
	}
	return truncateReadableBrowserContent(best, readableContentMaxLen), nil
}

func extractReadableSkillBrowserContentViaRecipe(ctx context.Context, svc BrowserServiceInterface, url string) (string, error) {
	selectors, err := json.Marshal(map[string]string{
		"main_content":      "main, article, [role='main'], .sl-markdown-content, [data-pagefind-body], .docs-content, .documentation, .doc-content, .content",
		"secondary_content": ".markdown-body, .prose, [data-content], .content-area, .docs-page",
		"page_heading":      "h1",
	})
	if err != nil {
		return "", err
	}
	result, err := svc.ExecuteRecipe(ctx, "extract", map[string]string{
		"url":       url,
		"selectors": string(selectors),
	})
	if err != nil {
		return "", err
	}
	if !result.Success {
		if strings.TrimSpace(result.Message) != "" {
			return "", fmt.Errorf("%s", result.Message)
		}
		return "", fmt.Errorf("browser extract recipe failed")
	}

	extracted, _ := result.Data["extracted"].(map[string]interface{})
	candidates := []string{
		cleanReadableBrowserContent(browserStringValue(extracted["main_content"])),
		cleanReadableBrowserContent(browserStringValue(extracted["secondary_content"])),
	}
	best := ""
	for _, candidate := range candidates {
		if len([]rune(candidate)) > len([]rune(best)) {
			best = candidate
		}
	}
	heading := cleanReadableBrowserContent(browserStringValue(extracted["page_heading"]))
	if heading != "" && !strings.Contains(strings.ToLower(best), strings.ToLower(heading)) {
		best = strings.TrimSpace(heading + "\n\n" + best)
	}
	if len([]rune(best)) < readableContentMinChars {
		return "", fmt.Errorf("readable browser content too short")
	}
	return truncateReadableBrowserContent(best, readableContentMaxLen), nil
}

func cleanReadableBrowserContent(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(raw, "\n")
	cleaned := make([]string, 0, len(lines))
	previousBlank := false
	for _, line := range lines {
		line = strings.Join(strings.Fields(strings.TrimSpace(line)), " ")
		if line == "" {
			if previousBlank {
				continue
			}
			previousBlank = true
			cleaned = append(cleaned, "")
			continue
		}
		previousBlank = false
		cleaned = append(cleaned, line)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func truncateReadableBrowserContent(raw string, max int) string {
	if max <= 0 {
		return raw
	}
	runes := []rune(raw)
	if len(runes) <= max {
		return raw
	}
	return strings.TrimSpace(string(runes[:max]))
}

func browserCountFromPayload(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func browserStringValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// extractRecipeParams extracts recipe params from the skill input.
// Supports both params as a map[string]any and as a JSON string.
func extractRecipeParams(input map[string]any) map[string]string {
	params := make(map[string]string)

	// Try params as map
	if p, ok := input["params"].(map[string]any); ok {
		for k, v := range p {
			params[k] = fmt.Sprintf("%v", v)
		}
		return params
	}

	// Try params as JSON string
	if p, ok := input["params"].(string); ok && p != "" {
		var m map[string]string
		if json.Unmarshal([]byte(p), &m) == nil {
			return m
		}
	}

	// Fall back: extract known recipe params from top-level input
	for _, key := range []string{"query", "engine", "max_results", "url", "fields", "selectors", "multiple", "submit", "username", "password", "username_selector", "password_selector", "submit_selector"} {
		if v, ok := input[key]; ok {
			params[key] = fmt.Sprintf("%v", v)
		}
	}
	return params
}
