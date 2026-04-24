//go:build darwin

package pdf

type createDarwinRenderer struct{}

var darwinCreateDocumentNativeRenderFunc = renderCreatePDFDarwinNative

func (createDarwinRenderer) render(lines []createStyledLine, fontPlan createFontPlan) ([]byte, int, int, error) {
	return renderCreateWithOptionalFallback(darwinCreateDocumentNativeRenderFunc, defaultDarwinCreateFallbackRenderer(), lines, fontPlan)
}
