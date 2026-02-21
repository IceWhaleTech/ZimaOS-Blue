// Package bootstrap provides shared server initialization logic
package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/password"
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
	LLMRegistry   *llm.ProviderRegistry
	ToolRegistry  *tools.Registry
	SkillRegistry *skill.Registry
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

	// LLM registry
	s.LLMRegistry = llm.NewProviderRegistry()
	registerLLMProviders(s.LLMRegistry, appCfg)

	// Tool registry
	s.ToolRegistry = tools.NewRegistry()
	tools.RegisterBuiltinTools(s.ToolRegistry)

	// Skill registry
	s.SkillRegistry = skill.NewRegistry()
	builtin.RegisterAll(s.SkillRegistry)

	// Bridge skills that should be callable by the LLM into the tool registry.
	if uiReviewer := s.SkillRegistry.Get("ui_reviewer"); uiReviewer != nil {
		tools.RegisterSkill(s.ToolRegistry, uiReviewer)
	}

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

	ollamaURL := os.Getenv("OLLAMA_URL")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}
	registry.Register(llm.NewOllamaProvider(ollamaURL))
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
