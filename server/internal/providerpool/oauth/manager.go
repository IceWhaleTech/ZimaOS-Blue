package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

var ErrTokenExpiredReconnectRequired = errors.New("oauth token expired and reconnect is required")

// Manager handles OAuth token lifecycle for LLM providers.
type Manager struct {
	store    TokenStore
	configs  map[string]*ProviderConfig
	pending  map[string]*pendingAuth // state -> pending auth
	mu       sync.RWMutex

	port int // Actual server port for OAuth callbacks (0 = use config default)

	// Cached Copilot JWT (short-lived, ~30 min)
	copilotToken   string
	copilotExpiry  time.Time
	copilotEndpoint string // API endpoint from token response (e.g., api.individual.githubcopilot.com)
	copilotMu      sync.Mutex

	refreshLoopOnce sync.Once
}

type pendingAuth struct {
	ProviderType string
	ProviderID   string // The provider pool ID to associate with
	CodeVerifier string
	State        string
	CreatedAt    time.Time
}

// tokenResponse is the standard OAuth token response.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
}

// AuthStartResult is returned when starting an OAuth flow.
type AuthStartResult struct {
	// For authorization code flow
	AuthURL string `json:"auth_url,omitempty"`
	State   string `json:"state,omitempty"`

	// For device code flow
	DeviceCode      string `json:"device_code,omitempty"`
	UserCode        string `json:"user_code,omitempty"`
	VerificationURI string `json:"verification_uri,omitempty"`
	ExpiresIn       int    `json:"expires_in,omitempty"`
	Interval        int    `json:"interval,omitempty"`
}

// NewManager creates a new OAuth manager.
func NewManager(store TokenStore) *Manager {
	return &Manager{
		store:   store,
		configs: AllProviderConfigs(),
		pending: make(map[string]*pendingAuth),
		port:    0, // 0 means use config default
	}
}

// SetPort sets the actual server port for OAuth callbacks.
func (m *Manager) SetPort(port int) {
	m.port = port
}

// StartAuth begins an OAuth flow for the given provider type.
// providerID is the provider pool ID to associate the token with.
func (m *Manager) StartAuth(ctx context.Context, providerID, providerType string) (*AuthStartResult, error) {
	cfg := m.configs[providerType]
	if cfg == nil {
		return nil, fmt.Errorf("unknown oauth provider type: %s", providerType)
	}

	switch cfg.FlowType {
	case FlowTypeAuthCode:
		return m.startAuthCodeFlow(providerID, cfg)
	case FlowTypeDeviceCode:
		return m.startDeviceCodeFlow(ctx, providerID, cfg)
	default:
		return nil, fmt.Errorf("unsupported flow type: %s", cfg.FlowType)
	}
}

func (m *Manager) startAuthCodeFlow(providerID string, cfg *ProviderConfig) (*AuthStartResult, error) {
	// Generate PKCE
	verifier, err := generateRandomBytes(64)
	if err != nil {
		return nil, err
	}
	codeVerifier := base64.RawURLEncoding.EncodeToString(verifier)
	challenge := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(challenge[:])

	// Generate state
	stateBytes, err := generateRandomBytes(32)
	if err != nil {
		return nil, err
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	// Store pending auth
	m.mu.Lock()
	m.pending[state] = &pendingAuth{
		ProviderType: cfg.ID,
		ProviderID:   providerID,
		CodeVerifier: codeVerifier,
		State:        state,
		CreatedAt:    timeutil.NowTime(),
	}
	m.mu.Unlock()

	// Build auth URL
	// Use manager's port if set, otherwise fall back to config default
	redirectPort := cfg.RedirectPort
	if m.port > 0 {
		redirectPort = m.port
	}
	params := url.Values{
		"client_id":             {cfg.ClientID},
		"redirect_uri":         {fmt.Sprintf("http://localhost:%d%s", redirectPort, cfg.RedirectPath)},
		"response_type":        {"code"},
		"scope":                {strings.Join(cfg.Scopes, " ")},
		"state":                {state},
		"access_type":          {"offline"},
		"prompt":               {"consent"},
		"code_challenge":       {codeChallenge},
		"code_challenge_method": {"S256"},
	}

	authURL := cfg.AuthURL + "?" + params.Encode()

	return &AuthStartResult{
		AuthURL: authURL,
		State:   state,
	}, nil
}

func (m *Manager) startDeviceCodeFlow(ctx context.Context, providerID string, cfg *ProviderConfig) (*AuthStartResult, error) {
	dcResp, err := RequestDeviceCode(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// Store pending auth with device code as state
	m.mu.Lock()
	m.pending[dcResp.DeviceCode] = &pendingAuth{
		ProviderType: cfg.ID,
		ProviderID:   providerID,
		State:        dcResp.DeviceCode,
		CreatedAt:    timeutil.NowTime(),
	}
	m.mu.Unlock()

	return &AuthStartResult{
		DeviceCode:      dcResp.DeviceCode,
		UserCode:        dcResp.UserCode,
		VerificationURI: dcResp.VerificationURI,
		ExpiresIn:       dcResp.ExpiresIn,
		Interval:        dcResp.Interval,
	}, nil
}

// HandleCallback processes an OAuth callback with an authorization code.
func (m *Manager) HandleCallback(ctx context.Context, state, code string) (*Token, error) {
	m.mu.Lock()
	pending, ok := m.pending[state]
	if ok {
		delete(m.pending, state)
	}
	m.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("invalid or expired state")
	}

	// Check expiry (10 min)
	if time.Since(pending.CreatedAt) > 10*time.Minute {
		return nil, fmt.Errorf("auth state expired")
	}

	cfg := m.configs[pending.ProviderType]
	if cfg == nil {
		return nil, fmt.Errorf("unknown provider type: %s", pending.ProviderType)
	}

	// Exchange code for tokens
	tokenResp, err := m.exchangeCode(ctx, cfg, code, pending.CodeVerifier)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}

	// Get user info
	email, err := m.getUserEmail(ctx, cfg, tokenResp.AccessToken)
	if err != nil {
		slog.Warn("[oauth] failed to get user email", "error", err)
	}

	// Discover project ID for Google Cloud Code providers
	var projectID string
	if cfg.LoadCodeAssistURL != "" {
		projectID, err = m.discoverProjectID(ctx, cfg, tokenResp.AccessToken)
		if err != nil {
			slog.Warn("[oauth] failed to discover project ID", "error", err)
		}
	}

	// Determine endpoint
	endpoint := ""
	if len(cfg.CloudCodeEndpoints) > 0 {
		endpoint = cfg.CloudCodeEndpoints[0]
	}

	// Create token with empty ID - SaveToken will check for duplicates by email
	// and update existing token instead of creating duplicates
	token := &Token{
		ProviderID:   pending.ProviderID,
		ProviderType: pending.ProviderType,
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenExpiry:  timeutil.NowTime().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		Scopes:       cfg.Scopes,
		Email:        email,
		ProjectID:    projectID,
		Endpoint:     endpoint,
		// ID left empty - SaveToken will generate one or reuse existing by email
	}

	// Persist
	if err := m.store.SaveToken(pending.ProviderID, token); err != nil {
		return nil, fmt.Errorf("save token: %w", err)
	}

	return token, nil
}

// CompleteDeviceFlow polls for the device code flow completion.
func (m *Manager) CompleteDeviceFlow(ctx context.Context, deviceCode string, interval int) (*Token, error) {
	m.mu.RLock()
	pending, ok := m.pending[deviceCode]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("invalid device code")
	}

	cfg := m.configs[pending.ProviderType]
	if cfg == nil {
		return nil, fmt.Errorf("unknown provider type: %s", pending.ProviderType)
	}

	tokenResp, err := PollDeviceToken(ctx, cfg, deviceCode, interval)
	if err != nil {
		return nil, err
	}

	// Clean up pending
	m.mu.Lock()
	delete(m.pending, deviceCode)
	m.mu.Unlock()

	// Get user info
	email, err := m.getUserEmail(ctx, cfg, tokenResp.AccessToken)
	if err != nil {
		slog.Warn("[oauth] failed to get user email", "error", err)
	}

	token := &Token{
		ProviderID:   pending.ProviderID,
		ProviderType: pending.ProviderType,
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenExpiry:  timeutil.NowTime().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		Scopes:       cfg.Scopes,
		Email:        email,
		// ID left empty - SaveToken will generate one or reuse existing by email
	}

	if err := m.store.SaveToken(pending.ProviderID, token); err != nil {
		return nil, fmt.Errorf("save token: %w", err)
	}

	return token, nil
}

// GetAccessToken returns a valid access token for the provider, refreshing if needed.
// It tries all tokens for the provider and returns the first valid one.
func (m *Manager) GetAccessToken(providerID string) (string, error) {
	tokens, err := m.store.LoadTokens(providerID)
	if err != nil || len(tokens) == 0 {
		return "", fmt.Errorf("no oauth tokens for provider %s", providerID)
	}

	var lastErr error
	for _, token := range tokens {
		accessToken, err := m.getOrRefreshToken(token)
		if err == nil {
			return accessToken, nil
		}
		lastErr = err
		slog.Warn("[oauth] token unusable, trying next", "provider", providerID, "token_id", token.ID, "error", err)
	}
	if lastErr != nil {
		return "", fmt.Errorf("all oauth tokens unusable for provider %s: %w", providerID, lastErr)
	}
	return "", fmt.Errorf("all oauth tokens unusable for provider %s", providerID)
}

// GetCopilotAccessToken returns a Copilot-specific JWT for the provider.
// GitHub Copilot API requires exchanging the GitHub OAuth token for a short-lived
// Copilot JWT via https://api.github.com/copilot_internal/v2/token.
// The JWT is cached until 5 minutes before expiry.
func (m *Manager) GetCopilotAccessToken(providerID string) (string, error) {
	m.copilotMu.Lock()
	defer m.copilotMu.Unlock()

	// Return cached token if still valid (with 5 min buffer)
	if m.copilotToken != "" && timeutil.NowTime().Add(5*time.Minute).Before(m.copilotExpiry) {
		return m.copilotToken, nil
	}

	// Get the GitHub OAuth token first
	githubToken, err := m.GetAccessToken(providerID)
	if err != nil {
		return "", fmt.Errorf("get github token: %w", err)
	}

	// Exchange for Copilot JWT
	copilotJWT, expiresAt, endpoint, err := GetCopilotToken(context.Background(), githubToken)
	if err != nil {
		return "", fmt.Errorf("get copilot token: %w", err)
	}

	m.copilotToken = copilotJWT
	m.copilotExpiry = expiresAt
	m.copilotEndpoint = endpoint
	slog.Info("[oauth] copilot token refreshed", "provider", providerID, "expires_at", expiresAt, "endpoint", endpoint)
	return copilotJWT, nil
}

// GetCopilotEndpoint returns the Copilot API endpoint from the token response.
// Returns empty string if not available (will use default).
func (m *Manager) GetCopilotEndpoint() string {
	m.copilotMu.Lock()
	defer m.copilotMu.Unlock()
	return m.copilotEndpoint
}

// GetAccessTokenByID returns a valid access token for a specific token ID.
func (m *Manager) GetAccessTokenByID(providerID, tokenID string) (string, error) {
	token, err := m.store.LoadToken(providerID, tokenID)
	if err != nil {
		return "", err
	}
	return m.getOrRefreshToken(token)
}

func (m *Manager) getOrRefreshToken(token *Token) (string, error) {
	if !token.Expired() {
		return token.AccessToken, nil
	}

	if token.RefreshToken == "" {
		m.pruneToken(token, "expired_without_refresh")
		return "", fmt.Errorf("%w: token_id=%s provider=%s", ErrTokenExpiredReconnectRequired, token.ID, token.ProviderID)
	}

	cfg := m.configs[token.ProviderType]
	if cfg == nil {
		return "", fmt.Errorf("unknown provider type: %s", token.ProviderType)
	}

	newToken, err := m.refreshToken(context.Background(), cfg, token.RefreshToken)
	if err != nil {
		if isTerminalRefreshError(err) {
			m.pruneToken(token, "refresh_failed_terminal")
			return "", fmt.Errorf("%w: token_id=%s provider=%s refresh_error=%v", ErrTokenExpiredReconnectRequired, token.ID, token.ProviderID, err)
		}
		return "", fmt.Errorf("refresh token: %w", err)
	}

	token.AccessToken = newToken.AccessToken
	token.TokenExpiry = timeutil.NowTime().Add(time.Duration(newToken.ExpiresIn) * time.Second)
	if newToken.RefreshToken != "" {
		token.RefreshToken = newToken.RefreshToken
	}

	if err := m.store.SaveToken(token.ProviderID, token); err != nil {
		slog.Error("[oauth] failed to save refreshed token", "error", err)
	}

	return token.AccessToken, nil
}

func isTerminalRefreshError(err error) bool {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "invalid_grant"),
		strings.Contains(msg, "invalid_token"),
		strings.Contains(msg, "expired"),
		strings.Contains(msg, "revoked"),
		strings.Contains(msg, "unauthorized"),
		strings.Contains(msg, "bad credentials"):
		return true
	default:
		return false
	}
}

func (m *Manager) pruneToken(token *Token, reason string) {
	if token == nil || token.ProviderID == "" || token.ID == "" {
		return
	}
	if err := m.store.DeleteToken(token.ProviderID, token.ID); err != nil {
		slog.Warn("[oauth] failed to remove unusable token", "provider", token.ProviderID, "token_id", token.ID, "reason", reason, "error", err)
		return
	}
	slog.Info("[oauth] removed unusable token", "provider", token.ProviderID, "token_id", token.ID, "reason", reason)
}

// GetToken returns the full token for a specific provider and token ID.
func (m *Manager) GetToken(providerID, tokenID string) (*Token, error) {
	return m.store.LoadToken(providerID, tokenID)
}

// GetTokens returns all tokens for a provider.
func (m *Manager) GetTokens(providerID string) ([]*Token, error) {
	return m.store.LoadTokens(providerID)
}

// Disconnect revokes and removes a specific OAuth token.
func (m *Manager) Disconnect(providerID, tokenID string) error {
	return m.store.DeleteToken(providerID, tokenID)
}

// ImportToken imports an externally obtained token (e.g., from IDE scan).
func (m *Manager) ImportToken(token *Token) error {
	return m.store.SaveToken(token.ProviderID, token)
}

// ListTokens returns all stored tokens.
func (m *Manager) ListTokens() (map[string]*Token, error) {
	return m.store.ListTokens()
}

// CleanupPending removes expired pending auth states.
func (m *Manager) CleanupPending() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for state, pending := range m.pending {
		if time.Since(pending.CreatedAt) > 10*time.Minute {
			delete(m.pending, state)
		}
	}
}

// StartAutoRefresh starts a background loop that periodically refreshes OAuth tokens.
// Safe to call multiple times; only one loop will run.
func (m *Manager) StartAutoRefresh(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}

	m.refreshLoopOnce.Do(func() {
		go func() {
			m.RefreshAllTokens()

			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					slog.Info("[oauth] auto-refresh loop stopped", "reason", ctx.Err())
					return
				case <-ticker.C:
					m.RefreshAllTokens()
				}
			}
		}()
		slog.Info("[oauth] auto-refresh loop started", "interval", interval.String())
	})
}

// RefreshAllTokens tries to refresh all stored OAuth tokens that are expired or near expiry.
// Invalid/unrefreshable tokens are pruned by getOrRefreshToken.
func (m *Manager) RefreshAllTokens() {
	all, err := m.store.ListTokens()
	if err != nil {
		slog.Warn("[oauth] list tokens for auto-refresh failed", "error", err)
		return
	}
	if len(all) == 0 {
		return
	}

	for _, token := range all {
		if token == nil || token.ProviderID == "" {
			continue
		}
		if _, err := m.getOrRefreshToken(token); err != nil {
			slog.Debug("[oauth] auto-refresh skipped unusable token", "provider", token.ProviderID, "token_id", token.ID, "error", err)
		}
	}
}

func (m *Manager) exchangeCode(ctx context.Context, cfg *ProviderConfig, code, codeVerifier string) (*tokenResponse, error) {
	// Use manager's port if set, otherwise fall back to config default
	redirectPort := cfg.RedirectPort
	if m.port > 0 {
		redirectPort = m.port
	}
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {fmt.Sprintf("http://localhost:%d%s", redirectPort, cfg.RedirectPath)},
		"client_id":     {cfg.ClientID},
		"client_secret": {cfg.ClientSecret},
		"code_verifier": {codeVerifier},
	}

	return m.doTokenRequest(ctx, cfg.TokenURL, data)
}

func (m *Manager) refreshToken(ctx context.Context, cfg *ProviderConfig, refreshToken string) (*tokenResponse, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {cfg.ClientID},
		"client_secret": {cfg.ClientSecret},
	}

	return m.doTokenRequest(ctx, cfg.TokenURL, data)
}

func (m *Manager) doTokenRequest(ctx context.Context, tokenURL string, data url.Values) (*tokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed (%d): %s", resp.StatusCode, body)
	}

	var result tokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (m *Manager) getUserEmail(ctx context.Context, cfg *ProviderConfig, accessToken string) (string, error) {
	if cfg.UserInfoURL == "" {
		return "", nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.UserInfoURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var userInfo struct {
		Email string `json:"email"`
		Login string `json:"login"` // GitHub
	}
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return "", err
	}

	if userInfo.Email != "" {
		return userInfo.Email, nil
	}
	return userInfo.Login, nil
}

func (m *Manager) discoverProjectID(ctx context.Context, cfg *ProviderConfig, accessToken string) (string, error) {
	for _, endpoint := range cfg.CloudCodeEndpoints {
		projectID, err := m.tryLoadCodeAssist(ctx, endpoint+cfg.LoadCodeAssistURL, accessToken)
		if err == nil && projectID != "" {
			return projectID, nil
		}
	}
	return "", fmt.Errorf("failed to discover project ID from any endpoint")
}

func (m *Manager) tryLoadCodeAssist(ctx context.Context, url, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader("{}"))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("loadCodeAssist failed (%d): %s", resp.StatusCode, body)
	}

	var result struct {
		ProjectID        string `json:"projectId"`
		ManagedProjectID string `json:"managedProjectId"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if result.ManagedProjectID != "" {
		return result.ManagedProjectID, nil
	}
	return result.ProjectID, nil
}

func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}
