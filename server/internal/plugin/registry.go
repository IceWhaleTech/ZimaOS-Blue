package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
)

// Registry manages all loaded plugins
type Registry struct {
	plugins         map[string]*PluginInfo
	nativePlugins   map[string]NativePlugin
	tools           map[string]*Tool
	hooks           map[string][]HookHandler
	services        map[string]Service
	commands        map[string]*Command
	httpHandlers    map[string]HTTPHandler
	mu              sync.RWMutex
	isolationConfig *IsolationConfig
}

// NewRegistry creates a new plugin registry
func NewRegistry() *Registry {
	return NewRegistryWithConfig(nil)
}

// NewRegistryWithConfig creates a new plugin registry with custom isolation config
func NewRegistryWithConfig(config *IsolationConfig) *Registry {
	if config == nil {
		config = DefaultIsolationConfig()
	}
	return &Registry{
		plugins:         make(map[string]*PluginInfo),
		nativePlugins:   make(map[string]NativePlugin),
		tools:           make(map[string]*Tool),
		hooks:           make(map[string][]HookHandler),
		services:        make(map[string]Service),
		commands:        make(map[string]*Command),
		httpHandlers:    make(map[string]HTTPHandler),
		isolationConfig: config,
	}
}

// RegisterNativePlugin registers a Go native plugin
func (r *Registry) RegisterNativePlugin(plugin NativePlugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := plugin.ID()
	if _, exists := r.plugins[id]; exists {
		return fmt.Errorf("plugin %s already registered", id)
	}

	manifest := plugin.Manifest()
	r.plugins[id] = &PluginInfo{
		Manifest: manifest,
		Origin:   OriginNative,
		Status:   StatusLoaded,
		IsNative: true,
	}
	r.nativePlugins[id] = plugin

	logger.Info().Str("plugin_id", id).Msg("Registered native plugin")
	return nil
}

// LoadPluginsFromDir loads plugins from a directory
func (r *Registry) LoadPluginsFromDir(ctx context.Context, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist, skip
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginPath := filepath.Join(dir, entry.Name())
		if err := r.loadPlugin(ctx, pluginPath, OriginWorkspace); err != nil {
			logger.Warn().Err(err).Str("path", pluginPath).Msg("Failed to load plugin")
		}
	}

	return nil
}

// loadPlugin loads a single plugin from a path
func (r *Registry) loadPlugin(ctx context.Context, path string, origin PluginOrigin) error {
	// Look for manifest file
	manifestPath := filepath.Join(path, "clawdbot.plugin.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Validate manifest
	if manifest.ID == "" {
		return fmt.Errorf("manifest missing required field: id")
	}
	if manifest.ConfigSchema == nil {
		manifest.ConfigSchema = make(map[string]interface{})
	}

	// Fill in missing metadata fields with reasonable defaults
	if manifest.Name == "" {
		manifest.Name = formatPluginName(manifest.ID)
	}
	if manifest.Description == "" {
		manifest.Description = generatePluginDescription(&manifest)
	}
	if manifest.Version == "" {
		manifest.Version = "1.0.0"
	}
	if manifest.Author == "" {
		manifest.Author = "community"
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for duplicate
	if existing, exists := r.plugins[manifest.ID]; exists {
		logger.Warn().
			Str("plugin_id", manifest.ID).
			Str("existing_path", existing.Path).
			Str("new_path", path).
			Msg("Plugin already loaded, skipping")
		return nil
	}

	r.plugins[manifest.ID] = &PluginInfo{
		Manifest: &manifest,
		Origin:   origin,
		Status:   StatusLoaded,
		Path:     path,
		IsNative: false,
	}

	logger.Info().
		Str("plugin_id", manifest.ID).
		Str("path", path).
		Str("origin", string(origin)).
		Msg("Loaded plugin manifest")

	return nil
}

// InitializePlugins initializes all loaded plugins in dependency order
func (r *Registry) InitializePlugins(ctx context.Context) error {
	r.mu.RLock()
	pluginsCopy := make(map[string]*PluginInfo, len(r.plugins))
	for id, info := range r.plugins {
		pluginsCopy[id] = info
	}
	r.mu.RUnlock()

	// Resolve dependencies
	resolver := NewDependencyResolver(pluginsCopy)
	result := resolver.Resolve()

	// Log warnings
	for _, warning := range result.Warnings {
		logger.Warn().Msg(warning)
	}

	// Check for errors
	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			logger.Error().Err(err).Msg("Dependency resolution error")
		}
		// Continue with plugins that don't have dependency errors
	}

	// Initialize plugins in dependency order
	for _, pluginID := range result.Order {
		r.mu.RLock()
		plugin, exists := r.nativePlugins[pluginID]
		r.mu.RUnlock()

		if !exists {
			continue // Not a native plugin, skip
		}

		// Check if dependencies are satisfied
		satisfied, depErrors := resolver.CheckDependenciesSatisfied(pluginID)
		if !satisfied {
			for _, err := range depErrors {
				logger.Error().Err(err).Str("plugin_id", pluginID).Msg("Dependency not satisfied")
			}
			r.mu.Lock()
			if info, exists := r.plugins[pluginID]; exists {
				info.Status = StatusError
				info.Error = "dependency not satisfied"
			}
			r.mu.Unlock()
			continue
		}

		api := r.createPluginAPI(plugin.ID())

		// Use isolated execution for plugin initialization
		err := safeExecute(ctx, r.isolationConfig.InitTimeout, plugin.ID(), "init", func(ctx context.Context) error {
			return plugin.Init(ctx, api)
		})

		if err != nil {
			r.mu.Lock()
			if info, exists := r.plugins[plugin.ID()]; exists {
				info.Status = StatusError
				info.Error = err.Error()
			}
			r.mu.Unlock()
			logger.Error().Err(err).Str("plugin_id", plugin.ID()).Msg("Failed to initialize plugin")
			continue
		}
	}

	return nil
}

// StartPlugins starts all loaded plugins in dependency order
func (r *Registry) StartPlugins(ctx context.Context) error {
	r.mu.RLock()
	pluginsCopy := make(map[string]*PluginInfo, len(r.plugins))
	for id, info := range r.plugins {
		pluginsCopy[id] = info
	}
	r.mu.RUnlock()

	// Resolve dependencies for start order
	resolver := NewDependencyResolver(pluginsCopy)
	result := resolver.Resolve()

	// Start plugins in dependency order
	for _, pluginID := range result.Order {
		r.mu.RLock()
		plugin, exists := r.nativePlugins[pluginID]
		info := r.plugins[pluginID]
		r.mu.RUnlock()

		if !exists {
			continue // Not a native plugin, skip
		}

		// Skip plugins in error state
		if info != nil && info.Status == StatusError {
			logger.Warn().Str("plugin_id", pluginID).Msg("Skipping plugin in error state")
			continue
		}

		// Use isolated execution for plugin start
		err := safeExecute(ctx, r.isolationConfig.StartTimeout, plugin.ID(), "start", func(ctx context.Context) error {
			return plugin.Start(ctx)
		})

		if err != nil {
			r.mu.Lock()
			if info, exists := r.plugins[plugin.ID()]; exists {
				info.Status = StatusError
				info.Error = err.Error()
			}
			r.mu.Unlock()
			logger.Error().Err(err).Str("plugin_id", plugin.ID()).Msg("Failed to start plugin")
			continue
		}
		logger.Info().Str("plugin_id", plugin.ID()).Msg("Started plugin")
	}

	// Start registered services
	r.mu.RLock()
	services := make([]Service, 0, len(r.services))
	for _, s := range r.services {
		services = append(services, s)
	}
	r.mu.RUnlock()

	for _, service := range services {
		// Use isolated execution for service start
		err := safeExecute(ctx, r.isolationConfig.StartTimeout, "service", fmt.Sprintf("start.%s", service.Name()), func(ctx context.Context) error {
			return service.Start(ctx)
		})

		if err != nil {
			logger.Error().Err(err).Str("service", service.Name()).Msg("Failed to start service")
			continue
		}
		logger.Info().Str("service", service.Name()).Msg("Started service")
	}

	return nil
}

// StopPlugins stops all loaded plugins
func (r *Registry) StopPlugins(ctx context.Context) error {
	// Stop services first
	r.mu.RLock()
	services := make([]Service, 0, len(r.services))
	for _, s := range r.services {
		services = append(services, s)
	}
	r.mu.RUnlock()

	for _, service := range services {
		// Use isolated execution for service stop
		err := safeExecute(ctx, r.isolationConfig.StopTimeout, "service", fmt.Sprintf("stop.%s", service.Name()), func(ctx context.Context) error {
			return service.Stop(ctx)
		})

		if err != nil {
			logger.Error().Err(err).Str("service", service.Name()).Msg("Failed to stop service")
		}
	}

	// Stop native plugins
	r.mu.RLock()
	nativePlugins := make([]NativePlugin, 0, len(r.nativePlugins))
	for _, p := range r.nativePlugins {
		nativePlugins = append(nativePlugins, p)
	}
	r.mu.RUnlock()

	for _, plugin := range nativePlugins {
		// Use isolated execution for plugin stop
		err := safeExecute(ctx, r.isolationConfig.StopTimeout, plugin.ID(), "stop", func(ctx context.Context) error {
			return plugin.Stop(ctx)
		})

		if err != nil {
			logger.Error().Err(err).Str("plugin_id", plugin.ID()).Msg("Failed to stop plugin")
		}
	}

	return nil
}

// GetPlugin returns plugin info by ID
func (r *Registry) GetPlugin(id string) *PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.plugins[id]
}

// ListPlugins returns all loaded plugins
func (r *Registry) ListPlugins() []*PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugins := make([]*PluginInfo, 0, len(r.plugins))
	for _, p := range r.plugins {
		plugins = append(plugins, p)
	}
	return plugins
}

// GetTool returns a tool by name
func (r *Registry) GetTool(name string) *Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// ListTools returns all registered tools
func (r *Registry) ListTools() []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]*Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

// GetCommand returns a command by name
func (r *Registry) GetCommand(name string) *Command {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.commands[name]
}

// ListCommands returns all registered commands
func (r *Registry) ListCommands() []*Command {
	r.mu.RLock()
	defer r.mu.RUnlock()

	commands := make([]*Command, 0, len(r.commands))
	for _, c := range r.commands {
		commands = append(commands, c)
	}
	return commands
}

// TriggerHook triggers all handlers for a hook event
func (r *Registry) TriggerHook(ctx context.Context, event string, data interface{}) error {
	r.mu.RLock()
	handlers := r.hooks[event]
	timeout := r.isolationConfig.HookTimeout
	r.mu.RUnlock()

	for i, handler := range handlers {
		// Use isolated execution for hook handlers
		err := safeExecute(ctx, timeout, fmt.Sprintf("hook.%s.%d", event, i), "trigger", func(ctx context.Context) error {
			return handler(ctx, data)
		})

		if err != nil {
			logger.Error().Err(err).Str("event", event).Int("handler_index", i).Msg("Hook handler error")
			// Continue processing other handlers even if one fails
		}
	}

	return nil
}

// GetHTTPHandler returns an HTTP handler by path
func (r *Registry) GetHTTPHandler(path string) HTTPHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.httpHandlers[path]
}

// ResolveDependencies resolves plugin dependencies and returns the result
func (r *Registry) ResolveDependencies() *ResolutionResult {
	r.mu.RLock()
	pluginsCopy := make(map[string]*PluginInfo, len(r.plugins))
	for id, info := range r.plugins {
		pluginsCopy[id] = info
	}
	r.mu.RUnlock()

	resolver := NewDependencyResolver(pluginsCopy)
	return resolver.Resolve()
}

// GetPluginDependencies returns the dependencies of a plugin
func (r *Registry) GetPluginDependencies(pluginID string) []string {
	r.mu.RLock()
	pluginsCopy := make(map[string]*PluginInfo, len(r.plugins))
	for id, info := range r.plugins {
		pluginsCopy[id] = info
	}
	r.mu.RUnlock()

	resolver := NewDependencyResolver(pluginsCopy)
	return resolver.GetDependencies(pluginID)
}

// GetPluginDependents returns plugins that depend on the given plugin
func (r *Registry) GetPluginDependents(pluginID string) []string {
	r.mu.RLock()
	pluginsCopy := make(map[string]*PluginInfo, len(r.plugins))
	for id, info := range r.plugins {
		pluginsCopy[id] = info
	}
	r.mu.RUnlock()

	resolver := NewDependencyResolver(pluginsCopy)
	return resolver.GetDependents(pluginID)
}

// createPluginAPI creates a PluginAPI for a plugin
func (r *Registry) createPluginAPI(pluginID string) PluginAPI {
	return &pluginAPIImpl{
		registry: r,
		pluginID: pluginID,
	}
}

// pluginAPIImpl implements PluginAPI
type pluginAPIImpl struct {
	registry *Registry
	pluginID string
}

func (a *pluginAPIImpl) PluginID() string {
	return a.pluginID
}

func (a *pluginAPIImpl) PluginConfig() map[string]interface{} {
	a.registry.mu.RLock()
	defer a.registry.mu.RUnlock()

	if info, exists := a.registry.plugins[a.pluginID]; exists {
		return info.Config
	}
	return nil
}

func (a *pluginAPIImpl) RegisterTool(tool Tool) error {
	a.registry.mu.Lock()
	defer a.registry.mu.Unlock()

	if _, exists := a.registry.tools[tool.Name]; exists {
		return fmt.Errorf("tool %s already registered", tool.Name)
	}

	// Wrap the handler with isolation
	if tool.Handler != nil {
		tool.Handler = WrapToolHandler(a.pluginID, tool.Handler, a.registry.isolationConfig.ToolTimeout)
	}

	a.registry.tools[tool.Name] = &tool
	logger.Info().Str("plugin_id", a.pluginID).Str("tool", tool.Name).Msg("Registered tool")
	return nil
}

func (a *pluginAPIImpl) RegisterHook(event string, handler HookHandler) error {
	a.registry.mu.Lock()
	defer a.registry.mu.Unlock()

	// Wrap the handler with isolation
	wrappedHandler := WrapHookHandler(a.pluginID, handler, a.registry.isolationConfig.HookTimeout)

	a.registry.hooks[event] = append(a.registry.hooks[event], wrappedHandler)
	logger.Info().Str("plugin_id", a.pluginID).Str("event", event).Msg("Registered hook")
	return nil
}

func (a *pluginAPIImpl) RegisterService(service Service) error {
	a.registry.mu.Lock()
	defer a.registry.mu.Unlock()

	name := service.Name()
	if _, exists := a.registry.services[name]; exists {
		return fmt.Errorf("service %s already registered", name)
	}

	// Wrap the service with isolation
	isolatedService := NewIsolatedService(service, a.pluginID, a.registry.isolationConfig)

	a.registry.services[name] = isolatedService
	logger.Info().Str("plugin_id", a.pluginID).Str("service", name).Msg("Registered service")
	return nil
}

func (a *pluginAPIImpl) RegisterCommand(command Command) error {
	a.registry.mu.Lock()
	defer a.registry.mu.Unlock()

	if _, exists := a.registry.commands[command.Name]; exists {
		return fmt.Errorf("command %s already registered", command.Name)
	}

	// Wrap the handler with isolation
	if command.Handler != nil {
		command.Handler = WrapCommandHandler(a.pluginID, command.Handler, a.registry.isolationConfig.CommandTimeout)
	}

	a.registry.commands[command.Name] = &command
	logger.Info().Str("plugin_id", a.pluginID).Str("command", command.Name).Msg("Registered command")
	return nil
}

func (a *pluginAPIImpl) RegisterHTTPHandler(path string, handler HTTPHandler) error {
	a.registry.mu.Lock()
	defer a.registry.mu.Unlock()

	if _, exists := a.registry.httpHandlers[path]; exists {
		return fmt.Errorf("HTTP handler for %s already registered", path)
	}

	// Wrap the handler with isolation
	wrappedHandler := WrapHTTPHandler(a.pluginID, handler, a.registry.isolationConfig.HTTPTimeout)

	a.registry.httpHandlers[path] = wrappedHandler
	logger.Info().Str("plugin_id", a.pluginID).Str("path", path).Msg("Registered HTTP handler")
	return nil
}

func (a *pluginAPIImpl) Logger() Logger {
	return &pluginLogger{pluginID: a.pluginID}
}

// pluginLogger implements Logger for plugins
type pluginLogger struct {
	pluginID string
}

func (l *pluginLogger) Debug(msg string, fields ...interface{}) {
	logger.Debug().Str("plugin_id", l.pluginID).Interface("fields", fields).Msg(msg)
}

func (l *pluginLogger) Info(msg string, fields ...interface{}) {
	logger.Info().Str("plugin_id", l.pluginID).Interface("fields", fields).Msg(msg)
}

func (l *pluginLogger) Warn(msg string, fields ...interface{}) {
	logger.Warn().Str("plugin_id", l.pluginID).Interface("fields", fields).Msg(msg)
}

func (l *pluginLogger) Error(msg string, fields ...interface{}) {
	logger.Error().Str("plugin_id", l.pluginID).Interface("fields", fields).Msg(msg)
}

// StartPlugin starts a specific plugin by ID
func (r *Registry) StartPlugin(ctx context.Context, id string) error {
	r.mu.RLock()
	plugin, exists := r.nativePlugins[id]
	info := r.plugins[id]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("plugin %s not found or not a native plugin", id)
	}

	// Skip plugins in error state
	if info != nil && info.Status == StatusError {
		return fmt.Errorf("plugin %s is in error state", id)
	}

	err := safeExecute(ctx, r.isolationConfig.StartTimeout, id, "start", func(ctx context.Context) error {
		return plugin.Start(ctx)
	})

	if err != nil {
		r.mu.Lock()
		if info, exists := r.plugins[id]; exists {
			info.Status = StatusError
			info.Error = err.Error()
		}
		r.mu.Unlock()
		return fmt.Errorf("failed to start plugin %s: %w", id, err)
	}

	r.mu.Lock()
	if info, exists := r.plugins[id]; exists {
		info.Status = StatusLoaded
	}
	r.mu.Unlock()

	logger.Info().Str("plugin_id", id).Msg("Started plugin")
	return nil
}

// StopPlugin stops a specific plugin by ID
func (r *Registry) StopPlugin(ctx context.Context, id string) error {
	r.mu.RLock()
	plugin, exists := r.nativePlugins[id]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("plugin %s not found or not a native plugin", id)
	}

	err := safeExecute(ctx, r.isolationConfig.StopTimeout, id, "stop", func(ctx context.Context) error {
		return plugin.Stop(ctx)
	})

	if err != nil {
		return fmt.Errorf("failed to stop plugin %s: %w", id, err)
	}

	r.mu.Lock()
	if info, exists := r.plugins[id]; exists {
		info.Status = StatusStopped
	}
	r.mu.Unlock()

	logger.Info().Str("plugin_id", id).Msg("Stopped plugin")
	return nil
}

// UnregisterPlugin removes a plugin from the registry
func (r *Registry) UnregisterPlugin(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.plugins, id)
	delete(r.nativePlugins, id)

	logger.Info().Str("plugin_id", id).Msg("Unregistered plugin")
}

// SetPluginStatus updates the status of a plugin
func (r *Registry) SetPluginStatus(id string, status PluginStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, exists := r.plugins[id]
	if !exists {
		return fmt.Errorf("plugin %s not found", id)
	}

	info.Status = status
	logger.Info().Str("plugin_id", id).Str("status", string(status)).Msg("Updated plugin status")
	return nil
}

// formatPluginName converts plugin ID to display name
func formatPluginName(id string) string {
	// Convert kebab-case to Title Case
	words := strings.Split(id, "-")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

// generatePluginDescription generates a description based on the manifest
func generatePluginDescription(manifest *Manifest) string {
	name := manifest.Name
	if name == "" {
		name = formatPluginName(manifest.ID)
	}

	// Generate description based on plugin type
	if len(manifest.Channels) > 0 {
		return fmt.Sprintf("%s messaging integration", name)
	}
	if len(manifest.Skills) > 0 {
		return fmt.Sprintf("%s skills and capabilities", name)
	}
	if len(manifest.Providers) > 0 {
		return fmt.Sprintf("%s provider integration", name)
	}
	if manifest.Kind == PluginKindMemory {
		return fmt.Sprintf("%s memory storage", name)
	}
	if manifest.Kind == PluginKindTool {
		return fmt.Sprintf("%s tool integration", name)
	}

	// Default description
	return fmt.Sprintf("%s plugin", name)
}
