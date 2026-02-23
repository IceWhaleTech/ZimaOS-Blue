package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Manager handles OAuth token lifecycle for LLM providers.
type Manager struct {
	store    *Store
	configs  map[string]*ProviderConfig
	pending  map[string]*pendingAuth // state -> pending auth
	mu       sync.RWMutex
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
func NewManager(store *Store) *Manager {
	return &Manager{
		store:   store,
		configs: AllProviderConfigs(),
		pending: make(map[string]*pendingAuth),
	}
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
		CreatedAt:    time.Now(),
	}
	m.mu.Unlock()

	// Build auth URL
	params := url.Values{
		"client_id":             {cfg.ClientID},
		"redirect_uri":         {fmt.Sprintf("http://localhost:%d%s", cfg.RedirectPort, cfg.RedirectPath)},
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
		CreatedAt:    time.Now(),
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

	token := &Token{
		ProviderID:   pending.ProviderID,
		ProviderType: pending.ProviderType,
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenExpiry:  time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		Scopes:       cfg.Scopes,
		Email:        email,
		ProjectID:    projectID,
		Endpoint:     endpoint,
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
		TokenExpiry:  time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		Scopes:       cfg.Scopes,
		Email:        email,
	}

	if err := m.store.SaveToken(pending.ProviderID, token); err != nil {
		return nil, fmt.Errorf("save token: %w", err)
	}

	return token, nil
}

// GetAccessToken returns a valid access token for the provider, refreshing if needed.
func (m *Manager) GetAccessToken(providerID string) (string, error) {
	token, err := m.store.LoadToken(providerID)
	if err != nil {
		return "", err
	}

	if !token.Expired() {
		return token.AccessToken, nil
	}

	// Refresh
	if token.RefreshToken == "" {
		return "", fmt.Errorf("token expired and no refresh token available")
	}

	cfg := m.configs[token.ProviderType]
	if cfg == nil {
		return "", fmt.Errorf("unknown provider type: %s", token.ProviderType)
	}

	newToken, err := m.refreshToken(context.Background(), cfg, token.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("refresh token: %w", err)
	}

	// Update stored token
	token.AccessToken = newToken.AccessToken
	token.TokenExpiry = time.Now().Add(time.Duration(newToken.ExpiresIn) * time.Second)
	if newToken.RefreshToken != "" {
		token.RefreshToken = newToken.RefreshToken
	}

	if err := m.store.SaveToken(providerID, token); err != nil {
		slog.Error("[oauth] failed to save refreshed token", "error", err)
	}

	return token.AccessToken, nil
}

// GetToken returns the full token for a provider.
func (m *Manager) GetToken(providerID string) (*Token, error) {
	return m.store.LoadToken(providerID)
}

// Disconnect revokes and removes the OAuth token for a provider.
func (m *Manager) Disconnect(providerID string) error {
	return m.store.DeleteToken(providerID)
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

func (m *Manager) exchangeCode(ctx context.Context, cfg *ProviderConfig, code, codeVerifier string) (*tokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {fmt.Sprintf("http://localhost:%d%s", cfg.RedirectPort, cfg.RedirectPath)},
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
