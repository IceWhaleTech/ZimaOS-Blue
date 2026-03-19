//go:build !darwin

package speech

import (
	"context"
	"fmt"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
)

const ProviderMacOSNative stt.ProviderType = "macos-native"

// RequestSTTAuthorization is a no-op on non-macOS.
func RequestSTTAuthorization() (int, error) {
	return 0, fmt.Errorf("macOS STT not available on this platform")
}

// CurrentSTTAuthorizationState reports that macOS STT is unavailable on
// non-darwin builds.
func CurrentSTTAuthorizationState() (status int, authErr error, initialized bool) {
	return 0, fmt.Errorf("macOS native STT not available on this platform"), false
}

// RunMainRunLoop is a no-op on non-macOS.
func RunMainRunLoop() {}

// StopMainRunLoop is a no-op on non-macOS.
func StopMainRunLoop() {}

// SubmitToMainThread is a no-op on non-macOS — just runs inline.
func SubmitToMainThread(fn func()) { fn() }

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

// Name returns provider name
func (p *MacOSNativeSTT) Name() string {
	return "macOS Native (unavailable)"
}

// Type returns the provider type
func (p *MacOSNativeSTT) Type() stt.ProviderType {
	return ProviderMacOSNative
}

// Available returns false on non-macOS
func (p *MacOSNativeSTT) Available() bool {
	return false
}

// SupportedFormats returns empty list
func (p *MacOSNativeSTT) SupportedFormats() []stt.AudioFormat {
	return nil
}

// MaxDuration returns zero
func (p *MacOSNativeSTT) MaxDuration() time.Duration {
	return 0
}

// Transcribe returns error
func (p *MacOSNativeSTT) Transcribe(_ context.Context, _ *stt.TranscribeRequest) (*stt.TranscribeResponse, error) {
	return nil, fmt.Errorf("macOS native STT not available on this platform")
}

// TranscribeStream returns error
func (p *MacOSNativeSTT) TranscribeStream(_ context.Context, _ *stt.TranscribeRequest, _ stt.StreamCallback) error {
	return fmt.Errorf("macOS native STT not available on this platform")
}

// Close does nothing
func (p *MacOSNativeSTT) Close() {}

func (p *MacOSNativeSTT) SetRequireOnDevice(_ bool)           {}
func (p *MacOSNativeSTT) RequireOnDevice() bool               { return false }
func (p *MacOSNativeSTT) SupportsOnDevice() bool              { return false }
func (p *MacOSNativeSTT) DictationAvailable() bool            { return false }
func (p *MacOSNativeSTT) OfflineDictationLanguages() []string { return nil }

// OnDeviceUnavailableError indicates on-device recognition failed for the locale.
type OnDeviceUnavailableError struct {
	Locale string
	Detail string
}

func (e *OnDeviceUnavailableError) Error() string {
	return fmt.Sprintf("on-device recognition not available for locale %q: %s", e.Locale, e.Detail)
}

// GetTCCAppName returns empty string on non-macOS.
func GetTCCAppName() string { return "" }
