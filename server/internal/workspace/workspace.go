// Package workspace manages agent workspace files (SOUL.md, USER.md, IDENTITY.md, etc.)
// that are loaded into the system prompt to give the agent personality, memory, and context.
package workspace

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// MaxFileSize is the maximum allowed size for a workspace file (1 MB).
const MaxFileSize = 1 << 20

// Well-known workspace filenames.
const (
	FileSOUL      = "SOUL.md"
	FileUSER      = "USER.md"
	FileIDENTITY  = "IDENTITY.md"
	FileAGENTS    = "AGENTS.md"
	FileMEMORY    = "MEMORY.md"
	FileHEARTBEAT = "HEARTBEAT.md"
	FileBOOTSTRAP = "BOOTSTRAP.md"
)

// BootstrapFile represents a loaded workspace file.
type BootstrapFile struct {
	Name    string `json:"name"`
	Content string `json:"content,omitempty"`
	Missing bool   `json:"missing,omitempty"`
}

// Manager manages workspace files on disk.
type Manager struct {
	dir    string
	locale string // e.g. "en", "zh", "zh-CN"
	mu     sync.RWMutex
}

// NewManager creates a workspace manager for the given directory.
// Locale is auto-detected from the LANG environment variable.
func NewManager(dir string) *Manager {
	return &Manager{dir: dir, locale: detectLocale()}
}

// SetLocale sets the locale for template generation.
// Accepts BCP-47 tags like "en", "zh", "zh-CN". Falls back to "en" for unknown locales.
func (m *Manager) SetLocale(locale string) {
	m.locale = locale
}

// detectLocale reads the LANG environment variable and extracts the language code.
func detectLocale() string {
	for _, env := range []string{"LANG", "LC_ALL", "LANGUAGE"} {
		if v := os.Getenv(env); v != "" {
			// e.g. "zh_CN.UTF-8" → "zh"
			v = strings.SplitN(v, ".", 2)[0] // strip encoding
			v = strings.ReplaceAll(v, "_", "-")
			if len(v) >= 2 {
				return v[:2]
			}
		}
	}
	return "en"
}

// Dir returns the workspace directory path.
func (m *Manager) Dir() string {
	return m.dir
}

// EnsureWorkspace creates the workspace directory and writes default template
// files if they don't already exist. Safe to call multiple times.
func (m *Manager) EnsureWorkspace() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return fmt.Errorf("workspace: mkdir %s: %w", m.dir, err)
	}

	// Ensure memory/ subdirectory
	memDir := filepath.Join(m.dir, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		return fmt.Errorf("workspace: mkdir %s: %w", memDir, err)
	}

	ts := getTemplates(m.locale)

	// Write templates for files that don't exist yet (best-effort — don't abort on individual failures)
	for name, tmpl := range ts.templateMap() {
		path := m.resolveFilePath(name)
		if err := writeIfMissing(path, tmpl); err != nil {
			log.Printf("workspace: write %s: %v", name, err)
		}
	}

	// Write BOOTSTRAP.md only on truly fresh workspace (USER.md is still default template)
	bootstrapPath := filepath.Join(m.dir, FileBOOTSTRAP)
	userPath := filepath.Join(m.dir, FileUSER)
	if isDefaultUserTemplate(userPath) {
		_ = writeIfMissing(bootstrapPath, ts.bootstrap)
	}

	return nil
}

// LoadBootstrapFiles reads all well-known workspace files and returns them.
// Missing files are included with Missing=true and empty Content.
// Also includes BOOTSTRAP.md if it exists (first-run only).
func (m *Manager) LoadBootstrapFiles() []BootstrapFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := []string{FileSOUL, FileUSER, FileIDENTITY, FileAGENTS, FileMEMORY, FileHEARTBEAT}
	files := make([]BootstrapFile, 0, len(names)+3) // +3 for bootstrap + daily logs

	for _, name := range names {
		path := m.resolveFilePath(name)
		content, err := os.ReadFile(path)
		if err != nil {
			files = append(files, BootstrapFile{Name: name, Missing: true})
			continue
		}
		files = append(files, BootstrapFile{
			Name:    name,
			Content: string(content),
		})
	}

	// Include BOOTSTRAP.md if it exists (first-run guide, deleted after completion)
	bootstrapPath := filepath.Join(m.dir, FileBOOTSTRAP)
	if content, err := os.ReadFile(bootstrapPath); err == nil {
		files = append(files, BootstrapFile{Name: FileBOOTSTRAP, Content: string(content)})
	}

	// Include today's and yesterday's daily logs
	files = append(files, m.loadRecentDailyLogs()...)

	return files
}

// LoadContextFiles returns a map of filename→content for non-empty workspace files,
// ready to inject into SystemPromptBuilder. This is a lightweight path that avoids
// the overhead of LoadBootstrapFiles (no Missing entries, no BootstrapFile structs).
func (m *Manager) LoadContextFiles() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := []string{FileSOUL, FileUSER, FileIDENTITY, FileAGENTS, FileMEMORY, FileHEARTBEAT}
	ctx := make(map[string]string, len(names)+3)

	for _, name := range names {
		data, err := os.ReadFile(m.resolveFilePath(name))
		if err != nil {
			continue
		}
		s := string(data)
		if strings.TrimSpace(s) == "" {
			continue
		}
		ctx[name] = s
	}

	// Include BOOTSTRAP.md if it exists
	if data, err := os.ReadFile(filepath.Join(m.dir, FileBOOTSTRAP)); err == nil {
		if s := string(data); strings.TrimSpace(s) != "" {
			ctx[FileBOOTSTRAP] = s
		}
	}

	// Include recent daily logs
	for _, bf := range m.loadRecentDailyLogs() {
		ctx[bf.Name] = bf.Content
	}

	return ctx
}

// resolveFilePath returns the on-disk path for a workspace file.
// MEMORY.md lives under memory/ subdirectory; all others live in workspace root.
func (m *Manager) resolveFilePath(name string) string {
	if name == FileMEMORY {
		return filepath.Join(m.dir, "memory", FileMEMORY)
	}
	return filepath.Join(m.dir, name)
}

// ReadFile reads a single workspace file by name.
// Returns an error if the file exceeds MaxFileSize.
func (m *Manager) ReadFile(name string) (string, error) {
	if !isAllowedFile(name) {
		return "", fmt.Errorf("workspace: file %q not allowed", name)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	path := m.resolveFilePath(name)
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.Size() > MaxFileSize {
		return "", fmt.Errorf("workspace: file %q exceeds max size (%d bytes)", name, MaxFileSize)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteFile writes content to a workspace file by name.
func (m *Manager) WriteFile(name, content string) error {
	if !isAllowedFile(name) {
		return fmt.Errorf("workspace: file %q not allowed", name)
	}
	if len(content) > MaxFileSize {
		return fmt.Errorf("workspace: file %q exceeds max size (%d bytes)", name, MaxFileSize)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	return os.WriteFile(m.resolveFilePath(name), []byte(content), 0o644)
}

// isAllowedFile checks if a filename is in the allowlist.
// Also rejects path traversal attempts (slashes, .., etc.).
func isAllowedFile(name string) bool {
	// Reject any path separators or traversal
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return false
	}
	switch name {
	case FileSOUL, FileUSER, FileIDENTITY, FileAGENTS, FileMEMORY, FileHEARTBEAT, FileBOOTSTRAP:
		return true
	}
	return false
}

// IsBootstrapPending returns true if BOOTSTRAP.md exists (first-run not completed).
func (m *Manager) IsBootstrapPending() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, err := os.Stat(filepath.Join(m.dir, FileBOOTSTRAP))
	return err == nil
}

// CompleteBootstrap removes BOOTSTRAP.md after the first-run guide is done.
func (m *Manager) CompleteBootstrap() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	path := filepath.Join(m.dir, FileBOOTSTRAP)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// MemoryDir returns the path to the memory/ subdirectory.
func (m *Manager) MemoryDir() string {
	return filepath.Join(m.dir, "memory")
}

// AppendDailyLog appends content to today's daily log file (memory/YYYY-MM-DD.md).
func (m *Manager) AppendDailyLog(content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	today := timeutil.NowTime().Format("2006-01-02")
	logPath := filepath.Join(m.dir, "memory", today+".md")

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("workspace: open daily log: %w", err)
	}
	defer f.Close()

	// Write header if new file
	info, _ := f.Stat()
	if info.Size() == 0 {
		header := fmt.Sprintf("# Daily Log - %s\n\n", today)
		if _, err := f.WriteString(header); err != nil {
			return err
		}
	}

	// Append timestamped entry
	ts := timeutil.NowTime().Format("15:04")
	entry := fmt.Sprintf("## %s\n%s\n\n", ts, content)
	_, err = f.WriteString(entry)
	return err
}

// loadRecentDailyLogs reads today's and yesterday's daily logs.
// Checks both memory/ (workspace.AppendDailyLog) and memory/daily/ (LayeredMemoryService).
// Must be called with m.mu held (at least RLock).
func (m *Manager) loadRecentDailyLogs() []BootstrapFile {
	memDir := filepath.Join(m.dir, "memory")
	now := timeutil.NowTime()
	dates := []string{
		now.Format("2006-01-02"),
		now.AddDate(0, 0, -1).Format("2006-01-02"),
	}

	var files []BootstrapFile
	for _, date := range dates {
		name := date + ".md"
		// Check both memory/ and memory/daily/ — LayeredMemoryService writes to daily/ subdir
		candidates := []string{
			filepath.Join(memDir, name),
			filepath.Join(memDir, "daily", name),
		}
		for _, path := range candidates {
			content, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			s := string(content)
			if strings.TrimSpace(s) == "" {
				continue
			}
			files = append(files, BootstrapFile{
				Name:    "memory/" + name,
				Content: s,
			})
			break // Use first found, avoid duplicates for same date
		}
	}
	return files
}

// ListDailyLogs returns all daily log filenames sorted by date (newest first).
func (m *Manager) ListDailyLogs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	memDir := filepath.Join(m.dir, "memory")
	entries, err := os.ReadDir(memDir)
	if err != nil {
		return nil
	}

	var logs []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			logs = append(logs, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(logs)))
	return logs
}

// isDefaultUserTemplate checks if USER.md still has any locale's default template content.
func isDefaultUserTemplate(path string) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return true // File doesn't exist, treat as default
	}
	trimmed := strings.TrimSpace(string(content))
	for _, ts := range localizedTemplates {
		if trimmed == strings.TrimSpace(ts.user) {
			return true
		}
	}
	return false
}

// writeIfMissing writes content to path only if the file doesn't exist.
func writeIfMissing(path, content string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil // Already exists, skip
		}
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

// ReleaseSkills writes embedded SKILL.md files to {workspace}/.claude/skills/{name}/SKILL.md.
// Each file is overwritten on every startup to keep skills in sync with the binary.
func (m *Manager) ReleaseSkills(fsys fs.FS) error {
	entries, err := fs.ReadDir(fsys, "skills")
	if err != nil {
		return fmt.Errorf("workspace: read embedded skills: %w", err)
	}

	skillsDir := filepath.Join(m.dir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return fmt.Errorf("workspace: mkdir %s: %w", skillsDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := fs.ReadFile(fsys, "skills/"+entry.Name()+"/SKILL.md")
		if err != nil {
			continue
		}
		dir := filepath.Join(skillsDir, entry.Name())
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Printf("workspace: mkdir %s: %v", dir, err)
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), data, 0o644); err != nil {
			log.Printf("workspace: write %s/SKILL.md: %v", entry.Name(), err)
		}
	}
	return nil
}
