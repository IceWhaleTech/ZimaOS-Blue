//go:build cgo

package onnx

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

var (
	initMu      sync.Mutex
	initDone    bool
	initErr     error
	hintDataDir string // set before InitializeRuntime to help findLibrary
)

// SetLibraryPath sets the ONNX Runtime shared library path.
// Must be called before any session creation.
func SetLibraryPath(path string) {
	ort.SetSharedLibraryPath(path)
}

// SetDataDir provides a hint for where to find the ONNX Runtime library.
// Must be called before the first session creation.
func SetDataDir(dataDir string) {
	hintDataDir = dataDir
}

// libName returns the platform-specific ONNX Runtime library filename.
func libName() string {
	switch runtime.GOOS {
	case "darwin":
		return "libonnxruntime.dylib"
	case "windows":
		return "onnxruntime.dll"
	default:
		return "libonnxruntime.so"
	}
}

// findLibrary searches common locations for the ONNX Runtime shared library.
func findLibrary() string {
	name := libName()

	// 0. Hint data directory (set by caller who knows the actual data path)
	if hintDataDir != "" {
		p := filepath.Join(hintDataDir, "onnxruntime", name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// 1. Relative to executable (bundled with app)
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, candidate := range []string{
			filepath.Join(dir, name),
			filepath.Join(dir, "lib", name),
			filepath.Join(dir, "..", "lib", name),
			filepath.Join(dir, "..", "Resources", "lib", name), // macOS .app bundle
		} {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}

	// 2. Data directory (downloaded alongside models)
	if home, err := os.UserHomeDir(); err == nil {
		for _, base := range []string{
			filepath.Join(home, ".zimaos-blue", "data", "onnxruntime"),
			filepath.Join(home, ".zimaos-blue-dev", "data", "onnxruntime"),
		} {
			p := filepath.Join(base, name)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}

	// 3. System paths
	systemPaths := []string{
		"/usr/local/lib/" + name,
		"/usr/lib/" + name,
		"/opt/homebrew/lib/" + name,
	}
	for _, p := range systemPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return ""
}

// InitializeRuntime initializes the ONNX Runtime environment.
// Unlike sync.Once, this retries on failure so that downloading the
// shared library after a failed attempt can succeed.
func InitializeRuntime() error {
	initMu.Lock()
	defer initMu.Unlock()

	if initDone {
		return initErr
	}

	// Auto-detect library path if not explicitly set
	if p := findLibrary(); p != "" {
		ort.SetSharedLibraryPath(p)
	}
	initErr = ort.InitializeEnvironment()
	if initErr == nil {
		initDone = true
	}
	return initErr
}

// Session wraps ort.AdvancedSession with common lifecycle management.
// Use this for models with fixed input/output shapes (like pruner).
type Session struct {
	*ort.AdvancedSession
}

// NewSession creates a new ONNX session with pre-allocated tensors.
// Use this for models with fixed input/output shapes (like pruner).
func NewSession(modelPath string, inputNames, outputNames []string, inputs, outputs []ort.ArbitraryTensor) (*Session, error) {
	if err := InitializeRuntime(); err != nil {
		return nil, fmt.Errorf("initialize ONNX runtime: %w", err)
	}

	session, err := ort.NewAdvancedSession(modelPath, inputNames, outputNames, inputs, outputs, nil)
	if err != nil {
		return nil, fmt.Errorf("create ONNX session: %w", err)
	}

	return &Session{AdvancedSession: session}, nil
}

// Close destroys the session and releases resources.
func (s *Session) Close() error {
	if s.AdvancedSession != nil {
		s.Destroy()
	}
	return nil
}

// DynamicSession wraps ort.DynamicAdvancedSession with common lifecycle management.
// Use this for models with variable input shapes (like TTS).
type DynamicSession struct {
	*ort.DynamicAdvancedSession
}

// NewDynamicSession creates a new ONNX session for dynamic tensor inputs.
// Use this for models with variable input shapes (like TTS).
func NewDynamicSession(modelPath string, inputNames, outputNames []string) (*DynamicSession, error) {
	if err := InitializeRuntime(); err != nil {
		return nil, fmt.Errorf("initialize ONNX runtime: %w", err)
	}

	session, err := ort.NewDynamicAdvancedSession(modelPath, inputNames, outputNames, nil)
	if err != nil {
		return nil, fmt.Errorf("create ONNX session: %w", err)
	}

	return &DynamicSession{DynamicAdvancedSession: session}, nil
}

// Close destroys the session and releases resources.
func (s *DynamicSession) Close() error {
	if s.DynamicAdvancedSession != nil {
		s.Destroy()
	}
	return nil
}
