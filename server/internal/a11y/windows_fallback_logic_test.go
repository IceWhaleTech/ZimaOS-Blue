package a11y

import (
	"errors"
	"strings"
	"testing"
)

func TestWindowsSendTextWithClipboardFallback_PrefersClipboardPaste(t *testing.T) {
	clipboardCalls := 0
	unicodeCalls := 0

	err := windowsSendTextWithClipboardFallback(
		"hello",
		func(value string) error {
			clipboardCalls++
			if value != "hello" {
				t.Fatalf("clipboard value = %q, want hello", value)
			}
			return nil
		},
		func(string) error {
			unicodeCalls++
			return nil
		},
	)
	if err != nil {
		t.Fatalf("windowsSendTextWithClipboardFallback() error = %v", err)
	}
	if clipboardCalls != 1 {
		t.Fatalf("clipboardCalls = %d, want 1", clipboardCalls)
	}
	if unicodeCalls != 0 {
		t.Fatalf("unicodeCalls = %d, want 0", unicodeCalls)
	}
}

func TestWindowsSendTextWithClipboardFallback_FallsBackToUnicodeInput(t *testing.T) {
	clipboardCalls := 0
	unicodeCalls := 0

	err := windowsSendTextWithClipboardFallback(
		"hello",
		func(string) error {
			clipboardCalls++
			return errors.New("clipboard busy")
		},
		func(value string) error {
			unicodeCalls++
			if value != "hello" {
				t.Fatalf("unicode value = %q, want hello", value)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("windowsSendTextWithClipboardFallback() error = %v", err)
	}
	if clipboardCalls != 1 {
		t.Fatalf("clipboardCalls = %d, want 1", clipboardCalls)
	}
	if unicodeCalls != 1 {
		t.Fatalf("unicodeCalls = %d, want 1", unicodeCalls)
	}
}

func TestWindowsTypeWithFocusClickFallback_ClicksBeforeSendingText(t *testing.T) {
	var steps []string

	err := windowsTypeWithFocusClickFallback(
		"hello",
		true,
		100,
		200,
		600,
		func(x int, y int, holdMS int) error {
			if x != 100 || y != 200 || holdMS != 600 {
				t.Fatalf("click args = (%d,%d,%d), want (100,200,600)", x, y, holdMS)
			}
			steps = append(steps, "click")
			return nil
		},
		func() {
			steps = append(steps, "after_focus")
		},
		func(value string) error {
			if value != "hello" {
				t.Fatalf("send value = %q, want hello", value)
			}
			steps = append(steps, "send")
			return nil
		},
	)
	if err != nil {
		t.Fatalf("windowsTypeWithFocusClickFallback() error = %v", err)
	}
	if strings.Join(steps, ",") != "click,after_focus,send" {
		t.Fatalf("steps = %v, want click,after_focus,send", steps)
	}
}

func TestWindowsTypeWithFocusClickFallback_SkipsClickWhenBoundsUnavailable(t *testing.T) {
	clickCalls := 0
	sendCalls := 0

	err := windowsTypeWithFocusClickFallback(
		"hello",
		false,
		0,
		0,
		600,
		func(int, int, int) error {
			clickCalls++
			return nil
		},
		func() {
			t.Fatal("afterFocus should not run without bounds")
		},
		func(string) error {
			sendCalls++
			return nil
		},
	)
	if err != nil {
		t.Fatalf("windowsTypeWithFocusClickFallback() error = %v", err)
	}
	if clickCalls != 0 {
		t.Fatalf("clickCalls = %d, want 0", clickCalls)
	}
	if sendCalls != 1 {
		t.Fatalf("sendCalls = %d, want 1", sendCalls)
	}
}

func TestWindowsCaptureWithActiveWindowFallback_UsesPreferredCaptureWhenAvailable(t *testing.T) {
	preferredCalls := 0
	activeCalls := 0

	err := windowsCaptureWithActiveWindowFallback(
		func() error {
			preferredCalls++
			return nil
		},
		func() error {
			activeCalls++
			return nil
		},
	)
	if err != nil {
		t.Fatalf("windowsCaptureWithActiveWindowFallback() error = %v", err)
	}
	if preferredCalls != 1 {
		t.Fatalf("preferredCalls = %d, want 1", preferredCalls)
	}
	if activeCalls != 0 {
		t.Fatalf("activeCalls = %d, want 0", activeCalls)
	}
}

func TestWindowsCaptureWithActiveWindowFallback_FallsBackToActiveWindowCapture(t *testing.T) {
	preferredCalls := 0
	activeCalls := 0

	err := windowsCaptureWithActiveWindowFallback(
		func() error {
			preferredCalls++
			return errors.New("target window has no visible bounds")
		},
		func() error {
			activeCalls++
			return nil
		},
	)
	if err != nil {
		t.Fatalf("windowsCaptureWithActiveWindowFallback() error = %v", err)
	}
	if preferredCalls != 1 {
		t.Fatalf("preferredCalls = %d, want 1", preferredCalls)
	}
	if activeCalls != 1 {
		t.Fatalf("activeCalls = %d, want 1", activeCalls)
	}
}

func TestWindowsCaptureWithActiveWindowFallback_PreservesBothErrors(t *testing.T) {
	err := windowsCaptureWithActiveWindowFallback(
		func() error {
			return errors.New("powershell region capture failed")
		},
		func() error {
			return errors.New("Alt+PrintScreen fallback failed")
		},
	)
	if err == nil {
		t.Fatal("windowsCaptureWithActiveWindowFallback() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "powershell region capture failed") {
		t.Fatalf("error = %v, want preferred capture failure", err)
	}
	if !strings.Contains(err.Error(), "Alt+PrintScreen fallback failed") {
		t.Fatalf("error = %v, want active-window fallback failure", err)
	}
}

func TestWindowsScreenshotResultWithFallback_ReturnsResultOnSuccess(t *testing.T) {
	preferredCalls := 0
	activeCalls := 0

	result, err := windowsScreenshotResultWithFallback(
		"windows",
		42,
		"/tmp/host-window-42.png",
		func() error {
			preferredCalls++
			return nil
		},
		func() error {
			activeCalls++
			return nil
		},
	)
	if err != nil {
		t.Fatalf("windowsScreenshotResultWithFallback() error = %v", err)
	}
	if preferredCalls != 1 {
		t.Fatalf("preferredCalls = %d, want 1", preferredCalls)
	}
	if activeCalls != 0 {
		t.Fatalf("activeCalls = %d, want 0", activeCalls)
	}
	if result.HostOS != "windows" {
		t.Fatalf("host_os = %q, want windows", result.HostOS)
	}
	if result.WindowID != "42" {
		t.Fatalf("window_id = %q, want 42", result.WindowID)
	}
	if result.ImagePath != "/tmp/host-window-42.png" {
		t.Fatalf("image_path = %q, want /tmp/host-window-42.png", result.ImagePath)
	}
}

func TestWindowsScreenshotResultWithFallback_PreservesFailure(t *testing.T) {
	_, err := windowsScreenshotResultWithFallback(
		"windows",
		42,
		"/tmp/host-window-42.png",
		func() error {
			return errors.New("region capture failed")
		},
		func() error {
			return errors.New("Alt+PrintScreen fallback failed")
		},
	)
	if err == nil {
		t.Fatal("windowsScreenshotResultWithFallback() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "region capture failed") {
		t.Fatalf("error = %v, want region capture failure", err)
	}
	if !strings.Contains(err.Error(), "Alt+PrintScreen fallback failed") {
		t.Fatalf("error = %v, want active-window fallback failure", err)
	}
}
