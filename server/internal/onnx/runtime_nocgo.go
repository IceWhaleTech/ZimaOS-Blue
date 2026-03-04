//go:build !cgo

package onnx

import (
	"fmt"
	"runtime"
)

var errRuntimeNoCGO = fmt.Errorf("onnx runtime requires cgo (build with CGO_ENABLED=1)")

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

// SetLibraryPath is a no-op in !cgo builds.
func SetLibraryPath(string) {}

// SetDataDir is a no-op in !cgo builds.
func SetDataDir(string) {}

// InitializeRuntime reports ONNX runtime unavailability in !cgo builds.
func InitializeRuntime() error {
	return errRuntimeNoCGO
}

// Session is a no-cgo compatibility stub.
type Session struct{}

// NewSession reports ONNX runtime unavailability in !cgo builds.
func NewSession(string, []string, []string, []any, []any) (*Session, error) {
	return nil, errRuntimeNoCGO
}

// Run reports ONNX runtime unavailability in !cgo builds.
func (s *Session) Run() error {
	_ = s
	return errRuntimeNoCGO
}

// Close is a no-op in !cgo builds.
func (s *Session) Close() error {
	_ = s
	return nil
}

// DynamicSession is a no-cgo compatibility stub.
type DynamicSession struct{}

// NewDynamicSession reports ONNX runtime unavailability in !cgo builds.
func NewDynamicSession(string, []string, []string) (*DynamicSession, error) {
	return nil, errRuntimeNoCGO
}

// Close is a no-op in !cgo builds.
func (s *DynamicSession) Close() error {
	_ = s
	return nil
}
