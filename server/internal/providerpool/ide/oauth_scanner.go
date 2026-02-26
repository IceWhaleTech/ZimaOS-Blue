package ide

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool/oauth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// ScanOAuthToken scans for an OAuth token from a locally installed IDE.
func ScanOAuthToken(ideType IDEType) (*oauth.Token, error) {
	switch ideType {
	case IDETypeAntigravity:
		return scanAntigravityOAuth()
	case IDETypeCopilot:
		return scanCopilotOAuth()
	case IDETypeCodex:
		return scanCodexOAuth()
	default:
		return scanGeminiCLIOAuth()
	}
}

// ScanAllOAuthTokens scans all supported IDEs for OAuth tokens.
func ScanAllOAuthTokens() []*OAuthScanResult {
	var results []*OAuthScanResult

	for _, ideType := range []IDEType{IDETypeAntigravity, IDETypeCopilot, IDETypeCodex} {
		token, err := ScanOAuthToken(ideType)
		result := &OAuthScanResult{
			IDEType: ideType,
			IDEName: getIDEName(ideType),
		}
		if err != nil {
			result.Error = err.Error()
		} else {
			result.Found = true
			result.Email = token.Email
			result.ProviderType = token.ProviderType
		}
		results = append(results, result)
	}

	// Also try Gemini CLI (not tied to a specific IDE)
	token, err := scanGeminiCLIOAuth()
	result := &OAuthScanResult{
		IDEType: "gemini-cli",
		IDEName: "Gemini CLI",
	}
	if err != nil {
		result.Error = err.Error()
	} else {
		result.Found = true
		result.Email = token.Email
		result.ProviderType = token.ProviderType
	}
	results = append(results, result)

	return results
}

// OAuthScanResult represents the result of scanning for an OAuth token.
type OAuthScanResult struct {
	IDEType      IDEType `json:"ide_type"`
	IDEName      string  `json:"ide_name"`
	Found        bool    `json:"found"`
	Email        string  `json:"email,omitempty"`
	ProviderType string  `json:"provider_type,omitempty"`
	Error        string  `json:"error,omitempty"`
}

// scanAntigravityOAuth scans for Antigravity's OAuth credentials.
// Antigravity stores credentials in state.vscdb (SQLite) or storage.json.
func scanAntigravityOAuth() (*oauth.Token, error) {
	// Try state.vscdb first (primary storage for modern Antigravity)
	for _, path := range getAntigravityVscdbPaths() {
		expanded := expandPath(path)
		token, err := parseAntigravityVscdb(expanded)
		if err == nil {
			return token, nil
		}
	}

	// Fall back to JSON files
	for _, path := range getAntigravityStoragePaths() {
		expanded := expandPath(path)
		data, err := os.ReadFile(expanded)
		if err != nil {
			continue
		}

		token, err := parseAntigravityCredentials(data)
		if err != nil {
			continue
		}
		return token, nil
	}

	return nil, fmt.Errorf("no Antigravity OAuth credentials found")
}

func getAntigravityVscdbPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"~/Library/Application Support/Antigravity/User/globalStorage/state.vscdb",
		}
	case "linux":
		return []string{
			"~/.config/Antigravity/User/globalStorage/state.vscdb",
		}
	case "windows":
		return []string{
			"%APPDATA%/Antigravity/User/globalStorage/state.vscdb",
		}
	}
	return nil
}

func getAntigravityStoragePaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"~/Library/Application Support/Antigravity/User/globalStorage/storage.json",
			"~/Library/Application Support/Antigravity/User/globalStorage/google.oauth.json",
		}
	case "linux":
		return []string{
			"~/.config/Antigravity/User/globalStorage/storage.json",
			"~/.config/Antigravity/User/globalStorage/google.oauth.json",
		}
	case "windows":
		return []string{
			"%APPDATA%/Antigravity/User/globalStorage/storage.json",
			"%APPDATA%/Antigravity/User/globalStorage/google.oauth.json",
		}
	}
	return nil
}

func parseAntigravityCredentials(data []byte) (*oauth.Token, error) {
	var store map[string]interface{}
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}

	// Look for refresh token in various possible keys
	refreshToken := ""
	for _, key := range []string{
		"google.oauth.refreshToken",
		"refreshToken",
		"google.auth.refreshToken",
	} {
		if val, ok := store[key].(string); ok && val != "" {
			refreshToken = val
			break
		}
	}

	if refreshToken == "" {
		// Try nested structure
		if auth, ok := store["google.oauth"].(map[string]interface{}); ok {
			if rt, ok := auth["refreshToken"].(string); ok {
				refreshToken = rt
			}
		}
	}

	// Also look for access token (apiKey in Antigravity's format)
	accessToken := ""
	for _, key := range []string{"apiKey", "accessToken", "google.oauth.accessToken"} {
		if val, ok := store[key].(string); ok && val != "" {
			accessToken = val
			break
		}
	}

	if refreshToken == "" && accessToken == "" {
		return nil, fmt.Errorf("no token found")
	}

	// Extract email if available
	email := ""
	for _, key := range []string{"google.oauth.email", "email", "google.auth.email"} {
		if val, ok := store[key].(string); ok && val != "" {
			email = val
			break
		}
	}

	// Extract project ID if available
	projectID := ""
	for _, key := range []string{"google.oauth.projectId", "projectId", "managedProjectId"} {
		if val, ok := store[key].(string); ok && val != "" {
			projectID = val
			break
		}
	}

	cfg := oauth.AntigravityConfig()
	endpoint := ""
	if len(cfg.CloudCodeEndpoints) > 0 {
		endpoint = cfg.CloudCodeEndpoints[0]
	}

	return &oauth.Token{
		ProviderType: "antigravity",
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Email:        email,
		ProjectID:    projectID,
		Endpoint:     endpoint,
		Scopes:       cfg.Scopes,
		CreatedAt:    timeutil.NowTime(),
		UpdatedAt:    timeutil.NowTime(),
	}, nil
}

// parseAntigravityVscdb reads OAuth credentials from Antigravity's state.vscdb SQLite database.
func parseAntigravityVscdb(dbPath string) (*oauth.Token, error) {
	if _, err := os.Stat(dbPath); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", dbPath+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var value string
	err = db.QueryRow("SELECT value FROM ItemTable WHERE key = 'antigravityAuthStatus'").Scan(&value)
	if err != nil {
		return nil, fmt.Errorf("antigravityAuthStatus not found: %w", err)
	}

	var authStatus struct {
		APIKey string `json:"apiKey"`
		Email  string `json:"email"`
		Name   string `json:"name"`
	}
	if err := json.Unmarshal([]byte(value), &authStatus); err != nil {
		return nil, err
	}

	if authStatus.APIKey == "" {
		return nil, fmt.Errorf("no apiKey in antigravityAuthStatus")
	}

	cfg := oauth.AntigravityConfig()
	endpoint := ""
	if len(cfg.CloudCodeEndpoints) > 0 {
		endpoint = cfg.CloudCodeEndpoints[0]
	}

	return &oauth.Token{
		ProviderType: "antigravity",
		AccessToken:  authStatus.APIKey,
		Email:        authStatus.Email,
		Endpoint:     endpoint,
		Scopes:       cfg.Scopes,
		CreatedAt:    timeutil.NowTime(),
		UpdatedAt:    timeutil.NowTime(),
	}, nil
}

// scanGeminiCLIOAuth scans for Gemini CLI's OAuth credentials.
func scanGeminiCLIOAuth() (*oauth.Token, error) {
	paths := getGeminiCLICredPaths()

	for _, path := range paths {
		expanded := expandPath(path)
		data, err := os.ReadFile(expanded)
		if err != nil {
			continue
		}

		token, err := parseGeminiCLICredentials(data)
		if err != nil {
			continue
		}
		return token, nil
	}

	return nil, fmt.Errorf("no Gemini CLI OAuth credentials found")
}

func getGeminiCLICredPaths() []string {
	home, _ := os.UserHomeDir()
	if home == "" {
		return nil
	}

	return []string{
		filepath.Join(home, ".gemini", "oauth_creds.json"),
		filepath.Join(home, ".config", "gemini", "oauth_creds.json"),
		filepath.Join(home, ".gemini-cli", "oauth_creds.json"),
	}
}

func parseGeminiCLICredentials(data []byte) (*oauth.Token, error) {
	var creds struct {
		RefreshToken string `json:"refresh_token"`
		AccessToken  string `json:"access_token"`
		Email        string `json:"email"`
		ProjectID    string `json:"project_id"`
		ExpiresAt    string `json:"expires_at"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}

	if creds.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token found")
	}

	cfg := oauth.GeminiCLIConfig()
	endpoint := ""
	if len(cfg.CloudCodeEndpoints) > 0 {
		endpoint = cfg.CloudCodeEndpoints[0]
	}

	return &oauth.Token{
		ProviderType: "gemini-cli",
		AccessToken:  creds.AccessToken,
		RefreshToken: creds.RefreshToken,
		Email:        creds.Email,
		ProjectID:    creds.ProjectID,
		Endpoint:     endpoint,
		Scopes:       cfg.Scopes,
		CreatedAt:    timeutil.NowTime(),
		UpdatedAt:    timeutil.NowTime(),
	}, nil
}

// scanCopilotOAuth scans for GitHub Copilot's OAuth token.
func scanCopilotOAuth() (*oauth.Token, error) {
	paths := getCopilotPaths()

	for _, path := range paths {
		expanded := expandPath(path)
		data, err := os.ReadFile(expanded)
		if err != nil {
			continue
		}

		token, err := parseCopilotCredentials(data)
		if err != nil {
			continue
		}
		return token, nil
	}

	return nil, fmt.Errorf("no GitHub Copilot OAuth credentials found")
}

func parseCopilotCredentials(data []byte) (*oauth.Token, error) {
	// Copilot hosts.json format: {"github.com": {"oauth_token": "gho_...", "user": "username"}}
	var hosts map[string]map[string]interface{}
	if err := json.Unmarshal(data, &hosts); err != nil {
		return nil, err
	}

	for host, info := range hosts {
		if !strings.Contains(host, "github") {
			continue
		}

		oauthToken, _ := info["oauth_token"].(string)
		if oauthToken == "" {
			continue
		}

		user, _ := info["user"].(string)

		return &oauth.Token{
			ProviderType: "copilot",
			AccessToken:  oauthToken,
			Email:        user,
			CreatedAt:    timeutil.NowTime(),
			UpdatedAt:    timeutil.NowTime(),
		}, nil
	}

	return nil, fmt.Errorf("no oauth_token found in hosts.json")
}

// scanCodexOAuth scans for OpenAI Codex CLI's OAuth credentials.
// Codex stores tokens at ~/.codex/auth.json
func scanCodexOAuth() (*oauth.Token, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	authPath := filepath.Join(home, ".codex", "auth.json")
	data, err := os.ReadFile(authPath)
	if err != nil {
		return nil, fmt.Errorf("no Codex auth.json found: %w", err)
	}

	return parseCodexCredentials(data)
}

func parseCodexCredentials(data []byte) (*oauth.Token, error) {
	var creds struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresAt    int64  `json:"expires_at"` // Unix timestamp in milliseconds
		AccountID    string `json:"account_id"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}

	if creds.AccessToken == "" && creds.RefreshToken == "" {
		return nil, fmt.Errorf("no tokens in auth.json")
	}

	var expiry time.Time
	if creds.ExpiresAt > 0 {
		expiry = time.UnixMilli(creds.ExpiresAt)
	}

	cfg := oauth.CodexConfig()
	return &oauth.Token{
		ProviderType: "codex",
		AccessToken:  creds.AccessToken,
		RefreshToken: creds.RefreshToken,
		TokenExpiry:  expiry,
		Email:        creds.AccountID, // account_id is the best identifier we have
		Endpoint:     cfg.APIEndpoint,
		Scopes:       cfg.Scopes,
		CreatedAt:    timeutil.NowTime(),
		UpdatedAt:    timeutil.NowTime(),
	}, nil
}
