package workspace

import (
	"os"
	"path/filepath"
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
		path := filepath.Join(dir, name)
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

	for _, name := range []string{FileSOUL, FileUSER, FileIDENTITY, FileAGENTS, FileMEMORY, FileHEARTBEAT} {
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

func TestWorkspaceTool(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	mgr.EnsureWorkspace()

	tool := NewWorkspaceTool(mgr)

	// Check definition
	def := tool.Definition()
	if def.Name != "workspace_file" {
		t.Errorf("unexpected tool name: %s", def.Name)
	}

	// Write via tool
	_, err := tool.Execute(nil, map[string]interface{}{
		"action":   "write",
		"filename": FileUSER,
		"content":  "# Updated User",
	})
	if err != nil {
		t.Fatalf("tool write: %v", err)
	}

	// Read via tool
	result, err := tool.Execute(nil, map[string]interface{}{
		"action":   "read",
		"filename": FileUSER,
	})
	if err != nil {
		t.Fatalf("tool read: %v", err)
	}
	if !strings.Contains(result.(string), "Updated User") {
		t.Errorf("expected result to contain 'Updated User', got: %s", result)
	}

	// Append daily via tool
	_, err = tool.Execute(nil, map[string]interface{}{
		"action":  "append_daily",
		"content": "Test daily entry",
	})
	if err != nil {
		t.Fatalf("tool append_daily: %v", err)
	}

	// Complete bootstrap via tool
	_, err = tool.Execute(nil, map[string]interface{}{
		"action": "complete_bootstrap",
	})
	if err != nil {
		t.Fatalf("tool complete_bootstrap: %v", err)
	}
	if mgr.IsBootstrapPending() {
		t.Error("bootstrap should be completed after tool call")
	}

	// Invalid action
	_, err = tool.Execute(nil, map[string]interface{}{
		"action":   "delete",
		"filename": FileUSER,
	})
	if err == nil {
		t.Error("expected error for invalid action")
	}
}

func TestLocaleTemplates(t *testing.T) {
	// Chinese locale
	dir := t.TempDir()
	mgr := NewManager(dir)
	mgr.SetLocale("zh")
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
	ts := getTemplates("fr")
	if !strings.Contains(ts.soul, "AI Assistant") {
		t.Error("unknown locale should fall back to English")
	}

	// BCP-47 prefix matching
	ts = getTemplates("zh-TW")
	if !strings.Contains(ts.soul, "AI 助手") {
		t.Error("zh-TW should match zh templates")
	}
}
