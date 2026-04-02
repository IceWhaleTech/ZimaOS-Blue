package workspace

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEnsureWorkspace(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	if err := mgr.EnsureWorkspace(); err != nil {
		t.Fatalf("EnsureWorkspace: %v", err)
	}

	// All template files should exist
	for name := range getTemplates("en").templateMap() {
		path := mgr.resolveFilePath(name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s to exist: %v", name, err)
		}
	}

	// memory/ dir should exist
	if _, err := os.Stat(filepath.Join(dir, "memory")); err != nil {
		t.Error("expected memory/ dir to exist")
	}

	// BOOTSTRAP.md should exist on fresh workspace
	if _, err := os.Stat(filepath.Join(dir, FileBOOTSTRAP)); err != nil {
		t.Error("expected BOOTSTRAP.md to exist on fresh workspace")
	}

	// Calling again should not overwrite
	custom := "# Custom SOUL"
	os.WriteFile(filepath.Join(dir, FileSOUL), []byte(custom), 0o644)
	if err := mgr.EnsureWorkspace(); err != nil {
		t.Fatalf("EnsureWorkspace (2nd): %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, FileSOUL))
	if string(data) != custom {
		t.Error("EnsureWorkspace overwrote existing file")
	}
}

func TestEnsureWorkspace_NoBootstrapAfterUserEdited(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	mgr.EnsureWorkspace()

	// Simulate user has edited USER.md
	os.WriteFile(filepath.Join(dir, FileUSER), []byte("# My custom user"), 0o644)
	// Remove BOOTSTRAP.md
	os.Remove(filepath.Join(dir, FileBOOTSTRAP))

	// Re-ensure should NOT recreate BOOTSTRAP.md since USER.md is customized
	mgr.EnsureWorkspace()
	if _, err := os.Stat(filepath.Join(dir, FileBOOTSTRAP)); err == nil {
		t.Error("BOOTSTRAP.md should not be recreated after user edited USER.md")
	}
}

func TestEnsureWorkspace_RemovesStaleBootstrapAfterUserEdited(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	mgr.EnsureWorkspace()

	bootstrapPath := filepath.Join(dir, FileBOOTSTRAP)
	if _, err := os.Stat(bootstrapPath); err != nil {
		t.Fatalf("expected BOOTSTRAP.md to exist before cleanup: %v", err)
	}

	os.WriteFile(filepath.Join(dir, FileUSER), []byte("# My custom user"), 0o644)

	if err := mgr.EnsureWorkspace(); err != nil {
		t.Fatalf("EnsureWorkspace cleanup: %v", err)
	}
	if _, err := os.Stat(bootstrapPath); err == nil {
		t.Error("expected stale BOOTSTRAP.md to be removed after user edited USER.md")
	}
	if mgr.IsBootstrapPending() {
		t.Error("expected bootstrap to be non-pending after user edited USER.md")
	}
}

func TestBootstrapLifecycle(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	mgr.EnsureWorkspace()

	// Should be pending
	if !mgr.IsBootstrapPending() {
		t.Error("expected bootstrap to be pending")
	}

	// Bootstrap file should appear in LoadBootstrapFiles
	files := mgr.LoadBootstrapFiles()
	found := false
	for _, f := range files {
		if f.Name == FileBOOTSTRAP {
			found = true
			if f.Content == "" {
				t.Error("BOOTSTRAP.md should have content")
			}
		}
	}
	if !found {
		t.Error("BOOTSTRAP.md not found in LoadBootstrapFiles")
	}

	// Complete bootstrap
	if err := mgr.CompleteBootstrap(); err != nil {
		t.Fatalf("CompleteBootstrap: %v", err)
	}

	// Should no longer be pending
	if mgr.IsBootstrapPending() {
		t.Error("expected bootstrap to be completed")
	}

	// Idempotent
	if err := mgr.CompleteBootstrap(); err != nil {
		t.Fatalf("CompleteBootstrap (2nd): %v", err)
	}
}

func TestLoadBootstrapFiles(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	mgr.EnsureWorkspace()

	files := mgr.LoadBootstrapFiles()
	if len(files) == 0 {
		t.Fatal("expected bootstrap files")
	}

	found := map[string]bool{}
	for _, f := range files {
		found[f.Name] = true
		if f.Missing {
			t.Errorf("%s should not be missing", f.Name)
		}
		if f.Content == "" {
			t.Errorf("%s should have content", f.Name)
		}
	}

	for _, name := range []string{FileSOUL, FileUSER, FileIDENTITY, FileAGENTS, FileTOOLS, FileMEMORY} {
		if !found[name] {
			t.Errorf("missing %s in bootstrap files", name)
		}
	}
}

func TestLoadContextFiles(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	mgr.EnsureWorkspace()

	ctx := mgr.LoadContextFiles()
	if len(ctx) == 0 {
		t.Fatal("expected context files")
	}
	if _, ok := ctx[FileSOUL]; !ok {
		t.Error("expected SOUL.md in context files")
	}
	if _, ok := ctx[FileTOOLS]; !ok {
		t.Error("expected TOOLS.md in context files")
	}
}

func TestLoadContextFiles_FallsBackToNestedMemoryFile(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	mgr.EnsureWorkspace()

	rootMemory := filepath.Join(dir, FileMEMORY)
	if err := os.Remove(rootMemory); err != nil {
		t.Fatalf("remove root memory: %v", err)
	}
	nestedContent := "# Nested Memory\n\n- Favorite language: Rust\n"
	if err := os.WriteFile(filepath.Join(dir, "memory", FileMEMORY), []byte(nestedContent), 0o644); err != nil {
		t.Fatalf("write nested memory: %v", err)
	}
	mgr.InvalidateFileCache()

	ctx := mgr.LoadContextFiles()
	if got := ctx[FileMEMORY]; got != nestedContent {
		t.Fatalf("MEMORY.md context = %q, want nested fallback", got)
	}

	files := mgr.LoadBootstrapFiles()
	for _, file := range files {
		if file.Name != FileMEMORY {
			continue
		}
		if file.Missing {
			t.Fatal("expected fallback MEMORY.md to be treated as present")
		}
		if file.Content != nestedContent {
			t.Fatalf("bootstrap MEMORY.md = %q, want nested fallback", file.Content)
		}
		return
	}
	t.Fatal("expected MEMORY.md entry in bootstrap files")
}

func TestLoadContextFiles_PrefersRootMemoryOverNestedFallback(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	mgr.EnsureWorkspace()

	rootContent := "# Root Memory\n\n- Use root file.\n"
	if err := os.WriteFile(filepath.Join(dir, FileMEMORY), []byte(rootContent), 0o644); err != nil {
		t.Fatalf("write root memory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "memory", FileMEMORY), []byte("# Nested Memory\n\n- Ignore nested.\n"), 0o644); err != nil {
		t.Fatalf("write nested memory: %v", err)
	}
	mgr.InvalidateFileCache()

	ctx := mgr.LoadContextFiles()
	if got := ctx[FileMEMORY]; got != rootContent {
		t.Fatalf("MEMORY.md context = %q, want root content", got)
	}
}

func TestStaleBootstrapIgnoredAfterUserEdited(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	mgr.EnsureWorkspace()

	os.WriteFile(filepath.Join(dir, FileUSER), []byte("# My custom user"), 0o644)

	if mgr.IsBootstrapPending() {
		t.Error("expected stale bootstrap file to be ignored once USER.md is customized")
	}

	files := mgr.LoadBootstrapFiles()
	for _, f := range files {
		if f.Name == FileBOOTSTRAP {
			t.Fatal("did not expect stale BOOTSTRAP.md in bootstrap file list")
		}
	}

	ctx := mgr.LoadContextFiles()
	if _, ok := ctx[FileBOOTSTRAP]; ok {
		t.Fatal("did not expect stale BOOTSTRAP.md in context files")
	}
}

func TestReadWriteFile(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	mgr.EnsureWorkspace()

	// Write
	if err := mgr.WriteFile(FileUSER, "# My User"); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Read back
	content, err := mgr.ReadFile(FileUSER)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if content != "# My User" {
		t.Errorf("got %q, want %q", content, "# My User")
	}
	if _, err := os.Stat(filepath.Join(dir, FileBOOTSTRAP)); err == nil {
		t.Error("expected WriteFile(USER.md) to remove BOOTSTRAP.md once user content is customized")
	}
	if mgr.IsBootstrapPending() {
		t.Error("expected bootstrap to be non-pending after WriteFile(USER.md)")
	}

	// Disallowed file
	if err := mgr.WriteFile("evil.sh", "rm -rf /"); err == nil {
		t.Error("expected error for disallowed file")
	}
	if _, err := mgr.ReadFile("evil.sh"); err == nil {
		t.Error("expected error for disallowed file")
	}

	// Path traversal attempts
	traversals := []string{
		"../etc/passwd",
		"..\\windows\\system32",
		"SOUL.md/../../../etc/shadow",
		"SOUL.md/../../secret",
		"./SOUL.md",
	}
	for _, name := range traversals {
		if err := mgr.WriteFile(name, "pwned"); err == nil {
			t.Errorf("expected error for path traversal %q", name)
		}
		if _, err := mgr.ReadFile(name); err == nil {
			t.Errorf("expected error for path traversal read %q", name)
		}
	}

	// File size limit
	huge := strings.Repeat("x", MaxFileSize+1)
	if err := mgr.WriteFile(FileUSER, huge); err == nil {
		t.Error("expected error for oversized file")
	}
	// Exactly at limit should succeed
	exact := strings.Repeat("x", MaxFileSize)
	if err := mgr.WriteFile(FileUSER, exact); err != nil {
		t.Errorf("expected write at max size to succeed: %v", err)
	}
}

func TestDailyLog(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	mgr.EnsureWorkspace()

	// Append to daily log
	if err := mgr.AppendDailyLog("User prefers dark mode"); err != nil {
		t.Fatalf("AppendDailyLog: %v", err)
	}
	if err := mgr.AppendDailyLog("Discussed project architecture"); err != nil {
		t.Fatalf("AppendDailyLog (2nd): %v", err)
	}

	// Should appear in daily log list
	logs := mgr.ListDailyLogs()
	if len(logs) == 0 {
		t.Fatal("expected daily logs")
	}

	// Should appear in LoadBootstrapFiles
	files := mgr.LoadBootstrapFiles()
	foundDaily := false
	for _, f := range files {
		if strings.HasPrefix(f.Name, "memory/") {
			foundDaily = true
			if !strings.Contains(f.Content, "dark mode") {
				t.Error("daily log should contain appended content")
			}
		}
	}
	if !foundDaily {
		t.Error("daily log not found in LoadBootstrapFiles")
	}
}

func TestLocaleTemplates(t *testing.T) {
	// Chinese locale
	dir := t.TempDir()
	t.Setenv("LANG", "zh")
	mgr := NewManager(dir)
	if err := mgr.EnsureWorkspace(); err != nil {
		t.Fatalf("EnsureWorkspace (zh): %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, FileSOUL))
	if err != nil {
		t.Fatalf("ReadFile SOUL.md: %v", err)
	}
	if !strings.Contains(string(data), "AI 助手") {
		t.Error("expected Chinese SOUL.md content")
	}
	if strings.Contains(string(data), "AI Assistant") {
		t.Error("Chinese SOUL.md should not contain English text")
	}

	// Fallback for unknown locale
	ts := getTemplates("xx-YY")
	if !strings.Contains(ts.soul, "AI Assistant") {
		t.Error("unknown locale should fall back to English")
	}

	// French locale should use French templates
	ts = getTemplates("fr")
	if !strings.Contains(ts.soul, "Assistant IA") {
		t.Error("fr should use French templates")
	}
	if strings.TrimSpace(ts.tools) == "" {
		t.Error("fr should receive TOOLS.md via field-level english fallback")
	}

	// BCP-47 prefix matching: zh-TW has its own template (Traditional Chinese)
	ts = getTemplates("zh-TW")
	if !strings.Contains(ts.soul, "AI 助手") {
		t.Error("zh-TW should contain Chinese content")
	}
	if !strings.Contains(ts.soul, "個人雲端") {
		t.Error("zh-TW should use Traditional Chinese templates")
	}
}

func TestAgentsTemplate_LoadsPlatformSpecificTemplate(t *testing.T) {
	suffixByGOOS := map[string]string{
		"darwin":  "_darwin",
		"linux":   "_linux",
		"windows": "_windows",
	}

	suffix, ok := suffixByGOOS[runtime.GOOS]
	if !ok {
		t.Skipf("unsupported runtime.GOOS in test expectations: %s", runtime.GOOS)
	}

	for _, locale := range []string{"en", "zh"} {
		ts := getTemplates(locale)
		if ts == nil {
			t.Fatalf("expected %s templates", locale)
		}

		wantBytes, err := fs.ReadFile(templatesFS, "templates/"+locale+"/AGENTS"+suffix+".md")
		if err != nil {
			t.Fatalf("read platform AGENTS for %s: %v", locale, err)
		}

		got := strings.TrimSpace(ts.agents)
		want := strings.TrimSpace(string(wantBytes))
		if got != want {
			t.Fatalf("%s AGENTS should load platform-specific template %q", locale, "AGENTS"+suffix+".md")
		}

		if strings.Contains(got, "`uname` / `$OSTYPE` / `$PSVersionTable`") {
			t.Fatalf("%s AGENTS should come from the platform-specific file, not the generic OS/Shell checklist", locale)
		}
	}
}

func TestEnsureWorkspace_WritesDetectedAgentsTemplate(t *testing.T) {
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("LC_ALL", "")
	t.Setenv("LANGUAGE", "")

	dir := t.TempDir()
	mgr := NewManager(dir)
	if err := mgr.EnsureWorkspace(); err != nil {
		t.Fatalf("EnsureWorkspace: %v", err)
	}

	gotBytes, err := os.ReadFile(filepath.Join(dir, FileAGENTS))
	if err != nil {
		t.Fatalf("read workspace AGENTS.md: %v", err)
	}

	got := strings.TrimSpace(string(gotBytes))
	want := strings.TrimSpace(getTemplates(DetectLocale()).agents)
	if got != want {
		t.Fatal("workspace AGENTS.md should match the detected template content written by EnsureWorkspace")
	}
}

func TestLocalizedAgentsTemplate_RemainsLocalizedAfterPlatformSplit(t *testing.T) {
	ts := getTemplates("fr")
	if ts == nil {
		t.Fatal("expected fr templates")
	}

	if !strings.Contains(ts.agents, "Les informations privées restent privées") {
		t.Fatal("fr AGENTS should remain localized after loading the platform-specific template")
	}
	if strings.Contains(ts.agents, "Private things stay private") {
		t.Fatal("fr AGENTS should not fall back to English content")
	}
}

func TestNormalizeDefaultTemplateToEnglish(t *testing.T) {
	zh := getTemplates("zh")
	if zh == nil {
		t.Fatal("expected zh templates")
	}
	en := getTemplates("en")
	if en == nil {
		t.Fatal("expected en templates")
	}

	got := NormalizeDefaultTemplateToEnglish(FileSOUL, zh.soul)
	if strings.TrimSpace(got) != strings.TrimSpace(en.soul) {
		t.Fatal("expected zh default SOUL.md to normalize to english template")
	}

	custom := "# Blue\n这是自定义内容"
	got = NormalizeDefaultTemplateToEnglish(FileSOUL, custom)
	if got != custom {
		t.Fatal("expected custom content to remain unchanged")
	}
}

func TestNormalizeAppleLanguage(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"zh-Hans-CN", "zh-CN"},
		{"zh-Hant-TW", "zh-TW"},
		{"zh-Hans", "zh"},
		{"zh-Hant", "zh"},
		{"en-GB", "en-GB"},
		{"ja", "ja"},
		{"en", "en"},
		{"pt-BR", "pt-BR"},
	}
	for _, tt := range tests {
		got := normalizeAppleLanguage(tt.input)
		if got != tt.want {
			t.Errorf("normalizeAppleLanguage(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDetectLocale_EnvVar(t *testing.T) {
	// Set LANG and verify it takes priority
	t.Setenv("LANG", "ja_JP.UTF-8")
	t.Setenv("LC_ALL", "")
	t.Setenv("LANGUAGE", "")
	got := doDetectLocale()
	if got != "ja-JP" {
		t.Errorf("doDetectLocale() with LANG=ja_JP.UTF-8 = %q, want %q", got, "ja-JP")
	}
}

func TestDetectLocale_DarwinFallback(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only test")
	}
	// Clear env vars to force OS-specific fallback
	t.Setenv("LANG", "")
	t.Setenv("LC_ALL", "")
	t.Setenv("LANGUAGE", "")
	got := doDetectLocale()
	// On macOS, should get something from system preferences, not "en" fallback
	// (unless the system is actually English, which is also fine)
	if len(got) < 2 {
		t.Errorf("doDetectLocale() on darwin with no env = %q, expected valid locale", got)
	}
	t.Logf("detected macOS locale: %s", got)
}

// --- ReleaseSkills tests ---

func TestParseSkillFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		version string
		os      []string
		enabled string
	}{
		{
			name:    "full frontmatter",
			input:   "---\nname: browser\nversion: \"1.2.0\"\nenabled: true\n---\n# Browser",
			version: "1.2.0",
			enabled: "true",
		},
		{
			name:  "no frontmatter",
			input: "# Browser\nSome content",
		},
		{
			name:    "with top-level os",
			input:   "---\nname: test\nversion: \"1.0.0\"\nos: [\"darwin\"]\n---\n# Test",
			version: "1.0.0",
			os:      []string{"darwin"},
		},
		{
			name:  "with metadata zimaos-blue os",
			input: "---\nname: test\nmetadata: {\"zimaos-blue\":{\"os\":[\"darwin\"]}}\n---\n# Test",
			os:    []string{"darwin"},
		},
		{
			name:  "with metadata legacy vendor os",
			input: "---\nname: test\nmetadata: {\"open" + "claw\":{\"os\":[\"darwin\"]}}\n---\n# Test",
			os:    []string{"darwin"},
		},
		{
			name:    "enabled false",
			input:   "---\nname: test\nenabled: false\n---\n# Test",
			enabled: "false",
		},
		{
			name:  "no version (optional)",
			input: "---\nname: test\n---\n# Test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := parseSkillFrontmatter([]byte(tt.input))
			if meta.Version != tt.version {
				t.Errorf("version = %q, want %q", meta.Version, tt.version)
			}
			if len(meta.OS) != len(tt.os) {
				t.Errorf("os = %v, want %v", meta.OS, tt.os)
			}
			if meta.Enabled != tt.enabled {
				t.Errorf("enabled = %q, want %q", meta.Enabled, tt.enabled)
			}
		})
	}
}

func TestContentEqual(t *testing.T) {
	// Same content → equal
	a := []byte("---\nname: test\n---\n# Test")
	b := []byte("---\nname: test\n---\n# Test")
	if !contentEqual(a, b) {
		t.Error("identical content should be equal")
	}

	// Different content → not equal
	c := []byte("---\nname: test\n---\n# Updated Test")
	if contentEqual(a, c) {
		t.Error("different content should not be equal")
	}

	// Same content but different enabled → equal (enabled is ignored)
	d := []byte("---\nname: test\nenabled: true\n---\n# Test")
	e := []byte("---\nname: test\nenabled: false\n---\n# Test")
	if !contentEqual(d, e) {
		t.Error("content differing only in enabled should be equal")
	}

	// One has enabled, other doesn't → equal if rest matches
	f := []byte("---\nname: test\n---\n# Test")
	g := []byte("---\nname: test\nenabled: true\n---\n# Test")
	if !contentEqual(f, g) {
		t.Error("content differing only by presence of enabled should be equal")
	}
}

func TestStripFrontmatterField(t *testing.T) {
	input := []byte("---\nname: test\nenabled: true\nos: [\"darwin\"]\n---\n# Test")
	result := string(stripFrontmatterField(input, "enabled"))
	if strings.Contains(result, "enabled") {
		t.Error("enabled field should be stripped")
	}
	if !strings.Contains(result, "name: test") {
		t.Error("other fields should be preserved")
	}
	if !strings.Contains(result, "# Test") {
		t.Error("body should be preserved")
	}
}

func TestSetFrontmatterField(t *testing.T) {
	// Replace existing field
	input := "---\nname: test\nenabled: true\n---\n# Test"
	result := string(setFrontmatterField([]byte(input), "enabled", "false"))
	if !strings.Contains(result, "enabled: false") {
		t.Errorf("expected enabled: false, got %q", result)
	}

	// Add new field
	input = "---\nname: test\n---\n# Test"
	result = string(setFrontmatterField([]byte(input), "enabled", "true"))
	if !strings.Contains(result, "enabled: true") {
		t.Errorf("expected enabled: true added, got %q", result)
	}
}

func makeTestFS(t *testing.T, skills map[string]string) *testFS {
	t.Helper()
	return &testFS{skills: skills}
}

// testFS implements fs.FS for testing ReleaseSkills.
type testFS struct {
	skills map[string]string // name → SKILL.md content
}

func (f *testFS) Open(name string) (fs.File, error) {
	return nil, &os.PathError{Op: "open", Path: name, Err: os.ErrNotExist}
}

func (f *testFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name != "skills" {
		return nil, os.ErrNotExist
	}
	var entries []fs.DirEntry
	for k := range f.skills {
		entries = append(entries, &fakeDirEntry{name: k, isDir: true})
	}
	return entries, nil
}

func (f *testFS) ReadFile(name string) ([]byte, error) {
	// name is "skills/<id>/SKILL.md"
	parts := strings.Split(name, "/")
	if len(parts) == 3 && parts[0] == "skills" && parts[2] == "SKILL.md" {
		if content, ok := f.skills[parts[1]]; ok {
			return []byte(content), nil
		}
	}
	return nil, os.ErrNotExist
}

type fakeDirEntry struct {
	name  string
	isDir bool
}

func (e *fakeDirEntry) Name() string               { return e.name }
func (e *fakeDirEntry) IsDir() bool                { return e.isDir }
func (e *fakeDirEntry) Type() os.FileMode          { return 0 }
func (e *fakeDirEntry) Info() (os.FileInfo, error) { return nil, nil }

// testFSAdapter wraps testFS to implement fs.FS + fs.ReadDirFS + fs.ReadFileFS
type testFSAdapter struct{ inner *testFS }

func (a *testFSAdapter) Open(name string) (fs.File, error)          { return a.inner.Open(name) }
func (a *testFSAdapter) ReadDir(name string) ([]fs.DirEntry, error) { return a.inner.ReadDir(name) }
func (a *testFSAdapter) ReadFile(name string) ([]byte, error)       { return a.inner.ReadFile(name) }

func TestReleaseSkills_ContentChanged(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Write old content on disk with enabled: true
	skillDir := filepath.Join(dir, ".claude", "skills", "test-skill")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: test-skill\nenabled: true\n---\n# Old Content"), 0o644)

	// Release with new content
	fsys := &testFSAdapter{&testFS{skills: map[string]string{
		"test-skill": "---\nname: test-skill\n---\n# New Content",
	}}}
	if err := mgr.ReleaseSkills(fsys); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	content := string(data)
	if !strings.Contains(content, "New Content") {
		t.Error("should have updated to new content")
	}
	if !strings.Contains(content, "enabled: true") {
		t.Error("should have preserved enabled: true")
	}
}

func TestReleaseSkills_ContentSame(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Write content on disk with user's enabled state
	skillDir := filepath.Join(dir, ".claude", "skills", "test-skill")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: test-skill\nenabled: false\n---\n# Same Content"), 0o644)

	// Release with same content (no enabled field in embedded)
	fsys := &testFSAdapter{&testFS{skills: map[string]string{
		"test-skill": "---\nname: test-skill\n---\n# Same Content",
	}}}
	mgr.ReleaseSkills(fsys)

	data, _ := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	content := string(data)
	if !strings.Contains(content, "enabled: false") {
		t.Error("should have preserved user's enabled: false (content unchanged)")
	}
}

func TestReleaseSkills_NewSkill(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	fsys := &testFSAdapter{&testFS{skills: map[string]string{
		"new-skill": "---\nname: new-skill\n---\n# New Skill",
	}}}
	mgr.ReleaseSkills(fsys)

	data, _ := os.ReadFile(filepath.Join(dir, ".claude", "skills", "new-skill", "SKILL.md"))
	if !strings.Contains(string(data), "New Skill") {
		t.Error("new skill should be written")
	}
}

func TestReleaseSkills_PlatformFilter(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Use an OS that doesn't match runtime.GOOS
	fakeOS := "fakeos"
	fsys := &testFSAdapter{&testFS{skills: map[string]string{
		"platform-skill": "---\nname: platform-skill\nos: [\"" + fakeOS + "\"]\n---\n# Platform",
	}}}
	mgr.ReleaseSkills(fsys)

	skillDir := filepath.Join(dir, ".claude", "skills", "platform-skill")
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Error("platform-mismatched skill should not be released")
	}
}

func TestReleaseSkills_CleanupMismatch(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Pre-create a skill directory (simulating previous release on different platform)
	skillDir := filepath.Join(dir, ".claude", "skills", "mac-only")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("old"), 0o644)

	// Release with non-matching OS
	fsys := &testFSAdapter{&testFS{skills: map[string]string{
		"mac-only": "---\nname: mac-only\nos: [\"fakeos\"]\n---\n# Mac Only",
	}}}
	mgr.ReleaseSkills(fsys)

	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Error("should have cleaned up platform-mismatched skill directory")
	}
}

func TestReleaseSkills_PreserveEnabled(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Write on disk with enabled: false
	skillDir := filepath.Join(dir, ".claude", "skills", "test-skill")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: test-skill\nenabled: false\n---\n# Old"), 0o644)

	// Release with new content (which has enabled: true by default)
	fsys := &testFSAdapter{&testFS{skills: map[string]string{
		"test-skill": "---\nname: test-skill\nenabled: true\n---\n# New",
	}}}
	mgr.ReleaseSkills(fsys)

	data, _ := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if !strings.Contains(string(data), "enabled: false") {
		t.Errorf("should have preserved enabled: false, got: %s", string(data))
	}
}
