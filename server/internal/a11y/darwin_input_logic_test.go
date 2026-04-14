//go:build darwin

package a11y

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDarwinTypeWithFocusClickFallback_ClicksBeforeSendingText(t *testing.T) {
	var steps []string

	err := darwinTypeWithFocusClickFallback(
		"hello",
		false,
		func() error {
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
		t.Fatalf("darwinTypeWithFocusClickFallback() error = %v", err)
	}
	if strings.Join(steps, ",") != "click,after_focus,send" {
		t.Fatalf("steps = %v, want click,after_focus,send", steps)
	}
}

func TestDarwinTypeWithFocusClickFallback_SkipsRedundantClickWhenAlreadyFocused(t *testing.T) {
	var steps []string

	err := darwinTypeWithFocusClickFallback(
		"hello",
		true,
		func() error {
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
		t.Fatalf("darwinTypeWithFocusClickFallback() error = %v", err)
	}
	if strings.Join(steps, ",") != "after_focus,send" {
		t.Fatalf("steps = %v, want after_focus,send", steps)
	}
}

func TestDarwinSendTextWithClipboardFallback_PrefersClipboardPaste(t *testing.T) {
	clipboardCalls := 0
	unicodeCalls := 0

	err := darwinSendTextWithClipboardFallback(
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
		t.Fatalf("darwinSendTextWithClipboardFallback() error = %v", err)
	}
	if clipboardCalls != 1 {
		t.Fatalf("clipboardCalls = %d, want 1", clipboardCalls)
	}
	if unicodeCalls != 0 {
		t.Fatalf("unicodeCalls = %d, want 0", unicodeCalls)
	}
}

func TestDarwinSendTextWithClipboardFallback_FallsBackToUnicodeInput(t *testing.T) {
	clipboardCalls := 0
	unicodeCalls := 0

	err := darwinSendTextWithClipboardFallback(
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
		t.Fatalf("darwinSendTextWithClipboardFallback() error = %v", err)
	}
	if clipboardCalls != 1 {
		t.Fatalf("clipboardCalls = %d, want 1", clipboardCalls)
	}
	if unicodeCalls != 1 {
		t.Fatalf("unicodeCalls = %d, want 1", unicodeCalls)
	}
}

func TestDarwinHighlightInputBounds_CallsOverlayWhenBoundsExist(t *testing.T) {
	highlightCalls := 0

	darwinHighlightInputBounds(
		darwinRect{
			Origin: darwinPoint{X: 10, Y: 20},
			Size:   darwinSize{Width: 200, Height: 32},
		},
		true,
		func(bounds darwinRect, duration time.Duration) error {
			highlightCalls++
			if bounds.Origin.X != 10 || bounds.Origin.Y != 20 {
				t.Fatalf("bounds origin = %+v, want {10 20}", bounds.Origin)
			}
			if bounds.Size.Width != 200 || bounds.Size.Height != 32 {
				t.Fatalf("bounds size = %+v, want {200 32}", bounds.Size)
			}
			if duration != darwinInputHighlightDuration {
				t.Fatalf("duration = %v, want %v", duration, darwinInputHighlightDuration)
			}
			return nil
		},
	)
	if highlightCalls != 1 {
		t.Fatalf("highlightCalls = %d, want 1", highlightCalls)
	}
}

func TestDarwinHighlightInputBounds_SkipsOverlayWithoutBounds(t *testing.T) {
	highlightCalls := 0

	darwinHighlightInputBounds(
		darwinRect{},
		false,
		func(darwinRect, time.Duration) error {
			highlightCalls++
			return nil
		},
	)
	if highlightCalls != 0 {
		t.Fatalf("highlightCalls = %d, want 0", highlightCalls)
	}
}

func TestDarwinVerifySemanticTextEntry_PrefersAXValueReadback(t *testing.T) {
	captureCalls := 0
	ocrCalls := 0

	ok := darwinVerifySemanticTextEntry(
		nil,
		"你好，Orca",
		func() string { return "你好，Orca" },
		darwinRect{},
		false,
		func(_ context.Context, _ darwinRect) ([]byte, error) {
			captureCalls++
			return []byte("png"), nil
		},
		func(_ context.Context, _ []byte) (string, error) {
			ocrCalls++
			return "你好，Orca", nil
		},
	)
	if !ok {
		t.Fatal("darwinVerifySemanticTextEntry() = false, want true")
	}
	if captureCalls != 0 {
		t.Fatalf("captureCalls = %d, want 0", captureCalls)
	}
	if ocrCalls != 0 {
		t.Fatalf("ocrCalls = %d, want 0", ocrCalls)
	}
}

func TestDarwinVerifySemanticTextEntry_FallsBackToOCR(t *testing.T) {
	captureCalls := 0
	ocrCalls := 0

	ok := darwinVerifySemanticTextEntry(
		nil,
		"你好，Orca",
		func() string { return "" },
		darwinRect{
			Origin: darwinPoint{X: 10, Y: 20},
			Size:   darwinSize{Width: 240, Height: 36},
		},
		true,
		func(_ context.Context, bounds darwinRect) ([]byte, error) {
			captureCalls++
			if bounds.Size.Width != 240 {
				t.Fatalf("bounds size = %+v, want width 240", bounds.Size)
			}
			return []byte("png"), nil
		},
		func(_ context.Context, imagePNG []byte) (string, error) {
			ocrCalls++
			if string(imagePNG) != "png" {
				t.Fatalf("imagePNG = %q, want png", string(imagePNG))
			}
			return "你好 Orca", nil
		},
	)
	if !ok {
		t.Fatal("darwinVerifySemanticTextEntry() = false, want true")
	}
	if captureCalls != 1 {
		t.Fatalf("captureCalls = %d, want 1", captureCalls)
	}
	if ocrCalls != 1 {
		t.Fatalf("ocrCalls = %d, want 1", ocrCalls)
	}
}

func TestDarwinVerifySemanticTextEntry_ReturnsFalseWhenVerificationFails(t *testing.T) {
	ok := darwinVerifySemanticTextEntry(
		nil,
		"hello orca",
		func() string { return "draft" },
		darwinRect{},
		false,
		nil,
		nil,
	)
	if ok {
		t.Fatal("darwinVerifySemanticTextEntry() = true, want false")
	}
}
