//go:build !windows

package speech

import (
	"context"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
)

// windowsNativeASR is a stub type on non-Windows platforms.
type windowsNativeASR struct{}

func NewWindowsNativeASR() stt.Provider {
	return nil
}

func (w *windowsNativeASR) Name() string                    { return "" }
func (w *windowsNativeASR) Type() stt.ProviderType          { return "" }
func (w *windowsNativeASR) SupportedFormats() []stt.AudioFormat { return nil }
func (w *windowsNativeASR) MaxDuration() time.Duration       { return 0 }
func (w *windowsNativeASR) Close() error                     { return nil }
func (w *windowsNativeASR) InstalledLanguages() []string     { return nil }

func (w *windowsNativeASR) Transcribe(_ context.Context, _ *stt.TranscribeRequest) (*stt.TranscribeResponse, error) {
	return nil, nil
}

func (w *windowsNativeASR) TranscribeStream(_ context.Context, _ *stt.TranscribeRequest, _ stt.StreamCallback) error {
	return nil
}
