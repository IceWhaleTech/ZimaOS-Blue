//go:build windows && cgo
// +build windows,cgo

package windows

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWindowsASRProvider_Create(t *testing.T) {
	tests := []struct {
		name     string
		language string
	}{
		{"English", "en-US"},
		{"Chinese", "zh-CN"},
		{"Japanese", "ja-JP"},
		{"Korean", "ko-KR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewWindowsASRProvider(tt.language)
			require.NotNil(t, provider)
			defer provider.Close()
			assert.NotNil(t, provider.handle)
		})
	}
}

func TestWindowsASRProvider_Recognize(t *testing.T) {
	provider := NewWindowsASRProvider("en-US")
	require.NotNil(t, provider)
	defer provider.Close()

	// Test with placeholder audio data
	audioData := make([]byte, 1024)
	req := &RecognizeRequest{
		Audio:    bytes.NewReader(audioData),
		Language: "en-US",
	}

	resp, err := provider.Recognize(context.Background(), req)

	// Current implementation returns placeholder
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Text)
	assert.GreaterOrEqual(t, resp.Confidence, float32(0.0))
	assert.LessOrEqual(t, resp.Confidence, float32(1.0))
}

func TestWindowsASRProvider_Close(t *testing.T) {
	provider := NewWindowsASRProvider("en-US")
	require.NotNil(t, provider)

	provider.Close()
	assert.Nil(t, provider.handle)

	// Should not panic on double close
	provider.Close()
}
