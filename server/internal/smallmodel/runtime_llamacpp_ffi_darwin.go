package smallmodel

import "fmt"

// macOS builds run as code-signed binaries in common packaging flows.
// The libffi loader used by the FFI backend can fail strict library validation
// before runtime mode selection. Keep FFI mode unavailable on darwin builds.
type llamaCppFFIBackend struct{}

func newLlamaCppFFIBackend() *llamaCppFFIBackend {
	return &llamaCppFFIBackend{}
}

func (b *llamaCppFFIBackend) EnsureLoaded() error {
	if b == nil {
		return fmt.Errorf("llama ffi backend is nil")
	}
	return fmt.Errorf("llama ffi backend unavailable on darwin builds")
}
