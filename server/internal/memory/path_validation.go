package memory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	// ErrInvalidPath indicates a caller supplied a memory path outside the memory root.
	ErrInvalidPath = errors.New("invalid memory path")
	// ErrInvalidDailyLogDate indicates a caller supplied an invalid daily log date.
	ErrInvalidDailyLogDate = errors.New("invalid daily log date")
)

func resolveMemoryPath(root, relPath string) (string, error) {
	cleanRoot := filepath.Clean(strings.TrimSpace(root))
	cleanRelPath := strings.TrimSpace(relPath)
	if cleanRoot == "" || cleanRelPath == "" || filepath.IsAbs(cleanRelPath) {
		return "", ErrInvalidPath
	}

	candidate := filepath.Clean(filepath.Join(cleanRoot, cleanRelPath))
	rel, err := filepath.Rel(cleanRoot, candidate)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", ErrInvalidPath
	}

	return candidate, nil
}

func normalizeDailyLogDate(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%w: date is required", ErrInvalidDailyLogDate)
	}

	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil || parsed.Format("2006-01-02") != trimmed {
		return "", fmt.Errorf("%w: expected YYYY-MM-DD", ErrInvalidDailyLogDate)
	}

	return trimmed, nil
}
