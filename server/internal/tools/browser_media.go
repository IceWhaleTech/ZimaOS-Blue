package tools

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func decodeBrowserScreenshotBase64(encoded string) ([]byte, error) {
	trimmed := strings.TrimSpace(encoded)
	if trimmed == "" {
		return nil, fmt.Errorf("screenshot payload is empty")
	}
	if idx := strings.Index(trimmed, ","); idx >= 0 && strings.Contains(trimmed[:idx], ";base64") {
		trimmed = trimmed[idx+1:]
	}
	raw, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("decode screenshot: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("screenshot payload is empty")
	}
	return raw, nil
}

func browserScreenshotStorageDir(mediaDir string) string {
	mediaDir = strings.TrimSpace(mediaDir)
	if mediaDir != "" {
		return filepath.Join(mediaDir, "browser")
	}
	return filepath.Join(os.TempDir(), "zimaos-blue", "browser")
}

// SaveBrowserScreenshotBase64 persists a browser screenshot and returns the saved file path.
func SaveBrowserScreenshotBase64(mediaDir string, encoded string) (string, error) {
	raw, err := decodeBrowserScreenshotBase64(encoded)
	if err != nil {
		return "", err
	}
	dir := browserScreenshotStorageDir(mediaDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create browser media dir: %w", err)
	}
	file, err := os.CreateTemp(dir, "screenshot-*.png")
	if err != nil {
		return "", fmt.Errorf("create screenshot file: %w", err)
	}
	path := file.Name()
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("write screenshot file: %w", err)
	}
	if err := file.Chmod(0o644); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("chmod screenshot file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close screenshot file: %w", err)
	}
	return path, nil
}
