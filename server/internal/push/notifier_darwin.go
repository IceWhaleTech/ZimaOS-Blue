//go:build darwin

package push

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"go.uber.org/zap"
)

type darwinNotifier struct {
	logger *zap.Logger
}

// NewNotifier returns a macOS notifier that creates Apple Reminders via
// AppleScript and shows a Notification Center banner.
func NewNotifier(logger *zap.Logger) Notifier {
	return &darwinNotifier{logger: logger.Named("notifier")}
}

func (n *darwinNotifier) Notify(ctx context.Context, title, body string) error {
	// 1. Create an Apple Reminder via AppleScript (actual Reminders.app integration)
	n.createAppleReminder(ctx, body)

	// 2. Show Notification Center banner
	script := `display notification "` + escapeAppleScript(body) + `" with title "` + escapeAppleScript(title) + `"`
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		n.logger.Warn("osascript notification failed",
			zap.String("output", string(out)), zap.Error(err))
		return err
	}
	return nil
}

// createAppleReminder adds a reminder to Apple Reminders.app via AppleScript.
func (n *darwinNotifier) createAppleReminder(ctx context.Context, body string) {
	dueDate := time.Now().Format("January 2, 2006 3:04:05 PM")
	script := fmt.Sprintf(`tell application "Reminders"
	set defaultList to default list
	tell defaultList
		make new reminder with properties {name:"%s", due date:date "%s", body:"Created by Blue"}
	end tell
end tell`, escapeAppleScript(body), escapeAppleScript(dueDate))

	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		n.logger.Warn("failed to create Apple Reminder",
			zap.String("output", string(out)), zap.Error(err))
	}
}

func escapeAppleScript(s string) string {
	var buf []byte
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			buf = append(buf, '\\', '\\')
		case '"':
			buf = append(buf, '\\', '"')
		default:
			buf = append(buf, s[i])
		}
	}
	return string(buf)
}
