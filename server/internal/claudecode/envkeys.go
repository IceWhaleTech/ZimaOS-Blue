package claudecode

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/labstack/echo/v4"
)

// APIKeyInfo represents information about a detected API key.
type APIKeyInfo struct {
	Provider    string `json:"provider"`              // Provider name (e.g., "anthropic", "openai", "google")
	EnvVar      string `json:"env_var,omitempty"`     // Environment variable name
	Source      string `json:"source"`                // "env", "ide-config", "file"
	IDEName     string `json:"ide_name,omitempty"`    // IDE name if source is "ide-config"
	Configured  bool   `json:"configured"`            // Whether the key is set
	MaskedValue string `json:"masked_value,omitempty"` // Masked key value (e.g., "sk-...abc")
}

// DetectedKeysResponse is the response for the detected API keys endpoint.
type DetectedKeysResponse struct {
	Keys []APIKeyInfo `json:"keys"`
}

// Common environment variable names for LLM providers
var llmEnvVars = map[string]string{
	// Anthropic/Claude
	"ANTHROPIC_API_KEY": "anthropic",
	"CLAUDE_API_KEY":    "anthropic",

	// OpenAI
	"OPENAI_API_KEY": "openai",

	// Google/Gemini
	"GOOGLE_API_KEY":       "google",
	"GEMINI_API_KEY":       "google",
	"GOOGLE_GENERATIVE_AI_API_KEY": "google",

	// Azure OpenAI
	"AZURE_OPENAI_API_KEY": "azure-openai",
	"AZURE_OPENAI_KEY":     "azure-openai",

	// Cohere
	"COHERE_API_KEY": "cohere",
	"CO_API_KEY":     "cohere",

	// Mistral
	"MISTRAL_API_KEY": "mistral",

	// Groq
	"GROQ_API_KEY": "groq",

	// Together AI
	"TOGETHER_API_KEY":    "together",
	"TOGETHERAI_API_KEY":  "together",

	// Perplexity
	"PERPLEXITY_API_KEY": "perplexity",
	"PPLX_API_KEY":       "perplexity",

	// Fireworks
	"FIREWORKS_API_KEY": "fireworks",

	// Replicate
	"REPLICATE_API_TOKEN": "replicate",

	// Hugging Face
	"HUGGINGFACE_API_KEY": "huggingface",
	"HF_TOKEN":            "huggingface",
	"HF_API_KEY":          "huggingface",

	// DeepSeek
	"DEEPSEEK_API_KEY": "deepseek",

	// Moonshot (Kimi)
	"MOONSHOT_API_KEY": "moonshot",

	// Zhipu (GLM)
	"ZHIPU_API_KEY": "zhipu",

	// Baichuan
	"BAICHUAN_API_KEY": "baichuan",

	// Qwen (Alibaba)
	"DASHSCOPE_API_KEY": "qwen",
	"QWEN_API_KEY":      "qwen",

	// Doubao (ByteDance)
	"DOUBAO_API_KEY": "doubao",
	"ARK_API_KEY":    "doubao",

	// Antigravity
	"ANTIGRAVITY_ACCESS_TOKEN": "antigravity",
	"ANTIGRAVITY_TOKEN":        "antigravity",
}

// EnvKeysHandler handles API key detection endpoints.
type EnvKeysHandler struct{}

// NewEnvKeysHandler creates a new EnvKeysHandler.
func NewEnvKeysHandler() *EnvKeysHandler {
	return &EnvKeysHandler{}
}

// RegisterRoutes registers the environment keys API routes.
func (h *EnvKeysHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/env-keys", h.GetDetectedKeys)
}

// GetDetectedKeys returns all detected API keys from environment and IDE configs.
// GET /api/v1/claudecode/env-keys
func (h *EnvKeysHandler) GetDetectedKeys(c echo.Context) error {
	keys := []APIKeyInfo{}
	seen := make(map[string]bool) // Track seen provider+source combinations

	// 1. Check environment variables
	for envVar, provider := range llmEnvVars {
		value := os.Getenv(envVar)
		if value != "" {
			// Deduplicate by provider+env_var
			key := provider + ":" + envVar
			if !seen[key] {
				seen[key] = true
				keys = append(keys, APIKeyInfo{
					Provider:    provider,
					EnvVar:      envVar,
					Source:      "env",
					Configured:  true,
					MaskedValue: maskAPIKey(value),
				})
			}
		}
	}

	// 2. Check IDE config files for Antigravity token
	antigravityKeys := h.findAntigravityTokens()
	for _, k := range antigravityKeys {
		key := k.Provider + ":ide:" + k.IDEName
		if !seen[key] {
			seen[key] = true
			keys = append(keys, k)
		}
	}

	// 3. Check Claude Code CLI config for API key
	claudeKeys := h.findClaudeCodeConfig()
	for _, k := range claudeKeys {
		key := k.Provider + ":file"
		if !seen[key] {
			seen[key] = true
			keys = append(keys, k)
		}
	}

	return c.JSON(http.StatusOK, DetectedKeysResponse{Keys: keys})
}

// maskAPIKey masks an API key for display (shows first 4 and last 4 characters).
func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

// findAntigravityTokens searches for Antigravity access tokens in IDE config files.
func (h *EnvKeysHandler) findAntigravityTokens() []APIKeyInfo {
	keys := []APIKeyInfo{}
	home, err := os.UserHomeDir()
	if err != nil {
		return keys
	}

	// IDE config locations for Antigravity
	type ideConfig struct {
		name     string
		paths    []string
		jsonPath string // JSON path to token (e.g., "antigravity.accessToken")
	}

	ideConfigs := []ideConfig{
		// VS Code settings
		{
			name:     "VS Code",
			paths:    []string{".vscode/settings.json", ".config/Code/User/settings.json"},
			jsonPath: "antigravity.accessToken",
		},
		// Cursor settings
		{
			name:     "Cursor",
			paths:    []string{".cursor/settings.json", ".config/Cursor/User/settings.json"},
			jsonPath: "antigravity.accessToken",
		},
		// Antigravity specific config
		{
			name:     "Antigravity",
			paths:    []string{".antigravity/config.json", ".config/antigravity/config.json"},
			jsonPath: "accessToken",
		},
	}

	// Add platform-specific paths
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			ideConfigs = append(ideConfigs,
				ideConfig{
					name:     "VS Code (Windows)",
					paths:    []string{filepath.Join(appData, "Code", "User", "settings.json")},
					jsonPath: "antigravity.accessToken",
				},
				ideConfig{
					name:     "Cursor (Windows)",
					paths:    []string{filepath.Join(appData, "Cursor", "User", "settings.json")},
					jsonPath: "antigravity.accessToken",
				},
			)
		}
	} else if runtime.GOOS == "darwin" {
		ideConfigs = append(ideConfigs,
			ideConfig{
				name:     "VS Code (macOS)",
				paths:    []string{"Library/Application Support/Code/User/settings.json"},
				jsonPath: "antigravity.accessToken",
			},
			ideConfig{
				name:     "Cursor (macOS)",
				paths:    []string{"Library/Application Support/Cursor/User/settings.json"},
				jsonPath: "antigravity.accessToken",
			},
			ideConfig{
				name:     "Antigravity (macOS)",
				paths:    []string{"Library/Application Support/Antigravity/User/settings.json"},
				jsonPath: "antigravity.accessToken",
			},
		)
	}

	for _, cfg := range ideConfigs {
		for _, path := range cfg.paths {
			var fullPath string
			if filepath.IsAbs(path) {
				fullPath = path
			} else {
				fullPath = filepath.Join(home, path)
			}

			token := h.extractTokenFromJSON(fullPath, cfg.jsonPath)
			if token != "" {
				keys = append(keys, APIKeyInfo{
					Provider:    "antigravity",
					Source:      "ide-config",
					IDEName:     cfg.name,
					Configured:  true,
					MaskedValue: maskAPIKey(token),
				})
				break // Found token for this IDE, skip other paths
			}
		}
	}

	return keys
}

// findClaudeCodeConfig searches for Claude Code CLI config files.
func (h *EnvKeysHandler) findClaudeCodeConfig() []APIKeyInfo {
	keys := []APIKeyInfo{}
	home, err := os.UserHomeDir()
	if err != nil {
		return keys
	}

	// Claude Code CLI config locations
	configPaths := []string{
		filepath.Join(home, ".claude", "config.json"),
		filepath.Join(home, ".config", "claude", "config.json"),
	}

	for _, configPath := range configPaths {
		if _, err := os.Stat(configPath); err == nil {
			// Check for API key in config
			apiKey := h.extractTokenFromJSON(configPath, "apiKey")
			if apiKey != "" {
				keys = append(keys, APIKeyInfo{
					Provider:    "anthropic",
					Source:      "file",
					Configured:  true,
					MaskedValue: maskAPIKey(apiKey),
				})
				break
			}
		}
	}

	return keys
}

// extractTokenFromJSON extracts a token from a JSON file using a dot-separated path.
func (h *EnvKeysHandler) extractTokenFromJSON(filePath, jsonPath string) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return ""
	}

	// Navigate the JSON path
	parts := strings.Split(jsonPath, ".")
	current := interface{}(config)

	for _, part := range parts {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[part]
		} else {
			return ""
		}
	}

	if str, ok := current.(string); ok {
		return str
	}

	return ""
}
