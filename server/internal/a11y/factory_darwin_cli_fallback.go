//go:build darwin

package a11y

import (
	"context"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var darwinActivateAppFrontmostWait = 1500 * time.Millisecond
var darwinActivateAppFrontmostPollInterval = 200 * time.Millisecond

type darwinSystemCLI struct {
	run func(ctx context.Context, name string, args ...string) (string, error)
}

var darwinCLIFallback = darwinSystemCLI{
	run: darwinRunSystemCLICommand,
}

func darwinRunSystemCLICommand(ctx context.Context, name string, args ...string) (string, error) {
	var cmd *exec.Cmd
	if ctx != nil {
		cmd = exec.CommandContext(ctx, name, args...)
	} else {
		cmd = exec.Command(name, args...)
	}
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func (cli darwinSystemCLI) exec(ctx context.Context, name string, args ...string) (string, error) {
	if cli.run != nil {
		return cli.run(ctx, name, args...)
	}
	return darwinRunSystemCLICommand(ctx, name, args...)
}

func (cli darwinSystemCLI) activateApp(appName string) error {
	appName = strings.TrimSpace(appName)
	if appName == "" {
		return nil
	}
	if _, err := cli.exec(nil, "open", "-a", appName); err == nil {
		if !darwinActivationNeedsFrontmostOpenRetry(appName) {
			return nil
		}
		if cli.waitForFrontmostAppName(nil, appName, darwinActivateAppFrontmostWait) {
			return nil
		}
		if _, err := cli.exec(nil, "open", "-a", appName); err == nil && cli.waitForFrontmostAppName(nil, appName, darwinActivateAppFrontmostWait) {
			return nil
		}
	}
	script := fmt.Sprintf(`tell application "%s" to activate`, darwinEscapeAppleScript(appName))
	if output, err := cli.exec(nil, "osascript", "-e", script); err != nil {
		return fmt.Errorf("activate app %q failed: %s: %w", appName, output, err)
	}
	return nil
}

func (cli darwinSystemCLI) waitForFrontmostAppName(ctx context.Context, appName string, timeout time.Duration) bool {
	if ctx == nil {
		ctx = context.Background()
	}
	deadline := time.Now().Add(timeout)
	for {
		frontmost, err := cli.frontmostAppName(ctx)
		if err == nil && darwinFrontmostAppMatches(appName, frontmost) {
			return true
		}
		if timeout <= 0 || time.Now().After(deadline) {
			return false
		}
		if err := darwinSleepWithContext(ctx, darwinActivateAppFrontmostPollInterval); err != nil {
			return false
		}
	}
}

func (cli darwinSystemCLI) frontmostAppName(ctx context.Context) (string, error) {
	asn, err := cli.exec(ctx, "lsappinfo", "front")
	if err != nil {
		return "", err
	}
	asn = strings.TrimSpace(asn)
	if asn == "" {
		return "", fmt.Errorf("lsappinfo front returned empty app serial number")
	}
	info, err := cli.exec(ctx, "lsappinfo", "info", "-only", "name", asn)
	if err != nil {
		return "", err
	}
	name := darwinParseLSAppInfoDisplayName(info)
	if name == "" {
		return "", fmt.Errorf("lsappinfo info returned empty display name")
	}
	return name, nil
}

func darwinParseLSAppInfoDisplayName(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, `"LSDisplayName"=`) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, `"LSDisplayName"=`))
		value = strings.Trim(value, `"`)
		value = strings.ReplaceAll(value, `\"`, `"`)
		return strings.TrimSpace(value)
	}
	return ""
}

func darwinActivationNeedsFrontmostOpenRetry(appName string) bool {
	return darwinLooksLikeFeishuAlias(appName)
}

func darwinFrontmostAppMatches(targetName string, frontmostName string) bool {
	target := normalizeDarwinActivationAppName(targetName)
	frontmost := normalizeDarwinActivationAppName(frontmostName)
	if target == "" || frontmost == "" {
		return false
	}
	if target == frontmost || strings.Contains(target, frontmost) || strings.Contains(frontmost, target) {
		return true
	}
	return darwinLooksLikeFeishuAlias(target) && darwinLooksLikeFeishuAlias(frontmost)
}

func normalizeDarwinActivationAppName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	value = strings.NewReplacer(" ", "", "-", "", "_", "", ",", "", ".", "", "，", "", "、", "").Replace(value)
	return value
}

func darwinLooksLikeFeishuAlias(value string) bool {
	value = normalizeDarwinActivationAppName(value)
	if value == "" {
		return false
	}
	for _, alias := range []string{"feishu", "飞书", "lark"} {
		if strings.Contains(value, normalizeDarwinActivationAppName(alias)) {
			return true
		}
	}
	return false
}

func (cli darwinSystemCLI) captureWindow(ctx context.Context, windowID string, path string) (string, error) {
	return cli.exec(ctx, "screencapture", "-x", "-l", windowID, path)
}

func (cli darwinSystemCLI) captureRegion(ctx context.Context, bounds darwinRect, path string) (string, error) {
	left := int(math.Round(bounds.Origin.X))
	top := int(math.Round(bounds.Origin.Y))
	width := int(math.Round(bounds.Size.Width))
	height := int(math.Round(bounds.Size.Height))
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	return cli.exec(ctx, "screencapture", "-x", fmt.Sprintf("-R%d,%d,%d,%d", left, top, width, height), path)
}

func (cli darwinSystemCLI) pasteTextWithTemporaryClipboard(ctx context.Context, text string) (string, error) {
	return cli.exec(
		ctx,
		"osascript",
		"-e", "on run argv",
		"-e", "set oldClipboard to the clipboard",
		"-e", "set the clipboard to item 1 of argv",
		"-e", `tell application "System Events" to keystroke "v" using command down`,
		"-e", "delay 0.05",
		"-e", "set the clipboard to oldClipboard",
		"-e", "end run",
		"--",
		text,
	)
}

func (cli darwinSystemCLI) showHighlightOverlay(ctx context.Context, bounds darwinRect, duration time.Duration) error {
	script := `ObjC.import('AppKit');
ObjC.import('QuartzCore');
function run(argv) {
  var x = parseFloat(argv[0]);
  var y = parseFloat(argv[1]);
  var width = Math.max(1, parseFloat(argv[2]));
  var height = Math.max(1, parseFloat(argv[3]));
  var seconds = Math.max(0.1, parseFloat(argv[4]));
  var app = $.NSApplication.sharedApplication;
  app.setActivationPolicy($.NSApplicationActivationPolicyAccessory);
  var screen = $.NSScreen.mainScreen.frame;
  var rect = $.NSMakeRect(0, 0, screen.size.width, screen.size.height);
  var window = $.NSWindow.alloc.initWithContentRectStyleMaskBackingDefer(rect, 0, $.NSBackingStoreBuffered, false);
  window.setOpaque(false);
  window.setBackgroundColor($.NSColor.clearColor);
  window.setLevel($.NSStatusWindowLevel);
  window.setIgnoresMouseEvents(true);
  window.setHasShadow(false);
  var content = $.NSView.alloc.initWithFrame(rect);
  content.setWantsLayer(true);
  var dimLayer = $.CALayer.layer;
  dimLayer.setFrame(rect);
  dimLayer.setBackgroundColor($.NSColor.colorWithCalibratedWhiteAlpha(0.0, 0.28).CGColor);
  content.layer.addSublayer(dimLayer);
  var holeY = screen.size.height - y - height;
  var holeRect = $.NSMakeRect(x, holeY, width, height);
  var outer = $.NSBezierPath.bezierPathWithRect(rect);
  var inner = $.NSBezierPath.bezierPathWithRoundedRectXRadiusYRadius(holeRect, 8.0, 8.0);
  outer.appendBezierPath(inner);
  var maskLayer = $.CAShapeLayer.layer;
  maskLayer.setFillRule($.kCAFillRuleEvenOdd);
  maskLayer.setPath(outer.CGPath);
  dimLayer.setMask(maskLayer);
  var borderLayer = $.CAShapeLayer.layer;
  borderLayer.setPath(inner.CGPath);
  borderLayer.setLineWidth(3.0);
  borderLayer.setFillColor($.NSColor.clearColor.CGColor);
  borderLayer.setStrokeColor($.NSColor.systemBlueColor.CGColor);
  content.layer.addSublayer(borderLayer);
  window.setContentView(content);
  window.orderFrontRegardless();
  $.NSThread.sleepForTimeInterval(seconds);
  window.orderOut(nil);
}`
	_, err := cli.exec(
		ctx,
		"osascript",
		"-l", "JavaScript",
		"-e", script,
		"--",
		strconv.FormatFloat(bounds.Origin.X, 'f', -1, 64),
		strconv.FormatFloat(bounds.Origin.Y, 'f', -1, 64),
		strconv.FormatFloat(bounds.Size.Width, 'f', -1, 64),
		strconv.FormatFloat(bounds.Size.Height, 'f', -1, 64),
		strconv.FormatFloat(duration.Seconds(), 'f', -1, 64),
	)
	if err != nil {
		return fmt.Errorf("show highlight overlay failed: %w", err)
	}
	return nil
}

func (cli darwinSystemCLI) openAccessibilitySettings() error {
	const settingsURL = "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility"
	if output, err := cli.exec(nil, "open", settingsURL); err != nil {
		return fmt.Errorf("open accessibility settings failed: %s: %w", output, err)
	}
	return nil
}

func darwinOpenAccessibilitySettings() error {
	return darwinCLIFallback.openAccessibilitySettings()
}
