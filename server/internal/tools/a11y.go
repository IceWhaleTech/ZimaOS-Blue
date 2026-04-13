package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	a11yruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

var (
	a11yBackendFactoryMu sync.RWMutex
	a11yBackendFactory   = a11yruntime.DefaultHostBackend
)

type A11yTool struct {
	mu         sync.RWMutex
	backend    a11yruntime.Backend
	browser    *BrowserTool
	lastWindow string
	lastRefMap map[int]string
	mediaDir   string
}

func NewA11yTool() *A11yTool {
	return &A11yTool{}
}

func (t *A11yTool) SetBackend(backend a11yruntime.Backend) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.backend = backend
}

func (t *A11yTool) Backend() a11yruntime.Backend {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.backend
}

func (t *A11yTool) SetMediaDir(dir string) {
	t.mu.Lock()
	t.mediaDir = strings.TrimSpace(dir)
	browser := t.browser
	t.mu.Unlock()
	if browser != nil {
		browser.SetMediaDir(strings.TrimSpace(dir))
	}
}

func (t *A11yTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "a11y",
		Description: "Accessibility-first control for supported host OS windows and browser tabs, including snapshots, focus, actions, scrolling, keyboard input, and screenshots.",
		Icon:        "sparkles",
		SearchHints: []string{
			"desktop app window control",
			"native application automation",
			"focus click type scroll screenshot",
			"menu button dialog host os ui",
			"browser tab accessibility snapshot",
			"web page interactive elements ref click",
		},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "capabilities, windows, focus, snapshot, snapshot_interactive, act, scroll, pointer_move, key, or screenshot",
				},
				"surface": map[string]interface{}{
					"type":        "string",
					"description": "Optional surface selector: host or browser. Defaults to host unless browser-specific identifiers are provided.",
				},
				"window_id": map[string]interface{}{
					"type":        "string",
					"description": "Optional host window ID, or browser target alias when surface=browser.",
				},
				"target_id": map[string]interface{}{
					"type":        "string",
					"description": "Optional browser target/tab ID when surface=browser.",
				},
				"url": map[string]interface{}{
					"type":        "string",
					"description": "Optional browser URL for screenshot/navigation-derived browser actions.",
				},
				"params": map[string]interface{}{
					"type":                 "object",
					"description":          "Action details: ref/act_type/value for act; direction/lines for scroll; x/y for pointer_move; keys for key; hold_ms for long_press.",
					"additionalProperties": true,
				},
			},
			"required": []string{"action"},
		},
	}
}

func (t *A11yTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	t.mu.RLock()
	backend := t.backend
	browser := t.browser
	t.mu.RUnlock()

	action := normalizeA11yAction(firstCompatString(args, "action", "op", "operation", "command"))
	if action == "" {
		return nil, errors.New("action is required")
	}
	if resolveA11ySurface(args, browser != nil) == "browser" {
		return t.doBrowser(ctx, browser, action, args)
	}
	if backend == nil {
		return a11yJSON(map[string]interface{}{
			"error":      "host accessibility backend not available",
			"error_code": "backend_unavailable",
		}), nil
	}
	windowID := firstCompatString(args, "window_id", "windowId", "target_id", "targetId")

	switch action {
	case a11yruntime.ActionCapabilities:
		return t.doCapabilities(ctx, backend)
	case a11yruntime.ActionWindows:
		return t.doWindows(ctx, backend)
	case a11yruntime.ActionFocus:
		return t.doFocus(ctx, backend, windowID)
	case a11yruntime.ActionSnapshot:
		return t.doSnapshot(ctx, backend, windowID, false)
	case a11yruntime.ActionSnapshotInteractive:
		return t.doSnapshot(ctx, backend, windowID, true)
	case a11yruntime.ActionAct:
		return t.doAct(ctx, backend, args, windowID)
	case a11yruntime.ActionScroll:
		return t.doScroll(ctx, backend, args, windowID)
	case a11yruntime.ActionPointerMove:
		return t.doPointerMove(ctx, backend, args)
	case a11yruntime.ActionKey:
		return t.doKey(ctx, backend, args, windowID)
	case a11yruntime.ActionScreenshot:
		return t.doScreenshot(ctx, backend, windowID)
	default:
		return a11yJSON(map[string]interface{}{
			"error":      fmt.Sprintf("invalid action: %s", action),
			"error_code": "unsupported_action",
		}), nil
	}
}

func normalizeA11yAction(action string) string {
	trimmed := strings.TrimSpace(strings.ToLower(action))
	if trimmed == "" {
		return ""
	}
	switch trimmed {
	case a11yruntime.ActionCapabilities,
		a11yruntime.ActionWindows,
		a11yruntime.ActionFocus,
		a11yruntime.ActionSnapshot,
		a11yruntime.ActionSnapshotInteractive,
		a11yruntime.ActionAct,
		a11yruntime.ActionScroll,
		a11yruntime.ActionPointerMove,
		a11yruntime.ActionKey,
		a11yruntime.ActionScreenshot:
		return trimmed
	}
	normalized := strings.NewReplacer(" ", "", "_", "", "-", "").Replace(trimmed)
	switch normalized {
	case "list", "listwindow", "listwindows", "windowlist", "windowslist", "tabs", "listtab", "listtabs":
		return a11yruntime.ActionWindows
	case "snapshotinteractive", "interactivesnapshot":
		return a11yruntime.ActionSnapshotInteractive
	case "pointermove", "movepointer", "mousemove":
		return a11yruntime.ActionPointerMove
	}
	return trimmed
}

func (t *A11yTool) SetBrowser(backend BrowserBackend) {
	t.mu.Lock()
	if t.browser == nil && backend != nil {
		t.browser = NewBrowserTool()
		if strings.TrimSpace(t.mediaDir) != "" {
			t.browser.SetMediaDir(t.mediaDir)
		}
	}
	browser := t.browser
	t.mu.Unlock()
	if browser != nil {
		browser.SetBackend(backend)
	}
}

func (t *A11yTool) BrowserBackend() BrowserBackend {
	t.mu.RLock()
	browser := t.browser
	t.mu.RUnlock()
	if browser == nil {
		return nil
	}
	return browser.Backend()
}

type browserTabFocusCompat interface {
	FocusTab(ctx context.Context, targetID string) error
}

type browserKeyCompat interface {
	PressKeys(ctx context.Context, targetID string, keys []string, holdMS int) error
}

func (t *A11yTool) doBrowser(ctx context.Context, browser *BrowserTool, action string, args map[string]interface{}) (interface{}, error) {
	if browser == nil || browser.Backend() == nil {
		return a11yJSON(map[string]interface{}{
			"error":      "browser accessibility backend not available",
			"error_code": "backend_unavailable",
		}), nil
	}
	switch action {
	case a11yruntime.ActionCapabilities:
		return t.doBrowserCapabilities(browser), nil
	case a11yruntime.ActionWindows:
		return t.doBrowserWindows(ctx, browser)
	case a11yruntime.ActionFocus:
		return t.doBrowserFocus(ctx, browser, args)
	case a11yruntime.ActionSnapshot, a11yruntime.ActionSnapshotInteractive, a11yruntime.ActionAct, a11yruntime.ActionScreenshot:
		return t.doBrowserDelegated(ctx, browser, action, args)
	case a11yruntime.ActionScroll:
		return t.doBrowserScroll(ctx, browser, args)
	case a11yruntime.ActionKey:
		return t.doBrowserKey(ctx, browser, args)
	case a11yruntime.ActionPointerMove:
		return a11yJSON(map[string]interface{}{
			"surface":    "browser",
			"error":      "pointer_move is not supported for browser surface",
			"error_code": "unsupported_action",
		}), nil
	default:
		return a11yJSON(map[string]interface{}{
			"surface":    "browser",
			"error":      fmt.Sprintf("invalid browser action: %s", action),
			"error_code": "unsupported_action",
		}), nil
	}
}

func (t *A11yTool) doBrowserCapabilities(browser *BrowserTool) string {
	backend := browser.Backend()
	supported := []string{"capabilities", "windows", "snapshot", "snapshot_interactive", "act", "scroll", "screenshot"}
	unsupported := []string{"pointer_move"}
	if _, ok := backend.(browserTabFocusCompat); ok {
		supported = append(supported, "focus")
	} else {
		unsupported = append(unsupported, "focus")
	}
	if _, ok := backend.(browserKeyCompat); ok {
		supported = append(supported, "key")
	} else {
		unsupported = append(unsupported, "key")
	}
	return a11yJSON(map[string]interface{}{
		"surface":             "browser",
		"host_os":             "browser",
		"permissions":         []interface{}{},
		"supported_actions":   supported,
		"unsupported_actions": unsupported,
		"message":             "Browser accessibility bridge ready",
	})
}

func (t *A11yTool) doBrowserWindows(ctx context.Context, browser *BrowserTool) (interface{}, error) {
	tabs, err := browser.Backend().Tabs(ctx)
	if err != nil {
		return a11yJSON(map[string]interface{}{
			"surface":    "browser",
			"error":      err.Error(),
			"error_code": "backend_unavailable",
		}), nil
	}
	syncBrowserToolTarget(browser, rememberedBrowserTargetFromTabs(tabs))
	windows := make([]map[string]interface{}, 0, len(tabs))
	for _, tab := range tabs {
		windows = append(windows, map[string]interface{}{
			"id":        tab.TargetID,
			"target_id": tab.TargetID,
			"title":     tab.Title,
			"url":       tab.URL,
			"focused":   tab.Active,
		})
	}
	return a11yJSON(map[string]interface{}{
		"surface": "browser",
		"host_os": "browser",
		"windows": windows,
		"tabs":    tabs,
		"message": fmt.Sprintf("%d browser tabs listed", len(tabs)),
	}), nil
}

func (t *A11yTool) doBrowserFocus(ctx context.Context, browser *BrowserTool, args map[string]interface{}) (interface{}, error) {
	targetID := browserTargetIDFromArgs(args, browser.cachedTarget())
	if targetID == "" {
		return nil, errors.New("target_id is required for browser focus")
	}
	focuser, ok := browser.Backend().(browserTabFocusCompat)
	if !ok {
		return a11yJSON(map[string]interface{}{
			"surface":    "browser",
			"error":      "browser tab focus is not supported by this backend",
			"error_code": "unsupported_action",
		}), nil
	}
	if err := focuser.FocusTab(ctx, targetID); err != nil {
		return a11yJSON(map[string]interface{}{
			"surface":    "browser",
			"error":      err.Error(),
			"error_code": "backend_unavailable",
		}), nil
	}
	syncBrowserToolTarget(browser, targetID)
	return a11yJSON(map[string]interface{}{
		"surface":        "browser",
		"host_os":        "browser",
		"target_id":      targetID,
		"window_id":      targetID,
		"execution_mode": "automation",
		"message":        "Browser tab focused",
	}), nil
}

func (t *A11yTool) doBrowserDelegated(ctx context.Context, browser *BrowserTool, action string, args map[string]interface{}) (interface{}, error) {
	targetID := browserTargetIDFromArgs(args, browser.cachedTarget())
	url := firstCompatString(args, "url", "href")
	browserArgs := map[string]interface{}{
		"action": action,
	}
	if targetID != "" {
		browserArgs["target_id"] = targetID
	}
	if url != "" {
		browserArgs["url"] = url
	}
	if params, ok := compatArgValue(args, "params"); ok {
		browserArgs["params"] = params
	}
	if action == a11yruntime.ActionAct {
		if actType := firstCompatString(args, "act_type", "actType"); actType != "" {
			browserArgs["act_type"] = actType
		}
		if value := firstCompatString(args, "value", "text"); value != "" {
			browserArgs["value"] = value
		}
	}
	raw, err := browser.Execute(ctx, browserArgs)
	if err != nil {
		return nil, err
	}
	return t.wrapBrowserPayload(browser, action, raw), nil
}

func (t *A11yTool) doBrowserScroll(ctx context.Context, browser *BrowserTool, args map[string]interface{}) (interface{}, error) {
	targetID := browserTargetIDFromArgs(args, browser.cachedTarget())
	direction := strings.ToLower(strings.TrimSpace(firstCompatString(args, "direction", "act_type", "actType")))
	if direction == "" {
		direction = "down"
	}
	scroller, ok := browser.Backend().(browserPageScrollCompat)
	if !ok {
		return a11yJSON(map[string]interface{}{
			"surface":    "browser",
			"error":      "browser page scrolling is not supported by this backend",
			"error_code": "unsupported_action",
		}), nil
	}
	_, x, y, ok := BrowserLegacyPageScrollDelta("scroll_page", direction)
	if !ok {
		return a11yJSON(map[string]interface{}{
			"surface":    "browser",
			"error":      fmt.Sprintf("invalid browser scroll direction: %s", direction),
			"error_code": "unsupported_action",
		}), nil
	}
	if err := scroller.PageScroll(ctx, targetID, x, y); err != nil {
		return a11yJSON(map[string]interface{}{
			"surface":    "browser",
			"error":      err.Error(),
			"error_code": "backend_unavailable",
		}), nil
	}
	syncBrowserToolTarget(browser, targetID)
	return a11yJSON(map[string]interface{}{
		"surface":        "browser",
		"host_os":        "browser",
		"target_id":      targetID,
		"window_id":      targetID,
		"execution_mode": "automation",
		"message":        fmt.Sprintf("Scrolled browser page %s", direction),
	}), nil
}

func (t *A11yTool) doBrowserKey(ctx context.Context, browser *BrowserTool, args map[string]interface{}) (interface{}, error) {
	keyer, ok := browser.Backend().(browserKeyCompat)
	if !ok {
		return a11yJSON(map[string]interface{}{
			"surface":    "browser",
			"error":      "browser key injection is not supported by this backend",
			"error_code": "unsupported_action",
		}), nil
	}
	targetID := browserTargetIDFromArgs(args, browser.cachedTarget())
	keys, ok := compatStringSlice(args, "keys", "key")
	if !ok {
		return nil, errors.New("keys are required for browser key")
	}
	holdMS, ok := firstCompatIntDeep(args, "hold_ms", "holdMs")
	if !ok {
		holdMS = a11yruntime.DefaultHoldMS
	}
	holdMS = a11yruntime.NormalizeHoldMS(holdMS)
	if err := keyer.PressKeys(ctx, targetID, keys, holdMS); err != nil {
		return a11yJSON(map[string]interface{}{
			"surface":    "browser",
			"error":      err.Error(),
			"error_code": mapBrowserPayloadErrorCode(err.Error()),
		}), nil
	}
	syncBrowserToolTarget(browser, targetID)
	return a11yJSON(map[string]interface{}{
		"surface":        "browser",
		"host_os":        "browser",
		"target_id":      targetID,
		"window_id":      targetID,
		"execution_mode": "automation",
		"message":        "Browser keys sent",
	}), nil
}

func (t *A11yTool) wrapBrowserPayload(browser *BrowserTool, action string, raw interface{}) string {
	text, ok := raw.(string)
	if !ok {
		return a11yJSON(map[string]interface{}{
			"surface":    "browser",
			"error":      "invalid browser response payload",
			"error_code": "backend_unavailable",
		})
	}
	payload, ok := coerceCompatMap(text)
	if !ok {
		return a11yJSON(map[string]interface{}{
			"surface":    "browser",
			"error":      "invalid browser response payload",
			"error_code": "backend_unavailable",
		})
	}
	payload["surface"] = "browser"
	if targetID := strings.TrimSpace(asString(payload["target_id"])); targetID != "" {
		payload["window_id"] = targetID
	} else if targetID := strings.TrimSpace(browser.cachedTarget()); targetID != "" {
		payload["target_id"] = targetID
		payload["window_id"] = targetID
	}
	if action == a11yruntime.ActionSnapshot || action == a11yruntime.ActionSnapshotInteractive {
		if refMap := browserCachedRefPayload(browser); refMap != nil {
			payload["ref_map"] = refMap
		}
	}
	if action == a11yruntime.ActionScreenshot {
		if screenshot := strings.TrimSpace(asString(payload["screenshot"])); screenshot != "" {
			payload["image_path"] = screenshot
		}
	}
	if _, hasError := payload["error"]; hasError {
		if _, ok := payload["error_code"]; !ok {
			payload["error_code"] = mapBrowserPayloadErrorCode(asString(payload["error"]))
		}
	}
	return a11yJSON(payload)
}

func browserCachedRefPayload(browser *BrowserTool) interface{} {
	browser.mu.RLock()
	defer browser.mu.RUnlock()
	if browser.lastRefMode == "interactive" && len(browser.lastInteractiveRefMap) > 0 {
		out := make(map[int]string, len(browser.lastInteractiveRefMap))
		for key, value := range browser.lastInteractiveRefMap {
			out[key] = value
		}
		return out
	}
	if len(browser.lastRefMap) > 0 {
		out := make(map[int]int, len(browser.lastRefMap))
		for key, value := range browser.lastRefMap {
			out[key] = value
		}
		return out
	}
	return nil
}

func syncBrowserToolTarget(browser *BrowserTool, targetID string) {
	targetID = strings.TrimSpace(targetID)
	if browser == nil || targetID == "" {
		return
	}
	browser.mu.Lock()
	defer browser.mu.Unlock()
	if browser.lastTarget != "" && browser.lastTarget != targetID {
		browser.lastRefMap = nil
		browser.lastInteractiveRefMap = nil
	}
	browser.lastTarget = targetID
}

func rememberedBrowserTargetFromTabs(tabs []BrowserTabResult) string {
	if len(tabs) == 0 {
		return ""
	}
	for _, tab := range tabs {
		if tab.Active && strings.TrimSpace(tab.TargetID) != "" {
			return strings.TrimSpace(tab.TargetID)
		}
	}
	if len(tabs) == 1 {
		return strings.TrimSpace(tabs[0].TargetID)
	}
	return ""
}

func browserTargetIDFromArgs(args map[string]interface{}, fallback string) string {
	targetID := firstCompatString(args, "target_id", "targetId", "tab_id", "tabId", "window_id", "windowId")
	if strings.TrimSpace(targetID) != "" {
		return strings.TrimSpace(targetID)
	}
	return strings.TrimSpace(fallback)
}

func resolveA11ySurface(args map[string]interface{}, hasBrowser bool) string {
	switch strings.ToLower(strings.TrimSpace(firstCompatString(args, "surface", "scope", "context"))) {
	case "browser", "web", "page", "tab":
		return "browser"
	case "host", "desktop", "native":
		return "host"
	}
	if hasBrowser {
		if strings.TrimSpace(firstCompatString(args, "target_id", "targetId", "tab_id", "tabId", "url", "href")) != "" {
			return "browser"
		}
	}
	return "host"
}

func mapBrowserPayloadErrorCode(message string) string {
	lower := strings.ToLower(strings.TrimSpace(message))
	switch {
	case strings.Contains(lower, "no page snapshot loaded"), strings.Contains(lower, "snapshot"):
		return "stale_ref"
	case strings.Contains(lower, "unsupported"), strings.Contains(lower, "invalid action"):
		return "unsupported_action"
	default:
		return "backend_unavailable"
	}
}

func (t *A11yTool) doCapabilities(ctx context.Context, backend a11yruntime.Backend) (interface{}, error) {
	result, err := backend.Capabilities(ctx)
	if err != nil {
		return a11yErrorPayload(err), nil
	}
	return a11yJSON(map[string]interface{}{
		"host_os":             result.HostOS,
		"permissions":         result.Permissions,
		"supported_actions":   result.SupportedActions,
		"unsupported_actions": result.UnsupportedActions,
		"message":             result.Message,
	}), nil
}

func (t *A11yTool) doWindows(ctx context.Context, backend a11yruntime.Backend) (interface{}, error) {
	windows, err := backend.ListWindows(ctx)
	if err != nil {
		return a11yErrorPayload(err), nil
	}
	t.syncWindowContext(rememberedWindowFromList(windows))
	return a11yJSON(map[string]interface{}{
		"host_os": backend.HostOS(),
		"windows": windows,
		"message": "Host windows listed",
	}), nil
}

func (t *A11yTool) doFocus(ctx context.Context, backend a11yruntime.Backend, windowID string) (interface{}, error) {
	result, err := backend.FocusWindow(ctx, windowID)
	if err != nil {
		return a11yErrorPayload(err), nil
	}
	resolvedWindow := strings.TrimSpace(valueOrDefault(result.WindowID, windowID))
	t.clearSnapshotRefs()
	t.syncWindowContext(resolvedWindow)
	return a11yJSON(map[string]interface{}{
		"host_os":        valueOrDefault(result.HostOS, backend.HostOS()),
		"window_id":      resolvedWindow,
		"execution_mode": result.ExecutionMode,
		"message":        valueOrDefault(result.Message, "Window focused"),
	}), nil
}

func (t *A11yTool) doSnapshot(ctx context.Context, backend a11yruntime.Backend, windowID string, interactive bool) (interface{}, error) {
	windowID = t.effectiveWindow(windowID)
	action := a11yruntime.ActionSnapshot
	if interactive {
		action = a11yruntime.ActionSnapshotInteractive
	}
	if degraded, handled, err := t.maybeHandleHostPermissionFallback(ctx, backend, action, windowID); handled || err != nil {
		return degraded, err
	}
	var (
		result a11yruntime.SnapshotResult
		err    error
	)
	if interactive {
		result, err = backend.SnapshotInteractive(ctx, windowID)
	} else {
		result, err = backend.Snapshot(ctx, windowID)
	}
	if err != nil {
		return a11yErrorPayload(err), nil
	}
	t.cacheRefs(valueOrDefault(result.WindowID, windowID), result.RefMap)
	return a11yJSON(map[string]interface{}{
		"host_os":    valueOrDefault(result.HostOS, backend.HostOS()),
		"window_id":  valueOrDefault(result.WindowID, windowID),
		"title":      result.Title,
		"tree":       result.Tree,
		"ref_map":    result.RefMap,
		"image_path": result.ImagePath,
		"message":    valueOrDefault(result.Message, "Host accessibility snapshot ready"),
	}), nil
}

func (t *A11yTool) doAct(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string) (interface{}, error) {
	rawRef, ok := firstCompatValueDeep(args, "ref")
	if !ok {
		return nil, errors.New("ref is required for act")
	}
	ref, ok := coerceA11yRef(rawRef)
	if !ok {
		return nil, errors.New("ref must be an integer or @N string")
	}
	actType := strings.ToLower(strings.TrimSpace(firstCompatString(args, "act_type", "actType")))
	if actType == "" {
		return nil, errors.New("act_type is required for act")
	}
	value := firstCompatString(args, "value", "text")
	holdMS, ok := firstCompatIntDeep(args, "hold_ms", "holdMs")
	if !ok {
		holdMS = a11yruntime.DefaultHoldMS
	}
	holdMS = a11yruntime.NormalizeHoldMS(holdMS)
	if degraded, handled, err := t.maybeHandleHostPermissionFallback(ctx, backend, a11yruntime.ActionAct, strings.TrimSpace(windowID)); handled || err != nil {
		return degraded, err
	}

	t.mu.RLock()
	refMap := cloneA11yRefMap(t.lastRefMap)
	cachedWindow := t.lastWindow
	t.mu.RUnlock()
	if len(refMap) == 0 {
		return a11yJSON(map[string]interface{}{
			"error":      "no host snapshot loaded",
			"error_code": "stale_ref",
		}), nil
	}
	if strings.TrimSpace(windowID) != "" && strings.TrimSpace(cachedWindow) != "" && strings.TrimSpace(windowID) != strings.TrimSpace(cachedWindow) {
		return a11yJSON(map[string]interface{}{
			"error":      fmt.Sprintf("ref @%d belongs to window %q; take a new snapshot for %q first", ref, cachedWindow, windowID),
			"error_code": "stale_ref",
		}), nil
	}
	if windowID == "" {
		windowID = cachedWindow
	}
	if _, ok := refMap[ref]; !ok {
		return a11yJSON(map[string]interface{}{
			"error":      fmt.Sprintf("ref @%d is no longer valid; take a new snapshot first", ref),
			"error_code": "stale_ref",
		}), nil
	}
	if gated, handled, err := t.maybeRequireA11yCheckpoint(ctx, "act", actType); handled || err != nil {
		return gated, err
	}
	result, err := backend.Act(ctx, windowID, ref, refMap, actType, value, holdMS)
	if err != nil {
		return a11yErrorPayload(err), nil
	}
	t.syncWindowContext(valueOrDefault(result.WindowID, windowID))
t.clearSnapshotRefs()
	return a11yJSON(map[string]interface{}{
		"host_os":        valueOrDefault(result.HostOS, backend.HostOS()),
		"window_id":      valueOrDefault(result.WindowID, windowID),
		"execution_mode": result.ExecutionMode,
		"message":        valueOrDefault(result.Message, "Host action completed"),
	}), nil
}

func (t *A11yTool) doScroll(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string) (interface{}, error) {
	windowID = t.effectiveWindow(windowID)
	direction := strings.ToLower(strings.TrimSpace(firstCompatString(args, "direction")))
	if direction == "" {
		direction = "down"
	}
	lines, ok := firstCompatIntDeep(args, "lines")
	if !ok || lines <= 0 {
		lines = 6
	}
	if degraded, handled, err := t.maybeHandleHostPermissionFallback(ctx, backend, a11yruntime.ActionScroll, windowID); handled || err != nil {
		return degraded, err
	}
	result, err := backend.Scroll(ctx, windowID, direction, lines)
	if err != nil {
		return a11yErrorPayload(err), nil
	}
	t.syncWindowContext(valueOrDefault(result.WindowID, windowID))
t.clearSnapshotRefs()
	return a11yJSON(map[string]interface{}{
		"host_os":        valueOrDefault(result.HostOS, backend.HostOS()),
		"window_id":      valueOrDefault(result.WindowID, windowID),
		"execution_mode": result.ExecutionMode,
		"message":        valueOrDefault(result.Message, fmt.Sprintf("Scrolled %s", direction)),
	}), nil
}

func (t *A11yTool) doPointerMove(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}) (interface{}, error) {
	x, ok := firstCompatIntDeep(args, "x")
	if !ok {
		return nil, errors.New("x is required for pointer_move")
	}
	y, ok := firstCompatIntDeep(args, "y")
	if !ok {
		return nil, errors.New("y is required for pointer_move")
	}
	if degraded, handled, err := t.maybeHandleHostPermissionFallback(ctx, backend, a11yruntime.ActionPointerMove, ""); handled || err != nil {
		return degraded, err
	}
	result, err := backend.PointerMove(ctx, x, y)
	if err != nil {
		return a11yErrorPayload(err), nil
	}
t.clearSnapshotRefs()
	return a11yJSON(map[string]interface{}{
		"host_os":        valueOrDefault(result.HostOS, backend.HostOS()),
		"execution_mode": result.ExecutionMode,
		"message":        valueOrDefault(result.Message, "Pointer moved"),
	}), nil
}

func (t *A11yTool) doKey(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string) (interface{}, error) {
	windowID = t.effectiveWindow(windowID)
	keys, ok := compatStringSlice(args, "keys")
	if !ok || len(keys) == 0 {
		return nil, errors.New("keys is required for key")
	}
	holdMS, ok := firstCompatIntDeep(args, "hold_ms", "holdMs")
	if !ok {
		holdMS = a11yruntime.DefaultHoldMS
	}
	holdMS = a11yruntime.NormalizeHoldMS(holdMS)
	if degraded, handled, err := t.maybeHandleHostPermissionFallback(ctx, backend, a11yruntime.ActionKey, windowID); handled || err != nil {
		return degraded, err
	}
	if gated, handled, err := t.maybeRequireA11yCheckpoint(ctx, "key", "key"); handled || err != nil {
		return gated, err
	}
	result, err := backend.Key(ctx, windowID, keys, holdMS)
	if err != nil {
		return a11yErrorPayload(err), nil
	}
	t.syncWindowContext(valueOrDefault(result.WindowID, windowID))
t.clearSnapshotRefs()
	return a11yJSON(map[string]interface{}{
		"host_os":        valueOrDefault(result.HostOS, backend.HostOS()),
		"window_id":      valueOrDefault(result.WindowID, windowID),
		"execution_mode": result.ExecutionMode,
		"message":        valueOrDefault(result.Message, "Keys sent"),
	}), nil
}

func (t *A11yTool) doScreenshot(ctx context.Context, backend a11yruntime.Backend, windowID string) (interface{}, error) {
	windowID = t.effectiveWindow(windowID)
	result, err := backend.Screenshot(ctx, windowID)
	if err != nil {
		return a11yErrorPayload(err), nil
	}
	t.syncWindowContext(valueOrDefault(result.WindowID, windowID))
	return a11yJSON(map[string]interface{}{
		"host_os":    valueOrDefault(result.HostOS, backend.HostOS()),
		"window_id":  valueOrDefault(result.WindowID, windowID),
		"image_path": result.ImagePath,
		"message":    valueOrDefault(result.Message, "Host screenshot captured"),
	}), nil
}

func (t *A11yTool) cacheRefs(windowID string, refMap map[int]string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastWindow = strings.TrimSpace(windowID)
	t.lastRefMap = cloneA11yRefMap(refMap)
}

func (t *A11yTool) clearRefs() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastWindow = ""
	t.lastRefMap = nil
}

func (t *A11yTool) clearSnapshotRefs() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastRefMap = nil
}

func (t *A11yTool) rememberWindow(windowID string) {
	windowID = strings.TrimSpace(windowID)
	if windowID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastWindow = windowID
}

func (t *A11yTool) syncWindowContext(windowID string) {
	windowID = strings.TrimSpace(windowID)
	if windowID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.lastWindow != "" && t.lastWindow != windowID {
		t.lastRefMap = nil
	}
	t.lastWindow = windowID
}

func (t *A11yTool) effectiveWindow(windowID string) string {
	windowID = strings.TrimSpace(windowID)
	if windowID != "" {
		return windowID
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastWindow
}

func rememberedWindowFromList(windows []a11yruntime.WindowInfo) string {
	if len(windows) == 0 {
		return ""
	}
	for _, window := range windows {
		if window.Focused && strings.TrimSpace(window.ID) != "" {
			return strings.TrimSpace(window.ID)
		}
	}
	if len(windows) == 1 {
		return strings.TrimSpace(windows[0].ID)
	}
	return ""
}

func (t *A11yTool) maybeHandleHostPermissionFallback(ctx context.Context, backend a11yruntime.Backend, action string, windowID string) (interface{}, bool, error) {
	if !hostA11yPermissionRequired(action) {
		return nil, false, nil
	}
	caps, err := backend.Capabilities(ctx)
	if err != nil {
		return nil, false, nil
	}
	permission, denied := firstDeniedRequiredPermission(caps.Permissions)
	if !denied {
		return nil, false, nil
	}

	hostOS := valueOrDefault(caps.HostOS, backend.HostOS())
	message := strings.TrimSpace(permission.Message)
	if message == "" {
		message = "Required host accessibility permission is not granted"
	}
	payload := map[string]interface{}{
		"host_os":            hostOS,
		"permissions":        caps.Permissions,
		"permission_warning": true,
		"degraded":           true,
		"fallback_actions": []string{
			a11yruntime.ActionWindows,
			a11yruntime.ActionFocus,
			a11yruntime.ActionScreenshot,
		},
	}
	if trimmedWindow := strings.TrimSpace(windowID); trimmedWindow != "" {
		payload["window_id"] = trimmedWindow
	}

	switch action {
	case a11yruntime.ActionSnapshot, a11yruntime.ActionSnapshotInteractive:
		t.clearSnapshotRefs()
		screenshot, screenshotErr := backend.Screenshot(ctx, windowID)
		if screenshotErr == nil {
			resolvedWindow := valueOrDefault(screenshot.WindowID, windowID)
			t.syncWindowContext(resolvedWindow)
			payload["window_id"] = resolvedWindow
			payload["image_path"] = screenshot.ImagePath
			payload["fallback_action"] = a11yruntime.ActionScreenshot
			payload["message"] = fmt.Sprintf("%s Returning a degraded screenshot fallback until permission is granted.", message)
			return a11yJSON(payload), true, nil
		}
		payload["error"] = message
		payload["error_code"] = "permission_required"
		payload["message"] = fmt.Sprintf("%s Screenshot fallback is also unavailable.", message)
		return a11yJSON(payload), true, nil
	default:
		t.clearSnapshotRefs()
		payload["error"] = message
		payload["error_code"] = "permission_required"
		payload["message"] = fmt.Sprintf("%s Use windows, focus, or screenshot until permission is granted.", message)
		return a11yJSON(payload), true, nil
	}
}

func hostA11yPermissionRequired(action string) bool {
	switch strings.TrimSpace(strings.ToLower(action)) {
	case a11yruntime.ActionSnapshot,
		a11yruntime.ActionSnapshotInteractive,
		a11yruntime.ActionAct,
		a11yruntime.ActionScroll,
		a11yruntime.ActionPointerMove,
		a11yruntime.ActionKey:
		return true
	default:
		return false
	}
}

func firstDeniedRequiredPermission(permissions []a11yruntime.PermissionStatus) (a11yruntime.PermissionStatus, bool) {
	for _, permission := range permissions {
		if permission.Required && !permission.Granted {
			return permission, true
		}
	}
	return a11yruntime.PermissionStatus{}, false
}

func (t *A11yTool) maybeRequireA11yCheckpoint(ctx context.Context, operation string, action string) (interface{}, bool, error) {
	if !IsA11yActionHighRisk(operation, action) {
		return nil, false, nil
	}
	result, hasRequester, err := RequestBrowserCheckpoint(ctx, BrowserCheckpointRequest{
		Required:  true,
		RiskLevel: "high",
		Step:      operation,
		Action:    action,
	})
	if err != nil {
		return a11yJSON(map[string]interface{}{
			"error":      fmt.Sprintf("checkpoint failed: %s", err),
			"error_code": "checkpoint_failed",
		}), true, nil
	}
	if !hasRequester {
		return nil, false, nil
	}
	if result.Pending {
		return a11yJSON(map[string]interface{}{
			"checkpoint_pending": true,
			"checkpoint_id":      result.CheckpointID,
			"resume_required":    true,
			"message":            result.Message,
		}), true, nil
	}
	if result.Decision != BrowserCheckpointApprove {
		return a11yJSON(map[string]interface{}{
			"error":      "host action denied by user",
			"error_code": "checkpoint_denied",
		}), true, nil
	}
	return nil, false, nil
}

func IsA11yActionHighRisk(action string, actType string) bool {
	switch strings.TrimSpace(strings.ToLower(action)) {
	case "act":
		switch strings.TrimSpace(strings.ToLower(actType)) {
		case "click", "double_click", "right_click", "type", "select", "toggle", "expand", "collapse", "submit", "long_press", "key":
			return true
		default:
			return false
		}
	case "key":
		return true
	default:
		return false
	}
}

func RegisterHostA11yTool(registry *Registry, mediaDir string) *A11yTool {
	if registry == nil {
		return nil
	}
	a11yBackendFactoryMu.RLock()
	factory := a11yBackendFactory
	a11yBackendFactoryMu.RUnlock()
	if factory == nil {
		return nil
	}
	backend := factory(mediaDir)
	if backend == nil {
		return nil
	}
	tool := NewA11yTool()
	tool.SetMediaDir(mediaDir)
	tool.SetBackend(backend)
	registry.Register(tool)
	return tool
}

func cloneA11yRefMap(in map[int]string) map[int]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[int]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func compatStringSlice(args map[string]interface{}, keys ...string) ([]string, bool) {
	value, ok := firstCompatValueDeep(args, keys...)
	if !ok {
		return nil, false
	}
	list, ok := coerceCompatStringList(value)
	if !ok || len(list) == 0 {
		return nil, false
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if text := strings.TrimSpace(asString(item)); text != "" {
			out = append(out, text)
		}
	}
	return out, len(out) > 0
}

func a11yErrorPayload(err error) string {
	if err == nil {
		return a11yJSON(map[string]interface{}{})
	}
	if runtimeErr, ok := err.(*a11yruntime.RuntimeError); ok {
		payload := map[string]interface{}{
			"error":      runtimeErr.Message,
			"error_code": runtimeErr.Code,
		}
		for key, value := range runtimeErr.Details {
			payload[key] = value
		}
		return a11yJSON(payload)
	}
	return a11yJSON(map[string]interface{}{
		"error":      err.Error(),
		"error_code": "backend_unavailable",
	})
}

func valueOrDefault(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func setA11yBackendFactoryForTest(factory func(string) a11yruntime.Backend) func() {
	a11yBackendFactoryMu.Lock()
	prev := a11yBackendFactory
	a11yBackendFactory = factory
	a11yBackendFactoryMu.Unlock()
	return func() {
		a11yBackendFactoryMu.Lock()
		a11yBackendFactory = prev
		a11yBackendFactoryMu.Unlock()
	}
}

func mustJSONString(data map[string]interface{}) string {
	raw, _ := json.Marshal(data)
	return string(raw)
}

func a11yJSON(data map[string]interface{}) string {
	raw, _ := json.Marshal(data)
	return string(raw)
}

func coerceA11yRef(v interface{}) (int, bool) {
	if raw, ok := v.(string); ok {
		raw = strings.TrimSpace(strings.TrimPrefix(raw, "@"))
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
