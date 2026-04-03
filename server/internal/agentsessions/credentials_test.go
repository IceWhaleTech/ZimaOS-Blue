package agentsessions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool/oauth"
)

type stubOAuthCredentialSource struct {
	tokensByProvider map[string][]*oauth.Token
	accessByTokenID  map[string]string
	tokenErr         error
	accessErrByID    map[string]error
}

func (s *stubOAuthCredentialSource) GetTokens(providerID string) ([]*oauth.Token, error) {
	if s.tokenErr != nil {
		return nil, s.tokenErr
	}
	source := s.tokensByProvider[providerID]
	out := make([]*oauth.Token, 0, len(source))
	for _, token := range source {
		if token == nil {
			continue
		}
		cp := *token
		out = append(out, &cp)
	}
	return out, nil
}

func (s *stubOAuthCredentialSource) GetAccessTokenByID(_ string, tokenID string) (string, error) {
	if err := s.accessErrByID[tokenID]; err != nil {
		return "", err
	}
	if token, ok := s.accessByTokenID[tokenID]; ok {
		return token, nil
	}
	return "", fmt.Errorf("missing access token for %s", tokenID)
}

func TestProviderPoolCredentialResolverMaterializesCodexOAuth(t *testing.T) {
	expiry := time.Now().Add(2 * time.Hour).UTC()
	oauthSource := &stubOAuthCredentialSource{
		tokensByProvider: map[string][]*oauth.Token{
			"openai-codex": {
				{
					ID:           "codex-token-1",
					ProviderID:   "openai-codex",
					ProviderType: "codex",
					AccessToken:  "stale-access",
					RefreshToken: "refresh-token",
					TokenExpiry:  expiry,
					Email:        "acct_123",
				},
			},
		},
		accessByTokenID: map[string]string{
			"codex-token-1": "fresh-access",
		},
	}
	resolver := NewProviderPoolCredentialResolver(func() OAuthCredentialSource {
		return oauthSource
	}, nil, zap.NewNop())

	creds, err := resolver.Materialize(context.Background(), AgentProfile{
		ID:       "codex",
		Protocol: ProtocolACP,
		Command:  []string{"npx", "@zed-industries/codex-acp"},
	})
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if creds == nil {
		t.Fatal("Materialize() returned nil credentials")
	}

	codexHome := creds.Env["CODEX_HOME"]
	if codexHome == "" {
		t.Fatal("expected CODEX_HOME to be set")
	}

	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		AccountID    string `json:"account_id"`
		Tokens       struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			AccountID    string `json:"account_id"`
		} `json:"tokens"`
	}
	data, readErr := os.ReadFile(filepath.Join(codexHome, "auth.json"))
	if readErr != nil {
		t.Fatalf("ReadFile(auth.json) error = %v", readErr)
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("json.Unmarshal(auth.json) error = %v", err)
	}
	if payload.Tokens.AccessToken != "fresh-access" {
		t.Fatalf("nested access token = %q, want %q", payload.Tokens.AccessToken, "fresh-access")
	}
	if payload.Tokens.RefreshToken != "refresh-token" {
		t.Fatalf("nested refresh token = %q, want %q", payload.Tokens.RefreshToken, "refresh-token")
	}
	if payload.AccountID != "acct_123" || payload.Tokens.AccountID != "acct_123" {
		t.Fatalf("account id payload = %+v, want acct_123", payload)
	}

	if creds.Cleanup == nil {
		t.Fatal("expected cleanup to be set")
	}
	if err := creds.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if _, err := os.Stat(codexHome); !os.IsNotExist(err) {
		t.Fatalf("CODEX_HOME still exists after cleanup: %v", err)
	}
}

func TestProviderPoolCredentialResolverMaterializesCodexOAuthFromCLICommand(t *testing.T) {
	expiry := time.Now().Add(2 * time.Hour).UTC()
	oauthSource := &stubOAuthCredentialSource{
		tokensByProvider: map[string][]*oauth.Token{
			"openai-codex": {
				{
					ID:           "codex-cli-token",
					ProviderID:   "openai-codex",
					ProviderType: "codex",
					AccessToken:  "stale-access",
					RefreshToken: "refresh-token",
					TokenExpiry:  expiry,
					Email:        "acct_cli",
				},
			},
		},
		accessByTokenID: map[string]string{
			"codex-cli-token": "fresh-access",
		},
	}
	resolver := NewProviderPoolCredentialResolver(func() OAuthCredentialSource {
		return oauthSource
	}, nil, zap.NewNop())

	creds, err := resolver.Materialize(context.Background(), AgentProfile{
		ID:       "custom-acp",
		Protocol: ProtocolACP,
		Name:     "project-agent",
		Title:    "Project Agent",
		Command:  []string{"/usr/local/bin/codex"},
	})
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if creds == nil {
		t.Fatal("Materialize() returned nil credentials")
	}
	if got := creds.Env["CODEX_HOME"]; got == "" {
		t.Fatal("expected CODEX_HOME to be set for codex CLI command")
	}
	if err := creds.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
}

func TestProviderPoolCredentialResolverMaterializesBuiltinTemplateByCredentialProviderID(t *testing.T) {
	expiry := time.Now().Add(2 * time.Hour).UTC()
	oauthSource := &stubOAuthCredentialSource{
		tokensByProvider: map[string][]*oauth.Token{
			"openai-codex": {
				{
					ID:           "codex-template-token",
					ProviderID:   "openai-codex",
					ProviderType: "codex",
					AccessToken:  "stale-access",
					RefreshToken: "refresh-token",
					TokenExpiry:  expiry,
					Email:        "acct_template",
				},
			},
		},
		accessByTokenID: map[string]string{
			"codex-template-token": "fresh-access",
		},
	}
	resolver := NewProviderPoolCredentialResolver(func() OAuthCredentialSource {
		return oauthSource
	}, nil, zap.NewNop())

	creds, err := resolver.Materialize(context.Background(), AgentProfile{
		ID:                   "codex",
		Protocol:             ProtocolACP,
		TemplateOnly:         true,
		CredentialProviderID: "openai-codex",
	})
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if creds == nil {
		t.Fatal("Materialize() returned nil credentials")
	}
	if got := creds.Env["CODEX_HOME"]; got == "" {
		t.Fatal("expected CODEX_HOME to be set from credential_provider_id without command fallback")
	}
	if err := creds.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
}

func TestProviderPoolCredentialResolverMaterializesGeminiOAuth(t *testing.T) {
	expiry := time.Now().Add(90 * time.Minute).UTC()
	oauthSource := &stubOAuthCredentialSource{
		tokensByProvider: map[string][]*oauth.Token{
			"google-gemini-cli": {
				{
					ID:           "gemini-token-1",
					ProviderID:   "google-gemini-cli",
					ProviderType: "gemini-cli",
					AccessToken:  "stale-access",
					RefreshToken: "refresh-token",
					TokenExpiry:  expiry,
					Email:        "user@example.com",
					ProjectID:    "project-123",
				},
			},
		},
		accessByTokenID: map[string]string{
			"gemini-token-1": "fresh-access",
		},
	}
	resolver := NewProviderPoolCredentialResolver(func() OAuthCredentialSource {
		return oauthSource
	}, nil, zap.NewNop())

	creds, err := resolver.Materialize(context.Background(), AgentProfile{
		ID:       "gemini",
		Protocol: ProtocolACP,
		Command:  []string{"gemini", "--acp"},
	})
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if creds == nil {
		t.Fatal("Materialize() returned nil credentials")
	}

	homeDir := creds.Env["HOME"]
	if homeDir == "" {
		t.Fatal("expected HOME to be set")
	}
	if got := creds.Env["XDG_CONFIG_HOME"]; got != filepath.Join(homeDir, ".config") {
		t.Fatalf("XDG_CONFIG_HOME = %q, want %q", got, filepath.Join(homeDir, ".config"))
	}

	paths := []string{
		filepath.Join(homeDir, ".gemini", "oauth_creds.json"),
		filepath.Join(homeDir, ".config", "gemini", "oauth_creds.json"),
		filepath.Join(homeDir, ".gemini-cli", "oauth_creds.json"),
	}
	for _, path := range paths {
		var payload struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			Email        string `json:"email"`
			ProjectID    string `json:"project_id"`
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("ReadFile(%s) error = %v", path, readErr)
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			t.Fatalf("json.Unmarshal(%s) error = %v", path, err)
		}
		if payload.AccessToken != "fresh-access" {
			t.Fatalf("%s access token = %q, want %q", path, payload.AccessToken, "fresh-access")
		}
		if payload.RefreshToken != "refresh-token" {
			t.Fatalf("%s refresh token = %q, want %q", path, payload.RefreshToken, "refresh-token")
		}
		if payload.Email != "user@example.com" {
			t.Fatalf("%s email = %q, want %q", path, payload.Email, "user@example.com")
		}
		if payload.ProjectID != "project-123" {
			t.Fatalf("%s project_id = %q, want %q", path, payload.ProjectID, "project-123")
		}
	}

	if creds.Cleanup == nil {
		t.Fatal("expected cleanup to be set")
	}
	if err := creds.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if _, err := os.Stat(homeDir); !os.IsNotExist(err) {
		t.Fatalf("HOME still exists after cleanup: %v", err)
	}
}

func TestProviderPoolCredentialResolverMaterializesGeminiOAuthFromCLICommand(t *testing.T) {
	expiry := time.Now().Add(90 * time.Minute).UTC()
	oauthSource := &stubOAuthCredentialSource{
		tokensByProvider: map[string][]*oauth.Token{
			"google-gemini-cli": {
				{
					ID:           "gemini-cli-token",
					ProviderID:   "google-gemini-cli",
					ProviderType: "gemini-cli",
					AccessToken:  "stale-access",
					RefreshToken: "refresh-token",
					TokenExpiry:  expiry,
					Email:        "user@example.com",
					ProjectID:    "project-123",
				},
			},
		},
		accessByTokenID: map[string]string{
			"gemini-cli-token": "fresh-access",
		},
	}
	resolver := NewProviderPoolCredentialResolver(func() OAuthCredentialSource {
		return oauthSource
	}, nil, zap.NewNop())

	creds, err := resolver.Materialize(context.Background(), AgentProfile{
		ID:       "custom-acp",
		Protocol: ProtocolACP,
		Name:     "project-agent",
		Title:    "Project Agent",
		Command:  []string{"/opt/bin/gemini", "--acp"},
	})
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if creds == nil {
		t.Fatal("Materialize() returned nil credentials")
	}
	if got := creds.Env["HOME"]; got == "" {
		t.Fatal("expected HOME to be set for gemini CLI command")
	}
	if err := creds.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
}

func TestProviderPoolCredentialResolverMaterializesCustomACPByCredentialProviderID(t *testing.T) {
	expiry := time.Now().Add(2 * time.Hour).UTC()
	oauthSource := &stubOAuthCredentialSource{
		tokensByProvider: map[string][]*oauth.Token{
			"openai-codex": {
				{
					ID:           "codex-custom-token",
					ProviderID:   "openai-codex",
					ProviderType: "codex",
					AccessToken:  "stale-access",
					RefreshToken: "refresh-token",
					TokenExpiry:  expiry,
					Email:        "acct_custom",
				},
			},
		},
		accessByTokenID: map[string]string{
			"codex-custom-token": "fresh-access",
		},
	}
	resolver := NewProviderPoolCredentialResolver(func() OAuthCredentialSource {
		return oauthSource
	}, nil, zap.NewNop())

	creds, err := resolver.Materialize(context.Background(), AgentProfile{
		ID:                   "custom-codex",
		Protocol:             ProtocolACP,
		Command:              []string{"/opt/acp/bin/codex", "serve"},
		CredentialProviderID: "openai-codex",
	})
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if creds == nil {
		t.Fatal("Materialize() returned nil credentials")
	}
	if got := creds.Env["CODEX_HOME"]; got == "" {
		t.Fatal("expected CODEX_HOME to be set for custom ACP profile")
	}
	if err := creds.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
}

func TestProviderPoolCredentialResolverInfersClaudeAPIKey(t *testing.T) {
	resolver := NewProviderPoolCredentialResolver(
		nil,
		func(providerID string) (string, error) {
			if providerID != "anthropic" {
				return "", fmt.Errorf("unexpected provider id %q", providerID)
			}
			return "sk-ant-test", nil
		},
		zap.NewNop(),
	)

	creds, err := resolver.Materialize(context.Background(), AgentProfile{
		ID:       "claude",
		Protocol: ProtocolACP,
		Command:  []string{"npx", "-y", "@zed-industries/claude-agent-acp"},
	})
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if creds == nil {
		t.Fatal("Materialize() returned nil credentials")
	}
	if got := creds.Env["ANTHROPIC_API_KEY"]; got != "sk-ant-test" {
		t.Fatalf("ANTHROPIC_API_KEY = %q, want %q", got, "sk-ant-test")
	}
}

func TestProviderPoolCredentialResolverInfersClaudeAPIKeyFromCLICommand(t *testing.T) {
	resolver := NewProviderPoolCredentialResolver(
		nil,
		func(providerID string) (string, error) {
			if providerID != "anthropic" {
				return "", fmt.Errorf("unexpected provider id %q", providerID)
			}
			return "sk-ant-cli", nil
		},
		zap.NewNop(),
	)

	creds, err := resolver.Materialize(context.Background(), AgentProfile{
		ID:       "custom-acp",
		Protocol: ProtocolACP,
		Name:     "project-agent",
		Title:    "Project Agent",
		Command:  []string{"/usr/local/bin/claude"},
	})
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if creds == nil {
		t.Fatal("Materialize() returned nil credentials")
	}
	if got := creds.Env["ANTHROPIC_API_KEY"]; got != "sk-ant-cli" {
		t.Fatalf("ANTHROPIC_API_KEY = %q, want %q", got, "sk-ant-cli")
	}
}

func TestProviderPoolCredentialResolverSkipsUnavailableCredentials(t *testing.T) {
	resolver := NewProviderPoolCredentialResolver(func() OAuthCredentialSource {
		return &stubOAuthCredentialSource{tokenErr: fmt.Errorf("oauth store offline")}
	}, nil, zap.NewNop())

	creds, err := resolver.Materialize(context.Background(), AgentProfile{
		ID:       "codex",
		Protocol: ProtocolACP,
		Command:  []string{"npx", "@zed-industries/codex-acp"},
	})
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if creds != nil {
		t.Fatalf("expected nil credentials when provider-pool source is unavailable, got %+v", creds)
	}
}
