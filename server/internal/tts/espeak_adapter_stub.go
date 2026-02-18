//go:build !espeak

package tts

import (
	"context"
	"fmt"
)

// EspeakNGAdapter is a stub when built without espeak tag.
type EspeakNGAdapter struct{}

// NewEspeakNGAdapter returns a stub adapter.
func NewEspeakNGAdapter(_ string) *EspeakNGAdapter {
	return &EspeakNGAdapter{}
}

func (a *EspeakNGAdapter) Name() string        { return "eSpeak-NG (disabled)" }
func (a *EspeakNGAdapter) Type() ProviderType  { return ProviderEspeakNG }
func (a *EspeakNGAdapter) Available() bool     { return false }

func (a *EspeakNGAdapter) Synthesize(_ context.Context, _ *SynthesizeRequest) (*SynthesizeResponse, error) {
	return nil, fmt.Errorf("espeak-ng not available: build with -tags espeak")
}

func (a *EspeakNGAdapter) SynthesizeStream(_ context.Context, _ *SynthesizeRequest, _ StreamCallback) error {
	return fmt.Errorf("espeak-ng not available: build with -tags espeak")
}

func (a *EspeakNGAdapter) ListVoices(_ context.Context) ([]Voice, error) {
	return nil, fmt.Errorf("espeak-ng not available: build with -tags espeak")
}

func (a *EspeakNGAdapter) SupportedFormats() []AudioFormat { return nil }
func (a *EspeakNGAdapter) MaxTextLength() int              { return 0 }
func (a *EspeakNGAdapter) Close()                          {}
