package a11y

type windowsRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

func windowsVisibleRectSize(rect windowsRect) (int, int, bool) {
	width := int(rect.Right - rect.Left)
	height := int(rect.Bottom - rect.Top)
	return width, height, width > 0 && height > 0
}

func windowsRectHasArea(rect windowsRect) bool {
	_, _, ok := windowsVisibleRectSize(rect)
	return ok
}

func windowsRectCenter(rect windowsRect) (int, int) {
	return int((rect.Left + rect.Right) / 2), int((rect.Top + rect.Bottom) / 2)
}

func windowsFallbackTargetFromRect(hwnd uintptr, rect windowsRect) windowsFallbackTarget {
	target := windowsFallbackTarget{HWND: hwnd}
	if !windowsRectHasArea(rect) {
		return target
	}
	target.HasBounds = true
	target.CenterX, target.CenterY = windowsRectCenter(rect)
	return target
}
