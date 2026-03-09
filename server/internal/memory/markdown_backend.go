package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// PureMarkdownBackend implements MemoryBackend using only Markdown files.
// No SQLite database required - all data stored as human-readable Markdown.
//
// Storage layout:
//   - daily/<date>.md  — append-only daily log (one per day, pruned after 30 days)
//   - MEMORY.md        — curated long-term knowledge (managed by LayeredMemoryService)
type PureMarkdownBackend struct {
	store   *MarkdownMemoryStore
	baseDir string
}

// NewPureMarkdownBackend creates a new pure Markdown backend.
func NewPureMarkdownBackend(baseDir string) (*PureMarkdownBackend, error) {
	if baseDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		baseDir = filepath.Join(homeDir, ".zimaos-blue", "memory")
	}

	// Ensure daily directory exists
	if err := os.MkdirAll(filepath.Join(baseDir, "daily"), 0755); err != nil {
		return nil, err
	}

	return &PureMarkdownBackend{
		store:   NewMarkdownMemoryStore(baseDir),
		baseDir: baseDir,
	}, nil
}

func (b *PureMarkdownBackend) Name() string {
	return "markdown"
}

// Remember appends a memory entry to today's daily log.
func (b *PureMarkdownBackend) Remember(ctx context.Context, content string, tags []string) (*MemoryChunk, error) {
	now := timeutil.NowTime()
	relID := filepath.Join("daily", now.Format("2006-01-02")+".md")
	dailyPath := filepath.Join(b.baseDir, relID)

	f, err := os.OpenFile(dailyPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, _ := f.Stat()
	if info.Size() == 0 {
		f.WriteString(fmt.Sprintf("# Daily Log - %s\n\n", now.Format("2006-01-02")))
	}

	f.WriteString(fmt.Sprintf("## %s\n\n", now.Format("15:04:05")))
	if len(tags) > 0 {
		f.WriteString(fmt.Sprintf("**Tags:** %s\n\n", strings.Join(tags, ", ")))
	}
	f.WriteString(content + "\n\n---\n\n")

	return &MemoryChunk{
		ID:        relID,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Recall searches memories using keyword matching.
func (b *PureMarkdownBackend) Recall(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	mdResults, err := b.store.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	results := make([]SearchResult, len(mdResults))
	for i, r := range mdResults {
		results[i] = SearchResult{
			Chunk: MemoryChunk{
				ID:      r.FilePath,
				Content: r.Content,
			},
			Score:        r.Score,
			KeywordScore: r.Score,
			MatchTypes:   []string{r.MatchType},
		}
	}
	return results, nil
}

// Forget removes a daily log file by relative path.
func (b *PureMarkdownBackend) Forget(ctx context.Context, id string) error {
	path := filepath.Join(b.baseDir, id)
	if !strings.HasPrefix(path, b.baseDir) {
		return fmt.Errorf("invalid path")
	}
	return os.Remove(path)
}

// ForgetAll removes all daily log files.
func (b *PureMarkdownBackend) ForgetAll(ctx context.Context) error {
	dailyDir := filepath.Join(b.baseDir, "daily")
	if err := os.RemoveAll(dailyDir); err != nil {
		return err
	}
	return os.MkdirAll(dailyDir, 0755)
}

// Get reads a memory file.
func (b *PureMarkdownBackend) Get(ctx context.Context, id string) (*MemoryChunk, error) {
	content, err := b.store.ReadFile(ctx, id, 0, 0)
	if err != nil {
		return nil, err
	}
	return &MemoryChunk{
		ID:      id,
		Content: content,
	}, nil
}

// Prune removes old daily logs.
func (b *PureMarkdownBackend) Prune(ctx context.Context) (int, error) {
	// Keep last 30 days by default
	cutoff := timeutil.NowTime().AddDate(0, 0, -30)
	dailyDir := filepath.Join(b.baseDir, "daily")
	deleted := 0

	files, err := os.ReadDir(dailyDir)
	if err != nil {
		return 0, nil
	}

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		dateStr := strings.TrimSuffix(f.Name(), ".md")
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil || date.After(cutoff) {
			continue
		}
		if os.Remove(filepath.Join(dailyDir, f.Name())) == nil {
			deleted++
		}
	}
	return deleted, nil
}

// Stats returns memory statistics.
func (b *PureMarkdownBackend) Stats(ctx context.Context) (*MemoryStats, error) {
	files, err := b.store.ListFiles(ctx)
	if err != nil {
		return nil, err
	}

	var totalSize int64
	var oldest, newest time.Time
	for _, f := range files {
		totalSize += f.Size
		if oldest.IsZero() || f.ModifiedAt.Before(oldest) {
			oldest = f.ModifiedAt
		}
		if newest.IsZero() || f.ModifiedAt.After(newest) {
			newest = f.ModifiedAt
		}
	}

	return &MemoryStats{
		TotalChunks:    len(files),
		TotalSizeBytes: totalSize,
		OldestChunk:    oldest.Format(time.RFC3339),
		NewestChunk:    newest.Format(time.RFC3339),
		Backend:        "markdown",
	}, nil
}
