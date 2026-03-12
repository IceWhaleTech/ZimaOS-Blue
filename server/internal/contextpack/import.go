package contextpack

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ImportDir(src, registryRoot string) error {
	src = strings.TrimSpace(src)
	registryRoot = strings.TrimSpace(registryRoot)
	if src == "" || registryRoot == "" {
		return fmt.Errorf("source and registry root are required")
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("source must be a directory")
	}
	if err := os.MkdirAll(registryRoot, 0o755); err != nil {
		return err
	}

	destRoot := registryRoot
	if hasDirectBucket(src) {
		destRoot = filepath.Join(registryRoot, filepath.Base(src))
	}

	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(destRoot, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}

func hasDirectBucket(root string) bool {
	for _, name := range []string{"docs", "skills"} {
		if info, err := os.Stat(filepath.Join(root, name)); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Chmod(0o644)
}
