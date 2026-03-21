package scenecompose

import (
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/onnx"
)

func TestU2NetPStatusDefaultsToNotDownloaded(t *testing.T) {
	manager := NewU2NetPModelManager(t.TempDir(), "/api/v1/media/fallback/models/u2netp/status")
	status := manager.GetStatus()
	if status.ModelID != "u2netp" {
		t.Fatalf("model_id = %q, want u2netp", status.ModelID)
	}
	if status.State != "not_downloaded" {
		t.Fatalf("state = %q, want not_downloaded", status.State)
	}
	if status.Ready {
		t.Fatal("expected ready=false before download")
	}
}

func TestU2NetPIdleUnloadClearsSession(t *testing.T) {
	manager := NewU2NetPModelManager(t.TempDir(), "")
	manager.idleTimeout = 10 * time.Millisecond
	manager.session = &onnx.DynamicSession{}
	manager.touchActivity()
	time.Sleep(30 * time.Millisecond)
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.session != nil {
		t.Fatal("expected session to be unloaded after idle timeout")
	}
}
