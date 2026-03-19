//go:build darwin

package convert

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func requireCommand(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s unavailable: %v", name, err)
	}
}

func waitForTaskResult(t *testing.T, svc *Service, taskID string) *ConvertTask {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	task, err := svc.WaitForTask(ctx, "user-1", "conv-1", taskID, 20*time.Second)
	if err != nil {
		t.Fatalf("WaitForTask failed: %v", err)
	}
	if task == nil {
		t.Fatal("expected task result")
	}
	return task
}

func TestConvertServiceDarwinTextToRTF(t *testing.T) {
	requireCommand(t, "textutil")

	svc := setupConvertTestService(t)
	sourcePath := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(sourcePath, []byte("hello from convert"), 0o640); err != nil {
		t.Fatalf("write source: %v", err)
	}

	task, err := svc.Submit(context.Background(), "user-1", "conv-1", TaskRequest{
		Action:       ActionConvert,
		Sources:      []string{sourcePath},
		TargetFormat: "rtf",
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	result := waitForTaskResult(t, svc, task.ID)
	if result.Status != StatusSucceeded {
		t.Fatalf("status=%s message=%q error=%q", result.Status, result.Message, result.Error)
	}
	if len(result.Outputs) != 1 {
		t.Fatalf("outputs=%d, want 1", len(result.Outputs))
	}
	if got := filepath.Ext(result.Outputs[0].Name); got != ".rtf" {
		t.Fatalf("output ext=%q, want .rtf", got)
	}
	if result.Outputs[0].SizeBytes <= 0 {
		t.Fatalf("size_bytes=%d, want >0", result.Outputs[0].SizeBytes)
	}
}

func TestConvertServiceDarwinMarkdownToTextAlias(t *testing.T) {
	requireCommand(t, "textutil")

	svc := setupConvertTestService(t)
	sourcePath := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(sourcePath, []byte("# hello\n\nfrom markdown"), 0o640); err != nil {
		t.Fatalf("write source: %v", err)
	}

	task, err := svc.Submit(context.Background(), "user-1", "conv-1", TaskRequest{
		Action:       ActionConvert,
		Sources:      []string{sourcePath},
		TargetFormat: "text",
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	result := waitForTaskResult(t, svc, task.ID)
	if result.Status != StatusSucceeded {
		t.Fatalf("status=%s message=%q error=%q", result.Status, result.Message, result.Error)
	}
	if len(result.Outputs) != 1 {
		t.Fatalf("outputs=%d, want 1", len(result.Outputs))
	}
	if got := filepath.Ext(result.Outputs[0].Name); got != ".txt" {
		t.Fatalf("output ext=%q, want .txt", got)
	}
	if result.Outputs[0].SizeBytes <= 0 {
		t.Fatalf("size_bytes=%d, want >0", result.Outputs[0].SizeBytes)
	}
}

func TestConvertServiceDarwinTTSM4A(t *testing.T) {
	requireCommand(t, "say")
	requireCommand(t, "afconvert")

	svc := setupConvertTestService(t)
	task, err := svc.Submit(context.Background(), "user-1", "conv-1", TaskRequest{
		Action: ActionTTS,
		Text:   "Hello from Blue convert tests.",
		Options: TaskOptions{
			Speech: SpeechOptions{
				TTS: TTSSpeechOptions{Format: "m4a"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	result := waitForTaskResult(t, svc, task.ID)
	if result.Status != StatusSucceeded {
		t.Fatalf("status=%s message=%q error=%q", result.Status, result.Message, result.Error)
	}
	if len(result.Outputs) != 1 {
		t.Fatalf("outputs=%d, want 1", len(result.Outputs))
	}
	output := result.Outputs[0]
	if !strings.HasSuffix(output.Name, ".m4a") {
		t.Fatalf("output name=%q, want .m4a suffix", output.Name)
	}
	if output.PreviewKind != PreviewAudio {
		t.Fatalf("preview_kind=%q, want %q", output.PreviewKind, PreviewAudio)
	}
	if output.MimeType != "audio/mp4" {
		t.Fatalf("mime_type=%q, want audio/mp4", output.MimeType)
	}
	if output.SizeBytes <= 0 {
		t.Fatalf("size_bytes=%d, want >0", output.SizeBytes)
	}
}
