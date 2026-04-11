package pdf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillbundle"
)

func pdfiumRuntimeURLCandidates() []string {
	return skillbundle.GitHubRawURLCandidates(
		pdfiumRuntimeRepoOwner,
		pdfiumRuntimeRepoName,
		pdfiumRuntimeRepoRef,
		pdfiumRuntimeSourcePath,
	)
}

func (s *Service) ensureRuntimeWASMBytes(ctx context.Context) ([]byte, error) {
	wasmPath := filepath.Join(s.runtimeDir, pdfiumRuntimeFileName)
	if _, err := os.Stat(wasmPath); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat PDF runtime: %w", err)
		}
		if !s.autoDownload {
			return nil, fmt.Errorf("missing PDF runtime %s", pdfiumRuntimeFileName)
		}
		if err := os.MkdirAll(s.runtimeDir, 0o750); err != nil {
			return nil, fmt.Errorf("create PDF runtime dir: %w", err)
		}
		if err := s.downloadWithFallback(ctx, pdfiumRuntimeURLCandidates(), wasmPath, "PDF runtime"); err != nil {
			return nil, err
		}
	}
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		return nil, fmt.Errorf("read PDF runtime: %w", err)
	}
	return wasmBytes, nil
}
