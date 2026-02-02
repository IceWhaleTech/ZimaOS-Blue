package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLayeredMemoryService(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "layered-memory-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create service
	svc, err := NewLayeredMemoryService(nil, LayeredMemoryConfig{
		BaseDir:            tmpDir,
		DailyRetentionDays: 7,
	})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	ctx := context.Background()

	// Test AppendToDaily
	t.Run("AppendToDaily", func(t *testing.T) {
		err := svc.AppendToDaily(ctx, "Test memory content", []string{"test", "memory"})
		if err != nil {
			t.Errorf("AppendToDaily failed: %v", err)
		}

		// Verify file exists
		today := time.Now().Format("2006-01-02")
		logPath := filepath.Join(tmpDir, "daily", today+".md")
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			t.Error("daily log file was not created")
		}
	})

	// Test ListDailyLogs
	t.Run("ListDailyLogs", func(t *testing.T) {
		dates, err := svc.ListDailyLogs(ctx)
		if err != nil {
			t.Errorf("ListDailyLogs failed: %v", err)
		}
		if len(dates) == 0 {
			t.Error("expected at least one daily log")
		}
	})

	// Test GetDailyLog
	t.Run("GetDailyLog", func(t *testing.T) {
		today := time.Now().Format("2006-01-02")
		content, err := svc.GetDailyLog(ctx, today)
		if err != nil {
			t.Errorf("GetDailyLog failed: %v", err)
		}
		if content == "" {
			t.Error("expected non-empty content")
		}
		if !strings.Contains(content, "Test memory content") {
			t.Error("content should contain the appended text")
		}
	})

	// Test PromoteToLongTerm
	t.Run("PromoteToLongTerm", func(t *testing.T) {
		err := svc.PromoteToLongTerm(ctx, "Important knowledge to remember", "Knowledge")
		if err != nil {
			t.Errorf("PromoteToLongTerm failed: %v", err)
		}

		// Verify file exists
		longTermPath := filepath.Join(tmpDir, "MEMORY.md")
		if _, err := os.Stat(longTermPath); os.IsNotExist(err) {
			t.Error("long-term memory file was not created")
		}
	})

	// Test GetLongTermMemory
	t.Run("GetLongTermMemory", func(t *testing.T) {
		content, err := svc.GetLongTermMemory(ctx)
		if err != nil {
			t.Errorf("GetLongTermMemory failed: %v", err)
		}
		if !strings.Contains(content, "Important knowledge to remember") {
			t.Error("long-term memory should contain promoted content")
		}
	})

	// Test GetConfig
	t.Run("GetConfig", func(t *testing.T) {
		cfg := svc.GetConfig()
		if cfg.DailyRetentionDays != 7 {
			t.Errorf("expected retention days 7, got %d", cfg.DailyRetentionDays)
		}
	})
}

func TestLayeredMemoryService_PruneDailyLogs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "layered-memory-prune-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	svc, err := NewLayeredMemoryService(nil, LayeredMemoryConfig{
		BaseDir:            tmpDir,
		DailyRetentionDays: 1, // Only keep 1 day
	})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	ctx := context.Background()
	dailyDir := filepath.Join(tmpDir, "daily")

	// Create an old log file (2 days ago)
	oldDate := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	oldLogPath := filepath.Join(dailyDir, oldDate+".md")
	if err := os.WriteFile(oldLogPath, []byte("# Old log"), 0644); err != nil {
		t.Fatalf("failed to create old log: %v", err)
	}

	// Create today's log
	err = svc.AppendToDaily(ctx, "Today's content", nil)
	if err != nil {
		t.Fatalf("failed to append to daily: %v", err)
	}

	// Prune
	deleted, err := svc.PruneDailyLogs(ctx)
	if err != nil {
		t.Errorf("PruneDailyLogs failed: %v", err)
	}

	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	// Verify old log is gone
	if _, err := os.Stat(oldLogPath); !os.IsNotExist(err) {
		t.Error("old log should have been deleted")
	}
}

func TestMemoryExtractor(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "memory-extractor-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	svc, err := NewLayeredMemoryService(nil, LayeredMemoryConfig{
		BaseDir:            tmpDir,
		DailyRetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	ctx := context.Background()
	dailyDir := filepath.Join(tmpDir, "daily")

	// Create an old log file with important content
	oldDate := time.Now().AddDate(0, 0, -3).Format("2006-01-02")
	oldLogContent := `# Daily Log - ` + oldDate + `

## 10:30:00

**Tags:** decision, important

We decided to use the new API design. This is a critical decision for the project.

---

## 14:00:00

**Tags:** task

Remember to fix the bug in the login flow. This is an important task.

---

## 16:00:00

The weather is nice today.

---
`
	oldLogPath := filepath.Join(dailyDir, oldDate+".md")
	if err := os.WriteFile(oldLogPath, []byte(oldLogContent), 0644); err != nil {
		t.Fatalf("failed to create old log: %v", err)
	}

	// Create extractor
	extractor := NewMemoryExtractor(svc)
	extractor.SetMinScore(0.3) // Lower threshold for testing

	// Extract from logs older than 2 days
	promoted, err := extractor.ExtractFromDailyLogs(ctx, 2)
	if err != nil {
		t.Errorf("ExtractFromDailyLogs failed: %v", err)
	}

	if promoted == 0 {
		t.Error("expected at least one promoted entry")
	}

	// Verify long-term memory was created
	longTermContent, err := svc.GetLongTermMemory(ctx)
	if err != nil {
		t.Errorf("GetLongTermMemory failed: %v", err)
	}

	if longTermContent == "" {
		t.Error("expected non-empty long-term memory")
	}
}

func TestMemoryExtractor_ParseLogEntries(t *testing.T) {
	extractor := &MemoryExtractor{
		scorer:   NewImportanceScorer(DefaultImportanceConfig()),
		minScore: 0.5,
	}

	content := `# Daily Log - 2026-02-03

## 10:30:00

**Tags:** test, important

This is the first entry content.

---

## 14:00:00

Second entry without tags.

---
`

	entries := extractor.parseLogEntries(content)

	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}

	if len(entries) > 0 {
		if !strings.Contains(entries[0].content, "first entry") {
			t.Error("first entry content mismatch")
		}
		if len(entries[0].tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(entries[0].tags))
		}
	}

	if len(entries) > 1 {
		if !strings.Contains(entries[1].content, "Second entry") {
			t.Error("second entry content mismatch")
		}
	}
}

func TestMemoryExtractor_InferCategory(t *testing.T) {
	extractor := &MemoryExtractor{}

	tests := []struct {
		content  string
		tags     []string
		expected string
	}{
		{"Some content", []string{"decision"}, "Decisions"},
		{"Some content", []string{"task"}, "Tasks"},
		{"We decided to use Go", nil, "Decisions"},
		{"Fixed the bug", nil, "Technical"},
		{"Random content", nil, "General"},
	}

	for _, tt := range tests {
		result := extractor.inferCategory(tt.content, tt.tags)
		if result != tt.expected {
			t.Errorf("inferCategory(%q, %v) = %q, want %q", tt.content, tt.tags, result, tt.expected)
		}
	}
}
