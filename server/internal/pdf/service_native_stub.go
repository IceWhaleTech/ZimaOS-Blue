//go:build !darwin

package pdf

import (
	"context"
	"os"
)

func nativePDFEngineName() string {
	return engineName
}

func nativePDFShouldPreferInfo() bool {
	return false
}

func nativePDFShouldPreferExtract(_ ExtractRequest) bool {
	return false
}

func nativePDFResultShouldShortCircuit(_ ExtractRequest, _ ExtractResult) bool {
	return false
}

func tryNativePDFInfo(_ context.Context, _ string, _ os.FileInfo) (DocumentInfo, bool, error) {
	return DocumentInfo{}, false, nil
}

func tryNativePDFExtract(_ context.Context, _ ExtractRequest, _ string, _ os.FileInfo) (ExtractResult, bool, error) {
	return ExtractResult{}, false, nil
}
