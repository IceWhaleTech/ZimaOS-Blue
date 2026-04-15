//go:build darwin

package a11y

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDarwinSystemCLI_ActivateAppPrefersOpen(t *testing.T) {
	var calls []string
	cli := darwinSystemCLI{
		run: func(_ context.Context, name string, args ...string) (string, error) {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return "", nil
		},
	}

	if err := cli.activateApp("Feishu"); err != nil {
		t.Fatalf("activateApp() error = %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("calls = %v, want one open call", calls)
	}
	if calls[0] != "open -a Feishu" {
		t.Fatalf("first call = %q, want %q", calls[0], "open -a Feishu")
	}
}

func TestDarwinSystemCLI_ActivateAppFallsBackToRestrictedAppleScript(t *testing.T) {
	var calls []string
	cli := darwinSystemCLI{
		run: func(_ context.Context, name string, args ...string) (string, error) {
			calls = append(calls, name+" "+strings.Join(args, " "))
			if name == "open" {
				return "open failed", errors.New("exit status 1")
			}
			return "", nil
		},
	}

	if err := cli.activateApp(`App "Quote"`); err != nil {
		t.Fatalf("activateApp() error = %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %v, want open + osascript", calls)
	}
	if calls[0] != `open -a App "Quote"` {
		t.Fatalf("first call = %q", calls[0])
	}
	if !strings.HasPrefix(calls[1], "osascript -e tell application ") {
		t.Fatalf("second call = %q, want osascript activation fallback", calls[1])
	}
	if !strings.Contains(calls[1], `App \"Quote\"`) {
		t.Fatalf("second call = %q, want escaped app name", calls[1])
	}
}

func TestDarwinSystemCLI_ActivateAppReturnsFallbackFailure(t *testing.T) {
	cli := darwinSystemCLI{
		run: func(_ context.Context, name string, args ...string) (string, error) {
			if name == "open" {
				return "open failed", errors.New("exit status 1")
			}
			return "script failed", errors.New("exit status 2")
		},
	}

	err := cli.activateApp("Feishu")
	if err == nil {
		t.Fatal("activateApp() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), `activate app "Feishu" failed`) {
		t.Fatalf("error = %v, want app context", err)
	}
	if !strings.Contains(err.Error(), "script failed") {
		t.Fatalf("error = %v, want osascript output", err)
	}
}

func TestDarwinSystemCLI_CaptureWindowUsesScreencapture(t *testing.T) {
	var call string
	cli := darwinSystemCLI{
		run: func(_ context.Context, name string, args ...string) (string, error) {
			call = name + " " + strings.Join(args, " ")
			return "", nil
		},
	}

	if _, err := cli.captureWindow(context.Background(), "6263", "/tmp/window.png"); err != nil {
		t.Fatalf("captureWindow() error = %v", err)
	}
	if call != "screencapture -x -l 6263 /tmp/window.png" {
		t.Fatalf("call = %q, want %q", call, "screencapture -x -l 6263 /tmp/window.png")
	}
}

func TestDarwinSystemCLI_PasteTextWithTemporaryClipboardUsesClipboardRestoreFlow(t *testing.T) {
	var name string
	var args []string
	cli := darwinSystemCLI{
		run: func(_ context.Context, command string, commandArgs ...string) (string, error) {
			name = command
			args = append([]string(nil), commandArgs...)
			return "", nil
		},
	}

	if _, err := cli.pasteTextWithTemporaryClipboard(context.Background(), "hello\nit's me"); err != nil {
		t.Fatalf("pasteTextWithTemporaryClipboard() error = %v", err)
	}
	if name != "osascript" {
		t.Fatalf("name = %q, want osascript", name)
	}
	if len(args) < 11 {
		t.Fatalf("args len = %d, want at least 11", len(args))
	}
	joined := strings.Join(args, " ")
	for _, needle := range []string{
		`-e on run argv`,
		`-e set oldClipboard to the clipboard`,
		`-e set the clipboard to item 1 of argv`,
		`-e tell application "System Events" to keystroke "v" using command down`,
		`-e set the clipboard to oldClipboard`,
		`-- hello`,
	} {
		if !strings.Contains(joined, needle) {
			t.Fatalf("args missing %q in %q", needle, joined)
		}
	}
	if args[len(args)-1] != "hello\nit's me" {
		t.Fatalf("last arg = %q, want original text payload", args[len(args)-1])
	}
}

func TestDarwinSystemCLI_OpenAccessibilitySettingsUsesRestrictedOpen(t *testing.T) {
	var call string
	cli := darwinSystemCLI{
		run: func(_ context.Context, name string, args ...string) (string, error) {
			call = name + " " + strings.Join(args, " ")
			return "", nil
		},
	}

	if err := cli.openAccessibilitySettings(); err != nil {
		t.Fatalf("openAccessibilitySettings() error = %v", err)
	}
	if call != "open x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility" {
		t.Fatalf("call = %q, want settings deep link open", call)
	}
}

func TestDarwinSystemCLI_ShowHighlightOverlayUsesMaskedJavaScriptOverlay(t *testing.T) {
	var name string
	var args []string
	cli := darwinSystemCLI{
		run: func(_ context.Context, command string, commandArgs ...string) (string, error) {
			name = command
			args = append([]string(nil), commandArgs...)
			return "", nil
		},
	}

	err := cli.showHighlightOverlay(
		context.Background(),
		darwinRect{
			Origin: darwinPoint{X: 120, Y: 240},
			Size:   darwinSize{Width: 320, Height: 48},
		},
		350*time.Millisecond,
	)
	if err != nil {
		t.Fatalf("showHighlightOverlay() error = %v", err)
	}
	if name != "osascript" {
		t.Fatalf("name = %q, want osascript", name)
	}
	joined := strings.Join(args, " ")
	for _, needle := range []string{
		`-l JavaScript`,
		`ObjC.import('QuartzCore')`,
		`NSWindow.alloc.initWithContentRectStyleMaskBackingDefer`,
		`setBackgroundColor($.NSColor.colorWithCalibratedWhiteAlpha(0.0, 0.28).CGColor)`,
		`bezierPathWithRoundedRectXRadiusYRadius`,
		`setFillRule($.kCAFillRuleEvenOdd)`,
		`setStrokeColor($.NSColor.systemBlueColor.CGColor)`,
		`setIgnoresMouseEvents(true)`,
		`-- 120 240 320 48 0.35`,
	} {
		if !strings.Contains(joined, needle) {
			t.Fatalf("args missing %q in %q", needle, joined)
		}
	}
}

func TestDarwinSystemCLI_CaptureRegionUsesScreencaptureRect(t *testing.T) {
	var call string
	cli := darwinSystemCLI{
		run: func(_ context.Context, name string, args ...string) (string, error) {
			call = name + " " + strings.Join(args, " ")
			return "", nil
		},
	}

	_, err := cli.captureRegion(
		context.Background(),
		darwinRect{
			Origin: darwinPoint{X: 12, Y: 34},
			Size:   darwinSize{Width: 320, Height: 48},
		},
		"/tmp/region.png",
	)
	if err != nil {
		t.Fatalf("captureRegion() error = %v", err)
	}
	if call != "screencapture -x -R12,34,320,48 /tmp/region.png" {
		t.Fatalf("call = %q, want %q", call, "screencapture -x -R12,34,320,48 /tmp/region.png")
	}
}
