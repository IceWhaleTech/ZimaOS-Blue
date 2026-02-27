package ide

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
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
	for _, authPath := range getCodexAuthPaths() {
		data, err := os.ReadFile(authPath)
		if err != nil {
			continue
		}
		token, err := parseCodexCredentials(data)
		if err == nil {
			return token, nil
		}
	}

	// macOS Codex CLI may persist credentials in Keychain.
	if runtime.GOOS == "darwin" {
		if token, err := readCodexOAuthFromKeychain(); err == nil {
			return token, nil
		}
	}

	return nil, fmt.Errorf("no Codex OAuth credentials found")
}

func getCodexAuthPaths() []string {
	paths := []string{}
	if codexHome := strings.TrimSpace(os.Getenv("CODEX_HOME")); codexHome != "" {
		paths = append(paths, filepath.Join(expandPath(codexHome), "auth.json"))
	}
	paths = append(paths, expandPath("~/.codex/auth.json"))
	return paths
}

func parseCodexCredentials(data []byte) (*oauth.Token, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	accessToken := getString(raw, "access_token")
	refreshToken := getString(raw, "refresh_token")
	accountID := getString(raw, "account_id")
	expiresAt := getInt64(raw, "expires_at")

	if tokens, ok := raw["tokens"].(map[string]interface{}); ok {
		if accessToken == "" {
			accessToken = getString(tokens, "access_token")
		}
		if refreshToken == "" {
			refreshToken = getString(tokens, "refresh_token")
		}
		if accountID == "" {
			accountID = getString(tokens, "account_id")
		}
		if expiresAt == 0 {
			expiresAt = getInt64(tokens, "expires_at")
		}
	}

	if accessToken == "" && refreshToken == "" {
		return nil, fmt.Errorf("no tokens in auth.json")
	}

	var expiry time.Time
	if expiresAt > 0 {
		// Compatible with both unix seconds and unix milliseconds.
		if expiresAt < 1_000_000_000_000 {
			expiry = time.Unix(expiresAt, 0)
		} else {
			expiry = time.UnixMilli(expiresAt)
		}
	}

	cfg := oauth.CodexConfig()
	return &oauth.Token{
		ProviderType: "codex",
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenExpiry:  expiry,
		Email:        accountID, // account_id is the best identifier we have
		Endpoint:     cfg.APIEndpoint,
		Scopes:       cfg.Scopes,
		CreatedAt:    timeutil.NowTime(),
		UpdatedAt:    timeutil.NowTime(),
	}, nil
}

func getString(obj map[string]interface{}, key string) string {
	if val, ok := obj[key].(string); ok {
		return strings.TrimSpace(val)
	}
	return ""
}

func getInt64(obj map[string]interface{}, key string) int64 {
	val, ok := obj[key]
	if !ok {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		if v == "" {
			return 0
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			return n
		}
	}
	return 0
}

func readCodexOAuthFromKeychain() (*oauth.Token, error) {
	codexHome := resolveCodexHomePath()
	account := computeCodexKeychainAccount(codexHome)

	out, err := exec.Command("security", "find-generic-password", "-s", "Codex Auth", "-a", account, "-w").Output()
	if err != nil {
		return nil, err
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &payload); err != nil {
		return nil, err
	}

	tokens, _ := payload["tokens"].(map[string]interface{})
	accessToken := ""
	refreshToken := ""
	accountID := ""
	if tokens != nil {
		accessToken = getString(tokens, "access_token")
		refreshToken = getString(tokens, "refresh_token")
		accountID = getString(tokens, "account_id")
	}
	if accessToken == "" {
		accessToken = getString(payload, "access_token")
	}
	if refreshToken == "" {
		refreshToken = getString(payload, "refresh_token")
	}
	if accountID == "" {
		accountID = getString(payload, "account_id")
	}
	if accessToken == "" && refreshToken == "" {
		return nil, fmt.Errorf("no tokens in keychain payload")
	}

	// Keychain payload usually stores last_refresh; estimate expiry as one hour.
	lastRefresh := getInt64(payload, "last_refresh")
	expiry := time.Time{}
	if lastRefresh > 0 {
		if lastRefresh < 1_000_000_000_000 {
			expiry = time.Unix(lastRefresh, 0).Add(time.Hour)
		} else {
			expiry = time.UnixMilli(lastRefresh).Add(time.Hour)
		}
	}

	cfg := oauth.CodexConfig()
	return &oauth.Token{
		ProviderType: "codex",
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenExpiry:  expiry,
		Email:        accountID,
		Endpoint:     cfg.APIEndpoint,
		Scopes:       cfg.Scopes,
		CreatedAt:    timeutil.NowTime(),
		UpdatedAt:    timeutil.NowTime(),
	}, nil
}

func resolveCodexHomePath() string {
	if codexHome := strings.TrimSpace(os.Getenv("CODEX_HOME")); codexHome != "" {
		expanded := expandPath(codexHome)
		if real, err := filepath.EvalSymlinks(expanded); err == nil {
			return real
		}
		return expanded
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		return expandPath("~/.codex")
	}
	return filepath.Join(home, ".codex")
}

func computeCodexKeychainAccount(codexHome string) string {
	sum := sha256.Sum256([]byte(codexHome))
	return "cli|" + hex.EncodeToString(sum[:])[:16]
}
