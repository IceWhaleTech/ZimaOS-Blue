package bootstrap

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestPPTServiceUsesFallbackWhenOnlyFallbackModelsExist(t *testing.T) {
	storage := mediagen.NewMediaStorage(filepath.Join(t.TempDir(), "media"), "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	manager := mediagen.NewManager(storage, nil, "zh-CN")
	manager.SetFallbackEngine(mediagen.NewFallbackEngine(mediagen.FallbackConfig{Enabled: true}, storage, nil, nil, "zh-CN"))

	service := newPPTService(manager, storage, nil, nil)
	if service == nil {
		t.Fatal("expected ppt service")
	}

	result, err := service.Generate(context.Background(), tools.PPTRequest{
		Description: "帮我做一页PPT，标题：2026 产品战略；副标题：AI 驱动增长；要点：提升转化率；降低成本；全球化扩张",
		StylePreset: "banana_slides",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Skipped {
		t.Fatalf("result unexpectedly skipped: %#v", result)
	}
	if result.Model != mediagen.FallbackModelWebCanvasT2I {
		t.Fatalf("model = %q, want %q", result.Model, mediagen.FallbackModelWebCanvasT2I)
	}
	if len(result.ImageURLs) == 0 {
		t.Fatalf("image_urls = %#v, want cached fallback output", result.ImageURLs)
	}
}
