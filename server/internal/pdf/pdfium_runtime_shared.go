//go:build !darwin

package pdf

import (
	"context"
	"fmt"

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
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(embeddedPDFiumRuntimeWASM) == 0 {
		return nil, fmt.Errorf("embedded PDF runtime %s is empty", pdfiumRuntimeFileName)
	}
	return embeddedPDFiumRuntimeWASM, nil
}
