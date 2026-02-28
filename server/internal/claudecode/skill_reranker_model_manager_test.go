package claudecode

import (
	"strings"
	"testing"
)

func TestSkillRerankerModelManager_ModelFileSources(t *testing.T) {
	mgr := NewSkillRerankerModelManager(t.TempDir(), "cross-encoder/ms-marco-MiniLM-L-6-v2")
	f := mgr.modelFile()
	if !strings.Contains(f.URL, "huggingface.co") {
		t.Fatalf("primary url should be huggingface, got %q", f.URL)
	}
	if len(f.Mirrors) != 2 {
		t.Fatalf("expected 2 mirrors, got %d", len(f.Mirrors))
	}
	if !strings.Contains(f.Mirrors[0], "modelscope.cn") {
		t.Fatalf("mirror[0] should be modelscope, got %q", f.Mirrors[0])
	}
	if !strings.Contains(f.Mirrors[1], "hf-mirror.com") {
		t.Fatalf("mirror[1] should be hf-mirror, got %q", f.Mirrors[1])
	}
}

func TestSkillRerankerModelManager_DefaultRepo(t *testing.T) {
	mgr := NewSkillRerankerModelManager(t.TempDir(), "")
	if mgr.repo != defaultSkillRerankerRepo {
		t.Fatalf("default repo mismatch: got %q want %q", mgr.repo, defaultSkillRerankerRepo)
	}
}

func TestSkillRerankerModelManager_EnsureReady_DisableAutoDownload(t *testing.T) {
	mgr := NewSkillRerankerModelManager(t.TempDir(), defaultSkillRerankerRepo)
	err := mgr.EnsureReady(t.Context(), false)
	if err == nil {
		t.Fatalf("expected error when model missing and auto download disabled")
	}
	if !strings.Contains(err.Error(), "auto download is disabled") {
		t.Fatalf("unexpected error: %v", err)
	}
}
