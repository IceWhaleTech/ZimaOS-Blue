// Package workspace manages agent workspace files (SOUL.md, USER.md, IDENTITY.md, etc.)
// that are loaded into the system prompt to give the agent personality, memory, and context.
package workspace

import (
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
// OS-specific detection (macOS defaults, Windows registry).
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

// windowsLocale reads the Windows display language via PowerShell.
// Falls back to Get-WinSystemLocale if Get-WinUserLanguageList is unavailable.
func windowsLocale() string {
	// Try user language list first (most accurate for UI language)
	if out, err := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command",
		"(Get-WinUserLanguageList)[0].LanguageTag").Output(); err == nil {
		v := strings.TrimSpace(string(out))
		if len(v) >= 2 {
			return v
		}
	}

	// Fallback: system locale
	if out, err := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command",
		"(Get-WinSystemLocale).Name").Output(); err == nil {
		v := strings.TrimSpace(string(out))
		v = strings.ReplaceAll(v, "_", "-")
		if len(v) >= 2 {
			return v
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

	ts := getTemplates(DetectLocale())

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
// Uses mtime-based file caching to avoid redundant disk reads.
func (m *Manager) LoadBootstrapFiles() []BootstrapFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := []string{FileSOUL, FileUSER, FileIDENTITY, FileAGENTS, FileTOOLS, FileMEMORY, FileHEARTBEAT}
	files := make([]BootstrapFile, 0, len(names)+3) // +3 for bootstrap + daily logs

	for _, name := range names {
		content, ok := m.readFileCached(m.resolveFilePath(name))
		if !ok {
			files = append(files, BootstrapFile{Name: name, Missing: true})
			continue
		}
		files = append(files, BootstrapFile{
			Name:    name,
			Content: content,
		})
	}

	// Include BOOTSTRAP.md if it exists (first-run guide, deleted after completion)
	if content, ok := m.readFileCached(filepath.Join(m.dir, FileBOOTSTRAP)); ok {
		files = append(files, BootstrapFile{Name: FileBOOTSTRAP, Content: content})
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

	names := []string{FileSOUL, FileUSER, FileIDENTITY, FileAGENTS, FileTOOLS, FileMEMORY, FileHEARTBEAT}
	ctx := make(map[string]string, len(names)+3)

	for _, name := range names {
		s, ok := m.readFileCached(m.resolveFilePath(name))
		if !ok || strings.TrimSpace(s) == "" {
			continue
		}
		ctx[name] = s
	}

	// Include BOOTSTRAP.md if it exists
	if s, ok := m.readFileCached(filepath.Join(m.dir, FileBOOTSTRAP)); ok {
		if strings.TrimSpace(s) != "" {
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
	case FileSOUL, FileUSER, FileIDENTITY, FileAGENTS, FileTOOLS, FileMEMORY, FileHEARTBEAT, FileBOOTSTRAP:
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
	if err == nil {
		m.InvalidateFileCache()
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
	trimmed := strings.TrimSpace(string(content))
	for _, locale := range availableLocales() {
		ts := getTemplates(locale)
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
// Content-aware: only overwrites if the embedded content differs from the on-disk content
// (ignoring the `enabled` field which is user-managed state).
// Platform-aware: skips skills whose `os` field doesn't match runtime.GOOS.
// Preserves the `enabled` field from the existing on-disk file across upgrades.
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

		embeddedMeta := parseSkillFrontmatter(data)

		// Platform check — skip skills not for this OS
		if !platformMatch(embeddedMeta.OS) {
			// Clean up if previously released on a different platform
			dir := filepath.Join(skillsDir, entry.Name())
			if _, err := os.Stat(dir); err == nil {
				os.RemoveAll(dir)
				log.Printf("workspace: removed platform-mismatched skill %s", entry.Name())
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
	Version string   // optional — parsed but not used for release decisions
	OS      []string // from top-level `os:` or nested `metadata.<vendor>.os`
	Enabled string   // "true" or "false" — preserved across upgrades
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
		case "version":
			meta.Version = strings.Trim(value, `"'`)
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
