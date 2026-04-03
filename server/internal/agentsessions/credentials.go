package agentsessions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool/oauth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type CredentialResolver interface {
	Materialize(ctx context.Context, profile AgentProfile) (*MaterializedCredentials, error)
}

type MaterializedCredentials struct {
	Env     map[string]string
	Cleanup func() error
}

type OAuthCredentialSource interface {
	GetTokens(providerID string) ([]*oauth.Token, error)
	GetAccessTokenByID(providerID, tokenID string) (string, error)
}

type ProviderPoolCredentialResolver struct {
	getOAuthSource func() OAuthCredentialSource
	lookupAPIKey   func(providerID string) (string, error)
	logger         *zap.Logger
}

func NewProviderPoolCredentialResolver(
	getOAuthSource func() OAuthCredentialSource,
	lookupAPIKey func(providerID string) (string, error),
	logger *zap.Logger,
) *ProviderPoolCredentialResolver {
	return &ProviderPoolCredentialResolver{
		getOAuthSource: getOAuthSource,
		lookupAPIKey:   lookupAPIKey,
		logger:         logger,
	}
}

func (r *ProviderPoolCredentialResolver) Materialize(_ context.Context, profile AgentProfile) (*MaterializedCredentials, error) {
	providerID := resolveCredentialProviderID(profile)
	switch providerID {
	case "":
		return nil, nil
	case "anthropic":
		return r.materializeAnthropicAPIKey(providerID)
	case "openai-codex":
		return r.materializeCodexOAuth(providerID)
	case "google-gemini-cli":
		return r.materializeGeminiCLIOAuth(providerID)
	default:
		return nil, nil
	}
}

func (r *ProviderPoolCredentialResolver) materializeAnthropicAPIKey(providerID string) (*MaterializedCredentials, error) {
	apiKey := r.resolveAPIKey(providerID)
	if apiKey == "" {
		return nil, nil
	}
	return &MaterializedCredentials{
		Env: map[string]string{
			"ANTHROPIC_API_KEY": apiKey,
		},
	}, nil
}

func (r *ProviderPoolCredentialResolver) materializeCodexOAuth(providerID string) (*MaterializedCredentials, error) {
	token, err := r.resolveOAuthToken(providerID)
	if err != nil {
		return nil, err
	}
	if token == nil || strings.TrimSpace(token.RefreshToken) == "" {
		return nil, nil
	}

	codexHome, err := os.MkdirTemp("", "blue-agent-codex-*")
	if err != nil {
		return nil, fmt.Errorf("create Codex credential dir: %w", err)
	}

	payload := map[string]interface{}{
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
		"account_id":    strings.TrimSpace(token.Email),
		"expires_at":    codexExpiryMillis(token.TokenExpiry),
		"last_refresh":  timeutil.NowTime().UTC().Format(time.RFC3339),
		"tokens": map[string]interface{}{
			"access_token":  token.AccessToken,
			"refresh_token": token.RefreshToken,
			"account_id":    strings.TrimSpace(token.Email),
			"expires_at":    codexExpiryMillis(token.TokenExpiry),
		},
	}
	if err := writeCredentialJSON(filepath.Join(codexHome, "auth.json"), payload); err != nil {
		_ = os.RemoveAll(codexHome)
		return nil, err
	}

	return &MaterializedCredentials{
		Env: map[string]string{
			"CODEX_HOME": codexHome,
		},
		Cleanup: func() error {
			return os.RemoveAll(codexHome)
		},
	}, nil
}

func (r *ProviderPoolCredentialResolver) materializeGeminiCLIOAuth(providerID string) (*MaterializedCredentials, error) {
	token, err := r.resolveOAuthToken(providerID)
	if err != nil {
		return nil, err
	}
	if token == nil || strings.TrimSpace(token.RefreshToken) == "" {
		return nil, nil
	}

	homeDir, err := os.MkdirTemp("", "blue-agent-gemini-*")
	if err != nil {
		return nil, fmt.Errorf("create Gemini credential dir: %w", err)
	}

	configHome := filepath.Join(homeDir, ".config")
	payload := map[string]interface{}{
		"refresh_token": token.RefreshToken,
		"access_token":  token.AccessToken,
		"email":         strings.TrimSpace(token.Email),
		"project_id":    strings.TrimSpace(token.ProjectID),
		"expires_at":    geminiExpiryString(token.TokenExpiry),
		"expiry_date":   geminiExpiryMillis(token.TokenExpiry),
	}
	paths := []string{
		filepath.Join(homeDir, ".gemini", "oauth_creds.json"),
		filepath.Join(configHome, "gemini", "oauth_creds.json"),
		filepath.Join(homeDir, ".gemini-cli", "oauth_creds.json"),
	}
	for _, path := range paths {
		if err := writeCredentialJSON(path, payload); err != nil {
			_ = os.RemoveAll(homeDir)
			return nil, err
		}
	}

	return &MaterializedCredentials{
		Env: map[string]string{
			"HOME":            homeDir,
			"XDG_CONFIG_HOME": configHome,
		},
		Cleanup: func() error {
			return os.RemoveAll(homeDir)
		},
	}, nil
}

func (r *ProviderPoolCredentialResolver) resolveAPIKey(providerID string) string {
	if r == nil || r.lookupAPIKey == nil {
		return ""
	}
	apiKey, err := r.lookupAPIKey(strings.TrimSpace(providerID))
	if err != nil || strings.TrimSpace(apiKey) == "" {
		if r.logger != nil && err != nil {
			r.logger.Debug("agent session API key lookup skipped",
				zap.String("provider_id", providerID),
				zap.Error(err),
			)
		}
		return ""
	}
	return strings.TrimSpace(apiKey)
}

func (r *ProviderPoolCredentialResolver) resolveOAuthToken(providerID string) (*oauth.Token, error) {
	if r == nil || r.getOAuthSource == nil {
		return nil, nil
	}
	source := r.getOAuthSource()
	if source == nil {
		return nil, nil
	}

	tokens, err := source.GetTokens(strings.TrimSpace(providerID))
	if err != nil {
		if r.logger != nil {
			r.logger.Debug("agent session OAuth token lookup skipped",
				zap.String("provider_id", providerID),
				zap.Error(err),
			)
		}
		return nil, nil
	}

	for _, token := range tokens {
		if token == nil || strings.TrimSpace(token.ID) == "" {
			continue
		}
		accessToken, err := source.GetAccessTokenByID(providerID, token.ID)
		if err != nil || strings.TrimSpace(accessToken) == "" {
			if r.logger != nil && err != nil {
				r.logger.Debug("agent session OAuth token unusable",
					zap.String("provider_id", providerID),
					zap.String("token_id", token.ID),
					zap.Error(err),
				)
			}
			continue
		}

		current := *token
		current.AccessToken = strings.TrimSpace(accessToken)

		refreshed, refreshErr := findOAuthTokenByID(source, providerID, token.ID)
		if refreshErr == nil && refreshed != nil {
			current = *refreshed
			current.AccessToken = strings.TrimSpace(accessToken)
		}
		return &current, nil
	}

	return nil, nil
}

func findOAuthTokenByID(source OAuthCredentialSource, providerID, tokenID string) (*oauth.Token, error) {
	tokens, err := source.GetTokens(providerID)
	if err != nil {
		return nil, err
	}
	for _, token := range tokens {
		if token != nil && token.ID == tokenID {
			current := *token
			return &current, nil
		}
	}
	return nil, fmt.Errorf("oauth token %s not found for provider %s", tokenID, providerID)
}

func resolveCredentialProviderID(profile AgentProfile) string {
	if providerID := strings.ToLower(strings.TrimSpace(profile.CredentialProviderID)); providerID != "" {
		return providerID
	}

	command := strings.ToLower(strings.Join(profile.Command, " "))
	binary := normalizedCommandBinary(profile.Command)
	switch {
	case binary == "claude", binary == "claude-code", strings.Contains(command, "claude-agent-acp"):
		return "anthropic"
	case binary == "codex", strings.Contains(command, "codex-acp"):
		return "openai-codex"
	case (binary == "gemini" && commandHasArg(profile.Command, "--acp")) ||
		(strings.Contains(command, "gemini") && strings.Contains(command, "--acp")):
		return "google-gemini-cli"
	}

	for _, candidate := range []string{profile.ID, profile.Name, profile.Title} {
		switch strings.ToLower(strings.TrimSpace(candidate)) {
		case "claude", "claude-code", "claude code", "claudecode":
			return "anthropic"
		case "codex":
			return "openai-codex"
		case "gemini", "gemini-cli", "gemini cli":
			return "google-gemini-cli"
		}
	}

	return ""
}

func normalizedCommandBinary(command []string) string {
	if len(command) == 0 {
		return ""
	}
	binary := strings.TrimSpace(command[0])
	if binary == "" {
		return ""
	}
	return strings.TrimSuffix(strings.ToLower(filepath.Base(binary)), ".exe")
}

func commandHasArg(command []string, target string) bool {
	needle := strings.TrimSpace(strings.ToLower(target))
	if needle == "" {
		return false
	}
	for _, part := range command {
		if strings.TrimSpace(strings.ToLower(part)) == needle {
			return true
		}
	}
	return false
}

func writeCredentialJSON(path string, payload interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create credential dir for %s: %w", path, err)
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials for %s: %w", path, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write credentials to %s: %w", path, err)
	}
	return nil
}

func codexExpiryMillis(expiry time.Time) int64 {
	if expiry.IsZero() {
		expiry = timeutil.NowTime().Add(time.Hour)
	}
	return expiry.UTC().UnixMilli()
}

func geminiExpiryMillis(expiry time.Time) int64 {
	if expiry.IsZero() {
		expiry = timeutil.NowTime().Add(time.Hour)
	}
	return expiry.UTC().UnixMilli()
}

func geminiExpiryString(expiry time.Time) string {
	if expiry.IsZero() {
		expiry = timeutil.NowTime().Add(time.Hour)
	}
	return expiry.UTC().Format(time.RFC3339)
}

func mergeStringMap(base, overlay map[string]string) map[string]string {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	out := make(map[string]string, len(base)+len(overlay))
	for key, value := range base {
		if strings.TrimSpace(key) == "" {
			continue
		}
		out[key] = value
	}
	for key, value := range overlay {
		if strings.TrimSpace(key) == "" {
			continue
		}
		out[key] = value
	}
	return out
}
