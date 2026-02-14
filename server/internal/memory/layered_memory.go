package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MemoryLayer represents a layer type in the dual-layer architecture.
type MemoryLayer string

const (
	// LayerDaily represents the daily log layer (ephemeral, append-only).
	LayerDaily MemoryLayer = "daily"
	// LayerLongTerm represents the long-term memory layer (curated, persistent).
	LayerLongTerm MemoryLayer = "longterm"
)

// LayeredMemoryConfig holds configuration for the layered memory system.
type LayeredMemoryConfig struct {
	// BaseDir is the root directory for memory files.
	BaseDir string
	// DailyRetentionDays is how many days to keep daily logs (default: 30).
	DailyRetentionDays int
	// AutoPromoteThreshold is the minimum score for auto-promotion to long-term (0-1).
	AutoPromoteThreshold float32
}

// LayeredMemoryService provides a dual-layer memory architecture.
// Layer 1: Daily logs - append-only notes for each day
// Layer 2: Long-term memory - curated persistent knowledge
type LayeredMemoryService struct {
	config         LayeredMemoryConfig
	baseService    *UnifiedMemoryService
	mu             sync.RWMutex
	dailyLogPath   string
	longTermPath   string
}

// NewLayeredMemoryService creates a new layered memory service.
func NewLayeredMemoryService(baseService *UnifiedMemoryService, config LayeredMemoryConfig) (*LayeredMemoryService, error) {
	if config.BaseDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		config.BaseDir = filepath.Join(homeDir, ".zimaos-blue", "memory")
	}

	if config.DailyRetentionDays == 0 {
		config.DailyRetentionDays = 30
	}

	if config.AutoPromoteThreshold == 0 {
		config.AutoPromoteThreshold = 0.8
	}

	svc := &LayeredMemoryService{
		config:       config,
		baseService:  baseService,
		dailyLogPath: filepath.Join(config.BaseDir, "daily"),
		longTermPath: filepath.Join(config.BaseDir, "MEMORY.md"),
	}

	// Ensure directories exist
	if err := os.MkdirAll(svc.dailyLogPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create daily log directory: %w", err)
	}

	return svc, nil
}

// getTodayLogPath returns the path to today's daily log file.
func (s *LayeredMemoryService) getTodayLogPath() string {
	today := time.Now().Format("2006-01-02")
	return filepath.Join(s.dailyLogPath, today+".md")
}

// AppendToDaily appends content to today's daily log.
func (s *LayeredMemoryService) AppendToDaily(ctx context.Context, content string, tags []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logPath := s.getTodayLogPath()

	// Create or open the file
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open daily log: %w", err)
	}
	defer f.Close()

	// Check if file is empty (new file)
	info, _ := f.Stat()
	if info.Size() == 0 {
		// Write header for new daily log
		header := fmt.Sprintf("# Daily Log - %s\n\n", time.Now().Format("2006-01-02"))
		if _, err := f.WriteString(header); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
	}

	// Format entry
	timestamp := time.Now().Format("15:04:05")
	entry := fmt.Sprintf("## %s\n\n", timestamp)
	if len(tags) > 0 {
		entry += "**Tags:** "
		for i, tag := range tags {
			if i > 0 {
				entry += ", "
			}
			entry += tag
		}
		entry += "\n\n"
	}
	entry += content + "\n\n---\n\n"

	if _, err := f.WriteString(entry); err != nil {
		return fmt.Errorf("failed to write entry: %w", err)
	}

	// Also store in the base service for search
	if s.baseService != nil {
		metadata := map[string]string{
			"layer": string(LayerDaily),
			"date":  time.Now().Format("2006-01-02"),
		}
		for i, tag := range tags {
			metadata[fmt.Sprintf("tag_%d", i)] = tag
		}
		// Store but don't fail if base service fails
		_, _ = s.baseService.Remember(ctx, content, tags)
	}

	return nil
}

// PromoteToLongTerm promotes content to the long-term memory layer.
func (s *LayeredMemoryService) PromoteToLongTerm(ctx context.Context, content string, category string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Open or create long-term memory file
	f, err := os.OpenFile(s.longTermPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open long-term memory: %w", err)
	}
	defer f.Close()

	// Check if file is empty
	info, _ := f.Stat()
	if info.Size() == 0 {
		header := "# Long-Term Memory\n\n> Curated knowledge base for ZimaOS-Blue\n\n---\n\n"
		if _, err := f.WriteString(header); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
	}

	// Format entry
	entry := fmt.Sprintf("## %s\n\n", category)
	entry += fmt.Sprintf("*Added: %s*\n\n", time.Now().Format("2006-01-02 15:04"))
	entry += content + "\n\n---\n\n"

	if _, err := f.WriteString(entry); err != nil {
		return fmt.Errorf("failed to write entry: %w", err)
	}

	// Also store in base service with long-term tag
	if s.baseService != nil {
		tags := []string{"longterm", category}
		_, _ = s.baseService.Remember(ctx, content, tags)
	}

	return nil
}

// GetDailyLog reads a specific day's log.
func (s *LayeredMemoryService) GetDailyLog(ctx context.Context, date string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	logPath := filepath.Join(s.dailyLogPath, date+".md")
	content, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no log found for date: %s", date)
		}
		return "", fmt.Errorf("failed to read daily log: %w", err)
	}

	return string(content), nil
}

// GetLongTermMemory reads the long-term memory file.
func (s *LayeredMemoryService) GetLongTermMemory(ctx context.Context) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	content, err := os.ReadFile(s.longTermPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // Empty is OK
		}
		return "", fmt.Errorf("failed to read long-term memory: %w", err)
	}

	return string(content), nil
}

// ListDailyLogs returns a list of available daily log dates.
func (s *LayeredMemoryService) ListDailyLogs(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dailyLogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to list daily logs: %w", err)
	}

	var dates []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if len(name) > 3 && name[len(name)-3:] == ".md" {
			dates = append(dates, name[:len(name)-3])
		}
	}

	return dates, nil
}

// PruneDailyLogs removes daily logs older than retention period.
func (s *LayeredMemoryService) PruneDailyLogs(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -s.config.DailyRetentionDays)
	deleted := 0

	entries, err := os.ReadDir(s.dailyLogPath)
	if err != nil {
		return 0, fmt.Errorf("failed to list daily logs: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if len(name) < 13 || name[len(name)-3:] != ".md" {
			continue
		}

		dateStr := name[:10] // "2006-01-02"
		logDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		if logDate.Before(cutoff) {
			logPath := filepath.Join(s.dailyLogPath, name)
			if err := os.Remove(logPath); err == nil {
				deleted++
			}
		}
	}

	return deleted, nil
}

// GetConfig returns the current configuration.
func (s *LayeredMemoryService) GetConfig() LayeredMemoryConfig {
	return s.config
}

// MemoryExtractor extracts important memories from daily logs to long-term memory.
type MemoryExtractor struct {
	layeredMemory *LayeredMemoryService
	scorer        *ImportanceScorer
	minScore      float32
	maxPerDay     int
}

// NewMemoryExtractor creates a new memory extractor.
func NewMemoryExtractor(layeredMemory *LayeredMemoryService) *MemoryExtractor {
	return &MemoryExtractor{
		layeredMemory: layeredMemory,
		scorer:        NewImportanceScorer(DefaultImportanceConfig()),
		minScore:      0.7,
		maxPerDay:     5,
	}
}

// SetMinScore sets the minimum importance score for promotion.
func (e *MemoryExtractor) SetMinScore(score float32) {
	e.minScore = score
}

// ExtractFromDailyLogs extracts important content from daily logs older than daysOld.
func (e *MemoryExtractor) ExtractFromDailyLogs(ctx context.Context, daysOld int) (int, error) {
	if e.layeredMemory == nil {
		return 0, nil
	}

	dates, err := e.layeredMemory.ListDailyLogs(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to list daily logs: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -daysOld)
	promoted := 0

	for _, dateStr := range dates {
		logDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		if logDate.After(cutoff) {
			continue
		}

		count, err := e.extractFromDay(ctx, dateStr)
		if err == nil {
			promoted += count
		}
	}

	return promoted, nil
}

// extractFromDay extracts important content from a specific day's log.
func (e *MemoryExtractor) extractFromDay(ctx context.Context, date string) (int, error) {
	content, err := e.layeredMemory.GetDailyLog(ctx, date)
	if err != nil {
		return 0, err
	}

	entries := e.parseLogEntries(content)
	if len(entries) == 0 {
		return 0, nil
	}

	type scoredEntry struct {
		content  string
		score    float32
		category string
	}

	var scored []scoredEntry
	for _, entry := range entries {
		chunk := &MemoryChunk{Content: entry.content, CreatedAt: entry.timestamp}
		score := e.scorer.Score(chunk)
		if score >= e.minScore {
			scored = append(scored, scoredEntry{
				content:  entry.content,
				score:    score,
				category: e.inferCategory(entry.content, entry.tags),
			})
		}
	}

	// Sort by score descending
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	promoted := 0
	for i := 0; i < len(scored) && i < e.maxPerDay; i++ {
		if err := e.layeredMemory.PromoteToLongTerm(ctx, scored[i].content, scored[i].category); err == nil {
			promoted++
		}
	}

	return promoted, nil
}

type logEntry struct {
	timestamp time.Time
	content   string
	tags      []string
}

func (e *MemoryExtractor) parseLogEntries(content string) []logEntry {
	var entries []logEntry
	var currentEntry *logEntry
	var contentLines []string

	for _, line := range splitLines(content) {
		if len(line) > 3 && line[:3] == "## " {
			if currentEntry != nil && len(contentLines) > 0 {
				currentEntry.content = joinLines(contentLines)
				entries = append(entries, *currentEntry)
			}
			timeStr := line[3:]
			if len(timeStr) >= 8 {
				if t, err := time.Parse("15:04:05", timeStr[:8]); err == nil {
					currentEntry = &logEntry{timestamp: t}
					contentLines = nil
				}
			}
			continue
		}
		if currentEntry != nil && len(line) > 9 && line[:9] == "**Tags:**" {
			currentEntry.tags = parseTags(line[9:])
			continue
		}
		if line == "---" || line == "" || (len(line) > 0 && line[0] == '#') {
			continue
		}
		if currentEntry != nil {
			contentLines = append(contentLines, line)
		}
	}

	if currentEntry != nil && len(contentLines) > 0 {
		currentEntry.content = joinLines(contentLines)
		entries = append(entries, *currentEntry)
	}

	return entries
}

func (e *MemoryExtractor) inferCategory(content string, tags []string) string {
	for _, tag := range tags {
		switch tag {
		case "session", "conversation":
			return "Conversations"
		case "task", "todo":
			return "Tasks"
		case "decision":
			return "Decisions"
		}
	}
	contentLower := toLower(content)
	if containsWord(contentLower, "decided") || containsWord(contentLower, "agreed") {
		return "Decisions"
	}
	if containsWord(contentLower, "bug") || containsWord(contentLower, "fix") {
		return "Technical"
	}
	return "General"
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	result := lines[0]
	for i := 1; i < len(lines); i++ {
		result += "\n" + lines[i]
	}
	return result
}

func parseTags(s string) []string {
	var tags []string
	current := ""
	for _, c := range s {
		if c == ',' {
			if tag := trimSpace(current); tag != "" {
				tags = append(tags, tag)
			}
			current = ""
		} else {
			current += string(c)
		}
	}
	if tag := trimSpace(current); tag != "" {
		tags = append(tags, tag)
	}
	return tags
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func containsWord(s, word string) bool {
	for i := 0; i <= len(s)-len(word); i++ {
		if s[i:i+len(word)] == word {
			return true
		}
	}
	return false
}
