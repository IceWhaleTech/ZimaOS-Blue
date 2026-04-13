//go:build windows && cgo
// +build windows,cgo

package windows

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWindowsTTSProvider_ListVoices(t *testing.T) {
	provider := NewWindowsTTSProvider()
	require.NotNil(t, provider)

	voices, err := provider.ListVoices(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, voices, "should have at least one voice")

	// Verify voice structure
	for _, v := range voices {
		assert.NotEmpty(t, v.ID)
		assert.NotEmpty(t, v.Name)
		assert.NotEmpty(t, v.Language)
	}
}

func TestWindowsTTSProvider_Synthesize(t *testing.T) {
	provider := NewWindowsTTSProvider()
	require.NotNil(t, provider)
	defer provider.Close()

	ctx := context.Background()
	voices, err := provider.ListVoices(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, voices)

	req := &SynthesizeRequest{
		Text:   "Hello, this is a test.",
		Voice:  voices[0].ID,
		Speed:  1.0,
		Pitch:  1.0,
		Volume: 1.0,
	}

	resp, err := provider.Synthesize(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Audio)
	assert.Equal(t, "audio/wav", resp.ContentType)
	assert.Equal(t, 22050, resp.SampleRate)
}

func TestWindowsTTSProvider_SynthesizeChinese(t *testing.T) {
	provider := NewWindowsTTSProvider()
	require.NotNil(t, provider)
	defer provider.Close()

	ctx := context.Background()
	req := &SynthesizeRequest{
		Text:   "你好，这是一个测试。",
		Speed:  1.0,
		Pitch:  1.0,
		Volume: 1.0,
	}

	resp, err := provider.Synthesize(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Audio)
}

func TestWindowsTTSProvider_SpeedPitchVolume(t *testing.T) {
	provider := NewWindowsTTSProvider()
	require.NotNil(t, provider)
	defer provider.Close()

	ctx := context.Background()
	tests := []struct {
		name   string
		speed  float32
		pitch  float32
		volume float32
	}{
		{"Fast", 1.5, 1.0, 1.0},
		{"Slow", 0.5, 1.0, 1.0},
		{"HighVolume", 1.0, 1.0, 1.5},
		{"LowVolume", 1.0, 1.0, 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &SynthesizeRequest{
				Text:   "Testing parameters.",
				Speed:  tt.speed,
				Pitch:  tt.pitch,
				Volume: tt.volume,
			}

			resp, err := provider.Synthesize(ctx, req)
			require.NoError(t, err)
			assert.NotEmpty(t, resp.Audio)
		})
	}
}

func TestDetectLanguageFromText(t *testing.T) {
	tests := []struct {
		text     string
		expected string
	}{
		{"Hello world", "en"},
		{"你好世界", "zh"},
		{"こんにちは", "ja"},
		{"안녕하세요", "ko"},
		{"", "en"},
		{"Mixed 中文 text", "zh"},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := detectLanguageFromText(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchesLanguage(t *testing.T) {
	tests := []struct {
		voiceLang    string
		detectedLang string
		expected     bool
	}{
		{"en-US", "en", true},
		{"zh-CN", "zh", true},
		{"ja-JP", "ja", true},
		{"en-US", "zh", false},
		{"", "en", false},
		{"en", "", false},
	}

	for _, tt := range tests {
		result := matchesLanguage(tt.voiceLang, tt.detectedLang)
		assert.Equal(t, tt.expected, result)
	}
}

func TestWindowsTTSProvider_AutoVoiceSelection(t *testing.T) {
	provider := NewWindowsTTSProvider()
	require.NotNil(t, provider)
	defer provider.Close()

	ctx := context.Background()
	tests := []struct {
		name string
		text string
	}{
		{"AutoEnglish", "Hello world"},
		{"AutoChinese", "你好世界"},
		{"AutoJapanese", "こんにちは"},
		{"AutoKorean", "안녕하세요"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &SynthesizeRequest{
				Text:   tt.text,
				Voice:  "",
				Speed:  1.0,
				Pitch:  1.0,
				Volume: 1.0,
			}

			resp, err := provider.Synthesize(ctx, req)
			// Some languages may not have voices installed, that's OK
			if err != nil {
				t.Logf("%s: Voice not available (expected if language not installed): %v", tt.name, err)
				return
			}
			assert.NotEmpty(t, resp.Audio)
		})
	}
}

func TestWindowsTTSProvider_Close(t *testing.T) {
	provider := NewWindowsTTSProvider()
	require.NotNil(t, provider)

	provider.Close()
	assert.Nil(t, provider.handle)

	// Should not panic on double close
	provider.Close()
}
