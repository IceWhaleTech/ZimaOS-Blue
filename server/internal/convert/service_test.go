package convert

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupConvertTestService(t *testing.T) *Service {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "convert-test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	svc, err := NewService(db, tmpDir)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func TestRecordAttachmentAndPromptSummary(t *testing.T) {
	svc := setupConvertTestService(t)
	ref, err := svc.RecordAttachment(context.Background(), "user-1", "conv-1", "note.txt", "text/plain", []byte("hello"))
	if err != nil {
		t.Fatalf("RecordAttachment failed: %v", err)
	}
	if !strings.HasPrefix(ref, "att:") {
		t.Fatalf("ref=%q, want att: prefix", ref)
	}
	task := &ConvertTask{
		ID:             "task-1",
		UserID:         "user-1",
		ConversationID: "conv-1",
		Action:         ActionTTS,
		Status:         StatusSucceeded,
		Outputs: []ConvertOutput{{
			ID:          "out-1",
			Name:        "speech.wav",
			MimeType:    "audio/wav",
			PreviewKind: PreviewAudio,
			Path:        filepath.Join(svc.outputDir("task-1"), "speech.wav"),
		}},
	}
	if err := os.WriteFile(task.Outputs[0].Path, []byte("wav"), 0o640); err != nil {
		t.Fatalf("write output: %v", err)
	}
	if err := svc.store.CreateTask(task); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	summary := svc.BuildPromptSummary(context.Background(), "user-1", "conv-1", 10)
	if !strings.Contains(summary, ref) {
		t.Fatalf("summary missing attachment ref: %s", summary)
	}
	if !strings.Contains(summary, "out:task-1:out-1") {
		t.Fatalf("summary missing output ref: %s", summary)
	}
}

func TestResolveSourceIsolation(t *testing.T) {
	svc := setupConvertTestService(t)
	ref, err := svc.RecordAttachment(context.Background(), "user-1", "conv-1", "note.txt", "text/plain", []byte("hello"))
	if err != nil {
		t.Fatalf("RecordAttachment failed: %v", err)
	}
	resolved, err := svc.resolveSource(context.Background(), "user-1", "conv-1", ref)
	if err != nil {
		t.Fatalf("resolveSource failed: %v", err)
	}
	if resolved.Name != "note.txt" {
		t.Fatalf("resolved.Name=%q, want note.txt", resolved.Name)
	}
	if _, err := svc.resolveSource(context.Background(), "user-2", "conv-1", ref); err == nil {
		t.Fatal("expected isolation error for another user")
	}
	if _, err := svc.resolveSource(context.Background(), "user-1", "conv-2", ref); err == nil {
		t.Fatal("expected isolation error for another conversation")
	}
}

func TestResolveOutputSourceIsolation(t *testing.T) {
	svc := setupConvertTestService(t)
	outputPath := filepath.Join(svc.outputDir("task-out"), "speech.wav")
	if err := os.WriteFile(outputPath, []byte("wav"), 0o640); err != nil {
		t.Fatalf("write output: %v", err)
	}
	task := &ConvertTask{
		ID:             "task-out",
		UserID:         "user-1",
		ConversationID: "conv-1",
		Action:         ActionTTS,
		Status:         StatusSucceeded,
		Outputs: []ConvertOutput{{
			ID:          "out-1",
			Name:        "speech.wav",
			MimeType:    "audio/wav",
			PreviewKind: PreviewAudio,
			Path:        outputPath,
		}},
	}
	if err := svc.store.CreateTask(task); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	resolved, err := svc.resolveSource(context.Background(), "user-1", "conv-1", "out:task-out:out-1")
	if err != nil {
		t.Fatalf("resolveSource failed: %v", err)
	}
	if resolved.Path != outputPath {
		t.Fatalf("resolved.Path=%q, want %q", resolved.Path, outputPath)
	}
	if resolved.Category != "audio" {
		t.Fatalf("resolved.Category=%q, want audio", resolved.Category)
	}
	if _, err := svc.resolveSource(context.Background(), "user-2", "conv-1", "out:task-out:out-1"); err == nil {
		t.Fatal("expected isolation error for another user")
	}
	if _, err := svc.resolveSource(context.Background(), "user-1", "conv-2", "out:task-out:out-1"); err == nil {
		t.Fatal("expected isolation error for another conversation")
	}
}

func TestValidateRequestRules(t *testing.T) {
	tests := []struct {
		name    string
		req     TaskRequest
		wantErr string
	}{
		{
			name:    "missing action",
			req:     TaskRequest{},
			wantErr: "action is required",
		},
		{
			name:    "tts requires text",
			req:     TaskRequest{Action: ActionTTS},
			wantErr: "text is required for tts",
		},
		{
			name:    "asr requires single source",
			req:     TaskRequest{Action: ActionASR, Sources: []string{"a", "b"}},
			wantErr: "asr requires exactly one source",
		},
		{
			name:    "convert requires sources",
			req:     TaskRequest{Action: ActionConvert},
			wantErr: "sources is required",
		},
		{
			name: "tts valid",
			req:  TaskRequest{Action: ActionTTS, Text: "hello"},
		},
		{
			name: "asr valid",
			req:  TaskRequest{Action: ActionASR, Sources: []string{"att:1"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := setupConvertTestService(t).validateRequest(tc.req)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("validateRequest returned error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("validateRequest error = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestCancelTaskUpdatesStatus(t *testing.T) {
	svc := setupConvertTestService(t)
	task := &ConvertTask{
		ID:             "task-cancel",
		UserID:         "user-1",
		ConversationID: "conv-1",
		Action:         ActionConvert,
		Status:         StatusPending,
		Request:        &TaskRequest{Action: ActionConvert},
	}
	if err := svc.store.CreateTask(task); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	cancelled, err := svc.CancelTask(context.Background(), "user-1", "conv-1", task.ID)
	if err != nil {
		t.Fatalf("CancelTask failed: %v", err)
	}
	if cancelled.Status != StatusCancelled {
		t.Fatalf("status=%s, want %s", cancelled.Status, StatusCancelled)
	}
}
