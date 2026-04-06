//go:build darwin

package ocr

import (
	"context"
	"reflect"
	"testing"
)

func TestNewTesseractServiceUsesMacOSNativeOCROnDarwin(t *testing.T) {
	svc := NewTesseractService(nil, Config{})
	if _, ok := any(svc).(*MacOSNativeService); !ok {
		t.Fatalf("service = %T, want *MacOSNativeService", svc)
	}
}

func TestMacOSNativeServiceExtractUsesNativeExtractor(t *testing.T) {
	svc := NewMacOSNativeService(nil, Config{PreferredModels: []string{"chi_sim", "eng"}})
	called := false
	svc.extract = func(ctx context.Context, imagePNG []byte, languages []string) (string, error) {
		called = true
		if got := string(imagePNG); got != "png-bytes" {
			t.Fatalf("image bytes = %q, want %q", got, "png-bytes")
		}
		wantLanguages := []string{"zh-Hans", "en-US"}
		if !reflect.DeepEqual(languages, wantLanguages) {
			t.Fatalf("languages = %#v, want %#v", languages, wantLanguages)
		}
		return " hello\x00 world ", nil
	}

	result, err := svc.Extract(context.Background(), []byte("png-bytes"))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if !called {
		t.Fatal("expected native extractor to run")
	}
	if result.Text != "hello world" {
		t.Fatalf("text = %q, want %q", result.Text, "hello world")
	}
	if result.Engine != macOSNativeEngineName {
		t.Fatalf("engine = %q, want %q", result.Engine, macOSNativeEngineName)
	}
	if result.Model != "zh-Hans,en-US" {
		t.Fatalf("model = %q, want %q", result.Model, "zh-Hans,en-US")
	}
}

func TestMacOSNativeServiceExtractRequiresImageBytes(t *testing.T) {
	svc := NewMacOSNativeService(nil, Config{})
	if _, err := svc.Extract(context.Background(), nil); err == nil {
		t.Fatal("expected empty image bytes to fail")
	}
}
