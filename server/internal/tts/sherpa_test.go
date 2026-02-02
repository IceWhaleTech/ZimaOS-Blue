package tts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSherpaDownloadManager_IsModelComplete(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := &SherpaDownloadManager{ModelDir: tmpDir}

	tests := []struct {
		name      string
		modelType string
		setup     func(string) error
		want      bool
	}{
		{
			name:      "piper-en complete",
			modelType: "piper-en",
			setup: func(dir string) error {
				modelDir := filepath.Join(dir, "vits-piper-en_US-lessac-medium")
				if err := os.MkdirAll(modelDir, 0755); err != nil {
					return err
				}
				// Create required files for Piper
				if err := os.WriteFile(filepath.Join(modelDir, "en_US-lessac-medium.onnx"), []byte("model"), 0644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(modelDir, "config.json"), []byte("{}"), 0644)
			},
			want: true,
		},
		{
			name:      "piper-en missing onnx",
			modelType: "piper-en",
			setup: func(dir string) error {
				modelDir := filepath.Join(dir, "vits-piper-en_US-lessac-medium")
				if err := os.MkdirAll(modelDir, 0755); err != nil {
					return err
				}
				// Only create config.json, missing ONNX
				return os.WriteFile(filepath.Join(modelDir, "config.json"), []byte("{}"), 0644)
			},
			want: false,
		},
		{
			name:      "piper-en missing config",
			modelType: "piper-en",
			setup: func(dir string) error {
				modelDir := filepath.Join(dir, "vits-piper-en_US-lessac-medium")
				if err := os.MkdirAll(modelDir, 0755); err != nil {
					return err
				}
				// Only create ONNX, missing config
				return os.WriteFile(filepath.Join(modelDir, "en_US-lessac-medium.onnx"), []byte("model"), 0644)
			},
			want: false,
		},
		{
			name:      "unknown model type",
			modelType: "unknown",
			setup: func(dir string) error {
				return nil
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			mgr.ModelDir = testDir
			if err := tt.setup(testDir); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			got := mgr.IsModelComplete(tt.modelType)
			if got != tt.want {
				t.Errorf("IsModelComplete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSherpaProvider_VerifyModelFiles(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		modelType string
		setup     func(string) error
		want      bool
	}{
		{
			name:      "piper-en valid",
			modelType: "piper-en",
			setup: func(dir string) error {
				modelDir := filepath.Join(dir, "vits-piper-en_US-lessac-medium")
				if err := os.MkdirAll(modelDir, 0755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(modelDir, "en_US-lessac-medium.onnx"), []byte("model"), 0644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(modelDir, "config.json"), []byte("{}"), 0644)
			},
			want: true,
		},
		{
			name:      "piper-en-hfc valid",
			modelType: "piper-en-hfc",
			setup: func(dir string) error {
				modelDir := filepath.Join(dir, "vits-piper-en_US-hfc_female-medium")
				if err := os.MkdirAll(modelDir, 0755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(modelDir, "en_US-hfc_female-medium.onnx"), []byte("model"), 0644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(modelDir, "config.json"), []byte("{}"), 0644)
			},
			want: true,
		},
		{
			name:      "piper-en incomplete",
			modelType: "piper-en",
			setup: func(dir string) error {
				modelDir := filepath.Join(dir, "vits-piper-en_US-lessac-medium")
				return os.MkdirAll(modelDir, 0755)
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			if err := tt.setup(testDir); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			provider := &SherpaProvider{
				modelDir:    testDir,
				modelType:   tt.modelType,
				downloadMgr: &SherpaDownloadManager{ModelDir: testDir},
			}

			got := provider.verifyModelFiles()
			if got != tt.want {
				t.Errorf("verifyModelFiles() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSherpaProvider_GetModelPath(t *testing.T) {
	tests := []struct {
		name      string
		modelType string
		want      string
	}{
		{
			name:      "piper-en",
			modelType: "piper-en",
			want:      "vits-piper-en_US-lessac-medium",
		},
		{
			name:      "piper-en-hfc",
			modelType: "piper-en-hfc",
			want:      "vits-piper-en_US-hfc_female-medium",
		},
		{
			name:      "piper-de",
			modelType: "piper-de",
			want:      "vits-piper-de_DE-thorsten-medium",
		},
		{
			name:      "piper-es",
			modelType: "piper-es",
			want:      "vits-piper-es_ES-davefx-medium",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &SherpaProvider{
				modelDir:  "/tmp",
				modelType: tt.modelType,
			}

			got := provider.getModelPath()
			if !strings.HasSuffix(got, tt.want) {
				t.Errorf("getModelPath() = %s, want suffix %s", got, tt.want)
			}
		})
	}
}
