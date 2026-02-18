//go:build !darwin

package tts

import (
	"context"
	"fmt"
)

// MacOSNativeTTS stub for non-macOS systems
type MacOSNativeTTS struct{}

// NewMacOSNativeTTS creates a stub provider
func NewMacOSNativeTTS() *MacOSNativeTTS {
	return &MacOSNativeTTS{}
}

// Initialize returns error on non-macOS
func (p *MacOSNativeTTS) Initialize() error {
	return fmt.Errorf("macOS native TTS not available on this platform")
}

// Synthesize returns error
func (p *MacOSNativeTTS) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	return nil, fmt.Errorf("macOS native TTS not available on this platform")
}

// SynthesizeStream returns error
func (p *MacOSNativeTTS) SynthesizeStream(ctx context.Context, req *SynthesizeRequest, callback StreamCallback) error {
	return fmt.Errorf("macOS native TTS not available on this platform")
}

// ListVoices returns empty list
func (p *MacOSNativeTTS) ListVoices(ctx context.Context) ([]Voice, error) {
	return []Voice{}, nil
}

// SupportedFormats returns empty list
func (p *MacOSNativeTTS) SupportedFormats() []AudioFormat {
	return []AudioFormat{}
}

// MaxTextLength returns 0
func (p *MacOSNativeTTS) MaxTextLength() int {
	return 0
}

// Name returns provider name
func (p *MacOSNativeTTS) Name() string {
	return "macOS Native (unavailable)"
}

// Type returns provider type
func (p *MacOSNativeTTS) Type() ProviderType {
	return "macos-native"
}

// Close does nothing
func (p *MacOSNativeTTS) Close() {}

// Available returns false on non-macOS systems
func (p *MacOSNativeTTS) Available() bool {
	return false
}
