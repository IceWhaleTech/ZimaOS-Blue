package a11y

import (
	"errors"
	"testing"
)

func TestWindowsFocusResultWithResolver_ResolvesAndBringsFront(t *testing.T) {
	resolveCalls := 0
	bringFrontCalls := 0

	result, err := windowsFocusResultWithResolver(
		"windows",
		"42",
		func(windowID string) (uintptr, WindowInfo, error) {
			resolveCalls++
			if windowID != "42" {
				t.Fatalf("windowID = %q, want 42", windowID)
			}
			return 42, WindowInfo{ID: "42", Title: "Feishu"}, nil
		},
		func(hwnd uintptr) {
			bringFrontCalls++
			if hwnd != 42 {
				t.Fatalf("hwnd = %d, want 42", hwnd)
			}
		},
	)
	if err != nil {
		t.Fatalf("windowsFocusResultWithResolver() error = %v", err)
	}
	if resolveCalls != 1 {
		t.Fatalf("resolveCalls = %d, want 1", resolveCalls)
	}
	if bringFrontCalls != 1 {
		t.Fatalf("bringFrontCalls = %d, want 1", bringFrontCalls)
	}
	if result.HostOS != "windows" {
		t.Fatalf("host_os = %q, want windows", result.HostOS)
	}
	if result.WindowID != "42" {
		t.Fatalf("window_id = %q, want 42", result.WindowID)
	}
	if result.ExecutionMode != "input" {
		t.Fatalf("execution_mode = %q, want input", result.ExecutionMode)
	}
	if result.Message != "Window focused" {
		t.Fatalf("message = %q, want Window focused", result.Message)
	}
}

func TestWindowsFocusResultWithResolver_PropagatesResolveError(t *testing.T) {
	bringFrontCalls := 0

	_, err := windowsFocusResultWithResolver(
		"windows",
		"missing",
		func(string) (uintptr, WindowInfo, error) {
			return 0, WindowInfo{}, errors.New("target window not found")
		},
		func(uintptr) {
			bringFrontCalls++
		},
	)
	if err == nil {
		t.Fatal("windowsFocusResultWithResolver() error = nil, want failure")
	}
	if err.Error() != "target window not found" {
		t.Fatalf("error = %v, want target window not found", err)
	}
	if bringFrontCalls != 0 {
		t.Fatalf("bringFrontCalls = %d, want 0", bringFrontCalls)
	}
}

func TestWindowsResolveWindowFromList_PrefersFocusedWindowByDefault(t *testing.T) {
	hwnd, info, err := windowsResolveWindowFromList(
		"",
		[]WindowInfo{
			{ID: "101", Title: "First"},
			{ID: "202", Title: "Focused", Focused: true},
			{ID: "303", Title: "Third"},
		},
	)
	if err != nil {
		t.Fatalf("windowsResolveWindowFromList() error = %v", err)
	}
	if hwnd != 202 {
		t.Fatalf("hwnd = %d, want 202", hwnd)
	}
	if info.ID != "202" {
		t.Fatalf("info.ID = %q, want 202", info.ID)
	}
}

func TestWindowsResolveWindowFromList_FallsBackToFirstWindowWhenNoneFocused(t *testing.T) {
	hwnd, info, err := windowsResolveWindowFromList(
		"",
		[]WindowInfo{
			{ID: "101", Title: "First"},
			{ID: "202", Title: "Second"},
		},
	)
	if err != nil {
		t.Fatalf("windowsResolveWindowFromList() error = %v", err)
	}
	if hwnd != 101 {
		t.Fatalf("hwnd = %d, want 101", hwnd)
	}
	if info.ID != "101" {
		t.Fatalf("info.ID = %q, want 101", info.ID)
	}
}

func TestWindowsResolveWindowFromList_MatchesExplicitWindowID(t *testing.T) {
	hwnd, info, err := windowsResolveWindowFromList(
		"303",
		[]WindowInfo{
			{ID: "101", Title: "First"},
			{ID: "303", Title: "Third"},
		},
	)
	if err != nil {
		t.Fatalf("windowsResolveWindowFromList() error = %v", err)
	}
	if hwnd != 303 {
		t.Fatalf("hwnd = %d, want 303", hwnd)
	}
	if info.ID != "303" {
		t.Fatalf("info.ID = %q, want 303", info.ID)
	}
}

func TestWindowsResolveWindowFromList_ReturnsNoWindowsErrorForEmptyList(t *testing.T) {
	_, _, err := windowsResolveWindowFromList("", nil)
	if err == nil {
		t.Fatal("windowsResolveWindowFromList() error = nil, want backend_unavailable")
	}
	runtimeErr, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Code != "backend_unavailable" {
		t.Fatalf("code = %q, want backend_unavailable", runtimeErr.Code)
	}
	if runtimeErr.Message != "no host windows available" {
		t.Fatalf("message = %q, want no host windows available", runtimeErr.Message)
	}
}

func TestWindowsResolveWindowFromList_ReturnsNotFoundForMissingWindowID(t *testing.T) {
	_, _, err := windowsResolveWindowFromList(
		"999",
		[]WindowInfo{
			{ID: "101", Title: "First"},
			{ID: "202", Title: "Second"},
		},
	)
	if err == nil {
		t.Fatal("windowsResolveWindowFromList() error = nil, want backend_unavailable")
	}
	runtimeErr, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Code != "backend_unavailable" {
		t.Fatalf("code = %q, want backend_unavailable", runtimeErr.Code)
	}
	if runtimeErr.Message != "target window not found" {
		t.Fatalf("message = %q, want target window not found", runtimeErr.Message)
	}
	if runtimeErr.Details["window_id"] != "999" {
		t.Fatalf("details.window_id = %#v, want 999", runtimeErr.Details["window_id"])
	}
}

func TestWindowsShouldIncludeEnumeratedWindow_RequiresVisibleNonBlankTitle(t *testing.T) {
	tests := []struct {
		name    string
		visible bool
		title   string
		want    bool
	}{
		{name: "visible titled window", visible: true, title: "Feishu", want: true},
		{name: "hidden window", visible: false, title: "Feishu", want: false},
		{name: "blank title", visible: true, title: "   ", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := windowsShouldIncludeEnumeratedWindow(tc.visible, tc.title); got != tc.want {
				t.Fatalf("windowsShouldIncludeEnumeratedWindow(%v, %q) = %v, want %v", tc.visible, tc.title, got, tc.want)
			}
		})
	}
}

func TestWindowsWindowInfoFromEnumeration_SetsFocusedAndFields(t *testing.T) {
	info := windowsWindowInfoFromEnumeration(202, 202, "Feishu", "飞书", 9876)
	if info.ID != "202" {
		t.Fatalf("info.ID = %q, want 202", info.ID)
	}
	if info.Title != "Feishu" {
		t.Fatalf("info.Title = %q, want Feishu", info.Title)
	}
	if info.AppName != "飞书" {
		t.Fatalf("info.AppName = %q, want 飞书", info.AppName)
	}
	if info.PID != 9876 {
		t.Fatalf("info.PID = %d, want 9876", info.PID)
	}
	if !info.Focused {
		t.Fatal("info.Focused = false, want true")
	}
}

func TestWindowsSnapshotToken_FormatsRootAndPath(t *testing.T) {
	if got := windowsSnapshotToken(42, nil); got != "42|root" {
		t.Fatalf("windowsSnapshotToken(root) = %q, want 42|root", got)
	}
	if got := windowsSnapshotToken(42, []int{1, 3, 5}); got != "42|1.3.5" {
		t.Fatalf("windowsSnapshotToken(path) = %q, want 42|1.3.5", got)
	}
}

func TestWindowsParseSnapshotToken_ParsesRootAndPath(t *testing.T) {
	hwnd, path, err := windowsParseSnapshotToken("42|root")
	if err != nil {
		t.Fatalf("windowsParseSnapshotToken(root) error = %v", err)
	}
	if hwnd != 42 {
		t.Fatalf("hwnd = %d, want 42", hwnd)
	}
	if len(path) != 0 {
		t.Fatalf("len(path) = %d, want 0", len(path))
	}

	hwnd, path, err = windowsParseSnapshotToken("84|1.3.5")
	if err != nil {
		t.Fatalf("windowsParseSnapshotToken(path) error = %v", err)
	}
	if hwnd != 84 {
		t.Fatalf("hwnd = %d, want 84", hwnd)
	}
	if len(path) != 3 || path[0] != 1 || path[1] != 3 || path[2] != 5 {
		t.Fatalf("path = %#v, want []int{1,3,5}", path)
	}
}

func TestWindowsParseSnapshotToken_RejectsInvalidToken(t *testing.T) {
	tests := []string{
		"",
		"42",
		"oops|1.2",
		"42|1.two",
	}

	for _, token := range tests {
		t.Run(token, func(t *testing.T) {
			if _, _, err := windowsParseSnapshotToken(token); err == nil {
				t.Fatalf("windowsParseSnapshotToken(%q) error = nil, want failure", token)
			}
		})
	}
}

func TestWindowsParseHWNDWithForeground_ParsesExplicitValueWithoutForegroundLookup(t *testing.T) {
	foregroundCalls := 0
	hwnd, err := windowsParseHWNDWithForeground(" 42 ", func() uintptr {
		foregroundCalls++
		return 99
	})
	if err != nil {
		t.Fatalf("windowsParseHWNDWithForeground() error = %v", err)
	}
	if hwnd != 42 {
		t.Fatalf("hwnd = %d, want 42", hwnd)
	}
	if foregroundCalls != 0 {
		t.Fatalf("foregroundCalls = %d, want 0", foregroundCalls)
	}
}

func TestWindowsParseHWNDWithForeground_UsesForegroundForBlankValue(t *testing.T) {
	foregroundCalls := 0
	hwnd, err := windowsParseHWNDWithForeground("", func() uintptr {
		foregroundCalls++
		return 77
	})
	if err != nil {
		t.Fatalf("windowsParseHWNDWithForeground() error = %v", err)
	}
	if hwnd != 77 {
		t.Fatalf("hwnd = %d, want 77", hwnd)
	}
	if foregroundCalls != 1 {
		t.Fatalf("foregroundCalls = %d, want 1", foregroundCalls)
	}
}

func TestWindowsParseHWNDWithForeground_ReturnsBackendUnavailableWhenForegroundMissing(t *testing.T) {
	_, err := windowsParseHWNDWithForeground("", func() uintptr {
		return 0
	})
	if err == nil {
		t.Fatal("windowsParseHWNDWithForeground() error = nil, want backend_unavailable")
	}
	runtimeErr, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Code != "backend_unavailable" {
		t.Fatalf("code = %q, want backend_unavailable", runtimeErr.Code)
	}
	if runtimeErr.Message != "foreground window is unavailable" {
		t.Fatalf("message = %q, want foreground window is unavailable", runtimeErr.Message)
	}
}

func TestWindowsResolveActionTargetFromRef_ResolvesTokenAndOptionalWindowOverride(t *testing.T) {
	parseTokenCalls := 0
	parseHWNDCalls := 0

	hwnd, path, err := windowsResolveActionTargetFromRef(
		7,
		map[int]string{7: "42|1.3"},
		"99",
		func(token string) (uintptr, []int, error) {
			parseTokenCalls++
			if token != "42|1.3" {
				t.Fatalf("token = %q, want 42|1.3", token)
			}
			return 42, []int{1, 3}, nil
		},
		func(value string) (uintptr, error) {
			parseHWNDCalls++
			if value != "99" {
				t.Fatalf("windowID = %q, want 99", value)
			}
			return 99, nil
		},
	)
	if err != nil {
		t.Fatalf("windowsResolveActionTargetFromRef() error = %v", err)
	}
	if parseTokenCalls != 1 {
		t.Fatalf("parseTokenCalls = %d, want 1", parseTokenCalls)
	}
	if parseHWNDCalls != 1 {
		t.Fatalf("parseHWNDCalls = %d, want 1", parseHWNDCalls)
	}
	if hwnd != 99 {
		t.Fatalf("hwnd = %d, want 99", hwnd)
	}
	if len(path) != 2 || path[0] != 1 || path[1] != 3 {
		t.Fatalf("path = %#v, want []int{1,3}", path)
	}
}

func TestWindowsResolveActionTargetFromRef_IgnoresInvalidWindowOverride(t *testing.T) {
	hwnd, path, err := windowsResolveActionTargetFromRef(
		7,
		map[int]string{7: "42|1.3"},
		"oops",
		func(string) (uintptr, []int, error) {
			return 42, []int{1, 3}, nil
		},
		func(string) (uintptr, error) {
			return 0, errors.New("bad window id")
		},
	)
	if err != nil {
		t.Fatalf("windowsResolveActionTargetFromRef() error = %v", err)
	}
	if hwnd != 42 {
		t.Fatalf("hwnd = %d, want 42", hwnd)
	}
	if len(path) != 2 || path[0] != 1 || path[1] != 3 {
		t.Fatalf("path = %#v, want []int{1,3}", path)
	}
}

func TestWindowsResolveActionTargetFromRef_ReturnsStaleRefForMissingOrInvalidToken(t *testing.T) {
	tests := []struct {
		name  string
		ref   int
		refMap map[int]string
		parse func(string) (uintptr, []int, error)
	}{
		{
			name:  "missing ref",
			ref:   9,
			refMap: map[int]string{7: "42|1.3"},
			parse: func(string) (uintptr, []int, error) {
				t.Fatal("parse should not run for missing ref")
				return 0, nil, nil
			},
		},
		{
			name:  "invalid token",
			ref:   7,
			refMap: map[int]string{7: "bad"},
			parse: func(string) (uintptr, []int, error) {
				return 0, nil, errors.New("invalid token")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := windowsResolveActionTargetFromRef(
				tc.ref,
				tc.refMap,
				"",
				tc.parse,
				func(string) (uintptr, error) {
					t.Fatal("parseHWND should not run")
					return 0, nil
				},
			)
			if err == nil {
				t.Fatal("windowsResolveActionTargetFromRef() error = nil, want stale_ref")
			}
			runtimeErr, ok := err.(*RuntimeError)
			if !ok {
				t.Fatalf("error type = %T, want *RuntimeError", err)
			}
			if runtimeErr.Code != "stale_ref" {
				t.Fatalf("code = %q, want stale_ref", runtimeErr.Code)
			}
		})
	}
}
