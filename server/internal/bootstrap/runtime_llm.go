package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

var errRuntimeLLMUnavailable = fmt.Errorf("runtime llm backend not configured")

type runtimeLLMProviderRef struct {
	mu       sync.RWMutex
	provider llm.Provider
}

func newRuntimeLLMProviderRef() *runtimeLLMProviderRef {
	return &runtimeLLMProviderRef{}
}

func (r *runtimeLLMProviderRef) SetProvider(provider llm.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.provider = provider
}

func (r *runtimeLLMProviderRef) currentProvider() llm.Provider {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.provider
}

func (r *runtimeLLMProviderRef) Name() string {
	if provider := r.currentProvider(); provider != nil {
		return provider.Name()
	}
	return "runtime"
}

func (r *runtimeLLMProviderRef) Models() []string {
	if provider := r.currentProvider(); provider != nil {
		return provider.Models()
	}
	return nil
}

func (r *runtimeLLMProviderRef) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	provider := r.currentProvider()
	if provider == nil {
		return nil, errRuntimeLLMUnavailable
	}
	return provider.Chat(ctx, req)
}

func (r *runtimeLLMProviderRef) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	provider := r.currentProvider()
	if provider == nil {
		return nil, errRuntimeLLMUnavailable
	}
	return provider.ChatStream(ctx, req)
}

func (r *runtimeLLMProviderRef) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) error {
	provider := r.currentProvider()
	if provider == nil {
		return errRuntimeLLMUnavailable
	}
	return provider.ChatStreamCallback(ctx, req, callback)
}

type proxyBridgeProvider struct {
	bridge            *proxybridge.Bridge
	claudeCodeHandler *claudecode.Handler
	providerPool      *providerpool.Pool
}

func newProxyBridgeProvider(bridge *proxybridge.Bridge, handler *claudecode.Handler, pool *providerpool.Pool) *proxyBridgeProvider {
	return &proxyBridgeProvider{
		bridge:            bridge,
		claudeCodeHandler: handler,
		providerPool:      pool,
	}
}

func (p *proxyBridgeProvider) Name() string {
	return "proxy"
}

func (p *proxyBridgeProvider) Models() []string {
	return nil
}

func (p *proxyBridgeProvider) normalize(req *llm.ChatRequest) {
	if p == nil {
		return
	}
	req.Model = resolveDefaultModelForCCCLI(req.Model, p.claudeCodeHandler, p.providerPool)
}

func (p *proxyBridgeProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if p == nil || p.bridge == nil {
		return nil, fmt.Errorf("proxy bridge is not configured")
	}
	p.normalize(&req)
	return p.bridge.Chat(ctx, req)
}

func (p *proxyBridgeProvider) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	if p == nil || p.bridge == nil {
		return nil, fmt.Errorf("proxy bridge is not configured")
	}
	p.normalize(&req)
	ch := make(chan llm.StreamChunk, 64)
	go func() {
		defer close(ch)
		_ = p.bridge.ChatStream(ctx, req, func(chunk llm.StreamChunk) error {
			ch <- chunk
			return nil
		})
	}()
	return ch, nil
}

func (p *proxyBridgeProvider) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) error {
	if p == nil || p.bridge == nil {
		return fmt.Errorf("proxy bridge is not configured")
	}
	p.normalize(&req)
	return p.bridge.ChatStream(ctx, req, callback)
}

type claudeCodeRuntimeFactory struct {
	mu            sync.Mutex
	handler       *claudecode.Handler
	appConfig     *config.Config
	serverPort    int
	toolRegistry  *tools.Registry
	workspaceDir  string
	apiKeyService *auth.APIKeyService
	logger        *zap.Logger

	provider    *claudecode.Provider
	signature   string
	proxyAPIKey string
}

func newClaudeCodeRuntimeFactory(handler *claudecode.Handler, appConfig *config.Config, serverPort int, toolRegistry *tools.Registry, workspaceDir string, apiKeyService *auth.APIKeyService, logger *zap.Logger) *claudeCodeRuntimeFactory {
	return &claudeCodeRuntimeFactory{
		handler:       handler,
		appConfig:     appConfig,
		serverPort:    serverPort,
		toolRegistry:  toolRegistry,
		workspaceDir:  workspaceDir,
		apiKeyService: apiKeyService,
		logger:        logger,
	}
}

func (f *claudeCodeRuntimeFactory) Provider() (*claudecode.Provider, error) {
	if f == nil {
		return nil, fmt.Errorf("claude code runtime factory is not configured")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	cfg, signature := f.buildConfigLocked()
	if f.provider != nil && signature == f.signature {
		return f.provider, nil
	}

	provider := claudecode.NewProvider(cfg)
	if f.toolRegistry != nil {
		provider.SetToolRegistry(f.toolRegistry)
	}
	if strings.TrimSpace(f.workspaceDir) != "" {
		provider.SetWorkspace(workspace.NewManager(f.workspaceDir))
	}

	f.provider = provider
	f.signature = signature
	return provider, nil
}

func (f *claudeCodeRuntimeFactory) buildConfigLocked() (*claudecode.ClaudeCodeConfig, string) {
	cfg := buildClaudeCodeRuntimeConfig(f.appConfig, f.handler, f.serverPort, f.workspaceDir, f.ensureProxyAPIKeyLocked())
	sigBytes, err := json.Marshal(cfg)
	if err != nil {
		return cfg, fmt.Sprintf("fallback:%v", err)
	}
	return cfg, string(sigBytes)
}

func (f *claudeCodeRuntimeFactory) ensureProxyAPIKeyLocked() string {
	if strings.TrimSpace(f.proxyAPIKey) != "" {
		return f.proxyAPIKey
	}
	if f.apiKeyService != nil {
		info, err := f.apiKeyService.CreateKey(context.Background(), &auth.CreateKeyRequest{
			Name:   "claudecode-internal",
			Scopes: []string{"chat", "proxy", "route:auto"},
		})
		if err == nil && info != nil && strings.TrimSpace(info.Key) != "" {
			f.proxyAPIKey = strings.TrimSpace(info.Key)
			return f.proxyAPIKey
		}
		if f.logger != nil {
			f.logger.Warn("Failed to create internal API key for Claude Code runtime, falling back to placeholder token", zap.Error(err))
		}
	}
	f.proxyAPIKey = "blue-internal-claudecode"
	return f.proxyAPIKey
}

type runtimeDispatchProvider struct {
	proxy             llm.Provider
	claudeCodeHandler *claudecode.Handler
	claudeCodeFactory *claudeCodeRuntimeFactory
}

func newRuntimeDispatchProvider(proxy llm.Provider, handler *claudecode.Handler, factory *claudeCodeRuntimeFactory) *runtimeDispatchProvider {
	return &runtimeDispatchProvider{
		proxy:             proxy,
		claudeCodeHandler: handler,
		claudeCodeFactory: factory,
	}
}

func (p *runtimeDispatchProvider) Name() string {
	if provider, err := p.activeProvider(); err == nil && provider != nil {
		return provider.Name()
	}
	return "runtime"
}

func (p *runtimeDispatchProvider) Models() []string {
	if provider, err := p.activeProvider(); err == nil && provider != nil {
		return provider.Models()
	}
	return nil
}

func (p *runtimeDispatchProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	provider, err := p.activeProvider()
	if err != nil {
		return nil, err
	}
	req = p.normalizeRequest(req)
	return provider.Chat(ctx, req)
}

func (p *runtimeDispatchProvider) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	provider, err := p.activeProvider()
	if err != nil {
		return nil, err
	}
	req = p.normalizeRequest(req)
	return provider.ChatStream(ctx, req)
}

func (p *runtimeDispatchProvider) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) error {
	provider, err := p.activeProvider()
	if err != nil {
		return err
	}
	req = p.normalizeRequest(req)
	return provider.ChatStreamCallback(ctx, req, callback)
}

func (p *runtimeDispatchProvider) activeProvider() (llm.Provider, error) {
	if p != nil && p.claudeCodeHandler != nil && p.claudeCodeHandler.IsEnabled() {
		if p.claudeCodeFactory == nil {
			return nil, fmt.Errorf("claude code runtime is not configured")
		}
		return p.claudeCodeFactory.Provider()
	}
	if p == nil || p.proxy == nil {
		return nil, fmt.Errorf("proxy bridge is not configured")
	}
	return p.proxy, nil
}

func (p *runtimeDispatchProvider) normalizeRequest(req llm.ChatRequest) llm.ChatRequest {
	if p == nil || p.claudeCodeHandler == nil || !p.claudeCodeHandler.IsEnabled() {
		return req
	}
	if model := strings.TrimSpace(req.Model); model == "" || strings.EqualFold(model, "auto") || strings.EqualFold(model, defaultCCCLIModel) {
		req.Model = ""
	}
	return req
}

func buildClaudeCodeRuntimeConfig(appCfg *config.Config, handler *claudecode.Handler, serverPort int, workspaceDir, proxyAPIKey string) *claudecode.ClaudeCodeConfig {
	base := claudecode.DefaultClaudeCodeConfig()
	cfg := &base

	overlayLegacyClaudeCodeConfig(cfg, appCfg)
	overlayClaudeCodeCLIConfig(cfg, appCfg)

	if strings.TrimSpace(workspaceDir) != "" {
		cfg.WorkspaceDir = strings.TrimSpace(workspaceDir)
	}

	snapshot := claudecode.ClaudeCodePersistentConfig{}
	if handler != nil {
		snapshot = handler.ConfigSnapshot()
	}

	if model := strings.TrimSpace(snapshot.DefaultModel); model != "" {
		cfg.DefaultModel = model
	}
	cfg.Enabled = true
	cfg.APIKey = strings.TrimSpace(proxyAPIKey)
	cfg.BaseURL = fmt.Sprintf("http://127.0.0.1:%d", serverPort)
	cfg.ActualProvider = "proxy"
	cfg.ActualModel = ""
	cfg.Sandbox.Enabled = snapshot.SandboxEnabled
	cfg.Sandbox.NetworkEnabled = snapshot.NetworkEnabled
	cfg.Sandbox.AllowedPaths = buildClaudeCodeAllowedPaths(cfg.Sandbox.AllowedPaths, workspaceDir, snapshot)
	return cfg
}

func buildClaudeCodeAllowedPaths(existing []string, workspaceDir string, snapshot claudecode.ClaudeCodePersistentConfig) []string {
	seen := make(map[string]struct{})
	paths := make([]string, 0, len(existing)+len(snapshot.DirectoryWhitelist)+1)
	addPath := func(raw string) {
		path := strings.TrimSpace(raw)
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}

	for _, path := range existing {
		addPath(path)
	}
	addPath(workspaceDir)
	if snapshot.WhitelistEnabled {
		for _, entry := range snapshot.DirectoryWhitelist {
			addPath(entry.Path)
		}
	}
	return paths
}

func overlayLegacyClaudeCodeConfig(dst *claudecode.ClaudeCodeConfig, appCfg *config.Config) {
	if dst == nil || appCfg == nil {
		return
	}
	legacy := appCfg.ClaudeCode
	if command := strings.TrimSpace(legacy.Command); command != "" {
		dst.Command = command
		dst.Backend.Command = command
	}
	if workspaceDir := strings.TrimSpace(legacy.WorkspaceDir); workspaceDir != "" {
		dst.WorkspaceDir = workspaceDir
	}
	if model := strings.TrimSpace(legacy.DefaultModel); model != "" {
		dst.DefaultModel = model
	}
	if legacy.Timeout > 0 {
		dst.Timeout = legacy.Timeout
	}
	if legacy.SessionTTL > 0 {
		dst.SessionTTL = legacy.SessionTTL
	}
	if len(legacy.Backend.Args) > 0 {
		dst.Backend.Args = append([]string{}, legacy.Backend.Args...)
	}
	if len(legacy.Backend.ResumeArgs) > 0 {
		dst.Backend.ResumeArgs = append([]string{}, legacy.Backend.ResumeArgs...)
	}
	if output := strings.TrimSpace(legacy.Backend.Output); output != "" {
		dst.Backend.Output = output
	}
	if input := strings.TrimSpace(legacy.Backend.Input); input != "" {
		dst.Backend.Input = input
	}
	if legacy.Backend.MaxPromptArgChars > 0 {
		dst.Backend.MaxPromptArgChars = legacy.Backend.MaxPromptArgChars
	}
	if modelArg := strings.TrimSpace(legacy.Backend.ModelArg); modelArg != "" {
		dst.Backend.ModelArg = modelArg
	}
	if len(legacy.Backend.ModelAliases) > 0 {
		dst.Backend.ModelAliases = cloneStringMap(legacy.Backend.ModelAliases)
	}
	if sessionArg := strings.TrimSpace(legacy.Backend.SessionArg); sessionArg != "" {
		dst.Backend.SessionArg = sessionArg
	}
	if sessionMode := strings.TrimSpace(legacy.Backend.SessionMode); sessionMode != "" {
		dst.Backend.SessionMode = sessionMode
	}
	if promptArg := strings.TrimSpace(legacy.Backend.SystemPromptArg); promptArg != "" {
		dst.Backend.SystemPromptArg = promptArg
	}
	if promptMode := strings.TrimSpace(legacy.Backend.SystemPromptMode); promptMode != "" {
		dst.Backend.SystemPromptMode = promptMode
	}
	if promptWhen := strings.TrimSpace(legacy.Backend.SystemPromptWhen); promptWhen != "" {
		dst.Backend.SystemPromptWhen = promptWhen
	}
	if len(legacy.Backend.Env) > 0 {
		if dst.Backend.Env == nil {
			dst.Backend.Env = map[string]string{}
		}
		for key, value := range legacy.Backend.Env {
			dst.Backend.Env[key] = value
		}
	}
	if len(legacy.Backend.ClearEnv) > 0 {
		dst.Backend.ClearEnv = append([]string{}, legacy.Backend.ClearEnv...)
	}
	dst.Backend.Serialize = legacy.Backend.Serialize
}

func overlayClaudeCodeCLIConfig(dst *claudecode.ClaudeCodeConfig, appCfg *config.Config) {
	if dst == nil || appCfg == nil {
		return
	}
	backend := appCfg.ClaudeCodeCLI.Backend
	if command := strings.TrimSpace(backend.Command); command != "" {
		dst.Command = command
		dst.Backend.Command = command
	}
	if workspaceDir := strings.TrimSpace(backend.WorkspaceDir); workspaceDir != "" {
		dst.WorkspaceDir = workspaceDir
	}
	if model := strings.TrimSpace(backend.DefaultModel); model != "" {
		dst.DefaultModel = model
	}
	if timeout := parseDurationOrZero(backend.Timeout); timeout > 0 {
		dst.Timeout = timeout
	}
	if sessionTTL := parseDurationOrZero(backend.SessionTTL); sessionTTL > 0 {
		dst.SessionTTL = sessionTTL
	}
	if len(backend.Args) > 0 {
		dst.Backend.Args = append([]string{}, backend.Args...)
	}
	if len(backend.ResumeArgs) > 0 {
		dst.Backend.ResumeArgs = append([]string{}, backend.ResumeArgs...)
	}
	if output := strings.TrimSpace(backend.Output); output != "" {
		dst.Backend.Output = output
	}
	if resumeOutput := strings.TrimSpace(backend.ResumeOutput); resumeOutput != "" {
		dst.Backend.ResumeOutput = resumeOutput
	}
	if input := strings.TrimSpace(backend.Input); input != "" {
		dst.Backend.Input = input
	}
	if backend.MaxPromptArgChars > 0 {
		dst.Backend.MaxPromptArgChars = backend.MaxPromptArgChars
	}
	if modelArg := strings.TrimSpace(backend.ModelArg); modelArg != "" {
		dst.Backend.ModelArg = modelArg
	}
	if len(backend.ModelAliases) > 0 {
		dst.Backend.ModelAliases = cloneStringMap(backend.ModelAliases)
	}
	if sessionArg := strings.TrimSpace(backend.SessionArg); sessionArg != "" {
		dst.Backend.SessionArg = sessionArg
	}
	if sessionMode := strings.TrimSpace(backend.SessionMode); sessionMode != "" {
		dst.Backend.SessionMode = sessionMode
	}
	if promptArg := strings.TrimSpace(backend.SystemPromptArg); promptArg != "" {
		dst.Backend.SystemPromptArg = promptArg
	}
	if promptMode := strings.TrimSpace(backend.SystemPromptMode); promptMode != "" {
		dst.Backend.SystemPromptMode = promptMode
	}
	if promptWhen := strings.TrimSpace(backend.SystemPromptWhen); promptWhen != "" {
		dst.Backend.SystemPromptWhen = promptWhen
	}
	dst.Backend.Serialize = backend.Serialize
}

func parseDurationOrZero(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0
	}
	return d
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
