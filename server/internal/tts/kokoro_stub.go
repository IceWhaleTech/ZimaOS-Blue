//go:build !kokoro

package tts

import "context"

// KokoroAvailable reports whether Kokoro was compiled in.
func KokoroAvailable() bool { return false }

// kokoroStub is a placeholder when the kokoro build tag is not set.
type kokoroStub struct{}

func NewKokoroProvider(_ string) *kokoroStub { return &kokoroStub{} }

func (k *kokoroStub) Name() string                    { return "Kokoro" }
func (k *kokoroStub) Type() ProviderType               { return ProviderKokoro }
func (k *kokoroStub) Available() bool                   { return false }
func (k *kokoroStub) Synthesize(_ context.Context, _ *SynthesizeRequest) (*SynthesizeResponse, error) {
	return nil, ErrProviderDisabled
}
func (k *kokoroStub) SynthesizeStream(_ context.Context, _ *SynthesizeRequest, _ StreamCallback) error {
	return ErrProviderDisabled
}
func (k *kokoroStub) ListVoices(_ context.Context) ([]Voice, error) { return nil, ErrProviderDisabled }
func (k *kokoroStub) SupportedFormats() []AudioFormat               { return nil }
func (k *kokoroStub) MaxTextLength() int                            { return 0 }
func (k *kokoroStub) GetInitStage() string                          { return "" }
