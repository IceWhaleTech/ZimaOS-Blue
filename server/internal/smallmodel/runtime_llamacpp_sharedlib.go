package smallmodel

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	smallModelLlamaLibDirEnv       = "SMALL_MODEL_LLAMA_LIB_DIR"
	smallModelLlamaCompatLibDirEnv = "ZIMAOS_AI_LIB"
	smallModelLlamaLoaderLibDirEnv = "ZIMAOS_BLUE_LIB"
)

func resolveLlamaCppSharedLibFile(libName string) (string, string, error) {
	candidates := llamaCppLibDirCandidates()
	checked := make([]string, 0, len(candidates))
	seen := map[string]struct{}{}

	for _, dir := range candidates {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		absDir, err := filepath.Abs(dir)
		if err != nil {
			absDir = dir
		}
		if _, ok := seen[absDir]; ok {
			continue
		}
		seen[absDir] = struct{}{}

		libFile := libraryFilename(absDir, libName)
		checked = append(checked, libFile)
		if _, statErr := os.Stat(libFile); statErr == nil {
			return absDir, libFile, nil
		}
	}

	return "", "", fmt.Errorf(
		"llama runtime library not found (set %s or %s; checked %d path(s): %s)",
		smallModelLlamaLibDirEnv,
		smallModelLlamaLoaderLibDirEnv,
		len(checked),
		strings.Join(checked, ", "),
	)
}

func llamaCppLibDirCandidates() []string {
	candidates := []string{
		strings.TrimSpace(os.Getenv(smallModelLlamaLibDirEnv)),
		strings.TrimSpace(os.Getenv(smallModelLlamaLoaderLibDirEnv)),
		strings.TrimSpace(os.Getenv(smallModelLlamaCompatLibDirEnv)),
		"pkg/api/libs",
		"libs",
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, "pkg", "api", "libs"),
			filepath.Join(exeDir, "libs"),
		)
	}

	return candidates
}

func libraryFilename(dir, lib string) string {
	switch runtime.GOOS {
	case "linux", "freebsd":
		return filepath.Join(dir, fmt.Sprintf("lib%s.so", lib))
	case "windows":
		return filepath.Join(dir, fmt.Sprintf("%s.dll", lib))
	case "darwin":
		return filepath.Join(dir, fmt.Sprintf("lib%s.dylib", lib))
	default:
		return filepath.Join(dir, lib)
	}
}
