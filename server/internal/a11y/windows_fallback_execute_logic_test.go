package a11y

import (
	"strings"
	"testing"
)

func TestWindowsExecuteFallbackWithInput_PointerBranchesDispatchExpectedCallback(t *testing.T) {
	tests := []struct {
		name     string
		fallback string
		wantStep string
	}{
		{name: "click", fallback: windowsActionInputClick, wantStep: "click"},
		{name: "double click", fallback: windowsActionInputDoubleClick, wantStep: "double"},
		{name: "right click", fallback: windowsActionInputRightClick, wantStep: "right"},
		{name: "long press", fallback: windowsActionInputLongPress, wantStep: "long"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var steps []string
			err := windowsExecuteFallbackWithInput(
				windowsFallbackTarget{
					HWND:      42,
					HasBounds: true,
					CenterX:   100,
					CenterY:   200,
				},
				tc.fallback,
				"",
				750,
				windowsFallbackExecutor{
					BringFront: func(hwnd uintptr) {
						if hwnd != 42 {
							t.Fatalf("BringFront hwnd = %d, want 42", hwnd)
						}
						steps = append(steps, "front")
					},
					Click: func(x int, y int, holdMS int) error {
						if x != 100 || y != 200 || holdMS != 750 {
							t.Fatalf("Click args = (%d,%d,%d), want (100,200,750)", x, y, holdMS)
						}
						steps = append(steps, "click")
						return nil
					},
					DoubleClick: func(x int, y int) error {
						if x != 100 || y != 200 {
							t.Fatalf("DoubleClick args = (%d,%d), want (100,200)", x, y)
						}
						steps = append(steps, "double")
						return nil
					},
					RightClick: func(x int, y int, holdMS int) error {
						if x != 100 || y != 200 || holdMS != 750 {
							t.Fatalf("RightClick args = (%d,%d,%d), want (100,200,750)", x, y, holdMS)
						}
						steps = append(steps, "right")
						return nil
					},
					LongPress: func(x int, y int, holdMS int) error {
						if x != 100 || y != 200 || holdMS != 750 {
							t.Fatalf("LongPress args = (%d,%d,%d), want (100,200,750)", x, y, holdMS)
						}
						steps = append(steps, "long")
						return nil
					},
				},
			)
			if err != nil {
				t.Fatalf("windowsExecuteFallbackWithInput() error = %v", err)
			}
			if got := strings.Join(steps, ","); got != "front,"+tc.wantStep {
				t.Fatalf("steps = %q, want %q", got, "front,"+tc.wantStep)
			}
		})
	}
}

func TestWindowsExecuteFallbackWithInput_FocusClickWithoutBoundsIsNoop(t *testing.T) {
	var steps []string

	err := windowsExecuteFallbackWithInput(
		windowsFallbackTarget{HWND: 42},
		windowsActionInputFocusClick,
		"",
		600,
		windowsFallbackExecutor{
			BringFront: func(hwnd uintptr) {
				if hwnd != 42 {
					t.Fatalf("BringFront hwnd = %d, want 42", hwnd)
				}
				steps = append(steps, "front")
			},
			Click: func(int, int, int) error {
				t.Fatal("Click should not run without bounds")
				return nil
			},
		},
	)
	if err != nil {
		t.Fatalf("windowsExecuteFallbackWithInput() error = %v", err)
	}
	if got := strings.Join(steps, ","); got != "front" {
		t.Fatalf("steps = %q, want front", got)
	}
}

func TestWindowsExecuteFallbackWithInput_TypeWithoutBoundsStillSendsText(t *testing.T) {
	var steps []string

	err := windowsExecuteFallbackWithInput(
		windowsFallbackTarget{HWND: 42},
		windowsActionInputType,
		"hello",
		600,
		windowsFallbackExecutor{
			BringFront: func(hwnd uintptr) {
				if hwnd != 42 {
					t.Fatalf("BringFront hwnd = %d, want 42", hwnd)
				}
				steps = append(steps, "front")
			},
			Click: func(int, int, int) error {
				t.Fatal("Click should not run without bounds")
				return nil
			},
			AfterFocus: func() {
				t.Fatal("AfterFocus should not run without bounds")
			},
			SendText: func(value string) error {
				if value != "hello" {
					t.Fatalf("SendText value = %q, want hello", value)
				}
				steps = append(steps, "send")
				return nil
			},
		},
	)
	if err != nil {
		t.Fatalf("windowsExecuteFallbackWithInput() error = %v", err)
	}
	if got := strings.Join(steps, ","); got != "front,send" {
		t.Fatalf("steps = %q, want front,send", got)
	}
}

func TestWindowsExecuteFallbackWithInput_RejectsPointerFallbackWithoutBounds(t *testing.T) {
	err := windowsExecuteFallbackWithInput(
		windowsFallbackTarget{HWND: 42},
		windowsActionInputClick,
		"",
		600,
		windowsFallbackExecutor{
			BringFront: func(uintptr) {},
			Click: func(int, int, int) error {
				t.Fatal("Click should not run without bounds")
				return nil
			},
		},
	)
	if err == nil {
		t.Fatal("windowsExecuteFallbackWithInput() error = nil, want unsupported_action")
	}
	runtimeErr, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Code != "unsupported_action" {
		t.Fatalf("code = %q, want unsupported_action", runtimeErr.Code)
	}
}

func TestWindowsExecuteFallbackWithInput_RejectsUnsupportedFallback(t *testing.T) {
	err := windowsExecuteFallbackWithInput(
		windowsFallbackTarget{HWND: 42, HasBounds: true, CenterX: 100, CenterY: 200},
		"input_magic",
		"",
		600,
		windowsFallbackExecutor{},
	)
	if err == nil {
		t.Fatal("windowsExecuteFallbackWithInput() error = nil, want unsupported_action")
	}
	runtimeErr, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Code != "unsupported_action" {
		t.Fatalf("code = %q, want unsupported_action", runtimeErr.Code)
	}
}
