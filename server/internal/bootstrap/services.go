// Package bootstrap provides shared server initialization logic
package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a2ui"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/backup"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	ocrruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ocr"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/password"
	pdfextract "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pdf"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/worker"
)

// Services holds all initialized services
type Services struct {
	DB            *sql.DB
	DBConn        *database.SQLiteConn // Read-write separated connection
	Config        *config.Config
	Logger        *zap.Logger
	UserService   *user.Service
	UserRepo      *user.SQLiteRepository
	JWTService    *auth.JWTService
	APIKeyService *auth.APIKeyService
	MemoryStore   *memory.Store
	A2UIManager   *a2ui.Manager
	OCRService    *ocrruntime.TesseractService
	PDFService    *pdfextract.Service
	LLMRegistry   *llm.ProviderRegistry
	ToolRegistry  *tools.Registry
	SkillRegistry *skill.Registry
	MgmtTool      *tools.MgmtTool
	WorkerPool    *worker.Pool
	DataDir       string
}

// ResolveWorkspaceDir returns the shared runtime workspace directory.
// Explicit non-default workspace settings win; otherwise we keep the
// historical data-dir workspace to avoid surprising runtime behavior.
func ResolveWorkspaceDir(dataDir string, appCfg *config.Config) string {
	candidates := make([]string, 0, 3)
	if appCfg != nil {
		if v := normalizeExplicitWorkspaceDir(appCfg.ClaudeCodeCLI.Backend.WorkspaceDir); v != "" {
			candidates = append(candidates, v)
		}
		if v := normalizeExplicitWorkspaceDir(appCfg.ClaudeCode.WorkspaceDir); v != "" {
			candidates = append(candidates, v)
		}
	}
	if strings.TrimSpace(dataDir) != "" {
		candidates = append(candidates, filepath.Join(dataDir, "workspace"))
	}
	for _, candidate := range candidates {
		if normalized := normalizeWorkspacePath(candidate); normalized != "" {
			return normalized
		}
	}
	return "."
}

func normalizeExplicitWorkspaceDir(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if filepath.Clean(trimmed) == "." {
		return ""
	}
	return normalizeWorkspacePath(trimmed)
}

func normalizeWorkspacePath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if !filepath.IsAbs(trimmed) {
		if abs, err := filepath.Abs(trimmed); err == nil {
			trimmed = abs
		}
	}
	return filepath.Clean(trimmed)
}

func ResolveBuiltinToolAllowedPaths(appCfg *config.Config, dataDir string) []string {
	return resolveBuiltinToolAllowedPaths(appCfg, dataDir)
}

func resolveBuiltinToolAllowedPaths(appCfg *config.Config, dataDir string) []string {
	workspaceRoot := ResolveWorkspaceDir(dataDir, appCfg)

	paths := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)
	addPath := func(raw string) {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return
		}
		clean := normalizeWorkspacePath(trimmed)
		if clean == "" {
			return
		}
		if _, ok := seen[clean]; !ok {
			seen[clean] = struct{}{}
			paths = append(paths, clean)
		}
		if resolved, err := filepath.EvalSymlinks(clean); err == nil {
			resolved = filepath.Clean(resolved)
			if _, ok := seen[resolved]; !ok {
				seen[resolved] = struct{}{}
				paths = append(paths, resolved)
			}
		}
	}

	addPath(workspaceRoot)

	if shouldAllowTmpForWorkspace(paths) {
		addPath("/tmp")
		addPath("/private/tmp")
		if tmpDir := strings.TrimSpace(os.TempDir()); tmpDir != "" {
			addPath(tmpDir)
		}
	}

	return paths
}

func shouldAllowTmpForWorkspace(paths []string) bool {
	tempRoots := []string{"/tmp", "/private/tmp"}
	if tmpDir := strings.TrimSpace(os.TempDir()); tmpDir != "" {
		tempRoots = append(tempRoots, tmpDir)
		if resolved, err := filepath.EvalSymlinks(tmpDir); err == nil {
			tempRoots = append(tempRoots, resolved)
		}
	}
	for _, candidate := range paths {
		for _, root := range tempRoots {
			if pathWithinRoot(root, candidate) {
				return true
			}
		}
	}
	return false
}

func pathWithinRoot(root, target string) bool {
	root = filepath.Clean(strings.TrimSpace(root))
	target = filepath.Clean(strings.TrimSpace(target))
	if root == "" || target == "" {
		return false
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func openPrimaryDatabase(cfg *ServerConfig, logger *zap.Logger) (*database.SQLiteConn, error) {
	dbPath := filepath.Join(cfg.DataDir, "blue.db")

	open := func() (*database.SQLiteConn, error) {
		return database.OpenSQLite(dbPath, nil)
	}

	dbConn, err := open()
	if err == nil {
		return dbConn, nil
	}
	if !database.IsSQLiteCorruptionError(err) {
		return nil, err
	}

	if logger != nil {
		logger.Warn("Primary database open reported corruption; attempting startup auto-recovery",
			zap.String("db_path", dbPath),
			zap.Error(err),
		)
	}

	mgr, mgrErr := backup.NewManager(backup.Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          filepath.Join(cfg.DataDir, "backups"),
		SkillsPath:    filepath.Join(cfg.DataDir, "workspace", ".claude", "skills"),
	}, cfg.DataDir, cfg.DataDir)
	if mgrErr != nil {
		return nil, fmt.Errorf("open database: %w (init backup manager: %v)", err, mgrErr)
	}

	result, recoverErr := mgr.CheckAndAutoRecover(context.Background(), []string{dbPath})
	if recoverErr != nil {
		return nil, fmt.Errorf("open database: %w (startup auto-recovery failed: %v)", err, recoverErr)
	}

	if logger != nil && result != nil {
		if len(result.RepairedDatabases) > 0 {
			logger.Info("Primary database repaired during startup recovery",
				zap.Strings("repaired_databases", result.RepairedDatabases),
			)
		}
		if result.Recovered {
			logger.Info("Primary database restored from backup during startup recovery",
				zap.Strings("corrupted_databases", result.CorruptedDatabases),
				zap.String("backup_id", result.BackupID),
			)
		}
	}

	dbConn, retryErr := open()
	if retryErr != nil {
		return nil, fmt.Errorf("open database after startup auto-recovery: %w", retryErr)
	}
	return dbConn, nil
}

// InitServices initializes all core services
func InitServices(cfg *ServerConfig, appCfg *config.Config, logger *zap.Logger) (*Services, error) {
	trace := NewStartupTrace("bootstrap.init_services", logger)
	trace.Mark("enter")

	s := &Services{
		Config:  appCfg,
		Logger:  logger,
		DataDir: cfg.DataDir,
	}

	// Initialize database
	if err := os.MkdirAll(cfg.DataDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	trace.Mark("data_dir_ready")

	dbPath := filepath.Join(cfg.DataDir, "blue.db")
	dbConn, err := openPrimaryDatabase(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	s.DB = dbConn.Writer // backward compat: writer is the default
	s.DBConn = dbConn
	trace.Mark("db_opened", zap.String("db_path", dbPath))

	// User service
	userRepo, err := user.NewSQLiteRepository(s.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize user repository: %w", err)
	}
	s.UserRepo = userRepo
	trace.Mark("user_repo_ready")

	passwordHasher := password.NewHasher(&password.Config{
		Memory:      64 * 1024,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	})

	passwordPolicy := password.NewPolicy(&password.PolicyConfig{
		MinLength:        appCfg.Security.Password.MinLength,
		RequireUppercase: appCfg.Security.Password.RequireUppercase,
		RequireLowercase: appCfg.Security.Password.RequireLowercase,
		RequireNumber:    appCfg.Security.Password.RequireNumber,
		RequireSpecial:   appCfg.Security.Password.RequireSpecial,
	})

	s.UserService = user.NewService(userRepo, passwordHasher, passwordPolicy, nil)
	trace.Mark("user_service_ready")

	// JWT service
	s.JWTService = auth.NewJWTService(&auth.JWTConfig{
		Secret:            appCfg.Security.JWT.Secret,
		Expiration:        appCfg.Security.JWT.Expiration,
		RefreshExpiration: appCfg.Security.JWT.RefreshExpiration,
		Issuer:            appCfg.Security.JWT.Issuer,
	})
	trace.Mark("jwt_ready")

	// API Key service (shares main DB)
	s.APIKeyService, err = auth.NewAPIKeyServiceWithDB(s.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize API key service: %w", err)
	}
	trace.Mark("api_key_service_ready")

	// Memory store (shares main DB)
	s.MemoryStore, err = memory.NewStoreWithDB(s.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize memory store: %w", err)
	}
	trace.Mark("memory_store_ready")

	s.A2UIManager = a2ui.NewManager(logger)
	trace.Mark("a2ui_ready")
	s.OCRService = ocrruntime.NewTesseractService(logger, ocrruntime.Config{
		ModelDir:     filepath.Join(cfg.DataDir, "models", "tesseract"),
		AutoDownload: true,
		WorkerCount:  1,
	})
	s.PDFService = pdfextract.NewService(logger, s.OCRService)
	trace.Mark("ocr_pdf_ready")

	// LLM registry
	s.LLMRegistry = llm.NewProviderRegistry()
	registerLLMProviders(s.LLMRegistry, appCfg)
	trace.Mark("llm_registry_ready")

	// Tool registry (read, write, web_search + memory registered lazily)
	s.ToolRegistry = tools.NewRegistry()
	tools.RegisterBuiltinToolsWithConfig(
		s.ToolRegistry,
		buildWebSearchConfig(appCfg),
		buildWebFetchConfig(appCfg),
		resolveBuiltinToolAllowedPaths(appCfg, cfg.DataDir),
		0,
	)
	tools.AttachPDFServiceToWebTools(s.ToolRegistry, s.PDFService)
	tools.RegisterFactoryToolDefinitions(s.ToolRegistry)
	tools.RegisterAgentTools(s.ToolRegistry, appCfg)
	tools.RegisterSessionTools(s.ToolRegistry, sessionListAdapter{store: s.MemoryStore})
	tools.RegisterCanvasTools(s.ToolRegistry, s.A2UIManager)
	tools.RegisterPDFTool(s.ToolRegistry, s.PDFService)
	trace.Mark("tool_registry_ready")

	// Skill registry (for skill list UI and IPC — NOT bridged to LLM tools)
	s.SkillRegistry = skill.NewRegistry()
	builtin.RegisterAll(s.SkillRegistry)
	trace.Mark("skill_registry_ready")

	// Worker pool (shared by echo and echolib)
	s.WorkerPool = worker.NewPool(context.Background(), 10)
	trace.Mark("worker_pool_ready")
	trace.Mark("complete")

	return s, nil
}

func registerLLMProviders(registry *llm.ProviderRegistry, cfg *config.Config) {
	registry.Register(llm.NewOpenAIProvider(os.Getenv("OPENAI_API_KEY"), ""))

	claudeKey := os.Getenv("ANTHROPIC_API_KEY")
	claudeBaseURL := ""
	if cfg.ClaudeCode.APIKey != "" {
		claudeKey = cfg.ClaudeCode.APIKey
	}
	if cfg.ClaudeCode.BaseURL != "" {
		claudeBaseURL = cfg.ClaudeCode.BaseURL
	}
	registry.Register(llm.NewClaudeProvider(claudeKey, claudeBaseURL))

	ollamaURL := strings.TrimSpace(os.Getenv("OLLAMA_URL"))
	if ollamaURL != "" {
		registry.Register(llm.NewOllamaProvider(ollamaURL))
	}
	registry.Register(llm.NewCustomProvider(os.Getenv("CUSTOM_API_KEY"), os.Getenv("CUSTOM_API_URL")))
	registry.Register(llm.NewGrokProvider(os.Getenv("GROK_API_KEY"), ""))
	registry.Register(llm.NewQwenProvider(os.Getenv("QWEN_API_KEY"), ""))
	registry.Register(llm.NewSiliconFlowProvider(os.Getenv("SILICONFLOW_API_KEY"), ""))
}

// Close closes all services
func (s *Services) Close() {
	if s.APIKeyService != nil {
		s.APIKeyService.Close()
	}
	if s.PDFService != nil {
		_ = s.PDFService.Close()
	}
	if s.OCRService != nil {
		_ = s.OCRService.Close()
	}
	if s.DB != nil {
		s.DB.Close()
	}
}

// LoadProvidersFromPool loads providers from Provider Pool and registers them in LLM registry
func LoadProvidersFromPool(pool *providerpool.Pool, llmRegistry *llm.ProviderRegistry) {
	if pool == nil || llmRegistry == nil {
		return
	}

	providers := pool.Registry.ListEnabled()
	for _, provider := range providers {
		var apiKey, baseURL string
		if len(provider.APIKeys) > 0 {
			for _, key := range provider.APIKeys {
				if key.Enabled && key.Key != "" {
					apiKey = key.Key
					break
				}
			}
		}
		baseURL = provider.BaseURL

		var llmProvider llm.Provider
		switch provider.ID {
		case "openai":
			llmProvider = llm.NewOpenAIProvider(apiKey, baseURL)
		case "anthropic":
			llmProvider = llm.NewClaudeProvider(apiKey, baseURL)
		case "ollama":
			if baseURL == "" {
				baseURL = "http://localhost:11434"
			}
			llmProvider = llm.NewOllamaProvider(baseURL)
		case "custom":
			llmProvider = llm.NewCustomProvider(apiKey, baseURL)
		case "grok":
			llmProvider = llm.NewGrokProvider(apiKey, baseURL)
		case "qwen":
			llmProvider = llm.NewQwenProvider(apiKey, baseURL)
		case "venice":
			llmProvider = llm.NewVeniceProvider(apiKey, baseURL)
		case "bedrock":
			llmProvider = llm.NewBedrockProvider(apiKey, baseURL)
		case "siliconflow":
			llmProvider = llm.NewSiliconFlowProvider(apiKey, baseURL)
		case "glm":
			llmProvider = llm.NewGLMProvider(apiKey, baseURL)
		}
		if llmProvider != nil {
			llmRegistry.Register(llmProvider)
		}
	}
}

type sessionListAdapter struct {
	store *memory.Store
}

func sessionScopedUserID(ctx context.Context, requestedUserID string) string {
	if ctxUserID := strings.TrimSpace(tools.GetUserID(ctx)); ctxUserID != "" {
		return ctxUserID
	}
	return strings.TrimSpace(requestedUserID)
}

func (a sessionListAdapter) ListSessions(ctx context.Context, limit, offset int, userID string) ([]tools.SessionSummary, error) {
	if a.store == nil {
		return nil, nil
	}
	userID = sessionScopedUserID(ctx, userID)
	var (
		convs []memory.Conversation
		err   error
	)
	if userID != "" {
		convs, err = a.store.ListConversations(ctx, limit, offset, userID)
	} else {
		convs, err = a.store.ListConversations(ctx, limit, offset)
	}
	if err != nil {
		return nil, err
	}
	out := make([]tools.SessionSummary, 0, len(convs))
	for _, conv := range convs {
		out = append(out, tools.SessionSummary{
			ID:        conv.ID,
			Title:     conv.Title,
			UserID:    conv.UserID,
			Pinned:    conv.Pinned,
			CreatedAt: conv.CreatedAt,
			UpdatedAt: conv.UpdatedAt,
		})
	}
	return out, nil
}

func (a sessionListAdapter) GetSession(ctx context.Context, sessionID string) (*tools.SessionSummary, error) {
	if a.store == nil {
		return nil, nil
	}
	conv, err := a.store.GetConversation(ctx, sessionID, sessionScopedUserID(ctx, ""))
	if err != nil {
		if err == memory.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	if conv == nil {
		return nil, nil
	}
	return &tools.SessionSummary{
		ID:        conv.ID,
		Title:     conv.Title,
		UserID:    conv.UserID,
		Pinned:    conv.Pinned,
		CreatedAt: conv.CreatedAt,
		UpdatedAt: conv.UpdatedAt,
	}, nil
}

func (a sessionListAdapter) GetSessionMessages(ctx context.Context, sessionID string, limit, offset int) ([]tools.SessionMessage, error) {
	if a.store == nil {
		return nil, nil
	}
	messages, err := a.store.GetMessages(ctx, sessionID, limit, offset, sessionScopedUserID(ctx, ""))
	if err != nil {
		return nil, err
	}
	out := make([]tools.SessionMessage, 0, len(messages))
	for _, msg := range messages {
		out = append(out, tools.SessionMessage{
			ID:         msg.ID,
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
			ToolName:   msg.ToolName,
			Provider:   msg.Provider,
			Model:      msg.Model,
			CreatedAt:  msg.CreatedAt,
		})
	}
	return out, nil
}

func (a sessionListAdapter) CreateSession(ctx context.Context, title, userID string, pinned bool) (*tools.SessionSummary, error) {
	if a.store == nil {
		return nil, nil
	}
	userID = sessionScopedUserID(ctx, userID)
	var (
		conv *memory.Conversation
		err  error
	)
	if userID != "" {
		conv, err = a.store.CreateConversation(ctx, title, userID)
	} else {
		conv, err = a.store.CreateConversation(ctx, title)
	}
	if err != nil {
		return nil, err
	}
	if pinned {
		if err := a.store.PinConversation(ctx, conv.ID, userID); err != nil {
			return nil, err
		}
		conv, err = a.store.GetConversation(ctx, conv.ID, userID)
		if err != nil {
			return nil, err
		}
	}
	return &tools.SessionSummary{
		ID:        conv.ID,
		Title:     conv.Title,
		UserID:    conv.UserID,
		Pinned:    conv.Pinned,
		CreatedAt: conv.CreatedAt,
		UpdatedAt: conv.UpdatedAt,
	}, nil
}

func (a sessionListAdapter) AppendSessionMessage(ctx context.Context, sessionID string, msg tools.SessionMessage) (*tools.SessionMessage, error) {
	if a.store == nil {
		return nil, nil
	}
	userID := sessionScopedUserID(ctx, "")
	created, err := a.store.AddMessage(ctx, sessionID, memory.Message{
		Role:       msg.Role,
		Content:    msg.Content,
		ToolCallID: msg.ToolCallID,
		ToolName:   msg.ToolName,
		Provider:   msg.Provider,
		Model:      msg.Model,
	}, userID)
	if err != nil {
		return nil, err
	}
	return &tools.SessionMessage{
		ID:         created.ID,
		Role:       created.Role,
		Content:    created.Content,
		ToolCallID: created.ToolCallID,
		ToolName:   created.ToolName,
		Provider:   created.Provider,
		Model:      created.Model,
		CreatedAt:  created.CreatedAt,
	}, nil
}
