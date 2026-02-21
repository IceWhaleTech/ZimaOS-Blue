// +build !windows

package windows

import (
	"context"
	"fmt"
)

// WindowsTTSProvider stub for non-Windows platforms
type WindowsTTSProvider struct{}

func NewWindowsTTSProvider() *WindowsTTSProvider {
	return nil
}

func (p *WindowsTTSProvider) ListVoices(ctx context.Context) ([]Voice, error) {
	return nil, fmt.Errorf("Windows TTS not available on this platform")
}

func (p *WindowsTTSProvider) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	return nil, fmt.Errorf("Windows TTS not available on this platform")
}

func (p *WindowsTTSProvider) Close() {}
