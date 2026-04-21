package a11y

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ocr"
)

func windowsVerifySemanticTextEntry(
	ctx context.Context,
	value string,
	readback func() string,
	bounds windowsRect,
	hasBounds bool,
	captureRegion func(context.Context, windowsRect) ([]byte, error),
	extractText func(context.Context, []byte) (string, error),
) (bool, string) {
	if windowsTextEntryReadbackMatches(value, windowsValueFromFunc(readback)) {
		return true, "ax_value"
	}
	if !hasBounds || captureRegion == nil || extractText == nil {
		return false, ""
	}
	imagePNG, err := captureRegion(ctx, bounds)
	if err != nil || len(imagePNG) == 0 {
		return false, ""
	}
	observed, err := extractText(ctx, imagePNG)
	if err != nil {
		return false, ""
	}
	if windowsTextEntryOCRMatches(value, observed) {
		return true, "ocr"
	}
	return false, ""
}

func windowsValueFromFunc(fn func() string) string {
	if fn == nil {
		return ""
	}
	return fn()
}

func windowsTextEntryReadbackMatches(expected string, observed string) bool {
	return windowsNormalizeTextEntryReadback(expected) == windowsNormalizeTextEntryReadback(observed)
}

func windowsTextEntryOCRMatches(expected string, observed string) bool {
	normalizedExpected := windowsNormalizeTextEntryLoose(expected)
	normalizedObserved := windowsNormalizeTextEntryLoose(observed)
	if normalizedExpected == "" || normalizedObserved == "" {
		return false
	}
	return strings.Contains(normalizedObserved, normalizedExpected)
}

func windowsNormalizeTextEntryReadback(value string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(value)))
	return strings.Join(fields, " ")
}

func windowsNormalizeTextEntryLoose(value string) string {
	var out []rune
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			out = append(out, r)
		}
	}
	return string(out)
}

func windowsCaptureRegionPNG(ctx context.Context, bounds windowsRect) ([]byte, error) {
	path := filepath.Join(os.TempDir(), fmt.Sprintf("zimaos-blue-computer-use-region-%d.png", time.Now().UnixNano()))
	defer os.Remove(path)
	output, err := windowsCLIFallback.captureRegion(ctx, bounds, path)
	if err != nil {
		return nil, fmt.Errorf("powershell region screenshot failed: %s: %w", strings.TrimSpace(output), err)
	}
	imagePNG, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read region screenshot: %w", err)
	}
	return imagePNG, nil
}

func windowsExtractTextFromPNG(ctx context.Context, imagePNG []byte) (string, error) {
	svc := ocr.NewTesseractService(nil, ocr.Config{PreferredModels: []string{"eng", "chi_sim"}})
	if svc == nil {
		return "", fmt.Errorf("ocr service not available")
	}
	defer svc.Close()
	result, err := svc.Extract(ctx, imagePNG)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}
