package tools

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"strings"

	a11yruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

const a11yConversationVisualConfidenceThreshold = 0.55

type a11yConversationImageRegion struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

var a11yConversationVisualSearchRegions = []a11yConversationImageRegion{
	{X: 0, Y: 0, Width: 0.45, Height: 1},
	{X: 0, Y: 0, Width: 1, Height: 1},
}

func init() {
	a11yLocateConversationVisualHit = locateA11yConversationVisualHitFromImage
	a11yLocateConversationVisualHitFromPNG = locateA11yConversationVisualHitFromPNGBytes
}

var a11yLocateConversationVisualHitFromPNG func(ctx context.Context, imagePNG []byte, selectorName string) (a11yConversationVisualHit, error)

func locateA11yConversationVisualHitFromImage(ctx context.Context, imagePath string, selectorName string) (a11yConversationVisualHit, error) {
	var lastErr error = a11yruntime.NewError("target_not_found", "conversation visual locator did not find a unique high-confidence match", map[string]interface{}{
		"target_name": selectorName,
	})
	for _, region := range a11yConversationVisualSearchRegions {
		lines, err := locateA11yConversationTextLines(ctx, imagePath, region)
		if err != nil {
			lastErr = err
			continue
		}
		hit, err := resolveA11yConversationVisualHitFromLines(lines, selectorName, region)
		if err == nil {
			return hit, nil
		}
		lastErr = err
	}
	return a11yConversationVisualHit{}, lastErr
}

func locateA11yConversationVisualHitFromPNGBytes(ctx context.Context, imagePNG []byte, selectorName string) (a11yConversationVisualHit, error) {
	var lastErr error = a11yruntime.NewError("target_not_found", "conversation visual locator did not find a unique high-confidence match", map[string]interface{}{
		"target_name": selectorName,
	})
	for _, region := range a11yConversationVisualSearchRegions {
		lines, err := locateA11yConversationTextLinesFromPNG(ctx, imagePNG, region)
		if err != nil {
			lastErr = err
			continue
		}
		hit, err := resolveA11yConversationVisualHitFromLines(lines, selectorName, region)
		if err == nil {
			return hit, nil
		}
		lastErr = err
	}
	return a11yConversationVisualHit{}, lastErr
}

func locateA11yConversationTextLines(ctx context.Context, imagePath string, region a11yConversationImageRegion) ([]a11yruntime.TextLine, error) {
	targetPath := strings.TrimSpace(imagePath)
	cleanup := func() {}
	if !a11yConversationImageRegionIsFull(region) {
		croppedPath, closeFunc, err := cropA11yConversationSearchImage(targetPath, region)
		if err != nil {
			return nil, err
		}
		targetPath = croppedPath
		cleanup = closeFunc
	}
	defer cleanup()
	return a11yruntime.LocateTextInImage(ctx, targetPath)
}

func a11yConversationImageRegionIsFull(region a11yConversationImageRegion) bool {
	return region.X == 0 && region.Y == 0 && region.Width == 1 && region.Height == 1
}

func locateA11yConversationTextLinesFromPNG(ctx context.Context, imagePNG []byte, region a11yConversationImageRegion) ([]a11yruntime.TextLine, error) {
	targetPNG := imagePNG
	if !a11yConversationImageRegionIsFull(region) {
		croppedPNG, err := cropA11yConversationSearchImageBytes(targetPNG, region)
		if err != nil {
			return nil, err
		}
		targetPNG = croppedPNG
	}
	return a11yruntime.LocateTextInPNG(ctx, targetPNG)
}

func cropA11yConversationSearchImage(imagePath string, region a11yConversationImageRegion) (string, func(), error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return "", nil, fmt.Errorf("open screenshot for conversation crop: %w", err)
	}
	defer file.Close()

	source, err := png.Decode(file)
	if err != nil {
		return "", nil, fmt.Errorf("decode screenshot for conversation crop: %w", err)
	}
	croppedPNG, err := cropA11yConversationDecodedImageToPNG(source, region)
	if err != nil {
		return "", nil, err
	}
	tmpFile, err := os.CreateTemp("", "zimaos-blue-computer-use-conversation-*.png")
	if err != nil {
		return "", nil, fmt.Errorf("create conversation crop temp file: %w", err)
	}
	defer tmpFile.Close()
	if _, err := tmpFile.Write(croppedPNG); err != nil {
		os.Remove(tmpFile.Name())
		return "", nil, fmt.Errorf("write conversation crop image: %w", err)
	}
	return tmpFile.Name(), func() { _ = os.Remove(tmpFile.Name()) }, nil
}

func cropA11yConversationSearchImageBytes(imagePNG []byte, region a11yConversationImageRegion) ([]byte, error) {
	source, err := png.Decode(bytes.NewReader(imagePNG))
	if err != nil {
		return nil, fmt.Errorf("decode screenshot for conversation crop: %w", err)
	}
	return cropA11yConversationDecodedImageToPNG(source, region)
}

func cropA11yConversationDecodedImageToPNG(source image.Image, region a11yConversationImageRegion) ([]byte, error) {
	bounds := source.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("conversation crop source image has invalid bounds")
	}
	minX := bounds.Min.X + int(region.X*float64(width))
	minY := bounds.Min.Y + int(region.Y*float64(height))
	maxX := bounds.Min.X + int((region.X+region.Width)*float64(width))
	maxY := bounds.Min.Y + int((region.Y+region.Height)*float64(height))
	if maxX <= minX {
		maxX = minX + 1
	}
	if maxY <= minY {
		maxY = minY + 1
	}
	if maxX > bounds.Max.X {
		maxX = bounds.Max.X
	}
	if maxY > bounds.Max.Y {
		maxY = bounds.Max.Y
	}
	cropBounds := image.Rect(0, 0, maxX-minX, maxY-minY)
	cropped := image.NewRGBA(cropBounds)
	draw.Draw(cropped, cropBounds, source, image.Point{X: minX, Y: minY}, draw.Src)
	var out bytes.Buffer
	if err := png.Encode(&out, cropped); err != nil {
		return nil, fmt.Errorf("encode conversation crop image: %w", err)
	}
	return out.Bytes(), nil
}

func resolveA11yConversationVisualHitFromLines(lines []a11yruntime.TextLine, selectorName string, region a11yConversationImageRegion) (a11yConversationVisualHit, error) {
	query := normalizeA11yTargetName(selectorName)
	if query == "" {
		return a11yConversationVisualHit{}, a11yruntime.NewError("target_not_found", "conversation visual locator did not find a unique high-confidence match", map[string]interface{}{
			"target_name": selectorName,
		})
	}
	matches := make([]a11yConversationVisualHit, 0, 2)
	for _, line := range lines {
		if line.Confidence < a11yConversationVisualConfidenceThreshold {
			continue
		}
		if !strings.Contains(normalizeA11yTargetName(line.Text), query) {
			continue
		}
		if line.Bounds.Width <= 0 || line.Bounds.Height <= 0 {
			continue
		}
		matches = append(matches, a11yConversationVisualHit{
			Point: a11yruntime.NormalizedPoint{
				X: region.X + (line.Bounds.X+line.Bounds.Width/2)*region.Width,
				Y: region.Y + (line.Bounds.Y+line.Bounds.Height/2)*region.Height,
			},
			Confidence: line.Confidence,
		})
	}
	if len(matches) != 1 {
		return a11yConversationVisualHit{}, a11yruntime.NewError("target_not_found", "conversation visual locator did not find a unique high-confidence match", map[string]interface{}{
			"target_name": selectorName,
			"matches":     len(matches),
		})
	}
	return matches[0], nil
}
