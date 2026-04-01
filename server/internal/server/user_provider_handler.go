package server

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
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
	readDB   *sql.DB
	registry *llm.ProviderRegistry
}

// NewUserProviderHandler creates a new user provider handler and runs migrations.
func NewUserProviderHandler(db *sql.DB, registry *llm.ProviderRegistry) (*UserProviderHandler, error) {
	return NewUserProviderHandlerWithReadDB(db, db, registry)
}

// NewUserProviderHandlerWithReadDB creates a new user provider handler with
// separate write and read database handles.
func NewUserProviderHandlerWithReadDB(writeDB, readDB *sql.DB, registry *llm.ProviderRegistry) (*UserProviderHandler, error) {
	if readDB == nil {
		readDB = writeDB
	}
	h := &UserProviderHandler{db: writeDB, readDB: readDB, registry: registry}
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

type userProviderConfigRow struct {
	ID           string `json:"id" zorm:"id"`
	UserID       string `json:"user_id" zorm:"user_id"`
	ProviderName string `json:"provider_name" zorm:"provider_name"`
	APIKey       string `json:"api_key" zorm:"api_key"`
	BaseURL      string `json:"base_url" zorm:"base_url"`
	Enabled      bool   `json:"enabled" zorm:"enabled"`
}

func (h *UserProviderHandler) writeTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, h.db, "user_provider_configs")
}

func (h *UserProviderHandler) readTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, h.readDB, "user_provider_configs")
}

func boolToSQLiteInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func getUserIDStrict(c echo.Context) string {
	if claims := auth.GetUserFromContext(c); claims != nil {
		return claims.UserID
	}
	return ""
}

// List returns all provider configs for the current user, merged with available providers.
func (h *UserProviderHandler) List(c echo.Context) error {
	userID := getUserIDStrict(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	var rows []userProviderConfigRow
	_, err := h.readTable(c.Request().Context()).Select(
		&rows,
		z.Fields("id", "provider_name", "api_key", "base_url", "enabled"),
		z.Where(z.Eq("user_id", userID)),
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list configs")
	}
	userConfigs := make(map[string]*UserProviderConfigResponse)
	for _, row := range rows {
		userConfigs[row.ProviderName] = &UserProviderConfigResponse{
			ProviderName: row.ProviderName,
			HasAPIKey:    row.APIKey != "",
			APIKey:       maskAPIKey(row.APIKey),
			BaseURL:      row.BaseURL,
			Enabled:      row.Enabled,
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
	userID := getUserIDStrict(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	name := c.Param("name")

	var rows []userProviderConfigRow
	_, err := h.readTable(c.Request().Context()).Select(
		&rows,
		z.Fields("id", "provider_name", "api_key", "base_url", "enabled"),
		z.Where(z.Eq("user_id", userID), z.Eq("provider_name", name)),
		z.Limit(1),
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get config")
	}
	if len(rows) == 0 {
		return c.JSON(http.StatusOK, UserProviderConfigResponse{
			ProviderName: name,
			Enabled:      true,
		})
	}
	cfg := rows[0]

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
	userID := getUserIDStrict(c)
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
	var curRows []userProviderConfigRow
	_, err := h.readTable(ctx).Select(
		&curRows,
		z.Fields("api_key", "base_url", "enabled"),
		z.Where(z.Eq("user_id", userID), z.Eq("provider_name", name)),
		z.Limit(1),
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to check config")
	}

	apiKey, baseURL, enabled := "", "", true
	if len(curRows) > 0 {
		apiKey = curRows[0].APIKey
		baseURL = curRows[0].BaseURL
		enabled = curRows[0].Enabled
	}
	if len(curRows) == 0 {
		enabled = true // default for new records
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

	_, err = h.writeTable(ctx).Insert(map[string]interface{}{
		"id":            uuid.New().String(),
		"user_id":       userID,
		"provider_name": name,
		"api_key":       apiKey,
		"base_url":      baseURL,
		"enabled":       boolToSQLiteInt(enabled),
		"created_at":    now,
		"updated_at":    now,
	}, z.OnConflictDoUpdateSet(
		[]string{"user_id", "provider_name"},
		[]string{"api_key", "base_url", "enabled", "updated_at"},
	))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to save config")
	}

	return h.Get(c)
}

// Delete removes a provider config for the current user.
func (h *UserProviderHandler) Delete(c echo.Context) error {
	userID := getUserIDStrict(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	name := c.Param("name")

	_, err := h.writeTable(c.Request().Context()).Delete(
		z.Where(z.Eq("user_id", userID), z.Eq("provider_name", name)),
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete config")
	}

	return c.NoContent(http.StatusNoContent)
}

// Test tests a provider connection using the user's API key.
func (h *UserProviderHandler) Test(c echo.Context) error {
	userID := getUserIDStrict(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	name := c.Param("name")

	var rows []userProviderConfigRow
	_, err := h.readTable(c.Request().Context()).Select(
		&rows,
		z.Fields("api_key", "base_url"),
		z.Where(z.Eq("user_id", userID), z.Eq("provider_name", name)),
		z.Limit(1),
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get config")
	}
	if len(rows) == 0 || rows[0].APIKey == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":    false,
			"messageKey": "apiKeyRequired",
		})
	}
	apiKey, baseURL := rows[0].APIKey, rows[0].BaseURL

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
	var rows []userProviderConfigRow
	_, err := h.readTable(ctx.Request().Context()).Select(
		&rows,
		z.Fields("api_key", "base_url", "enabled"),
		z.Where(z.Eq("user_id", userID), z.Eq("provider_name", providerName), z.Eq("enabled", true)),
		z.Limit(1),
	)
	if err != nil || len(rows) == 0 || rows[0].APIKey == "" {
		return "", "", false
	}
	return rows[0].APIKey, rows[0].BaseURL, true
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
