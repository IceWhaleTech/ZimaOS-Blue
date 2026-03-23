package skillbundle

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func FindArchiveInstallRoot(extractDir, skillID string) (string, string, error) {
	entry, err := FindEntryDocument(extractDir, skillID)
	if err != nil {
		return "", "", err
	}
	if _, err := EnsureCompatibilitySkillDoc(entry.Dir, entry.Path); err != nil {
		return "", "", err
	}
	return entry.Dir, entry.Path, nil
}

func ArchiveExtension(primaryURL, fallbackURL, contentType string) string {
	for _, raw := range []string{primaryURL, fallbackURL} {
		lower := strings.ToLower(strings.TrimSpace(raw))
		switch {
		case strings.Contains(lower, ".tar.gz"):
			return ".tar.gz"
		case strings.Contains(lower, ".tgz"):
			return ".tgz"
		case strings.Contains(lower, ".zip"):
			return ".zip"
		}
	}
	lowerType := strings.ToLower(contentType)
	switch {
	case strings.Contains(lowerType, "zip"):
		return ".zip"
	case strings.Contains(lowerType, "gzip"), strings.Contains(lowerType, "tar"):
		return ".tar.gz"
	default:
		return ".archive"
	}
}

func ExtractArchiveFile(archivePath, extractDir, downloadURL, contentType string) error {
	format, err := DetectArchiveFormat(archivePath, downloadURL, contentType)
	if err != nil {
		return err
	}
	switch format {
	case "zip":
		return extractZipArchive(archivePath, extractDir)
	case "tar.gz":
		return extractTarGzArchive(archivePath, extractDir)
	default:
		return fmt.Errorf("unsupported archive format: %s", format)
	}
}

func DetectArchiveFormat(archivePath, downloadURL, contentType string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	header := make([]byte, 4)
	n, err := file.Read(header)
	if err != nil && err != io.EOF {
		return "", err
	}
	header = header[:n]
	if len(header) >= 2 && header[0] == 'P' && header[1] == 'K' {
		return "zip", nil
	}
	if len(header) >= 2 && header[0] == 0x1f && header[1] == 0x8b {
		return "tar.gz", nil
	}

	lowerURL := strings.ToLower(strings.TrimSpace(downloadURL))
	switch {
	case strings.Contains(lowerURL, ".zip"):
		return "zip", nil
	case strings.Contains(lowerURL, ".tar.gz"), strings.Contains(lowerURL, ".tgz"):
		return "tar.gz", nil
	}

	lowerType := strings.ToLower(contentType)
	switch {
	case strings.Contains(lowerType, "zip"):
		return "zip", nil
	case strings.Contains(lowerType, "gzip"), strings.Contains(lowerType, "tar"):
		return "tar.gz", nil
	default:
		return "", fmt.Errorf("unsupported archive format for %s", downloadURL)
	}
}

func extractZipArchive(archivePath, extractDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		target, err := SafeArchiveTarget(extractDir, file.Name)
		if err != nil {
			return err
		}
		if target == "" {
			continue
		}
		mode := file.Mode()
		if mode&os.ModeSymlink != 0 {
			return fmt.Errorf("zip symlink entry is not supported: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		perm := mode.Perm()
		if perm == 0 {
			perm = 0o644
		}
		dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(dst, rc); err != nil {
			dst.Close()
			rc.Close()
			return err
		}
		if err := dst.Close(); err != nil {
			rc.Close()
			return err
		}
		if err := rc.Close(); err != nil {
			return err
		}
	}
	return nil
}

func extractTarGzArchive(archivePath, extractDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := SafeArchiveTarget(extractDir, header.Name)
		if err != nil {
			return err
		}
		if target == "" {
			continue
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			perm := os.FileMode(header.Mode).Perm()
			if perm == 0 {
				perm = 0o644
			}
			dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
			if err != nil {
				return err
			}
			if _, err := io.Copy(dst, reader); err != nil {
				dst.Close()
				return err
			}
			if err := dst.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("tar symlink entry is not supported: %s", header.Name)
		}
	}
}

func SafeArchiveTarget(root, name string) (string, error) {
	cleaned := strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	cleaned = strings.TrimPrefix(cleaned, "/")
	cleaned = filepath.Clean(filepath.FromSlash(cleaned))
	if cleaned == "." || cleaned == "" {
		return "", nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry escapes destination: %s", name)
	}
	target := filepath.Join(root, cleaned)
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry escapes destination: %s", name)
	}
	return target, nil
}
