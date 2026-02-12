package plugin

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
)

// IsolationConfig configures plugin isolation behavior
type IsolationConfig struct {
	// InitTimeout is the maximum time allowed for plugin initialization
	InitTimeout time.Duration
	// StartTimeout is the maximum time allowed for plugin start
	StartTimeout time.Duration
	// StopTimeout is the maximum time allowed for plugin stop
	StopTimeout time.Duration
	// ToolTimeout is the maximum time allowed for tool execution
	ToolTimeout time.Duration
	// HookTimeout is the maximum time allowed for hook execution
	HookTimeout time.Duration
	// CommandTimeout is the maximum time allowed for command execution
	CommandTimeout time.Duration
	// HTTPTimeout is the maximum time allowed for HTTP handler execution
	HTTPTimeout time.Duration
}

// DefaultIsolationConfig returns the default isolation configuration
func DefaultIsolationConfig() *IsolationConfig {
	return &IsolationConfig{
		InitTimeout:    30 * time.Second,
		StartTimeout:   30 * time.Second,
		StopTimeout:    10 * time.Second,
		ToolTimeout:    60 * time.Second,
		HookTimeout:    30 * time.Second,
		CommandTimeout: 60 * time.Second,
		HTTPTimeout:    30 * time.Second,
	}
}

// PluginError represents an error that occurred in a plugin
type PluginError struct {
	PluginID  string
	Operation string
	Err       error
	Panic     bool
	Stack     string
}

func (e *PluginError) Error() string {
	if e.Panic {
		return fmt.Sprintf("plugin %s panicked during %s: %v", e.PluginID, e.Operation, e.Err)
	}
	return fmt.Sprintf("plugin %s error during %s: %v", e.PluginID, e.Operation, e.Err)
}

func (e *PluginError) Unwrap() error {
	return e.Err
}

// safeExecute runs a function with panic recovery and timeout
func safeExecute(ctx context.Context, timeout time.Duration, pluginID, operation string, fn func(ctx context.Context) error) (err error) {
	// Create timeout context if timeout is specified
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Channel to receive result
	done := make(chan error, 1)

	go func() {
		// Recover from panic
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				logger.Error().
					Str("plugin_id", pluginID).
					Str("operation", operation).
					Interface("panic", r).
					Str("stack", stack).
					Msg("Plugin panicked")

				done <- &PluginError{
					PluginID:  pluginID,
					Operation: operation,
					Err:       fmt.Errorf("%v", r),
					Panic:     true,
					Stack:     stack,
				}
			}
		}()

		done <- fn(ctx)
	}()

	select {
	case err = <-done:
		return err
	case <-ctx.Done():
		return &PluginError{
			PluginID:  pluginID,
			Operation: operation,
			Err:       ctx.Err(),
			Panic:     false,
		}
	}
}

// safeExecuteWithResult runs a function with panic recovery and timeout, returning a result
func safeExecuteWithResult[T any](ctx context.Context, timeout time.Duration, pluginID, operation string, fn func(ctx context.Context) (T, error)) (result T, err error) {
	// Create timeout context if timeout is specified
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	type resultType struct {
		value T
		err   error
	}

	// Channel to receive result
	done := make(chan resultType, 1)

	go func() {
		// Recover from panic
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				logger.Error().
					Str("plugin_id", pluginID).
					Str("operation", operation).
					Interface("panic", r).
					Str("stack", stack).
					Msg("Plugin panicked")

				var zero T
				done <- resultType{
					value: zero,
					err: &PluginError{
						PluginID:  pluginID,
						Operation: operation,
						Err:       fmt.Errorf("%v", r),
						Panic:     true,
						Stack:     stack,
					},
				}
			}
		}()

		v, e := fn(ctx)
		done <- resultType{value: v, err: e}
	}()

	select {
	case res := <-done:
		return res.value, res.err
	case <-ctx.Done():
		var zero T
		return zero, &PluginError{
			PluginID:  pluginID,
			Operation: operation,
			Err:       ctx.Err(),
			Panic:     false,
		}
	}
}

// WrapToolHandler wraps a tool handler with isolation
func WrapToolHandler(pluginID string, handler ToolHandler, timeout time.Duration) ToolHandler {
	return func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
		return safeExecuteWithResult(ctx, timeout, pluginID, "tool", func(ctx context.Context) (interface{}, error) {
			return handler(ctx, params)
		})
	}
}

// WrapHookHandler wraps a hook handler with isolation
func WrapHookHandler(pluginID string, handler HookHandler, timeout time.Duration) HookHandler {
	return func(ctx context.Context, data interface{}) error {
		return safeExecute(ctx, timeout, pluginID, "hook", func(ctx context.Context) error {
			return handler(ctx, data)
		})
	}
}

// WrapCommandHandler wraps a command handler with isolation
func WrapCommandHandler(pluginID string, handler CommandHandler, timeout time.Duration) CommandHandler {
	return func(ctx context.Context, args []string) (string, error) {
		return safeExecuteWithResult(ctx, timeout, pluginID, "command", func(ctx context.Context) (string, error) {
			return handler(ctx, args)
		})
	}
}

// WrapHTTPHandler wraps an HTTP handler with isolation
func WrapHTTPHandler(pluginID string, handler HTTPHandler, timeout time.Duration) HTTPHandler {
	return func(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error) {
		return safeExecuteWithResult(ctx, timeout, pluginID, "http", func(ctx context.Context) (*HTTPResponse, error) {
			return handler(ctx, req)
		})
	}
}

// WrapServiceStart wraps a service start with isolation
func WrapServiceStart(pluginID, serviceName string, startFn func(ctx context.Context) error, timeout time.Duration) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return safeExecute(ctx, timeout, pluginID, fmt.Sprintf("service.%s.start", serviceName), startFn)
	}
}

// WrapServiceStop wraps a service stop with isolation
func WrapServiceStop(pluginID, serviceName string, stopFn func(ctx context.Context) error, timeout time.Duration) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return safeExecute(ctx, timeout, pluginID, fmt.Sprintf("service.%s.stop", serviceName), stopFn)
	}
}

// IsolatedService wraps a service with isolation
type IsolatedService struct {
	service  Service
	pluginID string
	config   *IsolationConfig
}

// NewIsolatedService creates a new isolated service wrapper
func NewIsolatedService(service Service, pluginID string, config *IsolationConfig) *IsolatedService {
	if config == nil {
		config = DefaultIsolationConfig()
	}
	return &IsolatedService{
		service:  service,
		pluginID: pluginID,
		config:   config,
	}
}

func (s *IsolatedService) Name() string {
	return s.service.Name()
}

func (s *IsolatedService) Start(ctx context.Context) error {
	return safeExecute(ctx, s.config.StartTimeout, s.pluginID, fmt.Sprintf("service.%s.start", s.service.Name()), s.service.Start)
}

func (s *IsolatedService) Stop(ctx context.Context) error {
	return safeExecute(ctx, s.config.StopTimeout, s.pluginID, fmt.Sprintf("service.%s.stop", s.service.Name()), s.service.Stop)
}

// IsolatedPlugin wraps a plugin with isolation
type IsolatedPlugin struct {
	plugin NativePlugin
	config *IsolationConfig
}

// NewIsolatedPlugin creates a new isolated plugin wrapper
func NewIsolatedPlugin(plugin NativePlugin, config *IsolationConfig) *IsolatedPlugin {
	if config == nil {
		config = DefaultIsolationConfig()
	}
	return &IsolatedPlugin{
		plugin: plugin,
		config: config,
	}
}

func (p *IsolatedPlugin) ID() string {
	return p.plugin.ID()
}

func (p *IsolatedPlugin) Manifest() *Manifest {
	return p.plugin.Manifest()
}

func (p *IsolatedPlugin) IsNative() bool {
	return p.plugin.IsNative()
}

func (p *IsolatedPlugin) Init(ctx context.Context, api PluginAPI) error {
	return safeExecute(ctx, p.config.InitTimeout, p.plugin.ID(), "init", func(ctx context.Context) error {
		return p.plugin.Init(ctx, api)
	})
}

func (p *IsolatedPlugin) Start(ctx context.Context) error {
	return safeExecute(ctx, p.config.StartTimeout, p.plugin.ID(), "start", func(ctx context.Context) error {
		return p.plugin.Start(ctx)
	})
}

func (p *IsolatedPlugin) Stop(ctx context.Context) error {
	return safeExecute(ctx, p.config.StopTimeout, p.plugin.ID(), "stop", func(ctx context.Context) error {
		return p.plugin.Stop(ctx)
	})
}
