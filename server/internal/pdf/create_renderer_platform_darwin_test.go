//go:build darwin

package pdf

import "testing"

func TestDefaultCreateRendererFactoryUsesDarwinRendererOnDarwin(t *testing.T) {
	renderer := defaultCreateRendererFactory()
	if _, ok := renderer.(createDarwinRenderer); !ok {
		t.Fatalf("defaultCreateRendererFactory() = %T, want createDarwinRenderer", renderer)
	}
}
