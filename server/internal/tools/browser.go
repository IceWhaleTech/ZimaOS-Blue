package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// BrowserBackend defines the browser interface needed by the BrowserTool.
type BrowserBackend interface {
	Start(ctx context.Context) error
	Navigate(ctx context.Context, url string, targetID string) (BrowserNavResult, error)
	CookieHeader(ctx context.Context, targetID string, url string) (string, error)
	ObserveNetwork(ctx context.Context, targetID string, maxEntries int, clear bool) (BrowserObservedNetworkResult, error)
	WaitNetworkIdle(ctx context.Context, targetID string, idleMS int, timeoutMS int) error
	AccessibilityTree(ctx context.Context, targetID string, maxDepth int) (BrowserA11yTreeResult, error)
	InteractiveElements(ctx context.Context, targetID string) (BrowserInteractiveResult, error)
	CountInteractiveElements(ctx context.Context, targetID string) (int, error)
	ActByRef(ctx context.Context, targetID string, ref int, refMap map[int]int, action string, value string) error
	ActByInteractiveRef(ctx context.Context, targetID string, ref int, refMap map[int]string, action string, value string) error
	Screenshot(ctx context.Context, url string) (string, error)
	ScreenshotTab(ctx context.Context, targetID string) (string, error)
	CloseTab(ctx context.Context, targetID string) error
	Tabs(ctx context.Context) ([]BrowserTabResult, error)
	ExecuteRecipe(ctx context.Context, recipe string, params map[string]string) (BrowserRecipeResult, error)
	ListRecipes(ctx context.Context) []BrowserRecipeInfo
}

type browserPageScrollCompat interface {
	PageScroll(ctx context.Context, targetID string, x, y int) error
}

// BrowserNavResult represents a navigation result.
type BrowserNavResult struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	TargetID string `json:"target_id"`
}

// BrowserNetworkEvent represents an observed request/response pair from a real browser session.
type BrowserNetworkEvent struct {
	Method       string            `json:"method,omitempty"`
	URL          string            `json:"url,omitempty"`
	Status       int               `json:"status,omitempty"`
	ContentType  string            `json:"content_type,omitempty"`
	ResourceType string            `json:"resource_type,omitempty"`
	Initiator    string            `json:"initiator,omitempty"`
	DurationMS   int64             `json:"duration_ms,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	BodySample   string            `json:"body_sample,omitempty"`
}

// BrowserObservedNetworkResult summarizes recent network activity for a tab.
type BrowserObservedNetworkResult struct {
	TargetID string                `json:"target_id,omitempty"`
	Events   []BrowserNetworkEvent `json:"events,omitempty"`
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

// BrowserTool provides browser automation as a native tool.
type BrowserTool struct {
	mu                    sync.RWMutex
	backend               BrowserBackend
	mediaDir              string
	lastRefMap            map[int]int
	lastInteractiveRefMap map[int]string
	lastRefMode           string // "a11y" or "interactive"
	lastTarget            string
	relayApprovedSessions map[string]struct{}
}

// NewBrowserTool creates a new browser tool.
func NewBrowserTool() *BrowserTool {
	return &BrowserTool{
		relayApprovedSessions: make(map[string]struct{}),
	}
}

// SetBackend injects the browser backend.
func (t *BrowserTool) SetBackend(b BrowserBackend) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.backend = b
}

// SetMediaDir configures the public media directory used for persisted screenshots.
func (t *BrowserTool) SetMediaDir(dir string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.mediaDir = strings.TrimSpace(dir)
}

// Backend returns the current browser backend.
func (t *BrowserTool) Backend() BrowserBackend {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.backend
}

func (t *BrowserTool) normalizeScreenshotPayload(data string) string {
	t.mu.RLock()
	mediaDir := t.mediaDir
	t.mu.RUnlock()
	if strings.TrimSpace(data) == "" {
		return data
	}
	savedPath, err := SaveBrowserScreenshotBase64(mediaDir, data)
	if err != nil || savedPath == "" {
		return data
	}
	return savedPath
}

type relayAwareBrowserBackend interface {
	UsesRelay(ctx context.Context, targetID string) bool
}

type readableContentBrowserBackend interface {
	ExtractText(ctx context.Context, targetID, selector string) (string, error)
}

func (t *BrowserTool) relayApprovalKey(ctx context.Context) string {
	sessionID := strings.TrimSpace(GetSessionID(ctx))
	userID := strings.TrimSpace(GetUserID(ctx))
	switch {
	case userID != "" && sessionID != "":
		return userID + ":" + sessionID
	case sessionID != "":
		return sessionID
	default:
		return ""
	}
}

func (t *BrowserTool) relayApprovedForSession(key string) bool {
	if strings.TrimSpace(key) == "" {
		return false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, ok := t.relayApprovedSessions[key]
	return ok
}

func (t *BrowserTool) markRelayApprovedForSession(key string) {
	if strings.TrimSpace(key) == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.relayApprovedSessions == nil {
		t.relayApprovedSessions = make(map[string]struct{})
	}
	t.relayApprovedSessions[key] = struct{}{}
}

func (t *BrowserTool) maybeRequireRelayApproval(ctx context.Context, b BrowserBackend, targetID, checkpointURL, action string) (interface{}, bool, error) {
	if relayBackend, ok := b.(relayURLAwareBrowserBackend); ok {
		if !relayBackend.UsesRelayFor(ctx, targetID, checkpointURL) {
			return nil, false, nil
		}
	} else {
		relayBackend, ok := b.(relayAwareBrowserBackend)
		if !ok || !relayBackend.UsesRelay(ctx, targetID) {
			return nil, false, nil
		}
	}

	sessionKey := t.relayApprovalKey(ctx)
	if t.relayApprovedForSession(sessionKey) {
		return nil, false, nil
	}

	if strings.TrimSpace(checkpointURL) == "" {
		checkpointURL = t.resolveCheckpointURL(ctx, b, targetID)
	}

	cpResult, hasRequester, cpErr := RequestBrowserCheckpoint(ctx, BrowserCheckpointRequest{
		Required:  true,
		RiskLevel: "high",
		Step:      "relay",
		Action:    action,
		URL:       checkpointURL,
	})
	if cpErr != nil {
		return jsonErr(fmt.Sprintf("relay checkpoint failed: %s", cpErr)), true, nil
	}
	if !hasRequester {
		return nil, false, nil
	}
	if cpResult.Pending {
		return jsonResult(map[string]interface{}{
			"checkpoint_pending": true,
			"checkpoint_id":      cpResult.CheckpointID,
			"resume_required":    true,
			"message":            cpResult.Message,
		}), true, nil
	}
	if cpResult.Decision != BrowserCheckpointApprove {
		return jsonErr("relay browser session access denied by user"), true, nil
	}

	t.markRelayApprovedForSession(sessionKey)
	return nil, false, nil
}

func (t *BrowserTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "browser",
		Description: "Live browser fallback for login walls, JS-heavy pages, interactive actions, screenshots, and tab/session reuse. Prefer web_query for ordinary reading.",
		Icon:        "browser",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "navigate, snapshot, snapshot_interactive, snapshot_auto, act, screenshot, tabs, close, recipe, or recipes",
				},
				"url": map[string]interface{}{
					"type":        "string",
					"description": "Target URL for navigate, or optional screenshot URL.",
				},
				"target_id": map[string]interface{}{
					"type":        "string",
					"description": "Optional tab target ID. Defaults to the active tab when supported.",
				},
				"recipe": map[string]interface{}{
					"type":        "string",
					"description": "Recipe name for action=recipe.",
				},
				"params": map[string]interface{}{
					"type":                 "object",
					"description":          "Action details. For act use ref/act_type/value. For recipe pass recipe inputs. Optional vision=true enables visual follow-up when supported.",
					"additionalProperties": true,
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

	action := firstCompatString(args, "action", "op", "operation", "command")
	if action == "" {
		return nil, errors.New("action is required")
	}
	rawAction := strings.ToLower(strings.TrimSpace(action))
	action, actType := CanonicalizeBrowserAction(action, firstCompatString(args, "act_type", "actType"))
	if rawAction == "read" {
		if firstCompatString(args, "url", "href") != "" {
			action = "navigate"
		} else {
			action = "snapshot_auto"
		}
	}
	if actType != "" {
		args["act_type"] = actType
	}
	targetID := firstCompatString(args, "target_id", "targetId")
	vision, _ := compatBoolArg(args, "vision")
	navURL := firstCompatString(args, "url", "href")

	switch action {
	case "navigate":
		return t.doNavigate(ctx, backend, args, vision)
	case "snapshot":
		if targetID == "" && navURL != "" {
			return t.doNavigate(ctx, backend, args, vision)
		}
		return t.doSnapshot(ctx, backend, targetID)
	case "snapshot_interactive":
		if targetID == "" && navURL != "" {
			return t.doNavigate(ctx, backend, args, vision)
		}
		return t.doSnapshotInteractive(ctx, backend, targetID)
	case "snapshot_auto":
		if targetID == "" && navURL != "" {
			return t.doNavigate(ctx, backend, args, vision)
		}
		return t.doAutoSnapshot(ctx, backend, targetID, vision)
	case "act":
		return t.doAct(ctx, backend, args, targetID)
	case "scroll_page":
		return t.doPageScroll(ctx, backend, targetID, actType)
	case "screenshot":
		return t.doScreenshot(ctx, backend, args)
	case "tabs":
		return t.doTabs(ctx, backend)
	case "close":
		return t.doClose(ctx, backend, targetID)
	case "recipe":
		return t.doRecipe(ctx, backend, args)
	case "recipes":
		return t.doListRecipes(ctx, backend)
	default:
		return nil, fmt.Errorf("invalid action: %s", action)
	}
}

// --- Actions ---

func (t *BrowserTool) doNavigate(ctx context.Context, b BrowserBackend, args map[string]interface{}, vision bool) (interface{}, error) {
	url := firstCompatString(args, "url", "href")
	if url == "" {
		return nil, errors.New("url is required for navigate")
	}
	targetID := firstCompatString(args, "target_id", "targetId")
	ctx = WithBrowserRouteHint(ctx, BrowserRouteHint{
		Action:         "navigate",
		FollowupAction: "snapshot_auto",
		Vision:         vision,
		RequiresImage:  vision,
	})
	if gated, handled, err := t.maybeRequireRelayApproval(ctx, b, targetID, url, "use_connected_session"); handled || err != nil {
		return gated, err
	}

	emitBrowserProgress(ctx, "start", "Starting browser", "running", url)
	_ = b.Start(ctx)
	emitBrowserProgress(ctx, "start", "Starting browser", "success", url)

	emitBrowserProgress(ctx, "navigate", "Navigating", "running", url)
	nav, err := b.Navigate(ctx, url, targetID)
	if err != nil {
		emitBrowserProgress(ctx, "navigate", "Navigating", "failed", url)
		return jsonErr(fmt.Sprintf("navigation failed: %s", err)), nil
	}
	emitBrowserProgress(ctx, "navigate", "Navigating", "success", url)

	emitBrowserProgress(ctx, "snapshot", "Reading page", "running", url)
	result, rErr := t.doAutoSnapshot(ctx, b, nav.TargetID, vision)
	if rErr != nil {
		emitBrowserProgress(ctx, "snapshot", "Reading page", "failed", url)
	} else {
		emitBrowserProgress(ctx, "snapshot", "Reading page", "success", url)
	}
	return result, rErr
}

func (t *BrowserTool) doSnapshot(ctx context.Context, b BrowserBackend, targetID string) (interface{}, error) {
	if gated, handled, err := t.maybeRequireRelayApproval(ctx, b, targetID, "", "inspect_connected_session"); handled || err != nil {
		return gated, err
	}
	lang := browserToolLanguage(GetLang(ctx))
	a11y, err := b.AccessibilityTree(ctx, targetID, 10)
	if err != nil {
		return jsonErr(err.Error()), nil
	}
	t.cacheRefs(a11y.TargetID, a11y.RefMap, nil)
	payload := map[string]interface{}{
		"tree":      a11y.Tree,
		"url":       a11y.URL,
		"title":     a11y.Title,
		"target_id": a11y.TargetID,
		"message":   BrowserPageMessage(lang, a11y.Title, a11y.URL, a11y.Tree, -1),
	}
	t.maybeAugmentSnapshotWithReadableContent(ctx, b, payload)
	return jsonResult(payload), nil
}

func (t *BrowserTool) doSnapshotInteractive(ctx context.Context, b BrowserBackend, targetID string) (interface{}, error) {
	if gated, handled, err := t.maybeRequireRelayApproval(ctx, b, targetID, "", "inspect_connected_session"); handled || err != nil {
		return gated, err
	}
	lang := browserToolLanguage(GetLang(ctx))
	result, err := b.InteractiveElements(ctx, targetID)
	if err != nil {
		return jsonErr(err.Error()), nil
	}
	t.cacheRefs(result.TargetID, nil, result.RefMap)
	payload := map[string]interface{}{
		"tree":      result.Tree,
		"url":       result.URL,
		"title":     result.Title,
		"target_id": result.TargetID,
		"count":     result.Count,
		"strategy":  "interactive",
		"message":   BrowserPageMessage(lang, result.Title, result.URL, result.Tree, result.Count),
	}
	t.maybeAugmentSnapshotWithReadableContent(ctx, b, payload)
	return jsonResult(payload), nil
}

const (
	browserInteractiveThreshold  = 30
	browserA11yTreeMaxLen        = 6000
	browserReadableContentMaxLen = 3200
)

func (t *BrowserTool) doAutoSnapshot(ctx context.Context, b BrowserBackend, targetID string, vision bool) (interface{}, error) {
	if gated, handled, err := t.maybeRequireRelayApproval(ctx, b, targetID, "", "inspect_connected_session"); handled || err != nil {
		return gated, err
	}
	lang := browserToolLanguage(GetLang(ctx))
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
				m["note"] = BrowserLargeDOMNoteMessage(lang, count)
				b2, _ := json.Marshal(m)
				return string(b2), nil
			}
		}
		return result, nil
	}

	t.cacheA11y(a11y.TargetID, a11y.RefMap)
	payload := map[string]interface{}{
		"tree":      a11y.Tree,
		"url":       a11y.URL,
		"title":     a11y.Title,
		"target_id": a11y.TargetID,
		"strategy":  "a11y",
		"message":   BrowserPageMessage(lang, a11y.Title, a11y.URL, a11y.Tree, -1),
	}
	t.maybeAugmentSnapshotWithReadableContent(ctx, b, payload)
	return jsonResult(payload), nil
}

func (t *BrowserTool) doScreenshotWithInteractive(ctx context.Context, b BrowserBackend, targetID string) (interface{}, error) {
	lang := browserToolLanguage(GetLang(ctx))
	interactive, err := b.InteractiveElements(ctx, targetID)
	if err != nil {
		data, sErr := b.ScreenshotTab(ctx, targetID)
		if sErr != nil {
			return jsonErr(sErr.Error()), nil
		}
		data = t.normalizeScreenshotPayload(data)
		return jsonResult(map[string]interface{}{
			"screenshot": data,
			"strategy":   "screenshot",
			"message":    BrowserScreenshotInteractiveUnavailableMessage(lang),
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
			"message":   BrowserPageMessage(lang, interactive.Title, interactive.URL, interactive.Tree, interactive.Count),
		}), nil
	}
	data = t.normalizeScreenshotPayload(data)

	payload := map[string]interface{}{
		"screenshot": data,
		"tree":       interactive.Tree,
		"url":        interactive.URL,
		"title":      interactive.Title,
		"target_id":  interactive.TargetID,
		"count":      interactive.Count,
		"strategy":   "screenshot+interactive",
		"message":    BrowserPageWithScreenshotInteractiveMessage(lang, interactive.Title, interactive.URL, interactive.Tree, interactive.Count),
	}
	t.maybeAugmentSnapshotWithReadableContent(ctx, b, payload)
	return jsonResult(payload), nil
}

func (t *BrowserTool) doAct(ctx context.Context, b BrowserBackend, args map[string]interface{}, targetID string) (interface{}, error) {
	rawRef, ok := compatArgValue(args, "ref")
	if !ok {
		return nil, errors.New("ref is required for act (pass 12 or \"@12\" from the accessibility tree)")
	}
	ref, ok := CoerceBrowserRef(rawRef)
	if !ok {
		return nil, errors.New("ref must be an integer or @N string for act")
	}
	actType := firstCompatString(args, "act_type", "actType")
	_, actType = CanonicalizeBrowserAction("act", actType)
	if actType == "" {
		return nil, errors.New("act_type is required for act")
	}
	value := firstCompatString(args, "value", "text")

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
	if gated, handled, err := t.maybeRequireRelayApproval(ctx, b, targetID, "", "use_connected_session"); handled || err != nil {
		return gated, err
	}

	if IsBrowserActionHighRisk("act", actType, "") {
		currentURL := t.resolveCheckpointURL(ctx, b, targetID)
		var screenshot *BrowserCheckpointScreenshot
		if targetID != "" {
			if shot, shotErr := b.ScreenshotTab(ctx, targetID); shotErr == nil && shot != "" {
				screenshot = &BrowserCheckpointScreenshot{
					MimeType: "image/png",
					Data:     shot,
				}
			}
		}
		cpResult, hasRequester, cpErr := RequestBrowserCheckpoint(ctx, BrowserCheckpointRequest{
			Required:   true,
			RiskLevel:  "high",
			Step:       "act",
			Action:     actType,
			URL:        currentURL,
			Screenshot: screenshot,
		})
		if cpErr != nil {
			return jsonErr(fmt.Sprintf("checkpoint failed: %s", cpErr)), nil
		}
		if hasRequester {
			if cpResult.Pending {
				return jsonResult(map[string]interface{}{
					"checkpoint_pending": true,
					"checkpoint_id":      cpResult.CheckpointID,
					"resume_required":    true,
					"message":            cpResult.Message,
				}), nil
			}
			if cpResult.Decision != BrowserCheckpointApprove {
				return jsonErr("browser action denied by user"), nil
			}
		}
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
		"message": BrowserActionPerformedMessage(browserToolLanguage(GetLang(ctx)), actType, ref),
	}), nil
}

func (t *BrowserTool) doPageScroll(ctx context.Context, b BrowserBackend, targetID string, actType string) (interface{}, error) {
	direction, x, y, ok := BrowserLegacyPageScrollDelta("scroll_page", actType)
	if !ok {
		return nil, fmt.Errorf("invalid page scroll action: %s", actType)
	}
	scroller, ok := b.(browserPageScrollCompat)
	if !ok {
		return nil, fmt.Errorf("browser backend does not support page scroll compatibility")
	}
	if gated, handled, err := t.maybeRequireRelayApproval(ctx, b, targetID, "", "use_connected_session"); handled || err != nil {
		return gated, err
	}
	if err := scroller.PageScroll(ctx, targetID, x, y); err != nil {
		return jsonErr(fmt.Sprintf("page scroll %s failed: %s", direction, err)), nil
	}
	return jsonResult(map[string]interface{}{
		"success":   true,
		"target_id": targetID,
		"message":   BrowserPageScrolledMessage(browserToolLanguage(GetLang(ctx)), direction),
	}), nil
}

// CoerceBrowserRef accepts either a raw integer ref or an accessibility-tree ref like "@12".
func CoerceBrowserRef(v interface{}) (int, bool) {
	if raw, ok := v.(string); ok {
		raw = strings.TrimSpace(raw)
		raw = strings.TrimPrefix(raw, "@")
		if raw == "" {
			return 0, false
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			return 0, false
		}
		return n, true
	}
	return coerceCompatInt(v)
}

func (t *BrowserTool) doScreenshot(ctx context.Context, b BrowserBackend, args map[string]interface{}) (interface{}, error) {
	url := firstCompatString(args, "url", "href")
	targetID := firstCompatString(args, "target_id", "targetId")
	lang := browserToolLanguage(GetLang(ctx))
	checkpointURL := strings.TrimSpace(url)
	if checkpointURL == "" {
		checkpointURL = t.resolveCheckpointURL(ctx, b, targetID)
	}
	if gated, handled, err := t.maybeRequireRelayApproval(ctx, b, targetID, checkpointURL, "use_connected_session"); handled || err != nil {
		return gated, err
	}
	progressURL := checkpointURL
	if progressURL == "" {
		progressURL = url
	}
	emitBrowserProgress(ctx, "screenshot", "Capturing screenshot", "running", progressURL)
	_ = b.Start(ctx)
	var (
		data    string
		err     error
		message string
	)
	switch {
	case url != "":
		data, err = b.Screenshot(ctx, url)
		message = BrowserScreenshotMessage(lang, url, "")
	case targetID != "":
		data, err = b.ScreenshotTab(ctx, targetID)
		message = BrowserScreenshotMessage(lang, "", targetID)
	default:
		data, err = b.ScreenshotTab(ctx, "")
		message = BrowserScreenshotMessage(lang, "", "")
	}
	if err != nil {
		emitBrowserProgress(ctx, "screenshot", "Capturing screenshot", "failed", progressURL)
		return jsonErr(err.Error()), nil
	}
	emitBrowserProgress(ctx, "screenshot", "Capturing screenshot", "success", progressURL)
	data = t.normalizeScreenshotPayload(data)
	out := map[string]interface{}{
		"screenshot": data,
		"message":    message,
	}
	if targetID != "" {
		out["target_id"] = targetID
	}
	if url != "" {
		out["url"] = url
	}
	return jsonResult(out), nil
}

func (t *BrowserTool) doTabs(ctx context.Context, b BrowserBackend) (interface{}, error) {
	if gated, handled, err := t.maybeRequireRelayApproval(ctx, b, "", "", "list_connected_tabs"); handled || err != nil {
		return gated, err
	}
	tabs, err := b.Tabs(ctx)
	if err != nil {
		return jsonErr(err.Error()), nil
	}
	return jsonResult(map[string]interface{}{
		"tabs":    tabs,
		"count":   len(tabs),
		"message": BrowserOpenTabsMessage(browserToolLanguage(GetLang(ctx)), len(tabs)),
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
	if gated, handled, err := t.maybeRequireRelayApproval(ctx, b, targetID, "", "use_connected_session"); handled || err != nil {
		return gated, err
	}
	if err := b.CloseTab(ctx, targetID); err != nil {
		return jsonErr(err.Error()), nil
	}
	return jsonResult(map[string]interface{}{
		"closed":  true,
		"message": BrowserTabClosedMessage(browserToolLanguage(GetLang(ctx)), targetID),
	}), nil
}

func (t *BrowserTool) doRecipe(ctx context.Context, b BrowserBackend, args map[string]interface{}) (interface{}, error) {
	recipeName := firstCompatString(args, "recipe", "recipe_name", "recipeName")
	if recipeName == "" {
		return nil, errors.New("recipe is required for action=recipe")
	}

	params := make(map[string]string)
	if raw, ok := compatArgValue(args, "params"); ok {
		switch p := raw.(type) {
		case map[string]interface{}:
			for k, v := range p {
				params[k] = fmt.Sprintf("%v", v)
			}
		case map[string]string:
			for k, v := range p {
				params[k] = v
			}
		}
	}
	recipeTargetID := firstCompatString(args, "target_id", "targetId")
	if gated, handled, err := t.maybeRequireRelayApproval(ctx, b, recipeTargetID, strings.TrimSpace(params["url"]), "use_connected_session"); handled || err != nil {
		return gated, err
	}

	if IsBrowserActionHighRisk("recipe", "", recipeName) {
		cpResult, hasRequester, cpErr := RequestBrowserCheckpoint(ctx, BrowserCheckpointRequest{
			Required:  true,
			RiskLevel: "high",
			Step:      "recipe",
			Action:    recipeName,
			URL:       strings.TrimSpace(params["url"]),
		})
		if cpErr != nil {
			return jsonErr(fmt.Sprintf("checkpoint failed: %s", cpErr)), nil
		}
		if hasRequester {
			if cpResult.Pending {
				return jsonResult(map[string]interface{}{
					"checkpoint_pending": true,
					"checkpoint_id":      cpResult.CheckpointID,
					"resume_required":    true,
					"message":            cpResult.Message,
				}), nil
			}
			if cpResult.Decision != BrowserCheckpointApprove {
				return jsonErr("browser recipe denied by user"), nil
			}
		}
	}

	emitBrowserProgress(ctx, "recipe", "Running "+recipeName, "running", "", map[string]interface{}{"recipe_name": recipeName})
	_ = b.Start(ctx)
	result, err := b.ExecuteRecipe(ctx, recipeName, params)
	if err != nil {
		emitBrowserProgress(ctx, "recipe", "Running "+recipeName, "failed", "", map[string]interface{}{"recipe_name": recipeName})
		return jsonErr(fmt.Sprintf("recipe %s failed: %s", recipeName, err)), nil
	}
	emitBrowserProgress(ctx, "recipe", "Running "+recipeName, "success", "", map[string]interface{}{"recipe_name": recipeName})

	data := map[string]interface{}{
		"success": result.Success,
		"message": result.Message,
	}
	for k, v := range result.Data {
		data[k] = v
	}
	if result.TargetID != "" {
		data["target_id"] = result.TargetID
	}
	return jsonResult(data), nil
}

func (t *BrowserTool) doListRecipes(ctx context.Context, b BrowserBackend) (interface{}, error) {
	infos := b.ListRecipes(ctx)
	return jsonResult(map[string]interface{}{
		"recipes": infos,
		"count":   len(infos),
		"message": BrowserRecipesAvailableMessage(browserToolLanguage(GetLang(ctx)), len(infos)),
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

func (t *BrowserTool) cachedTarget() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return strings.TrimSpace(t.lastTarget)
}

func (t *BrowserTool) resolveCheckpointURL(ctx context.Context, b BrowserBackend, targetID string) string {
	targetID = strings.TrimSpace(targetID)
	tabs, err := b.Tabs(ctx)
	if err != nil {
		return ""
	}
	for _, tab := range tabs {
		tabID := strings.TrimSpace(tab.TargetID)
		tabURL := strings.TrimSpace(tab.URL)
		if targetID != "" && tabID == targetID {
			return tabURL
		}
	}
	for _, tab := range tabs {
		if tab.Active {
			return strings.TrimSpace(tab.URL)
		}
	}
	if len(tabs) == 1 {
		return strings.TrimSpace(tabs[0].URL)
	}
	return ""
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

func (t *BrowserTool) maybeAugmentSnapshotWithReadableContent(ctx context.Context, b BrowserBackend, payload map[string]interface{}) {
	if payload == nil || b == nil {
		return
	}
	lang := browserToolLanguage(GetLang(ctx))
	url := strings.TrimSpace(asString(payload["url"]))
	title := strings.TrimSpace(asString(payload["title"]))
	tree := strings.TrimSpace(asString(payload["tree"]))
	targetID := strings.TrimSpace(asString(payload["target_id"]))
	count, _ := coerceCompatInt(payload["count"])
	if !shouldTryReadableBrowserContent(url, title, tree, count) {
		return
	}

	content, err := extractReadableBrowserContent(ctx, b, targetID, url)
	if err != nil || strings.TrimSpace(content) == "" {
		return
	}

	payload["content"] = content
	payload["content_format"] = "text"
	payload["content_strategy"] = "extract_recipe"
	if tree == "" {
		payload["message"] = BrowserReadableContentMessage(lang, title, url, content)
		return
	}
	payload["message"] = BrowserReadableContentWithTreeMessage(
		lang,
		title,
		url,
		content,
		strings.TrimSpace(asString(payload["strategy"])),
		tree,
	)
}

func shouldTryReadableBrowserContent(url, title, tree string, count int) bool {
	if strings.TrimSpace(url) == "" {
		return false
	}
	if !browserLooksDocumentationPage(url, title) {
		return false
	}
	if strings.TrimSpace(tree) == "" {
		return true
	}
	if count >= browserInteractiveThreshold {
		return true
	}
	if browserTreeLooksNavigationHeavy(tree) {
		return true
	}
	// Documentation pages benefit from direct readable extraction even when the
	// snapshot tree is compact; the extractor is cheap and silently falls back.
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

func extractReadableBrowserContent(ctx context.Context, b BrowserBackend, targetID, url string) (string, error) {
	if extractor, ok := b.(readableContentBrowserBackend); ok && strings.TrimSpace(targetID) != "" {
		if content, err := extractReadableBrowserContentFromTab(ctx, extractor, targetID); err == nil && strings.TrimSpace(content) != "" {
			return content, nil
		}
	}
	return extractReadableBrowserContentViaRecipe(ctx, b, url)
}

func extractReadableBrowserContentFromTab(ctx context.Context, extractor readableContentBrowserBackend, targetID string) (string, error) {
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
		text, err := extractor.ExtractText(ctx, targetID, selector)
		if err != nil {
			continue
		}
		candidate := cleanBrowserReadableContent(text)
		if len([]rune(candidate)) > len([]rune(best)) {
			best = candidate
		}
	}
	heading, _ := extractor.ExtractText(ctx, targetID, "h1")
	heading = cleanBrowserReadableContent(heading)
	if heading != "" && !strings.Contains(strings.ToLower(best), strings.ToLower(heading)) {
		best = strings.TrimSpace(heading + "\n\n" + best)
	}
	if len([]rune(best)) < webFetchMinReadableChars {
		return "", errors.New("readable browser content too short")
	}
	return truncateRunes(best, browserReadableContentMaxLen), nil
}

func extractReadableBrowserContentViaRecipe(ctx context.Context, b BrowserBackend, url string) (string, error) {
	selectors, err := json.Marshal(map[string]string{
		"main_content":      "main, article, [role='main'], .sl-markdown-content, [data-pagefind-body], .docs-content, .documentation, .doc-content, .content",
		"secondary_content": ".markdown-body, .prose, [data-content], .content-area, .docs-page",
		"page_heading":      "h1",
	})
	if err != nil {
		return "", err
	}
	result, err := b.ExecuteRecipe(ctx, "extract", map[string]string{
		"url":       url,
		"selectors": string(selectors),
	})
	if err != nil {
		return "", err
	}
	if !result.Success {
		return "", errors.New(firstNonEmpty(strings.TrimSpace(result.Message), "browser extract recipe failed"))
	}

	extracted, _ := coerceCompatMap(result.Data["extracted"])
	candidates := []string{
		cleanBrowserReadableContent(asString(extracted["main_content"])),
		cleanBrowserReadableContent(asString(extracted["secondary_content"])),
	}
	best := ""
	for _, candidate := range candidates {
		if len([]rune(candidate)) > len([]rune(best)) {
			best = candidate
		}
	}
	heading := cleanBrowserReadableContent(asString(extracted["page_heading"]))
	if heading != "" && !strings.Contains(strings.ToLower(best), strings.ToLower(heading)) {
		best = strings.TrimSpace(heading + "\n\n" + best)
	}
	if len([]rune(best)) < webFetchMinReadableChars {
		return "", errors.New("readable browser content too short")
	}
	return truncateRunes(best, browserReadableContentMaxLen), nil
}

func cleanBrowserReadableContent(raw string) string {
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

// emitBrowserProgress pushes a streaming progress card to the client.
// No-op when no card emitter is set in the context.
func emitBrowserProgress(ctx context.Context, stepID, stepName, status, url string, extras ...map[string]interface{}) {
	card := map[string]interface{}{
		"type":   "browser-progress",
		"step":   stepID,
		"name":   stepName,
		"status": status,
		"url":    url,
	}
	if len(extras) > 0 {
		for key, value := range extras[0] {
			card[key] = value
		}
	}
	EmitCard(ctx, card)
}
