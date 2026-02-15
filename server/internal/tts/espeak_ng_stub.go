//go:build !espeak

package tts

import (
	"context"
	"fmt"
	"io"
)

// EspeakNGRequest represents a synthesis request (stub).
type EspeakNGRequest struct {
	Text     string `json:"text"`
	Language string `json:"language"`
	Voice    string `json:"voice"`
	Rate     int    `json:"rate"`
	Pitch    int    `json:"pitch"`
	Volume   int    `json:"volume"`
}

// EspeakNGProvider is a stub when built without espeak tag.
type EspeakNGProvider struct{}

// NewEspeakNGProvider returns a stub provider.
func NewEspeakNGProvider(_ string) *EspeakNGProvider {
	return &EspeakNGProvider{}
}

func (p *EspeakNGProvider) Initialize() error {
	return fmt.Errorf("espeak-ng not available: build with -tags espeak")
}

func (p *EspeakNGProvider) Synthesize(_ context.Context, _ *EspeakNGRequest) (io.ReadCloser, error) {
	return nil, fmt.Errorf("espeak-ng not available: build with -tags espeak")
}

func (p *EspeakNGProvider) Name() string             { return "eSpeak-NG (disabled)" }
func (p *EspeakNGProvider) Type() string              { return "espeak-ng" }
func (p *EspeakNGProvider) SupportedLanguages() []string { return nil }
func (p *EspeakNGProvider) Close()                    {}
