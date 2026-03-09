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
	extracted := "- User preference: concise output"
	if err := refresher.RefreshMemory(ctx, extracted, "agent:ch:peer"); err != nil {
		t.Fatalf("refresh memory failed: %v", err)
	}
	if err := refresher.RefreshMemory(ctx, extracted, "agent:ch:peer"); err != nil {
		t.Fatalf("refresh memory duplicate failed: %v", err)
	}

	dailyPath := filepath.Join(tmpDir, "daily", timeutil.NowTime().Format("2006-01-02")+".md")
	data, err := os.ReadFile(dailyPath)
	if err != nil {
		t.Fatalf("read daily log failed: %v", err)
	}
	if got := strings.Count(string(data), extracted); got != 1 {
		t.Fatalf("duplicate memory should be deduped, count=%d", got)
	}
}
