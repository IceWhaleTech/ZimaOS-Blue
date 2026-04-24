//go:build !darwin

package pdf

import "testing"

func TestDefaultCreateRendererFactoryUsesFPDFOnNonDarwin(t *testing.T) {
	renderer := defaultCreateRendererFactory()
	if _, ok := renderer.(createFPDFRenderer); !ok {
		t.Fatalf("defaultCreateRendererFactory() = %T, want createFPDFRenderer", renderer)
	}
}
