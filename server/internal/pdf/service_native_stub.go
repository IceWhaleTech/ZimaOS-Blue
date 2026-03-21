//go:build !darwin

package pdf

import (
	"context"
	"os"
)

func nativePDFEngineName() string {
	return engineName
}

func tryNativePDFInfo(_ context.Context, _ string, _ os.FileInfo) (DocumentInfo, bool, error) {
	return DocumentInfo{}, false, nil
}

func tryNativePDFExtract(_ context.Context, _ ExtractRequest, _ string, _ os.FileInfo) (ExtractResult, bool, error) {
	return ExtractResult{}, false, nil
}
