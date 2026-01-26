package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dop251/goja"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/logger"
)

// JSBridge handles loading and executing JavaScript/TypeScript plugins
type JSBridge struct {
	registry *Registry
	vms      map[string]*goja.Runtime
}

// NewJSBridge creates a new JavaScript bridge
func NewJSBridge(registry *Registry) *JSBridge {
	return &JSBridge{
		registry: registry,
		vms:      make(map[string]*goja.Runtime),
	}
}

// LoadJSPlugin loads a JavaScript plugin from a path
func (b *JSBridge) LoadJSPlugin(ctx context.Context, pluginPath string, origin PluginOrigin) error {
	// Read manifest
	manifestPath := filepath.Join(pluginPath, "clawdbot.plugin.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	if manifest.ID == "" {
		return fmt.Errorf("manifest missing required field: id")
	}

	// Find entry point
	entryPoint, err := b.findEntryPoint(pluginPath)
	if err != nil {
		return fmt.Errorf("failed to find entry point: %w", err)
	}

	// Read plugin code
	code, err := os.ReadFile(entryPoint)
	if err != nil {
		return fmt.Errorf("failed to read plugin code: %w", err)
	}

	// Create VM
	vm := goja.New()
	b.vms[manifest.ID] = vm

	// Setup environment
	if err := b.setupEnvironment(vm, manifest.ID, pluginPath); err != nil {
		return fmt.Errorf("failed to setup environment: %w", err)
	}

	// Transpile TypeScript if needed
	jsCode := string(code)
	if strings.HasSuffix(entryPoint, ".ts") || strings.HasSuffix(entryPoint, ".mts") {
		jsCode = b.transpileTypeScript(jsCode)
	}

	// Wrap code in module format
	wrappedCode := b.wrapAsModule(jsCode)

	// Execute plugin
	_, err = vm.RunString(wrappedCode)
	if err != nil {
		return fmt.Errorf("failed to execute plugin: %w", err)
	}

	// Register plugin info
	b.registry.mu.Lock()
	b.registry.plugins[manifest.ID] = &PluginInfo{
		Manifest: &manifest,
		Origin:   origin,
		Status:   StatusLoaded,
		Path:     pluginPath,
		IsNative: false,
	}
	b.registry.mu.Unlock()

	logger.Info().
		Str("plugin_id", manifest.ID).
		Str("entry_point", entryPoint).
		Msg("Loaded JS plugin")

	return nil
}

// findEntryPoint finds the plugin entry point file
func (b *JSBridge) findEntryPoint(pluginPath string) (string, error) {
	// Check package.json for clawdbot.extensions
	pkgPath := filepath.Join(pluginPath, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		var pkg struct {
			Clawdbot struct {
				Extensions []string `json:"extensions"`
			} `json:"clawdbot"`
		}
		if err := json.Unmarshal(data, &pkg); err == nil && len(pkg.Clawdbot.Extensions) > 0 {
			entryPoint := filepath.Join(pluginPath, pkg.Clawdbot.Extensions[0])
			if _, err := os.Stat(entryPoint); err == nil {
				return entryPoint, nil
			}
		}
	}

	// Try common entry points
	candidates := []string{
		"index.ts", "index.js", "index.mts", "index.mjs",
		"src/index.ts", "src/index.js",
		"plugin.ts", "plugin.js",
	}

	for _, candidate := range candidates {
		path := filepath.Join(pluginPath, candidate)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no entry point found")
}

// setupEnvironment sets up the JavaScript environment
func (b *JSBridge) setupEnvironment(vm *goja.Runtime, pluginID, pluginPath string) error {
	// Create console object
	console := vm.NewObject()
	console.Set("log", func(call goja.FunctionCall) goja.Value {
		args := make([]interface{}, len(call.Arguments))
		for i, arg := range call.Arguments {
			args[i] = arg.Export()
		}
		logger.Info().Str("plugin_id", pluginID).Interface("args", args).Msg("console.log")
		return goja.Undefined()
	})
	console.Set("error", func(call goja.FunctionCall) goja.Value {
		args := make([]interface{}, len(call.Arguments))
		for i, arg := range call.Arguments {
			args[i] = arg.Export()
		}
		logger.Error().Str("plugin_id", pluginID).Interface("args", args).Msg("console.error")
		return goja.Undefined()
	})
	console.Set("warn", func(call goja.FunctionCall) goja.Value {
		args := make([]interface{}, len(call.Arguments))
		for i, arg := range call.Arguments {
			args[i] = arg.Export()
		}
		logger.Warn().Str("plugin_id", pluginID).Interface("args", args).Msg("console.warn")
		return goja.Undefined()
	})
	vm.Set("console", console)

	// Create plugin API
	api := b.createJSPluginAPI(vm, pluginID)
	vm.Set("__pluginApi", api)

	// Create require function (simplified)
	vm.Set("require", func(call goja.FunctionCall) goja.Value {
		moduleName := call.Argument(0).String()

		// Handle clawdbot/plugin-sdk
		if moduleName == "clawdbot/plugin-sdk" || moduleName == "@clawdbot/plugin-sdk" {
			return b.createPluginSDK(vm)
		}

		// For other modules, return empty object
		logger.Warn().Str("module", moduleName).Msg("Module not available")
		return vm.NewObject()
	})

	// Set __dirname and __filename
	vm.Set("__dirname", pluginPath)
	vm.Set("__filename", filepath.Join(pluginPath, "index.js"))

	return nil
}

// createJSPluginAPI creates the plugin API for JavaScript
func (b *JSBridge) createJSPluginAPI(vm *goja.Runtime, pluginID string) *goja.Object {
	api := vm.NewObject()
	goAPI := b.registry.createPluginAPI(pluginID)

	// registerTool
	api.Set("registerTool", func(call goja.FunctionCall) goja.Value {
		toolObj := call.Argument(0).Export()
		toolMap, ok := toolObj.(map[string]interface{})
		if !ok {
			logger.Error().Msg("registerTool: invalid tool object")
			return goja.Undefined()
		}

		tool := Tool{
			Name:        getString(toolMap, "name"),
			Description: getString(toolMap, "description"),
			Parameters:  getMap(toolMap, "parameters"),
		}

		// Get handler function
		if handlerVal, ok := toolMap["handler"]; ok {
			if handlerFunc, ok := handlerVal.(func(goja.FunctionCall) goja.Value); ok {
				tool.Handler = func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
					paramsVal := vm.ToValue(params)
					result := handlerFunc(goja.FunctionCall{Arguments: []goja.Value{paramsVal}})
					return result.Export(), nil
				}
			}
		}

		if err := goAPI.RegisterTool(tool); err != nil {
			logger.Error().Err(err).Msg("Failed to register tool")
		}
		return goja.Undefined()
	})

	// registerHook / on
	registerHookFn := func(call goja.FunctionCall) goja.Value {
		event := call.Argument(0).String()
		handlerVal := call.Argument(1)

		if handlerFunc, ok := goja.AssertFunction(handlerVal); ok {
			goAPI.RegisterHook(event, func(ctx context.Context, data interface{}) error {
				dataVal := vm.ToValue(data)
				_, err := handlerFunc(goja.Undefined(), dataVal)
				if err != nil {
					return fmt.Errorf("hook error: %v", err)
				}
				return nil
			})
		}
		return goja.Undefined()
	}
	api.Set("registerHook", registerHookFn)
	api.Set("on", registerHookFn)

	// registerCommand
	api.Set("registerCommand", func(call goja.FunctionCall) goja.Value {
		cmdObj := call.Argument(0).Export()
		cmdMap, ok := cmdObj.(map[string]interface{})
		if !ok {
			return goja.Undefined()
		}

		cmd := Command{
			Name:        getString(cmdMap, "name"),
			Description: getString(cmdMap, "description"),
		}

		if handlerVal, ok := cmdMap["handler"]; ok {
			if handlerFunc, ok := handlerVal.(func(goja.FunctionCall) goja.Value); ok {
				cmd.Handler = func(ctx context.Context, args []string) (string, error) {
					argsVal := vm.ToValue(args)
					result := handlerFunc(goja.FunctionCall{Arguments: []goja.Value{argsVal}})
					return result.String(), nil
				}
			}
		}

		goAPI.RegisterCommand(cmd)
		return goja.Undefined()
	})

	// Logger
	loggerObj := vm.NewObject()
	loggerObj.Set("debug", func(call goja.FunctionCall) goja.Value {
		goAPI.Logger().Debug(call.Argument(0).String())
		return goja.Undefined()
	})
	loggerObj.Set("info", func(call goja.FunctionCall) goja.Value {
		goAPI.Logger().Info(call.Argument(0).String())
		return goja.Undefined()
	})
	loggerObj.Set("warn", func(call goja.FunctionCall) goja.Value {
		goAPI.Logger().Warn(call.Argument(0).String())
		return goja.Undefined()
	})
	loggerObj.Set("error", func(call goja.FunctionCall) goja.Value {
		goAPI.Logger().Error(call.Argument(0).String())
		return goja.Undefined()
	})
	api.Set("logger", loggerObj)

	// Plugin metadata
	api.Set("id", pluginID)

	return api
}

// createPluginSDK creates the plugin SDK module
func (b *JSBridge) createPluginSDK(vm *goja.Runtime) goja.Value {
	sdk := vm.NewObject()

	// Export common types and utilities
	sdk.Set("definePlugin", func(call goja.FunctionCall) goja.Value {
		return call.Argument(0)
	})

	return sdk
}

// transpileTypeScript performs basic TypeScript to JavaScript transpilation
// Note: This is a simplified version. For production, consider using a proper transpiler.
func (b *JSBridge) transpileTypeScript(code string) string {
	// Remove type annotations (simplified)
	// In production, use a proper TypeScript compiler or esbuild

	// Remove import type statements
	lines := strings.Split(code, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import type") {
			continue
		}
		result = append(result, line)
	}

	code = strings.Join(result, "\n")

	// Remove type annotations from function parameters (basic)
	// This is very simplified - production should use proper transpilation

	return code
}

// wrapAsModule wraps code in a module format
func (b *JSBridge) wrapAsModule(code string) string {
	return fmt.Sprintf(`
(function(exports, require, module, __filename, __dirname, __pluginApi) {
	var api = __pluginApi;

	// Plugin code
	%s

	// If default export is a function, call it with api
	if (typeof module.exports === 'function') {
		module.exports(api);
	} else if (module.exports && typeof module.exports.register === 'function') {
		module.exports.register(api);
	} else if (typeof exports.default === 'function') {
		exports.default(api);
	} else if (exports.default && typeof exports.default.register === 'function') {
		exports.default.register(api);
	}
})({}, require, {exports: {}}, __filename, __dirname, __pluginApi);
`, code)
}

// Close closes all VMs
func (b *JSBridge) Close() {
	// goja VMs don't need explicit cleanup
	b.vms = make(map[string]*goja.Runtime)
}

// Helper functions
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getMap(m map[string]interface{}, key string) map[string]interface{} {
	if v, ok := m[key]; ok {
		if m, ok := v.(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}
