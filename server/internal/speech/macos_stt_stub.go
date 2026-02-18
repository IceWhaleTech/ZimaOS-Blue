//go:build !darwin

package speech

import (
	"context"
	"fmt"
)

// MacOSNativeSTT stub for non-macOS systems
type MacOSNativeSTT struct{}

// NewMacOSNativeSTT creates a stub provider
func NewMacOSNativeSTT() *MacOSNativeSTT {
	return &MacOSNativeSTT{}
}

// Initialize returns error on non-macOS
func (p *MacOSNativeSTT) Initialize() error {
	return fmt.Errorf("macOS native STT not available on this platform")
}

// Recognize returns error
func (p *MacOSNativeSTT) Recognize(ctx context.Context, audioPath string) (string, error) {
	return "", fmt.Errorf("macOS native STT not available on this platform")
}

// Name returns provider name
func (p *MacOSNativeSTT) Name() string {
	return "macOS Native (unavailable)"
}

// Available returns false on non-macOS
func (p *MacOSNativeSTT) Available() bool {
	return false
}

// Close does nothing
func (p *MacOSNativeSTT) Close() {}
