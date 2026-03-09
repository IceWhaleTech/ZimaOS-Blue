package smallmodel

import (
	"context"
	"strings"
	"testing"
)

func TestManagerDefaults(t *testing.T) {
	m := NewManager(t.TempDir())
	st := m.GetStatus()
	if st.ModelID != ModelID {
		t.Fatalf("model id mismatch: got %q want %q", st.ModelID, ModelID)
	}
	if st.Runtime != RuntimeType {
		t.Fatalf("runtime mismatch: got %q want %q", st.Runtime, RuntimeType)
	}
	if st.Ready {
		t.Fatal("expected model not ready in temp dir")
	}
}

func TestEnsureReadyNoAutoDownload(t *testing.T) {
	m := NewManager(t.TempDir())
	err := m.EnsureReady(context.Background(), false)
	if err == nil {
		t.Fatal("expected error when model missing and auto download disabled")
	}
	if !strings.Contains(err.Error(), "auto download is disabled") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestManagerDefaultDownloadsUseCurrentGGUFAsset(t *testing.T) {
	m := NewManager(t.TempDir())
	files := requiredModelFiles(m.assets)
	if len(files) != 1 {
		t.Fatalf("requiredModelFiles() len = %d, want 1", len(files))
	}
	if got := files[0].Filename; got != defaultModelFilename {
		t.Fatalf("model filename = %q, want %q", got, defaultModelFilename)
	}
	if got := files[0].URL; !strings.Contains(got, "Qwen3.5-0.8B.Q4_K_M.gguf") {
		t.Fatalf("model URL = %q, want canonical GGUF filename", got)
	}
	if m.MMProjPath() != "" {
		t.Fatalf("MMProjPath() = %q, want empty by default", m.MMProjPath())
	}
}

func TestLegacyModelAssetConfigStillUsesFallbackCandidates(t *testing.T) {
	assets := modelAssetConfig{
		repo:          "Qwen/Qwen3.5-0.8B-GGUF",
		modelFilename: "qwen3.5-0.8b-q4_k_m.gguf",
	}
	if !usesDefaultModelDownloadCandidates(assets) {
		t.Fatal("expected legacy repo/filename pair to use fallback candidates")
	}
	file := modelDownloadFile(assets)
	if !strings.Contains(file.URL, "Qwen3.5-0.8B.Q4_K_M.gguf") {
		t.Fatalf("fallback URL = %q, want canonical GGUF candidate first", file.URL)
	}
}
