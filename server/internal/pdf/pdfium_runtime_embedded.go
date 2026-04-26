//go:build !darwin

package pdf

import _ "embed"

// embeddedPDFiumRuntimeWASM is pinned to the upstream go-pdfium runtime asset
// at pdfiumRuntimeRepoRef and embedded for non-macOS builds.
//
//go:embed testdata/pdfium.wasm
var embeddedPDFiumRuntimeWASM []byte
