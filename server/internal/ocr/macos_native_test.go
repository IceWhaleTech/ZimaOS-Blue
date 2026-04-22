//go:build darwin

package ocr

import (
	"context"
	"reflect"
	"testing"
)

func TestWithMacOSNativeThreadAffinityLocksAndUnlocksInOrder(t *testing.T) {
	var calls []string
	restore := setMacOSNativeThreadHooksForTest(
		func() { calls = append(calls, "lock") },
		func() { calls = append(calls, "unlock") },
	)
	defer restore()

	got, err := withMacOSNativeThreadAffinity(func() (string, error) {
		calls = append(calls, "body")
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("withMacOSNativeThreadAffinity returned error: %v", err)
	}
	if got != "ok" {
		t.Fatalf("result = %q, want %q", got, "ok")
	}

	wantCalls := []string{"lock", "body", "unlock"}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestWithMacOSNativeThreadAffinityUnlocksOnError(t *testing.T) {
	var calls []string
	restore := setMacOSNativeThreadHooksForTest(
		func() { calls = append(calls, "lock") },
		func() { calls = append(calls, "unlock") },
	)
	defer restore()

	wantErr := context.Canceled
	_, err := withMacOSNativeThreadAffinity(func() (string, error) {
		calls = append(calls, "body")
		return "", wantErr
	})
	if err != wantErr {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}

	wantCalls := []string{"lock", "body", "unlock"}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

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
