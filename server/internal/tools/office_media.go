package tools

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type officeEmbeddedImage struct {
	Index       int
	RelID       string
	Source      string
	EntryName   string
	Target      string
	ContentType string
	Data        []byte
	WidthPx     int
	HeightPx    int
}

func officeLoadEmbeddedImage(source string) (officeEmbeddedImage, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return officeEmbeddedImage{}, fmt.Errorf("image source is empty")
	}

	data, hintedType, err := officeReadImageSource(source)
	if err != nil {
		return officeEmbeddedImage{}, err
	}
	width, height, err := officeImagePixelSize(data)
	if err != nil {
		return officeEmbeddedImage{}, err
	}
	ext, contentType, err := officeImageExtAndContentType(source, hintedType, data)
	if err != nil {
		return officeEmbeddedImage{}, err
	}
	return officeEmbeddedImage{
		Source:      source,
		ContentType: contentType,
		Data:        data,
		WidthPx:     width,
		HeightPx:    height,
		EntryName:   "image." + ext,
	}, nil
}

func officeReadImageSource(source string) ([]byte, string, error) {
	if strings.HasPrefix(strings.ToLower(source), "data:") {
		return officeReadDataURLImage(source)
	}

	path := filepath.Clean(source)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read image %s: %w", source, err)
	}
	return data, "", nil
}

func officeReadDataURLImage(source string) ([]byte, string, error) {
	comma := strings.IndexByte(source, ',')
	if comma <= 5 {
		return nil, "", fmt.Errorf("invalid data url image source")
	}
	header := source[5:comma]
	payload := source[comma+1:]
	if !strings.Contains(strings.ToLower(header), ";base64") {
		return nil, "", fmt.Errorf("data url image must be base64 encoded")
	}
	contentType := header
	if semi := strings.IndexByte(contentType, ';'); semi >= 0 {
		contentType = contentType[:semi]
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(payload))
	if err != nil {
		return nil, "", fmt.Errorf("decode data url image: %w", err)
	}
	return data, strings.TrimSpace(contentType), nil
}

func officeImagePixelSize(data []byte) (int, int, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, fmt.Errorf("decode image config: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, fmt.Errorf("image has invalid size %dx%d", cfg.Width, cfg.Height)
	}
	return cfg.Width, cfg.Height, nil
}

func officeImageExtAndContentType(source, hintedType string, data []byte) (string, string, error) {
	contentType := strings.ToLower(strings.TrimSpace(hintedType))
	if contentType == "" {
		contentType = strings.ToLower(strings.TrimSpace(http.DetectContentType(data)))
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(source)), ".")

	switch {
	case contentType == "image/png" || ext == "png":
		return "png", "image/png", nil
	case contentType == "image/jpeg" || contentType == "image/jpg" || ext == "jpg" || ext == "jpeg":
		return "jpg", "image/jpeg", nil
	case contentType == "image/gif" || ext == "gif":
		return "gif", "image/gif", nil
	default:
		return "", "", fmt.Errorf("unsupported image format for %s", source)
	}
}

func officeImageAltText(block officeDocBlock) string {
	if text := strings.TrimSpace(block.Text); text != "" {
		return text
	}
	if source := strings.TrimSpace(block.Source); source != "" {
		return filepath.Base(source)
	}
	return "Image"
}

func officeImageFitEMU(widthPx, heightPx, maxWidthEMU, maxHeightEMU int) (int, int) {
	if widthPx <= 0 || heightPx <= 0 {
		return maxWidthEMU, maxHeightEMU
	}
	cx := widthPx * 9525
	cy := heightPx * 9525
	if maxWidthEMU > 0 && cx > maxWidthEMU {
		cy = cy * maxWidthEMU / cx
		cx = maxWidthEMU
	}
	if maxHeightEMU > 0 && cy > maxHeightEMU {
		cx = cx * maxHeightEMU / cy
		cy = maxHeightEMU
	}
	if cx <= 0 {
		cx = 9525
	}
	if cy <= 0 {
		cy = 9525
	}
	return cx, cy
}

func officeImageBySource(images []officeEmbeddedImage, source string) *officeEmbeddedImage {
	source = strings.TrimSpace(source)
	for idx := range images {
		if images[idx].Source == source {
			return &images[idx]
		}
	}
	return nil
}

func officeImageContentTypeDefaults(images []officeEmbeddedImage) string {
	if len(images) == 0 {
		return ""
	}
	seen := map[string]struct{}{}
	var sb strings.Builder
	for _, image := range images {
		switch image.ContentType {
		case "image/png":
			if _, ok := seen["png"]; ok {
				continue
			}
			seen["png"] = struct{}{}
			sb.WriteString(`<Default Extension="png" ContentType="image/png"/>`)
		case "image/jpeg":
			if _, ok := seen["jpg"]; ok {
				continue
			}
			seen["jpg"] = struct{}{}
			sb.WriteString(`<Default Extension="jpg" ContentType="image/jpeg"/>`)
		case "image/gif":
			if _, ok := seen["gif"]; ok {
				continue
			}
			seen["gif"] = struct{}{}
			sb.WriteString(`<Default Extension="gif" ContentType="image/gif"/>`)
		}
	}
	return sb.String()
}
