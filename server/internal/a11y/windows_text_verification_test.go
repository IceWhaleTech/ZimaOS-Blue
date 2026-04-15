package a11y

import (
	"context"
	"testing"
)

func TestWindowsVerifySemanticTextEntry_PrefersAccValueReadback(t *testing.T) {
	captureCalls := 0
	ocrCalls := 0

	ok, method := windowsVerifySemanticTextEntry(
		nil,
		"hello orca",
		func() string { return "hello orca" },
		windowsRect{},
		false,
		func(_ context.Context, _ windowsRect) ([]byte, error) {
			captureCalls++
			return []byte("png"), nil
		},
		func(_ context.Context, _ []byte) (string, error) {
			ocrCalls++
			return "hello orca", nil
		},
	)
	if !ok {
		t.Fatal("windowsVerifySemanticTextEntry() = false, want true")
	}
	if method != "ax_value" {
		t.Fatalf("method = %q, want ax_value", method)
	}
	if captureCalls != 0 {
		t.Fatalf("captureCalls = %d, want 0", captureCalls)
	}
	if ocrCalls != 0 {
		t.Fatalf("ocrCalls = %d, want 0", ocrCalls)
	}
}

func TestWindowsVerifySemanticTextEntry_FallsBackToOCR(t *testing.T) {
	captureCalls := 0
	ocrCalls := 0

	ok, method := windowsVerifySemanticTextEntry(
		nil,
		"你好，Orca",
		func() string { return "" },
		windowsRect{Left: 10, Top: 20, Right: 250, Bottom: 56},
		true,
		func(_ context.Context, bounds windowsRect) ([]byte, error) {
			captureCalls++
			if bounds.Right != 250 || bounds.Bottom != 56 {
				t.Fatalf("bounds = %+v, want right=250 bottom=56", bounds)
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
		t.Fatal("windowsVerifySemanticTextEntry() = false, want true")
	}
	if method != "ocr" {
		t.Fatalf("method = %q, want ocr", method)
	}
	if captureCalls != 1 {
		t.Fatalf("captureCalls = %d, want 1", captureCalls)
	}
	if ocrCalls != 1 {
		t.Fatalf("ocrCalls = %d, want 1", ocrCalls)
	}
}

func TestWindowsVerifySemanticTextEntry_ReturnsFalseWhenVerificationFails(t *testing.T) {
	ok, method := windowsVerifySemanticTextEntry(
		nil,
		"hello orca",
		func() string { return "draft" },
		windowsRect{},
		false,
		nil,
		nil,
	)
	if ok {
		t.Fatal("windowsVerifySemanticTextEntry() = true, want false")
	}
	if method != "" {
		t.Fatalf("method = %q, want empty", method)
	}
}
