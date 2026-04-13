//go:build windows && !cgo

package windows

import (
	"context"
	"fmt"
)

// WindowsTTSProvider is a no-cgo fallback for Windows-targeted builds.
type WindowsTTSProvider struct{}

func NewWindowsTTSProvider() *WindowsTTSProvider {
	return nil
}

func (p *WindowsTTSProvider) ListVoices(ctx context.Context) ([]Voice, error) {
	return nil, fmt.Errorf("Windows TTS requires cgo in this build")
}

func (p *WindowsTTSProvider) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	return nil, fmt.Errorf("Windows TTS requires cgo in this build")
}

func (p *WindowsTTSProvider) Close() {}
