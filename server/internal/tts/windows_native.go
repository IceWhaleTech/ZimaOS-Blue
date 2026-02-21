// +build windows

package tts

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech/windows"
)

// WindowsNativeTTSProvider wraps Windows native TTS
type WindowsNativeTTSProvider struct {
	provider      *windows.WindowsTTSProvider
	defaultVoice  string
	defaultFormat AudioFormat
	maxTextLength int
}

// NewWindowsNativeTTSProvider creates a new Windows native TTS provider
func NewWindowsNativeTTSProvider() *WindowsNativeTTSProvider {
	provider := windows.NewWindowsTTSProvider()
	if provider == nil {
		return nil
	}

	return &WindowsNativeTTSProvider{
		provider:      provider,
		defaultFormat: FormatWAV,
		maxTextLength: 5000,
	}
}

func (p *WindowsNativeTTSProvider) Name() string {
	return "Windows Native TTS"
}

func (p *WindowsNativeTTSProvider) Type() ProviderType {
	return ProviderWindowsNative
}

func (p *WindowsNativeTTSProvider) SupportedFormats() []AudioFormat {
	return []AudioFormat{FormatWAV}
}

func (p *WindowsNativeTTSProvider) MaxTextLength() int {
	return p.maxTextLength
}

func (p *WindowsNativeTTSProvider) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	if len(req.Text) > p.maxTextLength {
		return nil, ErrTextTooLong
	}

	winReq := &windows.SynthesizeRequest{
		Text:   req.Text,
		Voice:  req.Voice,
		Speed:  req.Speed,
		Pitch:  req.Pitch,
		Volume: req.Volume,
	}

	resp, err := p.provider.Synthesize(ctx, winReq)
	if err != nil {
		return nil, fmt.Errorf("Windows TTS synthesis failed: %w", err)
	}

	return &SynthesizeResponse{
		Audio:       io.NopCloser(bytes.NewReader(resp.Audio)),
		ContentType: resp.ContentType,
		Format:      FormatWAV,
	}, nil
}

func (p *WindowsNativeTTSProvider) SynthesizeStream(ctx context.Context, req *SynthesizeRequest, callback StreamCallback) error {
	resp, err := p.Synthesize(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Audio.Close()

	data, err := io.ReadAll(resp.Audio)
	if err != nil {
		return fmt.Errorf("failed to read audio: %w", err)
	}
	return callback(data)
}

func (p *WindowsNativeTTSProvider) ListVoices(ctx context.Context) ([]Voice, error) {
	winVoices, err := p.provider.ListVoices(ctx)
	if err != nil {
		return nil, err
	}

	voices := make([]Voice, len(winVoices))
	for i, v := range winVoices {
		voices[i] = Voice{
			ID:       v.ID,
			Name:     v.Name,
			Language: v.Language,
			Gender:   v.Gender,
			Provider: "Windows Native",
			Quality:  "high",
		}
	}
	return voices, nil
}

func (p *WindowsNativeTTSProvider) Close() {
	if p.provider != nil {
		p.provider.Close()
	}
}
