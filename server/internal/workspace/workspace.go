// Package workspace manages agent workspace files (SOUL.md, USER.md, IDENTITY.md, etc.)
// that are loaded into the system prompt to give the agent personality, memory, and context.
package workspace

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	FileTOOLS     = "TOOLS.md"
	FileMEMORY    = "MEMORY.md"
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
	dir string
	mu  sync.RWMutex

	// fileCache caches file contents keyed by path, with mtime-based invalidation.
	fileCacheMu sync.RWMutex
	fileCache   map[string]fileCacheEntry
}

// fileCacheEntry holds a cached file read result.
type fileCacheEntry struct {
	content string
	modTime int64 // UnixNano
}

// NewManager creates a workspace manager for the given directory.
func NewManager(dir string) *Manager {
	return &Manager{dir: dir, fileCache: make(map[string]fileCacheEntry)}
}

// readFileCached reads a file with mtime-based caching.
// Returns content and true if file exists, or "" and false if not.
func (m *Manager) readFileCached(path string) (string, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return "", false
	}
	mtime := info.ModTime().UnixNano()

	m.fileCacheMu.RLock()
	if e, ok := m.fileCache[path]; ok && e.modTime == mtime {
		m.fileCacheMu.RUnlock()
		return e.content, true
	}
	m.fileCacheMu.RUnlock()

	// Cache miss or stale — read from disk
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	content := string(data)

	m.fileCacheMu.Lock()
	m.fileCache[path] = fileCacheEntry{content: content, modTime: mtime}
	m.fileCacheMu.Unlock()

	return content, true
}

// loadContextFileLocked resolves a well-known workspace context file to content.
// MEMORY.md supports a backward-compatible fallback to memory/MEMORY.md when the
// root file is missing or empty. Caller must hold m.mu (at least RLock).
func (m *Manager) loadContextFileLocked(name string) (string, bool) {
	primaryPath := m.resolveFilePath(name)
	primaryContent, primaryOK := m.readFileCached(primaryPath)
	if name != FileMEMORY {
		return primaryContent, primaryOK
	}
	if primaryOK && strings.TrimSpace(primaryContent) != "" {
		return primaryContent, true
	}

	fallbackPath := filepath.Join(m.dir, "memory", FileMEMORY)
	if fallbackContent, fallbackOK := m.readFileCached(fallbackPath); fallbackOK && strings.TrimSpace(fallbackContent) != "" {
		return fallbackContent, true
	}

	return primaryContent, primaryOK
}

// InvalidateFileCache clears the file cache (call after writes).
func (m *Manager) InvalidateFileCache() {
	m.fileCacheMu.Lock()
	m.fileCache = make(map[string]fileCacheEntry)
	m.fileCacheMu.Unlock()
}

var (
	detectedLocale   string
	detectLocaleOnce sync.Once
)

// DetectLocale returns the cached system locale (computed once per process).
func DetectLocale() string {
	detectLocaleOnce.Do(func() {
		detectedLocale = doDetectLocale()
	})
	return detectedLocale
}

// doDetectLocale detects the system locale.
// Checks LANG/LC_ALL/LANGUAGE env vars first, then falls back to
// OS-specific detection (macOS defaults, Windows APIs).
// Returns BCP-47 tag like "zh-CN", "ja-JP", "en-US", falling back to "en".
func doDetectLocale() string {
	// 1. Standard env vars (Linux, explicit overrides)
	for _, env := range []string{"LANG", "LC_ALL", "LANGUAGE"} {
		if v := os.Getenv(env); v != "" {
			// e.g. "zh_CN.UTF-8" → "zh-CN"
			v = strings.SplitN(v, ".", 2)[0] // strip encoding
			v = strings.ReplaceAll(v, "_", "-")
			if len(v) >= 2 {
				return v
			}
		}
	}

	// 2. OS-specific fallback
	switch runtime.GOOS {
	case "darwin":
		if loc := darwinLocale(); loc != "" {
			return loc
		}
	case "windows":
		if loc := windowsLocale(); loc != "" {
			return loc
		}
	}

	return "en"
}

// darwinLocale reads macOS system language from defaults.
// Tries AppleLocale first ("zh_CN" → "zh-CN"), then AppleLanguages ("zh-Hans-CN" → "zh-CN").
func darwinLocale() string {
	// AppleLocale: "zh_CN", "en_US", "ja_JP"
	if out, err := exec.Command("defaults", "read", "NSGlobalDomain", "AppleLocale").Output(); err == nil {
		v := strings.TrimSpace(string(out))
		v = strings.ReplaceAll(v, "_", "-")
		if len(v) >= 2 {
			return v
		}
	}

	// AppleLanguages: plist array, first entry is the preferred language
	// e.g. "zh-Hans-CN", "en-GB", "ja-JP"
	if out, err := exec.Command("defaults", "read", "NSGlobalDomain", "AppleLanguages").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			line = strings.Trim(line, `",`)
			if len(line) < 2 || line == "(" || line == ")" {
				continue
			}
			return normalizeAppleLanguage(line)
		}
	}

	return ""
}

// normalizeAppleLanguage converts Apple language tags to BCP-47 locale codes.
// "zh-Hans-CN" → "zh-CN", "zh-Hans" → "zh", "en-GB" → "en-GB", "ja" → "ja".
func normalizeAppleLanguage(tag string) string {
	parts := strings.Split(tag, "-")
	if len(parts) == 1 {
		return parts[0] // "ja" → "ja"
	}
	// If middle part is a script tag (4 letters, e.g. "Hans", "Hant"), skip it
	if len(parts) == 3 && len(parts[1]) == 4 {
		return parts[0] + "-" + parts[2] // "zh-Hans-CN" → "zh-CN"
	}
	if len(parts) == 2 && len(parts[1]) == 4 {
		return parts[0] // "zh-Hans" → "zh"
	}
	return tag // "en-GB" → "en-GB"
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

	// Ensure memory/ subdirectory for daily logs
	memDir := filepath.Join(m.dir, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		return fmt.Errorf("workspace: mkdir %s: %w", memDir, err)
	}

	ts := resolveTemplates(DetectLocale())

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
	} else {
		if err := m.removeBootstrapLocked(); err != nil {
			log.Printf("workspace: remove stale %s: %v", FileBOOTSTRAP, err)
		}
	}

	return nil
}

// LoadBootstrapFiles reads all well-known workspace files and returns them.
// Missing files are included with Missing=true and empty Content.
// Also includes BOOTSTRAP.md if it exists (first-run only).
// Uses mtime-based file caching to avoid redundant disk reads.
func (m *Manager) LoadBootstrapFiles() []BootstrapFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := []string{FileSOUL, FileUSER, FileIDENTITY, FileAGENTS, FileTOOLS, FileMEMORY}
	files := make([]BootstrapFile, 0, len(names)+3) // +3 for bootstrap + daily logs

	for _, name := range names {
		content, ok := m.loadContextFileLocked(name)
		if !ok {
			files = append(files, BootstrapFile{Name: name, Missing: true})
			continue
		}
		files = append(files, BootstrapFile{
			Name:    name,
			Content: content,
		})
	}

	// Include BOOTSTRAP.md only while first-run onboarding is actually pending.
	if m.bootstrapPendingLocked() {
		content, ok := m.readFileCached(filepath.Join(m.dir, FileBOOTSTRAP))
		if ok {
			files = append(files, BootstrapFile{Name: FileBOOTSTRAP, Content: content})
		}
	}

	// Include today's and yesterday's daily logs
	files = append(files, m.loadRecentDailyLogs()...)

	return files
}

// LoadContextFiles returns a map of filename→content for non-empty workspace files,
// ready to inject into SystemPromptBuilder. This is a lightweight path that avoids
// the overhead of LoadBootstrapFiles (no Missing entries, no BootstrapFile structs).
// Uses mtime-based file caching to avoid redundant disk reads.
func (m *Manager) LoadContextFiles() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := []string{FileSOUL, FileUSER, FileIDENTITY, FileAGENTS, FileTOOLS, FileMEMORY}
	ctx := make(map[string]string, len(names)+3)

	for _, name := range names {
		s, ok := m.loadContextFileLocked(name)
		if !ok || strings.TrimSpace(s) == "" {
			continue
		}
		ctx[name] = s
	}

	// Include BOOTSTRAP.md only while first-run onboarding is actually pending.
	if m.bootstrapPendingLocked() {
		if s, ok := m.readFileCached(filepath.Join(m.dir, FileBOOTSTRAP)); ok {
			if strings.TrimSpace(s) != "" {
				ctx[FileBOOTSTRAP] = s
			}
		}
	}

	// Include recent daily logs
	for _, bf := range m.loadRecentDailyLogs() {
		ctx[bf.Name] = bf.Content
	}

	return ctx
}

// resolveFilePath returns the on-disk path for a workspace file.
// All workspace files live in the workspace root directory.
func (m *Manager) resolveFilePath(name string) string {
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

	err := os.WriteFile(m.resolveFilePath(name), []byte(content), 0o644)
	if err == nil {
		m.InvalidateFileCache()
		if name == FileUSER && !isDefaultUserContent(content) {
			if rmErr := m.removeBootstrapLocked(); rmErr != nil {
				return rmErr
			}
		}
	}
	return err
}

// isAllowedFile checks if a filename is in the allowlist.
// Also rejects path traversal attempts (slashes, .., etc.).
func isAllowedFile(name string) bool {
	// Reject any path separators or traversal
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return false
	}
	switch name {
	case FileSOUL, FileUSER, FileIDENTITY, FileAGENTS, FileTOOLS, FileMEMORY, FileBOOTSTRAP:
		return true
	}
	return false
}

func (m *Manager) bootstrapPendingLocked() bool {
	if _, err := os.Stat(filepath.Join(m.dir, FileBOOTSTRAP)); err != nil {
		return false
	}
	return isDefaultUserTemplate(filepath.Join(m.dir, FileUSER))
}

func (m *Manager) removeBootstrapLocked() error {
	err := os.Remove(filepath.Join(m.dir, FileBOOTSTRAP))
	if os.IsNotExist(err) {
		return nil
	}
	if err == nil {
		m.InvalidateFileCache()
	}
	return err
}

// IsBootstrapPending returns true only while first-run onboarding is actually pending.
func (m *Manager) IsBootstrapPending() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.bootstrapPendingLocked()
}

// CompleteBootstrap removes BOOTSTRAP.md after the first-run guide is done.
func (m *Manager) CompleteBootstrap() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.removeBootstrapLocked()
}

// MemoryDir returns the path to the memory/ subdirectory.
func (m *Manager) MemoryDir() string {
	return filepath.Join(m.dir, "memory")
}

// ContextDir returns the path to the local context pack registry.
func (m *Manager) ContextDir() string {
	return filepath.Join(m.dir, ".blue", "context")
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
	if err == nil {
		m.InvalidateFileCache()
	}
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
			s, ok := m.readFileCached(path)
			if !ok || strings.TrimSpace(s) == "" {
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
	return isDefaultUserContent(string(content))
}

func isDefaultUserContent(content string) bool {
	trimmed := strings.TrimSpace(content)
	for _, locale := range availableLocales() {
		ts := resolveTemplates(locale)
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

// ReleaseSkills writes embedded SKILL.md files to {workspace}/.agents/skills/{name}/SKILL.md.
// Content-aware: only overwrites if the embedded content differs from the on-disk content
// (ignoring the `enabled` field which is user-managed state).
// Platform-aware: skips skills whose `os` field doesn't match runtime.GOOS.
// Preserves the `enabled` field from the existing on-disk file across upgrades,
// including legacy peer copies that still live under {workspace}/.claude/skills.
func (m *Manager) ReleaseSkills(fsys fs.FS) error {
	entries, err := fs.ReadDir(fsys, "skills")
	if err != nil {
		return fmt.Errorf("workspace: read embedded skills: %w", err)
	}

	skillsDir := filepath.Join(m.dir, ".agents", "skills")
	legacySkillsDir := filepath.Join(m.dir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return fmt.Errorf("workspace: mkdir %s: %w", skillsDir, err)
	}

	embeddedSkillData := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, readErr := fs.ReadFile(fsys, "skills/"+entry.Name()+"/SKILL.md")
		if readErr != nil {
			continue
		}
		embeddedSkillData[entry.Name()] = data
	}

	migrateLegacyBundledSkillDirs(skillsDir, embeddedSkillData)
	migrateLegacyBundledPeerSkillDirs(skillsDir, legacySkillsDir, embeddedSkillData)
	pruneRemovedPlaceholderSkills(skillsDir)
	pruneRemovedPlaceholderSkills(legacySkillsDir)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, ok := embeddedSkillData[entry.Name()]
		if !ok {
			continue
		}

		embeddedMeta := parseSkillFrontmatter(data)

		// Platform check — skip skills not for this OS
		if !platformMatch(embeddedMeta.OS) {
			// Clean up if previously released on a different platform
			dir := filepath.Join(skillsDir, entry.Name())
			if _, err := os.Stat(dir); err == nil {
				os.RemoveAll(dir)
				log.Printf("workspace: removed platform-mismatched skill %s", entry.Name())
			}
			legacyDir := filepath.Join(legacySkillsDir, entry.Name())
			if _, err := os.Stat(legacyDir); err == nil {
				os.RemoveAll(legacyDir)
				log.Printf("workspace: removed legacy platform-mismatched skill %s", entry.Name())
			}
			continue
		}

		dir := filepath.Join(skillsDir, entry.Name())
		mdPath := filepath.Join(dir, "SKILL.md")

		// Content check — skip if embedded content matches on-disk (ignoring enabled field)
		existingData, readErr := os.ReadFile(mdPath)
		if readErr == nil {
			if contentEqual(existingData, data) {
				continue // same content — preserve user's enabled state
			}
			// Content changed: preserve enabled state from existing file
			existingMeta := parseSkillFrontmatter(existingData)
			if existingMeta.Enabled != "" {
				data = setFrontmatterField(data, "enabled", existingMeta.Enabled)
			}
		} else {
			legacyData, legacyErr := os.ReadFile(filepath.Join(legacySkillsDir, entry.Name(), "SKILL.md"))
			if legacyErr == nil {
				legacyMeta := parseSkillFrontmatter(legacyData)
				if legacyMeta.Enabled != "" {
					data = setFrontmatterField(data, "enabled", legacyMeta.Enabled)
				}
			}
		}

		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Printf("workspace: mkdir %s: %v", dir, err)
			continue
		}
		if err := os.WriteFile(mdPath, data, 0o644); err != nil {
			log.Printf("workspace: write %s/SKILL.md: %v", entry.Name(), err)
		}
	}
	return nil
}

// ReleaseContextPacks writes embedded context pack files to {workspace}/.blue/context/...
func (m *Manager) ReleaseContextPacks(fsys fs.FS) error {
	entries, err := fs.ReadDir(fsys, "packs")
	if err != nil {
		return fmt.Errorf("workspace: read embedded context packs: %w", err)
	}

	root := m.ContextDir()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("workspace: mkdir %s: %w", root, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		prefix := filepath.ToSlash(filepath.Join("packs", entry.Name()))
		if walkErr := fs.WalkDir(fsys, prefix, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel := strings.TrimPrefix(path, "packs/")
			if rel == path {
				rel = strings.TrimPrefix(path, "packs")
			}
			rel = strings.TrimPrefix(rel, "/")
			target := filepath.Join(root, filepath.FromSlash(rel))
			if d.IsDir() {
				return os.MkdirAll(target, 0o755)
			}
			data, readErr := fs.ReadFile(fsys, path)
			if readErr != nil {
				return readErr
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if existing, readErr := os.ReadFile(target); readErr == nil && string(existing) == string(data) {
				return nil
			}
			return os.WriteFile(target, data, 0o644)
		}); walkErr != nil {
			log.Printf("workspace: release context pack %s: %v", entry.Name(), walkErr)
		}
	}
	return nil
}

// contentEqual compares two SKILL.md byte slices, ignoring the `enabled` field.
// This allows the release logic to detect real content changes while treating
// the user-managed `enabled` field as transparent.
func contentEqual(a, b []byte) bool {
	return string(stripFrontmatterField(a, "enabled")) == string(stripFrontmatterField(b, "enabled"))
}

// stripFrontmatterField returns a copy of data with the given field removed from frontmatter.
func stripFrontmatterField(data []byte, key string) []byte {
	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return data
	}
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return data
	}

	var filtered []string
	for _, line := range strings.Split(parts[1], "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+":") {
			continue // skip this field
		}
		filtered = append(filtered, line)
	}
	return []byte("---" + strings.Join(filtered, "\n") + "---" + parts[2])
}

// skillMeta holds parsed frontmatter fields relevant to release decisions.
type skillMeta struct {
	Name        string   // skill name from frontmatter
	Version     string   // optional — parsed but not used for release decisions
	Description string   // skill description from frontmatter
	Category    string   // skill category from frontmatter
	OS          []string // from top-level `os:` or nested `metadata.<vendor>.os`
	Enabled     string   // "true" or "false" — preserved across upgrades
}

// parseSkillFrontmatter extracts version, os, and enabled from YAML frontmatter.
// Supports both top-level fields and nested metadata vendor JSON.
func parseSkillFrontmatter(data []byte) skillMeta {
	var meta skillMeta
	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return meta
	}
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return meta
	}
	frontmatter := parts[1]

	var metadataBlock string
	for _, line := range strings.Split(frontmatter, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		colonIdx := strings.Index(trimmed, ":")
		if colonIdx == -1 {
			// Could be continuation of metadata block
			if metadataBlock != "" {
				metadataBlock += trimmed
			}
			continue
		}
		key := strings.TrimSpace(trimmed[:colonIdx])
		value := strings.TrimSpace(trimmed[colonIdx+1:])

		switch key {
		case "name":
			meta.Name = strings.Trim(value, `"'`)
		case "version":
			meta.Version = strings.Trim(value, `"'`)
		case "description":
			meta.Description = strings.Trim(value, `"'`)
		case "category":
			meta.Category = strings.Trim(value, `"'`)
		case "enabled":
			meta.Enabled = strings.Trim(value, `"'`)
		case "os":
			// Top-level os: ["darwin"] or os: darwin
			meta.OS = parseOSList(value)
		case "metadata":
			// Start collecting metadata block (JSON)
			metadataBlock = value
		}

		// Continue collecting metadata lines
		if metadataBlock != "" && key != "metadata" && !strings.HasPrefix(trimmed, " ") && !strings.HasPrefix(trimmed, "\t") {
			// New top-level key — stop collecting
		}
	}

	// Parse metadata vendor os from JSON if no top-level os found
	if len(meta.OS) == 0 && metadataBlock != "" {
		meta.OS = parseMetadataOS(metadataBlock)
	}

	return meta
}

type removedPlaceholderSignature struct {
	Description  string
	BodyPhrases  []string
	CanonicalSHA string
}

var removedPlaceholderSkills = map[string]removedPlaceholderSignature{
	"datetime": {
		Description:  "Disabled placeholder for date/time/timezone helpers. ZimaOS Blue does not currently register a dedicated builtin datetime skill in the live runtime.",
		BodyPhrases:  []string{"This skill is currently disabled.", "there is no dedicated builtin `datetime` skill registration backing that interface."},
		CanonicalSHA: "b254b47a2fdf02bd4a40bd0551c9bcc279840be75619585e9fd2097740a0d314",
	},
	"search": {
		Description:  "Disabled placeholder for the deprecated search skill name. ZimaOS Blue now uses web_query as the canonical public web skill.",
		BodyPhrases:  []string{"This skill is currently disabled.", "The old `search` name is deprecated"},
		CanonicalSHA: "4c4412807a26b066055e4db6c0dbaf837ba6e641d312c569dd3f9fca7c903e0e",
	},
	"timer": {
		Description:  "Disabled placeholder for session-local countdown timers. ZimaOS Blue does not currently register a live builtin timer skill at runtime.",
		BodyPhrases:  []string{"This skill is currently disabled.", "ZimaOS Blue does not currently register a live builtin `timer` skill at runtime"},
		CanonicalSHA: "9cbe0145b5b9033c91a8cb4a6f1cc2e5f0be77305f03ac17bd0508e25f404e7a",
	},
	"unit_converter": {
		Description:  "Disabled placeholder for unit conversion helpers. ZimaOS Blue does not currently register a dedicated builtin unit_converter skill in the live runtime.",
		BodyPhrases:  []string{"This skill is currently disabled.", "there is no dedicated builtin `unit_converter` skill registration behind that contract."},
		CanonicalSHA: "ecd08147365789f32000b86fb4605740f9bac71ebc26b32cd57d1512f14034ef",
	},
}

func pruneRemovedPlaceholderSkills(skillsDir string) {
	for name := range removedPlaceholderSkills {
		dir := filepath.Join(skillsDir, name)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Printf("workspace: read removed placeholder skill dir %s: %v", name, err)
			}
			continue
		}
		if len(entries) != 1 || entries[0].IsDir() || entries[0].Name() != "SKILL.md" {
			continue
		}

		mdPath := filepath.Join(dir, "SKILL.md")
		data, err := os.ReadFile(mdPath)
		if err != nil {
			log.Printf("workspace: read removed placeholder skill %s: %v", name, err)
			continue
		}
		if !matchesRemovedPlaceholderSkill(name, data) {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("workspace: prune removed placeholder skill %s: %v", name, err)
			continue
		}
		log.Printf("workspace: pruned removed placeholder skill %s", name)
	}
}

func migrateLegacyBundledSkillDirs(skillsDir string, embeddedSkillData map[string][]byte) {
	migrateLegacyBundledSkillDir(skillsDir, "web_search", "web_query", embeddedSkillData["web_query"])
}

func migrateLegacyBundledPeerSkillDirs(targetSkillsDir, legacySkillsDir string, embeddedSkillData map[string][]byte) {
	migrateLegacyBundledPeerSkillDir(targetSkillsDir, legacySkillsDir, "web_search", "web_query", embeddedSkillData["web_query"])
}

func migrateLegacyBundledSkillDir(skillsDir, legacyName, canonicalName string, canonicalData []byte) {
	if len(canonicalData) == 0 {
		return
	}

	legacyDir := filepath.Join(skillsDir, legacyName)
	entries, err := os.ReadDir(legacyDir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("workspace: read legacy bundled skill dir %s: %v", legacyName, err)
		}
		return
	}
	if len(entries) != 1 || entries[0].IsDir() || entries[0].Name() != "SKILL.md" {
		return
	}

	legacyPath := filepath.Join(legacyDir, "SKILL.md")
	legacyData, err := os.ReadFile(legacyPath)
	if err != nil {
		log.Printf("workspace: read legacy bundled skill %s: %v", legacyName, err)
		return
	}
	if !contentEqual(legacyData, canonicalData) {
		return
	}

	migratedData := canonicalData
	legacyMeta := parseSkillFrontmatter(legacyData)
	if legacyMeta.Enabled != "" {
		migratedData = setFrontmatterField(migratedData, "enabled", legacyMeta.Enabled)
	}

	canonicalDir := filepath.Join(skillsDir, canonicalName)
	canonicalPath := filepath.Join(canonicalDir, "SKILL.md")
	if existingData, readErr := os.ReadFile(canonicalPath); readErr == nil {
		if !contentEqual(existingData, canonicalData) {
			return
		}
		existingMeta := parseSkillFrontmatter(existingData)
		if existingMeta.Enabled != "" {
			migratedData = setFrontmatterField(canonicalData, "enabled", existingMeta.Enabled)
		}
	}

	if err := os.MkdirAll(canonicalDir, 0o755); err != nil {
		log.Printf("workspace: mkdir migrated bundled skill dir %s: %v", canonicalName, err)
		return
	}
	if err := os.WriteFile(canonicalPath, migratedData, 0o644); err != nil {
		log.Printf("workspace: write migrated bundled skill %s: %v", canonicalName, err)
		return
	}
	if err := os.RemoveAll(legacyDir); err != nil {
		log.Printf("workspace: remove legacy bundled skill dir %s: %v", legacyName, err)
		return
	}
	log.Printf("workspace: migrated legacy bundled skill dir %s -> %s", legacyName, canonicalName)
}

func migrateLegacyBundledPeerSkillDir(targetSkillsDir, legacySkillsDir, legacyName, canonicalName string, canonicalData []byte) {
	if len(canonicalData) == 0 {
		return
	}

	legacyDir := filepath.Join(legacySkillsDir, legacyName)
	entries, err := os.ReadDir(legacyDir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("workspace: read legacy peer bundled skill dir %s: %v", legacyName, err)
		}
		return
	}
	if len(entries) != 1 || entries[0].IsDir() || entries[0].Name() != "SKILL.md" {
		return
	}

	legacyPath := filepath.Join(legacyDir, "SKILL.md")
	legacyData, err := os.ReadFile(legacyPath)
	if err != nil {
		log.Printf("workspace: read legacy peer bundled skill %s: %v", legacyName, err)
		return
	}
	if !contentEqual(legacyData, canonicalData) {
		return
	}

	migratedData := canonicalData
	legacyMeta := parseSkillFrontmatter(legacyData)
	if legacyMeta.Enabled != "" {
		migratedData = setFrontmatterField(migratedData, "enabled", legacyMeta.Enabled)
	}

	canonicalDir := filepath.Join(targetSkillsDir, canonicalName)
	canonicalPath := filepath.Join(canonicalDir, "SKILL.md")
	if existingData, readErr := os.ReadFile(canonicalPath); readErr == nil {
		if !contentEqual(existingData, canonicalData) {
			return
		}
		existingMeta := parseSkillFrontmatter(existingData)
		if existingMeta.Enabled != "" {
			migratedData = setFrontmatterField(canonicalData, "enabled", existingMeta.Enabled)
		}
	}

	if err := os.MkdirAll(canonicalDir, 0o755); err != nil {
		log.Printf("workspace: mkdir migrated peer bundled skill dir %s: %v", canonicalName, err)
		return
	}
	if err := os.WriteFile(canonicalPath, migratedData, 0o644); err != nil {
		log.Printf("workspace: write migrated peer bundled skill %s: %v", canonicalName, err)
		return
	}
	if err := os.RemoveAll(legacyDir); err != nil {
		log.Printf("workspace: remove legacy peer bundled skill dir %s: %v", legacyName, err)
		return
	}
	log.Printf("workspace: migrated legacy peer bundled skill dir %s -> %s", legacyName, canonicalName)
}

func matchesRemovedPlaceholderSkill(name string, data []byte) bool {
	signature, ok := removedPlaceholderSkills[name]
	if !ok {
		return false
	}

	meta := parseSkillFrontmatter(data)
	if meta.Name != name || meta.Version != "0.1.0" || meta.Description != signature.Description || meta.Category != "internal" {
		return false
	}

	body := skillBody(data)
	for _, phrase := range signature.BodyPhrases {
		if !strings.Contains(body, phrase) {
			return false
		}
	}

	sum := sha256.Sum256(stripFrontmatterField(data, "enabled"))
	return fmt.Sprintf("%x", sum[:]) == signature.CanonicalSHA
}

func skillBody(data []byte) string {
	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return content
	}
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return content
	}
	return parts[2]
}

// parseOSList parses os field value: `["darwin"]`, `["darwin","linux"]`, or `darwin`
func parseOSList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	// JSON array format
	if strings.HasPrefix(value, "[") {
		var list []string
		if json.Unmarshal([]byte(value), &list) == nil {
			return list
		}
	}
	// Single value
	return []string{strings.Trim(value, `"'`)}
}

const (
	skillMetadataVendorPrimary  = "zimaos-blue"
	skillMetadataVendorAlias    = "zimaos_blue"
	skillMetadataVendorBackward = "open" + "claw"
)

// parseMetadataOS extracts os from nested metadata JSON: { "<vendor>": { "os": ["darwin"] } }
func parseMetadataOS(block string) []string {
	block = strings.TrimSpace(block)
	// Try to parse as JSON
	var outer map[string]json.RawMessage
	if json.Unmarshal([]byte(block), &outer) != nil {
		return nil
	}
	var vendorRaw json.RawMessage
	for _, vendorKey := range []string{skillMetadataVendorPrimary, skillMetadataVendorAlias, skillMetadataVendorBackward} {
		if raw, ok := outer[vendorKey]; ok {
			vendorRaw = raw
			break
		}
	}
	if len(vendorRaw) == 0 {
		return nil
	}
	var vendor map[string]json.RawMessage
	if json.Unmarshal(vendorRaw, &vendor) != nil {
		return nil
	}
	osRaw, ok := vendor["os"]
	if !ok {
		return nil
	}
	var osList []string
	if json.Unmarshal(osRaw, &osList) == nil {
		return osList
	}
	return nil
}

// platformMatch returns true if the current OS matches the skill's os list.
// Empty list means all platforms.
func platformMatch(osList []string) bool {
	if len(osList) == 0 {
		return true
	}
	for _, os := range osList {
		if os == runtime.GOOS {
			return true
		}
	}
	return false
}

// setFrontmatterField sets or replaces a field in YAML frontmatter.
func setFrontmatterField(data []byte, key, value string) []byte {
	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return data
	}
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return data
	}

	lines := strings.Split(parts[1], "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+":") {
			lines[i] = key + ": " + value
			found = true
			break
		}
	}
	if !found {
		// Add before the last empty line
		lines = append(lines[:len(lines)-1], key+": "+value, "")
	}

	return []byte("---" + strings.Join(lines, "\n") + "---" + parts[2])
}
