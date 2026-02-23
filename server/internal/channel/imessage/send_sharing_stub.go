//go:build !darwin

package imessage

import (
	"context"
	"fmt"
)

// sendViaSharingService is not available on non-macOS platforms.
func (c *Channel) sendViaSharingService(_ context.Context, _, _ string) error {
	return fmt.Errorf("NSSharingService fallback only available on macOS")
}
