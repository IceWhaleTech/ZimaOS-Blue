package a11y

import (
	"fmt"
	"strconv"
)

type windowsFallbackTarget struct {
	HWND      uintptr
	HasBounds bool
	CenterX   int
	CenterY   int
}

type windowsFallbackExecutor struct {
	BringFront  func(uintptr)
	Click       func(int, int, int) error
	DoubleClick func(int, int) error
	RightClick  func(int, int, int) error
	LongPress   func(int, int, int) error
	AfterFocus  func()
	ClipboardPaste func(string) error
	UnicodeInput   func(string) error
}

func windowsSendTextWithClipboardFallback(
	value string,
	clipboardPaste func(string) error,
	unicodeInput func(string) error,
) (string, error) {
	var clipboardErr error
	if clipboardPaste != nil {
		if err := clipboardPaste(value); err == nil {
			return "clipboard", nil
		} else {
			clipboardErr = err
		}
	}
	if unicodeInput != nil {
		return "unicode", unicodeInput(value)
	}
	return "", clipboardErr
}

func windowsTypeWithFocusClickFallback(
	value string,
	alreadyFocused bool,
	hasBounds bool,
	centerX int,
	centerY int,
	holdMS int,
	click func(int, int, int) error,
	afterFocus func(),
	sendText func(string) error,
) error {
	if hasBounds && !alreadyFocused && click != nil {
		if err := click(centerX, centerY, holdMS); err != nil {
			return err
		}
	}
	if (hasBounds || alreadyFocused) && afterFocus != nil {
		afterFocus()
	}
	if sendText == nil {
		return nil
	}
	return sendText(value)
}

func windowsCaptureWithActiveWindowFallback(
	preferred func() error,
	activeWindow func() error,
) error {
	var preferredErr error
	if preferred != nil {
		if err := preferred(); err == nil {
			return nil
		} else {
			preferredErr = err
		}
	}
	if activeWindow == nil {
		return preferredErr
	}
	if err := activeWindow(); err != nil {
		if preferredErr == nil {
			return err
		}
		return fmt.Errorf("%v; active window fallback failed: %w", preferredErr, err)
	}
	return nil
}

func windowsExecuteFallbackWithInput(
	target windowsFallbackTarget,
	fallback string,
	value string,
	holdMS int,
	primarySucceeded bool,
	executor windowsFallbackExecutor,
) (string, error) {
	if target.HWND != 0 && executor.BringFront != nil {
		executor.BringFront(target.HWND)
	}

	pointerBoundsError := func() error {
		return NewError("unsupported_action", "element bounds are unavailable for pointer fallback", nil)
	}
	fallbackUnavailableError := func() error {
		return NewError("unsupported_action", "input fallback is unavailable", map[string]interface{}{"fallback": fallback})
	}

	switch fallback {
	case windowsActionInputClick:
		if !target.HasBounds {
			return "", pointerBoundsError()
		}
		if executor.Click == nil {
			return "", fallbackUnavailableError()
		}
		if err := executor.Click(target.CenterX, target.CenterY, holdMS); err != nil {
			return "", err
		}
		return "input_click", nil
	case windowsActionInputDoubleClick:
		if !target.HasBounds {
			return "", pointerBoundsError()
		}
		if executor.DoubleClick == nil {
			return "", fallbackUnavailableError()
		}
		if err := executor.DoubleClick(target.CenterX, target.CenterY); err != nil {
			return "", err
		}
		return "input_double_click", nil
	case windowsActionInputRightClick:
		if !target.HasBounds {
			return "", pointerBoundsError()
		}
		if executor.RightClick == nil {
			return "", fallbackUnavailableError()
		}
		if err := executor.RightClick(target.CenterX, target.CenterY, holdMS); err != nil {
			return "", err
		}
		return "input_right_click", nil
	case windowsActionInputLongPress:
		if !target.HasBounds {
			return "", pointerBoundsError()
		}
		if executor.LongPress == nil {
			return "", fallbackUnavailableError()
		}
		if err := executor.LongPress(target.CenterX, target.CenterY, holdMS); err != nil {
			return "", err
		}
		return "input_long_press", nil
	case windowsActionInputFocusClick:
		if !target.HasBounds {
			return "input_focus_click", nil
		}
		if executor.Click == nil {
			return "", fallbackUnavailableError()
		}
		if err := executor.Click(target.CenterX, target.CenterY, holdMS); err != nil {
			return "", err
		}
		return "input_focus_click", nil
	case windowsActionInputType:
		inputMethod := ""
		if err := windowsTypeWithFocusClickFallback(
			value,
			primarySucceeded,
			target.HasBounds,
			target.CenterX,
			target.CenterY,
			holdMS,
			executor.Click,
			executor.AfterFocus,
			func(value string) error {
				if !target.HasBounds && !primarySucceeded && executor.UnicodeInput != nil {
					inputMethod = "unicode"
					return executor.UnicodeInput(value)
				}
				method, err := windowsSendTextWithClipboardFallback(value, executor.ClipboardPaste, executor.UnicodeInput)
				if method != "" {
					inputMethod = method
				}
				return err
			},
		); err != nil {
			return "", err
		}
		if inputMethod == "" {
			inputMethod = "input_type"
		}
		return inputMethod, nil
	default:
		return "", fallbackUnavailableError()
	}
}

func windowsScreenshotResultWithFallback(
	hostOS string,
	hwnd uintptr,
	path string,
	preferred func() error,
	activeWindow func() error,
) (ScreenshotResult, error) {
	if err := windowsCaptureWithActiveWindowFallback(preferred, activeWindow); err != nil {
		return ScreenshotResult{HostOS: hostOS}, err
	}
	return ScreenshotResult{
		HostOS:    hostOS,
		WindowID:  strconv.FormatUint(uint64(hwnd), 10),
		ImagePath: path,
		Message:   "Host screenshot captured",
	}, nil
}
