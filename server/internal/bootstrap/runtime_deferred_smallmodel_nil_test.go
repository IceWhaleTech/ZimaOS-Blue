package bootstrap

import (
	"path/filepath"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
)

func TestBindRuntimeSkillReranker_TypedNilRerankerFallsBackToStandaloneManager(t *testing.T) {
	tmp := t.TempDir()
	settings := &stubRuntimeSkillRerankerSettingsTarget{}

	var autoReranker *agentcore.AutoSkillReranker
	var reranker runtimeSkillRerankerTarget = autoReranker

	bindRuntimeSkillReranker(settings, reranker, runtimeSkillRerankerDefaults{
		dataDir:   tmp,
		modelRepo: "repo/model",
	})

	if settings.manager == nil || settings.managerCalls != 1 {
		t.Fatalf("expected fallback skill-reranker manager to be installed for typed nil reranker, got %#v", settings)
	}
	want := filepath.Join(tmp, "skill-reranker", "model.onnx")
	if got := settings.manager.ModelPath(); got != want {
		t.Fatalf("fallback manager model path = %q, want %q", got, want)
	}
}
