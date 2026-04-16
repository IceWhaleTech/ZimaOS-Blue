package tools

import (
	"path/filepath"
	"testing"
)

func TestSaveBrowserScreenshotBase64UsesWebPExtensionForWebPDataURL(t *testing.T) {
	dir := t.TempDir()

	path, err := SaveBrowserScreenshotBase64(dir, "data:image/webp;base64,ZmFrZQ==")
	if err != nil {
		t.Fatalf("SaveBrowserScreenshotBase64() error = %v", err)
	}
	if ext := filepath.Ext(path); ext != ".webp" {
		t.Fatalf("filepath.Ext(%q) = %q, want .webp", path, ext)
	}
}
