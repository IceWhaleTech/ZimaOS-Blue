//go:build darwin

package a11y

import "time"

const darwinInputHighlightDuration = 350 * time.Millisecond

func darwinHighlightInputBounds(
	bounds darwinRect,
	hasBounds bool,
	highlight func(darwinRect, time.Duration) error,
) {
	if !hasBounds || highlight == nil {
		return
	}
	if bounds.Size.Width <= 1 || bounds.Size.Height <= 1 {
		return
	}
	_ = highlight(bounds, darwinInputHighlightDuration)
}

func darwinTypeWithFocusClickFallback(
	value string,
	alreadyFocused bool,
	click func() error,
	afterFocus func(),
	sendText func(string) error,
) error {
	if !alreadyFocused && click != nil {
		if err := click(); err != nil {
			return err
		}
	}
	if afterFocus != nil {
		afterFocus()
	}
	if sendText == nil {
		return nil
	}
	return sendText(value)
}

func darwinSendTextWithClipboardFallback(
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
