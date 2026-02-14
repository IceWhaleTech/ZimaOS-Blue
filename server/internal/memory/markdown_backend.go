package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PureMarkdownBackend implements MemoryBackend using only Markdown files.
// No SQLite database required - all data stored as human-readable Markdown.
type PureMarkdownBackend struct {
	store *MarkdownMemoryStore
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

	// Ensure directories exist
	if err := os.MkdirAll(filepath.Join(baseDir, "daily"), 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(baseDir, "memories"), 0755); err != nil {
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

// Remember stores a memory as a Markdown file.
func (b *PureMarkdownBackend) Remember(ctx context.Context, content string, tags []string) (*MemoryChunk, error) {
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	now := time.Now()

	// Build Markdown content
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Memory %s\n\n", id[:8]))
	sb.WriteString(fmt.Sprintf("**Created:** %s\n\n", now.Format("2006-01-02 15:04:05")))
	if len(tags) > 0 {
		sb.WriteString(fmt.Sprintf("**Tags:** %s\n\n", strings.Join(tags, ", ")))
	}
	sb.WriteString("---\n\n")
	sb.WriteString(content)
	sb.WriteString("\n")

	// Save to memories folder
	filename := fmt.Sprintf("%s-%s.md", now.Format("2006-01-02"), id[:8])
	path := filepath.Join(b.baseDir, "memories", filename)

	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		return nil, err
	}

	// Also append to daily log
	dailyPath := filepath.Join(b.baseDir, "daily", now.Format("2006-01-02")+".md")
	b.appendToDaily(dailyPath, content, tags, now)

	return &MemoryChunk{
		ID:        id,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (b *PureMarkdownBackend) appendToDaily(path string, content string, tags []string, t time.Time) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	info, _ := f.Stat()
	if info.Size() == 0 {
		f.WriteString(fmt.Sprintf("# Daily Log - %s\n\n", t.Format("2006-01-02")))
	}

	f.WriteString(fmt.Sprintf("## %s\n\n", t.Format("15:04:05")))
	if len(tags) > 0 {
		f.WriteString(fmt.Sprintf("**Tags:** %s\n\n", strings.Join(tags, ", ")))
	}
	f.WriteString(content + "\n\n---\n\n")
}

// Recall searches memories using keyword matching.
func (b *PureMarkdownBackend) Recall(ctx context.Context, query string, limit int) ([]HybridSearchResult, error) {
	mdResults, err := b.store.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	results := make([]HybridSearchResult, len(mdResults))
	for i, r := range mdResults {
		results[i] = HybridSearchResult{
			Chunk: MemoryChunk{
				ID:      r.FilePath,
				Content: r.Content,
			},
			CombinedScore: r.Score,
			KeywordScore:  r.Score,
			MatchTypes:    []string{r.MatchType},
		}
	}
	return results, nil
}

// Forget removes a memory file.
func (b *PureMarkdownBackend) Forget(ctx context.Context, id string) error {
	path := filepath.Join(b.baseDir, id)
	if !strings.HasPrefix(path, b.baseDir) {
		return fmt.Errorf("invalid path")
	}
	return os.Remove(path)
}

// ForgetAll removes all memory files.
func (b *PureMarkdownBackend) ForgetAll(ctx context.Context) error {
	memoriesDir := filepath.Join(b.baseDir, "memories")
	if err := os.RemoveAll(memoriesDir); err != nil {
		return err
	}
	return os.MkdirAll(memoriesDir, 0755)
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
	cutoff := time.Now().AddDate(0, 0, -30)
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
