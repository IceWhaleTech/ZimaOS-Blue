package a11y

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type windowsSystemCLI struct {
	run func(ctx context.Context, name string, args ...string) (string, error)
}

var windowsCLIFallback = windowsSystemCLI{
	run: windowsRunSystemCLICommand,
}

func windowsRunSystemCLICommand(ctx context.Context, name string, args ...string) (string, error) {
	var cmd *exec.Cmd
	if ctx != nil {
		cmd = exec.CommandContext(ctx, name, args...)
	} else {
		cmd = exec.Command(name, args...)
	}
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func (cli windowsSystemCLI) exec(ctx context.Context, name string, args ...string) (string, error) {
	if cli.run != nil {
		return cli.run(ctx, name, args...)
	}
	return windowsRunSystemCLICommand(ctx, name, args...)
}

func (cli windowsSystemCLI) captureWindow(ctx context.Context, width int, height int, left int32, top int32, path string) (string, error) {
	psPath := strings.ReplaceAll(path, `'`, `''`)
	command := fmt.Sprintf("Add-Type -AssemblyName System.Drawing; $bmp=New-Object System.Drawing.Bitmap(%d,%d); $g=[System.Drawing.Graphics]::FromImage($bmp); $g.CopyFromScreen(%d,%d,0,0,$bmp.Size); $bmp.Save('%s',[System.Drawing.Imaging.ImageFormat]::Png); $g.Dispose(); $bmp.Dispose()", width, height, left, top, psPath)
	return cli.exec(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", command)
}

func (cli windowsSystemCLI) captureRegion(ctx context.Context, rect windowsRect, path string) (string, error) {
	width, height, ok := windowsVisibleRectSize(rect)
	if !ok {
		return "", fmt.Errorf("capture region requires visible bounds")
	}
	return cli.captureWindow(ctx, width, height, rect.Left, rect.Top, path)
}

func (cli windowsSystemCLI) captureActiveWindow(ctx context.Context, path string) (string, error) {
	psPath := windowsPSEscapeLiteral(path)
	command := strings.Join([]string{
		"Add-Type -AssemblyName System.Windows.Forms",
		"Add-Type -AssemblyName System.Drawing",
		"$ws=New-Object -ComObject WScript.Shell",
		"$backup=$null",
		"try { $backup=[System.Windows.Forms.Clipboard]::GetDataObject() } catch { $backup=$null }",
		"try {",
		"$ws.SendKeys('%{PRTSC}')",
		"Start-Sleep -Milliseconds 150",
		"$img=[System.Windows.Forms.Clipboard]::GetImage()",
		"if ($null -eq $img) { throw 'active window screenshot clipboard image unavailable' }",
		fmt.Sprintf("$img.Save('%s',[System.Drawing.Imaging.ImageFormat]::Png)", psPath),
		"} finally {",
		"if ($backup -ne $null) { [System.Windows.Forms.Clipboard]::SetDataObject($backup,$true) } else { [System.Windows.Forms.Clipboard]::Clear() }",
		"}",
	}, "; ")
	return cli.exec(ctx, "powershell", "-NoProfile", "-NonInteractive", "-STA", "-Command", command)
}

func (cli windowsSystemCLI) pasteTextWithTemporaryClipboard(ctx context.Context, text string) (string, error) {
	psText := windowsPSEscapeLiteral(text)
	command := strings.Join([]string{
		"Add-Type -AssemblyName System.Windows.Forms",
		"$ws=New-Object -ComObject WScript.Shell",
		"$backup=$null",
		"try { $backup=[System.Windows.Forms.Clipboard]::GetDataObject() } catch { $backup=$null }",
		"try {",
		fmt.Sprintf("[System.Windows.Forms.Clipboard]::SetText('%s')", psText),
		"Start-Sleep -Milliseconds 50",
		"$ws.SendKeys('^v')",
		"Start-Sleep -Milliseconds 50",
		"} finally {",
		"if ($backup -ne $null) { [System.Windows.Forms.Clipboard]::SetDataObject($backup,$true) } else { [System.Windows.Forms.Clipboard]::Clear() }",
		"}",
	}, "; ")
	return cli.exec(ctx, "powershell", "-NoProfile", "-NonInteractive", "-STA", "-Command", command)
}

func windowsPSEscapeLiteral(value string) string {
	return strings.ReplaceAll(value, `'`, `''`)
}
