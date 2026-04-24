//go:build darwin

package pdf

import (
	"errors"
	"testing"
)

func TestCreateDarwinRendererUsesNativeRendererWhenFallbackIsNil(t *testing.T) {
	previous := darwinCreateDocumentNativeRenderFunc
	defer func() {
		darwinCreateDocumentNativeRenderFunc = previous
	}()

	var called bool
	darwinCreateDocumentNativeRenderFunc = func(lines []createStyledLine, _ createFontPlan) ([]byte, int, int, error) {
		called = true
		if len(lines) != 1 || lines[0].Text != "native" {
			t.Fatalf("native renderer got lines = %#v", lines)
		}
		return []byte("%PDF-native-darwin"), 4, 6, nil
	}

	data, pageCount, lineCount, err := createDarwinRenderer{}.render([]createStyledLine{{Text: "native"}}, createFontPlan{})
	if err != nil {
		t.Fatalf("createDarwinRenderer.render() error = %v", err)
	}
	if !called {
		t.Fatal("expected createDarwinRenderer to call native renderer")
	}
	if got := string(data); got != "%PDF-native-darwin" {
		t.Fatalf("data = %q, want %q", got, "%PDF-native-darwin")
	}
	if pageCount != 4 || lineCount != 6 {
		t.Fatalf("pageCount/lineCount = %d/%d, want 4/6", pageCount, lineCount)
	}
}

func TestCreateDarwinRendererReturnsUnavailableWhenNativeRendererUnavailableAndNoFallback(t *testing.T) {
	previous := darwinCreateDocumentNativeRenderFunc
	defer func() {
		darwinCreateDocumentNativeRenderFunc = previous
	}()

	darwinCreateDocumentNativeRenderFunc = func(_ []createStyledLine, _ createFontPlan) ([]byte, int, int, error) {
		return nil, 0, 0, errCreateRendererUnavailable
	}

	_, _, _, err := createDarwinRenderer{}.render([]createStyledLine{{Text: "native"}}, createFontPlan{})
	if !errors.Is(err, errCreateRendererUnavailable) {
		t.Fatalf("createDarwinRenderer.render() error = %v, want errCreateRendererUnavailable", err)
	}
}
