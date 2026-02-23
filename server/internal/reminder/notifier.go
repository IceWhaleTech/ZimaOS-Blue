// Package reminder provides persistent reminder management with SQLite storage.
package reminder

import "context"

// Notifier delivers native OS notifications when reminders fire.
// Platform-specific implementations are selected at compile time via build tags.
type Notifier interface {
	// Notify sends a native OS notification with the given title and body.
	// Implementations should be best-effort: errors are logged but never fatal.
	Notify(ctx context.Context, title, body string) error
}
