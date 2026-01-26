package extauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ProviderClient is a client for an external auth provider.
type ProviderClient struct {
	config     *ProviderConfig
	discovery  *DiscoveryDocument
	httpClient *http.Client
}

// NewProviderClient creates a new provider client.
func NewProviderClient(config *ProviderConfig) (*ProviderClient, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	return &ProviderClient{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Discover fetches the OIDC discovery document.
func (c *ProviderClient) Discover(ctx context.Context) (*DiscoveryDocument, error) {
	if c.discovery != nil {
		return c.discovery, nil
	}

	// Use pre-configured endpoints for known providers
	switch c.config.Type {
	case ProviderGitHub:
		c.discovery = &DiscoveryDocument{
			Issuer:                "https://github.com",
			AuthorizationEndpoint: "https://github.com/login/oauth/authorize",
			TokenEndpoint:         "https://github.com/login/oauth/access_token",
			UserInfoEndpoint:      "https://api.github.com/user",
		}
		return c.discovery, nil

	case ProviderGoogle:
		if c.config.IssuerURL == "" {
			c.config.IssuerURL = "https://accounts.google.com"
		}

	case ProviderMicrosoft:
		if c.config.IssuerURL == "" {
			c.config.IssuerURL = "https://login.microsoftonline.com/common/v2.0"
		}
	}

	// Use configured URLs if all are provided
	if c.config.AuthURL != "" && c.config.TokenURL != "" {
		c.discovery = &DiscoveryDocument{
			Issuer:                c.config.IssuerURL,
			AuthorizationEndpoint: c.config.AuthURL,
			TokenEndpoint:         c.config.TokenURL,
			UserInfoEndpoint:      c.config.UserInfoURL,
			JWKSURI:               c.config.JWKSURL,
		}
		return c.discovery, nil
	}

	// Fetch discovery document
	if c.config.IssuerURL == "" {
		return nil, ErrDiscoveryFailed
	}

	discoveryURL := strings.TrimSuffix(c.config.IssuerURL, "/") + "/.well-known/openid-configuration"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDiscoveryFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: status %d: %s", ErrDiscoveryFailed, resp.StatusCode, string(body))
	}

	var doc DiscoveryDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDiscoveryFailed, err)
	}

	c.discovery = &doc
	return c.discovery, nil
}

// AuthorizationURL returns the authorization URL for the auth flow.
func (c *ProviderClient) AuthorizationURL(state, nonce, codeChallenge string) (string, error) {
	doc, err := c.Discover(context.Background())
	if err != nil {
		return "", err
	}

	u, err := url.Parse(doc.AuthorizationEndpoint)
	if err != nil {
		return "", fmt.Errorf("invalid authorization endpoint: %w", err)
	}

	q := u.Query()
	q.Set("client_id", c.config.ClientID)
	q.Set("redirect_uri", c.config.RedirectURL)
	q.Set("response_type", "code")
	q.Set("state", state)

	// Set scopes
	scopes := c.config.Scopes
	if len(scopes) == 0 {
		scopes = c.defaultScopes()
	}
	q.Set("scope", strings.Join(scopes, " "))

	// Add nonce for OIDC (not for GitHub)
	if nonce != "" && c.config.Type != ProviderGitHub {
		q.Set("nonce", nonce)
	}

	// Add PKCE code challenge
	if codeChallenge != "" {
		q.Set("code_challenge", codeChallenge)
		q.Set("code_challenge_method", "S256")
	}

	// Provider-specific parameters
	switch c.config.Type {
	case ProviderGoogle:
		q.Set("access_type", "offline")
		q.Set("prompt", "consent")
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

// defaultScopes returns the default scopes for the provider type.
func (c *ProviderClient) defaultScopes() []string {
	switch c.config.Type {
	case ProviderGitHub:
		return []string{"user:email", "read:user"}
	case ProviderGoogle:
		return []string{"openid", "email", "profile"}
	case ProviderMicrosoft:
		return []string{"openid", "email", "profile", "offline_access"}
	default:
		return []string{"openid", "email", "profile"}
	}
}

// ExchangeCode exchanges an authorization code for tokens.
func (c *ProviderClient) ExchangeCode(ctx context.Context, code, codeVerifier string) (*TokenResponse, error) {
	doc, err := c.Discover(ctx)
	if err != nil {
		return nil, err
	}

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", c.config.RedirectURL)
	data.Set("client_id", c.config.ClientID)
	data.Set("client_secret", c.config.ClientSecret)

	if codeVerifier != "" {
		data.Set("code_verifier", codeVerifier)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, doc.TokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// GitHub requires Accept header for JSON response
	if c.config.Type == ProviderGitHub {
		req.Header.Set("Accept", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: status %d: %s", ErrInvalidCode, resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &tokenResp, nil
}

// RefreshToken refreshes an access token using a refresh token.
func (c *ProviderClient) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	doc, err := c.Discover(ctx)
	if err != nil {
		return nil, err
	}

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", c.config.ClientID)
	data.Set("client_secret", c.config.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, doc.TokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token refresh failed: status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &tokenResp, nil
}

// GetUserInfo fetches user info from the userinfo endpoint.
func (c *ProviderClient) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	doc, err := c.Discover(ctx)
	if err != nil {
		return nil, err
	}

	if doc.UserInfoEndpoint == "" {
		return nil, fmt.Errorf("userinfo endpoint not available")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, doc.UserInfoEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create userinfo request: %w", err)
	}

	// Set authorization header based on provider
	switch c.config.Type {
	case ProviderGitHub:
		req.Header.Set("Authorization", "token "+accessToken)
		req.Header.Set("Accept", "application/vnd.github.v3+json")
	default:
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch userinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("userinfo request failed: status %d: %s", resp.StatusCode, string(body))
	}

	// Parse based on provider type
	switch c.config.Type {
	case ProviderGitHub:
		return c.parseGitHubUser(resp.Body, accessToken)
	default:
		return c.parseOIDCUserInfo(resp.Body)
	}
}

// parseOIDCUserInfo parses standard OIDC userinfo response.
func (c *ProviderClient) parseOIDCUserInfo(body io.Reader) (*UserInfo, error) {
	var userInfo UserInfo
	if err := json.NewDecoder(body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode userinfo: %w", err)
	}
	return &userInfo, nil
}

// parseGitHubUser parses GitHub's user response into UserInfo.
func (c *ProviderClient) parseGitHubUser(body io.Reader, accessToken string) (*UserInfo, error) {
	var ghUser struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}

	if err := json.NewDecoder(body).Decode(&ghUser); err != nil {
		return nil, fmt.Errorf("failed to decode GitHub user: %w", err)
	}

	userInfo := &UserInfo{
		Subject:  fmt.Sprintf("%d", ghUser.ID),
		Email:    ghUser.Email,
		Name:     ghUser.Name,
		Picture:  ghUser.AvatarURL,
		Username: ghUser.Login,
	}

	// If email is not public, fetch from emails endpoint
	if userInfo.Email == "" {
		email, err := c.fetchGitHubEmail(context.Background(), accessToken)
		if err == nil {
			userInfo.Email = email
		}
	}

	return userInfo, nil
}

// fetchGitHubEmail fetches the primary email from GitHub's emails endpoint.
func (c *ProviderClient) fetchGitHubEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "token "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch emails")
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	// Find primary verified email
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}

	// Fall back to any verified email
	for _, e := range emails {
		if e.Verified {
			return e.Email, nil
		}
	}

	return "", fmt.Errorf("no verified email found")
}

// Config returns the provider configuration.
func (c *ProviderClient) Config() *ProviderConfig {
	return c.config
}
