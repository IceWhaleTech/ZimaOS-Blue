package extauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// TokenGenerator generates access tokens for authenticated users.
type TokenGenerator interface {
	GenerateAccessToken(userID, username, role string) (string, error)
}

// DefaultService implements the Service interface.
type DefaultService struct {
	providers    map[string]*ProviderClient
	stateStore   StateStore
	accountStore AccountStore
	userStore    UserStore
	tokenGen     TokenGenerator
	mu           sync.RWMutex
	stateTTL     time.Duration
}

// ServiceConfig holds the service configuration.
type ServiceConfig struct {
	Providers      []*ProviderConfig
	StateStore     StateStore
	AccountStore   AccountStore
	UserStore      UserStore
	TokenGenerator TokenGenerator
	StateTTL       time.Duration
}

// NewService creates a new external auth service.
func NewService(cfg *ServiceConfig) (*DefaultService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	stateTTL := cfg.StateTTL
	if stateTTL == 0 {
		stateTTL = 10 * time.Minute
	}

	s := &DefaultService{
		providers:    make(map[string]*ProviderClient),
		stateStore:   cfg.StateStore,
		accountStore: cfg.AccountStore,
		userStore:    cfg.UserStore,
		tokenGen:     cfg.TokenGenerator,
		stateTTL:     stateTTL,
	}

	// Initialize providers
	for _, providerCfg := range cfg.Providers {
		if !providerCfg.Enabled {
			continue
		}

		client, err := NewProviderClient(providerCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider %s: %w", providerCfg.ID, err)
		}

		s.providers[providerCfg.ID] = client
	}

	return s, nil
}

// ListProviders returns all enabled providers.
func (s *DefaultService) ListProviders(ctx context.Context) ([]*ProviderInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	providers := make([]*ProviderInfo, 0, len(s.providers))
	for _, client := range s.providers {
		cfg := client.Config()
		providers = append(providers, &ProviderInfo{
			ID:      cfg.ID,
			Name:    cfg.Name,
			Type:    cfg.Type,
			IconURL: cfg.IconURL,
			Order:   cfg.Order,
		})
	}

	return providers, nil
}

// GetProvider returns a provider by ID.
func (s *DefaultService) GetProvider(ctx context.Context, id string) (*ProviderConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.providers[id]
	if !ok {
		return nil, ErrProviderNotFound
	}

	return client.Config(), nil
}

// Authorize starts the authorization flow.
func (s *DefaultService) Authorize(ctx context.Context, req *AuthorizeRequest) (*AuthorizeResponse, error) {
	s.mu.RLock()
	client, ok := s.providers[req.ProviderID]
	s.mu.RUnlock()

	if !ok {
		return nil, ErrProviderNotFound
	}

	// Generate state and nonce
	state, err := generateRandomString(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	nonce, err := generateRandomString(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Generate PKCE code verifier and challenge
	codeVerifier, err := generateRandomString(64)
	if err != nil {
		return nil, fmt.Errorf("failed to generate code verifier: %w", err)
	}
	codeChallenge := generateCodeChallenge(codeVerifier)

	// Store state
	authState := &AuthState{
		State:        state,
		Nonce:        nonce,
		ProviderID:   req.ProviderID,
		RedirectURI:  req.RedirectURI,
		CodeVerifier: codeVerifier,
		CreatedAt:    timeutil.NowTime(),
		ExpiresAt:    timeutil.NowTime().Add(s.stateTTL),
		UserID:       req.UserID,
	}

	if s.stateStore != nil {
		if err := s.stateStore.Save(ctx, authState); err != nil {
			return nil, fmt.Errorf("failed to save state: %w", err)
		}
	}

	// Generate authorization URL
	authURL, err := client.AuthorizationURL(state, nonce, codeChallenge)
	if err != nil {
		return nil, fmt.Errorf("failed to generate auth URL: %w", err)
	}

	return &AuthorizeResponse{
		AuthURL: authURL,
		State:   state,
	}, nil
}

// Callback handles the authorization callback.
func (s *DefaultService) Callback(ctx context.Context, req *CallbackRequest) (*CallbackResponse, error) {
	// Check for error from provider
	if req.Error != "" {
		return nil, fmt.Errorf("provider error: %s: %s", req.Error, req.ErrorDescription)
	}

	// Retrieve and validate state
	var authState *AuthState
	if s.stateStore != nil {
		var err error
		authState, err = s.stateStore.Get(ctx, req.State)
		if err != nil {
			return nil, ErrInvalidState
		}

		// Delete state after retrieval
		_ = s.stateStore.Delete(ctx, req.State)

		// Check expiry
		if timeutil.NowTime().After(authState.ExpiresAt) {
			return nil, ErrStateExpired
		}

		// Verify provider matches
		if authState.ProviderID != req.ProviderID {
			return nil, ErrInvalidState
		}
	}

	// Get provider client
	s.mu.RLock()
	client, ok := s.providers[req.ProviderID]
	s.mu.RUnlock()

	if !ok {
		return nil, ErrProviderNotFound
	}

	// Exchange code for tokens
	codeVerifier := ""
	if authState != nil {
		codeVerifier = authState.CodeVerifier
	}

	tokenResp, err := client.ExchangeCode(ctx, req.Code, codeVerifier)
	if err != nil {
		return nil, err
	}

	// Get user info
	userInfo, err := client.GetUserInfo(ctx, tokenResp.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	// Validate email domain if configured
	cfg := client.Config()
	if len(cfg.AllowedDomains) > 0 && userInfo.Email != "" {
		if !isEmailDomainAllowed(userInfo.Email, cfg.AllowedDomains) {
			return nil, ErrEmailNotAllowed
		}
	}

	// Check if account is already linked
	var linkedAccount *LinkedAccount
	if s.accountStore != nil {
		linkedAccount, _ = s.accountStore.GetByProviderUser(ctx, req.ProviderID, userInfo.Subject)
	}

	var user *AuthenticatedUser
	isNewUser := false

	if linkedAccount != nil {
		// Existing linked account - get user
		if s.userStore != nil {
			user, err = s.userStore.GetByID(ctx, linkedAccount.UserID)
			if err != nil {
				return nil, fmt.Errorf("failed to get user: %w", err)
			}
		}
	} else if authState != nil && authState.UserID != "" {
		// Linking to existing user
		if s.userStore != nil {
			user, err = s.userStore.GetByID(ctx, authState.UserID)
			if err != nil {
				return nil, fmt.Errorf("failed to get user: %w", err)
			}
		}
	} else if cfg.AutoCreateUser {
		// Auto-create new user
		if s.userStore != nil {
			// Check if user with email exists
			user, _ = s.userStore.GetByEmail(ctx, userInfo.Email)
			if user == nil {
				// Create new user
				user = &AuthenticatedUser{
					ID:         generateUserID(),
					Email:      userInfo.Email,
					Name:       userInfo.Name,
					Picture:    userInfo.Picture,
					Roles:      []string{cfg.DefaultRole},
					ProviderID: req.ProviderID,
				}

				// Apply role mappings
				user.Roles = s.applyRoleMappings(cfg, userInfo, user.Roles)

				if err := s.userStore.Create(ctx, user); err != nil {
					return nil, fmt.Errorf("failed to create user: %w", err)
				}
				isNewUser = true
			}
		}
	} else {
		return nil, ErrUserNotFound
	}

	// Create or update linked account
	if s.accountStore != nil && user != nil {
		account := &LinkedAccount{
			ID:             generateAccountID(),
			UserID:         user.ID,
			ProviderID:     req.ProviderID,
			ProviderUserID: userInfo.Subject,
			Email:          userInfo.Email,
			Name:           userInfo.Name,
			Picture:        userInfo.Picture,
			AccessToken:    tokenResp.AccessToken,
			RefreshToken:   tokenResp.RefreshToken,
			TokenExpiry:    timeutil.NowTime().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
			CreatedAt:      timeutil.NowTime(),
			UpdatedAt:      timeutil.NowTime(),
		}

		if linkedAccount != nil {
			account.ID = linkedAccount.ID
			account.CreatedAt = linkedAccount.CreatedAt
		}

		if err := s.accountStore.Save(ctx, account); err != nil {
			return nil, fmt.Errorf("failed to save linked account: %w", err)
		}
	}

	// Generate access token for the user
	var accessToken string
	if s.tokenGen != nil {
		role := "user"
		if len(user.Roles) > 0 {
			role = user.Roles[0]
		}
		token, err := s.tokenGen.GenerateAccessToken(user.ID, user.Email, role)
		if err != nil {
			return nil, fmt.Errorf("failed to generate access token: %w", err)
		}
		accessToken = token
	}

	return &CallbackResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    tokenResp.ExpiresIn,
		IsNewUser:    isNewUser,
	}, nil
}

// RefreshToken refreshes an access token.
func (s *DefaultService) RefreshToken(ctx context.Context, providerID, refreshToken string) (*TokenResponse, error) {
	s.mu.RLock()
	client, ok := s.providers[providerID]
	s.mu.RUnlock()

	if !ok {
		return nil, ErrProviderNotFound
	}

	return client.RefreshToken(ctx, refreshToken)
}

// GetUserInfo gets user info from the provider.
func (s *DefaultService) GetUserInfo(ctx context.Context, providerID, accessToken string) (*UserInfo, error) {
	s.mu.RLock()
	client, ok := s.providers[providerID]
	s.mu.RUnlock()

	if !ok {
		return nil, ErrProviderNotFound
	}

	return client.GetUserInfo(ctx, accessToken)
}

// LinkAccount links an external account to an existing user.
func (s *DefaultService) LinkAccount(ctx context.Context, userID string, req *AuthorizeRequest) (*AuthorizeResponse, error) {
	req.UserID = userID
	return s.Authorize(ctx, req)
}

// UnlinkAccount unlinks an external account from a user.
func (s *DefaultService) UnlinkAccount(ctx context.Context, userID, providerID string) error {
	if s.accountStore == nil {
		return nil
	}
	return s.accountStore.Delete(ctx, userID, providerID)
}

// GetLinkedAccounts returns all linked accounts for a user.
func (s *DefaultService) GetLinkedAccounts(ctx context.Context, userID string) ([]*LinkedAccount, error) {
	if s.accountStore == nil {
		return nil, nil
	}
	return s.accountStore.GetByUser(ctx, userID)
}

// applyRoleMappings applies role mappings based on user info.
func (s *DefaultService) applyRoleMappings(cfg *ProviderConfig, userInfo *UserInfo, defaultRoles []string) []string {
	if len(cfg.RoleMappings) == 0 {
		return defaultRoles
	}

	roles := make(map[string]bool)
	for _, r := range defaultRoles {
		roles[r] = true
	}

	for _, mapping := range cfg.RoleMappings {
		matched := false

		switch mapping.Claim {
		case "groups":
			for _, g := range userInfo.Groups {
				if g == mapping.Value {
					matched = true
					break
				}
			}
		case "roles":
			for _, r := range userInfo.Roles {
				if r == mapping.Value {
					matched = true
					break
				}
			}
		case "email":
			if userInfo.Email == mapping.Value {
				matched = true
			}
		case "email_domain":
			if strings.HasSuffix(userInfo.Email, "@"+mapping.Value) {
				matched = true
			}
		}

		if matched {
			roles[mapping.Role] = true
		}
	}

	result := make([]string, 0, len(roles))
	for r := range roles {
		result = append(result, r)
	}
	return result
}

// AddProvider adds a new provider at runtime.
func (s *DefaultService) AddProvider(cfg *ProviderConfig) error {
	client, err := NewProviderClient(cfg)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.providers[cfg.ID] = client
	s.mu.Unlock()

	return nil
}

// RemoveProvider removes a provider at runtime.
func (s *DefaultService) RemoveProvider(id string) {
	s.mu.Lock()
	delete(s.providers, id)
	s.mu.Unlock()
}

// Helper functions

func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func isEmailDomainAllowed(email string, allowedDomains []string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	domain := strings.ToLower(parts[1])

	for _, allowed := range allowedDomains {
		if strings.ToLower(allowed) == domain {
			return true
		}
	}
	return false
}

func generateUserID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("user_%s", base64.RawURLEncoding.EncodeToString(bytes))
}

func generateAccountID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("acc_%s", base64.RawURLEncoding.EncodeToString(bytes))
}
