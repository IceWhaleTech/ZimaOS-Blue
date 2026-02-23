//go:build linux

package reminder

import (
	"context"
	"os/exec"

	"go.uber.org/zap"
)

type linuxNotifier struct {
	logger        *zap.Logger
	hasNotifySend bool
}

// NewNotifier returns a Linux notifier that uses notify-send if available.
func NewNotifier(logger *zap.Logger) Notifier {
	n := &linuxNotifier{logger: logger.Named("notifier")}
	if _, err := exec.LookPath("notify-send"); err == nil {
		n.hasNotifySend = true
	}
	return n
}

func (n *linuxNotifier) Notify(ctx context.Context, title, body string) error {
	if !n.hasNotifySend {
		return nil
	}

	cmd := exec.CommandContext(ctx, "notify-send", "--app-name=Blue", title, body)
	if out, err := cmd.CombinedOutput(); err != nil {
		n.logger.Warn("notify-send failed",
			zap.String("output", string(out)), zap.Error(err))
		return err
	}
	return nil
}
