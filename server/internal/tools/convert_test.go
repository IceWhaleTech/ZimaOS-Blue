package tools

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	convertpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	_ "github.com/mattn/go-sqlite3"
)

func TestParseConvertTaskRequestSupportsNestedCamelCaseArgs(t *testing.T) {
	req, err := parseConvertTaskRequest(map[string]interface{}{
		"input": map[string]interface{}{
			"action":       "tts",
			"targetFormat": "wav",
			"message":      "hello world",
			"taskId":       "task-1",
			"waitMs":       1500,
			"sources":      []interface{}{"/tmp/a.txt", "/tmp/b.txt"},
			"options": map[string]interface{}{
				"speech": map[string]interface{}{
					"tts": map[string]interface{}{
						"format": "mp3",
					},
					"asr": map[string]interface{}{
						"onDeviceOnly": true,
					},
				},
				"video": map[string]interface{}{
					"startMs":         100,
					"endMs":           200,
					"frameIntervalMs": 50,
					"segments": []interface{}{
						map[string]interface{}{"startMs": 1, "endMs": 2},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("parseConvertTaskRequest() error = %v", err)
	}
	if req.Action != string(convertpkg.ActionTTS) {
		t.Fatalf("action = %q, want %q", req.Action, convertpkg.ActionTTS)
	}
	if req.TargetFormat != "mp3" {
		t.Fatalf("target_format = %q, want mp3", req.TargetFormat)
	}
	if req.Text != "hello world" {
		t.Fatalf("text = %q, want hello world", req.Text)
	}
	if req.TaskID != "task-1" || req.WaitMS != 1500 {
		t.Fatalf("unexpected task fields: %#v", req)
	}
	if len(req.Sources) != 2 {
		t.Fatalf("sources = %#v, want 2", req.Sources)
	}
	if !req.Options.Speech.ASR.OnDeviceOnly {
		t.Fatalf("expected on-device-only true")
	}
	if req.Options.Video.StartMS != 100 || req.Options.Video.EndMS != 200 || req.Options.Video.FrameIntervalMS != 50 {
		t.Fatalf("unexpected video options: %#v", req.Options.Video)
	}
	if len(req.Options.Video.Segments) != 1 || req.Options.Video.Segments[0].StartMS != 1 || req.Options.Video.Segments[0].EndMS != 2 {
		t.Fatalf("unexpected segments: %#v", req.Options.Video.Segments)
	}
}

func TestConvertToolRequestLocalPathApprovalAllowAlwaysPersistsDirectory(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "convert_tool_test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	service, err := convertpkg.NewService(db, t.TempDir())
	if err != nil {
		t.Fatalf("new convert service: %v", err)
	}
	defer service.Close()

	dirStore, err := NewDirAllowlistStore(db)
	if err != nil {
		t.Fatalf("new dir allowlist store: %v", err)
	}

	broker := sse.NewBroker()
	approvals := NewApprovalManager(broker)
	tool := NewConvertTool(service, approvals, dirStore)

	localFile := filepath.Join(t.TempDir(), "outside", "input.txt")
	localDir := filepath.Dir(localFile)

	done := make(chan struct{})
	go func() {
		defer close(done)
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if req := approvals.GetPending("default"); req != nil {
				approvals.ResolveApproval(req.ID, ApprovalAllowAlways)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := tool.requestLocalPathApproval(ctx, []string{localFile}); err != nil {
		t.Fatalf("first requestLocalPathApproval() error = %v", err)
	}
	<-done

	if entry := dirStore.Match(localDir); entry == nil {
		t.Fatalf("expected %q to be persisted in dir allowlist", localDir)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel2()
	if err := tool.requestLocalPathApproval(ctx2, []string{localFile}); err != nil {
		t.Fatalf("second requestLocalPathApproval() should skip approval for persisted dir, got %v", err)
	}
}
