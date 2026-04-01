package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

func TestSessionMemoryRefresherStoresAndDedups(t *testing.T) {
	tmpDir := t.TempDir()
	mdBackend, err := memory.NewPureMarkdownBackend(tmpDir)
	if err != nil {
		t.Fatalf("new markdown backend: %v", err)
	}
	unified := memory.NewUnifiedMemoryService(mdBackend)

	handler := NewMemoryHandler()
	handler.SetUnifiedService(unified)
	refresher := NewSessionMemoryRefresher(handler)

	ctx := context.Background()
	extracted := strings.Join([]string{
		"- User preference: concise output",
		"- Uploaded file available at /tmp/session/upload.md",
	}, "\n")
	if err := refresher.RefreshMemory(ctx, extracted, "agent:ch:peer"); err != nil {
		t.Fatalf("refresh memory failed: %v", err)
	}
	if err := refresher.RefreshMemory(ctx, "* user preference: concise output", "agent:ch:peer"); err != nil {
		t.Fatalf("refresh memory duplicate failed: %v", err)
	}

	dailyPath := filepath.Join(tmpDir, "daily", timeutil.NowTime().Format("2006-01-02")+".md")
	data, err := os.ReadFile(dailyPath)
	if err != nil {
		t.Fatalf("read daily log failed: %v", err)
	}
	logText := string(data)
	if strings.Contains(logText, "/tmp/session/upload.md") {
		t.Fatalf("transient upload path should not be persisted: %q", logText)
	}
	if got := strings.Count(logText, "User preference: concise output"); got != 1 {
		t.Fatalf("duplicate memory should be deduped after normalization, count=%d text=%q", got, logText)
	}
}
