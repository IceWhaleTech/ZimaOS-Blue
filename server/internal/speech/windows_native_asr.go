// +build windows

package speech

import (
	"context"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech/windows"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
)

type windowsNativeASR struct {
	provider *windows.WindowsASRProvider
}

func NewWindowsNativeASR() stt.Provider {
	provider := windows.NewWindowsASRProvider("en-US")
	if provider == nil {
		return nil
	}
	return &windowsNativeASR{provider: provider}
}

func (w *windowsNativeASR) Name() string {
	return "Windows Native"
}

func (w *windowsNativeASR) Type() stt.ProviderType {
	return "windows-native"
}

func (w *windowsNativeASR) Transcribe(ctx context.Context, req *stt.TranscribeRequest) (*stt.TranscribeResponse, error) {
	recognizeReq := &windows.RecognizeRequest{
		Audio:    req.Audio,
		Language: req.Language,
	}
	resp, err := w.provider.Recognize(ctx, recognizeReq)
	if err != nil {
		return nil, err
	}
	return &stt.TranscribeResponse{
		Text:       resp.Text,
		Confidence: float64(resp.Confidence),
		Language:   resp.Language,
	}, nil
}

func (w *windowsNativeASR) TranscribeStream(ctx context.Context, req *stt.TranscribeRequest, callback stt.StreamCallback) error {
	// Windows SAPI doesn't support streaming, fall back to regular transcription
	resp, err := w.Transcribe(ctx, req)
	if err != nil {
		return err
	}
	return callback(resp)
}

func (w *windowsNativeASR) SupportedFormats() []stt.AudioFormat {
	return []stt.AudioFormat{stt.FormatWAV, stt.FormatPCM}
}

func (w *windowsNativeASR) MaxDuration() time.Duration {
	return 60 * time.Second
}

func (w *windowsNativeASR) Close() error {
	if w.provider != nil {
		w.provider.Close()
	}
	return nil
}

// InstalledLanguages returns the list of installed speech recognition languages
func (w *windowsNativeASR) InstalledLanguages() []string {
	return windows.GetInstalledLanguages()
}
