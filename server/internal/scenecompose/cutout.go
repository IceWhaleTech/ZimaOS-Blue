package scenecompose

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
)

type CutoutStrategy struct {
	manager CutoutManager
}

func NewCutoutStrategy(manager CutoutManager) *CutoutStrategy {
	return &CutoutStrategy{manager: manager}
}

func (s *CutoutStrategy) Apply(ctx context.Context, src *ResolvedImage) (*CutoutResult, error) {
	if src == nil || src.Image == nil {
		return nil, fmt.Errorf("cutout source image is missing")
	}
	if src.HasAlpha {
		return &CutoutResult{Image: cloneImage(src.Image)}, nil
	}
	simple := simpleBackgroundCutout(src.Image)
	if simple != nil && simple.Mask != nil {
		if s.manager == nil {
			return simple, nil
		}
		if s.manager.IsReady() {
			if refined, err := s.manager.Cutout(ctx, src.Image); err == nil && refined != nil && refined.Mask != nil {
				return refineCutoutResult(refined), nil
			}
		} else {
			s.manager.EnsureReadyAsync(ctx)
		}
		return refineCutoutResult(simple), nil
	}

	if s.manager != nil {
		if s.manager.IsReady() {
			if refined, err := s.manager.Cutout(ctx, src.Image); err == nil && refined != nil && refined.Mask != nil {
				return refineCutoutResult(refined), nil
			}
		} else {
			s.manager.EnsureReadyAsync(ctx)
		}
	}
	return nil, fmt.Errorf("cutout strategy could not isolate the subject")
}

func simpleBackgroundCutout(src image.Image) *CutoutResult {
	bounds := src.Bounds()
	if bounds.Empty() {
		return nil
	}
	mask := image.NewAlpha(bounds)
	bg := averageCornerColor(src)
	foregroundCount := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.RGBAModel.Convert(src.At(x, y)).(color.RGBA)
			dist := colorDistance(bg, c)
			alpha := uint8(255)
			switch {
			case dist < 18:
				alpha = 0
			case dist < 36:
				alpha = uint8((dist - 18) * 14)
			}
			mask.SetAlpha(x, y, color.Alpha{A: alpha})
			if alpha > 0 {
				foregroundCount++
			}
		}
	}
	if foregroundCount == 0 {
		return nil
	}
	mask = featherMask(mask, 3)
	return &CutoutResult{
		Image: applyMask(src, mask),
		Mask:  mask,
	}
}

func refineCutoutResult(result *CutoutResult) *CutoutResult {
	if result == nil || result.Mask == nil {
		return result
	}
	result.Mask = featherMask(result.Mask, 3)
	result.Image = applyMask(result.Image, result.Mask)
	return result
}

func averageCornerColor(src image.Image) color.RGBA {
	bounds := src.Bounds()
	samples := []image.Point{
		bounds.Min,
		{X: bounds.Max.X - 1, Y: bounds.Min.Y},
		{X: bounds.Min.X, Y: bounds.Max.Y - 1},
		{X: bounds.Max.X - 1, Y: bounds.Max.Y - 1},
	}
	var r, g, b float64
	for _, point := range samples {
		c := color.RGBAModel.Convert(src.At(point.X, point.Y)).(color.RGBA)
		r += float64(c.R)
		g += float64(c.G)
		b += float64(c.B)
	}
	return color.RGBA{
		R: uint8(r / float64(len(samples))),
		G: uint8(g / float64(len(samples))),
		B: uint8(b / float64(len(samples))),
		A: 255,
	}
}

func colorDistance(a, b color.RGBA) float64 {
	dr := float64(a.R) - float64(b.R)
	dg := float64(a.G) - float64(b.G)
	db := float64(a.B) - float64(b.B)
	return math.Sqrt(dr*dr + dg*dg + db*db)
}
