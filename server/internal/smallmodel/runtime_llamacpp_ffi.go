package smallmodel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/loader"
	"github.com/jupiterrider/ffi"
)

const (
	smallModelLlamaLibDirEnv       = "SMALL_MODEL_LLAMA_LIB_DIR"
	smallModelLlamaCompatLibDirEnv = "ZIMAOS_AI_LIB"
)

// llamaCppFFIBackend performs runtime-level shared-library probes via libffi.
// Inference stays on the existing server/CLI path for stability.
type llamaCppFFIBackend struct {
	once sync.Once
	err  error

	libDir   string
	llamaLib ffi.Lib
	mtmdLib  ffi.Lib
}

func newLlamaCppFFIBackend() *llamaCppFFIBackend {
	return &llamaCppFFIBackend{}
}

func (b *llamaCppFFIBackend) EnsureLoaded() error {
	if b == nil {
		return fmt.Errorf("llama ffi backend is nil")
	}
	b.once.Do(func() {
		b.err = b.load()
	})
	return b.err
}

func (b *llamaCppFFIBackend) load() error {
	libDir, libFile, err := resolveLlamaCppSharedLibFile("llama")
	if err != nil {
		return err
	}
	b.libDir = libDir

	llamaLib, err := loader.LoadLibrary(libDir, "llama")
	if err != nil {
		return fmt.Errorf("load llama shared library %q: %w", libFile, err)
	}
	b.llamaLib = llamaLib

	if err := verifyLlamaFFISymbols(llamaLib); err != nil {
		return err
	}

	// mtmd is optional at probe stage; server/CLI path still handles multimodal inference.
	if loader.LibraryExists(libDir, "mtmd") {
		mtmdLib, mtmdErr := loader.LoadLibrary(libDir, "mtmd")
		if mtmdErr != nil {
			return fmt.Errorf("load mtmd shared library from %q: %w", libDir, mtmdErr)
		}
		b.mtmdLib = mtmdLib
	}

	return nil
}

func verifyLlamaFFISymbols(lib ffi.Lib) error {
	if _, err := lib.Prep("llama_backend_init", &ffi.TypeVoid); err != nil {
		return fmt.Errorf("llama ffi symbol missing: llama_backend_init: %w", err)
	}
	if _, err := lib.Prep("llama_backend_free", &ffi.TypeVoid); err != nil {
		return fmt.Errorf("llama ffi symbol missing: llama_backend_free: %w", err)
	}
	if _, err := lib.Prep("llama_model_load_from_file", &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer); err == nil {
		return nil
	}
	if _, err := lib.Prep("llama_load_model_from_file", &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer); err == nil {
		return nil
	}
	return fmt.Errorf("llama ffi symbols missing: llama_model_load_from_file/llama_load_model_from_file")
}

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

		libFile := loader.LibraryFilename(absDir, libName)
		checked = append(checked, libFile)
		if _, statErr := os.Stat(libFile); statErr == nil {
			return absDir, libFile, nil
		}
	}

	return "", "", fmt.Errorf(
		"llama runtime library not found (set %s or %s; checked %d path(s): %s)",
		smallModelLlamaLibDirEnv,
		loader.EnvLibPath,
		len(checked),
		strings.Join(checked, ", "),
	)
}

func llamaCppLibDirCandidates() []string {
	candidates := []string{
		strings.TrimSpace(os.Getenv(smallModelLlamaLibDirEnv)),
		strings.TrimSpace(os.Getenv(loader.EnvLibPath)),
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
