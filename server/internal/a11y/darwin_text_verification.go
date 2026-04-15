//go:build darwin

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

func darwinVerifySemanticTextEntry(
	ctx context.Context,
	value string,
	readback func() string,
	bounds darwinRect,
	hasBounds bool,
	captureRegion func(context.Context, darwinRect) ([]byte, error),
	extractText func(context.Context, []byte) (string, error),
) (bool, string) {
	if darwinTextEntryReadbackMatches(value, valueFromFunc(readback)) {
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
	if darwinTextEntryOCRMatches(value, observed) {
		return true, "ocr"
	}
	return false, ""
}

func valueFromFunc(fn func() string) string {
	if fn == nil {
		return ""
	}
	return fn()
}

func darwinTextEntryReadbackMatches(expected string, observed string) bool {
	return darwinNormalizeTextEntryReadback(expected) == darwinNormalizeTextEntryReadback(observed)
}

func darwinTextEntryOCRMatches(expected string, observed string) bool {
	normalizedExpected := darwinNormalizeTextEntryLoose(expected)
	normalizedObserved := darwinNormalizeTextEntryLoose(observed)
	if normalizedExpected == "" || normalizedObserved == "" {
		return false
	}
	return strings.Contains(normalizedObserved, normalizedExpected)
}

func darwinNormalizeTextEntryReadback(value string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(value)))
	return strings.Join(fields, " ")
}

func darwinNormalizeTextEntryLoose(value string) string {
	var out []rune
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			out = append(out, r)
		}
	}
	return string(out)
}

func darwinCaptureRegionPNG(ctx context.Context, bounds darwinRect) ([]byte, error) {
	path := filepath.Join(os.TempDir(), fmt.Sprintf("zimaos-blue-a11y-region-%d.png", time.Now().UnixNano()))
	defer os.Remove(path)
	output, err := darwinCLIFallback.captureRegion(ctx, bounds, path)
	if err != nil {
		return nil, fmt.Errorf("screencapture region failed: %s: %w", strings.TrimSpace(output), err)
	}
	imagePNG, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read region screenshot: %w", err)
	}
	return imagePNG, nil
}

func darwinExtractTextFromPNG(ctx context.Context, imagePNG []byte) (string, error) {
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
