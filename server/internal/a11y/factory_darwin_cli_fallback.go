//go:build darwin

package a11y

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

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
		return nil
	}
	script := fmt.Sprintf(`tell application "%s" to activate`, darwinEscapeAppleScript(appName))
	if output, err := cli.exec(nil, "osascript", "-e", script); err != nil {
		return fmt.Errorf("activate app %q failed: %s: %w", appName, output, err)
	}
	return nil
}

func (cli darwinSystemCLI) captureWindow(ctx context.Context, windowID string, path string) (string, error) {
	return cli.exec(ctx, "screencapture", "-x", "-l", windowID, path)
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
