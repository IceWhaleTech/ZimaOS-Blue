package bootstrap

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tts"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/voice"
)

type ttsToolAdapter struct {
	speech speech.Service
	voice  voice.Service
}

func (a ttsToolAdapter) Status(_ context.Context) interface{} {
	if a.speech != nil {
		return a.speech.GetStatus()
	}
	return map[string]interface{}{"tts": map[string]interface{}{"ready": false}}
}

func (a ttsToolAdapter) Config(_ context.Context) map[string]interface{} {
	ttsSvc := a.ttsService()
	if ttsSvc == nil {
		return map[string]interface{}{"speed": 1.0, "pitch": 0.0, "volume": 100.0}
	}
	speed, pitch, volume := ttsSvc.GetConfig()
	return map[string]interface{}{
		"speed":    speed,
		"pitch":    pitch,
		"volume":   volume,
		"provider": string(ttsSvc.GetDefaultProvider()),
	}
}

func (a ttsToolAdapter) ListVoices(ctx context.Context) ([]tools.TTSVoiceInfo, error) {
	ttsSvc := a.ttsService()
	if ttsSvc == nil {
		return nil, errors.New("TTS service not available")
	}
	voices, err := ttsSvc.ListVoices(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]tools.TTSVoiceInfo, 0, len(voices))
	for _, voiceInfo := range voices {
		out = append(out, tools.TTSVoiceInfo{
			ID:          voiceInfo.ID,
			Name:        voiceInfo.Name,
			Language:    voiceInfo.Language,
			Gender:      voiceInfo.Gender,
			Description: voiceInfo.Description,
			Provider:    voiceInfo.Provider,
			Quality:     voiceInfo.Quality,
		})
	}
	return out, nil
}

func (a ttsToolAdapter) Synthesize(ctx context.Context, req tools.TTSSynthesizeRequest) (*tools.TTSAudioResult, error) {
	ttsSvc := a.ttsService()
	if ttsSvc == nil {
		return nil, errors.New("TTS service not available")
	}
	speed, pitch, volume := ttsSvc.GetConfig()
	if req.Speed != nil {
		speed = *req.Speed
	}
	if req.Pitch != nil {
		pitch = *req.Pitch
	}
	if req.Volume != nil {
		volume = *req.Volume
	}

	synthReq := &tts.SynthesizeRequest{
		Text:   req.Text,
		Voice:  req.Voice,
		Format: parseTTSAudioFormat(req.Format),
		Speed:  speed,
		Pitch:  pitch,
		Volume: volume,
	}

	provider := tts.ProviderType(strings.TrimSpace(req.Provider))
	if provider == "" {
		provider = ttsSvc.GetDefaultProvider()
	}

	var (
		resp *tts.SynthesizeResponse
		err  error
	)
	if req.Provider != "" {
		resp, err = ttsSvc.SynthesizeWithProvider(ctx, provider, synthReq)
	} else {
		resp, err = ttsSvc.Synthesize(ctx, synthReq)
	}
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Audio == nil {
		return nil, errors.New("TTS synthesis returned no audio")
	}
	defer resp.Audio.Close()
	audio, err := io.ReadAll(resp.Audio)
	if err != nil {
		return nil, err
	}
	return &tools.TTSAudioResult{
		Audio:           audio,
		Format:          string(resp.Format),
		ContentType:     resp.ContentType,
		DurationSeconds: resp.Duration,
		Provider:        string(provider),
		Voice:           req.Voice,
	}, nil
}

func (a ttsToolAdapter) SpeakLocally(ctx context.Context, text string) (bool, error) {
	if a.voice == nil {
		return false, nil
	}
	return a.voice.SpeakLocally(ctx, text)
}

func (a ttsToolAdapter) StopSpeaking(context.Context) error {
	if a.voice == nil {
		return errors.New("voice service not available")
	}
	a.voice.StopSpeaking()
	return nil
}

func (a ttsToolAdapter) ttsService() tts.Service {
	if a.speech == nil {
		return nil
	}
	return a.speech.GetTTSService()
}

func parseTTSAudioFormat(format string) tts.AudioFormat {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "mp3":
		return tts.FormatMP3
	case "opus":
		return tts.FormatOPUS
	case "aac":
		return tts.FormatAAC
	case "flac":
		return tts.FormatFLAC
	case "pcm":
		return tts.FormatPCM
	default:
		return tts.FormatWAV
	}
}
