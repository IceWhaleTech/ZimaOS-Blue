package browser

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

const relayExtensionDirName = "blue-browser-relay-extension"

//go:embed relayextension/*
var relayExtensionFS embed.FS

// EnsureRelayExtensionDir materializes Blue's unpacked browser relay extension files.
func EnsureRelayExtensionDir(root string) (string, error) {
	sub, err := fs.Sub(relayExtensionFS, "relayextension")
	if err != nil {
		return "", err
	}

	targetDir := filepath.Join(root, relayExtensionDirName)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}

	err = fs.WalkDir(sub, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		targetPath := filepath.Join(targetDir, path)
		if d.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}

		data, err := fs.ReadFile(sub, path)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, 0o644)
	})
	if err != nil {
		return "", err
	}

	return targetDir, nil
}
