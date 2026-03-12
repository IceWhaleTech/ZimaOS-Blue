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
	if len(files) != 2 {
		t.Fatalf("requiredModelFiles() len = %d, want 2 (model + mmproj)", len(files))
	}
	if got := files[0].Filename; got != defaultModelFilename {
		t.Fatalf("model filename = %q, want %q", got, defaultModelFilename)
	}
	if got := files[0].URL; !strings.Contains(got, "unsloth/Qwen3.5-0.8B-GGUF/resolve/main/Qwen3.5-0.8B-Q4_K_M.gguf") {
		t.Fatalf("model URL = %q, want unsloth GGUF asset", got)
	}
	hasModelScopeMirror := false
	for _, mirror := range files[0].Mirrors {
		if strings.Contains(mirror, "modelscope.cn/models/") {
			hasModelScopeMirror = true
			break
		}
	}
	if !hasModelScopeMirror {
		t.Fatalf("model mirrors = %v, want modelscope fallback", files[0].Mirrors)
	}
	if strings.Contains(files[0].URL, "/Qwen/Qwen3.5-0.8B-GGUF/") {
		t.Fatalf("model URL = %q, should not default to auth-gated official repo", files[0].URL)
	}
	if got := files[1].Filename; got != defaultMMProjFilename {
		t.Fatalf("mmproj filename = %q, want %q", got, defaultMMProjFilename)
	}
	if got := files[0].Size; got != defaultModelSize {
		t.Fatalf("model size = %q, want %q", got, defaultModelSize)
	}
	if got := files[1].Size; got != defaultMMProjSize {
		t.Fatalf("mmproj size = %q, want %q", got, defaultMMProjSize)
	}
	if got := files[1].URL; !strings.Contains(got, "unsloth/Qwen3.5-0.8B-GGUF/resolve/main/mmproj-F16.gguf") {
		t.Fatalf("mmproj URL = %q, want unsloth mmproj asset", got)
	}
	hasMMProjModelScopeMirror := false
	for _, mirror := range files[1].Mirrors {
		if strings.Contains(mirror, "modelscope.cn/models/") {
			hasMMProjModelScopeMirror = true
			break
		}
	}
	if !hasMMProjModelScopeMirror {
		t.Fatalf("mmproj mirrors = %v, want modelscope fallback", files[1].Mirrors)
	}
	if mmprojPath := m.MMProjPath(); mmprojPath == "" || !strings.HasSuffix(mmprojPath, defaultMMProjFilename) {
		t.Fatalf("MMProjPath() = %q, want suffix %q", mmprojPath, defaultMMProjFilename)
	}
}

func TestNonUnslothAssetConfigUsesDirectRepoURLs(t *testing.T) {
	assets := modelAssetConfig{
		repo:          "Qwen/Qwen3.5-0.8B-GGUF",
		modelFilename: "qwen3.5-0.8b-q4_k_m.gguf",
	}
	if usesDefaultModelDownloadCandidates(assets) {
		t.Fatal("expected non-unsloth repo/filename pair to skip default unsloth-only candidates")
	}
	file := modelDownloadFile(assets)
	if !strings.Contains(file.URL, "huggingface.co/Qwen/Qwen3.5-0.8B-GGUF/resolve/main/qwen3.5-0.8b-q4_k_m.gguf") {
		t.Fatalf("fallback URL = %q, want direct repo URL", file.URL)
	}
	if strings.Contains(file.URL, "unsloth/Qwen3.5-0.8B-GGUF") {
		t.Fatalf("fallback URL = %q, should not force unsloth for explicit non-unsloth repo", file.URL)
	}
}
