//go:build !darwin

package pdf

func defaultCreateRendererFactory() createRenderer {
	return createFPDFRenderer{}
}
