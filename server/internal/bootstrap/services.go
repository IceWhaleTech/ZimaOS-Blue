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

// InitServices initializes all core services
func InitServices(cfg *ServerConfig, appCfg *config.Config, logger *zap.Logger) (*Services, error) {
	s := &Services{
		Config:  appCfg,
		Logger:  logger,
		DataDir: cfg.DataDir,
	}

	// Initialize database
	if err := os.MkdirAll(cfg.DataDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(cfg.DataDir, "blue.db")
	dbConn, err := database.OpenSQLite(dbPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	s.DB = dbConn.Writer // backward compat: writer is the default
	s.DBConn = dbConn

	// User service
	userRepo, err := user.NewSQLiteRepository(s.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize user repository: %w", err)
	}
	s.UserRepo = userRepo

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

	// JWT service
	s.JWTService = auth.NewJWTService(&auth.JWTConfig{
		Secret:            appCfg.Security.JWT.Secret,
		Expiration:        appCfg.Security.JWT.Expiration,
		RefreshExpiration: appCfg.Security.JWT.RefreshExpiration,
		Issuer:            appCfg.Security.JWT.Issuer,
	})

	// API Key service (shares main DB)
	s.APIKeyService, err = auth.NewAPIKeyServiceWithDB(s.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize API key service: %w", err)
	}

	// Memory store (shares main DB)
	s.MemoryStore, err = memory.NewStoreWithDB(s.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize memory store: %w", err)
	}

	s.A2UIManager = a2ui.NewManager(logger)
	s.OCRService = ocrruntime.NewTesseractService(logger, ocrruntime.Config{
		ModelDir:     filepath.Join(cfg.DataDir, "models", "tesseract"),
		AutoDownload: true,
		WorkerCount:  1,
	})
	s.PDFService = pdfextract.NewService(logger, s.OCRService)

	// LLM registry
	s.LLMRegistry = llm.NewProviderRegistry()
	registerLLMProviders(s.LLMRegistry, appCfg)

	// Tool registry (read, write, web_search + memory registered lazily)
	s.ToolRegistry = tools.NewRegistry()
	tools.RegisterBuiltinToolsWithConfig(s.ToolRegistry, buildWebSearchConfig(appCfg), buildWebFetchConfig(appCfg), nil, 0)
	tools.RegisterFactoryToolDefinitions(s.ToolRegistry)
	tools.RegisterAgentTools(s.ToolRegistry, appCfg)
	tools.RegisterSessionTools(s.ToolRegistry, sessionListAdapter{store: s.MemoryStore})
	tools.RegisterCanvasTools(s.ToolRegistry, s.A2UIManager)
	tools.RegisterPDFTool(s.ToolRegistry, s.PDFService)

	// Skill registry (for skill list UI and IPC — NOT bridged to LLM tools)
	s.SkillRegistry = skill.NewRegistry()
	builtin.RegisterAll(s.SkillRegistry)

	// Worker pool (shared by echo and echolib)
	s.WorkerPool = worker.NewPool(context.Background(), 10)

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

func (a sessionListAdapter) ListSessions(ctx context.Context, limit, offset int, userID string) ([]tools.SessionSummary, error) {
	if a.store == nil {
		return nil, nil
	}
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
	conv, err := a.store.GetConversation(ctx, sessionID)
	if err != nil {
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
	messages, err := a.store.GetMessages(ctx, sessionID, limit, offset)
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
		if err := a.store.PinConversation(ctx, conv.ID); err != nil {
			return nil, err
		}
		conv, err = a.store.GetConversation(ctx, conv.ID)
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
	created, err := a.store.AddMessage(ctx, sessionID, memory.Message{
		Role:       msg.Role,
		Content:    msg.Content,
		ToolCallID: msg.ToolCallID,
		ToolName:   msg.ToolName,
		Provider:   msg.Provider,
		Model:      msg.Model,
	})
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
