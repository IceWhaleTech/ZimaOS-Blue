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
	parsed, err := parseConvertTaskRequest(map[string]interface{}{
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
	req := parsed.TaskRequest
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

func TestParseConvertTaskRequestSupportsSimpleInputOutputPaths(t *testing.T) {
	parsed, err := parseConvertTaskRequest(map[string]interface{}{
		"input":  "docs/report.docx",
		"output": "exports/report.pdf",
	})
	if err != nil {
		t.Fatalf("parseConvertTaskRequest() error = %v", err)
	}
	if !parsed.SimpleMode {
		t.Fatal("expected simple mode to be enabled")
	}
	req := parsed.TaskRequest
	if req.Action != string(convertpkg.ActionConvert) {
		t.Fatalf("action = %q, want %q", req.Action, convertpkg.ActionConvert)
	}
	if len(req.Sources) != 1 || req.Sources[0] != "docs/report.docx" {
		t.Fatalf("sources = %#v, want docs/report.docx", req.Sources)
	}
	if req.OutputPath != "exports/report.pdf" {
		t.Fatalf("output_path = %q, want exports/report.pdf", req.OutputPath)
	}
	if req.TargetFormat != "pdf" {
		t.Fatalf("target_format = %q, want pdf", req.TargetFormat)
	}
}

func TestConvertToolResolveLocalPathsSupportsRelativeWorkspacePaths(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "convert_tool_relative.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	service, err := convertpkg.NewService(db, t.TempDir())
	if err != nil {
		t.Fatalf("new convert service: %v", err)
	}
	defer service.Close()

	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	tool := NewConvertTool(service, nil, nil, []string{workspaceDir})

	paths, err := tool.resolveLocalPaths(context.Background(), []string{"docs/input.md", "exports/output.pdf"})
	if err != nil {
		t.Fatalf("resolveLocalPaths() error = %v", err)
	}
	if got, want := paths[0], filepath.Join(workspaceDir, "docs", "input.md"); got != want {
		t.Fatalf("paths[0] = %q, want %q", got, want)
	}
	if got, want := paths[1], filepath.Join(workspaceDir, "exports", "output.pdf"); got != want {
		t.Fatalf("paths[1] = %q, want %q", got, want)
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

	userID := "user-convert-test"
	broker := sse.NewBroker()
	sub := broker.Subscribe(userID)
	defer broker.Unsubscribe(userID, sub)
	approvals := NewApprovalManager(broker)
	tool := NewConvertTool(service, approvals, dirStore, nil)

	localFile := filepath.Join(t.TempDir(), "outside", "input.txt")
	localDir := filepath.Dir(localFile)

	done := make(chan error, 1)
	ctx, cancel := context.WithTimeout(WithUserID(context.Background(), userID), 5*time.Second)
	defer cancel()
	go func() {
		done <- tool.requestLocalPathApproval(ctx, []string{localFile})
	}()

	deadline := time.After(5 * time.Second)
	var req *ApprovalRequest
	for req == nil {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for approval request")
		default:
			req = approvals.GetPending(userID)
			if req == nil {
				time.Sleep(10 * time.Millisecond)
			}
		}
	}
	if !approvals.ResolveApprovalWithBinding(req.ID, ApprovalAllowAlways, req.BindingHash) {
		t.Fatalf("failed to resolve pending approval: %+v", req)
	}

	if err := <-done; err != nil {
		t.Fatalf("first requestLocalPathApproval() error = %v", err)
	}

	if entry := dirStore.Match(localDir); entry == nil {
		t.Fatalf("expected %q to be persisted in dir allowlist", localDir)
	}

	ctx2, cancel2 := context.WithTimeout(WithUserID(context.Background(), userID), 200*time.Millisecond)
	defer cancel2()
	if err := tool.requestLocalPathApproval(ctx2, []string{localFile}); err != nil {
		t.Fatalf("second requestLocalPathApproval() should skip approval for persisted dir, got %v", err)
	}
}

func TestSimpleConvertTaskResultOmitsTaskIDForSyncResults(t *testing.T) {
	task := &convertpkg.ConvertTask{
		ID:           "task-sync",
		Status:       convertpkg.StatusSucceeded,
		Action:       convertpkg.ActionConvert,
		Message:      "Document converted",
		TargetFormat: "pdf",
	}

	got := simpleConvertTaskResult(task, false)
	if _, ok := got["task_id"]; ok {
		t.Fatalf("task_id should be omitted for sync results: %#v", got)
	}
	if got["async"] != false {
		t.Fatalf("async = %#v, want false", got["async"])
	}
}

func TestSimpleConvertTaskResultIncludesTaskIDForAsyncResults(t *testing.T) {
	task := &convertpkg.ConvertTask{
		ID:           "task-async",
		Status:       convertpkg.StatusProcessing,
		Action:       convertpkg.ActionConvert,
		Message:      "Processing",
		TargetFormat: "pdf",
	}

	got := simpleConvertTaskResult(task, true)
	if got["task_id"] != "task-async" {
		t.Fatalf("task_id = %#v, want task-async", got["task_id"])
	}
	if got["async"] != true {
		t.Fatalf("async = %#v, want true", got["async"])
	}
}

func TestConvertWaitDefaultsAndCapsAreLonger(t *testing.T) {
	if defaultConvertSyncWait != 5*time.Minute {
		t.Fatalf("defaultConvertSyncWait = %v, want %v", defaultConvertSyncWait, 5*time.Minute)
	}
	if maxConvertWait != 300*time.Second {
		t.Fatalf("maxConvertWait = %v, want %v", maxConvertWait, 300*time.Second)
	}
	if got := requestedConvertWait(600_000); got != 300*time.Second {
		t.Fatalf("requestedConvertWait clamp = %v, want %v", got, 300*time.Second)
	}
}
