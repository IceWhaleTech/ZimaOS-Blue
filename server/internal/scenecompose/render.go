package scenecompose

import (
	"image"
	"image/color"
	stdDraw "image/draw"
	"math"

	xdraw "golang.org/x/image/draw"
)

type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

type renderForeground struct {
	Plan  ForegroundPlan
	Asset *ResolvedImage
	Cut   *CutoutResult
}

type placement struct {
	x        int
	y        int
	w        int
	h        int
	groundY  int
	grounded bool
}

func (r *Renderer) Render(width, height int, background *ResolvedImage, foregrounds []renderForeground) image.Image {
	if width <= 0 {
		width = 1280
	}
	if height <= 0 {
		height = 896
	}
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	if background != nil && background.Image != nil {
		drawCover(canvas, background.Image)
	} else {
		fillGradient(canvas, color.RGBA{R: 26, G: 53, B: 76, A: 255}, color.RGBA{R: 12, G: 22, B: 31, A: 255})
	}

	placements := make([]placement, 0, len(foregrounds))
	for idx, fg := range foregrounds {
		placements = append(placements, layoutPlacement(width, height, fg.Plan.Layout, idx))
	}
	if len(placements) > 1 && rectOverlap(placements[0], placements[1]) {
		if placements[1].x >= placements[0].x {
			placements[1].x = minInt(width-placements[1].w/2-24, placements[1].x+int(0.12*float64(width)))
		} else {
			placements[1].x = maxInt(placements[1].w/2+24, placements[1].x-int(0.12*float64(width)))
		}
	}

	for idx, fg := range foregrounds {
		drawForeground(canvas, background, fg, placements[idx])
	}
	applyVignette(canvas)
	applyFilmGrain(canvas, 4)
	return canvas
}

func drawForeground(canvas *image.RGBA, background *ResolvedImage, fg renderForeground, pos placement) {
	if fg.Cut == nil || fg.Cut.Image == nil {
		return
	}
	resized := resizeTo(fg.Cut.Image, pos.w, pos.h)
	if resized == nil {
		return
	}
	if pos.grounded {
		drawShadow(canvas, pos)
	}
	tuned := tintTowardBackground(resized, sampleBackgroundColor(canvas, pos))
	topLeft := image.Pt(pos.x-pos.w/2, pos.groundY-pos.h)
	rect := image.Rectangle{Min: topLeft, Max: topLeft.Add(tuned.Bounds().Size())}
	stdDraw.Draw(canvas, rect, tuned, tuned.Bounds().Min, stdDraw.Over)
}

func layoutPlacement(width, height int, layout LayoutHint, idx int) placement {
	xMap := map[string]float64{"left": 0.28, "center": 0.50, "right": 0.72}
	yMap := map[string]float64{"low": 0.68, "middle": 0.58, "high": 0.42}
	scaleMap := map[string]float64{"small": 0.22, "medium": 0.32, "large": 0.42}
	x := xMap[normalizeEnum(layout.Horizontal, []string{"left", "center", "right"}, "center")]
	y := yMap[normalizeEnum(layout.Vertical, []string{"low", "middle", "high"}, "low")]
	scale := scaleMap[normalizeEnum(layout.Scale, []string{"small", "medium", "large"}, "medium")]
	h := maxInt(80, int(scale*float64(height)))
	w := int(float64(h) * 0.9)
	if idx > 0 {
		w = int(float64(w) * 0.92)
	}
	cx := int(x * float64(width))
	cy := int(y * float64(height))
	top := cy - h/2
	bottom := cy + h/2
	if layout.Grounded {
		bottom = minInt(int(0.88*float64(height)), maxInt(int(0.12*float64(height))+h, int(float64(height)*0.82)))
		switch normalizeEnum(layout.Vertical, []string{"low", "middle", "high"}, "low") {
		case "middle":
			bottom = int(float64(height) * 0.72)
		case "high":
			bottom = int(float64(height) * 0.58)
		default:
			bottom = int(float64(height) * 0.82)
		}
		top = bottom - h
	}
	if top < int(0.12*float64(height)) {
		top = int(0.12 * float64(height))
		bottom = top + h
	}
	if bottom > int(0.88*float64(height)) {
		bottom = int(0.88 * float64(height))
		top = bottom - h
	}
	if cx-w/2 < 12 {
		cx = w/2 + 12
	}
	if cx+w/2 > width-12 {
		cx = width - w/2 - 12
	}
	return placement{
		x:        cx,
		y:        top + h/2,
		w:        w,
		h:        h,
		groundY:  bottom,
		grounded: layout.Grounded,
	}
}

func rectOverlap(a, b placement) bool {
	left := maxInt(a.x-a.w/2, b.x-b.w/2)
	right := minInt(a.x+a.w/2, b.x+b.w/2)
	top := maxInt(a.groundY-a.h, b.groundY-b.h)
	bottom := minInt(a.groundY, b.groundY)
	return right-left > minInt(a.w, b.w)/4 && bottom-top > minInt(a.h, b.h)/4
}

func drawCover(dst *image.RGBA, src image.Image) {
	sw := src.Bounds().Dx()
	sh := src.Bounds().Dy()
	if sw <= 0 || sh <= 0 {
		fillGradient(dst, color.RGBA{R: 26, G: 53, B: 76, A: 255}, color.RGBA{R: 12, G: 22, B: 31, A: 255})
		return
	}
	dw := dst.Bounds().Dx()
	dh := dst.Bounds().Dy()
	scale := math.Max(float64(dw)/float64(sw), float64(dh)/float64(sh))
	tw := maxInt(1, int(float64(sw)*scale))
	th := maxInt(1, int(float64(sh)*scale))
	scaled := image.NewRGBA(image.Rect(0, 0, tw, th))
	xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), src, src.Bounds(), stdDraw.Over, nil)
	offset := image.Pt((tw-dw)/2, (th-dh)/2)
	stdDraw.Draw(dst, dst.Bounds(), scaled, offset, stdDraw.Src)
}

func resizeTo(src image.Image, width, height int) *image.RGBA {
	if src == nil || width <= 0 || height <= 0 {
		return nil
	}
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), stdDraw.Over, nil)
	return dst
}

func fillGradient(dst *image.RGBA, top, bottom color.RGBA) {
	bounds := dst.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		t := float64(y-bounds.Min.Y) / float64(maxInt(1, bounds.Dy()-1))
		c := blendColor(top, bottom, t)
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dst.SetRGBA(x, y, c)
		}
	}
}

func drawShadow(dst *image.RGBA, pos placement) {
	shadowW := maxInt(32, int(float64(pos.w)*0.78))
	shadowH := maxInt(14, int(float64(pos.h)*0.14))
	cx := pos.x
	cy := minInt(dst.Bounds().Max.Y-1, pos.groundY+shadowH/4)
	rx := float64(shadowW) / 2
	ry := float64(shadowH) / 2
	for y := cy - shadowH; y <= cy+shadowH; y++ {
		if y < 0 || y >= dst.Bounds().Max.Y {
			continue
		}
		for x := cx - shadowW; x <= cx+shadowW; x++ {
			if x < 0 || x >= dst.Bounds().Max.X {
				continue
			}
			nx := float64(x-cx) / rx
			ny := float64(y-cy) / ry
			dist := nx*nx + ny*ny
			if dist > 1.4 {
				continue
			}
			alpha := uint8(math.Max(0, 56*(1-dist/1.4)))
			existing := dst.RGBAAt(x, y)
			dst.SetRGBA(x, y, alphaBlend(existing, color.RGBA{A: alpha}))
		}
	}
}

func sampleBackgroundColor(src *image.RGBA, pos placement) color.RGBA {
	bounds := src.Bounds()
	sampleY := minInt(bounds.Max.Y-1, maxInt(bounds.Min.Y, pos.groundY-pos.h/2))
	sampleX := minInt(bounds.Max.X-1, maxInt(bounds.Min.X, pos.x))
	return src.RGBAAt(sampleX, sampleY)
}

func tintTowardBackground(src *image.RGBA, bg color.RGBA) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	targetBrightness := brightness(bg)
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			c := src.RGBAAt(x, y)
			if c.A == 0 {
				continue
			}
			factor := 0.92 + 0.16*targetBrightness
			r := clampUint8(float64(c.R) * factor)
			g := clampUint8(float64(c.G) * factor)
			b := clampUint8(float64(c.B) * factor)
			if bg.R > bg.B+18 {
				r = clampUint8(float64(r) * 1.04)
			}
			if bg.B > bg.R+18 {
				b = clampUint8(float64(b) * 1.04)
			}
			dst.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: c.A})
		}
	}
	return dst
}

func applyVignette(dst *image.RGBA) {
	bounds := dst.Bounds()
	cx := float64(bounds.Dx()) / 2
	cy := float64(bounds.Dy()) / 2
	maxDist := math.Sqrt(cx*cx + cy*cy)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := dst.RGBAAt(x, y)
			dx := float64(x-bounds.Min.X) - cx
			dy := float64(y-bounds.Min.Y) - cy
			dist := math.Sqrt(dx*dx+dy*dy) / maxDist
			factor := 1 - 0.18*math.Pow(dist, 1.8)
			dst.SetRGBA(x, y, color.RGBA{
				R: clampUint8(float64(c.R) * factor),
				G: clampUint8(float64(c.G) * factor),
				B: clampUint8(float64(c.B) * factor),
				A: c.A,
			})
		}
	}
}

func applyFilmGrain(dst *image.RGBA, strength int) {
	if strength <= 0 {
		return
	}
	bounds := dst.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := dst.RGBAAt(x, y)
			noise := ((x*13 + y*29) % (strength*2 + 1)) - strength
			dst.SetRGBA(x, y, color.RGBA{
				R: clampUint8(float64(int(c.R) + noise)),
				G: clampUint8(float64(int(c.G) + noise)),
				B: clampUint8(float64(int(c.B) + noise)),
				A: c.A,
			})
		}
	}
}

func brightness(c color.RGBA) float64 {
	return (0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)) / 255.0
}

func blendColor(a, b color.RGBA, t float64) color.RGBA {
	return color.RGBA{
		R: clampUint8(float64(a.R)*(1-t) + float64(b.R)*t),
		G: clampUint8(float64(a.G)*(1-t) + float64(b.G)*t),
		B: clampUint8(float64(a.B)*(1-t) + float64(b.B)*t),
		A: 255,
	}
}

func alphaBlend(dst, src color.RGBA) color.RGBA {
	alpha := float64(src.A) / 255
	return color.RGBA{
		R: clampUint8(float64(dst.R) * (1 - alpha)),
		G: clampUint8(float64(dst.G) * (1 - alpha)),
		B: clampUint8(float64(dst.B) * (1 - alpha)),
		A: 255,
	}
}

func clampUint8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v + 0.5)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func imageHasAlpha(src image.Image) bool {
	if src == nil {
		return false
	}
	bounds := src.Bounds()
	if bounds.Empty() {
		return false
	}
	stepX := maxInt(1, bounds.Dx()/24)
	stepY := maxInt(1, bounds.Dy()/24)
	for y := bounds.Min.Y; y < bounds.Max.Y; y += stepY {
		for x := bounds.Min.X; x < bounds.Max.X; x += stepX {
			_, _, _, a := src.At(x, y).RGBA()
			if a < 0xffff {
				return true
			}
		}
	}
	return false
}

func cloneImage(src image.Image) *image.RGBA {
	if src == nil {
		return nil
	}
	dst := image.NewRGBA(src.Bounds())
	stdDraw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, stdDraw.Src)
	return dst
}

func applyMask(src image.Image, mask *image.Alpha) *image.RGBA {
	if src == nil {
		return nil
	}
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.RGBAModel.Convert(src.At(x, y)).(color.RGBA)
			if mask != nil {
				c.A = mask.AlphaAt(x, y).A
			}
			dst.SetRGBA(x, y, c)
		}
	}
	return dst
}

func featherMask(src *image.Alpha, radius int) *image.Alpha {
	if src == nil || radius <= 0 {
		return src
	}
	bounds := src.Bounds()
	dst := image.NewAlpha(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			total := 0
			count := 0
			for oy := -radius; oy <= radius; oy++ {
				for ox := -radius; ox <= radius; ox++ {
					nx := x + ox
					ny := y + oy
					if nx < bounds.Min.X || nx >= bounds.Max.X || ny < bounds.Min.Y || ny >= bounds.Max.Y {
						continue
					}
					total += int(src.AlphaAt(nx, ny).A)
					count++
				}
			}
			dst.SetAlpha(x, y, color.Alpha{A: uint8(total / maxInt(1, count))})
		}
	}
	return dst
}
