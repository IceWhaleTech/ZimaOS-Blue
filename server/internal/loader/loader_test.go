package loader_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-AI/pkg/loader"
)

func TestLibraryFilename(t *testing.T) {
	dir := "/opt/libs"
	got := loader.LibraryFilename(dir, "ggml")

	switch runtime.GOOS {
	case "linux", "freebsd":
		want := filepath.Join(dir, "libggml.so")
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	case "windows":
		want := filepath.Join(dir, "ggml.dll")
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	case "darwin":
		want := filepath.Join(dir, "libggml.dylib")
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}

func TestLibraryExists(t *testing.T) {
	if loader.LibraryExists("/nonexistent", "ggml") {
		t.Fatal("expected false for nonexistent path")
	}
}

func TestDiscoverLibs(t *testing.T) {
	_, err := loader.DiscoverLibs("Z:\\this_path_does_not_exist_anywhere")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}
