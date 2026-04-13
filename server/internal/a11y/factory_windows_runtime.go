//go:build windows

package a11y

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows"
)

const (
	windowsOBJIDWindow = 0
	windowsCHILDIDSelf = 0

	windowsInputMouse    = 0
	windowsInputKeyboard = 1

	windowsMouseEventMove      = 0x0001
	windowsMouseEventLeftDown  = 0x0002
	windowsMouseEventLeftUp    = 0x0004
	windowsMouseEventRightDown = 0x0008
	windowsMouseEventRightUp   = 0x0010
	windowsMouseEventWheel     = 0x0800
	windowsMouseEventHWheel    = 0x01000

	windowsKeyEventKeyUp   = 0x0002
	windowsKeyEventUnicode = 0x0004

	windowsWheelDelta    = 120
	windowsShowRestore   = 9
	windowsSnapshotDepth = 6
	windowsSnapshotNodes = 256
)

var (
	windowsIIDIAccessible = ole.NewGUID("{618736E0-3C3D-11CF-810C-00AA00389B71}")

	user32DLL                      = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows                = user32DLL.NewProc("EnumWindows")
	procGetWindowTextLengthW       = user32DLL.NewProc("GetWindowTextLengthW")
	procGetWindowTextW             = user32DLL.NewProc("GetWindowTextW")
	procIsWindowVisible            = user32DLL.NewProc("IsWindowVisible")
	procGetWindowThreadProcessID   = user32DLL.NewProc("GetWindowThreadProcessId")
	procGetForegroundWindow        = user32DLL.NewProc("GetForegroundWindow")
	procSetForegroundWindow        = user32DLL.NewProc("SetForegroundWindow")
	procShowWindow                 = user32DLL.NewProc("ShowWindow")
	procSetCursorPos               = user32DLL.NewProc("SetCursorPos")
	procSendInput                  = user32DLL.NewProc("SendInput")
	procGetWindowRect              = user32DLL.NewProc("GetWindowRect")
	oleaccDLL                      = windows.NewLazySystemDLL("oleacc.dll")
	procAccessibleObjectFromWindow = oleaccDLL.NewProc("AccessibleObjectFromWindow")
	procAccessibleChildren         = oleaccDLL.NewProc("AccessibleChildren")
	procWindowFromAccessibleObject = oleaccDLL.NewProc("WindowFromAccessibleObject")
	procGetRoleTextW               = oleaccDLL.NewProc("GetRoleTextW")
	procGetStateTextW              = oleaccDLL.NewProc("GetStateTextW")

	windowsResolveWindowFunc       = windowsResolveWindow
	windowsSnapshotMSAAFunc        = windowsSnapshotMSAA
	windowsGetRectFunc             = windowsGetRect
	windowsCaptureWindowFunc       = windowsCaptureWindowImage
	windowsCaptureActiveWindowFunc = windowsCaptureActiveWindowImage
	windowsPasteTextFunc           = windowsPasteTextInput
)

func init() {
	windowsCurrentForegroundHWNDFunc = func() uintptr {
		hwnd, _, _ := procGetForegroundWindow.Call()
		return hwnd
	}
}

type windowsAccessibleChild struct {
	Dispatch *ole.IDispatch
	ChildID  int32
}

type windowsAccessibleTarget struct {
	Dispatch *ole.IDispatch
	ChildID  int32
	HWND     uintptr
}

type windowsMouseInput struct {
	DX          int32
	DY          int32
	MouseData   uint32
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

type windowsKeyboardInput struct {
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

type windowsInput struct {
	Type uint32
	_    uint32
	Data [32]byte
}

func (b *windowsBackend) ListWindows(context.Context) ([]WindowInfo, error) {
	return windowsEnumerateVisibleWindows()
}

func (b *windowsBackend) FocusWindow(_ context.Context, windowID string) (ActionResult, error) {
	return windowsFocusResultWithResolver(b.HostOS(), windowID, windowsResolveWindowFunc, windowsBringWindowToFront)
}

func (b *windowsBackend) Snapshot(ctx context.Context, windowID string) (SnapshotResult, error) {
	return b.snapshot(ctx, windowID, false)
}

func (b *windowsBackend) SnapshotInteractive(ctx context.Context, windowID string) (SnapshotResult, error) {
	return b.snapshot(ctx, windowID, true)
}

func (b *windowsBackend) snapshot(ctx context.Context, windowID string, interactiveOnly bool) (SnapshotResult, error) {
	return windowsSnapshotWithImageFallback(
		ctx,
		b.HostOS(),
		windowID,
		interactiveOnly,
		windowsResolveWindowFunc,
		windowsSnapshotMSAAFunc,
		b.Screenshot,
	)
}

func (b *windowsBackend) Act(_ context.Context, windowID string, ref int, refMap map[int]string, actType string, value string, holdMS int) (ActionResult, error) {
	hwnd, path, err := windowsResolveActionTargetFromRef(ref, refMap, windowID, windowsParseSnapshotToken, windowsParseHWND)
	if err != nil {
		return ActionResult{HostOS: b.HostOS()}, err
	}
	var actionResult ActionResult
	err = windowsWithResolvedTarget(hwnd, path, func(target windowsAccessibleTarget) error {
		meta := windowsReadActionMetadata(target)
		plan := planWindowsAction(actType, meta)
		result, err := windowsActionResultWithPlan(
			b.HostOS(),
			strconv.FormatUint(uint64(hwnd), 10),
			actType,
			value,
			holdMS,
			plan,
			func(plan windowsActionPlan, value string) error {
				return windowsExecutePrimaryAction(target, plan, value)
			},
			func(fallback string, value string, holdMS int) error {
				return windowsExecuteFallback(target, fallback, value, holdMS)
			},
		)
		if err != nil {
			return err
		}
		actionResult = result
		return nil
	})
	if err != nil {
		return ActionResult{HostOS: b.HostOS()}, err
	}
	return actionResult, nil
}

func (b *windowsBackend) Scroll(_ context.Context, windowID string, direction string, lines int) (ActionResult, error) {
	hwnd, _, err := windowsResolveWindowFunc(windowID)
	if err != nil {
		return ActionResult{HostOS: b.HostOS()}, err
	}
	return windowsScrollResultWithInput(
		b.HostOS(),
		strconv.FormatUint(uint64(hwnd), 10),
		direction,
		lines,
		func() {
			windowsBringWindowToFront(hwnd)
		},
		func(horizontal bool, delta int32) error {
			flag := uint32(windowsMouseEventWheel)
			if horizontal {
				flag = windowsMouseEventHWheel
			}
			return windowsSendMouseInput(flag, uint32(delta))
		},
	)
}

func (b *windowsBackend) PointerMove(_ context.Context, x int, y int) (ActionResult, error) {
	return windowsPointerMoveResult(b.HostOS(), x, y, func(x int, y int) error {
		if _, _, callErr := procSetCursorPos.Call(uintptr(x), uintptr(y)); callErr != syscall.Errno(0) {
			return fmt.Errorf("SetCursorPos failed: %w", callErr)
		}
		return nil
	})
}

func (b *windowsBackend) Key(_ context.Context, windowID string, keys []string, holdMS int) (ActionResult, error) {
	hwnd, _, err := windowsResolveWindowFunc(windowID)
	if err != nil {
		return ActionResult{HostOS: b.HostOS()}, err
	}
	return windowsKeyResultWithInput(
		b.HostOS(),
		strconv.FormatUint(uint64(hwnd), 10),
		keys,
		holdMS,
		func() {
			windowsBringWindowToFront(hwnd)
		},
		windowsSendKeys,
	)
}

func (b *windowsBackend) Screenshot(_ context.Context, windowID string) (ScreenshotResult, error) {
	hwnd, _, err := windowsResolveWindowFunc(windowID)
	if err != nil {
		return ScreenshotResult{HostOS: b.HostOS()}, err
	}
	rect, err := windowsGetRectFunc(hwnd)
	dir := filepath.Join(os.TempDir(), "zimaos-blue", "a11y")
	if strings.TrimSpace(b.mediaDir) != "" {
		dir = filepath.Join(b.mediaDir, "a11y")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return ScreenshotResult{HostOS: b.HostOS()}, err
	}
	path := filepath.Join(dir, fmt.Sprintf("host-window-%d.png", hwnd))
	preferredCapture := func() error {
		if err != nil {
			return err
		}
		width, height, ok := windowsVisibleRectSize(rect)
		if !ok {
			return NewError("backend_unavailable", "target window has no visible bounds", nil)
		}
		if output, captureErr := windowsCaptureWindowFunc(width, height, rect.Left, rect.Top, path); captureErr != nil {
			return fmt.Errorf("powershell screenshot failed: %s: %w", output, captureErr)
		}
		return nil
	}
	activeWindowCapture := func() error {
		windowsBringWindowToFront(hwnd)
		time.Sleep(120 * time.Millisecond)
		output, fallbackErr := windowsCaptureActiveWindowFunc(path)
		if fallbackErr != nil {
			return fmt.Errorf("active window screenshot failed: %s: %w", output, fallbackErr)
		}
		return nil
	}
	return windowsScreenshotResultWithFallback(b.HostOS(), hwnd, path, preferredCapture, activeWindowCapture)
}

func windowsSnapshotMSAA(hostOS string, hwnd uintptr, info WindowInfo, interactiveOnly bool) (SnapshotResult, error) {
	var result SnapshotResult
	err := windowsWithCOM(func() error {
		root, err := windowsAccessibleObjectFromWindow(hwnd)
		if err != nil {
			return err
		}
		defer root.Release()

		visited := 0
		node := windowsBuildSnapshotNode(root, windowsCHILDIDSelf, hwnd, nil, 0, &visited)
		if node == nil {
			return NewError("backend_unavailable", "MSAA snapshot is empty", map[string]interface{}{"window_id": info.ID})
		}
		tree, refMap := BuildSnapshotTree(node, interactiveOnly)
		result = SnapshotResult{
			HostOS:   hostOS,
			WindowID: info.ID,
			Title:    info.Title,
			Tree:     tree,
			RefMap:   refMap,
			Message:  "Host accessibility snapshot ready",
		}
		return nil
	})
	if err != nil {
		return SnapshotResult{HostOS: hostOS}, err
	}
	return result, nil
}

func windowsCaptureWindowImage(width int, height int, left int32, top int32, path string) (string, error) {
	return windowsCLIFallback.captureWindow(nil, width, height, left, top, path)
}

func windowsCaptureActiveWindowImage(path string) (string, error) {
	return windowsCLIFallback.captureActiveWindow(nil, path)
}

func windowsPasteTextInput(text string) error {
	output, err := windowsCLIFallback.pasteTextWithTemporaryClipboard(nil, text)
	if err != nil {
		return fmt.Errorf("powershell clipboard paste failed: %s: %w", output, err)
	}
	return nil
}

func windowsEnumerateVisibleWindows() ([]WindowInfo, error) {
	foreground, _, _ := procGetForegroundWindow.Call()
	windowsList := make([]WindowInfo, 0, 32)
	callback := syscall.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		visible, _, _ := procIsWindowVisible.Call(hwnd)
		title := windowsGetWindowText(hwnd)
		if !windowsShouldIncludeEnumeratedWindow(visible != 0, title) {
			return 1
		}
		pid := windowsGetWindowPID(hwnd)
		windowsList = append(windowsList, windowsWindowInfoFromEnumeration(hwnd, foreground, title, windowsProcessName(pid), pid))
		return 1
	})
	if hr, _, callErr := procEnumWindows.Call(callback, 0); hr == 0 {
		return nil, fmt.Errorf("EnumWindows failed: %w", callErr)
	}
	return windowsList, nil
}

func windowsResolveWindow(windowID string) (uintptr, WindowInfo, error) {
	list, err := windowsEnumerateVisibleWindows()
	if err != nil {
		return 0, WindowInfo{}, err
	}
	return windowsResolveWindowFromList(windowID, list)
}

func windowsAccessibleObjectFromWindow(hwnd uintptr) (*ole.IDispatch, error) {
	var unknown *ole.IUnknown
	hr, _, _ := procAccessibleObjectFromWindow.Call(hwnd, uintptr(windowsOBJIDWindow), uintptr(unsafe.Pointer(windowsIIDIAccessible)), uintptr(unsafe.Pointer(&unknown)))
	if windowsHRFailed(hr) || unknown == nil {
		return nil, fmt.Errorf("AccessibleObjectFromWindow failed: 0x%08X", uint32(hr))
	}
	defer unknown.Release()
	dispatch, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, fmt.Errorf("QueryInterface(IDispatch) failed: %w", err)
	}
	return dispatch, nil
}

func windowsBuildSnapshotNode(dispatch *ole.IDispatch, childID int32, hwnd uintptr, path []int, depth int, visited *int) *Node {
	if dispatch == nil || visited == nil || *visited >= windowsSnapshotNodes || depth > windowsSnapshotDepth {
		return nil
	}
	*visited = *visited + 1

	roleText, stateText, defaultAction, bounds, valueWritable := windowsReadNodeFields(dispatch, childID)
	name := windowsAccessibleString(dispatch, "accName", childID)
	value := windowsAccessibleString(dispatch, "accValue", childID)
	role := windowsNormalizeRole(roleText)
	meta := windowsActionMetadataFromFields(roleText, stateText, defaultAction, windowsRectHasArea(bounds), valueWritable)
	interactive := windowsNodeInteractive(meta)
	node := &Node{
		Role:          role,
		Name:          name,
		Value:         value,
		Description:   stateText,
		DefaultAction: defaultAction,
		Interactive:   interactive,
	}
	if interactive || strings.TrimSpace(defaultAction) != "" {
		node.Token = windowsSnapshotToken(hwnd, path)
	}
	if childID != windowsCHILDIDSelf {
		return node
	}
	childCount := windowsAccessibleChildCount(dispatch)
	if childCount <= 0 {
		return node
	}
	children, err := windowsAccessibleChildren(dispatch, childCount)
	if err != nil {
		return node
	}
	for idx, child := range children {
		childPath := append(append([]int(nil), path...), idx)
		if child.Dispatch != nil {
			childNode := windowsBuildSnapshotNode(child.Dispatch, windowsCHILDIDSelf, hwnd, childPath, depth+1, visited)
			child.Dispatch.Release()
			if childNode != nil {
				node.Children = append(node.Children, childNode)
			}
			continue
		}
		childNode := windowsBuildSnapshotNode(dispatch, child.ChildID, hwnd, childPath, depth+1, visited)
		if childNode != nil {
			node.Children = append(node.Children, childNode)
		}
	}
	return node
}

func windowsReadActionMetadata(target windowsAccessibleTarget) windowsActionMetadata {
	roleText, stateText, defaultAction, bounds, valueWritable := windowsReadNodeFields(target.Dispatch, target.ChildID)
	return windowsActionMetadataFromFields(roleText, stateText, defaultAction, windowsRectHasArea(bounds), valueWritable)
}

func windowsReadNodeFields(dispatch *ole.IDispatch, childID int32) (string, string, string, windowsRect, bool) {
	roleText := windowsRoleFromProperty(dispatch, childID)
	stateText := windowsStateFromProperty(dispatch, childID)
	defaultAction := windowsAccessibleString(dispatch, "accDefaultAction", childID)
	bounds, _ := windowsAccessibleLocation(dispatch, childID)
	valueWritable := windowsLikelyValueWritable(roleText)
	return roleText, stateText, defaultAction, bounds, valueWritable
}

func windowsRoleFromProperty(dispatch *ole.IDispatch, childID int32) string {
	value, ok := windowsAccessibleVariantProperty(dispatch, "accRole", childID)
	if !ok {
		return ""
	}
	defer value.Clear()
	return windowsVariantTextValue(value, windowsRoleText, func(v *ole.VARIANT) string {
		return v.ToString()
	})
}

func windowsStateFromProperty(dispatch *ole.IDispatch, childID int32) string {
	value, ok := windowsAccessibleVariantProperty(dispatch, "accState", childID)
	if !ok {
		return ""
	}
	defer value.Clear()
	return windowsVariantTextValue(value, windowsStateText, func(v *ole.VARIANT) string {
		return v.ToString()
	})
}

func windowsAccessibleString(dispatch *ole.IDispatch, property string, childID int32) string {
	value, ok := windowsAccessibleVariantProperty(dispatch, property, childID)
	if !ok {
		return ""
	}
	defer value.Clear()
	if value.VT == ole.VT_BSTR {
		return strings.TrimSpace(value.ToString())
	}
	if raw := value.Value(); raw != nil {
		return strings.TrimSpace(fmt.Sprint(raw))
	}
	return ""
}

func windowsAccessibleVariantProperty(dispatch *ole.IDispatch, property string, childID int32) (*ole.VARIANT, bool) {
	if dispatch == nil {
		return nil, false
	}
	child := ole.NewVariant(ole.VT_I4, int64(childID))
	value, err := oleutil.GetProperty(dispatch, property, child)
	if err != nil {
		return nil, false
	}
	return value, true
}

func windowsAccessibleChildCount(dispatch *ole.IDispatch) int32 {
	if dispatch == nil {
		return 0
	}
	value, err := oleutil.GetProperty(dispatch, "accChildCount")
	if err != nil || value == nil {
		return 0
	}
	defer value.Clear()
	if count, ok := windowsVariantInt32Value(value); ok {
		return count
	}
	return 0
}

func windowsAccessibleChildren(dispatch *ole.IDispatch, childCount int32) ([]windowsAccessibleChild, error) {
	if dispatch == nil || childCount <= 0 {
		return nil, nil
	}
	variants := make([]ole.VARIANT, childCount)
	var obtained int32
	hr, _, _ := procAccessibleChildren.Call(
		uintptr(unsafe.Pointer(dispatch)),
		0,
		uintptr(childCount),
		uintptr(unsafe.Pointer(&variants[0])),
		uintptr(unsafe.Pointer(&obtained)),
	)
	if windowsHRFailed(hr) {
		return nil, fmt.Errorf("AccessibleChildren failed: 0x%08X", uint32(hr))
	}
	children := make([]windowsAccessibleChild, 0, obtained)
	for idx := 0; idx < int(obtained); idx++ {
		item := variants[idx]
		switch item.VT {
		case ole.VT_DISPATCH:
			child := item.ToIDispatch()
			if child != nil {
				child.AddRef()
				_ = item.Clear()
				children = append(children, windowsAccessibleChild{Dispatch: child, ChildID: windowsCHILDIDSelf})
			}
		case ole.VT_I4:
			children = append(children, windowsAccessibleChild{ChildID: int32(item.Val)})
			_ = item.Clear()
		default:
			_ = item.Clear()
		}
	}
	return children, nil
}

func windowsAccessibleLocation(dispatch *ole.IDispatch, childID int32) (windowsRect, bool) {
	if dispatch == nil {
		return windowsRect{}, false
	}
	var left, top, width, height int32
	child := ole.NewVariant(ole.VT_I4, int64(childID))
	result, err := oleutil.CallMethod(dispatch, "accLocation", &left, &top, &width, &height, child)
	if result != nil {
		result.Clear()
	}
	if err != nil {
		return windowsRect{}, false
	}
	return windowsRect{Left: left, Top: top, Right: left + width, Bottom: top + height}, true
}

func windowsExecutePrimaryAction(target windowsAccessibleTarget, plan windowsActionPlan, value string) error {
	switch plan.Primary {
	case windowsActionDefaultAction:
		child := ole.NewVariant(ole.VT_I4, int64(target.ChildID))
		result, err := oleutil.CallMethod(target.Dispatch, "accDoDefaultAction", child)
		if result != nil {
			result.Clear()
		}
		return err
	case windowsActionSelectFocus, windowsActionSelectSelection:
		child := ole.NewVariant(ole.VT_I4, int64(target.ChildID))
		result, err := oleutil.CallMethod(target.Dispatch, "accSelect", int32(plan.SelectFlags), child)
		if result != nil {
			result.Clear()
		}
		return err
	case windowsActionPutValue:
		child := ole.NewVariant(ole.VT_I4, int64(target.ChildID))
		result, err := oleutil.CallMethod(target.Dispatch, "put_accValue", child, value)
		if result != nil {
			result.Clear()
		}
		return err
	default:
		return fmt.Errorf("unsupported primary action %q", plan.Primary)
	}
}

func windowsExecuteFallback(target windowsAccessibleTarget, fallback string, value string, holdMS int) error {
	bounds, hasBounds := windowsAccessibleLocation(target.Dispatch, target.ChildID)
	derivedTarget := windowsFallbackTargetFromRect(target.HWND, bounds)
	derivedTarget.HasBounds = hasBounds && derivedTarget.HasBounds
	return windowsExecuteFallbackWithInput(
		derivedTarget,
		fallback,
		value,
		holdMS,
		windowsFallbackExecutor{
			BringFront: windowsBringWindowToFront,
			Click: func(x int, y int, holdMS int) error {
				return windowsClickAt(x, y, false, false, holdMS)
			},
			DoubleClick: windowsDoubleClickAt,
			RightClick:  windowsRightClickAt,
			LongPress:   windowsLongPressAt,
			AfterFocus: func() {
				time.Sleep(50 * time.Millisecond)
			},
			SendText: func(value string) error {
				return windowsSendTextWithClipboardFallback(value, windowsPasteTextFunc, windowsSendUnicodeText)
			},
		},
	)
}

func windowsWithResolvedTarget(hwnd uintptr, path []int, fn func(target windowsAccessibleTarget) error) error {
	return windowsWithCOM(func() error {
		root, err := windowsAccessibleObjectFromWindow(hwnd)
		if err != nil {
			return err
		}
		defer root.Release()

		current := root
		releaseChain := make([]*ole.IDispatch, 0, len(path))
		childID := int32(windowsCHILDIDSelf)
		for idx, step := range path {
			children, err := windowsAccessibleChildren(current, windowsAccessibleChildCount(current))
			if err != nil {
				for i := len(releaseChain) - 1; i >= 0; i-- {
					releaseChain[i].Release()
				}
				return err
			}
			if step < 0 || step >= len(children) {
				windowsReleaseChildren(children, -1)
				for i := len(releaseChain) - 1; i >= 0; i-- {
					releaseChain[i].Release()
				}
				return NewError("stale_ref", "snapshot path is no longer valid; take a new snapshot first", nil)
			}
			child := children[step]
			windowsReleaseChildren(children, step)
			childID = child.ChildID
			if child.Dispatch != nil {
				current = child.Dispatch
				releaseChain = append(releaseChain, child.Dispatch)
				childID = windowsCHILDIDSelf
				continue
			}
			if idx != len(path)-1 {
				for i := len(releaseChain) - 1; i >= 0; i-- {
					releaseChain[i].Release()
				}
				return NewError("stale_ref", "snapshot path is no longer valid; take a new snapshot first", nil)
			}
		}
		defer func() {
			for i := len(releaseChain) - 1; i >= 0; i-- {
				releaseChain[i].Release()
			}
		}()
		if resolved := windowsHWNDFromAccessibleObject(current); resolved != 0 {
			hwnd = resolved
		}
		return fn(windowsAccessibleTarget{Dispatch: current, ChildID: childID, HWND: hwnd})
	})
}

func windowsRoleText(role uint32) string {
	length, _, _ := procGetRoleTextW.Call(uintptr(role), 0, 0)
	if length == 0 {
		return ""
	}
	buf := make([]uint16, length+1)
	procGetRoleTextW.Call(uintptr(role), uintptr(unsafe.Pointer(&buf[0])), uintptr(length+1))
	return strings.TrimSpace(windows.UTF16ToString(buf))
}

func windowsHWNDFromAccessibleObject(dispatch *ole.IDispatch) uintptr {
	if dispatch == nil {
		return 0
	}
	var hwnd uintptr
	hr, _, _ := procWindowFromAccessibleObject.Call(uintptr(unsafe.Pointer(dispatch)), uintptr(unsafe.Pointer(&hwnd)))
	if windowsHRFailed(hr) {
		return 0
	}
	return hwnd
}

func windowsStateText(mask uint32) string {
	return windowsStateTextFromMask(mask, func(flag uint32) string {
		length, _, _ := procGetStateTextW.Call(uintptr(flag), 0, 0)
		if length == 0 {
			return ""
		}
		buf := make([]uint16, length+1)
		procGetStateTextW.Call(uintptr(flag), uintptr(unsafe.Pointer(&buf[0])), uintptr(length+1))
		return strings.TrimSpace(windows.UTF16ToString(buf))
	})
}

func windowsBringWindowToFront(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	procShowWindow.Call(hwnd, windowsShowRestore)
	procSetForegroundWindow.Call(hwnd)
}

func windowsGetWindowText(hwnd uintptr) string {
	length, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if length == 0 {
		return ""
	}
	buf := make([]uint16, length+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), length+1)
	return strings.TrimSpace(windows.UTF16ToString(buf))
}

func windowsGetWindowPID(hwnd uintptr) uint32 {
	var pid uint32
	procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return pid
}

func windowsProcessName(pid uint32) string {
	if pid == 0 {
		return ""
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)
	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(handle, 0, &buf[0], &size); err != nil {
		return ""
	}
	name := filepath.Base(windows.UTF16ToString(buf[:size]))
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func windowsGetRect(hwnd uintptr) (windowsRect, error) {
	var rect windowsRect
	hr, _, callErr := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	if hr == 0 {
		return windowsRect{}, fmt.Errorf("GetWindowRect failed: %w", callErr)
	}
	return rect, nil
}

func windowsClickAt(x int, y int, right bool, hold bool, holdMS int) error {
	if _, _, callErr := procSetCursorPos.Call(uintptr(x), uintptr(y)); callErr != syscall.Errno(0) {
		return fmt.Errorf("SetCursorPos failed: %w", callErr)
	}
	downFlag := uint32(windowsMouseEventLeftDown)
	upFlag := uint32(windowsMouseEventLeftUp)
	if right {
		downFlag = windowsMouseEventRightDown
		upFlag = windowsMouseEventRightUp
	}
	if err := windowsSendMouseInput(downFlag, 0); err != nil {
		return err
	}
	if hold {
		time.Sleep(time.Duration(holdMS) * time.Millisecond)
	} else {
		time.Sleep(40 * time.Millisecond)
	}
	return windowsSendMouseInput(upFlag, 0)
}

func windowsRightClickAt(x int, y int, holdMS int) error {
	return windowsClickAt(x, y, true, false, holdMS)
}

func windowsLongPressAt(x int, y int, holdMS int) error {
	return windowsClickAt(x, y, false, true, holdMS)
}

func windowsDoubleClickAt(x int, y int) error {
	if err := windowsClickAt(x, y, false, false, 0); err != nil {
		return err
	}
	time.Sleep(40 * time.Millisecond)
	return windowsClickAt(x, y, false, false, 0)
}

func windowsReleaseChildren(children []windowsAccessibleChild, keep int) {
	for idx, child := range children {
		if idx == keep {
			continue
		}
		if child.Dispatch != nil {
			child.Dispatch.Release()
		}
	}
}

func windowsSendMouseInput(flags uint32, data uint32) error {
	var input windowsInput
	input.Type = windowsInputMouse
	mouse := windowsMouseInput{DwFlags: flags, MouseData: data}
	*(*windowsMouseInput)(unsafe.Pointer(&input.Data[0])) = mouse
	return windowsSendInputs([]windowsInput{input})
}

func windowsSendUnicodeText(text string) error {
	inputs := make([]windowsInput, 0, len(text)*2)
	for _, r := range text {
		down := windowsInput{Type: windowsInputKeyboard}
		up := windowsInput{Type: windowsInputKeyboard}
		*(*windowsKeyboardInput)(unsafe.Pointer(&down.Data[0])) = windowsKeyboardInput{WScan: uint16(r), DwFlags: windowsKeyEventUnicode}
		*(*windowsKeyboardInput)(unsafe.Pointer(&up.Data[0])) = windowsKeyboardInput{WScan: uint16(r), DwFlags: windowsKeyEventUnicode | windowsKeyEventKeyUp}
		inputs = append(inputs, down, up)
	}
	return windowsSendInputs(inputs)
}

func windowsSendKeys(keys []string, holdMS int) error {
	cleaned := make([]string, 0, len(keys))
	for _, key := range keys {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	if len(cleaned) == 0 {
		return NewError("unsupported_action", "keys are required", nil)
	}
	if len(cleaned) == 1 {
		if vk, ok := windowsVirtualKey(cleaned[0]); ok {
			if err := windowsSendVirtualKey(vk, true); err != nil {
				return err
			}
			time.Sleep(time.Duration(holdMS) * time.Millisecond)
			return windowsSendVirtualKey(vk, false)
		}
		return windowsSendUnicodeText(cleaned[0])
	}

	modifiers := make([]uint16, 0, len(cleaned))
	var primary uint16
	primarySet := false
	for _, key := range cleaned {
		vk, ok := windowsVirtualKey(key)
		if !ok {
			continue
		}
		if windowsIsModifierKey(key) {
			modifiers = append(modifiers, vk)
			continue
		}
		primary = vk
		primarySet = true
	}
	for _, modifier := range modifiers {
		if err := windowsSendVirtualKey(modifier, true); err != nil {
			return err
		}
	}
	if primarySet {
		if err := windowsSendVirtualKey(primary, true); err != nil {
			return err
		}
		time.Sleep(time.Duration(holdMS) * time.Millisecond)
		if err := windowsSendVirtualKey(primary, false); err != nil {
			return err
		}
	}
	for idx := len(modifiers) - 1; idx >= 0; idx-- {
		if err := windowsSendVirtualKey(modifiers[idx], false); err != nil {
			return err
		}
	}
	return nil
}

func windowsSendVirtualKey(vk uint16, down bool) error {
	var input windowsInput
	input.Type = windowsInputKeyboard
	flags := uint32(0)
	if !down {
		flags = windowsKeyEventKeyUp
	}
	*(*windowsKeyboardInput)(unsafe.Pointer(&input.Data[0])) = windowsKeyboardInput{WVk: vk, DwFlags: flags}
	return windowsSendInputs([]windowsInput{input})
}

func windowsSendInputs(inputs []windowsInput) error {
	if len(inputs) == 0 {
		return nil
	}
	sent, _, callErr := procSendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		unsafe.Sizeof(inputs[0]),
	)
	if sent != uintptr(len(inputs)) {
		return fmt.Errorf("SendInput failed: %w", callErr)
	}
	return nil
}

func windowsWithCOM(fn func() error) error {
	err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	shouldUninit := err == nil
	if err != nil && !strings.Contains(strings.ToUpper(err.Error()), "80010106") {
		return fmt.Errorf("CoInitializeEx failed: %w", err)
	}
	if shouldUninit {
		defer ole.CoUninitialize()
	}
	return fn()
}

func windowsHRFailed(hr uintptr) bool {
	return int32(hr) < 0
}
