package a11y

import (
	"fmt"
	"strconv"
	"strings"
)

var windowsCurrentForegroundHWNDFunc = func() uintptr { return 0 }

func windowsFocusResultWithResolver(
	hostOS string,
	windowID string,
	resolve func(string) (uintptr, WindowInfo, error),
	bringFront func(uintptr),
) (ActionResult, error) {
	hwnd, _, err := resolve(windowID)
	if err != nil {
		return ActionResult{HostOS: hostOS}, err
	}
	if bringFront != nil {
		bringFront(hwnd)
	}
	return ActionResult{
		HostOS:        hostOS,
		WindowID:      strconv.FormatUint(uint64(hwnd), 10),
		ExecutionMode: "input",
		Message:       "Window focused",
	}, nil
}

func windowsResolveWindowFromList(windowID string, list []WindowInfo) (uintptr, WindowInfo, error) {
	if len(list) == 0 {
		return 0, WindowInfo{}, NewError("backend_unavailable", "no host windows available", nil)
	}

	windowID = strings.TrimSpace(windowID)
	if windowID == "" {
		for _, item := range list {
			if item.Focused {
				return windowsWindowInfoHWND(item)
			}
		}
		return windowsWindowInfoHWND(list[0])
	}

	for _, item := range list {
		if item.ID == windowID {
			return windowsWindowInfoHWND(item)
		}
	}

	return 0, WindowInfo{}, NewError("backend_unavailable", "target window not found", map[string]interface{}{"window_id": windowID})
}

func windowsWindowInfoHWND(info WindowInfo) (uintptr, WindowInfo, error) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(info.ID), 10, 64)
	if err != nil {
		return 0, WindowInfo{}, err
	}
	return uintptr(parsed), info, nil
}

func windowsShouldIncludeEnumeratedWindow(visible bool, title string) bool {
	return visible && strings.TrimSpace(title) != ""
}

func windowsWindowInfoFromEnumeration(hwnd uintptr, foreground uintptr, title string, appName string, pid uint32) WindowInfo {
	return WindowInfo{
		ID:      strconv.FormatUint(uint64(hwnd), 10),
		Title:   title,
		AppName: appName,
		PID:     int(pid),
		Focused: hwnd == foreground,
	}
}

func windowsSnapshotToken(hwnd uintptr, path []int) string {
	if len(path) == 0 {
		return fmt.Sprintf("%d|root", hwnd)
	}
	parts := make([]string, 0, len(path))
	for _, item := range path {
		parts = append(parts, strconv.Itoa(item))
	}
	return fmt.Sprintf("%d|%s", hwnd, strings.Join(parts, "."))
}

func windowsParseSnapshotToken(token string) (uintptr, []int, error) {
	parts := strings.SplitN(strings.TrimSpace(token), "|", 2)
	if len(parts) != 2 {
		return 0, nil, fmt.Errorf("invalid token")
	}
	hwnd, err := windowsParseHWND(parts[0])
	if err != nil {
		return 0, nil, err
	}
	if parts[1] == "" || parts[1] == "root" {
		return hwnd, nil, nil
	}
	rawSteps := strings.Split(parts[1], ".")
	path := make([]int, 0, len(rawSteps))
	for _, raw := range rawSteps {
		step, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return 0, nil, err
		}
		path = append(path, step)
	}
	return hwnd, path, nil
}

func windowsParseHWNDWithForeground(value string, foreground func() uintptr) (uintptr, error) {
	if strings.TrimSpace(value) == "" {
		var hwnd uintptr
		if foreground != nil {
			hwnd = foreground()
		}
		if hwnd == 0 {
			return 0, NewError("backend_unavailable", "foreground window is unavailable", nil)
		}
		return hwnd, nil
	}
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, err
	}
	return uintptr(parsed), nil
}

func windowsParseHWND(value string) (uintptr, error) {
	return windowsParseHWNDWithForeground(value, windowsCurrentForegroundHWNDFunc)
}

func windowsResolveActionTargetFromRef(
	ref int,
	refMap map[int]string,
	windowID string,
	parseToken func(string) (uintptr, []int, error),
	parseHWND func(string) (uintptr, error),
) (uintptr, []int, error) {
	token := strings.TrimSpace(refMap[ref])
	if token == "" {
		return 0, nil, NewError("stale_ref", fmt.Sprintf("ref @%d is no longer valid; take a new snapshot first", ref), nil)
	}

	if parseToken == nil {
		parseToken = windowsParseSnapshotToken
	}
	hwnd, path, err := parseToken(token)
	if err != nil {
		return 0, nil, NewError("stale_ref", fmt.Sprintf("ref @%d is no longer valid; take a new snapshot first", ref), nil)
	}

	if trimmed := strings.TrimSpace(windowID); trimmed != "" {
		if parseHWND == nil {
			parseHWND = windowsParseHWND
		}
		if explicit, parseErr := parseHWND(trimmed); parseErr == nil {
			hwnd = explicit
		}
	}

	return hwnd, path, nil
}
