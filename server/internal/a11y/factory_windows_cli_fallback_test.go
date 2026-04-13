package a11y

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestWindowsSystemCLI_CaptureWindowUsesRestrictedPowerShell(t *testing.T) {
	var name string
	var args []string
	cli := windowsSystemCLI{
		run: func(_ context.Context, command string, commandArgs ...string) (string, error) {
			name = command
			args = append([]string(nil), commandArgs...)
			return "", nil
		},
	}

	if _, err := cli.captureWindow(context.Background(), 1280, 720, 100, 200, `C:\tmp\window.png`); err != nil {
		t.Fatalf("captureWindow() error = %v", err)
	}
	if name != "powershell" {
		t.Fatalf("name = %q, want powershell", name)
	}
	if len(args) != 4 {
		t.Fatalf("args len = %d, want 4", len(args))
	}
	if strings.Join(args[:3], " ") != "-NoProfile -NonInteractive -Command" {
		t.Fatalf("prefix args = %q, want PowerShell safe flags", strings.Join(args[:3], " "))
	}
	script := args[3]
	for _, needle := range []string{
		"Add-Type -AssemblyName System.Drawing",
		"System.Drawing.Bitmap(1280,720)",
		"CopyFromScreen(100,200,0,0,$bmp.Size)",
		"$bmp.Save('C:\\tmp\\window.png'",
	} {
		if !strings.Contains(script, needle) {
			t.Fatalf("script missing %q in %q", needle, script)
		}
	}
}

func TestWindowsSystemCLI_CaptureWindowEscapesSingleQuotesInPath(t *testing.T) {
	var script string
	cli := windowsSystemCLI{
		run: func(_ context.Context, _ string, args ...string) (string, error) {
			script = args[len(args)-1]
			return "", nil
		},
	}

	if _, err := cli.captureWindow(context.Background(), 10, 20, 1, 2, `C:\tmp\o'hare.png`); err != nil {
		t.Fatalf("captureWindow() error = %v", err)
	}
	if !strings.Contains(script, `$bmp.Save('C:\tmp\o''hare.png'`) {
		t.Fatalf("script = %q, want escaped single quote path", script)
	}
}

func TestWindowsSystemCLI_CaptureWindowReturnsUnderlyingFailure(t *testing.T) {
	cli := windowsSystemCLI{
		run: func(_ context.Context, _ string, _ ...string) (string, error) {
			return "powershell failed", errors.New("exit status 1")
		},
	}

	output, err := cli.captureWindow(context.Background(), 10, 20, 1, 2, `C:\tmp\window.png`)
	if err == nil {
		t.Fatal("captureWindow() error = nil, want failure")
	}
	if output != "powershell failed" {
		t.Fatalf("output = %q, want powershell failed", output)
	}
}

func TestWindowsSystemCLI_CaptureActiveWindowUsesClipboardRestoreFlow(t *testing.T) {
	var name string
	var args []string
	cli := windowsSystemCLI{
		run: func(_ context.Context, command string, commandArgs ...string) (string, error) {
			name = command
			args = append([]string(nil), commandArgs...)
			return "", nil
		},
	}

	if _, err := cli.captureActiveWindow(context.Background(), `C:\tmp\active-window.png`); err != nil {
		t.Fatalf("captureActiveWindow() error = %v", err)
	}
	if name != "powershell" {
		t.Fatalf("name = %q, want powershell", name)
	}
	if len(args) != 5 {
		t.Fatalf("args len = %d, want 5", len(args))
	}
	if strings.Join(args[:4], " ") != "-NoProfile -NonInteractive -STA -Command" {
		t.Fatalf("prefix args = %q, want PowerShell clipboard-safe flags", strings.Join(args[:4], " "))
	}
	script := args[4]
	for _, needle := range []string{
		"Add-Type -AssemblyName System.Windows.Forms",
		"Add-Type -AssemblyName System.Drawing",
		"New-Object -ComObject WScript.Shell",
		"$ws.SendKeys('%{PRTSC}')",
		"[System.Windows.Forms.Clipboard]::GetImage()",
		"$img.Save('C:\\tmp\\active-window.png'",
		"[System.Windows.Forms.Clipboard]::SetDataObject($backup,$true)",
	} {
		if !strings.Contains(script, needle) {
			t.Fatalf("script missing %q in %q", needle, script)
		}
	}
}

func TestWindowsSystemCLI_PasteTextWithTemporaryClipboardUsesClipboardRestoreFlow(t *testing.T) {
	var name string
	var args []string
	cli := windowsSystemCLI{
		run: func(_ context.Context, command string, commandArgs ...string) (string, error) {
			name = command
			args = append([]string(nil), commandArgs...)
			return "", nil
		},
	}

	if _, err := cli.pasteTextWithTemporaryClipboard(context.Background(), "hello\nit's me"); err != nil {
		t.Fatalf("pasteTextWithTemporaryClipboard() error = %v", err)
	}
	if name != "powershell" {
		t.Fatalf("name = %q, want powershell", name)
	}
	if len(args) != 5 {
		t.Fatalf("args len = %d, want 5", len(args))
	}
	if strings.Join(args[:4], " ") != "-NoProfile -NonInteractive -STA -Command" {
		t.Fatalf("prefix args = %q, want PowerShell clipboard-safe flags", strings.Join(args[:4], " "))
	}
	script := args[4]
	for _, needle := range []string{
		"Add-Type -AssemblyName System.Windows.Forms",
		"New-Object -ComObject WScript.Shell",
		"[System.Windows.Forms.Clipboard]::SetText('hello",
		"it''s me')",
		"$ws.SendKeys('^v')",
		"[System.Windows.Forms.Clipboard]::SetDataObject($backup,$true)",
	} {
		if !strings.Contains(script, needle) {
			t.Fatalf("script missing %q in %q", needle, script)
		}
	}
}
