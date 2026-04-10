package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestImageGenerateAdapterPrefersSpecificWorkspaceRootOverGenericTempScope(t *testing.T) {
	tempRoot := filepath.Clean(os.TempDir())
	if tempRoot == "" || !filepath.IsAbs(tempRoot) {
		t.Skip("temp root unavailable")
	}

	workspaceDir := filepath.Join(tempRoot, "pinchbench-image-tool-adapter-test", t.Name())
	if err := os.RemoveAll(workspaceDir); err != nil {
		t.Fatalf("remove workspace: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(filepath.Join(tempRoot, "pinchbench-image-tool-adapter-test"))
		_ = os.Remove(filepath.Join(tempRoot, "robot_cafe.png"))
	})
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	storage := mediagen.NewMediaStorage(filepath.Join(t.TempDir(), "media"), "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	manager := mediagen.NewManager(storage, nil, "")
	manager.RegisterProvider(mediagen.NewFakeMediaProvider())

	ctx := tools.WithFSScope(context.Background(), []string{tempRoot}, map[string]string{"tmp": tempRoot})
	adapter := newImageGenerateAdapter(manager, []string{workspaceDir, tempRoot})
	result, err := adapter(ctx, tools.ImageGenerateRequest{
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
	if _, err := os.Stat(filepath.Join(tempRoot, "robot_cafe.png")); err == nil {
		t.Fatal("expected generic temp root not to receive robot_cafe.png")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat generic temp root file: %v", err)
	}
}

func TestRelativizeImageOutputPathRecognizesSymlinkedWorkspaceAlias(t *testing.T) {
	realWorkspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(realWorkspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	aliasDir := filepath.Join(t.TempDir(), "workspace-alias")
	if err := os.Symlink(realWorkspaceDir, aliasDir); err != nil {
		t.Skipf("symlink unsupported in test environment: %v", err)
	}

	got, ok := relativizeImageOutputPath(filepath.Join(aliasDir, "robot_cafe.png"), []string{realWorkspaceDir})
	if !ok {
		t.Fatal("expected symlinked workspace output path to relativize")
	}
	if got != "robot_cafe.png" {
		t.Fatalf("relative path = %q, want robot_cafe.png", got)
	}
}

func TestRelativizeImageOutputPathDoesNotUseGenericTempRootAsWorkspaceRoot(t *testing.T) {
	tempRoot := filepath.Clean(os.TempDir())
	if tempRoot == "" || !filepath.IsAbs(tempRoot) {
		t.Skip("temp root unavailable")
	}

	outputPath := filepath.Join(tempRoot, "pinchbench-workspace", "robot_cafe.png")
	if got, ok := relativizeImageOutputPath(outputPath, []string{tempRoot}); ok {
		t.Fatalf("expected generic temp root match to stay absolute, got ok=true rel=%q", got)
	}
}

func TestWaitForImageTaskFallsBackToUnscopedLookup(t *testing.T) {
	storage := mediagen.NewMediaStorage(filepath.Join(t.TempDir(), "media"), "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	manager := mediagen.NewManager(storage, nil, "")
	manager.RegisterProvider(mediagen.NewFakeMediaProvider())

	task, err := manager.Generate(context.Background(), &mediagen.MediaRequest{
		Type:   mediagen.MediaTypeImage,
		Prompt: "draw a cozy robot reading",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	waitCtx, cancel := context.WithTimeout(tools.WithUserID(context.Background(), "preview-user"), 2*time.Second)
	defer cancel()

	waited, err := waitForImageTask(waitCtx, manager, task.ID, "preview-user")
	if err != nil {
		t.Fatalf("waitForImageTask returned error: %v", err)
	}
	if waited == nil || waited.ID != task.ID {
		t.Fatalf("waited task = %#v, want id %q", waited, task.ID)
	}
}

func TestImageTaskLookupAdapterFallsBackToUnscopedLookup(t *testing.T) {
	storage := mediagen.NewMediaStorage(filepath.Join(t.TempDir(), "media"), "/api/media/generated")
	if err := storage.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	manager := mediagen.NewManager(storage, nil, "")
	manager.RegisterProvider(mediagen.NewFakeMediaProvider())

	task, err := manager.Generate(context.Background(), &mediagen.MediaRequest{
		Type:   mediagen.MediaTypeImage,
		Prompt: "draw a cozy robot reading",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	lookup := newImageTaskLookupAdapter(manager)
	result, err := lookup(tools.WithUserID(context.Background(), "preview-user"), task.ID)
	if err != nil {
		t.Fatalf("lookup returned error: %v", err)
	}
	if result == nil || result.ID != task.ID {
		t.Fatalf("lookup result = %#v, want id %q", result, task.ID)
	}
}
