package browser

import (
	"encoding/base64"
	"fmt"
	"strings"
)

const defaultScreenshotQuality = 88

func normalizeScreenshotCaptureOptions(format ScreenshotFormat, quality int) (ScreenshotFormat, int) {
	if strings.TrimSpace(string(format)) == "" {
		format = FormatWebP
	}
	switch format {
	case FormatPNG, FormatJPEG, FormatWebP:
	default:
		format = FormatPNG
	}
	if quality <= 0 {
		quality = defaultScreenshotQuality
	}
	return format, quality
}

func screenshotDataMIMEType(format ScreenshotFormat) string {
	switch format {
	case FormatJPEG:
		return "image/jpeg"
	case FormatWebP:
		return "image/webp"
	default:
		return "image/png"
	}
}

func encodeInlineScreenshot(data []byte, format ScreenshotFormat) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	if format == FormatPNG {
		return encoded
	}
	return fmt.Sprintf("data:%s;base64,%s", screenshotDataMIMEType(format), encoded)
}
