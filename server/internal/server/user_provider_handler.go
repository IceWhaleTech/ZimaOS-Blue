package server

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// UserProviderConfig represents a user's provider configuration.
type UserProviderConfig struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	ProviderName string    `json:"provider_name"`
	APIKey       string    `json:"api_key,omitempty"`
	BaseURL      string    `json:"base_url,omitempty"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserProviderHandler handles per-user provider configuration.
type UserProviderHandler struct {
	db       *sql.DB
	registry *llm.ProviderRegistry
}

// NewUserProviderHandler creates a new user provider handler and runs migrations.
func NewUserProviderHandler(db *sql.DB, registry *llm.ProviderRegistry) (*UserProviderHandler, error) {
	h := &UserProviderHandler{db: db, registry: registry}
	if err := h.migrate(); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *UserProviderHandler) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS user_provider_configs (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		provider_name TEXT NOT NULL,
		api_key TEXT NOT NULL DEFAULT '',
		base_url TEXT NOT NULL DEFAULT '',
		enabled BOOLEAN NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		UNIQUE(user_id, provider_name)
	);
	CREATE INDEX IF NOT EXISTS idx_user_provider_user_id ON user_provider_configs(user_id);
	`
	_, err := h.db.Exec(schema)
	return err
}

// RegisterRoutes registers the user provider API routes.
func (h *UserProviderHandler) RegisterRoutes(g *echo.Group) {
	g.GET("", h.List)
	g.GET("/:name", h.Get)
	g.PUT("/:name", h.Upsert)
	g.DELETE("/:name", h.Delete)
	g.POST("/:name/test", h.Test)
}

func getUserIDFromContext(c echo.Context) string {
	if claims := auth.GetUserFromContext(c); claims != nil {
		return claims.UserID
	}
	return ""
}

// List returns all provider configs for the current user, merged with available providers.
func (h *UserProviderHandler) List(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	rows, err := h.db.QueryContext(c.Request().Context(),
		"SELECT id, provider_name, api_key, base_url, enabled, created_at, updated_at FROM user_provider_configs WHERE user_id = ?",
		userID,
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list configs")
	}
	defer rows.Close()

	userConfigs := make(map[string]*UserProviderConfigResponse)
	for rows.Next() {
		var cfg UserProviderConfig
		if err := rows.Scan(&cfg.ID, &cfg.ProviderName, &cfg.APIKey, &cfg.BaseURL, &cfg.Enabled, &cfg.CreatedAt, &cfg.UpdatedAt); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to scan config")
		}
		userConfigs[cfg.ProviderName] = &UserProviderConfigResponse{
			ProviderName: cfg.ProviderName,
			HasAPIKey:    cfg.APIKey != "",
			APIKey:       maskAPIKey(cfg.APIKey),
			BaseURL:      cfg.BaseURL,
			Enabled:      cfg.Enabled,
			Configured:   true,
		}
	}

	// Merge with all available providers
	providerNames := h.registry.List()
	result := make([]UserProviderConfigResponse, 0, len(providerNames))
	for _, name := range providerNames {
		if cfg, ok := userConfigs[name]; ok {
			result = append(result, *cfg)
		} else {
			result = append(result, UserProviderConfigResponse{
				ProviderName: name,
				Enabled:      true,
			})
		}
	}

	return c.JSON(http.StatusOK, result)
}

// UserProviderConfigResponse is the API response for user provider config.
type UserProviderConfigResponse struct {
	ProviderName string `json:"provider_name"`
	HasAPIKey    bool   `json:"has_api_key"`
	APIKey       string `json:"api_key,omitempty"`
	BaseURL      string `json:"base_url,omitempty"`
	Enabled      bool   `json:"enabled"`
	Configured   bool   `json:"configured"`
}

// Get returns a single provider config for the current user.
func (h *UserProviderHandler) Get(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	name := c.Param("name")

	var cfg UserProviderConfig
	err := h.db.QueryRowContext(c.Request().Context(),
		"SELECT id, provider_name, api_key, base_url, enabled FROM user_provider_configs WHERE user_id = ? AND provider_name = ?",
		userID, name,
	).Scan(&cfg.ID, &cfg.ProviderName, &cfg.APIKey, &cfg.BaseURL, &cfg.Enabled)

	if err == sql.ErrNoRows {
		return c.JSON(http.StatusOK, UserProviderConfigResponse{
			ProviderName: name,
			Enabled:      true,
		})
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get config")
	}

	return c.JSON(http.StatusOK, UserProviderConfigResponse{
		ProviderName: cfg.ProviderName,
		HasAPIKey:    cfg.APIKey != "",
		APIKey:       maskAPIKey(cfg.APIKey),
		BaseURL:      cfg.BaseURL,
		Enabled:      cfg.Enabled,
		Configured:   true,
	})
}

// UpsertRequest is the request body for creating/updating a user provider config.
type UpsertRequest struct {
	APIKey  *string `json:"api_key,omitempty"`
	BaseURL *string `json:"base_url,omitempty"`
	Enabled *bool   `json:"enabled,omitempty"`
}

// Upsert creates or updates a provider config for the current user.
func (h *UserProviderHandler) Upsert(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	name := c.Param("name")

	var req UpsertRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	now := timeutil.NowTime()
	ctx := c.Request().Context()

	// Read current values for merge
	var curAPIKey, curBaseURL string
	var curEnabled bool
	err := h.db.QueryRowContext(ctx,
		"SELECT api_key, base_url, enabled FROM user_provider_configs WHERE user_id = ? AND provider_name = ?",
		userID, name,
	).Scan(&curAPIKey, &curBaseURL, &curEnabled)

	apiKey, baseURL, enabled := curAPIKey, curBaseURL, curEnabled
	if err == sql.ErrNoRows {
		enabled = true // default for new records
	} else if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to check config")
	}

	// Apply provided fields
	if req.APIKey != nil {
		apiKey = *req.APIKey
	}
	if req.BaseURL != nil {
		baseURL = *req.BaseURL
	}
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	_, err = h.db.ExecContext(ctx,
		`INSERT INTO user_provider_configs (id, user_id, provider_name, api_key, base_url, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(user_id, provider_name) DO UPDATE SET api_key = ?, base_url = ?, enabled = ?, updated_at = ?`,
		uuid.New().String(), userID, name, apiKey, baseURL, enabled, now, now,
		apiKey, baseURL, enabled, now,
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to save config")
	}

	return h.Get(c)
}

// Delete removes a provider config for the current user.
func (h *UserProviderHandler) Delete(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	name := c.Param("name")

	_, err := h.db.ExecContext(c.Request().Context(),
		"DELETE FROM user_provider_configs WHERE user_id = ? AND provider_name = ?",
		userID, name,
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete config")
	}

	return c.NoContent(http.StatusNoContent)
}

// Test tests a provider connection using the user's API key.
func (h *UserProviderHandler) Test(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	name := c.Param("name")

	var apiKey, baseURL string
	err := h.db.QueryRowContext(c.Request().Context(),
		"SELECT api_key, base_url FROM user_provider_configs WHERE user_id = ? AND provider_name = ?",
		userID, name,
	).Scan(&apiKey, &baseURL)

	if err == sql.ErrNoRows || apiKey == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "apiKeyRequired",
		})
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get config")
	}

	// Create a temporary provider to test
	provider := createProviderInstance(name, apiKey, baseURL)
	if provider == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "providerNotSupported",
		})
	}

	models := provider.Models()
	if len(models) == 0 {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "noModelsAvailable",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":    true,
		"messageKey": "testSuccess",
		"models":     models,
	})
}

// GetUserProviderKey returns the user's API key for a provider, or empty if not configured.
func (h *UserProviderHandler) GetUserProviderKey(ctx echo.Context, userID, providerName string) (apiKey, baseURL string, found bool) {
	var key, url string
	var enabled bool
	err := h.db.QueryRowContext(ctx.Request().Context(),
		"SELECT api_key, base_url, enabled FROM user_provider_configs WHERE user_id = ? AND provider_name = ? AND enabled = 1",
		userID, providerName,
	).Scan(&key, &url, &enabled)
	if err != nil || key == "" {
		return "", "", false
	}
	return key, url, true
}

// createProviderInstance creates a temporary LLM provider instance for testing.
func createProviderInstance(name, apiKey, baseURL string) llm.Provider {
	switch name {
	case "claude":
		if baseURL == "" {
			baseURL = "https://api.anthropic.com"
		}
		return llm.NewClaudeProvider(apiKey, baseURL)
	case "openai":
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		return llm.NewOpenAIProvider(apiKey, baseURL)
	case "grok":
		if baseURL == "" {
			baseURL = "https://api.x.ai"
		}
		return llm.NewGrokProvider(apiKey, baseURL)
	case "qwen":
		if baseURL == "" {
			baseURL = "https://dashscope.aliyuncs.com/compatible-mode"
		}
		return llm.NewQwenProvider(apiKey, baseURL)
	case "siliconflow":
		if baseURL == "" {
			baseURL = "https://api.siliconflow.cn/v1"
		}
		return llm.NewSiliconFlowProvider(apiKey, baseURL)
	case "custom":
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		return llm.NewCustomProvider(apiKey, baseURL)
	case "ollama":
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return llm.NewOllamaProvider(baseURL)
	default:
		return nil
	}
}
