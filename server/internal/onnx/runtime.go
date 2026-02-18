package onnx

import (
	"fmt"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

var (
	initOnce sync.Once
	initErr  error
)

// InitializeRuntime initializes the ONNX Runtime environment once.
// It's safe to call multiple times - initialization only happens once.
func InitializeRuntime() error {
	initOnce.Do(func() {
		initErr = ort.InitializeEnvironment()
	})
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
