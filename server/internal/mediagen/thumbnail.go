package mediagen

import (
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const thumbnailMaxDim = 384

// generateThumbnail creates a JPEG thumbnail for an image file.
// Returns the thumbnail's local served URL, or "" on failure (best-effort).
func (s *MediaStorage) generateThumbnail(localPath string) string {
	f, err := os.Open(localPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		log.Printf("[mediagen] thumbnail decode error: %v", err)
		return ""
	}

	thumb := resizeImage(img, thumbnailMaxDim)

	thumbDir := filepath.Join(s.baseDir, "thumbnails")
	_ = os.MkdirAll(thumbDir, 0755)

	thumbName := uuid.New().String() + ".jpg"
	thumbPath := filepath.Join(thumbDir, thumbName)

	out, err := os.Create(thumbPath)
	if err != nil {
		return ""
	}
	defer out.Close()

	if err := jpeg.Encode(out, thumb, &jpeg.Options{Quality: 75}); err != nil {
		os.Remove(thumbPath)
		return ""
	}

	return s.baseURL + "/thumbnails/" + thumbName
}

// resizeImage scales an image so its longest side is maxDim pixels.
// Uses simple nearest-neighbor for speed — thumbnails don't need high quality.
func resizeImage(src image.Image, maxDim int) image.Image {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	if w <= maxDim && h <= maxDim {
		return src
	}

	var newW, newH int
	if w >= h {
		newW = maxDim
		newH = h * maxDim / w
	} else {
		newH = maxDim
		newW = w * maxDim / h
	}
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		srcY := y * h / newH
		for x := 0; x < newW; x++ {
			srcX := x * w / newW
			dst.Set(x, y, src.At(bounds.Min.X+srcX, bounds.Min.Y+srcY))
		}
	}
	return dst
}

// isImageFile checks if a filename has an image extension.
func isImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif":
		return true
	}
	return false
}

// Register PNG decoder (JPEG is registered by image/jpeg import above).
func init() {
	// Ensure both decoders are registered for image.Decode.
	_ = png.Decode
}
