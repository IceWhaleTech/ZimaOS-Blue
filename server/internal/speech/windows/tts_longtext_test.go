//go:build windows && cgo
// +build windows,cgo

package windows

import (
	"context"
	"strings"
	"testing"
)

// TestWindowsTTSProvider_LongText tests synthesis with long text
func TestWindowsTTSProvider_LongText(t *testing.T) {
	provider := NewWindowsTTSProvider()
	if provider == nil {
		t.Fatal("Failed to create TTS provider")
	}
	defer provider.Close()

	ctx := context.Background()

	// Test with a long text similar to what might come from a real request
	longText := `这是一个用户注册登录的流程图，用 Mermaid 语法绘制：

flowchart TD
    A([开始]) --> B{新用户?}
    B -->|是| C[填写注册信息]
    B -->|否| D[输入用户名密码]
    C --> E[验证信息]
    E -->|失败| C
    E -->|成功| F[创建账户]
    F --> G[登录成功]
    D --> H[验证凭证]
    H -->|失败| I[显示错误]
    I --> D
    H -->|成功| G
    G --> J([结束])`

	req := &SynthesizeRequest{
		Text:   longText,
		Voice:  "",
		Speed:  1.0,
		Pitch:  1.0,
		Volume: 1.0,
	}

	resp, err := provider.Synthesize(ctx, req)
	if err != nil {
		t.Fatalf("Long text synthesis failed: %v", err)
	}
	if len(resp.Audio) == 0 {
		t.Fatal("Empty audio for long text")
	}
	t.Logf("Long text: generated %d bytes", len(resp.Audio))
}

// TestWindowsTTSProvider_VeryLongText tests with extremely long text
func TestWindowsTTSProvider_VeryLongText(t *testing.T) {
	provider := NewWindowsTTSProvider()
	if provider == nil {
		t.Fatal("Failed to create TTS provider")
	}
	defer provider.Close()

	ctx := context.Background()

	// Create a very long text by repeating
	baseText := "This is a test sentence for text to speech synthesis. "
	veryLongText := strings.Repeat(baseText, 50) // ~2500 characters

	req := &SynthesizeRequest{
		Text:   veryLongText,
		Voice:  "",
		Speed:  1.0,
		Pitch:  1.0,
		Volume: 1.0,
	}

	resp, err := provider.Synthesize(ctx, req)
	if err != nil {
		t.Fatalf("Very long text synthesis failed: %v", err)
	}
	if len(resp.Audio) == 0 {
		t.Fatal("Empty audio for very long text")
	}
	t.Logf("Very long text (%d chars): generated %d bytes", len(veryLongText), len(resp.Audio))
}

// TestWindowsTTSProvider_SpecialCharacters tests with special characters
func TestWindowsTTSProvider_SpecialCharacters(t *testing.T) {
	provider := NewWindowsTTSProvider()
	if provider == nil {
		t.Fatal("Failed to create TTS provider")
	}
	defer provider.Close()

	ctx := context.Background()

	testCases := []struct {
		name string
		text string
	}{
		{"Punctuation", "Hello, world! How are you? I'm fine, thank you."},
		{"Numbers", "The year is 2026, and the time is 15:38."},
		{"Mixed", "User123 logged in at 2026-02-21 15:38:15 from IP 192.168.1.1"},
		{"Code", "function test() { return 'hello'; }"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &SynthesizeRequest{
				Text:   tc.text,
				Voice:  "",
				Speed:  1.0,
				Pitch:  1.0,
				Volume: 1.0,
			}

			resp, err := provider.Synthesize(ctx, req)
			if err != nil {
				t.Fatalf("Synthesis failed for %s: %v", tc.name, err)
			}
			if len(resp.Audio) == 0 {
				t.Fatalf("Empty audio for %s", tc.name)
			}
			t.Logf("%s: generated %d bytes", tc.name, len(resp.Audio))
		})
	}
}
