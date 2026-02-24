//go:build !darwin && !linux && !windows

package push

import (
	"context"

	"go.uber.org/zap"
)

type noopNotifier struct{}

// NewNotifier returns a no-op notifier for unsupported platforms.
func NewNotifier(_ *zap.Logger) Notifier {
	return &noopNotifier{}
}

func (n *noopNotifier) Notify(_ context.Context, _, _ string) error {
	return nil
}
