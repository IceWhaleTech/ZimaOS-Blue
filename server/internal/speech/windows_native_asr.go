// +build windows

package speech

import (
	"context"
	"log/slog"
	"strings"
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
		// SAPI returns code -11 when no speech is detected, and "no recognition result"
		// when resultText is nil. Treat these as empty transcription, not errors.
		errMsg := err.Error()
		if strings.Contains(errMsg, "code -11") || strings.Contains(errMsg, "no recognition result") {
			slog.Debug("[windows-stt] no speech detected, returning empty text")
			return &stt.TranscribeResponse{Text: "", Language: req.Language}, nil
		}
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
