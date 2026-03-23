package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	basetask "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/task"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestImageGenerateAdapterSavesRelativeOutputIntoWorkspaceRoot(t *testing.T) {
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	storage := mediagen.NewMediaStorage(filepath.Join(t.TempDir(), "media"), "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	manager := mediagen.NewManager(storage, nil, "")
	manager.RegisterProvider(mediagen.NewFakeMediaProvider())

	adapter := newImageGenerateAdapter(manager, []string{workspaceDir})
	result, err := adapter(context.Background(), tools.ImageGenerateRequest{
		Prompt:     "draw a cozy robot reading",
		OutputPath: "robot_cafe.png",
		Wait:       true,
	})
	if err != nil {
		t.Fatalf("adapter returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if _, err := os.Stat(filepath.Join(workspaceDir, "robot_cafe.png")); err != nil {
		t.Fatalf("expected image in workspace root: %v", err)
	}
}

func TestImageTaskFromMediaTaskPreservesSlideFallbackMetadata(t *testing.T) {
	result := imageTaskFromMediaTask(&mediagen.MediaTask{
		BaseTask: basetask.BaseTask{Status: mediagen.TaskStatusSucceeded},
		Type:     mediagen.MediaTypeImage,
		Provider: "fallback",
		Model:    mediagen.FallbackModelWebCanvasT2I,
		FallbackInfo: &mediagen.MediaFallbackInfo{
			Used:        true,
			Strategy:    mediagen.FallbackStrategyWebCanvas,
			RenderMode:  "slide",
			TemplateID:  "text_only",
			StylePreset: "nano_slides",
		},
	}, "t2i")
	if result == nil || result.Fallback == nil {
		t.Fatalf("fallback = %#v, want metadata", result)
	}
	if result.Fallback.RenderMode != "slide" {
		t.Fatalf("render_mode = %q, want slide", result.Fallback.RenderMode)
	}
	if result.Fallback.TemplateID != "text_only" {
		t.Fatalf("template_id = %q, want text_only", result.Fallback.TemplateID)
	}
	if result.Fallback.StylePreset != "nano_slides" {
		t.Fatalf("style_preset = %q, want nano_slides", result.Fallback.StylePreset)
	}
}

func TestImageGenerateAdapterSavesAbsoluteWorkspaceOutputIntoWorkspaceRoot(t *testing.T) {
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	storage := mediagen.NewMediaStorage(filepath.Join(t.TempDir(), "media"), "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	manager := mediagen.NewManager(storage, nil, "")
	manager.RegisterProvider(mediagen.NewFakeMediaProvider())

	adapter := newImageGenerateAdapter(manager, []string{workspaceDir})
	result, err := adapter(context.Background(), tools.ImageGenerateRequest{
		Prompt:     "draw a cozy robot reading",
		OutputPath: filepath.Join(workspaceDir, "robot_cafe.png"),
		Wait:       true,
	})
	if err != nil {
		t.Fatalf("adapter returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if got := result.Request["path"]; got != "robot_cafe.png" {
		t.Fatalf("expected saved request path to be relative workspace path, got=%v", got)
	}
	if _, err := os.Stat(filepath.Join(workspaceDir, "robot_cafe.png")); err != nil {
		t.Fatalf("expected image in workspace root: %v", err)
	}
}
