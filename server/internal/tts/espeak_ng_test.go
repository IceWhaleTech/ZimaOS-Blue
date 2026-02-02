package tts

import (
	"context"
	"testing"
)

func TestEspeakNGProvider_Synthesize(t *testing.T) {
	provider := NewEspeakNGProvider("/tmp/espeak-ng")

	// Mark English as available for testing
	provider.MarkLanguageAvailable("en", true)

	tests := []struct {
		name    string
		req     *EspeakNGRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid synthesis",
			req: &EspeakNGRequest{
				Text:     "Hello world",
				Language: "en",
				Rate:     150,
				Pitch:    50,
				Volume:   100,
			},
			wantErr: false,
		},
		{
			name: "empty text",
			req: &EspeakNGRequest{
				Text:     "",
				Language: "en",
			},
			wantErr: true,
			errMsg:  "text cannot be empty",
		},
		{
			name: "text too long",
			req: &EspeakNGRequest{
				Text:     string(make([]byte, 5001)),
				Language: "en",
			},
			wantErr: true,
			errMsg:  "text too long",
		},
		{
			name: "language not available",
			req: &EspeakNGRequest{
				Text:     "Hello",
				Language: "zh",
			},
			wantErr: true,
			errMsg:  "language pack not downloaded",
		},
		{
			name: "rate clamping",
			req: &EspeakNGRequest{
				Text:     "Hello",
				Language: "en",
				Rate:     50, // Below minimum
			},
			wantErr: false,
		},
		{
			name: "pitch clamping",
			req: &EspeakNGRequest{
				Text:     "Hello",
				Language: "en",
				Pitch:    150, // Above maximum
			},
			wantErr: false,
		},
		{
			name: "volume clamping",
			req: &EspeakNGRequest{
				Text:     "Hello",
				Language: "en",
				Volume:   150, // Above maximum
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			audio, err := provider.Synthesize(context.Background(), tt.req)

			if (err != nil) != tt.wantErr {
				t.Errorf("Synthesize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" {
				if err == nil || err.Error() != tt.errMsg {
					t.Errorf("Synthesize() error = %v, want %v", err, tt.errMsg)
				}
				return
			}

			if !tt.wantErr && audio == nil {
				t.Error("Synthesize() returned nil audio")
			}
		})
	}
}

func TestEspeakNGProvider_LanguageManagement(t *testing.T) {
	provider := NewEspeakNGProvider("/tmp/espeak-ng")

	// Test marking language available
	provider.MarkLanguageAvailable("en", true)
	if !provider.IsLanguageAvailable("en") {
		t.Error("Language should be available after marking")
	}

	// Test marking language unavailable
	provider.MarkLanguageAvailable("en", false)
	if provider.IsLanguageAvailable("en") {
		t.Error("Language should not be available after unmarking")
	}

	// Test getting all language packs info
	packs := provider.GetAllLanguagePackInfo()
	if len(packs) == 0 {
		t.Error("Should have language pack info")
	}

	// Verify structure
	for _, pack := range packs {
		if pack.Language == "" {
			t.Error("Language pack should have language code")
		}
		if pack.Name == "" {
			t.Error("Language pack should have name")
		}
		if pack.SizeKB == 0 {
			t.Error("Language pack should have size")
		}
	}
}

func TestEspeakNGProvider_AudioDuration(t *testing.T) {
	provider := NewEspeakNGProvider("/tmp/espeak-ng")
	provider.MarkLanguageAvailable("en", true)

	tests := []struct {
		name     string
		text     string
		rate     int
		minBytes int
	}{
		{
			name:     "short text",
			text:     "Hi",
			rate:     150,
			minBytes: 44, // WAV header
		},
		{
			name:     "long text",
			text:     "This is a longer text with more words to test duration calculation",
			rate:     150,
			minBytes: 44,
		},
		{
			name:     "slow rate",
			text:     "Hello",
			rate:     80,
			minBytes: 44,
		},
		{
			name:     "fast rate",
			text:     "Hello",
			rate:     500,
			minBytes: 44,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			audio, err := provider.Synthesize(context.Background(), &EspeakNGRequest{
				Text:     tt.text,
				Language: "en",
				Rate:     tt.rate,
				Pitch:    50,
				Volume:   100,
			})

			if err != nil {
				t.Fatalf("Synthesize() error = %v", err)
			}

			// Read audio data
			buf := make([]byte, 1024*1024)
			n, err := audio.Read(buf)
			if err != nil && err.Error() != "EOF" {
				t.Fatalf("Read() error = %v", err)
			}

			if n < tt.minBytes {
				t.Errorf("Audio size = %d, want >= %d", n, tt.minBytes)
			}

			audio.Close()
		})
	}
}

func TestEspeakNGProvider_SupportedLanguages(t *testing.T) {
	provider := NewEspeakNGProvider("/tmp/espeak-ng")
	langs := provider.SupportedLanguages()

	if len(langs) != 27 {
		t.Errorf("SupportedLanguages() returned %d languages, want 27", len(langs))
	}

	// Check for common languages
	expectedLangs := map[string]bool{
		"en": false,
		"zh": false,
		"ja": false,
		"es": false,
		"fr": false,
	}

	for _, lang := range langs {
		if _, ok := expectedLangs[lang]; ok {
			expectedLangs[lang] = true
		}
	}

	for lang, found := range expectedLangs {
		if !found {
			t.Errorf("Expected language %s not found", lang)
		}
	}
}
