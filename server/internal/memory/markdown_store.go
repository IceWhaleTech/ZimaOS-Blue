package memory

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// MarkdownMemoryStore provides Markdown-based memory storage for agent queries.
// This allows agents to search and read memories stored as human-readable Markdown files.
type MarkdownMemoryStore struct {
	baseDir     string
	longTermDir string // directory where MEMORY.md lives; defaults to baseDir
}

// MarkdownSearchResult represents a search result from Markdown files.
type MarkdownSearchResult struct {
	FilePath   string   `json:"file_path"`
	FileName   string   `json:"file_name"`
	LineNumber int      `json:"line_number"`
	Content    string   `json:"content"`
	Context    []string `json:"context,omitempty"` // Lines around the match
	Score      float32  `json:"score"`
	MatchType  string   `json:"match_type"` // "exact", "fuzzy", "heading"
}

// NewMarkdownMemoryStore creates a new Markdown memory store.
func NewMarkdownMemoryStore(baseDir string) *MarkdownMemoryStore {
	return &MarkdownMemoryStore{baseDir: baseDir, longTermDir: baseDir}
}

// SetLongTermDir sets the directory where MEMORY.md lives (e.g. workspace root).
func (s *MarkdownMemoryStore) SetLongTermDir(dir string) {
	s.longTermDir = dir
}

// Search searches all Markdown files for the given query.
func (s *MarkdownMemoryStore) Search(ctx context.Context, query string, maxResults int) ([]MarkdownSearchResult, error) {
	if maxResults <= 0 {
		maxResults = 10
	}

	var results []MarkdownSearchResult
	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)

	// Walk through all .md files
	err := filepath.Walk(s.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		fileResults, err := s.searchFile(path, queryLower, queryWords)
		if err != nil {
			return nil // Skip files with errors
		}
		results = append(results, fileResults...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Limit results
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	return results, nil
}

// searchFile searches a single Markdown file.
func (s *MarkdownMemoryStore) searchFile(path string, queryLower string, queryWords []string) ([]MarkdownSearchResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var results []MarkdownSearchResult
	var lines []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	relPath, _ := filepath.Rel(s.baseDir, path)
	fileName := filepath.Base(path)

	for i, line := range lines {
		lineLower := strings.ToLower(line)
		score := float32(0)
		matchType := ""

		// Exact match
		if strings.Contains(lineLower, queryLower) {
			score = 1.0
			matchType = "exact"
		} else {
			// Word match scoring
			matchedWords := 0
			for _, word := range queryWords {
				if strings.Contains(lineLower, word) {
					matchedWords++
				}
			}
			if matchedWords > 0 {
				score = float32(matchedWords) / float32(len(queryWords)) * 0.8
				matchType = "partial"
			}
		}

		// Boost headings
		if strings.HasPrefix(line, "#") && score > 0 {
			score *= 1.2
			matchType = "heading"
		}

		if score > 0.3 {
			// Get context (2 lines before and after)
			contextLines := s.getContext(lines, i, 2)

			results = append(results, MarkdownSearchResult{
				FilePath:   relPath,
				FileName:   fileName,
				LineNumber: i + 1,
				Content:    line,
				Context:    contextLines,
				Score:      score,
				MatchType:  matchType,
			})
		}
	}

	return results, nil
}

// getContext returns lines around the given index.
func (s *MarkdownMemoryStore) getContext(lines []string, index, radius int) []string {
	start := index - radius
	if start < 0 {
		start = 0
	}
	end := index + radius + 1
	if end > len(lines) {
		end = len(lines)
	}

	context := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		if i != index {
			context = append(context, lines[i])
		}
	}
	return context
}

// ReadFile reads a Markdown file by relative path.
func (s *MarkdownMemoryStore) ReadFile(ctx context.Context, relPath string, fromLine, numLines int) (string, error) {
	fullPath := filepath.Join(s.baseDir, relPath)

	// Security check
	if !strings.HasPrefix(fullPath, s.baseDir) {
		return "", fmt.Errorf("invalid path")
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}

	if fromLine <= 0 && numLines <= 0 {
		return string(content), nil
	}

	lines := strings.Split(string(content), "\n")
	if fromLine <= 0 {
		fromLine = 1
	}
	if numLines <= 0 {
		numLines = len(lines)
	}

	start := fromLine - 1
	if start >= len(lines) {
		return "", nil
	}
	end := start + numLines
	if end > len(lines) {
		end = len(lines)
	}

	return strings.Join(lines[start:end], "\n"), nil
}

// ListFiles lists all Markdown files in the memory directory.
func (s *MarkdownMemoryStore) ListFiles(ctx context.Context) ([]FileInfo, error) {
	var files []FileInfo

	err := filepath.Walk(s.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		relPath, _ := filepath.Rel(s.baseDir, path)
		files = append(files, FileInfo{
			Path:       relPath,
			Name:       info.Name(),
			Size:       info.Size(),
			ModifiedAt: info.ModTime(),
		})
		return nil
	})

	// Sort by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModifiedAt.After(files[j].ModifiedAt)
	})

	return files, err
}

// FileInfo holds information about a Markdown file.
type FileInfo struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
}

// GetRecentEntries returns recent entries from daily logs.
func (s *MarkdownMemoryStore) GetRecentEntries(ctx context.Context, days int) ([]DailyEntry, error) {
	if days <= 0 {
		days = 7
	}

	dailyDir := filepath.Join(s.baseDir, "daily")
	var entries []DailyEntry

	cutoff := timeutil.NowTime().AddDate(0, 0, -days)

	files, err := os.ReadDir(dailyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return entries, nil
		}
		return nil, err
	}

	datePattern := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})\.md$`)

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		matches := datePattern.FindStringSubmatch(file.Name())
		if len(matches) != 2 {
			continue
		}

		date, err := time.Parse("2006-01-02", matches[1])
		if err != nil || date.Before(cutoff) {
			continue
		}

		content, err := os.ReadFile(filepath.Join(dailyDir, file.Name()))
		if err != nil {
			continue
		}

		entries = append(entries, DailyEntry{
			Date:    matches[1],
			Content: string(content),
		})
	}

	// Sort by date descending
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Date > entries[j].Date
	})

	return entries, nil
}

// DailyEntry represents a daily log entry.
type DailyEntry struct {
	Date    string `json:"date"`
	Content string `json:"content"`
}

// GetLongTermMemory reads the MEMORY.md file.
func (s *MarkdownMemoryStore) GetLongTermMemory(ctx context.Context) (string, error) {
	path := filepath.Join(s.longTermDir, "MEMORY.md")
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(content), nil
}
