package pdf

func defaultDarwinCreateFallbackRenderer() createRenderer {
	return nil
}

func createAlignedTextX(left, availableWidth, textWidth float64, align string) float64 {
	if align != "R" {
		return left
	}
	x := left + availableWidth - textWidth
	if x < left {
		return left
	}
	return x
}

func createBottomOriginY(top, height float64) float64 {
	y := createPageHeight - top - height
	if y < 0 {
		return 0
	}
	return y
}
