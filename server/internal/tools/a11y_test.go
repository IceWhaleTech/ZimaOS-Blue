package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	a11yruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

type a11yCompatBackend struct {
	capabilities                  a11yruntime.CapabilitiesResult
	hostOS                        string
	windows                       []a11yruntime.WindowInfo
	windowsResults                [][]a11yruntime.WindowInfo
	allWindows                    []a11yruntime.WindowInfo
	allWindowsResults             [][]a11yruntime.WindowInfo
	snapshotResult                a11yruntime.SnapshotResult
	interactiveResult             a11yruntime.SnapshotResult
	interactiveResults            []a11yruntime.SnapshotResult
	structuredSnapshot            *a11yruntime.Snapshot
	structuredSnapshots           []*a11yruntime.Snapshot
	interactiveResultAfterSubmit  a11yruntime.SnapshotResult
	structuredSnapshotAfterSubmit *a11yruntime.Snapshot
	snapshotErr                   error
	focusResultWindowID           string
	scrollResultWindowID          string
	keyResultWindowID             string
	screenshotResultID            string
	lastFocusWindowID             string
	lastSnapshotWindowID          string
	snapshotWindowHistory         []string
	lastActWindowID               string
	lastActRef                    int
	lastActType                   string
	lastActValue                  string
	lastActHoldMS                 int
	lastActRefMap                 map[int]string
	actTypeHistory                []string
	actRefHistory                 []int
	actValueHistory               []string
	lastScrollWindowID            string
	lastScrollDirection           string
	lastScrollLines               int
	lastPointerMoveX              int
	lastPointerMoveY              int
	lastPointClickWindow          string
	lastPointClick                a11yruntime.NormalizedPoint
	pointClickHistory             []a11yruntime.NormalizedPoint
	pointClickWindowIDs           []string
	lastPixelClickWindow          string
	lastPixelClickX               int
	lastPixelClickY               int
	lastPixelClickHoldMS          int
	pixelClickHistory             []a11yWindowPixelClick
	lastKeyWindowID               string
	lastKeys                      []string
	lastKeyHoldMS                 int
	keyHistory                    [][]string
	lastFocusedTypeWindowID       string
	lastFocusedTypeValue          string
	lastFocusedTypeHoldMS         int
	focusedTypeHistory            []string
	focusedTypeWindowHistory      []string
	actBlockCh                    chan struct{}
	keyBlockCh                    chan struct{}
	pointClickBlockCh             chan struct{}
	lastScreenshotWindow          string
	lastGroundingScreenshotWindow string
	actExecutionMode              string
	scrollExecutionMode           string
	keyExecutionMode              string
	screenshotImagePath           string
	screenshotGroundingBytes      []byte
	focusedTypeResultSet          bool
	focusedTypeResult             a11yruntime.ActionResult
	actResultSet                  bool
	actResult                     a11yruntime.ActionResult
	actCalls                      int
	interactiveCalls              int
	listWindowsCalls              int
	listAllWindowsCalls           int
	activateAppCalls              []string
	activateAppResult             a11yruntime.ActionResult
	activateAppErrs               map[string]error
	actErrorsByType               map[string]error
	keyErrorsByChord              map[string]error
	focusedTypeErr                error
	pointClickErr                 error
	pixelClickErr                 error
	runtimeUpdateHistory          []a11ySnapshotRuntimeUpdate
	interactiveCallsAfterSubmit   int
	structuredSnapshotCalls       int
}

type a11yWindowPixelClick struct {
	WindowID string
	X        int
	Y        int
	HoldMS   int
}

type a11ySnapshotRuntimeUpdate struct {
	WindowID string
	Token    string
	ActType  string
	Value    string
}

type a11yBrowserCompatBackend struct {
	tabs                    []BrowserTabResult
	a11yResult              BrowserA11yTreeResult
	interactiveResult       BrowserInteractiveResult
	screenshotTabData       string
	screenshotURLData       string
	lastA11yTargetID        string
	lastInteractiveTargetID string
	lastActMode             string
	lastActTargetID         string
	lastActRef              int
	lastActAction           string
	lastActValue            string
	lastPageScrollTargetID  string
	lastPageScrollX         int
	lastPageScrollY         int
	lastScreenshotTabTarget string
	lastScreenshotURL       string
	lastFocusedTargetID     string
	lastPressedTargetID     string
	lastPressedKeys         []string
	lastPressedHoldMS       int
}

func (b *a11yCompatBackend) HostOS() string {
	if strings.TrimSpace(b.hostOS) != "" {
		return strings.TrimSpace(b.hostOS)
	}
	return "darwin"
}

func (b *a11yCompatBackend) Capabilities(context.Context) (a11yruntime.CapabilitiesResult, error) {
	if b.capabilities.HostOS == "" {
		return a11yruntime.CapabilitiesResult{
			HostOS:             "darwin",
			SupportedActions:   []string{"capabilities", "windows", "focus", "snapshot", "snapshot_interactive", "act", "scroll", "pointer_move", "key", "screenshot"},
			UnsupportedActions: []string{},
		}, nil
	}
	return b.capabilities, nil
}

func (b *a11yCompatBackend) ListWindows(context.Context) ([]a11yruntime.WindowInfo, error) {
	b.listWindowsCalls++
	if len(b.windowsResults) > 0 {
		result := b.windowsResults[0]
		b.windowsResults = b.windowsResults[1:]
		return append([]a11yruntime.WindowInfo(nil), result...), nil
	}
	if len(b.windows) == 0 {
		return []a11yruntime.WindowInfo{{ID: "win-1", Title: "Example", Focused: true}}, nil
	}
	return b.windows, nil
}

func (b *a11yCompatBackend) ListAllWindows(context.Context) ([]a11yruntime.WindowInfo, error) {
	b.listAllWindowsCalls++
	if len(b.allWindowsResults) > 0 {
		result := b.allWindowsResults[0]
		b.allWindowsResults = b.allWindowsResults[1:]
		return append([]a11yruntime.WindowInfo(nil), result...), nil
	}
	if len(b.allWindows) == 0 {
		return nil, nil
	}
	return append([]a11yruntime.WindowInfo(nil), b.allWindows...), nil
}

func (b *a11yCompatBackend) FocusWindow(_ context.Context, windowID string) (a11yruntime.ActionResult, error) {
	b.lastFocusWindowID = windowID
	return a11yruntime.ActionResult{WindowID: b.focusResultWindowID, ExecutionMode: "semantic", Message: "focused"}, nil
}

func (b *a11yCompatBackend) ActivateApp(_ context.Context, appName string) (a11yruntime.ActionResult, error) {
	b.activateAppCalls = append(b.activateAppCalls, appName)
	if err := b.activateAppErrs[appName]; err != nil {
		return a11yruntime.ActionResult{}, err
	}
	result := b.activateAppResult
	if strings.TrimSpace(result.HostOS) == "" {
		result.HostOS = "darwin"
	}
	if strings.TrimSpace(result.ExecutionMode) == "" {
		result.ExecutionMode = "automation"
	}
	if strings.TrimSpace(result.Message) == "" {
		result.Message = "Application activated"
	}
	return result, nil
}

func (b *a11yCompatBackend) Snapshot(_ context.Context, windowID string) (a11yruntime.SnapshotResult, error) {
	b.lastSnapshotWindowID = windowID
	b.snapshotWindowHistory = append(b.snapshotWindowHistory, windowID)
	if b.snapshotErr != nil {
		return a11yruntime.SnapshotResult{}, b.snapshotErr
	}
	if b.snapshotResult.WindowID == "" {
		return a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-1",
			Title:    "Example",
			Tree:     "@1 [button] \"Continue\"",
			RefMap:   map[int]string{1: "token-1"},
		}, nil
	}
	return b.snapshotResult, nil
}

func (b *a11yCompatBackend) SnapshotInteractive(_ context.Context, windowID string) (a11yruntime.SnapshotResult, error) {
	b.lastSnapshotWindowID = windowID
	b.snapshotWindowHistory = append(b.snapshotWindowHistory, windowID)
	b.interactiveCalls++
	if len(b.runtimeUpdateHistory) > 0 && b.runtimeUpdateHistory[len(b.runtimeUpdateHistory)-1].ActType == "submit" {
		b.interactiveCallsAfterSubmit++
	}
	if len(b.interactiveResults) > 0 {
		result := b.interactiveResults[0]
		b.interactiveResults = b.interactiveResults[1:]
		if result.WindowID == "" {
			result.WindowID = windowID
		}
		if result.HostOS == "" {
			result.HostOS = "darwin"
		}
		return result, nil
	}
	if b.interactiveResult.WindowID == "" {
		return a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-1",
			Title:    "Example",
			Tree:     "@1 [button] \"Continue\"",
			RefMap:   map[int]string{1: "token-1"},
		}, nil
	}
	return b.interactiveResult, nil
}

func (b *a11yCompatBackend) Act(_ context.Context, windowID string, ref int, refMap map[int]string, actType string, value string, holdMS int) (a11yruntime.ActionResult, error) {
	if b.actBlockCh != nil {
		<-b.actBlockCh
	}
	b.lastActWindowID = windowID
	b.lastActRef = ref
	b.lastActType = actType
	b.lastActValue = value
	b.lastActHoldMS = holdMS
	b.lastActRefMap = refMap
	b.actTypeHistory = append(b.actTypeHistory, actType)
	b.actRefHistory = append(b.actRefHistory, ref)
	b.actValueHistory = append(b.actValueHistory, value)
	b.actCalls++
	if err := b.actErrorsByType[actType]; err != nil {
		return a11yruntime.ActionResult{}, err
	}
	if b.actResultSet {
		result := b.actResult
		if strings.TrimSpace(result.WindowID) == "" {
			result.WindowID = windowID
		}
		if strings.TrimSpace(result.HostOS) == "" {
			result.HostOS = "darwin"
		}
		if strings.TrimSpace(result.Message) == "" {
			result.Message = "ok"
		}
		if strings.TrimSpace(result.ExecutionMode) == "" {
			result.ExecutionMode = "semantic"
		}
		return result, nil
	}
	mode := b.actExecutionMode
	if mode == "" {
		mode = "semantic"
	}
	return a11yruntime.ActionResult{ExecutionMode: mode, Message: "ok"}, nil
}

func (b *a11yCompatBackend) Scroll(_ context.Context, windowID string, direction string, lines int) (a11yruntime.ActionResult, error) {
	b.lastScrollWindowID = windowID
	b.lastScrollDirection = direction
	b.lastScrollLines = lines
	mode := b.scrollExecutionMode
	if mode == "" {
		mode = "input"
	}
	return a11yruntime.ActionResult{WindowID: b.scrollResultWindowID, ExecutionMode: mode, Message: "scrolled"}, nil
}

func (b *a11yCompatBackend) PointerMove(context.Context, int, int) (a11yruntime.ActionResult, error) {
	return a11yruntime.ActionResult{ExecutionMode: "input", Message: "moved"}, nil
}

func (b *a11yCompatBackend) ClickWindowPoint(_ context.Context, windowID string, point a11yruntime.NormalizedPoint, holdMS int) (a11yruntime.ActionResult, error) {
	if b.pointClickBlockCh != nil {
		<-b.pointClickBlockCh
	}
	b.lastPointClickWindow = windowID
	b.lastPointClick = point
	b.pointClickWindowIDs = append(b.pointClickWindowIDs, windowID)
	b.pointClickHistory = append(b.pointClickHistory, point)
	if b.pointClickErr != nil {
		return a11yruntime.ActionResult{}, b.pointClickErr
	}
	return a11yruntime.ActionResult{
		HostOS:             b.HostOS(),
		WindowID:           windowID,
		ExecutionMode:      "input",
		TargetHit:          true,
		InputMethod:        "input_click",
		VerificationPassed: true,
		VerificationMethod: "point_click",
		Message:            "Host action completed",
	}, nil
}

func (b *a11yCompatBackend) ClickWindowPixel(_ context.Context, windowID string, x int, y int, holdMS int) (a11yruntime.ActionResult, error) {
	b.lastPixelClickWindow = windowID
	b.lastPixelClickX = x
	b.lastPixelClickY = y
	b.lastPixelClickHoldMS = holdMS
	b.pixelClickHistory = append(b.pixelClickHistory, a11yWindowPixelClick{
		WindowID: windowID,
		X:        x,
		Y:        y,
		HoldMS:   holdMS,
	})
	if b.pixelClickErr != nil {
		return a11yruntime.ActionResult{}, b.pixelClickErr
	}
	return a11yruntime.ActionResult{
		HostOS:             b.HostOS(),
		WindowID:           windowID,
		ExecutionMode:      "input",
		TargetHit:          true,
		InputMethod:        "input_click",
		VerificationPassed: true,
		VerificationMethod: "point_click",
		Message:            "Host action completed",
	}, nil
}

func (b *a11yCompatBackend) Key(_ context.Context, windowID string, keys []string, holdMS int) (a11yruntime.ActionResult, error) {
	if b.keyBlockCh != nil {
		<-b.keyBlockCh
	}
	b.lastKeyWindowID = windowID
	b.lastKeys = append([]string(nil), keys...)
	b.lastKeyHoldMS = holdMS
	b.keyHistory = append(b.keyHistory, append([]string(nil), keys...))
	if err := b.keyErrorsByChord[strings.Join(keys, "+")]; err != nil {
		return a11yruntime.ActionResult{}, err
	}
	mode := b.keyExecutionMode
	if mode == "" {
		mode = "input"
	}
	return a11yruntime.ActionResult{WindowID: b.keyResultWindowID, ExecutionMode: mode, Message: "keys"}, nil
}

func (b *a11yCompatBackend) TypeFocusedText(_ context.Context, windowID string, value string, holdMS int) (a11yruntime.ActionResult, error) {
	b.lastFocusedTypeWindowID = windowID
	b.lastFocusedTypeValue = value
	b.lastFocusedTypeHoldMS = holdMS
	b.focusedTypeWindowHistory = append(b.focusedTypeWindowHistory, windowID)
	b.focusedTypeHistory = append(b.focusedTypeHistory, value)
	if b.focusedTypeErr != nil {
		return a11yruntime.ActionResult{}, b.focusedTypeErr
	}
	if b.focusedTypeResultSet {
		result := b.focusedTypeResult
		if strings.TrimSpace(result.WindowID) == "" {
			result.WindowID = windowID
		}
		if strings.TrimSpace(result.HostOS) == "" {
			result.HostOS = b.HostOS()
		}
		if strings.TrimSpace(result.ExecutionMode) == "" {
			result.ExecutionMode = "input"
		}
		if strings.TrimSpace(result.InputMethod) == "" {
			result.InputMethod = "clipboard"
		}
		if strings.TrimSpace(result.VerificationMethod) == "" {
			result.VerificationMethod = "focused_text"
		}
		if strings.TrimSpace(result.Message) == "" {
			result.Message = "Host action completed"
		}
		result.TargetHit = true
		result.VerificationPassed = true
		return result, nil
	}
	return a11yruntime.ActionResult{
		HostOS:             b.HostOS(),
		WindowID:           windowID,
		ExecutionMode:      "input",
		TargetHit:          true,
		VerificationPassed: true,
		VerificationMethod: "focused_text",
		InputMethod:        "clipboard",
		Message:            "Host action completed",
	}, nil
}

func (b *a11yCompatBackend) Screenshot(_ context.Context, windowID string) (a11yruntime.ScreenshotResult, error) {
	b.lastScreenshotWindow = windowID
	imagePath := b.screenshotImagePath
	if imagePath == "" {
		imagePath = "/tmp/example.png"
	}
	resultWindow := windowID
	if b.screenshotResultID != "" {
		resultWindow = b.screenshotResultID
	}
	return a11yruntime.ScreenshotResult{HostOS: "darwin", WindowID: resultWindow, ImagePath: imagePath}, nil
}

func (b *a11yCompatBackend) ScreenshotForGrounding(_ context.Context, windowID string) (a11yruntime.ScreenshotResult, error) {
	b.lastGroundingScreenshotWindow = windowID
	resultWindow := windowID
	if b.screenshotResultID != "" {
		resultWindow = b.screenshotResultID
	}
	return a11yruntime.ScreenshotResult{
		HostOS:     "darwin",
		WindowID:   resultWindow,
		ImageBytes: append([]byte(nil), b.screenshotGroundingBytes...),
	}, nil
}

func (b *a11yCompatBackend) CurrentStructuredSnapshot(windowID string) (*a11yruntime.Snapshot, bool) {
	if b == nil {
		return nil, false
	}
	b.structuredSnapshotCalls++
	snapshot := b.structuredSnapshot
	if len(b.structuredSnapshots) > 0 {
		snapshot = b.structuredSnapshots[0]
		if len(b.structuredSnapshots) > 1 {
			b.structuredSnapshots = b.structuredSnapshots[1:]
		}
	}
	if snapshot == nil {
		return nil, false
	}
	if strings.TrimSpace(windowID) != "" && strings.TrimSpace(snapshot.WindowID) != "" && strings.TrimSpace(windowID) != strings.TrimSpace(snapshot.WindowID) {
		return nil, false
	}
	return snapshot.Clone(), true
}

func (b *a11yCompatBackend) UpdateSnapshotAfterAction(windowID string, token string, actType string, value string) {
	if b == nil {
		return
	}
	update := a11ySnapshotRuntimeUpdate{
		WindowID: strings.TrimSpace(windowID),
		Token:    strings.TrimSpace(token),
		ActType:  strings.TrimSpace(strings.ToLower(actType)),
		Value:    value,
	}
	b.runtimeUpdateHistory = append(b.runtimeUpdateHistory, update)
	if update.ActType == "submit" {
		if b.structuredSnapshotAfterSubmit != nil {
			b.structuredSnapshot = b.structuredSnapshotAfterSubmit.Clone()
		}
		if strings.TrimSpace(b.interactiveResultAfterSubmit.Tree) != "" || strings.TrimSpace(b.interactiveResultAfterSubmit.WindowID) != "" || len(b.interactiveResultAfterSubmit.RefMap) > 0 {
			b.interactiveResult = b.interactiveResultAfterSubmit
		}
	}
}

func (b *a11yBrowserCompatBackend) Start(context.Context) error { return nil }

func (b *a11yBrowserCompatBackend) Navigate(_ context.Context, url string, targetID string) (BrowserNavResult, error) {
	return BrowserNavResult{URL: url, Title: "Example", TargetID: targetID}, nil
}

func (b *a11yBrowserCompatBackend) CookieHeader(context.Context, string, string) (string, error) {
	return "", nil
}

func (b *a11yBrowserCompatBackend) ObserveNetwork(context.Context, string, int, bool) (BrowserObservedNetworkResult, error) {
	return BrowserObservedNetworkResult{}, nil
}

func (b *a11yBrowserCompatBackend) WaitNetworkIdle(context.Context, string, int, int) error {
	return nil
}

func (b *a11yBrowserCompatBackend) AccessibilityTree(_ context.Context, targetID string, _ int) (BrowserA11yTreeResult, error) {
	b.lastA11yTargetID = targetID
	result := b.a11yResult
	if result.TargetID == "" {
		result.TargetID = targetID
	}
	if result.URL == "" {
		result.URL = "https://example.com"
	}
	if result.Title == "" {
		result.Title = "Example"
	}
	if result.Tree == "" {
		result.Tree = "@1 [link] \"Example\""
	}
	if len(result.RefMap) == 0 {
		result.RefMap = map[int]int{1: 101}
	}
	return result, nil
}

func (b *a11yBrowserCompatBackend) InteractiveElements(_ context.Context, targetID string) (BrowserInteractiveResult, error) {
	b.lastInteractiveTargetID = targetID
	result := b.interactiveResult
	if result.TargetID == "" {
		result.TargetID = targetID
	}
	if result.URL == "" {
		result.URL = "https://example.com"
	}
	if result.Title == "" {
		result.Title = "Example"
	}
	if result.Tree == "" {
		result.Tree = "@1 [button] \"Continue\""
	}
	if len(result.RefMap) == 0 {
		result.RefMap = map[int]string{1: "button-continue"}
	}
	if result.Count == 0 {
		result.Count = len(result.RefMap)
	}
	return result, nil
}

func (b *a11yBrowserCompatBackend) CountInteractiveElements(context.Context, string) (int, error) {
	return 1, nil
}

func (b *a11yBrowserCompatBackend) ActByRef(_ context.Context, targetID string, ref int, _ map[int]int, action string, value string) error {
	b.lastActMode = "a11y"
	b.lastActTargetID = targetID
	b.lastActRef = ref
	b.lastActAction = action
	b.lastActValue = value
	return nil
}

func (b *a11yBrowserCompatBackend) ActByInteractiveRef(_ context.Context, targetID string, ref int, _ map[int]string, action string, value string) error {
	b.lastActMode = "interactive"
	b.lastActTargetID = targetID
	b.lastActRef = ref
	b.lastActAction = action
	b.lastActValue = value
	return nil
}

func (b *a11yBrowserCompatBackend) Screenshot(_ context.Context, url string) (string, error) {
	b.lastScreenshotURL = url
	if b.screenshotURLData != "" {
		return b.screenshotURLData, nil
	}
	return "browser-shot-url", nil
}

func (b *a11yBrowserCompatBackend) ScreenshotTab(_ context.Context, targetID string) (string, error) {
	b.lastScreenshotTabTarget = targetID
	if b.screenshotTabData != "" {
		return b.screenshotTabData, nil
	}
	return "browser-shot-tab", nil
}

func (b *a11yBrowserCompatBackend) CloseTab(context.Context, string) error { return nil }

func (b *a11yBrowserCompatBackend) Tabs(context.Context) ([]BrowserTabResult, error) {
	if len(b.tabs) == 0 {
		return []BrowserTabResult{{TargetID: "tab-1", URL: "https://example.com", Title: "Example", Active: true}}, nil
	}
	return append([]BrowserTabResult(nil), b.tabs...), nil
}

func (b *a11yBrowserCompatBackend) ExecuteRecipe(context.Context, string, map[string]string) (BrowserRecipeResult, error) {
	return BrowserRecipeResult{Success: true, Message: "ok"}, nil
}

func (b *a11yBrowserCompatBackend) ListRecipes(context.Context) []BrowserRecipeInfo { return nil }

func (b *a11yBrowserCompatBackend) PageScroll(_ context.Context, targetID string, x, y int) error {
	b.lastPageScrollTargetID = targetID
	b.lastPageScrollX = x
	b.lastPageScrollY = y
	return nil
}

func (b *a11yBrowserCompatBackend) FocusTab(_ context.Context, targetID string) error {
	b.lastFocusedTargetID = targetID
	return nil
}

func (b *a11yBrowserCompatBackend) PressKeys(_ context.Context, targetID string, keys []string, holdMS int) error {
	b.lastPressedTargetID = targetID
	b.lastPressedKeys = append([]string(nil), keys...)
	b.lastPressedHoldMS = holdMS
	return nil
}

func TestA11yDefinition_UsesCompactParamsEnvelope(t *testing.T) {
	tool := NewA11yTool()
	props := tool.Definition().Parameters["properties"].(map[string]interface{})
	if _, ok := props["params"]; !ok {
		t.Fatal("expected params property in a11y schema")
	}
	for _, key := range []string{"ref", "target_name", "target_role", "act_type", "value"} {
		if _, ok := props[key]; !ok {
			t.Fatalf("expected %q in a11y schema for top-level compatibility", key)
		}
	}
}

func TestA11yToolExecute_CapabilitiesReturnsHostActionsAndPermissions(t *testing.T) {
	backend := &a11yCompatBackend{
		capabilities: a11yruntime.CapabilitiesResult{
			HostOS:             "darwin",
			SupportedActions:   []string{"capabilities", "snapshot", "act"},
			UnsupportedActions: []string{"script"},
			Permissions: []a11yruntime.PermissionStatus{
				{Name: "accessibility", Granted: true, Required: true},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{"action": "capabilities"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["host_os"] != "darwin" {
		t.Fatalf("host_os = %v, want darwin", out["host_os"])
	}
	if _, ok := out["supported_actions"].([]interface{}); !ok {
		t.Fatalf("supported_actions = %#v, want JSON array", out["supported_actions"])
	}
	if _, ok := out["permissions"].([]interface{}); !ok {
		t.Fatalf("permissions = %#v, want JSON array", out["permissions"])
	}
}

func TestA11yToolExecute_ListAliasMapsToWindows(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-1", Title: "Example", Focused: true},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{"action": "list"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if _, ok := out["windows"].([]interface{}); !ok {
		t.Fatalf("windows = %#v, want JSON array", out["windows"])
	}
	if out["message"] != "Host windows listed" {
		t.Fatalf("message = %v, want Host windows listed", out["message"])
	}
}

func TestA11yToolExecute_ListWindowsAliasMapsToWindows(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-2", Title: "Feishu", Focused: true},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{"action": "list_windows"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if _, ok := out["windows"].([]interface{}); !ok {
		t.Fatalf("windows = %#v, want JSON array", out["windows"])
	}
	if out["message"] != "Host windows listed" {
		t.Fatalf("message = %v, want Host windows listed", out["message"])
	}
}

func TestA11yToolExecute_FocusWindowAliasMapsToFocus(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "focus_window",
		"window_id": "win-2",
	})
	if err != nil {
		t.Fatalf("focus_window Execute() error = %v", err)
	}
	if backend.lastFocusWindowID != "win-2" {
		t.Fatalf("lastFocusWindowID = %q, want win-2", backend.lastFocusWindowID)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "focused" {
		t.Fatalf("message = %v, want focused", out["message"])
	}
}

func TestA11yToolExecute_InspectWindowUITreeAliasMapsToSnapshotInteractive(t *testing.T) {
	backend := &a11yCompatBackend{
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-tree",
			Title:    "Feishu",
			Tree:     "@1 [button] \"Continue\"",
			RefMap:   map[int]string{1: "token-1"},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "inspect_window_ui_tree",
		"window_id": "win-tree",
	})
	if err != nil {
		t.Fatalf("inspect_window_ui_tree Execute() error = %v", err)
	}
	if backend.interactiveCalls != 1 {
		t.Fatalf("interactiveCalls = %d, want 1", backend.interactiveCalls)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["tree"] != "@1 [button] \"Continue\"" {
		t.Fatalf("tree = %v, want interactive tree", out["tree"])
	}
}

func TestA11yToolExecute_SnapshotCachesRefsAndActUsesBackend(t *testing.T) {
	backend := &a11yCompatBackend{
		snapshotResult: a11yruntime.SnapshotResult{
			HostOS:    "darwin",
			WindowID:  "win-9",
			Title:     "Example",
			Tree:      "@1 [button] \"Continue\"",
			RefMap:    map[int]string{1: "token-continue"},
			ImagePath: "/tmp/host-window-9.png",
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "snapshot",
		"window_id": "win-9",
	}); err != nil {
		t.Fatalf("snapshot Execute() error = %v", err)
	}

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "act",
		"window_id": "win-9",
		"params": map[string]interface{}{
			"ref":      "@1",
			"act_type": "click",
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.lastActRef != 1 || backend.lastActWindowID != "win-9" {
		t.Fatalf("last act = ref %d window %q, want ref 1 window win-9", backend.lastActRef, backend.lastActWindowID)
	}
	if got := backend.lastActRefMap[1]; got != "token-continue" {
		t.Fatalf("lastActRefMap[1] = %q, want token-continue", got)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["execution_mode"] != "semantic" {
		t.Fatalf("execution_mode = %v, want semantic", out["execution_mode"])
	}
}

func TestA11yToolExecute_SnapshotIncludesImagePath(t *testing.T) {
	backend := &a11yCompatBackend{
		snapshotResult: a11yruntime.SnapshotResult{
			HostOS:    "darwin",
			WindowID:  "win-9",
			Title:     "Example",
			Tree:      "@1 [button] \"Continue\"",
			RefMap:    map[int]string{1: "token-continue"},
			ImagePath: "/tmp/host-window-9.png",
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "snapshot",
		"window_id": "win-9",
	})
	if err != nil {
		t.Fatalf("snapshot Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["image_path"] != "/tmp/host-window-9.png" {
		t.Fatalf("image_path = %v, want /tmp/host-window-9.png", out["image_path"])
	}
	if out["message"] != "Host computer-use snapshot ready" {
		t.Fatalf("message = %v, want Host computer-use snapshot ready", out["message"])
	}
}

func TestA11yToolExecute_MissingHostBackendUsesComputerUseBranding(t *testing.T) {
	tool := NewA11yTool()

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "snapshot",
	})
	if err != nil {
		t.Fatalf("snapshot Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error"] != "host computer-use backend not available" {
		t.Fatalf("error = %v, want host computer-use backend not available", out["error"])
	}
	if out["error_code"] != "backend_unavailable" {
		t.Fatalf("error_code = %v, want backend_unavailable", out["error_code"])
	}
}

func TestA11yToolExecute_BrowserCapabilitiesUsesComputerUseBranding(t *testing.T) {
	tool := NewA11yTool()
	tool.SetBackend(&a11yCompatBackend{})
	tool.SetBrowser(&a11yBrowserCompatBackend{})

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":  "capabilities",
		"surface": "browser",
	})
	if err != nil {
		t.Fatalf("capabilities Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "Browser computer-use bridge ready" {
		t.Fatalf("message = %v, want Browser computer-use bridge ready", out["message"])
	}
}

func TestA11yToolExecute_StaleRefAfterSnapshotReplacement(t *testing.T) {
	backend := &a11yCompatBackend{
		snapshotResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-1",
			Title:    "First",
			Tree:     "@2 [button] \"Old\"",
			RefMap:   map[int]string{2: "token-old"},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{"action": "snapshot", "window_id": "win-1"}); err != nil {
		t.Fatalf("first snapshot error = %v", err)
	}

	backend.snapshotResult = a11yruntime.SnapshotResult{
		HostOS:   "darwin",
		WindowID: "win-1",
		Title:    "Second",
		Tree:     "@1 [button] \"New\"",
		RefMap:   map[int]string{1: "token-new"},
	}
	if _, err := tool.Execute(context.Background(), map[string]interface{}{"action": "snapshot", "window_id": "win-1"}); err != nil {
		t.Fatalf("second snapshot error = %v", err)
	}

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "act",
		"params": map[string]interface{}{
			"ref":      "@2",
			"act_type": "click",
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.actCalls != 0 {
		t.Fatalf("actCalls = %d, want 0 when stale ref should short-circuit", backend.actCalls)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "stale_ref" {
		t.Fatalf("error_code = %v, want stale_ref", out["error_code"])
	}
}

func TestA11yToolExecute_PropagatesPermissionRequiredDetails(t *testing.T) {
	backend := &a11yCompatBackend{
		snapshotErr: a11yruntime.NewError("permission_required", "Grant Accessibility permission", map[string]interface{}{
			"image_path": "/tmp/host-window-1.png",
			"permissions": []a11yruntime.PermissionStatus{
				{Name: "accessibility", Granted: false, Required: true, Message: "Grant Accessibility permission"},
			},
		}),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{"action": "snapshot"})
	if err != nil {
		t.Fatalf("snapshot Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "permission_required" {
		t.Fatalf("error_code = %v, want permission_required", out["error_code"])
	}
	if _, ok := out["permissions"].([]interface{}); !ok {
		t.Fatalf("permissions = %#v, want JSON array", out["permissions"])
	}
	if out["image_path"] != "/tmp/host-window-1.png" {
		t.Fatalf("image_path = %v, want /tmp/host-window-1.png", out["image_path"])
	}
}

func TestA11yToolExecute_SnapshotWarnsEarlyAndFallsBackToScreenshotWhenPermissionDenied(t *testing.T) {
	backend := &a11yCompatBackend{
		capabilities: a11yruntime.CapabilitiesResult{
			HostOS: "darwin",
			Permissions: []a11yruntime.PermissionStatus{
				{Name: "accessibility", Granted: false, Required: true, Message: "Grant Accessibility permission"},
			},
		},
		screenshotImagePath: "/tmp/host-window-2.png",
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{"action": "snapshot", "window_id": "win-2"})
	if err != nil {
		t.Fatalf("snapshot Execute() error = %v", err)
	}
	if backend.lastSnapshotWindowID != "" {
		t.Fatalf("lastSnapshotWindowID = %q, want empty because snapshot should not run", backend.lastSnapshotWindowID)
	}
	if backend.lastScreenshotWindow != "win-2" {
		t.Fatalf("lastScreenshotWindow = %q, want win-2", backend.lastScreenshotWindow)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if degraded, ok := out["degraded"].(bool); !ok || !degraded {
		t.Fatalf("degraded = %#v, want true", out["degraded"])
	}
	if warning, ok := out["permission_warning"].(bool); !ok || !warning {
		t.Fatalf("permission_warning = %#v, want true", out["permission_warning"])
	}
	if out["image_path"] != "/tmp/host-window-2.png" {
		t.Fatalf("image_path = %v, want /tmp/host-window-2.png", out["image_path"])
	}
	if _, ok := out["permissions"].([]interface{}); !ok {
		t.Fatalf("permissions = %#v, want JSON array", out["permissions"])
	}
}

func TestA11yToolExecute_ActWarnsEarlyWhenPermissionDenied(t *testing.T) {
	backend := &a11yCompatBackend{
		capabilities: a11yruntime.CapabilitiesResult{
			HostOS: "darwin",
			Permissions: []a11yruntime.PermissionStatus{
				{Name: "accessibility", Granted: false, Required: true, Message: "Grant Accessibility permission"},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)
	tool.cacheRefs("win-2", map[int]string{2: "token-2"})

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "act",
		"window_id": "win-2",
		"params": map[string]interface{}{
			"ref":      "@2",
			"act_type": "click",
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.actCalls != 0 {
		t.Fatalf("actCalls = %d, want 0 when permission warning should short-circuit", backend.actCalls)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "permission_required" {
		t.Fatalf("error_code = %v, want permission_required", out["error_code"])
	}
	if degraded, ok := out["degraded"].(bool); !ok || !degraded {
		t.Fatalf("degraded = %#v, want true", out["degraded"])
	}
	if _, ok := out["fallback_actions"].([]interface{}); !ok {
		t.Fatalf("fallback_actions = %#v, want JSON array", out["fallback_actions"])
	}
}

func TestA11yToolExecute_ActSuccessInvalidatesSnapshotRefs(t *testing.T) {
	backend := &a11yCompatBackend{
		snapshotResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-1",
			Title:    "First",
			Tree:     "@1 [button] \"Open\"",
			RefMap:   map[int]string{1: "token-open"},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{"action": "snapshot", "window_id": "win-1"}); err != nil {
		t.Fatalf("snapshot error = %v", err)
	}
	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "act",
		"params": map[string]interface{}{
			"ref":      "@1",
			"act_type": "click",
		},
	}); err != nil {
		t.Fatalf("first act error = %v", err)
	}

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "act",
		"params": map[string]interface{}{
			"ref":      "@1",
			"act_type": "click",
		},
	})
	if err != nil {
		t.Fatalf("second act Execute() error = %v", err)
	}
	if backend.actCalls != 1 {
		t.Fatalf("actCalls = %d, want 1 after refs are invalidated", backend.actCalls)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "stale_ref" {
		t.Fatalf("error_code = %v, want stale_ref", out["error_code"])
	}
}

func TestA11yToolExecute_KeySuccessInvalidatesSnapshotRefs(t *testing.T) {
	backend := &a11yCompatBackend{
		snapshotResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-1",
			Title:    "First",
			Tree:     "@1 [button] \"Open\"",
			RefMap:   map[int]string{1: "token-open"},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{"action": "snapshot", "window_id": "win-1"}); err != nil {
		t.Fatalf("snapshot error = %v", err)
	}
	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "key",
		"params": map[string]interface{}{
			"keys": []interface{}{"cmd", "l"},
		},
	}); err != nil {
		t.Fatalf("key error = %v", err)
	}

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "act",
		"params": map[string]interface{}{
			"ref":      "@1",
			"act_type": "click",
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.actCalls != 0 {
		t.Fatalf("actCalls = %d, want 0 after key invalidates refs", backend.actCalls)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "stale_ref" {
		t.Fatalf("error_code = %v, want stale_ref", out["error_code"])
	}
}

func TestA11yToolExecute_ScrollSuccessInvalidatesSnapshotRefsEvenWhenWindowUnchanged(t *testing.T) {
	backend := &a11yCompatBackend{
		snapshotResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-1",
			Title:    "First",
			Tree:     "@1 [button] \"Open\"",
			RefMap:   map[int]string{1: "token-open"},
		},
		scrollResultWindowID: "win-1",
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{"action": "snapshot", "window_id": "win-1"}); err != nil {
		t.Fatalf("snapshot error = %v", err)
	}
	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "scroll",
		"params": map[string]interface{}{
			"direction": "down",
		},
	}); err != nil {
		t.Fatalf("scroll error = %v", err)
	}

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "act",
		"params": map[string]interface{}{
			"ref":      "@1",
			"act_type": "click",
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.actCalls != 0 {
		t.Fatalf("actCalls = %d, want 0 after scroll invalidates refs", backend.actCalls)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "stale_ref" {
		t.Fatalf("error_code = %v, want stale_ref", out["error_code"])
	}
}

func TestA11yToolExecute_PropagatesBackendUnavailableImageDetails(t *testing.T) {
	backend := &a11yCompatBackend{
		snapshotErr: a11yruntime.NewError("backend_unavailable", "MSAA snapshot is empty", map[string]interface{}{
			"window_id":  "win-9",
			"image_path": "/tmp/host-window-9.png",
		}),
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{"action": "snapshot", "window_id": "win-9"})
	if err != nil {
		t.Fatalf("snapshot Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "backend_unavailable" {
		t.Fatalf("error_code = %v, want backend_unavailable", out["error_code"])
	}
	if out["window_id"] != "win-9" {
		t.Fatalf("window_id = %v, want win-9", out["window_id"])
	}
	if out["image_path"] != "/tmp/host-window-9.png" {
		t.Fatalf("image_path = %v, want /tmp/host-window-9.png", out["image_path"])
	}
}

func TestA11yToolExecute_FocusInvalidatesSnapshotRefs(t *testing.T) {
	backend := &a11yCompatBackend{
		snapshotResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-1",
			Title:    "First",
			Tree:     "@1 [button] \"Open\"",
			RefMap:   map[int]string{1: "token-open"},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{"action": "snapshot", "window_id": "win-1"}); err != nil {
		t.Fatalf("snapshot error = %v", err)
	}
	if _, err := tool.Execute(context.Background(), map[string]interface{}{"action": "focus", "window_id": "win-2"}); err != nil {
		t.Fatalf("focus error = %v", err)
	}

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "act",
		"params": map[string]interface{}{
			"ref":      "@1",
			"act_type": "click",
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.actCalls != 0 {
		t.Fatalf("actCalls = %d, want 0 after focus invalidates refs", backend.actCalls)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "stale_ref" {
		t.Fatalf("error_code = %v, want stale_ref", out["error_code"])
	}
}

func TestA11yToolExecute_FocusCachesWindowForSubsequentScreenshot(t *testing.T) {
	backend := &a11yCompatBackend{focusResultWindowID: "win-2"}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{"action": "focus", "window_id": "win-1"}); err != nil {
		t.Fatalf("focus error = %v", err)
	}
	if _, err := tool.Execute(context.Background(), map[string]interface{}{"action": "screenshot"}); err != nil {
		t.Fatalf("screenshot error = %v", err)
	}
	if backend.lastScreenshotWindow != "win-2" {
		t.Fatalf("lastScreenshotWindow = %q, want win-2", backend.lastScreenshotWindow)
	}
}

func TestA11yToolExecute_FocusResolvesUniqueExactWindowAliasBeforeFocus(t *testing.T) {
	backend := &a11yCompatBackend{
		focusResultWindowID: "win-2",
		windows: []a11yruntime.WindowInfo{
			{ID: "win-1", Title: "Code", AppName: "Code"},
			{ID: "win-2", Title: "Feishu", AppName: "Feishu"},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "focus",
		"window_id": "Feishu",
	}); err != nil {
		t.Fatalf("focus error = %v", err)
	}
	if backend.lastFocusWindowID != "win-2" {
		t.Fatalf("lastFocusWindowID = %q, want win-2", backend.lastFocusWindowID)
	}
}

func TestA11yToolExecute_SnapshotResolvesWindowTitleBeforeSnapshot(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-1", Title: "Code", AppName: "Code"},
			{ID: "win-2", Title: "Feishu", AppName: "Feishu"},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "snapshot_interactive",
		"window_title": "Feishu",
	}); err != nil {
		t.Fatalf("snapshot_interactive error = %v", err)
	}
	if backend.lastSnapshotWindowID != "win-2" {
		t.Fatalf("lastSnapshotWindowID = %q, want win-2", backend.lastSnapshotWindowID)
	}
}

func TestA11yToolExecute_SnapshotResolvesFuzzyWindowTitleAliasesBeforeSnapshot(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-1", Title: "Code", AppName: "Code"},
			{ID: "win-2", Title: "Lark - Orca", AppName: "Lark"},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "snapshot_interactive",
		"window_title": "Feishu，飞书，Lark",
	}); err != nil {
		t.Fatalf("snapshot_interactive error = %v", err)
	}
	if backend.lastSnapshotWindowID != "win-2" {
		t.Fatalf("lastSnapshotWindowID = %q, want win-2", backend.lastSnapshotWindowID)
	}
}

func TestA11yToolExecute_SnapshotResolvesBestFuzzyWindowTitleCandidateBeforeSnapshot(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-1", Title: "Feishu - Docs", AppName: "Lark"},
			{ID: "win-2", Title: "Lark Feishu Orca", AppName: "Lark"},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "snapshot_interactive",
		"window_title": "Feishu 飞书 Lark Orca",
	}); err != nil {
		t.Fatalf("snapshot_interactive error = %v", err)
	}
	if backend.lastSnapshotWindowID != "win-2" {
		t.Fatalf("lastSnapshotWindowID = %q, want win-2", backend.lastSnapshotWindowID)
	}
}

func TestA11yToolExecute_FocusRejectsAmbiguousWindowAlias(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-1", Title: "Feishu", AppName: "Feishu"},
			{ID: "win-2", Title: "Feishu", AppName: "Feishu"},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "focus",
		"window_title": "Feishu",
	})
	if err != nil {
		t.Fatalf("focus Execute() error = %v", err)
	}
	if backend.lastFocusWindowID != "" {
		t.Fatalf("lastFocusWindowID = %q, want empty on ambiguity", backend.lastFocusWindowID)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "backend_unavailable" {
		t.Fatalf("error_code = %v, want backend_unavailable", out["error_code"])
	}
	if out["error"] != "target window is ambiguous" {
		t.Fatalf("error = %v, want target window is ambiguous", out["error"])
	}
}

func TestA11yToolExecute_WindowsCachesFocusedWindowForSubsequentScreenshot(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-1", Title: "Code"},
			{ID: "win-2", Title: "Feishu", Focused: true},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{"action": "windows"}); err != nil {
		t.Fatalf("windows error = %v", err)
	}
	if _, err := tool.Execute(context.Background(), map[string]interface{}{"action": "screenshot"}); err != nil {
		t.Fatalf("screenshot error = %v", err)
	}
	if backend.lastScreenshotWindow != "win-2" {
		t.Fatalf("lastScreenshotWindow = %q, want win-2", backend.lastScreenshotWindow)
	}
}

func TestA11yToolExecute_WindowsRefreshInvalidatesSnapshotRefs(t *testing.T) {
	backend := &a11yCompatBackend{
		snapshotResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-1",
			Title:    "First",
			Tree:     "@1 [button] \"Open\"",
			RefMap:   map[int]string{1: "token-open"},
		},
		windows: []a11yruntime.WindowInfo{
			{ID: "win-1", Title: "Code"},
			{ID: "win-2", Title: "Feishu", Focused: true},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{"action": "snapshot", "window_id": "win-1"}); err != nil {
		t.Fatalf("snapshot error = %v", err)
	}
	if _, err := tool.Execute(context.Background(), map[string]interface{}{"action": "windows"}); err != nil {
		t.Fatalf("windows error = %v", err)
	}

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "act",
		"params": map[string]interface{}{
			"ref":      "@1",
			"act_type": "click",
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.actCalls != 0 {
		t.Fatalf("actCalls = %d, want 0 after window refresh invalidates refs", backend.actCalls)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "stale_ref" {
		t.Fatalf("error_code = %v, want stale_ref", out["error_code"])
	}
}

func TestA11yToolExecute_ScrollCachesResolvedWindowForSubsequentKey(t *testing.T) {
	backend := &a11yCompatBackend{scrollResultWindowID: "win-3"}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "scroll",
		"window_id": "win-1",
		"params": map[string]interface{}{
			"direction": "down",
		},
	}); err != nil {
		t.Fatalf("scroll error = %v", err)
	}
	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "key",
		"params": map[string]interface{}{
			"keys": []interface{}{"cmd", "l"},
		},
	}); err != nil {
		t.Fatalf("key error = %v", err)
	}
	if backend.lastKeyWindowID != "win-3" {
		t.Fatalf("lastKeyWindowID = %q, want win-3", backend.lastKeyWindowID)
	}
}

func TestA11yToolExecute_ScrollRefreshInvalidatesSnapshotRefs(t *testing.T) {
	backend := &a11yCompatBackend{
		snapshotResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-1",
			Title:    "First",
			Tree:     "@1 [button] \"Open\"",
			RefMap:   map[int]string{1: "token-open"},
		},
		scrollResultWindowID: "win-2",
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{"action": "snapshot", "window_id": "win-1"}); err != nil {
		t.Fatalf("snapshot error = %v", err)
	}
	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "scroll",
		"params": map[string]interface{}{
			"direction": "down",
		},
	}); err != nil {
		t.Fatalf("scroll error = %v", err)
	}

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "act",
		"params": map[string]interface{}{
			"ref":      "@1",
			"act_type": "click",
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.actCalls != 0 {
		t.Fatalf("actCalls = %d, want 0 after scroll refresh invalidates refs", backend.actCalls)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "stale_ref" {
		t.Fatalf("error_code = %v, want stale_ref", out["error_code"])
	}
}

func TestA11yToolExecute_ActRejectsWindowMismatchAgainstSnapshotContext(t *testing.T) {
	backend := &a11yCompatBackend{
		snapshotResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-1",
			Title:    "First",
			Tree:     "@1 [button] \"Open\"",
			RefMap:   map[int]string{1: "token-open"},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{"action": "snapshot", "window_id": "win-1"}); err != nil {
		t.Fatalf("snapshot error = %v", err)
	}

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "act",
		"window_id": "win-2",
		"params": map[string]interface{}{
			"ref":      "@1",
			"act_type": "click",
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.actCalls != 0 {
		t.Fatalf("actCalls = %d, want 0 on mismatched window context", backend.actCalls)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "stale_ref" {
		t.Fatalf("error_code = %v, want stale_ref", out["error_code"])
	}
}

func TestA11yToolExecute_HighRiskActionPendingCheckpointShortCircuits(t *testing.T) {
	backend := &a11yCompatBackend{
		snapshotResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-risk",
			Title:    "Risky",
			Tree:     "@1 [button] \"Continue\"",
			RefMap:   map[int]string{1: "token-risk"},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	ctx := WithSessionID(context.Background(), "conv-a11y-risk")
	ctx = WithBrowserCheckpointRequester(ctx, func(_ context.Context, req BrowserCheckpointRequest) (BrowserCheckpointResult, error) {
		return BrowserCheckpointResult{
			Pending:      true,
			CheckpointID: "cp-a11y",
			Message:      "need confirmation",
		}, nil
	})

	if _, err := tool.Execute(ctx, map[string]interface{}{"action": "snapshot", "window_id": "win-risk"}); err != nil {
		t.Fatalf("snapshot error = %v", err)
	}
	raw, err := tool.Execute(ctx, map[string]interface{}{
		"action": "act",
		"params": map[string]interface{}{
			"ref":      "@1",
			"act_type": "click",
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.actCalls != 0 {
		t.Fatalf("actCalls = %d, want 0 when checkpoint is pending", backend.actCalls)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if got := out["checkpoint_pending"]; got != true {
		t.Fatalf("checkpoint_pending = %v, want true", got)
	}
	if got := out["checkpoint_id"]; got != "cp-a11y" {
		t.Fatalf("checkpoint_id = %v, want cp-a11y", got)
	}
}

func TestA11yToolExecute_KeyPendingCheckpointShortCircuits(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	var captured BrowserCheckpointRequest
	ctx := WithSessionID(context.Background(), "conv-a11y-key-risk")
	ctx = WithBrowserCheckpointRequester(ctx, func(_ context.Context, req BrowserCheckpointRequest) (BrowserCheckpointResult, error) {
		captured = req
		return BrowserCheckpointResult{
			Pending:      true,
			CheckpointID: "cp-a11y-key",
			Message:      "need confirmation",
		}, nil
	})

	raw, err := tool.Execute(ctx, map[string]interface{}{
		"action": "key",
		"params": map[string]interface{}{
			"keys": []interface{}{"cmd", "q"},
		},
	})
	if err != nil {
		t.Fatalf("key Execute() error = %v", err)
	}
	if len(backend.lastKeys) != 0 {
		t.Fatalf("lastKeys = %#v, want no backend key dispatch when checkpoint is pending", backend.lastKeys)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if got := out["checkpoint_pending"]; got != true {
		t.Fatalf("checkpoint_pending = %v, want true", got)
	}
	if got := out["checkpoint_id"]; got != "cp-a11y-key" {
		t.Fatalf("checkpoint_id = %v, want cp-a11y-key", got)
	}
	if captured.Step != "key" {
		t.Fatalf("checkpoint step = %q, want key", captured.Step)
	}
	if captured.Action != "key" {
		t.Fatalf("checkpoint action = %q, want key", captured.Action)
	}
}

func TestA11yToolExecute_HighRiskApprovalCachedForSession(t *testing.T) {
	backend := &a11yCompatBackend{
		snapshotResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-risk",
			Title:    "Risky",
			Tree:     "@1 [button] \"Continue\"",
			RefMap:   map[int]string{1: "token-risk"},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	seen := 0
	ctx := WithUserID(WithSessionID(context.Background(), "conv-a11y-approve-once"), "user-a11y")
	ctx = WithBrowserCheckpointRequester(ctx, func(_ context.Context, req BrowserCheckpointRequest) (BrowserCheckpointResult, error) {
		seen++
		return BrowserCheckpointResult{
			Decision:     BrowserCheckpointApprove,
			CheckpointID: "cp-a11y-approve-once",
		}, nil
	})

	if _, err := tool.Execute(ctx, map[string]interface{}{"action": "snapshot", "window_id": "win-risk"}); err != nil {
		t.Fatalf("snapshot error = %v", err)
	}
	for i := 0; i < 2; i++ {
		if i > 0 {
			if _, err := tool.Execute(ctx, map[string]interface{}{"action": "snapshot", "window_id": "win-risk"}); err != nil {
				t.Fatalf("snapshot #%d error = %v", i+1, err)
			}
		}
		if _, err := tool.Execute(ctx, map[string]interface{}{
			"action": "act",
			"params": map[string]interface{}{
				"ref":      "@1",
				"act_type": "click",
			},
		}); err != nil {
			t.Fatalf("act Execute() #%d error = %v", i+1, err)
		}
	}
	if seen != 1 {
		t.Fatalf("checkpoint seen = %d, want 1 after repeated high-risk actions in same session", seen)
	}
	if backend.actCalls != 2 {
		t.Fatalf("actCalls = %d, want 2 successful act dispatches", backend.actCalls)
	}
}

func TestA11yToolExecute_HighRiskApprovalCoversKeyAndActInSameSession(t *testing.T) {
	backend := &a11yCompatBackend{
		snapshotResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-risk",
			Title:    "Risky",
			Tree:     "@1 [button] \"Continue\"",
			RefMap:   map[int]string{1: "token-risk"},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	seen := 0
	ctx := WithUserID(WithSessionID(context.Background(), "conv-a11y-mixed-approve-once"), "user-a11y")
	ctx = WithBrowserCheckpointRequester(ctx, func(_ context.Context, req BrowserCheckpointRequest) (BrowserCheckpointResult, error) {
		seen++
		return BrowserCheckpointResult{
			Decision:     BrowserCheckpointApprove,
			CheckpointID: "cp-a11y-mixed-approve-once",
		}, nil
	})

	if _, err := tool.Execute(ctx, map[string]interface{}{
		"action": "key",
		"params": map[string]interface{}{
			"keys": []interface{}{"cmd", "k"},
		},
	}); err != nil {
		t.Fatalf("key Execute() error = %v", err)
	}
	if _, err := tool.Execute(ctx, map[string]interface{}{"action": "snapshot", "window_id": "win-risk"}); err != nil {
		t.Fatalf("snapshot error = %v", err)
	}
	if _, err := tool.Execute(ctx, map[string]interface{}{
		"action": "act",
		"params": map[string]interface{}{
			"ref":      "@1",
			"act_type": "click",
		},
	}); err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if seen != 1 {
		t.Fatalf("checkpoint seen = %d, want 1 across key + act in same session", seen)
	}
	if len(backend.lastKeys) == 0 {
		t.Fatalf("lastKeys = %#v, want key dispatch to succeed", backend.lastKeys)
	}
	if backend.actCalls != 1 {
		t.Fatalf("actCalls = %d, want 1 successful act dispatch", backend.actCalls)
	}
}

func TestA11yToolExecute_ActForwardsHoldMS(t *testing.T) {
	backend := &a11yCompatBackend{
		snapshotResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-hold",
			Title:    "Hold",
			Tree:     "@1 [button] \"Hold\"",
			RefMap:   map[int]string{1: "token-hold"},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{"action": "snapshot", "window_id": "win-hold"}); err != nil {
		t.Fatalf("snapshot error = %v", err)
	}

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "act",
		"params": map[string]interface{}{
			"ref":      "@1",
			"act_type": "long_press",
			"hold_ms":  1234,
		},
	}); err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}

	if backend.lastActHoldMS != 1234 {
		t.Fatalf("lastActHoldMS = %d, want 1234", backend.lastActHoldMS)
	}
}

func TestA11yToolExecute_ActIncludesTelemetryFieldsWhenVerificationFails(t *testing.T) {
	backend := &a11yCompatBackend{
		snapshotResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-telemetry",
			Title:    "Telemetry",
			Tree:     "@1 [input] \"Composer\"",
			RefMap:   map[int]string{1: "token-input"},
		},
		actResult: a11yruntime.ActionResult{
			HostOS:             "darwin",
			WindowID:           "win-telemetry",
			ExecutionMode:      "input",
			Intent:             "message",
			TargetHit:          false,
			VerificationPassed: false,
			VerificationMethod: "ocr",
			InputMethod:        "clipboard",
			Fallbacks:          []string{"set_value", "clipboard", "verify_failed"},
			OverlayMode:        "mask",
			Message:            "Host action completed",
		},
		actResultSet: true,
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "snapshot",
		"window_id": "win-telemetry",
	}); err != nil {
		t.Fatalf("snapshot error = %v", err)
	}

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "act",
		"params": map[string]interface{}{
			"ref":      "@1",
			"act_type": "type",
			"intent":   "message",
			"value":    "hello",
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if got := out["intent"]; got != "message" {
		t.Fatalf("intent = %v, want message", got)
	}
	if got, ok := out["target_hit"]; !ok || got != false {
		t.Fatalf("target_hit = %#v (present=%v), want false", got, ok)
	}
	if got, ok := out["verification_passed"]; !ok || got != false {
		t.Fatalf("verification_passed = %#v (present=%v), want false", got, ok)
	}
	if got := out["verification_method"]; got != "ocr" {
		t.Fatalf("verification_method = %v, want ocr", got)
	}
	if got := out["input_method"]; got != "clipboard" {
		t.Fatalf("input_method = %v, want clipboard", got)
	}
	if got := out["overlay_mode"]; got != "mask" {
		t.Fatalf("overlay_mode = %v, want mask", got)
	}
	fallbacks, ok := out["fallbacks"].([]interface{})
	if !ok || len(fallbacks) != 3 {
		t.Fatalf("fallbacks = %#v, want 3 entries", out["fallbacks"])
	}
}

func TestA11yToolExecute_ActClickWithWindowPixelCoordinatesUsesPixelClickCompat(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "act",
		"params": map[string]interface{}{
			"act_type":  "click",
			"window_id": "win-pixel",
			"x":         100,
			"y":         120,
			"hold_ms":   750,
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if backend.actCalls != 0 {
		t.Fatalf("actCalls = %d, want 0 when pixel click compat handles the request", backend.actCalls)
	}
	if len(backend.pixelClickHistory) != 1 {
		t.Fatalf("pixelClickHistory = %#v, want one pixel click", backend.pixelClickHistory)
	}
	if got := backend.pixelClickHistory[0]; got.WindowID != "win-pixel" || got.X != 100 || got.Y != 120 || got.HoldMS != 750 {
		t.Fatalf("pixelClickHistory[0] = %#v, want window=win-pixel x=100 y=120 hold=750", got)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["execution_mode"] != "input" {
		t.Fatalf("execution_mode = %v, want input", out["execution_mode"])
	}
	if out["window_id"] != "win-pixel" {
		t.Fatalf("window_id = %v, want win-pixel", out["window_id"])
	}
}

func TestA11yToolExecute_ActClickWithCoordinatesAndSelectorStaysSemantic(t *testing.T) {
	backend := &a11yCompatBackend{
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-semantic",
			Title:    "Feishu",
			Tree:     "@1 [button] \"Continue\"",
			RefMap:   map[int]string{1: "token-continue"},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "act",
		"params": map[string]interface{}{
			"act_type":    "click",
			"window_id":   "win-semantic",
			"target_role": "button",
			"target_name": "Continue",
			"x":           100,
			"y":           120,
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if len(backend.pixelClickHistory) != 0 {
		t.Fatalf("pixelClickHistory = %#v, want no pixel click when selector is explicit", backend.pixelClickHistory)
	}
	if backend.actCalls != 1 {
		t.Fatalf("actCalls = %d, want 1 semantic click", backend.actCalls)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["execution_mode"] != "semantic" {
		t.Fatalf("execution_mode = %v, want semantic", out["execution_mode"])
	}
}

func TestA11yToolExecute_KeyDefaultsAndForwardsHoldMS(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "key",
		"params": map[string]interface{}{
			"keys": []interface{}{"cmd", "shift", "p"},
		},
	}); err != nil {
		t.Fatalf("key Execute() default hold error = %v", err)
	}
	if backend.lastKeyHoldMS != 600 {
		t.Fatalf("default lastKeyHoldMS = %d, want 600", backend.lastKeyHoldMS)
	}

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "key",
		"params": map[string]interface{}{
			"keys":    []interface{}{"ctrl", "alt", "delete"},
			"hold_ms": 900,
		},
	}); err != nil {
		t.Fatalf("key Execute() custom hold error = %v", err)
	}
	if backend.lastKeyHoldMS != 900 {
		t.Fatalf("custom lastKeyHoldMS = %d, want 900", backend.lastKeyHoldMS)
	}
}

func TestA11yToolExecute_KeyNormalizesSingleShortcutLiteralString(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "key",
		"params": map[string]interface{}{
			"keys": "command+k",
		},
	})
	if err != nil {
		t.Fatalf("key Execute() error = %v", err)
	}
	if len(backend.keyHistory) != 1 {
		t.Fatalf("keyHistory = %#v, want one forwarded key chord", backend.keyHistory)
	}
	if got := backend.keyHistory[0]; len(got) != 2 || got[0] != "cmd" || got[1] != "k" {
		t.Fatalf("keyHistory[0] = %#v, want [cmd k]", got)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if _, ok := out["message"].(string); !ok {
		t.Fatalf("message = %#v, want string", out["message"])
	}
}

func TestA11yToolExecute_KeyUsesSubmitKeysWhenKeysMissing(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "key",
		"params": map[string]interface{}{
			"submit_keys": []interface{}{"Meta+k"},
		},
	})
	if err != nil {
		t.Fatalf("key Execute() error = %v", err)
	}
	if len(backend.keyHistory) != 1 {
		t.Fatalf("keyHistory = %#v, want one forwarded submit key chord", backend.keyHistory)
	}
	if got := backend.keyHistory[0]; len(got) != 2 || got[0] != "cmd" || got[1] != "k" {
		t.Fatalf("keyHistory[0] = %#v, want [cmd k]", got)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if _, ok := out["message"].(string); !ok {
		t.Fatalf("message = %#v, want string", out["message"])
	}
}

func TestA11yToolExecute_KeyboardInputAliasUsesKeyAction(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "keyboard_input",
		"params": map[string]interface{}{
			"keys": "command+k",
		},
	})
	if err != nil {
		t.Fatalf("keyboard_input Execute() error = %v", err)
	}
	if len(backend.keyHistory) != 1 {
		t.Fatalf("keyHistory = %#v, want one forwarded key chord", backend.keyHistory)
	}
	if got := backend.keyHistory[0]; len(got) != 2 || got[0] != "cmd" || got[1] != "k" {
		t.Fatalf("keyHistory[0] = %#v, want [cmd k]", got)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if _, ok := out["message"].(string); !ok {
		t.Fatalf("message = %#v, want string", out["message"])
	}
}

func TestA11yToolExecute_KeySequenceAliasParsesShortcutLiteralValue(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "key_sequence",
		"value":  "command+k",
	})
	if err != nil {
		t.Fatalf("key_sequence Execute() error = %v", err)
	}
	if len(backend.keyHistory) != 1 {
		t.Fatalf("keyHistory = %#v, want one forwarded key chord", backend.keyHistory)
	}
	if got := backend.keyHistory[0]; len(got) != 2 || got[0] != "cmd" || got[1] != "k" {
		t.Fatalf("keyHistory[0] = %#v, want [cmd k]", got)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if _, ok := out["message"].(string); !ok {
		t.Fatalf("message = %#v, want string", out["message"])
	}
}

func TestA11yToolExecute_KeyActionPlainValueDoesNotBecomeShortcut(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "key",
		"value":  "hello",
	}); err == nil {
		t.Fatal("key Execute() error = nil, want missing-keys error")
	}
	if len(backend.keyHistory) != 0 {
		t.Fatalf("keyHistory = %#v, want no key dispatch for plain value", backend.keyHistory)
	}
}

func TestA11yToolExecute_TypeAliasWithoutValueUsesSubmitKeysAsKeyChord(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "type",
		"submit_keys": []interface{}{"command", "f"},
	})
	if err != nil {
		t.Fatalf("type Execute() error = %v", err)
	}
	if len(backend.keyHistory) != 1 {
		t.Fatalf("keyHistory = %#v, want one forwarded key chord", backend.keyHistory)
	}
	if got := backend.keyHistory[0]; len(got) != 2 || got[0] != "command" || got[1] != "f" {
		t.Fatalf("keyHistory[0] = %#v, want [command f]", got)
	}
	if len(backend.actTypeHistory) != 0 {
		t.Fatalf("actTypeHistory = %#v, want no semantic type action", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if _, ok := out["message"].(string); !ok {
		t.Fatalf("message = %#v, want string", out["message"])
	}
}

func TestA11yToolExecute_TypeAliasWithoutValueUsesKeysAsKeyChord(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "type",
		"params": map[string]interface{}{
			"keys": []interface{}{"cmd", "k"},
		},
	})
	if err != nil {
		t.Fatalf("type Execute() error = %v", err)
	}
	if len(backend.keyHistory) != 1 {
		t.Fatalf("keyHistory = %#v, want one forwarded key chord", backend.keyHistory)
	}
	if got := backend.keyHistory[0]; len(got) != 2 || got[0] != "cmd" || got[1] != "k" {
		t.Fatalf("keyHistory[0] = %#v, want [cmd k]", got)
	}
	if len(backend.actTypeHistory) != 0 {
		t.Fatalf("actTypeHistory = %#v, want no semantic type action", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if _, ok := out["message"].(string); !ok {
		t.Fatalf("message = %#v, want string", out["message"])
	}
}

func TestA11yToolExecute_TypeAliasWithValueFallsBackToFocusedTextWhenInputSelectorMisses(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-send",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "type",
		"window_title": "Feishu",
		"value":        "test_group",
	})
	if err != nil {
		t.Fatalf("type Execute() error = %v", err)
	}
	if got := len(backend.focusedTypeHistory); got != 1 {
		t.Fatalf("focusedTypeHistory len = %d, want 1", got)
	}
	if backend.lastFocusedTypeValue != "test_group" {
		t.Fatalf("lastFocusedTypeValue = %q, want test_group", backend.lastFocusedTypeValue)
	}
	if backend.lastFocusedTypeWindowID != "win-feishu" {
		t.Fatalf("lastFocusedTypeWindowID = %q, want win-feishu", backend.lastFocusedTypeWindowID)
	}
	if len(backend.actTypeHistory) != 0 {
		t.Fatalf("actTypeHistory = %#v, want no semantic act call", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != nil {
		t.Fatalf("error_code = %v, want nil after focused text fallback", out["error_code"])
	}
	if out["input_method"] != "clipboard" {
		t.Fatalf("input_method = %v, want clipboard", out["input_method"])
	}
	if out["verification_method"] != "focused_text" {
		t.Fatalf("verification_method = %v, want focused_text", out["verification_method"])
	}
}

func TestA11yToolExecute_TypeAliasWithValueFallbackStillRunsSubmitKeys(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-send",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "type",
		"window_title": "Feishu",
		"value":        "test_group",
		"submit_keys":  []interface{}{"command", "k"},
	})
	if err != nil {
		t.Fatalf("type Execute() error = %v", err)
	}
	if got := len(backend.focusedTypeHistory); got != 1 {
		t.Fatalf("focusedTypeHistory len = %d, want 1", got)
	}
	if len(backend.keyHistory) != 1 {
		t.Fatalf("keyHistory = %#v, want one submit key chord", backend.keyHistory)
	}
	if got := backend.keyHistory[0]; len(got) != 2 || got[0] != "command" || got[1] != "k" {
		t.Fatalf("keyHistory[0] = %#v, want [command k]", got)
	}
	if len(backend.actTypeHistory) != 0 {
		t.Fatalf("actTypeHistory = %#v, want no semantic act call", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != nil {
		t.Fatalf("error_code = %v, want nil after focused text fallback + submit keys", out["error_code"])
	}
}

func TestA11yToolExecute_TypeAliasWithShortcutLiteralValueUsesKeyChord(t *testing.T) {
	backend := &a11yCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "type",
		"value":  "command+k",
	})
	if err != nil {
		t.Fatalf("type Execute() error = %v", err)
	}
	if len(backend.keyHistory) != 1 {
		t.Fatalf("keyHistory = %#v, want one forwarded key chord", backend.keyHistory)
	}
	if got := backend.keyHistory[0]; len(got) != 2 || got[0] != "cmd" || got[1] != "k" {
		t.Fatalf("keyHistory[0] = %#v, want [cmd k]", got)
	}
	if len(backend.focusedTypeHistory) != 0 {
		t.Fatalf("focusedTypeHistory = %#v, want no focused text typing", backend.focusedTypeHistory)
	}
	if len(backend.actTypeHistory) != 0 {
		t.Fatalf("actTypeHistory = %#v, want no semantic type action", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if _, ok := out["message"].(string); !ok {
		t.Fatalf("message = %#v, want string", out["message"])
	}
}

func TestA11yToolExecute_TypeAliasWithPlainPlusTextDoesNotUseShortcutFallback(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-send",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "type",
		"window_title": "Feishu",
		"value":        "a+b",
	})
	if err != nil {
		t.Fatalf("type Execute() error = %v", err)
	}
	if len(backend.keyHistory) != 0 {
		t.Fatalf("keyHistory = %#v, want no key chord fallback for plain plus text", backend.keyHistory)
	}
	if got := len(backend.focusedTypeHistory); got != 1 {
		t.Fatalf("focusedTypeHistory len = %d, want 1", got)
	}
	if backend.lastFocusedTypeValue != "a+b" {
		t.Fatalf("lastFocusedTypeValue = %q, want a+b", backend.lastFocusedTypeValue)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != nil {
		t.Fatalf("error_code = %v, want nil after focused text fallback", out["error_code"])
	}
}

func TestA11yToolExecute_TypeAliasWithSingleSubmitShortcutLiteralStillSubmits(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-send",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "type",
		"window_title": "Feishu",
		"value":        "test_group",
		"submit_keys":  []interface{}{"Meta+k"},
	})
	if err != nil {
		t.Fatalf("type Execute() error = %v", err)
	}
	if got := len(backend.focusedTypeHistory); got != 1 {
		t.Fatalf("focusedTypeHistory len = %d, want 1", got)
	}
	if len(backend.keyHistory) != 1 {
		t.Fatalf("keyHistory = %#v, want one submit key chord", backend.keyHistory)
	}
	if got := backend.keyHistory[0]; len(got) != 2 || got[0] != "cmd" || got[1] != "k" {
		t.Fatalf("keyHistory[0] = %#v, want [cmd k]", got)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != nil {
		t.Fatalf("error_code = %v, want nil after focused text fallback + normalized submit chord", out["error_code"])
	}
}

func TestA11yToolExecute_TypeAliasMessageValueDoesNotUseShortcutFallback(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [list_item] \"Orca\"\n@2 [document]\n@3 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-orca-conversation",
					2: "token-editor",
					3: "token-send",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Feishu",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "type",
		"app_name":     "Feishu",
		"conversation": "Orca",
		"value":        "command+k",
	})
	if err != nil {
		t.Fatalf("type Execute() error = %v", err)
	}
	if len(backend.keyHistory) != 0 {
		t.Fatalf("keyHistory = %#v, want no key chord fallback for message text", backend.keyHistory)
	}
	if len(backend.actTypeHistory) != 2 || backend.actTypeHistory[0] != "click" || backend.actTypeHistory[1] != "type" {
		t.Fatalf("actTypeHistory = %#v, want [click type]", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["message"] != "ok" {
		t.Fatalf("message = %v, want ok", out["message"])
	}
}

func TestA11yToolExecute_TypeAliasWithExplicitInputRoleFallsBackToFocusedText(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-send",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "type",
		"window_title": "Feishu",
		"target_role":  "input",
		"value":        "test_group",
	})
	if err != nil {
		t.Fatalf("type Execute() error = %v", err)
	}
	if got := len(backend.focusedTypeHistory); got != 1 {
		t.Fatalf("focusedTypeHistory len = %d, want 1", got)
	}
	if backend.lastFocusedTypeValue != "test_group" {
		t.Fatalf("lastFocusedTypeValue = %q, want test_group", backend.lastFocusedTypeValue)
	}
	if len(backend.actTypeHistory) != 0 {
		t.Fatalf("actTypeHistory = %#v, want no semantic act call", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != nil {
		t.Fatalf("error_code = %v, want nil after explicit input-role focused fallback", out["error_code"])
	}
}

func TestA11yToolExecute_TypeAliasWithNamedInputTargetDoesNotUseFocusedTextFallback(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-send",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "type",
		"window_title": "Feishu",
		"target_role":  "input",
		"target_name":  "Composer",
		"value":        "test_group",
	})
	if err != nil {
		t.Fatalf("type Execute() error = %v", err)
	}
	if got := len(backend.focusedTypeHistory); got != 0 {
		t.Fatalf("focusedTypeHistory len = %d, want 0 for named target", got)
	}
	if len(backend.actTypeHistory) != 0 {
		t.Fatalf("actTypeHistory = %#v, want no semantic act call on unresolved named target", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "target_not_found" {
		t.Fatalf("error_code = %v, want target_not_found for named target miss", out["error_code"])
	}
}

func TestA11yToolExecute_ActTypeDoesNotUseFocusedTextAliasFallback(t *testing.T) {
	backend := &a11yCompatBackend{
		windows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "Feishu", AppName: "Feishu"},
		},
		interactiveResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Feishu",
			Tree:     "@1 [button] \"Send\"",
			RefMap: map[int]string{
				1: "token-send",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "act",
		"window_title": "Feishu",
		"params": map[string]interface{}{
			"act_type": "type",
			"value":    "test_group",
		},
	})
	if err != nil {
		t.Fatalf("act Execute() error = %v", err)
	}
	if got := len(backend.focusedTypeHistory); got != 0 {
		t.Fatalf("focusedTypeHistory len = %d, want 0 for generic act", got)
	}
	if len(backend.actTypeHistory) != 0 {
		t.Fatalf("actTypeHistory = %#v, want no semantic act call on unresolved target", backend.actTypeHistory)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "target_not_found" {
		t.Fatalf("error_code = %v, want target_not_found", out["error_code"])
	}
}

func TestA11yToolExecute_BrowserSnapshotDelegatesAndIncludesRefMap(t *testing.T) {
	host := &a11yCompatBackend{}
	browser := &a11yBrowserCompatBackend{
		a11yResult: BrowserA11yTreeResult{
			Tree:     "@1 [link] \"Example\"",
			URL:      "https://example.com",
			Title:    "Example",
			TargetID: "tab-7",
			RefMap:   map[int]int{1: 77},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(host)
	tool.SetBrowser(browser)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "snapshot",
		"surface":   "browser",
		"target_id": "tab-7",
	})
	if err != nil {
		t.Fatalf("snapshot Execute() error = %v", err)
	}
	if host.lastSnapshotWindowID != "" {
		t.Fatalf("host snapshot should not run for browser surface, got %q", host.lastSnapshotWindowID)
	}
	if browser.lastA11yTargetID != "tab-7" {
		t.Fatalf("browser lastA11yTargetID = %q, want tab-7", browser.lastA11yTargetID)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["surface"] != "browser" {
		t.Fatalf("surface = %v, want browser", out["surface"])
	}
	if out["target_id"] != "tab-7" {
		t.Fatalf("target_id = %v, want tab-7", out["target_id"])
	}
	if out["window_id"] != "tab-7" {
		t.Fatalf("window_id = %v, want tab-7", out["window_id"])
	}
	if _, ok := out["ref_map"].(map[string]interface{}); !ok {
		t.Fatalf("ref_map = %#v, want JSON object", out["ref_map"])
	}
}

func TestA11yToolExecute_BrowserActUsesCachedBrowserRefs(t *testing.T) {
	host := &a11yCompatBackend{}
	browser := &a11yBrowserCompatBackend{
		a11yResult: BrowserA11yTreeResult{
			Tree:     "@1 [button] \"Continue\"",
			URL:      "https://example.com",
			Title:    "Example",
			TargetID: "tab-7",
			RefMap:   map[int]int{1: 501},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(host)
	tool.SetBrowser(browser)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "snapshot",
		"surface":   "browser",
		"target_id": "tab-7",
	}); err != nil {
		t.Fatalf("browser snapshot error = %v", err)
	}

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":  "act",
		"surface": "browser",
		"params": map[string]interface{}{
			"ref":      "@1",
			"act_type": "click",
		},
	})
	if err != nil {
		t.Fatalf("browser act error = %v", err)
	}
	if host.actCalls != 0 {
		t.Fatalf("host actCalls = %d, want 0 for browser surface", host.actCalls)
	}
	if browser.lastActMode != "a11y" {
		t.Fatalf("browser lastActMode = %q, want a11y", browser.lastActMode)
	}
	if browser.lastActTargetID != "tab-7" {
		t.Fatalf("browser lastActTargetID = %q, want tab-7", browser.lastActTargetID)
	}
	if browser.lastActRef != 1 || browser.lastActAction != "click" {
		t.Fatalf("browser act = ref %d action %q, want ref 1 action click", browser.lastActRef, browser.lastActAction)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["surface"] != "browser" {
		t.Fatalf("surface = %v, want browser", out["surface"])
	}
}

func TestA11yToolExecute_BrowserWindowsSeedActiveTargetForSnapshot(t *testing.T) {
	browser := &a11yBrowserCompatBackend{
		tabs: []BrowserTabResult{
			{TargetID: "tab-1", URL: "https://one.example", Title: "One", Active: false},
			{TargetID: "tab-2", URL: "https://two.example", Title: "Two", Active: true},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(&a11yCompatBackend{})
	tool.SetBrowser(browser)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":  "windows",
		"surface": "browser",
	}); err != nil {
		t.Fatalf("browser windows error = %v", err)
	}

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":  "snapshot",
		"surface": "browser",
	}); err != nil {
		t.Fatalf("browser snapshot error = %v", err)
	}
	if browser.lastA11yTargetID != "tab-2" {
		t.Fatalf("browser lastA11yTargetID = %q, want seeded active tab tab-2", browser.lastA11yTargetID)
	}
}

func TestA11yToolExecute_BrowserListAliasMapsToWindows(t *testing.T) {
	backend := &a11yBrowserCompatBackend{
		tabs: []BrowserTabResult{
			{TargetID: "tab-1", URL: "https://example.com", Title: "Example", Active: true},
		},
	}
	tool := NewA11yTool()
	tool.SetBrowser(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"surface": "browser",
		"action":  "list_windows",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["surface"] != "browser" {
		t.Fatalf("surface = %v, want browser", out["surface"])
	}
	if _, ok := out["tabs"].([]interface{}); !ok {
		t.Fatalf("tabs = %#v, want JSON array", out["tabs"])
	}
}

func TestA11yToolExecute_BrowserFocusCachesTargetForFollowups(t *testing.T) {
	browser := &a11yBrowserCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(&a11yCompatBackend{})
	tool.SetBrowser(browser)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "focus",
		"surface":   "browser",
		"target_id": "tab-9",
	}); err != nil {
		t.Fatalf("browser focus error = %v", err)
	}
	if browser.lastFocusedTargetID != "tab-9" {
		t.Fatalf("lastFocusedTargetID = %q, want tab-9", browser.lastFocusedTargetID)
	}

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":  "snapshot",
		"surface": "browser",
	}); err != nil {
		t.Fatalf("browser snapshot error = %v", err)
	}
	if browser.lastA11yTargetID != "tab-9" {
		t.Fatalf("browser lastA11yTargetID = %q, want cached focused target tab-9", browser.lastA11yTargetID)
	}
}

func TestA11yToolExecute_BrowserScrollUsesPageScrollCompat(t *testing.T) {
	browser := &a11yBrowserCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(&a11yCompatBackend{})
	tool.SetBrowser(browser)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "scroll",
		"surface":   "browser",
		"target_id": "tab-4",
		"params": map[string]interface{}{
			"direction": "down",
		},
	})
	if err != nil {
		t.Fatalf("browser scroll error = %v", err)
	}
	if browser.lastPageScrollTargetID != "tab-4" {
		t.Fatalf("lastPageScrollTargetID = %q, want tab-4", browser.lastPageScrollTargetID)
	}
	if browser.lastPageScrollY <= 0 {
		t.Fatalf("lastPageScrollY = %d, want > 0", browser.lastPageScrollY)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["surface"] != "browser" {
		t.Fatalf("surface = %v, want browser", out["surface"])
	}
}

func TestA11yToolExecute_BrowserScreenshotUsesTargetAlias(t *testing.T) {
	browser := &a11yBrowserCompatBackend{screenshotTabData: "browser-shot-tab"}
	tool := NewA11yTool()
	tool.SetBackend(&a11yCompatBackend{})
	tool.SetBrowser(browser)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "screenshot",
		"surface":   "browser",
		"window_id": "tab-12",
	})
	if err != nil {
		t.Fatalf("browser screenshot error = %v", err)
	}
	if browser.lastScreenshotTabTarget != "tab-12" {
		t.Fatalf("lastScreenshotTabTarget = %q, want tab-12", browser.lastScreenshotTabTarget)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["surface"] != "browser" {
		t.Fatalf("surface = %v, want browser", out["surface"])
	}
	if out["window_id"] != "tab-12" {
		t.Fatalf("window_id = %v, want tab-12", out["window_id"])
	}
}

func TestA11yToolExecute_BrowserKeyUsesKeyCompat(t *testing.T) {
	browser := &a11yBrowserCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(&a11yCompatBackend{})
	tool.SetBrowser(browser)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "key",
		"surface":   "browser",
		"target_id": "tab-15",
		"params": map[string]interface{}{
			"keys":    []interface{}{"enter"},
			"hold_ms": 250,
		},
	})
	if err != nil {
		t.Fatalf("browser key error = %v", err)
	}
	if browser.lastPressedTargetID != "tab-15" {
		t.Fatalf("lastPressedTargetID = %q, want tab-15", browser.lastPressedTargetID)
	}
	if len(browser.lastPressedKeys) != 1 || browser.lastPressedKeys[0] != "enter" {
		t.Fatalf("lastPressedKeys = %#v, want [enter]", browser.lastPressedKeys)
	}
	if browser.lastPressedHoldMS != 250 {
		t.Fatalf("lastPressedHoldMS = %d, want 250", browser.lastPressedHoldMS)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["surface"] != "browser" {
		t.Fatalf("surface = %v, want browser", out["surface"])
	}
}

func TestA11yToolExecute_BrowserKeySupportsSingleChordString(t *testing.T) {
	browser := &a11yBrowserCompatBackend{}
	tool := NewA11yTool()
	tool.SetBackend(&a11yCompatBackend{})
	tool.SetBrowser(browser)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "key",
		"surface":   "browser",
		"target_id": "tab-16",
		"params": map[string]interface{}{
			"key": "ctrl+c",
		},
	})
	if err != nil {
		t.Fatalf("browser key error = %v", err)
	}
	if browser.lastPressedTargetID != "tab-16" {
		t.Fatalf("lastPressedTargetID = %q, want tab-16", browser.lastPressedTargetID)
	}
	if len(browser.lastPressedKeys) != 1 || browser.lastPressedKeys[0] != "ctrl+c" {
		t.Fatalf("lastPressedKeys = %#v, want [ctrl+c]", browser.lastPressedKeys)
	}
	if browser.lastPressedHoldMS != a11yruntime.DefaultHoldMS {
		t.Fatalf("lastPressedHoldMS = %d, want default %d", browser.lastPressedHoldMS, a11yruntime.DefaultHoldMS)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["surface"] != "browser" {
		t.Fatalf("surface = %v, want browser", out["surface"])
	}
}

func TestA11yToolExecute_FocusUsesAllWindowsFallbackWhenWindowIsOnAnotherSpace(t *testing.T) {
	backend := &a11yCompatBackend{
		windowsResults: [][]a11yruntime.WindowInfo{
			{},
		},
		allWindows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "飞书", AppName: "飞书"},
		},
		focusResultWindowID: "win-feishu",
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "focus",
		"app_name": "Feishu,飞书,Lark",
	})
	if err != nil {
		t.Fatalf("focus Execute() error = %v", err)
	}
	if backend.lastFocusWindowID != "win-feishu" {
		t.Fatalf("lastFocusWindowID = %q, want win-feishu", backend.lastFocusWindowID)
	}
	if backend.listWindowsCalls != 1 {
		t.Fatalf("listWindowsCalls = %d, want 1", backend.listWindowsCalls)
	}
	if backend.listAllWindowsCalls != 1 {
		t.Fatalf("listAllWindowsCalls = %d, want 1", backend.listAllWindowsCalls)
	}
	if len(backend.activateAppCalls) != 0 {
		t.Fatalf("activateAppCalls = %v, want no app activation fallback", backend.activateAppCalls)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["window_id"] != "win-feishu" {
		t.Fatalf("window_id = %v, want win-feishu", out["window_id"])
	}
}

func TestA11yToolExecute_FocusActivatesAppWhenAllWindowsFallbackIsAmbiguous(t *testing.T) {
	backend := &a11yCompatBackend{
		windowsResults: [][]a11yruntime.WindowInfo{
			{},
			{
				{ID: "win-feishu", Title: "Lark - Orca", AppName: "Lark", Focused: true},
			},
		},
		allWindows: []a11yruntime.WindowInfo{
			{ID: "win-feishu-hidden-1", Title: "Lark - Team Alpha", AppName: "Lark"},
			{ID: "win-feishu-hidden-2", Title: "Lark - Team Beta", AppName: "Lark"},
		},
		focusResultWindowID: "win-feishu",
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "focus",
		"app_name": "Feishu,飞书,Lark",
	})
	if err != nil {
		t.Fatalf("focus Execute() error = %v", err)
	}
	if len(backend.activateAppCalls) == 0 {
		t.Fatal("activateAppCalls = 0, want activation retry after all-windows ambiguity")
	}
	if backend.lastFocusWindowID != "win-feishu" {
		t.Fatalf("lastFocusWindowID = %q, want win-feishu", backend.lastFocusWindowID)
	}
	if backend.listAllWindowsCalls != 1 {
		t.Fatalf("listAllWindowsCalls = %d, want 1", backend.listAllWindowsCalls)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["window_id"] != "win-feishu" {
		t.Fatalf("window_id = %v, want win-feishu", out["window_id"])
	}
}

func TestA11yToolExecute_ActionSelectActivatesAppWhenWindowIsNotYetVisible(t *testing.T) {
	backend := &a11yCompatBackend{
		windowsResults: [][]a11yruntime.WindowInfo{
			{},
			{
				{ID: "win-feishu", Title: "Lark - Orca", AppName: "Lark", Focused: true},
			},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Lark - Orca",
				Tree:     "@1 [list_item] \"Orca\"",
				RefMap: map[int]string{
					1: "token-orca-conversation",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "Lark - Orca",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "select",
		"app_name":     "Feishu,飞书,Lark",
		"conversation": "Orca",
	}); err != nil {
		t.Fatalf("select Execute() error = %v", err)
	}
	if len(backend.activateAppCalls) == 0 {
		t.Fatal("activateAppCalls = 0, want activation fallback")
	}
	if backend.lastActWindowID != "win-feishu" {
		t.Fatalf("lastActWindowID = %q, want win-feishu", backend.lastActWindowID)
	}
	if backend.lastActRef != 1 {
		t.Fatalf("lastActRef = %d, want 1", backend.lastActRef)
	}
}

func TestA11yToolExecute_ActionSelectUsesAllWindowsFallbackWhenWindowIsOnAnotherSpace(t *testing.T) {
	backend := &a11yCompatBackend{
		windowsResults: [][]a11yruntime.WindowInfo{
			{},
		},
		allWindows: []a11yruntime.WindowInfo{
			{ID: "win-feishu", Title: "飞书", AppName: "飞书"},
		},
		interactiveResults: []a11yruntime.SnapshotResult{
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "飞书",
				Tree:     "@1 [list_item] \"Orca\"",
				RefMap: map[int]string{
					1: "token-orca-conversation",
				},
			},
			{
				HostOS:   "darwin",
				WindowID: "win-feishu",
				Title:    "飞书",
				Tree:     "@1 [document]\n@2 [button] \"Send\"",
				RefMap: map[int]string{
					1: "token-editor",
					2: "token-send",
				},
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "select",
		"app_name":     "Feishu,飞书,Lark",
		"conversation": "Orca",
	}); err != nil {
		t.Fatalf("select Execute() error = %v", err)
	}
	if backend.lastActWindowID != "win-feishu" {
		t.Fatalf("lastActWindowID = %q, want win-feishu", backend.lastActWindowID)
	}
	if backend.lastActRef != 1 {
		t.Fatalf("lastActRef = %d, want 1", backend.lastActRef)
	}
	if backend.listAllWindowsCalls != 1 {
		t.Fatalf("listAllWindowsCalls = %d, want 1", backend.listAllWindowsCalls)
	}
}

func TestA11yToolExecute_SnapshotActivatesAppWhenWindowIsNotYetVisible(t *testing.T) {
	backend := &a11yCompatBackend{
		windowsResults: [][]a11yruntime.WindowInfo{
			{},
			{
				{ID: "win-feishu", Title: "Lark - Orca", AppName: "Lark", Focused: true},
			},
		},
		snapshotResult: a11yruntime.SnapshotResult{
			HostOS:   "darwin",
			WindowID: "win-feishu",
			Title:    "Lark - Orca",
			Tree:     "@1 [document]",
			RefMap: map[int]string{
				1: "token-editor",
			},
		},
	}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "snapshot",
		"app_name": "Feishu,飞书,Lark",
	})
	if err != nil {
		t.Fatalf("snapshot Execute() error = %v", err)
	}
	if len(backend.activateAppCalls) == 0 {
		t.Fatal("activateAppCalls = 0, want activation fallback")
	}
	if backend.lastSnapshotWindowID != "win-feishu" {
		t.Fatalf("lastSnapshotWindowID = %q, want win-feishu", backend.lastSnapshotWindowID)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["window_id"] != "win-feishu" {
		t.Fatalf("window_id = %v, want win-feishu", out["window_id"])
	}
}

func TestA11yToolExecute_KeyFailsClosedWhenHostKeyInjectionHangs(t *testing.T) {
	prevTimeout := a11yHostInputActionTimeout
	a11yHostInputActionTimeout = 10 * time.Millisecond
	defer func() { a11yHostInputActionTimeout = prevTimeout }()

	blockCh := make(chan struct{})
	defer close(blockCh)

	backend := &a11yCompatBackend{keyBlockCh: blockCh}
	tool := NewA11yTool()
	tool.SetBackend(backend)

	raw, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":    "key",
		"window_id": "win-1",
		"keys":      []interface{}{"down"},
	})
	if err != nil {
		t.Fatalf("key Execute() error = %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.(string)), &out); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if out["error_code"] != "backend_timeout" {
		t.Fatalf("error_code = %v, want backend_timeout", out["error_code"])
	}
	if out["action"] != "key" {
		t.Fatalf("action = %v, want key", out["action"])
	}
}

func TestIsA11yActionHighRisk(t *testing.T) {
	cases := []struct {
		actType  string
		expected bool
	}{
		{actType: "click", expected: true},
		{actType: "type", expected: true},
		{actType: "toggle", expected: true},
		{actType: "submit", expected: true},
		{actType: "long_press", expected: true},
		{actType: "focus", expected: false},
		{actType: "scroll", expected: false},
	}

	for _, tc := range cases {
		if got := IsA11yActionHighRisk("act", tc.actType); got != tc.expected {
			t.Fatalf("IsA11yActionHighRisk(act,%q)=%v want %v", tc.actType, got, tc.expected)
		}
	}
}
