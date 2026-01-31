package teams

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel"
)

func TestNew(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.TeamsConfig{
		Enabled:     true,
		AppID:       "test-app-id",
		AppPassword: "test-app-password",
	}

	ch := New(cfg, logger)

	if ch == nil {
		t.Fatal("expected channel to be created")
	}
	if ch.Name() != "teams" {
		t.Errorf("expected name 'teams', got '%s'", ch.Name())
	}
	if ch.Type() != "teams" {
		t.Errorf("expected type 'teams', got '%s'", ch.Type())
	}
}

func TestChannel_Info(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.TeamsConfig{
		Enabled:     true,
		AppID:       "test-app-id",
		AppPassword: "test-app-password",
	}

	ch := New(cfg, logger)
	info := ch.Info()

	if info.Name != "teams" {
		t.Errorf("expected name 'teams', got '%s'", info.Name)
	}
	if info.Type != "teams" {
		t.Errorf("expected type 'teams', got '%s'", info.Type)
	}
	if info.Status != channel.StatusDisconnected {
		t.Errorf("expected status 'disconnected', got '%s'", info.Status)
	}
	if !info.Enabled {
		t.Error("expected enabled to be true")
	}
}

func TestChannel_IsConnected(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.TeamsConfig{
		Enabled:     true,
		AppID:       "test-app-id",
		AppPassword: "test-app-password",
	}

	ch := New(cfg, logger)

	if ch.IsConnected() {
		t.Error("expected channel to not be connected initially")
	}
}

func TestChannel_Messages(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.TeamsConfig{
		Enabled:     true,
		AppID:       "test-app-id",
		AppPassword: "test-app-password",
	}

	ch := New(cfg, logger)
	messages := ch.Messages()

	if messages == nil {
		t.Error("expected messages channel to not be nil")
	}
}

func TestChannel_Stop_NotStarted(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.TeamsConfig{
		Enabled:     true,
		AppID:       "test-app-id",
		AppPassword: "test-app-password",
	}

	ch := New(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ch.Stop(ctx)
	if err != nil {
		t.Errorf("expected no error stopping non-started channel, got: %v", err)
	}
}

func TestChannel_Start_Success(t *testing.T) {
	// Create mock OAuth server
	oauthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/botframework.com/oauth2/v2.0/token" {
			response := map[string]interface{}{
				"access_token": "test-access-token",
				"token_type":   "Bearer",
				"expires_in":   3600,
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer oauthServer.Close()

	logger := zap.NewNop()
	cfg := channel.TeamsConfig{
		Enabled:     true,
		AppID:       "test-app-id",
		AppPassword: "test-app-password",
	}

	ch := NewWithOptions(cfg, logger, oauthServer.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ch.Start(ctx)
	if err != nil {
		t.Fatalf("expected no error starting channel, got: %v", err)
	}

	if !ch.IsConnected() {
		t.Error("expected channel to be connected after start")
	}

	// Clean up
	ch.Stop(ctx)
}

func TestChannel_Start_AuthFailure(t *testing.T) {
	// Create mock OAuth server that returns error
	oauthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		response := map[string]interface{}{
			"error":             "invalid_client",
			"error_description": "Invalid client credentials",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer oauthServer.Close()

	logger := zap.NewNop()
	cfg := channel.TeamsConfig{
		Enabled:     true,
		AppID:       "invalid-app-id",
		AppPassword: "invalid-password",
	}

	ch := NewWithOptions(cfg, logger, oauthServer.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ch.Start(ctx)
	if err == nil {
		t.Fatal("expected error starting channel with invalid credentials")
	}

	if ch.IsConnected() {
		t.Error("expected channel to not be connected after auth failure")
	}
}

func TestChannel_Send_NotInitialized(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.TeamsConfig{
		Enabled:     true,
		AppID:       "test-app-id",
		AppPassword: "test-app-password",
	}

	ch := New(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ch.Send(ctx, channel.OutgoingMessage{
		ChatID:  "test-chat",
		Content: "Hello",
	})

	if err == nil {
		t.Error("expected error sending message when not initialized")
	}
}

func TestChannel_SendStreaming_NotInitialized(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.TeamsConfig{
		Enabled:     true,
		AppID:       "test-app-id",
		AppPassword: "test-app-password",
	}

	ch := New(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	content := make(chan string)
	done := make(chan struct{})

	err := ch.SendStreaming(ctx, "test-chat", "", content, done)

	if err == nil {
		t.Error("expected error sending streaming message when not initialized")
	}
}

func TestChannel_isTeamAllowed(t *testing.T) {
	tests := []struct {
		name         string
		allowedTeams []string
		teamID       string
		expected     bool
	}{
		{
			name:         "empty allowed list allows all",
			allowedTeams: []string{},
			teamID:       "any-team",
			expected:     true,
		},
		{
			name:         "team ID in allowed list",
			allowedTeams: []string{"team-1", "team-2"},
			teamID:       "team-1",
			expected:     true,
		},
		{
			name:         "team ID not in allowed list",
			allowedTeams: []string{"team-1", "team-2"},
			teamID:       "team-3",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			cfg := channel.TeamsConfig{
				Enabled:      true,
				AppID:        "test-app-id",
				AppPassword:  "test-app-password",
				AllowedTeams: tt.allowedTeams,
			}

			ch := New(cfg, logger)
			result := ch.isTeamAllowed(tt.teamID)

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestChannel_isUserAllowed(t *testing.T) {
	tests := []struct {
		name         string
		allowedUsers []string
		userID       string
		expected     bool
	}{
		{
			name:         "empty allowed list allows all",
			allowedUsers: []string{},
			userID:       "any-user",
			expected:     true,
		},
		{
			name:         "user ID in allowed list",
			allowedUsers: []string{"user-1", "user-2"},
			userID:       "user-1",
			expected:     true,
		},
		{
			name:         "user ID not in allowed list",
			allowedUsers: []string{"user-1", "user-2"},
			userID:       "user-3",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			cfg := channel.TeamsConfig{
				Enabled:      true,
				AppID:        "test-app-id",
				AppPassword:  "test-app-password",
				AllowedUsers: tt.allowedUsers,
			}

			ch := New(cfg, logger)
			result := ch.isUserAllowed(tt.userID)

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
