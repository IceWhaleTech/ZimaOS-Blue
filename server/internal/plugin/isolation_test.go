package plugin

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSafeExecute_Success(t *testing.T) {
	ctx := context.Background()
	called := false

	err := safeExecute(ctx, time.Second, "test-plugin", "test-op", func(ctx context.Context) error {
		called = true
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !called {
		t.Error("expected function to be called")
	}
}

func TestSafeExecute_Error(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("test error")

	err := safeExecute(ctx, time.Second, "test-plugin", "test-op", func(ctx context.Context) error {
		return expectedErr
	})

	if err != expectedErr {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}

func TestSafeExecute_Panic(t *testing.T) {
	ctx := context.Background()

	err := safeExecute(ctx, time.Second, "test-plugin", "test-op", func(ctx context.Context) error {
		panic("test panic")
	})

	if err == nil {
		t.Error("expected error from panic")
	}

	pluginErr, ok := err.(*PluginError)
	if !ok {
		t.Errorf("expected PluginError, got %T", err)
	}
	if !pluginErr.Panic {
		t.Error("expected Panic to be true")
	}
	if pluginErr.PluginID != "test-plugin" {
		t.Errorf("expected plugin ID 'test-plugin', got %s", pluginErr.PluginID)
	}
	if pluginErr.Stack == "" {
		t.Error("expected stack trace to be captured")
	}
}

func TestSafeExecute_Timeout(t *testing.T) {
	ctx := context.Background()

	err := safeExecute(ctx, 50*time.Millisecond, "test-plugin", "test-op", func(ctx context.Context) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	})

	if err == nil {
		t.Error("expected timeout error")
	}

	pluginErr, ok := err.(*PluginError)
	if !ok {
		t.Errorf("expected PluginError, got %T", err)
	}
	if pluginErr.Panic {
		t.Error("expected Panic to be false for timeout")
	}
	if !errors.Is(pluginErr.Err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", pluginErr.Err)
	}
}

func TestSafeExecute_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := safeExecute(ctx, 0, "test-plugin", "test-op", func(ctx context.Context) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	})

	if err == nil {
		t.Error("expected context canceled error")
	}

	pluginErr, ok := err.(*PluginError)
	if !ok {
		t.Errorf("expected PluginError, got %T", err)
	}
	if !errors.Is(pluginErr.Err, context.Canceled) {
		t.Errorf("expected Canceled, got %v", pluginErr.Err)
	}
}

func TestSafeExecuteWithResult_Success(t *testing.T) {
	ctx := context.Background()

	result, err := safeExecuteWithResult(ctx, time.Second, "test-plugin", "test-op", func(ctx context.Context) (string, error) {
		return "success", nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != "success" {
		t.Errorf("expected 'success', got %s", result)
	}
}

func TestSafeExecuteWithResult_Panic(t *testing.T) {
	ctx := context.Background()

	result, err := safeExecuteWithResult(ctx, time.Second, "test-plugin", "test-op", func(ctx context.Context) (string, error) {
		panic("test panic")
	})

	if err == nil {
		t.Error("expected error from panic")
	}
	if result != "" {
		t.Errorf("expected empty result, got %s", result)
	}

	pluginErr, ok := err.(*PluginError)
	if !ok {
		t.Errorf("expected PluginError, got %T", err)
	}
	if !pluginErr.Panic {
		t.Error("expected Panic to be true")
	}
}

func TestSafeExecuteWithResult_Timeout(t *testing.T) {
	ctx := context.Background()

	result, err := safeExecuteWithResult(ctx, 50*time.Millisecond, "test-plugin", "test-op", func(ctx context.Context) (int, error) {
		time.Sleep(200 * time.Millisecond)
		return 42, nil
	})

	if err == nil {
		t.Error("expected timeout error")
	}
	if result != 0 {
		t.Errorf("expected zero value, got %d", result)
	}
}

func TestWrapToolHandler_Success(t *testing.T) {
	handler := func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
		return params["value"], nil
	}

	wrapped := WrapToolHandler("test-plugin", handler, time.Second)

	result, err := wrapped(context.Background(), map[string]interface{}{"value": "test"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != "test" {
		t.Errorf("expected 'test', got %v", result)
	}
}

func TestWrapToolHandler_Panic(t *testing.T) {
	handler := func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
		panic("tool panic")
	}

	wrapped := WrapToolHandler("test-plugin", handler, time.Second)

	result, err := wrapped(context.Background(), nil)
	if err == nil {
		t.Error("expected error from panic")
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

func TestWrapHookHandler_Success(t *testing.T) {
	called := false
	handler := func(ctx context.Context, data interface{}) error {
		called = true
		return nil
	}

	wrapped := WrapHookHandler("test-plugin", handler, time.Second)

	err := wrapped(context.Background(), nil)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !called {
		t.Error("expected handler to be called")
	}
}

func TestWrapHookHandler_Panic(t *testing.T) {
	handler := func(ctx context.Context, data interface{}) error {
		panic("hook panic")
	}

	wrapped := WrapHookHandler("test-plugin", handler, time.Second)

	err := wrapped(context.Background(), nil)
	if err == nil {
		t.Error("expected error from panic")
	}
}

func TestWrapCommandHandler_Success(t *testing.T) {
	handler := func(ctx context.Context, args []string) (string, error) {
		return "result: " + args[0], nil
	}

	wrapped := WrapCommandHandler("test-plugin", handler, time.Second)

	result, err := wrapped(context.Background(), []string{"test"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != "result: test" {
		t.Errorf("expected 'result: test', got %s", result)
	}
}

func TestWrapCommandHandler_Panic(t *testing.T) {
	handler := func(ctx context.Context, args []string) (string, error) {
		panic("command panic")
	}

	wrapped := WrapCommandHandler("test-plugin", handler, time.Second)

	result, err := wrapped(context.Background(), nil)
	if err == nil {
		t.Error("expected error from panic")
	}
	if result != "" {
		t.Errorf("expected empty result, got %s", result)
	}
}

func TestWrapHTTPHandler_Success(t *testing.T) {
	handler := func(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error) {
		return &HTTPResponse{StatusCode: 200, Body: []byte("OK")}, nil
	}

	wrapped := WrapHTTPHandler("test-plugin", handler, time.Second)

	result, err := wrapped(context.Background(), &HTTPRequest{Method: "GET", Path: "/"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}
}

func TestWrapHTTPHandler_Panic(t *testing.T) {
	handler := func(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error) {
		panic("http panic")
	}

	wrapped := WrapHTTPHandler("test-plugin", handler, time.Second)

	result, err := wrapped(context.Background(), nil)
	if err == nil {
		t.Error("expected error from panic")
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

// Mock service for testing
type mockService struct {
	name      string
	startErr  error
	stopErr   error
	startPanic bool
	stopPanic  bool
}

func (s *mockService) Name() string { return s.name }

func (s *mockService) Start(ctx context.Context) error {
	if s.startPanic {
		panic("service start panic")
	}
	return s.startErr
}

func (s *mockService) Stop(ctx context.Context) error {
	if s.stopPanic {
		panic("service stop panic")
	}
	return s.stopErr
}

func TestIsolatedService_Start_Success(t *testing.T) {
	svc := &mockService{name: "test-service"}
	isolated := NewIsolatedService(svc, "test-plugin", nil)

	err := isolated.Start(context.Background())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestIsolatedService_Start_Panic(t *testing.T) {
	svc := &mockService{name: "test-service", startPanic: true}
	isolated := NewIsolatedService(svc, "test-plugin", nil)

	err := isolated.Start(context.Background())
	if err == nil {
		t.Error("expected error from panic")
	}
}

func TestIsolatedService_Stop_Success(t *testing.T) {
	svc := &mockService{name: "test-service"}
	isolated := NewIsolatedService(svc, "test-plugin", nil)

	err := isolated.Stop(context.Background())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestIsolatedService_Stop_Panic(t *testing.T) {
	svc := &mockService{name: "test-service", stopPanic: true}
	isolated := NewIsolatedService(svc, "test-plugin", nil)

	err := isolated.Stop(context.Background())
	if err == nil {
		t.Error("expected error from panic")
	}
}

// Mock plugin for testing
type mockPlugin struct {
	id         string
	manifest   *Manifest
	initErr    error
	startErr   error
	stopErr    error
	initPanic  bool
	startPanic bool
	stopPanic  bool
}

func (p *mockPlugin) ID() string         { return p.id }
func (p *mockPlugin) Manifest() *Manifest { return p.manifest }
func (p *mockPlugin) IsNative() bool      { return true }

func (p *mockPlugin) Init(ctx context.Context, api PluginAPI) error {
	if p.initPanic {
		panic("plugin init panic")
	}
	return p.initErr
}

func (p *mockPlugin) Start(ctx context.Context) error {
	if p.startPanic {
		panic("plugin start panic")
	}
	return p.startErr
}

func (p *mockPlugin) Stop(ctx context.Context) error {
	if p.stopPanic {
		panic("plugin stop panic")
	}
	return p.stopErr
}

func TestIsolatedPlugin_Init_Success(t *testing.T) {
	plugin := &mockPlugin{
		id:       "test-plugin",
		manifest: &Manifest{ID: "test-plugin", Name: "Test Plugin"},
	}
	isolated := NewIsolatedPlugin(plugin, nil)

	err := isolated.Init(context.Background(), nil)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestIsolatedPlugin_Init_Panic(t *testing.T) {
	plugin := &mockPlugin{
		id:        "test-plugin",
		manifest:  &Manifest{ID: "test-plugin", Name: "Test Plugin"},
		initPanic: true,
	}
	isolated := NewIsolatedPlugin(plugin, nil)

	err := isolated.Init(context.Background(), nil)
	if err == nil {
		t.Error("expected error from panic")
	}
}

func TestIsolatedPlugin_Start_Success(t *testing.T) {
	plugin := &mockPlugin{
		id:       "test-plugin",
		manifest: &Manifest{ID: "test-plugin", Name: "Test Plugin"},
	}
	isolated := NewIsolatedPlugin(plugin, nil)

	err := isolated.Start(context.Background())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestIsolatedPlugin_Start_Panic(t *testing.T) {
	plugin := &mockPlugin{
		id:         "test-plugin",
		manifest:   &Manifest{ID: "test-plugin", Name: "Test Plugin"},
		startPanic: true,
	}
	isolated := NewIsolatedPlugin(plugin, nil)

	err := isolated.Start(context.Background())
	if err == nil {
		t.Error("expected error from panic")
	}
}

func TestIsolatedPlugin_Stop_Success(t *testing.T) {
	plugin := &mockPlugin{
		id:       "test-plugin",
		manifest: &Manifest{ID: "test-plugin", Name: "Test Plugin"},
	}
	isolated := NewIsolatedPlugin(plugin, nil)

	err := isolated.Stop(context.Background())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestIsolatedPlugin_Stop_Panic(t *testing.T) {
	plugin := &mockPlugin{
		id:        "test-plugin",
		manifest:  &Manifest{ID: "test-plugin", Name: "Test Plugin"},
		stopPanic: true,
	}
	isolated := NewIsolatedPlugin(plugin, nil)

	err := isolated.Stop(context.Background())
	if err == nil {
		t.Error("expected error from panic")
	}
}

func TestPluginError_Error(t *testing.T) {
	err := &PluginError{
		PluginID:  "test-plugin",
		Operation: "init",
		Err:       errors.New("test error"),
		Panic:     false,
	}

	expected := "plugin test-plugin error during init: test error"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestPluginError_Error_Panic(t *testing.T) {
	err := &PluginError{
		PluginID:  "test-plugin",
		Operation: "init",
		Err:       errors.New("test panic"),
		Panic:     true,
		Stack:     "stack trace here",
	}

	expected := "plugin test-plugin panicked during init: test panic"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestPluginError_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	err := &PluginError{
		PluginID:  "test-plugin",
		Operation: "init",
		Err:       innerErr,
	}

	if err.Unwrap() != innerErr {
		t.Error("Unwrap should return inner error")
	}
}

func TestDefaultIsolationConfig(t *testing.T) {
	config := DefaultIsolationConfig()

	if config.InitTimeout != 30*time.Second {
		t.Errorf("expected InitTimeout 30s, got %v", config.InitTimeout)
	}
	if config.StartTimeout != 30*time.Second {
		t.Errorf("expected StartTimeout 30s, got %v", config.StartTimeout)
	}
	if config.StopTimeout != 10*time.Second {
		t.Errorf("expected StopTimeout 10s, got %v", config.StopTimeout)
	}
	if config.ToolTimeout != 60*time.Second {
		t.Errorf("expected ToolTimeout 60s, got %v", config.ToolTimeout)
	}
	if config.HookTimeout != 30*time.Second {
		t.Errorf("expected HookTimeout 30s, got %v", config.HookTimeout)
	}
	if config.CommandTimeout != 60*time.Second {
		t.Errorf("expected CommandTimeout 60s, got %v", config.CommandTimeout)
	}
	if config.HTTPTimeout != 30*time.Second {
		t.Errorf("expected HTTPTimeout 30s, got %v", config.HTTPTimeout)
	}
}
