//go:build windows

package push

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
	// Use ToastGeneric XML template for modern Windows 10/11 toast layout
	xmlContent := fmt.Sprintf(`<toast><visual><binding template="ToastGeneric">`+
		`<text>%s</text>`+
		`<text>%s</text>`+
		`<text placement="attribution">Blue Assistant</text>`+
		`</binding></visual>`+
		`<audio src="ms-winsoundevent:Notification.Reminder"/>`+
		`</toast>`, escapeXML(title), escapeXML(body))

	ps := fmt.Sprintf(
		`[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null; `+
			`[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom, ContentType = WindowsRuntime] | Out-Null; `+
			`$xml = New-Object Windows.Data.Xml.Dom.XmlDocument; `+
			`$xml.LoadXml('%s'); `+
			`$toast = [Windows.UI.Notifications.ToastNotification]::new($xml); `+
			`[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('Blue').Show($toast)`,
		escapePowerShell(xmlContent),
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

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

func escapePowerShell(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
