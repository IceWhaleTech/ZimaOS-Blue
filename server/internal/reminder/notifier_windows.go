//go:build windows

package reminder

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"go.uber.org/zap"
)

type windowsNotifier struct {
	logger *zap.Logger
}

// NewNotifier returns a Windows notifier that uses PowerShell toast notifications.
func NewNotifier(logger *zap.Logger) Notifier {
	return &windowsNotifier{logger: logger.Named("notifier")}
}

func (n *windowsNotifier) Notify(ctx context.Context, title, body string) error {
	ps := fmt.Sprintf(
		`[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null; `+
			`$template = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02); `+
			`$textNodes = $template.GetElementsByTagName('text'); `+
			`$textNodes.Item(0).AppendChild($template.CreateTextNode('%s')) | Out-Null; `+
			`$textNodes.Item(1).AppendChild($template.CreateTextNode('%s')) | Out-Null; `+
			`$toast = [Windows.UI.Notifications.ToastNotification]::new($template); `+
			`[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('Blue').Show($toast)`,
		escapePowerShell(title), escapePowerShell(body),
	)

	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		n.logger.Warn("PowerShell toast notification failed",
			zap.String("output", string(out)), zap.Error(err))
		return err
	}
	return nil
}

func escapePowerShell(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
