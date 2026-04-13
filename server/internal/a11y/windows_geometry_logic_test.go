package a11y

import "testing"

func TestWindowsRectHasArea_RequiresPositiveWidthAndHeight(t *testing.T) {
	tests := []struct {
		name string
		rect windowsRect
		want bool
	}{
		{name: "visible rect", rect: windowsRect{Left: 10, Top: 20, Right: 110, Bottom: 220}, want: true},
		{name: "zero width", rect: windowsRect{Left: 10, Top: 20, Right: 10, Bottom: 220}, want: false},
		{name: "zero height", rect: windowsRect{Left: 10, Top: 20, Right: 110, Bottom: 20}, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := windowsRectHasArea(tc.rect); got != tc.want {
				t.Fatalf("windowsRectHasArea(%+v) = %v, want %v", tc.rect, got, tc.want)
			}
		})
	}
}

func TestWindowsRectCenter_ReturnsMidpoint(t *testing.T) {
	x, y := windowsRectCenter(windowsRect{Left: 10, Top: 20, Right: 110, Bottom: 220})
	if x != 60 || y != 120 {
		t.Fatalf("windowsRectCenter() = (%d,%d), want (60,120)", x, y)
	}
}

func TestWindowsVisibleRectSize_ReturnsDimensionsAndVisibility(t *testing.T) {
	width, height, ok := windowsVisibleRectSize(windowsRect{Left: 10, Top: 20, Right: 110, Bottom: 220})
	if width != 100 || height != 200 || !ok {
		t.Fatalf("windowsVisibleRectSize(visible) = (%d,%d,%v), want (100,200,true)", width, height, ok)
	}

	width, height, ok = windowsVisibleRectSize(windowsRect{Left: 10, Top: 20, Right: 10, Bottom: 220})
	if width != 0 || height != 200 || ok {
		t.Fatalf("windowsVisibleRectSize(zero width) = (%d,%d,%v), want (0,200,false)", width, height, ok)
	}
}

func TestWindowsFallbackTargetFromRect_SetsBoundsAndCenter(t *testing.T) {
	target := windowsFallbackTargetFromRect(42, windowsRect{Left: 10, Top: 20, Right: 110, Bottom: 220})
	if target.HWND != 42 {
		t.Fatalf("target.HWND = %d, want 42", target.HWND)
	}
	if !target.HasBounds {
		t.Fatal("target.HasBounds = false, want true")
	}
	if target.CenterX != 60 || target.CenterY != 120 {
		t.Fatalf("center = (%d,%d), want (60,120)", target.CenterX, target.CenterY)
	}

	empty := windowsFallbackTargetFromRect(42, windowsRect{})
	if empty.HasBounds {
		t.Fatal("empty.HasBounds = true, want false")
	}
	if empty.CenterX != 0 || empty.CenterY != 0 {
		t.Fatalf("empty center = (%d,%d), want (0,0)", empty.CenterX, empty.CenterY)
	}
}
