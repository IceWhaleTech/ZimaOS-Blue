package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	a11yruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

var (
	a11yBackendFactoryMu       sync.RWMutex
	a11yBackendFactory         = a11yruntime.DefaultHostBackend
	a11yHostInputActionTimeout = 10 * time.Second
)

type A11yTool struct {
	mu               sync.RWMutex
	backend          a11yruntime.Backend
	browser          *BrowserTool
	lastWindow       string
	lastWindowHint   string
	lastRefMap       map[int]string
	lastRefs         []a11ySnapshotEntry
	mediaDir         string
	clickCache       map[string]a11yConversationClickPoint
	chatGrounder     a11yChatGrounder
	chatMemory       *a11yChatStageMemory
	approvedSessions map[string]struct{}
	taskMemory       map[string]string
	cuaBrain         cuaBrain
	cuaActor         cuaActor
	cuaLLM           LLMBridge
}

func NewA11yTool() *A11yTool {
	return &A11yTool{
		chatMemory:       newA11yChatStageMemory(),
		approvedSessions: make(map[string]struct{}),
		taskMemory:       make(map[string]string),
	}
}

func (t *A11yTool) SetBackend(backend a11yruntime.Backend) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.backend = backend
}

func (t *A11yTool) SetChatGrounder(grounder a11yChatGrounder) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.chatGrounder = grounder
}

func (t *A11yTool) SetLLMBridge(bridge LLMBridge) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cuaLLM = bridge
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
	actionEnum := []string{
		a11yruntime.ActionCapabilities,
		a11yruntime.ActionWindows,
		a11yruntime.ActionFocus,
		a11yruntime.ActionSnapshot,
		a11yruntime.ActionSnapshotInteractive,
		a11yruntime.ActionAct,
		"message",
		"type",
		"select",
		"click",
		"toggle",
		"task",
		"wait",
		"record_info",
		"done",
		"open_app",
		"input_text",
		"Click",
		"RightSingle",
		"move_mouse",
		"scroll_up",
		"scroll_down",
		"Hotkey",
		"multi_Hotkey",
		a11yruntime.ActionScroll,
		a11yruntime.ActionPointerMove,
		a11yruntime.ActionKey,
		a11yruntime.ActionScreenshot,
	}
	nonStrictActionEnum := []string{
		a11yruntime.ActionCapabilities,
		a11yruntime.ActionWindows,
		a11yruntime.ActionFocus,
		a11yruntime.ActionSnapshot,
		a11yruntime.ActionSnapshotInteractive,
		a11yruntime.ActionAct,
		"type",
		"select",
		"click",
		"toggle",
		"task",
		"wait",
		"record_info",
		"done",
		"open_app",
		"input_text",
		"Click",
		"RightSingle",
		"move_mouse",
		"scroll_up",
		"scroll_down",
		"Hotkey",
		"multi_Hotkey",
		a11yruntime.ActionScroll,
		a11yruntime.ActionPointerMove,
		a11yruntime.ActionScreenshot,
	}
	shortcutArraySchema := map[string]interface{}{
		"type":        "array",
		"description": "Shortcut key sequence",
		"items": map[string]interface{}{
			"type": "string",
		},
	}
	shortcutLiteralSchema := map[string]interface{}{
		"type":        "string",
		"description": "Shortcut literal like `enter` or `cmd+k`.",
	}
	topLevelFieldBranch := func(action, field string, fieldSchema map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type": "string",
					"enum": []string{action},
				},
				field: fieldSchema,
			},
			"required":             []string{"action", field},
			"additionalProperties": true,
		}
	}
	paramsFieldBranch := func(action, field string, fieldSchema map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type": "string",
					"enum": []string{action},
				},
				"params": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						field: fieldSchema,
					},
					"required":             []string{field},
					"additionalProperties": true,
				},
			},
			"required":             []string{"action", "params"},
			"additionalProperties": true,
		}
	}
	return ToolDefinition{
		Name:        "computer_use",
		Description: "Desktop/browser computer-use actions. Prefer `task` for multi-step CUA loops; direct actions include `message|type|select|click|toggle`; CUA aliases include `open_app|input_text|Click|RightSingle|move_mouse|scroll_up|scroll_down|Hotkey|multi_Hotkey|record_info|done`.",
		Icon:        "sparkles",
		SearchHints: []string{
			"computer use automation",
		},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        actionEnum,
					"description": "Action. Prefer `message|type|select|click|toggle`; use `key` with `keys`/`submit_keys` for shortcuts. Do not use `open`, `open_location`, `list_apps`, browser `read`, or top-level `press`.",
				},
				"surface": map[string]interface{}{
					"type":        "string",
					"description": "`host|browser`",
				},
				"window_id": map[string]interface{}{
					"type":        "string",
					"description": "Window",
				},
				"window_title": map[string]interface{}{
					"type":        "string",
					"description": "Title",
				},
				"app_name": map[string]interface{}{
					"type":        "string",
					"description": "App",
				},
				"target_id": map[string]interface{}{
					"type":        "string",
					"description": "Target",
				},
				"url": map[string]interface{}{
					"type":        "string",
					"description": "URL",
				},
				"params": map[string]interface{}{
					"type":                 "object",
					"description":          "Args (top-level or params). For `message`, include `value`. For keyboard submit/shortcuts, use `key` with `keys` or `submit_keys`.",
					"additionalProperties": true,
				},
				"ref": map[string]interface{}{
					"type":        "string",
					"description": "Target ref",
				},
				"target_name": map[string]interface{}{
					"type":        "string",
					"description": "Target name",
				},
				"target_role": map[string]interface{}{
					"type":        "string",
					"description": "Target role",
				},
				"act_type": map[string]interface{}{
					"type":        "string",
					"description": "Act type",
				},
				"keys": shortcutArraySchema,
				"value": map[string]interface{}{
					"type":        "string",
					"description": "Text value",
				},
				"intent": map[string]interface{}{
					"type":        "string",
					"description": "Scenario intent",
				},
				"conversation": map[string]interface{}{
					"type":        "string",
					"description": "Conversation name",
				},
				"control": map[string]interface{}{
					"type":        "string",
					"description": "Control name",
				},
				"setting": map[string]interface{}{
					"type":        "string",
					"description": "Setting name",
				},
				"item": map[string]interface{}{
					"type":        "string",
					"description": "Selectable item",
				},
				"option": map[string]interface{}{
					"type":        "string",
					"description": "Selectable option",
				},
				"choice": map[string]interface{}{
					"type":        "string",
					"description": "Selectable choice",
				},
				"submit": map[string]interface{}{
					"type":        "boolean",
					"description": "Submit after type",
				},
				"submit_keys": map[string]interface{}{
					"type":        "array",
					"description": "Submit key sequence",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"submit_target_name": map[string]interface{}{
					"type":        "string",
					"description": "Submit target name",
				},
				"submit_target_role": map[string]interface{}{
					"type":        "string",
					"description": "Submit target role",
				},
				"prefer_visual": map[string]interface{}{
					"type":        "boolean",
					"description": "Prefer visual conversation locate (OCR+click) over keyboard-based search when available.",
				},
				"locate_strategy": map[string]interface{}{
					"type":        "string",
					"description": "Conversation locate strategy override: `visual|visual_first|structured_search|quick_switcher`.",
				},
			},
			"required": []string{"action"},
			"anyOf": []map[string]interface{}{
				topLevelFieldBranch("message", "value", map[string]interface{}{"type": "string"}),
				paramsFieldBranch("message", "value", map[string]interface{}{"type": "string"}),
				topLevelFieldBranch(a11yruntime.ActionKey, "keys", shortcutArraySchema),
				topLevelFieldBranch(a11yruntime.ActionKey, "submit_keys", shortcutArraySchema),
				topLevelFieldBranch(a11yruntime.ActionKey, "value", shortcutLiteralSchema),
				paramsFieldBranch(a11yruntime.ActionKey, "keys", shortcutArraySchema),
				paramsFieldBranch(a11yruntime.ActionKey, "submit_keys", shortcutArraySchema),
				paramsFieldBranch(a11yruntime.ActionKey, "value", shortcutLiteralSchema),
				map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action": map[string]interface{}{
							"type": "string",
							"enum": nonStrictActionEnum,
						},
					},
					"required":             []string{"action"},
					"additionalProperties": true,
				},
			},
		},
	}
}

func (t *A11yTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	t.mu.RLock()
	backend := t.backend
	browser := t.browser
	t.mu.RUnlock()

	action := resolveA11yAction(args)
	if action == "" {
		return nil, errors.New("action is required")
	}
	args = normalizeCUAA11yArgs(action, args)
	if resolveA11ySurface(args, browser != nil) == "browser" {
		return t.doBrowser(ctx, browser, action, args)
	}
	switch action {
	case "record_info":
		return t.doRecordInfo(args), nil
	case "task":
		return t.doTask(ctx, backend, args)
	case "wait":
		return t.doWait(ctx, args)
	case "done":
		return a11yJSON(map[string]interface{}{"status": "completed", "message": valueOrDefault(firstCompatString(args, "text", "message"), "Task completed")}), nil
	}
	if backend == nil {
		return a11yJSON(map[string]interface{}{
			"error":      "host computer-use backend not available",
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
		return t.doFocus(ctx, backend, args, windowID)
	case a11yruntime.ActionSnapshot:
		return t.doSnapshot(ctx, backend, args, windowID, false)
	case a11yruntime.ActionSnapshotInteractive:
		return t.doSnapshot(ctx, backend, args, windowID, true)
	case a11yruntime.ActionAct:
		return t.doAct(ctx, backend, args, windowID)
	case "message", "type", "select", "click", "toggle":
		if action == "click" {
			if _, ok := firstCompatValueDeep(args, "position"); ok {
				return t.doAct(ctx, backend, args, windowID)
			}
		}
		return t.doScenarioAct(ctx, backend, args, windowID, action)
	case a11yruntime.ActionScroll:
		return t.doScroll(ctx, backend, args, windowID)
	case a11yruntime.ActionPointerMove:
		return t.doPointerMove(ctx, backend, args)
	case a11yruntime.ActionKey:
		return t.doKey(ctx, backend, args, windowID)
	case a11yruntime.ActionScreenshot:
		return t.doScreenshot(ctx, backend, args, windowID)
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
		"message",
		"type",
		"select",
		"click",
		"toggle",
		a11yruntime.ActionScroll,
		a11yruntime.ActionPointerMove,
		a11yruntime.ActionKey,
		a11yruntime.ActionScreenshot,
		"task",
		"wait",
		"record_info",
		"done":
		return trimmed
	}
	normalized := strings.NewReplacer(" ", "", "_", "", "-", "").Replace(trimmed)
	switch normalized {
	case "task", "runagenttask", "computerusetask":
		return "task"
	case "done", "finish", "complete":
		return "done"
	case "wait", "pause":
		return "wait"
	case "recordinfo", "remember", "memorize":
		return "record_info"
	case "openapp", "launchapp":
		return a11yruntime.ActionFocus
	case "inputtext", "input_text":
		return "type"
	case "list", "listwindow", "listwindows", "windowlist", "windowslist", "tabs", "listtab", "listtabs":
		return a11yruntime.ActionWindows
	case "focuswindow", "activate", "activatewindow", "activateapp":
		return a11yruntime.ActionFocus
	case "snapshotinteractive", "interactivesnapshot":
		return a11yruntime.ActionSnapshotInteractive
	case "inspectwindowuitree", "accessibilitytree", "uitree":
		return a11yruntime.ActionSnapshotInteractive
	case "keysequence", "keyboardinput", "sendkeys", "sendkey", "submitkeys", "submitkey", "presskey", "presskeys":
		return a11yruntime.ActionKey
	case "message", "chat", "reply", "sendmessage", "sendmsg", "sendchat", "chatmessage":
		return "message"
	case "type", "input", "write", "filltext", "settext", "inputvalue":
		return "type"
	case "select", "choose", "pick", "switchto":
		return "select"
	case "click", "tap", "press", "mouseclick", "leftclick", "leftsingle", "singleclick", "leftmouseclick", "rightsingle", "rightclickpixel":
		return "click"
	case "drag", "dragmouse", "mousedrag":
		return "drag"
	case "toggle", "switch":
		return "toggle"
	case "pointermove", "movepointer", "mousemove", "moveto", "movemouse":
		return a11yruntime.ActionPointerMove
	case "scrollup", "scrolldown":
		return a11yruntime.ActionScroll
	case "hotkey", "multihotkey", "shortcut", "keyboardshortcut":
		return a11yruntime.ActionKey
	}
	if fuzzy := fuzzyNormalizeA11yAction(trimmed); fuzzy != "" {
		return fuzzy
	}
	return trimmed
}

func fuzzyNormalizeA11yAction(action string) string {
	if action == "" {
		return ""
	}
	if fuzzy := fuzzyNormalizeA11yActionChinese(action); fuzzy != "" {
		return fuzzy
	}

	words := a11yActionWordSet(action)
	switch {
	case a11yActionHasAnyWord(words, "screenshot", "screen", "capture", "shot") &&
		a11yActionHasAnyWord(words, "take", "capture", "grab", "screen", "screenshot", "shot"):
		return a11yruntime.ActionScreenshot
	case a11yActionHasAnyWord(words, "interactive", "element", "elements", "control", "controls") &&
		a11yActionHasAnyWord(words, "show", "inspect", "view", "snapshot", "list"):
		return a11yruntime.ActionSnapshotInteractive
	case a11yActionHasAnyWord(words, "accessibility", "ui") &&
		a11yActionHasAnyWord(words, "tree", "elements", "controls", "inspect", "show"):
		return a11yruntime.ActionSnapshotInteractive
	case a11yActionHasAnyWord(words, "window", "windows", "tab", "tabs") &&
		a11yActionHasAnyWord(words, "list", "show", "view", "inspect"):
		return a11yruntime.ActionWindows
	case a11yActionHasAnyWord(words, "focus", "activate", "bring", "raise") &&
		a11yActionHasAnyWord(words, "window", "app", "application"):
		return a11yruntime.ActionFocus
	case a11yActionHasAnyWord(words, "message", "chat", "reply", "dm") &&
		a11yActionHasAnyWord(words, "send", "write", "compose", "reply", "message", "chat", "dm"):
		return "message"
	case a11yActionHasAnyWord(words, "type", "input", "write", "enter", "fill") &&
		a11yActionHasAnyWord(words, "text", "message", "value", "field", "input", "composer", "textarea"):
		return "type"
	case a11yActionHasAnyWord(words, "select", "choose", "pick", "switch", "open") &&
		a11yActionHasAnyWord(words, "conversation", "thread", "chat", "item", "row", "entry", "target"):
		return "select"
	case a11yActionHasAnyWord(words, "toggle", "enable", "disable") &&
		a11yActionHasAnyWord(words, "setting", "settings", "option", "switch", "toggle"):
		return "toggle"
	case a11yActionHasAnyWord(words, "turn") &&
		a11yActionHasAnyWord(words, "on", "off") &&
		a11yActionHasAnyWord(words, "setting", "settings", "option", "switch", "toggle"):
		return "toggle"
	case a11yActionHasAnyWord(words, "click", "tap", "press", "hit") &&
		a11yActionHasAnyWord(words, "button", "link", "control", "item", "option"):
		return "click"
	case a11yActionHasAnyWord(words, "keyboard", "shortcut", "hotkey", "keys", "key", "keystroke"):
		return a11yruntime.ActionKey
	case a11yActionHasAnyWord(words, "scroll", "swipe"):
		return a11yruntime.ActionScroll
	}

	return ""
}

func fuzzyNormalizeA11yActionChinese(action string) string {
	switch {
	case strings.Contains(action, "截图"), strings.Contains(action, "截屏"), strings.Contains(action, "截个图"), strings.Contains(action, "屏幕截图"):
		return a11yruntime.ActionScreenshot
	case strings.Contains(action, "交互控件"), strings.Contains(action, "交互元素"), strings.Contains(action, "可点击元素"),
		strings.Contains(action, "无障碍树"), strings.Contains(action, "界面树"):
		return a11yruntime.ActionSnapshotInteractive
	case strings.Contains(action, "发送消息"), strings.Contains(action, "发消息"), strings.Contains(action, "发送信息"),
		strings.Contains(action, "回复消息"), strings.Contains(action, "回消息"):
		return "message"
	case strings.Contains(action, "输入文字"), strings.Contains(action, "输入文本"), strings.Contains(action, "键入文字"),
		strings.Contains(action, "输入内容"), strings.Contains(action, "填写内容"):
		return "type"
	case strings.Contains(action, "切换会话"), strings.Contains(action, "选择会话"), strings.Contains(action, "切到会话"),
		strings.Contains(action, "切换聊天"), strings.Contains(action, "选择对话"):
		return "select"
	case strings.Contains(action, "打开开关"), strings.Contains(action, "切换开关"), strings.Contains(action, "启用设置"),
		strings.Contains(action, "关闭设置"), strings.Contains(action, "打开设置开关"):
		return "toggle"
	case strings.Contains(action, "点击按钮"), strings.Contains(action, "点击控件"), strings.Contains(action, "单击按钮"),
		strings.Contains(action, "点一下按钮"):
		return "click"
	case strings.Contains(action, "聚焦窗口"), strings.Contains(action, "激活窗口"), strings.Contains(action, "切到窗口"),
		strings.Contains(action, "激活应用"):
		return a11yruntime.ActionFocus
	case strings.Contains(action, "查看窗口列表"), strings.Contains(action, "列出窗口"), strings.Contains(action, "窗口列表"):
		return a11yruntime.ActionWindows
	case strings.Contains(action, "快捷键"), strings.Contains(action, "键盘快捷键"), strings.Contains(action, "按键组合"):
		return a11yruntime.ActionKey
	case strings.Contains(action, "滚动页面"), strings.Contains(action, "向下滚动"), strings.Contains(action, "向上滚动"):
		return a11yruntime.ActionScroll
	}
	return ""
}

func a11yActionWordSet(action string) map[string]struct{} {
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r):
			return r
		default:
			return ' '
		}
	}, action)
	fields := strings.Fields(cleaned)
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		out[field] = struct{}{}
	}
	return out
}

func a11yActionHasAnyWord(words map[string]struct{}, candidates ...string) bool {
	if len(words) == 0 {
		return false
	}
	for _, candidate := range candidates {
		if _, ok := words[candidate]; ok {
			return true
		}
	}
	return false
}

func resolveA11yAction(args map[string]interface{}) string {
	rawAction := firstCompatString(args, "action", "op", "operation", "command", "name", "type")
	action := normalizeA11yAction(rawAction)
	normalizedRaw := strings.NewReplacer(" ", "", "_", "", "-", "").Replace(strings.ToLower(strings.TrimSpace(rawAction)))
	if normalizedRaw == "send" || normalizedRaw == "submit" {
		if _, ok := resolveA11yKeySequenceArgs(args); ok {
			return a11yruntime.ActionKey
		}
		if strings.TrimSpace(firstCompatString(args, "value", "text", "content", "message", "body", "input", "string", "conversation", "thread", "chat", "contact")) != "" {
			return "message"
		}
	}
	if action == a11yruntime.ActionKey {
		return action
	}
	if normalizedRaw == "pressenter" || normalizedRaw == "pressreturn" {
		return a11yruntime.ActionKey
	}
	if !strings.EqualFold(strings.TrimSpace(rawAction), "press") {
		return action
	}
	if action != "click" {
		return action
	}
	if _, ok := resolveA11yKeySequenceArgs(args); ok {
		return a11yruntime.ActionKey
	}
	return action
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

type hostAllWindowsLister interface {
	ListAllWindows(ctx context.Context) ([]a11yruntime.WindowInfo, error)
}

type hostAppActivator interface {
	ActivateApp(ctx context.Context, appName string) (a11yruntime.ActionResult, error)
}

type hostFocusedTextTyper interface {
	TypeFocusedText(ctx context.Context, windowID string, value string, holdMS int) (a11yruntime.ActionResult, error)
}

type hostWindowPixelClicker interface {
	ClickWindowPixel(ctx context.Context, windowID string, x int, y int, holdMS int) (a11yruntime.ActionResult, error)
}

type hostWindowPointDragger interface {
	DragWindowPoint(ctx context.Context, windowID string, start a11yruntime.NormalizedPoint, end a11yruntime.NormalizedPoint, holdMS int) (a11yruntime.ActionResult, error)
}

type a11yActOptions struct {
	allowFocusedTypeAliasFallback          bool
	skipOutcomeVerificationWithoutGrounder bool
}

func (t *A11yTool) doBrowser(ctx context.Context, browser *BrowserTool, action string, args map[string]interface{}) (interface{}, error) {
	if browser == nil || browser.Backend() == nil {
		return a11yJSON(map[string]interface{}{
			"error":      "browser computer-use backend not available",
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
		"message":             "Browser computer-use bridge ready",
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
		if recoveredTarget, ok := recoverBrowserFocusTargetFromTabs(ctx, browser, targetID, err); ok {
			syncBrowserToolTarget(browser, recoveredTarget)
			return a11yJSON(map[string]interface{}{
				"surface":             "browser",
				"host_os":             "browser",
				"target_id":           recoveredTarget,
				"window_id":           recoveredTarget,
				"execution_mode":      "automation",
				"recovered_target_id": recoveredTarget,
				"message":             "Browser tab focus recovered using active tab",
			}), nil
		}
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

func recoverBrowserFocusTargetFromTabs(ctx context.Context, browser *BrowserTool, requestedTargetID string, focusErr error) (string, bool) {
	if browser == nil || focusErr == nil {
		return "", false
	}
	lower := strings.ToLower(strings.TrimSpace(focusErr.Error()))
	if !strings.Contains(lower, "tab not found") {
		return "", false
	}
	tabs, err := browser.Backend().Tabs(ctx)
	if err != nil {
		return "", false
	}
	recoveredTarget := strings.TrimSpace(rememberedBrowserTargetFromTabs(tabs))
	if recoveredTarget == "" || strings.EqualFold(recoveredTarget, strings.TrimSpace(requestedTargetID)) {
		return "", false
	}
	return recoveredTarget, true
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
	keys, ok := resolveA11yBrowserKeySequenceArgs(args)
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

func resolveA11yBrowserKeySequenceArgs(args map[string]interface{}) ([]string, bool) {
	if keys, ok := compatStringSlice(args, "keys", "key"); ok && len(keys) > 0 {
		return keys, true
	}
	if submitKeys, ok := compatStringSlice(args, "submit_keys", "submitKeys"); ok && len(submitKeys) > 0 {
		return submitKeys, true
	}
	if raw := strings.TrimSpace(firstCompatString(args, "value", "text")); raw != "" && strings.Contains(raw, "+") {
		return []string{raw}, true
	}
	return nil, false
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
		"supported_actions":   appendA11yScenarioActions(result.SupportedActions),
		"unsupported_actions": result.UnsupportedActions,
		"message":             result.Message,
	}), nil
}

func normalizeCUAA11yArgs(action string, args map[string]interface{}) map[string]interface{} {
	if len(args) == 0 {
		return args
	}
	out := make(map[string]interface{}, len(args)+8)
	for key, value := range args {
		out[key] = value
	}
	copyAlias := func(canonical string, aliases ...string) {
		if _, ok := firstCompatValueDeep(out, canonical); ok {
			return
		}
		if value, ok := firstCompatValueDeep(args, aliases...); ok {
			out[canonical] = value
		}
	}
	copyStringAlias := func(canonical string, aliases ...string) {
		if strings.TrimSpace(firstCompatString(out, canonical)) != "" {
			return
		}
		if value := firstCompatString(args, aliases...); value != "" {
			out[canonical] = value
		}
	}

	copyStringAlias("window_id", "windowId", "window", "windowID", "target_window", "targetWindow", "win")
	copyStringAlias("app_name", "app", "application", "application_name", "applicationName", "appName", "program", "process", "bundle", "bundle_name", "bundleName")
	copyStringAlias("conversation", "chat", "thread", "contact", "recipient", "receiver", "to", "target_chat", "targetChat", "channel", "group", "room", "group_name", "groupName")
	copyStringAlias("target_name", "targetName", "target", "label", "title")
	copyStringAlias("target_role", "targetRole", "role", "control_role", "controlRole")
	copyStringAlias("goal", "task", "instruction", "objective", "request", "query")
	copyAlias("position", "point", "coordinate", "coordinates", "coord", "coords", "xy", "location", "click_position", "clickPosition", "mouse_position", "mousePosition")
	copyAlias("start", "from", "start_position", "startPosition", "start_point", "startPoint", "source")
	copyAlias("end", "to", "end_position", "endPosition", "end_point", "endPoint", "destination")
	copyAlias("keys", "key", "shortcut", "shortcuts", "hotkey", "hotkeys", "chord", "key_sequence", "keySequence", "keySeq", "keystroke", "keystrokes", "button", "buttons")
	copyAlias("submit_keys", "submitKeys", "send_keys", "sendKeys", "send_key", "sendKey", "submit_key", "submitKey", "send_shortcut", "sendShortcut", "submit_shortcut", "submitShortcut")
	copyStringAlias("direction", "scroll_direction", "scrollDirection", "dir", "scroll", "wheel_direction", "wheelDirection")
	copyAlias("lines", "amount", "distance", "delta", "scroll_lines", "scrollLines", "notches")
	copyAlias("hold_ms", "holdMs", "hold", "duration_ms", "durationMs")
	copyAlias("ms", "delay_ms", "delayMs", "timeout_ms", "timeoutMs", "duration_ms", "durationMs")
	copyAlias("seconds", "secs", "sec", "delay_seconds", "delaySeconds", "duration_seconds", "durationSeconds")
	copyStringAlias("file_name", "fileName", "filename", "file", "memory_file", "memoryFile", "key")

	out["action"] = action
	switch action {
	case "type", "message":
		copyStringAlias("value", "text", "content", "message", "body", "input", "string")
	case "record_info", "done":
		copyStringAlias("text", "value", "content", "message", "body", "input", "string")
	case "click":
		if _, ok := firstCompatValueDeep(out, "position"); ok {
			out["act_type"] = "click"
		}
	case "drag":
		copyAlias("position", "path", "points")
	case a11yruntime.ActionPointerMove:
		if point, ok := coerceA11yNormalizedPoint(firstCompatRawValue(out, "position")); ok {
			out["x"] = int(point.X * 1000)
			out["y"] = int(point.Y * 1000)
		}
	case a11yruntime.ActionScroll:
		rawAction := strings.ToLower(strings.TrimSpace(firstCompatString(args, "action", "op", "name", "type")))
		normalized := strings.NewReplacer("_", "", "-", "", " ", "").Replace(rawAction)
		if firstCompatString(out, "direction") == "" {
			if normalized == "scrollup" {
				out["direction"] = "up"
			} else if normalized == "scrolldown" {
				out["direction"] = "down"
			}
		}
		if _, ok := firstCompatValueDeep(out, "lines"); !ok {
			if dy, ok := firstCompatIntDeep(out, "dy"); ok && dy > 0 {
				out["lines"] = dy
			}
		}
	case a11yruntime.ActionKey:
		if _, ok := resolveA11yKeySequenceArgs(out); !ok {
			if keys := cuaHotkeyArgs(out); len(keys) > 0 {
				out["keys"] = keys
			}
		}
	}
	return out
}

func cuaHotkeyArgs(args map[string]interface{}) []string {
	keys := make([]string, 0, 3)
	for _, key := range []string{"key1", "key2", "key3"} {
		value := firstCompatString(args, key)
		if value == "" {
			continue
		}
		if normalized, _, ok := normalizeA11yShortcutToken(value); ok {
			keys = append(keys, normalized)
		} else {
			keys = append(keys, strings.TrimSpace(value))
		}
	}
	return keys
}

func coerceA11yNormalizedPoint(raw interface{}) (a11yruntime.NormalizedPoint, bool) {
	values, ok := coerceA11yFloatSlice(raw)
	if !ok || len(values) < 2 {
		return a11yruntime.NormalizedPoint{}, false
	}
	x, y := values[0], values[1]
	if x > 1 || y > 1 {
		x = x / 1000
		y = y / 1000
	}
	return a11yruntime.NormalizedPoint{X: x, Y: y}, true
}

func coerceA11yFloatSlice(raw interface{}) ([]float64, bool) {
	switch typed := raw.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil, false
		}
		var decoded []float64
		if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil && len(decoded) > 0 {
			return decoded, true
		}
		parts := strings.FieldsFunc(strings.Trim(trimmed, "[]()"), func(r rune) bool {
			return r == ',' || unicode.IsSpace(r)
		})
		out := make([]float64, 0, len(parts))
		for _, part := range parts {
			if strings.TrimSpace(part) == "" {
				continue
			}
			value, ok := coerceA11yFloat(part)
			if !ok {
				return nil, false
			}
			out = append(out, value)
		}
		return out, len(out) > 0
	case []interface{}:
		out := make([]float64, 0, len(typed))
		for _, item := range typed {
			value, ok := coerceA11yFloat(item)
			if !ok {
				return nil, false
			}
			out = append(out, value)
		}
		return out, len(out) > 0
	case []float64:
		return append([]float64(nil), typed...), len(typed) > 0
	case []int:
		out := make([]float64, 0, len(typed))
		for _, item := range typed {
			out = append(out, float64(item))
		}
		return out, len(out) > 0
	case map[string]interface{}:
		x, xOK := coerceA11yFloat(firstCompatMapValue(typed, "x", "left"))
		y, yOK := coerceA11yFloat(firstCompatMapValue(typed, "y", "top"))
		if xOK && yOK {
			return []float64{x, y}, true
		}
	case map[string]float64:
		x, xOK := typed["x"]
		y, yOK := typed["y"]
		if xOK && yOK {
			return []float64{x, y}, true
		}
	case map[string]int:
		x, xOK := typed["x"]
		y, yOK := typed["y"]
		if xOK && yOK {
			return []float64{float64(x), float64(y)}, true
		}
	}
	return nil, false
}

func firstCompatMapValue(values map[string]interface{}, keys ...string) interface{} {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value
		}
	}
	return nil
}

func coerceA11yFloat(raw interface{}) (float64, bool) {
	switch typed := raw.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		value, err := typed.Float64()
		return value, err == nil
	case string:
		value, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return value, err == nil
	}
	return 0, false
}

func (t *A11yTool) doRecordInfo(args map[string]interface{}) string {
	fileName := strings.TrimSpace(firstCompatString(args, "file_name", "fileName", "name"))
	text := firstCompatString(args, "text", "value", "content")
	if fileName == "" {
		fileName = "memory.txt"
	}
	t.mu.Lock()
	if t.taskMemory == nil {
		t.taskMemory = make(map[string]string)
	}
	t.taskMemory[fileName] = text
	t.mu.Unlock()
	return a11yJSON(map[string]interface{}{
		"status":    "recorded",
		"file_name": fileName,
		"message":   "Computer-use memory recorded",
	})
}

func (t *A11yTool) doTask(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}) (interface{}, error) {
	readFiles := compatStringListFromAny(firstCompatRawValue(args, "read_files", "readFiles"))
	memories := make(map[string]string, len(readFiles))
	t.mu.RLock()
	for _, fileName := range readFiles {
		if text, ok := t.taskMemory[fileName]; ok {
			memories[fileName] = text
		}
	}
	t.mu.RUnlock()
	if len(readFiles) > 0 {
		return a11yJSON(map[string]interface{}{
			"status":   "completed",
			"goal":     firstCompatString(args, "goal", "task"),
			"memories": memories,
			"message":  "Computer-use task memory read",
		}), nil
	}
	if backend == nil {
		return a11yJSON(map[string]interface{}{
			"status":     "blocked",
			"error":      "host computer-use backend not available",
			"error_code": "backend_unavailable",
		}), nil
	}
	return t.doCUATask(ctx, backend, args, memories), nil
}

func (t *A11yTool) doCUATask(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, memories map[string]string) string {
	return t.runNativeCUATask(ctx, backend, args, memories)
}

func compatStringListFromAny(raw interface{}) []string {
	items, ok := coerceCompatStringList(raw)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text := strings.TrimSpace(asString(item)); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func (t *A11yTool) doWait(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	ms, ok := firstCompatIntDeep(args, "ms", "milliseconds")
	if !ok {
		if seconds, secondsOK := firstCompatIntDeep(args, "seconds", "sec"); secondsOK {
			ms = seconds * 1000
		}
	}
	if ms > 0 {
		select {
		case <-ctx.Done():
			return a11yErrorPayload(ctx.Err()), nil
		case <-time.After(time.Duration(ms) * time.Millisecond):
		}
	}
	return a11yJSON(map[string]interface{}{"status": "waited", "message": "Wait completed"}), nil
}

func appendA11yScenarioActions(actions []string) []string {
	if len(actions) == 0 {
		return []string{"message", "type", "select", "click", "toggle", "task", "wait", "record_info", "done"}
	}
	out := append([]string(nil), actions...)
	seen := make(map[string]struct{}, len(out))
	for _, action := range out {
		seen[strings.TrimSpace(strings.ToLower(action))] = struct{}{}
	}
	for _, action := range []string{"message", "type", "select", "click", "toggle", "task", "wait", "record_info", "done"} {
		if _, ok := seen[action]; ok {
			continue
		}
		out = append(out, action)
	}
	return out
}

func (t *A11yTool) doWindows(ctx context.Context, backend a11yruntime.Backend) (interface{}, error) {
	windows, err := backend.ListWindows(ctx)
	if err != nil {
		return a11yErrorPayload(err), nil
	}
	if remembered := rememberedWindowFromList(windows); remembered != "" {
		t.syncWindowContext(remembered)
	}
	return a11yJSON(map[string]interface{}{
		"host_os": backend.HostOS(),
		"windows": windows,
		"message": "Host windows listed",
	}), nil
}

func (t *A11yTool) doFocus(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string) (interface{}, error) {
	resolvedTarget, _, err := t.resolveHostWindowID(ctx, backend, args, windowID)
	if err != nil {
		if activated, handled := t.tryActivateHostAppForFocus(ctx, backend, args, err); handled {
			return activated, nil
		}
		return a11yErrorPayload(err), nil
	}
	result, err := backend.FocusWindow(ctx, resolvedTarget)
	if err != nil {
		return a11yErrorPayload(err), nil
	}
	resolvedWindow := strings.TrimSpace(valueOrDefault(result.WindowID, resolvedTarget))
	t.clearSnapshotRefs()
	t.syncWindowContextWithHint(resolvedWindow, a11yWindowQueryHintFromArgs(args))
	return a11yJSON(map[string]interface{}{
		"host_os":        valueOrDefault(result.HostOS, backend.HostOS()),
		"window_id":      resolvedWindow,
		"execution_mode": result.ExecutionMode,
		"message":        valueOrDefault(result.Message, "Window focused"),
	}), nil
}

func (t *A11yTool) tryActivateHostAppForWindowResolve(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string, resolveErr error) (string, *a11yWindowMatch, a11yruntime.ActionResult, error, bool) {
	runtimeErr, ok := resolveErr.(*a11yruntime.RuntimeError)
	if !ok || runtimeErr.Code != "backend_unavailable" || (runtimeErr.Message != "target window not found" && runtimeErr.Message != "target window is ambiguous") {
		return "", nil, a11yruntime.ActionResult{}, nil, false
	}
	activator, ok := backend.(hostAppActivator)
	if !ok {
		return "", nil, a11yruntime.ActionResult{}, nil, false
	}
	appNames := parseA11yWindowMatchAliases(firstCompatString(args, "app_name", "appName", "application", "app"))
	if len(appNames) == 0 {
		return "", nil, a11yruntime.ActionResult{}, nil, false
	}

	windowResolveRetryable := func(err error) bool {
		if err == nil {
			return false
		}
		runtimeErr, ok := err.(*a11yruntime.RuntimeError)
		if !ok || runtimeErr.Code != "backend_unavailable" {
			return false
		}
		switch runtimeErr.Message {
		case "target window not found", "target window is ambiguous":
			return true
		default:
			return false
		}
	}

	lastErr := resolveErr
	var activationResult a11yruntime.ActionResult
	activated := false
	for _, appName := range appNames {
		result, err := activator.ActivateApp(ctx, appName)
		if err != nil {
			lastErr = err
			continue
		}
		activated = true
		activationResult = result
		// Activation can take a moment to update the "focused window" markers used for disambiguation.
		// Retry resolve a few times before giving up to avoid flakey "target window is ambiguous" errors.
		for attempt := 0; attempt < 3; attempt++ {
			if ctx != nil && ctx.Err() != nil {
				break
			}
			resolvedTarget, match, retryErr := t.resolveHostWindowID(ctx, backend, args, windowID)
			if retryErr == nil && strings.TrimSpace(resolvedTarget) != "" {
				return resolvedTarget, match, activationResult, nil, true
			}
			if retryErr != nil {
				lastErr = retryErr
				if !windowResolveRetryable(retryErr) {
					break
				}
			}
		}
	}
	if !activated {
		return "", nil, a11yruntime.ActionResult{}, lastErr, true
	}
	return "", nil, activationResult, lastErr, true
}

func (t *A11yTool) tryActivateHostAppForFocus(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, resolveErr error) (interface{}, bool) {
	resolvedTarget, _, result, err, handled := t.tryActivateHostAppForWindowResolve(ctx, backend, args, "", resolveErr)
	if !handled {
		return nil, false
	}
	if strings.TrimSpace(resolvedTarget) != "" {
		focusResult, focusErr := backend.FocusWindow(ctx, resolvedTarget)
		if focusErr == nil {
			resolvedWindow := strings.TrimSpace(valueOrDefault(focusResult.WindowID, resolvedTarget))
			t.clearSnapshotRefs()
			t.syncWindowContextWithHint(resolvedWindow, a11yWindowQueryHintFromArgs(args))
			return a11yJSON(map[string]interface{}{
				"host_os":        valueOrDefault(focusResult.HostOS, backend.HostOS()),
				"window_id":      resolvedWindow,
				"execution_mode": focusResult.ExecutionMode,
				"message":        valueOrDefault(focusResult.Message, "Window focused"),
			}), true
		}
		err = focusErr
	}
	if err != nil {
		if strings.TrimSpace(result.WindowID) == "" {
			return a11yErrorPayload(err), true
		}
	}
	resolvedWindow := strings.TrimSpace(result.WindowID)
	t.clearSnapshotRefs()
	t.syncWindowContext(resolvedWindow)
	payload := map[string]interface{}{
		"host_os":        valueOrDefault(result.HostOS, backend.HostOS()),
		"execution_mode": valueOrDefault(result.ExecutionMode, "automation"),
		"message":        valueOrDefault(result.Message, "Application activated"),
	}
	if resolvedWindow != "" {
		payload["window_id"] = resolvedWindow
	}
	return a11yJSON(payload), true
}

func (t *A11yTool) doSnapshot(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string, interactive bool) (interface{}, error) {
	resolvedTarget, _, err := t.resolveHostWindowID(ctx, backend, args, windowID)
	if err != nil {
		if retriedTarget, _, _, retryErr, handled := t.tryActivateHostAppForWindowResolve(ctx, backend, args, windowID, err); handled {
			if retryErr != nil {
				return a11yErrorPayload(retryErr), nil
			}
			resolvedTarget = retriedTarget
		} else {
			return a11yErrorPayload(err), nil
		}
	}
	action := a11yruntime.ActionSnapshot
	if interactive {
		action = a11yruntime.ActionSnapshotInteractive
	}
	if degraded, handled, err := t.maybeHandleHostPermissionFallback(ctx, backend, action, resolvedTarget); handled || err != nil {
		return degraded, err
	}
	var (
		result      a11yruntime.SnapshotResult
		snapshotErr error
	)
	if interactive {
		result, snapshotErr = backend.SnapshotInteractive(ctx, resolvedTarget)
	} else {
		result, snapshotErr = backend.Snapshot(ctx, resolvedTarget)
	}
	if snapshotErr != nil {
		return a11yErrorPayload(snapshotErr), nil
	}
	t.cacheSnapshotContext(valueOrDefault(result.WindowID, resolvedTarget), result.RefMap, result.Tree)
	payload := map[string]interface{}{
		"host_os":    valueOrDefault(result.HostOS, backend.HostOS()),
		"window_id":  valueOrDefault(result.WindowID, resolvedTarget),
		"title":      result.Title,
		"tree":       result.Tree,
		"ref_map":    result.RefMap,
		"image_path": result.ImagePath,
		"message":    valueOrDefault(result.Message, "Host computer-use snapshot ready"),
	}
	applyA11yTelemetryPayload(payload, result.ActionTelemetry)
	return a11yJSON(payload), nil
}

func (t *A11yTool) doScenarioAct(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string, intent string) (interface{}, error) {
	if strings.EqualFold(strings.TrimSpace(intent), "type") {
		return t.doTypeScenarioAct(ctx, backend, args, windowID)
	}
	opts := a11yActOptions{}
	if normalizeA11yActIntent(intent) == "message" {
		opts.allowFocusedTypeAliasFallback = true
	}
	if normalizeA11yActIntent(firstCompatString(args, "intent", "scene", "scenario", "goal")) != "" {
		return t.doActWithOptions(ctx, backend, args, windowID, opts)
	}
	cloned := make(map[string]interface{}, len(args)+1)
	for key, value := range args {
		cloned[key] = value
	}
	cloned["intent"] = intent
	return t.doActWithOptions(ctx, backend, cloned, windowID, opts)
}

func (t *A11yTool) doTypeScenarioAct(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string) (interface{}, error) {
	value := strings.TrimSpace(firstCompatString(args, "value", "text"))
	if value == "" {
		if keys, ok := compatStringSlice(args, "keys"); ok && len(keys) > 0 {
			cloned := make(map[string]interface{}, len(args)+1)
			for key, value := range args {
				cloned[key] = value
			}
			cloned["keys"] = append([]string(nil), keys...)
			return t.doKey(ctx, backend, cloned, windowID)
		}
		if submitKeys, ok := compatStringSlice(args, "submit_keys", "submitKeys"); ok && len(submitKeys) > 0 {
			cloned := make(map[string]interface{}, len(args)+1)
			for key, value := range args {
				cloned[key] = value
			}
			cloned["keys"] = append([]string(nil), submitKeys...)
			return t.doKey(ctx, backend, cloned, windowID)
		}
	}
	if shortcutKeys, ok := parseA11yShortcutLiteral(value); ok && a11yTypeAliasShortcutLiteralEligible(args) {
		cloned := make(map[string]interface{}, len(args)+1)
		for key, value := range args {
			cloned[key] = value
		}
		cloned["keys"] = append([]string(nil), shortcutKeys...)
		return t.doKey(ctx, backend, cloned, windowID)
	}
	cloned := make(map[string]interface{}, len(args)+2)
	for key, value := range args {
		cloned[key] = value
	}
	cloned["act_type"] = "type"
	if _, provided := compatBoolArg(args, "submit"); !provided {
		cloned["submit"] = false
	}
	return t.doActWithOptions(ctx, backend, cloned, windowID, a11yActOptions{
		allowFocusedTypeAliasFallback: true,
	})
}

func (t *A11yTool) doAct(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string) (interface{}, error) {
	return t.doActWithOptions(ctx, backend, args, windowID, a11yActOptions{
		skipOutcomeVerificationWithoutGrounder: true,
	})
}

func (t *A11yTool) doActWithOptions(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string, opts a11yActOptions) (interface{}, error) {
	value := firstCompatString(args, "value", "text")
	intent := inferA11yActIntent(args, firstCompatString(args, "intent", "scene", "scenario", "goal"), value)
	conversationSelector := resolveA11yConversationSelector(args, intent)
	actType := strings.ToLower(strings.TrimSpace(firstCompatString(args, "act_type", "actType")))
	if actType == "" {
		actType = a11yIntentDefaultActType(intent)
	}
	if actType == "" && strings.TrimSpace(value) != "" {
		actType = "type"
	}
	if actType == "" {
		return nil, errors.New("act_type is required for act unless value is provided for text input")
	}
	if intent == "message" && strings.TrimSpace(value) == "" {
		return nil, errors.New("value is required for intent=message")
	}
	holdMS, ok := firstCompatIntDeep(args, "hold_ms", "holdMs")
	if !ok {
		holdMS = a11yruntime.DefaultHoldMS
	}
	holdMS = a11yruntime.NormalizeHoldMS(holdMS)
	chatState, _ := t.maybeStartA11yChatExecution(args, intent, backend.HostOS())
	if chatState != nil {
		ctx = withA11yChatExecutionState(ctx, chatState)
		a11yRecordChatStage(ctx, chatState, a11yChatStageActivateApp, a11yChatStageStatusOK, "reuse_existing_window", "", "", nil)
	}
	chatErrorPayload := func(err error) interface{} {
		annotated := a11yAnnotateChatError(err, chatState)
		payload := a11yErrorPayloadMap(annotated)
		if chatState != nil {
			a11yPersistChatTrajectoryArtifacts(ctx, chatState, payload)
			chatState.applyToPayload(payload)
		}
		return a11yJSON(payload)
	}
	chatConversationSuccessPayload := func(result a11yruntime.ActionResult, window string) interface{} {
		payload := map[string]interface{}{
			"host_os":   valueOrDefault(result.HostOS, backend.HostOS()),
			"window_id": strings.TrimSpace(valueOrDefault(result.WindowID, window)),
			"message":   valueOrDefault(result.Message, "ok"),
		}
		if strings.TrimSpace(result.ExecutionMode) != "" {
			payload["execution_mode"] = result.ExecutionMode
		}
		payload["target_hit"] = true
		applyA11yTelemetryPayload(payload, result.ActionTelemetry)
		if chatState != nil {
			chatState.applyToPayload(payload)
			a11yPersistChatTrajectoryArtifacts(ctx, chatState, payload)
			chatState.applyToPayload(payload)
		}
		return a11yJSON(payload)
	}
	resolvedTarget, match, err := t.resolveHostWindowID(ctx, backend, args, windowID)
	if err != nil {
		if retriedTarget, retriedMatch, _, retryErr, handled := t.tryActivateHostAppForWindowResolve(ctx, backend, args, windowID, err); handled {
			if retryErr != nil {
				return chatErrorPayload(retryErr), nil
			}
			resolvedTarget = retriedTarget
			match = retriedMatch
			if chatState != nil {
				a11yRecordChatStage(ctx, chatState, a11yChatStageActivateApp, a11yChatStageStatusOK, "activate_app_retry", "", "", nil)
			}
		} else {
			return chatErrorPayload(err), nil
		}
	}
	if chatState != nil {
		a11yRecordChatStage(ctx, chatState, a11yChatStageAcquireWindow, a11yChatStageStatusOK, "window_resolve", "", "", nil)
	}
	t.maybeAutoFocusExactMatch(ctx, backend, match, resolvedTarget)
	if degraded, handled, err := t.maybeHandleHostPermissionFallback(ctx, backend, a11yruntime.ActionAct, strings.TrimSpace(resolvedTarget)); handled || err != nil {
		return degraded, err
	}
	if gated, handled, err := t.maybeRequireA11yCheckpoint(ctx, "act", actType); handled || err != nil {
		return gated, err
	}
	if pointResult, handled, pointErr := t.tryWindowPixelClickCompat(ctx, backend, args, resolvedTarget, actType, holdMS); handled {
		if pointErr != nil {
			return chatErrorPayload(pointErr), nil
		}
		resolvedWindow := strings.TrimSpace(valueOrDefault(pointResult.WindowID, resolvedTarget))
		t.syncWindowContextWithHint(resolvedWindow, a11yWindowQueryHintFromArgs(args))
		t.clearSnapshotRefs()
		payload := map[string]interface{}{
			"host_os":        valueOrDefault(pointResult.HostOS, backend.HostOS()),
			"window_id":      resolvedWindow,
			"execution_mode": pointResult.ExecutionMode,
			"message":        valueOrDefault(pointResult.Message, "Host action completed"),
			"target_hit":     pointResult.TargetHit || pointResult.VerificationPassed,
		}
		if pointResult.InputMethod != "" {
			payload["input_method"] = pointResult.InputMethod
		}
		if pointResult.VerificationMethod != "" {
			payload["verification_method"] = pointResult.VerificationMethod
		}
		if a11yShouldExposeVerification(pointResult, actType, intent) {
			payload["verification_passed"] = pointResult.VerificationPassed
		}
		if chatState != nil {
			chatState.applyToPayload(payload)
			a11yPersistChatTrajectoryArtifacts(ctx, chatState, payload)
			chatState.applyToPayload(payload)
		}
		return a11yJSON(payload), nil
	}
	if normalizeA11yActIntent(intent) == "message" {
		resolvedTarget, conversationErr := t.maybeActivateA11yMessageConversation(ctx, backend, args, resolvedTarget, holdMS, intent)
		if conversationErr != nil {
			return chatErrorPayload(conversationErr), nil
		}
		if chatState != nil && chatState.intent == "message" {
			resolvedTarget, conversationErr = t.confirmA11yChatComposerReady(ctx, backend, resolvedTarget)
			if conversationErr != nil {
				return chatErrorPayload(conversationErr), nil
			}
		}
	}

	var (
		ref             int
		refMap          map[int]string
		targetTelemetry a11yruntime.TargetResolution
		selector        a11yTargetSelector
		result          a11yruntime.ActionResult
		usedFocusedType bool
	)
	if rawRef, ok := firstCompatValueDeep(args, "ref"); ok {
		var valid bool
		ref, valid = coerceA11yRef(rawRef)
		if !valid {
			return nil, errors.New("ref must be an integer or @N string")
		}
		t.mu.RLock()
		refMap = cloneA11yRefMap(t.lastRefMap)
		cachedWindow := t.lastWindow
		t.mu.RUnlock()
		if len(refMap) == 0 {
			return a11yJSON(map[string]interface{}{
				"error":      "no host snapshot loaded",
				"error_code": "stale_ref",
			}), nil
		}
		if strings.TrimSpace(resolvedTarget) != "" && strings.TrimSpace(cachedWindow) != "" && strings.TrimSpace(resolvedTarget) != strings.TrimSpace(cachedWindow) {
			return a11yJSON(map[string]interface{}{
				"error":      fmt.Sprintf("ref @%d belongs to window %q; take a new snapshot for %q first", ref, cachedWindow, resolvedTarget),
				"error_code": "stale_ref",
			}), nil
		}
		if resolvedTarget == "" {
			resolvedTarget = cachedWindow
		}
		if _, ok := refMap[ref]; !ok {
			return a11yJSON(map[string]interface{}{
				"error":      fmt.Sprintf("ref @%d is no longer valid; take a new snapshot first", ref),
				"error_code": "stale_ref",
			}), nil
		}
	} else {
		selector = resolveA11yActTargetSelector(args, actType, intent)
		if !selector.Provided() {
			if strings.EqualFold(actType, "type") {
				selector = a11yTargetSelector{Role: "input"}
			} else {
				return nil, errors.New("ref is required for act unless target_name or target_role is provided")
			}
		}
		var resolveErr error
		targetTelemetry, resolveErr = t.resolveActTarget(ctx, backend, resolvedTarget, selector)
		if resolveErr != nil {
			if focusedResult, handled, focusedErr := t.tryFocusedTypeAliasFallback(ctx, backend, args, resolvedTarget, actType, intent, holdMS, selector, resolveErr, opts); handled {
				if focusedErr != nil {
					return chatErrorPayload(focusedErr), nil
				}
				result = focusedResult
				usedFocusedType = true
				if strings.TrimSpace(result.WindowID) != "" {
					resolvedTarget = strings.TrimSpace(result.WindowID)
				}
			}
		}
		if resolveErr != nil && !usedFocusedType {
			if chatState != nil && chatState.intent == "select" && conversationSelector.Provided() {
				activatedWindow, conversationErr, handled := t.tryA11yConversationFallbackChain(ctx, backend, args, resolvedTarget, conversationSelector, holdMS, resolveErr)
				if handled {
					if conversationErr != nil {
						return chatErrorPayload(conversationErr), nil
					}
					return chatConversationSuccessPayload(a11yruntime.ActionResult{HostOS: backend.HostOS()}, activatedWindow), nil
				}
				activatedWindow, conversationErr = t.maybeActivateA11yMessageConversation(ctx, backend, args, resolvedTarget, holdMS, intent)
				if conversationErr != nil {
					return chatErrorPayload(conversationErr), nil
				}
				return chatConversationSuccessPayload(a11yruntime.ActionResult{HostOS: backend.HostOS()}, activatedWindow), nil
			}
			return chatErrorPayload(resolveErr), nil
		}
		if !usedFocusedType && chatState != nil && chatState.intent == "select" && conversationSelector.Provided() {
			a11yRecordChatStage(ctx, chatState, a11yChatStageLocateConversation, a11yChatStageStatusOK, "structured_match", "", "", nil)
			t.rememberA11yChatStrategy(chatState, a11yChatStageLocateConversation, "structured_match")
		}
		if !usedFocusedType {
			ref = targetTelemetry.Ref
			refMap = cloneA11yRefMap(targetTelemetry.RefMap)
			if strings.TrimSpace(targetTelemetry.WindowID) != "" {
				resolvedTarget = strings.TrimSpace(targetTelemetry.WindowID)
			}
		}
	}
	submitPlan, submitPlanErr := t.resolveActSubmitPlan(ctx, backend, args, resolvedTarget, actType, ref, intent)
	if submitPlanErr != nil {
		return chatErrorPayload(submitPlanErr), nil
	}
	if chatState != nil && chatState.intent == "message" {
		typeStrategy := "type"
		if submitPlan.Enabled {
			typeStrategy = "type_and_send"
		}
		a11yRecordChatStage(ctx, chatState, a11yChatStageTypeOrSend, a11yChatStageStatusOK, typeStrategy, "", "", nil)
	}
	if !usedFocusedType {
		actionErr := error(nil)
		result, actionErr = a11yRunActionResultWithTimeout(ctx, actType, resolvedTarget, func(actionCtx context.Context) (a11yruntime.ActionResult, error) {
			return backend.Act(actionCtx, resolvedTarget, ref, refMap, actType, value, holdMS)
		})
		if actionErr != nil {
			if fallbackActType, ok := a11yFallbackActTypeOnUnsupported(actType, intent, actionErr); ok {
				result, actionErr = a11yRunActionResultWithTimeout(ctx, fallbackActType, resolvedTarget, func(actionCtx context.Context) (a11yruntime.ActionResult, error) {
					return backend.Act(actionCtx, resolvedTarget, ref, refMap, fallbackActType, value, holdMS)
				})
			}
		}
		if actionErr != nil {
			return chatErrorPayload(actionErr), nil
		}
	}
	if runtime, ok := backend.(a11yruntime.SnapshotRuntime); ok {
		token := ""
		if !usedFocusedType && len(refMap) > 0 {
			token = refMap[ref]
		}
		runtime.UpdateSnapshotAfterAction(valueOrDefault(result.WindowID, resolvedTarget), token, actType, value)
	}
	if !usedFocusedType && chatState != nil && chatState.intent == "select" && conversationSelector.Provided() {
		confirmedWindow, confirmErr := t.confirmA11yMessageConversationActivated(ctx, backend, valueOrDefault(result.WindowID, resolvedTarget), conversationSelector)
		if confirmErr != nil {
			return chatErrorPayload(confirmErr), nil
		}
		if strings.TrimSpace(confirmedWindow) != "" {
			result.WindowID = confirmedWindow
		}
	}
	if submitPlan.Enabled {
		submitConfirmation := buildA11ySubmitConfirmation(value, ref, refMap)
		submitResult, submitErr := t.executeActSubmitPlanWithConfirmation(ctx, backend, resolvedTarget, holdMS, submitPlan, submitConfirmation)
		if submitErr != nil {
			return chatErrorPayload(submitErr), nil
		}
		result = mergeA11yActionResults(result, submitResult)
	}
	if chatState != nil && chatState.intent == "message" {
		if submitPlan.Enabled {
			if verification := chatState.submitEvidenceSnapshot(); len(verification) > 0 {
				if _, ok := verification["status"]; !ok {
					verification["status"] = "sent"
				}
				a11yRecordChatStage(ctx, chatState, a11yChatStageTypeOrSend, a11yChatStageStatusOK, "submit_phase_confirmation", "", "", verification)
			} else if t.a11yChatGrounder() == nil && (opts.skipOutcomeVerificationWithoutGrounder || strings.TrimSpace(chatState.conversation) != "") {
			} else if verifyErr := t.verifyA11yChatOutcome(ctx, backend, valueOrDefault(result.WindowID, resolvedTarget), true, value); verifyErr != nil {
				return chatErrorPayload(verifyErr), nil
			}
		} else {
			a11yRecordChatStage(ctx, chatState, a11yChatStageTypeOrSend, a11yChatStageStatusOK, "draft_confirmation", "", "", map[string]interface{}{
				"status":            "drafted",
				"confirmation":      "draft_confirmation",
				"grounding_skipped": true,
			})
		}
	}
	t.syncWindowContextWithHint(valueOrDefault(result.WindowID, resolvedTarget), a11yWindowQueryHintFromArgs(args))
	t.clearSnapshotRefs()
	telemetry := mergeA11yTargetAndActionTelemetry(targetTelemetry, result.ActionTelemetry)
	payload := map[string]interface{}{
		"host_os":        valueOrDefault(result.HostOS, backend.HostOS()),
		"window_id":      valueOrDefault(result.WindowID, resolvedTarget),
		"execution_mode": result.ExecutionMode,
		"message":        valueOrDefault(result.Message, "Host action completed"),
	}
	effectiveIntent := valueOrDefault(result.Intent, intent)
	if strings.TrimSpace(effectiveIntent) != "" {
		payload["intent"] = effectiveIntent
	}
	targetHit := result.TargetHit || result.VerificationPassed
	if !targetHit && !a11yActionResultHasTelemetry(result) {
		targetHit = true
	}
	payload["target_hit"] = targetHit
	if result.InputMethod != "" {
		payload["input_method"] = result.InputMethod
	}
	if result.VerificationMethod != "" {
		payload["verification_method"] = result.VerificationMethod
	}
	if a11yShouldExposeVerification(result, actType, effectiveIntent) {
		payload["verification_passed"] = result.VerificationPassed
	}
	if fallbacks := mergeA11yFallbacks(result.Fallbacks, telemetry.Fallbacks); len(fallbacks) > 0 {
		payload["fallbacks"] = fallbacks
	}
	if result.OverlayMode != "" {
		payload["overlay_mode"] = result.OverlayMode
	}
	applyA11yTelemetryPayload(payload, telemetry)
	if chatState != nil {
		chatState.applyToPayload(payload)
		a11yPersistChatTrajectoryArtifacts(ctx, chatState, payload)
		chatState.applyToPayload(payload)
	}
	return a11yJSON(payload), nil
}

func (t *A11yTool) tryWindowPixelClickCompat(
	ctx context.Context,
	backend a11yruntime.Backend,
	args map[string]interface{},
	windowID string,
	actType string,
	holdMS int,
) (a11yruntime.ActionResult, bool, error) {
	if strings.TrimSpace(strings.ToLower(actType)) != "click" {
		return a11yruntime.ActionResult{}, false, nil
	}
	if strings.TrimSpace(windowID) == "" {
		return a11yruntime.ActionResult{}, false, nil
	}
	if _, ok := firstCompatValueDeep(args, "ref"); ok {
		return a11yruntime.ActionResult{}, false, nil
	}
	if firstCompatString(args, "target_name", "targetName", "target_role", "targetRole") != "" {
		return a11yruntime.ActionResult{}, false, nil
	}
	if point, ok := coerceA11yNormalizedPoint(firstCompatRawValue(args, "position")); ok {
		result, err := backend.ClickWindowPoint(ctx, windowID, point, holdMS)
		if err != nil {
			return a11yruntime.ActionResult{}, true, err
		}
		if strings.TrimSpace(result.HostOS) == "" {
			result.HostOS = backend.HostOS()
		}
		if strings.TrimSpace(result.WindowID) == "" {
			result.WindowID = strings.TrimSpace(windowID)
		}
		return result, true, nil
	}
	x, ok := firstCompatIntDeep(args, "x")
	if !ok {
		return a11yruntime.ActionResult{}, false, nil
	}
	y, ok := firstCompatIntDeep(args, "y")
	if !ok {
		return a11yruntime.ActionResult{}, false, nil
	}
	clicker, ok := backend.(hostWindowPixelClicker)
	if !ok {
		return a11yruntime.ActionResult{}, false, nil
	}
	result, err := a11yRunActionResultWithTimeout(ctx, "pixel_click", windowID, func(actionCtx context.Context) (a11yruntime.ActionResult, error) {
		return clicker.ClickWindowPixel(actionCtx, windowID, x, y, holdMS)
	})
	if err != nil {
		return a11yruntime.ActionResult{}, true, err
	}
	if strings.TrimSpace(result.HostOS) == "" {
		result.HostOS = backend.HostOS()
	}
	if strings.TrimSpace(result.WindowID) == "" {
		result.WindowID = strings.TrimSpace(windowID)
	}
	return result, true, nil
}

func (t *A11yTool) tryFocusedTypeAliasFallback(
	ctx context.Context,
	backend a11yruntime.Backend,
	args map[string]interface{},
	windowID string,
	actType string,
	intent string,
	holdMS int,
	selector a11yTargetSelector,
	resolveErr error,
	opts a11yActOptions,
) (a11yruntime.ActionResult, bool, error) {
	if !opts.allowFocusedTypeAliasFallback {
		return a11yruntime.ActionResult{}, false, nil
	}
	if strings.TrimSpace(strings.ToLower(backend.HostOS())) != "darwin" {
		return a11yruntime.ActionResult{}, false, nil
	}
	normalizedIntent := normalizeA11yActIntent(intent)
	if normalizedIntent != "" {
		state := getA11yChatExecutionState(ctx)
		if normalizedIntent != "message" || state == nil || state.intent != "message" {
			return a11yruntime.ActionResult{}, false, nil
		}
	}
	if strings.TrimSpace(strings.ToLower(actType)) != "type" {
		return a11yruntime.ActionResult{}, false, nil
	}
	explicitSelector := a11yTargetSelector{
		Name: firstCompatString(args, "target_name", "targetName"),
		Role: firstCompatString(args, "target_role", "targetRole"),
	}
	if explicitSelector.Provided() {
		if strings.TrimSpace(explicitSelector.Name) != "" {
			return a11yruntime.ActionResult{}, false, nil
		}
		switch normalizeA11yTargetRole(explicitSelector.Role) {
		case "input", "text_input", "text", "editor":
		default:
			return a11yruntime.ActionResult{}, false, nil
		}
	}
	runtimeErr, ok := resolveErr.(*a11yruntime.RuntimeError)
	if !ok {
		return a11yruntime.ActionResult{}, false, nil
	}
	if runtimeErr.Code != "target_not_found" {
		// When the structured snapshot is unavailable (e.g. AX window lookup fails),
		// fall back to focused text input if the scenario is a chat "message".
		if !a11yIsAXWindowLookupFailure(resolveErr) {
			return a11yruntime.ActionResult{}, false, nil
		}
	}
	if runtimeErr.Code == "target_not_found" {
		// ok
	} else if runtimeErr.Code == "backend_unavailable" {
		// ok (AX window lookup failed)
	} else {
		return a11yruntime.ActionResult{}, false, nil
	}
	switch normalizeA11yTargetRole(selector.Role) {
	case "input", "text_input", "text", "editor":
	default:
		return a11yruntime.ActionResult{}, false, nil
	}
	typer, ok := backend.(hostFocusedTextTyper)
	if !ok {
		return a11yruntime.ActionResult{}, false, nil
	}
	value := firstCompatString(args, "value", "text")
	// Best-effort focus: click the lower portion of the window where chat composers
	// typically live before pasting text into the focused control.
	if strings.TrimSpace(windowID) != "" {
		_, _ = a11yRunActionResultWithTimeout(ctx, "point_click", windowID, func(actionCtx context.Context) (a11yruntime.ActionResult, error) {
			return backend.ClickWindowPoint(actionCtx, windowID, a11yruntime.NormalizedPoint{X: 0.4, Y: 0.93}, holdMS)
		})
	}
	if normalizedIntent == "message" && strings.TrimSpace(windowID) != "" {
		_, _ = a11yRunActionResultWithTimeout(ctx, "key", windowID, func(actionCtx context.Context) (a11yruntime.ActionResult, error) {
			return backend.Key(actionCtx, windowID, []string{"command", "a"}, holdMS)
		})
	}
	result, err := a11yRunActionResultWithTimeout(ctx, actType, windowID, func(actionCtx context.Context) (a11yruntime.ActionResult, error) {
		return typer.TypeFocusedText(actionCtx, windowID, value, holdMS)
	})
	if err != nil {
		return a11yruntime.ActionResult{}, true, err
	}
	if strings.TrimSpace(result.HostOS) == "" {
		result.HostOS = backend.HostOS()
	}
	if strings.TrimSpace(result.WindowID) == "" {
		result.WindowID = windowID
	}
	if strings.TrimSpace(result.ExecutionMode) == "" {
		result.ExecutionMode = "input"
	}
	if strings.TrimSpace(result.VerificationMethod) == "" {
		result.VerificationMethod = "focused_text"
	}
	if strings.TrimSpace(result.Message) == "" {
		result.Message = "Host action completed"
	}
	result.TargetHit = true
	result.VerificationPassed = true
	result.Fallbacks = mergeA11yFallbacks(result.Fallbacks, []string{"focused_text"})
	return result, true, nil
}

func a11yActionResultHasTelemetry(result a11yruntime.ActionResult) bool {
	return strings.TrimSpace(result.Intent) != "" ||
		strings.TrimSpace(result.VerificationMethod) != "" ||
		strings.TrimSpace(result.InputMethod) != "" ||
		len(result.Fallbacks) > 0 ||
		strings.TrimSpace(result.OverlayMode) != "" ||
		a11yTelemetryHasData(result.ActionTelemetry)
}

func a11yShouldExposeVerification(result a11yruntime.ActionResult, actType string, intent string) bool {
	if result.VerificationPassed {
		return true
	}
	if strings.TrimSpace(result.VerificationMethod) != "" {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(actType), "type") {
		return true
	}
	switch normalizeA11yActIntent(intent) {
	case "message", "type":
		return true
	default:
		return false
	}
}

func mergeA11yTargetAndActionTelemetry(target a11yruntime.TargetResolution, action a11yruntime.ActionTelemetry) a11yruntime.ActionTelemetry {
	merged := action
	if merged.SnapshotRevision == 0 {
		merged.SnapshotRevision = target.SnapshotRevision
	}
	if !merged.CacheHit {
		merged.CacheHit = target.CacheHit
	}
	if merged.NodeCount == 0 {
		merged.NodeCount = target.NodeCount
	}
	if merged.CandidateCount == 0 {
		merged.CandidateCount = target.CandidateCount
	}
	if merged.QueryMS == 0 {
		merged.QueryMS = target.QueryMS
	}
	if len(merged.Fallbacks) == 0 && len(target.Fallbacks) > 0 {
		merged.Fallbacks = append([]string(nil), target.Fallbacks...)
	}
	return merged
}

func applyA11yTelemetryPayload(payload map[string]interface{}, telemetry a11yruntime.ActionTelemetry) {
	if payload == nil || !a11yTelemetryHasData(telemetry) {
		return
	}
	if telemetry.SnapshotRevision > 0 {
		payload["snapshot_revision"] = telemetry.SnapshotRevision
	}
	if telemetry.CacheHit {
		payload["cache_hit"] = true
	}
	if telemetry.NodeCount > 0 {
		payload["node_count"] = telemetry.NodeCount
	}
	if telemetry.CandidateCount > 0 {
		payload["candidate_count"] = telemetry.CandidateCount
	}
	if telemetry.TreeFetchMS > 0 {
		payload["tree_fetch_ms"] = telemetry.TreeFetchMS
	}
	if telemetry.TreeSerializeMS > 0 {
		payload["tree_serialize_ms"] = telemetry.TreeSerializeMS
	}
	if telemetry.QueryMS > 0 {
		payload["query_ms"] = telemetry.QueryMS
	}
	if telemetry.ActionMS > 0 {
		payload["action_ms"] = telemetry.ActionMS
	}
	if telemetry.VerificationMS > 0 {
		payload["verification_ms"] = telemetry.VerificationMS
	}
	if telemetry.EndToEndMS > 0 {
		payload["end_to_end_ms"] = telemetry.EndToEndMS
	}
	if len(telemetry.Fallbacks) > 0 {
		if existing, ok := payload["fallbacks"].([]string); ok {
			payload["fallbacks"] = mergeA11yFallbacks(existing, telemetry.Fallbacks)
			return
		}
		if existing, ok := payload["fallbacks"].([]interface{}); ok {
			values := make([]string, 0, len(existing))
			for _, item := range existing {
				if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
					values = append(values, text)
				}
			}
			payload["fallbacks"] = mergeA11yFallbacks(values, telemetry.Fallbacks)
			return
		}
		payload["fallbacks"] = append([]string(nil), telemetry.Fallbacks...)
	}
}

func a11yTelemetryHasData(telemetry a11yruntime.ActionTelemetry) bool {
	return telemetry.SnapshotRevision > 0 ||
		telemetry.CacheHit ||
		telemetry.NodeCount > 0 ||
		telemetry.CandidateCount > 0 ||
		telemetry.TreeFetchMS > 0 ||
		telemetry.TreeSerializeMS > 0 ||
		telemetry.QueryMS > 0 ||
		telemetry.ActionMS > 0 ||
		telemetry.VerificationMS > 0 ||
		telemetry.EndToEndMS > 0 ||
		len(telemetry.Fallbacks) > 0
}

func mergeA11yFallbacks(groups ...[]string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, 4)
	for _, group := range groups {
		for _, item := range group {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (t *A11yTool) doScroll(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string) (interface{}, error) {
	resolvedTarget, match, err := t.resolveHostWindowID(ctx, backend, args, windowID)
	if err != nil {
		if retriedTarget, retriedMatch, _, retryErr, handled := t.tryActivateHostAppForWindowResolve(ctx, backend, args, windowID, err); handled {
			if retryErr != nil {
				return a11yErrorPayload(retryErr), nil
			}
			resolvedTarget = retriedTarget
			match = retriedMatch
		} else {
			return a11yErrorPayload(err), nil
		}
	}
	t.maybeAutoFocusExactMatch(ctx, backend, match, resolvedTarget)
	direction := strings.ToLower(strings.TrimSpace(firstCompatString(args, "direction")))
	if direction == "" {
		direction = "down"
	}
	lines, ok := firstCompatIntDeep(args, "lines")
	if !ok || lines <= 0 {
		lines = 6
	}
	if degraded, handled, err := t.maybeHandleHostPermissionFallback(ctx, backend, a11yruntime.ActionScroll, resolvedTarget); handled || err != nil {
		return degraded, err
	}
	result, err := backend.Scroll(ctx, resolvedTarget, direction, lines)
	if err != nil {
		return a11yErrorPayload(err), nil
	}
	t.syncWindowContextWithHint(valueOrDefault(result.WindowID, resolvedTarget), a11yWindowQueryHintFromArgs(args))
	t.clearSnapshotRefs()
	return a11yJSON(map[string]interface{}{
		"host_os":        valueOrDefault(result.HostOS, backend.HostOS()),
		"window_id":      valueOrDefault(result.WindowID, resolvedTarget),
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
	resolvedTarget, match, err := t.resolveHostWindowID(ctx, backend, args, windowID)
	if err != nil {
		if retriedTarget, retriedMatch, _, retryErr, handled := t.tryActivateHostAppForWindowResolve(ctx, backend, args, windowID, err); handled {
			if retryErr != nil {
				return a11yErrorPayload(retryErr), nil
			}
			resolvedTarget = retriedTarget
			match = retriedMatch
		} else {
			return a11yErrorPayload(err), nil
		}
	}
	t.maybeAutoFocusExactMatch(ctx, backend, match, resolvedTarget)
	keys, ok := resolveA11yKeySequenceArgs(args)
	if !ok || len(keys) == 0 {
		return nil, errors.New("keys is required for key")
	}
	holdMS, ok := firstCompatIntDeep(args, "hold_ms", "holdMs")
	if !ok {
		holdMS = a11yruntime.DefaultHoldMS
	}
	holdMS = a11yruntime.NormalizeHoldMS(holdMS)
	if degraded, handled, err := t.maybeHandleHostPermissionFallback(ctx, backend, a11yruntime.ActionKey, resolvedTarget); handled || err != nil {
		return degraded, err
	}
	if gated, handled, err := t.maybeRequireA11yCheckpoint(ctx, "key", "key"); handled || err != nil {
		return gated, err
	}
	result, err := a11yRunActionResultWithTimeout(ctx, "key", resolvedTarget, func(actionCtx context.Context) (a11yruntime.ActionResult, error) {
		return backend.Key(actionCtx, resolvedTarget, keys, holdMS)
	})
	if err != nil {
		return a11yErrorPayload(err), nil
	}
	t.syncWindowContextWithHint(valueOrDefault(result.WindowID, resolvedTarget), a11yWindowQueryHintFromArgs(args))
	t.clearSnapshotRefs()
	return a11yJSON(map[string]interface{}{
		"host_os":        valueOrDefault(result.HostOS, backend.HostOS()),
		"window_id":      valueOrDefault(result.WindowID, resolvedTarget),
		"execution_mode": result.ExecutionMode,
		"message":        valueOrDefault(result.Message, "Keys sent"),
	}), nil
}

func (t *A11yTool) doScreenshot(ctx context.Context, backend a11yruntime.Backend, args map[string]interface{}, windowID string) (interface{}, error) {
	resolvedTarget, _, err := t.resolveHostWindowID(ctx, backend, args, windowID)
	if err != nil {
		if retriedTarget, _, _, retryErr, handled := t.tryActivateHostAppForWindowResolve(ctx, backend, args, windowID, err); handled {
			if retryErr != nil {
				return a11yErrorPayload(retryErr), nil
			}
			resolvedTarget = retriedTarget
		} else {
			return a11yErrorPayload(err), nil
		}
	}
	result, err := backend.Screenshot(ctx, resolvedTarget)
	if err != nil {
		return a11yErrorPayload(err), nil
	}
	t.syncWindowContextWithHint(valueOrDefault(result.WindowID, resolvedTarget), a11yWindowQueryHintFromArgs(args))
	return a11yJSON(map[string]interface{}{
		"host_os":    valueOrDefault(result.HostOS, backend.HostOS()),
		"window_id":  valueOrDefault(result.WindowID, resolvedTarget),
		"image_path": result.ImagePath,
		"message":    valueOrDefault(result.Message, "Host screenshot captured"),
	}), nil
}

func (t *A11yTool) cacheRefs(windowID string, refMap map[int]string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastWindow = strings.TrimSpace(windowID)
	t.lastRefMap = cloneA11yRefMap(refMap)
	t.lastRefs = nil
}

func (t *A11yTool) cacheSnapshotContext(windowID string, refMap map[int]string, tree string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastWindow = strings.TrimSpace(windowID)
	t.lastRefMap = cloneA11yRefMap(refMap)
	t.lastRefs = parseA11ySnapshotEntries(tree)
}

func (t *A11yTool) clearRefs() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastWindow = ""
	t.lastWindowHint = ""
	t.lastRefMap = nil
	t.lastRefs = nil
}

func (t *A11yTool) clearSnapshotRefs() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastRefMap = nil
	t.lastRefs = nil
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
	t.syncWindowContextWithHint(windowID, "")
}

func (t *A11yTool) syncWindowContextWithHint(windowID string, hint string) {
	windowID = strings.TrimSpace(windowID)
	hint = strings.TrimSpace(hint)
	if windowID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.lastWindow != "" && t.lastWindow != windowID {
		t.lastRefMap = nil
		t.lastRefs = nil
	}
	t.lastWindow = windowID
	t.lastWindowHint = hint
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

func (t *A11yTool) effectiveWindowContext() (string, string) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastWindow, t.lastWindowHint
}

func (t *A11yTool) currentA11yChatStructuredSnapshot(backend a11yruntime.Backend, windowID string) (*a11yruntime.Snapshot, string, bool) {
	provider, ok := backend.(a11yStructuredSnapshotProvider)
	if !ok {
		return nil, "", false
	}
	requestedWindow := strings.TrimSpace(windowID)
	if snapshot, ok := provider.CurrentStructuredSnapshot(requestedWindow); ok && snapshot != nil {
		return snapshot, valueOrDefault(strings.TrimSpace(snapshot.WindowID), requestedWindow), true
	}
	rememberedWindow, _ := t.effectiveWindowContext()
	rememberedWindow = strings.TrimSpace(rememberedWindow)
	if rememberedWindow == "" || rememberedWindow == requestedWindow {
		return nil, "", false
	}
	if snapshot, ok := provider.CurrentStructuredSnapshot(rememberedWindow); ok && snapshot != nil {
		return snapshot, valueOrDefault(strings.TrimSpace(snapshot.WindowID), rememberedWindow), true
	}
	return nil, "", false
}

func (t *A11yTool) maybeAutoFocusExactMatch(ctx context.Context, backend a11yruntime.Backend, match *a11yWindowMatch, windowID string) {
	if match == nil || !match.Exact || !match.Unique {
		return
	}
	switch strings.TrimSpace(match.MatchedBy) {
	case "window_title", "app_name", "window_title+app_name":
	default:
		return
	}
	resolved := strings.TrimSpace(windowID)
	if resolved == "" {
		return
	}
	if result, err := backend.FocusWindow(ctx, resolved); err == nil {
		t.syncWindowContext(valueOrDefault(result.WindowID, resolved))
	}
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

func a11yWindowQueryHintFromArgs(args map[string]interface{}) string {
	return a11yWindowQueryHint(
		firstCompatString(args, "window_title", "windowTitle", "title"),
		firstCompatString(args, "app_name", "appName", "application", "app"),
	)
}

func a11yWindowQueryHint(windowTitle string, appName string) string {
	parts := []string{
		normalizeA11yWindowMatchValue(windowTitle),
		normalizeA11yWindowMatchValue(appName),
	}
	return strings.Trim(strings.Join(parts, "|"), "|")
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
	sessionKey := t.a11yApprovalKey(ctx)
	if t.a11yApprovedForSession(sessionKey) {
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
	t.markA11yApprovedForSession(sessionKey)
	return nil, false, nil
}

func (t *A11yTool) a11yApprovalKey(ctx context.Context) string {
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

func (t *A11yTool) a11yApprovedForSession(key string) bool {
	if strings.TrimSpace(key) == "" {
		return false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, ok := t.approvedSessions[key]
	return ok
}

func (t *A11yTool) markA11yApprovedForSession(key string) {
	if strings.TrimSpace(key) == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.approvedSessions == nil {
		t.approvedSessions = make(map[string]struct{})
	}
	t.approvedSessions[key] = struct{}{}
}

func IsA11yActionHighRisk(action string, actType string) bool {
	switch strings.TrimSpace(strings.ToLower(action)) {
	case "act", "message", "type", "select", "click", "toggle":
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

func cloneA11ySnapshotEntries(in []a11ySnapshotEntry) []a11ySnapshotEntry {
	if len(in) == 0 {
		return nil
	}
	out := make([]a11ySnapshotEntry, len(in))
	copy(out, in)
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
	return a11yJSON(a11yErrorPayloadMap(err))
}

func a11yErrorPayloadMap(err error) map[string]interface{} {
	if err == nil {
		return map[string]interface{}{}
	}
	if runtimeErr, ok := err.(*a11yruntime.RuntimeError); ok {
		payload := map[string]interface{}{
			"error":      runtimeErr.Message,
			"error_code": runtimeErr.Code,
		}
		for key, value := range runtimeErr.Details {
			payload[key] = value
		}
		return payload
	}
	return map[string]interface{}{
		"error":      err.Error(),
		"error_code": "backend_unavailable",
	}
}

func a11yRunActionResultWithTimeout(ctx context.Context, action string, windowID string, fn func(context.Context) (a11yruntime.ActionResult, error)) (a11yruntime.ActionResult, error) {
	timeout := a11yHostInputActionTimeout
	if timeout <= 0 {
		return fn(ctx)
	}

	type actionResult struct {
		result a11yruntime.ActionResult
		err    error
	}

	actionCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	done := make(chan actionResult, 1)
	go func() {
		result, err := fn(actionCtx)
		done <- actionResult{result: result, err: err}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case out := <-done:
		return out.result, out.err
	case <-ctx.Done():
		return a11yruntime.ActionResult{}, ctx.Err()
	case <-timer.C:
		details := map[string]interface{}{
			"action":     strings.TrimSpace(valueOrDefault(action, "host_action")),
			"timeout_ms": timeout.Milliseconds(),
		}
		if trimmedWindow := strings.TrimSpace(windowID); trimmedWindow != "" {
			details["window_id"] = trimmedWindow
		}
		return a11yruntime.ActionResult{}, a11yruntime.NewError("backend_timeout", fmt.Sprintf("host %s action timed out", strings.TrimSpace(valueOrDefault(action, "host_action"))), details)
	}
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
